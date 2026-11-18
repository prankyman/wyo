package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/testkit/httpkit"
)

// listeners hands the program listeners of the test's own, on free ports, and remembers where each address went.
type listeners struct {
	mu        sync.Mutex
	asked     []string
	listening map[string]string // the address asked for -> the address it got
	opened    chan struct{}     // one message for every listener handed out
}

func (l *listeners) listen(addr string) (net.Listener, error) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	l.mu.Lock()
	l.asked = append(l.asked, addr)
	l.listening[addr] = ln.Addr().String()
	l.mu.Unlock()

	l.opened <- struct{}{}

	return ln, nil
}

// output is what the program writes, safe to read while it runs.
type output struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (o *output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.buf.Write(p)
}

func (o *output) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.buf.String()
}

type started struct {
	l      *listeners
	stop   context.CancelFunc
	done   chan int
	stdout *output
	stderr *output
}

func start(t *testing.T, args []string, listenOn func(string) (net.Listener, error)) *started {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s := &started{
		l:    &listeners{listening: map[string]string{}, opened: make(chan struct{}, 2)},
		stop: cancel, done: make(chan int, 1), stdout: &output{}, stderr: &output{},
	}
	if listenOn == nil {
		listenOn = s.l.listen
	}

	go func() { s.done <- run(ctx, args, s.stdout, s.stderr, listenOn) }()

	return s
}

// waitForTwoAddresses returns when the program has been given both listeners.
func (s *started) waitForTwoAddresses(t *testing.T) {
	t.Helper()

	timeout := time.After(10 * time.Second)

	for range 2 {
		select {
		case <-s.l.opened:
		case code := <-s.done:
			t.Fatalf("the program ended with %d: %s", code, s.stderr)
		case <-timeout:
			t.Fatalf("the program never listened on both addresses: %s", s.stderr)
		}
	}
}

func (s *started) addr(asked string) string {
	s.l.mu.Lock()
	defer s.l.mu.Unlock()

	return s.l.listening[asked]
}

func TestBothFakesAnswerOnTheAddressesTheyWereAskedToListenOn(t *testing.T) {
	t.Parallel()

	s := start(t, []string{"--marketplaces", "127.0.0.1:18791", "--fulfilment", "127.0.0.1:18792"}, nil)
	s.waitForTwoAddresses(t)

	if got := httpkit.Get(t, s.addr("127.0.0.1:18791"), "/__control/health"); got.Status != http.StatusOK || got.Body != `{"status":"ok"}` {
		t.Fatalf("the fake Wildberries answered %d %s", got.Status, got.Body)
	}

	if got := httpkit.Get(t, s.addr("127.0.0.1:18792"), "/__control/health"); got.Status != http.StatusOK {
		t.Fatalf("the fake fulfilment centre answered %d", got.Status)
	}

	// Each is the right fake: a path of one is not a path of the other (without a key both answer 401 first, so ask a control call only one has).
	if got := httpkit.Get(t, s.addr("127.0.0.1:18791"), "/__control/setup"); got.Status != http.StatusMethodNotAllowed && got.Status != http.StatusNotFound {
		t.Fatalf("GET /__control/setup on the fake Wildberries answered %d", got.Status)
	}

	s.stop()

	if code := <-s.done; code != 0 {
		t.Fatalf("exit code %d: %s", code, s.stderr.String())
	}

	for _, fake := range []string{`"fake":"marketplaces"`, `"fake":"fulfilment"`} {
		if !strings.Contains(s.stderr.String(), fake) {
			t.Errorf("the log does not name %s: %s", fake, s.stderr.String())
		}
	}

	if !strings.Contains(s.stderr.String(), `"stopping"`) {
		t.Fatalf("the program did not log that it stops: %s", s.stderr.String())
	}
}

func TestPortZeroIsAllowedAndTheChosenAddressesAreLogged(t *testing.T) {
	t.Parallel()

	s := start(t, []string{"--marketplaces", "127.0.0.1:0", "--fulfilment", "127.0.0.1:0"}, nil)
	s.waitForTwoAddresses(t)

	s.stop()
	<-s.done

	for _, chosen := range s.l.listening {
		if !strings.Contains(s.stderr.String(), chosen) {
			t.Fatalf("the chosen address %s is not logged: %s", chosen, s.stderr.String())
		}
	}
}

func TestTheDefaultAddressesAreThePortsOfTheDocuments(t *testing.T) {
	t.Parallel()

	s := start(t, nil, nil)
	s.waitForTwoAddresses(t)

	for _, asked := range []string{"127.0.0.1:8791", "127.0.0.1:8792"} {
		if s.addr(asked) == "" {
			t.Errorf("nothing listened on %s: %v", asked, s.l.asked)
		}
	}

	s.stop()
	<-s.done
}

func TestAnAddressThatIsNotThisMachineIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want int
		say  string
	}{
		{name: "every interface", args: []string{"--marketplaces", "0.0.0.0:8791"}, want: 2, say: "127.0.0.1"},
		{name: "another machine", args: []string{"--fulfilment", "192.168.1.5:8792"}, want: 2, say: "127.0.0.1"},
		{name: "no port", args: []string{"--marketplaces", "127.0.0.1"}, want: 2, say: "port"},
		{name: "the same address twice", args: []string{"--marketplaces", "127.0.0.1:9000", "--fulfilment", "127.0.0.1:9000"}, want: 2, say: "same"},
		{name: "the old flag of the fake Wildberries alone", args: []string{"--wildberries", "127.0.0.1:9000"}, want: 2, say: "wildberries"},
		{name: "an unknown flag", args: []string{"--colour", "red"}, want: 2, say: "colour"},
		{name: "an argument nobody asked for", args: []string{"extra"}, want: 2, say: "extra"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := start(t, tc.args, func(string) (net.Listener, error) { return nil, errors.New("must not listen") })

			if code := <-s.done; code != tc.want || !strings.Contains(s.stderr.String(), tc.say) {
				t.Fatalf("exit code %d, stderr %q; want %d and %q", code, s.stderr.String(), tc.want, tc.say)
			}
		})
	}
}

func TestHelpEndsWithZero(t *testing.T) {
	t.Parallel()

	s := start(t, []string{"--help"}, func(string) (net.Listener, error) { return nil, errors.New("must not listen") })

	if code := <-s.done; code != 0 || !strings.Contains(s.stderr.String(), "wildberries") {
		t.Fatalf("exit code %d, stderr %q", code, s.stderr.String())
	}
}

func TestAnAddressThatCannotBeListenedOnEndsTheProgramAndFreesTheOther(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		opened   []net.Listener
		listenOn = func(addr string) (net.Listener, error) {
			if strings.HasSuffix(addr, ":8792") {
				return nil, errors.New("address already in use")
			}

			ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
			mu.Lock()
			opened = append(opened, ln)
			mu.Unlock()

			return ln, err
		}
	)

	s := start(t, nil, listenOn)

	if code := <-s.done; code != 1 || !strings.Contains(s.stderr.String(), "address already in use") {
		t.Fatalf("exit code %d, stderr %q", code, s.stderr.String())
	}

	mu.Lock()
	defer mu.Unlock()

	for _, ln := range opened {
		if c, err := (&net.Dialer{Timeout: time.Second}).DialContext(context.Background(), "tcp", ln.Addr().String()); err == nil {
			_ = c.Close()

			t.Errorf("the listener that was opened first still accepts connections")
		}
	}
}

func TestOneAddressServesTheFakeWildberriesAndTheFakeOzon(t *testing.T) {
	t.Parallel()

	s := start(t, []string{"--marketplaces", "127.0.0.1:18795", "--fulfilment", "127.0.0.1:18796"}, nil)
	s.waitForTwoAddresses(t)

	addr := s.addr("127.0.0.1:18795")

	// The health of each: the fake Ozon takes its host off the front of the path, so its control calls answer with it.
	for _, path := range []string{"/__control/health", "/api-seller.ozon.ru/__control/health"} {
		if got := httpkit.Get(t, addr, path); got.Status != http.StatusOK || got.Body != `{"status":"ok"}` {
			t.Fatalf("GET %s answered %d %s", path, got.Status, got.Body)
		}
	}

	// A method the fake Wildberries answers, and the fake Ozon would not: without a key the fake Wildberries refuses with 401,
	// and the fake Ozon would not know the path and answer 404.
	if got := httpkit.Get(t, addr, "/api/v3/warehouses"); got.Status != http.StatusUnauthorized {
		t.Fatalf("GET /api/v3/warehouses answered %d %s, want the fake Wildberries' 401", got.Status, got.Body)
	}

	// A call to the fake Ozon without a key: the kit refuses it before any method is looked at, in the fake Ozon's words;
	// the fake Wildberries would refuse it in its own («token is missing»).
	if got := httpkit.Get(t, addr, "/api-seller.ozon.ru/v1/roles"); got.Status != http.StatusUnauthorized || !strings.Contains(got.Body, "Invalid Api-Key") {
		t.Fatalf("GET /api-seller.ozon.ru/v1/roles answered %d %s, want the fake Ozon's 401 for an invalid key", got.Status, got.Body)
	}

	s.stop()
	<-s.done
}
