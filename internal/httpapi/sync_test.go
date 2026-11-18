package httpapi_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/httpapi"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/apikit"
)

func TestRefreshIsAnsweredAtMostOnceInFiveMinutes(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	noon := apikit.Noon().UTC()
	next := noon.Add(5 * time.Minute)

	if before := o.call(t, http.MethodGet, "/api/v1/sync/refresh", ""); before.Status != http.StatusOK || before.Body != `{}` {
		t.Fatalf("never refreshed: %d %s", before.Status, before.Body)
	}

	started := o.call(t, http.MethodPost, "/api/v1/sync/refresh", `{}`)
	state := fmt.Sprintf(`{"lastRefreshAt":%q,"nextAllowedAt":%q}`, noon.Format(time.RFC3339), next.Format(time.RFC3339))

	if started.Status != http.StatusAccepted || started.Body != state {
		t.Fatalf("the first press: %d %s", started.Status, started.Body)
	}

	o.f.Clock.Advance(4 * time.Minute)

	soon := o.call(t, http.MethodPost, "/api/v1/sync/refresh", `{}`)
	if want := fmt.Sprintf(`{"error":{"code":"too_soon","params":{"nextAllowedAt":%q}}}`, next.Format(time.RFC3339)); soon.Status != http.StatusTooManyRequests || soon.Body != want {
		t.Fatalf("a press too soon: %d %s", soon.Status, soon.Body)
	}

	if waiting := o.call(t, http.MethodGet, "/api/v1/sync/refresh", ""); waiting.Body != state {
		t.Fatalf("while it may not be pressed: %s", waiting.Body)
	}

	o.f.Clock.Advance(time.Minute)

	if again := o.call(t, http.MethodGet, "/api/v1/sync/refresh", ""); again.Body != fmt.Sprintf(`{"lastRefreshAt":%q}`, noon.Format(time.RFC3339)) {
		t.Fatalf("after five minutes: %s", again.Body)
	}
}

// failingRefresher is a refresher whose start fails for a reason of its own.
type failingRefresher struct{}

func (failingRefresher) Refresh(context.Context) (sync.RefreshState, error) {
	return sync.RefreshState{}, errors.New("the readings could not start")
}

func (failingRefresher) RefreshState() sync.RefreshState { return sync.RefreshState{} }

func TestARefreshThatCannotStartIsAnsweredInternal(t *testing.T) {
	t.Parallel()

	o := signedIn(t)

	deps := o.f.Deps
	deps.Refresher = failingRefresher{}

	h, err := httpapi.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got := apikit.Do(t, h, apikit.Request{Method: http.MethodPost, Path: "/api/v1/sync/refresh", Body: `{}`, Cookie: o.cookie})
	if got.Status != http.StatusInternalServerError || got.Body != `{"error":{"code":"internal"}}` || !strings.Contains(o.f.Logs.String(), "the readings could not start") {
		t.Fatalf("%d %s", got.Status, got.Body)
	}
}
