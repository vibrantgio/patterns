package notifications_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/toast"
	"github.com/vibrantgio/patterns/notifications"
	"github.com/vibrantgio/theme/tokens"
)

const (
	frameW, frameH = 320, 240
)

var (
	frameSize = image.Pt(frameW, frameH)
	// Sharp corner radius. Anti-aliased rounded corners vary slightly
	// between GPU contexts, breaking determinism.
	sharpRadius = tokens.RadiusScale{}
)

// defaultShaper returns the shaper every golden here draws with: the default
// typography's faces pinned, system fonts off, so the stored images are the
// same on every machine. A golden test pins its faces with
// DeterministicShaper; application code takes the fallback Shaper.
func defaultShaper(t *testing.T) *text.Shaper {
	t.Helper()
	return tokens.DefaultTypography.DeterministicShaper()
}

// scene renders w over a flat background sized to the constraints.
func scene(w layout.Widget, bg color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// noteText is the message each status role carries. ASCII only: Latin text in
// Roboto rasterises identically on every machine, and no symbol reaches a
// stored image.
func noteText(r toast.Role) string {
	switch r {
	case toast.Success:
		return "Workspace saved"
	case toast.Warning:
		return "Connection is slow"
	case toast.Error:
		return "Upload failed"
	default:
		return "Syncing tokens"
	}
}

// item returns one notification of the given status role, carrying that
// role's message.
func item(id int64, r toast.Role) notifications.Notification {
	return notifications.Notification{ID: id, Role: r, Text: noteText(r)}
}

// TestColumnGolden records or diffs the stored scenes. The role's leading
// edge and the column ordering are the load-bearing visual signal and the
// text carries the LabelMedium role; one scene stands the column on the
// bottom edge's midpoint, where the design language puts a transient
// confirmation. The scenes composite over a real pane background
// (SurfaceAt(LevelChrome)), so a toast fill that stops separating from real
// app backgrounds fails the diff instead of hiding behind an arbitrary grey.
func TestColumnGolden(t *testing.T) {
	shaper := defaultShaper(t)
	lightBG := tokens.DefaultLight.SurfaceAt(tokens.LevelChrome)
	darkBG := tokens.DefaultDark.SurfaceAt(tokens.LevelChrome)

	cases := []struct {
		name   string
		props  notifications.Props
		items  []notifications.Notification
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{
			name:   "light-empty",
			props:  notifications.Props{Position: notifications.TopRight, Shaper: shaper},
			items:  nil,
			colors: tokens.DefaultLight,
			bg:     lightBG,
		},
		{
			name:  "light-three-stacked",
			props: notifications.Props{Position: notifications.TopRight, Shaper: shaper},
			items: []notifications.Notification{
				item(1, toast.Info),
				item(2, toast.Success),
				item(3, toast.Warning),
			},
			colors: tokens.DefaultLight,
			bg:     lightBG,
		},
		{
			name:   "dark-warning-toast",
			props:  notifications.Props{Position: notifications.BottomRight, Shaper: shaper},
			items:  []notifications.Notification{item(1, toast.Warning)},
			colors: tokens.DefaultDark,
			bg:     darkBG,
		},
		{
			name:  "light-bottom-center",
			props: notifications.Props{Position: notifications.BottomCenter, Shaper: shaper},
			items: []notifications.Notification{
				item(1, toast.Info),
				item(2, toast.Success),
			},
			colors: tokens.DefaultLight,
			bg:     lightBG,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := notifications.Render(shaper, tc.props, tc.items, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelMedium)
			golden.Render(t, tc.name, frameSize, scene(w, tc.bg))
		})
	}
}

// TestEmptyAndPopulatedColumnsDiffer catches regressions where the
// populated branch silently no-ops.
func TestEmptyAndPopulatedColumnsDiffer(t *testing.T) {
	shaper := defaultShaper(t)
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	props := notifications.Props{Position: notifications.TopRight, Shaper: shaper}

	empty := notifications.Render(shaper, props, nil, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelMedium)
	full := notifications.Render(shaper, props, []notifications.Notification{item(1, toast.Info)}, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelMedium)

	imgE := golden.Capture(t, frameSize, scene(empty, bg))
	imgF := golden.Capture(t, frameSize, scene(full, bg))
	if n := golden.PixelDiff(imgE, imgF); n == 0 {
		t.Error("empty and populated columns render identically; expected a toast to appear when populated")
	}
}

// TestColumnPositionAnchoring confirms that swapping Position relocates
// the rendered toast. A TopRight column must differ pixel-wise from a
// BottomLeft column with the same notifications.
func TestColumnPositionAnchoring(t *testing.T) {
	shaper := defaultShaper(t)
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	items := []notifications.Notification{item(1, toast.Info)}

	tr := notifications.Render(shaper, notifications.Props{Position: notifications.TopRight, Shaper: shaper}, items, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelMedium)
	bl := notifications.Render(shaper, notifications.Props{Position: notifications.BottomLeft, Shaper: shaper}, items, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelMedium)

	imgTR := golden.Capture(t, frameSize, scene(tr, bg))
	imgBL := golden.Capture(t, frameSize, scene(bl, bg))
	if n := golden.PixelDiff(imgTR, imgBL); n == 0 {
		t.Error("TopRight and BottomLeft columns render identically; expected corner anchoring")
	}
}
