package control

import (
	"encoding/json"
	"errors"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

func (s *Server) save(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := cleanID(r)
		var out any
		var raw json.RawMessage
		if !decode(w, r, &raw) {
			return
		}
		e := s.Store.Tx(func(tx *gorm.DB) error {
			if r.Method == http.MethodPatch {
				var mergeError error
				raw, mergeError = mergePatch(tx, kind, id, raw)
				if mergeError != nil {
					return mergeError
				}
			}
			switch kind {
			case "games":
				var in Game
				if json.Unmarshal(raw, &in) != nil || !nameOK(in.Name) {
					return errInvalid
				}
				old, _ := load[Game](tx, id)
				in.ID = id
				in.PublishedVersion = old.PublishedVersion
				in.UpdatedAt = time.Now().UTC()
				out = in
				if e := validateProfile(in.Draft); e != nil {
					return e
				}
				if e := tx.Save(&in).Error; e != nil {
					return e
				}
			case "nodes":
				var in Node
				if json.Unmarshal(raw, &in) != nil || !nameOK(in.Name) || !validHost(in.Address) {
					return errInvalid
				}
				old, _ := load[Node](tx, id)
				in.ID = id
				in.BootstrapHash = old.BootstrapHash
				in.BootstrapUntil = old.BootstrapUntil
				in.TokenHash = old.TokenHash
				in.CertificateHash = old.CertificateHash
				in.LastSeen = old.LastSeen
				in.AgentVersion = old.AgentVersion
				in.ActualVersions = old.ActualVersions
				out = in
				if e := tx.Save(&in).Error; e != nil {
					return e
				}
			case "endpoints":
				var in struct {
					Endpoint
					Password string `json:"password"`
				}
				if json.Unmarshal(raw, &in) != nil || !validHost(in.Host) || in.Port < 1 || (in.Capabilities.Managed && in.Port < 1024) || in.PortEnd < in.Port || in.PortEnd > 65535 || !nameOK(in.Name) {
					return errInvalid
				}
				var n Node
				if tx.First(&n, "id = ?", in.NodeID).Error != nil {
					return errors.New("unknown node")
				}
				old, _ := load[Endpoint](tx, id)
				in.ID = id
				in.CredentialID = old.CredentialID
				in.OwnerID = old.OwnerID
				if in.Password != "" {
					if !validKey(in.Cipher, in.Password) {
						return errors.New("invalid SS2022 key or cipher")
					}
					enc, e := s.Store.Seal(in.Password)
					if e != nil {
						return e
					}
					cred := Credential{ID: ID(), Ciphertext: enc}
					if e = tx.Create(&cred).Error; e != nil {
						return e
					}
					in.CredentialID = cred.ID
				}
				if in.Kind != "realm" && in.Kind != "ss2022" {
					return errors.New("unsupported endpoint kind")
				}
				out = in.Endpoint
				if e := tx.Save(&in.Endpoint).Error; e != nil {
					return e
				}
			case "lines":
				var in Line
				if json.Unmarshal(raw, &in) != nil || !nameOK(in.Name) || in.Capacity < 1 {
					return errInvalid
				}
				old, _ := load[Line](tx, id)
				in.ID = id
				in.PublishedVersion = old.PublishedVersion
				in.Draining = old.Draining
				in.UpdatedAt = time.Now().UTC()
				out = in
				if e := tx.Save(&in).Error; e != nil {
					return e
				}
			case "users":
				var in struct {
					Username    string `json:"username"`
					Password    string `json:"password"`
					Role        string `json:"role"`
					Enabled     bool   `json:"enabled"`
					MaxSessions int    `json:"max_sessions"`
				}
				if json.Unmarshal(raw, &in) != nil || !nameOK(in.Username) || !slices.Contains([]string{"admin", "operator", "publisher", "viewer", "player"}, in.Role) || in.MaxSessions < 1 || in.MaxSessions > 20 {
					return errInvalid
				}
				old, e := load[User](tx, id)
				if e != nil && in.Password == "" {
					return errors.New("password required")
				}
				if in.Password != "" {
					if len(in.Password) < 14 || len(in.Password) > 72 {
						return errors.New("password must contain 14-72 bytes")
					}
					h, e := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
					if e != nil {
						return e
					}
					old.PasswordHash = string(h)
					tx.Where("user_id = ?", id).Delete(&Token{})
				}
				if id == actor(r).ID && (!in.Enabled || in.Role != "admin") {
					return errors.New("cannot demote or disable your own administrator")
				}
				old.ID = id
				old.Username = in.Username
				old.Role = in.Role
				old.Enabled = in.Enabled
				old.MaxSessions = in.MaxSessions
				out = old
				if e = tx.Save(&old).Error; e != nil {
					return e
				}
			case "bindings":
				var in Binding
				if json.Unmarshal(raw, &in) != nil {
					return errInvalid
				}
				in.ID = id
				old, _ := load[Binding](tx, id)
				in.VerifiedAt = old.VerifiedAt
				if in.EndpointID != old.EndpointID || in.ExpectedExit != old.ExpectedExit || in.UserID != old.UserID {
					in.VerifiedAt = nil
				}
				if in.Kind != "shared" && in.Kind != "dedicated" && in.Kind != "byo" {
					return errInvalid
				}
				if _, e := load[User](tx, in.UserID); e != nil {
					return e
				}
				if _, e := load[Endpoint](tx, in.EndpointID); e != nil {
					return e
				}
				if in.Kind == "dedicated" {
					var n int64
					tx.Model(&Binding{}).Where("endpoint_id = ? AND enabled = ? AND id <> ?", in.EndpointID, true, id).Count(&n)
					if n > 0 {
						return errors.New("egress already assigned")
					}
				}
				out = in
				if e := tx.Save(&in).Error; e != nil {
					return e
				}
			case "releases":
				var in Release
				if json.Unmarshal(raw, &in) != nil {
					return errInvalid
				}
				u, e := url.Parse(in.URL)
				if e != nil || u.Scheme != "https" || len(in.SHA256) != 64 || in.Signature == "" || !nameOK(in.Version) {
					return errors.New("HTTPS URL, SHA256 and signed release manifest required")
				}
				in.ID = id
				in.CreatedAt = time.Now().UTC()
				out = in
				if e = tx.Save(&in).Error; e != nil {
					return e
				}
			}
			return audit(tx, actor(r).ID, "resource.save", id, kind)
		})
		respond(w, out, e)
	}
}
func (s *Server) games(w http.ResponseWriter, r *http.Request) {
	listRows[Game](w, s.Store.DB.Where("enabled = ?", true))
}
func (s *Server) regions(w http.ResponseWriter, r *http.Request) {
	g, e := load[Game](s.Store.DB, r.PathValue("id"))
	respond(w, g.Regions, e)
}
func (s *Server) device(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID           string         `json:"id"`
		Name         string         `json:"name"`
		Capabilities c.Capabilities `json:"capabilities"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.ID) < 8 || len(in.ID) > 64 || !nameOK(in.Name) {
		fail(w, 400, "device identifier and name required")
		return
	}
	out := Device{ID: in.ID, UserID: actor(r).ID, Name: in.Name, Capabilities: in.Capabilities, LastSeen: timePointer(time.Now().UTC())}
	e := s.Store.Tx(func(tx *gorm.DB) error {
		old, e := load[Device](tx, in.ID)
		if e == nil && old.UserID != out.UserID {
			return errors.New("device belongs to a different account")
		}
		return tx.Save(&out).Error
	})
	respond(w, out, e)
}
func (s *Server) clientLines(w http.ResponseWriter, r *http.Request) {
	var rows []Line
	s.Store.DB.Where("draining = ?", false).Find(&rows)
	out := []map[string]any{}
	for _, l := range rows {
		if !permitted(l, actor(r)) {
			continue
		}
		v, e := load[LineVersion](s.Store.DB, l.PublishedVersion)
		if e != nil || v.Status != "ready" {
			continue
		}
		if len(v.Hops) == 0 || !egressAllowed(s.Store.DB, v.Hops[len(v.Hops)-1].EndpointID, actor(r).ID) {
			continue
		}
		region := r.URL.Query().Get("region_id")
		game := r.URL.Query().Get("game_id")
		if region != "" && !slices.Contains(v.Regions, region) || game != "" && len(v.GameIDs) > 0 && !slices.Contains(v.GameIDs, game) {
			continue
		}
		var count int64
		s.Store.DB.Model(&Session{}).Where("line_id = ? AND state <> ?", l.ID, "CLOSED").Count(&count)
		healthy := count < int64(l.Capacity)
		for _, h := range v.Hops {
			if h.Capabilities.Managed {
				n, e := load[Node](s.Store.DB, h.NodeID)
				healthy = healthy && e == nil && n.Enabled && timeSince(n.LastSeen) < s.Config.NodeTimeout
			}
		}
		out = append(out, map[string]any{"id": l.ID, "name": l.Name, "version": v.ID, "regions": v.Regions, "mode": v.Mode, "available": healthy, "capacity": l.Capacity, "active_sessions": count, "probe": v.Probe, "priority": l.Priority})
	}
	slices.SortStableFunc(out, func(a, b map[string]any) int {
		aa := a["probe"].(c.ProbeResult)
		bb := b["probe"].(c.ProbeResult)
		if aa.RTTMS < bb.RTTMS {
			return -1
		}
		if aa.RTTMS > bb.RTTMS {
			return 1
		}
		return 0
	})
	write(w, 200, out)
}
func (s *Server) bindings(w http.ResponseWriter, r *http.Request) {
	listRows[Binding](w, s.Store.DB.Where("user_id = ?", actor(r).ID))
}
func (s *Server) clientBinding(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EndpointID   string `json:"endpoint_id"`
		Name         string `json:"name"`
		Host         string `json:"host"`
		Port         int    `json:"port"`
		Cipher       string `json:"cipher"`
		Password     string `json:"password"`
		ExpectedExit string `json:"expected_exit"`
	}
	if !decode(w, r, &in) {
		return
	}
	u := actor(r)
	var out Binding
	err := s.Store.Tx(func(tx *gorm.DB) error {
		if in.EndpointID == "" {
			if !s.Config.AllowImported {
				return errors.New("operator has not enabled external egress registration")
			}
			if !validHost(in.Host) || in.Port < 1 || in.Port > 65535 || !validKey(in.Cipher, in.Password) || !nameOK(in.Name) {
				return errInvalid
			}
			node := Node{ID: ID(), Name: in.Name, Address: in.Host, Enabled: true, Capabilities: c.Capabilities{TCP: true, UDP: true, IPv4: true}}
			if e := tx.Create(&node).Error; e != nil {
				return e
			}
			enc, e := s.Store.Seal(in.Password)
			if e != nil {
				return e
			}
			credential := Credential{ID: ID(), Ciphertext: enc}
			if e = tx.Create(&credential).Error; e != nil {
				return e
			}
			ep := Endpoint{ID: ID(), OwnerID: u.ID, NodeID: node.ID, Name: in.Name, Kind: "ss2022", Host: in.Host, Port: in.Port, PortEnd: in.Port, Cipher: in.Cipher, CredentialID: credential.ID, Enabled: true, Capabilities: node.Capabilities}
			if e = tx.Create(&ep).Error; e != nil {
				return e
			}
			out = Binding{ID: ID(), UserID: u.ID, EndpointID: ep.ID, Kind: "byo", ExpectedExit: in.ExpectedExit, Enabled: true}
			if e = tx.Create(&out).Error; e != nil {
				return e
			}
			return audit(tx, u.ID, "egress.register", out.ID, "pending operator line publication and network validation")
		}
		var existing Binding
		if tx.First(&existing, "user_id = ? AND endpoint_id = ?", u.ID, in.EndpointID).Error == nil {
			out = existing
			return nil
		}
		var lines []Line
		tx.Find(&lines)
		for _, l := range lines {
			if !permitted(l, u) {
				continue
			}
			v, e := load[LineVersion](tx, l.PublishedVersion)
			if e == nil && v.Status == "ready" && len(v.Hops) > 0 && v.Hops[len(v.Hops)-1].EndpointID == in.EndpointID {
				now := time.Now().UTC()
				out = Binding{ID: ID(), UserID: u.ID, EndpointID: in.EndpointID, Kind: "shared", Enabled: true, VerifiedAt: &now, ExpectedExit: v.Probe.ExitIP}
				return tx.Create(&out).Error
			}
		}
		return errors.New("egress not available to this account")
	})
	respond(w, out, err)
}
func (s *Server) latestRelease(w http.ResponseWriter, r *http.Request) {
	var rel Release
	e := s.Store.DB.Where("published = ? AND channel = ?", true, "stable").Order("created_at DESC").First(&rel).Error
	respond(w, rel, e)
}
func (s *Server) publishGame(w http.ResponseWriter, r *http.Request) {
	var out Game
	e := s.Store.Tx(func(tx *gorm.DB) error {
		var e error
		out, e = load[Game](tx, r.PathValue("id"))
		if e != nil {
			return e
		}
		if len(out.Regions) == 0 {
			return errors.New("configure at least one region before publishing")
		}
		if e = validateProfile(out.Draft); e != nil {
			return e
		}
		var maxVersion int
		tx.Model(&GameVersion{}).Where("game_id = ?", out.ID).Select("COALESCE(MAX(version),0)").Scan(&maxVersion)
		if raw := r.PathValue("version"); raw != "" {
			v, e := strconv.Atoi(raw)
			if e != nil || v < 1 {
				return errors.New("invalid profile version")
			}
			var previous GameVersion
			if tx.First(&previous, "game_id = ? AND version = ?", out.ID, v).Error == nil {
				out.PublishedVersion = v
				if e = tx.Save(&out).Error; e != nil {
					return e
				}
				return audit(tx, actor(r).ID, "game.profile.activate", out.ID, raw)
			}
			if v != maxVersion+1 {
				return errors.New("profile version does not exist")
			}
		}
		out.PublishedVersion = maxVersion + 1
		v := GameVersion{ID: ID(), GameID: out.ID, Version: out.PublishedVersion, Profile: out.Draft, CreatedAt: time.Now().UTC()}
		if e = tx.Create(&v).Error; e != nil {
			return e
		}
		if e = tx.Save(&out).Error; e != nil {
			return e
		}
		return audit(tx, actor(r).ID, "game.publish", out.ID, "")
	})
	respond(w, out, e)
}
func (s *Server) validateLine(w http.ResponseWriter, r *http.Request) {
	l, e := load[Line](s.Store.DB, r.PathValue("id"))
	if e != nil {
		respond(w, nil, e)
		return
	}
	h, e := s.compile(s.Store.DB, l)
	respond(w, map[string]any{"structurally_valid": e == nil, "hops": h, "network_verified": false}, e)
}
func (s *Server) publishLine(w http.ResponseWriter, r *http.Request) {
	var out LineVersion
	e := s.Store.Tx(func(tx *gorm.DB) error {
		l, e := load[Line](tx, r.PathValue("id"))
		if e != nil {
			return e
		}
		h, e := s.compile(tx, l)
		if e != nil {
			return e
		}
		if l.Mode == "imported" {
			probeNode, err := load[Node](tx, s.Config.ProbeNodeID)
			if err != nil || !probeNode.Enabled || timeSince(probeNode.LastSeen) > s.Config.NodeTimeout {
				return errors.New("configure an online PROBE_NODE_ID for imported line validation")
			}
		}
		if s.Config.ProbeURL == "" || s.Config.ProbeUDP == "" {
			return errors.New("configure controlled TCP and UDP probe endpoints before publication")
		}
		var pending int64
		tx.Model(&LineVersion{}).Where("line_id = ? AND status = ?", l.ID, "validating").Count(&pending)
		if pending > 0 {
			return errors.New("publication already in progress")
		}
		var n int64
		tx.Model(&LineVersion{}).Where("line_id = ?", l.ID).Count(&n)
		out = LineVersion{ID: ID(), LineID: l.ID, Number: int(n) + 1, Status: "validating", Hops: h, Regions: l.Regions, GameIDs: l.GameIDs, Mode: l.Mode, ExpectedExit: l.ExpectedExit, CreatedAt: time.Now().UTC()}
		if e = tx.Create(&out).Error; e != nil {
			return e
		}
		probe := Session{ID: ID(), UserID: actor(r).ID, LineID: l.ID, LineVersion: out.ID, Purpose: "validation"}
		if e = s.provision(tx, &probe, out, c.Profile{UDP: true}); e != nil {
			return e
		}
		return audit(tx, actor(r).ID, "line.publish.begin", l.ID, out.ID)
	})
	respond(w, out, e)
}
func (s *Server) versions(w http.ResponseWriter, r *http.Request) {
	listRows[LineVersion](w, s.Store.DB.Where("line_id = ?", r.PathValue("id")).Order("number DESC"))
}
func (s *Server) rollbackLine(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version string `json:"version_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	e := s.Store.Tx(func(tx *gorm.DB) error {
		v, e := load[LineVersion](tx, in.Version)
		if e != nil || v.LineID != r.PathValue("id") || v.Status != "ready" {
			return errors.New("choose a verified version of this line")
		}
		if e = tx.Model(&Line{}).Where("id = ?", v.LineID).Update("published_version", v.ID).Error; e != nil {
			return e
		}
		return audit(tx, actor(r).ID, "line.rollback", v.LineID, v.ID)
	})
	respond(w, map[string]bool{"ok": e == nil}, e)
}
func (s *Server) drainLine(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Draining bool `json:"draining"`
	}
	if !decode(w, r, &in) {
		return
	}
	e := s.Store.Tx(func(tx *gorm.DB) error {
		if _, e := load[Line](tx, r.PathValue("id")); e != nil {
			return e
		}
		if e := tx.Model(&Line{}).Where("id = ?", r.PathValue("id")).Update("draining", in.Draining).Error; e != nil {
			return e
		}
		return audit(tx, actor(r).ID, "line.drain", r.PathValue("id"), "")
	})
	respond(w, map[string]bool{"ok": e == nil}, e)
}
func (s *Server) terminateLine(w http.ResponseWriter, r *http.Request) {
	e := s.Store.Tx(func(tx *gorm.DB) error {
		tx.Model(&Line{}).Where("id = ?", r.PathValue("id")).Update("draining", true)
		var rows []Session
		tx.Where("line_id = ? AND state <> ?", r.PathValue("id"), "CLOSED").Find(&rows)
		for _, x := range rows {
			if e := s.stopping(tx, &x, "line terminated by operator"); e != nil {
				return e
			}
		}
		return audit(tx, actor(r).ID, "line.terminate", r.PathValue("id"), "")
	})
	respond(w, map[string]bool{"ok": e == nil}, e)
}

var _ = strings.TrimSpace

func mergePatch(tx *gorm.DB, kind, id string, patch json.RawMessage) (json.RawMessage, error) {
	var existing any
	switch kind {
	case "games":
		existing = &Game{}
	case "nodes":
		existing = &Node{}
	case "endpoints":
		existing = &Endpoint{}
	case "lines":
		existing = &Line{}
	case "bindings":
		existing = &Binding{}
	case "users":
		existing = &User{}
	case "releases":
		existing = &Release{}
	default:
		return nil, errInvalid
	}
	if e := tx.First(existing, "id = ?", id).Error; e != nil {
		return nil, e
	}
	b, e := json.Marshal(existing)
	if e != nil {
		return nil, e
	}
	var base, delta map[string]any
	if json.Unmarshal(b, &base) != nil || json.Unmarshal(patch, &delta) != nil {
		return nil, errInvalid
	}
	mergeObjects(base, delta)
	return json.Marshal(base)
}
func mergeObjects(base, delta map[string]any) {
	for k, v := range delta {
		if child, ok := v.(map[string]any); ok {
			if old, ok := base[k].(map[string]any); ok {
				mergeObjects(old, child)
				continue
			}
		}
		base[k] = v
	}
}
