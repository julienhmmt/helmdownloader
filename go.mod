module github.com/julienhmmt/helmdownloader

go 1.26.0

// go1.26.5 patches crypto/tls ECH privacy leak (GO-2026-5856 / CVE-2026-42505)
// and os.Root symlink escape (CVE-2026-39822). With GOTOOLCHAIN=auto, the go
// command downloads this toolchain when the host is older.
toolchain go1.26.5

require (
	charm.land/bubbles/v2 v2.2.1
	charm.land/bubbletea/v2 v2.1.0
	charm.land/lipgloss/v2 v2.0.6
	github.com/charmbracelet/x/ansi v0.11.9
	github.com/google/go-containerregistry v0.22.1
	github.com/klauspost/compress v1.20.1
	github.com/stretchr/testify v1.12.1
	golang.org/x/sync v0.24.0
	golang.org/x/sys v0.49.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/harmonica v0.2.0 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20261008173134-6b8d4baf91b4 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/docker/cli v29.7.2+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.8 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.30 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sahilm/fuzzy v0.1.3 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/xo/terminfo v1.0.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	gotest.tools/v3 v3.5.2 // indirect
)
