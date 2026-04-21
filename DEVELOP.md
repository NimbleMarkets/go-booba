# Development Guide

## Command Documentation

The `booba` CLI is built on [spf13/cobra](https://github.com/spf13/cobra) and ships generated documentation alongside the binary:

- **Man pages** — `docs/man/booba.1` and one file per subcommand. Install with `cp docs/man/*.1 /usr/local/share/man/man1/`.
- **Markdown** — `docs/markdown/booba.md` and subcommand files, suitable for wikis or docs sites.
- **Shell completions** — `completions/booba.{bash,zsh,fish}`. Source the appropriate file from your shell rc, or install to your system's completion directory.

Regenerate everything after changing commands or flags:

```sh
task docs:build
```

The hidden `booba docs` subcommand drives this: `booba docs man -o <dir>`, `booba docs markdown -o <dir>`, and `booba completion <shell>`.

