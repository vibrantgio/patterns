package shell_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// emptyStateText is the platform's own line for a content column with
// nothing in it, which is what the stored reading was taken off, so the box
// this test reads back can be held against that reading: the capture gives
// the glyphs a box 267×26 at 1x.
const emptyStateText = "No Message Selected"

// emptyStateFrame is the frame the assertions below are made in, and
// emptyStateGrowth the amount the second frame is larger on both axes. It
// is even on both axes so that half of it is a whole pixel: a line centred
// in a frame grown by it stands exactly half of it further in on each axis,
// and the rounding is the same in both frames.
var (
	emptyStateFrame  = image.Pt(600, 400)
	emptyStateGrowth = image.Pt(40, 60)
)

// emptyStateCases are the two appearances. The fill is the column's own —
// what the pattern flattens the label colour over.
var emptyStateCases = []struct {
	name   string
	colors tokens.PlatformColors
}{
	{"light", tokens.PlatformLight},
	{"dark", tokens.PlatformDark},
}

// TestEmptyStateCentresInTheFrameItIsGiven reads the line back off a
// rendering of it and holds it against the two halves of the pattern's
// contract: the glyphs stand centred on both axes of the frame the call was
// given, and they are painted in TertiaryLabel flattened over the fill
// under them.
//
// The centring is read twice, in a frame and in a larger one, because a
// line standing in the middle of one frame could be standing at a fixed
// offset instead: a frame grown by an even amount moves a centred line by
// exactly half the growth, and a line placed any other way does not move
// that way.
func TestEmptyStateCentresInTheFrameItIsGiven(t *testing.T) {
	sh := defaultShaper(t)
	for _, tc := range emptyStateCases {
		t.Run(tc.name, func(t *testing.T) {
			fill := tc.colors.TextBackground
			small := emptyStateGlyphBox(t, sh, tc.colors, fill, emptyStateFrame)
			grown := image.Pt(emptyStateFrame.X+emptyStateGrowth.X, emptyStateFrame.Y+emptyStateGrowth.Y)
			large := emptyStateGlyphBox(t, sh, tc.colors, fill, grown)

			if small.Empty() {
				t.Fatal("the frame carries no glyphs at all, want the line the call was given")
			}
			// The glyph box is the painted glyphs' own and the line box
			// is what the pattern centres, so the two centres stand within
			// what the descenders reach below the box's middle. One pixel
			// across and two down is the whole of that difference at this
			// size; a line standing on a lead rather than in the middle is
			// hundreds of pixels out.
			assertCentred(t, "the frame", small, emptyStateFrame, 1, 2)
			assertCentred(t, "the larger frame", large, grown, 1, 2)

			if got, want := large.Min.X-small.Min.X, emptyStateGrowth.X/2; got != want {
				t.Errorf("a frame %d wider moved the line %d across, want half the growth, %d",
					emptyStateGrowth.X, got, want)
			}
			if got, want := large.Min.Y-small.Min.Y, emptyStateGrowth.Y/2; got != want {
				t.Errorf("a frame %d deeper moved the line %d down, want half the growth, %d",
					emptyStateGrowth.Y, got, want)
			}
		})
	}
}

// TestTertiaryLabelOverTheColumnIsTheCaptureReading holds the pair of
// values the colour above is built from against the stored capture: the
// platform's tertiary label over a white content column lands on the
// #bdbdbd the glyphs plateau at there.
func TestTertiaryLabelOverTheColumnIsTheCaptureReading(t *testing.T) {
	got := vgcolor.Flatten(tokens.PlatformLight.TertiaryLabel, tokens.PlatformLight.TextBackground)
	if want := (color.NRGBA{R: 0xbd, G: 0xbd, B: 0xbd, A: 0xff}); got != want {
		t.Errorf("TertiaryLabel over the column is #%02x%02x%02x, want the capture's #bdbdbd",
			got.R, got.G, got.B)
	}
}

// TestEmptyStateSpendsTertiaryLabel pins the colour the glyphs plateau at:
// the platform's tertiary label flattened over the column's own fill, which
// in the light appearance is the #bdbdbd the stored capture reads. A fully
// covered pixel is painted the flattened colour exactly, so the plateau is
// an equality and not a tolerance.
func TestEmptyStateSpendsTertiaryLabel(t *testing.T) {
	sh := defaultShaper(t)
	for _, tc := range emptyStateCases {
		t.Run(tc.name, func(t *testing.T) {
			fill := tc.colors.TextBackground
			img := emptyStateImage(t, sh, tc.colors, fill, emptyStateFrame)
			want := vgcolor.Flatten(tc.colors.TertiaryLabel, fill)
			got := plateauOf(img, fill)
			if got.R != want.R || got.G != want.G || got.B != want.B {
				t.Errorf("the glyphs plateau at #%02x%02x%02x, want TertiaryLabel over the fill, #%02x%02x%02x",
					got.R, got.G, got.B, want.R, want.G, want.B)
			}
		})
	}
}

// assertCentred holds a glyph box's centre against its frame's, within the
// difference the glyphs' own extents account for.
func assertCentred(t *testing.T, what string, box image.Rectangle, frame image.Point, tolX, tolY int) {
	t.Helper()
	// Doubled, so a centre landing between two pixels is compared without
	// a rounding of its own.
	if got, want := box.Min.X+box.Max.X, frame.X; abs(got-want) > 2*tolX {
		t.Errorf("in %s the line's glyphs centre at %.1f across, want the frame's %.1f",
			what, float64(got)/2, float64(want)/2)
	}
	if got, want := box.Min.Y+box.Max.Y, frame.Y; abs(got-want) > 2*tolY {
		t.Errorf("in %s the line's glyphs centre at %.1f down, want the frame's %.1f",
			what, float64(got)/2, float64(want)/2)
	}
}

// emptyStateImage renders the pattern alone on the fill it stands on.
func emptyStateImage(t *testing.T, sh *text.Shaper, c tokens.PlatformColors,
	fill color.NRGBA, size image.Point) *image.RGBA {
	t.Helper()
	return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, fill, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return shell.EmptyState(gtx, sh, c, tokens.DefaultTypography, fill, emptyStateText)
	})
}

// emptyStateGlyphBox is the box the painted glyphs occupy in that rendering:
// every pixel the fill does not account for, which at a size this far above
// the body roles is the line and nothing else.
func emptyStateGlyphBox(t *testing.T, sh *text.Shaper, c tokens.PlatformColors,
	fill color.NRGBA, size image.Point) image.Rectangle {
	t.Helper()
	return glyphBox(emptyStateImage(t, sh, c, fill, size), fill)
}

// glyphBox is the bounding box of everything in img that is not fill.
func glyphBox(img *image.RGBA, fill color.NRGBA) image.Rectangle {
	b := img.Bounds()
	box := image.Rectangle{}
	first := true
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if c := img.RGBAAt(x, y); c.R == fill.R && c.G == fill.G && c.B == fill.B {
				continue
			}
			if first {
				box, first = image.Rect(x, y, x+1, y+1), false
				continue
			}
			box = box.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return box
}

// plateauOf is the painted colour furthest from the fill, which is the
// colour of the pixels the glyphs cover completely.
func plateauOf(img *image.RGBA, fill color.NRGBA) color.RGBA {
	var found color.RGBA
	far := -1
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			d := abs(int(c.R)-int(fill.R)) + abs(int(c.G)-int(fill.G)) + abs(int(c.B)-int(fill.B))
			if d > far {
				far, found = d, c
			}
		}
	}
	return found
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
