package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"github.com/mbfczzz/FlowLight-Accelerator/packages/network"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var safeID = regexp.MustCompile(`^[a-f0-9]{32}$`)

type Binaries struct {
	SSserver string            `json:"ssserver"`
	SSlocal  string            `json:"sslocal"`
	Realm    string            `json:"realm"`
	Hashes   map[string]string `json:"sha256"`
}
type Process struct {
	Runtime c.Runtime `json:"runtime"`
	PID     int       `json:"pid"`
	Binary  string    `json:"binary"`
	Config  string    `json:"config"`
	cmd     *exec.Cmd
	done    chan struct{}
}
type Manager struct {
	sync.Mutex
	Dir       string
	Binaries  Binaries
	Processes map[string]*Process
	pending   []c.Runtime
}

func NewManager(dir string, b Binaries) (*Manager, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	m := &Manager{Dir: dir, Binaries: b, Processes: map[string]*Process{}}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return nil, e
	}
	for _, ent := range entries {
		if filepath.Ext(ent.Name()) != ".runtime" {
			continue
		}
		var p Process
		data, e := os.ReadFile(filepath.Join(dir, ent.Name()))
		if e != nil {
			return nil, e
		}
		if json.Unmarshal(data, &p) != nil || !safeID.MatchString(p.Runtime.ID) {
			return nil, errors.New("invalid owned runtime journal")
		}
		if ownedProcess(p.PID, p.Config) {
			if proc, e := os.FindProcess(p.PID); e == nil {
				_ = proc.Kill()
			}
		}
		// Reconcile against the control service before restarting a persisted
		// runtime. A stale local wall clock must not revive an expired grant.
		m.pending = append(m.pending, p.Runtime)
	}
	return m, nil
}
func (m *Manager) binary(kind string) (string, error) {
	path := ""
	switch kind {
	case "ss2022":
		path = m.Binaries.SSserver
	case "realm":
		path = m.Binaries.Realm
	case "sslocal":
		path = m.Binaries.SSlocal
	}
	want := m.Binaries.Hashes[kind]
	if path == "" || len(want) != 64 {
		return "", fmt.Errorf("configure pinned %s binary and sha256", kind)
	}
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return "", errors.New("network binary checksum mismatch")
	}
	return filepath.Abs(path)
}
func (m *Manager) Prepare(rt c.Runtime) error {
	m.Lock()
	defer m.Unlock()
	if !safeID.MatchString(rt.ID) || !safeID.MatchString(rt.SessionID) || rt.ListenPort < 1024 || rt.ListenPort > 65535 || time.Now().After(rt.ExpiresAt) {
		return errors.New("invalid or expired runtime")
	}
	if old := m.Processes[rt.ID]; old != nil {
		if old.Runtime.SessionID != rt.SessionID || old.Runtime.ListenPort != rt.ListenPort {
			return errors.New("runtime identifier conflict")
		}
		return nil
	}
	binary, e := m.binary(rt.Kind)
	if e != nil {
		return e
	}
	path := filepath.Join(m.Dir, rt.ID+".json")
	mode := "tcp_only"
	if rt.UDP {
		mode = "tcp_and_udp"
	}
	var data []byte
	if rt.Kind == "ss2022" {
		data, e = json.Marshal(map[string]any{"server": "0.0.0.0", "server_port": rt.ListenPort, "password": rt.Password, "method": rt.Cipher, "mode": mode, "timeout": 60})
	} else if rt.Kind == "realm" {
		if rt.RemoteHost == "" || rt.RemotePort < 1 || rt.RemotePort > 65535 {
			return errors.New("invalid forwarding destination")
		}
		data, e = json.Marshal(map[string]any{"network": map[string]any{"no_tcp": false, "use_udp": rt.UDP}, "endpoints": []map[string]string{{"listen": net.JoinHostPort("0.0.0.0", fmt.Sprint(rt.ListenPort)), "remote": net.JoinHostPort(rt.RemoteHost, fmt.Sprint(rt.RemotePort))}}})
	} else {
		return errors.New("unsupported runtime kind")
	}
	if e != nil {
		return e
	}
	if e = atomicWrite(path, data); e != nil {
		return e
	}
	cmd := exec.Command(binary, "-c", path)
	constrainChild(cmd)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if e = cmd.Start(); e != nil {
		return e
	}
	p := &Process{Runtime: rt, PID: cmd.Process.Pid, Binary: binary, Config: path, cmd: cmd, done: make(chan struct{})}
	m.Processes[rt.ID] = p
	record, _ := json.Marshal(p)
	if e = atomicWrite(filepath.Join(m.Dir, rt.ID+".runtime"), record); e != nil {
		cmd.Process.Kill()
		delete(m.Processes, rt.ID)
		return e
	}
	go func() {
		_ = cmd.Wait()
		close(p.done)
		m.Lock()
		defer m.Unlock()
		if m.Processes[rt.ID] == p {
			delete(m.Processes, rt.ID)
		}
	}()
	return nil
}
func (m *Manager) Stop(id string) error {
	m.Lock()
	defer m.Unlock()
	if !safeID.MatchString(id) {
		return errors.New("invalid runtime id")
	}
	if p := m.Processes[id]; p != nil {
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
			select {
			case <-p.done:
			case <-time.After(5 * time.Second):
				return errors.New("runtime did not stop")
			}
		}
		delete(m.Processes, id)
	}
	for _, suffix := range []string{".json", ".runtime"} {
		e := os.Remove(filepath.Join(m.Dir, id+suffix))
		if e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	return nil
}
func (m *Manager) Renew(leases map[string]time.Time) error {
	m.Lock()
	pending := m.pending
	m.pending = nil
	for _, p := range m.Processes {
		expiry, ok := leases[p.Runtime.SessionID]
		if !ok {
			expiry = time.Now().Add(-time.Second)
		}
		p.Runtime.ExpiresAt = expiry
		b, _ := json.Marshal(p)
		if e := atomicWrite(filepath.Join(m.Dir, p.Runtime.ID+".runtime"), b); e != nil {
			m.Unlock()
			return e
		}
	}
	m.Unlock()
	for _, rt := range pending {
		expiry, ok := leases[rt.SessionID]
		if !ok || time.Now().After(expiry) {
			if e := m.Stop(rt.ID); e != nil {
				return e
			}
			continue
		}
		rt.ExpiresAt = expiry
		if e := m.Prepare(rt); e != nil {
			return e
		}
	}
	return nil
}
func (m *Manager) Expire() {
	m.Lock()
	ids := []string{}
	for id, p := range m.Processes {
		if time.Now().After(p.Runtime.ExpiresAt) {
			ids = append(ids, id)
		}
	}
	m.Unlock()
	for _, id := range ids {
		_ = m.Stop(id)
	}
}
func (m *Manager) Probe(ctx context.Context, conn c.Connection) (c.ProbeResult, error) {
	binary, e := m.binary("sslocal")
	if e != nil {
		return c.ProbeResult{}, e
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return c.ProbeResult{}, e
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	path := filepath.Join(m.Dir, fmt.Sprintf("probe-%d.json", port))
	mode := "tcp_only"
	if conn.UDP {
		mode = "tcp_and_udp"
	}
	b, _ := json.Marshal(map[string]any{"server": conn.Host, "server_port": conn.Port, "password": conn.Password, "method": conn.Cipher, "local_address": "127.0.0.1", "local_port": port, "mode": mode})
	if e = atomicWrite(path, b); e != nil {
		return c.ProbeResult{}, e
	}
	defer os.Remove(path)
	cmd := exec.CommandContext(ctx, binary, "-c", path)
	constrainChild(cmd)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if e = cmd.Start(); e != nil {
		return c.ProbeResult{}, e
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	socks := fmt.Sprintf("127.0.0.1:%d", port)
	ready := false
	for i := 0; i < 30; i++ {
		connection, e := net.DialTimeout("tcp", socks, 100*time.Millisecond)
		if e == nil {
			connection.Close()
			ready = true
			break
		}
		select {
		case <-ctx.Done():
			return c.ProbeResult{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !ready {
		return c.ProbeResult{}, errors.New("SS local listener did not become ready")
	}
	udp := ""
	if conn.UDP {
		udp = conn.ProbeUDP
	}
	return network.Probe(ctx, socks, conn.ProbeURL, udp)
}
func atomicWrite(path string, b []byte) error {
	tmp := path + ".tmp"
	if e := os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}

func (m *Manager) ActualVersions() []string {
	m.Lock()
	defer m.Unlock()
	seen := map[string]bool{}
	out := []string{}
	for _, p := range m.Processes {
		if p.Runtime.VersionID != "" && !seen[p.Runtime.VersionID] {
			seen[p.Runtime.VersionID] = true
			out = append(out, p.Runtime.VersionID)
		}
	}
	return out
}
