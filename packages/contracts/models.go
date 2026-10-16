package contracts

import "time"

type Capabilities struct {
	TCP     bool `json:"tcp"`
	UDP     bool `json:"udp"`
	IPv4    bool `json:"ipv4"`
	IPv6    bool `json:"ipv6"`
	Managed bool `json:"managed"`
}
type Profile struct {
	Processes      []string `json:"processes"`
	Paths          []string `json:"paths"`
	Domains        []string `json:"domains"`
	CIDRs          []string `json:"cidrs"`
	ExcludeDomains []string `json:"exclude_domains"`
	UDP            bool     `json:"udp"`
}
type Hop struct {
	EndpointID   string       `json:"endpoint_id"`
	NodeID       string       `json:"node_id"`
	Kind         string       `json:"kind"`
	Host         string       `json:"host"`
	Port         int          `json:"port"`
	PortEnd      int          `json:"port_end"`
	Cipher       string       `json:"cipher"`
	CredentialID string       `json:"credential_id,omitempty"`
	Capabilities Capabilities `json:"capabilities"`
}
type Connection struct {
	Host         string  `json:"host"`
	Port         int     `json:"port"`
	Cipher       string  `json:"cipher"`
	Password     string  `json:"password"`
	UDP          bool    `json:"udp"`
	Profile      Profile `json:"profile"`
	ProbeURL     string  `json:"probe_url"`
	ProbeUDP     string  `json:"probe_udp"`
	ExpectedExit string  `json:"expected_exit,omitempty"`
}
type Runtime struct {
	VersionID  string    `json:"version_id,omitempty"`
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	Kind       string    `json:"kind"`
	ListenPort int       `json:"listen_port"`
	RemoteHost string    `json:"remote_host,omitempty"`
	RemotePort int       `json:"remote_port,omitempty"`
	Cipher     string    `json:"cipher,omitempty"`
	Password   string    `json:"password,omitempty"`
	UDP        bool      `json:"udp"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// CheckTask is one node's part in a line connectivity check: open a test
// listener on the first free port of Ports, or measure Targets.
type CheckTask struct {
	Hop     int           `json:"hop"`
	Ports   []int         `json:"ports,omitempty"`
	UDP     bool          `json:"udp,omitempty"`
	Seconds int           `json:"seconds,omitempty"`
	Token   string        `json:"token,omitempty"`
	Targets []CheckTarget `json:"targets,omitempty"`
}

// CheckTarget is one link to measure. With URL, it is the node's own
// internet access through the probe service; otherwise Host:Port, which
// with Token is a check listener that must answer, and without it a
// service that only has to accept.
type CheckTarget struct {
	ID        string `json:"id"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	Token     string `json:"token,omitempty"`
	UDP       bool   `json:"udp,omitempty"`
	URL       string `json:"url,omitempty"`
	UDPTarget string `json:"udp_target,omitempty"`
}

// CheckMeasure is one protocol over one link: OK, the median round trip
// and its spread, and the share of samples lost.
type CheckMeasure struct {
	OK      bool    `json:"ok"`
	RTTMS   float64 `json:"rtt_ms"`
	MinMS   float64 `json:"min_ms"`
	MaxMS   float64 `json:"max_ms"`
	Loss    float64 `json:"loss"`
	Samples int     `json:"samples"`
	Error   string  `json:"error,omitempty"`
}
type CheckLinkResult struct {
	ID     string        `json:"id"`
	TCP    *CheckMeasure `json:"tcp,omitempty"`
	UDP    *CheckMeasure `json:"udp,omitempty"`
	ExitIP string        `json:"exit_ip,omitempty"`
}

// CheckReport answers a CheckTask: the port listened on, or the links.
type CheckReport struct {
	Port  int               `json:"port,omitempty"`
	Links []CheckLinkResult `json:"links,omitempty"`
}
type JobPayload struct {
	Runtime       *Runtime    `json:"runtime,omitempty"`
	Connection    *Connection `json:"connection,omitempty"`
	Check         *CheckTask  `json:"check,omitempty"`
	EndpointKinds []string    `json:"endpoint_kinds,omitempty"`
}
type ProbeResult struct {
	TCP        bool      `json:"tcp"`
	UDP        bool      `json:"udp"`
	RTTMS      float64   `json:"rtt_ms"`
	JitterMS   float64   `json:"jitter_ms"`
	Loss       float64   `json:"loss"`
	Samples    int       `json:"samples"`
	ExitIP     string    `json:"exit_ip"`
	ObservedAt time.Time `json:"observed_at"`
}
type Job struct {
	ID        string     `json:"id"`
	NodeID    string     `json:"node_id"`
	SessionID string     `json:"session_id"`
	VersionID string     `json:"version_id"`
	Kind      string     `json:"kind"`
	Payload   JobPayload `json:"payload"`
	Claim     string     `json:"claim"`
	Attempt   int        `json:"attempt"`
}
