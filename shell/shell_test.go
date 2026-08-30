package shell_test

import (
	"context"
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/navbar"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

const (
	shmW, shmH       = 480, 256 // sidebar-header-main canvas
	splitW, splitH   = 480, 128 // split-pane canvas
	vsplitW, vsplitH = 128, 480 // vertical-axis split-pane canvas
	dragCanvasW      = 200
	dragCanvasH      = 100
	tabCanvasW       = 480
	tabCanvasH       = 256
)

var (
	shmSize    = image.Pt(shmW, shmH)
	splitSize  = image.Pt(splitW, splitH)
	vsplitSize = image.Pt(vsplitW, vsplitH)
	dragSize   = image.Pt(dragCanvasW, dragCanvasH)
	vdragSize  = image.Pt(dragCanvasH, dragCanvasW) // 100×200: tall canvas for Y drags
	tabSize    = image.Pt(tabCanvasW, tabCanvasH)
)

// defaultShaper returns the shaper every golden here draws with: the default
// typography's faces pinned, system fonts off, so the stored images are the
// same on every machine. A golden test pins its faces with
// DeterministicShaper; application code takes the fallback Shaper. See
// AGENTS.md.
func defaultShaper(t *testing.T) *text.Shaper {
	t.Helper()
	return tokens.DefaultTypography.DeterministicShaper()
}

// Shell draws no text of its own: everything legible in these goldens comes
// through the composed navbar and sidebar. These labels are what make the
// slot boundaries legible. Labels must stay ASCII — Latin text in Roboto
// rasterises identically on every machine via DeterministicShaper, but a
// symbol may not.
var (
	navLinkLabels     = []string{"Docs", "Components"}
	sidebarItemLabels = []string{"Overview", "Tokens"}
)

// shellNavbar returns the navbar props every shell golden composes.
func shellNavbar(shaper *text.Shaper) navbar.Props {
	links := make([]navbar.Link, len(navLinkLabels))
	for i, l := range navLinkLabels {
		links[i] = navbar.Link{Label: l}
	}
	return navbar.Props{Links: links, Shaper: shaper}
}

// shellSidebar returns the sidebar props every shell golden composes.
func shellSidebar(shaper *text.Shaper) sidebar.Props {
	items := make([]sidebar.Item, len(sidebarItemLabels))
	for i, l := range sidebarItemLabels {
		items[i] = sidebar.Item{Icon: testIcon(), Label: l, OnClick: func(_ layout.Context) {}}
	}
	return sidebar.Props{Items: items, Shaper: shaper}
}

func testIcon() layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(16, 16)
		paint.FillShape(gtx.Ops, color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 0xff}, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	}
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

// TestShellGolden records or diffs the four Measurable goldens.
func TestShellGolden(t *testing.T) {
	shaper := defaultShaper(t)
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	darkBG := color.NRGBA{R: 20, G: 20, B: 20, A: 255}
	leftFill := color.NRGBA{R: 0x22, G: 0x55, B: 0x88, A: 0xff}
	rightFill := color.NRGBA{R: 0x88, G: 0x55, B: 0x22, A: 0xff}
	mainFill := color.NRGBA{R: 0x33, G: 0x99, B: 0x66, A: 0xff}

	shmSidebarProps := shellSidebar(shaper)

	shmProps := func() shell.Props {
		return shell.Props{
			Layout: shell.SidebarHeaderMain,
			Navbar: shellNavbar(shaper),
			Main:   fillRect(mainFill),
		}
	}

	splitProps := func(axis layout.Axis) shell.Props {
		return shell.Props{
			Layout:    shell.SplitPane,
			Left:      fillRect(leftFill),
			Right:     fillRect(rightFill),
			SplitAxis: axis,
		}
	}

	cases := []struct {
		name         string
		props        shell.Props
		sidebarProps *sidebar.Props
		colors       tokens.ColorTokens
		bg           color.NRGBA
		size         image.Point
		ratio        float32
	}{
		{"light-sidebar-header-main", shmProps(), &shmSidebarProps, tokens.DefaultLight, lightBG, shmSize, 0},
		{"dark-sidebar-header-main", shmProps(), &shmSidebarProps, tokens.DefaultDark, darkBG, shmSize, 0},
		{"light-split-pane-50-50", splitProps(layout.Horizontal), nil, tokens.DefaultLight, lightBG, splitSize, 0.5},
		{"light-split-pane-30-70", splitProps(layout.Horizontal), nil, tokens.DefaultLight, lightBG, splitSize, 0.3},
		{"light-split-pane-vertical-30-70", splitProps(layout.Vertical), nil, tokens.DefaultLight, lightBG, vsplitSize, 0.3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sidebarW layout.Widget
			if tc.sidebarProps != nil {
				sidebarW = sidebar.Render(shaper, *tc.sidebarProps, false, tc.colors, tokens.Spacing, tokens.DefaultTypography.LabelLarge, tokens.Comfortable)
			}
			w := shell.Render(shaper, tc.props, sidebarW, tc.colors, tokens.Spacing, tokens.DefaultTypography.LabelLarge, tokens.Comfortable, tc.ratio)
			golden.Render(t, tc.name, tc.size, scene(w, tc.bg))
		})
	}
}

// densityTheme returns a theme whose density is d, with sharp corners
// for golden determinism, mirroring components' density tests.
func densityTheme(d tokens.Density) theme.Theme {
	th := theme.Default()
	th.Density = rx.Of(d)
	th.Radius = rx.Of(tokens.RadiusScale{})
	return th
}

// TestShellCompactGolden records or diffs the compact-density golden
// through the LIVE pipeline (the static Render path is frozen at
// tokens.Comfortable): the navbar slot pins at ControlHeight + 2·PaddingY
// = 40 dp instead of 52, and the composed sidebar's item pitch drops to
// 28 dp through its own density subscription.
func TestShellCompactGolden(t *testing.T) {
	shaper := defaultShaper(t)
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	mainFill := color.NRGBA{R: 0x33, G: 0x99, B: 0x66, A: 0xff}
	th := densityTheme(tokens.Compact)

	sbProps := shellSidebar(shaper)
	props := shell.Props{
		Layout:  shell.SidebarHeaderMain,
		Sidebar: sidebar.Sidebar(rx.Of(th), sbProps),
		Navbar:  shellNavbar(shaper),
		Main:    fillRect(mainFill),
	}
	w := liveWidget(t, shell.Shell(rx.Of(th), props))
	golden.Render(t, "light-compact-sidebar-header-main", shmSize, scene(w, lightBG))
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
		t.Fatal("Shell did not emit an initial widget")
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

// TestShellSplitPaneDividerDrag verifies that pressing on the seam and
// dragging horizontally emits ratio updates via OnSplitChange. With
// PxPerDp=1 and canvas 200×100 at initial ratio 0.5, the painted seam
// (1 px) sits at x=100 and the grab band (6 px, centred on it) at
// x ∈ [98, 104). A press at (100, 50) followed by a drag to (150, 50)
// shifts the ratio by 50/200 = +0.25, so the expected new ratio is 0.75.
func TestShellSplitPaneDividerDrag(t *testing.T) {
	var got []float32
	props := shell.Props{
		Layout:        shell.SplitPane,
		SplitRatio:    rx.Of(float32(0.5)),
		OnSplitChange: func(_ layout.Context, r float32) { got = append(got, r) },
	}
	w := liveWidget(t, shell.Shell(rx.Of(theme.Default()), props))

	r := new(gioinput.Router)
	ops := new(op.Ops)
	// Warm-up frames so the divider's clip area is registered with the
	// router before pointer events are queued.
	driveFrame(w, ops, r, dragSize)
	driveFrame(w, ops, r, dragSize)

	press := f32.Pt(100, 50)
	drag := f32.Pt(150, 50)
	// Press at the divider, Move to drag — the router converts the Move
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

// TestShellSplitPaneVerticalDividerDrag is the SplitAxis=Vertical
// counterpart of TestShellSplitPaneDividerDrag. With PxPerDp=1 and a
// 100×200 canvas at initial ratio 0.5, the horizontal seam sits at
// y=100 under a 6 px grab band at y ∈ [98, 104). A press at (50, 100)
// followed by a drag to (50, 150) shifts the ratio by 50/200 = +0.25,
// so the expected new ratio is 0.75.
func TestShellSplitPaneVerticalDividerDrag(t *testing.T) {
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
// the band the way a platform divider does instead of splitting it into
// two pieces. The full-height run is why the band is not simply exempted
// from the seam: an edge that stops short of the window's top leaves the
// two panes' fills meeting with nothing between them, and the seam is
// the only thing saying where one pane ends and the other begins.
func TestShellSplitPaneSeamIsAHairline(t *testing.T) {
	shaper := defaultShaper(t)
	colors := tokens.DefaultLight
	props := shell.Props{
		Layout: shell.SplitPane,
		Left:   fillRect(color.NRGBA{R: 0x22, G: 0x55, B: 0x88, A: 0xff}),
		Right:  fillRect(color.NRGBA{R: 0x88, G: 0x55, B: 0x22, A: 0xff}),
	}
	w := shell.Render(shaper, props, nil, colors, tokens.Spacing,
		tokens.DefaultTypography.LabelLarge, tokens.Comfortable, 0.5)
	img := golden.Capture(t, splitSize, w)

	// runAt reports the seam's start column and width on one row, where
	// "seam" is any run of pixels matching the Divider token.
	runAt := func(y int) (start, width int) {
		start = -1
		for x := 0; x < splitW; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			isSeam := uint8(r>>8) == colors.Divider.R &&
				uint8(g>>8) == colors.Divider.G &&
				uint8(b>>8) == colors.Divider.B
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

	// Canvas 200×100 at ratio 0.5 with PxPerDp=1: the seam is the single
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

// TestShellSidebarHeaderMainTabTraversal verifies that Tab focus
// traverses the shell in document order sidebar → navbar → main. Three
// regions each contribute a focusable; the navbar gets an *external*
// handle (a Brand clickable owned by the test) and the main slot gets
// another. The sidebar item is owned by the sidebar package and is
// only externally observable as "neither brand nor main focused". With
// a seed clickable anchoring focus before the shell, the expected
// sequence is:
//
//	Tab #1 → sidebar item       (seed=false, brand=false, main=false)
//	Tab #2 → navbar brand       (brand=true)
//	Tab #3 → navbar link        (brand=false, main=false)
//	Tab #4 → main               (main=true)
//
// Any other ordering of regions would move brand and/or main to
// different positions in the sequence.
func TestShellSidebarHeaderMainTabTraversal(t *testing.T) {
	shaper := defaultShaper(t)
	var mainClick widget.Clickable
	var brandClick widget.Clickable
	var seedClick widget.Clickable

	mainWidget := func(gtx layout.Context) layout.Dimensions {
		return mainClick.Layout(gtx, fillRect(color.NRGBA{R: 0, G: 200, B: 0, A: 255}))
	}
	brandWidget := func(gtx layout.Context) layout.Dimensions {
		return brandClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(40, 20)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 80, G: 80, B: 200, A: 255}, clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: size}
		})
	}

	props := shell.Props{
		Layout: shell.SidebarHeaderMain,
		Sidebar: sidebar.Sidebar(rx.Of(theme.Default()), sidebar.Props{
			Items: []sidebar.Item{
				{Icon: testIcon(), Label: sidebarItemLabels[0], OnClick: func(_ layout.Context) {}},
			},
			Collapsed: rx.Of(false),
			Shaper:    shaper,
		}),
		Navbar: navbar.Props{
			Brand: brandWidget,
			Links: []navbar.Link{
				{Label: navLinkLabels[0], OnClick: func(_ layout.Context) {}},
			},
			Shaper: shaper,
		},
		Main: mainWidget,
	}
	body := shell.Shell(rx.Of(theme.Default()), props)
	bodyW := liveWidget(t, body)

	// Compose: a seed clickable (zero-size visual) then the shell. The
	// seed is a focus anchor whose position in the op-stream is before
	// the shell, so MoveFocus(Forward) from the seed enters the shell
	// at its first focusable.
	composed := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return seedClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(1, 1)}
				})
			}),
			layout.Flexed(1, bodyW),
		)
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)

	// Two warm-up frames for stable hit-test data.
	driveFrame(composed, ops, r, tabSize)
	driveFrame(composed, ops, r, tabSize)

	// Drain any synthetic focus events on the seed so the router retains
	// focus when explicitly set, matching the FocusGroup idiom used in
	// the navbar test.
	drainFocus := func() {
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(tabSize),
			Ops:         ops,
			Source:      r.Source(),
		}
		for _, tag := range []any{&seedClick, &brandClick, &mainClick} {
			for {
				if _, ok := gtx.Event(key.FocusFilter{Target: tag}); !ok {
					break
				}
			}
		}
	}
	drainFocus()

	// Anchor focus at the seed.
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(tabSize),
		Ops:         ops,
		Source:      r.Source(),
	}
	gtx.Execute(key.FocusCmd{Tag: &seedClick})
	driveFrame(composed, ops, r, tabSize)

	check := func(stage string, wantSeed, wantBrand, wantMain bool) {
		t.Helper()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(tabSize),
			Ops:         ops,
			Source:      r.Source(),
		}
		gotSeed := gtx.Focused(&seedClick)
		gotBrand := gtx.Focused(&brandClick)
		gotMain := gtx.Focused(&mainClick)
		if gotSeed != wantSeed || gotBrand != wantBrand || gotMain != wantMain {
			t.Errorf("%s: focused(seed)=%v brand=%v main=%v; want seed=%v brand=%v main=%v",
				stage, gotSeed, gotBrand, gotMain, wantSeed, wantBrand, wantMain)
		}
	}

	check("after Focus(seed)", true, false, false)

	// Tab #1 → into the sidebar's first item.
	r.MoveFocus(key.FocusForward)
	driveFrame(composed, ops, r, tabSize)
	check("Tab #1 (→ sidebar item)", false, false, false)

	// Tab #2 → into the navbar's brand (externally observable). If the
	// shell composed navbar before sidebar, this stop would already
	// have been Tab #1.
	r.MoveFocus(key.FocusForward)
	driveFrame(composed, ops, r, tabSize)
	check("Tab #2 (→ navbar brand)", false, true, false)

	// Tab #3 → into the navbar's first link.
	r.MoveFocus(key.FocusForward)
	driveFrame(composed, ops, r, tabSize)
	check("Tab #3 (→ navbar link)", false, false, false)

	// Tab #4 → into main.
	r.MoveFocus(key.FocusForward)
	driveFrame(composed, ops, r, tabSize)
	check("Tab #4 (→ main)", false, false, true)
}

// TestShellCustomSidebarWidget confirms that a caller-supplied
// rx.Observable[layout.Widget] (not sidebar.Sidebar) works as the Sidebar
// slot and that the op-stream order is sidebar → navbar → main, preserving
// Tab focus traversal. Structure mirrors TestShellSidebarHeaderMainTabTraversal;
// the only delta is Props.Sidebar being a plain rx.Of widget instead of
// sidebar.Sidebar.
func TestShellCustomSidebarWidget(t *testing.T) {
	shaper := defaultShaper(t)
	var mainClick widget.Clickable
	var brandClick widget.Clickable
	var seedClick widget.Clickable
	var customSBClick widget.Clickable

	customSBWidget := func(gtx layout.Context) layout.Dimensions {
		return customSBClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(40, 256)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 60, G: 60, B: 60, A: 255}, clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: size}
		})
	}

	mainWidget := func(gtx layout.Context) layout.Dimensions {
		return mainClick.Layout(gtx, fillRect(color.NRGBA{R: 0, G: 200, B: 0, A: 255}))
	}
	brandWidget := func(gtx layout.Context) layout.Dimensions {
		return brandClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(40, 20)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 80, G: 80, B: 200, A: 255}, clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: size}
		})
	}

	props := shell.Props{
		Layout:  shell.SidebarHeaderMain,
		Sidebar: rx.Of[layout.Widget](customSBWidget),
		Navbar: navbar.Props{
			Brand: brandWidget,
			Links: []navbar.Link{
				{Label: navLinkLabels[0], OnClick: func(_ layout.Context) {}},
			},
			Shaper: shaper,
		},
		Main: mainWidget,
	}
	body := shell.Shell(rx.Of(theme.Default()), props)
	bodyW := liveWidget(t, body)

	composed := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return seedClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(1, 1)}
				})
			}),
			layout.Flexed(1, bodyW),
		)
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)

	driveFrame(composed, ops, r, tabSize)
	driveFrame(composed, ops, r, tabSize)

	drainFocus := func() {
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(tabSize),
			Ops:         ops,
			Source:      r.Source(),
		}
		for _, tag := range []any{&seedClick, &customSBClick, &brandClick, &mainClick} {
			for {
				if _, ok := gtx.Event(key.FocusFilter{Target: tag}); !ok {
					break
				}
			}
		}
	}
	drainFocus()

	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(tabSize),
		Ops:         ops,
		Source:      r.Source(),
	}
	gtx.Execute(key.FocusCmd{Tag: &seedClick})
	driveFrame(composed, ops, r, tabSize)

	check := func(stage string, wantSeed, wantCustomSB, wantBrand, wantMain bool) {
		t.Helper()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(tabSize),
			Ops:         ops,
			Source:      r.Source(),
		}
		gotSeed := gtx.Focused(&seedClick)
		gotCustomSB := gtx.Focused(&customSBClick)
		gotBrand := gtx.Focused(&brandClick)
		gotMain := gtx.Focused(&mainClick)
		if gotSeed != wantSeed || gotCustomSB != wantCustomSB || gotBrand != wantBrand || gotMain != wantMain {
			t.Errorf("%s: seed=%v customSB=%v brand=%v main=%v; want seed=%v customSB=%v brand=%v main=%v",
				stage, gotSeed, gotCustomSB, gotBrand, gotMain, wantSeed, wantCustomSB, wantBrand, wantMain)
		}
	}

	check("after Focus(seed)", true, false, false, false)

	// Tab #1 → custom sidebar widget (rendered first in Flex op-stream).
	r.MoveFocus(key.FocusForward)
	driveFrame(composed, ops, r, tabSize)
	check("Tab #1 (→ custom sidebar)", false, true, false, false)

	// Tab #2 → navbar brand.
	r.MoveFocus(key.FocusForward)
	driveFrame(composed, ops, r, tabSize)
	check("Tab #2 (→ navbar brand)", false, false, true, false)

	// Tab #3 → navbar link.
	r.MoveFocus(key.FocusForward)
	driveFrame(composed, ops, r, tabSize)
	check("Tab #3 (→ navbar link)", false, false, false, false)

	// Tab #4 → main.
	r.MoveFocus(key.FocusForward)
	driveFrame(composed, ops, r, tabSize)
	check("Tab #4 (→ main)", false, false, false, true)
}
