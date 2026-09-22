package modal_test

import (
	"image/color"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/modal"
	"github.com/vibrantgio/theme/tokens"
)

// TestTheDialogWearsNoRimAgainstTheWindowBehindIt pins the boundary the
// platform draws between a sheet and the window it interrupts.
//
// MEASURED, save-dialog-{light,dark}.png: the sheet runs x 165-534 and its
// leading boundary steps in ONE column from the dimmed window to the sheet's
// own fill — x 164 reads #cccccc light and #191a1b dark, x 165 reads #ffffff
// and #232a2f — with no stroke column of a third value on any of the four
// sides, in either appearance. Its top and bottom rows read the same way
// clear of the corners. So the sheet wears no rim, and what tells a dark
// dialog from the window beneath it is the scrim alone: the window keeps
// 0.74 of itself under the measured dim while the dialog keeps all of it.
//
// The test walks the row through the dialog's vertical middle, clear of both
// corners, and requires the surface's own fill on its first covered column
// and the one beside it: a rim would land a third value there.
func TestTheDialogWearsNoRimAgainstTheWindowBehindIt(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.PlatformColors
	}{
		{"light", tokens.PlatformLight},
		{"dark", tokens.PlatformDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shaper := defaultShaper(t)
			props := modal.Props{Title: decisionTitle, Body: fillRect(tc.colors.ControlAccent, 40), Shaper: shaper}
			img := golden.Capture(t, frameSize, scene(
				modal.Render(shaper, props, true, tc.colors,
					tokens.Spacing, sharpRadius, tokens.DefaultTypography.TitleMedium, tokens.Comfortable),
				tc.colors.WindowBackground))

			at := func(x, y int) color.NRGBA {
				o := y*img.Stride + x*4
				return color.NRGBA{R: img.Pix[o], G: img.Pix[o+1], B: img.Pix[o+2], A: img.Pix[o+3]}
			}
			mid := frameH / 2
			fill := tc.colors.WindowBackground
			// The first column carrying the dialog's own fill, scanning in
			// from the frame's leading edge along that row.
			lead := -1
			for x := 0; x < frameW; x++ {
				if at(x, mid) == fill {
					lead = x
					break
				}
			}
			if lead <= 0 {
				t.Fatalf("no column on row %d carries the dialog's fill %v", mid, fill)
			}
			if got := at(lead+1, mid); got != fill {
				t.Errorf("the column beside the dialog's leading edge reads %v, want its own fill %v: the dialog wears no rim", got, fill)
			}
			// The same on the trailing side.
			trail := -1
			for x := frameW - 1; x >= 0; x-- {
				if at(x, mid) == fill {
					trail = x
					break
				}
			}
			if trail <= lead {
				t.Fatalf("the dialog's trailing edge came back at %d against a leading edge at %d", trail, lead)
			}
			if got := at(trail-1, mid); got != fill {
				t.Errorf("the column beside the dialog's trailing edge reads %v, want its own fill %v: the dialog wears no rim", got, fill)
			}
			// And the window behind it keeps the measured dim rather than
			// meeting the dialog at some value of its own: the dialog is
			// told from it by the scrim.
			dimmed := at(4, mid)
			if dimmed == fill {
				t.Fatalf("the frame's leading margin reads the dialog's own fill %v", fill)
			}
			if got := at(lead-1, mid); got == fill {
				t.Errorf("the column outside the dialog's leading edge reads its fill %v", got)
			}
		})
	}
}
