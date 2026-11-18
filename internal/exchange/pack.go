package exchange

import (
	"context"
	"slices"
	"strconv"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// The exchange of an adapter that reports the packing (the contract part orders.pack) is steps 2 and 3 of the plan's flow
// for it: nothing is written to the marketplace when the goods are reserved; when the fulfilment centre reports packing
// finished the marking codes are sent, their verdict is waited for, the packing is reported with the fulfilment centre's
// parcels (which splits the marketplace's order when there are several), and the labels, which the marketplace makes only
// after that, are fetched for each parcel and attached.

// flow is the way the orders of an adapter go through the exchange.
type flow int

const (
	// flowNone: the adapter does not take part in the exchange.
	flowNone flow = iota
	// flowConfirm: the order is confirmed into a batch when the goods are reserved, and its label follows at once.
	flowConfirm
	// flowPack: the packing is reported after the codes are accepted, and the labels follow when the marketplace made them.
	flowPack
)

// flowOf chooses the flow by the parts the adapter claims.
func flowOf(adapter marketplace.Adapter) flow {
	switch d := adapter.Describe(); {
	case d.Supports(marketplace.OrdersConfirm):
		return flowConfirm
	case d.Supports(marketplace.OrdersPack):
		return flowPack
	}

	return flowNone
}

const (
	// packTriesMax is how many times a report the marketplace did not take is made again.
	packTriesMax = 3
	// packFast is how often the packing is read during the first packFastFor after it was reported; packSlow after.
	packFast    = 20 * time.Second
	packFastFor = 10 * time.Minute
	packSlow    = 5 * time.Minute
	// labelsAfter is how long after the packing is confirmed the labels are asked for: the marketplace recommends 45–60
	// seconds.
	labelsAfter = 60 * time.Second
	// labelFast is how often a labels task is read during the first labelFastFor; labelSlow after.
	labelFast    = 10 * time.Second
	labelFastFor = 5 * time.Minute
	labelSlow    = 5 * time.Minute
	// codesResendsMax is how many times the full set of codes is sent again after the marketplace did not manage to check it.
	codesResendsMax = 1
)

// stepPack is the packing report, which «Передать заново» repeats.
const stepPack = "pack"

// The problems of the packing and the labels (the interface's `problem.<code>`).
const (
	problemPackContents    = "pack_contents_differ"
	problemPackReopened    = "pack_reopened_after_report"
	problemPackParts       = "pack_parts_unknown"
	problemLabelsNotMade   = "labels_not_made"
	problemCodesNotChecked = "codes_not_checked"
)

// itemOf is the marketplace's number of the product on the order's line n (the fulfilment centre numbers the lines it was
// sent from 1, in the order they were sent).
func itemOf(o *order, line int64) string {
	if line < 1 || int(line) > len(o.row.Lines) {
		return ""
	}

	return o.row.Lines[line-1].ProductID
}

// parcelsOf are the contents of the fulfilment centre's parcels in the marketplace's words: for each parcel the products
// by their marketplace number with the units packed.
func parcelsOf(o *order, fc fulfilment.Order) []marketplace.Parcel {
	out := make([]marketplace.Parcel, 0, len(fc.Parcels))

	for _, p := range fc.Parcels {
		var lines []marketplace.ParcelLine

		for _, u := range p.Units {
			item := itemOf(o, int64(u.Line))

			if i := slices.IndexFunc(lines, func(l marketplace.ParcelLine) bool { return l.ItemID == item }); i >= 0 {
				lines[i].Quantity++

				continue
			}

			lines = append(lines, marketplace.ParcelLine{ItemID: item, Quantity: 1})
		}

		out = append(out, marketplace.Parcel{Lines: lines})
	}

	return out
}

// totals adds the quantities of lines by product.
func totals(parcels []marketplace.Parcel, extra []marketplace.ParcelLine) map[string]int {
	sum := map[string]int{}

	for _, p := range parcels {
		for _, l := range p.Lines {
			sum[l.ItemID] += l.Quantity
		}
	}

	for _, l := range extra {
		sum[l.ItemID] += l.Quantity
	}

	return sum
}

// sameLines says whether the parcels hold exactly the products and quantities the marketplace's order holds.
func sameLines(parcels []marketplace.Parcel, order []marketplace.ParcelLine) bool {
	have, want := totals(parcels, nil), totals(nil, order)

	if len(have) != len(want) {
		return false
	}

	for item, n := range want {
		if have[item] != n {
			return false
		}
	}

	return true
}

// linesKey is a parcel's contents as comparable text, in the order of the products' numbers.
func linesKey(lines []marketplace.ParcelLine) string {
	sorted := slices.Clone(lines)
	slices.SortFunc(sorted, func(a, b marketplace.ParcelLine) int {
		if a.ItemID != b.ItemID {
			if a.ItemID < b.ItemID {
				return -1
			}

			return 1
		}

		return a.Quantity - b.Quantity
	})

	key := ""
	for _, l := range sorted {
		key += l.ItemID + "×" + strconv.Itoa(l.Quantity) + ";"
	}

	return key
}

// assign gives each parcel the marketplace's number of the part with the same contents; two parcels with the same
// contents may take either part, for their labels are alike. A marketplace that gave no parts (the packing was reported
// before) leaves the order's own number on its one parcel.
func assign(o *order, parcels []marketplace.Parcel, parts []marketplace.PackedPart) []string {
	numbers := make([]string, len(parcels))
	taken := make([]bool, len(parts))

	for i, p := range parcels {
		for j, part := range parts {
			if !taken[j] && linesKey(part.Lines) == linesKey(p.Lines) {
				numbers[i], taken[j] = part.Number, true

				break
			}
		}
	}

	// A part whose contents the marketplace did not give back in the same words is taken in order.
	for i := range numbers {
		if numbers[i] != "" {
			continue
		}

		for j, part := range parts {
			if !taken[j] {
				numbers[i], taken[j] = part.Number, true

				break
			}
		}
	}

	if len(parts) == 0 && len(numbers) == 1 {
		numbers[0] = o.row.Number
	}

	return numbers
}

// parcelRows are the order's parcels as kept, made when the fulfilment centre's parcels are known: a row for each, with
// what an earlier report already gave kept.
func (s *Service) parcelRows(ctx context.Context, o *order, count int) ([]store.Parcel, error) {
	have, err := s.opts.Store.Read().ParcelsOf(ctx, o.x.OrderID)
	if err != nil {
		return nil, err
	}

	rows := make([]store.Parcel, 0, count)

	for n := 1; n <= count; n++ {
		i := slices.IndexFunc(have, func(p store.Parcel) bool { return p.Number == int64(n) })
		if i >= 0 {
			rows = append(rows, have[i])

			continue
		}

		rows = append(rows, store.Parcel{OrderID: o.x.OrderID, Number: int64(n)})
	}

	return rows, nil
}

// packLater sets when the packing is read next: every 20 seconds during the first 10 minutes after the report, then every
// 5 minutes.
func (s *Service) packLater(ctx context.Context, o *order) {
	now := s.opts.Clock.Now()

	wait := packSlow
	if o.x.PackReportedAt.IsZero() || now.Sub(o.x.PackReportedAt) < packFastFor {
		wait = packFast
	}

	o.x.PackNextAt = now.Add(wait)
	s.save(ctx, o)
}

// packStep is step 3, done when the codes are accepted (or none is needed) and by the job while the packing is read: the
// order is read at the marketplace; a cancelled one is passed on, a packed one has its labels planned, one the report did not
// take (or that was never reported) is reported, and one reported and not yet packed is read again later. Nothing is
// written twice: the report is made only for an order the marketplace shows not packed.
func (s *Service) packStep(ctx context.Context, sess *session, o *order) {
	adapter, conn, err := s.conn(ctx, o, jobVerdicts)
	if err != nil {
		s.packLater(ctx, o)

		return
	}

	packer, ok := part[marketplace.OrderPacker](adapter, marketplace.OrdersPack, conn.Test)
	if !ok {
		return
	}

	state, err := packer.PackState(ctx, conn, o.row.Number)
	if err != nil {
		if !s.marketplaceProblem(ctx, sess, o, stepPack, err) {
			s.packLater(ctx, o)
		}

		return
	}

	switch {
	case state.Cancelled:
		o.x.PackNextAt = time.Time{}
		s.passCancel(ctx, sess, o)
	case state.Packed:
		s.packConfirmed(ctx, sess, o)
	case o.x.PackReportedAt.IsZero() || state.Failed:
		s.report(ctx, sess, o, packer, conn, state)
	default:
		s.packLater(ctx, o)
	}
}

// settledAtMarketplace reads the posting when the codes cannot be sent or checked and says whether the marketplace has settled it already: a
// cancelled one is passed on, one shown packed gets its labels planned and its codes are never sent. It writes nothing to the
// marketplace.
func (s *Service) settledAtMarketplace(ctx context.Context, sess *session, o *order) bool {
	adapter, conn, err := s.conn(ctx, o, jobEvents)
	if err != nil {
		return false
	}

	packer, ok := part[marketplace.OrderPacker](adapter, marketplace.OrdersPack, conn.Test)
	if !ok {
		return false
	}

	state, err := packer.PackState(ctx, conn, o.row.Number)
	if err != nil {
		// The codes step meets the same failure and says what to do about it.
		return false
	}

	switch {
	case state.Cancelled:
		// The checking ends; the cancellation is passed on once, whoever noticed it first.
		o.x.NextCheckAt = time.Time{}

		if o.x.CancelAnswer != "" {
			s.save(ctx, o)

			return true
		}

		s.passCancel(ctx, sess, o)

		return true
	case state.Packed:
		o.x.CodesState, o.x.NextCheckAt = codesNotTaken, time.Time{}
		o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "codes_not_needed"})
		s.packConfirmed(ctx, sess, o)

		return true
	}

	return false
}

// report reports the packing with the fulfilment centre's parcels. A report the marketplace refused or did not take three
// times is a problem in the marketplace's words.
func (s *Service) report(ctx context.Context, sess *session, o *order, packer marketplace.OrderPacker, conn marketplace.Conn, state marketplace.PackState) {
	if state.Failed && o.x.PackTries >= packTriesMax {
		// The reading of the packing ends here: «Передать заново» starts the reports again.
		o.x.PackNextAt = time.Time{}
		s.problem(ctx, sess, o, store.Problem{
			Code: problemMarketplace, Params: map[string]any{"step": stepPack}, MpCode: "packing_not_taken", Words: state.Words,
		}, stepPack)

		return
	}

	fc, err := sess.fc.Order(ctx, o.x.FcID)
	if err != nil {
		if !s.failed(ctx, err) {
			s.problem(ctx, sess, o, fcProblem(stepPack, err), stepPack)

			return
		}

		s.packLater(ctx, o)

		return
	}

	if fc.Status != fulfilment.OrderPacked {
		// Packing was restarted since: the next order.packing_finished brings the order here again.
		o.x.PackNextAt = time.Time{}
		s.save(ctx, o)

		return
	}

	parcels := parcelsOf(o, fc)
	if !sameLines(parcels, state.Lines) {
		s.problem(ctx, sess, o, store.Problem{Code: problemPackContents}, stepPack)

		return
	}

	parts, err := packer.Pack(ctx, conn, o.row.Number, parcels)
	if err != nil {
		if !s.marketplaceProblem(ctx, sess, o, stepPack, err) {
			s.packLater(ctx, o)
		}

		return
	}

	rows, err := s.parcelRows(ctx, o, len(parcels))
	if err != nil {
		s.opts.Log.Error("the parcels could not be read", "order", o.x.OrderID, "err", err)

		return
	}

	for i, number := range assign(o, parcels, parts) {
		rows[i].MpNumber = number
	}

	s.write(ctx, func(q *store.Queries) error { return q.ReplaceParcels(ctx, o.x.OrderID, rows) })

	now := s.opts.Clock.Now()
	o.x.PackReportedAt, o.x.PackTries = now, o.x.PackTries+1
	o.added = append(o.added, store.ExchangeHistory{At: now, State: o.x.State, Code: "packing_reported"})
	s.clearProblem(ctx, sess, o)
	s.packLater(ctx, o)
}

// packConfirmed notes that the marketplace shows the order packed and plans the labels of its parcels, which the
// marketplace makes only after the packing.
func (s *Service) packConfirmed(ctx context.Context, sess *session, o *order) {
	now := s.opts.Clock.Now()

	fc, err := sess.fc.Order(ctx, o.x.FcID)
	if err != nil {
		s.failed(ctx, err)
		s.packLater(ctx, o)

		return
	}

	rows, err := s.parcelRows(ctx, o, max(1, len(fc.Parcels)))
	if err != nil {
		s.opts.Log.Error("the parcels could not be read", "order", o.x.OrderID, "err", err)

		return
	}

	for i := range rows {
		if rows[i].MpNumber == "" && len(rows) == 1 {
			// The packing was reported before the wyo server did (in the marketplace's cabinet): the order's own number.
			rows[i].MpNumber = o.row.Number
		}
	}

	// A posting packed in several parcels is split into parts whose numbers only the answer of the report names. When the answer
	// was lost (or the packing was reported before the wyo server did) the numbers are not known and no label can be asked for: the
	// owner is told, and nothing is guessed.
	if slices.ContainsFunc(rows, func(p store.Parcel) bool { return p.MpNumber == "" }) {
		o.x.PackNextAt = time.Time{}
		s.problem(ctx, sess, o, store.Problem{Code: problemPackParts}, stepPack)

		return
	}

	for i := range rows {
		if rows[i].LabelSentAt.IsZero() {
			rows[i].LabelNextAt = now.Add(labelsAfter)
		}
	}

	s.write(ctx, func(q *store.Queries) error { return q.ReplaceParcels(ctx, o.x.OrderID, rows) })

	o.x.PackedAt, o.x.PackNextAt = now, time.Time{}
	o.added = append(o.added, store.ExchangeHistory{At: now, State: o.x.State, Code: "packing_confirmed"})
	s.clearProblem(ctx, sess, o)
	s.save(ctx, o)
}

// checkPacking reads the packing of the orders due.
func (s *Service) checkPacking(ctx context.Context, sess *session, at time.Time) error {
	due, err := s.opts.Store.Read().DuePackChecks(ctx, at, batch)
	if err != nil {
		return err
	}

	for _, x := range due {
		o, err := s.load(ctx, x)
		if err != nil {
			return err
		}

		s.packStep(ctx, sess, o)
	}

	return nil
}

// labelsLater sets when a parcel's labels task is read next: every 10 seconds during the first 5 minutes after it was asked,
// then every 5 minutes.
func (s *Service) labelsLater(p *store.Parcel) {
	now := s.opts.Clock.Now()

	wait := labelSlow
	if p.LabelAskedAt.IsZero() || now.Sub(p.LabelAskedAt) < labelFastFor {
		wait = labelFast
	}

	p.LabelNextAt = now.Add(wait)
}

// checkLabels asks for and reads the labels of the parcels due, one order at a time.
func (s *Service) checkLabels(ctx context.Context, sess *session, at time.Time) error {
	due, err := s.opts.Store.Read().DueLabelChecks(ctx, at, batch)
	if err != nil {
		return err
	}

	var seen []int64

	for _, p := range due {
		if slices.Contains(seen, p.OrderID) {
			continue
		}

		seen = append(seen, p.OrderID)

		x, err := s.opts.Store.Read().ExchangeOrder(ctx, p.OrderID)
		if err != nil {
			return err
		}

		o, err := s.load(ctx, x)
		if err != nil {
			return err
		}

		s.labelStep(ctx, sess, o, at)
	}

	return nil
}

// labelTypeOf is the kind of label the order's warehouse wants (the marketplace's word for it), empty when it names none.
func (s *Service) labelTypeOf(ctx context.Context, o *order) string {
	list, err := s.opts.Store.Read().Warehouses(ctx, o.row.AccountID)
	if err != nil {
		return ""
	}

	for _, w := range list {
		if w.Number == o.row.WarehouseNumber {
			return w.LabelType
		}
	}

	return ""
}

// labelStep is step 3's labels for one order: for each parcel whose label is not sent yet the label is asked for (one ticket
// for each parcel, so that each parcel gets the file of its own part), then the ticket is read until the label is made, and
// the label is attached to its parcel at the fulfilment centre.
func (s *Service) labelStep(ctx context.Context, sess *session, o *order, at time.Time) {
	rows, err := s.opts.Store.Read().ParcelsOf(ctx, o.x.OrderID)
	if err != nil {
		s.opts.Log.Error("the parcels could not be read", "order", o.x.OrderID, "err", err)

		return
	}

	adapter, conn, err := s.conn(ctx, o, jobVerdicts)
	if err != nil {
		return
	}

	maker, ok := part[marketplace.LabelMaker](adapter, marketplace.LabelsFetch, conn.Test)
	if !ok {
		return
	}

	for i := range rows {
		if !rows[i].LabelSentAt.IsZero() || rows[i].LabelNextAt.IsZero() || rows[i].LabelNextAt.After(at) {
			continue
		}

		var stop bool

		if rows[i].Ticket == "" {
			stop = s.askLabel(ctx, sess, o, maker, conn, &rows[i])
		} else {
			stop = s.readLabel(ctx, sess, o, maker, conn, &rows[i])
		}

		if stop {
			return
		}
	}

	s.labelsDone(ctx, sess, o)
}

// saveParcel writes one parcel of an order.
func (s *Service) saveParcel(ctx context.Context, p store.Parcel) {
	s.write(ctx, func(q *store.Queries) error { return q.SaveParcel(ctx, p) })
}

// askLabel asks for the label of one parcel and keeps the ticket on it. It says whether the order's labels stop here (a
// problem was recorded).
func (s *Service) askLabel(ctx context.Context, sess *session, o *order, maker marketplace.LabelMaker, conn marketplace.Conn, p *store.Parcel) bool {
	ticket, err := maker.AskLabels(ctx, conn, []string{p.MpNumber}, s.labelTypeOf(ctx, o))
	if err != nil {
		if s.marketplaceProblem(ctx, sess, o, stepLabels, err) {
			return true
		}

		s.labelsLater(p)
		s.saveParcel(ctx, *p)

		return false
	}

	now := s.opts.Clock.Now()
	p.Ticket, p.LabelAskedAt, p.LabelTries, p.LabelNextAt = ticket, now, p.LabelTries+1, now.Add(labelFast)
	s.saveParcel(ctx, *p)

	// The history says once that the labels were asked for, not once for each parcel: it is told at the first parcel's.
	if p.Number == 1 {
		s.write(ctx, func(q *store.Queries) error {
			return q.InsertExchangeHistory(ctx, o.x.OrderID, store.ExchangeHistory{At: now, State: o.x.State, Code: "labels_asked"})
		})
	}

	return false
}

// readLabel reads the labels task of one parcel. A label that is made is attached at the fulfilment centre to its parcel; one
// the marketplace could not make is a problem in its words. It says whether the order's labels stop here.
func (s *Service) readLabel(ctx context.Context, sess *session, o *order, maker marketplace.LabelMaker, conn marketplace.Conn, p *store.Parcel) bool {
	state, err := maker.ReadLabels(ctx, conn, p.Ticket)
	if err != nil {
		if s.marketplaceProblem(ctx, sess, o, stepLabels, err) {
			return true
		}

		s.labelsLater(p)
		s.saveParcel(ctx, *p)

		return false
	}

	switch {
	case state.Failed:
		s.problem(ctx, sess, o, store.Problem{Code: problemLabelsNotMade, MpCode: "labels_task_failed", Words: state.Words}, stepLabels)

		return true
	case !state.Ready:
		s.labelsLater(p)
		s.saveParcel(ctx, *p)

		return false
	}

	if words, unmade := state.Unmade[p.MpNumber]; unmade {
		s.problem(ctx, sess, o, store.Problem{Code: problemLabelsNotMade, MpCode: "label_not_made", Words: words}, stepLabels)

		return true
	}

	j := slices.IndexFunc(state.Labels, func(l marketplace.Label) bool { return l.Number == p.MpNumber })
	if j < 0 {
		s.problem(ctx, sess, o, store.Problem{Code: problemLabelsNotMade, MpCode: "label_missing"}, stepLabels)

		return true
	}

	if err := sess.fc.PutLabel(ctx, o.x.FcID, int(p.Number), state.Labels[j].ContentType, state.Labels[j].Data); err != nil {
		if !s.failed(ctx, err) {
			s.problem(ctx, sess, o, fcProblem(stepLabels, err), stepLabels)
		}

		return true
	}

	p.LabelPrint, p.LabelSentAt, p.LabelNextAt = state.Labels[j].Print(), s.opts.Clock.Now(), time.Time{}
	s.saveParcel(ctx, *p)

	return false
}

// labelsDone moves the order on when every parcel has its label.
func (s *Service) labelsDone(ctx context.Context, sess *session, o *order) {
	rows, err := s.opts.Store.Read().ParcelsOf(ctx, o.x.OrderID)
	if err != nil || len(rows) == 0 || slices.ContainsFunc(rows, func(p store.Parcel) bool { return p.LabelSentAt.IsZero() }) {
		return
	}

	s.clearProblem(ctx, sess, o)

	// An order whose fulfilment centre reported it packed before its codes were sent (it takes none) is «packed» until now.
	if o.x.State == stateReserved || o.x.State == stateCodesAccepted || o.x.State == statePacked ||
		(o.x.State == stateCodesChecking && o.x.CodesState == codesNotTaken) {
		s.note(o, stateLabelSent, "label_sent")
	} else {
		o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "label_sent"})
	}

	s.save(ctx, o)
}

// resetLabels makes the labels of an order be asked for again (the owner pressed «Передать заново»).
func (s *Service) resetLabels(ctx context.Context, o *order) error {
	rows, err := s.opts.Store.Read().ParcelsOf(ctx, o.x.OrderID)
	if err != nil {
		return err
	}

	now := s.opts.Clock.Now()

	for i := range rows {
		if rows[i].LabelSentAt.IsZero() {
			rows[i].Ticket, rows[i].LabelNextAt = "", now
			s.saveParcel(ctx, rows[i])
		}
	}

	return nil
}
