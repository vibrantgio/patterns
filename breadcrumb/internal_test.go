package breadcrumb

import (
	"testing"

	"gioui.org/layout"

	"github.com/vibrantgio/theme/tokens"
)

// TestLabelColorRule asserts the Specific contract: in a breadcrumb of n
// items, the last segment renders in Text (current location) and the
// preceding segments render in neutral 700. The goldens carry real labels
// and so do show the two foregrounds apart, but only as pixels; this
// pure-Go test guards the rule itself, on every step of both ramps.
func TestLabelColorRule(t *testing.T) {
	for _, c := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		const n = 3
		for i := 0; i < n; i++ {
			got := labelColor(i, n, c)
			want := c.Ramps.Neutral.Step(700)
			if i == n-1 {
				want = c.Text
			}
			if got != want {
				t.Errorf("idx %d of %d (Surface=%v): got %v, want %v", i, n, c.Surface, got, want)
			}
		}
	}
}

// TestLabelColorSingleSegment confirms that with one item the lone
// segment is treated as the current location (Text), matching the
// "last item" rule degenerate case.
func TestLabelColorSingleSegment(t *testing.T) {
	c := tokens.DefaultLight
	if got := labelColor(0, 1, c); got != c.Text {
		t.Errorf("single segment: got %v, want Text %v", got, c.Text)
	}
}

// TestSegmentIdentityFallsBackToLabel pins the rule a caller can trip over
// silently: a Segment with no Key is addressed by its Label, which is stable
// enough while labels are the path and not stable at all once two places
// share one.
func TestSegmentIdentityFallsBackToLabel(t *testing.T) {
	for _, tc := range []struct {
		seg  Segment
		want string
	}{
		{Segment{Key: "/design", Label: "Design"}, "/design"},
		{Segment{Label: "Design"}, "Design"},
		{Segment{}, ""},
	} {
		if got := tc.seg.identity(); got != tc.want {
			t.Errorf("Segment{Key:%q, Label:%q}.identity() = %q, want %q",
				tc.seg.Key, tc.seg.Label, got, tc.want)
		}
	}
}

// TestTrailStateKeepsAndRetiresIdentities is the bookkeeping behind a trail
// that changes shape: an identity that is still in the trail keeps the
// clickable it had, one that has left is dropped rather than accumulated, and
// one that comes back starts over. A retired identity drew no input area on
// the frame that retired it, so there is nothing left to address it with.
func TestTrailStateKeepsAndRetiresIdentities(t *testing.T) {
	noop := func(layout.Context) {}
	deep := []Segment{
		{Key: "/", Label: "Home", OnClick: noop},
		{Key: "/design", Label: "Design", OnClick: noop},
		{Key: "/design/tokens", Label: "Tokens"},
	}
	shallow := []Segment{{Key: "/", Label: "Home", OnClick: noop}}

	var s trailState
	if _, clicks := s.adopt(deep); len(s.segs) != 3 || clicks[2] != nil {
		t.Fatalf("after a three-segment trail: %d identities held and clicks[2]=%v; want 3 and nil for the inert segment",
			len(s.segs), clicks[2])
	}
	home := s.segs["/"]

	if _, _ = s.adopt(deep); s.segs["/"] != home {
		t.Error("an identity still in the trail was given a new clickable")
	}

	s.adopt(shallow)
	if len(s.segs) != 1 {
		t.Errorf("after the trail shortened, %d identities held: %v; want only the one drawn", len(s.segs), s.segs)
	}
	if s.segs["/"] != home {
		t.Error("the surviving identity was given a new clickable")
	}

	s.adopt(deep)
	if s.segs["/design"] == nil {
		t.Fatal("an identity back in the trail was not minted again")
	}
	if len(s.order) != 3 {
		t.Errorf("order = %v; want the three identities this frame drew, in trail order", s.order)
	}
}

// TestTrailStateSharesOneAffordancePerIdentity holds the degenerate trail —
// the same place twice — to its documented rule: one identity is one
// affordance, the first of them owns the callback, and a click there fires
// once rather than twice.
func TestTrailStateSharesOneAffordancePerIdentity(t *testing.T) {
	var first, second int
	segs := []Segment{
		{Key: "/", Label: "Home", OnClick: func(layout.Context) { first++ }},
		{Key: "/", Label: "Home again", OnClick: func(layout.Context) { second++ }},
		{Key: "/design", Label: "Design"},
	}

	var s trailState
	_, clicks := s.adopt(segs)
	if len(s.segs) != 2 {
		t.Fatalf("%d identities held for a trail naming two places; want 2", len(s.segs))
	}
	if clicks[0] == nil || clicks[0] != clicks[1] {
		t.Errorf("clicks[0]=%p clicks[1]=%p; the two positions of one place share one affordance", clicks[0], clicks[1])
	}

	s.segs["/"].onClick(layout.Context{})
	if first != 1 || second != 0 {
		t.Errorf("first=%d second=%d; the first segment with the identity owns the callback", first, second)
	}
}
