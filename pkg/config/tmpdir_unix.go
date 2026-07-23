//go:build unix

package config

// fallbackTempDirs returns Unix-specific temporary directories to try when
// the configured or system temp dir is not writable.
func fallbackTempDirs() []string {
	return []string{"/tmp", "/var/tmp"}
}
