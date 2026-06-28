-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 007 rollback: primary_key_ref + RLS
-- ─────────────────────────────────────────────────────────────────────────────

-- Drop RLS policies
DROP POLICY IF EXISTS audit_events_isolation    ON audit_events;
DROP POLICY IF EXISTS recovery_methods_isolation ON recovery_methods;
DROP POLICY IF EXISTS credentials_isolation      ON credentials;
DROP POLICY IF EXISTS identity_keys_isolation    ON identity_keys;
DROP POLICY IF EXISTS identities_isolation       ON identities;
DROP POLICY IF EXISTS tenants_isolation          ON tenants;

-- Disable RLS
ALTER TABLE audit_events     DISABLE ROW LEVEL SECURITY;
ALTER TABLE recovery_methods DISABLE ROW LEVEL SECURITY;
ALTER TABLE credentials      DISABLE ROW LEVEL SECURITY;
ALTER TABLE identity_keys    DISABLE ROW LEVEL SECURITY;
ALTER TABLE identities       DISABLE ROW LEVEL SECURITY;
ALTER TABLE tenants          DISABLE ROW LEVEL SECURITY;

-- Drop primary_key_id column
ALTER TABLE identities DROP COLUMN IF EXISTS primary_key_id;
