// Package appicon draws a fallback app icon at runtime (no binary
// assets) and packs images into Windows ICO containers. Apps with their
// own artwork can skip this package entirely and pass PNG/ICO bytes to
// appshell and tray directly; apps without artwork get a decent
// placeholder glyph in their own colors.
package appicon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

// Palette colors the rendered glyph.
type Palette struct {
	BG color.NRGBA // disc background
	FG color.NRGBA // ring + hands
}

// DefaultPalette is the Gantry accent blue on a dark disc (the same
// colors as the default --gantry-* theme).
func DefaultPalette() Palette {
	return Palette{
		BG: color.NRGBA{R: 16, G: 16, B: 18, A: 255},
		FG: color.NRGBA{R: 110, G: 168, B: 254, A: 255},
	}
}

func clamp01(v float64) float64 {
	return math.Min(1, math.Max(0, v))
}

func lerp(a, b color.NRGBA, t float64) color.NRGBA {
	m := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return color.NRGBA{R: m(a.R, b.R), G: m(a.G, b.G), B: m(a.B, b.B), A: 255}
}

// seg is one axis-aligned stroke of the glyph, in 32x32 design space
// (centerline coordinates; every stroke shares one thickness).
type seg struct {
	x1, y1, x2, y2 float64
}

// gantrySegs is the Gantry mark: a portal frame (two legs and a beam),
// the hoist house on top, and the cable - the same shape as the SVG
// logo the scaffold's index page shows.
var gantrySegs = []seg{
	{5, 10, 5, 27},   // left leg
	{27, 10, 27, 27}, // right leg
	{5, 10, 27, 10},  // main beam
	{11, 5, 11, 10},  // house left
	{21, 5, 21, 10},  // house right
	{11, 5, 21, 5},   // house top
	{16, 10, 16, 17}, // cable
}

// Render draws the Gantry glyph (a dark disc carrying the gantry frame
// and a hoist dot) at any edge length in the given palette.
func Render(sz int, p Palette) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, sz, sz))
	s := float64(sz)
	c := s/2 - 0.5      // pixel-center of the disc
	r := s * 15 / 32    // disc radius
	scale := s / 32     // design space -> pixels
	half := 1.3 * scale // half stroke thickness
	hoistX, hoistY, hoistR := 16*scale-0.5, 20*scale-0.5, 2.4*scale

	// stroke coverage: distance from the pixel center to the nearest
	// stroke, softened half a pixel for anti-aliased edges.
	strokeCov := func(px, py float64) float64 {
		best := 0.0
		for _, g := range gantrySegs {
			x1, y1 := g.x1*scale-0.5, g.y1*scale-0.5
			x2, y2 := g.x2*scale-0.5, g.y2*scale-0.5
			dx := math.Max(math.Max(x1-px, px-x2), 0)
			dy := math.Max(math.Max(y1-py, py-y2), 0)
			cov := clamp01(half - math.Hypot(dx, dy) + 0.5)
			if cov > best {
				best = cov
			}
		}
		cov := clamp01(hoistR - math.Hypot(px-hoistX, py-hoistY) + 0.5)
		if cov > best {
			best = cov
		}
		return best
	}

	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			dx, dy := float64(x)-c, float64(y)-c
			cov := clamp01(r - math.Hypot(dx, dy) + 0.5) // disc edge vs transparent
			if cov == 0 {
				continue
			}
			col := lerp(p.BG, p.FG, strokeCov(float64(x), float64(y)))
			col.A = uint8(cov*255 + 0.5)
			img.SetNRGBA(x, y, col)
		}
	}
	return img
}

// PNG encodes an image as PNG bytes.
func PNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// ICO wraps a single image in an ICO container (tray icons want ICO).
func ICO(img *image.NRGBA) []byte {
	data := PNG(img)
	if data == nil {
		return nil
	}
	return buildICO([]icoFrame{{size: img.Rect.Dx(), data: data}})
}

// icoFrame is one image inside an ICO container: either a PNG stream or
// a classic DIB (see dib).
type icoFrame struct {
	size int
	data []byte
}

// buildICO wraps frames in an ICONDIR + ICONDIRENTRY table.
func buildICO(frames []icoFrame) []byte {
	out := []byte{0, 0, 1, 0}
	out = binary.LittleEndian.AppendUint16(out, uint16(len(frames)))
	offset := 6 + 16*len(frames)
	for _, f := range frames {
		edge := byte(f.size) // 0 means 256
		if f.size >= 256 {
			edge = 0
		}
		out = append(out, edge, edge, 0, 0)
		out = binary.LittleEndian.AppendUint16(out, 1)  // planes
		out = binary.LittleEndian.AppendUint16(out, 32) // bits per pixel
		out = binary.LittleEndian.AppendUint32(out, uint32(len(f.data)))
		out = binary.LittleEndian.AppendUint32(out, uint32(offset))
		offset += len(f.data)
	}
	for _, f := range frames {
		out = append(out, f.data...)
	}
	return out
}

// dib encodes one frame as a classic 32-bit ICO bitmap: BITMAPINFOHEADER
// with doubled height, bottom-up BGRA pixels, then an all-zero AND mask
// (transparency comes from the alpha channel). Old shell code paths only
// understand this format at small sizes, so MultiICO uses it below 128px.
func dib(img *image.NRGBA) []byte {
	sz := img.Rect.Dx()
	maskRow := ((sz + 31) / 32) * 4
	buf := make([]byte, 0, 40+sz*sz*4+maskRow*sz)
	hdr := make([]byte, 40)
	binary.LittleEndian.PutUint32(hdr[0:], 40)
	binary.LittleEndian.PutUint32(hdr[4:], uint32(sz))
	binary.LittleEndian.PutUint32(hdr[8:], uint32(sz*2))
	binary.LittleEndian.PutUint16(hdr[12:], 1)
	binary.LittleEndian.PutUint16(hdr[14:], 32)
	buf = append(buf, hdr...)
	for y := sz - 1; y >= 0; y-- {
		row := img.Pix[y*img.Stride : y*img.Stride+sz*4]
		for x := 0; x < sz*4; x += 4 {
			buf = append(buf, row[x+2], row[x+1], row[x], row[x+3])
		}
	}
	return append(buf, make([]byte, maskRow*sz)...)
}

// MultiICO builds the multi-resolution Windows .ico suitable for
// embedding into an exe resource: DIB frames at the classic shell sizes,
// PNG frames (Vista+) at the large ones to keep the file small. render
// draws one frame at the requested edge length.
func MultiICO(render func(sz int) *image.NRGBA) []byte {
	var frames []icoFrame
	for _, sz := range []int{16, 24, 32, 48, 64} {
		frames = append(frames, icoFrame{size: sz, data: dib(render(sz))})
	}
	for _, sz := range []int{128, 256} {
		data := PNG(render(sz))
		if data == nil {
			return nil
		}
		frames = append(frames, icoFrame{size: sz, data: data})
	}
	return buildICO(frames)
}
