package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/refusal"
	"github.com/prankyman/wyo/internal/sync"
)

func refreshBody(s sync.RefreshState) gen.RefreshState {
	return gen.RefreshState{LastRefreshAt: moment(s.LastAt.UTC()), NextAllowedAt: moment(s.NextAllowedAt.UTC())}
}

// GetRefresh answers GET /api/v1/sync/refresh.
func (a *api) GetRefresh(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, refreshBody(a.deps.Refresher.RefreshState()))
}

// RefreshOrders answers POST /api/v1/sync/refresh: the readings start under the wyo server's lifetime and go on after
// the answer. Pressed again within five minutes, it is refused with `too_soon` and the moment it may be pressed. The body
// (the interface sends {}) carries nothing.
func (a *api) RefreshOrders(w http.ResponseWriter, r *http.Request) {
	state, err := a.deps.Refresher.Refresh(a.deps.Lifetime)
	if errors.Is(err, sync.ErrTooSoon) {
		refusal.Write(w, http.StatusTooManyRequests, refusal.Error{
			Code: "too_soon", Params: map[string]any{"nextAllowedAt": state.NextAllowedAt.UTC().Format(time.RFC3339)},
		})

		return
	}

	if err != nil {
		a.fail(w, r, err)

		return
	}

	writeJSON(w, http.StatusAccepted, refreshBody(state))
}
