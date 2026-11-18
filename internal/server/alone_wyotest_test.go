//go:build wyotest

package server_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/daykit"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// Rule R3, alone by default (plan 3, section F): the wyo server's scheduler runs through a made-up day with «External
// API» empty, behind an outbound gate that records every address its clients are asked to call. With accounts, only the
// marketplaces' hosts appear (here through the fake Wildberries), besides the release address and the certificate
// authority; with no account, no marketplace host appears at all.
func TestAMadeUpDayReachesOnlyTheMarketplacesOfTheAccounts(t *testing.T) {
	t.Parallel()

	for _, accounts := range []int{0, 2} {
		t.Run(map[int]string{0: "no account", 2: "two accounts"}[accounts], func(t *testing.T) {
			t.Parallel()

			clk := clock.NewFixed(synckit.Noon())
			fake := fakeWildberries(t, clk)
			gate, record := recordingGate(t, fake.URL)

			var day *daykit.Day

			app := openApp(t, server.Options{
				Clock: clk, Gate: gate, Random: synckit.Middle{}, Location: synckit.Noon().Location(),
				Wait: func(ctx context.Context, d time.Duration) error { return day.Wait(ctx, d) },
			})

			for i := range accounts {
				name := []string{"north", "south"}[i]
				addAccount(t, app, name, token(t.Name()+name, clk.Now()), clk.Now())
			}

			// One party per job: the copy's and the engine's.
			day = daykit.New(clk, 1+len(app.Engine.Jobs()), clk.Now().Add(24*time.Hour))

			if err := app.RunWith(t.Context(), func(ctx context.Context) error {
				select {
				case <-day.Done():
				case <-ctx.Done():
				}

				return nil
			}); err != nil {
				t.Fatalf("RunWith: %v", err)
			}

			recorded := record.all()
			onlyWildberriesThroughTheFake(t, recorded, fake.URL)

			rows, err := app.Store.Read().JournalEntries(t.Context(), store.JournalFilter{PageSize: 100000})
			if err != nil {
				t.Fatalf("JournalEntries: %v", err)
			}

			if accounts == 0 {
				if len(recorded) != 0 || len(rows) != 0 {
					t.Fatalf("with no account the wyo server reached %v and journaled %d requests", recorded, len(rows))
				}

				return
			}

			// Every 30 minutes for a day: the readings happened, each one a journal row, each one recorded by the gate.
			reached := map[string]bool{}
			for _, line := range recorded {
				_, address, _ := strings.Cut(line, " ")
				reached[hostOfFake(address, fake.URL)] = true
			}

			if !reached["marketplace-api.wildberries.ru"] || !reached["statistics-api.wildberries.ru"] || len(rows) < 2*48 {
				t.Fatalf("hosts %v, %d journal rows, %d recorded addresses: the day's readings did not happen", reached, len(rows), len(recorded))
			}
		})
	}
}
