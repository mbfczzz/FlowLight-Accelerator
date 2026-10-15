package control

import (
	"encoding/json"
	"testing"
	"time"

	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
)

func TestNodeUsageCountsGrowthFromTheExitOnly(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	s.Store.DB.Create(&Node{ID: "node-2", Name: "relay", Address: "relay.example", Enabled: true, TokenHash: hash("relay-token")})
	s.Store.DB.Create(&LineVersion{ID: "lv-1", LineID: "line-x", Number: 1, Status: "ready", Hops: []c.Hop{{EndpointID: "relay", NodeID: "node-2"}, {EndpointID: "ep-1", NodeID: "node-1"}}})
	now := time.Now().UTC()
	s.Store.DB.Create(&Session{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", UserID: u.ID, GameID: "valorant", LineID: "line-x", LineVersion: "lv-1", Purpose: "game", State: "ACTIVE", CreatedAt: now, LeaseUntil: now.Add(time.Minute)})
	s.Store.DB.Create(&Session{ID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", UserID: u.ID, LineID: "line-x", LineVersion: "lv-1", Purpose: "validation", State: "ACTIVE", CreatedAt: now, LeaseUntil: now.Add(time.Minute)})

	report := func(nodeToken, session string, bytes int64) {
		t.Helper()
		w := request(s, nodeToken, "POST", "/api/node/v1/usage", map[string]any{"usage": []map[string]any{{"runtime_id": "cccccccccccccccccccccccccccccccc", "session_id": session, "bytes": bytes}}}, "")
		if w.Code != 200 {
			t.Fatalf("usage report: %d %s", w.Code, w.Body.String())
		}
	}
	bytesOf := func(id string) int64 {
		var x Session
		s.Store.DB.First(&x, "id = ?", id)
		return x.Bytes
	}
	hourly := func() (total int64) {
		var rows []TrafficHour
		s.Store.DB.Find(&rows)
		for _, r := range rows {
			total += r.Bytes
		}
		return
	}

	game := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	report("node-test-token", game, 1000)
	report("node-test-token", game, 1000) // the same total again: nothing new
	report("relay-token", game, 99999)    // a relay is not the exit: ignored
	if bytesOf(game) != 1000 || hourly() != 1000 {
		t.Fatalf("after a repeated total: session %d, hourly %d", bytesOf(game), hourly())
	}
	report("node-test-token", game, 3500)
	if bytesOf(game) != 3500 || hourly() != 3500 {
		t.Fatalf("after growth: session %d, hourly %d", bytesOf(game), hourly())
	}
	report("node-test-token", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 700)
	if bytesOf("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb") != 700 || hourly() != 3500 {
		t.Fatal("a verification session must count on itself but stay out of the statistics")
	}

	var week struct {
		Series []trafficPoint `json:"series"`
		Totals struct {
			Bytes    int64 `json:"bytes"`
			Sessions int64 `json:"sessions"`
			Users    int64 `json:"users"`
		} `json:"totals"`
		Users []trafficShare `json:"users"`
		Games []trafficShare `json:"games"`
		Nodes []trafficShare `json:"nodes"`
	}
	w := request(s, token, "GET", "/api/admin/v1/traffic?days=7&tz=480", nil, "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &week) != nil {
		t.Fatalf("traffic: %d %s", w.Code, w.Body.String())
	}
	if len(week.Series) != 7 || week.Series[6].Bytes != 3500 || week.Totals.Bytes != 3500 || week.Totals.Sessions != 1 || week.Totals.Users != 1 {
		t.Fatalf("weekly view wrong: %+v", week)
	}
	if len(week.Users) != 1 || week.Users[0].ID != u.ID || week.Games[0].ID != "valorant" || week.Nodes[0].ID != "node-1" {
		t.Fatalf("rankings wrong: users %+v games %+v nodes %+v", week.Users, week.Games, week.Nodes)
	}

	var today struct {
		Series []trafficPoint `json:"series"`
	}
	w = request(s, token, "GET", "/api/admin/v1/traffic?days=1&tz=480", nil, "")
	json.Unmarshal(w.Body.Bytes(), &today)
	hour := time.Now().In(time.FixedZone("", 480*60)).Hour()
	if len(today.Series) != 24 || today.Series[hour].Bytes != 3500 {
		t.Fatalf("today's hourly view wrong at hour %d: %+v", hour, today.Series)
	}
}
