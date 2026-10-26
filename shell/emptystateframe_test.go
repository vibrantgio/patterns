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

// TestEmptyStateFrameTakesNoLane holds the frame an empty state centres in
// against the bar the region's rows would scroll under: an empty region
// shows no bar, so the frame is the full frame whether the bar stands or
// fades. Both densities are read, so a lane in device pixels cannot hide.
func TestEmptyStateFrameTakesNoLane(t *testing.T) {
	c := tokens.PlatformLight
	fading := scrollbar.FromTokens(c, c.ControlBackground)
	standing := fading.Always()
	frame := image.Pt(600, 400)

	for _, scale := range []float32{1, 2} {
		gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: scale, PxPerSp: scale}}
		for _, bar := range []scrollbar.Style{standing, fading} {
			if got := shell.EmptyStateFrame(gtx, frame, bar); got != frame {
				t.Errorf("at %vx with FadeDelay %v the frame is %v, want the full frame %v",
					scale, bar.FadeDelay, got, frame)
			}
		}
	}
}
