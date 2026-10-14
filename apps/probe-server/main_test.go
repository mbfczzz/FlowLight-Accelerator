package main

import (
	"net"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientIPTrustsOnlyConfiguredProxies(t *testing.T) {
	trusted, e := parsePrefixes("127.0.0.1/32, 10.0.0.0/8")
	if e != nil {
		t.Fatal(e)
	}
	cases := []struct{ remote, forwarded, want string }{
		{"127.0.0.1:5000", "198.51.100.7", "198.51.100.7"},
		{"10.1.2.3:5000", "203.0.113.9, 198.51.100.7", "198.51.100.7"},
		{"203.0.113.50:5000", "198.51.100.7", "203.0.113.50"},
		{"127.0.0.1:5000", "", "127.0.0.1"},
		{"127.0.0.1:5000", "not-an-ip", "127.0.0.1"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/probe", nil)
		r.RemoteAddr = c.remote
		if c.forwarded != "" {
			r.Header.Set("X-Forwarded-For", c.forwarded)
		}
		if got := clientIP(r, trusted); got != c.want {
			t.Errorf("remote %s forwarded %q: got %s want %s", c.remote, c.forwarded, got, c.want)
		}
	}
	if _, e := parsePrefixes("bad"); e == nil {
		t.Fatal("invalid CIDR accepted")
	}
}

func TestUDPEchoIsRateLimitedPerSource(t *testing.T) {
	l := newRateLimit(3)
	now := time.Now()
	a := &net.UDPAddr{IP: net.ParseIP("198.51.100.1"), Port: 1000}
	b := &net.UDPAddr{IP: net.ParseIP("198.51.100.2"), Port: 1000}
	for i := 0; i < 3; i++ {
		if !l.allow(a, now) {
			t.Fatal("legitimate probe rate refused")
		}
	}
	if l.allow(a, now) {
		t.Fatal("flood from one source echoed")
	}
	if !l.allow(b, now) {
		t.Fatal("another source throttled by the first")
	}
	if !l.allow(a, now.Add(time.Second)) {
		t.Fatal("limit did not reset after the window")
	}
}
