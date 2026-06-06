package uploads_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gesellix/go-trmnl/internal/uploads"
)

func TestSweep(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-2 * time.Hour)

	write := func(name string, modAged bool) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if modAged {
			if err := os.Chtimes(p, old, old); err != nil {
				t.Fatal(err)
			}
		}
	}

	write("placeholder.bmp", true)  // never pruned
	write("aaaa.bmp", true)         // referenced -> kept
	write("aaaa.png", true)         // referenced -> kept
	write("bbbb.bmp", true)         // orphan, old -> removed
	write("bbbb.png", true)         // orphan, old -> removed
	write("cccc.bmp", false)        // orphan but recent -> kept (grace)
	write("dddd.bmp.tmp-123", true) // stale temp file -> removed
	write("assets/photo.png", true) // uploaded asset in subdir -> untouched

	removed, err := uploads.Sweep(dir, map[string]bool{"aaaa": true}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 3 {
		t.Errorf("removed = %d, want 3", removed)
	}

	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}
	for _, keep := range []string{"placeholder.bmp", "aaaa.bmp", "aaaa.png", "cccc.bmp", "assets/photo.png"} {
		if !exists(keep) {
			t.Errorf("expected %s to be kept", keep)
		}
	}
	for _, gone := range []string{"bbbb.bmp", "bbbb.png", "dddd.bmp.tmp-123"} {
		if exists(gone) {
			t.Errorf("expected %s to be removed", gone)
		}
	}
}

func TestSweepAssets(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-2 * time.Hour)
	write := func(name string, aged bool) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if aged {
			if err := os.Chtimes(p, old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	write("used.jpg", true)    // referenced -> kept
	write("orphan.png", true)  // unreferenced, old -> removed
	write("recent.gif", false) // unreferenced but recent -> kept (grace)

	removed, err := uploads.SweepAssets(dir, map[string]bool{"used.jpg": true}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "orphan.png")); err == nil {
		t.Error("orphan.png should have been removed")
	}
	for _, keep := range []string{"used.jpg", "recent.gif"} {
		if _, err := os.Stat(filepath.Join(dir, keep)); err != nil {
			t.Errorf("%s should have been kept", keep)
		}
	}

	// Missing directory is a no-op, not an error.
	if n, err := uploads.SweepAssets(filepath.Join(dir, "nope"), nil, time.Hour); err != nil || n != 0 {
		t.Errorf("missing dir: n=%d err=%v, want 0/nil", n, err)
	}
}
