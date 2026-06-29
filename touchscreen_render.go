package viamstreamdeck

import (
	"image"
	"image/draw"
	"math"

	"github.com/dh1tw/streamdeck"
	"github.com/golang/freetype/truetype"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	xdraw "golang.org/x/image/draw"
)

// touchscreenFontSize is the point size used for strip text. At 72 DPI a single
// line at this size sits comfortably within the 100px strip height.
const touchscreenFontSize = 48

// scrollGapPx is the blank space appended after the text before it repeats, so a
// scrolling string reads cleanly instead of butting head-to-tail.
const scrollGapPx = 80

// emojiFallbackFont is the bundled font used for any glyph the primary font
// lacks (chess symbols, warning/pause signs, emoji). The default text font
// (M+) has no such glyphs, so without this fallback they render as .notdef boxes.
const emojiFallbackFont = "NotoEmoji-Regular.ttf"

// stripFonts pairs a primary text font with an emoji/symbol fallback so a single
// string can mix Latin text and symbols neither font covers alone. Each rune is
// drawn from whichever font actually has it.
type stripFonts struct {
	primary     font.Face
	primaryFont *truetype.Font
	fallback    font.Face
	fallbackTTF *truetype.Font
}

// newStripFonts builds the primary (configured or default) face plus the bundled
// emoji fallback, both at the strip font size.
func newStripFonts(fontName *string) stripFonts {
	opts := &truetype.Options{Size: touchscreenFontSize, DPI: 72}

	pf := streamdeck.MonoRegular
	if fontName != nil {
		if f := GetFont(*fontName); f != nil {
			pf = f
		}
	}

	sf := stripFonts{primaryFont: pf, primary: truetype.NewFace(pf, opts)}
	if ff := GetFont(emojiFallbackFont); ff != nil {
		sf.fallbackTTF = ff
		sf.fallback = truetype.NewFace(ff, opts)
	}
	return sf
}

// faceFor returns the face that can render r: the primary font if it has the
// glyph, otherwise the fallback if that has it, otherwise the primary (which
// will draw a .notdef box - unavoidable if neither font covers the rune).
func (sf stripFonts) faceFor(r rune) font.Face {
	if sf.primaryFont.Index(r) != 0 {
		return sf.primary
	}
	if sf.fallbackTTF != nil && sf.fallbackTTF.Index(r) != 0 {
		return sf.fallback
	}
	return sf.primary
}

// measure returns the total advance width of text in pixels, picking a per-rune
// face so the width accounts for fallback glyphs.
func (sf stripFonts) measure(text string) int {
	var total fixed.Int26_6
	for _, r := range text {
		total += font.MeasureString(sf.faceFor(r), string(r))
	}
	return total.Ceil()
}

// drawString draws text starting at (x, baseline), switching face per rune so
// symbols come from the fallback font and text from the primary.
func (sf stripFonts) drawString(dst *image.RGBA, src image.Image, x, baseline int, text string) {
	d := &font.Drawer{Dst: dst, Src: src}
	d.Dot = fixed.P(x, baseline)
	for _, r := range text {
		d.Face = sf.faceFor(r)
		d.DrawString(string(r))
	}
}

// renderStrip composes an 800x100 RGBA frame for the touchscreen: an optional
// background image (scaled to cover and center-cropped) or a solid background
// color, with text drawn on top. When scroll is false the text is centered;
// when true it is drawn at -scrollOffsetPx and repeated after scrollGapPx so the
// caller can advance scrollOffsetPx for a seamless horizontal scroll.
func renderStrip(text, textColor, bgColor string, fontName *string, bg image.Image, scroll bool, scrollOffsetPx int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, touchscreenWidth, touchscreenHeight))

	// Background: solid color, then image on top if provided.
	draw.Draw(dst, dst.Bounds(), image.NewUniform(getColor(bgColor, "black")), image.Point{}, draw.Src)
	if bg != nil {
		drawCover(dst, bg)
	}

	if text == "" {
		return dst
	}

	fonts := newStripFonts(fontName)
	src := image.NewUniform(getColor(textColor, "white"))

	// Vertically center the line using the primary face metrics.
	m := fonts.primary.Metrics()
	baseline := (touchscreenHeight-(m.Ascent+m.Descent).Ceil())/2 + m.Ascent.Ceil()
	textW := fonts.measure(text)

	if !scroll {
		x := (touchscreenWidth - textW) / 2
		if x < 5 {
			x = 5
		}
		fonts.drawString(dst, src, x, baseline, text)
		return dst
	}

	// Scrolling: wrap the offset into one period and draw two copies so the tail
	// of one and the head of the next are both visible at the seam. The period is
	// at least the full strip width so a short message still travels all the way
	// across (with a clean gap) before repeating, instead of looping within a
	// fraction of the strip.
	period := max(textW, touchscreenWidth) + scrollGapPx
	off := ((scrollOffsetPx % period) + period) % period
	for _, x := range []int{-off, -off + period} {
		fonts.drawString(dst, src, x, baseline, text)
	}
	return dst
}

// drawCover scales src to fully cover the strip (preserving aspect ratio) and
// center-crops the overflow, matching the look of the library's FillPanel.
func drawCover(dst *image.RGBA, src image.Image) {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw == 0 || sh == 0 {
		return
	}
	scale := math.Max(float64(touchscreenWidth)/float64(sw), float64(touchscreenHeight)/float64(sh))
	dw := int(scale * float64(sw))
	dh := int(scale * float64(sh))
	ox := (touchscreenWidth - dw) / 2
	oy := (touchscreenHeight - dh) / 2
	xdraw.CatmullRom.Scale(dst, image.Rect(ox, oy, ox+dw, oy+dh), src, b, xdraw.Over, nil)
}
