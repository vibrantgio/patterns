package shell_test

import (
	"context"
	"image"
	"image/color"
	"testing"
	"time"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/attachment"
	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/tooltip"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// centredRow lays trigger out at the trailing end of a row that centres
// what it holds on its middle, across the whole of the slot it is given: the
// offset of trigger is decided only after the row has measured it. trail is
// the padding the row keeps after trigger, in pixels.
func centredRow(trigger layout.Widget, trail int) layout.Widget {
	pad := func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(trail, 0)}
	}
	return func(gtx layout.Context) layout.Dimensions {
		return attachment.PlacingFlex(gtx, layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle},
			attachment.PlacedFlexed(1, fillNothing),
			attachment.PlacedRigid(trigger),
			attachment.PlacedRigid(pad))
	}
}

// rowTooltip is a shown tooltip, below a small square trigger, standing at
// the trailing end of a centred row; it is shown only while the context
// says so (see showTooltips).
func rowTooltip(c tokens.PlatformColors) layout.Widget {
	props := tooltip.Props{
		Text:          "A tooltip at the window's edge",
		Placement:     tooltip.Bottom,
		AvailableRoom: attachment.AvailableRoom,
		Trigger: func(gtx layout.Context) layout.Dimensions {
			sz := image.Pt(roomTrigger, roomTrigger)
			paint.FillShape(gtx.Ops, c.SystemBlue, clip.Rect{Max: sz}.Op())
			return layout.Dimensions{Size: sz}
		},
	}
	sh := tokens.DefaultTypography.DeterministicShaper()
	shown := tooltip.Render(sh, props, true, c, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.Caption1)
	hidden := tooltip.Render(sh, props, false, c, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.Caption1)
	return centredRow(func(gtx layout.Context) layout.Dimensions {
		if gtx.Values[showTooltips] == true {
			return shown(gtx)
		}
		return hidden(gtx)
	}, 0)
}

// TestTooltipInCentredRowIsInsideWindow lays a centred row out across an
// arrangement's main slot, with a tooltip at its trailing end, against the
// window's trailing edge. The row decides the tooltip's offset only after
// measuring it, and then places its surface in the area moved by that
// offset, so the surface is shifted inward and drawn whole inside the window.
func TestTooltipInCentredRowIsInsideWindow(t *testing.T) {
	c := tokens.PlatformLight
	assertTooltipInsideWindow(t, twoColSize, func(gtx layout.Context) layout.Dimensions {
		d, _ := shell.TwoColumn{PaneWidth: 200, Surface: c.WindowBackground, ContentFill: c.TextBackground,
			Pane: shell.PaneColumn{Rows: fillNothing}, Main: rowTooltip(c)}.Layout(gtx, c)
		return d
	})
}

// focusFrame lays w out against ops and r at now, with the window's content
// area set as the available room when setRoom says so, and ends the frame.
func focusFrame(w layout.Widget, ops *op.Ops, r *gioinput.Router, size image.Point, now time.Time, setRoom bool) {
	ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(size),
		Ops:         ops,
		Source:      r.Source(),
		Now:         now,
	}
	if setRoom {
		gtx = shell.WithAvailableRoom(gtx, image.Rectangle{Max: size})
	}
	w(gtx)
	r.Frame(ops)
}

// pixelsPastWindowByFocus lays a window out that is a plain fill with a
// centred row across it, and a tooltip at the row's trailing end around a
// button. The keyboard moves to the button and the tooltip is shown after
// its delay; the window is then captured inside a marked band and the band's
// pixels the tooltip painted over are counted.
func pixelsPastWindowByFocus(t *testing.T, size image.Point, setRoom bool) int {
	t.Helper()
	c := tokens.PlatformLight
	const delay = 50 * time.Millisecond
	th := rx.Of(theme.Default())
	var save widget.Clickable
	live := func(obs rx.Observable[layout.Widget]) layout.Widget {
		var w layout.Widget
		if err := obs.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
			if !done && next != nil {
				w = next
			}
		}).Wait(); err != nil || w == nil {
			t.Fatalf("the stream emitted no layout.Widget: %v", err)
		}
		return w
	}
	saveButton := live(button.Button(th, button.Props{Title: "Save", Clickable: &save}))
	// A button fills the width it is given; the trigger gives it its own.
	btn := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(64))
		return saveButton(gtx)
	}
	arb := tooltip.NewArbiter()
	tip := live(tooltip.Tooltip(th, tooltip.Props{
		Text: "Save the note to the notebook it was opened from", Trigger: btn,
		Placement: tooltip.Bottom, Delay: delay, Arbiter: arb,
		TriggerTag: &save, AvailableRoom: attachment.AvailableRoom,
		Shaper: tokens.DefaultTypography.DeterministicShaper(),
	}))
	// The padding after the button keeps its focus ring, which is drawn
	// outside its box, inside the window.
	row := centredRow(tip, 8)
	window := func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, c.WindowBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return row(gtx)
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)
	t0 := time.Unix(1700000000, 0)
	focusFrame(window, ops, r, size, t0, setRoom)
	r.MoveFocus(key.FocusForward)
	focusFrame(window, ops, r, size, t0, setRoom)
	if !r.Source().Focused(&save) {
		t.Fatal("the Tab did not stop on the button")
	}
	shownAt := t0.Add(delay).Add(time.Millisecond)
	focusFrame(window, ops, r, size, shownAt, setRoom)

	outer := size.Add(image.Pt(2*roomMargin, 2*roomMargin))
	img := golden.Capture(t, outer, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, roomMarker, clip.Rect{Max: outer}.Op())
		st := op.Offset(image.Pt(roomMargin, roomMargin)).Push(gtx.Ops)
		wgtx := gtx
		wgtx.Constraints = layout.Exact(size)
		wgtx.Source = r.Source()
		wgtx.Now = shownAt
		if setRoom {
			wgtx = shell.WithAvailableRoom(wgtx, image.Rectangle{Max: size})
		}
		window(wgtx)
		st.Pop()
		return layout.Dimensions{Size: outer}
	})
	marker := color.RGBA{R: roomMarker.R, G: roomMarker.G, B: roomMarker.B, A: roomMarker.A}
	win := image.Rectangle{Max: size}.Add(image.Pt(roomMargin, roomMargin))
	n := 0
	for y := 0; y < outer.Y; y++ {
		for x := 0; x < outer.X; x++ {
			if !image.Pt(x, y).In(win) && img.RGBAAt(x, y) != marker {
				n++
			}
		}
	}
	return n
}

// TestTooltipShownByFocusInCentredRowIsInsideWindow shows the tooltip of a
// button at the trailing end of a centred row by moving the keyboard to the
// button, and reads that it is placed as one shown by hover is: with no
// available room set it is drawn past the window's trailing edge, which
// shows the band sees it, and with the window's content area set by the root
// the row places it whole inside the window.
func TestTooltipShownByFocusInCentredRowIsInsideWindow(t *testing.T) {
	size := image.Pt(480, 120)
	if n := pixelsPastWindowByFocus(t, size, false); n == 0 {
		t.Fatalf("with no available room the focused tooltip painted nothing past the window: the test does not see it")
	}
	if n := pixelsPastWindowByFocus(t, size, true); n != 0 {
		t.Errorf("with the window's available room set, %d pixels were painted past the window's edge, want 0", n)
	}
}
