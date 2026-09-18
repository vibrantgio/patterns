package modal_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/modal"
	"github.com/vibrantgio/theme/tokens"
)

// TestModalStandsAboveADeferredShadow holds the floating level's order
// against the one thing that can quietly break it.
//
// A bordered toolbar control's drop shadow falls outside the control's own
// box, so it is drawn in the band's own pass — recorded and handed to
// op.Defer, which runs it after every column of the window has laid out. A
// modal painted in place would land UNDER that pass whatever the reading
// order said, and the shadow would be drawn across the modal's opaque
// surface. Ruling of 2026-09-18, the Level entry: the floating level stands
// above everything in the window, so the modal defers as the popover, the
// tooltip and the menu do, and deferred ops run in the order they were
// deferred.
//
// The proof is a pixel the shadow reaches that the modal's surface covers:
// with the modal open it must read exactly what it reads with no toolbar
// control in the window at all. Both appearances, the platform answering its
// shadow per appearance.
func TestModalStandsAboveADeferredShadow(t *testing.T) {
	shaper := defaultShaper(t)

	// The control sits inside the modal's surface, which is 3/4 of the frame
	// across and centred: x 40-280 of a 320 px frame. Its shadow reaches
	// below it, so the row read is under the control and still well inside
	// the surface.
	const ctlX, ctlY = 120, 96

	for _, sc := range []struct {
		name   string
		colors tokens.PlatformColors
		bg     color.NRGBA
	}{
		{"light", tokens.PlatformLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
		{"dark", tokens.PlatformDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
	} {
		colors := sc.colors
		window := func(withControl, withModal bool) layout.Widget {
			return func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, sc.bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
				if withControl {
					// A band with one bordered control in it, which casts
					// the platform's drop shadow through op.Defer.
					paint.FillShape(gtx.Ops, colors.SidebarMaterial,
						clip.Rect{Min: image.Pt(0, ctlY-8), Max: image.Pt(frameW, ctlY+48)}.Op())
					st := op.Offset(image.Pt(ctlX, ctlY)).Push(gtx.Ops)
					gtx.Constraints.Min = image.Point{}
					button.RenderChrome(nil, colors, tokens.Comfortable,
						button.RenderState{Surface: colors.SidebarMaterial})(gtx)
					st.Pop()
				}
				w := modal.Render(shaper, modal.Props{Title: panelTitle, Shaper: shaper},
					withModal, colors, tokens.Spacing, sharpRadius,
					tokens.DefaultTypography.TitleMedium, tokens.Comfortable)
				return w(gtx)
			}
		}

		// Under the control, inside its shadow's reach and well inside the
		// modal's surface.
		band := image.Rect(ctlX, ctlY+36, ctlX+40, ctlY+44)
		differ := func(a, b *image.RGBA) int {
			n := 0
			for y := band.Min.Y; y < band.Max.Y; y++ {
				for x := band.Min.X; x < band.Max.X; x++ {
					if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
						n++
					}
				}
			}
			return n
		}

		bare := golden.Capture(t, frameSize, window(true, false))
		bareNoCtl := golden.Capture(t, frameSize, window(false, false))
		withCtl := golden.Capture(t, frameSize, window(true, true))
		without := golden.Capture(t, frameSize, window(false, true))
		if bare == nil || bareNoCtl == nil || withCtl == nil || without == nil {
			return // headless unavailable; Capture called t.Skip
		}

		// The shadow is actually there: without the modal, those pixels are
		// the control's shadow and not the bare band.
		if n := differ(bare, bareNoCtl); n == 0 {
			t.Fatalf("%s: the control casts no shadow over %v; the claim below would hold vacuously", sc.name, band)
		}
		if n := differ(withCtl, without); n > 0 {
			t.Errorf("%s: %d pixel(s) of the modal's surface carry the toolbar control's shadow; the modal is not on the floating level",
				sc.name, n)
		}
	}
}

// TestFocusHaloInsideAModalIsDrawn holds the other half of the modal's
// deferral: the modal's own paint is deferred, and a focused control inside
// it defers its halo too, so the halo is a defer recorded while a deferred
// macro runs. Gio appends those to the same pass and runs them after it, but
// nothing in this library would notice if it stopped — a focused control
// would simply lose its halo inside a modal and keep it everywhere else.
//
// The proof: a focused button in the modal's body draws pixels an unfocused
// one does not.
func TestFocusHaloInsideAModalIsDrawn(t *testing.T) {
	shaper := defaultShaper(t)
	colors := tokens.PlatformLight

	scene := func(focused bool) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, color.NRGBA{R: 240, G: 240, B: 240, A: 255},
				clip.Rect{Max: gtx.Constraints.Max}.Op())
			body := button.Render(shaper, "Save", colors, tokens.Spacing, sharpRadius,
				tokens.DefaultTypography.LabelLarge, tokens.Comfortable,
				button.RenderState{Focused: focused, Surface: colors.WindowBackground})
			w := modal.Render(shaper, modal.Props{Title: panelTitle, Body: body, Shaper: shaper},
				true, colors, tokens.Spacing, sharpRadius,
				tokens.DefaultTypography.TitleMedium, tokens.Comfortable)
			return w(gtx)
		}
	}

	focused := golden.Capture(t, frameSize, scene(true))
	resting := golden.Capture(t, frameSize, scene(false))
	if focused == nil || resting == nil {
		return // headless unavailable; Capture called t.Skip
	}
	if n := golden.PixelDiff(focused, resting); n == 0 {
		t.Error("a focused button inside a modal draws exactly what an unfocused one draws; the halo did not survive the modal's own defer")
	}
}
