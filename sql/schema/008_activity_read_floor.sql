-- +goose Up
-- A read floor underneath the per-event read rows 007 introduced.
--
-- 007 stores one row per (user, event) read, which is what makes "mark this one
-- row read" mean exactly that. The cost is that "mark all as read" writes a row
-- per visible event and the table grows with users x events, forever. This adds
-- a floor: a seq at or below which everything is already read for that reader.
-- Marking everything read then raises one integer and deletes the rows the
-- floor now covers, instead of inserting one per event.
--
-- Read state is therefore "seq <= floor OR an exception row exists", and the
-- exception rows only ever describe events ABOVE the floor. That is still one
-- source of truth for a single event's state — the two halves cannot disagree,
-- because they answer disjoint ranges of seq — which is the property the badge
-- and the feed depend on.
--
-- KNOWN CAVEAT, accepted: seq is assigned at INSERT, not at COMMIT (see the
-- spec's "Ordering and cursors"). A transaction that starts earlier but commits
-- later can carry a lower seq than one already delivered, so raising the floor
-- can swallow an event that commits afterwards below it — it would never show
-- as unread. The watermark this project started with had exactly that property
-- everywhere; here it is confined to mark-all, and per-row marking is unaffected.
-- At this scale (a handful of users, effectively one writer at a time) the
-- window is vanishingly small and the cost is one feed line not being bold.
--
-- CONSEQUENCE, verified by test rather than assumed: the floor is one number
-- per user, not one per group, so it spans every group they are in. Joining a
-- group therefore behaves differently depending on the joiner:
--
--   * a reader with no floor yet sees that group's whole backlog as unread,
--     exactly as 007 alone would have;
--   * a reader who has cleared their badge has a floor above that history, so
--     the backlog arrives already read.
--
-- The second case is the lossy one, and it is accepted: a new member did not
-- miss those events, they were not there. Activity after joining is above the
-- floor either way, so joining costs the backlog only, never future events.
-- A per-group floor would remove the asymmetry at the price of a row per
-- (user, group) and a more expensive read; revisit only if it actually bites.
CREATE TABLE activity_read_floors (
    user_id   TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    floor_seq BIGINT NOT NULL,
    read_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE activity_read_floors;
