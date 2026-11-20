package marketplace_test

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

func TestADescriptionSaysItsMessageKeysAndWhatItSupports(t *testing.T) {
	t.Parallel()

	d := stub{code: "alpha"}.Describe()

	if d.MessageKey() != "marketplace.alpha" || !d.Supports(marketplace.OrdersNew) || d.Supports(marketplace.OrdersFeed) {
		t.Fatalf("MessageKey %q, supports new %v, feed %v", d.MessageKey(), d.Supports(marketplace.OrdersNew), d.Supports(marketplace.OrdersFeed))
	}
}

func TestEveryPartOfTheContractIsNamedAndOnlyThisStagesPartsAreSupported(t *testing.T) {
	t.Parallel()

	all := marketplace.AllCapabilities()

	supported := map[marketplace.Capability]bool{}
	for _, c := range all {
		if marketplace.Supported(c) {
			supported[c] = true
		}
	}

	want := map[marketplace.Capability]bool{
		marketplace.KeyCheck: true, marketplace.WarehousesRead: true, marketplace.OrdersNew: true, marketplace.OrdersStates: true,
		marketplace.OrdersFeed: true, marketplace.OrdersConfirm: true, marketplace.OrdersPack: true, marketplace.LabelsFetch: true, marketplace.CodesReport: true,
		marketplace.CodesVerdict: true, marketplace.HandoverGroup: true, marketplace.HandoverClose: true,
		marketplace.HandoverDocuments: true, marketplace.OrdersChanges: true, marketplace.OrdersNamed: true,
		marketplace.NotificationsReceive: true, marketplace.OrdersReady: true, marketplace.OrdersCourierCode: true,
	}

	if !reflect.DeepEqual(supported, want) {
		t.Fatalf("supported parts %v, want %v", supported, want)
	}

	for _, c := range []marketplace.Capability{
		marketplace.StockPush, marketplace.StockRead, marketplace.SalesRead, marketplace.MoneyRead, marketplace.ProductsRead,
		marketplace.SuppliesRequest, marketplace.SuppliesContents,
	} {
		found := false
		for _, named := range all {
			found = found || named == c
		}

		if !found || marketplace.Supported(c) {
			t.Fatalf("%s: named %v, supported %v; want named and not supported", c, found, marketplace.Supported(c))
		}
	}
}

func TestACallErrorAndARefusalSayWhatHappenedWithoutAKey(t *testing.T) {
	t.Parallel()

	var err error = &marketplace.CallError{Outcome: "limited", Status: http.StatusTooManyRequests}

	var callErr *marketplace.CallError
	if !errors.As(err, &callErr) || callErr.Error() != "the marketplace call ended limited (status 429)" {
		t.Fatalf("CallError: %v", err)
	}

	err = &marketplace.Refusal{Code: "key_refused", Status: http.StatusUnauthorized}

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Error() != "the marketplace refused: key_refused (status 401)" {
		t.Fatalf("Refusal: %v", err)
	}
}

func TestACallErrorOrARefusalWithoutAnAnswerSaysSo(t *testing.T) {
	t.Parallel()

	if got := (&marketplace.CallError{Outcome: "no_connection"}).Error(); got != "the marketplace call ended no_connection" {
		t.Fatalf("CallError = %q", got)
	}

	if got := (&marketplace.Refusal{Code: "key_expired"}).Error(); got != "the marketplace refused: key_expired" {
		t.Fatalf("Refusal = %q", got)
	}
}

func TestASchemeOfAMarketplacesOwnNameIsReadAsTheSchemeOfWhoPacks(t *testing.T) {
	t.Parallel()

	cases := map[marketplace.Scheme]marketplace.Scheme{
		marketplace.SchemeFBS: marketplace.SchemeFBS, marketplace.SchemeFBO: marketplace.SchemeFBO,
		marketplace.SchemeExpress: marketplace.SchemeFBS, marketplace.SchemeFBY: marketplace.SchemeFBO,
	}

	for scheme, want := range cases {
		if got := scheme.Base(); got != want {
			t.Errorf("%s.Base() = %s, want %s", scheme, got, want)
		}
	}
}

func TestALimitIsMetByItsOwnStatusOr429(t *testing.T) {
	t.Parallel()

	own := marketplace.LimitGroup{LimitStatus: 420}
	plain := marketplace.LimitGroup{}

	if !own.MetBy(420) || own.MetBy(http.StatusTooManyRequests) || !plain.MetBy(http.StatusTooManyRequests) || plain.MetBy(420) {
		t.Fatalf("own: 420 %v, 429 %v; plain: 429 %v, 420 %v", own.MetBy(420), own.MetBy(429), plain.MetBy(429), plain.MetBy(420))
	}
}
