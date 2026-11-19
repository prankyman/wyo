package ozon_test

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
)

func TestOneParcelIsReportedWithItsProductsBySKUAndTheAdditionalDataAndReadAsPacked(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	r.posting(t, key, "44444444-0001-1", warehouseFBS, product(2001, "SHIRT", 2), product(2002, "SOCKS", 1))

	parts, err := r.adapter.Pack(t.Context(), conn, "44444444-0001-1", []marketplace.Parcel{{Lines: []marketplace.ParcelLine{{ItemID: "2001", Quantity: 2}, {ItemID: "2002", Quantity: 1}}}})
	want := []marketplace.PackedPart{{Number: "44444444-0001-1", Lines: []marketplace.ParcelLine{{ItemID: "2001", Quantity: 2}, {ItemID: "2002", Quantity: 1}}}}

	if err != nil || !reflect.DeepEqual(parts, want) {
		t.Fatalf("Pack = %+v, %v; want %+v", parts, err, want)
	}

	sent := r.calls(t, key, "/v4/posting/fbs/ship")
	if len(sent) != 1 {
		t.Fatalf("%d reports reached Ozon, want one", len(sent))
	}

	body := strings.Join(strings.Fields(sent[0].Body), "")
	for _, piece := range []string{`"posting_number":"44444444-0001-1"`, `"packages":[{"products":[{"product_id":2001,"quantity":2},{"product_id":2002,"quantity":1}]}]`, `"with":{"additional_data":true}`} {
		if !strings.Contains(body, piece) {
			t.Errorf("the report %s lacks %s", sent[0].Body, piece)
		}
	}

	state, err := r.adapter.PackState(t.Context(), conn, "44444444-0001-1")
	if err != nil || !state.Packed || state.Failed || state.Cancelled || !reflect.DeepEqual(state.Lines, want[0].Lines) {
		t.Fatalf("PackState = %+v, %v; want the posting packed with its two lines", state, err)
	}
}

func TestSeveralParcelsSplitThePostingAndEachPartIsReadPackedOnItsOwn(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	r.posting(t, key, "44444444-0002-1", warehouseFBS, product(2001, "SHIRT", 1), product(2002, "SOCKS", 2))

	parts, err := r.adapter.Pack(t.Context(), conn, "44444444-0002-1", []marketplace.Parcel{
		{Lines: []marketplace.ParcelLine{{ItemID: "2001", Quantity: 1}, {ItemID: "2002", Quantity: 1}}},
		{Lines: []marketplace.ParcelLine{{ItemID: "2002", Quantity: 1}}},
	})
	if err != nil || len(parts) != 2 || parts[0].Number == parts[1].Number {
		t.Fatalf("Pack = %+v, %v; want two parts with numbers of their own", parts, err)
	}

	if !reflect.DeepEqual(parts[0].Lines, []marketplace.ParcelLine{{ItemID: "2001", Quantity: 1}, {ItemID: "2002", Quantity: 1}}) ||
		!reflect.DeepEqual(parts[1].Lines, []marketplace.ParcelLine{{ItemID: "2002", Quantity: 1}}) {
		t.Fatalf("the parts hold %+v", parts)
	}

	for _, part := range parts {
		state, err := r.adapter.PackState(t.Context(), conn, part.Number)
		if err != nil || !state.Packed || !reflect.DeepEqual(state.Lines, part.Lines) {
			t.Fatalf("PackState(%s) = %+v, %v; want packed with the part's lines", part.Number, state, err)
		}
	}
}

func TestOzonsRefusalsOfTheReportAreRefusalsInItsOwnWordsAndAnUnknownProductIsNeverSent(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	r.posting(t, key, "44444444-0003-1", warehouseFBS, product(2002, "SOCKS", 2))

	_, err := r.adapter.Pack(t.Context(), conn, "44444444-0003-1", []marketplace.Parcel{{Lines: []marketplace.ParcelLine{{ItemID: "2002", Quantity: 1}}}})

	refusal := asRefusal(t, err)
	if refusal.Code != "HAS_INCORRECT_PRODUCT_QUANTITY" || refusal.Params["text"] != "HAS_INCORRECT_PRODUCT_QUANTITY" || refusal.Status == 0 {
		t.Fatalf("a report that leaves a unit out is refused as %+v", refusal)
	}

	_, err = r.adapter.Pack(t.Context(), conn, "99999999-0001-1", []marketplace.Parcel{{Lines: []marketplace.ParcelLine{{ItemID: "2002", Quantity: 1}}}})
	if refusal = asRefusal(t, err); refusal.Code != "POSTING_NOT_FOUND" {
		t.Fatalf("a posting Ozon does not know is refused as %+v", refusal)
	}

	before := len(r.calls(t, key, "/v4/posting/fbs/ship"))

	_, err = r.adapter.Pack(t.Context(), conn, "44444444-0003-1", []marketplace.Parcel{{Lines: []marketplace.ParcelLine{{ItemID: "no-sku", Quantity: 1}}}})
	if refusal = asRefusal(t, err); refusal.Code != "not_a_number" || len(r.calls(t, key, "/v4/posting/fbs/ship")) != before {
		t.Fatalf("a product that has no SKU is %+v and nothing may be sent for it", refusal)
	}
}

func TestAPostingThatIsAlreadyPackedIsDoneAndGivesNoParts(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	r.posting(t, key, "44444444-0004-1", warehouseFBS, product(2002, "SOCKS", 1))

	parcels := []marketplace.Parcel{{Lines: []marketplace.ParcelLine{{ItemID: "2002", Quantity: 1}}}}

	if _, err := r.adapter.Pack(t.Context(), conn, "44444444-0004-1", parcels); err != nil {
		t.Fatalf("the first Pack: %v", err)
	}

	parts, err := r.adapter.Pack(t.Context(), conn, "44444444-0004-1", parcels)
	if err != nil || parts != nil {
		t.Fatalf("a second Pack = %+v, %v; want success with no parts (POSTING_ALREADY_SHIPPED means it is done)", parts, err)
	}
}

func TestACancelledPostingAndAReportThatDidNotTakeEffectAreRead(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	parcels := []marketplace.Parcel{{Lines: []marketplace.ParcelLine{{ItemID: "2002", Quantity: 1}}}}

	// A 200 is not the end: the report takes effect only after two readings of the posting, and the first report ends in ship_failed.
	r.posting(t, key, "44444444-0005-1", warehouseFBS, product(2002, "SOCKS", 1))
	r.plan(t, key, "/__control/postings/ship", map[string]any{"posting_number": "44444444-0005-1", "readings": 2, "failures": 1})

	if _, err := r.adapter.Pack(t.Context(), conn, "44444444-0005-1", parcels); err != nil {
		t.Fatalf("Pack: %v", err)
	}

	readings := func(n int) []string {
		var seen []string

		for range n {
			state, err := r.adapter.PackState(t.Context(), conn, "44444444-0005-1")
			if err != nil {
				t.Fatalf("PackState: %v", err)
			}

			if state.Failed && state.Words != "ship_failed" {
				t.Fatalf("a failed report gives the words %q, want Ozon's substatus ship_failed", state.Words)
			}

			seen = append(seen, stateWord(state))
		}

		return seen
	}

	if seen := readings(3); !slices.Equal(seen, []string{"waiting", "waiting", "failed"}) {
		t.Fatalf("the readings after the first report were %v", seen)
	}

	// The report is made again, as Ozon says, and this time it takes effect after the same two readings.
	if _, err := r.adapter.Pack(t.Context(), conn, "44444444-0005-1", parcels); err != nil {
		t.Fatalf("Pack again: %v", err)
	}

	if seen := readings(3); !slices.Equal(seen, []string{"waiting", "waiting", "packed"}) {
		t.Fatalf("the readings after the second report were %v", seen)
	}

	r.posting(t, key, "44444444-0005-2", warehouseFBS, product(2002, "SOCKS", 1))
	r.control(t, key, "/__control/postings/status", map[string]any{"posting_number": "44444444-0005-2", "status": "cancelled", "cancellation_type": "customer"})

	cancelled, err := r.adapter.PackState(t.Context(), conn, "44444444-0005-2")
	if err != nil || !cancelled.Cancelled || cancelled.Packed || cancelled.Failed || cancelled.Words == "" {
		t.Fatalf("PackState of a cancelled posting = %+v, %v; want cancelled with Ozon's reason", cancelled, err)
	}
}

func TestAPostingSplitIntoPartsReadsSplitAndNeverCancelled(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	// The split made in the cabinet: Ozon shows the original as cancelled_from_split_pending (OZC §6), which is no cancellation.
	r.posting(t, key, "44444444-0008-1", warehouseFBS, product(2002, "SOCKS", 2))
	r.control(t, key, "/__control/postings/split", map[string]any{
		"posting_number": "44444444-0008-1",
		"parts": []map[string]any{
			{"posting_number": "44444444-0008-2", "products": []map[string]any{{"sku": 2002, "quantity": 1}}},
			{"posting_number": "44444444-0008-3", "products": []map[string]any{{"sku": 2002, "quantity": 1}}},
		},
	})

	split, err := r.adapter.PackState(t.Context(), conn, "44444444-0008-1")
	if err != nil || !split.Split || split.Cancelled || split.Packed || split.Failed {
		t.Fatalf("PackState of a split original = %+v, %v; want only Split", split, err)
	}

	// A posting that is cancelled stays Cancelled and is not Split.
	r.posting(t, key, "44444444-0008-4", warehouseFBS, product(2002, "SOCKS", 1))
	r.control(t, key, "/__control/postings/status", map[string]any{"posting_number": "44444444-0008-4", "status": "cancelled", "cancellation_type": "customer"})

	cancelled, err := r.adapter.PackState(t.Context(), conn, "44444444-0008-4")
	if err != nil || !cancelled.Cancelled || cancelled.Split {
		t.Fatalf("PackState of a cancelled posting = %+v, %v; want Cancelled and not Split", cancelled, err)
	}
}

func TestAnOriginalOfOurOwnSplitReadsSplitAndEveryPartPacked(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	r.posting(t, key, "44444444-0009-1", warehouseFBS, product(2002, "SOCKS", 2))
	r.control(t, key, "/__control/ship-split", map[string]any{"original_split_pending": true})

	parts, err := r.adapter.Pack(t.Context(), conn, "44444444-0009-1", []marketplace.Parcel{
		{Lines: []marketplace.ParcelLine{{ItemID: "2002", Quantity: 1}}}, {Lines: []marketplace.ParcelLine{{ItemID: "2002", Quantity: 1}}},
	})
	if err != nil || len(parts) != 2 || parts[0].Number != "44444444-0009-2" || parts[1].Number != "44444444-0009-3" {
		t.Fatalf("Pack = %+v, %v; want two parts with new numbers", parts, err)
	}

	original, err := r.adapter.PackState(t.Context(), conn, "44444444-0009-1")
	if err != nil || !original.Split || original.Cancelled || original.Packed {
		t.Fatalf("PackState of the original = %+v, %v; want only Split", original, err)
	}

	for _, part := range parts {
		if state, err := r.adapter.PackState(t.Context(), conn, part.Number); err != nil || !state.Packed || state.Split || state.Cancelled {
			t.Fatalf("PackState(%s) = %+v, %v; want packed", part.Number, state, err)
		}
	}
}

func TestAMissingRoleOnTheReadingOfAPostingNamesTheReadLevelAndNotTheWriteLevel(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	r.posting(t, key, "44444444-0010-1", warehouseFBS, product(2002, "SOCKS", 1))

	if err := fakekit.NewControl(r.kit.URL, key["api_key"], r.kit.HTTP).QueueFault(t.Context(), fakekit.Fault{
		Kind: fakekit.FaultRefuse, PathPrefix: "/v3/posting/fbs/get", Status: http.StatusForbidden,
		Body: `{"code":7,"message":"Api-Key is missing a required role for a method","details":[]}`,
	}); err != nil {
		t.Fatalf("QueueFault: %v", err)
	}

	_, err := r.adapter.PackState(t.Context(), conn, "44444444-0010-1")

	refusal := asRefusal(t, err)
	if refusal.Code != "access_level_missing" || refusal.Params["level"] != "Posting FBS read-only" {
		t.Fatalf("a missing role on /v3/posting/fbs/get is refused as %+v, want access_level_missing naming «Posting FBS read-only»", refusal)
	}
}

// stateWord is a one-word reading of a pack state.
func stateWord(s marketplace.PackState) string {
	switch {
	case s.Packed:
		return "packed"
	case s.Failed:
		return "failed"
	case s.Cancelled:
		return "cancelled"
	}

	return "waiting"
}

func TestAKeyThatLacksTheWritingLevelIsRefusedBeforeAnythingIsSent(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key, "posting_fbs_write")

	r.posting(t, key, "44444444-0006-1", warehouseFBS, product(2002, "SOCKS", 1))

	parcels := []marketplace.Parcel{{Lines: []marketplace.ParcelLine{{ItemID: "2002", Quantity: 1}}}}
	checks := []struct {
		name string
		call func() error
	}{
		{"Pack", func() error { _, err := r.adapter.Pack(t.Context(), conn, "44444444-0006-1", parcels); return err }},
		{"AskLabels", func() error {
			_, err := r.adapter.AskLabels(t.Context(), conn, []string{"44444444-0006-1"}, "BIG")
			return err
		}},
		{"ReportProductCodes", func() error {
			_, err := r.adapter.ReportProductCodes(t.Context(), conn, "44444444-0006-1", marketplace.IdentifierMarking, nil)

			return err
		}},
		{"MakeBatch", func() error {
			_, err := r.adapter.MakeBatch(t.Context(), conn, marketplace.BatchRequest{Warehouse: "1001", Day: synckitNoon()})

			return err
		}},
		{"CloseBatch", func() error { return r.adapter.CloseBatch(t.Context(), conn, "1") }},
		{"Documents", func() error { _, err := r.adapter.Documents(t.Context(), conn, "1"); return err }},
	}

	for _, c := range checks {
		refusal := asRefusal(t, c.call())
		if refusal.Code != "write_level_missing" || refusal.Params["level"] != "Posting FBS" {
			t.Errorf("%s is refused as %+v, want write_level_missing naming the level «Posting FBS»", c.name, refusal)
		}
	}

	if n := len(r.calls(t, key, "/v4/posting/fbs/ship")) + len(r.calls(t, key, "/v1/carriage/create")) + len(r.calls(t, key, "/v3/posting/fbs/package-label/create")); n != 0 {
		t.Fatalf("%d writing calls reached Ozon, want none", n)
	}
}

func TestAQuantityBelowOneIsRefusedByTheAdapterAndNothingIsSent(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	r.posting(t, key, "44444444-0007-1", warehouseFBS, product(2002, "SOCKS", 2))

	_, err := r.adapter.Pack(t.Context(), conn, "44444444-0007-1", []marketplace.Parcel{{Lines: []marketplace.ParcelLine{{ItemID: "2002", Quantity: 0}}}})

	if refusal := asRefusal(t, err); refusal.Code != "not_a_number" || refusal.Params["what"] != "quantity" || len(r.calls(t, key, "/v4/posting/fbs/ship")) != 0 {
		t.Fatalf("a quantity of nothing is %+v and nothing may be sent for it", refusal)
	}
}

func TestThePartsOfAReportAreReadFromItsDetailsAndOtherwiseFromTheNumbersOfItsResult(t *testing.T) {
	t.Parallel()

	details := []any{
		map[string]any{"posting_number": "100-1", "products": []any{map[string]any{"sku": 2001, "quantity": 2}}},
		map[string]any{"posting_number": "", "products": []any{}},
		map[string]any{"posting_number": "100-2", "products": []any{}},
	}

	want := []marketplace.PackedPart{
		{Number: "100-1", Lines: []marketplace.ParcelLine{{ItemID: "2001", Quantity: 2}}},
		{Number: "100-2"},
	}

	if got := ozon.PartsOf(details, nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("the parts of the details: %+v, want %+v", got, want)
	}

	if got, want := ozon.PartsOf(nil, []any{"100-1", "100-2"}), []marketplace.PackedPart{{Number: "100-1"}, {Number: "100-2"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("the parts of the result: %+v, want %+v", got, want)
	}

	if got := ozon.PartsOf(nil, nil); len(got) != 0 {
		t.Fatalf("an answer that names no part gives %+v", got)
	}
}
