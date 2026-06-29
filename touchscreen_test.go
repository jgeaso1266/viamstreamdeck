package viamstreamdeck

import (
	"image"
	"image/color"
	"testing"

	"go.viam.com/test"
)

func TestRenderStripDimensionsAndBackground(t *testing.T) {
	img := renderStrip("hi", "white", "red", nil, nil, false, 0)

	b := img.Bounds()
	test.That(t, b.Dx(), test.ShouldEqual, touchscreenWidth)
	test.That(t, b.Dy(), test.ShouldEqual, touchscreenHeight)

	// A corner pixel should be the background color (red), away from centered text.
	r, g, bl, _ := img.At(2, 2).RGBA()
	test.That(t, r>>8, test.ShouldEqual, uint32(0xff))
	test.That(t, g>>8, test.ShouldEqual, uint32(0x00))
	test.That(t, bl>>8, test.ShouldEqual, uint32(0x00))
}

func TestRenderStripScrollShiftsText(t *testing.T) {
	// With a different scroll offset, the rendered text pixels should move, so the
	// two frames must differ somewhere.
	a := renderStrip("scrolling status text", "white", "black", nil, nil, true, 0)
	b := renderStrip("scrolling status text", "white", "black", nil, nil, true, 50)

	test.That(t, imagesDiffer(a, b), test.ShouldBeTrue)
}

func TestStripFontsFallback(t *testing.T) {
	sf := newStripFonts(nil)

	// The bundled emoji fallback must be present for symbol glyphs.
	test.That(t, sf.fallbackTTF, test.ShouldNotBeNil)

	// Latin comes from the primary font; the chess pawn (absent from M+) must
	// resolve to the emoji fallback face rather than a .notdef box.
	test.That(t, sf.faceFor('A'), test.ShouldEqual, sf.primary)
	test.That(t, sf.faceFor('♟'), test.ShouldEqual, sf.fallback)

	// A mixed string measures wider than the bare text, confirming the symbol
	// contributes a real (fallback) glyph advance.
	test.That(t, sf.measure("♟ A") > sf.measure("A"), test.ShouldBeTrue)
}

func TestFormatTouchscreenText(t *testing.T) {
	test.That(t, formatTouchscreenText(&TouchscreenConfig{Format: "temp: %v"}, 21.5), test.ShouldEqual, "temp: 21.5")
	test.That(t, formatTouchscreenText(&TouchscreenConfig{Text: "status: "}, "ok"), test.ShouldEqual, "status: ok")
	test.That(t, formatTouchscreenText(&TouchscreenConfig{}, 7), test.ShouldEqual, "7")
}

func imagesDiffer(a, b *image.RGBA) bool {
	if a.Bounds() != b.Bounds() {
		return true
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if !sameColor(a.At(x, y), b.At(x, y)) {
				return true
			}
		}
	}
	return false
}

func sameColor(c1, c2 color.Color) bool {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}
