// Package alert provides the Patterns Alert pattern: a tinted-Surface
// rounded banner with a leading variant icon, a Title, and an arbitrary
// Body widget. Variants are Info, Success, Warning, and Error.
//
// Alert is a callable Go function consuming a components theme observable,
// returning a stream of layout.Widget. Source is intentionally short and
// free of opaque configuration — copy it into your own app and modify as
// needed.
//
// One thing about the variants is worth knowing before you theme them: all
// four draw the same right-pointing chevron glyph, differing only in
// colour; the per-variant icon set arrives with components/icon.
//
// Colour: each variant is a status role, and the banner is that role's
// tonal container with the role's own mark on it — StatusContainer and
// OnStatusContainer, both realized at a tone by the theme rather than mixed
// here. Info is the info role, not the accent: an informational banner that
// wore the brand said whatever the brand happened to say, and under a
// red-heavy brand it said "error" louder than the error variant did.
//
// The banner fills the constraints it is given rather than shrinking to
// its content — it reports gtx.Constraints.Max as its size — so an Alert
// handed a full-height column takes the whole column. Give it a
// height-constrained slot.
package alert

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	pllayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Variant selects the alert's semantic palette.
type Variant int

const (
	Info Variant = iota
	Success
	Warning
	Error
)

// Props configures an Alert. Title may be empty (the title row is omitted);
// Body may be nil (only the icon and title render).
type Props struct {
	Variant Variant
	Title   string
	Body    layout.Widget

	// Shaper is an explicit per-instance override of the text shaper. Leave
	// it nil in normal use: the alert then shapes its title with the theme's
	// shaper (Typography.Shaper()), which is built once for the process and
	// shared by every component reading that typography — the cache lives
	// behind the Typography value, so it survives the copy this component's
	// map function makes of it. Set it only when this instance must shape
	// with a different shaper than the theme provides.
	//
	// A shaper is not safe to use from two goroutines; Gio lays the widget
	// forest out on the one goroutine that runs the event loop, which is
	// what makes sharing it correct. See theme/tokens.Typography.Shaper.
	Shaper *text.Shaper
}

// Alert returns an rx.Observable[layout.Widget] that emits a new widget
// whenever any consumed theme token changes.
func Alert(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	// Flatten the nested theme observables into a concrete snapshot. The
	// typography emission supplies both the TitleMedium text style and the
	// theme's cached shaper: the theme owns the typeface.
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest4(t.Color, t.Spacing, t.Radius, t.Typography),
			func(n rx.Tuple4[tokens.ColorTokens, tokens.SpacingScale, tokens.RadiusScale, tokens.Typography]) resolvedTokens {
				typ := n.Fourth
				return resolvedTokens{
					color:   n.First,
					spacing: n.Second,
					radius:  n.Third,
					title:   typ.TitleMedium,
					shaper:  typ.Shaper(),
				}
			},
		)
	})
	return rx.Defer(func() rx.Observable[layout.Widget] {
		return rx.Map(resolved, func(tok resolvedTokens) layout.Widget {
			// Props.Shaper is an explicit override; the theme's shaper is
			// the default.
			shaper := props.Shaper
			if shaper == nil {
				shaper = tok.shaper
			}
			return func(gtx layout.Context) layout.Dimensions {
				return drawAlert(gtx, shaper, props, tok.color, tok.spacing, tok.radius, tok.title)
			}
		})
	})
}

// Render produces a layout.Widget for an alert with pre-resolved tokens.
// Intended for golden-image testing and static demonstrations; production
// code should use Alert, which reads the shaper and the same text style
// off the theme.
//
// title is the TitleMedium role's whole text style — typeface, weight,
// size and line height all reach the shaper, exactly as they do on the
// live path. Pass tokens.DefaultTypography.TitleMedium for the default
// desktop look. There is no density parameter: an alert is a surface
// sized by its content, not a control.
func Render(
	shaper *text.Shaper,
	props Props,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	title tokens.TextStyle,
) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return drawAlert(gtx, shaper, props, colors, sp, rad, title)
	}
}

type resolvedTokens struct {
	color   tokens.ColorTokens
	spacing tokens.SpacingScale
	radius  tokens.RadiusScale
	title   tokens.TextStyle // the TitleMedium role: typeface, weight, size, line height
	shaper  *text.Shaper     // the theme's shaper; nil in the Render path
}

const iconDp = 20

func drawAlert(gtx layout.Context, shaper *text.Shaper, props Props, colors tokens.ColorTokens, sp tokens.SpacingScale, rad tokens.RadiusScale, title tokens.TextStyle) layout.Dimensions {
	size := gtx.Constraints.Max
	r := gtx.Dp(unit.Dp(rad.Lg))

	role := roleOf(props.Variant)
	accent := colors.OnStatusContainer(role)
	bg := colors.StatusContainer(role)

	rrect := clip.RRect{Rect: image.Rectangle{Max: size}, SE: r, SW: r, NE: r, NW: r}
	paint.FillShape(gtx.Ops, bg, rrect.Op(gtx.Ops))

	layout.UniformInset(unit.Dp(sp.S4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
			layout.Rigid(iconWidget(iconDp, accent)),
			layout.Rigid(pllayout.HSpacer(sp.S3)),
			layout.Flexed(1, contentColumn(shaper, props, colors, sp, title)),
		)
	})

	return layout.Dimensions{Size: size}
}

// iconWidget renders the variant icon — a right-pointing filled chevron —
// into a fixed sizeDp square. All variants share this chevron shape and
// differentiate by colour only.
func iconWidget(sizeDp float32, col color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		sz := gtx.Dp(unit.Dp(sizeDp))
		drawChevron(gtx, sz/2, sz/2, sz, col)
		return layout.Dimensions{Size: image.Pt(sz, sz)}
	}
}

func contentColumn(shaper *text.Shaper, props Props, colors tokens.ColorTokens, sp tokens.SpacingScale, title tokens.TextStyle) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		var ws []layout.Widget
		if props.Title != "" {
			ws = append(ws, titleWidget(shaper, props.Title, colors.Text, title))
		}
		if props.Body != nil {
			if len(ws) > 0 {
				ws = append(ws, pllayout.VSpacer(sp.S1))
			}
			ws = append(ws, props.Body)
		}
		if len(ws) == 0 {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
		}
		return pllayout.Col(gtx, ws...)
	}
}

func titleWidget(shaper *text.Shaper, label string, fg color.NRGBA, style tokens.TextStyle) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		mColor := op.Record(gtx.Ops)
		paint.ColorOp{Color: fg}.Add(gtx.Ops)
		material := mColor.Stop()
		// Shape with the TitleMedium role's typeface, weight, size and
		// line height. A zero weight in style falls back to SemiBold, so
		// the title keeps its emphasis against the body even when style
		// carries only a size.
		f := typeset.Font(style, font.SemiBold)
		wl := typeset.Label(style, 1)
		return typeset.Layout(gtx, shaper, wl, f, unit.Sp(style.Size), label, material)
	}
}

func drawChevron(gtx layout.Context, cx, cy, sz int, col color.NRGBA) {
	half := float32(sz) / 2
	quarter := float32(sz) / 4
	fcx := float32(cx)
	fcy := float32(cy)

	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(fcx-quarter, fcy-half))
	p.LineTo(f32.Pt(fcx+quarter, fcy))
	p.LineTo(f32.Pt(fcx-quarter, fcy+half))
	p.Close()
	paint.FillShape(gtx.Ops, col, clip.Outline{Path: p.End()}.Op())
}

// roleOf maps Variant to its status role in the token set. All four are
// status roles — info included — so all four flip with light/dark and
// follow whatever seed, palette or high-contrast variant the theme is
// emitting, and none of them wears the accent.
func roleOf(v Variant) tokens.Role {
	switch v {
	case Error:
		return tokens.RoleError
	case Success:
		return tokens.RoleSuccess
	case Warning:
		return tokens.RoleWarning
	default:
		return tokens.RoleInfo
	}
}
