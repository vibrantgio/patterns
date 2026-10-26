package shell

import (
	"image"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/pointershape"
	"github.com/vibrantgio/theme/tokens"
)

// MarkDp is the box both drawings of a chrome mark centre their figure in.
// The set the figures come from was drawn and weight-matched against the
// platform's own symbols at 16, 20 and 24 dp, and a mark with
// axis-aligned edges takes the top of that range: those edges land on whole
// device pixels at 16 and 24 and between them at 20, where a figure's faint
// lines smear into one grey column.
const MarkDp = 24

// PaneMark draws one BARE chrome mark: the figure alone, in the fill a
// toolbar draws its own symbols in, over whatever it stands on.
//
// Which of the two drawings a mark takes is a property of what it stands
// on and not of the control it belongs to. A mark standing on the PANE is
// bare — no capsule, no fill, no rim — because the pane is not a toolbar.
// MEASURED, voicememos-multi-folder-2026-09-18.png: the pane's two marks
// carry no capsule, no fill and no rim, while every mark in the toolbar
// beside them does, and the bare marks reach #4b4b4b at their darkest — a
// floor a 1 px stroke at 1x cannot pass — against the #4d4d4d the
// toolbar's own symbols plateau at.
func PaneMark(gtx layout.Context, c tokens.PlatformColors, click *widget.Clickable, mark icons.Painter, title string) layout.Dimensions {
	box := gtx.Dp(unit.Dp(MarkDp))
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.ClassOp(semantic.Button).Add(gtx.Ops)
		semantic.LabelOp(title).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointershape.OverSize(gtx.Ops, image.Pt(box, box), pointer.CursorPointer)
		if mark != nil {
			mark(gtx, box, c.ToolbarLabel)
		}
		return layout.Dimensions{Size: image.Pt(box, box)}
	})
}

// ToolbarControl draws one BORDERED toolbar control: the figure centred in
// the platform's capsule, the capsule's own fill and rim, and the drop
// shadow it casts on what lies under the toolbar.
//
// It is [PaneMark]'s counterpart, the drawing a mark takes while it stands
// in a toolbar rather than on a pane. checked is the persistent state of a
// control that records a yes.
//
// Its width is the capsule's, whatever room the row gives it, so a toolbar
// lays it out once.
//
// The shadow is cast AROUND the clickable and never inside it: it falls
// outside the control's own box, and a clickable clips what it wraps to the
// box it reports.
func ToolbarControl(gtx layout.Context, c tokens.PlatformColors, rad tokens.RadiusScale, den tokens.Density,
	click *widget.Clickable, mark icons.Painter, title string, checked bool) layout.Dimensions {
	state := button.RenderState{
		Variant: button.Chrome,
		Hovered: click.Hovered(),
		Pressed: click.Pressed(),
		Focused: gtx.Focused(click),
		Checked: checked,
	}
	face := button.BorderedFace(mark, c, rad, den, state)
	return button.BorderedShadow(gtx, c, state, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.ClassOp(semantic.Button).Add(gtx.Ops)
			semantic.LabelOp(title).Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			return face(gtx)
		})
	})
}

// ToggleSidebar is the one button that sends the pane away and brings it
// back. It has two places and not two controls: bare at the pane's trailing
// corner while the pane stands, bordered at the toolbar's leading end once
// it is away. Only one of the two is ever on screen, so there is ONE
// clickable and one message — a second gesture source for one affordance
// gains nothing and costs the loop its single source.
//
// Which place the button takes is the arrangement's answer and not the
// window's: the window hands the button over and [TwoColumn] puts it where
// the pane's state says.
type ToggleSidebar struct {
	// Click is the one gesture source, held by the window across frames.
	Click *widget.Clickable

	// Mark is the figure both places are drawn with, so the two read as one control
	// and not as two. Nil draws no figure.
	//
	// It is one drawing that never morphs, as on the platform: a mark that
	// changed would leave the reader guessing whether it shows the present
	// state or the next one. What the button is about to do is in the title
	// it carries.
	Mark icons.Painter

	// PaneTitle is spoken and shown while the button stands on the pane,
	// ToolbarTitle while it stands in the toolbar. The two differ because
	// the act does: one sends the pane away and the other brings it back.
	PaneTitle, ToolbarTitle string

	// OnPress answers a press, wherever the button stood when it was
	// pressed. The window sends its own message from here, so the one
	// gesture source produces one message op. Nil answers nothing, which
	// is the static render path.
	OnPress func(gtx layout.Context)
}

// drain answers one press on the button. It is called once per frame by
// the arrangement, wherever the button stands, so the one clickable is
// read in one place whichever of its two places is on screen.
func (t *ToggleSidebar) drain(gtx layout.Context) {
	if t == nil || t.Click == nil || !t.Click.Clicked(gtx) {
		return
	}
	if t.OnPress != nil {
		t.OnPress(gtx)
	}
}
