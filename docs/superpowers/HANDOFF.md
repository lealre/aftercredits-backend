# Activity feed — handoff

**Branch:** `feat/activity-events` (draft PR, do not merge)
**Last updated:** 2026-08-13
**Plan:** `docs/superpowers/plans/2026-08-10-activity-feed-phase-1.md`
**Spec:** `docs/superpowers/specs/2026-08-10-group-activity-feed-design.md`

Read the spec first for *why*, then the plan for *what*. This file is only
where we stopped and what to do next.

---

## Where this stopped

Two of the plan's seven tasks are committed. **The next job is a fix to Task 3,
not Task 2** — see "Next work" below. Nothing is wired to HTTP yet, so the
feature is invisible at runtime and all of this is safe to iterate on.

| Task | State | Commit |
|---|---|---|
| 1 — migration 005 (`activity_events`, `activity_reads`) | done, reviewed clean | `dd5df5b` |
| 3 — unit of work + store seam | **done, review found a critical hole** | `90e5740` |
| 2 — activity model / queries / store methods | not started | |
| 4 — recorder + flush middleware | not started | |
| 5 — the eleven emit sites | not started | |
| 6 — feed / unread / read endpoints | not started | |
| 7 — frontend bell + polling | not started | |

**Execution order is 1, 3, 2, 4, 5, 6, 7** — not the plan's numeric order.
Task 3 builds the `s.qq(ctx)` / `s.inTx(ctx, …)` seam that Task 2's store code
calls, so it has to land first. The plan text has been corrected to match.

Everything currently passes: `go build ./...`, `go vet ./...`, `gofmt -l .`
clean, and the full `go test -count=1 ./...` green including the
testcontainer-backed `internal/postgres` and `tests/` suites.

---

## Next work: fix Task 3's transaction hole

The Task 3 review found a **critical, latent** defect. The implementer wrote
exactly what the plan specified; the plan was wrong. It is latent only because
nothing wires the middleware yet — Task 4 makes it live, so it must be fixed
first.

### The bug

`qq(ctx)` only **joins** a transaction. It never begins one — `u.Active()`
returns nil until something calls `u.Tx(ctx)`, and only `inTx` does that.

So when a request's **first** write goes through `qq`, that write runs on the
pool and commits immediately, outside the unit of work. A later `inTx` then
begins the request transaction. If the activity-event insert fails, the
middleware rolls back — and the business write is already committed, with no
event recorded.

The result is that atomicity silently depends on the order of statements within
a request. That is exactly the guarantee this whole mechanism exists to provide.

Concrete example once Task 4 lands: `DELETE /groups/{g}/titles/{t}/comments/{id}`
calls `Store.DeleteComment`, which uses `qq` → commits on the pool. The
"comment deleted" event is then recorded through `inTx`, which begins the
transaction. Any later failure rolls that back: the comment is gone, the feed
never mentions it.

### The fix (decided — do both parts)

**Part 1 — convert the writes.** The rule is **reads → `qq`, every write →
`inTx`**, regardless of statement count. Thirteen methods currently use `qq`
for a write and must move to `inTx`:

| File | Methods |
|---|---|
| `internal/postgres/titles.go` | `AddTitle`, `DeleteTitle`, `UpdateTitle` |
| `internal/postgres/users.go` | `AddUser`, `DeleteUserById`, `UpdateUserInfo`, `UpdateUserLastLoginAt`, `UpdateUserGroup`, `RemoveGroupFromUser` |
| `internal/postgres/groups.go` | `UpdateGroupInfo`, `SoftDeleteGroup` |
| `internal/postgres/comments.go` | `DeleteComment` |
| `internal/postgres/ratings.go` | `DeleteRating` |

Verify the list against the code rather than trusting this table — it came from
a review, and methods may have moved.

**Part 2 — handle the aborted-transaction interaction.** Inside an ambient
transaction, a SQL error aborts the *entire* transaction: every subsequent
statement fails with `current transaction is aborted, commands ignored until
end of transaction block`. Standalone mode did not behave that way.

One existing caller depends on the old behaviour:
`internal/services/titles/titles.go:75-83` calls `AddTitle`, and on a
duplicate-key error swallows it and does a `GetTitleById` read-back. Once
`AddTitle` uses `inTx`, that read-back executes inside an aborted transaction
and fails. Today it is accidentally safe *only* because `AddTitle` uses `qq` —
so Part 1 turns this live. Two acceptable fixes:

- wrap the `inTx` body in a savepoint so a failed statement rolls back to the
  savepoint rather than poisoning the whole transaction; or
- change that caller to check existence first instead of writing and swallowing
  the conflict.

Prefer the savepoint if other swallow-and-continue callers turn up; prefer
fixing the caller if this is the only one. Grep for services that ignore a
store error and keep going before choosing.

**Part 3 — widen the guard test.** `TestNoDirectQueryUse` in
`internal/postgres/store_test.go` greps only for `s.q.`. `Store` also holds
`pool *pgxpool.Pool`, so a new method calling `s.pool.Begin(...)` bypasses the
unit of work just as completely while passing the guard. Every pre-refactor
write in this package was written as `s.pool.Begin`, so it is the likelier
regression. Widen the regex to `\bs\.(q|pool)\.[A-Z]`, keeping `store.go`
exempt.

### How to verify the fix

A test that fails before and passes after, not just a green suite:

1. Seed a request context with a unit of work.
2. Call one of the 13 methods (e.g. `DeleteComment`).
3. Roll the unit of work back **without** committing.
4. Assert the row is still there.

Before the fix it is gone (committed on the pool); after, it survives. Mutation
check it: revert the method to `qq`, confirm the test fails, restore.

---

## Deferred findings (from the Task 3 review)

None of these block progress; the final whole-branch review should triage them.

- **`store.go` is allowlisted whole-file** in the guard test, so any future
  method added to that file may use `s.q.` freely. Scoping the exemption to the
  two lines inside `qq`/`inTx` would be tighter.
- **The guard regex depends on the receiver being named `s`.** A method written
  as `func (st *Store) …` using `st.q.` passes. Convention-dependent, not a live
  hole.
- **`Commit`/`Rollback` do not clear `u.tx`** (`internal/uow/uow.go`). After a
  commit, `Active()` still returns the closed transaction, so a later store call
  on the same context gets `pgx: tx is closed` instead of falling back to the
  pool — and the natural middleware shape (`defer u.Rollback(ctx)` plus
  `u.Commit(ctx)`) yields `ErrTxClosed` from the deferred call instead of a
  clean no-op. Setting `u.tx = nil` under the lock makes both idempotent.
- **The shared `pgx.Tx` is not serialized across goroutines.** A pgx connection
  is not goroutine-safe. Not reachable today (the only goroutines are in
  `cmd/routines`, which has no unit of work), but Task 4 should document it.
- **`internal/postgres/testharness_test.go:144`** — the comment above
  `tableNames` still says it lists "every table `001_init.sql` creates", which
  stopped being true when migration 005 added two more.

---

## Things that will bite you

Learned the hard way in this repo; all of them are real, not hypothetical.

- **`truncatePreGroupScoped` (`internal/postgres/testharness_test.go:267`) must
  NOT list the activity tables.** It runs inside `TestMigration003BackfillsGroupId`
  with the schema pinned at goose v2, where those tables do not exist yet. The
  original Task 1 brief got this wrong and the implementer correctly refused it.
  The other three reset sites (`tableNames`, and the two general `TRUNCATE`
  statements) *do* list them.
- **The transaction must begin lazily.** `AddTitleToGroup` calls an external
  metadata provider *before* its group write. An eagerly-begun request
  transaction would hold a pooled Postgres connection open across an internet
  API call — pool exhaustion on the Raspberry Pi this runs on. Do not
  "simplify" `uow.New` into beginning at request start.
- **A `:one` sqlc query that can match multiple rows silently returns the first.**
  This already caused a real bug in this repo. Any query keyed on
  `(user, title)` needs the group too.
- **nil vs empty is an observable API contract** (`docs/CONVENTIONS.md` §5).
  Store read-many returns `[]models.X{}` never nil, *including error paths*.
  Season maps are nil when absent. A decoded Go struct cannot tell `null` from
  `[]`, so tests asserting that distinction must read the raw response body.
- **Do not create a third test file for an existing entity.** `tests/` is
  strictly paired: `X_test.go` (only `func Test…`) and `X_setup_test.go` (only
  helpers). A genuinely new entity gets a new pair — `activity` will — but
  nothing else. Verify with `grep -c '^func Test' tests/*_setup_test.go`, which
  must be 0 for every file.
- **`internal/database` is sqlc-generated.** Never hand-edit it; change
  `sql/queries/*.sql` and run `sqlc generate` from the repo root. Applied
  migrations are never edited either — add a new one.
- **Do not touch the live database.** `docker exec aftercredits-postgres …` is
  the user's real data. The suites spin up their own testcontainers; use a
  throwaway container for manual checks and remove it afterwards.

---

## Working agreements

- **Never commit to `main` and never merge a PR.** Work on the branch; the user
  merges.
- Do not tag or deploy without being asked — versions batch several PRs before
  release (`docs/CHANGELOG.md` is hand-written, newest first, from the
  operator's perspective).
- No personal data in PR descriptions, including row counts from the user's
  database.
- Regression tests must be mutation-checked: revert the fix, confirm the test
  fails, restore. A test that passes either way is worse than none.

---

## Running the SDD loop

The plan is written for `superpowers:subagent-driven-development`. The
controller's scratch (ledger, per-task briefs, review packages) lives in
`.superpowers/sdd/2026-08-10-activity-feed-phase-1/`, which is git-ignored — so
a fresh clone will not have it. That is fine: the ledger's purpose is surviving
compaction within a session, and this document plus `git log` is the durable
record.

If you continue with that skill, it will create its own ledger. Start it at the
Task 3 fix described above, then resume at Task 2.
