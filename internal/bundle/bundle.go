// Package bundle assembles a chart, its values, and its image tarballs into a
// single compressed archive ready for transfer to airgapped infrastructure.
package bundle

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ImageEntry pairs an image tarball on disk with the retagged reference it
// contains.
type ImageEntry struct {
	// TarPath is the path to the saved image archive.
	TarPath string
	// SourceRef is the original image reference.
	SourceRef string
	// DestRef is the retagged reference baked into the tarball.
	DestRef string
}

// Spec describes the contents of a chart bundle.
type Spec struct {
	// ChartName is used to name the output archive.
	ChartName string
	// ChartVersion is included in the output archive name.
	ChartVersion string
	// ChartPath is the pulled chart .tgz to embed.
	ChartPath string
	// Values is the rendered default values.yaml content (optional).
	Values string
	// Images are the saved image tarballs to embed.
	Images []ImageEntry
	// OutputDir is where the bundle archive is written.
	OutputDir string
}

// Create writes the bundle archive and returns its path. The archive contains:
//
//	<chart>.tgz            the Helm chart
//	values.yaml            default chart values
//	images/<name>.tar      one tarball per image, retagged
//	images.txt             source -> dest reference manifest
func Create(spec Spec) (path string, err error) {
	if err = os.MkdirAll(spec.OutputDir, 0o755); err != nil {
		return "", err
	}
	outName := fmt.Sprintf("%s-%s-bundle.tar.gz", spec.ChartName, spec.ChartVersion)
	outPath := filepath.Join(spec.OutputDir, outName)
	out, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = out.Close()
		if err != nil {
			_ = os.Remove(outPath)
		}
	}()
	gzipWriter := gzip.NewWriter(out)
	defer func() {
		_ = gzipWriter.Close()
	}()
	tarWriter := tar.NewWriter(gzipWriter)
	defer func() {
		_ = tarWriter.Close()
	}()
	if err = writeFileFromDisk(tarWriter, spec.ChartPath, filepath.Base(spec.ChartPath)); err != nil {
		return "", err
	}
	if spec.Values != "" {
		if err = writeBytes(tarWriter, "values.yaml", []byte(spec.Values)); err != nil {
			return "", err
		}
	}
	var manifest strings.Builder
	for _, image := range spec.Images {
		name := "images/" + filepath.Base(image.TarPath)
		if err = writeFileFromDisk(tarWriter, image.TarPath, name); err != nil {
			return "", err
		}
		fmt.Fprintf(&manifest, "%s\t%s\t%s\n", image.SourceRef, image.DestRef, name)
	}
	if err = writeBytes(tarWriter, "images.txt", []byte(manifest.String())); err != nil {
		return "", err
	}
	return outPath, nil
}

// writeFileFromDisk copies the file at srcPath into the archive under name.
func writeFileFromDisk(tarWriter *tar.Writer, srcPath, name string) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	header := &tar.Header{
		Name:    name,
		Mode:    0o644,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}
	_, err = io.Copy(tarWriter, file)
	return err
}

// writeBytes writes an in-memory file into the archive under name.
func writeBytes(tarWriter *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name: name,
		Mode: 0o644,
		Size: int64(len(data)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}
	_, err := tarWriter.Write(data)
	return err
}
