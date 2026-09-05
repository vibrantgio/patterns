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
	// The canvas must be tall enough that the shared CTA row is not clipped
	// when every card stretches to the tallest tier — the highlighted Team
	// with four feature lines plus the Popular chip.
	canvasW, canvasH = 720, 400
	// scene leaves an S5-equivalent margin around the pricing row so
	// the row's outer cards retain breathing room from the canvas edge.
	marginPx = 20
)

var (
	canvasSize = image.Pt(canvasW, canvasH)
	// Sharp corner radius keeps the goldens deterministic — anti-aliased
	// rounded corners and the card radii both vary
	// slightly between GPU contexts, breaking pixel-exact diffs.
	sharpRadius = tokens.RadiusScale{}
)

// defaultShaper returns the shaper every golden here draws with: the default
// typography's faces pinned, system fonts off, so the stored images are the
// same on every machine. A golden test pins its faces with
// DeterministicShaper; application code takes the fallback Shaper.
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

// tierSpec is one tier's text. Latin text in Roboto rasterises identically
// on every machine with the faces pinned and DeterministicShaper in use.
// ASCII only — no symbol reaches a stored image, and the leading checkmark
// on each feature is a clip path the package draws itself, not a glyph.
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

// tier returns the i'th tier, optionally the recommended one.
func tier(i int, recommended bool) pricing.Tier {
	spec := tierSpecs[i]
	return pricing.Tier{
		Name:        spec.name,
		Price:       spec.price,
		Cadence:     spec.cadence,
		Features:    spec.features,
		CTA:         &pricing.CTA{Label: "Choose"},
		Recommended: recommended,
	}
}

// threeTiers returns the full row, with the middle tier recommended when
// asked: that tier becomes a card and wears the "Popular" badge on its name
// row, while the other two stay groups.
func threeTiers(recommendMiddle bool) []pricing.Tier {
	return []pricing.Tier{tier(0, false), tier(1, recommendMiddle), tier(2, false)}
}

// TestPricingGolden records or diffs the four Measurable goldens.
func TestPricingGolden(t *testing.T) {
	shaper := defaultShaper(t)
	// The row stands on the content, and a group tier takes that surface's
	// own fill — so the scene has to be that surface for the golden to show
	// the tier the way a page does.
	lightBG := tokens.DefaultLight.SurfaceAt(tokens.Level0)
	darkBG := tokens.DefaultDark.SurfaceAt(tokens.Level0)

	three := threeTiers(false)
	threeRecommended := threeTiers(true)
	single := []pricing.Tier{tier(1, false)}

	cases := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
		tiers  []pricing.Tier
	}{
		{"light-three-tier", tokens.DefaultLight, lightBG, three},
		{"dark-three-tier", tokens.DefaultDark, darkBG, three},
		{"light-three-tier-recommended", tokens.DefaultLight, lightBG, threeRecommended},
		{"dark-three-tier-recommended", tokens.DefaultDark, darkBG, threeRecommended},
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

// TestPricingRecommendedDiffers confirms the Recommended flag changes the
// rendered output. Two three-tier rows are rendered with the middle tier
// flipped; the resulting images must differ.
func TestPricingRecommendedDiffers(t *testing.T) {
	shaper := defaultShaper(t)
	bg := tokens.DefaultLight.SurfaceAt(tokens.Level0)

	plain := pricing.Props{Tiers: threeTiers(false), Shaper: shaper}
	recommended := pricing.Props{Tiers: threeTiers(true), Shaper: shaper}

	a := golden.Capture(t, canvasSize, scene(pricing.Render(shaper, plain, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable), bg))
	b := golden.Capture(t, canvasSize, scene(pricing.Render(shaper, recommended, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable), bg))
	if n := golden.PixelDiff(a, b); n == 0 {
		t.Error("plain and recommended pricing render identically; expected the middle tier's raise and its Popular badge to introduce differences")
	}
}

// TestRecommendedTierIsRaisedAndTheRestAreNot holds the ruling in pixels:
// the recommended tier is a card, so its interior is the raise walked from
// the content; every other tier is a group, so its interior is the
// content's own fill, byte for byte.
func TestRecommendedTierIsRaisedAndTheRestAreNot(t *testing.T) {
	shaper := defaultShaper(t)
	for _, sc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"light", tokens.DefaultLight},
		{"dark", tokens.DefaultDark},
	} {
		t.Run(sc.name, func(t *testing.T) {
			page := sc.colors.SurfaceAt(tokens.Level0)
			raise := sc.colors.RaisedOn(page).Fill
			props := pricing.Props{Tiers: threeTiers(true), Shaper: shaper}
			img := golden.Capture(t, canvasSize, scene(
				pricing.Render(shaper, props, sc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable), page))
			// The row is three equal columns inside the scene's margin;
			// the sample sits in the middle of each column, a few pixels
			// under its top edge, which is inside the S5 inset and so
			// clear of both the hairline and the tier's first row of text.
			row := canvasSize.X - 2*marginPx
			at := func(col int) color.NRGBA {
				x := marginPx + row*col/3 + row/6
				r, g, b, _ := img.At(x, marginPx+4).RGBA()
				return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
			}
			for _, tc := range []struct {
				col  int
				want color.NRGBA
				what string
			}{
				{0, page, "a group tier takes the content's own fill"},
				{1, raise, "the recommended tier is raised on the content"},
				{2, page, "a group tier takes the content's own fill"},
			} {
				if got := at(tc.col); got != tc.want {
					t.Errorf("tier %d is #%02x%02x%02x, want #%02x%02x%02x: %s",
						tc.col, got.R, got.G, got.B, tc.want.R, tc.want.G, tc.want.B, tc.what)
				}
			}
		})
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

// TestPricingUnevenFeaturesMatchTallest holds the invariant that a row
// whose tiers have 1, 4 and 2 feature lines is as tall as a row of three
// copies of the 4-line tier. The short cards stretch; they do not set the
// height.
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
