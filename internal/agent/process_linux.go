//go:build linux

package agent

import (
	"os/exec"
	"syscall"
)

func constrainChild(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
