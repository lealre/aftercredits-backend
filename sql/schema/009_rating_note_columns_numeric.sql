-- +goose Up
-- 006 repaired every stored note to its intended one-decimal value but left
-- the column type alone: both ratings.note and rating_seasons.rating are
-- still REAL (float32), so the class of bug 006 fixed can reopen the moment
-- a new row lands with residual binary noise. It already has, in production:
-- a feed line read "rated <title> with note 5.599999904632568" because
-- float32(5.6) is 5.5999999046325684 and widening it to float64 only
-- resurfaces every digit. This migration finishes what 006 started by
-- changing the type, not just the value: NUMERIC(3,1) stores a one-decimal
-- note exactly, so there is no binary expansion left to leak. NUMERIC(3,1)
-- holds 0.0-99.9, comfortably covering the note range (0-10, so the maximum
-- 10.0 fits with room to spare).
--
-- Both ALTERs round explicitly as they convert, and that repair is safe for
-- exactly the reason 006's was: round(x, 1) is a total function of the
-- stored value alone, computed with no other row or table consulted, so
-- there is no case to detect and nothing for an operator to decide. 006 only
-- repaired the rows that existed at the time — every write since has passed
-- validateNote's *tolerance* check (within 1e-3 of a tenth), not an
-- *exactness* check, so a row written yesterday can still carry the same
-- float32 residue 006 was cleaning up, and this is the one remaining chance
-- to resolve it rather than carry it across as a now exact-looking NUMERIC.
--
-- The round() is, admittedly, belt and suspenders here: ALTER COLUMN TYPE
-- casts the USING expression's result to the column's declared type as its
-- last step, and that cast to NUMERIC(3,1) would itself coerce a
-- two-decimal intermediate value to one decimal even without the explicit
-- round() — verified directly, not assumed. It is kept anyway because
-- relying on that implicit coercion would make this statement's correctness
-- depend on a Postgres subtlety a reader would have to know to trust it,
-- where an explicit round(x, 1) is legible on its own and costs nothing.
--
-- The cast is written `note::numeric` rather than through double precision
-- deliberately, though not because the alternative would break this
-- statement. A REAL cast straight to numeric takes float4's
-- shortest-round-trip decimal, so a stored 6.7 casts to exactly 6.7 and the
-- round() is a no-op; going via float8 would widen it to 6.699999809265137
-- first. Checked against every distinct note in a real database, both orders
-- produce identical results here, because the explicit round(...,1) absorbs
-- the widening either way.
--
-- The trap is real, but it belongs to 006, whose WHERE clause compared the
-- two sides WITHOUT rounding and so matched every already-correct row and
-- rewrote it on every run. Naming it here is worth doing — the same
-- expression appears in both files and a reader will wonder — but claiming
-- this statement depends on it would overstate the danger. The direct cast
-- is simply the one that needs no rounding to be correct.
--
-- This is necessarily a full table rewrite — Postgres has no incremental
-- path for a column type change — same as every other ALTER COLUMN TYPE in
-- this schema's history. It runs as the deploy's migration step, before the
-- server starts, per this project's migration convention (docs/CONVENTIONS.md
-- #7), so a lock held for the rewrite is expected there, not a surprise.
ALTER TABLE ratings
    ALTER COLUMN note TYPE NUMERIC(3,1)
    USING round(note::numeric, 1);

ALTER TABLE rating_seasons
    ALTER COLUMN rating TYPE NUMERIC(3,1)
    USING round(rating::numeric, 1);

-- +goose Down
-- Unlike 006, this direction has no ambiguous case to refuse: every
-- NUMERIC(3,1) value is in range for a REAL, so a plain cast is the whole
-- downgrade. It is lossy in the same sense the column always was before this
-- migration (REAL cannot exactly hold every one-decimal value) — that
-- reintroduced imprecision is the point of rolling back, not a defect in the
-- rollback.
ALTER TABLE rating_seasons
    ALTER COLUMN rating TYPE REAL
    USING rating::real;

ALTER TABLE ratings
    ALTER COLUMN note TYPE REAL
    USING note::real;
