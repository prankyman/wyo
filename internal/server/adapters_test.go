package server

import (
	"testing"

	"github.com/prankyman/wyo/internal/testkit/memadapter"
)

func TestTwoAdaptersOfOneCodeStopTheServerFromStarting(t *testing.T) {
	t.Parallel()

	if _, err := registryOf(memadapter.New(10), memadapter.New(10)); err == nil {
		t.Fatal("two adapters of one code were both registered")
	}

	registry, err := registryOf(memadapter.New(10))
	if err != nil {
		t.Fatalf("one adapter: %v", err)
	}

	if _, ok := registry.Get(memadapter.Code); !ok {
		t.Fatalf("the adapter %q is not in the registry", memadapter.Code)
	}
}
