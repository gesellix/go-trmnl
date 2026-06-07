package quote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gesellix/go-trmnl/internal/plugins"
)

func TestMotivationalDeterministicPerDay(t *testing.T) {
	p := New()
	day := time.Date(2026, 6, 7, 9, 0, 0, 0, time.UTC)
	raw, err := p.DataModel(context.Background(), plugins.RenderInput{Now: day, Settings: []byte(`{"provider":"motivational"}`)})
	if err != nil {
		t.Fatal(err)
	}
	d := raw.(Data)
	if d.Text == "" || d.Author == "" {
		t.Fatalf("empty motivational quote: %+v", d)
	}
	// Same day -> same quote.
	raw2, _ := p.DataModel(context.Background(), plugins.RenderInput{Now: day, Settings: []byte(`{"provider":"motivational"}`)})
	if raw2.(Data).Text != d.Text {
		t.Errorf("motivational quote not stable within a day")
	}
}

func TestDefaultProviderIsMotivational(t *testing.T) {
	p := New()
	raw, err := p.DataModel(context.Background(), plugins.RenderInput{Now: time.Now(), Settings: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if raw.(Data).Text == "" {
		t.Error("default provider produced no quote")
	}
}

func TestZenQuotes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/random", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"q":"Stay hungry.","a":"Steve Jobs","h":"<b/>"}]`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := New()
	p.zenBase = ts.URL
	raw, err := p.DataModel(context.Background(), plugins.RenderInput{Settings: []byte(`{"provider":"zenquotes","mode":"random"}`)})
	if err != nil {
		t.Fatal(err)
	}
	d := raw.(Data)
	if d.Text != "Stay hungry." || d.Author != "Steve Jobs" || d.Attribution != "zenquotes.io" {
		t.Errorf("zenquotes mapping wrong: %+v", d)
	}
}

func TestStoic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"text":"The obstacle is the way.","author":"Marcus Aurelius"}`))
	}))
	defer ts.Close()

	p := New()
	p.stoicBase = ts.URL
	raw, err := p.DataModel(context.Background(), plugins.RenderInput{Settings: []byte(`{"provider":"stoic"}`)})
	if err != nil {
		t.Fatal(err)
	}
	d := raw.(Data)
	if d.Text != "The obstacle is the way." || d.Author != "Marcus Aurelius" {
		t.Errorf("stoic mapping wrong: %+v", d)
	}
}

func TestCustomProvider(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"quote":"Cut with intent.","by":"CUT/daily"}`))
	}))
	defer ts.Close()

	p := New()
	settingsJSON := `{"provider":"custom","url":"` + ts.URL + `","text_field":"quote","author_field":"by"}`
	raw, err := p.DataModel(context.Background(), plugins.RenderInput{Settings: []byte(settingsJSON)})
	if err != nil {
		t.Fatal(err)
	}
	d := raw.(Data)
	if d.Text != "Cut with intent." || d.Author != "CUT/daily" {
		t.Errorf("custom mapping wrong: %+v", d)
	}
}

func TestUnknownProviderErrors(t *testing.T) {
	p := New()
	_, err := p.DataModel(context.Background(), plugins.RenderInput{Settings: []byte(`{"provider":"nope"}`)})
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestRenderProducesPanel(t *testing.T) {
	p := New()
	d := Data{
		Text:        "The only way to do great work is to love what you do, and to keep going even when the path is long.",
		Author:      "Steve Jobs",
		Attribution: "zenquotes.io",
		Label:       "Daily quote",
	}
	img, err := p.Render(context.Background(), plugins.RenderInput{Width: 800, Height: 480}, d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Errorf("bounds = %v, want 800x480", b)
	}
}
