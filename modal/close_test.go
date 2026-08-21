package modal_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/modal"
	themecolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// A panel's close mark is a drawn cross, not a glyph and not a filled
// control, so what it has to clear is WCAG 1.4.11's floor for a non-text
// graphic: 3:1 against the surface immediately behind it. 1.4.3's 4.5:1 is
// the floor for TEXT and is the wrong criterion to reach for here, which is
// why the one that applies is named rather than left as a bare number.
const closeMarkFloor = 3.0

// closeTargetDp is the pointer target the mark is owed on each axis. It is
// tokens.MinHitTarget, the STANDALONE-control floor, and not the smaller one
// an inline mark inside a chip takes.
//
// Which of the two applies is a question about neighbours, not about how
// large the mark is drawn. The smaller floor exists for a mark riding inside
// another control, where every dp of slop is taken off the thing it rides on
// and off whatever sits next to that. This mark rides on nothing: it stands
// alone at the corner of a surface, with the header's own inset on two sides
// of it and a title that is not a control on the third. There is nothing here
// for the slop to be stolen from, so the standalone floor is the one to meet,
// and the affordance meets it by being an ordinary components/button — the
// square it draws is smaller than the target it answers to.
const closeTargetDp = int(tokens.MinHitTarget)

// markOnly is the fixture both measurements below are taken from: a panel
// with no title and no body, so the only thing drawn inside its surface is
// the close mark. That is what lets the mark be found in the pixels by
// looking for what is not the surface fill, with no coordinate written down
// anywhere and nothing to drift when the header's layout changes.
func markOnly() modal.Props {
	return modal.Props{Title: "", Body: nil}
}

// surfaceAndMark locates, in a captured frame, the dialog surface and the
// ink drawn inside it, and reports the peak contrast that ink reaches against
// the surface fill.
//
// Both are found by colour rather than by arithmetic. The surface is the
// bounding box of every pixel holding its own fill token; the mark is
// whatever inside that box, clear of the 1 dp edge stroke, departs from it.
func surfaceAndMark(img *image.RGBA, fill color.NRGBA) (surface, mark image.Rectangle, ink color.NRGBA, contrast float64) {
	at := func(x, y int) color.NRGBA {
		p := img.RGBAAt(x, y)
		return color.NRGBA{R: p.R, G: p.G, B: p.B, A: 255}
	}
	b := img.Bounds()
	surface = image.Rectangle{Min: image.Pt(b.Max.X, b.Max.Y), Max: b.Min}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if at(x, y) != fill {
				continue
			}
			surface.Min.X = min(surface.Min.X, x)
			surface.Min.Y = min(surface.Min.Y, y)
			surface.Max.X = max(surface.Max.X, x+1)
			surface.Max.Y = max(surface.Max.Y, y+1)
		}
	}
	if surface.Empty() {
		return surface, image.Rectangle{}, ink, 0
	}

	// Two pixels in from the surface's own bounds: one for the edge stroke,
	// one for the pixel it antialiases into.
	inner := surface.Inset(2)
	mark = image.Rectangle{Min: image.Pt(inner.Max.X, inner.Max.Y), Max: inner.Min}
	for y := inner.Min.Y; y < inner.Max.Y; y++ {
		for x := inner.Min.X; x < inner.Max.X; x++ {
			px := at(x, y)
			// Six levels of departure: below that is the dither and the
			// rounding a solid fill picks up on its way through the GPU.
			if absDiffU8(px.R, fill.R) < 6 && absDiffU8(px.G, fill.G) < 6 && absDiffU8(px.B, fill.B) < 6 {
				continue
			}
			mark.Min.X = min(mark.Min.X, x)
			mark.Min.Y = min(mark.Min.Y, y)
			mark.Max.X = max(mark.Max.X, x+1)
			mark.Max.Y = max(mark.Max.Y, y+1)
			if cr := themecolor.ContrastRatio(px, fill); cr > contrast {
				contrast, ink = cr, px
			}
		}
	}
	return surface, mark, ink, contrast
}

func absDiffU8(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// capturePanel renders the mark-only panel in one scheme and returns the
// frame together with the surface fill it was drawn on.
func capturePanel(t *testing.T, c tokens.ColorTokens, bg color.NRGBA) (*image.RGBA, color.NRGBA) {
	t.Helper()
	shaper := defaultShaper(t)
	// The default radius rather than the goldens' sharp one: this is a
	// measurement and not a stored image, and the live pipeline the pointer
	// half of these tests drives rounds its corners, so measuring the same
	// shape keeps the two halves talking about one dialog.
	w := modal.Render(shaper, markOnly(), true, c, tokens.Spacing, tokens.Radius,
		tokens.DefaultTypography.TitleMedium, tokens.Comfortable)
	return golden.Capture(t, canvasSize, scene(w, bg)), c.SurfaceAt(tokens.Level2)
}

// TestCloseMarkContrast measures the close mark against the surface behind
// it, in both schemes, from the rendered pixels rather than from the tokens
// the renderer was handed — a mark that clears the floor in theory and is
// drawn a hairline thinner than it measures is a mark nobody can see.
//
// Run it with -v to read the numbers.
func TestCloseMarkContrast(t *testing.T) {
	for _, sc := range []struct {
		name string
		c    tokens.ColorTokens
		bg   color.NRGBA
	}{
		{"light", tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
		{"dark", tokens.DefaultDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
	} {
		t.Run(sc.name, func(t *testing.T) {
			img, fill := capturePanel(t, sc.c, sc.bg)
			surface, mark, ink, contrast := surfaceAndMark(img, fill)
			if surface.Empty() {
				t.Fatal("no surface found: nothing in the frame holds the level-2 fill")
			}
			if mark.Empty() {
				t.Fatal("no close mark found: the panel's surface is bare")
			}
			t.Logf("surface %v, mark %v (%d×%d px), ink %v on fill %v, %.2f:1",
				surface, mark, mark.Dx(), mark.Dy(), ink, fill, contrast)

			if contrast < closeMarkFloor {
				t.Errorf("close mark on the dialog surface = %.2f:1, want at least %.1f:1",
					contrast, closeMarkFloor)
			}
			// The mark is square by construction — two diagonals of one
			// length — and a stretched one would mean the icon box stopped
			// being square somewhere upstream.
			if mark.Dx() != mark.Dy() {
				t.Errorf("close mark measures %d×%d px, want a square mark", mark.Dx(), mark.Dy())
			}
			// It is a mark and not a speck. The platform reference puts a
			// window's own close control at 14 px and the reference reading
			// app's navigation marks at 10 px on a display where one pixel is
			// one dp; a mark under that band is one nobody finds.
			if mark.Dx() < 9 {
				t.Errorf("close mark measures %d px across, want at least 9", mark.Dx())
			}
		})
	}
}

// TestCloseTargetMeetsTheStandaloneFloor measures the mark's POINTER target
// the only way a target can honestly be measured: by pressing at points
// around it and seeing which of them close the panel.
//
// The run is walked outward from the mark's own centre until a press stops
// arriving, on each of the four sides, so what comes out is the live target's
// width and height and not a restatement of the constant that produced them.
// The walk is bounded well inside the surface: past the surface a press lands
// on the scrim, which closes a panel too, and would report a target running
// off to the window's edge.
func TestCloseTargetMeetsTheStandaloneFloor(t *testing.T) {
	// Find the mark's centre from the static render, which lays the header
	// out exactly as the live pipeline does.
	img, fill := capturePanel(t, tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	surface, mark, _, _ := surfaceAndMark(img, fill)
	if mark.Empty() {
		t.Fatal("no close mark found to measure the target of")
	}
	cx := (mark.Min.X + mark.Max.X) / 2
	cy := (mark.Min.Y + mark.Max.Y) / 2

	var closed int
	w := liveModal(t, modal.Props{
		Open:    rx.Of(true),
		OnClose: func(_ layout.Context) { closed++ },
	})
	r := new(gioinput.Router)
	ops := new(op.Ops)
	// Frame 1 registers the areas, frame 2 settles the focus the modal takes
	// on opening. Presses land from frame 3 on.
	driveFrame(w, ops, r, canvasSize)
	driveFrame(w, ops, r, canvasSize)

	press := func(x, y int) bool {
		before := closed
		pt := f32.Pt(float32(x), float32(y))
		r.Queue(
			pointer.Event{Kind: pointer.Press, Position: pt, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
			pointer.Event{Kind: pointer.Release, Position: pt, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		)
		driveFrame(w, ops, r, canvasSize)
		return closed > before
	}

	if !press(cx, cy) {
		t.Fatalf("a press on the mark itself at (%d,%d) did not close the panel", cx, cy)
	}

	// bound keeps the walk clear of the surface's edge by a margin wider than
	// the floor it is measuring, so a target that overshot would still be
	// seen rather than being cut off at the bound.
	const bound = 30
	reach := func(dx, dy int) int {
		for n := 1; n <= bound; n++ {
			x, y := cx+n*dx, cy+n*dy
			if !image.Pt(x, y).In(surface.Inset(2)) {
				t.Fatalf("the walk (%d,%d) left the surface at n=%d before the target ended", dx, dy, n)
			}
			if !press(x, y) {
				return n - 1
			}
		}
		return bound
	}
	left, right := reach(-1, 0), reach(1, 0)
	up, down := reach(0, -1), reach(0, 1)
	width, height := left+right+1, up+down+1
	t.Logf("close target measures %d×%d dp around (%d,%d): %d left, %d right, %d up, %d down",
		width, height, cx, cy, left, right, up, down)

	if width < closeTargetDp {
		t.Errorf("close target is %d dp wide, want at least %d", width, closeTargetDp)
	}
	if height < closeTargetDp {
		t.Errorf("close target is %d dp tall, want at least %d", height, closeTargetDp)
	}
	// The drawn mark is a fraction of the target it answers to, which is the
	// point of the slop and is worth failing on if it ever inverts.
	if width <= mark.Dx() || height <= mark.Dy() {
		t.Errorf("close target %d×%d does not exceed the %d×%d mark drawn in it",
			width, height, mark.Dx(), mark.Dy())
	}
}
