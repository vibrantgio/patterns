package modal

import (
	"image"
	"testing"

	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/theme/tokens"
)

// footerConstraints lays the footer out over a wide row and returns the
// constraints each action was handed, in order.
func footerConstraints(t *testing.T, actions int) []layout.Constraints {
	t.Helper()
	got := make([]layout.Constraints, 0, actions)
	probe := func(gtx layout.Context) layout.Dimensions {
		got = append(got, gtx.Constraints)
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 24)}
	}
	props := Props{}
	for range actions {
		props.Actions = append(props.Actions, probe)
	}
	w := footerWidget(props, resolvedTokens{spacing: tokens.Spacing})
	if w == nil {
		t.Fatal("footerWidget returned nil for a footer with actions")
	}
	var ops op.Ops
	w(layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(400, 100)},
		Ops:         &ops,
	})
	return got
}

// TestTheFooterLaysEveryActionOutInThePlatformsWidth is the width moving from
// the callers into the pattern: a caller hands the footer an action and not a
// width, and every action gets the same measured box.
func TestTheFooterLaysEveryActionOutInThePlatformsWidth(t *testing.T) {
	got := footerConstraints(t, 2)
	if len(got) != 2 {
		t.Fatalf("the footer laid out %d actions, want 2", len(got))
	}
	for i, cs := range got {
		if cs.Max.X != dialogButtonWDp {
			t.Errorf("action %d was given %d px of width, want the platform's %d",
				i, cs.Max.X, dialogButtonWDp)
		}
		if cs.Min.X != 0 {
			t.Errorf("action %d was given a %d px minimum; the box is a budget, not a floor a wider label has to break",
				i, cs.Min.X)
		}
	}
}

// TestAWiderLabelWidensItsOwnButtonAlone is the other half of that rule: the
// box is what a label that fits takes, and a label that does not fit widens
// its button by its own measure — the button beside it unmoved.
func TestAWiderLabelWidensItsOwnButtonAlone(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	var short, long layout.Dimensions
	action := func(label string, out *layout.Dimensions) layout.Widget {
		w := button.Render(shaper, label, tokens.PlatformLight, tokens.Spacing,
			tokens.Radius, tokens.DefaultTypography.LabelLarge, tokens.Comfortable,
			button.RenderState{})
		return func(gtx layout.Context) layout.Dimensions {
			d := w(gtx)
			*out = d
			return d
		}
	}
	props := Props{Actions: []layout.Widget{
		action("Cancel", &short),
		action("Replace every matching note", &long),
	}}
	var ops op.Ops
	footerWidget(props, resolvedTokens{spacing: tokens.Spacing})(layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(400, 100)},
		Ops:         &ops,
	})
	if short.Size.X != dialogButtonWDp {
		t.Errorf("a label inside the box drew %d px wide, want the platform's %d",
			short.Size.X, dialogButtonWDp)
	}
	if long.Size.X <= dialogButtonWDp {
		t.Errorf("a label wider than the box drew %d px, want more than %d — it was elided into the box instead of widening its button",
			long.Size.X, dialogButtonWDp)
	}
}

// TestADialogOpensOnItsBodysFirstFocusable is the Modal entry's rule: the
// first field holds the keyboard when a dialog opens, as the platform's
// sheet shows. The body declares which control that is; the header's close
// affordance and the footer's answers wait their turn in the Tab cycle.
func TestADialogOpensOnItsBodysFirstFocusable(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()

	// A body of two focusables and a footer of two, all live buttons so each
	// registers the tag it owns. The body's first is what must take the
	// keyboard.
	var first, second, cancel, confirm widget.Clickable
	bodyFirst := liveButton(t, shaper, "First", &first)
	bodySecond := liveButton(t, shaper, "Second", &second)
	body := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(bodyFirst), layout.Rigid(bodySecond))
	}
	props := Props{
		Body:             body,
		Actions:          []layout.Widget{liveButton(t, shaper, "Cancel", &cancel), liveButton(t, shaper, "OK", &confirm)},
		DynamicFocusTags: func() []event.Tag { return []event.Tag{&first, &second} },
		ActionFocusTags:  []event.Tag{&cancel, &confirm},
		OnClose:          func(_ layout.Context) {},
		Shaper:           shaper,
	}

	for _, tc := range []struct {
		name string
		prep func(p *Props)
		want event.Tag
		why  string
	}{
		{"panel", func(p *Props) { p.Decision = nil }, &first,
			"a panel opens on its body's first control, not on the close affordance"},
		{"decision", func(p *Props) { p.Decision = &Decision{} }, &first,
			"a decision opens on its body's first control, not on its first answer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := props
			tc.prep(&p)
			if got := focusedTagAfterOpen(t, shaper, p); got != tc.want {
				t.Errorf("the dialog opened with the keyboard on %v, want the body's first control %v — %s",
					got, tc.want, tc.why)
			}
		})
	}
}

// TestADialogWithNothingToFocusInItsBodyOpensOnTheCycle pins the other end:
// a question and two answers has no field, so the opening focus falls to the
// first tag in the Tab cycle.
func TestADialogWithNothingToFocusInItsBodyOpensOnTheCycle(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	var cancel, confirm widget.Clickable
	props := Props{
		Body:            func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(100, 40)} },
		Actions:         []layout.Widget{liveButton(t, shaper, "Cancel", &cancel), liveButton(t, shaper, "OK", &confirm)},
		ActionFocusTags: []event.Tag{&cancel, &confirm},
		Decision:        &Decision{},
		Shaper:          shaper,
	}
	if got := focusedTagAfterOpen(t, shaper, props); got != &cancel {
		t.Errorf("a decision with nothing to focus in its body opened on %v, want its first answer", got)
	}
}

// focusedTagAfterOpen opens the modal, drives the two frames a live focus
// command needs — one to register the tags, one for the router to act on the
// command — and reports which of the modal's tags holds the keyboard.
func focusedTagAfterOpen(t *testing.T, shaper *text.Shaper, props Props) event.Tag {
	t.Helper()
	st := newState(props)
	st.pushed = true
	st.wantInitialFocus = true
	st.arb.push(st)
	t.Cleanup(func() { st.arb.pop(st) })

	tok := resolvedTokens{
		color:   tokens.PlatformLight,
		spacing: tokens.Spacing,
		radius:  tokens.RadiusScale{},
		title:   tokens.DefaultTypography.TitleMedium,
	}
	closeW := liveCloseWidget(t, st, shaper)
	r := new(gioinput.Router)
	var ops op.Ops
	drive := func() {
		ops.Reset()
		drawModal(layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(320, 240)),
			Ops:         &ops,
			Source:      r.Source(),
		}, shaper, props, tok, st, true, closeW)
		r.Frame(&ops)
	}
	drive()
	drive()

	ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(320, 240)),
		Ops:         &ops,
		Source:      r.Source(),
	}
	tags, _ := focusTags(props, st)
	for _, tag := range tags {
		if gtx.Focused(tag) {
			return tag
		}
	}
	return nil
}
