package yandex

import (
	"errors"
	"fmt"
	"time"

	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

const (
	statusProcessing    = "PROCESSING"
	substatusStarted    = "STARTED"
	statusCancelled     = "CANCELLED"
	programFBS          = "FBS"
	notifyOrderCreated  = "ORDER_CREATED"
	notifyStatusChanged = "ORDER_STATUS_UPDATED"
	notifyCancelled     = "ORDER_CANCELLED"
	notifyUpdated       = "ORDER_UPDATED"
)

type orderRequest struct {
	OrderID      *int64      `json:"order_id"`
	CampaignID   int64       `json:"campaign_id"`
	ProgramType  string      `json:"program_type"`
	Fake         bool        `json:"fake"`
	Status       string      `json:"status"`
	Substatus    string      `json:"substatus"`
	ShipmentDate string      `json:"shipment_date"`
	ShipmentTime string      `json:"shipment_time"`
	WarehouseID  string      `json:"warehouse_id"`
	Items        []orderItem `json:"items"`
}

// addOrder makes a new order, which the wyo server learns of by getBusinessOrders and by the notification ORDER_CREATED.
func (st *state) addOrder(in orderRequest) (outcome, error) {
	if st.campaignOf(in.CampaignID) == nil {
		return outcome{}, fmt.Errorf("there is no shop %d: set it up first", in.CampaignID)
	}

	program := in.ProgramType
	if program == "" {
		program = programFBS
	}

	if !yandexapi.SellingProgramType(program).Valid() {
		return outcome{}, fmt.Errorf("program_type %q is not one of Яндекс Маркет's", program)
	}

	status, substatus, err := orderStatus(in.Status, in.Substatus)
	if err != nil {
		return outcome{}, err
	}

	items, err := checkItems(in.Items)
	if err != nil {
		return outcome{}, err
	}

	if err := checkShipment(in.ShipmentDate, in.ShipmentTime); err != nil {
		return outcome{}, err
	}

	id, err := st.orderNumber(in.OrderID)
	if err != nil {
		return outcome{}, err
	}

	now := st.now()
	o := &order{
		seq: st.newSeq(), id: id, campaignID: in.CampaignID, program: program, fake: in.Fake, status: status, substatus: substatus,
		created: now, updated: now, items: items, warehouseID: in.WarehouseID,
	}

	st.setShipment(o, in.ShipmentDate, in.ShipmentTime)
	st.orders = append(st.orders, o)

	return outcome{orderID: id, notices: st.notice(notifyOrderCreated, orderCreated{
		notification: o.header(notifyOrderCreated), CreatedAt: stamp(o.created), Items: o.notified(),
	})}, nil
}

// orderStatus reads a status and a substatus a test gave. Nothing at all is PROCESSING and STARTED, as an order is when it
// comes; a status of PROCESSING alone is STARTED; any other status needs its substatus.
func orderStatus(status, substatus string) (string, string, error) {
	if status == "" {
		status = statusProcessing
	}

	if !yandexapi.OrderStatusType(status).Valid() {
		return "", "", fmt.Errorf("status %q is not one of Яндекс Маркет's", status)
	}

	if substatus == "" && status == statusProcessing {
		substatus = substatusStarted
	}

	if substatus == "" {
		return "", "", fmt.Errorf("the status %s needs a substatus", status)
	}

	if !yandexapi.OrderSubstatusType(substatus).Valid() {
		return "", "", fmt.Errorf("substatus %q is not one of Яндекс Маркет's", substatus)
	}

	return status, substatus, nil
}

func checkItems(items []orderItem) ([]orderItem, error) {
	if len(items) == 0 {
		return nil, errors.New("an order needs at least one line in items")
	}

	out := make([]orderItem, len(items))

	for i, it := range items {
		switch {
		case it.OfferID == "":
			return nil, fmt.Errorf("line %d has no offer_id", i+1)
		case it.Count < 1:
			return nil, fmt.Errorf("line %d needs a count of at least 1", i+1)
		}

		for _, t := range it.RequiredInstanceTypes {
			if !yandexapi.OrderItemInstanceType(t).Valid() {
				return nil, fmt.Errorf("required_instance_types of line %d holds %q, which is not one of Яндекс Маркет's", i+1, t)
			}
		}

		// The fake's choice, not Яндекс Маркет's: a line with no name is named after its article.
		if it.OfferName == "" {
			it.OfferName = "Product " + it.OfferID
		}

		out[i] = it
	}

	return out, nil
}

// checkShipment checks the day of a shipment (YYYY-MM-DD, the form of the schema BusinessOrderShipmentDTO) and its time (HH:MM
// or HH:MM:SS). A time needs a day.
func checkShipment(date, clock string) error {
	if date == "" {
		if clock != "" {
			return errors.New("shipment_time needs a shipment_date")
		}

		return nil
	}

	if _, err := time.Parse(dateLayout, date); err != nil {
		return fmt.Errorf("shipment_date %q is not a date written YYYY-MM-DD", date)
	}

	if clock == "" {
		return nil
	}

	if _, err := time.Parse("15:04", clock); err != nil {
		if _, err := time.Parse("15:04:05", clock); err != nil {
			return fmt.Errorf("shipment_time %q is not a time written HH:MM", clock)
		}
	}

	return nil
}

// setShipment puts the order into the shipment of its warehouse and day.
func (st *state) setShipment(o *order, date, clock string) {
	o.shipmentDate, o.shipmentTime = date, clock

	if date != "" {
		o.shipmentID = st.shipmentFor(o.warehouseID, date)
	}
}

// orderNumber is the number a test gave, which no order has yet, or the next one the fake gives: after the highest there is.
func (st *state) orderNumber(given *int64) (int64, error) {
	if given != nil {
		if *given <= 0 || st.orderOf(*given) != nil {
			return 0, fmt.Errorf("the order number %d is not a positive number used once", *given)
		}

		return *given, nil
	}

	highest := int64(orderNumberBase)

	for _, o := range st.orders {
		highest = max(highest, o.id)
	}

	return highest + 1, nil
}

type statusRequest struct {
	OrderID   int64  `json:"order_id"`
	Status    string `json:"status"`
	Substatus string `json:"substatus"`
}

// setStatus changes the status of an order, which moves its update date.
func (st *state) setStatus(in statusRequest) (outcome, error) {
	o := st.orderOf(in.OrderID)
	if o == nil {
		return outcome{}, fmt.Errorf("there is no order %d", in.OrderID)
	}

	if in.Status == "" {
		return outcome{}, errors.New("status is required")
	}

	status, substatus, err := orderStatus(in.Status, in.Substatus)
	if err != nil {
		return outcome{}, err
	}

	o.status, o.substatus, o.updated = status, substatus, st.now()

	return outcome{notices: st.notice(notifyStatusChanged, statusChanged{
		notification: o.header(notifyStatusChanged), Status: status, Substatus: substatus, UpdatedAt: stamp(o.updated),
	})}, nil
}

type cancelRequest struct {
	OrderID   int64  `json:"order_id"`
	Substatus string `json:"substatus"`
}

// cancelOrder cancels an order for a reason (its substatus).
func (st *state) cancelOrder(in cancelRequest) (outcome, error) {
	o := st.orderOf(in.OrderID)
	if o == nil {
		return outcome{}, fmt.Errorf("there is no order %d", in.OrderID)
	}

	if !yandexapi.OrderSubstatusType(in.Substatus).Valid() {
		return outcome{}, fmt.Errorf("substatus %q is not a reason of Яндекс Маркет's: give one such as SHOP_FAILED", in.Substatus)
	}

	o.status, o.substatus, o.updated = statusCancelled, in.Substatus, st.now()

	return outcome{notices: st.notice(notifyCancelled, orderCancelled{
		notification: o.header(notifyCancelled), CancelledAt: stamp(o.updated), Items: o.notified(),
	})}, nil
}

type shipmentRequest struct {
	OrderID      int64  `json:"order_id"`
	ShipmentDate string `json:"shipment_date"`
	ShipmentTime string `json:"shipment_time"`
}

// moveShipment moves the order to the shipment of another day.
func (st *state) moveShipment(in shipmentRequest) (outcome, error) {
	o := st.orderOf(in.OrderID)
	if o == nil {
		return outcome{}, fmt.Errorf("there is no order %d", in.OrderID)
	}

	if in.ShipmentDate == "" {
		return outcome{}, errors.New("shipment_date is required")
	}

	if err := checkShipment(in.ShipmentDate, in.ShipmentTime); err != nil {
		return outcome{}, err
	}

	st.setShipment(o, in.ShipmentDate, in.ShipmentTime)
	o.updated = st.now()

	return outcome{notices: st.notice(notifyUpdated, orderUpdated{
		notification: o.header(notifyUpdated), UpdateType: "SHIPMENT_DATE_UPDATED", UpdatedAt: stamp(o.updated),
	})}, nil
}
