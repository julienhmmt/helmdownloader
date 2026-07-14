package registry

import (
	"bytes"
	"errors"
	"io"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienhmmt/helmdownloader/pkg/log"
)

func TestCountingWriter_ReportsAfterThreshold(t *testing.T) {
	var buf bytes.Buffer
	var reports []int64
	cw := &countingWriter{
		w:     &buf,
		total: 2 * progressThreshold,
		onBytes: func(written, total int64) {
			reports = append(reports, written)
			assert.Equal(t, int64(2*progressThreshold), total)
		},
	}
	chunk := make([]byte, progressThreshold-1)
	n, err := cw.Write(chunk)
	require.NoError(t, err)
	assert.Equal(t, len(chunk), n)
	assert.Empty(t, reports)
	n, err = cw.Write([]byte{0})
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, reports, 1)
	assert.Equal(t, int64(progressThreshold), reports[0])
	n, err = cw.Write(make([]byte, progressThreshold))
	require.NoError(t, err)
	assert.Equal(t, progressThreshold, n)
	require.Len(t, reports, 2)
	assert.Equal(t, int64(2*progressThreshold), reports[1])
	assert.Equal(t, int64(2*progressThreshold), int64(buf.Len()))
}

func TestCountingWriter_NilOnBytesNoop(t *testing.T) {
	var buf bytes.Buffer
	cw := &countingWriter{w: &buf}
	n, err := cw.Write(make([]byte, progressThreshold*2))
	require.NoError(t, err)
	assert.Equal(t, progressThreshold*2, n)
	assert.Equal(t, progressThreshold*2, buf.Len())
}

func TestCountingWriter_PropagatesWriteError(t *testing.T) {
	boom := errors.New("disk full")
	cw := &countingWriter{
		w: writerFunc(func([]byte) (int, error) { return 0, boom }),
		onBytes: func(_, _ int64) {
			t.Fatal("onBytes should not be called when write fails with 0 bytes")
		},
	}
	_, err := cw.Write([]byte("x"))
	assert.ErrorIs(t, err, boom)
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestEstimateSize_EmptyOnLayerListError(t *testing.T) {
	total := estimateSize(failLayersImage{}, log.Discard())
	assert.Equal(t, int64(0), total)
}

func TestEstimateSize_EmptyOnLayerSizeError(t *testing.T) {
	total := estimateSize(failSizeImage{}, log.Discard())
	assert.Equal(t, int64(0), total)
}

func TestEstimateSize_SumsLayersAndConfig(t *testing.T) {
	total := estimateSize(sizedImage{
		layerSizes: []int64{100, 200},
		configSize: 50,
	}, log.Discard())
	assert.Equal(t, int64(350), total)
}

type failLayersImage struct{ stubImage }

func (failLayersImage) Layers() ([]v1.Layer, error) { return nil, errors.New("no layers") }

type failSizeImage struct{ stubImage }

func (failSizeImage) Layers() ([]v1.Layer, error) {
	return []v1.Layer{failSizeLayer{}}, nil
}

type sizedImage struct {
	stubImage
	layerSizes []int64
	configSize int64
}

func (s sizedImage) Layers() ([]v1.Layer, error) {
	out := make([]v1.Layer, len(s.layerSizes))
	for i, sz := range s.layerSizes {
		out[i] = fixedSizeLayer{size: sz}
	}
	return out, nil
}

func (s sizedImage) Manifest() (*v1.Manifest, error) {
	return &v1.Manifest{Config: v1.Descriptor{Size: s.configSize}}, nil
}

type stubImage struct{}

func (stubImage) Layers() ([]v1.Layer, error)         { return nil, nil }
func (stubImage) MediaType() (types.MediaType, error) { return types.DockerManifestSchema2, nil }
func (stubImage) Size() (int64, error)                { return 0, nil }
func (stubImage) ConfigName() (v1.Hash, error)        { return v1.Hash{}, nil }
func (stubImage) ConfigFile() (*v1.ConfigFile, error) { return &v1.ConfigFile{}, nil }
func (stubImage) RawConfigFile() ([]byte, error)      { return nil, nil }
func (stubImage) Digest() (v1.Hash, error)            { return v1.Hash{}, nil }
func (stubImage) Manifest() (*v1.Manifest, error)     { return &v1.Manifest{}, nil }
func (stubImage) RawManifest() ([]byte, error)        { return nil, nil }
func (stubImage) LayerByDigest(v1.Hash) (v1.Layer, error) {
	return nil, errors.New("unused")
}
func (stubImage) LayerByDiffID(v1.Hash) (v1.Layer, error) {
	return nil, errors.New("unused")
}

type failSizeLayer struct{}

func (failSizeLayer) Digest() (v1.Hash, error)           { return v1.Hash{}, nil }
func (failSizeLayer) DiffID() (v1.Hash, error)           { return v1.Hash{}, nil }
func (failSizeLayer) Compressed() (io.ReadCloser, error) { return nil, errors.New("unused") }
func (failSizeLayer) Uncompressed() (io.ReadCloser, error) {
	return nil, errors.New("unused")
}
func (failSizeLayer) Size() (int64, error)                { return 0, errors.New("no size") }
func (failSizeLayer) MediaType() (types.MediaType, error) { return types.DockerLayer, nil }

type fixedSizeLayer struct{ size int64 }

func (l fixedSizeLayer) Digest() (v1.Hash, error)           { return v1.Hash{}, nil }
func (l fixedSizeLayer) DiffID() (v1.Hash, error)           { return v1.Hash{}, nil }
func (l fixedSizeLayer) Compressed() (io.ReadCloser, error) { return nil, errors.New("unused") }
func (l fixedSizeLayer) Uncompressed() (io.ReadCloser, error) {
	return nil, errors.New("unused")
}
func (l fixedSizeLayer) Size() (int64, error)                { return l.size, nil }
func (l fixedSizeLayer) MediaType() (types.MediaType, error) { return types.DockerLayer, nil }
