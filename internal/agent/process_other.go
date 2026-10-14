//go:build !linux

package agent

import "os/exec"

func constrainChild(cmd *exec.Cmd) {}
