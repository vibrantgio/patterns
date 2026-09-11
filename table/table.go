// Package table provides the Patterns Table pattern: a sortable, filterable,
// virtualised data table. Body rows are laid out only for the current
// viewport (O(visible) per frame) via components/list, independent of how many
// rows the Items observable carries.
//
// Sort and filter are external transforms. The Items observable emits
// already-sorted, already-filtered slices; the table renders whatever it
// receives and surfaces the header click's purpose via OnSort. Filter UI is the
// consumer's responsibility (typically a textfield above the table whose
// changes re-emit a filtered Items slice). This keeps the table dumb: no
// opaque runtime configuration, source is the spec, copy and modify as
// needed.
//
// # Keyboard reach
//
// Body rows are not interactive: they take no focus tag, no click and
// no selection, and the only keyboard reach in the table is Tab onto a
// sortable header cell. Props.Current does not change that: it is a fill
// the consumer asks for over a row it already knows about, drawn only for
// rows the frame laid out, and the table stores no selection of its own
// for a keyboard to reach.
//
// Row selection must be built on components/list's LayoutSelectable, which
// moves an index over every row, not on a focus tag per row, which cannot
// exist for a row the frame skipped. The list.State this package already
// holds is where that selection lives.
//
// Per-row component state (editors, checkboxes, expanders) is preserved across
// sort/filter by wiring components/keyed.Defer into a Column's Cell closure: the
// consumer captures a *keyed.Deferred[K, *WidgetState] in the rx.Defer scope
// holding the Items observable, and returns the same state pointer for the
// same row key on every emission. The table itself stores no per-row state
// — every Column.Cell call is fresh.
package table

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
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
	"github.com/vibrantgio/components/list"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Column declares one column of a Table. Cell is invoked once per visible
// row per frame; the returned layout.Widget is constrained to the column's
// computed width and a fixed row height. Width is a hint: a non-zero
// value pins the column to that pixel width; zero flexes the column
// equally with other flexed columns. Sortable=true makes the header
// clickable and draws a sort chevron when this column is the active sort.
type Column[T any] struct {
	Header   string
	Cell     func(item T) layout.Widget
	Width    unit.Dp
	Sortable bool
}

// Sort describes the table's current display sort. Column is the
// zero-based column index, or -1 to indicate no active sort. Asc=true
// renders the chevron pointing up.
type Sort struct {
	Column int
	Asc    bool
}

// Props configures a Table[T]. Columns is read once per emission (cheap
// for typical column counts); Items is the data slice, already filtered
// and sorted by the consumer.
type Props[T any] struct {
	Columns []Column[T]

	// Items is the row data, already filtered and sorted. The table
	// renders only what it receives — sort/filter state is the
	// consumer's responsibility. Required.
	Items rx.Observable[[]T]

	// Sort drives the header chevron's display state. A nil Sort is
	// treated as a constant Sort{Column: -1} (no active sort).
	Sort rx.Observable[Sort]

	// OnSort is invoked when the user clicks a Sortable header. May be
	// nil. The consumer typically cycles None → Asc → Desc → None for
	// the clicked column and re-emits Sort and a re-sorted Items slice.
	OnSort func(gtx layout.Context, col int)

	// Unemphasized draws the current row the way the platform draws a
	// selection in a window that is not frontmost: the unemphasized grey
	// under the ordinary label, rather than the accent-following selection
	// colour under the foreground the platform pairs with it. The zero
	// value is the frontmost window.
	Unemphasized bool

	// Current marks the row the window is currently showing — the record
	// open in a detail pane beside the table, the item a reader navigated
	// to. It is called once per VISIBLE row per frame and its row is filled
	// with the platform's selection colour before the cells draw. Nil (the
	// default) marks nothing.
	//
	// This is a display mark, not selection state: the table stores nothing,
	// the predicate answers from whatever the consumer already holds, and a
	// row scrolled out of the viewport costs nothing because it is never
	// asked. Keyboard traversal over rows is still unbuilt; build it on
	// components/list's LayoutSelectable, which moves an index over every
	// row, not on a focus tag per row.
	Current func(item T) bool

	// Shaper is an explicit per-instance override of the text shaper. Leave
	// it nil in normal use: the table then shapes its header labels with the
	// theme's shaper (Typography.Shaper()), which is built once for the
	// process and shared by every component reading that typography — the
	// cache lives behind the Typography value, so it survives the copy this
	// component's map function makes of it. Set it only when this instance
	// must shape with a different shaper than the theme provides.
	//
	// A shaper is not safe to use from two goroutines; Gio lays every
	// layout.Widget out on the one goroutine that runs the event loop,
	// which is what makes sharing it correct. See theme/tokens.Typography.Shaper.
	Shaper *text.Shaper
}

// Layout-affecting constants. Row and header heights come from the
// density: both are exactly Density.RowHeight, the platform's own list row,
// so the body's vertical extent stays deterministic and the components/list
// viewport can serve constant-time look-aheads. Sortable header cells
// tile the header band edge to edge (like stacked rows, extending their
// pointer area would steal a neighbour's slop), so their hit area stays
// the cell bounds. cellPadDp is the horizontal cell padding — 12 dp,
// shadcn's px-3 on inputs, which does not follow density.
const (
	cellPadDp     = 12
	chevronSizeDp = 10
	seamDp        = 1
	minColumnDp   = 64
)

type resolvedTokens struct {
	color   tokens.PlatformColors
	spacing tokens.SpacingScale
	header  tokens.TextStyle // the LabelLarge role: typeface, weight, size, line height
	density tokens.Density   // row/header height source
	shaper  *text.Shaper     // the theme's shaper; nil in the Render path
	// unemphasized draws the current row's fill the way the platform draws
	// a selection in a window that is not frontmost (`Props.Unemphasized`).
	unemphasized bool
}

// Table returns an rx.Observable[layout.Widget] that emits a new one
// whenever a consumed theme token, Items, or Sort changes. Header clicks
// invoke OnSort; the body is laid out via components/list.Layout so per-frame
// cost is O(visible-rows), not O(len(items)).
func Table[T any](th rx.Observable[theme.Theme], props Props[T]) rx.Observable[layout.Widget] {
	items := props.Items
	if items == nil {
		items = rx.Of[[]T](nil)
	}
	sort := props.Sort
	if sort == nil {
		sort = rx.Of(Sort{Column: -1})
	}
	// Flatten the nested theme observables into a concrete snapshot. The
	// typography emission supplies both the LabelLarge text style for the
	// header and the theme's cached shaper.
	tokensObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest4(t.Platform, t.Spacing, t.Typography, t.Density),
			func(n rx.Tuple4[tokens.PlatformColors, tokens.SpacingScale, tokens.Typography, tokens.Density]) resolvedTokens {
				typ := n.Third
				return resolvedTokens{
					color:        n.First,
					spacing:      n.Second,
					header:       typ.LabelLarge,
					density:      n.Fourth,
					shaper:       typ.Shaper(),
					unemphasized: props.Unemphasized,
				}
			},
		)
	})
	inputs := rx.CombineLatest3(tokensObs, items, sort)
	return rx.Defer(func() rx.Observable[layout.Widget] {
		state := list.NewState()
		rows := &rowIndex{}
		clicks := make([]widget.Clickable, len(props.Columns))
		return rx.Map(inputs, func(n rx.Tuple3[resolvedTokens, []T, Sort]) layout.Widget {
			tok, items, sk := n.First, n.Second, n.Third
			shaper := props.Shaper
			if shaper == nil {
				shaper = tok.shaper
			}
			return func(gtx layout.Context) layout.Dimensions {
				processHeaderClicks(gtx, props.Columns, clicks, props.OnSort)
				return drawTable(gtx, shaper, props.Columns, items, sk, state, rows, clicks, tok, props.Current)
			}
		})
	})
}

// Render produces a layout.Widget for a table with a fixed dataset and
// pre-resolved tokens. Intended for golden-image testing and static
// demonstrations; production code should use Table, which reads both of the
// parameters below off the theme.
//
// header is the LabelLarge role's whole text style — typeface, weight, size
// and line height all reach the shaper — and d is the density the grid draws
// at (header row and body rows are each exactly Density.RowHeight). Pass
// tokens.DefaultTypography.LabelLarge and tokens.Comfortable for the default
// desktop look. A zero header Weight falls back to bold, so a hand-built
// size-only style still renders with the bold weight.
func Render[T any](
	shaper *text.Shaper,
	columns []Column[T],
	items []T,
	sk Sort,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	header tokens.TextStyle,
	d tokens.Density,
) layout.Widget {
	tok := resolvedTokens{color: colors, spacing: sp, header: header, density: d}
	state := list.NewState()
	rows := &rowIndex{}
	return func(gtx layout.Context) layout.Dimensions {
		return drawTable(gtx, shaper, columns, items, sk, state, rows, nil, tok, nil)
	}
}

// processHeaderClicks drains pending click events for each header. Only
// columns marked Sortable participate; non-sortable columns have a nil
// click in the slice so any pointer event is ignored.
func processHeaderClicks[T any](
	gtx layout.Context,
	columns []Column[T],
	clicks []widget.Clickable,
	onSort func(gtx layout.Context, col int),
) {
	for i := range columns {
		if !columns[i].Sortable {
			continue
		}
		if clicks[i].Clicked(gtx) && onSort != nil {
			onSort(gtx, i)
		}
	}
}

// rowIndex caches the row positions 0..n-1 so the body can be laid out over
// positions rather than over items: components/list hands its row function
// the item and not the position, and the alternating stripe is a property of
// the position. The slice is rebuilt only when the row count changes, so the
// per-frame cost stays O(visible); its memory is one int per row beside the
// caller's own slice.
type rowIndex struct{ idx []int }

func (r *rowIndex) upTo(n int) []int {
	if len(r.idx) != n {
		r.idx = make([]int, n)
		for i := range r.idx {
			r.idx[i] = i
		}
	}
	return r.idx
}

// drawTable renders the full table: header row + virtualised body. Width
// is partitioned across columns once per frame (O(cols), independent of
// row count); the body is laid out via components/list so only viewport-
// visible rows incur per-row cost.
func drawTable[T any](
	gtx layout.Context,
	shaper *text.Shaper,
	columns []Column[T],
	items []T,
	sk Sort,
	state *list.State,
	rows *rowIndex,
	clicks []widget.Clickable,
	tok resolvedTokens,
	current func(item T) bool,
) layout.Dimensions {
	size := gtx.Constraints.Max
	// The grid is printed on the platform's content fill, which is what a
	// list, a table and a text view all stand on there.
	paint.FillShape(gtx.Ops, tok.color.ControlBackground, clip.Rect{Max: size}.Op())

	widths := columnWidths(gtx, columns, size.X)
	// The header is a row in the grid, so its height is exactly
	// Density.RowHeight, like the body rows below it.
	headerH := gtx.Dp(unit.Dp(tok.density.RowHeight))
	if headerH > size.Y {
		headerH = size.Y
	}

	// Header row.
	hSize := image.Pt(size.X, headerH)
	hStack := op.Offset(image.Point{}).Push(gtx.Ops)
	hGtx := gtx
	hGtx.Constraints = layout.Exact(hSize)
	drawHeaderRow(hGtx, shaper, columns, widths, sk, clicks, tok)
	hStack.Pop()

	// Body.
	bodyY := headerH
	bodyH := size.Y - bodyY
	if bodyH <= 0 {
		return layout.Dimensions{Size: size}
	}
	bStack := op.Offset(image.Pt(0, bodyY)).Push(gtx.Ops)
	bGtx := gtx
	bGtx.Constraints = layout.Exact(image.Pt(size.X, bodyH))
	list.Layout(bGtx, state, rows.upTo(len(items)), func(rGtx layout.Context, i int) layout.Dimensions {
		item := items[i]
		return drawRow(rGtx, columns, widths, item, tok, current != nil && current(item), i%2 == 1)
	})
	bStack.Pop()

	return layout.Dimensions{Size: size}
}

// columnWidths resolves the per-column pixel widths. Explicit Width
// values are honoured; zero-width columns share the remainder equally.
// Returns a slice of length len(columns); element widths sum to totalW
// (modulo integer rounding).
func columnWidths[T any](gtx layout.Context, columns []Column[T], totalW int) []int {
	n := len(columns)
	out := make([]int, n)
	if n == 0 || totalW <= 0 {
		return out
	}
	minW := gtx.Dp(unit.Dp(minColumnDp))
	used := 0
	flexed := 0
	for i := range columns {
		if columns[i].Width > 0 {
			w := gtx.Dp(columns[i].Width)
			if w < minW {
				w = minW
			}
			out[i] = w
			used += w
		} else {
			flexed++
		}
	}
	remaining := totalW - used
	if flexed > 0 {
		if remaining < flexed*minW {
			remaining = flexed * minW
		}
		share := remaining / flexed
		extra := remaining - share*flexed
		for i := range columns {
			if columns[i].Width == 0 {
				w := share
				if extra > 0 {
					w++
					extra--
				}
				out[i] = w
			}
		}
	}
	return out
}

// drawHeaderRow renders the bold-weight header labels with optional sort
// chevrons and clickable hit areas for sortable columns. The trailing
// seam marks the boundary between the header and the body.
func drawHeaderRow[T any](
	gtx layout.Context,
	shaper *text.Shaper,
	columns []Column[T],
	widths []int,
	sk Sort,
	clicks []widget.Clickable,
	tok resolvedTokens,
) layout.Dimensions {
	size := gtx.Constraints.Max
	// The header band stands on the grid's own plane and takes no fill of
	// its own: on this platform a table's header is the content's fill
	// under the header text, closed by a separator along its foot. That
	// hairline is the header's own and is drawn below.
	paint.FillShape(gtx.Ops, tok.color.ControlBackground, clip.Rect{Max: size}.Op())

	x := 0
	for i, col := range columns {
		w := widths[i]
		if w <= 0 {
			continue
		}
		st := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		cellGtx := gtx
		cellGtx.Constraints = layout.Exact(image.Pt(w, size.Y))
		drawHeaderCell(cellGtx, shaper, col, i == sk.Column, sk.Asc, clickFor(clicks, i, col.Sortable), tok)
		st.Pop()
		x += w
	}

	seamH := gtx.Dp(unit.Dp(seamDp))
	if seamH < 1 {
		seamH = 1
	}
	seamRect := image.Rect(0, size.Y-seamH, size.X, size.Y)
	paint.FillShape(gtx.Ops, vgcolor.Flatten(tok.color.Separator, tok.color.ControlBackground), clip.Rect(seamRect).Op())

	return layout.Dimensions{Size: size}
}

// drawHeaderCell renders one header label + optional sort chevron inside
// a fixed-size column box, wiring a Clickable if the column is sortable.
func drawHeaderCell[T any](
	gtx layout.Context,
	shaper *text.Shaper,
	col Column[T],
	active bool,
	asc bool,
	click *widget.Clickable,
	tok resolvedTokens,
) layout.Dimensions {
	size := gtx.Constraints.Max
	padH := gtx.Dp(unit.Dp(cellPadDp))

	inner := func(gtx layout.Context) layout.Dimensions {
		// Label.
		labelMaxW := size.X - 2*padH
		if active && col.Sortable {
			labelMaxW -= gtx.Dp(unit.Dp(chevronSizeDp)) + padH/2
		}
		if labelMaxW > 0 {
			labelGtx := gtx
			labelGtx.Constraints.Min = image.Point{}
			labelGtx.Constraints.Max.X = labelMaxW
			labelGtx.Constraints.Max.Y = size.Y

			mColor := op.Record(gtx.Ops)
			paint.ColorOp{Color: vgcolor.Flatten(tok.color.HeaderText, tok.color.ControlBackground)}.Add(gtx.Ops)
			material := mColor.Stop()

			// Shape with the LabelLarge role's typeface, weight, size and
			// line height. A zero Weight (a hand-built size-only style)
			// keeps the header's bold weight.
			style := tok.header
			f := typeset.Font(style, font.Bold)
			wl := typeset.Label(style, 1)
			mLabel := op.Record(gtx.Ops)
			labelDims := typeset.Layout(
				labelGtx,
				shaper,
				wl,
				f,
				unit.Sp(style.Size),
				col.Header,
				material,
			)
			labelCall := mLabel.Stop()

			offY := (size.Y - labelDims.Size.Y) / 2
			if offY < 0 {
				offY = 0
			}
			st := op.Offset(image.Pt(padH, offY)).Push(gtx.Ops)
			labelCall.Add(gtx.Ops)
			st.Pop()
		}

		if active && col.Sortable {
			chev := gtx.Dp(unit.Dp(chevronSizeDp))
			cx := size.X - padH - chev/2
			cy := size.Y / 2
			drawSortChevron(gtx, cx, cy, chev, vgcolor.Flatten(tok.color.HeaderText, tok.color.ControlBackground), asc)
		}

		return layout.Dimensions{Size: size}
	}

	gtx.Constraints = layout.Exact(size)
	if click == nil {
		return inner(gtx)
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(col.Header).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		return inner(gtx)
	})
}

// drawRow renders one body row by invoking each column's Cell closure
// inside a fixed-size cell box, then painting the bottom seam.
// rowH is the density's row height (Density.RowHeight, the platform's own
// list row), not the cell's intrinsic size, so per-row layout cost stays
// bounded regardless of cell content. Rows are stacked full-width strips:
// their hit area stays the row bounds (no 44 dp extension — rows would steal
// each other's slop).
//
// odd stripes the row: the platform lays its alternating content background
// under every second row of a list, which is what Finder's list view draws.
// current then fills the row with the platform's selection colour BEFORE the
// cells draw, so a Cell closure's own painting still lands on top of it and
// the grid line still closes the row underneath.
func drawRow[T any](
	gtx layout.Context,
	columns []Column[T],
	widths []int,
	item T,
	tok resolvedTokens,
	current bool,
	odd bool,
) layout.Dimensions {
	rowH := gtx.Dp(unit.Dp(tok.density.RowHeight))
	totalW := gtx.Constraints.Max.X
	rowSize := image.Pt(totalW, rowH)

	if odd {
		paint.FillShape(gtx.Ops, vgcolor.Flatten(tok.color.AlternatingContentBackground, tok.color.ControlBackground),
			clip.Rect{Max: rowSize}.Op())
	}
	if current {
		paint.FillShape(gtx.Ops, rowFill(tok), clip.Rect{Max: rowSize}.Op())
	}

	x := 0
	for i, col := range columns {
		w := widths[i]
		if w <= 0 {
			continue
		}
		st := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		cellGtx := gtx
		cellGtx.Constraints = layout.Exact(image.Pt(w, rowH))
		if col.Cell != nil {
			cw := col.Cell(item)
			if cw != nil {
				cw(cellGtx)
			}
		}
		st.Pop()
		x += w
	}

	seamH := gtx.Dp(unit.Dp(seamDp))
	if seamH < 1 {
		seamH = 1
	}
	seamRect := image.Rect(0, rowH-seamH, totalW, rowH)
	paint.FillShape(gtx.Ops, tok.color.Grid, clip.Rect(seamRect).Op())

	return layout.Dimensions{Size: rowSize}
}

// rowFill is what the current row is filled with: the platform's selection
// colour, which follows the accent, or its unemphasized grey where the
// window is not the frontmost one.
func rowFill(tok resolvedTokens) color.NRGBA {
	if tok.unemphasized {
		return tok.color.UnemphasizedSelectedContentBackground
	}
	return tok.color.SelectedContentBackground
}

// RenderTextCell renders a single line of text in the platform's label
// within the cell's allocated rectangle, with horizontal padding equal to
// cellPadDp. Exported so consumers building their own Cell closures can
// match the table's stock text style.
//
// body is the BodyMedium role's whole text style — typeface, weight, size
// and line height all reach the shaper. Pass
// tokens.DefaultTypography.BodyMedium for the default desktop look. There
// is no density parameter: the row that owns the cell is what density
// sizes, and [Render] takes it there; a cell only fills the rectangle it
// is handed.
func RenderTextCell(
	shaper *text.Shaper,
	colors tokens.PlatformColors,
	body tokens.TextStyle,
	s string,
) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		padH := gtx.Dp(unit.Dp(cellPadDp))
		labelMaxW := size.X - 2*padH
		if labelMaxW <= 0 {
			return layout.Dimensions{Size: size}
		}
		labelGtx := gtx
		labelGtx.Constraints.Min = image.Point{}
		labelGtx.Constraints.Max.X = labelMaxW
		labelGtx.Constraints.Max.Y = size.Y

		mColor := op.Record(gtx.Ops)
		paint.ColorOp{Color: vgcolor.Flatten(colors.Label, colors.ControlBackground)}.Add(gtx.Ops)
		material := mColor.Stop()

		// Shape with the BodyMedium role's typeface, weight, size and line
		// height. A zero Weight (a hand-built size-only style) keeps the
		// shaper's default weight, as this cell always did.
		f := typeset.Font(body, font.Normal)
		wl := typeset.Label(body, 1)
		mLabel := op.Record(gtx.Ops)
		labelDims := typeset.Layout(labelGtx, shaper, wl, f, unit.Sp(body.Size), s, material)
		labelCall := mLabel.Stop()

		offY := (size.Y - labelDims.Size.Y) / 2
		if offY < 0 {
			offY = 0
		}
		st := op.Offset(image.Pt(padH, offY)).Push(gtx.Ops)
		labelCall.Add(gtx.Ops)
		st.Pop()
		return layout.Dimensions{Size: size}
	}
}

// drawSortChevron paints a small filled triangle centred at (cx, cy)
// pointing up (asc) or down (desc).
func drawSortChevron(gtx layout.Context, cx, cy, sz int, c color.NRGBA, asc bool) {
	half := float32(sz) / 2
	fcx := float32(cx)
	fcy := float32(cy)
	var p clip.Path
	p.Begin(gtx.Ops)
	if asc {
		p.MoveTo(f32.Pt(fcx-half, fcy+half/2))
		p.LineTo(f32.Pt(fcx, fcy-half/2))
		p.LineTo(f32.Pt(fcx+half, fcy+half/2))
	} else {
		p.MoveTo(f32.Pt(fcx-half, fcy-half/2))
		p.LineTo(f32.Pt(fcx, fcy+half/2))
		p.LineTo(f32.Pt(fcx+half, fcy-half/2))
	}
	p.Close()
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
}

func clickFor(clicks []widget.Clickable, i int, sortable bool) *widget.Clickable {
	if !sortable || i < 0 || i >= len(clicks) {
		return nil
	}
	return &clicks[i]
}
