# Stock at Wildberries, Ozon and Яндекс Маркет: what the official documents say (read 2026-11-21)

> Research for the three stock plans of Stage 2 (`.agents/plans/Marketplace_connector_plan.md`): showing what lies in each
> marketplace's own warehouses and in the seller's warehouses at it, and writing a number to a seller's warehouse. Cited «STK» with the
> part's letter and section: «STK W§1.1» is Wildberries, «STK O§1.3» Ozon, «STK Y§1.4» Яндекс Маркет. Nothing was sent to any
> marketplace's API: only description files, documentation pages and help articles were read. Copies downloaded for the comparisons
> were kept only for the day and are not in the repository. «Not documented» means the files and pages read do not say it; each such
> gap is listed at the end of its part and becomes a check on the first real account.

Earlier research this builds on: `.agents/research/wildberries-official-description-files-check-2026-11-17.md`,
`.agents/research/wildberries-token-check-2026-11-18.md`, `.agents/research/ozon-official-seller-api-check-2026-11-18.md`,
`.agents/research/yandex-market-official-partner-api-check-2026-11-19.md`.

## Part W — Wildberries
Scope: (a) showing what lies in Wildberries' own warehouses, (b) sending the fulfilment centre's free stock to the seller's own
warehouses on Wildberries. Sources: Wildberries' own description files (committed copies of 2026-11-17 in
`/home/igor/projects/wyo/api/wildberries/`, cited by file and line) and one official help-centre page. Every fact carries its source.
"Not documented" = the files and the page do not say it. Nothing was checked against a real account; no method was called.

Short names: ITEMS = `02-items.yaml`, GEN = `01-general.yaml`, AN = `11-analytics.yaml`, REP = `12-reports.yaml`,
FBS = `03-orders-fbs.yaml` (all in `/home/igor/projects/wyo/api/wildberries/`; line numbers are of the committed copies, which equal the
live files for everything below except FBS, see 6).
HELP = Wildberries help-centre page «Управление остатками по модели FBS» (shows «Обновлено 27.08.2026»),
`https://seller.wildberries.ru/instructions/ru/ru/material/stocks-management`, read 2026-11-21. It was reached from the address
printed in ITEMS line 219 (`.../instructions/material/A-123?...`, which redirects there). The article text sits in the page's
embedded data, not in its visible markup; extracted copy: (a copy kept for the day).

Existing research this builds on (not repeated): `.agents/research/wildberries-official-description-files-check-2026-11-17.md`,
`.agents/research/wildberries-token-check-2026-11-18.md` (token fields, bit masks, the unexplained `x-readonly-method` flag).

---

### 1. Writing stock to a seller warehouse

All three methods: host `https://marketplace-api.wildberries.ru` (sandbox `https://marketplace-api-sandbox.wildberries.ru`), path
`/api/v3/stocks/{warehouseId}` (ITEMS 6198-6205). `warehouseId` = the seller warehouse's own id, integer int64 (ITEMS 8667-8675).
Token category «Маркетплейс» (`x-category: marketplace`; tag text ITEMS 203-212; category table in GEN 177, row «Маркетплейс»). No
`x-token-types` block on these methods; GEN says «Если блока с ключом в методе нет, вы можете использовать его с любым типом токена»
(GEN 177, inside the long `x-description`) - so personal, service, base and test tokens qualify (test token only in the sandbox).

#### 1.1 Update stock: `PUT /api/v3/stocks/{warehouseId}` (`putV3StocksWarehouseId`, ITEMS 6206-6346)
- Body: `{"stocks":[{"chrtId":<uint64>,"amount":<int>}, ...]}`; `stocks` required, `minItems 1`, **`maxItems 1000`** (ITEMS 6240-6266);
  `chrtId` and `amount` both required per item.
- **What identifies the product: `chrtId`** («ID размера товара», one per size of a Wildberries article). Not a barcode. Upload by
  barcode is refused: 400 `SKUUploadDisabled` «Uploading stock is not allowed by 'sku'. Please use the 'chrtId' key» (ITEMS
  9262-9266, used at 6283). Since when: **not documented** (the dated release notes answer HTTP 498 to scripts, see 7).
- **`amount`: `maximum 100000`; no minimum is written** (ITEMS 6262-6266). Whether 0 and negative numbers are accepted: not documented
  in the description. HELP says only that the portal's XLSX upload accepts counts above 0, while manual entry has no such limit
  (HELP, «Ограничения»). A number above 100000 gives 409 `UploadDataLimit` «Upload limit exceeded», with the offending item (ITEMS 9287-9295).
- **Trap**: «Названия параметров запроса не валидируются. При отправке некорректных названий вы получите успешный ответ (204), но
  остатки не обновятся» (ITEMS 6219-6221). A wrongly spelled field still returns 204.
- Success: **204**, no body (ITEMS 6279-6280).
- **Rate limit** (ITEMS 6223-6234): the same pool for all stock-on-seller-warehouse methods except delete: **1 minute | 300 requests |
  interval 200 ms | burst 20**; «Один запрос с кодами ответов 4XX учитывается как 10 запросов». Sandbox: at most 1 request per second summed over
  all Marketplace methods.
  - GEN gives, for the whole Marketplace category, by token type: personal and service 300/min, **base and test 150/min, interval
    200 ms, burst 10** (GEN 124-133). The stock method page names no token type. Which wins for a base token: **not documented**
    (inference: the base-token figure).
  - Headers (GEN 119-165): the algorithm is a token bucket. `X-Ratelimit-Remaining` = how many requests may be made right now without
    a pause (present on every status except 429); on **429** also `X-Ratelimit-Retry` (seconds until the next request is accepted),
    `X-Ratelimit-Limit` (the burst maximum) and `X-Ratelimit-Reset` (seconds until the burst is back to the maximum). A 4XX drops
    `X-Ratelimit-Remaining` by 10 at once (GEN 154).
- **Errors** (ITEMS 6281-6346; general table GEN 91-95; shared bodies ITEMS 8841-8960 and 10112-10141):
  | Code | Body | Meaning (Wildberries' text) |
  | --- | --- | --- |
  | 400 | `{code,message}`: `IncorrectRequestBody`, `IncorrectRequest`, `IncorrectParameter`, `SKUUploadDisabled` | malformed request; upload by barcode refused |
  | 401 | problem+json (title, detail, requestId, ...) | token missing, expired or invalid (GEN 90) |
  | 402 | problem+json | only for services of the business-solutions catalogue: balance too low (ITEMS 8841) |
  | 403 | `{code:"AccessDenied",message}` or problem+json `scope is not allowed for this resource` | wrong token category («Response403TokenCategory», «Неверная категория токена») or no access |
  | 404 | `{code:"NotFound",message:"Not found"}` | warehouse not found |
  | 406 | `{code,message}`: `StocksUpdateBlock` «Stocks update block due supplier ban»; `WarehouseStocksUpdateBlock` «The warehouse is processing» | update blocked because the seller is banned; warehouse under technical work, «Повторите запрос позже» (ITEMS 6306-6316, 9277-9286) |
  | 409 | **array** of `{data:[{sku:"",chrtId,amount}], code, message}`; `sku` is «Не используется» (ITEMS 6317-6343, 7452-7487) | codes below |
  | 429 | problem+json | too many requests |
  409 codes (ITEMS 9287-9385): `UploadDataLimit` (amount above 100000); `CargoWarehouseRestrictionMGT` (warehouse not for small goods; use
  a warehouse with `cargoType` 1); `CargoWarehouseRestrictionSGT` (not for oversize goods; needs `cargoType` 2);
  `CargoWarehouseRestrictionSGTKGTPlus` (needs `cargoType` 2 or 3); `CargoWarehouseRestrictionKGTPlus` (needs `cargoType` 3);
  `NotFound` (the item's chrtId is not found, listed in `data`); `StoreIsProcessing` «The store is processing» (warehouse is being
  updated or deleted, «Повторите попытку через несколько секунд»); `DeliveryTypeRestriction` (the product's subject cannot be sold with
  this warehouse's delivery type); `SubjectRestriction` (the subject cannot be sold from this warehouse); `TnvedRequired` («Loading stocks
  is not possible. Add HS code to the product card» - the card lacks its customs commodity code). The `data` array lists the offending
  items. Whether one bad item rejects the whole batch or only itself: **not documented**.
- A new or edited card cannot take stock for up to 30 minutes: «Синхронизация данных с сервисами может занимать до 30 минут. В течение
  этого времени невозможно добавить остатки на склады и настроить цены» (ITEMS 3066 for card edit; 4256 and 4520 for card creation).
- Read-only token (bit 30 of `s`, see the token research): `PUT` carries `x-readonly-method: false` (ITEMS 6209), the stock read
  `POST` carries `true` (ITEMS 6450). The flag is not explained anywhere in the files (token research, section 1 «Read-only»). That a
  read-only token is refused on `PUT`/`DELETE` and accepted on the read `POST` is an inference, **not a documented sentence**.

#### 1.2 Delete stock: `DELETE /api/v3/stocks/{warehouseId}` (`deleteV3StocksWarehouseId`, ITEMS 6347-6447)
- Body `{"chrtIds":[...]}`, required, `minItems 1`, **`maxItems 1000`** (ITEMS 6384-6396). Removes the stock *record*.
- «Действие необратимо. Удаленный остаток будет необходимо загрузить повторно для возобновления продаж» (ITEMS 6360). So delete is not
  "send 0": the description does not say what 0 does; it says deletion needs a re-upload to resume sales.
- **Its own, much tighter limit: 1 minute | 10 requests | interval 6 s | burst 2** (ITEMS 6362-6372, table line 6368); 4XX counts as 10.
  Success 204.
- Errors: 400 (same four codes, `SKUUploadDisabled` included), 401, 402, 403, 404, **409 `StoreIsProcessing`** (a single object here, not
  an array; ITEMS 6436-6443, 9351-9355), 429.

#### 1.3 Read stock of a seller warehouse: `POST /api/v3/stocks/{warehouseId}` (`postV3StocksWarehouseId`, ITEMS 6448-6553)
- Body `{"chrtIds":[...]}`, required; `maxItems 1000`, **no `minItems`** (ITEMS 6477-6490). The body must name the chrtIds; the method has no
  "list everything" form. How to read a warehouse's whole stock with it: **not documented**.
- 200: `{"stocks":[{"chrtId":uint64,"amount":int}]}` (ITEMS 6495-6520). Whether chrtIds without a record are omitted or come back as 0:
  **not documented**.
- Limit: the same pool as the update (300/min, 200 ms, burst 20; 4XX = 10; ITEMS 6465).
- Errors: 400 (also `SKUUploadDisabled`, «Получение остатков по баркодам невозможно»), 401, 402, 403, 404, 429; no 409.
- Another read of the same thing: AN `POST /api/analytics/v1/stocks-report/seller-warehouses` returns stock of the seller's
  warehouses by size (see 4.1); it needs a personal or service token, not a base token.

---

### 2. Which warehouses take stock

`GET /api/v3/warehouses` (`getV3Warehouses`, ITEMS 6621-6694): host `marketplace-api.wildberries.ru`, category «Маркетплейс», no
parameters, 200 = array of seller warehouses; «Метод возвращает список всех складов продавца. Может использоваться для работы с
остатками товаров». Limit **1 minute | 300 requests | 200 ms | burst 20** for all warehouse methods; 4XX = 10 (ITEMS 6646).
Fields of one warehouse (schema `Warehouse`, ITEMS 8039-8110):
- `name`; `officeId` (int64, the id of the Wildberries warehouse it is tied to); `id` (int64, the seller warehouse id used in the stock path);
- `cargoType` (ITEMS 8060): 1 = small goods (МГТ), 2 = oversize (СГТ), 3 = large (КГТ+);
- `deliveryType` (ITEMS 8072): **1 = delivery to a Wildberries warehouse (FBS), 2 = delivery by the seller (DBS), 3 = Деливери WB
  (DBW), 5 = self-pickup (C&C), 6 = express delivery by the seller (EDBS)**;
- `isDeleting` (ITEMS 8088): «Склад удаляется ... После удаления склад пропадёт из списка»;
- `isProcessing` (ITEMS 8098): «Данные склада обновляются ... `true` — да, обновление и удаление остатков недоступно. Обновление данных
  может занимать несколько минут». While it is `true` the stock methods answer 409 `StoreIsProcessing` or 406 `WarehouseStocksUpdateBlock`.
- **Which kinds of warehouse the stock method accepts**: the tag text says the stock methods are for «модели продаж со склада
  продавца» (ITEMS 222); the stock method lists `DeliveryTypeRestriction` and the three `cargoType` restrictions among its 409 codes.
  **No sentence lists the accepted delivery types.** The type list has FBS, DBS, DBW, C&C and EDBS; HELP's sidebar has stock-management
  pages for FBS, C&C, and DBS/EDBS. Whether DBW (Деливери WB) stock goes through the same method: not documented.
- Creating a seller warehouse `POST /api/v3/warehouses` (ITEMS 6697-6788): body `name` (1-200 characters) and `officeId` (≥ 1, a
  Wildberries warehouse that is not already used); creates **FBS** warehouses «кроме сверхгабаритных (СГТ)»; 201 → `{id}`.
  `PUT /api/v3/warehouses/{id}` changes `name` and `officeId` («Можно менять не чаще одного раза в сутки»), not for oversize
  (`cargoType` 2) warehouses. `DELETE /api/v3/warehouses/{id}` → 204. `GET /api/v3/offices` (ITEMS 6554) lists Wildberries warehouses
  to tie to. All share the 300/min pool of the warehouse methods.
- A warehouse that was deleted or whose type changed: only `isDeleting`, `isProcessing` and the 404 on the stock path are documented.
  What happens to its stock records, and whether `deliveryType`/`cargoType` can change after creation (the update takes only `name`
  and `officeId`): **not documented**.
- An FBS order also names both: `warehouseId` (seller warehouse) and `officeId` (Wildberries warehouse) (FBS 6030-6037).

---

### 3. How a product is matched

`POST /content/v2/get/cards/list` (`postV2GetCardsList`, ITEMS 2249-2860): host **`content-api.wildberries.ru`** (sandbox
`content-api-sandbox.wildberries.ru`), category **«Контент»** (`x-category: content`; GEN category table, row «Контент») - a different
category from the stock methods (Маркетплейс). Cards in the trash are not returned (ITEMS 2262-2264).
- Per card (ITEMS 2605-2720): `nmID` (Wildberries article), `imtID` (group id of merged cards), `vendorCode` (**the seller's article**),
  `brand`, `title`, `subjectID`, `subjectName`, `createdAt`, `updatedAt` and **`sizes[]`**, each `{chrtID (uint64, «Числовой ID размера для
  данного артикула WB»), techSize, wbSize, skus[] (array of barcodes)}`. A product without sizes has one size with `techSize "0"` (AN, text of
  the sizes method). So **chrtID + skus (barcodes) + vendorCode + nmID all come from this one method.** Spelling: `chrtID` here, `chrtId` in stock methods.
- Paging (ITEMS 2262-2290, 2417-2432): `settings.cursor.limit` **max 100** (default 10); copy `updatedAt` and `nmID` from the reply's
  `cursor` into the next request; repeat until `cursor.total` < `limit`. `settings.sort.ascending` sorts by `updatedAt`. For «только карточки,
  созданные или обновлённые после предыдущей выгрузки»: sort ascending and keep the last cursor. Filters: `textSearch` («Поиск по
  артикулу продавца, артикулу WB, баркоду»; whether it is an exact match: **not documented**), `withPhoto`, `brands`, `objectIDs`, `tagIDs`, `imtID`.
- Limit (ITEMS 2314): **1 minute | 80 requests | interval 750 ms | burst 5**; sandbox 1 request per second.
- Another documented way to get a chrtId for a barcode: **no method is named for it**. What exists: (a) the `textSearch` filter above
  (barcode in, card with `sizes[].chrtID` out); (b) an FBS order carries `chrtId`, `skus`, `article`, `nmId` together (FBS 5655-5691,
  6016-6045) - only for goods that were ordered; (c) the analytics stock reports return `chrtId` with `nmId` but no barcode (AN 6010-6050);
  (d) `GET /api/v1/warehouse_remains` returns `barcode`, `vendorCode`, `nmId`, `techSize` but no chrtId (REP 711-790).
- `skus` is an array: several barcodes per size are possible. Barcodes can be Wildberries-generated (`POST /content/v2/barcodes`,
  up to 5000 per call, ITEMS 4141-4240) or the seller's own; both land in `skus`.

---

### 4. Stock in Wildberries' own warehouses (read-only display)

#### 4.1 The «История остатков» (stocks history) methods, host `https://seller-analytics-api.wildberries.ru`
Tag text (AN 94-118): the data comes from the portal's detailed product table and the region widget; «Остатки в ответах данных
методов — на текущий день». Token category «Аналитика» (AN 96-98). Stock by day for up to 3 months comes from the CSV reports of
type `STOCK_HISTORY_DAILY_CSV` (AN 112-114). No deprecation note in AN for any of these.
1. **`POST /api/analytics/v1/stocks-report/wb-warehouses`** (`postV1StocksReportWbWarehouses`, AN 1443-1525): «возвращает текущие остатки
   товаров на складах WB»; **«Данные обновляются 1 раз в 30 минут»**; one row = one size on one Wildberries warehouse.
   `x-token-types`: **personal, service, base-with-secret** (a plain base token is refused: 403 example `Response403TokenType` «base
   token is not allowed», ITEMS 10126, AN 6948). Limit **1 minute | 3 requests | 20 s | burst 1**. Body (`InventoryRequest`, AN
   5956-5991): `nmIds` (max 1000), `chrtIds` (used only for the listed `nmIds`), `limit` (max and default 250000), `offset`. 200 →
   `data.items[]` of `nmId`, `chrtId`, `warehouseId` (example -999999), `warehouseName`, `regionName` (country of shipment),
   **`quantity`** («доступное клиентам для добавления в корзину»), **`inWayToClient`**, **`inWayFromClient`** (AN 5993-6053). 204 = no data.
   What the warehouse ids and names mean is explained on release note 605 (`/release-notes?id=605`), which could not be read.
2. **`POST /api/analytics/v1/stocks-report/seller-warehouses`** (AN 1526-1606): the same request; stock on the **seller's** warehouses;
   `x-token-types`: **personal, service only**; same 3/min limit and 30-minute refresh. Row: `nmId, chrtId, warehouseId, warehouseName,
   regionName, quantity` (AN 6055-6100).
3. **`POST /api/v2/stocks-report/products/{groups|products|sizes}`** and **`/offices`** (AN 1607-1897): aggregated tables (by group,
   by product, by size with optional per-warehouse detail, by warehouse/region); «Данные отчёта обновляются 1 раз в 2 часа». Limits by
   token type: personal, service, base-with-secret **3/min (20 s, burst 3)**; **plain base token 2 requests per hour (30 min, burst 1)**.
   Seller warehouses appear only aggregated, `regionName` «Свой склад» (AN 1769, 1840). Per warehouse the metrics include `stockCount`
   («Остатки на текущий день, шт.», AN 4475).
   For a per-size, per-warehouse list of Wildberries' own warehouses with quantity, method 1 is the direct one.

#### 4.2 Older report: `GET /api/v1/warehouse_remains` (REP 467-816)
Three steps, host `seller-analytics-api.wildberries.ru`, category «Аналитика» (REP 41-57). (1) `GET /api/v1/warehouse_remains` creates a
task; query switches `groupByBrand/Subject/Sa/Nm/Barcode/Size`, `filterPics`, `filterVolume`, `locale`; limit personal, service,
base-with-secret **1 per minute (burst 5)**, base **4 per hour** (REP 481-493). (2) `GET .../tasks/{task_id}/status` (1 per 5 s; base
4/h). (3) `GET .../tasks/{task_id}/download` (1 per minute, burst 1; base 4/h). «Готовый отчёт хранится 2 часа» (REP 56). Reply rows
(REP 711-790): `brand, subjectName, vendorCode, nmId, barcode, techSize, volume` and `warehouses[]` of `{warehouseName, quantity}` (only
for non-zero `quantity`); the list holds warehouses plus summary lines such as «В пути до получателей», «В пути возвраты на склад WB»,
«Всего находится на складах». 204 = no data. Described as the report on «остатках на складах WB». How fresh the data is: **not
documented**. No deprecation note in REP.

#### 4.3 Statistics `supplier/stocks`
**Not in the committed or the live files.** `12-reports.yaml` offers only `GET /api/v1/supplier/orders` and `GET /api/v1/supplier/sales`
under «Основные отчёты» (REP 170, 285; tag text REP 29-38). A stock method of the statistics category is therefore not documented
today; whether it was removed, and when: **not documented**.

#### 4.4 What this means for the plan (inference, not a Wildberries statement)
For showing Wildberries' own warehouses: `wb-warehouses` (refresh every 30 min, quantity and in-transit per size per warehouse, up to
250000 rows per request, 3 requests per minute) needs a **personal or service token**. With a plain base token only the 2-per-hour
aggregated tables or the 4-per-hour `warehouse_remains` task are available. Which token type a self-installed wyo seller will hold is
a question for the token research.

---

### 5. How Wildberries treats the number sent

From HELP (read 2026-11-21) and the method texts:
- **An order lowers the stock automatically**: «Информация на сайте и в приложении для покупателей обновляется в течение 15 минут с
  момента загрузки изменений. Когда покупатель закажет ваш товар, остатки уменьшатся автоматически» (HELP, section on the XLSX
  upload). The portal's stock history lists «новые заказы» among the events that change the quantity (HELP, «Посмотреть историю
  изменения остатков»). The next number the seller sends replaces the stored one. A race (an order between the read and the next
  write): **not documented**.
- **What the number means**: «Заказы поступают на каждый склад в соответствии с остатками, которые вы указали для него» (HELP). The
  API calls the field only «Остаток» (ITEMS 6270). The analytics `quantity` is «доступное клиентам для добавления в корзину» (AN 6031).
  Whether the seller's figure is "free to sell" or "physical": **not documented**; the stated effect (orders follow the figure) makes
  it "available to sell".
- **Warning / fine**: «Будьте внимательны при загрузке остатков на конкретный склад. Если товара не будет в наличии и вы не сможете
  отправить его покупателю, будет применяться штраф за невыполненные заказы» (HELP). An FBS order carries a flag «заказ сделан на
  товар с нулевым остатком»; «Такой заказ можно отменить без штрафа за отмену» (FBS 6107-6109; also 5614-5616, 6473-6475). So an
  order can still arrive at stock 0 and cancelling it costs no cancellation fine.
- **At 0**: no sentence about `amount: 0`. A card keeps selling while any stock lies on a warehouse, even from the trash: «Карточка будет
  продаваться, пока по ней есть остатки на складе, даже если её переместили в корзину. Чтобы полностью снять карточку с продажи,
  обнулите остатки» (ITEMS 3489) - so **zeroing the stock stops the sale of a card**. HELP's filter finds «товары с нулевыми остатками»,
  so zero rows exist in the portal.
- **Delete** = irreversible, a re-upload is needed to resume sales (ITEMS 6360).
- **Hidden cards**: a card with stock but no photo, price, dimensions or category is not shown on the site; a blocked card likewise
  (HELP, «Частые вопросы»).
- **History in the portal**: 3 months, only from 30.04.2026; events include «изменения или удаления товаров через API» (HELP).
- A second writer: HELP warns that stock may be zeroed or set automatically if «дополнительные расширения» (third-party tools) are
  installed, and advises deleting all API tokens in the cabinet's settings and disabling parser extensions.
- **Latency**: the buyer-visible number updates within **15 minutes** of an upload (HELP; stated for the XLSX upload, the API: not stated).

---

### 6. Live files compared with the committed copies

Downloaded 2026-11-21 (HTTP 200 each) from `https://dev.wildberries.ru/api/swagger/yaml/ru/<file>` into
a copy kept for the day and compared with `diff`.
- `01-general.yaml`, `02-items.yaml`, `11-analytics.yaml`, `12-reports.yaml`: **byte-identical to the committed copies** (0 diff
  lines). Every method in sections 1-4 is unchanged.
- `03-orders-fbs.yaml`: 621 diff lines, **none mentions `chrtId`, stock or warehouses** (checked). The differences concern FBS supplies
  and statuses: `GET /api/marketplace/v3/fbs/shipping-points` (marked `deprecated`, «будет отключен 20 ноября») and the SPOT methods
  and the OKSM country list are absent from the live file; the order status `delivering_by_carrier` (from 12 November, replaces
  `accepted_by_carrier` and `sent_to_carrier`, cross-border only) and `postponed_delivery` are new; a hint towards the archive method.
  Unrelated to stock; the committed copy of FBS is stale on those points (for whichever plan touches supplies or order statuses).

---

### 7. Gaps (what a real account would have to show)

1. Whether a read-only token is refused on `PUT`/`DELETE` stock (expected 403) and accepted on the read `POST`.
2. Which request limit holds for a base token on the Marketplace category: 150/min (GEN) or 300/min (stock page); read the
   `X-Ratelimit-Remaining` header after a burst.
3. Whether `amount: 0` is accepted by `PUT` and what it does (sales stop? card hidden?); whether a negative number is refused.
4. Whether one bad `chrtId` in a batch of 1000 rejects the whole batch or only itself (the 409 body lists only the offenders).
5. Whether the stock read omits unknown chrtIds or returns 0 for them; how to list a warehouse's whole stock with it.
6. Whether DBW (3), C&C (5) and EDBS (6) warehouses take stock through the same `PUT` as FBS (1) and DBS (2).
7. The race between an order and the next write; whether the sent number replaces the counter outright.
8. How fast stock reaches the buyer's page after an API write (HELP gives 15 minutes for an XLSX upload).
9. Since when upload by barcode is refused, and every other dated change: the release notes and knowledge base at
   `dev.wildberries.ru` answer HTTP 498 (an anti-bot challenge) to scripts, so they are unread.
10. The meaning of warehouse ids and names in the analytics stock methods (negative ids, «Свой склад»): release note 605, unread.
11. Which token type the seller will hold decides whether `wb-warehouses` (personal, service, base-with-secret) can be used at all.
12. Freshness of `warehouse_remains`; whether its `warehouses[]` includes the seller's own warehouses.
13. Whether a size's `chrtId` can change when a card is edited or merged (a chrtId that vanishes): not documented.
14. What becomes of a deleted warehouse's stock records, and whether a warehouse's type can change.

## Part O — Ozon
Scope: what the wyo server needs for the stock stage: (a) show what lies in Ozon's own warehouses (FBO) and in the seller's warehouses,
(b) send a fulfilment centre's free stock to the seller's own warehouses (FBS and rFBS). Nothing was called at Ozon; no page of
`ozon.ru` was fetched.

### Sources and how they are cited

| Cite | Source | Read |
| ---- | ------ | ---- |
| **[JSON]** | `/home/igor/projects/wyo/api/ozon/seller-api.json` (Ozon's own file, `info.version` 2.1, 497 methods, saved by the maintainer 2026-11-18, SHA-256 `57f27f1a08d09f75473601183b8ee2d0f2e3336c084ca295d41d4969a0be8b2f`, see `/home/igor/projects/wyo/api/ozon/README.md`). Location is written as method path plus JSON place, e.g. `/v2/products/stocks` `post.description`. Tag texts are `tags[name=...].description`; **Errors** = tag `Errors` (table «Частые ошибки»), **News** = tag `News` (Ozon's dated list of changes), **Process** = tag `Process` («Порядок работы с методами»), **Limits** = tag `Limits`. | 2026-11-21 |
| **[ROLES]** | `/home/igor/projects/wyo/docs/marketplaces/Ozon - Маппинг ролей и методов Seller API – Ozon for dev.pdf` (24 pages; the table is dated by Ozon 15 August 2024; Ozon writes on page 1 «Методы иногда устаревают, но мы стараемся постоянно обновлять информацию»). No PDF tool exists on this machine; the text was pulled out of the PDF's own text streams with a small Python script (a few letter pairs such as «fb», «fi», «ф» in some words came out as `?`; method paths stay readable). Page numbers below are the PDF's own page order. | 2026-11-21 |
| **[GH]** | `https://raw.githubusercontent.com/vispar-tech/ozon-seller-api-schema/main/schemas/ozon-seller-api-openapi.json`, a third party's copy, last commit `86921fb8013a` of 2026-11-20 ("chore: update schema (2026-11-20)"), 4 380 658 bytes, SHA-256 `f4f387d2340e4ccfe91c48342bf6467d4e0b2b83f29424f780f732a121047897`. Downloaded to (a copy kept for the day) and only read as JSON. | 2026-11-21 |
| **[RES]** | existing research `/home/igor/projects/wyo/.agents/research/ozon-official-seller-api-check-2026-11-18.md` (sections 3, 4, 5); **[DOC]** `/home/igor/projects/wyo/docs/ozon.md`; **[CFG]** `/home/igor/projects/wyo/api/codegen/ozon.cfg.yaml`. | 2026-11-21 |

What the wyo server has today [CFG]: only `WarehouseListV2` (`/v2/warehouse/list`) and `ProductAPI_GetProductInfoList` (`/v3/product/info/list`)
of the methods below are generated; none of the stock methods is.

General limits for every method [JSON, Limits]: «Если лимит не указан в описании метода, вы можете сделать не больше 50 запросов в
секунду с одного Client ID.» Exceeding a limit gives `429 Too Many Requests`; the answers carry the headers `Ratelimit-Remaining`
(requests left before the 429) and `Retry-After` (seconds until a repeat is allowed; returned with the 429). [JSON, Process]: «Если от
вашего имени поступает много одинаковых или ошибочных запросов, мы можем ограничить доступ к Seller API без предупреждения» and, in tag
`Auth`: «Ozon может отозвать ключ, если обнаружит подозрительную активность или слишком большую нагрузку на свои системы». Error words
of every method [JSON, Errors, «Все методы»]: `Circle is open` (a method is blocked for some minutes under heavy load), `Internal error`,
`Invalid Api-Key…`, `Api-key is deactivated…`, `Api-Key is missing a required role for a method`, `Api-Key is restricted to specific IP
addresses`, `You have reached request rate limit per second` (max 50 requests per second from one Client ID), `method is not allowed`.

---

### 1. Writing stock to a seller's (FBS / rFBS) warehouse

#### 1.1 The current method

`POST /v2/products/stocks` («Обновить количество товаров на складах», `operationId` `ProductAPI_ProductsStocksV2`, tag `Prices&StocksAPI`) [JSON].
It is **not** marked `deprecated`. No newer write method for stock exists in the file (searched every path for `stock`/`остат`: the only
other writer is `/v1/product/digital/stocks/import`, «доступен только продавцам, работающим с цифровыми товарами», body `stocks[]{offer_id, stock}`,
answer `status[]{offer_id, product_id, sku, updated, errors[]}`; no warehouse, no limits stated; not for this stage).

The older method **`/v1/product/import/stocks` is gone**: News 6 May 2025 «Метод будет отключён 27 мая 2025 года. Переключитесь на
/v2/products/stocks»; News 27 May 2025 «Метод устарел, удалили его из документации». It is not in the committed file (but it is still
named in the 2024 role table, [ROLES] p. 17, role `Product`).

#### 1.2 Body (`requestBody` → `productv2ProductsStocksRequest`)

```json
{ "stocks": [ { "offer_id": "PH11042", "product_id": 313455276, "stock": 100, "warehouse_id": 22142605386000 } ] }
```
(Ozon's own example, `requestBody.content.application/json.example`.)

| Field | Type | Ozon's words [JSON, `components.schemas.productv2ProductsStocksRequestStock.properties`] |
| ----- | ---- | ----------------------------------------------------------------------------------------- |
| `stocks[]` | array | «Информация о товарах на складах.» |
| `offer_id` | string | «Идентификатор товара в системе продавца — артикул.» |
| `product_id` | int64 | «Идентификатор товара в системе Ozon — `product_id`.» |
| `stock` | int64 | «Количество товара в наличии без учёта зарезервированных товаров.» |
| `warehouse_id` | int64 | «Идентификатор склада, полученный из метода /v1/warehouse/list» (a link to the old method that Ozon switched off on 7 April 2026, see section 3; the current list is `/v2/warehouse/list`). |

- **Which identifier is used:** `post.description`: «Если запрос содержит оба параметра — `offer_id` и `product_id`, изменения применятся к товару с
  `offer_id`. Для избежания неоднозначности используйте только один из параметров.»
- **Defect of the committed file:** the schema's `required` list holds `product_id`, `quant_size`, `stock`, `warehouse_id`, but
  `quant_size` is not among the properties, and `offer_id` (which the text says may be used instead of `product_id`) is not in `required`.
  News 16 June 2025 marked `stocks.quant_size` (request) and `result.quant_size` (answer) deprecated and News 26 June 2025 says
  «Удалили параметры `stocks.quant_size` из запроса и `result.quant_size` из ответа метода». The `required` list was not cleaned. Ozon's
  economy tariff («Эконом», the thing quants belonged to) is switched off: News 8 September 2026 «Тариф «Эконом» отключён, удалили методы из
  документации» (about `/v1/product/quant/list`, `/v1/product/quant/info`). The error `SIZE_REQUIRED_FOR_NOT_UNIQUE_OFFER_ID` (below) still
  mentions `quant_size`. **What happens if `quant_size` is absent from a real request: not documented** (the 2025 news say it was removed).
  A program that builds a client from the file must therefore not treat `quant_size` as a field to send.

#### 1.3 Items per request, quantity, rates

All from `post.description` [JSON] unless stated:

- **Items per request:** «За один запрос можно изменить наличие для 100 пар товар-склад.» (The schema has no `maxItems`.)
- **Request rate:** «С одного аккаунта продавца можно отправить до 80 запросов в минуту.» (= up to 8 000 product-warehouse pairs per minute.)
- **Per pair interval:** «Обновлять остатки у одной пары товар-склад можно только 1 раз в 30 секунд, иначе в параметре `result.errors` в ответе будет ошибка
  `TOO_MANY_REQUESTS`.» Errors table has two words for this: `TOO_MANY_REQUESTS` («Вы слишком часто обновляли остатки для одного артикула. Остатки для
  одного артикула можно обновлять не чаще одного раза в 30 секунд.») and `Stock is updated too frequently` («…Обновлять остатки товара на одном складе можно только
  1 раз в 30 секунд. Убедитесь, что ваша интеграция не обновляет остатки в фоновом режиме.»). News 23 June 2025 added the second and reworded the first.
  The two texts differ (one says «для одного артикула», the other «на одном складе»); **which one Ozon returns in which case: not documented.**
- **Quantity range:** only the upper bound is documented, by the error `STOCK_TOO_BIG`: «Вы указали слишком большое значение для остатка товара. Укажите
  количество меньше миллиона». The type is int64. **Lower bound, negative values, and whether 0 is accepted by this method: not documented.** (Indirect: Process says, for
  archiving a product, «Перед архивированием товара обнулите его остатки» and «Товар попадёт в продажу, только когда вы установите его остаток», which implies 0 is a
  value one can set; the method's own text does not say it.)
- **When a product may take stock:** «Вы можете задать наличие товара только после того, как его статус сменится на `price_sent`.» «Остатки крупногабаритных товаров
  можно обновлять только на предназначенных для них складах.»
- **Status codes:** `responses` lists `200`, `400` («Неверный параметр»), `403` («Доступ запрещён»), `404`, `409` («Конфликт запроса»), `500`; the error body is `rpcStatus`
  `{code:int32, message, details[]{typeUrl, value}}`. `429` is not listed for this method but is the general answer [Limits].
- **Header parameters:** `Client-Id` (string, required, «Идентификатор клиента»), `Api-Key` (string, required, «API-ключ»). Server `//api-seller.ozon.ru`.

#### 1.4 The answer (HTTP 200)

`result[]` — one object per requested pair, **errors are per item inside a 200** [JSON `productv2ProductsStocksResponse`]:

| Field | Ozon's words |
| ----- | ------------ |
| `result[].warehouse_id` | «Идентификатор склада.» |
| `result[].product_id` | «Идентификатор товара в системе Ozon — `product_id`.» |
| `result[].offer_id` | «Идентификатор товара в системе продавца — артикул.» |
| `result[].updated` | «Если запрос выполнен успешно и остатки обновлены — `true`.» |
| `result[].errors[]` | «Массив ошибок, которые возникли при обработке запроса.» each `{code: string «Код ошибки.», message: string «Причина ошибки.»}` |

Example: `{"result":[{"warehouse_id":22142605386000,"product_id":118597312,"offer_id":"PH11042","updated":true,"errors":[]}]}`.
(The example's `product_id` 118597312 differs from the request example's 313455276; it is Ozon's sample only.)

#### 1.5 Per-item error codes [JSON, Errors, section `/v2/products/stocks`; table last changed: News 21 July 2026 added `PRODUCT_IS_ARCHIVED`, 26 May 2026 reworded `FLAMMABLE_…`]

| Code (`errors[].code`) | Ozon's meaning |
| ---------------------- | -------------- |
| `product_is_not_created` | «Товар не прошёл модерацию, поэтому обновить остатки пока нельзя. Дождитесь статуса `price_sent` и попробуйте ещё раз.» |
| `offer_id_not_found` | «Товара с таким артикулом нет в личном кабинете.» |
| `NOT_FOUND_ERROR` | «Товар не найден в личном кабинете.» |
| `WAREHOUSE_NOT_FOUND` | «Склад `warehouse_id` не найден. Проверьте наличие ошибок в ID склада и его статус — он должен быть активный.» |
| `Request validation error: invalid ProductsStocksRequest.Stocks[0]: embedded message failed validation` | «В запросе не указан идентификатор склада.» (the index in the text is the item's position) |
| `PRODUCT_IS_ARCHIVED` | «Товар в архиве. Чтобы обновить остаток, разархивируйте товар.» |
| `SOURCE_TYPE_NOT_FOUND` | «У товара нет SKU. Проверьте, что товар создан и правильно настроен.» |
| `STOCK_TOO_BIG` | «слишком большое значение… Укажите количество меньше миллиона» |
| `TOO_MANY_REQUESTS` | more than once in 30 s for the same article (see 1.3) |
| `Stock is updated too frequently` | more than once in 30 s for the same product on one warehouse (see 1.3) |
| `SIZE_REQUIRED_FOR_NOT_UNIQUE_OFFER_ID` | «Артикул совпадает с артикулом другого товара. Для обычного товара укажите параметр `quant_size = 1`, а для кванта — `quant_size = 2` и больше.» (stale: `quant_size` was removed from the method in 2025) |
| `MULTIBOX_NOT_ALLOWED_FOR_FBS` | «При работе по схеме FBS нельзя объединить товары из нескольких коробок в один товар. Удалите значение из поля `stock` и повторите попытку.» |
| `PRODUCT_HAS_NOT_BEEN_TAGGED_YET` | «Товар ещё не пометили тегами «КГТ» или «неКГТ», так как не указаны габариты товара или система для расстановки тегов ещё не обработала его.» |
| `NON_KGT_ON_KGT_WAREHOUSE` | «Попытка установить или обновить остаток некрупногабаритного товара на КГТ складе.» |
| `OVER_MAX_OVH_NON_KGT` | «Через выбранный склад нельзя продавать крупногабаритные товары. Выберите другой склад или создайте новый и повторите попытку.» |
| `OVER_MAX_OVH_KGT` | «Вес или габариты товара превышают максимальные значения для выбранного склада. Измените характеристики товара или выберите другой склад.» |
| `FLAMMABLE_ONLY_ON_SELF_OR_PROVIDER_DELIVERY` | «Легковоспламеняющиеся товары можно продавать только со своего склада с доставкой самостоятельно или сторонней службой. Выберите другой склад или создайте новый» |
| `MP_DELIVERY_ONLY_3PL_ERROR` | «Товар нельзя размещать на складе с методом доставки «Ozon логистика».» |
| `CB_DELIVERY_ONLY_FBP` | «Этот товар доступен к продаже только с FBP склада.» |

Which of these will not go away on a repeat (a fix of the product or the warehouse choice is needed) and which will (`TOO_MANY_REQUESTS`,
`Stock is updated too frequently`, `Circle is open`, `Internal error`, 429): Ozon does not classify them; **not documented**.

#### 1.6 Role needed

- [ROLES] (table of 15 Aug 2024): `/v2/products/stocks` is in the role **`Product`** («Позволяет получать информацию и управлять товарами»), PDF p. 18 (role
  starts p. 16). The same role lists the removed `/v1/product/import/stocks` (p. 17). It is **not** in `Product read-only` (pp. 19–20), not in `Warehouse` (pp. 22–23),
  and not in `Admin read only` (a read-only role; it lists no `import` or `products/stocks`). `Admin` («Позволяет оперировать всеми методами Seller API») opens all.
- The key's own answer to `/v1/roles` (`roles[].methods[]`) is what the wyo server checks [DOC "Метод разрешён, если…"]; whether Ozon puts `/v2/products/stocks` there:
  not documented, it needs a real key.

---

### 2. Reading the stock of the seller's own warehouses

All are `POST` on `https://api-seller.ozon.ru`, tag `Prices&StocksAPI`. **No method here states a rate limit of its own**, so the general 50 requests per second per Client ID
applies. None of them is in the 2024 role table ([ROLES] has no `/v4/…`, no `/v2/product/info/stocks-by-warehouse/fbs`, no `/v1/product/info/warehouse/stocks`,
no `/v1/product/info/stocks-by-warehouse/fbo`, no `/v1/analytics/stocks`, no `/v2/warehouse/list`, no `/v3/product/list`, no `/v3/product/info/list`; checked by searching the extracted text).
**Role of each: not documented**; the older versions' roles are in 2.7.

#### 2.1 `/v2/product/info/stocks-by-warehouse/fbs` («Получить информацию об остатках на складах продавца», `ProductAPI_GetProductInfoStocksByWarehouseFbsV2`) — the one that has a row per warehouse and product

- Added as beta 2 December 2025 (News), `limit` made required 12 February 2026 (News); it is the successor of the switched-off `/v1/product/info/stocks-by-warehouse/fbs`.
  The method has **no `description` text** in [JSON]. Process says it is the method «для получения информации о количестве остатков для FBS и rFBS» and the one to check reservation with.
- Request [JSON]: `limit` (uint64, **required**, `maximum` 1000 «Количество значений в ответе.»), `cursor` («Указатель для выборки следующих данных.»), and exactly one of `sku` (array of int64,
  `maxItems` 1000 «Идентификаторы товаров в системе Ozon — SKU.») or `offer_id` (array of string, `maxItems` 1000, «артикул») (a `oneOf` of `required: [sku]` / `required: [offer_id]`).
  **Without a list of identifiers: not documented** (the `oneOf` suggests a list is required; the pager `cursor` is documented).
- Answer: `cursor`, `has_next` («`true`, если в ответе вернули не все товары.»), `products[]`:
  `offer_id`, `product_id` (Ozon words «Идентификатор товара в системе продавца — артикул», a slip in the text; the field is int64), `sku`, `warehouse_id`, `warehouse_name` («Название склада.»),
  **`present`** («Общее количество товара на складе.»), **`reserved`** («Количество зарезервированных товаров на складе.»), **`free_stock`** («Количество доступных для продажи товаров.»).
  How the three relate (is `present` = `free_stock` + `reserved`): not stated.
- Whether a product with 0 everywhere, or a warehouse that never got a stock for this product, has a row: not documented.
- Errors listed: `400`, `403`, `404`, `409`, `500`. Any per-method error code: none in Errors.

#### 2.2 `/v1/product/info/warehouse/stocks` («Получить информацию по остаткам на складе FBS и rFBS», `ProductInfoWarehouseStocks`) — one warehouse, all its products

- Beta 2 October 2025, moved to the main section 30 October 2025 (News). No description text.
- Request: `warehouse_id` (int64, required), `limit` (int64, required, `minimum` 1, `maximum` 1000), `cursor`. Example `{"cursor":"","limit":10,"warehouse_id":1020003080073000}`.
- Answer: `cursor` («Если параметр пустой, данных больше нет.»), `has_next` («`true` — сделайте повторный запрос с другим значением `cursor`…»), `stocks[]{sku, product_id, offer_id, warehouse_id, present, reserved, free_stock, updated_at}`
  (`present` «Общее количество товара на складе.», `reserved` «Количество зарезервированных товаров на складе.», `free_stock` «Количество товара на складе, которое доступно для заказа.»,
  `updated_at` date-time «Дата последнего обновления товара.»). The example's `cursor` is the last `sku` («147035011»).
- This is the only read method that returns the time of the last change of a number (`updated_at`).

#### 2.3 `/v4/product/info/stocks` («Информация о количестве товаров», `ProductAPI_GetProductInfoStocks`) — per product, all schemes at once, **no warehouse number**

- Moved from beta to main 10 February 2025 (News); its predecessor `/v3/product/info/stocks` is gone (News 10 February 2025; still named in the 2024 role table).
  Description: «Возвращает информацию о количестве товаров по схемам FBO, FBS, rFBS и FBP: сколько единиц есть в наличии, сколько зарезервировано покупателями.»
  Warning: «23 ноября 2026 года отключим параметр `total` в ответе метода. Переключитесь на `total_items`.» (News 24 September 2026). 23 November 2026 is two days after the day read.
- Request: `limit` (int32, required, «Минимум — 1, максимум — 1000.»), `filter` (required): `offer_id[]`, `product_id[]` (int64), `visibility` (default `ALL` = «все товары, кроме архивных»; 28 values, e.g. `EMPTY_STOCK`
  «товары, у которых не указано наличие», `ARCHIVED`, `IN_SALE`), `with_quant{created, exists}` («Товары по тарифу «Эконом»»), plus `cursor`.
  The number of identifiers per request in `filter.offer_id` / `filter.product_id`: **not documented** (no `maxItems`; `limit` is 1000).
- Answer: `cursor`, `total_items` (int64; `total` is deprecated), `items[]{offer_id, product_id, stocks[]}`; each `stocks[]` row: `type` («Тип склада: `fbs` — склад продавца, доставка силами Ozon; `rfbs` — склад
  продавца, доставка силами продавца; `fbo` — склад Ozon; `fbp` — склад партнёра»), `sku`, `present` («Сейчас на складе»), `reserved` («Зарезервировано»), `shipment_type`
  (`SHIPMENT_TYPE_GENERAL`/`BOX`/`PALLET`), and `warehouse_ids[]` (**deprecated**: News 27 November 2025 «Пометили устаревшим параметр `items.stocks.warehouse_ids`»;
  «Идентификаторы складов, на которых хранился или хранится товар»). So **this method cannot say which warehouse holds what**; it gives one row per scheme (and shipment type) per product.
  Example in the file: an `fbs` row `present 150, reserved 25` and an `fbo` row `present 75, reserved 10` for one product.
- Process: «Для получения информации о количестве остатков для FBO, FBS, rFBS и FBP используйте /v4/product/info/stocks.»

#### 2.4 `/v1/product/info/stocks-by-warehouse/fbs` (`ProductAPI_ProductStocksByWarehouseFbs`) — **do not use**

«Метод устаревает и будет отключён 7 апреля 2026 года. Переключитесь на /v2/product/info/stocks-by-warehouse/fbs.» (News 24 March 2026; first marked 2 December 2025 as «будет отключён в будущем».)
The date is past on the day read, yet the committed file (saved 2026-11-18) still lists it. Request: `sku` or `offer_id` (if both, only `sku` is used); answer `result[]`. Whether Ozon really switched it off: not documented
(a real call would say; it is not made here).

#### 2.5 Read methods that cover stock in a report form

`/v1/report/warehouse/stock` («Отчёт об остатках на FBS-складе», `ReportAPI_CreateStockByWarehouseReport`) creates a report to download later; it was not read in detail as it is a report job, not a stock read.

#### 2.6 Where to look for a product's stock presence

`/v3/product/list` items have booleans `has_fbs_stocks`, `has_fbo_stocks` (section 4) and `/v3/product/info/list` has `stocks{has_stock, stocks[]{present, reserved, sku, source}}`,
`visibility_details.has_stock`, `discounted_fbo_stocks` («Остатки уценённого товара на складе Ozon») (section 4).

---

### 3. Which warehouses take stock

#### 3.1 `/v2/warehouse/list` (`WarehouseListV2`, tag `WarehouseAPI`) — the current method

- Description: «Метод возвращает список складов FBS и rFBS. Чтобы получить список складов FBO, используйте метод /v1/warehouse/fbo/list.» Process lists it as the way to «получите список складов».
- Request: `limit` (**required**, `maximum` 200 «Количество значений в ответе.»), `cursor`, `warehouse_ids[]` (int64, `maximum` 200). Answer: `warehouses[]`, `cursor`, `has_next`.
  History (News): `limit`/`cursor` added 27 November 2025; `has_next` 16 December 2025; `pause_at` 7 April 2026. **No rate limit of its own is stated** for the v2 method (the old v1 said «Метод можно использовать 1 раз в минуту»).
- Fields of `warehouses[]` [JSON]: `warehouse_id`, `name`, `status` («Статус склада.», **the v2 text names no values**), `warehouse_type` («Тип склада.», no values named; the example holds `"string"`),
  `is_rfbs` («Признак работы склада по схеме rFBS.»), `is_express` («Признак доставки express. Время доставки до покупателя не больше 60 минут.»), `is_comfort` («Признак доставки comfort. Время доставки
  до покупателя от 60 минут.»), `is_kgt` («Признак, что склад принимает крупногабаритные товары.»), `has_entrusted_acceptance` («Признак подключения доверительной приемки.»), `pause_at` («Дата, когда продавец поставил
  склад на паузу. Если значение `null`, склад активный. Только для rFBS-склада.»), `has_postings_limit` / `postings_limit` («Лимит заказов. `-1`, если лимита нет.») / `min_postings_limit`, `is_auto_assembly`,
  `is_waybill_enabled`, `with_item_list`, `carriage_label_type`, `cut_in_time`, `sla_cut_in`, `first_mile{type, dropoff_point_id, timeslot_id, timeslot_from, timeslot_to, first_mile_is_changing}`, `timetable{…}`, `working_days[]`,
  `address_info{address, latitude, longitude, utc}`, `phone`, `courier_phones`, `courier_comment`, `created_at`, `updated_at`.
- **Status values:** only the **old** method documents them (`/v1/warehouse/list`, `result[].status`): `new` — «Активируется», `created` — «Активный», `disabled` — «В архиве», `blocked` — «Заблокирован»,
  `disabled_due_to_limit` — «На паузе», `error` — «Ошибка» (left side = Seller API word, right side = word in the seller cabinet). The example of v2 shows `"status": "created"`. **Whether v2 uses the same six words: not documented.**
- **Which warehouses accept stock:** the documents say only (a) `WAREHOUSE_NOT_FOUND`: «Проверьте… его статус — он должен быть активный» [Errors, `/v2/products/stocks`]; (b) Process: «После загрузки товаров для схем FBS и rFBS можно перейти к
  обновлению остатков»; the stock method's text addresses FBS and rFBS warehouses by one `warehouse_id` and one body. There is **no field that says «this warehouse takes stock»**; what to do for the other states: below.
- **Archived (`disabled`):** `/v1/warehouse/archive` (`ArchiveWarehouseFBS`, body `warehouse_id`, `reason` ≤ 200 characters, `return_point_id`) moves a FBS warehouse to the archive; its text does not say what happens to the stock or to
  stock writes. What a stock write to an archived warehouse returns: **not documented** (`WAREHOUSE_NOT_FOUND` is the only candidate and the Errors table does not say it is returned for an archived one).
- **Paused rFBS (`pause_at` not null / `disabled_due_to_limit` «На паузе»):** `/v1/warehouse/rfbs/pause`: «Скрывает товары склада с витрины. Изменять сток не нужно.»; `/v1/warehouse/rfbs/unpause`: «Возвращает товары склада на витрину.» So on a pause the stock **stays** and Ozon says it need not be changed.
  Whether a write to a paused warehouse is accepted: not documented.
- **`has_entrusted_acceptance`:** only the flag «Признак подключения доверительной приемки» (v1: «`true`, если доверительная приёмка включена на складе»); nothing in the documents ties it to stock.
- **Express / comfort:** flags only; the Errors table has no code for express. `MP_DELIVERY_ONLY_3PL_ERROR` («Товар нельзя размещать на складе с методом доставки «Ozon логистика».») and `FLAMMABLE_ONLY_ON_SELF_OR_PROVIDER_DELIVERY` exist.
  Whether a stock written for an express warehouse (delivery in up to 60 minutes) behaves differently: **not documented.**
- **KGT:** `is_kgt` — the warehouse takes large goods; `/v1/warehouse/list` also `is_kgt`. The stock method's text: «Остатки крупногабаритных товаров можно обновлять только на предназначенных для них складах.» and the codes `NON_KGT_ON_KGT_WAREHOUSE`,
  `OVER_MAX_OVH_KGT`, `OVER_MAX_OVH_NON_KGT`.
- **Products that cannot be delivered from a warehouse:** `/v1/warehouse/warehouses-with-invalid-products` («Возвращает идентификаторы складов, на которых находятся товары с ограничениями. Такие товары недоступны для доставки со склада.») and
  `/v1/warehouse/invalid-products/get` (`warehouse_id`, `last_id`; answer `has_next`, `last_id`, `validation_results[]`). Not read further.
- **Role** (2024 table): `/v1/warehouse/list` is in `Warehouse` («Позволяет получить информацию по складам», PDF pp. 22–23), and in `Admin read only` (PDF p. 8); `/v2/warehouse/list` is **not in the table** (not documented). The wyo server already treats a missing warehouse role
  as a rejected key [DOC].

#### 3.2 `/v1/warehouse/list` (`WarehouseAPI_WarehouseList`) — switched off

«Метод устаревает и будет отключён 7 апреля 2026 года. Переключитесь на /v2/warehouse/list.» (News 16 February 2026 first said 20 March 2026; 24 March 2026 moved the date to 7 April 2026.) Still in the committed file; the stock method's own
text and the answer of `/v2/products/stocks` still link to it. Request `limit` (≤ 200), `offset`, `with.able_to_set_price`; answer `result[]` with `status` table above, `is_rfbs`, `has_entrusted_acceptance`, `is_kgt`, `is_karantin` («склад не работает из-за карантина»),
`is_economy`, `is_able_to_set_price`, etc. Not for the new stage; useful only for the meaning of the six status words.

#### 3.3 Ozon's own warehouses (FBO)

`/v1/warehouse/ozon/list` («Получить список складов Ozon»; «Возвращает список складов Ozon, которые работают по схемам FBO и FBO Fresh, и возвратных складов»): request `warehouse_types[]`; answer `warehouses[]{warehouse_id, name,
short_name, address, timezone, is_active, warehouse_type, country_iso_numeric, is_cross_dock, is_distribution_center, is_edo, is_express, is_for_supply}`. This is where `/v1/product/info/stocks-by-warehouse/fbo`'s `warehouse_id` comes from. Role: not documented.
`/v1/warehouse/fbo/list` is a search of supply points (`search` of at least 4 characters) and is not a list of stock warehouses.

---

### 4. How a product is matched

The three numbers of a product are `offer_id` (the seller's article), `product_id` (Ozon's product number) and `sku` (Ozon's «SKU») [RES section 5; DOC «Три номера товара»]. The stock write takes `offer_id` or `product_id` (1.2). The stock reads take
`offer_id`, `sku` or `product_id` depending on the method (2.1–2.3).

#### 4.1 `/v3/product/list` («Список товаров», `ProductAPI_GetProductList`)

- Moved from beta to main 11 February 2025 (News). Warning: «23 ноября 2026 года отключим параметр `result.total` в ответе метода. Переключитесь на `result.total_items`.»
- «Метод для получения списка всех товаров.» «Если вы используете фильтр по идентификатору `offer_id` или `product_id`, остальные параметры заполнять не обязательно. Вернутся все товары с указанными идентификаторами, независимо от видимости.
  За один раз вы можете использовать только одну группу идентификаторов, не больше 1000 товаров.» «Если вы не используете для отображения идентификаторы, укажите `limit` и `last_id` в следующих запросах.»
- Request: `filter{offer_id[] ≤ 1000, product_id[] ≤ 1000, skus[] ≤ 1000 (added 9 July 2026), visibility (default ALL, «все товары, кроме архивных»)}`, `last_id` («При первом запросе оставьте это поле пустым. Чтобы получить следующие значения, укажите
  `last_id` из ответа предыдущего запроса.»), `limit` (int64, «Минимум — 1, максимум — 1000.»). `limit` is not marked required in the schema.
- Answer: `result{items[], last_id, total (deprecated), total_items}`; `items[]{offer_id, product_id, sku (added 9 July 2026), archived, has_fbo_stocks, has_fbs_stocks, is_discounted, quants[]{quant_code, quant_size}}`.
  **No barcodes here.** How the end of the list is shown (empty `last_id`, or fewer items than `limit`): not documented for this method (the stock readers show `has_next`/empty `cursor`).
- Role: not in the 2024 table (`/v2/product/list` is: `Product`, `Product read-only`, `Admin read only`); `/v3/product/list` role: not documented.

#### 4.2 `/v3/product/info/list` («Получить информацию о товарах по идентификаторам», `ProductAPI_GetProductInfoList`) — already used by wyo [CFG, DOC]

- Moved from beta to main 22 January 2025 (News). «В одном запросе вы можете передать не больше 1000 товаров по параметрам `offer_id`, `product_id` и `sku` в сумме.» No page/cursor.
- Request: `offer_id[]`, `product_id[]` (int64), `sku[]` (int64). Answer `items[]` with, among about 50 fields: `id` («Идентификатор товара в системе Ozon — `product_id`»), `offer_id`, `sku`, `barcodes[]` («Все штрихкоды товара»), `name`, `is_archived`
  («`true`, если товар архивирован вручную»), `is_autoarchived`, `is_kgt` («Только для схемы FBS»), `statuses{status, moderate_status, validation_status, is_created, status_updated_at, …}`,
  `stocks{has_stock («`true`, если есть остаток на складах»), stocks[]{present, reserved, sku, source}}`, `visibility_details{has_price, has_stock}`, `sources[]{sku, source («`SDS` — FBO и FBS с одинаковым SKU; `FBO`; `FBS`»), shipment_type, quant_code}`, `availabilities[]`.
- Role: `/v3/product/info/list` not in the 2024 table (`/v2/product/info/list` is; the removed `/v2/…` method was replaced, News 17 February 2025). The wyo server maps it to `Product read-only` by guess [DOC]; real key answer is the only proof.
- A stock write wants `price_sent` status [1.3]; where a product's status word `price_sent` is read: `items[].statuses.status` [not checked value-by-value here: the field is a free string «Статус товара.»; **the list of its values: not documented in this file**].

#### 4.3 What the seller's article (`offer_id`) is

String, «Идентификатор товара в системе продавца — артикул». The error `SIZE_REQUIRED_FOR_NOT_UNIQUE_OFFER_ID` («Артикул совпадает с артикулом другого товара») shows that two products can carry one article (it belonged to quants, an ended tariff). `/v1/product/update/offer-id` («Если `offer_id` уже используется для другого товара, вернётся ошибка `OFFER_ID_ALREADY_EXISTS`») suggests an article is meant to be unique among a seller's products now; the documents do not state it as a rule. **Case, length, spaces and allowed characters of an article: not documented in this file.** For a stock write `offer_id` is the only seller-side key (there is no barcode key); a product found by barcode has to go through
`/v3/product/info/list` (`barcodes[]`) to its `offer_id`/`sku` first.

---

### 5. Stock in Ozon's own warehouses (FBO, only for display)

#### 5.1 Candidates

| Method | State | What it gives |
| ------ | ----- | ------------- |
| `/v1/product/info/stocks-by-warehouse/fbo` (`GetProductInfoStocksByWarehouseFbo`) | main; added 2 July 2026 (News «Добавили новый метод для получения информации о стоках на складах FBO»); Process mislabels it «обновите цены и остатки товаров» (copy slip) | per product and per Ozon warehouse: `present`, `reserved` |
| `/v1/analytics/stocks` (`AnalyticsAPI_AnalyticsStocks`, tag `AnalyticsAPI`) | main since 24 March 2026 (News «Перенесли метод из бета-раздела в основной»); added 9 April 2025 | per SKU and per warehouse/cluster: availability, in-transit, defect, turnover, many counters |
| `/v4/product/info/stocks` | main | per product: one `fbo` row with `present`, `reserved` — no warehouse (2.3) |
| `/v2/analytics/stock_on_warehouses` (`AnalyticsAPI_AnalyticsGetStockOnWarehousesV2`) | «В будущем метод будет отключён. Переключитесь на /v1/analytics/stocks.» (**no date**) | `result.rows[]{sku, item_code, item_name, free_to_sell_amount, promised_amount, reserved_amount, warehouse_name}` (limit/offset; `warehouse_type` `ALL`/`EXPRESS_DARK_STORE`/`NOT_EXPRESS_DARK_STORE`); «Отличается от отчёта в разделе Аналитика → Отчёты → Отчёт по остаткам и товарам в пути на склады Ozon в личном кабинете» |
| `/v1/analytics/manage/stocks` (`AnalyticsAPI_ManageStocks`, tag `BetaMethod`) | **«22 января 2026 года метод будет отключён»** (News 4 December 2025 had said 22 December 2025); still in the committed file | do not use |

#### 5.2 `/v1/product/info/stocks-by-warehouse/fbo`

- «Передайте в запросе `offer_ids` или `skus`.» Request: `limit` (uint64, **required**, maximum 1000), `cursor`, `offer_ids[]` (maximum 1000), `skus[]` (int64, maximum 1000). The schema writes `maximum` on the arrays (the sense is the count of items; the text does not say).
- Answer: `cursor`, `has_next`, `products[]{offer_id, product_id, sku, warehouse_id («Можно получить с помощью метода /v1/warehouse/ozon/list»), present («Общее количество товара на складе.»), reserved («Количество зарезервированных товаров на складе.»)}`.
  No `free` field (so free = `present` − `reserved` is an inference; not stated). No `name` of the warehouse: names come from `/v1/warehouse/ozon/list`. No own rate limit; role not documented (not in the 2024 table).
- Freshness: not documented.

#### 5.3 `/v1/analytics/stocks`

- «Используйте метод, чтобы получить аналитику по остаткам товаров на складах в реальном времени. Метод соответствует разделу **FBO → Управление остатками** в личном кабинете.» News 22 July 2026: «С 17 августа 2026 года метод возвращает информацию об остатках в реальном времени.»
  (so before that date the data were not real time; the date is past on the day read).
- Request: `skus[]` (int64, **required**, `maximum` 100), filters `cluster_ids[]` **or** `macrolocal_cluster_ids[]` («В запросе используйте только одно из полей: `cluster_ids` или `macrolocal_cluster_ids`, иначе вернётся ошибка»), `warehouse_ids[]`, `item_tags[]`, `placement_zone[]`,
  `turnover_grades[]`, `unmarked_stocks_only`. No `limit`/`cursor` in the request.
- Answer `items[]`, per SKU, warehouse and cluster: `sku`, `offer_id`, `name`, `warehouse_id`, `warehouse_name`, `cluster_id`, `cluster_name`, `macrolocal_cluster_id`, **`available_stock_count`** («Количество товаров, которые доступны к продаже. Соответствует столбцу «Доступно к продаже».»),
  **`valid_stock_count`** («Количество товаров, которые готовятся к продаже. Соответствует столбцу «Готовим к продаже».»), `requested_stock_count` (in supply requests), `transit_stock_count`, `inbound_replenishment` («в перемещении»), `outbound_pending_delivery` («которые доставляются покупателям»),
  `return_from_customer_stock_count`, `return_to_seller_stock_count`, `outbound_returns_*`, `excess_stock_count`, `expiring_stock_count`, `stock_defect_stock_count`, `transit_defect_stock_count`, `stock_not_being_sold`, `other_stock_count`, `waiting_docs_stock_count`, `waiting_docs_to_export_stock_count`, `ads`, `ads_cluster`,
  `idc`, `idc_cluster`, `days_without_sales*`, `turnover_grade*`, `item_tags[]`, `placement_zone[]`. Fields added 11 September 2026 (News): the `inbound_*`/`outbound_*` ones and `stock_not_being_sold`.
- Own rate limit: not stated. Role: not in the 2024 table (not documented); in the table the report role `Report` holds `/v1/analytics/stock_on_warehouses` and `/v2/analytics/stock_on_warehouses` (PDF pp. 20–21), `Admin read only` holds both (pp. 2–3).
- For display only, `available_stock_count` is the closest to «what can be sold», but which counter makes the «FBO stock» a customer sees: Ozon does not say; the `/v1/product/info/stocks-by-warehouse/fbo` pair (`present`, `reserved`) is the simpler one.

---

### 6. How Ozon treats the number sent

Everything below is from [JSON]; **the seller-help page that Ozon links for this subject** (`seller-edu.ozon.ru/fbs/logistics-settings/upravlenie-ostatkami/#как-меняются-остатки-в-разных-ситуациях`, Process, «Подробнее об изменении остатков») was **not read** (refused to this machine; not a file of this repository).

- **The number to send is the free quantity, without the reserved one:** warning in `/v2/products/stocks` `post.description`: «Переданный остаток — количество товара в наличии без учёта зарезервированных товаров. Перед обновлением остатков проверьте количество зарезервированных товаров с помощью метода
  /v2/product/info/stocks-by-warehouse/fbs или /v1/product/info/warehouse/stocks.» The field `stock`: «Количество товара в наличии без учёта зарезервированных товаров.»
- **Reservation:** Process, «Резерв товаров»: «Если товары заказывают юридические лица, оплата может поступать не сразу — система зарезервирует товары. Чтобы проверить остатки, используйте методы /v3/posting/fbs/get, /v3/posting/fbs/list или /v3/posting/fbs/unfulfilled/list. Если в ответе `is_legal = true`, значит среди остатков есть зарезервированные
  товары. Вы можете обновить остатки так, чтобы новое количество товаров было больше свободного остатка и зарезервированного товара в сумме. Система спишет старый остаток и рассчитает новый.» The read methods name the reserved part: `reserved` («Количество зарезервированных товаров на складе»; in `/v4`: «сколько зарезервировано покупателями»).
  Which reservations exist besides the legal-entity ones, and how long they last: **not documented**.
- **Does an order lower the seller warehouse's stock by itself?** For FBO: «Для схемы FBO остатки обновляются автоматически по факту продажи.» (Process, «Обновите цены и остатки товаров».) For FBS/rFBS: **not documented in the file** (the sentence is only about FBO; the description of the posting methods does not mention stock; the linked help page was not read).
  The existence of `reserved` and `free_stock` next to `present` on FBS warehouses, and of a warning to send the quantity «без учёта зарезервированных», points to Ozon holding back the ordered units on its side, but that is an inference, not a documented rule.
- **"Available" or "present":** the write takes the free quantity («без учёта зарезервированных»); the reads give `present` (total on the warehouse), `reserved`, and `free_stock` (available for sale) — `/v2/product/info/stocks-by-warehouse/fbs` and `/v1/product/info/warehouse/stocks` only; `/v4/product/info/stocks` gives `present` and `reserved` only. Whether `present` = `free_stock` + `reserved`: not stated.
- **At 0:** not documented for the write itself. Process: «Товар попадёт в продажу, только когда вы установите его остаток»; «Перед архивированием товара обнулите его остатки» (`/v1/product/archive`, up to 100 `product_id` per request); `/v4/product/info/stocks` has the filter `EMPTY_STOCK` («товары, у которых не указано наличие»); `has_stock`/`visibility_details.has_stock` flags exist in `/v3/product/info/list`.
- **Large goods, flammable goods, economy tariff, 3PL «Ozon логистика»:** limits of what a warehouse takes, see the error codes 1.5.
- **Warnings that could matter to a polling or writing program** (quoted): no background updates — «Убедитесь, что ваша интеграция не обновляет остатки в фоновом режиме» (`Stock is updated too frequently`); heavy or repeated or wrong requests can get the key restricted or revoked without notice (see the top); a key lives 3 months («Срок действия ключа — 3 месяца», tag `Auth`).
  Any rule «Ozon may reset a stock» or «a minimum stock»: **not found in the file.**
- **rFBS and express:** nothing about different stock rules (section 3.1). Pause of an rFBS warehouse: «Изменять сток не нужно.»

---

### 7. Differences between the committed file and the GitHub copy of 2026-11-20 (read 2026-11-21)

Method: both files read as JSON (so spacing and key order do not count), each method of sections 1–5 compared **with its `$ref` references resolved** (so a change in a shared definition shows too), plus the whole tree.

- **Every method of sections 1–5: no difference.** Compared: `/v2/products/stocks`, `/v4/product/info/stocks`, `/v2/product/info/stocks-by-warehouse/fbs`, `/v1/product/info/stocks-by-warehouse/fbs`, `/v1/product/info/warehouse/stocks`,
  `/v1/product/info/stocks-by-warehouse/fbo`, `/v2/warehouse/list`, `/v1/warehouse/list`, `/v3/product/list`, `/v3/product/info/list`, `/v1/analytics/stocks`, `/v2/analytics/stock_on_warehouses`, `/v1/analytics/manage/stocks`, `/v1/warehouse/ozon/list`, `/v1/product/digital/stocks/import`. Both files have 497 paths, the same paths, the same `info`,
  `servers`; the tags `Limits`, `Process` and `Errors` (which carry the limits, the reserve text and the error table used above) are identical.
- Differences elsewhere (none touches stock): 8 methods — image-address examples `ir.ozone.ru` → `ir-20.ozone.ru` in `/v1/description-category/attribute/values`, `/v4/product/info/attributes`, `/v1/posting/fbs/package-label/get`, `/v1/report/info`, `/v1/report/list`; description texts changed in `/v1/carriage/act-discrepancy/pdf`,
  `/v1/posting/cutoff/set`, `/v1/finance/accrual/by-day`; and one new dated entry in the tag `News`: 19 November 2026 («Обновили описание методов» for the first two and `/v1/finance/accrual/by-day`; «Обновили описание параметра `status`» for `/v1/carriage/get`). (The `/v1/carriage/get` entry changes a `status` description that the file's compared method body does not show as different; it is not stock and was not examined.)
- Consequence: nothing about stock changed between 2026-11-18 and 2026-11-20 in the GitHub copy; the committed file is as current as the copy. The latest stock-related entry in `News` is 17 November 2026: `/v2/products/stocks` and three supply-order methods «Обновили описание методов» (what changed in the text is not shown; the description read above is the one after that change).

### 7a. Dated list of the Ozon changes that matter (from [JSON, News], newest first)

| Date | Change |
| ---- | ------ |
| 23 Nov 2026 (announced 24 Sep 2026) | `total` of `/v4/product/info/stocks`, `/v5/product/info/prices` and `result.total` of `/v3/product/list` switched off; use `total_items` |
| 17 Nov 2026 | description of `/v2/products/stocks` updated (content not shown) |
| 8 Sep 2026 | the economy tariff «Эконом» ended; `/v1/product/quant/list`, `/v1/product/quant/info` removed from the documents |
| 17 Aug 2026 (announced 22 Jul 2026) | `/v1/analytics/stocks` returns stock in real time |
| 21 Jul 2026 | error `PRODUCT_IS_ARCHIVED` added for `/v2/products/stocks` |
| 9 Jul 2026 | `filter.skus` and `result.items.sku` added to `/v3/product/list` |
| 2 Jul 2026 | `/v1/product/info/stocks-by-warehouse/fbo` added |
| 7 Apr 2026 | `/v1/warehouse/list` and `/v1/product/info/stocks-by-warehouse/fbs` to be switched off; `warehouses.pause_at` added to `/v2/warehouse/list` |
| 24 Mar 2026 | `/v1/analytics/stocks` moved from beta to main |
| 12 Feb 2026 | `limit` made required in `/v2/product/info/stocks-by-warehouse/fbs` |
| 22 Jan 2026 | `/v1/analytics/manage/stocks` to be switched off |
| 27 Nov 2025 | `warehouses` of `/v4/product/info/stocks` marked deprecated; `limit`/`cursor` added to `/v2/warehouse/list` |
| 30 Oct 2025 | `/v1/product/info/warehouse/stocks` moved from beta to main |
| 26 Jun 2025 | `quant_size` removed from `/v2/products/stocks` |
| 23 Jun 2025 | error `Stock is updated too frequently` added; `TOO_MANY_REQUESTS` reworded |
| 27 May 2025 | `/v1/product/import/stocks` removed |

---

### 8. Gaps: what the documents do not say, and a real account (a test key) would have to show

Each gap below stays open until a real call is made (not allowed here; R3 says tests use the fake Ozon).

1. **Which role opens each new method** (`/v2/products/stocks` is only known in the 2024 table as `Product`; `/v4/product/info/stocks`, `/v2/product/info/stocks-by-warehouse/fbs`, `/v1/product/info/warehouse/stocks`, `/v1/product/info/stocks-by-warehouse/fbo`, `/v1/analytics/stocks`, `/v3/product/list`, `/v2/warehouse/list`: not in the table). The key's `/v1/roles` answer is the only proof.
2. **Is the `/v1/product/info/stocks-by-warehouse/fbs` method really switched off now**, and the same for `/v1/warehouse/list` (date 7 April 2026 is past)? The committed file still lists both.
3. **`quant_size`:** is a request without it accepted (all news say it was removed)? The file's `required` list still has it.
4. **Quantity limits of `stock`:** negative values; whether 0 is accepted and what 0 does to the shop window; the exact upper limit («меньше миллиона»: is 999 999 accepted?); fractional numbers cannot be sent (int64).
5. **Which error word Ozon returns for the 30-second rule** (`TOO_MANY_REQUESTS` or `Stock is updated too frequently`), and whether such an item is **partly applied** (one `result[]` row per pair with its own `errors[]` is documented; whether the whole request can fail with 429 instead is not). Whether a duplicate pair inside one request is accepted.
6. **Status words of `/v2/warehouse/list`** (`status`, `warehouse_type`): the six of the old list are not confirmed for v2. What a write to an archived, blocked, paused or still-activating warehouse returns.
7. **Does an FBS order lower the stock by itself and what exactly `present`, `reserved`, `free_stock` mean for FBS/rFBS** (whether `present` − `reserved` = `free_stock`; when `reserved` returns to free after a cancelled order; reservations other than the legal-entity kind). The linked seller-help page was not readable here.
8. **Whether the number sent is replaced or compared** («Система спишет старый остаток и рассчитает новый»): after a write of N, does `free_stock` read back N, or N minus something, and how soon (the delay between a write and the read) — not documented.
9. **Paging end markers:** `/v3/product/list` (`last_id` empty at the end?), `/v4/product/info/stocks` (`cursor` empty at the end?), `/v2/product/info/stocks-by-warehouse/fbs` (`has_next` and `cursor` both documented). Number of identifiers allowed in `/v4/product/info/stocks` filters. The own rate limits of every read method (none stated; the general 50 per second applies).
10. **Does the read of one warehouse/product row exist when no stock was ever written** (a row of 0, or no row).
11. **Article rules** (`offer_id` characters, length, case) — not in the file.
12. **Freshness** of the FBO readers other than `/v1/analytics/stocks` («в реальном времени» since 17 August 2026): not stated. Whether `/v1/product/info/stocks-by-warehouse/fbo` (`present`, `reserved`) and `/v1/analytics/stocks` (`available_stock_count`) agree.
13. **Express and rFBS stock rules**, and `has_entrusted_acceptance` — no stock rule documented.
14. **Which warehouse a product's stock belongs to when a seller has several warehouses** — the writer needs one `warehouse_id` per pair; there is no field to write one stock to all the seller's warehouses at once. One pair = one number; the wyo server therefore has to decide, per warehouse, what the fulfilment centre's free stock is split to (a decision of the plan, not a fact from Ozon).
15. **Seller cabinet «Управление остатками» page** (`seller-edu.ozon.ru`) and `dev.ozon.ru` news: refused to this machine; only the file and the PDF were read.
16. **The 2024 role table's age:** it is dated 15 August 2024 and names removed methods (`/v1/product/import/stocks`, `/v3/product/info/stocks`).

### Practical summary for the plan

- Write: `POST /v2/products/stocks`; ≤ 100 pairs per request; ≤ 80 requests per minute per seller account; a pair only once per 30 seconds; answers are per item inside a 200 (`updated`, `errors[]`); send `offer_id` **or** `product_id` and `warehouse_id` and `stock`; do **not** send `quant_size`; role `Product` (2024 table).
- Read per warehouse and product: `/v2/product/info/stocks-by-warehouse/fbs` (≤ 1 000 `sku` or `offer_id`, `limit` ≤ 1 000, cursor; `present`, `reserved`, `free_stock`) or `/v1/product/info/warehouse/stocks` (one warehouse, cursor, `updated_at`). Per product only: `/v4/product/info/stocks` (all schemes, no warehouse id).
- FBO for display: `/v1/product/info/stocks-by-warehouse/fbo` (`present`, `reserved` per Ozon warehouse; names from `/v1/warehouse/ozon/list`) or `/v1/analytics/stocks` (≤ 100 `skus`, real time since 17 August 2026, many counters).
- Warehouses: `/v2/warehouse/list` (≤ 200 per page, cursor); `is_rfbs`, `is_express`, `has_entrusted_acceptance`, `pause_at` (null = active, rFBS only), `status` (values undocumented for v2).
- Product match: `/v3/product/list` (≤ 1 000 per page, `last_id`) and `/v3/product/info/list` (≤ 1 000 identifiers in total; `barcodes[]`, `sku`, `id`, `offer_id`, `stocks`).
- Stale or ending: `/v1/product/info/stocks-by-warehouse/fbs`, `/v1/warehouse/list` (end date 7 April 2026, past), `/v1/analytics/manage/stocks` (22 January 2026, past), `/v2/analytics/stock_on_warehouses` (no date), `total` fields (23 November 2026).

## Part Y — Яндекс Маркет
> Research for the stock plan (showing what lies in each marketplace's own warehouses; sending a fulfilment centre's free stock to the
> seller's own FBS / Express warehouses). Cited «YMC» = `/home/igor/projects/wyo/.agents/research/yandex-market-official-partner-api-check-2026-11-19.md`.
> Nothing was sent to the API: only documentation pages (`.md` versions), the description files and the GitHub repository were read.

### 0. Sources and how they were read

- **S1 — the committed description tree** `/home/igor/projects/wyo/api/yandex/` = commit `9be9f98e111133efdb31f7260bdb27358b378b29`
  (SHA-256 of `openapi.yaml` `16157bbf…15ae`, re-checked equal to `git show 9be9f98e…:openapi/openapi.yaml`). Paths are cited as
  `paths/<file>:<line>`. Server: `https://api.partner.market.yandex.ru` (`openapi.yaml:26-27`). Authorization: header `Api-Key`
  (`openapi.yaml` securitySchemes, ~line 357).
- **S2 — documentation pages**, read on 2026-11-21 as Markdown, all answered HTTP 200 (none refused):
  `https://yandex.ru/dev/market/partner-api/doc/ru/` + `step-by-step/stocks.md`, `step-by-step/warehouses.md`, `step-by-step/reports.md`,
  `concepts/limits.md`, `concepts/error-codes.md`, `concepts/pagination.md`, `reference/stocks/{updateStocksOnPartnerWarehouses,getStocksOnPartnerWarehouses,updateStocks,getStocks}.md`,
  `reference/reports/{generateStocksReport,generateStocksOnWarehousesReport}.md`, `reference/warehouses/getPartnerWarehouses.md`,
  `reference/business-offer-mappings/getOfferMappings.md`, `_auto/scopes_summary/pages/{offers-and-cards-management,inventory-and-order-processing}.md`.
  Copies were kept for the day only (not in the repository).
- **S3 — the seller help article** (Яндекс Маркет's own «Справка для продавцов»), «Как управлять остатками»:
  `https://yandex.ru/support/marketplace/ru/assortment/operations/stocks.html`, read 2026-11-21 (HTTP 200, page text extracted).
  It is the page the API documents point to for «all the general rules of passing stock apply to the API too»
  (`step-by-step/stocks.md`). The anchor `#unified-stocks` (what warehouse groups are) was NOT found in the text of that page.
- **S4 — the newest commit of the GitHub repository**: `b71cd4391c4e3a1e0819ace67806a4ec35b970ba` of 2026-11-20 14:05:17 +0300, cloned into
  (a copy kept for the day), compared with `9be9f98e…` (section 6).

### 1. Writing stock to a seller warehouse

#### 1.1 Which method is current

There are two writing methods; **neither is deprecated** (no `deprecated: true`, no `x-deprecation-config` in either file; checked by
`grep -i deprecated` over both path files). The choice is by whether the cabinet has **groups of warehouses**, not by age:

| Cabinet | Method | operationId | Source |
| --- | --- | --- | --- |
| **no groups of warehouses** | `POST https://api.partner.market.yandex.ru/v3/businesses/{businessId}/offers/stocks/update` | `updateStocksOnPartnerWarehouses` | `paths/v3_businesses_businessId_offers_stocks_update.yaml:1-29`; S2 `step-by-step/stocks.md` |
| **groups of warehouses exist** | `PUT https://api.partner.market.yandex.ru/v2/campaigns/{campaignId}/offers/stocks` | `updateStocks` | `paths/v2_campaigns_campaignId_offers_stocks.yaml:1-29`; S2 |

Quotes (S1/S2): v3 — «Метод подходит, только если в кабинете нет групп складов»
(`paths/v3_businesses_businessId_offers_stocks_update.yaml:~38`); v2 — «Метод актуален только для кабинетов с группами складов. Если в
кабинете нет групп складов, используйте метод POST v3/businesses/{businessId}/offers/stocks/update»
(`paths/v2_campaigns_campaignId_offers_stocks.yaml`, description of `put`). v2 is marked in the file `x-apphost … enabled: false`
(internal flag, not a deprecation). Models that accept both: FBS, Express, DBS (`tags: stocks, fbs, dbs, express`; S2 «Метод доступен для
моделей: FBS, Экспресс и DBS»). Both: «Данные в каталоге обновляются не мгновенно. Это занимает до нескольких минут.» (S1, S2).

**A third, spreadsheet/CMS way exists** and may be used at the same time: «вы можете передавать остатки … с помощью API и в кабинете
через YML или через Excel-файлы» (S2 `step-by-step/stocks.md`). Whole-file Excel updates have a mode «полностью — для тех товаров, которых
не будет в файле, остатки обнулятся» (S3) — a manual update can therefore zero what the API sent.

#### 1.2 Body — v3, no groups (`UpdateStocksOnWarehousesRequest`)

`components/schemas/UpdateStocksOnWarehousesRequest.yaml:1-18`, `UpdateStockOnWarehouseItemDTO.yaml:1-37`, `ShopSku.yaml`:

```
{ "skuItems": [ { "sku": "<your SKU>", "partnerWarehouseId": <int64>, "count": <int>, "updatedAt": "<ISO 8601 with offset>" } ] }
```

- `skuItems`: required, **1 to 2000** items (`minItems: 1`, `maxItems: 2000`, line 16). «все пары `sku` и `partnerWarehouseId` должны быть уникальными».
- `sku`: **the seller's own SKU** (the field `offerId` in the catalog methods; here named `sku`), string 1–255 characters
  (`ShopSku.yaml`: `minLength 1`, `maxLength 255`, pattern `^(?=.*\S.*)[^\x00-\x08\x0A-\x1f\x7f]{1,255}$`), whitespace at both ends is trimmed
  automatically («`"  SKU123  "` и `"SKU123"` будут обработаны как одинаковые»). «Обязательно указывайте SKU **в точности** так, как он указан в
  каталоге. Например, 557722 и 0557722 — это два разных SKU.» A SKU «нельзя освободить и использовать заново для другого товара» (`ShopSku.yaml`).
- `partnerWarehouseId`: **required**, int64, min 1; «Идентификатор склада, на котором требуется обновить остатки» — got from
  `POST /v3/businesses/{businessId}/warehouses` (section 3). So the stock belongs to a **warehouse of the cabinet**, not to a shop.
- `count`: **required**, int64, **0 to 2 000 000 000**, «Количество доступного товара».
- `updatedAt`: optional, date-time «ISO 8601 со смещением относительно UTC. Например, `2017-11-21T00:42:42+03:00`»;
  «Если вы не передали параметр `updatedAt`, используется текущее время.»
- The path parameter `businessId` (`components/parameters/BusinessId.yaml`, int64, min 1) is the cabinet number (from `GET /v2/campaigns`).
- Request body at most **512 KB** for every method (S2 `concepts/limits.md`, «Функциональные ограничения»; error «The request is too big»).

#### 1.3 Body — v2, groups (`UpdateStocksRequest`)

`components/schemas/UpdateStocksRequest.yaml:1-17`, `UpdateStockDTO.yaml`, `UpdateStockItemDTO.yaml`:

```
{ "skus": [ { "sku": "<your SKU>", "items": [ { "count": <int>, "updatedAt": "<date-time>" } ] } ] }
```

- Identifier is the **shop** (`campaignId` in the path, int64 min 1) — «Идентификатор кампании (магазина)»; no warehouse id in the body.
- `skus`: **1 to 2000** (`maxItems: 2000`), `sku` unique in one request («Не допускается передача двух объектов с одинаковым `sku`»).
- `items`: exactly **1** (`minItems: 1`, `maxItems: 1`, `UpdateStockDTO.yaml:13-14`), with `count` 0–2 000 000 000 and optional `updatedAt`.
- «Для группы складов передавайте остатки только для **одного любого склада**. Информация для остальных складов в этой группе обновится
  автоматически.» (`paths/v2_campaigns_campaignId_offers_stocks.yaml`; S2 `step-by-step/stocks.md`, `step-by-step/warehouses.md`). A shop's
  campaign id for a group: «укажите идентификатор кампании любого магазина в этой группе» (S2 `step-by-step/warehouses.md`, section «Узнать группы складов»).
- Answer `200` with an empty body (`EmptyApiResponse`, `responses/200.yaml`: «Пустой ответ»); the v3 file lists the `200` description «Пустой ответ.»

#### 1.4 Limits (S1 `x-resource-limit-config`, confirmed by S2 pages)

| Method | Limit | Counted by | Source |
| --- | --- | --- | --- |
| v3 update | **50 requests a minute** (same «без подписки» and with the paid «Медиум» subscription) | request (`path: resp`, cost 1) | `paths/v3_…_offers_stocks_update.yaml:16-24`; S2 «⚙️ Лимит: 50 запросов в минуту» |
| v2 update | **100 000 items a minute** (same both tiers) | `req.skus` — the size of the collection sent | `paths/v2_campaigns_campaignId_offers_stocks.yaml:16-24`; S2 «Лимит: 100 000 товаров в минуту» |

- Per-item rule for «by volume» limits: «если ограничение по объему данных — стоимость равна размеру коллекции в запросе или ответе»
  (S2 `concepts/limits.md`, «Уменьшение ресурсного лимита»). **Inference:** v3 at full size is 50 × 2000 = 100 000 items a minute, the same ceiling.
- Parallel requests: «Разрешается выполнять не более четырёх одновременных запросов», applied per cabinet for `/businesses/{businessId}/…`
  paths and per shop for `/campaigns/{campaignId}/…` paths (S2 `concepts/limits.md`, «Глобальные ограничения»).
- **Limit headers, in every response:** `X-RateLimit-Resource-Limit` (the numeric limit), `X-RateLimit-Resource-Until` (the date the limit
  applies to, RFC 822, e.g. `Thu, 10 Jul 2018 00:42:42 GMT`), `X-RateLimit-Resource-Remaining` (what is left) (S2 `concepts/limits.md`).
- A met limit: HTTP **`420 Enhance Your Calm`**; bodies such as «Hit rate limit of 4 parallel requests for campaignId 12345» and
  «Hit rate limit of 'N' requests per 'period' for resource 'R' for {entity}» (S2 `concepts/error-codes.md`, section 420, lines 614-623).
- What uses up the limit: 2xx and 4xx do; 5xx does not (S2 `concepts/limits.md`; also YMC §4).
- Update frequency advice: «Передавать остатки нужно как можно чаще — хотя бы раз в день.» «с момента передачи данных до изменений на
  Маркете проходит до 15 минут» (S3). (The API pages say «до нескольких минут» — the two documents differ; neither is a guarantee.)

#### 1.5 Documented errors (S2 `concepts/error-codes.md`, section «Ошибки при работе с остатками» `#stocks`, lines 288-307)

All are HTTP `400 Bad Request` (the v3 and v2 files point their `400` to this section: `…_update.yaml` response `400`, `responses/400Stocks.yaml`):

| Message | Meaning (the page's translation) | What the page says to do | Methods |
| --- | --- | --- | --- |
| `Duplicate shop-sku 'sku' for warehouse 'count'` | duplicate shop-sku for the warehouse | check the uniqueness of `sku` values | v2 update, v3 update |
| `Stock for 'sku' should not be in the future` | `updatedAt` must not be in the future | set `updatedAt` no later than now | v2 update, v3 update |
| `Stock for 'sku' should not be older than a day` | `updatedAt` must not be older than 24 hours | send a current `updatedAt` (not older than 24 h) | v2 update, v3 update |
| `Forbidden to use Shop SKUs with any other filters.` | a SKU list cannot be combined with other filters | do not send a SKU list with `archived` | v2 getStocks, v3 getStocks |
| `Warehouse with id 'warehouseId' not found or is not accessible` | warehouse not found or not available to the cabinet | check the warehouse id | v2 getStocks, v3 getStocks |
| `Offers not found for SKUs: 'offerIds'` | non-existent product identifiers | check `offerIds` | listed for **v2 update** (`PUT /v2/campaigns/{campaignId}/offers/stocks`), not for v3 update |

Common 400 messages that also apply (same page, lines 59-106): `'field' must be less than or equal to 'value'` (e.g. `count` over 2 000 000 000),
`'field' must be greater than or equal to 'value'`, `'field' size must be between 'min' and 'max'` (items over 2000), `Illegal input at 'path'`,
`JSON: {message}`, `The request is too big: …` (over 512 KB), `Required request body is missing`. Other codes: **401**
(`Api-Key token is invalid / revoked / not specified …`, lines 418-450), **403** (`Access denied`; `API for campaign 'campaignId' disabled …`
for inactivity, no active contract, no placement program, manually disabled; lines 451-506), **404**, **420** (limits), **423 Locked**
(«Метод невозможно использовать для этого магазина»), **500**, **503** (S2 `concepts/error-codes.md`). Each of these maps to the
HTTP status that the path files list: `400, 401, 403, 404, 420, 500` (v3 update, v2 update).

#### 1.6 Access level for writing

- **`offers-and-cards-management`** — «Управление товарами и карточками» — or **`all-methods`** — «Полное управление кабинетом». Both
  write methods list exactly these two and **no read-only level** (`paths/v3_…_offers_stocks_update.yaml:10-14`;
  `paths/v2_campaigns_campaignId_offers_stocks.yaml:10-14`; S2 each page «необходим один из доступов в списке»; S2
  `scopes_summary/pages/offers-and-cards-management.md`, section «Остатки и оборачиваемость», lines 147-170).
- **NOT `inventory-and-order-processing`.** That level («Обработка заказов и учёт товаров») does not open any stock method
  (grep of S2 `scopes_summary/pages/inventory-and-order-processing.md`: only warehouse methods). The wyo server today treats
  `OFFERS_AND_CARDS_MANAGEMENT` as «нет изменения заказов, да чтение товаров» (`/home/igor/projects/wyo/docs/yandex.md:186-200`); writing stock will need
  that level to be treated as a write level, as the orders write level is `INVENTORY_AND_ORDER_PROCESSING`.
- The warehouse list the write needs (`getPartnerWarehouses`) opens with `inventory-and-order-processing` (and `…:read-only`) or `all-methods`
  (section 3) — a key with only `offers-and-cards-management` can write stock but cannot list the warehouse ids (**Inference** from the two lists).

### 2. Reading stock

#### 2.1 The shop's own warehouses (FBS / Express / DBS)

- **No groups of warehouses:** `POST /v3/businesses/{businessId}/offers/stocks` (`getStocksOnPartnerWarehouses`,
  `paths/v3_businesses_businessId_offers_stocks.yaml`). Query: `pageToken` (string, from the earlier `paging.nextPageToken`), `limit`
  (**default 50, min 1, max 100**, a larger value is reduced to 100: `PageLimit100Truncate.yaml`; S2). Body `GetStocksOnPartnerWarehousesRequest`:
  `partnerWarehouseId` (required, int64 ≥ 1), `archived` (bool; default = not archived), `offerIds` (1 to **500** SKUs, unique, nullable):
  «Возвращается информация об остатках всех переданных SKU, включая товары в архиве»; «Такой список возвращается только целиком … не заполняйте
  `pageToken`, `limit`, `archived`» — a SKU list together with `archived` gives the 400 `Forbidden to use Shop SKUs with any other filters.`
  Answer: `result.partnerWarehouseId`, `result.offers[]` { `offerId`, `stocks[]` { `type`, `count` }, `updatedAt` }, `result.paging.nextPageToken`
  (`GetStocksOnPartnerWarehousesDTO.yaml`, `PartnerWarehouseOfferDTO.yaml`).
  **Limit: 500 requests a minute** (`:18-26`; S2). **Access:** `offers-and-cards-management`, `offers-and-cards-management:read-only`,
  `all-methods`, `all-methods:read-only` (`:10-17`).
- **Groups of warehouses (and FBY, LaaS):** `POST /v2/campaigns/{campaignId}/offers/stocks` (`getStocks`,
  `paths/v2_campaigns_campaignId_offers_stocks.yaml:66-93`). Same `pageToken`/`limit` (default 50, max 100). Body optional
  (`GetWarehouseStocksRequest`): `stocksWarehouseId` (int64), `hasStocks` (FBY/LaaS only; use only with `stocksWarehouseId`; `false` = out of
  stock, `true` = in stock), `withTurnover` (FBY/LaaS only; default `false`), `archived`, `offerIds` (1–500). Answer: `result.warehouses[]`
  { `warehouseId`, `offers[]` { `offerId`, `stocks[]`, `turnoverSummary`, `updatedAt` } }, `paging`.
  **Limit: 100 000 products a minute**, counted by `resp.result.warehouses.offers.offerId` — the offers returned (`:75-84`; S2 «Лимит: 100 000 товаров в минуту»).
  **Access:** the same four as above. «Для модели FBS: в ответе может вернуться не только партнерский склад, но и склад возвратов Маркета. Это
  возможно, если возврат поступил в указанную продавцом точку возвратов и долго не был забран.» The method is current «для моделей FBY и
  LaaS; для моделей FBS, DBS и Экспресс, если в кабинете есть группы складов» (description of the `post`).

#### 2.2 The stock types in the answer (`WarehouseStockType.yaml`; `WarehouseStockDTO`: `type` + `count` int64, both required)

| `type` | Meaning in the document |
| --- | --- |
| `AVAILABLE` | «Доступный к заказу» (the name in the cabinet's report «Остатки на складе») — goods available for sale |
| `FIT` | «Годный» — goods «доступен для продажи или уже зарезервирован» (available **or already reserved**) |
| `FREEZE` | goods «зарезервирован для заказов» (reserved for orders) |
| `QUARANTINE` | «Карантин» — temporarily not for sale (e.g. moved from one room of the warehouse to another) |
| `DEFECT` | «Брак» — defective |
| `EXPIRED` | «Просрочен» — expired |
| `UTILIZATION` | to be disposed of |

The seller help confirms the same words for FBY (S3): «Годные — пригодные для продажи товары, которые сейчас на складе — доступные для
заказа плюс уже заказанные, но еще не отгруженные (резерв)»; «Резерв — заказанные покупателями товары, которые пока еще не передали в
доставку»; «Доступно для заказа — количество товаров, которое Маркет показывает на витрине покупателям». **Inference:** `AVAILABLE` = `FIT` − `FREEZE`;
the documents do not state the formula. **Not documented:** which of the seven types an FBS/Express warehouse actually returns (the schema is the same
for every model; the example in S2 shows only `"stocks": [null]`).
`turnoverSummary` (FBY/LaaS, only with `withTurnover: true`): `turnover` (`LOW` ≥120 days, `ALMOST_LOW` 100–120, `HIGH` 45–100, `VERY_HIGH` 0–45,
`NO_SALES`, `FREE_STORE`) and `turnoverDays` (number) (`TurnoverDTO.yaml`, `TurnoverType.yaml`).

#### 2.3 Market's own (FBY / LaaS) stock for display

`POST /v2/campaigns/{campaignId}/offers/stocks` of the **FBY shop's** `campaignId` (the campaign list `GET /v2/campaigns` gives each shop's
`placementType`: `FBS` (FBS or Express), `FBY`, `DBS`, `LAAS` — `components/schemas/PlacementType.yaml`), with `stocksWarehouseId` from
`GET /v2/warehouses` (`getFulfillmentWarehouses`, section 3): «Для моделей FBY и LaaS: информация об остатках может возвращаться с нескольких
складов Маркета, у которых будут разные `warehouseId`».

#### 2.4 Reports of stock on warehouses (generate, then fetch)

| Report | Method | Limit | Access | For whom |
| --- | --- | --- | --- | --- |
| business-wide, no groups | `POST /v3/businesses/{businessId}/reports/stocks/generate` (`generateStocksReport`) | **1 request in 2 minutes** (Медиум subscription: 1 a minute) | the same four as the stock read methods | all shops of the cabinet, FBS/DBS/Express, no groups |
| with groups, or FBY/LaaS | `POST /v2/reports/stocks-on-warehouses/generate` (`generateStocksOnWarehousesReport`) | **1 request in 2 minutes** (Медиум: 1 a minute) | same four | FBY/LaaS and cabinets with groups |

(`paths/v3_…_reports_stocks_generate.yaml:10-31`, `paths/v2_reports_stocks-on-warehouses_generate.yaml:12-33`; S2 «Лимит без подписки: 1 запрос в 2 минуты».)

- Query `format`: `FILE` (XLSX, the default), `CSV` (ZIP of CSV files, one per sheet) or `JSON` (ZIP of JSON files) (`ReportFormatType.yaml`).
- Body v3 (optional): `categoryIds`, `hasStocks`, `partnerWarehouseIds` (`GenerateStocksReportRequest.yaml`). Body v2: **either** `businessId` **or**
  `campaignId`/`campaignIds`, not both; `warehouseIds`, `reportDate` (FBY/LaaS; data of the **previous** day, `ГГГГ-ММ-ДД`), `categoryIds`,
  `hasStocks`, `campaignIds` (`GenerateStocksOnWarehousesReportRequest.yaml`). For the shop warehouse `campaignId` «скоро станет недоступен» —
  use `businessId` + `campaignIds`.
- Answer: `result.reportId` (string) and `result.estimatedGenerationTime` (ms) (`GenerateReportDTO.yaml`). Then `GET /v2/reports/info/{reportId}`
  (`getReportInfo`, limit **100 a minute**, `paths/v2_reports_info_reportId.yaml:44-52`) gives `status` `PENDING` / `PROCESSING` / `FAILED` / `DONE`, sub-status
  `NO_DATA` / `TOO_LARGE` / `RESOURCE_NOT_FOUND`, `generationRequestedAt`, `generationFinishedAt`, `file` (the link). **The link lives 60 minutes**; every
  info request makes a new one («сразу после получения ссылки скачайте отчет … Не сохраняйте ссылку»; `ReportInfoDTO.yaml`). Error 420
  `Active reports limit exceeded, limit='limit'` when too many are generated at once (S2 `concepts/error-codes.md`, section 420).
- Columns, sheet «Остатки на складах» (`stocks_business`, S2 `generateStocksReport.md`): `SHOP_SKU` (your SKU), `PRODUCT_NAME`,
  `PLACEMENT_TYPE`, `WAREHOUSE_AND_SHOP`, `COUNT` («Доступно для заказа»), `RESERVE` («Резерв»), `PRICE`, `STATUS`, `ERRORS`, `COMMENT`.
  For Market's warehouses (`stocks_on_warehouses`, S2 `generateStocksOnWarehousesReport.md`): `SHOP_SKU`, `ARTICLE` (your SKU), `MARKET_SKU`,
  `PRODUCT_NAME`, `VALID`, `RESERVED`, `AVAILABLE_FOR_ORDER`, `QUARANTINE`, `UTILIZATION`, `DEFECT`, `EXPIRED`, sizes, `WAREHOUSE`, `SELLING_STATUS`, `RECOMMENDATIONS`.
- **When a report is preferred:** the documents do not say. **Inference** from the numbers: the report carries the reserve (`RESERVE`) of every shop and
  warehouse of the cabinet in one file, but one request in 2 minutes and a minutes-long wait; the read method answers at once, 100 products a page, up to 500 SKUs
  by list, 500 requests a minute (v3) — better for a screen that must be current.

### 3. Which warehouses take stock

| Method | Path | Notes | Limit | Access | Source |
| --- | --- | --- | --- | --- | --- |
| `getPartnerWarehouses` | `POST /v3/businesses/{businessId}/warehouses` | for cabinets **without** groups; «возвращает только отдельные склады и не возвращает группы складов»; `limit` default 15, max 30; body (optional): `warehouseIds`, `components` (`ADDRESS`) | **100 requests an hour** | `inventory-and-order-processing`, `…:read-only`, `all-methods`, `all-methods:read-only` | `paths/v3_businesses_businessId_warehouses.yaml:10-60`; `PageLimit30.yaml` |
| `getPagedWarehouses` | `POST /v2/businesses/{businessId}/warehouses` | for cabinets **with** groups; body: `campaignIds` (≤25), `components` (`ADDRESS`, `STATUS`); `limit` default 15, max 30 | **100 an hour** | same four | `paths/v2_businesses_businessId_warehouses.yaml:81-132`; `GetPagedWarehousesRequest.yaml` |
| `getWarehouses` | `GET /v2/businesses/{businessId}/warehouses` | **deprecated**: degradation starts **2027-01-18**, shutdown **2027-04-05**, replacement `getPagedWarehouses`; the answer separates `warehouses` (not in groups) and `warehouseGroups` (each with `mainWarehouse`) | 100 a minute | same four | `paths/v2_businesses_businessId_warehouses.yaml:10-51` (`x-deprecation-config` lines 31-35); `WarehousesDTO.yaml`, `WarehouseGroupDTO.yaml` |
| `getFulfillmentWarehouses` | `GET /v2/warehouses` | **Market's own** (FBY/LaaS) warehouses: `id`, `name`, `address`; optional `campaignId` | 100 an hour | many levels, incl. `offers-and-cards-management(:read-only)` and `inventory-and-order-processing(:read-only)` | `paths/v2_warehouses.yaml`; `FulfillmentWarehouseDTO.yaml` |

Telling the kinds apart:

- **`getPartnerWarehouses` answer** (`PartnerWarehouseDTO.yaml`, `PartnerWarehouseModelDTO.yaml`, `WarehouseProgramType.yaml`): per warehouse `id` (int64),
  `name`, `models[]`, optional `address`; each model has `placementType` — **`FBS`, `DBS` or `EXPRESS`** — `campaignId` («Идентификатор магазина, связанного с этой
  моделью работы склада … Возвращается, если связь с магазином однозначно определена») and `apiAvailability` (`AVAILABLE`,
  `DISABLED_BY_INACTIVITY` — no goods on the showcase for over 90 days, `DISABLED_BY_NO_ACTIVE_CONTRACT`, `MANUALLY_DISABLED`,
  `DISABLED_BY_NO_PLACEMENT_TYPE`; `ApiAvailabilityStatusType.yaml`). **FBY is not among them**: the list holds only the cabinet's own warehouses; FBY has
  its own method above. All three models (FBS, DBS, Express) are tagged on the stock methods.
- **`getPagedWarehouses` answer** (`WarehouseDetailsDTO.yaml`): `id`, `name`, `campaignId`, `express` (bool, «Возможна ли доставка для модели Экспресс»), `groupInfo`
  { `id`, `name` } (only for warehouses inside a group: «Склады с одинаковым `id` в `WarehouseGroupInfoDTO` входят в одну группу»), `address`, `status`
  (`DISABLED_MANUALLY` or `OTHER`, only with `components: STATUS`). No `placementType` — the FBS / DBS / Express split here is by the shop's `campaignId`
  and the `express` flag.
- **Which kinds accept stock from the seller:** the stock methods are tagged **`fbs, dbs, express`** (`paths/…offers_stocks_update.yaml:5-8`) and S2 «Метод доступен для
  моделей: FBS, Экспресс и DBS». **FBY / LaaS: no** — «Исключение — модели FBY и LaaS, потому что товары хранятся на складе Маркета и он сам может их
  пересчитать» (S2 `step-by-step/stocks.md`), and «Делать это нужно всем магазинам — кроме FBY-магазинов» (S3).
- **How to know if a cabinet has groups:** the page says to call `getPagedWarehouses` with `campaignIds` and look at `groupInfo` (S2 `step-by-step/warehouses.md`,
  «Узнать группы складов для передачи остатков»). The business settings (`POST /v2/businesses/{businessId}/settings`, `getBusinessSettings`, 100 an hour, any level)
  also hold `warehouseModel`: `CAMPAIGN` — «интерфейс складов на уровне магазинов», `WAREHOUSE` — «интерфейс единых складов на уровне кабинета»
  (`BusinessWarehouseModelType.yaml`, `BusinessSettingsDTO.yaml`). **Not documented:** that `warehouseModel` equals «has groups» (an **Inference** only).
- A warehouse's model can be switched off by the seller or by Market: `POST /v3/businesses/{businessId}/warehouse/models/status` (`updateWarehouseModelStatus`, 100 an hour,
  `all-methods`, `inventory-and-order-processing`); after the switch-off the goods of that model «скрываются через 15 минут»; switched on «возвращаются … через 15
  минут, а если модель была выключена 30 дней или дольше — через 4 часа» (S2 `step-by-step/warehouses.md`). It is not stock, but the wyo server must not send stock to a
  model that is switched off (the document says nothing about what Market answers then).

### 4. How a product is matched — `POST /v2/businesses/{businessId}/offer-mappings` (`getOfferMappings`)

`paths/v2_businesses_businessId_offer-mappings.yaml:1-90`, S2 `reference/business-offer-mappings/getOfferMappings.md`:

- **Seller's SKU** = `offerId` in the answer (`result.offerMappings[].offer.offerId`, `ShopSku.yaml`, 1–255 characters). The same string is `sku` in the stock write and `offerId`
  in the stock read — one identifier across the three methods.
- **Barcodes:** `offer.barcodes` (array of strings, nullable; EAN-13, EAN-8, UPC-A, UPC-E, Code 128, ISBN; «внутренние штрихкоды, начинающиеся на 2 или 02, и коды формата
  Code 128 не являются GTIN», `OfferBarcodes.yaml`). Barcodes are **not** an input to the stock methods.
- **Market's SKU:** `mapping.marketSku` (int64, min 1, «Идентификатор карточки на Маркете»; may be missing when the product is not yet bound to a card) with `marketSkuName`,
  `marketCategoryId`, `marketCategoryName` (`GetMappingDTO.yaml`, `UpdateMappingDTO.yaml`, `MarketSku.yaml`).
- Also: `offer.archived` (bool), `offer.cardStatus`, `offer.campaigns[]` { `campaignId`, `status` } where `status` can be `NO_STOCKS` («Нет на складе»), `PUBLISHED`, `ARCHIVED`
  and others (`OfferCampaignStatusType.yaml`).
- Request body (optional): `offerIds` (1 to **100**, unique; «Такой список возвращается только целиком» — do not combine with `pageToken`, `limit`, filters), `cardStatuses`, `categoryIds`,
  `vendorNames`, `tags`, `archived`. No body = all products, paged. Query: `pageToken`, `limit` (default 50, max 100, reduced to max), `language` (`RU` default) (`GetOfferMappingsRequest.yaml:8`).
- **Limit:** **100 requests a minute** (no subscription), **600 a minute** with the «Медиум» subscription (`paths/…offer-mappings.yaml:33-43`; S2).
- **Access:** `offers-and-cards-management`, `…:read-only`, `all-methods`, `all-methods:read-only` (`:20-25`).

### 5. How Яндекс Маркет treats the number sent

**5.1 Who lowers it.** Two documents, said in different words:

- S2 `step-by-step/stocks.md`, «Какое число передавать»: «Остаток товара — это число единиц, **доступных для заказа на Маркете**. Когда покупатель делает заказ, число доступных
  товаров уменьшается, — поэтому и остатки тоже. Например, если было 20 и кто-то заказал 2 единицы, остаток составит 18.» Sales outside Market also: «В любом случае сразу
  уменьшайте остаток товара.»
- S3 «Какое число передавать»: «В остатках нужно передавать количество свободных для новых заказов товаров — сколько их фактически есть на складе и готово к продаже.»
  «Если покупатель на Маркете создаст новый заказ или отменит его до отгрузки, **данные на витрине обновятся автоматически**.» The table of events in S3 reads:
  - new order for 2 units: Market «добавляет в резерв заказанное количество» and «уменьшает значение доступных на витрине товаров на это количество» (6 → 4 on the showcase);
  - the column «Что нужно передать в остатках» of the same table says **«Передавайте только количество товаров, доступное для продажи»** (4) for the first order and
    **«Вычтите из остатков количество заказанных товаров»** (3, 1) for the next ones — the two instructions are in the same table and can be read as contradicting each other;
  - order handed to delivery: for DBS Market lifts the reserve; **for FBS and Express «не снимает резерв, пока заказ не будет доставлен»**; the seller sends «то же количество, что вы передали после получения заказа»;
  - FBS order cancelled **after** packing («Отменен после Готов к отгрузке»): Market «оставляет товары в резерве, пока вы не передадите остатки — чтобы у вас было время на их
    распаковку»; the seller unpacks, adds the freed units and sends the new value — «Чтобы освободившийся товар вернулся на витрину, вам нужно распаковать его и передать
    актуальные данные об остатках»;
  - a cancel before packing / before delivery: Market lifts the reserve itself and the showcase rises by the freed units.
  **Summary:** Market reserves an ordered quantity itself and lowers what the showcase shows, but the documents do not say plainly whether the number sent next already
  has to exclude the seller's own reserved (unshipped) orders. The wyo plan must settle this against a real cabinet (section 7, item 1).

**5.2 `updatedAt`.** «Дата и время последнего обновления информации об остатках» / «момент, по состоянию на который новое значение актуально» (S1 `UpdateStockItemDTO.yaml`;
S2 `step-by-step/stocks.md`, «Каждая запись включает … Дату и время»). Omitted → the current time (S1). The 400 errors above fix its range: **not in the future** and
**not older than 24 hours**. **Not documented:** that Market ignores an update older than the stored one (no such sentence in any page read). The step-by-step page for groups
describes `updatedAt` as the moment the value is correct, which suggests ordering, but only as an **Inference**.

**5.3 Zero.** `count: 0` is valid (`minimum: 0`). The catalog status `NO_STOCKS` («Нет на складе») exists for a product in a shop (`OfferCampaignStatusType.yaml`). The page of Excel
updates says a **full** file update zeroes every product absent from the file (S3). **Not documented:** whether 0 hides the product from the showcase at once, or how long that takes.

**5.4 Frequency / staleness.** «Передавать остатки нужно как можно чаще — хотя бы раз в день.» (S3). **Not documented:** any rule «a stock not updated for N days is reset»
(no page read says so: grep over the five stock pages and the three step-by-step pages for «обнул», «не обновл», «дней», «суток» finds only the 24-hour limit of `updatedAt`
and turnover days). Showcase delay: up to «нескольких минут» (S2) / «до 15 минут» (S3).

**5.5 Express / minimum.** No Express-specific stock rule and no minimum other than `count ≥ 0` (S2/S3: the only mention of Express is the reserve staying until delivery, 5.1).

**5.6 The test the documents suggest** (S2 `step-by-step/stocks.md`, «Как проверить работоспособность интеграции»): hide the goods (`POST /v2/campaigns/{campaignId}/hidden-offers`),
send stock, then unhide (`POST /v2/campaigns/{campaignId}/hidden-offers/delete`). Orders created on the cabinet's «Тестовый заказ» carry `fake: true` (YMC §9).

### 6. Differences between the committed tree and the newest commit

The newest commit of `https://github.com/yandex-market/yandex-market-partner-api` on 2026-11-21 is `b71cd4391c4e3a1e0819ace67806a4ec35b970ba` (2026-11-20 14:05:17 +0300, «Update
2026-11-20T11:05:17Z»). `git diff --name-status 9be9f98e… b71cd43 -- openapi`: 35 added files and 2 changed files (the commits between: `4ca8b76`, `b71cd43`).

- **Added:** the size-table methods (`/v2/businesses/{businessId}/size-tables…`, 5 paths) with their schemas and the parameter `PageLimit100Default25.yaml`; none touches stock.
- **Changed:** `openapi/openapi.yaml` (the five new paths only) and `components/schemas/OrderSubstatusType.yaml` (the value `COURIER_DELIVERY_POSTPAID_FAILED` removed).
- **Per method of sections 1-4: none.** No change in `updateStocksOnPartnerWarehouses`, `updateStocks`, `getStocksOnPartnerWarehouses`, `getStocks`, `generateStocksReport`,
  `generateStocksOnWarehousesReport`, `getReportInfo`, `getPartnerWarehouses`, `getPagedWarehouses`, `getWarehouses`, `getFulfillmentWarehouses`, `getOfferMappings`, nor in any of their
  schemas (`git diff --name-only … | grep -i -E "stock|warehouse|offer|report|campaign|settings|Turnover|ShopSku|PackagingForward|ApiAvail|responses/"` prints nothing but the new parameter file).
- There is **no other path file with `stocks` in its name** besides the five of the task list (`ls paths | grep -i stock`: `v2_campaigns_campaignId_offers_stocks`, `v2_reports_stocks-on-warehouses_generate`,
  `v3_businesses_businessId_offers_stocks_update`, `v3_businesses_businessId_offers_stocks`, `v3_businesses_businessId_reports_stocks_generate`). Related but not in the task list:
  `v3_businesses_businessId_warehouse_models_status` and the deprecated `v2_campaigns_campaignId_warehouse_status` (`updateWarehouseStatus`, `deprecated: true`, degradation 2027-01-18, shutdown 2027-04-05,
  replacement `updateWarehouseModelStatus`).
- Also compared: the documentation pages (S2) say the same as the description files for every limit, access level and range quoted above.
- **Generation note** for the plan: `api/codegen/yandex.cfg.yaml` lists 14 operation ids today (`getAuthTokenInfo`, `getCampaigns`, `getBusinessOrders`, `getPartnerWarehouses`, `getOfferMappings`, …); none of the stock
  methods is generated yet. A stock plan adds `updateStocksOnPartnerWarehouses`, `getStocksOnPartnerWarehouses` and, for groups and for FBY, `updateStocks`, `getStocks`, `getPagedWarehouses`, `getFulfillmentWarehouses`
  (and `getBusinessSettings` if the plan reads `warehouseModel`) — the configuration file says every listed method is named by a test (`internal/archtest/rules_test.go`).

### 7. Gaps — what the documents do not say and a real cabinet would have to show

1. Whether the number to send must already **exclude the seller's reserved unshipped orders** (S3's table says both «передавайте количество, доступное для продажи» and «вычтите из остатков заказанное»; section 5.1).
2. Whether Market ignores an update whose `updatedAt` is older than the stored one; what Market does with two updates of the same `sku` with the same `updatedAt`.
3. What **0** does to the showcase and how fast; whether Market itself resets a stock that was not updated for N days (not found in any page read).
4. Which of the seven stock types (`AVAILABLE`, `FIT`, `FREEZE`, …) an FBS/Express warehouse's answer holds; whether `AVAILABLE` = `FIT` − `FREEZE`; what a returns warehouse's row looks like (the file says only that it can appear).
5. Whether a cabinet «has groups» is the same fact as `warehouseModel: WAREHOUSE` in the business settings, or as a non-empty `groupInfo` of `getPagedWarehouses`; what `getPartnerWarehouses` answers for a cabinet with groups (docs: «не возвращает группы»); the explanation of groups, anchor `#unified-stocks`, was not found in the help page text read.
6. What Market answers when stock is sent for a SKU that does not exist in the catalog, for an archived SKU, for a warehouse whose model (FBS/DBS/Express) is switched off, or for a shop whose API is disabled (`403 API for campaign … disabled`) — only the v2 update has «Offers not found for SKUs» in the error list; v3 update's behaviour is not listed.
7. Whether a single item error rejects the whole batch (the answer is an empty `200`; no per-item result exists in the schema) — i.e. whether a batch with one bad SKU is applied partly or not at all.
8. Which exact HTTP status and body Market sends for the stock-specific 400s (the list gives the message text only; the body shape is the common `{status:"ERROR", errors:[{code,message}]}` of YMC §4).
9. The real delay from send to showcase (docs: «до нескольких минут» vs «до 15 минут») and whether a read right after a write shows the new value.
10. The access level a stock-writing key needs is `offers-and-cards-management` / `all-methods`; whether the key creation form in the cabinet names that level exactly «Управление товарами и карточками» (it does in S2 `scopes_summary`), and whether a key with only it can list the warehouse ids (it cannot, by the lists; Inference).
11. Whether a cabinet can contain both campaign-level (`CAMPAIGN`) and cabinet-level (`WAREHOUSE`) warehouse models at once, and what the stock write of a shop that shares a warehouse with another shop does for the other shop (S2 says only «Информация для остальных складов в этой группе обновится автоматически»).
12. The real-cabinet limit state: whether the account carries the paid «Медиум» subscription (offer-mappings 600 a minute instead of 100; reports 1 a minute instead of 1 in 2 minutes); the stock methods have the same limit for both tiers.
