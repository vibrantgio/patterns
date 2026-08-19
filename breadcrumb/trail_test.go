package breadcrumb_test

import (
	"context"
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/breadcrumb"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// The trail every test here starts from: a real path, in document order, so
// the trailing segment is the current location. Keys are the paths the
// segments navigate to; labels are what the row draws.
const (
	homeKey   = "/"
	designKey = "/design"
	tokensKey = "/design/tokens"
	assetsKey = "/assets"
)

// newTrail builds the pre-resolved trail under test with the deterministic
// shaper and the default light tokens.
func newTrail(t *testing.T) breadcrumb.TrailLayout {
	t.Helper()
	return breadcrumb.NewTrail(defaultShaper(t), tokens.DefaultLight, tokens.Spacing, tokens.DefaultTypography.TitleSmall)
}

// segments builds a trail from key/label pairs, giving every segment but the
// last an OnClick that records its key — the conventional trail, where the
// current location is inert.
func segments(fired map[string]int, pairs ...[2]string) []breadcrumb.Segment {
	segs := make([]breadcrumb.Segment, len(pairs))
	for i, p := range pairs {
		segs[i] = breadcrumb.Segment{Key: p[0], Label: p[1]}
		if i < len(pairs)-1 {
			key := p[0]
			segs[i].OnClick = func(_ layout.Context) { fired[key]++ }
		}
	}
	return segs
}

// driveTrail lays the segments out against ops + router for one frame, the
// way a window would, and returns the row's Dimensions.
func driveTrail(w breadcrumb.TrailLayout, ops *op.Ops, r *gioinput.Router, size image.Point, segs []breadcrumb.Segment) layout.Dimensions {
	ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(size),
		Ops:         ops,
		Source:      r.Source(),
	}
	dims := w(gtx, segs)
	r.Frame(ops)
	return dims
}

// clickAt queues a press and a release at x on the row's mid-height, which is
// what a pointer click is to a widget.Clickable.
func clickAt(r *gioinput.Router, x int) {
	hit := f32.Pt(float32(x), float32(canvasH)/2)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: hit, Source: pointer.Touch},
		pointer.Event{Kind: pointer.Release, Position: hit, Source: pointer.Touch},
	)
}

// rowWidth measures the natural width of a trail of labels through the static
// Render path, which draws the same row without keeping any state. Loose
// constraints are what make the answer the row's own width rather than the
// canvas's.
func rowWidth(t *testing.T, shaper *text.Shaper, labels ...string) int {
	t.Helper()
	items := make([]breadcrumb.Item, len(labels))
	for i, l := range labels {
		items[i] = breadcrumb.Item{Label: l}
	}
	w := breadcrumb.Render(shaper, breadcrumb.Props{Items: items, Shaper: shaper},
		tokens.DefaultLight, tokens.Spacing, tokens.DefaultTypography.TitleSmall)
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(1<<14, 1<<14)},
		Ops:         new(op.Ops),
	}
	return w(gtx).Size.X
}

// centreOf returns the x of the middle of segment i in a row of these labels,
// derived from measured widths rather than from restated spacing constants:
// the separator's width is whatever two adjacent labels cost beyond the two
// labels alone.
func centreOf(t *testing.T, shaper *text.Shaper, i int, labels ...string) int {
	t.Helper()
	sep := rowWidth(t, shaper, labels[0], labels[0]) - 2*rowWidth(t, shaper, labels[0])
	x := 0
	for j := 0; j < i; j++ {
		x += rowWidth(t, shaper, labels[j]) + sep
	}
	return x + rowWidth(t, shaper, labels[i])/2
}

// TestTrailClickRoutesToItsSegment is the base case: a trail that does not
// change between the frame that draws it and the frame that reports the
// click. Each segment answers for itself, and the current location — which
// carries no OnClick — answers for nobody.
func TestTrailClickRoutesToItsSegment(t *testing.T) {
	shaper := defaultShaper(t)
	labels := []string{"Home", "Design", "Tokens"}

	for _, tc := range []struct {
		name string
		idx  int
		want map[string]int
	}{
		{"ancestor", 0, map[string]int{homeKey: 1}},
		{"parent", 1, map[string]int{designKey: 1}},
		{"current location", 2, map[string]int{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fired := map[string]int{}
			segs := segments(fired,
				[2]string{homeKey, labels[0]},
				[2]string{designKey, labels[1]},
				[2]string{tokensKey, labels[2]},
			)
			w := breadcrumb.NewTrail(shaper, tokens.DefaultLight, tokens.Spacing, tokens.DefaultTypography.TitleSmall)

			r := new(gioinput.Router)
			ops := new(op.Ops)
			driveTrail(w, ops, r, canvasSize, segs)

			clickAt(r, centreOf(t, shaper, tc.idx, labels...))
			driveTrail(w, ops, r, canvasSize, segs)

			if len(fired) != len(tc.want) {
				t.Fatalf("fired=%v, want %v", fired, tc.want)
			}
			for k, n := range tc.want {
				if fired[k] != n {
					t.Errorf("fired[%q]=%d, want %d (all: %v)", k, fired[k], n, fired)
				}
			}
		})
	}
}

// TestTrailClickRoutesAfterReshuffle is the case the Key exists for. The
// click is made on the second segment of one trail and reported on a frame
// that draws a different trail, in which some other place stands where the
// clicked one stood. Routing by position would navigate to that other place;
// routing by key navigates where the user clicked.
func TestTrailClickRoutesAfterReshuffle(t *testing.T) {
	shaper := defaultShaper(t)
	before := []string{"Home", "Design", "Tokens"}
	fired := map[string]int{}

	first := segments(fired,
		[2]string{homeKey, before[0]},
		[2]string{designKey, before[1]},
		[2]string{tokensKey, before[2]},
	)
	// The same three places, reshuffled: Assets now holds the position
	// Design held, and Design has moved to the end.
	second := segments(fired,
		[2]string{homeKey, "Home"},
		[2]string{assetsKey, "Assets"},
		[2]string{designKey, "Design"},
	)

	w := newTrail(t)
	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveTrail(w, ops, r, canvasSize, first)

	clickAt(r, centreOf(t, shaper, 1, before...))
	driveTrail(w, ops, r, canvasSize, second)

	if fired[designKey] != 1 {
		t.Errorf("click on Design fired %d time(s) for %q, want 1 (all: %v)", fired[designKey], designKey, fired)
	}
	if fired[assetsKey] != 0 {
		t.Errorf("Assets took over the clicked position and fired %d time(s); the click was on Design", fired[assetsKey])
	}
	if fired[homeKey] != 0 {
		t.Errorf("Home fired %d time(s); the click was on Design", fired[homeKey])
	}
}

// TestTrailClickSurvivesSegmentLeavingTrail covers the other half of a trail
// that changed: the clicked segment is not in the new trail at all. It was on
// screen when it was pressed, so its click is reported rather than dropped —
// and it is reported once, not again on the frames after.
func TestTrailClickSurvivesSegmentLeavingTrail(t *testing.T) {
	shaper := defaultShaper(t)
	before := []string{"Home", "Design", "Tokens"}
	fired := map[string]int{}

	first := segments(fired,
		[2]string{homeKey, before[0]},
		[2]string{designKey, before[1]},
		[2]string{tokensKey, before[2]},
	)
	// A sibling branch: Design is gone from the trail entirely.
	second := segments(fired,
		[2]string{homeKey, "Home"},
		[2]string{assetsKey, "Assets"},
	)

	w := newTrail(t)
	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveTrail(w, ops, r, canvasSize, first)

	clickAt(r, centreOf(t, shaper, 1, before...))
	driveTrail(w, ops, r, canvasSize, second)
	if fired[designKey] != 1 {
		t.Fatalf("Design fired %d time(s) after leaving the trail, want 1 (all: %v)", fired[designKey], fired)
	}

	driveTrail(w, ops, r, canvasSize, second)
	driveTrail(w, ops, r, canvasSize, second)
	if fired[designKey] != 1 {
		t.Errorf("Design fired %d time(s) over three frames, want 1", fired[designKey])
	}
	if len(fired) != 1 {
		t.Errorf("fired=%v, want only %q", fired, designKey)
	}
}

// inserted is the reshuffled trail the two tests below both wait on: the same
// places, with Assets inserted where Design stood, so Design is still a
// clickable ancestor but one position further along.
func inserted(fired map[string]int) []breadcrumb.Segment {
	return segments(fired,
		[2]string{homeKey, "Home"},
		[2]string{assetsKey, "Assets"},
		[2]string{designKey, "Design"},
		[2]string{tokensKey, "Tokens"},
	)
}

// TestTrailKeyboardFocusFollowsTheSegment is the sharper half of the same
// question, and the one positional state cannot answer. The keyboard is
// tabbed onto Design, the trail is reshuffled under that focus, and Enter is
// pressed. Focus belongs to a place, not to a slot, so Enter goes where the
// focus was left — to Design, wherever Design now stands — and not to
// whichever place has since moved into Design's old position.
func TestTrailKeyboardFocusFollowsTheSegment(t *testing.T) {
	fired := map[string]int{}
	first := segments(fired,
		[2]string{homeKey, "Home"},
		[2]string{designKey, "Design"},
		[2]string{tokensKey, "Tokens"},
	)
	second := inserted(fired)

	w := newTrail(t)
	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveTrail(w, ops, r, canvasSize, first)

	// Tab to the second focus stop. Only clickable segments take one, so the
	// stops are Home then Design; the current location has none.
	r.MoveFocus(key.FocusForward)
	driveTrail(w, ops, r, canvasSize, first)
	r.MoveFocus(key.FocusForward)
	driveTrail(w, ops, r, canvasSize, first)

	// The trail is reshuffled under the focus, then Enter is pressed.
	// widget.Clickable requires a matched Press and Release to register.
	driveTrail(w, ops, r, canvasSize, second)
	r.Queue(
		key.Event{Name: key.NameReturn, State: key.Press},
		key.Event{Name: key.NameReturn, State: key.Release},
	)
	driveTrail(w, ops, r, canvasSize, second)

	if fired[designKey] != 1 {
		t.Errorf("Enter on the focused segment fired Design %d time(s), want 1 (all: %v)", fired[designKey], fired)
	}
	if fired[assetsKey] != 0 {
		t.Errorf("Enter fired Assets %d time(s); Assets inherited Design's position, not its focus", fired[assetsKey])
	}
}

// TestTrailHeldPressDoesNotNavigateSomewhereElse is the pointer's version of
// the same rule. A press is held on Design while the trail reshuffles under
// it, and the release lands where Assets now is. Design was not released on
// and Assets was not pressed on, so nothing navigates — rather than Assets
// inheriting a press meant for Design.
func TestTrailHeldPressDoesNotNavigateSomewhereElse(t *testing.T) {
	shaper := defaultShaper(t)
	before := []string{"Home", "Design", "Tokens"}
	fired := map[string]int{}

	first := segments(fired,
		[2]string{homeKey, before[0]},
		[2]string{designKey, before[1]},
		[2]string{tokensKey, before[2]},
	)
	second := inserted(fired)

	w := newTrail(t)
	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveTrail(w, ops, r, canvasSize, first)

	hit := f32.Pt(float32(centreOf(t, shaper, 1, before...)), float32(canvasH)/2)
	r.Queue(pointer.Event{Kind: pointer.Press, Position: hit, Source: pointer.Touch})
	driveTrail(w, ops, r, canvasSize, second)
	r.Queue(pointer.Event{Kind: pointer.Release, Position: hit, Source: pointer.Touch})
	driveTrail(w, ops, r, canvasSize, second)
	driveTrail(w, ops, r, canvasSize, second)

	if len(fired) != 0 {
		t.Errorf("a press held across a reshuffle navigated: %v; want nothing", fired)
	}
}

// TestTrailGrowsAndShrinks walks a trail down into a tree and back up,
// clicking a segment at each depth, to show that state minted and retired
// between frames stays addressable while it is on screen.
func TestTrailGrowsAndShrinks(t *testing.T) {
	shaper := defaultShaper(t)
	fired := map[string]int{}

	deep := []string{"Home", "Design", "Tokens"}
	shallow := []string{"Home", "Design"}

	w := newTrail(t)
	r := new(gioinput.Router)
	ops := new(op.Ops)

	deepSegs := segments(fired,
		[2]string{homeKey, deep[0]},
		[2]string{designKey, deep[1]},
		[2]string{tokensKey, deep[2]},
	)
	shallowSegs := segments(fired,
		[2]string{homeKey, shallow[0]},
		[2]string{designKey, shallow[1]},
	)

	driveTrail(w, ops, r, canvasSize, shallowSegs)
	clickAt(r, centreOf(t, shaper, 0, shallow...))
	driveTrail(w, ops, r, canvasSize, deepSegs)
	clickAt(r, centreOf(t, shaper, 1, deep...))
	driveTrail(w, ops, r, canvasSize, shallowSegs)

	if fired[homeKey] != 1 || fired[designKey] != 1 {
		t.Errorf("fired=%v, want one click each on %q and %q", fired, homeKey, designKey)
	}
	if fired[tokensKey] != 0 {
		t.Errorf("the current location fired %d time(s), want 0", fired[tokensKey])
	}
}

// TestTrailLiveRoutesAcrossTokenChange drives the theme-fed path and changes
// a token between the click and the frame that reports it. Every emission
// shares the one interaction state, so the click still lands.
func TestTrailLiveRoutesAcrossTokenChange(t *testing.T) {
	shaper := defaultShaper(t)
	labels := []string{"Home", "Design", "Tokens"}
	fired := map[string]int{}
	segs := segments(fired,
		[2]string{homeKey, labels[0]},
		[2]string{designKey, labels[1]},
		[2]string{tokensKey, labels[2]},
	)

	// A theme whose colours go light then dark: the stream emits one layout
	// per token change, and the trail is drawn through the first and reported
	// through the second.
	th := theme.Default()
	th.Color = rx.From(tokens.DefaultLight, tokens.DefaultDark)

	var emitted []breadcrumb.TrailLayout
	if err := breadcrumb.Trail(rx.Of(th), breadcrumb.TrailProps{Shaper: shaper}).
		Subscribe(context.Background(), func(next breadcrumb.TrailLayout, _ error, done bool) {
			if !done && next != nil {
				emitted = append(emitted, next)
			}
		}).Wait(); err != nil {
		t.Fatalf("Trail subscribe: %v", err)
	}
	if len(emitted) < 2 {
		t.Fatalf("a colour change emitted %d layout(s), want one per change", len(emitted))
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveTrail(emitted[0], ops, r, canvasSize, segs)

	clickAt(r, centreOf(t, shaper, 1, labels...))
	driveTrail(emitted[1], ops, r, canvasSize, segs)

	if fired[designKey] != 1 {
		t.Errorf("across a colour change, Design fired %d time(s), want 1 (all: %v)", fired[designKey], fired)
	}
}

// TestTrailDrawsTheSameRowAsRender pins the visual half of the addition: the
// frame-time trail draws the row the static path draws, chevrons, spacing,
// colours and all. Nothing about the goldens moves because nothing about the
// drawing changed.
func TestTrailDrawsTheSameRowAsRender(t *testing.T) {
	shaper := defaultShaper(t)
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	fired := map[string]int{}
	segs := segments(fired,
		[2]string{homeKey, "Home"},
		[2]string{designKey, "Design"},
		[2]string{tokensKey, "Tokens"},
	)

	static := breadcrumb.Render(shaper, breadcrumb.Props{Items: trail(), Shaper: shaper},
		tokens.DefaultLight, tokens.Spacing, tokens.DefaultTypography.TitleSmall)
	live := newTrail(t)

	fromRender := golden.Capture(t, canvasSize, scene(static, bg))
	fromTrail := golden.Capture(t, canvasSize, scene(func(gtx layout.Context) layout.Dimensions {
		return live(gtx, segs)
	}, bg))
	if n := golden.PixelDiff(fromRender, fromTrail); n != 0 {
		t.Errorf("the frame-time trail and Render differ in %d pixel(s); the row is meant to be the same row", n)
	}
}

// TestTrailEmptyRendersZero holds the empty trail to the same rule as an
// empty Items.
func TestTrailEmptyRendersZero(t *testing.T) {
	w := newTrail(t)
	r := new(gioinput.Router)
	ops := new(op.Ops)
	if dims := driveTrail(w, ops, r, canvasSize, nil); dims.Size != (image.Point{}) {
		t.Errorf("empty trail measured %v, want zero Dimensions", dims.Size)
	}
}
