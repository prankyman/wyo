package accounts_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/store"
)

// account adds an account row as it would be after its readings: in a state, with a key that ends at expires.
func (r *rig) account(t *testing.T, name string, state marketplace.AccountState, expires time.Time) int64 {
	t.Helper()

	var id int64

	if err := r.kit.Store.Do(t.Context(), func(q *store.Queries) error {
		row, err := q.InsertAccount(t.Context(), store.Account{
			Marketplace: "memory", Name: name, State: string(state), SecretName: name + ".key", KeyLast4: "1234",
			KeyExpiresAt: expires, DeadlineHours: 48, CreatedAt: r.kit.Clock.Now(),
		})
		id = row.ID

		return err
	}); err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	return id
}

func TestTheBellTellsOfKeysThatEndAndAccountsThatCannotBeRead(t *testing.T) {
	t.Parallel()

	day := 24 * time.Hour

	tests := []struct {
		name    string
		state   marketplace.AccountState
		expires time.Duration // from now; 0 is a key that names no end
		want    []notices.Notice
	}{
		{name: "a working key far from its end", state: marketplace.AccountWorking, expires: 15 * day},
		{name: "a key that names no end", state: marketplace.AccountWorking},
		{
			name: "fourteen days before the end", state: marketplace.AccountKeyExpiring, expires: 14 * day,
			want: []notices.Notice{{ID: "accounts:%d:key_expiring:14", Code: "account_key_expiring", Tone: notices.ToneWarn, Params: map[string]any{"days": 14}}},
		},
		{
			name: "four days and an hour before the end", state: marketplace.AccountKeyExpiring, expires: 4*day + time.Hour,
			want: []notices.Notice{{ID: "accounts:%d:key_expiring:14", Code: "account_key_expiring", Tone: notices.ToneWarn, Params: map[string]any{"days": 5}}},
		},
		{
			name: "three days before the end: told again", state: marketplace.AccountKeyExpiring, expires: 3 * day,
			want: []notices.Notice{{ID: "accounts:%d:key_expiring:3", Code: "account_key_expiring", Tone: notices.ToneWarn, Params: map[string]any{"days": 3}}},
		},
		{
			name: "an hour before the end", state: marketplace.AccountWorking, expires: time.Hour,
			want: []notices.Notice{{ID: "accounts:%d:key_expiring:3", Code: "account_key_expiring", Tone: notices.ToneWarn, Params: map[string]any{"days": 1}}},
		},
		{
			name: "ended, before a reading noticed it", state: marketplace.AccountWorking, expires: -time.Minute,
			want: []notices.Notice{{ID: "accounts:%d:key_expired", Code: "account_key_expired", Tone: notices.ToneStop}},
		},
		{
			name: "ended", state: marketplace.AccountKeyExpired, expires: -day,
			want: []notices.Notice{{ID: "accounts:%d:key_expired", Code: "account_key_expired", Tone: notices.ToneStop}},
		},
		{
			name: "not accepted", state: marketplace.AccountKeyRefused, expires: 100 * day,
			want: []notices.Notice{{ID: "accounts:%d:key_refused", Code: "account_key_refused", Tone: notices.ToneStop}},
		},
		{
			name: "without connection", state: marketplace.AccountNoConnection, expires: 100 * day,
			want: []notices.Notice{{ID: "accounts:%d:no_connection", Code: "account_no_connection", Tone: notices.ToneWarn}},
		},
		{
			name: "without connection and ending soon: both", state: marketplace.AccountNoConnection, expires: 2 * day,
			want: []notices.Notice{
				{ID: "accounts:%d:no_connection", Code: "account_no_connection", Tone: notices.ToneWarn},
				{ID: "accounts:%d:key_expiring:3", Code: "account_key_expiring", Tone: notices.ToneWarn, Params: map[string]any{"days": 2}},
			},
		},
		{name: "switched off, however its key stands", state: marketplace.AccountDisabled, expires: -day},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)

			var expires time.Time
			if tc.expires != 0 {
				expires = r.kit.Clock.Now().Add(tc.expires)
			}

			id := r.account(t, "Склад «Север»", tc.state, expires)

			got, err := r.service.Notices(t.Context())
			if err != nil {
				t.Fatalf("Notices: %v", err)
			}

			want := []notices.Notice{}

			for _, n := range tc.want {
				n.ID = fmt.Sprintf(n.ID, id)
				n.Source = notices.SourceAccounts

				params := map[string]any{"account": "Склад «Север»"}
				for k, v := range n.Params {
					params[k] = v
				}

				n.Params = params
				want = append(want, n)
			}

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got  %+v\nwant %+v", got, want)
			}
		})
	}
}

func TestTheBellTellsOfNotificationsThatStoppedArrivingUntilTheNextOneArrives(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	ctx := t.Context()
	now := r.kit.Clock.Now()
	id := r.account(t, "Маркет", marketplace.AccountWorking, time.Time{})

	lines := func() []notices.Notice {
		list, err := r.service.Notices(ctx)
		if err != nil {
			t.Fatalf("Notices: %v", err)
		}

		return list
	}

	if err := r.kit.Store.Do(ctx, func(q *store.Queries) error { return q.RecordNotificationSeen(ctx, id, now.Add(-time.Hour)) }); err != nil {
		t.Fatal(err)
	}

	if got := lines(); len(got) != 0 {
		t.Fatalf("notifications that arrive gave lines: %+v", got)
	}

	if err := r.kit.Store.Do(ctx, func(q *store.Queries) error { return q.RecordNotificationsLost(ctx, id, now) }); err != nil {
		t.Fatal(err)
	}

	want := []notices.Notice{{
		ID: fmt.Sprintf("accounts:%d:notifications_lost:%d", id, now.Unix()), Source: notices.SourceAccounts,
		Code: "account_notifications_lost", Tone: notices.ToneWarn, Params: map[string]any{"account": "Маркет"},
	}}
	if got := lines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("notices %+v, want %+v", got, want)
	}

	if err := r.kit.Store.Do(ctx, func(q *store.Queries) error { return q.RecordNotificationSeen(ctx, id, now.Add(time.Minute)) }); err != nil {
		t.Fatal(err)
	}

	if got := lines(); len(got) != 0 {
		t.Fatalf("after the next notification the line stays: %+v", got)
	}
}
