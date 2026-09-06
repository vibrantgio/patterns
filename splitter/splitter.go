// Package splitter provides the splitter: the control that resizes two
// regions against each other. It is the seam between them made operable —
// it draws as the seam draws, at the seam's width and in the seam's
// colour, and thickens and firms while a hand is on it. Its hit area is
// wider than the line it draws, the resize pointer shows over it, and
// dragging it moves the boundary within the bounds the caller states,
// never past them.
//
// The package owns the line and the hand on it, and nothing else. It
// lays out no regions and reserves no space: the caller arranges the two
// regions, states where their boundary sits, and hears the new position
// back through a callback. That is what lets one splitter serve a
// boundary held as a fraction of the available length and one held as an
// absolute trailing width — both are pixels along an axis by the time
// they reach here, and the caller converts back into whatever it keeps.
//
// Everything the package takes and reports along the main axis is in
// pixels, measured from the origin of the layout.Context it is given. A
// caller holding dp converts with gtx.Dp on the way in and divides by
// gtx.Metric.PxPerDp on the way out. The cross axis is not stated at all:
// the seam runs the whole cross extent of the constraints it is handed,
// so a caller that wants less constrains a copy of the context.
package splitter

import (
	"image"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/theme/tokens"
)

const (
	// SeamDp is what the splitter paints at rest: the seam's own width,
	// the pane pattern's hairline, which is the width the platform's own
	// splitters take. A seam runs a whole edge, so its width is the width
	// of the scar it leaves across everything it crosses: at a hairline it
	// crosses a band the way a platform splitter does, and anything wider
	// severs the band into two pieces.
	SeamDp = pane.SeamDp

	// GrabbedDp is what it paints while a hand is on it: the same line,
	// thick enough to read as a change of state rather than as a second
	// edge beside the first. The hit area does not move with it — a target
	// that grew under the hand holding it would be a target that jumped.
	GrabbedDp = 2

	// HitDp is the band the line is taken by. Paint and hit are
	// deliberately different sizes: a hairline is what the eye wants and
	// an impossible pointer target, so the band reaches into both regions
	// rather than reserving a gutter of its own. It is added to the op
	// stream after the regions, so the topmost area takes the hit — which
	// is what should happen to a press this close to the boundary.
	HitDp = 6

	// firmStep is the neutral ramp step the line takes while grabbed: the
	// strong border step, one the resting seam never reaches in either
	// scheme, so the change is legible without the line changing hue.
	firmStep = 500
)

// SeamWidth is what the line paints at rest, in pixels — never less than
// one, because a hairline that rounds to nothing is no line at all.
func SeamWidth(gtx layout.Context) int {
	return max(gtx.Dp(unit.Dp(SeamDp)), 1)
}

// HitWidth is the width of the band the line is taken by, in pixels. It
// is never narrower than the line: the target is wider than what it
// drags, or it is not a target.
func HitWidth(gtx layout.Context) int {
	return max(gtx.Dp(unit.Dp(HitDp)), SeamWidth(gtx))
}

// SeamColor is the colour of the resting line: the Seam token, whose job
// is the line between two regions.
func SeamColor(c tokens.ColorTokens) color.NRGBA { return c.Seam }

// GrabbedColor is the colour of the line while a hand is on it: the same
// neutral ramp, firmer.
func GrabbedColor(c tokens.ColorTokens) color.NRGBA {
	return c.Ramps.Neutral.Step(firmStep)
}

// Props states one splitter for one frame.
type Props struct {
	// Axis is the axis the boundary moves along: layout.Horizontal for a
	// vertical line parting two side-by-side regions, layout.Vertical for
	// a horizontal one parting two stacked regions.
	Axis layout.Axis

	// Boundary is where the two regions meet, in pixels along Axis from
	// the origin of the context the splitter is laid out in. The line is
	// drawn there and a drag is measured from there.
	Boundary float32

	// Min and Max bound Boundary: the room the two regions allow between
	// them, in the same pixels. A drag is reported clamped to them and
	// never past them, so a caller that states neither pins the boundary
	// at zero.
	Min, Max float32

	// Colors resolves the line's two colours.
	Colors tokens.ColorTokens

	// OnChange is invoked with the new boundary, already clamped, for
	// every pointer event that moves it. It is called during Update, on
	// the frame goroutine.
	OnChange func(boundary float32)
}

// State is one splitter's hand across frames: whether it is being
// dragged, whether a pointer rests on it, and where a drag began. The
// caller holds one per splitter for the lifetime of the composition it
// belongs to and touches it only on the frame goroutine.
//
// A nil *State is a splitter nothing can take hold of: it draws the
// resting line and adds no hit area and no pointer. That is the static
// render path — a stored image has no hand in it.
type State struct {
	// tag is non-zero-size so its address is a unique event tag.
	tag      struct{ _ byte }
	press    float32 // main-axis pointer position at press
	start    float32 // boundary at press
	dragging bool
	hovering bool
}

// Dragging reports whether a drag is in progress.
func (s *State) Dragging() bool { return s != nil && s.dragging }

// Grabbed reports whether a hand is on the splitter — dragging it, or
// resting on it about to. It is what the thickened line says.
func (s *State) Grabbed() bool { return s != nil && (s.dragging || s.hovering) }

// Update takes the splitter's pointer events and reports the boundary
// they move it to. Layout calls it, so a caller that arranges its regions
// after the splitter has nothing else to do; a caller whose regions are
// arranged from the boundary calls Update first, so that the whole frame
// is drawn at the position this frame's drag reached rather than the
// previous one's. Calling it twice in a frame is harmless: the second
// call finds the queue drained.
func (s *State) Update(gtx layout.Context, p Props) {
	if s == nil {
		return
	}
	for {
		e, ok := gtx.Event(pointer.Filter{
			Target: &s.tag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Enter | pointer.Leave,
		})
		if !ok {
			break
		}
		pe, ok := e.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Press:
			s.press = p.Axis.FConvert(pe.Position).X
			s.start = p.Boundary
			s.dragging = true
		case pointer.Drag:
			if !s.dragging {
				continue
			}
			at := clamp(s.start+p.Axis.FConvert(pe.Position).X-s.press, p.Min, p.Max)
			if p.OnChange != nil {
				p.OnChange(at)
			}
		case pointer.Release, pointer.Cancel:
			s.dragging = false
		case pointer.Enter:
			s.hovering = true
		case pointer.Leave:
			s.hovering = false
		}
	}
}

// Layout draws the line at Boundary and puts the hit area and the resize
// pointer over it, both running the full cross extent of the context's
// constraints. It returns the rectangle the line itself paints, which is
// the only space the splitter takes; the hit area overlaps the regions on
// either side and takes none.
//
// The regions are drawn first and the line after them, so a region that
// overruns its constraints cannot erase it; the hit area comes last,
// because it is the only one of the three whose rectangle overlaps its
// neighbours.
func (s *State) Layout(gtx layout.Context, p Props) layout.Dimensions {
	s.Update(gtx, p)

	axis := p.Axis
	total := axis.Convert(gtx.Constraints.Max).X
	cross := axis.Convert(gtx.Constraints.Max).Y
	seamPx := SeamWidth(gtx)

	at := int(p.Boundary + 0.5)
	if at < 0 {
		at = 0
	}
	if at > total {
		at = total
	}

	// The line at rest and the line under a hand are the same line: the
	// thickening grows from the resting pixel rather than replacing it.
	width, fill := seamPx, SeamColor(p.Colors)
	if s.Grabbed() {
		width, fill = max(gtx.Dp(unit.Dp(GrabbedDp)), seamPx), GrabbedColor(p.Colors)
	}
	lineMin := at - (width-seamPx)/2
	line := image.Rectangle{
		Min: axis.Convert(image.Pt(lineMin, 0)),
		Max: axis.Convert(image.Pt(lineMin+width, cross)),
	}
	paint.FillShape(gtx.Ops, fill, clip.Rect(line).Op())

	if s == nil {
		return layout.Dimensions{Size: line.Size()}
	}

	// The hit area is centred on the resting line, not on the drawn one,
	// so that taking hold of the splitter does not move what is held.
	hitPx := HitWidth(gtx)
	hitMin := max(at-(hitPx-seamPx)/2, 0)
	hitMax := min(hitMin+hitPx, total)
	hit := image.Rectangle{
		Min: axis.Convert(image.Pt(hitMin, 0)),
		Max: axis.Convert(image.Pt(hitMax, cross)),
	}
	area := clip.Rect(hit).Push(gtx.Ops)
	event.Op(gtx.Ops, &s.tag)
	cursor := pointer.CursorColResize
	if axis == layout.Vertical {
		cursor = pointer.CursorRowResize
	}
	cursor.Add(gtx.Ops)
	area.Pop()

	return layout.Dimensions{Size: line.Size()}
}

func clamp(v, lo, hi float32) float32 {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
