package agent

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	contract "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"github.com/mbfczzz/FlowLight-Accelerator/packages/network"
)

// Line connectivity checks (internal/control/check.go): a test listener on
// a port of this node's endpoint, or measurements from this node to the
// next one on the path and to the internet.

func checkListen(t contract.CheckTask) (*contract.CheckReport, error) {
	token, e := hex.DecodeString(t.Token)
	if e != nil || len(t.Ports) == 0 || len(t.Ports) > 16 {
		return nil, errors.New("invalid check task")
	}
	seconds := min(max(t.Seconds, 10), 120)
	port, e := network.CheckListen(serviceHost, t.Ports, token, t.UDP, time.Duration(seconds)*time.Second)
	if e != nil {
		return nil, e
	}
	return &contract.CheckReport{Port: port}, nil
}

func checkDial(ctx context.Context, t contract.CheckTask) (*contract.CheckReport, error) {
	if len(t.Targets) == 0 || len(t.Targets) > 4 {
		return nil, errors.New("invalid check task")
	}
	return &contract.CheckReport{Links: network.Measure(ctx, t.Targets)}, nil
}
