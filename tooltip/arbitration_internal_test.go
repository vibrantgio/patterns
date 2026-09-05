package tooltip

import "testing"

// TestArbiterClaimHidesIncumbent verifies that claiming the arbiter is the
// whole of the incumbent's dismissal: visibility is read directly off the
// arbiter rather than mirrored beside it, so the claim itself hides
// whoever held top before it.
func TestArbiterClaimHidesIncumbent(t *testing.T) {
	var a, b tooltipState

	arb := NewArbiter()
	arb.claim(&a)
	if !arb.isTop(&a) {
		t.Fatalf("first claim did not take top")
	}

	arb.claim(&b)
	if arb.isTop(&a) || !arb.isTop(&b) {
		t.Fatalf("after b's claim: isTop(a) = %v, isTop(b) = %v; want false, true", arb.isTop(&a), arb.isTop(&b))
	}

	// Re-claiming by the holder is idempotent.
	arb.claim(&b)
	if !arb.isTop(&b) {
		t.Fatalf("re-claim by the holder lost top")
	}
}

// TestArbiterReleaseOnlyByHolder covers the departing straggler: a tooltip
// that was overtaken and then loses hover must not hide the tooltip that
// replaced it.
func TestArbiterReleaseOnlyByHolder(t *testing.T) {
	var a, b tooltipState
	arb := NewArbiter()
	arb.claim(&a)
	arb.claim(&b)

	arb.release(&a)
	if !arb.isTop(&b) {
		t.Fatal("a's release evicted b, which had already taken top")
	}
	arb.release(&b)
	if arb.isTop(&b) || arb.top != nil {
		t.Fatalf("b's release left top = %v; want nil", arb.top)
	}
}

// TestNilArbiterArbitratesAlone verifies that a Props with no Arbiter gets
// one of its own, so two of them arbitrate with nobody rather than sharing
// state. The scope a lock-free value is safe at is one window, and the only
// way to reach that scope is to say so.
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
