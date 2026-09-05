package pricing

// This file is an internal test (package pricing, not pricing_test) so it
// can exercise checkInk directly, the way theme/tokens/ink_test.go
// exercises ColorTokens.InkOn and components/richtext/link_test.go
// exercises richtext.FromTokens's LinkColor field. checkmarkWidget has no
// exported field to read the drawn ink back off of, so the derivation
// itself is the seam this file measures.
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

// tierInkSweepSeeds is the seed population this file reads the feature
// checkmarks' ink claims against, the same one
// theme/tokens and components/richtext sweep their derivations with: the
// default seed, the nine macOS system accents, both ends of the tonal
// axis, three pastels stated at a dark scheme's tone, and four hundred
// random colours from a fixed source.
//
// A palette published for a dark scheme states its accents high on the
// tonal axis, and a brand seeded with one of them derives a light scheme
// whose primary pin can sit a whisper off its own ground — the shape the
// three pastels exercise.
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

// tierKinds is the pair of tiers a row holds: the group every ordinary tier
// is, and the card the recommended one is. They stand at different levels,
// so a checkmark's ink is derived twice.
var tierKinds = []struct {
	name string
	tier Tier
}{
	{"group tier", Tier{}},
	{"recommended card", Tier{Recommended: true}},
}

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

// TestCheckInkClearsTheGraphicFloorForEverySeed holds the invariant that
// whatever a caller seeds the palette with, the ink drawn directly on a
// tier — the feature checkmarks — reaches WCAG 1.4.11 against that tier's
// own fill, on the group and on the recommended card alike.
func TestCheckInkClearsTheGraphicFloorForEverySeed(t *testing.T) {
	worstLight, worstDark := 99.0, 99.0
	var worstLightAt, worstDarkAt string
	for _, seed := range tierInkSweepSeeds() {
		for _, s := range tierInkSweepSchemes(seed) {
			for _, k := range tierKinds {
				fill := tierFill(s.tok, k.tier)
				ink := checkInk(s.tok, fill)
				got := color.ContrastRatio(ink, fill)
				if got < tokens.GraphicFloor {
					t.Errorf("seed %s: %s: %s: check ink %s on %s measures %.2f:1, under the %.1f:1 graphic floor",
						tierInkHex(seed), s.name, k.name, tierInkHex(ink), tierInkHex(fill), got, tokens.GraphicFloor)
				}
				if s.light && got < worstLight {
					worstLight, worstLightAt = got, tierInkHex(seed)
				}
				if !s.light && got < worstDark {
					worstDark, worstDarkAt = got, tierInkHex(seed)
				}
			}
		}
	}
	t.Logf("over %d seeds: worst light check ink %.2f:1 (%s), worst dark check ink %.2f:1 (%s)",
		len(tierInkSweepSeeds()), worstLight, worstLightAt, worstDark, worstDarkAt)
}

// TestTheCanonicalSeedsCheckInkIsThePrimaryPin holds the invariant that on
// the seed every golden is rendered from, the brand's own colour clears the
// floor on both tier surfaces and is what the checkmarks get.
func TestTheCanonicalSeedsCheckInkIsThePrimaryPin(t *testing.T) {
	for _, s := range []struct {
		name string
		tok  tokens.ColorTokens
	}{
		{"DefaultLight", tokens.DefaultLight},
		{"DefaultDark", tokens.DefaultDark},
	} {
		for _, k := range tierKinds {
			if ink := checkInk(s.tok, tierFill(s.tok, k.tier)); ink != s.tok.Primary {
				t.Errorf("%s: %s: check ink is %s, not the Primary pin %s — a golden moved",
					s.name, k.name, tierInkHex(ink), tierInkHex(s.tok.Primary))
			}
		}
	}
}

// TestAPastelSeedsCheckInkLeavesThePin holds the invariant on a light
// scheme seeded with a dark scheme's accent: the bare pin sits under the
// graphic floor on the recommended card, so the checkmarks there must not
// be the bare pin.
func TestAPastelSeedsCheckInkLeavesThePin(t *testing.T) {
	seed := stdcolor.NRGBA{0x89, 0xb4, 0xfa, 0xff}
	light, dark := tokens.FromSeed(seed)
	recommended := Tier{Recommended: true}

	lightFill := tierFill(light, recommended)
	if bare := color.ContrastRatio(light.Primary, lightFill); bare >= tokens.GraphicFloor {
		t.Fatalf("this seed's bare light pin now measures %.2f:1 on the card — the test no longer reads the shape it was written for", bare)
	}
	if lightInk := checkInk(light, lightFill); lightInk == light.Primary {
		t.Errorf("light check ink is still the bare pin %s", tierInkHex(light.Primary))
	}

	darkInk := checkInk(dark, tierFill(dark, recommended))
	if darkInk != dark.Primary {
		t.Errorf("dark check ink walked to %s; the dark pin %s clears its card and should stand",
			tierInkHex(darkInk), tierInkHex(dark.Primary))
	}
}
