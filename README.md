# HelmDownloader

A TUI (Terminal User Interface) application for downloading Helm charts and their container images, then bundling them into a single archive for airgapped infrastructure.

## Features

- **Search**: Search for Helm charts on [ArtifactHub](https://artifacthub.io)
- **Select**: Choose Helm charts and their versions
- **Auto-discover**: Automatically extract all container image references from a rendered chart and its `values.yaml`, including the split `registry`/`repository`/`tag`/`digest` form used by many charts
- **Review**: Manually add, remove, or toggle individual images before downloading
- **Download**: Daemonless image pulling using [go-containerregistry](https://github.com/google/go-containerregistry) (no Docker required)
- **Archive**: Create a single compressed bundle per chart containing the chart, values, and all retagged image tarballs

## Prerequisites

[Helm](https://helm.sh/docs/intro/install/) must be installed and on your `PATH` (or set `helm_bin` in the config). It is used to pull and render charts; image pulling itself is daemonless and needs no Docker. helmdownloader checks for a working helm at startup and exits with a clear message if it is missing.

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
| `-registry-prefix` | (from config) | Private registry prefix for retagging |
| `-platform` | (from config) | Target platform for images, e.g. `linux/amd64` |
| `-output` | (from config) | Output directory for bundles (default: archives) |
| `-work-dir` | (from config) | Work directory for intermediate files (charts, images). If empty, a temporary directory is used |
| `-concurrency` | `4` | Maximum number of images downloaded in parallel |
| `-retries` | `2` | Retry attempts per failed image pull (exponential backoff) |
| `-proxy` | (from config) | Proxy URL for network requests (e.g. `http://proxy.domain.local:3128`) |
| `-v` | `false` | Enable verbose logging (shortcut for `--log-level=debug`) |
| `-log-level` | `info` | Set log level: `silent`, `info`, or `debug` |
| `-log-file` | `helmdownloader.log` | Path for log output |

### Configuration File

Create `~/.config/helmdownloader/config.yaml`:

```yaml
registry_prefix: "rgy01.domain.local"
platform: "linux/amd64"
output_dir: "archives"
work_dir: ""
concurrency: 4
retries: 2
https_proxy: "http://proxy.domain.local:3128"
helm_bin: "helm"
artifacthub_url: "https://artifacthub.io"
search_limit: 20
verbose: true
log_level: "debug"
log_file: "helmdownloader.log"
```

## Bundle Format

Each bundle is a `.tar.gz` named `<chart>-<version>-bundle.tar.gz` containing:

```text
<chart>-<version>.tgz     # the Helm chart
values.yaml               # default chart values
images/
  <image1>.tar            # retagged image tarball
  <image2>.tar
images.txt                # manifest: source_ref  dest_ref  tar_name
load.sh                   # loads and pushes every image to the registry
```

The `images.txt` manifest maps original references to their retagged counterparts, making it easy to script the import side on airgapped infrastructure.

On the airgapped side, extract the bundle and run the generated `load.sh` to load every image into your container engine and push it to the target registry:

```bash
tar xzf argo-cd-1.0.0-bundle.tar.gz
./load.sh            # uses docker by default
ENGINE=podman ./load.sh
```

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
- ⚠️ Images behind conditional logic (e.g. `{{- if .Values.monitoring.enabled }}`) may be missed if the condition is false with default values

You can always manually add missing images using the `a` key on the Review screen.

## Requirements

- [Go](https://go.dev) 1.26+ (to build)
- [Helm](https://helm.sh) 3.x (runtime dependency, must be in `$PATH`)
- Network access to ArtifactHub and the chart/image registries

## License

GNU AGPL v3 — see [LICENSE](LICENSE)
