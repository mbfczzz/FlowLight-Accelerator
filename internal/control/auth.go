package control

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
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
			fail(w, 401, "登录已过期，请重新登录")
			return
		}
		var u User
		if s.Store.DB.First(&u, "id = ?", token.UserID).Error != nil || !u.Enabled {
			fail(w, 401, "账号已被停用或不存在")
			return
		}
		if len(roles) > 0 {
			ok := u.Role == "admin"
			for _, role := range roles {
				ok = ok || role == u.Role
			}
			if !ok {
				fail(w, 403, "当前账号没有权限执行此操作")
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
			if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
				fail(w, 401, "node client certificate required")
				return
			}
			fingerprint := hash(string(r.TLS.PeerCertificates[0].Raw))
			if fingerprint != n.CertificateHash {
				if n.PendingCertificateHash == "" || fingerprint != n.PendingCertificateHash {
					fail(w, 401, "node client certificate required")
					return
				}
				// First use of a renewed certificate retires the previous one.
				if e := s.Store.Tx(func(tx *gorm.DB) error {
					return tx.Model(&Node{}).Where("id = ? AND pending_certificate_hash = ?", n.ID, fingerprint).Updates(map[string]any{"certificate_hash": fingerprint, "pending_certificate_hash": ""}).Error
				}); e != nil {
					fail(w, 503, "node identity temporarily unavailable")
					return
				}
				n.CertificateHash, n.PendingCertificateHash = fingerprint, ""
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

// Login throttling: each client address gets loginPerAddress attempts a
// minute, and each username loginFailures failed attempts per loginWindow.
// The username limit stops distributed guessing; it does not grow with the
// number of attacker addresses.
const (
	loginPerAddress = 30
	loginFailures   = 10
	loginWindow     = 15 * time.Minute
)

// dummyHash keeps unknown usernames as slow as wrong passwords, so response
// time does not reveal which accounts exist.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("flowlight-unknown-account"), 12)

// clientAddress is the TCP peer, or the X-Real-IP set by a reverse proxy the
// operator listed in TRUSTED_PROXIES. Without that, every login through the
// admin proxy would share one throttling bucket.
func (s *Server) clientAddress(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	peer, e := netip.ParseAddr(host)
	if e != nil {
		return host
	}
	for _, p := range s.Config.TrustedProxies {
		if p.Contains(peer.Unmap()) {
			if real, e := netip.ParseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); e == nil {
				return real.Unmap().String()
			}
			break
		}
	}
	return host
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.Allow("addr:"+s.clientAddress(r), loginPerAddress, time.Minute) {
		fail(w, 429, "登录尝试过于频繁，请一分钟后再试")
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	name := strings.TrimSpace(in.Username)
	account := "user:" + strings.ToLower(name)
	if s.limiter.Count(account, loginWindow) >= loginFailures {
		fail(w, 429, "该账号密码错误次数过多，请 15 分钟后再试")
		return
	}
	var u User
	found := s.Store.DB.First(&u, "username = ?", name).Error == nil
	hashed := []byte(u.PasswordHash)
	if !found {
		hashed = dummyHash
	}
	if bcrypt.CompareHashAndPassword(hashed, []byte(in.Password)) != nil || !found || !u.Enabled {
		s.limiter.Allow(account, loginFailures, loginWindow)
		fail(w, 401, "账号或密码错误")
		return
	}
	s.limiter.Reset(account)
	// The console is for staff. A player who signs in there gets no tokens:
	// the console would turn them away anyway, and tokens it never uses
	// would stay valid for their whole lifetime.
	if u.Role == "player" && strings.HasPrefix(r.URL.Path, "/api/admin/") {
		fail(w, 403, "此账号没有管理后台权限")
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
		fail(w, 401, "登录已过期，请重新登录")
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
		fail(w, 400, "请求格式错误，请刷新页面后重试")
		return false
	}
	var tail any
	if d.Decode(&tail) != io.EOF {
		fail(w, 400, "请求格式错误，请刷新页面后重试")
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

// respond maps handler outcomes to status codes. Validation and state
// conflicts are 400, a missing row is 404, and anything else is an internal
// failure: database and driver text must not reach clients.
func respond(w http.ResponseWriter, out any, e error) {
	if e != nil {
		switch {
		case errors.Is(e, gorm.ErrRecordNotFound):
			fail(w, 404, "要操作的内容不存在，可能已被删除，请刷新后重试")
		case isInvalid(e):
			fail(w, 400, e.Error())
		case isRetryable(e):
			code, _ := retryCodeOf(e)
			write(w, http.StatusConflict, map[string]any{"error": e.Error(), "code": code})
		default:
			log.Printf("request failed: %v", e)
			fail(w, 500, "控制服务内部错误，请稍后重试；如持续出现，请查看控制服务日志")
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

// Allow records one event for k in a fixed window and reports whether the
// count is still within limit.
func (l *limiter) Allow(k string, limit int, window time.Duration) bool {
	l.Lock()
	defer l.Unlock()
	now := time.Now()
	if len(l.Entries) > 10000 {
		for key, e := range l.Entries {
			if now.Sub(e.Start) > loginWindow {
				delete(l.Entries, key)
			}
		}
	}
	e := l.Entries[k]
	if now.Sub(e.Start) > window {
		e = limitEntry{Start: now}
	}
	e.Count++
	l.Entries[k] = e
	return e.Count <= limit
}

// Count returns the events recorded for k in the current window.
func (l *limiter) Count(k string, window time.Duration) int {
	l.Lock()
	defer l.Unlock()
	e := l.Entries[k]
	if time.Since(e.Start) > window {
		return 0
	}
	return e.Count
}
func (l *limiter) Reset(k string) {
	l.Lock()
	defer l.Unlock()
	delete(l.Entries, k)
}
