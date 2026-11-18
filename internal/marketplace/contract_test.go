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
		marketplace.OrdersFeed: true,
	}

	if !reflect.DeepEqual(supported, want) {
		t.Fatalf("supported parts %v, want %v", supported, want)
	}

	for _, c := range []marketplace.Capability{
		marketplace.OrdersConfirm, marketplace.LabelsFetch, marketplace.CodesReport, marketplace.CodesVerdict,
		marketplace.HandoverGroup, marketplace.HandoverClose, marketplace.HandoverDocuments, marketplace.StockPush,
		marketplace.StockRead, marketplace.SalesRead, marketplace.MoneyRead, marketplace.ProductsRead,
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
