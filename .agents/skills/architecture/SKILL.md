---
name: architecture
description: HelmDownloader architecture map, data flow, package boundaries, and key design invariants. Invoke when placing new code, understanding Prepare/Download/Bundle, image discovery, digests, or airgap bundle layout.
triggers:
  - user
  - model
---

# Architecture

## End-to-end flow

```
Search (ArtifactHub) → Versions → Prepare (helm pull + template)
  → Review (toggle/add images) → Download (parallel registry.Save)
  → Bundle (.tar.gz | .tar.zst) → Done
```

Entry: `main.go` loads `config`, merges CLI flags, validates compression,
runs `helm.Check`, then `tui.Run`. The TUI owns screens; `pkg/pipeline`
owns orchestration.

## Package map

| Package | Responsibility | Key types / APIs |
| ------- | -------------- | ---------------- |
| `pkg/config` | YAML + defaults | `Config`, `Default()`, `Load` |
| `pkg/artifacthub` | REST search/versions | `Client.Search`, `Versions`; `Package.IsOCI()` |
| `pkg/helm` | Shell `helm` only | `Pull`, `Template`, `ShowValues`, `SubchartValues`, `Check`; hermetic `HELM_REPOSITORY_CONFIG` + `HELM_REPOSITORY_CACHE` per work dir |
| `pkg/images` | Discover + retag | `Extract`, `Image`, `Retag`, `PullRef` |
| `pkg/registry` | Daemonless pull | `Puller.Save(ctx, src, dest, path, onBytes) (digest, err)` |
| `pkg/bundle` | Archive + verify | `Create(Spec)`, `Verify`, `Diff`; codecs in `compress.go` |
| `pkg/pipeline` | Orchestration | `Prepare`, `Download`, `Bundle`; seams `imageSaver`, `helmClient` |
| `pkg/log` | File logger | silent / info / debug |
| `internal/tui` | Bubble Tea UI | model / update / view / commands split |

`main` also hosts non-TUI subcommands: `verify`, `diff` → `bundle.Verify` / `bundle.Diff`.

## Design invariants (do not break)

1. **Daemonless**: no Docker client/daemon. Only external binary is `helm`.
2. **Image discovery is best-effort**: charts render with defaults (plus
   optional `-values` / `-set`). Order of sources scanned:
   - rendered manifests (`helm template`)
   - top-level `values.yaml`
   - every subchart `charts/*/values.yaml` (`helm.SubchartValues`)
   - split form `registry` / `repository` / `tag` / `digest` under `image` keys
   - user can add more on Review (`a` key) or via `-import-images`
3. **Digests are pinned**: `registry.Save` returns resolved manifest digest →
   `images.txt`, `manifest.json`, and `.digest` sidecar (for `-resume`).
   Tarball remains **tag-referenced** (docker tar cannot be digest-tagged);
   digest is for verification, not for load identity.
4. **Batch resilience**: `Download` tries every image; returns
   `[]bundle.ImageEntry` + `[]ImageFailure`. Never abort the batch on one
   failure. Fixed-slot result array keeps **input order**.
5. **Retry**: `saveWithRetry` exponential backoff; cancellable via context.
   Tests shrink `retryBaseDelay`.
6. **Preflight**: helm presence, compression codec, free disk (`-min-free-mb`)
   fail before the long download path.
7. **Bundle integrity**: every file hashed into `sha256sums.txt`. Generated
   `load.sh` verifies checksums, is idempotent, honors `DRY_RUN=1`.
8. **Resume**: with fixed `-work-dir` + `-resume`, reuse existing tarballs and
   `.digest` sidecars instead of re-pulling.
9. **Hermetic helm**: each pull sets `HELM_REPOSITORY_CONFIG` and
   `HELM_REPOSITORY_CACHE` under the work dir — never touch the user's
   `~/.config/helm/repositories.yaml`.

## Bundle layout

```
<chart>-<version>-bundle.tar.{gz|zst}
├── <chart>-<version>.tgz   # filepath.Base(ChartPath) from helm pull
├── values.yaml
├── images/<sanitized-ref>.tar
├── images.txt              # source_ref  dest_ref  tar_name  digest
├── manifest.json           # provenance: tool, chart, codec, digests
├── sbom.spdx.json          # SPDX 2.3: chart + images with pinned digests
├── sha256sums.txt
└── load.sh
```

## Where to put new code

| Change | Location |
| ------ | -------- |
| New CLI flag / config field | `pkg/config` + `main.go` + README |
| Image discovery heuristic | `pkg/images` (unit tests first) |
| Pull / save / auth / progress | `pkg/registry` |
| Archive contents / load.sh / verify | `pkg/bundle` |
| Concurrency / retry / resume / disk | `pkg/pipeline` |
| New TUI screen or keybinding | `internal/tui` (messages → update → view) |
| ArtifactHub API field | `pkg/artifacthub` |
| Platform-specific syscall | build-tagged files next to caller |

## Context cancellation

TUI holds a root `ctx` cancelled on quit/reset so in-flight helm and registry
ops abort. Pipeline methods take `context.Context` — honor it in any new
long-running path.

## Related skills

- `/go-conventions` for Go style
- `/testing` for fakes and table tests
- `/tui` for Bubble Tea patterns
- `/config` for adding settings
- `/verify-change` for done criteria
