-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 005 rollback: recovery_methods
-- ─────────────────────────────────────────────────────────────────────────────

DROP TABLE IF EXISTS recovery_methods;
DROP TYPE IF EXISTS recovery_status;
DROP TYPE IF EXISTS recovery_type;
