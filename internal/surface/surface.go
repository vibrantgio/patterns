// Package surface draws the two edges a pattern's own surface can have: the
// card's fill, and the group's hairline — and answers what a pattern stands
// on, so every pattern in this library spells that answer the same way.
//
// The first two live together because they are the two halves of one ruling
// — a card singles something out by wearing the platform's box, a group
// divides the page by drawing a line at the surface it is already on — and a
// pattern that draws either by hand drifts from the other.
package surface

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// Card paints the platform's box: one rounded fill and nothing else. The
// platform's grouped box carries no hairline and no shadow — its edge is a
// two-to-three pixel ramp straight from the plane to the fill — so the
// caller hands in the fill and the box is that fill.
func Card(gtx layout.Context, bounds image.Rectangle, r int, fill color.NRGBA) {
	rrect := clip.RRect{Rect: bounds, SE: r, SW: r, NE: r, NW: r}
	paint.FillShape(gtx.Ops, fill, rrect.Op(gtx.Ops))
}

// Group paints the hairline a group draws at its own edge, and nothing
// else: a group takes the fill of the surface it is in, so whatever is
// already painted inside its bounds is left exactly as it was found.
//
// The line lies wholly inside those bounds. A stroke is centred on the path
// it follows, so it is drawn at twice the width under a clip of the group's
// own shape, which takes the outside half away: the group's painted
// footprint is then its bounds and not its bounds plus half a line.
func Group(gtx layout.Context, bounds image.Rectangle, r int, seam color.NRGBA) {
	rrect := clip.RRect{Rect: bounds, SE: r, SW: r, NE: r, NW: r}
	w := hairline(gtx)
	stroke := clip.Stroke{Path: rrect.Path(gtx.Ops), Width: float32(2 * w)}.Op()
	area := rrect.Push(gtx.Ops)
	paint.FillShape(gtx.Ops, seam, stroke)
	area.Pop()
}

// Or returns the surface the caller stated, or plane — the fill the pattern
// stands on unless the caller moved it — when it stated none.
//
// The platform's labels, seams, overlays and shadow carry a coverage rather
// than a colour, and a pattern flattens each against what is actually under
// it (theme/color.Flatten) before it hands Gio a fill. Which surface that is
// is the one thing a pattern cannot work out for itself, so a caller that
// put it on a card, a selected row or a chrome band says so through the
// pattern's Surface property.
//
// A surface is an opaque fill, so alpha zero is no answer rather than a
// transparent one, and alpha zero is the zero value a Surface property is
// left at.
func Or(stated, plane color.NRGBA) color.NRGBA {
	if stated.A == 0 {
		return plane
	}
	return stated
}

// hairline is one device pixel at the current scale, floored at one: a line
// the display cannot draw is not a line.
func hairline(gtx layout.Context) int {
	if w := gtx.Dp(unit.Dp(1)); w > 1 {
		return w
	}
	return 1
}

// Backdrop is the window's own plane where nothing stands on it — the
// slivers showing around an inset pane. The platform's under-page
// background carries a coverage in the light appearance, so it is flattened
// onto the window background it lies on.
func Backdrop(p tokens.PlatformColors) color.NRGBA {
	return vgcolor.Flatten(p.UnderPageBackground, p.WindowBackground)
}
