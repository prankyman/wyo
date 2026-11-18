package server

import (
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
)

// adapters is THE list of the marketplace adapters the wyo server holds, each pointed at testBase — the fake
// marketplace's address of a test build, or "" for the real marketplaces. A new marketplace is one line here. This file
// is the one place outside the adapters that names a marketplace (internal/archtest, rule R6).
func adapters(testBase string) (*marketplace.Registry, error) {
	return registryOf(
		wildberries.New(wildberries.Options{TestBase: testBase}),
	)
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
