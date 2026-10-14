package control

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"gorm.io/gorm"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"
)

func completeJob(t *testing.T, s *Server, kind string, success bool) {
	t.Helper()
	w := request(s, "node-test-token", "GET", "/api/node/v1/jobs", nil, "")
	var jobs []c.Job
	if json.Unmarshal(w.Body.Bytes(), &jobs) != nil || len(jobs) != 1 || jobs[0].Kind != kind {
		t.Fatalf("expected %s job: %s", kind, w.Body.String())
	}
	body := map[string]any{"claim": jobs[0].Claim, "success": success, "probe": c.ProbeResult{TCP: true, UDP: true, Samples: 3, ExitIP: "203.0.113.2", RTTMS: 20}}
	if !success {
		body["error"] = "runtime did not stop"
	}
	if w = request(s, "node-test-token", "POST", "/api/node/v1/jobs/"+jobs[0].ID+"/result", body, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
}

func TestFailedRevokeKeepsPublishedVersion(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	var line Line
	s.Store.DB.First(&line, "id = ?", "line-1")
	line.EndpointIDs = []string{"ep-1"}
	s.Store.DB.Save(&line)
	s.Store.DB.Model(&Endpoint{}).Where("id = ?", "ep-1").Update("cipher", "2022-blake3-aes-128-gcm")
	w := request(s, token, "POST", "/api/admin/v1/lines/line-1/publish", nil, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var version LineVersion
	json.Unmarshal(w.Body.Bytes(), &version)
	completeJob(t, s, "prepare", true)
	completeJob(t, s, "probe", true)
	completeJob(t, s, "revoke", false)
	s.Store.DB.First(&version, "id = ?", version.ID)
	s.Store.DB.First(&line, "id = ?", "line-1")
	if version.Status != "ready" || line.PublishedVersion != version.ID {
		t.Fatalf("failed cleanup unpublished a verified version: status=%s published=%s", version.Status, line.PublishedVersion)
	}
}

func TestNodeHeartbeatKeepsAdministratorState(t *testing.T) {
	s, _, u := setup(t)
	fixture(t, s, u)
	s.Store.DB.Model(&Node{}).Where("id = ?", "node-1").Update("capabilities", `{"tcp":true,"udp":true,"ipv4":true,"ipv6":true,"managed":true}`)
	body := map[string]any{"version": "test", "capabilities": c.Capabilities{TCP: true, UDP: true, IPv4: true, Managed: true}}
	if w := request(s, "node-test-token", "POST", "/api/node/v1/heartbeat", body, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var n Node
	s.Store.DB.First(&n, "id = ?", "node-1")
	if !n.Capabilities.IPv6 || n.Reported == nil || n.Reported.IPv6 || n.effective().IPv6 || !n.effective().Managed {
		t.Fatalf("heartbeat rewrote declared capabilities: declared=%+v reported=%+v", n.Capabilities, n.Reported)
	}
	// An administrator disables the node while a heartbeat that already
	// authenticated is waiting for the lock.
	stale := n
	s.Store.DB.Model(&Node{}).Where("id = ?", "node-1").Update("enabled", false)
	r := httptest.NewRequest("POST", "/api/node/v1/heartbeat", strings.NewReader(`{"version":"test","capabilities":{"tcp":true}}`))
	r = r.WithContext(context.WithValue(r.Context(), nodeKey, stale))
	w := httptest.NewRecorder()
	s.nodeHeartbeat(w, r)
	s.Store.DB.First(&n, "id = ?", "node-1")
	if w.Code != 401 || n.Enabled {
		t.Fatalf("stale heartbeat re-enabled the node: code=%d enabled=%v", w.Code, n.Enabled)
	}
}

func TestOperatorStopsSessionsFromAdminRouteOnly(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	w := request(s, token, "POST", "/api/client/v1/sessions", CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}, "operator-test")
	var x Session
	json.Unmarshal(w.Body.Bytes(), &x)
	operator := User{ID: ID(), Username: "operator", Enabled: true, Role: "operator", MaxSessions: 1}
	s.Store.DB.Create(&operator)
	var auth map[string]any
	s.Store.Tx(func(tx *gorm.DB) error { var e error; auth, e = s.issue(tx, operator, ""); return e })
	op := auth["access_token"].(string)
	if w = request(s, op, "POST", "/api/client/v1/sessions/"+x.ID+"/stop", nil, ""); w.Code != 404 {
		t.Fatalf("client route let an operator stop another user's session: %d", w.Code)
	}
	if w = request(s, op, "POST", "/api/admin/v1/sessions/"+x.ID+"/stop", nil, ""); w.Code != 200 {
		t.Fatalf("operator could not stop a session from the admin route: %s", w.Body.String())
	}
}

func TestRelayGetsNoSessionKeyAndLeaseCoversPreparation(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	managed := c.Capabilities{TCP: true, UDP: true, IPv4: true, Managed: true}
	s.Store.DB.Create(&LineVersion{ID: "version-2", LineID: "line-1", Number: 2, Status: "ready", Regions: []string{"kr"}, Mode: "managed", Hops: []c.Hop{
		{EndpointID: "ep-relay", NodeID: "node-1", Kind: "realm", Host: "relay.example", Port: 21000, PortEnd: 21005, Capabilities: managed},
		{EndpointID: "ep-1", NodeID: "node-1", Kind: "ss2022", Host: "node.example", Port: 20000, PortEnd: 20005, Cipher: "2022-blake3-aes-128-gcm", Capabilities: managed},
	}})
	s.Store.DB.Model(&Line{}).Where("id = ?", "line-1").Update("published_version", "version-2")
	w := request(s, token, "POST", "/api/client/v1/sessions", CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}, "relay-test")
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	var x Session
	json.Unmarshal(w.Body.Bytes(), &x)
	if x.LeaseUntil.Sub(x.CreatedAt) < s.Config.PrepareTimeout {
		t.Fatal("initial lease expires before preparation can time out")
	}
	var jobs []Job
	s.Store.DB.Where("session_id = ? AND kind = ?", x.ID, "prepare").Find(&jobs)
	if len(jobs) != 2 {
		t.Fatalf("expected a prepare job per hop, got %d", len(jobs))
	}
	for _, j := range jobs {
		var p c.JobPayload
		if e := s.Store.Unseal(j.PayloadCipher, &p); e != nil {
			t.Fatal(e)
		}
		if p.Runtime.Kind == "realm" && (p.Runtime.Password != "" || p.Runtime.Cipher != "") {
			t.Fatal("relay runtime received the end-to-end session key")
		}
		if p.Runtime.Kind == "ss2022" && p.Runtime.Password == "" {
			t.Fatal("exit runtime lost its session key")
		}
	}
}

func TestRuntimeFailureStopsSessionAndLateEventsAreIgnored(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	w := request(s, token, "POST", "/api/client/v1/sessions", CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}, "failure-test")
	var x Session
	json.Unmarshal(w.Body.Bytes(), &x)
	events := map[string]any{"events": []map[string]string{{"session_id": x.ID, "kind": "runtime_failed"}, {"session_id": "unknown-session", "kind": "runtime_failed"}}}
	if w = request(s, "node-test-token", "POST", "/api/node/v1/session-events", events, ""); w.Code != 200 {
		t.Fatalf("runtime failure rejected: %s", w.Body.String())
	}
	s.Store.DB.First(&x, "id = ?", x.ID)
	if x.State != "STOPPING" || x.EndReason != "runtime_failed" {
		t.Fatalf("runtime failure did not stop the session: %s %s", x.State, x.EndReason)
	}
}

func TestLoginThrottlesFailuresPerAccount(t *testing.T) {
	s, _, _ := setup(t)
	bad := map[string]string{"username": "admin", "password": "wrong-password-0000"}
	for i := 0; i < loginFailures; i++ {
		if w := request(s, "", "POST", "/api/client/v1/auth/login", bad, ""); w.Code != 401 {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
	good := map[string]string{"username": "admin", "password": "test-password-123456"}
	if w := request(s, "", "POST", "/api/client/v1/auth/login", good, ""); w.Code != 429 {
		t.Fatalf("account not throttled after repeated failures: %d", w.Code)
	}
	s.limiter.Reset("user:admin")
	if w := request(s, "", "POST", "/api/client/v1/auth/login", good, ""); w.Code != 200 {
		t.Fatalf("valid login refused: %d", w.Code)
	}
	if w := request(s, "", "POST", "/api/client/v1/auth/login", map[string]string{"username": "nobody", "password": "x"}, ""); w.Code != 401 {
		t.Fatalf("unknown account: %d", w.Code)
	}
}

func TestTrustedProxyNamesClientForThrottling(t *testing.T) {
	s, _, _ := setup(t)
	s.Config.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("10.0.0.5/32")}
	r := httptest.NewRequest("POST", "/", nil)
	r.RemoteAddr = "10.0.0.5:4000"
	r.Header.Set("X-Real-IP", "198.51.100.9")
	if got := s.clientAddress(r); got != "198.51.100.9" {
		t.Fatalf("trusted proxy ignored: %s", got)
	}
	r.RemoteAddr = "203.0.113.4:4000"
	if got := s.clientAddress(r); got != "203.0.113.4" {
		t.Fatalf("untrusted X-Real-IP honoured: %s", got)
	}
}

func TestInternalErrorsAreNotLeaked(t *testing.T) {
	w := httptest.NewRecorder()
	respond(w, nil, errors.New("Error 1062 (23000): Duplicate entry 'x' for key 'users.username'"))
	if w.Code != 500 || strings.Contains(w.Body.String(), "Duplicate") {
		t.Fatalf("driver error leaked: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	respond(w, nil, badRequest("line capacity exhausted"))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "line capacity exhausted") {
		t.Fatalf("validation error hidden: %d %s", w.Code, w.Body.String())
	}
}

func TestDuplicateUsernameIsAValidationError(t *testing.T) {
	s, token, _ := setup(t)
	body := map[string]any{"username": "admin", "password": "another-password-1234", "role": "viewer", "enabled": true, "max_sessions": 1}
	if w := request(s, token, "POST", "/api/admin/v1/users", body, ""); w.Code != 400 || !strings.Contains(w.Body.String(), "already exists") {
		t.Fatalf("duplicate username: %d %s", w.Code, w.Body.String())
	}
}

func TestLatestReleaseIsHighestVersionAndEditsKeepAge(t *testing.T) {
	s, token, _ := setup(t)
	release := func(method, path, version string) Release {
		body := map[string]any{"version": version, "channel": "stable", "url": "https://example.com/" + version, "sha256": strings.Repeat("a", 64), "signature": "sig", "published": true}
		w := request(s, token, method, path, body, "")
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		var out Release
		json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	old := release("POST", "/api/admin/v1/releases", "1.9.0")
	release("POST", "/api/admin/v1/releases", "1.10.0")
	var before Release
	s.Store.DB.First(&before, "id = ?", old.ID)
	time.Sleep(10 * time.Millisecond)
	release("PATCH", "/api/admin/v1/releases/"+old.ID, "1.9.0")
	var after Release
	s.Store.DB.First(&after, "id = ?", old.ID)
	if !after.CreatedAt.Equal(before.CreatedAt) {
		t.Fatal("editing a release reset its creation time")
	}
	w := request(s, token, "GET", "/api/client/v1/releases/latest", nil, "")
	var latest Release
	json.Unmarshal(w.Body.Bytes(), &latest)
	if latest.Version != "1.10.0" {
		t.Fatalf("latest release is %q", latest.Version)
	}
}

func TestAdminListsPage(t *testing.T) {
	s, token, u := setup(t)
	for i := 0; i < 5; i++ {
		s.Store.DB.Create(&Audit{ID: ID(), Actor: u.ID, Action: "test", CreatedAt: time.Now().UTC()})
	}
	seen := map[string]bool{}
	for offset := 0; offset < 10; offset += 2 {
		w := request(s, token, "GET", "/api/admin/v1/audits?limit=2&offset="+strconv.Itoa(offset), nil, "")
		var rows []Audit
		json.Unmarshal(w.Body.Bytes(), &rows)
		for _, row := range rows {
			if seen[row.ID] {
				t.Fatal("page repeated a row")
			}
			seen[row.ID] = true
		}
		if len(rows) < 2 {
			break
		}
	}
	var total int64
	s.Store.DB.Model(&Audit{}).Count(&total)
	if int64(len(seen)) != total {
		t.Fatalf("paging returned %d of %d rows", len(seen), total)
	}
}

func TestMetricsScrapeToken(t *testing.T) {
	s, _, _ := setup(t)
	s.Config.MetricsToken = "scrape-token-for-tests"
	if w := request(s, "scrape-token-for-tests", "GET", "/metrics", nil, ""); w.Code != 200 || !strings.Contains(w.Body.String(), "flowlight_sessions") {
		t.Fatalf("scrape token rejected: %d", w.Code)
	}
	if w := request(s, "wrong-token", "GET", "/metrics", nil, ""); w.Code != 401 {
		t.Fatalf("wrong scrape token accepted: %d", w.Code)
	}
}

func TestExternalEgressMustBePublicAndBounded(t *testing.T) {
	s, token, _ := setup(t)
	s.Config.AllowImported = true
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 16))
	register := func(host string) int {
		body := map[string]any{"name": "mine", "host": host, "port": 8388, "cipher": "2022-blake3-aes-128-gcm", "password": key}
		return request(s, token, "POST", "/api/client/v1/egress-bindings", body, "").Code
	}
	for _, host := range []string{"127.0.0.1", "10.1.2.3", "192.168.1.1", "169.254.169.254", "localhost", "metadata.internal", "100.64.0.1"} {
		if code := register(host); code != 400 {
			t.Fatalf("internal address %s accepted: %d", host, code)
		}
	}
	for i := 0; i < maxOwnedEgress; i++ {
		if code := register("203.0.113." + strconv.Itoa(i+1)); code != 200 {
			t.Fatalf("public egress refused: %d", code)
		}
	}
	if code := register("198.51.100.1"); code != 400 {
		t.Fatalf("egress limit not enforced: %d", code)
	}
}

func TestPurgeRemovesSpentRows(t *testing.T) {
	s, _, u := setup(t)
	old := time.Now().UTC().Add(-60 * 24 * time.Hour)
	s.Store.DB.Create(&Token{Hash: "expired", UserID: u.ID, Kind: "access", ExpiresAt: old})
	s.Store.DB.Create(&Idempotency{ID: "old-key", SessionID: "x", CreatedAt: old})
	s.Store.DB.Create(&Job{ID: "old-job", Status: "succeeded", CreatedAt: old})
	s.Store.DB.Create(&Job{ID: "old-pending", Status: "pending", CreatedAt: old})
	if e := s.Purge(); e != nil {
		t.Fatal(e)
	}
	var n int64
	s.Store.DB.Model(&Token{}).Where("hash = ?", "expired").Count(&n)
	if n != 0 {
		t.Fatal("expired token kept")
	}
	s.Store.DB.Model(&Idempotency{}).Count(&n)
	if n != 0 {
		t.Fatal("old idempotency key kept")
	}
	s.Store.DB.Model(&Job{}).Where("id = ?", "old-job").Count(&n)
	if n != 0 {
		t.Fatal("finished job kept")
	}
	s.Store.DB.Model(&Job{}).Where("id = ?", "old-pending").Count(&n)
	if n != 1 {
		t.Fatal("unfinished job purged")
	}
}

func TestConfiguredProbeAgentValidatesManagedLines(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	s.Config.ProbeNodeID = "probe-node"
	w := request(s, token, "POST", "/api/client/v1/sessions", CreateSession{GameID: "valorant", RegionID: "kr", LineID: "line-1", DeviceID: "test-device"}, "probe-agent-test")
	var x Session
	json.Unmarshal(w.Body.Bytes(), &x)
	completeJob(t, s, "prepare", true)
	var probe Job
	if e := s.Store.DB.First(&probe, "session_id = ? AND kind = ?", x.ID, "probe").Error; e != nil {
		t.Fatal(e)
	}
	if probe.NodeID != "probe-node" {
		t.Fatalf("probe assigned to %s", probe.NodeID)
	}
}
