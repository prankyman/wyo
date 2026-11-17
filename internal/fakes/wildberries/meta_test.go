package wildberries_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

const code1 = "0104600000000001215EirD_orEif7X\u001d91EE12\u001d92VygELprJ87wrc/qF/Vq/KEFUVpZm4vDPQt2kqUw308Y="

type metaDetail struct {
	Key      string  `json:"key"`
	Value    *string `json:"value"`
	Decision string  `json:"decision"`
}

// meta reads the marking codes and the verdicts of one order.
func (r *rig) meta(id int64) []metaDetail {
	r.t.Helper()

	var out struct {
		Orders []struct {
			ID          int64        `json:"id"`
			MetaDetails []metaDetail `json:"metaDetails"`
		} `json:"orders"`
	}

	r.rawJSON(http.MethodPost, "/api/marketplace/v3/orders/meta", ids2json([]int64{id}), http.StatusOK, &out)

	if len(out.Orders) != 1 || out.Orders[0].ID != id {
		r.t.Fatalf("meta of %d: %+v", id, out.Orders)
	}

	return out.Orders[0].MetaDetails
}

func putCodes(r *rig, id int64, codes ...string) (int, string) {
	r.t.Helper()

	body := `{"sgtins":[`

	for i, c := range codes {
		if i > 0 {
			body += ","
		}

		quoted, _ := json.Marshal(c)
		body += string(quoted)
	}

	status, _, text := r.raw(http.MethodPut, "/api/v3/orders/"+strconv.FormatInt(id, 10)+"/meta/sgtin", body+`]}`)

	return status, text
}

func TestMarkingCodesAreSentReadAndRemovedWithVerdictsATestChooses(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	id := r.newOrder(map[string]any{"required_meta": []string{"sgtin"}, "optional_meta": []string{"expiration"}})
	r.supplyWith(id)

	same(t, "before a code is sent", r.meta(id), []metaDetail{
		{Key: "sgtin", Decision: "required"}, {Key: "expiration", Decision: "optional"},
	})

	if status, body := putCodes(r, id, code1); status != http.StatusNoContent {
		t.Fatalf("put: %d %s", status, body)
	}

	same(t, "after a code is sent", r.meta(id), []metaDetail{
		{Key: "sgtin", Value: ptr(code1), Decision: "sgtinIntroduced"}, {Key: "expiration", Decision: "optional"},
	})

	for _, verdict := range []string{"pending", "sgtinNotFound", "sgtinNoGS", "deadlineExceeded"} {
		r.control(http.MethodPost, "/__control/verdicts", map[string]any{"order": id, "key": "sgtin", "decision": verdict}, nil)
		same(t, "verdict "+verdict, r.meta(id)[0].Decision, verdict)
	}

	status, _, body := r.raw(http.MethodDelete, "/api/v3/orders/"+strconv.FormatInt(id, 10)+"/meta?key=sgtin", "")
	if status != http.StatusNoContent {
		t.Fatalf("delete: %d %s", status, body)
	}

	same(t, "after the codes are removed", r.meta(id)[0], metaDetail{Key: "sgtin", Decision: "required"})
}

func TestACodeIsRefusedWhenItCannotBeAttached(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	needs := r.newOrder(map[string]any{"required_meta": []string{"sgtin"}})
	noMeta := r.newOrder(map[string]any{})
	r.supplyWith(noMeta)

	cases := []struct {
		name   string
		id     int64
		codes  []string
		status int
		code   string
	}{
		{name: "the order is still new", id: needs, codes: []string{code1}, status: 409, code: "FailedToUpdateMeta"},
		{name: "the order takes no such code", id: noMeta, codes: []string{code1}, status: 409, code: "MetaKeyNotRequired"},
		{name: "an order that is not there", id: 424242, codes: []string{code1}, status: 404},
		{name: "no codes", id: noMeta, codes: nil, status: 400, code: "IncorrectRequestBody"},
		{name: "a code too short", id: noMeta, codes: []string{"short"}, status: 400, code: "IncorrectRequestBody"},
		{name: "a code too long", id: noMeta, codes: []string{strings.Repeat("x", 136)}, status: 400, code: "IncorrectRequestBody"},
	}

	for _, tc := range cases {
		status, body := putCodes(r, tc.id, tc.codes...)
		if status != tc.status || (tc.code != "" && code(t, body) != tc.code) {
			t.Errorf("%s: %d %s, want %d %s", tc.name, status, body, tc.status, tc.code)
		}
	}

	status, _, body := r.raw(http.MethodPut, "/api/v3/orders/"+strconv.FormatInt(noMeta, 10)+"/meta/sgtin", `x`)
	if status != http.StatusBadRequest || code(t, body) != "IncorrectRequestBody" {
		t.Errorf("not json: %d %s", status, body)
	}
}

func TestMoreThanAHundredCodesAreRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	id := r.newOrder(map[string]any{"required_meta": []string{"sgtin"}})
	r.supplyWith(id)

	codes := make([]string, 101)
	for i := range codes {
		codes[i] = code1
	}

	if status, body := putCodes(r, id, codes...); status != http.StatusBadRequest || code(t, body) != "IncorrectRequestBody" {
		t.Fatalf("101 codes: %d %s", status, body)
	}

	if status, body := putCodes(r, id, codes[:100]...); status != http.StatusNoContent {
		t.Fatalf("100 codes: %d %s", status, body)
	}
}

func TestADeleteOfMetaIsRefusedWhenItCannotBeDone(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	id := r.newOrder(map[string]any{"required_meta": []string{"sgtin"}})

	cases := []struct {
		name, path string
		status     int
		code       string
	}{
		{name: "no key", path: "/api/v3/orders/" + strconv.FormatInt(id, 10) + "/meta", status: 400, code: "IncorrectParameter"},
		{name: "a key that does not exist", path: "/api/v3/orders/" + strconv.FormatInt(id, 10) + "/meta?key=colour", status: 400, code: "IncorrectParameter"},
		{name: "an order not in assembly", path: "/api/v3/orders/" + strconv.FormatInt(id, 10) + "/meta?key=sgtin", status: 409, code: "FailedToUpdateMeta"},
		{name: "an order that is not there", path: "/api/v3/orders/424242/meta?key=sgtin", status: 409, code: "FailedToUpdateMeta"},
		{name: "an order id that is not a number", path: "/api/v3/orders/abc/meta?key=sgtin", status: 400, code: "IncorrectParameter"},
	}

	for _, tc := range cases {
		status, _, body := r.raw(http.MethodDelete, tc.path, "")
		if status != tc.status || code(t, body) != tc.code {
			t.Errorf("%s: %d %s", tc.name, status, body)
		}
	}
}

func TestMetaOfManyOrdersIsReadInOneCall(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	a := r.newOrder(map[string]any{"required_meta": []string{"sgtin"}})
	b := r.newOrder(map[string]any{})

	var out struct {
		Orders []struct {
			ID          int64        `json:"id"`
			MetaDetails []metaDetail `json:"metaDetails"`
		} `json:"orders"`
	}

	r.rawJSON(http.MethodPost, "/api/marketplace/v3/orders/meta", ids2json([]int64{a, b, 99999}), http.StatusOK, &out)
	same(t, "orders found", len(out.Orders), 2)
	same(t, "an order that takes no codes", len(out.Orders[1].MetaDetails), 0)

	many := make([]int64, 101)
	for i := range many {
		many[i] = a
	}

	for name, body := range map[string]string{"over a hundred": ids2json(many), "none": `{"orders":[]}`, "not json": `x`} {
		status, _, text := r.raw(http.MethodPost, "/api/marketplace/v3/orders/meta", body)
		if status != http.StatusBadRequest || code(t, text) != "IncorrectRequestBody" {
			t.Errorf("%s: %d %s", name, status, text)
		}
	}
}

func TestABadVerdictControlCallIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	id := r.newOrder(map[string]any{"required_meta": []string{"sgtin"}})

	for name, body := range map[string]map[string]any{
		"no order": {"key": "sgtin", "decision": "pending"}, "an order that is not there": {"order": 1, "key": "sgtin", "decision": "pending"},
		"no decision": {"order": id, "key": "sgtin"}, "a key the order does not take": {"order": id, "key": "uin", "decision": "pending"},
	} {
		if err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/verdicts", body, nil); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}
