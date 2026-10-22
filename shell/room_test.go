package shell_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/tooltip"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/tokens"
)

// roomMargin is the band drawn around the window in the room tests: a
// tooltip surface placed past the window's edge paints into it.
const roomMargin = 96

// roomMarker is the band's colour, one no arrangement or tooltip paints.
var roomMarker = color.NRGBA{R: 0xff, G: 0x00, B: 0xff, A: 0xff}

// roomTrigger is the side of the square a corner tooltip's trigger takes.
const roomTrigger = 16

// cornerTooltip is a slot that places a shown tooltip at its own trailing
// bottom corner, below the trigger, and moves the available room by the
// offset it places the tooltip at, as a caller of the shell does.
//
// The tooltip is shown only while the context says so (see showTooltips),
// so one window is captured with its tooltip shown and hidden.
func cornerTooltip(c tokens.PlatformColors) layout.Widget {
	props := tooltip.Props{
		Text:          "A tooltip at the window's edge",
		Placement:     tooltip.Bottom,
		AvailableRoom: shell.AvailableRoom,
		Trigger: func(gtx layout.Context) layout.Dimensions {
			sz := image.Pt(roomTrigger, roomTrigger)
			paint.FillShape(gtx.Ops, c.SystemBlue, clip.Rect{Max: sz}.Op())
			return layout.Dimensions{Size: sz}
		},
	}
	sh := tokens.DefaultTypography.DeterministicShaper()
	shown := tooltip.Render(sh, props, true, c, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.LabelSmall)
	hidden := tooltip.Render(sh, props, false, c, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.LabelSmall)
	return func(gtx layout.Context) layout.Dimensions {
		tip := hidden
		if gtx.Values[showTooltips] == true {
			tip = shown
		}
		size := gtx.Constraints.Max
		off := size.Sub(image.Pt(roomTrigger, roomTrigger))
		st := op.Offset(off).Push(gtx.Ops)
		tgtx := shell.AvailableRoomAt(gtx, off)
		tgtx.Constraints = layout.Exact(image.Pt(roomTrigger, roomTrigger))
		tip(tgtx)
		st.Pop()
		return layout.Dimensions{Size: size}
	}
}

// showTooltips is the context value that shows the corner tooltips.
const showTooltips = "room-test.show-tooltips"

// pixelsPastWindow lays window out at size inside a marked band, once with
// its tooltips shown and once hidden, and counts the band's pixels that
// differ: what a shown tooltip painted past the window's edge. The pane's
// shadow reaches past the window's foot in both captures and is not
// counted. setRoom says whether the root sets the window's content area as
// the available room.
func pixelsPastWindow(t *testing.T, size image.Point, setRoom bool, window layout.Widget) int {
	t.Helper()
	outer := size.Add(image.Pt(2*roomMargin, 2*roomMargin))
	capture := func(show bool) *image.RGBA {
		return golden.Capture(t, outer, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, roomMarker, clip.Rect{Max: outer}.Op())
			st := op.Offset(image.Pt(roomMargin, roomMargin)).Push(gtx.Ops)
			wgtx := gtx
			wgtx.Constraints = layout.Exact(size)
			wgtx.Values = map[string]any{showTooltips: show}
			if setRoom {
				wgtx = shell.WithAvailableRoom(wgtx, image.Rectangle{Max: size})
			}
			window(wgtx)
			st.Pop()
			return layout.Dimensions{Size: outer}
		})
	}
	shown, hidden := capture(true), capture(false)
	win := image.Rectangle{Max: size}.Add(image.Pt(roomMargin, roomMargin))
	n := 0
	for y := 0; y < outer.Y; y++ {
		for x := 0; x < outer.X; x++ {
			if !image.Pt(x, y).In(win) && shown.RGBAAt(x, y) != hidden.RGBAAt(x, y) {
				n++
			}
		}
	}
	return n
}

// assertTooltipInsideWindow reads one arrangement twice: with no available
// room set the corner tooltip is drawn past the window's edge, which shows
// the band sees it, and with the window's content area set by the root the
// tooltip is drawn whole inside the window.
func assertTooltipInsideWindow(t *testing.T, size image.Point, window layout.Widget) {
	t.Helper()
	if n := pixelsPastWindow(t, size, false, window); n == 0 {
		t.Fatalf("with no available room the corner tooltip painted nothing past the window: the test does not see it")
	}
	if n := pixelsPastWindow(t, size, true, window); n != 0 {
		t.Errorf("with the window's available room set, %d pixels were painted past the window's edge, want 0", n)
	}
}

// TestTooltipInTrailingSlotIsInsideWindow places a shown tooltip at the
// trailing bottom corner of each arrangement's trailing slot, below its
// trigger, and reads that it is drawn whole inside the window: the
// arrangement moves the area the root set into the slot's coordinates, so
// the tooltip takes the side above and keeps inside the window's trailing
// edge.
func TestTooltipInTrailingSlotIsInsideWindow(t *testing.T) {
	c := tokens.PlatformLight
	size := twoColSize
	for _, tc := range []struct {
		name   string
		window func(gtx layout.Context) layout.Dimensions
	}{
		{"two-column-main", func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.TwoColumn{PaneWidth: 200, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing}, Main: cornerTooltip(c)}.Layout(gtx, c)
			return d
		}},
		{"two-column-main-first", func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.TwoColumn{PaneWidth: 200, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing}, Main: cornerTooltip(c), MainFirst: true}.Layout(gtx, c)
			return d
		}},
		{"two-column-inspector", func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.TwoColumn{PaneWidth: 200, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing}, Main: fillNothing,
				Inspector: cornerTooltip(c), InspectorWidth: 160, InspectorGap: 6}.Layout(gtx, c)
			return d
		}},
		{"two-column-footer", func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.TwoColumn{PaneWidth: 200, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing}, Main: fillNothing,
				Footer: cornerTooltip(c), FooterDepth: 22}.Layout(gtx, c)
			return d
		}},
		{"two-column-over", func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.TwoColumn{PaneWidth: 200, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing}, Main: fillNothing, Over: cornerTooltip(c)}.Layout(gtx, c)
			return d
		}},
		{"pane-column-rows", func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.TwoColumn{PaneWidth: 200, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Header: probe(c.SystemBlue, 184, 24), Rows: cornerTooltip(c)},
				Main: fillNothing}.Layout(gtx, c)
			return d
		}},
		{"pane-column-footer", func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.TwoColumn{PaneWidth: 200, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing, Footer: cornerTooltip(c), FooterDepth: 22},
				Main: fillNothing}.Layout(gtx, c)
			return d
		}},
		{"three-column-main", func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.ThreeColumn{PaneWidth: 200, ListWidth: 240, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing}, List: shell.ListColumn{Rows: fillNothing},
				Main: cornerTooltip(c)}.Layout(gtx, c)
			return d
		}},
		{"three-column-main-first", func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.ThreeColumn{PaneWidth: 200, ListWidth: 240, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing}, List: shell.ListColumn{Rows: fillNothing},
				Main: cornerTooltip(c), MainFirst: true}.Layout(gtx, c)
			return d
		}},
		{"three-column-footer", func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.ThreeColumn{PaneWidth: 200, ListWidth: 240, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing}, List: shell.ListColumn{Rows: fillNothing},
				Main: fillNothing, Footer: cornerTooltip(c), FooterDepth: 22}.Layout(gtx, c)
			return d
		}},
		{"list-column-rows", func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.ThreeColumn{PaneWidth: 200, ListWidth: 240, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing}, List: shell.ListColumn{Rows: cornerTooltip(c)},
				Main: fillNothing}.Layout(gtx, c)
			return d
		}},
		{"list-column-footer", func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.ThreeColumn{PaneWidth: 200, ListWidth: 240, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing},
				List: shell.ListColumn{Rows: fillNothing, Footer: cornerTooltip(c), FooterDepth: 22},
				Main: fillNothing}.Layout(gtx, c)
			return d
		}},
		{"pane-frame-main", func(gtx layout.Context) layout.Dimensions {
			return shell.PaneFrame{Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Sidebar: fillNothing, Main: cornerTooltip(c)}.Layout(gtx, c)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertTooltipInsideWindow(t, size, tc.window)
		})
	}
}

// TestAvailableRoomUnsetIsUnbounded reads the value a caller reads where no
// root set an area: a rectangle wide enough to place any annotation as no
// available room does.
func TestAvailableRoomUnsetIsUnbounded(t *testing.T) {
	gtx := measureCtx(twoColSize)
	r := shell.AvailableRoom(gtx)
	if !image.Rect(-1<<19, -1<<19, 1<<19, 1<<19).In(r) {
		t.Errorf("AvailableRoom with nothing set = %v, want an unbounded rectangle", r)
	}
	if got := shell.AvailableRoom(shell.AvailableRoomAt(gtx, image.Pt(10, 10))); got != r {
		t.Errorf("AvailableRoomAt with nothing set moved the area to %v", got)
	}
	set := shell.WithAvailableRoom(gtx, image.Rect(0, 0, 100, 50))
	if got := shell.AvailableRoom(shell.AvailableRoomAt(set, image.Pt(10, 20))); got != image.Rect(-10, -20, 90, 30) {
		t.Errorf("AvailableRoomAt moved the area to %v, want (-10,-20)-(90,30)", got)
	}
	if got := shell.AvailableRoom(gtx); got != r {
		t.Errorf("WithAvailableRoom wrote into the caller's context: %v", got)
	}
}
