package store_test

import (
	"database/sql"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

// TestTheLabelOfEveryOrderOfTheFourthMigrationBecomesItsFirstParcel writes one Wildberries order on each exchange state into
// the database as migration 0004 leaves it (testdata/0004.db), with the label columns the exchange kept in the order's own row
// then, upgrades it, and reads each order's parcel back: parcel 1, the order's own number, the label's fingerprint and the
// moment it was sent as they were. The columns the label lived in are gone from the order's row.
func TestTheLabelOfEveryOrderOfTheFourthMigrationBecomesItsFirstParcel(t *testing.T) {
	t.Parallel()

	path := copyOfSnapshot(t, "0004.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open the copy: %v", err)
	}

	states := []string{"not_sent", "sent", "held", "reserved", "label_sent", "packed", "codes_checking", "codes_accepted", "handed_over", "cancelled"}
	// The states in which an order carries a label; the others never had one.
	labelled := []string{"label_sent", "packed", "codes_checking", "codes_accepted", "handed_over"}
	base := time.Date(2026, 11, 17, 12, 0, 0, 0, moscow).Unix()

	for i, state := range states {
		number := fmt.Sprintf("%d", 5000+i)
		fingerprint, sentAt := "", "NULL"

		if slices.Contains(labelled, state) {
			fingerprint, sentAt = "print-"+number, fmt.Sprintf("%d", base+int64(i))
		}

		for _, insert := range []string{
			fmt.Sprintf(`INSERT INTO orders (account_id, number, scheme, join_key, warehouse_number, created_at, state, updated_at)
				SELECT id, '%s', 'fbs', 'rid.%s', '1001', %d, 'new', %d FROM marketplace_accounts LIMIT 1`, number, number, base, base),
			fmt.Sprintf(`INSERT INTO exchange_orders (order_id, state, label_print, label_sent_at, sent_at, updated_at)
				SELECT id, '%s', '%s', %s, %d, %d FROM orders WHERE number = '%s'`, state, fingerprint, sentAt, base, base, number),
		} {
			if _, err := raw.ExecContext(t.Context(), insert); err != nil {
				t.Fatalf("an order in state %s as migration 0004 kept it: %v", state, err)
			}
		}
	}

	// A unit's code as migration 0004 kept it, with no line.
	if _, err := raw.ExecContext(t.Context(), `INSERT INTO order_codes (order_id, unit, identifier, code)
		SELECT id, 1, 'marking', 'code' FROM orders WHERE number = '5004'`); err != nil {
		t.Fatalf("a code as migration 0004 kept it: %v", err)
	}

	if err := raw.Close(); err != nil {
		t.Fatalf("close the copy: %v", err)
	}

	s := storekit.OpenAt(t, path)
	accounts, err := s.Read().Accounts(t.Context())

	if err != nil || len(accounts) != 1 {
		t.Fatalf("Accounts: %+v, %v", accounts, err)
	}

	for i, state := range states {
		number := fmt.Sprintf("%d", 5000+i)

		o, err := s.Read().OrderByNumber(t.Context(), accounts[0].ID, number)
		if err != nil {
			t.Fatalf("order %s (%s): %v", number, state, err)
		}

		x, err := s.Read().ExchangeOrder(t.Context(), o.ID)
		if err != nil || x.State != state || !x.PackReportedAt.IsZero() || x.PackTries != 0 || !x.PackNextAt.IsZero() ||
			x.CodesResends != 0 || !x.PackedAt.IsZero() {
			t.Fatalf("the exchange of order %s after the upgrade: %+v, %v; want state %s and nothing of the packing reported", number, x, err, state)
		}

		want := store.Parcel{OrderID: o.ID, Number: 1, MpNumber: number}
		if slices.Contains(labelled, state) {
			want.LabelPrint, want.LabelSentAt = "print-"+number, time.Unix(base+int64(i), 0).UTC()
		}

		parcels, err := s.Read().ParcelsOf(t.Context(), o.ID)
		if err != nil || !reflect.DeepEqual(parcels, []store.Parcel{want}) {
			t.Errorf("the parcels of order %s (%s) after the upgrade: %+v, %v; want %+v", number, state, parcels, err, want)
		}

		if due, _ := s.Read().DueLabelChecks(t.Context(), time.Unix(base+86400, 0), 100); len(due) != 0 {
			t.Fatalf("a label of an order that existed before the upgrade is asked for again: %+v", due)
		}
	}

	// The code of an order that existed has no line yet; the warehouses of the account have no label type and no automatic assembly.
	o, _ := s.Read().OrderByNumber(t.Context(), accounts[0].ID, "5004")

	if codes, err := s.Read().OrderCodes(t.Context(), o.ID); err != nil || len(codes) != 1 || codes[0].Line != 0 || codes[0].Code != "code" {
		t.Fatalf("the code of an order after the upgrade: %+v, %v", codes, err)
	}

	if warehouses, err := s.Read().Warehouses(t.Context(), accounts[0].ID); err != nil || len(warehouses) != 1 ||
		warehouses[0].LabelType != "" || warehouses[0].AutoAssembly {
		t.Fatalf("the warehouses after the upgrade: %+v, %v", warehouses, err)
	}

	assertNoColumns(t, path, "exchange_orders", "label_print", "label_sent_at")
}

// assertNoColumns fails when a table of the database file at path still has one of the columns.
func assertNoColumns(t *testing.T, path, table string, columns ...string) {
	t.Helper()

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}

	defer func() { _ = raw.Close() }()

	rows, err := raw.QueryContext(t.Context(), "SELECT name FROM pragma_table_info(?)", table)
	if err != nil {
		t.Fatalf("the columns of %s: %v", table, err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("read a column name: %v", err)
		}

		if slices.Contains(columns, name) {
			t.Errorf("table %s still has the column %s", table, name)
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("read the columns of %s: %v", table, err)
	}
}
