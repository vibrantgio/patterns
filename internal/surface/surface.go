// Package surface draws the two edges a pattern's own surface can have: the
// card's raise, and the group's hairline.
//
// They live together because they are the two halves of one ruling — a card
// singles something out by standing a step above what it is in, a group
// divides the page by drawing a line at the level it is already at — and a
// pattern that draws either by hand drifts from the other.
package surface

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/theme/tokens"
)

// Card paints a rounded surface raised on what it stands on: the raise's
// own fill, plus the seam that raise owes where the scheme has no lighter
// step left to tell it with. Nothing else — a card is raised, not floating,
// so it casts no shadow, and it is never outlined.
//
// The seam is painted as the card's own rectangle in the seam colour with
// the fill laid back over it one pixel in, rather than as a stroke: a
// stroke is centred on the edge it follows, so half of it would land
// outside the card and the card's painted footprint would depend on which
// scheme was running.
func Card(gtx layout.Context, bounds image.Rectangle, r int, raise tokens.Raise) {
	rrect := clip.RRect{Rect: bounds, SE: r, SW: r, NE: r, NW: r}
	if !raise.Seamed {
		paint.FillShape(gtx.Ops, raise.Fill, rrect.Op(gtx.Ops))
		return
	}
	w := hairline(gtx)
	paint.FillShape(gtx.Ops, raise.Seam, rrect.Op(gtx.Ops))
	ir := r - w
	if ir < 0 {
		ir = 0
	}
	inner := clip.RRect{Rect: bounds.Inset(w), SE: ir, SW: ir, NE: ir, NW: ir}
	paint.FillShape(gtx.Ops, raise.Fill, inner.Op(gtx.Ops))
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

// hairline is one device pixel at the current scale, floored at one: a line
// the display cannot draw is not a line.
func hairline(gtx layout.Context) int {
	if w := gtx.Dp(unit.Dp(1)); w > 1 {
		return w
	}
	return 1
}
