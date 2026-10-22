package shell_test

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/tokens"
)

// TestEmptyStateFrameTakesAStandingBarsLane holds the frame an empty state
// centres in against the bar the region's rows would scroll under: a bar
// that stands takes its lane off the frame's trailing side and nothing else,
// and a bar that fades takes nothing. Both densities are read, so the lane
// is the bar's width in device pixels and not in dp.
func TestEmptyStateFrameTakesAStandingBarsLane(t *testing.T) {
	c := tokens.PlatformLight
	fading := scrollbar.FromTokens(c, c.ControlBackground)
	standing := fading
	standing.FadeDelay = 0
	frame := image.Pt(600, 400)

	for _, scale := range []float32{1, 2} {
		gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: scale, PxPerSp: scale}}
		lane := gtx.Dp(standing.Width())
		if lane <= 0 {
			t.Fatalf("at %vx the bar's lane is %d px, want a lane to take off", scale, lane)
		}
		if got, want := shell.EmptyStateFrame(gtx, frame, standing), image.Pt(frame.X-lane, frame.Y); got != want {
			t.Errorf("at %vx a standing bar leaves the frame %v, want %v — the lane off the trailing side only", scale, got, want)
		}
		if got := shell.EmptyStateFrame(gtx, frame, fading); got != frame {
			t.Errorf("at %vx a fading bar leaves the frame %v, want the full frame %v", scale, got, frame)
		}
	}

	// A frame narrower than the lane is left with no width rather than a
	// negative one.
	gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	if got := shell.EmptyStateFrame(gtx, image.Pt(2, 50), standing); got != image.Pt(0, 50) {
		t.Errorf("a frame narrower than the lane is left %v, want (0,50)", got)
	}
}
