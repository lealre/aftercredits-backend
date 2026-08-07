-- +goose Up
-- The Mongo store enforced one comment per (user, title) via the
-- userId_and_titleId_unique index; 001_init.sql missed the equivalent.
ALTER TABLE comments ADD CONSTRAINT comments_user_id_title_id_key UNIQUE (user_id, title_id);

-- +goose Down
ALTER TABLE comments DROP CONSTRAINT comments_user_id_title_id_key;
