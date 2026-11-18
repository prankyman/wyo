package ozon_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/fakes/ozon"
	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// The two methods of the labels (OZC section 8) and the host the made-up label files live on: Ozon's own example link names
// ir.ozone.ru (OZC section 8).
const (
	pathLabelCreate = "/v3/posting/fbs/package-label/create"
	pathLabelGet    = "/v2/posting/fbs/package-label/get"
	labelHost       = "ir.ozone.ru"
)

type labelTasks struct {
	Tasks []struct {
		TaskID   int64  `json:"task_id"`
		TaskType string `json:"task_type"`
	} `json:"tasks"`
}

type labelAnswer struct {
	FileURL string `json:"file_url"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Status struct {
		Code                 string `json:"code"`
		PostingsCount        int    `json:"postings_count"`
		PrintedPostingsCount int    `json:"printed_postings_count"`
		UnprintedPostings    []struct {
			PostingNumber string `json:"posting_number"`
			Message       string `json:"message"`
		} `json:"unprinted_postings"`
	} `json:"status"`
}

// packedRig is a rig with the postings 100-1 and 200-1 packed (awaiting_deliver), each of one unit of 5001.
func packedRig(t *testing.T) *rig {
	t.Helper()

	r := newReadyRig(t)
	r.twoProducts()

	for _, number := range []string{"100-1", "200-1"} {
		r.newPosting(number, 24, nil)

		if _, got := r.ship(number, false, pack(5001, 1)); got.status != http.StatusOK {
			t.Fatalf("pack %s: %d %s", number, got.status, got.body)
		}
	}

	return r
}

// labelTasksFor asks for the labels of the postings and returns the task of each type.
func (r *rig) labelTasksFor(numbers ...string) map[string]int64 {
	r.t.Helper()

	var out labelTasks

	r.answer(pathLabelCreate, map[string]any{"posting_numbers": numbers}, &out)

	tasks := map[string]int64{}
	for _, task := range out.Tasks {
		tasks[task.TaskType] = task.TaskID
	}

	return tasks
}

func (r *rig) label(task int64) labelAnswer {
	r.t.Helper()

	var out labelAnswer

	r.answer(pathLabelGet, map[string]any{"task_id": task}, &out)

	return out
}

// fetch gets the file of a link the way the wyo server does: at the fake's own address with the link's host as the first part
// of the path, and with the headers given (none is how the wyo server asks for it).
func (r *rig) fetch(link string, headers map[string]string) reply {
	r.t.Helper()

	u, err := url.Parse(link)
	if err != nil {
		r.t.Fatalf("the link %q: %v", link, err)
	}

	return r.send(http.MethodGet, "/"+u.Host+u.Path, headers, "")
}

func TestAskingForLabelsMakesTwoTasksOneOfEachTypeLikeOzonsOwnExample(t *testing.T) {
	t.Parallel()

	r := packedRig(t)

	got := r.call(pathLabelCreate, map[string]any{"posting_numbers": []string{"100-1", "200-1"}})
	same(t, "the call answers", got.status, http.StatusOK)

	var out labelTasks

	strictInto(t, got.body, &ozonapi.PostingV3PostingFbsPackageLabelCreateResponse{})
	decode(t, got.body, &out)

	same(t, "the number of tasks", len(out.Tasks), 2)
	same(t, "the first type", out.Tasks[0].TaskType, "big_label")
	same(t, "the second type", out.Tasks[1].TaskType, "small_label")

	if out.Tasks[0].TaskID == 0 || out.Tasks[0].TaskID == out.Tasks[1].TaskID {
		t.Fatalf("the tasks are %+v, want two numbers of their own", out.Tasks)
	}
}

func TestAskingForLabelsIsRefusedForAPostingThatIsNotThereOrNotPackedOrNoneAtAll(t *testing.T) {
	t.Parallel()

	r := packedRig(t)
	r.newPosting("300-1", 24, nil)

	cases := []struct {
		name    string
		numbers []string
		status  int
		words   string
	}{
		{name: "no number", numbers: nil, status: http.StatusBadRequest, words: "posting_numbers is required"},
		{name: "a posting that is not there", numbers: []string{"100-1", "nobody-1"}, status: http.StatusNotFound, words: "POSTING_NOT_FOUND"},
		{name: "a posting that is not awaiting deliver", numbers: []string{"300-1"}, status: http.StatusBadRequest, words: wordsIncorrectStatus},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			refused(t, r.forTest(t).call(pathLabelCreate, map[string]any{"posting_numbers": tc.numbers}), tc.status, tc.words)
		})
	}
}

func TestATaskIsPendingThenInProgressForTheReadingsAskedForThenCompletedWithAFileOnOzonesHost(t *testing.T) {
	t.Parallel()

	r := packedRig(t)
	r.control("labels", map[string]any{"readings": 3}, nil)

	task := r.labelTasksFor("100-1")["big_label"]

	for i, want := range []string{"pending", "in_progress", "in_progress"} {
		got := r.label(task)

		same(t, "the status of reading "+want, got.Status.Code, want)
		same(t, "no file yet", got.FileURL, "")

		if i == 0 && got.Status.PostingsCount != 1 {
			t.Fatalf("the task covers %d postings, want 1", got.Status.PostingsCount)
		}
	}

	done := r.label(task)

	same(t, "the status", done.Status.Code, "completed")
	same(t, "the postings asked for", done.Status.PostingsCount, 1)
	same(t, "the postings printed", done.Status.PrintedPostingsCount, 1)
	same(t, "no unprinted posting", len(done.Status.UnprintedPostings), 0)

	link, err := url.Parse(done.FileURL)
	if err != nil || link.Scheme != "https" || link.Host != labelHost || !strings.HasSuffix(link.Path, ".pdf") {
		t.Fatalf("the file link is %q, want an https link to %s ending in .pdf", done.FileURL, labelHost)
	}

	same(t, "reading it again gives the same link", r.label(task).FileURL, done.FileURL)
}

func TestTheLabelFileIsServedWithoutAKeyBeginsAsAPDFAndNamesItsPostingsAndItsType(t *testing.T) {
	t.Parallel()

	r := packedRig(t)
	tasks := r.labelTasksFor("100-1", "200-1")

	big, small := r.label(tasks["big_label"]), r.label(tasks["small_label"])
	got := r.fetch(big.FileURL, nil)

	same(t, "the file answers", got.status, http.StatusOK)
	same(t, "its type", got.header.Get("Content-Type"), "application/pdf")

	if !strings.HasPrefix(got.body, "%PDF-") {
		t.Fatalf("the file begins %q, want %%PDF-", got.body[:min(10, len(got.body))])
	}

	for _, want := range []string{"100-1", "200-1", "big_label"} {
		if !strings.Contains(got.body, want) {
			t.Fatalf("the file does not name %q: %q", want, got.body)
		}
	}

	if other := r.fetch(small.FileURL, nil); !strings.Contains(other.body, "small_label") || big.FileURL == small.FileURL {
		t.Fatalf("the small label's file is %q at %s, want a file of its own", other.body, small.FileURL)
	}
}

func TestAPostingTheCommandNamesIsAmongTheUnprintedAndKeepsOutOfTheFile(t *testing.T) {
	t.Parallel()

	r := packedRig(t)
	r.control("labels", map[string]any{"unprinted": []map[string]any{{"posting_number": "200-1", "message": "the label cannot be made"}}}, nil)

	done := r.label(r.labelTasksFor("100-1", "200-1")["big_label"])

	same(t, "the status", done.Status.Code, "completed")
	same(t, "the postings printed", done.Status.PrintedPostingsCount, 1)

	if u := done.Status.UnprintedPostings; len(u) != 1 || u[0].PostingNumber != "200-1" || u[0].Message != "the label cannot be made" {
		t.Fatalf("the unprinted postings are %+v", u)
	}

	file := r.fetch(done.FileURL, nil)

	if !strings.Contains(file.body, "100-1") || strings.Contains(file.body, "200-1") {
		t.Fatalf("the file holds %q, want the label of 100-1 only", file.body)
	}
}

func TestATaskWhoseEveryPostingIsUnprintedHasNoFile(t *testing.T) {
	t.Parallel()

	r := packedRig(t)
	r.control("labels", map[string]any{"unprinted": []map[string]any{{"posting_number": "100-1", "message": "no"}}}, nil)

	done := r.label(r.labelTasksFor("100-1")["big_label"])

	same(t, "the status", done.Status.Code, "completed")
	same(t, "the file", done.FileURL, "")
	same(t, "the postings printed", done.Status.PrintedPostingsCount, 0)
}

func TestTheNextTasksEndInAnErrorAsOftenAsCommandedAndThenTheyAreMade(t *testing.T) {
	t.Parallel()

	r := packedRig(t)
	r.control("labels", map[string]any{"error_tasks": 1, "error": map[string]any{"code": "LABEL_ERROR", "message": "the file was not made"}}, nil)

	failed := r.label(r.labelTasksFor("100-1")["big_label"])

	same(t, "the status", failed.Status.Code, "error")
	same(t, "the error's code", failed.Error.Code, "LABEL_ERROR")
	same(t, "the error's words", failed.Error.Message, "the file was not made")
	same(t, "no file", failed.FileURL, "")

	same(t, "the next task", r.label(r.labelTasksFor("100-1")["big_label"]).Status.Code, "completed")
}

func TestTheLinkCanBeOnAHostOutsideOzonesDomainAndTheFileCanBeNotAPDFOrTooLarge(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		plan     map[string]any
		wantHost string
		wantType string
		wantLen  int
		wantHead string
	}{
		{name: "a foreign host", plan: map[string]any{"foreign_host": "files.example.com"}, wantHost: "files.example.com"},
		{name: "a file that is not a PDF", plan: map[string]any{"file": "text"}, wantHost: labelHost, wantType: "text/html", wantHead: "<html>"},
		{name: "a file over 5 MB", plan: map[string]any{"file": "large"}, wantHost: labelHost, wantType: "application/pdf", wantLen: 5<<20 + 1, wantHead: "%PDF-"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := packedRig(t)
			r.control("labels", tc.plan, nil)

			done := r.label(r.labelTasksFor("100-1")["big_label"])

			link, err := url.Parse(done.FileURL)
			if err != nil || link.Host != tc.wantHost {
				t.Fatalf("the link is %q, want the host %s", done.FileURL, tc.wantHost)
			}

			if tc.wantHead == "" {
				return
			}

			got := r.fetch(done.FileURL, nil)

			same(t, "the type of the file", got.header.Get("Content-Type"), tc.wantType)

			if !strings.HasPrefix(got.body, tc.wantHead) || (tc.wantLen > 0 && len(got.body) != tc.wantLen) {
				t.Fatalf("the file is %d bytes beginning %q, want %d beginning %q", len(got.body), got.body[:min(12, len(got.body))], tc.wantLen, tc.wantHead)
			}
		})
	}
}

func TestTheFakeListsEveryFetchOfALabelFileAndWhetherItCarriedAKey(t *testing.T) {
	t.Parallel()

	r := packedRig(t)
	link := r.label(r.labelTasksFor("100-1")["big_label"]).FileURL

	r.fetch(link, nil)
	r.fetch(link, map[string]string{"Api-Key": r.key})
	r.fetch(link, map[string]string{"Client-Id": r.clientID})
	r.fetch(link, map[string]string{"Authorization": "Bearer " + r.key})

	got := r.send(http.MethodGet, "/__control/label-files", map[string]string{"Authorization": r.key}, "")
	same(t, "the control answers", got.status, http.StatusOK)

	var out struct {
		Files []struct {
			Path    string `json:"path"`
			WithKey bool   `json:"with_key"`
		} `json:"files"`
	}

	decode(t, got.body, &out)

	var carried []bool

	for _, f := range out.Files {
		carried = append(carried, f.WithKey)

		if !strings.HasPrefix(f.Path, "/s3/") || strings.Contains(f.Path, r.key) {
			t.Fatalf("a fetch is listed with the path %q", f.Path)
		}
	}

	same(t, "which fetches carried a key", asJSON(t, carried), "[false,true,true,true]")
}

func TestAFileThatWasNeverMadeIsNotFoundAndAFileIsOnlyRead(t *testing.T) {
	t.Parallel()

	r := packedRig(t)
	link := r.label(r.labelTasksFor("100-1")["big_label"]).FileURL

	same(t, "an unknown file", r.fetch("https://"+labelHost+"/s3/ord-tmp-12/big_label/ticket-none.pdf", nil).status, http.StatusNotFound)

	u, _ := url.Parse(link)
	same(t, "a file written to", r.send(http.MethodPost, "/"+u.Host+u.Path, nil, "x").status, http.StatusMethodNotAllowed)
}

func TestAnUnknownTaskIsNotFoundAndTheFakeStandsForTheFileHostAsWellAsTheApiHost(t *testing.T) {
	t.Parallel()

	r := packedRig(t)

	refused(t, r.call(pathLabelGet, map[string]any{"task_id": 1}), http.StatusNotFound, "TASK_NOT_FOUND")
	same(t, "the hosts the fake stands for", strings.Join(ozon.New(ozon.Options{Clock: r.clock}).Hosts(), ","), ozon.Host+","+ozon.FileHost)
}

func TestTheAnswersOfTheLabelMethodsAreInTheShapeOfOzonsOwnDescriptionFile(t *testing.T) {
	t.Parallel()

	r := packedRig(t)
	r.control("labels", map[string]any{"unprinted": []map[string]any{{"posting_number": "200-1", "message": "no"}}}, nil)

	tasks := r.labelTasksFor("100-1", "200-1")

	strictInto(t, r.call(pathLabelGet, ozonapi.PostingV2PostingFbsPackageLabelGetRequest{TaskId: tasks["big_label"]}).body, &ozonapi.PostingV2PostingFbsPackageLabelGetResponse{})
	strictInto(t, r.call(pathLabelGet, ozonapi.PostingV2PostingFbsPackageLabelGetRequest{TaskId: tasks["small_label"]}).body, &ozonapi.PostingV2PostingFbsPackageLabelGetResponse{})
}
