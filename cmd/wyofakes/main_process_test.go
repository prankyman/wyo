package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

const childMarker = "wyofakes-child"

// TestMainInChildProcess is two tests in one function: started by `go test`, it starts the test binary again as a child
// that runs main() (the marker argument is how the child knows); the child's own run is the one that does it.
func TestMainInChildProcess(t *testing.T) {
	if flag.Arg(0) == childMarker {
		os.Args = []string{"wyofakes", "--wildberries", "127.0.0.1:0", "--fulfilment", "127.0.0.1:0"}

		main() // never returns: main ends with os.Exit

		return
	}

	t.Parallel()

	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMainInChildProcess$", childMarker) //nolint:gosec // runs this test binary again
			cmd.WaitDelay = 5 * time.Second

			stderr, err := cmd.StderrPipe()
			if err != nil {
				t.Fatalf("pipe: %v", err)
			}

			if err := cmd.Start(); err != nil {
				t.Fatalf("start: %v", err)
			}

			addresses := map[string]string{}

			lines := bufio.NewScanner(stderr)
			for len(addresses) < 2 && lines.Scan() {
				var entry struct {
					Msg     string `json:"msg"`
					Fake    string `json:"fake"`
					Address string `json:"address"`
				}

				if json.Unmarshal(lines.Bytes(), &entry) == nil && entry.Msg == "listening" {
					addresses[entry.Fake] = entry.Address
				}
			}

			if len(addresses) != 2 {
				t.Fatalf("the program logged the addresses %v, want both fakes", addresses)
			}

			for fake, addr := range addresses {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/__control/health", nil)
				if err != nil {
					t.Fatalf("request: %v", err)
				}

				res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
				if err != nil {
					t.Fatalf("%s: health: %v", fake, err)
				}

				_ = res.Body.Close()

				if res.StatusCode != http.StatusOK {
					t.Fatalf("%s: health answered %d", fake, res.StatusCode)
				}
			}

			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatalf("signal: %v", err)
			}

			_, _ = io.Copy(io.Discard, stderr)

			if err := cmd.Wait(); err != nil {
				t.Fatalf("after %s the program did not exit with 0: %v", sig, err)
			}
		})
	}
}
