package popover_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/attachment"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/popover"
	"github.com/vibrantgio/theme/tokens"
)

var (
	anchorBlue = color.NRGBA{R: 80, G: 160, B: 220, A: 255}
	bandRed    = color.NRGBA{R: 200, G: 40, B: 40, A: 255}
	bandGreen  = color.NRGBA{R: 40, G: 160, B: 40, A: 255}
)

// bands is content w wide and two bands of h tall: red above green. Like a
// column, it reports no more height than its constraints allow, so content
// measured at less than its own height cannot be scrolled to its end.
func bands(w, h int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bandRed, clip.Rect{Max: image.Pt(w, h)}.Op())
		paint.FillShape(gtx.Ops, bandGreen, clip.Rect{Min: image.Pt(0, h), Max: image.Pt(w, 2*h)}.Op())
		return layout.Dimensions{Size: image.Pt(w, min(2*h, gtx.Constraints.Max.Y))}
	}
}

// atY lays w out in a frame of size at y inside the scene, as a window's
// layout places a row: the popover is handed the room the window set, moved
// into the frame's coordinates.
func atY(w layout.Widget, y int, size image.Point) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		st := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		cgtx := gtx
		cgtx.Constraints = layout.Exact(size)
		w(cgtx)
		st.Pop()
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// rowsIn reports the first and last row of column x painted in c.
func rowsIn(img *image.RGBA, x int, c color.NRGBA) (lo, hi int, ok bool) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		r, g, bl, _ := img.At(x, y).RGBA()
		if uint8(r>>8) != c.R || uint8(g>>8) != c.G || uint8(bl>>8) != c.B {
			continue
		}
		if !ok {
			lo, ok = y, true
		}
		hi = y
	}
	return lo, hi, ok
}

// TestAPopoverWithNoRoomBelowTakesTheSideAbove places a popover that prefers
// the side below in a frame at the foot of the window. The room below the
// anchor cannot hold the surface and the room above holds it whole, so the
// surface is drawn whole above the anchor, and the tail's tip is on the
// anchor's top edge at its midline.
func TestAPopoverWithNoRoomBelowTakesTheSideAbove(t *testing.T) {
	const top = 180
	frame := image.Pt(frameW, frameH-top)
	props := popover.Props{
		Anchor:    fixedRect(anchorBlue, 60, 28),
		Content:   bands(80, 18),
		Placement: popover.Bottom,
		AvailableRoom: func(layout.Context) image.Rectangle {
			return image.Rect(0, -top, frameW, frameH-top)
		},
	}
	w := popover.Render(props, true, tokens.PlatformLight, tokens.Spacing, sharpRadius)
	img := golden.Capture(t, frameSize, scene(atY(w, top, frame), color.NRGBA{R: 240, G: 240, B: 240, A: 255}))

	// The anchor is 28 tall, centred in the 60-tall frame: rows 196 to 223.
	// Its top rows are under the surface's shadow, so its foot is what is
	// read in its own colour.
	anchorTop := top + (frame.Y-28)/2
	if _, hi, ok := rowsIn(img, frameW/2, anchorBlue); !ok || hi != anchorTop+27 {
		t.Fatalf("anchor ends at row %d (found %v), want %d", hi, ok, anchorTop+27)
	}
	// The content is drawn whole, both bands, above the anchor.
	redLo, _, okR := rowsIn(img, frameW/2, bandRed)
	_, greenHi, okG := rowsIn(img, frameW/2, bandGreen)
	if !okR || !okG {
		t.Fatalf("content not drawn whole: red %v, green %v", okR, okG)
	}
	if greenHi-redLo+1 != 36 {
		t.Errorf("content spans rows %d to %d, want 36 rows", redLo, greenHi)
	}
	if greenHi >= anchorTop {
		t.Errorf("content ends at row %d, not above the anchor at %d", greenHi, anchorTop)
	}
	// Nothing of the surface is below the anchor.
	if _, _, ok := fillRun(img, anchorTop+28+12, tokens.PlatformLight.WindowBackground); ok {
		t.Errorf("surface fill found below the anchor")
	}
	// The tail's tip is drawn on the row above the anchor's top edge, at the
	// anchor's midline.
	lo, hi, ok := drawnRun(img, anchorTop-1, frameW/2-40, frameW/2+40)
	if !ok {
		t.Fatalf("no tail drawn on the row above the anchor")
	}
	if mid := (lo + hi) / 2; mid < frameW/2-2 || mid > frameW/2+2 {
		t.Errorf("tail tip centred at x=%d, want the anchor's midline at %d", mid, frameW/2)
	}
}

// TestAPopoverTallerThanBothSidesScrollsInsideItself places a popover whose
// content is taller than the room above and below the anchor. The surface is
// capped to the side it wins and ends on the room's edge, the content beyond
// it is not drawn, and a scroll over the surface brings the rest into view.
func TestAPopoverTallerThanBothSidesScrollsInsideItself(t *testing.T) {
	room := image.Rect(0, 0, frameW, 200)
	props := popover.Props{
		Anchor:        fixedRect(anchorBlue, 60, 28),
		Content:       bands(80, 200),
		Placement:     popover.Bottom,
		AvailableRoom: func(layout.Context) image.Rectangle { return room },
	}
	w := popover.Render(props, true, tokens.PlatformLight, tokens.Spacing, sharpRadius)
	frame := func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(room.Size())
			return w(gtx)
		}
	}
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}

	img := golden.Capture(t, frameSize, scene(frame(w), bg))
	// The anchor's foot is at 114, the gap is 8: below it the room leaves 78
	// rows, more than the 78 above, so the preferred side is kept.
	_, surfHi, ok := rowsIn(img, frameW/2+45, tokens.PlatformLight.WindowBackground)
	if !ok || surfHi > room.Max.Y {
		t.Errorf("surface ends at row %d (found %v), past the room's edge at %d", surfHi, ok, room.Max.Y)
	}
	if _, _, ok := rowsIn(img, frameW/2, bandGreen); ok {
		t.Errorf("the content's lower band is drawn: the surface did not cap it")
	}
	redLo, _, ok := rowsIn(img, frameW/2, bandRed)
	if !ok || redLo <= 114 {
		t.Fatalf("content's top band not drawn below the anchor (row %d, found %v)", redLo, ok)
	}

	// Scroll over the surface: the lower band comes into view.
	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveFrame(frame(w), ops, r, frameSize)
	driveFrame(frame(w), ops, r, frameSize)
	at := f32.Pt(frameW/2, 160)
	r.Queue(pointer.Event{Kind: pointer.Scroll, Position: at, Source: pointer.Mouse, Scroll: f32.Pt(0, 1000)})
	driveFrame(frame(w), ops, r, frameSize)
	img = golden.Capture(t, frameSize, scene(frame(w), bg))
	_, greenHi, ok := rowsIn(img, frameW/2, bandGreen)
	if !ok {
		t.Fatalf("after a scroll over the surface the content's lower band is still not drawn")
	}
	// Scrolled to its end, the content's last row stands on the viewport's
	// foot: the surface ends at the room's edge less its padding.
	if want := surfHi - 12; greenHi < want-1 {
		t.Errorf("scrolled to its end, the content ends at row %d, short of the viewport's foot at %d: the end cannot be scrolled to", greenHi, want)
	}
}

// TestAPopoverInAPlacingFlexChildIsPlacedInTheWindowsRoom lays a popover out
// in the second child of a flex that places its children's floating surfaces,
// under a window that set the available room. The popover reports no room of
// its own: its surface is recorded and placed by the flex in the window's
// room, so with no room below the anchor it takes the side above.
func TestAPopoverInAPlacingFlexChildIsPlacedInTheWindowsRoom(t *testing.T) {
	const top = 180
	props := popover.Props{
		Anchor:    fixedRect(anchorBlue, 60, 28),
		Content:   bands(80, 18),
		Placement: popover.Bottom,
	}
	w := popover.Render(props, true, tokens.PlatformLight, tokens.Spacing, sharpRadius)
	window := func(gtx layout.Context) layout.Dimensions {
		gtx = attachment.WithAvailableRoom(gtx, image.Rectangle{Max: gtx.Constraints.Max})
		return attachment.PlacingFlex(gtx, layout.Flex{Axis: layout.Vertical},
			attachment.PlacedRigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(frameW, top)}
			}),
			attachment.PlacedRigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(image.Pt(frameW, frameH-top))
				return w(gtx)
			}),
		)
	}
	img := golden.Capture(t, frameSize, scene(window, color.NRGBA{R: 240, G: 240, B: 240, A: 255}))
	_, greenHi, ok := rowsIn(img, frameW/2, bandGreen)
	if !ok || greenHi >= top+16 {
		t.Errorf("content ends at row %d (found %v), want above the anchor at %d", greenHi, ok, top+16)
	}
}

// TestAPopoverInAWindowReportingNoRoomIsKeptInsideTheFrame passes
// attachment.AvailableRoom in a window that set no available room, so the
// room it reports bounds nothing. The surface is then placed against the
// frame as a popover with no AvailableRoom is: centred on an anchor at the
// frame's trailing edge it would run past that edge, and it is shifted back
// to end on it.
func TestAPopoverInAWindowReportingNoRoomIsKeptInsideTheFrame(t *testing.T) {
	colors := tokens.PlatformLight
	props := popover.Props{
		Anchor:        fixedRect(color.NRGBA{R: 80, G: 160, B: 220, A: 255}, 60, 28),
		Content:       fixedRect(color.NRGBA{R: 120, G: 120, B: 120, A: 255}, 160, 36),
		Placement:     popover.Bottom,
		Align:         popover.AlignTrailing,
		AvailableRoom: attachment.AvailableRoom,
	}
	w := popover.Render(props, true, colors, tokens.Spacing, sharpRadius)
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	img := golden.Capture(t, frameSize, scene(inRoom(w, roomW), bg))
	lo, hi, ok := fillRun(img, 148, colors.WindowBackground)
	if !ok {
		t.Fatal("no surface pixels on the row under the surface's top edge")
	}
	if hi >= roomW || hi < roomW-3 {
		t.Errorf("surface ends at x=%d; inside the %d-wide frame it ends on its edge at %d", hi, roomW, roomW-1)
	}
	if lo < 0 {
		t.Errorf("surface starts at x=%d, off the frame's leading edge", lo)
	}
}

// TestAPopoverRecordedTwoLayoutsDeepInAWindowReportingNoRoomIsKeptInsideTheFrame
// records the popover's surface in a layout that places its child at an
// offset, inside another layout that places the surfaces, in a window that
// set no available room. The room handed down is the unbounded one moved by
// that offset, which still bounds nothing, so the surface is placed against
// the frame and ends on its trailing edge.
func TestAPopoverRecordedTwoLayoutsDeepInAWindowReportingNoRoomIsKeptInsideTheFrame(t *testing.T) {
	colors := tokens.PlatformLight
	props := popover.Props{
		Anchor:    fixedRect(color.NRGBA{R: 80, G: 160, B: 220, A: 255}, 60, 28),
		Content:   fixedRect(color.NRGBA{R: 120, G: 120, B: 120, A: 255}, 160, 36),
		Placement: popover.Bottom,
		Align:     popover.AlignTrailing,
	}
	w := popover.Render(props, true, colors, tokens.Spacing, sharpRadius)
	off := image.Pt(0, 1)
	window := func(gtx layout.Context) layout.Dimensions {
		sgtx, s := attachment.WithSurfaces(gtx)
		st := op.Offset(off).Push(gtx.Ops)
		inRoom(w, roomW)(attachment.AvailableRoomAt(sgtx, off))
		st.Pop()
		attachment.PlaceSurfaces(gtx, s, attachment.AvailableRoom(gtx))
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	img := golden.Capture(t, frameSize, scene(window, color.NRGBA{R: 240, G: 240, B: 240, A: 255}))
	lo, hi, ok := fillRun(img, 148, colors.WindowBackground)
	if !ok {
		t.Fatal("no surface pixels on the row under the surface's top edge")
	}
	if hi >= roomW || hi < roomW-3 {
		t.Errorf("surface ends at x=%d; inside the %d-wide frame it ends on its edge at %d", hi, roomW, roomW-1)
	}
	if lo < 0 {
		t.Errorf("surface starts at x=%d, off the frame's leading edge", lo)
	}
}
