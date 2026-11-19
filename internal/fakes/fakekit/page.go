package fakekit

import (
	"errors"
	"strconv"
)

// ErrBadPageToken is a page token the fake never gave.
var ErrBadPageToken = errors.New("the page token is not one this method gave")

// Paged cuts the next page out of items, which are in the order of seq. The token is the place of the last item of the page
// before (a real marketplace's tokens are opaque, and the fake's is a number): an item that comes between two pages falls
// after the token, and the pages already read do not change. next is empty when the page is the last.
func Paged[T any](items []T, seq func(T) int, token string, size int) (page []T, next string, err error) {
	after := 0

	if token != "" {
		after, err = strconv.Atoi(token)
		if err != nil || after < 0 {
			return nil, "", ErrBadPageToken
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
