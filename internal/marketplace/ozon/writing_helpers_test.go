package ozon_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// The products of the tests of the writing half, besides the three every key has: one that needs a marking code (and so takes
// one), one that only may take one, and one that needs a customs declaration number.
const (
	skuPlain   = 2002
	skuMarked  = 2004
	skuMayMark = 2005
	skuCustoms = 2006
)

// withWriting adds to the key's catalogue the products that ask for something before their posting can be packed.
func withWriting(s setup) {
	products, _ := s["products"].([]map[string]any)

	s["products"] = append(products,
		map[string]any{"sku": skuMarked, "offer_id": "MARKED", "name": "Товар с кодом", "mark": "needed"},
		map[string]any{"sku": skuMayMark, "offer_id": "MAYMARK", "name": "Товар, где код возможен", "mark": "possible"},
		map[string]any{"sku": skuCustoms, "offer_id": "CUSTOMS", "name": "Товар с декларацией", "gtd_needed": true},
	)
}

// The marking codes of the tests, as the scanner gives them, with the group separator (character 29) between the parts.
const (
	codeA = "0104630568317423215EirD_orEif7X\x1d91EE12\x1d92VygELprJ87wrc/qF/Vq/KEFUVpZm4vDPQt2kqUw308Y="
	codeB = "0104630568317430215Zq2k9Wn1xA3\x1d91EE12\x1d92Jf8LpRs77wrc/qF/Vq/KEFUVpZm4vDPQt2kqUw308Y="
)

// asRefusal says the error is a refusal of the marketplace and returns it.
func asRefusal(t *testing.T, err error) *marketplace.Refusal {
	t.Helper()

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("the error %v is no refusal of the marketplace", err)
	}

	return refusal
}

// calls lists the calls the fake received for the key whose path ends with the suffix.
func (r *rig) calls(t *testing.T, key marketplace.Key, suffix string) []fakekit.Call {
	t.Helper()

	list, err := fakekit.NewControl(r.kit.URL, key["api_key"], r.kit.HTTP).Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	var found []fakekit.Call

	for _, c := range list {
		if strings.HasSuffix(c.Path, suffix) {
			found = append(found, c)
		}
	}

	return found
}

// get asks the fake for a control answer.
func (r *rig) get(t *testing.T, key marketplace.Key, path string, out any) {
	t.Helper()

	if err := fakekit.NewControl(r.kit.URL, key["api_key"], r.kit.HTTP).Do(t.Context(), http.MethodGet, path, nil, out); err != nil {
		t.Fatalf("control GET %s: %v", path, err)
	}
}

// codesHeld are the marking codes the fake holds on the posting's exemplars, by SKU, in the order of the exemplars ("" for an
// exemplar that holds none).
func (r *rig) codesHeld(t *testing.T, key marketplace.Key, number string) map[int64][]string {
	t.Helper()

	var held ozonapi.V6FbsPostingProductExemplarCreateOrGetV6Response

	r.get(t, key, "/__control/exemplars?posting_number="+number, &held)

	out := map[int64][]string{}

	for _, p := range *held.Products {
		for _, e := range *p.Exemplars {
			code := ""

			for _, m := range *e.Marks {
				if *m.MarkType == "mandatory_mark" {
					code = *m.Mark
				}
			}

			out[*p.ProductId] = append(out[*p.ProductId], code)
		}
	}

	return out
}

// packedPosting makes a posting of the lines, sends its codes when it needs any, and reports its packing in one parcel: it
// stands for the walk up to the hand-over. It returns the number of the posting.
func (r *rig) packedPosting(t *testing.T, key marketplace.Key, conn marketplace.Conn, number string, warehouse int, products ...map[string]any) {
	t.Helper()

	r.posting(t, key, number, warehouse, products...)

	var parcel marketplace.Parcel

	for _, p := range products {
		sku, okSKU := p["sku"].(int)
		quantity, okQuantity := p["quantity"].(int)

		if !okSKU || !okQuantity {
			t.Fatalf("the product %v has no whole sku and quantity", p)
		}

		parcel.Lines = append(parcel.Lines, marketplace.ParcelLine{ItemID: strconv.Itoa(sku), Quantity: quantity})
	}

	parts, err := r.adapter.Pack(t.Context(), conn, number, []marketplace.Parcel{parcel})
	if err != nil || len(parts) != 1 {
		t.Fatalf("Pack(%s) = %+v, %v", number, parts, err)
	}
}

// plan commands the fake for the key's seller.
func (r *rig) plan(t *testing.T, key marketplace.Key, path string, in any) {
	t.Helper()

	r.control(t, key, path, in)
}

// stubRig is the adapter in front of a handler of the test's own, which answers the way the test says and keeps what it was
// sent. The key is made up.
func stubRig(t *testing.T, handler http.Handler) (*rig, marketplace.Conn) {
	t.Helper()

	kit := synckit.New(t, handler)
	r := &rig{kit: kit, adapter: ozon.New(ozon.Options{TestBase: kit.URL, Clock: kit.Clock})}

	return r, r.conn(t, marketplace.Key{"client_id": "500001", "api_key": "stub-key"})
}

// answer writes a JSON answer.
func answer(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(v)
}

// unitsOf gives a marking code to each unit of the posting's lines (the code of the unit's place in the posting).
func unitsOf(order marketplace.Order) []marketplace.UnitCode {
	var units []marketplace.UnitCode

	for _, l := range order.Lines {
		for i := range l.Quantity {
			units = append(units, marketplace.UnitCode{ItemID: l.ProductID, Code: "01046305683174232150" + strconv.Itoa(len(units)) + strconv.Itoa(i) + "\x1d91EE12"})
		}
	}

	return units
}

// orderOf reads one posting as the adapter lists it among the new ones.
func (r *rig) orderOf(t *testing.T, conn marketplace.Conn, number string) marketplace.Order {
	t.Helper()

	list, err := r.adapter.NewOrders(t.Context(), conn)
	if err != nil {
		t.Fatalf("NewOrders: %v", err)
	}

	for _, o := range list {
		if o.Number == number {
			return o
		}
	}

	t.Fatalf("the posting %s is not among the new ones", number)

	return marketplace.Order{}
}

// giveVerdict makes Ozon's check of the posting's codes end the way the kind says: the plan is for the next sending of the
// codes, so the codes are sent again.
func (r *rig) giveVerdict(t *testing.T, key marketplace.Key, number string, kind marketplace.VerdictKind) {
	t.Helper()

	plan := map[string]any{"posting_number": number}

	switch kind {
	case marketplace.VerdictAccepted:
	case marketplace.VerdictPending:
		plan["in_process_readings"] = 1
	case marketplace.VerdictRefused:
		plan["error_sets"], plan["error_codes"] = 1, []string{"MARK_NOT_FOUND"}
	case marketplace.VerdictAgain:
		plan["failed_sets"] = 1
	case marketplace.VerdictNotRequired, marketplace.VerdictUnfinished, marketplace.VerdictMissing:
		t.Fatalf("Ozon cannot be told to give the verdict %s", kind)
	}

	r.control(t, key, "/__control/postings/exemplar-check", plan)

	conn := r.conn(t, key)
	if sent, err := r.adapter.ReportProductCodes(t.Context(), conn, number, marketplace.IdentifierMarking, unitsOf(r.orderOf(t, conn, number))); err != nil || !sent {
		t.Fatalf("the codes of %s were not sent again: %v, %v", number, sent, err)
	}
}

// synckitNoon is the moment the fake's clock stands at.
func synckitNoon() time.Time { return synckit.Noon() }

// asNeeds says the error is a *marketplace.NeedsIdentifier and gives it.
func asNeeds(err error, into **marketplace.NeedsIdentifier) bool { return errors.As(err, into) }

// itoa writes a whole number as text.
func itoa(n int) string { return strconv.Itoa(n) }

// asWait says the error is a *marketplace.BatchWait and gives it.
func asWait(err error, into **marketplace.BatchWait) bool { return errors.As(err, into) }

// asDocumentsWait says the error is a *marketplace.DocumentsWait and gives it.
func asDocumentsWait(err error, into **marketplace.DocumentsWait) bool { return errors.As(err, into) }

// sameStrings says two lists hold the same texts, in any order.
func sameStrings(a, b []string) bool {
	left, right := slices.Clone(a), slices.Clone(b)
	slices.Sort(left)
	slices.Sort(right)

	return slices.Equal(left, right)
}
