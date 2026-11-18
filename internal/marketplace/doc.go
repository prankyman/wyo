// Package marketplace is the one adapter contract every marketplace implements (rule R6), its registry and the
// connector's own words for accounts and orders. Nothing marketplace-specific lives here: each marketplace is an
// adapter in a folder of its own below this one, and only that folder knows the marketplace.
//
// The contract:
//
//   - Adapter is what every adapter is: its Description (the code, the colour mark, the key form's fields, the parts
//     it supports — what GET /api/v1/marketplaces shows), its Routes (every request it may send, with its limit group,
//     the key category it needs and whether it writes), its limits, how it puts a key into a request and how it reads a
//     refusal's code and text.
//   - Each part of the contract is an interface of its own: KeyChecker (key.check), WarehouseReader (warehouses.read),
//     NewOrderReader (orders.new), StateReader (orders.states), FeedReader (orders.feed), and the writing parts of
//     Plan 4 (writing.go): OrderConfirmer (orders.confirm), LabelFetcher (labels.fetch), CodeReporter (codes.report),
//     VerdictReader (codes.verdict), HandoverGrouper (handover.group), HandoverCloser (handover.close) and
//     HandoverDocumenter (handover.documents). AllCapabilities names the later parts too; Supported says which ones the
//     engine drives today.
//   - An adapter never makes a request itself. It is given a Conn by the synchronisation engine (internal/sync), whose
//     Doer adds the key, keeps the budget, repeats what may be repeated and writes the journal.
//   - A refusal of the marketplace reaches the caller as a *Refusal: a code of the adapter's refusals (with its params
//     and, for a key, the form field it lies on), the account state it leads to and whether it stops the account's
//     jobs. A call that got no readable answer is a *CallError with one of the journal's six outcomes.
//   - contracttest.RunAdapterContract is the one test suite every adapter passes against its fake.
//
// For the accounts service and the HTTP handlers: Registry.All gives the adapters for GET /api/v1/marketplaces;
// sync.Engine.CheckNewKey reads and checks a key before it is saved (its *Refusal's Code and Field go to the form);
// the key text it returns is kept with secretbox.Vault.Put under the account's secret name, and the account row with
// store.Queries.InsertAccount; sync.KeyState gives the state a key's end of life leads to.
package marketplace
