package store_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// countsAllowed are the queries that count orders and may count a test order, each with its reason: the places where the
// owner acts on an order, where a test order is shown marked because trying the whole walk is what it is for, and the
// counts that are no total of the orders at all.
var countsAllowed = map[string]string{
	"CountOrdersOfAccount":   "says only whether an account has orders, so that it can be switched off and not deleted",
	"CountOrderViews":        "the order list's own count of the orders its filter shows, test ones marked among them",
	"CountTestOrders":        "counts the test orders themselves, so that the home page says how many its numbers leave out",
	"CountExchangeAttention": "the list «Требуют внимания»: a test order that needs the owner is shown there, marked",
	"CountNotReservedInTime": "the list «Требуют внимания»: a test order not reserved in time needs the owner as a real one does",
}

var (
	queryName = regexp.MustCompile(`(?m)^-- name: (\w+)`)
	// readsOrders finds a query that reads the orders or the exchange's rows of orders.
	readsOrders = regexp.MustCompile(`(?i)\b(FROM|JOIN)\s+(exchange_)?orders\b`)
	testFilter  = regexp.MustCompile(`\btest\s*=\s*0\b`)
)

// TestATestOrderIsCountedInNoTotal is the policy test of principle X, «a test order is counted in no total»: every query
// of internal/store/queries/ whose name begins with Count or Sum and that reads orders keeps only the orders whose test
// mark is 0, or is on the allow-list above with its reason. An entry of the list that names no such query fails, so the
// list can only shrink.
func TestATestOrderIsCountedInNoTotal(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no query file: %v", err)
	}

	var counting []string

	for _, file := range files {
		raw, err := os.ReadFile(file) //nolint:gosec // G304: a file of queries/
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}

		text := string(raw)
		marks := queryName.FindAllStringSubmatchIndex(text, -1)

		for i, m := range marks {
			name := text[m[2]:m[3]]
			end := len(text)

			if i+1 < len(marks) {
				end = marks[i+1][0]
			}

			body := text[m[1]:end]
			if (!strings.HasPrefix(name, "Count") && !strings.HasPrefix(name, "Sum")) || !readsOrders.MatchString(body) {
				continue
			}

			counting = append(counting, name)

			if _, allowed := countsAllowed[name]; !allowed && !testFilter.MatchString(body) {
				t.Errorf("%s (%s) counts orders and does not leave the test orders out (test = 0); a test order is counted in no "+
					"total (constitution, principle X)", name, filepath.Base(file))
			}
		}
	}

	for name := range countsAllowed {
		if !slices.Contains(counting, name) {
			t.Errorf("the allow-list names %s, which is no query that counts orders: remove it", name)
		}
	}
}
