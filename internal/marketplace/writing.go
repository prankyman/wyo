package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// The identifiers a marketplace may require or accept for one unit of goods of an order, in the connector's words.
const (
	// IdentifierMarking is the code of the state marking system printed on the unit.
	IdentifierMarking = "marking"
	// IdentifierIMEI is a phone's number.
	IdentifierIMEI = "imei"
	// IdentifierUIN is a jewellery item's number.
	IdentifierUIN = "uin"
	// IdentifierGTIN is the goods' international number.
	IdentifierGTIN = "gtin"
	// IdentifierExpiry is the goods' expiry date.
	IdentifierExpiry = "expiry"
	// IdentifierCustoms is the number of a customs declaration.
	IdentifierCustoms = "customs"
)

// VerdictKind is a marketplace's answer about the identifiers of an order, in the connector's words.
type VerdictKind string

// The verdicts.
const (
	// VerdictAccepted: the identifier was checked and accepted.
	VerdictAccepted VerdictKind = "accepted"
	// VerdictNotRequired: no check is needed (attached and not checked, or not attached and not required).
	VerdictNotRequired VerdictKind = "not_required"
	// VerdictPending: the check goes on; the answer is not final.
	VerdictPending VerdictKind = "pending"
	// VerdictUnfinished: the marketplace stopped waiting for the check and goes on with it later; the hand-over is not
	// stopped by it.
	VerdictUnfinished VerdictKind = "unfinished"
	// VerdictMissing: the identifier is required and none is attached.
	VerdictMissing VerdictKind = "missing"
	// VerdictRefused: the identifier was refused.
	VerdictRefused VerdictKind = "refused"
)

// Final says whether a verdict is the marketplace's last word: everything but VerdictPending.
func (v VerdictKind) Final() bool { return v != VerdictPending }

// Passes says whether a verdict lets the order be handed over.
func (v VerdictKind) Passes() bool {
	return v == VerdictAccepted || v == VerdictNotRequired || v == VerdictUnfinished
}

// BatchState is a marketplace's group of orders that leave together (a supply, as some marketplaces call it), as read now: its
// number, whether it is closed (handed to delivery, takes no more orders), the numbers of its orders and of its transport
// boxes, and whether its shipping (how, when and where it is shipped) is set, with the point.
type BatchState struct {
	Number        string
	Closed        bool
	Orders        []string
	Boxes         []string
	ShippingSet   bool
	ShippingPoint string
}

// Label is the label of one order: a picture to print (ContentType image/png), and the text its code holds, which tells
// two labels apart.
type Label struct {
	Number      string
	ContentType string
	Data        []byte
	Code        string
}

// Document is a file of a hand-over: the code the receiving point scans, or a transport box's label. Code is the text the
// file's code holds.
type Document struct {
	ContentType string
	Data        []byte
	Code        string
}

// Verdict is a marketplace's verdict on one identifier of an order: the identifier, the verdict, the marketplace's own
// word for it and whether a value is attached.
type Verdict struct {
	Identifier string
	Kind       VerdictKind
	Words      string
	Filled     bool
}

// OrderVerdicts are the verdicts of one order.
type OrderVerdicts struct {
	Number   string
	Verdicts []Verdict
}

// ShippingPoint is a place a batch can be shipped to.
type ShippingPoint struct {
	ID      string
	Name    string
	Address string
}

// Shipping is how, when and where a batch is shipped: the adapter's hand-over settings, the point and the day.
type Shipping struct {
	Settings map[string]string
	Point    string
	Day      time.Time
}

// OrderConfirmer is the part orders.confirm: an order is confirmed by putting it into an open batch, which moves it to
// assembly and makes the marketplace issue its label. Batch reads a batch before a write that changes it.
type OrderConfirmer interface {
	NewBatch(ctx context.Context, conn Conn, name string) (string, error)
	Batch(ctx context.Context, conn Conn, number string) (BatchState, error)
	PutInBatch(ctx context.Context, conn Conn, batch string, numbers []string) error
}

// LabelFetcher is the part labels.fetch: the labels of confirmed orders, as pictures of 58 × 40 mm.
type LabelFetcher interface {
	Labels(ctx context.Context, conn Conn, numbers []string) ([]Label, error)
}

// CodeReporter is the part codes.report: an order's codes of one identifier, sent exactly as they were scanned, and
// removed.
type CodeReporter interface {
	ReportCodes(ctx context.Context, conn Conn, number, identifier string, codes []string) error
	RemoveCodes(ctx context.Context, conn Conn, number, identifier string) error
}

// VerdictReader is the part codes.verdict.
type VerdictReader interface {
	Verdicts(ctx context.Context, conn Conn, numbers []string) ([]OrderVerdicts, error)
}

// HandoverGrouper is the part handover.group: a hand-over's batch is made with OrderConfirmer.NewBatch and filled with
// PutInBatch; this part adds its transport boxes, lists the points it can be shipped to, sets its shipping and deletes a
// batch that is still empty.
type HandoverGrouper interface {
	AddBoxes(ctx context.Context, conn Conn, batch string, count int) error
	ShippingPoints(ctx context.Context, conn Conn, batch string, settings map[string]string, day time.Time) ([]ShippingPoint, error)
	SetShipping(ctx context.Context, conn Conn, batch string, shipping Shipping) error
	DeleteBatch(ctx context.Context, conn Conn, batch string) error
}

// HandoverCloser is the part handover.close: the batch is handed to delivery and takes no more orders.
type HandoverCloser interface {
	CloseBatch(ctx context.Context, conn Conn, batch string) error
}

// HandoverDocumenter is the part handover.documents: the closed batch's code and its transport boxes' labels.
type HandoverDocumenter interface {
	BatchCode(ctx context.Context, conn Conn, batch string) (Document, error)
	BoxLabels(ctx context.Context, conn Conn, batch string, boxes []string) ([]Document, error)
}

// Print is a label's fingerprint: what tells a changed label from the one attached. It is the SHA-256 of the text its code
// holds, or of the picture when the marketplace gives no text.
func (l Label) Print() string {
	sum := sha256.New()

	if l.Code != "" {
		_, _ = sum.Write([]byte(l.Code))
	} else {
		_, _ = sum.Write(l.Data)
	}

	return hex.EncodeToString(sum.Sum(nil))
}
