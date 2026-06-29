package viamstreamdeck

import (
	"image"

	"github.com/dh1tw/streamdeck"

	"go.viam.com/rdk/logging"
)

// Stream Deck Plus touchscreen LCD strip dimensions. The strip is written
// through the streamdeck library's single HID handle via SetTouchscreenImage;
// libusb claims the USB interface exclusively, so a second handle is impossible
// and the strip must share the handle the library already owns.
const (
	touchscreenWidth  = streamdeck.TouchscreenWidth  // 800
	touchscreenHeight = streamdeck.TouchscreenHeight // 100
)

// touchscreen drives the LCD strip. It holds no handle of its own - it writes
// through the shared *streamdeck.StreamDeck, whose SetTouchscreenImage serializes
// strip frames against button writes with the library's internal lock.
type touchscreen struct {
	sd     *streamdeck.StreamDeck
	logger logging.Logger
}

// newTouchscreen returns a strip driver for the Stream Deck Plus, or nil for any
// other model (whose handle has no LCD strip), so callers can no-op.
func newTouchscreen(ms *ModelSetup, sd *streamdeck.StreamDeck, logger logging.Logger) *touchscreen {
	if ms == nil || sd == nil || ms.Conf.ProductID != streamdeck.Plus.ProductID {
		return nil
	}
	return &touchscreen{sd: sd, logger: logger}
}

// fillStrip draws img across the entire 800x100 strip.
func (t *touchscreen) fillStrip(img image.Image) error {
	return t.sd.FillTouchscreen(img)
}

// clear blanks the strip to solid black.
func (t *touchscreen) clear() error {
	black := image.NewRGBA(image.Rect(0, 0, touchscreenWidth, touchscreenHeight))
	return t.fillStrip(black)
}
