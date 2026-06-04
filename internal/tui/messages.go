package tui

import (
	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/bundle"
	"github.com/julienhmmt/helmdownloader/pkg/pipeline"
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
	err     error
}

// byteProgressMsg reports byte-level progress for the image currently pulling.
type byteProgressMsg struct {
	ref     string
	written int64
	total   int64
}

// downloadDoneMsg carries the result of a download pass: the entries that
// succeeded and the references that failed.
type downloadDoneMsg struct {
	entries  []bundle.ImageEntry
	failures []pipeline.ImageFailure
}

// doneMsg signals that the bundle has been written.
type doneMsg struct {
	bundlePath string
}

// errMsg carries a failure from any asynchronous step.
type errMsg struct {
	err error
}
