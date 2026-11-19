//go:build wyotest && wyomeasure

package server_test

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
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

// The measurement of plan 3, section H, widened by plans 5 and 7, section H: the memory of the wyo server with four
// accounts — two of Wildberries, one of Ozon and one of Яндекс Маркет, whose orders have three products on average and
// each a notification — and 10 000 orders in its database, reading the fakes at the plans' periods for a made-up day. It runs only when asked for
// (`pnpm go:measure`, the build tag wyomeasure), because it measures, not checks: the pipeline prints its line, and the
// README's requirements take the number with the date.
//
// What the number is: the peak resident memory of the whole test process (VmHWM of /proc/self/status) and the Go heap
// in use after the day. The process also holds the fakes and the test itself, so the number is an upper bound of the wyo
// server's own memory.
func TestTheMemoryOfADayWithFourAccountsAndTenThousandOrders(t *testing.T) {
	const (
		accountCount = 4
		orderCount   = 10_000
		newEachDay   = 30 // new orders each account has at the fake on the day
	)

	clk := clock.NewFixed(synckit.Noon())
	fake := fakeMarketplaces(t, clk)

	var day *daykit.Day

	cfg := server.Options{
		Clock: clk, Random: synckit.Middle{}, Location: synckit.Noon().Location(),
		Wait: func(ctx context.Context, d time.Duration) error { return day.Wait(ctx, d) },
	}
	cfg.Config.Test.MarketplaceURL = fake.URL

	app := openApp(t, cfg)

	var ids []int64

	for i := range accountCount - 2 {
		name := fmt.Sprintf("account-%d", i+1)
		tokenText := token("measure-"+name, clk.Now())
		ids = append(ids, addAccount(t, app, name, tokenText, clk.Now()))

		control := fakekit.NewControl(fake.URL, tokenText, fake.Client())
		for range newEachDay {
			if err := control.Do(t.Context(), "POST", "/__control/orders", map[string]any{"article": "ART-" + name}, nil); err != nil {
				t.Fatalf("an order at the fake: %v", err)
			}
		}
	}

	ozonID, key := addOzonAccount(t, app, fake.URL, "account-ozon", clk.Now())
	ids = append(ids, ozonID)

	control := fakekit.NewControl(fake.URL+"/"+ozonHost, key["api_key"], fake.Client())
	for i := range newEachDay {
		products := []map[string]any{}
		for line := range 2 + i%3 {
			products = append(products, map[string]any{"sku": 2001, "offer_id": fmt.Sprintf("ART-%d", line), "name": "Товар", "quantity": 1 + line})
		}

		if err := control.Do(t.Context(), "POST", "/__control/postings", map[string]any{
			"posting_number": fmt.Sprintf("20000000-%04d-1", i), "order_number": "20000000", "warehouse_id": 1001,
			"shipment_date": clk.Now().Add(5 * time.Hour), "products": products,
		}, nil); err != nil {
			t.Fatalf("a posting at the fake: %v", err)
		}
	}

	ids = append(ids, measuredYandexAccount(t, app, fake.URL, clk.Now(), newEachDay))

	seedOrders(t, app, ids, orderCount, clk.Now())

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

	rows, err := app.Store.Read().CountJournalEntries(t.Context(), store.JournalFilter{})
	if err != nil {
		t.Fatalf("CountJournalEntries: %v", err)
	}

	orders, err := app.Store.Read().CountOrderViews(t.Context(), store.OrderFilter{})
	if err != nil {
		t.Fatalf("CountOrderViews: %v", err)
	}

	runtime.GC()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	t.Logf("wyo-memory: peak_rss_kb=%d rss_kb=%d go_heap_in_use_kb=%d go_sys_kb=%d accounts=%d orders=%d journal_rows=%d day=%s",
		status(t, "VmHWM"), status(t, "VmRSS"), mem.HeapInuse/1024, mem.Sys/1024, accountCount, orders, rows, clk.Now().Format(time.RFC3339))

	if orders < orderCount || rows == 0 {
		t.Fatalf("the day did not run as measured: %d orders, %d journal rows", orders, rows)
	}
}

// seedOrders writes count orders spread over the 90 days before now, shared among the accounts: seven in ten packed by
// the seller, most of them received long ago, one in twenty still on its way (so the states are asked for).
func seedOrders(t *testing.T, app *server.App, accounts []int64, count int, now time.Time) {
	t.Helper()

	if err := app.Store.Do(t.Context(), func(q *store.Queries) error {
		for i := range count {
			state, scheme := "received", "fbs"

			switch {
			case i%20 == 0:
				state = "in_delivery"
			case i%13 == 0:
				state = "cancelled"
			}

			if i%10 >= 7 {
				scheme = "fbo"
			}

			created := now.Add(-time.Duration(i%90)*24*time.Hour - time.Duration(i%600)*time.Minute)
			number := strconv.Itoa(900_000_000 + i)

			// The orders of the last account, the Яндекс Маркет one, have two, three or four products: three on average.
			account := i % len(accounts)
			lines := []store.OrderLine{{Article: fmt.Sprintf("ART-%d", i%400), Barcodes: []string{fmt.Sprintf("46%011d", i)}, Quantity: 1}}

			if account == len(accounts)-1 {
				for extra := range 1 + i%3 {
					lines = append(lines, store.OrderLine{Article: fmt.Sprintf("ART-%d-%d", i%400, extra), Quantity: 1 + extra})
				}
			}

			if _, _, err := q.InsertOrder(t.Context(), store.Order{
				AccountID: accounts[account], Number: number, Scheme: scheme, JoinKey: "srid-" + number, Lines: lines, CreatedAt: created,
				State: state, SellerStatus: "complete", MarketplaceStatus: "sold", UpdatedAt: created,
			}); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		t.Fatalf("seed the orders: %v", err)
	}
}

// status reads one value in kB from /proc/self/status (Linux).
func status(t *testing.T, field string) int {
	t.Helper()

	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Fatalf("read /proc/self/status: %v", err)
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, field+":"); ok {
			kb, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), "kB")))
			if err != nil {
				t.Fatalf("%s: %v", field, err)
			}

			return kb
		}
	}

	t.Fatalf("no %s in /proc/self/status", field)

	return 0
}

// measuredYandexAccount adds the Яндекс Маркет account of the measurement: count orders at the fake of two, three or four
// products, each named by a notification the wyo server stored and read, as notifications for every change bring them.
func measuredYandexAccount(t *testing.T, app *server.App, fakeURL string, now time.Time, count int) int64 {
	t.Helper()

	id := addYandexAccount(t, app, fakeURL, "account-yandex", now)
	key := "yandex-account-yandex-" + strings.ReplaceAll(t.Name(), "/", "-")
	control := fakekit.NewControl(fakeURL+"/"+yandexHost, key, nil)

	var numbers []string

	for i := range count {
		items := []map[string]any{}
		for line := range 2 + i%3 {
			items = append(items, map[string]any{"offer_id": fmt.Sprintf("ART-%d", line), "offer_name": "Товар", "count": 1 + line})
		}

		var out struct {
			OrderID int64 `json:"order_id"`
		}

		if err := control.Do(t.Context(), "POST", "/__control/orders", map[string]any{"campaign_id": 5001, "items": items}, &out); err != nil {
			t.Fatalf("an order at the fake Яндекс Маркет: %v", err)
		}

		numbers = append(numbers, strconv.FormatInt(out.OrderID, 10))
	}

	if err := app.Store.Do(t.Context(), func(q *store.Queries) error {
		for _, n := range numbers {
			if _, err := q.InsertNotification(t.Context(), store.Notification{AccountID: id, At: now, Kind: "ORDER_CREATED", Numbers: []string{n}}); err != nil {
				return err
			}
		}

		return q.RecordNotificationSeen(t.Context(), id, now)
	}); err != nil {
		t.Fatalf("store the notifications: %v", err)
	}

	if err := app.Engine.ReadNotified(t.Context()); err != nil {
		t.Fatalf("ReadNotified: %v", err)
	}

	return id
}
