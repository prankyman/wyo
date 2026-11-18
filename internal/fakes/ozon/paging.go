package ozon

import (
	"errors"
	"strconv"
)

// errBadCursor is a cursor the fake never gave.
var errBadCursor = errors.New("the cursor is not one this method gave")

// paged cuts the next page out of items, which are in the order of seq. The cursor is the place of the last item of the page
// before (Ozon's own cursors are opaque, and the fake's is a number): an item that comes between two pages falls after the
// cursor, and the pages already read do not change. next is empty when the page is the last.
func paged[T any](items []T, seq func(T) int, cursor string, size int) (page []T, next string, err error) {
	after := 0

	if cursor != "" {
		after, err = strconv.Atoi(cursor)
		if err != nil || after < 0 {
			return nil, "", errBadCursor
		}
	}

	rest := items

	for len(rest) > 0 && seq(rest[0]) <= after {
		rest = rest[1:]
	}

	if len(rest) <= size {
		return rest, "", nil
	}

	page = rest[:size]

	return page, strconv.Itoa(seq(page[size-1])), nil
}

// The largest page of each list, as Ozon's description file gives it («maximum» of the field limit).
const (
	largestPostingsPage  = 100
	largestWarehousePage = 200
)

// pageSize reads the limit of a request. A limit that is negative or larger than the method allows is refused (400, in the
// fake's words). A limit that is left out is the largest page, unless the method requires one, as the warehouse list does
// («required» of the field limit in the description file).
func (c *call) pageSize(requested int64, largest int, required bool) (int, bool) {
	switch {
	case requested < 0 || requested > int64(largest):
		badRequest(c.w, "limit must be from 1 to "+strconv.Itoa(largest))

		return 0, false
	case requested == 0 && required:
		badRequest(c.w, "limit is required, from 1 to "+strconv.Itoa(largest))

		return 0, false
	case requested == 0:
		return largest, true
	default:
		return int(requested), true
	}
}
