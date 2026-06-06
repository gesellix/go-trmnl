// Package uploads manages the rendered-image cache directory: pruning stale
// files that are no longer referenced by any screen.
//
// Rendered images are content-addressed (`<hash>.bmp` / `<hash>.png`), so a
// screen that re-renders leaves its previous files orphaned. The display path
// re-renders when a cached file is missing, so removing unreferenced files is
// safe and self-healing.
package uploads

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Placeholder is the fallback image filename, which is never pruned.
const Placeholder = "placeholder.bmp"

// Sweep removes files directly in dir whose name stem is not in keep, that are
// not the placeholder, and that were last modified before now-olderThan.
// Subdirectories (e.g. the uploaded-asset store) are not touched. It returns the
// number of files removed.
//
// keep holds content-hash stems to retain (typically every screen's current
// rendered_hash). The olderThan grace period protects freshly written files
// from being removed before the device fetches them or the screen row updates.
func Sweep(dir string, keep map[string]bool, olderThan time.Duration) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-olderThan)

	removed := 0
	for _, e := range entries {
		if e.IsDir() || e.Name() == Placeholder {
			continue
		}
		if keep[stem(e.Name())] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue // too recent to prune
		}
		if os.Remove(filepath.Join(dir, e.Name())) == nil {
			removed++
		}
	}
	return removed, nil
}

// SweepAssets removes files directly in dir (the uploaded-asset store) whose
// exact name is not in keep and that are older than now-olderThan. A missing
// directory is treated as empty. Unlike Sweep, assets are matched by full file
// name (they carry arbitrary extensions), not by content-hash stem.
func SweepAssets(dir string, keep map[string]bool, olderThan time.Duration) (int, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-olderThan)

	removed := 0
	for _, e := range entries {
		if e.IsDir() || keep[e.Name()] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if os.Remove(filepath.Join(dir, e.Name())) == nil {
			removed++
		}
	}
	return removed, nil
}

// stem strips a rendered-image extension so "<hash>.bmp" and "<hash>.png" both
// map to "<hash>". Names without those extensions (e.g. atomic-write temp files
// like "<hash>.bmp.tmp-123") return unchanged and so are eligible for pruning.
func stem(name string) string {
	switch {
	case strings.HasSuffix(name, ".bmp"):
		return strings.TrimSuffix(name, ".bmp")
	case strings.HasSuffix(name, ".png"):
		return strings.TrimSuffix(name, ".png")
	default:
		return name
	}
}
