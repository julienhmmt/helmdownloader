//go:build unix

package pipeline

import "golang.org/x/sys/unix"

// freeBytes returns the number of bytes available to an unprivileged user on
// the filesystem backing path.
func freeBytes(path string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil
}
