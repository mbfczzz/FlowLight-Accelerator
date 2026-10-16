package control

import (
	"slices"
	"strings"
	"testing"
	"time"

	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"gorm.io/gorm"
)

func TestDeleteRefusesWhatIsStillInUse(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	now := time.Now().UTC()
	create := func(rows ...any) {
		t.Helper()
		for _, row := range rows {
			if e := s.Store.DB.Create(row).Error; e != nil {
				t.Fatal(e)
			}
		}
	}
	remove := func(path string, code int, says string) {
		t.Helper()
		w := request(s, token, "DELETE", "/api/admin/v1/"+path, nil, "")
		if w.Code != code || !strings.Contains(w.Body.String(), says) {
			t.Fatalf("DELETE %s: %d %s, want %d containing %q", path, w.Code, w.Body.String(), code, says)
		}
	}
	exists := func(row any, id string) bool {
		var n int64
		s.Store.DB.Model(row).Where("id = ?", id).Count(&n)
		return n > 0
	}
	setState := func(id, state string) {
		t.Helper()
		if e := s.Store.DB.Model(&Session{}).Where("id = ?", id).Update("state", state).Error; e != nil {
			t.Fatal(e)
		}
	}

	// Lines and games are what players see: an operator cannot remove them.
	operator := User{ID: ID(), Username: "ops", Enabled: true, Role: "operator", MaxSessions: 1}
	create(&operator)
	var auth map[string]any
	s.Store.Tx(func(tx *gorm.DB) error { var e error; auth, e = s.issue(tx, operator, ""); return e })
	if w := request(s, auth["access_token"].(string), "DELETE", "/api/admin/v1/lines/line-1", nil, ""); w.Code != 403 {
		t.Fatalf("operator deleted a line: %d", w.Code)
	}

	// The fixture's line reaches ep-1 only through its published version.
	remove("nodes/node-1", 400, "「exit」")
	remove("endpoints/ep-1", 400, "「test line」")

	create(&Session{ID: "live", UserID: u.ID, GameID: "valorant", LineID: "line-1", LineVersion: "version-1", Purpose: "game", State: "ACTIVE", CreatedAt: now, PrepareUntil: now.Add(time.Minute), LeaseUntil: now.Add(time.Minute)})
	remove("lines/line-1", 400, "终止线路")
	s.Store.DB.Model(&Line{}).Where("id = ?", "line-1").Update("game_ids", `["valorant"]`)
	remove("games/valorant", 400, "「test line」")
	s.Store.DB.Model(&Line{}).Where("id = ?", "line-1").Update("game_ids", `[]`)
	remove("games/valorant", 400, "正在加速")

	setState("live", "CLOSED")
	remove("lines/line-1", 200, "")
	if exists(&Line{}, "line-1") || !exists(&LineVersion{}, "version-1") {
		t.Fatal("a deleted line keeps its versions, and only them")
	}

	// A version still in use, though no line points to it any more.
	create(&Session{ID: "verifying", UserID: u.ID, LineID: "line-1", LineVersion: "version-1", Purpose: "validation", State: "PREPARING", CreatedAt: now, PrepareUntil: now.Add(time.Minute), LeaseUntil: now.Add(time.Minute)})
	remove("endpoints/ep-1", 400, "进行中的会话")
	setState("verifying", "CLOSED")

	create(&Binding{ID: "binding-1", UserID: u.ID, EndpointID: "ep-1", Kind: "ss2022", Enabled: true})
	remove("endpoints/ep-1", 400, "落地绑定")
	create(&Session{ID: "bound", UserID: u.ID, BindingID: "binding-1", Purpose: "game", State: "ACTIVE", CreatedAt: now, PrepareUntil: now.Add(time.Minute), LeaseUntil: now.Add(time.Minute)})
	remove("bindings/binding-1", 400, "进行中的会话")
	setState("bound", "CLOSED")
	remove("bindings/binding-1", 200, "")

	create(&Credential{ID: "credential-1", Ciphertext: "x", CreatedAt: now})
	s.Store.DB.Model(&Endpoint{}).Where("id = ?", "ep-1").Update("credential_id", "credential-1")
	remove("endpoints/ep-1", 200, "")
	if exists(&Endpoint{}, "ep-1") || exists(&Credential{}, "credential-1") {
		t.Fatal("the endpoint's credential outlived it")
	}

	create(&Job{ID: "job-1", NodeID: "node-1", Kind: "start", Status: "pending", CreatedAt: now})
	remove("nodes/node-1", 200, "")
	var job Job
	s.Store.DB.First(&job, "id = ?", "job-1")
	if exists(&Node{}, "node-1") || job.Status != "cancelled" {
		t.Fatalf("node left behind: job %s", job.Status)
	}
	if w := request(s, "node-test-token", "POST", "/api/node/v1/heartbeat", map[string]any{}, ""); w.Code != 401 {
		t.Fatalf("a deleted node's agent still signs in: %d", w.Code)
	}

	create(&CoverArt{ID: "cover-1", Data: []byte{1}, CreatedAt: now})
	s.Store.DB.Model(&Game{}).Where("id = ?", "valorant").Update("cover_image", "cover-1")
	remove("games/valorant", 200, "")
	if exists(&Game{}, "valorant") || exists(&GameVersion{}, "profile-1") || exists(&CoverArt{}, "cover-1") {
		t.Fatal("the game's versions or cover outlived it")
	}

	create(&Release{ID: "release-1", Version: "0.1.6", Channel: "stable", CreatedAt: now})
	remove("releases/release-1", 200, "")
	remove("releases/release-1", 404, "")
	remove("users/"+u.ID, 405, "")

	// History keeps the name the row had.
	var got []string
	s.Store.DB.Model(&Audit{}).Where("action = ?", "resource.delete").Pluck("detail", &got)
	want := []string{"线路「test line」", "落地绑定", "服务端点「exit」", "节点服务器「test」", "游戏「VALORANT」", "客户端版本 0.1.6"}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("audit: %q", got)
	}
}

// The draft path alone is enough to keep an endpoint.
func TestDeleteEndpointRefusedByDraftPath(t *testing.T) {
	s, token, u := setup(t)
	fixture(t, s, u)
	s.Store.DB.Create(&Line{ID: "line-2", Name: "draft line", EndpointIDs: []string{"ep-1"}, Mode: "managed"})
	s.Store.DB.Create(&LineVersion{ID: "version-2", LineID: "line-2", Number: 1, Status: "failed", Hops: []c.Hop{{EndpointID: "ep-1", NodeID: "node-1"}}, CreatedAt: time.Now().UTC()})
	s.Store.DB.Delete(&Line{}, "id = ?", "line-1")
	w := request(s, token, "DELETE", "/api/admin/v1/endpoints/ep-1", nil, "")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "「draft line」") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
