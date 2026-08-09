package tooltip

// Arbiter is the "only one tooltip visible at a time" register shared by a
// set of tooltips. It is a plain value — no mutex, no atomics, no
// observable — because every write and every read happens during layout, on
// the single goroutine Gio runs a frame on. The hazard a synchronised bus
// guards against cannot arise here, so there is nothing for a guard to buy.
// See ADR-008 in the plan.
//
// # The register is the scope
//
// Tooltips that share an Arbiter arbitrate with one another and with nobody
// else. A window is one widget tree laid out by one goroutine, so one
// Arbiter per window is both the correct scope for "which tooltip is up"
// and the only scope a lock-free value is safe at. A Props with no Arbiter
// joins the package-level default set, which is right for a process with
// one window and only for that.
//
// # The register is also the visibility
//
// Popover keeps a hold flag of its own beside the register and has the
// arbiter call OnDismiss when the two must be brought back into step. A
// tooltip has no caller state and no dismissal callback — it is visible
// exactly while it holds top — so the store that makes a claimant top is
// the whole of the incumbent's dismissal. There is nothing to notify and no
// second copy to keep in step, which is why claim takes no layout.Context
// and is one assignment.
//
// The incumbent stops painting on the first frame it is laid out after
// losing top: in the same frame when the claimant comes earlier in the
// widget tree, on the next frame when it does not, because a widget already
// laid out cannot un-paint. The Subject-era arbitration cost that frame too
// — it cost it in both tree orders — so nothing here is slower and one
// ordering is a frame quicker.
//
// # Claim on the edge, never on the level
//
// A tooltip's show condition is a dwell timer, and "entry + delay is in the
// past" stays true for as long as the pointer sits still. Claiming on that
// level would have a tooltip hidden by a later claimant take top straight
// back on its next layout, and the two would trade the register every
// frame. Callers claim once per dwell; tooltipState.claimed is the latch.
type Arbiter struct {
	// top is the tooltip currently visible, or nil. The state pointer is
	// the identity: it is unique per subscription for as long as anything
	// can reference it, which is exactly the lifetime an id counter used to
	// synthesise.
	top *tooltipState
}

// NewArbiter returns an empty arbitration set. Create one per window and
// hand it to every tooltip in that window's tree.
func NewArbiter() *Arbiter { return new(Arbiter) }

// defaultArbiter is the set a Props with no Arbiter joins. It restores the
// process-global scope tooltip arbitration had before ADR-008, which is
// indistinguishable from per-window in a single-window process — the shape
// of every application in this organization today.
var defaultArbiter Arbiter

// claim makes st the visible tooltip. Whichever tooltip held top is hidden
// by this very store, because a tooltip paints only while it holds top; see
// the type doc for why that is the whole of it.
func (a *Arbiter) claim(st *tooltipState) { a.top = st }

// release drops st's hold on top. A tooltip that was already overtaken does
// not hold top and releasing is then a no-op, which is what keeps a
// departing straggler from hiding the tooltip that replaced it.
func (a *Arbiter) release(st *tooltipState) {
	if a.top == st {
		a.top = nil
	}
}

// isTop reports whether st currently holds top, which for a tooltip is the
// same question as whether it is visible.
func (a *Arbiter) isTop(st *tooltipState) bool { return a.top == st }
