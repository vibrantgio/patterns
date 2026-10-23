package shell_test

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/tooltip"
	"github.com/vibrantgio/patterns/navbar"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/tokens"
)

// squareTooltip is a tooltip below a small square trigger, laid out at the
// trigger's own size wherever its caller places it; it is shown only while
// the context says so (see showTooltips).
func squareTooltip(c tokens.PlatformColors) layout.Widget {
	props := tooltip.Props{
		Text:          "A tooltip at the window's edge",
		Placement:     tooltip.Bottom,
		AvailableRoom: shell.AvailableRoom,
		Trigger: func(gtx layout.Context) layout.Dimensions {
			sz := image.Pt(roomTrigger, roomTrigger)
			paint.FillShape(gtx.Ops, c.SystemBlue, clip.Rect{Max: sz}.Op())
			return layout.Dimensions{Size: sz}
		},
	}
	sh := tokens.DefaultTypography.DeterministicShaper()
	shown := tooltip.Render(sh, props, true, c, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.LabelSmall)
	hidden := tooltip.Render(sh, props, false, c, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.LabelSmall)
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(image.Pt(roomTrigger, roomTrigger))
		if gtx.Values[showTooltips] == true {
			return shown(gtx)
		}
		return hidden(gtx)
	}
}

// footerTooltip is a footer slot that takes its own depth: a square tooltip
// at its leading end, standing on the slot's top edge, in a slot 22 px deep.
func footerTooltip(c tokens.PlatformColors) layout.Widget {
	tip := squareTooltip(c)
	return func(gtx layout.Context) layout.Dimensions {
		tip(gtx)
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 22)}
	}
}

// sectionTooltip is a stacked page's section 40 px deep with a square
// tooltip at its trailing end.
func sectionTooltip(c tokens.PlatformColors) layout.Widget {
	tip := squareTooltip(c)
	return func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		off := image.Pt(w-roomTrigger, 0)
		st := op.Offset(off).Push(gtx.Ops)
		tip(shell.AvailableRoomAt(gtx, off))
		st.Pop()
		return layout.Dimensions{Size: image.Pt(w, 40)}
	}
}

// TestTooltipPlacedAfterMeasuringIsInsideWindow places a shown tooltip in
// each layout that places its child only after measuring it, against the
// window's edge, and reads that it is drawn whole inside the window: the
// layout places the tooltip's surface once it has placed the child, in the
// window's available room moved by the child's offset.
func TestTooltipPlacedAfterMeasuringIsInsideWindow(t *testing.T) {
	c := tokens.PlatformLight
	sh := tokens.DefaultTypography.DeterministicShaper()
	page := image.Pt(480, int(shell.NavbarHeight())+40)
	for _, tc := range []struct {
		name   string
		size   image.Point
		window func(gtx layout.Context) layout.Dimensions
	}{
		{"two-column-toolbar-trailing", twoColSize, func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.TwoColumn{PaneWidth: 200, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing}, Main: fillNothing,
				Toolbar: shell.Toolbar{Trailing: []layout.Widget{squareTooltip(c)},
					Metrics: shell.ToolbarMetrics{Trail: 1}}}.Layout(gtx, c)
			return d
		}},
		{"two-column-toolbar-pair", twoColSize, func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.TwoColumn{PaneWidth: 200, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing}, Main: fillNothing,
				Toolbar: shell.Toolbar{Trailing: []layout.Widget{shell.ToolbarPair(fillNothingSmall, squareTooltip(c))},
					Metrics: shell.ToolbarMetrics{Trail: 1}}}.Layout(gtx, c)
			return d
		}},
		{"three-column-toolbar-trailing", twoColSize, func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.ThreeColumn{PaneWidth: 200, ListWidth: 240, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing}, List: shell.ListColumn{Rows: fillNothing}, Main: fillNothing,
				Toolbar: shell.Toolbar{Trailing: []layout.Widget{squareTooltip(c)},
					Metrics: shell.ToolbarMetrics{Trail: 1}}}.Layout(gtx, c)
			return d
		}},
		{"pane-strip-mark", twoColSize, func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.TwoColumn{PaneWidth: 60, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing, Marks: []layout.Widget{squareTooltip(c)}},
				Main: fillNothing}.Layout(gtx, c)
			return d
		}},
		{"pane-column-footer-own-depth", twoColSize, func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.TwoColumn{PaneWidth: 200, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing, Footer: footerTooltip(c)},
				Main: fillNothing}.Layout(gtx, c)
			return d
		}},
		{"list-column-footer-own-depth", twoColSize, func(gtx layout.Context) layout.Dimensions {
			d, _ := shell.ThreeColumn{PaneWidth: 200, ListWidth: 240, Surface: c.WindowBackground, ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{Rows: fillNothing},
				List: shell.ListColumn{Rows: fillNothing, Footer: footerTooltip(c)},
				Main: fillNothing}.Layout(gtx, c)
			return d
		}},
		{"empty-state-action", image.Pt(320, 72), func(gtx layout.Context) layout.Dimensions {
			return shell.EmptyState(gtx, sh, c, tokens.DefaultTypography, c.TextBackground,
				shell.EmptyStateProps{Line: "No note selected", Actions: []layout.Widget{squareTooltip(c)}})
		}},
		{"stacked-page-section", page, shell.RenderStackedPage(sh, shell.Props{}, []layout.Widget{sectionTooltip(c)},
			c, tokens.Spacing, tokens.DefaultTypography.LabelLarge, tokens.Comfortable)},
		{"stacked-page-navbar-trailing-action", page, shell.RenderStackedPage(sh,
			shell.Props{Navbar: navbar.Props{Actions: []layout.Widget{fillNothingSmall, squareTooltip(c)}, Shaper: sh}},
			[]layout.Widget{fillNothingBand}, c, tokens.Spacing, tokens.DefaultTypography.LabelLarge, tokens.Comfortable)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertTooltipInsideWindow(t, tc.size, tc.window)
		})
	}
}

// fillNothingBand is a stacked page's section 40 px deep that draws nothing.
func fillNothingBand(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 40)}
}

// fillNothingSmall is a control that draws nothing at a small square size.
func fillNothingSmall(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{Size: image.Pt(roomTrigger, roomTrigger)}
}

// TestTooltipInArrangementInsideCollectedChildIsAtTrigger lays an
// arrangement out as a child of a row that places it only after measuring
// it, at an offset, with a shown tooltip in the arrangement's main slot. It
// reads the window against the same arrangement placed at the same offset by
// a plain transform with the area moved: the two are drawn alike, so the
// arrangement moves the row's tooltip.Surfaces by its slot's offset and the
// surface is placed at its trigger.
func TestTooltipInArrangementInsideCollectedChildIsAtTrigger(t *testing.T) {
	c := tokens.PlatformLight
	size := image.Pt(640, 360)
	arr := image.Pt(480, 300)
	lead := image.Pt(80, 0)
	arrangement := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(arr)
		d, _ := shell.TwoColumn{PaneWidth: 160, Surface: c.WindowBackground, ContentFill: c.TextBackground,
			Pane: shell.PaneColumn{Rows: fillNothing}, Main: cornerTooltip(c)}.Layout(gtx, c)
		return d
	}
	capture := func(window layout.Widget) *image.RGBA {
		return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			gtx.Values = map[string]any{showTooltips: true}
			gtx = shell.WithAvailableRoom(gtx, image.Rectangle{Max: size})
			return window(gtx)
		})
	}
	collected := capture(func(gtx layout.Context) layout.Dimensions {
		return shell.PlacingFlex(gtx, layout.Flex{Axis: layout.Horizontal},
			shell.PlacedRigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: lead}
			}),
			shell.PlacedRigid(arrangement))
	})
	direct := capture(func(gtx layout.Context) layout.Dimensions {
		st := op.Offset(lead).Push(gtx.Ops)
		arrangement(shell.AvailableRoomAt(gtx, lead))
		st.Pop()
		return layout.Dimensions{Size: size}
	})
	n := 0
	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			if collected.RGBAAt(x, y) != direct.RGBAAt(x, y) {
				n++
			}
		}
	}
	if n != 0 {
		t.Errorf("the arrangement inside the row drew %d pixels unlike the one placed by a transform, want 0", n)
	}
}

// flexChild is one child of the rows TestFlexOffsetsFollowFlex compares: a
// rectangle of its own size and baseline, at least as long as the flex's
// minimum along the main axis, drawn in its own colour.
type flexChild struct {
	size     image.Point
	baseline int
	colour   color.NRGBA
}

func (fc flexChild) layout(gtx layout.Context) layout.Dimensions {
	sz := fc.size
	sz.X = max(sz.X, gtx.Constraints.Min.X)
	sz.Y = max(sz.Y, gtx.Constraints.Min.Y)
	paint.FillShape(gtx.Ops, fc.colour, clip.Rect{Max: sz}.Op())
	return layout.Dimensions{Size: sz, Baseline: fc.baseline}
}

// TestFlexOffsetsFollowFlex lays rows out with layout.Flex for every
// spacing and alignment, on both axes, with and without a gap and with and
// without a flexed child, each child drawn in its own colour, and reads
// each child's offset off the capture: flexOffsets returns the same offsets
// from the sizes the children reported.
func TestFlexOffsetsFollowFlex(t *testing.T) {
	spacings := []layout.Spacing{layout.SpaceEnd, layout.SpaceStart, layout.SpaceSides,
		layout.SpaceAround, layout.SpaceBetween, layout.SpaceEvenly}
	alignments := []layout.Alignment{layout.Start, layout.End, layout.Middle, layout.Baseline}
	colours := []color.NRGBA{{R: 0xff, A: 0xff}, {G: 0xff, A: 0xff}, {B: 0xff, A: 0xff}}
	for _, axis := range []layout.Axis{layout.Horizontal, layout.Vertical} {
		cell := axis.Convert(image.Pt(150, 60))
		pitch := cell.Add(image.Pt(10, 10))
		type row struct {
			flex    layout.Flex
			flexed  bool
			name    string
			at      image.Point
			dims    []layout.Dimensions
			flexIdx int
		}
		var rows []*row
		for si, sp := range spacings {
			col := 0
			for _, al := range alignments {
				for _, gap := range []int{0, 3} {
					for _, flexed := range []bool{false, true} {
						rows = append(rows, &row{
							flex:   layout.Flex{Axis: axis, Spacing: sp, Alignment: al, Gap: gap},
							flexed: flexed,
							name:   fmt.Sprintf("axis %v spacing %v alignment %v gap %d flexed %v", axis, sp, al, gap, flexed),
							at:     image.Pt(col*pitch.X, si*pitch.Y),
						})
						if axis == layout.Vertical {
							rows[len(rows)-1].at = image.Pt(si*pitch.X, col*pitch.Y)
						}
						col++
					}
				}
			}
		}
		children := []flexChild{
			{size: axis.Convert(image.Pt(20, 10)), baseline: 3, colour: colours[0]},
			{size: axis.Convert(image.Pt(30, 24)), baseline: 5, colour: colours[1]},
			{size: axis.Convert(image.Pt(12, 17)), baseline: 9, colour: colours[2]},
		}
		var extent image.Point
		for _, r := range rows {
			extent.X = max(extent.X, r.at.X+pitch.X)
			extent.Y = max(extent.Y, r.at.Y+pitch.Y)
		}
		img := golden.Capture(t, extent, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, color.NRGBA{A: 0xff}, clip.Rect{Max: extent}.Op())
			for _, r := range rows {
				r.dims = make([]layout.Dimensions, len(children))
				fc := make([]layout.FlexChild, len(children))
				for i, ch := range children {
					w := func(gtx layout.Context) layout.Dimensions {
						r.dims[i] = ch.layout(gtx)
						return r.dims[i]
					}
					if r.flexed && i == 1 {
						fc[i] = layout.Flexed(0.5, w)
					} else {
						fc[i] = layout.Rigid(w)
					}
				}
				st := op.Offset(r.at).Push(gtx.Ops)
				cgtx := gtx
				cgtx.Constraints = layout.Exact(cell)
				r.flex.Layout(cgtx, fc...)
				st.Pop()
			}
			return layout.Dimensions{Size: extent}
		})
		for _, r := range rows {
			want := shell.FlexOffsets(r.flex, layout.Exact(cell), r.dims)
			for i, ch := range children {
				got, ok := topLeftOf(img, image.Rectangle{Min: r.at, Max: r.at.Add(cell)}, ch.colour)
				if !ok {
					t.Errorf("%s: child %d was not drawn", r.name, i)
					continue
				}
				if got.Sub(r.at) != want[i] {
					t.Errorf("%s: child %d at %v in layout.Flex, flexOffsets says %v", r.name, i, got.Sub(r.at), want[i])
				}
			}
		}
	}
}

// topLeftOf returns the top-left corner of the pixels of colour c inside r.
func topLeftOf(img *image.RGBA, r image.Rectangle, c color.NRGBA) (image.Point, bool) {
	want := color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	corner := r.Max
	found := false
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if img.RGBAAt(x, y) == want {
				corner = image.Pt(min(corner.X, x), min(corner.Y, y))
				found = true
			}
		}
	}
	return corner, found
}
