package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

func main() {
	tcp := flag.String("http", "127.0.0.1:9080", "HTTP probe listener")
	udp := flag.String("udp", "0.0.0.0:9081", "UDP echo probe listener")
	cert := flag.String("tls-cert", "", "serve the HTTP probe over TLS with this certificate (recommended)")
	key := flag.String("tls-key", "", "private key for -tls-cert")
	proxies := flag.String("trusted-proxies", "", "comma-separated CIDRs of reverse proxies whose X-Forwarded-For is trusted")
	flag.Parse()
	trusted, e := parsePrefixes(*proxies)
	if e != nil {
		log.Fatal(e)
	}
	if (*cert == "") != (*key == "") {
		log.Fatal("-tls-cert and -tls-key must be set together")
	}
	go echo(*udp)
	m := http.NewServeMux()
	m.HandleFunc("GET /probe", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ip": clientIP(r, trusted)})
	})
	srv := &http.Server{Addr: *tcp, Handler: m, ReadHeaderTimeout: 5 * time.Second}
	if *cert != "" {
		log.Fatal(srv.ListenAndServeTLS(*cert, *key))
	}
	log.Fatal(srv.ListenAndServe())
}

// echo returns 28-byte FLA1 probes to their sender. Read errors other than a
// closed socket are transient and must not silently stop the UDP probe.
func echo(addr string) {
	conn, e := net.ListenPacket("udp", addr)
	if e != nil {
		log.Fatal(e)
	}
	defer conn.Close()
	b := make([]byte, 128)
	limit := newRateLimit(udpPerSecond)
	for {
		n, peer, e := conn.ReadFrom(b)
		if e != nil {
			if errors.Is(e, net.ErrClosed) {
				return
			}
			log.Printf("udp probe read: %v", e)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if n == 28 && bytes.Equal(b[:4], []byte("FLA1")) && limit.allow(peer, time.Now()) {
			_, _ = conn.WriteTo(b[:n], peer)
		}
	}
}

// Probes send a handful of datagrams; anything faster is someone using the
// echo as a reflector with a spoofed source.
const udpPerSecond = 20

// rateLimit caps echoes per source address in one-second windows.
type rateLimit struct {
	perSecond int
	window    time.Time
	counts    map[string]int
}

func newRateLimit(perSecond int) *rateLimit {
	return &rateLimit{perSecond: perSecond, counts: map[string]int{}}
}
func (l *rateLimit) allow(peer net.Addr, now time.Time) bool {
	if now.Sub(l.window) >= time.Second {
		l.window = now
		clear(l.counts)
	}
	host := peer.String()
	if a, ok := peer.(*net.UDPAddr); ok {
		host = a.IP.String()
	}
	l.counts[host]++
	return l.counts[host] <= l.perSecond
}

// clientIP reports the address the probe was actually reached from. Behind a
// trusted reverse proxy, that is the last address the proxy appended to
// X-Forwarded-For; otherwise it is the TCP peer.
func clientIP(r *http.Request, trusted []netip.Prefix) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	peer, e := netip.ParseAddr(host)
	if e != nil || !contains(trusted, peer.Unmap()) {
		return host
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	if forwarded, e := netip.ParseAddr(strings.TrimSpace(hops[len(hops)-1])); e == nil {
		return forwarded.Unmap().String()
	}
	return host
}
func contains(prefixes []netip.Prefix, a netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
func parsePrefixes(s string) ([]netip.Prefix, error) {
	out := []netip.Prefix{}
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		p, e := netip.ParsePrefix(part)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, nil
}
