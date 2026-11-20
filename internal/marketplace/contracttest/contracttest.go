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

	// The fields below are optional; a case that needs one the fixture lacks is left out, and says so in its test's name.

	// LabelType is the kind of label asked of a marketplace that makes labels later (marketplace.Warehouse.LabelType).
	LabelType string
	// NewSplittableOrder makes a new order of at least two units, which the packing report can split into parcels.
	NewSplittableOrder func(t *testing.T, key map[string]string) string
	// SplitOrder makes the marketplace split that order (made by NewSplittableOrder) into parts of its own, as the seller's
	// cabinet does: the order is then no longer an order of its own, and the marketplace shows it as split, not as cancelled.
	SplitOrder func(t *testing.T, key map[string]string, number string)
	// ExtraVerdicts are the verdicts beyond the four every adapter can be told that the fake can be told (VerdictAgain).
	ExtraVerdicts []marketplace.VerdictKind
	// NoVerdicts are the verdicts of the four the marketplace has no word for, so that no fake can be told to give them (a
	// marketplace that never stops waiting for its check has no «unfinished»).
	NoVerdicts []marketplace.VerdictKind
	// ClosingKeepsOrders says closing a batch does not move its orders to delivery: the marketplace's own carrier does that
	// later, when it takes them (a shipment that is approved first and taken after).
	ClosingKeepsOrders bool
	// NeedsOtherIdentifier makes an order for which the marketplace needs an identifier other than a marking code, and
	// returns its number and the identifier it needs.
	NeedsOtherIdentifier func(t *testing.T, key map[string]string) (number, identifier string)
	// BatchNotReady makes the marketplace unable to make a batch yet (it answers with a *marketplace.BatchWait).
	BatchNotReady func(t *testing.T, key map[string]string)

	// The fields below are for the parts of Plan 7.

	// ProfileFromCheck says the key does not name the seller's profile itself (an opaque key): ReadKey leaves the profile
	// empty and CheckKey finds it by asking the marketplace.
	ProfileFromCheck bool
	// NewTestOrder makes a new test order (one the seller makes in the cabinet to try the connection) and returns its number.
	NewTestOrder func(t *testing.T, key map[string]string) string
	// OrderNotification gives the body of the marketplace's notification that the order of the number appeared, and
	// PingNotification the body of its check that the address answers.
	OrderNotification func(number string) []byte
	PingNotification  func() []byte

	// The fields below are for the parts of Plan 8; an adapter that claims none of them needs none.

	// LabelSizes are the label sizes the marketplace makes (the cases ask a label in each, and in none); UnsupportedSize is a size
	// it does not make (the zero value: none is tried).
	LabelSizes      []marketplace.LabelSize
	UnsupportedSize marketplace.LabelSize
	// NewCourierOrder makes a new order of a scheme that leaves by a courier's code, and ExpectCourierCode makes the marketplace
	// expect that code for it, with that many wrong tries allowed; replacing says the marketplace is replacing the code, so that
	// its next check answers that a new code is coming.
	NewCourierOrder   func(t *testing.T, key map[string]string) string
	ExpectCourierCode func(t *testing.T, key map[string]string, number, code string, attempts int, replacing bool)
	// ShipmentKinds are the marketplace's own words for how a batch it makes leaves (BatchState.Kind); MakeShipment makes the batch
	// the marketplace makes by itself, with the orders in it. PassCutoff makes the marketplace's time for closing the batch of
	// the orders come, and ExcludeFromShipment makes it take the orders out of it.
	ShipmentKinds       []string
	MakeShipment        func(t *testing.T, key map[string]string, spec ShipmentSpec)
	PassCutoff          func(t *testing.T, key map[string]string, orders []string)
	ExcludeFromShipment func(t *testing.T, key map[string]string, orders []string)
}

// maxFeedReadings bounds the readings of a feed in one case: a feed that never ends is a defect.
const maxFeedReadings = 10

// maxReadings bounds the readings of a state that the marketplace takes a few readings to settle (a packing, a labels
// task): a state that never settles is a defect.
const maxReadings = 12

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

	for c, run := range changesCases() {
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

	checkFlow(t, a, desc)
	checkSteps(t, a, desc)

	expect(t, desc.OrdersPeriod >= 0 && desc.StatesPeriod >= 0, "the periods %v and %v of the exchange are not negative", desc.OrdersPeriod, desc.StatesPeriod)
	expect(t, !desc.CodesReplace || desc.Supports(marketplace.CodesReport) || desc.CodesInPack, "the adapter says its codes replace and claims no codes.report and does not take its codes in the packing report")
	checkPlan8(t, a, desc)
	expect(t, !desc.Supports(marketplace.OrdersChanges) || (!desc.Supports(marketplace.OrdersNew) && !desc.Supports(marketplace.OrdersStates)),
		"the adapter claims orders.changes beside orders.new or orders.states: the changes are read in place of both")
	expect(t, !desc.Supports(marketplace.NotificationsReceive) || desc.Supports(marketplace.OrdersNamed),
		"the adapter claims notifications.receive and not orders.named, which reads the orders a notification names")
	expect(t, desc.MaxInFlight >= 0 && len(desc.ProgramHeader) <= 100, "at most %d requests at once and the program header %q",
		desc.MaxInFlight, desc.ProgramHeader)

	writes := false

	for _, c := range desc.Capabilities {
		expect(t, marketplace.Supported(c), "the adapter claims %s, which the engine does not drive", c)
		expect(t, a.Capable(c, false), "the adapter claims %s but says a real key cannot do it", c)

		writes = writes || slices.Contains([]marketplace.Capability{
			marketplace.OrdersConfirm, marketplace.OrdersPack, marketplace.CodesReport, marketplace.HandoverGroup, marketplace.HandoverClose,
			marketplace.OrdersReady, marketplace.OrdersCourierCode,
		}, c)
	}

	for _, route := range a.Routes() {
		for _, test := range []bool{false, true} {
			_, ok := a.Limit(route.Group, test)
			expect(t, ok, "%s %s counts in the group %q, which has no limit (test key: %v)", route.Method, route.Template, route.Group, test)
		}

		expect(t, strings.HasPrefix(route.Template, "/"), "the route %q does not begin with /", route.Template)
		expect(t, !route.Write || writes, "%s %s writes, and the adapter claims no part that writes", route.Method, route.Template)
		checkRouteHost(t, a, route)
	}
}

// checkRouteHost checks the host a route is pinned to and that no key is promised away from it.
func checkRouteHost(t *testing.T, a marketplace.Adapter, route marketplace.Route) {
	t.Helper()

	if route.Host != "" {
		_, wildcard := marketplace.WildcardDomain(route.Host)
		expect(t, !strings.Contains(route.Host, "*") || wildcard, "%s %s is pinned to %q, which is neither a host nor *.domain of two labels", route.Method, route.Template, route.Host)
		expect(t, slices.ContainsFunc(a.Hosts(), func(h string) bool { return strings.EqualFold(h, route.Host) }),
			"%s %s is pinned to %q, which the adapter's hosts do not list, so the outbound gate would refuse it", route.Method, route.Template, route.Host)
	}

	expect(t, !route.NoKey || (route.Host != "" && !route.Write), "%s %s goes without a key: it must name its host and must not write", route.Method, route.Template)
}

// claimsConfirm and claimsPack say which flow of the exchange the adapter takes: confirming an order into a batch, or
// reporting its packing.
func claimsConfirm(a marketplace.Adapter) bool {
	return a.Describe().Supports(marketplace.OrdersConfirm)
}

func claimsPack(a marketplace.Adapter) bool { return a.Describe().Supports(marketplace.OrdersPack) }

// checkFlow checks that the adapter takes one flow, and that every part the flow needs is claimed together with its
// interface.
func checkFlow(t *testing.T, a marketplace.Adapter, desc marketplace.Description) {
	t.Helper()

	expect(t, !claimsConfirm(a) || !claimsPack(a), "the adapter claims orders.confirm and orders.pack: the exchange takes one flow, chosen by the part")

	_, labelsNow := a.(marketplace.LabelFetcher)
	_, labelsLater := a.(marketplace.LabelMaker)
	_, labelsByBox := a.(marketplace.BoxLabeler)
	expect(t, !desc.Supports(marketplace.LabelsFetch) || btoi(labelsNow)+btoi(labelsLater)+btoi(labelsByBox) == 1,
		"the adapter claims labels.fetch and implements %d of the three forms of it, want exactly one", btoi(labelsNow)+btoi(labelsLater)+btoi(labelsByBox))

	_, plain := a.(marketplace.CodeReporter)
	_, withProduct := a.(marketplace.ProductCodeReporter)
	expect(t, !desc.Supports(marketplace.CodesReport) || plain != withProduct, "the adapter claims codes.report and implements %d of the two forms of it, want exactly one", btoi(plain)+btoi(withProduct))

	expect(t, !desc.Supports(marketplace.HandoverGroup) || claimsConfirm(a) || claimsPack(a),
		"the adapter claims handover.group and neither orders.confirm nor orders.pack: it has no way to make an order ready for a batch")
}

func btoi(b bool) int {
	if b {
		return 1
	}

	return 0
}

// checkSteps checks the steps the adapter declares: they are the six steps' names in their order, each has the part that
// does it, and an adapter that claims handover.group needs the two steps that group.
func checkSteps(t *testing.T, a marketplace.Adapter, desc marketplace.Description) {
	t.Helper()

	last := -1

	for _, step := range desc.HandoverSteps {
		at := slices.Index(marketplace.HandoverStepsAll, step)
		expect(t, at > last, "the hand-over step %q is not one of the six, is there twice or is out of their order", step)

		last = at
	}

	if desc.NeedsStep(marketplace.StepBoxes) && desc.Supports(marketplace.HandoverGroup) {
		_, ok := a.(marketplace.BoxAdder)
		expect(t, ok, "the adapter needs the step boxes and does not implement BoxAdder")
	}

	if desc.NeedsStep(marketplace.StepShipping) && desc.Supports(marketplace.HandoverGroup) {
		_, ok := a.(marketplace.ShippingSetter)
		expect(t, ok, "the adapter needs the step shipping and does not implement ShippingSetter")
	}

	expect(t, !desc.NeedsStep(marketplace.StepClose) || !desc.Supports(marketplace.HandoverGroup) || desc.Supports(marketplace.HandoverClose),
		"the adapter needs the step close and does not claim handover.close")
	expect(t, !desc.NeedsStep(marketplace.StepDocuments) || !desc.Supports(marketplace.HandoverGroup) || desc.Supports(marketplace.HandoverDocuments),
		"the adapter needs the step documents and does not claim handover.documents")
	expect(t, !desc.Supports(marketplace.HandoverGroup) || (desc.NeedsStep(marketplace.StepBatch) && desc.NeedsStep(marketplace.StepMove)),
		"the adapter claims handover.group and does not need the steps batch and move that it does")
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
	expect(t, err == nil && len(kept) > 0 && (facts.ProfileID != "") != f.ProfileFromCheck,
		"a working key: %+v, fields kept %v, %v; the key names its profile: %v", facts, len(kept) > 0, err, !f.ProfileFromCheck)

	checked, err := c.CheckKey(t.Context(), f.Conn(t, key), facts)
	expect(t, err == nil && checked.ProfileID != "" && (f.ProfileFromCheck || checked.ProfileID == facts.ProfileID), "CheckKey: %+v, %v", checked, err)
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

// checkPlan8 checks the promises of the description that belong to the parts of Plan 8: codes in the packing report need the
// interface that takes them and no codes.report, a batch the marketplace makes by itself has neither transport boxes nor shipping
// to set, and an adapter that names a scheme that leaves by a courier's code claims the part that checks the code.
func checkPlan8(t *testing.T, a marketplace.Adapter, desc marketplace.Description) {
	t.Helper()

	_, packs := a.(marketplace.BoxPacker)
	_, readies := a.(marketplace.OrderReadier)

	expect(t, !desc.CodesInPack || (packs && desc.Supports(marketplace.OrdersPack) && !desc.Supports(marketplace.CodesReport)),
		"the adapter says its codes go in the packing report: it must implement BoxPacker, claim orders.pack and no codes.report")
	expect(t, !desc.Supports(marketplace.OrdersReady) || (readies && packs), "the adapter claims orders.ready and does not implement OrderReadier with BoxPacker, which reads the order before it is written")
	expect(t, !desc.BatchFound || (desc.Supports(marketplace.HandoverGroup) && !desc.NeedsStep(marketplace.StepBoxes) && !desc.NeedsStep(marketplace.StepShipping)),
		"the adapter says its batch is found: it claims handover.group and needs neither the step boxes nor the step shipping")
	expect(t, len(desc.CourierSchemes) == 0 || desc.Supports(marketplace.OrdersCourierCode), "the adapter names schemes that leave by a courier's code and does not claim orders.courier_code")
}
