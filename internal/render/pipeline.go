package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"os"
	"path/filepath"
)

// Renderer converts source images into cached 1-bit BMP and PNG files on disk,
// named by the content hash of the reduced image so identical content dedups
// and changed content busts the device cache.
type Renderer struct {
	dir string
}

// NewRenderer returns a Renderer that writes into dir (the uploads directory).
func NewRenderer(dir string) *Renderer { return &Renderer{dir: dir} }

// Result describes the cached output of a render.
type Result struct {
	Hash    string // content hash (also the filename stem)
	BMPName string // e.g. "<hash>.bmp"
	PNGName string // e.g. "<hash>.png"
}

// Process reduces src to monochrome using mode, then ensures both a 1-bit BMP
// and a 1-bit PNG named by content hash exist on disk. Encoding is skipped when
// the files already exist.
func (r *Renderer) Process(src image.Image, mode Mode) (Result, error) {
	mono := Monochrome(src, mode)

	sum := sha256.Sum256(mono.Pix)
	hash := hex.EncodeToString(sum[:16])
	res := Result{Hash: hash, BMPName: hash + ".bmp", PNGName: hash + ".png"}

	if err := r.writeIfAbsent(res.BMPName, func(buf *bytes.Buffer) error {
		return EncodeBMP1(buf, mono)
	}); err != nil {
		return Result{}, err
	}
	if err := r.writeIfAbsent(res.PNGName, func(buf *bytes.Buffer) error {
		return EncodePNG1(buf, mono)
	}); err != nil {
		return Result{}, err
	}
	return res, nil
}

func (r *Renderer) writeIfAbsent(name string, encode func(*bytes.Buffer) error) error {
	path := filepath.Join(r.dir, name)
	if _, err := os.Stat(path); err == nil {
		return nil // already cached
	}
	var buf bytes.Buffer
	if err := encode(&buf); err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	// Write atomically so a concurrent reader never sees a partial file.
	tmp, err := os.CreateTemp(r.dir, name+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}
