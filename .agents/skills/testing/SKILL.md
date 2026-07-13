---
name: testing
description: How to write tests in helmdownloader — table-driven tests, fakes for imageSaver/helmClient, race requirements, short/smoke splits. Invoke when adding or changing tests.
triggers:
  - user
  - model
---

# Testing

## Requirements

- Every `pkg/` package has (or gets) `_test.go`. Keep coverage when adding code.
- Prefer **table-driven** tests + subtests.
- Use **testify** (`assert` / `require`) where the suite already does.
- Package under test: black-box (`package foo_test`) when testing public API only;
  white-box (`package foo`) when exercising unexported helpers is necessary.
- Name: `Test<Function>_<Scenario>` or `Test<Type>_<Scenario>`.

## Commands

```bash
make test              # go test ./... -count=1
make test-race         # with -race — required before done
go test ./pkg/images/ -run TestExtract -v
go test ./... -short   # skip smoke / network
```

## Test seams (do not remove)

In `pkg/pipeline`:

| Seam | Production | Tests |
| ---- | ---------- | ----- |
| `imageSaver` | `*registry.Puller` | fake with controllable Save / onBytes |
| `helmClient` | `*helm.Client` | fake Pull / Template / ShowValues / SubchartValues |
| `retryBaseDelay` | 1s | shrink for fast retry tests |

`imageSaver.Save` signature includes `onBytes registry.BytesFunc` — fakes must
accept and optionally invoke it so progress paths stay covered.

## Patterns

### Table-driven

```go
func TestExtract_SplitImageMap(t *testing.T) {
    tests := []struct {
        name string
        yaml string
        want []string
    }{
        {
            name: "registry repository tag",
            yaml: "image:\n  registry: docker.io\n  repository: nginx\n  tag: \"1.27\"\n",
            want: []string{"docker.io/nginx:1.27"},
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := images.Extract(tt.yaml)
            // assert refs...
        })
    }
}
```

### Helpers

```go
func mustWrite(t *testing.T, path, body string) {
    t.Helper()
    // ...
}
```

### Smoke / integration

`pkg/pipeline/smoke_test.go` needs helm + network. Pattern:

```go
if testing.Short() {
    t.Skip("skipping smoke test in -short mode")
}
```

Do not turn unit tests into network tests. Prefer fakes.

## What to cover when changing...

| Change area | At minimum |
| ----------- | ---------- |
| `pkg/images` | table of YAML shapes (string + split map + multi-doc) |
| `pkg/registry` | Save success path with fake registry or file fixtures; progress callbacks |
| `pkg/bundle` | Create layout, checksums, safe names, Verify/Diff, compression codecs |
| `pkg/pipeline` Download | order preservation, partial failures, resume sidecars, retries, disk check |
| `pkg/pipeline` Prepare | values/set options, subchart values merge, temp vs fixed work dir cleanup |
| `pkg/config` | Default + Load missing file + partial YAML |
| `internal/tui` | pure helpers (sort/filter, imagelist, convert); message handling without full tea.Program when possible |

## Concurrency bugs

Download uses errgroup + mutex. Any change to shared state in workers requires
`make test-race` green. Prefer reproducing the race in a unit test before the fix.

## Do not

- Assert on log output unless the package is `pkg/log`.
- Sleep fixed wall-clock delays; shrink `retryBaseDelay` instead.
- Hit real ArtifactHub/registries in unit tests.
