package yandex

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// writingControlRoutes are the control calls of the writing methods (the list is in control.go).
func (s *Server) writingControlRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/__control/orders/verdict", controlCall(s, false, (*state).chooseVerdict))
	mux.HandleFunc("/__control/orders/courier-code", controlCall(s, false, (*state).expectCourierCode))
	mux.HandleFunc("/__control/shipments", controlCall(s, false, (*state).addShipment))
	mux.HandleFunc("/__control/shipments/update", controlCall(s, false, (*state).updateShipment))
}

type verdictRequest struct {
	OrderID   int64  `json:"order_id"`
	Status    string `json:"status"`
	Substatus string `json:"substatus"`
}

// chooseVerdict makes every marking code of the order read as the status; an empty status gives the fake's own course back.
func (st *state) chooseVerdict(in verdictRequest) (outcome, error) {
	o := st.orderOf(in.OrderID)
	if o == nil {
		return outcome{}, fmt.Errorf("there is no order %d", in.OrderID)
	}

	if in.Status != "" && !yandexapi.CisStatusType(in.Status).Valid() {
		return outcome{}, fmt.Errorf("status %q is not a status of Яндекс Маркет's schema CisStatusType", in.Status)
	}

	if in.Substatus != "" && !yandexapi.CisSubstatusType(in.Substatus).Valid() {
		return outcome{}, fmt.Errorf("substatus %q is not a substatus of Яндекс Маркет's schema CisSubstatusType", in.Substatus)
	}

	o.verdict = verdictChoice{status: in.Status, substatus: in.Substatus}

	return outcome{}, nil
}

type courierCodeRequest struct {
	OrderID      int64  `json:"order_id"`
	Code         string `json:"code"`
	AttemptsLeft *int   `json:"attempts_left"`
	NeedUpdate   bool   `json:"need_update"`
}

// defaultAttempts are the tries an expected code allows when a test names none (the fake's choice: the documents give no number).
const defaultAttempts = 3

// expectCourierCode makes the order expect the code a courier names.
func (st *state) expectCourierCode(in courierCodeRequest) (outcome, error) {
	o := st.orderOf(in.OrderID)

	switch {
	case o == nil:
		return outcome{}, fmt.Errorf("there is no order %d", in.OrderID)
	case in.Code == "":
		return outcome{}, errors.New("code is required")
	case in.AttemptsLeft != nil && *in.AttemptsLeft < 0:
		return outcome{}, errors.New("attempts_left cannot be negative")
	}

	attempts := defaultAttempts
	if in.AttemptsLeft != nil {
		attempts = *in.AttemptsLeft
	}

	o.eac = &eacCheck{code: in.Code, attemptsLeft: attempts, attempts: attempts, needUpdate: in.NeedUpdate}

	return outcome{}, nil
}

type shipmentMake struct {
	WarehouseID  int64   `json:"warehouse_id"`
	Date         string  `json:"date"`
	Kind         string  `json:"kind"`
	OrderIDs     []int64 `json:"order_ids"`
	CutoffPassed bool    `json:"cutoff_passed"`
	Excluding    []int64 `json:"excluding_order_ids"`
}

// addShipment makes a daily shipment; the orders it names join it.
func (st *state) addShipment(in shipmentMake) (outcome, error) {
	switch {
	case in.WarehouseID <= 0:
		return outcome{}, errors.New("warehouse_id is a positive number")
	case !slices.Contains([]string{kindWithdraw, kindImport}, in.Kind):
		return outcome{}, fmt.Errorf("kind %q is WITHDRAW or IMPORT", in.Kind)
	}

	if _, err := time.Parse(dateLayout, in.Date); err != nil {
		return outcome{}, fmt.Errorf("date %q is not a date written YYYY-MM-DD", in.Date)
	}

	if err := st.knownOrders(in.OrderIDs, in.Excluding); err != nil {
		return outcome{}, err
	}

	st.nextShip++

	s := &shipment{
		seq: st.newSeq(), id: shipmentNumberBase + st.nextShip, warehouseID: in.WarehouseID, date: in.Date, kind: in.Kind,
		cutoffPassed: in.CutoffPassed, excluding: in.Excluding,
	}

	st.firstMile = append(st.firstMile, s)
	st.join(s, in.OrderIDs)

	return outcome{shipmentID: s.id}, nil
}

// knownOrders says every number is an order of the key.
func (st *state) knownOrders(lists ...[]int64) error {
	for _, list := range lists {
		for _, id := range list {
			if st.orderOf(id) == nil {
				return fmt.Errorf("there is no order %d", id)
			}
		}
	}

	return nil
}

// join puts the orders into the shipment, and each shows the shipment's number.
func (st *state) join(s *shipment, orders []int64) {
	s.orderIDs = slices.Clone(orders)

	for _, id := range orders {
		st.orderOf(id).shipmentID = s.id
	}
}

type shipmentUpdate struct {
	ShipmentID   int64    `json:"shipment_id"`
	CutoffPassed *bool    `json:"cutoff_passed"`
	Excluding    *[]int64 `json:"excluding_order_ids"`
	OrderIDs     *[]int64 `json:"order_ids"`
}

// updateShipment changes what a test chose of a shipment: its cutoff, the orders being taken out of it, the orders in it.
func (st *state) updateShipment(in shipmentUpdate) (outcome, error) {
	i := slices.IndexFunc(st.firstMile, func(s *shipment) bool { return s.id == in.ShipmentID })
	if i < 0 {
		return outcome{}, fmt.Errorf("there is no shipment %d", in.ShipmentID)
	}

	if err := st.knownOrders(deref(in.Excluding), deref(in.OrderIDs)); err != nil {
		return outcome{}, err
	}

	s := st.firstMile[i]

	if in.CutoffPassed != nil {
		s.cutoffPassed = *in.CutoffPassed
	}

	if in.Excluding != nil {
		s.excluding = *in.Excluding
	}

	if in.OrderIDs != nil {
		st.join(s, *in.OrderIDs)
	}

	return outcome{shipmentID: s.id}, nil
}
