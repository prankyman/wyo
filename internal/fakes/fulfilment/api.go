package fulfilment

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
	"github.com/prankyman/wyo/internal/refusal"
)

// api implements the generated server of the neutral API. Every call runs the same way: the account's gate (maintenance,
// switched off, read-only), the body (size, then shape), then the call's own rules on the key's state.
type api struct{ s *Server }

var _ fcapi.ServerInterface = (*api)(nil)

// result is what a call answers.
type result struct {
	status int
	body   any
	fail   *refusal.Error
}

func ok(status int, body any) result { return result{status: status, body: body} }

func fail(status int, code fcapi.ErrorErrorCode, found ...refusal.Issue) result {
	return result{status: status, fail: &refusal.Error{Code: string(code), Issues: found}}
}

func notFound() result { return fail(http.StatusNotFound, fcapi.ErrorErrorCodeNotFound) }

func badRequest(found issues) result {
	return fail(http.StatusBadRequest, fcapi.ErrorErrorCodeBadRequest, found...)
}

func (res result) write(w http.ResponseWriter) {
	if res.fail != nil {
		refusal.Write(w, res.status, *res.fail)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.status)

	_ = json.NewEncoder(w).Encode(res.body)
}

// badParameter answers a query or path parameter the generated wrapper could not read.
func badParameter(w http.ResponseWriter, _ *http.Request, err error) {
	var found issues

	var invalid *fcapi.InvalidParamFormatError

	var required *fcapi.RequiredParamError

	switch {
	case errors.As(err, &invalid):
		found.add(invalid.ParamName, "invalid_format")
	case errors.As(err, &required):
		found.add(required.ParamName, "required")
	default:
		found.add("", "invalid_value")
	}

	badRequest(found).write(w)
}

// gate refuses a call the account does not allow and says whether to go on. The order is the contract's: no key (the kit
// already answered), maintenance, the work switched off, the API switched off, a read-only key on a write.
func (s *Server) gate(w http.ResponseWriter, r *http.Request, write bool) (string, bool) {
	key := s.kit.KeyOf(r)

	var refused *result

	s.kit.Use(key, func(st *state) {
		switch {
		case st.maintenance:
			res := fail(http.StatusServiceUnavailable, fcapi.ErrorErrorCodeUnavailable)
			refused = &res
		case st.forbidden:
			res := fail(http.StatusForbidden, fcapi.ErrorErrorCodeForbidden)
			refused = &res
		case st.apiOff:
			res := fail(http.StatusForbidden, fcapi.ErrorErrorCodeApiOff)
			refused = &res
		case write && st.readOnly:
			res := fail(http.StatusForbidden, fcapi.ErrorErrorCodeReadOnlyKey)
			refused = &res
		}
	})

	if refused != nil {
		refused.write(w)

		return key, false
	}

	return key, true
}

// act runs a call that has no body.
func (a *api) act(w http.ResponseWriter, r *http.Request, write bool, fn func(st *state) result) {
	key, proceed := a.s.gate(w, r, write)
	if !proceed {
		return
	}

	var res result

	a.s.with(key, func(st *state) { res = fn(st) })
	res.write(w)
}

// mutate runs a call that has a JSON body: its size first, then its shape, then the call's own rules.
func mutate[T any](a *api, w http.ResponseWriter, r *http.Request, limit int64, check func(*T) issues, fn func(st *state, in T) result) {
	key, proceed := a.s.gate(w, r, true)
	if !proceed {
		return
	}

	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		fail(http.StatusRequestEntityTooLarge, fcapi.ErrorErrorCodeTooLarge).write(w)

		return
	}

	var in T

	if found := decodeStrict(raw, &in); len(found) > 0 {
		badRequest(found).write(w)

		return
	}

	if check != nil {
		if found := check(&in); len(found) > 0 {
			badRequest(found).write(w)

			return
		}
	}

	var res result

	a.s.with(key, func(st *state) { res = fn(st, in) })
	res.write(w)
}

func (a *api) Account(w http.ResponseWriter, r *http.Request) {
	a.act(w, r, false, func(st *state) result {
		access := fcapi.ReadWrite
		if st.readOnly {
			access = fcapi.ReadOnly
		}

		return ok(http.StatusOK, fcapi.Account{
			Api:          fcapi.AccountApi{Enabled: !st.apiOff},
			Key:          fcapi.AccountKey{Access: access, Name: keyName},
			Limits:       fcapi.AccountLimits{BodyBytes: maxJSONBody, FileBodyBytes: maxFileBody, MaxPageSize: maxPage, PageSize: defaultPage, RequestsPerMinute: 300, RetentionDays: 30},
			Marketplaces: append([]fcapi.AccountMarketplace{}, st.marketplaces...),
			Seller:       fcapi.AccountSeller{Name: st.sellerName},
			ServerTime:   st.now(),
		})
	})
}

func (a *api) Warehouses(w http.ResponseWriter, r *http.Request) {
	a.act(w, r, false, func(st *state) result {
		return ok(http.StatusOK, map[string]any{"warehouses": append([]fcapi.Warehouse{}, st.warehouses...)})
	})
}

func (a *api) OrderCreate(w http.ResponseWriter, r *http.Request) {
	mutate(a, w, r, maxJSONBody, validateOrderIn, func(st *state, in fcapi.OrderIn) result {
		if in.Marketplace != nil && !st.enabled(*in.Marketplace) {
			return fail(http.StatusForbidden, fcapi.ErrorErrorCodeMarketplaceNotEnabled)
		}

		key := marketplaceKey(in.Marketplace, in.ExternalId)
		sent := fingerprint(in.Lines)

		if existing, found := st.byExternal[key]; found {
			if existing.fingerprint != sent {
				return fail(http.StatusConflict, fcapi.ErrorErrorCodeExternalIdReused)
			}

			return ok(http.StatusOK, map[string]any{"state": "accepted", "duplicate": true, "order": st.renderOrder(existing)})
		}

		if existing, found := st.heldByExt[key]; found && existing.waiting() {
			if existing.fingerprint != sent {
				return fail(http.StatusConflict, fcapi.ErrorErrorCodeExternalIdReused)
			}

			return ok(http.StatusAccepted, map[string]any{"state": "held", "held": st.renderHeld(existing)})
		}

		reasons, why, warehouse := st.evaluate(in)
		if len(reasons) > 0 {
			return ok(http.StatusAccepted, map[string]any{"state": "held", "held": st.renderHeld(st.hold(in, reasons, why))})
		}

		return ok(http.StatusCreated, map[string]any{"state": "accepted", "order": st.renderOrder(st.accept(in, fcapi.Api, warehouse))})
	})
}

func (a *api) OrderGet(w http.ResponseWriter, r *http.Request, id string) {
	a.act(w, r, false, func(st *state) result {
		o, found := st.orders[id]
		if !found {
			return notFound()
		}

		return ok(http.StatusOK, map[string]any{"order": st.renderOrder(o)})
	})
}

// pageOf reads limit and cursor. The cursor is the number of items already read, written with a letter in front so that
// a sender treats it as the opaque text it is.
func pageOf(limit *int, cursor *string) (offset, size int, found issues) {
	size = defaultPage

	if limit != nil {
		if *limit < 1 || *limit > maxPage {
			found.add("limit", "out_of_range")
		} else {
			size = *limit
		}
	}

	if cursor != nil {
		n, err := strconv.Atoi(strings.TrimPrefix(*cursor, "c"))
		if err != nil || !strings.HasPrefix(*cursor, "c") || n < 0 {
			found.add("cursor", "invalid")
		} else {
			offset = n
		}
	}

	return offset, size, found
}

// window cuts one page out of a list of n items and says where the next page starts, or nil.
func window(n, offset, size int) (from, to int, next *string) {
	from = min(offset, n)
	to = min(from+size, n)

	if to < n {
		cursor := "c" + strconv.Itoa(to)
		next = &cursor
	}

	return from, to, next
}

func (a *api) OrderList(w http.ResponseWriter, r *http.Request, params fcapi.OrderListParams) {
	a.act(w, r, false, func(st *state) result {
		offset, size, found := pageOf(params.Limit, params.Cursor)

		if params.Status != nil {
			switch *params.Status {
			case fcapi.OrderListParamsStatusAccepted, fcapi.OrderListParamsStatusCancelled, fcapi.OrderListParamsStatusPacked,
				fcapi.OrderListParamsStatusPacking, fcapi.OrderListParamsStatusPicking, fcapi.OrderListParamsStatusReserved,
				fcapi.OrderListParamsStatusShipped, fcapi.OrderListParamsStatusWaitingStock:
			default:
				found.add("status", "invalid")
			}
		}

		if len(found) > 0 {
			return badRequest(found)
		}

		var matching []*order

		for i := len(st.orderList) - 1; i >= 0; i-- {
			o := st.orderList[i]
			if matchesList(o, params) {
				matching = append(matching, o)
			}
		}

		sortNewestChangeFirst(matching)

		from, to, next := window(len(matching), offset, size)
		orders := make([]fcapi.Order, 0, to-from)

		for _, o := range matching[from:to] {
			orders = append(orders, st.renderOrder(o))
		}

		return ok(http.StatusOK, map[string]any{"orders": orders, "next_cursor": next})
	})
}

func matchesList(o *order, p fcapi.OrderListParams) bool {
	switch {
	case p.UpdatedSince != nil && o.updatedAt.Before(*p.UpdatedSince):
		return false
	case p.Status != nil && string(o.status) != string(*p.Status):
		return false
	case p.Marketplace != nil && *p.Marketplace == "none" && o.marketplace != nil:
		return false
	case p.Marketplace != nil && *p.Marketplace != "none" && str(o.marketplace) != *p.Marketplace:
		return false
	case p.ExternalId != nil && o.externalID != *p.ExternalId:
		return false
	}

	return true
}

func (a *api) OrderPatch(w http.ResponseWriter, r *http.Request, id string) {
	mutate(a, w, r, maxJSONBody, validatePatch, func(st *state, in patchBody) result {
		o, refused := st.changeable(id)
		if refused != nil {
			return *refused
		}

		switch st.changeAnswer(o) {
		case changeHandedOver:
			return fail(http.StatusConflict, fcapi.ErrorErrorCodeHandedOver)
		case changeCancelled:
			return fail(http.StatusConflict, fcapi.ErrorErrorCodeConflict)
		case changeNow, changeWaits, changeNeedsPerson:
		}

		if in.ShipBy != nil {
			o.shipBy = in.ShipBy.UTC()
		}

		if note, present, _ := in.note(); present {
			o.packingNote = note
		}

		o.updatedAt = st.now()

		return ok(http.StatusOK, map[string]any{"state": "updated", "order": st.renderOrder(o)})
	})
}

// changeable finds an order the sender may change: it exists and came through the API.
func (st *state) changeable(id string) (*order, *result) {
	o, found := st.orders[id]
	if !found {
		res := notFound()

		return nil, &res
	}

	if o.via != fcapi.Api {
		res := fail(http.StatusConflict, fcapi.ErrorErrorCodeNotApiOrder)

		return nil, &res
	}

	return o, nil
}

func (a *api) OrderCancel(w http.ResponseWriter, r *http.Request, id string) {
	mutate(a, w, r, maxJSONBody, validateCancel, func(st *state, _ fcapi.OrderCancel) result {
		o, refused := st.changeable(id)
		if refused != nil {
			return *refused
		}

		answer := func(state string) result {
			return ok(http.StatusOK, map[string]any{"state": state, "order": st.renderOrder(o)})
		}

		switch st.changeAnswer(o) {
		case changeHandedOver:
			return fail(http.StatusConflict, fcapi.ErrorErrorCodeHandedOver)
		case changeCancelled:
			return answer("cancelled")
		case changeNeedsPerson:
			st.askPerson(o, "cancel")

			return ok(http.StatusAccepted, map[string]any{"state": "needs_person", "order": st.renderOrder(o)})
		case changeWaits:
			o.pending = &pendingChange{cancel: true}

			return ok(http.StatusAccepted, map[string]any{"state": "waiting", "order": st.renderOrder(o)})
		case changeNow:
			st.cancel(o, "outside_system")

			return answer("cancelled")
		}

		return answer("cancelled")
	})
}

func (a *api) OrderReduce(w http.ResponseWriter, r *http.Request, id string) {
	mutate(a, w, r, maxJSONBody, validateReduce, func(st *state, in fcapi.OrderReduce) result {
		o, refused := st.changeable(id)
		if refused != nil {
			return *refused
		}

		switch st.changeAnswer(o) {
		case changeHandedOver:
			return fail(http.StatusConflict, fcapi.ErrorErrorCodeHandedOver)
		case changeCancelled:
			return fail(http.StatusConflict, fcapi.ErrorErrorCodeConflict)
		case changeNow, changeWaits, changeNeedsPerson:
		}

		wanted, found := st.wantedAfter(o, in)
		if len(found) > 0 {
			return badRequest(found)
		}

		switch st.changeAnswer(o) {
		case changeNeedsPerson:
			st.askPerson(o, "reduce")

			return ok(http.StatusAccepted, map[string]any{"state": "needs_person", "order": st.renderOrder(o)})
		case changeWaits:
			if o.pending == nil {
				o.pending = &pendingChange{wanted: map[int]int{}}
			}

			if !o.pending.cancel {
				for line, n := range wanted {
					o.pending.wanted[line] = n
				}
			}

			return ok(http.StatusAccepted, map[string]any{"state": "waiting", "order": st.renderOrder(o)})
		case changeNow, changeHandedOver, changeCancelled:
		}

		st.cut(o, wanted)

		state := "reduced"
		if o.status == fcapi.OrderStatusCancelled {
			state = "cancelled"
		}

		return ok(http.StatusOK, map[string]any{"state": state, "order": st.renderOrder(o)})
	})
}

// wantedAfter reads a cut: for each named line, how many units are wanted now. A number above the current one, or a line
// the order does not have, is refused.
func (st *state) wantedAfter(o *order, in fcapi.OrderReduce) (map[int]int, issues) {
	var found issues

	wanted := map[int]int{}

	for i, l := range in.Lines {
		path := "lines[" + strconv.Itoa(i) + "]."

		var line *orderLine

		for _, candidate := range o.lines {
			if candidate.number == l.Line {
				line = candidate
			}
		}

		switch {
		case line == nil:
			found.add(path+"line", "unknown_line")
		case l.Quantity > line.wanted():
			found.add(path+"quantity", "above_current")
		default:
			wanted[l.Line] = l.Quantity
		}
	}

	return wanted, found
}

func (st *state) parcelOf(o *order, number int) *parcel {
	for _, p := range o.parcels {
		if p.number == number {
			return p
		}
	}

	return nil
}

func (a *api) LabelPut(w http.ResponseWriter, r *http.Request, id string, number int) {
	mutate(a, w, r, maxFileBody, validateLabel, func(st *state, in fcapi.Label) result {
		o, refused := st.changeable(id)
		if refused != nil {
			return *refused
		}

		p := st.parcelOf(o, number)
		if p == nil {
			return notFound()
		}

		switch o.status {
		case fcapi.OrderStatusShipped:
			return fail(http.StatusConflict, fcapi.ErrorErrorCodeHandedOver)
		case fcapi.OrderStatusCancelled:
			return fail(http.StatusConflict, fcapi.ErrorErrorCodeConflict)
		case fcapi.OrderStatusAccepted, fcapi.OrderStatusWaitingStock, fcapi.OrderStatusReserved, fcapi.OrderStatusPicking,
			fcapi.OrderStatusPacking, fcapi.OrderStatusPacked:
		}

		owner := id + "/" + strconv.Itoa(number)

		if in.ScanCode != nil {
			if holder, taken := st.scanCodes[*in.ScanCode]; taken && holder != owner {
				return fail(http.StatusConflict, fcapi.ErrorErrorCodeConflict)
			}
		}

		st.dropLabel(o, p)

		p.label = &labelRecord{contentType: string(in.ContentType), scanCode: in.ScanCode, data: in.Data}
		if in.ScanCode != nil {
			st.scanCodes[*in.ScanCode] = owner
		}

		st.emit("order.label_attached", o, nil, nil, map[string]any{"parcel": number})

		return ok(http.StatusOK, map[string]any{"state": "updated", "order": st.renderOrder(o)})
	})
}

func (st *state) dropLabel(o *order, p *parcel) {
	if p.label != nil && p.label.scanCode != nil {
		delete(st.scanCodes, *p.label.scanCode)
	}

	p.label = nil
	o.updatedAt = st.now()
}

func (a *api) LabelDelete(w http.ResponseWriter, r *http.Request, id string, number int) {
	a.act(w, r, true, func(st *state) result {
		o, refused := st.changeable(id)
		if refused != nil {
			return *refused
		}

		p := st.parcelOf(o, number)
		if p == nil {
			return notFound()
		}

		switch {
		case o.status == fcapi.OrderStatusShipped:
			return fail(http.StatusConflict, fcapi.ErrorErrorCodeHandedOver)
		case o.status == fcapi.OrderStatusCancelled, p.closedAt != nil:
			return fail(http.StatusConflict, fcapi.ErrorErrorCodeConflict)
		}

		st.dropLabel(o, p)

		return ok(http.StatusOK, map[string]any{"state": "updated", "order": st.renderOrder(o)})
	})
}

func (a *api) OrderProblemSet(w http.ResponseWriter, r *http.Request, id string) {
	mutate(a, w, r, maxJSONBody, validateProblem, func(st *state, in fcapi.ProblemIn) result {
		o, refused := st.changeable(id)
		if refused != nil {
			return *refused
		}

		o.problem = &fcapi.Problem{At: st.now().Format(time.RFC3339), Code: in.Code, Message: in.Message}
		o.updatedAt = st.now()

		return ok(http.StatusOK, map[string]any{"state": "updated", "order": st.renderOrder(o)})
	})
}

func (a *api) OrderProblemClear(w http.ResponseWriter, r *http.Request, id string) {
	a.act(w, r, true, func(st *state) result {
		o, refused := st.changeable(id)
		if refused != nil {
			return *refused
		}

		o.problem = nil
		o.updatedAt = st.now()

		return ok(http.StatusOK, map[string]any{"state": "updated", "order": st.renderOrder(o)})
	})
}

func (a *api) HeldList(w http.ResponseWriter, r *http.Request) {
	a.act(w, r, false, func(st *state) result {
		held := []fcapi.HeldOrder{}

		for _, h := range st.heldList {
			if h.waiting() {
				held = append(held, st.renderHeld(h))
			}
		}

		return ok(http.StatusOK, map[string]any{"held_orders": held})
	})
}

func (a *api) HeldGet(w http.ResponseWriter, r *http.Request, id string) {
	a.act(w, r, false, func(st *state) result {
		h, found := st.held[id]
		if !found {
			return notFound()
		}

		return ok(http.StatusOK, map[string]any{"held": st.renderHeld(h)})
	})
}

func (a *api) HeldWithdraw(w http.ResponseWriter, r *http.Request, id string) {
	a.act(w, r, true, func(st *state) result {
		h, found := st.held[id]
		if !found {
			return notFound()
		}

		if h.waiting() {
			h.state = fcapi.HeldOrderStateWithdrawn
		}

		return ok(http.StatusOK, map[string]any{"held": st.renderHeld(h)})
	})
}
