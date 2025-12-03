# booba - Web-based BubbleTea TUIs using libghostty

<p>
    <a href="https://github.com/NimbleMarkets/booba/tags"><img src="https://img.shields.io/github/tag/NimbleMarkets/booba.svg" alt="Latest Release"></a>
    <a href="https://pkg.go.dev/github.com/NimbleMarkets/booba?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="GoDoc"></a>
    <a href="https://github.com/NimbleMarkets/booba/blob/main/CODE_OF_CONDUCT.md"><img src="https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa.svg"  alt="Code Of Conduct"></a>
</p>

`booba` is a Golang module that facilitates embedding BubbleTea Terminal User Interfaces (TUIs) into a Web Browser.  Generally, these are access via a local terminal or via SSH.   This module exposes an HTTP-based terminal connection to a BubbleTea program.

There are two facets we address with this package:

 * Running a full BubbleTea program in a Web Browser

 * Running a Terminal in a browser that connects over WebSockets to a BubbleTea backend

The primary enabling technologies of this are:

 * [`libghostty`](https://github.com/ghostty-org/ghostty)
 * [`ghostty-web`](https://github.com/coder/ghostty-web)
 * [`BubbleTea`](https://github.com/charmbracelet/bubbletea)
 * [`WebAssembly`](https://webassembly.org)

The name `booba` is a portmanteau of the words Boba and Boo, the key ingredient of Bubble Tea leading to a Ghost's exclamation of joy.

## Embedding a BubbleTea Application in a Web Browser

We can take entire BubbleTea applications and embed them into a Web Browser.  The primary limitation is that all of its dependencies can also compiled to WebAssembly. 

TODO: instructions for doing this.   
TODO: link to live example

## Web Frontend for BubbleTea-based service

Otherwise, one might have a BubbleTea program running on a remote machine.  While one might use `ssh` to access it,  `booba` enables an HTTP-based interface to it.   Effectively, we serve up a Ghostty terminal from an HTTP endpoint and extend the terminal via WebSockets.

TODO: instructions for doing this.   
TODO: link to live example

## Open Collaboration

We welcome contributions and feedback.  Please adhere to our [Code of Conduct](./CODE_OF_CONDUCT.md) when engaging our community.

 * [GitHub Issues](https://github.com/NimbleMarkets/booba/issues)
 * [GitHub Pull Requests](https://github.com/NimbleMarkets/booba/pulls)

## Acknowledgements

Thanks to the [Ghostty developers](https://github.com/ghostty-org/ghostty), the [ghostty-web](https://github.com/coder/ghostty-web) developers, and to [Charm.sh](https://charm.sh) for making the command line glamorous with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Thanks to [@BigJK](https://github.com/BigJk/bubbletea-in-wasm) for the initial inspiration when I was exploring this before `libghostty`.

## License

Released under the [MIT License](https://en.wikipedia.org/wiki/MIT_License), see [LICENSE.txt](./LICENSE.txt).

Copyright (c) 2025 [Neomantra Corp](https://www.neomantra.com).   

----
Made with :heart: and :fire: by the team behind [Nimble.Markets](https://nimble.markets).
