package quote

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Quote is a provider-agnostic quotation.
type Quote struct {
	Text        string
	Author      string
	Attribution string
}

// fetchZen fetches from ZenQuotes (https://zenquotes.io). mode is "random"
// (default) or "today". The free tier requires crediting zenquotes.io.
func (p *Plugin) fetchZen(ctx context.Context, mode string) (Quote, error) {
	path := "/random"
	if mode == "today" {
		path = "/today"
	}
	var items []struct {
		Q string `json:"q"`
		A string `json:"a"`
	}
	if err := p.getJSON(ctx, p.zenBase+path, &items); err != nil {
		return Quote{}, fmt.Errorf("quote: zenquotes: %w", err)
	}
	if len(items) == 0 {
		return Quote{}, fmt.Errorf("quote: zenquotes: empty response")
	}
	return Quote{Text: items[0].Q, Author: items[0].A, Attribution: "zenquotes.io"}, nil
}

// fetchStoic fetches from stoic-quotes.com.
func (p *Plugin) fetchStoic(ctx context.Context) (Quote, error) {
	var out struct {
		Text   string `json:"text"`
		Author string `json:"author"`
	}
	if err := p.getJSON(ctx, p.stoicBase, &out); err != nil {
		return Quote{}, fmt.Errorf("quote: stoic: %w", err)
	}
	return Quote{Text: out.Text, Author: out.Author, Attribution: "stoic-quotes.com"}, nil
}

// fetchCustom polls a user-supplied JSON endpoint and extracts the quote and
// author by configurable field names. If the response is a JSON array, the
// first element is used (matching APIs like ZenQuotes). This also covers
// sources without a dedicated provider, e.g. CUT/daily.
func (p *Plugin) fetchCustom(ctx context.Context, s settings) (Quote, error) {
	if s.URL == "" {
		return Quote{}, fmt.Errorf("quote: custom provider needs a url")
	}
	textKey := s.TextField
	if textKey == "" {
		textKey = "text"
	}
	authorKey := s.AuthorField
	if authorKey == "" {
		authorKey = "author"
	}

	var raw json.RawMessage
	if err := p.getJSON(ctx, s.URL, &raw); err != nil {
		return Quote{}, fmt.Errorf("quote: custom: %w", err)
	}
	obj, err := firstObject(raw)
	if err != nil {
		return Quote{}, fmt.Errorf("quote: custom: %w", err)
	}
	q := Quote{Text: asString(obj[textKey]), Author: asString(obj[authorKey])}
	if q.Text == "" {
		return Quote{}, fmt.Errorf("quote: custom: no %q field in response", textKey)
	}
	return q, nil
}

// firstObject decodes raw as a JSON object, or the first element of a JSON
// array of objects.
func firstObject(raw json.RawMessage) (map[string]any, error) {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		var arr []map[string]any
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, err
		}
		if len(arr) == 0 {
			return nil, fmt.Errorf("empty array")
		}
		return arr[0], nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
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

// motivational returns a quote from a built-in, offline list. It rotates by day
// so the screen changes daily but is stable within a day (and in tests).
func motivational(now time.Time) Quote {
	q := motivationalQuotes[now.YearDay()%len(motivationalQuotes)]
	return q
}

// motivationalQuotes is a small curated, public-domain-ish set shipped in the
// binary (no network, no auth, no rate limits).
var motivationalQuotes = []Quote{
	{Text: "The secret of getting ahead is getting started.", Author: "Mark Twain"},
	{Text: "It always seems impossible until it's done.", Author: "Nelson Mandela"},
	{Text: "Whether you think you can or you think you can't, you're right.", Author: "Henry Ford"},
	{Text: "Action is the foundational key to all success.", Author: "Pablo Picasso"},
	{Text: "Quality means doing it right when no one is looking.", Author: "Henry Ford"},
	{Text: "What you do today can improve all your tomorrows.", Author: "Ralph Marston"},
	{Text: "Well done is better than well said.", Author: "Benjamin Franklin"},
	{Text: "Start where you are. Use what you have. Do what you can.", Author: "Arthur Ashe"},
	{Text: "Little by little, one travels far.", Author: "J. R. R. Tolkien"},
	{Text: "Do what you can, with what you have, where you are.", Author: "Theodore Roosevelt"},
	{Text: "Perseverance is not a long race; it is many short races one after another.", Author: "Walter Elliot"},
	{Text: "The best way out is always through.", Author: "Robert Frost"},
	{Text: "Energy and persistence conquer all things.", Author: "Benjamin Franklin"},
	{Text: "He who has a why to live can bear almost any how.", Author: "Friedrich Nietzsche"},
	{Text: "Simplicity is the ultimate sophistication.", Author: "Leonardo da Vinci"},
}
