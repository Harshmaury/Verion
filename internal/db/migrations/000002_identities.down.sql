-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 002 rollback: identities
-- ─────────────────────────────────────────────────────────────────────────────

DROP TRIGGER IF EXISTS identities_updated_at ON identities;
DROP TABLE IF EXISTS identities;
DROP TYPE IF EXISTS identity_status;
DROP TYPE IF EXISTS identity_type;
