package control

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
)

// checkFixture is the fixture's line made two hops: a relay on node-2 in
// front of the exit on node-1, both run by the platform.
func checkFixture(t *testing.T) (*Server, string, *[]c.CheckTarget) {
	t.Helper()
	s, token, u := setup(t)
	fixture(t, s, u)
	now := time.Now().UTC()
	caps := c.Capabilities{TCP: true, UDP: true, IPv4: true, Managed: true}
	for _, row := range []any{
		&Node{ID: "node-2", Name: "relay", Address: "relay.example", Enabled: true, Capabilities: caps, LastSeen: timePointer(now), AgentVersion: "0.3.0", TokenHash: hash("relay-token")},
		&Endpoint{ID: "ep-0", NodeID: "node-2", Name: "entry", Kind: "realm", Host: "relay.example", Port: 21000, PortEnd: 21003, Enabled: true, Capabilities: caps},
		&LineVersion{ID: "version-2", LineID: "line-1", Number: 2, Status: "ready", Hops: []c.Hop{
			{EndpointID: "ep-0", NodeID: "node-2", Kind: "realm", Host: "relay.example", Port: 21000, PortEnd: 21003, Capabilities: caps},
			{EndpointID: "ep-1", NodeID: "node-1", Kind: "ss2022", Host: "node.example", Port: 20000, PortEnd: 20005, Cipher: "2022-blake3-aes-128-gcm", Capabilities: caps},
		}, CreatedAt: now},
	} {
		if e := s.Store.DB.Create(row).Error; e != nil {
			t.Fatal(e)
		}
	}
	s.Store.DB.Model(&Line{}).Where("id = ?", "line-1").Update("published_version", "version-2")
	var mu sync.Mutex
	dialled := &[]c.CheckTarget{}
	s.checkDialer = func(_ context.Context, target c.CheckTarget) c.CheckLinkResult {
		mu.Lock()
		*dialled = append(*dialled, target)
		mu.Unlock()
		return c.CheckLinkResult{ID: target.ID, TCP: &c.CheckMeasure{OK: true, RTTMS: 30, Samples: 4}, UDP: &c.CheckMeasure{OK: true, RTTMS: 31, Samples: 5}}
	}
	return s, token, dialled
}

func claim(t *testing.T, s *Server, nodeToken string) []c.Job {
	t.Helper()
	w := request(s, nodeToken, "GET", "/api/node/v1/jobs", nil, "")
	var jobs []c.Job
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &jobs) != nil {
		t.Fatalf("claim: %d %s", w.Code, w.Body.String())
	}
	return jobs
}

func answer(t *testing.T, s *Server, nodeToken string, j c.Job, ok bool, message string, report *c.CheckReport) {
	t.Helper()
	w := request(s, nodeToken, "POST", "/api/node/v1/jobs/"+j.ID+"/result", map[string]any{"claim": j.Claim, "success": ok, "error": message, "check": report}, "")
	if w.Code != 200 {
		t.Fatalf("result: %d %s", w.Code, w.Body.String())
	}
}

// latest waits for the check to satisfy done, which the control service's
// own dial reaches in the background.
func latest(t *testing.T, s *Server, token string, done func(LineCheck) bool) LineCheck {
	t.Helper()
	var k LineCheck
	for i := 0; i < 100; i++ {
		w := request(s, token, "GET", "/api/admin/v1/lines/line-1/check", nil, "")
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &k) != nil {
			t.Fatalf("check: %d %s", w.Code, w.Body.String())
		}
		if done(k) {
			return k
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("check never got there: %+v", k)
	return k
}

func link(k LineCheck, id string) checkLink {
	for _, l := range k.Links {
		if l.ID == id {
			return l
		}
	}
	return checkLink{}
}

func TestConnectivityCheckMeasuresEveryLink(t *testing.T) {
	s, token, dialled := checkFixture(t)
	w := request(s, token, "POST", "/api/admin/v1/lines/line-1/check", nil, "")
	var k LineCheck
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &k) != nil || k.Phase != "listen" || k.Version != 2 || len(k.Nodes) != 2 || len(k.Links) != 3 {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "token") {
		t.Fatal("listener tokens shown to the admin")
	}
	if again := request(s, token, "POST", "/api/admin/v1/lines/line-1/check", nil, ""); !strings.Contains(again.Body.String(), k.ID) {
		t.Fatal("a second check started while one runs")
	}

	// Each node opens a listener in its own endpoint's range.
	relay, exit := claim(t, s, "relay-token"), claim(t, s, "node-test-token")
	if len(relay) != 1 || relay[0].Kind != "check-listen" || len(exit) != 1 || exit[0].Kind != "check-listen" {
		t.Fatalf("listen jobs: %+v %+v", relay, exit)
	}
	for _, p := range relay[0].Payload.Check.Ports {
		if p < 21000 || p > 21003 {
			t.Fatalf("relay port %d outside its range", p)
		}
	}
	exitPort, exitToken := exit[0].Payload.Check.Ports[0], exit[0].Payload.Check.Token
	relayPort, relayToken := relay[0].Payload.Check.Ports[0], relay[0].Payload.Check.Token
	if len(exitToken) != 32 || exitToken == relayToken {
		t.Fatalf("listener tokens: %q %q", exitToken, relayToken)
	}
	answer(t, s, "relay-token", relay[0], true, "", &c.CheckReport{Port: relayPort})
	answer(t, s, "node-test-token", exit[0], true, "", &c.CheckReport{Port: exitPort})

	// The relay dials the exit's listener; the exit reaches the internet;
	// the control service dials the relay.
	relay, exit = claim(t, s, "relay-token"), claim(t, s, "node-test-token")
	if len(relay) != 1 || relay[0].Kind != "check-dial" || len(exit) != 1 || exit[0].Kind != "check-dial" {
		t.Fatalf("dial jobs: %+v %+v", relay, exit)
	}
	hop := relay[0].Payload.Check.Targets[0]
	if hop.ID != "link-0" || hop.Host != "node.example" || hop.Port != exitPort || hop.Token != exitToken || !hop.UDP {
		t.Fatalf("relay target: %+v", hop)
	}
	internet := exit[0].Payload.Check.Targets[0]
	if internet.ID != "internet" || internet.URL != "https://probe.example/probe" || internet.UDPTarget != "probe.example:9081" {
		t.Fatalf("internet target: %+v", internet)
	}
	answer(t, s, "relay-token", relay[0], true, "", &c.CheckReport{Links: []c.CheckLinkResult{{ID: "link-0", TCP: &c.CheckMeasure{OK: true, RTTMS: 12, Samples: 4}, UDP: &c.CheckMeasure{OK: true, RTTMS: 13, Samples: 5}}}})
	answer(t, s, "node-test-token", exit[0], true, "", &c.CheckReport{Links: []c.CheckLinkResult{{ID: "internet", TCP: &c.CheckMeasure{OK: true, RTTMS: 40, Samples: 3}, UDP: &c.CheckMeasure{OK: true, RTTMS: 41, Loss: 0.2, Samples: 5}, ExitIP: "203.0.113.10"}}})

	k = latest(t, s, token, func(k LineCheck) bool { return k.Phase == "done" })
	if link(k, "public").Status != "ok" || link(k, "link-0").Status != "ok" || link(k, "internet").Status != "warn" || link(k, "internet").ExitIP != "203.0.113.10" || k.FinishedAt == nil {
		t.Fatalf("results: %+v", k.Links)
	}
	if len(*dialled) != 1 || (*dialled)[0].Host != "relay.example" || (*dialled)[0].Port != relayPort || (*dialled)[0].Token != relayToken || !(*dialled)[0].UDP {
		t.Fatalf("control dial: %+v", *dialled)
	}

	// Done, each node is told to close its own test listener; an agent from
	// before 0.3.2 does not know the task, and its listener times out.
	if k.Nodes[0].Release != "pending" || k.Nodes[1].Release != "pending" {
		t.Fatalf("listeners not being released: %+v", k.Nodes)
	}
	relay, exit = claim(t, s, "relay-token"), claim(t, s, "node-test-token")
	if len(relay) != 1 || relay[0].Kind != "check-release" || relay[0].Payload.Check.Ports[0] != relayPort || relay[0].Payload.Check.Token != relayToken ||
		len(exit) != 1 || exit[0].Kind != "check-release" || exit[0].Payload.Check.Ports[0] != exitPort || exit[0].Payload.Check.Token != exitToken {
		t.Fatalf("release jobs: %+v %+v", relay, exit)
	}
	answer(t, s, "relay-token", relay[0], true, "", nil)
	answer(t, s, "node-test-token", exit[0], false, "unsupported structured task", nil)
	k = latest(t, s, token, func(k LineCheck) bool { return true })
	if k.Nodes[0].Release != "done" || k.Nodes[1].Release != "expired" {
		t.Fatalf("released: %+v", k.Nodes)
	}
}

func TestConnectivityCheckReleaseIsNotWaitedForever(t *testing.T) {
	s, token, _ := checkFixture(t)
	request(s, token, "POST", "/api/admin/v1/lines/line-1/check", nil, "")
	relay, exit := claim(t, s, "relay-token"), claim(t, s, "node-test-token")
	answer(t, s, "relay-token", relay[0], true, "", &c.CheckReport{Port: relay[0].Payload.Check.Ports[0]})
	answer(t, s, "node-test-token", exit[0], true, "", &c.CheckReport{Port: exit[0].Payload.Check.Ports[0]})
	s.Store.DB.Model(&LineCheck{}).Where("line_id = ?", "line-1").Update("phase_at", time.Now().UTC().Add(-time.Minute))
	k := latest(t, s, token, func(k LineCheck) bool { return k.Phase == "done" })
	if k.Nodes[0].Release != "pending" {
		t.Fatalf("not releasing: %+v", k.Nodes)
	}
	// Nobody confirms.
	s.Store.DB.Model(&LineCheck{}).Where("id = ?", k.ID).Update("finished_at", time.Now().UTC().Add(-time.Minute))
	k = latest(t, s, token, func(k LineCheck) bool { return k.Nodes[0].Release != "pending" })
	if k.Nodes[0].Release != "expired" || k.Nodes[1].Release != "expired" {
		t.Fatalf("release never gave up: %+v", k.Nodes)
	}
}

func TestConnectivityCheckNamesOldAgentsAndOfflineNodes(t *testing.T) {
	s, token, _ := checkFixture(t)
	s.Store.DB.Model(&Node{}).Where("id = ?", "node-1").Update("agent_version", "0.2.0")
	request(s, token, "POST", "/api/admin/v1/lines/line-1/check", nil, "")
	relay, exit := claim(t, s, "relay-token"), claim(t, s, "node-test-token")
	answer(t, s, "relay-token", relay[0], true, "", &c.CheckReport{Port: relay[0].Payload.Check.Ports[0]})
	answer(t, s, "node-test-token", exit[0], false, "unsupported structured task", nil)
	exit = claim(t, s, "node-test-token")
	answer(t, s, "node-test-token", exit[0], false, "unsupported structured task", nil)
	k := latest(t, s, token, func(k LineCheck) bool { return k.Phase == "done" })
	if !strings.Contains(k.Nodes[1].ListenError, "旧版本（0.2.0）") || !strings.Contains(link(k, "link-0").Note, "没有开启检测端口") || !strings.Contains(link(k, "internet").Note, "旧版本") || link(k, "public").Status != "ok" {
		t.Fatalf("old agent: %+v %+v", k.Nodes, k.Links)
	}
	// The relay was never asked to dial a listener that is not there, only
	// to close its own; the exit opened none.
	if jobs := claim(t, s, "relay-token"); len(jobs) != 1 || jobs[0].Kind != "check-release" {
		t.Fatalf("relay asked to dial: %+v", jobs)
	}
	if jobs := claim(t, s, "node-test-token"); len(jobs) != 0 {
		t.Fatalf("exit asked to close a listener it never opened: %+v", jobs)
	}

	s.Store.DB.Model(&Node{}).Where("id = ?", "node-2").Update("last_seen", time.Now().UTC().Add(-time.Hour))
	request(s, token, "POST", "/api/admin/v1/lines/line-1/check", nil, "")
	if jobs := claim(t, s, "relay-token"); len(jobs) != 0 {
		t.Fatal("an offline node was given work")
	}
	exit = claim(t, s, "node-test-token")
	answer(t, s, "node-test-token", exit[0], true, "", &c.CheckReport{Port: exit[0].Payload.Check.Ports[0]})
	k = latest(t, s, token, func(k LineCheck) bool { return k.Phase == "dial" })
	if !strings.Contains(k.Nodes[0].ListenError, "不在线") || link(k, "public").Status != "failed" || !strings.Contains(link(k, "link-0").Note, "无法从这里发起检测") {
		t.Fatalf("offline relay: %+v %+v", k.Nodes, k.Links)
	}
}

func TestConnectivityCheckStepsRunOutOfTime(t *testing.T) {
	s, token, _ := checkFixture(t)
	age := func() {
		t.Helper()
		if e := s.Store.DB.Model(&LineCheck{}).Where("line_id = ? AND phase <> ?", "line-1", "done").Update("phase_at", time.Now().UTC().Add(-time.Minute)).Error; e != nil {
			t.Fatal(e)
		}
	}
	// Measuring jobs left open; closing the listeners comes after.
	open := func(id string) int64 {
		var n int64
		s.Store.DB.Model(&Job{}).Where("check_id = ? AND kind <> ? AND status IN ?", id, "check-release", []string{"pending", "running"}).Count(&n)
		return n
	}

	// The exit never takes up its listener: it is silent, and is not asked
	// to measure either, so the check ends without waiting for it.
	request(s, token, "POST", "/api/admin/v1/lines/line-1/check", nil, "")
	relay := claim(t, s, "relay-token")
	answer(t, s, "relay-token", relay[0], true, "", &c.CheckReport{Port: relay[0].Payload.Check.Ports[0]})
	age()
	k := latest(t, s, token, func(k LineCheck) bool { return k.Phase == "done" })
	if k.Nodes[1].Listen != "failed" || !k.Nodes[1].Silent || !strings.Contains(k.Nodes[1].ListenError, "15 秒") {
		t.Fatalf("listen timeout: %+v", k.Nodes)
	}
	if link(k, "public").Status != "ok" || !strings.Contains(link(k, "link-0").Note, "「exit」没有开启检测端口") || !strings.Contains(link(k, "internet").Note, "没有响应检测任务") {
		t.Fatalf("after a silent exit: %+v", k.Links)
	}
	if n := open(k.ID); n != 0 {
		t.Fatalf("%d jobs left open", n)
	}

	// Both listen, then neither measures. The relay's queue also holds the
	// last check's release.
	request(s, token, "POST", "/api/admin/v1/lines/line-1/check", nil, "")
	listen := func(jobs []c.Job) c.Job {
		t.Helper()
		for _, j := range jobs {
			if j.Kind == "check-listen" {
				return j
			}
		}
		t.Fatalf("no listen job: %+v", jobs)
		return c.Job{}
	}
	relayListen, exitListen := listen(claim(t, s, "relay-token")), listen(claim(t, s, "node-test-token"))
	answer(t, s, "relay-token", relayListen, true, "", &c.CheckReport{Port: relayListen.Payload.Check.Ports[0]})
	answer(t, s, "node-test-token", exitListen, true, "", &c.CheckReport{Port: exitListen.Payload.Check.Ports[0]})
	k = latest(t, s, token, func(k LineCheck) bool { return k.Phase == "dial" && link(k, "public").Status != "pending" })
	age()
	k = latest(t, s, token, func(k LineCheck) bool { return k.Phase == "done" })
	if link(k, "link-0").Status != "failed" || !strings.Contains(link(k, "internet").Note, "规定时间") {
		t.Fatalf("dial timeout: %+v", k.Links)
	}
	if n := open(k.ID); n != 0 {
		t.Fatalf("%d jobs left open", n)
	}
}

func TestConnectivityCheckUsesChosenPorts(t *testing.T) {
	s, token, _ := checkFixture(t)
	finish := func() {
		t.Helper()
		s.Store.DB.Model(&LineCheck{}).Where("phase <> ?", "done").Updates(map[string]any{"phase": "done"})
		s.Store.DB.Model(&Job{}).Where("check_id <> ?", "").Update("status", "cancelled")
	}
	listenPorts := func() map[string][]int {
		t.Helper()
		out := map[string][]int{}
		for _, nodeToken := range []string{"relay-token", "node-test-token"} {
			for _, j := range claim(t, s, nodeToken) {
				out[nodeToken] = j.Payload.Check.Ports
			}
		}
		return out
	}

	// Out of the endpoint's range, or held by a session: refused, saying why.
	w := request(s, token, "POST", "/api/admin/v1/lines/line-1/check", map[string]any{"ports": map[string]int{"ep-0": 30000}}, "")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "21000–21003") {
		t.Fatalf("out of range: %d %s", w.Code, w.Body.String())
	}
	s.Store.DB.Create(&Reservation{ID: "held", SessionID: "someone", NodeID: "node-1", Port: 20002})
	w = request(s, token, "POST", "/api/admin/v1/lines/line-1/check", map[string]any{"ports": map[string]int{"ep-1": 20002}}, "")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "正被一个会话使用") {
		t.Fatalf("held port: %d %s", w.Code, w.Body.String())
	}

	// Exactly the chosen ports are opened.
	w = request(s, token, "POST", "/api/admin/v1/lines/line-1/check", map[string]any{"ports": map[string]int{"ep-0": 21000, "ep-1": 20000}}, "")
	var k LineCheck
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &k) != nil || k.Requested["ep-0"] != 21000 || k.Nodes[0].RangeStart != 21000 || k.Nodes[0].RangeEnd != 21003 {
		t.Fatalf("chosen: %d %s", w.Code, w.Body.String())
	}
	ports := listenPorts()
	if len(ports["relay-token"]) != 1 || ports["relay-token"][0] != 21000 || len(ports["node-test-token"]) != 1 || ports["node-test-token"][0] != 20000 {
		t.Fatalf("listen ports: %v", ports)
	}
	finish()

	// Opening the panel again keeps them.
	w = request(s, token, "POST", "/api/admin/v1/lines/line-1/check", nil, "")
	if ports = listenPorts(); w.Code != 200 || ports["relay-token"][0] != 21000 || ports["node-test-token"][0] != 20000 {
		t.Fatalf("reused: %d %v", w.Code, ports)
	}
	finish()

	// A kept port a session has since taken is reported on its hop.
	s.Store.DB.Model(&Reservation{}).Where("id = ?", "held").Update("port", 20000)
	request(s, token, "POST", "/api/admin/v1/lines/line-1/check", nil, "")
	k = latest(t, s, token, func(k LineCheck) bool { return true })
	if k.Nodes[1].Listen != "failed" || !strings.Contains(k.Nodes[1].ListenError, "正被一个会话使用") || k.Nodes[0].Listen != "pending" {
		t.Fatalf("kept port now held: %+v", k.Nodes)
	}
	claim(t, s, "relay-token")
	finish()

	// An empty choice goes back to automatic: the free ports from the
	// lowest, as sessions take them; 20000 is held by a session.
	w = request(s, token, "POST", "/api/admin/v1/lines/line-1/check", map[string]any{"ports": map[string]int{}}, "")
	ports = listenPorts()
	if w.Code != 200 || !slices.Equal(ports["relay-token"], []int{21000, 21001, 21002, 21003}) || !slices.Equal(ports["node-test-token"], []int{20001, 20002, 20003, 20004, 20005}) {
		t.Fatalf("automatic: %d %v", w.Code, ports)
	}
}
