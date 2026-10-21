package shell

import (
	"sync/atomic"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/pane"
)

// PaneWindow is the window treatment a shell stands in, with the caller's
// own options after it: the content extends behind a transparent title bar,
// so the strip across the window's top is the application's to lay out in
// and the pane may run to the window's own top edge.
//
// Away from the treatment the platform keeps its decorations, the
// measurements report zero and the arrangement lays out from its own edge
// insets.
func PaneWindow(opts ...app.Option) []app.Option {
	return append(desktop.FullSizeContent(), opts...)
}

// buttonPlace remembers the placement the window's control buttons were
// last asked for, so the request is sent when it changes and not on every
// emission.
var buttonPlace atomic.Value

// buttonPlacement is a full placement request: where the buttons' leading
// edge sits and the line they are centred on, both in dp from the window's
// top-leading corner, zero per axis meaning the platform's own geometry
// there.
type buttonPlacement struct{ leading, center unit.Dp }

// StandWindowButtons re-asserts the platform's three control buttons and
// places them where a pane's own strip keeps them. Gio re-hides them on
// every configuration rebuild, so the re-assertion is registered on the mvu
// configure seam rather than made once.
//
// The buttons are measured from the window's own glass and from nothing
// drawn beneath them, so nothing the application draws may move them: not
// the pane arriving and not the pane leaving. [pane.Buttons] is where they
// stand, derived from the platform's measured inset.
//
// standing is the screen's answer to whether the application lays out in
// the strip at all. A screen that lays its content out BELOW the native
// strip asks for no placement, so the whole strip is the platform's there.
// A nil predicate is a window whose every screen stands in the strip.
func StandWindowButtons(w *mvu.Window, standing func() bool) {
	desktop.ShowWindowButtons(w)
	PlaceWindowButtons(standing == nil || standing())
}

// PlaceWindowButtons asks the window to place its control buttons where the
// screen on show says, and sends the request only when the answer changes.
// A window whose screens differ calls it as each comes up.
func PlaceWindowButtons(standing bool) {
	p := buttonPlacement{}
	if standing {
		p = buttonPlacement{leading: pane.Buttons.Leading, center: pane.Buttons.Center}
	}
	if prev, ok := buttonPlace.Load().(buttonPlacement); ok && prev == p {
		return
	}
	buttonPlace.Store(p)
	desktop.PlaceWindowButtonsAt(p.leading, p.center)
}

// ChromeTop is how far down a window's overlays must start: the strip the
// screen on show lays out itself, and the native strip where that is taller
// — with the pane away the buttons are back on the platform's own line, and
// the strip they stand in can reach below a shallower one. An overlay
// clears the larger of the two, since either would put it on top of a live
// control.
//
// band is the depth the screen lays out itself, which is [ToolbarDepth] for
// a screen carrying a toolbar and zero for one laying out under the native
// strip.
func ChromeTop(band unit.Dp) unit.Dp {
	if inset := desktop.TopInset(); inset > band {
		return inset
	}
	return band
}

// CapChrome offsets an overlay down past everything at the top of the
// window that is not document, and claims what is left of the native
// strip's drag over the run it insets. It is what a toast or a dialog
// stands in, because either landing on the toolbar would cover controls
// that are live under it.
func CapChrome(band func() unit.Dp, w layout.Widget) layout.Widget {
	return desktop.CapTop(func() unit.Dp { return ChromeTop(band()) }, w)
}
