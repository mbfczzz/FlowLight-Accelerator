package agent

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	contract "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"github.com/mbfczzz/FlowLight-Accelerator/packages/network"
)

// Line connectivity checks (internal/control/check.go): a test listener on
// a port of this node's endpoint, or measurements from this node to the
// next one on the path and to the internet.

type testListener struct {
	token string
	stop  func()
}

// testPorts are this agent's open test listeners by port.
var testPorts = struct {
	sync.Mutex
	open map[int]testListener
}{open: map[int]testListener{}}

// releaseTestPort closes this agent's test listener on port, if it has one,
// and if token is given only when it is that check's. A session about to
// listen there comes first, and so does a new check that chose that very
// port while the last one's listener is still open; a finished check closes
// its own.
func releaseTestPort(port int, token string) {
	testPorts.Lock()
	l, found := testPorts.open[port]
	if found && (token == "" || l.token == token) {
		delete(testPorts.open, port)
	} else {
		found = false
	}
	testPorts.Unlock()
	if found {
		l.stop()
	}
}

func checkListen(t contract.CheckTask) (*contract.CheckReport, error) {
	token, e := hex.DecodeString(t.Token)
	if e != nil || len(t.Ports) == 0 || len(t.Ports) > 16 {
		return nil, errors.New("invalid check task")
	}
	// Automatic choices skip ports already in use, another check's
	// included; a port the admin chose is taken over.
	if len(t.Ports) == 1 {
		releaseTestPort(t.Ports[0], "")
	}
	seconds := min(max(t.Seconds, 10), 120)
	port, stop, e := network.CheckListen(serviceHost, t.Ports, token, t.UDP, time.Duration(seconds)*time.Second)
	if e != nil {
		return nil, e
	}
	testPorts.Lock()
	testPorts.open[port] = testListener{token: t.Token, stop: stop}
	testPorts.Unlock()
	return &contract.CheckReport{Port: port}, nil
}

// checkRelease closes a finished check's listener. One already closed, or
// since taken over by a session or a newer check, is left as it is.
func checkRelease(t contract.CheckTask) error {
	if len(t.Ports) != 1 || t.Token == "" {
		return errors.New("invalid check task")
	}
	releaseTestPort(t.Ports[0], t.Token)
	return nil
}

func checkDial(ctx context.Context, t contract.CheckTask) (*contract.CheckReport, error) {
	if len(t.Targets) == 0 || len(t.Targets) > 4 {
		return nil, errors.New("invalid check task")
	}
	return &contract.CheckReport{Links: network.Measure(ctx, t.Targets)}, nil
}
