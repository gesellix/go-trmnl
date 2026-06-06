package screens_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gesellix/go-trmnl/internal/render"
	"github.com/gesellix/go-trmnl/internal/screens"
	"github.com/gesellix/go-trmnl/internal/store"

	_ "github.com/gesellix/go-trmnl/internal/plugins/clock"
)

func TestRenderWritesFilesAndRecordsHash(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	pg, _ := st.CreatePlugin("clock", "Clock")
	sc, _ := st.CreateScreen(pg.ID, "Clock", `{"use_24h":true}`)

	r := render.NewRenderer(dir)
	res, err := screens.Render(context.Background(), st, r, filepath.Join(dir, "assets"), nil, sc, render.FloydSteinberg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if res.Hash == "" {
		t.Fatal("empty hash")
	}
	for _, name := range []string{res.BMPName, res.PNGName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing output %s: %v", name, err)
		}
	}
	if got, _ := st.GetScreen(sc.ID); got.RenderedHash.String != res.Hash {
		t.Errorf("rendered hash not recorded on screen")
	}
}

func TestRenderUnknownPluginType(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.Open(filepath.Join(dir, "t.db"))
	t.Cleanup(func() { _ = st.Close() })

	pg, _ := st.CreatePlugin("nonexistent", "X")
	sc, _ := st.CreateScreen(pg.ID, "X", "{}")

	r := render.NewRenderer(dir)
	if _, err := screens.Render(context.Background(), st, r, dir, nil, sc, render.Threshold); err == nil {
		t.Error("expected error for unknown plugin type")
	}
}
