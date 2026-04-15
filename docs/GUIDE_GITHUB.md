# Hosting a BubbleTea Program on GitHub Pages

See **[NimbleMarkets/go-booba-example](https://github.com/NimbleMarkets/go-booba-example)** —
a complete, forkable example that compiles a BubbleTea program to WebAssembly
and deploys it to GitHub Pages. No backend server needed.

The example repo includes:

- A working BubbleTea program with both native and WASM entry points
- A GitHub Actions workflow that builds the WASM binary and deploys to Pages
- The WASM overlay build script (workaround for BubbleTea v2's missing `js/wasm` tags)
- An `index.html` that wires up ghostty-web and booba's WASM adapter

Fork it, enable Pages (Settings → Pages → Source: GitHub Actions), and push.
