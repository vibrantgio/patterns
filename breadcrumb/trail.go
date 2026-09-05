package breadcrumb

import (
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Segment is one segment of a trail handed over per frame. Label and OnClick
// are the same two facts Item carries — the segment is clickable exactly when
// its own OnClick is non-nil, and the trailing segment by position still takes
// the current-location colour whatever its OnClick says.
//
// Key is the addition, and it is what makes a trail safe to reshape between
// frames: it names the place the segment stands for, stably, for as long as
// that place keeps appearing in the trail. A path, an id, a node's name — any
// string the caller can produce again next frame for the same destination.
// Key is never drawn.
//
// A Segment with an empty Key takes its Label as its identity, which is right
// while labels are the path and wrong as soon as two places share a label; a
// trail whose segments can collide should say so with a Key.
type Segment struct {
	Key     string
	Label   string
	OnClick func(gtx layout.Context)
}

// identity is the string a segment's clicks are addressed to.
func (s Segment) identity() string {
	if s.Key != "" {
		return s.Key
	}
	return s.Label
}

// TrailLayout draws one frame of a trail: the caller passes the segments as
// they stand this frame, and gets back the row's Dimensions. An empty
// segments lays out to zero Dimensions, as an empty Items does.
//
// The value holds the trail's interaction state and must be kept across
// frames — built once where the window is composed, called every frame. A
// TrailLayout built inside the frame it draws has no state older than itself
// and can therefore never report a click, because a click is only ever
// delivered on the frame after the one that drew what was clicked.
type TrailLayout func(gtx layout.Context, segments []Segment) layout.Dimensions

// TrailProps configures a Trail. The segments are not here: they are the
// argument to each frame's TrailLayout.
type TrailProps struct {
	// Shaper is an explicit per-instance override of the text shaper. Leave
	// it nil in normal use: the trail then shapes its labels with the theme's
	// shaper (Typography.Shaper()), exactly as Props.Shaper describes.
	Shaper *text.Shaper

	// Chevron is the square each separator is drawn in, exactly as
	// Props.Chevron describes it. Zero takes DefaultChevron.
	Chevron unit.Dp
}

// Trail returns an rx.Observable[TrailLayout] that emits a new layout function
// whenever any consumed theme token changes. Every emission shares the one
// interaction state held for the subscription, so a token change mid-gesture
// costs no clicks.
//
// This is the frame-time counterpart of Breadcrumb: Breadcrumb fixes its items
// and their clickables when the stream is built, which is what a trail known
// up front wants; Trail fixes neither, for a trail that is decided again on
// every frame.
func Trail(th rx.Observable[theme.Theme], props TrailProps) rx.Observable[TrailLayout] {
	resolved := resolveTokens(th)

	return rx.Defer(func() rx.Observable[TrailLayout] {
		st := new(trailState)

		return rx.Map(resolved, func(tok resolvedTokens) TrailLayout {
			// Props.Shaper is an explicit override; the theme's shaper is
			// the default.
			shaper := props.Shaper
			if shaper == nil {
				shaper = tok.shaper
			}
			return func(gtx layout.Context, segments []Segment) layout.Dimensions {
				return st.layout(gtx, shaper, segments, tok.color, tok.spacing, tok.label, props.Chevron)
			}
		})
	})
}

// NewTrail returns a TrailLayout with pre-resolved tokens, for callers that
// resolve the theme themselves and for tests. It is the frame-time
// counterpart of Render, with one difference the name is meant to warn about:
// Render is stateless and may be built inside the frame that calls it, while
// the value NewTrail returns owns the trail's clickables and must outlive the
// frame — see TrailLayout.
//
// props carries the configuration that is not a token, as it does for
// Render; its Shaper is unread here, the shaper being the first argument.
//
// label is the TitleSmall role's whole text style, as it is for Render; pass
// tokens.DefaultTypography.TitleSmall for the default desktop look.
func NewTrail(
	shaper *text.Shaper,
	props TrailProps,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	label tokens.TextStyle,
) TrailLayout {
	st := new(trailState)
	return func(gtx layout.Context, segments []Segment) layout.Dimensions {
		return st.layout(gtx, shaper, segments, colors, sp, label, props.Chevron)
	}
}

// trailState is what a frame-time trail keeps between frames: one
// widget.Clickable per segment identity, retained for as long as the identity
// keeps appearing in the trail.
//
// # Identity is the segment's key, never its position
//
// A click is delivered to the frame after the one that drew the segment
// clicked, and by then the trail may be a different trail — a directory up, a
// sibling branch, one segment shorter. Gio addresses a queued pointer event to
// the tag the previous frame registered, so keying this state by identity
// makes that address the place the user clicked: the click on "Design"
// reaches Design's own clickable however the trail has since been reshaped,
// and reaches nothing at all if the caller has since given Design a different
// key. Keying by position instead — which is correct for Breadcrumb, whose
// trail cannot change — would hand the click to whichever segment now stands
// where the clicked one stood, and that is a navigation to somewhere the user
// never clicked.
//
// Two segments resolving to one identity are one affordance: they share a
// clickable, the first of them decides whether it is clickable and what it
// does, and a click fires once. One identity is one place, so going there
// twice is the same going.
//
// No mutex and no atomics: every read and every write happens during layout,
// on the single goroutine Gio runs a frame on.
type trailState struct {
	// segs is the identity → state map, allocated on first use.
	segs map[string]*trailSegment
	// order is the identities the last laid-out frame drew, in trail order,
	// so clicks are drained left to right rather than in map order.
	order []string
	// frame counts layout passes, marking which identities the current pass
	// has drawn so the rest can be retired without a second map.
	frame uint64
}

// trailSegment is one identity's state.
type trailSegment struct {
	click widget.Clickable
	// onClick is the callback the identity was last drawn with. It is the
	// callback that fires, because it is the one the user clicked: the frame
	// that drew the affordance is the frame whose purpose the click carries.
	onClick func(gtx layout.Context)
	// frame is the value of trailState.frame when this identity was last
	// drawn.
	frame uint64
}

// layout drains the clicks the previous frame's trail queued, then draws this
// frame's. The drain runs first on purpose: until it has, the state still
// describes the trail the user was looking at when they pressed.
func (s *trailState) layout(
	gtx layout.Context,
	shaper *text.Shaper,
	segments []Segment,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	style tokens.TextStyle,
	chevron unit.Dp,
) layout.Dimensions {
	s.fire(gtx)
	items, clicks := s.adopt(segments)
	return drawBreadcrumb(gtx, shaper, items, clicks, colors, sp, style, chevron)
}

// fire reports the clicks queued against the identities the previous frame
// drew. An identity that has since left the trail still reports: it was on
// screen when it was clicked, and dropping the click would lose an input the
// user actually made.
func (s *trailState) fire(gtx layout.Context) {
	for _, key := range s.order {
		seg := s.segs[key]
		if seg == nil || seg.onClick == nil {
			continue
		}
		for seg.click.Clicked(gtx) {
			seg.onClick(gtx)
		}
	}
}

// adopt takes this frame's segments as the trail: it mints state for
// identities not seen before, keeps the state of identities that are back,
// retires the rest, and returns the items and per-item clickables the drawing
// code takes. A retired identity registered no input area this frame, so
// nothing can queue against it after fire has drained it.
func (s *trailState) adopt(segments []Segment) ([]Item, []*widget.Clickable) {
	s.frame++
	s.order = s.order[:0]

	items := make([]Item, len(segments))
	clicks := make([]*widget.Clickable, len(segments))
	for i, seg := range segments {
		items[i] = Item{Label: seg.Label, OnClick: seg.OnClick}

		key := seg.identity()
		st := s.segs[key]
		if st == nil {
			if s.segs == nil {
				s.segs = make(map[string]*trailSegment, len(segments))
			}
			st = new(trailSegment)
			s.segs[key] = st
		}
		if st.frame != s.frame {
			// First appearance of this identity in this frame; a later
			// duplicate joins the affordance rather than replacing it.
			st.frame = s.frame
			st.onClick = seg.OnClick
			s.order = append(s.order, key)
		}
		if st.onClick != nil {
			clicks[i] = &st.click
		}
	}

	for key, st := range s.segs {
		if st.frame != s.frame {
			delete(s.segs, key)
		}
	}
	return items, clicks
}
