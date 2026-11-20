package fulfilment

import (
	"net/http"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

// renderWarehouses is the list GET /warehouses answers. The label sizes are always a list, empty when the warehouse chooses
// no size for any marketplace: the contract requires the field.
func (st *state) renderWarehouses() []fcapi.Warehouse {
	out := make([]fcapi.Warehouse, len(st.warehouses))

	for i, w := range st.warehouses {
		w.LabelSizes = append([]fcapi.LabelSize{}, w.LabelSizes...)
		out[i] = w
	}

	return out
}

// checkLabelSizes says what is wrong with a warehouse's label sizes, or "" when they are fine: each names a marketplace and
// a size above zero, and no marketplace is named twice.
func checkLabelSizes(sizes []fcapi.LabelSize) string {
	seen := map[string]bool{}

	for _, s := range sizes {
		switch {
		case s.Marketplace == "":
			return "a label size names no marketplace"
		case s.WidthMm < 1 || s.HeightMm < 1:
			return "a label size of " + s.Marketplace + " is not above zero millimetres"
		case seen[s.Marketplace]:
			return "the label size of " + s.Marketplace + " is given twice"
		}

		seen[s.Marketplace] = true
	}

	return ""
}

// controlLabelSizes sets the label sizes of one warehouse, as a fulfilment centre's people would choose them.
func (s *Server) controlLabelSizes(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LabelSizes *[]fcapi.LabelSize `json:"label_sizes"`
	}

	if !controlBody(w, r, &in, false) {
		return
	}

	if in.LabelSizes == nil {
		fakekit.ControlError(w, http.StatusBadRequest, "label_sizes is a list, empty for no choice")

		return
	}

	if why := checkLabelSizes(*in.LabelSizes); why != "" {
		fakekit.ControlError(w, http.StatusBadRequest, why)

		return
	}

	found := false

	s.with(s.kit.KeyOf(r), func(st *state) {
		for i := range st.warehouses {
			if st.warehouses[i].Code == r.PathValue("code") {
				st.warehouses[i].LabelSizes, found = append([]fcapi.LabelSize{}, *in.LabelSizes...), true
			}
		}
	})

	if !found {
		fakekit.ControlError(w, http.StatusNotFound, "no such warehouse")

		return
	}

	done(w)
}
