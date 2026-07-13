---
name: verify-change
description: Done criteria and verification loop for helmdownloader changes. Invoke before claiming work is complete, or when running CI-equivalent checks locally.
triggers:
  - user
  - model
---

# Verify change (done criteria)

Run these before saying a change is done. Prefer Make targets.

## Minimum bar (always)

```bash
# 1. Format (if you touched .go files)
gofmt -w $(git diff --name-only --diff-filter=ACM | grep '\.go$' || true)
# goimports with local prefix if available:
# goimports -local github.com/julienhmmt/helmdownloader -w <files>

# 2. Race tests — non-negotiable (pipeline is concurrent)
make test-race

# 3. Lint when you touched production logic or imports
make go-lint
```

## When to go further

| You changed… | Also run |
| ------------ | -------- |
| Dependencies (`go.mod`) | `go mod tidy` + `go mod verify` + full `make test-race` |
| Registry / pipeline download / retry | unit tests for order, failures, resume; `make test-race` |
| Bundle format / load.sh / checksums | `pkg/bundle` tests; if you produce a real archive, `helmdownloader verify <path>` |
| Image extraction | tables in `pkg/images` covering string + split map forms |
| TUI | package tests + manual keypath smoke if interaction changed |
| Platform / disk space | build on unix path; keep `diskspace_other.go` compiling |
| Security-sensitive paths (auth, proxy, paths) | `make security` |

## CI parity

GitHub Actions (`.github/workflows/go.yml`):

- `go build -v ./...`
- `go vet ./...`
- `go test -race -count=1 ./...`
- `golangci-lint`
- `govulncheck`

Local shortcut: `make security` covers vet + lint + vuln.

## Definition of done

- [ ] Behavior matches request; no silent TODO left for the user.
- [ ] Table-driven tests for new logic; fakes used for helm/registry seams.
- [ ] `make test-race` green.
- [ ] Lint clean if Go sources changed.
- [ ] Config/flag/README updated if a setting was added.
- [ ] No secrets, logs, binaries, or `archives/` staged.
- [ ] Errors wrapped with `%w`.

## Smoke (optional, not required for pure unit changes)

```bash
make build
./helmdownloader -h   # or run TUI briefly if you have a terminal
# after a real download:
./helmdownloader verify archives/<bundle>.tar.gz
```

Network-dependent pipeline smoke tests skip under `-short`; CI runs full tests.
