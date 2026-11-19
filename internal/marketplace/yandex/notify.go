package yandex

import (
	"encoding/json"
	"errors"
	"net/netip"
	"strconv"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// networks are the address ranges Яндекс Маркет sends its notifications from: «5.45.207.0/25», «141.8.142.0/25»,
// «5.255.253.0/25» (YMC §8, «Проверяйте каждый запрос по диапазонам IP-адресов»).
var networks = []netip.Prefix{
	netip.MustParsePrefix("5.45.207.0/25"), netip.MustParsePrefix("141.8.142.0/25"), netip.MustParsePrefix("5.255.253.0/25"),
}

// The kinds of notification the wyo server reads (YMC §8 and Яндекс Маркет's page of «POST notification», read 2026-11-19):
// the check, and the four that name an order whose state, lines or deadline may have changed. Every other kind (returns,
// cancellation requests of DBS, reviews, chats) is answered and not read.
const kindPing = "PING"

var orderKinds = map[string]bool{
	"ORDER_CREATED": true, "ORDER_UPDATED": true, "ORDER_STATUS_UPDATED": true, "ORDER_CANCELLED": true,
}

// NotificationNetworks implements marketplace.NotificationReceiver.
func (a *Adapter) NotificationNetworks() []netip.Prefix { return networks }

// ReadNotification implements marketplace.NotificationReceiver: «notificationType», and «orderId» for the kinds that name
// an order. Nothing else of the body is kept.
func (a *Adapter) ReadNotification(body []byte) (marketplace.Notification, error) {
	var in struct {
		Type    string `json:"notificationType"`
		OrderID *int64 `json:"orderId"`
	}

	if err := json.Unmarshal(body, &in); err != nil {
		return marketplace.Notification{}, errors.New("the body is not JSON")
	}

	switch {
	case in.Type == "":
		return marketplace.Notification{}, errors.New("notificationType is missing")
	case in.Type == kindPing:
		return marketplace.Notification{Kind: in.Type, Ping: true}, nil
	case !orderKinds[in.Type]:
		return marketplace.Notification{Kind: in.Type}, nil
	case in.OrderID == nil:
		return marketplace.Notification{}, errors.New("orderId is missing")
	}

	return marketplace.Notification{Kind: in.Type, Numbers: []string{strconv.FormatInt(*in.OrderID, 10)}}, nil
}

// NotificationAnswer implements marketplace.NotificationReceiver: «{ version, name, time }» (YMC §8).
func (a *Adapter) NotificationAnswer(version string, now time.Time) []byte {
	raw, _ := json.Marshal(map[string]string{"version": version, "name": "wyo", "time": now.UTC().Format(time.RFC3339)}) // texts always encode

	return raw
}

// NotificationRefusal implements marketplace.NotificationReceiver: «{ error: { type, message } }» with the type
// WRONG_EVENT_FORMAT («неправильный тип уведомления»), as Яндекс Маркет asks for a notification that cannot be read.
func (a *Adapter) NotificationRefusal(reason string) []byte {
	raw, _ := json.Marshal(map[string]map[string]string{"error": {"type": "WRONG_EVENT_FORMAT", "message": reason}}) // texts always encode

	return raw
}
