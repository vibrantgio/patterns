package pagination_test

import (
	"context"
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/pagination"
	tcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

const (
	canvasW, canvasH = 360, 48
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

// TestPaginationGolden records or diffs the three Measurable goldens.
//
// A zero radius scale (sharp corners) keeps the cell edges deterministic. The
// distinguishing signal across goldens is which cell carries the Primary fill:
// page-1-of-5 highlights the first cell, page-3-of-5 the third, and the light
// and dark variants flip the palette.
func TestPaginationGolden(t *testing.T) {
	shaper := defaultShaper(t)
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	darkBG := color.NRGBA{R: 20, G: 20, B: 20, A: 255}
	sharpRadius := tokens.RadiusScale{}

	cases := []struct {
		name      string
		page      int
		pageCount int
		colors    tokens.ColorTokens
		bg        color.NRGBA
	}{
		{"light-page-1-of-5", 1, 5, tokens.DefaultLight, lightBG},
		{"light-page-3-of-5", 3, 5, tokens.DefaultLight, lightBG},
		{"dark-page-3-of-5", 3, 5, tokens.DefaultDark, darkBG},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			props := pagination.Props{Page: tc.page, PageCount: tc.pageCount, Shaper: shaper}
			w := pagination.Render(shaper, props, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelLarge, tokens.Comfortable)
			golden.Render(t, tc.name, canvasSize, scene(w, tc.bg))
		})
	}
}

// TestPaginationCurrentPagePositionDiffers confirms that moving the active
// page shifts the Primary-coloured cell to a different x position. Guards
// against a regression in which all cells render with identical styling.
func TestPaginationCurrentPagePositionDiffers(t *testing.T) {
	shaper := defaultShaper(t)
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	sharpRadius := tokens.RadiusScale{}

	render := func(page int) *image.RGBA {
		props := pagination.Props{Page: page, PageCount: 5, Shaper: shaper}
		w := pagination.Render(shaper, props, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelLarge, tokens.Comfortable)
		return golden.Capture(t, canvasSize, scene(w, bg))
	}

	one := render(1)
	three := render(3)
	if n := golden.PixelDiff(one, three); n == 0 {
		t.Error("page-1-of-5 and page-3-of-5 render identically; expected the Primary-coloured cell to move")
	}
}

// TestPaginationLightDarkDiffer confirms swapping the colour token set
// changes the rendered output.
func TestPaginationLightDarkDiffer(t *testing.T) {
	shaper := defaultShaper(t)
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	sharpRadius := tokens.RadiusScale{}

	props := pagination.Props{Page: 3, PageCount: 5, Shaper: shaper}
	light := pagination.Render(shaper, props, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelLarge, tokens.Comfortable)
	dark := pagination.Render(shaper, props, tokens.DefaultDark, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelLarge, tokens.Comfortable)

	imgLight := golden.Capture(t, canvasSize, scene(light, bg))
	imgDark := golden.Capture(t, canvasSize, scene(dark, bg))
	if n := golden.PixelDiff(imgLight, imgDark); n == 0 {
		t.Error("light and dark render identically; expected token-pair colour differences")
	}
}

// TestTheCurrentPageWearsTheChosenItemStep confirms the cell for the page
// the reader is on fills from the Primary ramp's tinted step 300 — the same
// fill a sidebar's open row and a table's selected row take — and its digit
// is derived from that ramp's own step 700 rather than the theme's OnPrimary
// token, which is derived against the ramp's pin and does not clear WCAG AA
// over the tinted step. Both schemes are checked, and the resting cell
// beside the current one is checked too, since a mark says nothing if every
// cell wears it.
func TestTheCurrentPageWearsTheChosenItemStep(t *testing.T) {
	shaper := defaultShaper(t)
	sharpRadius := tokens.RadiusScale{}
	const page, pageCount = 3, 5

	// The cell's own geometry, derived rather than measured: the row is a
	// leading chevron, then one ControlHeight square per page, every pair
	// separated by an S2 gap, laid out middle-aligned in the canvas. Sampling
	// three pixels in from the cell's leading top corner clears both the
	// (sharp) corner and the centred digit.
	side := int(tokens.Comfortable.ControlHeight)
	gap := int(tokens.Spacing.S2)
	cellAt := func(n int) image.Point {
		x := side + gap + (n-1)*(side+gap)
		y := (canvasH - side) / 2
		return image.Pt(x+3, y+3)
	}

	for _, tc := range []struct {
		name string
		c    tokens.ColorTokens
		bg   color.NRGBA
	}{
		{"light", tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
		{"dark", tokens.DefaultDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			props := pagination.Props{Page: page, PageCount: pageCount, Shaper: shaper}
			w := pagination.Render(shaper, props, tc.c, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelLarge, tokens.Comfortable)
			img := golden.Capture(t, canvasSize, scene(w, tc.bg))

			at := func(p image.Point) color.NRGBA {
				r, g, b, _ := img.At(p.X, p.Y).RGBA()
				return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
			}

			tint := tc.c.Ramps.Primary.Step(300)
			if got := at(cellAt(page)); got != tint {
				t.Errorf("current page cell = %v, want the chosen-item step %v", got, tint)
			}
			if got := at(cellAt(page)); got == tc.c.Primary {
				t.Errorf("current page cell fills with the Primary pin %v; the pin inverts against the step between schemes", got)
			}
			if got, want := at(cellAt(page-1)), tc.c.Ramps.Neutral.Step(300); got != want {
				t.Errorf("resting page cell = %v, want the neutral fill %v", got, want)
			}

			// The digit's ink comes from the fill's own ramp; OnPrimary is
			// derived against the ramp's pin and does not clear WCAG AA over
			// the tinted step used here.
			ink := tc.c.Ramps.Primary.Step(700)
			if got := tcolor.ContrastRatio(ink, tint); got < aaBodyText {
				t.Errorf("current page digit %v over %v = %.2f:1, below WCAG AA body text %.1f:1", ink, tint, got, aaBodyText)
			}
			if got := tcolor.ContrastRatio(tc.c.OnPrimary, tint); got >= aaBodyText {
				t.Errorf("OnPrimary %v now reads %.2f:1 over %v; this test's premise has moved", tc.c.OnPrimary, got, tint)
			}

			// And it reads at the weight the cells beside it do, so the
			// current page is the coloured cell rather than the loud or the
			// faint one.
			resting := tcolor.ContrastRatio(tc.c.Ramps.Neutral.Step(700), tc.c.Ramps.Neutral.Step(300))
			current := tcolor.ContrastRatio(ink, tint)
			if d := current / resting; d < 0.75 || d > 1.35 {
				t.Errorf("current digit reads %.2f:1 against the resting cells' %.2f:1; one cell in the row is a different weight from the others", current, resting)
			}
		})
	}
}

// aaBodyText is WCAG 2.1 AA's contrast floor for body-sized text, which a
// page digit in the LabelLarge role is.
const aaBodyText = 4.5

// liveWidget subscribes to obs and returns its last emitted widget.
func liveWidget(t *testing.T, obs rx.Observable[layout.Widget]) layout.Widget {
	t.Helper()
	var w layout.Widget
	if err := obs.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("Pagination subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("Pagination did not emit an initial widget")
	}
	return w
}

// densityTheme returns a theme whose density is d, with sharp corners
// for golden determinism, mirroring components' density tests.
func densityTheme(d tokens.Density) theme.Theme {
	th := theme.Default()
	th.Density = rx.Of(d)
	th.Radius = rx.Of(tokens.RadiusScale{})
	return th
}

// TestPaginationCompactGolden records or diffs the compact-density golden
// through the LIVE pipeline (the static Render path is frozen at
// tokens.Comfortable): the page squares densify to the 28 dp
// ControlHeight and the chevron glyphs to the 16 dp icon size.
func TestPaginationCompactGolden(t *testing.T) {
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	props := pagination.Props{Page: 3, PageCount: 5, Shaper: defaultShaper(t)}
	w := liveWidget(t, pagination.Pagination(rx.Of(densityTheme(tokens.Compact)), props))
	golden.Render(t, "light-compact-page-3-of-5", canvasSize, scene(w, lightBG))
}
