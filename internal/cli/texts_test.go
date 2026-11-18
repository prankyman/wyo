package cli

import (
	"bytes"
	"testing"
	"testing/fstest"
)

func TestTextsFillParametersAndFallBackToTheKey(t *testing.T) {
	t.Parallel()

	texts, err := loadTexts(fstest.MapFS{
		"ru.json": {Data: []byte(`{"cli.hello": "Привет, {name}! Версия {version}.", "cli.plain": "Готово."}`)},
	}, "ru")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		key    string
		params map[string]any
		want   string
	}{
		{name: "parameters", key: "cli.hello", params: map[string]any{"name": "владелец", "version": "0.0.2"}, want: "Привет, владелец! Версия 0.0.2."},
		{name: "no parameters", key: "cli.plain", want: "Готово."},
		{name: "a key with no text is written as itself", key: "cli.absent", want: "cli.absent"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := texts.T(tc.key, tc.params); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}

	var out bytes.Buffer
	texts.Say(&out, "cli.plain", nil)

	if out.String() != "Готово.\n" {
		t.Fatalf("Say wrote %q", out.String())
	}
}

func TestTextsOfAMissingOrBrokenFileFail(t *testing.T) {
	t.Parallel()

	if _, err := loadTexts(fstest.MapFS{}, "ru"); err == nil {
		t.Fatal("a missing language file must be an error")
	}

	if _, err := loadTexts(fstest.MapFS{"ru.json": {Data: []byte("{")}}, "ru"); err == nil {
		t.Fatal("a broken language file must be an error")
	}
}

func TestTheEmbeddedRussianTextsHaveEveryCommandsKey(t *testing.T) {
	t.Parallel()

	texts := defaultTexts()
	for _, key := range []string{"cli.usage", "cli.unknownCommand", "cli.failed"} {
		if got := texts.T(key, nil); got == key {
			t.Errorf("messages/ru.json has no text for %q", key)
		}
	}
}
