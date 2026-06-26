---
title: "HelmDownloader: from bash scripts to a single binary"
date: 2026-06-01
tags: ["kubernetes", "helm", "airgap", "golang"]
params:
  author: "Julien HOMMET"
draft: false
---

If you have ever had to deploy Kubernetes onto a platform with no Internet access, you know the drill. No `helm pull` quietly reaching out to the web. No `docker pull` fetching the image at deploy time. Everything has to be prepared **beforehand**, copied onto some media, and reloaded on the other side of the wall.

And the worst part with Helm isn't the chart itself. It's tracking down **every** container image it pulls in. Any serious chart hides a dozen of them: the main container, the sidecars, the initContainers, the metrics exporter, the admission webhook… Miss one, and your deployment is stuck in `ImagePullBackOff` at the worst possible moment.

HelmDownloader solves exactly that problem.

## The starting point: a collection of bash scripts

It didn't start as a tool. It started as a **collection of bash scripts**. One script per application. One for Argo CD, one for Prometheus, one for cert-manager, and so on.

The problems with that approach show up fast:

- **Not modular**: every script duplicated the same logic (pull the chart, extract the images, retag, archive), with copy-pasted variations from one file to the next.
- **Not practical**: adding a new application meant starting from an existing script, hacking it around, and praying nothing broke.
- **Doesn't scale**: the image list was often hard-coded. A new chart version changes its images? Reopen the script and fix it by hand.

In short: it worked, but it was fragile, painful to maintain, and it didn't scale.

## The turning point: a single binary, built with Claude Code

Instead of writing yet another bash script, I used [Claude Code](https://www.claude.com/product/claude-code) to rethink the tool from scratch and turn it into a **single binary** written in Go.

The payoff is clear:

- **One tool, all charts.** No more script per application. Search any chart, select it, and the same pipeline handles the rest.
- **Sort and filter the results.** Sort matches by stars, name, or last-updated date, and filter by author or publishing company — all on the already-fetched results, no extra queries.
- **Automatic image discovery.** No more hard-coded list. The tool runs `helm template`, recursively walks every rendered manifest, the top-level `values.yaml`, and every subchart `charts/*/values.yaml`, and extracts all image references — including the split `registry` / `repository` / `tag` / `digest` form many charts use.
- **Faster bundling.** Image pulling is *daemonless* via [go-containerregistry](https://github.com/google/go-containerregistry): **no Docker required**. Images download in parallel, with retries and exponential backoff. A failed image doesn't abort the batch — you see the full failure set at once.
- **Verifiable bundles.** Every bundled file is sha256'd into `sha256sums.txt`, image manifest digests are pinned in `images.txt` and `manifest.json`, and the generated `load.sh` verifies checksums before pushing.
- **Smaller archives.** Pick `gzip` (`.tar.gz`) or `zstd` (`.tar.zst`) compression. A disk-space preflight and a `-resume` flag (reuse already-pulled tarballs) make big batches safe to retry.
- **Shippable.** A static Go binary, compiled for Linux, macOS and Windows, on amd64 and arm64. Drop it in, run it.
- **No dependencies.** No need to have `docker` nor `podman` on your connected machine — *HelmDownloader* uses an internal Golang lib.
- **Hermetic pulls.** Each `helm pull` runs against a private repository config and cache scoped to the work dir, so the tool ignores your global helm repos entirely. A stale or removed local repo can't break a pull with `Error: no cached repo found. (try 'helm repo update')` — no `helm repo update` needed first.

Where every new application used to need a new bash script, there is now a single tool that covers them all — and adapts automatically to version changes.

## What it actually does

HelmDownloader is a **TUI** (terminal user interface) app that:

1. **Searches** for a Helm chart on [ArtifactHub](https://artifacthub.io)
2. **Selects** the chart and version you want
3. **Auto-discovers** every container image in the chart
4. **Lets you review** the list: add, remove, toggle images one by one
5. **Downloads** the images (no Docker) and retags them for your private registry
6. **Bundles** everything into a **single compressed archive** ready to cross the airgap

The bundle it produces contains the chart, its `values.yaml`, each image as a tarball, an `images.txt` manifest mapping original references to their retagged versions (with pinned digests), a `manifest.json` provenance file, a `sha256sums.txt` checksum list, and a `load.sh` script that verifies checksums, then reloads and pushes everything to your registry on the other side. `load.sh` is idempotent (skips images already present) and honors `DRY_RUN=1`.

This is the **airgap** use case in a nutshell: one file to transfer, one command to run on arrival.

## Mini tutorial: from chart to bundle in 2 minutes

### Prerequisites

[Helm](https://helm.sh/docs/intro/install/) must be installed and on your `PATH`. It's the only runtime dependency — image pulling itself needs no Docker daemon.

### Install

```bash
go install github.com/julienhmmt/helmdownloader@latest
```

Or grab the binary for your platform straight from the [releases page](https://github.com/julienhmmt/helmdownloader/releases).

### Step 1 — Launch the tool

Just do `./helmdownloader`.

There is some CLI args to customize the tool, like the registry address, the arch, logs or the destination path.

```bash
helmdownloader -registry-prefix "rgy01.domain.local" -platform "linux/amd64" -output "./archives"
```

The interface opens on a search screen.

### Step 2 — Search and select

Type a chart name (for example `argo-cd`), hit `Enter`, then navigate the results to pick a chart and version. Each row shows stars, repo, publisher, app version, and description. On a crowded result list, press `s` to sort by stars, name, or update date, `o` to flip the direction, and `f`/`F`/`Tab` to filter by author or company.

| Screen | Keys |
| ------ | ---- |
| Search | `Enter` to search, `Esc` to quit |
| Results | `Enter` select, `/` fuzzy filter, `s` sort field, `o` direction, `f` filter field, `F` type filter, `Tab` cycle values |
| Versions | `Enter` to select |
| Review | `Space` toggle, `a` add, `d` delete, `Enter` download |

### Step 3 — Review the images

The tool shows every image it discovered. A conditional image got missed (hidden behind a `{{- if .Values.monitoring.enabled }}` that's off by default)? Press `a` to add it by hand. `Space` to uncheck what you don't need.

### Step 4 — Download and bundle

`Enter` kicks off the pipeline. Images download in parallel, get retagged to the registry if specified, and the archive `argo-cd-<version>-bundle.tar.gz` lands in `./archives` (by default, or in your custom path).

### Step 5 — On the other side of the airgap

Transfer the bundle to the disconnected platform, then:

```bash
tar xzf argo-cd-1.0.0-bundle.tar.gz
./load.sh                 # verifies checksums, then loads + pushes (docker by default)
ENGINE=podman ./load.sh   # or this command to use podman
DRY_RUN=1 ./load.sh       # print load/push commands without running them
```

The script verifies `sha256sums.txt`, reloads each image, and pushes it to your registry. The chart itself is ready to deploy with `helm install`.

## Persistent configuration

To avoid repeating the same flags on every run, drop a `~/.config/helmdownloader/config.yaml`:

```yaml
registry_prefix: "rgy01.domain.local"
platform: "linux/amd64"
output_dir: "archives"
concurrency: 4
retries: 2
compression: "gzip"          # or zstd for smaller archives
min_free_disk_mb: 500        # disk preflight; 0 disables
https_proxy: "http://proxy.domain.local:3128"
```

## Advanced configuration

Beyond the basic configuration shown above, HelmDownloader supports additional options for fine-tuning its behavior:

### Extended config.yaml options

```yaml
registry_prefix: "rgy01.domain.local"
platform: "linux/amd64"
output_dir: "archives"
work_dir: ""                    # Optional: work directory for intermediate files (charts, images). If empty, a temporary directory is used
concurrency: 4                   # Max parallel image downloads
retries: 2                       # Retry attempts per failed image pull (exponential backoff)
compression: "gzip"              # Bundle codec: gzip (.tar.gz) or zstd (.tar.zst, smaller)
min_free_disk_mb: 500            # Min free disk space (MiB) on the work dir before download; 0 disables
resume: false                    # Reuse image tarballs already present in a persistent work_dir
https_proxy: "http://proxy.domain.local:3128"
helm_bin: "helm"                 # Optional: helm executable name or path
artifacthub_url: "https://artifacthub.io"  # Optional: base URL of the ArtifactHub API
search_limit: 20                 # Optional: caps the number of search results requested
verbose: true                    # Optional: enables detailed logging to a file
log_level: "debug"               # Optional: controls logging verbosity (silent, info, debug)
log_file: "helmdownloader.log"  # Optional: path where verbose output is written
```

### Additional CLI flags

```bash
helmdownloader \
  -registry-prefix "my.registry.local" \
  -platform "linux/amd64" \
  -output "./archives" \
  -work-dir "./workdir" \
  -concurrency 4 \
  -retries 2 \
  -compression "zstd" \
  -min-free-mb 500 \
  -resume \
  -values "extra-values.yaml" \
  -set "monitoring.enabled=true" \
  -proxy "http://proxy.domain.local:3128" \
  -v \
  -log-level "debug" \
  -log-file "helmdownloader.log" \
  -config "~/.config/helmdownloader/config.yaml"
```

| Flag | Description |
| ---- | ----------- |
| `-config` | Path to config file (default: `~/.config/helmdownloader/config.yaml`) |
| `-work-dir` | Work directory for intermediate files (charts, images). If empty, a temporary directory is used |
| `-concurrency` | Maximum number of images downloaded in parallel (default: 4) |
| `-retries` | Retry attempts per failed image pull (default: 2) |
| `-compression` | Bundle codec: `gzip` (`.tar.gz`) or `zstd` (`.tar.zst`, smaller) |
| `-min-free-mb` | Minimum free disk space (MiB) on the work dir before download; `0` disables |
| `-resume` | Reuse image tarballs already present in a persistent work dir (use with `-work-dir`) |
| `-values` | Extra values file layered onto the chart for image discovery (repeatable) |
| `-set` | Values override `key=value` for image discovery, e.g. `monitoring.enabled=true` (repeatable) |
| `-proxy` | Proxy URL for network requests (e.g. `http://proxy.domain.local:3128`) |
| `-v` | Enable verbose logging (shortcut for `--log-level=debug`) |
| `-log-level` | Set log level: `silent`, `info`, or `debug` (default: `info`) |
| `-log-file` | Path for log output (default: `helmdownloader.log`) |

### Environment variables

If the proxy is not set via CLI or config, HelmDownloader automatically checks the following environment variables:

- `HTTP_PROXY`
- `HTTPS_PROXY`

## Why you should try it

If you run Kubernetes in an airgapped environment, you almost certainly have your own bash scripts for this kind of task. HelmDownloader is those scripts, but:

- **one tool** instead of one per application;
- **automatic discovery** of images instead of a list to maintain by hand;
- **no Docker**, in parallel, so faster;
- **a verifiable bundle** (pinned digests + sha256 checksums) with its own reload script.

It's the tool I wish I'd had the day I started stacking up scripts. The code is licensed under AGPL v3 and open to contributions.

➡️ [github.com/julienhmmt/helmdownloader](https://github.com/julienhmmt/helmdownloader)
