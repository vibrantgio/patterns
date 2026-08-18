// Package toast provides the Patterns Toast pattern: a position-anchored
// column of transient notifications. A toast request is an event, so it
// becomes a message: widget code calls [Notify], which lands a [Requested]
// on the frame's ops queue, the application's Update reduces it onto a
// [Queue] it holds in its model, and [Stack] renders that queue through
// Props.Toasts. Each toast auto-dismisses after its Lifetime, which is a
// second message ([Expired], carried by the [Expire] command), fading out
// via effects/tween over a trailing fade window resolved from the theme's
// motion scale (Theme.Motion's DurSlow stop).
//
//	// in the application's Update
//	case toast.Requested:
//	    q, t := model.toasts.Add(m)
//	    model.toasts = q
//	    return model, toast.Expire(t.ID, t.Lifetime)
//	case toast.Expired:
//	    model.toasts = model.toasts.Remove(m.ID)
//
//	// in the application's composition root
//	toast.Stack(th, toast.Props{
//	    Position: toast.TopRight,
//	    Toasts:   rx.Map(modelObs, func(m Model) []toast.Toast { return m.toasts.Items() }),
//	})
//
// The queue is model state, so a toast is reproducible from a message log,
// assertable through Update without a frame, and visible in any model dump.
// What stays on the frame goroutine is the alpha: the fade is derived from
// Toast.At and gtx.Now during layout and belongs to nobody but the frame,
// while the *disappearance* is the model's (ADR-008 destinations 1 and 2 in
// one component).
//
// Until G0C.3 the entry point was a package-scoped Notify(level, text)
// publishing to a process-global Subject that every Stack subscribed. That
// signature is gone rather than deprecated: a message needs the frame's
// *op.Ops and the old one had no way to reach it, so the only shim that
// could have delivered anything was another process-global. See Notify.
//
// The package follows the Phase 4 Composition contract: Stack is a
// callable Go function consuming a components theme observable, returning a
// stream of layout.Widget. The source is intentionally short and free of
// opaque configuration — copy it into your own app and modify as needed.
//
// Colour: each toast is an inverse chip. Its base fills at the token set's
// InverseSurface and its message reads in OnInverseSurface — the pair built
// from the counterpart scheme, so the chip is dark on a light scheme and
// light on a dark one and separates from every surface it can appear over
// by construction rather than by out-elevating them. Its level shows as a
// leading edge in that level's ramp, and the Level3 cast shadow stays: the
// chip floats and can leave, and the shadow is what says so.
//
// That supersedes the elevation reading this pattern was built on, where
// the base filled at SurfaceAt(Level2) tinted 20% with the level accent and
// ringed by a 1 dp accent outline. Storeys are a way for a surface to
// separate from the one ground beneath it; a toast has no such ground — it
// can appear over a pane at any storey — so no storey is far enough from
// all of them, and the ring was doing the separating the fill could not.
// An inverse fill is one step away from every surface in the scheme, so
// the ring is gone and the accent moved to the edge, where it says which
// level this is instead of drawing the chip's shape.
package toast

import (
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/effects/depth"
	"github.com/vibrantgio/effects/tween"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Level selects the toast's semantic palette.
type Level int

const (
	Info Level = iota
	Success
	Warning
	Error
)

// Position is where on the canvas the stack anchors: one of the four
// corners, or the midpoint of the bottom edge. Newest toast renders
// nearest the anchored edge; older toasts sit further from it.
type Position int

const (
	TopRight Position = iota
	BottomRight
	TopLeft
	BottomLeft
	// BottomCenter anchors the stack to the middle of the bottom edge:
	// the column is centred between the canvas's side edges, the newest
	// toast sits one edge margin above the bottom, and older ones stack
	// upward from it. It is where a transient confirmation belongs — the
	// reader is looking at what they just acted on, not at a corner — and
	// it takes the same edge margin, gap and fade as the corners do.
	//
	// It is last in the enum because the four corners' values are what
	// callers have compiled against; a centred anchor is an addition, not
	// a renumbering.
	BottomCenter
)

// DefaultLifetime is the auto-dismiss duration Queue.Add applies to a
// Requested that names none.
const DefaultLifetime = 4 * time.Second

// The trailing slice of Lifetime during which a toast tweens its alpha
// from 1.0 to 0.0 resolves from the theme's motion scale: Theme.Motion's
// DurSlow stop (MD3 medium4, 400 ms — E3.1's mapping of the local 400 ms
// constant it replaced). Short enough that the dismiss feels snappy but
// long enough that the fade is perceptible at 60 fps. It reaches the
// frame path as resolvedTokens.fade.

// Toast is one queued notification. It is model state: Queue.Add builds it
// from a Requested and nothing mutates it afterwards.
//
// At is the instant the toast was asked for — gtx.Now inside a frame,
// time.Now elsewhere — and it is what the fade is measured from. A zero At
// disables fading for that toast (the Render path, and any Toast built by
// hand): it paints fully opaque until something removes it from the queue.
type Toast struct {
	ID       int64
	Level    Level
	Text     string
	At       time.Time
	Lifetime time.Duration
}

// Requested is the message a toast request becomes. Notify lands one from
// inside a frame; Request builds one for a command goroutine. Lifetime is
// optional and defaults to DefaultLifetime when Queue.Add sees it zero.
type Requested struct {
	Level    Level
	Text     string
	At       time.Time
	Lifetime time.Duration
}

// Expired retires the toast with the given ID. The Expire command emits it
// once the toast's Lifetime has run; an application may also emit it itself
// to dismiss a toast early.
type Expired struct{ ID int64 }

// Notify asks for a toast from inside a frame. It lands a Requested on the
// frame's ops queue, stamped with the frame's own clock, and the
// application's Update queues it — the same path components/button's OnClick
// messages take.
//
// The message is collected off gtx.Ops, and mvu's collector is keyed on the
// exact buffer the frame is being recorded into: a call made from a widget
// recording somewhere else — inside a components/cache.FrameCache body, most of
// all — is dropped silently. Emit from the widget that owns gtx.Ops.
//
// This replaces the pre-G0C.3 Notify(level, text), which published to a
// process-global Subject. The signature change is deliberate and is not a
// deprecation: the new path needs gtx and the old signature cannot supply
// it, so every call site has to be visited. Callers see "not enough
// arguments in call to toast.Notify", which names the fix.
func Notify(gtx layout.Context, level Level, text string) {
	mvu.MessageOp{Message: Requested{Level: level, Text: text, At: gtx.Now}}.Add(gtx.Ops)
}

// Request builds the same message from outside a frame — a command
// goroutine, a test — stamping it with the wall clock. A command that
// returns one raises a toast without touching the renderer or knowing which
// goroutine it is on:
//
//	mvu.Do(func() (mvu.Message, error) {
//	    if err := save(path, doc); err != nil {
//	        return toast.Request(toast.Error, "Save failed"), nil
//	    }
//	    return toast.Request(toast.Success, "Saved"), nil
//	})
func Request(level Level, text string) Requested {
	return Requested{Level: level, Text: text, At: time.Now()}
}

// Expire is the command that retires toast id after it has been up for the
// given duration. rx.Timer (not time.Sleep) keeps it cancellable, so
// quitting the app with a toast on screen does not block the runner's
// teardown. Removing a toast the application already removed is a no-op, so
// a late timer is harmless and needs no generation guard.
func Expire(id int64, after time.Duration) mvu.Command {
	return mvu.Command{Observable: rx.Map(rx.Timer[int](after), func(int) any {
		return Expired{ID: id}
	})}
}

// Queue is the toast queue an application holds in its model: oldest first,
// newest last. The zero Queue is empty and ready to use.
//
// It is a value, and Add and Remove return a new one whose slice is freshly
// allocated at exactly its own length — so no Queue ever aliases another,
// a previous model still shows the toasts that were up when it was current,
// and an append by a caller holding Items cannot reach into it.
type Queue struct {
	items []Toast
	next  int64
}

// Add queues r and returns the new Queue together with the Toast it queued.
// The returned toast's ID is what Expire and Expired name, and its Lifetime
// is r's or DefaultLifetime — read it rather than re-deriving it, so the
// timer and the fade cannot disagree.
func (q Queue) Add(r Requested) (Queue, Toast) {
	q.next++
	t := Toast{ID: q.next, Level: r.Level, Text: r.Text, At: r.At, Lifetime: r.Lifetime}
	if t.Lifetime <= 0 {
		t.Lifetime = DefaultLifetime
	}
	items := make([]Toast, 0, len(q.items)+1)
	q.items = append(append(items, q.items...), t)
	return q, t
}

// Remove drops the toast with the given ID. An ID that is not queued is a
// no-op: an Expired arriving after the toast was dismissed some other way
// changes nothing.
func (q Queue) Remove(id int64) Queue {
	for i, t := range q.items {
		if t.ID != id {
			continue
		}
		items := make([]Toast, 0, len(q.items)-1)
		items = append(items, q.items[:i]...)
		q.items = append(items, q.items[i+1:]...)
		return q
	}
	return q
}

// Items returns the queued toasts, oldest first — the value an application
// maps onto Props.Toasts.
func (q Queue) Items() []Toast { return q.items }

// Len reports how many toasts are queued.
func (q Queue) Len() int { return len(q.items) }

// Props configures a Stack.
type Props struct {
	Position Position

	// Toasts is the queue to render, normally derived from the model:
	// rx.Map(modelObs, func(m Model) []toast.Toast { return m.toasts.Items() }).
	//
	// A Stack with no Toasts renders an empty column forever. That is the
	// one place this component is not additive across G0C.3: a caller that
	// kept passing only Position used to receive every toast in the process
	// through the package Subject and now receives none, with nothing to
	// fail at compile time. It is the reason cadence's next tag is a minor
	// bump rather than a patch.
	Toasts rx.Observable[[]Toast]

	// Lifetime is the fallback auto-dismiss duration for toasts that carry
	// none of their own — hand-built queues, demos, goldens. Toast.Lifetime
	// wins where it is set, which is everything Queue.Add produces, and
	// DefaultLifetime applies when neither is.
	Lifetime time.Duration

	// Shaper is an explicit per-instance override of the text shaper. Leave
	// it nil in normal use: the stack then shapes its toast text with the
	// theme's shaper (Typography.Shaper()), which is built once for the
	// process and shared by every component reading that typography — the
	// cache lives behind the Typography value, so it survives the copy this
	// component's map function makes of it (spectrum F5.1). Set it only when
	// this instance must shape with a different shaper than the theme
	// provides.
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
	style   tokens.TextStyle // the LabelMedium role: typeface, weight, size, line height
	shaper  *text.Shaper     // the theme's shaper; nil in the Render path
	// elevation is snapshotted so a theme elevation change re-emits the
	// widget; the base fill resolves through SurfaceAt, which reads the
	// default tokens.Elevation scale.
	elevation tokens.ElevationScale
	// fade is the trailing fade window, the motion scale's DurSlow stop.
	// Zero (the Render path) disables fading: toasts paint fully opaque
	// until they leave the queue.
	fade time.Duration
}

// Stack returns an rx.Observable[layout.Widget] that renders Props.Toasts as
// a positioned column. It holds no state of its own: the queue arrives from
// the model, and the only thing the frame decides is each toast's alpha.
// Expiry is not the widget's job — a toast past its Lifetime paints nothing
// and waits for the Expired message to take it out of the model.
func Stack(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	// Flatten the nested theme observables into a concrete snapshot. The
	// typography emission supplies both the LabelMedium text style and the
	// theme's cached shaper (ADR-003: the theme owns the typeface); the
	// motion emission supplies the fade window (rx tops out at
	// CombineLatest5, hence the nested CombineLatest2).
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest2(
				rx.CombineLatest5(t.Color, t.Spacing, t.Radius, t.Typography, t.Elevation),
				t.Motion,
			),
			func(n rx.Tuple2[rx.Tuple5[tokens.ColorTokens, tokens.SpacingScale, tokens.RadiusScale, tokens.Typography, tokens.ElevationScale], tokens.MotionScale]) resolvedTokens {
				typ := n.First.Fourth
				return resolvedTokens{
					color:     n.First.First,
					spacing:   n.First.Second,
					radius:    n.First.Third,
					style:     typ.LabelMedium,
					shaper:    typ.Shaper(),
					elevation: n.First.Fifth,
					fade:      n.Second.DurSlow,
				}
			},
		)
	})
	toasts := props.Toasts
	if toasts == nil {
		toasts = rx.Of([]Toast(nil))
	}
	return rx.Map(rx.CombineLatest2(resolved, toasts), func(n rx.Tuple2[resolvedTokens, []Toast]) layout.Widget {
		tok, queued := n.First, n.Second
		// Props.Shaper is an explicit override; the theme's shaper is
		// the default.
		shaper := props.Shaper
		if shaper == nil {
			shaper = tok.shaper
		}
		return func(gtx layout.Context) layout.Dimensions {
			return drawStackLive(gtx, shaper, props, tok, queued)
		}
	})
}

// Render produces a layout.Widget for a fixed []Toast snapshot with
// pre-resolved tokens. Intended for golden-image testing and static
// demonstrations; production code should use Stack, which takes the
// shaper and the same text style off the theme. The returned widget
// performs no input handling, no fading, and schedules no invalidation.
//
// label is the LabelMedium role's whole text style — typeface, weight,
// size and line height all reach the shaper, exactly as they do on the
// live path. Pass tokens.DefaultTypography.LabelMedium for the default
// desktop look. There is no density parameter: a toast's height is a
// legibility floor around its message, not a control height (E1.4).
func Render(
	shaper *text.Shaper,
	props Props,
	toasts []Toast,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	label tokens.TextStyle,
) layout.Widget {
	tok := resolvedTokens{color: colors, spacing: sp, radius: rad, style: label}
	return func(gtx layout.Context) layout.Dimensions {
		return drawStackStatic(gtx, shaper, props, tok, toasts)
	}
}

// placed is one queued toast plus the alpha this frame paints it at. Alpha
// is the whole of the per-frame state a stack has, and it is derived, not
// stored: nothing here survives the frame it was computed on.
type placed struct {
	toast Toast
	alpha float64
}

// drawStackLive computes each toast's fade alpha, schedules the next
// invalidation, and paints. It never prunes: a toast whose lifetime has run
// paints at alpha 0 until the Expired message takes it out of the model, so
// the queue on screen and the queue in the model are the same list.
func drawStackLive(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	tok resolvedTokens,
	queued []Toast,
) layout.Dimensions {
	now := gtx.Now
	items := make([]placed, len(queued))
	var nextFade time.Time
	fading := false
	for i, t := range queued {
		lifetime := lifetimeOf(t, props)
		items[i] = placed{toast: t, alpha: fadeAlpha(t.At, lifetime, tok.fade, now)}
		if t.At.IsZero() || tok.fade <= 0 {
			continue
		}
		switch start := t.At.Add(lifetime - tok.fade); {
		case now.Before(start):
			// Wake once, when this toast starts fading.
			if nextFade.IsZero() || start.Before(nextFade) {
				nextFade = start
			}
		case items[i].alpha > 0:
			fading = true
		}
	}
	// A toast mid-fade redraws every frame so the alpha animates; otherwise
	// one scheduled wake at the earliest fade start is enough. Everything
	// past expiry is the model's business and arrives as a message.
	if fading {
		gtx.Execute(op.InvalidateCmd{})
	} else if !nextFade.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: nextFade})
	}
	return paintStack(gtx, shaper, props, tok, items)
}

// drawStackStatic paints the supplied toasts at full opacity with no
// scheduling. Used by Render for goldens.
func drawStackStatic(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	tok resolvedTokens,
	toasts []Toast,
) layout.Dimensions {
	items := make([]placed, len(toasts))
	for i, t := range toasts {
		items[i] = placed{toast: t, alpha: 1}
	}
	return paintStack(gtx, shaper, props, tok, items)
}

// lifetimeOf resolves the auto-dismiss duration for one toast: its own
// Lifetime (everything Queue.Add produces has one), else the stack-wide
// Props.Lifetime, else DefaultLifetime.
func lifetimeOf(t Toast, props Props) time.Duration {
	if t.Lifetime > 0 {
		return t.Lifetime
	}
	if props.Lifetime > 0 {
		return props.Lifetime
	}
	return DefaultLifetime
}

// Toast surface metrics. The toast is a transient notification surface,
// not a control: its height hugs the label plus spacing-scale padding,
// and toastMinHDp is a legibility floor, so none of it follows density
// (E1.4 verdict — the 36 dp floor coinciding with the Comfortable control
// height is incidental).
const (
	toastWidthDp = 240
	toastMinHDp  = 36
)

// paintStack lays out the column of toasts at the canvas anchor
// Props.Position names, each at the alpha its placed entry carries.
func paintStack(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	tok resolvedTokens,
	items []placed,
) layout.Dimensions {
	canvas := gtx.Constraints.Max
	edgePad := gtx.Dp(unit.Dp(tok.spacing.S4))
	gap := gtx.Dp(unit.Dp(tok.spacing.S2))
	width := gtx.Dp(unit.Dp(toastWidthDp))
	if width > canvas.X-2*edgePad {
		width = canvas.X - 2*edgePad
		if width < 0 {
			width = 0
		}
	}

	// The two anchors are read separately because they do not pair up: a
	// centred column hugs neither side edge, so the horizontal question
	// has three answers where the vertical still has two.
	topAnchored := props.Position == TopLeft || props.Position == TopRight

	var x int
	switch props.Position {
	case TopLeft, BottomLeft:
		x = edgePad
	case BottomCenter:
		// The column's own middle on the canvas's. Width is already
		// clamped to the space between the two edge margins, so a
		// canvas too narrow for the full width centres what is left
		// rather than overhanging either edge.
		x = (canvas.X - width) / 2
	default:
		x = canvas.X - edgePad - width
	}

	// Render order: newest nearest the anchored edge. items[len-1] is
	// the newest. For top-anchored stacks we walk newest-first downward;
	// for bottom-anchored stacks we walk newest-first upward.
	order := make([]int, len(items))
	if topAnchored {
		for i := range items {
			order[i] = len(items) - 1 - i
		}
	} else {
		for i := range items {
			order[i] = i
		}
	}

	// First measure all toasts so bottom-anchored stacks can position
	// from the bottom up.
	heights := make([]int, len(items))
	macros := make([]op.CallOp, len(items))
	for vis, idx := range order {
		macro := op.Record(gtx.Ops)
		toastGtx := gtx
		toastGtx.Constraints = layout.Constraints{
			Min: image.Pt(width, gtx.Dp(unit.Dp(toastMinHDp))),
			Max: image.Pt(width, canvas.Y),
		}
		dims := paintToast(toastGtx, shaper, tok, items[idx])
		macros[vis] = macro.Stop()
		heights[vis] = dims.Size.Y
	}

	var y int
	if topAnchored {
		y = edgePad
	} else {
		total := 0
		for i, h := range heights {
			total += h
			if i > 0 {
				total += gap
			}
		}
		y = canvas.Y - edgePad - total
	}

	for vis := range order {
		off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		macros[vis].Add(gtx.Ops)
		off.Pop()
		y += heights[vis] + gap
	}

	return layout.Dimensions{Size: canvas}
}

// paintToast paints one inverse chip sized to its content: a Level3 cast
// shadow under a flat InverseSurface fill, its message in
// OnInverseSurface, and a leading edge one spacing stop wide in the
// level's own ramp. The fade alpha is applied to the shadow (via its
// opacity argument), the fill, the edge and the text colour.
//
// Two judgements were re-made over the new fill rather than carried over
// from the tinted level-2 one. The shadow stays: the inverse fill already
// separates the chip from everything under it, but separation is not the
// only thing a shadow says — this surface is temporary and floating, and
// nothing else in the frame says that. The outline goes: a ring in the
// accent was what made the old fill read as a shape at all, and the fill
// now measures 13.0:1 against the Surface panes in both schemes and no
// worse than 7.6:1 against the deepest surface storey, so the ring is
// decoration — and decoration that cost the level its one strong signal,
// which the leading edge now carries.
func paintToast(
	gtx layout.Context,
	shaper *text.Shaper,
	tok resolvedTokens,
	it placed,
) layout.Dimensions {
	padH := gtx.Dp(unit.Dp(tok.spacing.S3))
	padV := gtx.Dp(unit.Dp(tok.spacing.S2))
	edgeW := gtx.Dp(unit.Dp(tok.spacing.S1))
	r := gtx.Dp(unit.Dp(tok.radius.Md))
	alpha := it.alpha

	fill := withAlpha(tok.color.InverseSurface, alpha)
	edge := withAlpha(edgeColor(it.toast.Level, tok.color), alpha)
	fg := withAlpha(tok.color.OnInverseSurface, alpha)

	// Pre-record the label so we can size the surface around its dims. The
	// leading edge takes its width off the label's, so the trailing margin
	// stays one padH and the text stands one padH clear of the edge.
	mColor := op.Record(gtx.Ops)
	paint.ColorOp{Color: fg}.Add(gtx.Ops)
	material := mColor.Stop()
	mLabel := op.Record(gtx.Ops)
	labelGtx := gtx
	labelGtx.Constraints = layout.Constraints{
		Max: image.Pt(gtx.Constraints.Max.X-edgeW-2*padH, gtx.Constraints.Max.Y),
	}
	// Shape with the LabelMedium role's typeface, weight, size and line
	// height. Zero fields (the legacy Render path synthesizes a size-only
	// style) fall back to the shaper's defaults.
	style := tok.style
	f := typeset.Font(style, font.Normal)
	wl := typeset.Label(style, 1)
	labelDims := typeset.Layout(labelGtx, shaper, wl, f, unit.Sp(style.Size), it.toast.Text, material)
	labelCall := mLabel.Stop()

	w := gtx.Constraints.Max.X
	h := labelDims.Size.Y + 2*padV
	if h < gtx.Constraints.Min.Y {
		h = gtx.Constraints.Min.Y
	}

	// The shadow says the chip is a temporary layer, not that it is
	// separate — the inverse fill has that covered. It rounds to the
	// fill's radius so its interior cannot show through the corners, and
	// it takes the toast's fade alpha directly so it never outlives the
	// surface.
	depth.Shadow(gtx, image.Rectangle{Max: image.Pt(w, h)}, tokens.Level3, r, float32(alpha))

	rect := clip.RRect{Rect: image.Rectangle{Max: image.Pt(w, h)}, SE: r, SW: r, NE: r, NW: r}
	paint.FillShape(gtx.Ops, fill, rect.Op(gtx.Ops))
	// The level's edge, clipped to the chip so it wears the same rounded
	// corners the fill does rather than squaring off the leading side.
	clipped := rect.Op(gtx.Ops).Push(gtx.Ops)
	paint.FillShape(gtx.Ops, edge, clip.Rect{Max: image.Pt(edgeW, h)}.Op())
	clipped.Pop()

	labelY := padV
	if labelDims.Size.Y < h-2*padV {
		labelY = (h - labelDims.Size.Y) / 2
	}
	labelOff := op.Offset(image.Pt(edgeW+padH, labelY)).Push(gtx.Ops)
	labelCall.Add(gtx.Ops)
	labelOff.Pop()

	return layout.Dimensions{Size: image.Pt(w, h)}
}

// fadeAlpha returns a toast's alpha in [0,1] for a frame at now. A zero at
// (the Render path, or a hand-built Toast) means "fully opaque". Otherwise
// the alpha tweens from 1.0 to 0.0 across the final fade window (the
// theme's DurSlow stop) of the lifetime via effects/tween.LerpFloat64, and
// stays at 0 past expiry while the Expired message travels the loop; a zero
// fade window paints fully opaque until then.
func fadeAlpha(at time.Time, lifetime, fade time.Duration, now time.Time) float64 {
	if at.IsZero() || lifetime <= 0 {
		return 1
	}
	age := now.Sub(at)
	if age >= lifetime {
		return 0
	}
	if fade <= 0 || age < lifetime-fade {
		return 1
	}
	tw := tween.Tween[float64]{
		From:   1,
		To:     0,
		Frames: int(fade / time.Millisecond),
		Lerp:   tween.LerpFloat64,
	}
	frame := int((age - (lifetime - fade)) / time.Millisecond)
	return tw.At(frame)
}

// edgeStep is the rung of the level's ramp the leading edge takes. It is
// the deepest rung that still reads over the inverse ground in both
// schemes, which is what fixes it: the ramps are paired scales, so in a
// light scheme step 400 is a light tint against a dark chip (7.86–7.89:1
// across the four levels) and in a dark scheme the same step is a deep
// shade against a light one (7.61–7.63:1), while step 500 — the ramps'
// mid-value rung — collapses to 2.19:1 on the dark scheme's light chip.
// Every level clears the 3:1 a non-text graphic owes its ground with room
// to spare, in both schemes and in the high-contrast variant, whose 100–600
// stops are the default scale's.
const edgeStep = 400

// edgeColor maps Level to the colour of its leading edge: that level's own
// ramp at edgeStep, so it flips with light/dark and follows whatever seed,
// palette or high-contrast variant the theme is emitting.
//
// It reads a ramp rather than the pinned base the fill used to be tinted
// with. The pins are tuned to be filled and written on — their depth is
// chosen against the scheme's own grounds — and against the inverse ground
// they are on the wrong side: a dark scheme's pins sit at L* 82, which is
// most of the way to that scheme's own light chip.
//
// Until F4.6 Success and Warning were Tailwind green and amber literals,
// duplicated byte-for-byte between this file and alert/alert.go; theme's
// hue-fixed success and warning ramps replaced both copies.
func edgeColor(l Level, c tokens.ColorTokens) color.NRGBA {
	switch l {
	case Error:
		return c.Ramps.Error.Step(edgeStep)
	case Success:
		return c.Ramps.Success.Step(edgeStep)
	case Warning:
		return c.Ramps.Warning.Step(edgeStep)
	default:
		return c.Ramps.Primary.Step(edgeStep)
	}
}

func withAlpha(c color.NRGBA, a float64) color.NRGBA {
	if a >= 1 {
		return c
	}
	if a <= 0 {
		return color.NRGBA{}
	}
	out := c
	out.A = uint8(float64(c.A)*a + 0.5)
	return out
}
