package control

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// The node package: node-install.sh, and the agent with the forwarding
// programs it runs, in Config.NodeDist/<arch>/. The control image builds
// the agent from the same source and adds pinned ssserver, sslocal and
// realm (deploy/node/fetch-tools.sh), so a node gets programs that match
// this control service from the address it must reach anyway. None of it
// is secret; the one-time enrollment secret is what admits a node.

//go:embed node-install.sh
var nodeInstallScript string

var (
	nodeArches = map[string]bool{"amd64": true, "arm64": true}
	nodeFiles  = []string{"flowlight-node", "ssserver", "sslocal", "realm"}
)

type nodeSums struct {
	once sync.Once
	text string
	err  error
}

func (s *Server) nodeInstall(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = io.WriteString(w, strings.ReplaceAll(nodeInstallScript, "\r\n", "\n"))
}

// nodeFile serves one program, or SHA256SUMS for the arch.
func (s *Server) nodeFile(w http.ResponseWriter, r *http.Request) {
	arch, name := r.PathValue("arch"), r.PathValue("name")
	if !nodeArches[arch] || s.Config.NodeDist == "" {
		fail(w, 404, "node package not found")
		return
	}
	if name == "SHA256SUMS" {
		text, e := s.nodeSums(arch)
		if e != nil {
			fail(w, 404, "node package not available on this control service")
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = io.WriteString(w, text)
		return
	}
	known := false
	for _, f := range nodeFiles {
		known = known || f == name
	}
	if !known {
		fail(w, 404, "node package not found")
		return
	}
	f, e := os.Open(filepath.Join(s.Config.NodeDist, arch, name))
	if e != nil {
		fail(w, 404, "node package not available on this control service")
		return
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		fail(w, 404, "node package not available on this control service")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, name, info.ModTime(), f)
}

// nodeSums lists the checksums of an arch's programs, computed once: the
// files are part of the image and do not change while it runs.
func (s *Server) nodeSums(arch string) (string, error) {
	v, _ := s.sums.LoadOrStore(arch, &nodeSums{})
	sums := v.(*nodeSums)
	sums.once.Do(func() {
		var b strings.Builder
		for _, name := range nodeFiles {
			f, e := os.Open(filepath.Join(s.Config.NodeDist, arch, name))
			if e != nil {
				sums.err = e
				return
			}
			h := sha256.New()
			_, e = io.Copy(h, f)
			f.Close()
			if e != nil {
				sums.err = e
				return
			}
			fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(h.Sum(nil)), name)
		}
		sums.text = b.String()
	})
	return sums.text, sums.err
}
