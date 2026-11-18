package issue_test

import (
	"errors"
	"testing"

	"github.com/prankyman/wyo/internal/issue"
)

func TestAnErrorNamesItsFirstIssueAndKeepsAllOfThem(t *testing.T) {
	t.Parallel()

	err := error(&issue.Error{Issues: []issue.Issue{
		{Path: "newPassword", Code: issue.TooShort, Params: map[string]any{"min": 10}},
		{Path: "currentPassword", Code: issue.Wrong},
	}})

	var invalid *issue.Error
	if !errors.As(err, &invalid) || len(invalid.Issues) != 2 {
		t.Fatalf("got %v", err)
	}

	if want := "2 fields were refused (first: newPassword too_short)"; err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}

func TestTheParamsOfARefusalAreThoseOfTheFirstIssueThatHasAny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		issues []issue.Issue
		want   map[string]any
	}{
		{name: "none", issues: []issue.Issue{{Path: "a", Code: issue.Wrong}}},
		{
			name:   "the first with any",
			issues: []issue.Issue{{Path: "a", Code: issue.Wrong}, {Path: "b", Code: issue.TooShort, Params: map[string]any{"min": 3}}, {Path: "c", Code: issue.TooLong, Params: map[string]any{"max": 9}}},
			want:   map[string]any{"min": 3},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := (&issue.Error{Issues: tc.issues}).Params()
			if len(got) != len(tc.want) || got["min"] != tc.want["min"] {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
