//go:build !linux

package agent

func ownedProcess(pid int, config string) bool { return false }
