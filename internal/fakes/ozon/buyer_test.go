package ozon_test

import (
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/fakes/ozon"
)

func TestEveryPostingAnswerCarriesTheMadeUpDataOfTheBuyer(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newPosting("7001-1", 24, nil)

	fromList := r.call(pathUnfulfilled, map[string]any{"filter": r.everything()})
	fromGet := r.call(pathGet, map[string]any{"posting_number": "7001-1"})

	for name, answer := range map[string]reply{"the unfulfilled list": fromList, "the single posting": fromGet} {
		for _, value := range []string{
			ozon.BuyerName, ozon.BuyerPhone, ozon.BuyerAddress, ozon.BuyerCity, ozon.BuyerZipCode, ozon.BuyerComment,
		} {
			if !strings.Contains(answer.body, value) {
				t.Errorf("%s does not carry the buyer's %q: %s", name, value, answer.body)
			}
		}
	}

	// Ozon's file gives the customer of the single posting no e-mail, so only the list carries it.
	if !strings.Contains(fromList.body, ozon.BuyerEmail) || strings.Contains(fromGet.body, ozon.BuyerEmail) {
		t.Fatalf("the e-mail: in the list %v, in the single posting %v; want only in the list", strings.Contains(fromList.body, ozon.BuyerEmail), strings.Contains(fromGet.body, ozon.BuyerEmail))
	}

	var list listAnswer

	decode(t, fromList.body, &list)

	buyer := list.Postings[0]
	if buyer.Customer.Name != ozon.BuyerName || buyer.Customer.Address.AddressTail != ozon.BuyerAddress || buyer.Addressee.Name != ozon.BuyerName {
		t.Fatalf("the buyer of the list: %+v", buyer.Customer)
	}
}

func TestTheBuyerValuesAreExportedForTestsElsewhereToSearchFor(t *testing.T) {
	t.Parallel()

	want := []string{
		ozon.BuyerName, ozon.BuyerPhone, ozon.BuyerEmail, ozon.BuyerAddress, ozon.BuyerCity, ozon.BuyerZipCode, ozon.BuyerComment,
	}

	got := ozon.BuyerValues()

	sameStrings(t, "the buyer values", got, want)

	for _, value := range got {
		if value == "" {
			t.Fatal("a buyer value is empty: a search for it would find everything")
		}
	}
}
