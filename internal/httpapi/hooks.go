package httpapi

import (
	"net/http"
	"net/netip"

	"github.com/prankyman/wyo/internal/refusal"
)

// ReceiveMarketplaceNotification answers POST /hooks/marketplace/{text}: a marketplace's notification for one account,
// stored and answered at once in the marketplace's own form (internal/notify). Anything that is not one of the wyo server's
// accounts' notifications is answered as an address that does not exist.
func (a *api) ReceiveMarketplaceNotification(w http.ResponseWriter, r *http.Request, text string) {
	a.receiveMarketplace(w, r, text)
}

// ReceiveMarketplaceNotificationAtItsMethod answers POST /hooks/marketplace/{text}/notification alike: whether a marketplace
// adds the name of its method to the address the seller enters is not sourced.
func (a *api) ReceiveMarketplaceNotificationAtItsMethod(w http.ResponseWriter, r *http.Request, text string) {
	a.receiveMarketplace(w, r, text)
}

func (a *api) receiveMarketplace(w http.ResponseWriter, r *http.Request, text string) {
	from, err := netip.ParseAddr(a.clientAddress(r))
	if err != nil || a.deps.Hooks == nil {
		refusal.Code(w, http.StatusNotFound, "not_found")

		return
	}

	answer := a.deps.Hooks.Receive(r.Context(), text, from, r.Body)
	if answer.Status == http.StatusNotFound {
		refusal.Code(w, http.StatusNotFound, "not_found")

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(answer.Status)
	_, _ = w.Write(answer.Body)
}
