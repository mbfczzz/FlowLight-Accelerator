package control

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Cover pictures are uploaded by an operator, who is responsible for having
// the right to use them. Every upload is decoded and re-encoded here, so
// the clients only ever decode a JPEG this service produced, and stored
// under the SHA-256 of that JPEG, so a client can cache it forever and
// check what it downloaded.
const (
	coverUploadMax = 10 << 20
	coverMinWidth  = 640
	coverMinHeight = 360
	coverMaxSide   = 8000
	// Covers are kept at this width; smaller pictures keep their own.
	coverWidth = 1280
)

var coverName = regexp.MustCompile(`^[0-9a-f]{64}$`)

// coverGround fills transparent parts of a picture: the client's dark ground.
var coverGround = color.RGBA{0x13, 0x12, 0x17, 0xff}

// makeCover turns an uploaded picture into the stored JPEG: the largest
// 16:9 area around its centre, scaled to coverWidth at most.
func makeCover(raw []byte) ([]byte, error) {
	cfg, _, e := image.DecodeConfig(bytes.NewReader(raw))
	if e != nil {
		return nil, badRequest("无法识别这张图片，请上传 JPG、PNG 或 WebP 格式")
	}
	if cfg.Width > coverMaxSide || cfg.Height > coverMaxSide {
		return nil, badRequest("图片尺寸过大：宽和高都不能超过 " + strconv.Itoa(coverMaxSide) + " 像素")
	}
	if cfg.Width < coverMinWidth || cfg.Height < coverMinHeight {
		return nil, badRequest("图片太小：至少 " + strconv.Itoa(coverMinWidth) + "×" + strconv.Itoa(coverMinHeight) + "，建议 1920×1080 的横版图")
	}
	src, _, e := image.Decode(bytes.NewReader(raw))
	if e != nil {
		return nil, badRequest("图片已损坏或格式不完整，请换一张")
	}
	b := src.Bounds()
	cw, ch := b.Dx(), b.Dx()*9/16
	if ch > b.Dy() {
		cw, ch = b.Dy()*16/9, b.Dy()
	}
	crop := image.Rect(0, 0, cw, ch).Add(image.Pt(b.Min.X+(b.Dx()-cw)/2, b.Min.Y+(b.Dy()-ch)/2))
	w := min(cw, coverWidth)
	h := w * 9 / 16
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), image.NewUniform(coverGround), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(out, out.Bounds(), src, crop, draw.Over, nil)
	var buf bytes.Buffer
	if e = jpeg.Encode(&buf, out, &jpeg.Options{Quality: 86}); e != nil {
		return nil, e
	}
	return buf.Bytes(), nil
}

// setCover stores an uploaded picture as a game's cover.
func (s *Server) setCover(w http.ResponseWriter, r *http.Request) {
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, coverUploadMax))
	if e != nil {
		fail(w, 400, "图片超过 10 MB，请压缩后再上传")
		return
	}
	art, e := makeCover(raw)
	if e != nil {
		respond(w, nil, e)
		return
	}
	sum := sha256.Sum256(art)
	name := hex.EncodeToString(sum[:])
	s.changeCover(w, r, name, art)
}

// clearCover returns a game to its generated cover.
func (s *Server) clearCover(w http.ResponseWriter, r *http.Request) {
	s.changeCover(w, r, "", nil)
}

func (s *Server) changeCover(w http.ResponseWriter, r *http.Request, name string, art []byte) {
	id := r.PathValue("id")
	var out Game
	e := s.Store.Tx(func(tx *gorm.DB) error {
		g, e := load[Game](tx, id)
		if e != nil {
			return e
		}
		if art != nil {
			row := CoverArt{ID: name, Data: art, CreatedAt: time.Now().UTC()}
			if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; e != nil {
				return e
			}
		}
		old := g.CoverImage
		if e = tx.Model(&Game{}).Where("id = ?", id).UpdateColumn("cover_image", name).Error; e != nil {
			return e
		}
		if e = dropCoverIfUnused(tx, old); e != nil {
			return e
		}
		g.CoverImage = name
		out = g
		detail := "cleared"
		if name != "" {
			detail = name
		}
		return audit(tx, actor(r).ID, "game.cover", id, detail)
	})
	respond(w, out, e)
}

// dropCoverIfUnused deletes a stored picture no game shows any more.
func dropCoverIfUnused(tx *gorm.DB, name string) error {
	if name == "" {
		return nil
	}
	var n int64
	if e := tx.Model(&Game{}).Where("cover_image = ?", name).Count(&n).Error; e != nil || n > 0 {
		return e
	}
	return tx.Delete(&CoverArt{}, "id = ?", name).Error
}

// coverFile serves a stored cover. Covers are shown before sign-in and
// carry nothing private, so this needs no token; names are SHA-256 digests
// that only the game lists reveal.
func (s *Server) coverFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !coverName.MatchString(name) {
		fail(w, 404, "封面不存在")
		return
	}
	var art CoverArt
	if e := s.Store.DB.First(&art, "id = ?", name).Error; e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			fail(w, 404, "封面不存在")
			return
		}
		respond(w, nil, e)
		return
	}
	etag := `"` + name + `"`
	h := w.Header()
	h.Set("Content-Type", "image/jpeg")
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	h.Set("ETag", etag)
	h.Set("X-Content-Type-Options", "nosniff")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Length", strconv.Itoa(len(art.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(art.Data)
}
