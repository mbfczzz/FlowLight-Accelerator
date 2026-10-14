package control

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGameVersionsRemainImmutableAcrossActivation(t *testing.T) {
	s, token, _ := setup(t)
	patch := map[string]any{"regions": []string{"kr"}, "draft": c.Profile{Processes: []string{"first.exe"}, UDP: true}}
	if w := request(s, token, "PATCH", "/api/admin/v1/games/valorant", patch, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for i := 0; i < 2; i++ {
		if w := request(s, token, "POST", "/api/admin/v1/games/valorant/profiles/publish", nil, ""); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		patch["draft"] = c.Profile{Processes: []string{"next.exe"}, UDP: true}
		if w := request(s, token, "PATCH", "/api/admin/v1/games/valorant", patch, ""); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	if w := request(s, token, "POST", "/api/admin/v1/games/valorant/profiles/1/publish", nil, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var first GameVersion
	s.Store.DB.First(&first, "game_id = ? AND version = ?", "valorant", 1)
	if len(first.Profile.Processes) != 1 || first.Profile.Processes[0] != "first.exe" {
		t.Fatal("published profile mutated")
	}
	if w := request(s, token, "POST", "/api/admin/v1/games/valorant/profiles/publish", nil, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var game Game
	s.Store.DB.First(&game, "id = ?", "valorant")
	if game.PublishedVersion != 3 {
		t.Fatal("version number reused after rollback")
	}
}

func TestCancelledValidationReleasesPublicationLock(t *testing.T) {
	s, token, user := setup(t)
	fixture(t, s, user)
	var line Line
	s.Store.DB.First(&line, "id = ?", "line-1")
	line.EndpointIDs = []string{"ep-1"}
	s.Store.DB.Save(&line)
	s.Store.DB.Model(&Endpoint{}).Where("id = ?", "ep-1").Update("cipher", "2022-blake3-aes-128-gcm")
	w := request(s, token, "POST", "/api/admin/v1/lines/line-1/publish", nil, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var version LineVersion
	if err := json.Unmarshal(w.Body.Bytes(), &version); err != nil {
		t.Fatal(err)
	}
	var session Session
	s.Store.DB.First(&session, "line_version = ? AND purpose = ?", version.ID, "validation")
	w = request(s, token, "POST", "/api/admin/v1/sessions/"+session.ID+"/stop", nil, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.Store.DB.First(&version, "id = ?", version.ID)
	if version.Status != "failed" {
		t.Fatalf("cancelled validation stuck: %s", version.Status)
	}
	s.Store.DB.First(&line, "id = ?", line.ID)
	if line.PublishedVersion != "version-1" {
		t.Fatal("cancelled publication replaced active version")
	}
}

func TestEnrollmentOneTimeAndRealMTLS(t *testing.T) {
	s, token, _ := setup(t)
	s.Config.RequireMTLS = true
	s.Store.DB.Create(&Node{ID: "enrollment-test", Name: "test", Enabled: true, Capabilities: c.Capabilities{TCP: true, UDP: true, IPv4: true, Managed: true}})
	w := request(s, token, "POST", "/api/admin/v1/nodes/enrollment-test/enrollment", nil, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var enrollment map[string]string
	json.Unmarshal(w.Body.Bytes(), &enrollment)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"node_id": "enrollment-test", "bootstrap_secret": enrollment["bootstrap_secret"], "csr": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr}))}
	w = request(s, "", "POST", "/api/node/v1/register", body, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var identity map[string]string
	json.Unmarshal(w.Body.Bytes(), &identity)
	if again := request(s, "", "POST", "/api/node/v1/register", body, ""); again.Code == 200 {
		t.Fatal("enrollment credential reused")
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(s.CA.PEM)
	server := httptest.NewUnstartedServer(s.Handler())
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.VerifyClientCertIfGiven, ClientCAs: pool}
	server.StartTLS()
	defer server.Close()
	client := server.Client()
	client.Timeout = 5 * time.Second
	heartbeat := func() int {
		req, _ := http.NewRequest("POST", server.URL+"/api/node/v1/heartbeat", strings.NewReader(`{"version":"test","capabilities":{"tcp":true,"udp":true,"ipv4":true,"managed":true}}`))
		req.Header.Set("Authorization", "Bearer "+identity["token"])
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if code := heartbeat(); code != 401 {
		t.Fatalf("node bearer accepted without certificate: %d", code)
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	pair, err := tls.X509KeyPair([]byte(identity["certificate"]), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		t.Fatal(err)
	}
	transport := client.Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.Certificates = []tls.Certificate{pair}
	client.Transport = transport
	defer transport.CloseIdleConnections()
	if code := heartbeat(); code != 200 {
		t.Fatalf("enrolled mTLS failed: %d", code)
	}
	// Renewal: the old certificate keeps working until the new one is used.
	nextKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	nextCSR, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, nextKey)
	if err != nil {
		t.Fatal(err)
	}
	renewal, _ := json.Marshal(map[string]string{"csr": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: nextCSR}))})
	req, _ := http.NewRequest("POST", server.URL+"/api/node/v1/certificate", strings.NewReader(string(renewal)))
	req.Header.Set("Authorization", "Bearer "+identity["token"])
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var renewed map[string]string
	json.NewDecoder(res.Body).Decode(&renewed)
	res.Body.Close()
	if res.StatusCode != 200 || renewed["certificate"] == "" {
		t.Fatalf("certificate renewal failed: %d", res.StatusCode)
	}
	if code := heartbeat(); code != 200 {
		t.Fatalf("previous certificate rejected before the renewed one was used: %d", code)
	}
	nextDER, _ := x509.MarshalECPrivateKey(nextKey)
	nextPair, err := tls.X509KeyPair([]byte(renewed["certificate"]), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: nextDER}))
	if err != nil {
		t.Fatal(err)
	}
	next := transport.Clone()
	next.TLSClientConfig.Certificates = []tls.Certificate{nextPair}
	defer next.CloseIdleConnections()
	client.Transport = next
	if code := heartbeat(); code != 200 {
		t.Fatalf("renewed certificate rejected: %d", code)
	}
	old := transport.Clone()
	defer old.CloseIdleConnections()
	client.Transport = old
	if code := heartbeat(); code != 401 {
		t.Fatalf("retired certificate still accepted: %d", code)
	}
}
