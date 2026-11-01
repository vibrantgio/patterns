package findbar_test

import (
	"image"
	"image/color"
	"sync"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/patterns/findbar"
	"github.com/vibrantgio/theme/tokens"
)

// The test window: a sidebar of sidebarW columns, and the column the bar
// searches in the rest, at one pixel per dp.
var windowSize = image.Pt(800, 240)

const sidebarW = 220

var schemes = []struct {
	name string
	c    tokens.PlatformColors
}{
	{"light", tokens.PlatformLight},
	{"dark", tokens.PlatformDark},
}

// columnFill is the fill of the column the bar searches: the platform's
// content plane, the fill a document column stands on.
func columnFill(c tokens.PlatformColors) color.NRGBA { return c.ControlBackground }

// content is the column's content: a focusable area with its own tag, which
// the bar gives the keyboard back to.
type content struct{ tag *int }

func (k content) layout(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, k.tag)
	for {
		if _, ok := gtx.Event(key.FocusFilter{Target: k.tag}); !ok {
			break
		}
	}
	return layout.Dimensions{Size: size}
}

// window lays out the sidebar in its own fill and the column beside it, with
// the bar over the column only.
func window(c tokens.PlatformColors, bar, column layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		paint.FillShape(gtx.Ops, c.SidebarMaterial, clip.Rect{Max: image.Pt(sidebarW, size.Y)}.Op())
		paint.FillShape(gtx.Ops, columnFill(c), clip.Rect{Min: image.Pt(sidebarW, 0), Max: size}.Op())
		defer op.Offset(image.Pt(sidebarW, 0)).Push(gtx.Ops).Pop()
		gtx.Constraints = layout.Exact(image.Pt(size.X-sidebarW, size.Y))
		return findbar.Over(bar, column)(gtx)
	}
}

// liveBar is a live bar in a window under test: the state it is given, the
// latest layout.Widget it emitted, and what its callbacks saw.
type liveBar struct {
	t       *testing.T
	send    rx.Observer[findbar.State]
	state   findbar.State
	content content
	field   event.Tag
	r       *gioinput.Router
	ops     *op.Ops

	mu     sync.Mutex
	latest layout.Widget
	gen    int

	closes   int
	changes  []string
	nexts    int
	prevs    int
	countAsk int
}

func newLiveBar(t *testing.T) *liveBar {
	t.Helper()
	b := &liveBar{t: t, content: content{tag: new(int)}, r: new(gioinput.Router), ops: new(op.Ops)}
	send, stateObs := rx.Subject[findbar.State](0, 1)
	b.send = send
	th := rx.Of(golden.StaticTheme(tokens.PlatformLight))
	obs := findbar.FindBar(th, findbar.Props{
		Open:       stateObs,
		Fill:       columnFill,
		Content:    func() event.Tag { return b.content.tag },
		OnClose:    func(layout.Context) { b.closes++ },
		OnChange:   func(_ layout.Context, q string) { b.changes = append(b.changes, q) },
		OnNext:     func(layout.Context) { b.nexts++ },
		OnPrevious: func(layout.Context) { b.prevs++ },
		Count:      func() string { b.countAsk++; return "8" },
		FocusTag:   func(tag event.Tag) { b.field = tag },
	})
	sub := obs.Subscribe(rx.GoroutineContext(), func(w layout.Widget, err error, done bool) {
		if !done && err == nil && w != nil {
			b.mu.Lock()
			b.latest = w
			b.gen++
			b.mu.Unlock()
		}
	})
	t.Cleanup(sub.Unsubscribe)
	b.set(findbar.State{})
	return b
}

// set hands the bar a state, waits for the bar to emit for it, and lays out
// two frames.
func (b *liveBar) set(st findbar.State) {
	b.t.Helper()
	b.mu.Lock()
	before := b.gen
	b.mu.Unlock()
	b.state = st
	b.send.Next(st)
	deadline := time.Now().Add(2 * time.Second)
	for {
		b.mu.Lock()
		gen := b.gen
		b.mu.Unlock()
		if gen > before {
			break
		}
		if time.Now().After(deadline) {
			b.t.Fatal("the bar did not emit for the state")
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	b.frame()
	b.frame()
}

func (b *liveBar) frame() {
	b.mu.Lock()
	w := b.latest
	b.mu.Unlock()
	b.ops.Reset()
	window(tokens.PlatformLight, w, b.content.layout)(layout.Context{
		Constraints: layout.Exact(windowSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         b.ops,
		Source:      b.r.Source(),
	})
	b.r.Frame(b.ops)
}

func (b *liveBar) open(query string) {
	b.t.Helper()
	b.set(findbar.State{Open: true, Opens: b.state.Opens + 1, Query: query})
}

// closeIfAsked closes the bar as a caller does when the bar asks to close.
func (b *liveBar) closeIfAsked() {
	b.t.Helper()
	if b.closes > 0 {
		b.closes = 0
		b.set(findbar.State{Opens: b.state.Opens})
	}
}

func (b *liveBar) inField() bool   { return b.field != nil && b.r.Source().Focused(b.field) }
func (b *liveBar) inContent() bool { return b.r.Source().Focused(b.content.tag) }

func (b *liveBar) press(name key.Name, mods key.Modifiers) {
	b.r.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	b.frame()
}

func (b *liveBar) click(at f32.Point) {
	b.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse},
	)
	b.frame()
}

// doneAt is the middle of Done: 48 by 20, 9 px in from the column's trailing
// edge, 6 rows down in the bar.
var doneAt = f32.Pt(float32(windowSize.X-9-24), 6+10)

// The bar opens with the keyboard in its field. A second request with the
// bar open gives the keyboard back to the field.
func TestTheBarOpensWithTheKeyboardInItsField(t *testing.T) {
	b := newLiveBar(t)
	if b.inField() {
		t.Fatal("the keyboard is in the field while the bar is closed")
	}
	b.open("")
	if !b.inField() {
		t.Fatal("the bar opened without the keyboard in its field")
	}
	// Move the keyboard to the content. An emission with no new request
	// leaves it there, and a new request gives it to the field.
	b.r.Source().Execute(key.FocusCmd{Tag: b.content.tag})
	b.frame()
	b.set(findbar.State{Open: true, Opens: b.state.Opens})
	b.frame()
	if b.inField() {
		t.Fatal("an emission with no new request gave the keyboard to the field")
	}
	b.open("")
	if !b.inField() {
		t.Error("a second request with the bar open did not give the keyboard to the field")
	}
}

// The bar can open holding a given query: the field holds it and reports
// it, the count shows while the field holds the keyboard, and the previous
// and next control is switched on.
func TestTheBarOpensHoldingAGivenQuery(t *testing.T) {
	b := newLiveBar(t)
	b.open("Gio")
	if n := len(b.changes); n == 0 || b.changes[n-1] != "Gio" {
		t.Fatalf("the bar opened with the query and reported %q, want \"Gio\" last", b.changes)
	}
	if !b.inField() {
		t.Fatal("the bar opened with a query without the keyboard in its field")
	}
	asked := b.countAsk
	b.frame()
	if b.countAsk == asked {
		t.Error("the field holds the keyboard and a query, and the bar did not ask for the count")
	}
	// An empty query on a later request keeps the field's text.
	b.press(key.NameEscape, 0)
	b.closeIfAsked()
	before := len(b.changes)
	b.open("")
	if got := b.changes[before:]; len(got) != 0 {
		t.Errorf("a request with no query changed the field's text: %q", got)
	}
}

// The bar opens holding the query selected whole, with the caret at its
// end, as a platform field selects its text when it takes the keyboard.
func TestTheBarOpensHoldingTheQuerySelectedWhole(t *testing.T) {
	b := newLiveBar(t)
	b.open("Gio")
	ed, ok := b.field.(*widget.Editor)
	if !ok {
		t.Fatalf("the field's focus tag is %T, want its editor", b.field)
	}
	if got := ed.SelectedText(); got != "Gio" {
		t.Errorf("the bar opened with %q selected, want the whole query \"Gio\"", got)
	}
	if caret, _ := ed.Selection(); caret != 3 {
		t.Errorf("the caret stands at %d, want 3, the end of the query", caret)
	}
}

// The count shows only while the field holds the keyboard: with the
// keyboard in the content, the bar does not ask for it.
func TestTheCountShowsOnlyWhileTheFieldHoldsTheKeyboard(t *testing.T) {
	b := newLiveBar(t)
	b.open("Gio")
	b.r.Source().Execute(key.FocusCmd{Tag: b.content.tag})
	b.frame()
	b.frame()
	if !b.inContent() {
		t.Fatal("the keyboard did not move to the content")
	}
	asked := b.countAsk
	b.frame()
	if b.countAsk != asked {
		t.Errorf("the keyboard is in the content and the bar asked for the count %d times", b.countAsk-asked)
	}
}

// Escape closes the bar and gives the keyboard back to the content, text
// typed or none, and with the keyboard in the content too.
func TestEscapeClosesTheBarAndGivesTheKeyboardBack(t *testing.T) {
	b := newLiveBar(t)
	for _, tc := range []struct {
		name  string
		query string
		where func()
	}{
		{"an empty field", "", func() {}},
		{"text typed", "Gio", func() {}},
		{"the keyboard in the content", "Gio", func() {
			b.r.Source().Execute(key.FocusCmd{Tag: b.content.tag})
			b.frame()
		}},
	} {
		b.open(tc.query)
		tc.where()
		b.press(key.NameEscape, 0)
		if b.closes != 1 {
			t.Fatalf("Escape with %s asked the bar to close %d times, want 1", tc.name, b.closes)
		}
		b.frame()
		if !b.inContent() {
			t.Errorf("after Escape with %s the keyboard is not in the content", tc.name)
		}
		b.closeIfAsked()
	}
	// With the bar closed, Escape is not the bar's.
	b.press(key.NameEscape, 0)
	if b.closes != 0 {
		t.Errorf("Escape with the bar closed asked it to close %d times", b.closes)
	}
}

// Done closes the bar and gives the keyboard back to the content.
func TestDoneClosesTheBarAndGivesTheKeyboardBack(t *testing.T) {
	b := newLiveBar(t)
	b.open("Gio")
	b.click(doneAt)
	if b.closes != 1 {
		t.Fatalf("Done asked the bar to close %d times, want 1", b.closes)
	}
	b.frame()
	if !b.inContent() {
		t.Error("after Done the keyboard is not in the content")
	}
}

// The caller closing the bar gives the keyboard back to the content when the
// field holds it.
func TestTheCallerClosingTheBarGivesTheKeyboardBack(t *testing.T) {
	b := newLiveBar(t)
	b.open("Gio")
	b.set(findbar.State{Opens: b.state.Opens})
	if !b.inContent() {
		t.Error("the caller closed the bar with the keyboard in its field, and the keyboard is not in the content")
	}
}

// Enter and Shift+Enter in the field step to the next and the previous match.
func TestEnterStepsThroughTheMatches(t *testing.T) {
	b := newLiveBar(t)
	b.open("Gio")
	b.press(key.NameReturn, 0)
	b.press(key.NameReturn, key.ModShift)
	if b.nexts != 1 || b.prevs != 1 {
		t.Errorf("Enter and Shift+Enter stepped next %d and previous %d times, want 1 and 1", b.nexts, b.prevs)
	}
}

// frameAt draws the window in c with the bar in the state field, or with no
// bar when field is nil.
func frameAt(t *testing.T, c tokens.PlatformColors, field *input.RenderState) *image.RGBA {
	t.Helper()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	bar := func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }
	if field != nil {
		bar = findbar.Render(shaper, c, tokens.Spacing, tokens.Radius, tokens.DefaultTypography, tokens.Comfortable, columnFill(c), *field)
	}
	column := func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Max} }
	return golden.Capture(t, windowSize, window(c, bar, column))
}

func pixelAt(img *image.RGBA, x, y int) color.NRGBA {
	p := img.RGBAAt(x, y)
	return color.NRGBA{R: p.R, G: p.G, B: p.B, A: p.A}
}

func near(a, b color.NRGBA, tol int) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}

// runs reads the row y of img from x0 to x1 inclusive as alternating runs of
// columns on fill and off it, starting with the first column's kind.
func runs(img *image.RGBA, fill color.NRGBA, y, x0, x1 int) []int {
	var out []int
	prev, n := false, 0
	for x := x0; x <= x1; x++ {
		off := !near(pixelAt(img, x, y), fill, 2)
		if x > x0 && off != prev {
			out = append(out, n)
			n = 0
		}
		prev = off
		n++
	}
	return append(out, n)
}

// offRows reads the column x of img from y0 to y1 inclusive and returns the
// first and last row off fill.
func offRows(img *image.RGBA, fill color.NRGBA, x, y0, y1 int) (first, last int) {
	first, last = -1, -1
	for y := y0; y <= y1; y++ {
		if !near(pixelAt(img, x, y), fill, 2) {
			if first < 0 {
				first = y
			}
			last = y
		}
	}
	return first, last
}

// The bar stands over the column it searches and over no other, in that
// column's fill with the seam under it, at the reference's rows, read at the
// bar's middle row as in textedit-find-unfocused-{light,dark}.png: 31 rows of
// fill and a 1 px seam; 7 clear columns, the field, 7, the previous and next
// control 33 wide, 8, Done 48 wide, 9. The field is 22 rows with 5 above it,
// the buttons 20 rows with 6 above them.
func TestTheBarStandsOverTheColumnItSearchesOnly(t *testing.T) {
	w := windowSize.X
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			closed := frameAt(t, tc.c, nil)
			img := frameAt(t, tc.c, &input.RenderState{Text: "Gio"})
			fill := columnFill(tc.c)

			for y := 0; y < windowSize.Y; y++ {
				for x := 0; x < sidebarW; x++ {
					if a, b := pixelAt(img, x, y), pixelAt(closed, x, y); a != b {
						t.Fatalf("the sidebar at (%d,%d) is %v with the bar open, %v closed", x, y, a, b)
					}
				}
			}
			for y := 0; y < findbar.FillDp; y++ {
				for _, x := range []int{sidebarW, sidebarW + 6, w - 9, w - 1} {
					if got := pixelAt(img, x, y); got != fill {
						t.Errorf("the bar at (%d,%d) is %v, want the column's fill %v", x, y, got, fill)
					}
				}
			}
			seam := findbar.Seam(tc.c, fill)
			for x := sidebarW; x < w; x++ {
				if got := pixelAt(img, x, findbar.FillDp); got != seam {
					t.Fatalf("the seam at x=%d is %v, want the separator over the fill %v", x, got, seam)
				}
			}
			if got := pixelAt(img, sidebarW-1, findbar.FillDp); got != tc.c.SidebarMaterial {
				t.Errorf("the seam reaches the sidebar: (%d,%d) is %v", sidebarW-1, findbar.FillDp, got)
			}

			mid := 15
			got := runs(img, fill, mid, sidebarW, w-1)
			if len(got) < 6 || got[0] != 7 {
				t.Fatalf("the bar's middle row reads %v, want 7 clear columns before the field", got)
			}
			tail := got[len(got)-5:]
			want := []int{7, 33, 8, 48, 9}
			for i := range want {
				if tail[i] != want[i] {
					t.Fatalf("the bar's trailing runs read %v, want %v after the field's edge", tail, want)
				}
			}
			fieldStart := sidebarW + 7
			fieldEnd := w - 1 - 9 - 48 - 8 - 33 - 7
			if f, l := offRows(img, fill, fieldStart+60, 0, findbar.FillDp-1); f != 5 || l != 26 {
				t.Errorf("the field covers rows %d–%d of the bar, want 5–26", f, l)
			}
			if f, l := offRows(img, fill, w-1-9-24-20, 0, findbar.FillDp-1); f != 6 || l != 25 {
				t.Errorf("Done covers rows %d–%d of the bar, want 6–25", f, l)
			}
			if f, l := offRows(img, fill, fieldEnd+1+7+3, 0, findbar.FillDp-1); f != 6 || l != 25 {
				t.Errorf("the previous and next control covers rows %d–%d of the bar, want 6–25", f, l)
			}
		})
	}
}

// The seam is the measured one: Mail's hairline under its bar is #e6e6e6
// light and #343434 dark over the column's #ffffff and #1e1e1e.
func TestTheSeamIsMailsHairline(t *testing.T) {
	for _, tc := range []struct {
		c    tokens.PlatformColors
		want color.NRGBA
	}{
		{tokens.PlatformLight, color.NRGBA{0xe6, 0xe6, 0xe6, 0xff}},
		{tokens.PlatformDark, color.NRGBA{0x34, 0x34, 0x34, 0xff}},
	} {
		if got := findbar.Seam(tc.c, columnFill(tc.c)); got != tc.want {
			t.Errorf("the seam over %v is %v, want %v", columnFill(tc.c), got, tc.want)
		}
	}
}

// The goldens: the bar over a column beside a sidebar, the field holding a
// query with the keyboard in it and its count, in both schemes.
func TestFindBarGolden(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := frameAt(t, tc.c, &input.RenderState{Text: "Gio", Count: "8", Focused: true})
			golden.Compare(t, tc.name+"-findbar", img)
		})
	}
}
