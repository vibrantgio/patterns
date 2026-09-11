// Package pane provides the Patterns Pane: a chrome column set in from a
// window's leading, top and bottom edges rather than being one of them,
// rounded on all four corners, carrying its own hairline just inside that
// edge, with the backdrop showing around it on every side. Dismissed, it
// takes no width at all and what stood beside it reflows from the window's
// own leading edge.
//
// WHAT KIND OF CHROME THIS IS. A window's regions divide into the document
// and the chrome that frames it, and chrome divides again. FLUSH chrome
// runs to the window's own edge and cannot be sent away, so it takes no
// outline and a plain seam parts it from what it abuts. A PANE is the other
// kind — a control sends it out of the window, so it is an OBJECT, and the
// inset, the corner radius and the hairline are the three things that say
// so together. This package is the second kind and only the second kind.
//
// THE PANE IS READ THROUGH ITS EDGES, NOT THROUGH ITS LIGHTNESS. A pane is
// chrome, so it wears the platform's chrome material in both schemes, and
// chrome lies flat on the backdrop and has nothing to cast onto, so there is
// no shadow here. [Surface] is the fill and [SeamColor] the edge, both the
// platform's own names rather than steps of a ramp.
//
// WHY THE HAIRLINE IS DRAWN AT ALL. An inset object needs no seam where the
// backdrop showing around it does that work. On macOS 26 the backdrop does
// not do that work in the light appearance: the chrome material and the
// window's plane are both white there, so a pane set into the backdrop is
// told from it by the under-page grey alone. One drawing serves both
// schemes, so the hairline stays, and it is the platform's separator laid
// over what is beneath it.
//
// The edge is drawn INSIDE the pane's own rounded rectangle, never on the
// backdrop outside it: half a line lying on the backdrop would blur the one
// boundary a reader uses to tell where the pane stops. It is painted as two
// concentric fills rather than as a stroke, because a stroke is centred on
// the path it follows and antialiases both of its sides — a one-pixel one
// arrives as two rows of half-strength colour and the line the palette asked
// for is never actually painted.
//
// THE TOP STRIP IS DERIVED FROM THE WINDOW BUTTONS. Under a full-size
// content treatment the window's control buttons are measured from the
// window's own glass and from nothing drawn beneath them; a pane set in
// under them must be cut deep enough to hold them with the same air below
// as above. [StripDp] is that arithmetic and not a taste, and it puts the
// buttons' centre line on the strip's middle line, where a control standing
// in the strip centres — so the strip's own controls and the window's read
// as one row of chrome. [Strip] lays that band out.
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
// controls as parameters — this is chrome geometry composed inside a frame
// the application owns, not a screen-level stream of components, so it is
// laid out directly from a [layout.Context] and a palette rather than built
// from an observable. Source is intentionally short — copy it into your own
// app and modify as needed.
package pane

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/internal/surface"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// The pane's geometry. Every number here is stated once and derived from
// there; nothing in this package reads a density or a spacing step, because
// a pane's inset is a property of the window it is set into and not of how
// tightly its rows are set.
const (
	// MarginDp is the inset the pane stands off the window's leading, top
	// and bottom edges — the slivers of backdrop the reader sees around it.
	// Those slivers claim no window drag of their own: a hand aims for the
	// strip, not for an eight-dp gap, and a move action there would promise
	// a handle too thin to hit.
	MarginDp = 8

	// RadiusDp rounds the pane's four corners. The pane stands inside the
	// window rather than being its edge, so its corners are its own to
	// round — the window's, which the platform rounds, are a margin away.
	RadiusDp = 10

	// SeamDp is the width of the pane's internal hairline — drawn because
	// the backdrop's own step is too small to part the pane from it, which
	// the package comment records the measurement for. The width is the one
	// the platform's own splitters take, and the one a window's other
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
)

// Buttons is where a window that wears this pattern stands its three
// control buttons, derived from [ButtonInsetDp] by the rule the platform's
// own windows follow. Every number in it is the WINDOW's: no pane state, no
// screen and no content enters into any of them, and dismissing the pane
// moves none of them — a control that belongs to the window cannot shift
// because a pane the reader dismissed used to be behind it.
var Buttons = desktop.ButtonRunAt(ButtonInsetDp)

// Surface is the fill the pane wears: the platform's chrome material, the
// fill every sidebar, toolbar and inspector on this platform carries. It is
// a function of the set rather than a field, so that code holding a whole
// set and code holding a frame-time snapshot can name the same fill.
func Surface(c tokens.PlatformColors) color.NRGBA {
	return c.SidebarMaterial
}

// SeamColor is the colour of the pane's own edge: the platform's separator,
// black or white at a tenth, flattened onto what lies beneath the edge.
//
// What lies beneath is the backdrop, because the edge is painted as the
// outermost ring of the pane's own rectangle and the pane stands on the
// window's plane. The platform composites an alpha name in encoded sRGB, so
// the flatten is taken there and Gio is handed an opaque line.
func SeamColor(c tokens.PlatformColors) color.NRGBA {
	return vgcolor.Flatten(c.Separator, surface.Backdrop(c))
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
// set anything into. A caller reads the emptiness rather than a flag.
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
func Layout(gtx layout.Context, c tokens.PlatformColors, bounds image.Rectangle, contents layout.Widget) {
	if bounds.Empty() {
		return
	}
	r := gtx.Dp(unit.Dp(RadiusDp))
	w := max(gtx.Dp(unit.Dp(SeamDp)), 1)
	// Two concentric fills rather than a stroke, for the reason the package
	// doc gives: filling the pane in the seam's colour and filling the inset
	// pane back in over it leaves exactly one pixel of the seam's own colour
	// down every straight run, with the corners' arcs antialiased against
	// each other the way a fence's rim is drawn.
	rr := clip.RRect{Rect: bounds, NE: r, NW: r, SE: r, SW: r}
	paint.FillShape(gtx.Ops, SeamColor(c), rr.Op(gtx.Ops))
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

// FillTrailingCorners fills the strip the pane's two trailing corners round
// away from, in the fill of the region standing flush against that edge.
//
// The pane is set in from the window's leading, top and bottom edges and
// flush with what it stands beside on the fourth, so the backdrop shows on
// three sides and not on the fourth: behind a corner arc on the flush side
// stands the region the pane abuts, not the window's plane. Without this the
// two arcs read as nicks of backdrop bitten out of the boundary.
//
// Call it before [Layout], which paints over the whole strip but the arcs. A
// caller that stands the pane in the open — with the backdrop showing on all
// four sides, as the pattern's own stored images do — calls it not at all.
func FillTrailingCorners(gtx layout.Context, fill color.NRGBA, bounds image.Rectangle) {
	r := gtx.Dp(unit.Dp(RadiusDp))
	if bounds.Empty() || r <= 0 {
		return
	}
	strip := image.Rect(bounds.Max.X-r, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
	paint.FillShape(gtx.Ops, fill, clip.Rect(strip).Op())
}

// Strip lays out the pane's top band: the window control buttons' span
// skipped at the leading end, a stretch that moves the window across the
// middle, and the caller's controls at the trailing corner, one margin in
// from the pane's trailing edge.
//
// buttonsEnd is where the buttons end in WINDOW coordinates — what the
// platform reports, or the window's own edge inset where it has no such
// controls. The pane stands one margin inside the window's leading edge, so
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
