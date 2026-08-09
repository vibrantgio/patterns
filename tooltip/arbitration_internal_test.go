package tooltip

import "testing"

// TestArbiterClaimHidesIncumbent pins the core of ADR-008's idiom as
// tooltip takes it: the claim is the event, and because visibility is read
// off the register rather than mirrored beside it, the store is the whole
// of the incumbent's dismissal.
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

// TestNilArbiterJoinsTheDefaultSet documents the compatibility default: a
// Props with no Arbiter of its own arbitrates process-globally, which is
// what tooltip did before ADR-008 and is correct for one window.
func TestNilArbiterJoinsTheDefaultSet(t *testing.T) {
	own := NewArbiter()
	if st := newState(Props{}); st.arb != &defaultArbiter {
		t.Errorf("nil Arbiter did not join the default set; got %p", st.arb)
	}
	if st := newState(Props{Arbiter: own}); st.arb != own {
		t.Errorf("explicit Arbiter was not used; got %p, want %p", st.arb, own)
	}
}
