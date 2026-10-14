package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/mbfczzz/FlowLight-Accelerator/internal/control"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	server, e := control.New(store, control.Config{Origin: env("ADMIN_ORIGIN", "http://localhost:5173"), AllowImported: os.Getenv("ALLOW_IMPORTED") == "true", RequireMTLS: prod || os.Getenv("NODE_MTLS_REQUIRED") == "true", CAPath: filepath.Join(dir, "pki"), ProbeURL: os.Getenv("PROBE_URL"), ProbeUDP: os.Getenv("PROBE_UDP"), ProbeNodeID: os.Getenv("PROBE_NODE_ID")})
	if e != nil {
		log.Fatal(e)
	}
	if e = server.ConnectCache(os.Getenv("REDIS_URL")); e != nil {
		log.Fatal("invalid Redis configuration")
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
	go func() {
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
}
