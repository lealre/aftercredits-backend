-- +goose Up
-- group_titles.title_type was written exactly once (add-to-group) with a
-- hard-coded 'movie' for every title — a vestige carried over from Mongo.
-- Nothing reads it; titles.type is the only source of truth for a title's
-- type. Deploy-wise this rides the existing rule from migration 003: stop
-- the backend before `database -migrate` (the old binary still writes the
-- column on add-to-group).
ALTER TABLE group_titles DROP COLUMN title_type;

-- +goose Down
-- The dropped values are not restorable — they were the hard-coded 'movie'
-- for every row anyway, which the DEFAULT reproduces faithfully.
ALTER TABLE group_titles ADD COLUMN title_type TEXT NOT NULL DEFAULT '';
