// Package adapterkit holds the helpers every marketplace adapter shares, written once (rule «everything is a component
// and is reused»): reading the optional fields of a generated client's answers, writing a marketplace's number as text,
// cutting a marketplace's long refusal text, telling a success from an answer without a body, and cutting a list into
// requests of at most N. It lives outside internal/marketplace, where every folder is an adapter, and knows no
// marketplace.
package adapterkit

import (
	"fmt"
	"net/http"
)

// maxRefusalText is how much of a marketplace's message a refusal keeps.
const maxRefusalText = 300

// Value reads an optional field: its value, or the zero value when it is absent.
func Value[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}

	return *p
}

// Number writes a marketplace's number as text, "" when it is absent: the connector never counts with it.
func Number[T int | int64 | uint64](p *T) string {
	if p == nil {
		return ""
	}

	return fmt.Sprint(*p)
}

// Cut keeps the first 300 characters of a marketplace's text.
func Cut(text string) string {
	if runes := []rune(text); len(runes) > maxRefusalText {
		return string(runes[:maxRefusalText])
	}

	return text
}

// Succeeded says whether an answer's status is a success (2xx).
func Succeeded(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}

// EachPart calls fn with the list in parts of at most size, in order, and stops at the first error.
func EachPart[T any](list []T, size int, fn func(part []T) error) error {
	for start := 0; start < len(list); start += size {
		if err := fn(list[start:min(start+size, len(list))]); err != nil {
			return err
		}
	}

	return nil
}
