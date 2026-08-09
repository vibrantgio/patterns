// Package modal provides the Cadence Modal pattern: a centered elevated
// surface dialog over a full-window scrim backdrop, with a header (title +
// close affordance), padded body, and optional footer action row.
//
// Elevation (goal G-E2): the dialog surface fills at SurfaceAt(Level2)
// (Neutral step 300), one storey above the standing level-1 content
// panes it covers, with the 1 dp Neutral step-500 stroke. Level 2 — not
// the deeper level 3 that unscrimmed overlays (popover, dropdown menu)
// take — because the modal does not separate by fill alone: the scrim
// dims everything beneath it and is the modal's isolating cue, so its
// surface needs only one tonal storey. No shadow: E2.2 reserved cast
// shadows for surfaces that float and can leave without a scrim.
//
// The package follows the Phase 4 Composition contract: Modal is a callable
// Go function consuming a Prism theme observable, returning a stream of
// layout.Widget. The source is intentionally short and free of opaque
// configuration — copy it into your own app and modify as needed.
//
// # The dialog grammar
//
// Desktop dialogs come in two archetypes, and this package has exactly two.
// Which one you get is derived from [Props.Decision] — nil or not — and
// reported by [Props.Intent]. Everything below follows from that one word;
// none of it is separately configurable, because the wrong combinations are
// what a boolean per affordance would let you write down.
//
// A PANEL ([IntentPanel], the zero value) is a place you opened and can
// leave. It MANDATES a ghost close X top-right, a backdrop click that
// invokes Props.OnClose, and Escape likewise: leaving costs nothing, so
// every cheap exit is offered. It FORBIDS claiming Return, which belongs to
// whatever holds focus inside it. It has no footer of its own — a panel's
// changes apply live, which is the reason it can be left at any moment. If
// you find yourself adding a Save button to a panel, you have a decision.
//
// A DECISION ([IntentDecision], [Props.Decision] non-nil) is a question you
// must answer. It MANDATES right-aligned footer actions ending in a default
// that answers Return ([Decision.DefaultAction]), and Escape bound to
// [Decision.Cancel]. It FORBIDS an X anywhere and an active backdrop: the
// backdrop still absorbs presses so nothing behind it is reached, but it
// answers none of them, because dismissal is itself one of the answers and a
// stray click must not give it. It also forbids Return reaching a
// destructive primary — see [Decision] for why that is a matter of the
// struct's shape rather than of documentation.
//
// # Arrival is not this package's business
//
// The modal owns dismissal. It does not own how you arrived, and it has no
// notion of an accelerator: the ⌘,/Ctrl-, that opens a settings panel has to
// be live when no modal exists, which is precisely the state a modal is not
// in. Bind it in app chrome, with Gio's own key.ModShortcut (Cmd on darwin,
// Ctrl elsewhere) rather than a GOOS test of your own, and land a message
// that flips the flag [Props.Open] reads. The reference implementation is
// workbench's feeds app — shortcut.go for the binding, preferences.go for
// the panel it opens.
//
// Tab and Shift+Tab cycle keyboard focus within the modal's focusable items
// and do not escape to background content. Only the topmost modal on the
// coordination stack receives input; modals underneath remain painted but
// inert.
//
// Focus ownership: the close affordance and each footer action own their own
// focus tag and focus ring (the close button is a prism/button; actions
// likewise register their own tags, e.g. a prism/button's caller-owned
// *widget.Clickable). The modal does not wrap an action or draw a ring around
// it — it only adds the caller-declared Props.ActionFocusTags to its Tab cycle
// (route (a)), so a focused action shows exactly one ring: its own.
//
// Open/close is instantaneous in this package; entrance/exit transitions
// are deferred to a later Pulse-integration goal.
package modal

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/prism/button"
	pllayout "github.com/vibrantgio/prism/layout"
	"github.com/vibrantgio/spectrum/theme"
	"github.com/vibrantgio/spectrum/tokens"
	"github.com/vibrantgio/spectrum/typeset"
)

// Intent names the archetype a modal belongs to. Desktop dialogs come in
// two, and their affordances travel together rather than varying
// independently:
//
//   - A PANEL is a place you opened and can leave. Obsidian's and
//     Claude.app's settings are the shape: a small quiet X top-right,
//     Escape and a backdrop click both close it, changes apply live so a
//     footer is optional, and an app accelerator (⌘,) opens it. Leaving
//     costs nothing, so every cheap exit is offered.
//
//   - A DECISION is a question you must answer. Apple's alerts are the
//     shape: right-aligned footer actions ending in a default that answers
//     Return, Escape bound to Cancel, no X anywhere, and an inert backdrop.
//     Dismissal is itself one of the answers, so a stray click must not
//     give it.
//
// Exposing those affordances as independent booleans would permit every
// wrong combination, including the one this package used to render: a
// "Discard changes?" dialog wearing a panel's X over a backdrop that
// dismissed it. So there are no such booleans. The intent is declared once,
// by supplying [Props.Decision] or leaving it nil, and every affordance is
// derived from it — which is why a decision dialog with a dismissing
// backdrop cannot be written down in this API at all.
//
// Intent is a derived value, not a field: [Props.Intent] reports it. There
// is nothing to keep in sync and nothing that can contradict the callbacks.
type Intent int

const (
	// IntentPanel is the dismissable panel: a ghost close X, a dismissing
	// backdrop, Escape closes, Return unclaimed. It is the zero value, so
	// every Props written before this axis existed keeps its behaviour.
	IntentPanel Intent = iota

	// IntentDecision is the decision dialog: no X, an inert backdrop,
	// Escape invokes Cancel, Return activates the default action.
	IntentDecision
)

// String returns the archetype's name in the vocabulary the design system
// uses everywhere else.
func (i Intent) String() string {
	switch i {
	case IntentPanel:
		return "panel"
	case IntentDecision:
		return "decision"
	}
	return fmt.Sprintf("Intent(%d)", int(i))
}

// Decision turns a modal into a decision dialog. Set it on [Props.Decision]
// and the modal drops its close X, stops dismissing on a backdrop click, and
// binds the two keys desktop conventions reserve for a dialog: Escape to
// Cancel and Return to the default action.
//
// The callbacks here are the KEYBOARD bindings, not the footer. The footer is
// still Props.Actions (with Props.ActionFocusTags), because an action's widget
// is the caller's — a prism/button, a link, whatever the dialog needs. List
// the same two actions in both places: Cancel and the primary, in that order,
// right-aligned, the way both platforms order them.
//
// A decision dialog needs at least one focusable action in ActionFocusTags.
// The modal's key bindings are registered against the tags it holds focus
// with, so a decision that declares none is a dialog no key reaches.
//
// # The default action, and why it is derived
//
// Apple's rule, adopted wholesale: a destructive primary is never the
// Return-bound default. When the primary destroys something, Cancel takes the
// default — "Discard changes?" answering Return with Discard is the exact
// failure this forbids.
//
// That rule is enforced by the shape of this struct rather than by
// documentation alone: there is no field with which to nominate the default.
// A caller says what the primary DOES (Confirm) and whether it destroys
// (Destructive); [Decision.DefaultAction] derives which callback Return
// reaches. Binding Return to a destructive primary is not a mistake you can
// make here, because it is not a sentence this API can spell.
type Decision struct {
	// Confirm is the primary action — the one the dialog is asking for.
	// It is the Return-bound default unless Destructive says otherwise.
	Confirm func(gtx layout.Context)

	// Destructive marks Confirm as destroying something the user cannot get
	// back: discarding edits, deleting a record, ending a session. It moves
	// the Return binding to Cancel and nothing else — the footer's own
	// colours are the caller's to choose (prism/button's emphasis axis, or a
	// role colour).
	Destructive bool

	// Cancel is the dismissing answer: what Escape invokes, and what Return
	// invokes when Confirm is destructive. A nil Cancel falls back to
	// Props.OnClose for Escape, so Escape always leaves.
	Cancel func(gtx layout.Context)
}

// DefaultAction returns the callback Return activates: Confirm normally,
// Cancel when Confirm is destructive.
//
// It returns nil when the destructive case has no Cancel to fall back to.
// Nil means Return does nothing — the modal still CLAIMS the key, so it
// cannot fall through to a focused destructive button. A dead Return is the
// safe end of that trade.
func (d Decision) DefaultAction() func(gtx layout.Context) {
	if d.Destructive {
		return d.Cancel
	}
	return d.Confirm
}

// Props configures a Modal. Body and OnClose may both be nil; Actions may
// contain nil entries (skipped). Title may be empty (the header still
// renders the close affordance).
type Props struct {
	// Open emits true to show the modal and false to hide it. A nil Open
	// is treated as a constant false (modal never opens).
	Open rx.Observable[bool]

	Title   string
	Body    layout.Widget
	OnClose func(gtx layout.Context)
	Actions []layout.Widget

	// Decision, when non-nil, makes this a decision dialog rather than a
	// dismissable panel: no close X, an inert backdrop, Escape to Cancel and
	// Return to the default action. Leave it nil for a panel. See [Intent]
	// for the two archetypes and [Decision] for the destructive-default rule.
	Decision *Decision

	// HideClose, when true, omits the top-right close button on a PANEL. Use
	// it when the footer Actions already provide explicit dismissal (e.g. a
	// Cancel button) — Escape and a scrim click still trigger OnClose.
	//
	// It is not consulted on a decision dialog, which never draws the X:
	// hiding the close affordance is derived from the intent there, not
	// requested. HideClose is therefore only ever additive — it can hide a
	// panel's X, and it can never show a decision's.
	//
	// Deprecated: a modal that hides its X because its footer answers for it
	// is describing a decision dialog. Say so with Decision and get the inert
	// backdrop and the key bindings that belong with it; the X goes away on
	// its own. HideClose keeps working for panels that genuinely want no X.
	HideClose bool

	// DynamicFocusTags, if non-nil, is called every frame and its tags join
	// the Tab cycle after the close button and BEFORE ActionFocusTags. Use
	// it for focusables whose tags change across the modal's lifetime —
	// e.g. a prism TextField rebuilt per open (its editor tag, exposed via
	// TextFieldProps.FocusTag, is new each rebuild). The first tag in the
	// cycle receives initial focus when the modal opens.
	DynamicFocusTags func() []event.Tag

	// ActionFocusTags lists the focus tags of the focusable Actions, in the
	// order they should join the modal's Tab cycle (after the close button).
	//
	// Footer actions own their own focus tags and focus ring — the modal does
	// not wrap an action or draw a ring around it. A prism/button action, for
	// example, is built with a caller-owned *widget.Clickable; passing that
	// &clickable here adds it to the Tab cycle (and the Escape trap) with no
	// doubled outer ring. A non-focusable action (plain widget) simply omits
	// its tag. nil entries are skipped. See ActionFocusTags vs Actions: the
	// two slices are independent — list a tag here only for actions that
	// participate in keyboard focus.
	ActionFocusTags []event.Tag

	// Shaper is an explicit per-instance override of the text shaper. Leave
	// it nil in normal use: the modal then shapes its title with the theme's
	// shaper (Typography.Shaper()), which is built once for the process and
	// shared by every component reading that typography — the cache lives
	// behind the Typography value, so it survives the copy this component's
	// map function makes of it (spectrum F5.1), and the close button
	// likewise falls back to the theme shaper. Set it only when this
	// instance must shape with a different shaper than the theme provides.
	//
	// A shaper is not safe to use from two goroutines; Gio lays the widget
	// forest out on the one goroutine that runs the event loop, which is
	// what makes sharing it correct. See spectrum/tokens.Typography.Shaper.
	Shaper *text.Shaper
}

// Intent reports which archetype these Props describe. It is derived from
// Decision alone, so it can never disagree with the callbacks.
func (p Props) Intent() Intent {
	if p.Decision != nil {
		return IntentDecision
	}
	return IntentPanel
}

// showsClose reports whether the header draws the close X. A decision dialog
// never does; a panel does unless HideClose says otherwise.
func (p Props) showsClose() bool {
	return p.Intent() == IntentPanel && !p.HideClose
}

// dismissOnBackdrop reports whether a press on the scrim invokes OnClose.
// Only a panel dismisses that way: on a decision dialog dismissal is one of
// the answers, and a stray click must not give it.
func (p Props) dismissOnBackdrop() bool {
	return p.Intent() == IntentPanel
}

// onEscape returns the callback Escape invokes: Decision.Cancel on a decision
// dialog, OnClose on a panel — and OnClose too when a decision names no
// Cancel, so Escape always leaves.
func (p Props) onEscape() func(gtx layout.Context) {
	if p.Decision != nil && p.Decision.Cancel != nil {
		return p.Decision.Cancel
	}
	return p.OnClose
}

type resolvedTokens struct {
	color   tokens.ColorTokens
	spacing tokens.SpacingScale
	radius  tokens.RadiusScale
	title   tokens.TextStyle // the TitleMedium role: typeface, weight, size, line height
	shaper  *text.Shaper     // the theme's shaper; nil in the Render path
	// elevation is snapshotted so a theme elevation change re-emits the
	// widget; the surface fill resolves through SurfaceAt, which reads
	// the default tokens.Elevation scale.
	elevation tokens.ElevationScale
}

// Modal returns an rx.Observable[layout.Widget] that emits a new widget
// whenever the theme or Open state changes. The widget renders a scrim and
// centered surface when open, or no pixels at all when closed. State
// (focus tags, the modal-stack id, the close-button clickable) persists
// across emissions in the rx.Defer scope.
func Modal(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget] {
	open := props.Open
	if open == nil {
		open = rx.Of(false)
	}

	// Flatten the nested theme observables into a concrete snapshot. The
	// typography emission supplies both the TitleMedium text style and the
	// theme's cached shaper (ADR-003: the theme owns the typeface).
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[resolvedTokens] {
		return rx.Map(
			rx.CombineLatest5(t.Color, t.Spacing, t.Radius, t.Typography, t.Elevation),
			func(n rx.Tuple5[tokens.ColorTokens, tokens.SpacingScale, tokens.RadiusScale, tokens.Typography, tokens.ElevationScale]) resolvedTokens {
				typ := n.Fourth
				return resolvedTokens{
					color:     n.First,
					spacing:   n.Second,
					radius:    n.Third,
					title:     typ.TitleMedium,
					shaper:    typ.Shaper(),
					elevation: n.Fifth,
				}
			},
		)
	})

	return rx.Defer(func() rx.Observable[layout.Widget] {
		st := newState()

		// The close affordance is a GHOST prism/button icon-only variant — a
		// panel's X is present without being the subject, which is what the
		// filled square it used to be got wrong: it out-weighed the title
		// beside it. The modal owns its clickable (&st.closeClick) so the
		// focus trap stays keyed to a single tag and no doubled focus ring is
		// drawn; OnClose is routed through the button's OnClick. Build once
		// here in the rx.Defer scope and fold the latest emitted widget into
		// the input pipeline — never subscribe inside the per-frame widget
		// closure.
		closeBtn := rx.Of[layout.Widget](nil)
		if props.showsClose() {
			closeBtn = button.Button(th, button.Props{
				Icon:        crossIcon,
				Description: "Close",
				Emphasis:    button.Ghost,
				Clickable:   &st.closeClick,
				OnClick:     props.OnClose,
				// Pass the override through untouched: a nil Shaper lets
				// the button default to the theme's shaper on its own.
				Shaper: props.Shaper,
			})
		}

		inputs := rx.CombineLatest3(resolved, open, closeBtn)

		return rx.Map(inputs, func(next rx.Tuple3[resolvedTokens, bool, layout.Widget]) layout.Widget {
			tok, openNow, closeW := next.First, next.Second, next.Third

			// Props.Shaper is an explicit override; the theme's shaper is
			// the default.
			shaper := props.Shaper
			if shaper == nil {
				shaper = tok.shaper
			}

			// Transition tracking — push on open, pop on close.
			if openNow && !st.pushed {
				stackPush(st.id)
				st.pushed = true
				st.wantInitialFocus = true
			}
			if !openNow && st.pushed {
				stackPop(st.id)
				st.pushed = false
			}

			return func(gtx layout.Context) layout.Dimensions {
				if !openNow {
					return layout.Dimensions{Size: gtx.Constraints.Max}
				}
				live := isTop(st.id)
				return drawModal(gtx, shaper, props, tok, st, live, closeW)
			}
		})
	})
}

// Render produces a layout.Widget for a modal with pre-resolved tokens and
// an explicit open flag. Intended for golden-image testing and static
// demonstrations; production code should use Modal, which reads both of the
// parameters below off the theme. The returned widget performs no input
// handling: pass open=true to render the scrim and surface, open=false to
// render nothing (the widget consumes the constraints but paints no pixels).
//
// title is the TitleMedium role's whole text style — typeface, weight, size
// and line height all reach the shaper — and d is the density the close
// button draws at. It is the one thing density sizes here: the surface
// itself is content-sized, and the close affordance is an icon-only
// prism/button, which takes a density and no text style at all. Pass
// tokens.DefaultTypography.TitleMedium and tokens.Comfortable for the
// default desktop look. On a decision dialog there is no close affordance to
// size: the intent removes it, here as on the live path, so the two render
// the same header.
func Render(
	shaper *text.Shaper,
	props Props,
	open bool,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	title tokens.TextStyle,
	d tokens.Density,
) layout.Widget {
	tok := resolvedTokens{color: colors, spacing: sp, radius: rad, title: title}
	st := newState()
	// Static, inert close affordance: the same icon painter and the same GHOST
	// register the live path uses, rendered through button.RenderIcon so
	// goldens stay text-free and deterministic. Radius is threaded straight
	// through (callers pass a sharp radius for golden determinism).
	var closeW layout.Widget
	if props.showsClose() {
		closeW = button.RenderIcon(crossIcon, colors, sp, rad, d, button.RenderState{Emphasis: button.Ghost})
	}
	return func(gtx layout.Context) layout.Dimensions {
		if !open {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
		return drawModal(gtx, shaper, props, tok, st, false, closeW)
	}
}

// modalState holds per-subscription stable tags and the open-flag tracker.
// One instance is owned by each Modal subscription (and by static Render
// invocations, where focus and input handling are inert).
type modalState struct {
	id               int64
	pushed           bool
	wantInitialFocus bool

	// Stable tags so the router can route events across frames. The close
	// button's clickable doubles as its focus tag (driven by prism/button).
	// Footer actions own their own focus tags (Props.ActionFocusTags); the
	// modal holds none on their behalf.
	scrimTag   int
	surfaceTag int
	closeClick widget.Clickable
}

func newState() *modalState {
	return &modalState{id: allocStackID()}
}

// focusCount returns the number of focusable elements: the close button
// (unless hidden) plus one per non-nil caller-declared action focus tag.
func focusCount(props Props) int {
	n := 0
	if props.showsClose() {
		n = 1
	}
	for _, t := range props.ActionFocusTags {
		if t != nil {
			n++
		}
	}
	return n
}

// drawModal paints the scrim, centered surface, header (title + close),
// body, and footer actions, then processes input when live is true.
func drawModal(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	tok resolvedTokens,
	st *modalState,
	live bool,
	closeWidget layout.Widget,
) layout.Dimensions {
	canvas := gtx.Constraints.Max
	r := gtx.Dp(unit.Dp(tok.radius.Lg))
	gap := gtx.Dp(unit.Dp(tok.spacing.S3))

	// The default action is drained BEFORE the footer lays out, and that
	// ordering is the whole mechanism. Gio delivers a key event to the first
	// caller whose filter matches and then removes it from the queue, and a
	// focused widget.Clickable filters Return on its own tag — so whoever
	// asks first wins. Asking here gives the desktop rule both platforms
	// share: Return activates the DEFAULT action whatever holds focus, while
	// Space activates the FOCUSED control. A panel claims neither key.
	if live {
		processDefaultAction(gtx, props, st)
	}

	// Scrim — full-canvas dimmer. Pointer events that miss the surface
	// hit the scrim tag and trigger OnClose.
	scrimColor := scrimColor(tok.color)
	scrimRect := image.Rectangle{Max: canvas}
	scrimClip := clip.Rect(scrimRect).Push(gtx.Ops)
	paint.FillShape(gtx.Ops, scrimColor, clip.Rect(scrimRect).Op())
	if live {
		event.Op(gtx.Ops, &st.scrimTag)
	}
	scrimClip.Pop()

	// Surface width — 75% of canvas clamped to a sensible min/max in dp.
	// Height HUGS THE CONTENT: the content is laid out once into a macro
	// (stateful widgets process their events exactly once), the surface is
	// sized to the recorded dims, and the macro is replayed inside the
	// positioned surface. maxH caps the surface at the old 75%-of-canvas
	// bound; overflowing content is clipped to the surface.
	surfW := clampInt(canvas.X*3/4, gtx.Dp(unit.Dp(180)), gtx.Dp(unit.Dp(560)))
	if surfW > canvas.X {
		surfW = canvas.X
	}
	// 560dp (not the historical 420) so tall forms — e.g. an alert plus
	// four fields plus actions — fit before the overflow clip engages.
	maxH := clampInt(canvas.Y*3/4, gtx.Dp(unit.Dp(120)), gtx.Dp(unit.Dp(560)))
	if maxH > canvas.Y {
		maxH = canvas.Y
	}
	inset := gtx.Dp(unit.Dp(tok.spacing.S5))

	contentGtx := gtx
	contentGtx.Constraints = layout.Constraints{
		Min: image.Pt(surfW-2*inset, 0),
		Max: image.Pt(surfW-2*inset, maxH-2*inset),
	}
	contentMacro := op.Record(gtx.Ops)
	contentDims := drawSurfaceContents(contentGtx, shaper, props, tok, gap, closeWidget)
	content := contentMacro.Stop()

	surfH := clampInt(contentDims.Size.Y+2*inset, gtx.Dp(unit.Dp(120)), maxH)
	surfPos := image.Pt((canvas.X-surfW)/2, (canvas.Y-surfH)/2)

	// Surface — rounded rectangle, registered as a pointer absorber so
	// presses on its area do not reach the scrim and dismiss the modal.
	// Level 2 on the elevation ladder: one tonal storey above the level-1
	// panes underneath; the scrim, not the fill, is the isolating cue
	// (see the package doc).
	off := op.Offset(surfPos).Push(gtx.Ops)
	surfRRect := clip.RRect{Rect: image.Rectangle{Max: image.Pt(surfW, surfH)}, SE: r, SW: r, NE: r, NW: r}
	paint.FillShape(gtx.Ops, tok.color.SurfaceAt(tokens.Level2), surfRRect.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, tok.color.Ramps.Neutral.Step(500), clip.Stroke{
		Path:  surfRRect.Path(gtx.Ops),
		Width: float32(gtx.Dp(unit.Dp(1))),
	}.Op())

	// Absorb pointer events on the surface.
	if live {
		absorbClip := clip.Rect{Max: image.Pt(surfW, surfH)}.Push(gtx.Ops)
		event.Op(gtx.Ops, &st.surfaceTag)
		absorbClip.Pop()
	}

	// Surface inset content (header / body / footer) — the macro recorded
	// above, replayed at the inset origin and clipped to the surface.
	contentClip := clip.Rect{Max: image.Pt(surfW, surfH)}.Push(gtx.Ops)
	contentOff := op.Offset(image.Pt(inset, inset)).Push(gtx.Ops)
	content.Add(gtx.Ops)
	contentOff.Pop()
	contentClip.Pop()
	off.Pop()

	if live {
		processInput(gtx, props, st)
	}

	return layout.Dimensions{Size: canvas}
}

// drawSurfaceContents lays out the header row, the body, and the footer
// action row vertically inside the already-inset surface gtx.
func drawSurfaceContents(
	gtx layout.Context,
	shaper *text.Shaper,
	props Props,
	tok resolvedTokens,
	gap int,
	closeWidget layout.Widget,
) layout.Dimensions {
	header := headerWidget(shaper, props, tok, closeWidget)
	footer := footerWidget(props, tok)

	children := []layout.FlexChild{
		layout.Rigid(header),
		layout.Rigid(spacerV(gap)),
	}
	if props.Body != nil {
		children = append(children, layout.Rigid(props.Body))
	}
	if footer != nil {
		children = append(children, layout.Rigid(spacerV(gap)))
		children = append(children, layout.Rigid(footer))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// headerWidget renders the title (drawn only when non-empty) on the left and
// the close affordance — a prism/button icon variant, built upstream and
// threaded in as closeWidget — on the right. The button owns its own focus
// ring and click handling; the header only positions it.
func headerWidget(shaper *text.Shaper, props Props, tok resolvedTokens, closeWidget layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		titleFlex := layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if props.Title == "" {
				// Empty title contributes no height; the Rigid close button
				// drives the header row height via Middle alignment.
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
			}
			mColor := op.Record(gtx.Ops)
			paint.ColorOp{Color: tok.color.Text}.Add(gtx.Ops)
			material := mColor.Stop()
			// Shape with the TitleMedium role's typeface, weight, size and
			// line height. The legacy Render path synthesizes a size-only
			// style; its zero weight falls back to SemiBold so the title
			// keeps its pre-Typography emphasis against the body.
			style := tok.title
			f := typeset.Font(style, font.SemiBold)
			wl := typeset.Label(style, 1)
			return typeset.Layout(gtx, shaper, wl, f, unit.Sp(style.Size), props.Title, material)
		})
		if closeWidget == nil {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, titleFlex)
		}
		closeFlex := layout.Rigid(closeWidget)
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, titleFlex, closeFlex)
	}
}

// footerWidget renders a right-aligned row of action widgets. Each action is
// laid out bare: it owns its own focus tag and focus ring (the modal neither
// wraps it nor decorates it), and joins the Tab cycle via Props.ActionFocusTags.
// Returns nil when there are no non-nil actions.
func footerWidget(props Props, tok resolvedTokens) layout.Widget {
	any := false
	for _, a := range props.Actions {
		if a != nil {
			any = true
			break
		}
	}
	if !any {
		return nil
	}
	gap := tok.spacing.S2

	return func(gtx layout.Context) layout.Dimensions {
		// The right-alignment filler must claim NO cross-axis height: with a
		// content-sized surface the row's Max.Y is all remaining space, and
		// a Constraints.Max-sized filler would inflate the footer to fill it
		// (the actions would float mid-surface over a sea of empty space).
		filler := func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
		}
		children := []layout.FlexChild{layout.Flexed(1, filler)}
		first := true
		for _, a := range props.Actions {
			a := a
			if a == nil {
				continue
			}
			if !first {
				children = append(children, layout.Rigid(pllayout.HSpacer(gap)))
			}
			first = false
			children = append(children, layout.Rigid(a))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

// processInput drains the scrim, surface, close-button, escape, and tab
// events generated this frame and dispatches them: scrim/close/escape →
// OnClose, tab/shift-tab → cycle focus among the registered modal tags.
func processInput(gtx layout.Context, props Props, st *modalState) {
	// Drain FocusFilter events for each focus tag so the router retains focus
	// when set, mirroring prism/layout.FocusGroup.Update.
	tags := focusTags(props, st)
	for _, tag := range tags {
		for {
			if _, ok := gtx.Event(key.FocusFilter{Target: tag}); !ok {
				break
			}
		}
	}

	// Set initial focus to the close button on the first frame after Open
	// transitions to true. Subsequent transitions are tracked by the rx
	// pipeline; here we just consume the flag.
	if st.wantInitialFocus {
		// tags can be empty (HideClose with no action tags) — nothing to
		// focus then, but never panic.
		if len(tags) > 0 {
			gtx.Execute(key.FocusCmd{Tag: tags[0]})
		}
		st.wantInitialFocus = false
	}

	// Backdrop click → OnClose, on a PANEL only. A decision dialog drains
	// the presses (so they reach nothing behind the scrim) and answers
	// none of them: dismissal is one of the decision's answers, and a stray
	// click must not make it for you.
	dismiss := props.dismissOnBackdrop()
	for {
		e, ok := gtx.Event(pointer.Filter{Target: &st.scrimTag, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pe, ok := e.(pointer.Event); ok && pe.Kind == pointer.Press && dismiss {
			fire(gtx, props.OnClose)
		}
	}

	// Drain surface presses so they are not re-dispatched anywhere else.
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &st.surfaceTag, Kinds: pointer.Press}); !ok {
			break
		}
	}

	// The close button is a prism/button instance: it drains its own
	// Clicked() and invokes props.OnClose via Props.OnClick. The modal must
	// NOT also check st.closeClick.Clicked here — the button has already
	// consumed the event, so this check would always be false.

	// Escape → OnClose on a panel, Decision.Cancel on a decision dialog
	// (falling back to OnClose when the decision names no Cancel, so Escape
	// always leaves). Register the filter against every modal focus tag so
	// the event fires whenever any modal element has focus.
	escape := props.onEscape()
	for _, tag := range tags {
		for {
			e, ok := gtx.Event(key.Filter{Focus: tag, Name: key.NameEscape})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				fire(gtx, escape)
			}
		}
	}

	// Tab / Shift+Tab focus cycling among modal tags. Registering the
	// filter with Focus: tag traps Tab before the router's default
	// MoveFocus advances focus to background content.
	curIdx := currentFocusIdx(gtx, tags)
	for i, tag := range tags {
		for {
			e, ok := gtx.Event(key.Filter{Focus: tag, Name: key.NameTab, Optional: key.ModShift})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				dir := 1
				if ke.Modifiers.Contain(key.ModShift) {
					dir = -1
				}
				n := len(tags)
				if n == 0 {
					continue
				}
				base := curIdx
				if base < 0 {
					base = i
				}
				nextIdx := (base + dir + n) % n
				gtx.Execute(key.FocusCmd{Tag: tags[nextIdx]})
				curIdx = nextIdx
			}
		}
	}
}

// processDefaultAction claims Return (and the numeric keypad's Enter) for a
// decision dialog's default action. It is a no-op for a panel, which leaves
// both keys to whatever holds focus.
//
// The claim is unconditional within a decision dialog — it happens even when
// [Decision.DefaultAction] resolves to nil, which is the destructive-primary
// case with no Cancel. Letting Return fall through there would hand it to
// whichever footer button has focus, and on a "Discard changes?" dialog that
// is precisely the destruction Apple's rule forbids Return to reach. A dead
// Return is the safe end of the trade.
func processDefaultAction(gtx layout.Context, props Props, st *modalState) {
	if props.Decision == nil {
		return
	}
	act := props.Decision.DefaultAction()
	for _, tag := range focusTags(props, st) {
		for {
			e, ok := gtx.Event(
				key.Filter{Focus: tag, Name: key.NameReturn},
				key.Filter{Focus: tag, Name: key.NameEnter},
			)
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				fire(gtx, act)
			}
		}
	}
}

// focusTags returns the ordered slice of focus tags belonging to this modal:
// the close button first, then the caller-declared action focus tags. Action
// tags are owned by the action widgets themselves (Props.ActionFocusTags); the
// modal only sequences them for Tab cycling and the Escape trap.
func focusTags(props Props, st *modalState) []event.Tag {
	tags := make([]event.Tag, 0, focusCount(props))
	if props.showsClose() {
		tags = append(tags, &st.closeClick)
	}
	if props.DynamicFocusTags != nil {
		for _, t := range props.DynamicFocusTags() {
			if t != nil {
				tags = append(tags, t)
			}
		}
	}
	for _, t := range props.ActionFocusTags {
		if t != nil {
			tags = append(tags, t)
		}
	}
	return tags
}

// currentFocusIdx returns the index of the currently-focused modal tag,
// or -1 if no modal element holds focus.
func currentFocusIdx(gtx layout.Context, tags []event.Tag) int {
	for i, tag := range tags {
		if gtx.Focused(tag) {
			return i
		}
	}
	return -1
}

// crossIcon paints an "×" shape — two diagonal strokes — into a
// sizePx×sizePx box at the current origin in colour col. It is the modal
// close button's glyph, satisfying the button.Props.Icon painter contract
// (clip.Path / clip.Stroke only — no font or SVG rasterisation) so goldens
// stay deterministic across GPU contexts.
func crossIcon(gtx layout.Context, sizePx int, col color.NRGBA) {
	w := float32(sizePx)
	h := float32(sizePx)
	pad := float32(gtx.Dp(unit.Dp(6)))
	stroke := float32(gtx.Dp(unit.Dp(2)))
	if stroke < 1 {
		stroke = 1
	}
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(pad, pad))
	p.LineTo(f32.Pt(w-pad, h-pad))
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: p.End(), Width: stroke}.Op())

	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(w-pad, pad))
	p.LineTo(f32.Pt(pad, h-pad))
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: p.End(), Width: stroke}.Op())
}

// scrimColor returns a translucent dim laid over the scene background.
// Light themes get a black scrim; dark themes also use black for
// consistency with material-style scrims that dim by reducing luminance.
func scrimColor(_ tokens.ColorTokens) color.NRGBA {
	return color.NRGBA{R: 0, G: 0, B: 0, A: 0x80}
}

// spacerV returns a vertical-spacer widget that consumes hPx pixels in
// the Y axis and zero pixels in X. Used inside the vertical Flex stack.
func spacerV(hPx int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(0, hPx)}
	}
}

// fire invokes cb when cb is non-nil. Centralised so OnClose is never
// called against a nil pointer.
func fire(gtx layout.Context, cb func(gtx layout.Context)) {
	if cb != nil {
		cb(gtx)
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
