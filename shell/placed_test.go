package shell_test

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/attachment"
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
		AvailableRoom: attachment.AvailableRoom,
		Trigger: func(gtx layout.Context) layout.Dimensions {
			sz := image.Pt(roomTrigger, roomTrigger)
			paint.FillShape(gtx.Ops, c.SystemBlue, clip.Rect{Max: sz}.Op())
			return layout.Dimensions{Size: sz}
		},
	}
	sh := tokens.DefaultTypography.DeterministicShaper()
	shown := tooltip.Render(sh, props, true, c, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.SmallSystemFont)
	hidden := tooltip.Render(sh, props, false, c, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.SmallSystemFont)
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
		tip(attachment.AvailableRoomAt(gtx, off))
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
			c, tokens.Spacing, tokens.DefaultTypography.SystemFont, tokens.Comfortable)},
		{"stacked-page-navbar-trailing-action", page, shell.RenderStackedPage(sh,
			shell.Props{Navbar: navbar.Props{Actions: []layout.Widget{fillNothingSmall, squareTooltip(c)}, Shaper: sh}},
			[]layout.Widget{fillNothingBand}, c, tokens.Spacing, tokens.DefaultTypography.SystemFont, tokens.Comfortable)},
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
// arrangement moves the row's attachment.Surfaces by its slot's offset and the
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
		return attachment.PlacingFlex(gtx, layout.Flex{Axis: layout.Horizontal},
			attachment.PlacedRigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: lead}
			}),
			attachment.PlacedRigid(arrangement))
	})
	direct := capture(func(gtx layout.Context) layout.Dimensions {
		st := op.Offset(lead).Push(gtx.Ops)
		arrangement(attachment.AvailableRoomAt(gtx, lead))
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
