---
name: obsidian-vault
description: Build or refresh the HelmDownloader Obsidian brain - the notes that explain what HelmDownloader is, how it is built, and how it runs. Use when the user says "update the vault", "refresh the brain", "obsidian", "regenerate the HelmDownloader notes", or after a large change that makes the vault stale (new package, new flag, new screen, new config key, version bump).
---

# HelmDownloader Obsidian Vault

Generate a set of linked Markdown notes that model this project, so a human can
understand and operate HelmDownloader without reading the code.

## Where it lives

The vault is **outside this repo**:

```text
/Users/jh/Library/Mobile Documents/iCloud~md~obsidian/Documents/JhoBoxes/Projets/Helmdownloader
```

> [!important] Shell cannot read iCloud, but it can write
> iCloud Drive under `~/Library/Mobile Documents` is protected by macOS TCC. A
> shell gets `Operation not permitted` on `ls` (and `find_file_by_name` / grep
> return nothing), but `mkdir` / `touch` / `cp` into the path succeed, and the
> file read/write/edit tools reach it fine.
>
> Practical recipe: author in a local staging dir (for example `/tmp/hdvault`),
> run the verification there, then `cp -r /tmp/hdvault/. "<vault path>/"`. Do not
> burn time debugging the shell read failure, and do not stage into the repo.

## Do not invent content

Every note is derived from something in the repo. Read first, write second.

| Note group | Primary sources |
| --- | --- |
| Home | `git describe --tags --always`, `git rev-list --count HEAD`, `README.md`, `go.mod` |
| Product | `README.md`, `AGENTS.md`, `config.example.yaml`, `pkg/images/images.go`, `pkg/batch/list.go` |
| Architecture | `main.go`, `pkg/*/*.go`, `.agents/skills/architecture/SKILL.md` |
| Bundle Format | `pkg/bundle/{bundle,provenance,sbom,verify,compress}.go` |
| Pipeline Lifecycle | `pkg/pipeline/pipeline.go`, `pkg/pipeline/diskspace_*.go` |
| TUI | `internal/tui/*.go`, `.agents/skills/tui/SKILL.md` |
| CLI and Configuration | `main.go`, `pkg/config/config.go`, `config.example.yaml`, `README.md` |
| Operations | `Makefile`, `.github/workflows/*.yml`, `.goreleaser.yaml`, `.golangci.yml`, `docker-compose.sonarqube.yml`, `tools/`, `sonar-projects/` |
| History | `git log`, `git tag`, `RELEASE_NOTES_v*.md`, `plans/README.md` |
| PVMSS | the PVMSS vault and `/Users/jh/git/gh/pvmss/AGENTS.md` |

**Read the code, not the docs about the code.** `README.md` is accurate about
user-facing behaviour; `AGENTS.md` and `.agents/skills/` are accurate about
conventions; internal prose can lag. Every drift found goes into
`Reference/Open Questions and Drift.md` with the exact command to re-check it.

## Structure

```text
Helmdownloader Home.md     <- the entry point, a map of content
Product/                   Product Overview, Feature Inventory, Image Discovery,
                           Airgap Workflow, Batch Mode, Security Review Workflow
Architecture/              Architecture Overview, System Diagrams, Pipeline Lifecycle,
                           Package Map, Bundle Format, Glossary
TUI/                       TUI Overview, Screens and Keys, Themes
CLI/                       CLI Reference, Configuration
Operations/                Build and Test, CI and Release, SonarQube, Troubleshooting
Reference/                 Project History, PVMSS, Reading the Codebase Cheaply,
                           Open Questions and Drift, Vault Maintenance
```

Create the folders only for notes that exist. A light folder tree plus a strong
`Helmdownloader Home.md` index is the balance: Obsidian's file explorer stays
navigable, and the graph view still works because every note links out.

**Every note basename must be unique** (Obsidian resolves `[[Name]]` by basename).
That is why there is `Product Overview` and `Architecture Overview`, not two
`Overview` notes.

## Conventions

- **Title case** for note names. Spaces, not hyphens.
- **`[[Wikilinks]]`** for every cross-reference. No relative Markdown links.
- **Every note ends with a `## Related` section** linking its neighbours.
- **Every index-style list is a wikilink list.**
- **Diagrams are text first.** ASCII in a fenced block for the version that
  renders anywhere, Mermaid in a ```mermaid block for the version Obsidian
  renders. `Architecture/System Diagrams.md` is the model: context, packages and
  data flow, pipeline sequence, parallel download, TUI state machine, bundle
  layout, offline verification.
- **Callouts** (`> [!info]`, `> [!warning]`, `> [!tip]`) for the non-obvious fact
  on a page.
- **No em-dash (U+2014) in the vault.** Use a spaced ASCII hyphen ` - `. The repo
  docs are inconsistent on this; the vault is not.
- **English** for prose. Keep product vocabulary as-is (`bundle`, `airgap`,
  `retag`, `digest`, `sidecar`).
- Tables over prose for anything enumerable: flags, config keys, screens, exit
  codes, bundle members, make targets.

## The facts worth carrying into every refresh

These are the ones a reader needs and that are easy to lose:

- **One Go module** `github.com/julienhmmt/helmdownloader` (Go 1.26, `toolchain
  go1.26.5`) covering `main`, `pkg/*` and `internal/tui`.
- **Daemonless by design.** No Docker client or daemon. The only external binary
  is `helm`. Image I/O is `go-containerregistry` only.
- **One chart, one bundle.** `<chart>-<version>-bundle.tar.gz` (or `.tar.zst`).
  A TUI session can chain charts, but the artifacts stay separate.
- **Digests are pinned, tarballs stay tagged.** `registry.Save` returns the
  manifest digest, recorded in `images.txt`, `manifest.json` and the `.digest`
  sidecar. A docker tar cannot be digest-tagged, so the digest is for
  verification, not for load identity.
- **One image failure never aborts the batch.** `Download` returns
  `[]bundle.ImageEntry` plus `[]ImageFailure`, written into fixed slots so results
  stay in **input order**.
- **`load.sh` fails closed.** It verifies `sha256sums.txt` (which covers
  `load.sh` itself) and exits 1 if neither `sha256sum` nor `shasum` exists.
- **Helm pulls are hermetic.** `HELM_REPOSITORY_CONFIG` and
  `HELM_REPOSITORY_CACHE` are scoped under the work dir; the global
  `~/.config/helm` is never read, and `helm repo update` is never needed.
- **Image discovery is best-effort.** Sources scanned in order: rendered
  manifests, top-level `values.yaml`, every `charts/*/values.yaml`. Images behind
  a false default condition need `-values` / `-set`. Manual `a` on Review and
  `-import-images` are the escape hatches.
- **`-resume` is integrity-gated.** Reuse needs a non-empty tarball, a `.digest`
  sidecar, a matching `.sha256` sidecar, and a well-formed tar trailer. Older work
  dirs re-pull once, safely.
- **`Bundle` has no context.** Cancelling mid-write risks a partial archive, so
  `Esc` is a no-op during bundling; only `Ctrl+C` quits.
- **Config is one source of truth**: `config.Config` field + `config.Default` +
  CLI flag in `main.go` + README row. Repeatable flags use `stringSlice`.
- **The `log_file` merge is intentionally asymmetric**: a non-empty YAML
  `log_file` wins over the `-log-file` flag. Do not "fix" it silently.
- **Test seams are load-bearing**: `imageSaver` and `helmClient` in
  `pkg/pipeline`, plus `retryBaseDelay` and `maxMetadataFileSize`.
- **`make test-race` is mandatory** before a change is done (the pipeline is
  concurrent).
- **PVMSS is the author's other project**, a tooling reference, not a dependency.
  It also runs SonarQube on port 9000, which collides with this repo's local
  SonarQube stack.

## Refreshing an existing vault

1. Read `Reference/Open Questions and Drift.md` first and re-run its verification
   commands. Resolve or update each item.
2. Diff the sources: package list (`ls pkg/*/ internal/`), flags
   (`grep -n 'flag\.' main.go`), config keys
   (`grep -n 'yaml:"' pkg/config/config.go`), make targets
   (`grep -n '^[a-z-]*:' Makefile`), screens
   (`grep -n 'state[A-Z]' internal/tui/model.go`), version
   (`git describe --tags --always`).
3. Update only the notes whose sources moved. Append to
   `Reference/Project History.md`; do not rewrite history.
4. Report what changed and what you could not verify.

## Verify before handing over

Run from the staging copy (the shell cannot read the iCloud path):

```bash
cd /path/to/staging/vault

# every wikilink resolves to a real note.
# NOTE: use `while read`, not `for x in $(...)` - note names contain spaces,
# and a for-loop silently splits "Bundle Format" into two words
# and reports two bogus dangling links.
grep -rho '\[\[[^]]*\]\]' . | sed 's/\[\[//;s/\]\]//;s/|.*//' | sort -u |
while IFS= read -r l; do
  [ -n "$(find . -name "$l.md" -print -quit)" ] || echo "DANGLING: $l"
done

# no em-dashes
grep -rn $'\u2014' . || echo "clean"

# mermaid and code fences balanced (opening count must equal closing count)
grep -rc '^```' . | awk -F: '{s+=$2} END {print "fence lines:", s, "(must be even)"}'

# markdownlint - MD013 (line length) off, wide tables are intentional.
# Write the config to a real file: markdownlint-cli2 rejects process
# substitution (`--config <(echo ...)`) with "Unable to use configuration file".
printf '{"MD013": false}\n' > /tmp/mdlint.json
npx --yes markdownlint-cli2 --config /tmp/mdlint.json "**/*.md"
```

### Three lint traps this vault has already hit

1. **Unescaped `|` inside a table row.** A cell containing a shell pipe (for
   example `zstd -d < file | tar xf -`) splits the row into extra cells
   (MD056/MD060). Escape it: `` `zstd -d < file \| tar xf -` ``. Same for
   `GET\|POST` and `?level=info\|debug`.
2. **Bare URLs** (MD034). Wrap them: `<http://localhost:9000>`. URLs inside
   backticks are fine.
3. **Mixed emphasis** (MD049, style `consistent`). The *first* emphasis in a file
   sets the style for the whole file. Pick one: `_x_` or `*x*`. This vault uses
   `*x*` in prose.
4. **MD041 on a frontmatter-only first block.** markdownlint's default
   `front_matter_title` pattern is `^title\s*[:=]`, so YAML frontmatter without a
   `title:` key is not recognised as front matter and MD041 fires. Give the Home
   note a `title:` key (it also mirrors what Obsidian Bases and Dataview read)
   rather than disabling MD041.

Fenced code blocks are invisible to these checks - scan fence-aware, or you will
"fix" ASCII diagrams.

Then copy the staging dir into the iCloud vault path with `cp -r`, and tell the
user the vault path and what you changed.

## Related skills

- `/architecture` for the code-side map the vault mirrors
- `/tui` for screen and key facts
- `/config` for flag and config facts
