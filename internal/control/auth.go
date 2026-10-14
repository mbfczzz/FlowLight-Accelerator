package control

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type contextKey string

const userKey contextKey = "user"
const nodeKey contextKey = "node"

func actor(r *http.Request) User { return r.Context().Value(userKey).(User) }
func bearer(r *http.Request) string {
	s := r.Header.Get("Authorization")
	if strings.HasPrefix(s, "Bearer ") {
		return strings.TrimPrefix(s, "Bearer ")
	}
	return ""
}
func (s *Server) auth(next http.HandlerFunc, roles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var token Token
		if raw := bearer(r); raw == "" || s.Store.DB.First(&token, "hash = ? AND kind = ?", hash(raw), "access").Error != nil || token.Used || time.Now().After(token.ExpiresAt) {
			fail(w, 401, "authentication required")
			return
		}
		var u User
		if s.Store.DB.First(&u, "id = ?", token.UserID).Error != nil || !u.Enabled {
			fail(w, 401, "account unavailable")
			return
		}
		if len(roles) > 0 {
			ok := u.Role == "admin"
			for _, role := range roles {
				ok = ok || role == u.Role
			}
			if !ok {
				fail(w, 403, "insufficient role")
				return
			}
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	}
}
func (s *Server) nodeAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var n Node
		if raw := bearer(r); raw == "" || s.Store.DB.First(&n, "token_hash = ?", hash(raw)).Error != nil || !n.Enabled {
			fail(w, 401, "node identity required")
			return
		}
		if s.Config.RequireMTLS {
			if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 || hash(string(r.TLS.PeerCertificates[0].Raw)) != n.CertificateHash {
				fail(w, 401, "node client certificate required")
				return
			}
		}
		next(w, r.WithContext(context.WithValue(r.Context(), nodeKey, n)))
	}
}
func (s *Server) issue(tx *gorm.DB, u User, family string) (map[string]any, error) {
	a, b := secret(), secret()
	now := time.Now().UTC()
	if family == "" {
		family = ID()
	}
	for _, t := range []Token{{Hash: hash(a), UserID: u.ID, Kind: "access", Family: family, ExpiresAt: now.Add(15 * time.Minute)}, {Hash: hash(b), UserID: u.ID, Kind: "refresh", Family: family, ExpiresAt: now.Add(24 * time.Hour * 7)}} {
		if e := tx.Create(&t).Error; e != nil {
			return nil, e
		}
	}
	return map[string]any{"access_token": a, "refresh_token": b, "expires_in": 900, "user": u}, nil
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !s.limiter.Allow(ip) {
		fail(w, 429, "too many login attempts")
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	var u User
	if s.Store.DB.First(&u, "username = ?", strings.TrimSpace(in.Username)).Error != nil || !u.Enabled || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		fail(w, 401, "invalid credentials")
		return
	}
	var out map[string]any
	e := s.Store.Tx(func(tx *gorm.DB) error { var e error; out, e = s.issue(tx, u, ""); return e })
	respond(w, out, e)
}
func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Refresh string `json:"refresh_token"`
	}
	if !decode(w, r, &in) {
		return
	}
	var out map[string]any
	var rejected bool
	e := s.Store.Tx(func(tx *gorm.DB) error {
		var t Token
		if tx.First(&t, "hash = ? AND kind = ?", hash(in.Refresh), "refresh").Error != nil || time.Now().After(t.ExpiresAt) {
			rejected = true
			return nil
		}
		if t.Used {
			rejected = true
			return tx.Where("family = ?", t.Family).Delete(&Token{}).Error
		}
		var u User
		if tx.First(&u, "id = ?", t.UserID).Error != nil || !u.Enabled {
			rejected = true
			return nil
		}
		t.Used = true
		if e := tx.Save(&t).Error; e != nil {
			return e
		}
		var err error
		out, err = s.issue(tx, u, t.Family)
		return err
	})
	if rejected {
		fail(w, 401, "refresh token expired or reused")
		return
	}
	respond(w, out, e)
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	var t Token
	s.Store.DB.First(&t, "hash = ?", hash(bearer(r)))
	respond(w, map[string]bool{"ok": true}, s.Store.DB.Where("family = ?", t.Family).Delete(&Token{}).Error)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		fail(w, 400, "invalid JSON request: "+e.Error())
		return false
	}
	var tail any
	if d.Decode(&tail) != io.EOF {
		fail(w, 400, "multiple JSON objects are not accepted")
		return false
	}
	return true
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	write(w, status, map[string]any{"error": msg})
}
func respond(w http.ResponseWriter, out any, e error) {
	if e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			fail(w, 404, "resource not found")
		} else {
			fail(w, 400, e.Error())
		}
		return
	}
	write(w, 200, out)
}

type limitEntry struct {
	Start time.Time
	Count int
}
type limiter struct {
	sync.Mutex
	Entries map[string]limitEntry
}

func (l *limiter) Allow(k string) bool {
	l.Lock()
	defer l.Unlock()
	now := time.Now()
	if len(l.Entries) > 10000 {
		for key, e := range l.Entries {
			if now.Sub(e.Start) > time.Minute {
				delete(l.Entries, key)
			}
		}
	}
	e := l.Entries[k]
	if now.Sub(e.Start) > time.Minute {
		e = limitEntry{Start: now}
	}
	e.Count++
	l.Entries[k] = e
	return e.Count <= 15
}
