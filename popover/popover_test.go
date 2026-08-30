package popover_test

import (
	"context"
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/font"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/popover"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

const (
	canvasW, canvasH = 320, 240
)

var (
	canvasSize = image.Pt(canvasW, canvasH)
	// Sharp corner radius. Anti-aliased rounded corners vary slightly
	// between GPU contexts, breaking determinism.
	sharpRadius = tokens.RadiusScale{}
)

// fixedRect is a sharp-edged solid widget with explicit width and height.
// Used for both Anchor and Content stand-ins so their hit rects are
// predictable and the goldens stay deterministic.
func fixedRect(c color.NRGBA, widthDp, heightDp float32) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(unit.Dp(widthDp)), gtx.Dp(unit.Dp(heightDp)))
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	}
}

// defaultShaper returns the shaper every golden here draws with: the default
// typography's faces pinned, system fonts off, so the stored images are the
// same on every machine. A golden test pins its faces with
// DeterministicShaper; application code takes the fallback Shaper.
func defaultShaper(t *testing.T) *text.Shaper {
	t.Helper()
	return tokens.DefaultTypography.DeterministicShaper()
}

// textContent returns the popover's content: a short line of real text in the
// BodyMedium role.
//
// Popover, like card, carries no Shaper in its Props because it draws no text
// of its own — Anchor and Content are both caller-supplied widgets, so the
// typeface inside a popover is settled by whoever builds them. This is that
// caller. ASCII only — no symbol reaches a stored image.
//
// The string length is load-bearing, and deliberately near the limit: the
// surface grows to fit it, and Left placement puts that surface between the
// centred anchor and the canvas edge. "Sort ascending" leaves 3 px of
// clearance there. A longer line would run off the left of left-dark, where
// the canvas cannot grow — the interaction tests below address the anchor by
// hardcoded coordinates in this 320×240 frame.
//
// It draws through theme/typeset, like every other text site in this
// organization: a role's LineHeight is the CSS line box, and handing it to
// gioui.org/widget.Label does not produce that box. A MaxLines:1 label is
// exactly the case widget.Label measures identically at every line height,
// so the content is BodyMedium and stands in its declared 20 dp box rather
// than its 17 px of ink, and the surface grows with it.
func textContent(t *testing.T, fg color.NRGBA) layout.Widget {
	t.Helper()
	shaper := defaultShaper(t)
	style := tokens.DefaultTypography.BodyMedium
	return func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		paint.ColorOp{Color: fg}.Add(gtx.Ops)
		material := m.Stop()

		f := typeset.Font(style, font.Normal)
		lbl := typeset.Label(style, 1)
		// Min is dropped so the content reports its own size rather than the
		// popover's; typeset.Layout re-applies whatever constraints remain.
		gtx.Constraints.Min = image.Point{}
		return typeset.Layout(gtx, shaper, lbl, f, unit.Sp(style.Size), "Sort ascending", material)
	}
}

// scene renders w over a flat background sized to the constraints.
func scene(w layout.Widget, bg color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// ---- Golden tests ----

// TestPopoverGolden records or diffs the four Measurable goldens — one
// per Placement, alternating light and dark theme. The anchor is a small
// solid rectangle and the content is a line of real text; the tail
// triangle is the only diagonal-edged shape in each frame.
func TestPopoverGolden(t *testing.T) {
	anchor := fixedRect(color.NRGBA{R: 80, G: 160, B: 220, A: 255}, 60, 28)

	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	darkBG := color.NRGBA{R: 20, G: 20, B: 20, A: 255}

	cases := []struct {
		name      string
		placement popover.Placement
		colors    tokens.ColorTokens
		bg        color.NRGBA
	}{
		{"top-light", popover.Top, tokens.DefaultLight, lightBG},
		{"bottom-light", popover.Bottom, tokens.DefaultLight, lightBG},
		{"left-dark", popover.Left, tokens.DefaultDark, darkBG},
		{"right-dark", popover.Right, tokens.DefaultDark, darkBG},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			props := popover.Props{
				Anchor:    anchor,
				Content:   textContent(t, tc.colors.Text),
				Placement: tc.placement,
			}
			w := popover.Render(props, true, tc.colors, tokens.Spacing, sharpRadius)
			golden.Render(t, tc.name, canvasSize, scene(w, tc.bg))
		})
	}
}

// TestPopoverOpenAndClosedDiffer confirms that flipping the open flag
// changes the rendered output. Catches regressions where the open branch
// silently no-ops.
func TestPopoverOpenAndClosedDiffer(t *testing.T) {
	anchor := fixedRect(color.NRGBA{R: 80, G: 160, B: 220, A: 255}, 60, 28)
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	props := popover.Props{Anchor: anchor, Content: textContent(t, tokens.DefaultLight.Text), Placement: popover.Top}

	open := popover.Render(props, true, tokens.DefaultLight, tokens.Spacing, sharpRadius)
	closed := popover.Render(props, false, tokens.DefaultLight, tokens.Spacing, sharpRadius)

	imgOpen := golden.Capture(t, canvasSize, scene(open, bg))
	imgClosed := golden.Capture(t, canvasSize, scene(closed, bg))
	if n := golden.PixelDiff(imgOpen, imgClosed); n == 0 {
		t.Error("open and closed popover render identically; expected the surface + tail to appear when open")
	}
}

// ---- Interaction tests ----

// livePopover subscribes to the Popover observable, drains the trampoline
// scheduler with Wait(), and returns the latest emitted layout.Widget.
// State referenced by the widget closure remains valid for the test's
// lifetime because it is captured by the rx.Defer scope.
func livePopover(t *testing.T, props popover.Props) layout.Widget {
	t.Helper()
	obs := popover.Popover(rx.Of(theme.Default()), props)
	var w layout.Widget
	if err := obs.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("Popover subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("Popover did not emit an initial widget")
	}
	return w
}

// driveFrame lays out w against ops + router, returns the rendered dims.
// ops is reset before layout; events queued on the router before the call
// are delivered during w's layout pass and r.Frame.
func driveFrame(w layout.Widget, ops *op.Ops, r *gioinput.Router, size image.Point) layout.Dimensions {
	ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(size),
		Ops:         ops,
		Source:      r.Source(),
	}
	dims := w(gtx)
	r.Frame(ops)
	return dims
}

// TestOutsideClickInvokesOnDismiss verifies the Measurable interaction —
// a pointer.Press outside the popover's anchor and surface bounds invokes
// OnDismiss. A press inside the surface (canvas centre, where the anchor
// lives, then near the anchor) must NOT invoke OnDismiss.
func TestOutsideClickInvokesOnDismiss(t *testing.T) {
	var dismissed int
	anchor := fixedRect(color.NRGBA{R: 80, G: 160, B: 220, A: 255}, 60, 28)
	content := fixedRect(color.NRGBA{R: 120, G: 120, B: 120, A: 255}, 80, 36)

	w := livePopover(t, popover.Props{
		Open:      rx.Of(true),
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Top,
		Arbiter:   popover.NewArbiter(),
		OnDismiss: func(_ layout.Context) { dismissed++ },
	})

	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveFrame(w, ops, r, canvasSize)
	driveFrame(w, ops, r, canvasSize)

	// (1) Press at the canvas corner — guaranteed outside both the anchor
	// (centred ~30 dp around canvas centre) and the surface (above it for
	// Placement=Top). OnDismiss must fire.
	corner := f32.Pt(4, 4)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: corner, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: corner, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
	driveFrame(w, ops, r, canvasSize)
	if dismissed == 0 {
		t.Fatalf("outside click did not invoke OnDismiss; dismissed = %d", dismissed)
	}
	outsideHits := dismissed

	// (2) Press at the canvas centre — guaranteed inside the anchor — must
	// not bleed through to the outside-absorber and dismiss.
	centre := f32.Pt(canvasW/2, canvasH/2)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: centre, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: centre, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
	driveFrame(w, ops, r, canvasSize)
	if dismissed != outsideHits {
		t.Errorf("anchor click bled through to outside-absorber; OnDismiss went from %d to %d", outsideHits, dismissed)
	}
}

// TestOutsideClickDismissesWithChipSizedCanvas replicates the popover-
// canvas coupling (mindchat's model picker): the caller hands the popover
// an Exact anchor-sized box, so the anchor covers the whole canvas and an
// outside press can only land beyond it. OnDismiss must still fire for a
// press elsewhere in the window, and an anchor press must still be
// absorbed silently.
func TestOutsideClickDismissesWithChipSizedCanvas(t *testing.T) {
	var dismissed int
	chip := image.Pt(60, 28)
	anchor := fixedRect(color.NRGBA{R: 80, G: 160, B: 220, A: 255}, 60, 28)
	content := fixedRect(color.NRGBA{R: 120, G: 120, B: 120, A: 255}, 80, 36)

	w := livePopover(t, popover.Props{
		Open:      rx.Of(true),
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Bottom,
		Arbiter:   popover.NewArbiter(),
		OnDismiss: func(_ layout.Context) { dismissed++ },
	})

	// The coupling: the popover is offset into a corner of the frame and
	// constrained to exactly the chip's box.
	chipPos := image.Pt(200, 8)
	coupled := func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		defer op.Offset(chipPos).Push(gtx.Ops).Pop()
		cg := gtx
		cg.Constraints = layout.Exact(chip)
		w(cg)
		return layout.Dimensions{Size: size}
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveFrame(coupled, ops, r, canvasSize)
	driveFrame(coupled, ops, r, canvasSize)

	// (1) Press far from the chip and from the surface hanging below it —
	// outside the chip-sized canvas entirely. OnDismiss must fire.
	far := f32.Pt(8, float32(canvasH-8))
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: far, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: far, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
	driveFrame(coupled, ops, r, canvasSize)
	if dismissed == 0 {
		t.Fatalf("press outside the chip-sized canvas did not invoke OnDismiss; dismissed = %d", dismissed)
	}
	outsideHits := dismissed

	// (2) Press on the chip itself — the anchor absorber must win.
	centre := f32.Pt(float32(chipPos.X+chip.X/2), float32(chipPos.Y+chip.Y/2))
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: centre, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: centre, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
	driveFrame(coupled, ops, r, canvasSize)
	if dismissed != outsideHits {
		t.Errorf("anchor press bled through to the outside-absorber; OnDismiss went from %d to %d", outsideHits, dismissed)
	}
}

// TestArbitrationDismissesPriorPopover verifies that opening a second
// popover dismisses the first, because arbitration is frame state.
//
// The claim is a layout-time event — a popover takes top on the first
// frame it is drawn open — so entering B into the tree is what "opening B"
// means here. Both widgets are laid out against one gtx, the way mvu lays
// out its layers, and the assertions are that:
//
//   - A is dismissed in the same frame B claims, in BOTH tree orders. The
//     claimant dismisses the incumbent from inside its own layout pass, so
//     the incumbent does not have to be reached later in the tree, or on a
//     later frame, to find out that it lost.
//   - the dismissal fires exactly once. It is an event, not a per-frame poll
//     of "am I still top".
func TestArbitrationDismissesPriorPopover(t *testing.T) {
	anchor := fixedRect(color.NRGBA{R: 80, G: 160, B: 220, A: 255}, 60, 28)
	content := fixedRect(color.NRGBA{R: 120, G: 120, B: 120, A: 255}, 80, 36)

	for _, tc := range []struct {
		name          string
		claimantFirst bool
	}{
		{"incumbent laid out first", false},
		{"claimant laid out first", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var aDismissed, bDismissed int
			// One arbitration set for this test: the scope of arbitration is
			// the scope of the value, so the test cannot disturb, or be
			// disturbed by, any other popover in the process.
			arb := popover.NewArbiter()

			aWidget := livePopover(t, popover.Props{
				Open:      rx.Of(true),
				Anchor:    anchor,
				Content:   content,
				Placement: popover.Top,
				Arbiter:   arb,
				OnDismiss: func(_ layout.Context) { aDismissed++ },
			})
			bWidget := livePopover(t, popover.Props{
				Open:      rx.Of(true),
				Anchor:    anchor,
				Content:   content,
				Placement: popover.Bottom,
				Arbiter:   arb,
				OnDismiss: func(_ layout.Context) { bDismissed++ },
			})

			r := new(gioinput.Router)
			ops := new(op.Ops)

			// Frame 1: only A is in the tree, so only A has claimed top.
			driveFrame(aWidget, ops, r, canvasSize)
			if aDismissed != 0 {
				t.Fatalf("A dismissed before B entered the tree; aDismissed = %d", aDismissed)
			}

			// Frame 2: B enters the tree and claims.
			frame := func(gtx layout.Context) layout.Dimensions {
				if tc.claimantFirst {
					bWidget(gtx)
					aWidget(gtx)
				} else {
					aWidget(gtx)
					bWidget(gtx)
				}
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			driveFrame(frame, ops, r, canvasSize)
			if aDismissed != 1 {
				t.Fatalf("B's claim did not dismiss A in the same frame; aDismissed = %d, want 1", aDismissed)
			}
			if bDismissed != 0 {
				t.Fatalf("the claimant dismissed itself; bDismissed = %d, want 0", bDismissed)
			}

			// Frame 3: A has not yet been closed by its caller, so it is
			// still drawn and still not top — and must not be told again.
			driveFrame(frame, ops, r, canvasSize)
			if aDismissed != 1 {
				t.Fatalf("dismissal re-fired on a later frame; aDismissed = %d, want 1 (it is an event, not a poll)", aDismissed)
			}
		})
	}
}

// TestOpenNowIsReadEveryFrame pins the OpenNow spelling of open-ness: the
// caller owns a plain bool, the widget reads it during layout, and no
// emission stands between the flip and the frame that shows it. It is
// the SAME widget value on all four frames — the one the stream emitted for
// the theme — which is the whole point: with Props.Open the flag can only
// change by re-emitting, and the emission arrives on another goroutine, a
// frame later, into an atomic cell the caller has to keep beside it.
func TestOpenNowIsReadEveryFrame(t *testing.T) {
	anchor := fixedRect(color.NRGBA{R: 80, G: 160, B: 220, A: 255}, 60, 28)

	var open bool
	var contentDraws int
	content := func(gtx layout.Context) layout.Dimensions {
		contentDraws++
		return fixedRect(color.NRGBA{R: 120, G: 120, B: 120, A: 255}, 80, 36)(gtx)
	}

	w := livePopover(t, popover.Props{
		OpenNow:   func() bool { return open },
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Top,
		Arbiter:   popover.NewArbiter(),
	})

	r := new(gioinput.Router)
	ops := new(op.Ops)

	driveFrame(w, ops, r, canvasSize)
	if contentDraws != 0 {
		t.Fatalf("closed popover laid out its content; contentDraws = %d, want 0", contentDraws)
	}

	open = true
	driveFrame(w, ops, r, canvasSize)
	if contentDraws != 1 {
		t.Fatalf("OpenNow flipped true but the same widget did not open; contentDraws = %d, want 1", contentDraws)
	}
	driveFrame(w, ops, r, canvasSize)
	if contentDraws != 2 {
		t.Fatalf("popover did not stay open; contentDraws = %d, want 2", contentDraws)
	}

	open = false
	driveFrame(w, ops, r, canvasSize)
	if contentDraws != 2 {
		t.Fatalf("OpenNow flipped false but the popover stayed open; contentDraws = %d, want 2", contentDraws)
	}
}

// TestOpenNowArbitratesOnTheEdge pins the rule that the claim must be an
// edge, not a level. OpenNow is read every frame and
// stays true for as long as the caller's flag does, so a popover that has
// been overtaken must not take top back on its next layout. st.holds is the
// latch that makes it an edge, and it does not care where the flag came from.
func TestOpenNowArbitratesOnTheEdge(t *testing.T) {
	anchor := fixedRect(color.NRGBA{R: 80, G: 160, B: 220, A: 255}, 60, 28)
	content := fixedRect(color.NRGBA{R: 120, G: 120, B: 120, A: 255}, 80, 36)
	arb := popover.NewArbiter()

	var aOpen, bOpen bool
	var aDismissed, bDismissed int
	aWidget := livePopover(t, popover.Props{
		OpenNow:   func() bool { return aOpen },
		Anchor:    anchor,
		Content:   content,
		Arbiter:   arb,
		OnDismiss: func(layout.Context) { aDismissed++; aOpen = false },
	})
	bWidget := livePopover(t, popover.Props{
		OpenNow:   func() bool { return bOpen },
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Bottom,
		Arbiter:   arb,
		OnDismiss: func(layout.Context) { bDismissed++; bOpen = false },
	})
	frame := func(gtx layout.Context) layout.Dimensions {
		aWidget(gtx)
		bWidget(gtx)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)

	aOpen = true
	driveFrame(frame, ops, r, canvasSize)
	if aDismissed != 0 {
		t.Fatalf("A dismissed with nobody to dismiss it; aDismissed = %d", aDismissed)
	}

	// B opens. A is dismissed inside B's layout pass, which closes A's own
	// flag — the caller-owned bool is the only copy of the state.
	bOpen = true
	driveFrame(frame, ops, r, canvasSize)
	if aDismissed != 1 || bDismissed != 0 {
		t.Fatalf("B's claim: aDismissed = %d, bDismissed = %d; want 1, 0", aDismissed, bDismissed)
	}

	// Several quiet frames: B's flag is still true, so a level-guarded claim
	// would re-take top every frame and the two would trade it forever.
	for i := 0; i < 3; i++ {
		driveFrame(frame, ops, r, canvasSize)
	}
	if aDismissed != 1 || bDismissed != 0 {
		t.Fatalf("a level-guarded claim re-fired: aDismissed = %d, bDismissed = %d; want 1, 0", aDismissed, bDismissed)
	}
}

// TestOpenNowWinsOverOpen documents the precedence, because a Props that sets
// both is a caller who has not decided which destination the state belongs
// to and should get the frame-owned answer, not a silent mixture.
func TestOpenNowWinsOverOpen(t *testing.T) {
	var contentDraws int
	w := livePopover(t, popover.Props{
		Open:    rx.Of(true),
		OpenNow: func() bool { return false },
		Anchor:  fixedRect(color.NRGBA{R: 80, G: 160, B: 220, A: 255}, 60, 28),
		Content: func(gtx layout.Context) layout.Dimensions {
			contentDraws++
			return layout.Dimensions{}
		},
		Arbiter: popover.NewArbiter(),
	})
	driveFrame(w, new(op.Ops), new(gioinput.Router), canvasSize)
	if contentDraws != 0 {
		t.Fatalf("Open won over OpenNow; contentDraws = %d, want 0", contentDraws)
	}
}

// ---- Placement tests ----

// roomW is the width of the sub-canvas the placement tests hand the popover
// inside the wider scene. The scene is wider so an unclamped surface has
// somewhere to spill to and the capture records it.
const roomW = 200

// inRoom hands w a canvas roomW wide at the scene's leading edge, full
// height. Nothing clips it: a surface that ran off the canvas would still be
// drawn, which is what makes the clamp assertions mean anything.
func inRoom(w layout.Widget, width int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(image.Pt(width, canvasH))
		return w(gtx)
	}
}

// fillRun reports the leftmost and rightmost x on row y painted in c.
func fillRun(img *image.RGBA, y int, c color.NRGBA) (lo, hi int, ok bool) {
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		r, g, bl, _ := img.At(x, y).RGBA()
		if uint8(r>>8) != c.R || uint8(g>>8) != c.G || uint8(bl>>8) != c.B {
			continue
		}
		if !ok {
			lo, ok = x, true
		}
		hi = x
	}
	return lo, hi, ok
}

// inkRun reports the leftmost and rightmost x on row y that is not the
// scene's background, so an anti-aliased tip counts as ink.
func inkRun(img *image.RGBA, y int, bg color.NRGBA) (lo, hi int, ok bool) {
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		r, g, bl, _ := img.At(x, y).RGBA()
		if uint8(r>>8) == bg.R && uint8(g>>8) == bg.G && uint8(bl>>8) == bg.B {
			continue
		}
		if !ok {
			lo, ok = x, true
		}
		hi = x
	}
	return lo, hi, ok
}

// placementScene renders one open popover in a roomW-wide canvas inside the
// standard scene and returns the capture plus the surface fill to look for.
func placementScene(t *testing.T, align popover.Alignment, contentW float32, room int) (*image.RGBA, color.NRGBA) {
	t.Helper()
	colors := tokens.DefaultLight
	props := popover.Props{
		Anchor:    fixedRect(color.NRGBA{R: 80, G: 160, B: 220, A: 255}, 60, 28),
		Content:   fixedRect(color.NRGBA{R: 120, G: 120, B: 120, A: 255}, contentW, 36),
		Placement: popover.Bottom,
		Align:     align,
	}
	w := popover.Render(props, true, colors, tokens.Spacing, sharpRadius)
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	return golden.Capture(t, canvasSize, scene(inRoom(w, room), bg)), colors.SurfaceAt(tokens.Level3)
}

// TestSurfaceIsNudgedBackInsideTheCanvas is the reflow contract: a surface
// centred on an anchor standing at the canvas's trailing edge would run off
// that edge, and the popover moves it back rather than letting it clip.
//
// The anchor is 60 wide against a 200-wide canvas, so trailing-aligned it
// spans [140, 200] with its midline at 170; the surface is 160 + 2*S3 = 184
// wide, which centred on 170 would run to 262. Clamped it ends on 200.
func TestSurfaceIsNudgedBackInsideTheCanvas(t *testing.T) {
	img, fill := placementScene(t, popover.AlignTrailing, 160, roomW)
	// A row between the surface's top edge and its content: the anchor's
	// foot is at 134, the gap is S2, so the surface starts at 142.
	lo, hi, ok := fillRun(img, 148, fill)
	if !ok {
		t.Fatal("no surface pixels on the row under the surface's top edge")
	}
	if hi >= roomW {
		t.Errorf("surface runs to x=%d, past the %d-wide canvas it was given", hi, roomW)
	}
	if hi < roomW-3 {
		t.Errorf("surface ends at x=%d; a clamped surface stands on the canvas edge at %d", hi, roomW-1)
	}
	if lo < 0 {
		t.Errorf("surface starts at x=%d, off the canvas's leading edge", lo)
	}
}

// TestSurfaceWiderThanItsCanvasIsLeftAlone documents the escape hatch: a
// caller that cut its canvas to the anchor has said nothing about the room
// it has, and shoving an over-wide surface against one edge would only move
// the overflow to the other.
func TestSurfaceWiderThanItsCanvasIsLeftAlone(t *testing.T) {
	const room = 60
	img, fill := placementScene(t, popover.AlignTrailing, 160, room)
	lo, hi, ok := fillRun(img, 148, fill)
	if !ok {
		t.Fatal("no surface pixels on the row under the surface's top edge")
	}
	if lo >= 0 && hi < room {
		t.Errorf("surface at [%d,%d] was squeezed into a %d-wide canvas it cannot fit", lo, hi, room)
	}
}

// TestTailPointsAtTheDrawnAnchor is the other half of the seam: the anchor
// reports the shape it drew and the popover stands it at the canvas's
// trailing edge, so the tail aims at the drawn control's midline (170 for a
// 60-wide anchor against a 200-wide canvas) rather than at the canvas's own
// (100), even after the surface beneath it has been nudged.
func TestTailPointsAtTheDrawnAnchor(t *testing.T) {
	img, fill := placementScene(t, popover.AlignTrailing, 160, roomW)
	// Mid-tail: the anchor's foot is at 134 and the tail bridges the S2 gap
	// to the surface at 142.
	lo, hi, ok := fillRun(img, 138, fill)
	if !ok {
		t.Fatal("no tail pixels between the anchor's foot and the surface")
	}
	const drawnMid = roomW - 30
	if mid := (lo + hi) / 2; mid < drawnMid-1 || mid > drawnMid+1 {
		t.Errorf("tail centred on x=%d (run [%d,%d]); the drawn anchor's midline is %d", mid, lo, hi, drawnMid)
	}
}

// TestTailMeetsTheAnchorAndTheSurface is the seam at both ends: the tip
// stands on the anchor's foot rather than floating short of it, and the
// surface's outline is interrupted at the base rather than drawn across it.
func TestTailMeetsTheAnchorAndTheSurface(t *testing.T) {
	img, fill := placementScene(t, popover.AlignTrailing, 160, roomW)
	const (
		foot     = 134 // the anchor is 28 tall centred in a 240 canvas
		edge     = 142 // and the surface stands S2 below it
		drawnMid = roomW - 30
	)
	lo, hi, ok := inkRun(img, foot, color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	if !ok {
		t.Fatalf("the row at the anchor's foot (y=%d) is bare; the tail floats above the anchor", foot)
	}
	if mid := (lo + hi) / 2; mid < drawnMid-2 || mid > drawnMid+2 {
		t.Errorf("the ink at the anchor's foot runs [%d,%d]; the tail's tip belongs on %d", lo, hi, drawnMid)
	}
	// On the surface's own top edge the outline gives way to the tail's
	// fill across the tail's width — the interruption is what makes the two
	// outlines one contour instead of a border through a triangle.
	lo, hi, ok = fillRun(img, edge, fill)
	if !ok {
		t.Fatalf("the surface's top edge (y=%d) is unbroken outline; the tail's base is drawn through", edge)
	}
	if lo > drawnMid-4 || hi < drawnMid+4 {
		t.Errorf("the outline gives way only over [%d,%d]; the tail's base is %d wide about %d", lo, hi, 12, drawnMid)
	}
}
