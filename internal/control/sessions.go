package control

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"gorm.io/gorm"
	"net/http"
	"slices"
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
		return errors.New("empty published path")
	}
	now := time.Now().UTC()
	x.CreatedAt = now
	x.UpdatedAt = now
	x.State = "PREPARING"
	x.PrepareUntil = now.Add(s.Config.PrepareTimeout)
	x.LeaseUntil = now.Add(s.Config.LeaseDuration)
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
				return errors.New("node offline")
			}
			allocated := false
			for port := h.Port; port <= h.PortEnd; port++ {
				var count int64
				tx.Model(&Reservation{}).Where("node_id = ? AND port = ?", h.NodeID, port).Count(&count)
				if count == 0 {
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
				return errors.New("node port pool exhausted")
			}
		}
	}
	connection := c.Connection{Host: hops[0].Host, Port: hops[0].Port, Cipher: hops[len(hops)-1].Cipher, Password: password, UDP: true, Profile: p, ProbeURL: s.Config.ProbeURL, ProbeUDP: s.Config.ProbeUDP, ExpectedExit: v.ExpectedExit}
	for _, h := range hops {
		connection.UDP = connection.UDP && h.Capabilities.UDP
	}
	if p.UDP && !connection.UDP {
		return errors.New("game requires UDP across the complete path")
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
			runtime := c.Runtime{VersionID: v.ID, ID: reservations[i].ID, SessionID: x.ID, Kind: h.Kind, ListenPort: h.Port, Cipher: h.Cipher, Password: password, UDP: connection.UDP, ExpiresAt: x.LeaseUntil}
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
		fail(w, 400, "Idempotency-Key of 8-128 characters required")
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
				return errors.New("idempotency key reused with different input")
			}
			return tx.First(&out, "id = ?", old.SessionID).Error
		}
		var d Device
		if tx.First(&d, "id = ? AND user_id = ?", in.DeviceID, u.ID).Error != nil {
			return errors.New("register this device first")
		}
		var active int64
		tx.Model(&Session{}).Where("device_id = ? AND state <> ?", d.ID, "CLOSED").Count(&active)
		if active > 0 {
			return errors.New("device already has a session")
		}
		tx.Model(&Session{}).Where("user_id = ? AND state <> ? AND purpose = ?", u.ID, "CLOSED", "game").Count(&active)
		if active >= int64(u.MaxSessions) {
			return errors.New("account concurrency limit reached")
		}
		var g Game
		if tx.First(&g, "id = ?", in.GameID).Error != nil || !g.Enabled || g.PublishedVersion == 0 || !slices.Contains(g.Regions, in.RegionID) {
			return errors.New("game profile or region not published")
		}
		var profile GameVersion
		if e := tx.First(&profile, "game_id = ? AND version = ?", g.ID, g.PublishedVersion).Error; e != nil {
			return e
		}
		var l Line
		if tx.First(&l, "id = ?", in.LineID).Error != nil || l.Draining || !permitted(l, u) {
			return errors.New("line unavailable or unauthorized")
		}
		var v LineVersion
		if tx.First(&v, "id = ? AND status = ?", l.PublishedVersion, "ready").Error != nil {
			return errors.New("line is not verified and published")
		}
		if !slices.Contains(v.Regions, in.RegionID) || (len(v.GameIDs) > 0 && !slices.Contains(v.GameIDs, in.GameID)) {
			return errors.New("line does not cover game/region")
		}
		if v.Hops[0].Capabilities.IPv6 && !v.Hops[0].Capabilities.IPv4 && !d.Capabilities.IPv6 {
			return errors.New("device cannot use IPv6-only ingress")
		}
		tx.Model(&Session{}).Where("line_id = ? AND state <> ?", l.ID, "CLOSED").Count(&active)
		if active >= int64(l.Capacity) {
			return errors.New("line capacity exhausted")
		}
		exitID := v.Hops[len(v.Hops)-1].EndpointID
		if !egressAllowed(tx, exitID, u.ID) {
			return errors.New("egress belongs to another account")
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
				return errors.New("egress binding not verified")
			}
			if b.FixedIP && b.ExpectedExit != "" {
				v.ExpectedExit = b.ExpectedExit
			}
			if v.Hops[len(v.Hops)-1].EndpointID != b.EndpointID {
				return errors.New("selected line does not terminate at this egress")
			}
		}
		out = Session{ID: ID(), UserID: u.ID, DeviceID: d.ID, GameID: g.ID, RegionID: in.RegionID, LineID: l.ID, LineVersion: v.ID, ProfileVersion: g.PublishedVersion, BindingID: in.BindingID, Purpose: "game"}
		if e := s.provision(tx, &out, v, profile.Profile); e != nil {
			return e
		}
		return tx.Create(&Idempotency{ID: idemID, SessionID: out.ID, RequestHash: hash(string(body))}).Error
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
			fail(w, 500, "cannot decrypt session configuration")
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
			return errors.New("session is closing")
		}
		var currentLine Line
		if tx.First(&currentLine, "id = ?", out.LineID).Error != nil || !permitted(currentLine, actor(r)) {
			return errors.New("line access was revoked")
		}
		var version LineVersion
		if tx.First(&version, "id = ?", out.LineVersion).Error != nil || len(version.Hops) == 0 || !egressAllowed(tx, version.Hops[len(version.Hops)-1].EndpointID, actor(r).ID) {
			return errors.New("egress access was revoked")
		}
		if out.BindingID != "" {
			var binding Binding
			if tx.First(&binding, "id = ? AND user_id = ?", out.BindingID, actor(r).ID).Error != nil || !binding.Enabled {
				return errors.New("egress access was revoked")
			}
		}
		if time.Now().After(out.LeaseUntil) {
			return errors.New("session lease expired")
		}
		allowed := map[string][]string{"CONNECTING": {"ACTIVE", "DEGRADED"}, "ACTIVE": {"ACTIVE", "DEGRADED", "RECONNECTING"}, "DEGRADED": {"ACTIVE", "DEGRADED", "RECONNECTING"}, "RECONNECTING": {"ACTIVE", "DEGRADED", "RECONNECTING"}, "PREPARING": {"PREPARING"}}
		if !slices.Contains(allowed[out.State], in.State) {
			return errors.New("invalid session transition")
		}
		if in.State == "ACTIVE" {
			var conn c.Connection
			if e := s.Store.Unseal(out.ConnectionCipher, &conn); e != nil {
				return e
			}
			if !in.Probe.TCP || (conn.Profile.UDP && !in.Probe.UDP) || in.Probe.Samples < 1 {
				return errors.New("active state requires tunnel probe evidence")
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
		if actor(r).Role != "admin" {
			q = q.Where("user_id = ?", actor(r).ID)
		}
		if e := q.First(&x).Error; e != nil {
			return e
		}
		return s.stopping(tx, &x, "requested")
	})
	respond(w, x, e)
}
func (s *Server) Reap() error {
	return s.Store.Tx(func(tx *gorm.DB) error {
		var sessions []Session
		if e := tx.Where("state <> ?", "CLOSED").Find(&sessions).Error; e != nil {
			return e
		}
		now := time.Now().UTC()
		for _, x := range sessions {
			if x.State == "STOPPING" && x.StopUntil != nil && now.After(*x.StopUntil) {
				if e := s.closeSession(tx, &x); e != nil {
					return e
				}
				continue
			}
			if now.After(x.LeaseUntil) || (x.State == "PREPARING" && now.After(x.PrepareUntil)) {
				if x.Purpose == "validation" {
					tx.Model(&LineVersion{}).Where("id = ? AND status <> ?", x.LineVersion, "ready").Updates(map[string]any{"status": "failed", "error": "preparation or lease timeout"})
				}
				if e := s.stopping(tx, &x, "lease or preparation timeout"); e != nil {
					return e
				}
			}
		}
		return nil
	})
}
