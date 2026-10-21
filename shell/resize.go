package shell

import (
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/patterns/splitter"
)

// PaneResize is the splitter on the pane's trailing edge, where the content
// column stands flush against it and the pane's own RIM is the boundary
// between the two.
//
// The line this splitter draws at rest IS that rim: the same pixel, in the
// pane's own rim colour, drawn over it rather than beside it. A resting
// window is unchanged by the splitter being there; what the reader gains is
// a band to take the edge by and a thickening under the hand that takes it.
// A second line a few dp off the first would read as a stray edge.
//
// The boundary is the rim's own LEADING edge and not the pane's trailing
// one, which is the pixel after it: the splitter draws from the boundary
// outward, so the pane's edge as the boundary would put the line in the
// content column instead of on the pane.
//
// A nil *PaneResize is a pane whose width is fixed. A nil State is a
// boundary nothing can take hold of, which is the static render path.
type PaneResize struct {
	// State is the hand across frames, held by the window.
	State *splitter.State

	// Min and Max are the pane's own bounds. The content column's floor
	// stands over them: the pane stops where widening it further would
	// take the main column under the narrowest measure it may lay out at.
	Min, Max unit.Dp

	// Floor is that narrowest measure. It gives way to what the main column
	// already has, so a window too narrow for every column's minimum
	// together does not jump the boundary out from under the hand taking
	// hold of it: a boundary may always be dragged the way that gives the
	// main column room, and never the way that takes more of it away.
	Floor unit.Dp

	// OnChange reports the pane's new width, already clamped, for every
	// pointer event that moves it.
	OnChange func(width unit.Dp)

	// at and dragged carry this frame's drag from the splitter's own
	// update back into the arrangement that is about to measure off it.
	at      unit.Dp
	dragged bool
}

// InspectorResize is the splitter on the inspector's leading edge.
//
// The inspector is integral chrome — flush, with no button and no way to
// leave — so it is not outlined the way the pane is. What parts it from the
// content column is the platform's plain seam, which is why this boundary
// draws the seam's own colour where the pane's draws the rim's.
//
// The line runs from the toolbar's lower edge to the window's foot, because
// no line crosses a toolbar. The hand-hold is shorter still: the strips
// above and below the columns are the window's own — one carries the
// window's drag, the other reports on the document — and neither is resized
// by this boundary, so the hold is the document row alone.
type InspectorResize struct {
	// State is the hand across frames, held by the window.
	State *splitter.State

	// Min and Max are the inspector's own bounds, with the main column's
	// floor over them.
	Min, Max unit.Dp

	// Floor is the main column's narrowest measure, read as
	// [PaneResize.Floor] is.
	Floor unit.Dp

	// OnChange reports the inspector's new width, already clamped.
	OnChange func(width unit.Dp)

	at      unit.Dp
	dragged bool
}

// PxPerDp is the metric a boundary reported in pixels is carried back to dp
// by, never zero: a width kept in dp is the same physical measure on a
// screen of a different pixel density, which is the whole reason a kept
// width is kept in dp rather than in what a drag reports.
func PxPerDp(gtx layout.Context) float32 {
	if gtx.Metric.PxPerDp <= 0 {
		return 1
	}
	return gtx.Metric.PxPerDp
}

// floorOver answers the room a column may not be squeezed out of: the floor
// it states, or what it already has where the window is too narrow for that
// much.
func floorOver(gtx layout.Context, floor unit.Dp, have int) int {
	return min(gtx.Dp(floor), have)
}

// moved reports one new width to the window and keeps it, so the
// arrangement — a value built fresh every frame — lays the rest of the
// frame out at where this frame's drag left the boundary rather than
// where the last one did.
func (r *PaneResize) moved(w unit.Dp) {
	r.at, r.dragged = w, true
	if r.OnChange != nil {
		r.OnChange(w)
	}
}

// width answers the width this frame's drag left, or have where no drag
// moved it.
func (r *PaneResize) width(have unit.Dp) unit.Dp {
	if r == nil || !r.dragged {
		return have
	}
	r.dragged = false
	return r.at
}

// moved and width are [PaneResize.moved] and [PaneResize.width] for the
// inspector's boundary.
func (r *InspectorResize) moved(w unit.Dp) {
	r.at, r.dragged = w, true
	if r.OnChange != nil {
		r.OnChange(w)
	}
}

func (r *InspectorResize) width(have unit.Dp) unit.Dp {
	if r == nil || !r.dragged {
		return have
	}
	r.dragged = false
	return r.at
}
