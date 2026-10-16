package agent

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/mbfczzz/FlowLight-Accelerator/packages/network"
)

// The latency echo (network.ServeEcho) for sessions that leave the line at
// this node. It listens on loopback only, where this node's SS server
// reaches it for a tunnel and nothing outside can, on one port for the
// whole node. The port is kept across restarts, so sessions that outlive
// the agent still find it.

type echoService struct {
	sync.Mutex
	port     int
	listener net.Listener
}

const echoFile = "echo.port"

// Echo returns the port of this node's latency echo, starting it if it is
// not running: on the port it had before when that is still free, or else
// on a new one.
func (m *Manager) Echo() (int, error) {
	m.echo.Lock()
	defer m.echo.Unlock()
	if m.echo.listener != nil {
		return m.echo.port, nil
	}
	if b, e := os.ReadFile(filepath.Join(m.Dir, echoFile)); e == nil {
		if port, e := strconv.Atoi(strings.TrimSpace(string(b))); e == nil && port > 0 && port < 65536 {
			if l, e := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port))); e == nil {
				m.serveEcho(l)
				return m.echo.port, nil
			}
		}
	}
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return 0, e
	}
	m.serveEcho(l)
	return m.echo.port, m.saveEcho()
}

// echoGiveWay moves the echo off port, which a runtime is about to listen
// on. Sessions given the old port lose their echo and report the round
// trip to the probe service instead.
func (m *Manager) echoGiveWay(port int) {
	m.echo.Lock()
	defer m.echo.Unlock()
	if m.echo.listener == nil || m.echo.port != port {
		return
	}
	// The new listener opens before the old one closes, so it cannot be
	// given the same port.
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	m.echo.listener.Close()
	m.echo.listener = nil
	if e != nil {
		log.Printf("latency echo stopped: %v", e)
		return
	}
	m.serveEcho(l)
	if e = m.saveEcho(); e != nil {
		log.Printf("latency echo port not saved: %v", e)
	}
}

func (m *Manager) closeEcho() {
	m.echo.Lock()
	defer m.echo.Unlock()
	if m.echo.listener != nil {
		m.echo.listener.Close()
		m.echo.listener = nil
	}
}

// serveEcho and saveEcho run with m.echo locked.
func (m *Manager) serveEcho(l net.Listener) {
	m.echo.listener = l
	m.echo.port = l.Addr().(*net.TCPAddr).Port
	go network.ServeEcho(l)
}
func (m *Manager) saveEcho() error {
	return atomicWrite(filepath.Join(m.Dir, echoFile), []byte(fmt.Sprintln(m.echo.port)))
}
