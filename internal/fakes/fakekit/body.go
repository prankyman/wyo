package fakekit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// ControlBody reads the JSON body of a control call into dst and refuses a field dst does not have, so that a test that
// misspells one learns it at once. An empty body leaves dst as it was. When the body cannot be used it answers 400
// itself and returns false.
func ControlBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		ControlError(w, http.StatusBadRequest, "unreadable body")

		return false
	}

	if len(raw) == 0 {
		return true
	}

	if err := strictJSON(raw, dst); err != nil {
		ControlError(w, http.StatusBadRequest, "the body is not valid: "+err.Error())

		return false
	}

	return true
}

// strictJSON decodes one JSON value into dst and refuses fields dst does not have.
func strictJSON(raw []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	return decoder.Decode(dst)
}

// ControlOK answers a control call that has been done: {"status": "ok"}.
func ControlOK(w http.ResponseWriter) {
	JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
