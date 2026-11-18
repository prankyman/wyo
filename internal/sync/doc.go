// Package sync is the synchronisation engine: the one caller every request to a marketplace goes through (the key taken
// from the secret store only for the moment of the call, the program named, the budget kept, the deadline, the repeats,
// one journal row per request), the jobs that read each account on the scheduler of internal/schedule, the marks of the
// feeds and the intake of what an adapter read into the orders.
//
// What the accounts service and the HTTP handlers use:
//
//   - Caller.Conn(Target) gives an adapter its marketplace.Conn for one account and one job.
//   - Engine.CheckNewKey reads and checks a key before it is saved (marketplace.KeyChecker through the caller);
//     Engine.CheckAccount checks a saved account's key again and reads its warehouses.
//   - Engine.Jobs are the scheduled jobs; Engine.Refresh is «Обновить» (at most once in RefreshEvery).
//   - KeyState is the state a key's end of life gives an account.
//
// The package is named after its folder; inside it the standard library's sync is imported as stdsync.
package sync
