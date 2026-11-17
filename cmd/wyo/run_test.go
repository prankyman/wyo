package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/buildinfo"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/testkit/httpkit"
)

type environment map[string]string

func (e environment) lookup(key string) (string, bool) {
	v, ok := e[key]

	return v, ok
}

// started runs the program on a listener of the test's own and returns its address and a way to stop it.
type started struct {
	addr   string
	stop   context.CancelFunc
	done   chan int
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func start(t *testing.T, args []string, env environment) *started {
	t.Helper()

	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s := &started{addr: ln.Addr().String(), stop: cancel, done: make(chan int, 1), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}

	go func() {
		s.done <- run(ctx, args, env.lookup, s.stdout, s.stderr, func(string) (net.Listener, error) { return ln, nil })
	}()

	return s
}

func (s *started) get(t *testing.T, path string) (int, string) {
	t.Helper()

	got := httpkit.Get(t, s.addr, path)

	return got.Status, got.Body
}

func (s *started) stopAndWait(t *testing.T) int {
	t.Helper()

	s.stop()

	select {
	case code := <-s.done:
		return code
	case <-time.After(10 * time.Second):
		t.Fatal("the program did not stop")

		return -1
	}
}

func TestVersionFlagPrintsTheVersionAndExits(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"--version"}, environment{}.lookup, &stdout, &stderr, nil)

	if code != 0 || strings.TrimSpace(stdout.String()) != "dev" || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestBadSettingsExitWithTwoAndSayWhy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "an unknown flag", args: []string{"--nope"}, want: "nope"},
		{name: "a bad address", args: []string{"--listen", "nowhere"}, want: "listen"},
		{name: "an unknown mode", args: []string{"--mode", "staging"}, want: "staging"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			code := run(context.Background(), tc.args, environment{}.lookup, &stdout, &stderr, nil)

			if code != 2 || !strings.Contains(stderr.String(), tc.want) || stdout.Len() != 0 {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestHelpExitsWithZero(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"--help"}, environment{}.lookup, &stdout, &stderr, nil)

	if code != 0 || !strings.Contains(stderr.String(), "listen") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestAListenAddressThatCannotBeUsedExitsWithOne(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	failing := func(string) (net.Listener, error) { return nil, errors.New("address in use") }

	code := run(context.Background(), []string{"--listen", "127.0.0.1:8790"}, environment{}.lookup, &stdout, &stderr, failing)

	if code != 1 || !strings.Contains(stderr.String(), "address in use") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestServesTheInterfaceAndMetaThenStopsGracefully(t *testing.T) {
	t.Parallel()

	s := start(t, nil, environment{})

	status, body := s.get(t, "/api/v1/meta")
	if status != http.StatusOK || !strings.Contains(body, `"name":"wyo"`) || !strings.Contains(body, `"version":"dev"`) {
		t.Fatalf("meta: %d %s", status, body)
	}

	if status, _ := s.get(t, "/"); status != http.StatusOK {
		t.Fatalf("the start page answered %d", status)
	}

	if status, body := s.get(t, "/api/v1/nothing"); status != http.StatusNotFound || !strings.Contains(body, "not_found") {
		t.Fatalf("an unknown api address: %d %s", status, body)
	}

	if code := s.stopAndWait(t); code != 0 {
		t.Fatalf("exit code %d after a clean stop; stderr %q", code, s.stderr.String())
	}
}

func TestMetaReportsTheModeOfTheBuild(t *testing.T) {
	t.Parallel()

	wantMode := config.ModeRelease
	if buildinfo.TestBuild {
		wantMode = config.ModeDevelopment
	}

	s := start(t, nil, environment{})

	_, body := s.get(t, "/api/v1/meta")
	if !strings.Contains(body, `"mode":"`+string(wantMode)+`"`) {
		t.Fatalf("body %s, want mode %s", body, wantMode)
	}

	s.stopAndWait(t)
}

func TestLogsGoToStderrAsJSON(t *testing.T) {
	t.Parallel()

	s := start(t, nil, environment{})

	s.get(t, "/api/v1/meta")
	s.stopAndWait(t)

	if !strings.Contains(s.stderr.String(), `"msg":"listening"`) || s.stdout.Len() != 0 {
		t.Fatalf("stderr %q, stdout %q", s.stderr.String(), s.stdout.String())
	}
}

func TestListenOpensAPortAndReportsAFailure(t *testing.T) {
	t.Parallel()

	ln, err := listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	defer func() { _ = ln.Close() }()

	if _, err := listen(ln.Addr().String()); err == nil {
		t.Fatal("an address in use must be reported")
	}
}
