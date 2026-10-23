package shell_test

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/splitter"
	"github.com/vibrantgio/theme/tokens"
)

// twoColSize is the window every arrangement test here measures: wide
// enough for a pane, a document and an inspector beside each other.
var twoColSize = image.Pt(800, 400)

// fillNothing is a slot that takes everything it is given and paints
// nothing, so a measurement is of the arrangement and not of a payload.
func fillNothing(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// measureCtx is a context that lays nothing out: it measures an
// arrangement at one window size with one device pixel per dp.
func measureCtx(size image.Point) layout.Context {
	return layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(size),
		Ops:         new(op.Ops),
	}
}

// TestTwoColumnPaneAndColumns reads the arrangement's own geometry: the
// pane one margin inside the window's leading, top and bottom edges, the
// content column flush against its trailing side, and the document row
// running between the toolbar and the footer and no further.
func TestTwoColumnPaneAndColumns(t *testing.T) {
	gtx := measureCtx(twoColSize)
	f := shell.TwoColumn{
		PaneWidth:   240,
		Toolbar:     shell.Toolbar{Lead: 24},
		Main:        fillNothing,
		Footer:      fillNothing,
		FooterDepth: 20,
	}
	g := f.Measure(gtx, twoColSize)

	if got, want := g.Pane.Min, image.Pt(pane.MarginDp, pane.MarginDp); got != want {
		t.Errorf("the pane starts at %v, want one margin inside the window's leading and top edges, %v", got, want)
	}
	if got, want := g.Pane.Max.Y, twoColSize.Y-pane.MarginDp; got != want {
		t.Errorf("the pane ends at y=%d, want one margin above the window's foot, %d", got, want)
	}
	if got, want := g.Pane.Dx(), 240; got != want {
		t.Errorf("the pane is %d wide, want the width it was given, %d", got, want)
	}
	if got, want := g.ContentX, g.Pane.Max.X; got != want {
		t.Errorf("the content column starts at x=%d, want the pane's trailing edge %d — the one side the pane is not set in from", got, want)
	}
	if got, want := g.RowTop, int(shell.ToolbarDepth); got != want {
		t.Errorf("the document row starts at y=%d, want the platform's toolbar depth %d", got, want)
	}
	if got, want := g.FootTop, twoColSize.Y-20; got != want {
		t.Errorf("the footer starts at y=%d, want the window's foot less the footer's own depth, %d", got, want)
	}
	if got, want := g.RowH, g.FootTop-g.RowTop; got != want {
		t.Errorf("the document row is %d deep, want what stands between the toolbar and the footer, %d", got, want)
	}
	if got, want := g.Toolbar, image.Rect(g.ContentX, 0, twoColSize.X, int(shell.ToolbarDepth)); got != want {
		t.Errorf("the toolbar lays out at %v, want the content column's share of the window's top strip %v", got, want)
	}
}

// TestTwoColumnPaneAwayReflows reads the hidden contract: the pane takes no
// width at all and the content column reflows from the window's own edge.
func TestTwoColumnPaneAwayReflows(t *testing.T) {
	gtx := measureCtx(twoColSize)
	f := shell.TwoColumn{PaneWidth: 240, PaneHidden: true, Main: fillNothing}
	g := f.Measure(gtx, twoColSize)

	if !g.Pane.Empty() {
		t.Errorf("the pane measures %v with the pane away, want the empty rectangle that IS the contract", g.Pane)
	}
	if g.ContentX != 0 {
		t.Errorf("the content column starts at x=%d with the pane away, want the window's own leading edge", g.ContentX)
	}
}

// TestTwoColumnEmptyInspectorTakesNoWidth reads the slot's contract: an
// inspector nothing fills takes no width, and the gap before it goes with
// it.
func TestTwoColumnEmptyInspectorTakesNoWidth(t *testing.T) {
	gtx := measureCtx(twoColSize)
	base := shell.TwoColumn{PaneWidth: 240, Main: fillNothing, InspectorWidth: 320, InspectorGap: 6}

	empty := base.Measure(gtx, twoColSize)
	if !empty.Inspector.Empty() {
		t.Errorf("an unfilled inspector measures %v, want nothing at all", empty.Inspector)
	}
	if got, want := empty.Main.Max.X, twoColSize.X; got != want {
		t.Errorf("the main column ends at x=%d with no inspector, want the window's own edge %d", got, want)
	}

	base.Inspector = fillNothing
	filled := base.Measure(gtx, twoColSize)
	if got, want := filled.Inspector.Dx(), 320; got != want {
		t.Errorf("the inspector is %d wide, want the absolute width it was given, %d", got, want)
	}
	if got, want := filled.Inspector.Max.X, twoColSize.X; got != want {
		t.Errorf("the inspector ends at x=%d, want the window's own trailing edge %d", got, want)
	}
	if got, want := filled.Main.Max.X, filled.Inspector.Min.X-6; got != want {
		t.Errorf("the main column ends at x=%d, want one gap before the seam, %d", got, want)
	}
}

// TestToolbarLeadPassesWithoutAStep reads the corrected leading rule: the
// larger of the toolbar's own gutter and the inset past the window's
// control buttons, both as columns of the WINDOW. Read as a flag at either
// end the leading edge would step at the instant the pane reaches zero;
// read this way the two rests give the same answer.
func TestToolbarLeadPassesWithoutAStep(t *testing.T) {
	gtx := measureCtx(twoColSize)
	const gutter, buttonsEnd, fallback = 24, 80, 12
	past := buttonsEnd + int(pane.ButtonGapDp)

	// Pane standing: the content column starts past the buttons, so the
	// buttons claim nothing of the toolbar and the gutter is the whole of
	// the lead.
	buttons, run := shell.ToolbarLead(gtx, 248, gutter, buttonsEnd, fallback)
	if buttons != 0 {
		t.Errorf("the buttons claim %d px of a toolbar that starts past them, want none", buttons)
	}
	if run != gutter {
		t.Errorf("the toolbar leads from %d px with the pane standing, want its own gutter %d", run, gutter)
	}

	// Pane away: the toolbar starts at the window's leading edge, so the
	// buttons' run stands inside it and the lead is the larger of the two.
	buttons, run = shell.ToolbarLead(gtx, 0, gutter, buttonsEnd, fallback)
	if buttons != past {
		t.Errorf("the buttons claim %d px with the pane away, want their run and the air after it, %d", buttons, past)
	}
	if buttons+run != past {
		t.Errorf("the toolbar's content starts at %d px, want the larger of the gutter %d and the buttons' inset %d", buttons+run, gutter, past)
	}

	// Mid-travel: every column between the two rests answers the larger of
	// the two and never less than the gutter, so the leading edge moves
	// monotonically and never steps.
	prev := -1
	for x := 0; x <= 248; x++ {
		b, g := shell.ToolbarLead(gtx, x, gutter, buttonsEnd, fallback)
		if b+g < gutter {
			t.Fatalf("at contentX=%d the toolbar leads from %d px, under its own gutter %d", x, b+g, gutter)
		}
		if prev >= 0 && b+g > prev {
			t.Fatalf("at contentX=%d the toolbar's lead grew to %d px from %d: the leading edge stepped", x, b+g, prev)
		}
		prev = b + g
	}

	// With no buttons to clear the toolbar falls back to the inset it was
	// given, and its own gutter still wins where it is wider.
	if b, g := shell.ToolbarLead(gtx, 0, gutter, 0, fallback); b+g != gutter {
		t.Errorf("with no buttons the toolbar leads from %d px, want its own gutter %d", b+g, gutter)
	}
	if b, g := shell.ToolbarLead(gtx, 0, 4, 0, fallback); b+g != fallback {
		t.Errorf("with no buttons and a narrow gutter the toolbar leads from %d px, want the stated fallback %d", b+g, fallback)
	}
}

// markFill is the fill the toolbar probes paint, one per group, so a row of
// the rendered window reads back as runs.
var (
	leadFill  = color.NRGBA{R: 0xff, A: 0xff}
	titleFill = color.NRGBA{B: 0xff, A: 0xff}
	trailFill = color.NRGBA{G: 0x99, A: 0xff}
)

// probe paints a solid box of w by h in fill and reports it.
func probe(fill color.NRGBA, w, h int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, fill, clip.Rect(image.Rect(0, 0, w, h)).Op())
		return layout.Dimensions{Size: image.Pt(w, h)}
	}
}

// runsOf reads the row y of img back as the runs of fill it carries.
func runsOf(img *image.RGBA, y int, fill color.NRGBA) [][2]int {
	var runs [][2]int
	start := -1
	for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
		c := img.RGBAAt(x, y)
		on := c.R == fill.R && c.G == fill.G && c.B == fill.B
		switch {
		case on && start < 0:
			start = x
		case !on && start >= 0:
			runs = append(runs, [2]int{start, x})
			start = -1
		}
	}
	if start >= 0 {
		runs = append(runs, [2]int{start, img.Bounds().Max.X})
	}
	return runs
}

// TestToolbarMetricsArePlatformReadings reads the four measurements off a
// rendered toolbar: the leading gutter the window states, the measured gap
// between two merely adjacent controls, the measured gap between the last
// of them and a bare title, and the trailing gutter the last control keeps
// from the window's own edge.
func TestToolbarMetricsArePlatformReadings(t *testing.T) {
	const box = 20
	size := image.Pt(500, 120)
	f := shell.TwoColumn{
		PaneHidden:  true,
		ContentFill: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		Toolbar: shell.Toolbar{
			Lead:     24,
			Leading:  []layout.Widget{probe(leadFill, box, box), probe(leadFill, box, box)},
			Center:   probe(titleFill, 40, box),
			Trailing: []layout.Widget{probe(trailFill, box, box)},
		},
		WindowButtonsEnd: func() unit.Dp { return 0 },
	}
	img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		dims, _ := f.Layout(gtx, tokens.PlatformLight)
		return dims
	})
	y := int(shell.ToolbarDepth) / 2

	lead := runsOf(img, y, leadFill)
	if len(lead) != 2 {
		t.Fatalf("the leading group reads back as %d runs, want the two controls it holds", len(lead))
	}
	if got, want := lead[0][0], 24; got != want {
		t.Errorf("the leading group starts at x=%d, want the gutter the window stated, %d", got, want)
	}
	if got, want := lead[1][0]-lead[0][1], int(shell.PlatformToolbar.ControlGap); got != want {
		t.Errorf("two adjacent controls stand %d apart, want the measured ControlGap %d", got, want)
	}

	title := runsOf(img, y, titleFill)
	if len(title) != 1 {
		t.Fatalf("the centre reads back as %d runs, want one", len(title))
	}
	if got, want := title[0][0]-lead[1][1], int(shell.PlatformToolbar.TitleGap); got != want {
		t.Errorf("the bare title stands %d from the control before it, want the measured TitleGap %d", got, want)
	}

	// The toolbar centres what it holds: its depth IS the strip the window
	// buttons stand on, and the platform puts them on its middle, so a
	// control centred here stands on their line without being told to.
	if top, bot := columnRun(img, lead[0][0]+1, trailFill, leadFill); top >= 0 {
		if want := (int(shell.ToolbarDepth) - (bot - top)) / 2; top != want {
			t.Errorf("a control stands from row %d, want it centred in the toolbar at row %d", top, want)
		}
	}
	if top, bot := columnRun(img, title[0][0]+1, trailFill, titleFill); top >= 0 {
		if want := (int(shell.ToolbarDepth) - (bot - top)) / 2; top != want {
			t.Errorf("the centre stands from row %d, want it centred in the toolbar at row %d", top, want)
		}
	}

	trail := runsOf(img, y, trailFill)
	if len(trail) != 1 {
		t.Fatalf("the trailing group reads back as %d runs, want one", len(trail))
	}
	if got, want := size.X-trail[0][1], int(shell.PlatformToolbar.Trail); got != want {
		t.Errorf("the last control stands %d from the window's trailing edge, want the measured Trail %d", got, want)
	}
}

// columnRun reads the rows of column x that carry fill.
func columnRun(img *image.RGBA, x int, _ color.NRGBA, fill color.NRGBA) (top, bottom int) {
	top, bottom = -1, -1
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		c := img.RGBAAt(x, y)
		if c.R == fill.R && c.G == fill.G && c.B == fill.B {
			if top < 0 {
				top = y
			}
			bottom = y + 1
		}
	}
	return top, bottom
}

// TestToolbarPairStandsCloser reads the second measurement: two controls
// that belong together stand at the PairGap, which is closer than two
// merely adjacent.
func TestToolbarPairStandsCloser(t *testing.T) {
	const box = 20
	size := image.Pt(400, 120)
	f := shell.TwoColumn{
		PaneHidden:  true,
		ContentFill: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		Toolbar: shell.Toolbar{
			Lead:    24,
			Leading: []layout.Widget{shell.ToolbarPair(probe(leadFill, box, box), probe(leadFill, box, box))},
		},
		WindowButtonsEnd: func() unit.Dp { return 0 },
	}
	img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		dims, _ := f.Layout(gtx, tokens.PlatformLight)
		return dims
	})
	runs := runsOf(img, int(shell.ToolbarDepth)/2, leadFill)
	if len(runs) != 2 {
		t.Fatalf("the pair reads back as %d runs, want two", len(runs))
	}
	if got, want := runs[1][0]-runs[0][1], int(shell.PlatformToolbar.PairGap); got != want {
		t.Errorf("two controls that belong together stand %d apart, want the measured PairGap %d", got, want)
	}
	if shell.PlatformToolbar.PairGap >= shell.PlatformToolbar.ControlGap {
		t.Errorf("the PairGap %v is not closer than the ControlGap %v", shell.PlatformToolbar.PairGap, shell.PlatformToolbar.ControlGap)
	}
}

// TestToggleSidebarIsOneGestureSource presses the button in both of its
// places through one clickable: bare at the pane's trailing corner while
// the pane stands, bordered at the toolbar's leading end once it is away.
// Only one of the two is ever on screen, so a second source would be a
// second source for one affordance.
func TestToggleSidebarIsOneGestureSource(t *testing.T) {
	size := image.Pt(400, 200)
	presses, hidden := 0, false
	toggle := &shell.ToggleSidebar{
		Click:        new(widget.Clickable),
		Mark:         icons.Mark(icons.SidebarLeft),
		PaneTitle:    "Hide the sidebar",
		ToolbarTitle: "Show the sidebar",
		OnPress:      func(layout.Context) { presses++; hidden = !hidden },
	}
	frame := func() shell.TwoColumn {
		return shell.TwoColumn{
			PaneWidth:        240,
			PaneHidden:       hidden,
			Toggle:           toggle,
			Radius:           tokens.Radius,
			Density:          tokens.Comfortable,
			Toolbar:          shell.Toolbar{Lead: 24},
			WindowButtonsEnd: func() unit.Dp { return 0 },
		}
	}
	w := func(gtx layout.Context) layout.Dimensions {
		dims, _ := frame().Layout(gtx, tokens.PlatformLight)
		return dims
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveFrame(w, ops, r, size)
	driveFrame(w, ops, r, size)

	// The pane's own corner: one margin in from its trailing edge, centred
	// on the strip the window buttons stand in.
	g := frame().Measure(measureCtx(size), size)
	corner := f32.Pt(float32(g.Pane.Max.X-pane.MarginDp-shell.MarkDp/2),
		float32(g.Pane.Min.Y+int(pane.StripDp)/2))
	clickAt(r, w, ops, size, corner)
	if presses != 1 {
		t.Fatalf("a press on the pane's own corner answered %d times, want once", presses)
	}
	if !hidden {
		t.Fatal("the pane did not go away under its own button")
	}

	// The same clickable now stands bordered at the toolbar's leading end,
	// and the press reaches it there.
	driveFrame(w, ops, r, size)
	clickAt(r, w, ops, size, f32.Pt(24+shell.MarkDp/2, float32(int(shell.ToolbarDepth)/2)))
	if presses != 2 {
		t.Errorf("a press at the toolbar's leading end answered %d presses in all, want two", presses)
	}
	if hidden {
		t.Error("the pane did not come back under the toolbar's half of its button")
	}
}

// clickAt queues one press and release at p and drives the frame that
// answers them.
func clickAt(r *gioinput.Router, w layout.Widget, ops *op.Ops, size image.Point, p f32.Point) {
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: p, Source: pointer.Touch},
		pointer.Event{Kind: pointer.Release, Position: p, Source: pointer.Touch},
	)
	driveFrame(w, ops, r, size)
}

// TestPaneResizeReportsTheWidthItWasDraggedTo drags the pane's trailing
// boundary and reads the width back: the boundary carries the pane's
// trailing edge and the report takes the margin back off, so what is kept
// is the pane's own width.
func TestPaneResizeReportsTheWidthItWasDraggedTo(t *testing.T) {
	const start = 240
	size := image.Pt(800, 400)
	width := unit.Dp(start)
	st := new(splitter.State)
	build := func() shell.TwoColumn {
		return shell.TwoColumn{
			PaneWidth: width,
			Toolbar:   shell.Toolbar{Lead: 24},
			Main:      fillNothing,
			PaneResize: &shell.PaneResize{
				State: st, Min: 160, Max: 480, Floor: 200,
				OnChange: func(w unit.Dp) { width = w },
			},
			WindowButtonsEnd: func() unit.Dp { return 0 },
		}
	}
	w := func(gtx layout.Context) layout.Dimensions {
		dims, _ := build().Layout(gtx, tokens.PlatformLight)
		return dims
	}
	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveFrame(w, ops, r, size)
	driveFrame(w, ops, r, size)

	edge := float32(pane.MarginDp + start - splitter.SeamDp)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: f32.Pt(edge, 200), Source: pointer.Touch},
		pointer.Event{Kind: pointer.Move, Position: f32.Pt(edge+40, 200), Source: pointer.Touch},
		pointer.Event{Kind: pointer.Release, Position: f32.Pt(edge+40, 200), Source: pointer.Touch},
	)
	driveFrame(w, ops, r, size)

	if got, want := width, unit.Dp(start+40); got != want {
		t.Errorf("the pane was dragged to %v, want %v", got, want)
	}
}

// TestArrangementMemoryKeepsWidthsAndTheShownState reads the memory the
// shell keeps for a window: the defaults where nothing was kept, the
// clamp on a file a text editor could have written, one write per burst,
// and the shown state kept at all — a reader who sends the pane away and
// finds it back on the next launch is reading a defect.
func TestArrangementMemoryKeepsWidthsAndTheShownState(t *testing.T) {
	def := shell.Arrangement{PaneWidth: 240, InspectorWidth: 320, PaneShown: true}
	limits := shell.Limits{Pane: shell.Clamp{Min: 160, Max: 480}, Inspector: shell.Clamp{Min: 160, Max: 640}}
	path := filepath.Join(t.TempDir(), "layout.json")

	m := shell.OpenArrangement(path, time.Millisecond, def, limits)
	if got := m.Arrangement(); got != def {
		t.Errorf("a window with nothing kept opens on %+v, want the defaults %+v", got, def)
	}

	// The opening arrangement is not news, and neither is one equal to it.
	m.Record(def)
	m.Record(def)
	m.Stop()
	if _, err := os.Stat(path); err == nil {
		t.Error("the arrangement a window opened with was written back; want nothing written until something moves")
	}

	moved := shell.Arrangement{PaneWidth: 300, InspectorWidth: 320, PaneShown: false}
	m.Record(moved)
	m.Stop()

	back := shell.OpenArrangement(path, time.Millisecond, def, limits)
	if got := back.Arrangement(); got != moved {
		t.Errorf("the next launch opens on %+v, want what the reader left, %+v", got, moved)
	}

	// A width a text editor could have put there is clamped on read: a
	// window the reader cannot use is not worth obeying a file for.
	if err := os.WriteFile(path, []byte(`{"pane":9000,"inspector":1,"paneShown":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	clamped := shell.OpenArrangement(path, time.Millisecond, def, limits).Arrangement()
	if clamped.PaneWidth != limits.Pane.Max || clamped.InspectorWidth != limits.Inspector.Min {
		t.Errorf("a file outside the window's bounds opened on %+v, want it clamped to %+v", clamped, limits)
	}
}

// TestTwoColumnGolden records the arrangement in both appearances: the pane
// with its column, the toolbar over the content, the inspector behind its
// seam and the footer under the column.
func TestTwoColumnGolden(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.PlatformColors
	}{
		{"light-two-column-inspector", tokens.PlatformLight},
		{"dark-two-column-inspector", tokens.PlatformDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.colors
			f := shell.TwoColumn{
				PaneWidth:     200,
				Surface:       c.WindowBackground,
				ContentFill:   c.TextBackground,
				InspectorFill: pane.Surface(c),
				Pane: shell.PaneColumn{
					Header: probe(c.SystemBlue, 184, 24),
					Rows:   fillRect(pane.Surface(c)),
				},
				Toolbar: shell.Toolbar{
					Lead:     24,
					Leading:  []layout.Widget{probe(c.SystemIndigo, 20, 20)},
					Trailing: []layout.Widget{probe(c.SystemTeal, 20, 20)},
				},
				Main:            fillRect(c.TextBackground),
				Inspector:       fillRect(pane.Surface(c)),
				InspectorWidth:  160,
				InspectorGap:    6,
				Footer:          probe(c.SystemGreen, 120, 12),
				FooterDepth:     22,
				Radius:          tokens.Radius,
				Density:         tokens.Comfortable,
				InspectorResize: &shell.InspectorResize{Min: 120, Max: 240, Floor: 120},
				PaneResize:      &shell.PaneResize{Min: 160, Max: 320, Floor: 200},
			}
			golden.Render(t, tc.name, twoColSize, func(gtx layout.Context) layout.Dimensions {
				dims, _ := f.Layout(gtx, c)
				return dims
			})
		})
	}
}

// TestPaneWindowKeepsTheCallersOptions reads the window treatment: the
// full-size-content options first and the caller's own after them, so a
// window states its title and size and nothing about the treatment.
func TestPaneWindowKeepsTheCallersOptions(t *testing.T) {
	mine := []app.Option{app.Title("Vault"), app.Size(1100, 800)}
	opts := shell.PaneWindow(mine...)
	if len(opts) < len(mine) {
		t.Fatalf("the treatment answered %d options, want at least the caller's %d", len(opts), len(mine))
	}
	if got, want := len(opts)-len(mine), len(desktop.FullSizeContent()); got != want {
		t.Errorf("the treatment added %d options of its own, want the full-size-content treatment's %d", got, want)
	}
}

// TestChromeTopClearsTheDeeperStrip reads what an overlay must clear: the
// strip the screen lays out itself, and the native strip where that is
// deeper — either would put a toast on top of a live control.
func TestChromeTopClearsTheDeeperStrip(t *testing.T) {
	// Off a window the platform reports no strip of its own, so the
	// screen's own depth is the whole answer and a screen laying out under
	// the native strip clears nothing.
	if got, want := shell.ChromeTop(shell.ToolbarDepth), unit.Dp(shell.ToolbarDepth); got < want {
		t.Errorf("an overlay over a screen carrying a toolbar clears %v, want at least the toolbar's own depth %v", got, want)
	}
	if got := shell.ChromeTop(0); got < desktopTopInset() {
		t.Errorf("an overlay over a screen carrying no toolbar clears %v, want the native strip %v", got, desktopTopInset())
	}
}

func desktopTopInset() unit.Dp { return desktop.TopInset() }
