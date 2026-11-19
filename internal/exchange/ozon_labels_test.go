package exchange_test

import (
	"strings"
	"testing"
	"time"
)

func TestALabelOzonDoesNotMakeIsAProblemInOzonsWordsAndTheRetryFetchesIt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		plan  func(number string) map[string]any
		check func(t *testing.T, code, words string)
	}{
		{
			name: "the posting is among the unprinted ones",
			plan: func(number string) map[string]any {
				return map[string]any{"unprinted": []map[string]any{{"posting_number": number, "message": "Этикетка не напечатана: нет данных о весе"}}}
			},
			check: func(t *testing.T, code, words string) {
				t.Helper()

				if code != "label_not_made" || words != "Этикетка не напечатана: нет данных о весе" {
					t.Fatalf("the code %q and the words %q", code, words)
				}
			},
		},
		{
			name: "the task ends in an error",
			plan: func(string) map[string]any {
				return map[string]any{"error_tasks": 1, "error": map[string]any{"code": "LABEL_ERROR", "message": "Ошибка формирования файла"}}
			},
			check: func(t *testing.T, code, words string) {
				t.Helper()

				if code != "labels_task_failed" || !strings.Contains(words, "Ошибка формирования файла") {
					t.Fatalf("the code %q and the words %q", code, words)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.walkPosting(t, 1)
			x := r.reserved(t, number)
			r.ozControl(t, "/__control/labels", tc.plan(number))
			r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
			r.until(t, "a problem on the order", func() bool { return r.problem(t, number).Code != "" })

			p := r.problem(t, number)
			if p.Code != "labels_not_made" || !p.Sent {
				t.Fatalf("the problem: %+v", p)
			}

			tc.check(t, p.MpCode, p.Words)

			if x = r.exchanged(t, number); x.FailedStep != "labels" || x.Attention != "problem" {
				t.Fatalf("the order: %+v", x)
			}

			if got := r.fcOrderProblem(t, number); got == nil || got.Code != "labels_not_made" || got.Message != p.Words {
				t.Fatalf("the problem at the fulfilment centre: %+v", got)
			}

			// No label was attached; nothing more is asked for however long the wyo server waits.
			for range 5 {
				r.tick(t, 5*time.Minute)
			}

			if got := len(r.ozCallsTo(t, "/v3/posting/fbs/package-label/create")); got != 1 {
				t.Fatalf("%d label tasks were made, want 1 while the problem stands", got)
			}

			if got := count(r.fcWrites(t), "PUT /api/v1/orders/{id}/problem"); got != 1 {
				t.Fatalf("the problem was reported to the fulfilment centre %d times, want once", got)
			}

			if got := count(r.history(t, number), "problem"); got != 1 {
				t.Fatalf("the history holds %d problem rows, want one: %v", got, r.history(t, number))
			}

			// Ozon can make the label now; the owner presses «Передать заново».
			r.ozControl(t, "/__control/labels", map[string]any{})

			if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
				t.Fatalf("Retry: %v", err)
			}

			if got := r.untilState(t, number, "label_sent"); got.Problem.Code != "" {
				t.Fatalf("a problem stands after the retry: %+v", got.Problem)
			}

			if _, _, text := r.fcLabel(t, number, 1); !strings.HasPrefix(text, "%PDF") {
				t.Fatalf("the label at the fulfilment centre: %q", text)
			}

			if r.fcOrderProblem(t, number) != nil {
				t.Fatal("the problem is still on the order at the fulfilment centre")
			}
		})
	}
}

func TestALabelFileThatCannotBeTakenIsRefusedBeforeAnythingIsFetchedOrPassedOn(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		plan map[string]any
		code string
		// fetched is how many files the wyo server asked the fake for.
		fetched int
	}{
		{name: "a link on a host outside ozone.ru", plan: map[string]any{"foreign_host": "files.example.com"}, code: "file_link_refused", fetched: 0},
		{name: "a file that is not a PDF", plan: map[string]any{"file": "text"}, code: "file_not_a_label_or_document", fetched: 1},
		{name: "a PDF over the 5 MB the neutral API takes", plan: map[string]any{"file": "large"}, code: "file_too_large", fetched: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.walkPosting(t, 1)
			r.reserved(t, number)
			r.ozControl(t, "/__control/labels", tc.plan)
			r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
			r.until(t, "a problem on the order", func() bool { return r.problem(t, number).Code != "" })

			p := r.problem(t, number)
			if p.Code != "marketplace_refused" || p.Params["step"] != "labels" || p.MpCode != tc.code {
				t.Fatalf("the problem: %+v", p)
			}

			if got := len(r.labelFiles(t)); got != tc.fetched {
				t.Fatalf("%d label files were fetched, want %d", got, tc.fetched)
			}

			for _, f := range r.labelFiles(t) {
				if f.WithKey {
					t.Fatalf("a label file was fetched with a key: %s", f.Path)
				}
			}

			// No label reached the fulfilment centre.
			if got := count(r.fcWrites(t), "PUT /api/v1/orders/{id}/parcels/{n}/label"); got != 0 {
				t.Fatalf("%d labels were passed on", got)
			}

			if rows := r.fileRows(t); len(rows) != tc.fetched {
				t.Fatalf("the journal names %d label files (%v), want %d", len(rows), rows, tc.fetched)
			}

			r.ozAssertNothingSecretLeft(t)
		})
	}
}

func TestTheLabelOfTheWarehousesLabelTypeIsTheOneFetchedAndTheFileIsFetchedAfterTheTaskIsMade(t *testing.T) {
	t.Parallel()

	cases := []struct {
		labelType string
		wantKind  string
	}{
		{labelType: "BIG", wantKind: "/big_label/"},
		{labelType: "SMALL", wantKind: "/small_label/"},
		{labelType: "UNSPECIFIED", wantKind: "/big_label/"},
	}

	for _, tc := range cases {
		t.Run(tc.labelType, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t, func(s ozSetup) {
				s["warehouses"] = []map[string]any{{"id": ozSellerWarehouse, "name": "Склад FBS", "label_type": tc.labelType}}
			})
			r.connect(t)

			// The task is not made at the first readings: the wyo server reads it again.
			r.ozControl(t, "/__control/labels", map[string]any{"readings": 3})

			number := r.walkPosting(t, 1)
			r.reserved(t, number)
			r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
			r.untilLabels(t, number)

			files := r.labelFiles(t)
			if len(files) != 1 || !strings.Contains(files[0].Path, tc.wantKind) {
				t.Fatalf("the label files fetched: %+v, want one of %s", files, tc.wantKind)
			}

			if got := len(r.ozCallsTo(t, "/v2/posting/fbs/package-label/get")); got != 4 {
				t.Fatalf("the task was read %d times, want 3 readings that find it not made and a fourth that finds it made", got)
			}

			if got := len(r.ozCallsTo(t, "/v3/posting/fbs/package-label/create")); got != 1 {
				t.Fatalf("%d label tasks were made", got)
			}
		})
	}
}
