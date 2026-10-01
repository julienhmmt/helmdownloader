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
	imageFailures    map[string]int
	bundledVersions  []string
}

func (f *fakeRunner) Prepare(_ context.Context, pkg artifacthub.Package, version string) (pipeline.Prepared, error) {
	if err := f.failPrepare[pkg.Name]; err != nil {
		return pipeline.Prepared{}, err
	}
	f.preparedVersions = append(f.preparedVersions, pkg.Name+"@"+version)
	return pipeline.Prepared{ChartPath: pkg.Name, Images: []images.Image{{Ref: "img:1", Selected: true}, {Ref: "img:2", Selected: true}, {Ref: "img:3", Selected: true}}}, nil
}

func (f *fakeRunner) Download(_ context.Context, prepared pipeline.Prepared, refs []string, _ pipeline.ProgressFunc, _ pipeline.ByteProgressFunc) ([]bundle.ImageEntry, []pipeline.ImageFailure, error) {
	var entries = make([]bundle.ImageEntry, 0, len(refs))
	var failures []pipeline.ImageFailure
	for index, ref := range refs {
		if index < f.imageFailures[prepared.ChartPath] {
			failures = append(failures, pipeline.ImageFailure{Ref: ref, Err: fmt.Errorf("pull failed")})
		} else {
			entries = append(entries, bundle.ImageEntry{SourceRef: ref})
		}
	}
	return entries, failures, nil
}

func (f *fakeRunner) Bundle(_ pipeline.Prepared, pkg artifacthub.Package, version string, _ []bundle.ImageEntry) (string, error) {
	f.bundledVersions = append(f.bundledVersions, pkg.Name+"@"+version)
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

func TestRun_ImageCompleteness(t *testing.T) {
	for _, missing := range []int{0, 1, 3} {
		t.Run(fmt.Sprintf("missing=%d", missing), func(t *testing.T) {
			var res = fakeResolver{pkgs: map[string]artifacthub.Package{"a": {Name: "a", Version: "1"}, "b": {Name: "b", Version: "1"}}}
			var runner = &fakeRunner{imageFailures: map[string]int{"a": missing}}
			var out bytes.Buffer
			var err = run3(t, res, runner, []ChartRef{{Repo: "r", Name: "a"}, {Repo: "r", Name: "b"}}, &out)
			if (err != nil) != (missing > 0) {
				t.Fatalf("missing=%d: unexpected batch error %v", missing, err)
			}
			var completed = 2
			if missing > 0 {
				completed = 1
				if !strings.Contains(err.Error(), "incomplete") || !strings.Contains(out.String(), fmt.Sprintf("PARTIAL (%d image(s) failed) -> archives/a-1.tar.gz", missing)) {
					t.Fatalf("partial bundle not reported: err=%v output=%s", err, out.String())
				}
			}
			if !strings.Contains(out.String(), fmt.Sprintf("%d/2 chart(s) succeeded", completed)) || !strings.Contains(out.String(), "[2/2] r/b ... ok -> archives/b-1.tar.gz") {
				t.Errorf("unexpected batch output: %s", out.String())
			}
			if strings.Join(runner.bundledVersions, ",") != "a@1,b@1" {
				t.Errorf("must bundle partial results and continue in order: %v", runner.bundledVersions)
			}
		})
	}
}

// run3 adapts the unexported run to the test (keeps the fakes local).
func run3(t *testing.T, res resolver, r runner, refs []ChartRef, out *bytes.Buffer) error {
	t.Helper()
	return run(context.Background(), res, r, refs, out)
}
