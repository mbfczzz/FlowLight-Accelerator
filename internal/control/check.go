package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	mrand "math/rand/v2"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"gorm.io/gorm"
)

// LineCheck is one run of a line's connectivity check: every node on the
// path players use and every link between them, measured by the nodes
// themselves (packages/network/check.go).
//
// It runs in two steps. First each platform-run endpoint's node opens a
// short-lived test listener on a free port of that endpoint's range, so the
// firewall rules sessions need are the ones tested. Then each node dials the
// next one's listener (or, for an endpoint the platform does not run, its
// service), the exit reaches the probe service, and the control service
// dials the entry, as a player on the internet would.
type LineCheck struct {
	ID        string      `json:"id" gorm:"primaryKey;size:64"`
	LineID    string      `json:"line_id" gorm:"index;size:64"`
	VersionID string      `json:"version_id" gorm:"size:64"`
	Version   int         `json:"version"`
	Phase     string      `json:"phase" gorm:"size:16"`
	Nodes     []checkNode `json:"nodes" gorm:"serializer:json;type:text"`
	Links     []checkLink `json:"links" gorm:"serializer:json;type:text"`
	// The path as it was when the check began, and each hop's listener token.
	Hops       []c.Hop    `json:"-" gorm:"serializer:json;type:text"`
	Tokens     []string   `json:"-" gorm:"serializer:json;type:text"`
	ActorID    string     `json:"actor_id" gorm:"size:64"`
	CreatedAt  time.Time  `json:"created_at" gorm:"index"`
	PhaseAt    time.Time  `json:"phase_at"`
	FinishedAt *time.Time `json:"finished_at" gorm:"default:null"`
}

// checkNode is one hop: its endpoint, the node it runs on, and its test
// listener: pending, ok, failed, or skipped when the platform does not run
// the endpoint and its own service is dialled instead.
type checkNode struct {
	Role         string     `json:"role"`
	EndpointID   string     `json:"endpoint_id"`
	EndpointName string     `json:"endpoint_name"`
	Kind         string     `json:"kind"`
	Address      string     `json:"address"`
	Managed      bool       `json:"managed"`
	UDP          bool       `json:"udp"`
	NodeID       string     `json:"node_id"`
	NodeName     string     `json:"node_name"`
	Online       bool       `json:"online"`
	LastSeen     *time.Time `json:"last_seen"`
	AgentVersion string     `json:"agent_version"`
	Listen       string     `json:"listen"`
	ListenError  string     `json:"listen_error,omitempty"`
	Port         int        `json:"port,omitempty"`
	// Silent: the node never took up its listener task, so it is not
	// asked to measure either.
	Silent bool `json:"silent,omitempty"`
}

// checkLink is one step of the path. From -1 is the control service; To
// len(hops) is the internet. Status is pending, ok, warn (some samples
// lost), failed, or skipped; Note says why a link was not measured.
type checkLink struct {
	ID          string          `json:"id"`
	From        int             `json:"from"`
	To          int             `json:"to"`
	UDPExpected bool            `json:"udp_expected"`
	Status      string          `json:"status"`
	TCP         *c.CheckMeasure `json:"tcp,omitempty"`
	UDP         *c.CheckMeasure `json:"udp,omitempty"`
	ExitIP      string          `json:"exit_ip,omitempty"`
	Note        string          `json:"note,omitempty"`
}

const (
	// Agents ask for work every 2 s; a listener not up by then is not coming.
	checkListenWait = 15 * time.Second
	checkDialWait   = 45 * time.Second
	// Listeners outlive both steps, then close on their own.
	checkListenSeconds = 90
	checkPortChoices   = 8
	// A node that comes back later has nothing left to measure.
	checkJobLife = time.Minute
)

// checkPath is what players use: the published version, or the draft path
// while nothing is published.
func checkPath(tx *gorm.DB, l Line) ([]c.Hop, string, int, error) {
	if l.PublishedVersion != "" {
		if v, e := load[LineVersion](tx, l.PublishedVersion); e == nil && len(v.Hops) > 0 {
			return v.Hops, v.ID, v.Number, nil
		}
	}
	if len(l.EndpointIDs) == 0 {
		return nil, "", 0, badRequest("这条线路还没有选择服务端点，没有可以检查的路径")
	}
	hops := []c.Hop{}
	for _, id := range l.EndpointIDs {
		ep, e := load[Endpoint](tx, id)
		if e != nil {
			return nil, "", 0, badRequest("线路的路径里有已经删除的服务端点，请先编辑线路")
		}
		hops = append(hops, c.Hop{EndpointID: ep.ID, NodeID: ep.NodeID, Kind: ep.Kind, Host: ep.Host, Port: ep.Port, PortEnd: ep.PortEnd, Capabilities: ep.Capabilities})
	}
	return hops, "", 0, nil
}

func hopRole(i, n int) string {
	switch {
	case n == 1:
		return "入口兼落地"
	case i == 0:
		return "入口"
	case i == n-1:
		return "落地"
	}
	return "中转"
}

func (s *Server) online(n Node) bool {
	return n.Enabled && n.LastSeen != nil && timeSince(n.LastSeen) <= s.Config.NodeTimeout
}

func offlineReason(n Node) string {
	switch {
	case n.ID == "":
		return "节点服务器已被删除"
	case !n.Enabled:
		return "节点服务器「" + n.Name + "」已停用"
	case n.LastSeen == nil:
		return "节点服务器「" + n.Name + "」还没有接入：请在“节点服务器”里点“接入命令”，在这台服务器上运行"
	}
	return "节点服务器「" + n.Name + "」不在线（超过 45 秒没有心跳），请检查服务器上的节点程序（systemctl status flowlight-node）"
}

// checkPorts picks free ports of a hop's range, in random order so a port
// a session is about to take is unlikely to be first.
func checkPorts(tx *gorm.DB, h c.Hop) ([]int, error) {
	end := max(h.PortEnd, h.Port)
	var taken []int
	if e := tx.Model(&Reservation{}).Where("node_id = ? AND port BETWEEN ? AND ?", h.NodeID, h.Port, end).Pluck("port", &taken).Error; e != nil {
		return nil, e
	}
	free := []int{}
	for p := h.Port; p <= end && p <= 65535; p++ {
		if p > 0 && !slices.Contains(taken, p) {
			free = append(free, p)
		}
	}
	mrand.Shuffle(len(free), func(i, j int) { free[i], free[j] = free[j], free[i] })
	return free[:min(len(free), checkPortChoices)], nil
}

func checkToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) checkJob(tx *gorm.DB, k *LineCheck, hop int, kind string, task c.CheckTask) error {
	cipher, e := s.Store.Seal(c.JobPayload{Check: &task})
	if e != nil {
		return e
	}
	return tx.Create(&Job{ID: ID(), NodeID: k.Hops[hop].NodeID, CheckID: k.ID, Kind: kind, PayloadCipher: cipher, Status: "pending", CreatedAt: time.Now().UTC()}).Error
}

// startCheck begins a check of a line, or returns the one under way: two at
// once would race for the same test ports.
func (s *Server) startCheck(w http.ResponseWriter, r *http.Request) {
	var out LineCheck
	var after []func()
	e := s.Store.Tx(func(tx *gorm.DB) error {
		l, e := load[Line](tx, r.PathValue("id"))
		if e != nil {
			return e
		}
		if tx.Where("line_id = ? AND phase <> ?", l.ID, "done").Order("created_at desc").First(&out).Error == nil {
			after, e = s.advanceCheck(tx, &out)
			return e
		}
		hops, versionID, number, e := checkPath(tx, l)
		if e != nil {
			return e
		}
		now := time.Now().UTC()
		k := LineCheck{ID: ID(), LineID: l.ID, VersionID: versionID, Version: number, Phase: "listen", Hops: hops, Tokens: make([]string, len(hops)), ActorID: actor(r).ID, CreatedAt: now, PhaseAt: now}
		nodes := map[int]Node{}
		for i, h := range hops {
			var n Node
			tx.First(&n, "id = ?", h.NodeID)
			nodes[i] = n
			ep, _ := load[Endpoint](tx, h.EndpointID)
			address := net.JoinHostPort(h.Host, strconv.Itoa(h.Port))
			if h.PortEnd > h.Port {
				address += "-" + strconv.Itoa(h.PortEnd)
			}
			k.Nodes = append(k.Nodes, checkNode{Role: hopRole(i, len(hops)), EndpointID: h.EndpointID, EndpointName: ep.Name, Kind: h.Kind, Address: address, Managed: h.Capabilities.Managed, UDP: h.Capabilities.UDP, NodeID: h.NodeID, NodeName: n.Name, Online: s.online(n), LastSeen: n.LastSeen, AgentVersion: n.AgentVersion, Listen: "skipped"})
		}
		// UDP is measured only where a check listener answers it.
		last := len(hops) - 1
		k.Links = append(k.Links, checkLink{ID: "public", From: -1, To: 0, UDPExpected: hops[0].Capabilities.Managed && hops[0].Capabilities.UDP, Status: "pending"})
		for i := 0; i < last; i++ {
			k.Links = append(k.Links, checkLink{ID: "link-" + strconv.Itoa(i), From: i, To: i + 1, UDPExpected: hops[i+1].Capabilities.Managed && hops[i].Capabilities.UDP && hops[i+1].Capabilities.UDP, Status: "pending"})
		}
		internet := checkLink{ID: "internet", From: last, To: len(hops), UDPExpected: s.Config.ProbeUDP != "" && hops[last].Capabilities.UDP, Status: "pending"}
		if s.Config.ProbeURL == "" {
			internet.Status, internet.Note = "skipped", "控制服务没有配置探测服务（PROBE_URL），无法检测落地的互联网出口"
		}
		k.Links = append(k.Links, internet)

		for i, h := range hops {
			if !h.Capabilities.Managed {
				continue
			}
			node := &k.Nodes[i]
			if !node.Online {
				node.Listen, node.ListenError = "failed", offlineReason(nodes[i])
				continue
			}
			ports, e := checkPorts(tx, h)
			if e != nil {
				return e
			}
			if len(ports) == 0 {
				node.Listen, node.ListenError = "failed", "端口范围内的端口都在被会话使用，暂时无法开启检测监听"
				continue
			}
			node.Listen = "pending"
			k.Tokens[i] = checkToken()
			if e = s.checkJob(tx, &k, i, "check-listen", c.CheckTask{Hop: i, Ports: ports, UDP: h.Capabilities.UDP, Seconds: checkListenSeconds, Token: k.Tokens[i]}); e != nil {
				return e
			}
		}
		if e = tx.Create(&k).Error; e != nil {
			return e
		}
		if e = audit(tx, actor(r).ID, "line.check", l.ID, k.ID); e != nil {
			return e
		}
		after, e = s.advanceCheck(tx, &k)
		out = k
		return e
	})
	for _, f := range after {
		go f()
	}
	respond(w, out, e)
}

// lineCheck returns a line's latest check, or null, moving a check whose
// step has run out of time along.
func (s *Server) lineCheck(w http.ResponseWriter, r *http.Request) {
	var out *LineCheck
	var after []func()
	e := s.Store.Tx(func(tx *gorm.DB) error {
		var k LineCheck
		if e := tx.Where("line_id = ?", r.PathValue("id")).Order("created_at desc").First(&k).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return nil
			}
			return e
		}
		var e error
		after, e = s.advanceCheck(tx, &k)
		out = &k
		return e
	})
	for _, f := range after {
		go f()
	}
	respond(w, out, e)
}

// advanceCheck moves a check to its next step once the current one is
// complete or out of time. What it returns runs after the transaction: the
// control service's own measurement, which must not hold the store.
func (s *Server) advanceCheck(tx *gorm.DB, k *LineCheck) ([]func(), error) {
	now := time.Now().UTC()
	var after []func()
	waiting := func(kind string) int64 {
		var n int64
		tx.Model(&Job{}).Where("check_id = ? AND kind = ? AND status IN ?", k.ID, kind, []string{"pending", "running"}).Count(&n)
		return n
	}
	switch k.Phase {
	case "listen":
		if waiting("check-listen") > 0 && now.Sub(k.PhaseAt) < checkListenWait {
			return nil, nil
		}
		for i := range k.Nodes {
			if k.Nodes[i].Listen == "pending" {
				k.Nodes[i].Listen, k.Nodes[i].Silent = "failed", true
				k.Nodes[i].ListenError = "节点没有在 15 秒内开启检测监听：节点程序可能没有运行，或连不上控制服务"
			}
		}
		if e := tx.Model(&Job{}).Where("check_id = ? AND kind = ? AND status IN ?", k.ID, "check-listen", []string{"pending", "running"}).Update("status", "cancelled").Error; e != nil {
			return nil, e
		}
		var e error
		if after, e = s.dialPhase(tx, k); e != nil {
			return nil, e
		}
		k.Phase, k.PhaseAt = "dial", now
	case "dial":
		pending := slices.ContainsFunc(k.Links, func(l checkLink) bool { return l.Status == "pending" })
		if pending && now.Sub(k.PhaseAt) < checkDialWait {
			return nil, nil
		}
		for i := range k.Links {
			if k.Links[i].Status == "pending" {
				k.Links[i].Status, k.Links[i].Note = "failed", "没有在规定时间内收到这一段的检测结果"
			}
		}
		if e := tx.Model(&Job{}).Where("check_id = ? AND status IN ?", k.ID, []string{"pending", "running"}).Update("status", "cancelled").Error; e != nil {
			return nil, e
		}
		k.Phase, k.FinishedAt = "done", timePointer(now)
	default:
		return nil, nil
	}
	if e := tx.Save(k).Error; e != nil {
		return nil, e
	}
	// The next step may have nothing to wait for.
	more, e := s.advanceCheck(tx, k)
	return append(after, more...), e
}

// dialPhase hands each node the links that start at it, and returns the
// control service's own dial of the entry.
func (s *Server) dialPhase(tx *gorm.DB, k *LineCheck) ([]func(), error) {
	// target is the hop as a link reaches it, or why it cannot be reached.
	target := func(l checkLink) (c.CheckTarget, string) {
		if l.To == len(k.Hops) {
			t := c.CheckTarget{ID: l.ID, URL: s.Config.ProbeURL}
			if l.UDPExpected {
				t.UDPTarget = s.Config.ProbeUDP
			}
			return t, ""
		}
		h, node := k.Hops[l.To], k.Nodes[l.To]
		if !h.Capabilities.Managed {
			return c.CheckTarget{ID: l.ID, Host: h.Host, Port: h.Port}, ""
		}
		if node.Listen != "ok" {
			return c.CheckTarget{}, "「" + node.EndpointName + "」没有开启检测端口，这一段无法测量"
		}
		return c.CheckTarget{ID: l.ID, Host: h.Host, Port: node.Port, Token: k.Tokens[l.To], UDP: l.UDPExpected}, ""
	}
	var after []func()
	tasks := map[int][]c.CheckTarget{}
	for i := range k.Links {
		l := &k.Links[i]
		if l.Status != "pending" {
			continue
		}
		t, reason := target(*l)
		if reason == "" && l.From >= 0 && !k.Nodes[l.From].Online {
			var n Node
			tx.First(&n, "id = ?", k.Nodes[l.From].NodeID)
			reason = "无法从这里发起检测：" + offlineReason(n)
		}
		if reason == "" && l.From >= 0 && k.Nodes[l.From].Silent {
			reason = "无法从这里发起检测：「" + k.Nodes[l.From].NodeName + "」上的节点程序没有响应检测任务"
		}
		if reason != "" {
			l.Status, l.Note = "failed", reason
			continue
		}
		if l.From < 0 {
			id := k.ID
			after = append(after, func() { s.controlDial(id, t) })
			continue
		}
		tasks[l.From] = append(tasks[l.From], t)
	}
	for hop, targets := range tasks {
		if e := s.checkJob(tx, k, hop, "check-dial", c.CheckTask{Hop: hop, Targets: targets}); e != nil {
			return nil, e
		}
	}
	return after, nil
}

// controlDial measures the entry from the control service.
func (s *Server) controlDial(id string, t c.CheckTarget) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result := s.checkDialer(ctx, t)
	var after []func()
	e := s.Store.Tx(func(tx *gorm.DB) error {
		k, e := load[LineCheck](tx, id)
		if e != nil || k.Phase != "dial" {
			return e
		}
		applyLink(&k, result)
		if e = tx.Save(&k).Error; e != nil {
			return e
		}
		after, e = s.advanceCheck(tx, &k)
		return e
	})
	if e != nil {
		log.Printf("connectivity check %s: %v", id, e)
	}
	for _, f := range after {
		go f()
	}
}

func applyLink(k *LineCheck, r c.CheckLinkResult) {
	for i := range k.Links {
		l := &k.Links[i]
		if l.ID != r.ID || l.Status != "pending" {
			continue
		}
		l.TCP, l.UDP, l.ExitIP = r.TCP, r.UDP, r.ExitIP
		if !l.UDPExpected {
			l.UDP = nil
		}
		switch {
		case l.TCP == nil || !l.TCP.OK:
			l.Status = "failed"
		case l.UDPExpected && (l.UDP == nil || !l.UDP.OK):
			l.Status = "failed"
		case l.TCP.Loss > 0 || (l.UDP != nil && l.UDP.Loss > 0):
			l.Status = "warn"
		default:
			l.Status = "ok"
		}
	}
}

// checkFailure says what a node's failed check task means for the admin.
func checkFailure(n Node, message string) string {
	switch {
	case strings.Contains(message, "unsupported structured task"):
		return "节点服务器「" + n.Name + "」的节点程序是旧版本（" + n.AgentVersion + "），不支持连通性检查。请在“节点服务器”里点“接入命令”，在这台服务器上运行不带凭据的同一命令来更新"
	case strings.Contains(message, "address already in use"):
		return "检测用的端口都被占用了（" + message + "）"
	case strings.Contains(message, "permission denied"):
		return "节点程序没有权限监听这个端口：1024 以下的端口需要额外权限（" + message + "）"
	case message == "":
		return "节点没有说明原因"
	}
	return message
}

// checkJobDone records a node's answer to its part of a check.
func (s *Server) checkJobDone(tx *gorm.DB, j Job, n Node, ok bool, message string, report *c.CheckReport) ([]func(), error) {
	k, e := load[LineCheck](tx, j.CheckID)
	if e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, e
	}
	var p c.JobPayload
	if e = s.Store.Unseal(j.PayloadCipher, &p); e != nil || p.Check == nil {
		return nil, e
	}
	task := *p.Check
	reason := checkFailure(n, message)
	switch {
	case j.Kind == "check-listen" && k.Phase == "listen" && task.Hop >= 0 && task.Hop < len(k.Nodes):
		node := &k.Nodes[task.Hop]
		if node.Listen != "pending" {
			break
		}
		if ok && report != nil && slices.Contains(task.Ports, report.Port) {
			node.Listen, node.Port = "ok", report.Port
		} else {
			if ok {
				reason = "节点没有报告它监听的端口"
			}
			node.Listen, node.ListenError = "failed", reason
		}
	case j.Kind == "check-dial" && k.Phase == "dial":
		for _, t := range task.Targets {
			if !ok || report == nil {
				failLink(&k, t.ID, reason)
				continue
			}
			found := false
			for _, r := range report.Links {
				if r.ID == t.ID {
					applyLink(&k, r)
					found = true
				}
			}
			if !found {
				failLink(&k, t.ID, "节点没有返回这一段的结果")
			}
		}
	default:
		return nil, nil
	}
	if e = tx.Save(&k).Error; e != nil {
		return nil, e
	}
	return s.advanceCheck(tx, &k)
}

func failLink(k *LineCheck, id, note string) {
	for i := range k.Links {
		if k.Links[i].ID == id && k.Links[i].Status == "pending" {
			k.Links[i].Status, k.Links[i].Note = "failed", note
		}
	}
}
