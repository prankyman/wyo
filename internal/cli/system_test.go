package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/release"
)

func TestTheRealMachineAnswersWhatTheCommandsAsk(t *testing.T) {
	t.Parallel()

	sys := OS{}
	dir := t.TempDir()

	if sys.Euid() != os.Geteuid() {
		t.Fatal("Euid")
	}

	uid, gid, err := sys.OwnerOf(dir)
	if err != nil || uid != os.Getuid() || gid != os.Getgid() {
		t.Fatalf("OwnerOf: %d %d %v", uid, gid, err)
	}

	if _, _, err := sys.OwnerOf(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("OwnerOf of nothing must fail")
	}

	if err := sys.Chown(dir, uid, gid); err != nil {
		t.Fatalf("Chown to its own owner: %v", err)
	}

	if err := sys.Chown(filepath.Join(dir, "absent"), uid, gid); !os.IsNotExist(unwrapAll(err)) {
		t.Fatalf("Chown of nothing: %v", err)
	}

	if os.Geteuid() != 0 {
		if err := sys.BecomeUser(uid, gid); err == nil {
			t.Fatal("an ordinary user cannot change its groups")
		}
	}

	if path, err := sys.Executable(); err != nil || path == "" {
		t.Fatalf("Executable: %q %v", path, err)
	}

	if _, ok := sys.Service().(release.Systemctl); !ok {
		t.Fatal("the service is systemd's wyo.service")
	}

	if addrs, err := sys.LookupHost(t.Context(), "localhost"); err != nil || len(addrs) == 0 {
		t.Fatalf("LookupHost: %v %v", addrs, err)
	}

	if _, err := sys.LookupHost(t.Context(), "name.invalid"); err == nil {
		t.Fatal("a name of the reserved .invalid domain resolves to nothing")
	}

	if _, err := sys.OwnAddresses(); err != nil {
		t.Fatalf("OwnAddresses: %v", err)
	}

	if free, err := sys.FreeBytes(dir); err != nil || free == 0 {
		t.Fatalf("FreeBytes: %d %v", free, err)
	}

	if _, err := sys.FreeBytes(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("FreeBytes of nothing must fail")
	}

	if sys.IsTerminal() {
		t.Fatal("a test's standard input is not a keyboard")
	}

	if line, err := sys.ReadSecret(strings.NewReader("secret-text\nnext")); err != nil || line != "secret-text" {
		t.Fatalf("ReadSecret: %q %v", line, err)
	}

	if _, err := readLine(strings.NewReader("")); err == nil {
		t.Fatal("an empty input has no line")
	}

	if line, err := readLine(strings.NewReader("last line without its end")); err != nil || line != "last line without its end" {
		t.Fatalf("readLine: %q %v", line, err)
	}
}

func unwrapAll(err error) error {
	for {
		next := unwrap(err)
		if next == nil {
			return err
		}

		err = next
	}
}

func unwrap(err error) error {
	u, ok := err.(interface{ Unwrap() error }) //nolint:errorlint // walking the chain by hand.
	if !ok {
		return nil
	}

	return u.Unwrap()
}

func TestTheRealMachineReadsPortsAndCertificates(t *testing.T) {
	t.Parallel()

	sys := OS{}
	tlsServer := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(tlsServer.Close)

	address := strings.TrimPrefix(tlsServer.URL, "https://")
	if err := sys.Dial(t.Context(), address); err != nil {
		t.Fatalf("Dial: %v", err)
	}

	notAfter, err := sys.CertificateNotAfter(t.Context(), address, "example.com")
	if err != nil || !notAfter.Equal(tlsServer.Certificate().NotAfter) {
		t.Fatalf("CertificateNotAfter: %v %v", notAfter, err)
	}

	plain := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(plain.Close)

	if _, err := sys.CertificateNotAfter(t.Context(), strings.TrimPrefix(plain.URL, "http://"), "example.com"); err == nil {
		t.Fatal("a server without TLS has no certificate")
	}

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	closed := ln.Addr().String()
	_ = ln.Close()

	if err := sys.Dial(t.Context(), closed); err == nil {
		t.Fatal("nothing answers on a closed port")
	}
}

func TestThePauseEndsWithItsTimeOrItsContext(t *testing.T) {
	t.Parallel()

	if err := (OS{}).Pause(t.Context(), time.Millisecond); err != nil {
		t.Fatalf("a short pause: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := (OS{}).Pause(ctx, time.Hour); err == nil {
		t.Fatal("a pause whose context ended returns at once with an error")
	}
}

func TestRootsRightsAreDroppedGroupsFirstAndUserLast(t *testing.T) {
	t.Parallel()

	refused := errors.New("refused")
	cases := []struct {
		name      string
		refuseAt  string
		wantSteps string
		wantErr   string
	}{
		{name: "all three", wantSteps: "groups 997, group 997, user 998"},
		{name: "the groups refused", refuseAt: "groups", wantSteps: "groups 997", wantErr: "set the groups"},
		{name: "the group refused", refuseAt: "group", wantSteps: "groups 997, group 997", wantErr: "set the group"},
		{name: "the user refused", refuseAt: "user", wantSteps: "groups 997, group 997, user 998", wantErr: "set the user"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var steps []string

			step := func(name string, id int) error {
				steps = append(steps, fmt.Sprintf("%s %d", name, id))
				if name == tc.refuseAt {
					return refused
				}

				return nil
			}

			err := becomeUser(identityCalls{
				groups: func(ids []int) error { return step("groups", ids[0]) },
				group:  func(id int) error { return step("group", id) },
				user:   func(id int) error { return step("user", id) },
			}, 998, 997)

			if got := strings.Join(steps, ", "); got != tc.wantSteps {
				t.Fatalf("steps %q, want %q", got, tc.wantSteps)
			}

			if (tc.wantErr == "") != (err == nil) || (err != nil && (!strings.Contains(err.Error(), tc.wantErr) || !errors.Is(err, refused))) {
				t.Fatalf("error %v, want %q", err, tc.wantErr)
			}
		})
	}
}
