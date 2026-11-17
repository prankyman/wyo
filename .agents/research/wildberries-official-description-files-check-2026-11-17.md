# Wildberries' official description files, read on 2026-11-17: the methods the first Wildberries plans need, their limits, hosts and what is still not sourced

> The research check of the Stage 1 plans (Plans 2–4 of this repository; the high-level plan is
> `.agents/plans/Marketplace_connector_plan.md`). Cited in the plans as **«WBC»**. It re-reads, on the day the plans were written, the
> facts that the earlier research (cited there as «WBF», «WBA», «WBP», «MC»; kept by the maintainer outside this repository) took from
> the same official files on 2026-10-21 and 2026-11-17.

## 1. How this was sourced

1. **Wildberries' own description files** (the OpenAPI files Wildberries publishes for its API; plain HTTP download works, no sign-in),
   downloaded 2026-11-17 and read by a script that lists every method with its host, limit table and token types:

    | Cited as | Address                                                              | Lines  |
    | -------- | -------------------------------------------------------------------- | ------ |
    | [GEN]    | `https://dev.wildberries.ru/api/swagger/yaml/ru/01-general.yaml`     | 2 347  |
    | [ITEMS]  | `https://dev.wildberries.ru/api/swagger/yaml/ru/02-items.yaml`       | 10 141 |
    | [FBS]    | `https://dev.wildberries.ru/api/swagger/yaml/ru/03-orders-fbs.yaml`  | 7 150  |
    | [FBW]    | `https://dev.wildberries.ru/api/swagger/yaml/ru/07-orders-fbw.yaml`  | 2 606  |
    | [AN]     | `https://dev.wildberries.ru/api/swagger/yaml/ru/11-analytics.yaml`   | 7 075  |
    | [REP]    | `https://dev.wildberries.ru/api/swagger/yaml/ru/12-reports.yaml`     | 3 760  |

    The English copies are at the same addresses with `/en/` in place of `/ru/`.

2. **Not read today:** the HTML pages of `dev.wildberries.ru` (knowledge base, news, release notes) — the earlier research found them
   closed to scripts. A fact that only those pages hold is marked **"earlier research, not re-read"** below.
3. Marks: **NOT SOURCED** — no official text found; **Inference** — a reading of sourced text, not a quotation.

A limit is written as Wildberries writes it: period | requests in the period | interval between requests | burst (the number of
requests that may be sent at once without the interval).

## 2. Every method the plans use still exists (2026-11-17)

Host of every row unless said otherwise: `https://marketplace-api.wildberries.ru`. **S** — the method also has the sandbox host
`https://marketplace-api-sandbox.wildberries.ru`. "300" is the limit «1 мин | 300 запросов | 200 мс | 20 запросов».

| Purpose                                                         | Method and path                                                             | Name in the file                           | S   | Limit                                     |
| --------------------------------------------------------------- | --------------------------------------------------------------------------- | ------------------------------------------ | --- | ----------------------------------------- |
| Does the key reach this category of the API                     | `GET /ping` (one per host; 18 hosts listed)                                 | `getPing` [GEN]                            | S   | «30 сек \| 3 запроса \| 10 сек \| 99 запросов» |
| The seller's name, profile id, ИНН                              | `GET https://common-api.wildberries.ru/api/v1/seller-info`                  | `getV1SellerInfo` [GEN]                    | –   | «1 мин \| 1 запрос \| 1 мин \| 10 запросов»   |
| The seller's warehouses                                         | `GET /api/v3/warehouses`                                                    | `getV3Warehouses` [ITEMS]                  | S   | 300                                       |
| Wildberries' offices (the warehouse a seller warehouse is tied to) | `GET /api/v3/offices`                                                    | `getV3Offices` [ITEMS]                     | S   | 300                                       |
| New assembly orders                                             | `GET /api/v3/orders/new`                                                    | `getV3OrdersNew` [FBS]                     | S   | 300                                       |
| Orders by period                                                | `GET /api/v3/orders`                                                        | `getV3Orders` [FBS]                        | S   | 300                                       |
| States of known orders (up to 1000 ids)                         | `POST /api/v3/orders/status`                                                | `postV3OrdersStatus` [FBS]                 | S   | 300                                       |
| Orders Wildberries wants shipped again                          | `GET /api/v3/supplies/orders/reshipment`                                    | `getV3SuppliesOrdersReshipment` [FBS]      | S   | 300                                       |
| Cancel an order (never called by the connector by itself)       | `PATCH /api/v3/orders/{orderId}/cancel`                                     | `patchV3OrdersOrderIdCancel` [FBS]         | S   | «1 мин \| 100 запросов \| 600 мс \| 20 запросов» |
| Order stickers                                                  | `POST /api/v3/orders/stickers`                                              | `postV3OrdersStickers` [FBS]               | S   | 300                                       |
| Read the marking codes and Wildberries' verdict (up to 100)     | `POST /api/marketplace/v3/orders/meta`                                      | `postV3OrdersMeta` [FBS]                   | S   | 300                                       |
| Remove every code of one kind from an order                     | `DELETE /api/v3/orders/{orderId}/meta`                                      | `deleteV3OrdersOrderIdMeta` [FBS]          | S   | 300                                       |
| Send marking codes of an order                                  | `PUT /api/v3/orders/{orderId}/meta/sgtin`                                   | `putV3OrdersOrderIdMetaSgtin` [FBS]        | S   | «1 мин \| 1000 запросов \| 60 мс \| 20 запросов» |
| Create a supply                                                 | `POST /api/v3/supplies`                                                     | `postV3Supplies` [FBS]                     | S   | 300                                       |
| List supplies                                                   | `GET /api/v3/supplies`                                                      | `getV3Supplies` [FBS]                      | S   | 300                                       |
| Put orders into a supply (up to 100)                            | `PATCH /api/marketplace/v3/supplies/{supplyId}/orders`                      | `patchV3SuppliesSupplyIdOrders` [FBS]      | S   | 300                                       |
| One supply; delete an empty one                                 | `GET` / `DELETE /api/v3/supplies/{supplyId}`                                | `getV3SuppliesSupplyId`, `deleteV3SuppliesSupplyId` [FBS] | S | 300                         |
| The order ids of a supply                                       | `GET /api/marketplace/v3/supplies/{supplyId}/order-ids`                     | `getV3SuppliesSupplyIdOrderIds` [FBS]      | S   | 300                                       |
| Move an order from a closed supply back to assembly             | `PATCH /api/marketplace/v3/fbs/supplies/{supplyId}/orders/transfer`         | `patchV3FbsSuppliesSupplyIdOrdersTransfer` [FBS] | – | 300                                 |
| Shipping points for a supply                                    | `GET /api/marketplace/v3/fbs/supplies/{supplyId}/shipping-points`           | `getV3FbsSuppliesSupplyIdShippingPoints` [FBS] | – | 300                                   |
| Set the shipping type, date and point                           | `PATCH /api/marketplace/v3/fbs/supplies/shipping-method`                    | `patchV3FbsSuppliesShippingMethod` [FBS]   | –   | 300                                       |
| Close the supply (hand it to delivery)                          | `PATCH /api/v3/supplies/{supplyId}/deliver`                                 | `patchV3SuppliesSupplyIdDeliver` [FBS]     | S   | 300                                       |
| The supply's QR code                                            | `GET /api/v3/supplies/{supplyId}/barcode`                                   | `getV3SuppliesSupplyIdBarcode` [FBS]       | S   | 300                                       |
| Transport boxes: list, add, delete                              | `GET` / `POST` / `DELETE /api/v3/supplies/{supplyId}/trbx`                  | `…SupplyIdTrbx` [FBS]                      | S   | 300                                       |
| Transport boxes' stickers                                       | `POST /api/v3/supplies/{supplyId}/trbx/stickers`                            | `postV3SuppliesSupplyIdTrbxStickers` [FBS] | S   | 300                                       |
| Orders of every scheme, for the order list (FBO rows)           | `GET https://statistics-api.wildberries.ru/api/v1/supplier/orders`          | `getV1SupplierOrders` [REP]                | –   | «1 мин \| 1 запрос \| 1 мин \| 10 запросов»   |
| Sales (Stage 3 of the high-level plan)                          | `GET https://statistics-api.wildberries.ru/api/v1/supplier/sales`           | `getV1SupplierSales` [REP]                 | –   | «1 мин \| 1 запрос \| 1 мин \| 1 запрос»      |

## 3. What changed since the earlier research (2026-10-21)

- [FBS] grew from 7 017 to 7 150 lines. No method of the table above was removed or renamed.
- `PATCH …/fbs/supplies/{supplyId}/orders/transfer` now carries a limit table (300); on 2026-10-21 it had none.
- The old `GET /api/marketplace/v3/fbs/shipping-points` is still in the file, marked deprecated. The plans never call it; they use
  `GET …/fbs/supplies/{supplyId}/shipping-points`.
- Methods present that the plans do not use: passes (`/api/v3/passes…`), cross-border stickers and status history, the buyer-data
  method `POST /api/v3/orders/client` (**never called**: it returns a buyer's name and phone), the archive of orders older than three
  months, the «СПОТ» methods (`…/fbs/supplies/{supplyId}/spot`, `…/spot/list`, `…/stickers/spot`, `…/dictionaries/countries/oksm`),
  the auto-return settings (the only FBS methods limited to the token types `personal`, `service`, `base-with-secret`).
- [REP] and [AN] give limit tables per token type for the statistics methods; for a token of the type «Базовый» without a secret the
  orders feed is «3 ч | 1 запрос». The plans therefore ask for a Personal token (section 4).

## 4. The token, as [GEN] describes it today

- «Чтобы авторизоваться в API, вам понадобится токен. Он действует 180 дней после создания. Добавляйте токен в заголовок запроса
  `Authorization`.» [GEN, the description of the authorisation]
- The same description names the token types and says of the Personal token that it serves the seller's own programs, including
  programs «развёрнутым на собственной или арендованной инфраструктуре», and that it may not be handed to third parties or used in
  cloud services (the full quotation is in the high-level plan, rule R4, from the page saved in `docs/marketplaces/`).
- The token is a JWT (a signed text of three parts whose middle part can be read without a key). Its fields, as the earlier research
  read them in Wildberries' knowledge base (**earlier research, not re-read**): `exp` — the end of its life; `sid` — the seller's
  profile id, the same value `GET /api/v1/seller-info` returns; `acc` — the type (1 «Базовый», 2 «Тестовый», 3 «Персональный»,
  4 «Сервисный»); `s` — a bit mask of the categories the token opens (bit 4 «Маркетплейс», bit 5 «Статистика», bit 30 «только на
  чтение»); `t` — true for a test token.
- A method that returns the token's type, categories or end of life is **NOT SOURCED**: none is in [GEN]. The documented way is to
  read the token's own fields and to call `/ping` on each host whose category is needed.

## 5. Limits and waiting, as [GEN] describes them today

- «Один запрос с кодами ответов `4XX` учитывается как 10 запросов» (the category «Маркетплейс»); `X-Ratelimit-Remaining` then falls by
  10 at once.
- «`X-Ratelimit-Remaining` — это количество запросов, которое можно выполнить на данный момент без пауз. … Он встречается во всех
  статусах ответов, кроме ошибки `429`.»
- On the answer 429 the headers `X-Ratelimit-Retry` (seconds until a request may be repeated), `X-Ratelimit-Limit` and
  `X-Ratelimit-Reset` are named in the same section. Their exact behaviour on the first real account is a check of the first release.
- [GEN]'s example table for the category «Маркетплейс» gives the Personal token «1 минута | 300 запросов | 200 миллисекунд | 20
  запросов» and the «Базовый» and «Тестовый» tokens «1 минута | 150 запросов | 200 миллисекунд | 10 запросов»; [FBS] adds for the
  sandbox «максимум 1 запрос в секунду суммарно для всех методов Маркетплейса».
- A header by which a program must name itself is **NOT SOURCED** for Wildberries: [GEN] has no such requirement. The connector names
  itself anyway (rule R5 of the high-level plan).

## 6. The orders feed, as [REP] describes it today (`getV1SupplierOrders`)

- «Метод возвращает информацию о заказах. Данные обновляются раз в 30 минут. 1 строка = 1 заказ = 1 сборочное задание = 1 единица
  товара. Для определения заказа рекомендуем использовать поле `srid`. Информация о заказе хранится 90 дней с момента оформления.»
- «В ответах могут отсутствовать заказы, по которым не подтверждена оплата.» — the feed is not complete at every moment.
- Reading on from a mark: `dateFrom` is required («Время передаётся в часовом поясе Москва (UTC+3)»); without `flag` or with
  `flag=0` the answer holds the rows whose `lastChangeDate` is later than `dateFrom`, at most about 80 000 rows; «Во втором и далее
  запросе в параметре `dateFrom` используйте полное значение поля `lastChangeDate` из последней строки ответа на предыдущий запрос.
  Если в ответе отдаётся пустой массив `[]`, все заказы уже выгружены.»
- Fields the plans use: `srid` («`srid` равен `rid` в ответах методов сборочных заданий» — so a row of the feed and an FBS assembly
  order are joined by it), `gNumber` (the buyer's basket), `date`, `lastChangeDate`, `warehouseName`, `warehouseType` («Тип склада
  хранения товаров» — tells a marketplace warehouse from the seller's), `supplierArticle`, `nmId`, `barcode`, `subject`, `brand`,
  `techSize`, `isCancel`, `cancelDate`.
- Fields the plans never store: `countryName`, `oblastOkrugName`, `regionName` (where the buyer is) and the prices (Stage 3 of the
  high-level plan reads money).
- `warehouseType` takes exactly two values in the file: «Склад WB» (the goods lay in a Wildberries warehouse — the scheme the
  high-level plan calls FBO) and «Склад продавца» (the seller's warehouse — FBS and the schemes where the seller delivers).

## 7. Still NOT SOURCED — the checks of the first real account or test token

Carried over from the earlier research; none could be settled from the files:

1. Does `GET /api/v3/orders/new` still list an order the buyer cancelled within the first hour?
2. Does `PUT …/meta/sgtin` replace the codes sent before or add to them?
3. Which `wbStatus` a buyer's cancellation shows after the order is on assembly.
4. Which text the order sticker's code gives to a scanner (the field `barcode` of the sticker's answer is the likely one).
5. Whether a sticker stays valid when its order is moved between two open supplies (the files state a new sticker only for the
   transfer back from delivery).
6. Which supplies are refused with `FulfillmentSupplyBoxesRequired` when they have no transport boxes.
7. Which of the two limits a test token really meets in the sandbox: 150 requests a minute or 1 request a second (section 5).
8. How long the verdict `pending` of a marking code lasts in practice.
9. Whether the sandbox closes a supply without shipping parameters (the shipping methods exist only on the production host).
