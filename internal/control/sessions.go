package control

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"gorm.io/gorm"
	"net/http"
	"slices"
	"strings"
	"time"
)

type CreateSession struct {
	GameID    string `json:"game_id"`
	RegionID  string `json:"region_id"`
	LineID    string `json:"line_id"`
	DeviceID  string `json:"device_id"`
	BindingID string `json:"egress_binding_id"`
}

func (s *Server) job(tx *gorm.DB, node, session, version, kind string, p c.JobPayload) error {
	cipher, e := s.Store.Seal(p)
	if e != nil {
		return e
	}
	return tx.Create(&Job{ID: ID(), NodeID: node, SessionID: session, VersionID: version, Kind: kind, PayloadCipher: cipher, Status: "pending", CreatedAt: time.Now().UTC()}).Error
}
func (s *Server) provision(tx *gorm.DB, x *Session, v LineVersion, p c.Profile) error {
	hops := append([]c.Hop{}, v.Hops...)
	if len(hops) == 0 {
		return badRequest("线路发布内容为空，请重新发布这条线路")
	}
	now := time.Now().UTC()
	x.CreatedAt = now
	x.UpdatedAt = now
	x.State = "PREPARING"
	x.PrepareUntil = now.Add(s.Config.PrepareTimeout)
	// Clients only start renewing once the path is ready, so the first lease
	// must outlast preparation; otherwise PrepareTimeout could never apply.
	x.LeaseUntil = now.Add(max(s.Config.LeaseDuration, s.Config.PrepareTimeout))
	x.Enforcement = "node-lease"
	var password string
	if v.Hops[len(v.Hops)-1].Capabilities.Managed {
		n := 16
		if hops[len(hops)-1].Cipher == "2022-blake3-aes-256-gcm" {
			n = 32
		}
		key := make([]byte, n)
		if _, e := rand.Read(key); e != nil {
			return e
		}
		password = base64.StdEncoding.EncodeToString(key)
	} else {
		if v.Mode == "imported" {
			x.Enforcement = "external-credentials"
		}
		var cred Credential
		if e := tx.First(&cred, "id = ?", hops[len(hops)-1].CredentialID).Error; e != nil {
			return e
		}
		if e := s.Store.Unseal(cred.Ciphertext, &password); e != nil {
			return e
		}
	}
	reservations := []Reservation{}
	if v.Mode != "imported" {
		for i, h := range hops {
			if !h.Capabilities.Managed {
				reservations = append(reservations, Reservation{})
				continue
			}
			var node Node
			if tx.First(&node, "id = ?", h.NodeID).Error != nil || !node.Enabled || timeSince(node.LastSeen) > s.Config.NodeTimeout {
				return badRequest("线路节点离线，请稍后重试或更换线路")
			}
			var taken []int
			if e := tx.Model(&Reservation{}).Where("node_id = ? AND port BETWEEN ? AND ?", h.NodeID, h.Port, h.PortEnd).Pluck("port", &taken).Error; e != nil {
				return e
			}
			used := map[int]bool{}
			for _, p := range taken {
				used[p] = true
			}
			// The lowest free port, so a front forwarder or firewall that
			// lets only the start of the range through serves the first
			// sessions.
			allocated := false
			for port := h.Port; port <= h.PortEnd; port++ {
				if !used[port] {
					res := Reservation{ID: ID(), SessionID: x.ID, NodeID: h.NodeID, Port: port}
					if e := tx.Create(&res).Error; e != nil {
						return e
					}
					reservations = append(reservations, res)
					hops[i].Port = port
					allocated = true
					break
				}
			}
			if !allocated {
				// Every port on this hop is held by a live session. A line
				// whose capacity exceeds its port supply is refused when it is
				// compiled, so reaching here means the holders are sessions
				// that are still running: this clears as they end, which is
				// what the client is told to wait for. (It is still possible
				// for a line to be edited down to fewer ports after it was
				// published; the operator's capacity check catches that on the
				// next publish.)
				return retryLater("ports-busy", "线路端口正被其他会话占用，稍后会自动释放；若持续出现请联系运营方扩容")
			}
		}
	}
	connection := c.Connection{Host: hops[0].Host, Port: hops[0].Port, Cipher: hops[len(hops)-1].Cipher, Password: password, UDP: true, Profile: p, ProbeURL: s.Config.ProbeURL, ProbeUDP: s.Config.ProbeUDP, ExpectedExit: v.ExpectedExit}
	for _, h := range hops {
		connection.UDP = connection.UDP && h.Capabilities.UDP
	}
	if p.UDP && !connection.UDP {
		return badRequest("该游戏需要 UDP，但这条线路不支持，请更换线路")
	}
	var e error
	x.ConnectionCipher, e = s.Store.Seal(connection)
	if e != nil {
		return e
	}
	if e = tx.Create(x).Error; e != nil {
		return e
	}
	if v.Mode != "imported" {
		for i := len(hops) - 1; i >= 0; i-- {
			if !hops[i].Capabilities.Managed {
				continue
			}
			h := hops[i]
			runtime := c.Runtime{VersionID: v.ID, ID: reservations[i].ID, SessionID: x.ID, Kind: h.Kind, ListenPort: h.Port, UDP: connection.UDP, ExpiresAt: x.LeaseUntil}
			// Realm relays forward ciphertext and never need the session key.
			if h.Kind == "ss2022" {
				runtime.Cipher = h.Cipher
				runtime.Password = password
				// The exit's latency echo, so probes stop timing at the exit.
				runtime.Echo = i == len(hops)-1
			}
			if i+1 < len(hops) {
				runtime.RemoteHost = hops[i+1].Host
				runtime.RemotePort = hops[i+1].Port
			}
			if e = s.job(tx, h.NodeID, x.ID, v.ID, "prepare", c.JobPayload{Runtime: &runtime}); e != nil {
				return e
			}
		}
	} else {
		if e = s.job(tx, s.Config.ProbeNodeID, x.ID, v.ID, "probe", c.JobPayload{Connection: &connection}); e != nil {
			return e
		}
	}
	return audit(tx, x.UserID, "session.create", x.ID, x.Purpose)
}
func (s *Server) beginSession(w http.ResponseWriter, r *http.Request) {
	u := actor(r)
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 8 || len(key) > 128 {
		fail(w, 400, "请求缺少幂等标识，请更新客户端")
		return
	}
	var in CreateSession
	if !decode(w, r, &in) {
		return
	}
	body, _ := json.Marshal(in)
	idemID := hash(u.ID + ":" + key)
	var out Session
	e := s.Store.Tx(func(tx *gorm.DB) error {
		var old Idempotency
		if tx.First(&old, "id = ?", idemID).Error == nil {
			if old.RequestHash != hash(string(body)) {
				return badRequest("请求重复提交，请重试")
			}
			if e := tx.First(&out, "id = ?", old.SessionID).Error; e != nil {
				return e
			}
			// Replaying a key whose session has already ended must not look
			// like a fresh start: the caller would go on to poll a session
			// that is gone and report a session-ended error for a start it
			// just made. Say the truth instead, and let the retryable path
			// cover it when the ending is still in progress.
			if out.State == "CLOSED" {
				return badRequest("这次请求对应的加速会话已经结束，请重新开始加速")
			}
			return nil
		}
		var d Device
		if tx.First(&d, "id = ? AND user_id = ?", in.DeviceID, u.ID).Error != nil {
			return badRequest("设备尚未登记，请重新登录")
		}
		// A device holds one session at a time. Two kinds of leftover are not
		// a dead end, because they clear by themselves: one that is already
		// stopping (its nodes are confirming the forwarding is gone), and one
		// whose lease has run out (the client that held it is gone, and the
		// reaper stops it within seconds). The client waits and retries
		// instead of telling the player no. Every leftover is looked at: one
		// live session among them is a plain refusal, since waiting would
		// never clear it.
		var holding []Session
		if e := tx.Where("device_id = ? AND state <> ?", d.ID, "CLOSED").Order("created_at").Find(&holding).Error; e != nil {
			return e
		}
		var wait error
		now := time.Now()
		for _, h := range holding {
			switch {
			case h.State == "STOPPING":
				if wait == nil {
					wait = retryLater("session-closing", "上一个加速会话正在结束，请稍候重试")
				}
			case now.After(h.LeaseUntil):
				if wait == nil {
					wait = retryLater("session-stale", "上一个加速会话尚未释放，正在自动清理，请稍候重试")
				}
			default:
				return badRequest("此设备已有一个加速会话在运行")
			}
		}
		if wait != nil {
			return wait
		}
		var active int64
		tx.Model(&Session{}).Where("user_id = ? AND state <> ? AND purpose = ?", u.ID, "CLOSED", "game").Count(&active)
		if active >= int64(u.MaxSessions) {
			return badRequest("账号同时加速的设备数已达上限")
		}
		var g Game
		if tx.First(&g, "id = ?", in.GameID).Error != nil || !g.Enabled || g.PublishedVersion == 0 || !slices.Contains(g.Regions, in.RegionID) {
			return badRequest("该游戏或区服尚未开放")
		}
		var profile GameVersion
		if e := tx.First(&profile, "game_id = ? AND version = ?", g.ID, g.PublishedVersion).Error; e != nil {
			return e
		}
		var l Line
		if tx.First(&l, "id = ?", in.LineID).Error != nil || l.Draining || !permitted(l, u) {
			return badRequest("该线路不可用或未授权给当前账号")
		}
		var v LineVersion
		if tx.First(&v, "id = ? AND status = ?", l.PublishedVersion, "ready").Error != nil {
			return badRequest("该线路尚未通过验证发布")
		}
		if !slices.Contains(v.Regions, in.RegionID) || (len(v.GameIDs) > 0 && !slices.Contains(v.GameIDs, in.GameID)) {
			return badRequest("该线路不支持所选游戏或区服")
		}
		if v.Hops[0].Capabilities.IPv6 && !v.Hops[0].Capabilities.IPv4 && !d.Capabilities.IPv6 {
			return badRequest("此设备无法使用仅支持 IPv6 的线路入口")
		}
		// Validation sessions do not occupy player capacity.
		tx.Model(&Session{}).Where("line_id = ? AND state <> ? AND purpose = ?", l.ID, "CLOSED", "game").Count(&active)
		if active >= int64(l.Capacity) {
			return badRequest("线路容量已满，请选择其他线路")
		}
		exitID := v.Hops[len(v.Hops)-1].EndpointID
		if !egressAllowed(tx, exitID, u.ID) {
			return badRequest("该线路的落地属于其他账号")
		}
		if in.BindingID == "" {
			var owned Binding
			if tx.First(&owned, "endpoint_id = ? AND user_id = ? AND enabled = ?", exitID, u.ID, true).Error == nil {
				in.BindingID = owned.ID
			}
		}
		if in.BindingID != "" {
			var b Binding
			if tx.First(&b, "id = ? AND user_id = ?", in.BindingID, u.ID).Error != nil || !b.Enabled || b.VerifiedAt == nil {
				return badRequest("落地绑定尚未验证")
			}
			if b.FixedIP && b.ExpectedExit != "" {
				v.ExpectedExit = b.ExpectedExit
			}
			if v.Hops[len(v.Hops)-1].EndpointID != b.EndpointID {
				return badRequest("所选线路与落地绑定不匹配")
			}
		}
		out = Session{ID: ID(), UserID: u.ID, DeviceID: d.ID, GameID: g.ID, RegionID: in.RegionID, LineID: l.ID, LineVersion: v.ID, ProfileVersion: g.PublishedVersion, BindingID: in.BindingID, Purpose: "game"}
		if e := s.provision(tx, &out, v, profile.Profile); e != nil {
			return e
		}
		return tx.Create(&Idempotency{ID: idemID, SessionID: out.ID, RequestHash: hash(string(body)), CreatedAt: time.Now().UTC()}).Error
	})
	if e != nil {
		respond(w, nil, e)
		return
	}
	write(w, 202, out)
}
func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	var x Session
	if e := s.Store.DB.First(&x, "id = ? AND user_id = ?", r.PathValue("id"), actor(r).ID).Error; e != nil {
		respond(w, nil, e)
		return
	}
	out := map[string]any{"session": x}
	if x.State == "CONNECTING" || x.State == "ACTIVE" || x.State == "DEGRADED" || x.State == "RECONNECTING" {
		var conn c.Connection
		if e := s.Store.Unseal(x.ConnectionCipher, &conn); e != nil {
			fail(w, 500, "无法解密会话配置：请确认控制服务的 ENCRYPTION_KEY 没有被更改")
			return
		}
		out["connection"] = conn
	}
	write(w, 200, out)
}
func (s *Server) heartbeatSession(w http.ResponseWriter, r *http.Request) {
	var in struct {
		State string        `json:"state"`
		Probe c.ProbeResult `json:"probe"`
	}
	if !decode(w, r, &in) {
		return
	}
	var out Session
	e := s.Store.Tx(func(tx *gorm.DB) error {
		if e := tx.First(&out, "id = ? AND user_id = ?", r.PathValue("id"), actor(r).ID).Error; e != nil {
			return e
		}
		if out.State == "CLOSED" || out.State == "STOPPING" {
			return badRequest("加速会话正在关闭")
		}
		var currentLine Line
		if tx.First(&currentLine, "id = ?", out.LineID).Error != nil || !permitted(currentLine, actor(r)) {
			return badRequest("该线路的使用权限已被收回")
		}
		var version LineVersion
		if tx.First(&version, "id = ?", out.LineVersion).Error != nil || len(version.Hops) == 0 || !egressAllowed(tx, version.Hops[len(version.Hops)-1].EndpointID, actor(r).ID) {
			return badRequest("落地的使用权限已被收回")
		}
		if out.BindingID != "" {
			var binding Binding
			if tx.First(&binding, "id = ? AND user_id = ?", out.BindingID, actor(r).ID).Error != nil || !binding.Enabled {
				return badRequest("落地的使用权限已被收回")
			}
		}
		if time.Now().After(out.LeaseUntil) {
			return badRequest("加速会话已过期")
		}
		allowed := map[string][]string{"CONNECTING": {"ACTIVE", "DEGRADED"}, "ACTIVE": {"ACTIVE", "DEGRADED", "RECONNECTING"}, "DEGRADED": {"ACTIVE", "DEGRADED", "RECONNECTING"}, "RECONNECTING": {"ACTIVE", "DEGRADED", "RECONNECTING"}, "PREPARING": {"PREPARING"}}
		if !slices.Contains(allowed[out.State], in.State) {
			return badRequest("加速会话状态异常")
		}
		if in.State == "ACTIVE" {
			var conn c.Connection
			if e := s.Store.Unseal(out.ConnectionCipher, &conn); e != nil {
				return e
			}
			if !in.Probe.TCP || (conn.Profile.UDP && !in.Probe.UDP) || in.Probe.Samples < 1 {
				return badRequest("缺少隧道检测结果，不能标记为已连接")
			}
		}
		if out.State == "PREPARING" {
			return nil
		}
		out.State = in.State
		out.Probe = in.Probe
		out.LeaseUntil = time.Now().UTC().Add(s.Config.LeaseDuration)
		out.UpdatedAt = time.Now().UTC()
		return tx.Save(&out).Error
	})
	respond(w, out, e)
}
func (s *Server) stopping(tx *gorm.DB, x *Session, reason string) error {
	if x.State == "CLOSED" || x.State == "STOPPING" {
		return nil
	}
	x.State = "STOPPING"
	x.EndReason = reason
	if x.Purpose == "validation" {
		if e := tx.Model(&LineVersion{}).Where("id = ? AND status = ?", x.LineVersion, "validating").Updates(map[string]any{"status": "failed", "error": reason}).Error; e != nil {
			return e
		}
	}
	x.StopUntil = timePointer(x.LeaseUntil.Add(s.Config.ReclaimGrace))
	x.UpdatedAt = time.Now().UTC()
	if e := tx.Save(x).Error; e != nil {
		return e
	}
	var reservations []Reservation
	if e := tx.Where("session_id = ?", x.ID).Find(&reservations).Error; e != nil {
		return e
	}
	for _, res := range reservations {
		if e := s.job(tx, res.NodeID, x.ID, x.LineVersion, "revoke", c.JobPayload{Runtime: &c.Runtime{ID: res.ID, SessionID: x.ID, ListenPort: res.Port}}); e != nil {
			return e
		}
	}
	if len(reservations) == 0 {
		return s.closeSession(tx, x)
	}
	return audit(tx, x.UserID, "session.stop", x.ID, reason)
}
func (s *Server) closeSession(tx *gorm.DB, x *Session) error {
	x.State = "CLOSED"
	x.UpdatedAt = time.Now().UTC()
	if e := tx.Where("session_id = ?", x.ID).Delete(&Reservation{}).Error; e != nil {
		return e
	}
	if e := tx.Model(&Job{}).Where("session_id = ? AND status IN ?", x.ID, []string{"pending", "running"}).Update("status", "cancelled").Error; e != nil {
		return e
	}
	if e := tx.Save(x).Error; e != nil {
		return e
	}
	return audit(tx, x.UserID, "session.closed", x.ID, x.EndReason)
}
func (s *Server) stopSession(w http.ResponseWriter, r *http.Request) {
	var x Session
	e := s.Store.Tx(func(tx *gorm.DB) error {
		q := tx.Where("id = ?", r.PathValue("id"))
		// The admin route is already restricted to operators and admins.
		if actor(r).Role != "admin" && !strings.HasPrefix(r.URL.Path, "/api/admin/") {
			q = q.Where("user_id = ?", actor(r).ID)
		}
		if e := q.First(&x).Error; e != nil {
			return e
		}
		return s.stopping(tx, &x, "requested")
	})
	respond(w, x, e)
}

// Reap stops sessions whose lease or preparation lapsed and closes stopping
// sessions past their reclaim grace. Each session is handled in its own
// transaction, re-read under the lock, so one failing row neither rolls back
// nor blocks the rest.
func (s *Server) Reap() error {
	// Time is compared in Go: SQLite compares timestamps as text.
	var live []Session
	if e := s.Store.DB.Select("id", "state", "lease_until", "prepare_until", "stop_until").Where("state <> ?", "CLOSED").Find(&live).Error; e != nil {
		return e
	}
	var failures []error
	for _, candidate := range live {
		if !reapDue(candidate, time.Now()) {
			continue
		}
		e := s.Store.Tx(func(tx *gorm.DB) error {
			x, e := load[Session](tx, candidate.ID)
			if e != nil || !reapDue(x, time.Now()) {
				return e
			}
			if x.State == "STOPPING" {
				return s.closeSession(tx, &x)
			}
			return s.stopping(tx, &x, "lease or preparation timeout")
		})
		if e != nil {
			failures = append(failures, fmt.Errorf("session %s: %w", candidate.ID, e))
		}
	}
	return errors.Join(failures...)
}
func reapDue(x Session, now time.Time) bool {
	switch x.State {
	case "CLOSED":
		return false
	case "STOPPING":
		// A STOPPING row written without a deadline never becomes due, so it
		// would hold its device and ports forever, and a client waiting for
		// the previous session to finish would wait for nothing. Only an
		// older row or a full-row update can look like this; close it.
		return x.StopUntil == nil || now.After(*x.StopUntil)
	case "PREPARING":
		return now.After(x.LeaseUntil) || now.After(x.PrepareUntil)
	}
	return now.After(x.LeaseUntil)
}
