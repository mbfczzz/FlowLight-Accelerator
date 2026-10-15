package control

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"log"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
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
	// TrustedProxies are reverse proxies whose X-Real-IP header names the
	// real client, used for login throttling.
	TrustedProxies []netip.Prefix
	// MetricsToken is a long-lived read-only credential for scraping
	// /metrics. Access tokens are too short lived for a scrape config.
	MetricsToken string
}
type Server struct {
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
	m.HandleFunc("GET /metrics", s.metricsAuth(s.metrics))
	for _, prefix := range []string{"/api/client/v1", "/api/admin/v1"} {
		m.HandleFunc("POST "+prefix+"/auth/login", s.login)
		m.HandleFunc("POST "+prefix+"/auth/refresh", s.refresh)
		m.HandleFunc("POST "+prefix+"/auth/logout", s.auth(s.logout))
		m.HandleFunc("GET "+prefix+"/me", s.auth(func(w http.ResponseWriter, r *http.Request) { write(w, 200, actor(r)) }))
	}
	m.HandleFunc("GET /api/client/v1/games", s.auth(s.games))
	m.HandleFunc("GET /api/client/v1/covers/{name}", s.coverFile)
	m.HandleFunc("GET /api/client/v1/games/{id}/regions", s.auth(s.regions))
	m.HandleFunc("POST /api/client/v1/devices", s.auth(s.device))
	m.HandleFunc("GET /api/client/v1/lines", s.auth(s.clientLines))
	m.HandleFunc("POST /api/client/v1/egress-bindings", s.auth(s.clientBinding))
	m.HandleFunc("GET /api/client/v1/egress-bindings", s.auth(s.bindings))
	m.HandleFunc("POST /api/client/v1/sessions", s.auth(s.beginSession))
	m.HandleFunc("GET /api/client/v1/sessions/{id}", s.auth(s.getSession))
	m.HandleFunc("POST /api/client/v1/sessions/{id}/heartbeat", s.auth(s.heartbeatSession))
	m.HandleFunc("POST /api/client/v1/sessions/{id}/stop", s.auth(s.stopSession))
	// No token: a client too old to sign in must still find its update.
	m.HandleFunc("GET /api/client/v1/releases/latest", s.latestRelease)
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
	m.HandleFunc("PUT /api/admin/v1/games/{id}/cover", s.auth(s.setCover, "operator", "publisher"))
	m.HandleFunc("DELETE /api/admin/v1/games/{id}/cover", s.auth(s.clearCover, "operator", "publisher"))
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
	m.HandleFunc("POST /api/node/v1/certificate", s.nodeAuth(s.renewCertificate))
	m.HandleFunc("GET /api/node/v1/jobs", s.nodeAuth(s.claimJobs))
	m.HandleFunc("POST /api/node/v1/jobs/{id}/result", s.nodeAuth(s.jobResult))
	m.HandleFunc("POST /api/node/v1/session-events", s.nodeAuth(s.nodeEvents))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if failure := recover(); failure != nil {
				log.Printf("panic serving %s %s: %v\n%s", r.Method, r.URL.Path, failure, debug.Stack())
				fail(w, 500, "控制服务内部错误，请稍后重试；如持续出现，请查看控制服务日志")
			}
		}()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if origin := r.Header.Get("Origin"); origin != "" {
			if origin != s.Config.Origin {
				fail(w, 403, "请求来源不被允许：请通过控制服务 ADMIN_ORIGIN 中配置的后台地址访问")
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
	lastPurge := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if e := s.Reap(); e != nil {
				log.Printf("session reaper: %v", e)
			}
			if time.Since(lastPurge) > time.Hour {
				if e := s.Purge(); e != nil {
					log.Printf("retention purge: %v", e)
				}
				lastPurge = time.Now()
			}
		}
	}
}

// Retention for rows nothing reads once they are spent.
const (
	idempotencyRetention = 24 * time.Hour
	jobRetention         = 30 * 24 * time.Hour
)

// Purge deletes expired tokens, old idempotency keys and finished jobs.
// Sessions and audits are kept: they are the operational record.
func (s *Server) Purge() error {
	now := time.Now().UTC()
	db := s.Store.DB
	if e := db.Where("expires_at < ?", now).Delete(&Token{}).Error; e != nil {
		return e
	}
	if e := db.Where("created_at < ?", now.Add(-idempotencyRetention)).Delete(&Idempotency{}).Error; e != nil {
		return e
	}
	return db.Where("created_at < ? AND status IN ?", now.Add(-jobRetention), []string{"succeeded", "failed", "cancelled"}).Delete(&Job{}).Error
}

// metricsAuth accepts the operator's long-lived scrape token, or a bearer
// access token from an administrator. Access tokens expire in 15 minutes, so
// a scrape configuration cannot rely on one.
func (s *Server) metricsAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Config.MetricsToken != "" && subtle.ConstantTimeCompare([]byte(bearer(r)), []byte(s.Config.MetricsToken)) == 1 {
			next(w, r)
			return
		}
		s.auth(next, "viewer", "operator", "publisher")(w, r)
	}
}
func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	out := map[string]int64{}
	var n int64
	s.Store.DB.Model(&Game{}).Count(&n)
	out["games"] = n
	s.Store.DB.Model(&Session{}).Where("state <> ?", "CLOSED").Count(&n)
	out["active_sessions"] = n
	// Stored timestamps are UTC and SQLite compares them as text, so the
	// bound must be UTC as well.
	s.Store.DB.Model(&Node{}).Where("enabled = ? AND last_seen > ?", true, time.Now().UTC().Add(-s.Config.NodeTimeout)).Count(&n)
	out["online_nodes"] = n
	s.Store.DB.Model(&LineVersion{}).Where("status = ?", "ready").Count(&n)
	out["verified_versions"] = n
	// What the setup checklist needs: each step is done when its count is
	// above zero.
	s.Store.DB.Model(&Game{}).Where("published_version > ?", 0).Count(&n)
	out["published_games"] = n
	s.Store.DB.Model(&Node{}).Count(&n)
	out["nodes"] = n
	s.Store.DB.Model(&Endpoint{}).Count(&n)
	out["endpoints"] = n
	s.Store.DB.Model(&Line{}).Count(&n)
	out["lines"] = n
	s.Store.DB.Model(&User{}).Where("role = ?", "player").Count(&n)
	out["players"] = n
	write(w, 200, out)
}
func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	var rows []struct {
		State string
		Total int64
	}
	if e := s.Store.DB.Model(&Session{}).Select("state, count(*) as total").Group("state").Scan(&rows).Error; e != nil {
		fail(w, 503, "session state unavailable")
		return
	}
	counts := map[string]int64{}
	for _, row := range rows {
		counts[row.State] = row.Total
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	for _, state := range []string{"PREPARING", "CONNECTING", "ACTIVE", "DEGRADED", "RECONNECTING", "STOPPING"} {
		fmt.Fprintf(w, "flowlight_sessions{state=%q} %d\n", state, counts[state])
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

// listPage serves one page of an admin list: ?limit= (default 200, at most
// 1000) and ?offset=. Rows are ordered by the given column and then by id,
// so consecutive pages neither repeat nor skip rows.
func listPage[T any](w http.ResponseWriter, r *http.Request, db *gorm.DB, order string, annotate ...func([]T) error) {
	limit, offset := 200, 0
	if n, e := strconv.Atoi(r.URL.Query().Get("limit")); e == nil && n > 0 {
		limit = min(n, 1000)
	}
	if n, e := strconv.Atoi(r.URL.Query().Get("offset")); e == nil && n > 0 {
		offset = n
	}
	if order != "" {
		db = db.Order(order)
	}
	v := []T{}
	e := db.Order("id").Limit(limit).Offset(offset).Find(&v).Error
	for _, add := range annotate {
		if e == nil {
			e = add(v)
		}
	}
	respond(w, v, e)
}

// annotateLines attaches the newest version's status and failure reason.
func (s *Server) annotateLines(rows []Line) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]string, len(rows))
	for i, l := range rows {
		ids[i] = l.ID
	}
	var versions []LineVersion
	if e := s.Store.DB.Select("line_id", "number", "status", "error").Where("line_id IN ?", ids).Order("number DESC").Find(&versions).Error; e != nil {
		return e
	}
	latest := map[string]LineVersion{}
	for _, v := range versions {
		if _, seen := latest[v.LineID]; !seen {
			latest[v.LineID] = v
		}
	}
	for i := range rows {
		if v, ok := latest[rows[i].ID]; ok {
			rows[i].LatestStatus = v.Status
			rows[i].LatestError = v.Error
		}
	}
	return nil
}
func (s *Server) list(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db := s.Store.DB
		switch kind {
		case "games":
			listPage[Game](w, r, db, "")
		case "nodes":
			listPage[Node](w, r, db, "")
		case "endpoints":
			listPage[Endpoint](w, r, db, "")
		case "lines":
			listPage(w, r, db, "", s.annotateLines)
		case "bindings":
			listPage[Binding](w, r, db, "")
		case "users":
			listPage[User](w, r, db, "")
		case "devices":
			listPage[Device](w, r, db, "")
		case "sessions":
			listPage[Session](w, r, db, "created_at DESC")
		case "jobs":
			listPage[Job](w, r, db, "created_at DESC")
		case "audits":
			listPage[Audit](w, r, db, "created_at DESC")
		case "releases":
			listPage[Release](w, r, db, "created_at DESC")
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

var errInvalid = badRequest("提交的内容格式不正确，请刷新页面后重试")

// invalid marks an error the client caused and can correct. respond reports
// these as 400 with their message; any other error is internal.
type invalid struct{ error }

func badRequest(message string) error { return invalid{errors.New(message)} }
func isInvalid(e error) bool {
	var i invalid
	return errors.As(e, &i)
}

// nameOK accepts a non-blank name of at most 100 characters, the size of
// the username column.
func nameOK(s string) bool {
	return len(strings.TrimSpace(s)) > 0 && utf8.RuneCountInString(s) <= 100
}
