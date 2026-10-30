# HelmDownloader v0.5.1

Maintenance release: Go toolchain and dependency updates. No functional change, same bundle layout and CLI as v0.5.0.

## Changes

- **Go 1.27.3** is now the minimum toolchain (`go 1.27.0`, `toolchain go1.27.3`); release binaries are built with it
- Direct dependencies:
  - `charm.land/bubbletea/v2` 2.0.10 → 2.1.1
  - `github.com/charmbracelet/x/ansi` 0.11.8 → 0.11.9
  - `github.com/klauspost/compress` 1.20.1 → 1.20.3
  - `golang.org/x/sync` 0.23.0 → 0.24.0
  - `golang.org/x/sys` 0.48.0 → 0.49.0
- Indirect dependencies: `docker/cli` 29.9.2, `docker-credential-helpers` 0.9.12, `logrus` 1.10.2, `go-runewidth` 0.0.31, `xo/terminfo` 1.3.0, `ultraviolet`
- CI: golangci-lint v2.15.0 (required for Go 1.27 modules)

## Upgrade notes

- **Building from source** now needs Go **1.27.3+** (or `GOTOOLCHAIN=auto`). Release binaries are unaffected for end users.

## Install

```bash
# Go
go install github.com/julienhmmt/helmdownloader@v0.5.1

# Or download the release asset for your OS/arch from GitHub Releases,
# verify checksums.txt, then run:
./helmdownloader version
```

## Full changelog

https://github.com/julienhmmt/helmdownloader/compare/v0.5.0...v0.5.1
