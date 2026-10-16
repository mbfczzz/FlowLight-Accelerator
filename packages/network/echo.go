package network

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"sort"
	"time"

	"golang.org/x/net/proxy"
)

// The latency echo: an exit node answers on its loopback, its SS server
// dials it for a tunnel, and a client times frames bounced off it, so the
// round trip covers the line up to the exit and not the leg on to the probe
// service, which game traffic never takes. A frame is the probe server's
// 28 bytes, FLA1 and 24 bytes the sender chose, and comes back unchanged.

const (
	echoSamples = 5
	// What one connection may ask of the echo.
	echoFrames    = 64
	echoLifetime  = 30 * time.Second
	echoParallel  = 32
	echoFrameWait = 5 * time.Second
)

// ServeEcho answers latency frames on l until l is closed.
func ServeEcho(l net.Listener) {
	busy := make(chan struct{}, echoParallel)
	for {
		conn, e := l.Accept()
		if e != nil {
			if errors.Is(e, net.ErrClosed) {
				return
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		select {
		case busy <- struct{}{}:
		default:
			conn.Close()
			continue
		}
		go func() {
			defer func() { <-busy }()
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(echoLifetime))
			b := make([]byte, echoBytes)
			for i := 0; i < echoFrames; i++ {
				if _, e := io.ReadFull(conn, b); e != nil || !bytes.Equal(b[:4], []byte(checkMagic)) {
					return
				}
				if _, e := conn.Write(b); e != nil {
					return
				}
			}
		}()
	}
}

// EchoRTT bounces frames off the echo at target through the SOCKS5 proxy
// at socks: one untimed, which opens the path through the line, then
// echoSamples timed over the same connection. It returns the sorted round
// trips in milliseconds.
func EchoRTT(ctx context.Context, socks, target string) ([]float64, error) {
	dialer, e := proxy.SOCKS5("tcp", socks, nil, &net.Dialer{Timeout: 5 * time.Second})
	if e != nil {
		return nil, e
	}
	conn, e := dialer.(proxy.ContextDialer).DialContext(ctx, "tcp", target)
	if e != nil {
		return nil, e
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.SetDeadline(time.Now()) })
	defer stop()
	frame := make([]byte, echoBytes)
	got := make([]byte, echoBytes)
	copy(frame, checkMagic)
	rtts := []float64{}
	for i := 0; i <= echoSamples; i++ {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		rand.Read(frame[4:])
		conn.SetDeadline(time.Now().Add(echoFrameWait))
		start := time.Now()
		if _, e = conn.Write(frame); e != nil {
			return nil, e
		}
		if _, e = io.ReadFull(conn, got); e != nil {
			return nil, e
		}
		if !bytes.Equal(got, frame) {
			return nil, errors.New("echo returned a different frame")
		}
		if i > 0 {
			rtts = append(rtts, float64(time.Since(start).Microseconds())/1000)
		}
	}
	sort.Float64s(rtts)
	return rtts, nil
}
