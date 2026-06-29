package viamstreamdeck

import (
	"context"
	"fmt"
	"image"
	"time"

	"go.viam.com/rdk/resource"

	"github.com/erh/vmodutils"
)

// scrollFPS is how often the scroll goroutine redraws the strip. The text
// content itself is refreshed on the slower per-second reconfigure tick.
const scrollFPS = 20

// defaultScrollSpeed is the horizontal scroll rate (px/sec) when unset.
const defaultScrollSpeed = 60

// tsLoopKey captures the touchscreen config fields that affect how the scroll
// goroutine renders. When it changes between reconfigures we restart the loop;
// when it's identical we leave the loop running and only the polled text
// updates. It excludes Text/dynamic fields, which flow through tsText instead.
type tsLoopKey struct {
	scroll    bool
	speed     int
	textColor string
	bgColor   string
	font      string
	image     string
}

func makeTsLoopKey(tc *TouchscreenConfig) tsLoopKey {
	k := tsLoopKey{
		scroll:    tc.Scroll,
		speed:     tc.ScrollSpeed,
		textColor: tc.TextColor,
		bgColor:   tc.BackgroundColor,
		image:     tc.Image,
	}
	if tc.TextFont != nil {
		k.font = *tc.TextFont
	}
	return k
}

// setText stores the latest resolved strip text for the scroll goroutine.
func (sdc *streamdeckComponent) setText(s string) {
	sdc.tsMu.Lock()
	sdc.tsText = s
	sdc.tsHasText = true
	sdc.tsMu.Unlock()
}

// currentText returns the last resolved strip text.
func (sdc *streamdeckComponent) currentText() string {
	sdc.tsMu.Lock()
	defer sdc.tsMu.Unlock()
	return sdc.tsText
}

// activeTouchscreen returns the strip config for the current page: the per-page
// entry in Touchscreens if one exists, otherwise the global Touchscreen default.
// Called with configLock held.
func (sdc *streamdeckComponent) activeTouchscreen() *TouchscreenConfig {
	if sdc.currentPage != "" {
		if tc, ok := sdc.conf.Touchscreens[sdc.currentPage]; ok && tc != nil {
			return tc
		}
	}
	return sdc.conf.Touchscreen
}

// setActiveTouchscreen stores tc back into whichever slot activeTouchscreen reads
// from, so runtime update_display edits the strip that's currently showing.
// Called with configLock held.
func (sdc *streamdeckComponent) setActiveTouchscreen(tc *TouchscreenConfig) {
	if sdc.currentPage != "" {
		if _, ok := sdc.conf.Touchscreens[sdc.currentPage]; ok {
			sdc.conf.Touchscreens[sdc.currentPage] = tc
			return
		}
	}
	sdc.conf.Touchscreen = tc
}

// resolveTouchscreenText computes the text to show for the given config. With no
// Component it returns the static Text. Otherwise it polls the component's
// DoCommand and formats the requested response field - this is what makes the
// strip dynamic like the buttons. On any failure it keeps the last good text so
// a transient error doesn't blank the strip. Called with configLock held.
func (sdc *streamdeckComponent) resolveTouchscreenText(ctx context.Context, tc *TouchscreenConfig) string {
	if tc.Component == "" {
		return tc.Text
	}

	var r resource.Resource
	if sdc.isSelfReference(tc.Component) {
		r = sdc
	} else {
		var ok bool
		r, ok = vmodutils.FindDep(sdc.deps, tc.Component)
		if !ok {
			sdc.logger.Warnf("touchscreen component %s not found", tc.Component)
			return sdc.currentText()
		}
	}

	res, err := r.DoCommand(ctx, tc.Request)
	if err != nil {
		sdc.logger.Warnf("touchscreen DoCommand for %s failed: %v", tc.Component, err)
		return sdc.currentText()
	}

	var val interface{}
	if tc.Field != "" {
		val = res[tc.Field]
	} else {
		val = res
	}
	return formatTouchscreenText(tc, val)
}

// formatTouchscreenText renders val per the config: Format verb if set, else the
// static Text as a prefix, else the bare value.
func formatTouchscreenText(tc *TouchscreenConfig, val interface{}) string {
	if tc.Format != "" {
		return fmt.Sprintf(tc.Format, val)
	}
	if tc.Text != "" {
		return fmt.Sprintf("%s%v", tc.Text, val)
	}
	return fmt.Sprintf("%v", val)
}

// updateTouchscreen resolves the current text and renders the strip. For static
// (non-scroll) config it draws one frame per call, so the per-second reconfigure
// tick keeps it live. For scrolling config it manages a goroutine, only
// (re)starting it when the rendering config changes. Called with configLock held.
func (sdc *streamdeckComponent) updateTouchscreen(ctx context.Context) error {
	if sdc.ts == nil {
		return nil
	}

	tc := sdc.activeTouchscreen()
	if tc == nil {
		// No strip config for this page: stop any loop and blank the strip.
		sdc.stopScroll()
		if sdc.tsHasText {
			sdc.tsHasText = false
			return sdc.ts.clear()
		}
		return nil
	}

	sdc.setText(sdc.resolveTouchscreenText(ctx, tc))

	key := makeTsLoopKey(tc)
	var bg image.Image
	if tc.Image != "" {
		bg = assetImages[tc.Image]
	}

	if !tc.Scroll {
		sdc.stopScroll()
		sdc.tsApplied = key
		img := renderStrip(sdc.currentText(), tc.TextColor, tc.BackgroundColor, tc.TextFont, bg, false, 0)
		return sdc.ts.fillStrip(img)
	}

	// Scrolling: keep the running loop if nothing relevant changed.
	if sdc.tsCancel != nil && key == sdc.tsApplied {
		return nil
	}

	sdc.stopScroll()
	sdc.tsApplied = key

	loopCtx, cancel := context.WithCancel(context.Background())
	sdc.tsCancel = cancel
	go sdc.runScroll(loopCtx, tc.TextColor, tc.BackgroundColor, tc.TextFont, bg, tc.ScrollSpeed)
	return nil
}

// runScroll redraws the strip at scrollFPS, advancing the horizontal offset and
// always rendering the latest polled text.
func (sdc *streamdeckComponent) runScroll(ctx context.Context, textColor, bgColor string, fontName *string, bg image.Image, speed int) {
	if speed <= 0 {
		speed = defaultScrollSpeed
	}
	pxPerFrame := speed / scrollFPS
	if pxPerFrame < 1 {
		pxPerFrame = 1
	}

	ticker := time.NewTicker(time.Second / scrollFPS)
	defer ticker.Stop()

	offset := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if sdc.closed.Load() != 0 {
				return
			}
			img := renderStrip(sdc.currentText(), textColor, bgColor, fontName, bg, true, offset)
			if err := sdc.ts.fillStrip(img); err != nil {
				sdc.logger.Warnf("touchscreen scroll write failed: %v", err)
			}
			offset += pxPerFrame
		}
	}
}

// stopScroll cancels the scroll goroutine if running. Called with configLock held.
func (sdc *streamdeckComponent) stopScroll() {
	if sdc.tsCancel != nil {
		sdc.tsCancel()
		sdc.tsCancel = nil
	}
}

// closeTouchscreen stops the scroll loop and blanks the strip. The HID handle
// itself is owned by the streamdeck library and closed via sdc.sd.Close(). Safe
// to call when the touchscreen was never active.
func (sdc *streamdeckComponent) closeTouchscreen() error {
	if sdc.ts == nil {
		return nil
	}
	sdc.configLock.Lock()
	sdc.stopScroll()
	sdc.configLock.Unlock()

	return sdc.ts.clear()
}
