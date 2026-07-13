# CLAUDE

Agent guidance for this repository lives in **[AGENTS.md](./AGENTS.md)**.

Read `AGENTS.md` first. For task-specific depth, invoke the matching skill under
`.agents/skills/*/SKILL.md` (see the skill table in AGENTS.md).

## Hard rules (summary)

1. Always run `make test-race` before considering a change done.
2. Preserve test seams: `imageSaver` and `helmClient` in `pkg/pipeline`.
3. Config is one source of truth: `config.Config` + `Default` + CLI flag + README.
4. Wrap errors: `fmt.Errorf("...: %w", err)`.
5. No Docker/daemon dependency — go-containerregistry only; helm is the only external binary.
6. Platform code uses build tags with a no-op non-unix fallback.
7. `gofmt` / `goimports` mandatory (local-prefix `github.com/julienhmmt/helmdownloader`).
8. English for code and docs; prefer small, single-purpose files.

Do not invent a new task runner, commit secrets/binaries/`archives/`/`graphify-out/`,
weaken CI security steps, or abort the whole download batch on one image failure.
