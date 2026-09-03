-- +goose Up
-- Schema groundwork for exposing the API on the open internet. Three
-- independent, additive changes bundled into one migration so the deploy
-- applies them in a single step.
--
-- 1. users.token_version — the revocation primitive. A JWT now carries the
--    version it was minted under; AuthMiddleware compares it against the row's
--    current value on every request (the row it already loads), so bumping this
--    integer invalidates every token that user holds. Password change bumps it;
--    an explicit "log out everywhere" bumps it. Existing tokens all carry the
--    default 0, so this is transparent until the first bump.
ALTER TABLE users ADD COLUMN token_version INT NOT NULL DEFAULT 0;

-- 2. group_members(user_id) index. GetUserGroupIds filters on user_id alone,
--    and AuthMiddleware runs it on every authenticated request. The only index
--    on this table is the (group_id, user_id) primary key, which cannot serve a
--    user_id-leading lookup, so every authenticated request was a sequential
--    scan waiting for the table to grow. This makes it an index scan.
CREATE INDEX IF NOT EXISTS group_members_user_id_idx ON group_members(user_id);

-- 3. Length ceilings on user-controlled text, backed by DB CHECK constraints so
--    the cap survives a code path that forgets to validate. Application-level
--    validation is the first line (a clean 400); these are the backstop.
--
--    A CHECK added to a table with an over-long existing row would fail the
--    migration and, because the server waits on db-setup completing, crash-loop
--    the whole stack. So each column is truncated to its ceiling FIRST, in the
--    same transaction, before the constraint is added. Truncation is a no-op on
--    a well-behaved family database and self-heals anything that predates the
--    caps. char_length (characters, not bytes) matches the rune-counted caps
--    the service enforces. NOT VALID is not used: the tables are tiny and a
--    full validation here is instant, and a validated constraint is the point.
UPDATE users SET name = left(name, 64) WHERE char_length(name) > 64;
UPDATE users SET email = left(email, 254) WHERE char_length(email) > 254;
UPDATE users SET username = left(username, 32) WHERE char_length(username) > 32;
ALTER TABLE users ADD CONSTRAINT users_name_len_chk CHECK (char_length(name) <= 64);
ALTER TABLE users ADD CONSTRAINT users_email_len_chk CHECK (char_length(email) <= 254);
ALTER TABLE users ADD CONSTRAINT users_username_len_chk CHECK (char_length(username) <= 32);

UPDATE groups SET name = left(name, 64) WHERE char_length(name) > 64;
UPDATE groups SET description = left(description, 500) WHERE char_length(description) > 500;
ALTER TABLE groups ADD CONSTRAINT groups_name_len_chk CHECK (char_length(name) <= 64);
ALTER TABLE groups ADD CONSTRAINT groups_description_len_chk CHECK (char_length(description) <= 500);

UPDATE comments SET comment = left(comment, 2000) WHERE comment IS NOT NULL AND char_length(comment) > 2000;
ALTER TABLE comments ADD CONSTRAINT comments_len_chk CHECK (comment IS NULL OR char_length(comment) <= 2000);

-- Per-season comment text (TV series) shares the comment ceiling.
UPDATE comment_seasons SET comment = left(comment, 2000) WHERE char_length(comment) > 2000;
ALTER TABLE comment_seasons ADD CONSTRAINT comment_seasons_len_chk CHECK (char_length(comment) <= 2000);

-- +goose Down
ALTER TABLE comment_seasons DROP CONSTRAINT IF EXISTS comment_seasons_len_chk;
ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_len_chk;
ALTER TABLE groups DROP CONSTRAINT IF EXISTS groups_description_len_chk;
ALTER TABLE groups DROP CONSTRAINT IF EXISTS groups_name_len_chk;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_len_chk;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_len_chk;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_name_len_chk;
DROP INDEX IF EXISTS group_members_user_id_idx;
ALTER TABLE users DROP COLUMN IF EXISTS token_version;
