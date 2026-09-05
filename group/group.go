// Package group provides the Patterns Group pattern: a hairline drawn
// around related components so the eye chunks them, optionally labelled.
//
// A group divides the page. It is drawn at the level of the surface it is
// in and takes that surface's own fill — it paints nothing inside itself,
// only the line at its edge — so it raises nothing and nothing is derived
// against it: what a group holds stands on the surface the group is in, at
// that surface's own level. The hairline is the seam of two regions that
// share one fill, derived to be findable against that fill in either
// scheme (tokens.ColorTokens.SeamOn), which is the quiet line the platform
// draws and not the 3:1 mark a graphic carrying meaning owes.
//
// A group wears no role. It is not operated, so it has no emphasis to
// speak with, and a role-coloured hairline would borrow the accent's
// grammar for something the user never chose.
//
// A group may hold a card; it never holds another group. Which of the two
// a developer reaches for answers one question: am I dividing the page, or
// singling something out?
//
// Group is a callable Go function consuming a components theme observable
// and returning a stream of layout.Widget. The source is intentionally
// short and free of opaque configuration — copy it into your own app and
// modify as needed.
//
// Apart from its own label a group draws no text: the Content slots are
// caller-supplied layout.Widgets, so the typeface of anything inside a
// group is settled by whoever builds them. Nil slots are dropped from the
// stack entirely, and the S3 gaps fall only between the slots that
// survive.
//
// The group fills the constraints it is given rather than shrinking to its
// content — it reports gtx.Constraints.Max as its size — so a Group handed
// a full-height column takes the whole column. Constrain it yourself.
package group

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	pllayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/patterns/internal/surface"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Props configures a Group.
type Props struct {
	// Label names what the group holds. Empty leaves the group unlabelled
	// and its hairline unbroken.
	//
	// It is drawn top-leading, inside the hairline, as the first row of
	// the group's own stack — the platform's current idiom for a section
	// header over a bordered container. A label cut into the top line is
	// the fieldset legend's idiom, which has no native counterpart here
	// and does not survive a Lg corner radius.
	Label string

	// Content is what the group holds, stacked top to bottom under the
	// label with S3 gaps. Nil entries are dropped.
	Content []layout.Widget

	// Shaper is an explicit per-instance override of the text shaper.
	// Leave it nil in normal use: the group then shapes its label with the
	// theme's shaper, which is built once for the process and shared by
	// every component reading that typography.
	//
	// A shaper is not safe to use from two goroutines; Gio lays the widget
	// forest out on the one goroutine that runs the event loop, which is
	// what makes sharing it correct.
	Shaper *text.Shaper

	// Level is the level of the surface the group is in. The group draws
	// at that level rather than above it: the zero value is the content,
	// which is where most groups are, and a group inside a dialog names
	// Level2 so its hairline is derived against the dialog's fill rather
	// than against a content plane it is nowhere near.
	Level tokens.ElevationLevel
}

type resolvedTokens struct {
	color   tokens.ColorTokens
	spacing tokens.SpacingScale
	radius  tokens.RadiusScale
	label   tokens.TextStyle // the LabelLarge role: typeface, weight, size, line height
	shaper  *text.Shaper     // the theme's shaper; nil in the Render path
}

// Group returns an rx.Observable[layout.Widget] that emits a new widget
// whenever any consumed theme token changes.
func Group(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest4(t.Color, t.Spacing, t.Radius, t.Typography),
			func(n rx.Tuple4[tokens.ColorTokens, tokens.SpacingScale, tokens.RadiusScale, tokens.Typography]) resolvedTokens {
				typ := n.Fourth
				return resolvedTokens{
					color:   n.First,
					spacing: n.Second,
					radius:  n.Third,
					label:   typ.LabelLarge,
					shaper:  typ.Shaper(),
				}
			},
		)
	})
	return rx.Map(resolved, func(tok resolvedTokens) layout.Widget {
		shaper := props.Shaper
		if shaper == nil {
			shaper = tok.shaper
		}
		return Render(shaper, props, tok.color, tok.spacing, tok.radius, tok.label)
	})
}

// Render produces a layout.Widget for a group with pre-resolved tokens.
// Intended for golden-image testing and static demonstrations; production
// code should use Group.
//
// label is the LabelLarge role's whole text style — typeface, weight, size
// and line height all reach the shaper, exactly as they do on the live
// path. A group with no Label never asks the shaper anything, so a nil
// shaper is only an error for a labelled one.
func Render(
	shaper *text.Shaper,
	props Props,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	label tokens.TextStyle,
) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return draw(gtx, shaper, props, colors, sp, rad, label)
	}
}

func draw(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	label tokens.TextStyle,
) layout.Dimensions {
	size := gtx.Constraints.Max
	r := gtx.Dp(unit.Dp(rad.Lg))
	gap := gtx.Dp(unit.Dp(sp.S3))

	// The hairline parts two regions that share one fill: the surface
	// inside the group and the same surface outside it.
	surface.Group(gtx, image.Rectangle{Max: size}, r, colors.SeamOn(colors.SurfaceAt(props.Level)))

	layout.UniformInset(unit.Dp(sp.S4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		ws := make([]layout.Widget, 0, len(props.Content)+1)
		if props.Label != "" {
			ws = append(ws, labelWidget(shaper, props.Label, label, colors))
		}
		ws = append(ws, props.Content...)
		return stack(gtx, gap, ws...)
	})

	return layout.Dimensions{Size: size}
}

// labelWidget draws the group's own label: the LabelLarge role in the
// neutral ramp's low-contrast step, which is the step every quiet label in
// this system is set in. It is not the accent — a group wears no role — and
// not the Text pin, which would give a section header the weight of the
// content it names.
func labelWidget(shaper *text.Shaper, s string, style tokens.TextStyle, colors tokens.ColorTokens) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		paint.ColorOp{Color: colors.Ramps.Neutral.Step(700)}.Add(gtx.Ops)
		material := m.Stop()

		f := typeset.Font(style, font.Normal)
		lbl := typeset.Label(style, 1)
		// Min is dropped so the label reports the text it drew rather than
		// the group's own minimum, which is what keeps the S3 gap under it
		// the gap and not the rest of the column.
		gtx.Constraints.Min = image.Point{}
		return typeset.Layout(gtx, shaper, lbl, f, unit.Sp(style.Size), s, material)
	}
}

// stack lays out the non-nil children top-to-bottom with a gap of gapPx
// pixels between adjacent children. Nil children are skipped.
func stack(gtx layout.Context, gapPx int, children ...layout.Widget) layout.Dimensions {
	ws := make([]layout.Widget, 0, len(children)*2)
	for _, c := range children {
		if c == nil {
			continue
		}
		if len(ws) > 0 {
			ws = append(ws, gapWidget(gapPx))
		}
		ws = append(ws, c)
	}
	if len(ws) == 0 {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	return pllayout.Col(gtx, ws...)
}

// gapWidget reserves gapPx vertical pixels and zero horizontal space.
func gapWidget(gapPx int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(0, gapPx)}
	}
}
