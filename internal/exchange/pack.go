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
	problemPackPartGone    = "pack_part_cancelled"
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
// contents may take either part, for their labels are alike. A part whose contents are not known is given to a parcel only
// when every parcel left has the same contents, so that no label is guessed: otherwise the parcel keeps no number, and the
// owner is told (problemPackParts). A marketplace that gave no parts (the packing was reported before) leaves the order's own
// number on its one parcel.
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

	// A part whose contents the marketplace did not give back in the same words is taken in order, only among parcels alike.
	left := map[string]bool{}

	for i, p := range parcels {
		if numbers[i] == "" {
			left[linesKey(p.Lines)] = true
		}
	}

	if len(left) > 1 {
		return numbers
	}

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

// packProblem records a problem of the packing step. The packing is read no more: «Передать заново» or the next
// order.packing_finished starts it again, so that nothing refused is read, reported or told to the fulfilment centre again
// every 20 seconds.
func (s *Service) packProblem(ctx context.Context, sess *session, o *order, p store.Problem) {
	o.x.PackNextAt = time.Time{}
	s.problem(ctx, sess, o, p, stepPack)
}

// packFailed is a call of the packing step the marketplace did not answer with success: its refusal is a problem and the
// packing is read no more (packProblem); another failure (no connection, a limit) is read again later.
func (s *Service) packFailed(ctx context.Context, sess *session, o *order, err error) {
	o.x.PackNextAt = time.Time{}

	if !s.marketplaceProblem(ctx, sess, o, stepPack, err) {
		s.packLater(ctx, o)
	}
}

// splitParts are the numbers of the parts the marketplace split the order into when the wyo server reported its packing,
// one for each parcel; none when the report did not split it (one parcel keeps the order's own number) or when a part's
// number is not known.
func (s *Service) splitParts(ctx context.Context, o *order) ([]string, error) {
	if o.x.PackReportedAt.IsZero() {
		return nil, nil
	}

	rows, err := s.opts.Store.Read().ParcelsOf(ctx, o.x.OrderID)
	if err != nil {
		return nil, err
	}

	numbers := make([]string, 0, len(rows))
	split := false

	for _, p := range rows {
		if p.MpNumber == "" {
			return nil, nil
		}

		numbers = append(numbers, p.MpNumber)
		split = split || p.MpNumber != o.row.Number
	}

	if !split {
		return nil, nil
	}

	return numbers, nil
}

// packState reads how the order's packing stands at the marketplace. After a report that split the order, the parts are
// read by their own numbers: packed when every part is, failed when one is, with the lines of all; a part cancelled is
// named in gone, with its words in the state, and is never a cancellation of the order (the owner decides; the wyo server
// never guesses one). The order the report split is then no order of its own, whatever it shows. Otherwise the order is read
// by its own number, and an order split before any report (in the marketplace's cabinet) is a cancellation: its parts come
// as orders of their own.
func (s *Service) packState(ctx context.Context, packer marketplace.OrderPacker, conn marketplace.Conn, o *order) (marketplace.PackState, string, error) {
	parts, err := s.splitParts(ctx, o)
	if err != nil {
		return marketplace.PackState{}, "", err
	}

	if len(parts) == 0 {
		state, err := packer.PackState(ctx, conn, o.row.Number)

		// A report was sent and the parts it made are not known (its answer was lost, or a part matched no parcel): the order
		// shown split is that report's split, and stays Split, never a cancellation.
		if !state.Split || o.x.PackSentAt.IsZero() {
			state.Cancelled = state.Cancelled || state.Split
		}

		return state, "", err
	}

	all := marketplace.PackState{Packed: true}

	for _, number := range parts {
		state, err := packer.PackState(ctx, conn, number)
		if err != nil {
			return marketplace.PackState{}, "", err
		}

		if state.Cancelled || state.Split {
			return marketplace.PackState{Words: state.Words}, number, nil
		}

		all.Packed = all.Packed && state.Packed
		if state.Failed && !all.Failed {
			all.Failed, all.Words = true, state.Words
		}

		all.Lines = append(all.Lines, state.Lines...)
	}

	return all, "", nil
}

// partGone is the problem of a part of the order the marketplace cancelled after the report split the order.
func partGone(part string, state marketplace.PackState) store.Problem {
	return store.Problem{Code: problemPackPartGone, Params: map[string]any{"part": part}, Words: state.Words}
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

	if adapter.Describe().CodesInPack {
		// A marketplace of boxes is read again only for its «ready» write.
		o.x.PackNextAt = time.Time{}
		s.readyStep(ctx, sess, o)

		return
	}

	packer, ok := part[marketplace.OrderPacker](adapter, marketplace.OrdersPack, conn.Test)
	if !ok {
		return
	}

	state, gone, err := s.packState(ctx, packer, conn, o)
	if err != nil {
		s.packFailed(ctx, sess, o, err)

		return
	}

	switch {
	case gone != "":
		s.packProblem(ctx, sess, o, partGone(gone, state))
	case state.Split && !state.Cancelled:
		s.packProblem(ctx, sess, o, store.Problem{Code: problemPackParts})
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

	state, gone, err := s.packState(ctx, packer, conn, o)
	if err != nil {
		// The codes step meets the same failure and says what to do about it.
		return false
	}

	switch {
	case gone != "":
		o.x.NextCheckAt = time.Time{}
		s.packProblem(ctx, sess, o, partGone(gone, state))

		return true
	case state.Split && !state.Cancelled:
		o.x.NextCheckAt = time.Time{}
		s.packProblem(ctx, sess, o, store.Problem{Code: problemPackParts})

		return true
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
		s.packProblem(ctx, sess, o, store.Problem{
			Code: problemMarketplace, Params: map[string]any{"step": stepPack}, MpCode: "packing_not_taken", Words: state.Words,
		})

		return
	}

	fc, err := sess.fc.Order(ctx, o.x.FcID)
	if err != nil {
		if !s.failed(ctx, err) {
			s.packProblem(ctx, sess, o, fcProblem(stepPack, err))

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
		s.packProblem(ctx, sess, o, store.Problem{Code: problemPackContents})

		return
	}

	// The sending is recorded, with the next reading, before the report leaves: should the wyo server stop before the answer is
	// recorded, the packing is read again, and the order the report may have split is never taken for a cancellation (packState).
	o.x.PackSentAt = s.opts.Clock.Now()
	s.packLater(ctx, o)

	parts, err := packer.Pack(ctx, conn, o.row.Number, parcels)
	if err != nil {
		s.packFailed(ctx, sess, o, err)

		return
	}

	rows, err := s.parcelRows(ctx, o, len(parcels))
	if err != nil {
		s.opts.Log.Error("the parcels could not be read", "order", o.x.OrderID, "err", err)

		return
	}

	numbers := assign(o, parcels, partsWithLines(ctx, packer, conn, parts))
	for i, number := range numbers {
		rows[i].MpNumber = number
	}

	s.write(ctx, func(q *store.Queries) error { return q.ReplaceParcels(ctx, o.x.OrderID, rows) })

	now := s.opts.Clock.Now()
	o.x.PackReportedAt, o.x.PackTries = now, o.x.PackTries+1
	o.added = append(o.added, store.ExchangeHistory{At: now, State: o.x.State, Code: "packing_reported"})

	if len(parts) > 0 && slices.Contains(numbers, "") {
		s.packProblem(ctx, sess, o, store.Problem{Code: problemPackParts})

		return
	}

	s.clearProblem(ctx, sess, o)
	s.packLater(ctx, o)
}

// partsWithLines are the parts a report made, each with its contents: a part the answer gave without them is read by its own
// number. A part that cannot be read keeps no contents, and assign gives it to no parcel unless the parcels left are alike.
func partsWithLines(ctx context.Context, packer marketplace.OrderPacker, conn marketplace.Conn, parts []marketplace.PackedPart) []marketplace.PackedPart {
	out := slices.Clone(parts)

	for i := range out {
		if len(out[i].Lines) > 0 || len(parts) < 2 {
			continue
		}

		if state, err := packer.PackState(ctx, conn, out[i].Number); err == nil {
			out[i].Lines = state.Lines
		}
	}

	return out
}

// clearSettled clears the problem that a packing the marketplace shows packed, or a label attached, settles. A parcel reopened
// after the packing was reported is settled by neither: its problem stays, here and at the fulfilment centre, until the
// owner's «Передать заново».
func (s *Service) clearSettled(ctx context.Context, sess *session, o *order) {
	if o.x.Problem.Code == problemPackReopened {
		return
	}

	s.clearProblem(ctx, sess, o)
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
		s.packProblem(ctx, sess, o, store.Problem{Code: problemPackParts})

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
	s.clearSettled(ctx, sess, o)
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
