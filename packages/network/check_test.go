package network

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestCheckListenerAnswersOnlyItsOwnToken(t *testing.T) {
	token := bytes.Repeat([]byte{7}, CheckTokenBytes)
	busy, e := net.Listen("tcp", "0.0.0.0:0")
	if e != nil {
		t.Fatal(e)
	}
	defer busy.Close()
	// The first candidate is taken: the listener moves on to the next.
	want := freePort(t)
	port, stop, e := CheckListen("0.0.0.0", []int{busy.Addr().(*net.TCPAddr).Port, want}, token, true, 5*time.Second)
	if e != nil || port != want {
		t.Fatalf("listen: port %d, %v", port, e)
	}
	ctx := context.Background()
	r := CheckDial(ctx, c.CheckTarget{ID: "link-0", Host: "127.0.0.1", Port: port, Token: hex.EncodeToString(token), UDP: true})
	if r.ID != "link-0" || !r.TCP.OK || r.TCP.Loss != 0 || r.TCP.Samples != 4 || !r.UDP.OK || r.UDP.Loss != 0 || r.UDP.RTTMS < 0 {
		t.Fatalf("own token: %+v %+v", r.TCP, r.UDP)
	}

	other := hex.EncodeToString(bytes.Repeat([]byte{8}, CheckTokenBytes))
	r = CheckDial(ctx, c.CheckTarget{ID: "x", Host: "127.0.0.1", Port: port, Token: other, UDP: true})
	if r.TCP.OK || !strings.Contains(r.TCP.Error, "不是这次检测的监听程序") || r.UDP.OK || r.UDP.Loss != 1 {
		t.Fatalf("another check's token was answered: %+v %+v", r.TCP, r.UDP)
	}

	// A service that only has to accept: no token, no greeting.
	r = CheckDial(ctx, c.CheckTarget{ID: "plain", Host: "127.0.0.1", Port: busy.Addr().(*net.TCPAddr).Port, UDP: true})
	if !r.TCP.OK || r.UDP != nil {
		t.Fatalf("plain service: %+v %+v", r.TCP, r.UDP)
	}

	// Stopped, the port is free again at once, for a session or a new check.
	stop()
	again, e := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(port)))
	if e != nil {
		t.Fatalf("port still held after stop: %v", e)
	}
	again.Close()
}

func TestCheckListenerCloses(t *testing.T) {
	token := bytes.Repeat([]byte{1}, CheckTokenBytes)
	port, _, e := CheckListen("0.0.0.0", []int{freePort(t)}, token, false, 200*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(500 * time.Millisecond)
	r := CheckDial(context.Background(), c.CheckTarget{ID: "late", Host: "127.0.0.1", Port: port, Token: hex.EncodeToString(token)})
	if r.TCP.OK || r.TCP.Error == "" {
		t.Fatalf("listener still open: %+v", r.TCP)
	}
}

func TestCheckDialExplainsFailures(t *testing.T) {
	r := CheckDial(context.Background(), c.CheckTarget{ID: "closed", Host: "127.0.0.1", Port: freePort(t)})
	if r.TCP.OK || !strings.HasPrefix(r.TCP.Error, "连接被拒绝") {
		t.Fatalf("refused: %+v", r.TCP)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	// TEST-NET-1 is never routed: the dial times out. With a token, a local
	// transparent proxy that accepts every connection cannot pass for it.
	token := hex.EncodeToString(bytes.Repeat([]byte{2}, CheckTokenBytes))
	r = CheckDial(ctx, c.CheckTarget{ID: "silent", Host: "192.0.2.1", Port: 9, Token: token})
	if r.TCP.OK || r.TCP.Error == "" {
		t.Fatalf("silent: %+v", r.TCP)
	}
}

func TestCheckDialMeasuresInternetThroughTheProbe(t *testing.T) {
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"ip": "198.51.100.7"})
	}))
	defer probe.Close()
	echo, e := net.ListenPacket("udp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer echo.Close()
	go func() {
		b := make([]byte, 64)
		for {
			n, peer, e := echo.ReadFrom(b)
			if e != nil {
				return
			}
			if n == 28 && bytes.HasPrefix(b, []byte("FLA1")) {
				echo.WriteTo(b[:n], peer)
			}
		}
	}()
	r := CheckDial(context.Background(), c.CheckTarget{ID: "internet", URL: probe.URL, UDPTarget: echo.LocalAddr().String()})
	if !r.TCP.OK || r.TCP.Samples != 3 || r.ExitIP != "198.51.100.7" || !r.UDP.OK {
		t.Fatalf("internet: %+v %+v %q", r.TCP, r.UDP, r.ExitIP)
	}
}
