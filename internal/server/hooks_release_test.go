//go:build !wyotest

package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// A released wyo server takes a marketplace's notification only from the networks its adapter declares: a call from this
// machine, which a build for development and tests accepts from a fake marketplace, is an address that does not exist, and
// the same call from one of Яндекс Маркет's networks is answered as Яндекс Маркет wants.
func TestAReleasedServerTakesANotificationOnlyFromTheMarketplacesOwnNetworks(t *testing.T) {
	t.Parallel()

	app := openApp(t, server.Options{})
	now := synckit.Noon()

	if err := app.Store.Do(t.Context(), func(q *store.Queries) error {
		_, err := q.InsertAccount(t.Context(), store.Account{
			Marketplace: "yandex", Name: "Маркет", State: "working", SellerProfileID: "7001", SecretName: "account.y.key", KeyLast4: "abcd",
			DeadlineHours: 48, CreatedAt: now, HookText: "hook-release",
		})

		return err
	}); err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	for _, tc := range []struct {
		from   string
		status int
	}{
		{from: "127.0.0.1:40000", status: http.StatusNotFound},
		{from: "5.45.207.10:40000", status: http.StatusOK},
	} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hooks/marketplace/hook-release",
			strings.NewReader(`{"notificationType":"PING","time":"2026-11-18T09:00:00Z"}`))
		req.RemoteAddr = tc.from

		rec := httptest.NewRecorder()
		app.Handler().ServeHTTP(rec, req)

		if rec.Code != tc.status {
			t.Errorf("a notification from %s: %d %s, want %d", tc.from, rec.Code, rec.Body.String(), tc.status)
		}
	}
}
