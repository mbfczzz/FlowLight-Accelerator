package control

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodePackageIsServedWithChecksums(t *testing.T) {
	s, token, _ := setup(t)
	dist := t.TempDir()
	os.MkdirAll(filepath.Join(dist, "amd64"), 0755)
	want := map[string]string{}
	for _, name := range nodeFiles {
		body := []byte("program " + name)
		os.WriteFile(filepath.Join(dist, "amd64", name), body, 0755)
		sum := sha256.Sum256(body)
		want[name] = hex.EncodeToString(sum[:])
	}
	os.WriteFile(filepath.Join(dist, "amd64", "secret.txt"), []byte("not served"), 0644)
	s.Config.NodeDist = dist
	s.Config.PublicURL = "https://control.example:8443"

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}

	w := get("/dist/node/install.sh")
	if w.Code != 200 || !strings.HasPrefix(w.Body.String(), "#!/usr/bin/env bash\n") || strings.Contains(w.Body.String(), "\r") {
		t.Fatalf("install script: %d %q", w.Code, w.Body.String()[:40])
	}

	w = get("/dist/node/amd64/SHA256SUMS")
	if w.Code != 200 {
		t.Fatalf("checksums: %d", w.Code)
	}
	for _, line := range strings.Split(strings.TrimSpace(w.Body.String()), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || want[fields[1]] != fields[0] {
			t.Fatalf("bad checksum line %q", line)
		}
	}

	if w = get("/dist/node/amd64/realm"); w.Code != 200 || w.Body.String() != "program realm" {
		t.Fatalf("program: %d %q", w.Code, w.Body.String())
	}
	for _, path := range []string{"/dist/node/amd64/secret.txt", "/dist/node/x86/realm", "/dist/node/amd64/..%2fsecret.txt", "/dist/node/arm64/realm"} {
		if w = get(path); w.Code != 404 {
			t.Fatalf("%s should be 404, got %d", path, w.Code)
		}
	}

	w = request(s, token, "POST", "/api/admin/v1/nodes", map[string]any{"name": "HK-01", "region": "hk", "address": "203.0.113.9", "enabled": true}, "")
	var node Node
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &node) != nil {
		t.Fatal(w.Body.String())
	}
	w = request(s, token, "POST", "/api/admin/v1/nodes/"+node.ID+"/enrollment", nil, "")
	var enroll map[string]any
	json.Unmarshal(w.Body.Bytes(), &enroll)
	if w.Code != 200 || enroll["control_url"] != "https://control.example:8443" || enroll["bootstrap_secret"] == "" {
		t.Fatalf("enrollment should carry the public address: %d %s", w.Code, w.Body.String())
	}
}
