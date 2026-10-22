package shell_test

import (
	"context"
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/navbar"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/splitter"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

const (
	splitW, splitH   = 480, 128 // split-pane frame
	vsplitW, vsplitH = 128, 480 // vertical-axis split-pane frame
	dragFrameW       = 200
	dragFrameH       = 100
	tabFrameW        = 480
	tabFrameH        = 256
)

var (
	splitSize  = image.Pt(splitW, splitH)
	vsplitSize = image.Pt(vsplitW, vsplitH)
	dragSize   = image.Pt(dragFrameW, dragFrameH)
	vdragSize  = image.Pt(dragFrameH, dragFrameW) // 100×200: tall frame for Y drags
	tabSize    = image.Pt(tabFrameW, tabFrameH)
)

// defaultShaper returns the shaper every golden here draws with: the default
// typography's faces pinned, system fonts off, so the stored images are the
// same on every machine. A golden test pins its faces with
// DeterministicShaper; application code takes the fallback Shaper.
func defaultShaper(t *testing.T) *text.Shaper {
	t.Helper()
	return tokens.DefaultTypography.DeterministicShaper()
}

// Shell draws no text of its own: everything legible in these goldens comes
// through the composed navbar. These titles are what make the slot
// boundaries legible. Titles must stay ASCII — Latin text in Roboto
// rasterises identically on every machine via DeterministicShaper, but a
// symbol may not.
var navLinkTitles = []string{"Docs", "Components"}

// shellNavbar returns the navbar props every shell golden composes.
func shellNavbar(shaper *text.Shaper) navbar.Props {
	links := make([]navbar.Link, len(navLinkTitles))
	for i, l := range navLinkTitles {
		links[i] = navbar.Link{Title: l}
	}
	return navbar.Props{Links: links, Shaper: shaper}
}

// fillRect is a solid-coloured filler used as Main / Left / Right slots
// in goldens; its colour distinguishes regions visually.
func fillRect(c color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	}
}

func scene(w layout.Widget, bg color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// TestShellGolden records or diffs the split-pane goldens.
func TestShellGolden(t *testing.T) {
	shaper := defaultShaper(t)
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	leftFill := color.NRGBA{R: 0x22, G: 0x55, B: 0x88, A: 0xff}
	rightFill := color.NRGBA{R: 0x88, G: 0x55, B: 0x22, A: 0xff}

	splitProps := func(axis layout.Axis) shell.Props {
		return shell.Props{
			Layout:    shell.SplitPane,
			Left:      fillRect(leftFill),
			Right:     fillRect(rightFill),
			SplitAxis: axis,
		}
	}

	cases := []struct {
		name   string
		props  shell.Props
		colors tokens.PlatformColors
		bg     color.NRGBA
		size   image.Point
		ratio  float32
	}{
		{"light-split-pane-50-50", splitProps(layout.Horizontal), tokens.PlatformLight, lightBG, splitSize, 0.5},
		{"light-split-pane-30-70", splitProps(layout.Horizontal), tokens.PlatformLight, lightBG, splitSize, 0.3},
		{"light-split-pane-vertical-30-70", splitProps(layout.Vertical), tokens.PlatformLight, lightBG, vsplitSize, 0.3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := shell.Render(shaper, tc.props, tc.colors, tokens.Spacing, tokens.DefaultTypography.LabelLarge, tokens.Comfortable, tc.ratio)
			golden.Render(t, tc.name, tc.size, scene(w, tc.bg))
		})
	}
}

// ---- Interaction tests ----

func liveWidget(t *testing.T, sh rx.Observable[layout.Widget]) layout.Widget {
	t.Helper()
	var w layout.Widget
	if err := sh.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("Shell subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("Shell did not emit an initial layout.Widget")
	}
	return w
}

func driveFrame(w layout.Widget, ops *op.Ops, r *gioinput.Router, size image.Point) layout.Dimensions {
	ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(size),
		Ops:         ops,
		Source:      r.Source(),
	}
	dims := w(gtx)
	r.Frame(ops)
	return dims
}

// TestShellSplitPaneSplitterDrag verifies that pressing on the seam and
// dragging horizontally emits ratio updates via OnSplitChange. With
// PxPerDp=1 and frame 200×100 at initial ratio 0.5, the painted seam
// (1 px) sits at x=100 and the grab band (6 px, centred on it) at
// x ∈ [98, 104). A press at (100, 50) followed by a drag to (150, 50)
// shifts the ratio by 50/200 = +0.25, so the expected new ratio is 0.75.
func TestShellSplitPaneSplitterDrag(t *testing.T) {
	var got []float32
	props := shell.Props{
		Layout:        shell.SplitPane,
		SplitRatio:    rx.Of(float32(0.5)),
		OnSplitChange: func(_ layout.Context, r float32) { got = append(got, r) },
	}
	w := liveWidget(t, shell.Shell(rx.Of(theme.Default()), props))

	r := new(gioinput.Router)
	ops := new(op.Ops)
	// Warm-up frames so the splitter's clip area is registered with the
	// router before pointer events are queued.
	driveFrame(w, ops, r, dragSize)
	driveFrame(w, ops, r, dragSize)

	press := f32.Pt(100, 50)
	drag := f32.Pt(150, 50)
	// Press at the splitter, Move to drag — the router converts the Move
	// to a pointer.Drag for the press target. Release ends the gesture.
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: press, Source: pointer.Touch},
		pointer.Event{Kind: pointer.Move, Position: drag, Source: pointer.Touch},
		pointer.Event{Kind: pointer.Release, Position: drag, Source: pointer.Touch},
	)
	driveFrame(w, ops, r, dragSize)

	if len(got) == 0 {
		t.Fatalf("OnSplitChange not invoked; want at least one update")
	}
	last := got[len(got)-1]
	const want = 0.75
	const eps = 0.01
	if last < want-eps || last > want+eps {
		t.Errorf("final ratio = %v; want ~%v", last, want)
	}
}

// TestShellSplitPaneVerticalSplitterDrag is the SplitAxis=Vertical
// counterpart of TestShellSplitPaneSplitterDrag. With PxPerDp=1 and a
// 100×200 frame at initial ratio 0.5, the horizontal seam sits at
// y=100 under a 6 px grab band at y ∈ [98, 104). A press at (50, 100)
// followed by a drag to (50, 150) shifts the ratio by 50/200 = +0.25,
// so the expected new ratio is 0.75.
func TestShellSplitPaneVerticalSplitterDrag(t *testing.T) {
	var got []float32
	props := shell.Props{
		Layout:        shell.SplitPane,
		SplitAxis:     layout.Vertical,
		SplitRatio:    rx.Of(float32(0.5)),
		OnSplitChange: func(_ layout.Context, r float32) { got = append(got, r) },
	}
	w := liveWidget(t, shell.Shell(rx.Of(theme.Default()), props))

	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveFrame(w, ops, r, vdragSize)
	driveFrame(w, ops, r, vdragSize)

	press := f32.Pt(50, 100)
	drag := f32.Pt(50, 150)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: press, Source: pointer.Touch},
		pointer.Event{Kind: pointer.Move, Position: drag, Source: pointer.Touch},
		pointer.Event{Kind: pointer.Release, Position: drag, Source: pointer.Touch},
	)
	driveFrame(w, ops, r, vdragSize)

	if len(got) == 0 {
		t.Fatalf("OnSplitChange not invoked; want at least one update")
	}
	last := got[len(got)-1]
	const want = 0.75
	const eps = 0.01
	if last < want-eps || last > want+eps {
		t.Errorf("final ratio = %v; want ~%v", last, want)
	}
}

// TestShellSplitPaneSeamIsAHairline pins the two halves of the seam's
// shape that no golden can state on its own: it paints one pixel wide,
// and it paints that pixel on every row — the top row and the bottom row
// included.
//
// Both halves are load-bearing and they pull in opposite directions. The
// width is why an application may paint a band across the top of its
// window without the seam severing it: at a hairline the seam crosses
// the band the way a platform splitter does instead of splitting it into
// two pieces. The full-height run is why the band is not simply exempted
// from the seam: an edge that stops short of the window's top leaves the
// two panes' fills meeting with nothing between them, and the seam is
// the only thing saying where one pane ends and the other begins.
func TestShellSplitPaneSeamIsAHairline(t *testing.T) {
	shaper := defaultShaper(t)
	colors := tokens.PlatformLight
	props := shell.Props{
		Layout: shell.SplitPane,
		Left:   fillRect(color.NRGBA{R: 0x22, G: 0x55, B: 0x88, A: 0xff}),
		Right:  fillRect(color.NRGBA{R: 0x88, G: 0x55, B: 0x22, A: 0xff}),
	}
	w := shell.Render(shaper, props, colors, tokens.Spacing,
		tokens.DefaultTypography.LabelLarge, tokens.Comfortable, 0.5)
	img := golden.Capture(t, splitSize, w)

	// runAt reports the seam's start column and width on one row, where
	// "seam" is any run of pixels matching the platform's measured seam.
	seam := splitter.SeamColor(colors)
	runAt := func(y int) (start, width int) {
		start = -1
		for x := 0; x < splitW; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			isSeam := uint8(r>>8) == seam.R &&
				uint8(g>>8) == seam.G &&
				uint8(b>>8) == seam.B
			switch {
			case isSeam && start < 0:
				start, width = x, 1
			case isSeam:
				width++
			case start >= 0:
				return start, width
			}
		}
		return start, width
	}

	// Every row, not a sample of them: the seam must be the same
	// hairline at every row, top edge included.
	wantStart, wantWidth := runAt(splitH / 2)
	if wantWidth != 1 {
		t.Errorf("seam width at mid-height = %d px; want 1 (a hairline)", wantWidth)
	}
	for y := 0; y < splitH; y++ {
		start, width := runAt(y)
		if start != wantStart || width != wantWidth {
			t.Fatalf("seam at row %d = %d px at x=%d; want %d px at x=%d — the seam must be the same hairline on every row, top edge included",
				y, width, start, wantWidth, wantStart)
		}
	}
}

// TestShellSplitPaneGrabBandReachesIntoPanes verifies the other half of
// the seam's design: what it paints and what it grabs are different
// sizes. A hairline is an impossible pointer target, so the band that
// drags it reaches into both panes and is registered above them.
//
// The press below lands two pixels into the trailing pane, over a hit
// area that pane put under the whole of itself. It must drive the split
// and leave that area alone — the grab band shields it.
func TestShellSplitPaneGrabBandReachesIntoPanes(t *testing.T) {
	var got []float32
	paneTag := new(struct{ _ byte })
	panePresses := 0
	props := shell.Props{
		Layout:     shell.SplitPane,
		SplitRatio: rx.Of(float32(0.5)),
		Right: func(gtx layout.Context) layout.Dimensions {
			for {
				e, ok := gtx.Event(pointer.Filter{Target: paneTag, Kinds: pointer.Press})
				if !ok {
					break
				}
				if pe, ok := e.(pointer.Event); ok && pe.Kind == pointer.Press {
					panePresses++
				}
			}
			defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
			event.Op(gtx.Ops, paneTag)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		},
		OnSplitChange: func(_ layout.Context, r float32) { got = append(got, r) },
	}
	w := liveWidget(t, shell.Shell(rx.Of(theme.Default()), props))

	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveFrame(w, ops, r, dragSize)
	driveFrame(w, ops, r, dragSize)

	// Frame 200×100 at ratio 0.5 with PxPerDp=1: the seam is the single
	// pixel at x=100 and the trailing pane starts at x=101, so x=103 is
	// inside both the pane and the 6 px grab band at [98, 104).
	press := f32.Pt(103, 50)
	drag := f32.Pt(153, 50)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: press, Source: pointer.Touch},
		pointer.Event{Kind: pointer.Move, Position: drag, Source: pointer.Touch},
		pointer.Event{Kind: pointer.Release, Position: drag, Source: pointer.Touch},
	)
	driveFrame(w, ops, r, dragSize)
	// A second frame so any press the pane was handed would have been
	// drained by its own event loop before the assertion below.
	driveFrame(w, ops, r, dragSize)

	if len(got) == 0 {
		t.Fatalf("press %v is inside the grab band but OnSplitChange was not invoked", press)
	}
	if panePresses != 0 {
		t.Errorf("the trailing pane saw %d press(es) on the grab band; want 0 — the band takes the hit", panePresses)
	}
}
