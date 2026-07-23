package batch

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/bundle"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/images"
	"github.com/julienhmmt/helmdownloader/pkg/log"
	"github.com/julienhmmt/helmdownloader/pkg/pipeline"
)

// resolver is the subset of *artifacthub.Client that batch needs: turning a
// repo/name into a pullable Package plus its version list. Tests substitute a
// fake to drive Run without network.
type resolver interface {
	Detail(ctx context.Context, repoName, name string) (artifacthub.Package, []artifacthub.Version, error)
}

// runner is the subset of *pipeline.Pipeline that batch drives per chart. Tests
// substitute a fake to exercise the loop without helm or a registry.
type runner interface {
	Prepare(ctx context.Context, pkg artifacthub.Package, version string) (pipeline.Prepared, error)
	Download(ctx context.Context, prepared pipeline.Prepared, refs []string, progress pipeline.ProgressFunc, byteProgress pipeline.ByteProgressFunc) ([]bundle.ImageEntry, []pipeline.ImageFailure, error)
	Bundle(prepared pipeline.Prepared, pkg artifacthub.Package, version string, entries []bundle.ImageEntry) (string, error)
}

// Run downloads every chart in refs sequentially, writing one status line per
// chart to out and a final summary. A single chart failure is reported and the
// batch continues; Run returns a non-nil error only if at least one chart
// failed, so callers can exit non-zero. Images are pulled in parallel within a
// chart (pipeline concurrency); charts are processed in list order.
func Run(ctx context.Context, cfg config.Config, logger *log.Logger, refs []ChartRef, out io.Writer) error {
	client, err := artifacthub.New(cfg.ArtifactHubURL, cfg.HTTPSProxy, logger)
	if err != nil {
		return fmt.Errorf("create artifacthub client: %w", err)
	}
	return run(ctx, client, pipeline.New(cfg, logger), refs, out)
}

// run is the testable core: it takes the resolver and runner as interfaces.
func run(ctx context.Context, client resolver, pl runner, refs []ChartRef, out io.Writer) error {
	total := len(refs)
	var succeeded int
	for i, ref := range refs {
		fmt.Fprintf(out, "[%d/%d] %s ... ", i+1, total, ref)
		path, imgFailed, err := processChart(ctx, client, pl, ref)
		if err != nil {
			fmt.Fprintf(out, "FAILED: %v\n", err)
			continue
		}
		succeeded++
		if imgFailed > 0 {
			fmt.Fprintf(out, "ok (%d image(s) failed) -> %s\n", imgFailed, path)
		} else {
			fmt.Fprintf(out, "ok -> %s\n", path)
		}
	}
	fmt.Fprintf(out, "%d/%d chart(s) succeeded\n", succeeded, total)
	if succeeded < total {
		return fmt.Errorf("%d of %d chart(s) failed", total-succeeded, total)
	}
	return nil
}

// processChart resolves, prepares, downloads, and bundles one chart. It returns
// the bundle path and the number of images that failed to download (the bundle
// still ships the successful ones — one image failure must not abort a chart).
func processChart(ctx context.Context, client resolver, pl runner, ref ChartRef) (path string, imgFailed int, err error) {
	pkg, version, err := resolve(ctx, client, ref)
	if err != nil {
		return "", 0, err
	}
	prep, err := pl.Prepare(ctx, pkg, version)
	if err != nil {
		return "", 0, err
	}
	// A temp work dir is not cleaned by pipeline.Bundle (only persistent ones
	// are pruned); remove it here so a long batch does not leak temp dirs.
	defer cleanup(prep)

	entries, failures, err := pl.Download(ctx, prep, imageRefs(prep.Images), nil, nil)
	if err != nil {
		return "", 0, err
	}
	path, err = pl.Bundle(prep, pkg, version, entries)
	if err != nil {
		return "", 0, err
	}
	return path, len(failures), nil
}

// resolve turns a ChartRef into a pullable Package and a concrete version. With
// no pinned version it uses the latest published version; a pinned version is
// validated against the published list so a typo fails clearly before pulling.
func resolve(ctx context.Context, client resolver, ref ChartRef) (artifacthub.Package, string, error) {
	pkg, versions, err := client.Detail(ctx, ref.Repo, ref.Name)
	if err != nil {
		return artifacthub.Package{}, "", fmt.Errorf("resolve %s/%s: %w", ref.Repo, ref.Name, err)
	}
	if ref.Version == "" {
		if pkg.Version == "" {
			return pkg, "", fmt.Errorf("no published versions for %s/%s", ref.Repo, ref.Name)
		}
		return pkg, pkg.Version, nil
	}
	if !hasVersion(versions, ref.Version) {
		return pkg, ref.Version, fmt.Errorf("version %q not found for %s/%s", ref.Version, ref.Repo, ref.Name)
	}
	return pkg, ref.Version, nil
}

// hasVersion reports whether v is among the published versions.
func hasVersion(versions []artifacthub.Version, v string) bool {
	for _, ver := range versions {
		if ver.Version == v {
			return true
		}
	}
	return false
}

// imageRefs collects every discovered image reference; batch downloads them all
// (headless mode has no interactive review step).
func imageRefs(imgs []images.Image) []string {
	refs := make([]string, len(imgs))
	for i, img := range imgs {
		refs[i] = img.Ref
	}
	return refs
}

// cleanup removes a temporary work dir created by Prepare. A user-configured
// persistent work dir is left intact so -resume can reuse its tarballs.
func cleanup(prep pipeline.Prepared) {
	if prep.TempWorkDir && prep.WorkDir != "" {
		_ = os.RemoveAll(prep.WorkDir)
	}
}
