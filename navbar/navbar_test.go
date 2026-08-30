package navbar_test

import (
	"context"
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
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
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

const (
	canvasW, canvasH = 480, 64
)

var canvasSize = image.Pt(canvasW, canvasH)

// defaultShaper returns the shaper every golden here draws with: the default
// typography's faces pinned, system fonts off, so the stored images are the
// same on every machine. A golden test pins its faces with
// DeterministicShaper; application code takes the fallback Shaper.
func defaultShaper(t *testing.T) *text.Shaper {
	t.Helper()
	return tokens.DefaultTypography.DeterministicShaper()
}

// scene renders w into a canvas-sized constraint over a flat background.
func scene(w layout.Widget, bgColor color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// linkLabels names the two links in document order. Latin text in Roboto
// rasterises identically on every machine under DeterministicShaper; ASCII
// only, so no symbol reaches a stored image.
var linkLabels = [2]string{"Docs", "Components"}

// links returns the two-link fixture, optionally marking one Active.
// activeIdx < 0 means no link is active.
func links(activeIdx int) []navbar.Link {
	out := make([]navbar.Link, len(linkLabels))
	for i, l := range linkLabels {
		out[i] = navbar.Link{Label: l, Active: i == activeIdx}
	}
	return out
}

// TestNavbarGolden records or diffs the three Measurable goldens. Each link
// cell is its label plus (S3, S2) padding, so the Active link's Primary
// underline runs the width of the label.
func TestNavbarGolden(t *testing.T) {
	shaper := defaultShaper(t)
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	darkBG := color.NRGBA{R: 20, G: 20, B: 20, A: 255}

	defaultLinks := links(-1)
	activeSecond := links(1)

	cases := []struct {
		name   string
		links  []navbar.Link
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{"light-default", defaultLinks, tokens.DefaultLight, lightBG},
		{"dark-default", defaultLinks, tokens.DefaultDark, darkBG},
		{"light-active-second-link", activeSecond, tokens.DefaultLight, lightBG},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			props := navbar.Props{Links: tc.links, Shaper: shaper}
			w := navbar.Render(shaper, props, tc.colors, tokens.Spacing, tokens.DefaultTypography.LabelLarge, tokens.Comfortable)
			golden.Render(t, tc.name, canvasSize, scene(w, tc.bg))
		})
	}
}

// TestNavbarActiveVsDefaultDiffer guards the visual contract that an
// Active link adds Primary-coloured pixels in the link row.
func TestNavbarActiveVsDefaultDiffer(t *testing.T) {
	shaper := defaultShaper(t)
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}

	render := func(links []navbar.Link) *image.RGBA {
		props := navbar.Props{Links: links, Shaper: shaper}
		w := navbar.Render(shaper, props, tokens.DefaultLight, tokens.Spacing, tokens.DefaultTypography.LabelLarge, tokens.Comfortable)
		return golden.Capture(t, canvasSize, scene(w, bg))
	}

	def := render(links(-1))
	act := render(links(1))
	if n := golden.PixelDiff(def, act); n == 0 {
		t.Errorf("active and default render identically; expected Primary underline pixels")
	}
}

// ---- Interaction tests ----

// fillRect is a sharp-edged solid widget with a fixed size.
func fillRect(c color.NRGBA, w, h int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(w, h)
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	}
}

// liveWidget subscribes to nb, drains the trampoline scheduler, and
// returns the latest emitted layout.Widget. State referenced by the
// widget closure remains valid for the test's lifetime because it is
// captured by the rx.Defer scope.
func liveWidget(t *testing.T, nb rx.Observable[layout.Widget]) layout.Widget {
	t.Helper()
	var w layout.Widget
	if err := nb.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("Navbar subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("Navbar did not emit an initial widget")
	}
	return w
}

// driveFrame lays out w against ops + router and returns the dims.
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

// TestNavbarTabTraversal verifies Tab cycles focus through
// brand → links → actions in document order, and Shift+Tab reverses.
// Brand and action contribute focus stops via outer-test Clickables;
// the two link stops are owned by the navbar.
func TestNavbarTabTraversal(t *testing.T) {
	var brandClick, actionClick widget.Clickable
	brand := func(gtx layout.Context) layout.Dimensions {
		return brandClick.Layout(gtx, fillRect(color.NRGBA{R: 80, G: 80, B: 200, A: 255}, 40, 20))
	}
	action := func(gtx layout.Context) layout.Dimensions {
		return actionClick.Layout(gtx, fillRect(color.NRGBA{R: 200, G: 80, B: 80, A: 255}, 40, 20))
	}

	props := navbar.Props{
		Brand: brand,
		Links: []navbar.Link{
			{Label: linkLabels[0], OnClick: func(_ layout.Context) {}},
			{Label: linkLabels[1], OnClick: func(_ layout.Context) {}},
		},
		Actions: []layout.Widget{action},
		Shaper:  defaultShaper(t),
	}
	w := liveWidget(t, navbar.Navbar(rx.Of(theme.Default()), props))

	r := new(gioinput.Router)
	ops := new(op.Ops)

	// Frame 0: register tags.
	driveFrame(w, ops, r, canvasSize)

	// Drain any synthetic focus events for the externally-owned tags so
	// the router retains focus when set, matching the FocusGroup idiom.
	drainFocus := func() {
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(canvasSize),
			Ops:         ops,
			Source:      r.Source(),
		}
		for _, tag := range []any{&brandClick, &actionClick} {
			for {
				if _, ok := gtx.Event(key.FocusFilter{Target: tag}); !ok {
					break
				}
			}
		}
	}
	drainFocus()

	// Focus the brand explicitly.
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(canvasSize),
		Ops:         ops,
		Source:      r.Source(),
	}
	gtx.Execute(key.FocusCmd{Tag: &brandClick})
	driveFrame(w, ops, r, canvasSize)

	check := func(stage string, wantBrand, wantAction bool) {
		t.Helper()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(canvasSize),
			Ops:         ops,
			Source:      r.Source(),
		}
		gotBrand := gtx.Focused(&brandClick)
		gotAction := gtx.Focused(&actionClick)
		if gotBrand != wantBrand || gotAction != wantAction {
			t.Errorf("%s: focused(brand)=%v action=%v; want brand=%v action=%v",
				stage, gotBrand, gotAction, wantBrand, wantAction)
		}
	}

	check("after Focus(brand)", true, false)

	// Tab → expected stop is link 0 (neither brand nor action).
	r.MoveFocus(key.FocusForward)
	driveFrame(w, ops, r, canvasSize)
	check("Tab #1 (→ link 0)", false, false)

	// Tab → expected stop is link 1.
	r.MoveFocus(key.FocusForward)
	driveFrame(w, ops, r, canvasSize)
	check("Tab #2 (→ link 1)", false, false)

	// Tab → expected stop is action.
	r.MoveFocus(key.FocusForward)
	driveFrame(w, ops, r, canvasSize)
	check("Tab #3 (→ action)", false, true)

	// Now reverse the traversal. Shift+Tab from action: back to link 1.
	r.MoveFocus(key.FocusBackward)
	driveFrame(w, ops, r, canvasSize)
	check("Shift+Tab #1 (→ link 1)", false, false)

	// Shift+Tab → link 0.
	r.MoveFocus(key.FocusBackward)
	driveFrame(w, ops, r, canvasSize)
	check("Shift+Tab #2 (→ link 0)", false, false)

	// Shift+Tab → brand.
	r.MoveFocus(key.FocusBackward)
	driveFrame(w, ops, r, canvasSize)
	check("Shift+Tab #3 (→ brand)", true, false)
}

// TestNavbarLinkClickFiresOnClick verifies clicking a link invokes its
// OnClick callback. With PxPerDp=1, canvas 480×64, no brand, no actions, two
// links: each cell is its label plus (S3, Density.PaddingY) padding and an
// underline, separated by an S2 spacer, and the row is centred at canvas-mid.
// "Docs" and "Components" measure 57 and 105 px, so the row is 57+8+105 = 170
// wide and starts at x = 155; link 0 occupies x in [155, 212], y in [15, 49].
// A press/release at (180, 32) lands squarely inside it, clear of link 1.
func TestNavbarLinkClickFiresOnClick(t *testing.T) {
	var fired0, fired1 int
	props := navbar.Props{
		Links: []navbar.Link{
			{Label: linkLabels[0], OnClick: func(_ layout.Context) { fired0++ }},
			{Label: linkLabels[1], OnClick: func(_ layout.Context) { fired1++ }},
		},
		Shaper: defaultShaper(t),
	}
	w := liveWidget(t, navbar.Navbar(rx.Of(theme.Default()), props))

	r := new(gioinput.Router)
	ops := new(op.Ops)

	// Two warm-up frames so the router has stable hit-test data for the
	// link clip areas before pointer events are queued.
	driveFrame(w, ops, r, canvasSize)
	driveFrame(w, ops, r, canvasSize)

	hit := f32.Pt(180, 32)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: hit, Source: pointer.Touch},
		pointer.Event{Kind: pointer.Release, Position: hit, Source: pointer.Touch},
	)
	driveFrame(w, ops, r, canvasSize)

	if fired0 != 1 {
		t.Errorf("link 0 OnClick call count = %d, want 1", fired0)
	}
	if fired1 != 0 {
		t.Errorf("link 1 OnClick spuriously fired %d time(s)", fired1)
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

// barHeight is the height the navbar actually draws at density d: a link
// cell is its label's line box plus the density's vertical padding above
// and below plus the Active underline, and the bar insets that row by the
// same PaddingY again.
//
// It is computed rather than taken from Density.ControlHeight + 2·PaddingY:
// that pin does not budget for the link cell's 2 dp Active underline, so a
// window sized from it alone can squeeze the bottom padding to nothing and
// leave the underline flush against the last pixel row.
func barHeight(d tokens.Density, style tokens.TextStyle) int {
	cell := int(style.LineHeight) + 2*int(d.PaddingY) + navbarUnderlineDp
	return cell + 2*int(d.PaddingY)
}

// navbarUnderlineDp mirrors the unexported underlineDp in the navbar
// package: the thickness of the Active link's Primary indicator.
const navbarUnderlineDp = 2

// TestNavbarCompactGolden records or diffs the compact-density golden
// through the LIVE pipeline (the static Render path is frozen at
// tokens.Comfortable): the bar's vertical inset and the link padding
// drop to the Compact PaddingY (6 dp). The canvas is [barHeight] at
// Compact — 46 dp, not the shell's 40 dp pin; see barHeight for why.
func TestNavbarCompactGolden(t *testing.T) {
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	props := navbar.Props{
		Links: []navbar.Link{
			{Label: linkLabels[0], Active: true, OnClick: func(_ layout.Context) {}},
			{Label: linkLabels[1], OnClick: func(_ layout.Context) {}},
		},
		Shaper: defaultShaper(t),
	}
	w := liveWidget(t, navbar.Navbar(rx.Of(densityTheme(tokens.Compact)), props))
	h := barHeight(tokens.Compact, tokens.DefaultTypography.LabelLarge)
	golden.Render(t, "light-compact", image.Pt(canvasW, h), scene(w, lightBG))
}

// inkBand reports the first and last row of img holding the colour c, or
// (-1, -1) when it holds none.
func inkBand(img *image.RGBA, c color.NRGBA) (int, int) {
	b := img.Bounds()
	first, last := -1, -1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == c.R && uint8(g>>8) == c.G && uint8(bl>>8) == c.B {
				if first < 0 {
					first = y
				}
				last = y
				break
			}
		}
	}
	return first, last
}

// TestNavbarPutsEverySlotOnOneCentreLine is the alignment the bar is read on,
// measured: brand, links and actions each drawn so its own middle lands on the
// bar's middle, at every height the bar is given.
//
// The brand fixture is the shape that breaks a naive layout: a widget that
// honours a minimum height it was never meant to fill — a column of text
// laid out in a vertical Flex is the everyday one — comes back the full
// height of the row and inks only the top of it, which reads as inked well
// above the links beside it unless each slot is measured with no cross
// minimum at all.
//
// Rows are compared doubled, so that a middle landing between two pixel rows
// is one integer rather than a half that has to be tolerated: a slot of even
// height on a bar of even height centres on a boundary, and the bar's own
// middle is the boundary h/2.
func TestNavbarPutsEverySlotOnOneCentreLine(t *testing.T) {
	shaper := defaultShaper(t)
	style := tokens.DefaultTypography.LabelLarge
	d := tokens.Comfortable

	brandInk := color.NRGBA{R: 0, G: 0, B: 255, A: 255}
	actionInk := color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	const brandInkH, actionH = 10, 20
	// cellH is a link cell as linkWidget builds one: the label's line box,
	// the density's padding above and below, and the Active underline.
	cellH := int(style.LineHeight) + 2*int(d.PaddingY) + navbarUnderlineDp

	// A brand that fills the height it is offered and inks only the top of
	// it, which is what a run of text in a column does.
	brand := func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, brandInk, clip.Rect{Max: image.Pt(60, brandInkH)}.Op())
		return layout.Dimensions{Size: image.Pt(60, max(gtx.Constraints.Min.Y, brandInkH))}
	}

	// The bar at its own height, and at two heights a caller might pin it to.
	for _, h := range []int{barHeight(d, style), 64, 80} {
		props := navbar.Props{
			Brand:   brand,
			Links:   []navbar.Link{{Label: linkLabels[0], Active: true}, {Label: linkLabels[1]}},
			Actions: []layout.Widget{fillRect(actionInk, 30, actionH)},
			Shaper:  shaper,
		}
		w := navbar.Render(shaper, props, tokens.DefaultLight, tokens.Spacing, style, d)
		img := golden.Capture(t, image.Pt(canvasW, h), scene(w, color.NRGBA{R: 240, G: 240, B: 240, A: 255}))

		brandTop, brandBottom := inkBand(img, brandInk)
		actionTop, actionBottom := inkBand(img, actionInk)
		_, underlineBottom := inkBand(img, tokens.DefaultLight.Primary)
		if brandTop < 0 || actionTop < 0 || underlineBottom < 0 {
			t.Fatalf("bar %d px: brand, action or underline did not draw (%d, %d, %d); this proves nothing",
				h, brandTop, actionTop, underlineBottom)
		}
		// The link cell's top is read off the underline, which is flush with
		// its bottom edge — the cell itself draws no border to measure.
		cellTop := underlineBottom + 1 - cellH
		for _, c := range []struct {
			what     string
			centre2  int
			inkFirst int
			inkLast  int
		}{
			{"brand", brandTop + brandBottom, brandTop, brandBottom},
			{"links", 2*cellTop + cellH - 1, cellTop, cellTop + cellH - 1},
			{"actions", actionTop + actionBottom, actionTop, actionBottom},
		} {
			if c.centre2 != h-1 {
				t.Errorf("bar %d px: the %s slot occupies rows %d..%d and is centred on %.1f, want the bar's own middle %.1f",
					h, c.what, c.inkFirst, c.inkLast, float64(c.centre2)/2, float64(h-1)/2)
			}
		}
	}
}

// TestNavbarKeepsItsBottomPadding asserts that in a canvas of [barHeight]
// the Active underline — the lowest thing the bar draws — clears the
// bottom PaddingY, so the bar keeps the breathing room its own inset asks
// for and is nowhere near the edge it would be clipped at.
//
// The navbar fills its constraints, so its reported Dimensions can never
// report an overflow; only the pixels can. This reads them.
func TestNavbarKeepsItsBottomPadding(t *testing.T) {
	style := tokens.DefaultTypography.LabelLarge
	primary := tokens.DefaultLight.Primary

	for _, d := range []tokens.Density{tokens.Comfortable, tokens.Compact} {
		props := navbar.Props{
			Links:  []navbar.Link{{Label: linkLabels[0], Active: true, OnClick: func(_ layout.Context) {}}},
			Shaper: defaultShaper(t),
		}
		w := liveWidget(t, navbar.Navbar(rx.Of(densityTheme(d)), props))
		h := barHeight(d, style)
		img := golden.Capture(t, image.Pt(canvasW, h), scene(w, color.NRGBA{R: 240, G: 240, B: 240, A: 255}))

		lowest := -1
		for y := 0; y < h; y++ {
			for x := 0; x < canvasW; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if uint8(r>>8) == primary.R && uint8(g>>8) == primary.G && uint8(b>>8) == primary.B {
					lowest = y
					break
				}
			}
		}
		if lowest < 0 {
			t.Fatalf("density %+v: no Primary pixel in the bar; the Active underline did not draw, so this proves nothing", d)
		}
		if want := h - int(d.PaddingY); lowest >= want {
			t.Errorf("density %+v: the underline reaches row %d of a %d px bar, inside the %d dp bottom padding",
				d, lowest, h, int(d.PaddingY))
		}
	}
}
