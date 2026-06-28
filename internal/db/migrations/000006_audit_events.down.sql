-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 006 rollback: audit_events
-- ─────────────────────────────────────────────────────────────────────────────

DROP RULE IF EXISTS audit_events_no_delete ON audit_events;
DROP RULE IF EXISTS audit_events_no_update ON audit_events;
DROP TABLE IF EXISTS audit_events;
