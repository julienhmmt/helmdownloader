// Package tui implements the terminal user interface for helmdownloader.
package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

// model is the root Bubble Tea model holding all UI and domain state.
type model struct {
	cfg      config.Config
	client   *artifacthub.Client
	pipeline *pipeline.Pipeline
	styles   styleSet
	logger   *log.Logger

	state    state
	width    int
	height   int
	spinner  spinner.Model
	progress progress.Model
	search   textinput.Model
	addInput textinput.Model
	results  list.Model
	versions list.Model

	selectedPkg     artifacthub.Package
	selectedVersion string
	prepared        pipeline.Prepared
	reviewImages    []images.Image
	reviewCursor    int

	activity     chan tea.Msg
	downCurrent  int
	downTotal    int
	downRef      string
	downErr      error
	downBytesRef string
	downWritten  int64
	downSize     int64
	entries     []bundle.ImageEntry
	failures    []pipeline.ImageFailure
	bundlePath  string
	err         error
}

// New constructs the root model from cfg.
func New(cfg config.Config, logger *log.Logger) model {
	spin := spinner.New()
	spin.Spinner = spinner.Dot

	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(60),
	)

	search := textinput.New()
	search.Placeholder = "search charts (e.g. argo-cd, mattermost)…"
	search.Focus()
	search.CharLimit = 100

	add := textinput.New()
	add.Placeholder = "registry/repo:tag"
	add.CharLimit = 200

	resultsList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	resultsList.Title = "Charts"
	resultsList.SetShowHelp(false)

	versionsList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	versionsList.Title = "Versions"
	versionsList.SetShowHelp(false)

	return model{
		cfg:      cfg,
		client:   artifacthub.New(cfg.ArtifactHubURL, logger),
		pipeline: pipeline.New(cfg, logger),
		styles:   newStyles(),
		logger:   logger,
		state:    stateSearch,
		spinner:  spin,
		progress: prog,
		search:   search,
		addInput: add,
		results:  resultsList,
		versions: versionsList,
		activity: make(chan tea.Msg, 16),
	}
}

// Init starts the spinner ticking.
func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}
