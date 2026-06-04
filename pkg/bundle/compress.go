package bundle

import (
	"compress/gzip"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// compressor wraps an output writer in a compressing WriteCloser.
type compressor func(w io.Writer) (io.WriteCloser, error)

// ValidateCompression reports whether name is a supported codec, so callers can
// fail fast at startup instead of after a full download.
func ValidateCompression(name string) error {
	_, _, err := compressorFor(name)
	return err
}

// compressorFor maps a compression name to its codec and file extension.
// An empty name defaults to gzip. Unknown names are rejected.
func compressorFor(name string) (compressor, string, error) {
	switch name {
	case "", "gzip", "gz":
		return func(w io.Writer) (io.WriteCloser, error) {
			return gzip.NewWriter(w), nil
		}, "gz", nil
	case "zstd", "zst":
		return func(w io.Writer) (io.WriteCloser, error) {
			return zstd.NewWriter(w)
		}, "zst", nil
	default:
		return nil, "", fmt.Errorf("unknown compression %q (want gzip or zstd)", name)
	}
}
