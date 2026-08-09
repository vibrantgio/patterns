package popover

import (
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
)

// frameCtx is the minimum layout.Context the arbiter needs: it never draws,
// it only hands the context to an OnDismiss callback.
func frameCtx() layout.Context { return layout.Context{Ops: new(op.Ops)} }

// TestArbiterClaimDismissesIncumbent pins the core of ADR-008's idiom: the
// claim is the event, and it acts on the incumbent directly.
func TestArbiterClaimDismissesIncumbent(t *testing.T) {
	var a, b popoverState
	var aDismissed, bDismissed int
	a.dismiss = func(layout.Context) { aDismissed++ }
	b.dismiss = func(layout.Context) { bDismissed++ }

	arb := NewArbiter()
	arb.claim(frameCtx(), &a)
	if aDismissed != 0 || !arb.isTop(&a) {
		t.Fatalf("first claim: aDismissed = %d, isTop(a) = %v; want 0, true", aDismissed, arb.isTop(&a))
	}

	arb.claim(frameCtx(), &b)
	if aDismissed != 1 {
		t.Fatalf("b's claim did not dismiss a; aDismissed = %d, want 1", aDismissed)
	}
	if bDismissed != 0 {
		t.Fatalf("b dismissed itself on claiming; bDismissed = %d, want 0", bDismissed)
	}
	if arb.isTop(&a) || !arb.isTop(&b) {
		t.Fatalf("after b's claim: isTop(a) = %v, isTop(b) = %v; want false, true", arb.isTop(&a), arb.isTop(&b))
	}

	// Re-claiming by the holder is idempotent and does not dismiss it.
	arb.claim(frameCtx(), &b)
	if bDismissed != 0 || !arb.isTop(&b) {
		t.Fatalf("re-claim by the holder: bDismissed = %d, isTop(b) = %v; want 0, true", bDismissed, arb.isTop(&b))
	}
}

// TestArbiterReleaseOnlyByHolder covers the closing straggler: a popover
// that was overtaken and then closes must not evict the popover that
// replaced it.
func TestArbiterReleaseOnlyByHolder(t *testing.T) {
	var a, b popoverState
	arb := NewArbiter()
	arb.claim(frameCtx(), &a)
	arb.claim(frameCtx(), &b)

	arb.release(&a)
	if !arb.isTop(&b) {
		t.Fatal("a's release evicted b, which had already taken top")
	}
	arb.release(&b)
	if arb.isTop(&b) || arb.top != nil {
		t.Fatalf("b's release left top = %v; want nil", arb.top)
	}
}

// TestArbiterReleaseFromDismissKeepsNewTop is the re-entrancy case: an
// OnDismiss that immediately releases — the shape of a caller whose close
// path runs synchronously — must not clear the claim it was just told
// about. The arbiter reassigns top before firing, so it does not.
func TestArbiterReleaseFromDismissKeepsNewTop(t *testing.T) {
	var a, b popoverState
	arb := NewArbiter()
	a.dismiss = func(layout.Context) { arb.release(&a) }

	arb.claim(frameCtx(), &a)
	arb.claim(frameCtx(), &b)
	if !arb.isTop(&b) {
		t.Fatalf("a's re-entrant release cleared b's claim; top = %v", arb.top)
	}
}

// TestNilArbiterJoinsTheDefaultSet documents the compatibility default: a
// Props with no Arbiter of its own arbitrates process-globally, which is
// what popover did before ADR-008 and is correct for one window.
func TestNilArbiterJoinsTheDefaultSet(t *testing.T) {
	own := NewArbiter()
	if st := newState(Props{}); st.arb != &defaultArbiter {
		t.Errorf("nil Arbiter did not join the default set; got %p", st.arb)
	}
	if st := newState(Props{Arbiter: own}); st.arb != own {
		t.Errorf("explicit Arbiter was not used; got %p, want %p", st.arb, own)
	}
}
