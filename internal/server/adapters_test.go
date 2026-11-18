package server

import (
	"slices"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
)

// hostedAdapter is the marketplace in memory under a code of its own that calls the hosts it is given.
type hostedAdapter struct {
	*memadapter.Adapter

	code  string
	hosts []string
}

func (a hostedAdapter) Describe() marketplace.Description {
	d := a.Adapter.Describe()
	d.Code = a.code

	return d
}

func (a hostedAdapter) Hosts() []string { return a.hosts }

func TestTheGateIsGivenEveryHostOfEveryAdapterAWildcardEntryAmongThemAsItIs(t *testing.T) {
	t.Parallel()

	registry, err := registryOf(
		hostedAdapter{Adapter: memadapter.New(1), code: "alpha", hosts: []string{"api.alpha.test", "*.files.alpha.test"}},
		hostedAdapter{Adapter: memadapter.New(1), code: "beta", hosts: []string{"api.beta.test"}},
	)
	if err != nil {
		t.Fatalf("registryOf: %v", err)
	}

	got := hostsOf(registry)
	slices.Sort(got)

	if want := []string{"*.files.alpha.test", "api.alpha.test", "api.beta.test"}; !slices.Equal(got, want) {
		t.Fatalf("hostsOf = %v, want %v", got, want)
	}
}

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
