#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Verion — Project Initialization Script
# Run from: /home/harsh/workspace/projects/Verion
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

PROJECT_ROOT="$(pwd)"
GIT_REMOTE="git@github.com:Harshmaury/Verion.git"
GO_MODULE="github.com/Harshmaury/verion"

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║          VERION — Project Initialization                 ║"
echo "║          Universal Identity Operating System             ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "→ Project root : $PROJECT_ROOT"
echo "→ Git remote   : $GIT_REMOTE"
echo "→ Go module    : $GO_MODULE"
echo ""

# ── 1. Directory structure ────────────────────────────────────────────────────
echo "[1/7] Creating directory structure..."

mkdir -p \
  cmd/verion \
  internal/identity \
  internal/auth \
  internal/trust \
  internal/authz \
  internal/session \
  internal/crypto \
  internal/device \
  internal/recovery \
  internal/audit \
  pkg \
  api/proto \
  api/rest \
  docs/adr \
  deployments/docker \
  deployments/k8s \
  scripts

echo "      ✓ Directories created"

# ── 2. Go module ──────────────────────────────────────────────────────────────
echo "[2/7] Initializing Go module..."

go mod init "$GO_MODULE"
echo "      ✓ go.mod created"

# ── 3. .gitignore ─────────────────────────────────────────────────────────────
echo "[3/7] Creating .gitignore..."

cat > .gitignore << 'EOF'
# Binaries
bin/
dist/
*.exe
*.out

# Go
*.test
*.prof
vendor/

# Environment
.env
.env.*
*.local

# IDE
.vscode/settings.json
.idea/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Build artifacts
build/
tmp/

# Secrets — never commit these
*.pem
*.key
*.p12
*.pfx
secrets/
EOF

echo "      ✓ .gitignore created"

# ── 4. Makefile ───────────────────────────────────────────────────────────────
echo "[4/7] Creating Makefile..."

cat > Makefile << 'EOF'
.PHONY: help build test lint clean run dev

BINARY     = verion
CMD_PATH   = ./cmd/verion
BUILD_DIR  = bin

help:           ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build:          ## Build the binary
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD_PATH)
	@echo "✓ Built $(BUILD_DIR)/$(BINARY)"

test:           ## Run all tests
	go test ./... -v -race

lint:           ## Run linter (requires golangci-lint)
	golangci-lint run ./...

clean:          ## Remove build artifacts
	rm -rf $(BUILD_DIR)

run:            ## Build and run
	go run $(CMD_PATH)/main.go

tidy:           ## Tidy go.mod
	go mod tidy

fmt:            ## Format code
	gofmt -w .
EOF

echo "      ✓ Makefile created"

# ── 5. Entry point ────────────────────────────────────────────────────────────
echo "[5/7] Creating entry point..."

cat > cmd/verion/main.go << 'EOF'
package main

import "fmt"

func main() {
	fmt.Println("Verion — Universal Identity OS")
	fmt.Println("Version: 0.1.0-alpha")
}
EOF

echo "      ✓ cmd/verion/main.go created"

# ── 6. README ─────────────────────────────────────────────────────────────────
echo "[6/7] Creating README..."

cat > README.md << 'EOF'
# Verion

> **Universal Identity Operating System**

Verion is a next-generation Identity Operating System that provides universal,
cryptographically verifiable, privacy-preserving, and continuously adaptive
identity infrastructure for the digital ecosystem.

---

## Core Principle

> Trust is not granted; it is continuously established through independently verifiable evidence.

---

## Architecture

```
┌────────────────────────────────────────────────────┐
│                  Identity Registry                 │
├────────────────────────────────────────────────────┤
│            Identity Graph & Trust Model            │
├──────────────┬──────────────┬──────────────────────┤
│Authentication│Authorization │ Trust Evaluation     │
├──────────────┼──────────────┼──────────────────────┤
│Session Engine│Device Trust  │ Policy Decision      │
├──────────────┼──────────────┼──────────────────────┤
│Recovery      │Audit         │ Cryptographic Svc    │
├──────────────┴──────────────┴──────────────────────┤
│  APIs · SDKs · Gateways · Federation               │
└────────────────────────────────────────────────────┘
```

---

## Development Phases

| Phase | Scope | Status |
|-------|-------|--------|
| Phase 0 | Foundation — structure, tooling, ADRs | 🔄 In Progress |
| Phase 1 | Identity Core — registry, keys, storage | ⏳ Planned |
| Phase 2 | Authentication — passwordless, FIDO2 | ⏳ Planned |
| Phase 3 | Trust Engine — continuous scoring | ⏳ Planned |
| Phase 4 | Authorization — policy, dynamic access | ⏳ Planned |
| Phase 5 | Federation — OIDC, SAML, DID | ⏳ Planned |
| Phase 6 | Scale — performance, observability | ⏳ Planned |

---

## Getting Started

```bash
# Build
make build

# Run
make run

# Test
make test
```

---

## Technology Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.24 |
| API | gRPC + REST (OpenAPI) |
| Database | PostgreSQL |
| Cache | Redis |
| Container | Docker + Kubernetes |
| Crypto | TBD — see ADR-003 |

---

## Architecture Decisions

See [`docs/adr/`](docs/adr/) for all Architecture Decision Records.

---

## License

Proprietary — All rights reserved.
EOF

echo "      ✓ README.md created"

# ── 7. Git init ───────────────────────────────────────────────────────────────
echo "[7/7] Initializing Git repository..."

git init
git remote add origin "$GIT_REMOTE"
git add .
git commit -m "chore(init): initialize Verion project structure

- Directory layout for all planned components
- Go module initialized (github.com/Harshmaury/verion)
- Makefile with build/test/lint/run targets
- .gitignore for Go/IDE/OS artifacts
- README with architecture overview and phase plan
- Entry point cmd/verion/main.go"

echo "      ✓ Git repository initialized with first commit"

# ── Done ──────────────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║  ✓  Verion project initialized successfully              ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "Next steps:"
echo "  1. Push to GitHub  →  git push -u origin main"
echo "  2. Verify build    →  make build"
echo "  3. Read workflow   →  verion-workflow-20260627_120156.docx"
echo ""
echo "Current phase: Phase 0 — Foundation ✓"
echo "Next phase   : Phase 1 — Identity Core"
echo ""
