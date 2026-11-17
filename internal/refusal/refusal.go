// Package refusal writes the wyo server's one shape of refusal: {"error":{"code","params"?,"issues"?}}. The server
// never writes a sentence for a person; the interface turns the code into a text.
package refusal

import (
	"encoding/json"
	"net/http"
)

// Issue names one field that was refused and why.
type Issue struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

// Error is the refusal itself.
type Error struct {
	Code   string         `json:"code"`
	Params map[string]any `json:"params,omitempty"`
	Issues []Issue        `json:"issues,omitempty"`
}

// Body is the whole answer.
type Body struct {
	Error Error `json:"error"`
}

// Write answers with the given status and refusal.
func Write(w http.ResponseWriter, status int, e Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// The status line is already sent; a failed write to a closed connection has nobody left to tell.
	_ = json.NewEncoder(w).Encode(Body{Error: e})
}

// Code answers with the given status and a refusal that is only a code.
func Code(w http.ResponseWriter, status int, code string) {
	Write(w, status, Error{Code: code})
}
