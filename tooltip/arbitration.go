package tooltip

// Arbiter is the "only one tooltip visible at a time" rule shared by a
// set of tooltips. It is a plain value — no mutex, no atomics, no
// observable — because every write and every read happens during layout, on
// the single goroutine Gio runs a frame on. The hazard a synchronised bus
// guards against cannot arise here, so there is nothing for a guard to buy.
//
// # The arbiter is the scope
//
// Tooltips that share an Arbiter arbitrate with one another and with nobody
// else. A window is one layout.Widget tree laid out by one goroutine, so one
// Arbiter per window is both the correct scope for "which tooltip is up"
// and the only scope a lock-free value is safe at. Create one in the
// window's composition root and hand it to every tooltip in that window's
// tree.
//
// A Props with no Arbiter gets one of its own and therefore arbitrates with
// nobody: there is no package-level state for a second window to reach, so
// sharing is something a caller does on purpose or not at all. Two tooltips
// that both forget an Arbiter can be up together — a cosmetic fault anyone
// can see, which is the trade this makes against a race nobody can.
//
// # The arbiter is also the visibility
//
// Popover keeps a hold flag of its own beside the arbiter and has the
// arbiter call OnDismiss when the two must be brought back into step. A
// tooltip has no caller state and no dismissal callback — it is visible
// exactly while it holds top — so the store that makes a claimant top is
// the whole of the incumbent's dismissal. There is nothing to notify and no
// second copy to keep in step, which is why claim takes no layout.Context
// and is one assignment.
//
// The incumbent stops painting on the first frame it is laid out after
// losing top: in the same frame when the claimant comes earlier in the
// layout.Widget tree, on the next frame when it does not, because one already
// laid out cannot un-paint.
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
	// can reference it.
	top *tooltipState
}

// NewArbiter returns an empty arbitration set. Create one per window and
// hand it to every tooltip in that window's tree.
func NewArbiter() *Arbiter { return new(Arbiter) }

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
