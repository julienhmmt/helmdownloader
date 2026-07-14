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
	// status is a short, ephemeral line for soft feedback (deprecation confirm,
	// recoverable UX). Not an error state.
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
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(colorAccent)

	prog := progress.New(
		progress.WithColors(lipgloss.Color("#B8902E"), lipgloss.Color("#E6C766")),
		progress.WithWidth(60),
	)

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

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Padding(0, 1)

	chartDelegate := list.NewDefaultDelegate()
	chartDelegate.Styles = chartDelegateStyles()
	chartDelegate.SetSpacing(1)
	resultsList := list.New(nil, chartDelegate, 0, 0)
	resultsList.Title = "Charts"
	resultsList.SetShowHelp(false)
	resultsList.Styles.Title = titleStyle

	versionDelegate := list.NewDefaultDelegate()
	versionDelegate.Styles = chartDelegateStyles()
	versionDelegate.SetSpacing(1)
	versionsList := list.New(nil, versionDelegate, 0, 0)
	versionsList.Title = "Versions"
	versionsList.SetShowHelp(false)
	versionsList.Styles.Title = titleStyle

	ctx, cancel := context.WithCancel(context.Background())

	return model{
		cfg:           cfg,
		client:        artifacthub.New(cfg.ArtifactHubURL, logger),
		pipeline:      pipeline.New(cfg, logger),
		styles:        newStyles(),
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
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
}

// Init starts the spinner ticking.
func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}
