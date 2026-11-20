package marketplace

import "context"

// The contract's parts of Plan 8: a marketplace that takes the boxes of an order with their marking codes in one report,
// is told separately that the order is ready to ship, makes the day's batch of a warehouse by itself (it is found, compared
// and confirmed, never changed), makes each box's label in the size the fulfilment centre names, and checks the code a
// courier names for an order that leaves alone.

// LabelSize is the size in millimetres of the labels the fulfilment centre prints a marketplace's labels on; the zero value
// is a size it does not name, and then the marketplace's own default is asked for. The wyo server chooses nothing the
// fulfilment centre names: it passes the size on, and an adapter whose marketplace does not make it refuses with
// CodeLabelSizeUnsupported.
type LabelSize struct {
	WidthMM, HeightMM int
}

// Named says whether the fulfilment centre named a size.
func (s LabelSize) Named() bool { return s.WidthMM > 0 && s.HeightMM > 0 }

// CodeLabelSizeUnsupported is the refusal code of a label size the marketplace does not make; its params are the size's
// "width" and "height". Every adapter refuses with this code, so that the problem it makes is one, in each marketplace's
// words (marketplace.<code>.refusal.label_size_unsupported).
const CodeLabelSizeUnsupported = "label_size_unsupported"

// SizeUnsupported is the refusal of a label size the marketplace does not make.
func SizeUnsupported(size LabelSize) *Refusal {
	return &Refusal{Code: CodeLabelSizeUnsupported, Params: map[string]any{"width": size.WidthMM, "height": size.HeightMM}}
}

// SizedLabelFetcher is the part labels.fetch in the form of a marketplace that answers at once, asked in the size the
// fulfilment centre names; an adapter that implements it also implements LabelFetcher, which asks its own default.
type SizedLabelFetcher interface {
	LabelsOfSize(ctx context.Context, conn Conn, numbers []string, size LabelSize) ([]Label, error)
}

// BoxLine is the units of one of an order's lines in one box: Line is the line's place in the order, from 1 (the order of
// Order.Lines, the one in which the fulfilment centre was sent them), Quantity how many units, and Codes the marking codes of
// those units, each exactly as scanned (none, or one for each unit that carries one).
type BoxLine struct {
	Line     int
	Quantity int
	Codes    []string
}

// Box is the contents of one of the fulfilment centre's parcels.
type Box struct {
	Lines []BoxLine
}

// Boxed is the marketplace's answer to a report of the boxes: its own number of each box, in the order the boxes were given,
// and whether marking codes were sent, so that their verdict is awaited (codes.verdict).
type Boxed struct {
	Boxes []string
	Codes bool
}

// BoxState is how an order of a marketplace that takes its boxes in one report stands there now: Ready says it is ready to
// ship (its boxes can no longer change), Cancelled that it was cancelled, with the marketplace's Words; Counts are the units
// of each of the order's lines, in the order's order of lines (Counts[0] is line 1); Boxes are the marketplace's numbers of
// the boxes it holds (none before a report).
type BoxState struct {
	Ready, Cancelled bool
	Words            string
	Counts           []int
	Boxes            []string
}

// BoxPacker is the part orders.pack in the form of a marketplace that takes each box with its units and their marking codes
// in one report (Description.CodesInPack), a later report replacing the earlier until the order is ready. PackBoxes never
// removes goods from the order. A marketplace that needs an identifier the connector cannot give, or a marking code that was
// not scanned, answers with a *NeedsIdentifier.
type BoxPacker interface {
	PackBoxes(ctx context.Context, conn Conn, number string, boxes []Box) (Boxed, error)
	BoxState(ctx context.Context, conn Conn, number string) (BoxState, error)
}

// BoxLabeler is the part labels.fetch in the form of a marketplace that makes a label for each box of an order: the file of
// one box's label (the marketplace's number of the box), in the size given.
type BoxLabeler interface {
	BoxLabel(ctx context.Context, conn Conn, number, box string, size LabelSize) (Label, error)
}

// OrderReadier is the part orders.ready: the marketplace is told the order is packed and will be handed over. It is the only
// change of an order's status the wyo server ever writes, and it can write nothing else; the engine reads the order first
// (BoxState) and never writes it for an order that is ready already.
type OrderReadier interface {
	MakeReady(ctx context.Context, conn Conn, number string) error
}

// The answers to a courier's code (the neutral API's words).
const (
	CourierAccepted = "accepted"
	CourierRefused  = "refused"
	CourierNewCode  = "new_code_needed"
)

// CourierAnswer is a marketplace's answer to a courier's code: one of the Courier* words, the tries the marketplace still
// allows (below 0 when it names none) and its own words, when it gives some.
type CourierAnswer struct {
	Result       string
	AttemptsLeft int
	Words        string
}

// CourierCodeChecker is the part orders.courier_code: the code a courier named for an order is checked with the marketplace.
// An order of a scheme the adapter names in Description.CourierSchemes leaves the fulfilment centre alone, to a courier whose
// code was checked, and joins no hand-over.
type CourierCodeChecker interface {
	CheckCourierCode(ctx context.Context, conn Conn, number, code string) (CourierAnswer, error)
}

// LeavesByCourierCode says whether an order of a scheme leaves alone, to a courier whose code is checked.
func (d Description) LeavesByCourierCode(s Scheme) bool {
	if !d.Supports(OrdersCourierCode) {
		return false
	}

	for _, c := range d.CourierSchemes {
		if c == s {
			return true
		}
	}

	return false
}

// CloseWait is the answer of a marketplace that cannot close (confirm) a batch yet — its cutoff time has not come, or it is
// still taking orders out of the batch; Words are its own words. The hand-over waits, and tries again every five minutes
// while the request stands.
type CloseWait struct {
	Words string
}

// Error implements error.
func (w *CloseWait) Error() string { return "the marketplace cannot close the batch yet" }
