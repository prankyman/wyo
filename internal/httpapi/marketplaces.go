package httpapi

import (
	"net/http"
	"time"

	"github.com/prankyman/wyo/internal/accounts"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/marketplace"
)

// moment is a pointer to a moment, nil when it is zero: the description leaves such a field out.
func moment(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}

	return &t
}

// words is a list the description always has, empty rather than absent.
func words(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}

// exchangeFlow is how the exchange treats a marketplace's orders: "pack" when the adapter claims the part that reports the
// packing, "confirm" when it confirms the order at the reservation.
func exchangeFlow(d marketplace.Description) gen.MarketplaceExchangeFlow {
	if d.Supports(marketplace.OrdersPack) {
		return gen.Pack
	}

	return gen.Confirm
}

// declaredSteps are the steps of a hand-over the adapter needs, in the order they run.
func declaredSteps(d marketplace.Description) []gen.HandoverStep {
	steps := make([]gen.HandoverStep, 0, len(marketplace.HandoverStepsAll))

	for _, step := range marketplace.HandoverStepsAll {
		if d.NeedsStep(step) {
			steps = append(steps, gen.HandoverStep(step))
		}
	}

	return steps
}

func marketplaceBody(d marketplace.Description) gen.Marketplace {
	body := gen.Marketplace{
		Code: d.Code, Mark: d.Mark, KeyFields: []gen.KeyField{}, Capabilities: []string{}, HandoverSettings: []gen.SettingField{},
		GivesDeadline: d.GivesDeadline, ExchangeFlow: exchangeFlow(d), HandoverSteps: declaredSteps(d),
	}

	for _, f := range d.HandoverSettings {
		body.HandoverSettings = append(body.HandoverSettings, gen.SettingField{Name: f.Name, Choices: append([]string{}, f.Choices...)})
	}

	for _, f := range d.KeyFields {
		body.KeyFields = append(body.KeyFields, gen.KeyField{Name: f.Name, Secret: f.Secret})
	}

	for _, c := range d.Capabilities {
		body.Capabilities = append(body.Capabilities, string(c))
	}

	return body
}

// accountBody is an account as the description shows it: of its key, only the last four characters.
func accountBody(v accounts.View) gen.Account {
	failures := v.FailuresInRow

	body := gen.Account{
		Id: v.ID, Marketplace: v.Marketplace, Name: v.Name, State: gen.AccountState(v.State), Test: v.Test,
		SellerName: text(v.SellerName), KeyLast4: v.KeyLast4, KeyType: text(v.KeyType), KeyExpiresAt: moment(v.KeyExpiresAt),
		MissingCategories: words(v.MissingCategories), ReadOnly: v.ReadOnly, DeadlineHours: v.DeadlineHours,
		LastCheckAt: moment(v.LastCheckAt), LastSuccessAt: moment(v.LastSuccessAt), FailuresInRow: &failures,
		HasOrders: v.HasOrders, CreatedAt: v.CreatedAt,
	}

	if len(v.KeyFields) > 0 {
		shown := make([]gen.ShownKeyField, 0, len(v.KeyFields))
		for _, f := range v.KeyFields {
			shown = append(shown, gen.ShownKeyField{Name: f.Name, Value: f.Value})
		}

		body.KeyFields = &shown
	}

	if v.SellerDelivers > 0 {
		count := v.SellerDelivers
		body.SellerDeliversWarehouses = &count
	}

	if v.LastRefusal.Code != "" {
		body.LastRefusal = &gen.AccountRefusal{Code: v.LastRefusal.Code, At: v.LastRefusal.At}

		if v.LastRefusal.Status != 0 {
			status := v.LastRefusal.Status
			body.LastRefusal.Status = &status
		}
	}

	return body
}

// answerAccount answers an account, or the error of the call that was to give it.
func (a *api) answerAccount(w http.ResponseWriter, r *http.Request, status int, v accounts.View, err error) {
	if err != nil {
		a.fail(w, r, err)

		return
	}

	writeJSON(w, status, accountBody(v))
}

// ListMarketplaces answers GET /api/v1/marketplaces.
func (a *api) ListMarketplaces(w http.ResponseWriter, _ *http.Request) {
	body := gen.MarketplaceList{Marketplaces: []gen.Marketplace{}}

	for _, adapter := range a.deps.Marketplaces.All() {
		body.Marketplaces = append(body.Marketplaces, marketplaceBody(adapter.Describe()))
	}

	writeJSON(w, http.StatusOK, body)
}

// ListAccounts answers GET /api/v1/accounts.
func (a *api) ListAccounts(w http.ResponseWriter, r *http.Request) {
	list, err := a.deps.Accounts.List(r.Context())
	if err != nil {
		a.fail(w, r, err)

		return
	}

	body := gen.AccountList{Accounts: make([]gen.Account, 0, len(list))}
	for _, v := range list {
		body.Accounts = append(body.Accounts, accountBody(v))
	}

	writeJSON(w, http.StatusOK, body)
}

// CreateAccount answers POST /api/v1/accounts.
func (a *api) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var in gen.AccountCreate
	if !decode(w, r, &in) {
		return
	}

	if in.Key == nil {
		BadRequest(w, r, nil)

		return
	}

	v, err := a.deps.Accounts.Create(r.Context(), in.Marketplace, in.Name, in.Key)
	a.answerAccount(w, r, http.StatusCreated, v, err)
}

// GetAccount answers GET /api/v1/accounts/{id}.
func (a *api) GetAccount(w http.ResponseWriter, r *http.Request, id int64) {
	v, err := a.deps.Accounts.Get(r.Context(), id)
	a.answerAccount(w, r, http.StatusOK, v, err)
}

// UpdateAccount answers PATCH /api/v1/accounts/{id}.
func (a *api) UpdateAccount(w http.ResponseWriter, r *http.Request, id int64) {
	var in gen.AccountChange
	if !decode(w, r, &in) {
		return
	}

	v, err := a.deps.Accounts.Update(r.Context(), id, accounts.Change{Name: in.Name, DeadlineHours: in.DeadlineHours})
	a.answerAccount(w, r, http.StatusOK, v, err)
}

// DeleteAccount answers DELETE /api/v1/accounts/{id}.
func (a *api) DeleteAccount(w http.ResponseWriter, r *http.Request, id int64) {
	if err := a.deps.Accounts.Delete(r.Context(), id); err != nil {
		a.fail(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ReplaceAccountKey answers PUT /api/v1/accounts/{id}/key.
func (a *api) ReplaceAccountKey(w http.ResponseWriter, r *http.Request, id int64) {
	var in gen.AccountKey
	if !decode(w, r, &in) {
		return
	}

	if in.Key == nil {
		BadRequest(w, r, nil)

		return
	}

	v, err := a.deps.Accounts.ReplaceKey(r.Context(), id, in.Key)
	a.answerAccount(w, r, http.StatusOK, v, err)
}

// CheckAccount answers POST /api/v1/accounts/{id}/check. The body (the interface sends {}) carries nothing.
func (a *api) CheckAccount(w http.ResponseWriter, r *http.Request, id int64) {
	v, err := a.deps.Accounts.Check(r.Context(), id)
	a.answerAccount(w, r, http.StatusOK, v, err)
}

// DisableAccount answers POST /api/v1/accounts/{id}/disable.
func (a *api) DisableAccount(w http.ResponseWriter, r *http.Request, id int64) {
	v, err := a.deps.Accounts.Disable(r.Context(), id)
	a.answerAccount(w, r, http.StatusOK, v, err)
}

// EnableAccount answers POST /api/v1/accounts/{id}/enable.
func (a *api) EnableAccount(w http.ResponseWriter, r *http.Request, id int64) {
	v, err := a.deps.Accounts.Enable(r.Context(), id)
	a.answerAccount(w, r, http.StatusOK, v, err)
}
