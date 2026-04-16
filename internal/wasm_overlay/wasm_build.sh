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

# Locate WASM overlay stubs. First check next to this script (for booba
# repo and consumers who copied the files), then fall back to the go-booba
# module in the Go module cache (for consumers who only have this script).
STUB_DIR="$SCRIPT_DIR"
if [ ! -f "$STUB_DIR/signals_js.go" ]; then
    BOOBA_MOD="github.com/NimbleMarkets/go-booba"
    BOOBA_DIR="$(go list -m -json "$BOOBA_MOD" 2>/dev/null | grep '"Dir"' | head -1 | sed 's/.*"Dir": "//;s/".*//')"
    if [ -n "$BOOBA_DIR" ] && [ -f "$BOOBA_DIR/internal/wasm_overlay/signals_js.go" ]; then
        STUB_DIR="$BOOBA_DIR/internal/wasm_overlay"
    else
        echo "Error: WASM overlay stubs not found." >&2
        echo "Either place signals_js.go and tty_js.go next to this script," >&2
        echo "or add $BOOBA_MOD to your go.mod." >&2
        exit 1
    fi
fi

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
cp "$STUB_DIR/signals_js.go" "$TMPDIR/bubbletea/signals_js.go"
cp "$STUB_DIR/tty_js.go" "$TMPDIR/bubbletea/tty_js.go"

# Create a temporary go.mod with a replace directive
TMPMOD="$TMPDIR/go.mod"
cp "$PROJECT_DIR/go.mod" "$TMPMOD"
cp "$PROJECT_DIR/go.sum" "$TMPDIR/go.sum"
go mod edit -replace "$BT_MOD=$TMPDIR/bubbletea" "$TMPMOD"

# Build
GOOS=js GOARCH=wasm go build -modfile="$TMPMOD" "$@"
