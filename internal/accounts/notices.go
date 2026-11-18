package accounts

import (
	"context"
	"fmt"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/store"
)

// The days before a key's end the bell tells of it: first 14 days before (the account becomes «Ключ истекает»), and again
// 3 days before, as a new notice, so that it is seen once more.
const (
	firstWarningDays  = 14
	secondWarningDays = 3
)

const day = 24 * time.Hour

// Notices implements notices.Source: for every account that is not switched off, a key that ends within 14 days (and
// again within 3), a key that has ended, a key the marketplace did not accept, and an account without connection. Each
// notice names the account by the owner's name of it; the interface sends the owner to «Маркетплейсы».
func (s *Service) Notices(ctx context.Context) ([]notices.Notice, error) {
	rows, err := s.opts.Store.Read().Accounts(ctx)
	if err != nil {
		return nil, err
	}

	now := s.opts.Clock.Now()
	list := []notices.Notice{}

	for _, row := range rows {
		list = append(list, noticesOf(row, now)...)
	}

	return list, nil
}

// noticesOf are the notices of one account at the moment now.
func noticesOf(row store.Account, now time.Time) []notices.Notice {
	notice := func(cause, code, tone string, params map[string]any) notices.Notice {
		all := map[string]any{"account": row.Name}
		for k, v := range params {
			all[k] = v
		}

		return notices.Notice{
			ID: fmt.Sprintf("accounts:%d:%s", row.ID, cause), Source: notices.SourceAccounts, Code: code, Tone: tone, Params: all,
		}
	}

	ended := !row.KeyExpiresAt.IsZero() && !now.Before(row.KeyExpiresAt)

	switch marketplace.AccountState(row.State) {
	case marketplace.AccountDisabled:
		return nil
	case marketplace.AccountKeyRefused:
		return []notices.Notice{notice("key_refused", "account_key_refused", notices.ToneStop, nil)}
	case marketplace.AccountKeyExpired:
		ended = true
	case marketplace.AccountWorking, marketplace.AccountKeyExpiring, marketplace.AccountNoConnection:
	}

	if ended {
		return []notices.Notice{notice("key_expired", "account_key_expired", notices.ToneStop, nil)}
	}

	var list []notices.Notice

	if row.State == string(marketplace.AccountNoConnection) {
		list = append(list, notice("no_connection", "account_no_connection", notices.ToneWarn, nil))
	}

	if days, soon := daysLeft(row.KeyExpiresAt, now); soon {
		warning := firstWarningDays
		if days <= secondWarningDays {
			warning = secondWarningDays
		}

		list = append(list, notice(fmt.Sprintf("key_expiring:%d", warning), "account_key_expiring", notices.ToneWarn, map[string]any{"days": days}))
	}

	return list
}

// daysLeft is how many days, rounded up, a key still lives, and whether that is within the first warning.
func daysLeft(expiresAt, now time.Time) (int, bool) {
	if expiresAt.IsZero() {
		return 0, false
	}

	left := expiresAt.Sub(now)
	days := int((left + day - 1) / day)

	return days, days <= firstWarningDays
}
