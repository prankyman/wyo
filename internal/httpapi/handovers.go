package httpapi

import (
	"net/http"
	"slices"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/prankyman/wyo/internal/handover"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// handoversShown is how many hand-overs the list shows: the newest ones.
const handoversShown = 200

// problemBody is a problem as the interface words it, or nil when there is none.
func problemBody(p store.Problem) *gen.Problem {
	if p.Code == "" {
		return nil
	}

	body := &gen.Problem{Code: p.Code, At: p.At, MarketplaceCode: text(p.MpCode), Words: text(p.Words)}
	if len(p.Params) > 0 {
		params := p.Params
		body.Params = &params
	}

	return body
}

// handoverBody is a hand-over of the list.
func (a *api) handoverBody(r *http.Request, h store.Handover) (gen.Handover, []store.HandoverOrder, error) {
	q := a.deps.Store.Read()

	orders, err := q.HandoverOrders(r.Context(), h.ID)
	if err != nil {
		return gen.Handover{}, nil, err
	}

	supplies, err := q.HandoverSupplies(r.Context(), h.ID)
	if err != nil {
		return gen.Handover{}, nil, err
	}

	day, _ := time.Parse("2006-01-02", h.Day)
	body := gen.Handover{
		Id: h.ID, Number: h.Number, Day: openapi_types.Date{Time: day}, State: gen.HandoverState(h.State), Step: gen.HandoverStep(h.Step),
		Batches: []string{}, Orders: len(orders), Boxes: int(h.Boxes), Problem: problemBody(h.Problem), RequestedAt: h.RequestedAt,
		UpdatedAt: moment(h.UpdatedAt),
	}

	for _, s := range supplies {
		body.Batches = append(body.Batches, s.Number)

		if s.Kind != "" {
			body.BatchKind = text(s.Kind)
		}
	}

	if len(orders) > 0 {
		if acc, err := q.Account(r.Context(), orders[0].AccountID); err == nil {
			body.Marketplace = text(acc.Marketplace)
		}
	}

	return body, orders, nil
}

// ListHandovers answers GET /api/v1/handovers.
func (a *api) ListHandovers(w http.ResponseWriter, r *http.Request) {
	list, err := a.deps.Store.Read().Handovers(r.Context(), handoversShown)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	body := gen.HandoverList{Handovers: make([]gen.Handover, 0, len(list))}

	for _, h := range list {
		item, _, err := a.handoverBody(r, h)
		if err != nil {
			a.fail(w, r, err)

			return
		}

		body.Handovers = append(body.Handovers, item)
	}

	writeJSON(w, http.StatusOK, body)
}

// stepStates are the steps of a hand-over its marketplace's adapter needs, as the strip shows them. A step before the
// hand-over's current one in the order the steps run is done, whether the adapter needs it or not.
func stepStates(h store.Handover, d marketplace.Description) []gen.HandoverStepState {
	now := slices.Index(marketplace.HandoverStepsAll, h.Step)
	declared := declaredSteps(d)
	out := make([]gen.HandoverStepState, 0, len(declared))

	for _, step := range declared {
		pos := slices.Index(marketplace.HandoverStepsAll, string(step))
		state := "todo"

		switch {
		case h.State == handover.StateDocumentsSent || h.State == handover.StateCompleted || pos < now:
			state = "done"
		case pos == now && h.State == handover.StateProblem:
			state = "failed"
		case pos == now && h.State == handover.StateWaiting:
			state = "waiting"
		case pos == now:
			state = "now"
		}

		out = append(out, gen.HandoverStepState{Step: step, State: gen.HandoverStepStateState(state)})
	}

	return out
}

// descriptionOf is the description of the adapter whose code is given; an unknown code, or none (a hand-over whose orders are
// not known yet), describes an adapter that needs every step.
func (a *api) descriptionOf(code string) marketplace.Description {
	if adapter, ok := a.deps.Marketplaces.Get(code); ok {
		return adapter.Describe()
	}

	return marketplace.Description{}
}

// writeHandover answers with one hand-over's details.
func (a *api) writeHandover(w http.ResponseWriter, r *http.Request, id int64, status int) {
	h, err := a.deps.Store.Read().Handover(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	item, orders, err := a.handoverBody(r, h)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	body := gen.HandoverDetails{
		Handover: item, Steps: stepStates(h, a.descriptionOf(valueOf(item.Marketplace))), Closed: h.Closed,
		Retryable: h.State == handover.StateWaiting || h.State == handover.StateProblem,
		NeedsShippingPoint: h.State == handover.StateWaiting && (h.Problem.Code == "shipping_point_needed" ||
			h.Problem.Code == "shipping_point_gone"),
	}

	for _, o := range orders {
		body.OrderList = append(body.OrderList, struct {
			Id     int64  `json:"id"`
			Number string `json:"number"`
		}{Id: o.OrderID, Number: o.Number})
	}

	if body.OrderList == nil {
		body.OrderList = body.OrderList[:0]
	}

	writeJSON(w, status, body)
}

// GetHandover answers GET /api/v1/handovers/{id}.
func (a *api) GetHandover(w http.ResponseWriter, r *http.Request, id int64) {
	a.writeHandover(w, r, id, http.StatusOK)
}

// RetryHandover answers POST /api/v1/handovers/{id}/retry: «Повторить».
func (a *api) RetryHandover(w http.ResponseWriter, r *http.Request, id int64) {
	if err := a.deps.Exchange.RetryHandover(r.Context(), id); err != nil {
		a.fail(w, r, err)

		return
	}

	a.writeHandover(w, r, id, http.StatusAccepted)
}

// GetHandoverShippingPoints answers GET /api/v1/handovers/{id}/shipping-point.
func (a *api) GetHandoverShippingPoints(w http.ResponseWriter, r *http.Request, id int64) {
	points, chosen, err := a.deps.Exchange.Handovers().Points(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	body := gen.ShippingPointChoice{Points: make([]gen.ShippingPoint, 0, len(points)), Chosen: text(chosen)}
	for _, p := range points {
		body.Points = append(body.Points, gen.ShippingPoint{Id: p.ID, Name: p.Name, Address: text(p.Address)})
	}

	writeJSON(w, http.StatusOK, body)
}

// PutHandoverShippingPoint answers PUT /api/v1/handovers/{id}/shipping-point.
func (a *api) PutHandoverShippingPoint(w http.ResponseWriter, r *http.Request, id int64) {
	var in gen.ShippingPointPick
	if !decode(w, r, &in) {
		return
	}

	if err := a.deps.Exchange.PickShippingPoint(r.Context(), id, in.Point); err != nil {
		a.fail(w, r, err)

		return
	}

	a.writeHandover(w, r, id, http.StatusAccepted)
}
