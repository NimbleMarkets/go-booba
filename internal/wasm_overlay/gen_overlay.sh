#!/bin/sh
# Generate overlay.json with resolved paths for the WASM build.
# This injects js/wasm stub files into the BubbleTea module so it
# compiles to WebAssembly without a fork.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BT_DIR="$(go list -m -json charm.land/bubbletea/v2 | grep '"Dir"' | head -1 | sed 's/.*"Dir": "//;s/".*//')"

cat <<EOF
{
  "Replace": {
    "${BT_DIR}/signals_js.go": "${SCRIPT_DIR}/signals_js.go",
    "${BT_DIR}/tty_js.go": "${SCRIPT_DIR}/tty_js.go"
  }
}
EOF
