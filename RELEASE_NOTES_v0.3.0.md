# HelmDownloader v0.3.0

Integrity-hardened airgap bundles, safer resume, private-registry auth, offline `verify`/`diff`, SPDX SBOM, and a polished multi-theme TUI.

## Highlights

### Bundle integrity & offline tooling

- `load.sh` is included in `sha256sums.txt` so the only executable path on the airgapped host is checksummed
- `helmdownloader verify <bundle>` re-hashes every member offline and rejects missing image digests
- `helmdownloader diff <a> <b>` shows `+` added / `-` removed / `~` changed images by source ref and pinned digest
- Streaming verify/diff (no full archive in RAM) with metadata size caps (DoS guard)

### Downloads, resume, registries

- Per-image progress bars; Esc cancels busy work while keeping partial successes
- Single image failure never aborts the batch (fixed-slot, input-order results)
- `-resume` with content-hash (`.sha256`) + registry digest (`.digest`) sidecars — truncated/corrupt/tampered tarballs re-pull
- `-registry-auth` pulls via the default Docker keychain
- Configured `-proxy` applies to ArtifactHub, helm, and registry pulls (shared transport)

### Security & provenance

- SPDX 2.3 `sbom.spdx.json` in every bundle
- `manifest.json` provenance includes tool identity (`helmdownloader version` / ldflags)
- `-export-images` / `-import-images` JSON workflow for security review before download
- Manual `a` add and import paths validate refs with `name.ParseReference`
- Collision-safe image tarball filenames; ArtifactHub body size bounds

### TUI

- Themes: `auto`, `light`, `dark`, `high-contrast`, `ocean`, `matrix` (`-theme` or `Ctrl+T`)
- Sort/filter results by stars, name, updated; filter by author/company
- Official / deprecated badges; prerelease confirmation
- Windowed review list for large charts; empty-state + action feedback
- Richer done/error summaries and pre-download warnings

### Build & ops

- `helmdownloader version` (release builds inject the tag)
- Makefile targets: `build`, `build-release`, `test-race`, `coverage`, `security`
- Annotated [`config.example.yaml`](./config.example.yaml)
- Build toolchain pinned to **Go 1.26.5** (`toolchain` in `go.mod`) for stdlib fixes (GO-2026-5856, CVE-2026-39822)

## Upgrade notes

- **Resume work dirs from v0.2.x** without `.sha256` content-hash sidecars will re-pull once (safe). After a successful pull under 0.3.0, resume works as before.
- **Verify is stricter**: bundles whose `manifest.json` images lack digests fail. Prefer re-bundling with 0.3.0 for airgapped handoff.
- **Config path on macOS**: prefers `~/.config/helmdownloader/config.yaml` (XDG-style); falls back to the OS app-support path when missing.
- **Building from source** needs Go **1.26.5+** (or `GOTOOLCHAIN=auto` so the `toolchain` line in `go.mod` can fetch it). Release binaries are unaffected for end users.

## Install

```bash
# Go
go install github.com/julienhmmt/helmdownloader@v0.3.0

# Or download the release asset for your OS/arch from GitHub Releases,
# verify checksums.txt, extract, and run:
./helmdownloader version
```

## Checksums

Published as `checksums.txt` on the GitHub Release (goreleaser).

## Full changelog

See commits since `v0.2.1` on `main`, or the auto-generated goreleaser notes on the release page.
