package bundle

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienhmmt/helmdownloader/pkg/version"
)

// SPDX 2.3 JSON document structures. Only the fields needed for standard
// SBOM tooling ingestion are modeled; the format is documented at
// https://spdx.github.io/spdx-spec/v2.3/.

type spdxDocument struct {
	SPDXVersion       string         `json:"spdxVersion"`
	DataLicense       string         `json:"dataLicense"`
	SPDXID            string         `json:"SPDXID"`
	Name              string         `json:"name"`
	DocumentNamespace string         `json:"documentNamespace"`
	CreationInfo      spdxCreation   `json:"creationInfo"`
	Packages          []spdxPackage  `json:"packages"`
	Relationships     []spdxRelation `json:"relationships"`
}

type spdxCreation struct {
	Created            string   `json:"created"`
	Creators           []string `json:"creators"`
	LicenseListVersion string   `json:"licenseListVersion,omitempty"`
}

type spdxPackage struct {
	SPDXID           string            `json:"SPDXID"`
	Name             string            `json:"name"`
	VersionInfo      string            `json:"versionInfo,omitempty"`
	DownloadLocation string            `json:"downloadLocation,omitempty"`
	FilesAnalyzed    bool              `json:"filesAnalyzed"`
	LicenseConcluded string            `json:"licenseConcluded,omitempty"`
	LicenseDeclared  string            `json:"licenseDeclared,omitempty"`
	Copyright        string            `json:"copyrightText,omitempty"`
	ExternalRefs     []spdxExternalRef `json:"externalRefs,omitempty"`
	Checksums        []spdxChecksum    `json:"checksums,omitempty"`
}

type spdxExternalRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}

type spdxChecksum struct {
	Algorithm     string `json:"algorithm"`
	ChecksumValue string `json:"checksumValue"`
}

type spdxRelation struct {
	SPDXElementID      string `json:"spdxElementId"`
	RelationshipType   string `json:"relationshipType"`
	RelatedSPDXElement string `json:"relatedSpdxElement"`
}

// buildSBOM renders an SPDX 2.3 JSON document for the bundle: one package
// for the Helm chart and one per container image, each carrying its pinned
// digest as a checksum when present. now supplies the creation timestamp so
// tests can control it. The returned bytes are pretty-printed JSON.
func buildSBOM(spec Spec, _ string, now time.Time) ([]byte, error) {
	docNS := fmt.Sprintf("https://helmdownloader.example/spdx/%s-%s-%d",
		spec.ChartName, spec.ChartVersion, now.Unix())
	doc := spdxDocument{
		SPDXVersion:       "SPDX-2.3",
		DataLicense:       "CC0-1.0",
		SPDXID:            "SPDXRef-DOCUMENT",
		Name:              fmt.Sprintf("helmdownloader bundle for %s %s", spec.ChartName, spec.ChartVersion),
		DocumentNamespace: docNS,
		CreationInfo: spdxCreation{
			Created:  now.UTC().Format(time.RFC3339),
			Creators: []string{"Tool: " + version.String()},
		},
	}
	// Use the chart archive basename only — never absolute host paths from the build machine.
	chartLoc := filepath.Base(spec.ChartPath)
	if chartLoc == "" || chartLoc == "." {
		chartLoc = "NOASSERTION"
	}
	chartPkg := spdxPackage{
		SPDXID:           "SPDXRef-Package-Chart",
		Name:             spec.ChartName,
		VersionInfo:      spec.ChartVersion,
		DownloadLocation: chartLoc,
		FilesAnalyzed:    false,
		LicenseConcluded: "NOASSERTION",
		LicenseDeclared:  "NOASSERTION",
		Copyright:        "NOASSERTION",
	}
	doc.Packages = append(doc.Packages, chartPkg)
	for i, img := range spec.Images {
		pkgID := fmt.Sprintf("SPDXRef-Package-Image-%d", i+1)
		imgPkg := spdxPackage{
			SPDXID:           pkgID,
			Name:             img.SourceRef,
			DownloadLocation: img.SourceRef,
			FilesAnalyzed:    false,
			LicenseConcluded: "NOASSERTION",
			LicenseDeclared:  "NOASSERTION",
			Copyright:        "NOASSERTION",
			ExternalRefs: []spdxExternalRef{
				{
					ReferenceCategory: "PACKAGE-MANAGER",
					ReferenceType:     "purl",
					ReferenceLocator:  fmt.Sprintf("pkg:oci/%s", img.SourceRef),
				},
			},
		}
		if img.Digest != "" && img.Digest != "-" {
			algo, val := parseDigest(img.Digest)
			imgPkg.Checksums = []spdxChecksum{{Algorithm: algo, ChecksumValue: val}}
		}
		doc.Packages = append(doc.Packages, imgPkg)
		doc.Relationships = append(doc.Relationships, spdxRelation{
			SPDXElementID:      "SPDXRef-Package-Chart",
			RelationshipType:   "DEPENDS_ON",
			RelatedSPDXElement: pkgID,
		})
	}
	return json.MarshalIndent(doc, "", "  ")
}

// parseDigest splits a "sha256:hex" digest into SPDX checksum algorithm and
// value. SPDX uses uppercase algorithm names ("SHA256"). Unknown algorithms
// fall back to the raw string.
func parseDigest(d string) (algo, value string) {
	if i := strings.Index(d, ":"); i >= 0 {
		return strings.ToUpper(d[:i]), d[i+1:]
	}
	return "SHA256", d
}
