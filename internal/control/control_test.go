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
	"strings"
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
	s.Store.DB.Create(&Node{ID: "node-1", Name: "test", Address: "node.example", Enabled: true, Capabilities: c.Capabilities{TCP: true, UDP: true, IPv4: true, Managed: true}, LastSeen: timePointer(now), TokenHash: hash("node-test-token")})
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

// A device holds one session at a time. A session that is actually running
// is a plain refusal; one that is stopping, or one whose lease already ran
// out (the client that held it was killed, as an update does), clears by
// itself, so it is refused as retryable and the same request goes through
// once it is gone.
func TestSessionLeavingADeviceIsRetryable(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	in := CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}
	start := func(key string) *httptest.ResponseRecorder {
		t.Helper()
		return request(s, token, "POST", "/api/client/v1/sessions", in, key)
	}
	refusal := func(w *httptest.ResponseRecorder, wantCode string) {
		t.Helper()
		if w.Code != 409 {
			t.Fatalf("refusal was not marked retryable: %d %s", w.Code, w.Body.String())
		}
		var body struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Code != wantCode || body.Error == "" {
			t.Fatalf("retryable refusal carried the wrong body: %s", w.Body.String())
		}
	}
	// Finishes stopping the session, as its nodes confirming the revoke and
	// the reaper would.
	finish := func() {
		t.Helper()
		var x Session
		if e := s.Store.DB.First(&x, "device_id = ? AND state != ?", "test-device", "CLOSED").Error; e != nil {
			t.Fatal(e)
		}
		s.Store.DB.Model(&Job{}).Where("session_id = ? AND kind = ?", x.ID, "revoke").Update("status", "succeeded")
		s.Store.DB.Model(&Session{}).Where("id = ?", x.ID).Update("stop_until", time.Now().Add(-time.Minute))
		if e := s.Reap(); e != nil {
			t.Fatal(e)
		}
		var left int64
		s.Store.DB.Model(&Session{}).Where("device_id = ? AND state != ?", "test-device", "CLOSED").Count(&left)
		if left != 0 {
			t.Fatalf("session %s was not closed", x.ID)
		}
	}

	w := start("first-attempt")
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	if got := start("while-running"); got.Code != 400 ||
		!strings.Contains(got.Body.String(), "此设备已有一个加速会话在运行") {
		t.Fatalf("a running session was not refused plainly: %d %s", got.Code, got.Body.String())
	}

	// Stopping: the player asked for it, and the device is free again as
	// soon as the teardown finishes.
	var x Session
	s.Store.DB.First(&x, "device_id = ? AND state != ?", "test-device", "CLOSED")
	if got := request(s, token, "POST", "/api/client/v1/sessions/"+x.ID+"/stop", nil, ""); got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	refusal(start("while-closing"), "session-closing")
	finish()
	if got := start("after-closing"); got.Code != 202 {
		t.Fatalf("the device stayed blocked after the session closed: %d %s", got.Code, got.Body.String())
	}

	// Left behind by a client that was killed mid-session.
	s.Store.DB.Model(&Session{}).Where("device_id = ? AND state != ?", "test-device", "CLOSED").
		Updates(map[string]any{"state": "ACTIVE", "lease_until": time.Now().Add(-time.Minute)})
	refusal(start("while-stale"), "session-stale")
	if e := s.Reap(); e != nil {
		t.Fatal(e)
	}
	finish()
	if got := start("after-stale"); got.Code != 202 {
		t.Fatalf("the device stayed blocked after the leftover was cleared: %d %s", got.Code, got.Body.String())
	}

	// A live session behind an older one that is still closing: waiting
	// would never clear it, so the player is told plainly. Only the oldest
	// row used to be looked at, which sent the client into a wait that
	// could only time out.
	var live Session
	s.Store.DB.First(&live, "device_id = ? AND state != ?", "test-device", "CLOSED")
	older := live
	older.ID = "older-" + live.ID
	older.State = "STOPPING"
	older.CreatedAt = live.CreatedAt.Add(-time.Minute)
	if e := s.Store.DB.Create(&older).Error; e != nil {
		t.Fatal(e)
	}
	if got := start("behind-closing"); got.Code != 400 ||
		!strings.Contains(got.Body.String(), "此设备已有一个加速会话在运行") {
		t.Fatalf("a live session behind a closing one was answered with a wait: %d %s", got.Code, got.Body.String())
	}
}

// A retry after a retryable refusal must create a NEW session, not hand back
// the one whose idempotency key the first, successful attempt recorded. The
// client waits out "the previous session is still closing" and comes back
// with a fresh key; if the service answered with the old, closed session, the
// client would poll something that is already gone.
func TestRetryAfterRefusalCreatesANewSession(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	in := CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}

	var first Session
	json.Unmarshal(request(s, token, "POST", "/api/client/v1/sessions", in, "attempt-1").Body.Bytes(), &first)

	// The player stops it: it is now stopping, so a start is refused as
	// retryable.
	if got := request(s, token, "POST", "/api/client/v1/sessions/"+first.ID+"/stop", nil, ""); got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	if got := request(s, token, "POST", "/api/client/v1/sessions", in, "attempt-2"); got.Code != 409 {
		t.Fatalf("a closing session was not refused as retryable: %d %s", got.Code, got.Body.String())
	}
	// Nothing was created, so no key was recorded for that refusal.
	var rows int64
	s.Store.DB.Model(&Idempotency{}).Count(&rows)
	if rows != 1 {
		t.Fatalf("a refused attempt recorded an idempotency row: %d rows", rows)
	}

	// Let it finish closing, then retry with a fresh key, as the service does.
	s.Store.DB.Model(&Job{}).Where("session_id = ? AND kind = ?", first.ID, "revoke").Update("status", "succeeded")
	s.Store.DB.Model(&Session{}).Where("id = ?", first.ID).Update("stop_until", time.Now().Add(-time.Minute))
	if e := s.Reap(); e != nil {
		t.Fatal(e)
	}
	w := request(s, token, "POST", "/api/client/v1/sessions", in, "attempt-3")
	if w.Code != 202 {
		t.Fatalf("the retry did not create a session: %d %s", w.Code, w.Body.String())
	}
	var second Session
	json.Unmarshal(w.Body.Bytes(), &second)
	if second.ID == first.ID {
		t.Fatalf("the retry was handed the old session %s", first.ID)
	}
	if second.State == "CLOSED" || second.State == "STOPPING" {
		t.Fatalf("the retry created a session that is already ending: %s", second.State)
	}

	// And the SAME key still resolves to the session it made: the retry's own
	// replay must not create a third one.
	var again Session
	json.Unmarshal(request(s, token, "POST", "/api/client/v1/sessions", in, "attempt-3").Body.Bytes(), &again)
	if again.ID != second.ID {
		t.Fatalf("replaying the retry's key created another session: %s vs %s", again.ID, second.ID)
	}
}

// Two published rows for one version would both look live, and which one a
// client is offered comes down to creation order. Only one may be published.
func TestReleaseVersionIsUniquePerChannel(t *testing.T) {
	s, token, _ := setup(t)
	body := func(version, channel string, published bool) map[string]any {
		return map[string]any{
			"version":   version,
			"channel":   channel,
			"url":       "https://example.com/FlowLight.exe",
			"sha256":    strings.Repeat("a", 64),
			"published": published,
		}
	}
	post := func(version, channel string, published bool) *httptest.ResponseRecorder {
		t.Helper()
		return request(s, token, "POST", "/api/admin/v1/releases", body(version, channel, published), "")
	}
	if got := post("1.2.3", "stable", true); got.Code != 200 {
		t.Fatalf("first release refused: %d %s", got.Code, got.Body.String())
	}
	// A second published row for the same version is a refusal that says why.
	dup := post("1.2.3", "stable", true)
	if dup.Code != 400 || !strings.Contains(dup.Body.String(), "只能发布一条") {
		t.Fatalf("a duplicate published version was accepted: %d %s", dup.Code, dup.Body.String())
	}
	// The same version on the other channel is a different thing: a beta is
	// what beta-channel clients get.
	if got := post("1.2.3", "beta", true); got.Code != 200 {
		t.Fatalf("the same version on another channel was refused: %d %s", got.Code, got.Body.String())
	}
	// An unpublished duplicate is fine; it becomes a problem only when it is
	// published, which is the check above.
	if got := post("1.2.3", "stable", false); got.Code != 200 {
		t.Fatalf("an unpublished duplicate was refused: %d %s", got.Code, got.Body.String())
	}

	// The list marks the row a client is actually offered, one per channel.
	var rows []Release
	if w := request(s, token, "GET", "/api/admin/v1/releases", nil, ""); json.Unmarshal(w.Body.Bytes(), &rows) != nil {
		t.Fatal(w.Body.String())
	}
	served := map[string]int{}
	unpublished := 0
	for _, r := range rows {
		if r.Served {
			served[r.Channel]++
		}
		if r.Served && !r.Published {
			t.Errorf("an unpublished release was marked as served: %s", r.Version)
		}
		if !r.Published {
			unpublished++
		}
	}
	if served["stable"] != 1 || served["beta"] != 1 {
		t.Fatalf("the list marks the wrong number of served rows: %+v", served)
	}
	if unpublished != 1 {
		t.Fatalf("expected exactly one unpublished row, got %d", unpublished)
	}
}

// A STOPPING row with no deadline is never reaped, so it would hold its
// device and ports forever, and a client retrying "the previous session is
// still closing" would wait for something that never ends.
func TestStoppingWithoutADeadlineIsClosed(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	in := CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}
	if w := request(s, token, "POST", "/api/client/v1/sessions", in, "deadline-key-1"); w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	var x Session
	if e := s.Store.DB.First(&x, "device_id = ? AND state <> ?", "test-device", "CLOSED").Error; e != nil {
		t.Fatal(e)
	}
	if e := s.Store.DB.Model(&Session{}).Where("id = ?", x.ID).
		Updates(map[string]any{"state": "STOPPING", "stop_until": nil}).Error; e != nil {
		t.Fatal(e)
	}
	s.Store.DB.First(&x, "id = ?", x.ID)
	if x.StopUntil != nil {
		t.Skip("this driver did not store NULL")
	}

	if e := s.Reap(); e != nil {
		t.Fatal(e)
	}
	s.Store.DB.First(&x, "id = ?", x.ID)
	if x.State != "CLOSED" {
		t.Fatalf("a stopping row without a deadline stayed %s", x.State)
	}
	var ports int64
	s.Store.DB.Model(&Reservation{}).Where("session_id = ?", x.ID).Count(&ports)
	if ports != 0 {
		t.Fatalf("its ports were not released: %d", ports)
	}
	if got := request(s, token, "POST", "/api/client/v1/sessions", in, "deadline-key-2"); got.Code != 202 {
		t.Fatalf("the device stayed blocked: %d %s", got.Code, got.Body.String())
	}
}

// How long a stopped session blocks its device, which the Windows service's
// ClosingWait has to outlast. Changing the lease, the grace or the reaper's
// cadence moves this number, so the wait on the client side is derived from
// it rather than guessed.
func TestStoppedSessionBlocksForAtMostTheClientWait(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	in := CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}

	// The longest case: stopped while still preparing, so its lease is the
	// preparation timeout rather than the shorter running-lease figure.
	if w := request(s, token, "POST", "/api/client/v1/sessions", in, "window-key-1"); w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	var x Session
	if e := s.Store.DB.First(&x, "device_id = ? AND state <> ?", "test-device", "CLOSED").Error; e != nil {
		t.Fatal(e)
	}
	startedAt := time.Now()
	if got := request(s, token, "POST", "/api/client/v1/sessions/"+x.ID+"/stop", nil, ""); got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	s.Store.DB.First(&x, "id = ?", x.ID)
	if x.StopUntil == nil {
		t.Fatal("a stopped session got no deadline")
	}
	// The reaper only closes it after the deadline, and runs every 5s.
	worst := x.StopUntil.Sub(startedAt) + 5*time.Second
	if worst > 240*time.Second {
		t.Fatalf("a stopped session can hold its device for %v, longer than the service waits (240s)", worst)
	}
	t.Logf("a stopped session holds its device at most %v (client waits 240s)", worst.Round(time.Second))

	// And the device really is free once the deadline passes.
	s.Store.DB.Model(&Session{}).Where("id = ?", x.ID).Update("stop_until", time.Now().Add(-time.Second))
	s.Store.DB.Model(&Job{}).Where("session_id = ? AND kind = ?", x.ID, "revoke").Update("status", "succeeded")
	if e := s.Reap(); e != nil {
		t.Fatal(e)
	}
	if got := request(s, token, "POST", "/api/client/v1/sessions", in, "window-key-2"); got.Code != 202 {
		t.Fatalf("the device was still blocked after the deadline: %d %s", got.Code, got.Body.String())
	}
}

// A line whose capacity is larger than the ports its path can supply looks
// healthy and then fails for every player once the ports run out, with
// nothing on screen explaining why. Publishing such a line is refused, and
// the refusal names the endpoint to widen.
func TestLineCapacityCannotExceedItsPorts(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	// The fixture's exit endpoint serves 20000-20005, six ports, and the line
	// must point at it for the path to be compiled at all.
	var ep Endpoint
	if e := s.Store.DB.First(&ep, "id = ?", "ep-1").Error; e != nil {
		t.Fatal(e)
	}
	if have := ep.PortEnd - ep.Port + 1; have != 6 {
		t.Fatalf("fixture changed: the endpoint offers %d ports, the test assumes 6", have)
	}
	if e := s.Store.DB.Model(&Line{}).Where("id = ?", "line-1").
		Update("endpoint_ids", `["ep-1"]`).Error; e != nil {
		t.Fatal(e)
	}
	// The fixture leaves the cipher unset; compile requires one for an ss2022
	// landing, and the port check sits after it.
	if e := s.Store.DB.Model(&Endpoint{}).Where("id = ?", "ep-1").
		Update("cipher", "2022-blake3-aes-128-gcm").Error; e != nil {
		t.Fatal(e)
	}

	// Saving a draft is not validated: a line under construction may be
	// incomplete. What matters is that it cannot be published.
	save := func(capacity int) *httptest.ResponseRecorder {
		t.Helper()
		return request(s, token, "PATCH", "/api/admin/v1/lines/line-1", map[string]any{"capacity": capacity}, "")
	}
	if got := save(20); got.Code != 200 {
		t.Fatalf("saving a draft capacity failed: %d %s", got.Code, got.Body.String())
	}
	over := request(s, token, "POST", "/api/admin/v1/lines/line-1/publish", nil, "")
	if over.Code != 400 {
		t.Fatalf("a line whose capacity exceeds its ports was published: %d %s", over.Code, over.Body.String())
	}
	body := over.Body.String()
	if !strings.Contains(body, "端口") || !strings.Contains(body, "6") || !strings.Contains(body, "exit") {
		t.Fatalf("the refusal does not name the port limit and the endpoint: %s", body)
	}

	// At the port count it publishes: the path can supply one port per session.
	if got := save(6); got.Code != 200 {
		t.Fatalf("saving a capacity the path can supply failed: %d %s", got.Code, got.Body.String())
	}
	if got := request(s, token, "POST", "/api/admin/v1/lines/line-1/publish", nil, ""); got.Code != 200 {
		t.Fatalf("a capacity the path can supply was refused at publish: %d %s", got.Code, got.Body.String())
	}
}

// A path through the same endpoint twice can never be published, so the
// draft is refused when it is saved rather than when publishing fails.
func TestLinePathCannotRepeatAnEndpoint(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	got := request(s, token, "PATCH", "/api/admin/v1/lines/line-1", map[string]any{"endpoint_ids": []string{"ep-1", "ep-1"}}, "")
	if got.Code != 400 || !strings.Contains(got.Body.String(), "重复的服务端点") {
		t.Fatalf("a path repeating an endpoint was saved: %d %s", got.Code, got.Body.String())
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

// The exit reports its latency echo with the prepare task that asked for
// it; the probe and the client are given it in the connection.
func TestExitEchoReachesTheConnection(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	w := request(s, token, "POST", "/api/client/v1/sessions", CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}, "echo-attempt")
	var x Session
	json.Unmarshal(w.Body.Bytes(), &x)
	claim := func(kind string) c.Job {
		t.Helper()
		w := request(s, "node-test-token", "GET", "/api/node/v1/jobs", nil, "")
		var jobs []c.Job
		json.Unmarshal(w.Body.Bytes(), &jobs)
		if len(jobs) != 1 || jobs[0].Kind != kind {
			t.Fatal(w.Body.String())
		}
		return jobs[0]
	}
	j := claim("prepare")
	if j.Payload.Runtime == nil || !j.Payload.Runtime.Echo {
		t.Fatal("the exit was not asked for its latency echo")
	}
	w = request(s, "node-test-token", "POST", "/api/node/v1/jobs/"+j.ID+"/result", map[string]any{"claim": j.Claim, "success": true, "echo": 41234}, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	j = claim("probe")
	if j.Payload.Connection == nil || j.Payload.Connection.ExitEcho != "127.0.0.1:41234" {
		t.Fatalf("the probe was not given the echo: %+v", j.Payload.Connection)
	}
	w = request(s, "node-test-token", "POST", "/api/node/v1/jobs/"+j.ID+"/result", map[string]any{"claim": j.Claim, "success": true, "probe": c.ProbeResult{TCP: true, UDP: true, Samples: 5, ExitIP: "203.0.113.2", RTTMS: 20}}, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = request(s, token, "GET", "/api/client/v1/sessions/"+x.ID, nil, "")
	var out struct {
		Connection c.Connection `json:"connection"`
	}
	if json.Unmarshal(w.Body.Bytes(), &out) != nil || out.Connection.ExitEcho != "127.0.0.1:41234" {
		t.Fatalf("the client was not given the echo: %s", w.Body.String())
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
