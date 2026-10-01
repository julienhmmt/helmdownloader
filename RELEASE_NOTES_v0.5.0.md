# HelmDownloader v0.5.0

Self-contained airgap handoffs, a faster and safer image review, and a TUI that tells you more at every step — on the same bundle layout and integrity checks as v0.4.0.

## Highlights

### Airgap handoffs

- Every bundle now ships a checksummed `HOWTO.txt`: chart/version, platform, registry prefix, codec-matched verify/extract commands, and Docker/Podman loading guidance
- `manifest.json` records `status` (`complete` / `partial`), `platform`, `registryPrefix`, and `missingImages` so recipients can tell integrity from completeness
- After you quit the TUI, a plain-text summary of every bundle created in the session stays in the terminal scrollback (path, size, COMPLETE/PARTIAL, image counts, `verify` and extract commands)
- `batch` reports `PARTIAL` per chart and exits non-zero when any requested image is missing, while still writing partial bundles and processing later charts

### Image review

- `A` select all, `N` deselect all, `i` invert — no more toggling 40 sidecars one by one
- `e` saves the current reviewed list (toggles, additions, deletions) as importable JSON; an explicitly imported empty list stays empty
- Unpinned references (`latest` or no tag, no digest) are flagged with `⚠ latest` so non-reproducible images stand out before download

### TUI feedback

- Results and versions show a relative age (`updated:3mo ago`), so sorting by "updated" is no longer blind
- Cancelling a running download takes two `Esc` presses; the first one asks for confirmation
- Informational status lines (theme changes, hints) are no longer rendered in the error tone — red now means a warning
- Empty search submit shows a hint instead of doing nothing silently
- Download screen names the chart and version; bundling explains why `Esc` is disabled
- `Esc` on the error screen returns to the step that failed (search or prepare) instead of quitting

## Upgrade notes

- **Bundle additions, no breaking change**: new `HOWTO.txt` entry and new optional `manifest.json` fields. `verify` and `diff` on v0.4.0 bundles keep working; `load.sh` still fails closed without a `sha256sum`/`shasum` integrity check.
- **`batch` exit code is stricter**: a chart with any missing image now counts as incomplete and fails the run (bundles are still written). Adjust CI expectations if you relied on partial deliveries exiting 0.
- **Review keys**: `A`, `N`, `i`, `e` are new on the Review screen; `Esc` during download now requires a second press.
- **Building from source** still needs Go **1.26+** (or `GOTOOLCHAIN=auto`). Release binaries are unaffected for end users.

## Install

```bash
# Go
go install github.com/julienhmmt/helmdownloader@v0.5.0

# Or download the release asset for your OS/arch from GitHub Releases,
# verify checksums.txt, then run:
./helmdownloader version
```

## Checksums

Published as `checksums.txt` on the GitHub Release. Binaries are pure-Go and statically linked (`CGO_ENABLED=0`): Linux, macOS and Windows, amd64 and arm64.

## Full changelog

https://github.com/julienhmmt/helmdownloader/compare/v0.4.0...v0.5.0
