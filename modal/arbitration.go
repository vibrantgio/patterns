package modal

// Arbiter is the open-modal stack shared by a set of modals: the order they
// were opened in, bottom-to-top, and therefore which one is in front. It is a
// plain value — no mutex, no atomics, no observable — because every write and
// every read happens during layout, on the single goroutine Gio runs a frame
// on. The hazard a synchronised bus guards against cannot arise here, so
// there is nothing for a guard to buy. See ADR-008 in the plan.
//
// # The register is the scope
//
// Modals that share an Arbiter stack with one another and with nobody else. A
// window is one widget tree laid out by one goroutine, so one Arbiter per
// window is both the correct scope for "which modal is in front" and the only
// scope a lock-free value is safe at. Create one in the window's composition
// root and hand it to every modal in that window's tree.
//
// A Props with no Arbiter gets a stack of its own and is therefore always the
// modal in front of it. Until G0C.4 it joined a package-level default
// instead, which was indistinguishable from per-window in a single-window
// process and was a data race in a two-window one; there is now no package
// state for a second window to reach, so sharing is something a caller does
// on purpose or not at all. Two modals that both forget an Arbiter both take
// input — a fault anyone can drive into, which is the trade this makes
// against a race nobody can.
//
// # A stack, not a register — because modals nest
//
// Popover and tooltip hold a single top: a claimant evicts the incumbent and
// the incumbent is simply gone. A modal opened from inside another modal does
// not evict it; it covers it, and closing the inner one hands the front back
// to the outer one. That is the whole reason this is an ordered slice and not
// a pointer, and it is the third shape ADR-008's idiom takes.
//
// # The register is also the liveness
//
// Being in front is not a second copy of anything: [Arbiter.isTop] is the only
// place the question is answered, and it is asked once per frame by the modal
// asking it about itself. Only that modal processes keyboard and pointer
// input. The ones beneath keep painting — their scrims and surfaces are drawn
// unconditionally — but register no event tags and drain no keys, so they are
// visible and inert, which is what a stack of dialogs looks like.
//
// # Push on the edge — and a stack forgives a level
//
// A claim guarded by a level rather than an edge re-takes top on every frame,
// and for popover and tooltip that is a bug: their write is unconditionally
// "become top", so two participants trade the register every frame (ADR-008,
// G0C.2). A stack does not have that failure: push is "join if absent", so a
// modal already on the stack keeps the position it has and cannot climb over
// the modal that covered it. Callers still push on the edge of Open —
// modalState.pushed is the latch — because that is what makes the matching pop
// exact, but the level would not have cost a frame here.
type Arbiter struct {
	// open is the stack of modals currently open, bottom-to-top. Each
	// entry is a modal's own state pointer: unique for as long as anything
	// can reference it, which is exactly the lifetime an id counter used to
	// synthesise.
	open []*modalState
}

// NewArbiter returns an empty modal stack. Create one per window and hand it
// to every modal in that window's tree.
func NewArbiter() *Arbiter { return new(Arbiter) }

// push puts st in front. A modal already on the stack keeps the position it
// has: opening is an edge, so this cannot happen from a caller that latches
// its transitions, and a modal that climbed over the one covering it would be
// the wrong answer anyway.
func (a *Arbiter) push(st *modalState) {
	for _, x := range a.open {
		if x == st {
			return
		}
	}
	a.open = append(a.open, st)
}

// pop removes st from the stack, wherever it sits. Popping from the middle is
// the ordinary case, not an error: an outer modal whose Open goes false while
// an inner one is still up leaves without disturbing which modal is in front.
func (a *Arbiter) pop(st *modalState) {
	for i, x := range a.open {
		if x == st {
			a.open = append(a.open[:i], a.open[i+1:]...)
			return
		}
	}
}

// isTop reports whether st is the modal in front, which is the same question
// as whether st takes input this frame.
func (a *Arbiter) isTop(st *modalState) bool {
	n := len(a.open)
	return n > 0 && a.open[n-1] == st
}
