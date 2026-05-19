// Package static embeds booba's browser-side assets at compile time:
// the terminal wrapper (booba/), the ghostty-web terminal emulator
// (ghostty-web/), and the served host page (index.html).
//
// Embedding them here lets both the serve package and the booba-assets
// scaffolding tool ship the assets inside their binaries, with no
// dependency on the module cache, network, or working directory.
package static

import "embed"

// FS holds the embedded browser assets, rooted at this directory.
// Paths within it are "booba/...", "ghostty-web/...", and "index.html".
//
// The all: prefix is required so Vite's "__vite-*.js" chunks — whose
// names begin with an underscore — are embedded; go:embed excludes
// "_"- and "."-prefixed files by default.
//
//go:embed all:booba all:ghostty-web index.html
var FS embed.FS
