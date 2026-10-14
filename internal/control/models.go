package control

import (
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"time"
)

type User struct {
	ID           string    `json:"id" gorm:"primaryKey;size:64"`
	Username     string    `json:"username" gorm:"uniqueIndex;size:100"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Enabled      bool      `json:"enabled"`
	MaxSessions  int       `json:"max_sessions"`
	CreatedAt    time.Time `json:"created_at"`
}
type Token struct {
	Hash      string `gorm:"primaryKey;size:64"`
	UserID    string `gorm:"index;size:64"`
	Kind      string
	Family    string `gorm:"index;size:64"`
	ExpiresAt time.Time
	Used      bool
}
type Device struct {
	ID           string         `json:"id" gorm:"primaryKey;size:64"`
	UserID       string         `json:"user_id" gorm:"index;size:64"`
	Name         string         `json:"name"`
	Capabilities c.Capabilities `json:"capabilities" gorm:"serializer:json;type:text"`
	LastSeen     time.Time      `json:"last_seen" gorm:"default:null"`
}
type Game struct {
	ID               string    `json:"id" gorm:"primaryKey;size:64"`
	Name             string    `json:"name"`
	Regions          []string  `json:"regions" gorm:"serializer:json;type:text"`
	Draft            c.Profile `json:"draft" gorm:"serializer:json;type:text"`
	PublishedVersion int       `json:"published_version"`
	Enabled          bool      `json:"enabled"`
	UpdatedAt        time.Time `json:"updated_at"`
}
type GameVersion struct {
	ID        string    `json:"id" gorm:"primaryKey;size:64"`
	GameID    string    `json:"game_id" gorm:"uniqueIndex:game_version;size:64"`
	Version   int       `json:"version" gorm:"uniqueIndex:game_version"`
	Profile   c.Profile `json:"profile" gorm:"serializer:json;type:text"`
	CreatedAt time.Time `json:"created_at"`
}
type Node struct {
	BootstrapUntil  time.Time      `json:"-" gorm:"default:null"`
	ID              string         `json:"id" gorm:"primaryKey;size:64"`
	Name            string         `json:"name"`
	Region          string         `json:"region"`
	Address         string         `json:"address"`
	Enabled         bool           `json:"enabled"`
	Capabilities    c.Capabilities `json:"capabilities" gorm:"serializer:json;type:text"`
	BootstrapHash   string         `json:"-"`
	TokenHash       string         `json:"-"`
	CertificateHash string         `json:"-"`
	LastSeen        time.Time      `json:"last_seen" gorm:"default:null"`
	AgentVersion    string         `json:"agent_version"`
	ActualVersions  []string       `json:"actual_versions" gorm:"serializer:json;type:text"`
}
type Endpoint struct {
	OwnerID      string         `json:"owner_id,omitempty"`
	ID           string         `json:"id" gorm:"primaryKey;size:64"`
	NodeID       string         `json:"node_id" gorm:"index;size:64"`
	Name         string         `json:"name"`
	Kind         string         `json:"kind"`
	Host         string         `json:"host"`
	Port         int            `json:"port"`
	PortEnd      int            `json:"port_end"`
	Cipher       string         `json:"cipher"`
	CredentialID string         `json:"credential_id,omitempty"`
	Capabilities c.Capabilities `json:"capabilities" gorm:"serializer:json;type:text"`
	Enabled      bool           `json:"enabled"`
}
type Credential struct {
	ID         string `gorm:"primaryKey;size:64"`
	Ciphertext string `json:"-"`
	CreatedAt  time.Time
}
type Line struct {
	ID               string    `json:"id" gorm:"primaryKey;size:64"`
	Name             string    `json:"name"`
	Regions          []string  `json:"regions" gorm:"serializer:json;type:text"`
	GameIDs          []string  `json:"game_ids" gorm:"serializer:json;type:text"`
	UserIDs          []string  `json:"user_ids" gorm:"serializer:json;type:text"`
	EndpointIDs      []string  `json:"endpoint_ids" gorm:"serializer:json;type:text"`
	Capacity         int       `json:"capacity"`
	Priority         int       `json:"priority"`
	Mode             string    `json:"mode"`
	PublishedVersion string    `json:"published_version"`
	Draining         bool      `json:"draining"`
	ExpectedExit     string    `json:"expected_exit"`
	UpdatedAt        time.Time `json:"updated_at"`
}
type LineVersion struct {
	ID           string        `json:"id" gorm:"primaryKey;size:64"`
	LineID       string        `json:"line_id" gorm:"index;size:64"`
	Number       int           `json:"number"`
	Status       string        `json:"status"`
	Hops         []c.Hop       `json:"hops" gorm:"serializer:json;type:text"`
	Regions      []string      `json:"regions" gorm:"serializer:json;type:text"`
	GameIDs      []string      `json:"game_ids" gorm:"serializer:json;type:text"`
	Mode         string        `json:"mode"`
	ExpectedExit string        `json:"expected_exit"`
	Probe        c.ProbeResult `json:"probe" gorm:"serializer:json;type:text"`
	Error        string        `json:"error,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
}
type Binding struct {
	ID           string     `json:"id" gorm:"primaryKey;size:64"`
	UserID       string     `json:"user_id" gorm:"index;size:64"`
	EndpointID   string     `json:"endpoint_id"`
	Kind         string     `json:"kind"`
	VerifiedAt   *time.Time `json:"verified_at"`
	ExpectedExit string     `json:"expected_exit"`
	FixedIP      bool       `json:"fixed_ip"`
	Enabled      bool       `json:"enabled"`
}
type Session struct {
	ID               string        `json:"id" gorm:"primaryKey;size:64"`
	UserID           string        `json:"user_id" gorm:"index;size:64"`
	DeviceID         string        `json:"device_id" gorm:"index;size:64"`
	GameID           string        `json:"game_id"`
	RegionID         string        `json:"region_id"`
	LineID           string        `json:"line_id" gorm:"index;size:64"`
	LineVersion      string        `json:"line_version"`
	ProfileVersion   int           `json:"profile_version"`
	BindingID        string        `json:"egress_binding_id,omitempty"`
	Purpose          string        `json:"purpose"`
	State            string        `json:"state" gorm:"index;size:32"`
	ConnectionCipher string        `json:"-" gorm:"type:text"`
	LeaseUntil       time.Time     `json:"lease_until"`
	PrepareUntil     time.Time     `json:"prepare_until"`
	StopUntil        time.Time     `json:"stop_until" gorm:"default:null"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	EndReason        string        `json:"end_reason,omitempty"`
	Probe            c.ProbeResult `json:"probe" gorm:"serializer:json;type:text"`
	Enforcement      string        `json:"enforcement"`
}
type Reservation struct {
	ID        string `gorm:"primaryKey;size:64"`
	SessionID string `gorm:"index;size:64"`
	NodeID    string `gorm:"uniqueIndex:node_port;size:64"`
	Port      int    `gorm:"uniqueIndex:node_port"`
}
type Idempotency struct {
	ID          string `gorm:"primaryKey;size:64"`
	SessionID   string `gorm:"size:64"`
	RequestHash string
}
type Job struct {
	ID            string    `json:"id" gorm:"primaryKey;size:64"`
	NodeID        string    `json:"node_id" gorm:"index;size:64"`
	SessionID     string    `json:"session_id" gorm:"index;size:64"`
	VersionID     string    `json:"version_id"`
	Kind          string    `json:"kind"`
	PayloadCipher string    `json:"-" gorm:"type:text"`
	Status        string    `json:"status" gorm:"index;size:24"`
	ClaimHash     string    `json:"-"`
	ClaimUntil    time.Time `json:"claim_until" gorm:"default:null"`
	Attempts      int       `json:"attempts"`
	Result        string    `json:"result,omitempty" gorm:"type:text"`
	CreatedAt     time.Time `json:"created_at"`
}
type Audit struct {
	ID        string    `json:"id" gorm:"primaryKey;size:64"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Detail    string    `json:"detail" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
}
type Release struct {
	ID        string    `json:"id" gorm:"primaryKey;size:64"`
	Version   string    `json:"version"`
	Channel   string    `json:"channel"`
	URL       string    `json:"url"`
	SHA256    string    `json:"sha256"`
	Signature string    `json:"signature"`
	Required  bool      `json:"required"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
}
type Gate struct {
	ID       int `gorm:"primaryKey"`
	Revision int64
}
