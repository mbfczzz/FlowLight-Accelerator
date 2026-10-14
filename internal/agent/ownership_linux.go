//go:build linux

package agent

import (
	"bytes"
	"fmt"
	"os"
)

func ownedProcess(pid int, config string) bool {
	if pid < 2 {
		return false
	}
	b, e := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	return e == nil && bytes.Contains(b, []byte(config))
}
