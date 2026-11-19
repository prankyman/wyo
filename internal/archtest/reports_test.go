package archtest_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// reportsBeforeTheRule are the reports of .agents/reports/ written before the rule «an issue found on the way is fixed in
// the plan at once, and the report lists it under «Found while implementing»» (constitution 1.6.0, 2026-11-19).
var reportsBeforeTheRule = map[string]string{
	"wyo-repository-set-up-plan-1-report.md":                                                                           "written before the rule",
	"wyo-installable-program-plan-2-report.md":                                                                         "written before the rule",
	"wyo-wildberries-accounts-orders-journal-plan-3-report.md":                                                         "written before the rule",
	"wyo-external-api-wildberries-fbs-exchange-plan-4-report.md":                                                       "written before the rule",
	"wyo-ozon-accounts-orders-plan-5-report.md":                                                                        "written before the rule",
	"wyo-ozon-fbs-exchange-plan-6-report.md":                                                                           "written before the rule",
	"wyo-plan-6a-ozon-exchange-review-fixes-report.md":                                                                 "written before the rule",
	"wyo-plan-6a-ozon-exchange-review-fixes-endless-retries-shipment-recheck-split-original-docs-smoke-test-report.md": "written before the rule",
}

// foundHeading is the section every report has from the rule on: a heading «Found while implementing».
var foundHeading = regexp.MustCompile(`(?m)^#{2,3} Found while implementing\b`)

// TestEveryPlanReportListsWhatWasFoundWhileImplementing is the policy test of the rule of constitution 1.6.0 («Plan
// Implementation Workflow», step 2): every report of .agents/reports/ has the section «Found while implementing», which
// names every finding and how it was fixed, or why it could not be and which register item holds it. The reports written
// before the rule are the allow-list; an entry that names no report fails, so the list can only shrink.
func TestEveryPlanReportListsWhatWasFoundWhileImplementing(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join(moduleRoot, ".agents", "reports", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no report in .agents/reports/: %v", err)
	}

	var names []string

	for _, file := range files {
		name := filepath.Base(file)
		names = append(names, name)

		if _, before := reportsBeforeTheRule[name]; before {
			continue
		}

		raw, err := os.ReadFile(file) //nolint:gosec // G304: a report of this repository
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		if !foundHeading.Match(raw) {
			t.Errorf("%s has no section «Found while implementing» (constitution, «Plan Implementation Workflow», step 2)", name)
		}
	}

	for name := range reportsBeforeTheRule {
		if !slices.Contains(names, name) {
			t.Errorf("the allow-list names %s, which is no report: remove it", name)
		}
	}
}
