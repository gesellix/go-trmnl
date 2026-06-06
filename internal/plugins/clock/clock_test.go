package clock

import (
	"context"
	"testing"
	"time"

	// Embed tzdata so the timezone test passes regardless of system zoneinfo.
	_ "time/tzdata"

	"github.com/gesellix/go-trmnl/internal/plugins"
)

func TestDataModel24hAndLabel(t *testing.T) {
	p := &Plugin{}
	now := time.Date(2026, time.June, 6, 14, 8, 0, 0, time.UTC)
	in := plugins.RenderInput{Now: now, Settings: []byte(`{"use_24h":true,"label":"Office"}`)}

	raw, err := p.DataModel(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	d := raw.(data)
	if d.Time != "14:08" {
		t.Errorf("24h time = %q, want 14:08", d.Time)
	}
	if d.Label != "Office" {
		t.Errorf("label = %q", d.Label)
	}
	if d.Date != "Saturday, June 6" {
		t.Errorf("date = %q", d.Date)
	}
}

func TestDataModel12h(t *testing.T) {
	p := &Plugin{}
	now := time.Date(2026, time.June, 6, 14, 8, 0, 0, time.UTC)
	raw, _ := p.DataModel(context.Background(), plugins.RenderInput{Now: now, Settings: []byte(`{}`)})
	if raw.(data).Time != "2:08 PM" {
		t.Errorf("12h time = %q, want 2:08 PM", raw.(data).Time)
	}
}

func TestDataModelTimezone(t *testing.T) {
	p := &Plugin{}
	// In June, New York is on EDT (UTC-4), so 14:08 UTC is 10:08.
	now := time.Date(2026, time.June, 6, 14, 8, 0, 0, time.UTC)
	in := plugins.RenderInput{Now: now, Settings: []byte(`{"use_24h":true,"timezone":"America/New_York"}`)}
	raw, err := p.DataModel(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got := raw.(data).Time; got != "10:08" {
		t.Errorf("New York time = %q, want 10:08", got)
	}
}

func TestRenderDimensions(t *testing.T) {
	p := &Plugin{}
	in := plugins.RenderInput{Width: 800, Height: 480, Now: time.Now()}
	d, _ := p.DataModel(context.Background(), in)
	img, err := p.Render(context.Background(), in, d)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Errorf("bounds = %v", b)
	}
}
