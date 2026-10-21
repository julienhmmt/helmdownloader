# AGENTS.md

Agent guidance for [helmdownloader](https://github.com/julienhmmt/helmdownloader).

Keep this file short. Deep, task-specific guidance lives in project skills under
`.agents/skills/` — invoke them when the task matches.

## What this is

Terminal UI (Bubble Tea) that searches Helm charts on ArtifactHub, discovers
container images, pulls them **daemonlessly** (no Docker), and bundles
chart + values + retagged image tarballs into a single integrity-checked
`.tar.gz` / `.tar.zst` for airgapped infrastructure. Generated `load.sh` loads
and pushes every image on the airgapped side.

Also: `helmdownloader verify <bundle>` and `helmdownloader diff <a> <b>`.

Module: `github.com/julienhmmt/helmdownloader` · Go **1.26+** · License **AGPL-3.0**

## When to use which skill

| Task | Skill |
| ---- | ----- |
| Any non-trivial change, architecture, where to put code | `/architecture` |
| New package logic, Go style, interfaces, concurrency | `/go-conventions` |
| Writing or changing tests | `/testing` |
| TUI screens / Bubble Tea model-update-view | `/tui` |
| Config field, CLI flag, defaults | `/config` |
| Before calling a change done | `/verify-change` |

## Commands (Makefile)

| Target | Command | Notes |
| ------ | ------- | ----- |
| Build | `make build` | `go build -o helmdownloader .` |
| Release build | `make build-release` | stripped + trimpath |
| Test | `make test` | `go test ./... -count=1` |
| Test + race | `make test-race` | **required** before done (pipeline is concurrent) |
| Lint | `make go-lint` | golangci-lint v2 |
| Vet | `make go-vet` | |
| Vulns | `make govulncheck` | |
| Security suite | `make security` | vet + lint + vuln |
| Install | `make install` | |

Single package: `go test ./pkg/pipeline/ -run TestName -v`

Smoke tests need network/helm; they skip under `-short`.

## Hard rules

1. **Always run `make test-race`** before considering a change done.
2. **Preserve test seams**: `imageSaver` and `helmClient` in `pkg/pipeline` —
   tests inject fakes; do not couple production types into test-only paths.
3. **Config is one source of truth**: new setting = field on `config.Config` +
   default in `config.Default` + CLI flag in `main.go` + README docs. Repeatable
   flags use `stringSlice` in `main.go`.
4. **Wrap errors**: `fmt.Errorf("...: %w", err)`. `errorlint` enforces this.
5. **Do not add Docker/daemon dependency** — image I/O is go-containerregistry only.
   Helm is the only required external binary (`helm.Check` at startup).
6. **Platform code**: use build tags (`diskspace_unix.go` / `diskspace_other.go`);
   always keep a no-op fallback so non-unix builds compile.
7. **gofmt / goimports** mandatory; local-prefix is `github.com/julienhmmt/helmdownloader`.
8. **English** for code and docs. Prefer small, single-purpose files (see
   `internal/tui` model/update/view split).

## Non-goals / do not

- Do not invent a new task runner (Makefile is canonical).
- Do not commit secrets, `.env`, logs, binaries, `archives/`, or `graphify-out/`.
- Do not change CI security steps or weaken lint just to pass green.
- Do not break batch download behavior: one image failure must not abort the
  rest (`[]ImageFailure`); results stay in **input order** (fixed-slot array).

## Pointers

| Path | Role |
| ---- | ---- |
| `main.go` | CLI flags, subcommands (`verify`, `diff`, `batch`), preflight, TUI entry |
| `pkg/config` | YAML config + defaults |
| `pkg/batch` | Headless YAML-list download loop (automation; no TUI) |
| `pkg/artifacthub` | ArtifactHub REST (search, versions) |
| `pkg/helm` | Shell out to `helm` (hermetic repo/cache per work dir) |
| `pkg/images` | Extract image refs; retag with registry prefix |
| `pkg/registry` | Daemonless pull/save tarball + digest + byte progress |
| `pkg/bundle` | Assemble archive + `load.sh` + checksums + verify/diff/SBOM |
| `pkg/pipeline` | Prepare → Download → Bundle; retry; disk space |
| `pkg/log` | Leveled file logger |
| `internal/tui` | Bubble Tea screens |
| `README.md` | User-facing CLI, screens, security review workflow |
| `.golangci.yml` | Enabled linters |
| `.github/workflows/` | CI (race tests, lint, govulncheck) |
