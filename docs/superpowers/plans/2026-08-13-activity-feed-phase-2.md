# Activity Feed — Phase 2 (real-time delivery) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the badge's polling interval with a live server push, so activity appears as it happens.

**Architecture:** The event row's insert fires `pg_notify` carrying the event **id**. Each backend process `LISTEN`s on one dedicated connection, reads the notified row once, and fans the full DTO out to its own connected SSE clients. A client opens the stream first, then takes a **snapshot** (newest page + unread count) on `open`, and merges pushed frames by `id`. Reconnect repeats that sequence, which is why there is no `Last-Event-ID` replay path.

**Tech Stack:** Go 1.24, pgx/v5 (`pgxpool` + a dedicated `pgx.Conn` for `LISTEN`), sqlc, `net/http` SSE with `http.Flusher`, testcontainers-go (postgres:16), testify. Frontend: React 18 + TypeScript, TanStack Query v5, native `EventSource`.

**Spec:** `docs/superpowers/specs/2026-08-10-group-activity-feed-design.md` — the Phase 2 section, revised 2026-08-13 to push data rather than signal.

**Branch:** `feat/activity-stream`, off `feat/activity-events` (phase 1, unmerged draft PR #32). Phase 1 must merge before this does.

## Global Constraints

- Everything here is **behind the same `ACTIVITY_FEED_ENABLED` flag** as phase 1. Off means no notify, no listener, no stream routes — the plug-out property must survive phase 2.
- **No schema migration.** `pg_notify` needs none, and the ticket store is in-memory. If a task thinks it needs a table, stop and re-read the spec.
- `internal/database` is sqlc-generated; change `sql/queries/*.sql` and run `sqlc generate`. Never hand-edit.
- Nothing outside `internal/postgres` names a concrete database (CONVENTIONS §2). The listener lives in `internal/postgres`; the hub it feeds must not.
- Services declare sentinels + `ErrorMap` in `utils.go`; handlers translate through it (CONVENTIONS §3). `err` before `!ok`.
- Read-many returns `[]models.X{}`, never nil, including error paths (CONVENTIONS §5).
- `go` directive stays `1.24.0`. **New dependencies: none expected** — pgx already supports `LISTEN`/`WaitForNotification`, and SSE is `net/http`. If a task believes it needs one, stop and report.
- Tests: `internal/postgres/*_test.go` for store-level, `internal/activity` and `internal/server` for unit, `tests/` for HTTP. `tests/` stays strictly paired (`X_test.go` only `func Test…`).
- The stream is a long-lived request. Every test that opens one **must** close it and use a bounded timeout; a leaked stream hangs the suite rather than failing it.
- Do not commit to `main`; do not merge. Push the branch only.

## File Structure

**Created:**

| Path | Responsibility |
|---|---|
| `internal/activity/hub.go` | in-process fan-out: subscribe/unsubscribe, publish to matching subscribers |
| `internal/activity/hub_test.go` | hub unit tests (no database) |
| `internal/activity/ticket.go` | in-memory single-use stream tickets with a TTL |
| `internal/activity/ticket_test.go` | ticket unit tests |
| `internal/postgres/listen.go` | the `LISTEN` loop: notified id → row read → hand to the hub |
| `internal/postgres/listen_test.go` | end-to-end notify→listener test against a container |
| `internal/api/activity_stream_handler.go` | `POST /activity/stream-ticket`, `GET /activity/stream` |
| `internal/services/activity/stream.go` | ticket issue/redeem + subscription assembly for the handler |

**Modified:**

| Path | Change |
|---|---|
| `sql/queries/activity.sql` | `+ GetActivityEventById`; notify inside `InsertActivityEventRow`'s transaction |
| `internal/postgres/activity.go` | fire `pg_notify` per inserted event; `GetActivityEventById` |
| `internal/store/store.go` | `+ GetActivityEventById`; the listener is NOT on this interface (see Task 2) |
| `internal/server/server.go` | start the listener, register the two routes — same flag local |
| `internal/services/activity/utils.go` | ticket sentinels + `ErrorMap` entries |
| `docs/CHANGELOG.md` | operator-facing entry, incl. the nginx requirement |

**Frontend (separate repo, `/home/lealre/personal/aftercredits`, branch off `feat/activity-feed-bell`):**

| Path | Change |
|---|---|
| `src/hooks/useActivityStream.ts` (new) | `EventSource`, snapshot-on-open, merge by id |
| `src/hooks/useActivityFeed.ts` | interval removed; degraded-mode poll only when the stream never establishes |
| `src/services/backendService.ts` | `+ fetchActivityStreamTicket` |
| `nginx.conf` | `proxy_buffering off` + raised `proxy_read_timeout` on the stream route |

---

### Task 1: Notify on insert

The push begins where the row lands. `pg_notify` fires **inside** the sink's existing transaction, so it is delivered only if that commit succeeds — nobody is ever told about a row that rolled back.

**Files:**
- Modify: `sql/queries/activity.sql`, `internal/postgres/activity.go`, `internal/postgres/activity_test.go`

**Interfaces:**
- Produces: every `InsertActivityEvents` call emits one `NOTIFY activity_events, '<event id>'` per event, committed with the rows.
- The payload is the **id only**. `pg_notify`'s payload caps at 8000 bytes; an id is 36. The listener reads the row.

- [ ] **Step 1: Write the failing test**

Append to `internal/postgres/activity_test.go`. It listens on its own connection, inserts, and asserts the notification arrives with the right payload — the honest test for "did the notify fire", rather than trusting the SQL:

```go
func TestStore_InsertActivityEvents_Notifies(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	// A dedicated connection: LISTEN is per-connection, and a pooled one may
	// hand the LISTEN and the later wait to different sessions.
	conn, err := newTestPool(t).Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "LISTEN activity_events")
	require.NoError(t, err)

	actor, group := seedActorAndGroup(t, s) // existing helper shape; adapt to the file
	require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{{
		GroupId: group, ActorId: actor, ActorName: "Maria", Kind: "rating_added",
	}}))

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	n, err := conn.Conn().WaitForNotification(waitCtx)
	require.NoError(t, err, "the insert must notify; a timeout here means it did not")
	require.Equal(t, "activity_events", n.Channel)
	require.NotEmpty(t, n.Payload, "the payload carries the event id")

	// The payload must name a row that exists — the listener will read it.
	got, err := s.GetActivityEventById(ctx, n.Payload)
	require.NoError(t, err)
	require.Equal(t, "rating_added", got.Kind)
}
```

Adapt the seeding to whatever helpers `activity_test.go` already has; do not add a second way to seed.

- [ ] **Step 2: Run it — expect a failure at the notification wait**

`go test ./internal/postgres/ -run TestStore_InsertActivityEvents_Notifies -count=1 -v`
Expected: FAIL, timing out in `WaitForNotification` (nothing notifies yet), and a compile error on `GetActivityEventById`.

- [ ] **Step 3: Add the queries**

Append to `sql/queries/activity.sql`:

```sql
-- name: NotifyActivityEvent :exec
-- Fired inside the insert's transaction, so it is delivered only if that commit
-- succeeds. The payload is the event id, not the row: pg_notify caps payloads at
-- 8000 bytes, and the listener reads the row once and fans it out.
SELECT pg_notify('activity_events', sqlc.arg('event_id')::text);

-- name: GetActivityEventById :one
-- Used by the LISTEN loop to turn a notified id into the row it pushes. No
-- visibility predicate here: the loop has no reader, and the hub filters per
-- subscriber.
SELECT e.*, g.name AS group_name
FROM activity_events e
JOIN groups g ON g.id = e.group_id
WHERE e.id = $1;
```

`sqlc generate`, then confirm the generated `GetActivityEventByIdRow` carries `GroupName` (the DTO needs it, exactly as `GetActivityFeedRows` does).

- [ ] **Step 4: Implement**

In `InsertActivityEvents`, inside the existing `inTx` closure, after each successful `InsertActivityEventRow`:

```go
			// Same transaction as the insert: a rolled-back event notifies nobody.
			if err := q.NotifyActivityEvent(ctx, row.ID); err != nil {
				return err
			}
```

(Capture the returned row — the query is `:one` and already returns it.) Add `GetActivityEventById` beside the other reads, mapping through the same row→model helper the feed uses, returning `store.ErrRecordNotFound` via `notFound(err)`.

- [ ] **Step 5: Run to verify it passes**

`go test ./internal/postgres/ -count=1` — the new test and the whole package green.

- [ ] **Step 6: Commit**

```bash
git add sql/queries/activity.sql internal/database/ internal/postgres/ internal/store/store.go
git commit -m "feat: notify on activity insert, and read one event by id"
```

---

### Task 2: The hub — in-process fan-out

**Files:**
- Create: `internal/activity/hub.go`, `internal/activity/hub_test.go`

**Interfaces:**
- Produces:
  - `type Subscriber struct { UserId string; GroupIds []string; Events chan models.ActivityEvent }`
  - `func NewHub() *Hub`
  - `(*Hub).Subscribe(userId string, groupIds []string) *Subscriber`
  - `(*Hub).Unsubscribe(s *Subscriber)`
  - `(*Hub).Publish(event models.ActivityEvent)` — non-blocking

**Design constraints, each with a reason:**

- **`Publish` must never block.** It runs on the single `LISTEN` goroutine; one slow client would stall delivery for everyone. Each subscriber's channel is buffered (16), and a send to a full channel is **dropped**, not waited on. A dropped frame is survivable — the client's next snapshot repairs it — whereas a stalled listener is not.
- **The hub applies the visibility rules**, using the same predicate as the feed: deliver only if the event's `groupId` is in the subscriber's groups **and** `actorId != userId`. If this diverges from the SQL predicate, the stream shows things the feed does not.
- Group membership is captured **at subscribe time**. A user who joins a group mid-stream sees it on their next connect. Documented, not solved: the alternative is re-resolving membership per event, which turns every push into a query.
- `Unsubscribe` closes `Events` exactly once and is safe to call twice (the handler will `defer` it and may also react to a closed stream).

- [ ] **Step 1: Write the failing tests** (`hub_test.go`) — no database needed. Cover: a subscriber in the group receives; a subscriber in a different group does not; the actor does not receive their own event; a full channel drops rather than blocking (fill the buffer, then `Publish` with a timeout guard asserting it returns promptly); `Unsubscribe` closes the channel and a second call does not panic; concurrent `Publish`/`Subscribe` under `-race`.

- [ ] **Step 2:** Run — expect compile failure (`undefined: NewHub`).

- [ ] **Step 3: Implement** with a `sync.RWMutex` guarding a `map[*Subscriber]struct{}`. `Publish` takes the read lock and does non-blocking sends (`select { case s.Events <- e: default: }`).

- [ ] **Step 4:** `go test ./internal/activity/ -count=1 -race` — green, no races.

- [ ] **Step 5: Commit** — `feat: in-process fan-out hub for activity subscribers`

---

### Task 3: The listener

**Files:**
- Create: `internal/postgres/listen.go`, `internal/postgres/listen_test.go`

**Interfaces:**
- Produces: `func (s *Store) ListenActivity(ctx context.Context, publish func(models.ActivityEvent)) error` — blocks until ctx is done; acquires its **own** `pgx.Conn` (not a pooled one), `LISTEN activity_events`, then loops on `WaitForNotification`, reads each notified id, and calls `publish`.

**Why `publish func(...)` rather than the hub type:** keeps `internal/postgres` from importing `internal/activity`, so the store stays a database package (CONVENTIONS §2) and the hub stays testable without a database. `server.go` wires the two.

**Constraints:**

- **A dedicated connection**, via `pool.Acquire` held for the lifetime, or `pgx.ConnectConfig`. A pooled connection returned between the `LISTEN` and the wait loses the subscription silently.
- **Reconnect with backoff.** The Pi's Postgres restarts; a dead listener means the feature silently stops pushing until the process restarts. On any connection error: log, back off (capped, e.g. 1s→30s), re-establish, re-`LISTEN`. Note in the comment that events during the gap are not replayed — clients repair on their next snapshot.
- A notified id whose row is missing (deleted between notify and read) is logged and skipped, never fatal.
- Returns nil on `ctx.Done()`, so a shutdown is not an error.

- [ ] **Step 1: Write the failing test** — start `ListenActivity` in a goroutine with a captured `publish`, insert an event through the store, and assert the published event arrives within a timeout with the right kind and group name. Use `t.Cleanup` to cancel. Then a second case: an id notified for a row that does not exist is skipped without killing the loop (notify a bogus id directly with `SELECT pg_notify(...)`, then insert a real one and assert the real one still arrives).
- [ ] **Step 2:** Run — expect compile failure.
- [ ] **Step 3: Implement.**
- [ ] **Step 4:** `go test ./internal/postgres/ -count=1 -race`.
- [ ] **Step 5: Commit** — `feat: LISTEN loop turning notified ids into published events`

---

### Task 4: Stream tickets

`EventSource` cannot send an `Authorization` header, and the JWT must not go in a query string (nginx access logs, browser history).

**Files:**
- Create: `internal/activity/ticket.go`, `internal/activity/ticket_test.go`
- Modify: `internal/services/activity/utils.go` (sentinels + `ErrorMap`)

**Interfaces:**
- `func NewTicketStore() *TicketStore`
- `(*TicketStore).Issue(userId string) string` — 32 bytes from `crypto/rand`, hex
- `(*TicketStore).Redeem(ticket string) (userId string, ok bool)` — **single use**; a redeemed or expired ticket is gone
- TTL 60s, swept lazily on `Redeem` plus on `Issue` (no goroutine to leak)

- [ ] **Step 1: Failing tests** — issue then redeem returns the user; a second redeem fails; an expired ticket fails (inject a clock or construct with a past issue time — do not `time.Sleep(60s)`); an unknown ticket fails; tickets are unguessable (two issues differ; length is 64 hex chars); concurrent issue/redeem under `-race`.
- [ ] **Step 2:** Run — compile failure.
- [ ] **Step 3: Implement** — mutex-guarded `map[string]entry{userId, expiresAt}`.
- [ ] **Step 4:** `go test ./internal/activity/ -count=1 -race`.
- [ ] **Step 5: Commit** — `feat: single-use, short-lived stream tickets`

---

### Task 5: The SSE endpoints

**Files:**
- Create: `internal/api/activity_stream_handler.go`, `internal/services/activity/stream.go`
- Modify: `internal/server/server.go`, `internal/services/activity/utils.go`

**Endpoints:**

```
POST /activity/stream-ticket    → { "ticket": "…", "expiresIn": 60 }   (authenticated)
GET  /activity/stream?ticket=…  → text/event-stream                    (ticket-authenticated)
```

**The stream handler, in order:**

1. Redeem the ticket → 401 on failure (do **not** leak whether it was unknown vs expired).
2. Resolve the user's groups (the same read the feed uses) and `hub.Subscribe`.
3. Headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, `X-Accel-Buffering: no` — the last one makes nginx stop buffering even if the config is wrong, which is the failure the spec warns is silent.
4. `http.Flusher` — if the ResponseWriter is not one, 500 immediately rather than serving a stream nobody receives.
5. Loop on: an event → write `id:`/`event: activity`/`data: <DTO json>` then flush; a ~25s ticker → write a `:ping` comment then flush; `r.Context().Done()` → `defer hub.Unsubscribe` and return.
6. The `data` payload is produced by **the same mapper** as `GET /activity`'s elements — not a second serializer. This is the cost the spec accepted for pushing data, and the test in Task 6 pins it.

`GET /activity/stream` must be added to `api.PublicPaths`: it authenticates by ticket, not by Bearer token, so `AuthMiddleware` would reject it. `POST /activity/stream-ticket` must **not** be public.

Both routes are registered under the existing `activityFeedEnabled` local — the same read as phase 1's middleware and routes.

- [ ] **Step 1** Ticket handler + service, with tests (issue requires auth; response shape).
- [ ] **Step 2** Stream handler. Test with `httptest.NewServer` and a real HTTP client with a bounded deadline: connect, publish through the hub, assert the frame arrives and parses; assert a `:ping` arrives if the ticker is shortened for the test (make the interval a package var so a test can lower it); assert an invalid ticket gets 401; assert closing the client's context unsubscribes (the hub's subscriber count drops).
- [ ] **Step 3** Wire in `server.go`: build the hub and ticket store, start `ListenActivity` in a goroutine bound to a context cancelled on shutdown, register both routes — all inside the flag branch.
- [ ] **Step 4** `go build ./... && go vet ./... && gofmt -l .`; `go test ./internal/... -count=1 -race`; `go test ./tests/ -count=1`.
- [ ] **Step 5: Commit** — `feat: SSE stream and ticket endpoints for live activity`

---

### Task 6: The integration tests the spec asks for

**Files:** `tests/activity_test.go`, `tests/activity_setup_test.go`

Spec tests 10–15. The two that matter most:

- **Frame/DTO identity (test 11):** capture one pushed frame's `data` and the same event from `GET /activity`, and assert the JSON is **byte-identical**. This is the whole defence against the two representations drifting.
- **Snapshot/stream overlap renders once (test 12):** open the stream, publish an event, then take the snapshot, and assert the event appears exactly once when merged by `id`.

Plus: notify fires on commit and not on rollback (10); a reconnect ends with the same set as a client that never dropped, including a lower-`seq` late commit (13); the stream honours the same visibility predicate as the feed (14); ticket single-use and expiry over HTTP (15).

Every test that opens a stream: bounded timeout, explicit close, and no leaked goroutines.

- [ ] Write, run, commit — `test: pin the stream's contract against the feed's`

---

### Task 7: Frontend + nginx

**Files (frontend repo, branch off `feat/activity-feed-bell`):** `src/hooks/useActivityStream.ts` (new), `src/hooks/useActivityFeed.ts`, `src/services/backendService.ts`, `nginx.conf`.

- `useActivityStream`: fetch a ticket, open `EventSource`, and **on `open`** take the snapshot (newest page + unread count). Merge pushed frames by `id`; bump the badge for events not already counted.
- Remove `UNREAD_POLL_INTERVAL_MS` as the normal path. Keep a slow poll as a **degraded mode**, enabled only after the stream has failed to establish repeatedly, so a proxy that breaks streaming falls back to phase 1 behaviour rather than to nothing.
- `EventSource` must not go through `authFetch` (its 401 handler navigates away) — same reason `activityFetch` exists.
- `nginx.conf`: on the stream route, `proxy_buffering off;` and `proxy_read_timeout` well beyond 60s. Both are currently wrong and fail **silently** — the stream connects and never delivers.

- [ ] Implement; `npm run build`; `npm run lint` (report before/after); manual two-account check that a rating appears without a reload.

---

### Task 8: Docs and final verification

- [ ] `docs/CHANGELOG.md`: activity now arrives live; **the nginx change is an operator action** — without it the stream connects and silently delivers nothing.
- [ ] Full sweep: `sqlc generate` idempotent; `go build/vet/gofmt`; `go test ./internal/... ./tests/ -count=1 -race`; flag-off still means no notify, no listener, no routes.
- [ ] Whole-branch review, then push. **No PR** until phase 1 (#32) merges.
