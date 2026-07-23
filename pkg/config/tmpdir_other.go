//go:build !unix

package config

// fallbackTempDirs returns platform-specific temporary directories. Non-Unix
// builds have no extra well-known paths beyond os.TempDir().
func fallbackTempDirs() []string {
	return nil
}
