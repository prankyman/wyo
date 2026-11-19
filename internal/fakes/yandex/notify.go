package yandex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Notifications (YMC section 8; Яндекс Маркет's page https://yandex.ru/dev/market/partner-api/doc/ru/push-notifications/reference/sendNotification.md,
// read on 2026-11-19): Яндекс Маркет sends POST with a JSON body to the address the seller enters in the cabinet. An order's
// notification carries the order's number, the shop's number and the new status or the lines in short, not the whole order. The
// body of each kind below is the page's: the field names are its.
//
// The fake sends them from inside the control call that caused them (control.go), so that when the control call answers, the
// notification has been answered. A failed notification is not sent again: Яндекс Маркет sends it again, and for how long is not
// something the fake models (the fake's choice, not Яндекс Маркет's). The timeouts are Яндекс Маркет's: 10 seconds for an
// ordinary notification and 1 second for a PING (YMC section 8).
const (
	notifyPing      = "PING"
	notifyChat      = "CHAT_CREATED"
	timeoutPing     = time.Second
	timeoutOrdinary = 10 * time.Second
)

// notification is what the notifications of an order begin with.
type notification struct {
	NotificationType string `json:"notificationType"`
	CampaignID       int64  `json:"campaignId"`
	OrderID          int64  `json:"orderId"`
}

type notifyItem struct {
	OfferID string `json:"offerId"`
	Count   int    `json:"count"`
}

type orderCreated struct {
	notification

	CreatedAt string       `json:"createdAt"`
	Items     []notifyItem `json:"items"`
}

type statusChanged struct {
	notification

	Status    string `json:"status"`
	Substatus string `json:"substatus"`
	UpdatedAt string `json:"updatedAt"`
}

type orderCancelled struct {
	notification

	CancelledAt string       `json:"cancelledAt"`
	Items       []notifyItem `json:"items"`
}

type orderUpdated struct {
	notification

	UpdateType string `json:"updateType"`
	UpdatedAt  string `json:"updatedAt"`
}

type pingBody struct {
	NotificationType string `json:"notificationType"`
	Time             string `json:"time"`
}

// chatCreated is a kind the wyo server ignores. The fields are the page's (chatId, businessId, createdAt); their values are
// made up.
type chatCreated struct {
	NotificationType string `json:"notificationType"`
	ChatID           int64  `json:"chatId"`
	BusinessID       int64  `json:"businessId"`
	CreatedAt        string `json:"createdAt"`
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func (o *order) header(kind string) notification {
	return notification{NotificationType: kind, CampaignID: o.campaignID, OrderID: o.id}
}

func (o *order) notified() []notifyItem {
	out := make([]notifyItem, len(o.items))
	for i, it := range o.items {
		out[i] = notifyItem{OfferID: it.OfferID, Count: it.Count}
	}

	return out
}

// notice is one notification to send, decided while the state was locked: where it goes, what it says, and whether a test
// commanded it lost or sent twice.
type notice struct {
	kind    string
	address string
	body    []byte
	lost    bool
	twice   bool
}

// sent is one entry of what a control call reports: the kind of a notification and what its address answered, 0 when the
// notification was lost or the address could not be reached.
type sent struct {
	Type   string `json:"type"`
	Status int    `json:"status"`
}

// notice makes the notification of a kind, or none when no address is set. It uses up one of the commanded losses, or else one
// of the commanded duplicates.
func (st *state) notice(kind string, body any) []notice {
	if st.notify.address == "" {
		return nil
	}

	raw, err := json.Marshal(body)
	if err != nil {
		// The bodies are made of strings and numbers; this cannot happen.
		return nil
	}

	n := notice{kind: kind, address: st.notify.address, body: raw}

	switch {
	case st.notify.loseNext > 0:
		st.notify.loseNext--
		n.lost = true
	case st.notify.duplicateNext > 0:
		st.notify.duplicateNext--
		n.twice = true
	}

	return []notice{n}
}

// deliver sends the notices in order and reports each. The list is never nil, so a control call that sent nothing says so.
func (s *Server) deliver(ctx context.Context, notices []notice) []sent {
	out := make([]sent, 0, len(notices))

	for _, n := range notices {
		if n.lost {
			out = append(out, sent{Type: n.kind})

			continue
		}

		out = append(out, sent{Type: n.kind, Status: s.post(ctx, n)})

		if n.twice {
			out = append(out, sent{Type: n.kind, Status: s.post(ctx, n)})
		}
	}

	return out
}

// post sends one notification and returns what its address answered, or 0 when it could not be reached.
func (s *Server) post(ctx context.Context, n notice) int {
	timeout := timeoutOrdinary
	if n.kind == notifyPing {
		timeout = timeoutPing
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.address, bytes.NewReader(n.body))
	if err != nil {
		return 0
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := s.client.Do(req)
	if err != nil {
		return 0
	}

	defer func() { _ = res.Body.Close() }()

	_, _ = io.Copy(io.Discard, res.Body)

	return res.StatusCode
}

type notificationsRequest struct {
	Address       *string `json:"address"`
	LoseNext      *int    `json:"lose_next"`
	DuplicateNext *int    `json:"duplicate_next"`
	SendIgnored   bool    `json:"send_ignored"`
}

// setNotifications sets where the notifications go (which sends one PING) and what happens to the next ones. The PING is made
// before the new losses and duplicates are set, so they count the notifications sent after this call; the CHAT_CREATED of
// send_ignored is made after, so that it counts too.
func (st *state) setNotifications(in notificationsRequest) (outcome, error) {
	if err := in.check(); err != nil {
		return outcome{}, err
	}

	if in.SendIgnored && in.Address == nil && st.notify.address == "" {
		return outcome{}, errors.New("send_ignored needs an address: set one first")
	}

	var out outcome

	if in.Address != nil {
		st.notify.address = *in.Address
		out.notices = append(out.notices, st.notice(notifyPing, pingBody{NotificationType: notifyPing, Time: stamp(st.now())})...)
	}

	if in.LoseNext != nil {
		st.notify.loseNext = *in.LoseNext
	}

	if in.DuplicateNext != nil {
		st.notify.duplicateNext = *in.DuplicateNext
	}

	if in.SendIgnored {
		st.notify.nextChat++

		out.notices = append(out.notices, st.notice(notifyChat, chatCreated{
			NotificationType: notifyChat, ChatID: st.notify.nextChat, BusinessID: st.business.ID, CreatedAt: stamp(st.now()),
		})...)
	}

	return out, nil
}

func (in notificationsRequest) check() error {
	if in.Address != nil {
		u, err := url.Parse(*in.Address)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("address %q is not an http or https address with a host", *in.Address)
		}
	}

	if (in.LoseNext != nil && *in.LoseNext < 0) || (in.DuplicateNext != nil && *in.DuplicateNext < 0) {
		return errors.New("lose_next and duplicate_next cannot be negative")
	}

	return nil
}
