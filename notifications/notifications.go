// Package notifications provides the notifications pattern: the
// position-anchored column that receives the application's notifications,
// places them, stacks them against each other, times them, and presents each
// one as a toast (components/toast). A notification is raised by message,
// never drawn in place: layout code calls [Notify], which lands a [Requested]
// on the frame's ops queue, the application's Update reduces it onto a
// [Queue] it holds in its model, and [Column] renders that queue through
// Props.Notifications. Each notification leaves after its Lifetime, which is
// a second message ([Expired], carried by the [Expire] command), fading out
// via effects/tween over a trailing fade window resolved from the theme's
// motion scale (Theme.Motion's DurSlow stop).
//
//	// in the application's Update
//	case notifications.Requested:
//	    q, n := model.notes.Add(m)
//	    model.notes = q
//	    return model, notifications.Expire(n.ID, n.Lifetime)
//	case notifications.Expired:
//	    model.notes = model.notes.Remove(m.ID)
//
//	// in the application's composition root
//	notifications.Column(th, notifications.Props{
//	    Position:      notifications.TopRight,
//	    Notifications: rx.Map(modelObs, func(m Model) []notifications.Notification { return m.notes.Items() }),
//	})
//
// The queue is model state, so a notification is reproducible from a message
// log, assertable through Update without a frame, and visible in any model
// dump. What stays on the frame goroutine is the alpha: the fade is derived
// from Notification.At and gtx.Now during layout and belongs to nobody but
// the frame, while the *disappearance* is the model's.
//
// Column is a callable Go function consuming a components theme observable,
// returning a stream of layout.Widget. The source is intentionally short
// and free of opaque configuration — copy it into your own app and modify
// as needed.
//
// The pattern owns the queue, the placement and the timing, not the
// presentation: the toast's fill, its message and its leading edge are
// components/toast's. What the column adds under each toast is the
// platform's floating shadow, because only the placement knows where the
// surface landed — and a shadow is what says the surface floats and can
// leave.
package notifications

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/toast"
	"github.com/vibrantgio/effects/depth"
	"github.com/vibrantgio/effects/tween"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Position is where in the frame the column anchors: one of the four
// corners, or the midpoint of the bottom edge. The newest notification
// renders nearest the anchored edge; older ones sit further from it.
type Position int

const (
	TopRight Position = iota
	BottomRight
	TopLeft
	BottomLeft
	// BottomCenter anchors the column to the middle of the bottom edge:
	// the column is centred between the frame's side edges, the newest
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
// DurSlow stop (MD3 medium4, 400 ms). Short enough that the dismiss feels
// snappy but long enough that the fade is perceptible at 60 fps. It
// reaches the frame path as resolvedTokens.fade.

// Notification is one queued message the column presents. It is model
// state: Queue.Add builds it from a Requested and nothing mutates it
// afterwards.
//
// At is the instant the notification was raised — gtx.Now inside a frame,
// time.Now elsewhere — and it is what the fade is measured from. A zero At
// disables fading for that notification (the Render path, and any
// Notification built by hand): it paints fully opaque until something
// removes it from the queue.
type Notification struct {
	ID       int64
	Status   toast.Status
	Text     string
	At       time.Time
	Lifetime time.Duration
}

// Requested is the message a notification becomes. Notify lands one from
// inside a frame; Request builds one for a command goroutine. Lifetime is
// optional and defaults to DefaultLifetime when Queue.Add sees it zero.
type Requested struct {
	Status   toast.Status
	Text     string
	At       time.Time
	Lifetime time.Duration
}

// Expired retires the notification with the given ID. The Expire command
// emits it once the Lifetime has run; an application may also emit it itself
// to dismiss one early.
type Expired struct{ ID int64 }

// Notify raises a notification from inside a frame. It lands a Requested on
// the frame's ops queue, stamped with the frame's own clock, and the
// application's Update queues it — the same path components/button's OnClick
// messages take.
//
// The message is collected off gtx.Ops, and mvu's collector is keyed on the
// exact buffer the frame is being recorded into: a call made from a layout.Widget
// recording somewhere else — inside a components/cache.FrameCache body, most of
// all — is dropped silently. Emit from the layout.Widget that owns gtx.Ops.
func Notify(gtx layout.Context, status toast.Status, text string) {
	mvu.MessageOp{Message: Requested{Status: status, Text: text, At: gtx.Now}}.Add(gtx.Ops)
}

// Request builds the same message from outside a frame — a command
// goroutine, a test — stamping it with the wall clock. A command that
// returns one raises a notification without touching the renderer or knowing
// which goroutine it is on:
//
//	mvu.Do(func() (mvu.Message, error) {
//	    if err := save(path, doc); err != nil {
//	        return notifications.Request(toast.Error, "Save failed"), nil
//	    }
//	    return notifications.Request(toast.Success, "Saved"), nil
//	})
func Request(status toast.Status, text string) Requested {
	return Requested{Status: status, Text: text, At: time.Now()}
}

// Expire is the command that retires notification id after it has been up
// for the given duration. rx.Timer (not time.Sleep) keeps it cancellable, so
// quitting the app with a toast on screen does not block the runner's
// teardown. Removing a notification the application already removed is a
// no-op, so a late timer is harmless and needs no generation guard.
func Expire(id int64, after time.Duration) mvu.Command {
	return mvu.Command{Observable: rx.Map(rx.Timer[int](after), func(int) any {
		return Expired{ID: id}
	})}
}

// Queue is the notification queue an application holds in its model: oldest
// first, newest last. The zero Queue is empty and ready to use.
//
// It is a value, and Add and Remove return a new one whose slice is freshly
// allocated at exactly its own length — so no Queue ever aliases another,
// a previous model still shows the notifications that were up when it was
// current, and an append by a caller holding Items cannot reach into it.
type Queue struct {
	items []Notification
	next  int64
}

// Add queues r and returns the new Queue together with the Notification it
// queued. The returned notification's ID is what Expire and Expired name,
// and its Lifetime is r's or DefaultLifetime — read it rather than
// re-deriving it, so the timer and the fade cannot disagree.
func (q Queue) Add(r Requested) (Queue, Notification) {
	q.next++
	n := Notification{ID: q.next, Status: r.Status, Text: r.Text, At: r.At, Lifetime: r.Lifetime}
	if n.Lifetime <= 0 {
		n.Lifetime = DefaultLifetime
	}
	items := make([]Notification, 0, len(q.items)+1)
	q.items = append(append(items, q.items...), n)
	return q, n
}

// Remove drops the notification with the given ID. An ID that is not queued
// is a no-op: an Expired arriving after the notification was dismissed some
// other way changes nothing.
func (q Queue) Remove(id int64) Queue {
	for i, n := range q.items {
		if n.ID != id {
			continue
		}
		items := make([]Notification, 0, len(q.items)-1)
		items = append(items, q.items[:i]...)
		q.items = append(items, q.items[i+1:]...)
		return q
	}
	return q
}

// Items returns the queued notifications, oldest first — the value an
// application maps onto Props.Notifications.
func (q Queue) Items() []Notification { return q.items }

// Len reports how many notifications are queued.
func (q Queue) Len() int { return len(q.items) }

// Props configures a Column.
type Props struct {
	Position Position

	// Notifications is the queue to render, normally derived from the model:
	// rx.Map(modelObs, func(m Model) []notifications.Notification { return m.notes.Items() }).
	//
	// A Column with no Notifications renders an empty column forever.
	Notifications rx.Observable[[]Notification]

	// Lifetime is the fallback auto-dismiss duration for notifications that
	// carry none of their own — hand-built queues, demos, goldens.
	// Notification.Lifetime wins where it is set, which is everything
	// Queue.Add produces, and DefaultLifetime applies when neither is.
	Lifetime time.Duration

	// Shaper is an explicit per-instance override of the text shaper. Leave
	// it nil in normal use: the column then shapes each toast's message with
	// the theme's shaper (Typography.Shaper()), which is built once for the
	// process and shared by every component reading that typography — the
	// cache lives behind the Typography value, so it survives the copy this
	// pattern's map function makes of it. Set it only when this instance
	// must shape with a different shaper than the theme provides.
	//
	// A shaper is not safe to use from two goroutines; Gio lays every
	// layout.Widget out on the one goroutine that runs the event loop,
	// which is what makes sharing it correct. See theme/tokens.Typography.Shaper.
	Shaper *text.Shaper
}

type resolvedTokens struct {
	color   tokens.PlatformColors
	spacing tokens.SpacingScale
	radius  tokens.RadiusScale
	style   tokens.TextStyle // the LabelMedium role: typeface, weight, size, line height
	shaper  *text.Shaper     // the theme's shaper; nil in the Render path
	// elevation is snapshotted so a theme elevation change re-emits the
	// layout.Widget; the toast's fill resolves through the inverse pair,
	// which reads the default tokens.Elevation scale.
	elevation tokens.ElevationScale
	// fade is the trailing fade window, the motion scale's DurSlow stop.
	// Zero (the Render path) disables fading: toasts paint fully opaque
	// until they leave the queue.
	fade time.Duration
}

// Column returns an rx.Observable[layout.Widget] that renders
// Props.Notifications as a positioned column of toasts. It holds no state of
// its own: the queue arrives from the model, and the only thing the frame
// decides is each toast's alpha. Expiry is not the layout.Widget's job — a
// notification past its Lifetime paints nothing and waits for the Expired
// message to take it out of the model.
func Column(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	// Flatten the nested theme observables into a concrete snapshot. The
	// typography emission supplies both the LabelMedium text style and the
	// theme's cached shaper; the motion emission supplies the fade window
	// (rx tops out at CombineLatest5, hence the nested CombineLatest2).
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest2(
				rx.CombineLatest5(t.Platform, t.Spacing, t.Radius, t.Typography, t.Elevation),
				t.Motion,
			),
			func(n rx.Tuple2[rx.Tuple5[tokens.PlatformColors, tokens.SpacingScale, tokens.RadiusScale, tokens.Typography, tokens.ElevationScale], tokens.MotionScale]) resolvedTokens {
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
	queue := props.Notifications
	if queue == nil {
		queue = rx.Of([]Notification(nil))
	}
	return rx.Map(rx.CombineLatest2(resolved, queue), func(n rx.Tuple2[resolvedTokens, []Notification]) layout.Widget {
		tok, queued := n.First, n.Second
		// Props.Shaper is an explicit override; the theme's shaper is
		// the default.
		shaper := props.Shaper
		if shaper == nil {
			shaper = tok.shaper
		}
		return func(gtx layout.Context) layout.Dimensions {
			return drawColumnLive(gtx, shaper, props, tok, queued)
		}
	})
}

// Render produces a layout.Widget for a fixed []Notification snapshot with
// pre-resolved tokens. Intended for golden-image testing and static
// demonstrations; production code should use Column, which takes the
// shaper and the same text style off the theme. The returned layout.Widget
// performs no input handling, no fading, and schedules no invalidation.
//
// label is the LabelMedium role's whole text style — typeface, weight,
// size and line height all reach the shaper, exactly as they do on the
// live path. Pass tokens.DefaultTypography.LabelMedium for the default
// desktop look. There is no density parameter: a toast's height is a
// legibility floor around its message, not a control height.
func Render(
	shaper *text.Shaper,
	props Props,
	queued []Notification,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	label tokens.TextStyle,
) layout.Widget {
	tok := resolvedTokens{color: colors, spacing: sp, radius: rad, style: label}
	return func(gtx layout.Context) layout.Dimensions {
		return drawColumnStatic(gtx, shaper, props, tok, queued)
	}
}

// placed is one queued notification plus the alpha this frame paints its
// toast at. Alpha is the whole of the per-frame state a column has, and it
// is derived, not stored: nothing here survives the frame it was computed
// on.
type placed struct {
	note  Notification
	alpha float64
}

// drawColumnLive computes each toast's fade alpha, schedules the next
// invalidation, and paints. It never prunes: a notification whose lifetime
// has run paints nothing until the Expired message takes it out of the
// model, so the queue on screen and the queue in the model are the same
// list.
func drawColumnLive(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	tok resolvedTokens,
	queued []Notification,
) layout.Dimensions {
	now := gtx.Now
	items := make([]placed, len(queued))
	var nextFade time.Time
	fading := false
	for i, n := range queued {
		lifetime := lifetimeOf(n, props)
		items[i] = placed{note: n, alpha: fadeAlpha(n.At, lifetime, tok.fade, now)}
		if n.At.IsZero() || tok.fade <= 0 {
			continue
		}
		switch start := n.At.Add(lifetime - tok.fade); {
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
	return paintColumn(gtx, shaper, props, tok, items)
}

// drawColumnStatic paints the supplied notifications at full opacity with no
// scheduling. Used by Render for goldens.
func drawColumnStatic(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	tok resolvedTokens,
	queued []Notification,
) layout.Dimensions {
	items := make([]placed, len(queued))
	for i, n := range queued {
		items[i] = placed{note: n, alpha: 1}
	}
	return paintColumn(gtx, shaper, props, tok, items)
}

// lifetimeOf resolves the auto-dismiss duration for one notification: its
// own Lifetime (everything Queue.Add produces has one), else the
// column-wide Props.Lifetime, else DefaultLifetime.
func lifetimeOf(n Notification, props Props) time.Duration {
	if n.Lifetime > 0 {
		return n.Lifetime
	}
	if props.Lifetime > 0 {
		return props.Lifetime
	}
	return DefaultLifetime
}

// paintColumn lays out the toasts at the frame anchor Props.Position names,
// each at the alpha its placed entry carries, with the platform's floating
// shadow under it. The shadow is the column's rather than the toast's
// because only the placement knows where the surface landed; it rounds to
// the toast's own radius so the interior cannot show through the corners,
// and it is scaled by the same alpha so it never outlives the surface it
// belongs to.
func paintColumn(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	tok resolvedTokens,
	items []placed,
) layout.Dimensions {
	frame := gtx.Constraints.Max
	edgePad := gtx.Dp(unit.Dp(tok.spacing.S4))
	gap := gtx.Dp(unit.Dp(tok.spacing.S2))
	radius := gtx.Dp(unit.Dp(tok.radius.Md))
	width := gtx.Dp(unit.Dp(toast.WidthDp))
	if width > frame.X-2*edgePad {
		width = frame.X - 2*edgePad
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
		// The column's own middle on the frame's. Width is already
		// clamped to the space between the two edge margins, so a
		// frame too narrow for the full width centres what is left
		// rather than overhanging either edge.
		x = (frame.X - width) / 2
	default:
		x = frame.X - edgePad - width
	}

	// Render order: newest nearest the anchored edge. items[len-1] is
	// the newest. For top-anchored columns we walk newest-first downward;
	// for bottom-anchored columns we walk newest-first upward.
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

	// First measure every toast so bottom-anchored columns can position
	// from the bottom up.
	sizes := make([]image.Point, len(items))
	alphas := make([]float64, len(items))
	macros := make([]op.CallOp, len(items))
	for vis, idx := range order {
		macro := op.Record(gtx.Ops)
		toastGtx := gtx
		toastGtx.Constraints = layout.Constraints{
			Min: image.Pt(width, gtx.Dp(unit.Dp(toast.MinHeightDp))),
			Max: image.Pt(width, frame.Y),
		}
		alphas[vis] = items[idx].alpha
		dims := toast.Render(shaper, toast.Props{
			Status: items[idx].note.Status,
			Text:   items[idx].note.Text,
			Alpha:  alphas[vis],
		}, tok.color, tok.spacing, tok.radius, tok.style)(toastGtx)
		macros[vis] = macro.Stop()
		sizes[vis] = dims.Size
	}

	var y int
	if topAnchored {
		y = edgePad
	} else {
		total := 0
		for i, s := range sizes {
			total += s.Y
			if i > 0 {
				total += gap
			}
		}
		y = frame.Y - edgePad - total
	}

	for vis := range order {
		// A notification past its lifetime keeps its place in the column
		// and paints nothing, so the queue on screen and the queue in the
		// model stay the same list.
		if alphas[vis] > 0 {
			off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
			depth.Shadow(gtx, image.Rectangle{Max: sizes[vis]}, radius, fadedShadow(tok.color, float32(alphas[vis])))
			macros[vis].Add(gtx.Ops)
			off.Pop()
		}
		y += sizes[vis].Y + gap
	}

	return layout.Dimensions{Size: frame}
}

// fadeAlpha returns a toast's alpha in [0,1] for a frame at now. A zero at
// (the Render path, or a hand-built Notification) means "fully opaque".
// Otherwise the alpha tweens from 1.0 to 0.0 across the final fade window
// (the theme's DurSlow stop) of the lifetime via effects/tween.LerpFloat64,
// and stays at 0 past expiry while the Expired message travels the loop; a
// zero fade window paints fully opaque until then.
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

// fadedShadow is the platform's floating shadow at the share of its own
// coverage a toast on its way in or out is showing, which is how a surface
// that fades takes its shadow with it.
func fadedShadow(c tokens.PlatformColors, share float32) color.NRGBA {
	s := c.FloatingShadow
	s.A = uint8(float32(s.A)*share + 0.5)
	return s
}
