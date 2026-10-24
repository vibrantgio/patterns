// Package shell provides the Patterns Shell pattern: a top-level
// application layout.
//
// [TwoColumn] is the application shell the platform's own sidebar windows
// stand in, and the one a window with a pane composes itself from: the pane
// down the leading edge with its own column and its bare marks, the content
// column beside it carrying one [Toolbar] across its top, an optional
// inspector down the trailing edge, a footer under each column, a splitter
// on each boundary, and the one [ToggleSidebar] button that stands bare on
// the pane while it is there and bordered in the toolbar once it is away.
// It reports a [PaneLayout] — the arrangement in window coordinates — and
// keeps the widths and the shown state through an [ArrangementMemory].
//
// [ThreeColumn] is the arrangement beside it, for a window whose document is
// chosen from a list: the same pane, then a list column of absolute width on
// a splitter of its own with its rows and its footer, then the content
// column absorbing the rest, with one toolbar across the two of them and one
// share per column. It reports a [ThreeColumnLayout] and keeps its two
// widths through the same memory.
//
// [Shell] builds the third composition the package ships, the stacked page:
// a pinned full-width navbar over a shell-owned vertical scroll of page
// sections, which is the marketing-page shell. [Props] is what it is stated
// with and [RenderStackedPage] draws it with pre-resolved tokens and no
// event processing.
//
// [EmptyState] is what a region with nothing to show stands in place of its
// content — a line, an optional description, optional actions — centred as
// one group in the region's full frame and stated with [EmptyStateProps].
//
// A window's root sets its content area as the available room with
// [WithAvailableRoom], and every floating surface under it is passed
// [attachment.AvailableRoom]. A layout between the root and a floating surface
// that pushes an offset moves the available room and the floating surfaces
// ([attachment.Surfaces]) together with [attachment.AvailableRoomAt] before it lays
// its child out; a layout that pushes an offset without moving them places
// the surface away from its anchor. A layout that places its child only after
// measuring it sets an [attachment.Surfaces] with [attachment.WithSurfaces] and
// places the surfaces with [attachment.PlaceSurfacesAt] once the child is
// placed, as [attachment.PlacingFlex] does for a flex.
//
// Shell is a callable Go function consuming a components theme
// observable, returning a stream of layout.Widget. Source is
// intentionally short — copy it into your own app and modify as needed.
package shell

import (
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/patterns/navbar"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/theme/theme"
)

// Props configures the stacked page [Shell] and [RenderStackedPage] build.
type Props struct {
	// Navbar is the band pinned across the page's top edge.
	Navbar navbar.Props

	// Footer is an optional strip appended after the last section. It
	// scrolls with the content at its natural height instead of pinning
	// to the viewport; a nil Footer omits the strip.
	Footer layout.Widget

	// Sections are stacked top to bottom in a scroll region owned by
	// the shell. Each entry is a layout.Widget stream, so sections
	// re-render on theme change without a layer-boundary adapter; the
	// shell combines them and re-emits whenever any section emits. Nil
	// entries render empty. Each section spans the full page width
	// (less the ContentMaxWidth clamp, when set) and receives an
	// unbounded height, so it must return its natural height. The
	// static render path takes pre-built section layout.Widget values
	// via RenderStackedPage instead (Props.Sections is not consulted
	// there).
	Sections []rx.Observable[layout.Widget]

	// ContentMaxWidth, when positive, clamps every section (and the
	// scrolling Footer) to at most this width and centers the clamped
	// column on the page; the navbar stays full-bleed. The zero value
	// keeps sections at the full page width, in which case sections
	// own their internal max-width/centering — a full-bleed background
	// with a centered inner column composes naturally. Sections
	// narrower than the page still paint the page Background in the
	// side margins.
	ContentMaxWidth unit.Dp
}

// NavbarHeight returns the pinned depth of the navbar band the shell draws:
// [pane.BandDp], 52 dp, which is the band every stored toolbar capture
// measures — nineteen dp of inset either side of a fourteen dp window
// control button. patterns/navbar fills whatever it is handed, so the bar
// stands in the band rather than settling it.
//
// It takes no density. The band is the PLATFORM's: the three window control
// buttons stand a measured inset in from the window's own glass, the band
// that holds them centred follows from that one number, and no setting of
// how tightly a window sets its rows moves it.
//
// This is the number an app needs when it caps a shell window's top edge at
// the depth of the navbar band.
func NavbarHeight() unit.Dp {
	return unit.Dp(pane.BandDp)
}

// Shell returns an rx.Observable[layout.Widget] emitting the stacked page
// afresh whenever a consumed theme token or a composed section stream
// changes. Navbar event handling is delegated to patterns/navbar.
func Shell(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	return stackedPageObservable(th, props)
}

func emptyWidget(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{Size: gtx.Constraints.Min}
}
