package network

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
)

// Connectivity checks between the nodes of a line (internal/control/check.go).
// A node opens a short-lived listener on a port its endpoint serves from, so
// the firewall rules that sessions need are the ones tested; the node before
// it on the path dials that port. A TCP greeting and a UDP datagram carry the
// check's token in the probe server's FLA1 framing, and only a listener that
// knows the token answers, so a reply proves this check's listener was
// reached and not some other service on the way.

const (
	CheckTokenBytes = 16
	checkMagic      = "FLA1"
	checkTCPSamples = 4
	checkUDPSamples = 5
	// A probe echo is 28 bytes: the magic and 24 bytes the sender chose.
	echoBytes = 28
)

var errNotOurs = errors.New("not the check listener")

// CheckListen opens TCP, and UDP when asked, on host and the first of ports
// that is free in both, answers greetings that carry token until d has
// passed, and returns the port. host is the address session services listen
// on, so the check reaches the listener only where a session could.
func CheckListen(host string, ports []int, token []byte, udp bool, d time.Duration) (int, error) {
	if len(token) != CheckTokenBytes {
		return 0, errors.New("invalid check token")
	}
	greeting := append([]byte(checkMagic), token...)
	var last error
	for _, p := range ports {
		if p < 1 || p > 65535 {
			continue
		}
		addr := net.JoinHostPort(host, strconv.Itoa(p))
		tl, e := net.Listen("tcp", addr)
		if e != nil {
			last = e
			continue
		}
		var ul net.PacketConn
		if udp {
			if ul, e = net.ListenPacket("udp", addr); e != nil {
				tl.Close()
				last = e
				continue
			}
		}
		time.AfterFunc(d, func() {
			tl.Close()
			if ul != nil {
				ul.Close()
			}
		})
		go serveGreetings(tl, greeting)
		if ul != nil {
			go serveEchoes(ul, greeting)
		}
		return p, nil
	}
	if last == nil {
		last = errors.New("no port to listen on")
	}
	return 0, last
}

// A listener lives for a minute or two; these bound what anyone else who
// finds it can make it do.
const (
	maxGreetings   = 256
	echoesPerPeer  = 20
	greetingWindow = 3 * time.Second
)

func serveGreetings(l net.Listener, greeting []byte) {
	busy := make(chan struct{}, 16)
	for served := 0; served < maxGreetings; served++ {
		conn, e := l.Accept()
		if e != nil {
			return
		}
		busy <- struct{}{}
		go func() {
			defer func() { <-busy }()
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(greetingWindow))
			got := make([]byte, len(greeting))
			if _, e := io.ReadFull(conn, got); e == nil && bytes.Equal(got, greeting) {
				conn.Write(got)
			}
		}()
	}
	l.Close()
}

func serveEchoes(conn net.PacketConn, greeting []byte) {
	b := make([]byte, 128)
	limit := map[string]int{}
	window := time.Now()
	for {
		n, peer, e := conn.ReadFrom(b)
		if e != nil {
			if errors.Is(e, net.ErrClosed) {
				return
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if now := time.Now(); now.Sub(window) >= time.Second {
			window = now
			clear(limit)
		}
		host := peer.String()
		if a, ok := peer.(*net.UDPAddr); ok {
			host = a.IP.String()
		}
		limit[host]++
		if n == echoBytes && bytes.HasPrefix(b[:n], greeting) && limit[host] <= echoesPerPeer {
			conn.WriteTo(b[:n], peer)
		}
	}
}

// CheckDial measures one link (see CheckTarget).
func CheckDial(ctx context.Context, t c.CheckTarget) c.CheckLinkResult {
	out := c.CheckLinkResult{ID: t.ID}
	if t.URL != "" {
		out.TCP, out.ExitIP = measureInternet(ctx, t.URL)
		if t.UDPTarget != "" {
			out.UDP = measureUDP(ctx, t.UDPTarget, []byte(checkMagic))
		}
		return out
	}
	addr := net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
	var greeting []byte
	if t.Token != "" {
		token, e := hexToken(t.Token)
		if e != nil {
			out.TCP = &c.CheckMeasure{Error: e.Error()}
			return out
		}
		greeting = append([]byte(checkMagic), token...)
	}
	out.TCP = measureTCP(ctx, addr, greeting)
	if t.UDP && greeting != nil {
		out.UDP = measureUDP(ctx, addr, greeting)
	}
	return out
}

func hexToken(s string) ([]byte, error) {
	b, e := hex.DecodeString(s)
	if e != nil || len(b) != CheckTokenBytes {
		return nil, errors.New("invalid check token")
	}
	return b, nil
}

// measureTCP times the TCP handshake, which is one round trip, and with a
// greeting confirms the check listener answered it.
func measureTCP(ctx context.Context, addr string, greeting []byte) *c.CheckMeasure {
	samples := make([]float64, 0, checkTCPSamples)
	var last error
	for i := 0; i < checkTCPSamples && ctx.Err() == nil; i++ {
		rtt, e := tcpSample(ctx, addr, greeting)
		if e != nil {
			last = e
			// Nothing will change for the next sample of a refusal or a
			// timeout; three more would only take longer to say so.
			if len(samples) == 0 {
				break
			}
			continue
		}
		samples = append(samples, rtt)
	}
	return summarize(samples, checkTCPSamples, last, len(samples) > 0)
}

func tcpSample(ctx context.Context, addr string, greeting []byte) (float64, error) {
	d := net.Dialer{Timeout: 3 * time.Second}
	start := time.Now()
	conn, e := d.DialContext(ctx, "tcp", addr)
	if e != nil {
		return 0, e
	}
	rtt := float64(time.Since(start).Microseconds()) / 1000
	defer conn.Close()
	if greeting == nil {
		return rtt, nil
	}
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, e = conn.Write(greeting); e != nil {
		return 0, errNotOurs
	}
	back := make([]byte, len(greeting))
	if _, e = io.ReadFull(conn, back); e != nil || !bytes.Equal(back, greeting) {
		return 0, errNotOurs
	}
	return rtt, nil
}

// measureUDP sends echo requests one at a time, each given a second to
// return; prefix is the magic, with the check's token for a check listener.
func measureUDP(ctx context.Context, addr string, prefix []byte) *c.CheckMeasure {
	conn, e := net.DialTimeout("udp", addr, 3*time.Second)
	if e != nil {
		return &c.CheckMeasure{Samples: checkUDPSamples, Loss: 1, Error: Describe(e)}
	}
	defer conn.Close()
	samples := make([]float64, 0, checkUDPSamples)
	var last error
	for i := 0; i < checkUDPSamples && ctx.Err() == nil; i++ {
		packet := make([]byte, echoBytes)
		copy(packet, prefix)
		rand.Read(packet[len(prefix):])
		conn.SetDeadline(time.Now().Add(time.Second))
		start := time.Now()
		if _, e = conn.Write(packet); e != nil {
			last = e
			continue
		}
		back := make([]byte, 64)
		for {
			n, e := conn.Read(back)
			if e != nil {
				last = e
				break
			}
			// A late answer to an earlier request is not this one.
			if n == echoBytes && bytes.Equal(back[:n], packet) {
				samples = append(samples, float64(time.Since(start).Microseconds())/1000)
				break
			}
		}
	}
	// Games tolerate a lost datagram; most of them lost is a broken path.
	return summarize(samples, checkUDPSamples, last, len(samples) >= checkUDPSamples-1)
}

// measureInternet is the exit's own access: HTTP requests straight to the
// probe service, which also reports the address it saw them from.
func measureInternet(ctx context.Context, url string) (*c.CheckMeasure, string) {
	transport := &http.Transport{DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, MaxIdleConnsPerHost: 1}
	defer transport.CloseIdleConnections()
	ip, rtts, e := timedRequests(ctx, transport, url)
	if e != nil {
		return &c.CheckMeasure{Samples: tcpSamples, Loss: 1, Error: Describe(e)}, ip
	}
	return summarize(rtts, tcpSamples, nil, true), ip
}

func summarize(samples []float64, total int, last error, ok bool) *c.CheckMeasure {
	m := &c.CheckMeasure{OK: ok, Samples: total, Loss: float64(total-len(samples)) / float64(total)}
	if len(samples) > 0 {
		sort.Float64s(samples)
		m.RTTMS = samples[len(samples)/2]
		m.MinMS = samples[0]
		m.MaxMS = samples[len(samples)-1]
	}
	if last != nil && (!ok || len(samples) < total) {
		m.Error = Describe(last)
	}
	return m
}

// Describe says in a sentence what a failed connection most likely means.
func Describe(e error) string {
	var dns *net.DNSError
	var timeout interface{ Timeout() bool }
	switch {
	case errors.Is(e, errNotOurs):
		return "端口有回应，但不是这次检测的监听程序：端口可能被其他程序占用，或被转发到了别处"
	case errors.As(e, &dns):
		return "域名解析失败：" + dns.Name
	case isAny(e, errRefused):
		return "连接被拒绝：对方主机可达，但这个端口没有程序在监听"
	case isAny(e, errUnreachable):
		return "网络不可达：本机没有到对方的路由，例如对方地址是 IPv4 而本机只有 IPv6（或反过来）"
	case errors.Is(e, context.DeadlineExceeded) || (errors.As(e, &timeout) && timeout.Timeout()):
		return "没有回应：常见原因是防火墙或云服务商的安全组没有放行这个端口"
	}
	return e.Error()
}

func isAny(e error, targets []error) bool {
	for _, t := range targets {
		if errors.Is(e, t) {
			return true
		}
	}
	return false
}

// Measure runs targets side by side and returns their results in order.
func Measure(ctx context.Context, targets []c.CheckTarget) []c.CheckLinkResult {
	out := make([]c.CheckLinkResult, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = CheckDial(ctx, t)
		}()
	}
	wg.Wait()
	return out
}
