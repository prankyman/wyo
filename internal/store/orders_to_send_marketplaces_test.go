package store_test

import "testing"

// The exchange gives the list of orders to send the codes of every marketplace whose adapter hands orders over, and since
// Ozon joined Wildberries that is two of them. With two codes the list's limit once met a text where it wanted a number
// (SQLite: «datatype mismatch»), so no order of either marketplace was sent. The limit is the number of orders one run
// sends, and only that many come, whatever the number of marketplaces named.
func TestTheOrdersToSendAreListedWhenTwoMarketplacesAreNamedAndOnlyAsManyAsTheLimit(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()

	for _, number := range []string{"100", "101", "102"} {
		f.order(t, number, nil)
	}

	for _, codes := range [][]string{{"ozon", "wildberries"}, {"wildberries", "ozon", "another"}} {
		all, err := f.s.Read().OrdersToSend(ctx, 10, codes)
		if err != nil || len(all) != 3 {
			t.Fatalf("OrdersToSend of %v: %d orders, %v; want the 3 of the account that is matched", codes, len(all), err)
		}

		limited, err := f.s.Read().OrdersToSend(ctx, 2, codes)
		if err != nil || len(limited) != 2 || limited[0].Number != "100" || limited[1].Number != "101" {
			t.Fatalf("OrdersToSend of %v with the limit 2: %+v, %v; want the first two", codes, limited, err)
		}
	}
}
