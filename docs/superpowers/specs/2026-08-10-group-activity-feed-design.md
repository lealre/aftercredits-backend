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
`seq BIGSERIAL NOT NULL UNIQUE` is the ordering and cursor key. One column
solves three problems: a total order for pagination (CONVENTIONS §6), keyset
pagination that does not shift as new events arrive at the head, and the SSE
`Last-Event-ID` resume token in phase 2.

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

## Capture — unit of work + recorder

Two pieces, both generic:

**`internal/activity` — the recorder.** `activity.Record(ctx, event)` appends to
a buffer held in the request context. One line per emit site: no store handle, no
error path, no knowledge of transactions. Constructors per kind
(`activity.RatingAdded(groupId, title, note)`) keep call sites readable and make
a malformed event a compile error.

**`internal/uow` — the unit of work.** Middleware seeds a unit of work into the
request context for mutating methods. It does **not** begin a transaction
eagerly: the transaction starts lazily on the first store write. After the
handler returns, on a 2xx the buffered events are inserted into that same
transaction and it commits; on anything else it rolls back, so the business
write and its events vanish together.

The store gains one helper — `s.qq(ctx)` returns `s.q.WithTx(tx)` when a unit of
work has an active transaction and `s.q` otherwise. Handlers and services are
otherwise untouched.

**Why lazy.** `AddTitleToGroup` calls the external metadata provider
(`internal/api/groups_handler.go`, `titles.AddNewTitle(api.Db, api.Provider, …)`)
*before* its group write. Beginning the transaction at request start would hold a
Postgres connection open across that network call — a pool-exhaustion risk on the
Pi. Lazy begin means the provider call completes before the transaction exists.

**The footgun, stated plainly.** The transaction spans everything the handler
does after its first write. Today no handler writes, calls out, then writes
again. A future one that did would hold a transaction across the network. This
needs a comment at the seam and a test asserting the ordering, not just
intentions.

**The second cost.** Every `s.q.Foo(…)` becomes `s.qq(ctx).Foo(…)` — roughly 60
mechanical edits, where a missed one silently writes outside the transaction:
exactly the bug the mechanism exists to prevent. Mitigated by a test that fails
on any direct `s.q.` use outside the helper.

Rejected alternatives: a pure middleware wrapper cannot see group ids that arrive
in a request body (`POST /ratings`, `POST /comments`, `POST /groups/titles`) nor
the title's name, so it yields access-log entries rather than feed lines; a
direct write per handler multiplies eleven error paths that each have to be
remembered.

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

### Transport: SSE

Chosen over WebSocket and polling because the traffic is strictly server → client
and because `EventSource` gives reconnect *and* gap-free resume for free: the
server tags each message with `id: <seq>`, and the browser automatically returns
it as `Last-Event-ID` after a drop. WebSocket would require hand-rolling both and
adds a dependency for a bidirectional channel that is never used in reverse.

### The push is a signal, never the source of truth

The event row and a `pg_notify` commit together; each backend process `LISTEN`s
on a dedicated connection and fans out to its own connected clients. The message
carries no payload — the client re-reads from its cursor. Consequently:

- a server restart, a closed laptop, or a dropped connection lose nothing, since
  the client resumes from `seq`;
- a `NOTIFY` delivered to nobody is harmless, because the row is committed;
- a second backend instance works with no shared broker — which is the seam that
  makes this scale without Redis or NATS.

`pg_notify` is transactional: fired inside the transaction, it is delivered only
if the commit succeeds, so no one is ever notified about a rating that rolled
back.

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

Both are currently wrong in `nginx.conf` (frontend repo) and would fail silently:

- `proxy_buffering off` on the stream route — the default buffers the response,
  so nothing reaches the client until the buffer fills.
- `proxy_read_timeout` raised beyond the 60s default, and a `:ping` comment every
  ~25s regardless, so an idle stream is not cut.

### Phase 2 frontend

A `useActivityStream` hook replaces the polling interval, bumping the badge and
prepending rows. Polling remains as the reconnect fallback. The stream must not
go through `authFetch`, whose 401 handler navigates away.

## Testing

Per CONVENTIONS §8: paired `X_test.go` / `X_setup_test.go`, `t.Run` subtests each
starting with `resetDB(t)`, HTTP through helpers, no new file under `tests/`.

Phase 1:

1. Each of the eleven kinds is emitted exactly once by its endpoint, with the
   right actor, group, title name and payload.
2. **A failed request emits nothing** — force a service error after a partial
   write and assert both the business row and the event are absent.
   Mutation-checked; this is the core claim of the unit of work.
3. Own actions are excluded from the actor's feed and badge, and visible to
   other members.
4. A non-member sees nothing from that group; a departed member stops seeing it.
5. Delete events carry the title name and previous value captured pre-delete.
6. Keyset pagination returns every event exactly once with no duplicates or gaps.
7. Watermark: unread count falls to zero after `POST /activity/read`, and new
   events raise it again.
8. A store write that bypasses `s.qq(ctx)` fails a guard test.

Phase 2:

9. `pg_notify` fires on commit and does **not** fire on rollback.
10. A client resuming with `Last-Event-ID` receives exactly the events it missed.
11. Ticket auth: valid once, rejected on reuse and after expiry.

## Delivery

Two PRs against `main`, neither merged without review. Phase 1 is independently
shippable and useful. Changelog entries under whichever version is open at the
time; migration 005 is additive DDL, so unlike 003/004 it needs no
stop-the-backend deploy note.

Prerequisites, both to be done before implementation starts:

- v0.1.2 tagged and deployed (it is merged but deliberately untagged).
- Frontend PR #10 merged — the feed's deep link needs its reactive group id.
