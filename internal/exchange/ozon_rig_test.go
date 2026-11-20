package exchange_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeozon "github.com/prankyman/wyo/internal/fakes/ozon"
	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// ozRig is the exchange between a fake Ozon and a fake fulfilment centre, both in the test's own process, with one Ozon account
// whose two-field key is in the secret store and whose warehouse and marketplace are matched at the fulfilment centre. It is
// the rig of the Wildberries tests (rig_test.go) with Ozon's fake in place of Wildberries': every step of the fulfilment
// centre's part, every job and every reading of the fulfilment centre is the same function.
type ozRig struct {
	*rig
	oz  *fakekit.Control
	key marketplace.Key
}

// What the fake Ozon's seller has: three products and one FBS warehouse. The shirt takes a marking code, the socks take none,
// the cap may take one.
const (
	ozSellerWarehouse = 1001
	ozShirt           = 2001
	ozSocks           = 2002
	ozCap             = 2003
	ozCoat            = 2004
)

// Two made-up marking codes, each with the group separator a scanner reads, and the part of each that no output may hold.
const (
	ozCodeA = markingCode
	ozCodeB = "0104600000000024215mnopqrstuvwx\u001d93Qw3r"
)

// ozWritePaths are the methods of Ozon the wyo server writes with (every other POST of Ozon is a reading): a lost call is
// made of these, and a test counts these.
var ozWritePaths = []string{
	"/v6/fbs/posting/product/exemplar/set", "/v4/posting/fbs/ship", "/v3/posting/fbs/package-label/create",
	"/v1/carriage/create", "/v1/carriage/set-postings", "/v1/carriage/approve", "/v1/carriage/cancel",
}

// ozonCancelPaths are the methods of Ozon that cancel a posting or take a product off it: the wyo server never calls them.
var ozonCancelPaths = []string{"/v2/posting/fbs/cancel", "/v2/posting/fbs/product/cancel"}

// ozWrite says whether a request is a write of the wyo server: a writing method of Ozon, or any call of the neutral API that
// is not a reading.
func ozWrite(req *http.Request) bool {
	path := strings.TrimPrefix(req.URL.Path, "/"+fakeozon.Host)

	return slices.Contains(ozWritePaths, path) || (req.Method != http.MethodGet && strings.HasPrefix(path, "/api/v1/"))
}

// ozSetup is what the key is set up with at the fake Ozon; a test changes it before the rig is made.
type ozSetup map[string]any

// newOzonRig makes the rig; change adjusts the setup of the fake (its warehouses, its products).
func newOzonRig(t *testing.T, change ...func(ozSetup)) *ozRig {
	t.Helper()

	r := &ozRig{}
	brk := &breaker{isWrite: ozWrite}

	r.rig = newRigOn(t, false, brk, func(clk *clock.Fixed, brk *breaker) http.Handler {
		return brk.wrap(fakeozon.New(fakeozon.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler())
	}, func(kit *synckit.Kit) marketplace.Adapter {
		return ozon.New(ozon.Options{TestBase: kit.URL, Clock: kit.Clock})
	})

	name := strings.ReplaceAll(t.Name(), "/", "-")
	r.key = marketplace.Key{"client_id": "ozc-" + name, "api_key": "ozon-key-" + name}
	r.oz = fakekit.NewControl(r.kit.URL+"/"+fakeozon.Host, r.key["api_key"], r.kit.HTTP)

	setup := ozSetup{
		"client_id": r.key["client_id"], "seller_name": "Продавец", "expires_at": synckit.Noon().Add(60 * 24 * time.Hour),
		"roles": []map[string]any{{"name": "Admin", "methods": []string{}}},
		"warehouses": []map[string]any{
			{"id": ozSellerWarehouse, "name": "Склад FBS", "label_type": "BIG"},
		},
		"products": []map[string]any{
			{"sku": ozShirt, "offer_id": "SHIRT", "name": "Рубашка", "barcodes": []string{"4600001"}, "mark": "needed"},
			{"sku": ozSocks, "offer_id": "SOCKS", "name": "Носки", "barcodes": []string{"4600002"}},
			{"sku": ozCap, "offer_id": "CAP", "name": "Кепка", "barcodes": []string{"4600003"}, "mark": "possible"},
			{"sku": ozCoat, "offer_id": "COAT", "name": "Пальто", "barcodes": []string{"4600004"}},
		},
	}

	for _, c := range change {
		c(setup)
	}

	do(t, r.oz, http.MethodPost, "/__control/setup", setup, nil)
	do(t, r.fc, http.MethodPost, "/__control/setup", map[string]any{
		"marketplaces": []map[string]string{{"code": "wildberries", "name": "Wildberries"}, {"code": "ozon", "name": "Ozon"}},
	}, nil)

	r.addOzonAccount(t, setup)

	return r
}

// addOzonAccount keeps the key in the secret store, adds the account the way the accounts service does, reads its warehouses and
// matches its marketplace and its warehouse at the fulfilment centre.
func (r *ozRig) addOzonAccount(t *testing.T, setup ozSetup) {
	t.Helper()

	now := r.kit.Clock.Now()

	if err := r.kit.Vault.Put(t.Context(), "account.1.key", marketplace.PackKey(ozon.New(ozon.Options{}).Describe().KeyFields, r.key)); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	err := r.kit.Store.Do(t.Context(), func(q *store.Queries) error {
		var err error

		r.account, err = q.InsertAccount(t.Context(), store.Account{
			Marketplace: ozon.Code, Name: "Ozon shop", State: "working", SellerProfileID: r.key["client_id"], SecretName: "account.1.key",
			KeyLast4: "…" + r.key["api_key"][len(r.key["api_key"])-4:], KeyExpiresAt: synckit.Noon().Add(60 * 24 * time.Hour),
			DeadlineHours: 48, LastSuccessAt: now, CreatedAt: now,
		})
		if err != nil {
			return err
		}

		if err := q.PutMarketplaceMatch(t.Context(), r.account.ID, "ozon"); err != nil {
			return err
		}

		return q.PutWarehouseMatch(t.Context(), store.WarehouseMatch{
			AccountID: r.account.ID, WarehouseNumber: strconv.Itoa(ozSellerWarehouse), FcWarehouse: "main",
		})
	})
	if err != nil {
		t.Fatalf("the Ozon account: %v", err)
	}

	if err := r.engine.RunWarehousesOf(t.Context(), r.account.ID); err != nil {
		t.Fatalf("RunWarehousesOf: %v", err)
	}
}

// ozLine is a line of a posting: a product by its SKU and the units.
type ozLine struct{ sku, quantity int }

// ozShipBy is the deadline Ozon gives every posting of the tests unless a test says another: the evening of the next day.
func ozShipBy() time.Time { return synckit.Noon().Add(30 * time.Hour).UTC() }

// posting makes an FBS posting awaiting packaging at the fake Ozon and returns its number.
func (r *ozRig) posting(t *testing.T, n int, lines ...ozLine) string {
	t.Helper()

	return r.postingWith(t, n, nil, lines...)
}

// postingWith is posting with more fields of the posting (is_multibox).
func (r *ozRig) postingWith(t *testing.T, n int, extra map[string]any, lines ...ozLine) string {
	t.Helper()

	number := fmt.Sprintf("1100%04d-0001-1", n)
	products := make([]map[string]any, 0, len(lines))

	for _, l := range lines {
		products = append(products, map[string]any{"sku": l.sku, "offer_id": ozArticle(l.sku), "name": "Товар " + ozArticle(l.sku), "quantity": l.quantity})
	}

	spec := map[string]any{
		"posting_number": number, "order_number": strings.TrimSuffix(number, "-1"), "warehouse_id": ozSellerWarehouse,
		"shipment_date": ozShipBy(), "products": products,
	}

	maps.Copy(spec, extra)
	do(t, r.oz, http.MethodPost, "/__control/postings", spec, nil)

	return number
}

// ozArticle is the article of a product the setup made.
func ozArticle(sku int) string {
	return map[int]string{ozShirt: "SHIRT", ozSocks: "SOCKS", ozCap: "CAP", ozCoat: "COAT"}[sku]
}

// walkPosting is the posting the walks use: two lines, a shirt in two units (it takes a code on each) and a pair of socks.
func (r *ozRig) walkPosting(t *testing.T, n int) string {
	t.Helper()

	return r.posting(t, n, ozLine{ozShirt, 2}, ozLine{ozSocks, 1})
}

// send reads the posting and sends it to the fulfilment centre.
func (r *ozRig) send(t *testing.T, number string) store.ExchangeOrder {
	t.Helper()

	r.orders(t)

	return r.exchanged(t, number)
}

// reserved sends the posting and plays the fulfilment centre reserving it.
func (r *ozRig) reserved(t *testing.T, number string) store.ExchangeOrder {
	t.Helper()

	x := r.send(t, number)
	r.step(t, x.FcID, "reserve")
	r.events(t)

	return r.exchanged(t, number)
}

// ozUnit is a unit the fulfilment centre packed: the line it belongs to, the marking code scanned on it ("" for none) and the
// parcel it lies in (0 is the first).
type ozUnit struct {
	line   int
	code   string
	parcel int
}

// shirtsAndSocks packs the walk posting's three units in the parcels the numbers name: the two shirts take the codes.
func shirtsAndSocks(shirtParcels [2]int, socksParcel int) []ozUnit {
	return []ozUnit{{1, ozCodeA, shirtParcels[0]}, {1, ozCodeB, shirtParcels[1]}, {2, "", socksParcel}}
}

// packWith plays the fulfilment centre packing the order with the units, and reads the event.
func (r *ozRig) packWith(t *testing.T, number string, units ...ozUnit) store.ExchangeOrder {
	t.Helper()

	list := make([]map[string]any, 0, len(units))

	for _, u := range units {
		item := map[string]any{"line": u.line}
		if u.code != "" {
			item["marking_code"] = u.code
		}

		if u.parcel != 0 {
			item["parcel"] = u.parcel
		}

		list = append(list, item)
	}

	do(t, r.fc, http.MethodPost, "/__control/orders/"+r.exchanged(t, number).FcID+"/pack", map[string]any{"units": list}, nil)
	r.events(t)

	return r.exchanged(t, number)
}

// tick moves the clock and runs what is due the way the schedulers do: the states of the open orders every 5 minutes, the events,
// and the verdicts, the packing, the labels and the waiting hand-overs. A job that meets a lost call is simply run again.
func (r *rig) tick(t *testing.T, wait time.Duration) {
	t.Helper()

	r.kit.Clock.Advance(wait)

	_ = r.svc.RunStates(t.Context())
	_ = r.svc.RunEvents(t.Context())
	_ = r.svc.RunVerdicts(t.Context())
}

// until runs the jobs, 20 seconds apart, until the condition holds; it fails the test when it does not within 60 rounds, which is
// 20 minutes of the exchange's own time.
func (r *rig) until(t *testing.T, what string, done func() bool) {
	t.Helper()

	for range 60 {
		if done() {
			return
		}

		r.tick(t, 20*time.Second)
	}

	if !done() {
		t.Fatalf("after 20 minutes still not: %s", what)
	}
}

// untilState runs the jobs until the order is in the state.
func (r *rig) untilState(t *testing.T, number, state string) store.ExchangeOrder {
	t.Helper()

	r.until(t, "the order is "+state, func() bool { return r.exchanged(t, number).State == state })

	return r.exchanged(t, number)
}

// untilLabels runs the jobs until every parcel of the order has its label sent.
func (r *rig) untilLabels(t *testing.T, number string) []store.Parcel {
	t.Helper()

	r.until(t, "every parcel has its label", func() bool {
		rows := r.parcels(t, number)

		return len(rows) > 0 && !slices.ContainsFunc(rows, func(p store.Parcel) bool { return p.LabelSentAt.IsZero() })
	})

	return r.parcels(t, number)
}

// parcels are the order's parcels as the wyo server keeps them.
func (r *rig) parcels(t *testing.T, number string) []store.Parcel {
	t.Helper()

	rows, err := r.kit.Store.Read().ParcelsOf(t.Context(), r.exchanged(t, number).OrderID)
	if err != nil {
		t.Fatalf("ParcelsOf: %v", err)
	}

	return rows
}

// problem is the problem the order shows.
func (r *rig) problem(t *testing.T, number string) store.Problem {
	t.Helper()

	return r.exchanged(t, number).Problem
}

// ozCalls are the calls the fake Ozon received, in order.
func (r *ozRig) ozCalls(t *testing.T) []fakekit.Call {
	t.Helper()

	calls, err := r.oz.Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	return calls
}

// ozWrites are the writing calls the fake Ozon received and accepted, as their paths, in order: a refused write was not made.
func (r *ozRig) ozWrites(t *testing.T) []string {
	t.Helper()

	var out []string

	for _, c := range r.ozCalls(t) {
		if slices.Contains(ozWritePaths, c.Path) && c.Status < http.StatusMultipleChoices {
			out = append(out, c.Path)
		}
	}

	return out
}

// ozCallsTo are the calls the fake Ozon received at a path, accepted or not.
func (r *ozRig) ozCallsTo(t *testing.T, path string) []fakekit.Call {
	t.Helper()

	var out []fakekit.Call

	for _, c := range r.ozCalls(t) {
		if c.Path == path {
			out = append(out, c)
		}
	}

	return out
}

// ozCancels are the calls of the fake Ozon that would cancel a posting or take a product off it: there are none, ever.
func (r *ozRig) ozCancels(t *testing.T) []string {
	t.Helper()

	var out []string

	for _, c := range r.ozCalls(t) {
		if slices.Contains(ozonCancelPaths, c.Path) {
			out = append(out, c.Method+" "+c.Path)
		}
	}

	return out
}

// ozControl drives the fake Ozon; the posting's plan of the marking check, of the report and so on.
func (r *ozRig) ozControl(t *testing.T, path string, in any) {
	t.Helper()
	do(t, r.oz, http.MethodPost, path, in, nil)
}

// exemplarCodes are the marking codes the fake Ozon holds for the posting's units, by the product's SKU, one for each exemplar
// that has one; an exemplar without a code is a missing entry.
func (r *ozRig) exemplarCodes(t *testing.T, posting string) map[int64][]string {
	t.Helper()

	var got struct {
		Products []struct {
			SKU       int64 `json:"product_id"`
			Exemplars []struct {
				Marks []struct {
					Mark string `json:"mark"`
				} `json:"marks"`
			} `json:"exemplars"`
		} `json:"products"`
	}

	do(t, r.oz, http.MethodGet, "/__control/exemplars?posting_number="+posting, nil, &got)

	out := map[int64][]string{}

	for _, p := range got.Products {
		for _, e := range p.Exemplars {
			for _, m := range e.Marks {
				out[p.SKU] = append(out[p.SKU], m.Mark)
			}
		}
	}

	return out
}

// ozPost calls a method of the fake Ozon with the seller's own key, the way the wyo server does, and decodes the answer.
func (r *ozRig) ozPost(t *testing.T, path string, in, out any) {
	t.Helper()

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, r.kit.URL+"/"+fakeozon.Host+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	req.Header.Set("Api-Key", r.key["api_key"])
	req.Header.Set("Client-Id", r.key["client_id"])
	req.Header.Set("Content-Type", "application/json")

	res, err := r.kit.HTTP.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}

	defer func() { _ = res.Body.Close() }()

	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		t.Fatalf("decode the answer of %s (status %d): %v", path, res.StatusCode, err)
	}
}

// postingState is the status and the substatus the fake Ozon shows the posting with.
func (r *ozRig) postingState(t *testing.T, number string) (status, substatus string) {
	t.Helper()

	var got struct {
		Result struct {
			Status    string `json:"status"`
			Substatus string `json:"substatus"`
		} `json:"result"`
	}

	r.ozPost(t, "/v3/posting/fbs/get", map[string]any{"posting_number": number}, &got)

	return got.Result.Status, got.Result.Substatus
}

// fcLabel is the label file the fake fulfilment centre holds for the order's parcel, as the wyo server sent it.
func (r *rig) fcLabel(t *testing.T, number string, parcel int) (contentType string, scan *string, text string) {
	t.Helper()

	var got fcFile

	do(t, r.fc, http.MethodGet, "/__control/orders/"+r.exchanged(t, number).FcID+"/parcels/"+strconv.Itoa(parcel)+"/label", nil, &got)

	return got.ContentType, got.ScanCode, got.text(t)
}

// fcFile is a file the sender sent to the fulfilment centre, as its control calls show it.
type fcFile struct {
	ContentType string  `json:"content_type"`
	ScanCode    *string `json:"scan_code"`
	Data        string  `json:"data"`
}

// text is the file's bytes as text.
func (f fcFile) text(t *testing.T) string {
	t.Helper()

	raw, err := base64.StdEncoding.DecodeString(f.Data)
	if err != nil {
		t.Fatalf("the file is not base64: %v", err)
	}

	return string(raw)
}

// fcDocuments are the documents the fake fulfilment centre holds for a hand-over, as the wyo server sent them.
func (r *rig) fcDocuments(t *testing.T, handoverID string) []fcDocument {
	t.Helper()

	var got struct {
		Documents []fcDocument `json:"documents"`
	}

	do(t, r.fc, http.MethodGet, "/__control/handovers/"+handoverID+"/documents", nil, &got)

	return got.Documents
}

// fcDocument is a document of a hand-over as the sender sent it.
type fcDocument struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	fcFile
}

// fcOrderProblem is the problem the fulfilment centre shows on the order.
func (r *rig) fcOrderProblem(t *testing.T, number string) *fcapi.Problem {
	t.Helper()

	return r.fcOrder(t, r.exchanged(t, number).FcID).Problem
}

// ozAssertNothingSecretLeft searches every place a key or a marking code could reach (rule R4): the log, every column of the
// journal, the fake Ozon's call list (the Ozon key and the fulfilment centre's key, and the codes, which Ozon's fake never lists)
// and the fake fulfilment centre's (the Ozon key and the codes). A marking code goes to Ozon in the body of one call, which the
// fake Ozon keeps out of its call list; it is never in anything else.
func (r *ozRig) ozAssertNothingSecretLeft(t *testing.T) {
	t.Helper()

	secrets := []string{r.key["api_key"], r.fcKey, ozCodeA, ozCodeB, "abcdefghijkl", "mnopqrstuvwx"}
	logs, rows := r.kit.Logs.String(), r.kit.Rows(t)

	for _, secret := range secrets {
		if strings.Contains(logs, secret) {
			t.Fatalf("a secret or a marking code reached the log: %q", secret)
		}

		for _, row := range rows {
			if text := strings.Join([]string{row.AccountName, row.Marketplace, row.Method, row.Path, row.Outcome, row.ErrorCode, row.ErrorText, row.Job}, "\n"); strings.Contains(text, secret) {
				t.Fatalf("a secret or a marking code reached the journal: %+v", row)
			}
		}
	}

	for _, c := range r.fcCalls(t) {
		for _, secret := range []string{r.key["api_key"], ozCodeA, ozCodeB, "abcdefghijkl", "mnopqrstuvwx"} {
			if strings.Contains(callText(c), secret) {
				t.Fatalf("a secret or a marking code reached the fulfilment centre: %s %s", c.Method, c.Path)
			}
		}
	}

	for _, c := range r.ozCalls(t) {
		for _, secret := range []string{r.fcKey, ozCodeA, ozCodeB, "abcdefghijkl", "mnopqrstuvwx"} {
			if strings.Contains(callText(c), secret) {
				t.Fatalf("a key of the fulfilment centre or a marking code is in the fake Ozon's call list: %s %s", c.Method, c.Path)
			}
		}
	}
}

// documentOfKind is the first document of the kind, or nil.
func documentOfKind(docs []fcDocument, kind string) *fcDocument {
	i := slices.IndexFunc(docs, func(d fcDocument) bool { return d.Kind == kind })
	if i < 0 {
		return nil
	}

	return &docs[i]
}

// refuse makes the fake Ozon refuse the next call of a method (the kit's fault «refuse with this answer»), in Ozon's own error shape
// and with Ozon's words: the call is answered as Ozon would answer it, and what it would have done is not done.
func (r *ozRig) refuse(t *testing.T, path, words string) {
	t.Helper()

	err := r.oz.QueueFault(t.Context(), fakekit.Fault{
		Kind: fakekit.FaultRefuse, PathPrefix: path, Status: http.StatusBadRequest, Body: fakeozon.RefusalBody(words),
	})
	if err != nil {
		t.Fatalf("QueueFault: %v", err)
	}
}

// ozCut is the owner taking products off the posting in the Ozon cabinet: each product now has the quantity of the line.
func (r *ozRig) ozCut(t *testing.T, posting string, lines ...ozLine) {
	t.Helper()

	products := make([]map[string]any, 0, len(lines))
	for _, l := range lines {
		products = append(products, map[string]any{"sku": l.sku, "quantity": l.quantity})
	}

	r.ozControl(t, "/__control/postings/cut", map[string]any{"posting_number": posting, "products": products})
}

// ozStatus is the owner (or Ozon) giving the posting a status and a substatus.
func (r *ozRig) ozStatus(t *testing.T, posting, status, substatus string) {
	t.Helper()

	r.ozControl(t, "/__control/postings/status", map[string]any{"posting_number": posting, "status": status, "substatus": substatus})
}

// sentOrder is the order the wyo server sent to the fulfilment centre for the posting, as the fake received it.
func (r *ozRig) sentOrder(t *testing.T, posting string) fcapi.OrderIn {
	t.Helper()

	for _, c := range r.fcCalls(t) {
		if c.Method != http.MethodPost || c.Path != "/api/v1/orders" {
			continue
		}

		var in fcapi.OrderIn
		if err := json.Unmarshal([]byte(c.Body), &in); err != nil {
			t.Fatalf("the order the wyo server sent is not an order: %v", err)
		}

		if in.ExternalId == posting {
			return in
		}
	}

	t.Fatalf("no order for the posting %s was sent to the fulfilment centre", posting)

	return fcapi.OrderIn{}
}

// packed walks the walk posting to the moment its codes are accepted and its packing is reported, with the units the
// fulfilment centre packed.
func (r *ozRig) packed(t *testing.T, n int, units ...ozUnit) string {
	t.Helper()

	number := r.walkPosting(t, n)
	r.reserved(t, number)
	r.packWith(t, number, units...)
	r.untilState(t, number, "codes_accepted")

	return number
}

// shipRequest is the packing report the wyo server made, as the fake Ozon received it: for each package the products by SKU.
type shipRequest struct {
	PostingNumber string `json:"posting_number"`
	Packages      []struct {
		Products []struct {
			SKU      int `json:"product_id"`
			Quantity int `json:"quantity"`
		} `json:"products"`
	} `json:"packages"`
}

// reports are the packing reports the fake Ozon received for the posting.
func (r *ozRig) reports(t *testing.T, posting string) []shipRequest {
	t.Helper()

	var out []shipRequest

	for _, c := range r.ozCallsTo(t, "/v4/posting/fbs/ship") {
		var req shipRequest
		if err := json.Unmarshal([]byte(c.Body), &req); err != nil {
			t.Fatalf("a packing report is not JSON: %v", err)
		}

		if req.PostingNumber == posting {
			out = append(out, req)
		}
	}

	return out
}

// exemplarIDs are the numbers Ozon gave the posting's exemplars, by the product's SKU, in order.
func (r *ozRig) exemplarIDs(t *testing.T, posting string) map[int64][]int64 {
	t.Helper()

	var got struct {
		Products []struct {
			SKU       int64 `json:"product_id"`
			Exemplars []struct {
				ID int64 `json:"exemplar_id"`
			} `json:"exemplars"`
		} `json:"products"`
	}

	do(t, r.oz, http.MethodGet, "/__control/exemplars?posting_number="+posting, nil, &got)

	out := map[int64][]int64{}

	for _, p := range got.Products {
		for _, e := range p.Exemplars {
			out[p.SKU] = append(out[p.SKU], e.ID)
		}
	}

	return out
}

// ownerFills plays the owner in the Ozon cabinet, which fills an exemplar through the very method the wyo server writes codes with:
// every exemplar of the product gets the field (a made-up customs declaration is "absent", a number of boxes goes on the posting).
// The call is a write of the cabinet, not of the wyo server, and the test counts it where it counts the writes.
func (r *ozRig) ownerFills(t *testing.T, posting string, sku int64, boxes int, exemplar map[string]any) {
	t.Helper()

	exemplars := []map[string]any{}

	for _, id := range r.exemplarIDs(t, posting)[sku] {
		e := map[string]any{"exemplar_id": id}
		for k, v := range exemplar {
			e[k] = v
		}

		exemplars = append(exemplars, e)
	}

	body := map[string]any{"posting_number": posting, "products": []map[string]any{{"product_id": sku, "exemplars": exemplars}}}
	if boxes > 0 {
		body["multi_box_qty"] = boxes
	}

	var out map[string]any

	r.ozPost(t, "/v6/fbs/posting/product/exemplar/set", body, &out)
}

// labelFiles are the requests for label files the fake Ozon received: the path of each and whether it carried a key.
func (r *ozRig) labelFiles(t *testing.T) []struct {
	Path    string `json:"path"`
	WithKey bool   `json:"with_key"`
} {
	t.Helper()

	var got struct {
		Files []struct {
			Path    string `json:"path"`
			WithKey bool   `json:"with_key"`
		} `json:"files"`
	}

	do(t, r.oz, http.MethodGet, "/__control/label-files", nil, &got)

	return got.Files
}

// fileRows are the rows of the journal that name a label file.
func (r *ozRig) fileRows(t *testing.T) []string {
	t.Helper()

	var out []string

	for _, row := range r.kit.Rows(t) {
		if strings.Contains(row.Path, "label file") || strings.Contains(row.Path, "ozone") {
			out = append(out, row.Method+" "+row.Path)
		}
	}

	return out
}

// labelled walks a new walk posting to the moment its label is attached.
func (r *ozRig) labelled(t *testing.T, n int) string {
	t.Helper()

	number := r.walkPosting(t, n)
	r.reserved(t, number)
	r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
	r.untilLabels(t, number)

	return number
}

// shipment is the hand-over's one shipment at Ozon: the supply the wyo server keeps for it.
func (r *ozRig) shipment(t *testing.T, h store.Handover) store.Supply {
	t.Helper()

	supplies, err := r.kit.Store.Read().HandoverSupplies(t.Context(), h.ID)
	if err != nil || len(supplies) != 1 {
		t.Fatalf("the hand-over's shipments: %+v, %v", supplies, err)
	}

	return supplies[0]
}

// inShipment lists the postings the fake Ozon shows in the shipment.
func (r *ozRig) inShipment(t *testing.T, shipment string) []string {
	t.Helper()

	id, err := strconv.ParseInt(shipment, 10, 64)
	if err != nil {
		t.Fatalf("the shipment %q is no number: %v", shipment, err)
	}

	var got struct {
		Result []struct {
			PostingNumber string `json:"posting_number"`
		} `json:"result"`
	}

	r.ozPost(t, "/v2/posting/fbs/act/get-postings", map[string]any{"id": id}, &got)

	out := []string{}
	for _, p := range got.Result {
		out = append(out, p.PostingNumber)
	}

	return out
}

// shipmentStatus is the status the fake Ozon gives the shipment.
func (r *ozRig) shipmentStatus(t *testing.T, shipment string) string {
	t.Helper()

	id, err := strconv.ParseInt(shipment, 10, 64)
	if err != nil {
		t.Fatalf("the shipment %q is no number: %v", shipment, err)
	}

	var got struct {
		Status string `json:"status"`
	}

	r.ozPost(t, "/v1/carriage/get", map[string]any{"carriage_id": id}, &got)

	return got.Status
}

// postedPostings are the postings the wyo server gave the shipment in its calls of set-postings, call by call.
func (r *ozRig) postedPostings(t *testing.T) [][]string {
	t.Helper()

	var out [][]string

	for _, c := range r.ozCallsTo(t, "/v1/carriage/set-postings") {
		var req struct {
			PostingNumbers []string `json:"posting_numbers"`
		}

		if err := json.Unmarshal([]byte(c.Body), &req); err != nil {
			t.Fatalf("a call of set-postings is not JSON: %v", err)
		}

		out = append(out, req.PostingNumbers)
	}

	return out
}

// ozCancelled is Ozon (or the buyer) cancelling the posting; afterShip says it was cancelled after it was packed.
func (r *ozRig) ozCancelled(t *testing.T, posting, who string, afterShip bool) {
	t.Helper()

	r.ozControl(t, "/__control/postings/status", map[string]any{
		"posting_number": posting, "status": "cancelled", "cancellation_type": who, "cancelled_after_ship": afterShip,
	})
}

// drive walks one Ozon posting to the end as the schedulers and the fulfilment centre would, 20 seconds at a time: it runs the
// exchange's jobs (a job that fails is simply run again at the next beat) and plays the fulfilment centre's part whenever the wyo
// server has done what that part waits for; the wyo server is started again over the same database after the lost call — it reserves, packs the posting's three units in two parcels, asks for the hand-over's
// documents and hands the parcels over. It fails the test when the walk does not end in 60 minutes.
func (r *ozRig) drive(t *testing.T, number string) { r.driveUntil(t, number, nil) }

// driveUntil is drive that also ends, successfully, when stop says so.
func (r *ozRig) driveUntil(t *testing.T, number string, stop func() bool) {
	t.Helper()

	packed, asked, handedOver, restarted := false, false, false, false

	for range 180 {
		if stop != nil && stop() {
			return
		}

		r.kit.Clock.Advance(20 * time.Second)

		// A job that meets a lost call returns what the scheduler would log; the next beat runs it again.
		_ = r.svc.RunOrders(t.Context())
		_ = r.svc.RunStates(t.Context())
		_ = r.svc.RunEvents(t.Context())
		_ = r.svc.RunVerdicts(t.Context())

		// The call that was lost took the wyo server down: it starts again over the same database.
		if r.lostACall() && !restarted {
			r.restart()

			restarted = true
		}

		x, sent := r.maybeExchanged(t, number)
		if !sent || x.FcID == "" {
			continue
		}

		fc := r.fcOrder(t, x.FcID)

		switch {
		case fc.Status == "accepted":
			r.step(t, x.FcID, "reserve")
		case fc.Status == "reserved" && !packed:
			r.packWith(t, number, ozUnit{1, ozCodeA, 1}, ozUnit{2, "", 1}, ozUnit{1, ozCodeB, 2})

			packed = true
		case x.State == "label_sent" && !asked && fc.HandoverId != nil:
			do(t, r.fc, "POST", "/__control/handovers/"+*fc.HandoverId+"/request-documents", nil, nil)

			asked = true
		case asked && !handedOver && r.handoverDocumentsSent(t):
			r.step(t, x.FcID, "hand-over")

			handedOver = true
		case x.State == "handed_over" && r.handoverCompleted(t):
			return
		}
	}

	x, _ := r.maybeExchanged(t, number)
	t.Fatalf("the walk did not end: the order is %+v", x)
}

// ozSorted is a list sorted, so that two lists of writes compare as the counts of each write, not in the order they came: a lost
// call is made again later, and the writes of two parcels may then come the other way round.
func ozSorted(list []string) []string {
	out := slices.Clone(list)
	slices.Sort(out)

	return out
}
