package shell_test

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/tokens"
)

// capsuleDp is the width of a toolbar control titled with one symbol.
// MEASURED at 1x: mail-window.png's compose control spans x 404-441.
const capsuleDp = 38

// TestToolbarLaysEachControlOutOnceAtItsOwnWidth lays a control titled with
// a symbol and a control titled with text into a toolbar's leading group.
// The titled control is a push button, which fills the maximum it is given.
// Each control is laid out once in the frame: the symbol control at the
// capsule's width, the titled one at its text's width and an inset each
// side. The trailing group keeps its place at the trailing gutter.
func TestToolbarLaysEachControlOutOnceAtItsOwnWidth(t *testing.T) {
	const box, lead, title = 20, 24, "Share"
	size := image.Pt(600, 120)
	c := tokens.PlatformLight
	shaper := tokens.DefaultTypography.DeterministicShaper()
	style := tokens.DefaultTypography.SystemFont
	titled := button.Render(shaper, title, c, tokens.Spacing, tokens.Radius, style,
		tokens.Comfortable, button.RenderState{})
	var click widget.Clickable

	var symbolCalls, titledCalls, symbolW, titledW, wantTitled int
	f := shell.TwoColumn{
		PaneHidden:  true,
		ContentFill: c.TextBackground,
		Toolbar: shell.Toolbar{
			Lead: lead,
			Leading: []layout.Widget{
				func(gtx layout.Context) layout.Dimensions {
					symbolCalls++
					d := shell.ToolbarControl(gtx, c, tokens.Radius, tokens.Comfortable, &click, icons.Mark(icons.Plus), "Add", false)
					symbolW = d.Size.X
					return d
				},
				shell.ToolbarTitled(shaper, style, title, func(gtx layout.Context) layout.Dimensions {
					titledCalls++
					d := titled(gtx)
					titledW = d.Size.X
					return d
				}),
			},
			Trailing: []layout.Widget{probe(trailFill, box, box)},
		},
		WindowButtonsEnd: func() unit.Dp { return 0 },
	}
	img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		wantTitled = shell.ToolbarTitleWidth(gtx, shaper, style, title)
		dims, _ := f.Layout(gtx, c)
		return dims
	})

	if symbolCalls != 1 || titledCalls != 1 {
		t.Errorf("the symbol control is laid out %d times and the titled one %d times in one frame, want once each", symbolCalls, titledCalls)
	}
	if symbolW != capsuleDp {
		t.Errorf("the symbol control is %d px wide, want the capsule's %d", symbolW, capsuleDp)
	}
	if titledW != wantTitled {
		t.Errorf("the titled control is %d px wide, want its text and an inset each side, %d", titledW, wantTitled)
	}
	mid := int(shell.ToolbarDepth) / 2
	runs := runsOf(img, mid, trailFill)
	if len(runs) != 1 {
		t.Fatalf("the trailing group reads back as %d runs, want one", len(runs))
	}
	if got, want := runs[0][1], size.X-int(shell.PlatformToolbar.Trail); got != want {
		t.Errorf("the trailing control ends at x=%d, want x=%d, one Trail in from the edge", got, want)
	}
}
