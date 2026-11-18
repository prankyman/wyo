package ozon

import "regexp"

// markValue is a value of "mark" in a JSON body: the field that holds a marking code. A code may hold escaped characters (the
// separator is written \u001d), so a quote that is escaped does not end it.
var markValue = regexp.MustCompile(`("mark"\s*:\s*)"(?:[^"\\]|\\.)*"`)

// redactBody is what the call list keeps of a request's body: a marking code is a secret of the seller (rule R4), so the codes
// of a set are replaced by [redacted] and the rest of the body, which says which exemplar got a code, is kept.
func redactBody(path, body string) string {
	if path != pathExemplarSet {
		return body
	}

	return markValue.ReplaceAllString(body, `${1}"[redacted]"`)
}

// pathExemplarSet is the method that sends the marking codes.
const pathExemplarSet = "/v6/fbs/posting/product/exemplar/set"
