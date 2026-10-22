package shell_test

import (
	"context"
	"image"
	"image/color"
	"testing"

	gioinput "gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/patterns/navbar"
	"github.com/vibrantgio/theme/tokens"
)

const (
	tabFrameW = 480
	tabFrameH = 256
)

var tabSize = image.Pt(tabFrameW, tabFrameH)

// defaultShaper returns the shaper every golden here draws with: the default
// typography's faces pinned, system fonts off, so the stored images are the
// same on every machine. A golden test pins its faces with
// DeterministicShaper; application code takes the fallback Shaper.
func defaultShaper(t *testing.T) *text.Shaper {
	t.Helper()
	return tokens.DefaultTypography.DeterministicShaper()
}

// The stacked page draws no text of its own: everything legible in these
// goldens comes through the composed navbar. These titles are what make the
// slot boundaries legible. Titles must stay ASCII — Latin text in Roboto
// rasterises identically on every machine via DeterministicShaper, but a
// symbol may not.
var navLinkTitles = []string{"Docs", "Components"}

// shellNavbar returns the navbar props every shell golden composes.
func shellNavbar(shaper *text.Shaper) navbar.Props {
	links := make([]navbar.Link, len(navLinkTitles))
	for i, l := range navLinkTitles {
		links[i] = navbar.Link{Title: l}
	}
	return navbar.Props{Links: links, Shaper: shaper}
}

// fillRect is a solid-coloured filler used as a section slot in goldens;
// its colour distinguishes regions visually.
func fillRect(c color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	}
}

func scene(w layout.Widget, bg color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// ---- Interaction tests ----

func liveWidget(t *testing.T, sh rx.Observable[layout.Widget]) layout.Widget {
	t.Helper()
	var w layout.Widget
	if err := sh.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("Shell subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("Shell did not emit an initial layout.Widget")
	}
	return w
}

func driveFrame(w layout.Widget, ops *op.Ops, r *gioinput.Router, size image.Point) layout.Dimensions {
	ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(size),
		Ops:         ops,
		Source:      r.Source(),
	}
	dims := w(gtx)
	r.Frame(ops)
	return dims
}
