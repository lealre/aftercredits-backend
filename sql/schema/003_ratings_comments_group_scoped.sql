-- +goose Up
-- Ratings and comments were one fact per (user, title), globally unique and so
-- shared by every group the author belongs to: a group's title detail served
-- every other group's ratings, and a user could not rate the same title
-- differently in two groups. Make the group part of their identity —
-- (user_id, title_id, group_id), group_id NOT NULL — so a rating or comment
-- can never be read or written without a group.
--
-- The backfill refuses to guess. Today every row maps to exactly one group, but
-- LeaveGroup and RemoveTitleFromGroup can make a row unattributable between now
-- and deploy; a naive UPDATE ... FROM would silently pick an arbitrary group.
-- The assertion below runs before any write, and goose wraps the migration in a
-- transaction, so a raise rolls back the DDL too and the deploy fails cleanly
-- and re-runnably.
ALTER TABLE ratings ADD COLUMN group_id TEXT;
ALTER TABLE comments ADD COLUMN group_id TEXT;

-- Every (row, candidate group) pair: the groups that hold the title, that the
-- row's author is a member of, and that are not soft-deleted.
CREATE TEMP TABLE group_scope_candidates AS
SELECT
    'ratings'::TEXT AS source_table,
    r.id AS row_id,
    r.user_id AS user_id,
    r.title_id AS title_id,
    gt.group_id AS group_id
FROM ratings r
JOIN group_titles gt ON gt.title_id = r.title_id
JOIN group_members gm ON gm.group_id = gt.group_id AND gm.user_id = r.user_id
JOIN groups g ON g.id = gt.group_id AND NOT g.deleted
UNION ALL
SELECT
    'comments'::TEXT,
    c.id,
    c.user_id,
    c.title_id,
    gt.group_id
FROM comments c
JOIN group_titles gt ON gt.title_id = c.title_id
JOIN group_members gm ON gm.group_id = gt.group_id AND gm.user_id = c.user_id
JOIN groups g ON g.id = gt.group_id AND NOT g.deleted;

-- +goose StatementBegin
DO $$
DECLARE
    ambiguous TEXT;
    orphaned TEXT;
BEGIN
    SELECT string_agg(a.detail, E'\n' ORDER BY a.detail)
    INTO ambiguous
    FROM (
        -- user_id and title_id are functionally dependent on row_id, so min()
        -- just carries them through; the operator needs the title to act on the
        -- hint below.
        SELECT format(
            '%s.%s (user %s, title %s) -> %s',
            source_table,
            row_id,
            min(user_id),
            min(title_id),
            string_agg(DISTINCT group_id, ',' ORDER BY group_id)
        ) AS detail
        FROM group_scope_candidates
        GROUP BY source_table, row_id
        HAVING count(DISTINCT group_id) > 1
    ) a;

    IF ambiguous IS NOT NULL THEN
        RAISE EXCEPTION E'cannot backfill group_id: rows map to more than one group:\n%', ambiguous
            USING HINT = 'Each listed row is shared by several groups holding the title. Decide which group owns it (delete the row, or remove the title from the other groups) and re-run the migration.';
    END IF;

    SELECT string_agg(o.detail, E'\n' ORDER BY o.detail)
    INTO orphaned
    FROM (
        SELECT format(
            '%s.%s (user %s, title %s)',
            t.source_table,
            t.id,
            t.user_id,
            t.title_id
        ) AS detail
        FROM (
            SELECT 'ratings'::TEXT AS source_table, id, user_id, title_id FROM ratings
            UNION ALL
            SELECT 'comments'::TEXT, id, user_id, title_id FROM comments
        ) t
        WHERE NOT EXISTS (
            SELECT 1
            FROM group_scope_candidates c
            WHERE c.source_table = t.source_table AND c.row_id = t.id
        )
    ) o;

    IF orphaned IS NOT NULL THEN
        RAISE EXCEPTION E'cannot backfill group_id: rows map to no group:\n%', orphaned
            USING HINT = 'Each listed row has no live group that both holds the title and has the author as a member — the author left, or the title was removed. Delete the row or restore the membership/title, then re-run the migration.';
    END IF;
END
$$;
-- +goose StatementEnd

UPDATE ratings r
SET group_id = c.group_id
FROM group_scope_candidates c
WHERE c.source_table = 'ratings' AND c.row_id = r.id;

UPDATE comments cm
SET group_id = c.group_id
FROM group_scope_candidates c
WHERE c.source_table = 'comments' AND c.row_id = cm.id;

DROP TABLE group_scope_candidates;

ALTER TABLE ratings ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE comments ALTER COLUMN group_id SET NOT NULL;

ALTER TABLE ratings DROP CONSTRAINT ratings_user_id_title_id_key;
ALTER TABLE comments DROP CONSTRAINT comments_user_id_title_id_key;

ALTER TABLE ratings ADD CONSTRAINT ratings_user_id_title_id_group_id_unique UNIQUE (user_id, title_id, group_id);
ALTER TABLE comments ADD CONSTRAINT comments_user_id_title_id_group_id_unique UNIQUE (user_id, title_id, group_id);

-- NO ACTION, deliberately not ON DELETE CASCADE: groups are only ever
-- soft-deleted, so this FK never fires today. CASCADE would silently destroy
-- every rating and comment in a group the first time someone purges
-- soft-deleted rows; NO ACTION makes that purge fail loudly instead.
ALTER TABLE ratings ADD CONSTRAINT ratings_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups(id);
ALTER TABLE comments ADD CONSTRAINT comments_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups(id);

CREATE INDEX ratings_group_id_idx ON ratings(group_id);
CREATE INDEX comments_group_id_idx ON comments(group_id);

-- +goose Down
DROP INDEX comments_group_id_idx;
DROP INDEX ratings_group_id_idx;

ALTER TABLE comments DROP CONSTRAINT comments_group_id_fkey;
ALTER TABLE ratings DROP CONSTRAINT ratings_group_id_fkey;

ALTER TABLE comments DROP CONSTRAINT comments_user_id_title_id_group_id_unique;
ALTER TABLE ratings DROP CONSTRAINT ratings_user_id_title_id_group_id_unique;

-- One-way once used: re-adding the global uniqueness fails as soon as any user
-- holds two rows for one title in different groups, because collapsing them
-- would have to discard a real rating or comment.
ALTER TABLE comments ADD CONSTRAINT comments_user_id_title_id_key UNIQUE (user_id, title_id);
ALTER TABLE ratings ADD CONSTRAINT ratings_user_id_title_id_key UNIQUE (user_id, title_id);

ALTER TABLE comments DROP COLUMN group_id;
ALTER TABLE ratings DROP COLUMN group_id;
