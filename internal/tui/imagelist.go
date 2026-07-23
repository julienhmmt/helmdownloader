package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/julienhmmt/helmdownloader/pkg/images"
)

// imageListEntry is the JSON wire format for one entry in an exported image
// list. It is decoupled from images.Image so the on-disk format stays stable
// if images.Image grows internal-only fields later.
type imageListEntry struct {
	Ref      string `json:"ref"`
	Selected bool   `json:"selected"`
}

// exportImages writes the discovered image list to path as pretty-printed
// JSON. A missing path (cfg.ExportImages == "") is a no-op. The file is
// written atomically-ish: write to path + ".tmp" then rename, so a partial
// write never leaves a corrupt review file.
func exportImages(path string, imgs []images.Image) error {
	if path == "" {
		return nil
	}
	out := make([]imageListEntry, len(imgs))
	for i, img := range imgs {
		out[i] = imageListEntry{Ref: img.Ref, Selected: img.Selected}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal image list: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write image list: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename image list: %w", err)
	}
	return nil
}

// importImages reads an approved image list from path and returns it as
// []images.Image. Called once when entering Review (preparedMsg). A missing
// path (cfg.ImportImages == "") returns nil and no error, signaling "no
// import, use the discovered list as-is".
func importImages(path string) ([]images.Image, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read image list %s: %w", path, err)
	}
	var entries []imageListEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse image list %s: %w", path, err)
	}
	imgs := make([]images.Image, len(entries))
	for i, e := range entries {
		if !images.ValidRef(e.Ref) {
			return nil, fmt.Errorf("image list %s: entry %d: invalid image ref %q", path, i, e.Ref)
		}
		imgs[i] = images.Image{Ref: strings.TrimSpace(e.Ref), Selected: e.Selected}
	}
	return imgs, nil
}
