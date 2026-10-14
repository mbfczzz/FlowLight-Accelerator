package control

import (
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	ProbeNodeID    string
	Origin         string
	AllowImported  bool
	RequireMTLS    bool
	LeaseDuration  time.Duration
	PrepareTimeout time.Duration
	NodeTimeout    time.Duration
	ReclaimGrace   time.Duration
	ProbeURL       string
	ProbeUDP       string
	CAPath         string
}
type Server struct {
	cache   *redis.Client
	Store   *Store
	Config  Config
	limiter limiter
	CA      *Authority
}

func New(store *Store, c Config) (*Server, error) {
	if c.LeaseDuration == 0 {
		c.LeaseDuration = 90 * time.Second
	}
	if c.PrepareTimeout == 0 {
		c.PrepareTimeout = 2 * time.Minute
	}
	if c.NodeTimeout == 0 {
		c.NodeTimeout = 45 * time.Second
	}
	if c.ReclaimGrace == 0 {
		c.ReclaimGrace = 15 * time.Second
	}
	ca, e := LoadAuthority(c.CAPath)
	if e != nil {
		return nil, e
	}
	return &Server{Store: store, Config: c, CA: ca, limiter: limiter{Entries: map[string]limitEntry{}}}, nil
}
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		db, _ := s.Store.DB.DB()
		if db.PingContext(r.Context()) != nil {
			fail(w, 503, "database unavailable")
			return
		}
		write(w, 200, map[string]string{"status": "ok", "service": "flowlight-control", "version": "0.1.0"})
	})
	m.HandleFunc("GET /metrics", s.auth(s.metrics, "viewer", "operator", "publisher"))
	for _, prefix := range []string{"/api/client/v1", "/api/admin/v1"} {
		m.HandleFunc("POST "+prefix+"/auth/login", s.login)
		m.HandleFunc("POST "+prefix+"/auth/refresh", s.refresh)
		m.HandleFunc("POST "+prefix+"/auth/logout", s.auth(s.logout))
		m.HandleFunc("GET "+prefix+"/me", s.auth(func(w http.ResponseWriter, r *http.Request) { write(w, 200, actor(r)) }))
	}
	m.HandleFunc("GET /api/client/v1/games", s.auth(s.games))
	m.HandleFunc("GET /api/client/v1/games/{id}/regions", s.auth(s.regions))
	m.HandleFunc("POST /api/client/v1/devices", s.auth(s.device))
	m.HandleFunc("GET /api/client/v1/lines", s.auth(s.clientLines))
	m.HandleFunc("POST /api/client/v1/egress-bindings", s.auth(s.clientBinding))
	m.HandleFunc("GET /api/client/v1/egress-bindings", s.auth(s.bindings))
	m.HandleFunc("POST /api/client/v1/sessions", s.auth(s.beginSession))
	m.HandleFunc("GET /api/client/v1/sessions/{id}", s.auth(s.getSession))
	m.HandleFunc("POST /api/client/v1/sessions/{id}/heartbeat", s.auth(s.heartbeatSession))
	m.HandleFunc("POST /api/client/v1/sessions/{id}/stop", s.auth(s.stopSession))
	m.HandleFunc("GET /api/client/v1/releases/latest", s.auth(s.latestRelease))
	for _, kind := range []string{"games", "nodes", "endpoints", "lines", "bindings", "users", "devices", "sessions", "jobs", "audits", "releases"} {
		m.HandleFunc("GET /api/admin/v1/"+kind, s.auth(s.list(kind), "viewer", "operator", "publisher"))
	}
	for _, kind := range []string{"games", "nodes", "endpoints", "lines", "bindings", "users", "releases"} {
		roles := []string{"operator", "publisher"}
		if kind == "users" || kind == "bindings" {
			roles = []string{"admin"}
		}
		m.HandleFunc("POST /api/admin/v1/"+kind, s.auth(s.save(kind), roles...))
		m.HandleFunc("PATCH /api/admin/v1/"+kind+"/{id}", s.auth(s.save(kind), roles...))
	}
	m.HandleFunc("GET /api/admin/v1/overview", s.auth(s.overview, "viewer", "operator", "publisher"))
	m.HandleFunc("GET /api/admin/v1/jobs/{id}", s.auth(s.getJob, "viewer", "operator", "publisher"))
	m.HandleFunc("POST /api/admin/v1/games/{id}/profiles/publish", s.auth(s.publishGame, "publisher"))
	m.HandleFunc("POST /api/admin/v1/games/{id}/profiles/{version}/publish", s.auth(s.publishGame, "publisher"))
	m.HandleFunc("POST /api/admin/v1/lines/{id}/validate", s.auth(s.validateLine, "operator", "publisher"))
	m.HandleFunc("POST /api/admin/v1/lines/{id}/publish", s.auth(s.publishLine, "publisher"))
	m.HandleFunc("POST /api/admin/v1/lines/{id}/rollback", s.auth(s.rollbackLine, "publisher"))
	m.HandleFunc("POST /api/admin/v1/lines/{id}/drain", s.auth(s.drainLine, "publisher"))
	m.HandleFunc("POST /api/admin/v1/lines/{id}/terminate", s.auth(s.terminateLine, "publisher"))
	m.HandleFunc("GET /api/admin/v1/lines/{id}/versions", s.auth(s.versions, "viewer", "operator", "publisher"))
	m.HandleFunc("POST /api/admin/v1/nodes/{id}/enrollment", s.auth(s.enrollment, "operator"))
	m.HandleFunc("POST /api/admin/v1/sessions/{id}/stop", s.auth(s.stopSession, "operator"))
	m.HandleFunc("POST /api/node/v1/register", s.registerNode)
	m.HandleFunc("POST /api/node/v1/heartbeat", s.nodeAuth(s.nodeHeartbeat))
	m.HandleFunc("GET /api/node/v1/jobs", s.nodeAuth(s.claimJobs))
	m.HandleFunc("POST /api/node/v1/jobs/{id}/result", s.nodeAuth(s.jobResult))
	m.HandleFunc("POST /api/node/v1/session-events", s.nodeAuth(s.nodeEvents))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				fail(w, 500, "internal request failure")
			}
		}()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if origin := r.Header.Get("Origin"); origin != "" {
			if origin != s.Config.Origin {
				fail(w, 403, "origin is not allowed")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Idempotency-Key")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		m.ServeHTTP(w, r)
	})
}
func (s *Server) RunReaper(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = s.Reap()
			s.indexLeases(ctx)
		}
	}
}
func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	out := map[string]int64{}
	var n int64
	s.Store.DB.Model(&Game{}).Count(&n)
	out["games"] = n
	s.Store.DB.Model(&Session{}).Where("state <> ?", "CLOSED").Count(&n)
	out["active_sessions"] = n
	s.Store.DB.Model(&Node{}).Where("enabled = ? AND last_seen > ?", true, time.Now().Add(-s.Config.NodeTimeout)).Count(&n)
	out["online_nodes"] = n
	s.Store.DB.Model(&LineVersion{}).Where("status = ?", "ready").Count(&n)
	out["verified_versions"] = n
	write(w, 200, out)
}
func ptr(m map[string]int64, k string) *int64 { return new(int64) }
func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	for _, state := range []string{"PREPARING", "CONNECTING", "ACTIVE", "DEGRADED", "RECONNECTING", "STOPPING"} {
		var n int64
		s.Store.DB.Model(&Session{}).Where("state = ?", state).Count(&n)
		fmt.Fprintf(w, "flowlight_sessions{state=%q} %d\n", state, n)
	}
}
func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	var j Job
	e := s.Store.DB.First(&j, "id = ?", r.PathValue("id")).Error
	respond(w, j, e)
}
func load[T any](db *gorm.DB, id string) (T, error) {
	var v T
	e := db.First(&v, "id = ?", id).Error
	return v, e
}
func listRows[T any](w http.ResponseWriter, db *gorm.DB) {
	v := []T{}
	e := db.Limit(1000).Find(&v).Error
	respond(w, v, e)
}
func (s *Server) list(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db := s.Store.DB
		switch kind {
		case "games":
			listRows[Game](w, db)
		case "nodes":
			listRows[Node](w, db)
		case "endpoints":
			listRows[Endpoint](w, db)
		case "lines":
			listRows[Line](w, db)
		case "bindings":
			listRows[Binding](w, db)
		case "users":
			listRows[User](w, db)
		case "devices":
			listRows[Device](w, db)
		case "sessions":
			listRows[Session](w, db.Order("created_at DESC"))
		case "jobs":
			listRows[Job](w, db.Order("created_at DESC"))
		case "audits":
			listRows[Audit](w, db.Order("created_at DESC"))
		case "releases":
			listRows[Release](w, db.Order("created_at DESC"))
		}
	}
}
func cleanID(r *http.Request) string {
	id := r.PathValue("id")
	if id == "" {
		id = ID()
	}
	return id
}

var errInvalid = errors.New("invalid resource configuration")

func nameOK(s string) bool { return len(strings.TrimSpace(s)) > 0 && len(s) < 150 }
