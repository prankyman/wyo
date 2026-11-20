//go:build wyotest

package server_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/daykit"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// Rule R3, alone by default (plan 3, section F; plan 5, section F): the wyo server's scheduler runs through a made-up day
// with «External API» empty, behind an outbound gate that records every address its clients are asked to call. With
// accounts, only the hosts of their marketplaces appear (here through the fakes on one address), besides the release
// address and the certificate authority; a marketplace with no account is never reached (Яндекс Маркет's host among them);
// with no account, no marketplace host appears at all.
func TestAMadeUpDayReachesOnlyTheMarketplacesOfTheAccounts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		wildberries int
		ozon        int
		yandex      int
	}{
		{name: "no account"},
		{name: "two Wildberries accounts", wildberries: 2},
		{name: "a Wildberries and an Ozon account", wildberries: 1, ozon: 1},
		{name: "a Wildberries, an Ozon and a Яндекс Маркет account", wildberries: 1, ozon: 1, yandex: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clk := clock.NewFixed(synckit.Noon())
			fake := fakeMarketplaces(t, clk)
			gate, record := recordingGate(t, fake.URL)

			var day *daykit.Day

			app := openApp(t, server.Options{
				Clock: clk, Gate: gate, Random: synckit.Middle{}, Location: synckit.Noon().Location(),
				Wait: func(ctx context.Context, d time.Duration) error { return day.Wait(ctx, d) },
			})

			for i := range tc.wildberries {
				name := []string{"north", "south"}[i]
				addAccount(t, app, name, token(t.Name()+name, clk.Now()), clk.Now())
			}

			for range tc.ozon {
				addOzonAccount(t, app, fake.URL, "east", clk.Now())
			}

			for range tc.yandex {
				addYandexAccount(t, app, fake.URL, "west", clk.Now())

				// Orders arrive during the day, one of them a test order: with «Внешний API» empty they are only read.
				key := "yandex-west-" + strings.ReplaceAll(t.Name(), "/", "-")
				for _, test := range []bool{false, true} {
					if err := fakekit.NewControl(fake.URL+"/"+yandexHost, key, nil).Do(t.Context(), http.MethodPost, "/__control/orders", map[string]any{
						"campaign_id": 5001, "fake": test, "items": []map[string]any{{"offer_id": "SHIRT", "count": 1, "required_instance_types": []string{"CIS"}}},
					}, nil); err != nil {
						t.Fatalf("an order at the fake Яндекс Маркет: %v", err)
					}
				}
			}

			// One party per job: the copy's, the engine's and the exchange's (which do nothing while «Внешний API» is empty).
			day = daykit.New(clk, 1+len(app.Engine.Jobs())+len(app.Exchange.Jobs()), clk.Now().Add(24*time.Hour))

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
			onlyMarketplacesThroughTheFake(t, recorded, fake.URL)

			rows, err := app.Store.Read().JournalEntries(t.Context(), store.JournalFilter{PageSize: 100000})
			if err != nil {
				t.Fatalf("JournalEntries: %v", err)
			}

			if tc.wildberries+tc.ozon+tc.yandex == 0 {
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

			if reached[ozonHost] != (tc.ozon > 0) {
				t.Fatalf("Ozon's host reached: %v, with %d Ozon accounts; hosts %v", reached[ozonHost], tc.ozon, reached)
			}

			if reached[yandexHost] != (tc.yandex > 0) {
				t.Fatalf("Яндекс Маркет's host reached: %v, with %d accounts of it; hosts %v", reached[yandexHost], tc.yandex, reached)
			}

			// Nothing but readings reached a marketplace: no box report, status, courier's code or confirmation of a shipment.
			for _, row := range rows {
				if strings.HasSuffix(row.Path, "/boxes") || strings.HasSuffix(row.Path, "/verifyEac") || strings.HasSuffix(row.Path, "/confirm") ||
					(strings.HasSuffix(row.Path, "/status") && !strings.HasSuffix(row.Path, "/identifiers/status")) {
					t.Fatalf("with «Внешний API» empty the wyo server wrote %s %s", row.Method, row.Path)
				}
			}
		})
	}
}
