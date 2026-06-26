# HelmDownloader

A TUI (Terminal User Interface) application (v0.2.0) for downloading Helm charts and their container images, then bundling them into a single, integrity-checked archive for airgapped infrastructure.

## Features

- **Search**: Search for Helm charts on [ArtifactHub](https://artifacthub.io)
- **Select**: Choose Helm charts and their versions
- **Auto-discover**: Automatically extract all container image references from a rendered chart and its `values.yaml`, including the split `registry`/`repository`/`tag`/`digest` form used by many charts
- **Review**: Manually add, remove, or toggle individual images before downloading
- **Download**: Daemonless image pulling using [go-containerregistry](https://github.com/google/go-containerregistry) (no Docker required)
- **Archive**: Create a single compressed bundle per chart containing the chart, values, and all retagged image tarballs

## Prerequisites

[Helm](https://helm.sh/docs/intro/install/) must be installed and on your `PATH` (or set `helm_bin` in the config). It is used to pull and render charts; image pulling itself is daemonless and needs no Docker. helmdownloader checks for a working helm at startup and exits with a clear message if it is missing.

Chart pulls are **hermetic**: each `helm pull` runs against a private repository config and cache scoped to the work directory, so the tool never reads your global `~/.config/helm/repositories.yaml`. A stale or removed entry there cannot break an unrelated pull with `Error: no cached repo found. (try 'helm repo update')` — and you don't need to run `helm repo update` beforehand.

## Installation

```bash
go install github.com/julienhmmt/helmdownloader@latest
```

Or build from source:

```bash
git clone https://github.com/julienhmmt/helmdownloader.git
cd helmdownloader
go build -o helmdownloader .
```

## Usage

### Quick Start

```bash
./helmdownloader
```

The TUI starts in a search screen. Type a chart name (e.g. `argo-cd`), press `Enter`, then navigate through the results to select a chart and version.

### Screens

| Screen | Keys | Description |
| ------ | ---- | ------------ |
| **Search** | `Enter` to search, `Esc` to quit | Type a chart name to search ArtifactHub |
| **Results** | `Enter` to select, `/` to filter, `Esc` to back | Browse matching charts |
| **Versions** | `Enter` to select, `/` to filter, `Esc` to back | Pick a chart version |
| **Review** | `Space` toggle, `a` add, `d` delete, `Enter` download, `Esc` back | Review auto-discovered images |
| **Add Image** | `Enter` confirm, `Esc` cancel | Manually add an image reference |
| **Download** | (waits) | Pulls images and builds the bundle |
| **Done** | `n` new bundle, `q` quit | Shows the path to the created bundle |

### CLI Flags

```bash
./helmdownloader \
  -registry-prefix "my.registry.local" \
  -platform "linux/amd64" \
  -output "./archives" \
  -work-dir "./workdir" \
  -proxy "http://proxy.domain.local:3128" \
  -v \
  -log-level "debug" \
  -log-file "helmdownloader.log"
```

| Flag | Default | Description |
| ------ | ------- | ----------- |
| `-config` | `~/.config/helmdownloader/config.yaml` | Path to config file |
| `-values` | (none) | Extra values file layered onto the chart when rendering for image discovery (repeatable) |
| `-set` | (none) | Values override `key=value` for image discovery, e.g. `monitoring.enabled=true` (repeatable) |
| `-registry-prefix` | (from config) | Private registry prefix for retagging |
| `-platform` | (from config) | Target platform for images, e.g. `linux/amd64` |
| `-output` | (from config) | Output directory for bundles (default: archives) |
| `-work-dir` | (from config) | Work directory for intermediate files (charts, images). If empty, a temporary directory is used |
| `-resume` | `false` | Reuse image tarballs already present in a persistent work dir instead of re-pulling (use with `-work-dir`) |
| `-compression` | `gzip` | Bundle compression codec: `gzip` (`.tar.gz`) or `zstd` (`.tar.zst`, smaller) |
| `-min-free-mb` | `500` | Minimum free disk space (MiB) required on the work dir before downloading; `0` disables the check |
| `-concurrency` | `4` | Maximum number of images downloaded in parallel |
| `-retries` | `2` | Retry attempts per failed image pull (exponential backoff) |
| `-proxy` | (from config) | Proxy URL for network requests (e.g. `http://proxy.domain.local:3128`) |
| `-v` | `false` | Enable verbose logging (shortcut for `--log-level=debug`) |
| `-log-level` | `info` | Set log level: `silent`, `info`, or `debug` |
| `-log-file` | `helmdownloader.log` | Path for log output |
| `-export-images` | (none) | Write the discovered image list (JSON) to this path after rendering, for security review |
| `-import-images` | (none) | Read an approved image list (JSON) from this path at download time, overriding the discovered set |

### Configuration File

Create `~/.config/helmdownloader/config.yaml`:

```yaml
registry_prefix: "rgy01.domain.local"
platform: "linux/amd64"
output_dir: "archives"
work_dir: ""
concurrency: 4
retries: 2
compression: "gzip"          # gzip (.tar.gz) or zstd (.tar.zst, smaller)
min_free_disk_mb: 500        # free space required on work dir; 0 disables
resume: false                # reuse tarballs already in a persistent work_dir
https_proxy: "http://proxy.domain.local:3128"
helm_bin: "helm"
artifacthub_url: "https://artifacthub.io"
search_limit: 20
verbose: true
log_level: "debug"
log_file: "helmdownloader.log"
```

### Security Review Workflow

Use `-export-images` and `-import-images` to review the discovered image list with a security team before pulling:

```bash
# 1. Run with -export-images: discover images, write the list, then quit
#    from the Review screen (Esc) without downloading.
./helmdownloader -export-images images.json

# 2. Security team reviews/edits images.json (set selected: true/false,
#    remove untrusted refs, add missing ones).

# 3. Run with -import-images: the approved list overrides the discovered
#    set at download time.
./helmdownloader -import-images images.json
```

The JSON format is an array of entries:

```json
[
  {"ref": "quay.io/argoproj/argocd:v3.2.6", "selected": true},
  {"ref": "redis:7", "selected": false}
]
```

## Bundle Format

Each bundle is a `.tar.gz` (or `.tar.zst` with `-compression zstd`) named `<chart>-<version>-bundle.tar.gz` containing:

```text
<chart>-<version>.tgz     # the Helm chart
values.yaml               # default chart values
images/
  <image1>.tar            # retagged image tarball
  <image2>.tar
images.txt                # manifest: source_ref  dest_ref  tar_name  digest
manifest.json             # provenance: tool, chart, codec, images + digests
sbom.spdx.json            # SPDX 2.3 SBOM: chart + images with pinned digests
sha256sums.txt            # sha256 of every bundled file (sha256sum -c format)
load.sh                   # verifies checksums, then loads and pushes every image
```

The `images.txt` manifest maps original references to their retagged counterparts and records the resolved manifest digest (`sha256:...`, or `-` when the registry reported none) of exactly what was bundled, making it easy to script and verify the import side on airgapped infrastructure.

An SPDX 2.3 JSON SBOM (`sbom.spdx.json`) lists the chart and every image with its pinned manifest digest, for ingestion into standard SBOM tooling on the airgapped side.

On the airgapped side, extract the bundle and run the generated `load.sh` to load every image into your container engine and push it to the target registry:

```bash
tar xzf argo-cd-1.0.0-bundle.tar.gz
./load.sh                  # verifies checksums, then loads + pushes (docker by default)
ENGINE=podman ./load.sh    # use podman instead
DRY_RUN=1 ./load.sh        # print load/push commands without running them
```

`load.sh` verifies `sha256sums.txt` before touching the registry, skips loading any image already present locally (idempotent re-runs), and honors `DRY_RUN=1` for a no-op preview.

## Architecture

```text
┌──────────────┐    ┌───────────────┐    ┌─────────────┐
│   Search     │───>│   Versions    │───>│   Review    │
│  (TUI)       │    │   (TUI)       │    │   (TUI)     │
└──────────────┘    └───────────────┘    └──────┬──────┘
                                                  │
                           ┌──────────────────────┘
                           ▼
              ┌─────────────────────────────┐
              │  helm pull + helm template    │
              │  -> auto-extract images       │
              └──────────────┬────────────────┘
                             │
              ┌──────────────▼──────────────┐
              │  go-containerregistry       │
              │  pull (pinned platform)     │
              │  save as docker tarball     │
              └──────────────┬──────────────┘
                             │
              ┌──────────────▼──────────────┐
              │  bundle as .tar.gz          │
              └─────────────────────────────┘
```

### Packages

| Package | Responsibility |
| ------- | -------------- |
| `pkg/config` | YAML config loading with defaults |
| `pkg/artifacthub` | ArtifactHub REST API client |
| `pkg/helm` | Shell-outs to `helm` binary (pull, template, show values) |
| `pkg/images` | Parse rendered YAML manifests to extract `image:` references; retag with registry prefix |
| `pkg/registry` | Daemonless image pull and save via `go-containerregistry` |
| `pkg/bundle` | Assemble chart + values + image tarballs into a single `.tar.gz` |
| `pkg/pipeline` | Orchestrate the full flow with progress callbacks |
| `pkg/log` | Leveled logger (silent/info/debug) writing to the log file |
| `internal/tui` | Bubble Tea terminal UI with all screens |

## Image Discovery

Helm charts often declare images inside templates using `.Values.image.repository` and `.Values.image.tag`. To discover them, HelmDownloader renders the chart with default values using `helm template`, then recursively scans every YAML document for keys named `image`.

This means:

- ✅ Images in Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, etc. are found
- ✅ Images in initContainers are found
- ✅ Sidecar images are found
- ✅ Subchart images are found — every bundled `charts/*/values.yaml` is scanned, catching split-form images for components disabled by default
- ⚠️ Images behind conditional logic (e.g. `{{- if .Values.monitoring.enabled }}`) may be missed if the condition is false with default values

To surface conditional images at render time, pass extra values with `-values myvalues.yaml` or `-set monitoring.enabled=true` (both repeatable). These only widen discovery; the bundle still ships the chart's default `values.yaml`.

You can always manually add missing images using the `a` key on the Review screen.

## Requirements

- [Go](https://go.dev) 1.26+ (to build)
- [Helm](https://helm.sh) 3.x (runtime dependency, must be in `$PATH`)
- Network access to ArtifactHub and the chart/image registries

## License

GNU AGPL v3 — see [LICENSE](LICENSE)
