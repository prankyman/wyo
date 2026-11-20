package exchange_test

import (
	"net/http"
	"testing"
)

// TestAWildberriesStickerIsAskedForInTheSizeTheFulfilmentCentreNames sends and reserves a Wildberries order while the
// fulfilment centre names, for the order's warehouse and Wildberries, no size, 58 × 40 or 40 × 30: the sticker is asked for in
// exactly that size, Wildberries' 58 × 40 when none is named.
func TestAWildberriesStickerIsAskedForInTheSizeTheFulfilmentCentreNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		width, height int
		query         string
	}{
		{name: "none named", query: "height=40&type=png&width=58"},
		{name: "58 × 40", width: 58, height: 40, query: "height=40&type=png&width=58"},
		{name: "40 × 30", width: 40, height: 30, query: "height=30&type=png&width=40"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)

			if tc.width > 0 {
				do(t, r.fc, http.MethodPost, "/__control/warehouses/main/label-sizes", map[string]any{
					"label_sizes": []map[string]any{{"marketplace": "wildberries", "width_mm": tc.width, "height_mm": tc.height}},
				}, nil)
			}

			number, _ := r.send(t, map[string]any{})
			if x := r.reserve(t, number); x.State != "label_sent" {
				t.Fatalf("the order after the reservation: %+v", x)
			}

			if asked := r.wbCalls(t, http.MethodPost, "/api/v3/orders/stickers"); len(asked) != 1 || asked[0].Query != tc.query {
				t.Fatalf("the sticker was asked for with %+v; want %s", asked, tc.query)
			}
		})
	}
}
