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

// testPorts are this agent's open test listeners by port.
var testPorts = struct {
	sync.Mutex
	stop map[int]func()
}{stop: map[int]func(){}}

// releaseTestPort closes this agent's test listener on port, if it has one.
// A session about to listen there comes first, and so does a new check that
// chose that very port while the last one's listener is still open.
func releaseTestPort(port int) {
	testPorts.Lock()
	stop := testPorts.stop[port]
	delete(testPorts.stop, port)
	testPorts.Unlock()
	if stop != nil {
		stop()
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
		releaseTestPort(t.Ports[0])
	}
	seconds := min(max(t.Seconds, 10), 120)
	port, stop, e := network.CheckListen(serviceHost, t.Ports, token, t.UDP, time.Duration(seconds)*time.Second)
	if e != nil {
		return nil, e
	}
	testPorts.Lock()
	testPorts.stop[port] = stop
	testPorts.Unlock()
	return &contract.CheckReport{Port: port}, nil
}

func checkDial(ctx context.Context, t contract.CheckTask) (*contract.CheckReport, error) {
	if len(t.Targets) == 0 || len(t.Targets) > 4 {
		return nil, errors.New("invalid check task")
	}
	return &contract.CheckReport{Links: network.Measure(ctx, t.Targets)}, nil
}
