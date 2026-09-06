package tooltip

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/theme/tokens"
)

// These interaction tests are white-box because tooltip exposes no
// callbacks: visibility is "this state holds arbitration top". Tooltip has
// no dismissal callback to assert against — the register is the visibility
// — so the equivalent inspection happens here, against an Arbiter the test
// owns.

const intFrameW, intFrameH = 320, 240

var intFrame = image.Pt(intFrameW, intFrameH)

func intTrigger() layout.Widget {
	c := color.NRGBA{R: 80, G: 160, B: 220, A: 255}
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(unit.Dp(60)), gtx.Dp(unit.Dp(28)))
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	}
}

func intTok() resolvedTokens {
	return resolvedTokens{
		color:   tokens.DefaultLight,
		spacing: tokens.Spacing,
		radius:  tokens.RadiusScale{},
		style:   tokens.DefaultTypography.LabelSmall,
	}
}

func driveFrameAt(w layout.Widget, ops *op.Ops, r *gioinput.Router, size image.Point, now time.Time) {
	ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(size),
		Now:         now,
		Ops:         ops,
		Source:      r.Source(),
	}
	w(gtx)
	r.Frame(ops)
}

// hoverRig builds a live single-tooltip frame driver over its own Arbiter,
// and returns the arbiter, the tooltip's state and the layout.Widget.
func hoverRig(delay time.Duration) (*Arbiter, *tooltipState, layout.Widget) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	arb := NewArbiter()
	props := Props{Text: "Save", Trigger: intTrigger(), Placement: Top, Shaper: shaper, Arbiter: arb}
	st := newState(props)
	return arb, st, func(gtx layout.Context) layout.Dimensions {
		return drawTooltip(gtx, shaper, props, delay, intTok(), st, true)
	}
}

// TestHoverEntryAfterDelayShows verifies Measurable (a): hover entry
// followed by the delay elapsing takes arbitration top, which is what
// "visible" means. Before the delay, the tooltip must not hold top.
func TestHoverEntryAfterDelayShows(t *testing.T) {
	const delay = 50 * time.Millisecond
	arb, st, w := hoverRig(delay)

	r := new(gioinput.Router)
	ops := new(op.Ops)
	t0 := time.Unix(1700000000, 0)

	// Frame 1: register the hover and focus tags. Nothing in the queue yet.
	driveFrameAt(w, ops, r, intFrame, t0)
	if arb.isTop(st) {
		t.Fatalf("tooltip visible before any hover event; want hidden")
	}

	// Queue a pointer.Move at the frame centre (inside the trigger). The
	// router synthesizes pointer.Enter into the hover gesture next frame.
	r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(intFrameW/2, intFrameH/2), Source: pointer.Mouse})

	// Frame 2 at t0: hover Enter consumed, st.entryAt = t0, delay not
	// elapsed → still hidden.
	driveFrameAt(w, ops, r, intFrame, t0)
	if st.entryAt.IsZero() {
		t.Fatalf("hover Enter did not start the dwell")
	}
	if arb.isTop(st) {
		t.Fatalf("tooltip visible before delay elapsed; want hidden")
	}

	// Frame 3 at t0+delay+1ms: delay elapsed → the tooltip claims top.
	driveFrameAt(w, ops, r, intFrame, t0.Add(delay).Add(time.Millisecond))
	if !arb.isTop(st) {
		t.Fatalf("tooltip did not take arbitration top after the delay elapsed")
	}
}

// TestHoverExitHides verifies Measurable (b): once the tooltip is shown,
// hover Leave hides it — it releases top and the dwell latch resets, so a
// re-entry can show it again.
func TestHoverExitHides(t *testing.T) {
	const delay = 50 * time.Millisecond
	arb, st, w := hoverRig(delay)

	r := new(gioinput.Router)
	ops := new(op.Ops)
	t0 := time.Unix(1700000000, 0)
	tShown := t0.Add(delay).Add(time.Millisecond)

	// Bring up the tooltip via the same sequence as (a).
	driveFrameAt(w, ops, r, intFrame, t0)
	r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(intFrameW/2, intFrameH/2), Source: pointer.Mouse})
	driveFrameAt(w, ops, r, intFrame, t0)
	driveFrameAt(w, ops, r, intFrame, tShown)
	if !arb.isTop(st) {
		t.Fatalf("precondition failed: tooltip not shown after entry+delay")
	}

	// Move the pointer outside the trigger. The router emits Leave; the
	// gesture flips to !hovered → active goes false → top is released.
	r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(4, 4), Source: pointer.Mouse})
	driveFrameAt(w, ops, r, intFrame, tShown.Add(time.Millisecond))
	if arb.isTop(st) {
		t.Fatalf("tooltip still holds arbitration top after hover exit")
	}
	if st.claimed || !st.entryAt.IsZero() {
		t.Fatalf("exit left the dwell armed: claimed = %v, entryAt zero = %v; want false, true", st.claimed, st.entryAt.IsZero())
	}
}

// TestSecondTooltipDismissesFirst verifies Measurable (c): once tooltip A
// is shown, another tooltip taking arbitration top hides A, and A does not
// take it straight back. A's show condition is a level ("entry + delay is
// in the past"), so without the claimed latch A would re-claim on its very
// next layout and the two would trade the register every frame. One dwell,
// one show.
func TestSecondTooltipDismissesFirst(t *testing.T) {
	const delay = 50 * time.Millisecond
	arb, st, w := hoverRig(delay)

	r := new(gioinput.Router)
	ops := new(op.Ops)
	t0 := time.Unix(1700000000, 0)
	tShown := t0.Add(delay).Add(time.Millisecond)

	// Bring A up the same way.
	driveFrameAt(w, ops, r, intFrame, t0)
	r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(intFrameW/2, intFrameH/2), Source: pointer.Mouse})
	driveFrameAt(w, ops, r, intFrame, t0)
	driveFrameAt(w, ops, r, intFrame, tShown)
	if !arb.isTop(st) {
		t.Fatalf("precondition failed: tooltip A not shown after entry+delay")
	}

	// A second tooltip in the same set claims. Its dwell is simulated
	// directly: only the claim matters to A, and A's contract is provable
	// without a second live trigger.
	var other tooltipState
	arb.claim(&other)
	if arb.isTop(st) {
		t.Fatalf("A still holds top after another tooltip claimed it")
	}

	// A stays hovered for several more frames and must stay hidden: the
	// dwell it already spent does not buy it a second show.
	now := tShown
	for i := 0; i < 3; i++ {
		now = now.Add(time.Millisecond)
		driveFrameAt(w, ops, r, intFrame, now)
		if arb.isTop(st) {
			t.Fatalf("A took arbitration top back on frame %d while still hovered; the dwell latch did not hold", i+1)
		}
	}

	// Leaving and re-entering rearms it: the tooltip is not hidden forever,
	// only for this dwell.
	r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(4, 4), Source: pointer.Mouse})
	now = now.Add(time.Millisecond)
	driveFrameAt(w, ops, r, intFrame, now)
	r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(intFrameW/2, intFrameH/2), Source: pointer.Mouse})
	now = now.Add(time.Millisecond)
	driveFrameAt(w, ops, r, intFrame, now)
	driveFrameAt(w, ops, r, intFrame, now.Add(delay).Add(time.Millisecond))
	if !arb.isTop(st) {
		t.Fatalf("a fresh hover entry did not rearm the dwell; A never showed again")
	}
}

// TestOvertakenTooltipDoesNotStealBackInTheSameFrame is the tree-order half
// of the latch. When the claimant sits earlier in the layout.Widget tree than the
// incumbent, the incumbent lays out *after* losing top with its hover
// unchanged and its dwell long since elapsed. A claim guarded on "am I visible" would take
// top straight back, inside that same frame and after the claimant had
// already painted — two tooltips on screen, every frame, which is exactly
// the invariant this package exists to hold. Guarded on the dwell instead,
// the incumbent stays down.
func TestOvertakenTooltipDoesNotStealBackInTheSameFrame(t *testing.T) {
	const delay = 50 * time.Millisecond
	arb, st, w := hoverRig(delay)

	r := new(gioinput.Router)
	ops := new(op.Ops)
	t0 := time.Unix(1700000000, 0)
	tShown := t0.Add(delay).Add(time.Millisecond)

	driveFrameAt(w, ops, r, intFrame, t0)
	r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(intFrameW/2, intFrameH/2), Source: pointer.Mouse})
	driveFrameAt(w, ops, r, intFrame, t0)
	driveFrameAt(w, ops, r, intFrame, tShown)
	if !arb.isTop(st) {
		t.Fatalf("precondition failed: tooltip A not shown after entry+delay")
	}

	// Each frame lays the claimant out first, then A.
	var other tooltipState
	now := tShown
	for i := 1; i <= 3; i++ {
		now = now.Add(time.Millisecond)
		frame := func(gtx layout.Context) layout.Dimensions {
			arb.claim(&other)
			return w(gtx)
		}
		driveFrameAt(frame, ops, r, intFrame, now)
		if arb.isTop(st) {
			t.Fatalf("frame %d: A took top back in the same frame the claimant took it; both would paint", i)
		}
	}
}
