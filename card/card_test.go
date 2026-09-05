package card_test

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
	"github.com/vibrantgio/patterns/card"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

const (
	canvasW, canvasH = 280, 200
	// The card draws into its full constraints, so a golden that must
	// show the card's edge against the surface it stands on insets it by
	// this margin.
	marginPx = 16
)

var (
	canvasSize = image.Pt(canvasW, canvasH)
	// Sharp corner radius. Anti-aliased rounded corners vary slightly
	// between GPU contexts, breaking determinism. Sharp edges still
	// exercise the fill colour, outline stroke, and shadow presence.
	sharpRadius = tokens.RadiusScale{}
)

// fillRect is a simple sharp-edged solid widget used as a slot stand-in
// wherever the case is about slot geometry rather than slot content.
func fillRect(c color.NRGBA, heightDp float32) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		h := gtx.Dp(unit.Dp(heightDp))
		size := image.Pt(gtx.Constraints.Max.X, h)
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	}
}

// defaultShaper returns the shaper every golden here draws with: the default
// typography's faces pinned, system fonts off, so the stored images are the
// same on every machine. A golden test pins its faces with
// DeterministicShaper; application code takes the fallback Shaper.
func defaultShaper(t *testing.T) *text.Shaper {
	t.Helper()
	return tokens.DefaultTypography.DeterministicShaper()
}

// textSlot returns a slot widget that draws s in the given role.
//
// Card's Props carries no Shaper because it draws no text of its own: all
// three slots are caller-supplied widgets, so the typeface inside a card is
// settled by whoever builds them. This is that caller. Text slots rather
// than coloured bars let the goldens show the slot stack absorbing real
// content — the S3 gaps between surviving slots, and whether anything is
// clipped at the card's inner edge.
//
// ASCII only — no symbol reaches a stored image.
//
// Text here must draw through theme/typeset rather than
// gioui.org/widget.Label: a role's LineHeight is the CSS line box, and
// Label does not produce that box.
func textSlot(shaper *text.Shaper, style tokens.TextStyle, c color.NRGBA, maxLines int, s string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		paint.ColorOp{Color: c}.Add(gtx.Ops)
		material := m.Stop()

		f := typeset.Font(style, font.Normal)
		lbl := typeset.Label(style, maxLines)
		// Min is dropped so the slot reports the text it drew rather than the
		// card's own minimum, which is what makes the stack of slots visible in
		// the golden. typeset.Layout re-applies whatever constraints remain.
		gtx.Constraints.Min = image.Point{}
		return typeset.Layout(gtx, shaper, lbl, f, unit.Sp(style.Size), s, material)
	}
}

// slots returns the header / body / footer trio every card case draws, in the
// colours of the given token set: a title, a body long enough to wrap inside a
// 280 px card, and a footer line.
func slots(t *testing.T, c tokens.ColorTokens) (header, body, footer layout.Widget) {
	t.Helper()
	shaper := defaultShaper(t)
	typo := tokens.DefaultTypography
	return textSlot(shaper, typo.TitleMedium, c.Text, 1, "Density"),
		textSlot(shaper, typo.BodyMedium, c.Ramps.Neutral.Step(700), 3,
			"Comfortable and Compact set the control height and the padding around it."),
		textSlot(shaper, typo.LabelMedium, c.Primary, 1, "Read the token")
}

// scene renders w into a canvas-sized constraint. The optional margin
// leaves the surface the card stands on visible around it.
func scene(w layout.Widget, margin int, bgColor color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.UniformInset(unit.Dp(float32(margin))).Layout(gtx, w)
	}
}

// TestCardGolden records or diffs the canonical card renders.
// light-header-only asserts that a lone slot is not padded as though the
// other two were there but empty; light-margin leaves the surface the card
// stands on visible around it, so the raise is read at the card's edge.
func TestCardGolden(t *testing.T) {
	cases := []struct {
		name       string
		colors     tokens.ColorTokens
		headerOnly bool
		bg         color.NRGBA
		margin     int
	}{
		{
			name:   "light-normal",
			colors: tokens.DefaultLight,
			bg:     color.NRGBA{R: 240, G: 240, B: 240, A: 255},
			margin: 0,
		},
		{
			name:   "dark-normal",
			colors: tokens.DefaultDark,
			bg:     color.NRGBA{R: 20, G: 20, B: 20, A: 255},
			margin: 0,
		},
		{
			name:       "light-header-only",
			colors:     tokens.DefaultLight,
			headerOnly: true,
			bg:         color.NRGBA{R: 240, G: 240, B: 240, A: 255},
			margin:     0,
		},
		{
			name:   "light-margin",
			colors: tokens.DefaultLight,
			bg:     color.NRGBA{R: 240, G: 240, B: 240, A: 255},
			margin: marginPx,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header, body, footer := slots(t, tc.colors)
			props := card.Props{Header: header, Body: body, Footer: footer}
			if tc.headerOnly {
				props = card.Props{Header: header}
			}
			w := card.Render(props, tc.colors, tokens.Spacing, sharpRadius)
			golden.Render(t, tc.name, canvasSize, scene(w, tc.margin, tc.bg))
		})
	}
}

// TestCardEdgeIsNeverAnOutline holds the ruling in pixels: a card's edge is
// its raise and, where the scheme has run out of steps, the seam that raise
// owes — never a 3:1 outline. Level 0 in the light scheme has a step left,
// so the card's edge pixels are its own fill; level 1 has none, so they are
// the seam the raise owes and nothing louder.
func TestCardEdgeIsNeverAnOutline(t *testing.T) {
	c := tokens.DefaultLight
	for _, tc := range []struct {
		name  string
		level tokens.ElevationLevel
	}{
		{"level-0", tokens.Level0},
		{"level-1", tokens.Level1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raise := c.RaisedOn(c.SurfaceAt(tc.level))
			want := raise.Fill
			if raise.Seamed {
				want = raise.Seam
			}
			w := card.Render(card.Props{Header: fillRect(color.NRGBA{R: 60, G: 110, B: 200, A: 255}, 24), Level: tc.level},
				c, tokens.Spacing, sharpRadius)
			img := golden.Capture(t, canvasSize, scene(w, marginPx, c.SurfaceAt(tc.level)))
			got := img.At(marginPx, canvasH/2)
			r, g, b, _ := got.RGBA()
			if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B {
				t.Errorf("%s: card edge is #%02x%02x%02x, want the raise's own #%02x%02x%02x",
					tc.name, uint8(r>>8), uint8(g>>8), uint8(b>>8), want.R, want.G, want.B)
			}
		})
	}
}

// TestCardLightDarkDiffer confirms that swapping the colour token set
// changes the rendered output.
func TestCardLightDarkDiffer(t *testing.T) {
	header := fillRect(color.NRGBA{R: 60, G: 110, B: 200, A: 255}, 24)
	body := fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 48)
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}

	light := card.Render(card.Props{Header: header, Body: body}, tokens.DefaultLight, tokens.Spacing, sharpRadius)
	dark := card.Render(card.Props{Header: header, Body: body}, tokens.DefaultDark, tokens.Spacing, sharpRadius)

	imgLight := golden.Capture(t, canvasSize, scene(light, 0, bg))
	imgDark := golden.Capture(t, canvasSize, scene(dark, 0, bg))
	if n := golden.PixelDiff(imgLight, imgDark); n == 0 {
		t.Error("light and dark cards render identically; expected colour differences")
	}
}
