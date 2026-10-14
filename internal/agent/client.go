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
type Client struct {
	clockOffset atomic.Int64
	Config      Config
	HTTP        *http.Client
	Token       string
	Manager     *Manager
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
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13}}
	if cfg.ControlCA != "" {
		b, e := os.ReadFile(cfg.ControlCA)
		if e != nil {
			return nil, e
		}
		pool, e := x509.SystemCertPool()
		if e != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(b) {
			return nil, errors.New("invalid control CA")
		}
		transport.TLSClientConfig.RootCAs = pool
	}
	client := &Client{Config: cfg, HTTP: &http.Client{Transport: transport, Timeout: 15 * time.Second}}
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
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			return nil, e
		}
		csr, e := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: cfg.NodeID}}, key)
		if e != nil {
			return nil, e
		}
		var result struct {
			Token       string `json:"token"`
			Certificate string `json:"certificate"`
		}
		if e = client.call(context.Background(), "POST", "/api/node/v1/register", map[string]any{"node_id": cfg.NodeID, "bootstrap_secret": cfg.Bootstrap, "csr": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr}))}, &result); e != nil {
			return nil, e
		}
		der, _ := x509.MarshalECPrivateKey(key)
		id = identity{Token: result.Token, Certificate: result.Certificate, Key: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))}
		b, _ = json.Marshal(id)
		if e = atomicWrite(path, b); e != nil {
			return nil, e
		}
	} else {
		return nil, e
	}
	certificate, e := tls.X509KeyPair([]byte(id.Certificate), []byte(id.Key))
	if e != nil {
		return nil, e
	}
	transport.TLSClientConfig.Certificates = []tls.Certificate{certificate}
	client.Token = id.Token
	client.Manager, e = NewManager(filepath.Join(cfg.DataDir, "runtimes"), cfg.Binaries)
	return client, e
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
	res, e := c.HTTP.Do(req)
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
	e := c.call(ctx, "POST", "/api/node/v1/heartbeat", map[string]any{"version": "0.1.0", "capabilities": map[string]bool{"tcp": true, "udp": true, "ipv4": true, "managed": true}, "actual_versions": c.Manager.ActualVersions()}, &out)
	if e != nil {
		return e
	}
	offset := out.ServerTime.Sub(time.Now().UTC())
	c.clockOffset.Store(int64(offset))
	for id, expiry := range out.Leases {
		out.Leases[id] = expiry.Add(-offset)
	}
	return c.Manager.Renew(out.Leases)
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
		return false, e.Error(), probe
	}
	return true, "", probe
}
func (c *Client) Run(ctx context.Context) error {
	for {
		if e := c.heartbeat(ctx); e == nil {
			break
		}
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
		for {
			_ = c.heartbeat(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
	for {
		var jobs []contract.Job
		if e := c.call(ctx, "GET", "/api/node/v1/jobs", nil, &jobs); e == nil {
			for _, j := range jobs {
				taskCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
				ok, message, probe := c.execute(taskCtx, j)
				cancel()
				_ = c.call(ctx, "POST", "/api/node/v1/jobs/"+j.ID+"/result", map[string]any{"claim": j.Claim, "success": ok, "error": message, "probe": probe}, &map[string]any{})
			}
		}
		select {
		case <-ctx.Done():
			c.Manager.Lock()
			ids := []string{}
			for id := range c.Manager.Processes {
				ids = append(ids, id)
			}
			c.Manager.Unlock()
			for _, id := range ids {
				_ = c.Manager.Stop(id)
			}
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
