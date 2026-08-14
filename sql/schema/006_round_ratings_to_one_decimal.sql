-- +goose Up
-- A note has always been meant to be a one-decimal value (x.x), but nothing
-- enforced it: the range check was the only validation, and the UI's
-- step="0.1" is a browser hint rather than a constraint, so a client that
-- posted 8.55 had it stored verbatim. The write path now rejects finer input
-- (and rounds the derived TV-series mean), which leaves the rows written
-- before that to be brought up to the same contract.
--
-- On the standing question of whether a data repair belongs in a migration:
-- 003's backfill was contentious because it had to *guess* — it derived
-- group_id from other tables, some rows mapped to several groups and some to
-- none, and picking one silently would have invented a fact, which is why it
-- needed a guard block that aborts the deploy. This repair has no such case.
-- Rounding is total: every stored value has exactly one rounded form, computed
-- from that value alone with no other table consulted, so there is nothing to
-- detect, nothing to escalate to an operator, and no decision a guard could
-- usefully make. A plain UPDATE is the whole repair.
--
-- Both columns are REAL (float32), where most one-decimal values are not
-- exactly representable — 6.7 stores as 6.69999980926513671875. Both sides of
-- the comparison are therefore cast to numeric. Writing the left side as the
-- bare column instead (`note <> round(note::numeric, 1)`) is the trap: with no
-- real-to-numeric operator, Postgres promotes both sides to double precision,
-- the stored 6.7 widens to 6.69999980926513671875 while the right side is a
-- clean 6.7, and every already-correct row matches and is rewritten on every
-- run. Casting the column to numeric avoids that — float4's numeric cast is
-- the shortest representation identifying the value, so a stored 6.7 casts to
-- exactly 6.7 and the WHERE excludes it.
--
-- Rounding is idempotent in value regardless, since round(round(x)) = round(x);
-- what the WHERE adds is that a second run matches zero rows rather than
-- rewriting every row to the value it already holds.
UPDATE ratings
SET note = round(note::numeric, 1)
WHERE note::numeric <> round(note::numeric, 1);

UPDATE rating_seasons
SET rating = round(rating::numeric, 1)
WHERE rating::numeric <> round(rating::numeric, 1);

-- +goose Down
-- Irreversible by nature: the discarded digits are recorded nowhere, so there
-- is nothing to restore. A no-op rather than a failure, so rolling back past
-- this migration is not blocked by a repair a roll-back cannot undo.
SELECT 1;
