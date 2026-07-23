// Package version holds build-time identity for helmdownloader.
package version

// Version is the release version. Overridden at link time via
// -ldflags "-X github.com/julienhmmt/helmdownloader/pkg/version.Version=...".
// Default "dev" marks non-release builds.
var Version = "dev"

// String returns the tool identity, e.g. "helmdownloader 0.4.0" or
// "helmdownloader dev".
func String() string {
	return "helmdownloader " + Version
}
