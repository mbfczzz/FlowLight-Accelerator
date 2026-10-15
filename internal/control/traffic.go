package control

import (
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// Traffic as the exit node measured it (internal/agent/usage.go): upload
// and download together. A session's running total is Session.Bytes;
// TrafficHour splits it by UTC hour with the player, line, game and exit
// node, for the admin's statistics.

// TrafficHour is the traffic one session used in one hour.
type TrafficHour struct {
	Hour      time.Time `json:"hour" gorm:"column:hour_start;primaryKey"`
	SessionID string    `json:"session_id" gorm:"primaryKey;size:64"`
	UserID    string    `json:"user_id" gorm:"index;size:64"`
	LineID    string    `json:"line_id" gorm:"size:64"`
	GameID    string    `json:"game_id" gorm:"size:64"`
	NodeID    string    `json:"node_id" gorm:"size:64"`
	Bytes     int64     `json:"bytes"`
}

// nodeUsage takes a node's running totals. Only the exit node of a
// session's line counts it, and only growth is recorded, so a total sent
// again (a lost response, an agent restart) adds nothing.
func (s *Server) nodeUsage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Usage []struct {
			RuntimeID string `json:"runtime_id"`
			SessionID string `json:"session_id"`
			Bytes     int64  `json:"bytes"`
		} `json:"usage"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Usage) > 500 {
		fail(w, 400, "too many usage entries")
		return
	}
	node := r.Context().Value(nodeKey).(Node)
	hour := time.Now().UTC().Truncate(time.Hour)
	e := s.Store.Tx(func(tx *gorm.DB) error {
		for _, u := range in.Usage {
			if u.Bytes <= 0 || !safeSessionID(u.SessionID) {
				continue
			}
			var x Session
			if tx.First(&x, "id = ?", u.SessionID).Error != nil {
				continue
			}
			var v LineVersion
			if tx.First(&v, "id = ?", x.LineVersion).Error != nil || len(v.Hops) == 0 || v.Hops[len(v.Hops)-1].NodeID != node.ID {
				continue
			}
			delta := u.Bytes - x.Bytes
			if delta <= 0 {
				continue
			}
			if e := tx.Model(&Session{}).Where("id = ?", x.ID).UpdateColumn("bytes", gorm.Expr("bytes + ?", delta)).Error; e != nil {
				return e
			}
			// Verification sessions are the platform's own probes, not use.
			if x.Purpose != "game" {
				continue
			}
			res := tx.Model(&TrafficHour{}).Where("hour_start = ? AND session_id = ?", hour, x.ID).UpdateColumn("bytes", gorm.Expr("bytes + ?", delta))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				row := TrafficHour{Hour: hour, SessionID: x.ID, UserID: x.UserID, LineID: x.LineID, GameID: x.GameID, NodeID: node.ID, Bytes: delta}
				if e := tx.Create(&row).Error; e != nil {
					return e
				}
			}
		}
		return nil
	})
	respond(w, map[string]any{"ok": true}, e)
}

func safeSessionID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

type trafficShare struct {
	ID       string `json:"id"`
	Bytes    int64  `json:"bytes"`
	Sessions int64  `json:"sessions"`
}

type trafficPoint struct {
	Start time.Time `json:"start"`
	Bytes int64     `json:"bytes"`
}

// traffic answers the statistics page: ?days=1..90 local days ending today
// (1 = today, by hour), in the browser's zone (?tz= minutes east of UTC).
func (s *Server) traffic(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days < 1 || days > 90 {
		days = 7
	}
	tz, e := strconv.Atoi(r.URL.Query().Get("tz"))
	if e != nil || tz < -720 || tz > 840 {
		tz = 0
	}
	loc := time.FixedZone("", tz*60)
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(days - 1))
	from := start.UTC()
	db := s.Store.DB.Model(&TrafficHour{}).Where("hour_start >= ?", from)

	var hours []struct {
		HourStart time.Time
		Bytes     int64
	}
	if e := db.Session(&gorm.Session{}).Select("hour_start, SUM(bytes) AS bytes").Group("hour_start").Scan(&hours).Error; e != nil {
		respond(w, nil, e)
		return
	}
	// One bucket per local hour for today, per local day otherwise; empty
	// buckets are kept so the chart's axis is continuous.
	count := days
	if days == 1 {
		count = 24
	}
	series := make([]trafficPoint, count)
	for i := range series {
		if days == 1 {
			series[i].Start = start.Add(time.Duration(i) * time.Hour)
		} else {
			series[i].Start = start.AddDate(0, 0, i)
		}
	}
	for _, h := range hours {
		local := h.HourStart.In(loc)
		var i int
		if days == 1 {
			i = int(local.Sub(start) / time.Hour)
		} else {
			d := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
			i = int(d.Sub(start) / (24 * time.Hour))
		}
		if i >= 0 && i < count {
			series[i].Bytes += h.Bytes
		}
	}

	var totals struct {
		Bytes    int64 `json:"bytes"`
		Sessions int64 `json:"sessions"`
		Users    int64 `json:"users"`
	}
	if e := db.Session(&gorm.Session{}).Select("COALESCE(SUM(bytes), 0) AS bytes, COUNT(DISTINCT session_id) AS sessions, COUNT(DISTINCT user_id) AS users").Scan(&totals).Error; e != nil {
		respond(w, nil, e)
		return
	}
	group := func(col string) ([]trafficShare, error) {
		out := []trafficShare{}
		e := db.Session(&gorm.Session{}).Select(col + " AS id, SUM(bytes) AS bytes, COUNT(DISTINCT session_id) AS sessions").
			Group(col).Order("bytes DESC").Limit(10).Scan(&out).Error
		return out, e
	}
	out := map[string]any{"from": from, "days": days, "tz": tz, "series": series, "totals": totals}
	for key, col := range map[string]string{"users": "user_id", "lines": "line_id", "games": "game_id", "nodes": "node_id"} {
		rows, e := group(col)
		if e != nil {
			respond(w, nil, e)
			return
		}
		out[key] = rows
	}
	write(w, 200, out)
}
