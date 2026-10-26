package tabs_test

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/attachment"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/tooltip"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/tabs"
	"github.com/vibrantgio/theme/tokens"
)

// TestTooltipInContentIsInsideWindow places a shown tooltip at the trailing
// bottom corner of the selected tab's content, below its trigger, in a
// window the tabs fill, and reads that it is drawn whole inside the window:
// the tabs move the area the root set below the strip, so the tooltip takes
// the side above and keeps inside the window's trailing edge. With no
// available room set the same tooltip is drawn past the window's edge,
// which shows the reading sees it.
func TestTooltipInContentIsInsideWindow(t *testing.T) {
	const margin, trigger, lift = 96, 16, 8
	c := tokens.PlatformLight
	size := image.Pt(400, 200)
	sh := tokens.DefaultTypography.DeterministicShaper()
	props := tooltip.Props{
		Text:          "A tooltip at the window's edge",
		Placement:     tooltip.Bottom,
		AvailableRoom: attachment.AvailableRoom,
		Trigger: func(gtx layout.Context) layout.Dimensions {
			sz := image.Pt(trigger, trigger)
			paint.FillShape(gtx.Ops, c.SystemBlue, clip.Rect{Max: sz}.Op())
			return layout.Dimensions{Size: sz}
		},
	}
	window := func(show bool) layout.Widget {
		tip := tooltip.Render(sh, props, show, c, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.Caption1)
		content := func(gtx layout.Context) layout.Dimensions {
			// The trigger is placed lift pixels above the content's foot:
			// near enough that the window has no room for the tooltip
			// below it, and far enough that an available room not moved
			// below the strip, which runs one strip past the window's
			// foot, would report room there.
			off := gtx.Constraints.Max.Sub(image.Pt(trigger, trigger+lift))
			st := op.Offset(off).Push(gtx.Ops)
			tgtx := attachment.AvailableRoomAt(gtx, off)
			tgtx.Constraints = layout.Exact(image.Pt(trigger, trigger))
			tip(tgtx)
			st.Pop()
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
		return tabs.Render(sh, tabs.Props{Tabs: []tabs.Tab{{Title: "One", Content: content}, {Title: "Two"}}},
			0, c, tokens.Spacing, tokens.DefaultTypography.SystemFont, tokens.Comfortable)
	}
	past := func(setRoom bool) int {
		outer := size.Add(image.Pt(2*margin, 2*margin))
		capture := func(show bool) *image.RGBA {
			return golden.Capture(t, outer, func(gtx layout.Context) layout.Dimensions {
				st := op.Offset(image.Pt(margin, margin)).Push(gtx.Ops)
				wgtx := gtx
				wgtx.Constraints = layout.Exact(size)
				if setRoom {
					wgtx = shell.WithAvailableRoom(wgtx, image.Rectangle{Max: size})
				}
				window(show)(wgtx)
				st.Pop()
				return layout.Dimensions{Size: outer}
			})
		}
		shown, hidden := capture(true), capture(false)
		win := image.Rectangle{Max: size}.Add(image.Pt(margin, margin))
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
	if n := past(false); n == 0 {
		t.Fatalf("with no available room the tooltip painted nothing past the window: the test does not see it")
	}
	if n := past(true); n != 0 {
		t.Errorf("with the window's available room set, %d pixels were painted past the window's edge, want 0", n)
	}
}
