package modal

import (
	"image"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/spectrum/tokens"
)

// TestArbiterStacksAndRestores is the property that makes modal's arbiter a
// stack rather than popover's and tooltip's single register: a modal opened
// over another one covers it instead of evicting it, and closing the inner
// one hands the front back to the outer one.
func TestArbiterStacksAndRestores(t *testing.T) {
	var outer, inner modalState
	arb := NewArbiter()

	arb.push(&outer)
	if !arb.isTop(&outer) {
		t.Fatal("the first modal to open is not in front")
	}

	arb.push(&inner)
	if arb.isTop(&outer) || !arb.isTop(&inner) {
		t.Fatalf("after the inner modal opened: isTop(outer) = %v, isTop(inner) = %v; want false, true",
			arb.isTop(&outer), arb.isTop(&inner))
	}
	if len(arb.open) != 2 {
		t.Fatalf("the covered modal left the stack: open = %d, want 2", len(arb.open))
	}

	arb.pop(&inner)
	if !arb.isTop(&outer) {
		t.Fatal("closing the inner modal did not hand the front back to the outer one")
	}

	arb.pop(&outer)
	if arb.isTop(&outer) || len(arb.open) != 0 {
		t.Fatalf("the last pop left open = %v; want empty", arb.open)
	}
}

// TestArbiterPushDoesNotClimb is the level-safety claim in the Arbiter doc: a
// push by a modal already on the stack leaves it exactly where it is. This is
// why modal did not need tooltip's claimed latch — a register's write is
// unconditionally "become top" and so must be guarded by an edge, while a
// stack's push is "join if absent" and is idempotent on its own.
func TestArbiterPushDoesNotClimb(t *testing.T) {
	var outer, inner modalState
	arb := NewArbiter()
	arb.push(&outer)
	arb.push(&inner)

	for i := 0; i < 3; i++ {
		arb.push(&outer)
	}
	if !arb.isTop(&inner) {
		t.Fatal("a repeated push let the covered modal climb over the one in front")
	}
	if len(arb.open) != 2 {
		t.Fatalf("a repeated push duplicated a stack entry: open = %d, want 2", len(arb.open))
	}
}

// TestArbiterPopFromTheMiddle covers the outer modal that closes while an
// inner one is still up — a stack cannot assume the leaver is on top.
func TestArbiterPopFromTheMiddle(t *testing.T) {
	var outer, inner modalState
	arb := NewArbiter()
	arb.push(&outer)
	arb.push(&inner)

	arb.pop(&outer)
	if !arb.isTop(&inner) {
		t.Fatal("the outer modal's departure disturbed which modal is in front")
	}
	if len(arb.open) != 1 {
		t.Fatalf("open = %d after a middle pop, want 1", len(arb.open))
	}

	// Popping a modal that is not on the stack is a no-op, not a corruption.
	arb.pop(&outer)
	if !arb.isTop(&inner) || len(arb.open) != 1 {
		t.Fatalf("a second pop of an absent modal changed the stack: open = %v", arb.open)
	}
}

// TestNilArbiterJoinsTheDefaultSet documents the compatibility default: a
// Props with no Arbiter of its own stacks process-globally, which is what the
// modal stack did before ADR-008 and is correct for one window.
func TestNilArbiterJoinsTheDefaultSet(t *testing.T) {
	own := NewArbiter()
	if st := newState(Props{}); st.arb != &defaultArbiter {
		t.Errorf("nil Arbiter did not join the default set; got %p", st.arb)
	}
	if st := newState(Props{Arbiter: own}); st.arb != own {
		t.Errorf("explicit Arbiter was not used; got %p, want %p", st.arb, own)
	}
}

// TestTrackPushesAndPopsOnTheEdge pins modalState.track, the one place the
// stack is written from: one opening buys one push however many frames it is
// laid out for, and the matching close buys exactly one pop.
func TestTrackPushesAndPopsOnTheEdge(t *testing.T) {
	arb := NewArbiter()
	st := newState(Props{Arbiter: arb})

	for i := 0; i < 3; i++ {
		if live := st.track(true); !live {
			t.Fatalf("frame %d: the only open modal is not live", i)
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

// TestOnlyTheFrontModalTakesPointerInput is isTop's behaviour driven through
// the real router, and it is the contract G0C.2b had to keep exactly: only
// the modal in front registers absorbers and answers a backdrop press, and
// when it leaves the one it was covering answers again.
//
// Pointer rather than keyboard, deliberately: a press needs no focus, so the
// test measures the stack and nothing else.
func TestOnlyTheFrontModalTakesPointerInput(t *testing.T) {
	arb := NewArbiter()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	tok := resolvedTokens{
		color:   tokens.DefaultLight,
		spacing: tokens.Spacing,
		radius:  tokens.RadiusScale{},
		title:   tokens.DefaultTypography.TitleMedium,
	}

	var outerClosed, innerClosed int
	// HideClose keeps the header free of a live prism/button: this test is
	// about the scrim absorber, and a panel's backdrop is the dismissing one.
	outerProps := Props{Arbiter: arb, HideClose: true, OnClose: func(layout.Context) { outerClosed++ }}
	innerProps := Props{Arbiter: arb, HideClose: true, OnClose: func(layout.Context) { innerClosed++ }}
	outerSt, innerSt := newState(outerProps), newState(innerProps)

	// The two lines Modal's own widget closure runs, over an open flag the
	// test owns instead of an rx emission. track is the shared decision.
	mk := func(props Props, st *modalState, open *bool) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			live := st.track(*open)
			if !*open {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			return drawModal(gtx, shaper, props, tok, st, live, nil)
		}
	}
	outerOpen, innerOpen := true, true
	outerW := mk(outerProps, outerSt, &outerOpen)
	innerW := mk(innerProps, innerSt, &innerOpen)

	canvas := image.Pt(320, 240)
	r := new(gioinput.Router)
	ops := new(op.Ops)
	frame := func() {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(canvas),
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
		t.Fatal("the modal laid out last is not the one in front")
	}

	clickCorner()
	if innerClosed != 1 {
		t.Fatalf("the backdrop press did not reach the modal in front; innerClosed = %d, want 1", innerClosed)
	}
	if outerClosed != 0 {
		t.Fatalf("the covered modal answered a press it must not see; outerClosed = %d, want 0", outerClosed)
	}

	// The caller closes the inner modal. Its next laid-out frame pops it,
	// and the outer one is in front again — the stack's whole reason to be
	// an ordered slice rather than a single register.
	innerOpen = false
	frame()
	if !arb.isTop(outerSt) {
		t.Fatal("closing the inner modal did not hand the front back to the outer one")
	}

	// One more frame before the press, and the reason is the ordering cost
	// ADR-008 books for frame ownership, seen from the other side. The outer
	// modal is laid out BEFORE the leaver, so on the frame the pop happens it
	// has already been laid out inert and registered no absorbers; it
	// registers them on the frame after. A press in between reaches nothing
	// rather than the wrong modal, and no pixel is involved — live gates
	// event.Op registration, not drawing.
	frame()

	clickCorner()
	if outerClosed != 1 {
		t.Fatalf("the restored modal did not answer a backdrop press; outerClosed = %d, want 1", outerClosed)
	}
	if innerClosed != 1 {
		t.Fatalf("the closed modal answered a press after leaving; innerClosed = %d, want 1", innerClosed)
	}
}
