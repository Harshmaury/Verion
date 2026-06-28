-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 003 rollback: identity_keys
-- ─────────────────────────────────────────────────────────────────────────────

DROP TABLE IF EXISTS identity_keys;
DROP TYPE IF EXISTS key_status;
DROP TYPE IF EXISTS key_purpose;
DROP TYPE IF EXISTS key_type;
