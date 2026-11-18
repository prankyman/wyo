package logging_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/logging"
)

// canary is a made-up key. It must never appear in anything the logger writes.
const (
	canary    = "CANARY-key-8f3a91c7d2e45b60"
	canaryJWT = "eyJhbGciOiJFUzI1NiJ9.eyJzaWQiOiJjYW5hcnkifQ.c2lnbmF0dXJlLWNhbmFyeQ"
)

func lines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var out []map[string]any

	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}

		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("a log line is not JSON: %q: %v", line, err)
		}

		out = append(out, entry)
	}

	return out
}

func TestNewWritesOneJSONObjectPerLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	log := logging.New(&buf, slog.LevelInfo)
	log.Info("listening", "address", "127.0.0.1:8790")
	log.Debug("not written at the info level")

	got := lines(t, &buf)
	if len(got) != 1 {
		t.Fatalf("want 1 line, got %d: %s", len(got), buf.String())
	}

	if got[0]["msg"] != "listening" || got[0]["address"] != "127.0.0.1:8790" || got[0]["level"] != "INFO" {
		t.Fatalf("unexpected line: %v", got[0])
	}
}

func TestMask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "a long key shows its last four characters", in: "abcdef123456", want: "…3456"},
		{name: "nine characters are the shortest that show four", in: "123456789", want: "…6789"},
		{name: "eight characters are all hidden", in: "12345678", want: "…"},
		{name: "a short key is all hidden", in: "ab", want: "…"},
		{name: "an empty key stays empty", in: "", want: ""},
		{name: "counts characters, not bytes", in: "ключ-секрет-ёжик", want: "…ёжик"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := logging.Mask(tc.in); got != tc.want {
				t.Fatalf("Mask(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSecretNeverShowsItsValue(t *testing.T) {
	t.Parallel()

	secret := logging.Secret(canary)

	tests := []struct {
		name string
		got  string
	}{
		{name: "String", got: secret.String()},
		{name: "GoString", got: secret.GoString()},
		{name: "%v", got: fmt.Sprintf("%v", secret)},
		{name: "%+v", got: fmt.Sprintf("%+v", secret)},
		{name: "%#v", got: fmt.Sprintf("%#v", secret)},
		{name: "Sprint", got: fmt.Sprint(secret)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if strings.Contains(tc.got, canary) {
				t.Fatalf("%s printed the secret: %q", tc.name, tc.got)
			}
		})
	}

	asJSON, err := json.Marshal(struct{ Key logging.Secret }{Key: secret})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(asJSON), canary) {
		t.Fatalf("JSON carried the secret: %s", asJSON)
	}

	if secret.Reveal() != canary {
		t.Fatal("Reveal must return the real value to the code that needs to send it")
	}
}

type holder struct {
	Name string
	Key  logging.Secret
}

// TestCanaryNeverReachesALogLine is the canary test: every way a key could be handed to the logger.
func TestCanaryNeverReachesALogLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		log  func(l *slog.Logger)
	}{
		{name: "a Secret value", log: func(l *slog.Logger) { l.Info("call", "who", logging.Secret(canary)) }},
		{name: "a Secret inside a struct", log: func(l *slog.Logger) { l.Info("call", "holder", holder{Name: "n", Key: canary}) }},
		{name: "a Secret inside a group", log: func(l *slog.Logger) { l.Info("call", slog.Group("account", "key", logging.Secret(canary))) }},
		{name: "a field named key", log: func(l *slog.Logger) { l.Info("call", "key", canary) }},
		{name: "a field named api_key in capitals", log: func(l *slog.Logger) { l.Info("call", "API_KEY", canary) }},
		{name: "a field named apiKey", log: func(l *slog.Logger) { l.Info("call", "apiKey", canary) }},
		{name: "a field named api-key", log: func(l *slog.Logger) { l.Info("call", "api-key", canary) }},
		{name: "a field named keys", log: func(l *slog.Logger) { l.Info("call", "keys", canary) }},
		{name: "a field named token", log: func(l *slog.Logger) { l.Info("call", "token", canary) }},
		{name: "a field named authorization", log: func(l *slog.Logger) { l.Info("call", "authorization", canary) }},
		{name: "a field named password", log: func(l *slog.Logger) { l.Info("call", "password", canary) }},
		{name: "a field named secret inside a group", log: func(l *slog.Logger) { l.Info("call", slog.Group("g", "client_secret", canary)) }},
		{name: "a Bearer header in a text", log: func(l *slog.Logger) { l.Info("sent Authorization: Bearer " + canary) }},
		{name: "a Bearer header in a field", log: func(l *slog.Logger) { l.Info("call", "header", "Bearer "+canary) }},
		{name: "a Bearer header in an error", log: func(l *slog.Logger) { l.Error("failed", "err", errors.New("401 for Bearer "+canary)) }},
		{name: "a token that is a JWT in a field", log: func(l *slog.Logger) { l.Info("call", "value", canaryJWT) }},
		{name: "a token that is a JWT in an error", log: func(l *slog.Logger) {
			l.Error("failed", "err", fmt.Errorf("read %s: %w", canaryJWT, errors.New("bad")))
		}},
		{name: "a logger made with With", log: func(l *slog.Logger) { l.With("token", canary).Info("call") }},
		{name: "a logger made with WithGroup", log: func(l *slog.Logger) { l.WithGroup("g").With("password", canary).Info("call") }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			tc.log(logging.New(&buf, slog.LevelDebug))

			out := buf.String()
			if out == "" {
				t.Fatal("nothing was logged, the canary test proves nothing")
			}

			for _, forbidden := range []string{canary, "8f3a91c7d2e45b60", canaryJWT, "c2lnbmF0dXJlLWNhbmFyeQ"} {
				if strings.Contains(out, forbidden) {
					t.Fatalf("the log line carries %q: %s", forbidden, out)
				}
			}
		})
	}
}

func TestOrdinaryValuesAreKept(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logging.New(&buf, slog.LevelInfo).Info("synchronized", "orders", 12, "monkey", "banana", "note", "a Bearer of news")

	got := lines(t, &buf)[0]
	if got["orders"] != float64(12) || got["monkey"] != "banana" || got["note"] != "a Bearer of news" {
		t.Fatalf("an ordinary value was changed: %v", got)
	}
}

func TestRedactedFieldsSayTheyWereRedacted(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logging.New(&buf, slog.LevelInfo).Info("call", "token", canary)

	if got := lines(t, &buf)[0]["token"]; got != logging.Redacted {
		t.Fatalf("want %q, got %v", logging.Redacted, got)
	}
}

func TestScrubBlanksASignedTokenAndABearerKeyInAnyText(t *testing.T) {
	t.Parallel()

	text := "refused eyJhbGciOiJFUzI1NiJ9.eyJzaWQiOiIxIn0.c2lnbmF0dXJl and Bearer abcdefgh12345678, status 401"

	got := logging.Scrub(text)
	if got != "refused [redacted] and Bearer [redacted], status 401" {
		t.Fatalf("Scrub = %q", got)
	}
}
