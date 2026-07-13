---
name: tui
description: Bubble Tea TUI patterns for helmdownloader — screen states, model/update/view split, messages, concurrent download progress. Invoke when editing internal/tui.
triggers:
  - user
  - model
---

# TUI (`internal/tui`)

Charm Bubble Tea **v2** (`charm.land/bubbletea/v2`, bubbles, lipgloss).

## File split (follow this)

| File | Role |
| ---- | ---- |
| `model.go` | Root model, `state` enum, constructors |
| `update.go` | `Update(msg)` — state transitions |
| `view.go` | Rendering only |
| `commands.go` | `tea.Cmd` factories (search, prepare, download, bundle) |
| `messages.go` | Message types from async work |
| `items.go` | list.Item implementations for charts/versions |
| `imagelist.go` | Review-screen image list helpers |
| `sortfilter.go` | Results sort/filter projection (client-side only) |
| `styles.go` / `chrome.go` | Styles and chrome |
| `convert.go` | Domain ↔ UI conversions |
| `run.go` | `Run(cfg, logger)` entry |

Keep pure logic out of `View`. Keep side effects in `Cmd`s, not `Update` bodies
beyond launching cmds and assigning fields.

## Screen states

```
stateSearch → stateSearching → stateResults → stateFilterInput
  → stateVersions → statePreparing → stateReview → stateAddImage
  → stateDownloading → stateDownloadReview → stateBundling
  → stateDone | stateError
```

Adding a screen: new `state` constant → keys in `update.go` → view branch →
messages/cmds if async.

## Keys (user-facing; keep in sync with README)

| Screen | Important keys |
| ------ | -------------- |
| Search | Enter search, Esc quit |
| Results | Enter select, `/` fuzzy, `s` sort field, `o` sort dir, `f` filter field, `F` type filter, Tab cycle values |
| Versions | Enter, `/`, Esc |
| Review | Space toggle, `a` add, `d` delete, Enter download, Esc back |
| Done | `n` new, `q` quit |

## Async work

- Commands push progress on `model.activity` channel or via tea messages.
- Download shows **per-image** byte progress in `imageProgress map[string]imageProgress`
  so concurrent pulls all advance (do not flapper a single progress bar across refs).
- Root `ctx` / `cancel` on model: cancel on quit and on "new bundle" reset so
  helm/registry work stops.

## Sort / filter

- Operate on `allPackages` already fetched — **no** extra ArtifactHub calls when
  sort/filter changes.
- Default sort: stars, descending.

## Testing TUI

- Prefer testing pure helpers (`sortfilter`, `imagelist`, item convert) with tables.
- Message-handling tests can drive `Update` with crafted messages without a full program.
- Avoid brittle full-string snapshot tests of View unless they assert structure
  (error state, empty list) not pixel-perfect chrome.

## Do not

- Call network/helm/registry directly from `View`.
- Block in `Update` — always return a `Cmd` for slow work.
- Leak goroutines: honor `ctx` and cancel paths.
