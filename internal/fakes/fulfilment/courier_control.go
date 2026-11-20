package fulfilment

import (
	"net/http"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// courierRoutes registers the control calls that play the courier and the warehouse's worker (listed in control.go).
func (s *Server) courierRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /__control/orders/{id}/courier-code", s.controlCourierCode)
	mux.HandleFunc("POST /__control/orders/{id}/courier-code-checks/{check_id}/erase", s.controlEraseCode)
	mux.HandleFunc("GET /__control/orders/{id}/courier-code-checks", s.controlCourierChecks)
	mux.HandleFunc("POST /__control/warehouses/{code}/label-sizes", s.controlLabelSizes)
}

// controlCourierCode is a courier naming a code and a worker typing it: a new check and the event.
func (s *Server) controlCourierCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}

	if !controlBody(w, r, &in, false) {
		return
	}

	if in.Code == "" {
		fakekit.ControlError(w, http.StatusBadRequest, "code is the text the courier named")

		return
	}

	var (
		status = http.StatusOK
		out    any
	)

	s.with(s.kit.KeyOf(r), func(st *state) {
		o, found := st.orders[r.PathValue("id")]
		if !found {
			status, out = http.StatusNotFound, map[string]string{"error": "no such order"}

			return
		}

		check, err := st.nameCode(o, in.Code)
		if err != nil {
			status, out = http.StatusConflict, map[string]string{"error": err.Error()}

			return
		}

		out = map[string]any{"check_id": check.id, "order": st.renderOrder(o)}
	})

	fakekit.JSON(w, status, out)
}

// controlEraseCode erases a typed code as if the ten minutes had passed.
func (s *Server) controlEraseCode(w http.ResponseWriter, r *http.Request) {
	var (
		status = http.StatusOK
		out    any
	)

	s.with(s.kit.KeyOf(r), func(st *state) {
		o, found := st.orders[r.PathValue("id")]
		if !found || o.check(r.PathValue("check_id")) == nil {
			status, out = http.StatusNotFound, map[string]string{"error": "no such order or check"}

			return
		}

		o.check(r.PathValue("check_id")).erased = true
		o.updatedAt = st.now()
		out = map[string]any{"order": st.renderOrder(o)}
	})

	fakekit.JSON(w, status, out)
}

type courierCheckView struct {
	CheckID string             `json:"check_id"`
	Code    string             `json:"code"`
	TypedAt time.Time          `json:"typed_at"`
	Erased  bool               `json:"erased"`
	Answer  *courierAnswerView `json:"answer"`
}

type courierAnswerView struct {
	Result       string  `json:"result"`
	AttemptsLeft *int    `json:"attempts_left"`
	Message      *string `json:"message"`
}

// controlCourierChecks shows a test every check of an order with the answer the sender gave: the API never shows an answer back.
func (s *Server) controlCourierChecks(w http.ResponseWriter, r *http.Request) {
	var (
		status = http.StatusOK
		out    any
	)

	s.with(s.kit.KeyOf(r), func(st *state) {
		o, found := st.orders[r.PathValue("id")]
		if !found {
			status, out = http.StatusNotFound, map[string]string{"error": "no such order"}

			return
		}

		views := make([]courierCheckView, len(o.checks))

		for i, c := range o.checks {
			views[i] = courierCheckView{CheckID: c.id, Code: c.code, TypedAt: c.typedAt, Erased: c.erased}
			if c.answer != nil {
				views[i].Answer = &courierAnswerView{Result: string(c.answer.result), AttemptsLeft: c.answer.attemptsLeft, Message: c.answer.message}
			}
		}

		out = map[string]any{"checks": views}
	})

	fakekit.JSON(w, status, out)
}
