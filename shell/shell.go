// Package shell provides the Patterns Shell pattern: a top-level
// application layout. Four variants are offered via Props.Layout —
// SidebarHeaderMain composes a leading sidebar set into the window as a
// PANE, a band across the content column's top and a main content slot
// under it; SplitPane composes two slots abutting a draggable
// hairline seam on either axis; ThreeColumn composes a full-width
// top navbar, a leading sidebar, a main column, an optional resizable
// trailing aside, and an optional footer strip; StackedPage composes a
// pinned full-width navbar over a shell-owned vertical scroll of page
// sections — the marketing-page shell.
//
// Shell is a callable Go function consuming a components theme
// observable, returning a stream of layout.Widget. Source is
// intentionally short — copy it into your own app and modify as needed.
//
// The Sidebar slot accepts any rx.Observable[layout.Widget], so callers
// can supply a patterns/sidebar instance, a patterns/accordion-based
// column, or any other pre-built layout.Widget stream. The static Render path
// accepts a pre-built layout.Widget for the sidebar slot; Props.Sidebar
// is not consulted by Render.
package shell

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/patterns/internal/surface"
	"github.com/vibrantgio/patterns/navbar"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/splitter"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Layout selects which composition Shell renders.
type Layout int

const (
	// SidebarHeaderMain renders the sidebar as a PANE down the leading
	// edge — an inset rounded panel with the platform's rim and the shadow
	// it casts, the window's own plane showing around it — a navbar band
	// across the top of the content column beside it, and a main content
	// slot below that band. The composition is [PaneFrame]'s.
	SidebarHeaderMain Layout = iota
	// SplitPane renders Left and Right slots abutting a draggable
	// vertical hairline seam whose position is governed by SplitRatio.
	// The seam is a patterns/splitter: it runs the window's whole height,
	// so it is painted one hairline wide and taken by a band several
	// times that.
	SplitPane
	// ThreeColumn renders a navbar across the full width of the top
	// edge (unlike SidebarHeaderMain, where the sidebar's panel claims
	// the full height and the band starts beside it), then a leading
	// sidebar standing flush against the window's edge, a
	// flexed main column, and a trailing aside column separated from
	// main by a draggable vertical splitter, with an optional full-width
	// footer strip along the bottom. A nil Aside omits the trailing
	// column and its splitter, degenerating into a header-first sidebar
	// layout; a nil Footer omits the bottom strip. Each column scrolls
	// (or not) on its own — the shell hands every slot its full height.
	ThreeColumn
	// StackedPage renders a navbar pinned across the full top edge and
	// the Sections slots stacked in a shell-owned vertical scroll
	// region below it, with Footer appended after the last section so
	// it scrolls with the content instead of pinning to the viewport.
	// This is the marketing-page shell: hero, feature, pricing and
	// testimonial sections slot in as Sections.
	StackedPage
)

// Props configures a Shell. Fields not used by the chosen Layout are
// ignored (e.g., Left/Right/SplitRatio are unused when Layout is
// SidebarHeaderMain).
type Props struct {
	Layout Layout

	// SidebarHeaderMain slots.
	//
	// Sidebar is the pre-built sidebar layout.Widget stream. Any
	// rx.Observable[layout.Widget] is accepted — pass sidebar.Sidebar(th,
	// sidebarProps) for the default patterns/sidebar, or any other layout.Widget
	// stream. A nil Sidebar renders an empty leading column.
	Sidebar rx.Observable[layout.Widget]
	Navbar  navbar.Props
	Main    layout.Widget

	// SidebarWidth is the panel's own width, rim to rim, under
	// SidebarHeaderMain. Zero means [PaneWidthDp]. It is ignored by every
	// other Layout, whose sidebar column stands flush against the window's
	// edge and takes the width the slot itself reports.
	SidebarWidth unit.Dp

	// SidebarHidden sends the panel out of the window under
	// SidebarHeaderMain: it takes no width at all and the content column
	// reflows from the window's own leading edge. The control that brings
	// it back stands in the band, since a control travelling with the
	// panel cannot be the one that recalls it.
	SidebarHidden bool

	// SplitPane slots. Left is the leading pane and Right the trailing
	// pane; when SplitAxis is layout.Vertical, Left is the top pane and
	// Right the bottom pane.
	Left, Right layout.Widget

	// SplitAxis selects the axis along which Left and Right are
	// arranged. The zero value (layout.Horizontal) places them side by
	// side separated by a vertical splitter; layout.Vertical stacks Left
	// above Right separated by a horizontal splitter.
	SplitAxis layout.Axis

	// SplitRatio drives the position of the splitter as a fraction in
	// [0, 1] along SplitAxis. A nil SplitRatio is treated as a
	// constant 0.5.
	SplitRatio rx.Observable[float32]

	// OnSplitChange is invoked when the user drags the splitter. The
	// value is the new ratio in [0, 1]. May be nil.
	OnSplitChange func(gtx layout.Context, ratio float32)

	// ThreeColumn slots. Sidebar, Navbar and Main are shared with
	// SidebarHeaderMain (see above).
	//
	// Aside is the trailing column layout.Widget stream — a comments panel, an
	// inspector, or any other contextual surface. A nil Aside omits the
	// column and its splitter entirely.
	Aside rx.Observable[layout.Widget]

	// Footer is an optional full-width strip below the columns (a
	// status or transport bar). It is laid out at a fixed footerHDp
	// height; a nil Footer omits the strip.
	Footer layout.Widget

	// AsideWidth drives the width of the aside column as an absolute dp
	// value. Unlike SplitRatio, a window resize keeps the aside at its
	// width and lets the main column absorb the change — the right
	// behaviour for annotation and inspector panels. Values are clamped
	// to [minAsideDp, maxAsideDp]. A nil AsideWidth is treated as a
	// constant defaultAsideDp. External updates win only while the user
	// is not dragging the splitter.
	AsideWidth rx.Observable[unit.Dp]

	// OnAsideResize is invoked when the user drags the aside splitter.
	// The value is the new clamped width in dp. May be nil.
	OnAsideResize func(gtx layout.Context, width unit.Dp)

	// StackedPage slots. Navbar is shared with SidebarHeaderMain and
	// ThreeColumn; Footer is shared with ThreeColumn, but here it
	// scrolls with the content at its natural height instead of
	// pinning to the viewport at a fixed height.
	//
	// Sections are stacked top to bottom in a scroll region owned by
	// the shell. Each entry is a layout.Widget stream, matching the Sidebar
	// and Aside slots, so sections re-render on theme change without a
	// layer-boundary adapter; the shell combines them and re-emits
	// whenever any section emits. Nil entries render empty. Each
	// section spans the full page width (less the ContentMaxWidth
	// clamp, when set) and receives an unbounded height, so it must
	// return its natural height. The static Render path takes
	// pre-built section layout.Widget values via RenderStackedPage instead
	// (Props.Sections is not consulted there).
	Sections []rx.Observable[layout.Widget]

	// ContentMaxWidth, when positive, clamps every section (and the
	// scrolling Footer) to at most this width and centers the clamped
	// column on the page; the navbar stays full-bleed. The zero value
	// keeps sections at the full page width, in which case sections
	// own their internal max-width/centering — a full-bleed background
	// with a centered inner column composes naturally. Sections
	// narrower than the page still paint the page Background in the
	// side margins.
	ContentMaxWidth unit.Dp
}

// Layout-affecting constants. The footer slot has a fixed height and the
// navbar slot the platform's measured band (see NavbarHeight), so the main
// area is deterministic. The aside column tracks an absolute dp width clamped
// to [minAsideDp, maxAsideDp]. The footer is a status strip — a surface,
// not a control — so its height deliberately does not follow density.
const (
	footerHDp = 48

	// asideSplitterDp is the ThreeColumn aside splitter, which paints at
	// its full width and grabs the same rectangle. It is bounded above
	// by the full-width navbar and below by the footer, so it separates
	// two columns of chrome without ever reaching the window's edge.
	asideSplitterDp = 6

	// minRatio and maxRatio bound the SplitPane's boundary: neither pane
	// is ever dragged away entirely, so each keeps a twentieth of the
	// length whatever the other does. The seam between them — its width,
	// its colour, the band it is taken by — is patterns/splitter's, and it
	// runs the whole cross axis, top edge to bottom edge.
	minRatio       = 0.05
	maxRatio       = 0.95
	minAsideDp     = 160
	maxAsideDp     = 640
	defaultAsideDp = 320
)

// NavbarHeight returns the pinned depth of the navbar band the shell draws:
// [pane.BandDp], 52 dp, which is the band every stored toolbar capture
// measures — nineteen dp of inset either side of a fourteen dp window
// control button. patterns/navbar fills whatever it is handed, so the bar
// stands in the band rather than settling it.
//
// It takes no density. The band is the PLATFORM's: the three window control
// buttons stand a measured inset in from the window's own glass, the band
// that holds them centred follows from that one number, and no setting of
// how tightly a window sets its rows moves it.
//
// This is the number an app needs when it caps a shell window's top edge at
// the depth of the navbar band.
func NavbarHeight() unit.Dp {
	return unit.Dp(pane.BandDp)
}

// Shell returns an rx.Observable[layout.Widget] that emits a new one
// whenever a consumed theme token, the SplitRatio observable,
// or a composed sub-stream changes. Sidebar and navbar event handling
// is delegated to the respective packages; Shell only owns the
// SplitPane splitter's drag handler.
func Shell(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	switch props.Layout {
	case SplitPane:
		return splitPaneObservable(th, props)
	case ThreeColumn:
		return threeColumnObservable(th, props)
	case StackedPage:
		return stackedPageObservable(th, props)
	default:
		return sidebarHeaderMainObservable(th, props)
	}
}

// Render produces a layout.Widget for a shell with pre-resolved tokens
// and no event processing. Intended for golden-image testing and
// static demonstrations; production code should use Shell. splitRatio
// is honoured by SplitPane; SidebarHeaderMain uses the supplied sidebarW
// directly (Props.Sidebar is not consulted). Pass nil sidebarW to render
// an empty sidebar column. A ThreeColumn Props renders without an aside
// column — use RenderThreeColumn to supply a pre-built aside layout.Widget; a
// StackedPage Props renders only the navbar and footer — use
// RenderStackedPage to supply pre-built section layout.Widget values.
//
// label is the LabelLarge role's whole text style, which the shell
// spends on its navbar, and d is the density the navbar's own insets
// derive from — the band it stands in is the platform's measured depth and
// takes no density. Pass tokens.DefaultTypography.LabelLarge and
// tokens.Comfortable for the default desktop look.
func Render(
	shaper *text.Shaper,
	props Props,
	sidebarW layout.Widget,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	label tokens.TextStyle,
	d tokens.Density,
	splitRatio float32,
) layout.Widget {
	switch props.Layout {
	case SplitPane:
		return staticSplitPane(props.Left, props.Right, splitRatio, colors, props.SplitAxis)
	case ThreeColumn:
		return RenderThreeColumn(shaper, props, sidebarW, nil, colors, sp, label, d, defaultAsideDp)
	case StackedPage:
		return RenderStackedPage(shaper, props, nil, colors, sp, label, d)
	default:
		return staticSidebarHeaderMain(sidebarW, shaper, props, colors, sp, label, d)
	}
}

// ---- SidebarHeaderMain ---------------------------------------------------

func sidebarHeaderMainObservable(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	sb := props.Sidebar
	if sb == nil {
		sb = rx.Of[layout.Widget](emptyWidget)
	}
	nb := navbar.Navbar(th, props.Navbar)
	colorObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	combined := rx.CombineLatest3(sb, nb, colorObs)
	return rx.Map(combined, func(next rx.Tuple3[layout.Widget, layout.Widget, tokens.PlatformColors]) layout.Widget {
		sbW, nbW, colors := next.First, next.Second, next.Third
		return composeSidebarHeaderMain(sbW, nbW, props, colors)
	})
}

func staticSidebarHeaderMain(
	sidebarW layout.Widget,
	shaper *text.Shaper,
	props Props,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	label tokens.TextStyle,
	d tokens.Density,
) layout.Widget {
	if sidebarW == nil {
		sidebarW = emptyWidget
	}
	nbW := navbar.Render(shaper, props.Navbar, colors, sp, label, d)
	return composeSidebarHeaderMain(sidebarW, nbW, props, colors)
}

// composeSidebarHeaderMain hands the three slots to [PaneFrame], which is
// the composition: the window's own plane under everything, the sidebar's
// panel set one margin inside the window's leading, top and bottom edges,
// the navbar band across the content column at the platform's measured
// depth, the main content under it and the panel's shadow cast last.
//
// The frame lays its slots out in reading order — panel, band, main — which
// is the order Gio's focus group walks, so Tab traversal follows it.
//
// The window's plane is the platform's WindowBackground and the content
// column stands on its ControlBackground: a pane is read through its rim and
// the plane showing around it, so both fills are the platform's own names
// and neither boundary is a step of fill the caller chooses.
func composeSidebarHeaderMain(sb, nb layout.Widget, props Props, colors tokens.PlatformColors) layout.Widget {
	main := props.Main
	if main == nil {
		main = emptyWidget
	}
	f := PaneFrame{
		Width:       props.SidebarWidth,
		Hidden:      props.SidebarHidden,
		Plane:       colors.WindowBackground,
		ContentFill: colors.ControlBackground,
		Sidebar:     sb,
		Band:        nb,
		Main:        main,
	}
	return func(gtx layout.Context) layout.Dimensions {
		return f.Layout(gtx, colors)
	}
}

// ---- SplitPane -----------------------------------------------------------

// splitState is captured once per subscription and survives all
// emissions for the lifetime of the Shell instance. The splitter owns
// the hand on the seam; what is left here is the ratio it moves, which
// is the SplitPane's own idiom and not the splitter's.
type splitState struct {
	sp       splitter.State
	current  float32 // last seen ratio (from observable or drag)
	lastEmit float32 // last ratio passed to OnSplitChange
	emitted  bool
}

func splitPaneObservable(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	ratioObs := props.SplitRatio
	if ratioObs == nil {
		ratioObs = rx.Of(float32(0.5))
	}
	colorObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	inputs := rx.CombineLatest2(colorObs, ratioObs)
	return rx.Defer(func() rx.Observable[layout.Widget] {
		ds := &splitState{current: 0.5}
		return rx.Map(inputs, func(next rx.Tuple2[tokens.PlatformColors, float32]) layout.Widget {
			colors := next.First
			ext := clampRatio(next.Second)
			left := props.Left
			right := props.Right
			axis := props.SplitAxis
			onChange := props.OnSplitChange
			// applied defers the external-ratio hand-off to the layout.Widget:
			// splitState must only ever be touched on the frame goroutine.
			// This projector runs on the rx scheduler, so writing ds here
			// races with processDrag/drawSplitPane during layout.
			applied := false
			return func(gtx layout.Context) layout.Dimensions {
				// External ratio updates win when the user isn't actively
				// dragging — otherwise the displayed position would jump
				// back to whatever the caller most recently fed in
				// mid-drag. An emission arriving mid-drag is applied on
				// the first frame after release.
				if !applied && !ds.sp.Dragging() {
					ds.current = ext
					applied = true
				}
				processDrag(gtx, ds, axis, colors, onChange)
				return drawSplitPane(gtx, ds.current, left, right, colors, ds, axis, onChange)
			}
		})
	})
}

func staticSplitPane(left, right layout.Widget, ratio float32, colors tokens.PlatformColors, axis layout.Axis) layout.Widget {
	r := clampRatio(ratio)
	return func(gtx layout.Context) layout.Dimensions {
		return drawSplitPane(gtx, r, left, right, colors, nil, axis, nil)
	}
}

// processDrag hands this frame's pointer events to the splitter before
// anything is laid out from the ratio, so that the panes and the seam are
// drawn at the position the drag reached rather than one frame behind it.
func processDrag(
	gtx layout.Context,
	ds *splitState,
	axis layout.Axis,
	colors tokens.PlatformColors,
	onChange func(gtx layout.Context, ratio float32),
) {
	inner, boundary := splitGeometry(gtx, axis, ds.current)
	ds.sp.Update(gtx, splitProps(gtx, ds, axis, colors, inner, boundary, onChange))
}

// splitGeometry maps a ratio onto this frame's pixels: the length the two
// panes share once the seam has taken its own, and the boundary they meet
// at. The ratio is a fraction of that shared length, so the seam's width
// comes off it before the split rather than out of one pane.
func splitGeometry(gtx layout.Context, axis layout.Axis, ratio float32) (inner, boundary int) {
	inner = max(axis.Convert(gtx.Constraints.Max).X-splitter.SeamWidth(gtx), 0)
	boundary = min(max(int(float32(inner)*ratio+0.5), 0), inner)
	return inner, boundary
}

// splitProps states the splitter for one frame and converts what it
// reports back into the ratio the SplitPane keeps. A nil state is the
// static render path: a seam nothing can take hold of.
func splitProps(
	gtx layout.Context,
	ds *splitState,
	axis layout.Axis,
	colors tokens.PlatformColors,
	inner, boundary int,
	onChange func(gtx layout.Context, ratio float32),
) splitter.Props {
	p := splitter.Props{
		Axis:     axis,
		Boundary: float32(boundary),
		Min:      minRatio * float32(inner),
		Max:      maxRatio * float32(inner),
		Colors:   colors,
	}
	if ds == nil || inner <= 0 {
		return p
	}
	p.OnChange = func(at float32) {
		r := clampRatio(at / float32(inner))
		ds.current = r
		if onChange != nil && (!ds.emitted || ds.lastEmit != r) {
			ds.lastEmit = r
			ds.emitted = true
			onChange(gtx, r)
		}
	}
	return p
}

// drawSplitPane lays the panes along axis: for layout.Horizontal the
// panes sit side by side separated by a vertical hairline seam; for
// layout.Vertical they stack with a horizontal one. Geometry is
// computed in main-axis terms and mapped back through axis.Convert.
//
// The op-stream order is leading pane, trailing pane, splitter. The panes
// come first because Tab traversal follows the op stream and a reader
// expects leading before trailing. The splitter comes last because it
// draws the line the panes abut and puts its hit area over both of them:
// Gio hands a hit to the topmost area covering it and stops there, so
// laying the splitter out after the panes is what keeps a press two
// pixels from the seam a drag rather than a click on whatever the pane
// happens to have put at its edge.
func drawSplitPane(
	gtx layout.Context,
	ratio float32,
	left, right layout.Widget,
	colors tokens.PlatformColors,
	ds *splitState,
	axis layout.Axis,
	onChange func(gtx layout.Context, ratio float32),
) layout.Dimensions {
	size := gtx.Constraints.Max
	cross := axis.Convert(size).Y
	seamPx := splitter.SeamWidth(gtx)
	inner, leftPx := splitGeometry(gtx, axis, ratio)
	rightPx := inner - leftPx

	// Backstop so the seam is visible even if Left/Right are nil. It is the
	// BACKDROP: whatever a split pane does not cover is the bare window
	// plane, which nothing is drawn at and which is darker than the chrome
	// standing on it in both schemes.
	paint.FillShape(gtx.Ops, surface.Backdrop(colors), clip.Rect{Max: size}.Op())

	// Leading pane.
	if left != nil {
		st := op.Offset(image.Point{}).Push(gtx.Ops)
		lgtx := gtx
		lgtx.Constraints = layout.Exact(axis.Convert(image.Pt(leftPx, cross)))
		left(lgtx)
		st.Pop()
	}

	// Trailing pane.
	if right != nil {
		st := op.Offset(axis.Convert(image.Pt(leftPx+seamPx, 0))).Push(gtx.Ops)
		rgtx := gtx
		rgtx.Constraints = layout.Exact(axis.Convert(image.Pt(rightPx, cross)))
		right(rgtx)
		st.Pop()
	}

	// The seam the panes abut, and the hand on it.
	var sp *splitter.State
	if ds != nil {
		sp = &ds.sp
	}
	sp.Layout(gtx, splitProps(gtx, ds, axis, colors, inner, leftPx, onChange))

	return layout.Dimensions{Size: size}
}

// ---- helpers -------------------------------------------------------------

func clampRatio(r float32) float32 {
	if r < minRatio {
		return minRatio
	}
	if r > maxRatio {
		return maxRatio
	}
	return r
}

func emptyWidget(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{Size: gtx.Constraints.Min}
}
