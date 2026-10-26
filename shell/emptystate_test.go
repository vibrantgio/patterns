package shell_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
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
		return shell.EmptyState(gtx, sh, c, tokens.DefaultTypography, fill,
			shell.EmptyStateProps{Line: emptyStateText})
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

// emptyStateDescription and the action below are the two parts that may
// stand under the line. The description is short enough not to wrap in
// either frame the group tests use, so its height is the same in both and
// the block moves only because the frame grew.
const emptyStateDescription = "Choose a message in the list to read it here."

// emptyStateActionSize is the box the probe control takes and
// emptyStateActionColor what it paints, a colour no glyph in either
// appearance comes near, so the control's own rectangle can be read straight
// off the rendering and held against the place the group puts it.
var (
	emptyStateActionSize  = image.Pt(120, 24)
	emptyStateActionColor = color.NRGBA{R: 0xff, A: 0xff}
)

// TestEmptyStateLineAloneIsTheColumnsReading holds the grown pattern against
// what the column had before it: a props value carrying nothing but the line
// renders the same pixels as the line laid out by itself in the frame — the
// EmptyStateLine role, no line height, TertiaryLabel over the
// fill, centred by the frame's own floor. Every pixel is compared, so a
// group that moved the line by one row fails here.
func TestEmptyStateLineAloneIsTheColumnsReading(t *testing.T) {
	sh := defaultShaper(t)
	for _, tc := range emptyStateCases {
		t.Run(tc.name, func(t *testing.T) {
			fill := tc.colors.TextBackground
			style := tokens.DefaultTypography.EmptyStateLine
			style.LineHeight = 0
			got := emptyStateImage(t, sh, tc.colors, fill, emptyStateFrame)
			want := emptyStateLineOnly(t, sh, tc.colors, fill, style, emptyStateFrame)
			if n := golden.PixelDiff(want, got); n != 0 {
				t.Errorf("the line alone renders %d pixel(s) away from the line laid out by itself", n)
			}
		})
	}
}

// TestPaneSizeSetsTheLineInTheRowsStyle holds the other size the props
// carries: a section or a field handing in the style of the rows it stands
// for gets the line set in that style and nothing else about the pattern
// changes. The rendering is compared with the same line laid out by itself
// in that style, every pixel, and then with the column's own rendering,
// which it must differ from — otherwise the style handed in was dropped and
// the first comparison would be holding the pattern against itself.
func TestPaneSizeSetsTheLineInTheRowsStyle(t *testing.T) {
	sh := defaultShaper(t)
	rows := tokens.DefaultTypography.Body
	for _, tc := range emptyStateCases {
		t.Run(tc.name, func(t *testing.T) {
			fill := tc.colors.TextBackground
			got := golden.Capture(t, emptyStateFrame, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, fill, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return shell.EmptyState(gtx, sh, tc.colors, tokens.DefaultTypography, fill,
					shell.EmptyStateProps{Line: emptyStateText, LineStyle: rows})
			})
			want := emptyStateLineOnly(t, sh, tc.colors, fill, rows, emptyStateFrame)
			if n := golden.PixelDiff(want, got); n != 0 {
				t.Errorf("the pane's line renders %d pixel(s) away from the rows' own style", n)
			}
			column := emptyStateImage(t, sh, tc.colors, fill, emptyStateFrame)
			if golden.PixelDiff(column, got) == 0 {
				t.Error("the pane's line renders the column's measured size, want the rows' style")
			}
		})
	}
}

// TestEmptyStateCentresTheGroupItIsGiven holds the three parts together: a
// line with a description and one action centre in the frame as one block,
// not one part at a time.
//
// It is read off the action, because the action is the only part whose box
// is its own: the control paints a rectangle of a size this test states, so
// the place the group put it is exact where a glyph box carries the slack
// the face leaves above and below the text. The block's height is the three
// parts' heights and the two gaps, each measured the way the pattern lays
// it, so the place the action must stand is arithmetic rather than a
// recorded number.
//
// The reading is taken twice, in a frame and in one grown by an even amount
// on both axes, because a block standing in the middle of one frame could be
// standing at a fixed offset instead: a centred block moves by exactly half
// the growth and a block placed any other way does not.
func TestEmptyStateCentresTheGroupItIsGiven(t *testing.T) {
	sh := defaultShaper(t)
	line := tokens.DefaultTypography.EmptyStateLine
	line.LineHeight = 0
	for _, tc := range emptyStateCases {
		t.Run(tc.name, func(t *testing.T) {
			fill := tc.colors.TextBackground
			grown := image.Pt(emptyStateFrame.X+emptyStateGrowth.X, emptyStateFrame.Y+emptyStateGrowth.Y)
			small := emptyStateGroupImage(t, sh, tc.colors, fill, emptyStateFrame)
			large := emptyStateGroupImage(t, sh, tc.colors, fill, grown)

			lineH := emptyStateHeightOf(t, sh, line, emptyStateText, emptyStateFrame, 0)
			descH := emptyStateHeightOf(t, sh, tokens.DefaultTypography.Body,
				emptyStateDescription, emptyStateFrame, 0)
			// The metric the captures are taken under is one pixel per dp,
			// so the two gaps convert to their own numbers.
			gaps := int(shell.EmptyStateDescriptionGap) + int(shell.EmptyStateActionsGap)
			block := lineH + descH + gaps + emptyStateActionSize.Y
			if block >= emptyStateFrame.Y {
				t.Fatalf("the block measures %d in a frame %d deep; the frame no longer holds it",
					block, emptyStateFrame.Y)
			}

			want := image.Rectangle{
				Min: image.Pt((emptyStateFrame.X-emptyStateActionSize.X)/2,
					(emptyStateFrame.Y-block)/2+lineH+descH+gaps),
			}
			want.Max = want.Min.Add(emptyStateActionSize)
			action := boxOfColor(small, emptyStateActionColor)
			if action != want {
				t.Errorf("the action stands at %v, want %v — the block centred in the frame", action, want)
			}

			moved := boxOfColor(large, emptyStateActionColor)
			if got, want := moved.Min.X-action.Min.X, emptyStateGrowth.X/2; got != want {
				t.Errorf("a frame %d wider moved the block %d across, want half the growth, %d",
					emptyStateGrowth.X, got, want)
			}
			if got, want := moved.Min.Y-action.Min.Y, emptyStateGrowth.Y/2; got != want {
				t.Errorf("a frame %d deeper moved the block %d down, want half the growth, %d",
					emptyStateGrowth.Y, got, want)
			}
		})
	}
}

// emptyStateGroupImage renders the line, the description and one action on
// the fill they stand on.
func emptyStateGroupImage(t *testing.T, sh *text.Shaper, c tokens.PlatformColors,
	fill color.NRGBA, size image.Point) *image.RGBA {
	t.Helper()
	action := func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, emptyStateActionColor,
			clip.Rect{Max: emptyStateActionSize}.Op())
		return layout.Dimensions{Size: emptyStateActionSize}
	}
	return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, fill, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return shell.EmptyState(gtx, sh, c, tokens.DefaultTypography, fill, shell.EmptyStateProps{
			Line:        emptyStateText,
			Description: emptyStateDescription,
			Actions:     []layout.Widget{action},
		})
	})
}

// emptyStateLineOnly renders the line by itself in the frame, in the style
// handed over, the way a column stood it before the props value existed:
// one line, TertiaryLabel over the fill, centred by the frame's own floor.
func emptyStateLineOnly(t *testing.T, sh *text.Shaper, c tokens.PlatformColors,
	fill color.NRGBA, style tokens.TextStyle, size image.Point) *image.RGBA {
	t.Helper()
	return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, fill, clip.Rect{Max: gtx.Constraints.Max}.Op())
		gtx.Constraints = layout.Exact(size)
		typeset.TextAligned(gtx, sh, emptyStateText, style,
			vgcolor.Flatten(c.TertiaryLabel, fill), 0, font.Normal, text.Middle)
		return layout.Dimensions{Size: size}
	})
}

// emptyStateHeightOf is the height one part of the group takes: the text
// laid out across the frame's width with no floor under it, which is the
// box the pattern stacks.
func emptyStateHeightOf(t *testing.T, sh *text.Shaper, style tokens.TextStyle,
	txt string, frame image.Point, maxLines int) int {
	t.Helper()
	h := 0
	golden.Capture(t, frame, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Constraints{Min: image.Pt(frame.X, 0), Max: frame}
		h = typeset.TextAligned(gtx, sh, txt, style, color.NRGBA{A: 0xff}, maxLines,
			font.Normal, text.Middle).Size.Y
		return layout.Dimensions{Size: frame}
	})
	return h
}

// boxOfColor is the bounding box of every pixel painted exactly col.
func boxOfColor(img *image.RGBA, col color.NRGBA) image.Rectangle {
	box := image.Rectangle{}
	first := true
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if c := img.RGBAAt(x, y); c.R != col.R || c.G != col.G || c.B != col.B {
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

// longEmptyStateLine names a query far wider than the narrow frame below, set
// between the platform's quotation marks as typed.
const longEmptyStateLine = "No Results for \u201ca query typed far longer than the column is wide\u201d"

// TestALineWiderThanTheFrameIsSetOnMoreLines renders a line far wider than a
// narrow frame and reads it back: it is taller than one line, it stays inside
// the frame's sides and centred in it, and two lines that differ only at
// their ends draw differently, so no part of the line was left out.
func TestALineWiderThanTheFrameIsSetOnMoreLines(t *testing.T) {
	sh := defaultShaper(t)
	frame := image.Pt(240, 400)
	c := tokens.PlatformLight
	fill := c.TextBackground
	render := func(line string) *image.RGBA {
		return golden.Capture(t, frame, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, fill, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return shell.EmptyState(gtx, sh, c, tokens.DefaultTypography, fill,
				shell.EmptyStateProps{Line: line})
		})
	}
	short := glyphBox(render(emptyStateText[:2]), fill)
	img := render(longEmptyStateLine)
	box := glyphBox(img, fill)
	if box.Empty() || short.Empty() {
		t.Fatal("the line painted nothing at all")
	}
	if box.Min.X < 1 || box.Max.X > frame.X-1 {
		t.Errorf("the line spans columns %d..%d, against the frame's 0..%d", box.Min.X, box.Max.X-1, frame.X-1)
	}
	if box.Dy() < 2*short.Dy() {
		t.Errorf("the line is %d tall against %d for one line; it was not set on more lines", box.Dy(), short.Dy())
	}
	assertCentred(t, "a narrow frame", box, frame, 2, 2)

	other := longEmptyStateLine[:len(longEmptyStateLine)-len("wide\u201d")] + "broad\u201d"
	if golden.PixelDiff(img, render(other)) == 0 {
		t.Error("two lines that differ only at their ends drew the same; the end was left out")
	}
}
