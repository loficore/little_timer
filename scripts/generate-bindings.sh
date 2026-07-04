#!/bin/bash
# Generate and fix Wails bindings for Android compatibility
# ponytail: run this after updating wails_bindings.go to regenerate bindings

set -e

BINDINGS_VERSION="v3.0.0-alpha2.114"
WORKDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$WORKDIR")"

echo "=== Generating Wails bindings ==="
cd "$PROJECT_ROOT/neo-src"
go run github.com/wailsapp/wails/v3/cmd/wails3@${BINDINGS_VERSION} generate bindings \
    -ts -clean \
    -f '-tags=android' \
    -d /tmp/wails-bindings \
    ./cmd/server

echo "=== Copying and fixing bindings ==="
BINDINGS_SRC="/tmp/wails-bindings/little-timer/internal/app"
mkdir -p "$PROJECT_ROOT/assets/src/bindings/little-timer/internal/app"
mkdir -p "$PROJECT_ROOT/neo-src/cmd/server/assets/bindings/little-timer/internal/app"

# Copy fresh bindings
cp "$BINDINGS_SRC"/*.ts "$PROJECT_ROOT/assets/src/bindings/little-timer/internal/app/" 2>/dev/null || true
cp "$BINDINGS_SRC"/*.ts "$PROJECT_ROOT/neo-src/cmd/server/assets/bindings/little-timer/internal/app/" 2>/dev/null || true

# Fix @wailsio/runtime import paths
find "$PROJECT_ROOT/assets/src/bindings" -name "*.ts" -exec sed -i 's|@wailsio/runtime|/wails/runtime.js|g' {} \;
find "$PROJECT_ROOT/neo-src/cmd/server/assets/bindings" -name "*.ts" -exec sed -i 's|@wailsio/runtime|/wails/runtime.js|g' {} \;

echo "=== Done ==="
echo "Bindings generated and fixed at:"
echo "  - assets/src/bindings/little-timer/internal/app/"
echo "  - neo-src/cmd/server/assets/bindings/little-timer/internal/app/"
