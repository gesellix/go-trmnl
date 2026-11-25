package zectrix

import (
	"context"
	"testing"

	"github.com/gesellix/go-trmnl/internal/plugins"
)

func TestCodeInfo(t *testing.T) {
	cases := []struct {
		code int
		cat  category
		text string
	}{
		{0, catClear, "Clear"},
		{2, catPartly, "Partly cloudy"},
		{3, catCloudy, "Overcast"},
		{45, catFog, "Fog"},
		{63, catRain, "Rain"},
		{75, catSnow, "Snow"},
		{95, catStorm, "Thunderstorm"},
	}
	for _, c := range cases {
		cat, text := codeInfo(c.code)
		if cat != c.cat || text != c.text {
			t.Errorf("codeInfo(%d) = (%d, %q), want (%d, %q)", c.code, cat, text, c.cat, c.text)
		}
	}
}

// The data model (settings, units, geocoding, forecast fetch) is the weather
// plugin's and tested there.

func TestRenderProducesPanel(t *testing.T) {
	p := New()
	d := Data{Place: "Berlin", TempUnit: "C", WindUnit: "km/h", Temp: 18, FeelsLike: 17, Code: 61,
		CondLabel: "Rain", Humidity: 72, Wind: 14, Sunrise: "05:20", Sunset: "21:40",
		Days: []DayForecast{
			{Heading: "Today", Code: 2, Hi: 21, Lo: 12, UV: 5, PrecipPct: 10},
			{Heading: "Tomorrow", Code: 61, Hi: 17, Lo: 9, UV: 2, PrecipPct: 40},
		}}
	img, err := p.Render(context.Background(), plugins.RenderInput{Width: 800, Height: 480}, d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Errorf("bounds = %v, want 800x480", b)
	}
}

func TestLayoutScalesFromDesignSize(t *testing.T) {
	tests := []struct {
		w, h  int
		wantK float64
	}{
		{400, 300, 1},
		{800, 480, 1.6}, // height-bound: 480/300
		{800, 600, 2},
		{296, 128, 128.0 / 300},
	}
	for _, tt := range tests {
		if got := newLayout(tt.w, tt.h).k; got != tt.wantK {
			t.Errorf("newLayout(%d, %d).k = %v, want %v", tt.w, tt.h, got, tt.wantK)
		}
	}
}

func TestRegisteredWithWeatherDataModel(t *testing.T) {
	p, ok := plugins.Get("zectrix")
	if !ok {
		t.Fatal("zectrix plugin not registered")
	}
	if w, ok := plugins.Get("weather"); !ok || p.DefaultRefresh() != w.DefaultRefresh() {
		t.Errorf("refresh = %v, want the weather plugin's", p.DefaultRefresh())
	}
	if _, err := p.Render(context.Background(), plugins.RenderInput{Width: 400, Height: 300}, "not a model"); err == nil {
		t.Error("Render accepted an invalid data model")
	}
}
