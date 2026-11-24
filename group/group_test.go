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
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

const (
	frameW, frameH = 280, 200
	// The group draws into its full constraints, so a golden that must
	// show the hairline against the surface the group is in insets it by
	// this margin.
	marginPx = 16
)

var (
	frameSize = image.Pt(frameW, frameH)
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

// textSlot returns a content layout.Widget that draws s in the given role. A group
// draws no text but its own title, so everything it holds is caller-built.
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

// content is the pair of layout.Widget values every group case holds.
func content(t *testing.T, c tokens.PlatformColors) []layout.Widget {
	t.Helper()
	shaper := defaultShaper(t)
	typo := tokens.DefaultTypography
	return []layout.Widget{
		textSlot(shaper, typo.Callout, vgcolor.Flatten(c.Label, c.ControlBackground), 1, "Comfortable"),
		textSlot(shaper, typo.Callout, vgcolor.Flatten(c.SecondaryLabel, c.ControlBackground), 2,
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
// schemes, with a title and without. The scene fills the content surface so the
// group's interior and its surroundings are one fill, which is the whole
// point of the pattern: only the hairline says where it ends.
func TestGroupGolden(t *testing.T) {
	cases := []struct {
		name   string
		colors tokens.PlatformColors
		title  string
		count  string
	}{
		{name: "light-with-title", colors: tokens.PlatformLight, title: "Density"},
		{name: "dark-with-title", colors: tokens.PlatformDark, title: "Density"},
		{name: "light-without-title", colors: tokens.PlatformLight},
		{name: "dark-without-title", colors: tokens.PlatformDark},
		{name: "light-with-count", colors: tokens.PlatformLight, title: "Density", count: "2"},
		{name: "dark-with-count", colors: tokens.PlatformDark, title: "Density", count: "2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := group.Render(defaultShaper(t),
				group.Props{Title: tc.title, Count: tc.count, Content: content(t, tc.colors)},
				tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography.SectionHeading)
			golden.Render(t, tc.name, frameSize, scene(w, marginPx, tc.colors.ControlBackground))
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
		colors tokens.PlatformColors
	}{
		{"light", tokens.PlatformLight},
		{"dark", tokens.PlatformDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.colors.ControlBackground
			w := group.Render(defaultShaper(t), group.Props{}, tc.colors,
				tokens.Spacing, sharpRadius, tokens.DefaultTypography.SectionHeading)
			img := golden.Capture(t, frameSize, scene(w, marginPx, want))
			r, g, b, _ := img.At(frameW/2, frameH/2).RGBA()
			if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B {
				t.Errorf("inside the group is #%02x%02x%02x, want the surface's own #%02x%02x%02x",
					uint8(r>>8), uint8(g>>8), uint8(b>>8), want.R, want.G, want.B)
			}
		})
	}
}

// TestGroupHairlineIsTheSeam holds the mapping in pixels: the line at the
// group's edge is the platform's separator flattened onto the surface the
// group is in, and not the 3:1 mark a graphic carrying meaning would owe.
func TestGroupHairlineIsTheSeam(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.PlatformColors
	}{
		{"light", tokens.PlatformLight},
		{"dark", tokens.PlatformDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bg := tc.colors.ControlBackground
			want := vgcolor.Flatten(tc.colors.Separator, bg)
			w := group.Render(defaultShaper(t), group.Props{}, tc.colors,
				tokens.Spacing, sharpRadius, tokens.DefaultTypography.SectionHeading)
			img := golden.Capture(t, frameSize, scene(w, marginPx, bg))
			// Sharp corners, so the leading edge is exactly one column of
			// hairline at the margin and nothing is anti-aliased into it.
			r, g, b, _ := img.At(marginPx, frameH/2).RGBA()
			if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B {
				t.Errorf("hairline is #%02x%02x%02x, want the seam #%02x%02x%02x",
					uint8(r>>8), uint8(g>>8), uint8(b>>8), want.R, want.G, want.B)
			}
		})
	}
}

// TestGroupTitleChangesPixels confirms the title is drawn: a group without
// a title and one with a title differ, so a Title that never reached the shaper
// cannot pass silently.
func TestGroupTitleChangesPixels(t *testing.T) {
	c := tokens.PlatformLight
	shaper := defaultShaper(t)
	bare := group.Render(shaper, group.Props{Content: content(t, c)}, c,
		tokens.Spacing, sharpRadius, tokens.DefaultTypography.SectionHeading)
	named := group.Render(shaper, group.Props{Title: "Density", Content: content(t, c)}, c,
		tokens.Spacing, sharpRadius, tokens.DefaultTypography.SectionHeading)
	a := golden.Capture(t, frameSize, scene(bare, marginPx, c.ControlBackground))
	b := golden.Capture(t, frameSize, scene(named, marginPx, c.ControlBackground))
	if n := golden.PixelDiff(a, b); n == 0 {
		t.Error("groups with and without a title render identically; expected the title")
	}
}

// TestGroupCountAlwaysShows confirms that a title too long for the row is
// cut short before the count: the row with the count differs from the row
// without it, so the count reached the frame.
func TestGroupCountAlwaysShows(t *testing.T) {
	// The title and the count are told apart by hue, so the test reads
	// where each one is drawn.
	c := tokens.PlatformLight
	c.Label = color.NRGBA{B: 0xff, A: 0xff}
	c.SecondaryLabel = color.NRGBA{R: 0xff, A: 0xff}
	long := "A title much longer than the row of this group can show"
	counted := group.Render(defaultShaper(t), group.Props{Title: long, Count: "12"}, c,
		tokens.Spacing, sharpRadius, tokens.DefaultTypography.SectionHeading)
	img := golden.Capture(t, frameSize, scene(counted, marginPx, c.ControlBackground))

	// The row inside the group's padding.
	left := marginPx + int(tokens.Spacing.S4)
	right := frameW - marginPx - int(tokens.Spacing.S4)
	countMin, countMax, titleMax := frameW, -1, -1
	for y := range frameH {
		for x := range frameW {
			p := img.RGBAAt(x, y)
			switch {
			case p.R > 200 && p.G < 100 && p.B < 100:
				countMin, countMax = min(countMin, x), max(countMax, x)
			case p.B > 200 && p.R < 100 && p.G < 100:
				titleMax = max(titleMax, x)
			}
		}
	}
	if countMax < 0 {
		t.Fatal("no count pixels drawn; expected the count after a long title")
	}
	if titleMax < 0 {
		t.Fatal("no title pixels drawn in the label colour")
	}
	if countMin < left || countMax >= right {
		t.Errorf("count drawn at x %d-%d; expected it inside the row at x %d-%d", countMin, countMax, left, right-1)
	}
	if countMin <= titleMax {
		t.Errorf("count starts at x %d, at or before the title's end at x %d; expected it after the title", countMin, titleMax)
	}
}

// TestGroupScrollFrameReachesTheEdge checks the frame the group gives its
// scrolled content: it starts under the title, it reaches the hairline on the
// trailing side and at the bottom, it never covers the hairline, and its pad
// is the group's padding with the S3 gap on top.
func TestGroupScrollFrameReachesTheEdge(t *testing.T) {
	c := tokens.PlatformLight
	mark := color.NRGBA{R: 0xff, A: 0xff}
	var got layout.Inset
	scroll := func(gtx layout.Context, pad layout.Inset) layout.Dimensions {
		got = pad
		paint.FillShape(gtx.Ops, mark, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	g := group.Render(defaultShaper(t), group.Props{Title: "Things", Count: "3", Scroll: scroll}, c,
		tokens.Spacing, sharpRadius, tokens.DefaultTypography.SectionHeading)
	img := golden.Capture(t, frameSize, scene(g, marginPx, c.ControlBackground))

	want := layout.Inset{
		Top:    unit.Dp(tokens.Spacing.S3),
		Bottom: unit.Dp(tokens.Spacing.S4),
		Left:   unit.Dp(tokens.Spacing.S4),
		Right:  unit.Dp(tokens.Spacing.S4),
	}
	if got != want {
		t.Errorf("pad = %+v; expected %+v", got, want)
	}
	isMark := func(x, y int) bool { return img.RGBAAt(x, y) == color.RGBA{R: 0xff, A: 0xff} }
	// The group's box is the frame less the scene's margin; its hairline is
	// the box's outermost pixel.
	x0, y0, x1, y1 := marginPx, marginPx, frameW-marginPx-1, frameH-marginPx-1
	top := -1
	for y := y0; y <= y1 && top < 0; y++ {
		if isMark(x1-1, y) {
			top = y
		}
	}
	titleFoot := y0 + int(tokens.Spacing.S4) + int(tokens.DefaultTypography.SectionHeading.LineHeight)
	if top < titleFoot {
		t.Errorf("frame starts at y %d; expected it under the title, at y %d or below", top, titleFoot)
	}
	if !isMark(x1-1, y1-1) || !isMark(x0+1, y1-1) {
		t.Error("frame does not reach the pixel inside the hairline at the bottom corners")
	}
	for y := y0; y <= y1; y++ {
		if isMark(x1, y) || isMark(x0, y) {
			t.Fatalf("frame covers the hairline at y %d", y)
		}
	}
	for x := x0; x <= x1; x++ {
		if isMark(x, y1) {
			t.Fatalf("frame covers the bottom hairline at x %d", x)
		}
	}
}

// TestGroupTitleIsTheSectionHeadingRole pins the role a group's title is set
// in: the Section heading role, whole, from the typography it is given.
func TestGroupTitleIsTheSectionHeadingRole(t *testing.T) {
	typ := tokens.DefaultTypography
	if got := group.TitleStyle(typ); got != typ.SectionHeading {
		t.Errorf("TitleStyle = %+v, want the Section heading role %+v", got, typ.SectionHeading)
	}
}
