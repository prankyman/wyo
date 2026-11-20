package fulfilment_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

// warehousesRaw reads GET /warehouses as the wire shows it, so that a missing list and a null one are told apart.
func warehousesRaw(r *rig) []map[string]any {
	r.t.Helper()

	status, body := r.raw(http.MethodGet, "/api/v1/warehouses", "")
	if status != http.StatusOK {
		r.t.Fatalf("warehouses answered %d %s", status, body)
	}

	var out struct {
		Warehouses []map[string]any `json:"warehouses"`
	}

	if err := json.Unmarshal([]byte(body), &out); err != nil {
		r.t.Fatalf("warehouses: %v", err)
	}

	return out.Warehouses
}

func TestAWarehouseWithNoChoiceOfLabelSizeAnswersAnEmptyList(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	got := warehousesRaw(r)
	same(t, "label_sizes", got[0]["label_sizes"], []any{})

	r.setup(map[string]any{"warehouses": []map[string]string{{"code": "w1", "name": "One"}}})
	same(t, "label_sizes after a setup that names none", warehousesRaw(r)[0]["label_sizes"], []any{})
}

func TestAControlCallSetsTheLabelSizesOfAWarehouse(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.setup(map[string]any{"warehouses": []map[string]string{{"code": "w1", "name": "One"}, {"code": "w2", "name": "Two"}}})

	sizes := []fcapi.LabelSize{{Marketplace: "wildberries", WidthMm: 58, HeightMm: 40}, {Marketplace: "ozon", WidthMm: 100, HeightMm: 150}}
	r.control(http.MethodPost, "/__control/warehouses/w2/label-sizes", map[string]any{"label_sizes": sizes}, nil)

	res, err := r.client.WarehousesWithResponse(r.ctx())
	if err != nil || res.JSON200 == nil {
		t.Fatalf("warehouses: %v", err)
	}

	same(t, "the other warehouse", res.JSON200.Warehouses[0].LabelSizes, []fcapi.LabelSize{})
	same(t, "the set warehouse", res.JSON200.Warehouses[1].LabelSizes, sizes)

	r.control(http.MethodPost, "/__control/warehouses/w2/label-sizes", map[string]any{"label_sizes": []fcapi.LabelSize{}}, nil)
	same(t, "after an empty list", warehousesRaw(r)[1]["label_sizes"], []any{})
}

func TestASetupMayCarryTheLabelSizesOfItsWarehouses(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	sizes := []fcapi.LabelSize{{Marketplace: "wildberries", WidthMm: 58, HeightMm: 40}}
	r.setup(map[string]any{"warehouses": []map[string]any{{"code": "w1", "name": "One", "label_sizes": sizes}}})

	res, err := r.client.WarehousesWithResponse(r.ctx())
	if err != nil || res.JSON200 == nil {
		t.Fatalf("warehouses: %v", err)
	}

	same(t, "label_sizes", res.JSON200.Warehouses[0].LabelSizes, sizes)
}

func TestABadLabelSizeCallIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		path   string
		body   any
		status string
	}{
		{name: "a setup with a bad label size", path: "/__control/setup", body: map[string]any{"warehouses": []map[string]any{
			{"code": "w1", "name": "One", "label_sizes": []fcapi.LabelSize{{Marketplace: "ozon", WidthMm: 0, HeightMm: 40}}},
		}}, status: "400"},
		{name: "a warehouse that is not there", path: "/__control/warehouses/nowhere/label-sizes", body: map[string]any{"label_sizes": []fcapi.LabelSize{}}, status: "404"},
		{name: "no list", path: "/__control/warehouses/main/label-sizes", body: map[string]any{}, status: "400"},
		{name: "an unknown field", path: "/__control/warehouses/main/label-sizes", body: map[string]any{"label_sizes": []fcapi.LabelSize{}, "colour": "red"}, status: "400"},
		{name: "a width of zero", path: "/__control/warehouses/main/label-sizes", body: map[string]any{"label_sizes": []fcapi.LabelSize{{Marketplace: "ozon", WidthMm: 0, HeightMm: 40}}}, status: "400"},
		{name: "a height below zero", path: "/__control/warehouses/main/label-sizes", body: map[string]any{"label_sizes": []fcapi.LabelSize{{Marketplace: "ozon", WidthMm: 58, HeightMm: -1}}}, status: "400"},
		{name: "no marketplace", path: "/__control/warehouses/main/label-sizes", body: map[string]any{"label_sizes": []fcapi.LabelSize{{WidthMm: 58, HeightMm: 40}}}, status: "400"},
		{name: "one marketplace twice", path: "/__control/warehouses/main/label-sizes", body: map[string]any{"label_sizes": []fcapi.LabelSize{
			{Marketplace: "ozon", WidthMm: 58, HeightMm: 40}, {Marketplace: "ozon", WidthMm: 100, HeightMm: 150},
		}}, status: "400"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)

			err := r.ctl.Do(r.ctx(), http.MethodPost, tc.path, tc.body, nil)
			if err == nil || !strings.Contains(err.Error(), "answered "+tc.status) {
				t.Fatalf("answered %v, want %s", err, tc.status)
			}

			same(t, "label_sizes stay", warehousesRaw(r)[0]["label_sizes"], []any{})
		})
	}
}
