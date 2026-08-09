package modal_test

import (
	"context"
	"image"
	"image/color"
	"reflect"
	"testing"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/cadence/modal"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/prism/golden"
	"github.com/vibrantgio/spectrum/theme"
	"github.com/vibrantgio/spectrum/tokens"
)

const (
	canvasW, canvasH = 320, 240
)

var (
	canvasSize = image.Pt(canvasW, canvasH)
	// Sharp corner radius. Anti-aliased rounded corners vary slightly
	// between GPU contexts, breaking determinism.
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

// fillRect is a sharp-edged solid widget used as a Body or Action stand-in.
// Text and rounded paths are avoided in goldens because GPU font and AA
// rasterisation are non-deterministic across platforms.
func fillRect(c color.NRGBA, heightDp float32) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		h := gtx.Dp(unit.Dp(heightDp))
		size := image.Pt(gtx.Constraints.Max.X, h)
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	}
}

// fixedRect is a sharp-edged solid widget with explicit width and height.
// Used for footer action stand-ins so their hit rect is predictable.
func fixedRect(c color.NRGBA, widthDp, heightDp float32) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(unit.Dp(widthDp)), gtx.Dp(unit.Dp(heightDp)))
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	}
}

// scene renders w into a canvas-sized constraint over a flat background.
func scene(w layout.Widget, bgColor color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// The header text the goldens carry. It was blank until F4.4b, on the theory
// that font rasterisation was non-deterministic; F4.2 pinned the faces by
// configuration and F4.3 moved every golden onto DeterministicShaper, so Latin
// text in Roboto rasterises identically on every machine. ASCII only, per
// F4.2 — no symbol reaches a stored image.
//
// There are two of them because G0A.2 split the fixture set by intent, and a
// fixture's title is half of what makes it that intent legible. "Discard
// changes?" is a question you must answer; "Preferences" is a place you
// opened. Before the split every golden carried the question AND a panel's X
// over a dismissing scrim, which is the wrong screen the goal was called to
// fix — the images were recording it.
const (
	panelTitle    = "Preferences"
	decisionTitle = "Discard changes?"
	// modalTitle is the interaction tests' title; those tests are mostly
	// decisions, and none of them looks at pixels.
	modalTitle = decisionTitle
)

// ---- Golden tests ----

// TestModalGolden records or diffs the stored goldens, re-cut by G0A.2 to show
// one fixture of each intent rather than four of one:
//
//   - light-open / dark-open / light-closed are PANELS — a title and a quiet
//     ghost X, the surface you can leave.
//   - light-with-actions is the DECISION — the question, the footer that
//     answers it, and no X anywhere.
//
// The cross, scrim, surface, and action rectangles are deterministic clip
// shapes; the title carries the TitleMedium role.
func TestModalGolden(t *testing.T) {
	shaper := defaultShaper(t)
	body := fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40)
	cancel := fixedRect(color.NRGBA{R: 80, G: 160, B: 220, A: 255}, 60, 28)
	discard := fixedRect(color.NRGBA{R: 220, G: 100, B: 100, A: 255}, 60, 28)

	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	darkBG := color.NRGBA{R: 20, G: 20, B: 20, A: 255}

	cases := []struct {
		name     string
		open     bool
		title    string
		decision *modal.Decision
		actions  []layout.Widget
		colors   tokens.ColorTokens
		bg       color.NRGBA
	}{
		{"light-open", true, panelTitle, nil, nil, tokens.DefaultLight, lightBG},
		{"dark-open", true, panelTitle, nil, nil, tokens.DefaultDark, darkBG},
		{"light-closed", false, panelTitle, nil, nil, tokens.DefaultLight, lightBG},
		// The destructive primary is marked, so Return would reach Cancel and
		// not Discard. Nothing about that is visible here — the fixture states
		// it because a fixture is also documentation of the intended call.
		{"light-with-actions", true, decisionTitle,
			&modal.Decision{Destructive: true}, []layout.Widget{cancel, discard},
			tokens.DefaultLight, lightBG},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			props := modal.Props{
				Title:    tc.title,
				Body:     body,
				Actions:  tc.actions,
				Decision: tc.decision,
				Shaper:   shaper,
			}
			w := modal.Render(shaper, props, tc.open, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography.TitleMedium, tokens.Comfortable)
			golden.Render(t, tc.name, canvasSize, scene(w, tc.bg))
		})
	}
}

// TestModalOpenAndClosedDiffer confirms that flipping the open flag
// changes the rendered output. Catches regressions where the open
// branch silently no-ops.
func TestModalOpenAndClosedDiffer(t *testing.T) {
	shaper := defaultShaper(t)
	body := fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40)
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}

	open := modal.Render(shaper, modal.Props{Title: modalTitle, Body: body, Shaper: shaper}, true, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.TitleMedium, tokens.Comfortable)
	closed := modal.Render(shaper, modal.Props{Title: modalTitle, Body: body, Shaper: shaper}, false, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.TitleMedium, tokens.Comfortable)

	imgOpen := golden.Capture(t, canvasSize, scene(open, bg))
	imgClosed := golden.Capture(t, canvasSize, scene(closed, bg))
	if n := golden.PixelDiff(imgOpen, imgClosed); n == 0 {
		t.Error("open and closed modal render identically; expected scrim + surface in open")
	}
}

// densityTheme returns a theme whose density is d, with sharp corners
// for golden determinism — the E1.4 injection idiom, mirroring prism's
// density tests.
func densityTheme(d tokens.Density) theme.Theme {
	th := theme.Default()
	th.Density = rx.Of(d)
	th.Radius = rx.Of(tokens.RadiusScale{})
	return th
}

// TestModalCompactGolden records or diffs the compact-density golden
// through the LIVE pipeline. The modal itself is a surface — its inset
// and gaps stay on the spacing scale (E1.4 verdict) — but its close
// affordance is a live prism/button, which densifies to a 28 dp square
// through its own theme subscription; that shrinking button is what this
// golden pins down.
func TestModalCompactGolden(t *testing.T) {
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	body := fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40)
	obs := modal.Modal(rx.Of(densityTheme(tokens.Compact)), modal.Props{
		Open:   rx.Of(true),
		Title:  panelTitle,
		Body:   body,
		Shaper: defaultShaper(t),
	})
	var w layout.Widget
	if err := obs.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("Modal subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("Modal did not emit an initial widget")
	}
	golden.Render(t, "light-compact-open", canvasSize, scene(w, lightBG))
}

// ---- Interaction tests ----

// liveModal subscribes to the Modal observable, drains the trampoline
// scheduler with Wait(), and returns the latest emitted layout.Widget.
// State referenced by the widget closure remains valid for the test's
// lifetime because it is captured by the rx.Defer scope.
func liveModal(t *testing.T, props modal.Props) layout.Widget {
	t.Helper()
	if props.Shaper == nil {
		props.Shaper = defaultShaper(t)
	}
	obs := modal.Modal(rx.Of(theme.Default()), props)
	var w layout.Widget
	if err := obs.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("Modal subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("Modal did not emit an initial widget")
	}
	return w
}

// liveButtonAction subscribes to a labelled prism/button keyed to a caller-owned
// clickable and returns its latest emitted widget. The caller passes &clk in
// Props.ActionFocusTags so the button joins the modal's Tab cycle while owning
// its own focus tag and ring.
func liveButtonAction(t *testing.T, label string, clk *widget.Clickable) layout.Widget {
	t.Helper()
	obs := button.Button(rx.Of(theme.Default()), button.Props{
		Label:     label,
		Clickable: clk,
		Shaper:    defaultShaper(t),
	})
	var w layout.Widget
	if err := obs.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("button action subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("button action did not emit a widget")
	}
	return w
}

// driveFrame lays out w against ops + router, returns the rendered dims.
// ops is reset before layout; events queued on the router before the call
// are delivered during w's layout pass and r.Frame.
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

// TestEscapeInvokesOnClose verifies Measurable (a) — pressing Escape while
// the modal holds focus invokes the OnClose callback.
func TestEscapeInvokesOnClose(t *testing.T) {
	var closed int
	w := liveModal(t, modal.Props{
		Open:    rx.Of(true),
		Body:    fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
		OnClose: func(_ layout.Context) { closed++ },
	})

	r := new(gioinput.Router)
	ops := new(op.Ops)

	// Frame 1: register tags and request initial focus.
	driveFrame(w, ops, r, canvasSize)
	// Frame 2: focus has been applied; the close button now holds focus.
	driveFrame(w, ops, r, canvasSize)

	r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	driveFrame(w, ops, r, canvasSize)

	if closed != 1 {
		t.Errorf("OnClose call count after Escape = %d, want 1", closed)
	}
}

// TestCloseButtonActivatesOnClose verifies the close affordance — now a
// prism/button keyed to &st.closeClick — invokes OnClose when activated by
// keyboard while focused. The button drains its own Clicked() and routes
// through Props.OnClick; the modal no longer checks Clicked itself, so this
// is the only guard against the close button silently doing nothing.
func TestCloseButtonActivatesOnClose(t *testing.T) {
	var closed int
	w := liveModal(t, modal.Props{
		Open:    rx.Of(true),
		Body:    fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
		OnClose: func(_ layout.Context) { closed++ },
	})

	r := new(gioinput.Router)
	ops := new(op.Ops)

	// Frame 1 registers tags + requests initial focus; frame 2 applies it,
	// leaving the close button focused (same setup as the Escape test).
	driveFrame(w, ops, r, canvasSize)
	driveFrame(w, ops, r, canvasSize)

	// widget.Clickable registers a click on Return/Space release after a
	// matching press while focused — queue both in one frame.
	r.Queue(
		key.Event{Name: key.NameReturn, State: key.Press},
		key.Event{Name: key.NameReturn, State: key.Release},
	)
	driveFrame(w, ops, r, canvasSize)

	if closed != 1 {
		t.Errorf("OnClose call count after close-button activation = %d, want 1", closed)
	}
}

// TestBackdropClickInvokesOnClose verifies Measurable (c) — pressing inside
// the scrim region but outside the modal surface invokes OnClose. A press
// inside the surface must NOT invoke OnClose.
//
// This is the PANEL half of the intent contract; TestBackdropClickOnDecisionIsInert
// is the other half.
func TestBackdropClickInvokesOnClose(t *testing.T) {
	var closed int
	w := liveModal(t, modal.Props{
		Open:    rx.Of(true),
		Body:    fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
		OnClose: func(_ layout.Context) { closed++ },
	})

	r := new(gioinput.Router)
	ops := new(op.Ops)

	driveFrame(w, ops, r, canvasSize)
	driveFrame(w, ops, r, canvasSize)

	// (1) Press near the top-left corner — guaranteed scrim, never surface.
	corner := f32.Pt(4, 4)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: corner, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: corner, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
	driveFrame(w, ops, r, canvasSize)
	if closed == 0 {
		t.Fatalf("scrim click did not invoke OnClose; closed = %d", closed)
	}
	scrimClicks := closed

	// (2) Press at the canvas centre — guaranteed inside the surface — must
	// not invoke OnClose.
	centre := f32.Pt(canvasW/2, canvasH/2)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: centre, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: centre, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
	driveFrame(w, ops, r, canvasSize)
	if closed != scrimClicks {
		t.Errorf("surface click bled through to scrim; OnClose went from %d to %d", scrimClicks, closed)
	}
}

// TestTabTrapsFocusWithinModal verifies Measurable (b) — Tab cycles among
// modal focus tags and does not advance focus to a background-registered
// focusable, no matter how many times Tab is pressed.
func TestTabTrapsFocusWithinModal(t *testing.T) {
	// Two prism/button footer actions plus the implicit close button → three
	// modal tags. The actions own their clickables; those are the action focus
	// tags (route (a)), so the modal cycles among all three without wrapping.
	body := fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40)
	var clk1, clk2 widget.Clickable
	action1 := liveButtonAction(t, "A", &clk1)
	action2 := liveButtonAction(t, "B", &clk2)

	// A background focusable that the test will assert focus never reaches.
	backgroundTag := new(int)

	mw := liveModal(t, modal.Props{
		Open:            rx.Of(true),
		Body:            body,
		Actions:         []layout.Widget{action1, action2},
		ActionFocusTags: []event.Tag{&clk1, &clk2},
		OnClose:         func(_ layout.Context) {},
	})

	// Compose the modal over a background that also registers a focusable
	// tag. This is the harder version of the test: it proves Tab cannot
	// escape the modal even when the router has another focus target to
	// advance to.
	composed := func(gtx layout.Context) layout.Dimensions {
		// Background focusable: a 1×1 region with a FocusFilter.
		bgClip := clip.Rect{Max: image.Pt(1, 1)}.Push(gtx.Ops)
		event.Op(gtx.Ops, backgroundTag)
		// Drain the synthetic focus events so the router retains focus
		// when set, matching FocusGroup.Update's idiom.
		for {
			if _, ok := gtx.Event(key.FocusFilter{Target: backgroundTag}); !ok {
				break
			}
		}
		bgClip.Pop()
		return mw(gtx)
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)

	driveFrame(composed, ops, r, canvasSize)
	driveFrame(composed, ops, r, canvasSize) // initial focus is applied.

	// At this point the modal's close button holds focus. Press Tab N+1
	// times — far more than the number of focus stops in the modal — and
	// assert focus is still NOT on the background tag.
	for i := 0; i < 12; i++ {
		r.Queue(key.Event{Name: key.NameTab, State: key.Press})
		driveFrame(composed, ops, r, canvasSize)
	}

	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(canvasSize),
		Ops:         ops,
		Source:      r.Source(),
	}
	if gtx.Focused(backgroundTag) {
		t.Fatal("Tab cycle escaped the modal: background tag has focus")
	}
}

// TestShiftTabTrapsFocusWithinModal mirrors TestTabTrapsFocusWithinModal
// for the reverse direction. With Shift+Tab the router would otherwise
// MoveFocus(FocusBackward), so this is a distinct code path on Gio's
// side.
func TestShiftTabTrapsFocusWithinModal(t *testing.T) {
	body := fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40)
	var clk1, clk2 widget.Clickable
	action1 := liveButtonAction(t, "A", &clk1)
	action2 := liveButtonAction(t, "B", &clk2)

	backgroundTag := new(int)

	mw := liveModal(t, modal.Props{
		Open:            rx.Of(true),
		Body:            body,
		Actions:         []layout.Widget{action1, action2},
		ActionFocusTags: []event.Tag{&clk1, &clk2},
		OnClose:         func(_ layout.Context) {},
	})

	composed := func(gtx layout.Context) layout.Dimensions {
		bgClip := clip.Rect{Max: image.Pt(1, 1)}.Push(gtx.Ops)
		event.Op(gtx.Ops, backgroundTag)
		for {
			if _, ok := gtx.Event(key.FocusFilter{Target: backgroundTag}); !ok {
				break
			}
		}
		bgClip.Pop()
		return mw(gtx)
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)

	driveFrame(composed, ops, r, canvasSize)
	driveFrame(composed, ops, r, canvasSize)

	for i := 0; i < 12; i++ {
		r.Queue(key.Event{Name: key.NameTab, Modifiers: key.ModShift, State: key.Press})
		driveFrame(composed, ops, r, canvasSize)
	}

	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(canvasSize),
		Ops:         ops,
		Source:      r.Source(),
	}
	if gtx.Focused(backgroundTag) {
		t.Fatal("Shift+Tab cycle escaped the modal: background tag has focus")
	}
}

// ---- GX.5: footer actions own their own focus tags ----

// TestActionOwnsFocusTag confirms route (a): a prism/button action joins the
// Tab cycle via its own caller-owned clickable. After tabbing off the close
// button, the action's &clickable — not a modal-interposed tag — holds focus,
// which is what makes the button draw its own (single) focus ring.
func TestActionOwnsFocusTag(t *testing.T) {
	var clk widget.Clickable
	action := liveButtonAction(t, "OK", &clk)
	w := liveModal(t, modal.Props{
		Open:            rx.Of(true),
		Body:            fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
		Actions:         []layout.Widget{action},
		ActionFocusTags: []event.Tag{&clk},
		OnClose:         func(_ layout.Context) {},
	})

	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveFrame(w, ops, r, canvasSize) // register tags + request initial focus
	driveFrame(w, ops, r, canvasSize) // initial focus → close button

	r.Queue(key.Event{Name: key.NameTab, State: key.Press})
	driveFrame(w, ops, r, canvasSize) // focus → action's own clickable

	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(canvasSize),
		Ops:         ops,
		Source:      r.Source(),
	}
	if !gtx.Focused(&clk) {
		t.Error("after Tab from the close button, the prism/button action's own clickable must hold focus (route (a)); the modal must not interpose its own action tag")
	}
}

// TestActionFocusRingNotDoubled confirms the modal draws no focus ring around a
// focused action — so a prism/button action shows only its own ring, never a
// doubled outer one. Two blank actions reserve space and register a focus tag
// but paint nothing. If the modal drew a ring around the focused action it would
// land at the first action's x when that one is focused and at the second's x
// otherwise, so the two captures would differ; drawing nothing leaves the
// surface pixel-identical. Both captures keep the close button unfocused (focus
// sits on an action in each), so its ring cannot confound the diff.
func TestActionFocusRingNotDoubled(t *testing.T) {
	tag0, tag1 := new(int), new(int)
	blank := func(tag *int) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			sz := image.Pt(gtx.Dp(unit.Dp(64)), gtx.Dp(unit.Dp(32)))
			defer clip.Rect{Max: sz}.Push(gtx.Ops).Pop()
			event.Op(gtx.Ops, tag) // register as a focus/input target
			return layout.Dimensions{Size: sz}
		}
	}
	w := liveModal(t, modal.Props{
		Open:            rx.Of(true),
		Body:            fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
		Actions:         []layout.Widget{blank(tag0), blank(tag1)},
		ActionFocusTags: []event.Tag{tag0, tag1},
		OnClose:         func(_ layout.Context) {},
	})

	win, err := headless.NewWindow(canvasW, canvasH)
	if err != nil {
		t.Skipf("headless rendering not supported: %v", err)
		return
	}
	defer win.Release()
	r := new(gioinput.Router)
	var ops op.Ops
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}

	render := func(gpu bool) {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(canvasSize),
			Ops:         &ops,
			Source:      r.Source(),
		}
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		w(gtx)
		if gpu {
			if err := win.Frame(&ops); err != nil {
				t.Fatalf("Frame: %v", err)
			}
		}
		r.Frame(&ops)
	}
	shoot := func() *image.RGBA {
		render(true)
		img := image.NewRGBA(image.Rectangle{Max: canvasSize})
		if err := win.Screenshot(img); err != nil {
			t.Fatalf("Screenshot: %v", err)
		}
		return img
	}
	isFocused := func(tag event.Tag) bool {
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(canvasSize),
			Ops:         new(op.Ops),
			Source:      r.Source(),
		}
		return gtx.Focused(tag)
	}

	render(false) // register tags + request initial focus
	render(false) // initial focus → close button

	r.Queue(key.Event{Name: key.NameTab, State: key.Press})
	render(false)
	if !isFocused(tag0) {
		t.Fatal("first action tag not focused after one Tab")
	}
	imgA := shoot()

	r.Queue(key.Event{Name: key.NameTab, State: key.Press})
	render(false)
	if !isFocused(tag1) {
		t.Fatal("second action tag not focused after two Tabs")
	}
	imgB := shoot()

	if n := golden.PixelDiff(imgA, imgB); n != 0 {
		t.Errorf("focusing different footer actions changed %d pixel(s); the modal must draw no focus ring around actions (doubled/outer-ring regression)", n)
	}
}

// ---- G0A.2: the two intents ----

// scrimPress queues a press-and-release near the top-left corner — guaranteed
// scrim, never surface — and drives one frame.
func scrimPress(w layout.Widget, ops *op.Ops, r *gioinput.Router) {
	corner := f32.Pt(4, 4)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: corner, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: corner, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
	driveFrame(w, ops, r, canvasSize)
}

// pressReturn queues a Return press-and-release and drives one frame. Both
// halves go in one frame because a focused widget.Clickable registers its
// activation on the release that follows a matching press — so a test that
// only queued the press would prove nothing about whether the modal took the
// key away from the button.
func pressReturn(w layout.Widget, ops *op.Ops, r *gioinput.Router) {
	r.Queue(
		key.Event{Name: key.NameReturn, State: key.Press},
		key.Event{Name: key.NameReturn, State: key.Release},
	)
	driveFrame(w, ops, r, canvasSize)
}

// openFrames drives the two frames an opening modal needs: the first registers
// the tags and requests initial focus, the second applies it.
func openFrames(w layout.Widget, ops *op.Ops, r *gioinput.Router) {
	driveFrame(w, ops, r, canvasSize)
	driveFrame(w, ops, r, canvasSize)
}

// TestBackdropClickOnDecisionIsInert is the decision half of the scrim
// contract: on a decision dialog, dismissal is one of the answers, so a stray
// click on the backdrop must not give it. Neither OnClose nor Cancel may fire.
func TestBackdropClickOnDecisionIsInert(t *testing.T) {
	var closed, cancelled int
	var clk widget.Clickable
	w := liveModal(t, modal.Props{
		Open:            rx.Of(true),
		Title:           modalTitle,
		Body:            fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
		Actions:         []layout.Widget{liveButtonAction(t, "Cancel", &clk)},
		ActionFocusTags: []event.Tag{&clk},
		Decision:        &modal.Decision{Cancel: func(_ layout.Context) { cancelled++ }},
		OnClose:         func(_ layout.Context) { closed++ },
	})

	r := new(gioinput.Router)
	ops := new(op.Ops)
	openFrames(w, ops, r)
	scrimPress(w, ops, r)

	if closed != 0 || cancelled != 0 {
		t.Errorf("backdrop click on a decision dialog fired OnClose=%d Cancel=%d, want 0 and 0: "+
			"the scrim must be inert when dismissal is one of the decision's answers", closed, cancelled)
	}
}

// TestBackdropClickOnPanelCloses is the panel half stated in the intent's own
// terms, next to its opposite. TestBackdropClickInvokesOnClose covers the same
// ground from the older Measurable (c); this one exists so the pair reads as
// one contract and a future change cannot silence half of it unnoticed.
func TestBackdropClickOnPanelCloses(t *testing.T) {
	var closed int
	w := liveModal(t, modal.Props{
		Open:    rx.Of(true),
		Title:   modalTitle,
		Body:    fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
		OnClose: func(_ layout.Context) { closed++ },
	})

	r := new(gioinput.Router)
	ops := new(op.Ops)
	openFrames(w, ops, r)
	scrimPress(w, ops, r)

	if closed == 0 {
		t.Error("backdrop click on a panel did not invoke OnClose; a panel is a place you can leave")
	}
}

// TestEscapeWorksOnBothIntents pins the clause that survives the split:
// whatever the archetype, Escape leaves. On a panel it invokes OnClose; on a
// decision dialog it invokes Cancel, which is Apple's binding and the reason a
// decision needs no X.
func TestEscapeWorksOnBothIntents(t *testing.T) {
	t.Run("panel", func(t *testing.T) {
		var closed int
		w := liveModal(t, modal.Props{
			Open:    rx.Of(true),
			Title:   modalTitle,
			Body:    fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
			OnClose: func(_ layout.Context) { closed++ },
		})
		r := new(gioinput.Router)
		ops := new(op.Ops)
		openFrames(w, ops, r)
		r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
		driveFrame(w, ops, r, canvasSize)
		if closed != 1 {
			t.Errorf("Escape on a panel: OnClose called %d times, want 1", closed)
		}
	})

	t.Run("decision", func(t *testing.T) {
		var closed, cancelled, confirmed int
		var clk widget.Clickable
		w := liveModal(t, modal.Props{
			Open:            rx.Of(true),
			Title:           modalTitle,
			Body:            fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
			Actions:         []layout.Widget{liveButtonAction(t, "Cancel", &clk)},
			ActionFocusTags: []event.Tag{&clk},
			Decision: &modal.Decision{
				Confirm: func(_ layout.Context) { confirmed++ },
				Cancel:  func(_ layout.Context) { cancelled++ },
			},
			OnClose: func(_ layout.Context) { closed++ },
		})
		r := new(gioinput.Router)
		ops := new(op.Ops)
		openFrames(w, ops, r)
		r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
		driveFrame(w, ops, r, canvasSize)
		if cancelled != 1 {
			t.Errorf("Escape on a decision dialog: Cancel called %d times, want 1", cancelled)
		}
		if confirmed != 0 {
			t.Errorf("Escape invoked Confirm %d times; Escape is the dismissing answer", confirmed)
		}
		if closed != 0 {
			t.Errorf("Escape invoked OnClose %d times; a decision that names a Cancel routes Escape there", closed)
		}
	})

	t.Run("decision without a Cancel still leaves", func(t *testing.T) {
		var closed int
		var clk widget.Clickable
		w := liveModal(t, modal.Props{
			Open:            rx.Of(true),
			Title:           modalTitle,
			Body:            fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
			Actions:         []layout.Widget{liveButtonAction(t, "OK", &clk)},
			ActionFocusTags: []event.Tag{&clk},
			Decision:        &modal.Decision{Confirm: func(_ layout.Context) {}},
			OnClose:         func(_ layout.Context) { closed++ },
		})
		r := new(gioinput.Router)
		ops := new(op.Ops)
		openFrames(w, ops, r)
		r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
		driveFrame(w, ops, r, canvasSize)
		if closed != 1 {
			t.Errorf("Escape on a Cancel-less decision: OnClose called %d times, want 1", closed)
		}
	})
}

// TestReturnActivatesTheDefaultAction verifies the non-destructive case:
// Return reaches Confirm, and it reaches it even though a footer button holds
// focus — the desktop rule is that Return answers the DEFAULT action while
// Space answers the focused control. The focused button's own OnClick must
// therefore stay silent.
func TestReturnActivatesTheDefaultAction(t *testing.T) {
	var confirmed, cancelled, buttonClicked int
	var clk widget.Clickable
	action, err := button.Button(rx.Of(theme.Default()), button.Props{
		Label:     "Cancel",
		Clickable: &clk,
		Shaper:    defaultShaper(t),
		OnClick:   func(_ layout.Context) { buttonClicked++ },
	}).First()
	if err != nil {
		t.Fatalf("action button: %v", err)
	}

	w := liveModal(t, modal.Props{
		Open:            rx.Of(true),
		Title:           modalTitle,
		Body:            fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
		Actions:         []layout.Widget{action},
		ActionFocusTags: []event.Tag{&clk},
		Decision: &modal.Decision{
			Confirm: func(_ layout.Context) { confirmed++ },
			Cancel:  func(_ layout.Context) { cancelled++ },
		},
	})

	r := new(gioinput.Router)
	ops := new(op.Ops)
	openFrames(w, ops, r)
	pressReturn(w, ops, r)

	if confirmed != 1 {
		t.Errorf("Return on a decision dialog: Confirm called %d times, want 1", confirmed)
	}
	if cancelled != 0 {
		t.Errorf("Return invoked Cancel %d times; Confirm is not destructive, so it is the default", cancelled)
	}
	if buttonClicked != 0 {
		t.Errorf("Return also activated the focused footer button %d times; the modal must claim "+
			"Return before the footer lays out, leaving Space to the focused control", buttonClicked)
	}
}

// TestReturnNeverReachesADestructivePrimary is Apple's rule, tested at the
// only place it can be broken. "Discard changes?" answering Return with
// Discard is the exact failure this forbids.
func TestReturnNeverReachesADestructivePrimary(t *testing.T) {
	t.Run("Cancel takes the default", func(t *testing.T) {
		var discarded, cancelled int
		var clk widget.Clickable
		w := liveModal(t, modal.Props{
			Open:            rx.Of(true),
			Title:           modalTitle,
			Body:            fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
			Actions:         []layout.Widget{liveButtonAction(t, "Discard", &clk)},
			ActionFocusTags: []event.Tag{&clk},
			Decision: &modal.Decision{
				Confirm:     func(_ layout.Context) { discarded++ },
				Destructive: true,
				Cancel:      func(_ layout.Context) { cancelled++ },
			},
		})
		r := new(gioinput.Router)
		ops := new(op.Ops)
		openFrames(w, ops, r)
		pressReturn(w, ops, r)

		if discarded != 0 {
			t.Errorf("Return reached the destructive Confirm %d times; it must never", discarded)
		}
		if cancelled != 1 {
			t.Errorf("Return: Cancel called %d times, want 1 — a destructive primary hands the default to Cancel", cancelled)
		}
	})

	t.Run("no Cancel means a dead Return, not a fall-through", func(t *testing.T) {
		var discarded, buttonClicked int
		var clk widget.Clickable
		action, err := button.Button(rx.Of(theme.Default()), button.Props{
			Label:     "Discard",
			Clickable: &clk,
			Shaper:    defaultShaper(t),
			OnClick:   func(_ layout.Context) { buttonClicked++ },
		}).First()
		if err != nil {
			t.Fatalf("action button: %v", err)
		}
		w := liveModal(t, modal.Props{
			Open:            rx.Of(true),
			Title:           modalTitle,
			Body:            fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
			Actions:         []layout.Widget{action},
			ActionFocusTags: []event.Tag{&clk},
			Decision: &modal.Decision{
				Confirm:     func(_ layout.Context) { discarded++ },
				Destructive: true,
			},
		})
		r := new(gioinput.Router)
		ops := new(op.Ops)
		openFrames(w, ops, r)
		pressReturn(w, ops, r)

		if discarded != 0 || buttonClicked != 0 {
			t.Errorf("Return fell through on a destructive decision with no Cancel: Confirm=%d, focused button=%d; "+
				"the modal must claim the key and do nothing rather than let it reach the destruction",
				discarded, buttonClicked)
		}
	})
}

// TestPanelLeavesReturnToTheFocusedControl is the negative of the two tests
// above: a panel claims no default action, so Return still activates whatever
// holds focus — which on an opening panel is its close button.
func TestPanelLeavesReturnToTheFocusedControl(t *testing.T) {
	var closed int
	w := liveModal(t, modal.Props{
		Open:    rx.Of(true),
		Title:   modalTitle,
		Body:    fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40),
		OnClose: func(_ layout.Context) { closed++ },
	})

	r := new(gioinput.Router)
	ops := new(op.Ops)
	openFrames(w, ops, r)
	pressReturn(w, ops, r)

	if closed != 1 {
		t.Errorf("Return on a panel with its close button focused: OnClose called %d times, want 1", closed)
	}
}

// TestDecisionDrawsNoCloseAffordance checks the derivation in pixels rather
// than in predicates: the same Props with and without a Decision differ, and
// the difference is the X — a decision dialog rendered next to a panel whose
// close button is hidden by HideClose is pixel-identical.
func TestDecisionDrawsNoCloseAffordance(t *testing.T) {
	shaper := defaultShaper(t)
	body := fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40)
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}

	render := func(p modal.Props) layout.Widget {
		p.Title, p.Body, p.Shaper = modalTitle, body, shaper
		return modal.Render(shaper, p, true, tokens.DefaultLight, tokens.Spacing, sharpRadius,
			tokens.DefaultTypography.TitleMedium, tokens.Comfortable)
	}

	panel := golden.Capture(t, canvasSize, scene(render(modal.Props{}), bg))
	decision := golden.Capture(t, canvasSize, scene(render(modal.Props{Decision: &modal.Decision{}}), bg))
	hidden := golden.Capture(t, canvasSize, scene(render(modal.Props{HideClose: true}), bg))

	if n := golden.PixelDiff(panel, decision); n == 0 {
		t.Error("a decision dialog renders identically to a panel; it must drop the close X")
	}
	if n := golden.PixelDiff(decision, hidden); n != 0 {
		t.Errorf("a decision dialog differs from a close-less panel by %d pixel(s); "+
			"hiding the X is exactly what the intent derives", n)
	}
}

// TestHideCloseStillWorksOnAPanel is the deprecation window: the field keeps
// compiling AND keeps its meaning for the archetype it belongs to, so an
// existing caller is not silently changed underneath.
//
// As of G0B.2 the in-org caller list is EMPTY — this test and its two
// neighbours above are the field's only remaining users anywhere in the
// twenty-one repositories. It is kept for callers outside the organization,
// and it is the thing to delete first when the field goes.
func TestHideCloseStillWorksOnAPanel(t *testing.T) {
	shaper := defaultShaper(t)
	body := fillRect(color.NRGBA{R: 200, G: 200, B: 200, A: 255}, 40)
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}

	with := modal.Render(shaper, modal.Props{Title: modalTitle, Body: body, Shaper: shaper},
		true, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.TitleMedium, tokens.Comfortable)
	without := modal.Render(shaper, modal.Props{Title: modalTitle, Body: body, Shaper: shaper, HideClose: true},
		true, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography.TitleMedium, tokens.Comfortable)

	if n := golden.PixelDiff(golden.Capture(t, canvasSize, scene(with, bg)), golden.Capture(t, canvasSize, scene(without, bg))); n == 0 {
		t.Error("HideClose no longer hides a panel's close button; the deprecation window must keep it working")
	}
}

// TestDefaultActionDerivation is the rule as a unit: which callback Return
// reaches, for every shape a Decision can take. There is no field with which
// to override any row of this table — that absence is the enforcement.
func TestDefaultActionDerivation(t *testing.T) {
	confirm := func(layout.Context) {}
	cancel := func(layout.Context) {}
	name := func(f func(layout.Context)) string {
		switch {
		case f == nil:
			return "nil"
		case reflect.ValueOf(f).Pointer() == reflect.ValueOf(confirm).Pointer():
			return "Confirm"
		default:
			return "Cancel"
		}
	}

	cases := []struct {
		name string
		d    modal.Decision
		want string
	}{
		{"a plain confirmation defaults to Confirm", modal.Decision{Confirm: confirm, Cancel: cancel}, "Confirm"},
		{"a destructive primary hands the default to Cancel", modal.Decision{Confirm: confirm, Destructive: true, Cancel: cancel}, "Cancel"},
		{"a destructive primary with no Cancel has no default at all", modal.Decision{Confirm: confirm, Destructive: true}, "nil"},
		{"an empty decision has no default", modal.Decision{}, "nil"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := name(tc.d.DefaultAction()); got != tc.want {
				t.Errorf("DefaultAction() = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestIntentIsDerivedFromDecision pins the single source of truth: there is no
// intent field to fall out of step with the callbacks.
func TestIntentIsDerivedFromDecision(t *testing.T) {
	if got := (modal.Props{}).Intent(); got != modal.IntentPanel {
		t.Errorf("Props{}.Intent() = %v, want %v", got, modal.IntentPanel)
	}
	if got := (modal.Props{HideClose: true}).Intent(); got != modal.IntentPanel {
		t.Errorf("HideClose does not make a decision dialog: Intent() = %v, want %v", got, modal.IntentPanel)
	}
	if got := (modal.Props{Decision: &modal.Decision{}}).Intent(); got != modal.IntentDecision {
		t.Errorf("Props{Decision: …}.Intent() = %v, want %v", got, modal.IntentDecision)
	}
	if got, want := modal.IntentPanel.String()+"/"+modal.IntentDecision.String(), "panel/decision"; got != want {
		t.Errorf("Intent.String() pair = %q, want %q", got, want)
	}
}
