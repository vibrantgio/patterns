package shell

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/vibrantgio/components/tooltip"
)

// placedChild is one child of a row laid out by [placingFlex]: its
// layout.Widget and, for a child that takes a share of the space left over,
// its weight.
type placedChild struct {
	widget layout.Widget
	flexed bool
	weight float32
}

// placedRigid is a child laid out at its own size, as layout.Rigid is.
func placedRigid(w layout.Widget) placedChild { return placedChild{widget: w} }

// placedFlexed is a child that takes weight's share of the space left over,
// as layout.Flexed is.
func placedFlexed(weight float32, w layout.Widget) placedChild {
	return placedChild{widget: w, flexed: true, weight: weight}
}

// placingFlex lays children out with f, as f.Layout does, and then places
// every tooltip surface laid out in them inside the window's available room.
//
// A flex decides a child's offset only after it has measured every child, so
// no child is given the available room: each is laid out with the area
// removed and with a [tooltip.Surfaces] set. Once the flex has drawn the row,
// the offset of each child is worked out from the sizes it reported, by the
// rules f.Layout placed it by, and the child's surfaces are placed with
// [tooltip.PlaceSurfaces] inside a transform pushed at that offset, in the
// area moved by it. The surfaces are drawn after the whole row, and each
// through op.Defer, so they paint above it.
func placingFlex(gtx layout.Context, f layout.Flex, children ...placedChild) layout.Dimensions {
	dims := make([]layout.Dimensions, len(children))
	surfaces := make([]*tooltip.Surfaces, len(children))
	flex := make([]layout.FlexChild, len(children))
	for i, c := range children {
		w := func(gtx layout.Context) layout.Dimensions {
			gtx, surfaces[i] = tooltip.WithSurfaces(gtx)
			dims[i] = c.widget(gtx)
			return dims[i]
		}
		if c.flexed {
			flex[i] = layout.Flexed(c.weight, w)
		} else {
			flex[i] = layout.Rigid(w)
		}
	}
	d := f.Layout(withoutAvailableRoom(gtx), flex...)
	for i, off := range flexOffsets(f, gtx.Constraints, dims) {
		st := op.Offset(off).Push(gtx.Ops)
		placeSurfaces(gtx, surfaces[i], off)
		st.Pop()
	}
	return d
}

// placeSurfaces places the surfaces s recorded in a child placed at off, in
// the available room gtx has set moved by off. The caller has pushed the
// transform the child was placed at, or calls it from inside the child.
//
// Where gtx has a [tooltip.Surfaces] set, the layout is itself laid out in
// a child that is placed only after it is measured: the callbacks are then
// added to that Surfaces, moved by off, and called once the outer layout
// places its child.
func placeSurfaces(gtx layout.Context, s *tooltip.Surfaces, off image.Point) {
	cgtx := AvailableRoomAt(gtx, off)
	tooltip.PlaceSurfaces(cgtx, s, AvailableRoom(cgtx))
}

// flexOffsets returns the offset f.Layout places each child at, given the
// constraints the flex was laid out in and the size each child reported. It
// follows layout.Flex's own placement: the children one after the other along
// the main axis, with the gap and the spacing f states, each aligned across
// it by f.Alignment.
func flexOffsets(f layout.Flex, cs layout.Constraints, dims []layout.Dimensions) []image.Point {
	n := len(dims)
	mainMin := f.Axis.Convert(cs.Min).X
	crossMin := f.Axis.Convert(cs.Min).Y
	size, maxCross, maxBaseline := 0, crossMin, 0
	for _, d := range dims {
		s := f.Axis.Convert(d.Size)
		size += s.X
		maxCross = max(maxCross, s.Y)
		maxBaseline = max(maxBaseline, d.Size.Y-d.Baseline)
	}
	if n > 1 && f.Gap > 0 {
		size += f.Gap * (n - 1)
	}
	space := max(mainMin-size, 0)
	main := 0
	switch f.Spacing {
	case layout.SpaceSides:
		main += space / 2
	case layout.SpaceStart:
		main += space
	case layout.SpaceEvenly:
		main += space / (1 + n)
	case layout.SpaceAround:
		if n > 0 {
			main += space / (n * 2)
		}
	}
	offs := make([]image.Point, n)
	for i, d := range dims {
		s := f.Axis.Convert(d.Size)
		cross := 0
		switch f.Alignment {
		case layout.End:
			cross = maxCross - s.Y
		case layout.Middle:
			cross = (maxCross - s.Y) / 2
		case layout.Baseline:
			if f.Axis == layout.Horizontal {
				cross = maxBaseline - (d.Size.Y - d.Baseline)
			}
		}
		offs[i] = f.Axis.Convert(image.Pt(main, cross))
		main += s.X
		if i < n-1 {
			main += f.Gap
			switch f.Spacing {
			case layout.SpaceEvenly:
				main += space / (1 + n)
			case layout.SpaceAround:
				main += space / n
			case layout.SpaceBetween:
				main += space / (n - 1)
			}
		}
	}
	return offs
}
