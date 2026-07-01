-- ─────────────────────────────────────────────────────────────────────────────
-- Verion — PostgreSQL Initialization
-- Runs once when the container is first created
-- ─────────────────────────────────────────────────────────────────────────────

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Enable pgcrypto for server-side crypto helpers
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Enable citext for case-insensitive text (handles/emails)
CREATE EXTENSION IF NOT EXISTS "citext";

-- Confirm
SELECT 'Verion PostgreSQL initialized successfully' AS status;
