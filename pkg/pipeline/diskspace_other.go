//go:build !unix

package pipeline

// freeBytes is unsupported on non-unix platforms; it reports 0 with no error so
// the preflight check is skipped rather than blocking the download.
func freeBytes(string) (uint64, error) {
	return 0, nil
}
