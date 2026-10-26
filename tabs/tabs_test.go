package tabs_test

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

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/tabs"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

const (
	frameW, frameH = 240, 128
)

var frameSize = image.Pt(frameW, frameH)

// defaultShaper returns the shaper every golden here draws with: the default
// typography's faces pinned, system fonts off, so the stored images are the
// same on every machine. A golden test pins its faces with
// DeterministicShaper; application code takes the fallback Shaper.
func defaultShaper(t *testing.T) *text.Shaper {
	t.Helper()
	return tokens.DefaultTypography.DeterministicShaper()
}

// contentRect returns a layout.Widget that fills its constraints with a
// fixed colour. A per-tab distinct colour is used so swapping the
// selected index produces a visible diff in the tab's content region of
// each golden.
func contentRect(c color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

func scene(w layout.Widget, bg color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// tabTitles names the three tabs in document order. Latin text in Roboto
// rasterises identically on every machine with DeterministicShaper, so this
// stays ASCII — no symbol reaches a stored image.
//
// They are short on purpose. A tab cell is Rigid and sized to its title plus
// 2×S3 of padding, so three long titles would run off the 240 px frame; these
// three leave the strip comfortably inside it.
var tabTitles = []string{"Preview", "Code", "Notes"}

// contentPlane is the fill every specimen here is drawn on: the platform's
// content plane, the fill of a document column. The strip has no fill of its
// own, so the caller passes it.
func contentPlane(c tokens.PlatformColors) color.NRGBA { return c.ControlBackground }

// threeTabs returns a deterministic three-tab fixture. The per-tab content
// colours are unrelated to the theme so the same fixture is reused for the
// light and dark goldens; the titles carry the typography.
func threeTabs() []tabs.Tab {
	return []tabs.Tab{
		{Title: tabTitles[0], Content: contentRect(color.NRGBA{R: 0xff, G: 0x40, B: 0x40, A: 0xff})},
		{Title: tabTitles[1], Content: contentRect(color.NRGBA{R: 0x40, G: 0xc0, B: 0x60, A: 0xff})},
		{Title: tabTitles[2], Content: contentRect(color.NRGBA{R: 0x40, G: 0x70, B: 0xff, A: 0xff})},
	}
}

func singleTab() []tabs.Tab {
	return []tabs.Tab{
		{Title: tabTitles[0], Content: contentRect(color.NRGBA{R: 0xff, G: 0x40, B: 0x40, A: 0xff})},
	}
}

// TestTabsGolden records or diffs the three Measurable goldens.
func TestTabsGolden(t *testing.T) {
	shaper := defaultShaper(t)
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	darkBG := color.NRGBA{R: 20, G: 20, B: 20, A: 255}

	cases := []struct {
		name     string
		tabs     []tabs.Tab
		selected int
		colors   tokens.PlatformColors
		bg       color.NRGBA
	}{
		{"light-three-tabs-first-selected", threeTabs(), 0, tokens.PlatformLight, lightBG},
		{"dark-three-tabs-second-selected", threeTabs(), 1, tokens.PlatformDark, darkBG},
		{"light-single-tab", singleTab(), 0, tokens.PlatformLight, lightBG},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			props := tabs.Props{Tabs: tc.tabs, Shaper: shaper, Fill: contentPlane}
			w := tabs.Render(shaper, props, tc.selected, tc.colors, tokens.Spacing, tokens.DefaultTypography.SystemFont, tokens.Comfortable)
			golden.Render(t, tc.name, frameSize, scene(w, tc.bg))
		})
	}
}

// TestTabsSelectionUnderlineIsVisible guards the visual contract that
// the selected tab adds selection-coloured pixels to the strip relative
// to an unselected (out-of-range index) baseline.
func TestTabsSelectionUnderlineIsVisible(t *testing.T) {
	shaper := defaultShaper(t)
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}

	render := func(selected int) *image.RGBA {
		props := tabs.Props{Tabs: threeTabs(), Shaper: shaper, Fill: contentPlane}
		w := tabs.Render(shaper, props, selected, tokens.PlatformLight, tokens.Spacing, tokens.DefaultTypography.SystemFont, tokens.Comfortable)
		return golden.Capture(t, frameSize, scene(w, bg))
	}

	none := render(-1)
	first := render(0)
	if n := golden.PixelDiff(none, first); n == 0 {
		t.Errorf("selected and unselected render identically; expected the selection-coloured underline")
	}
}

// TestTheStripStandsOnTheFillItIsGiven reads the pattern's contract off
// sampled pixels: the strip has no fill of its own, so its band is the fill
// the caller passed and its trim is the platform's separator flattened
// onto that same fill. Two fills are passed per scheme — the content plane
// and the chrome material, which differ in both — so a band that answered
// with a constant would fail one of them.
func TestTheStripStandsOnTheFillItIsGiven(t *testing.T) {
	shaper := defaultShaper(t)
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}

	// An out-of-range selection draws no content and no underline, so the
	// strip's rows right of the last cell are bare band.
	stripH := int(tokens.Comfortable.ControlHeight)
	for _, sc := range []struct {
		name   string
		colors tokens.PlatformColors
	}{
		{"light", tokens.PlatformLight},
		{"dark", tokens.PlatformDark},
	} {
		for _, f := range []struct {
			name string
			pick func(tokens.PlatformColors) color.NRGBA
		}{
			{"content", func(c tokens.PlatformColors) color.NRGBA { return c.ControlBackground }},
			{"chrome", func(c tokens.PlatformColors) color.NRGBA { return c.SidebarMaterial }},
		} {
			t.Run(sc.name+"-"+f.name, func(t *testing.T) {
				props := tabs.Props{Tabs: threeTabs(), Shaper: shaper, Fill: f.pick}
				w := tabs.Render(shaper, props, -1, sc.colors, tokens.Spacing,
					tokens.DefaultTypography.SystemFont, tokens.Comfortable)
				img := golden.Capture(t, frameSize, scene(w, bg))
				at := func(x, y int) [3]uint8 {
					off := img.PixOffset(x, y)
					return [3]uint8{img.Pix[off], img.Pix[off+1], img.Pix[off+2]}
				}
				rgb := func(c color.NRGBA) [3]uint8 { return [3]uint8{c.R, c.G, c.B} }

				fill := f.pick(sc.colors)
				if got, want := at(frameSize.X-1, stripH/2), rgb(fill); got != want {
					t.Errorf("the strip's band is %v, want the fill it was given %v", got, want)
				}
				trim := vgcolor.Flatten(sc.colors.Separator, fill)
				if got, want := at(frameSize.X-1, stripH-1), rgb(trim); got != want {
					t.Errorf("the strip's foot is %v, want the separator flattened on that fill %v", got, want)
				}
			})
		}
	}
}

// TestTheTabsContentIsDrawnOnTheStripsFill reads the active tab's content
// region off sampled pixels for a passed fill that differs from the content
// plane in both schemes: the region is that fill, the same as the strip's
// band. The active tab's content paints nothing and the scene under the
// pattern is a grey neither scheme uses, so the region shows only what the
// pattern paints.
func TestTheTabsContentIsDrawnOnTheStripsFill(t *testing.T) {
	shaper := defaultShaper(t)
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	chrome := func(c tokens.PlatformColors) color.NRGBA { return c.SidebarMaterial }
	stripH := int(tokens.Comfortable.ControlHeight)
	for _, sc := range []struct {
		name   string
		colors tokens.PlatformColors
	}{
		{"light", tokens.PlatformLight},
		{"dark", tokens.PlatformDark},
	} {
		t.Run(sc.name, func(t *testing.T) {
			fill := chrome(sc.colors)
			if plane := contentPlane(sc.colors); fill == plane {
				t.Fatalf("the passed fill %v equals the content plane; the test needs them to differ", fill)
			}
			empty := func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			tt := []tabs.Tab{{Title: tabTitles[0], Content: empty}, {Title: tabTitles[1]}}
			props := tabs.Props{Tabs: tt, Shaper: shaper, Fill: chrome}
			w := tabs.Render(shaper, props, 0, sc.colors, tokens.Spacing,
				tokens.DefaultTypography.SystemFont, tokens.Comfortable)
			img := golden.Capture(t, frameSize, scene(w, bg))
			at := func(x, y int) [3]uint8 {
				off := img.PixOffset(x, y)
				return [3]uint8{img.Pix[off], img.Pix[off+1], img.Pix[off+2]}
			}
			band := at(frameSize.X-1, stripH/2)
			if want := [3]uint8{fill.R, fill.G, fill.B}; band != want {
				t.Errorf("the strip's band is %v, want the passed fill %v", band, want)
			}
			for _, p := range []image.Point{
				{0, stripH}, {frameSize.X / 2, (stripH + frameSize.Y) / 2}, {frameSize.X - 1, frameSize.Y - 1},
			} {
				if got := at(p.X, p.Y); got != band {
					t.Errorf("the tab's content at %v is %v, want the strip's fill %v", p, got, band)
				}
			}
		})
	}
}

// ---- Interaction tests ----

func liveWidget(t *testing.T, obs rx.Observable[layout.Widget]) layout.Widget {
	t.Helper()
	var w layout.Widget
	if err := obs.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("Tabs subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("Tabs did not emit an initial layout.Widget")
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

// TestTabsArrowAndHomeEndWrapAndFocus drives the WAI-ARIA tab pattern
// end-to-end. A tab cell is its title plus 2×S3 = 24 px of horizontal
// padding, never narrower than that padding alone, and stripH = 36 px (the
// Comfortable control height; PxPerDp = 1). Tab 0 is "Preview", so it
// is wider than the 24 px minimum and starts at x = 0 — a pointer click at
// (12, 20) lands squarely inside it whatever the title.
//
// Focus-follows-selection is verified using the "Enter trick": each
// arrow / Home / End press is followed by a Press+Release of NameReturn
// on the now-focused tab's widget.Clickable. The OnSelect callback fires
// twice — once for the navigation key (target index) and again for
// Enter (same target). If focus had not moved with selection, Enter
// would re-fire the previous tab's index instead, and the sequence
// would diverge from the expected list below.
func TestTabsArrowAndHomeEndWrapAndFocus(t *testing.T) {
	var calls []int
	props := tabs.Props{
		Tabs:     threeTabs(),
		Selected: rx.Of(0),
		OnSelect: func(_ layout.Context, idx int) { calls = append(calls, idx) },
		Shaper:   defaultShaper(t),
	}
	w := liveWidget(t, tabs.Tabs(rx.Of(theme.Default()), props))

	r := new(gioinput.Router)
	ops := new(op.Ops)
	// Two warm-up frames so the router has stable hit-test data for the
	// tab cell clip areas before pointer events are queued.
	driveFrame(w, ops, r, frameSize)
	driveFrame(w, ops, r, frameSize)

	// Click tab 0 → OnSelect(0) and focus moves to tab 0.
	hit := f32.Pt(12, 20)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: hit, Source: pointer.Touch},
		pointer.Event{Kind: pointer.Release, Position: hit, Source: pointer.Touch},
	)
	driveFrame(w, ops, r, frameSize)

	// pressKey sends a navigation key, drives a frame so the FocusCmd is
	// applied, then sends Enter Press+Release on the now-focused tab and
	// drives another frame so widget.Clickable.Clicked observes the
	// matched key pair. Two OnSelect calls per step: navigation + Enter.
	pressKey := func(name key.Name) {
		r.Queue(key.Event{Name: name, State: key.Press})
		driveFrame(w, ops, r, frameSize)
		r.Queue(
			key.Event{Name: key.NameReturn, State: key.Press},
			key.Event{Name: key.NameReturn, State: key.Release},
		)
		driveFrame(w, ops, r, frameSize)
	}

	pressKey(key.NameRightArrow) // → tab 1, Enter on tab 1
	pressKey(key.NameRightArrow) // → tab 2, Enter on tab 2
	pressKey(key.NameRightArrow) // wrap → tab 0, Enter on tab 0
	pressKey(key.NameLeftArrow)  // wrap → tab 2, Enter on tab 2
	pressKey(key.NameHome)       // → tab 0, Enter on tab 0
	pressKey(key.NameEnd)        // → tab 2, Enter on tab 2

	want := []int{
		0,    // initial click
		1, 1, // Right + Enter
		2, 2, // Right + Enter
		0, 0, // Right (wrap) + Enter
		2, 2, // Left (wrap) + Enter
		0, 0, // Home + Enter
		2, 2, // End + Enter
	}
	if !equalInts(calls, want) {
		t.Fatalf("OnSelect call sequence:\n got  %v\n want %v", calls, want)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// densityTheme returns a theme whose density is d, with sharp corners
// for golden determinism — the injection idiom, mirroring components'
// density tests.
func densityTheme(d tokens.Density) theme.Theme {
	th := theme.Default()
	th.Density = rx.Of(d)
	th.Radius = rx.Of(tokens.RadiusScale{})
	return th
}

// TestTabsCompactGolden records or diffs the compact-density golden
// through the LIVE pipeline (the static Render path is frozen at
// tokens.Comfortable): the strip height drops from 36 to 28 dp
// (ControlHeight) and the tab's content region grows by the difference.
func TestTabsCompactGolden(t *testing.T) {
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	props := tabs.Props{Tabs: threeTabs(), Selected: rx.Of(0), Shaper: defaultShaper(t), Fill: contentPlane}
	w := liveWidget(t, tabs.Tabs(rx.Of(densityTheme(tokens.Compact)), props))
	golden.Render(t, "light-compact-three-tabs-first-selected", frameSize, scene(w, lightBG))
}
