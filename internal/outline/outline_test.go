// The outline's pairing, measured rather than eyeballed: the line a pattern
// draws around its own surface, against the fill it circles and against every
// plane that surface can stand on.
package outline_test

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/vibrantgio/patterns/internal/outline"
	themecolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

func hex(c color.NRGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// storeys is the whole elevation ladder, and therefore the whole set of
// grounds any outline can be drawn against.
var storeys = []struct {
	name  string
	level tokens.ElevationLevel
}{
	{"level-0", tokens.Level0},
	{"level-1", tokens.Level1},
	{"level-2", tokens.Level2},
	{"level-3", tokens.Level3},
}

// Two grounds are measured per storey, because a line has two sides. Inside
// is the fill the pattern paints and therefore always knows; outside is the
// plane it stands on, which it does not — so the sweep holds the ink against
// every plane no deeper than its own fill, which is every plane a raised
// surface can be standing on.
func TestOutlineInkClearsTheGraphicFloor(t *testing.T) {
	for _, sc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"light", tokens.DefaultLight},
		{"dark", tokens.DefaultDark},
	} {
		t.Run(sc.name, func(t *testing.T) {
			c := sc.colors
			for _, fill := range storeys {
				ink := outline.Ink(c, c.SurfaceAt(fill.level))
				for _, plane := range storeys {
					if plane.level > fill.level {
						continue // deeper than the fill: not a plane this surface stands on
					}
					ground := c.SurfaceAt(plane.level)
					got := themecolor.ContrastRatio(ink, ground)
					t.Logf("%s outline %s against the %s plane %s: %.2f:1",
						fill.name, hex(ink), plane.name, hex(ground), got)
					if got < outline.Floor {
						t.Errorf("%s outline %s against the %s plane %s = %.2f:1, want at least %.1f:1",
							fill.name, hex(ink), plane.name, hex(ground), got, outline.Floor)
					}
				}
			}
		})
	}
}

// seeds is the spread the pairing is held over, because a palette is
// generated and the defaults are only one of its outputs: the default seed,
// the six saturated corners of sRGB, a seed with no chroma at all, and the
// two ends of the lightness range.
var seeds = []color.NRGBA{
	{R: 0x6c, G: 0x3a, B: 0xd4, A: 0xff}, // the default seed
	{R: 0xff, A: 0xff},
	{G: 0xff, A: 0xff},
	{B: 0xff, A: 0xff},
	{R: 0xff, G: 0xff, A: 0xff},
	{G: 0xff, B: 0xff, A: 0xff},
	{R: 0xff, G: 0x80, A: 0xff},
	{R: 0x80, G: 0x80, B: 0x80, A: 0xff},
	{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	{A: 0xff},
}

// TestOutlineInkClearsTheFloorForEverySeed walks the same pairings over a
// spread of seeds and both contrast variants: the neutral ramps carry the
// seed's tint, so the measurements move from seed to seed but the verdict
// may not.
func TestOutlineInkClearsTheFloorForEverySeed(t *testing.T) {
	worst := 99.0
	for _, seed := range seeds {
		light, dark := tokens.FromSeed(seed)
		lightHC, darkHC := tokens.FromSeedHighContrast(seed)
		for _, sc := range []struct {
			name   string
			colors tokens.ColorTokens
		}{
			{"light", light},
			{"dark", dark},
			{"light high-contrast", lightHC},
			{"dark high-contrast", darkHC},
		} {
			c := sc.colors
			for _, fill := range storeys {
				ink := outline.Ink(c, c.SurfaceAt(fill.level))
				for _, plane := range storeys {
					if plane.level > fill.level {
						continue
					}
					ground := c.SurfaceAt(plane.level)
					got := themecolor.ContrastRatio(ink, ground)
					if got < worst {
						worst = got
					}
					if got < outline.Floor {
						t.Errorf("seed %s %s: %s outline %s against the %s plane %s = %.2f:1, want at least %.1f:1",
							hex(seed), sc.name, fill.name, hex(ink), plane.name, hex(ground), got, outline.Floor)
					}
				}
			}
		}
	}
	t.Logf("worst outline pairing over the sweep: %.2f:1", worst)
}
