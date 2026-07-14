// Package tui implements the terminal user interface for helmdownloader.
package tui

import (
	"context"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/bundle"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/images"
	"github.com/julienhmmt/helmdownloader/pkg/log"
	"github.com/julienhmmt/helmdownloader/pkg/pipeline"
)

// state enumerates the screens of the application.
type state int

const (
	stateSearch state = iota
	stateSearching
	stateResults
	stateFilterInput
	stateVersions
	statePreparing
	stateReview
	stateAddImage
	stateDownloading
	stateDownloadReview
	stateBundling
	stateDone
	stateError
)

// imageProgress is the byte-level progress of one in-flight image pull.
type imageProgress struct {
	written int64
	total   int64
}

// model is the root Bubble Tea model holding all UI and domain state.
type model struct {
	cfg      config.Config
	client   *artifacthub.Client
	pipeline *pipeline.Pipeline
	styles   styleSet
	logger   *log.Logger

	// ctx is cancelled on any quit/reset path so in-flight helm and registry
	// operations abort instead of running to completion after the user leaves.
	// cancel is the matching cancel func; it is safe to call more than once.
	ctx    context.Context
	cancel context.CancelFunc

	// bgIsDark selects light vs dark palette colors. For theme=auto it starts
	// true (dark-friendly default) and is refined by BackgroundColorMsg.
	// For theme=light|dark it is fixed at construction time.
	bgIsDark bool
	// bgKnown is true once the palette has been finalized (forced theme, or
	// terminal background detection completed / skipped).
	bgKnown bool

	state    state
	width    int
	height   int
	spinner  spinner.Model
	progress progress.Model
	search   textinput.Model
	addInput textinput.Model
	filter   textinput.Model
	results  list.Model
	versions list.Model

	// allPackages holds the raw search results; the results list shows the
	// sort/filter projection of this slice. Keeping the raw set lets the user
	// change sort/filter without re-querying ArtifactHub.
	allPackages []artifacthub.Package
	sortField   sortField
	sortDir     sortDir
	filterField filterField
	filterValue string // substring typed in the filter input

	selectedPkg     artifacthub.Package
	selectedVersion string
	prepared        pipeline.Prepared
	reviewImages    []images.Image
	reviewCursor    int
	reviewOffset    int // first visible index in reviewImages (windowed list)

	activity    chan tea.Msg
	downCurrent int
	downTotal   int
	// imageProgress tracks byte-level progress per in-flight image ref,
	// so the download screen can show all concurrent pulls advancing
	// rather than flapping between refs.
	imageProgress map[string]imageProgress
	entries       []bundle.ImageEntry
	failures      []pipeline.ImageFailure
	bundlePath    string
	err           error
	// errStep labels which async step failed (search, prepare, download, bundle)
	// so the error screen can frame the message for the user.
	errStep string
	// status is a short, ephemeral line shown under the body (not an error
	// state). Cleared on most navigation. Prefer status over stateError for
	// recoverable UX (empty results, silent no-ops, soft validation).
	status string
	// reviewWarnAck tracks whether the user already acknowledged a progressive
	// safety warning on the review screen (deprecated chart / prerelease).
	reviewWarnAck bool
}

// setStatus stores a soft status message for the next render.
func (m *model) setStatus(s string) { m.status = s }

// clearStatus clears any soft status message.
func (m *model) clearStatus() { m.status = "" }

// newModel constructs the root model from cfg.
func newModel(cfg config.Config, logger *log.Logger) model {
	theme := config.NormalizeTheme(cfg.Theme)
	forcedDark, forced := themeForcedDark(theme)
	// Auto starts dark-friendly until BackgroundColorMsg arrives.
	bgIsDark := true
	if forced {
		bgIsDark = forcedDark
	}
	styles := newStyles(bgIsDark)

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(styles.palette.accent)

	fill, empty := progressColors(bgIsDark)
	prog := progress.New(
		progress.WithColors(fill, empty),
		progress.WithWidth(60),
	)
	prog.EmptyColor = empty

	search := textinput.New()
	search.Placeholder = "search charts (e.g. argo-cd, mattermost)…"
	search.Focus()
	search.CharLimit = 100

	add := textinput.New()
	add.Placeholder = "registry/repo:tag"
	add.CharLimit = 200

	filter := textinput.New()
	filter.Placeholder = "substring (e.g. bitnami, argo)…"
	filter.CharLimit = 100

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.palette.accent).Padding(0, 1)

	resultsList := list.New(nil, newHoverDelegate(styles.palette), 0, 0)
	resultsList.Title = "Charts"
	resultsList.SetShowHelp(false)
	resultsList.Styles.Title = titleStyle

	versionsList := list.New(nil, newHoverDelegate(styles.palette), 0, 0)
	versionsList.Title = "Versions"
	versionsList.SetShowHelp(false)
	versionsList.Styles.Title = titleStyle

	ctx, cancel := context.WithCancel(context.Background())

	client, clientErr := artifacthub.New(cfg.ArtifactHubURL, cfg.HTTPSProxy, logger)
	m := model{
		cfg:           cfg,
		client:        client,
		pipeline:      pipeline.New(cfg, logger),
		styles:        styles,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		bgIsDark:      bgIsDark,
		bgKnown:       forced,
		state:         stateSearch,
		spinner:       spin,
		progress:      prog,
		search:        search,
		addInput:      add,
		filter:        filter,
		results:       resultsList,
		versions:      versionsList,
		activity:      make(chan tea.Msg, 16),
		imageProgress: map[string]imageProgress{},
		sortField:     sortStars,
		sortDir:       sortDesc,
	}
	if clientErr != nil {
		m.state = stateError
		m.err = clientErr
	}
	return m
}

// themeForcedDark reports whether theme forces a dark palette and whether the
// theme is forced (not auto).
func themeForcedDark(theme string) (isDark bool, forced bool) {
	switch config.NormalizeTheme(theme) {
	case config.ThemeLight:
		return false, true
	case config.ThemeDark:
		return true, true
	default:
		return false, false
	}
}

// applyTheme rebuilds styles and list/spinner chrome for bgIsDark.
func (m *model) applyTheme(bgIsDark bool) {
	m.bgIsDark = bgIsDark
	m.bgKnown = true
	m.styles = newStyles(bgIsDark)
	m.spinner.Style = lipgloss.NewStyle().Foreground(m.styles.palette.accent)
	fill, empty := progressColors(bgIsDark)
	m.progress.FullColor = fill
	m.progress.EmptyColor = empty
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.palette.accent).Padding(0, 1)
	m.results.SetDelegate(newHoverDelegate(m.styles.palette))
	m.results.Styles.Title = titleStyle
	m.versions.SetDelegate(newHoverDelegate(m.styles.palette))
	m.versions.Styles.Title = titleStyle
	// Re-stamp package item palettes so list meta colors match.
	if len(m.allPackages) > 0 {
		m.refreshResults()
	}
}

// toggleTheme flips light/dark, forces the matching theme on cfg (so View paints
// a terminal background and auto detection no longer overrides the choice), and
// surfaces a short status line so the change is obvious.
func (m *model) toggleTheme() {
	nextDark := !m.bgIsDark
	if nextDark {
		m.cfg.Theme = config.ThemeDark
	} else {
		m.cfg.Theme = config.ThemeLight
	}
	m.applyTheme(nextDark)
	if nextDark {
		m.setStatus("Theme: dark")
	} else {
		m.setStatus("Theme: light")
	}
}

// Init starts the spinner and, for theme=auto, requests the terminal background.
func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if config.NormalizeTheme(m.cfg.Theme) == config.ThemeAuto {
		cmds = append(cmds, tea.RequestBackgroundColor)
	}
	return tea.Batch(cmds...)
}
