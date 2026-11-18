package marketplace_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

func TestTheVerdictAgainIsALastWordThatLetsNothingPass(t *testing.T) {
	t.Parallel()

	if !marketplace.VerdictAgain.Final() || marketplace.VerdictAgain.Passes() {
		t.Fatalf("again: final %v, passes %v; want final and not passing", marketplace.VerdictAgain.Final(), marketplace.VerdictAgain.Passes())
	}
}

func TestTheNamesOfABatchAreWrittenInOnePlaceInTheFormTheFulfilmentCentreAlreadyKnows(t *testing.T) {
	t.Parallel()

	moscow := time.FixedZone("MSK", 3*60*60)
	// 20:30 on the 18th in Moscow is already 03:30 on the 19th in Vladivostok.
	vladivostok := time.FixedZone("VLAT", 10*60*60)
	noon := time.Date(2026, 11, 18, 12, 0, 0, 0, moscow)

	if got := marketplace.AssemblyBatchName(noon, moscow); got != "wyo — сборка 18.11.2026" {
		t.Fatalf("AssemblyBatchName = %q", got)
	}

	if got := marketplace.AssemblyBatchName(time.Date(2026, 11, 18, 20, 30, 0, 0, moscow), vladivostok); got != "wyo — сборка 19.11.2026" {
		t.Fatalf("AssemblyBatchName counts the day in the given zone: %q", got)
	}

	day := time.Date(2026, 11, 19, 12, 0, 0, 0, moscow)
	if got := marketplace.HandoverBatchName("H-17", day); got != "wyo — передача H-17 19.11.2026" {
		t.Fatalf("HandoverBatchName = %q", got)
	}
}

func TestADescriptionNeedsTheStepsItListsOrAllSixWhenItListsNone(t *testing.T) {
	t.Parallel()

	if !slices.Equal(marketplace.HandoverStepsAll, []string{"batch", "move", "boxes", "shipping", "close", "documents"}) {
		t.Fatalf("HandoverStepsAll = %v", marketplace.HandoverStepsAll)
	}

	four := marketplace.Description{HandoverSteps: []string{marketplace.StepBatch, marketplace.StepMove, marketplace.StepClose, marketplace.StepDocuments}}

	for _, step := range marketplace.HandoverStepsAll {
		if !(marketplace.Description{}).NeedsStep(step) {
			t.Errorf("a description that lists none does not need %s", step)
		}

		want := step != marketplace.StepBoxes && step != marketplace.StepShipping
		if got := four.NeedsStep(step); got != want {
			t.Errorf("a description of four steps needs %s: %v, want %v", step, got, want)
		}
	}

	if (marketplace.Description{}).NeedsStep("nonsense") || four.NeedsStep("nonsense") {
		t.Fatal("a step nobody knows is needed")
	}
}

func TestAnIdentifierTheMarketplaceNeedsAndABatchItCannotMakeYetAreErrorsThatNameWhatIsMissing(t *testing.T) {
	t.Parallel()

	var err error = &marketplace.NeedsIdentifier{Identifier: marketplace.IdentifierCountry}

	var needs *marketplace.NeedsIdentifier
	if !errors.As(err, &needs) || needs.Identifier != "country" || err.Error() != "the marketplace needs the identifier country" {
		t.Fatalf("NeedsIdentifier: %v", err)
	}

	err = &marketplace.BatchWait{Words: "no shipment yet"}

	var wait *marketplace.BatchWait
	if !errors.As(err, &wait) || wait.Words != "no shipment yet" || err.Error() != "the marketplace cannot make the batch yet" {
		t.Fatalf("BatchWait: %v", err)
	}
}

func TestEveryIdentifierIsNamedOnceInTheConnectorsWords(t *testing.T) {
	t.Parallel()

	all := []string{
		marketplace.IdentifierMarking, marketplace.IdentifierIMEI, marketplace.IdentifierUIN, marketplace.IdentifierGTIN,
		marketplace.IdentifierExpiry, marketplace.IdentifierCustoms, marketplace.IdentifierMultibox, marketplace.IdentifierWeight,
		marketplace.IdentifierCountry, marketplace.IdentifierRNPT,
	}

	seen := map[string]bool{}

	for _, id := range all {
		if id == "" || seen[id] {
			t.Fatalf("the identifier %q is empty or named twice", id)
		}

		seen[id] = true
	}
}
