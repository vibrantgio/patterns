// twocolumn.go is the shell's two-column arrangement: a pane down the
// leading edge, a content column beside it carrying one toolbar across its
// top, and an optional inspector down the trailing edge. It is the
// arrangement the platform's own sidebar windows stand in, and the whole of
// what a window with a pane shares with every other.
//
// WHAT THE ARRANGEMENT DRAWS AND WHAT THE WINDOW DRAWS. The arrangement
// owns the geometry: which fills stand where, what the pane is, how deep
// the toolbar is, where each column begins and ends, which boundaries the
// reader may move, when the shadow is cast, and where the one Toggle
// Sidebar button stands. What stands IN the toolbar, what the pane's column
// shows, what the content column holds and what the inspector says are the
// window's, handed in as slots.

package shell

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/splitter"
	"github.com/vibrantgio/theme/tokens"
)

// PaneColumn is what stands inside the pane: its own strip across the top,
// then the slots under it in reading order.
//
// The strip is RESERVED by the column's arrangement and DRAWN afterwards,
// which is a statement about the keyboard and not about paint. Focus
// follows the order the ops are written in, and a pane reads head, rows,
// then the strip's marks: a reader who tabs out of the pane's own field
// means to reach what they just filtered for, and the pane's marks come
// after everything the pane is for.
type PaneColumn struct {
	// Marks are the pane's own controls, bare at the strip's trailing
	// corner in reading order, with the measured [pane.MarkGapDp] between
	// them. The Toggle Sidebar button goes last and the arrangement puts it
	// there: it is not one of these.
	Marks []layout.Widget

	// Head is the slot under the strip — a field, a chooser — taking the
	// depth it reports. Nil takes none.
	Head layout.Widget

	// Rows is the collection the pane shows. It is the one slot that
	// stretches, so it runs from under Head to the footer or to the pane's
	// own foot.
	Rows layout.Widget

	// Footer is the slot under the rows: a hairline the pane's own width
	// and the slot under it. Nil takes no height at all, hairline
	// included.
	Footer layout.Widget

	// FooterDepth is how deep that slot lays out. Zero with a Footer set
	// takes the depth the slot reports.
	FooterDepth unit.Dp
}

// PaneColumnLayout is what one frame arranged inside the pane, in WINDOW
// coordinates.
type PaneColumnLayout struct {
	Strip, Head, Rows, Footer image.Rectangle
}

// PaneLayout is the arrangement one frame laid out, in WINDOW coordinates
// — what a window measures its composition against rather than recomputing
// it from the tokens that placed it.
//
// Only the content column has a toolbar and a footer. The pane runs to the
// window's top and bottom edges, so there is nothing above it and nothing
// below it.
type PaneLayout struct {
	// Pane is the pane's own rectangle, empty in every state where there is
	// no pane to draw — away above all, where the emptiness IS the
	// contract and the content beside it reflows to the window's own edge.
	Pane image.Rectangle

	// ContentX is the leading edge the content column starts from: the
	// pane's trailing edge, which is the one side it is not set in from, or
	// the window's own edge where there is no pane.
	ContentX int

	// RowTop and RowH are the top and height of the document row — what
	// stands between the toolbar and the footer — and FootTop the line that
	// row stops on, which is where the footer begins.
	RowTop, RowH, FootTop int

	// Toolbar, Main, Inspector and Footer are the four boxes of the content
	// area. An empty rectangle is a slot nothing stands in.
	Toolbar, Main, Inspector, Footer image.Rectangle

	// Column is what the pane's own column arranged.
	Column PaneColumnLayout

	// ToolbarFill and InspectorFill are the fills the toolbar stands on: a
	// toolbar carries no fill of its own (reference/macos/controls.md), so
	// what a control standing in it flattens its foreground onto is the
	// fill of whatever region lies under it, continued upward. The toolbar
	// runs across both columns and the fill change alone says where the
	// boundary is, so a control at the trailing end stands on the
	// inspector's fill and one at the leading end on the content column's.
	// [PaneLayout.ToolbarFillAt] is the reading.
	ToolbarFill, InspectorFill color.NRGBA
}

// ToolbarFillAt answers what a control standing at column x of the window
// flattens its foreground onto.
func (g PaneLayout) ToolbarFillAt(x int) color.NRGBA {
	if !g.Inspector.Empty() && x >= g.Inspector.Min.X {
		return g.InspectorFill
	}
	return g.ToolbarFill
}

// TwoColumn is the shell's two-column arrangement with the inspector as an
// optional slot: the window's own fill under everything, the pane one
// margin inside the leading, top and bottom edges, the content column flush
// against its trailing side with the toolbar across that column's top, and
// — where the window fills the slot — an inspector down the trailing edge
// behind a seam.
//
// An empty inspector takes no width.
type TwoColumn struct {
	// PaneWidth is the pane's own width, rim to rim. Zero takes
	// [PaneWidthDp].
	PaneWidth unit.Dp

	// PaneHidden sends the pane out of the window: it takes no width at all
	// and the content column reflows from the window's own leading edge.
	PaneHidden bool

	// Pane is the column standing inside the pane.
	Pane PaneColumn

	// PaneResize is the splitter on the pane's trailing edge. Nil is a pane
	// whose width is fixed.
	PaneResize *PaneResize

	// Surface is the window's own fill, painted under everything — what
	// shows in the margins around the pane. A window already standing on
	// its surface leaves it zero.
	Surface color.NRGBA

	// ContentFill is the content column's own fill, painted from the pane's
	// trailing edge to the window's and running its full height. It is also
	// what stands behind the two corners the pane rounds away from on its
	// flush side, so neither reads as a nick bitten out of the boundary.
	ContentFill color.NRGBA

	// InspectorFill is the inspector's own fill, painted the window's full
	// height so the column does not read as a block hanging off the
	// toolbar. The inspector is chrome, so the fill a window passes here is
	// the platform's chrome material.
	InspectorFill color.NRGBA

	// Toolbar is the strip across the content column's top.
	Toolbar Toolbar

	// Toggle is the one Toggle Sidebar button, drawn bare at the pane's
	// trailing corner while the pane stands and bordered at the toolbar's
	// leading end once it is away. Nil is a pane that cannot be sent away.
	Toggle *ToggleSidebar

	// Radius and Density resolve the bordered drawing of that button.
	Radius  tokens.RadiusScale
	Density tokens.Density

	// Main is the content column under the toolbar.
	Main layout.Widget

	// MainFirst records the main slot and replays it under the toolbar's
	// ops rather than drawing it after them, so the toolbar's ops still
	// stand ahead of the column's in the reading order while the column's
	// code has already run. It is what a toolbar carrying a control whose
	// state is the document's answer needs — a find whose count the
	// document settles.
	MainFirst bool

	// Footer is the slot under the content column, across its whole width,
	// at FooterDepth. Nil takes no height.
	Footer      layout.Widget
	FooterDepth unit.Dp

	// Inspector is the trailing column. Nil takes no width at all, the gap
	// before it included.
	Inspector layout.Widget

	// InspectorWidth is its absolute width, so a window resize leaves it
	// alone and the main column absorbs the change.
	InspectorWidth unit.Dp

	// InspectorGap is the air the main column keeps between its own edge
	// and the seam the inspector stands behind. The splitter's band is
	// wider than its line and reaches into both columns, so this is not the
	// band — it is what stops the main column's last pixel standing hard
	// against the seam.
	InspectorGap unit.Dp

	// InspectorResize is the splitter on the inspector's leading edge. Nil
	// is an inspector whose width is fixed.
	InspectorResize *InspectorResize

	// Over is laid out last of all, after the shadow, at the content
	// column's own box: the surface a control in the toolbar hangs over
	// what stands below it.
	Over layout.Widget

	// WindowButtonsEnd pins the trailing edge of the window's control
	// buttons instead of measuring it. The measurement is a live window's —
	// off a frame it reports zero, and on one it reports whatever the host
	// puts the buttons at — and neither is a number a stored image may
	// depend on, so a static render states one. A live window leaves it nil
	// and measures.
	WindowButtonsEnd func() unit.Dp

	// Memory keeps the pane's width, the inspector's width and the pane's
	// shown state across launches. A nil memory is a working one: the
	// arrangement is laid out and nothing is kept, which is what a frame
	// laid out for measurement wants.
	Memory *ArrangementMemory

	// Report hands the arrangement back once it is laid out.
	Report func(PaneLayout)
}

// Measure answers the arrangement at a window of the given size without
// laying anything out, so a window can assert where its columns stand
// without drawing them.
func (f TwoColumn) Measure(gtx layout.Context, size image.Point) PaneLayout {
	h := max(size.Y, 0)
	barH := min(max(gtx.Dp(f.Toolbar.depth()), 0), h)
	footH := 0
	if f.Footer != nil {
		footH = min(max(gtx.Dp(f.FooterDepth), 0), h-barH)
	}
	// The two strips are taken in the order they matter when there is no
	// room for either: the toolbar first, because it carries the button
	// that brings a dismissed pane back, and the footer out of what is
	// left, because a window with no height for a document has nothing to
	// report about one.
	g := PaneLayout{RowTop: barH, RowH: h - barH - footH, FootTop: h - footH}
	w := f.PaneWidth
	if w <= 0 {
		w = PaneWidthDp
	}
	g.Pane = pane.Bounds(gtx, size, w, f.PaneHidden)
	g.ContentX = ContentX(g.Pane)

	rowW := max(size.X-g.ContentX, 0)
	if rowW > 0 {
		if barH > 0 {
			g.Toolbar = toolbarBounds(g.ContentX, rowW, barH)
		}
		if footH > 0 {
			g.Footer = image.Rect(g.ContentX, g.FootTop, size.X, size.Y)
		}
	}
	insp, mainW, inspX := f.columns(gtx, size, g.ContentX)
	if g.RowH > 0 {
		g.Main = image.Rect(g.ContentX, g.RowTop, g.ContentX+mainW, g.RowTop+g.RowH)
		if insp > 0 {
			g.Inspector = image.Rect(inspX, g.RowTop, inspX+insp, g.RowTop+g.RowH)
		}
	}
	g.ToolbarFill, g.InspectorFill = f.ContentFill, f.InspectorFill
	return g
}

// columns measures the content area beside the pane: what the inspector
// takes, cut to what the window has for it; the main column taking the
// rest; and the boundary one gap trailing of it. An empty inspector takes
// no width and no gap.
func (f TwoColumn) columns(gtx layout.Context, size image.Point, contentX int) (inspector, main, inspectorX int) {
	if f.Inspector == nil {
		return 0, max(size.X-contentX, 0), size.X
	}
	gap := max(gtx.Dp(f.InspectorGap), 1)
	insp := gtx.Dp(f.InspectorWidth)
	if avail := size.X - contentX - gap; insp > avail {
		insp = avail
	}
	insp = max(insp, 0)
	main = max(size.X-contentX-gap-insp, 0)
	return insp, main, contentX + main + gap
}

// buttonsEnd is the trailing edge of the window's control buttons: the
// pinned value where one was given, and the live window's measurement
// otherwise.
func (f TwoColumn) buttonsEnd() unit.Dp {
	if f.WindowButtonsEnd != nil {
		return f.WindowButtonsEnd()
	}
	return desktop.LeadingInset()
}

// Layout draws the arrangement in the order it reads, which is the focus
// ring's too: the pane's column, the toolbar, the content column, the
// inspector, and last the footer, which holds nothing the ring can stop on.
//
// The shadow the pane casts is painted after every column has painted its
// own fill: it falls on what stands AROUND the pane, and a column painting
// its fill after the pane has laid out would cover it.
// patterns/pane cuts the pane's own box out of the drawing, so casting it
// late lands what casting it early landed. The boundaries come after even
// that, so neither the columns' rows nor the toolbar crossing over them can
// cover a line and a press this close to a boundary reaches the boundary.
func (f TwoColumn) Layout(gtx layout.Context, c tokens.PlatformColors) (layout.Dimensions, PaneLayout) {
	size := gtx.Constraints.Max
	g := f.Measure(gtx, size)

	// Both boundaries are taken hold of before anything is laid out, so the
	// whole frame is drawn where this frame's drag left them rather than
	// where the last one did. The pane's goes first: the inspector's bounds
	// are measured from where the pane ends, and a pane that moved in this
	// same frame has already moved that.
	if !g.Pane.Empty() && f.PaneResize != nil {
		f.PaneResize.State.Update(gtx, f.paneProps(gtx, c, size, g))
		// The arrangement is a value built fresh every frame, so a drag
		// this frame reports into the window's own state and back into
		// this value before anything is measured off it.
		if w := f.PaneResize.width(f.PaneWidth); w != f.PaneWidth {
			f.PaneWidth = w
			g = f.Measure(gtx, size)
		}
	}
	if f.Inspector != nil && f.InspectorResize != nil {
		f.InspectorResize.State.Update(gtx, f.inspectorProps(gtx, c, size, g))
		if w := f.InspectorResize.width(f.InspectorWidth); w != f.InspectorWidth {
			f.InspectorWidth = w
		}
	}

	// One press on the one button, wherever this frame is about to put it.
	f.Toggle.drain(gtx)

	// What stands under the columns: the window's own fill, the content
	// column's fill beside the pane, the two corners the pane rounds away
	// from on its flush side, and the pane itself with its rim and the
	// column standing in it.
	column := g.Column
	PaneFrame{
		Surface:     f.Surface,
		ContentFill: f.ContentFill,
		Sidebar: func(gtx layout.Context) layout.Dimensions {
			dims, cl := f.paneColumn(gtx, c, g.Pane.Min)
			column = cl
			return dims
		},
	}.Under(gtx, c, size, g.Pane)
	g.Column = column

	insp, mainW, inspX := f.columns(gtx, size, g.ContentX)

	// The inspector's own fill, painted before the toolbar and running the
	// window's full height, so the two chrome columns are the same shape —
	// one down each edge, with the document between them.
	if insp > 0 && f.InspectorFill.A > 0 {
		paint.FillShape(gtx.Ops, f.InspectorFill,
			clip.Rect(image.Rect(inspX, 0, size.X, size.Y)).Op())
	}

	var (
		mainCall op.CallOp
		recorded bool
	)
	if f.Main != nil && f.MainFirst && g.RowH > 0 && mainW > 0 {
		macro := op.Record(gtx.Ops)
		mgtx := gtx
		mgtx.Constraints = layout.Exact(image.Pt(mainW, g.RowH))
		f.Main(mgtx)
		mainCall = macro.Stop()
		recorded = true
	}

	if !g.Toolbar.Empty() {
		buttons, gutter := ToolbarLead(gtx, g.ContentX, f.Toolbar.Lead, f.buttonsEnd(), f.Toolbar.ButtonFallback)
		st := op.Offset(g.Toolbar.Min).Push(gtx.Ops)
		bgtx := gtx
		bgtx.Constraints = layout.Exact(g.Toolbar.Size())
		f.Toolbar.layout(bgtx, buttons, gutter, f.toolbarLead(c))
		st.Pop()
	}

	switch {
	case recorded:
		st := op.Offset(image.Pt(g.ContentX, g.RowTop)).Push(gtx.Ops)
		mainCall.Add(gtx.Ops)
		st.Pop()
	case f.Main != nil && g.RowH > 0 && mainW > 0:
		st := op.Offset(image.Pt(g.ContentX, g.RowTop)).Push(gtx.Ops)
		mgtx := gtx
		mgtx.Constraints = layout.Exact(image.Pt(mainW, g.RowH))
		f.Main(mgtx)
		st.Pop()
	}

	if f.Inspector != nil && insp > 0 && g.RowH > 0 {
		st := op.Offset(image.Pt(inspX, g.RowTop)).Push(gtx.Ops)
		agtx := gtx
		agtx.Constraints = layout.Exact(image.Pt(insp, g.RowH))
		f.Inspector(agtx)
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
	if f.Inspector != nil && insp > 0 && f.InspectorResize != nil {
		f.layoutInspectorSplitter(gtx, c, size, g)
	}

	if f.Over != nil && g.RowH > 0 {
		st := op.Offset(image.Pt(g.ContentX, 0)).Push(gtx.Ops)
		ogtx := gtx
		ogtx.Constraints = layout.Exact(image.Pt(max(size.X-g.ContentX, 0), size.Y))
		f.Over(ogtx)
		st.Pop()
	}

	// What this frame arranged, offered to the memory that keeps it — last,
	// so a boundary the hand moved earlier in this same frame is reported
	// at where it ended up. Every frame reports and the memory drops what
	// says nothing new, since a boundary can move in any frame and no frame
	// says whether one did.
	f.Memory.Record(Arrangement{
		PaneWidth:      f.PaneWidth,
		InspectorWidth: f.InspectorWidth,
		PaneShown:      !f.PaneHidden,
	})
	if f.Report != nil {
		f.Report(g)
	}
	return layout.Dimensions{Size: size}, g
}

// toolbarLead is what the arrangement puts in front of the toolbar's own
// leading group: the Toggle Sidebar button, bordered, and only while the
// pane is away.
func (f TwoColumn) toolbarLead(c tokens.PlatformColors) []layout.Widget {
	t := f.Toggle
	if t == nil || t.Click == nil || !f.PaneHidden {
		return nil
	}
	return []layout.Widget{func(gtx layout.Context) layout.Dimensions {
		return ToolbarControl(gtx, c, f.Radius, f.Density, t.Click, t.Mark, t.ToolbarTitle, false)
	}}
}

// paneColumn lays the pane's column out at the pane's full size and reports
// what it arranged in window coordinates, origin being the pane's own
// top-leading corner.
func (f TwoColumn) paneColumn(gtx layout.Context, c tokens.PlatformColors, origin image.Point) (layout.Dimensions, PaneColumnLayout) {
	t := f.Toggle
	if f.PaneHidden {
		t = nil
	}
	lead := desktop.BandLeadFrom(f.buttonsEnd(), pane.ButtonGapDp, f.Toolbar.ButtonFallback)
	dims, cl := f.Pane.Layout(gtx, c, lead, t)
	for _, r := range []*image.Rectangle{&cl.Strip, &cl.Head, &cl.Rows, &cl.Footer} {
		if !r.Empty() {
			*r = r.Add(origin)
		}
	}
	return dims, cl
}

// Layout draws the column at the size it is given and reports what it
// arranged, in the column's OWN coordinates — [TwoColumn] carries the
// result into the window's by the pane's origin.
//
// lead is where the strip's content may start, in from the window's leading
// edge: what [ToolbarLead] answers for the toolbar beside it, so the two
// halves of a window's Toggle Sidebar button stand in one window column.
// toggle is that button, drawn bare last of the strip's marks; nil is a
// pane that cannot be sent away, and the arrangement passes nil while the
// pane is already away.
//
// It is exported because the pane's column is a composition in its own
// right: a window renders a picture of its own pane alone, and a test
// drives one without a window around it.
func (p PaneColumn) Layout(gtx layout.Context, c tokens.PlatformColors, lead unit.Dp, toggle *ToggleSidebar) (layout.Dimensions, PaneColumnLayout) {
	size := gtx.Constraints.Max
	stripH := min(gtx.Dp(unit.Dp(pane.StripDp)), size.Y)

	var headH, rowsH, footH int
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(size.X, stripH)}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.Head == nil {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
			}
			dims := p.Head(gtx)
			headH = dims.Size.Y
			return dims
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if p.Rows == nil {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			dims := p.Rows(gtx)
			rowsH = dims.Size.Y
			return dims
		}),
	}
	if p.Footer != nil {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := p.footer(gtx, c)
			footH = dims.Size.Y
			return dims
		}))
	}
	layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)

	// The strip is drawn after the slots it was reserved ahead of, which is
	// a statement about focus order and not about paint.
	sgtx := gtx
	sgtx.Constraints = layout.Exact(image.Pt(size.X, stripH))
	p.strip(sgtx, c, lead, toggle)

	headTop := stripH
	rowsTop := min(headTop+headH, size.Y)
	rowsBot := min(rowsTop+rowsH, size.Y)
	cl := PaneColumnLayout{
		Strip: image.Rect(0, 0, size.X, stripH),
		Rows:  image.Rect(0, rowsTop, size.X, rowsBot),
	}
	if headH > 0 {
		cl.Head = image.Rect(0, headTop, size.X, rowsTop)
	}
	if footH > 0 {
		cl.Footer = image.Rect(0, max(size.Y-footH, 0), size.X, size.Y)
	}
	return layout.Dimensions{Size: size}, cl
}

// strip draws the pane's own top strip: the window's control buttons' run
// skipped at the leading end, a stretch that moves the window across the
// middle, and the pane's marks bare at its trailing corner with the Toggle
// Sidebar button last of them.
//
// The pane's own marks come first and the button last, because a pane's own
// Toggle Sidebar button is the pane talking about itself and nothing the
// pane is for may stand behind it.
func (p PaneColumn) strip(gtx layout.Context, c tokens.PlatformColors, lead unit.Dp, toggle *ToggleSidebar) layout.Dimensions {
	marks := p.Marks
	if t := toggle; t != nil && t.Click != nil {
		marks = append(append([]layout.Widget{}, marks...), func(gtx layout.Context) layout.Dimensions {
			return PaneMark(gtx, c, t.Click, t.Mark, t.PaneTitle)
		})
	}
	controls := make([]layout.Widget, 0, 2*len(marks))
	for i, w := range marks {
		if i > 0 {
			controls = append(controls, pane.DragSpacer(pane.MarkGapDp))
		}
		controls = append(controls, w)
	}
	return pane.Strip(gtx, lead, controls...)
}

// footer draws the pane's own footer: a hairline the pane's width, drawn
// inside its outline and running only the pane's width, and the slot under
// it.
func (p PaneColumn) footer(gtx layout.Context, c tokens.PlatformColors) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return FooterSeam(gtx, c, pane.Surface(c))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if d := p.FooterDepth; d > 0 {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(d)))
			}
			return p.Footer(gtx)
		}),
	)
}

// FooterSeam draws the one hairline a footer stands under: the seam's own
// width in the seam's own colour, flattened onto the fill it is drawn over,
// so neither footer in a window states which separator it is.
func FooterSeam(gtx layout.Context, c tokens.PlatformColors, standsOn color.NRGBA) layout.Dimensions {
	h := splitter.SeamWidth(gtx)
	w := gtx.Constraints.Max.X
	paint.FillShape(gtx.Ops, splitter.SeamColor(c, standsOn), clip.Rect(image.Rect(0, 0, w, h)).Op())
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// paneProps states the splitter on the pane's trailing edge. The pane
// stands one margin inside the window's leading edge, so its width and its
// trailing edge differ by that margin, which is what the boundary carries
// and the report takes back off.
func (f TwoColumn) paneProps(gtx layout.Context, c tokens.PlatformColors, size image.Point, g PaneLayout) splitter.Props {
	r := f.PaneResize
	seamPx := splitter.SeamWidth(gtx)
	marginPx := gtx.Dp(unit.Dp(pane.MarginDp))
	insp, mainW, _ := f.columns(gtx, size, g.ContentX)
	gap := 0
	if f.Inspector != nil {
		gap = max(gtx.Dp(f.InspectorGap), 1)
	}

	lo := gtx.Dp(r.Min)
	hi := max(min(gtx.Dp(r.Max), size.X-marginPx-gap-insp-floorOver(gtx, r.Floor, mainW)), lo)
	scale := PxPerDp(gtx)
	return splitter.Props{
		Axis:     layout.Horizontal,
		Boundary: float32(g.Pane.Max.X - seamPx),
		Min:      float32(marginPx + lo - seamPx),
		Max:      float32(marginPx + hi - seamPx),
		Colors:   c,
		// At rest the line IS the pane's rim, so it draws the rim's own
		// colour; under a hand it firms as every other boundary does.
		Rest:    pane.RimColor(c),
		Surface: pane.Surface(c),
		OnChange: func(at float32) {
			r.moved(Clamp{Min: r.Min, Max: r.Max}.clamp(unit.Dp((at + float32(seamPx-marginPx)) / scale)))
		},
	}
}

// layoutPaneSplitter draws that hand-hold along the rows the pane's
// trailing edge runs straight: the line IS the rim, so it stops where the
// rim curves away, which [pane.EdgeSpan] is the span of — a line carried
// through a rounded corner would leave a pixel of itself out on the
// window's own surface. The hand-hold runs the same rows as the line, since
// a boundary a reader may take hold of along part of the length it is drawn
// at reads as two boundaries, one live and one not.
func (f TwoColumn) layoutPaneSplitter(gtx layout.Context, c tokens.PlatformColors, size image.Point, g PaneLayout) {
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

// inspectorProps states the splitter on the inspector's leading edge. The
// bounds are the inspector's own, and the main column's floor over them.
func (f TwoColumn) inspectorProps(gtx layout.Context, c tokens.PlatformColors, size image.Point, g PaneLayout) splitter.Props {
	r := f.InspectorResize
	gap := max(gtx.Dp(f.InspectorGap), 1)
	_, mainW, inspX := f.columns(gtx, size, g.ContentX)

	lo := gtx.Dp(r.Min)
	hi := max(min(gtx.Dp(r.Max), size.X-g.ContentX-gap-floorOver(gtx, r.Floor, mainW)), lo)
	scale := PxPerDp(gtx)
	return splitter.Props{
		Axis:     layout.Horizontal,
		Boundary: float32(inspX),
		Min:      float32(size.X - hi),
		Max:      float32(size.X - lo),
		Colors:   c,
		HitSpan:  splitter.Span{Min: g.RowTop, Max: g.RowTop + g.RowH},
		OnChange: func(at float32) {
			r.moved(Clamp{Min: r.Min, Max: r.Max}.clamp(unit.Dp((float32(size.X) - at) / scale)))
		},
	}
}

// layoutInspectorSplitter draws the trailing boundary from the toolbar's
// lower edge to the window's foot. No line crosses a toolbar —
// [pane.SeamTop] is the row it stops on — so the toolbar reads as one
// across every column.
func (f TwoColumn) layoutInspectorSplitter(gtx layout.Context, c tokens.PlatformColors, size image.Point, g PaneLayout) {
	top := pane.SeamTop(gtx, image.Rectangle{Max: size})
	h := size.Y - top
	if h <= 0 {
		return
	}
	props := f.inspectorProps(gtx, c, size, g)
	// The hand-hold is stated in window rows and this splitter is laid out
	// from the toolbar's foot, so the span moves with the origin.
	props.HitSpan = splitter.Span{Min: g.RowTop - top, Max: g.RowTop + g.RowH - top}
	st := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
	sgtx := gtx
	sgtx.Constraints = layout.Exact(image.Pt(size.X, h))
	f.InspectorResize.State.Layout(sgtx, props)
	st.Pop()
}
