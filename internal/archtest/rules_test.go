package archtest_test

import (
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

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
	"internal/marketplace/ozon/ozonapi":              {"internal/marketplace/ozon", "internal/fakes/ozon"},
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

func TestEveryAdapterStandsAloneInThisRepository(t *testing.T) {
	t.Parallel()

	if violations := archtest.AdaptersStandAlone(loadRepository(t)); len(violations) > 0 {
		t.Fatalf("an adapter reaches beyond its contract:\n%s", strings.Join(violations, "\n"))
	}
}

// namedIn are, for each marketplace, the only folders whose source may name it (rule R6): its adapter, the fakes and the
// program that starts them, and the one list of the adapters the wyo server is built with (internal/server/adapters.go),
// which must import each adapter to hold it. The outbound gate names none: it lists the hosts the adapters declare
// (principle IX). Tests and generated code are not looked at. Every adapter has a row: a new one without a row fails.
var namedIn = map[string][]string{
	"wildberries": {"internal/marketplace/wildberries", "internal/fakes", "cmd/wyofakes", "internal/server/adapters.go"},
	"ozon":        {"internal/marketplace/ozon", "internal/fakes", "cmd/wyofakes", "internal/server/adapters.go"},
}

func TestEveryMarketplaceIsNamedOnlyByItsAdapterInThisRepository(t *testing.T) {
	t.Parallel()

	root := os.DirFS(moduleRoot)

	codes, err := archtest.AdapterCodes(root)
	if err != nil || len(codes) == 0 {
		t.Fatalf("AdapterCodes = %v, %v: the rule would guard nothing", codes, err)
	}

	for _, code := range codes {
		allowed, listed := namedIn[code]
		if !listed {
			t.Errorf("the adapter %q has no row of namedIn: say which folders may name it", code)

			continue
		}

		found, err := archtest.NamedOnlyIn(root, code, allowed)
		if err != nil {
			t.Fatalf("NamedOnlyIn: %v", err)
		}

		if len(found) > 0 {
			t.Errorf("a marketplace is named outside its adapter:\n%s", strings.Join(found, "\n"))
		}
	}
}

func TestEveryClientOfAnAdapterIsBuiltOnTheEnginesConnInThisRepository(t *testing.T) {
	t.Parallel()

	found, err := archtest.ClientsOnTheConn(os.DirFS(moduleRoot), "internal/marketplace")
	if err != nil {
		t.Fatalf("ClientsOnTheConn: %v", err)
	}

	if len(found) > 0 {
		t.Fatalf("an adapter could call out by itself:\n%s", strings.Join(found, "\n"))
	}
}

// neverCalled are the names of the generated methods the wyo server must never call: it never cancels an order at
// Wildberries, whatever the exchange finds (plan 4, section G). Their only place is the package that generated them. Ozon's
// cancel methods are never generated at all (TestTheCancelMethodsOfOzonAreNeverGeneratedInThisRepository).
var neverCalled = []string{"PatchV3OrdersOrderIdCancel"}

const generatedWildberriesOrders = "internal/marketplace/wildberries/wbapi/orders"

func TestTheCancelMethodOfWildberriesIsCalledNowhereInThisRepository(t *testing.T) {
	t.Parallel()

	found, err := archtest.IdentifiersUsedOutside(os.DirFS(moduleRoot), neverCalled, generatedWildberriesOrders)
	if err != nil {
		t.Fatalf("IdentifiersUsedOutside: %v", err)
	}

	if len(found) > 0 {
		t.Fatalf("the wyo server must never cancel an order at Wildberries:\n%s", strings.Join(found, "\n"))
	}
}

func TestTheCancelMethodOfWildberriesIsCaughtWhereverItIsPlanted(t *testing.T) {
	t.Parallel()

	for _, where := range []string{
		"internal/marketplace/wildberries/orders.go", "internal/exchange/steps.go", "internal/fakes/wildberries/orders_test.go", "cmd/wyo/main.go",
	} {
		t.Run(where, func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				where: {Data: []byte("package x\nfunc cancel() { client.PatchV3OrdersOrderIdCancelWithResponse(ctx, 1) }")},
				generatedWildberriesOrders + "/orders.gen.go": {Data: []byte("// Code generated by x DO NOT EDIT.\npackage orders\nfunc PatchV3OrdersOrderIdCancel() {}")},
			}

			found, err := archtest.IdentifiersUsedOutside(tree, neverCalled, generatedWildberriesOrders)
			if err != nil || len(found) != 1 || !strings.Contains(found[0], where) {
				t.Fatalf("found %v, %v; want the planted use in %s only", found, err, where)
			}
		})
	}
}

// fulfilmentClient is the generated client of the neutral fulfilment-centre API.
const fulfilmentClient = "internal/fulfilment/fcapi"

// TestOnlyTheFulfilmentPackagesAndTheFakeFulfilmentCentreMayImportItsClient plants a package that imports the client of the
// fulfilment centre and asks the repository's own table of owners (generatedOwners, applied to the real tree by
// TestGeneratedPackagesAreImportedOnlyByTheirOwnersInThisRepository) whether it is allowed to.
func TestOnlyTheFulfilmentPackagesAndTheFakeFulfilmentCentreMayImportItsClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		importer  string
		violation bool
	}{
		{name: "the fulfilment package", importer: "internal/fulfilment"},
		{name: "a package below it", importer: "internal/fulfilment/receiver"},
		{name: "the fake fulfilment centre", importer: "internal/fakes/fulfilment"},
		{name: "the exchange engine", importer: "internal/exchange", violation: true},
		{name: "the hand-over steps", importer: "internal/handover", violation: true},
		{name: "a marketplace adapter", importer: "internal/marketplace/wildberries", violation: true},
		{name: "the fake Wildberries", importer: "internal/fakes/wildberries", violation: true},
		{name: "the program that starts the fakes", importer: "cmd/wyofakes", violation: true},
		{name: "the wyo server", importer: "cmd/wyo", violation: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			planted := []archtest.Package{
				{Path: fulfilmentClient, Generated: true},
				{Path: tc.importer, Imports: []string{fulfilmentClient}},
			}

			violations := archtest.GeneratedOnlyByOwners(planted, generatedOwners)
			mentioned := slices.ContainsFunc(violations, func(v string) bool { return strings.Contains(v, fulfilmentClient) && strings.Contains(v, tc.importer) })

			if mentioned != tc.violation {
				t.Fatalf("imported from %s: violations %v, want one: %v", tc.importer, violations, tc.violation)
			}
		})
	}
}

// guardedFromMarketplaces are the folders that know no marketplace: the exchange with a fulfilment centre works through
// the adapter contract and the neutral fulfilment-centre API alone (rule R6 and plan 4). They may not exist yet.
var guardedFromMarketplaces = []string{"internal/exchange", "internal/fulfilment", "internal/handover"}

func TestNoFolderOfTheExchangeNamesAMarketplaceInThisRepository(t *testing.T) {
	t.Parallel()

	root := os.DirFS(moduleRoot)

	codes, err := archtest.AdapterCodes(root)
	if err != nil || len(codes) == 0 {
		t.Fatalf("AdapterCodes = %v, %v: the rule would guard nothing", codes, err)
	}

	found, err := archtest.NamedNowhereIn(root, codes, guardedFromMarketplaces)
	if err != nil {
		t.Fatalf("NamedNowhereIn: %v", err)
	}

	if len(found) > 0 {
		t.Fatalf("a marketplace is named where the exchange works for all of them:\n%s", strings.Join(found, "\n"))
	}
}

func TestAMarketplaceNamedInTheExchangeIsCaughtWhateverThePackage(t *testing.T) {
	t.Parallel()

	for _, dir := range guardedFromMarketplaces {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				"internal/marketplace/wildberries/wildberries.go": {Data: []byte("package wildberries")},
				dir + "/planted.go": {Data: []byte("package x // marketplace-api.wildberries.ru")},
			}

			codes, err := archtest.AdapterCodes(tree)
			if err != nil {
				t.Fatalf("AdapterCodes: %v", err)
			}

			found, err := archtest.NamedNowhereIn(tree, codes, guardedFromMarketplaces)
			if err != nil || len(found) != 1 || !strings.Contains(found[0], dir+"/planted.go") {
				t.Fatalf("found %v, %v; want the planted name in %s only", found, err, dir)
			}
		})
	}
}

// ozonCancels are the operations of Ozon's description file that cancel a posting (OZC §5): the wyo server never cancels
// an order at a marketplace (principle XII), so the generator's list never names them and no generated code holds them.
var ozonCancels = []string{"PostingAPI_CancelFbsPosting", "PostingAPI_CancelFbsPostingProduct"}

func TestTheCancelMethodsOfOzonAreNeverGeneratedInThisRepository(t *testing.T) {
	t.Parallel()

	config, err := os.ReadFile(moduleRoot + "/api/codegen/ozon.cfg.yaml")
	if err != nil {
		t.Fatalf("read the generator's list: %v", err)
	}

	generated, err := os.ReadFile(moduleRoot + "/internal/marketplace/ozon/ozonapi/ozonapi.gen.go")
	if err != nil {
		t.Fatalf("read the generated client: %v", err)
	}

	if !strings.Contains(string(config), "include-operation-ids:") {
		t.Fatal("the generator's list names no operation: every operation of Ozon's file would be generated")
	}

	for _, op := range ozonCancels {
		if strings.Contains(string(config), op) {
			t.Errorf("api/codegen/ozon.cfg.yaml lists %s, a method that cancels a posting at Ozon", op)
		}

		if goName := strings.ReplaceAll(op, "_", ""); strings.Contains(string(generated), goName) {
			t.Errorf("the generated client holds %s, a method that cancels a posting at Ozon", goName)
		}
	}
}

// ozonMethods are the operations of Ozon's description file the generator's list must name, each the research's section 5 (OZC)
// gives a plan: the reading ones of Plan 5, then the writing ones of Plan 6 (the marking codes, the packing report, the
// labels, the shipment and its documents; the shipment's own cancel is among them, the two methods that cancel a posting are not).
var ozonMethods = []string{
	"AccessAPI_RolesByToken", "SellerAPI_SellerInfo", "WarehouseListV2", "PostingFbsUnfulfilledList", "PostingAPI_GetFbsPostingV3",
	"PostingFboList", "ProductAPI_GetProductInfoList",
	"PostingAPI_FbsPostingProductExemplarCreateOrGetV6", "PostingAPI_FbsPostingProductExemplarSetV6",
	"PostingAPI_FbsPostingProductExemplarStatusV5", "PostingAPI_ShipFbsPostingV4", "PostingFbsPackageLabelCreate",
	"PostingFbsPackageLabelGet", "CarriageAPI_CarriageDeliveryListV2", "CarriageAPI_CarriageCreate", "CarriageAPI_SetPostings",
	"CarriageAPI_CarriageApprove", "CarriageGet", "CarriageAPI_CarriageCancel", "PostingAPI_PostingFBSActCheckStatus",
	"PostingAPI_ActPostingList", "PostingAPI_PostingFBSGetAct", "PostingAPI_PostingFBSGetBarcode", "PostingAPI_PostingFBSGetBarcodeText",
}

func TestTheGeneratorsListOfOzonMethodsNamesExactlyTheMethodsThePlansNeed(t *testing.T) {
	t.Parallel()

	config, err := os.ReadFile(moduleRoot + "/api/codegen/ozon.cfg.yaml")
	if err != nil {
		t.Fatalf("read the generator's list: %v", err)
	}

	generated, err := os.ReadFile(moduleRoot + "/internal/marketplace/ozon/ozonapi/ozonapi.gen.go")
	if err != nil {
		t.Fatalf("read the generated client: %v", err)
	}

	var listed []string

	for line := range strings.Lines(string(config)) {
		if name, found := strings.CutPrefix(strings.TrimSpace(line), "- "); found {
			listed = append(listed, name)
		}
	}

	slices.Sort(listed)

	want := append([]string{}, ozonMethods...)
	slices.Sort(want)

	if strings.Join(listed, "\n") != strings.Join(want, "\n") {
		t.Errorf("the generator's list names\n%s\nwant\n%s", strings.Join(listed, "\n"), strings.Join(want, "\n"))
	}

	// A method that is listed has its call in the generated client: the client's call is the operation's name without its
	// underscores, followed by WithResponse.
	for _, op := range ozonMethods {
		if goName := strings.ReplaceAll(op, "_", "") + "WithResponse"; !strings.Contains(string(generated), goName) {
			t.Errorf("the generated client has no %s for %s: run pnpm generate", goName, op)
		}
	}
}

// The Go tests run a second time with the test tag `wyotest` only for the packages listed in
// scripts/go/test-tag-packages.txt, and the list is exactly the packages whose tests run different code in the two builds:
// one missing would lose the tests that exist only in the test build, one too many runs the same code twice for nothing
// (the maintainer, 2026-11-19: «run only the 10 packages twice», after a pull request's run took 25 minutes).
func TestTheRunWithTheTestTagHoldsExactlyThePackagesThatDifferBetweenTheBuilds(t *testing.T) {
	t.Parallel()

	release, err := archtest.LoadBuild(t.Context(), moduleRoot, modulePath, "")
	if err != nil {
		t.Fatalf("LoadBuild of the release build: %v", err)
	}

	test, err := archtest.LoadBuild(t.Context(), moduleRoot, modulePath, "wyotest")
	if err != nil {
		t.Fatalf("LoadBuild of the test build: %v", err)
	}

	want := archtest.DifferBetweenBuilds(release, test)
	if len(want) == 0 {
		t.Fatal("no package differs between the builds: the rule would guard nothing")
	}

	list, err := os.ReadFile(moduleRoot + "/scripts/go/test-tag-packages.txt")
	if err != nil {
		t.Fatalf("read the list: %v", err)
	}

	var listed []string

	for line := range strings.Lines(string(list)) {
		if name := strings.TrimSpace(line); name != "" && !strings.HasPrefix(name, "#") {
			listed = append(listed, strings.TrimPrefix(name, "./"))
		}
	}

	if !slices.Equal(listed, want) {
		t.Errorf("scripts/go/test-tag-packages.txt lists, in this order,\n%s\nwant\n%s",
			strings.Join(listed, "\n"), strings.Join(want, "\n"))
	}
}
