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

// Retag computes the destination reference for an image when mirrored behind
// prefix. The original registry path is preserved so the layout is predictable,
// e.g. prefix="rgy01.domain.local" + "quay.io/argoproj/argocd:v3.2.6"
// => "rgy01.domain.local/quay.io/argoproj/argocd:v3.2.6".
// Docker Hub shorthand like "redis:8" is normalized to "docker.io/library/...".
func Retag(ref, prefix string) string {
	normalized := normalizeRef(ref)
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return normalized
	}
	return prefix + "/" + normalized
}

// normalizeRef expands Docker Hub shorthand into a fully-qualified reference.
func normalizeRef(ref string) string {
	name := ref
	tag := ""
	if at := strings.Index(ref, "@"); at >= 0 {
		name, tag = ref[:at], ref[at:]
	} else if colon := strings.LastIndex(ref, ":"); colon >= 0 && !strings.Contains(ref[colon:], "/") {
		name, tag = ref[:colon], ref[colon:]
	}
	first := strings.SplitN(name, "/", 2)[0]
	hasRegistry := strings.ContainsAny(first, ".:") || first == "localhost"
	if !hasRegistry {
		if !strings.Contains(name, "/") {
			name = "library/" + name
		}
		name = "docker.io/" + name
	}
	return name + tag
}
