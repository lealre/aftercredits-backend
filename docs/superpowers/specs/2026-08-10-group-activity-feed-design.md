# Group activity feed — design

Status: approved 2026-08-10. Sub-project 2 of 2; sub-project 1 (group-scoped
ratings and comments) shipped in v0.1.2.

Built in two shippable phases. Phase 1 is complete and useful on its own with
client polling; phase 2 replaces polling with a live push. Each gets its own
plan, PR and review.

`docs/` is gitignored apart from `CHANGELOG.md` and `CONVENTIONS.md`, so this
file stays local by design.

## Goal

A logged-in user sees what everyone else in their groups has been doing —
"maria added *Dune: Part Two*", "joão rated *Oppenheimer* 9" — as a feed with an
unread badge. One global feed in the navbar merging every group the user belongs
to, each row labelled with its group and clickable through to that title.

The stated priority is the capture mechanism: it must be reusable and generic
across many endpoints, not bespoke per handler.

## Decisions

### An append-only event log, not derived state

`ratings` and `comments` already carry `user_id`, `group_id`, `created_at` and
`updated_at`, so a `UNION ALL` could synthesise part of a feed with no new write
path. Rejected because it cannot express:

- **who added a title** — `group_titles` has no actor column at all, and that is
  the first event the user asked for;
- **deletions** — the row is gone, nothing remains to derive from;
- **history** — only latest state survives, so 9 → 7 → 8 collapses into a single
  "updated" row and each rating can ever yield at most two feed entries.

State tables answer "what is true"; a feed asks "what happened". Deriving one
from the other thins out precisely as a group gets more active.

### Record once, fan out on read

One row per action, owned by the actor. Recipients are computed at read time:

```sql
WHERE e.group_id IN (my groups)
  AND e.actor_id <> me
  AND e.seq > my_watermark
```

The same predicate serves the feed, the unread badge and the push filter, so
visibility rules exist in exactly one place. Fan-out on write (a row per
event × recipient) is the alternative and is only worth it at celebrity scale;
here it would multiply storage by the member count and buy nothing.

The user's own actions **are** recorded — they are what notifies everyone else —
and are filtered out of that user's own feed and badge.

### Ordering and cursors: a `seq BIGSERIAL`

`id TEXT` (uuid, matching the repo's convention) is identity; a separate
`seq BIGSERIAL NOT NULL UNIQUE` is the ordering and cursor key. It solves two
problems: a total order for pagination (CONVENTIONS §6), and keyset pagination
that does not shift as new events arrive at the head. Phase 2 also tags each SSE
frame with it, but deliberately does **not** resume from it — see "Snapshot, then
stream", where the caveat below is the reason.

**Known caveat:** sequence values are assigned at INSERT time, not commit time,
so a transaction that starts earlier but commits later can carry a lower `seq`
than one already delivered. A client resuming from `seq > N` could therefore
miss it. At this scale (single writer in practice, a handful of users)
concurrent conflicting writes are vanishingly rare. The mitigation, if it ever
matters, is to resume from a small overlap window and dedupe client-side by
`id` — the client already has ids. Documented rather than pre-solved.

### Unread as a watermark

A single watermark per user in its own small table, rather than a row per
(event, user). The badge only ever needs a count and "mark all read", so
per-event rows would grow without bound to power something that never uses the
granularity. An absent row means "never read", so everything is unread.

The watermark is a **`seq`**, not a timestamp — the same key the feed orders and
paginates by, so "unread" and "after this page" cannot disagree. `read_at` is
kept alongside it purely as human-readable "when you last looked"; nothing reads
it in a predicate.

Kept in its own table rather than a column on `users`, so `models.User` — which
is loaded on every authenticated request — is untouched.

### Retention: keep everything

No pruning. Revisit if the table ever becomes a problem; `cmd/routines` is where
a prune job would live.

### No collapsing

A series marked season by season sends one `PATCH` per season
(`UpdateGroupTitleWatchedRequest.Season` is a single `*int`), so eight seasons
produce eight events. Explicitly chosen over windowed collapsing.

## Data model — migration 005

```sql
CREATE TABLE activity_events (
    id         TEXT PRIMARY KEY,
    seq        BIGSERIAL NOT NULL UNIQUE,
    group_id   TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    actor_id   TEXT NOT NULL,
    actor_name TEXT NOT NULL,
    kind       TEXT NOT NULL,
    title_id   TEXT,
    title_name TEXT,
    payload    JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX activity_events_group_seq_idx ON activity_events (group_id, seq DESC);

CREATE TABLE activity_reads (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    read_at TIMESTAMPTZ NOT NULL,
    read_seq BIGINT NOT NULL DEFAULT 0
);
```

`actor_name` and `title_name` are denormalized deliberately: the auth middleware
already holds the full `models.User` so the actor's name is free, and the title
is in scope at nearly every emit site. It also keeps a row readable after the
member leaves the group or the title is deleted from the catalogue. Renames do
not propagate — the accepted trade, and how activity feeds are normally built.

`title_id` carries **no** foreign key, for the same reason: an admin deleting a
title must not delete history. `group_id` **does** cascade, consistent with
`group_members`, `group_titles`, `ratings` and `comments`.

Migration 005 is pure DDL on new tables — no backfill, and no bearing on the
still-open question of whether data repair belongs inside migrations.

## Event catalogue

Eleven kinds across three entities and three verbs.

| kind | emitted from | payload |
|---|---|---|
| `title_added` | `POST /groups/titles` | — |
| `title_removed` | `DELETE /groups/{groupId}/titles/{titleId}` | — |
| `title_watched_changed` | `PATCH /groups/{id}/titles` | `watched`, `season?` |
| `rating_added` | `POST /ratings` | `note`, `season?` |
| `rating_updated` | `PATCH /ratings/{id}` | `note`, `previousNote`, `season?` |
| `rating_deleted` | `DELETE /ratings/{id}` | `previousNote` |
| `rating_season_deleted` | `DELETE /ratings/{id}/seasons/{season}` | `season`, `previousNote` |
| `comment_added` | `POST /comments` | `season?` |
| `comment_updated` | `PATCH …/comments/{commentId}` | `season?` |
| `comment_deleted` | `DELETE …/comments/{commentId}` | — |
| `comment_season_deleted` | `DELETE …/comments/{commentId}/seasons/{season}` | `season` |

Comment bodies are **not** copied into the payload: they can be long, they can be
edited, and the feed links to the title where the current text lives.

Delete events must capture `title_name` and the previous value **before** the row
is removed. Every delete path already loads the entity for its ownership guard,
so the data is in hand at the right moment.

`title_id` is null only for group-level events, of which phase 1 has none; the
column is nullable so member/group events can be added later without a
migration.

## Capture — recorder, sink, flush-after-commit

**Revised 2026-08-13.** The original design shared one transaction between the
business write and its events, so the two committed or vanished together. That
is dropped. The reasons are in "Delivery guarantee" below; the short version is
that atomicity welded the feature to Postgres and to every write path in the
store, and the feature is meant to be pluggable and switchable instead.

Three pieces:

**`internal/activity` — the recorder.** `activity.Record(ctx, event)` appends to
a buffer held in the request context. One line per emit site: no store handle, no
error path, no knowledge of transactions or transports. Constructors per kind
(`activity.RatingAdded(groupId, title, note)`) keep call sites readable and make
a malformed event a compile error. `Record` on a context with no recorder is a
no-op, which is what makes the feature switchable without touching emit sites.

**`activity.Sink` — the pluggable destination.** One method:

```go
type Sink interface { Append(ctx context.Context, events []Event) error }
```

Phase 1 ships one implementation, backed by `store.Store.InsertActivityEvents`.
Redis, RabbitMQ, a log-only sink for debugging, or a fan-out to several are
implementations of the same interface, and none of them touch the recorder, the
eleven emit sites, or the read model. The `Event` type is plain data (`map[string]any`
payload) precisely so it can be serialized to any of them.

**The flush — after commit, never inside it.** Middleware seeds a recorder into
the request context for mutating methods. After the handler returns, on a 2xx,
the buffered events go to the sink in one call. A sink failure is **logged and
swallowed**: the user's rating already committed, and the feed missing a line is
not a reason to fail their request. This inverts the original design's failure
mode, where a broken event insert would have rolled back a legitimate action.

Two details the implementation must get right: the flush runs after the response
is written, so the request context may already be cancelled — it needs
`context.WithoutCancel` plus its own timeout; and it is inline (not a goroutine),
which keeps event order within a request and avoids leaking work past the
process's lifetime at zero benefit for this traffic.

**The feature flag.** `ACTIVITY_FEED_ENABLED`, default **off**. When off, no
recorder is seeded (so `Record` no-ops), no sink is constructed, and the
`/activity*` routes are not registered. Merging the feature therefore changes
nothing in production until it is switched on per environment. The eleven emit
sites stay compiled in either way — they are one inert line each.

### Delivery guarantee

**Best-effort (at-most-once).** An event is lost if the process dies between the
business commit and the flush, or if the sink is down. Accepted for a
family-scale movie tracker: a missing "maria rated *Dune* 9" in the feed is a
cosmetic gap, not a data-integrity incident, and nothing downstream reconciles
against the log.

If at-least-once is ever wanted, the upgrade path does **not** require
revisiting this design: `activity_events` is already shaped like a transactional
outbox. Write the row in the business transaction, and have a relay
(`cmd/routines` is where it would live) publish committed rows to the broker and
track its position. That is strictly additive — the recorder, the emit sites and
the API are unchanged, and phase 2's `pg_notify` is a miniature of the same idea.

### Why not the atomic design

Rejected after building it (commit `90e5740`, reverted):

- It welded the feature to Postgres. A shared transaction is unavailable across
  Postgres and a broker, so "same convention, different provider" was impossible
  by construction rather than by omission.
- It entangled every write in the store. Fifty read sites and twelve write sites
  routed through an ambient-transaction seam, and the review found that a request
  whose first write joined no transaction committed outside the unit of work —
  making atomicity depend on statement order. The mechanism that existed to
  guarantee atomicity had a hole in exactly that guarantee.
- Its lazy-begin escape (needed because `AddTitleToGroup` calls the metadata
  provider before writing) left a documented footgun: any future handler that
  wrote, called out, then wrote again would hold a transaction across a network
  call.
- It made a failed event insert reject the user's action.

Dropping it also removes two problems outright: the thirteen-method ordering bug
above, and the aborted-transaction interaction where `internal/services/titles`
swallows a duplicate-key error and reads back (a savepoint would have been
needed only because of the ambient transaction).

Store writes keep the atomicity they had before this work: single statements
auto-commit, and the twelve genuinely multi-statement writes keep their own
explicit transaction via the store's `inTx` helper.

Also rejected: a pure middleware wrapper cannot see group ids that arrive in a
request body (`POST /ratings`, `POST /comments`, `POST /groups/titles`) nor the
title's name, so it yields access-log entries rather than feed lines; a direct
sink write per handler multiplies eleven error paths that each have to be
remembered, and puts transport knowledge in the handlers.

## Read model and API

```
GET  /activity?limit=&before=<seq>     paged feed, keyset, newest first
GET  /activity/unread-count            badge
POST /activity/read                    watermark := latest seq
```

`GET /activity` returns the user's groups' events excluding their own, newest
first, with each row carrying its group's name for the label and `titleId` for
the deep link. Membership is resolved per request — a user who leaves a group
stops seeing its history immediately, because visibility is computed at read
time rather than frozen into the row.

The feed does **not** reuse `generics.Page`. That envelope is
`Page`/`Size`/`TotalPages`/`TotalResults`, all of which are offset concepts that
mean nothing for a keyset cursor, and computing `TotalResults` would cost a
second `count(*)` over the whole log on every poll. The feed returns its own
shape instead:

```json
{ "events": [ … ], "nextBefore": 1234, "hasMore": true }
```

`nextBefore` is null when the page reaches the end. `events` follows
CONVENTIONS §5 — non-nil, `[]` when empty, asserted on the raw body.

## Phase 1 frontend

Bell in `src/components/Header.tsx`, the only chrome on every authenticated
page. TanStack Query drives both the badge and the feed, with a polling interval
— the codebase's first poller, so it must respect two existing traps: `authFetch`
hard-redirects to `/login` on any 401 (a background poll on an expired token
would eject the user mid-session), and queries must be gated on `enabled: !!token`.

Each row deep-links to its title inside its group, which requires the reactive
active-group id from frontend PR #10.

## Phase 2 — real-time delivery

**Revised 2026-08-13.** The original design pushed a *signal* and had the client
re-read from its cursor. That is dropped: the stream now carries the **event
data**, and the client reads exactly once per connection — a snapshot — rather
than once per event. Rationale and the trade in "Snapshot, then stream" below.

### Transport: SSE

Chosen over WebSocket and polling because the traffic is strictly server →
client, and `EventSource` gives automatic reconnect for free. WebSocket would
need that hand-rolled and adds a bidirectional channel that is never used in
reverse.

Note what is deliberately **not** relied on: `Last-Event-ID`. The browser sends
it on automatic reconnect, and the server could replay `WHERE seq > <id>` — but
this design re-reads a snapshot on every (re)connect instead, which subsumes
replay. See below.

### Snapshot, then stream

Two mechanisms, in a fixed order:

1. **Open the stream.**
2. **On `open`, fetch the snapshot** — the newest page of `GET /activity` plus
   `GET /activity/unread-count`. This is the only read per connection.
3. **Merge**, deduplicating by event `id`: the snapshot is the base, and any
   pushed events that arrived since `open` are folded in.

The order is the point. Reading *before* connecting leaves a window where an
event lands after the read but before the stream exists, and is lost with nothing
to notice it. Connecting first means every event is either already in the
snapshot (committed before its query ran) or arrives on the stream (committed
after) — nothing can fall between them. Events can appear in both, which is why
the merge dedupes by `id`.

The same sequence runs on **reconnect**, which is what replaces `Last-Event-ID`
replay. That buys three things:

- **No replay path in the backend.** No resume query, no `Last-Event-ID`
  handling, no server-side cursor bookkeeping.
- **It sidesteps the `seq` caveat** documented under "Ordering and cursors".
  `seq` is assigned at INSERT, not COMMIT, so a transaction that starts earlier
  but commits later can carry a *lower* seq than one already delivered. A strict
  `seq > N` resume skips it permanently; a snapshot read ordered `seq DESC`
  includes it, because it is simply inside the newest page.
- **Visibility is re-resolved on every connect.** Membership is a read-time
  predicate, so a user who left a group stops seeing its history from the next
  connection, without any cache invalidation logic.

The cost, stated plainly: an event older than the snapshot page that was
committed while the client was disconnected is only visible once the user pages
back through the feed. With a page of 15 and this app's traffic that is a corner
of a corner, and the feed remains complete on the server either way — the log is
still the source of truth, the stream is just the fast path to it.

### What the stream carries

The same DTO the REST feed returns, one event per message:

```
id: <seq>
event: activity
data: {"id":"…","seq":42,"groupId":"…","groupName":"…","actorId":"…",
       "actorName":"…","kind":"rating_added","titleId":"tt…",
       "titleName":"…","payload":{…},"createdAt":"…"}
```

`id: <seq>` is kept even though replay is not implemented — it costs nothing and
leaves the door open.

**One serialization, not two.** The frame's `data` is the *same* JSON shape as
`GET /activity`'s `events[]` elements, produced by the same mapper. This is the
main thing pushing data costs you, so it is pinned by a test asserting the two
are byte-identical for one event: two representations of one fact that drift is
how a client ends up handling both.

### Fan-out: `pg_notify`

The event row and a `pg_notify` commit together; each backend process `LISTEN`s
on a dedicated connection and fans out to its own connected clients.

`pg_notify`'s payload cap is 8000 bytes. An event's JSON is far below that
(comment bodies are deliberately not copied into payloads), but the cap is a hard
edge, so the notify carries the event **id** and the listening process reads that
row once and fans the full DTO out to its clients. One read per event per
process, not per client.

Consequences of the row being committed before anyone is notified:

- a `NOTIFY` delivered to nobody is harmless — the row is committed, and the next
  connect's snapshot picks it up;
- a second backend instance needs no shared broker, which is the seam that lets
  this scale without Redis or NATS;
- a server restart or a closed laptop loses nothing, because reconnect takes a
  fresh snapshot.

`pg_notify` is transactional: fired inside a transaction, it is delivered only if
that commit succeeds, so nobody is notified about a row that never landed. The
transaction meant here is the **sink's own** insert — phase 1 flushes after the
business commit, not inside it — so a rating that rolled back produces no event
to notify about, because the flush is status-gated and never runs. The notify
therefore belongs in the Postgres sink implementation alongside the insert; a
non-Postgres sink would carry its own push mechanism instead.

### Endpoints

```
POST /activity/stream-ticket           short-lived, single-use
GET  /activity/stream?ticket=…         text/event-stream
```

`EventSource` cannot send an `Authorization` header, and this app authenticates
with a Bearer JWT from localStorage. Putting that JWT in the query string would
leak it into nginx access logs and browser history, so an authenticated request
exchanges it for a single-use ticket (~60s TTL, in-memory) which the stream
consumes once.

### Deployment requirements

Both are currently wrong in `nginx.conf` (frontend repo) and would fail
**silently** — the stream connects and then simply never delivers, which is the
worst failure shape:

- `proxy_buffering off` on the stream route — the default buffers the response,
  so nothing reaches the client until the buffer fills.
- `proxy_read_timeout` raised beyond the 60s default, plus a `:ping` comment
  every ~25s regardless, so an idle stream is not cut.

### Phase 2 frontend

`useActivityStream` replaces the polling interval: open `EventSource`, take the
snapshot on `open`, then prepend pushed rows and bump the badge as messages
arrive.

- The stream must not go through `authFetch`, whose 401 handler navigates away —
  the same reason phase 1 has `activityFetch`.
- **The polling interval is removed, not kept as a fallback.** Reconnect is
  `EventSource`'s job and each reconnect takes a fresh snapshot, so a timer would
  duplicate that. The one case it covered — a browser or proxy where streaming
  never works at all — is instead handled by falling back to the phase 1 poll
  only after the stream has failed to establish repeatedly, so a broken
  environment degrades to phase 1 behaviour rather than to nothing.
- Pushed rows are merged by `id`, never appended blindly: the snapshot and the
  stream legitimately overlap.


## Testing

Per CONVENTIONS §8: paired `X_test.go` / `X_setup_test.go`, `t.Run` subtests each
starting with `resetDB(t)`, HTTP through helpers, no new file under `tests/`.

Phase 1:

1. Each of the eleven kinds is emitted exactly once by its endpoint, with the
   right actor, group, title name and payload.
2. **A non-2xx response emits nothing** — the flush is status-gated, so a
   rejected or failed request leaves no event behind. Mutation-checked.
   Note what is deliberately *not* claimed: a handler that commits its write and
   then fails leaves the row with no event. That is the accepted consequence of
   best-effort delivery, not a defect.
3. **A sink failure does not fail the request** — inject a sink that always
   errors and assert the business write still returns its normal 2xx and its row
   is present. This is the inverted failure mode and the reason the atomic
   design was dropped; it needs a test, not a comment.
4. Own actions are excluded from the actor's feed and badge, and visible to
   other members.
5. A non-member sees nothing from that group; a departed member stops seeing it.
6. Delete events carry the title name and previous value captured pre-delete.
7. Keyset pagination returns every event exactly once with no duplicates or gaps.
8. Watermark: unread count falls to zero after `POST /activity/read`, and new
   events raise it again.
9. **With the feature off**, a mutating request writes no event and the
   `/activity*` routes are absent — the plug-out path is asserted, not assumed.

Phase 2:

10. `pg_notify` fires on commit and does **not** fire on rollback.
11. **A pushed frame's `data` is byte-identical to the same event in
    `GET /activity`** — the one real cost of pushing data rather than signalling
    is two representations of one fact, so they are pinned rather than trusted.
12. **The snapshot and the stream overlap without duplicating.** An event
    committed between the stream opening and the snapshot query appears in both
    and must render once, deduplicated by `id`.
13. **A reconnect takes a fresh snapshot** and the client ends up with the same
    set as a client that never dropped — including an event whose `seq` is lower
    than one already delivered (the INSERT-vs-COMMIT ordering caveat), which is
    precisely what a `seq > Last-Event-ID` replay would have missed.
14. A stream only delivers events from the reader's own groups, excluding their
    own actions — the same predicate the feed uses, asserted on the stream too.
15. Ticket auth: valid once, rejected on reuse and after expiry.

## Delivery

Two PRs against `main`, neither merged without review. Phase 1 is independently
shippable and useful. Changelog entries under whichever version is open at the
time; migration 005 is additive DDL, so unlike 003/004 it needs no
stop-the-backend deploy note.

Prerequisites, both to be done before implementation starts:

- v0.1.2 tagged and deployed (it is merged but deliberately untagged).
- Frontend PR #10 merged — the feed's deep link needs its reactive group id.
