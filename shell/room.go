package shell

import (
	"image"
	"maps"

	"gioui.org/layout"
)

// availableRoomKey is the key the available room is set under in
// layout.Context.Values.
const availableRoomKey = "github.com/vibrantgio/patterns/shell.AvailableRoom"

// unboundedRoom is the extent [AvailableRoom] reports where no root set an
// area: wide and tall enough that a one-line annotation is never shortened
// by it and never kept off a side, so a tooltip given it is placed as one
// given no available room is.
const unboundedRoom = 1 << 20

// WithAvailableRoom returns gtx with r set as the available room, in gtx's
// own coordinates: the rectangle a floating surface laid out under gtx may
// be placed in. A window's root sets it to the window's content area before
// it lays out its arrangement.
//
// The area is passed on the context because Gio offers no readback of the
// transform an ancestor pushed: the root is the one place where the
// window's edges are known, and every arrangement between the root and a
// tooltip moves the area by the offset it places its child at (see
// [AvailableRoomAt]). gtx.Values is cloned rather than written, so the
// caller's context keeps the area it had.
func WithAvailableRoom(gtx layout.Context, r image.Rectangle) layout.Context {
	values := maps.Clone(gtx.Values)
	if values == nil {
		values = make(map[string]any, 1)
	}
	values[availableRoomKey] = r
	gtx.Values = values
	return gtx
}

// AvailableRoomAt returns gtx with the available room moved into the
// coordinates of a child placed at off: the area less off. An arrangement
// calls it with the offset it pushes before it lays the child out. Where no
// area is set, gtx is returned as it is.
func AvailableRoomAt(gtx layout.Context, off image.Point) layout.Context {
	r, ok := gtx.Values[availableRoomKey].(image.Rectangle)
	if !ok || off == (image.Point{}) {
		return gtx
	}
	return WithAvailableRoom(gtx, r.Sub(off))
}

// AvailableRoom reads the available room in gtx's own coordinates. Its
// signature is the one a tooltip's AvailableRoom takes, so a caller passes
// the function itself.
//
// Where no root set an area it reports an unbounded rectangle, which places
// a tooltip as no available room at all does.
func AvailableRoom(gtx layout.Context) image.Rectangle {
	if r, ok := gtx.Values[availableRoomKey].(image.Rectangle); ok {
		return r
	}
	return image.Rect(-unboundedRoom, -unboundedRoom, unboundedRoom, unboundedRoom)
}

// withoutAvailableRoom returns gtx with the available room removed, for a
// child whose offset is decided only after it is laid out: a row that
// centres what it holds, or a group that is measured before it is placed.
// An area that is not moved by that offset would place a tooltip against
// edges that are not the window's, so the child is given none, and a
// tooltip in it is placed as one given no available room.
func withoutAvailableRoom(gtx layout.Context) layout.Context {
	if _, ok := gtx.Values[availableRoomKey]; !ok {
		return gtx
	}
	values := maps.Clone(gtx.Values)
	delete(values, availableRoomKey)
	gtx.Values = values
	return gtx
}
