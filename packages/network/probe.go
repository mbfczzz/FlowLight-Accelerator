package network

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"golang.org/x/net/proxy"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"time"
)

// Probe proves the path out through the SOCKS5 proxy at socks: HTTP to the
// probe service (which also names the exit address) and UDP echoes. With
// echo, the exit's latency echo, the round trip reported is the one to the
// exit; an echo that does not answer leaves the probe service's.
func Probe(ctx context.Context, socks, urlString, udpTarget, echo string) (c.ProbeResult, error) {
	out := c.ProbeResult{ObservedAt: time.Now().UTC()}
	u, e := url.Parse(urlString)
	if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return out, errors.New("controlled probe URL required")
	}
	dialer, e := proxy.SOCKS5("tcp", socks, nil, &net.Dialer{Timeout: 5 * time.Second})
	if e != nil {
		return out, e
	}
	cd := dialer.(proxy.ContextDialer)
	// One warmed connection: the first request pays for the SOCKS, SS and
	// TLS handshakes and is not timed, so samples measure the path RTT.
	transport := &http.Transport{DialContext: cd.DialContext, MaxIdleConnsPerHost: 1}
	defer transport.CloseIdleConnections()
	ip, rtts, e := timedRequests(ctx, transport, urlString)
	if e != nil {
		return out, fmt.Errorf("TCP tunnel probe failed: %w", e)
	}
	out.ExitIP = ip
	out.TCP = true
	out.RTTMS = rtts[len(rtts)/2]
	out.JitterMS = rtts[len(rtts)-1] - rtts[0]
	out.Samples = len(rtts)
	if echo != "" {
		if rtts, e := EchoRTT(ctx, socks, echo); e == nil {
			out.RTTMS = rtts[len(rtts)/2]
			out.JitterMS = rtts[len(rtts)-1] - rtts[0]
			out.Samples = len(rtts)
		}
	}
	if udpTarget != "" {
		ok := 0
		var last error
		for i := 0; i < udpSamples; i++ {
			if e := UDPProbe(ctx, socks, udpTarget); e == nil {
				ok++
			} else {
				last = e
			}
		}
		out.Loss = float64(udpSamples-ok) / udpSamples
		out.UDP = ok >= udpRequired
		if !out.UDP {
			return out, fmt.Errorf("UDP tunnel probe failed: %d of %d replies: %v", ok, udpSamples, last)
		}
	}
	return out, nil
}

// A probe takes tcpSamples timed HTTP requests after one warm-up request, and
// udpSamples UDP echoes of which udpRequired must return.
const (
	tcpSamples  = 3
	udpSamples  = 5
	udpRequired = 4
)

// timedRequests makes one untimed request, then tcpSamples timed ones over
// the warmed connection, and returns the address the probe saw and the
// sorted round trips.
func timedRequests(ctx context.Context, transport *http.Transport, urlString string) (string, []float64, error) {
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ip := ""
	rtts := []float64{}
	for i := 0; i <= tcpSamples; i++ {
		start := time.Now()
		seen, e := probeOnce(ctx, client, urlString)
		if e != nil {
			return ip, rtts, e
		}
		ip = seen
		if i > 0 {
			rtts = append(rtts, float64(time.Since(start).Microseconds())/1000)
		}
	}
	sort.Float64s(rtts)
	return ip, rtts, nil
}

func probeOnce(ctx context.Context, client *http.Client, urlString string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", urlString, nil)
	res, e := client.Do(req)
	if e != nil {
		return "", e
	}
	defer res.Body.Close()
	var data struct {
		IP string `json:"ip"`
	}
	e = json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&data)
	// Drain so the connection is reused for the next sample.
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if e != nil || res.StatusCode != 200 || net.ParseIP(data.IP) == nil {
		return "", errors.New("probe endpoint must return JSON with actual peer ip")
	}
	return data.IP, nil
}
func UDPProbe(ctx context.Context, socks, target string) error {
	d := net.Dialer{Timeout: 3 * time.Second}
	tcp, e := d.DialContext(ctx, "tcp", socks)
	if e != nil {
		return e
	}
	defer tcp.Close()
	tcp.SetDeadline(time.Now().Add(4 * time.Second))
	if _, e = tcp.Write([]byte{5, 1, 0}); e != nil {
		return e
	}
	head := make([]byte, 2)
	if _, e = io.ReadFull(tcp, head); e != nil || !bytes.Equal(head, []byte{5, 0}) {
		return errors.New("SOCKS greeting rejected")
	}
	tcp.Write([]byte{5, 3, 0, 1, 0, 0, 0, 0, 0, 0})
	header := make([]byte, 4)
	if _, e = io.ReadFull(tcp, header); e != nil || header[1] != 0 {
		return errors.New("SOCKS UDP association rejected")
	}
	host, e := readAddress(tcp, header[3])
	if e != nil {
		return e
	}
	port := make([]byte, 2)
	if _, e = io.ReadFull(tcp, port); e != nil {
		return e
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		host, _, _ = net.SplitHostPort(socks)
	}
	udp, e := net.DialTimeout("udp", net.JoinHostPort(host, fmt.Sprint(binary.BigEndian.Uint16(port))), time.Second*3)
	if e != nil {
		return e
	}
	defer udp.Close()
	udp.SetDeadline(time.Now().Add(3 * time.Second))
	h, p, e := net.SplitHostPort(target)
	if e != nil {
		return e
	}
	var pnum int
	if _, e = fmt.Sscan(p, &pnum); e != nil || pnum < 1 || pnum > 65535 {
		return errors.New("invalid UDP target")
	}
	addr := []byte{3, byte(len(h))}
	if len(h) > 253 {
		return errors.New("target hostname too long")
	}
	addr = append(addr, []byte(h)...)
	addr = append(addr, byte(pnum>>8), byte(pnum))
	nonce := make([]byte, 24)
	rand.Read(nonce)
	payload := append([]byte("FLA1"), nonce...)
	packet := append([]byte{0, 0, 0}, addr...)
	packet = append(packet, payload...)
	if _, e = udp.Write(packet); e != nil {
		return e
	}
	b := make([]byte, 1024)
	n, e := udp.Read(b)
	if e != nil {
		return e
	}
	if n < 4 || b[2] != 0 {
		return errors.New("invalid SOCKS UDP response")
	}
	reader := bytes.NewReader(b[4:n])
	if _, e = readAddress(reader, b[3]); e != nil {
		return e
	}
	tail, e := io.ReadAll(reader)
	if e != nil || len(tail) < 2 || !bytes.Equal(tail[2:], payload) {
		return errors.New("UDP probe nonce mismatch")
	}
	return nil
}
func readAddress(r io.Reader, t byte) (string, error) {
	switch t {
	case 1:
		b := make([]byte, 4)
		_, e := io.ReadFull(r, b)
		return net.IP(b).String(), e
	case 4:
		b := make([]byte, 16)
		_, e := io.ReadFull(r, b)
		return net.IP(b).String(), e
	case 3:
		l := make([]byte, 1)
		if _, e := io.ReadFull(r, l); e != nil {
			return "", e
		}
		b := make([]byte, int(l[0]))
		_, e := io.ReadFull(r, b)
		return string(b), e
	}
	return "", errors.New("invalid SOCKS address")
}
