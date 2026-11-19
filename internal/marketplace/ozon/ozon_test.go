package ozon_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/contracttest"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestTheAdapterKeepsTheContractAgainstTheFake(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	// Every order of the suite is a posting of the fake with a number of its own; the suite makes several for one seller.
	var seq atomic.Int64

	newPosting := func(t *testing.T, key map[string]string, products ...map[string]any) string {
		number := fmt.Sprintf("%08d-0001-1", 11111111+seq.Add(1))
		r.posting(t, key, number, warehouseFBS, products...)

		return number
	}

	contracttest.RunAdapterContract(t, contracttest.Fixture{
		Adapter: r.adapter,
		Now:     synckit.Noon(),
		Key:     func(t *testing.T) map[string]string { return r.key(t, withWriting) },
		Conn:    func(t *testing.T, key map[string]string) marketplace.Conn { return r.conn(t, key) },
		NewOrder: func(t *testing.T, key map[string]string) string {
			return newPosting(t, key, product(2001, "SHIRT", 1))
		},
		NewMarkedOrder: func(t *testing.T, key map[string]string) string {
			return newPosting(t, key, product(skuMarked, "MARKED", 2))
		},
		NewSplittableOrder: func(t *testing.T, key map[string]string) string {
			return newPosting(t, key, product(skuPlain, "SOCKS", 2))
		},
		SplitOrder: func(t *testing.T, key map[string]string, number string) {
			r.control(t, key, "/__control/postings/split", map[string]any{
				"posting_number": number,
				"parts": []map[string]any{
					{"posting_number": number + "-a", "products": []map[string]any{{"sku": skuPlain, "quantity": 1}}},
					{"posting_number": number + "-b", "products": []map[string]any{{"sku": skuPlain, "quantity": 1}}},
				},
			})
		},
		NeedsOtherIdentifier: func(t *testing.T, key map[string]string) (string, string) {
			return newPosting(t, key, product(skuCustoms, "CUSTOMS", 1)), marketplace.IdentifierCustoms
		},
		// Ozon has no verdict that says it stopped waiting for its check, so it can never be told to give one.
		NoVerdicts:    []marketplace.VerdictKind{marketplace.VerdictUnfinished},
		ExtraVerdicts: []marketplace.VerdictKind{marketplace.VerdictAgain},
		LabelType:     "BIG",
		// Approving a shipment does not take its postings: Ozon's carrier does, later, and the posting stays «awaiting_deliver».
		ClosingKeepsOrders: true,
		SetVerdict: func(t *testing.T, key map[string]string, number string, kind marketplace.VerdictKind) {
			r.giveVerdict(t, key, number, kind)
		},
		BatchNotReady: func(t *testing.T, key map[string]string) {
			r.control(t, key, "/__control/carriages", map[string]any{"not_ready": 1})
		},
		CancelOrder: func(t *testing.T, key map[string]string, number string) {
			r.control(t, key, "/__control/postings/status", map[string]any{
				"posting_number": number, "status": "cancelled", "cancellation_type": "customer",
			})
		},
		NewFeedOrder: func(t *testing.T, key map[string]string) string {
			r.control(t, key, "/__control/fbo-postings", map[string]any{
				"posting_number": "22222222-0001-1", "order_number": "22222222", "status": "delivering",
				"created_at": synckit.Noon(), "products": []map[string]any{product(2002, "SOCKS", 1)},
			})

			return "22222222-0001-1"
		},
	})
}

// TestEveryRouteIsAMethodOfOzonsOwnDescriptionFileAndOfTheFake ties the adapter's routes, and so the journal's paths, to
// the methods of api/ozon/seller-api.json and to the routes the fake Ozon answers.
func TestEveryRouteIsAMethodOfOzonsOwnDescriptionFileAndOfTheFake(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../../api/ozon/seller-api.json")
	if err != nil {
		t.Fatalf("read Ozon's file: %v", err)
	}

	var file struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}

	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("Ozon's file: %v", err)
	}

	r := newRig(t)
	key := r.key(t)

	for _, route := range r.adapter.Routes() {
		if route.NoKey {
			continue // the file of a label: not a method of Ozon's API, and the fake serves it under the file host
		}

		if _, ok := file.Paths[route.Template][strings.ToLower(route.Method)]; !ok {
			t.Errorf("%s %s is not a method of Ozon's description file", route.Method, route.Template)
		}

		req, _ := http.NewRequestWithContext(t.Context(), route.Method, r.kit.URL+"/api-seller.ozon.ru"+route.Template, strings.NewReader("{}"))
		req.Header.Set("Client-Id", key["client_id"])
		req.Header.Set("Api-Key", key["api_key"])

		res, err := r.kit.HTTP.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", route.Template, err)
		}

		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()

		// A shipment or a task that does not exist is also answered 404, with the word of that error; a method the fake does
		// not have is answered «Not Found».
		if res.StatusCode == http.StatusMethodNotAllowed || (res.StatusCode == http.StatusNotFound && strings.Contains(string(body), `"Not Found"`)) {
			t.Errorf("the fake Ozon does not answer %s %s: %d", route.Method, route.Template, res.StatusCode)
		}
	}
}

// TestTheRoutesSayWhichCallsWriteAndWhichOneIsTheLabelFileWithoutAKey ties the writing half's routes to what each call does at
// Ozon, and the one route that goes to a file link to a host under ozone.ru and carries no key.
func TestTheRoutesSayWhichCallsWriteAndWhichOneIsTheLabelFileWithoutAKey(t *testing.T) {
	t.Parallel()

	writes := map[string]bool{
		"/v6/fbs/posting/product/exemplar/set": true, "/v4/posting/fbs/ship": true, "/v3/posting/fbs/package-label/create": true,
		"/v1/carriage/create": true, "/v1/carriage/set-postings": true, "/v1/carriage/approve": true, "/v1/carriage/cancel": true,
	}

	adapter := ozon.New(ozon.Options{})
	files := 0

	for _, route := range adapter.Routes() {
		if route.NoKey {
			files++

			if route.Method != http.MethodGet || route.Template != "/{file...}" || route.Host != "*.ozone.ru" || route.Write {
				t.Errorf("the file route is %+v: want a GET of any path under *.ozone.ru that writes nothing", route)
			}

			continue
		}

		if route.Write != writes[route.Template] {
			t.Errorf("%s writes: %v, want %v", route.Template, route.Write, writes[route.Template])
		}

		if route.Host != "" {
			t.Errorf("%s is pinned to %q: only the file's route is", route.Template, route.Host)
		}
	}

	if files != 1 {
		t.Errorf("%d routes go without a key, want the one of the label file", files)
	}

	if hosts := adapter.Hosts(); !slices.Equal(hosts, []string{"api-seller.ozon.ru", "*.ozone.ru"}) {
		t.Errorf("the hosts are %v", hosts)
	}

	desc := adapter.Describe()
	if !desc.CodesReplace || desc.OrdersPeriod != 5*time.Minute || desc.StatesPeriod != 5*time.Minute ||
		!slices.Equal(desc.HandoverSteps, []string{"batch", "move", "close", "documents"}) {
		t.Errorf("the description is %+v", desc)
	}

	for _, c := range []marketplace.Capability{
		marketplace.OrdersPack, marketplace.LabelsFetch, marketplace.CodesReport, marketplace.CodesVerdict,
		marketplace.HandoverGroup, marketplace.HandoverClose, marketplace.HandoverDocuments,
	} {
		if !desc.Supports(c) || !adapter.Capable(c, false) {
			t.Errorf("the adapter does not claim %s", c)
		}
	}

	if desc.Supports(marketplace.OrdersConfirm) {
		t.Error("Ozon has no confirmation of an order, and the adapter claims orders.confirm")
	}
}
