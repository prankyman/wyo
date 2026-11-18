package outbound_test

import (
	"errors"
	"net/url"
	"testing"

	"github.com/prankyman/wyo/internal/outbound"
)

func TestTheFulfilmentCentresAddressIsListedOnlyWhileTheSettingIsFilled(t *testing.T) {
	t.Parallel()

	gate := newGate(t)
	address, _ := url.Parse("https://fc.example.ru/api/v1/account")
	other, _ := url.Parse("https://other.example.ru/api/v1/account")

	if err := gate.Check(outbound.PurposeFulfilment, address); !errors.Is(err, outbound.ErrRefused) {
		t.Fatalf("with «Внешний API» empty: %v, want refused", err)
	}

	if err := gate.SetFulfilment("https://FC.example.ru:443"); err != nil {
		t.Fatalf("SetFulfilment: %v", err)
	}

	if err := gate.Check(outbound.PurposeFulfilment, address); err != nil {
		t.Fatalf("with the setting filled: %v", err)
	}

	if err := gate.Check(outbound.PurposeFulfilment, other); !errors.Is(err, outbound.ErrRefused) {
		t.Fatalf("another host: %v, want refused", err)
	}

	if err := gate.Check(outbound.PurposeMarketplace, address); !errors.Is(err, outbound.ErrRefused) {
		t.Fatalf("the fulfilment centre is not a marketplace: %v", err)
	}

	if err := gate.SetFulfilment(""); err != nil {
		t.Fatalf("SetFulfilment(empty): %v", err)
	}

	if err := gate.Check(outbound.PurposeFulfilment, address); !errors.Is(err, outbound.ErrRefused) {
		t.Fatalf("after the setting was emptied: %v, want refused", err)
	}

	if err := gate.SetFulfilment("not an address"); err == nil {
		t.Fatal("an address without a scheme and a host must be refused")
	}

	if got := gate.UserAgent(outbound.PurposeFulfilment); got != "wyo/1.2.3 (+https://github.com/prankyman/wyo)" {
		t.Fatalf("UserAgent = %q", got)
	}
}
