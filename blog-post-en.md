---
title: "HelmDownloader: from bash scripts to a single binary"
date: 2026-07-14
tags: ["kubernetes", "helm", "airgap", "golang"]
params:
  author: "Julien HOMMET"
draft: false
---

If you have ever had to deploy Kubernetes onto a platform with no Internet access, you know the drill. No `helm pull` quietly reaching out to the web. No `docker pull` fetching the image at deploy time. Everything has to be prepared **beforehand**, copied onto some media, and reloaded on the other side of the wall.

And the worst part with Helm isn't the chart itself. It's tracking down **every** container image it pulls in. Any serious chart hides a dozen of them: the main container, the sidecars, the initContainers, the metrics exporter, the admission webhook… Miss one, and your deployment is stuck in `ImagePullBackOff` at the worst possible moment.

HelmDownloader (**v0.3.0**) solves exactly that problem.

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
- **Sort and filter the results.** Sort matches by stars, name, or last-updated date, and filter by author or publishing company — all on the already-fetched results, no extra queries. Official and deprecated badges help you pick the right package.
- **Automatic image discovery.** No more hard-coded list. The tool runs `helm template`, recursively walks every rendered manifest, the top-level `values.yaml`, and every subchart `charts/*/values.yaml`, and extracts all image references — including the split `registry` / `repository` / `tag` / `digest` form many charts use. Pass `-values` / `-set` to surface images gated on non-default values.
- **Faster bundling.** Image pulling is *daemonless* via [go-containerregistry](https://github.com/google/go-containerregistry): **no Docker required**. Images download in parallel with per-image progress, retries, and exponential backoff. A failed image doesn't abort the batch — you see the full failure set at once. Esc cancels a busy download while keeping partial successes.
- **Verifiable bundles.** Every bundled file is sha256'd into `sha256sums.txt` (including `load.sh`), image manifest digests are pinned in `images.txt` and `manifest.json`, an SPDX 2.3 SBOM is written as `sbom.spdx.json`, and the generated `load.sh` verifies checksums before pushing.
- **Integrity tooling.** `helmdownloader verify <bundle>` re-hashes everything offline after transfer; `helmdownloader diff <a> <b>` shows which images were added, removed, or changed between two chart versions.
- **Security review workflow.** Export the discovered image list as JSON (`-export-images`), hand it to a security team, re-import the approved set (`-import-images`). Invalid refs fail closed before download.
- **Private registries.** `-registry-auth` uses the default Docker keychain (`docker login` / `podman login`, or `$DOCKER_CONFIG`).
- **Smaller archives.** Pick `gzip` (`.tar.gz`) or `zstd` (`.tar.zst`) compression. A disk-space preflight and a hardened `-resume` flag (reuse tarballs only when content-hash and registry digest sidecars match) make big batches safe to retry.
- **Themes.** Six palettes (`auto`, `light`, `dark`, `high-contrast`, `ocean`, `matrix`) via `-theme` or live with `Ctrl+T`.
- **Shippable.** A static Go binary (v0.3.0), compiled for Linux, macOS and Windows, on amd64 and arm64. Drop it in, run it. `helmdownloader version` prints the embedded release tag.
- **No Docker daemon.** No need to have `docker` nor `podman` on your connected machine for pulling — *HelmDownloader* uses an internal Go library (a container engine is only needed later, on the airgapped side, when running `load.sh`).
- **Hermetic pulls.** Each `helm pull` runs against a private repository config and cache scoped to the work dir, so the tool ignores your global helm repos entirely. A stale or removed local repo can't break a pull with `Error: no cached repo found. (try 'helm repo update')` — no `helm repo update` needed first.

Where every new application used to need a new bash script, there is now a single tool that covers them all — and adapts automatically to version changes.

## What it actually does

HelmDownloader is a **TUI** (terminal user interface) app that:

1. **Searches** for a Helm chart on [ArtifactHub](https://artifacthub.io)
2. **Selects** the chart and version you want
3. **Auto-discovers** every container image in the chart
4. **Lets you review** the list: add, remove, toggle images one by one (windowed for large charts; refs validated on add)
5. **Downloads** the images (no Docker) and retags them for your private registry
6. **Bundles** everything into a **single compressed archive** ready to cross the airgap

The bundle it produces contains the chart, its `values.yaml`, each image as a tarball, an `images.txt` manifest mapping original references to their retagged versions (with pinned digests), a `manifest.json` provenance file (tool name + version, chart, codec, images), an SPDX 2.3 `sbom.spdx.json`, a `sha256sums.txt` checksum list, and a `load.sh` script that verifies checksums, then reloads and pushes everything to your registry on the other side. `load.sh` is idempotent (skips images already present) and honors `DRY_RUN=1`.

This is the **airgap** use case in a nutshell: one file to transfer, one command to run on arrival.

## Mini tutorial: from chart to bundle in 2 minutes

### Prerequisites

[Helm](https://helm.sh/docs/intro/install/) must be installed and on your `PATH`. It's the only runtime dependency on the connected machine — image pulling itself needs no Docker daemon.

### Install

```bash
go install github.com/julienhmmt/helmdownloader@latest
```

Or grab the binary for your platform straight from the [releases page](https://github.com/julienhmmt/helmdownloader/releases).

### Step 1 — Launch the tool

Just do `./helmdownloader`.

There are CLI args to customize the tool: registry address, platform, themes, logs, destination path, and more.

```bash
helmdownloader \
  -registry-prefix "rgy01.domain.local" \
  -platform "linux/amd64" \
  -output "./archives" \
  -theme dark
```

The interface opens on a search screen. Press `Ctrl+T` anytime to open the theme picker.

### Step 2 — Search and select

Type a chart name (for example `argo-cd`), hit `Enter`, then navigate the results to pick a chart and version. Each row shows stars, repo, publisher, app version, and description, plus official/deprecated badges when applicable. On a crowded result list, press `s` to sort by stars, name, or update date, `o` to flip the direction, and `f`/`F`/`Tab` to filter by author or company.

| Screen | Keys |
| ------ | ---- |
| Search | `Enter` to search, `Ctrl+T` themes, `Esc` to quit |
| Results | `Enter` select, `/` fuzzy filter, `s` sort field, `o` direction, `f` filter field, `F` type filter, `Tab` cycle values |
| Versions | `Enter` to select, `/` to filter |
| Review | `Space` toggle, `a` add, `d` delete, `j`/`k` move, `PgUp`/`PgDn` page, `Enter` download |
| Download | `Esc` cancel (keeps partial successes) |
| Done | `n` new bundle, `q` quit |

### Step 3 — Review the images

The tool shows every image it discovered. A conditional image got missed (hidden behind a `{{- if .Values.monitoring.enabled }}` that's off by default)? Press `a` to add it by hand — refs are validated before they land in the list. `Space` to uncheck what you don't need. Long lists are windowed so large charts stay usable.

To widen discovery up front, pass extra values when you launch:

```bash
helmdownloader -set monitoring.enabled=true -values ./extra-values.yaml
```

### Step 4 — Download and bundle

`Enter` kicks off the pipeline. Images download in parallel with per-image progress, get retagged to the registry if specified, and the archive `argo-cd-<version>-bundle.tar.gz` (or `.tar.zst` with `-compression zstd`) lands in `./archives`. Partial successes are kept if you cancel mid-run with Esc.

### Step 5 — On the other side of the airgap

Transfer the bundle to the disconnected platform, then:

```bash
# optional offline check before extract/load
helmdownloader verify argo-cd-1.0.0-bundle.tar.gz

tar xzf argo-cd-1.0.0-bundle.tar.gz
./load.sh                 # verifies checksums, then loads + pushes (docker by default)
ENGINE=podman ./load.sh   # or use podman
DRY_RUN=1 ./load.sh       # print load/push commands without running them
```

The script verifies `sha256sums.txt`, reloads each image, and pushes it to your registry. The chart itself is ready to deploy with `helm install`.

When you upgrade a chart later, compare two bundles to see exactly what to re-mirror:

```bash
helmdownloader diff argo-cd-1.8.0-bundle.tar.gz argo-cd-1.9.0-bundle.tar.gz
```

## Persistent configuration

To avoid repeating the same flags on every run, copy the annotated example and edit:

```bash
mkdir -p ~/.config/helmdownloader
cp config.example.yaml ~/.config/helmdownloader/config.yaml
```

Minimal example:

```yaml
registry_prefix: "rgy01.domain.local"
platform: "linux/amd64"
output_dir: "archives"
concurrency: 4
retries: 2
compression: "gzip"          # or zstd for smaller archives
min_free_disk_mb: 500        # disk preflight; 0 disables
https_proxy: "http://proxy.domain.local:3128"
theme: "auto"                # auto | light | dark | high-contrast | ocean | matrix
```

## Advanced configuration

### Extended config.yaml options

```yaml
registry_prefix: "rgy01.domain.local"
platform: "linux/amd64"
output_dir: "archives"
work_dir: ""                    # fixed path enables -resume; empty = temp dir
concurrency: 4
retries: 2
compression: "gzip"              # gzip (.tar.gz) or zstd (.tar.zst)
min_free_disk_mb: 500
resume: false                    # reuse tarballs when content-hash + digest sidecars match
https_proxy: "http://proxy.domain.local:3128"
helm_bin: "helm"
artifacthub_url: "https://artifacthub.io"
search_limit: 20
theme: "auto"
registry_auth: false
values_files: []
set_values: []
export_images: ""
import_images: ""
verbose: true
log_level: "debug"
log_file: "helmdownloader.log"  # opened mode 0600 when logging is on
```

A fully annotated copy of every option lives in [`config.example.yaml`](https://github.com/julienhmmt/helmdownloader/blob/main/config.example.yaml).

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
  -registry-auth \
  -values "extra-values.yaml" \
  -set "monitoring.enabled=true" \
  -export-images images.json \
  -import-images images-approved.json \
  -theme dark \
  -proxy "http://proxy.domain.local:3128" \
  -v \
  -log-level "debug" \
  -log-file "helmdownloader.log" \
  -config "~/.config/helmdownloader/config.yaml"
```

| Flag | Description |
| ---- | ----------- |
| `-config` | Path to config file (default: `~/.config/helmdownloader/config.yaml`) |
| `-work-dir` | Work directory for intermediate files. Empty → temporary directory |
| `-concurrency` | Max parallel image downloads (default: 4) |
| `-retries` | Retry attempts per failed image pull (default: 2) |
| `-compression` | Bundle codec: `gzip` or `zstd` |
| `-min-free-mb` | Minimum free disk space (MiB) before download; `0` disables |
| `-resume` | Reuse tarballs in a persistent work dir when sidecars match |
| `-registry-auth` | Authenticated pulls via the default Docker keychain |
| `-values` | Extra values file for image discovery (repeatable) |
| `-set` | Values override `key=value` for image discovery (repeatable) |
| `-export-images` | Write discovered image list (JSON) after rendering |
| `-import-images` | Read approved image list (JSON) at download time |
| `-theme` | TUI theme: `auto`, `light`, `dark`, `high-contrast`, `ocean`, `matrix` |
| `-proxy` | Proxy URL for ArtifactHub, helm, and registry traffic |
| `-v` | Verbose logging (shortcut for `--log-level=debug`) |
| `-log-level` | `silent`, `info`, or `debug` |
| `-log-file` | Path for log output |

### Subcommands

```bash
helmdownloader version                              # binary identity (tag or dev)
helmdownloader verify argo-cd-1.0.0-bundle.tar.gz   # offline integrity check
helmdownloader diff old-bundle.tar.gz new-bundle.tar.gz
```

### Security review workflow

```bash
# 1. Discover images, write the list, quit from Review (Esc) without downloading
helmdownloader -export-images images.json

# 2. Security team reviews/edits images.json (selected: true/false, remove/add refs)

# 3. Re-run with the approved list
helmdownloader -import-images images.json
```

### Private registries

```bash
docker login registry.example.com
helmdownloader -registry-auth -registry-prefix registry.example.com/mirror
# or: DOCKER_CONFIG=/path/to/creds helmdownloader -registry-auth
```

### Environment variables

If the proxy is not set via CLI or config, HelmDownloader checks:

- `HTTP_PROXY`
- `HTTPS_PROXY`

## Why you should try it

If you run Kubernetes in an airgapped environment, you almost certainly have your own bash scripts for this kind of task. HelmDownloader is those scripts, but:

- **one tool** instead of one per application;
- **automatic discovery** of images instead of a list to maintain by hand;
- **no Docker daemon** on the connected machine, parallel pulls, so faster;
- **a verifiable bundle** (pinned digests + sha256 checksums + SBOM) with its own reload script;
- **offline tooling** to verify integrity after transfer and diff chart upgrades.

It's the tool I wish I'd had the day I started stacking up scripts. The code is licensed under AGPL v3 and open to contributions.

➡️ [github.com/julienhmmt/helmdownloader](https://github.com/julienhmmt/helmdownloader)
