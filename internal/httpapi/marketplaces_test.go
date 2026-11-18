package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
)

// owner is the API with its owner signed in.
type owner struct {
	f      *apikit.Fixture
	cookie *http.Cookie
}

func signedIn(t *testing.T) owner {
	t.Helper()

	f := apikit.New(t).WithOwner(t)

	return owner{f: f, cookie: f.SignIn(t)}
}

func (o owner) call(t *testing.T, method, path, body string) apikit.Response {
	t.Helper()

	return apikit.Do(t, o.f.Handler, apikit.Request{Method: method, Path: path, Body: body, Cookie: o.cookie})
}

// added adds an account of the adapter in memory through the API and returns it as answered.
func (o owner) added(t *testing.T, name, key string) gen.Account {
	t.Helper()

	got := o.call(t, http.MethodPost, "/api/v1/accounts", `{"marketplace":"memory","name":"`+name+`","key":{"token":"`+key+`"}}`)
	if got.Status != http.StatusCreated {
		t.Fatalf("add the account: %d %s", got.Status, got.Body)
	}

	return decodeAs[gen.Account](t, got.Body)
}

// decodeAs reads an answer into a type of api/wyo.yaml, refusing a field the type does not have.
func decodeAs[T any](t *testing.T, body string) T {
	t.Helper()

	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.DisallowUnknownFields()

	var v T
	if err := decoder.Decode(&v); err != nil {
		t.Fatalf("the answer is not a %T of api/wyo.yaml: %v: %s", v, err, body)
	}

	return v
}

func TestTheMarketplacesAreListedFromTheirAdapters(t *testing.T) {
	t.Parallel()

	got := signedIn(t).call(t, http.MethodGet, "/api/v1/marketplaces", "")

	// The memory adapter confirms an order (it does not claim to pack it) and declares no hand-over steps, so it needs all six.
	want := `{"marketplaces":[{"capabilities":["key.check","warehouses.read","orders.new","orders.states","orders.feed"],` +
		`"code":"memory","exchangeFlow":"confirm","givesDeadline":false,"handoverSettings":[],` +
		`"handoverSteps":["batch","move","boxes","shipping","close","documents"],"keyFields":[{"name":"token","secret":true}],"mark":"ink"}]}`
	if got.Status != http.StatusOK || got.Body != want {
		t.Fatalf("status %d\n got  %s\n want %s", got.Status, got.Body, want)
	}
}

func TestAnAddedAccountIsAnsweredWithItsKeysLastFourCharactersOnly(t *testing.T) {
	t.Parallel()

	o := signedIn(t)

	got := o.call(t, http.MethodPost, "/api/v1/accounts", `{"marketplace":"memory","name":"Основной","key":{"token":"mem-seller-7788"}}`)
	if got.Status != http.StatusCreated {
		t.Fatalf("status %d: %s", got.Status, got.Body)
	}

	noon := apikit.Noon().UTC().Format(time.RFC3339)
	want := fmt.Sprintf(`{"createdAt":%q,"deadlineHours":48,"failuresInRow":0,"hasOrders":false,"id":1,"keyLast4":"7788",`+
		`"keyType":"personal","lastCheckAt":%q,"lastSuccessAt":%q,"marketplace":"memory","missingCategories":[],"name":"Основной",`+
		`"readOnly":false,"sellerName":"Memory seller","state":"working","test":false}`, noon, noon, noon)
	if got.Body != want {
		t.Fatalf("got  %s\nwant %s", got.Body, want)
	}

	for _, path := range []string{"/api/v1/accounts", "/api/v1/accounts/1"} {
		if read := o.call(t, http.MethodGet, path, ""); read.Status != http.StatusOK || !strings.Contains(read.Body, want) {
			t.Fatalf("%s: %d %s", path, read.Status, read.Body)
		}
	}
}

func TestAnAccountThatCannotBeAddedIsRefusedWithACode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		fail       error
		wantStatus int
		wantBody   string
	}{
		{
			name: "no name", body: `{"marketplace":"memory","name":" ","key":{"token":"mem-a"}}`,
			wantStatus: http.StatusUnprocessableEntity, wantBody: `{"error":{"code":"invalid","issues":[{"path":"name","code":"required"}]}}`,
		},
		{
			name: "a key the adapter cannot read", body: `{"marketplace":"memory","name":"Магазин","key":{"token":"what"}}`,
			wantStatus: http.StatusUnprocessableEntity, wantBody: `{"error":{"code":"invalid","issues":[{"path":"key.token","code":"key_unreadable"}]}}`,
		},
		{
			name: "a refusal on the key with parameters", body: `{"marketplace":"memory","name":"Магазин","key":{"token":"mem-a"}}`,
			fail:       &marketplace.Refusal{Code: "category_missing", Field: "key.token", Params: map[string]any{"category": "marketplace"}, Status: 403},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   `{"error":{"code":"invalid","params":{"category":"marketplace"},"issues":[{"path":"key.token","code":"category_missing"}]}}`,
		},
		{
			name: "a refusal that lies on no field", body: `{"marketplace":"memory","name":"Магазин","key":{"token":"mem-a"}}`,
			fail:       &marketplace.Refusal{Code: "request_refused", Params: map[string]any{"status": 400}, Status: 400},
			wantStatus: http.StatusUnprocessableEntity, wantBody: `{"error":{"code":"request_refused","params":{"status":400}}}`,
		},
		{
			name: "no connection", body: `{"marketplace":"memory","name":"Магазин","key":{"token":"mem-a"}}`,
			fail:       &marketplace.CallError{Outcome: journal.ServerError, Status: 503},
			wantStatus: http.StatusBadGateway, wantBody: `{"error":{"code":"no_connection"}}`,
		},
		{
			name: "the marketplace's limit, with the wait it asked for", body: `{"marketplace":"memory","name":"Магазин","key":{"token":"mem-a"}}`,
			fail:       &marketplace.CallError{Outcome: journal.Limited, Status: 429, RetryAt: apikit.Noon().Add(90 * time.Second)},
			wantStatus: http.StatusServiceUnavailable, wantBody: `{"error":{"code":"marketplace_busy","params":{"retryAfterSeconds":90}}}`,
		},
		{
			name: "the marketplace's limit, with no wait named", body: `{"marketplace":"memory","name":"Магазин","key":{"token":"mem-a"}}`,
			fail:       &marketplace.CallError{Outcome: journal.Limited, Status: 429},
			wantStatus: http.StatusServiceUnavailable, wantBody: `{"error":{"code":"marketplace_busy"}}`,
		},
		{
			name: "a marketplace no adapter has", body: `{"marketplace":"nowhere","name":"Магазин","key":{"token":"mem-a"}}`,
			wantStatus: http.StatusUnprocessableEntity, wantBody: `{"error":{"code":"marketplace_unknown"}}`,
		},
		{
			name: "a field the description does not have", body: `{"marketplace":"memory","name":"Магазин","key":{"token":"mem-a"},"x":1}`,
			wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"bad_request"}}`,
		},
		{
			name: "no key at all", body: `{"marketplace":"memory","name":"Магазин"}`,
			wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"bad_request"}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			o := signedIn(t)
			o.f.Memory.Fail(tc.fail)

			got := o.call(t, http.MethodPost, "/api/v1/accounts", tc.body)
			if got.Status != tc.wantStatus || got.Body != tc.wantBody {
				t.Fatalf("status %d, body %s; want %d %s", got.Status, got.Body, tc.wantStatus, tc.wantBody)
			}

			if list := o.call(t, http.MethodGet, "/api/v1/accounts", ""); list.Body != `{"accounts":[]}` {
				t.Fatalf("an account was added: %s", list.Body)
			}
		})
	}
}

func TestAnAccountIsChangedWithinItsLimits(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	id := o.added(t, "Магазин", "mem-seller").Id
	path := fmt.Sprintf("/api/v1/accounts/%d", id)

	changed := o.call(t, http.MethodPatch, path, `{"name":"Склад «Север»","deadlineHours":24}`)
	if account := decodeAs[gen.Account](t, changed.Body); changed.Status != http.StatusOK || account.Name != "Склад «Север»" || account.DeadlineHours != 24 {
		t.Fatalf("status %d: %s", changed.Status, changed.Body)
	}

	tests := []struct {
		name, path, body string
		wantStatus       int
		wantBody         string
	}{
		{
			name: "too many hours", path: path, body: `{"deadlineHours":169}`, wantStatus: http.StatusUnprocessableEntity,
			wantBody: `{"error":{"code":"invalid","params":{"max":168,"min":1},"issues":[{"path":"deadlineHours","code":"out_of_range"}]}}`,
		},
		{
			name: "no hours", path: path, body: `{"name":"","deadlineHours":0}`, wantStatus: http.StatusUnprocessableEntity,
			wantBody: `{"error":{"code":"invalid","params":{"max":168,"min":1},"issues":[{"path":"name","code":"required"},{"path":"deadlineHours","code":"out_of_range"}]}}`,
		},
		{name: "an account that does not exist", path: "/api/v1/accounts/99", body: `{"deadlineHours":24}`, wantStatus: http.StatusNotFound, wantBody: `{"error":{"code":"not_found"}}`},
		{name: "not JSON", path: path, body: `{`, wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"bad_request"}}`},
	}

	for _, tc := range tests {
		if got := o.call(t, http.MethodPatch, tc.path, tc.body); got.Status != tc.wantStatus || got.Body != tc.wantBody {
			t.Errorf("%s: status %d, body %s", tc.name, got.Status, got.Body)
		}
	}
}

func TestAKeyIsReplacedOnlyByTheSameSellersKey(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	id := o.added(t, "Магазин", "mem-seller").Id
	path := fmt.Sprintf("/api/v1/accounts/%d/key", id)

	other := o.call(t, http.MethodPut, path, `{"key":{"token":"mem-another"}}`)
	if other.Status != http.StatusUnprocessableEntity || other.Body != `{"error":{"code":"invalid","issues":[{"path":"key.token","code":"other_seller"}]}}` {
		t.Fatalf("another seller's key: %d %s", other.Status, other.Body)
	}

	same := o.call(t, http.MethodPut, path, `{"key":{"token":"mem-seller"}}`)
	if account := decodeAs[gen.Account](t, same.Body); same.Status != http.StatusOK || account.KeyLast4 != "ller" {
		t.Fatalf("the same seller's key: %d %s", same.Status, same.Body)
	}

	if noKey := o.call(t, http.MethodPut, path, `{}`); noKey.Status != http.StatusBadRequest || noKey.Body != `{"error":{"code":"bad_request"}}` {
		t.Fatalf("no key at all: %d %s", noKey.Status, noKey.Body)
	}

	if missing := o.call(t, http.MethodPut, "/api/v1/accounts/99/key", `{"key":{"token":"mem-seller"}}`); missing.Status != http.StatusNotFound {
		t.Fatalf("an account that does not exist: %d %s", missing.Status, missing.Body)
	}
}

func TestAnAccountIsCheckedSwitchedOffAndOnWithAnEmptyBody(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	id := o.added(t, "Магазин", "mem-seller").Id

	steps := []struct {
		action    string
		fail      error
		wantState gen.AccountState
		refusal   string
	}{
		{action: "disable", wantState: gen.AccountStateDisabled},
		{action: "check", wantState: gen.AccountStateDisabled},
		{action: "enable", wantState: gen.AccountStateWorking},
		{action: "check", fail: &marketplace.Refusal{Code: "key_refused", Status: 401, State: marketplace.AccountKeyRefused, StopsJobs: true}, wantState: gen.AccountStateKeyRefused, refusal: "key_refused"},
	}

	for _, step := range steps {
		o.f.Memory.Fail(step.fail)

		got := o.call(t, http.MethodPost, fmt.Sprintf("/api/v1/accounts/%d/%s", id, step.action), `{}`)
		account := decodeAs[gen.Account](t, got.Body)

		if got.Status != http.StatusOK || account.State != step.wantState {
			t.Fatalf("%s: status %d: %s", step.action, got.Status, got.Body)
		}

		if step.refusal != "" && (account.LastRefusal == nil || account.LastRefusal.Code != step.refusal || *account.LastRefusal.Status != 401) {
			t.Fatalf("%s: the refusal is not on the account: %s", step.action, got.Body)
		}
	}

	for _, action := range []string{"check", "disable", "enable"} {
		if got := o.call(t, http.MethodPost, "/api/v1/accounts/99/"+action, `{}`); got.Status != http.StatusNotFound || got.Body != `{"error":{"code":"not_found"}}` {
			t.Fatalf("%s of an account that does not exist: %d %s", action, got.Status, got.Body)
		}
	}
}

func TestOnlyAnAccountWithoutOrdersIsDeleted(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	empty := o.added(t, "Пустой", "mem-empty").Id
	busy := o.added(t, "С заказами", "mem-busy").Id

	insertOrder(t, o.f.Store, store.Order{AccountID: busy, Number: "1", Scheme: "fbs", CreatedAt: apikit.Noon(), State: "new", UpdatedAt: apikit.Noon()})

	if got := o.call(t, http.MethodDelete, fmt.Sprintf("/api/v1/accounts/%d", busy), ""); got.Status != http.StatusConflict || got.Body != `{"error":{"code":"account_has_orders"}}` {
		t.Fatalf("an account with orders: %d %s", got.Status, got.Body)
	}

	if got := o.call(t, http.MethodDelete, fmt.Sprintf("/api/v1/accounts/%d", empty), ""); got.Status != http.StatusNoContent || got.Body != "" {
		t.Fatalf("an account without orders: %d %s", got.Status, got.Body)
	}

	if got := o.call(t, http.MethodDelete, fmt.Sprintf("/api/v1/accounts/%d", empty), ""); got.Status != http.StatusNotFound {
		t.Fatalf("a second deletion: %d %s", got.Status, got.Body)
	}

	if list := decodeAs[gen.AccountList](t, o.call(t, http.MethodGet, "/api/v1/accounts", "").Body); len(list.Accounts) != 1 || !list.Accounts[0].HasOrders {
		t.Fatalf("the list: %+v", list)
	}
}

// An account's answers never carry more of its key than the last four characters (rule R4): every call about an
// account is made, and the key's last five characters are in none of the answers.
func TestNoAccountAnswerCarriesMoreThanTheKeysLastFourCharacters(t *testing.T) {
	t.Parallel()

	const key = "mem-CANARY0123456789"

	o := signedIn(t)
	id := o.added(t, "Канарейка", key).Id
	at := fmt.Sprintf("/api/v1/accounts/%d", id)

	calls := []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/accounts", ""},
		{http.MethodGet, at, ""},
		{http.MethodPatch, at, `{"name":"Канарейка 2"}`},
		{http.MethodPut, at + "/key", `{"key":{"token":"` + key + `"}}`},
		{http.MethodPut, at + "/key", `{"key":{"token":"mem-CANARY-other-0123456789"}}`},
		{http.MethodPost, at + "/check", `{}`},
		{http.MethodPost, at + "/disable", `{}`},
		{http.MethodPost, at + "/enable", `{}`},
		{http.MethodPost, "/api/v1/accounts", `{"marketplace":"memory","name":"","key":{"token":"` + key + `"}}`},
		{http.MethodGet, "/api/v1/notices", ""},
		{http.MethodGet, "/api/v1/journal", ""},
	}

	for _, c := range calls {
		got := o.call(t, c.method, c.path, c.body)

		if strings.Contains(got.Body, key[len(key)-5:]) || strings.Contains(got.Body, "CANARY") {
			t.Fatalf("%s %s answered more of the key than its last four characters: %s", c.method, c.path, got.Body)
		}

		if c.method == http.MethodGet && strings.HasPrefix(c.path, "/api/v1/accounts") && !strings.Contains(got.Body, `"keyLast4":"6789"`) {
			t.Fatalf("%s %s does not show the last four characters: %s", c.method, c.path, got.Body)
		}
	}

	for name, logs := range map[string]string{"the API's": o.f.Logs.String(), "the engine's": o.f.Sync.Logs.String()} {
		if strings.Contains(logs, "CANARY") {
			t.Fatalf("the key is in %s log:\n%s", name, logs)
		}
	}
}

func insertOrder(t *testing.T, st *store.Store, o store.Order) int64 {
	t.Helper()

	var id int64

	if err := st.Do(t.Context(), func(q *store.Queries) error {
		var err error

		id, _, err = q.InsertOrder(t.Context(), o)

		return err
	}); err != nil {
		t.Fatalf("InsertOrder: %v", err)
	}

	return id
}
