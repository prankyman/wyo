package wildberries_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	fakewb "github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func tokenOf(change func(*fakewb.TokenClaims)) string {
	claims := fakewb.TokenClaims{
		ID: "t", Seller: fakeSeller, Type: fakewb.TokenPersonal, Expires: synckit.Noon().Add(180 * 24 * time.Hour),
		Categories: fakewb.CategoryMarketplace | fakewb.CategoryStatistics,
	}

	if change != nil {
		change(&claims)
	}

	return fakewb.MakeToken(claims)
}

func TestATokensOwnFieldsAreReadWithoutCallingAnybody(t *testing.T) {
	t.Parallel()

	in180Days := synckit.Noon().Add(180 * 24 * time.Hour).UTC().Truncate(time.Second)

	cases := []struct {
		name       string
		token      string
		want       marketplace.KeyFacts
		wantCode   string
		wantParams map[string]any
	}{
		{
			name: "a Personal token", token: tokenOf(nil),
			want: marketplace.KeyFacts{Type: "personal", ExpiresAt: in180Days, ProfileID: fakeSeller, Categories: []string{"marketplace", "statistics"}},
		},
		{
			name: "a test token, whatever its categories", token: tokenOf(func(c *fakewb.TokenClaims) { c.Type, c.Categories = fakewb.TokenTest, 0 }),
			want: marketplace.KeyFacts{Type: "test", Test: true, ExpiresAt: in180Days, ProfileID: fakeSeller},
		},
		{
			name: "only reading", token: tokenOf(func(c *fakewb.TokenClaims) { c.ReadOnly = true }),
			want: marketplace.KeyFacts{Type: "personal", ExpiresAt: in180Days, ProfileID: fakeSeller, Categories: []string{"marketplace", "statistics"}, ReadOnly: true},
		},
		{
			name: "without the statistics", token: tokenOf(func(c *fakewb.TokenClaims) { c.Categories = fakewb.CategoryMarketplace }),
			want: marketplace.KeyFacts{Type: "personal", ExpiresAt: in180Days, ProfileID: fakeSeller, Categories: []string{"marketplace"}, MissingCategories: []string{"statistics"}},
		},
		{
			name: "without an end", token: tokenOf(func(c *fakewb.TokenClaims) { c.Expires = time.Time{} }),
			want: marketplace.KeyFacts{Type: "personal", ProfileID: fakeSeller, Categories: []string{"marketplace", "statistics"}},
		},
		{name: "a basic token", token: tokenOf(func(c *fakewb.TokenClaims) { c.Type = fakewb.TokenBase }), wantCode: "key_type_not_accepted", wantParams: map[string]any{"type": "base"}},
		{name: "a service token", token: tokenOf(func(c *fakewb.TokenClaims) { c.Type = fakewb.TokenService }), wantCode: "key_type_not_accepted", wantParams: map[string]any{"type": "service"}},
		{name: "an ended token", token: tokenOf(func(c *fakewb.TokenClaims) { c.Expires = synckit.Noon() }), wantCode: "key_expired"},
		{name: "without the marketplace", token: tokenOf(func(c *fakewb.TokenClaims) { c.Categories = fakewb.CategoryStatistics }), wantCode: "category_missing", wantParams: map[string]any{"category": "marketplace"}},
		{name: "a type that does not exist", token: tokenOf(func(c *fakewb.TokenClaims) { c.Type = 9 }), wantCode: "key_unreadable"},
		{name: "without a seller", token: tokenOf(func(c *fakewb.TokenClaims) { c.Seller = "" }), wantCode: "key_unreadable"},
		{name: "a text that is not a token", token: "my-token-from-the-cabinet", wantCode: "key_unreadable"}, //nolint:gosec // G101: a made-up text, no key
		{name: "a middle part that is not JSON", token: "a.bm90IGpzb24.c", wantCode: "key_unreadable"},       //nolint:gosec // G101: a made-up text, no key
		{name: "spaces only", token: "   ", wantCode: "required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			facts, secret, err := wildberries.New(wildberries.Options{}).ReadKey(map[string]string{"token": " " + tc.token + "\n"}, synckit.Noon())

			if tc.wantCode == "" {
				if err != nil || secret != tc.token || !reflect.DeepEqual(facts, tc.want) {
					t.Fatalf("ReadKey = %+v, secret kept %v, %v\nwant %+v", facts, secret == tc.token, err, tc.want)
				}

				return
			}

			var r *marketplace.Refusal
			if !errors.As(err, &r) || r.Code != tc.wantCode || r.Field != "key.token" || !reflect.DeepEqual(r.Params, tc.wantParams) || secret != "" {
				t.Fatalf("ReadKey refused with %+v, secret %q; want %s %v on key.token", r, secret, tc.wantCode, tc.wantParams)
			}
		})
	}
}

// FuzzReadKey feeds the token reader anything a person may paste; it must answer facts or a refusal, never fail.
func FuzzReadKey(f *testing.F) {
	f.Add(tokenOf(nil))
	f.Add("a.b.c")
	f.Add("..")
	f.Add("eyJ.eyJzaWQiOjF9.x")

	f.Fuzz(func(t *testing.T, token string) {
		facts, secret, err := wildberries.New(wildberries.Options{}).ReadKey(map[string]string{"token": token}, synckit.Noon())

		var r *marketplace.Refusal
		if err != nil && !errors.As(err, &r) {
			t.Fatalf("an error that is not a refusal: %v", err)
		}

		if err == nil && (secret == "" || facts.ProfileID == "") {
			t.Fatalf("facts without a secret or a profile: %+v", facts)
		}
	})
}
