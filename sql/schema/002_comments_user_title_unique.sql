-- +goose Up
-- One comment per (user, title) was enforced by a unique index in the previous
-- datastore; 001_init.sql missed the equivalent.
ALTER TABLE comments ADD CONSTRAINT comments_user_id_title_id_key UNIQUE (user_id, title_id);

-- +goose Down
ALTER TABLE comments DROP CONSTRAINT comments_user_id_title_id_key;
