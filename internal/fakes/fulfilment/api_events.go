package fulfilment

import (
	"net/http"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

// The events and the hand-overs of the neutral API: reading the events, and the calls about a hand-over and its documents.

// eventTypes are the events the contract lists.
var eventTypes = map[string]bool{
	"order.accepted": true, "order.held": true, "order.reserved": true, "order.picking_started": true, "order.picked": true,
	"order.packing_finished": true, "order.label_attached": true, "order.parcel_closed": true, "order.packed": true,
	"order.reopened": true, "order.reduced": true, "order.change_waits": true, "order.cancelled": true,
	"order.handed_over": true, "order.courier_code_entered": true, "handover.requested": true, "handover.request_withdrawn": true, "handover.completed": true,
	"ping": true,
}

func (a *api) EventList(w http.ResponseWriter, r *http.Request, params fcapi.EventListParams) {
	a.act(w, r, false, func(st *state) result {
		var found issues

		after, size := 0, defaultPage

		if params.After != nil {
			if *params.After < 0 {
				found.add("after", "out_of_range")
			} else {
				after = *params.After
			}
		}

		if params.Limit != nil {
			if *params.Limit < 1 || *params.Limit > maxPage {
				found.add("limit", "out_of_range")
			} else {
				size = *params.Limit
			}
		}

		wanted := map[string]bool{}

		if params.Types != nil {
			for _, t := range strings.Split(*params.Types, ",") {
				if !eventTypes[t] {
					found.add("types", "unknown_type")

					break
				}

				wanted[t] = true
			}
		}

		if len(found) > 0 {
			return badRequest(found)
		}

		events := []fcapi.Event{}

		var next *int

		for i := after; i < len(st.events); i++ {
			e := st.events[i]
			if len(wanted) > 0 && !wanted[e.typ] {
				continue
			}

			if len(events) == size {
				last := events[len(events)-1].Sequence
				next = &last

				break
			}

			events = append(events, st.renderEvent(e))
		}

		return ok(http.StatusOK, map[string]any{"events": events, "next_after": next})
	})
}

func (a *api) HandoverList(w http.ResponseWriter, r *http.Request, params fcapi.HandoverListParams) {
	a.act(w, r, false, func(st *state) result {
		offset, size, found := pageOf(params.Limit, params.Cursor)

		if params.State != nil && *params.State != fcapi.HandoverListParamsStateOpen {
			found.add("state", "invalid")
		}

		if len(found) > 0 {
			return badRequest(found)
		}

		var matching []*handover

		for _, h := range st.hoList {
			if params.State == nil || h.state != fcapi.HandoverStateCompleted {
				matching = append(matching, h)
			}
		}

		from, to, next := window(len(matching), offset, size)
		out := make([]fcapi.Handover, 0, to-from)

		for _, h := range matching[from:to] {
			out = append(out, st.renderHandover(h))
		}

		return ok(http.StatusOK, map[string]any{"handovers": out, "next_cursor": next})
	})
}

func (a *api) HandoverGet(w http.ResponseWriter, r *http.Request, id string) {
	a.act(w, r, false, func(st *state) result {
		h, found := st.handovers[id]
		if !found {
			return notFound()
		}

		return ok(http.StatusOK, map[string]any{"handover": st.renderHandover(h)})
	})
}

func (a *api) HandoverDocuments(w http.ResponseWriter, r *http.Request, id string) {
	mutate(a, w, r, maxFileBody, validateDocuments, func(st *state, in fcapi.HandoverDocuments) result {
		h, found := st.handovers[id]
		if !found {
			return notFound()
		}

		if h.state != fcapi.HandoverStateDocumentsRequested && h.state != fcapi.HandoverStateDocumentsReady {
			return fail(http.StatusConflict, fcapi.ErrorErrorCodeConflict)
		}

		h.documents = make([]handoverDocument, len(in.Documents))
		for i, d := range in.Documents {
			h.documents[i] = handoverDocument{
				kind: fcapi.HandoverDocumentsKind(d.Kind), title: d.Title, contentType: string(d.ContentType), scanCode: d.ScanCode, data: d.Data,
			}
		}

		external := in.ExternalId
		h.externalID = &external
		h.state = fcapi.HandoverStateDocumentsReady

		return ok(http.StatusOK, map[string]any{"handover": st.renderHandover(h)})
	})
}

func (a *api) HandoverProblemSet(w http.ResponseWriter, r *http.Request, id string) {
	mutate(a, w, r, maxJSONBody, validateProblem, func(st *state, in fcapi.ProblemIn) result {
		h, found := st.handovers[id]
		if !found {
			return notFound()
		}

		if h.state == fcapi.HandoverStateCompleted {
			return fail(http.StatusConflict, fcapi.ErrorErrorCodeConflict)
		}

		h.problem = &fcapi.Problem{At: st.now().Format(time.RFC3339), Code: in.Code, Message: in.Message}

		return ok(http.StatusOK, map[string]any{"handover": st.renderHandover(h)})
	})
}

func (a *api) HandoverProblemClear(w http.ResponseWriter, r *http.Request, id string) {
	a.act(w, r, true, func(st *state) result {
		h, found := st.handovers[id]
		if !found {
			return notFound()
		}

		h.problem = nil

		return ok(http.StatusOK, map[string]any{"handover": st.renderHandover(h)})
	})
}
