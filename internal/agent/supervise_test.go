package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The test binary doubles as a fake network runtime so supervision can be
// exercised without the real ssserver.
func TestMain(m *testing.M) {
	if mode := os.Getenv("FLOWLIGHT_FAKE_RUNTIME"); mode != "" {
		fakeRuntime(mode)
		return
	}
	os.Exit(m.Run())
}
func fakeRuntime(mode string) {
	b, _ := os.ReadFile(os.Args[len(os.Args)-1])
	var cfg struct {
		Port int `json:"server_port"`
	}
	_ = json.Unmarshal(b, &cfg)
	if mode == "exit" {
		fmt.Fprintln(os.Stderr, "fake runtime: invalid key")
		os.Exit(3)
	}
	l, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.Port))
	if e != nil {
		os.Exit(4)
	}
	// Like ssserver with --manager-addr: report this process's traffic,
	// growing 1000 bytes per report.
	for i, a := range os.Args {
		if a == "--manager-addr" && i+1 < len(os.Args) {
			go func(addr string) {
				conn, e := net.Dial("udp", addr)
				if e != nil {
					return
				}
				for n := int64(1000); ; n += 1000 {
					fmt.Fprintf(conn, "stat: {\"%d\":%d}", cfg.Port, n)
					time.Sleep(40 * time.Millisecond)
				}
			}(os.Args[i+1])
		}
	}
	if mode == "crash" {
		// Outlive the readiness grace, then die as a crashing runtime would.
		time.Sleep(time.Second)
		os.Exit(5)
	}
	for {
		conn, e := l.Accept()
		if e != nil {
			return
		}
		conn.Close()
	}
}
func fakeManager(t *testing.T, dir, mode string) *Manager {
	t.Helper()
	t.Setenv("FLOWLIGHT_FAKE_RUNTIME", mode)
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(b)
	m, e := NewManager(dir, Binaries{SSserver: exe, Hashes: map[string]string{"ss2022": hex.EncodeToString(h[:])}})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func fakeRuntimeSpec(t *testing.T) c.Runtime {
	return c.Runtime{ID: "cccccccccccccccccccccccccccccccc", SessionID: "dddddddddddddddddddddddddddddddd", Kind: "ss2022", ListenPort: freePort(t), ExpiresAt: time.Now().Add(time.Minute)}
}

func TestPrepareFailsWhenRuntimeExitsAtStart(t *testing.T) {
	m := fakeManager(t, t.TempDir(), "exit")
	rt := fakeRuntimeSpec(t)
	e := m.Prepare(rt)
	if e == nil || !strings.Contains(e.Error(), "invalid key") {
		t.Fatalf("start failure not reported with its output: %v", e)
	}
	if len(m.Processes) != 0 {
		t.Fatal("failed runtime still registered")
	}
	if _, e := os.Stat(filepath.Join(m.Dir, rt.ID+".runtime")); !os.IsNotExist(e) {
		t.Fatal("failed runtime journal retained")
	}
}

func TestCrashedRuntimeIsRestartedThenReported(t *testing.T) {
	restartDelay = 20 * time.Millisecond
	defer func() { restartDelay = time.Second }()
	m := fakeManager(t, t.TempDir(), "crash")
	rt := fakeRuntimeSpec(t)
	if e := m.Prepare(rt); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if failed := m.TakeFailures(); len(failed) > 0 {
			if failed[0].SessionID != rt.SessionID {
				t.Fatalf("wrong runtime reported: %+v", failed[0])
			}
			m.Lock()
			running := len(m.Processes)
			m.Unlock()
			if running != 0 {
				t.Fatal("reported runtime still running")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("crashed runtime was never reported")
}

func usageOf(m *Manager, id string) int64 {
	for _, u := range m.Usage() {
		if u.RuntimeID == id {
			return u.Bytes
		}
	}
	return 0
}

func waitUsage(t *testing.T, m *Manager, id string, above int64) int64 {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if v := usageOf(m, id); v > above {
			return v
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("traffic of %s never passed %d", id, above)
	return 0
}

func TestTrafficTotalSurvivesRestartsStopAndAgentRestart(t *testing.T) {
	restartDelay = 20 * time.Millisecond
	defer func() { restartDelay = time.Second }()
	dir := t.TempDir()
	m := fakeManager(t, dir, "crash")
	rt := fakeRuntimeSpec(t)
	if e := m.Prepare(rt); e != nil {
		t.Fatal(e)
	}
	// The crashing process counts up to ~25 reports, then is restarted and
	// its successor starts again from 1000: the total must keep growing.
	first := waitUsage(t, m, rt.ID, 5000)
	m.Lock()
	beforeCrash := m.Processes[rt.ID]
	m.Unlock()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		m.Lock()
		p := m.Processes[rt.ID]
		m.Unlock()
		if p != nil && p != beforeCrash {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	m.Lock()
	p := m.Processes[rt.ID]
	base := int64(0)
	if p != nil {
		base = p.base
	}
	m.Unlock()
	if base < first {
		t.Fatalf("restarted runtime lost its earlier traffic: carried %d, had %d", base, first)
	}
	// The new process counts from zero; the total continues past the carry.
	afterRestart := waitUsage(t, m, rt.ID, base)

	// Stopped: the final total stays reportable until control has it.
	if e := m.Stop(rt.ID); e != nil {
		t.Fatal(e)
	}
	final := usageOf(m, rt.ID)
	if final < afterRestart {
		t.Fatalf("stopped runtime's total dropped: %d < %d", final, afterRestart)
	}
	m.UsageReported([]Usage{{rt.ID, rt.SessionID, final}})
	if usageOf(m, rt.ID) != 0 {
		t.Fatal("reported total of a stopped runtime was kept")
	}
}

func TestTrafficTotalResumesAfterAgentRestart(t *testing.T) {
	dir := t.TempDir()
	m := fakeManager(t, dir, "listen")
	rt := fakeRuntimeSpec(t)
	if e := m.Prepare(rt); e != nil {
		t.Fatal(e)
	}
	before := waitUsage(t, m, rt.ID, 3000)
	m.Shutdown()
	// The journal holds the total across the agent restart.
	again, e := NewManager(dir, m.Binaries)
	if e != nil {
		t.Fatal(e)
	}
	persisted := usageOf(again, rt.ID)
	if persisted < before {
		t.Fatalf("restarted agent lost the journal total: %d (had %d)", persisted, before)
	}
	if e := again.Renew(map[string]time.Time{rt.SessionID: time.Now().Add(time.Minute)}, time.Now()); e != nil {
		t.Fatal(e)
	}
	resumed := waitUsage(t, again, rt.ID, persisted+1000)
	if resumed <= persisted {
		t.Fatalf("resumed runtime did not continue from %d: %d", persisted, resumed)
	}
	again.Shutdown()
}

func TestTrafficTotalKeptWhenStoppedBetweenProcesses(t *testing.T) {
	// A long backoff: the runtime is stopped while waiting to restart.
	restartDelay = 5 * time.Second
	defer func() { restartDelay = time.Second }()
	dir := t.TempDir()
	m := fakeManager(t, dir, "crash")
	rt := fakeRuntimeSpec(t)
	if e := m.Prepare(rt); e != nil {
		t.Fatal(e)
	}
	counted := waitUsage(t, m, rt.ID, 3000)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		m.Lock()
		running := len(m.Processes)
		m.Unlock()
		if running == 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if e := m.Stop(rt.ID); e != nil {
		t.Fatal(e)
	}
	got := usageOf(m, rt.ID)
	if got < counted {
		t.Fatalf("total lost when stopped during restart backoff: %d < %d", got, counted)
	}
	// Still there after an agent restart, until reported.
	again, e := NewManager(dir, m.Binaries)
	if e != nil {
		t.Fatal(e)
	}
	if usageOf(again, rt.ID) != got {
		t.Fatalf("unreported final total lost over an agent restart: %d, want %d", usageOf(again, rt.ID), got)
	}
	again.UsageReported([]Usage{{rt.ID, rt.SessionID, got}})
	third, e := NewManager(dir, m.Binaries)
	if e != nil {
		t.Fatal(e)
	}
	if usageOf(third, rt.ID) != 0 {
		t.Fatal("a reported total came back after a restart")
	}
}

func TestParseStat(t *testing.T) {
	got, ok := parseStat([]byte(`stat: {"20000": 1234, "x": 5, "70000": 9}`))
	if !ok || len(got) != 1 || got[20000] != 1234 {
		t.Fatalf("parse: %v %v", got, ok)
	}
	if _, ok := parseStat([]byte("ping")); ok {
		t.Fatal("non-stat datagram accepted")
	}
}

func TestShutdownKeepsJournalsAndRestartResumes(t *testing.T) {
	dir := t.TempDir()
	m := fakeManager(t, dir, "listen")
	rt := fakeRuntimeSpec(t)
	if e := m.Prepare(rt); e != nil {
		t.Fatal(e)
	}
	m.Shutdown()
	if _, e := os.Stat(filepath.Join(dir, rt.ID+".runtime")); e != nil {
		t.Fatal("graceful shutdown discarded the journal")
	}
	if len(m.TakeFailures()) != 0 {
		t.Fatal("intentional shutdown reported as a failure")
	}
	next := fakeManager(t, dir, "listen")
	if len(next.pending) != 1 {
		t.Fatal("journal not reloaded")
	}
	if e := next.Renew(map[string]time.Time{rt.SessionID: time.Now().Add(time.Minute)}, time.Now()); e != nil {
		t.Fatal(e)
	}
	if len(next.Processes) != 1 {
		t.Fatal("leased runtime not resumed after restart")
	}
	if e := next.Stop(rt.ID); e != nil {
		t.Fatal(e)
	}
}

func TestRenewSparesRuntimeStartedAfterSnapshot(t *testing.T) {
	m := fakeManager(t, t.TempDir(), "listen")
	rt := fakeRuntimeSpec(t)
	sentAt := time.Now()
	if e := m.Prepare(rt); e != nil {
		t.Fatal(e)
	}
	defer m.Stop(rt.ID)
	if e := m.Renew(map[string]time.Time{}, sentAt); e != nil {
		t.Fatal(e)
	}
	m.Expire()
	if len(m.Processes) != 1 {
		t.Fatal("snapshot older than the runtime revoked it")
	}
	if e := m.Renew(map[string]time.Time{}, time.Now()); e != nil {
		t.Fatal(e)
	}
	m.Expire()
	if len(m.Processes) != 0 {
		t.Fatal("authoritative omission did not revoke the runtime")
	}
}

func TestCorruptJournalIsQuarantined(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee.runtime")
	if e := os.WriteFile(path, nil, 0600); e != nil {
		t.Fatal(e)
	}
	m, e := NewManager(dir, Binaries{})
	if e != nil {
		t.Fatalf("corrupt journal blocked startup: %v", e)
	}
	if len(m.pending) != 0 {
		t.Fatal("corrupt journal scheduled")
	}
	if _, e := os.Stat(path + ".corrupt"); e != nil {
		t.Fatal("corrupt journal not quarantined")
	}
}
