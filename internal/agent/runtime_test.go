package agent

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRefusesUnpinnedBinariesAndExpiredRuntime(t *testing.T) {
	m, e := NewManager(t.TempDir(), Binaries{})
	if e != nil {
		t.Fatal(e)
	}
	rt := c.Runtime{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SessionID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ListenPort: 20000, Kind: "ss2022", ExpiresAt: time.Now().Add(time.Minute)}
	if e = m.Prepare(rt); e == nil {
		t.Fatal("unpinned executable accepted")
	}
	rt.ExpiresAt = time.Now().Add(-time.Minute)
	if e = m.Prepare(rt); e == nil {
		t.Fatal("expired lease accepted")
	}
	if e = m.Stop("../outside"); e == nil {
		t.Fatal("unsafe runtime path accepted")
	}
}
func freePort(t *testing.T) int {
	t.Helper()
	for attempt := 0; attempt < 20; attempt++ {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		p := l.Addr().(*net.TCPAddr).Port
		udp, err := net.ListenPacket("udp", net.JoinHostPort("127.0.0.1", fmt.Sprint(p)))
		l.Close()
		if err != nil {
			continue
		}
		udp.Close()
		return p
	}
	t.Fatal("could not allocate a port available for both TCP and UDP")
	return 0
}

// Uses real, checksum-pinned executables against loopback HTTP and UDP targets.
// No TUN, routes, DNS or real game processes are changed by this test.
func TestNativeSS2022RealmTCPUDPAndLeaseRevoke(t *testing.T) {
	dir := os.Getenv("FLOWLIGHT_NATIVE_TEST_DIR")
	if dir == "" {
		t.Skip("set FLOWLIGHT_NATIVE_TEST_DIR to verified native tools")
	}
	bins := Binaries{SSserver: filepath.Join(dir, "ss", "ssserver.exe"), SSlocal: filepath.Join(dir, "ss", "sslocal.exe"), Realm: filepath.Join(dir, "realm", "realm.exe"), Hashes: map[string]string{}}
	for k, p := range map[string]string{"ss2022": bins.SSserver, "sslocal": bins.SSlocal, "realm": bins.Realm} {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		h := sha256.Sum256(b)
		bins.Hashes[k] = hex.EncodeToString(h[:])
	}
	m, e := NewManager(t.TempDir(), bins)
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		json.NewEncoder(w).Encode(map[string]string{"ip": ip})
	}))
	defer server.Close()
	udp, e := net.ListenPacket("udp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer udp.Close()
	go func() {
		b := make([]byte, 128)
		for {
			n, peer, e := udp.ReadFrom(b)
			if e != nil {
				return
			}
			if n == 28 {
				_, _ = udp.WriteTo(b[:n], peer)
			}
		}
	}()
	exitPort, entryPort := freePort(t), freePort(t)
	session := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	exit := c.Runtime{ID: "11111111111111111111111111111111", SessionID: session, Kind: "ss2022", ListenPort: exitPort, Cipher: "2022-blake3-aes-128-gcm", Password: base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")), UDP: true, ExpiresAt: time.Now().Add(time.Minute)}
	if e = m.Prepare(exit); e != nil {
		t.Fatal(e)
	}
	defer m.Stop(exit.ID)
	entry := c.Runtime{ID: "22222222222222222222222222222222", SessionID: session, Kind: "realm", ListenPort: entryPort, RemoteHost: "127.0.0.1", RemotePort: exitPort, UDP: true, ExpiresAt: exit.ExpiresAt}
	if e = m.Prepare(entry); e != nil {
		t.Fatal(e)
	}
	defer m.Stop(entry.ID)
	for _, port := range []int{exitPort, entryPort} {
		ready := false
		for i := 0; i < 50; i++ {
			conn, e := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmtPort(port)), 50*time.Millisecond)
			if e == nil {
				conn.Close()
				ready = true
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !ready {
			t.Fatalf("native process failed to listen on %d (exit=%d entry=%d)", port, exitPort, entryPort)
		}
	}
	result, e := m.Probe(context.Background(), c.Connection{Host: "127.0.0.1", Port: entryPort, Cipher: exit.Cipher, Password: exit.Password, UDP: true, ProbeURL: server.URL, ProbeUDP: udp.LocalAddr().String()})
	if e != nil {
		t.Fatal(e)
	}
	if !result.TCP || !result.UDP || result.Samples != 3 || result.ExitIP != "127.0.0.1" {
		t.Fatalf("incomplete actual probe: %+v", result)
	}
	t.Logf("actual SS2022 + Realm TCP/UDP passed: samples=%d RTT=%.2f ms loss=%.0f%%", result.Samples, result.RTTMS, result.Loss*100)
	// An authoritative heartbeat omitting the session must revoke it, even
	// when the previous local lease still has time left.
	if err := m.Renew(map[string]time.Time{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	m.Expire()
	conn, e := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmtPort(entryPort)), time.Second)
	if e == nil {
		conn.Close()
		t.Fatal("expired ingress still accepts connections")
	}
}
func fmtPort(p int) string { return fmt.Sprint(p) }

func TestRestartWaitsForAuthoritativeLease(t *testing.T) {
	dir := t.TempDir()
	rt := c.Runtime{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SessionID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Kind: "ss2022", ListenPort: 21000, ExpiresAt: time.Now().Add(24 * time.Hour)}
	journal, _ := json.Marshal(Process{Runtime: rt})
	path := filepath.Join(dir, rt.ID+".runtime")
	if err := os.WriteFile(path, journal, 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(dir, Binaries{})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Processes) != 0 || len(m.pending) != 1 {
		t.Fatal("persisted authorization resumed before reconciliation")
	}
	if err := m.Renew(map[string]time.Time{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("revoked journal retained")
	}
	if len(m.Processes) != 0 {
		t.Fatal("revoked runtime restarted")
	}
}

// A session takes its port from a test listener still open on it, and a
// check that chose a port takes it over from the last check's listener.
func TestTestPortsGiveWay(t *testing.T) {
	port := freePort(t)
	task := c.CheckTask{Ports: []int{port}, UDP: true, Seconds: 60, Token: "abababababababababababababababab"}
	if _, e := checkListen(task); e != nil {
		t.Fatal(e)
	}
	if _, e := checkListen(task); e != nil {
		t.Fatalf("a chosen port was not taken over: %v", e)
	}
	if portAvailable(port, true) {
		t.Fatal("test listener not open")
	}
	releaseTestPort(port, "")
	if !portAvailable(port, true) {
		t.Fatal("test listener still holds the port a session needs")
	}

	// A finished check closes its own listener, never a newer check's.
	if _, e := checkListen(task); e != nil {
		t.Fatal(e)
	}
	if e := checkRelease(c.CheckTask{Ports: []int{port}, Token: "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd"}); e != nil || portAvailable(port, true) {
		t.Fatalf("another check's release closed this listener: %v", e)
	}
	if e := checkRelease(c.CheckTask{Ports: []int{port}, Token: task.Token}); e != nil || !portAvailable(port, true) {
		t.Fatalf("the check's own release left its port held: %v", e)
	}
	if e := checkRelease(c.CheckTask{Ports: []int{port}, Token: task.Token}); e != nil {
		t.Fatalf("releasing a closed listener: %v", e)
	}
}
