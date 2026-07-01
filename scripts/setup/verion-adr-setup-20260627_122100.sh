#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Verion — ADR & Documentation Setup Script
# Run from: /home/harsh/workspace/projects/Verion
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

DROP="/mnt/c/Users/harsh/Downloads/Verion-drop"
TIMESTAMP="20260627_122100"

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║     VERION — Installing ADRs & Decision Register         ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

# ── Copy files into project structure ────────────────────────────────────────

echo "[1/5] Copying ADR-000 template..."
cp "$DROP/verion-adr-000-20260627_122100.md" docs/adr/ADR-000-template.md
echo "      ✓ docs/adr/ADR-000-template.md"

echo "[2/5] Copying ADR-001 Technology Stack..."
cp "$DROP/verion-adr-001-20260627_122100.md" docs/adr/ADR-001-technology-stack.md
echo "      ✓ docs/adr/ADR-001-technology-stack.md"

echo "[3/5] Copying ADR-002 Identity Data Model..."
cp "$DROP/verion-adr-002-20260627_122100.md" docs/adr/ADR-002-identity-data-model.md
echo "      ✓ docs/adr/ADR-002-identity-data-model.md"

echo "[4/5] Copying Master Decision Register..."
cp "$DROP/verion-decisions-20260627_122100.md" docs/DECISIONS.md
echo "      ✓ docs/DECISIONS.md"

echo "[5/5] Committing to Git..."
git add docs/
git commit -m "docs(adr): add ADR-000 template, ADR-001 tech stack, ADR-002 identity model

- ADR-000: Standard template for all future architectural decisions
- ADR-001: Technology stack — Go, gRPC+REST, PostgreSQL, Redis, Docker/k8s
- ADR-002: Identity data model — universal entity, crypto keys, credentials, audit
- DECISIONS.md: Master register of all decisions with status tracking"

git push origin main

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║  ✓  ADRs installed and pushed to GitHub                  ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "Files created:"
echo "  docs/adr/ADR-000-template.md"
echo "  docs/adr/ADR-001-technology-stack.md"
echo "  docs/adr/ADR-002-identity-data-model.md"
echo "  docs/DECISIONS.md"
echo ""
echo "Current phase : Phase 0 — Foundation ✓"
echo "Next          : Phase 1 — Identity Core (Docker + PostgreSQL schema)"
echo ""
