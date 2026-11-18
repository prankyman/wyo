package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/release"
	"github.com/prankyman/wyo/internal/server"
)

// fakeService is a made-up service manager.
type fakeService struct {
	active bool
	err    error
}

func (s *fakeService) Stop(context.Context) error  { return s.err }
func (s *fakeService) Start(context.Context) error { return s.err }
func (s *fakeService) Active(context.Context) (bool, error) {
	return s.active, s.err
}

// fakeSystem is a made-up machine: everything a command asks of it is written here by the test.
type fakeSystem struct {
	mu       sync.Mutex
	euid     int
	owner    [2]int
	ownerErr error
	became   *[2]int
	chowned  []string
	service  *fakeService
	hosts    []string
	own      []string
	free     uint64
	dialErr  error
	notAfter time.Time
	certErr  error
	terminal bool
	secrets  []string
}

var _ System = (*fakeSystem)(nil)

func (f *fakeSystem) Euid() int { return f.euid }
func (f *fakeSystem) OwnerOf(string) (int, int, error) {
	return f.owner[0], f.owner[1], f.ownerErr
}

func (f *fakeSystem) BecomeUser(uid, gid int) error {
	f.became = &[2]int{uid, gid}

	return nil
}

func (f *fakeSystem) Chown(path string, _, _ int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.chowned = append(f.chowned, filepath.Base(path))

	return nil
}
func (f *fakeSystem) Executable() (string, error) { return "/usr/local/bin/wyo", nil }
func (f *fakeSystem) Service() release.Service {
	if f.service == nil {
		return &fakeService{}
	}

	return f.service
}

func (f *fakeSystem) LookupHost(context.Context, string) ([]string, error) {
	if len(f.hosts) == 0 {
		return nil, errors.New("no such host")
	}

	return f.hosts, nil
}
func (f *fakeSystem) OwnAddresses() ([]string, error)            { return f.own, nil }
func (f *fakeSystem) FreeBytes(string) (uint64, error)           { return f.free, nil }
func (f *fakeSystem) Dial(context.Context, string) error         { return f.dialErr }
func (f *fakeSystem) Pause(context.Context, time.Duration) error { return nil }
func (f *fakeSystem) IsTerminal() bool                           { return f.terminal }
func (f *fakeSystem) CertificateNotAfter(context.Context, string, string) (time.Time, error) {
	return f.notAfter, f.certErr
}

func (f *fakeSystem) ReadSecret(io.Reader) (string, error) {
	if len(f.secrets) == 0 {
		return "", errors.New("no more input")
	}

	s := f.secrets[0]
	f.secrets = f.secrets[1:]

	return s, nil
}

// result is what one run of a command left.
type result struct {
	code   int
	stdout string
	stderr string
}

// runWith runs the wyo server's command line args with the environment env on the machine sys.
func runWith(t *testing.T, sys System, env map[string]string, args ...string) result {
	t.Helper()

	var stdout, stderr bytes.Buffer

	code := Run(t.Context(), &Env{
		Args: args, Lookup: func(k string) (string, bool) { v, ok := env[k]; return v, ok },
		Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr, System: sys,
	})

	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// initialisedFolder is a data folder made the way `wyo init` makes it.
func initialisedFolder(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "data")
	if _, err := server.Init(t.Context(), dir, nil); err != nil {
		t.Fatal(err)
	}

	return dir
}

// passwordFile writes a password file and returns its path.
func passwordFile(t *testing.T, password string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "password")
	writeFile(t, path, password+"\n")

	return path
}
