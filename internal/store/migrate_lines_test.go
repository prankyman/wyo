package store_test

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

// TestEveryOrderOfRelease005BecomesOneLineWithTheSameProduct writes two Wildberries orders into the database of release
// 0.0.5 (testdata/0003.db) as that release kept them, with the product in the order's own columns, upgrades it, and
// reads each order back as one line of quantity 1 with the same product.
func TestEveryOrderOfRelease005BecomesOneLineWithTheSameProduct(t *testing.T) {
	t.Parallel()

	path := copyOfSnapshot(t, "0003.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open the copy: %v", err)
	}

	for _, insert := range []string{
		`INSERT INTO orders (account_id, number, scheme, join_key, basket, warehouse_number, article, barcodes, product_id, kind, brand,
		 size, created_at, state, updated_at) SELECT id, '4001', 'fbs', 'rid.4001', 'g-7', '1001', 'ART-7', '4600007 4610007', '777',
		 'Рубашки', 'Бренд', 'M', 1763380800, 'new', 1763380800 FROM marketplace_accounts LIMIT 1`,
		`INSERT INTO orders (account_id, number, scheme, join_key, warehouse_name, article, barcodes, product_id, created_at, state,
		 updated_at) SELECT id, 'srid-9', 'fbo', 'srid-9', 'Коледино', 'ART-9', '4600009', '999', 1763380800, 'in_delivery', 1763380800
		 FROM marketplace_accounts LIMIT 1`,
	} {
		if _, err := raw.ExecContext(t.Context(), insert); err != nil {
			t.Fatalf("an order as release 0.0.5 kept it: %v", err)
		}
	}

	if err := raw.Close(); err != nil {
		t.Fatalf("close the copy: %v", err)
	}

	s := storekit.OpenAt(t, path)
	accounts, err := s.Read().Accounts(t.Context())
	if err != nil || len(accounts) == 0 {
		t.Fatalf("Accounts: %v, %v", accounts, err)
	}

	cases := []struct {
		number string
		want   store.OrderLine
	}{
		{number: "4001", want: store.OrderLine{
			ProductID: "777", Article: "ART-7", Barcodes: []string{"4600007", "4610007"}, Kind: "Рубашки", Brand: "Бренд", Size: "M", Quantity: 1,
		}},
		{number: "srid-9", want: store.OrderLine{ProductID: "999", Article: "ART-9", Barcodes: []string{"4600009"}, Quantity: 1}},
		{number: "1", want: store.OrderLine{Quantity: 1}},
	}

	for _, tc := range cases {
		got, err := s.Read().OrderByNumber(t.Context(), accounts[0].ID, tc.number)
		if err != nil || !reflect.DeepEqual(got.Lines, []store.OrderLine{tc.want}) || !got.ShipBy.IsZero() || got.Parent != "" {
			t.Errorf("order %s after the upgrade: %+v, %v; want one line %+v and no deadline of the marketplace", tc.number, got, err, tc.want)
		}
	}
}
