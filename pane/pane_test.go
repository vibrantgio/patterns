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

// TestBoundsStandsInsideTheWindow reads the whole of the panel's geometry:
// one margin in from the window's leading, top and bottom edges, the
// asked-for width, and the content flush against it on the fourth side.
// MEASURED, voicememos-multi-folder-2026-09-18.png: the panel at x 64-283,
// y 46-786 in a window at x 56-1031, y 38-794 — eight pixels of the
// window's own plane on three sides and none on the fourth.
func TestBoundsStandsInsideTheWindow(t *testing.T) {
	var ops op.Ops
	gtx := ctx(&ops, windowSize)
	got := pane.Bounds(gtx, windowSize, paneW, false)
	want := image.Rect(pane.MarginDp, pane.MarginDp, pane.MarginDp+paneW, windowH-pane.MarginDp)
	if got != want {
		t.Errorf("the panel stands at %v, want %v — one margin inside the window's leading, top and bottom edges", got, want)
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
	backdrop := col.WindowBackground
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
				t.Fatalf("a dismissed pane painted %v at (%d,%d); the window shows its own bare plane", got, x, y)
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
	if want := narrow.X/2 - pane.MarginDp; got.Dx() != want {
		t.Errorf("in a %d-wide window the panel took %d, want the half-window clamp less its margin, %d", narrow.X, got.Dx(), want)
	}
	for _, tc := range []struct {
		what string
		size image.Point
	}{
		{"no area", image.Pt(0, 0)},
		{"no width to stand in", image.Pt(1, windowH)},
		{"no height to set anything into", image.Pt(windowW, 2*pane.MarginDp)},
	} {
		sgtx := ctx(&ops, tc.size)
		if got := pane.Bounds(sgtx, tc.size, paneW, false); !got.Empty() {
			t.Errorf("a window with %s laid out a panel at %v", tc.what, got)
		}
	}
}

// TestStripHoldsTheWindowButtons pins the strip's arithmetic against the
// run it is cut for. The buttons are measured from the window's own glass
// and the strip from the panel's top edge, which is one margin inside that
// glass, so the strip must reach past both of the buttons' edges and its
// middle line must BE their centre line — that is what puts a control
// standing in the strip level with them.
func TestStripHoldsTheWindowButtons(t *testing.T) {
	buttonsTop := pane.ButtonInsetDp - pane.MarginDp
	buttonsBottom := buttonsTop + desktop.WindowButtonDiameter
	stripTop, stripBottom := 0, pane.StripDp
	if stripTop > buttonsTop {
		t.Errorf("the strip begins at y=%d, below the buttons' top edge at y=%d — the panel's content would start under them", stripTop, buttonsTop)
	}
	if stripBottom < buttonsBottom {
		t.Errorf("the strip ends at y=%d, above the buttons' bottom edge at y=%d — the panel's content would run under them", stripBottom, buttonsBottom)
	}
	if mid := pane.MarginDp + pane.StripDp/2; unit.Dp(mid) != pane.Buttons.Center {
		t.Errorf("the strip's middle line is y=%d in the window and the buttons' is y=%v; a control centred in the strip would sit off their line", mid, pane.Buttons.Center)
	}
	// The band the window's other columns carry is the deeper number, and
	// the panel's own top edge sits one margin inside it: MEASURED, the
	// band runs 52 in voicememos-multi-folder-2026-09-18.png and the panel
	// starts at row 8 of it.
	if got, want := pane.MarginDp+pane.StripDp+pane.MarginDp, pane.BandDp; got != want {
		t.Errorf("the panel's strip with a margin either side measures %d and the band %d; the panel's top edge does not sit inside the band", got, want)
	}
	if mid := pane.BandDp / 2; unit.Dp(mid) != pane.Buttons.Center {
		t.Errorf("the band's middle line is y=%d and the buttons' is y=%v; a control centred in the band would sit off their line", mid, pane.Buttons.Center)
	}
}

// TestStripStandsItsControlsAtTheTrailingCorner reads the band's own
// arrangement: the leading skip is the lead in window coordinates less the
// margin the panel already stands in, the drag fills the middle, and the
// controls end one margin clear of the panel's trailing edge — which is
// where the platform's own sidebar panel keeps its marks.
func TestStripStandsItsControlsAtTheTrailingCorner(t *testing.T) {
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
	// the control and the trailing margin need. With room the control is
	// pushed to the trailing corner; cut to the fit it lands in the same
	// place from the other direction.
	skip := buttonsEnd - pane.MarginDp
	for _, tc := range []struct {
		what           string
		width          int
		wantLo, wantHi int
	}{
		{"with room to spare", paneW, paneW - pane.MarginDp - markW, paneW - pane.MarginDp},
		{"cut to the exact fit", skip + markW + pane.MarginDp, skip, skip + markW},
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

// TestRimAndShadowAreThePlatformsNames pins the panel's two boundaries to
// the platform's measured materials rather than to a coverage solved
// against the fill: the rim is opaque in both appearances, so nothing is
// flattened, and the shadow is one peak the drawing spreads.
func TestRimAndShadowAreThePlatformsNames(t *testing.T) {
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := pane.RimColor(tc.colors), tc.colors.PaneRim; got != want {
				t.Errorf("the panel's rim is %v, want the platform's measured rim %v", got, want)
			}
			if got := pane.RimColor(tc.colors); got.A != 0xff {
				t.Errorf("the panel's rim has coverage %d, want an opaque answer — the measured pixel is the colour", got.A)
			}
			if got, want := pane.ShadowColor(tc.colors), tc.colors.PaneShadow; got != want {
				t.Errorf("the panel's shadow is %v, want the platform's measured peak %v", got, want)
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

// TestThePanelDrawsARimOnEverySideAndNoSeam reads a drawn panel: one pixel
// of the platform's rim down every one of its four edges, the chrome
// material one pixel inside each of them, the window's plane darkened by
// the panel's shadow outside them, and nowhere a separator line — an inset
// object needs no seam, and the band is crossed by nothing.
func TestThePanelDrawsARimOnEverySideAndNoSeam(t *testing.T) {
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			var bounds image.Rectangle
			w := func(gtx layout.Context) layout.Dimensions {
				bounds = pane.Bounds(gtx, gtx.Constraints.Max, paneW, false)
				pane.Layout(gtx, tc.colors, bounds, nil)
				pane.PaintShadow(gtx, tc.colors, bounds)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			plane := tc.colors.WindowBackground
			img := golden.Capture(t, windowSize, scene(w, plane))
			rim, fill := pane.RimColor(tc.colors), pane.Surface(tc.colors)
			seam := vgcolor.Flatten(tc.colors.Separator, fill)

			// Mid-run on each edge, clear of the corners' arcs: the rim, and
			// the fill one pixel inside it. The band's own rows are read
			// among them, because the panel is one object from its top edge
			// to its foot and the band crosses nothing.
			midY := (bounds.Min.Y + bounds.Max.Y) / 2
			midX := (bounds.Min.X + bounds.Max.X) / 2
			for _, probe := range []struct {
				what     string
				x, y     int
				inX, inY int
			}{
				{"leading", bounds.Min.X, midY, 1, 0},
				{"trailing", bounds.Max.X - 1, midY, -1, 0},
				{"trailing, in the band", bounds.Max.X - 1, bounds.Min.Y + pane.StripDp/2, -1, 0},
				{"top", midX, bounds.Min.Y, 0, 1},
				{"bottom", midX, bounds.Max.Y - 1, 0, -1},
			} {
				if got := img.RGBAAt(probe.x, probe.y); !sameColor(got, rim) {
					t.Errorf("the panel's %s edge at (%d,%d) draws %v, want the rim %v", probe.what, probe.x, probe.y, got, rim)
				}
				if got := img.RGBAAt(probe.x+probe.inX, probe.y+probe.inY); !sameColor(got, fill) {
					t.Errorf("one pixel inside the panel's %s edge draws %v, want the chrome material %v — the rim is wider than a hairline", probe.what, got, fill)
				}
				if got := img.RGBAAt(probe.x, probe.y); sameColor(got, seam) && rim != seam {
					t.Errorf("the panel's %s edge draws the separator %v; an inset panel is bounded by its rim, not by a seam", probe.what, seam)
				}
			}

			// Outside the panel stands the window's own plane, and the
			// shadow the panel casts on it: the column beside its leading
			// rim is not the bare plane. It is not told apart from the fill
			// here, because in the dark appearance the platform's chrome
			// material and its shadowed plane are two of 255 apart — which
			// is why the boundary is the rim and not a step of fill.
			outside := img.RGBAAt(bounds.Min.X-1, midY)
			if sameColor(outside, plane) {
				t.Errorf("the column beside the panel's leading rim draws the bare plane %v; the panel casts a shadow on it", plane)
			}
			// Far from the panel the plane recovers: past the reach there is
			// no shadow left to see.
			far := img.RGBAAt(bounds.Max.X+2*int(pane.ShadowReachDp), midY)
			if !sameColor(far, plane) {
				t.Errorf("twice the shadow's reach past the panel the window draws %v, want its bare plane %v", far, plane)
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
// inset rounded panel with its rim and the shadow it casts, the window's
// own plane showing around it, and a column standing inside it.
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
				pane.PaintShadow(gtx, tc.colors, b)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			golden.Render(t, tc.name+"-pane", windowSize, scene(w, tc.colors.WindowBackground))
		})
	}
}
