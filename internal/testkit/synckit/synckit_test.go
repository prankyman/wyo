package synckit_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
	"github.com/prankyman/wyo/internal/testkit/storekit"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestTheKitsMarketplaceIsReachedInProcessAndItsClockMovesOnlyWhenTheEngineWaits(t *testing.T) {
	t.Parallel()

	var seen *clock.Fixed

	kit := synckit.NewInProcess(t, func(clk *clock.Fixed) http.Handler {
		seen = clk

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_, _ = io.WriteString(w, r.Method+" "+string(body)) //nolint:gosec // G705: a test's handler echoes the test's own request
		})
	})

	if seen != kit.Clock || !kit.Clock.Now().Equal(synckit.Noon()) {
		t.Fatal("the marketplace is built on the kit's own clock, which starts at noon")
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, kit.URL+"/x", nil)

	res, err := kit.HTTP.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()

	if string(body) != "GET " {
		t.Fatalf("a request without a body reaches the handler with an empty one: %q", body)
	}

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()

	req, _ = http.NewRequestWithContext(cancelled, http.MethodGet, kit.URL+"/x", nil)
	if res, err := kit.HTTP.Do(req); err == nil {
		_ = res.Body.Close()
		t.Fatal("a request whose context ended was answered")
	}

	if (synckit.Middle{}).Int64N(10) != 5 || len(kit.Waits()) != 0 || len(kit.Rows(t)) != 0 || kit.Logger() == nil {
		t.Fatal("the kit's random source gives the middle; nothing waited, nothing journaled yet")
	}

	kit.Clock.Advance(time.Second)
}

func TestAKitOverAGivenDatabaseBuildsTheEngineOfTheGivenAdapters(t *testing.T) {
	t.Parallel()

	st := storekit.Open(t)
	clk := clock.NewFixed(synckit.Noon())
	kit := synckit.NewOn(t, st, clk, http.NotFoundHandler())

	engine, registry := kit.Engine(t, memadapter.New(10))

	if kit.Store != st || kit.Clock != clk || engine == nil {
		t.Fatal("the kit is not built over the given database and clock")
	}

	if _, known := registry.Get(memadapter.Code); !known || len(registry.All()) != 1 {
		t.Fatalf("the registry holds %d adapters", len(registry.All()))
	}

	// The orders, the safety read of the accounts read by changes (Plan 7), the warehouses, the key check, the journal's prune.
	if jobs := engine.Jobs(); len(jobs) != 5 {
		t.Fatalf("the engine has %d jobs", len(jobs))
	}
}
