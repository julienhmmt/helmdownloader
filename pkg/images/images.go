// Package images extracts container image references from rendered Helm
// manifests and computes their retagged names for a private registry.
package images

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Image is a single container image reference discovered in a chart.
type Image struct {
	// Ref is the original, fully-qualified reference, e.g.
	// "quay.io/argoproj/argocd:v3.2.6".
	Ref string
	// Selected indicates whether the user wants to include it in the bundle.
	Selected bool
}

// Extract parses one or more YAML sources (rendered manifests and/or a chart's
// values.yaml) and returns the unique, sorted set of image references found
// under any "image" key. Each source may contain multiple YAML documents.
func Extract(sources ...string) []Image {
	seen := map[string]struct{}{}
	for _, source := range sources {
		decoder := yaml.NewDecoder(strings.NewReader(source))
		for {
			var doc any
			if err := decoder.Decode(&doc); err != nil {
				break
			}
			collectImages(doc, seen)
		}
	}
	refs := make([]string, 0, len(seen))
	for ref := range seen {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	result := make([]Image, 0, len(refs))
	for _, ref := range refs {
		result = append(result, Image{Ref: ref, Selected: true})
	}
	return result
}

// collectImages walks an arbitrary YAML node, recording image references held
// by keys named "image". An image value may be a plain string
// ("nginx:1.27") or a map of split fields commonly used in chart values:
//
//	image:
//	  registry: docker.io
//	  repository: nginx
//	  tag: "1.27"
//	  digest: sha256:...
func collectImages(node any, seen map[string]struct{}) {
	switch typed := node.(type) {
	case map[string]any:
		for key, value := range typed {
			if strings.EqualFold(key, "image") {
				switch v := value.(type) {
				case string:
					if isImageRef(v) {
						seen[v] = struct{}{}
					}
				case map[string]any:
					if ref, ok := assembleImageRef(v); ok {
						seen[ref] = struct{}{}
					}
				}
			}
			collectImages(value, seen)
		}
	case []any:
		for _, item := range typed {
			collectImages(item, seen)
		}
	}
}

// assembleImageRef builds a reference from the split-field form of an image
// definition. repository is required; registry, tag, and digest are optional.
// A digest takes precedence over a tag when both are present.
func assembleImageRef(m map[string]any) (string, bool) {
	repository := scalarString(m["repository"])
	if repository == "" {
		return "", false
	}
	ref := repository
	if registry := scalarString(m["registry"]); registry != "" {
		ref = strings.TrimRight(registry, "/") + "/" + strings.TrimLeft(repository, "/")
	}
	if digest := scalarString(m["digest"]); digest != "" {
		if !strings.Contains(digest, ":") {
			digest = "sha256:" + digest
		}
		ref += "@" + digest
	} else if tag := scalarString(m["tag"]); tag != "" {
		ref += ":" + tag
	}
	if !isImageRef(ref) {
		return "", false
	}
	return ref, true
}

// scalarString renders a YAML scalar (string, int, float, bool) as a trimmed
// string. Tags are frequently unquoted numbers like 1.27 that YAML decodes as
// floats, so non-string scalars are stringified rather than ignored.
func scalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case bool:
		return fmt.Sprintf("%t", t)
	case int, int64, float64:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	default:
		return ""
	}
}

// isImageRef applies light heuristics to reject values that are clearly not
// image references (empty strings, templated leftovers, plain words).
func isImageRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.ContainsAny(ref, " \t\n{}") {
		return false
	}
	// A real reference carries a tag, a digest, or a registry path separator.
	return strings.Contains(ref, ":") || strings.Contains(ref, "@") || strings.Contains(ref, "/")
}

// defaultTag is used for the destination tag when a source reference carries no
// tag (e.g. it is pinned only by digest). A docker-style tarball must be tagged,
// and a digest cannot serve as a tag, so we fall back to "latest".
const defaultTag = "latest"

// splitRef separates an image reference into its repository name, tag, and
// digest. The tag and digest are returned without their ":"/"@" separators and
// are empty when absent. A reference may carry a tag, a digest, or both
// ("repo:tag@sha256:..."); the digest is parsed first so a registry port colon
// is not mistaken for a tag separator.
func splitRef(ref string) (repo, tag, digest string) {
	repo = strings.TrimSpace(ref)
	if at := strings.Index(repo, "@"); at >= 0 {
		digest = repo[at+1:]
		repo = repo[:at]
	}
	if colon := strings.LastIndex(repo, ":"); colon >= 0 && !strings.Contains(repo[colon+1:], "/") {
		tag = repo[colon+1:]
		repo = repo[:colon]
	}
	return repo, tag, digest
}

// normalizeName expands Docker Hub shorthand in a repository name into a
// fully-qualified name, e.g. "redis" => "docker.io/library/redis" and
// "mattermost/app" => "docker.io/mattermost/app". Names that already carry a
// registry host are returned unchanged.
func normalizeName(name string) string {
	first, _, _ := strings.Cut(name, "/")
	hasRegistry := strings.ContainsAny(first, ".:") || first == "localhost"
	if !hasRegistry {
		if !strings.Contains(name, "/") {
			name = "library/" + name
		}
		name = "docker.io/" + name
	}
	return name
}

// PullRef returns the canonical reference used to pull an image. A digest is
// preferred over a tag for precision, and the two are never combined because
// registry clients reject "repo:tag@digest". Docker Hub shorthand is expanded.
func PullRef(ref string) string {
	repo, tag, digest := splitRef(ref)
	repo = normalizeName(repo)
	switch {
	case digest != "":
		return repo + "@" + digest
	case tag != "":
		return repo + ":" + tag
	default:
		return repo
	}
}

// Retag computes the destination reference for an image when mirrored behind
// prefix. The original registry path is preserved so the layout is predictable,
// e.g. prefix="rgy01.domain.local" + "quay.io/argoproj/argocd:v3.2.6"
// => "rgy01.domain.local/quay.io/argoproj/argocd:v3.2.6".
// Docker Hub shorthand like "redis:8" is normalized to "docker.io/library/...".
//
// The destination is always a tag reference: any digest is dropped (a tarball
// cannot be tagged by digest) and references without a tag fall back to
// "latest". A "repo:tag@digest" source therefore mirrors to "prefix/repo:tag".
func Retag(ref, prefix string) string {
	repo, tag, _ := splitRef(ref)
	repo = normalizeName(repo)
	if tag == "" {
		tag = defaultTag
	}
	dest := repo + ":" + tag
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return dest
	}
	return prefix + "/" + dest
}
