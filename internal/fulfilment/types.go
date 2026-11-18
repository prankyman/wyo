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
)

// The states the exchange reads of an order and an account's key.
const (
	OrderReserved = fcapi.OrderStatusReserved
	OrderPacked   = fcapi.OrderStatusPacked
	KeyReadWrite  = fcapi.ReadWrite
)
