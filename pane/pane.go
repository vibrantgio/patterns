// Package pane provides the Patterns Pane: the chrome column down a
// window's leading edge, running flush to the window's leading, top and
// bottom edges, parted from what it stands beside by one seam. Dismissed,
// it takes no width at all and what stood beside it reflows from the
// window's own leading edge.
//
// WHAT KIND OF CHROME THIS IS. A window's regions divide into the document
// and the chrome that frames it. This column is FLUSH chrome: it runs to
// the window's own edges, so it takes no outline of its own and a plain
// seam parts it from what it abuts. Nothing is set into anything here —
// there is no margin around the column, no rounded corner and no hairline
// around three sides of it.
//
// MEASURED, finder-window-untinted-light.png and
// finder-window-untinted-dark.png: Finder's sidebar fill runs from the
// window's own opaque bounds to x=351 light, where the content's #ffffff
// begins at x=352 with no hairline between them in 112 of 114 sampled
// rows; dark, one pixel of #434343 stands at x=373 between the rail and
// the content. Above and below, the fill runs to the window's edges;
// what light shows between the window's bound and the fill is the eight
// pixel rim macOS 26 draws inside the frame, which is the platform's and
// not the application's to paint. There is no plane around the column and
// no object set into one.
//
// THE COLUMN IS READ THROUGH ITS ONE SEAM. A pane is chrome, so it wears
// the platform's chrome material in both schemes, and chrome lies flat on
// the backdrop and has nothing to cast onto, so there is no shadow here.
// [Surface] is the fill and [SeamColor] the one line, both the platform's
// own names. The seam is drawn INSIDE the column's own trailing edge, by
// the region leading, as the Language's seam says: one line, drawn once,
// by whoever is above or leading. That the line is drawn at all is the
// Language's rule and not the measurement above, which finds none in light
// and a value with no platform name in dark; the disagreement is filed, and
// the rule stands until it is ruled on.
//
// THE TOP STRIP IS DERIVED FROM THE WINDOW BUTTONS. Under a full-size
// content treatment the window's control buttons are measured from the
// window's own glass and from nothing drawn beneath them; a column running
// under them must be cut deep enough to hold them with the same air below
// as above. [StripDp] is that arithmetic and not a taste, and it puts the
// buttons' centre line on the strip's middle line, where a control standing
// in the strip centres — so the strip's own controls and the window's read
// as one row of chrome.
//
// THE RECALL CONVENTION. A control that travels with the column cannot be
// the one that recalls it. The column's own dismiss control rides the
// strip; the control that brings it back must stand somewhere that survives
// it — the window's own chrome row — and the two are the two halves of one
// switch rather than duplicates of one control, so they wear one figure and
// stand at one height in both states. This package draws neither: which
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
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// The pane's geometry. Every number here is stated once and derived from
// there; nothing in this package reads a density or a spacing step, because
// a chrome column's geometry is a property of the window it is an edge of
// and not of how tightly its rows are set.
const (
	// MarginDp is the air the top strip keeps at its trailing end, between
	// the last control standing in it and the column's trailing edge. The
	// column itself stands off nothing: it runs to the window's leading, top
	// and bottom edges, which the package comment records the measurement
	// for.
	MarginDp = 8

	// SeamDp is the width of the one line this column draws: its trailing
	// edge, where the document stands flush against it. The width is the one
	// the platform's own splitters take, and the one a window's other chrome
	// boundaries should take beside it, so that boundaries drawn for
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

	// StripDp is the column's own top strip: deep enough to hold the buttons
	// where the window puts them with the same air below them as above. The
	// column starts at the window's own top edge and the buttons are
	// measured from that same edge, so the strip owes nothing back at either
	// end — which lands the buttons' centre line on the strip's own middle.
	StripDp = 2*ButtonInsetDp + desktop.WindowButtonDiameter
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

// SeamColor is the colour of the one line this column draws: the
// platform's separator, black or white at a tenth, flattened onto what
// lies beneath the line.
//
// What lies beneath is the column's own fill, because the line is painted
// as the innermost strip of the column's own rectangle and the region
// beside it is flush against that edge. The platform composites an alpha
// name in encoded sRGB, so the flatten is taken there and Gio is handed an
// opaque line.
func SeamColor(c tokens.PlatformColors) color.NRGBA {
	return vgcolor.Flatten(c.Separator, Surface(c))
}

// Bounds answers the column's rectangle in the coordinates of a window of
// the given size: the window's own leading, top and bottom edges, as wide
// as width asks for. It is separate from the drawing so that a frame can
// measure its arrangement — where its content begins, whether the column
// fits at all — without laying anything out.
//
// The rectangle is EMPTY in the states where there is no column to draw:
// hidden, which is the whole of the hidden contract (the column takes no
// width at all and the caller lays its content out from the window's own
// leading edge, rather than collapsing to a rail that still has to be
// reasoned about), a window with no area, and a window too small to hold
// one. A caller reads the emptiness rather than a flag.
//
// The column may never take more than half the window: a narrow window owes
// its document a readable column before it owes the chrome its width.
func Bounds(gtx layout.Context, size image.Point, width unit.Dp, hidden bool) image.Rectangle {
	if hidden || size.X <= 0 || size.Y <= 0 {
		return image.Rectangle{}
	}
	w := gtx.Dp(width)
	if maxW := size.X / 2; w > maxW {
		w = maxW
	}
	if w <= 0 {
		return image.Rectangle{}
	}
	return image.Rect(0, 0, w, size.Y)
}

// Layout draws the column at bounds — its fill, then the one seam down its
// trailing edge — and lays contents inside it at the column's full size.
//
// The contents are clipped to the column, so a scrolled row that runs its
// full width cannot paint over the seam that says where the column stops.
// The seam is drawn last for the same reason: a full-width row, selected or
// not, may not erase it. An empty rectangle draws nothing, which is the
// dismissed state; a nil contents draws the column and nothing in it.
func Layout(gtx layout.Context, c tokens.PlatformColors, bounds image.Rectangle, contents layout.Widget) {
	if bounds.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, Surface(c), clip.Rect(bounds).Op())
	if contents != nil {
		area := clip.Rect(bounds).Push(gtx.Ops)
		st := op.Offset(bounds.Min).Push(gtx.Ops)
		cgtx := gtx
		cgtx.Constraints = layout.Exact(bounds.Size())
		contents(cgtx)
		st.Pop()
		area.Pop()
	}
	PaintSeam(gtx, c, bounds)
}

// PaintSeam draws the column's one line: a hairline down the inside of its
// trailing edge, in [SeamColor]. It is exported so a frame that draws the
// column's contents itself still draws the boundary this pattern owns, and
// so a splitter riding that boundary can find the pixel it stands on.
func PaintSeam(gtx layout.Context, c tokens.PlatformColors, bounds image.Rectangle) {
	w := max(gtx.Dp(unit.Dp(SeamDp)), 1)
	if bounds.Dx() <= w || bounds.Dy() <= 0 {
		return
	}
	edge := image.Rect(bounds.Max.X-w, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
	paint.FillShape(gtx.Ops, SeamColor(c), clip.Rect(edge).Op())
}

// Strip lays out the pane's top band: the window control buttons' span
// skipped at the leading end, a stretch that moves the window across the
// middle, and the caller's controls at the trailing corner, one margin in
// from the pane's trailing edge.
//
// buttonsEnd is where the buttons end in WINDOW coordinates — what the
// platform reports, or the window's own edge inset where it has no such
// controls. The column starts at the window's own leading edge, so the
// column-local skip is that measurement unchanged. The span is skipped
// rather than claimed because a move action declared over the buttons would
// fight them for the press.
//
// The controls are handed over in reading order and each takes its own
// width; a caller wanting air between two of them passes a spacer between
// them. The band's depth is the constraint the caller gives it, which
// should be [StripDp] — the strip is reserved by the pane's own vertical
// arrangement, and whether it is drawn in place or after the rest is a
// question about focus order that belongs to the caller.
func Strip(gtx layout.Context, buttonsEnd unit.Dp, controls ...layout.Widget) layout.Dimensions {
	lead := buttonsEnd
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
