#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Verion — Project Intelligence & Context Pack Setup
# Run from: /home/harsh/workspace/projects/Verion
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

DROP="/mnt/c/Users/harsh/Downloads/Verion-drop"
TIMESTAMP="20260627_125421"

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║     VERION — Installing Project Intelligence System      ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

echo "[1/5] Installing PROJECT.md..."
cp "$DROP/verion-PROJECT-${TIMESTAMP}.md" PROJECT.md
echo "      ✓ PROJECT.md"

echo "[2/5] Installing SESSION.md..."
cp "$DROP/verion-SESSION-${TIMESTAMP}.md" SESSION.md
echo "      ✓ SESSION.md"

echo "[3/5] Installing context-pack script..."
cp "$DROP/verion-context-pack-${TIMESTAMP}.sh" scripts/context-pack.sh
chmod +x scripts/context-pack.sh
echo "      ✓ scripts/context-pack.sh (executable)"

echo "[4/5] Running context-pack to generate first context zip..."
./scripts/context-pack.sh

echo "[5/5] Committing to Git..."
git add PROJECT.md SESSION.md scripts/context-pack.sh
git commit -m "chore(meta): add project intelligence system

- PROJECT.md: master AI context file with full project state
- SESSION.md: session log for AI context continuity
- scripts/context-pack.sh: zip tool to package context for new AI sessions"

git push origin main

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║  ✓  Project intelligence system installed                ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "From now on, before starting a new AI session:"
echo "  ./scripts/context-pack.sh"
echo ""
echo "Then upload the zip to the new AI and say:"
echo "  'Read PROJECT.md first, then SESSION.md. Continue.'"
echo ""
echo "Next: Phase 1 · Step 2 — Database Migration Schema"
echo ""
