// Package plugins defines the screen-plugin abstraction and a registry of
// available plugin types. Built-in plugins live in subpackages and register
// themselves via init(); import them for their side effects (the database/sql
// driver pattern).
package plugins

import (
	"context"
	"encoding/json"
	"image"
	"sort"
	"sync"
	"time"

	"github.com/gesellix/go-trmnl/internal/store"
)

// RenderInput carries everything a plugin needs to render one screen.
type RenderInput struct {
	Device    *store.Device
	Screen    *store.Screen
	Settings  json.RawMessage // == Screen.SettingsJSON
	Now       time.Time
	Width     int
	Height    int
	AssetsDir string // directory holding uploaded assets (for static images)
}

// Plugin renders a screen. DataModel is split from Render so that network/IO
// (e.g. fetching weather) is isolated and Render can be golden-tested with
// injected data.
type Plugin interface {
	// Type is the unique registry key, e.g. "clock".
	Type() string
	// Title is a human-friendly name shown in the admin UI.
	Title() string
	// DataModel computes/fetches the data the screen needs.
	DataModel(ctx context.Context, in RenderInput) (any, error)
	// Render draws the screen to an RGBA of in.Width x in.Height.
	Render(ctx context.Context, in RenderInput, data any) (*image.RGBA, error)
	// DefaultRefresh hints how long a render stays fresh (cache TTL). Zero
	// means always re-render.
	DefaultRefresh() time.Duration
}

var (
	mu       sync.RWMutex
	registry = map[string]Plugin{}
)

// Register adds a plugin to the registry. It panics on a duplicate type, which
// can only happen at init time.
func Register(p Plugin) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[p.Type()]; dup {
		panic("plugins: duplicate registration for type " + p.Type())
	}
	registry[p.Type()] = p
}

// Get returns the plugin registered for typ.
func Get(typ string) (Plugin, bool) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[typ]
	return p, ok
}

// All returns the registered plugins sorted by type, for listing in the UI.
func All() []Plugin {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Plugin, 0, len(registry))
	for _, p := range registry {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type() < out[j].Type() })
	return out
}
