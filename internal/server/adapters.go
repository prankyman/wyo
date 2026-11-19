package server

import (
	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
	"github.com/prankyman/wyo/internal/marketplace/yandex"
)

// adapters is THE list of the marketplace adapters the wyo server holds, each pointed at testBase — the fake
// marketplaces' address of a test build, or "" for the real marketplaces — and given the wyo server's clock. A new
// marketplace is one line here. This file is the one place outside the adapters that names a marketplace
// (internal/archtest, rule R6).
func adapters(testBase string, clk clock.Clock) (*marketplace.Registry, error) {
	return registryOf(
		wildberries.New(wildberries.Options{TestBase: testBase}),
		ozon.New(ozon.Options{TestBase: testBase, Clock: clk}),
		yandex.New(yandex.Options{TestBase: testBase, Clock: clk}),
	)
}

// hostsOf lists the hosts of every registered adapter: the outbound gate's one list of the marketplaces' addresses is
// built from it (principle IX).
func hostsOf(registry *marketplace.Registry) []string {
	var hosts []string
	for _, adapter := range registry.All() {
		hosts = append(hosts, adapter.Hosts()...)
	}

	return hosts
}

// registryOf registers every adapter of the list; two adapters of one code stop the wyo server from starting.
func registryOf(list ...marketplace.Adapter) (*marketplace.Registry, error) {
	registry := marketplace.NewRegistry()

	for _, adapter := range list {
		if err := registry.Register(adapter); err != nil {
			return nil, err
		}
	}

	return registry, nil
}
