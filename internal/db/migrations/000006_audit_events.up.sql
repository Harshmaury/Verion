-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 006: audit_events
-- Immutable append-only audit log for all identity system events
-- NO UPDATE or DELETE ever issued against this table
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE audit_events (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID        NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,

    -- What happened
    event_type      TEXT        NOT NULL,   -- e.g. 'identity.created', 'key.rotated', 'credential.revoked'
    entity_type     TEXT        NOT NULL,   -- 'identity' | 'key' | 'credential' | 'tenant' | 'recovery'
    entity_id       UUID        NOT NULL,

    -- Who did it
    actor_id        UUID        NULL,       -- Identity who performed the action
    actor_type      TEXT        NULL,       -- 'human' | 'service' | 'system' | 'ai_agent'

    -- Request context
    ip_address      INET        NULL,
    user_agent      TEXT        NULL,
    session_id      UUID        NULL,
    request_id      TEXT        NULL,       -- Distributed trace request ID

    -- Change data (what changed)
    before_state    JSONB       NULL,       -- State before the change
    after_state     JSONB       NULL,       -- State after the change
    metadata        JSONB       NOT NULL DEFAULT '{}',

    -- Outcome
    success         BOOLEAN     NOT NULL DEFAULT true,
    error_code      TEXT        NULL,       -- If success = false

    -- Immutable timestamp — set once, never changed
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Protect immutability: prevent UPDATE and DELETE at DB level
CREATE OR REPLACE RULE audit_events_no_update AS
    ON UPDATE TO audit_events DO INSTEAD NOTHING;

CREATE OR REPLACE RULE audit_events_no_delete AS
    ON DELETE TO audit_events DO INSTEAD NOTHING;

-- Query patterns
CREATE INDEX audit_events_tenant_id_idx     ON audit_events (tenant_id);
CREATE INDEX audit_events_entity_idx        ON audit_events (entity_type, entity_id);
CREATE INDEX audit_events_actor_id_idx      ON audit_events (actor_id) WHERE actor_id IS NOT NULL;
CREATE INDEX audit_events_event_type_idx    ON audit_events (event_type);
CREATE INDEX audit_events_occurred_at_idx   ON audit_events (occurred_at DESC);
CREATE INDEX audit_events_tenant_time_idx   ON audit_events (tenant_id, occurred_at DESC);

COMMENT ON TABLE audit_events IS 'Immutable append-only audit log. No UPDATE or DELETE ever. Partitioning by occurred_at planned for Phase 6.';
COMMENT ON COLUMN audit_events.before_state IS 'Full entity state before the change. NULL for creation events.';
COMMENT ON COLUMN audit_events.after_state IS 'Full entity state after the change. NULL for deletion/deactivation events.';
