# Activity feed — handoff

**Branch:** `feat/activity-events` (draft PR, do not merge)
**Last updated:** 2026-08-13
**Plan:** `docs/superpowers/plans/2026-08-10-activity-feed-phase-1.md`
**Spec:** `docs/superpowers/specs/2026-08-10-group-activity-feed-design.md`

Read the spec first for *why*, then the plan for *what*. This file is only
where we stopped and what to do next.

---

## Where this stopped

**The design changed on 2026-08-13.** The activity feed no longer shares a
transaction with the business write. Events are buffered per request and handed
to a pluggable `activity.Sink` **after** the write has committed, and the whole
feature sits behind `ACTIVITY_FEED_ENABLED` (default off). The reasoning is in
the spec under "Capture — recorder, sink, flush-after-commit" and "Why not the
atomic design"; the trade accepted is best-effort delivery — an event can be lost
if the process dies between the commit and the flush.

Two consequences for whoever picks this up:

- **Task 3's committed work is being removed, not fixed.** The critical hole the
  review found (thirteen writes whose atomicity depended on statement order) and
  the aborted-transaction interaction in `internal/services/titles` both
  disappear with the mechanism. Nothing needs a savepoint.
- **The next job is the unwind**, then Task 2. Nothing is wired to HTTP yet, so
  the feature is invisible at runtime and all of this is safe to iterate on.

| Task | State | Commit |
|---|---|---|
| 1 — migration 005 (`activity_events`, `activity_reads`) | done, reviewed clean | `dd5df5b` |
| 3 — ~~unit of work + store seam~~ → **unwind it** | built, then superseded by the redesign | `90e5740` (to be reverted) |
| 2 — activity model / queries / store methods | not started | |
| 4 — recorder + sink + flag-gated flush | not started, rescoped | |
| 5 — the eleven emit sites | not started, unchanged | |
| 6 — feed / unread / read endpoints | not started, routes now flag-gated | |
| 7 — frontend bell + polling | not started, unchanged | |

**Execution order is 1, 3, 2, 4, 5, 6, 7** — not the plan's numeric order. Task 3
now runs first among the remainder so that Task 2's store code is written against
the final shape (plain `s.q` for reads, `s.inTx` for its one write) instead of
against a seam that is about to be deleted.

Everything currently passes: `go build ./...`, `go vet ./...`, `gofmt -l .`
clean, and the full `go test -count=1 ./...` green including the
testcontainer-backed `internal/postgres` and `tests/` suites.

---

## Next work: Task 3 — unwind the unit of work

Full steps are in the plan (`### Task 3`). In short:

1. `rm -r internal/uow` and `rm internal/postgres/store_test.go` (the latter holds
   only `TestNoDirectQueryUse`, which guards a rule that no longer exists — with
   no ambient transaction, a direct `s.q.Foo(…)` write is correct because a single
   statement commits on its own).
2. In `internal/postgres/store.go`, delete `qq` and reduce `inTx` to its
   standalone branch (begin / defer rollback / fn / commit), dropping the `uow`
   import. `inTx` is worth keeping: it removes the `pool.Begin` boilerplate the
   twelve genuinely multi-statement writes used to repeat by hand.
3. `sed -i 's/s\.qq(ctx)\./s.q./g'` across
   `internal/postgres/{groups,ratings,comments,titles,users}.go` (50 sites), then
   `gofmt -w internal/postgres`.
4. Sweep: `grep -rn "qq(ctx)" internal/` and
   `grep -rn "internal/uow\|uow\." internal/ --include='*.go'` must both be
   silent; `grep -c 'inTx(ctx' internal/postgres/*.go` must still total twelve.
5. `go build ./... && go vet ./... && gofmt -l .`, then the
   `internal/postgres` and `tests/` suites.

Then continue with Task 2 as written.

### What to watch for in Task 4 (rescoped)

The parts that carry the design's weight now, all specified in the plan:

- The flush runs **after** the response, so the request context may already be
  cancelled — it needs `context.WithoutCancel` plus its own timeout, or events
  vanish whenever a client disconnects.
- A sink error is **logged and swallowed**. Propagating it would fail a request
  whose write already committed, which is precisely the failure mode the redesign
  exists to remove. There is a test for it; keep it.
- Reuse `responseRecorder` from `internal/server/middleware.go` for the status
  gate rather than adding a second wrapper.
- The middleware sits **inside** `AuthMiddleware` so the actor is in context and
  no emit site has to pass it.
- The flag must be read **once** and used for both the middleware and (Task 6)
  the routes. Half-on — events recorded with no endpoint to read them, or
  endpoints with nothing feeding them — is the one state to make unreachable.

---

## Deferred findings (from the Task 3 review)

Four of the five are **moot** once Task 3 is unwound, and are recorded here only
so the final review does not re-derive them:

- ~~`store.go` allowlisted whole-file in the guard test~~ — the guard test is
  deleted.
- ~~The guard regex depends on the receiver being named `s`~~ — same.
- ~~`Commit`/`Rollback` do not clear `u.tx`~~ — the package is deleted.
- ~~The shared `pgx.Tx` is not serialized across goroutines~~ — no shared
  transaction exists any more. (Worth remembering if anyone ever revisits
  atomicity: a pgx connection is not goroutine-safe.)

Still live:

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
