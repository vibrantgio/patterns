// Package findbar provides the find bar: the strip that a find within the
// content opens, under the title bar and over the column it searches. Its
// parts, leading to trailing, are the find field, the previous and next
// control, and Done.
//
// The bar stands over the column it searches and over no other. A sidebar
// or a rail beside that column keeps its own fill and its own height: lay the
// bar and the column out with [Over], inside the column's own share of the
// window. The bar's fill is the fill of the column it searches, never a value
// of its own. MEASURED, textedit-find-empty-{light,dark}.png and
// mail-find-bar-edge-{light,dark}.png: TextEdit's bar takes the title bar's
// fill over its one column, and Mail's bar covers the message's column alone
// in that column's own fill.
//
// The bar opens with the keyboard in its field, and it can open holding a
// given query ([State.Query]). Done and Escape close it, Escape wherever the
// keyboard is while the bar is open, and both give the keyboard back to the
// content ([Props.Content]). The count shows only while the field holds the
// keyboard. What a match is, how the matches are marked and what stepping
// means are the caller's: the bar holds a query and nothing else.
package findbar

import (
	"image"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/input"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/mvu"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// The bar's height and the clear space around its parts.
//
// MEASURED, textedit-find-empty-{light,dark}.png: the bar's fill runs 31 rows
// with a 1 px hairline under it. At the bar's middle row, 7 clear columns lead
// the field, 7 stand before the previous and next control, 8 before Done and
// 9 trail it. Mail's bar (mail-find-bar-edge-{light,dark}.png) is 31 rows too,
// and its field starts 7 px after the column's first pixel.
const (
	FillDp     = 31
	leadDp     = 7
	fieldGapDp = 7
	doneGapDp  = 8
	trailDp    = 9
)

// The platform's own text for the bar's parts.
const (
	defaultPrompt   = "Find"
	defaultDone     = "Done"
	defaultPrevious = "Previous"
	defaultNext     = "Next"
)

// Seam is the hairline under the bar: the platform's separator over the
// fill of the column the bar searches. MEASURED,
// mail-find-bar-edge-{light,dark}.png: Mail's hairline under its bar is the
// separator over the column's fill to the byte, #e6e6e6 light and #343434
// dark. TextEdit's bar draws a hairline of its own, recorded and not
// followed.
func Seam(c tokens.PlatformColors, fill color.NRGBA) color.NRGBA {
	return vgcolor.Flatten(c.Separator, fill)
}

// State is what the caller asks of the bar.
type State struct {
	// Open is whether the bar stands over the column.
	Open bool

	// Opens counts the caller's requests to open the bar. The bar gives the
	// keyboard to its field when Open turns true and each time Opens
	// changes while Open is true, so a second request with the bar open
	// gives the keyboard back to the field.
	Opens int

	// Query, when non-empty, is the text the field holds when the bar acts
	// on a request to open. An empty Query keeps the field's text.
	Query string
}

// Props configures a find bar.
type Props struct {
	// Open emits what the caller asks of the bar. A nil Open keeps the bar
	// closed.
	Open rx.Observable[State]

	// Fill answers the fill of the column the bar searches, for the scheme
	// the bar draws in. A nil Fill is the window's own fill, which is the
	// column's fill where the column draws none of its own.
	Fill func(tokens.PlatformColors) color.NRGBA

	// Content answers the focus tag of the content the bar searches. Done
	// and Escape give the keyboard to it. The bar asks for it on the frame
	// that closes the bar, so the content may make its tag after the bar is
	// built. A nil Content, or a nil tag, leaves the keyboard where it is.
	Content func() event.Tag

	// OnClose is called when Done is pressed or Escape is pressed while the
	// bar is open, and CloseMessage, if non-nil, is emitted as
	// mvu.MessageOp at the same time. The caller closes the bar by emitting
	// a State with Open false.
	OnClose      func(gtx layout.Context)
	CloseMessage any

	// OnChange is called with the field's text on every change, a query the
	// bar opens with included. Message, if non-nil, is emitted as
	// mvu.MessageOp on every change.
	OnChange func(gtx layout.Context, query string)
	Message  any

	// OnNext and OnPrevious are called when the reader steps to the next or
	// the previous match: Enter and Shift+Enter in the field, or the
	// previous and next control. NextMessage and PreviousMessage are
	// emitted as mvu.MessageOp at the same time.
	OnNext          func(gtx layout.Context)
	OnPrevious      func(gtx layout.Context)
	NextMessage     any
	PreviousMessage any

	// Count, if non-nil, is asked on every frame what the query has found.
	// The field shows the answer inside it before its clear mark, and only
	// while the field holds the keyboard and holds text.
	Count func() string

	// FocusTag, if non-nil, is called once when the bar is built with the
	// field's focus tag. A caller reads gtx.Focused with it to show the
	// matches only while the field holds the keyboard.
	FocusTag func(tag event.Tag)

	// Prompt is the field's prompt. Empty is the platform's "Find".
	Prompt string

	// DoneTitle is Done's title. Empty is the platform's "Done".
	DoneTitle string

	// PreviousDescription and NextDescription are the screen-reader labels
	// of the previous and next control. Empty is "Previous" and "Next".
	PreviousDescription string
	NextDescription     string
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// fillFor is the bar's fill for the colour set c.
func (p Props) fillFor(c tokens.PlatformColors) color.NRGBA {
	if p.Fill == nil {
		return c.WindowBackground
	}
	return p.Fill(c)
}

// bar is one find bar's frame state. The frame reads and writes it, and the
// field's callbacks write the query on the frame.
type bar struct {
	props Props

	// field is the field's focus tag, and setText replaces the field's text.
	field   event.Tag
	setText func(string)
	// done is Done's clickable. The bar reads whether Done holds the
	// keyboard when the caller closes the bar.
	done *widget.Clickable

	// query is the field's text.
	query string
	// inField is whether the field holds the keyboard on this frame. The
	// bar sets it before its parts lay out.
	inField bool

	// open and opens are the request the bar last acted on.
	open  bool
	opens int
}

// FindBar returns the find bar as a stream of layout.Widget values. The bar
// draws across the width it is given while [State.Open] is true, and takes no
// room while it is false. Lay it out on every frame, open or closed,
// so that it sees the caller close it: [Over] does this.
//
// Both integration paths are supported: the callbacks for FRP, and the
// Message fields for MVU.
func FindBar(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	open := props.Open
	if open == nil {
		open = rx.Of(State{})
	}
	colors := rx.DistinctUntilChanged(func(a, b tokens.PlatformColors) bool { return a == b })(
		rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] { return t.Platform }))

	return rx.Defer(func() rx.Observable[layout.Widget] {
		b := &bar{props: props, done: new(widget.Clickable)}
		fieldObs := input.SearchField(th, input.SearchFieldProps{
			Placeholder: orDefault(props.Prompt, defaultPrompt),
			Variant:     input.FindBar,
			Surface:     props.fillFor,
			FocusTag: func(tag event.Tag) {
				b.field = tag
				if props.FocusTag != nil {
					props.FocusTag(tag)
				}
			},
			SetText: func(set func(string)) { b.setText = set },
			OnChange: func(gtx layout.Context, text string) {
				b.query = text
				if props.OnChange != nil {
					props.OnChange(gtx, text)
				}
			},
			Message:         props.Message,
			OnNext:          props.OnNext,
			OnPrevious:      props.OnPrevious,
			NextMessage:     props.NextMessage,
			PreviousMessage: props.PreviousMessage,
			Count:           b.count,
		})
		// The previous and next control and Done take the fill they stand on
		// as a colour, so they are built again when the scheme changes.
		stepsObs := rx.SwitchMap(colors, func(c tokens.PlatformColors) rx.Observable[layout.Widget] {
			return button.FindBarSteps(th, button.FindBarStepsProps{
				HasQuery:            func() bool { return b.query != "" },
				Surface:             props.fillFor(c),
				PreviousDescription: orDefault(props.PreviousDescription, defaultPrevious),
				NextDescription:     orDefault(props.NextDescription, defaultNext),
				OnPrevious:          props.OnPrevious,
				OnNext:              props.OnNext,
				PreviousMessage:     props.PreviousMessage,
				NextMessage:         props.NextMessage,
			})
		})
		doneObs := rx.SwitchMap(colors, func(c tokens.PlatformColors) rx.Observable[layout.Widget] {
			return button.Button(th, button.Props{
				Title:     orDefault(props.DoneTitle, defaultDone),
				Variant:   button.FindBar,
				Surface:   props.fillFor(c),
				Clickable: b.done,
				OnClick:   b.close,
			})
		})
		parts := rx.CombineLatest3(fieldObs, stepsObs, doneObs)
		all := rx.CombineLatest3(parts, colors, open)
		return rx.Map(all, func(n rx.Tuple3[rx.Tuple3[layout.Widget, layout.Widget, layout.Widget], tokens.PlatformColors, State]) layout.Widget {
			field, steps, done := n.First.First, n.First.Second, n.First.Third
			c, st := n.Second, n.Third
			return func(gtx layout.Context) layout.Dimensions {
				b.frame(gtx, st)
				if !st.Open {
					return layout.Dimensions{}
				}
				return layoutBar(gtx, c, props.fillFor(c), field, steps, done)
			}
		})
	})
}

// frame acts on the caller's request and on Escape, before the bar's parts
// lay out. Escape is read first in the frame, so it reaches the bar before
// the content asks for it; the find bar field does not take it.
func (b *bar) frame(gtx layout.Context, st State) {
	switch {
	case st.Open && (!b.open || st.Opens != b.opens):
		if st.Query != "" && b.setText != nil {
			b.setText(st.Query)
		}
		if b.field != nil {
			gtx.Execute(key.FocusCmd{Tag: b.field})
		}
	case !st.Open && b.open:
		// The caller closed the bar. The keyboard goes back to the content
		// only from the bar's own parts.
		if (b.field != nil && gtx.Focused(b.field)) || gtx.Focused(b.done) {
			b.giveBack(gtx)
		}
	}
	b.open, b.opens = st.Open, st.Opens

	if st.Open {
		for {
			ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
			if !ok {
				break
			}
			if ke, isKey := ev.(key.Event); isKey && ke.State == key.Press {
				b.close(gtx)
			}
		}
	}
	b.inField = b.field != nil && gtx.Focused(b.field)
}

// close gives the keyboard back to the content and tells the caller.
func (b *bar) close(gtx layout.Context) {
	b.giveBack(gtx)
	if b.props.OnClose != nil {
		b.props.OnClose(gtx)
	}
	if b.props.CloseMessage != nil {
		mvu.MessageOp{Message: b.props.CloseMessage}.Add(gtx.Ops)
	}
}

func (b *bar) giveBack(gtx layout.Context) {
	if b.props.Content == nil {
		return
	}
	if tag := b.props.Content(); tag != nil {
		gtx.Execute(key.FocusCmd{Tag: tag})
	}
}

// count is the field's count: the caller's answer while the field holds the
// keyboard and holds text, and nothing otherwise. MEASURED,
// textedit-find-unfocused-{light,dark}.png: with the keyboard in the document
// the field shows no count.
func (b *bar) count() string {
	if b.props.Count == nil || !b.inField || b.query == "" {
		return ""
	}
	return b.props.Count()
}

// Over lays bar out over column: the bar takes its own rows at the top, and
// the column takes the rest, so the bar covers no part of the column. Give
// it the column's own share of the window, so that a sidebar beside the
// column keeps its own fill and its own height.
func Over(bar, column layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(bar),
			layout.Flexed(1, column),
		)
	}
}

// layoutBar lays the bar out across the width it is given: the fill and the
// seam under it, and the three parts leading to trailing. The field takes the
// width the other parts leave. Each part is centred on the fill's rows, with
// the odd row below the part, as the platform centres them. MEASURED,
// textedit-find-unfocused-{light,dark}.png: the 22 px field has 5 rows above
// it and 4 below, and a 20 px button has 6 above and 5 below.
func layoutBar(gtx layout.Context, c tokens.PlatformColors, fill color.NRGBA, field, steps, done layout.Widget) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(FillDp))
	hair := max(gtx.Dp(unit.Dp(1)), 1)
	paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
	paint.FillShape(gtx.Ops, Seam(c, fill), clip.Rect{
		Min: image.Pt(0, size.Y), Max: image.Pt(size.X, size.Y+hair),
	}.Op())

	centre := func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = 0
			gtx.Constraints.Max.Y = size.Y
			m := op.Record(gtx.Ops)
			dims := w(gtx)
			call := m.Stop()
			defer op.Offset(image.Pt(0, (size.Y-dims.Size.Y+1)/2)).Push(gtx.Ops).Pop()
			call.Add(gtx.Ops)
			return layout.Dimensions{Size: image.Pt(dims.Size.X, size.Y)}
		}
	}
	row := typeset.WithFill(gtx, fill)
	row.Constraints = layout.Exact(size)
	layout.Flex{}.Layout(row,
		layout.Rigid(complayout.HSpacer(leadDp)),
		layout.Flexed(1, centre(field)),
		layout.Rigid(complayout.HSpacer(fieldGapDp)),
		layout.Rigid(centre(steps)),
		layout.Rigid(complayout.HSpacer(doneGapDp)),
		layout.Rigid(centre(done)),
		layout.Rigid(complayout.HSpacer(trailDp)),
	)
	return layout.Dimensions{Size: image.Pt(size.X, size.Y+hair)}
}

// Render produces the bar in an explicit state, with no event processing.
// It is for golden images and static demonstrations; production code uses
// [FindBar].
//
// fill is the fill of the column the bar searches. field is the field's
// state; Render sets its variant and its surface. The previous and next
// control is switched on while the field holds text. typo gives the field's
// SystemFont role and Done's SmallSystemFont role.
func Render(
	shaper *text.Shaper,
	c tokens.PlatformColors,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	typo tokens.Typography,
	d tokens.Density,
	fill color.NRGBA,
	field input.RenderState,
) layout.Widget {
	field.Variant = input.FindBar
	field.Surface = fill
	search := input.RenderSearch(shaper, defaultPrompt, c, sp, rad, typo.SystemFont, d, field)
	seg := button.RenderState{Variant: button.FindBar, Surface: fill, Disabled: field.Text == ""}
	steps := func(gtx layout.Context) layout.Dimensions {
		return button.BorderedSegments(gtx, nil, c, rad, tokens.TextStyle{}, d, []button.BorderedSegment{
			{Icon: icons.Mark(icons.ChevronLeft), State: seg},
			{Icon: icons.Mark(icons.ChevronRight), State: seg},
		})
	}
	done := button.Render(shaper, defaultDone, c, sp, rad, typo.SmallSystemFont, d,
		button.RenderState{Variant: button.FindBar, Surface: fill})
	return func(gtx layout.Context) layout.Dimensions {
		return layoutBar(gtx, c, fill, search, steps, done)
	}
}
