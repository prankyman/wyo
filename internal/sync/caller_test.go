package sync_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// script is a marketplace of the test's own: it answers each call with the next status of its list (200 when the list
// is used up) and remembers what it received.
type script struct {
	mu       gosync.Mutex
	statuses []int
	headers  []http.Header
	bodies   []string
	auth     []string
	extra    http.Header
}

func (s *script) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	body, _ := io.ReadAll(r.Body)
	s.bodies = append(s.bodies, string(body))
	s.auth = append(s.auth, r.Header.Get("Authorization"))
	s.headers = append(s.headers, r.Header.Clone())

	status := http.StatusOK
	if len(s.statuses) > 0 {
		status, s.statuses = s.statuses[0], s.statuses[1:]
	}

	for name, values := range s.extra {
		w.Header()[name] = values
	}

	if status == http.StatusFound {
		http.Redirect(w, r, "/elsewhere", status)

		return
	}

	w.WriteHeader(status)
	_, _ = io.WriteString(w, `{"code":"Code`+http.StatusText(status)+`","message":"text with the key `+s.lastKey()+`"}`)
}

func (s *script) lastKey() string { return s.auth[len(s.auth)-1] }

func (s *script) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.bodies)
}

// adapter is the smallest adapter the caller needs; it names no real marketplace.
type adapter struct{}

func (adapter) Describe() marketplace.Description {
	return marketplace.Description{Code: "alpha", KeyFields: []marketplace.KeyField{{Name: "token", Secret: true}}}
}

func (adapter) Routes() []marketplace.Route {
	return []marketplace.Route{
		{Method: http.MethodGet, Template: "/api/orders/new", Group: "orders"},
		{Method: http.MethodPost, Template: "/api/orders/status", Group: "orders"},
		{Method: http.MethodPatch, Template: "/api/orders/{orderId}/cancel", Group: "orders", Write: true},
	}
}

func (adapter) Limit(group string, _ bool) (marketplace.LimitGroup, bool) {
	return sellerWarehouse, group == "orders"
}

func (adapter) Capable(marketplace.Capability, bool) bool    { return true }
func (adapter) Authorize(h http.Header, key marketplace.Key) { h.Set("Authorization", key["token"]) }
func (adapter) Hosts() []string                              { return []string{"127.0.0.1"} }

func (adapter) ErrorOf(_ int, body []byte) (string, string) {
	code, rest, _ := strings.Cut(strings.TrimPrefix(string(body), `{"code":"`), `","message":"`)

	return code, strings.TrimSuffix(strings.TrimSpace(rest), `"}`)
}

const secretKey = "a-distinctive-key-of-the-seller-0123456789"

// answer is what the adapter got: the status and the body, read and closed.
type answer struct {
	StatusCode int
	Body       string
}

func send(t *testing.T, k *synckit.Kit, target sync.Target, method, path, body string) (answer, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, k.URL+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	res, err := k.Caller.Conn(target).Doer.Do(req)
	if err != nil {
		return answer{}, err
	}

	defer func() { _ = res.Body.Close() }()

	read, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read the answer: %v", err)
	}

	return answer{StatusCode: res.StatusCode, Body: string(read)}, nil
}

func savedTarget(t *testing.T, k *synckit.Kit) sync.Target {
	t.Helper()

	if err := k.Vault.Put(t.Context(), "account.7.key", secretKey); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	return sync.Target{Adapter: adapter{}, AccountID: 7, Job: "orders.new", SecretName: "account.7.key"}
}

func outcomes(rows []store.JournalEntry) []string {
	var list []string
	for i := len(rows) - 1; i >= 0; i-- {
		list = append(list, rows[i].Outcome)
	}

	return list
}

func TestARequestCarriesTheKeyOnlyInItsHeaderAndLeavesOneJournalRowWithoutIt(t *testing.T) {
	t.Parallel()

	marketplaceSide := &script{}
	k := synckit.New(t, marketplaceSide)

	res, err := send(t, k, savedTarget(t, k), http.MethodPatch, "/api/orders/12345/cancel?reason=x", "")
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("Do: %v, %v", res, err)
	}

	if !strings.Contains(res.Body, "CodeOK") {
		t.Fatalf("the adapter gets the answer's body: %q", res.Body)
	}

	if marketplaceSide.auth[0] != secretKey {
		t.Fatalf("Authorization %q, want the key from the secret store", marketplaceSide.auth[0])
	}

	rows := k.Rows(t)
	if len(rows) != 1 {
		t.Fatalf("%d journal rows, want 1", len(rows))
	}

	row := rows[0]
	if row.AccountID != 7 || row.Marketplace != "alpha" || row.Method != http.MethodPatch || row.Path != "/api/orders/{orderId}/cancel" ||
		row.Status != http.StatusOK || row.Outcome != journal.Done || row.Job != "orders.new" {
		t.Fatalf("row %+v", row)
	}

	if strings.Contains(row.ErrorText+row.ErrorCode+row.Path, secretKey) || strings.Contains(k.Logs.String(), secretKey) {
		t.Fatalf("the key reached the journal or a log: %+v %q", row, k.Logs.String())
	}
}

func TestARequestNoRouteDeclaresIsRefusedBeforeItLeaves(t *testing.T) {
	t.Parallel()

	marketplaceSide := &script{}
	k := synckit.New(t, marketplaceSide)

	_, err := send(t, k, savedTarget(t, k), http.MethodDelete, "/api/orders/12345", "")
	if !errors.Is(err, sync.ErrNotDeclared) || marketplaceSide.calls() != 0 || len(k.Rows(t)) != 0 {
		t.Fatalf("err %v, %d calls, %d rows; want refused before anything left", err, marketplaceSide.calls(), len(k.Rows(t)))
	}
}

func TestAReadingIsRepeatedAfterOneAndThreeSecondsButAClientErrorOrAWriteNever(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		method, path string
		statuses     []int
		wantCalls    int
		wantOutcomes []string
		wantWaits    []time.Duration
		wantErr      string
		wantStatus   int
	}{
		{
			name: "two server errors, then a success", method: http.MethodPost, path: "/api/orders/status",
			statuses: []int{500, 503}, wantCalls: 3, wantOutcomes: []string{"server_error", "server_error", "done"},
			wantWaits: []time.Duration{time.Second, 3 * time.Second}, wantStatus: http.StatusOK,
		},
		{
			name: "three server errors", method: http.MethodGet, path: "/api/orders/new", statuses: []int{500, 500, 500},
			wantCalls: 3, wantOutcomes: []string{"server_error", "server_error", "server_error"},
			wantWaits: []time.Duration{time.Second, 3 * time.Second}, wantErr: "server_error",
		},
		{
			name: "a client error", method: http.MethodGet, path: "/api/orders/new", statuses: []int{400}, wantCalls: 1,
			wantOutcomes: []string{"refused"}, wantStatus: http.StatusBadRequest,
		},
		{
			name: "a refused key", method: http.MethodGet, path: "/api/orders/new", statuses: []int{401}, wantCalls: 1,
			wantOutcomes: []string{"key_refused"}, wantStatus: http.StatusUnauthorized,
		},
		{
			name: "a write that met a server error", method: http.MethodPatch, path: "/api/orders/1/cancel", statuses: []int{500},
			wantCalls: 1, wantOutcomes: []string{"server_error"}, wantErr: "server_error",
		},
		{
			name: "a redirect", method: http.MethodGet, path: "/api/orders/new", statuses: []int{302}, wantCalls: 1,
			wantOutcomes: []string{"refused"}, wantErr: "refused",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			marketplaceSide := &script{statuses: tc.statuses}
			k := synckit.New(t, marketplaceSide)

			res, err := send(t, k, savedTarget(t, k), tc.method, tc.path, `{"orders":[1]}`)

			var callErr *marketplace.CallError
			switch {
			case tc.wantErr != "" && (!errors.As(err, &callErr) || callErr.Outcome != tc.wantErr):
				t.Fatalf("err %v, want a CallError %s", err, tc.wantErr)
			case tc.wantErr == "" && (err != nil || res.StatusCode != tc.wantStatus):
				t.Fatalf("answer %v, %v; want the status %d handed to the adapter", res, err, tc.wantStatus)
			}

			if marketplaceSide.calls() != tc.wantCalls || !reflect.DeepEqual(outcomes(k.Rows(t)), tc.wantOutcomes) ||
				!reflect.DeepEqual(k.Waits(), tc.wantWaits) {
				t.Fatalf("calls %d, outcomes %v, waits %v", marketplaceSide.calls(), outcomes(k.Rows(t)), k.Waits())
			}

			for _, body := range marketplaceSide.bodies {
				if tc.method == http.MethodPost && body != `{"orders":[1]}` {
					t.Fatalf("a repeat sent the body %q", body)
				}
			}

			if rows := k.Rows(t); tc.wantStatus >= 400 && (rows[0].ErrorCode == "" || strings.Contains(rows[0].ErrorText, secretKey) ||
				!strings.Contains(rows[0].ErrorText, logging.Redacted)) {
				t.Fatalf("the refusal's code and text, the key blanked: %+v", rows[0])
			}
		})
	}
}

func TestTooManyRequestsIsWaitedOutInPlaceUpToThirtySecondsAndLongerByComingBackLater(t *testing.T) {
	t.Parallel()

	short := &script{statuses: []int{429}, extra: http.Header{"X-Ratelimit-Retry": {"2"}}}
	k := synckit.New(t, short)

	res, err := send(t, k, savedTarget(t, k), http.MethodGet, "/api/orders/new", "")
	if err != nil || res.StatusCode != http.StatusOK || short.calls() != 2 || !reflect.DeepEqual(k.Waits(), []time.Duration{2 * time.Second}) ||
		!reflect.DeepEqual(outcomes(k.Rows(t)), []string{"limited", "done"}) {
		t.Fatalf("a short wait: %v, %v, calls %d, waits %v, rows %v", res, err, short.calls(), k.Waits(), outcomes(k.Rows(t)))
	}

	long := &script{statuses: []int{429}, extra: http.Header{"X-Ratelimit-Retry": {"40"}}}
	k = synckit.New(t, long)

	_, err = send(t, k, savedTarget(t, k), http.MethodGet, "/api/orders/new", "")

	var callErr *marketplace.CallError
	if !errors.As(err, &callErr) || callErr.Outcome != journal.Limited || !callErr.RetryAt.Equal(synckit.Noon().Add(40*time.Second)) ||
		long.calls() != 1 || len(k.Waits()) != 0 {
		t.Fatalf("a long wait: %v, calls %d, waits %v", err, long.calls(), k.Waits())
	}

	if _, err := send(t, k, savedTarget(t, k), http.MethodGet, "/api/orders/new", ""); !errors.As(err, &callErr) || long.calls() != 1 {
		t.Fatalf("a call during the long wait must not leave: %v, calls %d", err, long.calls())
	}
}

func TestNoAnswerIsRepeatedAndThenNamedNoConnection(t *testing.T) {
	t.Parallel()

	k := synckit.New(t, &script{})
	target := savedTarget(t, k)
	k.URL = "http://127.0.0.1:1" // nothing listens on port 1

	_, err := send(t, k, target, http.MethodGet, "/api/orders/new", "")

	var callErr *marketplace.CallError
	if !errors.As(err, &callErr) || callErr.Outcome != journal.NoConnection ||
		!reflect.DeepEqual(outcomes(k.Rows(t)), []string{"no_connection", "no_connection", "no_connection"}) {
		t.Fatalf("err %v, rows %v", err, outcomes(k.Rows(t)))
	}
}

func TestAKeyIsReadFromTheSecretStoreAtEveryCallOrGivenForACheck(t *testing.T) {
	t.Parallel()

	marketplaceSide := &script{}
	k := synckit.New(t, marketplaceSide)
	target := savedTarget(t, k)

	if _, err := send(t, k, target, http.MethodGet, "/api/orders/new", ""); err != nil {
		t.Fatalf("first: %v", err)
	}

	if err := k.Vault.Put(t.Context(), "account.7.key", "the-replaced-key-of-the-seller-987"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, err := send(t, k, target, http.MethodGet, "/api/orders/new", ""); err != nil {
		t.Fatalf("second: %v", err)
	}

	check := sync.Target{Adapter: adapter{}, Job: "key.check", Key: logging.Secret("a-key-being-checked-0001")}
	if _, err := send(t, k, check, http.MethodGet, "/api/orders/new", ""); err != nil {
		t.Fatalf("check: %v", err)
	}

	want := []string{secretKey, "the-replaced-key-of-the-seller-987", "a-key-being-checked-0001"}
	if !reflect.DeepEqual(marketplaceSide.auth, want) {
		t.Fatalf("keys sent %v, want %v", marketplaceSide.auth, want)
	}

	missing := sync.Target{Adapter: adapter{}, AccountID: 8, Job: "orders.new", SecretName: "account.8.key"}
	if _, err := send(t, k, missing, http.MethodGet, "/api/orders/new", ""); !errors.Is(err, store.ErrNotFound) || marketplaceSide.calls() != 3 {
		t.Fatalf("a key that is not stored: %v, calls %d; want nothing sent", err, marketplaceSide.calls())
	}
}

func TestAJournalRowThatCannotBeWrittenNeverChangesTheAnswer(t *testing.T) {
	t.Parallel()

	k := synckit.New(t, &script{})

	if err := k.Store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	res, err := send(t, k, sync.Target{Adapter: adapter{}, Job: "key.check", Key: logging.Secret(secretKey)}, http.MethodGet, "/api/orders/new", "")
	if err != nil || res.StatusCode != http.StatusOK || !strings.Contains(k.Logs.String(), "a journal row could not be written") {
		t.Fatalf("answer %v, %v; log %q", res, err, k.Logs.String())
	}
}

// countingDoer is a client of another purpose: it counts what it is given and hands it to the test's server.
type countingDoer struct {
	next  marketplace.Doer
	calls int
}

func (c *countingDoer) Do(req *http.Request) (*http.Response, error) {
	c.calls++

	return c.next.Do(req)
}

func TestATargetWithAClientOfItsOwnIsSentThroughItAndStillWritesTheJournal(t *testing.T) {
	t.Parallel()

	k := synckit.New(t, &script{})
	own := &countingDoer{next: k.HTTP}

	target := savedTarget(t, k)
	target.Client = own

	res, err := send(t, k, target, http.MethodGet, "/api/orders/new", "")
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("Do: %v, %v", res, err)
	}

	if own.calls != 1 || len(k.Rows(t)) != 1 {
		t.Fatalf("the target's own client sent %d requests and the journal has %d rows; want 1 and 1", own.calls, len(k.Rows(t)))
	}
}
