package splitter_test

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
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/splitter"
	"github.com/vibrantgio/theme/tokens"
)

const (
	frameLong  = 200 // the frame's length along the splitter's axis
	frameShort = 100 // and across it
	boundary   = 100 // where the two regions meet, in px
	boundMin   = 20
	boundMax   = 180
)

// backdrop is what the frame is filled with before the splitter draws, so
// that a pixel that is not this colour is a pixel of the line.
var backdrop = color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff}

// harness is one splitter under test: the state, the boundary it reports
// back, and the frame it is laid out in.
type harness struct {
	st    splitter.State
	axis  layout.Axis
	at    float32
	hit   splitter.Span
	moves []float32
	size  image.Point
	ops   op.Ops
	r     gioinput.Router
	dims  layout.Dimensions
}

func newHarness(axis layout.Axis) *harness {
	size := image.Pt(frameLong, frameShort)
	if axis == layout.Vertical {
		size = image.Pt(frameShort, frameLong)
	}
	return &harness{axis: axis, at: boundary, size: size}
}

func (h *harness) props() splitter.Props {
	return splitter.Props{
		Axis:     h.axis,
		Boundary: h.at,
		Min:      boundMin,
		Max:      boundMax,
		Colors:   tokens.DefaultLight,
		HitSpan:  h.hit,
		OnChange: func(at float32) {
			h.at = at
			h.moves = append(h.moves, at)
		},
	}
}

// frame lays the splitter out once through a live router, so that hit
// area, pointer and drag all go through the same path an application's
// frame does.
func (h *harness) frame() {
	h.ops.Reset()
	gtx := layout.Context{
		Ops:         &h.ops,
		Constraints: layout.Exact(h.size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      h.r.Source(),
	}
	paint.FillShape(gtx.Ops, backdrop, clip.Rect{Max: h.size}.Op())
	h.dims = h.st.Layout(gtx, h.props())
	h.r.Frame(&h.ops)
}

// pt places a pointer at a main-axis position, halfway along the cross
// axis, in the frame's own coordinates.
func (h *harness) pt(main float32) f32.Point {
	if h.axis == layout.Vertical {
		return f32.Pt(frameShort/2, main)
	}
	return f32.Pt(main, frameShort/2)
}

// ptAt places a pointer at a main-axis position and a cross-axis one, in
// the frame's own coordinates.
func (h *harness) ptAt(main, cross float32) f32.Point {
	if h.axis == layout.Vertical {
		return f32.Pt(cross, main)
	}
	return f32.Pt(main, cross)
}

// drag presses at from and moves to to, releasing at the end.
func (h *harness) drag(from, to float32) {
	// Two warm-up frames so the hit area is known to the router before
	// the events are queued.
	h.frame()
	h.frame()
	h.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: h.pt(from), Source: pointer.Touch},
		pointer.Event{Kind: pointer.Move, Position: h.pt(to), Source: pointer.Touch},
		pointer.Event{Kind: pointer.Release, Position: h.pt(to), Source: pointer.Touch},
	)
	h.frame()
}

// last is the boundary the splitter reported last.
func (h *harness) last(t *testing.T) float32 {
	t.Helper()
	if len(h.moves) == 0 {
		t.Fatal("the splitter reported no boundary; want at least one")
	}
	return h.moves[len(h.moves)-1]
}

func axes() []struct {
	name string
	axis layout.Axis
} {
	return []struct {
		name string
		axis layout.Axis
	}{
		{"horizontal", layout.Horizontal},
		{"vertical", layout.Vertical},
	}
}

// TestSplitterDragMovesTheBoundaryWithThePointer verifies the plain case
// on both axes: the boundary follows the pointer's main-axis travel, one
// pixel per pixel, and ignores what the pointer does across the axis.
func TestSplitterDragMovesTheBoundaryWithThePointer(t *testing.T) {
	for _, tc := range axes() {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(tc.axis)
			h.drag(boundary, boundary+40)
			if got := h.last(t); got != boundary+40 {
				t.Errorf("boundary after a 40 px drag = %v; want %v", got, float32(boundary+40))
			}
		})
	}
}

// TestSplitterDragClampsAtBothEnds verifies the bounds each region
// allows: a drag well past either end stops at the end and is reported
// there, rather than being dropped or reported past it.
func TestSplitterDragClampsAtBothEnds(t *testing.T) {
	for _, tc := range axes() {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("trailing", func(t *testing.T) {
				h := newHarness(tc.axis)
				h.drag(boundary, frameLong*2)
				if got := h.last(t); got != boundMax {
					t.Errorf("boundary dragged past the trailing end = %v; want %v", got, float32(boundMax))
				}
			})
			t.Run("leading", func(t *testing.T) {
				h := newHarness(tc.axis)
				h.drag(boundary, -frameLong)
				if got := h.last(t); got != boundMin {
					t.Errorf("boundary dragged past the leading end = %v; want %v", got, float32(boundMin))
				}
			})
		})
	}
}

// TestSplitterHitAreaIsWiderThanTheLine verifies the half of the design
// no image can state: what the splitter paints and what it is taken by
// are different sizes. A press two pixels off the hairline — outside the
// line, inside the band — must start a drag.
func TestSplitterHitAreaIsWiderThanTheLine(t *testing.T) {
	h := newHarness(layout.Horizontal)
	h.drag(boundary+2, boundary+42)
	if got := h.last(t); got != boundary+40 {
		t.Errorf("boundary after a drag begun two pixels off the line = %v; want %v", got, float32(boundary+40))
	}
}

// TestSplitterHitSpanNarrowsTheBandAndNotTheLine verifies the one thing a
// caller may say about the cross axis: the seam runs the window's whole
// height and the hand-hold does not, because the bands crossing over it
// belong to the window rather than to the boundary. A press on the line
// outside the span moves nothing and falls through to whatever stands
// there; the same press inside it drags; and the line paints at the
// cross position the hand could not reach.
func TestSplitterHitSpanNarrowsTheBandAndNotTheLine(t *testing.T) {
	// The middle half of the cross extent is what a hand may take, so a
	// quarter of the way along is outside it and half way is inside.
	const (
		outside = frameShort / 8
		inside  = frameShort / 2
	)
	for _, a := range axes() {
		t.Run(a.name, func(t *testing.T) {
			h := newHarness(a.axis)
			h.hit = splitter.Span{Min: frameShort / 4, Max: 3 * frameShort / 4}
			h.frame()
			h.frame()

			h.r.Queue(
				pointer.Event{Kind: pointer.Press, Position: h.ptAt(boundary, outside), Source: pointer.Touch},
				pointer.Event{Kind: pointer.Move, Position: h.ptAt(boundary-40, outside), Source: pointer.Touch},
				pointer.Event{Kind: pointer.Release, Position: h.ptAt(boundary-40, outside), Source: pointer.Touch},
			)
			h.frame()
			if len(h.moves) != 0 {
				t.Errorf("a drag begun outside the span reported %v; want the splitter to have taken no hold at all", h.moves)
			}

			h.r.Queue(
				pointer.Event{Kind: pointer.Press, Position: h.ptAt(boundary, inside), Source: pointer.Touch},
				pointer.Event{Kind: pointer.Move, Position: h.ptAt(boundary-40, inside), Source: pointer.Touch},
				pointer.Event{Kind: pointer.Release, Position: h.ptAt(boundary-40, inside), Source: pointer.Touch},
			)
			h.frame()
			if got := h.last(t); got != boundary-40 {
				t.Errorf("boundary after a drag begun inside the span = %v; want %v", got, float32(boundary-40))
			}

			img := golden.Capture(t, h.size, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, backdrop, clip.Rect{Max: h.size}.Op())
				return h.st.Layout(gtx, h.props())
			})
			if !painted(img, h.axis, int(h.at), outside) {
				t.Error("the line stops where the band does; the span cuts back what a hand may take, not the seam")
			}
		})
	}
}

// TestSplitterThickensUnderTheHandAndRestores verifies the state the line
// itself says: at rest it paints the seam's hairline, with a hand on it
// it paints thicker, and when the hand leaves it is a hairline again.
// The widths are counted off the painted frame, not off the state.
func TestSplitterThickensUnderTheHandAndRestores(t *testing.T) {
	h := newHarness(layout.Horizontal)
	h.frame()
	h.frame()

	if h.st.Grabbed() {
		t.Fatal("the splitter reports a hand on it before any pointer has touched it")
	}
	if got := lineWidth(t, h); got != 1 {
		t.Errorf("resting line = %d px; want 1 (the seam's hairline)", got)
	}

	h.r.Queue(pointer.Event{Kind: pointer.Move, Position: h.pt(boundary), Source: pointer.Mouse})
	h.frame()
	if !h.st.Grabbed() {
		t.Fatal("a pointer rests on the splitter but it does not report a hand on it")
	}
	if got := lineWidth(t, h); got != 2 {
		t.Errorf("line under the hand = %d px; want 2 (thicker than the seam)", got)
	}

	h.r.Queue(pointer.Event{Kind: pointer.Move, Position: h.pt(boundary - 40), Source: pointer.Mouse})
	h.frame()
	if h.st.Grabbed() {
		t.Fatal("the pointer has left the splitter but it still reports a hand on it")
	}
	if got := lineWidth(t, h); got != 1 {
		t.Errorf("line after the hand left = %d px; want 1 (a hairline again)", got)
	}
}

// TestSplitterDragEndsOnRelease verifies that a release lets go: the
// boundary stops following the pointer.
func TestSplitterDragEndsOnRelease(t *testing.T) {
	h := newHarness(layout.Horizontal)
	h.drag(boundary, boundary+40)
	if h.st.Dragging() {
		t.Fatal("the splitter is still dragging after a release")
	}
	moves := len(h.moves)
	h.r.Queue(pointer.Event{Kind: pointer.Move, Position: h.pt(boundary + 80), Source: pointer.Touch})
	h.frame()
	if len(h.moves) != moves {
		t.Errorf("the splitter reported %d more boundaries after the release; want 0", len(h.moves)-moves)
	}
}

// TestSplitterShowsTheResizePointer verifies the pointer over the band:
// the column resize on a boundary that moves across the window, the row
// resize on one that moves up and down it.
func TestSplitterShowsTheResizePointer(t *testing.T) {
	cases := []struct {
		name string
		axis layout.Axis
		want pointer.Cursor
	}{
		{"horizontal", layout.Horizontal, pointer.CursorColResize},
		{"vertical", layout.Vertical, pointer.CursorRowResize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(tc.axis)
			h.frame()
			h.frame()
			h.r.Queue(pointer.Event{Kind: pointer.Move, Position: h.pt(boundary), Source: pointer.Mouse})
			h.frame()
			if got := h.r.Cursor(); got != tc.want {
				t.Errorf("pointer over the splitter = %v; want %v", got, tc.want)
			}
		})
	}
}

// TestSplitterWithoutStateDrawsALineNothingTakes is the static render
// path: a nil state draws the seam and adds neither hit area nor
// pointer, because a stored image has no hand in it.
func TestSplitterWithoutStateDrawsALineNothingTakes(t *testing.T) {
	var st *splitter.State
	if st.Grabbed() || st.Dragging() {
		t.Fatal("a nil state reports a hand on the splitter")
	}
	size := image.Pt(frameLong, frameShort)
	img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, backdrop, clip.Rect{Max: size}.Op())
		return st.Layout(gtx, splitter.Props{
			Axis:     layout.Horizontal,
			Boundary: boundary,
			Min:      boundMin,
			Max:      boundMax,
			Colors:   tokens.DefaultLight,
		})
	})
	if got := paintedRun(img, frameShort/2, frameLong); got != 1 {
		t.Errorf("line drawn without a state = %d px; want 1 (the seam's hairline)", got)
	}

	var r gioinput.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      r.Source(),
	}
	st.Layout(gtx, splitter.Props{Axis: layout.Horizontal, Boundary: boundary, Colors: tokens.DefaultLight})
	r.Frame(&ops)
	r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(boundary, frameShort/2), Source: pointer.Mouse})
	if got := r.Cursor(); got != pointer.CursorDefault {
		t.Errorf("pointer over a splitter nothing can take = %v; want %v", got, pointer.CursorDefault)
	}
}

// lineWidth captures the harness's current frame and counts the painted
// run across the line, so that what is asserted is what a reader sees
// rather than what the state believes.
func lineWidth(t *testing.T, h *harness) int {
	t.Helper()
	img := golden.Capture(t, h.size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, backdrop, clip.Rect{Max: h.size}.Op())
		return h.st.Layout(gtx, h.props())
	})
	return paintedRun(img, frameShort/2, frameLong)
}

// painted reports whether the pixel at a main-axis and cross-axis
// position carries anything but the backdrop.
func painted(img *image.RGBA, axis layout.Axis, main, cross int) bool {
	p := image.Pt(main, cross)
	if axis == layout.Vertical {
		p = image.Pt(cross, main)
	}
	r, g, b, _ := img.At(p.X, p.Y).RGBA()
	return uint8(r>>8) != backdrop.R || uint8(g>>8) != backdrop.G || uint8(b>>8) != backdrop.B
}

// paintedRun counts the longest run of pixels along row y that is not the
// backdrop, over the first n columns.
func paintedRun(img *image.RGBA, y, n int) int {
	run, best := 0, 0
	for x := range n {
		r, g, b, _ := img.At(x, y).RGBA()
		if uint8(r>>8) == backdrop.R && uint8(g>>8) == backdrop.G && uint8(b>>8) == backdrop.B {
			run = 0
			continue
		}
		run++
		best = max(best, run)
	}
	return best
}
