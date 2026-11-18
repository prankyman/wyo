package adapterkit_test

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/adapterkit"
)

func TestValueAndNumberReadOptionalFields(t *testing.T) {
	t.Parallel()

	text, whole, big := "x", 42, int64(-7)

	if got := adapterkit.Value(&text); got != "x" {
		t.Fatalf("Value(&%q) = %q", text, got)
	}

	if got := adapterkit.Value[string](nil); got != "" {
		t.Fatalf("Value(nil) = %q, want the zero value", got)
	}

	cases := []struct {
		name string
		got  string
		want string
	}{
		{name: "an int", got: adapterkit.Number(&whole), want: "42"},
		{name: "an int64", got: adapterkit.Number(&big), want: "-7"},
		{name: "nothing", got: adapterkit.Number[int](nil), want: ""},
	}

	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestCutKeepsTheFirstThreeHundredCharacters(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("я", 301)

	if got := []rune(adapterkit.Cut(long)); len(got) != 300 {
		t.Fatalf("Cut kept %d characters, want 300", len(got))
	}

	if got := adapterkit.Cut("short"); got != "short" {
		t.Fatalf("Cut(short) = %q", got)
	}
}

func TestSucceededIsTheTwoHundreds(t *testing.T) {
	t.Parallel()

	for status, want := range map[int]bool{
		http.StatusOK: true, http.StatusNoContent: true, http.StatusMultipleChoices: false, http.StatusBadRequest: false, 199: false,
	} {
		if got := adapterkit.Succeeded(status); got != want {
			t.Errorf("Succeeded(%d) = %v, want %v", status, got, want)
		}
	}
}

func TestEachPartCutsAListIntoRequestsOfN(t *testing.T) {
	t.Parallel()

	var parts [][]int

	err := adapterkit.EachPart([]int{1, 2, 3, 4, 5}, 2, func(part []int) error {
		parts = append(parts, part)
		return nil
	})
	if err != nil || !reflect.DeepEqual(parts, [][]int{{1, 2}, {3, 4}, {5}}) {
		t.Fatalf("EachPart = %v, %v", parts, err)
	}

	stop := errors.New("stop")
	calls := 0

	err = adapterkit.EachPart([]int{1, 2, 3}, 1, func([]int) error {
		calls++
		return stop
	})
	if !errors.Is(err, stop) || calls != 1 {
		t.Fatalf("EachPart went on after an error: %d calls, %v", calls, err)
	}

	if err := adapterkit.EachPart[int](nil, 10, func([]int) error { return stop }); err != nil {
		t.Fatalf("EachPart of nothing called fn: %v", err)
	}
}
