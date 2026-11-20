# Smoke test of Plan 8 — the Яндекс Маркет exchange through the fulfilment centre

Plan: `.agents/plans/wyo-yandex-market-fbs-and-express-exchange-with-a-fulfilment-centre-codes-labels-daily-shipment-courier-code.md`.
Run on 2026-11-20 on the development machine `server` (4 cores, 15 GB; 10 GB available), by a Node script with Playwright (the
`smoke-testing` skill's `node <script>` form; no MCP browser was available in the session), against the test build of the wyo server
(`bin/wyo-test`, built by the browser tests' setup after the last change of Go code) and the fakes (`bin/wyofakes`), in a data folder of
the run's own, with an owner made for the run. No key, password or cookie was printed or written down.

```
Smoke Test PASSED

Servers: started and stopped (the wyo server on 127.0.0.1:8790, the fake marketplaces on :8791, the fake fulfilment centre on :8792)
- Signing in through the form: passed
- R3, «Внешний API» empty: three accounts (Яндекс Маркет, Ozon, Wildberries) and a Яндекс Маркет order read; the fake fulfilment
  centre received no call: passed
- «Внешний API» filled, the warehouses and marketplaces matched, the fulfilment centre naming 58 × 40 for Яндекс Маркет and 40 × 30
  for Wildberries: the page «Внешний API» at 1280 and 390: passed
- An FBS order of two parcels with a marking code on each shirt, an Express order and a test order walked to «готов к отгрузке»;
  a Wildberries order's sticker asked for at 40 × 30 (the fulfilment centre's size) and attached; an Ozon posting sent and reserved
  beside them: «Заказы» and the FBS order's details at 1280 and 390: passed
- Express: the courier's first code refused, the second accepted, both answers at the fulfilment centre; the code in no answer of
  the wyo server; the order's details at 1280 and 390: passed
- The FBS hand-over: Яндекс Маркет's day's shipment holding another order is the problem «Состав отгрузки маркетплейса не совпал с
  передачей…» naming it, never confirmed; once the shipment holds only the hand-over's order, «Повторить» confirms it and sends the
  act: «Передачи» in both states, at 1280 and 390: passed
- «Журнал запросов», «Обзор» and the bell at 1280 and 390: passed
- After every screen: no console error, no raw text key, no sideways scroll at 390, no request to an address but the wyo server's
  own: passed
- R4: no key's beginning in any page's source: passed
- «Внешний API» emptied: a new order read, every job run; the fake fulfilment centre received no further call: passed
Waiting for the maintainer: the checks on a real Яндекс Маркет cabinet and a real fulfilment centre (register item 42)
Cleanup: completed — both servers stopped, the ports free, the data folder and the password file deleted, no automation browser left
```

What was looked at beyond the script's checks, in the screenshots: the Express order's details show «Как заказ уедет — курьер, который
назовёт код; wyo проверит его у маркетплейса», «готов к отгрузке» and when Маркет was told; the problem of the shipment names
`1000000001` and the kind «Маркет забирает со склада» beside the batch's number; the phone layout of «Передачи» holds the card without
sideways scroll. The screenshots were kept in the session's scratch folder only.

## Found while implementing

Nothing that is wrong. One wording to note: when the shipment holds an extra order and lacks none, the
problem's second sentence reads «…не вошли в отгрузку: —.», the same «—» the earlier batch problem `batch_differs` uses for an empty
list.
