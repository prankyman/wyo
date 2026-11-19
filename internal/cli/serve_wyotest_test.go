//go:build wyotest

package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/config"
)

// freePort is a port nothing listens on at the moment it is asked.
func freePort(t *testing.T) int {
	t.Helper()

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("not a TCP address: %v", ln.Addr())
	}

	_ = ln.Close()

	return addr.Port
}

// With a domain and no listening address, the wyo server holds its own certificate: it listens on its HTTP and HTTPS
// ports (a test build moves them) and answers /healthz on the HTTP port to this machine, while the certificate is still
// being asked for (here from an authority that does not answer).
func TestWithADomainTheServerListensOnItsOwnPorts(t *testing.T) {
	t.Parallel()

	httpPort, httpsPort := freePort(t), freePort(t)
	env := map[string]string{
		config.EnvDomain: "wyo.example.test", config.EnvHTTPPort: fmt.Sprint(httpPort), config.EnvHTTPSPort: fmt.Sprint(httpsPort),
		config.EnvACMEDirectory: "https://127.0.0.1:1/directory",
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan int, 1)

	var stderr bytes.Buffer

	go func() {
		done <- Run(ctx, &Env{
			Args: []string{"serve", "--data", filepath.Join(t.TempDir(), "data")}, Lookup: func(k string) (string, bool) { v, ok := env[k]; return v, ok },
			Stdout: io.Discard, Stderr: &stderr, System: &fakeSystem{}, Clock: clock.Real{},
		})
	}()

	address := fmt.Sprintf("http://127.0.0.1:%d/healthz", httpPort)
	client := &http.Client{Timeout: time.Second}

	var body string

	// The start takes about a second on an idle machine and several on a busy one (more than five once in GitHub Actions,
	// with the race detector beside other packages' tests): the test waits for the answer, for the server's exit, or for
	// half a minute, whichever comes first.
	wall := clock.Real{}
	deadline := wall.Now().Add(30 * time.Second)

	for body == "" && wall.Now().Before(deadline) {
		select {
		case code := <-done:
			t.Fatalf("the server stopped before it answered: exit %d; log:\n%s", code, stderr.String())
		default:
		}

		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, address, nil)
		if resp, err := client.Do(req); err == nil {
			data, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body = string(data)

			break
		}

		_ = (OS{}).Pause(t.Context(), 50*time.Millisecond)
	}

	cancel()

	if code := <-done; code != exitOK || !strings.Contains(body, "ok") {
		t.Fatalf("exit %d, /healthz said %q; log:\n%s", code, body, stderr.String())
	}

	if !strings.Contains(stderr.String(), "wyo.example.test") {
		t.Fatalf("the log names the domain it serves: %s", stderr.String())
	}
}

func TestAWrongFileOfTheTestAuthoritysRootsStopsTheStart(t *testing.T) {
	t.Parallel()

	roots := filepath.Join(t.TempDir(), "roots.pem")
	writeFile(t, roots, "no certificate here")

	cases := map[string]string{"a file that is not there": filepath.Join(t.TempDir(), "absent.pem"), "a file without a certificate": roots}
	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := runWith(t, &fakeSystem{}, map[string]string{config.EnvDomain: "wyo.example.test", config.EnvACMERoots: file}, "serve", "--data", t.TempDir())
			if got.code != exitFailure {
				t.Fatalf("got %+v", got)
			}
		})
	}
}
