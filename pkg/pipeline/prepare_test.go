package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/helm"
	"github.com/julienhmmt/helmdownloader/pkg/log"
)

// fakeHelm implements helmClient for Prepare tests. Each *Err field forces the
// matching method to fail; the *Out fields control successful returns. callXxx
// fields record invocations for assertion.
type fakeHelm struct {
	mu sync.Mutex

	pullErr       error
	showValuesErr error
	templateErr   error
	subchartErr   error

	pullCalls       []pullCall
	showValuesCalls []string
	templateCalls   []string
	subchartCalls   []string

	valuesOut   string
	templateOut string
	subchartOut []string
}

type pullCall struct {
	name, repoURL, version, destDir string
	oci                             bool
}

func (f *fakeHelm) Pull(_ context.Context, name, repoURL, version, destDir string, oci bool) (helm.PullResult, error) {
	f.mu.Lock()
	f.pullCalls = append(f.pullCalls, pullCall{name, repoURL, version, destDir, oci})
	f.mu.Unlock()
	if f.pullErr != nil {
		return helm.PullResult{}, f.pullErr
	}
	chartPath := filepath.Join(destDir, name+"-"+version+".tgz")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return helm.PullResult{}, err
	}
	if err := os.WriteFile(chartPath, []byte("fake-chart"), 0o644); err != nil {
		return helm.PullResult{}, err
	}
	return helm.PullResult{ChartPath: chartPath, Dir: destDir}, nil
}

func (f *fakeHelm) ShowValues(_ context.Context, chartPath string) (string, error) {
	f.mu.Lock()
	f.showValuesCalls = append(f.showValuesCalls, chartPath)
	f.mu.Unlock()
	if f.showValuesErr != nil {
		return "", f.showValuesErr
	}
	return f.valuesOut, nil
}

func (f *fakeHelm) Template(_ context.Context, chartPath string, _ ...helm.TemplateOption) (string, error) {
	f.mu.Lock()
	f.templateCalls = append(f.templateCalls, chartPath)
	f.mu.Unlock()
	if f.templateErr != nil {
		return "", f.templateErr
	}
	return f.templateOut, nil
}

func (f *fakeHelm) SubchartValues(chartPath string) ([]string, error) {
	f.mu.Lock()
	f.subchartCalls = append(f.subchartCalls, chartPath)
	f.mu.Unlock()
	if f.subchartErr != nil {
		return nil, f.subchartErr
	}
	return f.subchartOut, nil
}

func testPkg() artifacthub.Package {
	return artifacthub.Package{Name: "argo-cd", RepoURL: "https://charts.argoproj.io"}
}

func TestPrepare_HappyPath(t *testing.T) {
	h := &fakeHelm{
		valuesOut:   "image: redis:7\n",
		templateOut: "image: nginx:1.25\n",
	}
	cfg := config.Default()
	cfg.WorkDir = "" // force a temp work dir
	pl := newForTest(cfg, log.Discard(), h, &fakeSaver{})

	prep, err := pl.Prepare(context.Background(), testPkg(), "1.0.0")
	require.NoError(t, err)

	got := make([]string, len(prep.Images))
	for i, img := range prep.Images {
		got[i] = img.Ref
	}
	assert.ElementsMatch(t, []string{"nginx:1.25", "redis:7"}, got)
	assert.True(t, prep.TempWorkDir, "temp work dir should be flagged")
	assert.Len(t, h.pullCalls, 1)
	assert.Len(t, h.showValuesCalls, 1)
	assert.Len(t, h.templateCalls, 1)
}

func TestPrepare_PullError(t *testing.T) {
	h := &fakeHelm{pullErr: errors.New("pull boom")}
	cfg := config.Default()
	cfg.WorkDir = ""
	pl := newForTest(cfg, log.Discard(), h, &fakeSaver{})

	_, err := pl.Prepare(context.Background(), testPkg(), "1.0.0")
	require.Error(t, err)
}

func TestPrepare_ShowValuesError(t *testing.T) {
	h := &fakeHelm{showValuesErr: errors.New("values boom")}
	pl := newForTest(config.Default(), log.Discard(), h, &fakeSaver{})
	_, err := pl.Prepare(context.Background(), testPkg(), "1.0.0")
	require.Error(t, err)
}

func TestPrepare_TemplateError(t *testing.T) {
	h := &fakeHelm{templateErr: errors.New("template boom")}
	pl := newForTest(config.Default(), log.Discard(), h, &fakeSaver{})
	_, err := pl.Prepare(context.Background(), testPkg(), "1.0.0")
	require.Error(t, err)
}

func TestPrepare_SubchartErrorNonFatal(t *testing.T) {
	h := &fakeHelm{
		valuesOut:   "image: redis:7\n",
		templateOut: "image: nginx:1.25\n",
		subchartErr: errors.New("subchart boom"),
	}
	pl := newForTest(config.Default(), log.Discard(), h, &fakeSaver{})
	prep, err := pl.Prepare(context.Background(), testPkg(), "1.0.0")
	require.NoError(t, err, "subchart scan failure must not abort Prepare")
	assert.Len(t, prep.Images, 2)
}

func TestPrepare_PersistentWorkDirNotCleanedOnError(t *testing.T) {
	work := t.TempDir()
	h := &fakeHelm{pullErr: errors.New("pull boom")}
	cfg := config.Default()
	cfg.WorkDir = work
	pl := newForTest(cfg, log.Discard(), h, &fakeSaver{})

	_, err := pl.Prepare(context.Background(), testPkg(), "1.0.0")
	require.Error(t, err)
	_, statErr := os.Stat(work)
	assert.NoError(t, statErr, "persistent work dir must survive a Prepare error")
}

func TestPrepare_ValuesFilesAndSetValuesFlowThrough(t *testing.T) {
	h := &fakeHelm{
		valuesOut:   "image: redis:7\n",
		templateOut: "image: nginx:1.25\n",
	}
	cfg := config.Default()
	cfg.ValuesFiles = []string{"override.yaml"}
	cfg.SetValues = []string{"key=value"}
	pl := newForTest(cfg, log.Discard(), h, &fakeSaver{})

	_, err := pl.Prepare(context.Background(), testPkg(), "1.0.0")
	require.NoError(t, err)
	assert.Len(t, h.templateCalls, 1, "Template should be called once with the opts applied")
}

func TestPrepare_UsesConfiguredTempDir(t *testing.T) {
	tempBase := t.TempDir()
	h := &fakeHelm{
		valuesOut:   "image: redis:7\n",
		templateOut: "image: nginx:1.25\n",
	}
	cfg := config.Default()
	cfg.WorkDir = ""
	cfg.TempDir = tempBase
	pl := newForTest(cfg, log.Discard(), h, &fakeSaver{})

	prep, err := pl.Prepare(context.Background(), testPkg(), "1.0.0")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(prep.WorkDir, tempBase), "work dir should be under configured temp dir")
	assert.True(t, prep.TempWorkDir)
}
