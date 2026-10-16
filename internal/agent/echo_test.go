package agent

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func echoes(t *testing.T, port int) {
	t.Helper()
	conn, e := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
	if e != nil {
		t.Fatalf("latency echo not listening on %d: %v", port, e)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	frame := []byte("FLA1abcdefghijklmnopqrstuvwx")
	conn.Write(frame)
	got := make([]byte, len(frame))
	if _, e := io.ReadFull(conn, got); e != nil || string(got) != string(frame) {
		t.Fatalf("latency echo on %d did not answer: %q %v", port, got, e)
	}
}

// The echo keeps its port across agent restarts, for sessions that outlive
// the agent, and moves off it when a session is given that port.
func TestEchoKeepsItsPortAndGivesWay(t *testing.T) {
	dir := t.TempDir()
	m := fakeManager(t, dir, "listen")
	port, e := m.Echo()
	if e != nil {
		t.Fatal(e)
	}
	echoes(t, port)
	if again, _ := m.Echo(); again != port {
		t.Fatalf("a second request moved the echo from %d to %d", port, again)
	}
	m.Shutdown()

	m = fakeManager(t, dir, "listen")
	defer m.Shutdown()
	echoes(t, port)
	if again, _ := m.Echo(); again != port {
		t.Fatalf("restart moved the echo from %d to %d", port, again)
	}

	rt := fakeRuntimeSpec(t)
	rt.ListenPort = port
	if e := m.Prepare(rt); e != nil {
		t.Fatalf("the echo kept a port a session needs: %v", e)
	}
	defer m.Stop(rt.ID)
	moved, e := m.Echo()
	if e != nil || moved == port {
		t.Fatalf("echo did not move: %d %v", moved, e)
	}
	echoes(t, moved)
	b, _ := os.ReadFile(filepath.Join(dir, echoFile))
	if strings.TrimSpace(string(b)) != strconv.Itoa(moved) {
		t.Fatalf("saved echo port %q, want %d", b, moved)
	}
}
