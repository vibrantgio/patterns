// Package popover provides the Patterns Popover pattern: an anchored
// floating surface placed adjacent to a caller-supplied anchor,
// with a small triangular tail pointing at the anchor. Outside-
// click dismissal and popover-vs-popover arbitration are frame state:
// opening a second popover dismisses the first, through a plain Arbiter
// written and read during layout on the frame goroutine. See ADR-008 and
// arbitration.go.
//
// A popover floats, so its surface (and its tail) is the platform's window
// background under the platform's measured floating shadow, with the
// platform's separator around its edge — the hairline the platform draws
// around a popover, which a sheet does not have. components/input's dropdown
// menu, the same overlay class, is filled the same way.
//
// Popover is a callable Go function consuming a components theme
// observable, returning a stream of layout.Widget. The source is
// intentionally short and free of opaque configuration — copy it into
// your own app and modify as needed.
//
// THE SURFACE IS PLACED IN THE AVAILABLE ROOM. The popover stands its anchor
// in the frame its caller hands it, at [Alignment]'s edge of it, and places
// the surface beside the anchor by the recipe every floating surface is
// placed with, [github.com/vibrantgio/components/attachment]: inside the
// available room [Props.AvailableRoom] reports, the surface takes the
// opposite side when the preferred side has no room for it and the other
// holds it whole, takes the side with more room when neither holds it, is
// shifted inward along the other axis when it would cross the room's edge,
// and scrolls inside itself when it is taller than the side it wins. The tail points at the anchor through all of it. Where a layout
// set [attachment.Surfaces] on the context, the surface is recorded there and
// placed by that layout in the room it passes, as a tooltip's is.
//
// With no room reported the frame is the bound across the placement's axis:
// the surface keeps the side asked for and is shifted back inside the frame
// where it would run off. A frame cut to the anchor's own size cannot hold
// the surface, is not a bound, and the popover leaves such a surface where
// the anchor puts it. Aligning the anchor is the popover's job for the same
// reason: it is the one place the drawn anchor, the surface and the room are
// all known, so an anchor handed here reports the shape it drew and nothing
// wider.
//
// THE FLOATING SURFACE PAINTS LAST. The anchor is drawn where the caller
// put it, but the surface, its tail and its input handler go through
// op.Defer, so they run after every other operation in the frame. A popover
// opened from a slot laid out early — a navigation bar's action, a table
// header's control — would otherwise be painted over by every sibling laid
// out after that slot, and a floating surface stands above everything
// raised beneath it. Deferral carries the hit order with it: input
// registered in deferred operations is topmost, so a press on the surface
// is consumed by the surface rather than by what it covers. Deferral keeps
// the transform and drops the clip, so the surface is no longer held by
// whatever clipped the anchor: it is the caller's frame, not an ancestor's
// scissor, that bounds where a popover may stand. This is the idiom a
// floating surface in this organization is drawn with.
//
// Open/close is instantaneous in this package; entrance/exit transitions
// are deferred to a later Effects-integration goal.
package popover

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/attachment"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/effects/depth"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Placement is the side of the anchor on which the popover surface sits.
type Placement = attachment.Side

const (
	Top    = attachment.Top
	Bottom = attachment.Bottom
	Left   = attachment.Left
	Right  = attachment.Right
)

// Alignment is where across the frame the popover stands its anchor.
//
// It is the placement half of the frame contract: the anchor draws the
// shape it has and this says which of the frame's edges that shape is set
// against, so the popover knows the drawn rect exactly and can aim the tail
// at it. An anchor that widens its own report to reach an edge instead
// leaves the popover aiming at the middle of a box nothing was drawn in.
type Alignment int

const (
	// AlignCenter is the zero value: the anchor stands across the frame's
	// middle, which is what a frame cut to the anchor's own size wants.
	AlignCenter Alignment = iota

	// AlignLeading stands the anchor against the frame's leading edge.
	AlignLeading

	// AlignTrailing stands the anchor against the frame's trailing edge.
	AlignTrailing
)

// outsideMargin is how far the outside-press absorber reaches beyond the
// caller's frame on every side. Presses land anywhere in the window and
// every one of them outside this popover dismisses it, so the margin is
// simply larger than any display.
const outsideMargin = unit.Dp(8192)

// tailRun is the tail's span along the surface edge it stands on. Its depth
// is not a constant: it is whatever gap the placement left between the
// surface and the anchor, so the tail always bridges exactly.
const tailRun = unit.Dp(12)

// strokeWidth is the surface outline's weight, and the depth the tail's fill
// reaches back into the surface so that outline is interrupted rather than
// drawn across the tail's base.
const strokeWidth = unit.Dp(1)

// Props configures a Popover. Anchor must be non-nil; Content may be nil
// (the surface renders as an empty rounded rectangle of minimum size).
// OnDismiss is invoked when (a) a pointer.Press lands outside both the
// anchor and surface bounds, or (b) another popover in the same Arbiter
// takes arbitration top. Either way it is called once per dismissal, during
// layout, and must not draw. OnDismiss may be nil.
type Props struct {
	// Open emits true to show the popover and false to hide it. A nil
	// Open is treated as a constant false (popover never opens).
	//
	// Use this when open-ness is model state carried on a stream. Use
	// OpenNow instead when it is frame-scoped UI state the caller owns.
	Open rx.Observable[bool]

	// OpenNow reports whether the popover is open, read during layout on the
	// frame goroutine, once per frame. Use this for open-ness that nothing
	// outside the frame ever asks about — a per-row confirm, a context menu
	// — held by the caller as a plain bool rather than pushed onto a bus and
	// mirrored back.
	//
	// A non-nil OpenNow is the whole answer and Open is ignored. Popover then
	// re-emits only when the theme changes, because the flag no longer needs
	// an emission to be seen: the layout.Widget already runs every frame and reads
	// it there. Everything downstream is unaffected — the arbitration claim
	// and release are edges over whatever this returns, exactly as they are
	// over Open.
	OpenNow func() bool

	Anchor    layout.Widget
	Content   layout.Widget
	Placement Placement
	OnDismiss func(gtx layout.Context)

	// Align is which edge of the frame the anchor is stood against. The
	// zero value centres it. See [Alignment] and the package doc's frame
	// contract.
	Align Alignment

	// AvailableRoom, if non-nil, is asked on every frame the surface is
	// drawn for the window's reported available room: the rectangle the
	// surface may occupy, in the coordinates the popover is laid out in. A
	// window passes [attachment.AvailableRoom], which reports the room the
	// window set as the layouts above moved it. When none is reported, by a
	// nil AvailableRoom or a window that set no room, the surface is placed
	// inside the frame: it keeps the side asked for and is shifted inside
	// the frame across that side. Only a window that reports its room has
	// the surface flip, scroll and stay inside the window.
	AvailableRoom func(gtx layout.Context) image.Rectangle

	// Arbiter is the set of popovers this one arbitrates within: opening it
	// dismisses whichever popover of the same set was open. Give each window
	// its own (see Arbiter). A nil Arbiter gets this popover one of its own,
	// so it arbitrates with nobody: sharing is the explicit act.
	Arbiter *Arbiter
}

type resolvedTokens struct {
	color   tokens.PlatformColors
	spacing tokens.SpacingScale
	radius  tokens.RadiusScale
	// elevation is snapshotted so a theme elevation change re-emits the
	// layout.Widget; the shadow's reach resolves through effects/depth,
	// which reads the default tokens.Elevation scale.
	elevation tokens.ElevationScale
}

// Popover returns an rx.Observable[layout.Widget] that emits a new one
// whenever the theme or Open state changes. State (the arbitration hold,
// event tags) persists across emissions in the rx.Defer scope.
//
// With Props.OpenNow set the layout.Widget reads the flag itself, every frame,
// and
// the stream carries only the theme.
func Popover(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	open := props.Open
	if open == nil || props.OpenNow != nil {
		open = rx.Of(false)
	}
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest4(t.Platform, t.Spacing, t.Radius, t.Elevation),
			func(n rx.Tuple4[tokens.PlatformColors, tokens.SpacingScale, tokens.RadiusScale, tokens.ElevationScale]) resolvedTokens {
				return resolvedTokens{color: n.First, spacing: n.Second, radius: n.Third, elevation: n.Fourth}
			},
		)
	})
	inputs := rx.CombineLatest2(resolved, open)
	return rx.Defer(func() rx.Observable[layout.Widget] {
		st := newState(props)
		return rx.Map(inputs, func(next rx.Tuple2[resolvedTokens, bool]) layout.Widget {
			tok, emitted := next.First, next.Second
			return func(gtx layout.Context) layout.Dimensions {
				// Open-ness is either the last value the stream carried or,
				// for a caller that owns it as frame state, whatever OpenNow
				// says on this frame. Read here and not in the Map, because
				// the point of OpenNow is that no emission stands between
				// the flag changing and the frame that shows it.
				openNow := emitted
				if props.OpenNow != nil {
					openNow = props.OpenNow()
				}
				// Arbitration is frame state. The claim and the release
				// happen here, on the frame goroutine that will draw the
				// result, rather than where the Open observable emitted on
				// another one. The emitted layout.Widget is re-invoked every frame
				// until the next emission replaces it, so both transitions
				// are guarded by st.holds and run exactly once.
				switch {
				case openNow && !st.holds:
					st.holds = true
					st.rows = list.NewState()
					st.arb.claim(gtx, st)
				case !openNow && st.holds:
					st.holds = false
					st.arb.release(st)
				}
				// A popover overtaken by a later claimant was dismissed at
				// that moment; it keeps drawing until the caller's Open
				// catches up, but it is no longer live and takes no input.
				live := openNow && st.arb.isTop(st)
				return drawPopover(gtx, props, tok, st, openNow, live)
			}
		})
	})
}

// Render produces a layout.Widget for a popover with pre-resolved tokens
// and an explicit open flag. Intended for golden-image testing and static
// demonstrations; production code should use Popover. The returned
// layout.Widget
// performs no input handling: pass open=true to render the anchor +
// floating surface, open=false to render only the anchor.
func Render(
	props Props,
	open bool,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
) layout.Widget {
	tok := resolvedTokens{color: colors, spacing: sp, radius: rad}
	st := newState(props)
	return func(gtx layout.Context) layout.Dimensions {
		return drawPopover(gtx, props, tok, st, open, false)
	}
}

// popoverState holds this popover's arbitration set and hold flag, the
// dismissal callback the arbiter fires when someone else takes top, and the
// three event tags routed by processInput. One instance is owned by each
// Popover subscription (and by static Render invocations, where input
// handling and arbitration are both inert). Its address is also the
// popover's identity inside the Arbiter.
type popoverState struct {
	arb     *Arbiter
	dismiss func(gtx layout.Context)
	holds   bool

	// rows scrolls the content of a surface taller than the side it wins.
	rows *list.State

	outsideTag int
	anchorTag  int
	surfaceTag int
}

func newState(props Props) *popoverState {
	arb := props.Arbiter
	if arb == nil {
		arb = NewArbiter()
	}
	return &popoverState{arb: arb, dismiss: props.OnDismiss, rows: list.NewState()}
}

// drawPopover stands the anchor at Align's edge of the frame, then — when
// open — the floating surface beside it, placed in the available room by
// [drawSurface], plus a tail seated on the anchor. When live, it also
// registers three event tags (outside, anchor, surface) and dispatches the
// events drained for them.
func drawPopover(
	gtx layout.Context,
	props Props,
	tok resolvedTokens,
	st *popoverState,
	openNow, live bool,
) layout.Dimensions {
	frame := gtx.Constraints.Max

	// 1. Record the anchor into a macro to measure its dims; stand it at
	//    Align's edge of the frame. That rect is the DRAWN anchor — the
	//    basis for the surface's position and for where the tail points.
	anchorMacro := op.Record(gtx.Ops)
	anchorGtx := gtx
	anchorGtx.Constraints = layout.Constraints{Max: frame}
	var anchorDims layout.Dimensions
	if props.Anchor != nil {
		anchorDims = props.Anchor(anchorGtx)
	}
	anchorOps := anchorMacro.Stop()
	anchorPos := image.Pt(alignX(props.Align, frame.X, anchorDims.Size.X), (frame.Y-anchorDims.Size.Y)/2)
	anchorRect := image.Rectangle{Min: anchorPos, Max: anchorPos.Add(anchorDims.Size)}

	// 2. Outside-press absorber. The frame is the room this popover may
	//    use, not the window, so the absorber extends a wide margin beyond
	//    it on every side to catch presses anywhere in the window.
	//    Registered first so the anchor's clip tag, added after it, and the
	//    surface's, deferred past it, both win for presses inside their own
	//    bounds.
	if live {
		margin := gtx.Dp(outsideMargin)
		outsideClip := clip.Rect{
			Min: image.Pt(-margin, -margin),
			Max: image.Pt(frame.X+margin, frame.Y+margin),
		}.Push(gtx.Ops)
		event.Op(gtx.Ops, &st.outsideTag)
		outsideClip.Pop()
	}

	// 3. Anchor: anchor-absorber tag, then the recorded anchor ops at the
	//    aligned offset. The absorber catches presses on the anchor so
	//    they do not bubble to the outside-absorber and dismiss.
	{
		anchorOff := op.Offset(anchorPos).Push(gtx.Ops)
		if live {
			anchorClip := clip.Rect{Max: anchorDims.Size}.Push(gtx.Ops)
			event.Op(gtx.Ops, &st.anchorTag)
			anchorClip.Pop()
		}
		anchorOps.Add(gtx.Ops)
		anchorOff.Pop()
	}

	// 4. Surface + tail + content, only when open: drawn at once in the
	//    available room the props report, or, where a layout set
	//    attachment.Surfaces on the context, recorded there to be drawn in
	//    the room that layout passes.
	if openNow {
		draw := func(gtx layout.Context, room image.Rectangle) {
			room, bounded := roomOrFrame(room, props.Placement, frame)
			drawSurface(gtx, props, tok, st, live, anchorRect, frame, room, bounded)
		}
		if !attachment.Record(gtx, draw) {
			room := unreported
			if props.AvailableRoom != nil {
				room = props.AvailableRoom(gtx)
			}
			draw(gtx, room)
		}
	}

	if live {
		processInput(gtx, props, st)
	}

	return layout.Dimensions{Size: frame}
}

// alignX is where across a frame of width frameW a shape of width shapeW
// stands, for each Alignment. A shape wider than the frame starts at the
// leading edge whatever the alignment says, because there is no room to
// stand it anywhere else.
func alignX(a Alignment, frameW, shapeW int) int {
	slack := frameW - shapeW
	if slack <= 0 {
		return 0
	}
	switch a {
	case AlignLeading:
		return 0
	case AlignTrailing:
		return slack
	default:
		return slack / 2
	}
}

// unbounded is the extent along the placement's axis of the room a popover
// with no room reported is placed in: far enough that no surface is kept off
// the side asked for.
const unbounded = 1 << 20

// unboundedContent is the height content is measured at inside a reported
// room: its own height, which the surface then caps to the side it wins.
// It is gioui.org/layout's own choice for a scrolled child's extent.
const unboundedContent = 1e6

// unreported is the rectangle [attachment.AvailableRoom] reports where no
// window set the available room. It bounds nothing, so a popover given it is
// placed against its own frame instead.
var unreported = attachment.AvailableRoom(layout.Context{})

// roomOrFrame is the room the surface is placed in, in the popover's own
// coordinates, and whether a window reported it. Where none was reported it
// is the frame across the placement's axis and unbounded along it, so the
// surface keeps the side asked for and is held inside the frame across it.
//
// The room is known to be unreported by its size, not its position: a layout
// that records surfaces and places them at an offset moves the unbounded
// room by that offset, and the moved room still bounds nothing.
func roomOrFrame(room image.Rectangle, p Placement, frame image.Point) (image.Rectangle, bool) {
	if room.Size() != unreported.Size() {
		return room, true
	}
	if p.Vertical() {
		return image.Rect(0, -unbounded, frame.X, unbounded), false
	}
	return image.Rect(-unbounded, 0, unbounded, frame.Y), false
}

// drawSurface lays out the content and draws the floating surface, its tail
// and the content beside anchor inside room, recorded and handed to op.Defer
// so they paint and hit-test above every sibling the window lays out after
// this popover's slot (see the package doc). The surface absorbs presses;
// the tail is a triangular path bridging the gap to the anchor, drawn in the
// surface fill colour and carrying the surface's own edge around it. It
// floats: the window background under the platform's shadow, inside the
// platform's separator.
//
// The content is measured at half the frame. Inside a reported room it is
// measured at its own height and no wider than the room holds, and a surface
// taller than the side it wins is capped to that side's length, its content
// scrolling in a viewport with the platform's scroll bar.
func drawSurface(
	gtx layout.Context,
	props Props,
	tok resolvedTokens,
	st *popoverState,
	live bool,
	anchor image.Rectangle,
	frame image.Point,
	room image.Rectangle,
	bounded bool,
) {
	r := gtx.Dp(unit.Dp(tok.radius.Md))
	pad := gtx.Dp(unit.Dp(tok.spacing.S3))
	gap := gtx.Dp(unit.Dp(tok.spacing.S2))
	tailW := gtx.Dp(tailRun)

	preferred := props.Placement
	bar := scrollbar.FromTokens(tok.color, tok.color.WindowBackground)
	lane := 0
	if list.AnchorFor(bar) == list.Occupy {
		lane = gtx.Dp(bar.Width())
	}
	contentMax := image.Pt(frame.X/2, frame.Y/2)
	if bounded {
		// Across the room the content is no wider than the room less the
		// padding and a standing scroll bar's lane, and beside the anchor no
		// wider than the wider of the two sides. Down the room it is measured
		// at its own height: a surface taller than the side it wins is capped
		// below and its content scrolls.
		wide := room.Dx()
		if !preferred.Vertical() {
			wide = max(attachment.SideRoom(anchor, preferred, gap, room), attachment.SideRoom(anchor, preferred.Opposite(), gap, room))
		}
		contentMax = image.Pt(max(min(frame.X/2, wide-2*pad-lane), 0), unboundedContent)
	}
	contentMacro := op.Record(gtx.Ops)
	contentGtx := gtx
	contentGtx.Constraints = layout.Constraints{Max: contentMax}
	var contentDims layout.Dimensions
	if props.Content != nil {
		contentDims = props.Content(contentGtx)
	}
	contentOps := contentMacro.Stop()

	minW := gtx.Dp(unit.Dp(48))
	minH := gtx.Dp(unit.Dp(24))
	size := image.Pt(max(contentDims.Size.X+2*pad, minW), max(contentDims.Size.Y+2*pad, minH))

	side := attachment.TakeSide(anchor, size, preferred, gap, room)

	// A surface taller than the side it wins is capped to that side's
	// length, above or below the anchor, or to the room's height beside it,
	// and its content scrolls inside it. A standing scroll bar takes a lane
	// of its own, so the surface is that much wider.
	limit := room.Dy()
	if side.Vertical() {
		limit = attachment.SideRoom(anchor, side, gap, room)
	}
	scrolls := bounded && size.Y > limit
	if scrolls {
		size.Y = limit
		size.X += lane
	}

	surfaceRect := attachment.ShiftInward(attachment.Beside(anchor, size, side, gap), side, room, 0)

	floating := op.Record(gtx.Ops)
	fill := tok.color.WindowBackground
	// The edge is the platform's separator laid over the fill it circles,
	// and the tail's own edge is that same line for the same reason.
	edge := vgcolor.Flatten(tok.color.Separator, fill)
	stroke := float32(gtx.Dp(strokeWidth))
	surfOff := op.Offset(surfaceRect.Min).Push(gtx.Ops)
	surfRRect := clip.RRect{
		Rect: image.Rectangle{Max: surfaceRect.Size()},
		SE:   r, SW: r, NE: r, NW: r,
	}
	castShadow(gtx, image.Rectangle{Max: surfaceRect.Size()}, r, tok.color)
	paint.FillShape(gtx.Ops, fill, surfRRect.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, edge, clip.Stroke{
		Path:  surfRRect.Path(gtx.Ops),
		Width: stroke,
	}.Op())
	if live {
		absorbClip := clip.Rect{Max: surfaceRect.Size()}.Push(gtx.Ops)
		event.Op(gtx.Ops, &st.surfaceTag)
		absorbClip.Pop()
	}
	contentOff := op.Offset(image.Pt(pad, pad)).Push(gtx.Ops)
	if scrolls && size.Y > 2*pad {
		viewGtx := gtx
		viewGtx.Constraints = layout.Exact(image.Pt(size.X-2*pad, size.Y-2*pad))
		list.LayoutScrollbar(viewGtx, st.rows, bar, list.AnchorFor(bar), []int{0}, func(gtx layout.Context, _ int) layout.Dimensions {
			contentOps.Add(gtx.Ops)
			return contentDims
		})
	} else if !scrolls {
		contentOps.Add(gtx.Ops)
	}
	contentOff.Pop()
	surfOff.Pop()

	drawTail(gtx, anchor, surfaceRect, side, tailW, r, fill, edge, stroke)
	op.Defer(gtx.Ops, floating.Stop())
}

// drawTail paints the tail bridging the gap between the surface and the
// anchor, with its tip on the anchor. Its base stands on the surface edge
// facing the anchor and its depth is that gap, so the tail meets both and
// floats over neither. Its centre is the DRAWN anchor's midline, held back
// from the surface's rounded corners by the radius so the base always
// stands on the flat run of the edge.
//
// The fill reaches one stroke back into the surface, covering the outline
// under the base; the two slanted sides are then stroked in the same colour.
// The surface's edge therefore runs into the tail and out again rather than
// being drawn straight through it. Coordinates are frame-absolute (no
// transform is on the stack when this is called).
func drawTail(gtx layout.Context, anchor, surface image.Rectangle, p attachment.Side, run, radius int, fill, edge color.NRGBA, stroke float32) {
	var (
		depth     int
		centre    int
		lo, hi    int
		base, tip float32
		vertical  = p.Vertical()
		toward    float32
	)
	switch p {
	case attachment.Top:
		depth = anchor.Min.Y - surface.Max.Y
		centre, lo, hi = (anchor.Min.X+anchor.Max.X)/2, surface.Min.X+radius, surface.Max.X-radius
		base, toward = float32(surface.Max.Y), 1
	case attachment.Bottom:
		depth = surface.Min.Y - anchor.Max.Y
		centre, lo, hi = (anchor.Min.X+anchor.Max.X)/2, surface.Min.X+radius, surface.Max.X-radius
		base, toward = float32(surface.Min.Y), -1
	case attachment.Left:
		depth = anchor.Min.X - surface.Max.X
		centre, lo, hi = (anchor.Min.Y+anchor.Max.Y)/2, surface.Min.Y+radius, surface.Max.Y-radius
		base, toward = float32(surface.Max.X), 1
	case attachment.Right:
		depth = surface.Min.X - anchor.Max.X
		centre, lo, hi = (anchor.Min.Y+anchor.Max.Y)/2, surface.Min.Y+radius, surface.Max.Y-radius
		base, toward = float32(surface.Min.X), -1
	}
	if depth <= 0 || hi-lo < run {
		return
	}
	tip = base + toward*float32(depth)
	half := float32(run) / 2
	c := float32(min(max(centre, lo+run/2), hi-run/2))

	// The base sits one stroke inside the surface so the fill covers the
	// outline it interrupts; the stroked sides start on the edge itself, so
	// the two outlines meet.
	inner := base - toward*stroke
	pt := func(along, across float32) f32.Point {
		if vertical {
			return f32.Point{X: along, Y: across}
		}
		return f32.Point{X: across, Y: along}
	}

	var body clip.Path
	body.Begin(gtx.Ops)
	body.MoveTo(pt(c-half, inner))
	body.LineTo(pt(c+half, inner))
	body.LineTo(pt(c, tip))
	body.Close()
	paint.FillShape(gtx.Ops, fill, clip.Outline{Path: body.End()}.Op())

	var sides clip.Path
	sides.Begin(gtx.Ops)
	sides.MoveTo(pt(c-half, base))
	sides.LineTo(pt(c, tip))
	sides.LineTo(pt(c+half, base))
	paint.FillShape(gtx.Ops, edge, clip.Stroke{Path: sides.End(), Width: stroke}.Op())
}

// processInput drains the press events for this frame: anchor- and
// surface-tag presses are absorbed silently; outside-tag presses invoke
// OnDismiss.
func processInput(gtx layout.Context, props Props, st *popoverState) {
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &st.anchorTag, Kinds: pointer.Press}); !ok {
			break
		}
	}
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &st.surfaceTag, Kinds: pointer.Press}); !ok {
			break
		}
	}
	for {
		e, ok := gtx.Event(pointer.Filter{Target: &st.outsideTag, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pe, ok := e.(pointer.Event); ok && pe.Kind == pointer.Press {
			fire(gtx, props.OnDismiss)
		}
	}
}

// fire invokes cb when cb is non-nil. Centralised so OnDismiss is never
// called against a nil pointer.
func fire(gtx layout.Context, cb func(gtx layout.Context)) {
	if cb != nil {
		cb(gtx)
	}
}

// castShadow paints the platform's floating shadow under a surface: the
// measured colour and coverage handed to effects/depth, which carries them out
// over the reach the same captures measure.
func castShadow(gtx layout.Context, bounds image.Rectangle, radius int, c tokens.PlatformColors) {
	depth.Shadow(gtx, bounds, radius, c.FloatingShadow)
}
