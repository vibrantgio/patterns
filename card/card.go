// Package card provides the Patterns Card pattern: one rounded surface
// raised a step above the surface it stands on, with optional Header /
// Body / Footer slots.
//
// A card singles something out, and the raise is the whole of how it does
// it: no hairline of its own, never outlined, never wearing a role. What
// the developer wants to say about a card is a badge in its header. Where
// the scheme has no lighter step left, the raise is told by a seam at the
// card's edge instead of by its fill — that is the elevation's rule for
// every raise, not an outline.
//
// A card is raised, not floating, so it casts no shadow: a shadow marks
// what floats and can leave (a toast, a menu), which a card is not. It
// holds content, never another card; grouping inside a card is the card's
// own structure.
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
	"gioui.org/unit"

	"github.com/reactivego/rx"
	pllayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/patterns/internal/surface"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Props configures a Card. All slot fields are optional; nil slots are
// simply omitted from the inner stack.
type Props struct {
	Header layout.Widget
	Body   layout.Widget
	Footer layout.Widget

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

	// The card is raised by one tonal step walked from the surface it
	// stands on, and casts no shadow. Where the scheme has no step left,
	// the raise is told by the seam at its own edge instead — that is the
	// elevation's rule for every raise, not an outline.
	surface.Card(gtx, bounds, r, colors.RaisedOn(colors.SurfaceAt(props.Level)))

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
