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
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

const (
	frameW, frameH = 280, 200
	// The card draws into its full constraints, so a golden that must
	// show the card's edge against the surface it stands on insets it by
	// this margin.
	marginPx = 16
)

var (
	frameSize = image.Pt(frameW, frameH)
	// Sharp corner radius. Anti-aliased rounded corners vary slightly
	// between GPU contexts, breaking determinism. Sharp edges still
	// exercise the fill colour, outline stroke, and shadow presence.
	sharpRadius = tokens.RadiusScale{}
)

// fillRect is a simple sharp-edged solid layout.Widget used as a slot stand-in
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

// textSlot returns a slot layout.Widget that draws s in the given role.
//
// Card's Props carries no Shaper because it draws no text of its own: all
// three slots are caller-supplied layout.Widget values, so the typeface inside a card is
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
func slots(t *testing.T, c tokens.PlatformColors) (header, body, footer layout.Widget) {
	t.Helper()
	shaper := defaultShaper(t)
	typo := tokens.DefaultTypography
	return textSlot(shaper, typo.TitleMedium, vgcolor.Flatten(c.Label, c.CardFill), 1, "Density"),
		textSlot(shaper, typo.BodyMedium, vgcolor.Flatten(c.SecondaryLabel, c.CardFill), 3,
			"Comfortable and Compact set the control height and the padding around it."),
		textSlot(shaper, typo.LabelMedium, c.ControlAccent, 1, "Read the token")
}

// scene renders w into a frame-sized constraint. The optional margin
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
// stands on visible around it, so the box's step of fill is read at the
// card's edge.
func TestCardGolden(t *testing.T) {
	cases := []struct {
		name       string
		colors     tokens.PlatformColors
		headerOnly bool
		bg         color.NRGBA
		margin     int
	}{
		{
			name:   "light-normal",
			colors: tokens.PlatformLight,
			bg:     tokens.PlatformLight.ControlBackground,
			margin: 0,
		},
		{
			name:   "dark-normal",
			colors: tokens.PlatformDark,
			bg:     tokens.PlatformDark.ControlBackground,
			margin: 0,
		},
		{
			name:       "light-header-only",
			colors:     tokens.PlatformLight,
			headerOnly: true,
			bg:         tokens.PlatformLight.ControlBackground,
			margin:     0,
		},
		{
			name:   "light-margin",
			colors: tokens.PlatformLight,
			bg:     tokens.PlatformLight.ControlBackground,
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
			golden.Render(t, tc.name, frameSize, scene(w, tc.margin, tc.bg))
		})
	}
}

// TestCardEdgeIsTheBoxsFill holds the ruling in pixels: a card's edge is the
// platform's box and nothing else — the first pixel inside the card is
// CardFill, never a hairline and never a shadow.
func TestCardEdgeIsTheBoxsFill(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.PlatformColors
	}{
		{"light", tokens.PlatformLight},
		{"dark", tokens.PlatformDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.colors
			want := c.CardFill
			w := card.Render(card.Props{Header: fillRect(color.NRGBA{R: 60, G: 110, B: 200, A: 255}, 24)},
				c, tokens.Spacing, sharpRadius)
			img := golden.Capture(t, frameSize, scene(w, marginPx, c.ControlBackground))
			got := img.At(marginPx, frameH/2)
			r, g, b, _ := got.RGBA()
			if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B {
				t.Errorf("%s: card edge is #%02x%02x%02x, want the box's own #%02x%02x%02x",
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

	light := card.Render(card.Props{Header: header, Body: body}, tokens.PlatformLight, tokens.Spacing, sharpRadius)
	dark := card.Render(card.Props{Header: header, Body: body}, tokens.PlatformDark, tokens.Spacing, sharpRadius)

	imgLight := golden.Capture(t, frameSize, scene(light, 0, bg))
	imgDark := golden.Capture(t, frameSize, scene(dark, 0, bg))
	if n := golden.PixelDiff(imgLight, imgDark); n == 0 {
		t.Error("light and dark cards render identically; expected colour differences")
	}
}
