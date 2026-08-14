-- +goose Up
-- Per-event read state, replacing the single watermark 005 introduced.
--
-- A watermark can only ever express a contiguous newest-N: "unread" meant
-- "newer than read_seq", so marking one event read necessarily marked every
-- older event read too, and marking the newest cleared the whole badge. The UI
-- needs the opposite — clicking one row marks exactly that row and drops the
-- badge by one, leaving newer and older rows alone — and that is only
-- representable as a row per (user, event).
--
-- The cost 005 was avoiding is accepted deliberately here: this table grows
-- with users x events read, and "mark all as read" writes a row per visible
-- event instead of moving one integer. That is a choice, not an oversight. The
-- alternative — keeping the watermark as a floor underneath the per-event rows
-- — buys an O(1) mark-all at the price of two sources of truth for one fact,
-- and the badge and the feed must never disagree about a single event.
--
-- Both foreign keys cascade, mirroring the rows they describe: a read row says
-- "this user has seen this event", which is meaningless once either side is
-- gone. activity_events.group_id already cascades from groups, so deleting a
-- group deletes its events and now their read rows with them, and
-- activity_reads.user_id cascaded from users, which this keeps.
--
-- activity_reads is dropped rather than backfilled into rows: the feature has
-- never been enabled in production, so there is no real read state to preserve,
-- and expanding a watermark into one row per event below it would invent
-- exactly the volume this table is otherwise careful about. Down recreates it
-- empty, which is the honest inverse — the watermark's data cannot come back
-- from rows that were never written.
CREATE TABLE activity_event_reads (
    user_id  TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_id TEXT NOT NULL REFERENCES activity_events(id) ON DELETE CASCADE,
    read_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, event_id)
);

-- The primary key already answers "has this user read this event" and "every
-- event this user has read". This index is for the other direction: deleting a
-- group cascades into its events and from there into this table, and without an
-- index on the referencing column that cascade is a sequential scan of every
-- read row in the database.
CREATE INDEX activity_event_reads_event_id_idx ON activity_event_reads(event_id);

DROP TABLE activity_reads;

-- The feed, the badge, marking one event read and marking all of them read are
-- four queries that must agree exactly on which events a reader can see — a
-- badge counting an event the feed will not show is a permanently stuck badge.
-- The predicate therefore lives here, once, instead of being copied into each
-- of them: your groups, never your own actions, and nothing from a deleted
-- group. reader_id comes from the membership join, so a reader appears once per
-- group they belong to and every caller filters by it.
CREATE VIEW activity_visible_events AS
SELECT
    e.id,
    e.seq,
    e.group_id,
    e.actor_id,
    e.actor_name,
    e.kind,
    e.title_id,
    e.title_name,
    e.payload,
    e.created_at,
    g.name AS group_name,
    m.user_id AS reader_id
FROM activity_events e
JOIN group_members m ON m.group_id = e.group_id
JOIN groups g ON g.id = e.group_id AND NOT g.deleted
WHERE e.actor_id <> m.user_id;

-- +goose Down
DROP VIEW activity_visible_events;

CREATE TABLE activity_reads (
    user_id  TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    read_at  TIMESTAMPTZ NOT NULL,
    read_seq BIGINT NOT NULL DEFAULT 0
);

DROP TABLE activity_event_reads;
