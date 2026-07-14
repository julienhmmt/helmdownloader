package bundle

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// openBundleStream opens a bundle archive (.tar.gz or .tar.zst) and returns a
// reader over the decompressed tar stream plus a cleanup function that closes
// the underlying file and decompressor. The codec is selected from the file
// extension. Callers iterate the tar stream once; entries are not buffered, so
// memory stays bounded regardless of bundle size.
func openBundleStream(path string) (io.Reader, func(), error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open bundle: %w", err)
	}
	var stream io.Reader
	var (
		gz  *gzip.Reader
		zr  *zstd.Decoder
		cln = func() { _ = f.Close() }
	)
	switch {
	case strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".tgz"):
		gz, err = gzip.NewReader(f)
		if err != nil {
			cln()
			return nil, nil, fmt.Errorf("read gzip: %w", err)
		}
		stream = gz
		cln = func() { _ = gz.Close(); _ = f.Close() }
	case strings.HasSuffix(path, ".zst"):
		zr, err = zstd.NewReader(f)
		if err != nil {
			cln()
			return nil, nil, fmt.Errorf("read zstd: %w", err)
		}
		stream = zr
		cln = func() { zr.Close(); _ = f.Close() }
	default:
		cln()
		return nil, nil, fmt.Errorf("unknown bundle extension %q (want .tar.gz or .tar.zst)", path)
	}
	return stream, cln, nil
}

// maxMetadataFileSize caps sha256sums.txt / manifest.json reads so a hostile
// archive cannot exhaust memory during verify/diff. Real bundles stay well below.
// Mutable var so tests can lower the limit without allocating multi-MiB fixtures.
var maxMetadataFileSize int64 = 4 << 20 // 4 MiB

// readCapped reads from r until EOF or limit+1 bytes. Returns an error if the
// entry exceeds limit so callers can fail closed without buffering unbounded data.
func readCapped(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("metadata entry exceeds %d bytes", limit)
	}
	return data, nil
}

// Verify checks a bundle's integrity: every file listed in sha256sums.txt
// (including load.sh) is re-hashed and compared, and manifest.json is parsed
// to confirm it is well-formed, its tool field matches, and every image entry
// has a non-empty recorded digest (not "" or "-"). It does not re-pull images
// or contact any registry. Returns nil if the bundle is intact, or an error
// listing the first problem found.
//
// The bundle is streamed in a single pass: each regular tar entry is hashed
// on the fly (only its 64-byte hex digest is kept, never its contents), so
// memory stays bounded regardless of bundle size. sha256sums.txt and
// manifest.json are small metadata files held in memory for parsing, capped
// by maxMetadataFileSize.
func Verify(path string) error {
	stream, cln, err := openBundleStream(path)
	if err != nil {
		return err
	}
	defer cln()
	// digests maps every regular entry name to its sha256 hex digest. Only the
	// digest is retained, so a multi-GB image tar costs 64 bytes here.
	digests := map[string]string{}
	var sumsBytes, manifestBytes []byte
	tr := tar.NewReader(stream)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		switch hdr.Name {
		case "sha256sums.txt":
			data, err := readCapped(tr, maxMetadataFileSize)
			if err != nil {
				return fmt.Errorf("read sha256sums.txt: %w", err)
			}
			sumsBytes = data
			digests[hdr.Name] = hexSha256(data)
		case "manifest.json":
			data, err := readCapped(tr, maxMetadataFileSize)
			if err != nil {
				return fmt.Errorf("read manifest.json: %w", err)
			}
			manifestBytes = data
			digests[hdr.Name] = hexSha256(data)
		default:
			h := sha256.New()
			if _, err := io.Copy(h, tr); err != nil {
				return fmt.Errorf("read entry %s: %w", hdr.Name, err)
			}
			digests[hdr.Name] = hex.EncodeToString(h.Sum(nil))
		}
	}
	if sumsBytes == nil {
		return errors.New("bundle missing sha256sums.txt")
	}
	var mismatched []string
	for line := range strings.SplitSeq(strings.TrimRight(string(sumsBytes), "\n"), "\n") {
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("malformed checksum line %q", line)
		}
		wantHex, name := parts[0], parts[1]
		gotHex, ok := digests[name]
		if !ok {
			mismatched = append(mismatched, name+" (missing from archive)")
			continue
		}
		if gotHex != wantHex {
			mismatched = append(mismatched, name+" (checksum mismatch)")
		}
	}
	if manifestBytes == nil {
		return errors.New("bundle missing manifest.json")
	}
	var p provenance
	if err := json.Unmarshal(manifestBytes, &p); err != nil {
		return fmt.Errorf("parse manifest.json: %w", err)
	}
	if p.Tool != tool {
		return fmt.Errorf("manifest.json tool %q != expected %q", p.Tool, tool)
	}
	for i, img := range p.Images {
		if !digestRecorded(img.Digest) {
			return fmt.Errorf("manifest.json images[%d] %q missing digest", i, img.Source)
		}
	}
	if len(mismatched) > 0 {
		return fmt.Errorf("bundle verification failed:\n  %s", strings.Join(mismatched, "\n  "))
	}
	return nil
}

// digestRecorded reports whether d is a usable image digest for offline verify.
// Empty strings and the images.txt placeholder "-" are rejected.
func digestRecorded(d string) bool {
	d = strings.TrimSpace(d)
	return d != "" && d != "-"
}

// hexSha256 returns the hex-encoded sha256 of data.
func hexSha256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// DiffResult describes the image-level differences between two bundles.
type DiffResult struct {
	Added   []string     // refs in b but not in a
	Removed []string     // refs in a but not in b
	Changed []DiffChange // same ref, different digest
}

// DiffChange records a digest change for an image present in both bundles.
type DiffChange struct {
	Ref        string
	FromDigest string // empty if a had no digest
	ToDigest   string // empty if b has no digest
}

// Diff compares the image sets of two bundles (by source reference and
// pinned digest) and returns the differences. It reads only manifest.json
// from each bundle by streaming the archive and stopping as soon as
// manifest.json is found; image tar contents are never read into memory.
func Diff(aPath, bPath string) (DiffResult, error) {
	aImages, err := readManifestImages(aPath)
	if err != nil {
		return DiffResult{}, fmt.Errorf("read %s: %w", aPath, err)
	}
	bImages, err := readManifestImages(bPath)
	if err != nil {
		return DiffResult{}, fmt.Errorf("read %s: %w", bPath, err)
	}
	var result DiffResult
	for ref, d := range bImages {
		if aD, ok := aImages[ref]; !ok {
			result.Added = append(result.Added, ref)
		} else if aD != d {
			result.Changed = append(result.Changed, DiffChange{Ref: ref, FromDigest: aD, ToDigest: d})
		}
	}
	for ref := range aImages {
		if _, ok := bImages[ref]; !ok {
			result.Removed = append(result.Removed, ref)
		}
	}
	sort.Strings(result.Added)
	sort.Strings(result.Removed)
	sort.Slice(result.Changed, func(i, j int) bool { return result.Changed[i].Ref < result.Changed[j].Ref })
	return result, nil
}

// readManifestImages streams a bundle and returns a map of image source-ref →
// digest (digest is "" when the registry reported none), reading only
// manifest.json. Image tar entries are skipped without buffering.
func readManifestImages(path string) (map[string]string, error) {
	stream, cln, err := openBundleStream(path)
	if err != nil {
		return nil, err
	}
	defer cln()
	tr := tar.NewReader(stream)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || hdr.Name != "manifest.json" {
			continue
		}
		prov, err := readCapped(tr, maxMetadataFileSize)
		if err != nil {
			return nil, fmt.Errorf("read manifest.json: %w", err)
		}
		var p provenance
		if err := json.Unmarshal(prov, &p); err != nil {
			return nil, fmt.Errorf("parse manifest.json: %w", err)
		}
		m := make(map[string]string, len(p.Images))
		for _, img := range p.Images {
			m[img.Source] = img.Digest
		}
		return m, nil
	}
	return nil, errors.New("missing manifest.json")
}
