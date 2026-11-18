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
	// IdentifierMultibox is the count of boxes of a product that is sent in several.
	IdentifierMultibox = "multibox"
	// IdentifierWeight is the weight of a unit of goods sold by weight.
	IdentifierWeight = "weight"
	// IdentifierCountry is the country the goods were made in.
	IdentifierCountry = "country"
	// IdentifierRNPT is the number of an import registration of the goods (a batch's registration number).
	IdentifierRNPT = "rnpt"
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
	// VerdictAgain: the marketplace did not manage to check the identifier (it ran out of time); the full set is sent once
	// more. It is a last word of this try and lets nothing pass.
	VerdictAgain VerdictKind = "again"
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

// Label is the label of one order or one of its parts: a file to print (a PDF, a PNG or a JPEG, as the neutral API takes
// them), and the text its code holds when the marketplace gives one, which tells two labels apart.
type Label struct {
	Number      string
	ContentType string
	Data        []byte
	Code        string
}

// The kinds of a hand-over's document, as the neutral API names them (api/fulfilment-centre.yaml).
const (
	// DocHandoverCode is the code the receiving point scans.
	DocHandoverCode = "handover_code"
	// DocBoxLabel is a transport box's label.
	DocBoxLabel = "box_label"
	// DocAct is the list or the act that goes with the shipment.
	DocAct = "act"
)

// Document is a file of a hand-over: Kind is one of the Doc* constants, Title what the file is called (a box's number, the
// batch's number), and Code the text the file's code holds, when it holds one.
type Document struct {
	Kind        string
	Title       string
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

// BatchReader reads a batch before a write that changes it, whichever part made it.
type BatchReader interface {
	Batch(ctx context.Context, conn Conn, number string) (BatchState, error)
}

// OrderConfirmer is the part orders.confirm: an order is confirmed by putting it into an open batch, which moves it to
// assembly and makes the marketplace issue its label.
type OrderConfirmer interface {
	BatchReader
	NewBatch(ctx context.Context, conn Conn, name string) (string, error)
	PutInBatch(ctx context.Context, conn Conn, batch string, numbers []string) error
}

// ParcelLine is a quantity of one product: ItemID is the marketplace's number of the product (Line.ProductID).
type ParcelLine struct {
	ItemID   string
	Quantity int
}

// Parcel is the contents of one closed box.
type Parcel struct {
	Lines []ParcelLine
}

// PackedPart is a part the marketplace made of an order when its packing was reported: Number is the part's own number.
type PackedPart struct {
	Number string
	Lines  []ParcelLine
}

// PackState is how an order's packing stands at the marketplace now. Failed says the marketplace did not take the report
// (the report is made again); Cancelled says the order was cancelled; Words are the marketplace's own words about either,
// as received; Lines are the order's lines at the marketplace now.
type PackState struct {
	Packed, Failed, Cancelled bool
	Words                     string
	Lines                     []ParcelLine
}

// OrderPacker is the part orders.pack: the marketplace takes the report that an order is packed, with the contents of each
// closed box, and splits the order into one part for each box when there are several. Pack answers with the parts; a
// second call, PackState, reads whether the report took effect (an answer of success is not the end of it).
type OrderPacker interface {
	Pack(ctx context.Context, conn Conn, number string, parcels []Parcel) ([]PackedPart, error)
	PackState(ctx context.Context, conn Conn, number string) (PackState, error)
}

// LabelFetcher is the part labels.fetch in the form of a marketplace that answers at once: the labels of confirmed
// orders.
type LabelFetcher interface {
	Labels(ctx context.Context, conn Conn, numbers []string) ([]Label, error)
}

// LabelsState is the state of a labels task: Ready says every label of the task is made, and then Labels holds each with
// its file fetched; Unmade names, for a number whose label the marketplace could not make, the marketplace's words; Words
// are the marketplace's words when the whole task failed, and Failed says it did.
type LabelsState struct {
	Ready  bool
	Labels []Label
	Unmade map[string]string
	Words  string
	Failed bool
}

// LabelMaker is the part labels.fetch in the form of a marketplace that makes labels later: AskLabels asks for the labels
// of some numbers of the kind labelType (Warehouse.LabelType) and gives a ticket, ReadLabels reads the ticket until the
// labels are made.
type LabelMaker interface {
	AskLabels(ctx context.Context, conn Conn, numbers []string, labelType string) (string, error)
	ReadLabels(ctx context.Context, conn Conn, ticket string) (LabelsState, error)
}

// CodeReporter is the part codes.report: an order's codes of one identifier, sent exactly as they were scanned, and
// removed.
type CodeReporter interface {
	ReportCodes(ctx context.Context, conn Conn, number, identifier string, codes []string) error
	RemoveCodes(ctx context.Context, conn Conn, number, identifier string) error
}

// UnitCode is the marking code of one unit, with the product (Line.ProductID) it is on.
type UnitCode struct {
	ItemID string
	Code   string
}

// ProductCodeReporter is the part codes.report in the form of a marketplace that wants every code with its product, and
// replaces the codes it holds with each full set (Description.CodesReplace). The answer sent is false when the marketplace
// takes no code for this order, and then nothing was sent. A marketplace that needs an identifier it was not given answers
// with a *NeedsIdentifier.
type ProductCodeReporter interface {
	ReportProductCodes(ctx context.Context, conn Conn, number, identifier string, units []UnitCode) (bool, error)
}

// NeedsIdentifier is the answer of a marketplace that needs an identifier of the order which the connector cannot give
// (Identifier is one of the Identifier* constants); the owner gives it in the marketplace's own cabinet.
type NeedsIdentifier struct {
	Identifier string
}

// Error implements error.
func (n *NeedsIdentifier) Error() string {
	return "the marketplace needs the identifier " + n.Identifier
}

// VerdictReader is the part codes.verdict.
type VerdictReader interface {
	Verdicts(ctx context.Context, conn Conn, numbers []string) ([]OrderVerdicts, error)
}

// BatchRequest is a hand-over's batch as asked for: its name (HandoverBatchName), the marketplace's number of the
// warehouse the orders lie in and the day of the shipping.
type BatchRequest struct {
	Name      string
	Warehouse string
	Day       time.Time
}

// BatchWait is the answer of a marketplace that cannot make the batch yet; Words are its own words. The hand-over waits,
// and tries again every five minutes while the request stands.
type BatchWait struct {
	Words string
}

// Error implements error.
func (w *BatchWait) Error() string { return "the marketplace cannot make the batch yet" }

// HandoverGrouper is the part handover.group, which only groups: MakeBatch finds or makes the hand-over's batch (never a
// second one for the same request), FillBatch makes the batch hold exactly the given orders, so that a second call with
// the same orders changes nothing.
type HandoverGrouper interface {
	MakeBatch(ctx context.Context, conn Conn, req BatchRequest) (string, error)
	FillBatch(ctx context.Context, conn Conn, batch string, numbers []string) error
}

// BoxAdder adds the transport boxes of a batch, for the adapter that declares the step StepBoxes.
type BoxAdder interface {
	AddBoxes(ctx context.Context, conn Conn, batch string, count int) error
}

// ShippingSetter lists the points a batch can be shipped to and sets its shipping, for the adapter that declares the step
// StepShipping.
type ShippingSetter interface {
	ShippingPoints(ctx context.Context, conn Conn, batch string, settings map[string]string, day time.Time) ([]ShippingPoint, error)
	SetShipping(ctx context.Context, conn Conn, batch string, shipping Shipping) error
}

// DocumentsWait is the answer of a marketplace whose hand-over documents are not ready yet (it makes them while it is asked);
// Words are its own words. The hand-over waits and asks again at its next run.
type DocumentsWait struct {
	Words string
}

// Error implements error.
func (w *DocumentsWait) Error() string { return "the marketplace has not made the documents yet" }

// BatchDeleter deletes a batch that is not handed over yet.
type BatchDeleter interface {
	DeleteBatch(ctx context.Context, conn Conn, batch string) error
}

// HandoverCloser is the part handover.close: the batch is handed to delivery and takes no more orders.
type HandoverCloser interface {
	CloseBatch(ctx context.Context, conn Conn, batch string) error
}

// HandoverDocumenter is the part handover.documents: every document of the closed batch, each with its kind.
type HandoverDocumenter interface {
	Documents(ctx context.Context, conn Conn, batch string) ([]Document, error)
}

// batchDayLayout is the day in a batch's name: 18.11.2026.
const batchDayLayout = "02.01.2006"

// AssemblyBatchName is the name of the open batch an account's orders are confirmed into on a day, the day counted in
// loc: «wyo — сборка 18.11.2026». The exchange and the hand-over both name their batches here and nowhere else.
func AssemblyBatchName(now time.Time, loc *time.Location) string {
	return "wyo — сборка " + now.In(loc).Format(batchDayLayout)
}

// HandoverBatchName is the name of a hand-over's batch: «wyo — передача <the hand-over's number> 18.11.2026».
func HandoverBatchName(number string, day time.Time) string {
	return "wyo — передача " + number + " " + day.Format(batchDayLayout)
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
