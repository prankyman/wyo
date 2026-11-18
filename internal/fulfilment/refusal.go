package fulfilment

import (
	"errors"
	"net/http"

	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/marketplace"
)

// The states of «Внешний API» (api/wyo.yaml, ExternalApiState), besides "empty" (no row).
const (
	StateWorking      = "working"
	StateKeyRefused   = "key_refused"
	StateReadOnlyKey  = "read_only_key"
	StateAPIOff       = "api_off"
	StateForbidden    = "forbidden"
	StateUnavailable  = "unavailable"
	StateNoConnection = "no_connection"
	StateUnreadable   = "unreadable"
)

// Verdict is what a failed call to the fulfilment centre means: the state «Внешний API» falls into (empty: the setting
// stays working), and whether the call may simply be tried again by the next run.
type Verdict struct {
	State string
	Again bool
}

// refusals is THE table of the fulfilment centre's refusals (docs/fulfilment-centre-api.md, «Refusals»): what each one
// means for the setting. A refusal not listed is about one call only (a bad body, a conflict of one order): the setting
// stays working and the call is not repeated blindly.
var refusals = map[string]Verdict{
	"unauthorized":            {State: StateKeyRefused},
	"api_off":                 {State: StateAPIOff},
	"forbidden":               {State: StateForbidden},
	"read_only_key":           {State: StateReadOnlyKey},
	"unavailable":             {State: StateUnavailable, Again: true},
	"rate_limited":            {Again: true},
	"marketplace_not_enabled": {},
	"external_id_reused":      {},
	"handed_over":             {},
	"conflict":                {},
	"not_found":               {},
	"not_api_order":           {},
	"bad_request":             {},
	"too_large":               {},
}

// VerdictOf reads a failed call: a refusal of the fulfilment centre by its table, a call that met no answer (or a server
// error after the repeats, or a limit to wait out) as «no connection» to be tried again, anything else as an answer the
// wyo server could not read.
func VerdictOf(err error) Verdict {
	var refused *Refused
	if errors.As(err, &refused) {
		if v, known := refusals[refused.Code]; known {
			return v
		}

		switch {
		case refused.Status == http.StatusUnauthorized:
			return Verdict{State: StateKeyRefused}
		case refused.Status >= http.StatusInternalServerError:
			return Verdict{State: StateNoConnection, Again: true}
		}

		return Verdict{}
	}

	var callErr *marketplace.CallError
	if errors.As(err, &callErr) {
		// The caller hands a server error back after its repeats with the code the answer carried (503 unavailable: the
		// fulfilment centre is in maintenance).
		if v, known := refusals[callErr.Code]; known && v.State != "" {
			return v
		}

		if callErr.Outcome == journal.Limited {
			return Verdict{Again: true}
		}

		return Verdict{State: StateNoConnection, Again: true}
	}

	return Verdict{State: StateUnreadable, Again: true}
}

// CodeOf is the fulfilment centre's code of a refusal, or "" for a call that met no refusal.
func CodeOf(err error) string {
	var refused *Refused
	if errors.As(err, &refused) {
		return refused.Code
	}

	return ""
}
