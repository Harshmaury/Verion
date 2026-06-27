#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Verion — Context Pack Tool
# Packages all AI context files into a zip and drops to Windows download folder
#
# Usage: ./scripts/context-pack.sh
# Run from: /home/harsh/workspace/projects/Verion
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DROP="/mnt/c/Users/harsh/Downloads/Verion-drop"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
PACK_NAME="verion-context-${TIMESTAMP}.zip"
TMP_DIR="/tmp/verion-context-${TIMESTAMP}"

cd "$PROJECT_ROOT"

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║          VERION — Context Pack Tool                      ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "→ Timestamp : $TIMESTAMP"
echo "→ Output    : $DROP/$PACK_NAME"
echo ""

# ── Verify drop folder exists ─────────────────────────────────────────────────
if [ ! -d "$DROP" ]; then
  echo "✗ Drop folder not found: $DROP"
  echo "  Create it in Windows Explorer first."
  exit 1
fi

# ── Create temp staging directory ─────────────────────────────────────────────
mkdir -p "$TMP_DIR"

# ── Generate live project tree ────────────────────────────────────────────────
echo "[1/6] Generating project tree..."
if command -v tree &>/dev/null; then
  tree -a -I '.git|node_modules|vendor|bin|*.zip' --dirsfirst > "$TMP_DIR/PROJECT-TREE.txt"
else
  find . -not -path './.git/*' -not -path './bin/*' -not -name '*.zip' \
    | sort > "$TMP_DIR/PROJECT-TREE.txt"
fi
echo "      ✓ PROJECT-TREE.txt"

# ── Generate git log ─────────────────────────────────────────────────────────
echo "[2/6] Generating git log..."
git log --oneline --graph --decorate -20 > "$TMP_DIR/GIT-LOG.txt" 2>/dev/null || \
  echo "No git history yet" > "$TMP_DIR/GIT-LOG.txt"
echo "      ✓ GIT-LOG.txt"

# ── Copy core context files ───────────────────────────────────────────────────
echo "[3/6] Copying core context files..."

# Must-have files
FILES_TO_COPY=(
  "PROJECT.md"
  "SESSION.md"
  "README.md"
  "docs/DECISIONS.md"
)

for f in "${FILES_TO_COPY[@]}"; do
  if [ -f "$f" ]; then
    # Preserve directory structure inside zip
    mkdir -p "$TMP_DIR/$(dirname "$f")"
    cp "$f" "$TMP_DIR/$f"
    echo "      ✓ $f"
  else
    echo "      ⚠ Missing: $f (skipped)"
  fi
done

# ── Copy all ADRs ─────────────────────────────────────────────────────────────
echo "[4/6] Copying ADRs..."
if [ -d "docs/adr" ]; then
  mkdir -p "$TMP_DIR/docs/adr"
  for adr in docs/adr/*.md; do
    cp "$adr" "$TMP_DIR/$adr"
    echo "      ✓ $adr"
  done
else
  echo "      ⚠ No ADRs found"
fi

# ── Copy go.mod for module context ───────────────────────────────────────────
echo "[5/6] Copying module & config files..."
[ -f "go.mod" ]          && cp "go.mod"          "$TMP_DIR/go.mod"          && echo "      ✓ go.mod"
[ -f ".env.example" ]    && cp ".env.example"    "$TMP_DIR/.env.example"    && echo "      ✓ .env.example"
[ -f "docker-compose.yml" ] && cp "docker-compose.yml" "$TMP_DIR/docker-compose.yml" && echo "      ✓ docker-compose.yml"

# ── Create the zip ────────────────────────────────────────────────────────────
echo "[6/6] Creating zip..."
cd /tmp
zip -r "$DROP/$PACK_NAME" "verion-context-${TIMESTAMP}/" -x "*.DS_Store" > /dev/null
cd "$PROJECT_ROOT"

# ── Cleanup temp ─────────────────────────────────────────────────────────────
rm -rf "$TMP_DIR"

# ── Summary ──────────────────────────────────────────────────────────────────
ZIP_SIZE=$(du -sh "$DROP/$PACK_NAME" | cut -f1)

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║  ✓  Context pack ready                                   ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "File : $PACK_NAME"
echo "Size : $ZIP_SIZE"
echo "Drop : $DROP"
echo ""
echo "How to use in a new AI session:"
echo "  1. Upload $PACK_NAME to the AI conversation"
echo "  2. Say: 'Unzip and read all files. This is the Verion"
echo "     project. Read PROJECT.md first, then SESSION.md."
echo "     Continue from where we left off.'"
echo ""
