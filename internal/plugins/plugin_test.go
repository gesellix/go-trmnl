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
	a, err := Face(24, false)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Face(24, false)
	if a != b {
		t.Error("Face should return the same cached face for identical params")
	}
	if c, _ := Face(24, true); c == a {
		t.Error("bold and regular faces should differ")
	}
}
