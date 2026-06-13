package plugins

import (
	"context"
	"image"
	"testing"
	"time"
)

type fakePlugin struct{ typ string }

func (f *fakePlugin) Type() string                                        { return f.typ }
func (f *fakePlugin) Title() string                                       { return "Fake " + f.typ }
func (f *fakePlugin) DefaultRefresh() time.Duration                       { return time.Minute }
func (f *fakePlugin) DataModel(context.Context, RenderInput) (any, error) { return nil, nil }
func (f *fakePlugin) Render(context.Context, RenderInput, any) (*image.RGBA, error) {
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
}

func TestRegisterGetAll(t *testing.T) {
	Register(&fakePlugin{typ: "zzz-test-b"})
	Register(&fakePlugin{typ: "zzz-test-a"})

	if _, ok := Get("zzz-test-a"); !ok {
		t.Error("Get did not find a registered plugin")
	}
	if _, ok := Get("does-not-exist"); ok {
		t.Error("Get returned a plugin for an unknown type")
	}

	all := All()
	if len(all) < 2 {
		t.Fatalf("All returned %d plugins, want >= 2", len(all))
	}
	// All is sorted by type.
	for i := 1; i < len(all); i++ {
		if all[i-1].Type() > all[i].Type() {
			t.Errorf("All not sorted: %q before %q", all[i-1].Type(), all[i].Type())
		}
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("duplicate registration did not panic")
		}
	}()
	Register(&fakePlugin{typ: "dup-test"})
	Register(&fakePlugin{typ: "dup-test"})
}

func TestFaceCaches(t *testing.T) {
	a, err := FaceStyle(24, StyleSans)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := FaceStyle(24, StyleSans)
	if a != b {
		t.Error("FaceStyle should return the same cached face for identical params")
	}
	if c, _ := FaceStyle(24, StyleTitle); c == a {
		t.Error("different styles should differ")
	}
}

func TestFontSetIsolatedCaches(t *testing.T) {
	// Each FontSet owns its own face cache, so two sets never share faces even
	// for identical params. This is what makes concurrent renders with
	// different bundles safe.
	a := NewFontSet("", "classic", "", "", "")
	b := NewFontSet("", "classic", "", "", "")

	fa, err := a.Face(20, StyleSans)
	if err != nil {
		t.Fatal(err)
	}
	fa2, _ := a.Face(20, StyleSans)
	if fa != fa2 {
		t.Error("a FontSet should return the same cached face for identical params")
	}
	if fb, _ := b.Face(20, StyleSans); fb == fa {
		t.Error("distinct FontSets should not share faces")
	}
}

func TestFontSetFallsBackToGoFonts(t *testing.T) {
	// A missing override file falls back to the built-in Go fonts rather than
	// erroring, so rendering always succeeds.
	fs := NewFontSet(".", "classic", "non-existent.ttf", "", "")
	if _, err := fs.Face(20, StyleSans); err != nil {
		t.Fatalf("Face should fall back to Go fonts, got error: %v", err)
	}
}

func TestNilFontSetUsesDefault(t *testing.T) {
	// A nil FontSet (e.g. a draw-only unit test) falls back to the package
	// default Go fonts.
	var fs *FontSet
	if _, err := fs.Face(20, StyleSans); err != nil {
		t.Fatalf("nil FontSet should use the default set, got error: %v", err)
	}
}
