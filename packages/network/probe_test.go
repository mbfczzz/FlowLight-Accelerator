package network

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// socks5 is a minimal SOCKS5 server with CONNECT and UDP ASSOCIATE, standing in
// for sslocal. dropUDP decides whether to swallow the n-th UDP datagram.
type socks5 struct {
	listener net.Listener
	udp      net.PacketConn
	dials    atomic.Int32
	datagram atomic.Int32
	dropUDP  func(n int32) bool
}

func newSocks5(t *testing.T, dropUDP func(int32) bool) *socks5 {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	u, e := net.ListenPacket("udp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	s := &socks5{listener: l, udp: u, dropUDP: dropUDP}
	t.Cleanup(func() { l.Close(); u.Close() })
	go s.serveUDP()
	go func() {
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			go s.serve(c)
		}
	}()
	return s
}
func (s *socks5) serve(c net.Conn) {
	defer c.Close()
	head := make([]byte, 3)
	if _, e := io.ReadFull(c, head); e != nil {
		return
	}
	c.Write([]byte{5, 0})
	req := make([]byte, 4)
	if _, e := io.ReadFull(c, req); e != nil {
		return
	}
	host, e := readAddress(c, req[3])
	if e != nil {
		return
	}
	port := make([]byte, 2)
	io.ReadFull(c, port)
	if req[1] == 3 {
		a := s.udp.LocalAddr().(*net.UDPAddr)
		reply := []byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0}
		binary.BigEndian.PutUint16(reply[8:], uint16(a.Port))
		c.Write(reply)
		io.Copy(io.Discard, c)
		return
	}
	s.dials.Add(1)
	upstream, e := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port)))))
	if e != nil {
		c.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer upstream.Close()
	c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	go io.Copy(upstream, c)
	io.Copy(c, upstream)
}

// serveUDP echoes the SOCKS UDP payload back, as the probe server would.
func (s *socks5) serveUDP() {
	b := make([]byte, 2048)
	for {
		n, peer, e := s.udp.ReadFrom(b)
		if e != nil {
			return
		}
		if s.dropUDP != nil && s.dropUDP(s.datagram.Add(1)) {
			continue
		}
		s.udp.WriteTo(append([]byte(nil), b[:n]...), peer)
	}
}
func probeServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"ip": "203.0.113.7"})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestProbeReusesOneTunnelConnection(t *testing.T) {
	socks := newSocks5(t, nil)
	srv := probeServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, e := Probe(ctx, socks.listener.Addr().String(), srv.URL, "127.0.0.1:9", "")
	if e != nil {
		t.Fatal(e)
	}
	if out.Samples != tcpSamples || !out.TCP || !out.UDP || out.ExitIP != "203.0.113.7" || out.Loss != 0 {
		t.Fatalf("unexpected probe result: %+v", out)
	}
	if n := socks.dials.Load(); n != 1 {
		t.Fatalf("each RTT sample opened a new tunnel connection: %d dials", n)
	}
}

func TestProbeToleratesOneLostDatagram(t *testing.T) {
	socks := newSocks5(t, func(n int32) bool { return n == 2 })
	srv := probeServer(t)
	out, e := Probe(context.Background(), socks.listener.Addr().String(), srv.URL, "127.0.0.1:9", "")
	if e != nil {
		t.Fatalf("single lost datagram failed the probe: %v", e)
	}
	if !out.UDP || out.Loss != 0.2 {
		t.Fatalf("loss not measured: %+v", out)
	}
}

func TestProbeFailsOnHeavyUDPLoss(t *testing.T) {
	socks := newSocks5(t, func(n int32) bool { return n%2 == 0 })
	srv := probeServer(t)
	out, e := Probe(context.Background(), socks.listener.Addr().String(), srv.URL, "127.0.0.1:9", "")
	if e == nil || out.UDP {
		t.Fatalf("40%% UDP loss reported as healthy: %+v", out)
	}
	if !bytes.Contains([]byte(e.Error()), []byte("3 of 5")) {
		t.Fatalf("failure does not say how many replies arrived: %v", e)
	}
}
