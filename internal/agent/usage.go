package agent

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Traffic accounting. Every exit runtime is its own ssserver on its own
// port. Started with --manager-addr it reports `stat: {"<port>": <bytes>}`
// every 10 seconds to a UDP socket this agent listens on: upload and
// download together, counted since that process started. The agent keeps
// a running total per runtime that survives the process being restarted
// (base) and the agent restarting (Counted, in the runtime journal), and
// reports the totals; control turns them into increments, so a report
// that is sent twice is not counted twice. Relays are not counted: every
// byte of a session passes its exit.

// A running total reaches the journal at most this often; every write is
// two fsyncs under the manager's lock. Shutdown writes the latest.
const persistEvery = time.Minute

// pendingFile keeps totals of stopped runtimes until control has them, so
// an agent restart does not lose them. Not .json: NewManager deletes .json
// files without a journal.
const pendingFile = "usage-pending.state"

// Usage is one runtime's running total.
type Usage struct {
	RuntimeID string `json:"runtime_id"`
	SessionID string `json:"session_id"`
	Bytes     int64  `json:"bytes"`
}

// listenStats opens the socket ssserver reports to. Without it runtimes
// still run, they are just not counted.
func (m *Manager) listenStats() {
	conn, e := net.ListenPacket("udp", "127.0.0.1:0")
	if e != nil {
		log.Printf("traffic statistics unavailable: %v", e)
		return
	}
	m.statsAddr = conn.LocalAddr().String()
	go func() {
		buf := make([]byte, 64<<10)
		for {
			n, _, e := conn.ReadFrom(buf)
			if e != nil {
				if errors.Is(e, net.ErrClosed) {
					return
				}
				continue
			}
			m.recordStat(buf[:n])
		}
	}()
}

// parseStat reads one manager datagram: `stat: {"20000": 1234}`.
func parseStat(b []byte) (map[int]int64, bool) {
	s := strings.TrimSpace(string(b))
	rest, ok := strings.CutPrefix(s, "stat:")
	if !ok {
		return nil, false
	}
	var raw map[string]int64
	if json.Unmarshal([]byte(strings.TrimSpace(rest)), &raw) != nil {
		return nil, false
	}
	out := map[int]int64{}
	for k, v := range raw {
		port, e := strconv.Atoi(k)
		if e != nil || port < 1 || port > 65535 || v < 0 {
			continue
		}
		out[port] = v
	}
	return out, len(out) > 0
}

func (m *Manager) recordStat(b []byte) {
	stats, ok := parseStat(b)
	if !ok {
		return
	}
	m.Lock()
	defer m.Unlock()
	for _, p := range m.Processes {
		v, ok := stats[p.Runtime.ListenPort]
		if !ok || p.Runtime.Kind != "ss2022" || v <= p.current {
			continue
		}
		p.current = v
		p.Counted = p.base + p.current
		// Persisted so an agent restart resumes from this total.
		if time.Since(p.persisted) >= persistEvery {
			if record, e := json.Marshal(p); e == nil && atomicWrite(filepath.Join(m.Dir, p.Runtime.ID+".runtime"), record) == nil {
				p.persisted = time.Now()
			}
		}
	}
}

// Usage lists the running totals of exit runtimes, including ones that
// stopped since the last successful report.
func (m *Manager) Usage() []Usage {
	m.Lock()
	defer m.Unlock()
	out := []Usage{}
	for _, p := range m.Processes {
		if p.Counted > 0 {
			out = append(out, Usage{p.Runtime.ID, p.Runtime.SessionID, p.Counted})
		}
	}
	for _, u := range m.finished {
		out = append(out, u)
	}
	return out
}

// UsageReported forgets stopped runtimes whose final total control has.
func (m *Manager) UsageReported(sent []Usage) {
	m.Lock()
	defer m.Unlock()
	changed := false
	for _, u := range sent {
		if f, ok := m.finished[u.RuntimeID]; ok && f.Bytes <= u.Bytes {
			delete(m.finished, u.RuntimeID)
			changed = true
		}
	}
	if changed {
		m.savePending()
	}
}

// finish keeps a stopped runtime's total until it has been reported.
// Called with the lock held.
func (m *Manager) finish(rt string, session string, counted int64) {
	if counted <= 0 || len(m.finished) >= 10000 {
		return
	}
	if old, ok := m.finished[rt]; ok && old.Bytes >= counted {
		return
	}
	m.finished[rt] = Usage{rt, session, counted}
	m.savePending()
}

// savePending writes the unreported totals of stopped runtimes. Called with
// the lock held.
func (m *Manager) savePending() {
	path := filepath.Join(m.Dir, pendingFile)
	if len(m.finished) == 0 {
		if e := os.Remove(path); e != nil && !os.IsNotExist(e) {
			log.Printf("removing %s: %v", pendingFile, e)
		}
		return
	}
	list := make([]Usage, 0, len(m.finished))
	for _, u := range m.finished {
		list = append(list, u)
	}
	if data, e := json.Marshal(list); e == nil {
		if e = atomicWrite(path, data); e != nil {
			log.Printf("saving %s: %v", pendingFile, e)
		}
	}
}

// loadPending restores totals saved by an earlier agent process.
func (m *Manager) loadPending() {
	data, e := os.ReadFile(filepath.Join(m.Dir, pendingFile))
	if e != nil {
		return
	}
	var list []Usage
	if json.Unmarshal(data, &list) != nil {
		return
	}
	for _, u := range list {
		if safeID.MatchString(u.RuntimeID) && safeID.MatchString(u.SessionID) {
			m.finish(u.RuntimeID, u.SessionID, u.Bytes)
		}
	}
}
