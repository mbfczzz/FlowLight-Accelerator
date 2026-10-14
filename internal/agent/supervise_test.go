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
