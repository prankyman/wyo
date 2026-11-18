# Ozon's official Seller API description file and table of roles, read on 2026-11-18: the methods the Ozon plans need, the key, the limits, and what is still not sourced

> The research check of the Ozon plans of Stage 2 (Plans 5 and 6 of this repository; the high-level plan is
> `.agents/plans/Marketplace_connector_plan.md`). Cited in the plans as **«OZC»**. Every fact below is quoted from Ozon's own
> description file or Ozon's own table of roles, both saved by the maintainer on 2026-11-18; a gap is marked **NOT SOURCED**, and a
> reading of sourced text that is not a quotation is marked **Inference**.

## 1. How this was sourced

1. **Ozon's description file** (the OpenAPI file Ozon publishes for its Seller API), `https://docs.ozon.ru/api/seller/swagger.json`.
   Ozon refuses it to the machine this check ran on, which has an address in the United States: the answer is a redirect
   (`307 Temporary Redirect` to `…/swagger.json?__rr=1`), and following it ends in `403` with the header `ozon-antibot: 1`. **The
   maintainer saved it from a browser on 2026-11-18** as `docs/marketplaces/swagger.json` (Plan 5 moves it to `api/ozon/`):

    | Fact                | Value                                                              |
    | ------------------- | ------------------------------------------------------------------ |
    | Size                | 4 378 503 bytes                                                    |
    | SHA-256             | `57f27f1a08d09f75473601183b8ee2d0f2e3336c084ca295d41d4969a0be8b2f` |
    | `openapi`           | `3.0.0`                                                            |
    | `info.title`        | «Документация Ozon Seller API»                                     |
    | `info.version`      | `2.1`                                                              |
    | Methods (paths)     | 497                                                                |
    | Server              | `//api-seller.ozon.ru` (written without a scheme)                  |

   Cited **[OZ]**. A method's page is `https://docs.ozon.ru/api/seller/#operation/<operationId>`; a section is
   `https://docs.ozon.ru/api/seller/#tag/<name>`.

2. **A copy of the same file kept on GitHub by a third party**, not by Ozon: `https://github.com/vispar-tech/ozon-seller-api-schema`,
   file `schemas/ozon-seller-api-openapi.json`, commit `e22c5e9ee2a0` («chore: update schema (2026-11-18)», 2026-11-18 05:28 UTC; the
   copy is renewed about daily). **Compared with [OZ] after both were read as JSON** (so spacing and key order do not count): the same
   497 methods; the only differences are 9 values inside examples of 5 methods, where [OZ] names the image server `ir.ozone.ru` and the
   copy names `ir-20.ozone.ru` (`/v1/description-category/attribute/values`, `/v1/posting/fbs/package-label/get`, `/v1/report/info`,
   `/v1/report/list`, `/v4/product/info/attributes`). No method, field, description or rule differs. **Inference:** the copy may be
   used to see a later version of Ozon's file, compared the same way; the file the code is generated from stays Ozon's own.

3. **Ozon's table of roles**, «Маппинг ролей и методов Seller API», `https://dev.ozon.ru/start/300-Mapping-rolei-i-metodov-Seller-API/`,
   dated by Ozon 15 August 2024, saved as a PDF by the maintainer on 2026-11-18:
   `docs/marketplaces/Ozon - Маппинг ролей и методов Seller API – Ozon for dev.pdf` (24 pages). Cited **[ROLES]**. Ozon writes on it:
   «Методы иногда устаревают, но мы стараемся постоянно обновлять информацию, приведенную в таблице ниже.»

4. **Not read today:** every other page of `docs.ozon.ru`, `dev.ozon.ru` and `seller-edu.ozon.ru` (refused to this machine as above).
   The Ozon pages saved earlier in `docs/marketplaces/` (the seller contract, «Как собрать и отправить заказы по схеме FBS», «Деление
   отправлений методом /v4/posting/fbs/ship», «Частичная отмена заказа») were not re-read for this check.

## 2. The key

- **Two headers on every method:** `Client-Id` («Идентификатор клиента») and `Api-Key` («API-ключ»), both a `string`, both required
  [OZ, `components.parameters`]. The request format of section «Environment»: `Host: api-seller.ozon.ru`, `Client-Id: <Client-Id>`,
  `Api-Key: <Api-Key>`, `Content-Type: application/json` [OZ, `#tag/Environment`].
- **One environment only:** «Рабочая среда — это ваш магазин. Все отправленные запросы, кроме информационных, могут изменять данные в
  личном кабинете и на сайте Ozon. Рабочая среда: api-seller.ozon.ru» [OZ, `#tag/Environment`]. **Inference:** Ozon offers no test
  environment and no test orders; nothing in [OZ] names one.
- **How a seller makes a key** [OZ, `#tag/Auth`], quoted: «1. В личном кабинете перейдите в **Настройки → Seller API**
  (`https://seller.ozon.ru/app/settings/api-keys`). 2. Нажмите **Сгенерировать ключ**. 3. Придумайте название для ключа и выберите его
  уровень доступа. 4. Выберите цель использования ключа: для личного пользования; для внешнего сервиса, приложения — введите название
  сервиса, для которого выпускается ключ. 5. Нажмите **Сгенерировать**.»
- **Shown once:** «API-ключи хранятся в личном кабинете в скрытом виде. Вы увидите API-ключ только при создании. Сохраните его в
  надёжном месте или сразу укажите в своих системах. Вы не сможете восстановить доступ к несохранённым API-ключам.» [OZ, `#tag/Auth`]
- **Lifetime:** «Срок действия ключа — 3 месяца. После истечения срока действия сгенерируйте новый ключ в личном кабинете.» and
  «Получите дату истечения срока действия ключа в параметре `expires_at` метода /v1/roles.» [OZ, `#tag/Auth`]
- **Revoked by Ozon:** «Ozon может отозвать ключ, если обнаружит подозрительную активность или слишком большую нагрузку на свои
  системы.» **Not to be passed on:** «Не передавайте API-ключ посторонним людям. Запросы к API изменяют данные в вашем магазине на
  Ozon.» **Several keys:** «Вы можете создать несколько API-ключей.» **Networks:** a key may be limited to allowed networks («Если вы
  хотите изменить разрешённые сети для ключа, в списке API-ключей выберите нужный и нажмите [изменить]») [OZ, `#tag/Auth`].
- **The offer:** «Если оферта неактуальна, доступ к Seller API будет ограничен. Вы получите ошибку `403 Offer not signed`. Чтобы
  возобновить доступ, примите оферту.» [OZ, `#tag/Getting started`]. Times are UTC: «Seller API работает по UTC.» (same section).
- **`/v1/roles`** (`AccessAPI_RolesByToken`), «Метод для получения информации и ролях и методах, привязанных к API-ключу», no request
  fields; the answer: `expires_at` («Дата истечения срока действия ключа»), `roles[]` with `name` and `methods[]`. Its example lists
  the role `Posting FBS` with the methods `["/v1/posting"]`. **Inference:** a role's method may be a beginning of paths, not only a
  whole path. The full format of `methods[]`: **NOT SOURCED**.
- **`/v1/seller/info`** (`SellerAPI_SellerInfo`): `company.name` («Название компании на Ozon»), `company.legal_name`, `company.inn`,
  `company.ogrn`, plus ratings and the subscription.
- **Where the cabinet shows the Client ID** — from the page «Seller API» of a seller cabinet (the address in [OZ] quoted above),
  saved as a PDF by the maintainer from a seller cabinet (not a working shop) on 2026-11-18, read, and **deleted**: it shows that
  cabinet's numbers. The menu path is «Настройки» → «API интеграции» → «Seller API» (beside «Управление частными приложениями»,
  «OAuth-доступы», «Уведомления»). At the top of the page is the field «Client ID» with a button to copy it; below it, «API ключи»:
  «Для работы с Ozon Seller API необходимо использование клиентского идентификатора (Client ID) и уникального ключа (API Key)». In a
  cabinet whose offer is not accepted the section says «Нет подписанной оферты. Вы не можете создать API ключ, пока не подпишите
  оферту». The page «Учётная запись» of the same cabinet shows an «Ozon ID» that is a different number from the Client ID.
  **Inference:** the Client ID is a number (one cabinet seen; its format is not documented).
- **NOT SOURCED:** which of the two purposes of step 4 fits a program the seller runs on its own server; the HTTP status of every key
  error but `403 Offer not signed` (section 4).

## 3. Which role opens which method [ROLES]

| Role                         | Ozon's description                                                      | Methods of the Ozon plans the row lists (by the version it names)                                                                                                                                                                                                                                                            |
| ---------------------------- | ----------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Admin`                      | «Позволяет оперировать всеми методами Seller API»                        | all                                                                                                                                                                                                                                                                                                                          |
| `Admin read only`            | «…всеми информационными методами Seller API без внесения изменений»      | `/v3/posting/fbs/get`, `/v3/posting/fbs/list`, `/v3/posting/fbs/unfulfilled/list`, `/v2/posting/fbo/list`, `/v2/product/info/list`, `/v1/warehouse/list`, `/v2/posting/fbs/act/check-status`, `…/act/get-pdf`, `…/act/get-barcode`, `…/act/get-barcode/text`, `…/act/get-postings`, `/v1/posting/fbs/package-label/get`        |
| `Posting FBS`                | «…получать информацию и управлять постингами в схеме FBS»                | the reading ones of `Posting FBS read-only` and `/v4/posting/fbs/ship`, `/v4/posting/fbs/ship/package`, `/v5/fbs/posting/product/exemplar/create-or-get`, `/v5/…/exemplar/set`, `/v4/…/exemplar/status`, `/v4/…/exemplar/validate`, `/v1/posting/fbs/package-label/create`, `/v2/posting/fbs/cancel`, `/v2/posting/fbs/product/cancel` |
| `Posting FBS read-only`      | «…получать информацию по постингам в схеме FBS»                          | `/v3/posting/fbs/get`, `/v3/posting/fbs/list`, `/v3/posting/fbs/unfulfilled/list`, `/v2/posting/fbs/act/check-status`, `…/get-pdf`, `…/get-barcode`, `…/get-barcode/text`, `…/get-postings`, `/v4/…/exemplar/status`                                                                                                        |
| `Posting FBO`                | «…получать информацию о постингах в схеме FBO»                           | `/v2/posting/fbo/get`, `/v2/posting/fbo/list`                                                                                                                                                                                                                                                                                |
| `Product read-only`          | «…получать информацию о товарах»                                         | `/v2/product/info/list`, `/v2/product/info`                                                                                                                                                                                                                                                                                  |
| `Warehouse`                  | «…получить информацию по складам»                                        | `/v1/warehouse/list`, `/v1/delivery-method/list`                                                                                                                                                                                                                                                                             |

**The table is older than the methods the plans call.** It names none of: `/v1/roles`, `/v1/seller/info`, `/v2/warehouse/list`,
`/v4/posting/fbs/unfulfilled/list`, `/v4/posting/fbs/list`, `/v3/posting/fbo/list`, `/v3/product/info/list`, the `/v6` and `/v5`
exemplar methods of section 7, `/v3/posting/fbs/package-label/create`, `/v2/posting/fbs/package-label/get`, `/v2/carriage/delivery/list`,
`/v1/carriage/*`. Which role opens each of them: **NOT SOURCED**. **Inference:** the role that opens the earlier version of the same
method; the key's own `methods[]` from `/v1/roles` is the only list that says it for a given key.

## 4. Limits and errors

- «Если превысите их, вернётся ошибка `429 Too Many Requests`. Чтобы контролировать лимиты, в ответах возвращаются заголовки с
  информацией о текущем состоянии лимитов метода: `Ratelimit-Remaining` — сколько запросов ещё можно выполнить до ошибки `429 Too Many
  Requests`. `Retry-After` — через сколько секунд можно сделать повторный запрос. Возвращается при получении ошибки `429 Too Many
  Requests`. У некоторых методов есть свой лимит на количество запросов — он указан в описании метода. Если лимит не указан в описании
  метода, вы можете сделать не больше 50 запросов в секунду с одного Client ID.» [OZ, `#tag/Limits`]
- «Если от вашего имени поступает много одинаковых или ошибочных запросов, мы можем ограничить доступ к Seller API без
  предупреждения.» [OZ, `#tag/Process`]
- **No method the plans use states a limit of its own** (their descriptions were searched); `/v3/product/info/list` takes «не больше
  1000 товаров по параметрам `offer_id`, `product_id` и `sku` в сумме». The posting lists take a period of at most one year
  (`/v4/posting/fbs/list`, `/v4/posting/fbs/unfulfilled/list`: «он должен быть не больше одного года»; `/v3/posting/fbo/list`: «Если
  период больше года, вернётся ошибка `PERIOD_IS_TOO_LONG`»). The largest `limit` of a page: **NOT SOURCED** (the examples use 100).
- **Errors of every method** [OZ, `#tag/Errors`, «Все методы»]: `Circle is open` («система блокирует работу метода. В течение
  нескольких минут метод начнет работать как обычно»), `Internal error`, `Invalid Api-Key, please check the key and try again`,
  `Api-key is deactivated, use another one or generate a new one`, `Api-Key is missing a required role for a method`, `Api-Key is
  restricted to specific IP addresses`, `You have reached request rate limit per second`, `method is not allowed`. The body of an error
  is `rpcStatus`: `code` (a number), `message`, `details[]`. **The HTTP status of each of these words: NOT SOURCED.**

## 5. Every method the plans use

All are `POST` on `https://api-seller.ozon.ru`. None is marked `deprecated` in [OZ].

| Purpose                                              | Method                                              | `operationId`                                         | Plan |
| ---------------------------------------------------- | --------------------------------------------------- | ----------------------------------------------------- | ---- |
| The key's end date, roles and methods                | `/v1/roles`                                         | `AccessAPI_RolesByToken`                              | 5    |
| The seller's name                                    | `/v1/seller/info`                                   | `SellerAPI_SellerInfo`                                | 5    |
| The seller's FBS and rFBS warehouses                 | `/v2/warehouse/list`                                | `WarehouseListV2`                                     | 5    |
| Unprocessed FBS orders                               | `/v4/posting/fbs/unfulfilled/list`                  | `PostingFbsUnfulfilledList`                           | 5    |
| FBS orders of a period (not called; read for §6)     | `/v4/posting/fbs/list`                              | `PostingFbsList`                                      | —    |
| One FBS order                                        | `/v3/posting/fbs/get`                               | `PostingAPI_GetFbsPostingV3`                          | 5    |
| FBO orders of a period                               | `/v3/posting/fbo/list`                              | `PostingFboList`                                      | 5    |
| Products by SKU: barcodes, name                      | `/v3/product/info/list`                             | `ProductAPI_GetProductInfoList`                       | 5    |
| A posting's exemplars and what each product needs    | `/v6/fbs/posting/product/exemplar/create-or-get`    | `PostingAPI_FbsPostingProductExemplarCreateOrGetV6`   | 6    |
| Send the marking codes                               | `/v6/fbs/posting/product/exemplar/set`              | `PostingAPI_FbsPostingProductExemplarSetV6`           | 6    |
| Ozon's verdict on the codes                          | `/v5/fbs/posting/product/exemplar/status`           | `PostingAPI_FbsPostingProductExemplarStatusV5`        | 6    |
| Report the packing, split into parcels               | `/v4/posting/fbs/ship`                              | `PostingAPI_ShipFbsPostingV4`                         | 6    |
| Ask for a label                                      | `/v3/posting/fbs/package-label/create`              | `PostingFbsPackageLabelCreate`                        | 6    |
| Fetch the label                                      | `/v2/posting/fbs/package-label/get`                 | `PostingFbsPackageLabelGet`                           | 6    |
| The day's delivery methods and shipments             | `/v2/carriage/delivery/list`                        | `CarriageAPI_CarriageDeliveryListV2`                  | 6    |
| Create the shipment                                  | `/v1/carriage/create`                               | `CarriageAPI_CarriageCreate`                          | 6    |
| Set exactly its postings                             | `/v1/carriage/set-postings`                         | `CarriageAPI_SetPostings`                             | 6    |
| Approve it                                           | `/v1/carriage/approve`                              | `CarriageAPI_CarriageApprove`                         | 6    |
| Read it                                              | `/v1/carriage/get`                                  | `CarriageGet`                                         | 6    |
| Cancel it (a withdrawn hand-over, before approval)   | `/v1/carriage/cancel`                               | `CarriageAPI_CarriageCancel`                          | 6    |
| Are the documents ready; what is in the shipment     | `/v2/posting/fbs/act/check-status`                  | `PostingAPI_PostingFBSActCheckStatus`                 | 6    |
| The shipment's postings                              | `/v2/posting/fbs/act/get-postings`                  | `PostingAPI_ActPostingList`                           | 6    |
| The shipping list and the waybill                    | `/v2/posting/fbs/act/get-pdf`                       | `PostingAPI_PostingFBSGetAct`                         | 6    |
| The hand-over barcode, as a picture and as text      | `/v2/posting/fbs/act/get-barcode`, `…/text`         | `PostingAPI_PostingFBSGetBarcode`, `…GetBarcodeText`  | 6    |

**Never called:** `/v2/posting/fbs/cancel` (`PostingAPI_CancelFbsPosting`) and `/v2/posting/fbs/product/cancel`
(`PostingAPI_CancelFbsPostingProduct`) — a cancellation by the seller (high-level plan, the exchange's step 5).
**Marked `deprecated` in [OZ] and not used:** `/v3/posting/fbs/unfulfilled/list`, `/v3/posting/fbs/list`, `/v2/posting/fbo/list`. The
old label methods were switched off on 2 November 2026 (section «Обновления», 22 September 2026).

**The file's own types outside OpenAPI's list** (a generator may refuse them): `int` (`v2PostingFBSActGetPostingsRequest.id` — the
request of `/v2/posting/fbs/act/get-postings` — and three fields of `v2PostingFBSActListResult`), `array of strings`
(`v3PostingProductDetail.jw_uin` — inside the answer of `/v3/posting/fbs/get` — and two fields of
`productv4GetProductAttributesV4ResponseResult`), `bool`, `timestamp`, `enum` (in methods the plans do not use).

## 6. Orders — Ozon's «отправления» (postings)

- **Statuses** (`/v3/posting/fbs/get`, `result.status`): «`acceptance_in_progress` — идёт приёмка, `arbitration` — арбитраж,
  `awaiting_approve` — ожидает подтверждения, `awaiting_deliver` — ожидает отгрузки, `awaiting_packaging` — ожидает упаковки,
  `awaiting_registration` — ожидает регистрации, `awaiting_verification` — создано, `cancelled` — отменено,
  `cancelled_from_split_pending` — отменён из-за разделения отправления, `client_arbitration` — клиентский арбитраж доставки,
  `delivered` — доставлено, `delivering` — доставляется, `driver_pickup` — у водителя, `not_accepted` — не принят на сортировочном
  центре». Substatuses include `posting_split_pending`, `posting_canceled` and — the one that matters after packing — `ship_failed`
  (section 7). FBO statuses (`/v3/posting/fbo/list`): `awaiting_packaging`, `awaiting_deliver`, `delivering`, `delivered`, `cancelled`.
- **A posting's fields the plans read:** `posting_number` («Номер отправления»), `order_number` («Номер заказа, к которому относится
  отправление» — several postings may share it), `status`, `substatus`, `in_process_at`, `shipment_date` («Дата и время, до которой
  нужно собрать отправление. Показываем рекомендованное время отгрузки. По истечении этого времени начнёт применяться новый тариф»),
  `shipment_date_without_delay`, `delivery_method.warehouse_id`, `parent_posting_number` («Номер родительского отправления, в
  результате разделения которого появилось текущее»), `cancellation{cancel_reason, cancellation_initiator, cancellation_type
  (seller, client or customer, ozon, system, delivery), cancelled_after_ship}`, `products[]{sku, offer_id («артикул»), name,
  quantity}`, `requirements{products_requiring_mandatory_mark, …_gtd, …_rnpt, …_jw_uin, …_imei, …_weight, …_country,
  …_change_country}`, `is_multibox`, `tpl_integration_type`, `available_actions[]`. «Чтобы получать актуальную дату отгрузки,
  регулярно обновляйте информацию об отправлениях» (`/v4/posting/fbs/list`, `/v3/posting/fbs/get`).
- **Buyer data in every answer of the posting lists** (whatever `with` asks): `customer{name, phone, customer_email, customer_id,
  address{address_tail, city, region, zip_code, comment, latitude, longitude, …}}`, `addressee{name}`; with `with.analytics_data`:
  `city`, `region`; with `with.legal_info`: `company_name`, `inn`, `kpp`. The plans drop all of them at the adapter's edge.
- **The filters:** `/v4/posting/fbs/unfulfilled/list` — «Используйте фильтр по времени сборки — `cutoff` или по дате передачи
  отправления в доставку — `delivering_date`. Если использовать их вместе, в ответе вернётся ошибка.», `statuses[]`,
  `warehouse_ids[]`, `cursor` with `has_next`. `/v4/posting/fbs/list` — `since`, `to`, `last_changed_status_date{from, to}`,
  `statuses[]`, `warehouse_ids[]`. `/v3/posting/fbo/list` — `since`, `to`, `statuses[]`. **What `since` and `to` of the two lists are
  compared with (the creation or the last change): NOT SOURCED.**
- **Warehouses** (`/v2/warehouse/list`, «Метод возвращает список складов FBS и rFBS»): `warehouse_id`, `name`, `status`, `is_rfbs`
  («Признак работы склада по схеме rFBS»), `carriage_label_type` (`UNSPECIFIED`, `BIG` — «большая этикетка», `SMALL` — «маленькая
  этикетка»), `is_auto_assembly` («Признак включённой автосборки»), `has_entrusted_acceptance`, `first_mile.type` (`PICK_UP`,
  `DROP_OFF`), `cursor`, `has_next`.
- **Three numbers of a product:** `offer_id` — the seller's article; `product_id` — Ozon's product number; `sku` — «Идентификатор
  товара в системе Ozon — SKU». **In `/v4/posting/fbs/ship` and the exemplar methods the field named `product_id` holds the SKU**
  («`product_id` … Идентификатор товара в системе Ozon — SKU»; the error `UNKNOW_PRODUCT`: «Проверьте, что в поле `product_id` вы
  указали идентификатор товара в системе Ozon — SKU»). A product's barcodes: `/v3/product/info/list`, `items[].barcodes` («Все
  штрихкоды товара»).

## 7. Packing, splitting, marking codes

- **`/v4/posting/fbs/ship`**: «Ответ с кодом 200 не гарантирует успешную сборку заказа. Используйте метод /v3/posting/fbs/get, чтобы
  проверить, что заказ собран. Если в ответе указан result.substatus = ship_failed, повторите сборку заказа.» «Делит заказ на
  отправления и переводит его в статус `awaiting_deliver`. … Чтобы разделить заказ, передайте в массиве `packages` несколько
  объектов.» The request: `posting_number`, `packages[]{products[]{product_id (SKU), quantity}}`, `with.additional_data`; the answer:
  `result`, and with `additional_data` the list `additional_data[]{posting_number, products[]{sku, quantity, …}}`. Its errors
  [OZ, `#tag/Errors`]: `POSTING_NOT_FOUND`, `POSTING_ALREADY_CANCELLED`, `POSTING_ALREADY_SHIPPED` («Заказ уже собран»),
  `HAS_INCORRECT_STATUS`, `HAS_INCORRECT_PRODUCT_QUANTITY`, `UNKNOW_PRODUCT`/`UNKNOWN_PRODUCT_DEFINED`, `EXEMPLAR_INFO_ALREADY_DEFINED`,
  `MANDATORY_MARK_REDUNDANT` («Для товара не нужно передавать код маркировки»), `EXEMPLAR_INFO_NOT_FILLED_COMPLETELY`,
  `INCORRECT_OVH_FOR_POSTING`. **Which number each part of a split gets, and whether the first part keeps the original's number: NOT
  SOURCED** (the answer's `additional_data` names them).
- **`/v6/fbs/posting/product/exemplar/create-or-get`**: «Используйте метод для получения `exemplar_id`.» Per product: `product_id`
  (SKU), `quantity`, `exemplars[]{exemplar_id, …}`, `is_mandatory_mark_needed` («необходимо передать маркировку «Честный ЗНАК»»),
  `is_mandatory_mark_possible` («возможно заполнить»), `is_gtd_needed`, `is_rnpt_needed`, `is_jw_uin_needed`, `has_imei`,
  `is_weight_needed`; per posting `multi_box_qty`.
- **`/v6/fbs/posting/product/exemplar/set`**: «Асинхронный метод … Всегда передавайте полный набор данных по экземплярам и продуктам.
  … Код ответа 200 не гарантирует, что данные об экземплярах приняты. Он указывает, что создана задача для добавления информации.» A
  mark: `mark`, `mark_type` — `mandatory_mark`, `jw_uin`, `imei`, `seal_number` («номер пломбы», added 17 November 2026, section
  «Обновления»). The schema marks no field as required. Its errors: `EXEMPLAR_ID does not belong to product PRODUCT_ID`, `Posting must
  have 'awaiting_packaging' status` («Для передачи данных или проверки статуса отправление должно быть в статусе «Ожидает сборки»»).
- **The separator of a marking code** [OZ, `#tag/FBS&rFBSMarks`]: «Коды маркировки товара в системе «Честный ЗНАК» передаются с
  криптохвостом. … Символ <GS> должен кодироваться как последовательность `\u001d`. Двойное экранирование в виде `\\u001d` не
  допускается.»
- **`/v5/fbs/posting/product/exemplar/status`**: `status` — «`ship_available` — сборка доступна; `ship_not_available` — сборка
  недоступна; `validation_in_process` — экземпляры на проверке; `update_available`; `update_not_available`»; per mark `check_status` —
  «`processing` — на проверке; `failed` — система не успела обработать запрос; `passed` — заказ готов к сборке», `error_codes[]`. **The
  words `error_codes` may hold: NOT SOURCED.**
- **`/v1/fbs/posting/product/exemplar/update`**: «чтобы сохранить обновлённые данные по экземплярам для отправлений в статусе «Ожидает
  отгрузки»». **How it agrees with `set`'s error «Posting must have 'awaiting_packaging' status»: NOT SOURCED.**

## 8. Labels

- **`/v3/posting/fbs/package-label/create`**: «Создаёт задания на асинхронное формирование этикеток для отправлений в статусе
  «Ожидает отгрузки» — `awaiting_deliver`. Рекомендуем запрашивать этикетки через 45–60 секунд после сборки заказа.» The answer:
  `tasks[]{task_id, task_type}`, `task_type` «`big_label` — для обычной этикетки; `small_label` — для маленькой этикетки»; the
  example answers with both tasks.
- **`/v2/posting/fbs/package-label/get`**: `status.code` — «`pending` — задание в очереди; `in_progress` — формируются; `completed` —
  файл с этикетками готов; `error` — ошибка при создании файла», `file_url` («Ссылка на файл с этикетками»), `error{code, message}`,
  `status.unprinted_postings[]{posting_number, message}`. The example link:
  `https://ir.ozone.ru/s3/ord-tmp-12/small_label/ticket-00-0000-0000.pdf`.
- **NOT SOURCED:** the size of each label in millimetres; the host of a real `file_url`; whether fetching it needs the key; which of
  the posting's two barcodes (`barcodes.upper_barcode`, `barcodes.lower_barcode` — «Верхний / Нижний штрихкод на маркировке
  отправления») a scanner reads at the hand-over. **Inference:** the file is a PDF (the example link ends in `.pdf`).

## 9. The hand-over — Ozon's «отгрузка» (shipment)

- **`/v2/carriage/delivery/list`**: «Метод не возвращает информацию по методам доставки, у которых нет отправлений.»
  `filter.delivery_method_id` — «Для FBS-складов используйте значение параметра `warehouse_id`»; `filter.departure_date` — «По
  умолчанию — текущая дата». Per method: `carriages[]{id («Если `0` — отгрузка, которую можно создать»), status, postings_count,
  available_actions («`create` — можно создать отгрузку; `get_details`; `get_assembly_list`»)}`, `cutoff_at`, `mandatory_postings_count`,
  `mandatory_packaged_count`, `has_entrusted_acceptance`, `errors[]{code, description, status}`.
- **`/v1/carriage/create`**: «Перед началом работы проверьте возможность создания отгрузки методом /v2/carriage/delivery/list. Если
  methods.carriages.available_actions = create, вы можете создать отгрузку. … Используйте метод для создания первой FBS отгрузки. В неё
  попадут все отправления со статусом «Готов к отгрузке». Созданная отгрузка получит статус `new`. Для отгрузки в статусе `new` можно
  перезаписать состав отправлений методом /v1/carriage/set-postings. Если из отгрузки исключить часть отправлений, они могут попасть в
  следующую отгрузку.» The request: `delivery_method_id`, `departure_date`; the answer: `carriage_id`. Its error `POSTING_NOT_FOUND`:
  «Нет новых отправлений для добавления в отгрузку. Подтвердите предыдущую отгрузку методом v1/carriage/approve или добавьте новые
  отправления в существующую отгрузку методом /v1/carriage/set-postings.»
- **`/v1/carriage/set-postings`**: «Метод недоступен для продавцов из СНГ. Полностью перезаписывает список заказов в отгрузке.
  Передавайте только те заказы, которые находятся в статусе Ожидает отгрузки, и вы готовы их отгрузить. Менять состав можно только у
  отгрузок со статусом `new`. … Чтобы вернуться к списку заказов, удалите отгрузку с помощью метода /v1/carriage/cancel, и создайте
  новую.» The answer: `result[]{posting_number, result (true when done), error}`.
- **`/v1/carriage/approve`**: «Используйте метод, чтобы подтвердить отгрузку после её создания. После подтверждения отгрузка перейдёт
  в статус «Сформирована». После подтверждения отгрузки вы можете получить лист отгрузки методом /v2/posting/fbs/act/get-pdf и штрихкод
  отгрузки методом /v2/posting/fbs/act/get-barcode.» `containers_count` — «Используйте параметр, если вы подключены к доверительной
  приёмке и отгружаете заказы грузовыми местами. Если вы не подключены к доверительной приёмке, пропустите его.»
- **`/v1/carriage/get`**: `status` — «`received` — идёт приёмка, `closed` — завершена после приёмки, `sended` — отправлена,
  `cancelled` — отменена»; `cancel_availability{is_cancel_available, reason}`; `available_actions` (`get_shipping_list`,
  `get_act_of_acceptance`, `get_waybill`, …). **`/v1/carriage/cancel`**: the answer `carriage_status`, `error`.
- **`/v2/posting/fbs/act/check-status`** (its `id` is «также идентификатор перевозки»): `result.status` — «`in_process` — документы
  формируются, нужно подождать. `ready` — документы сформированы и готовы для скачивания. `error` — … запросите документы повторно.
  `cancelled` — … запросите их повторно. `The next postings aren't ready` — … Подождите некоторое время и проверьте результат
  запроса.»; `added_to_act[]` («Эти отправления нужно передать сегодня»), `removed_from_act[]` («Такие отправления нужно передавать со
  следующей отгрузкой»), `has_postings_for_next_carriage`, `is_partial`.
- **`/v2/posting/fbs/act/get-pdf`**: «продавцам из России — лист отгрузки и транспортную накладную; продавцам из СНГ — акт и
  транспортную накладную.» **`/v2/posting/fbs/act/get-barcode`**: «Метод для получения штрихкода, который нужно показать в пункте
  выдачи или сортировочном центре при отгрузке отправления.»; **`…/get-barcode/text`**: `result` — «Штрихкод в текстовом виде». The
  successful answers of `get-pdf` and `get-barcode` are declared as `application/pdf` and `image/png`, each with a schema of
  `file_content`, `file_name`, `content_type`. **Whether the body is the file itself or that object: NOT SOURCED.**
- **The conditions of forming** listed for the old method `/v2/posting/fbs/act/create` (`new_postings_are_possible` — «Используйте
  метод, когда закончится сегодняшнее время на сборку заказов», `package_time_not_passed`, `not_packaged`, `no_postings`, …): **whether
  they hold for `/v1/carriage/create`: NOT SOURCED.** Whether a second shipment can be created on the same day for the same warehouse:
  **NOT SOURCED** beyond the quotations above.

## 10. What the documents do not settle — the checks on the first real Ozon account

1. The full format of `/v1/roles`' `methods[]`, and which role opens each method of section 3's list.
2. The HTTP status of each key error of section 4.
3. ~~Where the cabinet shows the Client ID~~ — settled on 2026-11-18 by the maintainer's saved cabinet page (section 2).
4. What `since` and `to` of `/v4/posting/fbs/list` and `/v3/posting/fbo/list` are compared with.
5. The numbers of a split's parts (section 7), and how the original posting shows afterwards.
6. The words of `exemplar/status`' `error_codes`; whether codes can be changed after packing.
7. A label's size, the host of its link, whether the link needs the key, and which barcode a scanner reads.
8. The body of `act/get-pdf` and `act/get-barcode`; when a shipment can be created; a second shipment on the same day.
9. What `is_auto_assembly` (automatic assembly) does to a posting that the fulfilment centre still packs.
