package tag_test

import (
	"context"
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/tag"
	themecolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

const (
	canvasW, canvasH = 160, 48
)

var (
	canvasSize = image.Pt(canvasW, canvasH)
	// Sharp corner radius keeps the goldens deterministic — anti-aliased
	// rounded corners and the pill's Full radius both vary slightly
	// between GPU contexts, breaking pixel-exact diffs.
	sharpRadius = tokens.RadiusScale{}
)

// defaultShaper returns the shaper every golden here draws with: the default
// typography's faces pinned, system fonts off, so the stored images are the
// same on every machine. A golden test pins its faces with
// DeterministicShaper; application code takes the fallback Shaper. See
// AGENTS.md.
func defaultShaper(t *testing.T) *text.Shaper {
	t.Helper()
	return tokens.DefaultTypography.DeterministicShaper()
}

// scene renders w at the top-left of a canvas-sized constraint over a flat
// fill of the scheme's Surface pin — the ground a resting chip actually
// sits on, and the base its status variants tint, so the goldens record the
// tint against the pane it separates from.
func scene(w layout.Widget, bg color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// The five variants' specimen labels. The status labels are the fixture
// labels the design/mirror harness compares against; keep them in sync.
const (
	labelFilled  = "Popular"
	labelTonal   = "New in 2.0"
	labelSuccess = "Passing"
	labelWarning = "Degraded"
	labelError   = "Failing"
	labelDismiss = "Dismissible"
)

// TestTagGolden records or diffs one golden per variant per scheme: the two
// historical chips (pricing's Filled "Popular", hero's Tonal eyebrow) and
// the three status treatments, each over the scheme's Surface ground.
func TestTagGolden(t *testing.T) {
	shaper := defaultShaper(t)

	cases := []struct {
		name    string
		colors  tokens.ColorTokens
		label   string
		variant tag.Variant
	}{
		{"light-filled", tokens.DefaultLight, labelFilled, tag.Filled},
		{"light-tonal", tokens.DefaultLight, labelTonal, tag.Tonal},
		{"light-success", tokens.DefaultLight, labelSuccess, tag.Success},
		{"light-warning", tokens.DefaultLight, labelWarning, tag.Warning},
		{"light-error", tokens.DefaultLight, labelError, tag.Error},
		// Filled and tonal in the dark scheme too. Their pairings are
		// scheme-dependent like every other one here, and a variant stored
		// in one scheme only is a variant half its regressions can hide in.
		{"dark-filled", tokens.DefaultDark, labelFilled, tag.Filled},
		{"dark-tonal", tokens.DefaultDark, labelTonal, tag.Tonal},
		{"dark-success", tokens.DefaultDark, labelSuccess, tag.Success},
		{"dark-warning", tokens.DefaultDark, labelWarning, tag.Warning},
		{"dark-error", tokens.DefaultDark, labelError, tag.Error},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := tag.Render(shaper, tc.label, tc.variant, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelSmall)
			golden.Render(t, tc.name, canvasSize, scene(w, tc.colors.Surface))
		})
	}
}

// TestDismissibleGolden stores the close mark on the two grounds that read
// it differently: the tonal chip's tint, where the mark is the accent, and
// the filled chip's pin, where it is the on-colour.
func TestDismissibleGolden(t *testing.T) {
	shaper := defaultShaper(t)

	cases := []struct {
		name    string
		colors  tokens.ColorTokens
		variant tag.Variant
	}{
		{"light-tonal-dismiss", tokens.DefaultLight, tag.Tonal},
		{"light-filled-dismiss", tokens.DefaultLight, tag.Filled},
		{"dark-tonal-dismiss", tokens.DefaultDark, tag.Tonal},
		{"dark-filled-dismiss", tokens.DefaultDark, tag.Filled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var dismiss widget.Clickable
			w := tag.RenderDismissible(shaper, labelDismiss, tc.variant, &dismiss, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelSmall)
			golden.Render(t, tc.name, canvasSize, scene(w, tc.colors.Surface))
		})
	}
}

// TestStatusVariantsDiffer confirms the three status treatments are three
// different drawings — a level mapping bug that collapsed two levels onto
// one colour would slip past per-variant goldens only until the next
// regeneration, but never past this.
func TestStatusVariantsDiffer(t *testing.T) {
	shaper := defaultShaper(t)
	capture := func(v tag.Variant) *image.RGBA {
		w := tag.Render(shaper, "Status", v, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelSmall)
		return golden.Capture(t, canvasSize, scene(w, tokens.DefaultLight.Surface))
	}
	success, warning, err := capture(tag.Success), capture(tag.Warning), capture(tag.Error)
	if golden.PixelDiff(success, warning) == 0 {
		t.Error("success and warning tags render identically; expected the level pins to differ")
	}
	if golden.PixelDiff(warning, err) == 0 {
		t.Error("warning and error tags render identically; expected the level pins to differ")
	}
	if golden.PixelDiff(success, err) == 0 {
		t.Error("success and error tags render identically; expected the level pins to differ")
	}
}

// TestTagLightDarkDiffer confirms a status chip flips with the scheme: the
// hue-anchored level roles are paired light/dark ramps, not literals.
func TestTagLightDarkDiffer(t *testing.T) {
	shaper := defaultShaper(t)
	light := tag.Render(shaper, labelSuccess, tag.Success, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelSmall)
	dark := tag.Render(shaper, labelSuccess, tag.Success, tokens.DefaultDark, tokens.Spacing, sharpRadius, tokens.DefaultTypography.LabelSmall)

	imgLight := golden.Capture(t, canvasSize, scene(light, tokens.DefaultLight.Surface))
	imgDark := golden.Capture(t, canvasSize, scene(dark, tokens.DefaultDark.Surface))
	if golden.PixelDiff(imgLight, imgDark) == 0 {
		t.Error("light and dark status tags render identically; expected colour differences")
	}
}

// ---- Contrast ----

// wcagText is WCAG 1.4.3's AA floor for text below 18 pt: the label-small
// role is 11 sp, so every chip's label owes this over its own fill.
const wcagText = 4.5

// wcagGraphic is WCAG 1.4.11's floor for a non-text graphic — here the
// boundary that says a chip is a chip, whether that is the ring a tinted
// chip draws or the fill a pinned one separates with on its own.
const wcagGraphic = 3.0

// TestTagContrast measures every pairing a chip makes, in both schemes, and
// records the numbers in the test log.
//
// Three pairings per variant, and the third is the one that was failing.
// The label over the fill is a text pairing and the two grounds are
// obviously different colours, so nothing there was ever in doubt. The
// *pill* against the pane it rests on is the pairing a chip can lose
// without anything looking broken: a tint and a surface are the same
// lightness by construction, so the tonal chip's primary-200 fill measured
// 1.00:1 against the light Surface and 1.01:1 against the dark one — a pill
// nobody could see, with a perfectly legible label inside it. A tinted chip
// now rings itself and the ring carries the boundary; a filled chip's own
// fill does.
func TestTagContrast(t *testing.T) {
	for _, sc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"light", tokens.DefaultLight},
		{"dark", tokens.DefaultDark},
	} {
		sc := sc
		c := sc.colors
		cases := []struct {
			name  string
			fill  color.NRGBA
			label color.NRGBA
			ring  color.NRGBA // the zero value means the variant draws none
		}{
			{"filled", c.Primary, c.OnPrimary, color.NRGBA{}},
			{"tonal", c.Ramps.Primary.Step(200), c.Primary, c.Primary},
			{"success", c.StatusContainer(tokens.RoleSuccess), c.Text, c.Success},
			{"warning", c.StatusContainer(tokens.RoleWarning), c.Text, c.Warning},
			{"error", c.StatusContainer(tokens.RoleError), c.Text, c.Error},
		}
		for _, tc := range cases {
			t.Run(sc.name+"-"+tc.name, func(t *testing.T) {
				onFill := themecolor.ContrastRatio(tc.label, tc.fill)
				t.Logf("label on fill %.2f:1 (fill %s, label %s)", onFill, hexOf(tc.fill), hexOf(tc.label))
				if onFill < wcagText {
					t.Errorf("label on fill = %.2f:1, want at least %.1f:1", onFill, wcagText)
				}
				// The close mark is drawn in the label's colour, so it is
				// the same pairing measured against the graphic floor —
				// stated rather than assumed, because a variant that gave
				// the mark a colour of its own would need its own row.
				if onFill < wcagGraphic {
					t.Errorf("close mark on fill = %.2f:1, want at least %.1f:1", onFill, wcagGraphic)
				}

				edge, what := tc.ring, "ring"
				if (tc.ring == color.NRGBA{}) {
					edge, what = tc.fill, "fill"
				}
				for _, ground := range []struct {
					name string
					c    color.NRGBA
				}{
					{"Surface", c.Surface},
					{"Background", c.Background},
				} {
					got := themecolor.ContrastRatio(edge, ground.c)
					t.Logf("%s on %s %.2f:1 (%s %s, ground %s)", what, ground.name, got, what, hexOf(edge), hexOf(ground.c))
					if got < wcagGraphic {
						t.Errorf("%s on %s = %.2f:1, want at least %.1f:1", what, ground.name, got, wcagGraphic)
					}
				}
				if what == "ring" {
					got := themecolor.ContrastRatio(tc.ring, tc.fill)
					t.Logf("ring on its own fill %.2f:1", got)
					if got < wcagGraphic {
						t.Errorf("ring on its own fill = %.2f:1, want at least %.1f:1", got, wcagGraphic)
					}
				}
			})
		}
	}
}

// TestStatusFillsAreRealizedNotMixed is the standing guard on the
// reconciliation: a status chip's ground is its role's container, which
// holds the role's hue at the theme's own chroma, and not a composite of
// the pin over the neutral Surface. Mixing in non-linear sRGB is neither
// hue- nor chroma-preserving, and the four grounds it gave this chip came
// out too close to grey — and to each other — to be told apart.
func TestStatusFillsAreRealizedNotMixed(t *testing.T) {
	for _, sc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{{"light", tokens.DefaultLight}, {"dark", tokens.DefaultDark}} {
		c := sc.colors
		for _, r := range []struct {
			name string
			role tokens.Role
		}{
			{"success", tokens.RoleSuccess},
			{"warning", tokens.RoleWarning},
			{"error", tokens.RoleError},
		} {
			_, chroma, _ := themecolor.OKLChFromNRGBA(c.StatusContainer(r.role))
			t.Logf("%s %s container %s chroma %.4f", sc.name, r.name, hexOf(c.StatusContainer(r.role)), chroma)
			// The mixed grounds measured 0.0196–0.0433; the realized ones
			// hold one dial for every role. Anything back down at the old
			// numbers is a chip that has started mixing again.
			if chroma < 0.05 {
				t.Errorf("%s %s container chroma = %.4f, want the realized dial, not a mix toward grey", sc.name, r.name, chroma)
			}
		}
	}
}

func hexOf(c color.NRGBA) string {
	const d = "0123456789ABCDEF"
	return string([]byte{'#', d[c.R>>4], d[c.R&0xf], d[c.G>>4], d[c.G&0xf], d[c.B>>4], d[c.B&0xf]})
}

// ---- Geometry ----

// TestPillHeight pins the compressed pill: the label-small line box plus the
// S1 stop, spent once across both edges rather than once on each.
//
// The chip measured 24 px before — 16 + 2×S1 — which spent a third of its
// height on air around a 14 px ink box that already carries a pixel of
// leading on each side. Nothing about the type moved: the same 11 sp label
// in the same 16 dp line box.
func TestPillHeight(t *testing.T) {
	shaper := defaultShaper(t)
	style := tokens.DefaultTypography.LabelSmall
	want := int(style.LineHeight + tokens.Spacing.S1) // 16 + 4 = 20

	for _, label := range []string{labelFilled, labelTonal, labelSuccess, labelWarning, labelError, ""} {
		for v := tag.Filled; v <= tag.Error; v++ {
			plain := measureTag(t, tag.Render(shaper, label, v, tokens.DefaultLight, tokens.Spacing, tokens.Radius, style))
			if plain.Y != want {
				t.Errorf("variant %d %q: pill height = %d px, want %d px (line box %v + S1 %v)",
					v, label, plain.Y, want, style.LineHeight, tokens.Spacing.S1)
			}
			// The close mark rides inside the pill: a dismissible chip is
			// wider than a plain one and exactly as tall.
			var dismiss widget.Clickable
			withMark := measureTag(t, tag.RenderDismissible(shaper, label, v, &dismiss, tokens.DefaultLight, tokens.Spacing, tokens.Radius, style))
			if withMark.Y != plain.Y {
				t.Errorf("variant %d %q: dismissible height = %d px, plain = %d px; the mark must not make the chip taller",
					v, label, withMark.Y, plain.Y)
			}
			if withMark.X <= plain.X {
				t.Errorf("variant %d %q: dismissible width = %d px, plain = %d px; the mark must take room",
					v, label, withMark.X, plain.X)
			}
		}
	}
}

// TestPillFollowsItsLabelsBox is the answer to what the compression cost:
// nothing, because 20 dp is not a fixed height. The pill is whatever box the
// label reports plus the S1 stop, so a role with a taller line — or a script
// whose glyphs need more than Roboto's ascent and descent, which is what
// makes a label taller than its role's leading — makes a taller pill rather
// than a clipped one. Four of the five specimen labels have no descender at
// all, so this is the case a golden could never show.
func TestPillFollowsItsLabelsBox(t *testing.T) {
	shaper := defaultShaper(t)
	style := tokens.DefaultTypography.LabelSmall

	for _, lineHeight := range []float32{16, 20, 24, 32} {
		st := style
		st.LineHeight = lineHeight
		got := measureTag(t, tag.Render(shaper, labelWarning, tag.Warning, tokens.DefaultLight, tokens.Spacing, tokens.Radius, st))
		want := int(lineHeight + tokens.Spacing.S1)
		if got.Y != want {
			t.Errorf("line box %v: pill height = %d px, want %d px — the pill must follow its label's box, not a number",
				lineHeight, got.Y, want)
		}
	}
}

func measureTag(t *testing.T, w layout.Widget) image.Point {
	t.Helper()
	var ops op.Ops
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(400, 200)},
		Ops:         &ops,
	}
	return w(gtx).Size
}

// ---- Dismissal ----

// liveTag subscribes to a Tag observable and returns the emitted widget.
// The clickable the widget closes over lives in the deferred scope, so it
// stays valid for the test's lifetime.
func liveTag(t *testing.T, o rx.Observable[layout.Widget]) layout.Widget {
	t.Helper()
	var w layout.Widget
	if err := o.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("Tag subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("Tag did not emit an initial widget")
	}
	return w
}

func driveFrame(w layout.Widget, ops *op.Ops, r *gioinput.Router, size image.Point) layout.Dimensions {
	ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: size},
		Ops:         ops,
		Source:      r.Source(),
	}
	dims := w(gtx)
	r.Frame(ops)
	return dims
}

func clickAt(r *gioinput.Router, pos f32.Point) {
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: pos, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: pos, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
}

// TestDismissReportsTheClick is the event wiring end to end: a click on the
// mark reaches OnDismiss, and the chip is still there on the next frame —
// removal is the caller's to do, and a chip that vanished on its own would
// take that decision away from the only party that can make it.
func TestDismissReportsTheClick(t *testing.T) {
	var dismissed int
	w := liveTag(t, tag.Tag(rx.Of(theme.Default()), tag.Props{
		Label:     labelDismiss,
		Variant:   tag.Tonal,
		OnDismiss: func(_ layout.Context) { dismissed++ },
		Shaper:    defaultShaper(t),
	}))

	r := new(gioinput.Router)
	ops := new(op.Ops)
	dims := driveFrame(w, ops, r, canvasSize) // register the input area

	// The mark's centre: S2 in from the right edge plus half the mark.
	pos := f32.Pt(float32(dims.Size.X)-8-4.5, float32(dims.Size.Y)/2)
	clickAt(r, pos)
	after := driveFrame(w, ops, r, canvasSize)
	if dismissed != 1 {
		t.Fatalf("click on the mark: OnDismiss fired %d times, want 1", dismissed)
	}
	if after.Size != dims.Size {
		t.Errorf("the chip measured %v after being dismissed, was %v; the tag must not remove itself", after.Size, dims.Size)
	}
}

// TestDismissReportsOncePerBurst confirms a double click is one dismissal.
// Gio queues both, and reporting the second on the following frame would
// dismiss whatever the caller put in this chip's place.
func TestDismissReportsOncePerBurst(t *testing.T) {
	var dismissed int
	w := liveTag(t, tag.Tag(rx.Of(theme.Default()), tag.Props{
		Label:     labelDismiss,
		OnDismiss: func(_ layout.Context) { dismissed++ },
		Shaper:    defaultShaper(t),
	}))

	r := new(gioinput.Router)
	ops := new(op.Ops)
	dims := driveFrame(w, ops, r, canvasSize)

	pos := f32.Pt(float32(dims.Size.X)-8-4.5, float32(dims.Size.Y)/2)
	clickAt(r, pos)
	clickAt(r, pos)
	driveFrame(w, ops, r, canvasSize)
	driveFrame(w, ops, r, canvasSize)
	if dismissed != 1 {
		t.Errorf("double click on the mark: OnDismiss fired %d times, want 1", dismissed)
	}
}

// TestCloseHitTarget is the measurement the affordance is judged by. The
// drawn x is 9 dp; a click 11 px above the pill's own top edge — outside
// every pixel the chip paints — still lands, because the registered target
// is CloseHitDp square and centred on the mark.
func TestCloseHitTarget(t *testing.T) {
	var dismissed int
	w := liveTag(t, tag.Tag(rx.Of(theme.Default()), tag.Props{
		Label:     labelDismiss,
		Variant:   tag.Tonal,
		OnDismiss: func(_ layout.Context) { dismissed++ },
		Shaper:    defaultShaper(t),
	}))

	r := new(gioinput.Router)
	ops := new(op.Ops)
	dims := driveFrame(w, ops, r, canvasSize)

	// The target is CloseHitDp tall, centred on a mark centred in a pill
	// of dims.Size.Y, so it reaches (CloseHitDp − dims.Size.Y)/2 above the
	// pill. At the 20 px pill and the 24 dp target that is two rows; the
	// click lands on the outermost one.
	slop := (tag.CloseHitDp - dims.Size.Y) / 2
	if slop < 1 {
		t.Fatalf("pill height %d px leaves no slop under a %d dp target", dims.Size.Y, tag.CloseHitDp)
	}
	markCx := float32(dims.Size.X) - 8 - 4.5
	for _, p := range []struct {
		name string
		at   f32.Point
	}{
		{"above the pill", f32.Pt(markCx, -float32(slop)+0.5)},
		{"below the pill", f32.Pt(markCx, float32(dims.Size.Y+slop)-0.5)},
		// Sideways the target grows inward, over the chip's own trailing
		// padding and the tail of its label, rather than out past the
		// pill's right edge — so a row of chips set edge to edge still
		// has one target per chip.
		{"left of the mark", f32.Pt(markCx-float32(tag.CloseHitDp)/2+1, float32(dims.Size.Y)/2)},
	} {
		dismissed = 0
		clickAt(r, p.at)
		driveFrame(w, ops, r, canvasSize)
		if dismissed != 1 {
			t.Errorf("click %s at %v: OnDismiss fired %d times, want 1 — the target is not %d dp",
				p.name, p.at, dismissed, tag.CloseHitDp)
		}
	}

	// And the slop stops: a click a whole target's width away from the
	// mark is not a dismissal, or a chip would swallow its neighbours.
	dismissed = 0
	clickAt(r, f32.Pt(markCx, -float32(tag.CloseHitDp)))
	driveFrame(w, ops, r, canvasSize)
	if dismissed != 0 {
		t.Errorf("click a target's width above the mark: OnDismiss fired %d times, want 0", dismissed)
	}
}

// TestNilOnDismissTakesNoInput pins the nil-means-inert half: a chip with
// no OnDismiss draws no mark, and a click where the mark would have been
// falls through it.
func TestNilOnDismissTakesNoInput(t *testing.T) {
	shaper := defaultShaper(t)
	plain := liveTag(t, tag.Tag(rx.Of(theme.Default()), tag.Props{
		Label:   labelDismiss,
		Variant: tag.Tonal,
		Shaper:  shaper,
	}))

	r := new(gioinput.Router)
	ops := new(op.Ops)
	plainDims := driveFrame(plain, ops, r, canvasSize)

	var dismiss widget.Clickable
	marked := tag.RenderDismissible(shaper, labelDismiss, tag.Tonal, &dismiss, tokens.DefaultLight, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.LabelSmall)
	markedDims := measureTag(t, marked)
	if plainDims.Size.X >= markedDims.X {
		t.Errorf("plain chip %d px wide, dismissible %d px: a nil OnDismiss must not reserve the mark's room",
			plainDims.Size.X, markedDims.X)
	}

	// Nothing to click: the plain chip registers no area at all, so a
	// press anywhere on it reaches no tag. The assertion is that the
	// frame is drawn and nothing panics on a chip with no clickable —
	// the useful half is TestDismissReportsTheClick's positive case.
	clickAt(r, f32.Pt(float32(plainDims.Size.X)-4, float32(plainDims.Size.Y)/2))
	if got := driveFrame(plain, ops, r, canvasSize); got.Size != plainDims.Size {
		t.Errorf("plain chip measured %v after a click, was %v", got.Size, plainDims.Size)
	}
}

// TestPaintedBoundsMatchReportedBounds is the guard on the ring's placement,
// and it is a pixel test because the defect it catches is invisible to a
// size assertion: every variant reported the same box already, and the
// ringed ones painted two pixels taller than it.
//
// A stroke is centred on the path it follows, so a 1 dp ring stroked on the
// pill's own edge spent half its width outside the chip. In a row of chips
// the filled one then sat two pixels shorter than its neighbours and the gap
// beside it lost a pixel at each end, and the ring itself — split across a
// pixel boundary — never rendered at the colour it was chosen for. The ring
// is nested fills now, and this measures what actually reaches the canvas.
func TestPaintedBoundsMatchReportedBounds(t *testing.T) {
	shaper := defaultShaper(t)
	// A ground the chip cannot be confused with: neither scheme's fills,
	// rings or inks are this, so any pixel that is not it was painted.
	ground := color.NRGBA{R: 0x00, G: 0xff, B: 0x00, A: 0xff}

	for v := tag.Filled; v <= tag.Error; v++ {
		for _, dismissible := range []bool{false, true} {
			var reported image.Point
			var dismiss widget.Clickable
			chip := func(gtx layout.Context) layout.Dimensions {
				w := tag.Render(shaper, labelSuccess, v, tokens.DefaultLight, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.LabelSmall)
				if dismissible {
					w = tag.RenderDismissible(shaper, labelSuccess, v, &dismiss, tokens.DefaultLight, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.LabelSmall)
				}
				// Offset so a chip that overpainted its box on the top or
				// left has somewhere to do it that the capture can see.
				defer op.Offset(image.Pt(8, 8)).Push(gtx.Ops).Pop()
				reported = w(gtx).Size
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			img := golden.Capture(t, canvasSize, scene(chip, ground))
			if img == nil {
				return // headless unavailable; Capture called t.Skip
			}
			painted := paintedBounds(img, ground)
			want := image.Rect(8, 8, 8+reported.X, 8+reported.Y)
			if painted != want {
				t.Errorf("variant %d dismissible=%v: painted %v, reported %v — the chip draws outside the box it claims",
					v, dismissible, painted, want)
			}
		}
	}
}

// paintedBounds returns the smallest rectangle holding every pixel that is
// not exactly ground.
func paintedBounds(img *image.RGBA, ground color.NRGBA) image.Rectangle {
	out := image.Rectangle{}
	first := true
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == ground.R && uint8(g>>8) == ground.G && uint8(bl>>8) == ground.B {
				continue
			}
			p := image.Rect(x, y, x+1, y+1)
			if first {
				out, first = p, false
			} else {
				out = out.Union(p)
			}
		}
	}
	return out
}

// TestRingRendersAtItsOwnColour measures the ring off the canvas rather than
// off the token, because those were two different colours until the ring
// moved inside the pill.
//
// Stroked on the pill's own path, the 1 dp ring straddled a pixel boundary
// and came out as two rows at half strength: over the light success fill the
// pixels measured 1.64:1 where the colour they were drawn in measures
// 4.54:1, so the ratio this component was designed to and the ratio it
// shipped were not the same number. A ring that reads at the ratio it was
// chosen for has to land on whole pixels, and the only way to prove it did
// is to read the pixels.
func TestRingRendersAtItsOwnColour(t *testing.T) {
	shaper := defaultShaper(t)
	for _, sc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{{"light", tokens.DefaultLight}, {"dark", tokens.DefaultDark}} {
		for _, tc := range []struct {
			name string
			v    tag.Variant
			ring color.NRGBA
		}{
			{"tonal", tag.Tonal, sc.colors.Primary},
			{"success", tag.Success, sc.colors.Success},
			{"warning", tag.Warning, sc.colors.Warning},
			{"error", tag.Error, sc.colors.Error},
		} {
			t.Run(sc.name+"-"+tc.name, func(t *testing.T) {
				var size image.Point
				chip := func(gtx layout.Context) layout.Dimensions {
					size = tag.Render(shaper, labelSuccess, tc.v, sc.colors, tokens.Spacing, tokens.Radius, tokens.DefaultTypography.LabelSmall)(gtx).Size
					return layout.Dimensions{Size: gtx.Constraints.Max}
				}
				img := golden.Capture(t, canvasSize, scene(chip, sc.colors.Surface))
				if img == nil {
					return // headless unavailable; Capture called t.Skip
				}
				// The top edge at the pill's midpoint: the one place a Full
				// radius leaves a run of straight, axis-aligned ring.
				r, g, b, _ := img.At(size.X/2, 0).RGBA()
				got := color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
				t.Logf("top edge %s, ring token %s", hexOf(got), hexOf(tc.ring))
				if got != tc.ring {
					t.Errorf("the ring rendered %s where its token is %s: a ring off the pixel grid reads at a ratio nobody chose",
						hexOf(got), hexOf(tc.ring))
				}
			})
		}
	}
}
