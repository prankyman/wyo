package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/prankyman/wyo/internal/store/db"
)

// Connection is the one row of «Внешний API»: where the fulfilment centre's API is, the key's name in the secret store
// and its last four characters, the state of the last check and what the fulfilment centre said of itself, how events
// are received, and how far they have been read.
type Connection struct {
	Address            string
	SecretName         string
	KeyLast4           string
	State              string
	SellerName         string
	Marketplaces       []FcMarketplace
	CheckedAt          time.Time
	EventsMode         string
	HookText           string
	HookSecretName     string
	EventsMark         int64
	LastEventAt        time.Time
	LastNotificationAt time.Time
	LastPingAt         time.Time
	// LabelSizesReadAt is when the fulfilment centre's label sizes were last read (zero: never).
	LabelSizesReadAt time.Time
}

// FcMarketplace is a marketplace in the fulfilment centre's own list.
type FcMarketplace struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// WarehouseMatch is the fulfilment centre's warehouse that stands for a seller's warehouse on a marketplace.
type WarehouseMatch struct {
	AccountID       int64
	WarehouseNumber string
	FcWarehouse     string
}

// FcEvent is an event of the fulfilment centre as stored: its id, its number, its kind and its body as received.
type FcEvent struct {
	ID         string
	Sequence   int64
	Type       string
	Body       []byte
	ReceivedAt time.Time
}

// AccountHandover is an account's hand-over settings (the adapter's own names) and the shipping point last chosen.
type AccountHandover struct {
	Settings map[string]string
	Point    string
}

func connection(row db.FcConnection) Connection {
	var markets []FcMarketplace
	_ = json.Unmarshal([]byte(row.Marketplaces), &markets) // written by PutConnection only, always a JSON list

	return Connection{
		Address: row.Address, SecretName: row.SecretName, KeyLast4: row.KeyLast4, State: row.State, SellerName: row.SellerName,
		Marketplaces: markets, CheckedAt: moment(row.CheckedAt), EventsMode: row.EventsMode, HookText: row.HookText,
		HookSecretName: row.HookSecretName, EventsMark: row.EventsMark, LastEventAt: optionalMoment(row.LastEventAt),
		LastNotificationAt: optionalMoment(row.LastNotificationAt), LastPingAt: optionalMoment(row.LastPingAt),
		LabelSizesReadAt: optionalMoment(row.LabelSizesReadAt),
	}
}

// jsonText writes a value as JSON text; the values written here are the store's own types, which always encode.
func jsonText(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "null"
	}

	return string(data)
}

// Connection is «Внешний API»; ErrNotFound while it is empty.
func (q *Queries) Connection(ctx context.Context) (Connection, error) {
	row, err := q.q.GetConnection(ctx)
	if err != nil {
		return Connection{}, kind(err)
	}

	return connection(row), nil
}

// PutConnection fills «Внешний API» with a checked address and key; the way events are received and how far they were
// read are kept when the row exists.
func (q *Queries) PutConnection(ctx context.Context, c Connection) error {
	if c.Marketplaces == nil {
		c.Marketplaces = []FcMarketplace{}
	}

	return kind(q.q.PutConnection(ctx, db.PutConnectionParams{
		Address: c.Address, SecretName: c.SecretName, KeyLast4: c.KeyLast4, State: c.State, SellerName: c.SellerName,
		Marketplaces: jsonText(c.Marketplaces), CheckedAt: seconds(c.CheckedAt),
	}))
}

// SetConnectionState writes the state a call or a check found; ErrNotFound while «Внешний API» is empty.
func (q *Queries) SetConnectionState(ctx context.Context, state string, at time.Time) error {
	return changedOne(q.q.SetConnectionState(ctx, db.SetConnectionStateParams{State: state, CheckedAt: seconds(at)}))
}

// SetConnectionFacts writes what a check found: the state, the seller's name and the marketplaces.
func (q *Queries) SetConnectionFacts(ctx context.Context, state, sellerName string, markets []FcMarketplace, at time.Time) error {
	if markets == nil {
		markets = []FcMarketplace{}
	}

	return changedOne(q.q.SetConnectionFacts(ctx, db.SetConnectionFactsParams{
		State: state, SellerName: sellerName, Marketplaces: jsonText(markets), CheckedAt: seconds(at),
	}))
}

// DeleteConnection empties «Внешний API».
func (q *Queries) DeleteConnection(ctx context.Context) error {
	_, err := q.q.DeleteConnection(ctx)

	return kind(err)
}

// SetEventsMode writes how events are received, with the notifications' address text and secret name (empty when asked).
func (q *Queries) SetEventsMode(ctx context.Context, mode, hookText, hookSecretName string) error {
	return changedOne(q.q.SetEventsMode(ctx, db.SetEventsModeParams{EventsMode: mode, HookText: hookText, HookSecretName: hookSecretName}))
}

// SetEventsMark writes the number of the last event read in order and, when given, the moment of the last event.
func (q *Queries) SetEventsMark(ctx context.Context, mark int64, lastEventAt time.Time) error {
	return changedOne(q.q.SetEventsMark(ctx, db.SetEventsMarkParams{EventsMark: mark, LastEventAt: optionalSeconds(lastEventAt)}))
}

// TouchNotification writes when the last notification arrived, and when the last test one did.
func (q *Queries) TouchNotification(ctx context.Context, at time.Time, ping bool) error {
	return changedOne(q.q.TouchNotification(ctx, db.TouchNotificationParams{At: optionalSeconds(at), Ping: boolInt(ping)}))
}

// WarehouseMatches lists every match.
func (q *Queries) WarehouseMatches(ctx context.Context) ([]WarehouseMatch, error) {
	rows, err := q.q.ListWarehouseMatches(ctx)
	if err != nil {
		return nil, kind(err)
	}

	list := make([]WarehouseMatch, 0, len(rows))
	for _, r := range rows {
		list = append(list, WarehouseMatch{AccountID: r.AccountID, WarehouseNumber: r.WarehouseNumber, FcWarehouse: r.FcWarehouse})
	}

	return list, nil
}

// PutWarehouseMatch saves a match; an empty FcWarehouse removes it.
func (q *Queries) PutWarehouseMatch(ctx context.Context, m WarehouseMatch) error {
	if m.FcWarehouse == "" {
		return kind(q.q.DeleteWarehouseMatch(ctx, db.DeleteWarehouseMatchParams{AccountID: m.AccountID, WarehouseNumber: m.WarehouseNumber}))
	}

	return kind(q.q.PutWarehouseMatch(ctx, db.PutWarehouseMatchParams{
		AccountID: m.AccountID, WarehouseNumber: m.WarehouseNumber, FcWarehouse: m.FcWarehouse,
	}))
}

// MarketplaceMatches gives each matched account's code in the fulfilment centre's list.
func (q *Queries) MarketplaceMatches(ctx context.Context) (map[int64]string, error) {
	rows, err := q.q.ListMarketplaceMatches(ctx)
	if err != nil {
		return nil, kind(err)
	}

	out := make(map[int64]string, len(rows))
	for _, r := range rows {
		out[r.AccountID] = r.FcMarketplace
	}

	return out, nil
}

// PutMarketplaceMatch saves an account's code in the fulfilment centre's list; an empty code removes the match.
func (q *Queries) PutMarketplaceMatch(ctx context.Context, accountID int64, code string) error {
	if code == "" {
		return kind(q.q.DeleteMarketplaceMatch(ctx, accountID))
	}

	return kind(q.q.PutMarketplaceMatch(ctx, db.PutMarketplaceMatchParams{AccountID: accountID, FcMarketplace: code}))
}

// AccountHandover reads an account's hand-over settings and its shipping point.
func (q *Queries) AccountHandover(ctx context.Context, accountID int64) (AccountHandover, error) {
	row, err := q.q.GetAccountHandover(ctx, accountID)
	if err != nil {
		return AccountHandover{}, kind(err)
	}

	settings := map[string]string{}
	_ = json.Unmarshal([]byte(row.HandoverSettings), &settings) // written by SetAccountHandoverSettings only

	return AccountHandover{Settings: settings, Point: row.ShippingPoint}, nil
}

// SetAccountHandoverSettings writes an account's hand-over settings.
func (q *Queries) SetAccountHandoverSettings(ctx context.Context, accountID int64, settings map[string]string) error {
	if settings == nil {
		settings = map[string]string{}
	}

	return changedOne(q.q.SetAccountHandoverSettings(ctx, db.SetAccountHandoverSettingsParams{
		HandoverSettings: jsonText(settings), ID: accountID,
	}))
}

// SetAccountShippingPoint remembers the shipping point the owner chose for an account.
func (q *Queries) SetAccountShippingPoint(ctx context.Context, accountID int64, point string) error {
	return changedOne(q.q.SetAccountShippingPoint(ctx, db.SetAccountShippingPointParams{ShippingPoint: point, ID: accountID}))
}

// InsertFcEvent stores an event and says whether it is new; an event already stored (by its id or its number) is left.
func (q *Queries) InsertFcEvent(ctx context.Context, e FcEvent) (bool, error) {
	n, err := q.q.InsertFcEvent(ctx, db.InsertFcEventParams{
		ID: e.ID, Sequence: e.Sequence, Type: e.Type, Body: string(e.Body), ReceivedAt: seconds(e.ReceivedAt),
	})

	return n == 1, kind(err)
}

// UnappliedFcEvents lists up to limit events not applied yet whose number is at most upTo, in the order of their numbers.
func (q *Queries) UnappliedFcEvents(ctx context.Context, upTo int64, limit int) ([]FcEvent, error) {
	rows, err := q.q.ListUnappliedFcEvents(ctx, db.ListUnappliedFcEventsParams{Sequence: upTo, Limit: int64(limit)})
	if err != nil {
		return nil, kind(err)
	}

	list := make([]FcEvent, 0, len(rows))
	for _, r := range rows {
		list = append(list, FcEvent{ID: r.ID, Sequence: r.Sequence, Type: r.Type, Body: []byte(r.Body), ReceivedAt: moment(r.ReceivedAt)})
	}

	return list, nil
}

// MarkFcEventApplied records that an event was applied.
func (q *Queries) MarkFcEventApplied(ctx context.Context, id string, at time.Time) error {
	return kind(q.q.MarkFcEventApplied(ctx, db.MarkFcEventAppliedParams{AppliedAt: optionalSeconds(at), ID: id}))
}

// DeleteFcEventsBefore deletes the applied events received before a moment and says how many went.
func (q *Queries) DeleteFcEventsBefore(ctx context.Context, before time.Time) (int64, error) {
	n, err := q.q.DeleteFcEventsBefore(ctx, seconds(before))

	return n, kind(err)
}
