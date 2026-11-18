package memadapter_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
)

func TestTheMarketplaceInMemoryRemembersWhatItWasAskedAndFailsWhenTold(t *testing.T) {
	t.Parallel()

	a := memadapter.New(2)
	ctx := t.Context()
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	first, second := a.AddOrder(now), a.AddOrder(now)
	a.SetStatus(second, marketplace.Status{State: marketplace.StateAssembling})

	orders, err := a.NewOrders(ctx, marketplace.Conn{})
	if err != nil || len(orders) != 1 || orders[0].Number != first {
		t.Fatalf("only the new order is new: %+v, %v", orders, err)
	}

	states, err := a.States(ctx, marketplace.Conn{}, []string{first, second, "unknown"})
	if err != nil || len(states) != 2 || !reflect.DeepEqual(a.Batches(), [][]string{{first, second, "unknown"}}) {
		t.Fatalf("States = %+v, %v; batches %v", states, err, a.Batches())
	}

	a.Fail(errors.New("down"))

	for name, call := range map[string]func() error{
		"warehouses": func() error { _, err := a.Warehouses(ctx, marketplace.Conn{}); return err },
		"new orders": func() error { _, err := a.NewOrders(ctx, marketplace.Conn{}); return err },
		"states":     func() error { _, err := a.States(ctx, marketplace.Conn{}, []string{first}); return err },
		"feed":       func() error { _, err := a.Feed(ctx, marketplace.Conn{}, "", now); return err },
		"check":      func() error { _, err := a.CheckKey(ctx, marketplace.Conn{}, marketplace.KeyFacts{}); return err },
	} {
		if call() == nil {
			t.Errorf("%s answered while told to fail", name)
		}
	}

	if code, text := a.ErrorOf(500, []byte("body")); code != "" || text != "body" {
		t.Fatalf("ErrorOf = %q, %q", code, text)
	}
}

func TestAStuckFeedAnswersTheWholeFeedWithTheSameMarkAndCountsItsCalls(t *testing.T) {
	t.Parallel()

	a := memadapter.New(2)
	a.AddFeedRow(marketplace.FeedRow{Number: "fbo-1"})
	a.StickFeed()

	for range 2 {
		page, err := a.Feed(t.Context(), marketplace.Conn{}, "stuck", time.Time{})
		if err != nil || page.Done || page.Mark != "stuck" || len(page.Rows) != 1 {
			t.Fatalf("a stuck feed: %+v, %v", page, err)
		}
	}

	if n := a.FeedCalls(); n != 2 {
		t.Fatalf("FeedCalls = %d, want 2", n)
	}
}

// The marketplace in memory answers the writing parts of the contract in either flow, which the contract suite
// (internal/marketplace/contracttest) runs it through; the tests below are for what the suite does not look at: what the
// adapter declares, and the controls a test of the exchange drives it with.

var noon = time.Date(2026, 11, 18, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60))

func TestWithoutWritingTheMarketplaceInMemoryClaimsOnlyTheReadingParts(t *testing.T) {
	t.Parallel()

	d := memadapter.New(2).Describe()

	for _, part := range []marketplace.Capability{
		marketplace.OrdersConfirm, marketplace.OrdersPack, marketplace.LabelsFetch, marketplace.CodesReport, marketplace.CodesVerdict,
		marketplace.HandoverGroup, marketplace.HandoverClose, marketplace.HandoverDocuments,
	} {
		if d.Supports(part) {
			t.Errorf("a marketplace in memory that was not asked to write claims %s", part)
		}
	}

	if d.HandoverSteps != nil || d.CodesReplace || d.OrdersPeriod != 0 || d.StatesPeriod != 0 {
		t.Fatalf("the description of a reading marketplace carries %+v", d)
	}
}

func TestTheConfirmFlowClaimsOrdersConfirmWithSixStepsAndThePackFlowClaimsOrdersPackWithFourAndReplacingCodes(t *testing.T) {
	t.Parallel()

	confirm := memadapter.NewConfirming(2).Describe()
	pack := memadapter.NewPacking(2).Describe()

	if !confirm.Supports(marketplace.OrdersConfirm) || confirm.Supports(marketplace.OrdersPack) || confirm.CodesReplace ||
		!reflect.DeepEqual(confirm.HandoverSteps, marketplace.HandoverStepsAll) {
		t.Fatalf("the confirm flow: %+v", confirm)
	}

	wantSteps := []string{marketplace.StepBatch, marketplace.StepMove, marketplace.StepClose, marketplace.StepDocuments}
	if !pack.Supports(marketplace.OrdersPack) || pack.Supports(marketplace.OrdersConfirm) || !pack.CodesReplace || !reflect.DeepEqual(pack.HandoverSteps, wantSteps) {
		t.Fatalf("the pack flow: %+v", pack)
	}

	for _, d := range []marketplace.Description{confirm, pack} {
		for _, part := range []marketplace.Capability{
			marketplace.LabelsFetch, marketplace.CodesReport, marketplace.CodesVerdict, marketplace.HandoverGroup,
			marketplace.HandoverClose, marketplace.HandoverDocuments,
		} {
			if !d.Supports(part) {
				t.Errorf("%+v does not claim %s", d.Capabilities, part)
			}
		}
	}
}

func TestAPackedOrderShowsPackedOnlyAfterTheReadingsATestAsksFor(t *testing.T) {
	t.Parallel()

	a := memadapter.NewPacking(2)
	a.PackAfter(3)

	number := a.AddOrder(noon)
	lines := []marketplace.ParcelLine{{ItemID: "mem-product", Quantity: 1}}

	parts, err := a.Pack(t.Context(), marketplace.Conn{}, number, []marketplace.Parcel{{Lines: lines}})
	if err != nil || len(parts) != 1 || parts[0].Number != number {
		t.Fatalf("Pack = %+v, %v", parts, err)
	}

	for reading := 1; reading <= 3; reading++ {
		state, err := a.PackState(t.Context(), marketplace.Conn{}, number)
		if err != nil || state.Packed != (reading == 3) {
			t.Fatalf("reading %d: %+v, %v; want packed only at the third", reading, state, err)
		}
	}
}

func TestSeveralParcelsSplitAnOrderIntoPartsOfTheirOwnAndTheLinesMustAddUp(t *testing.T) {
	t.Parallel()

	a := memadapter.NewPacking(2)
	number := a.AddOrderOf(marketplace.Order{Lines: []marketplace.Line{{ProductID: "p1", Quantity: 2}}, CreatedAt: noon})

	one := []marketplace.ParcelLine{{ItemID: "p1", Quantity: 1}}

	parts, err := a.Pack(t.Context(), marketplace.Conn{}, number, []marketplace.Parcel{{Lines: one}, {Lines: one}})
	if err != nil || len(parts) != 2 || parts[0].Number == parts[1].Number || parts[0].Number == number {
		t.Fatalf("Pack = %+v, %v; want two parts with numbers of their own", parts, err)
	}

	other := a.AddOrderOf(marketplace.Order{Lines: []marketplace.Line{{ProductID: "p1", Quantity: 2}}, CreatedAt: noon})

	_, err = a.Pack(t.Context(), marketplace.Conn{}, other, []marketplace.Parcel{{Lines: one}})

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "parcels_differ" {
		t.Fatalf("parcels that hold one unit of two: %v, want the refusal parcels_differ", err)
	}
}

func TestAnOrderThatNeedsAMarkingCodeIsNotPackedBeforeItsCodesAreSent(t *testing.T) {
	t.Parallel()

	a := memadapter.NewPacking(2)
	number := a.AddMarkedOrder(noon)
	lines := []marketplace.ParcelLine{{ItemID: "mem-product", Quantity: 1}}

	_, err := a.Pack(t.Context(), marketplace.Conn{}, number, []marketplace.Parcel{{Lines: lines}})

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "marking_missing" {
		t.Fatalf("Pack without the codes: %v, want the refusal marking_missing", err)
	}

	sent, err := a.ReportProductCodes(t.Context(), marketplace.Conn{}, number, marketplace.IdentifierMarking, []marketplace.UnitCode{{ItemID: "mem-product", Code: "code-1"}})
	if err != nil || !sent {
		t.Fatalf("ReportProductCodes = %v, %v", sent, err)
	}

	if _, err := a.Pack(t.Context(), marketplace.Conn{}, number, []marketplace.Parcel{{Lines: lines}}); err != nil {
		t.Fatalf("Pack after the codes: %v", err)
	}
}

func TestAnIdentifierOtherThanAMarkingCodeCanBeRequiredByATest(t *testing.T) {
	t.Parallel()

	a := memadapter.NewPacking(2)
	number := a.AddMarkedOrder(noon)
	a.RequireIdentifier(number, marketplace.IdentifierCountry)

	_, err := a.ReportProductCodes(t.Context(), marketplace.Conn{}, number, marketplace.IdentifierMarking, []marketplace.UnitCode{{ItemID: "mem-product", Code: "code-1"}})

	var needs *marketplace.NeedsIdentifier
	if !errors.As(err, &needs) || needs.Identifier != marketplace.IdentifierCountry {
		t.Fatalf("ReportProductCodes = %v, want a *NeedsIdentifier for the country", err)
	}
}

func TestLabelsAreMadeAfterTheReadingsATestAsksForAndAChangedLabelChangesItsFingerprint(t *testing.T) {
	t.Parallel()

	a := memadapter.NewPacking(2)
	a.LabelsAfter(2)

	number := a.AddOrder(noon)

	if _, err := a.Pack(t.Context(), marketplace.Conn{}, number, []marketplace.Parcel{{Lines: []marketplace.ParcelLine{{ItemID: "mem-product", Quantity: 1}}}}); err != nil {
		t.Fatalf("Pack: %v", err)
	}

	ticket, err := a.AskLabels(t.Context(), marketplace.Conn{}, []string{number}, "BIG")
	if err != nil || ticket == "" {
		t.Fatalf("AskLabels = %q, %v", ticket, err)
	}

	for reading := 1; reading <= 2; reading++ {
		state, err := a.ReadLabels(t.Context(), marketplace.Conn{}, ticket)
		if err != nil || state.Ready != (reading == 2) {
			t.Fatalf("reading %d: %+v, %v; want ready only at the second", reading, state, err)
		}
	}

	if got := a.LabelTypesAsked(); !reflect.DeepEqual(got, []string{"BIG"}) {
		t.Fatalf("the kinds of label asked for: %v", got)
	}
}

func TestABatchOfTheNameAskedForIsFoundNotMadeTwiceAndAMarketplaceThatCannotYetAnswersABatchWait(t *testing.T) {
	t.Parallel()

	a := memadapter.NewPacking(2)
	req := marketplace.BatchRequest{Name: marketplace.HandoverBatchName("H-1", noon), Warehouse: "1", Day: noon}

	first, err := a.MakeBatch(t.Context(), marketplace.Conn{}, req)
	if err != nil {
		t.Fatalf("MakeBatch: %v", err)
	}

	second, err := a.MakeBatch(t.Context(), marketplace.Conn{}, req)
	if err != nil || second != first {
		t.Fatalf("MakeBatch twice = %q then %q, %v; want the same batch", first, second, err)
	}

	a.BatchWaits(1)

	_, err = a.MakeBatch(t.Context(), marketplace.Conn{}, marketplace.BatchRequest{Name: "another"})

	var wait *marketplace.BatchWait
	if !errors.As(err, &wait) || wait.Words == "" {
		t.Fatalf("MakeBatch while the marketplace cannot = %v, want a *BatchWait", err)
	}

	if _, err := a.MakeBatch(t.Context(), marketplace.Conn{}, marketplace.BatchRequest{Name: "another"}); err != nil {
		t.Fatalf("MakeBatch once the marketplace can: %v", err)
	}
}

func TestTheCallsAMarketplaceInMemoryWasAskedToWriteAreCountedByTheirName(t *testing.T) {
	t.Parallel()

	a := memadapter.NewConfirming(2)
	number := a.AddOrder(noon)

	batch, err := a.NewBatch(t.Context(), marketplace.Conn{}, "name")
	if err != nil {
		t.Fatalf("NewBatch: %v", err)
	}

	for range 2 {
		if err := a.PutInBatch(t.Context(), marketplace.Conn{}, batch, []string{number}); err != nil {
			t.Fatalf("PutInBatch: %v", err)
		}
	}

	if got := a.Calls("NewBatch"); got != 1 {
		t.Fatalf("NewBatch was called %d times, want 1", got)
	}

	if got := a.Calls("PutInBatch"); got != 2 {
		t.Fatalf("PutInBatch was called %d times, want 2", got)
	}
}

func TestABatchThatWasHandedOverIsNeverFoundAgainByItsName(t *testing.T) {
	t.Parallel()

	a := memadapter.NewPacking(2)
	ctx := t.Context()
	number := a.AddOrder(noon)
	req := marketplace.BatchRequest{Name: "same name", Warehouse: "1", Day: noon}

	if _, err := a.Pack(ctx, marketplace.Conn{}, number, []marketplace.Parcel{{Lines: []marketplace.ParcelLine{{ItemID: "mem-product", Quantity: 1}}}}); err != nil {
		t.Fatalf("Pack: %v", err)
	}

	first, _ := a.MakeBatch(ctx, marketplace.Conn{}, req)
	if err := a.FillBatch(ctx, marketplace.Conn{}, first, []string{number}); err != nil {
		t.Fatalf("FillBatch: %v", err)
	}

	if err := a.CloseBatch(ctx, marketplace.Conn{}, first); err != nil {
		t.Fatalf("CloseBatch: %v", err)
	}

	second, err := a.MakeBatch(ctx, marketplace.Conn{}, req)
	if err != nil || second == first {
		t.Fatalf("MakeBatch after the first was handed over = %q, %v; want a new batch, not %q", second, err, first)
	}
}
