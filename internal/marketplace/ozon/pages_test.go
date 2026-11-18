package ozon_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestEveryReadingPassesOnOzonsRefusal(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	deactivated := `{"code":7,"message":"Api-key is deactivated, use another one or generate a new one","details":[]}`

	cases := []struct {
		name string
		path string
		read func(t *testing.T, key marketplace.Key) error
	}{
		{name: "the roles", path: "/v1/roles", read: func(t *testing.T, key marketplace.Key) error {
			facts, _, _ := r.adapter.ReadKey(key, synckit.Noon())
			_, err := r.adapter.CheckKey(t.Context(), r.conn(t, key), facts)

			return err
		}},
		{name: "the seller", path: "/v1/seller/info", read: func(t *testing.T, key marketplace.Key) error {
			facts, _, _ := r.adapter.ReadKey(key, synckit.Noon())
			_, err := r.adapter.CheckKey(t.Context(), r.conn(t, key), facts)

			return err
		}},
		{name: "the warehouses", path: "/v2/warehouse/list", read: func(t *testing.T, key marketplace.Key) error {
			_, err := r.adapter.Warehouses(t.Context(), r.conn(t, key))

			return err
		}},
		{name: "the new postings", path: "/v4/posting/fbs/unfulfilled/list", read: func(t *testing.T, key marketplace.Key) error {
			_, err := r.adapter.NewOrders(t.Context(), r.conn(t, key))

			return err
		}},
		{name: "one posting", path: "/v3/posting/fbs/get", read: func(t *testing.T, key marketplace.Key) error {
			_, err := r.adapter.States(t.Context(), r.conn(t, key), []string{"77777777-0001-1"})

			return err
		}},
		{name: "the FBO postings", path: "/v3/posting/fbo/list", read: func(t *testing.T, key marketplace.Key) error {
			_, err := r.adapter.Feed(t.Context(), r.conn(t, key), "", synckit.Noon())

			return err
		}},
		{name: "the products", path: "/v3/product/info/list", read: func(t *testing.T, key marketplace.Key) error {
			_, err := r.adapter.NewOrders(t.Context(), r.conn(t, key))

			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key := r.key(t)
			r.posting(t, key, "77777777-0001-1", warehouseFBS, product(2001, "SHIRT", 1))

			if err := fakekit.NewControl(r.kit.URL, key["api_key"], r.kit.HTTP).QueueFault(t.Context(), fakekit.Fault{
				Kind: fakekit.FaultRefuse, PathPrefix: tc.path, Status: http.StatusForbidden, Body: deactivated,
			}); err != nil {
				t.Fatalf("QueueFault: %v", err)
			}

			var refusal *marketplace.Refusal
			if err := tc.read(t, key); !errors.As(err, &refusal) || refusal.Code != "key_deactivated" || !refusal.StopsJobs {
				t.Fatalf("a refusal of %s: %v; want the key deactivated", tc.path, err)
			}
		})
	}
}

func TestAProductsLevelTakenFromTheKeyLeavesTheLinesWithoutBarcodes(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	r.posting(t, key, "88888888-0001-1", warehouseFBS, product(2001, "SHIRT", 1))

	if err := fakekit.NewControl(r.kit.URL, key["api_key"], r.kit.HTTP).QueueFault(t.Context(), fakekit.Fault{
		Kind: fakekit.FaultRefuse, PathPrefix: "/v3/product/info/list", Status: http.StatusForbidden,
		Body: `{"code":7,"message":"Api-Key is missing a required role for a method","details":[]}`,
	}); err != nil {
		t.Fatalf("QueueFault: %v", err)
	}

	list, err := r.adapter.NewOrders(t.Context(), r.conn(t, key))
	if err != nil || len(list) != 1 || list[0].Lines[0].Barcodes != nil {
		t.Fatalf("NewOrders = %+v, %v; want the order without barcodes", list, err)
	}
}

func TestMoreThanAPageOfPostingsIsReadByItsCursor(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	for i := range 101 {
		r.posting(t, key, fmt.Sprintf("99999999-%04d-1", i), warehouseFBS, product(2001, "SHIRT", 1))
		r.control(t, key, "/__control/fbo-postings", map[string]any{
			"posting_number": fmt.Sprintf("99999998-%04d-1", i), "order_number": "99999998", "status": "delivering",
			"created_at": synckit.Noon(), "products": []map[string]any{product(2002, "SOCKS", 1)},
		})
	}

	list, err := r.adapter.NewOrders(t.Context(), conn)
	if err != nil || len(list) != 101 || r.callsTo(t, key, "/v4/posting/fbs/unfulfilled/list") != 2 {
		t.Fatalf("NewOrders: %d orders, %v; want 101 read in two pages", len(list), err)
	}

	page, err := r.adapter.Feed(t.Context(), conn, "", synckit.Noon())
	if err != nil || len(page.Rows) != 101 || r.callsTo(t, key, "/v3/posting/fbo/list") != 2 {
		t.Fatalf("Feed: %d rows, %v; want 101 read in two pages", len(page.Rows), err)
	}
}
