package cli

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/prankyman/wyo/internal/release"
)

// System is the machine a command runs on. The commands reach users, files, the service manager, names and addresses only
// through it, so their tests give them a made-up machine.
type System interface {
	// Euid is the user the command runs as; 0 is root.
	Euid() int
	// OwnerOf is the user and group that own path.
	OwnerOf(path string) (uid, gid int, err error)
	// BecomeUser makes the whole process run as uid and gid from now on.
	BecomeUser(uid, gid int) error
	// Chown gives path to uid and gid.
	Chown(path string, uid, gid int) error
	// Executable is the path of the running program.
	Executable() (string, error)
	// Service is the wyo server's service.
	Service() release.Service
	// LookupHost is what a name points to.
	LookupHost(ctx context.Context, host string) ([]string, error)
	// OwnAddresses are this machine's addresses, loopback left out.
	OwnAddresses() ([]string, error)
	// FreeBytes is the free space of the disk path lies on.
	FreeBytes(path string) (uint64, error)
	// Dial says whether something answers on addr.
	Dial(ctx context.Context, addr string) error
	// CertificateNotAfter is the end of the certificate addr presents for domain.
	CertificateNotAfter(ctx context.Context, addr, domain string) (time.Time, error)
	// IsTerminal says whether the standard input is a keyboard.
	IsTerminal() bool
	// ReadSecret reads one line from the keyboard without showing it.
	ReadSecret(stdin io.Reader) (string, error)
	// Pause waits for d or until ctx ends.
	Pause(ctx context.Context, d time.Duration) error
}

// OS is the real machine.
type OS struct{}

var _ System = OS{}

// Euid is the effective user.
func (OS) Euid() int { return os.Geteuid() }

// OwnerOf reads the owner of path.
func (OS) OwnerOf(path string) (int, int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, fmt.Errorf("owner of %s: %w", path, err)
	}

	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("owner of %s: the system does not tell", path)
	}

	return int(st.Uid), int(st.Gid), nil
}

// BecomeUser drops root's rights for good.
func (OS) BecomeUser(uid, gid int) error {
	return becomeUser(identityCalls{groups: syscall.Setgroups, group: syscall.Setgid, user: syscall.Setuid}, uid, gid)
}

// identityCalls are the system's three calls that change who a process is.
type identityCalls struct {
	groups func([]int) error
	group  func(int) error
	user   func(int) error
}

// becomeUser sets the groups, then the group, then the user. The order matters: once the user is not root, the groups
// and the group can no longer be changed, and the process would keep root's group.
func becomeUser(calls identityCalls, uid, gid int) error {
	if err := calls.groups([]int{gid}); err != nil {
		return fmt.Errorf("set the groups: %w", err)
	}

	if err := calls.group(gid); err != nil {
		return fmt.Errorf("set the group: %w", err)
	}

	if err := calls.user(uid); err != nil {
		return fmt.Errorf("set the user: %w", err)
	}

	return nil
}

// Chown gives path to uid and gid.
func (OS) Chown(path string, uid, gid int) error {
	if err := os.Lchown(path, uid, gid); err != nil {
		return fmt.Errorf("give %s to its owner: %w", path, err)
	}

	return nil
}

// Executable is the running program's path, links followed.
func (OS) Executable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("the program's path: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("the program's path: %w", err)
	}

	return resolved, nil
}

// Service is the unit the installer writes.
func (OS) Service() release.Service {
	return release.Systemctl{Command: "systemctl", Unit: "wyo.service"}
}

// LookupHost asks the system's resolver.
func (OS) LookupHost(ctx context.Context, host string) ([]string, error) {
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("look up %s: %w", host, err)
	}

	return addrs, nil
}

// OwnAddresses lists the machine's addresses.
func (OS) OwnAddresses() ([]string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("the machine's addresses: %w", err)
	}

	var own []string

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && !ipnet.IP.IsLinkLocalUnicast() {
			own = append(own, ipnet.IP.String())
		}
	}

	return own, nil
}

// FreeBytes is the space an ordinary user may still write on path's disk.
func (OS) FreeBytes(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, fmt.Errorf("free space of %s: %w", path, err)
	}

	return st.Bavail * uint64(st.Bsize), nil //nolint:gosec // G115: a block size is never negative.
}

// Dial opens and closes a connection.
func (OS) Dial(ctx context.Context, addr string) error {
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", addr, err)
	}

	return conn.Close()
}

// CertificateNotAfter reads the certificate a TLS server on this machine presents for domain. It only reads the date: the
// certificate is not judged here, which is why it is not verified.
func (OS) CertificateNotAfter(ctx context.Context, addr, domain string) (time.Time, error) {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 5 * time.Second},
		Config:    &tls.Config{ServerName: domain, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, //nolint:gosec // G402: only the end date is read, on this machine's own port; nothing is sent.
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return time.Time{}, fmt.Errorf("the certificate of %s: %w", domain, err)
	}

	defer func() { _ = conn.Close() }()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return time.Time{}, errors.New("not a TLS connection")
	}

	chain := tlsConn.ConnectionState().PeerCertificates
	if len(chain) == 0 {
		return time.Time{}, fmt.Errorf("the certificate of %s: none presented", domain)
	}

	return chain[0].NotAfter, nil
}

// IsTerminal says whether the standard input is a keyboard.
func (OS) IsTerminal() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// ReadSecret reads a line without echo from the keyboard; from anything else it reads a plain line.
func (o OS) ReadSecret(stdin io.Reader) (string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		line, err := term.ReadPassword(int(f.Fd()))
		if err != nil {
			return "", fmt.Errorf("read the password: %w", err)
		}

		return string(line), nil
	}

	return readLine(stdin)
}

// Pause waits for d or until ctx ends.
func (OS) Pause(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// readLine reads one line, its end taken off.
func readLine(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", fmt.Errorf("read a line: %w", err)
	}

	return strings.TrimRight(line, "\r\n"), nil
}
