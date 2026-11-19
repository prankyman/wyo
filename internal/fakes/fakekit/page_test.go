package fakekit_test

import (
	"errors"
	"testing"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

type numbered struct{ seq int }

func seqOf(n numbered) int { return n.seq }

func TestPagedCutsPagesThatFollowTheTokenToTheEnd(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		total int
		size  int
		pages []int
	}{
		{name: "the page holds everything", total: 3, size: 3, pages: []int{3}},
		{name: "the page holds more than there is", total: 3, size: 50, pages: []int{3}},
		{name: "one at a time", total: 3, size: 1, pages: []int{1, 1, 1}},
		{name: "the last page is short", total: 5, size: 2, pages: []int{2, 2, 1}},
		{name: "nothing to list", total: 0, size: 10, pages: []int{0}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			items := make([]numbered, tc.total)
			for i := range items {
				items[i] = numbered{seq: i + 1}
			}

			token := ""

			var sizes []int

			for range tc.pages {
				page, next, err := fakekit.Paged(items, seqOf, token, tc.size)
				if err != nil {
					t.Fatalf("page: %v", err)
				}

				sizes = append(sizes, len(page))
				token = next
			}

			if token != "" {
				t.Fatalf("the token after the last page is %q, want none", token)
			}

			if len(sizes) != len(tc.pages) {
				t.Fatalf("pages %v, want %v", sizes, tc.pages)
			}

			for i := range sizes {
				if sizes[i] != tc.pages[i] {
					t.Fatalf("pages %v, want %v", sizes, tc.pages)
				}
			}
		})
	}
}

func TestPagedKeepsThePagesAlreadyReadWhenAnItemComesBetweenTwoPages(t *testing.T) {
	t.Parallel()

	items := []numbered{{1}, {2}, {3}}

	first, token, err := fakekit.Paged(items, seqOf, "", 2)
	if err != nil || len(first) != 2 || token != "2" {
		t.Fatalf("first page %v, token %q, err %v, want two items and the token 2", first, token, err)
	}

	items = append(items, numbered{4})

	second, token, err := fakekit.Paged(items, seqOf, token, 2)
	if err != nil || len(second) != 2 || second[0].seq != 3 || token != "" {
		t.Fatalf("second page %v, token %q, err %v, want items 3 and 4 and no token", second, token, err)
	}
}

func TestPagedRefusesATokenItNeverGave(t *testing.T) {
	t.Parallel()

	for _, token := range []string{"abc", "-1"} {
		if _, _, err := fakekit.Paged([]numbered{{1}}, seqOf, token, 1); !errors.Is(err, fakekit.ErrBadPageToken) {
			t.Fatalf("token %q: got %v, want ErrBadPageToken", token, err)
		}
	}
}
