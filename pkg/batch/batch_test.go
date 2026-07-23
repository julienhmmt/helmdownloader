package batch

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/bundle"
	"github.com/julienhmmt/helmdownloader/pkg/images"
	"github.com/julienhmmt/helmdownloader/pkg/pipeline"
)

// fakeResolver returns canned Detail results keyed by name.
type fakeResolver struct {
	pkgs     map[string]artifacthub.Package
	versions map[string][]artifacthub.Version
	err      map[string]error
}

func (f fakeResolver) Detail(_ context.Context, _, name string) (artifacthub.Package, []artifacthub.Version, error) {
	if err := f.err[name]; err != nil {
		return artifacthub.Package{}, nil, err
	}
	return f.pkgs[name], f.versions[name], nil
}

// fakeRunner records the versions it prepared and can fail a named chart at a
// chosen stage.
type fakeRunner struct {
	preparedVersions []string
	failPrepare      map[string]error
	imageFailures    int
}

func (f *fakeRunner) Prepare(_ context.Context, pkg artifacthub.Package, version string) (pipeline.Prepared, error) {
	if err := f.failPrepare[pkg.Name]; err != nil {
		return pipeline.Prepared{}, err
	}
	f.preparedVersions = append(f.preparedVersions, pkg.Name+"@"+version)
	return pipeline.Prepared{Images: []images.Image{{Ref: "img:1"}}}, nil
}

func (f *fakeRunner) Download(_ context.Context, _ pipeline.Prepared, refs []string, _ pipeline.ProgressFunc, _ pipeline.ByteProgressFunc) ([]bundle.ImageEntry, []pipeline.ImageFailure, error) {
	entries := make([]bundle.ImageEntry, 0, len(refs))
	for _, r := range refs {
		entries = append(entries, bundle.ImageEntry{SourceRef: r})
	}
	fails := make([]pipeline.ImageFailure, f.imageFailures)
	return entries, fails, nil
}

func (f *fakeRunner) Bundle(_ pipeline.Prepared, pkg artifacthub.Package, version string, _ []bundle.ImageEntry) (string, error) {
	return "archives/" + pkg.Name + "-" + version + ".tar.gz", nil
}

func TestRun(t *testing.T) {
	res := fakeResolver{
		pkgs: map[string]artifacthub.Package{
			"nginx": {Name: "nginx", Version: "2.0"},
			"redis": {Name: "redis", Version: "9.9"},
		},
		versions: map[string][]artifacthub.Version{
			"nginx": {{Version: "2.0"}, {Version: "1.5"}},
		},
		err: map[string]error{"missing": fmt.Errorf("not found")},
	}
	refs := []ChartRef{
		{Repo: "r", Name: "nginx", Version: "1.5"}, // pinned, valid
		{Repo: "r", Name: "redis"},                 // latest
		{Repo: "r", Name: "missing"},               // resolve error
		{Repo: "r", Name: "nginx", Version: "9.9"}, // pinned, invalid
	}
	run := &fakeRunner{}
	var out bytes.Buffer
	err := run3(t, res, run, refs, &out)

	if err == nil {
		t.Fatal("expected non-nil error because charts failed")
	}
	got := out.String()
	// nginx pinned to its published 1.5, redis to latest 9.9.
	wantVers := []string{"nginx@1.5", "redis@9.9"}
	if strings.Join(run.preparedVersions, ",") != strings.Join(wantVers, ",") {
		t.Errorf("prepared versions = %v, want %v", run.preparedVersions, wantVers)
	}
	for _, want := range []string{
		"[1/4] r/nginx@1.5 ... ok -> archives/nginx-1.5.tar.gz",
		"[2/4] r/redis ... ok -> archives/redis-9.9.tar.gz",
		"FAILED",                  // missing chart
		`version "9.9" not found`, // invalid pin
		"2/4 chart(s) succeeded",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n---\n%s", want, got)
		}
	}
}

func TestRunAllSucceedNoError(t *testing.T) {
	res := fakeResolver{pkgs: map[string]artifacthub.Package{"a": {Name: "a", Version: "1"}}}
	run := &fakeRunner{imageFailures: 2}
	var out bytes.Buffer
	if err := run3(t, res, run, []ChartRef{{Repo: "r", Name: "a"}}, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "ok (2 image(s) failed)") {
		t.Errorf("expected partial-image note, got: %s", out.String())
	}
}

// run3 adapts the unexported run to the test (keeps the fakes local).
func run3(t *testing.T, res resolver, r runner, refs []ChartRef, out *bytes.Buffer) error {
	t.Helper()
	return run(context.Background(), res, r, refs, out)
}
