package release

import (
	"context"
	"net/http"

	"github.com/prankyman/wyo/internal/outbound"
)

// ReadyProbe makes the updater's question "does the new wyo server answer?": it asks the wyo server's own /readyz, on
// this machine (outbound.LocalOrigin, or outbound.LocalProxyOrigin behind a web server), and is true only for 200. The
// client is the one of outbound.PurposeLocal. A redirect is not an answer.
func ReadyProbe(doer outbound.Doer, address string) func(context.Context) bool {
	return func(ctx context.Context) bool {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return false
		}

		res, err := doer.Do(req)
		if err != nil {
			return false
		}

		_ = res.Body.Close()

		return res.StatusCode == http.StatusOK
	}
}
