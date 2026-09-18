// Package sidebar provides the Patterns Sidebar pattern: a collapsible
// vertical chrome column that swaps between an expanded width
// (label+icon) and a collapsed width (icon-only) on demand. The active
// Item is drawn as the platform draws a selected sidebar row: the pill
// [PaintSelection] fills, its label in the foreground the platform pairs
// with that fill.
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
// A row is a symbol, a label and, at the trailing end, a count when the
// entry has one: [Item.Icon], [Item.Label] and [Item.Count], each drawn in
// the column the platform draws it in ([SymbolInset], [LabelInset],
// [CountInset]). A row with no symbol or no count draws without one and the
// columns do not move, so the labels of a list whose entries differ still
// line up. A run of rows may be headed by a small label — [Item.Section] on
// the row that begins it — which stands in a block of [SectionHeight] above
// that row and is parted from the rows by air alone: the platform draws no
// line there, and neither does this.
//
// Items are stacked at the sidebar's own row pitch — [RowHeight], 32 dp,
// which is not the platform's list row — in a
// components/list scroll region filling the column below the toggle: a
// list longer than the column is tall scrolls by wheel or touch
// instead of painting past the bottom edge. No scrollbar is drawn — the
// bare list.Layout, the same idiom patterns/table's body uses. Items are
// stacked full-width rows, so each row's pointer area is the row bounds:
// anything added to one would be taken off the row beside it.
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
// the list inoperable. One tag for the list survives virtualisation;
// per-row tags cannot. Rows are consequently pointer targets only, and
// Tab passes the rail in a single step, which is also what a list of
// navigation choices should do.
//
// The collapse affordance takes no focus tag either — it answers
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
	"image/color"

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
	vgcolor "github.com/vibrantgio/theme/color"
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
// selection, which is what draws the platform's selected-row fill,
// so the highlight then follows the keyboard and the pointer from there;
// re-emitting Items with a different Active moves it back. At most one
// item should carry it — the first one that does wins. It is independent
// of OnClick.
type Item struct {
	Icon  layout.Widget
	Label string

	// Count is what stands at the row's trailing end — how many things the
	// entry holds. An empty Count is an entry with no count, and the row
	// draws without one; nothing else about the row moves.
	Count string

	// Section heads the run of rows this item begins with a small label.
	// It is set on the first item of the run and left empty on the rest;
	// an item carrying one is laid out [SectionHeight] taller, with the
	// heading in that block above the row. The heading is not an item: it
	// takes no selection, answers no click, and the keyboard steps over it.
	Section string

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
	// A shaper is not safe to use from two goroutines; Gio lays every
	// layout.Widget out on the one goroutine that runs the event loop,
	// which is what makes sharing it correct. See theme/tokens.Typography.Shaper.
	Shaper *text.Shaper

	// Unemphasized draws the selected row the way the platform draws one in
	// a window that is not frontmost: the unemphasized grey under the
	// ordinary label, rather than the accent-following selection colour
	// under the foreground the platform pairs with it. The zero value is
	// the frontmost window.
	Unemphasized bool
}

// Width constants.
// SpacingScale tops out at S24 = 96 dp, so the 192 dp expanded width
// (≈ 4 × S12) is a local constant rather than a new spacing-token field.
// Widths do not follow density (the column contract is fixed — see the
// package comment); the toggle's height does — it is exactly
// Density.ControlHeight.
const (
	expandedDp  = 192
	collapsedDp = 48
)

// RowHeight, SelectionInset and SelectionRadius are the sidebar's own
// geometry, and they are this package's rather than the density scale's:
// a chrome rail draws a taller row than a content list, so Density.RowHeight
// (20 dp, the platform's list row) does not answer for it and neither
// corrects the other.
//
// MEASURED off the organization's macOS reference, every reading a 1x window
// capture where one pixel is one point:
//
//   - RowHeight 32: Finder's selected sidebar row spans y 78–109 in
//     finder-window-untinted-light.png and y 90–121 in
//     finder-window-untinted-dark.png, and Voice Memos' spans y 363–394 in
//     voicememos-sidebar-light.png — 32 rows in all three.
//   - SelectionInset 10: that Finder pill spans x 52–341 inside a sidebar
//     whose fill spans x 42–351, so it is inset 10 from each edge of the
//     rail; the Voice Memos pill reads the same 10 against its own rail.
//   - SelectionRadius 8: a circular fit to the sub-pixel coverage of the
//     pill's top-left corner reads 7.9 in the Finder capture and 8.4 in the
//     Voice Memos one. The platform's own corner is a continuous curve, which
//     is why the two fits differ; 8 is what a circular corner draws.
const (
	RowHeight       unit.Dp = 32
	SelectionInset  unit.Dp = 10
	SelectionRadius unit.Dp = 8
)

// SymbolBox, SymbolInset, LabelInset and CountInset are where the three parts
// of a row stand, each an inset from the rail's own edge, and SectionHeight,
// SectionInset and SectionBaseline are the block a section's heading occupies.
// They are this package's for the reason RowHeight is: a chrome rail's row is
// not a content list's.
//
// MEASURED off voicememos-multi-folder-2026-09-18.png, the panel at x 64–283,
// and cross-checked against finder-window-untinted-dark.png and
// voicememos-sidebar-dark.png. reference/macos/controls.md carries the
// readings under "What a sidebar row measures".
//
//   - SymbolBox 24 and SymbolInset 17: the folder mark's drawn box runs
//     x 83.0–103.0, centred on x=93.0, which is 29 in from the panel's x=64;
//     Finder's narrower page mark stands on the same centre. A 24 dp square
//     set 17 in centres on 29.
//   - LabelInset 48: every row's name starts at x=112 or 113 against the
//     panel's x=64. Finder's rows start 47 in.
//   - CountInset 17: every count is drawn to x 266 or 267 against the panel's
//     trailing rim at x=283, and the selected row's count keeps that column.
//   - SectionHeight 42: the row above the heading ends at y=161 and the row
//     below it begins at y=203.
//   - SectionInset 17: "My Folders" starts at x=81, the symbol box's own
//     column, and Finder's three headings start there too.
//   - SectionBaseline 30: the heading's cap band is [183.0, 191.0], so its
//     baseline stands 30 into the block and its cap top the 22 already
//     recorded.
const (
	SymbolBox   unit.Dp = 24
	SymbolInset unit.Dp = 17
	LabelInset  unit.Dp = 48
	CountInset  unit.Dp = 17

	SectionHeight   unit.Dp = 42
	SectionInset    unit.Dp = 17
	SectionBaseline unit.Dp = 30
)

type resolvedTokens struct {
	color   tokens.PlatformColors
	spacing tokens.SpacingScale
	label   tokens.TextStyle // the LabelLarge role: typeface, weight, size, line height
	section tokens.TextStyle // the role a section's heading is set in; see SectionStyle
	density tokens.Density   // item/toggle height source
	shaper  *text.Shaper     // the theme's shaper; nil in the Render path
}

// Sidebar returns an rx.Observable[layout.Widget] that emits a new one
// whenever a consumed theme token or the Collapsed observable
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
			rx.CombineLatest4(t.Platform, t.Spacing, t.Typography, t.Density),
			func(n rx.Tuple4[tokens.PlatformColors, tokens.SpacingScale, tokens.Typography, tokens.Density]) resolvedTokens {
				typ := n.Third
				return resolvedTokens{
					color:   n.First,
					spacing: n.Second,
					label:   typ.LabelLarge,
					section: SectionStyle(typ),
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
				return drawSidebar(gtx, shaper, props, st, st.list, col, tok.color, tok.spacing, tok.label, tok.section, tok.density)
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
// size and line height all reach the shaper — section is the role a
// section's heading is set in ([SectionStyle] names it), and d is the
// density the column draws at (item rows and the collapse toggle are each
// exactly Density.ControlHeight). Pass tokens.DefaultTypography.LabelLarge,
// SectionStyle(tokens.DefaultTypography) and tokens.Comfortable for the
// default desktop look.
func Render(
	shaper *text.Shaper,
	props Props,
	collapsed bool,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	label tokens.TextStyle,
	section tokens.TextStyle,
	d tokens.Density,
) layout.Widget {
	state := list.NewState()
	// The highlight is the list's selection in both paths; with no events
	// to move it, the static path's selection is just Item.Active. Select
	// rather than Reveal: a prop declares which row is current, it does not
	// ask the viewport to move.
	state.Select(activeIndex(props.Items))
	return func(gtx layout.Context) layout.Dimensions {
		return drawSidebar(gtx, shaper, props, nil, state, collapsed, colors, sp, label, section, d)
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
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	style tokens.TextStyle,
	section tokens.TextStyle,
	d tokens.Density,
) layout.Dimensions {
	widthDp := float32(expandedDp)
	if collapsed {
		widthDp = collapsedDp
	}
	w := gtx.Dp(unit.Dp(widthDp))
	h := gtx.Constraints.Max.Y
	size := image.Pt(w, h)

	// A sidebar is chrome, so it wears the platform's chrome material: the
	// sidebar and toolbar fill, which in the light appearance is the
	// content's white exactly and in the dark one a blue-grey lighter than
	// the content.
	paint.FillShape(gtx.Ops, colors.SidebarMaterial, clip.Rect{Max: size}.Op())

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
	// full-width row at the sidebar's own pitch, [RowHeight].
	listH := h - toggleH
	if listH <= 0 {
		return layout.Dimensions{Size: size}
	}
	itemH := gtx.Dp(RowHeight)
	stk := op.Offset(image.Pt(0, toggleH)).Push(gtx.Ops)
	lGtx := gtx
	lGtx.Constraints = layout.Exact(image.Pt(w, listH))
	idx := make([]int, len(props.Items))
	for i := range idx {
		idx[i] = i
	}
	list.LayoutSelectable(lGtx, state, idx, func(rGtx layout.Context, i int, selected bool) layout.Dimensions {
		return drawItem(rGtx, shaper, props.Items[i], clickFor(st, i), selected, props.Unemphasized, image.Pt(w, itemH), collapsed, colors, sp, style, section)
	})
	stk.Pop()

	drawTrailingSeam(gtx, size, colors)

	return layout.Dimensions{Size: size}
}

// drawTrailingSeam draws the hairline down the rail's trailing edge: two
// flush regions, so the one leading draws the line that says where it ends —
// once, inside its own bounds. It is what parts the rail from the content
// beside it: in the light appearance the platform's chrome material IS the
// content's white, so without this line the two regions are one blank page.
// It is drawn last so a full-width row, selected or not, cannot erase it.
func drawTrailingSeam(gtx layout.Context, size image.Point, colors tokens.PlatformColors) {
	w := max(gtx.Dp(unit.Dp(1)), 1)
	if size.X <= w || size.Y <= 0 {
		return
	}
	edge := image.Rect(size.X-w, 0, size.X, size.Y)
	paint.FillShape(gtx.Ops, vgcolor.Flatten(colors.Separator, colors.SidebarMaterial), clip.Rect(edge).Op())
}

// SelectionFill is the fill the platform lays under a selected sidebar row:
// the accent in a frontmost window, and the unemphasized selection grey in a
// window that is not.
//
// It is deliberately neither SelectedContentBackground, which is what a
// content list's selected row wears, nor ControlAccent: the platform lifts
// the pill above the accent's own blue over the chrome material. Both
// readings are recorded as SidebarSelection, which follows the theme colour
// through PlatformColors.WithAccent.
func SelectionFill(colors tokens.PlatformColors, unemphasized bool) color.NRGBA {
	if unemphasized {
		return colors.UnemphasizedSelectedContentBackground
	}
	return colors.SidebarSelection
}

// SelectionLabel is the foreground the platform pairs with [SelectionFill]:
// the label it draws on an accent fill where the window is frontmost, and the
// ordinary label on the unemphasized grey where it is not. The caller
// flattens it onto the fill.
func SelectionLabel(colors tokens.PlatformColors, unemphasized bool) color.NRGBA {
	if unemphasized {
		return colors.Label
	}
	return colors.AlternateSelectedControlText
}

// PaintSelection fills the selected row's pill into a row of the given size
// at the current offset: [SelectionFill], inset [SelectionInset] from each
// edge of the rail, cornered at [SelectionRadius], filling the row's height.
//
// It is exported so an application drawing its own chrome rail — a file tree,
// a list of feeds — draws the platform's pill rather than one of its own.
func PaintSelection(gtx layout.Context, size image.Point, colors tokens.PlatformColors, unemphasized bool) {
	inset := gtx.Dp(SelectionInset)
	if 2*inset >= size.X {
		inset = 0
	}
	bounds := image.Rect(inset, 0, size.X-inset, size.Y)
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	r := gtx.Dp(SelectionRadius)
	if half := min(bounds.Dx(), bounds.Dy()) / 2; r > half {
		r = half
	}
	rr := clip.RRect{Rect: bounds, NE: r, NW: r, SE: r, SW: r}
	paint.FillShape(gtx.Ops, SelectionFill(colors, unemphasized), rr.Op(gtx.Ops))
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
// inner content box), in the platform's secondary label over the chrome.
func drawToggle(gtx layout.Context, tt *toggleTag, size image.Point, colors tokens.PlatformColors, d tokens.Density) {
	g := gtx.Dp(icon.Size(d))
	gx := (size.X - g) / 2
	gy := (size.Y - g) / 2
	if mark := icons.Mark(icons.Sidebar); mark != nil {
		st := op.Offset(image.Pt(gx, gy)).Push(gtx.Ops)
		mark(gtx, g, vgcolor.Flatten(colors.SecondaryLabel, colors.SidebarMaterial))
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
	unemphasized bool,
	size image.Point,
	collapsed bool,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	style tokens.TextStyle,
	section tokens.TextStyle,
) layout.Dimensions {
	// A row that begins a section is laid out a block taller and the heading
	// stands in that block. The heading is not part of the row: the pointer
	// area below covers the row alone, so clicking a heading does nothing and
	// the keyboard, which moves over items, never reaches one.
	head := 0
	if item.Section != "" && !collapsed {
		head = gtx.Dp(SectionHeight)
	}
	cell := image.Pt(size.X, size.Y+head)
	gtx.Constraints = layout.Exact(cell)
	if head > 0 {
		hGtx := gtx
		hGtx.Constraints = layout.Exact(image.Pt(size.X, head))
		PaintSection(hGtx, shaper, item.Section, section, image.Pt(size.X, head),
			vgcolor.Flatten(SectionForeground(colors), colors.SidebarMaterial))
	}
	defer op.Offset(image.Pt(0, head)).Push(gtx.Ops).Pop()

	// The selected row is the platform's pill, and a row that is not selected
	// takes no fill of its own and no hover tint either — the platform tints
	// neither a list row nor a sidebar row under the pointer, which the
	// reference captures measure.
	rowFill := colors.SidebarMaterial
	label := colors.Label
	if selected {
		rowFill, label = SelectionFill(colors, unemphasized), SelectionLabel(colors, unemphasized)
	}
	foreground := vgcolor.Flatten(label, rowFill)
	countFG := vgcolor.Flatten(CountForeground(colors, selected, unemphasized), rowFill)

	inner := func(gtx layout.Context) layout.Dimensions {
		if selected {
			PaintSelection(gtx, size, colors, unemphasized)
		}

		drawSymbol(gtx, item.Icon, size, collapsed)

		if collapsed {
			return layout.Dimensions{Size: size}
		}

		// The count first: it owns its column, so what is left of the row is
		// what the label may spend. A label longer than that is ellipsized
		// rather than allowed to run under the count.
		countW := 0
		if item.Count != "" {
			countW = drawTrailing(gtx, shaper, item.Count, style, size, countFG)
		}
		lead := gtx.Dp(LabelInset)
		gap := gtx.Dp(unit.Dp(sp.S2))
		room := size.X - lead - gtx.Dp(CountInset) - countW
		if countW > 0 {
			room -= gap
		}
		if room <= 0 {
			return layout.Dimensions{Size: size}
		}
		lGtx := gtx
		lGtx.Constraints.Min = image.Point{}
		lGtx.Constraints.Max = image.Pt(room, size.Y)
		rec := op.Record(gtx.Ops)
		dims := drawText(lGtx, shaper, item.Label, style, foreground)
		call := rec.Stop()
		stk := op.Offset(image.Pt(lead, (size.Y-dims.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		stk.Pop()
		return layout.Dimensions{Size: size}
	}

	rGtx := gtx
	rGtx.Constraints = layout.Exact(size)
	if click == nil || item.OnClick == nil {
		inner(rGtx)
		return layout.Dimensions{Size: cell}
	}
	dims := inner(rGtx)
	// The pointer target is the row bounds exactly. Rows tile edge to edge,
	// so anything added to one would be taken off its neighbours; the row's
	// full width is what makes it easy to land on.
	area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	semantic.LabelOp(item.Label).Add(gtx.Ops)
	semantic.EnabledOp(true).Add(gtx.Ops)
	pointer.CursorPointer.Add(gtx.Ops)
	click.Add(gtx.Ops)
	area.Pop()
	return layout.Dimensions{Size: cell}
}

// SectionStyle reports the type role a section's heading is set in: the
// smallest label role, which is what the platform's own heading measures as.
//
// MEASURED off voicememos-multi-folder-2026-09-18.png: the heading's cap band
// is 8 px against a row label's 10, and at the shipped face's cap ratio those
// are an 11 dp and a 14 dp role. The scale already carries both, so the
// heading takes the role it matches rather than a size of its own.
func SectionStyle(t tokens.Typography) tokens.TextStyle { return t.LabelSmall }

// SectionForeground is what a section's heading is drawn in: the platform's
// secondary label, which the heading in the reference capture flattens to on
// the panel's own fill to the byte in both appearances. The caller flattens it
// onto the fill.
func SectionForeground(colors tokens.PlatformColors) color.NRGBA { return colors.SecondaryLabel }

// CountForeground is what the count at a row's trailing end is drawn in: the
// sidebar's own measured count value off the pill, and the foreground the
// platform pairs with the pill on it — the count wears the selected row's
// white like the label beside it. The caller flattens it onto the fill.
func CountForeground(colors tokens.PlatformColors, selected, unemphasized bool) color.NRGBA {
	if selected {
		return SelectionLabel(colors, unemphasized)
	}
	return colors.SidebarCount
}

// PaintSection draws a section's heading into a block of the given size at the
// current offset: the label at [SectionInset] from the leading edge, its
// baseline [SectionBaseline] down the block, and nothing else — the platform
// parts a section from the rows above it by air, not by a line.
//
// It is exported so an application drawing its own chrome rail heads its
// sections the way the platform heads them.
func PaintSection(gtx layout.Context, shaper *text.Shaper, label string, style tokens.TextStyle, size image.Point, fg color.NRGBA) layout.Dimensions {
	if label == "" || size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: size}
	}
	lead := gtx.Dp(SectionInset)
	room := size.X - 2*lead
	if room <= 0 {
		return layout.Dimensions{Size: size}
	}
	lGtx := gtx
	lGtx.Constraints.Min = image.Point{}
	lGtx.Constraints.Max = image.Pt(room, size.Y)
	rec := op.Record(gtx.Ops)
	dims := drawText(lGtx, shaper, label, style, fg)
	call := rec.Stop()
	// The block's own top to the heading's baseline is what was measured, and
	// a line box is placed by its top, so the baseline the shaper reports is
	// what carries the one to the other.
	top := gtx.Dp(SectionBaseline) - (dims.Size.Y - dims.Baseline)
	stk := op.Offset(image.Pt(lead, top)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stk.Pop()
	return layout.Dimensions{Size: size}
}

// drawSymbol paints a row's symbol in the square the platform draws it in:
// [SymbolBox], set [SymbolInset] in from the rail's leading edge and centred
// on the row. A collapsed rail has no label to line the symbol up with, so
// there the square is centred in the rail instead.
func drawSymbol(gtx layout.Context, icon layout.Widget, size image.Point, collapsed bool) {
	if icon == nil {
		return
	}
	box := gtx.Dp(SymbolBox)
	if box > size.X {
		box = size.X
	}
	x := gtx.Dp(SymbolInset)
	if collapsed {
		x = (size.X - box) / 2
	}
	if x < 0 {
		x = 0
	}
	iGtx := gtx
	iGtx.Constraints = layout.Constraints{Max: image.Pt(box, size.Y)}
	rec := op.Record(gtx.Ops)
	d := icon(iGtx)
	call := rec.Stop()
	stk := op.Offset(image.Pt(x+(box-d.Size.X)/2, (size.Y-d.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stk.Pop()
}

// drawTrailing paints a row's count at the trailing end — its own trailing
// edge [CountInset] in from the rail's — and reports how wide it came out, so
// the caller knows what is left for the label.
func drawTrailing(gtx layout.Context, shaper *text.Shaper, txt string, style tokens.TextStyle, size image.Point, fg color.NRGBA) int {
	cGtx := gtx
	cGtx.Constraints.Min = image.Point{}
	cGtx.Constraints.Max = image.Pt(size.X, size.Y)
	rec := op.Record(gtx.Ops)
	dims := drawText(cGtx, shaper, txt, style, fg)
	call := rec.Stop()
	x := size.X - gtx.Dp(CountInset) - dims.Size.X
	if x < 0 {
		x = 0
	}
	stk := op.Offset(image.Pt(x, (size.Y-dims.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stk.Pop()
	return dims.Size.X
}

// drawText lays one line out in the role's own typeface, weight, size and
// line height. Zero fields — the Render path can synthesize a size-only style
// — fall back to the shaper's defaults.
func drawText(gtx layout.Context, shaper *text.Shaper, txt string, style tokens.TextStyle, fg color.NRGBA) layout.Dimensions {
	m := op.Record(gtx.Ops)
	paint.ColorOp{Color: fg}.Add(gtx.Ops)
	material := m.Stop()
	return typeset.Layout(gtx, shaper, typeset.Label(style, 1), typeset.Font(style, font.Normal), unit.Sp(style.Size), txt, material)
}
