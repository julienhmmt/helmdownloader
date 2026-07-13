---
name: go-conventions
description: Idiomatic Go conventions for this repo — errors, packages, interfaces, concurrency, formatting, linters. Invoke when writing or reviewing Go code.
triggers:
  - user
  - model
---

# Go conventions (helmdownloader)

## Tooling

- Go **1.26+** (`go.mod`)
- Format: `gofmt` + `goimports` (local-prefix: `github.com/julienhmmt/helmdownloader`)
- Lint: `make go-lint` → golangci-lint v2 (`.golangci.yml`)
  - Enabled: `errorlint`, `govet`, `ineffassign`, `misspell`, `revive`, `staticcheck`, `unused`

## Errors

```go
// wrap with %w — errorlint requires it
return nil, fmt.Errorf("pull chart %s: %w", name, err)
```

- Prefer sentinel or structured errors only when callers must `errors.Is` / `As`.
- Log at boundaries (`pkg/log`); return errors inward. Do not log *and* return
  the same error unless the call site is fire-and-forget (e.g. best-effort
  subchart scan).

## Packages

- Short, lowercase package names matching directory.
- `pkg/` is library code; `internal/tui` is private UI.
- One clear responsibility per package. Prefer adding to an existing package
  over creating a new one unless the domain is distinct.
- Keep files small and focused (split by concern like TUI model/update/view).

## Interfaces

Define small interfaces **at the consumer**, not the provider:

```go
// in pkg/pipeline — production *registry.Puller / *helm.Client satisfy these
type imageSaver interface {
    Save(ctx context.Context, srcRef, destRef, destPath string, onBytes registry.BytesFunc) (string, error)
}

type helmClient interface {
    Pull(ctx context.Context, name, repoURL, version, destDir string, oci bool) (helm.PullResult, error)
    ShowValues(ctx context.Context, chartPath string) (string, error)
    Template(ctx context.Context, chartPath string, opts ...helm.TemplateOption) (string, error)
    SubchartValues(chartPath string) ([]string, error)
}
```

Preserve these seams when refactoring. Tests inject fakes; production wiring
stays in `pipeline.New`.

## Concurrency

- Prefer `errgroup` with `SetLimit` for bounded work (see `Download`).
- Fixed-slot result slices when order matters; never `append` from workers
  without ordering.
- Guard shared counters with `sync.Mutex` (or channels); keep critical sections tiny.
- Always pass `context.Context`; check `ctx.Done()` in retries/sleeps.
- Race detector is mandatory: `make test-race`.

## Style checklist

- Exported identifiers documented with a full sentence starting with the name.
- No blank lines mid-function when it hurts density; early returns over deep nesting.
- No magic numbers — named constants (see `defaultRetryBaseDelay` in
  `pkg/pipeline`, `progressThreshold` in `pkg/registry`).
- Prefer pure helpers (`images.Extract`, `safeBundleName`) over methods with hidden state.
- Do not add comments that restate the code; only explain non-obvious *why*.

## Dependencies

- Prefer standard library when enough.
- Existing stack: Bubble Tea v2, go-containerregistry, yaml.v3, x/sync, testify,
  klauspost/compress (zstd).
- Adding a dependency: use `go get` with a version **≥7 days old**; `go mod tidy`;
  never pin to `latest` float. Do not add a Docker SDK.

## Anti-patterns in this codebase

- Coupling pipeline tests to real helm/network (smoke tests are the exception; they `t.Skip` under `-short`).
- Aborting the whole download batch on a single image error.
- Digest-tagging tarballs (unsupported — record digest separately).
- Reading the user's global helm repository cache for pulls.
