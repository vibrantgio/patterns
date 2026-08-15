package tag_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/tag"
	"github.com/vibrantgio/theme/tokens"
)

const (
	canvasW, canvasH = 160, 48
)

var (
	canvasSize = image.Pt(canvasW, canvasH)
	// Sharp corner radius keeps the goldens deterministic — anti-aliased
	// rounded corners and the pill's Full radius both vary slightly
	// between GPU contexts, breaking pixel-exact diffs.
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

// scene renders w at the top-left of a canvas-sized constraint over a flat
// fill of the scheme's Surface pin — the ground a resting chip actually
// sits on, and the base its status variants tint, so the goldens record the
// tint against the pane it separates from.
func scene(w layout.Widget, bg color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// The five variants' specimen labels. The status labels are the fixture
// labels the design/mirror harness compares against; keep them in sync.
const (
	labelFilled  = "Popular"
	labelTonal   = "New in 2.0"
	labelSuccess = "Passing"
	labelWarning = "Degraded"
	labelError   = "Failing"
)

// TestTagGolden records or diffs one golden per variant per scheme: the two
// historical chips (pricing's Filled "Popular", hero's Tonal eyebrow) and
// the three status treatments, each over the scheme's Surface ground.
func TestTagGolden(t *testing.T) {
	shaper := defaultShaper(t)

	cases := []struct {
		name    string
		colors  tokens.ColorTokens
		label   string
		variant tag.Variant
	}{
		{"light-filled", tokens.DefaultLight, labelFilled, tag.Filled},
		{"light-tonal", tokens.DefaultLight, labelTonal, tag.Tonal},
		{"light-success", tokens.DefaultLight, labelSuccess, tag.Success},
		{"light-warning", tokens.DefaultLight, labelWarning, tag.Warning},
		{"light-error", tokens.DefaultLight, labelError, tag.Error},
		{"dark-success", tokens.DefaultDark, labelSuccess, tag.Success},
		{"dark-warning", tokens.DefaultDark, labelWarning, tag.Warning},
		{"dark-error", tokens.DefaultDark, labelError, tag.Error},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := tag.Render(shaper, tc.label, tc.variant, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelSmall)
			golden.Render(t, tc.name, canvasSize, scene(w, tc.colors.Surface))
		})
	}
}

// TestStatusVariantsDiffer confirms the three status treatments are three
// different drawings — a level mapping bug that collapsed two levels onto
// one colour would slip past per-variant goldens only until the next
// regeneration, but never past this.
func TestStatusVariantsDiffer(t *testing.T) {
	shaper := defaultShaper(t)
	capture := func(v tag.Variant) *image.RGBA {
		w := tag.Render(shaper, "Status", v, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelSmall)
		return golden.Capture(t, canvasSize, scene(w, tokens.DefaultLight.Surface))
	}
	success, warning, err := capture(tag.Success), capture(tag.Warning), capture(tag.Error)
	if golden.PixelDiff(success, warning) == 0 {
		t.Error("success and warning tags render identically; expected the level pins to differ")
	}
	if golden.PixelDiff(warning, err) == 0 {
		t.Error("warning and error tags render identically; expected the level pins to differ")
	}
	if golden.PixelDiff(success, err) == 0 {
		t.Error("success and error tags render identically; expected the level pins to differ")
	}
}

// TestTagLightDarkDiffer confirms a status chip flips with the scheme: the
// fixed-hue level roles are paired light/dark ramps, not literals.
func TestTagLightDarkDiffer(t *testing.T) {
	shaper := defaultShaper(t)
	light := tag.Render(shaper, labelSuccess, tag.Success, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelSmall)
	dark := tag.Render(shaper, labelSuccess, tag.Success, tokens.DefaultDark, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelSmall)

	imgLight := golden.Capture(t, canvasSize, scene(light, tokens.DefaultLight.Surface))
	imgDark := golden.Capture(t, canvasSize, scene(dark, tokens.DefaultDark.Surface))
	if golden.PixelDiff(imgLight, imgDark) == 0 {
		t.Error("light and dark status tags render identically; expected colour differences")
	}
}
