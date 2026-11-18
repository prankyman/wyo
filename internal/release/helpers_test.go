package release_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/prankyman/wyo/internal/release"
)

// The file names of a release, written once for every test.
var (
	programAMD64 = release.ProgramFile("amd64")
	programARM64 = release.ProgramFile("arm64")
)

const (
	checksumsFile = release.ChecksumsFile
	signatureFile = release.SignatureFile
	versionFile   = release.VersionFile
	installerFile = "install.sh"
)

type keyPair struct {
	public  ed25519.PublicKey
	private ed25519.PrivateKey
}

func newKeys(t *testing.T) keyPair {
	t.Helper()

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate a key: %v", err)
	}

	return keyPair{public: public, private: private}
}

func (k keyPair) publicPEM(t *testing.T) []byte {
	t.Helper()

	der, err := x509.MarshalPKIXPublicKey(k.public)
	if err != nil {
		t.Fatalf("encode the public key: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

// checksumsOf writes the sha256sum format: "<hex>  <name>" for each file, sorted by name.
func checksumsOf(files map[string][]byte) []byte {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}

	sort.Strings(names)

	var out strings.Builder

	for _, name := range names {
		sum := sha256.Sum256(files[name])
		fmt.Fprintf(&out, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}

	return []byte(out.String())
}

// releaseFiles makes the files of one release: the four that are listed and the list with its signature.
func releaseFiles(keys keyPair, version, program string) map[string][]byte {
	listed := map[string][]byte{
		versionFile:   []byte(version + "\n"),
		programAMD64:  []byte(program),
		programARM64:  []byte(program),
		installerFile: []byte("#!/bin/sh\n"),
	}

	list := checksumsOf(listed)

	files := map[string][]byte{checksumsFile: list, signatureFile: ed25519.Sign(keys.private, list)}
	for name, content := range listed {
		files[name] = content
	}

	return files
}

// releaseServer serves a release's files and remembers what it was asked for.
type releaseServer struct {
	*httptest.Server

	mu       sync.Mutex
	files    map[string][]byte
	requests []string
	bare     bool
}

func newReleaseServer(t *testing.T, files map[string][]byte) *releaseServer {
	t.Helper()

	srv := &releaseServer{files: files, bare: true}

	srv.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.mu.Lock()
		defer srv.mu.Unlock()

		srv.requests = append(srv.requests, strings.TrimPrefix(r.URL.Path, "/"))

		if r.URL.RawQuery != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Referer") != "" {
			srv.bare = false
		}

		content, found := srv.files[strings.TrimPrefix(r.URL.Path, "/")]
		if !found {
			http.NotFound(w, r)

			return
		}

		_, _ = w.Write(content)
	}))
	t.Cleanup(srv.Close)

	return srv
}

func (s *releaseServer) asked() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.requests...)
}

func (s *releaseServer) onlyBareRequests() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.bare
}

// readFile reads a file the test or the code under test wrote.
func readFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path) //nolint:gosec // G304: a file of the test's own temporary folder or of testdata/.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return content
}
