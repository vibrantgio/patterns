package pane_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/pane"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// The window the goldens draw: wide enough that the pane is a column
// beside a document rather than half the window, deep enough that both of
// the pane's ends are visible with the backdrop showing past them.
const (
	windowW = 420
	windowH = 260
	paneW   = 160
)

var windowSize = image.Pt(windowW, windowH)

// themeCases are the two schemes every derivation here is checked in.
// Nothing in this package names a scheme, so a rule that held in one and
// not the other would be a rule that is not derived.
var themeCases = []struct {
	name   string
	colors tokens.PlatformColors
}{
	{"light", tokens.PlatformLight},
	{"dark", tokens.PlatformDark},
}

// ctx is a bare layout context at one pixel per dp, which is what the
// geometry assertions below read: at that scale a dp and a pixel are the
// same number, so the arithmetic under test is visible in the numbers.
func ctx(ops *op.Ops, size image.Point) layout.Context {
	gtx := layout.Context{Ops: ops}
	gtx.Metric.PxPerDp = 1
	gtx.Metric.PxPerSp = 1
	gtx.Constraints = layout.Exact(size)
	return gtx
}

func scene(w layout.Widget, bg color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// TestBoundsRunsToTheWindowsEdges reads the whole of the column's
// geometry: the window's own leading, top and bottom edges, the asked-for
// width, and nothing between the column and any of them. MEASURED off
// Finder's untinted captures, where the sidebar's fill runs to the window's
// bounds on three sides and gives way to the content on the fourth.
func TestBoundsRunsToTheWindowsEdges(t *testing.T) {
	var ops op.Ops
	gtx := ctx(&ops, windowSize)
	got := pane.Bounds(gtx, windowSize, paneW, false)
	want := image.Rect(0, 0, paneW, windowH)
	if got != want {
		t.Errorf("the column stands at %v, want %v — the window's own leading, top and bottom edges", got, want)
	}
}

// TestHiddenTakesNoWidth is the whole of the hidden contract: there is no
// collapsed rail to reason about, no residual column, and no flag for the
// caller to read beside the rectangle. The pane is simply not there, and
// what stood beside it starts at the window's own edge.
func TestHiddenTakesNoWidth(t *testing.T) {
	var ops op.Ops
	gtx := ctx(&ops, windowSize)
	if got := pane.Bounds(gtx, windowSize, paneW, true); !got.Empty() {
		t.Errorf("hidden, the pane occupies %v; it takes no width at all", got)
	}
	// And nothing is drawn for it either, so a caller that leans on Layout
	// rather than on the rectangle gets the same answer: no edge, no fill,
	// and the contents never run.
	col := tokens.PlatformLight
	backdrop := vgcolor.Flatten(col.UnderPageBackground, col.WindowBackground)
	w := func(gtx layout.Context) layout.Dimensions {
		pane.Layout(gtx, col, pane.Bounds(gtx, gtx.Constraints.Max, paneW, true),
			func(gtx layout.Context) layout.Dimensions {
				t.Error("a dismissed pane laid out its contents")
				return layout.Dimensions{}
			})
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	img := golden.Capture(t, windowSize, scene(w, backdrop))
	for x := 0; x < windowW; x++ {
		for y := 0; y < windowH; y++ {
			if got := img.RGBAAt(x, y); !sameColor(got, backdrop) {
				t.Fatalf("a dismissed pane painted %v at (%d,%d); the window shows the bare backdrop", got, x, y)
			}
		}
	}
}

// TestBoundsNeverTakesMoreThanHalfTheWindow: a narrow window owes its
// document a readable column before it owes the pane its width, and a
// window with no room to set anything into gets no pane at all.
func TestBoundsNeverTakesMoreThanHalfTheWindow(t *testing.T) {
	var ops op.Ops
	narrow := image.Pt(200, windowH)
	gtx := ctx(&ops, narrow)
	got := pane.Bounds(gtx, narrow, 400, false)
	if want := narrow.X / 2; got.Dx() != want {
		t.Errorf("in a %d-wide window the column took %d, want the half-window clamp %d", narrow.X, got.Dx(), want)
	}
	for _, tc := range []struct {
		what string
		size image.Point
	}{
		{"no area", image.Pt(0, 0)},
		{"no width to stand in", image.Pt(1, windowH)},
	} {
		sgtx := ctx(&ops, tc.size)
		if got := pane.Bounds(sgtx, tc.size, paneW, false); !got.Empty() {
			t.Errorf("a window with %s laid out a column at %v", tc.what, got)
		}
	}
}

// TestStripHoldsTheWindowButtons pins the strip's arithmetic against the
// run it is cut for. The buttons are measured from the window's own glass
// and the strip from the column's top edge, which is that same glass, so
// the strip must reach past both of the buttons' edges and its middle line
// must BE their centre line — that is what puts a control standing in the
// strip level with them.
func TestStripHoldsTheWindowButtons(t *testing.T) {
	buttonsTop, buttonsBottom := pane.ButtonInsetDp, pane.ButtonInsetDp+desktop.WindowButtonDiameter
	stripTop, stripBottom := 0, pane.StripDp
	if stripTop > buttonsTop {
		t.Errorf("the strip begins at y=%d, below the buttons' top edge at y=%d — the column's content would start under them", stripTop, buttonsTop)
	}
	if stripBottom < buttonsBottom {
		t.Errorf("the strip ends at y=%d, above the buttons' bottom edge at y=%d — the column's content would run under them", stripBottom, buttonsBottom)
	}
	if mid := stripTop + pane.StripDp/2; unit.Dp(mid) != pane.Buttons.Center {
		t.Errorf("the strip's middle line is y=%d and the buttons' is y=%v; a control centred in the strip would sit off their line", mid, pane.Buttons.Center)
	}
}

// TestStripSkipsTheButtonsAndEndsOnTheMargin reads the band's own
// arrangement: the leading skip is the buttons' window-coordinate trailing
// edge, unchanged, because the column starts at the window's own leading
// edge; the controls stand at the trailing corner, and one margin of drag
// follows them to the column's edge.
func TestStripSkipsTheButtonsAndEndsOnTheMargin(t *testing.T) {
	const (
		buttonsEnd = 90 // where the window's buttons end, in window coordinates
		markW      = 24
	)
	mark := color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}
	control := func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(markW, gtx.Constraints.Min.Y)
		paint.FillShape(gtx.Ops, mark, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	}
	// A strip with room to spare, and one cut to the exact width the skip,
	// the control and the trailing margin need. The first says where the
	// control lands; the second says the leading skip is really there,
	// since with no slack the flexed middle has nothing to give.
	lead := buttonsEnd
	for _, tc := range []struct {
		what           string
		width          int
		wantLo, wantHi int
	}{
		{"with room to spare", paneW, paneW - pane.MarginDp - markW, paneW - pane.MarginDp},
		{"cut to the exact fit", lead + markW + pane.MarginDp, lead, lead + markW},
	} {
		t.Run(tc.what, func(t *testing.T) {
			size := image.Pt(tc.width, pane.StripDp)
			w := func(gtx layout.Context) layout.Dimensions {
				return pane.Strip(gtx, buttonsEnd, control)
			}
			img := golden.Capture(t, size, scene(w, color.NRGBA{A: 0xff}))
			lo, hi := -1, -1
			y := pane.StripDp / 2
			for x := 0; x < size.X; x++ {
				if sameColor(img.RGBAAt(x, y), mark) {
					if lo < 0 {
						lo = x
					}
					hi = x + 1
				}
			}
			if lo != tc.wantLo || hi != tc.wantHi {
				t.Errorf("the strip's control spans [%d,%d) %s, want [%d,%d)", lo, hi, tc.what, tc.wantLo, tc.wantHi)
			}
		})
	}
}

// TestSeamColorIsTheSeparator pins the column's one line to the platform's
// name for it: separatorColor, black or white at a tenth, flattened onto
// the fill the line is painted inside — not a ratio solved against it.
func TestSeamColorIsTheSeparator(t *testing.T) {
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			want := vgcolor.Flatten(tc.colors.Separator, tc.colors.SidebarMaterial)
			if got := pane.SeamColor(tc.colors); got != want {
				t.Errorf("the column's seam is %v, want the platform's separator over its own fill %v", got, want)
			}
		})
	}
}

// TestSurfaceIsTheChromeMaterial: a pane is chrome, so it wears the
// platform's chrome material in both appearances.
func TestSurfaceIsTheChromeMaterial(t *testing.T) {
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := pane.Surface(tc.colors), tc.colors.SidebarMaterial; got != want {
				t.Errorf("the pane fills %v, want the chrome material %v", got, want)
			}
		})
	}
}

// TestTheColumnDrawsOneSeamAndNothingElse reads a drawn column: one pixel
// of the seam's own colour down the inside of its trailing edge, the chrome
// chrome material everywhere else inside it, and — the whole of the change
// CG1.1 made — no line and no plane on the other three sides, where the
// column runs to the window's own edges.
func TestTheColumnDrawsOneSeamAndNothingElse(t *testing.T) {
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			bounds := image.Rect(0, 0, paneW, windowH)
			w := func(gtx layout.Context) layout.Dimensions {
				pane.Layout(gtx, tc.colors, bounds, nil)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			backdrop := vgcolor.Flatten(tc.colors.UnderPageBackground, tc.colors.WindowBackground)
			img := golden.Capture(t, windowSize, scene(w, backdrop))
			seam, fill := pane.SeamColor(tc.colors), pane.Surface(tc.colors)

			y := pane.StripDp / 2
			if got := img.RGBAAt(bounds.Max.X-pane.SeamDp, y); !sameColor(got, seam) {
				t.Errorf("the column's trailing edge draws %v, want the seam %v", got, seam)
			}
			if got := img.RGBAAt(bounds.Max.X-pane.SeamDp-1, y); !sameColor(got, fill) {
				t.Errorf("one pixel inside the trailing edge draws %v, want the chrome material %v — the hairline is wider than a hairline", got, fill)
			}

			// The other three sides: the column's fill stands on the window's
			// own edge, so the first column, the first row and the last row
			// inside the bounds are all fill and never a line.
			for _, probe := range []struct {
				what string
				x, y int
			}{
				{"leading", bounds.Min.X, y},
				{"top", paneW / 2, bounds.Min.Y},
				{"bottom", paneW / 2, bounds.Max.Y - 1},
			} {
				if got := img.RGBAAt(probe.x, probe.y); !sameColor(got, fill) {
					t.Errorf("the column's %s edge at (%d,%d) draws %v, want the chrome material %v — nothing is drawn round a flush column",
						probe.what, probe.x, probe.y, got, fill)
				}
			}

			// And nothing of the window's plane is left showing beside it:
			// every row of the leading column carries the column's own fill.
			for y := 0; y < windowH; y++ {
				if got := img.RGBAAt(0, y); !sameColor(got, fill) {
					t.Fatalf("the column's leading edge draws %v at (0,%d); it runs to the window's own edge", got, y)
				}
			}
		})
	}
}

// sameColor compares two colours with one step of slack per channel, which
// is what a rounded fill's straight runs come back as.
func sameColor(got color.RGBA, want color.NRGBA) bool {
	d := func(a, b uint8) int {
		if a > b {
			return int(a) - int(b)
		}
		return int(b) - int(a)
	}
	return d(got.R, want.R) <= 1 && d(got.G, want.G) <= 1 && d(got.B, want.B) <= 1
}

// TestPaneGolden stores the pattern's own picture in both schemes: the
// chrome fill running to the window's leading, top and bottom edges, one
// seam down its trailing edge, and a column standing inside it.
func TestPaneGolden(t *testing.T) {
	contents := func(c color.NRGBA) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			// A block standing in the pane's body, below the strip: proof
			// the contents are handed the pane's own size and clipped to
			// its inside rather than to its boundary.
			defer op.Offset(image.Pt(0, pane.StripDp)).Push(gtx.Ops).Pop()
			size := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y-pane.StripDp)
			paint.FillShape(gtx.Ops, c, clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: size}
		}
	}
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w := func(gtx layout.Context) layout.Dimensions {
				b := pane.Bounds(gtx, gtx.Constraints.Max, paneW, false)
				pane.Layout(gtx, tc.colors, b, contents(tc.colors.SelectedContentBackground))
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			golden.Render(t, tc.name+"-pane", windowSize, scene(w, vgcolor.Flatten(tc.colors.UnderPageBackground, tc.colors.WindowBackground)))
		})
	}
}
