package yandex_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

// markedOrder makes an order of two lines at the fake: two units of a product that needs a marking code (line 1) and one that
// needs none (line 2).
func (r *rig) markedOrder(t *testing.T, key marketplace.Key, more ...map[string]any) string {
	t.Helper()

	in := map[string]any{"items": []map[string]any{item("SHIRT", 2, "CIS"), item("SOCKS", 1)}}
	for _, m := range more {
		for k, v := range m {
			in[k] = v
		}
	}

	return r.order(t, key, in)
}

// code is a marking code of its own, with the group separator between its parts.
func code(n int) string { return fmt.Sprintf("0104630568317423215Ei%08d\x1d91EE12", n) }

func line(n, quantity int, codes ...string) marketplace.BoxLine {
	return marketplace.BoxLine{Line: n, Quantity: quantity, Codes: codes}
}

func boxOf(lines ...marketplace.BoxLine) marketplace.Box { return marketplace.Box{Lines: lines} }

// fullOrder is the boxes that pack the order of markedOrder in two boxes.
func fullOrder() []marketplace.Box {
	return []marketplace.Box{boxOf(line(1, 1, code(1)), line(2, 1)), boxOf(line(1, 1, code(2)))}
}

func (r *rig) setStatus(t *testing.T, key marketplace.Key, number, status, substatus string) {
	t.Helper()

	id, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		t.Fatalf("order number %q: %v", number, err)
	}

	r.control(t, key, "/__control/orders/status", map[string]any{"order_id": id, "status": status, "substatus": substatus})
}

func orderID(t *testing.T, number string) int64 {
	t.Helper()

	id, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		t.Fatalf("order number %q: %v", number, err)
	}

	return id
}

// bodiesTo gives the bodies of the calls the fake received for the key at a path ending so.
func (r *rig) bodiesTo(t *testing.T, key marketplace.Key, suffix string) []string {
	t.Helper()

	var out []string

	for _, c := range r.callsTo(t, key, suffix) {
		out = append(out, c.Body)
	}

	return out
}

func mustContain(t *testing.T, what, text string, parts ...string) {
	t.Helper()

	for _, p := range parts {
		if !strings.Contains(text, p) {
			t.Fatalf("%s: %s does not hold %q", what, text, p)
		}
	}
}

func mustNotContain(t *testing.T, what, text string, parts ...string) {
	t.Helper()

	for _, p := range parts {
		if strings.Contains(text, p) {
			t.Fatalf("%s: %s holds %q", what, text, p)
		}
	}
}
