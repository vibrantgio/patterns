package dialog

import (
	"image"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/theme/tokens"
)

// TestArbiterStacksAndRestores is the property that makes dialog's arbiter a
// stack rather than popover's and tooltip's single arbiter: a dialog opened
// over another one covers it instead of evicting it, and closing the inner
// one hands the front back to the outer one.
func TestArbiterStacksAndRestores(t *testing.T) {
	var outer, inner dialogState
	arb := NewArbiter()

	arb.push(&outer)
	if !arb.isTop(&outer) {
		t.Fatal("the first dialog to open is not in front")
	}

	arb.push(&inner)
	if arb.isTop(&outer) || !arb.isTop(&inner) {
		t.Fatalf("after the inner dialog opened: isTop(outer) = %v, isTop(inner) = %v; want false, true",
			arb.isTop(&outer), arb.isTop(&inner))
	}
	if len(arb.open) != 2 {
		t.Fatalf("the covered dialog left the stack: open = %d, want 2", len(arb.open))
	}

	arb.pop(&inner)
	if !arb.isTop(&outer) {
		t.Fatal("closing the inner dialog did not hand the front back to the outer one")
	}

	arb.pop(&outer)
	if arb.isTop(&outer) || len(arb.open) != 0 {
		t.Fatalf("the last pop left open = %v; want empty", arb.open)
	}
}

// TestArbiterPushDoesNotClimb is the level-safety claim in the Arbiter doc: a
// push by a dialog already on the stack leaves it exactly where it is. This is
// why dialog did not need tooltip's claimed latch — an arbiter's write is
// unconditionally "become top" and so must be guarded by an edge, while a
// stack's push is "join if absent" and is idempotent on its own.
func TestArbiterPushDoesNotClimb(t *testing.T) {
	var outer, inner dialogState
	arb := NewArbiter()
	arb.push(&outer)
	arb.push(&inner)

	for i := 0; i < 3; i++ {
		arb.push(&outer)
	}
	if !arb.isTop(&inner) {
		t.Fatal("a repeated push let the covered dialog climb over the one in front")
	}
	if len(arb.open) != 2 {
		t.Fatalf("a repeated push duplicated a stack entry: open = %d, want 2", len(arb.open))
	}
}

// TestArbiterPopFromTheMiddle covers the outer dialog that closes while an
// inner one is still up — a stack cannot assume the leaver is on top.
func TestArbiterPopFromTheMiddle(t *testing.T) {
	var outer, inner dialogState
	arb := NewArbiter()
	arb.push(&outer)
	arb.push(&inner)

	arb.pop(&outer)
	if !arb.isTop(&inner) {
		t.Fatal("the outer dialog's departure disturbed which dialog is in front")
	}
	if len(arb.open) != 1 {
		t.Fatalf("open = %d after a middle pop, want 1", len(arb.open))
	}

	// Popping a dialog that is not on the stack is a no-op, not a corruption.
	arb.pop(&outer)
	if !arb.isTop(&inner) || len(arb.open) != 1 {
		t.Fatalf("a second pop of an absent dialog changed the stack: open = %v", arb.open)
	}
}

// TestNilArbiterArbitratesAlone pins the no-Arbiter default: a Props with no
// Arbiter gets one of its own, so two of them stack with nobody rather than
// sharing process-global state. The scope a lock-free value is safe at is
// one window, and the only way to reach that scope is to say so.
func TestNilArbiterArbitratesAlone(t *testing.T) {
	first, second := newState(Props{}), newState(Props{})
	if first.arb == nil || second.arb == nil {
		t.Fatal("a nil Props.Arbiter left the state with no arbiter at all")
	}
	if first.arb == second.arb {
		t.Errorf("two nil-Arbiter states share an arbiter (%p); each must get its own", first.arb)
	}

	own := NewArbiter()
	if st := newState(Props{Arbiter: own}); st.arb != own {
		t.Errorf("explicit Arbiter was not used; got %p, want %p", st.arb, own)
	}
}

// TestTrackPushesAndPopsOnTheEdge pins dialogState.track, the one place the
// stack is written from: one opening buys one push however many frames it is
// laid out for, and the matching close buys exactly one pop.
func TestTrackPushesAndPopsOnTheEdge(t *testing.T) {
	arb := NewArbiter()
	st := newState(Props{Arbiter: arb})

	for i := 0; i < 3; i++ {
		if live := st.track(true); !live {
			t.Fatalf("frame %d: the only open dialog is not live", i)
		}
	}
	if len(arb.open) != 1 {
		t.Fatalf("three open frames pushed %d times, want 1", len(arb.open))
	}

	for i := 0; i < 3; i++ {
		if live := st.track(false); live {
			t.Fatalf("closed frame %d reported live", i)
		}
	}
	if len(arb.open) != 0 {
		t.Fatalf("the close left %d on the stack, want 0", len(arb.open))
	}

	// Re-opening rejoins the stack rather than being swallowed by a stale
	// latch, and asks for initial focus again.
	st.wantInitialFocus = false
	if live := st.track(true); !live || len(arb.open) != 1 {
		t.Fatalf("re-open: live = %v, open = %d; want true, 1", live, len(arb.open))
	}
	if !st.wantInitialFocus {
		t.Error("re-opening did not request initial focus")
	}
}

// TestOnlyTheFrontDialogTakesPointerInput is isTop's behaviour driven through
// the real router: only the dialog in front registers its absorbers and answers a
// backdrop press, and when it leaves the one it was covering answers again.
//
// Pointer rather than keyboard, deliberately: a press needs no focus, so the
// test measures the stack and nothing else.
func TestOnlyTheFrontDialogTakesPointerInput(t *testing.T) {
	arb := NewArbiter()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	tok := resolvedTokens{
		color:   tokens.PlatformLight,
		spacing: tokens.Spacing,
		radius:  tokens.RadiusScale{},
		title:   tokens.DefaultTypography.Title2,
	}

	var outerClosed, innerClosed int
	// HideClose keeps the header free of a live components/button: this test is
	// about the scrim absorber, and a dismissible dialog's backdrop is the dismissing one.
	outerProps := Props{Arbiter: arb, HideClose: true, OnClose: func(layout.Context) { outerClosed++ }}
	innerProps := Props{Arbiter: arb, HideClose: true, OnClose: func(layout.Context) { innerClosed++ }}
	outerSt, innerSt := newState(outerProps), newState(innerProps)

	// The two lines Dialog's own layout.Widget closure runs, over an open flag the
	// test owns instead of an rx emission. track is the shared decision.
	mk := func(props Props, st *dialogState, open *bool) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			live := st.track(*open)
			if !*open {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			return drawDialog(gtx, shaper, props, tok, st, live, nil)
		}
	}
	outerOpen, innerOpen := true, true
	outerW := mk(outerProps, outerSt, &outerOpen)
	innerW := mk(innerProps, innerSt, &innerOpen)

	frameSize := image.Pt(320, 240)
	r := new(gioinput.Router)
	ops := new(op.Ops)
	frame := func() {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(frameSize),
			Ops:         ops,
			Source:      r.Source(),
		}
		outerW(gtx)
		innerW(gtx)
		r.Frame(ops)
	}
	// A corner press is guaranteed scrim and never surface.
	corner := f32.Pt(4, 4)
	clickCorner := func() {
		r.Queue(
			pointer.Event{Kind: pointer.Press, Position: corner, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
			pointer.Event{Kind: pointer.Release, Position: corner, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		)
		frame()
	}

	frame() // both join the stack; the inner one is laid out last and is in front
	if !arb.isTop(innerSt) {
		t.Fatal("the dialog laid out last is not the one in front")
	}

	clickCorner()
	if innerClosed != 1 {
		t.Fatalf("the backdrop press did not reach the dialog in front; innerClosed = %d, want 1", innerClosed)
	}
	if outerClosed != 0 {
		t.Fatalf("the covered dialog answered a press it must not see; outerClosed = %d, want 0", outerClosed)
	}

	// The caller closes the inner dialog. Its next laid-out frame pops it,
	// and the outer one is in front again — the stack's whole reason to be
	// an ordered slice rather than a single arbiter.
	innerOpen = false
	frame()
	if !arb.isTop(outerSt) {
		t.Fatal("closing the inner dialog did not hand the front back to the outer one")
	}

	// One more frame before the press: the outer
	// dialog is laid out BEFORE the leaver, so on the frame the pop happens it
	// has already been laid out inert and registered no absorbers; it
	// registers its absorbers on the frame after. A press in between reaches nothing
	// rather than the wrong dialog, and no pixel is involved — live gates
	// event.Op registration, not drawing.
	frame()

	clickCorner()
	if outerClosed != 1 {
		t.Fatalf("the restored dialog did not answer a backdrop press; outerClosed = %d, want 1", outerClosed)
	}
	if innerClosed != 1 {
		t.Fatalf("the closed dialog answered a press after leaving; innerClosed = %d, want 1", innerClosed)
	}
}

// TestACoveredDialogDrawnLastTakesNoPointerInput is the pointer half of
// "painted but inert", measured where z-order cannot answer for the stack.
// In TestOnlyTheFrontDialogTakesPointerInput the dialog in front is also the
// one laid out last, so its absorber sits on top of whatever a covered dialog
// might wrongly register a tag — a regression that made covered dialogs live for
// pointer input would pass that test on z-order alone. Here the covered
// dialog is laid out LAST: it opened first from later in the tree, and the
// front one opened a frame later from earlier in the tree, so push order and
// paint order disagree. If the covered dialog registered a scrim absorber at
// all, Gio's hit test would hand it the press; only live gating event.Op
// registration keeps it silent.
func TestACoveredDialogDrawnLastTakesNoPointerInput(t *testing.T) {
	arb := NewArbiter()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	tok := resolvedTokens{
		color:   tokens.PlatformLight,
		spacing: tokens.Spacing,
		radius:  tokens.RadiusScale{},
		title:   tokens.DefaultTypography.Title2,
	}

	var frontClosed, coveredClosed int
	frontProps := Props{Arbiter: arb, HideClose: true, OnClose: func(layout.Context) { frontClosed++ }}
	coveredProps := Props{Arbiter: arb, HideClose: true, OnClose: func(layout.Context) { coveredClosed++ }}
	frontSt, coveredSt := newState(frontProps), newState(coveredProps)

	mk := func(props Props, st *dialogState, open *bool) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			live := st.track(*open)
			if !*open {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			return drawDialog(gtx, shaper, props, tok, st, live, nil)
		}
	}
	frontOpen, coveredOpen := false, true
	frontW := mk(frontProps, frontSt, &frontOpen)         // earlier in the tree
	coveredW := mk(coveredProps, coveredSt, &coveredOpen) // later in the tree

	frameSize := image.Pt(320, 240)
	r := new(gioinput.Router)
	ops := new(op.Ops)
	frame := func() {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(frameSize),
			Ops:         ops,
			Source:      r.Source(),
		}
		frontW(gtx)
		coveredW(gtx)
		r.Frame(ops)
	}
	corner := f32.Pt(4, 4)
	clickCorner := func() {
		r.Queue(
			pointer.Event{Kind: pointer.Press, Position: corner, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
			pointer.Event{Kind: pointer.Release, Position: corner, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		)
		frame()
	}

	frame() // only the later-in-tree dialog is open; it is alone and in front
	frontOpen = true
	frame() // the front dialog opens: pushed later, so in front, yet drawn first
	if !arb.isTop(frontSt) {
		t.Fatal("the dialog pushed last is not the one in front")
	}

	clickCorner()
	if coveredClosed != 0 {
		t.Fatalf("the covered dialog answered a press because it was drawn last; coveredClosed = %d, want 0", coveredClosed)
	}
	if frontClosed != 1 {
		t.Fatalf("the backdrop press did not reach the dialog in front; frontClosed = %d, want 1", frontClosed)
	}
}
