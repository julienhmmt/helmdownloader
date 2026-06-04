# CLAUDE.md

Guidance for working in this repository.

## What This Is

HelmDownloader is a terminal UI (Bubble Tea) that searches Helm charts on
ArtifactHub, auto-discovers their container images, pulls the images
daemonlessly (no Docker), and bundles chart + values + retagged image tarballs
into a single `.tar.gz` for airgapped infrastructure. A generated `load.sh`
loads and pushes every image on the airgapped side.

## Commands

Use the `task` runner (Taskfile.yml):

| Task | Command |
| ---- | ------- |
| Build | `task build` (or `go build -o helmdownloader .`) |
| Test | `task test` (`go test ./... -count=1`) |
| Test w/ race | `task test-race` |
| Vet | `task go-vet` |
| Lint | `task go-lint` (golangci-lint) |
| Install | `task install` |

Run a single package's tests: `go test ./pkg/pipeline/ -run TestName -v`.

Always run `task test-race` before considering a change done — the pipeline
download path is concurrent.

## Architecture

Flow: **Search → Versions → Review → Download → Bundle**.

`main.go` loads config, merges CLI flag overrides, runs a helm preflight
(`helm.Check`), then starts the TUI. The TUI drives `pkg/pipeline`, which
orchestrates everything else.

| Package | Responsibility |
| ------- | -------------- |
| `pkg/config` | YAML config + defaults (`config.Default`, `config.Load`) |
| `pkg/artifacthub` | ArtifactHub REST client (search, versions) |
| `pkg/helm` | Shells out to `helm` (pull, template, show values); proxy via `HTTPS_PROXY` env |
| `pkg/images` | Extract `image:` refs from rendered manifests + values.yaml; retag with registry prefix |
| `pkg/registry` | Daemonless pull + save to docker tarball via go-containerregistry `tarball.Write`; resolves the manifest digest and streams byte progress through a counting writer |
| `pkg/bundle` | Assemble chart + values + image tars + `images.txt` + `manifest.json` + `sha256sums.txt` + `load.sh` into `.tar.gz` or `.tar.zst` (codec in `compress.go`) |
| `pkg/pipeline` | Orchestrate Prepare → Download → Bundle with progress callbacks + retry |
| `pkg/log` | Leveled logger (silent/info/debug) to a file |
| `internal/tui` | Bubble Tea screens (model/update/view split) |

## Key Design Decisions

- **Daemonless**: images pulled via go-containerregistry, not Docker. Helm is
  the only external binary, checked at startup.
- **Image discovery is best-effort**: charts render with default values, so
  images behind disabled conditionals are missed. Mitigations, in order: the
  rendered manifests, the top-level values.yaml, and every subchart
  `charts/*/values.yaml` (`helm.SubchartValues`) are all scanned for the split
  `registry`/`repository`/`tag`/`digest` form; `-values`/`-set` widen the render;
  the user adds the rest manually on the Review screen (`a` key).
- **Digests are pinned**: `registry.Save` returns the resolved manifest digest,
  which flows into `images.txt`, `manifest.json`, and a `.digest` sidecar (used
  by `-resume`). The retagged tarball is still tag-referenced (a tarball cannot
  be digest-tagged); the digest is recorded for verification, not loading.
- **Bundle integrity**: every bundled file is sha256'd into `sha256sums.txt`;
  the generated `load.sh` verifies it (sha256sum/shasum) before pushing, is
  idempotent (skips images already present), and honors `DRY_RUN=1`.
- **Failures don't abort the batch**: `pipeline.Download` attempts every image
  and returns `[]ImageFailure`, so the user sees the full failure set at once.
- **Retry with exponential backoff**: `saveWithRetry`, cancellable via context.
- **Results stay in input order** despite parallel completion (fixed-slot
  results array, not append-on-finish).
- **Preflight before the long path**: helm presence, compression codec, and
  free disk space (`-min-free-mb`) all fail fast before any download.

## Conventions

- Go 1.26+. `gofmt`/`goimports` mandatory.
- Wrap errors with context: `fmt.Errorf("...: %w", err)`.
- Table-driven tests with testify. Every `pkg/` has a `_test.go`; keep coverage
  when adding code. Tests substitute fakes for `imageSaver` / helm — preserve
  those seams.
- Files stay small and focused (the `internal/tui` model/update/view split is
  the pattern to follow).
- Config has one source of truth: add a field to `config.Config`, a default in
  `config.Default`, a CLI flag override in `main.go`, and document it in
  README. Repeatable flags use the `stringSlice` `flag.Value` in `main.go`.
- Platform-specific code is build-tagged (`diskspace_unix.go` /
  `diskspace_other.go`); keep a no-op fallback so non-unix builds compile.
- Tests substitute fakes for `imageSaver` (now `Save(..., onBytes)`) and helm —
  preserve those seams. Smoke tests need network/helm; they skip under `-short`.

## License

GNU AGPL v3.
