package popover

import "gioui.org/layout"

// Arbiter is the "only one popover open at a time" register shared by a set
// of popovers. It is a plain value — no mutex, no atomics, no observable —
// because every write and every read happens during layout, on the single
// goroutine Gio runs a frame on. The hazard a synchronised bus guards
// against cannot arise here, so there is nothing for a guard to buy. See
// ADR-008 in the plan.
//
// # The register is the scope
//
// Popovers that share an Arbiter arbitrate with one another and with nobody
// else. A window is one widget tree laid out by one goroutine, so one
// Arbiter per window is both the correct scope for "which popover is open"
// and the only scope a lock-free value is safe at. A Props with no Arbiter
// joins the package-level default set, which is right for a process with one
// window and only for that.
//
// # Arbitration is an event, not a poll
//
// Claiming top dismisses the popover that held it, there and then, from
// inside the claimant's own layout pass. The incumbent is not left to notice
// on some later frame that it lost, so the dismissal does not depend on which
// of the two the widget tree reaches first. What does still depend on tree
// order is one frame of the incumbent's *input* registration: an incumbent
// laid out before the claimant has already registered its absorbers for that
// frame and goes inert on the next one. Nothing is painted from that (live
// gates event.Op registration, not drawing), and the claimant's absorbers are
// registered later and therefore sit on top, so the stale pair costs no
// pixels and swallows no press.
type Arbiter struct {
	// top is the popover currently holding arbitration top, or nil. The
	// state pointer is the identity: it is unique per subscription for as
	// long as anything can reference it, which is exactly the lifetime an id
	// counter used to synthesise.
	top *popoverState
}

// NewArbiter returns an empty arbitration set. Create one per window and
// hand it to every popover in that window's tree.
func NewArbiter() *Arbiter { return new(Arbiter) }

// defaultArbiter is the set a Props with no Arbiter joins. It restores the
// process-global scope popover arbitration had before ADR-008, which is
// indistinguishable from per-window in a single-window process — the shape
// of every application in this organization today.
var defaultArbiter Arbiter

// claim makes st the top popover and dismisses the previous holder in this
// same frame. top is reassigned before the callback runs, so an OnDismiss
// that reaches back into release cannot clear the claim it was told about.
func (a *Arbiter) claim(gtx layout.Context, st *popoverState) {
	prev := a.top
	a.top = st
	if prev != nil && prev != st {
		fire(gtx, prev.dismiss)
	}
}

// release drops st's hold on top. A popover that was already overtaken does
// not hold top and releasing is then a no-op, which is what keeps a closing
// straggler from evicting the popover that replaced it.
func (a *Arbiter) release(st *popoverState) {
	if a.top == st {
		a.top = nil
	}
}

// isTop reports whether st currently holds arbitration top.
func (a *Arbiter) isTop(st *popoverState) bool { return a.top == st }
