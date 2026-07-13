---
name: config
description: How to add or change configuration and CLI flags in helmdownloader. Single source of truth across config.Config, defaults, flags, and README.
triggers:
  - user
  - model
---

# Config & CLI

## Single source of truth checklist

For **every** new setting, do all four:

1. Field on `config.Config` with `yaml:"snake_case"` tag and godoc.
2. Default in `config.Default()` when a zero value is wrong.
3. CLI flag in `main.go` that overrides when set (sentinel zeros:
   empty string, `0`, or `-1` for "flag not passed" — match existing patterns).
4. Document in `README.md` (flag table + example YAML).

Optional: environment fallback only if already used for that domain
(today: `HTTP_PROXY` / `HTTPS_PROXY` when proxy unset).

**`-log-file` exception:** the flag default is `helmdownloader.log`, and merge
is `if cfg.LogFile == "" { cfg.LogFile = *logFile }`. A non-empty config
`log_file` therefore wins; the CLI cannot override it. Do not "fix" this
without an intentional sentinel redesign.

## Layout

```
pkg/config/config.go   Config, Default, Load, DefaultPath
main.go                flags, merge into cfg, stringSlice for repeatables
~/.config/helmdownloader/config.yaml   user file (DefaultPath)
```

`Load` returns defaults if the file is missing; unset YAML fields keep defaults
because unmarshaling onto a pre-filled struct.

## Repeatable flags

```go
// main.go
type stringSlice []string
func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
    *s = append(*s, v)
    return nil
}
// usage: flag.Var(&valuesFiles, "values", "...")
```

Used for `-values` and `-set`. Do not invent a second pattern.

## Existing knobs (do not rename lightly)

| Config field | Flag | Default |
| ------------ | ---- | ------- |
| RegistryPrefix | `-registry-prefix` | `""` |
| Platform | `-platform` | `linux/amd64` |
| OutputDir | `-output` | `archives` |
| WorkDir | `-work-dir` | temp |
| Concurrency | `-concurrency` | `4` |
| Retries | `-retries` | `2` |
| HTTPSProxy | `-proxy` | env fallback |
| Compression | `-compression` | `gzip` (`zstd` ok) |
| MinFreeDiskMB | `-min-free-mb` | `500` (`0` disables) |
| Resume | `-resume` | false |
| RegistryAuth | `-registry-auth` | false |
| ValuesFiles | `-values` (repeat) | |
| SetValues | `-set` (repeat) | |
| ExportImages | `-export-images` | |
| ImportImages | `-import-images` | |
| HelmBin | (config only) | `helm` |
| ArtifactHubURL | (config only) | `https://artifacthub.io` |
| SearchLimit | (config only) | `20` |
| LogLevel / LogFile / Verbose | `-log-level`, `-log-file`, `-v` | |

## Validation

- Compression: `bundle.ValidateCompression` at startup (fail fast).
- Helm: `helm.Check` after config merge.
- Disk: pipeline `checkDiskSpace` before download when `MinFreeDiskMB > 0`.

## Tests

Add/extend `pkg/config/config_test.go` for Default and Load behavior.
If a flag merges into cfg with a non-obvious zero sentinel, cover the merge
logic or document it next to the flag in `main.go`.
