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

func Probe(ctx context.Context, socks, urlString, udpTarget string) (c.ProbeResult, error) {
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
	transport := &http.Transport{DialContext: cd.DialContext, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	rtts := []float64{}
	for i := 0; i < 3; i++ {
		start := time.Now()
		req, _ := http.NewRequestWithContext(ctx, "GET", urlString, nil)
		res, e := client.Do(req)
		if e != nil {
			return out, errors.New("TCP tunnel probe failed")
		}
		var data struct {
			IP string `json:"ip"`
		}
		e = json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&data)
		res.Body.Close()
		if e != nil || res.StatusCode != 200 || net.ParseIP(data.IP) == nil {
			return out, errors.New("probe endpoint must return JSON with actual peer ip")
		}
		out.ExitIP = data.IP
		rtts = append(rtts, float64(time.Since(start).Microseconds())/1000)
	}
	sort.Float64s(rtts)
	out.TCP = true
	out.RTTMS = rtts[1]
	out.JitterMS = rtts[2] - rtts[0]
	out.Samples = 3
	if udpTarget != "" {
		ok := 0
		for i := 0; i < 3; i++ {
			if UDPProbe(ctx, socks, udpTarget) == nil {
				ok++
			}
		}
		out.Loss = float64(3-ok) / 3
		out.UDP = ok == 3
		if !out.UDP {
			return out, errors.New("UDP tunnel probe failed")
		}
	}
	return out, nil
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
