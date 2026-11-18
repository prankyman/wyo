package marketplace_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

// twoFields is a key form of a marketplace that names the account by a number and authorises with a secret.
var twoFields = []marketplace.KeyField{{Name: "client_id"}, {Name: "api_key", Secret: true}}

// oneField is a key form of one secret field.
var oneField = []marketplace.KeyField{{Name: "token", Secret: true}}

func TestAKeyOfSeveralFieldsIsPackedIntoOneTextAndReadBack(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		fields []marketplace.KeyField
		key    marketplace.Key
	}{
		{name: "two fields", fields: twoFields, key: marketplace.Key{"client_id": "123456", "api_key": "a1b2-c3d4"}},
		{name: "one field", fields: oneField, key: marketplace.Key{"token": "x.y.z"}},
		{name: "a value with quotes and a colon", fields: twoFields, key: marketplace.Key{"client_id": `1"2`, "api_key": "k:ey"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			text := marketplace.PackKey(tc.fields, tc.key)
			if got := marketplace.UnpackKey(tc.fields, text); !reflect.DeepEqual(got, tc.key) {
				t.Fatalf("UnpackKey(PackKey(%v)) = %v", tc.key, got)
			}
		})
	}
}

func TestAKeyOfOneFieldIsKeptAsItsValueAsBefore(t *testing.T) {
	t.Parallel()

	if text := marketplace.PackKey(oneField, marketplace.Key{"token": "x.y.z"}); text != "x.y.z" {
		t.Fatalf("PackKey of one field = %q, want the value itself, as every release before kept it", text)
	}
}

func TestOnlyTheFormsFieldsArePacked(t *testing.T) {
	t.Parallel()

	text := marketplace.PackKey(twoFields, marketplace.Key{"client_id": "1", "api_key": "secret", "other": "dropped"})
	if strings.Contains(text, "dropped") {
		t.Fatalf("a field the form does not have was packed: %q", text)
	}
}

func TestATextStoredBeforeKeysWerePackedIsTheFirstFieldsValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		fields []marketplace.KeyField
		text   string
		want   marketplace.Key
	}{
		{name: "a token stored by release 0.0.5", fields: oneField, text: "eyJhbGciOi.eyJzaWQiOiIx.c2ln", want: marketplace.Key{"token": "eyJhbGciOi.eyJzaWQiOiIx.c2ln"}}, //nolint:gosec // G101: a made-up token of a test
		{name: "a text that only looks packed", fields: oneField, text: "wyo.key.v1:not json", want: marketplace.Key{"token": "wyo.key.v1:not json"}},                     //nolint:gosec // G101: a made-up token of a test
		{name: "a form without fields", fields: nil, text: "plain", want: marketplace.Key{"": "plain"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := marketplace.UnpackKey(tc.fields, tc.text); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("UnpackKey(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestTheLastFourCharactersComeFromTheSecretFieldOnly(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		fields []marketplace.KeyField
		key    marketplace.Key
		want   string
	}{
		{name: "the secret of two fields", fields: twoFields, key: marketplace.Key{"client_id": "987654", "api_key": "abcdefgh"}, want: "efgh"},
		{name: "one field", fields: oneField, key: marketplace.Key{"token": "tokenWXYZ"}, want: "WXYZ"},
		{name: "a short secret", fields: oneField, key: marketplace.Key{"token": "ab"}, want: "ab"},
		{name: "letters of two bytes", fields: oneField, key: marketplace.Key{"token": "ключик"}, want: "ючик"}, //nolint:gosec // G101: a made-up token of a test
		{name: "no secret field", fields: []marketplace.KeyField{{Name: "id"}}, key: marketplace.Key{"id": "123456"}, want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := marketplace.LastFour(tc.fields, tc.key); got != tc.want {
				t.Fatalf("LastFour = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTheSecretsAndTheShownFieldsOfAKey(t *testing.T) {
	t.Parallel()

	key := marketplace.Key{"client_id": "987654", "api_key": "abcdefgh"}

	if got := key.Secrets(twoFields); !reflect.DeepEqual(got, []string{"abcdefgh"}) {
		t.Fatalf("Secrets = %v, want only the API key", got)
	}

	if got := key.Shown(twoFields); !reflect.DeepEqual(got, []marketplace.ShownField{{Name: "client_id", Value: "987654"}}) {
		t.Fatalf("Shown = %v, want only the Client ID", got)
	}

	if got := (marketplace.Key{"token": "t"}).Shown(oneField); len(got) != 0 {
		t.Fatalf("a key of one secret field shows %v", got)
	}

	if got := (marketplace.Key{"": "plain"}).Secrets(nil); !reflect.DeepEqual(got, []string{"plain"}) {
		t.Fatalf("a key of a form without fields hides %v, want its one value", got)
	}
}
