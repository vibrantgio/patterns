// Package card provides the Patterns Card pattern: a rounded surface
// container with optional Header / Body / Footer slots, in either an
// outlined or filled look.
//
// A card is raised, not floating: both looks fill at the raise walked from
// the surface the card stands on, and neither casts a shadow — a shadow
// marks what floats and can leave (a toast, a menu), which a card is not.
// The two looks differ only at the edge: outlined circles that fill with a
// 1 dp neutral stroke, filled carries none and is read by the fill alone —
// except where the scheme has no step left to tell the raise with, and the
// filled card draws the seam its raise owes instead.
//
// Card is a callable Go function consuming a components theme observable
// and returning a stream of layout.Widget. The source is intentionally
// short and free of opaque configuration — copy it into your own app and
// modify as needed.
//
// Card draws no text of its own, which is why Props carries no Shaper
// where its sibling patterns do: all three slots are caller-supplied
// layout.Widgets, so the typeface of anything inside a card is settled by
// whoever builds those widgets. Nil slots are dropped from the stack
// entirely, and the S3 gaps fall only between the slots that survive — a
// Body-only card is not padded as though the Header and Footer were there
// but empty.
//
// The card fills the constraints it is given rather than shrinking to its
// content — it reports gtx.Constraints.Max as its size — so a Card handed
// a full-height column takes the whole column. Constrain it yourself.
package card

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	pllayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/patterns/internal/outline"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Props configures a Card. All slot fields are optional; nil slots are
// simply omitted from the inner stack. Both looks fill at the raise walked
// from [Props.Level]; Filled drops the outlined look's 1 dp neutral stroke,
// which is the only difference between them.
type Props struct {
	Header layout.Widget
	Body   layout.Widget
	Footer layout.Widget

	// Filled selects the look read by its fill alone, without the
	// outline. Defaults to the outlined look.
	Filled bool

	// Level is the level of the surface the card stands on, and the card
	// fills one step above it. The zero value is the content, which is
	// where most cards stand; a card inside a dialog names Level2 and
	// comes out one step above the dialog rather than one step above a
	// content plane it is nowhere near.
	Level tokens.ElevationLevel
}

// Card returns an rx.Observable[layout.Widget] that emits a new widget
// whenever any consumed theme token changes. The widget fills its
// available constraints and renders a rounded Surface, with the three
// slots stacked vertically inside an S4 inset and separated by S3 gaps.
func Card(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest4(t.Color, t.Spacing, t.Radius, t.Elevation),
			func(n rx.Tuple4[tokens.ColorTokens, tokens.SpacingScale, tokens.RadiusScale, tokens.ElevationScale]) resolvedTokens {
				return resolvedTokens{color: n.First, spacing: n.Second, radius: n.Third, elevation: n.Fourth}
			},
		)
	})
	return rx.Map(resolved, func(tok resolvedTokens) layout.Widget {
		return Render(props, tok.color, tok.spacing, tok.radius)
	})
}

// Render produces a layout.Widget for a card with pre-resolved tokens.
// Intended for golden-image testing and static demonstrations; production
// code should use Card.
func Render(props Props, colors tokens.ColorTokens, sp tokens.SpacingScale, rad tokens.RadiusScale) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return drawCard(gtx, props, colors, sp, rad)
	}
}

type resolvedTokens struct {
	color   tokens.ColorTokens
	spacing tokens.SpacingScale
	radius  tokens.RadiusScale
	// elevation is snapshotted so a theme elevation change re-emits the
	// widget; the fills themselves resolve through SurfaceAt, which reads
	// the default tokens.Elevation scale.
	elevation tokens.ElevationScale
}

func drawCard(gtx layout.Context, props Props, colors tokens.ColorTokens, sp tokens.SpacingScale, rad tokens.RadiusScale) layout.Dimensions {
	size := gtx.Constraints.Max
	bounds := image.Rectangle{Max: size}
	r := gtx.Dp(unit.Dp(rad.Lg))
	gap := gtx.Dp(unit.Dp(sp.S3))

	// Both looks are raised by the same tonal step, walked from the surface
	// the card stands on, and neither casts a shadow.
	raise := colors.RaisedOn(colors.SurfaceAt(props.Level))

	rrect := clip.RRect{Rect: bounds, SE: r, SW: r, NE: r, NW: r}
	paint.FillShape(gtx.Ops, raise.Fill, rrect.Op(gtx.Ops))

	switch {
	case !props.Filled:
		// The outlined card's edge is what makes it an object, so it is
		// derived rather than named: the neutral step that reaches the
		// graphic contrast floor against the fill it circles. It is also
		// louder than any seam, so an outlined card owes none.
		paint.FillShape(gtx.Ops, outline.Ink(colors, raise.Fill), clip.Stroke{
			Path:  rrect.Path(gtx.Ops),
			Width: float32(gtx.Dp(unit.Dp(1))),
		}.Op())
	case raise.Seamed:
		// The scheme has no step left to say the card is raised, so the
		// raise says it at its own edge instead. Drawn once, by the card:
		// the surface beneath has nothing to draw it with.
		//
		// Drawn as the card's own rectangle in the seam with the raise laid
		// back over it one pixel in, rather than as a stroke: a stroke is
		// centred on the edge it follows, so half of it would land outside
		// the card and the card's painted footprint would depend on which
		// scheme was running.
		w := gtx.Dp(unit.Dp(1))
		if w < 1 {
			w = 1
		}
		paint.FillShape(gtx.Ops, raise.Seam, rrect.Op(gtx.Ops))
		ir := r - w
		if ir < 0 {
			ir = 0
		}
		inner := clip.RRect{Rect: bounds.Inset(w), SE: ir, SW: ir, NE: ir, NW: ir}
		paint.FillShape(gtx.Ops, raise.Fill, inner.Op(gtx.Ops))
	}

	layout.UniformInset(unit.Dp(sp.S4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return stack(gtx, gap, props.Header, props.Body, props.Footer)
	})

	return layout.Dimensions{Size: size}
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
