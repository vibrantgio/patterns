package shell

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/tokens"
)

// PaneWidthDp is the width a leading pane takes when a caller names none:
// the sidebar panel's own column, rim to rim. It is patterns/sidebar's
// ExpandedWidth and not a second statement of it — that package took the
// reading and owns it, and a pane frame standing a sidebar in its panel
// cannot be a column wider or narrower than the column it holds.
const PaneWidthDp = sidebar.ExpandedWidth

// PaneFrame is the window composition a leading pane makes: the window's own
// plane under everything, the panel set one [pane.MarginDp] inside the
// window's leading, top and bottom edges with the platform's rim and the
// shadow it casts, the content column flush against the panel's trailing
// side, a band [pane.BandDp] deep across that column's top and the main
// content under it.
//
// The band is the PLATFORM's and not a density's: the window's three control
// buttons stand a measured inset in from the glass and the band that holds
// them centred follows from that one number, which patterns/pane states and
// this frame reads. The panel's own top strip is cut from the same inset one
// margin higher, so a control centred in the band and one centred in the
// strip stand on one line.
//
// WHAT THE FRAME DRAWS AND WHAT THE WINDOW DRAWS. The frame owns the
// geometry every window with a pane shares and nothing else: which fills
// stand where, what the panel is, how deep the band is and when the shadow is
// cast. What stands IN the band, what the column beside the panel holds and
// what the panel's column shows are the window's, handed in as the three
// slots. A window whose columns are arranged otherwise — a trailing aside, a
// status bar along the foot, a document laid out before the band above it —
// keeps its own arrangement and hands over [PaneFrame.Under] alone, which is
// the part every such window passes through unchanged.
type PaneFrame struct {
	// Width is the panel's own width, rim to rim. Zero means [PaneWidthDp].
	Width unit.Dp

	// Hidden sends the panel out of the window: it takes no width at all
	// and the content column reflows from the window's own leading edge.
	Hidden bool

	// Plane is the window's own plane, painted under everything — the fill
	// that shows in the margins around the panel. A window that already
	// stands on its plane, painting it in a layer beneath this frame, leaves
	// it zero and the frame paints none.
	Plane color.NRGBA

	// ContentFill is the content column's own surface, painted from the
	// panel's trailing edge to the window's and running its full height. It
	// is also what stands behind the two corners the panel rounds away from
	// on its flush side, so that neither reads as a nick of plane bitten out
	// of the boundary.
	ContentFill color.NRGBA

	// BandFill is what stands behind those two corners over the band's own
	// rows, for a window whose band paints a fill of its own across the
	// content column. Zero leaves the whole flush side standing on
	// ContentFill, which is the arrangement of a band that carries no fill.
	BandFill color.NRGBA

	// Sidebar is the column standing inside the panel. It is laid out at the
	// panel's full size and clipped to it.
	Sidebar layout.Widget

	// Band is what the content column carries across its top, laid out at
	// [pane.BandDp] deep.
	Band layout.Widget

	// Main is the content column under the band.
	Main layout.Widget
}

// Bounds answers the panel's rectangle in the coordinates of a window of the
// given size, empty in every state where there is no panel to draw. It is
// separate from the drawing so that a window can measure its own arrangement
// — where its content column begins, whether the panel fits at all — without
// laying anything out.
func (f PaneFrame) Bounds(gtx layout.Context, size image.Point) image.Rectangle {
	w := f.Width
	if w <= 0 {
		w = PaneWidthDp
	}
	return pane.Bounds(gtx, size, w, f.Hidden)
}

// ContentX answers where the content column begins beside a panel at bounds:
// the panel's trailing edge, which is the one side it is not set in from, or
// the window's own leading edge where there is no panel.
func ContentX(bounds image.Rectangle) int {
	if bounds.Empty() {
		return 0
	}
	return bounds.Max.X
}

// Under paints everything that stands under a window's columns: the window's
// own plane, the content column's surface, the two corners the panel rounds
// away from on its flush side, and the panel itself with the sidebar in it.
//
// It is exported because a window whose columns are arranged its own way
// still passes this half of the frame through unchanged. Such a caller spends
// Under where the reading order wants the panel — first, before its own
// columns — lays its columns out, and casts the panel's shadow with
// [pane.PaintShadow] once they have painted.
func (f PaneFrame) Under(gtx layout.Context, c tokens.PlatformColors, size image.Point, bounds image.Rectangle) {
	if f.Plane.A > 0 {
		paint.FillShape(gtx.Ops, f.Plane, clip.Rect{Max: size}.Op())
	}
	x := ContentX(bounds)
	if f.ContentFill.A > 0 && x < size.X {
		paint.FillShape(gtx.Ops, f.ContentFill,
			clip.Rect(image.Rect(x, 0, size.X, size.Y)).Op())
	}
	if bounds.Empty() {
		return
	}
	if f.BandFill.A > 0 {
		// A band that paints its own fill across the content column stands
		// behind the panel's top corner; what is left of the flush side
		// stands on the column below it.
		band := min(max(gtx.Dp(unit.Dp(pane.BandDp)), bounds.Min.Y), bounds.Max.Y)
		top, rest := bounds, bounds
		top.Max.Y, rest.Min.Y = band, band
		pane.FillTrailingCorners(gtx, f.BandFill, top)
		pane.FillTrailingCorners(gtx, f.ContentFill, rest)
	} else {
		pane.FillTrailingCorners(gtx, f.ContentFill, bounds)
	}
	pane.Layout(gtx, c, bounds, f.Sidebar)
}

// Layout composes the whole frame in the order it reads, which is the focus
// ring's too: the panel's column, then the band above the content, then the
// content itself — and last the shadow the panel casts.
//
// The shadow is cast after the columns because the ramp falls on what stands
// AROUND the panel: a column that paints its own surface after the panel has
// laid out would cover it. patterns/pane cuts the panel's own box out of the
// drawing, so casting it here lands what casting it under the panel landed.
func (f PaneFrame) Layout(gtx layout.Context, c tokens.PlatformColors) layout.Dimensions {
	size := gtx.Constraints.Max
	bounds := f.Bounds(gtx, size)
	f.Under(gtx, c, size, bounds)

	x := ContentX(bounds)
	w := size.X - x
	if w <= 0 {
		pane.PaintShadow(gtx, c, bounds)
		return layout.Dimensions{Size: size}
	}
	band := min(gtx.Dp(unit.Dp(pane.BandDp)), size.Y)
	if f.Band != nil && band > 0 {
		st := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		bgtx := gtx
		bgtx.Constraints = layout.Exact(image.Pt(w, band))
		f.Band(bgtx)
		st.Pop()
	}
	if f.Main != nil && size.Y-band > 0 {
		st := op.Offset(image.Pt(x, band)).Push(gtx.Ops)
		mgtx := gtx
		mgtx.Constraints = layout.Exact(image.Pt(w, size.Y-band))
		f.Main(mgtx)
		st.Pop()
	}
	pane.PaintShadow(gtx, c, bounds)
	return layout.Dimensions{Size: size}
}
