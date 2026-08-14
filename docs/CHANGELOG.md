<a name="unreleased"></a>
## Unreleased

The group activity feed — a log of what everyone in your groups has been doing,
plus an unread badge. **Off by default**, so deploying this changes nothing until
you switch it on. The frontend for it is not here yet, so with the flag on the
API answers but nothing in the app shows it.

* **New feature: the group activity feed.** Every group-scoped write — adding or
  removing a title, marking one watched, rating, commenting, and the deletions of
  each — records an event naming who did it, in which group, and to which title.
  Three new authenticated endpoints serve it: `GET /activity` (newest first,
  cursor-paginated), `GET /activity/unread-count` (the badge), and
  `POST /activity/read` (mark everything read)
* You see other members' actions, never your own, and only for groups you are
  currently in — leaving a group hides its history from you immediately
* **Enable it with `ACTIVITY_FEED_ENABLED=true`.** Unset or false means no events
  are recorded at all and the three routes are absent (404), so the feature can be
  switched off again without a rollback
* **Requires migration 005**, which adds two tables (`activity_events`,
  `activity_reads`). Unlike 003 and 004 it is additive DDL on new tables only, so
  it needs no stop-the-backend step — `database -migrate` can run with the app up
* Event delivery is deliberately best-effort: events are written after the change
  they describe has already been saved, so a crash at the wrong moment can lose a
  feed line, and a failure to record one can never fail the action itself. Your
  ratings and comments are never at risk from the feed
* A member's name and the title's name are stored on each event as they were at
  the time, so a feed line stays readable after someone leaves the group or a
  title is removed from the catalogue. Later renames do not change old lines
* Nothing is pruned; the log keeps everything
* **Activity now arrives live.** Two new authenticated endpoints —
  `POST /activity/stream-ticket` (mints a single-use, 60-second ticket) and
  `GET /activity/stream?ticket=…` (a server-sent-events stream) — push each
  event to a group's other members as it happens, replacing the unread
  badge's 10-second poll. Both live behind the same `ACTIVITY_FEED_ENABLED`
  flag as the rest of the feed: off means no `pg_notify`, no listener process,
  and the two routes are absent (404), exactly like the three phase 1 routes
* No new migration. This adds no schema change at all
* **Delivery stays best-effort, deliberately.** A missed or dropped live
  update is repaired by the snapshot the client takes on every connect and
  reconnect, never by retrying the write. This is the same guarantee phase 1
  already gave the feed itself — your ratings and comments are never put at
  risk by any of this
* Each backend process holds one additional, dedicated database connection
  for `LISTEN`, on top of its normal pool, for as long as the feature is
  enabled
* **Operator action if you run your own reverse proxy in front of the API:**
  a proxy that buffers responses will let the stream connect and then
  deliver nothing — a failure that looks perfectly healthy from the outside.
  The backend already sends `X-Accel-Buffering: no` on the stream response
  and pings the connection roughly every 25 seconds, and the bundled
  frontend's `nginx.conf` now has a dedicated location for the stream route
  with buffering off and a raised read timeout. If your proxy isn't that
  nginx config, give `/activity/stream` the equivalent: buffering disabled
  and a read timeout longer than 25 seconds
* **Watched activity now says what actually changed.** Marking a title watched
  and merely correcting the date it was watched used to produce the identical
  feed line. Each of those events now carries the watched date and the state
  before the change, so the feed can tell marking watched from marking not
  watched, adding a date from moving one, and a single season's change from the
  whole series'. No migration: this is extra detail inside the event's existing
  JSON payload
* **The unread badge is now per event, not a "newer than" watermark.** Clicking
  one line in the feed marks exactly that line read and drops the badge by one;
  older and newer lines keep whatever state they had. Before, read state could
  only ever be a contiguous newest-N, so reading one line silently read every
  older one and reading the newest cleared the whole badge
* Each event in `GET /activity` — and in every frame the live stream pushes —
  now carries `"read"`, the *asking* reader's own state, so the app can render
  one row read while its neighbours stay unread. It is always present; a pushed
  event is always `false`, because it was only just recorded
* **`POST /activity/read` is replaced by two routes:**
  `POST /activity/events/{id}/read` (mark that one event read; `204`, and `404`
  for an event you cannot see) and `POST /activity/read-all` (clear the badge,
  no body). Both are idempotent — sending either twice is a success that
  changes nothing the second time. The old route took a `seq` in the body,
  which no longer means anything, so it is gone rather than quietly
  reinterpreted; nothing shipped enabled against it
* **Requires migration 007**, which adds `activity_event_reads` (one row per
  user per event read), drops `activity_reads` (the watermark table), and adds
  the `activity_visible_events` view the feed, the badge and both mark-read
  routes now share so they cannot disagree about what you can see. It is
  additive plus one drop of a table only the feed used, so like 005 it needs no
  stop-the-backend step — but run it **before** starting the new image, since
  the old binary's mark-read query needs the dropped table. Any existing read
  state is discarded, which costs nothing: the feature has never run enabled
* The read table grows with users × events read, and clearing the badge writes
  a row per visible event rather than moving one integer. That is the accepted
  price of per-row read state; nothing is pruned here either
* **One of a group's titles can now be fetched on its own:**
  `GET /groups/{groupId}/titles/{titleId}` returns exactly what
  `GET /groups/{groupId}/titles` returns for that title in its list — the same
  object, ratings scoped to that group included — so the app can open a title's
  detail straight from a feed line instead of hunting for it across pages of the
  list. An unknown group, a group you are not in, and a title that group does
  not hold are all `404`, the same as the other routes under that path. No
  migration, and it is **not** behind `ACTIVITY_FEED_ENABLED`: it is an ordinary
  group-titles read that the feed happens to use

<a name="v0.1.2"></a>
## [v0.1.2](https://github.com/lealre/aftercredits-backend/compare/v0.1.1...v0.1.2) (2026-08-08)

Ratings and comments become group-scoped — which needs a schema migration and a
short stop of the backend during the deploy, see below. Plus a handful of bug
fixes — including titles going missing from a paged list, and spurious logouts
during a database blip — fewer database round-trips on
`GET /groups/{id}/titles`, and the query-parameter tests that endpoint never
had.

* **Ratings and comments are now per-group facts.** A rating or comment belongs to one group, keyed on (user, title, group), so the same title can be rated and commented differently in each group you share it with. Previously one rating was shared across every group the author belonged to
* Fix a group's title detail showing other groups' ratings. The group-titles read applied no group filter at all, so every rating on a title by any user in the system was served to every group holding it. Comments were already filtered; ratings never were
* Rating and comment responses now carry `groupId`
* Rating or commenting on a title you already rated or commented in another group is no longer rejected as a duplicate; a second one in the *same* group still conflicts
* The group-scoped comment routes now bind the comment id to the group in the URL: `PATCH`/`DELETE /groups/{groupId}/titles/{titleId}/comments/{commentId}` returns `404` when the id names a comment in a different group, instead of editing or deleting it
* **Requires migration 003.** It backfills `group_id` on every existing rating and comment from the one live group that holds the title and has the author as a member, and aborts cleanly — changing nothing, recording no version, safe to re-run — if any row maps to no group or to more than one. If it aborts, the message names the offending rows with their user and title
* **Deploy action: stop the backend container before running `database -migrate`, and start the new image after.** The migration makes `group_id` `NOT NULL` in the same step that adds it, so the previous binary's rating and comment inserts fail for as long as it stays up against the new schema
* Note that migration 003's `Down` is one-way once used: rolling back re-adds the global (user, title) uniqueness, which fails as soon as anyone holds two ratings or comments for one title in different groups
* Fix `orderBy=watched` returning a different order on every request. The id list is built by ranging a map, and nothing re-sorted it for that key, so the sort order — and therefore which titles landed on which page — was effectively random. It now sorts unwatched first (matching `ORDER BY watched ASC`) with a title-id tie-break, giving a total order
* Fix a database error being reported as `404 Not Found`. Six handlers checked `!ok` before `err`, so any store failure surfaced as "not found" instead of a 500
* Fix titles being duplicated or skipped when paging a sorted list. Any sort whose column repeats a value — the same rating, the same year, the same type — left those rows in no defined order, so a title could come back on two pages while another was never returned at all. Walking a 122-title group by rating gave 119 rows and 118 distinct titles. Every sort now ends in a title-id tie-break, so paging returns each title exactly once. Titles that tie are now ordered by id among themselves, which can change which of them lands on which page compared with before — that ordering was previously arbitrary and could differ between two identical requests, so there was no stable "before" to preserve. Where titles with no `updatedAt` appear is unchanged
* Fix a database blip logging everyone out. Any error other than "user not found" while loading the caller left the auth middleware looking at an empty user, which it reported as `401 Invalid or inactive user` — and never logged. A transient store failure is now a logged `500`; a genuinely unknown or deactivated user still gets the same `401` as before
* Guard `GET /groups/{id}/titles` and `GET /groups/{id}/users` with a single `EXISTS` query instead of loading the whole group and discarding it — the group, its members, every group title and every season row were being materialized twice per request
* Fetch ratings for the titles on the requested page rather than for every title in the group
* **`GET /groups/{id}/titles` now reads through a single SQL query.** It used to load the whole group — every member, every title and every season row — then filter, sort and paginate in memory before re-fetching the page. Filtering, sorting, paging and the result count now happen in one round trip in the database. Responses are unchanged, field for field, including the ordering of every sort key and how an empty page is represented
* Sorting a group's titles by `watched`, `watchedAt` or `addedAt` is now paginated by the database like every other sort key, and carries the same title-id tie-break, so those three can no longer repeat or skip a title across pages either
* Remove `EnsureGroupExists`, now that nothing calls it
* Drop `group_titles.title_type`. It was written exactly once — hard-coded to `movie` for every title, series included — never refreshed, and never read by anything; a title's type has only one source, the titles catalogue. Removed by migration 004, which is covered by the same deploy rule as 003 (stop the backend before `database -migrate`)
* Add integration coverage for the group-titles query parameters: pagination, page-past-the-last totals, `Content` null-vs-`[]`, `watched` and `titleType` filters, and ordering by `watched`, `watchedAt` and `addedAt`
* Move `CHANGELOG.md` and `CONVENTIONS.md` under `docs/`
* Remove the `git-chglog` configuration: the changelog is maintained by hand. Regenerating it overwrote the written entries, listed merge commits as changelog lines, and dropped any release that was never tagged
* Merge the two `v0.0.13` changelog entries, which the version's two PRs had each appended separately
* Document the conventions that were only implicit: the `store.Store` boundary, the nil-vs-empty response contract, checking `err` before `!ok` on `(bool, error)` guards, total ordering for paginated sorts, migrations as a deploy step, and the `X_test.go`/`X_setup_test.go` split

<a name="v0.1.1"></a>
## [v0.1.1](https://github.com/lealre/aftercredits-backend/compare/v0.1.0...v0.1.1) (2026-08-07)

Remove MongoDB from the codebase now that production runs on Postgres. No
behaviour change: the app, the deploy one-shot, the scheduled job and the
integration suite are untouched in what they do.

* Delete the Mongo store (`internal/mongodb`), the one-time migration engine and CLI (`internal/pgmigration`, `cmd/mongo-to-postgres`), the Mongo-era one-off scripts (`cmd/dev-migrations`) and the fixture generator (`cmd/test-fixtures`)
* Drop the `go.mongodb.org/mongo-driver` dependency
* Convert the test fixtures to the `models.Title` shape so the integration suite no longer decodes through Mongo types; fixture content is unchanged
* Remove the migration binary from both Dockerfiles, the legacy `mongo` service from docker-compose, and the `MONGO_*` blocks from both env examples
* Delete `scripts/backup_to_drive.sh`, which still used `mongodump` and had been superseded by `scripts/backup.sh` and the Pi's `pi/backup_to_drive.sh`
* Strip the vestigial `bson` struct tags from the titles service types and remove the now-historical cutover runbook from `pi/README.md`
* Read the scheduled tasks' database credentials from the deploy `.env` instead of duplicating them in `pi/.env`, so rotating the password cannot silently break the backups
* Rename the compose host-port variable to `POSTGRES_PORT_HOST`, so `POSTGRES_PORT` means the connection port everywhere

<a name="v0.1.0"></a>
## [v0.1.0](https://github.com/lealre/aftercredits-backend/compare/v0.0.12...v0.1.0) (2026-08-07)

Migrate the datastore from MongoDB to Postgres. Behaviour-preserving: no API,
response-shape or auth changes.

* Add a storage-neutral `store.Store` interface and `internal/models` domain types; services and handlers no longer depend on any concrete database package
* Add a Postgres implementation behind that interface: goose schema migrations plus sqlc-generated queries over pgx/v5
* Model titles as a hybrid — queried/sorted fields as real columns, the full title document in a `metadata` JSONB column
* Normalise the relational data: per-season state moves to child tables, and group membership collapses from Mongo's two-sided arrays into a single `group_members` join table
* Add a one-time `mongo-to-postgres` migration CLI that loads a `mongodump` backup in a single transaction and verifies every record by reading it back through the store
* Enforce one comment per (user, title) at the schema level — parity with the unique index the Mongo store had
* Run the app, the deploy one-shot, the scheduled title sync and the integration suite on Postgres; `database -migrate` applies the embedded goose migrations
* Switch backups from `mongodump` to `pg_dump`/`pg_restore`

<a name="v0.0.13"></a>
## v0.0.13 — Episodes on demand and group management (2026-07-31)

* Add GET /titles/{id}/episodes to load episodes on demand
* Omit the embedded episodes array from the group-titles list response (lighter payloads; seasons summary retained)
* Add PATCH /groups/{id} to rename a group (owner only)
* Add DELETE /groups/{id} soft-delete (owner only); excluded from all reads, member group lists cleaned up
* Add DELETE /groups/{id}/users/{userId} to leave a group (non-owner, self)
* Exclude soft-deleted groups from the unique (ownerId, name) index so names can be reused
* Add an optional group description (set on create, editable via PATCH /groups/{id})
* Backfill deleted=false + description="" on existing groups automatically via db-setup (`database -backfill-groups`), before the index reset

<a name="v0.0.12"></a>
## [v0.0.12](https://github.com/lealre/aftercredits-backend/compare/v0.0.11...v0.0.12) (2026-07-25)

* Route all handler DB access through the service layer (no direct api.Db calls)
* Add thin service passthroughs: titles.TitleExists, groups.GroupExists/GroupContainsTitle/EnsureGroupExists, users.UserExists
* Enforce the handlers-don't-touch-the-DB convention in CONVENTIONS.md (behavior-preserving refactor)

<a name="v0.0.11"></a>
## [v0.0.11](https://github.com/lealre/aftercredits-backend/compare/v0.0.10...v0.0.11) (2026-07-25)

* Require JWT_SECRET from env; remove the hardcoded signing secret
* Standardize the DB env var to MONGO_DB with a generic default; align env.example
* Make pagination configurable (DEFAULT_PAGE_SIZE, MAX_PAGE_SIZE, DEFAULT_SEARCH_LIMIT)
* Add an ErrorMap to the titles service for consistent handler error mapping
* Add CONVENTIONS.md (handlers vs services, error mapping, comment invariants)
* Add season/series tests: add TV series to group, get comments for a TV series, group-series aggregation
* Fix CHANGELOG repo URLs (fs-mcp -> aftercredits-backend) via .chglog config
* Guard partial/zero episode release dates in the imdbapi provider

<a name="v0.0.10"></a>
## [v0.0.10](https://github.com/lealre/aftercredits-backend/compare/v0.0.9...v0.0.10) (2026-07-25)

* Add OMDb provider (real IMDb rating + votes, Metacritic) as a full provider
* Add hybrid provider: TMDB metadata + OMDb IMDb ratings (new default deployment)
* Fix ratings showing TMDB's community score instead of the IMDb rating
* Document each provider's strengths/weaknesses (internal/titleprovider/README.md), linked from README
* Add OMDB_API_KEY config; TITLE_PROVIDER now accepts hybrid | tmdb | omdb | imdbapi

<a name="v0.0.9"></a>
## [v0.0.9](https://github.com/lealre/aftercredits-backend/compare/v0.0.8...v0.0.9) (2026-07-19)

* Add pluggable title metadata provider behind a Provider interface
* Add TMDB provider and select provider via TITLE_PROVIDER env var
* Migrate off api.imdbapi.dev (service discontinued) to TMDB
* Keep imdbapi.dev provider for env-var port-back

<a name="v0.0.6"></a>
## [v0.0.6](https://github.com/lealre/aftercredits-backend/compare/v0.0.5...v0.0.6) (2026-02-14)

* Add routines for pi - V0.0.6 ([#8](https://github.com/lealre/aftercredits-backend/issues/8))
* Update CHANGELOG.md
* Update README.md
* Add cron routine to use in pi
* Small improvements in routines scripts
* Add mongo client sample format for vscode extension
* Update script to backup

<a name="v0.0.5"></a>
## [v0.0.5](https://github.com/lealre/aftercredits-backend/compare/v0.0.4...v0.0.5) (2026-02-08)

* Merge pull request [#7](https://github.com/lealre/aftercredits-backend/issues/7) from lealre/v0.0.5
* Use rclone with drive
* Merge pull request [#6](https://github.com/lealre/aftercredits-backend/issues/6) from lealre/v0.0.5
* Add filter option for title type when getting titles from a group
* test: New top level watchedAt logic for seasons
* Update top level information when setting a season as watched
* Command to refresh info about titles from imbd
* Add method to batch titles from api
* Add new dev migration
* Add script to make backup to dropbox
* Update `CHANGELOG.md`

### Pull Requests
* Merge pull request [#7](https://github.com/lealre/aftercredits-backend/issues/7) from lealre/v0.0.5
* Merge pull request [#6](https://github.com/lealre/aftercredits-backend/issues/6) from lealre/v0.0.5


<a name="v0.0.4"></a>
## [v0.0.4](https://github.com/lealre/aftercredits-backend/compare/v0.0.3...v0.0.4) (2026-02-01)

* test: Deletion of ratings for series and movies
* Add deletion of ratings for movie and series
* Merge pull request [#5](https://github.com/lealre/aftercredits-backend/issues/5) from lealre/v0.0.4
* Merge pull request [#4](https://github.com/lealre/aftercredits-backend/issues/4) from lealre/seasons
* test[seasons]: Add tests to the new comments sctruct
* feat[seasons]: Update season struct for comments
* test[seasons]: Add tests to the new ratings sctruct
* feat[seasons]: Update ratings seasons struct
* feat[seasons]: Add back ratings index for ratings
* feat[seasons]: Update imdb episodes fetch to iterate until get the last page of the episodes
* chrome[seasons]: Update dev-migrations to use new titles struct under groups
* feat[seasons]: Add missing fileds for episodes in mapper
* test[seasons]: Add tests to delete a comment from a season
* feat[seasons]: Add endpoint to delete a comment from a season
* test[seasons]: Update group tests to new tv series logic
* feat[seasons]: Pass watched/unwatched information when getting all titles from a group for series
* feat[seasons]: Update groups error handling
* test[seasons]: Update group tests to new tv series logic
* feat[seasons]: Add logic to set watched seasons form tv series
* chore[seasons]: Add migration to update groups to new struct
* test[seasons]: Update group tests to new struct
* feat[seasons]: Update titles struct under groups from an array to a map
* feat[seasons]: Start group logic to update set watched
* chore[seasons]: Remove unused js file with mongo migrations
* test/chore[seasons]: Update script to generate test fixtures
* feat[seaons]: Add more dev migrations for delet and save tv series ratings and comments
* feat[seasons]: Add small documentation on method to update a tv serie comment
* feat/test[seasons]: Add tests to update a comment and update service
* feat/test[seasons]: Remove ambiguous check in ratings test
* feat[seasons]: Update Comments struct type
* feat[seasons]: Add documentation for adding a comment for a tv series flow
* feat[seasons]: Add missing logic fixes for tests
* test[seasons]: Add tests for adding comment for seasons
* test[seasons]: Refactor for test ratings
* feat[seasons]: First cut of adding a comment to a season
* feat[seasons]: Ratings updates for tv series after tests
* feat[seasons]: Add more tests to tv series ratings updates
* feat[seasons]: First cut of season ratings update logic
* test[seasons]: Add tests to when adding tv series ratings
* test[seasons]: Fix fixture titles structs for eppisodes and seasons
* feat[seasons]: Update structs of episodes and seasons for titles and ratings flow
* feat[seasons]: Update seasons information for ratings that already exists
* test[seasons]: Add script to create the test fixtures for tests
* test[seasons]: Split ratings tests for adding and updating
* feat[seasons]: Add seasons to rating level
* feat[seasons]: Add dev-migrations to validate changes in development
* feat[seasons]: Fetch information about season/episodes when title type is series/mini-serie
* test/refactor: Split group tests in different methods
* test: Add more tests for groups creation - empty or duplicated names for same user
* Add api error handling for groups empty or duplicated names
* Add new index do database - unique group name/ownerId combination non-null
* test: Update test to delet a user to actually try to delete another user that exists
* test: Update endpoint to user getting own information
* Add endpoint to get current user information based on the token
* Use common logic to database index creation for both cli migrationa and tests
* Add indexes logic to mongodb internal package
* test: Sync database migration with cli and add time.sleep to comments before test updating
* Remove old dockerhub push script
* Add flag to accept exisitng volume in `docker-compose.yaml`
* Update script to push image to dockerhub
* Update docker setup to push to hub
* fix: database filter was returning just name of the users from a group
* fix: groups with no titles was returning all titles from collections
* Update cli to delte/reset indexes and update indexes names
* Update `README.md`
* Update `env.example`
* Update `Dockerfile` to use the new database cli
* Add option to create a superuser in databse cli
* Add `Dockerfile` to push images to docker hub
* Update `README.md` with new repo name and how to dump/retore mongo data
* Add go cmd to create the database indexes
* Break `MONGO_URI` in its own variables
* Change how volumes are managed in docker and create scripts to dump/restore data
* Update changelog for version 0.0.3

### Pull Requests
* Merge pull request [#5](https://github.com/lealre/aftercredits-backend/issues/5) from lealre/v0.0.4
* Merge pull request [#4](https://github.com/lealre/aftercredits-backend/issues/4) from lealre/seasons


<a name="v0.0.3"></a>
## [v0.0.3](https://github.com/lealre/aftercredits-backend/compare/v0.0.2...v0.0.3) (2025-12-13)

* Add basic authentication and authorization ([#3](https://github.com/lealre/aftercredits-backend/issues/3))
* Add missing return statment
* Remove cmd folder from git track
* Add endpoint to get a group
* fix: Json fields to add a comment
* test: Extend group tests to check the user update when creating/adding a user
* Update groups enpoints to when creating a group or adding a user, also updating the user groups records in user collections. Just group owners can add new users to a group
* Update login response to send user info with token and json fields in api ErrorResponse to be camelCase
* test: Add basic tests to delete comments
* Update endpoint to delete comments
* test: Add basic tests to update comments
* Update endpoint to update comments
* fix: Group endpoint name to get comments form a title
* test: Add basic tests to get comments
* Update endpoint to get comments
* test: Add basic tests to add comments
* Update endpoint to add comments
* Update packages versions and rebase
* Refactor note ranghe validation for rating endpoint
* test: Add test to note validation when adding a note
* test: Extend tests to update ratings
* Remove endpoit to get batch of ratings
* test: Start tests for updating a rating
* Update enpoint to update a rating
* test: Add tests for adding ratings
* Update method to add rating to check group/title combination
* test: Refactor test setup in new reusable methods
* test: Add tests to setting a movie as watched related to auth
* fix: Groups endpoints related to permissions
* test: Extend groups endpoints test to auth
* Update groups and ratings endpoints based on auht
* Update comments endpoints to use auth
* Update ratings endpoints to use auth
* test: Update admin titles enpoints tests to use token
* Refactor titles endpoints to be used just for admin role
* Refactor auth workflow
* Update users errors lookup
* test: Update groups enpoints tests to use token
* test: Update database migration in test setup
* Add authorization checks in groups endpoints
* test: Add minimal tests to get and update users
* Add endpoint to get user and a very simple for update fields for now
* test: Fix test to delete a user
* Update endpoint to delete a user
* test: User creation validation and deletion with auth
* Add minimal fields validation for user creation
* Add user to context and authorizarion to users endpoints
* Add username on user response
* Update mongo migration scripts
* Base auth login handler
* Apply base global auth middleware to handlers
* Update packages versions
* test: Add test to new group watched fix
* fix(groups): Setting whatchedAt date when watched is false returns 400
* test: Add more tests to group titles api
* fix(groups): Setting a title as unwatched should always clear watchedAt field in database
* test: Start test to goup titles endpoints
* test: Add setup folder to each goroup and extend test to get users from a group
* test: Endpoint to add a user to a group
* Add endpoint to add a user to a group

<a name="v0.0.2"></a>
## [v0.0.2](https://github.com/lealre/aftercredits-backend/compare/v0.0.1...v0.0.2) (2025-11-19)

* Update changelog for version `0.0.2`
* Add groups management for users/titles ([#2](https://github.com/lealre/aftercredits-backend/issues/2))
* test: update titles test
* refactor: add default response in api for messages
* refactor: separate mappers inside services
* Add small documentation on custom titles pagination
* Remove watched field from titles when adding it
* Fix group titles pagination
* Add endpoint to remove title from a group
* Add check validation to title already in a group
* Add endpoint to update the title watche state in a group
* Add endpoint to add titles to a  group
* Add endpoint to get users from a group
* Fix bson placeholder to groups and json placeholder to ratings in groups
* Add endpoint to get titles by group ID with embedded ratings
* test: create group endpoint
* Add endpoint to create a group
* test: delete users endpoint
* Add endpoint to delete user (no auth for now)
* Check duplicated username (with test)
* Add POST to create user with minimal test and auth package
* Add first version of groups collection and extend users collections field
* Add small changelog configuration

<a name="v0.0.1"></a>
## v0.0.1 (2025-11-05)

* Add more tests to title api
* Add base sctruct for testing using testcontainers
* Remove related comments when title is deleted
* Remove hash check in `scripts/backup-volume.sh`
* Refactor how handlers uses the database instance ([#1](https://github.com/lealre/aftercredits-backend/issues/1))
* Complete refactor to users and comments
* Complete refactor to titles and ratings
* New base struct used to inject DB on apis
* Update logic to orderBy field in titles
* Add createdAt and updatedAt fields to ratings
* Add base code to the new struct of comments
* Add small notes on setWatched endpoint and update fields from movie to titles
* Refactor ratings code
* Refactor users code
* Remove txt file and update backup script
* Refactor titles code
* Add endpoint to batch titles ratings
* Add imdb rating filed to orderby
* Add url query to order ascending/descending and add type of title in titles response
* Change watched filter to fetch from backend
* Refactor code by adding the server folder and separate the handlers by files
* Fixing the requestID isolation per request
* Adding the `watchedAt` field as option to be updated
* Setting basic pagination to titles
* Script to add new columns in title and start titles pagination
* Add script to backup and use local volume to seed mongoDB data
* Add delete titles endpoint using cascade with ratings
* Add watched field to all records and new endpoint to udpate it
* Remove sample data
* Add rating update and change rating note to float32
* Add endpoint to get the users
* Update crypto package version
* Fix bugs when adding title and getting title rating
* Small changes to make firts test of POST ratings work
* Add mongdb scripts to vscode client. Add index to ratings (titleId + userId)
* Add basic logic to ratings
* More refactor; add interfaces for rating handlers endpointsa; and add user service
* Add basic operations to add a movie from url and get the full movies list
* Start backend - Fetch movie from api and MongoDb basic crud
* Initial commit
