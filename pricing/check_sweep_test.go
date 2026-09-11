package pricing

// This file is an internal test (package pricing, not pricing_test) so it
// can exercise checkForeground directly, the way theme/tokens/foreground_test.go
// exercises ColorTokens.ForegroundOnAtFloor and
// components/paragraph/link_test.go exercises paragraph.FromTokens's
// LinkColor field. checkmarkWidget has no exported field to read the drawn
// colour back off of, so the derivation itself is the seam this file
// measures.
//
// A tier's checkmarks are drawn on two different surfaces — the content
// itself inside a group tier, the raise inside the recommended card — so
// every claim here is held on both.

import (
	"fmt"
	stdcolor "image/color"
	"math/rand"
	"testing"

	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// checkSweepSeeds is the seed population this file reads the feature
// checkmarks' colour claims against, the same one
// theme/tokens and components/paragraph sweep their derivations with: the
// default seed, the nine macOS system accents, both ends of the tonal
// axis, three pastels stated at a dark scheme's tone, and four hundred
// random colours from a fixed source.
//
// A palette published for a dark scheme states its accents high on the
// tonal axis, and a brand seeded with one of them derives a light scheme
// whose primary pin can sit a whisper off the surface it stands on — the shape the
// three pastels exercise.
func checkSweepSeeds() []stdcolor.NRGBA {
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

func checkHex(c stdcolor.NRGBA) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

// tierKinds is the pair of tiers a row holds: the group every ordinary tier
// is, and the card the recommended one is. They stand at different levels,
// so a checkmark's colour is derived twice.
var tierKinds = []struct {
	name string
	tier Tier
}{
	{"group tier", Tier{}},
	{"recommended card", Tier{Recommended: true}},
}

// checkSweepSchemes yields every palette the sweep reads a seed as: both
// derivations, both schemes.
func checkSweepSchemes(seed stdcolor.NRGBA) []struct {
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

// TestCheckForegroundClearsTheGraphicFloorForEverySeed holds the invariant that
// whatever a caller seeds the palette with, the colour drawn directly on a
// tier — the feature checkmarks — reaches WCAG 1.4.11 against that tier's
// own fill, on the group and on the recommended card alike.
func TestCheckForegroundClearsTheGraphicFloorForEverySeed(t *testing.T) {
	worstLight, worstDark := 99.0, 99.0
	var worstLightAt, worstDarkAt string
	for _, seed := range checkSweepSeeds() {
		for _, s := range checkSweepSchemes(seed) {
			for _, k := range tierKinds {
				fill := tierFill(s.tok, k.tier)
				foreground := checkForeground(s.tok, fill)
				got := color.Magnitude(foreground, fill)
				if got < tokens.GraphicFloor {
					t.Errorf("seed %s: %s: %s: check colour %s on %s measures |Lc| %.2f, under the |Lc| %.1f graphic floor",
						checkHex(seed), s.name, k.name, checkHex(foreground), checkHex(fill), got, tokens.GraphicFloor)
				}
				if s.light && got < worstLight {
					worstLight, worstLightAt = got, checkHex(seed)
				}
				if !s.light && got < worstDark {
					worstDark, worstDarkAt = got, checkHex(seed)
				}
			}
		}
	}
	t.Logf("over %d seeds: worst light check colour |Lc| %.2f (%s), worst dark check colour |Lc| %.2f (%s)",
		len(checkSweepSeeds()), worstLight, worstLightAt, worstDark, worstDarkAt)
}

// TestTheCanonicalSeedsCheckForegroundIsThePrimaryPin holds the invariant that on
// the seed every golden is rendered from, the brand's own colour clears the
// floor on both tier surfaces and is what the checkmarks get.
func TestTheCanonicalSeedsCheckForegroundIsThePrimaryPin(t *testing.T) {
	for _, s := range []struct {
		name string
		tok  tokens.ColorTokens
	}{
		{"DefaultLight", tokens.DefaultLight},
		{"DefaultDark", tokens.DefaultDark},
	} {
		for _, k := range tierKinds {
			if foreground := checkForeground(s.tok, tierFill(s.tok, k.tier)); foreground != s.tok.Primary {
				t.Errorf("%s: %s: check colour is %s, not the Primary pin %s — a golden moved",
					s.name, k.name, checkHex(foreground), checkHex(s.tok.Primary))
			}
		}
	}
}

// TestAPastelSeedsCheckForegroundLeavesThePin holds the invariant on a light
// scheme seeded with a dark scheme's accent: the bare pin sits under the
// graphic floor on the recommended card, so the checkmarks there must not
// be the bare pin.
func TestAPastelSeedsCheckForegroundLeavesThePin(t *testing.T) {
	seed := stdcolor.NRGBA{0x89, 0xb4, 0xfa, 0xff}
	light, dark := tokens.FromSeed(seed)
	recommended := Tier{Recommended: true}

	lightFill := tierFill(light, recommended)
	if bare := color.Magnitude(light.Primary, lightFill); bare >= tokens.GraphicFloor {
		t.Fatalf("this seed's bare light pin now measures |Lc| %.2f on the card — the test no longer reads the shape it was written for", bare)
	}
	if lightForeground := checkForeground(light, lightFill); lightForeground == light.Primary {
		t.Errorf("light check colour is still the bare pin %s", checkHex(light.Primary))
	}

	darkForeground := checkForeground(dark, tierFill(dark, recommended))
	if darkForeground != dark.Primary {
		t.Errorf("dark check colour walked to %s; the dark pin %s clears its card and should stand",
			checkHex(darkForeground), checkHex(dark.Primary))
	}
}
