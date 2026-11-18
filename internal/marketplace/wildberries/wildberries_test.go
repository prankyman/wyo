package wildberries_test

import (
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/contracttest"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// verdictWords is the decision word of Wildberries for each kind of verdict of a marking code (03-orders-fbs.yaml).
var verdictWords = map[marketplace.VerdictKind]string{
	marketplace.VerdictAccepted: "sgtinIntroduced", marketplace.VerdictPending: "pending",
	marketplace.VerdictUnfinished: "deadlineExceeded", marketplace.VerdictRefused: "sgtinNotFound",
}

func TestTheAdapterKeepsTheContractAgainstTheFake(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	contracttest.RunAdapterContract(t, contracttest.Fixture{
		Adapter: r.adapter,
		Now:     synckit.Noon(),
		Key:     func(t *testing.T) map[string]string { return fields(r.token(t)) },
		Conn:    func(t *testing.T, key map[string]string) marketplace.Conn { return r.conn(t, key["token"], false) },
		NewOrder: func(t *testing.T, key map[string]string) string {
			return r.newOrder(t, key["token"], map[string]any{})
		},
		CancelOrder: func(t *testing.T, key map[string]string, number string) {
			r.control(t, key["token"], http.MethodPost, "/__control/orders/"+number+"/wb-status", map[string]any{"wb_status": "declined_by_client"}, nil)
		},
		NewFeedOrder: func(t *testing.T, key map[string]string) string {
			r.control(t, key["token"], http.MethodPost, "/__control/feed", map[string]any{
				"rows": []map[string]any{{"srid": "fbo-srid-1", "g_number": "g-1", "warehouse_type": "Склад WB", "warehouse_name": "Коледино"}},
			}, nil)

			return "fbo-srid-1"
		},
		NewMarkedOrder: func(t *testing.T, key map[string]string) string {
			return r.newOrder(t, key["token"], map[string]any{"required_meta": []string{"sgtin"}})
		},
		ChangeLabel: func(t *testing.T, key map[string]string, number string) {
			r.control(t, key["token"], http.MethodPost, "/__control/orders/"+number+"/new-sticker", nil, nil)
		},
		SetVerdict: func(t *testing.T, key map[string]string, number string, kind marketplace.VerdictKind) {
			r.verdict(t, key["token"], number, "sgtin", verdictWords[kind])
		},
		HandoverSettings: settings,
	})
}

// TestEveryRouteIsAMethodOfWildberriesOwnDescriptionFiles ties the adapter's routes, and so the journal's paths with
// their placeholders, to the paths of the files under api/wildberries/.
func TestEveryRouteIsAMethodOfWildberriesOwnDescriptionFiles(t *testing.T) {
	t.Parallel()

	files := []string{"01-general.yaml", "02-items.yaml", "03-orders-fbs.yaml", "12-reports.yaml"}
	docs := make([]*openapi3.T, 0, len(files))

	for _, file := range files {
		loader := openapi3.NewLoader()

		doc, err := loader.LoadFromFile(filepath.Join("..", "..", "..", "api", "wildberries", file))
		if err != nil {
			t.Fatalf("load %s: %v", file, err)
		}

		docs = append(docs, doc)
	}

	for _, route := range wildberries.New(wildberries.Options{}).Routes() {
		found := false

		for _, doc := range docs {
			item := doc.Paths.Value(route.Template)
			if item == nil || item.GetOperation(route.Method) == nil {
				continue
			}

			found = true

			// A route writes exactly when the file does not mark its method as only reading.
			readOnly, _ := item.GetOperation(route.Method).Extensions["x-readonly-method"].(bool)
			if route.Write == readOnly {
				t.Errorf("%s %s: Write is %v, but the file's x-readonly-method is %v", route.Method, route.Template, route.Write, readOnly)
			}
		}

		if !found {
			t.Errorf("%s %s is in none of the description files", route.Method, route.Template)
		}

		// The wyo server never cancels an order at Wildberries (plan 4, section G).
		if strings.HasSuffix(route.Template, "/cancel") {
			t.Errorf("%s %s cancels an order", route.Method, route.Template)
		}
	}
}

func TestTheAdapterClaimsTheWritingPartsAndDeclaresItsHandoverSettings(t *testing.T) {
	t.Parallel()

	desc := wildberries.New(wildberries.Options{}).Describe()

	for _, c := range []marketplace.Capability{
		marketplace.OrdersConfirm, marketplace.LabelsFetch, marketplace.CodesReport, marketplace.CodesVerdict,
		marketplace.HandoverGroup, marketplace.HandoverClose, marketplace.HandoverDocuments,
	} {
		if !desc.Supports(c) {
			t.Errorf("the adapter does not claim %s", c)
		}
	}

	want := []marketplace.SettingField{
		{Name: "shipping_type", Choices: []string{"selfShipping", "transportCompany"}},
		{Name: "city"},
		{Name: "office_type", Choices: []string{"sc", "sw", "pp", "pfc", "gpp"}},
	}

	if !reflect.DeepEqual(desc.HandoverSettings, want) {
		t.Fatalf("hand-over settings %+v, want %+v", desc.HandoverSettings, want)
	}
}

func TestATestKeyReadsNoFeedAndTalksToTheSandboxAtItsLimit(t *testing.T) {
	t.Parallel()

	a := wildberries.New(wildberries.Options{})

	if a.Capable(marketplace.OrdersFeed, true) || !a.Capable(marketplace.OrdersFeed, false) || !a.Capable(marketplace.OrdersNew, true) {
		t.Fatal("a test key reads everything but the feed, which has no sandbox host")
	}

	// The sandbox has no shipping-point and no shipping-method methods (research file, section 2, column S), so a test
	// key can neither group a hand-over nor close it nor fetch its documents (a supply is not closed without its shipping).
	for _, c := range []marketplace.Capability{marketplace.HandoverGroup, marketplace.HandoverClose, marketplace.HandoverDocuments} {
		if a.Capable(c, true) || !a.Capable(c, false) {
			t.Fatalf("%s: a test key %v, a real key %v; want false and true", c, a.Capable(c, true), a.Capable(c, false))
		}
	}

	for _, c := range []marketplace.Capability{marketplace.OrdersConfirm, marketplace.LabelsFetch, marketplace.CodesReport, marketplace.CodesVerdict} {
		if !a.Capable(c, true) || !a.Capable(c, false) {
			t.Fatalf("%s: both kinds of key can do it in the sandbox", c)
		}
	}

	real, _ := a.Limit("marketplace", false)
	sandbox, _ := a.Limit("marketplace", true)

	if real.Burst != 20 || real.Interval.Milliseconds() != 200 || real.ErrorCost != 10 || sandbox.Burst != 1 || sandbox.Interval.Seconds() != 1 {
		t.Fatalf("limits: real %+v, sandbox %+v", real, sandbox)
	}

	if _, ok := a.Limit("nothing", false); ok {
		t.Fatal("a group the adapter does not declare has a limit")
	}

	h := http.Header{}
	a.Authorize(h, marketplace.Key{"token": "the-token"})

	if h.Get("Authorization") != "the-token" {
		t.Fatalf("Authorization %q: Wildberries takes the token as it is", h.Get("Authorization"))
	}
}

func TestTheCodeAndTextOfEachShapeOfRefusalAreRead(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, body, code, text string
	}{
		{name: "the Error schema", body: `{"code":"IncorrectParameter","message":"Передан некорректный параметр"}`, code: "IncorrectParameter", text: "Передан некорректный параметр"},
		{name: "a problem", body: `{"title":"Unauthorized","detail":"token expired","status":401}`, code: "Unauthorized", text: "token expired"},
		{name: "the statistics' errors", body: `{"errors":["dateFrom: field required"]}`, text: `["dateFrom: field required"]`},
		{name: "not JSON", body: "Bad Gateway\n", text: "Bad Gateway"},
		{name: "nothing known", body: `{}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			code, text := wildberries.New(wildberries.Options{}).ErrorOf(http.StatusBadRequest, []byte(tc.body))
			if code != tc.code || strings.TrimSpace(text) != tc.text {
				t.Fatalf("ErrorOf = %q, %q; want %q, %q", code, text, tc.code, tc.text)
			}
		})
	}
}
