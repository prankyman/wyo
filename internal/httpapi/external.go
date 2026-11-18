package httpapi

import (
	"errors"
	"io"
	"net/http"
	"sort"

	"github.com/prankyman/wyo/internal/exchange"
	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/refusal"
	"github.com/prankyman/wyo/internal/store"
)

// maxNotification is the largest notification the wyo server accepts.
const maxNotification = 256 << 10

// externalBody is «Внешний API» as the interface shows it: never the key, only its last four characters.
func externalBody(c store.Connection) gen.ExternalApi {
	markets := make([]gen.FcMarketplace, 0, len(c.Marketplaces))
	for _, m := range c.Marketplaces {
		markets = append(markets, gen.FcMarketplace{Code: m.Code, Name: m.Name})
	}

	return gen.ExternalApi{
		State: gen.ExternalApiState(c.State), Address: text(c.Address), KeyLast4: text(c.KeyLast4), SellerName: text(c.SellerName),
		Marketplaces: &markets, CheckedAt: moment(c.CheckedAt), EventsMode: gen.EventsMode(c.EventsMode),
		LastEventAt: moment(c.LastEventAt),
	}
}

// emptyExternal is «Внешний API» not filled.
var emptyExternal = gen.ExternalApi{State: "empty", EventsMode: "poll"}

// GetExternalApi answers GET /api/v1/external-api.
func (a *api) GetExternalApi(w http.ResponseWriter, r *http.Request) {
	c, err := a.deps.Exchange.Connection(r.Context())
	if errors.Is(err, exchange.ErrNotFilled) {
		writeJSON(w, http.StatusOK, emptyExternal)

		return
	}

	if err != nil {
		a.fail(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, externalBody(c))
}

// PutExternalApi answers PUT /api/v1/external-api: «Проверить и сохранить».
func (a *api) PutExternalApi(w http.ResponseWriter, r *http.Request) {
	var in gen.ExternalApiConnect
	if !decode(w, r, &in) {
		return
	}

	c, err := a.deps.Exchange.Connect(r.Context(), in.Address, in.Key)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, externalBody(c))
}

// DeleteExternalApi answers DELETE /api/v1/external-api: «Отключить».
func (a *api) DeleteExternalApi(w http.ResponseWriter, r *http.Request) {
	if err := a.deps.Exchange.Disconnect(r.Context()); err != nil {
		a.fail(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CheckExternalApi answers POST /api/v1/external-api/check: «Проверить снова».
func (a *api) CheckExternalApi(w http.ResponseWriter, r *http.Request) {
	c, err := a.deps.Exchange.Check(r.Context())
	if err != nil {
		a.fail(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, externalBody(c))
}

// GetExternalApiWarehouses answers GET /api/v1/external-api/warehouses.
func (a *api) GetExternalApiWarehouses(w http.ResponseWriter, r *http.Request) {
	a.writeWarehouses(w, r)
}

func (a *api) writeWarehouses(w http.ResponseWriter, r *http.Request) {
	q := a.deps.Store.Read()

	accounts, err := q.Accounts(r.Context())
	if err != nil {
		a.fail(w, r, err)

		return
	}

	matches, err := q.WarehouseMatches(r.Context())
	if err != nil {
		a.fail(w, r, err)

		return
	}

	matched := map[[2]any]string{}
	for _, m := range matches {
		matched[[2]any{m.AccountID, m.WarehouseNumber}] = m.FcWarehouse
	}

	body := gen.WarehouseMatches{FcWarehouses: []gen.FcWarehouse{}, Rows: []gen.WarehouseMatch{}}

	for _, wh := range a.deps.Exchange.FcWarehouses(r.Context()) {
		body.FcWarehouses = append(body.FcWarehouses, gen.FcWarehouse{Code: wh.Code, Name: wh.Name, City: wh.City})
	}

	for _, acc := range accounts {
		list, err := q.Warehouses(r.Context(), acc.ID)
		if err != nil {
			a.fail(w, r, err)

			return
		}

		for _, wh := range list {
			body.Rows = append(body.Rows, gen.WarehouseMatch{
				AccountId: acc.ID, AccountName: acc.Name, Marketplace: acc.Marketplace, WarehouseNumber: wh.Number,
				WarehouseName: wh.Name, FcWarehouse: text(matched[[2]any{acc.ID, wh.Number}]),
			})
		}
	}

	writeJSON(w, http.StatusOK, body)
}

// PutExternalApiWarehouses answers PUT /api/v1/external-api/warehouses.
func (a *api) PutExternalApiWarehouses(w http.ResponseWriter, r *http.Request) {
	var in gen.WarehouseMatchesChange
	if !decode(w, r, &in) {
		return
	}

	err := a.deps.Store.Do(r.Context(), func(q *store.Queries) error {
		for _, row := range in.Rows {
			if _, err := q.Account(r.Context(), row.AccountId); err != nil {
				return err
			}

			if err := q.PutWarehouseMatch(r.Context(), store.WarehouseMatch{
				AccountID: row.AccountId, WarehouseNumber: row.WarehouseNumber, FcWarehouse: valueOf(row.FcWarehouse),
			}); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		a.fail(w, r, err)

		return
	}

	a.writeWarehouses(w, r)
}

// GetExternalApiMarketplaces answers GET /api/v1/external-api/marketplaces.
func (a *api) GetExternalApiMarketplaces(w http.ResponseWriter, r *http.Request) {
	a.writeMarketplaces(w, r)
}

func (a *api) writeMarketplaces(w http.ResponseWriter, r *http.Request) {
	q := a.deps.Store.Read()

	accounts, err := q.Accounts(r.Context())
	if err != nil {
		a.fail(w, r, err)

		return
	}

	matches, err := q.MarketplaceMatches(r.Context())
	if err != nil {
		a.fail(w, r, err)

		return
	}

	body := gen.MarketplaceMatches{FcMarketplaces: []gen.FcMarketplace{}, Rows: []gen.MarketplaceMatch{}}
	codes := map[string]bool{}

	if c, err := a.deps.Exchange.Connection(r.Context()); err == nil {
		for _, m := range c.Marketplaces {
			body.FcMarketplaces = append(body.FcMarketplaces, gen.FcMarketplace{Code: m.Code, Name: m.Name})
			codes[m.Code] = true
		}
	}

	for _, acc := range accounts {
		handover, err := q.AccountHandover(r.Context(), acc.ID)
		if err != nil {
			a.fail(w, r, err)

			return
		}

		row := gen.MarketplaceMatch{
			AccountId: acc.ID, AccountName: acc.Name, Marketplace: acc.Marketplace, ReadOnly: acc.ReadOnly,
			FcMarketplace: text(matches[acc.ID]), HandoverSettings: handover.Settings,
		}

		if row.FcMarketplace == nil && codes[acc.Marketplace] {
			row.Proposed = text(acc.Marketplace)
		}

		body.Rows = append(body.Rows, row)
	}

	sort.SliceStable(body.Rows, func(i, j int) bool { return body.Rows[i].AccountId < body.Rows[j].AccountId })
	writeJSON(w, http.StatusOK, body)
}

// PutExternalApiMarketplaces answers PUT /api/v1/external-api/marketplaces.
func (a *api) PutExternalApiMarketplaces(w http.ResponseWriter, r *http.Request) {
	var in gen.MarketplaceMatchesChange
	if !decode(w, r, &in) {
		return
	}

	err := a.deps.Store.Do(r.Context(), func(q *store.Queries) error {
		for _, row := range in.Rows {
			if _, err := q.Account(r.Context(), row.AccountId); err != nil {
				return err
			}

			if row.FcMarketplace != nil {
				if err := q.PutMarketplaceMatch(r.Context(), row.AccountId, *row.FcMarketplace); err != nil {
					return err
				}
			}

			if row.HandoverSettings != nil {
				if err := q.SetAccountHandoverSettings(r.Context(), row.AccountId, *row.HandoverSettings); err != nil {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		a.fail(w, r, err)

		return
	}

	a.writeMarketplaces(w, r)
}

// eventsBody is how events are received; the signing secret is never sent back.
func (a *api) eventsBody(r *http.Request, c store.Connection) gen.EventsSetting {
	body := gen.EventsSetting{
		Mode: gen.EventsMode(c.EventsMode), LastNotificationAt: moment(c.LastNotificationAt), LastPingAt: moment(c.LastPingAt),
		LastEventAt: moment(c.LastEventAt),
	}

	if c.EventsMode == "notify" && c.HookText != "" {
		saved := c.HookSecretName != ""
		body.HookUrl, body.SecretSaved = text(a.origin(r)+exchange.HookPath(c.HookText)), &saved
	}

	return body
}

// origin is the wyo server's own address as the browser reached it: what the fulfilment centre's system must call.
func (a *api) origin(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil && !a.deps.SecureCookie {
		scheme = "http"
	}

	return scheme + "://" + r.Host
}

// GetExternalApiEvents answers GET /api/v1/external-api/events.
func (a *api) GetExternalApiEvents(w http.ResponseWriter, r *http.Request) {
	c, err := a.deps.Exchange.Connection(r.Context())
	if errors.Is(err, exchange.ErrNotFilled) {
		writeJSON(w, http.StatusOK, gen.EventsSetting{Mode: "poll"})

		return
	}

	if err != nil {
		a.fail(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, a.eventsBody(r, c))
}

// PutExternalApiEvents answers PUT /api/v1/external-api/events.
func (a *api) PutExternalApiEvents(w http.ResponseWriter, r *http.Request) {
	var in gen.EventsSettingChange
	if !decode(w, r, &in) {
		return
	}

	if !in.Mode.Valid() {
		BadRequest(w, r, nil)

		return
	}

	c, err := a.deps.Exchange.SetEventsMode(r.Context(), string(in.Mode), valueOf(in.Secret))
	if err != nil {
		a.fail(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, a.eventsBody(r, c))
}

// ReceiveFulfilmentNotification answers POST /hooks/fulfilment/{text}: a signed notification, stored and answered at
// once. The body is read exactly as sent, because the signature covers its bytes.
func (a *api) ReceiveFulfilmentNotification(w http.ResponseWriter, r *http.Request, hookText string) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxNotification))
	if err != nil {
		refusal.Code(w, http.StatusRequestEntityTooLarge, "too_large")

		return
	}

	err = a.deps.Exchange.Receive(r.Context(), hookText, fulfilment.Notification{
		ID: r.Header.Get("webhook-id"), Timestamp: r.Header.Get("webhook-timestamp"), Signature: r.Header.Get("webhook-signature"),
		Body: body,
	})

	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, exchange.ErrUnknownHook):
		refusal.Code(w, http.StatusNotFound, "not_found")
	case errors.Is(err, exchange.ErrBadNotification):
		refusal.Code(w, http.StatusUnauthorized, "bad_signature")
	default:
		a.fail(w, r, err)
	}
}

// GetExternalApiOverview answers GET /api/v1/overview/external-api.
func (a *api) GetExternalApiOverview(w http.ResponseWriter, r *http.Request) {
	o, err := a.deps.Exchange.Overview(r.Context())
	if err != nil {
		a.fail(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, gen.ExternalApiOverview{
		State: gen.ExternalApiState(o.State), LastEventAt: moment(o.LastEventAt), Attention: int(o.Attention),
		HandoversWaiting: int(o.HandoversWaiting),
	})
}
