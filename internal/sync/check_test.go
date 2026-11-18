package sync_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	fakewb "github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestANewKeyIsReadAndCheckedBeforeItIsSaved(t *testing.T) {
	t.Parallel()

	r := newEngineRig(t)
	ctx := t.Context()

	token := fakewb.MakeToken(fakewb.TokenClaims{
		ID: t.Name(), Seller: fakeSeller, Type: fakewb.TokenPersonal, Expires: synckit.Noon().Add(30 * 24 * time.Hour),
		Categories: fakewb.CategoryMarketplace | fakewb.CategoryStatistics,
	})

	checked, secret, err := r.engine.CheckNewKey(ctx, "wildberries", 0, map[string]string{"token": token})
	if err != nil || secret != token || checked.SellerName != "Test seller" || checked.ProfileID != fakeSeller {
		t.Fatalf("CheckNewKey = %+v, secret kept %v, %v", checked, secret == token, err)
	}

	rows := r.kit.Rows(t)
	if len(rows) != 3 || rows[0].Job != "key.check" || rows[0].AccountID != 0 {
		t.Fatalf("the check's journal rows: %+v", rows)
	}

	if _, _, err := r.engine.CheckNewKey(ctx, "nowhere", 0, map[string]string{"token": token}); !errors.Is(err, sync.ErrUnknownMarketplace) {
		t.Fatalf("an unknown marketplace: %v", err)
	}

	var refusal *marketplace.Refusal
	if _, _, err := r.engine.CheckNewKey(ctx, "wildberries", 0, map[string]string{"token": "nonsense"}); !errors.As(err, &refusal) ||
		refusal.Code != "key_unreadable" {
		t.Fatalf("a text that is not a key: %v", err)
	}
}

func TestASavedKeyIsCheckedAgainItsFactsRewrittenAndItsWarehousesRead(t *testing.T) {
	t.Parallel()

	r := newEngineRig(t)
	ctx := t.Context()
	a := r.account(t, func(c *fakewb.TokenClaims) { c.Categories = fakewb.CategoryMarketplace })

	checked, err := r.engine.CheckAccount(ctx, a.ID)
	if err != nil || checked.State != "working" || checked.SellerName != "Test seller" || checked.LastCheckOutcome != "done" ||
		len(checked.MissingCategories) != 1 {
		t.Fatalf("CheckAccount = %+v, %v", checked, err)
	}

	warehouses, _ := r.kit.Store.Read().Warehouses(ctx, a.ID)
	if len(warehouses) != 1 || warehouses[0].Name != "Seller warehouse" {
		t.Fatalf("the warehouses read on a check: %+v", warehouses)
	}

	r.control(t, a.ID, http.MethodPost, "/__control/setup", map[string]any{"token_refused": "expired"}, nil)

	refused, err := r.engine.CheckAccount(ctx, a.ID)

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refused.State != "key_expired" || refused.LastRefusal.Code != "key_expired" ||
		refused.LastCheckOutcome != "key_refused" {
		t.Fatalf("a check Wildberries refuses: %+v, %v", refused, err)
	}

	if err := r.engine.RunKeyChecks(ctx); err != nil {
		t.Fatalf("RunKeyChecks: %v", err)
	}

	if _, err := r.engine.CheckAccount(ctx, 999); err == nil {
		t.Fatal("an account that does not exist was checked")
	}
}
