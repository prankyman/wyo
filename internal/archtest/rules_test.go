package archtest_test

import (
	"os"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/archtest"
)

const (
	moduleRoot = "../.."
	modulePath = "github.com/prankyman/wyo"
	// maxFileLines is the longest a source file may be (tests and generated code are exempt).
	maxFileLines = 800
)

// generatedOwners says which packages may import each generated package. A generated package that is not listed here
// fails the test, so a new generator is a decision about its owner.
var generatedOwners = map[string][]string{
	"internal/httpapi/gen":                           {"internal/httpapi"},
	"internal/store/db":                              {"internal/store"},
	"internal/fulfilment/fcapi":                      {"internal/fulfilment", "internal/fakes/fulfilment"},
	"internal/marketplace/wildberries/wbapi/general": {"internal/marketplace/wildberries", "internal/fakes/wildberries"},
	"internal/marketplace/wildberries/wbapi/items":   {"internal/marketplace/wildberries", "internal/fakes/wildberries"},
	"internal/marketplace/wildberries/wbapi/orders":  {"internal/marketplace/wildberries", "internal/fakes/wildberries"},
	"internal/marketplace/wildberries/wbapi/reports": {"internal/marketplace/wildberries", "internal/fakes/wildberries"},
}

func loadRepository(t *testing.T) []archtest.Package {
	t.Helper()

	// Both build variants: a file with the wyotest tag may import what a release file does not.
	pkgs, err := archtest.Load(t.Context(), moduleRoot, modulePath, "", "wyotest")
	if err != nil {
		t.Fatalf("archtest.Load: %v", err)
	}

	if len(pkgs) == 0 {
		t.Fatal("no package found; the rules would prove nothing")
	}

	return pkgs
}

func TestGeneratedPackagesAreImportedOnlyByTheirOwnersInThisRepository(t *testing.T) {
	t.Parallel()

	if violations := archtest.GeneratedOnlyByOwners(loadRepository(t), generatedOwners); len(violations) > 0 {
		t.Fatalf("generated code is reached from outside its owner:\n%s", strings.Join(violations, "\n"))
	}
}

func TestFakesAreImportedOnlyByFakesInThisRepository(t *testing.T) {
	t.Parallel()

	if violations := archtest.FakesOnlyInFakes(loadRepository(t)); len(violations) > 0 {
		t.Fatalf("a fake is reached from released code:\n%s", strings.Join(violations, "\n"))
	}
}

func TestTestSupportIsImportedOnlyByTestsInThisRepository(t *testing.T) {
	t.Parallel()

	if violations := archtest.TestSupportOnlyInTests(loadRepository(t)); len(violations) > 0 {
		t.Fatalf("test support is reached from released code:\n%s", strings.Join(violations, "\n"))
	}
}

func TestNoGoSourceFileIsTooLong(t *testing.T) {
	t.Parallel()

	long, err := archtest.LongFiles(os.DirFS(moduleRoot), maxFileLines)
	if err != nil {
		t.Fatalf("LongFiles: %v", err)
	}

	if len(long) > 0 {
		t.Fatalf("split these files:\n%s", strings.Join(long, "\n"))
	}
}
