// Package weather is a built-in plugin that shows current conditions and a
// short forecast, modeled on the TRMNL Weather screen. It fetches data from the
// free, key-less Open-Meteo API (https://open-meteo.com).
package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/plugins"
)

func init() { plugins.Register(New()) }

// Plugin renders a weather screen. The API base URLs and HTTP client are fields
// so tests can point them at a local server.
type Plugin struct {
	client       *http.Client
	forecastBase string
	geocodeBase  string
}

// New returns a weather plugin configured for the public Open-Meteo API.
func New() *Plugin {
	return &Plugin{
		client:       &http.Client{Timeout: 10 * time.Second},
		forecastBase: "https://api.open-meteo.com/v1/forecast",
		geocodeBase:  "https://geocoding-api.open-meteo.com/v1/search",
	}
}

// Type returns the registry key.
func (p *Plugin) Type() string { return "weather" }

// Title returns the human-friendly plugin name.
func (p *Plugin) Title() string { return "Weather" }

// DefaultRefresh returns the cache TTL hint for rendered weather screens.
func (p *Plugin) DefaultRefresh() time.Duration { return 30 * time.Minute }

// settings configures the weather screen.
type settings struct {
	Location  string  `json:"location"`  // optional city name, geocoded if lat/lon unset
	Latitude  float64 `json:"latitude"`  // preferred if non-zero
	Longitude float64 `json:"longitude"` //
	Units     string  `json:"units"`     // "metric" (default) or "imperial"
	Label     string  `json:"label"`     // shown in the corner
}

func (s settings) imperial() bool { return s.Units == "imperial" }

// Data is the rendered model. Exported so Render can be golden-tested with a
// hand-built value (no network).
type Data struct {
	Place    string
	TempUnit string // "C" or "F"
	WindUnit string // "km/h" or "mph"
	Label    string

	Temp     float64
	Code     int
	Humidity int
	Wind     float64

	Days []DayForecast
}

// DayForecast is one day in the forecast strip.
type DayForecast struct {
	Name string
	Code int
	Hi   float64
	Lo   float64
}

// DataModel fetches current conditions and the forecast from Open-Meteo.
func (p *Plugin) DataModel(ctx context.Context, in plugins.RenderInput) (any, error) {
	var s settings
	if len(in.Settings) > 0 {
		if err := json.Unmarshal(in.Settings, &s); err != nil {
			return nil, fmt.Errorf("weather: bad settings: %w", err)
		}
	}

	place := s.Location
	lat, lon := s.Latitude, s.Longitude
	if lat == 0 && lon == 0 {
		if s.Location == "" {
			return nil, fmt.Errorf("weather: set latitude/longitude or a location name")
		}
		gLat, gLon, name, err := p.geocode(ctx, s.Location)
		if err != nil {
			return nil, err
		}
		lat, lon, place = gLat, gLon, name
	}
	if place == "" {
		place = fmt.Sprintf("%.2f, %.2f", lat, lon)
	}

	tempUnit, windUnit := "celsius", "kmh"
	tu, wu := "C", "km/h"
	if s.imperial() {
		tempUnit, windUnit = "fahrenheit", "mph"
		tu, wu = "F", "mph"
	}

	fc, err := p.fetchForecast(ctx, lat, lon, tempUnit, windUnit)
	if err != nil {
		return nil, err
	}

	d := Data{
		Place:    place,
		TempUnit: tu,
		WindUnit: wu,
		Label:    s.Label,
		Temp:     fc.Current.Temperature,
		Code:     fc.Current.WeatherCode,
		Humidity: fc.Current.Humidity,
		Wind:     fc.Current.WindSpeed,
	}
	for i := range fc.Daily.Time {
		if i >= 4 {
			break
		}
		name := "Today"
		if t, err := time.Parse("2006-01-02", fc.Daily.Time[i]); err == nil && i > 0 {
			name = t.Format("Mon")
		}
		d.Days = append(d.Days, DayForecast{
			Name: name,
			Code: at(fc.Daily.WeatherCode, i),
			Hi:   atF(fc.Daily.TempMax, i),
			Lo:   atF(fc.Daily.TempMin, i),
		})
	}
	return d, nil
}

// --- Open-Meteo API ---

type forecastResp struct {
	Current struct {
		Temperature float64 `json:"temperature_2m"`
		Humidity    int     `json:"relative_humidity_2m"`
		WindSpeed   float64 `json:"wind_speed_10m"`
		WeatherCode int     `json:"weather_code"`
	} `json:"current"`
	Daily struct {
		Time        []string  `json:"time"`
		WeatherCode []int     `json:"weather_code"`
		TempMax     []float64 `json:"temperature_2m_max"`
		TempMin     []float64 `json:"temperature_2m_min"`
	} `json:"daily"`
}

func (p *Plugin) fetchForecast(ctx context.Context, lat, lon float64, tempUnit, windUnit string) (*forecastResp, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', 4, 64))
	q.Set("current", "temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m")
	q.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min")
	q.Set("timezone", "auto")
	q.Set("forecast_days", "4")
	q.Set("temperature_unit", tempUnit)
	q.Set("wind_speed_unit", windUnit)

	var out forecastResp
	if err := p.getJSON(ctx, p.forecastBase+"?"+q.Encode(), &out); err != nil {
		return nil, fmt.Errorf("weather: forecast: %w", err)
	}
	return &out, nil
}

type geocodeResp struct {
	Results []struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Name      string  `json:"name"`
		Country   string  `json:"country"`
	} `json:"results"`
}

func (p *Plugin) geocode(ctx context.Context, name string) (lat, lon float64, place string, err error) {
	q := url.Values{}
	q.Set("name", name)
	q.Set("count", "1")

	var out geocodeResp
	if err := p.getJSON(ctx, p.geocodeBase+"?"+q.Encode(), &out); err != nil {
		return 0, 0, "", fmt.Errorf("weather: geocode: %w", err)
	}
	if len(out.Results) == 0 {
		return 0, 0, "", fmt.Errorf("weather: location %q not found", name)
	}
	r := out.Results[0]
	place = r.Name
	if r.Country != "" {
		place = r.Name + ", " + r.Country
	}
	return r.Latitude, r.Longitude, place, nil
}

func (p *Plugin) getJSON(ctx context.Context, u string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// Render draws the weather screen to an RGBA image.
func (p *Plugin) Render(_ context.Context, in plugins.RenderInput, raw any) (*image.RGBA, error) {
	d, ok := raw.(Data)
	if !ok {
		return nil, fmt.Errorf("weather: invalid data model")
	}
	img := image.NewRGBA(image.Rect(0, 0, in.Width, in.Height))
	dc := gg.NewContextForRGBA(img)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)

	drawCurrent(dc, d)
	drawForecast(dc, d)
	drawLabel(dc, d, in.Width, in.Height)
	return img, nil
}

func at(s []int, i int) int {
	if i < len(s) {
		return s[i]
	}
	return 0
}

func atF(s []float64, i int) float64 {
	if i < len(s) {
		return s[i]
	}
	return 0
}
