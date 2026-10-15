package control

import (
	"context"
	"encoding/hex"
	"encoding/json"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Password lengths, in characters. Staff accounts can change lines and
// accounts, so they need the longer one; bcrypt reads at most 72 bytes.
const (
	playerPasswordMin = 8
	staffPasswordMin  = 14
	passwordMaxBytes  = 72
)

// checkName rejects an empty or overlong display name with a message that
// names the field.
func checkName(s, field string) error {
	if !nameOK(s) {
		return badRequest("请填写" + field + "（不超过 100 个字符）")
	}
	return nil
}

func checkHost(s, field string) error {
	if !validHost(s) {
		return badRequest(field + "不能为空，且不能包含空格、斜杠、引号或逗号")
	}
	return nil
}

func (s *Server) save(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := cleanID(r)
		var out any
		var raw json.RawMessage
		if !decode(w, r, &raw) {
			return
		}
		// bcrypt takes ~250 ms; hash before taking the transaction lock so a
		// password change does not stall every other write.
		var passwordHash []byte
		if kind == "users" {
			var p struct {
				Password string `json:"password"`
			}
			_ = json.Unmarshal(raw, &p)
			if p.Password != "" {
				// The role-specific minimum is checked once the role is known.
				if utf8.RuneCountInString(p.Password) < playerPasswordMin {
					respond(w, nil, badRequest("密码至少 "+strconv.Itoa(playerPasswordMin)+" 位"))
					return
				}
				if len(p.Password) > passwordMaxBytes {
					respond(w, nil, badRequest("密码过长：最多 72 字节（约 72 个英文字符或 24 个汉字）"))
					return
				}
				var e error
				if passwordHash, e = bcrypt.GenerateFromPassword([]byte(p.Password), 12); e != nil {
					respond(w, nil, e)
					return
				}
			}
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
				if json.Unmarshal(raw, &in) != nil {
					return errInvalid
				}
				if e := checkName(in.Name, "游戏名称"); e != nil {
					return e
				}
				old, _ := load[Game](tx, id)
				in.ID = id
				in.PublishedVersion = old.PublishedVersion
				in.CoverSlot = old.CoverSlot
				in.CoverImage = old.CoverImage
				if in.CoverSlot == 0 {
					var e error
					if in.CoverSlot, e = nextCoverSlot(tx); e != nil {
						return e
					}
				}
				in.CoverIndex = in.CoverSlot - 1
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
				if json.Unmarshal(raw, &in) != nil {
					return errInvalid
				}
				if e := checkName(in.Name, "节点名称"); e != nil {
					return e
				}
				if e := checkHost(in.Address, "节点地址"); e != nil {
					return e
				}
				old, _ := load[Node](tx, id)
				in.ID = id
				in.BootstrapHash = old.BootstrapHash
				in.BootstrapUntil = old.BootstrapUntil
				in.TokenHash = old.TokenHash
				in.CertificateHash = old.CertificateHash
				in.PendingCertificateHash = old.PendingCertificateHash
				in.LastSeen = old.LastSeen
				in.AgentVersion = old.AgentVersion
				in.ActualVersions = old.ActualVersions
				in.Reported = old.Reported
				out = in
				if e := tx.Save(&in).Error; e != nil {
					return e
				}
			case "endpoints":
				var in struct {
					Endpoint
					Password string `json:"password"`
				}
				if json.Unmarshal(raw, &in) != nil {
					return errInvalid
				}
				if e := checkName(in.Name, "端点名称"); e != nil {
					return e
				}
				if e := checkHost(in.Host, "服务地址"); e != nil {
					return e
				}
				if in.Port < 1 || in.Port > 65535 {
					return badRequest("起始端口须在 1–65535 之间")
				}
				if in.Capabilities.Managed && in.Port < 1024 {
					return badRequest("平台管理的端点，起始端口不能低于 1024")
				}
				if in.PortEnd < in.Port || in.PortEnd > 65535 {
					return badRequest("结束端口须不小于起始端口，且不超过 65535")
				}
				var n Node
				if tx.First(&n, "id = ?", in.NodeID).Error != nil {
					return badRequest("所属节点不存在，请重新选择")
				}
				old, _ := load[Endpoint](tx, id)
				in.ID = id
				in.CredentialID = old.CredentialID
				in.OwnerID = old.OwnerID
				if in.Password != "" {
					if !validKey(in.Cipher, in.Password) {
						return badRequest("线路密钥与加密方法不匹配：AES-128 需要 16 字节、AES-256 需要 32 字节的 Base64 密钥")
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
					return badRequest("服务类型只能是 SS2022 落地或 Realm 中转")
				}
				out = in.Endpoint
				if e := tx.Save(&in.Endpoint).Error; e != nil {
					return e
				}
			case "lines":
				var in Line
				if json.Unmarshal(raw, &in) != nil {
					return errInvalid
				}
				if e := checkName(in.Name, "线路名称"); e != nil {
					return e
				}
				if in.Capacity < 1 || in.Capacity > 10000 {
					return badRequest("并发容量须在 1–10000 之间")
				}
				for _, endpointID := range in.EndpointIDs {
					if strings.TrimSpace(endpointID) == "" {
						return badRequest("线路顺序里有未选择的服务端点")
					}
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
				if json.Unmarshal(raw, &in) != nil {
					return errInvalid
				}
				// Login trims the name it is given; a stored name with
				// surrounding spaces could never sign in.
				in.Username = strings.TrimSpace(in.Username)
				if e := checkName(in.Username, "账号"); e != nil {
					return e
				}
				if strings.ContainsAny(in.Username, " \t\r\n") {
					return badRequest("账号不能包含空格")
				}
				if !slices.Contains([]string{"admin", "operator", "publisher", "viewer", "player"}, in.Role) {
					return badRequest("请选择权限")
				}
				if in.MaxSessions < 1 || in.MaxSessions > 20 {
					return badRequest("并发会话上限须在 1–20 之间")
				}
				old, e := load[User](tx, id)
				if e != nil && in.Password == "" {
					return badRequest("新用户需要设置密码")
				}
				staff := in.Role != "player"
				if staff && in.Password != "" && utf8.RuneCountInString(in.Password) < staffPasswordMin {
					return badRequest("管理和运维账号的密码至少 " + strconv.Itoa(staffPasswordMin) + " 位")
				}
				// A player's password may be shorter than staff accounts allow.
				if staff && in.Password == "" && e == nil && old.Role == "player" {
					return badRequest("把玩家改为管理或运维角色时，请同时设置至少 " + strconv.Itoa(staffPasswordMin) + " 位的新密码")
				}
				if e != nil {
					old.CreatedAt = time.Now().UTC()
				}
				if in.Password != "" {
					old.PasswordHash = string(passwordHash)
					if e := tx.Where("user_id = ?", id).Delete(&Token{}).Error; e != nil {
						return e
					}
				}
				var clash int64
				if e := tx.Model(&User{}).Where("username = ? AND id <> ?", in.Username, id).Count(&clash).Error; e != nil {
					return e
				}
				if clash > 0 {
					return badRequest("账号已存在")
				}
				if id == actor(r).ID && (!in.Enabled || in.Role != "admin") {
					return badRequest("不能停用自己的账号，也不能取消自己的管理员权限")
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
					return badRequest("请选择来源（共享、独享或自带落地）")
				}
				if _, e := load[User](tx, in.UserID); e != nil {
					return badRequest("用户不存在，请重新选择")
				}
				if _, e := load[Endpoint](tx, in.EndpointID); e != nil {
					return badRequest("落地端点不存在，请重新选择")
				}
				if in.Kind == "dedicated" {
					var n int64
					tx.Model(&Binding{}).Where("endpoint_id = ? AND enabled = ? AND id <> ?", in.EndpointID, true, id).Count(&n)
					if n > 0 {
						return badRequest("该落地端点已独享给其他用户")
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
				if e := checkName(in.Version, "版本号"); e != nil {
					return e
				}
				u, e := url.Parse(in.URL)
				if e != nil || u.Scheme != "https" || u.Host == "" {
					return badRequest("下载地址必须是 https:// 开头的完整地址")
				}
				if _, e := hex.DecodeString(in.SHA256); e != nil || len(in.SHA256) != 64 {
					return badRequest("安装包 SHA256 应为 64 位十六进制字符")
				}
				if strings.TrimSpace(in.Signature) == "" {
					return badRequest("请填写可信发布清单签名")
				}
				in.ID = id
				// Editing an old release must not make it look newer.
				in.CreatedAt = time.Now().UTC()
				if old, e := load[Release](tx, id); e == nil {
					in.CreatedAt = old.CreatedAt
				}
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
	v := []Game{}
	e := s.Store.DB.Where("enabled = ?", true).Order("id").Limit(1000).Find(&v).Error
	respond(w, v, e)
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
		fail(w, 400, "设备信息不完整")
		return
	}
	out := Device{ID: in.ID, UserID: actor(r).ID, Name: in.Name, Capabilities: in.Capabilities, LastSeen: timePointer(time.Now().UTC())}
	e := s.Store.Tx(func(tx *gorm.DB) error {
		old, e := load[Device](tx, in.ID)
		if e == nil && old.UserID != out.UserID {
			return badRequest("此设备已绑定到其他账号")
		}
		return tx.Save(&out).Error
	})
	respond(w, out, e)
}

// clientLines lists the lines a player may use for a game and region. Rows
// are loaded in a fixed number of batched queries, independent of how many
// lines, hops and nodes there are.
func (s *Server) clientLines(w http.ResponseWriter, r *http.Request) {
	db, u := s.Store.DB, actor(r)
	region, game := r.URL.Query().Get("region_id"), r.URL.Query().Get("game_id")
	var rows []Line
	if e := db.Where("draining = ?", false).Find(&rows).Error; e != nil {
		respond(w, nil, e)
		return
	}
	versionIDs := []string{}
	for _, l := range rows {
		if permitted(l, u) && l.PublishedVersion != "" {
			versionIDs = append(versionIDs, l.PublishedVersion)
		}
	}
	var versionRows []LineVersion
	var counts []struct {
		LineID string
		Total  int64
	}
	if e := db.Where("id IN ? AND status = ?", versionIDs, "ready").Find(&versionRows).Error; e != nil {
		respond(w, nil, e)
		return
	}
	if e := db.Model(&Session{}).Select("line_id, count(*) as total").Where("state <> ? AND purpose = ?", "CLOSED", "game").Group("line_id").Scan(&counts).Error; e != nil {
		respond(w, nil, e)
		return
	}
	versions, active := map[string]LineVersion{}, map[string]int64{}
	exitIDs, nodeIDs := []string{}, []string{}
	for _, v := range versionRows {
		versions[v.ID] = v
		if len(v.Hops) > 0 {
			exitIDs = append(exitIDs, v.Hops[len(v.Hops)-1].EndpointID)
		}
		for _, h := range v.Hops {
			if h.Capabilities.Managed {
				nodeIDs = append(nodeIDs, h.NodeID)
			}
		}
	}
	for _, c := range counts {
		active[c.LineID] = c.Total
	}
	allowed, e := egressAllowedBatch(db, exitIDs, u.ID)
	if e != nil {
		respond(w, nil, e)
		return
	}
	var nodeRows []Node
	if e := db.Where("id IN ?", nodeIDs).Find(&nodeRows).Error; e != nil {
		respond(w, nil, e)
		return
	}
	online := map[string]bool{}
	for _, n := range nodeRows {
		online[n.ID] = n.Enabled && timeSince(n.LastSeen) < s.Config.NodeTimeout
	}
	out := []map[string]any{}
	for _, l := range rows {
		v, ok := versions[l.PublishedVersion]
		if !permitted(l, u) || !ok || len(v.Hops) == 0 || !allowed[v.Hops[len(v.Hops)-1].EndpointID] {
			continue
		}
		if region != "" && !slices.Contains(v.Regions, region) || game != "" && len(v.GameIDs) > 0 && !slices.Contains(v.GameIDs, game) {
			continue
		}
		count := active[l.ID]
		healthy := count < int64(l.Capacity)
		for _, h := range v.Hops {
			if h.Capabilities.Managed {
				healthy = healthy && online[h.NodeID]
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
				return badRequest("运营方未开放自备落地注册")
			}
			if !validHost(in.Host) || in.Port < 1 || in.Port > 65535 || !validKey(in.Cipher, in.Password) || !nameOK(in.Name) {
				return errInvalid
			}
			if !publicHost(in.Host) {
				return badRequest("自备落地必须是公网地址或域名")
			}
			if e := resolvesPublic(in.Host); e != nil {
				return e
			}
			var owned int64
			if e := tx.Model(&Binding{}).Where("user_id = ? AND kind = ?", u.ID, "byo").Count(&owned).Error; e != nil {
				return e
			}
			if owned >= maxOwnedEgress {
				return badRequest("自备落地数量已达上限，请先删除一个")
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
			if e == nil && v.Status == "ready" && len(v.Hops) > 0 && v.Hops[len(v.Hops)-1].EndpointID == in.EndpointID && egressAllowed(tx, in.EndpointID, u.ID) {
				now := time.Now().UTC()
				out = Binding{ID: ID(), UserID: u.ID, EndpointID: in.EndpointID, Kind: "shared", Enabled: true, VerifiedAt: &now, ExpectedExit: v.Probe.ExitIP}
				return tx.Create(&out).Error
			}
		}
		return badRequest("该落地不对当前账号开放")
	})
	respond(w, out, err)
}

// latestRelease returns the highest published stable version, so a hotfix
// for an older line published later never downgrades clients.
func (s *Server) latestRelease(w http.ResponseWriter, r *http.Request) {
	var rows []Release
	if e := s.Store.DB.Where("published = ? AND channel = ?", true, "stable").Find(&rows).Error; e != nil {
		respond(w, nil, e)
		return
	}
	if len(rows) == 0 {
		respond(w, nil, gorm.ErrRecordNotFound)
		return
	}
	best := rows[0]
	for _, rel := range rows[1:] {
		if c := compareVersions(rel.Version, best.Version); c > 0 || c == 0 && rel.CreatedAt.After(best.CreatedAt) {
			best = rel
		}
	}
	write(w, 200, best)
}

// compareVersions orders dotted numeric versions such as 1.10.2 > 1.9.7.
// A leading "v" and any pre-release suffix after "-" or "+" are ignored.
func compareVersions(a, b string) int {
	parts := func(v string) []int {
		v = strings.TrimPrefix(strings.TrimSpace(v), "v")
		if i := strings.IndexAny(v, "-+"); i >= 0 {
			v = v[:i]
		}
		out := []int{}
		for _, field := range strings.Split(v, ".") {
			n, _ := strconv.Atoi(field)
			out = append(out, n)
		}
		return out
	}
	x, y := parts(a), parts(b)
	for i := 0; i < max(len(x), len(y)); i++ {
		var p, q int
		if i < len(x) {
			p = x[i]
		}
		if i < len(y) {
			q = y[i]
		}
		if p != q {
			if p < q {
				return -1
			}
			return 1
		}
	}
	return 0
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
			return badRequest("发布前请至少配置一个区服")
		}
		if e = validateProfile(out.Draft); e != nil {
			return e
		}
		var maxVersion int
		tx.Model(&GameVersion{}).Where("game_id = ?", out.ID).Select("COALESCE(MAX(version),0)").Scan(&maxVersion)
		if raw := r.PathValue("version"); raw != "" {
			v, e := strconv.Atoi(raw)
			if e != nil || v < 1 {
				return badRequest("规则版本号不正确")
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
				return badRequest("该规则版本不存在")
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
				return badRequest("导入线路的验证需要一个在线的探测节点，请在控制服务配置 PROBE_NODE_ID")
			}
		}
		if s.Config.ProbeURL == "" || s.Config.ProbeUDP == "" {
			return badRequest("发布前请在控制服务配置 TCP 与 UDP 探测端点（PROBE_URL、PROBE_UDP）")
		}
		var pending int64
		tx.Model(&LineVersion{}).Where("line_id = ? AND status = ?", l.ID, "validating").Count(&pending)
		if pending > 0 {
			return badRequest("这条线路正在发布验证中，请等待结果")
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
			return badRequest("请选择这条线路已验证通过的版本")
		}
		// A validation finishing later would replace the version chosen here.
		var pending int64
		if e = tx.Model(&LineVersion{}).Where("line_id = ? AND status = ?", v.LineID, "validating").Count(&pending).Error; e != nil {
			return e
		}
		if pending > 0 {
			return badRequest("这条线路正在发布验证中，请先在“加速会话”里停止验证会话，再切换版本")
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
		if _, e := load[Line](tx, r.PathValue("id")); e != nil {
			return e
		}
		if e := tx.Model(&Line{}).Where("id = ?", r.PathValue("id")).Update("draining", true).Error; e != nil {
			return e
		}
		var rows []Session
		if e := tx.Where("line_id = ? AND state <> ?", r.PathValue("id"), "CLOSED").Find(&rows).Error; e != nil {
			return e
		}
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

// A player can register at most this many external exits.
const maxOwnedEgress = 5

// resolvesPublic rejects a hostname that points into internal address space,
// so a player cannot aim platform probes at the operator's own network.
func resolvesPublic(host string) error {
	if _, e := netip.ParseAddr(strings.Trim(host, "[]")); e != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		addrs, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if e != nil {
			return badRequest("自备落地的域名无法解析")
		}
		for _, a := range addrs {
			if !publicHost(a.Unmap().String()) {
				return badRequest("自备落地的域名解析到了内网地址")
			}
		}
	}
	return nil
}

// publicHost rejects loopback, private, link-local and other non-routable
// literals so a player cannot point platform probes at internal networks.
// Hostnames are accepted; the probe agent resolves them at validation time.
func publicHost(host string) bool {
	ip, e := netip.ParseAddr(strings.Trim(host, "[]"))
	if e != nil {
		lower := strings.ToLower(host)
		return lower != "localhost" && !strings.HasSuffix(lower, ".localhost") && !strings.HasSuffix(lower, ".local") && !strings.HasSuffix(lower, ".internal")
	}
	ip = ip.Unmap()
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !netip.MustParsePrefix("100.64.0.0/10").Contains(ip)
}
