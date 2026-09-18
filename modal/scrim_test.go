package modal_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/modal"
	"github.com/vibrantgio/theme/tokens"
)

// TestScrimLandsThePlatformsByteOverAPage pins the scrim over mixed content:
// a page half white, half the platform's accent, with the scrim over the
// whole of it. Both halves must come back where the platform composites a
// coverage — on the encoded byte, a·src + (1−a)·dst — and the transition
// between them has to hold, which is what tells a per-pixel composite from a
// single fitted coverage.
//
// The bytes are arithmetic on the platform's own numbers rather than a
// reading of a capture: black at 0x33 keeps 0.8 of what is beneath, so white
// gives 204 on every channel and #007aff gives 0, 98, 204. Handed to Gio as
// a coverage the same scrim lands #e7e7e7 over the white half, which is the
// defect this composites away.
func TestScrimLandsThePlatformsByteOverAPage(t *testing.T) {
	for _, tc := range []struct {
		name         string
		colors       tokens.PlatformColors
		white, blue  color.NRGBA
		wantOverPage [2]color.NRGBA
	}{
		{
			name:   "light",
			colors: tokens.PlatformLight,
			wantOverPage: [2]color.NRGBA{
				{R: 204, G: 204, B: 204, A: 0xff},
				{R: 0, G: 98, B: 204, A: 0xff},
			},
		},
		{
			name:   "dark",
			colors: tokens.PlatformDark,
			wantOverPage: [2]color.NRGBA{
				{R: 189, G: 189, B: 189, A: 0xff},
				{R: 0, G: 90, B: 189, A: 0xff},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shaper := defaultShaper(t)
			props := modal.Props{Title: decisionTitle, Body: fillRect(tc.colors.ControlAccent, 40), Shaper: shaper}
			img := golden.Capture(t, frameSize, func(gtx layout.Context) layout.Dimensions {
				half := frameW / 2
				paint.FillShape(gtx.Ops, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
					clip.Rect(image.Rect(0, 0, half, frameH)).Op())
				paint.FillShape(gtx.Ops, color.NRGBA{R: 0x00, G: 0x7a, B: 0xff, A: 0xff},
					clip.Rect(image.Rect(half, 0, frameW, frameH)).Op())
				return modal.Render(shaper, props, true, tc.colors,
					tokens.Spacing, sharpRadius, tokens.DefaultTypography.TitleMedium, tokens.Comfortable)(gtx)
			})
			// Well clear of the dialog, which is centred and three quarters
			// of the frame wide: one sample in each half, and one pixel
			// either side of the seam between them.
			for _, s := range []struct {
				name string
				at   image.Point
				want color.NRGBA
			}{
				{"the white half", image.Pt(8, 8), tc.wantOverPage[0]},
				{"the accent half", image.Pt(frameW-8, 8), tc.wantOverPage[1]},
				{"the last white pixel", image.Pt(frameW/2-1, 8), tc.wantOverPage[0]},
				{"the first accent pixel", image.Pt(frameW/2, 8), tc.wantOverPage[1]},
			} {
				o := s.at.Y*img.Stride + s.at.X*4
				got := color.NRGBA{R: img.Pix[o], G: img.Pix[o+1], B: img.Pix[o+2], A: img.Pix[o+3]}
				if got != s.want {
					t.Errorf("%s at %v: got %v, want %v", s.name, s.at, got, s.want)
				}
			}
		})
	}
}
