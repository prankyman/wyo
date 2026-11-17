// Package logging builds the wyo server's logger: one JSON object per line, with every key kept out of it.
//
// A key reaches no log line by four independent barriers: the Secret type prints as nothing in every verb; a field
// whose name says it holds a credential is replaced; a text that looks like an Authorization header or a signed token
// is cut out wherever it stands (a message, a field, an error's text); and Mask is the one way to show a key.
package logging

import (
	"io"
	"log/slog"
	"regexp"
	"strings"
)

// Redacted replaces everything that must not be written.
const Redacted = "[redacted]"

// Secret holds a credential. It prints as Redacted in every way it can be printed or encoded; only Reveal returns the value,
// and only the code that sends the credential should call it.
type Secret string

// Reveal returns the credential.
func (s Secret) Reveal() string { return string(s) }

// String implements fmt.Stringer.
func (Secret) String() string { return Redacted }

// GoString implements fmt.GoStringer, used by %#v.
func (Secret) GoString() string { return Redacted }

// LogValue implements slog.LogValuer.
func (Secret) LogValue() slog.Value { return slog.StringValue(Redacted) }

// MarshalText implements encoding.TextMarshaler (and so the JSON encoding).
func (Secret) MarshalText() ([]byte, error) { return []byte(Redacted), nil }

// maskedTail is how many characters of a key Mask shows.
const maskedTail = 4

// Mask shows only the last four characters of a key, for a screen or a line that must tell two keys apart. A key too
// short to hide its beginning is hidden whole.
func Mask(key string) string {
	if key == "" {
		return ""
	}

	runes := []rune(key)
	if len(runes) <= 2*maskedTail {
		return "…"
	}

	return "…" + string(runes[len(runes)-maskedTail:])
}

// New returns a logger that writes JSON lines to w at the given level and above.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceAttr,
	}))
}

// sensitiveWords are the words that, as a whole word of a field's name, make the field hold a credential. A word is
// whole: "api_key", "apiKey" and "keys" are sensitive, "monkey" is not.
var sensitiveWords = map[string]bool{
	"key": true, "apikey": true, "token": true, "secret": true, "password": true, "passwd": true,
	"authorization": true, "credential": true, "cookie": true,
}

var (
	// "Bearer" followed by something that can be a credential (8 or more characters of a token's alphabet).
	bearerPattern = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{8,}`)
	camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	// A signed token: three base64url parts separated by dots, the first two beginning like a JSON object ("eyJ").
	jwtPattern = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*`)
)

func replaceAttr(_ []string, a slog.Attr) slog.Attr {
	if isSensitiveName(a.Key) {
		return slog.String(a.Key, Redacted)
	}

	switch a.Value.Kind() {
	case slog.KindString:
		a.Value = slog.StringValue(scrub(a.Value.String()))
	case slog.KindAny:
		if err, ok := a.Value.Any().(error); ok {
			a.Value = slog.StringValue(scrub(err.Error()))
		}
	case slog.KindBool, slog.KindDuration, slog.KindFloat64, slog.KindInt64, slog.KindTime, slog.KindUint64,
		slog.KindGroup, slog.KindLogValuer:
	}

	return a
}

func isSensitiveName(name string) bool {
	words := strings.FieldsFunc(strings.ToLower(camelBoundary.ReplaceAllString(name, "${1}_${2}")), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})

	for _, word := range words {
		if sensitiveWords[word] || sensitiveWords[strings.TrimSuffix(word, "s")] {
			return true
		}
	}

	return false
}

func scrub(text string) string {
	text = bearerPattern.ReplaceAllString(text, "Bearer "+Redacted)

	return jwtPattern.ReplaceAllString(text, Redacted)
}
