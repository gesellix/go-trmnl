package plugins

import (
	"fmt"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

var (
	fontMu       sync.Mutex
	regularFont  *opentype.Font
	boldFont     *opentype.Font
	faceCache    = map[string]font.Face{}
	parseFontErr error
)

func parseFonts() {
	if regularFont != nil || parseFontErr != nil {
		return
	}
	regularFont, parseFontErr = opentype.Parse(goregular.TTF)
	if parseFontErr != nil {
		return
	}
	boldFont, parseFontErr = opentype.Parse(gobold.TTF)
}

// Face returns a cached font face from the embedded Go font at the given point
// size. Faces are safe to reuse across renders.
func Face(points float64, bold bool) (font.Face, error) {
	fontMu.Lock()
	defer fontMu.Unlock()
	parseFonts()
	if parseFontErr != nil {
		return nil, parseFontErr
	}
	key := fmt.Sprintf("%v-%t", points, bold)
	if f, ok := faceCache[key]; ok {
		return f, nil
	}
	src := regularFont
	if bold {
		src = boldFont
	}
	f, err := opentype.NewFace(src, &opentype.FaceOptions{Size: points, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	faceCache[key] = f
	return f, nil
}
