// Package issue is the one shape of a refused field, used by every package that checks what a person typed: the
// password rules, the settings registry. The API turns it into the refusal `invalid` with `issues`.
package issue

import "fmt"

// The codes of a refused field; the interface words `errors.<code>`.
const (
	Required      = "required"
	Wrong         = "wrong"
	TooShort      = "too_short"
	TooLong       = "too_long"
	SameAsCurrent = "same_as_current"
	OutOfRange    = "out_of_range"
	WrongKind     = "wrong_kind"
)

// Issue names one refused field and why. Params fill the text of the code: `min`, `max`.
type Issue struct {
	Path   string
	Code   string
	Params map[string]any
}

// Error is one or more refused fields; nothing was changed.
type Error struct {
	Issues []Issue
}

// Error implements error.
func (e *Error) Error() string {
	if len(e.Issues) == 1 {
		return fmt.Sprintf("1 field was refused (%s %s)", e.Issues[0].Path, e.Issues[0].Code)
	}

	return fmt.Sprintf("%d fields were refused (first: %s %s)", len(e.Issues), e.Issues[0].Path, e.Issues[0].Code)
}

// Params are the parameters the refusal carries: the interface's text of an issue is filled from one set, so they are
// those of the first issue that has any.
func (e *Error) Params() map[string]any {
	for _, i := range e.Issues {
		if len(i.Params) > 0 {
			return i.Params
		}
	}

	return nil
}
