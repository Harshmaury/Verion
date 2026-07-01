#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Verion — Docker Infrastructure Setup
# Run from: /home/harsh/workspace/projects/Verion
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

DROP="/mnt/c/Users/harsh/Downloads/Verion-drop"
TIMESTAMP="20260627_122646"

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║     VERION — Phase 1 · Step 1 · Docker Infrastructure   ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

# ── Copy files ────────────────────────────────────────────────────────────────

echo "[1/6] Copying docker-compose.yml..."
cp "$DROP/verion-docker-compose-${TIMESTAMP}.yml" docker-compose.yml
echo "      ✓ docker-compose.yml"

echo "[2/6] Creating postgres init directory..."
mkdir -p deployments/docker/postgres
cp "$DROP/verion-postgres-init-${TIMESTAMP}.sql" deployments/docker/postgres/init.sql
echo "      ✓ deployments/docker/postgres/init.sql"

echo "[3/6] Setting up environment file..."
cp "$DROP/verion-env-example-${TIMESTAMP}.txt" .env.example
# Only create .env if it doesn't exist (never overwrite existing secrets)
if [ ! -f .env ]; then
  cp .env.example .env
  echo "      ✓ .env created from example"
else
  echo "      ℹ .env already exists — skipping (not overwritten)"
fi

echo "[4/6] Updating .gitignore to protect secrets..."
if ! grep -q "^\.env$" .gitignore; then
  echo "" >> .gitignore
  echo "# Environment secrets" >> .gitignore
  echo ".env" >> .gitignore
  echo ".env.local" >> .gitignore
  echo "      ✓ .gitignore updated"
else
  echo "      ℹ .gitignore already excludes .env"
fi

echo "[5/6] Starting Docker services..."
docker compose up -d

echo ""
echo "      Waiting for services to be healthy..."
sleep 5

# Check postgres
echo ""
echo "      Checking PostgreSQL..."
docker compose exec postgres pg_isready -U verion -d verion && echo "      ✓ PostgreSQL is ready" || echo "      ✗ PostgreSQL not ready yet — wait a moment and check: docker compose ps"

# Check redis
echo ""
echo "      Checking Redis..."
docker compose exec redis redis-cli -a verion_redis_secret ping && echo "      ✓ Redis is ready" || echo "      ✗ Redis not ready yet"

echo "[6/6] Committing to Git..."
git add docker-compose.yml deployments/ .env.example .gitignore
git commit -m "feat(infra): add Docker Compose infrastructure for local development

- PostgreSQL 16 with uuid-ossp, pgcrypto, citext extensions
- Redis 7 with persistence and memory limits
- Health checks on both services
- .env.example with all required configuration keys
- deployments/docker/postgres/init.sql for DB initialization"

git push origin main

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║  ✓  Docker infrastructure running                        ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "Services:"
echo "  PostgreSQL → localhost:5432  (db: verion, user: verion)"
echo "  Redis      → localhost:6379  (password: verion_redis_secret)"
echo ""
echo "Useful commands:"
echo "  docker compose ps          # check service status"
echo "  docker compose logs -f     # follow logs"
echo "  docker compose down        # stop services"
echo "  docker compose down -v     # stop + delete volumes (fresh start)"
echo ""
echo "Next: Phase 1 · Step 2 — Database Migration Schema"
echo ""
