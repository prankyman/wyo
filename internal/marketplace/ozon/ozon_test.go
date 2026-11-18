package ozon_test

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/contracttest"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestTheAdapterKeepsTheContractAgainstTheFake(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	contracttest.RunAdapterContract(t, contracttest.Fixture{
		Adapter: r.adapter,
		Now:     synckit.Noon(),
		Key:     func(t *testing.T) map[string]string { return r.key(t) },
		Conn:    func(t *testing.T, key map[string]string) marketplace.Conn { return r.conn(t, key) },
		NewOrder: func(t *testing.T, key map[string]string) string {
			r.posting(t, key, "11111111-0001-1", warehouseFBS, product(2001, "SHIRT", 1))

			return "11111111-0001-1"
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
		if _, ok := file.Paths[route.Template][strings.ToLower(route.Method)]; !ok {
			t.Errorf("%s %s is not a method of Ozon's description file", route.Method, route.Template)
		}

		if route.Write {
			t.Errorf("%s %s writes; this adapter only reads", route.Method, route.Template)
		}

		req, _ := http.NewRequestWithContext(t.Context(), route.Method, r.kit.URL+"/api-seller.ozon.ru"+route.Template, strings.NewReader("{}"))
		req.Header.Set("Client-Id", key["client_id"])
		req.Header.Set("Api-Key", key["api_key"])

		res, err := r.kit.HTTP.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", route.Template, err)
		}

		_ = res.Body.Close()

		if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusMethodNotAllowed {
			t.Errorf("the fake Ozon does not answer %s %s: %d", route.Method, route.Template, res.StatusCode)
		}
	}
}
