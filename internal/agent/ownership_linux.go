//go:build linux

package agent

import (
	"bytes"
	"fmt"
	"os"
)

// binaryIsTrusted refuses group- or world-writable executables: the checksum
// proves what was read, not what will be executed.
func binaryIsTrusted(path string) error {
	info, e := os.Stat(path)
	if e != nil {
		return e
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%s is group- or world-writable", path)
	}
	return nil
}

func ownedProcess(pid int, config string) bool {
	if pid < 2 {
		return false
	}
	b, e := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	return e == nil && bytes.Contains(b, []byte(config))
}
