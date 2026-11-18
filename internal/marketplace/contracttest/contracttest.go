// Package contracttest is the one test suite of the adapter contract, written once and run for every adapter against its
// fake: an adapter that claims a part of the contract must pass that part's cases. It is test support: only tests import
// it (an architecture test).
package contracttest

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// Fixture is what an adapter's test gives the suite: the adapter, pointed at its fake, and the fake's control calls.
// Every function makes or acts on the state of one fresh seller at the fake, so cases never share one.
type Fixture struct {
	Adapter marketplace.Adapter
	// Now is the moment the fake's clock stands at.
	Now time.Time
	// Key gives the key form's fields of a working key of a fresh seller at the fake.
	Key func(t *testing.T) map[string]string
	// Conn gives the Conn of an account that holds the key (the test stores it where the engine's caller reads it).
	Conn func(t *testing.T, key map[string]string) marketplace.Conn
	// NewOrder makes a new order of the seller-warehouse scheme for the key's seller and returns its number as the
	// adapter gives it.
	NewOrder func(t *testing.T, key map[string]string) string
	// CancelOrder makes the buyer cancel that order.
	CancelOrder func(t *testing.T, key map[string]string, number string)
	// NewFeedOrder makes an order of the marketplace's own warehouse appear in the orders feed and returns its number.
	NewFeedOrder func(t *testing.T, key map[string]string) string

	// The functions below are for the parts that write (plan 4); an adapter that claims none of them needs none.

	// NewMarkedOrder makes a new order of the seller-warehouse scheme for which the marketplace requires a marking code,
	// and returns its number.
	NewMarkedOrder func(t *testing.T, key map[string]string) string
	// ChangeLabel makes the marketplace give that order another label, with another code.
	ChangeLabel func(t *testing.T, key map[string]string, number string)
	// SetVerdict makes the marketplace give the order's marking code a verdict of that kind.
	SetVerdict func(t *testing.T, key map[string]string, number string, kind marketplace.VerdictKind)
	// HandoverSettings are the account's hand-over settings that make the fake offer a shipping point.
	HandoverSettings map[string]string
}

// maxFeedReadings bounds the readings of a feed in one case: a feed that never ends is a defect.
const maxFeedReadings = 10

var (
	codePattern = regexp.MustCompile(`^[a-z]+$`)
	// namePattern is a name of a key field or a setting: lower-case words joined by underscores.
	namePattern = regexp.MustCompile(`^[a-z]+(_[a-z]+)*$`)
)

// RunAdapterContract runs the cases of every part the adapter claims.
func RunAdapterContract(t *testing.T, f Fixture) {
	t.Helper()

	desc := f.Adapter.Describe()

	t.Run("description", func(t *testing.T) { checkDescription(t, f.Adapter, desc) })

	cases := map[marketplace.Capability]func(t *testing.T, f Fixture){
		marketplace.KeyCheck:       keyCheck,
		marketplace.WarehousesRead: warehousesRead,
		marketplace.OrdersNew:      ordersNew,
		marketplace.OrdersStates:   ordersStates,
		marketplace.OrdersFeed:     ordersFeed,
	}

	for c, run := range writingCases() {
		cases[c] = run
	}

	for _, c := range desc.Capabilities {
		run, ok := cases[c]
		if !ok {
			t.Errorf("the adapter claims %s, which has no cases yet: the engine does not drive it", c)

			continue
		}

		t.Run(string(c), func(t *testing.T) { run(t, f) })
	}
}

// expect stops the case with the message when a promise of the contract is broken.
func expect(t *testing.T, kept bool, format string, args ...any) {
	t.Helper()

	if !kept {
		t.Fatalf(format, args...)
	}
}

func checkDescription(t *testing.T, a marketplace.Adapter, desc marketplace.Description) {
	t.Helper()

	expect(t, codePattern.MatchString(desc.Code) && desc.Mark != "" && len(desc.KeyFields) > 0,
		"code %q, mark %q, %d key fields: a code of lower-case letters, a mark and at least one field", desc.Code, desc.Mark, len(desc.KeyFields))

	for _, field := range desc.KeyFields {
		expect(t, namePattern.MatchString(field.Name), "the key field %q is not a name of lower-case words joined by underscores", field.Name)
	}

	names := map[string]bool{}

	for _, setting := range desc.HandoverSettings {
		expect(t, namePattern.MatchString(setting.Name) && !names[setting.Name], "the hand-over setting %q is not a name of lower-case words, or is there twice", setting.Name)
		expect(t, !slices.Contains(setting.Choices, ""), "the hand-over setting %q has an empty choice", setting.Name)

		names[setting.Name] = true
	}

	expect(t, !desc.Supports(marketplace.HandoverGroup) || desc.Supports(marketplace.OrdersConfirm),
		"the adapter claims handover.group and not orders.confirm: a hand-over's batch is made and filled by orders.confirm")

	writes := false

	for _, c := range desc.Capabilities {
		expect(t, marketplace.Supported(c), "the adapter claims %s, which the engine does not drive", c)
		expect(t, a.Capable(c, false), "the adapter claims %s but says a real key cannot do it", c)

		writes = writes || slices.Contains([]marketplace.Capability{
			marketplace.OrdersConfirm, marketplace.CodesReport, marketplace.HandoverGroup, marketplace.HandoverClose,
		}, c)
	}

	for _, route := range a.Routes() {
		for _, test := range []bool{false, true} {
			_, ok := a.Limit(route.Group, test)
			expect(t, ok, "%s %s counts in the group %q, which has no limit (test key: %v)", route.Method, route.Template, route.Group, test)
		}

		expect(t, strings.HasPrefix(route.Template, "/"), "the route %q does not begin with /", route.Template)
		expect(t, !route.Write || writes, "%s %s writes, and the adapter claims no part that writes", route.Method, route.Template)
	}
}

// implements says the adapter implements the interface of a part it claims.
func implements[T any](t *testing.T, a marketplace.Adapter, part marketplace.Capability) T {
	t.Helper()

	reader, ok := a.(T)
	expect(t, ok, "the adapter claims %s and does not implement its interface", part)

	return reader
}

func refusalCode(err error) (string, string) {
	var r *marketplace.Refusal
	if !errors.As(err, &r) {
		return "", ""
	}

	return r.Code, r.Field
}

func keyCheck(t *testing.T, f Fixture) {
	c := implements[marketplace.KeyChecker](t, f.Adapter, marketplace.KeyCheck)
	name := f.Adapter.Describe().KeyFields[0].Name
	field := "key." + name

	_, _, err := c.ReadKey(map[string]string{}, f.Now)
	code, at := refusalCode(err)
	expect(t, code == "required" && at == field, "an empty form: refusal %q on %q, want required on %q", code, at, field)

	_, _, err = c.ReadKey(map[string]string{name: "this is not a key"}, f.Now)
	code, at = refusalCode(err)
	expect(t, code == "key_unreadable" && at == field, "a text that is not a key: refusal %q on %q, want key_unreadable on %q", code, at, field)

	key := f.Key(t)

	facts, kept, err := c.ReadKey(key, f.Now)
	expect(t, err == nil && len(kept) > 0 && facts.ProfileID != "", "a working key: %+v, fields kept %v, %v", facts, len(kept) > 0, err)

	checked, err := c.CheckKey(t.Context(), f.Conn(t, key), facts)
	expect(t, err == nil && checked.ProfileID == facts.ProfileID, "CheckKey: %+v, %v", checked, err)
}

func warehousesRead(t *testing.T, f Fixture) {
	r := implements[marketplace.WarehouseReader](t, f.Adapter, marketplace.WarehousesRead)

	list, err := r.Warehouses(t.Context(), f.Conn(t, f.Key(t)))
	expect(t, err == nil && len(list) > 0 && list[0].Number != "" && list[0].Name != "", "Warehouses: %+v, %v", list, err)
}

func ordersNew(t *testing.T, f Fixture) {
	r := implements[marketplace.NewOrderReader](t, f.Adapter, marketplace.OrdersNew)
	key := f.Key(t)
	conn := f.Conn(t, key)
	number := f.NewOrder(t, key)

	for reading := range 2 {
		list, err := r.NewOrders(t.Context(), conn)
		expect(t, err == nil, "NewOrders: %v", err)

		var found []marketplace.Order
		for _, o := range list {
			if o.Number == number {
				found = append(found, o)
			}
		}

		expect(t, len(found) == 1 && found[0].Scheme == marketplace.SchemeFBS && found[0].Status.State == marketplace.StateNew &&
			!found[0].CreatedAt.IsZero() && everyLineIsAProduct(found[0].Lines),
			"reading %d: the new order %s once, new, of the seller-warehouse scheme, with its products; got %+v", reading+1, number, found)
	}
}

// everyLineIsAProduct says an order has at least one line and every line names its product and at least one unit.
func everyLineIsAProduct(lines []marketplace.Line) bool {
	for _, l := range lines {
		if l.Quantity < 1 || (l.ProductID == "" && l.Article == "") {
			return false
		}
	}

	return len(lines) > 0
}

func ordersStates(t *testing.T, f Fixture) {
	r := implements[marketplace.StateReader](t, f.Adapter, marketplace.OrdersStates)
	expect(t, r.MaxStates() > 0, "the adapter answers %d orders a call", r.MaxStates())

	key := f.Key(t)
	conn := f.Conn(t, key)
	number := f.NewOrder(t, key)

	list, err := r.States(t.Context(), conn, []string{number, "999999999999"})
	expect(t, err == nil && len(list) == 1 && list[0].Number == number && list[0].Status.State == marketplace.StateNew,
		"the state of a new order (and none of an unknown one): %+v, %v", list, err)

	f.CancelOrder(t, key, number)

	list, err = r.States(t.Context(), conn, []string{number})
	expect(t, err == nil && len(list) == 1 && list[0].Status.State == marketplace.StateCancelled &&
		list[0].Status.CancelledBy == marketplace.CancelledByBuyer, "the state of an order the buyer cancelled: %+v, %v", list, err)
}

func ordersFeed(t *testing.T, f Fixture) {
	r := implements[marketplace.FeedReader](t, f.Adapter, marketplace.OrdersFeed)
	key := f.Key(t)
	conn := f.Conn(t, key)
	number := f.NewFeedOrder(t, key)

	mark, seen := "", false

	for range maxFeedReadings {
		page, err := r.Feed(t.Context(), conn, mark, f.Now.Add(-time.Hour))
		expect(t, err == nil, "Feed: %v", err)

		for _, row := range page.Rows {
			seen = seen || (row.Number == number && row.Scheme == marketplace.SchemeFBO)
		}

		if page.Done {
			expect(t, page.Mark == mark, "the end of the feed keeps the mark (it may bring again the rows of the mark's own moment): %+v", page)
			expect(t, seen, "the feed ended without the order %s", number)

			return
		}

		expect(t, page.Mark != "" && page.Mark != mark, "a page with rows moves the mark: from %q to %q", mark, page.Mark)
		mark = page.Mark
	}

	t.Fatalf("the feed did not end after %d readings", maxFeedReadings)
}
