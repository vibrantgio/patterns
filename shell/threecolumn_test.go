package shell_test

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/splitter"
	"github.com/vibrantgio/theme/tokens"
)

// threeColSize is the window every three-column test here measures: wide
// enough for a pane, a list column at Mail's measured width and a content
// column over its floor beside them.
var threeColSize = image.Pt(1200, 500)

// TestThreeColumnColumnsAndSeam reads the arrangement's own geometry: the
// pane one margin inside the window's leading, top and bottom edges, the
// list column flush against its trailing side at the width it was given,
// one seam's width between the list and the content, and the content
// column absorbing everything after it.
func TestThreeColumnColumnsAndSeam(t *testing.T) {
	gtx := measureCtx(threeColSize)
	f := shell.ThreeColumn{
		PaneWidth: 240,
		ListWidth: 320,
		List:      shell.ListColumn{Rows: fillNothing},
		Toolbar:   shell.Toolbar{Lead: 24},
		Main:      fillNothing,
	}
	g := f.Measure(gtx, threeColSize)

	if got, want := g.Pane.Min, image.Pt(pane.MarginDp, pane.MarginDp); got != want {
		t.Errorf("the pane starts at %v, want one margin inside the window's leading and top edges, %v", got, want)
	}
	if got, want := g.ListX, g.Pane.Max.X; got != want {
		t.Errorf("the list column starts at x=%d, want the pane's trailing edge %d — the one side the pane is not set in from", got, want)
	}
	if got, want := g.List.Dx(), 320; got != want {
		t.Errorf("the list column is %d wide, want the absolute width it was given, %d", got, want)
	}
	if got, want := g.ContentX-g.List.Max.X, splitter.SeamDp; got != want {
		t.Errorf("the content column starts %d after the list ends, want one seam, %d", got, want)
	}
	if got, want := g.Main.Max.X, threeColSize.X; got != want {
		t.Errorf("the content column ends at x=%d, want the window's own trailing edge %d — it absorbs the rest", got, want)
	}
	if got, want := g.RowTop, int(shell.ToolbarDepth); got != want {
		t.Errorf("both columns start at y=%d, want the platform's toolbar depth %d", got, want)
	}
	// The list column runs to the window's foot: the content column's
	// status bar is that column's and stops at the seam.
	if got, want := g.List.Max.Y, threeColSize.Y; got != want {
		t.Errorf("the list column ends at y=%d, want the window's own foot %d", got, want)
	}

	// One toolbar, two shares, parted where the columns are.
	if got, want := g.ListToolbar, image.Rect(g.ListX, 0, g.List.Max.X, int(shell.ToolbarDepth)); got != want {
		t.Errorf("the toolbar's share over the list lays out at %v, want the list column's own span %v", got, want)
	}
	if got, want := g.Toolbar, image.Rect(g.ContentX, 0, threeColSize.X, int(shell.ToolbarDepth)); got != want {
		t.Errorf("the toolbar's share over the content lays out at %v, want the content column's own span %v", got, want)
	}
	if got, want := g.ToolbarFillAt(g.ListX+1), g.ListFill; got != want {
		t.Errorf("a control over the list flattens onto %v, want the list column's fill %v", got, want)
	}
	if got, want := g.ToolbarFillAt(g.ContentX+1), g.ContentFill; got != want {
		t.Errorf("a control over the content flattens onto %v, want the content column's fill %v", got, want)
	}
}

// TestThreeColumnListWidthIsAbsolute reads the measured contract: the list
// column keeps its width across a window resize and the content column
// absorbs the change, and the list gives way only where the window has no
// room left for the content column's own floor. Mail holds its list at 447
// from a window 1400 wide to one 1800 wide and squeezes it at 1000 to
// leave the content its 360.
func TestThreeColumnListWidthIsAbsolute(t *testing.T) {
	f := shell.ThreeColumn{
		PaneWidth:    240,
		ListWidth:    320,
		ContentFloor: 360,
		List:         shell.ListColumn{Rows: fillNothing},
		Main:         fillNothing,
	}
	wide := image.Pt(1600, 500)
	narrow := image.Pt(1200, 500)
	g1 := f.Measure(measureCtx(wide), wide)
	g2 := f.Measure(measureCtx(narrow), narrow)
	if g1.List.Dx() != 320 || g2.List.Dx() != 320 {
		t.Errorf("the list column measures %d at a window %d wide and %d at one %d wide, want its own %d at both",
			g1.List.Dx(), wide.X, g2.List.Dx(), narrow.X, 320)
	}
	if got, want := g1.Main.Dx()-g2.Main.Dx(), wide.X-narrow.X; got != want {
		t.Errorf("the content column grew by %d over %d of window, want every pixel of it", got, want)
	}

	// Squeezed: the window is too narrow for the pane, the list and the
	// content's floor together, so the list gives way and the content
	// keeps its floor. Mail's own reading — the list down to 307 at a
	// window 1000 wide, the content still 360.
	tight := image.Pt(900, 500)
	g3 := f.Measure(measureCtx(tight), tight)
	if got, want := g3.Main.Dx(), 360; got != want {
		t.Errorf("the content column measures %d in a window too narrow for every column, want its floor %d", got, want)
	}
	if g3.List.Dx() >= 320 {
		t.Errorf("the list column kept %d in a window with no room for it, want it squeezed under its own %d", g3.List.Dx(), 320)
	}

	// And it hands the width back as soon as the window has room again,
	// because what is kept is the width and not what the squeeze left.
	if got := f.Measure(measureCtx(wide), wide).List.Dx(); got != 320 {
		t.Errorf("the list column measures %d once the window is wide again, want the %d it kept", got, 320)
	}
}

// TestThreeColumnListStopsAtItsMinimum reads the other end of the
// measurement: a window too narrow even for the content's floor squeezes
// the list no further than its own minimum, which is where Mail stops and
// starts taking the room out of the pane instead.
func TestThreeColumnListStopsAtItsMinimum(t *testing.T) {
	f := shell.ThreeColumn{
		PaneWidth: 240,
		List:      shell.ListColumn{Rows: fillNothing},
		Main:      fillNothing,
	}
	size := image.Pt(500, 400)
	g := f.Measure(measureCtx(size), size)
	if got, want := g.List.Dx(), int(shell.PlatformList.Min); got != want {
		t.Errorf("the list column measures %d in a window with no room at all, want the platform's minimum %d", got, want)
	}
}

// TestPlatformListIsTheMailReading pins the three measurements to the
// capture they were read off, so a change to one is a change a reader has
// to argue with the picture about.
func TestPlatformListIsTheMailReading(t *testing.T) {
	want := shell.ListMetrics{Width: 447, Min: 200, ContentFloor: 360}
	if shell.PlatformList != want {
		t.Errorf("the platform's list reading is %+v, want what Mail measured, %+v", shell.PlatformList, want)
	}
	if shell.PlatformList.Width < 220 || shell.PlatformList.Width > 480 {
		t.Errorf("the measured default %v stands outside the stated 220 to 480; the measurement is what ships, and the disagreement belongs on record",
			shell.PlatformList.Width)
	}
}

// TestThreeColumnPaneAwayReflows reads the hidden contract: the pane takes
// no width at all and the list column reflows from the window's own
// leading edge.
func TestThreeColumnPaneAwayReflows(t *testing.T) {
	gtx := measureCtx(threeColSize)
	f := shell.ThreeColumn{PaneWidth: 240, PaneHidden: true, ListWidth: 320,
		List: shell.ListColumn{Rows: fillNothing}, Main: fillNothing}
	g := f.Measure(gtx, threeColSize)
	if !g.Pane.Empty() {
		t.Errorf("the pane measures %v with the pane away, want the empty rectangle that IS the contract", g.Pane)
	}
	if g.ListX != 0 {
		t.Errorf("the list column starts at x=%d with the pane away, want the window's own leading edge", g.ListX)
	}
}

// TestListColumnSlots reads the column's two slots: the rows starting at
// the column's own top, since the column has no header row for them to
// start under, and a footer that takes no height at all when nothing
// fills it.
func TestListColumnSlots(t *testing.T) {
	gtx := measureCtx(threeColSize)
	f := shell.ThreeColumn{
		PaneWidth: 240,
		ListWidth: 320,
		List:      shell.ListColumn{Rows: fillNothing},
		Main:      fillNothing,
	}
	var g shell.ThreeColumnLayout
	f.Report = func(l shell.ThreeColumnLayout) { g = l }
	gtx.Ops = new(op.Ops)
	f.Layout(gtx, tokens.PlatformLight)

	if got, want := g.ListColumn.Rows.Min, image.Pt(g.ListX, g.RowTop); got != want {
		t.Errorf("the list's rows start at %v, want the column's own top-leading corner in window coordinates, %v", got, want)
	}
	if !g.ListColumn.Footer.Empty() {
		t.Errorf("an unfilled footer measures %v, want no height at all", g.ListColumn.Footer)
	}
}

// TestListResizeReportsTheWidthItWasDraggedTo drags the list column's
// trailing boundary and reads the width back: the boundary IS the seam's
// leading edge, so what is reported is the column's own width.
func TestListResizeReportsTheWidthItWasDraggedTo(t *testing.T) {
	const start = 320
	size := image.Pt(1200, 500)
	width := unit.Dp(start)
	st := new(splitter.State)
	build := func() shell.ThreeColumn {
		return shell.ThreeColumn{
			PaneWidth: 240,
			ListWidth: width,
			List:      shell.ListColumn{Rows: fillNothing},
			Toolbar:   shell.Toolbar{Lead: 24},
			Main:      fillNothing,
			ListResize: &shell.ListResize{
				State: st, Min: 200,
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

	edge := float32(pane.MarginDp + 240 + start)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: f32.Pt(edge, 300), Source: pointer.Touch},
		pointer.Event{Kind: pointer.Move, Position: f32.Pt(edge-60, 300), Source: pointer.Touch},
		pointer.Event{Kind: pointer.Release, Position: f32.Pt(edge-60, 300), Source: pointer.Touch},
	)
	driveFrame(w, ops, r, size)

	if got, want := width, unit.Dp(start-60); got != want {
		t.Errorf("the list column was dragged to %v, want %v", got, want)
	}
}

// TestListSeamRunsTheWindowsFullHeight reads the seam off a rendered
// window: MEASURED off Mail at x=779 the seam carries the platform's
// measured seam colour at every row from the window's top edge to its
// foot, the toolbar included, and the toolbar's two shares are what it
// parts up there.
func TestListSeamRunsTheWindowsFullHeight(t *testing.T) {
	size := image.Pt(900, 400)
	c := tokens.PlatformLight
	f := shell.ThreeColumn{
		PaneWidth:        200,
		ListWidth:        240,
		Surface:          c.WindowBackground,
		ContentFill:      c.TextBackground,
		List:             shell.ListColumn{Rows: fillRect(c.TextBackground)},
		Main:             fillRect(c.TextBackground),
		ListResize:       &shell.ListResize{Min: 200},
		WindowButtonsEnd: func() unit.Dp { return 0 },
	}
	g := f.Measure(measureCtx(size), size)
	img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		dims, _ := f.Layout(gtx, c)
		return dims
	})

	seam := splitter.SeamColor(c)
	x := g.ContentX - 1
	for _, y := range []int{0, int(shell.ToolbarDepth) / 2, int(shell.ToolbarDepth), size.Y / 2, size.Y - 1} {
		got := img.RGBAAt(x, y)
		if got.R != seam.R || got.G != seam.G || got.B != seam.B {
			t.Errorf("the seam at row %d reads %v, want the platform's measured seam %v — it runs the window's whole height", y, got, seam)
		}
	}
	// And it is one pixel: the columns on either side of it are the
	// columns' own fill.
	for _, at := range []int{x - 1, x + 1} {
		got := img.RGBAAt(at, size.Y/2)
		if got.R != c.TextBackground.R || got.G != c.TextBackground.G || got.B != c.TextBackground.B {
			t.Errorf("column %d reads %v, want the column's own fill %v — the seam is one pixel", at, got, c.TextBackground)
		}
	}
}

// TestArrangementKeepsTheListWidth reads what the memory keeps for a
// three-column window: the list's width beside the pane's, clamped on read
// the way every other kept width is, and written as a key only by a window
// that has a list at all.
func TestArrangementKeepsTheListWidth(t *testing.T) {
	def := shell.Arrangement{PaneWidth: 240, ListWidth: 447, PaneShown: true}
	limits := shell.Limits{
		Pane: shell.Clamp{Min: 160, Max: 480},
		List: shell.Clamp{Min: 200},
	}
	path := filepath.Join(t.TempDir(), "layout.json")

	m := shell.OpenArrangement(path, time.Millisecond, def, limits)
	if got := m.Arrangement(); got != def {
		t.Errorf("a window with nothing kept opens on %+v, want the defaults %+v", got, def)
	}
	moved := shell.Arrangement{PaneWidth: 240, ListWidth: 300, PaneShown: true}
	m.Record(def)
	m.Record(moved)
	m.Stop()
	if got := shell.OpenArrangement(path, time.Millisecond, def, limits).Arrangement(); got != moved {
		t.Errorf("the next launch opens on %+v, want what the reader left, %+v", got, moved)
	}

	// The list's clamp states a minimum and no ceiling, which is the
	// platform's reading, and the minimum still bites.
	if err := os.WriteFile(path, []byte(`{"pane":240,"list":1,"paneShown":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := shell.OpenArrangement(path, time.Millisecond, def, limits).Arrangement().ListWidth; got != 200 {
		t.Errorf("a file under the list's minimum opened on %v, want it clamped to %v", got, unit.Dp(200))
	}

	// A two-column window writes no list key: a width of zero is a column
	// the window does not have.
	twoCol := filepath.Join(t.TempDir(), "layout.json")
	tm := shell.OpenArrangement(twoCol, time.Millisecond,
		shell.Arrangement{PaneWidth: 240, PaneShown: true}, shell.Limits{Pane: shell.Clamp{Min: 160, Max: 480}})
	tm.Record(shell.Arrangement{PaneWidth: 240, PaneShown: true})
	tm.Record(shell.Arrangement{PaneWidth: 300, PaneShown: true})
	tm.Stop()
	data, err := os.ReadFile(twoCol)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"list", "inspector"} {
		if containsKey(string(data), key) {
			t.Errorf("a two-column window's file says %q; want it to say nothing about a column it does not have:\n%s", key, data)
		}
	}
}

// containsKey reports whether the written arrangement carries a key.
func containsKey(s, key string) bool {
	q := `"` + key + `"`
	for i := 0; i+len(q) <= len(s); i++ {
		if s[i:i+len(q)] == q {
			return true
		}
	}
	return false
}

// TestThreeColumnGolden records the arrangement in both appearances: the
// pane with its column, the toolbar's two shares over the list and the
// content, the seam the window's full height, and the list column's own
// three slots — a header, the rows and a footer — beside the content
// column's own footer.
func TestThreeColumnGolden(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.PlatformColors
	}{
		{"light-three-column", tokens.PlatformLight},
		{"dark-three-column", tokens.PlatformDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.colors
			f := shell.ThreeColumn{
				PaneWidth:   200,
				ListWidth:   240,
				Surface:     c.WindowBackground,
				ContentFill: c.TextBackground,
				Pane: shell.PaneColumn{
					Header: probe(c.SystemBlue, 184, 24),
					Rows:   fillRect(pane.Surface(c)),
				},
				List: shell.ListColumn{
					Rows:        fillRect(c.TextBackground),
					Footer:      probe(c.SystemOrange, 100, 12),
					FooterDepth: 22,
				},
				ListToolbar: shell.Toolbar{
					Lead:     20,
					Center:   probe(c.SystemPurple, 56, 14),
					Trailing: []layout.Widget{probe(c.SystemPink, 20, 20)},
				},
				Toolbar: shell.Toolbar{
					Lead:     16,
					Leading:  []layout.Widget{probe(c.SystemIndigo, 20, 20)},
					Trailing: []layout.Widget{probe(c.SystemTeal, 20, 20)},
				},
				Main:        fillRect(c.TextBackground),
				Footer:      probe(c.SystemGreen, 120, 12),
				FooterDepth: 22,
				Radius:      tokens.Radius,
				Density:     tokens.Comfortable,
				PaneResize:  &shell.PaneResize{Min: 160, Max: 320, Floor: 200},
				ListResize:  &shell.ListResize{Min: 200},
			}
			golden.Render(t, tc.name, image.Pt(900, 400), func(gtx layout.Context) layout.Dimensions {
				dims, _ := f.Layout(gtx, c)
				return dims
			})
		})
	}
}
