// Package sidebar provides the Patterns Sidebar pattern: a collapsible
// vertical Surface column that swaps between an expanded width
// (label+icon) and a collapsed width (icon-only) on demand. The active
// Item is rendered on the Primary ramp's selected step, two steps past
// the Surface ground.
//
// Sidebar is a callable Go function consuming a components theme
// observable, returning a stream of layout.Widget. Source is
// intentionally short — copy it into your own app and modify as needed.
//
// The column's width is not negotiable: 192 dp expanded and 48 dp
// collapsed, both fixed constants in this file that ignore the
// horizontal constraint entirely. Height is whatever it is handed.
// Clamping the width to the constraint would introduce a third,
// unpredictable width where the expanded↔collapsed swap between two
// known numbers is the pattern's contract; a caller wanting a different
// rail width copies the file. Vertical overflow is handled by the
// scroll region below; horizontal space is the caller's explicit
// allocation. Collapsed is an rx.Observable[bool] the caller owns — the
// sidebar renders that state and does not hold it — and
// OnToggleCollapse is the request to change it, so wiring the
// affordance to nothing leaves a sidebar that cannot collapse.
//
// Items are stacked at the density's row pitch — exactly
// Density.ControlHeight (36 dp Comfortable, 28 dp Compact) — in a
// components/list scroll region filling the column below the toggle: a
// list longer than the column is tall scrolls by wheel or touch
// instead of painting past the bottom edge. No scrollbar is drawn — the
// bare list.Layout, the same idiom patterns/table's body uses. Items are
// stacked full-width rows, so each row's hit area stays the row bounds
// (extending it to the 44 dp pointer floor would steal the neighbouring
// row's slop).
//
// The whole rail is one keyboard stop, and the stop is the scroll region
// itself (components/list's [list.State.Focus]) rather than any row. Arrow-Up
// and Arrow-Down move a selection, Home and End reach the first and last
// item, the list scrolls whatever is selected into view, and Enter or
// Space activates it by calling that Item's OnClick. Items without an
// OnClick are still selectable — they are rows in the same list — and
// activating one does nothing.
//
// Per-row focus tags cannot work once items sit in a scroll region: a
// virtualised row is laid out only while it is in view, so it has a
// focus tag only while it is in view, and Arrow traversal would reach
// the visible rows and stop dead at the viewport edge with the rest of
// the list unreachable. One tag for the list survives virtualisation;
// per-row tags cannot. Rows are consequently pointer targets only, and
// Tab passes the rail in a single step, which is also what a list of
// navigation choices should do.
//
// The collapse affordance registers no focus tag either — it answers
// pointer clicks only — so the rail's single stop stays the item list.
// Its glyph is the icon set's sidebar mark (components/icons) — the
// control that shows and hides a window's sidebar — drawn at the icon
// rule's size for the density (components/icon.Size).
//
// Item.Active seeds the selection rather than competing with it: the
// highlighted row is always the list's selection, which starts at the
// Active item and then follows the keyboard and the pointer. Re-emitting
// Items with a different Active moves it back.
package sidebar

import (
	"image"

	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/icon"
	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Item is one entry in the sidebar's list. OnClick may be nil, in which
// case activating the item — by click, Enter or Space — does nothing;
// the row is still selectable, because the keyboard moves a selection
// over the list rather than over the subset of rows that happen to be
// interactive.
//
// Active marks the item the rail should start on. It seeds the list's
// selection, which is what draws the Primary selected-state background,
// so the highlight then follows the keyboard and the pointer from there;
// re-emitting Items with a different Active moves it back. At most one
// item should carry it — the first one that does wins. It is independent
// of OnClick.
type Item struct {
	Icon    layout.Widget
	Label   string
	OnClick func(gtx layout.Context)
	Active  bool
}

// activeIndex returns the index of the first Item marked Active, or -1.
func activeIndex(items []Item) int {
	for i := range items {
		if items[i].Active {
			return i
		}
	}
	return -1
}

// Props configures a Sidebar.
type Props struct {
	Items []Item

	// Collapsed drives the expanded↔collapsed width swap. A nil Collapsed
	// is treated as a constant false (always expanded).
	Collapsed rx.Observable[bool]

	// OnToggleCollapse is invoked when the toggle affordance is clicked.
	// May be nil.
	OnToggleCollapse func(gtx layout.Context)

	// Shaper is an explicit per-instance override of the text shaper. Leave
	// it nil in normal use: the sidebar then shapes its item labels with the
	// theme's shaper (Typography.Shaper()), which is built once for the
	// process and shared by every component reading that typography — the
	// cache lives behind the Typography value, so it survives the copy this
	// component's map function makes of it. Set it only when
	// this instance must shape with a different shaper than the theme
	// provides.
	//
	// A shaper is not safe to use from two goroutines; Gio lays the widget
	// forest out on the one goroutine that runs the event loop, which is
	// what makes sharing it correct. See theme/tokens.Typography.Shaper.
	Shaper *text.Shaper
}

// Width constants.
// SpacingScale tops out at S24 = 96 dp, so the 192 dp expanded width
// (≈ 4 × S12) is a local constant rather than a new spacing-token field.
// Widths do not follow density (the column contract is fixed — see the
// package comment); the item and toggle heights do — both are exactly
// Density.ControlHeight.
const (
	expandedDp  = 192
	collapsedDp = 48
	iconColDp   = 48
)

type resolvedTokens struct {
	color   tokens.ColorTokens
	spacing tokens.SpacingScale
	label   tokens.TextStyle // the LabelLarge role: typeface, weight, size, line height
	density tokens.Density   // item/toggle height source
	shaper  *text.Shaper     // the theme's shaper; nil in the Render path
}

// Sidebar returns an rx.Observable[layout.Widget] that emits a new
// widget whenever a consumed theme token or the Collapsed observable
// changes. Click handlers fire for any Item whose OnClick is non-nil,
// by mouse or by Enter/Space on the selected item; Arrow-Up/Down and
// Home/End move the selection across the whole list, including rows the
// scroll region has not laid out. Clicking the toggle affordance
// dispatches OnToggleCollapse.
func Sidebar(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	collapsed := props.Collapsed
	if collapsed == nil {
		collapsed = rx.Of(false)
	}
	// Flatten the nested theme observables into a concrete snapshot. The
	// typography emission supplies both the LabelLarge text style and the
	// theme's cached shaper.
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest4(t.Color, t.Spacing, t.Typography, t.Density),
			func(n rx.Tuple4[tokens.ColorTokens, tokens.SpacingScale, tokens.Typography, tokens.Density]) resolvedTokens {
				typ := n.Third
				return resolvedTokens{
					color:   n.First,
					spacing: n.Second,
					label:   typ.LabelLarge,
					density: n.Fourth,
					shaper:  typ.Shaper(),
				}
			},
		)
	})
	inputs := rx.CombineLatest2(resolved, collapsed)
	return rx.Defer(func() rx.Observable[layout.Widget] {
		st := &liveState{
			clicks: make([]gesture.Click, len(props.Items)),
			list:   list.NewState(),
			// -2, not -1: -1 is the legitimate "no Active item" answer, and
			// the seed below must run once even for that.
			lastActive: -2,
		}
		return rx.Map(inputs, func(next rx.Tuple2[resolvedTokens, bool]) layout.Widget {
			tok, col := next.First, next.Second
			// Props.Shaper is an explicit override; the theme's shaper is
			// the default.
			shaper := props.Shaper
			if shaper == nil {
				shaper = tok.shaper
			}
			return func(gtx layout.Context) layout.Dimensions {
				processInput(gtx, props, st)
				return drawSidebar(gtx, shaper, props, st, st.list, col, tok.color, tok.spacing, tok.label, tok.density)
			}
		})
	})
}

// liveState is the per-instance state the live pipeline holds across
// frames: one pointer gesture per item, the scroll region's state (which
// now also owns the selection and the rail's only focus tag), the
// toggle's pointer tag, and the bookkeeping that lets Item.Active seed
// the selection without overwriting it on every frame.
type liveState struct {
	clicks     []gesture.Click
	list       *list.State
	toggle     toggleTag
	lastActive int
	// pressedKey is the activation key currently held down on the list,
	// so Enter and Space fire on release after a press, as they do
	// everywhere else in the org.
	pressedKey key.Name
}

// Render produces a layout.Widget for a sidebar with pre-resolved
// tokens, an explicit collapsed flag, and no event processing.
// Intended for golden-image testing and static demonstrations;
// production code should use Sidebar, which reads both of the parameters
// below off the theme.
//
// label is the LabelLarge role's whole text style — typeface, weight,
// size and line height all reach the shaper — and d is the density the
// column draws at (item rows and the collapse toggle are each exactly
// Density.ControlHeight). Pass tokens.DefaultTypography.LabelLarge and
// tokens.Comfortable for the default desktop look.
func Render(
	shaper *text.Shaper,
	props Props,
	collapsed bool,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	label tokens.TextStyle,
	d tokens.Density,
) layout.Widget {
	state := list.NewState()
	// The highlight is the list's selection in both paths; with no events
	// to move it, the static path's selection is just Item.Active. Select
	// rather than Reveal: a prop declares which row is current, it does not
	// ask the viewport to move.
	state.Select(activeIndex(props.Items))
	return func(gtx layout.Context) layout.Dimensions {
		return drawSidebar(gtx, shaper, props, nil, state, collapsed, colors, sp, label, d)
	}
}

// toggleTag is a non-zero-size type so its address is a unique event
// tag for the toggle affordance's pointer hit area.
type toggleTag struct{ _ byte }

func processInput(gtx layout.Context, props Props, st *liveState) {
	// Adopt Item.Active whenever the caller changes it; between changes the
	// selection is the list's own, moved by keys and clicks. Without the
	// comparison a re-emission would drag the highlight back to Active on
	// every frame and the keyboard could never move it.
	if a := activeIndex(props.Items); a != st.lastActive {
		st.lastActive = a
		st.list.Select(a)
	}

	// Row clicks. gesture.Click is deliberately not widget.Clickable: a
	// Clickable registers a focus tag, and a per-row focus tag cannot
	// survive virtualisation. The rail's only focus tag is the list's, so
	// a click hands the keyboard there.
	for i := range st.clicks {
		for {
			e, ok := st.clicks[i].Update(gtx.Source)
			if !ok {
				break
			}
			if e.Kind != gesture.KindClick {
				continue
			}
			st.list.Select(i)
			gtx.Execute(key.FocusCmd{Tag: st.list.Focus()})
			if props.Items[i].OnClick != nil {
				props.Items[i].OnClick(gtx)
			}
		}
	}

	// Enter/Space on the list activates the selected item. Traversal itself
	// (Arrow-Up/Down, Home/End) is components/list's, drained inside
	// LayoutSelectable; activation is ours, because the list has no notion
	// of what a row does.
	tag := st.list.Focus()
	for {
		e, ok := gtx.Event(
			key.Filter{Focus: tag, Name: key.NameReturn},
			key.Filter{Focus: tag, Name: key.NameSpace},
		)
		if !ok {
			break
		}
		ke, ok := e.(key.Event)
		if !ok {
			continue
		}
		switch ke.State {
		case key.Press:
			st.pressedKey = ke.Name
		case key.Release:
			if st.pressedKey != ke.Name {
				break
			}
			st.pressedKey = ""
			sel := st.list.Selected()
			if sel >= 0 && sel < len(props.Items) && props.Items[sel].OnClick != nil {
				props.Items[sel].OnClick(gtx)
			}
		}
	}

	// Toggle: pointer-click only (no focus tag → never a Tab target, so the
	// rail stays a single keyboard stop).
	for {
		e, ok := gtx.Event(pointer.Filter{Target: &st.toggle, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pe, ok := e.(pointer.Event); ok && pe.Kind == pointer.Press {
			if props.OnToggleCollapse != nil {
				props.OnToggleCollapse(gtx)
			}
		}
	}
}

func drawSidebar(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	st *liveState,
	state *list.State,
	collapsed bool,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	style tokens.TextStyle,
	d tokens.Density,
) layout.Dimensions {
	widthDp := float32(expandedDp)
	if collapsed {
		widthDp = collapsedDp
	}
	w := gtx.Dp(unit.Dp(widthDp))
	h := gtx.Constraints.Max.Y
	size := image.Pt(w, h)

	// The sidebar is chrome furniture, so it fills at the window's floor —
	// the level beneath the paper, in both schemes. Filling colors.Surface
	// instead would be wrong: that neutral-ramp alias is a pairing rather
	// than a level, and it is the floor in the light scheme but the raised
	// rung in the dark scheme, painting the desk a level above the
	// document lying on it.
	paint.FillShape(gtx.Ops, colors.SurfaceAt(tokens.LevelBackdrop), clip.Rect{Max: size}.Op())

	// Toggle affordance at the top: a row like the items, so it shares
	// the density's control height.
	toggleH := gtx.Dp(unit.Dp(d.ControlHeight))
	var tt *toggleTag
	if st != nil {
		tt = &st.toggle
	}
	drawToggle(gtx, tt, image.Pt(w, toggleH), colors, d)

	// Items below the toggle, in a components/list scroll region filling the
	// rest of the column — no scrollbar, like table's body: wheel/touch
	// scrolling plus the list's own keyboard traversal. Each row is a
	// full-width row at the density's pitch: exactly ControlHeight, which
	// is what list.RowHeight resolves to.
	listH := h - toggleH
	if listH <= 0 {
		return layout.Dimensions{Size: size}
	}
	itemH := gtx.Dp(list.RowHeight(d))
	stk := op.Offset(image.Pt(0, toggleH)).Push(gtx.Ops)
	lGtx := gtx
	lGtx.Constraints = layout.Exact(image.Pt(w, listH))
	idx := make([]int, len(props.Items))
	for i := range idx {
		idx[i] = i
	}
	list.LayoutSelectable(lGtx, state, idx, func(rGtx layout.Context, i int, selected bool) layout.Dimensions {
		return drawItem(rGtx, shaper, props.Items[i], clickFor(st, i), selected, image.Pt(w, itemH), collapsed, colors, sp, style)
	})
	stk.Pop()

	return layout.Dimensions{Size: size}
}

func clickFor(st *liveState, i int) *gesture.Click {
	if st == nil || i >= len(st.clicks) {
		return nil
	}
	return &st.clicks[i]
}

// drawToggle paints the collapse affordance's glyph centred in a
// (w × h) area at the current offset and registers a pointer.Press hit
// area against tt. In test or static rendering (tt == nil) only the
// glyph is drawn.
//
// The glyph is the icon set's sidebar mark — the control that shows and
// hides a window's sidebar, resolved to the host platform's drawing —
// at the icon rule's size for the density (icon.Size: the control's
// inner content box), in the icon set's secondary neutral.
func drawToggle(gtx layout.Context, tt *toggleTag, size image.Point, colors tokens.ColorTokens, d tokens.Density) {
	g := gtx.Dp(icon.Size(d))
	gx := (size.X - g) / 2
	gy := (size.Y - g) / 2
	if mark := icons.Mark(icons.Sidebar); mark != nil {
		st := op.Offset(image.Pt(gx, gy)).Push(gtx.Ops)
		mark(gtx, g, colors.Ramps.Neutral.Step(700))
		st.Pop()
	}

	if tt == nil {
		return
	}
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, tt)
	pointer.CursorPointer.Add(gtx.Ops)
	area.Pop()
}

func drawItem(
	gtx layout.Context,
	shaper *text.Shaper,
	item Item,
	click *gesture.Click,
	selected bool,
	size image.Point,
	collapsed bool,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	style tokens.TextStyle,
) layout.Dimensions {
	inner := func(gtx layout.Context) layout.Dimensions {
		if selected {
			// Selected background is a step past the sidebar's Surface
			// ground on the Primary ramp, keeping the highlight's primary
			// hue as a real, addressable colour.
			active := colors.StateColor(tokens.RolePrimary, 200, tokens.StateSelected)
			paint.FillShape(gtx.Ops, active, clip.Rect{Max: size}.Op())
		}

		iconW := gtx.Dp(unit.Dp(iconColDp))
		if iconW > size.X {
			iconW = size.X
		}

		// Icon slot: centred inside the leading iconCol.
		if item.Icon != nil {
			iconGtx := gtx
			iconGtx.Constraints = layout.Constraints{
				Min: image.Point{},
				Max: image.Pt(iconW, size.Y),
			}
			st := op.Offset(image.Point{}).Push(gtx.Ops)
			rec := op.Record(gtx.Ops)
			d := item.Icon(iconGtx)
			call := rec.Stop()
			offX := (iconW - d.Size.X) / 2
			offY := (size.Y - d.Size.Y) / 2
			if offX < 0 {
				offX = 0
			}
			if offY < 0 {
				offY = 0
			}
			st.Pop()
			stk := op.Offset(image.Pt(offX, offY)).Push(gtx.Ops)
			call.Add(gtx.Ops)
			stk.Pop()
		}

		// Label slot: trailing, hidden when collapsed.
		if !collapsed && size.X > iconW {
			padH := gtx.Dp(unit.Dp(sp.S2))
			labelMaxW := size.X - iconW - padH
			if labelMaxW > 0 {
				mColor := op.Record(gtx.Ops)
				paint.ColorOp{Color: colors.Text}.Add(gtx.Ops)
				textMaterial := mColor.Stop()

				labelGtx := gtx
				labelGtx.Constraints.Min = image.Point{}
				labelGtx.Constraints.Max.X = labelMaxW
				labelGtx.Constraints.Max.Y = size.Y

				// Shape with the LabelLarge role's typeface, weight, size
				// and line height. Zero fields (the Render path can
				// synthesize a size-only style) fall back to the shaper's
				// defaults.
				f := typeset.Font(style, font.Normal)
				wl := typeset.Label(style, 1)
				mLabel := op.Record(gtx.Ops)
				labelDims := typeset.Layout(labelGtx, shaper, wl, f, unit.Sp(style.Size), item.Label, textMaterial)
				labelCall := mLabel.Stop()

				offY := (size.Y - labelDims.Size.Y) / 2
				stk := op.Offset(image.Pt(iconW, offY)).Push(gtx.Ops)
				labelCall.Add(gtx.Ops)
				stk.Pop()
			}
		}
		return layout.Dimensions{Size: size}
	}

	gtx.Constraints = layout.Exact(size)
	if click == nil || item.OnClick == nil {
		return inner(gtx)
	}
	dims := inner(gtx)
	// The pointer target is the row bounds exactly. Rows tile edge to edge,
	// so extending one to tokens.MinHitTarget would only take the slop off
	// its neighbours; the row's full width is what makes it easy to hit.
	area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	semantic.LabelOp(item.Label).Add(gtx.Ops)
	semantic.EnabledOp(true).Add(gtx.Ops)
	pointer.CursorPointer.Add(gtx.Ops)
	click.Add(gtx.Ops)
	area.Pop()
	return dims
}
