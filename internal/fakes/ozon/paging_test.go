package ozon_test

import (
	"strconv"
	"testing"
)

// The pages of a list are tried on the warehouse list, which any key can have in any number.
func TestPagesFollowTheCursorToTheEndWhateverTheSizeOfThePage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		total int
		limit int
		pages []int
	}{
		{name: "the page holds everything", total: 3, limit: 3, pages: []int{3}},
		{name: "the page holds more than there is", total: 3, limit: 200, pages: []int{3}},
		{name: "one at a time", total: 3, limit: 1, pages: []int{1, 1, 1}},
		{name: "the last page is short", total: 5, limit: 2, pages: []int{2, 2, 1}},
		{name: "nothing to list", total: 0, limit: 10, pages: []int{0}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)

			specs := make([]map[string]any, tc.total)
			for i := range specs {
				specs[i] = map[string]any{"id": 100 + i, "name": "Warehouse " + strconv.Itoa(i)}
			}

			r.control("setup", map[string]any{"warehouses": specs}, nil)

			cursor := ""

			var sizes []int

			for page := range tc.pages {
				var out warehousesAnswer

				r.answer(pathWarehouses, map[string]any{"limit": tc.limit, "cursor": cursor}, &out)

				sizes = append(sizes, len(out.Warehouses))

				last := page == len(tc.pages)-1
				if out.HasNext == last || (out.Cursor == "") != last {
					t.Fatalf("page %d: has_next %v, cursor %q", page+1, out.HasNext, out.Cursor)
				}

				cursor = out.Cursor
			}

			if len(sizes) != len(tc.pages) {
				t.Fatalf("pages %v, want %v", sizes, tc.pages)
			}

			for i, size := range sizes {
				same(t, "the size of page "+strconv.Itoa(i+1), size, tc.pages[i])
			}
		})
	}
}

func TestACursorBeyondTheEndAnswersAnEmptyLastPage(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	var out warehousesAnswer

	r.answer(pathWarehouses, map[string]any{"limit": 10, "cursor": "999"}, &out)

	if len(out.Warehouses) != 0 || out.HasNext || out.Cursor != "" {
		t.Fatalf("a cursor beyond the end answered %+v", out)
	}
}
