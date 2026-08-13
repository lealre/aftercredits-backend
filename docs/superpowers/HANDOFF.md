# Activity feed phase 2 (real-time) — handoff

**Branch:** `feat/activity-stream`, off `feat/activity-events` (phase 1, draft PR #32, **unmerged**)
**No PR.** Pushed for safekeeping and review only. Phase 1 has to merge first.
**Last updated:** 2026-08-13
**Spec:** `docs/superpowers/specs/2026-08-10-group-activity-feed-design.md` — Phase 2 section
**Plan:** `docs/superpowers/plans/2026-08-13-activity-feed-phase-2.md`

---

## What phase 2 is

Phase 1 shipped a working feed whose badge **polls** every 10s. Phase 2 replaces
the polling with a live server push, so activity appears as it happens.

## The design changed before any of it was built

The spec originally pushed a *signal* — a payload-free `NOTIFY` — and had the
client re-read from its cursor on every event. Revised 2026-08-13 (`553ab45`) to
**push the event data**, after the argument for signalling turned out to be
weaker than stated:

- The claim that a dropped message would permanently lose a feed row was
  **wrong**. `EventSource` reconnects automatically and resends `Last-Event-ID`,
  and the server can replay from it, so completeness was never the difference.
- What genuinely needs a read is the case that is *not* a reconnect: a fresh page
  load or a new tab has no `Last-Event-ID` to resume from.

So the shape is **snapshot, then stream**, in that order:

```
open the stream → on `open`, read the newest page + unread count → merge, dedupe by event id
```

The order is load-bearing. Reading *first* leaves a window where an event lands
after the read but before the stream exists, and is lost with nothing to notice.
Connecting first means every event is either in the snapshot (committed before
its query ran) or on the stream (committed after). They legitimately overlap,
hence the dedupe by `id`.

Running that same sequence on **reconnect** is what replaces `Last-Event-ID`
replay, and it buys three things:

- no replay path in the backend at all — no resume query, no cursor bookkeeping;
- it sidesteps the documented `seq` caveat: `seq` is assigned at INSERT, not
  COMMIT, so a `seq > N` resume permanently skips a late-committing lower-`seq`
  row, while a `seq DESC` page read simply contains it;
- visibility is re-resolved per connection, so leaving a group takes effect with
  no cache invalidation.

Costs, accepted and written down: one fact now has **two serializations** (the
SSE frame and the REST DTO), pinned by a test asserting they are byte-identical;
and an event older than the snapshot page, committed while disconnected, is only
seen by paging back.

---

## Where this stopped

| Task | State | Commit |
|---|---|---|
| 1 — notify on insert + read one event by id | **done**, reviewed clean after one fix round | `77663ad`, `87f1a37` |
| 2 — the fan-out hub (`internal/activity/hub.go`) | not started | |
| 3 — the `LISTEN` loop (`internal/postgres/listen.go`) | not started | |
| 4 — single-use stream tickets | not started | |
| 5 — the two SSE endpoints | not started | |
| 6 — the contract tests (spec tests 10–15) | not started | |
| 7 — frontend `useActivityStream` + the nginx fix | not started | |
| 8 — changelog + final verification | not started | |

Everything currently passes: `go build ./...`, `go vet ./...`, `gofmt -l .`
clean; `internal/postgres` and `tests/` suites green.

### Task 1, in detail

`InsertActivityEvents` now fires `pg_notify('activity_events', <event id>)` for
each event **inside its existing transaction**, and `GetActivityEventById` reads
one event (joined to `groups` for the name) for the listener to fan out.

Two things the review verified rather than assumed:

- The notify is bound to the transaction's `q`, not the pool's — so a
  rolled-back insert notifies nobody. There is now a test for that half too: a
  batch whose second event carries a bogus `group_id` fails the FK *after* the
  first event has inserted and notified, and the test asserts no notification
  arrives and the table is empty. The first event genuinely does notify before
  the failure, so the negative assertion means something.
- The payload is the **id**, not the row. `pg_notify` caps payloads at 8000
  bytes; the listener reads the row once and fans the DTO to its own clients.

Fixed during review: a duplicate row→model mapper had been added on the reasoning
that sharing one would need an interface or reflection. It doesn't — the two
generated row types are field-for-field identical, so
`activityEventRowToModel(database.GetActivityFeedRowsRow(row))` converts
directly. `mappers.go` now has zero net diff. That mattered beyond tidiness: two
mappers for one row shape drift by discipline alone, and a feed/stream mismatch
is exactly what Task 6's contract test exists to catch.

---

## Next: Task 2, the hub

Full steps in the plan. The constraints that carry the design:

- **`Publish` must never block.** It runs on the single `LISTEN` goroutine, so
  one slow client would stall delivery for everyone. Buffered channel per
  subscriber (16), and a send to a full channel **drops** rather than waits — a
  dropped frame is repaired by the client's next snapshot; a stalled listener is
  not repaired by anything.
- **The hub applies the same visibility rules as the feed**: the event's group is
  in the subscriber's groups, and `actorId != userId`. If this diverges from the
  SQL predicate, the stream shows things the feed does not.
- Membership is captured **at subscribe time**; joining a group mid-stream takes
  effect on the next connect. Documented, not solved — the alternative turns
  every push into a query.
- `Unsubscribe` closes the channel exactly once and is safe to call twice.

Then Task 3 (the listener) is the piece with the two easy-to-get-wrong details:
it must hold a **dedicated** connection (a pooled one returned between the
`LISTEN` and the wait loses the subscription silently) and it must **reconnect
with backoff** (the Pi's Postgres restarts; a dead listener means pushes stop
until the process does).

---

## Traps worth knowing before touching Tasks 5 and 7

- **nginx will break the stream silently.** `nginx.conf` (frontend repo) needs
  `proxy_buffering off` and `proxy_read_timeout` well beyond the 60s default on
  the stream route. Without them the stream *connects* and then delivers
  nothing — everything looks healthy. The handler also sends
  `X-Accel-Buffering: no` as a second line of defence, and a `:ping` comment
  every ~25s so an idle stream is not cut.
- **`GET /activity/stream` must go in `api.PublicPaths`** — it authenticates by
  ticket, not Bearer token, so `AuthMiddleware` would reject it.
  `POST /activity/stream-ticket` must **not** be public.
- **`EventSource` cannot send an `Authorization` header**, and the JWT must not
  go in a query string (nginx access logs, browser history) — hence the
  single-use, 60s ticket.
- **The stream must not go through `authFetch`** on the frontend: its 401 handler
  navigates away, which is the same trap phase 1's `activityFetch` exists to
  avoid.
- **Everything stays behind `ACTIVITY_FEED_ENABLED`.** Flag off must mean no
  notify, no listener, no stream routes — phase 1's plug-out property has to
  survive phase 2.

---

## Related state outside this branch

- **Phase 1 backend** — `feat/activity-events`, draft PR #32, complete and
  reviewed (tasks 1–6 of that plan). Its PR body needs replacing; the text is at
  `.superpowers/sdd/2026-08-10-activity-feed-phase-1/pr-body.md`.
- **Phase 1 frontend** — branch `feat/activity-feed-bell` in
  `/home/lealre/personal/aftercredits`, **uncommitted by instruction**. Bell,
  badge, panel, per-row read state, toast, 10s poll. Had **no independent
  review** (three subagent dispatches died to API 529s, so it was written
  inline). PR body ready at
  `.superpowers/sdd/2026-08-10-activity-feed-phase-1/frontend-pr-body.md`.
- **Local test stack**: container `aftercredits-postgres` on port **5433**, real
  migrated data at migration 005; backend on `:8080` with the flag on; frontend
  dev server on `:8081`. Test accounts `afmaria` / `afjoao` (password
  `pw-<username>-123`) share "Feed Test Group".
- **Open product question** carried from phase 1: for a season-scoped rating
  update, `previousNote` is the rating's overall pre-update note, so an event can
  pair a per-season new value with an overall previous one. Faithful to the plan;
  worth deciding before the feed UI leans on it.

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
