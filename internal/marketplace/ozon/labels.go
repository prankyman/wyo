package ozon

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// The two types of a label task, «task_type» (OZC §8): «big_label — для обычной этикетки; small_label — для маленькой этикетки».
const (
	taskBig   = "big_label"
	taskSmall = "small_label"
)

// The status of a task, «status.code» of /v2/posting/fbs/package-label/get (OZC §8): «pending — задание в очереди; in_progress —
// формируются; completed — файл с этикетками готов; error — ошибка при создании файла».
const (
	taskCompleted = "completed"
	taskError     = "error"
)

// taskTypeOf is the type of the label task a warehouse's label type asks for: its «carriage_label_type» is BIG («большая
// этикетка»), SMALL («маленькая этикетка») or UNSPECIFIED (OZC §6), and the ordinary label, big_label, is the one for a warehouse
// that names no kind (OZC §8). Which label a warehouse really wants when it names none is not sourced: the ordinary one is the
// cautious choice.
func taskTypeOf(labelType string) string {
	if strings.EqualFold(labelType, "SMALL") {
		return taskSmall
	}

	return taskBig
}

// ticket is what AskLabels hands the exchange and ReadLabels reads back: the label tasks Ozon made and the postings they were
// asked for. A task's answer does not name its postings, and the file a task makes holds every posting of the task, so the
// postings are kept with the task numbers. It is a short text of JSON, which holds no key.
type ticket struct {
	Tasks   []int64  `json:"t"`
	Numbers []string `json:"n"`
}

// AskLabels implements marketplace.LabelMaker: /v3/posting/fbs/package-label/create for the postings, which makes a task of
// each type, big_label and small_label (OZC §8, the example answers with both); the ticket holds the tasks of the type the
// warehouse's label type asks for. Ozon recommends to ask 45–60 seconds after the packing: the exchange waits, this does not.
func (a *Adapter) AskLabels(ctx context.Context, conn marketplace.Conn, numbers []string, labelType string) (string, error) {
	if err := mayWrite(conn); err != nil {
		return "", err
	}

	client, err := a.client(conn)
	if err != nil {
		return "", err
	}

	res, err := client.PostingFbsPackageLabelCreateWithResponse(ctx, &ozonapi.PostingFbsPackageLabelCreateParams{},
		ozonapi.PostingV3PostingFbsPackageLabelCreateRequest{PostingNumbers: numbers})
	if err != nil {
		return "", err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return "", writeRefusal(res.StatusCode(), res.Body)
	}

	want, made := taskTypeOf(labelType), ticket{Numbers: slices.Clone(numbers)}

	for _, task := range adapterkit.Value(res.JSON200.Tasks) {
		if task.TaskId != nil && adapterkit.Value(task.TaskType) == want {
			made.Tasks = append(made.Tasks, *task.TaskId)
		}
	}

	if len(made.Tasks) == 0 {
		return "", ownRefusal(codeLabelTaskMissing, "", map[string]any{"type": want})
	}

	raw, err := json.Marshal(made)

	return string(raw), err
}

// ReadLabels implements marketplace.LabelMaker: /v2/posting/fbs/package-label/get for each task of the ticket. While a task is
// pending or in progress the labels are not ready. A task in error fails the whole ticket, in Ozon's words. When every task
// is completed, its file is fetched from the link it gives (files.go) and gives a label to each posting of the ticket that the
// task did not leave out; a posting among «unprinted_postings» is told with Ozon's message and has no label. A file holds the
// labels of every posting of its task, so each posting it covers gets a label of its own that shares the file (the cautious
// choice: one label for one posting whatever the file's pages). No scan text is given: which of the posting's two barcodes a
// scanner reads is not sourced (OZC §8).
func (a *Adapter) ReadLabels(ctx context.Context, conn marketplace.Conn, ticketText string) (marketplace.LabelsState, error) {
	var asked ticket
	if err := json.Unmarshal([]byte(ticketText), &asked); err != nil || len(asked.Tasks) == 0 {
		return marketplace.LabelsState{}, ownRefusal(codeTicketUnreadable, "", nil)
	}

	if err := mayWrite(conn); err != nil {
		return marketplace.LabelsState{}, err
	}

	client, err := a.client(conn)
	if err != nil {
		return marketplace.LabelsState{}, err
	}

	var done []ozonapi.PostingV2PostingFbsPackageLabelGetResponse

	for _, id := range asked.Tasks {
		res, err := client.PostingFbsPackageLabelGetWithResponse(ctx, &ozonapi.PostingFbsPackageLabelGetParams{},
			ozonapi.PostingV2PostingFbsPackageLabelGetRequest{TaskId: id})
		if err != nil {
			return marketplace.LabelsState{}, err
		}

		if res.StatusCode() != http.StatusOK || res.JSON200 == nil || res.JSON200.Status == nil {
			return marketplace.LabelsState{}, writeRefusal(res.StatusCode(), res.Body)
		}

		switch adapterkit.Value(res.JSON200.Status.Code) {
		case taskCompleted:
			done = append(done, *res.JSON200)
		case taskError:
			return marketplace.LabelsState{Failed: true, Words: errorWords(res.JSON200.Error)}, nil
		default: // pending, in_progress, or a word Ozon's file does not list: not made yet
			return marketplace.LabelsState{}, nil
		}
	}

	return a.labelsOf(ctx, conn, asked.Numbers, done)
}

// errorWords is the error of a label task in Ozon's own words: its message, and its code when it gave no message.
func errorWords(e *ozonapi.PostingV2PostingFbsPackageLabelGetResponseError) string {
	if e == nil {
		return ""
	}

	if message := adapterkit.Value(e.Message); message != "" {
		return adapterkit.Cut(message)
	}

	return adapterkit.Cut(adapterkit.Value(e.Code))
}

// labelsOf makes the labels of the postings of the ticket from the tasks that are all completed. A posting a task left out
// of its file is Unmade with Ozon's message unless another task printed it.
func (a *Adapter) labelsOf(ctx context.Context, conn marketplace.Conn, numbers []string, tasks []ozonapi.PostingV2PostingFbsPackageLabelGetResponse) (marketplace.LabelsState, error) {
	state := marketplace.LabelsState{Ready: true, Unmade: map[string]string{}}
	labelled := map[string]bool{}

	for _, task := range tasks {
		left := map[string]string{}

		for _, u := range adapterkit.Value(task.Status.UnprintedPostings) {
			left[adapterkit.Value(u.PostingNumber)] = adapterkit.Cut(adapterkit.Value(u.Message))
		}

		var printed []string

		for _, n := range numbers {
			if _, out := left[n]; !out && !labelled[n] {
				printed = append(printed, n)
			}
		}

		if len(printed) > 0 {
			link := adapterkit.Value(task.FileUrl)
			if link == "" {
				return marketplace.LabelsState{}, ownRefusal(codeFileUnavailable, "", nil)
			}

			file, err := a.fetchFile(ctx, conn, link, kindPDF)
			if err != nil {
				return marketplace.LabelsState{}, err
			}

			for _, n := range printed {
				state.Labels = append(state.Labels, marketplace.Label{Number: n, ContentType: kindPDF.contentType, Data: file})
				labelled[n] = true
			}
		}

		for n, words := range left {
			if !labelled[n] && slices.Contains(numbers, n) {
				state.Unmade[n] = words
			}
		}
	}

	for n := range state.Unmade {
		if labelled[n] {
			delete(state.Unmade, n)
		}
	}

	return state, nil
}
