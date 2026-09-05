// Package pricing provides the Patterns Pricing pattern: a row of tier
// groups with one tier optionally recommended, suitable for a marketing
// landing or onboarding screen.
//
// A row of tiers is the page dividing itself, so a tier is a group: a
// hairline at the content's own level around what the tier holds, taking
// the content's fill. The one tier that must stand apart from the row is
// the recommended one, and that is a card: raised one step on the content,
// wearing the "Popular" badge in its header. The recommendation is the
// raise's work and the badge's word — never a role-coloured edge, which a
// group does not wear and a card is never given.
//
// Pricing is a callable Go function consuming a components theme observable,
// returning a stream of layout.Widget. The source is intentionally short and free of
// opaque configuration — copy it into your own app and modify as needed.
//
// Layout: each Tier renders with an S5 inset. Tiers sit in an equal-width,
// equal-height horizontal row separated by an S4 gutter. A first pass
// measures the tallest tier; a second pass stretches every tier to that
// height. Name, price and bullets stay at the top; the CTA sits on the
// bottom inset, not flush under the last bullet. Each tier contains — top
// to bottom — the tier name in title typography (the recommended tier puts
// its badge on that same row, trailing), a price / cadence pair in display
// typography with the cadence muted, a vertical feature list with a leading
// checkmark glyph rendered from a clip.Path, and a footer CTA button
// reusing components/button's filled visual.
//
// No responsive breakpoint to stack tiers vertically is provided —
// adopting this pattern at narrow widths is left to the caller.
package pricing

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/badge"
	"github.com/vibrantgio/components/button"
	pllayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/patterns/internal/surface"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// CTA describes a per-tier call-to-action. Label populates the button
// label and seeds the accessibility name; OnClick fires on activation.
type CTA struct {
	Label   string
	OnClick func(gtx layout.Context)
}

// Tier describes a single pricing card.
type Tier struct {
	// Name is the tier label rendered in title typography.
	Name string

	// Price is the prominent monetary string (e.g., "$29").
	Price string

	// Cadence is the muted suffix following Price (e.g., "/mo").
	Cadence string

	// Features is the vertical bullet list rendered under the price row.
	// Each entry gets a leading checkmark glyph.
	Features []string

	// CTA is the footer call-to-action button. May be nil to omit.
	CTA *CTA

	// Recommended marks the one tier that stands apart from the row: it is
	// drawn as a card, raised a step on the content the other tiers divide,
	// and wears a "Popular" badge on the name row, trailing.
	Recommended bool
}

// Props configures a Pricing row.
type Props struct {
	// Tiers is the ordered list of tier cards. Length 0 renders an empty
	// row; length 1 renders a single full-width card.
	Tiers []Tier

	// Shaper is an explicit per-instance override of the text shaper. Leave
	// it nil in normal use: the pricing row then shapes its tier text with
	// the theme's shaper (Typography.Shaper()), which is built once for the
	// process and shared by every component reading that typography — the
	// cache lives behind the Typography value, so it survives the copy this
	// component's map function makes of it. Set it only when this instance
	// must shape with a different shaper than the theme provides.
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
	popular tokens.TextStyle // the badge role the "Popular" label is set in, at the density
	name    tokens.TextStyle // the TitleLarge role (tier name)
	price   tokens.TextStyle // the DisplaySmall role (price)
	body    tokens.TextStyle // the BodyMedium role (cadence suffix, features)
	label   tokens.TextStyle // the LabelLarge role (CTA label)
	density tokens.Density   // CTA control height and inner padding
	shaper  *text.Shaper     // the theme's shaper; nil in the Render path
}

// Pricing returns an rx.Observable[layout.Widget] that emits a new
// widget whenever any consumed theme token changes. CTA click state
// survives across emissions: one widget.Clickable per tier is allocated
// once per subscription inside the rx.Defer scope.
func Pricing(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	// Flatten the nested theme observables into a concrete snapshot. The
	// typography emission supplies the badge/TitleLarge/DisplaySmall/
	// BodyMedium/LabelLarge text styles and the theme's cached shaper — the
	// theme owns the typeface; the density sizes the CTA and picks the
	// badge's own role.
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest5(t.Color, t.Spacing, t.Radius, t.Typography, t.Density),
			func(n rx.Tuple5[tokens.ColorTokens, tokens.SpacingScale, tokens.RadiusScale, tokens.Typography, tokens.Density]) resolvedTokens {
				typ := n.Fourth
				return resolvedTokens{
					color:   n.First,
					spacing: n.Second,
					radius:  n.Third,
					popular: badge.Style(typ, n.Fifth),
					name:    typ.TitleLarge,
					price:   typ.DisplaySmall,
					body:    typ.BodyMedium,
					label:   typ.LabelLarge,
					density: n.Fifth,
					shaper:  typ.Shaper(),
				}
			},
		)
	})

	return rx.Defer(func() rx.Observable[layout.Widget] {
		clicks := make([]widget.Clickable, len(props.Tiers))

		return rx.Map(resolved, func(tok resolvedTokens) layout.Widget {
			// Props.Shaper is an explicit override; the theme's shaper is
			// the default.
			shaper := props.Shaper
			if shaper == nil {
				shaper = tok.shaper
			}
			return func(gtx layout.Context) layout.Dimensions {
				for i := range props.Tiers {
					tier := &props.Tiers[i]
					if tier.CTA != nil && tier.CTA.OnClick != nil && clicks[i].Clicked(gtx) {
						tier.CTA.OnClick(gtx)
					}
				}
				return drawPricing(gtx, shaper, props, tok, clicks)
			}
		})
	})
}

// Render produces a layout.Widget for a pricing row with pre-resolved
// tokens. Intended for golden-image testing and static demonstrations;
// production code should use Pricing, which reads both of the parameters
// below off the theme. No event work is performed: the CTAs render as
// inert visuals.
//
// typo supplies the five roles the row draws — the badge role at d for the
// "Popular" mark, TitleLarge for the tier name, DisplaySmall for the
// price, BodyMedium for the cadence suffix and feature lines, LabelLarge
// for the CTA label — whole, so typeface, weight and line height reach
// the shaper exactly as they do on the live path. A pattern that spends
// more than one role takes the whole tokens.Typography rather than a
// role's tokens.TextStyle each: the roles it picks stay its own business,
// as they are on the live path. d is the density the CTA button draws at.
// Pass tokens.DefaultTypography and tokens.Comfortable for the default
// desktop look.
func Render(
	shaper *text.Shaper,
	props Props,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	typo tokens.Typography,
	d tokens.Density,
) layout.Widget {
	tok := resolvedTokens{
		color:   colors,
		spacing: sp,
		radius:  rad,
		popular: badge.Style(typo, d),
		name:    typo.TitleLarge,
		price:   typo.DisplaySmall,
		body:    typo.BodyMedium,
		label:   typo.LabelLarge,
		density: d,
	}
	return func(gtx layout.Context) layout.Dimensions {
		return drawPricing(gtx, shaper, props, tok, nil)
	}
}

func drawPricing(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	tok resolvedTokens,
	clicks []widget.Clickable,
) layout.Dimensions {
	if len(props.Tiers) == 0 {
		return layout.Dimensions{}
	}
	// Measure each card's natural height, then draw again with every
	// card's Min.Y set to that max so they share a floor. The Flex
	// row's own Size.Y is Constrained to the incoming Min and cannot
	// be the source of that max — an Exact canvas would stretch every
	// card to the window.
	rec := op.Record(gtx.Ops)
	_, maxH := layoutTiers(gtx, shaper, props, tok, clicks, 0)
	rec.Stop()
	dims, _ := layoutTiers(gtx, shaper, props, tok, clicks, maxH)
	return dims
}

func layoutTiers(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	tok resolvedTokens,
	clicks []widget.Clickable,
	minCardH int,
) (layout.Dimensions, int) {
	gap := pllayout.HSpacer(tok.spacing.S4)
	children := make([]layout.FlexChild, 0, 2*len(props.Tiers)-1)
	maxH := 0
	for i := range props.Tiers {
		if i > 0 {
			children = append(children, layout.Rigid(gap))
		}
		i := i
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			// Flex forwards the parent's cross-axis Min. An Exact
			// canvas would stretch every card to the window; drop
			// that Min, then apply the shared height we measured.
			if minCardH > 0 {
				gtx.Constraints.Min.Y = minCardH
				if gtx.Constraints.Max.Y < minCardH {
					gtx.Constraints.Max.Y = minCardH
				}
			} else {
				gtx.Constraints.Min.Y = 0
			}
			var click *widget.Clickable
			if i < len(clicks) {
				click = &clicks[i]
			}
			d := drawTier(gtx, shaper, props.Tiers[i], tok, click)
			if d.Size.Y > maxH {
				maxH = d.Size.Y
			}
			return d
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx, children...), maxH
}

// checkInk is the primary ink a tier draws its feature checkmarks in: the
// primary pin when it clears the graphic floor against the surface the
// checkmark is drawn on, and otherwise the step of the primary ramp that
// does ([tokens.ColorTokens.InkOn]). A checkmark carries meaning by itself,
// so it owes the graphic floor and derives against the tier's own fill
// rather than against the page — the two are different surfaces for the
// recommended tier, which is raised.
func checkInk(c tokens.ColorTokens, fill color.NRGBA) color.NRGBA {
	return c.InkOn(tokens.RolePrimary, fill, tokens.GraphicFloor)
}

// tierLevel is the level a tier's own content stands at: the recommended
// tier is a card, so what it holds stands one step above the content;
// every other tier is a group, which raises nothing.
func tierLevel(tier Tier) tokens.ElevationLevel {
	if tier.Recommended {
		return tokens.Level1
	}
	return tokens.Level0
}

// tierFill is the surface a tier's content is read against: the raise
// walked from the content for the recommended card, and the content's own
// fill for a group, which takes the surface it is in. Every derivation a
// tier makes asks here, so what a tier paints and what is read on it cannot
// drift apart.
func tierFill(c tokens.ColorTokens, tier Tier) color.NRGBA {
	if tier.Recommended {
		return c.RaisedOn(c.SurfaceAt(tokens.Level0)).Fill
	}
	return c.SurfaceAt(tokens.Level0)
}

// drawTier draws a single tier to its allocated width, with content height
// matching the inner stack plus S5 padding on all sides: a group's hairline
// for an ordinary tier, a card's raise for the recommended one.
func drawTier(
	gtx layout.Context,
	shaper *text.Shaper,
	tier Tier,
	tok resolvedTokens,
	click *widget.Clickable,
) layout.Dimensions {
	pad := gtx.Dp(unit.Dp(tok.spacing.S5))
	width := gtx.Constraints.Max.X
	minH := gtx.Constraints.Min.Y

	inner := gtx
	inner.Constraints.Min = image.Point{}
	inner.Constraints.Max.X = max(0, width-2*pad)
	if minH > 2*pad {
		innerH := minH - 2*pad
		inner.Constraints.Min.Y = innerH
		inner.Constraints.Max.Y = innerH
	} else {
		inner.Constraints.Max.Y = math.MaxInt32
	}

	macro := op.Record(gtx.Ops)
	innerDims := drawTierContent(inner, shaper, tier, tok, click)
	contentCall := macro.Stop()

	height := innerDims.Size.Y + 2*pad
	if height < minH {
		height = minH
	}
	r := gtx.Dp(unit.Dp(tok.radius.Lg))
	bounds := image.Rectangle{Max: image.Pt(width, height)}
	if tier.Recommended {
		surface.Card(gtx, bounds, r, tok.color.RaisedOn(tok.color.SurfaceAt(tokens.Level0)))
	} else {
		surface.Group(gtx, bounds, r, tok.color.SeamOn(tok.color.SurfaceAt(tokens.Level0)))
	}

	off := op.Offset(image.Pt(pad, pad)).Push(gtx.Ops)
	contentCall.Add(gtx.Ops)
	off.Pop()

	return layout.Dimensions{Size: image.Pt(width, height)}
}

// drawTierContent stacks name, price and bullets at the top. When the
// card is stretched to the row's shared height the CTA sits on the
// bottom inset; the S3 gap above it is the minimum, not the only, space.
func drawTierContent(
	gtx layout.Context,
	shaper *text.Shaper,
	tier Tier,
	tok resolvedTokens,
	click *widget.Clickable,
) layout.Dimensions {
	var top []layout.Widget
	top = append(top, nameRowWidget(shaper, tier, tok))
	top = append(top, priceRowWidget(shaper, tier.Price, tier.Cadence, tok))
	for _, f := range tier.Features {
		top = append(top, featureRowWidget(shaper, f, tier, tok))
	}

	gap := tok.spacing.S3
	if tier.CTA != nil {
		top = append(top, ctaWidget(shaper, tier.CTA, tok, click))
	}
	if tier.CTA == nil || gtx.Constraints.Min.Y <= 0 {
		return spacedCol(gtx, top, gap)
	}
	// Stretching: last item is the CTA. Keep it on the floor; the
	// items above stay at the top. The S3 above the CTA is the
	// minimum gap, already in spacedCol's last slot — pull the CTA
	// out and put the extra space above that gap.
	head, cta := top[:len(top)-1], top[len(top)-1]
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return spacedCol(gtx, head, gap)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)}
		}),
		layout.Rigid(pllayout.VSpacer(gap)),
		layout.Rigid(cta),
	)
}

func spacedCol(gtx layout.Context, ws []layout.Widget, gap float32) layout.Dimensions {
	if len(ws) == 0 {
		return layout.Dimensions{}
	}
	spaced := make([]layout.Widget, 0, 2*len(ws)-1)
	for i, w := range ws {
		if i > 0 {
			spaced = append(spaced, pllayout.VSpacer(gap))
		}
		spaced = append(spaced, w)
	}
	return pllayout.Col(gtx, spaced...)
}

// nameRowWidget is the tier's first row: the tier name leading. On the
// recommended tier the Popular badge sits on the same line, trailing at the
// inset's right edge.
//
// Aligned on the baseline rather than the middle. The two are set in
// different roles — TitleLarge and the badge's label role — so their boxes
// are different heights around different faces, and centring the boxes puts
// the two runs of type on two lines that are three pixels apart. Baseline
// alignment is what "on the same line" means for text, and the badge reports
// its label's baseline so that it can be asked.
func nameRowWidget(shaper *text.Shaper, tier Tier, tok resolvedTokens) layout.Widget {
	name := tierNameWidget(shaper, tier.Name, tok)
	if !tier.Recommended {
		return name
	}
	mark := popularBadgeWidget(shaper, tier, tok)
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(name),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
			}),
			layout.Rigid(pllayout.HSpacer(tok.spacing.S2)),
			layout.Rigid(mark),
		)
	}
}

// popularBadgeWidget renders "Popular" as a Neutral badge: the developer's
// word about the tier, which is the only word a card carries. Neutral
// rather than a role, because the card's own raise is what recommends the
// tier — a badge shouting Success on top of it would say it twice, in a
// vocabulary that means something else.
func popularBadgeWidget(shaper *text.Shaper, tier Tier, tok resolvedTokens) layout.Widget {
	// The badge's fill is derived against the surface it stands on rather
	// than against the page: on the recommended card that surface is the
	// raise, not the content.
	return badge.Render(shaper, "Popular", nil, badge.Neutral, tok.color, tok.spacing,
		tok.radius, tok.popular, badge.RenderState{Level: tierLevel(tier)})
}

// tierNameWidget renders the tier name in the TitleLarge role in
// Text. A zero style weight (the legacy Render path synthesizes
// size-only styles) falls back to SemiBold.
func tierNameWidget(shaper *text.Shaper, label string, tok resolvedTokens) layout.Widget {
	return textWidget(shaper, label, tok.color.Text, tok.name, font.SemiBold)
}

// priceRowWidget renders the price (DisplaySmall Text) followed by
// the muted cadence (BodyMedium neutral 700) in a horizontal row
// with an S1 gap. Cross-axis Alignment.End approximates baseline
// alignment for the prominent price next to its smaller cadence suffix.
func priceRowWidget(shaper *text.Shaper, price, cadence string, tok resolvedTokens) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		priceW := textWidget(shaper, price, tok.color.Text, tok.price, font.SemiBold)
		cadenceW := textWidget(shaper, cadence, tok.color.Ramps.Neutral.Step(700), tok.body, font.Normal)
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
			layout.Rigid(priceW),
			layout.Rigid(pllayout.HSpacer(tok.spacing.S1)),
			layout.Rigid(cadenceW),
		)
	}
}

// featureRowWidget renders a single feature bullet: a Primary checkmark
// glyph followed by the feature label in BodyMedium Text, joined
// by an S2 gap and centered vertically.
func featureRowWidget(shaper *text.Shaper, label string, tier Tier, tok resolvedTokens) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(checkmarkWidget(tier, tok)),
			layout.Rigid(pllayout.HSpacer(tok.spacing.S2)),
			layout.Rigid(textWidget(shaper, label, tok.color.Text, tok.body, font.Normal)),
		)
	}
}

// checkmarkWidget paints a small check ("✓") inside an S4 box using a
// clip.Path, in [checkInk] against the tier's own fill. The path is a
// two-segment polyline traced over the box; the stroke width is 2 dp.
func checkmarkWidget(tier Tier, tok resolvedTokens) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		box := gtx.Dp(unit.Dp(tok.spacing.S4))
		stroke := float32(gtx.Dp(unit.Dp(2)))
		s := float32(box)

		var path clip.Path
		path.Begin(gtx.Ops)
		path.MoveTo(f32.Pt(s*0.2, s*0.55))
		path.LineTo(f32.Pt(s*0.45, s*0.8))
		path.LineTo(f32.Pt(s*0.8, s*0.25))
		paint.FillShape(gtx.Ops, checkInk(tok.color, tierFill(tok.color, tier)), clip.Stroke{
			Path:  path.End(),
			Width: stroke,
		}.Op())
		return layout.Dimensions{Size: image.Pt(box, box)}
	}
}

// ctaWidget renders the per-tier CTA as a components/button filled visual,
// wrapped in widget.Clickable when a click target is provided. The
// button fills the card's inner width (components/button's intrinsic
// "fill Max.X" sizing), giving the typical full-width pricing CTA.
func ctaWidget(shaper *text.Shaper, cta *CTA, tok resolvedTokens, click *widget.Clickable) layout.Widget {
	rendered := button.Render(shaper, cta.Label, tok.color, tok.spacing, tok.radius, tok.label, tok.density, button.RenderState{})
	return func(gtx layout.Context) layout.Dimensions {
		if click == nil {
			return rendered(gtx)
		}
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp(cta.Label).Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			return rendered(gtx)
		})
	}
}

// textWidget renders a single-line label in the supplied colour and text
// style, through theme/typeset so the role's line height is the height
// of the line box. Empty labels collapse to zero dimensions so adjacent
// section gaps are the only vertical contribution. A zero style weight
// (the legacy Render path synthesizes size-only styles) falls back to
// fallbackWeight.
func textWidget(shaper *text.Shaper, label string, fg color.NRGBA, style tokens.TextStyle, fallbackWeight font.Weight) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if label == "" {
			return layout.Dimensions{}
		}
		mColor := op.Record(gtx.Ops)
		paint.ColorOp{Color: fg}.Add(gtx.Ops)
		material := mColor.Stop()
		wl := typeset.Label(style, 1)
		return typeset.Layout(gtx, shaper, wl, typeset.Font(style, fallbackWeight), unit.Sp(style.Size), label, material)
	}
}
