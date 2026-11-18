package ozon_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

// labelRig is a key with a packed posting of one unit and the Conn of its account.
func labelRig(t *testing.T, numbers ...string) (*rig, marketplace.Key, marketplace.Conn) {
	t.Helper()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	for _, n := range numbers {
		r.packedPosting(t, key, conn, n, warehouseFBS, product(skuPlain, "SOCKS", 1))
	}

	return r, key, conn
}

// fetches lists the requests the fake got for label files, and whether any carried a key.
func fetches(t *testing.T, r *rig, key marketplace.Key) (paths []string, withKey bool) {
	t.Helper()

	var got struct {
		Files []struct {
			Path    string `json:"path"`
			WithKey bool   `json:"with_key"`
		} `json:"files"`
	}

	r.get(t, key, "/__control/label-files", &got)

	for _, f := range got.Files {
		paths = append(paths, f.Path)
		withKey = withKey || f.WithKey
	}

	return paths, withKey
}

func TestTheLabelsOfAPostingAreAskedForTheWarehousesLabelTypeAndReadUntilTheyAreMade(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ labelType, task string }{
		{"BIG", "big_label"}, {"SMALL", "small_label"}, {"UNSPECIFIED", "big_label"}, {"", "big_label"},
	} {
		t.Run("label type "+tc.labelType, func(t *testing.T) {
			t.Parallel()

			r, key, conn := labelRig(t, "21111111-0001-1")
			r.plan(t, key, "/__control/labels", map[string]any{"readings": 2})

			ticket, err := r.adapter.AskLabels(t.Context(), conn, []string{"21111111-0001-1"}, tc.labelType)
			if err != nil || ticket == "" {
				t.Fatalf("AskLabels = %q, %v", ticket, err)
			}

			var held struct {
				Tasks   []int64  `json:"t"`
				Numbers []string `json:"n"`
			}

			if err := json.Unmarshal([]byte(ticket), &held); err != nil || len(held.Tasks) != 1 || !reflect.DeepEqual(held.Numbers, []string{"21111111-0001-1"}) {
				t.Fatalf("the ticket %q holds the tasks %v and the postings %v; want the one task of the type and the posting", ticket, held.Tasks, held.Numbers)
			}

			for reading := 1; reading <= 2; reading++ {
				state, err := r.adapter.ReadLabels(t.Context(), conn, ticket)
				if err != nil || state.Ready || state.Failed || len(state.Labels) != 0 {
					t.Fatalf("reading %d: %+v, %v; want the labels not made yet", reading, state, err)
				}
			}

			state, err := r.adapter.ReadLabels(t.Context(), conn, ticket)
			if err != nil || !state.Ready || len(state.Unmade) != 0 || len(state.Labels) != 1 {
				t.Fatalf("ReadLabels = %+v, %v; want the one label made", state, err)
			}

			label := state.Labels[0]
			if label.Number != "21111111-0001-1" || label.ContentType != "application/pdf" || label.Code != "" ||
				!bytes.HasPrefix(label.Data, []byte("%PDF")) || !strings.Contains(string(label.Data), tc.task) {
				t.Fatalf("the label is %q %q code %q: %.80q; want a PDF of the task %s with no scan text", label.Number, label.ContentType, label.Code, label.Data, tc.task)
			}

			paths, withKey := fetches(t, r, key)
			if len(paths) != 1 || withKey {
				t.Fatalf("the file was fetched %v (with a key: %v); want once and without a key", paths, withKey)
			}
		})
	}
}

func TestOneFileOfSeveralPostingsGivesEachPostingALabelOfItsOwnThatSharesIt(t *testing.T) {
	t.Parallel()

	numbers := []string{"21111111-0002-1", "21111111-0003-1"}
	r, key, conn := labelRig(t, numbers...)

	ticket, err := r.adapter.AskLabels(t.Context(), conn, numbers, "BIG")
	if err != nil {
		t.Fatalf("AskLabels: %v", err)
	}

	state, err := r.adapter.ReadLabels(t.Context(), conn, ticket)
	if err != nil || !state.Ready || len(state.Labels) != 2 || state.Labels[0].Number != numbers[0] || state.Labels[1].Number != numbers[1] ||
		!bytes.Equal(state.Labels[0].Data, state.Labels[1].Data) {
		t.Fatalf("ReadLabels = %+v, %v; want one label for each posting, both of the one file", state, err)
	}

	if paths, _ := fetches(t, r, key); len(paths) != 1 {
		t.Fatalf("the file was fetched %d times, want once", len(paths))
	}
}

func TestAPostingOzonLeavesOutIsToldWithOzonsWordsAndHasNoLabel(t *testing.T) {
	t.Parallel()

	numbers := []string{"21111111-0004-1", "21111111-0005-1"}
	r, key, conn := labelRig(t, numbers...)

	r.plan(t, key, "/__control/labels", map[string]any{"unprinted": []map[string]any{{"posting_number": numbers[1], "message": "Не удалось напечатать этикетку"}}})

	ticket, err := r.adapter.AskLabels(t.Context(), conn, numbers, "BIG")
	if err != nil {
		t.Fatalf("AskLabels: %v", err)
	}

	state, err := r.adapter.ReadLabels(t.Context(), conn, ticket)
	if err != nil || !state.Ready || len(state.Labels) != 1 || state.Labels[0].Number != numbers[0] ||
		!reflect.DeepEqual(state.Unmade, map[string]string{numbers[1]: "Не удалось напечатать этикетку"}) {
		t.Fatalf("ReadLabels = %+v, %v; want the first posting's label and the second told as unmade in Ozon's words", state, err)
	}

	// When every posting is left out there is no file at all: the labels are made, none of them.
	r.plan(t, key, "/__control/labels", map[string]any{"unprinted": []map[string]any{
		{"posting_number": numbers[0], "message": "первое"}, {"posting_number": numbers[1], "message": "второе"},
	}})

	ticket, err = r.adapter.AskLabels(t.Context(), conn, numbers, "BIG")
	if err != nil {
		t.Fatalf("AskLabels: %v", err)
	}

	before, _ := fetches(t, r, key)

	state, err = r.adapter.ReadLabels(t.Context(), conn, ticket)
	if after, _ := fetches(t, r, key); err != nil || !state.Ready || len(state.Labels) != 0 || len(state.Unmade) != 2 || len(after) != len(before) {
		t.Fatalf("ReadLabels = %+v, %v; want no label, both told as unmade and no file fetched", state, err)
	}
}

func TestATaskThatEndsInAnErrorFailsTheTicketInOzonsWords(t *testing.T) {
	t.Parallel()

	r, key, conn := labelRig(t, "21111111-0006-1")
	r.plan(t, key, "/__control/labels", map[string]any{"error_tasks": 1, "error": map[string]any{"code": "LABEL_ERROR", "message": "Ошибка при создании файла"}})

	ticket, err := r.adapter.AskLabels(t.Context(), conn, []string{"21111111-0006-1"}, "BIG")
	if err != nil {
		t.Fatalf("AskLabels: %v", err)
	}

	state, err := r.adapter.ReadLabels(t.Context(), conn, ticket)
	if err != nil || state.Ready || !state.Failed || state.Words != "Ошибка при создании файла" || len(state.Labels) != 0 {
		t.Fatalf("ReadLabels = %+v, %v; want the ticket failed with Ozon's message", state, err)
	}

	// With no message Ozon's code is the words.
	r.plan(t, key, "/__control/labels", map[string]any{"error_tasks": 1, "error": map[string]any{"code": "LABEL_ERROR"}})

	ticket, _ = r.adapter.AskLabels(t.Context(), conn, []string{"21111111-0006-1"}, "BIG")

	if state, err = r.adapter.ReadLabels(t.Context(), conn, ticket); err != nil || !state.Failed || state.Words != "LABEL_ERROR" {
		t.Fatalf("ReadLabels = %+v, %v; want Ozon's code as the words", state, err)
	}
}

func TestAFileThatCannotBePassedOnIsRefusedAndNothingIsFetchedFromAForeignHost(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		plan map[string]any
		code string
		host string
	}{
		{"a link to a foreign host", map[string]any{"foreign_host": "files.example.com"}, "file_link_refused", "files.example.com"},
		{"a file that is no PDF", map[string]any{"file": "text"}, "file_not_a_label_or_document", ""},
		{"a PDF over 5 MB", map[string]any{"file": "large"}, "file_too_large", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, key, conn := labelRig(t, "21111111-0007-1")
			r.plan(t, key, "/__control/labels", tc.plan)

			ticket, err := r.adapter.AskLabels(t.Context(), conn, []string{"21111111-0007-1"}, "BIG")
			if err != nil {
				t.Fatalf("AskLabels: %v", err)
			}

			_, err = r.adapter.ReadLabels(t.Context(), conn, ticket)

			refusal := asRefusal(t, err)
			if refusal.Code != tc.code || (tc.host != "" && refusal.Params["host"] != tc.host) {
				t.Fatalf("ReadLabels is refused as %+v; want %s", refusal, tc.code)
			}

			if paths, withKey := fetches(t, r, key); withKey || (tc.host != "" && len(paths) != 0) {
				t.Fatalf("the files fetched were %v (with a key: %v)", paths, withKey)
			}
		})
	}
}

func TestATicketThatIsNotOneOfOursOrNamesATaskOzonDoesNotKnowIsRefused(t *testing.T) {
	t.Parallel()

	r, _, conn := labelRig(t)

	for _, ticket := range []string{"no-such-ticket", "", `{"t":[],"n":[]}`} {
		if _, err := r.adapter.ReadLabels(t.Context(), conn, ticket); asRefusal(t, err).Code != "label_ticket_unreadable" {
			t.Errorf("the ticket %q is not refused as unreadable: %v", ticket, err)
		}
	}

	_, err := r.adapter.ReadLabels(t.Context(), conn, `{"t":[123],"n":["x"]}`)
	if refusal := asRefusal(t, err); refusal.Code != "TASK_NOT_FOUND" {
		t.Fatalf("a task Ozon does not know is refused as %+v", refusal)
	}
}

func TestTheLabelsOfAPostingThatIsNotPackedAreRefusedInOzonsWords(t *testing.T) {
	t.Parallel()

	r, key, conn := labelRig(t)
	r.posting(t, key, "21111111-0008-1", warehouseFBS, product(skuPlain, "SOCKS", 1))

	_, err := r.adapter.AskLabels(t.Context(), conn, []string{"21111111-0008-1"}, "BIG")
	if refusal := asRefusal(t, err); refusal.Code != "HAS_INCORRECT_STATUS" {
		t.Fatalf("AskLabels for a posting still waiting to be packed is refused as %+v", refusal)
	}
}
