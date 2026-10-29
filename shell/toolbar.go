package shell

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/attachment"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// ToolbarDepth is the strip's depth across a content column: the platform's
// toolbar, a 36 dp control with 8 above it and 8 below. It is
// [pane.BandDp] and not a second statement of it — the window's three
// control buttons stand a measured inset in from the glass and the depth
// that holds them centred follows from that one number, which patterns/pane
// states.
//
// It takes no tokens: the depth is the platform's and the same in every
// density, as the pane's strip beside it is.
const ToolbarDepth = pane.BandDp

// ToolbarMetrics are the gutters and gaps a toolbar stands its content at.
// Every one is a platform measurement read off the stored captures;
// [PlatformToolbar] carries the readings and a window states a deviation
// rather than a value.
type ToolbarMetrics struct {
	// Trail is what the toolbar leaves between its last control and the
	// window's own trailing edge. MEASURED at 1x, all four stored toolbar
	// windows agree on eight: finder-window-light.png's search capsule ends
	// x=991 in a window 1000 wide, finder-window-untinted-dark's search
	// field ends 1322 in one 1331 wide, mail-window.png's search recess ends
	// 1191 in one 1200 wide and voicememos-window.png's ends 967 in one 976
	// wide.
	Trail unit.Dp

	// ControlGap is the room between two bordered controls standing merely
	// adjacent. MEASURED, Finder, and the reading
	// reference/macos/controls.md carries.
	ControlGap unit.Dp

	// PairGap is the room between two bordered controls that belong
	// together, which is closer than merely adjacent. MEASURED, Notes.
	// [ToolbarPair] is how a group spends it.
	PairGap unit.Dp

	// TitleGap is the room between a bare title and a bordered control
	// beside it. MEASURED at 1x, read twice: in finder-window-light.png the
	// back/forward pair's fill ends at x=398 and the title's first painted
	// column stands at x=413, and in finder-window-untinted-dark.png the
	// same pair ends at x=398 with its title's first painted column at
	// x=412.
	TitleGap unit.Dp

	// Depth is the toolbar's own depth. Zero takes [ToolbarDepth].
	Depth unit.Dp
}

// PlatformToolbar is the platform's own reading of those five, which is
// what a toolbar stands at where its window states no deviation.
var PlatformToolbar = ToolbarMetrics{
	Trail:      8,
	ControlGap: 16,
	PairGap:    14,
	TitleGap:   14,
	Depth:      ToolbarDepth,
}

// resolve answers m with every unstated measurement taken from the
// platform's reading.
func (m ToolbarMetrics) resolve() ToolbarMetrics {
	if m.Trail <= 0 {
		m.Trail = PlatformToolbar.Trail
	}
	if m.ControlGap <= 0 {
		m.ControlGap = PlatformToolbar.ControlGap
	}
	if m.PairGap <= 0 {
		m.PairGap = PlatformToolbar.PairGap
	}
	if m.TitleGap <= 0 {
		m.TitleGap = PlatformToolbar.TitleGap
	}
	if m.Depth <= 0 {
		m.Depth = PlatformToolbar.Depth
	}
	return m
}

// Toolbar is the strip a content column carries across its top: the
// controls that act on the document, at the platform's own spacing.
//
// It is a value spent into [TwoColumn] rather than a layout call, because
// the arrangement decides where it starts — the column's own gutter while
// the pane stands, and past the window's control buttons once the pane is
// away ([ToolbarLead]) — and the arrangement is what knows which.
//
// Every control in the leading and the trailing group states its own width,
// and the toolbar lays it out once. A control titled with a symbol alone
// draws the capsule ([ToolbarControl]); a control titled with text is laid
// out through [ToolbarTitled], at the text's width and an inset each side. A
// slot of fixed width reports that width.
//
// Composed as the stored toolbars compose theirs: a leading group, the
// centre, a flexible middle that moves the window, a trailing group, and
// the trailing gutter. The middle and every gap carry the window's drag,
// since under the full-size-content treatment the native title bar hands
// none over and a toolbar may claim it back over the parts of itself that
// hold no control.
type Toolbar struct {
	// ButtonFallback is the leading inset the toolbar leads from where the
	// window has no control buttons to clear — away from the
	// full-size-content treatment, in headless rendering, and on every
	// platform that keeps its own decorations.
	ButtonFallback unit.Dp

	// Lead is the toolbar's own leading gutter: the content column's own
	// inset, so a control at the toolbar's leading end stands over the
	// first column of the content beneath it. It is stated by the window
	// and has no platform default — the number is the column's, not the
	// toolbar's.
	Lead unit.Dp

	// Leading holds what stands at the leading end, in reading order,
	// parted by [ToolbarMetrics.ControlGap]. The Toggle Sidebar button goes
	// in front of them while the pane is away, and the arrangement puts it
	// there: it is not one of these.
	Leading []layout.Widget

	// Center is what the toolbar says the window is showing — a title, or a
	// trail of the places it stands in. It follows the leading group one
	// [ToolbarMetrics.TitleGap] along, and nothing is laid out in its place
	// when it is nil.
	Center layout.Widget

	// Trailing holds what stands at the trailing end, in reading order,
	// parted by [ToolbarMetrics.ControlGap], with
	// [ToolbarMetrics.Trail] between the last of them and the window's
	// edge.
	Trailing []layout.Widget

	// Metrics states a deviation from [PlatformToolbar]. An unstated
	// measurement takes the platform's.
	Metrics ToolbarMetrics
}

// depth is the toolbar's depth in the metrics it was given.
func (t Toolbar) depth() unit.Dp { return t.Metrics.resolve().Depth }

// layout draws the toolbar into the context it is given, which is the
// content column's share of the window's top strip at the toolbar's own
// depth. buttons is the run at the leading end the window's control
// buttons claim, in pixels, over which no drag is declared; gutter is what
// the toolbar leaves after it.
//
// The flex centres what it holds. The depth IS the platform's toolbar, and
// the platform puts the window buttons on that strip's own middle, so a
// control centred here stands on the buttons' line without being told to.
// What reaches past the toolbar's foot is the drop shadow its controls
// cast, which the platform does not cut off at the lower edge either.
func (t Toolbar) layout(gtx layout.Context, buttons, gutter int, lead []layout.Widget) layout.Dimensions {
	m := t.Metrics.resolve()
	// The row centres every control on its middle and places the trailing
	// group from the end, so a control's offset is decided only after it is
	// laid out: the row places the tooltip surfaces of its controls once it
	// has placed them (see [attachment.PlacingFlex]).
	children := make([]attachment.PlacedChild, 0, len(lead)+len(t.Leading)+len(t.Trailing)+6)
	if buttons > 0 {
		// The buttons' own run is left alone: a move action declared over
		// them would fight them for the press.
		children = append(children, attachment.PlacedRigid(complayout.HSpacer(float32(buttons))))
	}
	if gutter > 0 {
		children = append(children, attachment.PlacedRigid(dragRun(gutter)))
	}
	group := append(append([]layout.Widget{}, lead...), t.Leading...)
	for i, w := range group {
		if i > 0 {
			children = append(children, attachment.PlacedRigid(pane.DragSpacer(m.ControlGap)))
		}
		children = append(children, attachment.PlacedRigid(w))
	}
	if t.Center != nil {
		if len(group) > 0 {
			children = append(children, attachment.PlacedRigid(pane.DragSpacer(m.TitleGap)))
		}
		children = append(children, attachment.PlacedRigid(t.Center))
	}
	// The bare run between the centre and the trailing group carries
	// nothing: finder-window-light.png reads 255 at every column of its
	// toolbar clear of a control. It is the flexible middle, so the
	// trailing group keeps its fixed end while everything before it keeps
	// the leading one.
	children = append(children, attachment.PlacedFlexed(1, pane.DragFill))
	for i, w := range t.Trailing {
		if i > 0 {
			children = append(children, attachment.PlacedRigid(pane.DragSpacer(m.ControlGap)))
		}
		children = append(children, attachment.PlacedRigid(w))
	}
	children = append(children, attachment.PlacedRigid(pane.DragSpacer(m.Trail)))
	// The toolbar hands what stands in it ROOM and not a depth: a control
	// or a title reports its own box and the row centres it. Left at the
	// row's own minimum every child would be cut to the full depth and
	// drawn from its top edge, which centres nothing.
	gtx.Constraints.Min.Y = 0
	return attachment.PlacingFlex(gtx, layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}, children...)
}

// ToolbarPair lays controls that belong together out as one entry of a
// toolbar group: parted by the measured [ToolbarMetrics.PairGap] rather
// than by the wider gap between two merely adjacent controls. It places the
// tooltip surfaces of its controls once it has placed them, as the toolbar
// does.
func ToolbarPair(controls ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]attachment.PlacedChild, 0, 2*len(controls))
		for i, w := range controls {
			if i > 0 {
				children = append(children, attachment.PlacedRigid(pane.DragSpacer(PlatformToolbar.PairGap)))
			}
			children = append(children, attachment.PlacedRigid(w))
		}
		return attachment.PlacingFlex(gtx, layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}, children...)
	}
}

// ToolbarTitleInsetDp is the room a toolbar control titled with text leaves
// between its edge and its text, on each side. components/button draws that
// control, and a push button never draws narrower than its title and the
// regular push button's title inset each side: 13 px, MEASURED in
// save-panel-untinted-{light,dark}.png and
// save-panel-footer-untinted-{light,dark}.png. A slot narrower than that
// floor makes the control overflow it. Calendar's Today button, a toolbar
// control, measures 15 px each side in reference/macos/controls.md.
const ToolbarTitleInsetDp unit.Dp = 13

// ToolbarTitleWidth answers the width of a toolbar control titled with
// text: the width of title set in style, and [ToolbarTitleInsetDp] each
// side. The shaper sets the text alone; the control is not laid out. The
// weight is font.Normal where style states none.
func ToolbarTitleWidth(gtx layout.Context, sh *text.Shaper, style tokens.TextStyle, title string) int {
	return typeset.LineWidth(gtx, sh, title, style, font.Normal) + 2*gtx.Dp(ToolbarTitleInsetDp)
}

// ToolbarTitled lays control, a toolbar control titled with title, out once
// at [ToolbarTitleWidth]. control draws its text in style through sh, so the
// width it is given is the width its text needs. Where the row has less
// room, control takes the room the row has.
func ToolbarTitled(sh *text.Shaper, style tokens.TextStyle, title string, control layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		w := min(ToolbarTitleWidth(gtx, sh, style, title), gtx.Constraints.Max.X)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
		return control(gtx)
	}
}

// ToolbarLead answers where a toolbar's own content may start, in pixels
// from the content column's leading edge: the larger of the toolbar's own
// gutter and the inset past the window's control buttons, BOTH expressed as
// columns of the window.
//
// It is one measurement in one state. Read instead as a flag at either end
// — the gutter while the pane stands, the buttons' inset once it is away —
// the leading edge steps at the instant the pane reaches zero, which is
// what a pane that collapses and expands rather than switching makes
// visible. The larger-of-two reading gives the same answer in both rests
// and passes between them without a step.
//
// Both halves of the Toggle Sidebar button lead from this one answer, so
// the control stands in one window column whichever way the pane goes.
//
// contentX is where the content column begins in the window. buttonsEnd is
// the trailing edge of the window's control buttons, or 0 where the window
// has none; fallback is what the toolbar leads from there. The two runs are
// reported apart because the buttons' own run may carry no window drag.
func ToolbarLead(gtx layout.Context, contentX int, gutter, buttonsEnd, fallback unit.Dp) (buttons, gutterRun int) {
	past := gtx.Dp(desktop.BandLeadFrom(buttonsEnd, pane.ButtonGapDp, fallback)) - contentX
	buttons = max(past, 0)
	return buttons, max(max(gtx.Dp(gutter), buttons)-buttons, 0)
}

// dragRun is a fixed run of pixels that moves the window when it is
// dragged, for a width already measured.
func dragRun(w int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return desktop.DragRun(gtx, w)
	}
}

// toolbarBounds answers the box a toolbar laid out in, for the arrangement
// to report.
func toolbarBounds(x, w, depth int) image.Rectangle {
	return image.Rect(x, 0, x+w, depth)
}
