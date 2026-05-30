package tui

import (
	"github.com/julienhmmt/helmdownloader/internal/artifacthub"
	"github.com/julienhmmt/helmdownloader/internal/pipeline"
)

// searchResultMsg carries the packages returned by an ArtifactHub search.
type searchResultMsg struct {
	packages []artifacthub.Package
}

// versionsMsg carries the versions available for the selected package.
type versionsMsg struct {
	versions []artifacthub.Version
}

// preparedMsg carries the result of pulling and rendering a chart.
type preparedMsg struct {
	prepared pipeline.Prepared
}

// progressMsg reports the download status of a single image.
type progressMsg struct {
	current int
	total   int
	ref     string
	failed  bool
}

// doneMsg signals that the bundle has been written.
type doneMsg struct {
	bundlePath string
}

// errMsg carries a failure from any asynchronous step.
type errMsg struct {
	err error
}
