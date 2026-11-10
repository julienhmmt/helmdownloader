# HelmDownloader v0.6.0

Reliability release: safer cancellation and resume, correct proxy handling, and fixes to image discovery and registry pulls. Same bundle layout and CLI flags as v0.5.1.

## Fixes

- **Proxy environment** — `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` are now handled by the standard Go and helm logic. Previously `NO_PROXY` was ignored and HTTPS traffic could go to the `HTTP_PROXY` value. An explicit `-proxy` or config value still forces a fixed proxy (#72)
- **Duplicate image references** — a repeated ref is pulled only once (two workers writing the same tarball could corrupt it). Adding or importing a duplicate ref in Review is rejected with a clear message (#71)
- **Image discovery** — a rendered YAML document with a duplicate key no longer stops discovery; later documents are still scanned (#70)
- **SIGINT/SIGTERM in batch mode** — stops the batch, fails the in-flight chart without bundling a partial result, and removes its temp work dir (#73)
- **Resume after a partial bundle** — image tarballs are kept so `-resume` can reuse them (#74)
- **Cancelling a download in the TUI** — images that finished before the cancel are kept and listed for review; a retry gets a fresh context (#75)
- **Chart pull directory** — charts are pulled into `<work_dir>/chart-pull`, so a leftover or unrelated `.tgz` in the work dir can no longer be picked up as the chart (#76)
- **Stalled registry** — a pull that receives no data for 2 minutes now fails and is retried instead of hanging forever. Large layers are not time-limited (#77)

## Upgrade notes

- If you relied on `HTTP_PROXY` being applied to HTTPS traffic, set `HTTPS_PROXY` or pass `-proxy` explicitly.
- Charts are now pulled into `<work_dir>/chart-pull`; stale `*.tgz` files in that subdirectory are removed before each pull.

## Install

```bash
# Go
go install github.com/julienhmmt/helmdownloader@v0.6.0

# Or download the release asset for your OS/arch from GitHub Releases,
# verify checksums.txt, then run:
./helmdownloader version
```

## Full changelog

https://github.com/julienhmmt/helmdownloader/compare/v0.5.1...v0.6.0
