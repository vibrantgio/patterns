// Package shell provides the Patterns Shell pattern: a top-level
// application layout. Four variants are offered via Props.Layout —
// SidebarHeaderMain composes a leading sidebar, a top navbar, and a
// main content slot; SplitPane composes two slots abutting a draggable
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
// column, or any other pre-built widget stream. The static Render path
// accepts a pre-built layout.Widget for the sidebar slot; Props.Sidebar
// is not consulted by Render.
package shell

import (
	"image"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/patterns/navbar"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Layout selects which composition Shell renders.
type Layout int

const (
	// SidebarHeaderMain renders a sidebar on the leading edge, a navbar
	// across the top of the remaining area, and a main content slot
	// below the navbar.
	SidebarHeaderMain Layout = iota
	// SplitPane renders Left and Right slots abutting a draggable
	// vertical hairline seam whose position is governed by SplitRatio.
	// The seam runs the window's whole height, so it is painted one
	// hairline wide and dragged by a band several times that — see
	// splitSeamDp and splitGrabDp.
	SplitPane
	// ThreeColumn renders a navbar across the full width of the top
	// edge (unlike SidebarHeaderMain, where the sidebar claims the full
	// height and the navbar starts after it), then a leading sidebar, a
	// flexed main column, and a trailing aside column separated from
	// main by a draggable vertical divider, with an optional full-width
	// footer strip along the bottom. A nil Aside omits the trailing
	// column and its divider, degenerating into a header-first sidebar
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
	// Sidebar is the pre-built sidebar widget stream. Any
	// rx.Observable[layout.Widget] is accepted — pass sidebar.Sidebar(th,
	// sidebarProps) for the default patterns/sidebar, or any other widget
	// stream. A nil Sidebar renders an empty leading column.
	Sidebar rx.Observable[layout.Widget]
	Navbar  navbar.Props
	Main    layout.Widget

	// SplitPane slots. Left is the leading pane and Right the trailing
	// pane; when SplitAxis is layout.Vertical, Left is the top pane and
	// Right the bottom pane.
	Left, Right layout.Widget

	// SplitAxis selects the axis along which Left and Right are
	// arranged. The zero value (layout.Horizontal) places them side by
	// side separated by a vertical divider; layout.Vertical stacks Left
	// above Right separated by a horizontal divider.
	SplitAxis layout.Axis

	// SplitRatio drives the position of the divider as a fraction in
	// [0, 1] along SplitAxis. A nil SplitRatio is treated as a
	// constant 0.5.
	SplitRatio rx.Observable[float32]

	// OnSplitChange is invoked when the user drags the divider. The
	// value is the new ratio in [0, 1]. May be nil.
	OnSplitChange func(gtx layout.Context, ratio float32)

	// ThreeColumn slots. Sidebar, Navbar and Main are shared with
	// SidebarHeaderMain (see above).
	//
	// Aside is the trailing column widget stream — a comments panel, an
	// inspector, or any other contextual surface. A nil Aside omits the
	// column and its divider entirely.
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
	// is not dragging the divider.
	AsideWidth rx.Observable[unit.Dp]

	// OnAsideResize is invoked when the user drags the aside divider.
	// The value is the new clamped width in dp. May be nil.
	OnAsideResize func(gtx layout.Context, width unit.Dp)

	// StackedPage slots. Navbar is shared with SidebarHeaderMain and
	// ThreeColumn; Footer is shared with ThreeColumn, but here it
	// scrolls with the content at its natural height instead of
	// pinning to the viewport at a fixed height.
	//
	// Sections are stacked top to bottom in a scroll region owned by
	// the shell. Each entry is a widget stream, matching the Sidebar
	// and Aside slots, so sections re-render on theme change without a
	// layer-boundary adapter; the shell combines them and re-emits
	// whenever any section emits. Nil entries render empty. Each
	// section spans the full page width (less the ContentMaxWidth
	// clamp, when set) and receives an unbounded height, so it must
	// return its natural height. The static Render path takes
	// pre-built section widgets via RenderStackedPage instead
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
// navbar slot a density-derived one (see NavbarHeight), so the main area
// is deterministic. The aside column tracks an absolute dp width clamped
// to [minAsideDp, maxAsideDp]. The footer is a status strip — a surface,
// not a control — so its height deliberately does not follow density.
const (
	footerHDp = 48

	// asideDividerDp is the ThreeColumn aside divider, which paints at
	// its full width and grabs the same rectangle. It is bounded above
	// by the full-width navbar and below by the footer, so it separates
	// two columns of furniture without ever reaching the window's edge.
	asideDividerDp = 6

	// splitSeamDp is what the SplitPane seam paints, and the room it
	// takes between the panes: a hairline.
	//
	// The seam is the one edge in this package that runs the whole cross
	// axis, top edge to bottom edge. Whatever band an application paints
	// across the top of its window, the seam crosses it — so the seam's
	// width is the width of the scar it leaves there, and a thick one
	// severs the band into two pieces with the window's title marooned on
	// the smaller of them. Platform split dividers are one point for the
	// same reason: at that width an edge reads as an edge, and anything
	// wider starts reading as a third column that nothing occupies.
	splitSeamDp = 1

	// splitGrabDp is the pointer band centred on that hairline.
	//
	// Paint and grab are deliberately different sizes. A hairline is what
	// the eye wants and a poor target for a pointer, so the band reaches
	// into both panes rather than reserving a gutter of its own, and it is
	// registered after them — the topmost area takes the hit, which is
	// what should happen to a press this close to the seam.
	splitGrabDp = 6

	minRatio       = 0.05
	maxRatio       = 0.95
	minAsideDp     = 160
	maxAsideDp     = 640
	defaultAsideDp = 320
)

// NavbarHeight returns the pinned height of the navbar band the shell draws
// for a density: ControlHeight + 2·PaddingY — a bar wrapping ControlHeight
// controls with the density's vertical control padding as breathing room (52
// dp Comfortable, 40 dp Compact). patterns/navbar insets its content by
// the same PaddingY, so a components/button action fills the slot exactly.
//
// This is the number an app needs when it caps a shell window's top edge at
// the depth of the navbar band.
func NavbarHeight(d tokens.Density) unit.Dp {
	return unit.Dp(d.ControlHeight + 2*d.PaddingY)
}

// Shell returns an rx.Observable[layout.Widget] that emits a new
// widget whenever a consumed theme token, the SplitRatio observable,
// or a composed sub-widget changes. Sidebar and navbar event handling
// is delegated to the respective packages; Shell only owns the
// SplitPane divider's drag handler.
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
// column — use RenderThreeColumn to supply a pre-built aside widget; a
// StackedPage Props renders only the navbar and footer — use
// RenderStackedPage to supply pre-built section widgets.
//
// label is the LabelLarge role's whole text style, which the shell
// spends on its navbar, and d is the density both the navbar and the
// navbar slot's pinned height derive from. Pass
// tokens.DefaultTypography.LabelLarge and tokens.Comfortable for the
// default desktop look.
func Render(
	shaper *text.Shaper,
	props Props,
	sidebarW layout.Widget,
	colors tokens.ColorTokens,
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
	densityObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Density] {
		return t.Density
	})
	combined := rx.CombineLatest3(sb, nb, densityObs)
	return rx.Map(combined, func(next rx.Tuple3[layout.Widget, layout.Widget, tokens.Density]) layout.Widget {
		sbW, nbW, d := next.First, next.Second, next.Third
		main := props.Main
		return composeSidebarHeaderMain(sbW, nbW, main, NavbarHeight(d))
	})
}

func staticSidebarHeaderMain(
	sidebarW layout.Widget,
	shaper *text.Shaper,
	props Props,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	label tokens.TextStyle,
	d tokens.Density,
) layout.Widget {
	if sidebarW == nil {
		sidebarW = emptyWidget
	}
	nbW := navbar.Render(shaper, props.Navbar, colors, sp, label, d)
	return composeSidebarHeaderMain(sidebarW, nbW, props.Main, NavbarHeight(d))
}

// composeSidebarHeaderMain stacks the three slots so that Tab focus
// traversal flows sidebar → navbar → main. Flex preserves child order
// in the op stream, which is the order Gio's focus group walks.
func composeSidebarHeaderMain(sb, nb, main layout.Widget, navbarH unit.Dp) layout.Widget {
	if main == nil {
		main = emptyWidget
	}
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		navH := gtx.Dp(navbarH)
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(sb),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.Y = size.Y
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.Y = navH
						gtx.Constraints.Max.Y = navH
						return nb(gtx)
					}),
					layout.Flexed(1, main),
				)
			}),
		)
	}
}

// ---- SplitPane -----------------------------------------------------------

// dragState is captured once per subscription and survives all
// emissions for the lifetime of the Shell instance.
type dragState struct {
	tag      dragTag
	press    float32 // pointer main-axis position at press, in shell-local coords
	startR   float32 // ratio at press
	active   bool
	current  float32 // last seen ratio (from observable or drag)
	lastEmit float32 // last ratio passed to OnSplitChange
	emitted  bool
}

// dragTag is a non-zero-size type so its address is a unique event
// tag for the divider's pointer hit area.
type dragTag struct{ _ byte }

func splitPaneObservable(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	ratioObs := props.SplitRatio
	if ratioObs == nil {
		ratioObs = rx.Of(float32(0.5))
	}
	colorObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	inputs := rx.CombineLatest2(colorObs, ratioObs)
	return rx.Defer(func() rx.Observable[layout.Widget] {
		ds := &dragState{current: 0.5}
		return rx.Map(inputs, func(next rx.Tuple2[tokens.ColorTokens, float32]) layout.Widget {
			colors := next.First
			ext := clampRatio(next.Second)
			left := props.Left
			right := props.Right
			axis := props.SplitAxis
			onChange := props.OnSplitChange
			// applied defers the external-ratio hand-off to the widget:
			// dragState must only ever be touched on the frame goroutine.
			// This projector runs on the rx scheduler, so writing ds here
			// races with processDrag/drawSplitPane during layout.
			applied := false
			return func(gtx layout.Context) layout.Dimensions {
				// External ratio updates win when the user isn't actively
				// dragging — otherwise the displayed position would jump
				// back to whatever the caller most recently fed in
				// mid-drag. An emission arriving mid-drag is applied on
				// the first frame after release.
				if !applied && !ds.active {
					ds.current = ext
					applied = true
				}
				processDrag(gtx, ds, axis, onChange)
				return drawSplitPane(gtx, ds.current, left, right, colors, ds, axis)
			}
		})
	})
}

func staticSplitPane(left, right layout.Widget, ratio float32, colors tokens.ColorTokens, axis layout.Axis) layout.Widget {
	r := clampRatio(ratio)
	return func(gtx layout.Context) layout.Dimensions {
		return drawSplitPane(gtx, r, left, right, colors, nil, axis)
	}
}

func processDrag(gtx layout.Context, ds *dragState, axis layout.Axis, onChange func(gtx layout.Context, ratio float32)) {
	total := float32(axis.Convert(gtx.Constraints.Max).X)
	if total <= 0 {
		return
	}
	for {
		e, ok := gtx.Event(pointer.Filter{
			Target: &ds.tag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		pe, ok := e.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Press:
			ds.press = axis.FConvert(pe.Position).X
			ds.startR = ds.current
			ds.active = true
		case pointer.Drag:
			if !ds.active {
				continue
			}
			delta := axis.FConvert(pe.Position).X - ds.press
			r := clampRatio(ds.startR + delta/total)
			ds.current = r
			if onChange != nil && (!ds.emitted || ds.lastEmit != r) {
				ds.lastEmit = r
				ds.emitted = true
				onChange(gtx, r)
			}
		case pointer.Release, pointer.Cancel:
			ds.active = false
		}
	}
}

// drawSplitPane lays the panes along axis: for layout.Horizontal the
// panes sit side by side separated by a vertical hairline seam; for
// layout.Vertical they stack with a horizontal one. Geometry is
// computed in main-axis terms and mapped back through axis.Convert.
//
// The op-stream order is leading pane, trailing pane, seam, grab band.
// The panes come first because Tab traversal follows the op stream and a
// reader expects leading before trailing. The seam is painted after both
// so a pane that overruns its constraints cannot erase it. The grab band
// comes last because it is the only one of the four whose rectangle
// overlaps its neighbours: Gio hands a hit to the topmost area covering
// it and stops there, so registering the band after the panes is what
// keeps a press two pixels from the seam a drag rather than a click on
// whatever the pane happens to have put at its edge.
func drawSplitPane(
	gtx layout.Context,
	ratio float32,
	left, right layout.Widget,
	colors tokens.ColorTokens,
	ds *dragState,
	axis layout.Axis,
) layout.Dimensions {
	size := gtx.Constraints.Max
	total := axis.Convert(size).X
	cross := axis.Convert(size).Y
	seamPx := gtx.Dp(unit.Dp(splitSeamDp))
	if seamPx < 1 {
		seamPx = 1
	}
	grabPx := gtx.Dp(unit.Dp(splitGrabDp))
	if grabPx < seamPx {
		grabPx = seamPx
	}
	inner := total - seamPx
	if inner < 0 {
		inner = 0
	}
	leftPx := int(float32(inner)*ratio + 0.5)
	if leftPx < 0 {
		leftPx = 0
	}
	if leftPx > inner {
		leftPx = inner
	}
	rightPx := inner - leftPx

	// Backstop so the seam is visible even if Left/Right are nil. It is the
	// window's floor: whatever a split pane does not cover is the desk the
	// panes lie on, and the desk must be the storey beneath the paper in
	// both schemes — the colors.Surface ramp alias is a rung that is the
	// floor only in the light scheme and the raised storey in the dark one.
	paint.FillShape(gtx.Ops, colors.SurfaceAt(tokens.LevelFloor), clip.Rect{Max: size}.Op())

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

	// Seam: the hairline the panes abut, drawn the full cross axis.
	seamRect := image.Rectangle{
		Min: axis.Convert(image.Pt(leftPx, 0)),
		Max: axis.Convert(image.Pt(leftPx+seamPx, cross)),
	}
	paint.FillShape(gtx.Ops, dividerColor(colors), clip.Rect(seamRect).Op())

	// Grab band: wider than the seam, centred on it, over both panes.
	if ds != nil {
		grabMin := leftPx - (grabPx-seamPx)/2
		grabMax := grabMin + grabPx
		if grabMin < 0 {
			grabMin = 0
		}
		if grabMax > total {
			grabMax = total
		}
		grabRect := image.Rectangle{
			Min: axis.Convert(image.Pt(grabMin, 0)),
			Max: axis.Convert(image.Pt(grabMax, cross)),
		}
		area := clip.Rect(grabRect).Push(gtx.Ops)
		event.Op(gtx.Ops, &ds.tag)
		cursor := pointer.CursorColResize
		if axis == layout.Vertical {
			cursor = pointer.CursorRowResize
		}
		cursor.Add(gtx.Ops)
		area.Pop()
	}

	return layout.Dimensions{Size: size}
}

// dividerColor is the semantic Divider token: one step past the Surface
// ground, so it still registers a pixel delta against Surface on both
// light and dark schemes.
func dividerColor(c tokens.ColorTokens) color.NRGBA {
	return c.Divider
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
