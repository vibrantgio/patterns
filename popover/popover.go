// Package popover provides the Cadence Popover pattern: an anchored
// elevated surface placed adjacent to a caller-supplied anchor widget,
// with a small triangular tail glyph pointing at the anchor. Outside-
// click dismissal and popover-vs-popover arbitration are frame state:
// opening a second popover dismisses the first, through a plain Arbiter
// written and read during layout on the frame goroutine. See ADR-008 and
// arbitration.go; before G0C.1 this ran through a prism/coordination
// Subject that nothing ever subscribed to.
//
// Elevation (goal G-E2): the popover surface (and its tail) fills at
// SurfaceAt(Level3) (Neutral step 400), the deepest rung of the ladder.
// A popover is an unscrimmed, shadowless transient overlay — unlike the
// modal (level 2), which has a scrim, and the toast (level-2 base),
// which keeps its cast shadow and accent outline, the popover's fill
// plus its 1 dp Neutral step-500 stroke are its only separation cues,
// so it takes the deepest tonal step. prism/input's dropdown menu, the
// same overlay class, sits at the same level.
//
// The package follows the Phase 4 Composition contract: Popover is a
// callable Go function consuming a Prism theme observable, returning a
// stream of layout.Widget. The source is intentionally short and free of
// opaque configuration — copy it into your own app and modify as needed.
//
// Open/close is instantaneous in this package; entrance/exit transitions
// are deferred to a later Pulse-integration goal. No collision-aware
// reflow — if the chosen Placement would clip the viewport, the surface
// clips. Automatic flip is deferred.
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

// outsideMargin is how far the outside-press absorber reaches beyond the
// caller's canvas on every side. The popover cannot know the window bounds
// from inside its canvas, so the margin is simply larger than any display.
const outsideMargin = unit.Dp(8192)

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
	// This is the spelling for a popover whose open-ness is model state
	// carried on a stream — ADR-008 destination 1. Use OpenNow instead when
	// it is frame-scoped UI state the caller owns.
	Open rx.Observable[bool]

	// OpenNow reports whether the popover is open, read during layout on the
	// frame goroutine, once per frame. It is the spelling for ADR-008
	// destination 2: a per-row confirm, a context menu — open-ness that
	// nothing outside the frame ever asks about, held by the caller as a
	// plain bool rather than pushed onto a bus and mirrored back.
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

// drawPopover lays out the anchor at the canvas centre, then — when open —
// the floating surface adjacent to the anchor per Placement plus a
// triangular tail glyph pointing at the anchor. When live, it also
// registers three event tags (outside, anchor, surface) and dispatches
// the events drained for them.
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
	tailH := gtx.Dp(unit.Dp(6))
	tailW := gtx.Dp(unit.Dp(12))

	// 1. Record the anchor into a macro to measure its dims; centre it
	//    in the canvas. The anchor's last-recorded layout rect is the
	//    basis for surface positioning math below.
	anchorMacro := op.Record(gtx.Ops)
	anchorGtx := gtx
	anchorGtx.Constraints = layout.Constraints{Max: canvas}
	var anchorDims layout.Dimensions
	if props.Anchor != nil {
		anchorDims = props.Anchor(anchorGtx)
	}
	anchorOps := anchorMacro.Stop()
	anchorPos := image.Pt((canvas.X-anchorDims.Size.X)/2, (canvas.Y-anchorDims.Size.Y)/2)
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
	}

	// 3. Outside-press absorber. The caller's canvas is often just the
	//    anchor's box (the popover-canvas coupling), so the absorber extends
	//    a wide margin beyond it on every side to catch presses anywhere in
	//    the window. Registered first so that anchor- and surface-clip tags
	//    (registered later) win for presses inside their own bounds.
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
	//    centred offset. The absorber catches presses on the anchor so
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
	//    anchor, drawn in the surface fill colour. Level 3 on the
	//    elevation ladder: an unscrimmed, shadowless transient overlay
	//    separates by fill alone (see the package doc).
	if openNow {
		fill := tok.color.SurfaceAt(tokens.Level3)
		surfOff := op.Offset(surfaceRect.Min).Push(gtx.Ops)
		surfRRect := clip.RRect{
			Rect: image.Rectangle{Max: surfaceRect.Size()},
			SE:   r, SW: r, NE: r, NW: r,
		}
		paint.FillShape(gtx.Ops, fill, surfRRect.Op(gtx.Ops))
		paint.FillShape(gtx.Ops, tok.color.Ramps.Neutral.Step(500), clip.Stroke{
			Path:  surfRRect.Path(gtx.Ops),
			Width: float32(gtx.Dp(unit.Dp(1))),
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

		drawTail(gtx, anchorRect, surfaceRect, props.Placement, tailW, tailH, fill)
	}

	if live {
		processInput(gtx, props, st)
	}

	return layout.Dimensions{Size: canvas}
}

// drawTail paints a triangle bridging the gap between the surface and the
// anchor, with its tip pointing at the anchor. The base of the triangle
// touches the surface edge facing the anchor; the tip touches the anchor
// edge facing the surface. Coordinates are canvas-absolute (no transform
// is on the stack when this is called).
func drawTail(gtx layout.Context, anchor, surface image.Rectangle, p Placement, w, h int, fill color.NRGBA) {
	fw := float32(w)
	fh := float32(h)
	var pts [3]f32.Point
	switch p {
	case Top:
		cx := float32((anchor.Min.X + anchor.Max.X) / 2)
		baseY := float32(surface.Max.Y)
		pts = [3]f32.Point{
			{X: cx - fw/2, Y: baseY},
			{X: cx + fw/2, Y: baseY},
			{X: cx, Y: baseY + fh},
		}
	case Bottom:
		cx := float32((anchor.Min.X + anchor.Max.X) / 2)
		baseY := float32(surface.Min.Y)
		pts = [3]f32.Point{
			{X: cx - fw/2, Y: baseY},
			{X: cx + fw/2, Y: baseY},
			{X: cx, Y: baseY - fh},
		}
	case Left:
		cy := float32((anchor.Min.Y + anchor.Max.Y) / 2)
		baseX := float32(surface.Max.X)
		pts = [3]f32.Point{
			{X: baseX, Y: cy - fw/2},
			{X: baseX, Y: cy + fw/2},
			{X: baseX + fh, Y: cy},
		}
	case Right:
		cy := float32((anchor.Min.Y + anchor.Max.Y) / 2)
		baseX := float32(surface.Min.X)
		pts = [3]f32.Point{
			{X: baseX, Y: cy - fw/2},
			{X: baseX, Y: cy + fw/2},
			{X: baseX - fh, Y: cy},
		}
	}
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(pts[0])
	path.LineTo(pts[1])
	path.LineTo(pts[2])
	path.Close()
	paint.FillShape(gtx.Ops, fill, clip.Outline{Path: path.End()}.Op())
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
