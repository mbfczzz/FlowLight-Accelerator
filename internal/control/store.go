package control

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/glebarez/sqlite"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Store struct {
	DB  *gorm.DB
	key []byte
	mu  sync.Mutex
}

func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func secret() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func Open(driver, dsn, key string) (*Store, error) {
	k, e := base64.StdEncoding.DecodeString(key)
	if e != nil || len(k) != 32 {
		return nil, errors.New("encryption key must be a base64-encoded 32-byte key")
	}
	var dial gorm.Dialector
	if driver == "mysql" {
		dial = mysql.Open(dsn)
	} else if driver == "sqlite" {
		dial = sqlite.Open(dsn)
	} else {
		return nil, errors.New("DB_DRIVER must be mysql or sqlite")
	}
	db, e := gorm.Open(dial, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		return nil, e
	}
	sqlDB, e := db.DB()
	if e != nil {
		return nil, e
	}
	if driver == "sqlite" {
		sqlDB.SetMaxOpenConns(1)
		db.Exec("PRAGMA foreign_keys=ON")
		db.Exec("PRAGMA busy_timeout=5000")
	} else {
		sqlDB.SetMaxOpenConns(20)
	}
	e = db.AutoMigrate(&Gate{}, &User{}, &Token{}, &Device{}, &Game{}, &GameVersion{}, &CoverArt{}, &Node{}, &Endpoint{}, &Credential{}, &Line{}, &LineVersion{}, &Binding{}, &Session{}, &Reservation{}, &Idempotency{}, &Job{}, &Audit{}, &Release{}, &TrafficHour{}, &LineCheck{})
	if e != nil {
		return nil, e
	}
	db.Clauses(clause.OnConflict{DoNothing: true}).Create(&Gate{ID: 1})
	if e = assignCoverSlots(db); e != nil {
		return nil, e
	}
	return &Store{DB: db, key: k}, nil
}

// assignCoverSlots gives games without a cover slot the next free ones in
// id order: games stored before slots existed, and the built-in drafts.
func assignCoverSlots(tx *gorm.DB) error {
	var ids []string
	if e := tx.Model(&Game{}).Where("cover_slot = 0").Order("id").Pluck("id", &ids).Error; e != nil {
		return e
	}
	for _, id := range ids {
		next, e := nextCoverSlot(tx)
		if e != nil {
			return e
		}
		if e = tx.Model(&Game{}).Where("id = ?", id).UpdateColumn("cover_slot", next).Error; e != nil {
			return e
		}
	}
	return nil
}

func nextCoverSlot(tx *gorm.DB) (int, error) {
	var top int
	e := tx.Model(&Game{}).Select("COALESCE(MAX(cover_slot), 0)").Scan(&top).Error
	return top + 1, e
}
func (s *Store) Tx(fn func(*gorm.DB) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var g Gate
		q := tx
		if tx.Dialector.Name() == "mysql" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if e := q.First(&g, 1).Error; e != nil {
			return e
		}
		return fn(tx)
	})
}
func (s *Store) Seal(v any) (string, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return "", e
	}
	block, e := aes.NewCipher(s.key)
	if e != nil {
		return "", e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return "", e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return "", e
	}
	return base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, b, []byte("flowlight:v1"))), nil
}
func (s *Store) Unseal(v string, out any) error {
	b, e := base64.StdEncoding.DecodeString(v)
	if e != nil {
		return e
	}
	block, _ := aes.NewCipher(s.key)
	g, _ := cipher.NewGCM(block)
	if len(b) < g.NonceSize() {
		return errors.New("invalid ciphertext")
	}
	plain, e := g.Open(nil, b[:g.NonceSize()], b[g.NonceSize():], []byte("flowlight:v1"))
	if e != nil {
		return e
	}
	return json.Unmarshal(plain, out)
}
func LocalKey(path string) (string, error) {
	b, e := os.ReadFile(path)
	if e == nil {
		return strings.TrimSpace(string(b)), nil
	}
	if !os.IsNotExist(e) {
		return "", e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return "", e
	}
	b = make([]byte, 32)
	if _, e = rand.Read(b); e != nil {
		return "", e
	}
	k := base64.StdEncoding.EncodeToString(b)
	e = os.WriteFile(path, []byte(k), 0600)
	return k, e
}
func (s *Store) Seed(username, password string) error {
	return s.Tx(func(tx *gorm.DB) error {
		var count int64
		tx.Model(&User{}).Count(&count)
		if count == 0 {
			if len(password) < 14 {
				return errors.New("初始管理员密码 ADMIN_PASSWORD 至少需要 14 个字符（initial ADMIN_PASSWORD must contain at least 14 characters）")
			}
			h, e := bcrypt.GenerateFromPassword([]byte(password), 12)
			if e != nil {
				return e
			}
			if e = tx.Create(&User{ID: ID(), Username: username, PasswordHash: string(h), Role: "admin", Enabled: true, MaxSessions: 2, CreatedAt: time.Now().UTC()}).Error; e != nil {
				return e
			}
		}
		for _, g := range []Game{{ID: "valorant", Name: "VALORANT", Regions: []string{}, Draft: c.Profile{Processes: []string{"VALORANT-Win64-Shipping.exe"}, UDP: true}, Enabled: true}, {ID: "lol", Name: "英雄联盟", Regions: []string{}, Draft: c.Profile{Processes: []string{"League of Legends.exe"}, UDP: true}, Enabled: true}} {
			if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&g).Error; e != nil {
				return e
			}
		}
		return assignCoverSlots(tx)
	})
}
func audit(tx *gorm.DB, actor, action, id, detail string) error {
	return tx.Create(&Audit{ID: ID(), Actor: actor, Action: action, Resource: id, Detail: detail, CreatedAt: time.Now().UTC()}).Error
}
func required(ok bool, msg string) error {
	if !ok {
		return badRequest(msg)
	}
	return nil
}
