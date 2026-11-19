package sync_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// ownHostAdapter is an adapter whose API routes name no host and whose list of hosts names only its own.
type ownHostAdapter struct{ adapter }

func (ownHostAdapter) Hosts() []string { return []string{"127.0.0.1"} }

func TestARequestForAHostItsAdapterDoesNotListIsRefusedBeforeItLeavesAndItsKeyStaysHome(t *testing.T) {
	t.Parallel()

	side := &script{}
	k := synckit.New(t, side)
	target := savedTarget(t, k)
	target.Adapter = ownHostAdapter{}

	// The first path segment names a host of another adapter; the adapter's host-less route would match it.
	_, err := send(t, k, target, http.MethodGet, "/cdn.other-market.test/api/orders/new", "")
	if !errors.Is(err, sync.ErrNotDeclared) || side.calls() != 0 || len(k.Rows(t)) != 0 {
		t.Fatalf("err %v, %d calls, %d rows; want refused before anything left", err, side.calls(), len(k.Rows(t)))
	}

	// The adapter's own host still works, and carries the key.
	res, err := send(t, k, target, http.MethodGet, "/api/orders/new", "")
	if err != nil || res.StatusCode != http.StatusOK || side.calls() != 1 || side.auth[0] != secretKey {
		t.Fatalf("own host: %v, %v, %d calls, auth %v", res, err, side.calls(), side.auth)
	}
}
