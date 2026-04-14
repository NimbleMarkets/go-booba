#!/bin/sh
# Build a Go WASM binary with BubbleTea js/wasm stubs injected.
#
# BubbleTea v2 lacks js/wasm build tags for platform-specific functions.
# This script copies the module to a temp directory, injects the stub
# files, and builds with a temporary replace directive.
#
# Usage: internal/wasm_overlay/wasm_build.sh [go build flags...] <packages>
#   e.g. internal/wasm_overlay/wasm_build.sh -o bin/example.wasm ./cmd/example
#
# See: https://github.com/charmbracelet/bubbletea/issues/1410

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Resolve bubbletea module directory from the module cache
BT_MOD="charm.land/bubbletea/v2"
BT_CACHE_DIR="$(go list -m -json "$BT_MOD" | grep '"Dir"' | head -1 | sed 's/.*"Dir": "//;s/".*//')"

if [ -z "$BT_CACHE_DIR" ] || [ ! -d "$BT_CACHE_DIR" ]; then
    echo "Error: could not find $BT_MOD in module cache." >&2
    echo "Run 'go mod download' first." >&2
    exit 1
fi

# Create temp directory
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Copy bubbletea and inject stubs
cp -r "$BT_CACHE_DIR" "$TMPDIR/bubbletea"
chmod -R u+w "$TMPDIR/bubbletea"
cp "$SCRIPT_DIR/signals_js.go" "$TMPDIR/bubbletea/signals_js.go"
cp "$SCRIPT_DIR/tty_js.go" "$TMPDIR/bubbletea/tty_js.go"

# Create a temporary go.mod with a replace directive
TMPMOD="$TMPDIR/go.mod"
cp "$PROJECT_DIR/go.mod" "$TMPMOD"
cp "$PROJECT_DIR/go.sum" "$TMPDIR/go.sum"
go mod edit -replace "$BT_MOD=$TMPDIR/bubbletea" "$TMPMOD"

# Build
GOOS=js GOARCH=wasm go build -modfile="$TMPMOD" "$@"
