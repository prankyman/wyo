package sync_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite" // the driver, to plant a refusal into the database from outside the store

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// refuse makes the database refuse one kind of write, as a full disk or a broken file would.
func refuse(t *testing.T, r *memoryRig, table, operation string) {
	t.Helper()

	raw, err := sql.Open("sqlite", r.kit.Store.Path())
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	defer func() { _ = raw.Close() }()

	//nolint:gosec // G202: the table and the operation are the test's own constants, never an input
	statement := "CREATE TRIGGER refuse_" + table + " BEFORE " + operation + " ON " + table + " BEGIN SELECT RAISE(ABORT, 'refused'); END"
	if _, err := raw.ExecContext(t.Context(), statement); err != nil {
		t.Fatalf("plant the refusal: %v", err)
	}
}

func TestAWriteTheDatabaseRefusesEndsTheReadingAsAFailure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		table, operation string
		before           func(r *memoryRig)
		run              func(t *testing.T, r *memoryRig) error
	}{
		{name: "a new order", table: "orders", operation: "INSERT", before: func(r *memoryRig) { r.adapter.AddOrder(synckit.Noon()) }},
		{name: "a new order's event", table: "order_events", operation: "INSERT", before: func(r *memoryRig) { r.adapter.AddOrder(synckit.Noon()) }},
		{
			name: "a change of state", table: "orders", operation: "UPDATE",
			before: func(r *memoryRig) {
				r.adapter.SetStatus("1", marketplace.Status{State: marketplace.StateAssembling, Seller: "confirm", Marketplace: "waiting"})
			},
		},
		{
			name: "the feed's mark", table: "sync_marks", operation: "INSERT",
			before: func(r *memoryRig) {
				r.adapter.AddFeedRow(marketplace.FeedRow{JoinKey: "x", Number: "x", Scheme: marketplace.SchemeFBS})
			},
		},
		{
			name: "the warehouses", table: "marketplace_warehouses", operation: "INSERT",
			run: func(t *testing.T, r *memoryRig) error { return r.engine.RunWarehouses(t.Context()) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newMemoryRig(t, 10)
			r.adapter.AddOrder(synckit.Noon())

			if err := r.engine.RunOrders(t.Context()); err != nil {
				t.Fatalf("the first reading: %v", err)
			}

			refuse(t, r, tc.table, tc.operation)

			if tc.before != nil {
				tc.before(r)
			}

			run := tc.run
			if run == nil {
				run = func(t *testing.T, r *memoryRig) error { return r.engine.RunOrders(t.Context()) }
			}

			if err := run(t, r); err != nil {
				t.Fatalf("a job ends without an error of its own: %v", err)
			}

			if got, _ := r.kit.Store.Read().Account(t.Context(), r.account.ID); got.FailuresInRow != 1 {
				t.Fatalf("the refused write is not counted as a failed reading: %+v", got)
			}
		})
	}
}
