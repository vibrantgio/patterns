package shell_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/tokens"
)

// TestToolbarBareButtonKeepsItsOwnWidth lays a bare push button into a
// toolbar's leading slot. A push button fills the maximum it is given, and
// the row gives each control the rest of the row as its maximum. The button
// is drawn at the width of its own text and insets, and the trailing group
// keeps its place at the trailing gutter.
func TestToolbarBareButtonKeepsItsOwnWidth(t *testing.T) {
	const box, lead = 20, 24
	size := image.Pt(600, 120)
	shaper := tokens.DefaultTypography.DeterministicShaper()
	bare := button.Render(shaper, "Add", tokens.PlatformLight, tokens.Spacing,
		tokens.Radius, tokens.DefaultTypography.SystemFont, tokens.Comfortable,
		button.RenderState{})

	// The button's content width: the width it reports when it is given
	// no room, which is its text and its two insets.
	var content int
	golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Constraints{Max: image.Pt(0, gtx.Constraints.Max.Y)}
		content = bare(gtx).Size.X
		return layout.Dimensions{Size: size}
	})
	if content <= 0 {
		t.Fatalf("the bare button reports %d px at no room", content)
	}

	var drawn int
	f := shell.TwoColumn{
		PaneHidden:  true,
		ContentFill: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		Toolbar: shell.Toolbar{
			Lead: lead,
			Leading: []layout.Widget{func(gtx layout.Context) layout.Dimensions {
				d := bare(gtx)
				drawn = d.Size.X
				return d
			}},
			Trailing: []layout.Widget{probe(trailFill, box, box)},
		},
		WindowButtonsEnd: func() unit.Dp { return 0 },
	}
	img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		dims, _ := f.Layout(gtx, tokens.PlatformLight)
		return dims
	})

	if drawn != content {
		t.Errorf("the bare button is drawn %d px wide in the leading slot, want its content width %d", drawn, content)
	}
	mid := int(shell.ToolbarDepth) / 2
	for x := lead + content; x < lead+content+8; x++ {
		if c := img.RGBAAt(x, mid); c.R != 0xff || c.G != 0xff || c.B != 0xff {
			t.Fatalf("column %d past the button's content width reads %v, want the content fill", x, c)
		}
	}
	runs := runsOf(img, mid, trailFill)
	if len(runs) != 1 {
		t.Fatalf("the trailing group reads back as %d runs, want one", len(runs))
	}
	if got, want := runs[0][1], size.X-int(shell.PlatformToolbar.Trail); got != want {
		t.Errorf("the trailing control ends at x=%d, want x=%d, one Trail in from the edge", got, want)
	}
}
