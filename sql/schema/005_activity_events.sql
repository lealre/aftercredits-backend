-- +goose Up
-- The append-only log behind the group activity feed. State tables answer
-- "what is true"; this answers "what happened" — which is why the feed is not
-- derived from ratings/comments (deletes leave nothing behind, and only the
-- latest update survives on a row).
--
-- actor_name and title_name are denormalized on purpose: the actor's full user
-- is already in the request context so the name is free, and a row must stay
-- readable after the member leaves the group or the title is deleted from the
-- catalogue. Renames therefore do not propagate — the accepted trade.

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

-- title_id carries no foreign key deliberately: an admin deleting a title from
-- the catalogue must not delete the history of what people did with it.
CREATE INDEX activity_events_group_seq_idx ON activity_events(group_id, seq DESC);

-- One watermark per user rather than a row per (event, user): the badge only
-- ever needs a count and "mark all read", so per-event rows would grow without
-- bound to power granularity nothing uses. An absent row means "never read".
-- read_seq is the authoritative watermark — the same key the feed paginates by,
-- so "unread" and "after this page" cannot disagree. read_at is informational.
CREATE TABLE activity_reads (
    user_id  TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    read_at  TIMESTAMPTZ NOT NULL,
    read_seq BIGINT NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE activity_reads;
DROP TABLE activity_events;
