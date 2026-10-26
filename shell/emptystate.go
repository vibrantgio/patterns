package shell

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/attachment"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// EmptyStateDescriptionGap is the room between the line and the description
// under it, and EmptyStateActionsGap the room above the actions row.
//
// NEITHER IS MEASURED: no stored capture holds the platform's content
// unavailable view, so there is no reading of the room between its three
// parts. Both are steps of the spacing scale — the description a step of 8
// and the actions a step of 16 — which keeps the description tied to the
// line it belongs to and the controls parted from both.
var (
	EmptyStateDescriptionGap = unit.Dp(tokens.Spacing.S2)
	EmptyStateActionsGap     = unit.Dp(tokens.Spacing.S4)
)

// EmptyStateProps states one empty state: the line, what may stand under it,
// and the size the line is set at.
type EmptyStateProps struct {
	// Line is the application's own sentence saying why there is nothing to
	// show: nothing selected, nothing in the collection, content still
	// arriving, a read that failed. It names no control — a control is
	// handed in through Actions — and it is one line: a frame too narrow for
	// it truncates it rather than wrapping it. An empty Line paints nothing.
	Line string

	// Description is what may stand under the line to say more. It is
	// optional, it wraps across the frame it is given, and it is set in the
	// body role in SecondaryLabel over the fill: the platform's content
	// unavailable view sets its description in the body text style one step
	// of label contrast below its own line, and no capture of that view is
	// stored, so the body role is the nearest the type set carries.
	Description string

	// Actions are the controls the caller hands in for the region to offer,
	// laid in one row under the description at the platform's control gap,
	// [PlatformToolbar]'s ControlGap, and centred with the rest. They are
	// optional. Each stands on the row's own middle, so controls of
	// different heights share one centre line.
	Actions []layout.Widget

	// LineStyle is the text style the line is set in. The zero value is the
	// column's reading: the EmptyStateLine role, with no line height, which
	// is what a content column stands. A pane's section or a
	// sheet's field hands in the style of the rows it stands for instead, so
	// that a line replacing four rows is set at the size those rows are.
	//
	// It is one field rather than a second constructor because the two
	// sizes are one pattern in two frames, not two patterns: everything else
	// — the colour, the centring, the parts under the line — is the same,
	// and a caller already handing over a props value states the size in it
	// rather than choosing a call. A zero value taking the platform's
	// reading is this package's idiom; [ToolbarMetrics] resolves the same
	// way.
	LineStyle tokens.TextStyle
}

// EmptyState paints what a region with nothing to show stands in place of
// its content: the line saying why there is none, the description that may
// stand under it, and the actions that may stand under that, centred both
// ways in the frame as one group. It is read and never operated — the
// affordance belongs to the controls in Actions — and it is not content: it
// does not stand on the region's measure.
//
// THE CALLER HANDS IT THE COLUMN'S FULL FRAME, the toolbar's share
// included, because that is the frame the group centres in. MEASURED off
// reference/macos/mail-empty-column-light.png and mail-empty-column-dark.png:
// the line's own frame is 35 px tall at window y 450 and the column's spans
// y 30–905, so both centres stand at 467.5, where a line centred on the row
// below the toolbar would stand 26 px lower.
//
// A region whose rows would scroll under a bar that stands passes the frame
// reduced by [EmptyStateFrame], which takes the bar's lane off it.
//
// The line's colour is TertiaryLabel over fill, the fill the region paints.
// MEASURED off the same pair: the glyphs plateau at #bdbdbd light on
// #ffffff and #595f62 dark on #232a2e, which is TertiaryLabel flattened
// over the column and no coverage of SecondaryLabel.
//
// The line is set in the typeface and the weight the body role carries, at
// the size [EmptyStateProps.LineStyle] resolves to.
func EmptyState(gtx layout.Context, sh *text.Shaper, c tokens.PlatformColors, typ tokens.Typography,
	fill color.NRGBA, p EmptyStateProps) layout.Dimensions {
	frame := gtx.Constraints.Max
	if p.Line == "" || frame.X <= 0 || frame.Y <= 0 {
		return layout.Dimensions{Size: frame}
	}
	// The group is laid out across the frame's width at its own height and
	// then moved down as one block, which is what centres the parts
	// together rather than each in a share of its own.
	//
	// The group is placed only after it is measured, and the actions in it
	// are centred across the width, so no part's offset is known while it is
	// laid out: the group is laid out with an [attachment.Surfaces] set, and the
	// tooltip surfaces in it are placed once the group is placed.
	ggtx, surfaces := attachment.WithSurfaces(gtx)
	ggtx.Constraints = layout.Constraints{Min: image.Pt(frame.X, 0), Max: frame}
	rec := op.Record(gtx.Ops)
	height := emptyStateGroup(ggtx, sh, c, typ, fill, p)
	call := rec.Stop()
	above := 0
	if surplus := frame.Y - height; surplus > 0 {
		// Half above rounded down, as typeset splits a floor's surplus, so
		// a line standing alone lands where it stood before the group.
		above = surplus / 2
	}
	off := op.Offset(image.Pt(0, above)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	attachment.PlaceSurfacesAt(gtx, surfaces, image.Pt(0, above))
	off.Pop()
	return layout.Dimensions{Size: frame}
}

// EmptyStateFrame returns the frame an empty state centres in for a region
// whose rows would scroll under bar: frame less the bar's lane on the
// trailing side when the bar stands, and frame unchanged when it fades. The
// frame an empty state centres in is the one the rows would occupy, and the
// rows stop short of a standing bar's lane as the platform's clip view stops
// short of its scroller.
//
// It takes the bar's style rather than the "Show scroll bars" setting
// because the style is the one value that holds both facts: the lane's width
// is [scrollbar.Style.Width], and whether the bar stands is what
// [list.AnchorFor] returns for it, so the rows and a region's empty state
// stop short of the same lane. The setting alone holds no width, and this
// package would then need a second value to be told it.
//
// The lane is taken off the trailing edge only, so the frame's origin does
// not move and the caller sets the result as its constraints' maximum.
func EmptyStateFrame(gtx layout.Context, frame image.Point, bar scrollbar.Style) image.Point {
	if list.AnchorFor(bar) != list.Occupy {
		return frame
	}
	frame.X = max(frame.X-gtx.Dp(bar.Width()), 0)
	return frame
}

// emptyStateGroup draws the parts one under the other across the width gtx
// gives and reports the block's own height. Each part is recorded and
// replayed at the running offset, so the block measures itself as it draws
// and no part is laid out twice.
func emptyStateGroup(gtx layout.Context, sh *text.Shaper, c tokens.PlatformColors, typ tokens.Typography,
	fill color.NRGBA, p EmptyStateProps) int {
	y := 0
	stack := func(gap int, w layout.Widget) {
		if y > 0 {
			y += gap
		}
		rec := op.Record(gtx.Ops)
		dims := w(attachment.AvailableRoomAt(gtx, image.Pt(0, y)))
		call := rec.Stop()
		if y > 0 {
			off := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			call.Add(gtx.Ops)
			off.Pop()
		} else {
			call.Add(gtx.Ops)
		}
		y += dims.Size.Y
	}

	stack(0, func(gtx layout.Context) layout.Dimensions {
		return typeset.TextAligned(gtx, sh, p.Line, emptyStateLineStyle(typ, p.LineStyle),
			vgcolor.Flatten(c.TertiaryLabel, fill), 1, font.Normal, text.Middle)
	})
	if p.Description != "" {
		stack(gtx.Dp(EmptyStateDescriptionGap), func(gtx layout.Context) layout.Dimensions {
			return typeset.TextAligned(gtx, sh, p.Description, typ.Body,
				vgcolor.Flatten(c.SecondaryLabel, fill), 0, font.Normal, text.Middle)
		})
	}
	if len(p.Actions) > 0 {
		stack(gtx.Dp(EmptyStateActionsGap), func(gtx layout.Context) layout.Dimensions {
			return emptyStateActions(gtx, p.Actions, gtx.Dp(PlatformToolbar.ControlGap))
		})
	}
	return y
}

// emptyStateLineStyle answers the style the line is set in: the caller's,
// or the column's reading where the caller states none.
func emptyStateLineStyle(typ tokens.Typography, style tokens.TextStyle) tokens.TextStyle {
	if style.Size > 0 {
		return style
	}
	style = typ.EmptyStateLine
	// The line box is the text's own, so the group's own height is what
	// centres it rather than a role's leading above and below the glyphs.
	style.LineHeight = 0
	return style
}

// emptyStateActions draws the controls in one row, parted by gap and centred
// across the width gtx gives. The row is as tall as its tallest control and
// every control stands on the row's middle.
//
// A control's offset is decided only after every control is measured, so
// each is laid out with an [attachment.Surfaces] set, and the tooltip surfaces
// in it are placed once the control is placed.
func emptyStateActions(gtx layout.Context, actions []layout.Widget, gap int) layout.Dimensions {
	full := gtx.Constraints.Max
	agtx := gtx
	// The controls take their own sizes, so the row's floor is dropped:
	// a Min of the frame's width would stretch the first control across it.
	agtx.Constraints = layout.Constraints{Max: full}
	var calls []op.CallOp
	var sizes []image.Point
	var surfaces []*attachment.Surfaces
	width, height := 0, 0
	for _, a := range actions {
		if a == nil {
			continue
		}
		cgtx, s := attachment.WithSurfaces(agtx)
		rec := op.Record(gtx.Ops)
		dims := a(cgtx)
		calls = append(calls, rec.Stop())
		surfaces = append(surfaces, s)
		sizes = append(sizes, dims.Size)
		if len(calls) > 1 {
			width += gap
		}
		width += dims.Size.X
		height = max(height, dims.Size.Y)
	}
	if len(calls) == 0 {
		return layout.Dimensions{}
	}
	x := 0
	if surplus := full.X - width; surplus > 0 {
		x = surplus / 2
	}
	for i, call := range calls {
		at := image.Pt(x, (height-sizes[i].Y)/2)
		off := op.Offset(at).Push(gtx.Ops)
		call.Add(gtx.Ops)
		attachment.PlaceSurfacesAt(gtx, surfaces[i], at)
		off.Pop()
		x += sizes[i].X + gap
	}
	return layout.Dimensions{Size: image.Pt(full.X, height)}
}
