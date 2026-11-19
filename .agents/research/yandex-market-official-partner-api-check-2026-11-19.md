# Яндекс Маркет's official Partner API documents and description file, read on 2026-11-19: the key, the limits, orders, notifications, test orders, the FBS and Express walks, the daily shipment, and what is still not sourced

> The research check of the Яндекс Маркет plans of Stage 2 (Plans 7 and 8 of this repository; the high-level plan is
> `.agents/plans/Marketplace_connector_plan.md`). Cited in the plans as **«YMC»**. Every fact below is quoted from Яндекс Маркет's own
> documents or its own description file, both read on 2026-11-19; a gap is marked **NOT SOURCED**, and a reading of sourced text that is
> not a quotation is marked **Inference**. No request was sent to the API itself for this check except one stray request, without a key,
> to the root of `api.partner.market.yandex.ru` (answer `404`); nothing else is ever sent there from a development machine (rule R3).

## 1. How this was sourced

1. **The documents** — `https://yandex.ru/dev/market/partner-api/doc/ru/`. Every page is also published as Markdown at the same address
   with `.md` at the end, and the index of every page is `https://yandex.ru/dev/market/partner-api/doc/ru/llms.txt`. Both are reachable
   from the development machine. A page cited below as «`concepts/limits`» is
   `https://yandex.ru/dev/market/partner-api/doc/ru/concepts/limits.md`.
2. **The description file** — the official repository `https://github.com/yandex-market/yandex-market-partner-api` (licence
   BSD-3-Clause), commit `9be9f98e111133efdb31f7260bdb27358b378b29` of 2026-11-10 14:02:26 +0300:

    | Fact                         | Value                                                                                                  |
    | ---------------------------- | ------------------------------------------------------------------------------------------------------ |
    | Files under `openapi/`       | 1 087 (`openapi.yaml`, `paths/*.yaml`, `components/{schemas,parameters,responses}/*.yaml`)              |
    | Size of `openapi/`           | 1 116 517 bytes                                                                                        |
    | SHA-256 of `openapi.yaml`    | `16157bbf948b88cd1df7abcd2073502d473e733714de3d244f37b36ac61e15ae`                                     |
    | Per method                   | `x-auth-scopes` (the access levels that open it), `x-resource-limit-config` (its limit), `deprecated` |

3. **The generator.** oapi-codegen v2.8.2, the version pinned in `scripts/tools/versions.sh`, stops with «fatal error: stack overflow»
   (an endless recursion in `codegen.GenerateGoSchema`, `schema.go:1005`) when it is given the multi-file description as published.
   Joined into one file — every `$ref` to a file `components/<kind>/<Name>.yaml` rewritten to `#/components/<kind>/<Name>`, nothing
   else changed — the same generator produced the client without an error (851 schemas, 60 parameters, 16 responses; 15 110 lines of
   Go for the methods of section 5). No reference was found that could not be rewritten this way.

## 2. The key

- **One kind of key is current: the Api-Key token**, sent in the header `Api-Key` (`concepts/api-key`, `concepts/authorization`). OAuth
  tokens are the old way and are not used for new work.
- «Токен действует бессрочно — он остается активным до тех пор, пока вы его не удалите.» The table of `concepts/api-key` compares the
  two kinds: Api-Key — «Доступ к магазинам: Все магазины в кабинете», «Срок действия токена: Неограничен».
- «Максимальное количество токенов для кабинета — 30.» «Создавать токены и управлять ими может только владелец кабинета и менеджер
  кабинета.»
- **Where it is made:** «В кабинете продавца на Маркете нажмите на иконку вашего аккаунта → **Настройки** и в меню слева выберите
  **API и модули**» → «Токены авторизации» → «Создать новый токен».
- **Access levels (scopes)** are chosen when the token is made. The ones the plans need: `inventory-and-order-processing` («Обработка
  заказов и учёт товаров») and its read-only twin `inventory-and-order-processing:read-only` («Просмотр информации о заказах»); the
  full `all-methods` and `all-methods:read-only`; `offers-and-cards-management:read-only` for barcodes (section 6).
- **Reading the key back:** `POST /v2/auth/token` (`getAuthTokenInfo`, `reference/auth/getAuthTokenInfo`) answers the token's name and
  its `authScopes` (enum values such as `INVENTORY_AND_ORDER_PROCESSING`). Its limit is 100 an hour.

## 3. Naming the program

`concepts/integration-signing`: «Формат: `X-Market-Integration: название интеграции/версия`», for example
`X-Market-Integration: Bitrix/1.0.0`; «Только ASCII символы», at most 100 characters. This fits rule R5 (the wyo server names itself in
every request).

## 4. Limits and errors

- **At once:** «Разрешается выполнять не более четырёх одновременных запросов» (`concepts/limits`, «глобальные ограничения»). A method
  may name its own number of parallel requests in `x-resource-limit-config.parallel-limit-value` (6 for `getBusinessOrders`).
- **Per method and hour:** each method has its own amount per period (`x-resource-limit-config`; the table of section 5).
- **A met limit is answered `420 Enhance Your Calm`**, not `429`, with the headers `X-RateLimit-Resource-Limit` (the limit),
  `X-RateLimit-Resource-Until` («дату, до которой действует ограничение, в формате RFC822», e.g. `Thu, 10 Jul 2018 00:42:42 GMT`) and
  `X-RateLimit-Resource-Remaining` (what is left).
- **What uses the limit up:** «при успешных ответах (2xx) ресурсный лимит уменьшается»; «при ошибках клиента (4xx …) ресурсный лимит
  также уменьшается»; «при ошибках со стороны Маркета (5xx, включая 500) ресурсный лимит не уменьшается».
- **A request body** is at most 512 KB.
- **An error body** is `{ "status": "ERROR", "errors": [{ "code", "message" }] }` (`concepts/error-codes`). Among the words: `401` —
  «Api-Key token is invalid», «Api-Key token is revoked»; `403` — «Token does not have any of the scopes to access the API method»,
  «API for campaign … disabled». The shipment methods answer, among others: «Cutoff time for shipments has not been reached yet»;
  «Some orders have not been processed yet. Please change the status of orders to READY_TO_SHIP»; «Shipment … has been confirmed
  already»; «Some orders are in the process of being excluded from shipment … wait up to 30 minutes».

## 5. The methods the plans use

| Method (`operationId`)              | Request                                                                     | Limit                    | Access level                             |
| ----------------------------------- | --------------------------------------------------------------------------- | ------------------------ | ---------------------------------------- |
| `getAuthTokenInfo`                  | `POST /v2/auth/token`                                                       | 100 an hour              | any                                      |
| `getCampaigns`                      | `GET /v2/campaigns`                                                         | 1 000 an hour            | any                                      |
| `getBusinessOrders`                 | `POST /v1/businesses/{businessId}/orders`                                   | 10 000 an hour, 6 at once | reading orders                          |
| `getPartnerWarehouses`              | `POST /v3/businesses/{businessId}/warehouses`                               | 100 an hour              | reading orders                           |
| `getOfferMappings`                  | `POST /v2/businesses/{businessId}/offer-mappings`                           | 100 a minute             | viewing products and cards               |
| `setOrderBoxLayout`                 | `PUT /v2/campaigns/{campaignId}/orders/{orderId}/boxes`                      | 10 000 an hour           | `inventory-and-order-processing`         |
| `getOrderIdentifiersStatus`         | `POST /v2/campaigns/{campaignId}/orders/{orderId}/identifiers/status`        | 1 000 an hour            | reading orders                           |
| `generateOrderLabels`               | `GET /v2/campaigns/{campaignId}/orders/{orderId}/delivery/labels`            | 10 000 an hour           | `inventory-and-order-processing`         |
| `generateOrderLabel`                | `GET …/delivery/shipments/{shipmentId}/boxes/{boxId}/label`                  | 10 000 an hour           | `inventory-and-order-processing`         |
| `updateOrderStatus`                 | `PUT /v2/campaigns/{campaignId}/orders/{orderId}/status`                     | 10 000 an hour           | `inventory-and-order-processing`         |
| `verifyOrderEac`                    | `PUT /v2/campaigns/{campaignId}/orders/{orderId}/verifyEac`                  | 10 000 an hour           | `inventory-and-order-processing`         |
| `searchShipments`                   | `PUT /v2/campaigns/{campaignId}/first-mile/shipments`                        | 200 an hour              | reading orders                           |
| `getShipment`                       | `GET /v2/campaigns/{campaignId}/first-mile/shipments/{shipmentId}`           | 200 an hour              | reading orders                           |
| `confirmShipment`                   | `POST …/first-mile/shipments/{shipmentId}/confirm`                           | 200 an hour              | `inventory-and-order-processing`         |
| `downloadShipmentAct`               | `GET …/first-mile/shipments/{shipmentId}/act`                                | 200 an hour              | `inventory-and-order-processing`         |

`getOrders` and `getOrder` (the per-shop order lists) are marked `deprecated`. Never used, because each lowers the shop's quality index
or deletes goods (sections 10 and 11): `updateOrderItems`, `acceptOrderCancellation` (DBS only), `updateOrderStatus` with `CANCELLED`,
`setOrderBoxLayout` with `allowRemove: true`, `transferOrdersFromShipment`.

## 6. Cabinet, shops, warehouses and products

- **The cabinet (business)** holds the shops (campaigns). `GET /v2/campaigns` lists the shops the key reaches, each with its `id`,
  `business.id` and `placementType` (`FBS` covers both FBS and Express shops).
- **Warehouses:** `POST /v3/businesses/{businessId}/warehouses` lists each warehouse's `id` (a number), `name` and `models[]`
  (`placementType`, `campaignId`, `apiAvailability`). The `/v2/…/warehouses` method is about groups of warehouses.
- **Barcodes** are not in an order. They come from `getOfferMappings`, which needs the access level for viewing products and cards.

## 7. Reading orders

- `POST /v1/businesses/{businessId}/orders` (`reference/orders/getBusinessOrders`): filters `campaignIds`, `statuses`, `substatuses`,
  `programTypes` (`FBY`, `FBS`, `DBS`, `EXPRESS`, `LAAS`), `dates` (by creation, by shipment or by update; at most 30 days),
  `fake`, `orderIds` (at most 50); pages by `pageToken`, at most 50 orders a page.
- **How often** (`concepts/best-practices`): «Экспресс — каждые 10-15 минут»; «FBS, DBS — не реже одного раза в час».
- **An order** carries `orderId`, `campaignId`, `programType`, `status`, `substatus`, `creationDate`, `updateDate`, `fake`, and
  `items[]` — each with `id` (the line's number in the order), `offerId` (the seller's article), `offerName`, `count`, `instances` and
  `requiredInstanceTypes` (`CIS`, `CIS_OPTIONAL`, `UIN`, `RNPT`, `GTD`); `delivery` with `warehouseId` («идентификатор склада в
  системе магазина», a text), `shipment` (`id`, `shipmentDate`, `shipmentTime`) and `boxesLayout`. The buyer's comment (`notes`) and
  address are dropped by the wyo server (no buyer datum is stored).
- **States:** `PROCESSING` with the substatus `STARTED` (the order may be worked on) or `READY_TO_SHIP` (packed and ready); then
  `DELIVERY`, `PICKUP`, `DELIVERED`, `CANCELLED` (with many reasons), `UNPAID`, `PENDING` and others.

## 8. Notifications

`push-notifications/index` and `push-notifications/reference/sendNotification`:

- Яндекс Маркет sends `POST notification` to the address the seller enters in the cabinet. «Маркет обменивается данными только с
  магазинами, которые работают по протоколу HTTPS с использованием SSL-сертификата. Он должен быть подписан официальным центром
  сертификации, самоподписанный сертификат не подойдет.» Which authorities count is not listed (**NOT SOURCED** for Let's Encrypt).
- The page advises checking every request against Яндекс Маркет's address ranges: `5.45.207.0/25`, `141.8.142.0/25`,
  `5.255.253.0/25`.
- **Types** (`notificationType`): `PING`, `ORDER_CREATED`, `ORDER_CANCELLED`, `ORDER_STATUS_UPDATED`, `ORDER_UPDATED`,
  `ORDER_RETURN_CREATED`, `ORDER_RETURN_STATUS_UPDATED`, `ORDER_CANCELLATION_REQUEST` (DBS), `ORDER_LINE_SERVICE_STATUS_UPDATED`, and
  types about reviews and chats. **An order's notification carries the order's number, the shop's number and the new status or the
  lines in short, not the whole order**; the documents point to `getBusinessOrders` for the details.
- **Answers:** «Таймаут на ответ для обычных уведомлений составляет 10 секунд, для проверочных уведомлений типа `PING` — 1 секунда»;
  the answer is `200` with `{ version, name, time }`. A failed notification is sent again. «Если API магазина недоступен в течение 14
  дней с момента отправки первого уведомления, магазин отключается от интеграции».
- **Test orders are notified too** (section 9).

## 9. Test orders

`concepts/sandbox` («Тестовые заказы»):

- Made by the seller in the cabinet: account icon → «Настройки» → «API и модули» → «Тестовый заказ»; paid with a test card, «деньги не
  списываются».
- «Маркет не начисляет плату за такие заказы. А ошибки в работе с ними не влияют на проверки и не используются в расчете индекса
  качества.»
- «Все созданные в интерфейсе отладки заказы поступают магазину со значением `true` параметра `fake`»; «методы, которые возвращают
  информацию по заказам, по умолчанию не включают данные по тестовым. Чтобы их получить, передайте значение `true` в параметре
  `fake`.» «Если у вас подключены API-уведомления, Маркет отправит вам запрос `POST notification` … также и по тестовым заказам.»
- «Информация о тестовых заказах и логи запросов хранятся в течение 10 дней.»

## 10. The FBS walk

`step-by-step/fbs` and the reference pages of each method:

1. Read the order (section 7).
2. **Boxes and marking codes in one request:** `setOrderBoxLayout` sends how the goods lie in boxes and, in `instances[].cis`, each
   unit's code (the group separator written as `\u001d`). A later sending replaces the earlier one until the order is
   `READY_TO_SHIP`. A line is sent with `fullCount`, or with `partialCount { current, total }` when one unit is split over several
   boxes. The answer names each box's `boxId`. `allowRemove: true` with fewer units deletes goods from the order — it cannot be undone
   and lowers the quality index.
3. **The verdict on the codes:** `getOrderIdentifiersStatus`. For FBS, `READY_TO_SHIP` is allowed only after every `CIS` and `UIN`
   passed.
4. **Labels:** `generateOrderLabels` gives one PDF with every box's label; `generateOrderLabel` one box's (its `shipmentId` is
   outdated: any number). The size is chosen by `format`: «`A9_HORIZONTALLY` — ярлык размером 58 × 40 мм без полей», «`A9` — ярлык
   размером 40 × 58 мм без полей», «`A7` — ярлык размером 75 × 120 мм (80,4 × 125,6 мм с учетом полей)» (the default), and `A4` (a
   sheet holding the label size chosen in the cabinet). The files come from the API's own host.
5. **«Готов к отгрузке»:** `updateOrderStatus` with `PROCESSING` / `READY_TO_SHIP` — the seller confirms the goods are there and will
   be shipped; only such orders enter the act.
6. Cancelling or cutting an order lowers the quality index.

## 11. The daily shipment (`first-mile`)

`reference/shipments/*`:

- Яндекс Маркет itself makes one shipment per warehouse and day, by the schedule set in the cabinet. `shipmentType`: «`IMPORT` — вы
  самостоятельно привозите заказы в выбранный сортировочный центр или пункт приема заказов»; «`WITHDRAW` — вы отгружаете заказы со
  своего склада курьерам Яндекс Маркета».
- `searchShipments` finds shipments by dates, statuses and `orderIds`; `getShipment` gives its `orderIds`, `warehouse.id`, the planned
  interval and `availableActions` (`CONFIRM`, `DOWNLOAD_ACT` and others). Statuses: `OUTBOUND_CREATED`,
  `OUTBOUND_READY_FOR_CONFIRMATION`, `OUTBOUND_CONFIRMED`, `OUTBOUND_SIGNED`, `ACCEPTED`, `ACCEPTED_WITH_DISCREPANCIES`, `FINISHED`,
  `ERROR`.
- `confirmShipment` confirms, only after the cutoff time; `downloadShipmentAct` gives the act as PDF once the shipment is confirmed.
  The waybill (`…/transportation-waybill`, only for `WITHDRAW`) is an `.xls` file.
- `transferOrdersFromShipment` moves orders to the next shipment: «Такие переносы снижают индекс качества магазина»; «Переносить
  заказы можно, если до формирования отгрузки осталось больше получаса»; «Перенос происходит не мгновенно, а занимает несколько
  минут.»
- **NOT SOURCED:** whether Яндекс Маркет moves an order that is not `READY_TO_SHIP` out of a shipment by itself. The error «Some orders
  are in the process of being excluded from shipment» suggests it can (**Inference**); the real-account check settles it.

## 12. Express

`step-by-step/express` and `reference/order-delivery/verifyOrderEac`:

1. Read the order (section 7). 2. Marking codes by `setOrderBoxLayout`, as FBS. 3. Pack, fetch the labels with
   `generateOrderLabels`, set `READY_TO_SHIP`. 4. «Передайте код подтверждения, который вам назвал курьер, в запросе `PUT …/verifyEac`.
   Если проверка кода выполнена успешно, передайте заказ курьеру.» There is no daily shipment and no act.

- The kind of code is the order's `delivery.eacType`, given «Если у магазина настроена работа с кодами подтверждения», at the order's
  statuses `COURIER_FOUND`, `COURIER_ARRIVED_TO_SENDER` and `DELIVERY_SERVICE_UNDELIVERED`: «`COURIER_TO_MERCHANT` — курьер передает
  код продавцу для получения заказа»; «`MERCHANT_TO_COURIER` (временно не возвращается) — продавец передает код курьеру для получения
  невыкупа»; `CHECKING_BY_MERCHANT` — «продавец проверяет код на своей стороне».
- **The answer** of `verifyOrderEac`: `verificationResult` — «`ACCEPTED` — код верный», «`REJECTED` — код неверный», «`NEED_UPDATE` —
  Маркет отправит новый код. Значение возвращается, если превышено количество попыток отправки кода» — and `attemptsLeft` («Количество
  оставшихся попыток проверки кода»).
- Cancelling or cutting an Express order lowers the quality index, as for FBS.
- **NOT SOURCED:** what the substatuses `COURIER_SEARCH`, `COURIER_FOUND`, `COURIER_IN_TRANSIT_TO_SENDER`, `COURIER_ARRIVED_TO_SENDER`,
  `COURIER_RECEIVED` and `COURIER_NOT_FOUND` mean exactly: the description file lists them (`OrderSubstatusType`) without words; and
  what `CHECKING_BY_MERCHANT` asks of the seller beyond its one sentence.

## 13. The other schemes

- **FBY** — the goods lie in Яндекс Маркет's warehouses; the seller only reads.
- **DBS** — the seller delivers to the buyer itself and needs the buyer's address.
- **LaaS** — listed in `programTypes`; not used by the plans.

## 14. What the documents do not settle — the checks on the first real account

1. Which warehouse an order belongs to: the order names a text `delivery.warehouseId` («в системе магазина»), while warehouses and
   shipments name a number (`id`, `warehouse.id`); a warehouse's `models[]` name the shop. The real-account check finds out which of
   them matches.
2. Whether Let's Encrypt certificates are accepted for notifications (section 8).
3. Whether an order that is not ready is moved out of a shipment by Яндекс Маркет itself (section 11).
4. The exact meaning of the courier substatuses and of `CHECKING_BY_MERCHANT` (section 12).
