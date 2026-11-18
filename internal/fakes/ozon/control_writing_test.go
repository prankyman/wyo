package ozon_test

import (
	"net/http"
	"testing"
)

// writingMethods are the seventeen methods the writing half of the connector calls. What stands before a method's own work (the key,
// the Client ID, the role, the verb) is the same for each, and is proven for each.
var writingMethods = []string{
	pathExemplarGet, pathExemplarSet, pathExemplarStatus, pathShip, pathLabelCreate, pathLabelGet, pathCarriageList, pathCarriageCreate,
	pathCarriageSet, pathCarriageApprove, pathCarriageGet, pathCarriageCancel, pathActStatus, pathActPostings, pathActPDF, pathActBarcode,
	pathActBarcodeText,
}

func TestEveryWritingMethodNeedsTheKeyItsClientIDItsRoleAndAPOST(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)

	for _, path := range writingMethods {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			own := r.forTest(t)

			refused(t, own.send(http.MethodPost, path, nil, ""), http.StatusUnauthorized, wordsInvalidKey)
			refused(t, own.send(http.MethodPost, path, map[string]string{"Api-Key": own.key, "Client-Id": "another-client"}, ""), http.StatusUnauthorized, wordsInvalidKey)

			got := own.send(http.MethodGet, path, own.keyHeaders(), "")

			refused(t, got, http.StatusMethodNotAllowed, wordsMethodMissing)
			same(t, "the Allow header", got.header.Get("Allow"), http.MethodPost)

			// A key whose role opens no method of the writing half is refused, with the words of Ozon.
			stranger := newRig(t)
			stranger.control("setup", map[string]any{"client_id": stranger.clientID, "roles": []map[string]any{{"name": "Warehouse", "methods": []string{"/v2/warehouse"}}}}, nil)

			refused(t, stranger.call(path, map[string]any{"posting_number": "100-1", "id": 1, "carriage_id": 1, "task_id": 1, "limit": 1}), http.StatusForbidden, wordsMissingARole)
		})
	}
}

func TestARoleThatListsTheWritingMethodsOpensThemAndNothingElse(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	r.control("setup", map[string]any{"roles": []map[string]any{{"name": "Posting FBS", "methods": []string{"/v4/posting/fbs/ship", "/v1/carriage", "/v2/carriage", "/v2/posting/fbs/act"}}}}, nil)

	r.newPosting("900-1", 24, nil)

	same(t, "the packing report is open", r.call(pathShip, shipBody("900-1", false, pack(5001, 1))).status, http.StatusOK)
	same(t, "the shipments are open", r.call(pathCarriageList, map[string]any{"limit": 5}).status, http.StatusOK)
	same(t, "the documents are open", r.call(pathActPostings, map[string]any{"id": 1}).status, http.StatusNotFound)
	refused(t, r.call(pathLabelCreate, map[string]any{"posting_numbers": []string{"900-1"}}), http.StatusForbidden, wordsMissingARole)
	refused(t, r.call(pathExemplarGet, map[string]any{"posting_number": "900-1"}), http.StatusForbidden, wordsMissingARole)
}

func TestAWritingControlCallThatCannotBeUsedIsRefusedWith400(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
		body any
	}{
		{name: "exemplar-check for a posting that is not there", path: "postings/exemplar-check", body: map[string]any{"posting_number": "nobody-1"}},
		{name: "exemplar-check with a negative number", path: "postings/exemplar-check", body: map[string]any{"posting_number": "100-1", "failed_sets": -1}},
		{name: "exemplar-check with error sets and no error codes", path: "postings/exemplar-check", body: map[string]any{"posting_number": "100-1", "error_sets": 1}},
		{name: "exemplar-check with a field it does not know", path: "postings/exemplar-check", body: map[string]any{"posting_number": "100-1", "colour": "red"}},
		{name: "ship for a posting that is not there", path: "postings/ship", body: map[string]any{"posting_number": "nobody-1"}},
		{name: "ship with a negative number", path: "postings/ship", body: map[string]any{"posting_number": "100-1", "readings": -1}},
		{name: "labels with a negative number", path: "labels", body: map[string]any{"readings": -1}},
		{name: "labels with error tasks and no error", path: "labels", body: map[string]any{"error_tasks": 1}},
		{name: "labels with a file that does not exist", path: "labels", body: map[string]any{"file": "doc"}},
		{name: "labels with a foreign host that is an address", path: "labels", body: map[string]any{"foreign_host": "https://files.example.com/a"}},
		{name: "labels with an unprinted posting without a number", path: "labels", body: map[string]any{"unprinted": []map[string]any{{"message": "x"}}}},
		{name: "labels with a field it does not know", path: "labels", body: map[string]any{"colour": "red"}},
		{name: "carriages with a negative number", path: "carriages", body: map[string]any{"not_ready": -1}},
		{name: "carriages with a document status that does not exist", path: "carriages", body: map[string]any{"document_statuses": []string{"ready"}}},
		{name: "carriages with a refused posting without a number", path: "carriages", body: map[string]any{"refused_postings": []map[string]any{{"error": "x"}}}},
		{name: "carriages with a field it does not know", path: "carriages", body: map[string]any{"colour": "red"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := markedPosting(t)
			r.controlRefused(tc.path, tc.body)
		})
	}
}

func TestAControlPlanReplacesTheOneBeforeIt(t *testing.T) {
	t.Parallel()

	r := packedRig(t)
	r.control("labels", map[string]any{"readings": 5, "unprinted": []map[string]any{{"posting_number": "100-1", "message": "no"}}}, nil)
	r.control("labels", map[string]any{}, nil)

	done := r.label(r.labelTasksFor("100-1")["big_label"])

	same(t, "a task made after the plan was replaced", done.Status.Code, "completed")
	same(t, "the posting is printed again", done.Status.PrintedPostingsCount, 1)
}
