-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 004 rollback: credentials
-- ─────────────────────────────────────────────────────────────────────────────

DROP TABLE IF EXISTS credentials;
DROP TYPE IF EXISTS credential_status;
DROP TYPE IF EXISTS credential_type;
