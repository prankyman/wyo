package yandex

import "github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"

// The made-up personal data every answer about orders carries. Яндекс Маркет's list of orders (getBusinessOrders) returns the
// buyer's comment (notes) and the delivery address with every order, and the courier's name and telephone while a courier
// brings the order (YMC section 7); the buyer's own name and telephone are not in this answer (the schema BusinessOrderDTO has
// no buyer, and Яндекс Маркет gives them by another method, getOrderBuyerInfo). The wyo server drops all of it at the adapter's
// edge. The values are made up and recognisable, so that a test elsewhere can search the wyo server's journal, logs, database
// and answers for them with BuyerValues and find nothing. Which of the fields the real Яндекс Маркет fills for which scheme is
// not sourced: the fake fills all of them for every order, which is the fake's choice, not Яндекс Маркет's.
const (
	BuyerComment   = "Позвонить за час до доставки"
	BuyerCity      = "Город-Выдуманный"
	BuyerStreet    = "Улица-Выдуманная"
	BuyerHouse     = "Дом-Выдуманный-1"
	BuyerApartment = "Квартира-Выдуманная-2"
	BuyerPostcode  = "987654"
	CourierName    = "Курьер Выдуманный"
	CourierPhone   = "+79876543210"
)

// BuyerValues lists every made-up personal value of the answers about orders.
func BuyerValues() []string {
	return []string{BuyerComment, BuyerCity, BuyerStreet, BuyerHouse, BuyerApartment, BuyerPostcode, CourierName, CourierPhone}
}

// delivery is the delivery part of an order's answer that carries the made-up address and courier.
func personalDelivery() (*yandexapi.BusinessOrderCourierDeliveryDTO, *yandexapi.BusinessOrderTransferDTO) {
	address := yandexapi.BusinessOrderDeliveryAddressDTO{
		City: ptr(BuyerCity), Street: ptr(BuyerStreet), House: ptr(BuyerHouse), Apartment: ptr(BuyerApartment), Postcode: ptr(BuyerPostcode),
	}

	return &yandexapi.BusinessOrderCourierDeliveryDTO{Address: &address},
		&yandexapi.BusinessOrderTransferDTO{Courier: &yandexapi.OrderCourierDTO{FullName: ptr(CourierName), Phone: ptr(CourierPhone)}}
}

func ptr[T any](v T) *T { return &v }
