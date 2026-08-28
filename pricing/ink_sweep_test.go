package pricing

// This file is an internal test (package pricing, not pricing_test) so it
// can exercise tierPrimaryInk directly, the way theme/tokens/ink_test.go
// exercises ColorTokens.InkOn and components/richtext/link_test.go
// exercises richtext.FromTokens's LinkColor field. drawTier and
// checkmarkWidget have no exported field to read the drawn ink back off
// of, so the derivation itself is the seam this file measures.

import (
	"fmt"
	stdcolor "image/color"
	"math/rand"
	"testing"

	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// tierInkSweepSeeds is the seed population this file reads the highlighted
// tier's ring and the feature checkmarks' ink claims against, the same one
// theme/tokens and components/richtext sweep their derivations with: the
// default seed, the nine macOS system accents, both ends of the tonal
// axis, three pastels stated at a dark scheme's tone, and four hundred
// random colours from a fixed source.
//
// The three pastels are the shape that produced the AV1 defect family. A
// palette published for a dark scheme states its accents high on the tonal
// axis, and a brand seeded with one of them derives a light scheme whose
// primary pin sits a whisper off its own ground — which is exactly what
// the highlighted tier's ring and every tier's checkmarks used to be
// coloured with.
func tierInkSweepSeeds() []stdcolor.NRGBA {
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

func tierInkHex(c stdcolor.NRGBA) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

// tierInkSweepSchemes yields every palette the sweep reads a seed as: both
// derivations, both schemes.
func tierInkSweepSchemes(seed stdcolor.NRGBA) []struct {
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

// TestTierPrimaryInkClearsTheGraphicFloorForEverySeed is AV1.2's
// site-level gate: whatever a caller seeds the palette with, the ink drawn
// directly on a tier card — the highlighted ring, the feature checkmarks —
// reaches WCAG 1.4.11 against the card's own level-1 fill.
func TestTierPrimaryInkClearsTheGraphicFloorForEverySeed(t *testing.T) {
	worstLight, worstDark := 99.0, 99.0
	var worstLightAt, worstDarkAt string
	for _, seed := range tierInkSweepSeeds() {
		for _, s := range tierInkSweepSchemes(seed) {
			fill := s.tok.SurfaceAt(tokens.Level1)
			ink := tierPrimaryInk(s.tok)
			got := color.ContrastRatio(ink, fill)
			if got < tokens.GraphicFloor {
				t.Errorf("seed %s: %s: tier ink %s on card %s measures %.2f:1, under the %.1f:1 graphic floor",
					tierInkHex(seed), s.name, tierInkHex(ink), tierInkHex(fill), got, tokens.GraphicFloor)
			}
			if s.light && got < worstLight {
				worstLight, worstLightAt = got, tierInkHex(seed)
			}
			if !s.light && got < worstDark {
				worstDark, worstDarkAt = got, tierInkHex(seed)
			}
		}
	}
	t.Logf("over %d seeds: worst light tier ink %.2f:1 (%s), worst dark tier ink %.2f:1 (%s)",
		len(tierInkSweepSeeds()), worstLight, worstLightAt, worstDark, worstDarkAt)
}

// TestTheCanonicalSeedsTierPrimaryInkIsThePrimaryPin states what this
// repair costs every stored image in the design system, which is nothing:
// on the seed every golden is rendered from, the brand's own colour clears
// the floor on the card and is what the ring and checkmarks get, exactly
// as before.
func TestTheCanonicalSeedsTierPrimaryInkIsThePrimaryPin(t *testing.T) {
	for _, s := range []struct {
		name string
		tok  tokens.ColorTokens
	}{
		{"DefaultLight", tokens.DefaultLight},
		{"DefaultDark", tokens.DefaultDark},
	} {
		if ink := tierPrimaryInk(s.tok); ink != s.tok.Primary {
			t.Errorf("%s: tier ink is %s, not the Primary pin %s — a golden moved",
				s.name, tierInkHex(ink), tierInkHex(s.tok.Primary))
		}
	}
}

// TestAPastelSeedsTierPrimaryInkLeavesThePin is the regression itself,
// read on the shape that produced it: a light scheme seeded with a dark
// scheme's accent. Before the gate this card's ring and checkmarks were
// the bare pin at a sub-floor ratio.
func TestAPastelSeedsTierPrimaryInkLeavesThePin(t *testing.T) {
	seed := stdcolor.NRGBA{0x89, 0xb4, 0xfa, 0xff}
	light, dark := tokens.FromSeed(seed)

	lightFill := light.SurfaceAt(tokens.Level1)
	if bare := color.ContrastRatio(light.Primary, lightFill); bare >= tokens.GraphicFloor {
		t.Fatalf("this seed's bare light pin now measures %.2f:1 on the card — the test no longer reads the shape it was written for", bare)
	}
	lightInk := tierPrimaryInk(light)
	if lightInk == light.Primary {
		t.Errorf("light tier ink is still the bare pin %s", tierInkHex(light.Primary))
	}

	darkInk := tierPrimaryInk(dark)
	if darkInk != dark.Primary {
		t.Errorf("dark tier ink walked to %s; the dark pin %s clears its card and should stand",
			tierInkHex(darkInk), tierInkHex(dark.Primary))
	}
}
