package ozon_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// The words of Ozon's key errors (.agents/research/ozon-official-seller-api-check-2026-11-18.md, section 4).
const (
	wordsInvalidKey    = "Invalid Api-Key, please check the key and try again"
	wordsDeactivated   = "Api-key is deactivated, use another one or generate a new one"
	wordsIPRestricted  = "Api-Key is restricted to specific IP addresses"
	wordsOfferNotSign  = "Offer not signed"
	wordsMissingARole  = "Api-Key is missing a required role for a method"
	wordsMethodMissing = "method is not allowed"
)

// validBody is a request each method of the fake answers with 200 once newReadyRig and a posting "100-1" are there.
func (r *rig) validBody(path string) any {
	switch path {
	case pathWarehouses:
		return map[string]any{"limit": 10}
	case pathUnfulfilled:
		return map[string]any{"filter": r.everything(), "limit": 10}
	case pathGet:
		return map[string]any{"posting_number": "100-1"}
	case pathFBO:
		return map[string]any{
			"filter": map[string]any{"since": at(r.clock.Now().Add(-24 * time.Hour)), "to": at(r.clock.Now())}, "limit": 10,
		}
	case pathProducts:
		return map[string]any{"sku": []string{"5001"}}
	default:
		return nil
	}
}

var allMethods = []string{pathRoles, pathSeller, pathWarehouses, pathUnfulfilled, pathGet, pathFBO, pathProducts}

func TestACallWithoutAKeyOrWithTheWrongClientIDIsRefusedAsInvalid(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	cases := []struct {
		name    string
		headers map[string]string
	}{
		{name: "no headers at all", headers: nil},
		{name: "no key", headers: map[string]string{"Client-Id": r.clientID}},
		{name: "no Client ID", headers: map[string]string{"Api-Key": r.key}},
		{name: "another Client ID", headers: map[string]string{"Api-Key": r.key, "Client-Id": "another-client"}},
		{name: "a key that was never set up", headers: map[string]string{"Api-Key": "another-key", "Client-Id": r.clientID}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			refused(t, r.forTest(t).send(http.MethodPost, pathRoles, tc.headers, ""), http.StatusUnauthorized, wordsInvalidKey)
		})
	}
}

func TestEveryMethodAnswersAKeyThatIsSetUp(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newPosting("100-1", 24, nil)

	for _, path := range allMethods {
		if got := r.call(path, r.validBody(path)); got.status != http.StatusOK {
			t.Fatalf("%s answered %d %s", path, got.status, got.body)
		}
	}
}

func TestTheRoleRuleOpensAMethodForAdminAnExactPathAndABeginningThatEndsAtASlash(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		roles []map[string]any
		path  string
		open  bool
	}{
		{name: "Admin opens every method", roles: []map[string]any{{"name": "Admin", "methods": []string{}}}, path: pathGet, open: true},
		{name: "Admin read only is not Admin", roles: []map[string]any{{"name": "Admin read only", "methods": []string{"/v2/warehouse/list"}}}, path: pathGet, open: false},
		{name: "the exact path", roles: []map[string]any{{"name": "Posting FBS", "methods": []string{"/v3/posting/fbs/get"}}}, path: pathGet, open: true},
		{name: "a beginning that ends at a slash", roles: []map[string]any{{"name": "Posting FBS", "methods": []string{"/v3/posting"}}}, path: pathGet, open: true},
		{name: "a beginning with the slash written", roles: []map[string]any{{"name": "Posting FBS", "methods": []string{"/v3/posting/"}}}, path: pathGet, open: true},
		{name: "the near miss: a beginning that does not end at a slash", roles: []map[string]any{{"name": "Posting FBS", "methods": []string{"/v3/post"}}}, path: pathGet, open: false},
		{name: "another method of the same version", roles: []map[string]any{{"name": "Posting FBS", "methods": []string{"/v4/posting/fbs/unfulfilled/list"}}}, path: pathGet, open: false},
		{name: "the role is found among others", roles: []map[string]any{{"name": "Warehouse", "methods": []string{"/v2/warehouse"}}, {"name": "Posting FBS", "methods": []string{"/v3/posting"}}}, path: pathGet, open: true},
		{name: "no role at all", roles: []map[string]any{}, path: pathGet, open: false},
		{name: "a role without methods", roles: []map[string]any{{"name": "Empty", "methods": []string{}}}, path: pathGet, open: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			r.newPosting("100-1", 24, nil)
			r.control("setup", map[string]any{"roles": tc.roles}, nil)

			got := r.call(tc.path, r.validBody(tc.path))
			if tc.open {
				same(t, "the status of an open method", got.status, http.StatusOK)

				return
			}

			refused(t, got, http.StatusForbidden, wordsMissingARole)
		})
	}
}

func TestTheRolesMethodIsOpenToEveryKeyThatIsSetUp(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{"roles": []map[string]any{}}, nil)

	same(t, "the status", r.call(pathRoles, nil).status, http.StatusOK)
}

func TestAKeyStateRefusesEveryMethodButRolesInTheWordsOfOzon(t *testing.T) {
	t.Parallel()

	cases := []struct {
		state string
		words string
	}{
		{state: "deactivated", words: wordsDeactivated},
		{state: "ip_restricted", words: wordsIPRestricted},
		{state: "offer_not_signed", words: wordsOfferNotSign},
	}

	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			r.newPosting("100-1", 24, nil)
			r.control("setup", map[string]any{"key_state": tc.state}, nil)

			for _, path := range allMethods[1:] {
				refused(t, r.call(path, r.validBody(path)), http.StatusForbidden, tc.words)
			}

			same(t, "the status of the roles method", r.call(pathRoles, nil).status, http.StatusOK)

			r.control("setup", map[string]any{"key_state": ""}, nil)
			same(t, "the status after the state is lifted", r.call(pathSeller, nil).status, http.StatusOK)
		})
	}
}

func TestAnEndedKeyShowsItsEndDateAndIsRefusedAsDeactivatedEverywhereElse(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	end := r.clock.Now().Add(90 * time.Second)
	r.control("setup", map[string]any{"expires_at": at(end)}, nil)

	same(t, "a call before the end", r.call(pathSeller, nil).status, http.StatusOK)

	r.clock.Advance(90 * time.Second)

	// The key ends at the moment its end date is reached.
	refused(t, r.call(pathSeller, nil), http.StatusForbidden, wordsDeactivated)

	var roles struct {
		ExpiresAt time.Time `json:"expires_at"`
	}

	r.answer(pathRoles, nil, &roles)

	if !roles.ExpiresAt.Equal(end) {
		t.Fatalf("an ended key shows the end date %v, want %v", roles.ExpiresAt, end)
	}
}

func TestEveryAddressWorksWithAndWithoutTheHostAsItsFirstPart(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	for _, prefix := range []string{"", "/api-seller.ozon.ru"} {
		same(t, "an API call with the prefix "+prefix, r.call(prefix+pathSeller, nil).status, http.StatusOK)

		got := r.send(http.MethodGet, prefix+"/__control/health", nil, "")
		if got.status != http.StatusOK || got.body != `{"status":"ok"}` {
			t.Fatalf("health with the prefix %q answered %d %s", prefix, got.status, got.body)
		}

		setup := r.send(http.MethodPost, prefix+"/__control/setup", map[string]string{"Authorization": r.key}, `{"seller_name":"Name"}`)
		if setup.status != http.StatusOK {
			t.Fatalf("a control call with the prefix %q answered %d %s", prefix, setup.status, setup.body)
		}
	}
}

func TestTheCallListShowsTheApiKeyAsRedactedAndNeverTheKeyItself(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.call(pathSeller, nil)
	r.call(pathRoles, nil)

	raw := r.send(http.MethodGet, "/__control/calls", map[string]string{"Authorization": r.key}, "").body
	if strings.Contains(raw, r.key) {
		t.Fatalf("the call list shows the API key: %s", raw)
	}

	calls, err := r.ctl.Calls(t.Context())
	if err != nil {
		t.Fatalf("calls: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("got %d calls, want the 2 API calls", len(calls))
	}

	for _, call := range calls {
		if got := call.Header["Api-Key"]; len(got) != 1 || got[0] != "[redacted]" {
			t.Fatalf("the Api-Key header of %s is recorded as %v, want [redacted]", call.Path, got)
		}

		if got := call.Header["Client-Id"]; len(got) != 1 || got[0] != r.clientID {
			t.Fatalf("the Client-Id header of %s is recorded as %v, want the Client ID", call.Path, got)
		}
	}

	same(t, "the path of the first call, without the host", calls[0].Path, pathSeller)
}
