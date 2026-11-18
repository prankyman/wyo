package ozon

import (
	"bytes"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// The labels of a packed posting (OZC section 8): /v3/posting/fbs/package-label/create makes tasks, /v2/posting/fbs/package-label/get
// reads one until it is completed and gives the link of the file.

// FileHost is the host the label files of the fake live on: Ozon's own example link names ir.ozone.ru (OZC section 8, «https://
// ir.ozone.ru/s3/ord-tmp-12/small_label/ticket-00-0000-0000.pdf»). Which host a real link names is not sourced (OZC section 8): this
// is the host of the example. A test build of the wyo server asks for it at <the fake's address>/ir.ozone.ru/…, as it asks for the
// API, so the fake stands for this host too.
const FileHost = "ir.ozone.ru"

// The two types of a label task, «task_type» (OZC section 8): «big_label — для обычной этикетки; small_label — для маленькой
// этикетки».
const (
	taskBig   = "big_label"
	taskSmall = "small_label"
)

// The status of a task, «status.code» (OZC section 8).
const (
	taskPending    = "pending"
	taskInProgress = "in_progress"
	taskCompleted  = "completed"
	taskError      = "error"
)

// The kinds of file a test may command: a PDF (the default), a file that is not a PDF, a PDF over the 5 MB the neutral API takes.
const (
	fileNotPDF   = "text"
	fileTooLarge = "large"
	filePDF      = "pdf"

	labelFileLimit = 5 << 20
	labelPath      = "/s3/ord-tmp-12/"
	taskNumberBase = 5_000_000
)

// unprinted is a posting a test commanded to stay out of the labels, with Ozon's words for it.
type unprinted struct{ posting, message string }

// labelPlan is what a test commanded for the label tasks of a key: how many readings a task needs, which postings stay unprinted,
// how many of the next requests end in an error, which kind of file and which host the link has.
type labelPlan struct {
	readings    int
	unprinted   []unprinted
	errorTasks  int
	errCode     string
	errMessage  string
	file        string
	foreignHost string
}

// labelTask is a task Ozon made for the postings of one request. The plan of the moment of the request is kept in it, so that a task
// is the same whenever it is read.
type labelTask struct {
	id           int64
	kind         string
	postings     []string
	readingsLeft int
	touched      bool
	failing      bool
	errCode      string
	errMessage   string
	unprinted    []unprinted
	file         string
	foreignHost  string
	link         string
}

// fileFetch is one request for a label file: its path (the file's host is not in it) and whether it carried any header that holds a
// key. The wyo server asks for a label file without one.
type fileFetch struct {
	path    string
	withKey bool
}

// /v3/posting/fbs/package-label/create (PostingFbsPackageLabelCreate): «Создаёт задания на асинхронное формирование этикеток для
// отправлений в статусе «Ожидает отгрузки» — awaiting_deliver.» The answer holds both tasks, big_label and small_label, as Ozon's
// own example does (OZC section 8); the caller takes the one of its warehouse's label type. The fake does not enforce the 45–60
// seconds Ozon recommends between the packing and the request.
func (c *call) labelCreate() {
	var req ozonapi.PostingV3PostingFbsPackageLabelCreateRequest
	if !c.body(&req) {
		return
	}

	if len(req.PostingNumbers) == 0 {
		badRequest(c.w, "posting_numbers is required")

		return
	}

	for _, number := range req.PostingNumbers {
		p, ok := c.postingFor(number)
		if !ok {
			return
		}

		if p.status != statusAwaitingDeliver {
			badRequest(c.w, wordsIncorrectStatus)

			return
		}
	}

	plan, failing := c.st.labels, false
	if plan.errorTasks > 0 {
		c.st.labels.errorTasks--
		failing = true
	}

	tasks := make([]ozonapi.PostingV3PostingFbsPackageLabelCreateResponseTasks, 0, 2)

	for _, kind := range []string{taskBig, taskSmall} {
		c.st.nextTask++

		task := &labelTask{
			id: taskNumberBase + c.st.nextTask, kind: kind, postings: slices.Clone(req.PostingNumbers), readingsLeft: plan.readings,
			failing: failing, errCode: plan.errCode, errMessage: plan.errMessage, unprinted: slices.Clone(plan.unprinted),
			file: plan.file, foreignHost: plan.foreignHost,
		}

		c.st.tasks = append(c.st.tasks, task)
		tasks = append(tasks, ozonapi.PostingV3PostingFbsPackageLabelCreateResponseTasks{TaskId: new(task.id), TaskType: new(kind)})
	}

	c.json(http.StatusOK, ozonapi.PostingV3PostingFbsPackageLabelCreateResponse{Tasks: &tasks})
}

// printed are the postings of a task that get a label: the ones no command left out.
func (t *labelTask) printed() []string {
	return slices.DeleteFunc(slices.Clone(t.postings), func(number string) bool {
		return slices.ContainsFunc(t.unprinted, func(u unprinted) bool { return u.posting == number })
	})
}

// /v2/posting/fbs/package-label/get (PostingFbsPackageLabelGet): the status of the task: «pending — задание в очереди; in_progress —
// формируются; completed — файл с этикетками готов; error — ошибка при создании файла», with the link of the file when it is
// completed. Each reading of a task that is not made yet uses up one of the readings the plan gave it: the first answers pending
// and the others in_progress. A completed task names the postings that got no label in status.unprinted_postings, and has no file
// when none got one.
func (c *call) labelGet() {
	var req ozonapi.PostingV2PostingFbsPackageLabelGetRequest
	if !c.body(&req) {
		return
	}

	task := c.st.taskOf(req.TaskId)
	if task == nil {
		writeRefusal(c.w, refusal{http.StatusNotFound, codeNotFound, "TASK_NOT_FOUND"})

		return
	}

	status := ozonapi.PostingV2PostingFbsPackageLabelGetResponseStatus{
		PostingsCount: count(len(task.postings)), PrintedPostingsCount: count(0),
		UnprintedPostings: &[]ozonapi.PostingV2PostingFbsPackageLabelGetResponseStatusUnprintedPostings{},
	}

	out := ozonapi.PostingV2PostingFbsPackageLabelGetResponse{Status: &status}

	switch {
	case task.readingsLeft > 0:
		status.Code = new(taskInProgress)
		if !task.touched {
			status.Code = new(taskPending)
		}

		task.readingsLeft--
		task.touched = true
	case task.failing:
		status.Code = new(taskError)
		out.Error = &ozonapi.PostingV2PostingFbsPackageLabelGetResponseError{Code: new(task.errCode), Message: new(task.errMessage)}
	default:
		status.Code = new(taskCompleted)
		c.completeTask(task, &status, &out)
	}

	c.json(http.StatusOK, out)
}

func (st *state) taskOf(id int64) *labelTask {
	for _, t := range st.tasks {
		if t.id == id {
			return t
		}
	}

	return nil
}

// completeTask fills the answer of a completed task: what was printed and what was not, and the link of the file. The link is made
// once, with the file on the first reading that finds the task completed; a link on a foreign host has no file here.
func (c *call) completeTask(task *labelTask, status *ozonapi.PostingV2PostingFbsPackageLabelGetResponseStatus, out *ozonapi.PostingV2PostingFbsPackageLabelGetResponse) {
	printed := task.printed()
	status.PrintedPostingsCount = count(len(printed))

	list := []ozonapi.PostingV2PostingFbsPackageLabelGetResponseStatusUnprintedPostings{}

	for _, number := range task.postings {
		if i := slices.IndexFunc(task.unprinted, func(u unprinted) bool { return u.posting == number }); i >= 0 {
			list = append(list, ozonapi.PostingV2PostingFbsPackageLabelGetResponseStatusUnprintedPostings{
				PostingNumber: new(number), Message: new(task.unprinted[i].message),
			})
		}
	}

	status.UnprintedPostings = &list

	if len(printed) == 0 {
		return
	}

	if task.link == "" {
		task.link = c.srv.publishLabel(c.srv.kit.KeyOf(c.r), task, printed)
	}

	out.FileUrl = new(task.link)
}

// publishLabel makes the file of a task and the link to it. The file is kept by the server, not by the key's state, because the
// wyo server asks for it without a key (OZC section 8: whether the link needs the key is not sourced; the fake gives it to a
// request with none). The owner's key is kept beside it only to list the fetch for that key.
func (s *Server) publishLabel(key string, task *labelTask, printed []string) string {
	s.filesMu.Lock()
	defer s.filesMu.Unlock()

	s.fileSeq++

	name := fmt.Sprintf("ticket-00-%04d-0000.pdf", s.fileSeq)
	host := FileHost

	if task.foreignHost != "" {
		host = task.foreignHost
	} else {
		contentType, body := labelFile(task.kind, printed, task.file)
		s.files[labelPath+task.kind+"/"+name] = labelFileEntry{key: key, contentType: contentType, body: body}
	}

	return "https://" + host + labelPath + task.kind + "/" + name
}

type labelFileEntry struct {
	key         string
	contentType string
	body        []byte
}

// labelFile is the file of a label task: a PDF that names its type and its postings, a file that is not a PDF, or a PDF over the
// limit of the neutral API.
func labelFile(kind string, postings []string, file string) (string, []byte) {
	if file == fileNotPDF {
		return "text/html", []byte("<html><body>not a label</body></html>")
	}

	head := fmt.Sprintf("%%PDF-1.4\n%% a label file made up by the fake Ozon\n%% type: %s\n%% postings: %s\n%%%%EOF\n", kind, strings.Join(postings, " "))

	if file == fileTooLarge {
		return "application/pdf", append([]byte(head), bytes.Repeat([]byte{'0'}, labelFileLimit+1-len(head))...)
	}

	return "application/pdf", []byte(head)
}

// withFiles serves the label files of the fake: a request whose path begins with the file host. A file is asked for without a key,
// so it never meets the kit, which refuses a call without one.
func (s *Server) withFiles(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest, found := strings.CutPrefix(r.URL.Path, "/"+FileHost+"/")
		if !found {
			next.ServeHTTP(w, r)

			return
		}

		s.serveLabelFile(w, r, "/"+rest)
	})
}

func (s *Server) serveLabelFile(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "a file is only read", http.StatusMethodNotAllowed)

		return
	}

	s.filesMu.Lock()
	entry, found := s.files[path]
	s.filesMu.Unlock()

	if !found {
		http.NotFound(w, r)

		return
	}

	withKey := r.Header.Get("Api-Key") != "" || r.Header.Get("Client-Id") != "" || r.Header.Get("Authorization") != ""

	s.kit.Use(entry.key, func(st *state) { st.fetches = append(st.fetches, fileFetch{path: path, withKey: withKey}) })

	w.Header().Set("Content-Type", entry.contentType)
	_, _ = w.Write(entry.body)
}
