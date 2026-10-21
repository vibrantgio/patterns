// threecolumn.go is the shell's three-column arrangement: the pane down
// the leading edge, a list column of absolute width beside it on a
// splitter, and the content column absorbing the rest, with one toolbar
// across the list and the content and one share per column. It is the
// arrangement Mail, Notes and Voice Memos are built in.
//
// WHAT THE MEASUREMENTS ARE. Every number [PlatformList] carries was read
// off a capture of Mail taken on this machine at 1x in both appearances,
// stored as reference/macos/mail-three-column-light.png and
// mail-three-column-dark.png, with the reading in
// reference/macos/controls.md. Where the capture and a stated value
// disagree the capture wins.
//
// WHAT THE ARRANGEMENT DRAWS AND WHAT THE WINDOW DRAWS is [TwoColumn]'s
// division: the arrangement owns where the columns stand, which boundaries
// the reader may move and which fill each share of the toolbar stands on;
// what stands in the toolbar and in each column is the window's, handed in
// as slots.

package shell

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/splitter"
	"github.com/vibrantgio/theme/tokens"
)

// ListMetrics are the list column's own measurements. A window states a
// deviation rather than a value, and [PlatformList] is what it deviates
// from.
type ListMetrics struct {
	// Width is the width the column opens at where the window kept none.
	Width unit.Dp

	// Min is the narrowest the column goes. Below it the platform takes
	// the room out of the pane instead of out of the list.
	Min unit.Dp

	// ContentFloor is the narrowest the content column beside it lays out
	// at. It is what stands in for a maximum, because the platform states
	// no maximum for the list: the column grows until the content beside
	// it reaches this, and no further.
	ContentFloor unit.Dp
}

// PlatformList is the platform's own reading of those three, MEASURED off
// Mail at 1x in both appearances on a window 1400 wide:
//
//   - Width 447: the list runs from the pane's trailing seam at x=332 to
//     the seam at x=779.
//   - Min 200: the trailing seam dragged to the window's leading edge
//     stops with the list 200 wide, and what is dragged out after that
//     comes out of the pane.
//   - ContentFloor 360: the seam dragged the other way stops with the
//     content column 360 wide, and a window narrowed to 1000 squeezes the
//     list to 307 to leave the content that same 360 — then hands the
//     list its 447 back as soon as the window has room for it again.
var PlatformList = ListMetrics{Width: 447, Min: 200, ContentFloor: 360}

// resolve answers m with every unstated measurement taken from the
// platform's reading.
func (m ListMetrics) resolve() ListMetrics {
	if m.Width <= 0 {
		m.Width = PlatformList.Width
	}
	if m.Min <= 0 {
		m.Min = PlatformList.Min
	}
	if m.ContentFloor <= 0 {
		m.ContentFloor = PlatformList.ContentFloor
	}
	return m
}

// ListColumn is what stands inside the list column: the header across its
// top, the rows under it, and the footer under those.
//
// Mail fills none of the three but Rows: its mailbox name and its filter
// stand in the toolbar's share over this column, not in a header of its
// own, which is why the header is a slot and not a row the arrangement
// draws. A window whose list names itself inside the column — a filter
// field it wants under the toolbar rather than in it — fills Header.
type ListColumn struct {
	// Header is the row at the top of the column holding what names it.
	// Nil takes no height at all.
	Header layout.Widget

	// HeaderDepth is how deep that row lays out. Zero with a Header set
	// takes the depth the slot reports.
	HeaderDepth unit.Dp

	// Rows is the collection the column shows. It is the one slot that
	// stretches, so it runs from under the header to the footer or to the
	// column's own foot.
	Rows layout.Widget

	// Footer is the slot under the rows: a hairline the column's own width
	// and the slot under it, as the pane's footer is. Nil takes no height
	// at all, hairline included.
	Footer layout.Widget

	// FooterDepth is how deep that slot lays out. Zero with a Footer set
	// takes the depth the slot reports.
	FooterDepth unit.Dp
}

// ListColumnLayout is what one frame arranged inside the list column, in
// WINDOW coordinates. An empty rectangle is a slot nothing stands in.
type ListColumnLayout struct {
	Header, Rows, Footer image.Rectangle
}

// ThreeColumnLayout is the arrangement one frame laid out, in WINDOW
// coordinates.
//
// It is its own type rather than [PaneLayout] with fields added, because
// the two arrangements do not report the same things: this one has two
// toolbar shares, two columns of its own and no inspector, and a
// [PaneLayout] carrying a list rectangle would say every two-column window
// has a list standing empty.
type ThreeColumnLayout struct {
	// Pane is the pane's own rectangle, empty where the pane is away — the
	// emptiness IS the contract, and the list column beside it reflows to
	// the window's own edge.
	Pane image.Rectangle

	// ListX and ContentX are the leading edges the two columns start from.
	// The seam stands between them: it is the pixel at ContentX-1 and it
	// belongs to neither column.
	ListX, ContentX int

	// RowTop is the line both columns' content starts on — the toolbar's
	// own depth — and RowH and FootTop the height of the content column's
	// document row and the line its footer begins on. The list column runs
	// from RowTop to the window's foot: its footer is its own and the
	// content column's status bar is not over it.
	RowTop, RowH, FootTop int

	// ListToolbar and Toolbar are the toolbar's two shares, one per
	// column. They are the one strip and the seam parts them.
	ListToolbar, Toolbar image.Rectangle

	// List and Main are the two columns' document boxes, and Footer the
	// content column's footer. An empty rectangle is a slot nothing stands
	// in.
	List, Main, Footer image.Rectangle

	// Column and ListColumn are what the pane's column and the list column
	// arranged inside themselves.
	Column     PaneColumnLayout
	ListColumn ListColumnLayout

	// ListFill and ContentFill are the fills the two toolbar shares stand
	// on. A toolbar carries no fill of its own, so what a control standing
	// in it flattens its foreground onto is the fill of the column under
	// it, continued upward; [ThreeColumnLayout.ToolbarFillAt] is the
	// reading. MEASURED off Mail, the two are the same colour in both
	// appearances — #ffffff light and #232a2e dark — and the seam between
	// them is the whole of what says where the boundary is.
	ListFill, ContentFill color.NRGBA
}

// ToolbarFillAt answers what a control standing at column x of the window
// flattens its foreground onto.
func (g ThreeColumnLayout) ToolbarFillAt(x int) color.NRGBA {
	if x < g.ContentX {
		return g.ListFill
	}
	return g.ContentFill
}

// ListResize is the splitter on the list column's trailing edge.
//
// What parts the list from the content is the platform's plain seam, so
// this boundary draws the seam's own colour, as the inspector's does and
// unlike the pane's, which rides the pane's rim.
//
// The line runs the window's WHOLE height, the toolbar included. MEASURED
// off Mail: at x=779 the seam reads #e6e6e6 light and #000000 dark at
// every row from the window's top edge to its foot, under the toolbar as
// well as beside the rows. The two columns are two regions floor to
// ceiling and the one toolbar lies across both of them. The hand-hold is
// shorter than the line: the toolbar is the window's own strip and carries
// the window's drag, so the hold is the document row alone.
type ListResize struct {
	// State is the hand across frames, held by the window.
	State *splitter.State

	// Min and Max are the list column's own bounds. A zero maximum states
	// no ceiling, which is the platform's reading: the column grows until
	// the content beside it reaches [ThreeColumn.ContentFloor], and that
	// floor is the whole of the ceiling.
	Min, Max unit.Dp

	// OnChange reports the column's new width, already clamped, for every
	// pointer event that moves it.
	OnChange func(width unit.Dp)

	at      unit.Dp
	dragged bool
}

// moved and width are [PaneResize.moved] and [PaneResize.width] for the
// list column's boundary.
func (r *ListResize) moved(w unit.Dp) {
	r.at, r.dragged = w, true
	if r.OnChange != nil {
		r.OnChange(w)
	}
}

func (r *ListResize) width(have unit.Dp) unit.Dp {
	if r == nil || !r.dragged {
		return have
	}
	r.dragged = false
	return r.at
}

// ThreeColumn is the shell's three-column arrangement: the window's own
// fill under everything, the pane one margin inside the leading, top and
// bottom edges, the list column flush against its trailing side at an
// absolute width, the seam, and the content column absorbing the rest.
//
// The list column's width is ABSOLUTE and keeps itself across a window
// resize: the content column absorbs the change, and the list gives way
// only where the window has no room left for the content's own floor.
// MEASURED off Mail, which holds its list at 447 from a window 1400 wide
// to one 1800 wide, squeezes it to 307 at 1000 to leave the content 360,
// and hands the 447 back when the window widens again.
type ThreeColumn struct {
	// PaneWidth is the pane's own width, rim to rim. Zero takes
	// [PaneWidthDp].
	PaneWidth unit.Dp

	// PaneHidden sends the pane out of the window: it takes no width at
	// all and the list column reflows from the window's own leading edge,
	// taking the window's control buttons and the Toggle Sidebar button
	// into its own share of the toolbar with it.
	PaneHidden bool

	// Pane is the column standing inside the pane.
	Pane PaneColumn

	// PaneResize is the splitter on the pane's trailing edge. Nil is a
	// pane whose width is fixed.
	PaneResize *PaneResize

	// ListWidth is the list column's own width. Zero takes
	// [ListMetrics.Width].
	ListWidth unit.Dp

	// ContentFloor is the narrowest the content column lays out at: what
	// the list column may not squeeze it under, both on a resize and under
	// a hand on the seam. Zero takes [ListMetrics.ContentFloor].
	//
	// It is stated once here rather than on each boundary, because it is
	// one measure of one column and two statements of it could disagree.
	ContentFloor unit.Dp

	// List is the column standing between the pane and the content.
	List ListColumn

	// ListResize is the splitter on the list column's trailing edge. Nil
	// is a list whose width is fixed.
	ListResize *ListResize

	// Surface is the window's own fill, painted under everything — what
	// shows in the margins around the pane. A window already standing on
	// its surface leaves it zero.
	Surface color.NRGBA

	// ListFill is the list column's own fill, painted the window's full
	// height so the column does not read as a block hanging off the
	// toolbar. Zero takes ContentFill, which is what Mail reads: the two
	// columns carry one fill and the seam is what parts them.
	ListFill color.NRGBA

	// ContentFill is the content column's own fill, painted from the
	// seam to the window's trailing edge and running its full height. It
	// is also what stands behind the two corners the pane rounds away from
	// on its flush side, so neither reads as a nick bitten out of the
	// boundary.
	ContentFill color.NRGBA

	// ListToolbar is the toolbar's share over the list column and Toolbar
	// its share over the content. They are two values and not two groups
	// of one, because MEASURED off Mail each share is a composition in its
	// own right: over the list stand the mailbox name at the share's
	// leading end and the filter and view-options controls at its
	// trailing, and over the content the new-message and message acts
	// leading with the search field trailing. One value with a list group
	// added could state only one end of the list's share.
	ListToolbar, Toolbar Toolbar

	// Toggle is the one Toggle Sidebar button, drawn bare at the pane's
	// trailing corner while the pane stands and bordered at the leading
	// end of the list's share of the toolbar once it is away. Nil is a
	// pane that cannot be sent away.
	Toggle *ToggleSidebar

	// Radius and Density resolve the bordered drawing of that button.
	Radius  tokens.RadiusScale
	Density tokens.Density

	// Main is the content column under the toolbar.
	Main layout.Widget

	// MainFirst records the main slot and replays it under the toolbar's
	// ops, as [TwoColumn.MainFirst] does.
	MainFirst bool

	// Footer is the slot under the content column, across that column's
	// width alone, at FooterDepth. Nil takes no height. The list column's
	// own footer is [ListColumn.Footer] and runs under the list alone.
	Footer      layout.Widget
	FooterDepth unit.Dp

	// Over is laid out last of all, after the shadow, at the box the two
	// document columns share: the surface a control in either share of the
	// toolbar hangs over what stands below it.
	Over layout.Widget

	// WindowButtonsEnd pins the trailing edge of the window's control
	// buttons instead of measuring it, as [TwoColumn.WindowButtonsEnd]
	// does. A live window leaves it nil and measures.
	WindowButtonsEnd func() unit.Dp

	// Memory keeps the pane's width, the list column's width and the
	// pane's shown state across launches. A nil memory is a working one.
	Memory *ArrangementMemory

	// Report hands the arrangement back once it is laid out.
	Report func(ThreeColumnLayout)
}

// metrics answers the list column's three measurements with the window's
// deviations over the platform's reading.
func (f ThreeColumn) metrics() ListMetrics {
	return ListMetrics{Width: f.ListWidth, ContentFloor: f.ContentFloor}.resolve()
}

// Measure answers the arrangement at a window of the given size without
// laying anything out, so a window can assert where its columns stand
// without drawing them.
func (f ThreeColumn) Measure(gtx layout.Context, size image.Point) ThreeColumnLayout {
	h := max(size.Y, 0)
	barH := min(max(f.toolbarDepth(gtx), 0), h)
	footH := 0
	if f.Footer != nil {
		footH = min(max(gtx.Dp(f.FooterDepth), 0), h-barH)
	}
	g := ThreeColumnLayout{RowTop: barH, RowH: h - barH - footH, FootTop: h - footH}

	w := f.PaneWidth
	if w <= 0 {
		w = PaneWidthDp
	}
	g.Pane = pane.Bounds(gtx, size, w, f.PaneHidden)
	g.ListX = ContentX(g.Pane)
	listW := f.listWidth(gtx, size, g.ListX)
	g.ContentX = min(g.ListX+listW+splitter.SeamWidth(gtx), size.X)

	if barH > 0 {
		if listW > 0 {
			g.ListToolbar = toolbarBounds(g.ListX, listW, barH)
		}
		if rowW := max(size.X-g.ContentX, 0); rowW > 0 {
			g.Toolbar = toolbarBounds(g.ContentX, rowW, barH)
		}
	}
	if g.RowH > 0 {
		if listW > 0 {
			// The list column runs to the window's foot: the content
			// column's footer is that column's and stops at the seam.
			g.List = image.Rect(g.ListX, g.RowTop, g.ListX+listW, size.Y)
		}
		if g.ContentX < size.X {
			g.Main = image.Rect(g.ContentX, g.RowTop, size.X, g.RowTop+g.RowH)
		}
	}
	if footH > 0 && g.ContentX < size.X {
		g.Footer = image.Rect(g.ContentX, g.FootTop, size.X, size.Y)
	}
	g.ListFill, g.ContentFill = f.listFill(), f.ContentFill
	return g
}

// listFill answers the list column's fill: its own where it states one,
// and the content column's otherwise — Mail's reading, where the two
// columns carry one fill.
func (f ThreeColumn) listFill() color.NRGBA {
	if f.ListFill.A > 0 {
		return f.ListFill
	}
	return f.ContentFill
}

// toolbarDepth is the one depth both shares stand at: a toolbar is one
// strip across the window and two shares at two depths would be two
// strips. The content column's share states it, and the list's own
// statement is taken only where the content states none.
func (f ThreeColumn) toolbarDepth(gtx layout.Context) int {
	return gtx.Dp(f.Toolbar.depth())
}

// listWidth is what the list column takes at this window size: the width
// it was given, cut to what is left after the pane and the content
// column's own floor, and never under the column's own minimum where the
// window has room for that much.
//
// This is the whole of "absolute, and keeps itself on a resize": the width
// is a number and not a ratio, so a window that grows hands every pixel of
// the growth to the content column, and one that shrinks takes from the
// content column until its floor and only then from the list.
func (f ThreeColumn) listWidth(gtx layout.Context, size image.Point, listX int) int {
	m := f.metrics()
	avail := max(size.X-listX-splitter.SeamWidth(gtx), 0)
	w := min(gtx.Dp(m.Width), avail)
	if room := avail - gtx.Dp(m.ContentFloor); w > room {
		w = max(room, min(gtx.Dp(m.Min), avail))
	}
	return max(w, 0)
}

// buttonsEnd is the trailing edge of the window's control buttons: the
// pinned value where one was given, and the live window's measurement
// otherwise.
func (f ThreeColumn) buttonsEnd() unit.Dp {
	return TwoColumn{WindowButtonsEnd: f.WindowButtonsEnd}.buttonsEnd()
}

// Layout draws the arrangement in the order it reads, which is the focus
// ring's too: the pane's column, the toolbar's two shares in column order,
// the list column, the content column, and last the content column's
// footer, which holds nothing the ring can stop on.
//
// The shadow the pane casts is painted after every column has painted its
// own fill, for [TwoColumn.Layout]'s reason, and the two boundaries after
// even that, so neither the columns' rows nor the toolbar crossing over
// them can cover a line.
func (f ThreeColumn) Layout(gtx layout.Context, c tokens.PlatformColors) (layout.Dimensions, ThreeColumnLayout) {
	size := gtx.Constraints.Max
	g := f.Measure(gtx, size)

	// Both boundaries are taken hold of before anything is laid out, so
	// the whole frame is drawn where this frame's drag left them. The
	// pane's goes first: the list column is measured from where the pane
	// ends, and a pane that moved in this same frame has already moved
	// that.
	if !g.Pane.Empty() && f.PaneResize != nil {
		f.PaneResize.State.Update(gtx, f.paneProps(gtx, c, size, g))
		if w := f.PaneResize.width(f.PaneWidth); w != f.PaneWidth {
			f.PaneWidth = w
			g = f.Measure(gtx, size)
		}
	}
	if f.ListResize != nil {
		f.ListResize.State.Update(gtx, f.listProps(gtx, c, size, g))
		if w := f.ListResize.width(f.ListWidth); w != f.ListWidth {
			f.ListWidth = w
			g = f.Measure(gtx, size)
		}
	}

	// One press on the one button, wherever this frame is about to put it.
	f.Toggle.drain(gtx)

	// What stands under the columns: the window's own fill, the two
	// columns' fills beside the pane, the two corners the pane rounds away
	// from on its flush side, and the pane itself with the column standing
	// in it. The list's fill is painted first and the content's over the
	// rest, both the window's full height, so the toolbar lies on the fill
	// of whichever column is under it.
	column := g.Column
	PaneFrame{
		Surface:     f.Surface,
		ContentFill: f.listFill(),
		Sidebar: func(gtx layout.Context) layout.Dimensions {
			dims, cl := TwoColumn{Pane: f.Pane, Toggle: f.Toggle, PaneHidden: f.PaneHidden,
				Toolbar: f.listToolbar(), WindowButtonsEnd: f.WindowButtonsEnd}.
				paneColumn(gtx, c, g.Pane.Min)
			column = cl
			return dims
		},
	}.Under(gtx, c, size, g.Pane)
	g.Column = column
	if f.ContentFill.A > 0 && g.ContentX < size.X {
		paint.FillShape(gtx.Ops, f.ContentFill,
			clip.Rect(image.Rect(g.ContentX, 0, size.X, size.Y)).Op())
	}

	var (
		mainCall op.CallOp
		recorded bool
	)
	if f.Main != nil && f.MainFirst && !g.Main.Empty() {
		macro := op.Record(gtx.Ops)
		mgtx := gtx
		mgtx.Constraints = layout.Exact(g.Main.Size())
		f.Main(mgtx)
		mainCall = macro.Stop()
		recorded = true
	}

	// The toolbar's two shares, in column order. Only the list's share
	// leads past the window's control buttons, and only while the pane is
	// away: the pane carries them otherwise, and the content's share never
	// stands at the window's leading edge.
	if !g.ListToolbar.Empty() {
		lead := f.listToolbar()
		buttons, gutter := ToolbarLead(gtx, g.ListX, lead.Lead, f.buttonsEnd(), lead.ButtonFallback)
		st := op.Offset(g.ListToolbar.Min).Push(gtx.Ops)
		bgtx := gtx
		bgtx.Constraints = layout.Exact(g.ListToolbar.Size())
		lead.layout(bgtx, buttons, gutter, f.toolbarLead(c))
		st.Pop()
	}
	if !g.Toolbar.Empty() {
		st := op.Offset(g.Toolbar.Min).Push(gtx.Ops)
		bgtx := gtx
		bgtx.Constraints = layout.Exact(g.Toolbar.Size())
		f.Toolbar.layout(bgtx, 0, gtx.Dp(f.Toolbar.Lead), nil)
		st.Pop()
	}

	if !g.List.Empty() {
		st := op.Offset(g.List.Min).Push(gtx.Ops)
		lgtx := gtx
		lgtx.Constraints = layout.Exact(g.List.Size())
		_, cl := f.List.Layout(lgtx, c, f.listFill())
		for _, r := range []*image.Rectangle{&cl.Header, &cl.Rows, &cl.Footer} {
			if !r.Empty() {
				*r = r.Add(g.List.Min)
			}
		}
		g.ListColumn = cl
		st.Pop()
	}

	switch {
	case recorded:
		st := op.Offset(g.Main.Min).Push(gtx.Ops)
		mainCall.Add(gtx.Ops)
		st.Pop()
	case f.Main != nil && !g.Main.Empty():
		st := op.Offset(g.Main.Min).Push(gtx.Ops)
		mgtx := gtx
		mgtx.Constraints = layout.Exact(g.Main.Size())
		f.Main(mgtx)
		st.Pop()
	}

	if f.Footer != nil && !g.Footer.Empty() {
		st := op.Offset(g.Footer.Min).Push(gtx.Ops)
		fgtx := gtx
		fgtx.Constraints = layout.Exact(g.Footer.Size())
		f.Footer(fgtx)
		st.Pop()
	}

	if !g.Pane.Empty() {
		pane.PaintShadow(gtx, c, g.Pane)
	}

	if !g.Pane.Empty() && f.PaneResize != nil {
		f.layoutPaneSplitter(gtx, c, size, g)
	}
	if f.ListResize != nil && g.ContentX < size.X {
		f.layoutListSplitter(gtx, c, size, g)
	}

	if f.Over != nil && g.RowH > 0 {
		st := op.Offset(image.Pt(g.ListX, 0)).Push(gtx.Ops)
		ogtx := gtx
		ogtx.Constraints = layout.Exact(image.Pt(max(size.X-g.ListX, 0), size.Y))
		f.Over(ogtx)
		st.Pop()
	}

	// What this frame arranged, offered to the memory that keeps it.
	f.Memory.Record(Arrangement{
		PaneWidth: f.PaneWidth,
		ListWidth: f.ListWidth,
		PaneShown: !f.PaneHidden,
	})
	if f.Report != nil {
		f.Report(g)
	}
	return layout.Dimensions{Size: size}, g
}

// listToolbar is the list's share with the content share's depth over it,
// so one strip stands at one depth however the two shares are stated.
func (f ThreeColumn) listToolbar() Toolbar {
	t := f.ListToolbar
	t.Metrics.Depth = f.Toolbar.depth()
	return t
}

// toolbarLead is what the arrangement puts in front of the list share's
// own leading group: the Toggle Sidebar button, bordered, and only while
// the pane is away. The list is the leading column then, so the button
// stands at its share's leading end and not the content share's.
func (f ThreeColumn) toolbarLead(c tokens.PlatformColors) []layout.Widget {
	t := f.Toggle
	if t == nil || t.Click == nil || !f.PaneHidden {
		return nil
	}
	return []layout.Widget{func(gtx layout.Context) layout.Dimensions {
		return ToolbarControl(gtx, c, f.Radius, f.Density, t.Click, t.Mark, t.ToolbarTitle, false)
	}}
}

// Layout draws the list column at the size it is given and reports what it
// arranged, in the column's OWN coordinates — [ThreeColumn] carries the
// result into the window's.
//
// standsOn is the opaque fill the column is drawn over, which its footer's
// own hairline is flattened onto. The zero value — no colour — is the
// platform's content fill.
//
// It is exported for [PaneColumn.Layout]'s reason: the column is a
// composition in its own right, and a test drives one without a window
// around it.
func (l ListColumn) Layout(gtx layout.Context, c tokens.PlatformColors, standsOn color.NRGBA) (layout.Dimensions, ListColumnLayout) {
	size := gtx.Constraints.Max
	var headH, rowsH, footH int
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if l.Header == nil {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
			}
			if d := l.HeaderDepth; d > 0 {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(d)))
			}
			dims := l.Header(gtx)
			headH = dims.Size.Y
			return dims
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if l.Rows == nil {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			dims := l.Rows(gtx)
			rowsH = dims.Size.Y
			return dims
		}),
	}
	if l.Footer != nil {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := l.footer(gtx, c, standsOn)
			footH = dims.Size.Y
			return dims
		}))
	}
	layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)

	rowsTop := min(headH, size.Y)
	cl := ListColumnLayout{Rows: image.Rect(0, rowsTop, size.X, min(rowsTop+rowsH, size.Y))}
	if headH > 0 {
		cl.Header = image.Rect(0, 0, size.X, rowsTop)
	}
	if footH > 0 {
		cl.Footer = image.Rect(0, max(size.Y-footH, 0), size.X, size.Y)
	}
	return layout.Dimensions{Size: size}, cl
}

// footer draws the list column's footer: a hairline the column's width and
// the slot under it, which is the pane's footer's own composition — one
// window states one footer and the two must read alike.
func (l ListColumn) footer(gtx layout.Context, c tokens.PlatformColors, standsOn color.NRGBA) layout.Dimensions {
	if standsOn.A == 0 {
		standsOn = c.TextBackground
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return FooterSeam(gtx, c, standsOn)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if d := l.FooterDepth; d > 0 {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(d)))
			}
			return l.Footer(gtx)
		}),
	)
}

// paneProps states the splitter on the pane's trailing edge, with the list
// column and the content column's floor over the pane's own maximum: the
// pane stops where widening it further would take the content under the
// narrowest measure it may lay out at, and the list is absolute and does
// not give way to make room for a wider pane.
func (f ThreeColumn) paneProps(gtx layout.Context, c tokens.PlatformColors, size image.Point, g ThreeColumnLayout) splitter.Props {
	r := f.PaneResize
	seamPx := splitter.SeamWidth(gtx)
	marginPx := gtx.Dp(unit.Dp(pane.MarginDp))
	listW := g.ContentX - g.ListX - seamPx
	have := max(size.X-g.ContentX, 0)

	lo := gtx.Dp(r.Min)
	hi := max(min(gtx.Dp(r.Max), size.X-marginPx-seamPx-listW-floorOver(gtx, f.metrics().ContentFloor, have)), lo)
	scale := PxPerDp(gtx)
	return splitter.Props{
		Axis:     layout.Horizontal,
		Boundary: float32(g.Pane.Max.X - seamPx),
		Min:      float32(marginPx + lo - seamPx),
		Max:      float32(marginPx + hi - seamPx),
		Colors:   c,
		Rest:     pane.RimColor(c),
		Surface:  pane.Surface(c),
		OnChange: func(at float32) {
			r.moved(Clamp{Min: r.Min, Max: r.Max}.clamp(unit.Dp((at + float32(seamPx-marginPx)) / scale)))
		},
	}
}

// layoutPaneSplitter draws that hand-hold along the rows the pane's
// trailing edge runs straight, as [TwoColumn.layoutPaneSplitter] does.
func (f ThreeColumn) layoutPaneSplitter(gtx layout.Context, c tokens.PlatformColors, size image.Point, g ThreeColumnLayout) {
	top, bottom := pane.EdgeSpan(gtx, g.Pane)
	h := bottom - top
	if g.Pane.Dy() <= 0 || h <= 0 {
		return
	}
	st := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
	sgtx := gtx
	sgtx.Constraints = layout.Exact(image.Pt(size.X, h))
	f.PaneResize.State.Layout(sgtx, f.paneProps(gtx, c, size, g))
	st.Pop()
}

// listProps states the splitter on the list column's trailing edge. The
// boundary is the seam's own leading edge — the splitter draws from the
// boundary outward — and the bounds are the column's own with the content
// column's floor over the top of them.
func (f ThreeColumn) listProps(gtx layout.Context, c tokens.PlatformColors, size image.Point, g ThreeColumnLayout) splitter.Props {
	r := f.ListResize
	m := f.metrics()
	seamPx := splitter.SeamWidth(gtx)
	avail := max(size.X-g.ListX-seamPx, 0)
	have := max(size.X-g.ContentX, 0)

	lo := min(gtx.Dp(max(r.Min, m.Min)), avail)
	hi := avail - floorOver(gtx, m.ContentFloor, have)
	if r.Max > 0 {
		hi = min(hi, gtx.Dp(r.Max))
	}
	hi = max(hi, lo)
	scale := PxPerDp(gtx)
	return splitter.Props{
		Axis:     layout.Horizontal,
		Boundary: float32(g.ContentX - seamPx),
		Min:      float32(g.ListX + lo),
		Max:      float32(g.ListX + hi),
		Colors:   c,
		Surface:  f.listFill(),
		HitSpan:  splitter.Span{Min: g.RowTop, Max: size.Y},
		OnChange: func(at float32) {
			r.moved(unit.Dp((at - float32(g.ListX)) / scale))
		},
	}
}

// layoutListSplitter draws the seam the window's full height. The
// inspector's seam stops at the toolbar's foot because nothing parts the
// toolbar over it; this one does not, because MEASURED off Mail it runs
// from the window's top edge to its foot and the toolbar's two shares are
// what it parts up there.
func (f ThreeColumn) layoutListSplitter(gtx layout.Context, c tokens.PlatformColors, size image.Point, g ThreeColumnLayout) {
	sgtx := gtx
	sgtx.Constraints = layout.Exact(size)
	f.ListResize.State.Layout(sgtx, f.listProps(gtx, c, size, g))
}
