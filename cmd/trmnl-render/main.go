// Command trmnl-render renders a single screen plugin to image files without a
// server, database, or registered device. It is a development aid for testing
// screen output and iterating on plugins.
//
//	trmnl-render -plugin clock -settings '{"use_24h":true,"label":"Office"}' -out clock
//	trmnl-render -plugin weather -settings '{"location":"Berlin"}' -out weather
//	trmnl-render -list
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	// Embed tzdata so timezone-aware plugins work without system zoneinfo.
	_ "time/tzdata"

	"github.com/gesellix/go-trmnl/internal/calendar"
	"github.com/gesellix/go-trmnl/internal/plugins"
	"github.com/gesellix/go-trmnl/internal/plugins/familycalendar"
	"github.com/gesellix/go-trmnl/internal/render"
	"github.com/gesellix/go-trmnl/internal/secret"
	"github.com/gesellix/go-trmnl/internal/store"

	// Register built-in plugins.
	_ "github.com/gesellix/go-trmnl/internal/plugins/clock"
	_ "github.com/gesellix/go-trmnl/internal/plugins/daysleft"
	_ "github.com/gesellix/go-trmnl/internal/plugins/quote"
	_ "github.com/gesellix/go-trmnl/internal/plugins/staticimage"
	_ "github.com/gesellix/go-trmnl/internal/plugins/weather"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "trmnl-render:", err)
		os.Exit(1)
	}
}

func run() error {
	plugin := flag.String("plugin", "clock", "plugin type to render")
	settings := flag.String("settings", "{}", "screen settings as JSON")
	out := flag.String("out", "screen", "output path prefix (writes <out>.bmp and <out>.png)")
	ditherName := flag.String("dither", "floyd_steinberg", "dither mode: floyd_steinberg or threshold")
	assets := flag.String("assets", ".", "assets directory (for the staticimage plugin)")
	width := flag.Int("width", render.Width, "image width")
	height := flag.Int("height", render.Height, "image height")
	dbPath := flag.String("db", "", "SQLite database path (enables the familycalendar plugin to read cached events)")
	secretKey := flag.String("secret-key", os.Getenv("TRMNL_SECRET_KEY"), "key to decrypt calendar credentials at rest")
	list := flag.Bool("list", false, "list available plugins and exit")
	flag.Parse()

	// The family calendar plugin needs DB-backed access; register it with a
	// service when -db is given, otherwise with nil (renders an empty agenda).
	var calSvc *calendar.Service
	if *dbPath != "" {
		st, err := store.Open(*dbPath)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer func() { _ = st.Close() }()
		calSvc = calendar.NewService(st, secret.New(*secretKey), "")
	}
	plugins.Register(familycalendar.New(calSvc))

	if *list {
		for _, p := range plugins.All() {
			fmt.Printf("%-12s %s\n", p.Type(), p.Title())
		}
		return nil
	}

	p, ok := plugins.Get(*plugin)
	if !ok {
		return fmt.Errorf("unknown plugin %q (use -list to see options)", *plugin)
	}

	in := plugins.RenderInput{
		Settings:  []byte(*settings),
		Now:       time.Now(),
		Width:     *width,
		Height:    *height,
		AssetsDir: *assets,
	}
	ctx := context.Background()
	data, err := p.DataModel(ctx, in)
	if err != nil {
		return fmt.Errorf("data model: %w", err)
	}
	img, err := p.Render(ctx, in, data)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	mono := render.Monochrome(img, render.ParseMode(*ditherName))
	if err := writeFile(*out+".bmp", func(f *os.File) error { return render.EncodeBMP1(f, mono) }); err != nil {
		return err
	}
	if err := writeFile(*out+".png", func(f *os.File) error { return render.EncodePNG1(f, mono) }); err != nil {
		return err
	}
	fmt.Printf("wrote %s.bmp and %s.png (%dx%d, %s)\n", *out, *out, *width, *height, *ditherName)
	return nil
}

func writeFile(path string, encode func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := encode(f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
