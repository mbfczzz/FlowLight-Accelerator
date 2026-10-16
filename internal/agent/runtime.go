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
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var safeID = regexp.MustCompile(`^[a-f0-9]{32}$`)

// A runtime that exits on its own is restarted this many times, with
// exponential backoff from restartDelay, before it is reported to control.
const maxRestarts = 3

var restartDelay = time.Second

type Binaries struct {
	SSserver string            `json:"ssserver"`
	SSlocal  string            `json:"sslocal"`
	Realm    string            `json:"realm"`
	Hashes   map[string]string `json:"sha256"`
}
type Process struct {
	Runtime  c.Runtime `json:"runtime"`
	PID      int       `json:"pid"`
	Binary   string    `json:"binary"`
	Config   string    `json:"config"`
	cmd      *exec.Cmd
	done     chan struct{}
	stopping bool
	started  time.Time
	restarts int
	output   *tail
	// Counted is the runtime's traffic so far (see usage.go): base from
	// earlier processes plus current from this one.
	Counted int64 `json:"counted,omitempty"`
	base    int64
	current int64
	// When Counted was last written to the journal (throttled, usage.go).
	persisted time.Time
}
type Manager struct {
	sync.Mutex
	Dir       string
	Binaries  Binaries
	Processes map[string]*Process
	pending   []c.Runtime
	failed    []c.Runtime
	timers    map[string]*time.Timer
	verified  sync.Map // binary path -> fileStamp of the last matching hash
	// Traffic accounting (usage.go): where ssserver reports, totals carried
	// into a runtime's next process, and totals of stopped runtimes not
	// reported yet.
	statsAddr string
	carry     map[string]Usage
	finished  map[string]Usage
}

// fileStamp identifies a file version cheaply. A binary is re-hashed only
// when it changes instead of on every prepare and probe.
type fileStamp struct {
	size    int64
	modTime time.Time
}

func NewManager(dir string, b Binaries) (*Manager, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	m := &Manager{Dir: dir, Binaries: b, Processes: map[string]*Process{}, timers: map[string]*time.Timer{}, carry: map[string]Usage{}, finished: map[string]Usage{}}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return nil, e
	}
	journals := map[string]bool{}
	for _, ent := range entries {
		if filepath.Ext(ent.Name()) == ".runtime" {
			journals[strings.TrimSuffix(ent.Name(), ".runtime")] = true
		}
	}
	for _, ent := range entries {
		// Probe configs, interrupted writes and configs whose journal is gone
		// hold keys nothing will use again.
		name := ent.Name()
		stale := strings.HasPrefix(name, "probe-") || filepath.Ext(name) == ".tmp" ||
			filepath.Ext(name) == ".json" && !journals[strings.TrimSuffix(name, ".json")]
		if stale {
			if e := os.Remove(filepath.Join(dir, name)); e != nil && !os.IsNotExist(e) {
				log.Printf("removing stale %s: %v", name, e)
			}
		}
	}
	for _, ent := range entries {
		if filepath.Ext(ent.Name()) != ".runtime" {
			continue
		}
		path := filepath.Join(dir, ent.Name())
		var p Process
		data, e := os.ReadFile(path)
		if e != nil {
			return nil, e
		}
		if json.Unmarshal(data, &p) != nil || !safeID.MatchString(p.Runtime.ID) {
			// A torn write must not keep the whole node offline. The grant it
			// described expires with its lease on the control side.
			log.Printf("quarantining unreadable runtime journal %s", ent.Name())
			if e := os.Rename(path, path+".corrupt"); e != nil {
				return nil, e
			}
			continue
		}
		if ownedProcess(p.PID, p.Config) {
			if proc, e := os.FindProcess(p.PID); e == nil {
				_ = proc.Kill()
			}
		}
		// Reconcile against the control service before restarting a persisted
		// runtime. A stale local wall clock must not revive an expired grant.
		m.pending = append(m.pending, p.Runtime)
		if p.Counted > 0 {
			m.carry[p.Runtime.ID] = Usage{p.Runtime.ID, p.Runtime.SessionID, p.Counted}
			m.finish(p.Runtime.ID, p.Runtime.SessionID, p.Counted)
		}
	}
	m.loadPending()
	m.listenStats()
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
	if e := binaryIsTrusted(path); e != nil {
		return "", e
	}
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return "", e
	}
	stamp := fileStamp{info.Size(), info.ModTime()}
	if seen, ok := m.verified.Load(path); !ok || seen.(fileStamp) != stamp {
		h := sha256.New()
		if _, e = io.Copy(h, f); e != nil {
			return "", e
		}
		if hex.EncodeToString(h.Sum(nil)) != want {
			m.verified.Delete(path)
			return "", errors.New("network binary checksum mismatch")
		}
		m.verified.Store(path, stamp)
	}
	return filepath.Abs(path)
}

// Prepare starts a runtime and reports success only once it accepts
// connections, so a process that dies on a bad key or a busy port fails the
// prepare task instead of the later probe.
func (m *Manager) Prepare(rt c.Runtime) error {
	p, e := m.start(rt, 0)
	if e != nil || p == nil {
		return e
	}
	if e = m.ready(p); e != nil {
		_ = m.Stop(rt.ID)
		return e
	}
	return nil
}
func (m *Manager) start(rt c.Runtime, restarts int) (*Process, error) {
	m.Lock()
	defer m.Unlock()
	if !safeID.MatchString(rt.ID) || !safeID.MatchString(rt.SessionID) || rt.ListenPort < 1024 || rt.ListenPort > 65535 || time.Now().After(rt.ExpiresAt) {
		return nil, errors.New("invalid or expired runtime")
	}
	if old := m.Processes[rt.ID]; old != nil {
		if old.Runtime.SessionID != rt.SessionID || old.Runtime.ListenPort != rt.ListenPort {
			return nil, errors.New("runtime identifier conflict")
		}
		return nil, nil
	}
	if t := m.timers[rt.ID]; t != nil {
		t.Stop()
		delete(m.timers, rt.ID)
	}
	binary, e := m.binary(rt.Kind)
	if e != nil {
		return nil, e
	}
	path := filepath.Join(m.Dir, rt.ID+".json")
	mode := "tcp_only"
	if rt.UDP {
		mode = "tcp_and_udp"
	}
	var data []byte
	if rt.Kind == "ss2022" {
		data, e = json.Marshal(map[string]any{"server": serviceHost, "server_port": rt.ListenPort, "password": rt.Password, "method": rt.Cipher, "mode": mode, "timeout": 60})
	} else if rt.Kind == "realm" {
		if rt.RemoteHost == "" || rt.RemotePort < 1 || rt.RemotePort > 65535 {
			return nil, errors.New("invalid forwarding destination")
		}
		data, e = json.Marshal(map[string]any{"network": map[string]any{"no_tcp": false, "use_udp": rt.UDP}, "endpoints": []map[string]string{{"listen": net.JoinHostPort(serviceHost, fmt.Sprint(rt.ListenPort)), "remote": net.JoinHostPort(rt.RemoteHost, fmt.Sprint(rt.RemotePort))}}})
	} else {
		return nil, errors.New("unsupported runtime kind")
	}
	if e != nil {
		return nil, e
	}
	if e = atomicWrite(path, data); e != nil {
		return nil, e
	}
	if !portAvailable(rt.ListenPort, rt.UDP) {
		return nil, fmt.Errorf("port %d is already in use", rt.ListenPort)
	}
	output := &tail{}
	args := []string{"-c", path}
	if rt.Kind == "ss2022" && m.statsAddr != "" {
		// Before -c: the config path stays the last argument.
		args = append([]string{"--manager-addr", m.statsAddr}, args...)
	}
	cmd := exec.Command(binary, args...)
	constrainChild(cmd)
	cmd.Stdout = output
	cmd.Stderr = output
	if e = cmd.Start(); e != nil {
		return nil, e
	}
	base := m.carry[rt.ID].Bytes
	p := &Process{Runtime: rt, PID: cmd.Process.Pid, Binary: binary, Config: path, cmd: cmd, done: make(chan struct{}), started: time.Now(), restarts: restarts, output: output, Counted: base, base: base}
	m.Processes[rt.ID] = p
	record, _ := json.Marshal(p)
	if e = atomicWrite(filepath.Join(m.Dir, rt.ID+".runtime"), record); e != nil {
		p.stopping = true
		cmd.Process.Kill()
		delete(m.Processes, rt.ID)
		return nil, e
	}
	// Only now does the new process own the carried total.
	delete(m.carry, rt.ID)
	p.persisted = time.Now()
	go func() {
		err := cmd.Wait()
		close(p.done)
		m.exited(p, err)
	}()
	return p, nil
}

// ready waits until the runtime accepts a connection and stays up for a
// moment, so a process that loses the port race is not reported as ready.
func (m *Manager) ready(p *Process) error {
	address := net.JoinHostPort("127.0.0.1", fmt.Sprint(p.Runtime.ListenPort))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-p.done:
			return fmt.Errorf("%s runtime exited during start: %s", p.Runtime.Kind, p.output)
		default:
		}
		if conn, e := net.DialTimeout("tcp", address, 200*time.Millisecond); e == nil {
			conn.Close()
			// The port must be ours: another process holding it would also
			// accept this connection, and our child would have exited.
			select {
			case <-p.done:
				return fmt.Errorf("%s runtime exited during start: %s", p.Runtime.Kind, p.output)
			case <-time.After(300 * time.Millisecond):
			}
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("%s runtime did not listen on port %d", p.Runtime.Kind, p.Runtime.ListenPort)
}

// exited handles a runtime process that ended. Unless it was stopped on
// purpose it is restarted with backoff; once restarts are exhausted it is
// queued for a runtime_failed report so control stops renewing the session.
func (m *Manager) exited(p *Process, err error) {
	m.Lock()
	defer m.Unlock()
	if m.Processes[p.Runtime.ID] != p {
		return
	}
	delete(m.Processes, p.Runtime.ID)
	if p.stopping {
		return
	}
	log.Printf("runtime %s (%s, session %s) exited unexpectedly: %v: %s", p.Runtime.ID, p.Runtime.Kind, p.Runtime.SessionID, err, p.output)
	restarts := p.restarts
	if time.Since(p.started) > time.Minute {
		restarts = 0
	}
	rt := p.Runtime
	if restarts >= maxRestarts || !time.Now().Before(rt.ExpiresAt) {
		m.failed = append(m.failed, rt)
		m.finish(rt.ID, rt.SessionID, p.Counted)
		return
	}
	// The next process counts from zero; its total continues from here.
	m.carry[rt.ID] = Usage{rt.ID, rt.SessionID, p.Counted}
	var timer *time.Timer
	timer = time.AfterFunc(restartDelay<<restarts, func() {
		m.Lock()
		current := m.timers[rt.ID] == timer
		if current {
			delete(m.timers, rt.ID)
		}
		m.Unlock()
		if !current {
			return
		}
		if _, e := m.start(rt, restarts+1); e != nil {
			log.Printf("runtime %s restart failed: %v", rt.ID, e)
			m.Lock()
			m.failed = append(m.failed, rt)
			if c, ok := m.carry[rt.ID]; ok {
				m.finish(rt.ID, c.SessionID, c.Bytes)
				delete(m.carry, rt.ID)
			}
			m.Unlock()
		}
	})
	m.timers[rt.ID] = timer
}

// TakeFailures returns runtimes that could not be kept running. ReturnFailures
// puts back any that could not be reported yet.
func (m *Manager) TakeFailures() []c.Runtime {
	m.Lock()
	defer m.Unlock()
	out := m.failed
	m.failed = nil
	return out
}
func (m *Manager) ReturnFailures(rts []c.Runtime) {
	m.Lock()
	defer m.Unlock()
	// Once the lease has lapsed, control stops the session on its own.
	for _, rt := range rts {
		if time.Now().Before(rt.ExpiresAt) {
			m.failed = append(m.failed, rt)
		}
	}
}
func (m *Manager) Stop(id string) error {
	if !safeID.MatchString(id) {
		return errors.New("invalid runtime id")
	}
	m.Lock()
	if t := m.timers[id]; t != nil {
		t.Stop()
		delete(m.timers, id)
	}
	p := m.Processes[id]
	if p != nil {
		p.stopping = true
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
	}
	m.Unlock()
	if p != nil {
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
			return errors.New("runtime did not stop")
		}
		m.Lock()
		if m.Processes[id] == p {
			delete(m.Processes, id)
		}
		m.finish(id, p.Runtime.SessionID, p.Counted)
		m.Unlock()
	}
	// Stopped between processes (restart backoff, or never resumed after an
	// agent restart): the carried total is all there is.
	m.Lock()
	if c, ok := m.carry[id]; ok {
		m.finish(id, c.SessionID, c.Bytes)
		delete(m.carry, id)
	}
	m.Unlock()
	for _, suffix := range []string{".json", ".runtime"} {
		e := os.Remove(filepath.Join(m.Dir, id+suffix))
		if e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	return nil
}

// Shutdown stops every runtime for an agent restart but keeps the journals,
// so the next start re-prepares grants whose leases control still renews.
func (m *Manager) Shutdown() {
	m.Lock()
	for id, t := range m.timers {
		t.Stop()
		delete(m.timers, id)
	}
	running := []*Process{}
	for _, p := range m.Processes {
		if record, e := json.Marshal(p); e == nil {
			_ = atomicWrite(filepath.Join(m.Dir, p.Runtime.ID+".runtime"), record)
		}
		p.stopping = true
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		running = append(running, p)
	}
	m.Unlock()
	for _, p := range running {
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
		}
	}
}

// Renew applies an authoritative lease snapshot requested at sentAt. A
// runtime started after that request may be missing only because the snapshot
// predates its session, so it is left to the next heartbeat.
func (m *Manager) Renew(leases map[string]time.Time, sentAt time.Time) error {
	m.Lock()
	pending := m.pending
	m.pending = nil
	for _, p := range m.Processes {
		expiry, ok := leases[p.Runtime.SessionID]
		if !ok {
			if p.started.After(sentAt) {
				continue
			}
			expiry = time.Now().Add(-time.Second)
		}
		if p.Runtime.ExpiresAt.Equal(expiry) {
			continue
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
			log.Printf("runtime %s could not resume: %v", rt.ID, e)
			m.Lock()
			m.failed = append(m.failed, rt)
			m.Unlock()
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

// atomicWrite flushes the file before the rename and the directory after it,
// so a power loss leaves either the old journal or the new one, never a torn
// one. Directory sync is unsupported on Windows and is best effort.
func atomicWrite(path string, b []byte) error {
	tmp := path + ".tmp"
	f, e := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if e != nil {
		return e
	}
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e != nil {
		os.Remove(tmp)
		return e
	}
	if e = os.Rename(tmp, path); e != nil {
		return e
	}
	if d, e := os.Open(filepath.Dir(path)); e == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
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

// serviceHost is the address session services listen on: IPv4 only. The
// connectivity check's test listener uses it too, so a path it finds open is
// one a session can use.
const serviceHost = "0.0.0.0"

// portAvailable reports whether both the TCP and, when required, the UDP
// listener can be bound, so a conflict fails the prepare task instead of
// surfacing later as a probe failure.
func portAvailable(port int, udp bool) bool {
	tcp, e := net.Listen("tcp", net.JoinHostPort(serviceHost, fmt.Sprint(port)))
	if e != nil {
		return false
	}
	tcp.Close()
	if !udp {
		return true
	}
	packet, e := net.ListenPacket("udp", net.JoinHostPort(serviceHost, fmt.Sprint(port)))
	if e != nil {
		return false
	}
	packet.Close()
	return true
}

// tail keeps the last few kilobytes a child process printed, for diagnostics.
type tail struct {
	sync.Mutex
	b []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.Lock()
	defer t.Unlock()
	t.b = append(t.b, p...)
	if len(t.b) > 2048 {
		t.b = t.b[len(t.b)-2048:]
	}
	return len(p), nil
}
func (t *tail) String() string {
	t.Lock()
	defer t.Unlock()
	return strings.TrimSpace(string(t.b))
}
