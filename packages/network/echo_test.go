package network

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func echoServer(t *testing.T) string {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { l.Close() })
	go ServeEcho(l)
	return l.Addr().String()
}

// slowProbeServer answers like the probe service, 150 ms late: the leg
// from the exit on to the probe service that game traffic never takes.
func slowProbeServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		json.NewEncoder(w).Encode(map[string]string{"ip": "203.0.113.7"})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestProbeTimesTheExitEcho(t *testing.T) {
	socks := newSocks5(t, nil)
	srv := slowProbeServer(t)
	out, e := Probe(context.Background(), socks.listener.Addr().String(), srv.URL, "127.0.0.1:9", echoServer(t))
	if e != nil {
		t.Fatal(e)
	}
	if out.Samples != echoSamples || out.ExitIP != "203.0.113.7" || !out.UDP {
		t.Fatalf("unexpected probe result: %+v", out)
	}
	if out.RTTMS >= 100 {
		t.Fatalf("round trip includes the leg to the probe service: %.1f ms", out.RTTMS)
	}
	if n := socks.dials.Load(); n != 2 {
		t.Fatalf("want one connection to the probe service and one to the echo, got %d dials", n)
	}
}

// An exit whose echo cannot be reached still passes, with the round trip to
// the probe service.
func TestProbeWithoutEchoKeepsTheProbeServiceTime(t *testing.T) {
	socks := newSocks5(t, nil)
	srv := slowProbeServer(t)
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	closed := l.Addr().String()
	l.Close()
	out, e := Probe(context.Background(), socks.listener.Addr().String(), srv.URL, "127.0.0.1:9", closed)
	if e != nil {
		t.Fatal(e)
	}
	if out.Samples != tcpSamples || out.RTTMS < 150 {
		t.Fatalf("fallback lost the probe service measurement: %+v", out)
	}
}

func TestEchoAnswersOnlyItsFrames(t *testing.T) {
	addr := echoServer(t)
	conn, e := net.Dial("tcp", addr)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	frame := []byte("FLA1abcdefghijklmnopqrstuvwx")
	for i := 0; i < 3; i++ {
		conn.Write(frame)
		got := make([]byte, len(frame))
		if _, e := io.ReadFull(conn, got); e != nil || string(got) != string(frame) {
			t.Fatalf("frame %d not echoed: %q %v", i, got, e)
		}
	}
	conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
	if n, e := conn.Read(make([]byte, 64)); e == nil {
		t.Fatalf("echo answered %d bytes of something else", n)
	}
}
