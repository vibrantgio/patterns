// Package tag provides the Patterns Tag pattern: the Full-radius pill chip
// the patterns draw — a label, not a control. It is the shared home of the
// chip that patterns/pricing ("Popular") and patterns/hero (the eyebrow)
// each used to draw locally, and it is where the chip's status vocabulary
// lives: real screens need status chips, and a vocabulary without one gets
// status invented for it (goal G-I2 — if the web surface wants it, the Gio
// vocabulary grows it first).
//
// Five variants:
//
//   - Filled (the default): the Primary pin under its on-colour —
//     pricing's "Popular" chip.
//   - Tonal: the primary-200 tinted fill under Primary text — hero's
//     eyebrow (ADR-007: ramp steps 100–300 are tinted fills).
//   - Success, Warning, Error: the status treatments. Each resolves its
//     level exactly as patterns/toast resolves the same level — the pinned
//     fixed-hue role (ColorTokens.Success/Warning/Error), blended 20% over
//     the Surface pin for the fill and drawn pure as a 1 dp outline, under
//     the Text pin — so status colour means one thing everywhere. The base
//     is Surface rather than the toast's level-2 fill because a chip rests
//     on the pane it labels; it does not float (ADR-005).
//
// Geometry is one chip for all five: S2/S1 padding, Full corner radius,
// sized to its label-small label, the SemiBold request resolving to the
// nearest registered face exactly as the pricing and hero call sites always
// requested it. A tag is a label, not a control: it has no interaction
// states, no density, and no hit target.
//
// The package follows the Phase 4 Composition contract: Tag is a callable
// Go function consuming a components theme observable, returning a stream of
// layout.Widget. The source is intentionally short and free of opaque
// configuration — copy it into your own app and modify as needed.
package tag

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Variant selects the chip's treatment.
type Variant int

const (
	// Filled is the default: the Primary pin under OnPrimary — the
	// pricing card's "Popular" chip.
	Filled Variant = iota
	// Tonal is the primary-200 tinted fill under Primary text — the
	// hero's eyebrow.
	Tonal
	// Success, Warning and Error are the status treatments: the level's
	// pinned role tinted 20% over Surface, ringed by the 1 dp level
	// outline, under the Text pin — the toast's level resolution at chip
	// scale.
	Success
	Warning
	Error
)

// Props configures a Tag.
type Props struct {
	// Label is the chip's text. An empty label still draws the pill at
	// its padding minimum, matching the historical eyebrow behaviour.
	Label string

	// Variant selects the treatment; the zero value is Filled.
	Variant Variant

	// Shaper is an explicit per-instance override of the text shaper. Leave
	// it nil in normal use: the tag then shapes its label with the theme's
	// shaper (Typography.Shaper()), which is built once for the process and
	// shared by every component reading that typography — the cache lives
	// behind the Typography value, so it survives the copy this component's
	// map function makes of it (spectrum F5.1). Set it only when this
	// instance must shape with a different shaper than the theme provides.
	//
	// A shaper is not safe to use from two goroutines; Gio lays the widget
	// forest out on the one goroutine that runs the event loop, which is
	// what makes sharing it correct. See theme/tokens.Typography.Shaper.
	Shaper *text.Shaper
}

type resolvedTokens struct {
	color   tokens.ColorTokens
	spacing tokens.SpacingScale
	radius  tokens.RadiusScale
	style   tokens.TextStyle // the LabelSmall role: typeface, weight, size, line height
	shaper  *text.Shaper     // the theme's shaper; nil in the Render path
}

// Tag returns an rx.Observable[layout.Widget] that emits a new widget
// whenever any consumed theme token changes. A tag holds no interaction
// state, so there is nothing to preserve across emissions.
func Tag(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	// Flatten the nested theme observables into a concrete snapshot. The
	// typography emission supplies the LabelSmall text style and the
	// theme's cached shaper (ADR-003: the theme owns the typeface). There
	// is no density: a tag is a label, not a control (E1.4).
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest4(t.Color, t.Spacing, t.Radius, t.Typography),
			func(n rx.Tuple4[tokens.ColorTokens, tokens.SpacingScale, tokens.RadiusScale, tokens.Typography]) resolvedTokens {
				typ := n.Fourth
				return resolvedTokens{
					color:   n.First,
					spacing: n.Second,
					radius:  n.Third,
					style:   typ.LabelSmall,
					shaper:  typ.Shaper(),
				}
			},
		)
	})

	return rx.Map(resolved, func(tok resolvedTokens) layout.Widget {
		// Props.Shaper is an explicit override; the theme's shaper is
		// the default.
		shaper := props.Shaper
		if shaper == nil {
			shaper = tok.shaper
		}
		return func(gtx layout.Context) layout.Dimensions {
			return draw(gtx, shaper, props.Label, props.Variant, tok)
		}
	})
}

// Render produces a layout.Widget for a tag with pre-resolved tokens.
// Intended for golden-image testing, static demonstrations, and the other
// patterns' own chips (pricing's "Popular", hero's eyebrow both draw
// through it); production code composing a screen should use Tag, which
// reads the parameters below off the theme.
//
// style is the LabelSmall role's whole text style — typeface, weight, size
// and line height all reach the shaper, exactly as they do on the live
// path. Pass tokens.DefaultTypography.LabelSmall for the default desktop
// look.
func Render(
	shaper *text.Shaper,
	label string,
	v Variant,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	style tokens.TextStyle,
) layout.Widget {
	tok := resolvedTokens{color: colors, spacing: sp, radius: rad, style: style}
	return func(gtx layout.Context) layout.Dimensions {
		return draw(gtx, shaper, label, v, tok)
	}
}

// draw paints one pill chip sized to its label: S2 horizontal and S1
// vertical padding around the label, Full corner radius, the variant's fill
// under the variant's text colour, and — for the status variants — the 1 dp
// level outline stroked on the pill's edge, exactly as the toast rings its
// surface. The padding minimums keep the pill visible when the label
// rasterises to zero width (e.g., in deterministic empty-label golden
// tests), matching the historical eyebrow behaviour.
func draw(
	gtx layout.Context,
	shaper *text.Shaper,
	label string,
	v Variant,
	tok resolvedTokens,
) layout.Dimensions {
	padH := gtx.Dp(unit.Dp(tok.spacing.S2))
	padV := gtx.Dp(unit.Dp(tok.spacing.S1))
	rad := gtx.Dp(unit.Dp(tok.radius.Full))

	fg, bg, outline, outlined := colors(v, tok.color)

	// Both historical call sites request SemiBold, which the pinned shaper
	// resolves to the Medium face (the nearest registered weight); a zero
	// style weight (a legacy size-only style) falls back the same way.
	mColor := op.Record(gtx.Ops)
	paint.ColorOp{Color: fg}.Add(gtx.Ops)
	material := mColor.Stop()

	labelGtx := gtx
	labelGtx.Constraints.Min = image.Point{}
	mLabel := op.Record(gtx.Ops)
	wl := typeset.Label(tok.style, 1)
	labelDims := typeset.Layout(labelGtx, shaper, wl, typeset.Font(tok.style, font.SemiBold), unit.Sp(tok.style.Size), label, material)
	labelCall := mLabel.Stop()

	w := labelDims.Size.X + 2*padH
	h := labelDims.Size.Y + 2*padV
	if minW := 2 * padH; w < minW {
		w = minW
	}
	if minH := 2 * padV; h < minH {
		h = minH
	}

	// components/layout.Pill's clamp, kept here because the status outline
	// needs the same rounded rect as a Path: clip.RRect does not clamp a
	// corner radius to the rect, so the Full sentinel (9999 dp) passed
	// straight in would spray paint across the canvas.
	if maxRad := min(w, h) / 2; rad > maxRad {
		rad = maxRad
	}
	rrect := clip.RRect{Rect: image.Rectangle{Max: image.Pt(w, h)}, SE: rad, SW: rad, NE: rad, NW: rad}
	paint.FillShape(gtx.Ops, bg, rrect.Op(gtx.Ops))
	if outlined {
		paint.FillShape(gtx.Ops, outline, clip.Stroke{
			Path:  rrect.Path(gtx.Ops),
			Width: float32(gtx.Dp(unit.Dp(1))),
		}.Op())
	}

	st := op.Offset(image.Pt(padH, padV)).Push(gtx.Ops)
	labelCall.Add(gtx.Ops)
	st.Pop()
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// colors resolves a variant to its text colour, fill, and — for the status
// variants — outline. The status levels read the same pinned fixed-hue
// roles patterns/toast's accentColor reads, and blend them over the ground
// with the same 20% tint its tintSurface applies, so a chip's success and a
// toast's success are one colour decision. All of it reads roles off the
// token set, so every variant flips with light/dark and follows whatever
// seed, palette or high-contrast variant the theme is emitting.
func colors(v Variant, c tokens.ColorTokens) (fg, bg, outline color.NRGBA, outlined bool) {
	switch v {
	case Tonal:
		return c.Primary, c.Ramps.Primary.Step(200), color.NRGBA{}, false
	case Success:
		return c.Text, tintSurface(c.Surface, c.Success), c.Success, true
	case Warning:
		return c.Text, tintSurface(c.Surface, c.Warning), c.Warning, true
	case Error:
		return c.Text, tintSurface(c.Surface, c.Error), c.Error, true
	default:
		return c.OnPrimary, c.Primary, color.NRGBA{}, false
	}
}

// tintSurface blends 20% of the accent over the given base — the exact
// blend patterns/toast applies to its level-2 fill, applied here to the
// Surface pin a resting chip sits on.
func tintSurface(base, accent color.NRGBA) color.NRGBA {
	return blend(base, accent, 0x33)
}

func blend(base, over color.NRGBA, alpha uint8) color.NRGBA {
	a := float32(alpha) / 255
	return color.NRGBA{
		R: uint8(float32(over.R)*a + float32(base.R)*(1-a)),
		G: uint8(float32(over.G)*a + float32(base.G)*(1-a)),
		B: uint8(float32(over.B)*a + float32(base.B)*(1-a)),
		A: 0xff,
	}
}
