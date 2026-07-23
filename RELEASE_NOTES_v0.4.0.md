# HelmDownloader v0.4.0

Headless batch automation, multi-chart selection in one session, CRD image discovery, and a configurable temp directory — built on the integrity-hardened bundle format from v0.3.0.

## Highlights

### Automation & batch

- `helmdownloader batch <list.yaml>` downloads a YAML list of charts headlessly — no TUI — for CI and scripted airgap pipelines
- `helmdownloader batch -config <path> <list.yaml>` runs the same loop under an explicit config
- One image (or one chart) failure never aborts the batch; results stay in input order (fixed-slot, `[]ImageFailure`)

### Multi-chart TUI

- Select and bundle several charts in a single session; after each bundle the TUI returns to search while keeping the list of bundles already created
- Each chart still flows through the same prepare → download → bundle pipeline

### Image discovery

- CRD-only charts (which ship no container images) are handled explicitly — they bundle the chart and values without failing on an empty image set
- Image refs continue to be validated with `name.ParseReference` on every add/import path

### Configuration

- `-temp-dir` sets the parent directory for temporary work directories, with a writable fallback when the configured path is not usable
- Config stays single-source-of-truth: `config.Config` field + `config.Default` + CLI flag + README

## Upgrade notes

- **No bundle-format changes** since v0.3.0 — `verify` and `diff` on v0.3.0 bundles keep working; `load.sh` still fails closed without a `sha256sum`/`shasum` integrity check.
- **Batch list format**: pass a YAML list of charts; see `README.md` → Subcommands → `batch`.
- **Building from source** still needs Go **1.26.5+** (or `GOTOOLCHAIN=auto` so the `toolchain` line in `go.mod` can fetch it). Release binaries are unaffected for end users.

## Install

```bash
# Go
go install github.com/julienhmmt/helmdownloader@v0.4.0

# Or download the release asset for your OS/arch from GitHub Releases,
# verify SHA256SUMS.txt, then run:
./helmdownloader version
```

## Checksums

Published as `SHA256SUMS.txt` on the GitHub Release. Binaries are pure-Go and statically linked (`CGO_ENABLED=0`): Linux and macOS, amd64 and arm64.

## Full changelog

https://github.com/julienhmmt/helmdownloader/compare/v0.3.0...v0.4.0
