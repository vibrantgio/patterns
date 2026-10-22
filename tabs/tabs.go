// Package tabs provides the Patterns Tabs pattern: a horizontal tab strip
// with an underline in the platform's selection colour on the selected tab,
// and the selected tab's content drawn below it.
//
// The strip has no fill of its own: it is drawn on the fill of the region
// it is laid out in, which its caller passes, and its trim is a hairline,
// the separator flattened onto that same fill along the strip's foot. The
// tab's content is content, drawn on the same fill as the strip.
//
// Tabs is a callable Go function consuming a components theme observable,
// returning a stream of layout.Widget. Source is intentionally short and
// free of opaque configuration — copy it into your own app and modify as
// needed.
package tabs

import (
	"image"
	"image/color"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/pointershape"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Tab is one entry in the tab strip. Content may be nil; when this tab is
// selected, a nil Content leaves the tab's content region showing the fill
// the strip is drawn on.
type Tab struct {
	Title   string
	Content layout.Widget
}

// Props configures a Tabs instance.
type Props struct {
	Tabs []Tab

	// Selected drives which tab is rendered as selected. A nil Selected
	// defaults to a constant 0. Values outside [0, len(Tabs)) render as
	// "no tab selected" (no underline, empty content area).
	Selected rx.Observable[int]

	// Unemphasized draws the selected tab's mark the way the platform draws
	// a selection in a window that is not frontmost: the unemphasized grey
	// rather than the accent-following selection colour. The zero value is
	// the frontmost window.
	Unemphasized bool

	// Fill answers the fill of the region this pattern is laid out in, for
	// the scheme being drawn. The pattern paints it under its whole frame:
	// the strip and the tab's content are both drawn on it. A tab strip has
	// no fill of its own, so this is the fill its titles flatten their
	// foreground onto and the fill the hairline along its foot is the
	// separator flattened onto. A caller that already painted its region's
	// fill passes that same fill, so the frame matches the region around it.
	//
	// It is a function of the colour set rather than a colour because a
	// live strip outlives a change of scheme and the fill it is drawn on is
	// a different colour on the other side of one; components/input's Surface
	// is the same shape for the same reason.
	//
	// Nil takes the platform's content fill, the fill of a document's
	// column.
	Fill func(tokens.PlatformColors) color.NRGBA

	// OnSelect is invoked when the user changes the selection via click,
	// Arrow-Left/Right (wrapping), Home, or End. May be nil.
	OnSelect func(gtx layout.Context, idx int)

	// Shaper is an explicit per-instance override of the text shaper. Leave
	// it nil in normal use: the tabs then shape their titles with the
	// theme's shaper (Typography.Shaper()), which is built once for the
	// process and shared by every component reading that typography — the
	// cache lives behind the Typography value, so it survives the copy this
	// component's map function makes of it. Set it only when
	// this instance must shape with a different shaper than the theme
	// provides.
	//
	// A shaper is not safe to use from two goroutines; Gio lays every
	// layout.Widget out on the one goroutine that runs the event loop,
	// which is what makes sharing it correct. See theme/tokens.Typography.Shaper.
	Shaper *text.Shaper
}

// fill answers the fill the strip and the tab's content are drawn on for the
// scheme being drawn: what the caller passed, and the platform's content fill
// where it passed nothing.
func (p Props) fill(c tokens.PlatformColors) color.NRGBA {
	if p.Fill == nil {
		return c.ControlBackground
	}
	return p.Fill(c)
}

// Strip dimensions. The strip height is exactly Density.ControlHeight and the
// tab cells fill it; a density-fixed value keeps the layout deterministic
// across goldens regardless of title content. Tab cells tile the strip edge to
// edge, so each cell's pointer area is the cell bounds: anything added to one
// would be taken off the tab beside it.
const underlineDp = 2

type resolvedTokens struct {
	color   tokens.PlatformColors
	spacing tokens.SpacingScale
	title   tokens.TextStyle // the LabelLarge role: typeface, weight, size, line height
	density tokens.Density   // strip height source
	shaper  *text.Shaper     // the theme's shaper; nil in the Render path
}

// Tabs returns an rx.Observable[layout.Widget] that emits a new one
// whenever a consumed theme token or the Selected observable changes.
// Per the WAI-ARIA tab pattern, focus follows selection: clicking a tab
// or pressing Arrow-Left/Right/Home/End moves both selection and focus.
func Tabs(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	selected := props.Selected
	if selected == nil {
		selected = rx.Of(0)
	}
	// Flatten the nested theme observables into a concrete snapshot. The
	// typography emission supplies both the LabelLarge text style and the
	// theme's cached shaper; the theme owns the typeface.
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest4(t.Platform, t.Spacing, t.Typography, t.Density),
			func(n rx.Tuple4[tokens.PlatformColors, tokens.SpacingScale, tokens.Typography, tokens.Density]) resolvedTokens {
				typ := n.Third
				return resolvedTokens{
					color:   n.First,
					spacing: n.Second,
					title:   typ.LabelLarge,
					density: n.Fourth,
					shaper:  typ.Shaper(),
				}
			},
		)
	})
	inputs := rx.CombineLatest2(resolved, selected)
	return rx.Defer(func() rx.Observable[layout.Widget] {
		clicks := make([]widget.Clickable, len(props.Tabs))
		return rx.Map(inputs, func(next rx.Tuple2[resolvedTokens, int]) layout.Widget {
			tok, sel := next.First, next.Second
			// Props.Shaper is an explicit override; the theme's shaper is
			// the default.
			shaper := props.Shaper
			if shaper == nil {
				shaper = tok.shaper
			}
			return func(gtx layout.Context) layout.Dimensions {
				processInput(gtx, props, clicks)
				return drawTabs(gtx, shaper, props, clicks, sel, tok.color, tok.spacing, tok.title, tok.density)
			}
		})
	})
}

// Render produces a layout.Widget for a tabs view with a fixed selected
// index and no event processing. Intended for golden-image testing and
// static demonstrations; production code should use Tabs, which reads
// both of the parameters below off the theme.
//
// title is the LabelLarge role's whole text style — typeface, weight,
// size and line height all reach the shaper — and d is the density the
// strip draws at (the strip is exactly Density.ControlHeight tall, with
// Density.PaddingY around each tab title). Pass
// tokens.DefaultTypography.LabelLarge and tokens.Comfortable for the
// default desktop look.
func Render(
	shaper *text.Shaper,
	props Props,
	selected int,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	title tokens.TextStyle,
	d tokens.Density,
) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return drawTabs(gtx, shaper, props, nil, selected, colors, sp, title, d)
	}
}

// processInput drains click and arrow-key events for each tab. Per the
// WAI-ARIA tab pattern, every selection change also moves focus to the
// newly-selected tab so the next keyboard event is routed correctly.
func processInput(gtx layout.Context, props Props, clicks []widget.Clickable) {
	n := len(props.Tabs)
	if n == 0 {
		return
	}
	move := func(target int) {
		// Modular wrap: callers pass i-1 / i+1 / 0 / n-1; this normalises
		// negatives so wrapping at the ends needs no special case.
		target = ((target % n) + n) % n
		if props.OnSelect != nil {
			props.OnSelect(gtx, target)
		}
		gtx.Execute(key.FocusCmd{Tag: &clicks[target]})
	}
	for i := range props.Tabs {
		if clicks[i].Clicked(gtx) {
			move(i)
		}
		for {
			e, ok := gtx.Event(key.Filter{Focus: &clicks[i], Name: key.NameLeftArrow})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				move(i - 1)
			}
		}
		for {
			e, ok := gtx.Event(key.Filter{Focus: &clicks[i], Name: key.NameRightArrow})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				move(i + 1)
			}
		}
		for {
			e, ok := gtx.Event(key.Filter{Focus: &clicks[i], Name: key.NameHome})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				move(0)
			}
		}
		for {
			e, ok := gtx.Event(key.Filter{Focus: &clicks[i], Name: key.NameEnd})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				move(n - 1)
			}
		}
	}
}

func drawTabs(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	clicks []widget.Clickable,
	selected int,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	style tokens.TextStyle,
	d tokens.Density,
) layout.Dimensions {
	size := gtx.Constraints.Max
	fill := props.fill(colors)
	// One fill under the whole frame: the strip and the tab's content are
	// drawn on the same fill, the one the caller passed. The pattern paints
	// it rather than relying on the caller, so a caller that painted
	// nothing still gets the strip and the tab's content on that fill.
	paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())

	stripH := gtx.Dp(unit.Dp(d.ControlHeight))
	if stripH > size.Y {
		stripH = size.Y
	}
	if stripH < size.Y {
		// The strip's trim: a hairline inside its own foot, the separator
		// flattened onto the fill the strip is drawn on, which separates
		// the titles from the tab's content below them.
		trimH := gtx.Dp(unit.Dp(1))
		if trimH < 1 {
			trimH = 1
		}
		paint.FillShape(gtx.Ops, vgcolor.Flatten(colors.Separator, fill),
			clip.Rect(image.Rect(0, stripH-trimH, size.X, stripH)).Op())
	}

	stripGtx := gtx
	stripGtx.Constraints = layout.Exact(image.Pt(size.X, stripH))
	drawStrip(stripGtx, shaper, props, clicks, selected, colors, sp, style, fill)

	if size.Y > stripH && selected >= 0 && selected < len(props.Tabs) && props.Tabs[selected].Content != nil {
		contentSize := image.Pt(size.X, size.Y-stripH)
		st := op.Offset(image.Pt(0, stripH)).Push(gtx.Ops)
		contentGtx := shell.AvailableRoomAt(gtx, image.Pt(0, stripH))
		contentGtx.Constraints = layout.Exact(contentSize)

		props.Tabs[selected].Content(contentGtx)
		st.Pop()
	}

	return layout.Dimensions{Size: size}
}

func drawStrip(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	clicks []widget.Clickable,
	selected int,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	style tokens.TextStyle,
	fill color.NRGBA,
) layout.Dimensions {
	if len(props.Tabs) == 0 {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	mark := activeMark(colors, props.Unemphasized)
	children := make([]layout.FlexChild, 0, len(props.Tabs))
	for i := range props.Tabs {
		i := i
		children = append(children, layout.Rigid(tabCell(
			shaper, props.Tabs[i].Title, clickFor(clicks, i), i == selected,
			colors, sp, style, mark, fill,
		)))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx, children...)
}

func clickFor(clicks []widget.Clickable, i int) *widget.Clickable {
	if i >= len(clicks) {
		return nil
	}
	return &clicks[i]
}

// activeMark is the colour a selected tab's underline is drawn in: the
// platform's selection colour, which follows the accent, or its unemphasized
// grey where the window is not the frontmost one. The platform marks the
// one-of-many the user is on with the same colour it fills a selected row
// with; a tab strip marks a title rather than a row, so the mark is a line.
func activeMark(colors tokens.PlatformColors, unemphasized bool) color.NRGBA {
	if unemphasized {
		return colors.UnemphasizedSelectedContentBackground
	}
	return colors.SelectedContentBackground
}

// tabCell renders a single tab title centred inside (S3, S2) padding,
// with a strip-height cell. When selected, an underline of underlineDp px
// is drawn along the cell's bottom edge in mark ([activeMark]). The cell
// width is at least 2×S3 so the underline is visible even when the title
// rasterises to zero width, which an empty Tab.Title does.
func tabCell(
	shaper *text.Shaper,
	title string,
	click *widget.Clickable,
	selected bool,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	style tokens.TextStyle,
	mark color.NRGBA,
	fill color.NRGBA,
) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		stripH := gtx.Constraints.Max.Y
		padH := gtx.Dp(unit.Dp(sp.S3))
		underlineH := gtx.Dp(unit.Dp(underlineDp))

		inner := func(gtx layout.Context) layout.Dimensions {
			titleGtx := gtx
			titleGtx.Constraints.Min = image.Point{}
			titleGtx.Constraints.Max.X -= 2 * padH
			if titleGtx.Constraints.Max.X < 0 {
				titleGtx.Constraints.Max.X = 0
			}

			fg := vgcolor.Flatten(colors.Label, fill)
			mTitle := op.Record(gtx.Ops)
			titleDims := typeset.Text(titleGtx, shaper, title, style, fg, 1)
			titleCall := mTitle.Stop()

			cellW := titleDims.Size.X + 2*padH
			cellH := stripH

			offY := (cellH - titleDims.Size.Y - underlineH) / 2
			if offY < 0 {
				offY = 0
			}
			st := op.Offset(image.Pt(padH, offY)).Push(gtx.Ops)
			titleCall.Add(gtx.Ops)
			st.Pop()

			if selected {
				underline := image.Rect(0, cellH-underlineH, cellW, cellH)
				paint.FillShape(gtx.Ops, mark, clip.Rect(underline).Op())
			}
			return layout.Dimensions{Size: image.Pt(cellW, cellH)}
		}

		if click == nil {
			return inner(gtx)
		}
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp(title).Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			dims := inner(gtx)
			pointershape.OverSize(gtx.Ops, dims.Size, pointer.CursorPointer)
			return dims
		})
	}
}
