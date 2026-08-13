# Group-scoped ratings and comments — design

Status: approved 2026-08-09. Sub-project 1 of 2; sub-project 2 is the group
activity feed, specced separately once this lands.

`docs/` is gitignored apart from `CHANGELOG.md` and `CONVENTIONS.md`, so this
file stays local by design.

## Problem

`ratings` and `comments` carry no `group_id`. Both are unique on
`(user_id, title_id)` globally (`sql/schema/001_init.sql:40`,
`sql/schema/002_comments_user_title_unique.sql:4`), so a rating is one fact per
user per title, shared across every group that user belongs to. Group is only an
authorization gate at write time: `POST /ratings` and `POST /comments` accept
and validate `groupId` (`internal/api/ratings_handlers.go:50`,
`internal/api/comments_handler.go:65`) and then discard it.

Two consequences:

1. **Ratings leak across groups.** The group-titles read path applies no group
   filter and no membership filter at all:

   ```sql
   -- sql/queries/ratings.sql:24
   -- name: GetRatingRowsByTitleIds :many
   SELECT * FROM ratings WHERE title_id = ANY($1::text[]);
   ```

   The result is assigned straight to `GroupRatings`
   (`internal/services/groups/groups.go:278, 287`), so a group's title detail
   ships every rating on that title by every user in the system. Latent today —
   there is one non-deleted group — and live the moment a second exists.
   Comments are filtered (`internal/services/comments/comments.go:24-29`);
   ratings never were.

2. **Ratings cannot be per-group.** There is no way to rate a title differently
   with different groups, and no way to attribute a rating to a group for the
   activity feed.

## Decision

Ratings and comments become **group-scoped facts**. The identity of a rating is
`(user, title, group)`, and `group_id` is `NOT NULL` — always.

Rejected alternatives:

- **Provenance** (`group_id` added, `UNIQUE(user_id, title_id)` kept). One row
  still serves all groups, so `group_id` is first-write-wins and goes stale on
  the first cross-group edit — `internal/services/ratings/ratings.go:143`
  finds the pre-existing row by `(user, title)` and updates it. The column would
  be unsafe to filter on and unreliable to display.
- **Do nothing**, resolving the group at activity-event emit time. Ambiguous by
  construction for a user in two groups sharing a title, and unresolvable for
  `PATCH /ratings/{id}` and `DELETE /ratings/{id}`, which carry no group.

Accepted costs:

- **`Down` is one-way once used.** After any user rates one title in two groups,
  re-adding `UNIQUE(user_id, title_id)` fails — collapsing the rows would have
  to discard a real rating. Verified against a live container. This is correct
  behavior and an accepted, conscious trade.
- **A brief write outage on deploy.** `NOT NULL` lands in one migration, so
  between `database -migrate` committing and the new binary going live, the old
  binary's `InsertRating` / `InsertComment` (explicit column lists without
  `group_id`) return 500. Mitigated by stopping the backend container during the
  migrate step. Must be called out in `docs/CHANGELOG.md`.

Out of scope, explicitly: importing a rating/comment from another group. The
schema does not foreclose it — with `(user, title, group)` uniqueness an import
is an insert with a different `group_id`.

## Migration `sql/schema/003_ratings_comments_group_scoped.sql`

One goose migration, in this order:

1. `ALTER TABLE ratings ADD COLUMN group_id TEXT;` (same for `comments`).
2. Build a temp table of every `(row, group)` pair each row could belong to,
   joining `group_titles` ∩ `group_members` ∩ non-deleted `groups`.
3. **Assert before writing.** A `DO $$` block raises if any row maps to more
   than one candidate group, or to none, listing the offending rows by id, user
   and title. goose wraps the migration in a transaction, so the raise rolls
   back both DDL and DML, records no version, and the deploy fails cleanly and
   re-runnably.
4. Backfill from the temp table.
5. `SET NOT NULL` on both columns.
6. Drop `ratings_user_id_title_id_key` and `comments_user_id_title_id_key`; add
   `UNIQUE (user_id, title_id, group_id)` on both.
7. `REFERENCES groups(id)` with **`ON DELETE CASCADE`**, plus an index on
   `group_id`.

Why the guard when the current data is unambiguous: verified against the live
database, all 178 ratings and 125 comments map to exactly one group, zero
ambiguous, zero orphans. But `LeaveGroup` and `RemoveTitleFromGroup` are live
endpoints that can make a row unattributable between now and deploy. Without
the guard the naive `UPDATE ... FROM` **silently picks an arbitrary group**
(demonstrated), and plain `SET NOT NULL` fails with `contains null values`,
naming no row.

Why CASCADE: `group_members` and `group_titles` already cascade from
`groups(id)` (`sql/schema/001_init.sql:81, 86`), so anything else would make
ratings and comments the odd ones out, and a hard delete would cascade the
members and titles away only to fail on the ratings. The semantics agree: a
rating is a fact about (user, title, group) and carries no meaning once the
group is gone. Groups are only ever soft-deleted today
(`UPDATE groups SET deleted = true`), so this never fires; when a purge of
soft-deleted groups is eventually written, cascading is what it wants, and it
spares that code from hand-ordering the child deletes.

An earlier draft used NO ACTION to make such a purge fail loudly. That was
defensive rather than principled: it does not prevent the deletion, it only
forces the caller to hand-write the child deletes in dependency order, which is
the error-prone sequencing cascades exist to remove. The accepted cost is that
hard-deleting a group is unrecoverable for its ratings and comments.

Migration adds columns, not tables, so `tableNames` and the table-count
assertion in `internal/postgres/testharness_test.go:144` are unaffected, as are
both TRUNCATE lists.

## Backend changes

Standard pipeline, per `docs/CONVENTIONS.md`.

**Storage.** `sql/queries/{ratings,comments}.sql` gain `group_id` in the insert
column lists and group predicates on the reads; `sqlc generate`;
`models.UserRating` and `models.Comment` gain `GroupId string`;
`internal/postgres/mappers.go` maps it; `internal/store/store.go` signatures
take `groupId`; `internal/postgres/{ratings,comments}.go` implement.

**The sharp edge — must be handled deliberately.**
`GetRatingRowByUserTitle` (`sql/queries/ratings.sql:18`) and
`GetUserCommentRowByTitle` (`sql/queries/comments.sql:19`) are sqlc `:one`
queries keyed on `(user, title)`. Under the new uniqueness they match multiple
rows, and `:one` compiles to `QueryRow` — it takes the first and **discards the
rest with no error**. Verified live: two rows returned, one silently chosen.
Both queries gain `AND group_id = $3`. All four call sites already have a
group in scope:

- `internal/services/ratings/ratings.go:143` (TV-series add)
- `internal/services/ratings/ratings.go:242` (movie duplicate check)
- `internal/services/comments/comments.go:110` (TV-series add)
- `internal/services/comments/comments.go:257` (TV-series update)

**Read paths.**

- `GetRatingRowsByTitleIds` gains `AND group_id = $2`, closing the leak.
- `GetCommentsByTitleId` drops its `usersFromGroup []string` parameter for
  `WHERE title_id = $1 AND group_id = $2`. This deletes the `GetGroupById` call
  at `internal/services/comments/comments.go:24`, which today materializes every
  group title and every season row purely to extract `group.Users`.

**API surface.** Nearly unchanged. `POST /ratings` and `POST /comments` already
send and validate `groupId`; they stop discarding it. Its duplicate check
becomes group-scoped, so a user is not blocked from rating a title they already
rated in another group. `PATCH`/`DELETE /ratings/{id}` and
`DELETE /ratings/{id}/seasons/{season}` need no new parameter — they are
id-addressed and the row now carries the group. Comment mutation routes already
carry `groupId` in the path.

Membership authorization stays where it is: `group_id` on the row records where
the fact belongs, and is never by itself proof the viewer may see it. Read paths
keep going through the existing membership guards.

## Frontend changes

`Rating` and `Comment` types gain `groupId`. `POST` calls already send it.

**Required for correctness, not tidiness:** `backendService.ts:381-385` resolves
a rating's id **client-side** by `(userId, titleId)` and then calls
`PATCH /ratings/{id}` with no group context. Once a user holds two ratings for
one title, a client working from an unfiltered list can silently overwrite the
wrong group's rating. Server-side group filtering on the read path closes this,
which is why that filter is load-bearing.

## Testing

Per `docs/CONVENTIONS.md` §8: paired `X_test.go` / `X_setup_test.go`, `t.Run`
subtests each starting with `resetDB(t)`, HTTP and DB assertions through
helpers, descriptive `require` messages.

Seven files change: `tests/{ratings,comments}_test.go`,
`tests/{ratings,comments}_setup_test.go`,
`internal/postgres/{ratings,comments}_test.go`, `tests/goups_test.go`.

Three existing tests assert behavior this change **inverts** and must be
rewritten rather than recompiled:

- `internal/postgres/ratings_test.go:121` `TestStore_AddRating_Duplicate`
- `internal/postgres/comments_test.go:282` `TestAddComment_DuplicateUserTitle`
  — both expect `ErrDuplicatedRecord` on a second `(user, title)` insert, which
  is now legal in a different group.
- `internal/postgres/comments_test.go:207`
  `TestStore_GetCommentsByTitleId_FiltersByUsersFromGroup` — built entirely
  around the member-list filter this change removes.

New coverage required, every one mutation-checked (revert the fix, confirm the
test fails, restore):

1. Same user rates the same title in two groups; both persist independently.
2. Same user comments on the same title in two groups; both persist.
3. **Leak regression:** group A's title detail must not contain group B's
   ratings. This is the test that would have caught the existing bug.
4. Comment read path returns only the requesting group's comments.
5. `PATCH`/`DELETE /ratings/{id}` affects only the targeted group's row.
6. TV-series per-season add/update resolves the correct group's row.
7. Duplicate within the *same* group still returns `ErrDuplicatedRecord`.

## Verification

1. `sqlc generate` produces no diff beyond the intended columns.
2. `go build ./...` and `go vet ./...` clean.
3. Full suite green, including `TestMigrationsUpDown` (up → down → up).
4. Migration applied to a copy of the real local database; row counts unchanged
   and every `group_id` populated.
5. Local stack rebuilt; rating and commenting exercised through the UI.

## Delivery

Two PRs against `main`, neither merged:

- backend: `feat/group-scoped-ratings-comments`
- frontend: matching branch

`docs/CHANGELOG.md` gets bullets under the open, untagged v0.1.2 entry —
including the deploy note about stopping the backend during migration. No tag,
no Docker Hub push, no Pi deploy; v0.1.2 stays open per the batching policy.
