package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/store/db"
)

// Account is one seller profile on one marketplace. Its key is in the secret store under SecretName; the row keeps only
// what may be shown of it. A zero time means "never" (or, for KeyExpiresAt, "the key names no end").
type Account struct {
	ID                int64
	Marketplace       string
	Name              string
	State             string
	Test              bool
	SellerName        string
	SellerProfileID   string
	SecretName        string
	KeyLast4          string
	KeyFields         []KeyField
	KeyType           string
	KeyExpiresAt      time.Time
	Categories        []string
	MissingCategories []string
	ReadOnly          bool
	DeadlineHours     int
	LastCheckAt       time.Time
	LastCheckOutcome  string
	LastSuccessAt     time.Time
	LastRefusal       AccountRefusal
	FailuresInRow     int
	CreatedAt         time.Time
}

// AccountRefusal is the last refusal of a marketplace for an account: the adapter's code and the answer's status.
type AccountRefusal struct {
	Code   string
	Status int
	At     time.Time
}

// AccountKey is what a passed key check writes over an account: the key's facts and a clean record (no refusal, no
// failures, the check counted as the last success).
type AccountKey struct {
	State             string
	Test              bool
	SellerName        string
	SellerProfileID   string
	KeyLast4          string
	KeyFields         []KeyField
	KeyType           string
	KeyExpiresAt      time.Time
	Categories        []string
	MissingCategories []string
	ReadOnly          bool
	CheckedAt         time.Time
	CheckOutcome      string
}

// KeyField is a field of an account's key that is not secret, with its value: the screens show it in full.
type KeyField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Warehouse is one of the seller's warehouses on a marketplace: the marketplace's own number, its name, and whether the
// seller delivers its orders to the buyer itself (they are never read), the kind of label the marketplace makes for it ("" when
// it names none) and whether the marketplace itself assembles its orders.
type Warehouse struct {
	Number         string
	Name           string
	SellerDelivers bool
	LabelType      string
	AutoAssembly   bool
}

// keyFieldsText keeps the shown fields of a key as JSON; no field is no text.
func keyFieldsText(list []KeyField) string {
	if len(list) == 0 {
		return ""
	}

	raw, _ := json.Marshal(list) // a list of texts always encodes

	return string(raw)
}

// keyFieldsOf reads them back; a text that cannot be read is no field.
func keyFieldsOf(text string) []KeyField {
	var list []KeyField
	if text == "" || json.Unmarshal([]byte(text), &list) != nil {
		return nil
	}

	return list
}

func optionalSeconds(t time.Time) sql.NullInt64 {
	if t.IsZero() {
		return sql.NullInt64{}
	}

	return sql.NullInt64{Int64: seconds(t), Valid: true}
}

func optionalMoment(v sql.NullInt64) time.Time {
	if !v.Valid {
		return time.Time{}
	}

	return moment(v.Int64)
}

// words keeps a list of codes or barcodes, none of which holds a space, as one text.
func words(list []string) string { return strings.Join(list, " ") }

// wordsOf reads such a text back; an empty text is no list at all.
func wordsOf(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	return strings.Fields(text)
}

func account(row db.MarketplaceAccount) Account {
	return Account{
		ID: row.ID, Marketplace: row.Marketplace, Name: row.Name, State: row.State, Test: row.Test == 1, SellerName: row.SellerName,
		SellerProfileID: row.SellerProfileID, SecretName: row.SecretName, KeyLast4: row.KeyLast4, KeyFields: keyFieldsOf(row.KeyFields),
		KeyType:      row.KeyType,
		KeyExpiresAt: optionalMoment(row.KeyExpiresAt), Categories: wordsOf(row.Categories),
		MissingCategories: wordsOf(row.MissingCategories), ReadOnly: row.ReadOnly == 1, DeadlineHours: int(row.DeadlineHours),
		LastCheckAt: optionalMoment(row.LastCheckAt), LastCheckOutcome: row.LastCheckOutcome,
		LastSuccessAt: optionalMoment(row.LastSuccessAt),
		LastRefusal: AccountRefusal{
			Code: row.LastRefusalCode, Status: int(row.LastRefusalStatus), At: optionalMoment(row.LastRefusalAt),
		},
		FailuresInRow: int(row.FailuresInRow), CreatedAt: moment(row.CreatedAt),
	}
}

// changedOne turns the count of changed rows into ErrNotFound when there was nothing to change.
func changedOne(changed int64, err error) error {
	if err != nil {
		return kind(err)
	}

	if changed == 0 {
		return ErrNotFound
	}

	return nil
}

// InsertAccount adds an account; ErrExists when its secret's name is taken.
func (q *Queries) InsertAccount(ctx context.Context, a Account) (Account, error) {
	row, err := q.q.InsertAccount(ctx, db.InsertAccountParams{
		Marketplace: a.Marketplace, Name: a.Name, State: a.State, Test: boolInt(a.Test), SellerName: a.SellerName,
		SellerProfileID: a.SellerProfileID, SecretName: a.SecretName, KeyLast4: a.KeyLast4, KeyType: a.KeyType,
		KeyExpiresAt: optionalSeconds(a.KeyExpiresAt), Categories: words(a.Categories), MissingCategories: words(a.MissingCategories),
		ReadOnly: boolInt(a.ReadOnly), DeadlineHours: int64(a.DeadlineHours), LastCheckAt: optionalSeconds(a.LastCheckAt),
		LastCheckOutcome: a.LastCheckOutcome, LastSuccessAt: optionalSeconds(a.LastSuccessAt), CreatedAt: seconds(a.CreatedAt),
		KeyFields: keyFieldsText(a.KeyFields),
	})
	if err != nil {
		return Account{}, kind(err)
	}

	return account(row), nil
}

// Account finds an account; ErrNotFound when there is none.
func (q *Queries) Account(ctx context.Context, id int64) (Account, error) {
	row, err := q.q.GetAccount(ctx, id)
	if err != nil {
		return Account{}, kind(err)
	}

	return account(row), nil
}

// Accounts lists every account in the order they were added.
func (q *Queries) Accounts(ctx context.Context) ([]Account, error) {
	rows, err := q.q.ListAccounts(ctx)
	if err != nil {
		return nil, kind(err)
	}

	list := make([]Account, 0, len(rows))
	for _, row := range rows {
		list = append(list, account(row))
	}

	return list, nil
}

// UpdateAccountKey writes the facts of a key that passed its check; ErrNotFound when there is no such account.
func (q *Queries) UpdateAccountKey(ctx context.Context, id int64, k AccountKey) error {
	return changedOne(q.q.UpdateAccountKey(ctx, db.UpdateAccountKeyParams{
		State: k.State, Test: boolInt(k.Test), SellerName: k.SellerName, SellerProfileID: k.SellerProfileID, KeyLast4: k.KeyLast4,
		KeyFields: keyFieldsText(k.KeyFields), KeyType: k.KeyType, KeyExpiresAt: optionalSeconds(k.KeyExpiresAt), Categories: words(k.Categories),
		MissingCategories: words(k.MissingCategories), ReadOnly: boolInt(k.ReadOnly), LastCheckAt: optionalSeconds(k.CheckedAt),
		LastCheckOutcome: k.CheckOutcome, LastSuccessAt: optionalSeconds(k.CheckedAt), ID: id,
	}))
}

// UpdateAccountSettings changes the account's own name and the hours to hand an order over.
func (q *Queries) UpdateAccountSettings(ctx context.Context, id int64, name string, deadlineHours int) error {
	return changedOne(q.q.UpdateAccountSettings(ctx, db.UpdateAccountSettingsParams{Name: name, DeadlineHours: int64(deadlineHours), ID: id}))
}

// SetAccountState changes only the state.
func (q *Queries) SetAccountState(ctx context.Context, id int64, state string) error {
	return changedOne(q.q.UpdateAccountState(ctx, db.UpdateAccountStateParams{State: state, ID: id}))
}

// RecordAccountCheck notes when the key was last checked and the outcome, whatever it was.
func (q *Queries) RecordAccountCheck(ctx context.Context, id int64, at time.Time, outcome string) error {
	return changedOne(q.q.RecordAccountCheck(ctx, db.RecordAccountCheckParams{
		LastCheckAt: optionalSeconds(at), LastCheckOutcome: outcome, ID: id,
	}))
}

// RecordAccountSuccess notes a successful reading: the failures in a row start again from 0, and the state becomes the
// one given unless the account is switched off or its key is stopped.
func (q *Queries) RecordAccountSuccess(ctx context.Context, id int64, state string, at time.Time) error {
	return changedOne(q.q.RecordAccountSuccess(ctx, db.RecordAccountSuccessParams{State: state, LastSuccessAt: optionalSeconds(at), ID: id}))
}

// RecordAccountFailure counts one more failure in a row and returns the count.
func (q *Queries) RecordAccountFailure(ctx context.Context, id int64) (int, error) {
	n, err := q.q.RecordAccountFailure(ctx, id)

	return int(n), kind(err)
}

// RecordAccountRefusal notes the marketplace's refusal; an empty state leaves the state as it was, and an account
// switched off stays switched off.
func (q *Queries) RecordAccountRefusal(ctx context.Context, id int64, state, code string, status int, at time.Time) error {
	return changedOne(q.q.RecordAccountRefusal(ctx, db.RecordAccountRefusalParams{
		State: state, LastRefusalCode: code, LastRefusalStatus: int64(status), LastRefusalAt: optionalSeconds(at), ID: id,
	}))
}

// MarkAccountWithoutConnection makes an account that is being read «no connection»; an account in any other state keeps
// it. It says whether the state changed.
func (q *Queries) MarkAccountWithoutConnection(ctx context.Context, id int64) (bool, error) {
	changed, err := q.q.MarkAccountWithoutConnection(ctx, id)

	return changed == 1, kind(err)
}

// DeleteAccount removes an account with its warehouses and marks; the database refuses it while an order of it exists.
func (q *Queries) DeleteAccount(ctx context.Context, id int64) error {
	return changedOne(q.q.DeleteAccount(ctx, id))
}

// CountOrdersOfAccount says how many orders came from an account.
func (q *Queries) CountOrdersOfAccount(ctx context.Context, id int64) (int64, error) {
	n, err := q.q.CountOrdersOfAccount(ctx, id)

	return n, kind(err)
}

// ReplaceWarehouses keeps exactly the given warehouses for an account.
func (q *Queries) ReplaceWarehouses(ctx context.Context, accountID int64, list []Warehouse, at time.Time) error {
	if err := q.q.DeleteWarehousesOfAccount(ctx, accountID); err != nil {
		return kind(err)
	}

	for _, w := range list {
		if err := q.q.InsertWarehouse(ctx, db.InsertWarehouseParams{
			AccountID: accountID, Number: w.Number, Name: w.Name, SellerDelivers: boolInt(w.SellerDelivers),
			LabelType: w.LabelType, AutoAssembly: boolInt(w.AutoAssembly), UpdatedAt: seconds(at),
		}); err != nil {
			return kind(err)
		}
	}

	return nil
}

// Warehouses lists an account's warehouses by name.
func (q *Queries) Warehouses(ctx context.Context, accountID int64) ([]Warehouse, error) {
	rows, err := q.q.ListWarehouses(ctx, accountID)
	if err != nil {
		return nil, kind(err)
	}

	list := make([]Warehouse, 0, len(rows))
	for _, row := range rows {
		list = append(list, Warehouse{
			Number: row.Number, Name: row.Name, SellerDelivers: row.SellerDelivers == 1, LabelType: row.LabelType, AutoAssembly: row.AutoAssembly == 1,
		})
	}

	return list, nil
}

// Mark is how far a feed of an account has been read; ErrNotFound when it was never read.
func (q *Queries) Mark(ctx context.Context, accountID int64, feed string) (string, error) {
	mark, err := q.q.GetMark(ctx, db.GetMarkParams{AccountID: accountID, Feed: feed})

	return mark, kind(err)
}

// PutMark moves a feed's mark.
func (q *Queries) PutMark(ctx context.Context, accountID int64, feed, mark string, at time.Time) error {
	return kind(q.q.PutMark(ctx, db.PutMarkParams{AccountID: accountID, Feed: feed, Mark: mark, ChangedAt: seconds(at)}))
}
