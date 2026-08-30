// Package tag provides the Patterns Tag pattern: a Full-radius pill chip
// carrying a label and, optionally, one dismiss affordance. It is also
// where the chip's status vocabulary lives.
//
// Five variants:
//
//   - Filled (the default): the Primary pin under its on-colour.
//   - Tonal: the primary-200 tinted fill under Primary text, ringed in
//     the Primary pin.
//   - Success, Warning, Error: the status treatments. Each is its role's
//     tonal container — the role's own hue realized at one measured chroma
//     and depth by the theme, never mixed here — under the Text pin. The
//     container is resolved against the Surface pin because a chip rests
//     on the pane it labels; it does not float.
//
// # A tinted pill states its own edge
//
// A pinned fill separates from its ground on its own contrast. A tinted
// fill cannot: a tint and the surface it rests on are the same lightness by
// construction, so a tinted fill alone cannot carry WCAG 1.4.11's 3:1
// boundary contrast. A tinted chip therefore draws a 1 dp ring in the
// variant's pinned base colour, which is what carries that 3:1.
//
// The ring is drawn inside the pill, as nested fills — the pill in the
// ring's colour, then the fill inset by the ring's width — rather than as a
// stroke on the pill's path: a stroke is centred on its path, so a 1 dp
// stroke on the pill's own edge would spend half its width outside the box
// the chip reports.
//
// # Geometry
//
// One chip for all five: S2 horizontal padding, the S1 stop as the whole
// vertical padding split between the two edges, Full corner radius, sized to
// its label-small label, the SemiBold request resolving to the nearest
// registered face.
//
// The vertical padding is stated as a total rather than a per-edge inset
// because the pill is a box on the 4-pt grid and its two edges are not: the
// label-small line box is 16 dp, so the pill measures 16 + S1 = 20 dp.
//
// # Dismissal
//
// A tag with a non-nil [Props.OnDismiss] draws a small close mark after its
// label and reports the click; one with a nil OnDismiss draws none and takes
// no input at all. The tag never removes itself: it has no idea what it
// labels, and the caller owns the collection it came from.
//
// Dismissal is not a sixth variant. It is orthogonal to the five, so every
// treatment above can carry the mark and the mark takes its colour from
// whichever one does.
//
// The mark is deliberately small and its target is not. The visible x is
// 9 dp, and the pointer area registered under it is [CloseHitDp] square,
// centred on the mark. On a default chip that is a 24 dp target on a 9 dp
// drawing: it clears the pill by 2 dp above and below, and sideways it grows
// inward over the chip's own trailing padding and the tail of its own label
// rather than out past the pill's edge — so a row of chips has one target
// per chip however tightly it is set.
//
// Tag is a callable Go function consuming a components theme observable,
// returning a stream of layout.Widget. The source is intentionally short
// and free of opaque configuration — copy it into your own app and modify
// as needed.
package tag

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Variant selects the chip's treatment.
type Variant int

const (
	// Filled is the default: the Primary pin under OnPrimary.
	Filled Variant = iota
	// Tonal is the primary-200 tinted fill under Primary text, ringed in
	// the Primary pin.
	Tonal
	// Success, Warning and Error are the status treatments: the level's
	// tonal container under the Text pin, ringed by the 1 dp level pin.
	Success
	Warning
	Error
)

// The close affordance's three numbers, in dp.
const (
	// closeMarkDp is the square the x is drawn in. It is smaller than the
	// label's line box on purpose: the mark is an affordance on a chip,
	// not a second word in it.
	//
	// It must be even: the pill's height (line box plus S1) is always even
	// on both axes, and only an even-sided mark centres in it without a
	// half-pixel offset.
	closeMarkDp = 8
	// closeStrokeDp is the width of each of the x's two strokes, in dp. It
	// is a quarter wider than the pill's own 1 dp ring because a diagonal
	// spends part of its width on anti-aliasing while the ring's
	// axis-aligned edges do not.
	closeStrokeDp = 1.25
	// CloseHitDp is the side of the pointer target the close mark
	// registers, in dp, centred on the mark and free to overhang the pill.
	//
	// It is WCAG 2.5.8 Target Size (Minimum), the AA criterion, and not
	// the 44 dp of tokens.MinHitTarget: 44 is this system's floor for a
	// *standalone* control with space around it, and extending a chip's
	// trailing mark to it would reach into the next chip in a tightly set
	// row. 24 is what a target without that space owes.
	CloseHitDp = 24
)

// Props configures a Tag.
type Props struct {
	// Label is the chip's text. An empty label still draws the pill at
	// its padding minimum.
	Label string

	// Variant selects the treatment; the zero value is Filled.
	Variant Variant

	// OnDismiss makes the chip dismissible. When it is non-nil the chip
	// draws its close mark and calls this on a click; when it is nil the
	// chip draws no mark and registers no pointer area.
	//
	// It reports that the user asked for this chip to go away, and nothing
	// more: the chip does not hide itself on the next frame, because what
	// it labels is the caller's and only the caller knows whether the
	// answer is to drop a row, clear a filter, or ask first.
	OnDismiss func(gtx layout.Context)

	// Shaper is an explicit per-instance override of the text shaper. Leave
	// it nil in normal use: the tag then shapes its label with the theme's
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

type resolvedTokens struct {
	color   tokens.ColorTokens
	spacing tokens.SpacingScale
	radius  tokens.RadiusScale
	style   tokens.TextStyle // the LabelSmall role: typeface, weight, size, line height
	shaper  *text.Shaper     // the theme's shaper; nil in the Render path
}

// Tag returns an rx.Observable[layout.Widget] that emits a new widget
// whenever any consumed theme token changes.
//
// A tag with no OnDismiss holds no interaction state, so there is nothing to
// preserve across emissions. A dismissible one holds exactly one clickable,
// which the deferred scope below keeps across every emission: a click lands
// on the frame after the one that drew the mark, so a clickable rebuilt per
// emission would drop whichever click a token change happened to straddle.
func Tag(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	// Flatten the nested theme observables into a concrete snapshot. The
	// typography emission supplies the LabelSmall text style and the
	// theme's cached shaper. There is no density: a tag is not a control,
	// and its one affordance takes its target from the pointer floor rather
	// than from a control height.
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

	return rx.Defer(func() rx.Observable[layout.Widget] {
		var dismiss widget.Clickable
		return rx.Map(resolved, func(tok resolvedTokens) layout.Widget {
			// Props.Shaper is an explicit override; the theme's shaper is
			// the default.
			shaper := props.Shaper
			if shaper == nil {
				shaper = tok.shaper
			}
			return func(gtx layout.Context) layout.Dimensions {
				if props.OnDismiss == nil {
					return draw(gtx, shaper, props.Label, props.Variant, tok, false, nil)
				}
				// Drained to empty and reported once: a double click on a
				// close mark is one dismissal, not two, and the second
				// click left queued would fire on the next frame against
				// a chip the caller has already taken away.
				dismissed := false
				for dismiss.Clicked(gtx) {
					dismissed = true
				}
				if dismissed {
					props.OnDismiss(gtx)
				}
				return draw(gtx, shaper, props.Label, props.Variant, tok, true, &dismiss)
			}
		})
	})
}

// Render produces a layout.Widget for a tag with pre-resolved tokens.
// Intended for golden-image testing, static demonstrations, and other
// callers that already hold resolved tokens; production code composing a
// screen should use Tag, which reads the parameters below off the theme.
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
		return draw(gtx, shaper, label, v, tok, false, nil)
	}
}

// RenderDismissible is [Render] for a chip that carries its close mark: the
// same pill, widened by the mark, with the mark's pointer area registered
// against close.
//
// It is the static half of [Props.OnDismiss], for golden-image testing and
// demonstrations, and it takes the clickable rather than a callback because
// on this path there is no frame loop to drain one: the caller owns the
// clickable, lays the widget out, and reads Clicked itself. A nil dismiss
// draws the mark and registers nothing, which is what a still image wants.
func RenderDismissible(
	shaper *text.Shaper,
	label string,
	v Variant,
	dismiss *widget.Clickable,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	style tokens.TextStyle,
) layout.Widget {
	tok := resolvedTokens{color: colors, spacing: sp, radius: rad, style: style}
	return func(gtx layout.Context) layout.Dimensions {
		return draw(gtx, shaper, label, v, tok, true, dismiss)
	}
}

// draw paints one pill chip sized to its label: S2 horizontal padding, the
// S1 stop split between the two vertical edges, Full corner radius, the
// variant's fill under the variant's text colour, and — for the tinted
// variants — the 1 dp role ring drawn inside the pill's edge. The padding
// minimums keep the pill visible when the label rasterises to zero width
// (e.g., in deterministic empty-label golden tests).
//
// dismissible widens the pill by the close mark and draws it; dismiss is the
// clickable the mark's pointer area is registered against, and a nil one
// draws the mark inert, which is what a still image wants.
func draw(
	gtx layout.Context,
	shaper *text.Shaper,
	label string,
	v Variant,
	tok resolvedTokens,
	dismissible bool,
	dismiss *widget.Clickable,
) layout.Dimensions {
	padH := gtx.Dp(unit.Dp(tok.spacing.S2))
	// The S1 stop is the pill's whole vertical padding, split between the
	// two edges — see the package comment. Integer division floors it, so
	// the two edges are equal at every scale factor.
	padV := gtx.Dp(unit.Dp(tok.spacing.S1)) / 2
	rad := gtx.Dp(unit.Dp(tok.radius.Full))

	fg, bg, ring, ringed := colors(v, tok.color)

	// SemiBold resolves through the pinned shaper to the Medium face, the
	// nearest registered weight; a zero style weight falls back the same
	// way.
	mColor := op.Record(gtx.Ops)
	paint.ColorOp{Color: fg}.Add(gtx.Ops)
	material := mColor.Stop()

	labelGtx := gtx
	labelGtx.Constraints.Min = image.Point{}
	mLabel := op.Record(gtx.Ops)
	wl := typeset.Label(tok.style, 1)
	labelDims := typeset.Layout(labelGtx, shaper, wl, typeset.Font(tok.style, font.SemiBold), unit.Sp(tok.style.Size), label, material)
	labelCall := mLabel.Stop()

	mark, gap := 0, 0
	if dismissible {
		mark = gtx.Dp(unit.Dp(closeMarkDp))
		// S2 rather than S1, so the mark sits in the same 8 dp of air on
		// both sides — the chip's trailing padding on one, this gap on the
		// other — instead of being glued to the last letter.
		gap = gtx.Dp(unit.Dp(tok.spacing.S2))
	}

	w := labelDims.Size.X + 2*padH + gap + mark
	h := labelDims.Size.Y + 2*padV
	if minW := 2*padH + gap + mark; w < minW {
		w = minW
	}
	if minH := 2 * padV; h < minH {
		h = minH
	}

	// components/layout.Pill's clamp, kept here because the ring needs the
	// same rounded rect as a Path: clip.RRect does not clamp a corner
	// radius to the rect, so the Full sentinel (9999 dp) passed straight in
	// would spray paint across the canvas.
	if maxRad := min(w, h) / 2; rad > maxRad {
		rad = maxRad
	}
	rrect := clip.RRect{Rect: image.Rectangle{Max: image.Pt(w, h)}, SE: rad, SW: rad, NE: rad, NW: rad}
	if !ringed {
		paint.FillShape(gtx.Ops, bg, rrect.Op(gtx.Ops))
	} else {
		// The ring as nested fills — the pill in the ring's colour, then the
		// fill inset by the ring's width — rather than as a stroke on the
		// pill's path: a stroke is centred on the path it follows, so a 1 dp
		// stroke on the pill's own edge would spend half its width outside
		// the box the chip reports and straddle a pixel boundary instead of
		// landing on the colour it was chosen in.
		band := gtx.Dp(unit.Dp(1))
		if band < 1 {
			band = 1
		}
		inner := image.Rect(band, band, w-band, h-band)
		if inner.Dx() <= 0 || inner.Dy() <= 0 {
			// A chip too small to hold a fill inside its ring is all ring.
			paint.FillShape(gtx.Ops, ring, rrect.Op(gtx.Ops))
		} else {
			innerRad := rad - band
			if maxRad := min(inner.Dx(), inner.Dy()) / 2; innerRad > maxRad {
				innerRad = maxRad
			}
			if innerRad < 0 {
				innerRad = 0
			}
			paint.FillShape(gtx.Ops, ring, rrect.Op(gtx.Ops))
			paint.FillShape(gtx.Ops, bg, clip.RRect{
				Rect: inner, SE: innerRad, SW: innerRad, NE: innerRad, NW: innerRad,
			}.Op(gtx.Ops))
		}
	}

	st := op.Offset(image.Pt(padH, padV)).Push(gtx.Ops)
	labelCall.Add(gtx.Ops)
	st.Pop()

	if dismissible {
		origin := image.Pt(w-padH-mark, (h-mark)/2)
		drawClose(gtx, origin, mark, fg)
		registerCloseTarget(gtx, label, origin, mark, dismiss)
	}
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// drawClose strokes the x in the mark-sized square at origin, in the same
// colour the chip's label is drawn in — so the affordance is measured
// against the fill by the pairing the label already cleared, and a chip can
// never grow a mark its own ground hides.
func drawClose(gtx layout.Context, origin image.Point, mark int, c color.NRGBA) {
	// The width is scaled rather than rounded to whole pixels: a ring is
	// axis-aligned, so a whole-pixel width lands on whole pixels and reads
	// at exactly its weight, but an x is two diagonals, anti-aliased at any
	// width. Measured on the label-small specimen at 1x, 1.25 dp lands the
	// Medium face's stems between a whole pixel's over- and under-weight.
	stroke := closeStrokeDp * gtx.Metric.PxPerDp
	if stroke < 1 {
		// A zero or unset metric would erase the mark; a sub-pixel width
		// would leave it a smear. Neither is better than the thinnest
		// stroke that draws.
		stroke = 1
	}
	// Inset by the stroke's half-width so the arms end inside the square
	// rather than bleeding a half-stroke past it on the diagonal.
	in := stroke / 2
	x0, y0 := float32(origin.X)+in, float32(origin.Y)+in
	x1, y1 := float32(origin.X+mark)-in, float32(origin.Y+mark)-in

	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(x0, y0))
	p.LineTo(f32.Pt(x1, y1))
	p.MoveTo(f32.Pt(x1, y0))
	p.LineTo(f32.Pt(x0, y1))
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: stroke}.Op())
}

// registerCloseTarget puts the clickable's pointer area over the mark,
// grown to CloseHitDp on each axis and centred on it. That is the point of
// the affordance: the drawn mark is 9 dp and the target it answers to is 24.
//
// The chip's own reported size is unaffected — a caller laying tags out in a
// row spaces the pills it can see, not the slop behind them — so where the
// slop of two widgets overlaps, the one laid out later wins it, exactly as
// Gio delivers to the topmost area. It rarely comes up between two chips:
// the mark sits a whole S2 in from the pill's trailing edge, so the target's
// growth sideways is spent inside the chip.
func registerCloseTarget(gtx layout.Context, label string, origin image.Point, mark int, dismiss *widget.Clickable) {
	if dismiss == nil {
		return
	}
	hit := gtx.Dp(unit.Dp(CloseHitDp))
	if hit < mark {
		hit = mark
	}
	off := op.Offset(image.Pt(origin.X-(hit-mark)/2, origin.Y-(hit-mark)/2)).Push(gtx.Ops)
	dismiss.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.ClassOp(semantic.Button).Add(gtx.Ops)
		// The chip's own label names the target: what the mark removes is
		// the chip, and a screen reader reading the mark should say which
		// chip rather than a word this package invented for it.
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		return layout.Dimensions{Size: image.Pt(hit, hit)}
	})
	off.Pop()
}

// colors resolves a variant to its text colour, fill, and — for the tinted
// variants — the ring that states the pill's edge. The status levels take
// their role's tonal container, realized at a tone by the theme rather than
// mixed here: compositing a pinned base over the neutral Surface in
// non-linear sRGB is neither hue- nor chroma-preserving, so each status
// container instead holds its role's hue at one consistent, theme-chosen
// chroma.
//
// All of it reads roles off the token set, so every variant flips with
// light/dark and follows whatever seed, palette or high-contrast variant the
// theme is emitting.
func colors(v Variant, c tokens.ColorTokens) (fg, bg, ring color.NRGBA, ringed bool) {
	switch v {
	case Tonal:
		return c.Primary, c.Ramps.Primary.Step(200), c.Primary, true
	case Success:
		return c.Text, c.StatusContainer(tokens.RoleSuccess), c.Success, true
	case Warning:
		return c.Text, c.StatusContainer(tokens.RoleWarning), c.Warning, true
	case Error:
		return c.Text, c.StatusContainer(tokens.RoleError), c.Error, true
	default:
		return c.OnPrimary, c.Primary, color.NRGBA{}, false
	}
}
