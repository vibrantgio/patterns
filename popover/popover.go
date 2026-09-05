// Package popover provides the Patterns Popover pattern: an anchored
// elevated surface placed adjacent to a caller-supplied anchor widget,
// with a small triangular tail glyph pointing at the anchor. Outside-
// click dismissal and popover-vs-popover arbitration are frame state:
// opening a second popover dismisses the first, through a plain Arbiter
// written and read during layout on the frame goroutine. See ADR-008 and
// arbitration.go.
//
// Elevation: the popover surface (and its tail) fills at
// SurfaceAt(Level3) (Neutral step 400), the deepest rung of the ladder.
// A popover is an unscrimmed, shadowless transient overlay — unlike the
// modal (level 2), which has a scrim, and the toast, which takes no rung
// at all and keeps its cast shadow, the popover's fill plus its 1 dp
// Neutral step-500 stroke are its only separation cues, so it takes the
// deepest tonal step. components/input's dropdown menu, the
// same overlay class, sits at the same level.
//
// Popover is a callable Go function consuming a components theme
// observable, returning a stream of layout.Widget. The source is
// intentionally short and free of opaque configuration — copy it into
// your own app and modify as needed.
//
// THE CANVAS IS THE ROOM. The popover stands its anchor in the canvas its
// caller hands it — at [Alignment]'s edge of it — and keeps the surface inside
// that canvas across the placement's cross axis, nudging it back where it
// would run off rather than letting it clip. So the canvas must be the room
// the caller actually has, the content column or the window, and not a box
// cut to the anchor's own size: a canvas that cannot hold the surface is not
// a bound, and the popover leaves such a surface where the anchor puts it.
// Aligning the anchor is the popover's job for the same reason — it is the
// one place the drawn anchor, the surface and the room are all known, so an
// anchor handed here reports the shape it drew and nothing wider.
//
// Open/close is instantaneous in this package; entrance/exit transitions
// are deferred to a later Effects-integration goal. Automatic flip to the
// opposite side of the anchor is deferred: the clamp is a cross-axis nudge,
// not a placement search.
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
	"github.com/vibrantgio/patterns/internal/outline"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Placement is the side of the anchor on which the popover surface sits.
type Placement int

const (
	Top Placement = iota
	Bottom
	Left
	Right
)

// Alignment is where across the canvas the popover stands its anchor.
//
// It is the placement half of the canvas contract: the anchor draws the
// shape it has and this says which of the canvas's edges that shape is set
// against, so the popover knows the drawn rect exactly and can aim the tail
// at it. An anchor that widens its own report to reach an edge instead
// leaves the popover aiming at the middle of a box nothing was drawn in.
type Alignment int

const (
	// AlignCenter is the zero value: the anchor stands across the canvas's
	// middle, which is what a canvas cut to the anchor's own size wants.
	AlignCenter Alignment = iota

	// AlignLeading stands the anchor against the canvas's leading edge.
	AlignLeading

	// AlignTrailing stands the anchor against the canvas's trailing edge.
	AlignTrailing
)

// outsideMargin is how far the outside-press absorber reaches beyond the
// caller's canvas on every side. Presses land anywhere in the window and
// every one of them outside this popover dismisses it, so the margin is
// simply larger than any display.
const outsideMargin = unit.Dp(8192)

// tailRun is the tail's span along the surface edge it stands on. Its depth
// is not a constant: it is whatever gap the placement left between the
// surface and the anchor, so the glyph always bridges exactly.
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
	// an emission to be seen: the widget already runs every frame and reads
	// it there. Everything downstream is unaffected — the arbitration claim
	// and release are edges over whatever this returns, exactly as they are
	// over Open.
	OpenNow func() bool

	Anchor    layout.Widget
	Content   layout.Widget
	Placement Placement
	OnDismiss func(gtx layout.Context)

	// Align is which edge of the canvas the anchor is stood against. The
	// zero value centres it. See [Alignment] and the package doc's canvas
	// contract.
	Align Alignment

	// Arbiter is the set of popovers this one arbitrates within: opening it
	// dismisses whichever popover of the same set was open. Give each window
	// its own (see Arbiter). A nil Arbiter gets this popover one of its own,
	// so it arbitrates with nobody: sharing is the explicit act.
	Arbiter *Arbiter
}

type resolvedTokens struct {
	color   tokens.ColorTokens
	spacing tokens.SpacingScale
	radius  tokens.RadiusScale
	// elevation is snapshotted so a theme elevation change re-emits the
	// widget; the surface fill resolves through SurfaceAt, which reads
	// the default tokens.Elevation scale.
	elevation tokens.ElevationScale
}

// Popover returns an rx.Observable[layout.Widget] that emits a new widget
// whenever the theme or Open state changes. State (the arbitration hold,
// event tags) persists across emissions in the rx.Defer scope.
//
// With Props.OpenNow set the widget reads the flag itself, every frame, and
// the stream carries only the theme.
func Popover(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	open := props.Open
	if open == nil || props.OpenNow != nil {
		open = rx.Of(false)
	}
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest4(t.Color, t.Spacing, t.Radius, t.Elevation),
			func(n rx.Tuple4[tokens.ColorTokens, tokens.SpacingScale, tokens.RadiusScale, tokens.ElevationScale]) resolvedTokens {
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
				// another one. The emitted widget is re-invoked every frame
				// until the next emission replaces it, so both transitions
				// are guarded by st.holds and run exactly once.
				switch {
				case openNow && !st.holds:
					st.holds = true
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
// demonstrations; production code should use Popover. The returned widget
// performs no input handling: pass open=true to render the anchor +
// floating surface, open=false to render only the anchor.
func Render(
	props Props,
	open bool,
	colors tokens.ColorTokens,
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

	outsideTag int
	anchorTag  int
	surfaceTag int
}

func newState(props Props) *popoverState {
	arb := props.Arbiter
	if arb == nil {
		arb = NewArbiter()
	}
	return &popoverState{arb: arb, dismiss: props.OnDismiss}
}

// drawPopover stands the anchor at Align's edge of the canvas, then — when
// open — the floating surface adjacent to it per Placement, nudged back
// inside the canvas where it would run off, plus a tail glyph seated on the
// anchor. When live, it also registers three event tags (outside, anchor,
// surface) and dispatches the events drained for them.
func drawPopover(
	gtx layout.Context,
	props Props,
	tok resolvedTokens,
	st *popoverState,
	openNow, live bool,
) layout.Dimensions {
	canvas := gtx.Constraints.Max
	r := gtx.Dp(unit.Dp(tok.radius.Md))
	pad := gtx.Dp(unit.Dp(tok.spacing.S3))
	gap := gtx.Dp(unit.Dp(tok.spacing.S2))
	tailW := gtx.Dp(tailRun)

	// 1. Record the anchor into a macro to measure its dims; stand it at
	//    Align's edge of the canvas. That rect is the DRAWN anchor — the
	//    basis for the surface's position and for where the tail points.
	anchorMacro := op.Record(gtx.Ops)
	anchorGtx := gtx
	anchorGtx.Constraints = layout.Constraints{Max: canvas}
	var anchorDims layout.Dimensions
	if props.Anchor != nil {
		anchorDims = props.Anchor(anchorGtx)
	}
	anchorOps := anchorMacro.Stop()
	anchorPos := image.Pt(alignX(props.Align, canvas.X, anchorDims.Size.X), (canvas.Y-anchorDims.Size.Y)/2)
	anchorRect := image.Rectangle{Min: anchorPos, Max: anchorPos.Add(anchorDims.Size)}

	// 2. If open, record the content into a macro to measure its dims;
	//    surface = content + 2*pad in both axes (clamped to a min size).
	var (
		surfaceRect image.Rectangle
		contentOps  op.CallOp
		contentDims layout.Dimensions
	)
	if openNow {
		contentMacro := op.Record(gtx.Ops)
		contentGtx := gtx
		contentGtx.Constraints = layout.Constraints{Max: image.Pt(canvas.X/2, canvas.Y/2)}
		if props.Content != nil {
			contentDims = props.Content(contentGtx)
		}
		contentOps = contentMacro.Stop()

		surfW := contentDims.Size.X + 2*pad
		surfH := contentDims.Size.Y + 2*pad
		minW := gtx.Dp(unit.Dp(48))
		minH := gtx.Dp(unit.Dp(24))
		if surfW < minW {
			surfW = minW
		}
		if surfH < minH {
			surfH = minH
		}

		anchorMidX := (anchorRect.Min.X + anchorRect.Max.X) / 2
		anchorMidY := (anchorRect.Min.Y + anchorRect.Max.Y) / 2
		switch props.Placement {
		case Top:
			x := anchorMidX - surfW/2
			y := anchorRect.Min.Y - gap - surfH
			surfaceRect = image.Rect(x, y, x+surfW, y+surfH)
		case Bottom:
			x := anchorMidX - surfW/2
			y := anchorRect.Max.Y + gap
			surfaceRect = image.Rect(x, y, x+surfW, y+surfH)
		case Left:
			x := anchorRect.Min.X - gap - surfW
			y := anchorMidY - surfH/2
			surfaceRect = image.Rect(x, y, x+surfW, y+surfH)
		case Right:
			x := anchorRect.Max.X + gap
			y := anchorMidY - surfH/2
			surfaceRect = image.Rect(x, y, x+surfW, y+surfH)
		}
		surfaceRect = clampToCanvas(surfaceRect, canvas, props.Placement)
	}

	// 3. Outside-press absorber. The canvas is the room this popover may
	//    use, not the window, so the absorber extends a wide margin beyond
	//    it on every side to catch presses anywhere in the window.
	//    Registered first so that anchor- and surface-clip tags (registered
	//    later) win for presses inside their own bounds.
	if live {
		margin := gtx.Dp(outsideMargin)
		outsideClip := clip.Rect{
			Min: image.Pt(-margin, -margin),
			Max: image.Pt(canvas.X+margin, canvas.Y+margin),
		}.Push(gtx.Ops)
		event.Op(gtx.Ops, &st.outsideTag)
		outsideClip.Pop()
	}

	// 4. Anchor: anchor-absorber tag, then the recorded anchor ops at the
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

	// 5. Surface + tail + content, only when open. The surface absorbs
	//    presses; the tail is a triangular path bridging the gap to the
	//    anchor, drawn in the surface fill colour and carrying the surface's
	//    own edge around it. Level 3 on the elevation ladder: an unscrimmed,
	//    shadowless transient overlay separates by fill alone (see the
	//    package doc).
	if openNow {
		fill := tok.color.SurfaceAt(tokens.Level3)
		// The surface's edge is derived against the storey it circles — the
		// same Level3 the fill is painted at, named once for both, and the
		// tail's own edge is the same ink for the same reason.
		edge := outline.Ink(tok.color, tok.color.SurfaceAt(tokens.Level3))
		stroke := float32(gtx.Dp(strokeWidth))
		surfOff := op.Offset(surfaceRect.Min).Push(gtx.Ops)
		surfRRect := clip.RRect{
			Rect: image.Rectangle{Max: surfaceRect.Size()},
			SE:   r, SW: r, NE: r, NW: r,
		}
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
		contentOps.Add(gtx.Ops)
		contentOff.Pop()
		surfOff.Pop()

		drawTail(gtx, anchorRect, surfaceRect, props.Placement, tailW, r, fill, edge, stroke)
	}

	if live {
		processInput(gtx, props, st)
	}

	return layout.Dimensions{Size: canvas}
}

// alignX is where across a canvas of width canvasW a shape of width shapeW
// stands, for each Alignment. A shape wider than the canvas starts at the
// leading edge whatever the alignment says, because there is no room to
// stand it anywhere else.
func alignX(a Alignment, canvasW, shapeW int) int {
	slack := canvasW - shapeW
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

// clampToCanvas nudges the surface back inside the canvas along the axis the
// placement does not travel on — the axis on which leaving the canvas is an
// overflow rather than the point. The placement axis is left alone: a
// Bottom-placed surface is meant to hang below the canvas.
//
// A canvas too small to hold the surface is not a bound and is left alone:
// shoving an over-wide surface against one edge only moves the overflow to
// the other, and the caller that cut its canvas to the anchor has said
// nothing about the room it has. The tail is aimed at the anchor rather than
// at the surface, so it keeps pointing where it did through the nudge.
func clampToCanvas(surface image.Rectangle, canvas image.Point, p Placement) image.Rectangle {
	switch p {
	case Top, Bottom:
		if surface.Dx() > canvas.X {
			return surface
		}
		if surface.Min.X < 0 {
			return surface.Add(image.Pt(-surface.Min.X, 0))
		}
		if surface.Max.X > canvas.X {
			return surface.Add(image.Pt(canvas.X-surface.Max.X, 0))
		}
	case Left, Right:
		if surface.Dy() > canvas.Y {
			return surface
		}
		if surface.Min.Y < 0 {
			return surface.Add(image.Pt(0, -surface.Min.Y))
		}
		if surface.Max.Y > canvas.Y {
			return surface.Add(image.Pt(0, canvas.Y-surface.Max.Y))
		}
	}
	return surface
}

// drawTail paints the glyph bridging the gap between the surface and the
// anchor, with its tip on the anchor. Its base stands on the surface edge
// facing the anchor and its depth is that gap, so the glyph meets both and
// floats over neither. Its centre is the DRAWN anchor's midline, held back
// from the surface's rounded corners by the radius so the base always
// stands on the flat run of the edge.
//
// The fill reaches one stroke back into the surface, covering the outline
// under the base; the two slanted sides are then stroked in the same ink.
// The surface's edge therefore runs into the tail and out again rather than
// being drawn straight through it. Coordinates are canvas-absolute (no
// transform is on the stack when this is called).
func drawTail(gtx layout.Context, anchor, surface image.Rectangle, p Placement, run, radius int, fill, edge color.NRGBA, stroke float32) {
	var (
		depth     int
		centre    int
		lo, hi    int
		base, tip float32
		vertical  = p == Top || p == Bottom
		toward    float32
	)
	switch p {
	case Top:
		depth = anchor.Min.Y - surface.Max.Y
		centre, lo, hi = (anchor.Min.X+anchor.Max.X)/2, surface.Min.X+radius, surface.Max.X-radius
		base, toward = float32(surface.Max.Y), 1
	case Bottom:
		depth = surface.Min.Y - anchor.Max.Y
		centre, lo, hi = (anchor.Min.X+anchor.Max.X)/2, surface.Min.X+radius, surface.Max.X-radius
		base, toward = float32(surface.Min.Y), -1
	case Left:
		depth = anchor.Min.X - surface.Max.X
		centre, lo, hi = (anchor.Min.Y+anchor.Max.Y)/2, surface.Min.Y+radius, surface.Max.Y-radius
		base, toward = float32(surface.Max.X), 1
	case Right:
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
