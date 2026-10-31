// Package static embeds booba's browser-side assets at compile time:
// the terminal wrapper (booba/), the ntwebterm terminal emulator
// (ntwebterm/), and the served host page (index.html).
//
// Embedding them here lets both the serve package and the booba-assets
// scaffolding tool ship the assets inside their binaries, with no
// dependency on the module cache, network, or working directory.
package static

import "embed"

// FS holds the embedded browser assets, rooted at this directory.
// Paths within it are "booba/...", "ntwebterm/...", and "index.html".
//
// Match runtime files explicitly, including underscore-prefixed Vite chunks.
// Directory patterns would also embed leftover declarations, maps, and CJS
// bundles from older builds.
//
//go:embed ghostty-web/*.js ghostty-web/*.wasm booba/*.js ntwebterm/*.js ntwebterm/*.wasm index.html
var FS embed.FS
