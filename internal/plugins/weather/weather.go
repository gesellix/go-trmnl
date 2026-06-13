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

// settings configures the weather screen. units is kept for backward
// compatibility; the finer-grained *_unit fields override it when set.
type settings struct {
	Location        string  `json:"location"`         // optional city name, geocoded if lat/lon unset
	Latitude        float64 `json:"latitude"`         // preferred if non-zero
	Longitude       float64 `json:"longitude"`        //
	Units           string  `json:"units"`            // "metric" (default) or "imperial"
	TempUnit        string  `json:"temp_unit"`        // "c" or "f"
	WindUnit        string  `json:"wind_unit"`        // "kmh", "mph", "ms", "kn"
	PrecipUnit      string  `json:"precip_unit"`      // "mm" or "inch"
	ForecastHeading string  `json:"forecast_heading"` // "relative" (Today/Tomorrow) or "date"
	Label           string  `json:"label"`            // shown in the corner
}

func (s settings) imperial() bool { return s.Units == "imperial" }

// resolveUnits returns the Open-Meteo API unit tokens and their display labels.
func (s settings) resolveUnits() (apiTemp, apiWind, apiPrecip, dispTemp, dispWind string) {
	// Temperature.
	switch s.TempUnit {
	case "f":
		apiTemp, dispTemp = "fahrenheit", "F"
	case "c":
		apiTemp, dispTemp = "celsius", "C"
	default:
		if s.imperial() {
			apiTemp, dispTemp = "fahrenheit", "F"
		} else {
			apiTemp, dispTemp = "celsius", "C"
		}
	}
	// Wind.
	apiWind = s.WindUnit
	if apiWind == "" {
		if s.imperial() {
			apiWind = "mph"
		} else {
			apiWind = "kmh"
		}
	}
	dispWind = map[string]string{"kmh": "km/h", "mph": "mph", "ms": "m/s", "kn": "kn"}[apiWind]
	if dispWind == "" {
		apiWind, dispWind = "kmh", "km/h"
	}
	// Precipitation.
	apiPrecip = s.PrecipUnit
	if apiPrecip == "" {
		if s.imperial() {
			apiPrecip = "inch"
		} else {
			apiPrecip = "mm"
		}
	}
	if apiPrecip != "inch" {
		apiPrecip = "mm"
	}
	return apiTemp, apiWind, apiPrecip, dispTemp, dispWind
}

// Data is the rendered model. Exported so Render can be golden-tested with a
// hand-built value (no network).
type Data struct {
	Place    string
	TempUnit string // "C" or "F"
	WindUnit string // "km/h", "mph", ...
	Label    string

	Temp      float64
	FeelsLike float64
	Code      int
	CondLabel string // e.g. "Partly cloudy"
	Humidity  int
	Wind      float64
	Sunrise   string // "05:20"
	Sunset    string // "21:40"

	Days []DayForecast
}

// DayForecast is one day in the forecast strip.
type DayForecast struct {
	Heading   string // "Today"/"Tomorrow" or "Jun 8"
	Code      int
	Hi        float64
	Lo        float64
	UV        int
	PrecipPct int
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

	apiTemp, apiWind, apiPrecip, dispTemp, dispWind := s.resolveUnits()
	fc, err := p.fetchForecast(ctx, lat, lon, apiTemp, apiWind, apiPrecip)
	if err != nil {
		return nil, err
	}

	_, cond := codeInfo(fc.Current.WeatherCode)
	d := Data{
		Place:     place,
		TempUnit:  dispTemp,
		WindUnit:  dispWind,
		Label:     s.Label,
		Temp:      fc.Current.Temperature,
		FeelsLike: fc.Current.Apparent,
		Code:      fc.Current.WeatherCode,
		CondLabel: cond,
		Humidity:  fc.Current.Humidity,
		Wind:      fc.Current.WindSpeed,
		Sunrise:   hhmm(at(fc.Daily.Sunrise, 0)),
		Sunset:    hhmm(at(fc.Daily.Sunset, 0)),
	}
	for i := range fc.Daily.Time {
		if i >= 2 {
			break
		}
		d.Days = append(d.Days, DayForecast{
			Heading:   heading(s.ForecastHeading, fc.Daily.Time[i], i),
			Code:      atI(fc.Daily.WeatherCode, i),
			Hi:        atF(fc.Daily.TempMax, i),
			Lo:        atF(fc.Daily.TempMin, i),
			UV:        int(atF(fc.Daily.UVMax, i) + 0.5),
			PrecipPct: atI(fc.Daily.PrecipProb, i),
		})
	}
	return d, nil
}

// heading returns the forecast row heading for day i.
func heading(mode, isoDate string, i int) string {
	if mode == "date" {
		if t, err := time.Parse("2006-01-02", isoDate); err == nil {
			return t.Format("Jan 2")
		}
	}
	if i == 0 {
		return "Today"
	}
	return "Tomorrow"
}

// hhmm parses an Open-Meteo local timestamp ("2006-01-02T15:04") to "15:04".
func hhmm(s string) string {
	if s == "" {
		return ""
	}
	if t, err := time.Parse("2006-01-02T15:04", s); err == nil {
		return t.Format("15:04")
	}
	return s
}

// --- Open-Meteo API ---

type forecastResp struct {
	Current struct {
		Temperature float64 `json:"temperature_2m"`
		Apparent    float64 `json:"apparent_temperature"`
		Humidity    int     `json:"relative_humidity_2m"`
		WindSpeed   float64 `json:"wind_speed_10m"`
		WeatherCode int     `json:"weather_code"`
	} `json:"current"`
	Daily struct {
		Time        []string  `json:"time"`
		WeatherCode []int     `json:"weather_code"`
		TempMax     []float64 `json:"temperature_2m_max"`
		TempMin     []float64 `json:"temperature_2m_min"`
		Sunrise     []string  `json:"sunrise"`
		Sunset      []string  `json:"sunset"`
		UVMax       []float64 `json:"uv_index_max"`
		PrecipProb  []int     `json:"precipitation_probability_max"`
	} `json:"daily"`
}

func (p *Plugin) fetchForecast(ctx context.Context, lat, lon float64, tempUnit, windUnit, precipUnit string) (*forecastResp, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', 4, 64))
	q.Set("current", "temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,wind_speed_10m")
	q.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min,sunrise,sunset,uv_index_max,precipitation_probability_max")
	q.Set("timezone", "auto")
	q.Set("forecast_days", "2")
	q.Set("temperature_unit", tempUnit)
	q.Set("wind_speed_unit", windUnit)
	q.Set("precipitation_unit", precipUnit)

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

	drawCurrent(dc, in.Fonts, d, in.Width)
	drawForecast(dc, in.Fonts, d, in.Width)
	drawLabel(dc, in.Fonts, d, in.Width, in.Height)
	return img, nil
}

func at(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return ""
}

func atI(s []int, i int) int {
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
