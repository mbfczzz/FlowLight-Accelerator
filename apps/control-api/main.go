package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/mbfczzz/FlowLight-Accelerator/internal/control"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func main() {
	dir := env("DATA_DIR", "runtime/control")
	key := os.Getenv("ENCRYPTION_KEY")
	prod := os.Getenv("APP_ENV") == "production"
	if key == "" {
		if prod {
			log.Fatal("ENCRYPTION_KEY required in production")
		}
		var e error
		key, e = control.LocalKey(filepath.Join(dir, "encryption.key"))
		if e != nil {
			log.Fatal(e)
		}
	}
	store, e := control.Open(env("DB_DRIVER", "sqlite"), env("DB_DSN", filepath.Join(dir, "control.db")), key)
	if e != nil {
		log.Fatal(e)
	}
	if e = store.Seed(env("ADMIN_USERNAME", "admin"), os.Getenv("ADMIN_PASSWORD")); e != nil {
		log.Fatal(e)
	}
	proxies := []netip.Prefix{}
	for _, part := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		p, e := netip.ParsePrefix(part)
		if e != nil {
			log.Fatalf("invalid TRUSTED_PROXIES entry %q", part)
		}
		proxies = append(proxies, p)
	}
	publicURL := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_URL")), "/")
	if u, e := url.Parse(publicURL); publicURL != "" && (e != nil || u.Host == "" || (u.Scheme != "https" && (prod || u.Scheme != "http"))) {
		log.Fatal("PUBLIC_URL must be the control service's https:// address")
	}
	// Held requests for work must end well inside the node client's 15 s.
	jobWait, e := time.ParseDuration(env("JOB_WAIT", "8s"))
	if e != nil || jobWait < 0 || jobWait > 12*time.Second {
		log.Fatal("JOB_WAIT must be a duration between 0 and 12s, such as 8s")
	}
	server, e := control.New(store, control.Config{PublicURL: publicURL, NodeDist: env("NODE_DIST", "/opt/flowlight/node"), MetricsToken: os.Getenv("METRICS_TOKEN"), TrustedProxies: proxies, Origin: env("ADMIN_ORIGIN", "http://localhost:5173"), AllowImported: os.Getenv("ALLOW_IMPORTED") == "true", RequireMTLS: prod || os.Getenv("NODE_MTLS_REQUIRED") == "true", CAPath: filepath.Join(dir, "pki"), ProbeURL: os.Getenv("PROBE_URL"), ProbeUDP: os.Getenv("PROBE_UDP"), ProbeNodeID: os.Getenv("PROBE_NODE_ID"), JobWait: jobWait})
	if e != nil {
		log.Fatal(e)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go server.RunReaper(ctx)
	httpServer := &http.Server{Addr: env("LISTEN_ADDR", "127.0.0.1:8090"), Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	cert, keyfile := os.Getenv("TLS_CERT"), os.Getenv("TLS_KEY")
	if prod && (cert == "" || keyfile == "") {
		log.Fatal("production requires TLS_CERT and TLS_KEY")
	}
	if cert != "" {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(server.CA.PEM)
		httpServer.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.VerifyClientCertIfGiven, ClientCAs: pool}
	}
	// ListenAndServe returns as soon as Shutdown begins, so wait for the
	// drain to finish before exiting.
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(c)
	}()
	log.Printf("FlowLight control listening on %s", httpServer.Addr)
	if cert != "" {
		e = httpServer.ListenAndServeTLS(cert, keyfile)
	} else {
		e = httpServer.ListenAndServe()
	}
	if e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
	<-drained
}
