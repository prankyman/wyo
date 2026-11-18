// This test is in the package itself: the grouping of a hand-over's orders is not exported, and it is the rule that decides
// how many supplies a hand-over makes.
package handover

import (
	"errors"
	"slices"
	"testing"

	"github.com/prankyman/wyo/internal/store"
)

func TestTheOrdersOfAHandoverAreSplitByAccountAndGroupInTheirOrder(t *testing.T) {
	t.Parallel()

	order := func(id, account int64, group string) store.HandoverOrder {
		return store.HandoverOrder{OrderID: id, AccountID: account, Group: group, Number: "n"}
	}

	cases := []struct {
		name string
		in   []store.HandoverOrder
		want [][]int64 // the order ids of each group
	}{
		{name: "no orders", in: nil, want: nil},
		{name: "one group", in: []store.HandoverOrder{order(1, 1, "a"), order(2, 1, "a")}, want: [][]int64{{1, 2}}},
		{name: "two groups of one account", in: []store.HandoverOrder{order(1, 1, "a"), order(2, 1, "b"), order(3, 1, "b")}, want: [][]int64{{1}, {2, 3}}},
		{name: "the same group name of two accounts", in: []store.HandoverOrder{order(1, 1, "a"), order(2, 2, "a")}, want: [][]int64{{1}, {2}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got [][]int64

			for _, g := range groups(tc.in) {
				var ids []int64
				for _, o := range g.orders {
					ids = append(ids, o.OrderID)
				}

				got = append(got, ids)
			}

			if !slices.EqualFunc(got, tc.want, slices.Equal) {
				t.Fatalf("groups = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTheStepsOfAHandoverGoInTheOrderTheMarketplaceNeedsThem(t *testing.T) {
	t.Parallel()

	want := []string{"batch", "move", "boxes", "shipping", "close", "documents"}
	if !slices.Equal(Steps, want) {
		t.Fatalf("Steps = %v, want %v", Steps, want)
	}

	if doneKey(1, "a", StepMove) == doneKey(1, "a", StepBoxes) || doneKey(1, "a", StepMove) == doneKey(2, "a", StepMove) ||
		doneKey(1, "a", StepMove) == doneKey(1, "b", StepMove) {
		t.Fatal("two different steps of a hand-over share a key")
	}
}

func TestTheStopOfARunIsNoErrorOfTheCaller(t *testing.T) {
	t.Parallel()

	if err := ignoreWait(errWait); err != nil {
		t.Fatalf("ignoreWait(errWait) = %v", err)
	}

	other := errors.New("other")
	if err := ignoreWait(other); !errors.Is(err, other) {
		t.Fatalf("ignoreWait(other) = %v", err)
	}
}
