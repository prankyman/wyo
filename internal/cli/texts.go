package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/prankyman/wyo/messages"
)

// defaultLanguage is the one language written in code (constitution, principle I).
const defaultLanguage = "ru"

// texts words what a command says to a person: every sentence is a key of messages/<language>.json, the same file the
// interface reads, with {name} placeholders. No sentence is written in Go.
type texts struct {
	messages map[string]string
}

func loadTexts(files fs.FS, language string) (texts, error) {
	data, err := fs.ReadFile(files, language+".json")
	if err != nil {
		return texts{}, fmt.Errorf("read the texts of %q: %w", language, err)
	}

	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return texts{}, fmt.Errorf("read the texts of %q: %w", language, err)
	}

	return texts{messages: m}, nil
}

// defaultTexts are the embedded Russian texts. The file is part of the program, so failing to read it is a broken build.
func defaultTexts() texts {
	t, err := loadTexts(messages.Files, defaultLanguage)
	if err != nil {
		panic(err)
	}

	return t
}

// T returns the text of key with its placeholders filled; a key with no text is returned as itself, so a missing text is
// visible instead of silent.
func (t texts) T(key string, params map[string]any) string {
	message, ok := t.messages[key]
	if !ok {
		return key
	}

	for name, value := range params {
		message = strings.ReplaceAll(message, "{"+name+"}", fmt.Sprint(value))
	}

	return message
}

// Say writes the text of key and a line end.
func (t texts) Say(w io.Writer, key string, params map[string]any) {
	_, _ = fmt.Fprintln(w, t.T(key, params))
}
