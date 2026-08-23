package pricing_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/pricing"
	"github.com/vibrantgio/theme/tokens"
)

const (
	// The canvas grew from 720×280 in F4.4b and 720×340 after F4.4b's
	// filled-in copy. AE7.1 stretches every card to the tallest — the
	// highlighted Team with four feature lines plus the Popular chip —
	// so 340 clips the shared CTA row. 400 clears it.
	canvasW, canvasH = 720, 400
	// scene leaves an S5-equivalent margin around the pricing row so
	// the row's outer cards retain breathing room from the canvas edge.
	marginPx = 20
)

var (
	canvasSize = image.Pt(canvasW, canvasH)
	// Sharp corner radius keeps the goldens deterministic — anti-aliased
	// rounded corners and the "Popular" chip's Full radius both vary
	// slightly between GPU contexts, breaking pixel-exact diffs.
	sharpRadius = tokens.RadiusScale{}
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

// scene renders w into a canvas-sized constraint over a flat background
// with a uniform margin so the outer cards do not touch the canvas edge.
func scene(w layout.Widget, bgColor color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.UniformInset(unit.Dp(float32(marginPx))).Layout(gtx, w)
	}
}

// tierSpec is one tier's text. The fields were all blank until F4.4b, on the
// theory that font rasterisation was non-deterministic; F4.2 pinned the faces
// by configuration and F4.3 moved every golden onto DeterministicShaper, so
// Latin text in Roboto rasterises identically on every machine. ASCII only,
// per F4.2 — no symbol reaches a stored image, and the leading checkmark on
// each feature is a clip path the package draws itself, not a glyph.
type tierSpec struct {
	name     string
	price    string
	cadence  string
	features []string
}

// tierSpecs are the three tiers in document order, so a three-up row reads as
// three distinct cards rather than three copies. The feature lines are short:
// a card in a three-up 720 px row is about 220 px wide, and each line is
// MaxLines:1 beside a checkmark, so a longer one would be ellipsized rather
// than wrapped.
var tierSpecs = [3]tierSpec{
	{"Starter", "$0", "/mo", []string{"One project", "Community help"}},
	{"Team", "$29", "/mo", []string{"Ten projects", "Email support", "Shared tokens", "Audit log"}},
	{"Studio", "$99", "/mo", []string{"Unlimited work", "Priority support", "Custom ramps"}},
}

// tier returns the i'th tier, optionally highlighted.
func tier(i int, highlighted bool) pricing.Tier {
	spec := tierSpecs[i]
	return pricing.Tier{
		Name:        spec.name,
		Price:       spec.price,
		Cadence:     spec.cadence,
		Features:    spec.features,
		CTA:         &pricing.CTA{Label: "Choose"},
		Highlighted: highlighted,
	}
}

// threeTiers returns the full row, with the middle tier highlighted when
// asked. Highlighting adds the package's own "Popular" chip on the name row.
func threeTiers(highlightMiddle bool) []pricing.Tier {
	return []pricing.Tier{tier(0, false), tier(1, highlightMiddle), tier(2, false)}
}

// TestPricingGolden records or diffs the four Measurable goldens.
func TestPricingGolden(t *testing.T) {
	shaper := defaultShaper(t)
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	darkBG := color.NRGBA{R: 20, G: 20, B: 20, A: 255}

	three := threeTiers(false)
	threeHighlighted := threeTiers(true)
	single := []pricing.Tier{tier(1, false)}

	cases := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
		tiers  []pricing.Tier
	}{
		{"light-three-tier", tokens.DefaultLight, lightBG, three},
		{"dark-three-tier", tokens.DefaultDark, darkBG, three},
		{"light-three-tier-highlighted", tokens.DefaultLight, lightBG, threeHighlighted},
		{"light-single-tier", tokens.DefaultLight, lightBG, single},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			props := pricing.Props{Tiers: tc.tiers, Shaper: shaper}
			w := pricing.Render(shaper, props, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable)
			golden.Render(t, tc.name, canvasSize, scene(w, tc.bg))
		})
	}
}

// TestPricingHighlightDiffers confirms the Highlighted flag changes the
// rendered output. Two three-tier rows are rendered with the middle tier
// flipped; the resulting images must differ.
func TestPricingHighlightDiffers(t *testing.T) {
	shaper := defaultShaper(t)
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}

	plain := pricing.Props{Tiers: threeTiers(false), Shaper: shaper}
	highlighted := pricing.Props{Tiers: threeTiers(true), Shaper: shaper}

	a := golden.Capture(t, canvasSize, scene(pricing.Render(shaper, plain, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable), bg))
	b := golden.Capture(t, canvasSize, scene(pricing.Render(shaper, highlighted, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable), bg))
	if n := golden.PixelDiff(a, b); n == 0 {
		t.Error("plain and highlighted pricing render identically; expected the 2 dp Primary border and Popular chip on the name row to introduce differences")
	}
}

// TestPricingLightDarkDiffer confirms that swapping the colour token set
// changes the rendered output.
func TestPricingLightDarkDiffer(t *testing.T) {
	shaper := defaultShaper(t)
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	tiers := threeTiers(true)

	light := pricing.Render(shaper, pricing.Props{Tiers: tiers, Shaper: shaper}, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable)
	dark := pricing.Render(shaper, pricing.Props{Tiers: tiers, Shaper: shaper}, tokens.DefaultDark, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable)

	imgLight := golden.Capture(t, canvasSize, scene(light, bg))
	imgDark := golden.Capture(t, canvasSize, scene(dark, bg))
	if n := golden.PixelDiff(imgLight, imgDark); n == 0 {
		t.Error("light and dark pricing render identically; expected colour differences")
	}
}

// TestPricingUnevenFeaturesMatchTallest pins AE7.1: a row whose tiers
// have 1, 4 and 2 feature lines is as tall as a row of three copies of
// the 4-line tier. The short cards stretch; they do not set the height.
func TestPricingUnevenFeaturesMatchTallest(t *testing.T) {
	shaper := defaultShaper(t)
	cta := &pricing.CTA{Label: "Go"}
	short := pricing.Tier{Name: "A", Features: []string{"one"}, CTA: cta}
	tall := pricing.Tier{Name: "B", Features: []string{"one", "two", "three", "four"}, CTA: cta}
	mid := pricing.Tier{Name: "C", Features: []string{"one", "two"}, CTA: cta}

	size := image.Pt(720, 400)
	render := func(tiers []pricing.Tier) layout.Dimensions {
		return drawOnce(t, size, pricing.Render(shaper, pricing.Props{Tiers: tiers, Shaper: shaper}, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable))
	}
	uneven := render([]pricing.Tier{short, tall, mid})
	allTall := render([]pricing.Tier{tall, tall, tall})
	allShort := render([]pricing.Tier{short, short, short})
	if uneven.Size.Y != allTall.Size.Y {
		t.Errorf("uneven row height %d, all-tall %d; cards must share the tallest height", uneven.Size.Y, allTall.Size.Y)
	}
	if allShort.Size.Y >= uneven.Size.Y {
		t.Errorf("short row height %d should be less than uneven %d", allShort.Size.Y, uneven.Size.Y)
	}
}

func drawOnce(t *testing.T, size image.Point, w layout.Widget) layout.Dimensions {
	t.Helper()
	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Constraints{Max: size},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	return w(gtx)
}
