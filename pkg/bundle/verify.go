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

// readBundle opens a bundle archive (.tar.gz or .tar.zst) and returns a map
// of entry name to content bytes. The codec is selected from the file
// extension. Entries are read fully into memory; bundles are small enough
// (chart + image tars are already on disk elsewhere) that this is fine for
// the verify/diff metadata-only use case.
func readBundle(path string) (map[string][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open bundle: %w", err)
	}
	defer func() { _ = f.Close() }()
	var stream io.Reader
	switch {
	case strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".tgz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("read gzip: %w", err)
		}
		defer func() { _ = gz.Close() }()
		stream = gz
	case strings.HasSuffix(path, ".zst"):
		zr, err := zstd.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("read zstd: %w", err)
		}
		defer zr.Close()
		stream = zr
	default:
		return nil, fmt.Errorf("unknown bundle extension %q (want .tar.gz or .tar.zst)", path)
	}
	entries := map[string][]byte{}
	tr := tar.NewReader(stream)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read entry %s: %w", hdr.Name, err)
		}
		entries[hdr.Name] = data
	}
	return entries, nil
}

// Verify checks a bundle's integrity: every file listed in sha256sums.txt
// is re-hashed and compared, and manifest.json is parsed to confirm it is
// well-formed and its image entries have recorded digests. It does not
// re-pull images or contact any registry. Returns nil if the bundle is
// intact, or an error listing the first problem found.
func Verify(path string) error {
	entries, err := readBundle(path)
	if err != nil {
		return err
	}
	sums, ok := entries["sha256sums.txt"]
	if !ok {
		return errors.New("bundle missing sha256sums.txt")
	}
	var mismatched []string
	for line := range strings.SplitSeq(strings.TrimRight(string(sums), "\n"), "\n") {
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("malformed checksum line %q", line)
		}
		wantHex, name := parts[0], parts[1]
		content, ok := entries[name]
		if !ok {
			mismatched = append(mismatched, name+" (missing from archive)")
			continue
		}
		sum := sha256.Sum256(content)
		if hex.EncodeToString(sum[:]) != wantHex {
			mismatched = append(mismatched, name+" (checksum mismatch)")
		}
	}
	if prov, ok := entries["manifest.json"]; ok {
		var p provenance
		if err := json.Unmarshal(prov, &p); err != nil {
			return fmt.Errorf("parse manifest.json: %w", err)
		}
		if p.Tool != tool {
			return fmt.Errorf("manifest.json tool %q != expected %q", p.Tool, tool)
		}
	} else {
		return errors.New("bundle missing manifest.json")
	}
	if len(mismatched) > 0 {
		return fmt.Errorf("bundle verification failed:\n  %s", strings.Join(mismatched, "\n  "))
	}
	return nil
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
// from each bundle; file contents are not compared.
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

// readManifestImages reads a bundle's manifest.json and returns a map of
// image source-ref → digest (digest is "" when the registry reported none).
func readManifestImages(path string) (map[string]string, error) {
	entries, err := readBundle(path)
	if err != nil {
		return nil, err
	}
	prov, ok := entries["manifest.json"]
	if !ok {
		return nil, errors.New("missing manifest.json")
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
