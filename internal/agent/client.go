package agent

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	contract "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

type Config struct {
	ControlURL string   `json:"control_url"`
	NodeID     string   `json:"node_id"`
	Bootstrap  string   `json:"bootstrap_secret,omitempty"`
	Token      string   `json:"token,omitempty"`
	DataDir    string   `json:"data_dir"`
	AllowHTTP  bool     `json:"allow_http_local_dev"`
	ControlCA  string   `json:"control_ca,omitempty"`
	Binaries   Binaries `json:"binaries"`
}
type identity struct {
	Token       string `json:"token"`
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
}

// Renew the node certificate once less than this much validity remains.
const renewBefore = 30 * 24 * time.Hour

type Client struct {
	clockOffset atomic.Int64
	Config      Config
	Token       string
	Manager     *Manager
	roots       *x509.CertPool
	http        atomic.Pointer[http.Client]
	notAfter    atomic.Int64
}

func Open(cfg Config) (*Client, error) {
	u, e := url.Parse(cfg.ControlURL)
	if e != nil || u.Host == "" {
		return nil, errors.New("control URL required")
	}
	if u.Scheme != "https" && !(cfg.AllowHTTP && u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1")) {
		return nil, errors.New("HTTPS control URL required; HTTP is limited to explicit loopback development")
	}
	if cfg.DataDir == "" {
		return nil, errors.New("data directory required")
	}
	if e = os.MkdirAll(cfg.DataDir, 0700); e != nil {
		return nil, e
	}
	client := &Client{Config: cfg}
	if cfg.ControlCA != "" {
		b, e := os.ReadFile(cfg.ControlCA)
		if e != nil {
			return nil, e
		}
		// control_ca pins the control service to this CA instead of adding
		// it to the system roots, so no public CA can impersonate control.
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(b) {
			return nil, errors.New("invalid control CA")
		}
		client.roots = pool
	}
	client.http.Store(client.transport(nil))
	var id identity
	path := filepath.Join(cfg.DataDir, "identity.json")
	b, e := os.ReadFile(path)
	if e == nil {
		if e = json.Unmarshal(b, &id); e != nil {
			return nil, e
		}
	} else if os.IsNotExist(e) {
		if cfg.Bootstrap == "" {
			return nil, errors.New("node enrollment required")
		}
		key, csr, e := newCSR(cfg.NodeID)
		if e != nil {
			return nil, e
		}
		var result struct {
			Token       string `json:"token"`
			Certificate string `json:"certificate"`
		}
		if e = client.call(context.Background(), "POST", "/api/node/v1/register", map[string]any{"node_id": cfg.NodeID, "bootstrap_secret": cfg.Bootstrap, "csr": csr}, &result); e != nil {
			return nil, e
		}
		id = identity{Token: result.Token, Certificate: result.Certificate, Key: key}
		b, _ = json.Marshal(id)
		if e = atomicWrite(path, b); e != nil {
			return nil, e
		}
	} else {
		return nil, e
	}
	client.Token = id.Token
	if e = client.use(id); e != nil {
		return nil, e
	}
	client.Manager, e = NewManager(filepath.Join(cfg.DataDir, "runtimes"), cfg.Binaries)
	return client, e
}

// transport builds a fresh client for a certificate. Reusing the transport
// from before enrollment would keep pooled connections that carry no
// certificate, so every identity change gets its own connection pool.
func (c *Client) transport(certificate *tls.Certificate) *http.Client {
	config := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: c.roots}
	if certificate != nil {
		config.Certificates = []tls.Certificate{*certificate}
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: config}, Timeout: 15 * time.Second}
}
func (c *Client) use(id identity) error {
	certificate, e := tls.X509KeyPair([]byte(id.Certificate), []byte(id.Key))
	if e != nil {
		return e
	}
	leaf, e := x509.ParseCertificate(certificate.Certificate[0])
	if e != nil {
		return e
	}
	c.notAfter.Store(leaf.NotAfter.UnixNano())
	previous := c.http.Swap(c.transport(&certificate))
	if previous != nil {
		previous.CloseIdleConnections()
	}
	return nil
}
func newCSR(nodeID string) (string, string, error) {
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return "", "", e
	}
	csr, e := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: nodeID}}, key)
	if e != nil {
		return "", "", e
	}
	der, e := x509.MarshalECPrivateKey(key)
	if e != nil {
		return "", "", e
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})), string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr})), nil
}

// renew rotates the node key and certificate ahead of expiry. The new
// identity is saved before it is used; control keeps accepting the old
// certificate until the new one is presented.
func (c *Client) renew(ctx context.Context) error {
	if time.Until(time.Unix(0, c.notAfter.Load())) > renewBefore {
		return nil
	}
	key, csr, e := newCSR(c.Config.NodeID)
	if e != nil {
		return e
	}
	var result struct {
		Certificate string `json:"certificate"`
	}
	if e = c.call(ctx, "POST", "/api/node/v1/certificate", map[string]any{"csr": csr}, &result); e != nil {
		return e
	}
	id := identity{Token: c.Token, Certificate: result.Certificate, Key: key}
	b, _ := json.Marshal(id)
	if e = atomicWrite(filepath.Join(c.Config.DataDir, "identity.json"), b); e != nil {
		return e
	}
	if e = c.use(id); e != nil {
		return e
	}
	log.Printf("node certificate renewed; valid until %s", time.Unix(0, c.notAfter.Load()).UTC().Format(time.RFC3339))
	return nil
}
func (c *Client) call(ctx context.Context, method, path string, body, out any) error {
	var data []byte
	if body != nil {
		var e error
		data, e = json.Marshal(body)
		if e != nil {
			return e
		}
	}
	req, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.Config.ControlURL, "/")+path, bytes.NewReader(data))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, e := c.http.Load().Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("control API rejected %s: HTTP %d", path, res.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(out)
	}
	return nil
}
func (c *Client) heartbeat(ctx context.Context) error {
	var out struct {
		Leases     map[string]time.Time `json:"leases"`
		ServerTime time.Time            `json:"server_time"`
	}
	sentAt := time.Now()
	// The managed listeners bind IPv4 only, so IPv6 is never reported.
	e := c.call(ctx, "POST", "/api/node/v1/heartbeat", map[string]any{"version": "0.1.0", "capabilities": map[string]bool{"tcp": true, "udp": true, "ipv4": true, "managed": true}, "actual_versions": c.Manager.ActualVersions()}, &out)
	if e != nil {
		return e
	}
	offset := out.ServerTime.Sub(time.Now().UTC())
	c.clockOffset.Store(int64(offset))
	for id, expiry := range out.Leases {
		out.Leases[id] = expiry.Add(-offset)
	}
	if e = c.Manager.Renew(out.Leases, sentAt); e != nil {
		return e
	}
	return c.reportFailures(ctx)
}

// reportFailures tells control about runtimes that died and could not be
// restarted, so their sessions stop instead of renewing a dead tunnel.
func (c *Client) reportFailures(ctx context.Context) error {
	failed := c.Manager.TakeFailures()
	if len(failed) == 0 {
		return nil
	}
	seen := map[string]bool{}
	events := []map[string]string{}
	for _, rt := range failed {
		if !seen[rt.SessionID] && len(events) < 100 {
			seen[rt.SessionID] = true
			events = append(events, map[string]string{"session_id": rt.SessionID, "kind": "runtime_failed"})
		}
	}
	if e := c.call(ctx, "POST", "/api/node/v1/session-events", map[string]any{"events": events}, &map[string]any{}); e != nil {
		c.Manager.ReturnFailures(failed)
		return e
	}
	return nil
}
func (c *Client) execute(ctx context.Context, j contract.Job) (bool, string, contract.ProbeResult) {
	var e error
	var probe contract.ProbeResult
	switch j.Kind {
	case "prepare":
		if j.Payload.Runtime == nil {
			return false, "runtime missing", probe
		}
		rt := *j.Payload.Runtime
		rt.ExpiresAt = rt.ExpiresAt.Add(-time.Duration(c.clockOffset.Load()))
		e = c.Manager.Prepare(rt)
	case "revoke":
		if j.Payload.Runtime == nil {
			return false, "runtime missing", probe
		}
		e = c.Manager.Stop(j.Payload.Runtime.ID)
	case "probe":
		if j.Payload.Connection == nil {
			return false, "connection missing", probe
		}
		probe, e = c.Manager.Probe(ctx, *j.Payload.Connection)
	default:
		e = errors.New("unsupported structured task")
	}
	if e != nil {
		log.Printf("%s task %s failed: %v", j.Kind, j.ID, e)
		return false, e.Error(), probe
	}
	return true, "", probe
}

const maxConcurrentProbes = 4

func (c *Client) runJob(ctx context.Context, j contract.Job) {
	taskCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	ok, message, probe := c.execute(taskCtx, j)
	cancel()
	if e := c.call(ctx, "POST", "/api/node/v1/jobs/"+j.ID+"/result", map[string]any{"claim": j.Claim, "success": ok, "error": message, "probe": probe}, &map[string]any{}); e != nil && ctx.Err() == nil {
		log.Printf("reporting %s task %s failed: %v", j.Kind, j.ID, e)
	}
}
func (c *Client) Run(ctx context.Context) error {
	for {
		e := c.heartbeat(ctx)
		if e == nil {
			break
		}
		log.Printf("heartbeat failed: %v", e)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.Manager.Expire()
			}
		}
	}()
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		healthy := true
		for {
			if e := c.heartbeat(ctx); e != nil && ctx.Err() == nil {
				if healthy {
					log.Printf("heartbeat failed: %v", e)
				}
				healthy = false
			} else {
				healthy = true
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			if e := c.renew(ctx); e != nil && ctx.Err() == nil {
				log.Printf("certificate renewal failed: %v", e)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
	// Probes can take up to 45 s; they run alongside the job loop so a slow
	// probe never delays prepare and revoke tasks queued behind it.
	probes := make(chan struct{}, maxConcurrentProbes)
	for {
		var jobs []contract.Job
		if e := c.call(ctx, "GET", "/api/node/v1/jobs", nil, &jobs); e == nil {
			for _, j := range jobs {
				if j.Kind != "probe" {
					c.runJob(ctx, j)
					continue
				}
				select {
				case probes <- struct{}{}:
				case <-ctx.Done():
				}
				go func() {
					defer func() { <-probes }()
					c.runJob(ctx, j)
				}()
			}
		}
		// Ask again at once while work is queued; idle polling stays at 2 s.
		wait := 2 * time.Second
		if len(jobs) > 0 {
			wait = 0
		}
		select {
		case <-ctx.Done():
			c.Manager.Shutdown()
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}
