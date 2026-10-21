// Package shell provides the Patterns Shell pattern: a top-level
// application layout.
//
// [TwoColumn] is the application shell the platform's own sidebar windows
// stand in, and the one a window with a pane composes itself from: the pane
// down the leading edge with its own column and its bare marks, the content
// column beside it carrying one [Toolbar] across its top, an optional
// inspector down the trailing edge, a footer under each column, a splitter
// on each boundary, and the one [ToggleSidebar] button that stands bare on
// the pane while it is there and bordered in the toolbar once it is away.
// It reports a [PaneLayout] — the arrangement in window coordinates — and
// keeps the widths and the shown state through an [ArrangementMemory].
//
// [Shell] and [Render] build the two compositions [Props].Layout selects
// between: SplitPane composes two slots abutting a draggable hairline seam
// on either axis, and StackedPage composes a pinned full-width navbar over
// a shell-owned vertical scroll of page sections — the marketing-page
// shell.
//
// Shell is a callable Go function consuming a components theme
// observable, returning a stream of layout.Widget. Source is
// intentionally short — copy it into your own app and modify as needed.
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
	// SplitPane renders Left and Right slots abutting a draggable
	// vertical hairline seam whose position is governed by SplitRatio.
	// The seam is a patterns/splitter: it runs the window's whole height,
	// so it is painted one hairline wide and taken by a band several
	// times that.
	SplitPane Layout = iota
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
// StackedPage).
type Props struct {
	Layout Layout

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

	// StackedPage slots.
	//
	// Navbar is the band pinned across the page's top edge.
	Navbar navbar.Props

	// Footer is an optional strip appended after the last section. It
	// scrolls with the content at its natural height instead of pinning
	// to the viewport; a nil Footer omits the strip.
	Footer layout.Widget

	// Sections are stacked top to bottom in a scroll region owned by
	// the shell. Each entry is a layout.Widget stream, so sections
	// re-render on theme change without a layer-boundary adapter; the
	// shell combines them and re-emits whenever any section emits. Nil
	// entries render empty. Each section spans the full page width
	// (less the ContentMaxWidth clamp, when set) and receives an
	// unbounded height, so it must return its natural height. The
	// static Render path takes pre-built section layout.Widget values
	// via RenderStackedPage instead (Props.Sections is not consulted
	// there).
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

// minRatio and maxRatio bound the SplitPane's boundary: neither pane is
// ever dragged away entirely, so each keeps a twentieth of the length
// whatever the other does. The seam between them — its width, its colour,
// the band it is taken by — is patterns/splitter's, and it runs the whole
// cross axis, top edge to bottom edge.
const (
	minRatio = 0.05
	maxRatio = 0.95
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
// or a composed sub-stream changes. Navbar event handling is delegated
// to patterns/navbar; Shell only owns the SplitPane splitter's drag
// handler.
func Shell(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	switch props.Layout {
	case StackedPage:
		return stackedPageObservable(th, props)
	default:
		return splitPaneObservable(th, props)
	}
}

// Render produces a layout.Widget for a shell with pre-resolved tokens
// and no event processing. Intended for golden-image testing and
// static demonstrations; production code should use Shell. splitRatio
// is honoured by SplitPane; a StackedPage Props renders only the navbar
// and footer — use RenderStackedPage to supply pre-built section
// layout.Widget values.
//
// title is the LabelLarge role's whole text style, which the shell
// spends on its navbar, and d is the density the navbar's own insets
// derive from — the band it stands in is the platform's measured depth and
// takes no density. Pass tokens.DefaultTypography.LabelLarge and
// tokens.Comfortable for the default desktop look.
func Render(
	shaper *text.Shaper,
	props Props,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	title tokens.TextStyle,
	d tokens.Density,
	splitRatio float32,
) layout.Widget {
	switch props.Layout {
	case StackedPage:
		return RenderStackedPage(shaper, props, nil, colors, sp, title, d)
	default:
		return staticSplitPane(props.Left, props.Right, splitRatio, colors, props.SplitAxis)
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
	// surface, which nothing is drawn at and which is darker than the chrome
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
