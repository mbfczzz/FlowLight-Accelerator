package control

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// remove deletes a configuration row, refusing while anything still uses
// it and naming what to clear first. History that refers to it (sessions,
// traffic, audit) is kept; the audit entry records the name it had.
func (s *Server) remove(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		e := s.Store.Tx(func(tx *gorm.DB) error {
			var label string
			var e error
			switch kind {
			case "nodes":
				label, e = removeNode(tx, id)
			case "endpoints":
				label, e = removeEndpoint(tx, id)
			case "lines":
				label, e = removeLine(tx, id)
			case "games":
				label, e = removeGame(tx, id)
			case "bindings":
				label, e = removeBinding(tx, id)
			case "releases":
				label, e = removeRelease(tx, id)
			default:
				e = errInvalid
			}
			if e != nil {
				return e
			}
			return audit(tx, actor(r).ID, "resource.delete", id, label)
		})
		respond(w, map[string]bool{"ok": e == nil}, e)
	}
}

// names lists up to five names, then how many more.
func names(list []string) string {
	if len(list) <= 5 {
		return "「" + strings.Join(list, "」「") + "」"
	}
	return "「" + strings.Join(list[:5], "」「") + "」等 " + strconv.Itoa(len(list)) + " 个"
}

// liveSessions counts sessions not yet closed that match the query.
func liveSessions(tx *gorm.DB, query string, args ...any) (int64, error) {
	var n int64
	e := tx.Model(&Session{}).Where("state <> ?", "CLOSED").Where(query, args...).Count(&n).Error
	return n, e
}

func removeNode(tx *gorm.DB, id string) (string, error) {
	n, e := load[Node](tx, id)
	if e != nil {
		return "", e
	}
	label := "节点服务器「" + n.Name + "」"
	var endpoints []string
	if e = tx.Model(&Endpoint{}).Where("node_id = ?", id).Order("name").Pluck("name", &endpoints).Error; e != nil {
		return "", e
	}
	if len(endpoints) > 0 {
		return "", badRequest(label + "上还有服务端点" + names(endpoints) + "。请先在“服务端点”里删除它们")
	}
	var reserved int64
	if e = tx.Model(&Reservation{}).Where("node_id = ?", id).Count(&reserved).Error; e != nil {
		return "", e
	}
	if reserved > 0 {
		return "", badRequest(label + "上还有进行中的会话，请等会话结束后再删除")
	}
	// Its agent's token goes with the row, so the agent can no longer sign
	// in; work queued for it is moot.
	if e = tx.Model(&Job{}).Where("node_id = ? AND status IN ?", id, []string{"pending", "running"}).Update("status", "cancelled").Error; e != nil {
		return "", e
	}
	return label, tx.Delete(&Node{}, "id = ?", id).Error
}

// usesEndpoint reports whether a line version's path goes through it.
func usesEndpoint(tx *gorm.DB, versionID, endpointID string) bool {
	v, e := load[LineVersion](tx, versionID)
	if e != nil {
		return false
	}
	for _, h := range v.Hops {
		if h.EndpointID == endpointID {
			return true
		}
	}
	return false
}

// linesUsing lists lines whose path, draft or published, includes the
// endpoint. Paths are short JSON lists, so they are checked in Go.
func linesUsing(tx *gorm.DB, endpointID string) ([]string, error) {
	var lines []Line
	if e := tx.Find(&lines).Error; e != nil {
		return nil, e
	}
	out := []string{}
	for _, l := range lines {
		used := slices.Contains(l.EndpointIDs, endpointID)
		if !used && l.PublishedVersion != "" {
			used = usesEndpoint(tx, l.PublishedVersion, endpointID)
		}
		if used {
			out = append(out, l.Name)
		}
	}
	return out, nil
}

func removeEndpoint(tx *gorm.DB, id string) (string, error) {
	ep, e := load[Endpoint](tx, id)
	if e != nil {
		return "", e
	}
	label := "服务端点「" + ep.Name + "」"
	lines, e := linesUsing(tx, id)
	if e != nil {
		return "", e
	}
	if len(lines) > 0 {
		return "", badRequest(label + "正被线路" + names(lines) + "使用（含已发布的版本）。请先从这些线路的路径里移除它并重新发布，或删除这些线路")
	}
	// A version being verified, or one a player still holds after its line
	// moved on, can use it without the line showing it.
	var versions []string
	if e = tx.Model(&Session{}).Where("state <> ?", "CLOSED").Distinct().Pluck("line_version", &versions).Error; e != nil {
		return "", e
	}
	for _, v := range versions {
		if usesEndpoint(tx, v, id) {
			return "", badRequest(label + "上还有进行中的会话（含发布验证），请等会话结束后再删除")
		}
	}
	var bound int64
	if e = tx.Model(&Binding{}).Where("endpoint_id = ?", id).Count(&bound).Error; e != nil {
		return "", e
	}
	if bound > 0 {
		return "", badRequest(label + "还有 " + strconv.FormatInt(bound, 10) + " 个落地绑定，请先在“落地绑定”里删除它们")
	}
	if e = tx.Delete(&Endpoint{}, "id = ?", id).Error; e != nil {
		return "", e
	}
	if ep.CredentialID != "" {
		return label, tx.Delete(&Credential{}, "id = ?", ep.CredentialID).Error
	}
	return label, nil
}

func removeLine(tx *gorm.DB, id string) (string, error) {
	l, e := load[Line](tx, id)
	if e != nil {
		return "", e
	}
	label := "线路「" + l.Name + "」"
	n, e := liveSessions(tx, "line_id = ?", id)
	if e != nil {
		return "", e
	}
	if n > 0 {
		return "", badRequest(label + "上还有 " + strconv.FormatInt(n, 10) + " 个进行中的会话（含发布验证）。请先在“更多”里终止线路，等会话结束后再删除")
	}
	if e = tx.Delete(&LineCheck{}, "line_id = ?", id).Error; e != nil {
		return "", e
	}
	// Its versions stay: past sessions and their traffic refer to them.
	return label, tx.Delete(&Line{}, "id = ?", id).Error
}

func removeGame(tx *gorm.DB, id string) (string, error) {
	g, e := load[Game](tx, id)
	if e != nil {
		return "", e
	}
	label := "游戏「" + g.Name + "」"
	var lines []Line
	if e = tx.Find(&lines).Error; e != nil {
		return "", e
	}
	using := []string{}
	for _, l := range lines {
		used := slices.Contains(l.GameIDs, id)
		if !used && l.PublishedVersion != "" {
			if v, e := load[LineVersion](tx, l.PublishedVersion); e == nil {
				used = slices.Contains(v.GameIDs, id)
			}
		}
		if used {
			using = append(using, l.Name)
		}
	}
	if len(using) > 0 {
		return "", badRequest("线路" + names(using) + "限定了" + label + "（含已发布的版本）。请先在这些线路的“限定游戏”里移除它并重新发布")
	}
	n, e := liveSessions(tx, "game_id = ?", id)
	if e != nil {
		return "", e
	}
	if n > 0 {
		return "", badRequest("还有 " + strconv.FormatInt(n, 10) + " 个会话正在加速" + label + "，请等会话结束后再删除")
	}
	if e = tx.Delete(&GameVersion{}, "game_id = ?", id).Error; e != nil {
		return "", e
	}
	if e = tx.Delete(&Game{}, "id = ?", id).Error; e != nil {
		return "", e
	}
	return label, dropCoverIfUnused(tx, g.CoverImage)
}

func removeBinding(tx *gorm.DB, id string) (string, error) {
	if _, e := load[Binding](tx, id); e != nil {
		return "", e
	}
	n, e := liveSessions(tx, "binding_id = ?", id)
	if e != nil {
		return "", e
	}
	if n > 0 {
		return "", badRequest("还有 " + strconv.FormatInt(n, 10) + " 个进行中的会话在使用这个落地绑定，请等会话结束后再删除")
	}
	return "落地绑定", tx.Delete(&Binding{}, "id = ?", id).Error
}

func removeRelease(tx *gorm.DB, id string) (string, error) {
	rel, e := load[Release](tx, id)
	if e != nil {
		return "", e
	}
	return "客户端版本 " + rel.Version, tx.Delete(&Release{}, "id = ?", id).Error
}
