package daysleft

import (
	"context"
	"testing"
	"time"

	"github.com/gesellix/go-trmnl/internal/plugins"
)

func TestDataModel(t *testing.T) {
	p := &Plugin{}
	// 2026-06-07 is the 158th day of a non-leap year.
	in := plugins.RenderInput{Now: time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)}
	raw, err := p.DataModel(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	d := raw.(Data)
	if d.Year != 2026 || d.Total != 365 {
		t.Fatalf("year/total = %d/%d, want 2026/365", d.Year, d.Total)
	}
	if d.Passed != 158 || d.Left != 365-158 || d.TodayIndex != 157 {
		t.Errorf("passed/left/today = %d/%d/%d, want 158/207/157", d.Passed, d.Left, d.TodayIndex)
	}
}

func TestDataModelLeapYearEnd(t *testing.T) {
	p := &Plugin{}
	in := plugins.RenderInput{Now: time.Date(2024, 12, 31, 23, 0, 0, 0, time.UTC)}
	raw, _ := p.DataModel(context.Background(), in)
	d := raw.(Data)
	if d.Total != 366 || d.Passed != 366 || d.Left != 0 || d.TodayIndex != 365 {
		t.Errorf("leap-year end: total/passed/left/today = %d/%d/%d/%d, want 366/366/0/365",
			d.Total, d.Passed, d.Left, d.TodayIndex)
	}
}

func TestRenderProducesPanel(t *testing.T) {
	p := &Plugin{}
	d := Data{Year: 2026, Passed: 158, Left: 207, Total: 365, TodayIndex: 157, Label: "2026"}
	img, err := p.Render(context.Background(), plugins.RenderInput{Width: 800, Height: 480}, d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Errorf("bounds = %v, want 800x480", b)
	}
}
