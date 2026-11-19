package ozon

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// The control calls that drive the writing methods, besides the ones in control.go. Each takes a JSON body, refuses a field it
// does not know, and answers {"status": "ok"} or 400 with the reason.
//
//	POST /__control/postings/exemplar-check   posting_number, in_process_readings (how many readings of the status find the check
//	                                          still going on after each set), failed_sets (how many of the next sets end with every
//	                                          mark failed, without a reason), error_sets and error_codes (how many of the next sets
//	                                          end with every mark failed and these error codes); replaces the posting's plan
//	POST /__control/postings/ship             posting_number, readings (how many readings of /v3/posting/fbs/get pass before a report
//	                                          of the packing takes effect; 0 is at once), failures (how many of the next reports end
//	                                          in the substatus ship_failed instead); replaces the posting's plan
//	POST /__control/labels                    readings (how many readings of a label task find it not made yet), unprinted [{posting_number,
//	                                          message}] (postings that stay out of the labels, with Ozon's words for it), error_tasks and
//	                                          error {code, message} (how many of the next requests make tasks that end in an error),
//	                                          file ("pdf", "text" for a file that is not a PDF, "large" for a PDF over 5 MB) and
//	                                          foreign_host (a host outside ozone.ru for the link); replaces the key's plan, which holds
//	                                          for the tasks made after it
//	POST /__control/carriages                 not_ready (how many readings of the delivery list find that no shipment can be made yet),
//	                                          refused_postings [{posting_number, error}] (postings that /v1/carriage/set-postings
//	                                          refuses, with the error), document_readings (how many readings of the documents'
//	                                          status find them in process after the approval), document_statuses (the statuses "error"
//	                                          or "cancelled" the next readings answer before ready), removed_from_act (postings that
//	                                          fall out of the act when it is ready) and documents_as_object (the shipping list and
//	                                          the barcode are answered as an object holding the file in base64, not as the file),
//	                                          late_postings (postings that fall into a shipment right after the next rewrite of its
//	                                          list, as a posting that became ready would: each once, and only a packed posting of
//	                                          the shipment's warehouse that no shipment holds), late_on_reads (postings that fall into
//	                                          a shipment still new right before a reading of it, /v1/carriage/get, one posting for each
//	                                          reading, in order, under the same conditions: a posting that became ready after the list
//	                                          was written and read back); replaces the key's plan, whose
//	                                          documents part holds for shipments approved after it
//	GET  /__control/label-files               every request for a label file: its path and with_key, whether it carried a key header
//	GET  /__control/exemplars?posting_number  the posting's exemplars with the codes they carry, as create-or-get answers them
func (s *Server) writingControlRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /__control/postings/exemplar-check", controlCall(s, (*state).planCheck))
	mux.HandleFunc("POST /__control/postings/ship", controlCall(s, (*state).planShip))
	mux.HandleFunc("POST /__control/labels", controlCall(s, (*state).planLabels))
	mux.HandleFunc("POST /__control/carriages", controlCall(s, (*state).planCarriages))
	mux.HandleFunc("GET /__control/label-files", s.controlLabelFiles)
	mux.HandleFunc("GET /__control/exemplars", s.controlExemplars)
}

type exemplarCheckRequest struct {
	PostingNumber     string   `json:"posting_number"`
	InProcessReadings int      `json:"in_process_readings"`
	FailedSets        int      `json:"failed_sets"`
	ErrorSets         int      `json:"error_sets"`
	ErrorCodes        []string `json:"error_codes"`
}

func (st *state) planCheck(in exemplarCheckRequest) error {
	p := st.postingOf(in.PostingNumber)

	switch {
	case p == nil:
		return fmt.Errorf("the posting %q does not exist", in.PostingNumber)
	case in.InProcessReadings < 0 || in.FailedSets < 0 || in.ErrorSets < 0:
		return errors.New("in_process_readings, failed_sets and error_sets cannot be negative")
	case in.ErrorSets > 0 && len(in.ErrorCodes) == 0:
		return errors.New("error_sets needs the error_codes to give")
	}

	p.plan = checkPlan{
		inProcessReadings: in.InProcessReadings, failedSets: in.FailedSets, errorSets: in.ErrorSets, errorCodes: slices.Clone(in.ErrorCodes),
	}

	return nil
}

type shipPlanRequest struct {
	PostingNumber string `json:"posting_number"`
	Readings      int    `json:"readings"`
	Failures      int    `json:"failures"`
}

func (st *state) planShip(in shipPlanRequest) error {
	p := st.postingOf(in.PostingNumber)

	switch {
	case p == nil:
		return fmt.Errorf("the posting %q does not exist", in.PostingNumber)
	case in.Readings < 0 || in.Failures < 0:
		return errors.New("readings and failures cannot be negative")
	}

	p.shipPlan = shipPlan{readings: in.Readings, failures: in.Failures}

	return nil
}

type carriagePlanRequest struct {
	NotReady        int `json:"not_ready"`
	RefusedPostings []struct {
		PostingNumber string `json:"posting_number"`
		Error         string `json:"error"`
	} `json:"refused_postings"`
	DocumentReadings  int      `json:"document_readings"`
	DocumentStatuses  []string `json:"document_statuses"`
	RemovedFromAct    []string `json:"removed_from_act"`
	DocumentsAsObject bool     `json:"documents_as_object"`
	LatePostings      []string `json:"late_postings"`
	LateOnReads       []string `json:"late_on_reads"`
}

func (st *state) planCarriages(in carriagePlanRequest) error {
	if in.NotReady < 0 || in.DocumentReadings < 0 {
		return errors.New("not_ready and document_readings cannot be negative")
	}

	for _, status := range in.DocumentStatuses {
		if !slices.Contains(documentStatuses, status) {
			return fmt.Errorf("document_statuses holds %q, got %q", documentStatuses, status)
		}
	}

	plan := carriagePlan{
		notReady: in.NotReady, docReadings: in.DocumentReadings, docStatuses: slices.Clone(in.DocumentStatuses),
		removedFrom: slices.Clone(in.RemovedFromAct), docsAsObject: in.DocumentsAsObject, late: slices.Clone(in.LatePostings),
		lateOnReads: slices.Clone(in.LateOnReads),
	}

	for _, r := range in.RefusedPostings {
		if r.PostingNumber == "" {
			return errors.New("a refused posting needs its posting_number")
		}

		plan.refused = append(plan.refused, refusedPosting{posting: r.PostingNumber, words: r.Error})
	}

	st.carr = plan

	return nil
}

type labelPlanRequest struct {
	Readings  int `json:"readings"`
	Unprinted []struct {
		PostingNumber string `json:"posting_number"`
		Message       string `json:"message"`
	} `json:"unprinted"`
	ErrorTasks int `json:"error_tasks"`
	Error      *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	File        string `json:"file"`
	ForeignHost string `json:"foreign_host"`
}

func (st *state) planLabels(in labelPlanRequest) error {
	switch {
	case in.Readings < 0 || in.ErrorTasks < 0:
		return errors.New("readings and error_tasks cannot be negative")
	case in.ErrorTasks > 0 && in.Error == nil:
		return errors.New("error_tasks needs the error to give")
	case !slices.Contains([]string{"", filePDF, fileNotPDF, fileTooLarge}, in.File):
		return fmt.Errorf("file is one of %q, %q or %q, got %q", filePDF, fileNotPDF, fileTooLarge, in.File)
	case strings.ContainsAny(in.ForeignHost, "/: "):
		return fmt.Errorf("foreign_host is a host name only, got %q", in.ForeignHost)
	}

	plan := labelPlan{readings: in.Readings, errorTasks: in.ErrorTasks, file: in.File, foreignHost: in.ForeignHost}

	if in.Error != nil {
		plan.errCode, plan.errMessage = in.Error.Code, in.Error.Message
	}

	for _, u := range in.Unprinted {
		if u.PostingNumber == "" {
			return errors.New("an unprinted posting needs its posting_number")
		}

		plan.unprinted = append(plan.unprinted, unprinted{posting: u.PostingNumber, message: u.Message})
	}

	st.labels = plan

	return nil
}

// controlLabelFiles lists the requests for label files, which the call list does not (they carry no key, so the kit has no key to
// list them under): a test learns from it that the wyo server asked for a file, and that it asked without a key.
func (s *Server) controlLabelFiles(w http.ResponseWriter, r *http.Request) {
	type fetch struct {
		Path    string `json:"path"`
		WithKey bool   `json:"with_key"`
	}

	files := []fetch{}

	s.kit.Use(s.kit.KeyOf(r), func(st *state) {
		for _, f := range st.fetches {
			files = append(files, fetch{Path: f.path, WithKey: f.withKey})
		}
	})

	fakekit.JSON(w, http.StatusOK, map[string]any{"files": files})
}

// controlExemplars shows a test the codes the fake holds: the call list never has them (redactBody).
func (s *Server) controlExemplars(w http.ResponseWriter, r *http.Request) {
	number := r.URL.Query().Get("posting_number")

	var (
		out    any
		status = http.StatusOK
	)

	s.kit.Use(s.kit.KeyOf(r), func(st *state) {
		p := st.postingOf(number)
		if p == nil {
			status, out = http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("the posting %q does not exist", number)}

			return
		}

		out = st.exemplarsWire(p)
	})

	fakekit.JSON(w, status, out)
}
