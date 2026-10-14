//go:build !linux

package agent

func binaryIsTrusted(path string) error { return nil }

func ownedProcess(pid int, config string) bool { return false }
