package control

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	mysqldriver "github.com/go-sql-driver/mysql"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func setup(t *testing.T) (*Server, string, User) {
	t.Helper()
	dir := t.TempDir()
	driver, dsn := "sqlite", filepath.Join(dir, "test.db")
	if source := os.Getenv("TEST_MYSQL_DSN"); source != "" {
		cfg, err := mysqldriver.ParseDSN(source)
		if err != nil {
			t.Fatal(err)
		}
		admin, err := sql.Open("mysql", source)
		if err != nil {
			t.Fatal(err)
		}
		schema := "flowlight_test_" + ID()
		if _, err = admin.Exec("CREATE DATABASE `" + schema + "`"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE `" + schema + "`"); admin.Close() })
		cfg.DBName = schema
		cfg.ParseTime = true
		driver, dsn = "mysql", cfg.FormatDSN()
	}
	store, e := Open(driver, dsn, base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db, _ := store.DB.DB(); _ = db.Close() })
	if e = store.Seed("admin", "test-password-123456"); e != nil {
		t.Fatal(e)
	}
	s, e := New(store, Config{CAPath: filepath.Join(dir, "pki"), ProbeURL: "https://probe.example/probe", ProbeUDP: "probe.example:9081"})
	if e != nil {
		t.Fatal(e)
	}
	var u User
	store.DB.First(&u, "username = ?", "admin")
	var auth map[string]any
	store.Tx(func(tx *gorm.DB) error { var e error; auth, e = s.issue(tx, u, ""); return e })
	return s, auth["access_token"].(string), u
}
func request(s *Server, token, method, path string, body any, key string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}
func fixture(t *testing.T, s *Server, u User) {
	t.Helper()
	now := time.Now().UTC()
	s.Store.DB.Create(&Device{ID: "test-device", UserID: u.ID, Name: "test", Capabilities: c.Capabilities{TCP: true, UDP: true, IPv4: true}})
	s.Store.DB.Model(&Game{}).Where("id = ?", "valorant").Updates(map[string]any{"regions": "[\"kr\"]", "published_version": 1})
	s.Store.DB.Create(&GameVersion{ID: "profile-1", GameID: "valorant", Version: 1, Profile: c.Profile{Processes: []string{"VALORANT-Win64-Shipping.exe"}, UDP: true}})
	s.Store.DB.Create(&Node{ID: "node-1", Name: "test", Address: "node.example", Enabled: true, Capabilities: c.Capabilities{TCP: true, UDP: true, IPv4: true, Managed: true}, LastSeen: now, TokenHash: hash("node-test-token")})
	s.Store.DB.Create(&Endpoint{ID: "ep-1", NodeID: "node-1", Name: "exit", Kind: "ss2022", Host: "node.example", Port: 20000, PortEnd: 20005, Enabled: true, Capabilities: c.Capabilities{TCP: true, UDP: true, IPv4: true, Managed: true}})
	s.Store.DB.Create(&Line{ID: "line-1", Name: "test line", Regions: []string{"kr"}, Capacity: 1, Mode: "managed", PublishedVersion: "version-1"})
	s.Store.DB.Create(&LineVersion{ID: "version-1", LineID: "line-1", Number: 1, Status: "ready", Regions: []string{"kr"}, Mode: "managed", Hops: []c.Hop{{EndpointID: "ep-1", NodeID: "node-1", Kind: "ss2022", Host: "node.example", Port: 20000, PortEnd: 20005, Cipher: "2022-blake3-aes-128-gcm", Capabilities: c.Capabilities{TCP: true, UDP: true, IPv4: true, Managed: true}}}})
}
func TestAuthorizationAndNoSecretsInCatalog(t *testing.T) {
	s, token, _ := setup(t)
	if w := request(s, "", "GET", "/api/admin/v1/users", nil, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := request(s, token, "GET", "/api/admin/v1/users", nil, "")
	if w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte("PasswordHash")) || bytes.Contains(w.Body.Bytes(), []byte("$2a$")) {
		t.Fatal(w.Body.String())
	}
	player := User{ID: ID(), Username: "viewer", Enabled: true, Role: "player", MaxSessions: 1}
	s.Store.DB.Create(&player)
	var auth map[string]any
	s.Store.Tx(func(tx *gorm.DB) error { var e error; auth, e = s.issue(tx, player, ""); return e })
	if w = request(s, auth["access_token"].(string), "GET", "/api/admin/v1/nodes", nil, ""); w.Code != 403 {
		t.Fatal(w.Code)
	}
}
func TestSessionIdempotencyCapacityAndStop(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	in := CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}
	w := request(s, token, "POST", "/api/client/v1/sessions", in, "request-one")
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	var x Session
	json.Unmarshal(w.Body.Bytes(), &x)
	again := request(s, token, "POST", "/api/client/v1/sessions", in, "request-one")
	var same Session
	json.Unmarshal(again.Body.Bytes(), &same)
	if same.ID != x.ID {
		t.Fatal("duplicate session")
	}
	in.RegionID = "jp"
	if got := request(s, token, "POST", "/api/client/v1/sessions", in, "request-one"); got.Code != 400 {
		t.Fatal("changed idempotency payload accepted")
	}
	in.RegionID = "kr"
	if got := request(s, token, "POST", "/api/client/v1/sessions", in, "another-request"); got.Code != 400 {
		t.Fatal("duplicate device accepted")
	}
	if got := request(s, token, "POST", "/api/client/v1/sessions/"+x.ID+"/heartbeat", map[string]any{"state": "ACTIVE"}, ""); got.Code != 400 {
		t.Fatal("unprepared session became active")
	}
	for i := 0; i < 2; i++ {
		if got := request(s, token, "POST", "/api/client/v1/sessions/"+x.ID+"/stop", nil, ""); got.Code != 200 {
			t.Fatal(got.Body.String())
		}
	}
	var count int64
	s.Store.DB.Model(&Job{}).Where("session_id = ? AND kind = ?", x.ID, "revoke").Count(&count)
	if count != 1 {
		t.Fatalf("duplicate revocation %d", count)
	}
	s.Store.DB.Model(&Session{}).Where("id = ?", x.ID).Update("stop_until", time.Now().Add(-time.Minute))
	if e := s.Reap(); e != nil {
		t.Fatal(e)
	}
	s.Store.DB.Model(&Reservation{}).Where("session_id = ?", x.ID).Count(&count)
	if count != 0 {
		t.Fatal("port leaked")
	}
}
func TestConcurrentStartReservesOnce(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	var wg sync.WaitGroup
	results := make(chan int, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- request(s, token, "POST", "/api/client/v1/sessions", CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}, "same-request").Code
		}()
	}
	wg.Wait()
	close(results)
	for code := range results {
		if code != 202 {
			t.Fatal(code)
		}
	}
	var count int64
	s.Store.DB.Model(&Session{}).Count(&count)
	if count != 1 {
		t.Fatal(count)
	}
	s.Store.DB.Model(&Reservation{}).Count(&count)
	if count != 1 {
		t.Fatal(count)
	}
}
func TestNodeClaimsScopedAndProbeRequired(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	w := request(s, token, "POST", "/api/client/v1/sessions", CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}, "node-test-request")
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	var x Session
	json.Unmarshal(w.Body.Bytes(), &x)
	w = request(s, "node-test-token", "GET", "/api/node/v1/jobs", nil, "")
	var jobs []c.Job
	if json.Unmarshal(w.Body.Bytes(), &jobs) != nil || len(jobs) != 1 {
		t.Fatal(w.Body.String())
	}
	j := jobs[0]
	if got := request(s, "node-test-token", "POST", "/api/node/v1/jobs/"+j.ID+"/result", map[string]any{"claim": "wrong", "success": true}, ""); got.Code != 400 {
		t.Fatal("stale claim accepted")
	}
	w = request(s, "node-test-token", "POST", "/api/node/v1/jobs/"+j.ID+"/result", map[string]any{"claim": j.Claim, "success": true}, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = request(s, "node-test-token", "GET", "/api/node/v1/jobs", nil, "")
	json.Unmarshal(w.Body.Bytes(), &jobs)
	if len(jobs) != 1 || jobs[0].Kind != "probe" {
		t.Fatal(w.Body.String())
	}
	j = jobs[0]
	w = request(s, "node-test-token", "POST", "/api/node/v1/jobs/"+j.ID+"/result", map[string]any{"claim": j.Claim, "success": true, "probe": c.ProbeResult{TCP: true, UDP: false, Samples: 3, ExitIP: "203.0.113.2"}}, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.Store.DB.First(&x, "id = ?", x.ID)
	if x.State != "STOPPING" {
		t.Fatalf("failed UDP probe did not stop: %s", x.State)
	}
}
func TestRefreshReplayRevokesFamily(t *testing.T) {
	s, _, u := setup(t)
	var auth map[string]any
	s.Store.Tx(func(tx *gorm.DB) error { var e error; auth, e = s.issue(tx, u, ""); return e })
	body := map[string]string{"refresh_token": auth["refresh_token"].(string)}
	w := request(s, "", "POST", "/api/client/v1/auth/refresh", body, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var next map[string]any
	json.Unmarshal(w.Body.Bytes(), &next)
	w = request(s, "", "POST", "/api/client/v1/auth/refresh", body, "")
	if w.Code != 401 {
		t.Fatal("refresh replay accepted")
	}
	if w = request(s, next["access_token"].(string), "GET", "/api/client/v1/me", nil, ""); w.Code != 401 {
		t.Fatal("replayed family still valid")
	}
}
func TestCredentialsEncryptedAndTrailingJSONRejected(t *testing.T) {
	s, _, _ := setup(t)
	cipher, e := s.Store.Seal("sensitive-key")
	if e != nil || bytes.Contains([]byte(cipher), []byte("sensitive-key")) {
		t.Fatal("encryption failed")
	}
	var plain string
	if e = s.Store.Unseal(cipher, &plain); e != nil || plain != "sensitive-key" {
		t.Fatal(e)
	}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"a":1} garbage`))
	var data map[string]int
	w := httptest.NewRecorder()
	if decode(w, r, &data) {
		t.Fatal("trailing garbage accepted")
	}
}

func TestPartialPatchAndPrivateEgressAccess(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	w := request(s, token, "PATCH", "/api/admin/v1/games/valorant", map[string]any{"regions": []string{"kr", "jp"}}, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var g Game
	json.Unmarshal(w.Body.Bytes(), &g)
	if g.Name != "VALORANT" || len(g.Draft.Processes) != 1 || len(g.Regions) != 2 {
		t.Fatal("partial patch lost fields")
	}
	s.Store.DB.Model(&Endpoint{}).Where("id = ?", "ep-1").Update("owner_id", "another-user")
	w = request(s, token, "POST", "/api/client/v1/sessions", CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}, "private-attempt")
	if w.Code != 400 {
		t.Fatal("another user's egress accepted")
	}
	var count int64
	s.Store.DB.Model(&Reservation{}).Count(&count)
	if count != 0 {
		t.Fatal("denied request reserved a port")
	}
}
func TestProbeSuccessPromotesSessionAndRevocationClosesIt(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	w := request(s, token, "POST", "/api/client/v1/sessions", CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}, "success-attempt")
	var x Session
	json.Unmarshal(w.Body.Bytes(), &x)
	for _, kind := range []string{"prepare", "probe"} {
		w = request(s, "node-test-token", "GET", "/api/node/v1/jobs", nil, "")
		var jobs []c.Job
		json.Unmarshal(w.Body.Bytes(), &jobs)
		if len(jobs) != 1 || jobs[0].Kind != kind {
			t.Fatal(w.Body.String())
		}
		j := jobs[0]
		w = request(s, "node-test-token", "POST", "/api/node/v1/jobs/"+j.ID+"/result", map[string]any{"claim": j.Claim, "success": true, "probe": c.ProbeResult{TCP: true, UDP: true, Samples: 3, ExitIP: "203.0.113.2", RTTMS: 20}}, "")
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	s.Store.DB.First(&x, "id = ?", x.ID)
	if x.State != "CONNECTING" {
		t.Fatal(x.State)
	}
	w = request(s, token, "POST", "/api/client/v1/sessions/"+x.ID+"/heartbeat", map[string]any{"state": "ACTIVE", "probe": c.ProbeResult{TCP: true, UDP: true, Samples: 3}}, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	request(s, token, "POST", "/api/client/v1/sessions/"+x.ID+"/stop", nil, "")
	w = request(s, "node-test-token", "GET", "/api/node/v1/jobs", nil, "")
	var jobs []c.Job
	json.Unmarshal(w.Body.Bytes(), &jobs)
	if len(jobs) != 1 || jobs[0].Kind != "revoke" {
		t.Fatal(w.Body.String())
	}
	j := jobs[0]
	w = request(s, "node-test-token", "POST", "/api/node/v1/jobs/"+j.ID+"/result", map[string]any{"claim": j.Claim, "success": true}, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.Store.DB.First(&x, "id = ?", x.ID)
	if x.State != "CLOSED" {
		t.Fatal(x.State)
	}
}
