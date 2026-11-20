package fulfilment

import "github.com/prankyman/wyo/internal/fulfilment/fcapi"

// The neutral API's objects, as the exchange and the hand-overs use them: only this package imports the generated code
// (an architecture test), so the others name its types through these.
type (
	Account    = fcapi.Account
	Warehouse  = fcapi.Warehouse
	Order      = fcapi.Order
	OrderIn    = fcapi.OrderIn
	OrderLine  = fcapi.OrderInLine
	HeldOrder  = fcapi.HeldOrder
	HeldReason = fcapi.HeldReason
	Event      = fcapi.Event
	Handover   = fcapi.Handover
	// OrderParcel is one parcel of an order, with when it was closed.
	OrderParcel = fcapi.OrderParcel
)

// The states the exchange reads of an order and an account's key.
const (
	OrderReserved = fcapi.OrderStatusReserved
	OrderPacked   = fcapi.OrderStatusPacked
	KeyReadWrite  = fcapi.ReadWrite
)

// A warehouse's label size for one marketplace, and the two ways an order leaves: in a day's hand-over, or alone to a courier
// who names a code. HandoverShipment and HandoverCourierCode are what an order is sent with; OrderLeavesShipment and
// OrderLeavesCourierCode are what an order reports.
type (
	LabelSize       = fcapi.LabelSize
	OrderHandover   = fcapi.OrderHandover
	OrderInHandover = fcapi.OrderInHandover
)

const (
	HandoverShipment       = fcapi.OrderInHandoverShipment
	HandoverCourierCode    = fcapi.OrderInHandoverCourierCode
	OrderLeavesShipment    = fcapi.OrderHandoverShipment
	OrderLeavesCourierCode = fcapi.OrderHandoverCourierCode
)

// The answers to a courier's code.
const (
	CourierAccepted      = string(fcapi.CourierCodeAnswerInResultAccepted)
	CourierRefused       = string(fcapi.CourierCodeAnswerInResultRefused)
	CourierNewCodeNeeded = string(fcapi.CourierCodeAnswerInResultNewCodeNeeded)
)
