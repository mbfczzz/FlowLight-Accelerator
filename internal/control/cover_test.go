package control

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http/httptest"
	"testing"
)

func picture(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	var buf bytes.Buffer
	if e := png.Encode(&buf, img); e != nil {
		t.Fatal(e)
	}
	return buf.Bytes()
}

func upload(s *Server, token, game string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest("PUT", "/api/admin/v1/games/"+game+"/cover", bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "image/png")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestCoverUploadIsReencodedServedAndKeptOnSave(t *testing.T) {
	s, token, _ := setup(t)
	if w := upload(s, token, "valorant", []byte("not a picture")); w.Code != 400 {
		t.Fatalf("a non-image must be refused: %d %s", w.Code, w.Body.String())
	}
	if w := upload(s, token, "valorant", picture(t, 320, 180)); w.Code != 400 {
		t.Fatalf("a picture below 640x360 must be refused: %d %s", w.Code, w.Body.String())
	}

	w := upload(s, token, "valorant", picture(t, 2000, 1000))
	var g Game
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &g) != nil || !coverName.MatchString(g.CoverImage) {
		t.Fatalf("upload failed: %d %s", w.Code, w.Body.String())
	}

	// Served without a token, as a 16:9 JPEG at most 1280 wide, named by
	// the digest of what is served.
	r := httptest.NewRequest("GET", "/api/client/v1/covers/"+g.CoverImage, nil)
	res := httptest.NewRecorder()
	s.Handler().ServeHTTP(res, r)
	if res.Code != 200 || res.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("cover not served: %d %v", res.Code, res.Header())
	}
	sum := sha256.Sum256(res.Body.Bytes())
	if hex.EncodeToString(sum[:]) != g.CoverImage {
		t.Fatal("served bytes do not match the cover name")
	}
	cfg, e := jpeg.DecodeConfig(bytes.NewReader(res.Body.Bytes()))
	if e != nil || cfg.Width != 1280 || cfg.Height != 720 {
		t.Fatalf("cover should be 1280x720 JPEG: %v %+v", e, cfg)
	}
	r = httptest.NewRequest("GET", "/api/client/v1/covers/"+g.CoverImage, nil)
	r.Header.Set("If-None-Match", `"`+g.CoverImage+`"`)
	res = httptest.NewRecorder()
	s.Handler().ServeHTTP(res, r)
	if res.Code != 304 {
		t.Fatalf("a cached cover should be answered 304: %d", res.Code)
	}

	// Saving the game, even with another picture name in the body, keeps it.
	if w := request(s, token, "PATCH", "/api/admin/v1/games/valorant", map[string]any{"name": "VALORANT", "cover_image": "0000"}, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var stored Game
	s.Store.DB.First(&stored, "id = ?", "valorant")
	if stored.CoverImage != g.CoverImage {
		t.Fatalf("saving the game changed its cover: %q", stored.CoverImage)
	}

	// Clearing drops the picture once no game shows it.
	if w := request(s, token, "DELETE", "/api/admin/v1/games/valorant/cover", nil, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var left int64
	s.Store.DB.Model(&CoverArt{}).Count(&left)
	if left != 0 {
		t.Fatalf("an unused cover was kept: %d", left)
	}
	r = httptest.NewRequest("GET", "/api/client/v1/covers/"+g.CoverImage, nil)
	res = httptest.NewRecorder()
	s.Handler().ServeHTTP(res, r)
	if res.Code != 404 {
		t.Fatalf("a dropped cover is still served: %d", res.Code)
	}
}
