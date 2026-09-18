// Package pane provides the Patterns Pane: the chrome column set in from a
// window's leading, top and bottom edges rather than being one of them —
// rounded on all four corners, with the platform's rim just inside that edge
// and the shadow it casts on what lies around it, the window's own plane
// showing on three sides and the content standing flush against the fourth.
// Dismissed, it takes no width at all and what stood beside it reflows from
// the window's own leading edge.
//
// WHAT KIND OF CHROME THIS IS. A window's regions divide into the document
// and the chrome that frames it, and chrome divides again. FLUSH chrome runs
// to the window's own edge and cannot be sent away, so it takes no outline
// and a plain seam parts it from what it abuts. A PANE is the other kind — a
// control sends it out of the window, so it is an OBJECT, and the inset, the
// corner radius, the rim and the shadow are what say so. This package is the
// second kind and only the second kind. An inset object needs no seam: the
// rim and the plane showing around it do that work.
//
// MEASURED, voicememos-multi-folder-2026-09-18.png at 1x, Voice Memos on
// macOS 26 over a black desktop. The window stands at x 56-1031, y 38-794
// and the sidebar panel at x 64-283, y 46-786: eight pixels of the window's
// own plane on the leading, top and bottom sides, and the content column
// beginning at x=284 with no gap on the fourth. The panel's interior is
// flat #f9f9f9, the platform's sidebar material through glass that has
// black behind it — finder-window-untinted-light.png, whose desktop is
// light, reads the same sidebar material at #f7f7f7 to the byte, so what
// the black moves is two of 255 and the fill is that material either way.
//
// [RadiusDp] is that panel's corner. Fitted to the rim's own centreline
// across the three corners the window's own rounding does not overlap —
// top-trailing, bottom-trailing and bottom-leading — a circle lands them at
// an rms of 0.12 to 0.15 of a pixel at r=17.2, 17.5 and 17.4, which is 17.35
// to the rim's centre and 17.85 to the panel's outer edge. The window's own
// corner in the same capture fits r=25.94 at an rms of 0.20, so the panel's
// rounding is the window's own less the inset it stands at, which is what
// [RadiusDp] is written as.
//
// THE PANEL IS READ THROUGH ITS RIM AND ITS SHADOW. A pane wears the
// platform's chrome material in both schemes, which in the light appearance
// is within two of 255 of the plane it stands on and in the dark appearance
// within two of the content beside it — so neither boundary is a step of
// fill, and both are the rim. [Surface] is the fill, [RimColor] the rim and
// [ShadowColor] the shadow's peak, all three the platform's own names.
//
// The rim is drawn INSIDE the panel's own rounded rectangle, never on the
// plane outside it: half a line lying on the plane would blur the one
// boundary a reader uses to tell where the panel stops. It is painted as two
// concentric fills rather than as a stroke, because a stroke is centred on
// the path it follows and antialiases both of its sides — a one-pixel one
// arrives as two rows of half-strength colour and the colour the palette
// asked for is never actually painted.
//
// THE TOP STRIP IS DERIVED FROM THE WINDOW BUTTONS. Under a full-size
// content treatment the window's control buttons are measured from the
// window's own glass and from nothing drawn beneath them, and they stand
// INSIDE this panel: MEASURED, the same capture, the three 14 px circles run
// x 75-134 and y 57-70, nineteen pixels in from the window's glass on both
// axes and so eleven in from the panel's own corner, their centre line at
// y=64. [StripDp] is that arithmetic and not a taste, and it puts the
// buttons' centre line on the strip's middle line, where a control standing
// in the strip centres. [BandDp] is the band the window's content column
// carries beside it, which is the deeper number: the panel's top edge sits
// one margin inside it, as the capture shows.
//
// THE PANEL'S OWN MARKS STAND BARE AT ITS TOP TRAILING CORNER. MEASURED,
// the same capture: the new-folder mark is drawn over x 209-230 and the
// sidebar toggle over x 252-271, both y 57-71 — on the buttons' own centre
// line, their centres 42 apart, the trailing one ending twelve pixels clear
// of the panel's rim — and neither carries a capsule, a fill or a rim. The
// platform's bordered toolbar control is the BAND's drawing, not the
// panel's. [Strip] stands the caller's controls at that corner.
//
// THE RECALL CONVENTION. A control that travels with the pane cannot be the
// one that recalls it. The pane's own dismiss control rides the strip; the
// control that brings the pane back must stand somewhere that survives the
// pane — the window's own chrome row — and the two are the two halves of one
// switch rather than duplicates of one control. This package draws neither:
// which figure, which label and where the recalling half stands are the
// window's business. What the package fixes is the geometry both halves
// stand on.
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

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/effects/depth"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// The pane's geometry. Every number here is stated once and derived from
// there; nothing in this package reads a density or a spacing step, because
// a pane's inset is a property of the window it is set into and not of how
// tightly its rows are set.
const (
	// MarginDp is the inset the panel stands off the window's leading, top
	// and bottom edges — the slivers of the window's own plane the reader
	// sees around it — and the air its strip keeps at its trailing end. The
	// panel is flush with what it stands beside on the fourth side, so the
	// margin is spent on three. Those slivers claim no window drag of their
	// own: a hand aims for the strip, not for an eight-dp gap, and a move
	// action there would promise a handle too thin to hit.
	MarginDp = 8

	// RadiusDp rounds the panel's four corners, concentric with the window's
	// own: the platform rounds the window at 26 and the panel stands one
	// margin inside it, which the package comment records the fit for.
	RadiusDp = 26 - MarginDp

	// RimDp is the width of the rim drawn just inside the panel's edge. A
	// rim runs a whole edge, so its width is the width of the scar it leaves
	// across everything it crosses, and the platform draws it at one pixel
	// on every side of every stored panel.
	RimDp = 1

	// ShadowReachDp is how far the panel's shadow carries past it and
	// ShadowSinkDp how far below the panel its rectangle sits, which is what
	// makes the shadow heavier under the panel than over it. The two are
	// fitted together off one capture with the peak
	// [tokens.PlatformColors.PaneShadow] carries, and are spent together.
	ShadowReachDp unit.Dp = 24
	ShadowSinkDp  unit.Dp = 9

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

	// StripDp is the panel's own top strip: deep enough to hold the buttons
	// where the window puts them with the same air below them as above. The
	// buttons' inset is measured from the glass and the strip from the
	// panel's own edge, so the strip owes the margin back at both ends —
	// which lands the buttons' centre line on the strip's own middle.
	StripDp = 2*(ButtonInsetDp-MarginDp) + desktop.WindowButtonDiameter

	// BandDp is the toolbar band the window's other columns carry, which the
	// panel passes through rather than cuts: nineteen pixels of inset either
	// side of a fourteen pixel circle makes 52, and 52 is the band every
	// stored toolbar capture measures — 8 px above a 36 px control and 8
	// below it. The panel's own top edge stands one margin inside it.
	BandDp = 2*ButtonInsetDp + desktop.WindowButtonDiameter

	// ButtonGapDp is the air a band owes the window's control buttons: the
	// clear band between the third circle's trailing edge and the leading
	// edge of the first control standing beside it.
	//
	// MEASURED at 1x, voicememos-window.png — the one stored window that
	// keeps a toolbar control beside its buttons over the sidebar region:
	// the three circles run x 19-78 and the sidebar toggle's capsule begins
	// at x=96, so seventeen columns of band stand between them. What
	// [desktop.LeadingInset] reports is the bare glass the third circle ends
	// at and carries no breathing room of its own, which is why the air is a
	// number of the strip's own.
	ButtonGapDp = 17

	// MarkGapDp is the clear band between two bare marks standing in the
	// panel's top trailing corner: MEASURED, the two marks' drawn centres
	// stand 42 apart in voicememos-multi-folder-2026-09-18.png, which at the
	// 24 dp mark box every chrome mark in this library is drawn in leaves
	// eighteen between the boxes. A caller passes it between two controls
	// handed to [Strip].
	MarkGapDp = 18
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

// RimColor is the colour of the rim the panel wears just inside its own
// edge: the platform's measured value, opaque in both appearances, so
// nothing is flattened here.
func RimColor(c tokens.PlatformColors) color.NRGBA {
	return c.PaneRim
}

// ShadowColor is the peak coverage of the shadow the panel casts on what
// lies around it, at the panel's own edge. [Layout] spreads it over
// [ShadowReachDp] from a rectangle sunk [ShadowSinkDp] below the panel.
func ShadowColor(c tokens.PlatformColors) color.NRGBA {
	return c.PaneShadow
}

// Bounds answers the panel's rectangle in the coordinates of a window of the
// given size: one [MarginDp] inside its leading, top and bottom edges, as
// wide as width asks for. It is separate from the drawing so that a frame
// can measure its arrangement — where its content begins, whether the panel
// fits at all — without laying anything out.
//
// The rectangle is EMPTY in the states where there is no panel to draw:
// hidden, which is the whole of the hidden contract (the pane takes no
// width at all and the caller lays its content out from the window's own
// leading edge, rather than collapsing to a rail that still has to be
// reasoned about), a window with no area, and a window too small to set
// anything into. A caller reads the emptiness rather than a flag.
//
// The panel and its margin may never take more than half the window: a
// narrow window owes its document a readable column before it owes the
// chrome its width.
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

// Layout draws the panel at bounds — its rim, then its fill — and lays
// contents inside it at the panel's full size.
//
// It does NOT paint the shadow. The ramp falls on what stands AROUND the
// panel, and in a window that is a column whose own surface is painted after
// the pane has laid out, which would cover it; so the shadow is
// [PaintShadow]'s and the caller spends it where its own painting is done.
// A caller that paints nothing over the ramp may call it either side of
// this.
//
// The contents are clipped to the FILL rather than to the boundary, so a
// scrolled row that runs the panel's full width can neither cross an edge,
// poke through a corner, nor paint over the rim that says the panel is an
// object. An empty rectangle draws nothing, which is the dismissed state; a
// nil contents draws the panel and nothing in it.
func Layout(gtx layout.Context, c tokens.PlatformColors, bounds image.Rectangle, contents layout.Widget) {
	if bounds.Empty() {
		return
	}
	r := gtx.Dp(unit.Dp(RadiusDp))
	w := max(gtx.Dp(unit.Dp(RimDp)), 1)

	// Two concentric fills rather than a stroke, for the reason the package
	// doc gives: filling the panel in the rim's colour and filling the inset
	// panel back in over it leaves exactly one pixel of the rim's own colour
	// down every straight run, with the corners' arcs antialiased against
	// each other the way a fence's rim is drawn.
	rr := clip.RRect{Rect: bounds, NE: r, NW: r, SE: r, SW: r}
	paint.FillShape(gtx.Ops, RimColor(c), rr.Op(gtx.Ops))
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

// PaintShadow paints the shadow the panel at bounds casts on what lies
// around it: the peak [ShadowColor] at the edge of a rectangle sunk
// [ShadowSinkDp] below the panel, falling to nothing [ShadowReachDp] out.
//
// WHERE TO CALL IT. The ramp falls outside the panel and on whatever stands
// there, so it must be painted after that. A window's document column fills
// its own surface after the pane has laid out — the pane comes first in the
// op stream because it comes first in the reading order — and that fill
// covers the ramp the panel cast on it. Such a caller paints the panel with
// [Layout] where the reading order wants it and calls this LAST, once its
// columns are down. A caller that paints nothing over the ramp calls it
// wherever it likes.
//
// The panel's own rounded box is cut out of the drawing, which is what makes
// the later call land what an earlier one landed: the panel's fill is
// opaque, so a ramp under it and a ramp with its box removed are the same
// image, and nothing this paints touches the panel's rim or its contents.
func PaintShadow(gtx layout.Context, c tokens.PlatformColors, bounds image.Rectangle) {
	if bounds.Empty() {
		return
	}
	col := ShadowColor(c)
	extent := gtx.Dp(ShadowReachDp)
	if col.A == 0 || extent <= 0 {
		return
	}
	r := gtx.Dp(unit.Dp(RadiusDp))
	sunk := bounds.Add(image.Pt(0, gtx.Dp(ShadowSinkDp)))
	// The whole drawing's extent: the sunk rectangle and the panel's own box,
	// grown by the reach. One pixel of slack keeps the ramp's last column
	// inside the outer contour.
	all := bounds.Union(sunk).Inset(-extent - 1)
	defer clip.Outline{Path: ringPath(gtx.Ops, all, bounds, r)}.Op().Push(gtx.Ops).Pop()
	depth.ShadowAt(gtx, sunk, r, ShadowReachDp, col)
}

// bezierCircle is the cubic-Bézier control-point ratio that best
// approximates a quarter circle: 4/3·(√2−1).
const bezierCircle = 0.55228475

// ringPath is outer with the rounded rectangle box cut out of it: the outer
// contour wound one way and the inner the other, so the inner is a hole under
// the non-zero winding rule Gio fills outlines by.
func ringPath(ops *op.Ops, outer, box image.Rectangle, radius int) clip.PathSpec {
	var p clip.Path
	p.Begin(ops)

	// The outer contour, clockwise on a screen whose y runs down.
	p.MoveTo(f32.Pt(float32(outer.Min.X), float32(outer.Min.Y)))
	p.LineTo(f32.Pt(float32(outer.Max.X), float32(outer.Min.Y)))
	p.LineTo(f32.Pt(float32(outer.Max.X), float32(outer.Max.Y)))
	p.LineTo(f32.Pt(float32(outer.Min.X), float32(outer.Max.Y)))
	p.Close()

	// The panel's box, counter-clockwise: down the leading side, along the
	// foot, up the trailing side and back across the top.
	x0, y0 := float32(box.Min.X), float32(box.Min.Y)
	x1, y1 := float32(box.Max.X), float32(box.Max.Y)
	r := float32(radius)
	k := bezierCircle * r
	p.MoveTo(f32.Pt(x0, y0+r))
	p.LineTo(f32.Pt(x0, y1-r))
	p.CubeTo(f32.Pt(x0, y1-r+k), f32.Pt(x0+r-k, y1), f32.Pt(x0+r, y1))
	p.LineTo(f32.Pt(x1-r, y1))
	p.CubeTo(f32.Pt(x1-r+k, y1), f32.Pt(x1, y1-r+k), f32.Pt(x1, y1-r))
	p.LineTo(f32.Pt(x1, y0+r))
	p.CubeTo(f32.Pt(x1, y0+r-k), f32.Pt(x1-r+k, y0), f32.Pt(x1-r, y0))
	p.LineTo(f32.Pt(x0+r, y0))
	p.CubeTo(f32.Pt(x0+r-k, y0), f32.Pt(x0, y0+r-k), f32.Pt(x0, y0+r))
	p.Close()

	return p.End()
}

// FillTrailingCorners fills the strip the panel's two trailing corners round
// away from, in the fill of the region standing flush against that edge.
//
// The panel is set in from the window's leading, top and bottom edges and
// flush with what it stands beside on the fourth, so the plane shows on
// three sides and not on the fourth: behind a corner arc on the flush side
// stands the region the panel abuts, not the window's plane. Without this
// the two arcs read as nicks of plane bitten out of the boundary.
//
// Call it before [Layout], which paints over the whole strip but the arcs. A
// caller that stands the panel in the open — with the plane showing on all
// four sides, as the pattern's own stored images do — calls it not at all.
func FillTrailingCorners(gtx layout.Context, fill color.NRGBA, bounds image.Rectangle) {
	r := gtx.Dp(unit.Dp(RadiusDp))
	if bounds.Empty() || r <= 0 {
		return
	}
	strip := image.Rect(bounds.Max.X-r, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
	paint.FillShape(gtx.Ops, fill, clip.Rect(strip).Op())
}

// EdgeSpan answers the rows the panel's trailing edge runs straight down, in
// the coordinates bounds is stated in: from where the top corner's arc lets
// go to where the bottom corner's begins. A boundary control riding that
// edge — a splitter resizing the panel — draws and takes hold over those
// rows and no others, so its line neither crosses a rounded corner nor
// leaves one pixel of itself out on the window's plane.
//
// The answer is empty where the panel is too short to have a straight run.
func EdgeSpan(gtx layout.Context, bounds image.Rectangle) (top, bottom int) {
	r := gtx.Dp(unit.Dp(RadiusDp))
	top, bottom = bounds.Min.Y+r, bounds.Max.Y-r
	if bottom < top {
		return 0, 0
	}
	return top, bottom
}

// SeamTop answers the first row a line between two FLUSH regions is drawn
// on: the toolbar band's lower edge, which is [BandDp] down from the top of
// the rectangle it is asked about. The band is one across the window's
// columns and no line crosses it, so such a seam starts under it. A column
// shorter than the band has no row left to draw on and this answers its
// foot.
//
// The panel itself parts from nothing with a line — its rim and the plane
// around it are the boundary — so this is for the window's other
// boundaries, and it is exported so that a splitter riding one of them draws
// over the same rows the seam does rather than beside it or through the
// band.
func SeamTop(gtx layout.Context, bounds image.Rectangle) int {
	top := bounds.Min.Y + gtx.Dp(unit.Dp(BandDp))
	return min(top, bounds.Max.Y)
}

// Strip lays out the panel's top band: the window control buttons' span
// skipped at the leading end, a stretch that moves the window across the
// middle, and the caller's controls at the trailing corner, one margin in
// from the panel's trailing edge.
//
// lead is where a band's own content may start, in WINDOW coordinates — the
// buttons' trailing edge plus the air the platform leaves after it, or the
// window's own edge inset where it has no such controls. The panel stands
// one margin inside the window's leading edge, so the panel-local skip is
// that measurement less the margin: the buttons are the window's and stand
// where it puts them; it is the panel that slid in under them. The span is
// skipped rather than claimed because a move action declared over the
// buttons would fight them for the press.
//
// The controls stand at the trailing corner because that is where the
// platform keeps a sidebar panel's own marks, which the package doc records
// the measurement for. They are handed over in reading order and each takes
// its own width; a caller wanting air between two of them passes a spacer of
// [MarkGapDp] between them. The band's depth is the constraint the caller
// gives it, which should be [StripDp] — the strip is reserved by the pane's
// own vertical arrangement, and whether it is drawn in place or after the
// rest is a question about focus order that belongs to the caller.
func Strip(gtx layout.Context, lead unit.Dp, controls ...layout.Widget) layout.Dimensions {
	lead -= MarginDp
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
