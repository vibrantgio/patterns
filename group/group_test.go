package group_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/group"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

const (
	canvasW, canvasH = 280, 200
	// The group draws into its full constraints, so a golden that must
	// show the hairline against the surface the group is in insets it by
	// this margin.
	marginPx = 16
)

var (
	canvasSize = image.Pt(canvasW, canvasH)
	// Sharp corner radius. Anti-aliased rounded corners vary slightly
	// between GPU contexts, breaking determinism.
	sharpRadius = tokens.RadiusScale{}
)

// defaultShaper returns the shaper every golden here draws with: the
// default typography's faces pinned, system fonts off, so the stored
// images are the same on every machine.
func defaultShaper(t *testing.T) *text.Shaper {
	t.Helper()
	return tokens.DefaultTypography.DeterministicShaper()
}

// textSlot returns a content widget that draws s in the given role. A group
// draws no text but its own label, so everything it holds is caller-built.
//
// ASCII only — no symbol reaches a stored image.
func textSlot(shaper *text.Shaper, style tokens.TextStyle, c color.NRGBA, maxLines int, s string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		paint.ColorOp{Color: c}.Add(gtx.Ops)
		material := m.Stop()

		f := typeset.Font(style, font.Normal)
		lbl := typeset.Label(style, maxLines)
		gtx.Constraints.Min = image.Point{}
		return typeset.Layout(gtx, shaper, lbl, f, unit.Sp(style.Size), s, material)
	}
}

// content is the pair of widgets every group case holds.
func content(t *testing.T, c tokens.ColorTokens) []layout.Widget {
	t.Helper()
	shaper := defaultShaper(t)
	typo := tokens.DefaultTypography
	return []layout.Widget{
		textSlot(shaper, typo.BodyMedium, c.Text, 1, "Comfortable"),
		textSlot(shaper, typo.BodyMedium, c.Ramps.Neutral.Step(700), 2,
			"Compact re-pitches every control on the page."),
	}
}

// scene renders w onto the surface the group is in, so the hairline has the
// fill it was derived against on both of its sides.
func scene(w layout.Widget, margin int, bgColor color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.UniformInset(unit.Dp(float32(margin))).Layout(gtx, w)
	}
}

// TestGroupGolden records or diffs the canonical group renders in both
// schemes, labelled and not. The scene fills the level-0 surface so the
// group's interior and its surroundings are one fill, which is the whole
// point of the pattern: only the hairline says where it ends.
func TestGroupGolden(t *testing.T) {
	cases := []struct {
		name   string
		colors tokens.ColorTokens
		label  string
	}{
		{name: "light-labelled", colors: tokens.DefaultLight, label: "Density"},
		{name: "dark-labelled", colors: tokens.DefaultDark, label: "Density"},
		{name: "light-unlabelled", colors: tokens.DefaultLight},
		{name: "dark-unlabelled", colors: tokens.DefaultDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := group.Render(defaultShaper(t),
				group.Props{Label: tc.label, Content: content(t, tc.colors)},
				tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelLarge)
			golden.Render(t, tc.name, canvasSize, scene(w, marginPx, tc.colors.SurfaceAt(tokens.Level0)))
		})
	}
}

// TestGroupPaintsNothingInside is the ruling in pixels: a group takes the
// fill of the surface it is in and paints nothing there, so a pixel inside
// its bounds and away from its content is the surface's own fill, byte for
// byte, in either scheme.
func TestGroupPaintsNothingInside(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"light", tokens.DefaultLight},
		{"dark", tokens.DefaultDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.colors.SurfaceAt(tokens.Level0)
			w := group.Render(defaultShaper(t), group.Props{}, tc.colors,
				tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelLarge)
			img := golden.Capture(t, canvasSize, scene(w, marginPx, want))
			r, g, b, _ := img.At(canvasW/2, canvasH/2).RGBA()
			if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B {
				t.Errorf("inside the group is #%02x%02x%02x, want the surface's own #%02x%02x%02x",
					uint8(r>>8), uint8(g>>8), uint8(b>>8), want.R, want.G, want.B)
			}
		})
	}
}

// TestGroupHairlineIsTheSeam holds the derivation: the line at the group's
// edge is tokens.SeamOn against the surface the group is in, and not the
// 3:1 mark a graphic carrying meaning would derive to.
func TestGroupHairlineIsTheSeam(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"light", tokens.DefaultLight},
		{"dark", tokens.DefaultDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bg := tc.colors.SurfaceAt(tokens.Level0)
			want := tc.colors.SeamOn(bg)
			w := group.Render(defaultShaper(t), group.Props{}, tc.colors,
				tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelLarge)
			img := golden.Capture(t, canvasSize, scene(w, marginPx, bg))
			// Sharp corners, so the leading edge is exactly one column of
			// hairline at the margin and nothing is anti-aliased into it.
			r, g, b, _ := img.At(marginPx, canvasH/2).RGBA()
			if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B {
				t.Errorf("hairline is #%02x%02x%02x, want the seam #%02x%02x%02x",
					uint8(r>>8), uint8(g>>8), uint8(b>>8), want.R, want.G, want.B)
			}
		})
	}
}

// TestGroupLabelChangesPixels confirms the label is drawn: an unlabelled
// group and a labelled one differ, so a Label that never reached the shaper
// cannot pass silently.
func TestGroupLabelChangesPixels(t *testing.T) {
	c := tokens.DefaultLight
	shaper := defaultShaper(t)
	bare := group.Render(shaper, group.Props{Content: content(t, c)}, c,
		tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelLarge)
	named := group.Render(shaper, group.Props{Label: "Density", Content: content(t, c)}, c,
		tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelLarge)
	a := golden.Capture(t, canvasSize, scene(bare, marginPx, c.SurfaceAt(tokens.Level0)))
	b := golden.Capture(t, canvasSize, scene(named, marginPx, c.SurfaceAt(tokens.Level0)))
	if n := golden.PixelDiff(a, b); n == 0 {
		t.Error("labelled and unlabelled groups render identically; expected the label")
	}
}
