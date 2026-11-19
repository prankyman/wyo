package yandex_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace/yandex"
)

func TestANotificationOfYandexMarketIsReadIntoTheOrdersItNames(t *testing.T) {
	t.Parallel()

	a := yandex.New(yandex.Options{})

	cases := []struct {
		name    string
		body    string
		kind    string
		ping    bool
		numbers []string
		fails   bool
	}{
		{name: "the check", body: `{"notificationType":"PING","time":"2026-11-19T09:00:00Z"}`, kind: "PING", ping: true},
		{
			name: "a new order", body: `{"notificationType":"ORDER_CREATED","campaignId":1,"orderId":123,"createdAt":"2026-11-19T09:00:00Z","items":[]}`,
			kind: "ORDER_CREATED", numbers: []string{"123"},
		},
		{
			name: "a changed order", body: `{"notificationType":"ORDER_UPDATED","campaignId":1,"orderId":124,"updateType":"SHIPMENT_DATE_UPDATED"}`,
			kind: "ORDER_UPDATED", numbers: []string{"124"},
		},
		{
			name: "a new state", body: `{"notificationType":"ORDER_STATUS_UPDATED","campaignId":1,"orderId":125,"status":"PROCESSING","substatus":"READY_TO_SHIP"}`,
			kind: "ORDER_STATUS_UPDATED", numbers: []string{"125"},
		},
		{
			name: "a cancellation", body: `{"notificationType":"ORDER_CANCELLED","campaignId":1,"orderId":126,"items":[]}`,
			kind: "ORDER_CANCELLED", numbers: []string{"126"},
		},
		{name: "a kind wyo does not read", body: `{"notificationType":"CHAT_CREATED","chatId":5}`, kind: "CHAT_CREATED"},
		{name: "an order's notification without its number", body: `{"notificationType":"ORDER_CREATED"}`, fails: true},
		{name: "no kind", body: `{"orderId":5}`, fails: true},
		{name: "not JSON", body: `<xml/>`, fails: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			n, err := a.ReadNotification([]byte(tc.body))
			if tc.fails {
				if err == nil {
					t.Fatalf("ReadNotification(%s) = %+v, want a refusal", tc.body, n)
				}

				return
			}

			if err != nil || n.Kind != tc.kind || n.Ping != tc.ping || !reflect.DeepEqual(n.Numbers, tc.numbers) {
				t.Fatalf("ReadNotification = %+v, %v", n, err)
			}
		})
	}
}

func TestTheAnswersToANotificationAreInYandexMarketsForm(t *testing.T) {
	t.Parallel()

	a := yandex.New(yandex.Options{})

	var ok struct{ Version, Name, Time string }
	if err := json.Unmarshal(a.NotificationAnswer("0.0.11", time.Date(2026, 11, 19, 12, 0, 0, 0, moscow)), &ok); err != nil ||
		ok.Version != "0.0.11" || ok.Name != "wyo" || ok.Time != "2026-11-19T09:00:00Z" {
		t.Fatalf("the answer: %+v, %v", ok, err)
	}

	var refused struct {
		Error struct{ Type, Message string }
	}
	if err := json.Unmarshal(a.NotificationRefusal("no orderId"), &refused); err != nil || refused.Error.Type != "WRONG_EVENT_FORMAT" ||
		refused.Error.Message != "no orderId" {
		t.Fatalf("the refusal: %+v, %v", refused, err)
	}
}
