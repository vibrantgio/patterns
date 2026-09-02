// Package pane provides the Patterns Floating Pane: a chrome column that
// floats just inside a window's leading, top and bottom edges rather than
// being one of them, rounded on all four corners, carrying its own hairline
// just inside that edge, with the window's ground showing around it on
// every side. Dismissed, it takes no width at all and what stood beside it
// reflows from the window's own leading edge.
//
// WHAT KIND OF FURNITURE THIS IS. A window's regions divide into document
// and furniture, and its furniture divides again. INTEGRAL furniture is
// fixed and flush: it cannot be sent away, so it takes no outline and its
// boundary is a plain seam. A FLOATING PANE is the other kind — a control
// slides it out of the window, so it is an OBJECT, and the inset, the
// corner radius and the internal hairline are the three things that say so
// together. This package is the second kind and only the second kind.
//
// ELEVATION IS READ THROUGH THE EDGE, NOT THROUGH LIGHTNESS. A pane is the
// window's furniture, so its level is the CHROME level — one measured step
// toward the scheme's dark extreme in both schemes — and the pane is
// DARKER than the document beside it and stays darker for being
// dismissible. A pane does not climb the levels by leaving the wall, and
// the chrome level's elevation is zero dp: chrome lies flat on the
// backdrop and has nothing to cast onto, so there is no shadow here and the
// edge does the whole of the work. [Surface] is the fill and [SeamInk] the edge, both
// derived from the palette rather than named as rungs.
//
// The edge is drawn INSIDE the pane's own rounded rectangle, never on the
// ground outside it: half a line lying on the window's ground would blur
// the one boundary a reader uses to tell where the pane stops. It is
// painted as two concentric fills rather than as a stroke, because a
// stroke is centred on the path it follows and antialiases both of its
// sides — a one-pixel one arrives as two rows of half-strength ink and the
// line the palette asked for is never actually painted.
//
// THE TOP STRIP IS DERIVED FROM THE WINDOW BUTTONS. Under a full-size
// content treatment the window's control buttons are measured from the
// window's own glass and from nothing drawn beneath them; a pane that
// floats under them must be cut deep enough to hold them with the same air
// below as above. [StripDp] is that arithmetic and not a taste, and it puts
// the buttons' centre line on the strip's middle line, where a control
// standing in the strip centres — so the strip's own controls and the
// window's read as one row of furniture. [Strip] lays that band out.
//
// THE RECALL CONVENTION. A control that travels with the pane cannot be the
// one that recalls it. The pane's own dismiss control rides the strip; the
// control that brings the pane back must stand somewhere that survives the
// pane — the window's own chrome row — and the two are the two halves of
// one switch rather than duplicates of one control, so they wear one figure
// and stand at one height in both states. This package draws neither: which
// figure, which label and where the recalling half stands are the window's
// business. What the package fixes is the geometry both halves stand on.
//
// WHAT THE CALLER SUPPLIES. The pane takes its contents, its width and its
// controls as parameters — this is furniture geometry composed inside a
// frame the application owns, not a screen-level widget stream, so it is
// laid out directly from a [layout.Context] and a palette rather than built
// from an observable. Source is intentionally short — copy it into your own
// app and modify as needed.
package pane

import (
	"image"
	"image/color"
	"math"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/mvu/desktop"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// The pane's geometry. Every number here is stated once and derived from
// there; nothing in this package reads a density or a spacing step, because
// a pane's float is a property of the window it floats in and not of how
// tightly its rows are set.
const (
	// MarginDp is the inset the pane floats off the window's leading, top
	// and bottom edges — the slivers of ground the reader sees around it.
	// Those slivers claim no window drag of their own: a hand aims for the
	// strip, not for an eight-dp gap, and a move action there would promise
	// a handle too thin to hit.
	MarginDp = 8

	// RadiusDp rounds the pane's four corners. The pane floats inside the
	// window rather than being its edge, so its corners are its own to
	// round — the window's, which the platform rounds, are a margin away.
	RadiusDp = 10

	// SeamDp is the width of the pane's internal hairline: the width the
	// platform's own split dividers take and the width a window's other
	// chrome boundaries should take beside it, so that boundaries drawn for
	// different reasons are still drawn at one weight. Wider is worse in a
	// way that is easy to miss — a seam runs a whole edge, so its width is
	// the width of the scar it leaves across everything it crosses.
	SeamDp = 1

	// ButtonInsetDp is how far the window control buttons sit in from the
	// window's own top and leading glass: the drawn circles' own edges,
	// equal on both axes, measured from the glass and from nothing else.
	// The number is the platform's, read off its sidebar apps — Finder,
	// Mail, Notes and Voice Memos all draw the circles nineteen pixels in
	// from both edges, which at one pixel per dp is nineteen. It is not what
	// the toolkit does left alone (unasked, the buttons land at nine, the
	// inset the platform's compact windows use), so a window that wants this
	// placement states it rather than defaulting to it.
	ButtonInsetDp = 19

	// StripDp is the pane's own top strip: deep enough to hold the buttons
	// where the window puts them with the same air below them as above. The
	// buttons' inset is measured from the glass and the strip from the
	// pane's own edge, so the strip owes the margin back at both ends —
	// which lands the buttons' centre line on the strip's own middle.
	StripDp = 2*(ButtonInsetDp-MarginDp) + desktop.WindowButtonDiameter

	// SeamRatio is how far the pane's edge stands from the fill it is drawn
	// on: 1.51:1, and it is a MEASUREMENT of the platform rather than a
	// floor anything has to clear. The platform draws this edge and draws it
	// quietly — Voice Memos outlines its floating panel at #3A3A3A on a
	// #1B1B1B panel, 1.514:1, while the flush side of the same window
	// carries no outline at all. That number is deliberately NOT the 3:1
	// graphic floor an object's outline derives to elsewhere in this system,
	// because the two lines are not the same kind of thing: a 3:1 mark
	// carries meaning by itself and owes its ground WCAG 1.4.11, while a
	// pane's own edge is a decorative seam saying "this region is an
	// object", read alongside the fill, the inset and the radius that say
	// the same thing. On these grounds 3:1 would answer ink far louder than
	// anything the platform draws around a sidebar.
	SeamRatio = 1.51
)

// Buttons is where a window that wears this pattern stands its three
// control buttons, derived from [ButtonInsetDp] by the rule the platform's
// own windows follow. Every number in it is the WINDOW's: no pane state, no
// screen and no content enters into any of them, and dismissing the pane
// moves none of them — a control that belongs to the window cannot shift
// because a pane the reader dismissed used to be behind it.
var Buttons = desktop.ButtonRunAt(ButtonInsetDp)

// Surface is the fill the pane wears: the CHROME level, one step under the
// content toward the scheme's dark extreme in both schemes. It is a
// function of the palette rather than a field, so that code holding a whole
// palette and code holding a frame-time snapshot can name the same fill.
func Surface(c tokens.ColorTokens) color.NRGBA {
	return c.SurfaceAt(tokens.LevelChrome)
}

// SeamInk is the ink of the pane's own edge, resolved against the fill it
// is drawn on rather than named as a rung.
//
// Two things are derived and neither names a scheme. The DISTANCE is
// [SeamRatio], solved in the luminance a contrast ratio is taken in and
// realized at the fill's own hue and chroma, the way elevation realizes a
// level — so the edge carries whatever tint the palette carries and none
// of its own. The DIRECTION is toward the scheme's own ink: a dark scheme's
// edge is lighter than its pane, as the platform draws it, and a light
// scheme's is darker, which is the only direction a light pane has room in
// — from a #E8E8E8 floor the whole distance left to white is 1.23:1, less
// than the whisper itself.
//
// On the default palettes it answers #BEBEBE on the light floor, 1.52:1,
// and #363636 on the dark one, 1.51:1 — the dark pairing within a level of
// the platform's own #3A3A3A on #1B1B1B.
func SeamInk(c tokens.ColorTokens) color.NRGBA {
	fill := Surface(c)
	y := vgcolor.RelativeLuminance(fill)
	target := SeamRatio*(y+0.05) - 0.05
	if inkL, fillL := lightness(c.Text), lightness(fill); inkL < fillL {
		target = (y+0.05)/SeamRatio - 0.05
	}
	target = min(max(target, 0), 1)
	_, chroma, hue := vgcolor.OKLChFromNRGBA(fill)
	return vgcolor.NRGBAFromToneChromaHue(tone(target), chroma, hue)
}

// lightness is a colour's CIELAB L*, which is what "toward the ink"
// compares: the seam's direction is a question about lightness and nothing
// else.
func lightness(c color.NRGBA) float64 {
	l, _, _ := vgcolor.LabFromNRGBA(c)
	return l
}

// tone is the CIELAB lightness of a relative luminance — the inverse of the
// Y a WCAG contrast ratio is taken on. A distance stated as a ratio is
// solved in Y; the toolkit realizes a colour from a tone, a chroma and a
// hue; this is the one step between them.
func tone(y float64) float64 {
	if y <= 216.0/24389.0 {
		return y * 24389.0 / 27.0
	}
	return 116*math.Cbrt(y) - 16
}

// Bounds answers the pane's rectangle in the coordinates of a window of the
// given size: one [MarginDp] inside its leading, top and bottom edges, as
// wide as width asks for. It is separate from the drawing so that a frame
// can measure its arrangement — where its content begins, whether the pane
// fits at all — without laying anything out.
//
// The rectangle is EMPTY in the states where there is no pane to draw:
// hidden, which is the whole of the hidden contract (the pane takes no
// width at all and the caller lays its content out from the window's own
// leading edge, rather than the pane collapsing to a rail that still has
// to be reasoned about), a window with no area, and a window too small to
// float anything in. A caller reads the emptiness rather than a flag.
//
// The pane and its margin may never take more than half the window: a
// narrow window owes its document a readable column before it owes the
// pane its width.
func Bounds(gtx layout.Context, size image.Point, width unit.Dp, hidden bool) image.Rectangle {
	if hidden || size.X <= 0 || size.Y <= 0 {
		return image.Rectangle{}
	}
	margin := gtx.Dp(unit.Dp(MarginDp))
	w := gtx.Dp(width)
	if maxW := size.X/2 - margin; w > maxW {
		w = maxW
	}
	if w <= 0 || size.Y <= 2*margin {
		return image.Rectangle{}
	}
	return image.Rect(margin, margin, margin+w, size.Y-margin)
}

// Layout draws the pane at bounds — its own edge, then its fill — and lays
// contents inside it at the pane's full size.
//
// The contents are clipped to the FILL rather than to the boundary, so a
// scrolled row that runs the pane's full width can neither cross an edge,
// poke through a corner, nor paint over the edge that says the pane is an
// object. An empty rectangle draws nothing, which is the dismissed state; a
// nil contents draws the pane and nothing in it.
func Layout(gtx layout.Context, c tokens.ColorTokens, bounds image.Rectangle, contents layout.Widget) {
	if bounds.Empty() {
		return
	}
	r := gtx.Dp(unit.Dp(RadiusDp))
	w := max(gtx.Dp(unit.Dp(SeamDp)), 1)
	// Two concentric fills rather than a stroke, for the reason the package
	// doc gives: filling the pane in the seam's ink and filling the inset
	// pane back in over it leaves exactly one pixel of the seam's own colour
	// down every straight run, with the corners' arcs antialiased against
	// each other the way a fence's rim is drawn.
	rr := clip.RRect{Rect: bounds, NE: r, NW: r, SE: r, SW: r}
	paint.FillShape(gtx.Ops, SeamInk(c), rr.Op(gtx.Ops))
	inner := clip.RRect{Rect: bounds.Inset(w), NE: max(r-w, 0), NW: max(r-w, 0), SE: max(r-w, 0), SW: max(r-w, 0)}
	paint.FillShape(gtx.Ops, Surface(c), inner.Op(gtx.Ops))
	if contents == nil {
		return
	}
	defer inner.Push(gtx.Ops).Pop()
	defer op.Offset(bounds.Min).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(bounds.Size())
	contents(gtx)
}

// Strip lays out the pane's top band: the window control buttons' span
// skipped at the leading end, a stretch that moves the window across the
// middle, and the caller's controls at the trailing corner, one margin in
// from the pane's trailing edge.
//
// buttonsEnd is where the buttons end in WINDOW coordinates — what the
// platform reports, or the window's own edge inset where it has no such
// controls. The pane floats one margin inside the window's leading edge, so
// the pane-local skip is that measurement less the margin: the buttons are
// the window's and stand where it puts them; it is the pane that slid in
// under them. The span is skipped rather than claimed because a move action
// declared over the buttons would fight them for the press.
//
// The controls are handed over in reading order and each takes its own
// width; a caller wanting air between two of them passes a spacer between
// them. The band's depth is the constraint the caller gives it, which
// should be [StripDp] — the strip is reserved by the pane's own vertical
// arrangement, and whether it is drawn in place or after the rest is a
// question about focus order that belongs to the caller.
func Strip(gtx layout.Context, buttonsEnd unit.Dp, controls ...layout.Widget) layout.Dimensions {
	lead := buttonsEnd - MarginDp
	if lead < 0 {
		lead = 0
	}
	children := make([]layout.FlexChild, 0, len(controls)+3)
	children = append(children,
		layout.Rigid(complayout.HSpacer(float32(lead))),
		layout.Flexed(1, DragFill))
	for _, w := range controls {
		children = append(children, layout.Rigid(w))
	}
	children = append(children, layout.Rigid(DragSpacer(MarginDp)))
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

// DragSpacer is a fixed-width gap that moves the window when it is dragged.
// A pane that owns the top of the window stands where the native title bar
// would otherwise be, and under a full-size content treatment that strip
// hands over no drag of its own — so the window's top edge is a handle only
// where the pane says it is, and it may say so over its empty space alone,
// since a move action swallows the press before any control beneath it sees
// one.
func DragSpacer(w unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return desktop.DragRun(gtx, gtx.Dp(w))
	}
}

// DragFill is the strip's flexible middle: everything between the buttons
// and the trailing controls, draggable end to end.
func DragFill(gtx layout.Context) layout.Dimensions {
	return desktop.DragRun(gtx, gtx.Constraints.Min.X)
}
