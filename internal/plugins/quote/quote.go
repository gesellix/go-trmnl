// Package quote is a built-in plugin that shows a quotation from a selectable
// provider (ZenQuotes, Stoic, a built-in Motivational list, or a custom JSON
// endpoint). Each provider normalizes to a common {Text, Author} shape.
package quote

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"net/http"
	"time"

	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/plugins"
)

func init() { plugins.Register(New()) }

// Plugin renders a quote screen. Base URLs and the HTTP client are fields so
// tests can point them at a local server.
type Plugin struct {
	client    *http.Client
	zenBase   string // https://zenquotes.io/api
	stoicBase string // https://stoic-quotes.com/api/quote
}

// New returns a quote plugin configured for the public provider APIs.
func New() *Plugin {
	return &Plugin{
		client:    &http.Client{Timeout: 10 * time.Second},
		zenBase:   "https://zenquotes.io/api",
		stoicBase: "https://stoic-quotes.com/api/quote",
	}
}

// Type returns the registry key.
func (p *Plugin) Type() string { return "quote" }

// Title returns the human-friendly plugin name.
func (p *Plugin) Title() string { return "Quote" }

// DefaultRefresh returns the cache TTL hint. Quotes change slowly; a long TTL
// also keeps us well within ZenQuotes' keyless rate limit.
func (p *Plugin) DefaultRefresh() time.Duration { return 6 * time.Hour }

// settings configures the quote screen.
type settings struct {
	Provider    string `json:"provider"`     // "motivational" (default), "zenquotes", "stoic", "custom"
	Mode        string `json:"mode"`         // zenquotes: "random" (default) or "today"
	URL         string `json:"url"`          // custom: JSON endpoint to poll
	TextField   string `json:"text_field"`   // custom: JSON key holding the quote text (default "text")
	AuthorField string `json:"author_field"` // custom: JSON key holding the author (default "author")
	Label       string `json:"label"`
}

// Data is the render model. Exported so Render can be golden-tested with a
// hand-built value (no network).
type Data struct {
	Text        string
	Author      string
	Attribution string // provider credit shown small in the footer
	Label       string
}

// DataModel fetches a quote from the configured provider.
func (p *Plugin) DataModel(ctx context.Context, in plugins.RenderInput) (any, error) {
	var s settings
	if len(in.Settings) > 0 {
		if err := json.Unmarshal(in.Settings, &s); err != nil {
			return nil, fmt.Errorf("quote: bad settings: %w", err)
		}
	}
	provider := s.Provider
	if provider == "" {
		provider = "motivational"
	}

	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}

	var (
		q   Quote
		err error
	)
	switch provider {
	case "motivational":
		q = motivational(now)
	case "zenquotes":
		q, err = p.fetchZen(ctx, s.Mode)
	case "stoic":
		q, err = p.fetchStoic(ctx)
	case "custom":
		q, err = p.fetchCustom(ctx, s)
	default:
		return nil, fmt.Errorf("quote: unknown provider %q", provider)
	}
	if err != nil {
		return nil, err
	}

	return Data{Text: q.Text, Author: q.Author, Attribution: q.Attribution, Label: s.Label}, nil
}

// Render draws the quote screen to an RGBA image.
func (p *Plugin) Render(_ context.Context, in plugins.RenderInput, raw any) (*image.RGBA, error) {
	d, ok := raw.(Data)
	if !ok {
		return nil, fmt.Errorf("quote: invalid data model")
	}
	img := image.NewRGBA(image.Rect(0, 0, in.Width, in.Height))
	dc := gg.NewContextForRGBA(img)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)
	drawQuote(dc, in.Fonts, d, in.Width, in.Height)
	return img, nil
}
