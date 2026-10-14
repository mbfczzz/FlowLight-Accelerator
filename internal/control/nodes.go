package control

import (
	"encoding/json"
	"errors"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"gorm.io/gorm"
	"net"
	"net/http"
	"time"
)

func (s *Server) enrollment(w http.ResponseWriter, r *http.Request) {
	token := secret()
	e := s.Store.Tx(func(tx *gorm.DB) error {
		n, e := load[Node](tx, r.PathValue("id"))
		if e != nil {
			return e
		}
		n.BootstrapHash = hash(token)
		n.BootstrapUntil = time.Now().UTC().Add(15 * time.Minute)
		if e = tx.Save(&n).Error; e != nil {
			return e
		}
		return audit(tx, actor(r).ID, "node.enroll", n.ID, "")
	})
	respond(w, map[string]any{"node_id": r.PathValue("id"), "bootstrap_secret": token, "node_ca": string(s.CA.PEM)}, e)
}
func (s *Server) registerNode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		NodeID    string `json:"node_id"`
		Bootstrap string `json:"bootstrap_secret"`
		CSR       string `json:"csr"`
	}
	if !decode(w, r, &in) {
		return
	}
	token := secret()
	var certificate string
	e := s.Store.Tx(func(tx *gorm.DB) error {
		n, e := load[Node](tx, in.NodeID)
		if e != nil || !n.Enabled || in.Bootstrap == "" || n.BootstrapHash != hash(in.Bootstrap) || time.Now().After(n.BootstrapUntil) {
			return errors.New("invalid enrollment identity")
		}
		var fingerprint string
		certificate, fingerprint, e = s.CA.Issue(in.CSR, n.ID)
		if e != nil {
			return e
		}
		n.BootstrapHash = ""
		n.TokenHash = hash(token)
		n.CertificateHash = fingerprint
		if e = tx.Save(&n).Error; e != nil {
			return e
		}
		return audit(tx, n.ID, "node.register", n.ID, "")
	})
	respond(w, map[string]any{"node_id": in.NodeID, "token": token, "certificate": certificate, "ca_certificate": string(s.CA.PEM)}, e)
}
func (s *Server) nodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version      string         `json:"version"`
		Capabilities c.Capabilities `json:"capabilities"`
		Versions     []string       `json:"actual_versions"`
	}
	if !decode(w, r, &in) {
		return
	}
	n := r.Context().Value(nodeKey).(Node)
	leases := map[string]time.Time{}
	e := s.Store.Tx(func(tx *gorm.DB) error {
		n.LastSeen = time.Now().UTC()
		n.AgentVersion = in.Version
		n.ActualVersions = in.Versions
		n.Capabilities.TCP = n.Capabilities.TCP && in.Capabilities.TCP
		n.Capabilities.IPv6 = n.Capabilities.IPv6 && in.Capabilities.IPv6
		n.Capabilities.UDP = n.Capabilities.UDP && in.Capabilities.UDP
		n.Capabilities.Managed = n.Capabilities.Managed && in.Capabilities.Managed
		if e := tx.Save(&n).Error; e != nil {
			return e
		}
		var reservations []Reservation
		tx.Where("node_id = ?", n.ID).Find(&reservations)
		for _, res := range reservations {
			var x Session
			if tx.First(&x, "id = ?", res.SessionID).Error == nil && x.State != "STOPPING" && x.State != "CLOSED" && time.Now().Before(x.LeaseUntil) {
				leases[res.SessionID] = x.LeaseUntil
			}
		}
		return nil
	})
	respond(w, map[string]any{"server_time": time.Now().UTC(), "leases": leases}, e)
}
func (s *Server) claimJobs(w http.ResponseWriter, r *http.Request) {
	n := r.Context().Value(nodeKey).(Node)
	out := []c.Job{}
	e := s.Store.Tx(func(tx *gorm.DB) error {
		var jobs []Job
		now := time.Now().UTC()
		if e := tx.Where("node_id = ? AND (status = ? OR (status = ? AND claim_until < ?))", n.ID, "pending", "running", now).Order("CASE WHEN kind = 'revoke' THEN 0 ELSE 1 END, created_at").Limit(1).Find(&jobs).Error; e != nil {
			return e
		}
		for _, j := range jobs {
			var x Session
			if tx.First(&x, "id = ?", j.SessionID).Error != nil || x.State == "CLOSED" || (x.State == "STOPPING" && j.Kind != "revoke") {
				j.Status = "cancelled"
				tx.Save(&j)
				continue
			}
			if j.Attempts >= 5 {
				j.Status = "failed"
				j.Result = "retry limit exceeded"
				tx.Save(&j)
				if e := s.stopping(tx, &x, j.Result); e != nil {
					return e
				}
				continue
			}
			claim := secret()
			j.Status = "running"
			j.ClaimHash = hash(claim)
			j.ClaimUntil = now.Add(60 * time.Second)
			j.Attempts++
			if e := tx.Save(&j).Error; e != nil {
				return e
			}
			var payload c.JobPayload
			if e := s.Store.Unseal(j.PayloadCipher, &payload); e != nil {
				return e
			}
			out = append(out, c.Job{ID: j.ID, NodeID: n.ID, SessionID: j.SessionID, VersionID: j.VersionID, Kind: j.Kind, Payload: payload, Claim: claim, Attempt: j.Attempts})
		}
		return nil
	})
	respond(w, out, e)
}
func (s *Server) jobResult(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Claim   string        `json:"claim"`
		Success bool          `json:"success"`
		Error   string        `json:"error"`
		Probe   c.ProbeResult `json:"probe"`
	}
	if !decode(w, r, &in) {
		return
	}
	node := r.Context().Value(nodeKey).(Node)
	e := s.Store.Tx(func(tx *gorm.DB) error {
		j, e := load[Job](tx, r.PathValue("id"))
		if e != nil || j.NodeID != node.ID {
			return errors.New("job not owned by node")
		}
		if in.Claim == "" || hash(in.Claim) != j.ClaimHash {
			return errors.New("stale job claim")
		}
		if j.Status == "succeeded" || j.Status == "failed" {
			return nil
		}
		if j.Status != "running" || time.Now().After(j.ClaimUntil) {
			return errors.New("job claim expired")
		}
		x, e := load[Session](tx, j.SessionID)
		if e != nil {
			return e
		}
		if x.State == "CLOSED" {
			return nil
		}
		if j.Kind != "revoke" && time.Now().After(x.LeaseUntil) {
			return errors.New("session lease expired before task acknowledgement")
		}
		j.Status = "succeeded"
		if !in.Success {
			j.Status = "failed"
		}
		if len(in.Error) > 500 {
			in.Error = in.Error[:500]
		}
		j.Result = in.Error
		if j.Kind == "probe" && in.Success {
			var conn c.Connection
			if e = s.Store.Unseal(x.ConnectionCipher, &conn); e != nil {
				return e
			}
			if !in.Probe.TCP || (conn.UDP && !in.Probe.UDP) || in.Probe.Samples < 3 || in.Probe.Loss < 0 || in.Probe.Loss > 1 || in.Probe.RTTMS < 0 || net.ParseIP(in.Probe.ExitIP) == nil || (conn.ExpectedExit != "" && conn.ExpectedExit != in.Probe.ExitIP) {
				in.Success = false
				j.Status = "failed"
				j.Result = "incomplete probe evidence or unexpected exit"
			}
		}
		if e = tx.Save(&j).Error; e != nil {
			return e
		}
		if !in.Success {
			if x.Purpose == "validation" {
				tx.Model(&LineVersion{}).Where("id = ?", x.LineVersion).Updates(map[string]any{"status": "failed", "error": j.Result})
			}
			return s.stopping(tx, &x, "node task failed: "+j.Kind)
		}
		if j.Kind == "revoke" {
			var outstanding int64
			tx.Model(&Job{}).Where("session_id = ? AND kind = ? AND status <> ?", x.ID, "revoke", "succeeded").Count(&outstanding)
			if outstanding == 0 {
				return s.closeSession(tx, &x)
			}
			return nil
		}
		if x.State == "STOPPING" {
			return nil
		}
		if j.Kind == "prepare" {
			var pending int64
			tx.Model(&Job{}).Where("session_id = ? AND kind = ? AND status <> ?", x.ID, "prepare", "succeeded").Count(&pending)
			if pending == 0 {
				var conn c.Connection
				if e = s.Store.Unseal(x.ConnectionCipher, &conn); e != nil {
					return e
				}
				v, e := load[LineVersion](tx, x.LineVersion)
				if e != nil {
					return e
				}
				return s.job(tx, v.Hops[0].NodeID, x.ID, x.LineVersion, "probe", c.JobPayload{Connection: &conn})
			}
		}
		if j.Kind == "probe" {
			in.Probe.ObservedAt = time.Now().UTC()
			x.Probe = in.Probe
			if x.Purpose == "validation" {
				v, e := load[LineVersion](tx, x.LineVersion)
				if e != nil {
					return e
				}
				v.Status = "ready"
				v.Probe = in.Probe
				if e = tx.Save(&v).Error; e != nil {
					return e
				}
				if e = tx.Model(&Line{}).Where("id = ?", v.LineID).Update("published_version", v.ID).Error; e != nil {
					return e
				}
				now := time.Now().UTC()
				tx.Model(&Binding{}).Where("endpoint_id = ? AND enabled = ? AND (expected_exit = ? OR expected_exit = ?)", v.Hops[len(v.Hops)-1].EndpointID, true, "", in.Probe.ExitIP).Update("verified_at", now)
				if e = audit(tx, x.UserID, "line.publish.ready", v.LineID, v.ID); e != nil {
					return e
				}
				return s.stopping(tx, &x, "validation complete")
			}
			x.State = "CONNECTING"
			return tx.Save(&x).Error
		}
		return nil
	})
	respond(w, map[string]bool{"accepted": e == nil}, e)
}
func (s *Server) nodeEvents(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Events []struct {
			SessionID string `json:"session_id"`
			Kind      string `json:"kind"`
		} `json:"events"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Events) > 100 {
		fail(w, 400, "event batch too large")
		return
	}
	n := r.Context().Value(nodeKey).(Node)
	e := s.Store.Tx(func(tx *gorm.DB) error {
		for _, ev := range in.Events {
			var count int64
			tx.Model(&Reservation{}).Where("node_id = ? AND session_id = ?", n.ID, ev.SessionID).Count(&count)
			if count == 0 {
				return errors.New("session not owned by node")
			}
			if ev.Kind == "lease_expired" || ev.Kind == "runtime_failed" {
				x, e := load[Session](tx, ev.SessionID)
				if e != nil {
					return e
				}
				if e = s.stopping(tx, &x, ev.Kind); e != nil {
					return e
				}
			}
		}
		return nil
	})
	respond(w, map[string]bool{"accepted": e == nil}, e)
}

var _ = json.Marshal
