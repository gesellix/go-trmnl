package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
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

// mockServer serves canned Open-Meteo geocode and forecast responses.
func mockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"latitude":52.52,"longitude":13.41,"name":"Berlin","country":"Germany"}]}`))
	})
	mux.HandleFunc("/v1/forecast", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"current":{"temperature_2m":18.3,"apparent_temperature":17.1,"relative_humidity_2m":72,"wind_speed_10m":14.0,"weather_code":61},
			"daily":{"time":["2026-06-06","2026-06-07"],
				"weather_code":[61,2],
				"temperature_2m_max":[21,17],
				"temperature_2m_min":[12,9],
				"sunrise":["2026-06-06T05:20","2026-06-07T05:21"],
				"sunset":["2026-06-06T21:40","2026-06-07T21:41"],
				"uv_index_max":[5.4,2.1],
				"precipitation_probability_max":[0,40]}
		}`))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func newMockPlugin(t *testing.T) *Plugin {
	t.Helper()
	ts := mockServer(t)
	p := New()
	p.forecastBase = ts.URL + "/v1/forecast"
	p.geocodeBase = ts.URL + "/v1/search"
	return p
}

func TestDataModelWithCoordinates(t *testing.T) {
	p := newMockPlugin(t)
	in := plugins.RenderInput{Settings: []byte(`{"latitude":52.52,"longitude":13.41,"units":"metric"}`), Width: 800, Height: 480}
	raw, err := p.DataModel(context.Background(), in)
	if err != nil {
		t.Fatalf("DataModel: %v", err)
	}
	d := raw.(Data)
	if d.Temp != 18.3 || d.FeelsLike != 17.1 || d.Humidity != 72 || d.Code != 61 {
		t.Errorf("unexpected current: %+v", d)
	}
	if d.TempUnit != "C" || d.WindUnit != "km/h" {
		t.Errorf("units = %q/%q, want C/km/h", d.TempUnit, d.WindUnit)
	}
	if d.Sunrise != "05:20" || d.Sunset != "21:40" {
		t.Errorf("sun times = %q/%q, want 05:20/21:40", d.Sunrise, d.Sunset)
	}
	if len(d.Days) != 2 || d.Days[0].Heading != "Today" || d.Days[1].Heading != "Tomorrow" {
		t.Errorf("forecast headings wrong: %+v", d.Days)
	}
	if d.Days[0].UV != 5 || d.Days[1].PrecipPct != 40 || d.Days[1].Hi != 17 {
		t.Errorf("forecast values wrong: %+v", d.Days)
	}
}

func TestForecastHeadingDate(t *testing.T) {
	p := newMockPlugin(t)
	in := plugins.RenderInput{Settings: []byte(`{"latitude":52.52,"longitude":13.41,"forecast_heading":"date"}`)}
	raw, err := p.DataModel(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	d := raw.(Data)
	if d.Days[0].Heading != "Jun 6" {
		t.Errorf("date heading = %q, want Jun 6", d.Days[0].Heading)
	}
}

func TestUnitOverrides(t *testing.T) {
	s := settings{Units: "metric", TempUnit: "f", WindUnit: "mph", PrecipUnit: "inch"}
	apiTemp, apiWind, apiPrecip, dispTemp, dispWind := s.resolveUnits()
	if apiTemp != "fahrenheit" || dispTemp != "F" {
		t.Errorf("temp override ignored: %s/%s", apiTemp, dispTemp)
	}
	if apiWind != "mph" || dispWind != "mph" {
		t.Errorf("wind override ignored: %s/%s", apiWind, dispWind)
	}
	if apiPrecip != "inch" {
		t.Errorf("precip override ignored: %s", apiPrecip)
	}
}

func TestDataModelGeocodesLocation(t *testing.T) {
	p := newMockPlugin(t)
	in := plugins.RenderInput{Settings: []byte(`{"location":"Berlin","units":"imperial"}`), Width: 800, Height: 480}
	raw, err := p.DataModel(context.Background(), in)
	if err != nil {
		t.Fatalf("DataModel: %v", err)
	}
	d := raw.(Data)
	if d.Place != "Berlin, Germany" {
		t.Errorf("place = %q, want Berlin, Germany", d.Place)
	}
	if d.TempUnit != "F" || d.WindUnit != "mph" {
		t.Errorf("imperial units = %q/%q, want F/mph", d.TempUnit, d.WindUnit)
	}
}

func TestDataModelRequiresLocation(t *testing.T) {
	p := New()
	_, err := p.DataModel(context.Background(), plugins.RenderInput{Settings: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected error when no location is configured")
	}
}

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
