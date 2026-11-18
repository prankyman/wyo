package exchange_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/exchange"
	"github.com/prankyman/wyo/internal/store"
)

func TestConnectingSavesTheSettingOnlyWhenTheFulfilmentCentreAnswersAndTheKeyMayWrite(t *testing.T) {
	t.Parallel()

	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()

	junk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>not an API</html>"))
	}))
	t.Cleanup(junk.Close)

	// The fake accepts any key; a fulfilment centre that does not know the key answers 401 in the API's shape.
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"unauthorized"}}`))
	}))
	t.Cleanup(refusing.Close)

	cases := []struct {
		name string
		// setup changes the fake fulfilment centre; address and key replace the rig's own when given.
		setup   map[string]any
		address string
		want    string
	}{
		{name: "a key that may only read", setup: map[string]any{"access": "read_only"}, want: "fc_read_only_key"},
		{name: "the API switched off", setup: map[string]any{"api_enabled": false}, want: "fc_api_off"},
		{name: "a key that may not use the API", setup: map[string]any{"forbidden": true}, want: "fc_forbidden"},
		{name: "a fulfilment centre in maintenance", setup: map[string]any{"maintenance": true}, want: "fc_unavailable"},
		{name: "a key it does not know", address: refusing.URL, want: "fc_key_refused"},
		{name: "an address nobody answers at", address: down.URL, want: "fc_no_connection"},
		{name: "an address that is no API", address: junk.URL, want: "fc_unreadable"},
		{name: "an address that is not an address", address: "ftp://example.test", want: "invalid_address"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)

			if tc.setup != nil {
				do(t, r.fc, http.MethodPost, "/__control/setup", tc.setup, nil)
			}

			address, key := r.fcURL, r.fcKey
			if tc.address != "" {
				address = tc.address
			}

			_, err := r.svc.Connect(t.Context(), address, key)

			var refused *exchange.Refused
			if !errors.As(err, &refused) || refused.Code != tc.want {
				t.Fatalf("Connect: %v, want the refusal %q", err, tc.want)
			}

			if strings.Contains(err.Error(), r.fcKey) {
				t.Fatalf("the key is in the error: %v", err)
			}

			if _, err := r.svc.Connection(t.Context()); !errors.Is(err, exchange.ErrNotFilled) {
				t.Fatalf("a refused setting was saved: %v", err)
			}

			if _, err := r.kit.Vault.Get(t.Context(), "fulfilment.key"); err == nil {
				t.Fatal("the refused key was saved")
			}

			if r.gate.listed() != "" {
				t.Fatalf("the outbound gate lists %q for a refused setting", r.gate.listed())
			}
		})
	}
}

func TestConnectingSavesTheAddressAndTheKeyListsTheAddressAndCheckingAgainFollowsTheFulfilmentCentre(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	do(t, r.fc, http.MethodPost, "/__control/setup", map[string]any{"seller_name": "Seller Ltd"}, nil)

	conn, err := r.svc.Connect(t.Context(), r.fcURL+"/api/v1/", r.fcKey)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if conn.Address != r.fcURL || conn.State != "working" || conn.SellerName != "Seller Ltd" || conn.KeyLast4 != r.fcKey[len(r.fcKey)-4:] ||
		len(conn.Marketplaces) == 0 || conn.EventsMode != "poll" {
		t.Fatalf("the setting: %+v", conn)
	}

	if saved, err := r.kit.Vault.Get(t.Context(), "fulfilment.key"); err != nil || saved != r.fcKey {
		t.Fatalf("the key in the secret store: %q, %v", saved, err)
	}

	if r.gate.listed() != r.fcURL {
		t.Fatalf("the gate lists %q", r.gate.listed())
	}

	// The key is changed to read-only at the fulfilment centre: checking again says so, and the exchange stops asking.
	do(t, r.fc, http.MethodPost, "/__control/setup", map[string]any{"access": "read_only"}, nil)

	if conn, err = r.svc.Check(t.Context()); err != nil || conn.State != "read_only_key" {
		t.Fatalf("Check: %+v, %v", conn, err)
	}

	before := len(r.fcCalls(t))

	r.newOrder(t, map[string]any{})
	r.orders(t)
	r.events(t)
	r.states(t)

	if after := len(r.fcCalls(t)); after != before {
		t.Fatalf("the exchange asked the fulfilment centre %d times with a key that may only read", after-before)
	}

	do(t, r.fc, http.MethodPost, "/__control/setup", map[string]any{"access": "read_write"}, nil)

	if conn, err = r.svc.Check(t.Context()); err != nil || conn.State != "working" {
		t.Fatalf("Check: %+v, %v", conn, err)
	}

	r.orders(t)

	if after := len(r.fcCalls(t)); after == before {
		t.Fatal("the exchange did not go on when the key could write again")
	}

	if warehouses := r.svc.FcWarehouses(t.Context()); len(warehouses) == 0 {
		t.Fatal("the fulfilment centre's warehouses are not listed")
	}
}

func TestCheckingAgainAnEmptySettingIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	if _, err := r.svc.Check(t.Context()); !errors.Is(err, exchange.ErrNotFilled) {
		t.Fatalf("Check: %v", err)
	}

	if _, err := r.svc.SetEventsMode(t.Context(), "notify", "x"); !errors.Is(err, exchange.ErrNotFilled) {
		t.Fatalf("SetEventsMode: %v", err)
	}

	if warehouses := r.svc.FcWarehouses(t.Context()); warehouses != nil {
		t.Fatalf("warehouses of an empty setting: %v", warehouses)
	}

	if notices, err := r.svc.Notices(t.Context()); err != nil || len(notices) != 0 {
		t.Fatalf("notices of an empty setting: %+v, %v", notices, err)
	}
}

func TestAFulfilmentCentreThatStopsAnsweringStopsTheExchangeUntilItAnswersAgain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		setup map[string]any
		state string
		// runs says whether the exchange goes on asking while in this state.
		runs bool
		code string
	}{
		{name: "the API switched off", setup: map[string]any{"api_enabled": false}, state: "api_off", code: "fc_key_refused"},
		{name: "a maintenance", setup: map[string]any{"maintenance": true}, state: "unavailable", runs: true, code: "fc_no_answer"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)
			do(t, r.fc, http.MethodPost, "/__control/setup", tc.setup, nil)

			r.newOrder(t, map[string]any{})
			r.orders(t)

			conn, err := r.svc.Connection(t.Context())
			if err != nil || conn.State != tc.state {
				t.Fatalf("the state after the failure: %+v, %v", conn, err)
			}

			notices, err := r.svc.Notices(t.Context())
			if err != nil || len(notices) != 1 || notices[0].Code != tc.code {
				t.Fatalf("the notices: %+v, %v", notices, err)
			}

			before := len(r.fcCalls(t))
			r.orders(t)

			if asked := len(r.fcCalls(t)) > before; asked != tc.runs {
				t.Fatalf("the exchange asked again: %v, want %v", asked, tc.runs)
			}

			do(t, r.fc, http.MethodPost, "/__control/setup", map[string]any{"api_enabled": true, "maintenance": false}, nil)

			if conn, err = r.svc.Check(t.Context()); err != nil || conn.State != "working" {
				t.Fatalf("Check: %+v, %v", conn, err)
			}

			r.orders(t)

			if x := r.exchanged(t, "100000001"); x.State != "sent" {
				t.Fatalf("the order after the fulfilment centre answered again: %+v", x)
			}
		})
	}
}

func TestEventsAreAskedForRegularlyWhilePollingAndOnlyEveryTenMinutesBesideNotifications(t *testing.T) {
	t.Parallel()

	asked := func(r *rig) int {
		n := 0

		for _, c := range r.fcCalls(t) {
			if c.Method == http.MethodGet && strings.HasSuffix(c.Path, "/events") {
				n++
			}
		}

		return n
	}

	r := newRig(t)
	r.connect(t)
	r.notifying(t)

	r.events(t)
	r.kit.Clock.Advance(time.Minute)
	r.events(t)

	if got := asked(r); got != 1 {
		t.Fatalf("events were asked %d times in two minutes beside notifications, want 1", got)
	}

	r.kit.Clock.Advance(10 * time.Minute)
	r.events(t)

	if got := asked(r); got != 2 {
		t.Fatalf("events were asked %d times after ten minutes, want 2", got)
	}

	// Back to asking: every run asks.
	if _, err := r.svc.SetEventsMode(t.Context(), "poll", ""); err != nil {
		t.Fatalf("SetEventsMode: %v", err)
	}

	r.events(t)
	r.events(t)

	if got := asked(r); got != 4 {
		t.Fatalf("events were asked %d times, want 4", got)
	}
}

func TestSwitchingNotificationsOnNeedsASecretThatCanBeOne(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	for _, secret := range []string{"", "not a secret", "whsec_AAAA"} {
		if _, err := r.svc.SetEventsMode(t.Context(), "notify", secret); !errors.Is(err, exchange.ErrInvalidSecret) {
			t.Fatalf("SetEventsMode with %q: %v", secret, err)
		}
	}

	n := r.notifying(t)

	again, err := r.svc.SetEventsMode(t.Context(), "notify", "")
	if err != nil || again.HookText != n.text || again.EventsMode != "notify" {
		t.Fatalf("notifications switched on again without a new secret: %+v, %v", again, err)
	}

	if exchange.HookPath(n.text) != "/hooks/fulfilment/"+n.text {
		t.Fatalf("the notifications' address: %q", exchange.HookPath(n.text))
	}

	polling, err := r.svc.SetEventsMode(t.Context(), "poll", "")
	if err != nil || polling.EventsMode != "poll" {
		t.Fatalf("SetEventsMode: %+v, %v", polling, err)
	}

	if err := r.svc.Receive(t.Context(), n.text, r.notification(t, n, []byte(`{}`), "msg", r.kit.Clock.Now())); !errors.Is(err, exchange.ErrUnknownHook) {
		t.Fatalf("a notification while asking: %v", err)
	}
}

func TestNoticesSayWhatNeedsTheOwner(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)
	do(t, r.fc, http.MethodPost, "/__control/products", map[string]any{"unknown": []string{"ART-HELD"}}, nil)

	r.send(t, map[string]any{"article": "ART-HELD"})                            // held
	_, _ = r.send(t, map[string]any{})                                          // sent and not reserved in a quarter of the deadline
	refused, _ := r.send(t, map[string]any{"required_meta": []string{"sgtin"}}) // a refused code
	byWarehouse, _ := r.send(t, map[string]any{})                               // cancelled at the warehouse
	waiting := r.packedOrder(t, false)                                          // a hand-over waiting for a point
	r.reserve(t, refused)
	r.pack(t, refused, markingCode)
	r.verdict(t, refused, "incorrectSeparator")
	r.verdicts(t, 20*time.Second)
	r.step(t, r.exchanged(t, byWarehouse).FcID, "cancel")
	r.events(t)
	r.setHandover(t, nil, "")
	r.ask(t, waiting)

	// An account whose key may only read is a notice too.
	err := r.kit.Store.Do(t.Context(), func(q *store.Queries) error {
		return q.UpdateAccountKey(t.Context(), r.account.ID, store.AccountKey{
			State: "working", SellerProfileID: fakeSeller, KeyLast4: "…" + r.token[len(r.token)-4:], KeyType: "personal", ReadOnly: true,
			CheckedAt: r.kit.Clock.Now(),
		})
	})
	if err != nil {
		t.Fatalf("UpdateAccountKey: %v", err)
	}

	r.kit.Clock.Advance(13 * time.Hour)

	notices, err := r.svc.Notices(t.Context())
	if err != nil {
		t.Fatalf("Notices: %v", err)
	}

	got := map[string]any{}
	for _, n := range notices {
		got[n.Code] = n.Params["count"]
		if n.Code == "fc_account_read_only" {
			got[n.Code] = n.Params["account"]
		}
	}

	want := map[string]any{
		"fc_orders_held": int64(1), "fc_not_reserved": int64(1), "fc_codes_refused": int64(1), "fc_cancelled_by_warehouse": int64(1),
		"fc_handover_waits": int64(1), "fc_account_read_only": "Shop",
	}

	for code, value := range want {
		if got[code] != value {
			t.Fatalf("the notice %s: %v, want %v (all: %v)", code, got[code], value, got)
		}
	}
}

func TestCheckingAgainFindsWhatTheFulfilmentCentreAnswersNow(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		setup map[string]any
		want  string
	}{
		{name: "the API switched off", setup: map[string]any{"api_enabled": false}, want: "api_off"},
		{name: "a key that may not use the API", setup: map[string]any{"forbidden": true}, want: "forbidden"},
		{name: "a maintenance", setup: map[string]any{"maintenance": true}, want: "unavailable"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)
			do(t, r.fc, http.MethodPost, "/__control/setup", tc.setup, nil)

			conn, err := r.svc.Check(t.Context())
			if err != nil || conn.State != tc.want {
				t.Fatalf("Check: %+v, %v", conn, err)
			}

			if warehouses := r.svc.FcWarehouses(t.Context()); tc.want == "unavailable" && warehouses != nil {
				t.Fatalf("warehouses while the fulfilment centre is in maintenance: %v", warehouses)
			}
		})
	}
}

func TestOnlyTheLastFourCharactersOfAKeyAreKept(t *testing.T) {
	t.Parallel()

	cases := []struct {
		key  string
		want string
	}{
		{key: "abcdefgh", want: "efgh"},
		{key: "abc", want: "abc"},
	}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)

			conn, err := r.svc.Connect(t.Context(), r.fcURL, tc.key)
			if err != nil || conn.KeyLast4 != tc.want {
				t.Fatalf("Connect: %+v, %v", conn, err)
			}
		})
	}
}
