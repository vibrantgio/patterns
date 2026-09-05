package tabs

// This file is an internal test (package tabs, not tabs_test) so it can
// exercise underlineInk directly, the way theme/tokens/ink_test.go
// exercises ColorTokens.InkOn and components/richtext/link_test.go
// exercises richtext.FromTokens's LinkColor field. tabCell has no exported
// field to read the drawn ink back off of, so the derivation itself is the
// seam this file measures.

import (
	"fmt"
	stdcolor "image/color"
	"math/rand"
	"testing"

	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// underlineSweepSeeds is the seed population this file reads the tab
// underline's ink claims against, the same one theme/tokens and
// components/richtext sweep their derivations with: the default seed, the
// nine macOS system accents, both ends of the tonal axis, three pastels
// stated at a dark scheme's tone, and four hundred random colours from a
// fixed source.
//
// A palette published for a dark scheme states its accents high on the
// tonal axis, so a brand seeded with one of them can derive a light scheme
// whose primary pin sits a whisper off its own ground — the three pastels
// exercise exactly that shape.
func underlineSweepSeeds() []stdcolor.NRGBA {
	rng := rand.New(rand.NewSource(20260827))
	seeds := []stdcolor.NRGBA{
		tokens.DefaultSeed,
		{0xff, 0x3b, 0x30, 0xff}, {0xff, 0x95, 0x00, 0xff}, {0xff, 0xcc, 0x00, 0xff},
		{0x28, 0xcd, 0x41, 0xff}, {0x00, 0x7a, 0xff, 0xff}, {0xaf, 0x52, 0xde, 0xff},
		{0xff, 0x2d, 0x55, 0xff}, {0x8e, 0x8e, 0x93, 0xff}, {0x00, 0x00, 0x00, 0xff},
		{0xff, 0xff, 0xff, 0xff},
		{0x89, 0xb4, 0xfa, 0xff}, {0xcb, 0xa6, 0xf7, 0xff}, {0xa6, 0xe3, 0xa1, 0xff},
	}
	for i := 0; i < 400; i++ {
		seeds = append(seeds, stdcolor.NRGBA{
			R: uint8(rng.Intn(256)), G: uint8(rng.Intn(256)), B: uint8(rng.Intn(256)), A: 0xff})
	}
	return seeds
}

func underlineHex(c stdcolor.NRGBA) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

// underlineSweepSchemes yields every palette the sweep reads a seed as:
// both derivations, both schemes.
func underlineSweepSchemes(seed stdcolor.NRGBA) []struct {
	name  string
	tok   tokens.ColorTokens
	light bool
} {
	light, dark := tokens.FromSeed(seed)
	hcLight, hcDark := tokens.FromSeedHighContrast(seed)
	return []struct {
		name  string
		tok   tokens.ColorTokens
		light bool
	}{
		{"FromSeed light", light, true},
		{"FromSeed dark", dark, false},
		{"FromSeedHighContrast light", hcLight, true},
		{"FromSeedHighContrast dark", hcDark, false},
	}
}

// underlinePanels are every panel Props.Ground can name. The strip band the
// underline sits on is the raise walked from each, which is what
// underlineBands answers.
var underlinePanels = []tokens.ElevationLevel{
	tokens.Level0, tokens.Level1, tokens.Level2, tokens.Level3,
}

// underlineBands are every strip band Props.Ground can actually produce: the
// raise walked from each panel, clamped where the scheme runs out of steps.
func underlineBands(c tokens.ColorTokens) []stdcolor.NRGBA {
	bands := make([]stdcolor.NRGBA, 0, len(underlinePanels))
	for _, panel := range underlinePanels {
		bands = append(bands, c.RaisedOn(c.SurfaceAt(panel)).Fill)
	}
	return bands
}

// TestUnderlineInkClearsTheGraphicFloorForEverySeed is the site-level
// gate: whatever a caller seeds the palette with, and whatever storey the
// tab panel stands on, the selected tab's underline reaches WCAG 1.4.11
// against the strip band it is actually drawn on.
func TestUnderlineInkClearsTheGraphicFloorForEverySeed(t *testing.T) {
	worstLight, worstDark := 99.0, 99.0
	var worstLightAt, worstDarkAt string
	for _, seed := range underlineSweepSeeds() {
		for _, s := range underlineSweepSchemes(seed) {
			for _, band := range underlineBands(s.tok) {
				ink := underlineInk(s.tok, band)
				got := color.ContrastRatio(ink, band)
				if got < tokens.GraphicFloor {
					t.Errorf("seed %s: %s: underline ink %s on band %s measures %.2f:1, under the %.1f:1 graphic floor",
						underlineHex(seed), s.name, underlineHex(ink), underlineHex(band), got, tokens.GraphicFloor)
				}
				if s.light && got < worstLight {
					worstLight, worstLightAt = got, underlineHex(seed)
				}
				if !s.light && got < worstDark {
					worstDark, worstDarkAt = got, underlineHex(seed)
				}
			}
		}
	}
	t.Logf("over %d seeds: worst light underline %.2f:1 (%s), worst dark underline %.2f:1 (%s)",
		len(underlineSweepSeeds()), worstLight, worstLightAt, worstDark, worstDarkAt)
}

// TestTheCanonicalSeedsUnderlineInkIsThePrimaryPin guards the canonical
// seed's golden images: the brand's own colour clears the floor on every
// strip band, so the underline is exactly the Primary pin.
func TestTheCanonicalSeedsUnderlineInkIsThePrimaryPin(t *testing.T) {
	for _, s := range []struct {
		name string
		tok  tokens.ColorTokens
	}{
		{"DefaultLight", tokens.DefaultLight},
		{"DefaultDark", tokens.DefaultDark},
	} {
		for _, band := range underlineBands(s.tok) {
			if ink := underlineInk(s.tok, band); ink != s.tok.Primary {
				t.Errorf("%s band %s: underline ink is %s, not the Primary pin %s — a golden moved",
					s.name, underlineHex(band), underlineHex(ink), underlineHex(s.tok.Primary))
			}
		}
	}
}

// TestAPastelSeedsUnderlineInkLeavesThePin covers a light scheme seeded
// with a dark scheme's accent: the bare pin is sub-floor on the strip
// band, so the underline must walk off it, while a dark scheme whose own
// pin clears the band keeps the pin.
func TestAPastelSeedsUnderlineInkLeavesThePin(t *testing.T) {
	seed := stdcolor.NRGBA{0x89, 0xb4, 0xfa, 0xff}
	light, dark := tokens.FromSeed(seed)

	lightBand := light.RaisedOn(light.SurfaceAt(tokens.Level0)).Fill
	if bare := color.ContrastRatio(light.Primary, lightBand); bare >= tokens.GraphicFloor {
		t.Fatalf("this seed's bare light pin now measures %.2f:1 on the strip band — the test no longer reads the shape it was written for", bare)
	}
	lightInk := underlineInk(light, lightBand)
	if lightInk == light.Primary {
		t.Errorf("light underline ink is still the bare pin %s", underlineHex(light.Primary))
	}

	darkBand := dark.RaisedOn(dark.SurfaceAt(tokens.Level0)).Fill
	darkInk := underlineInk(dark, darkBand)
	if darkInk != dark.Primary {
		t.Errorf("dark underline ink walked to %s; the dark pin %s clears its band and should stand",
			underlineHex(darkInk), underlineHex(dark.Primary))
	}
}
