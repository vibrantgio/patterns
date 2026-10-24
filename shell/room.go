package shell

import (
	"image"

	"gioui.org/layout"

	"github.com/vibrantgio/components/tooltip"
)

// WithAvailableRoom returns gtx with r set as the available room, in gtx's
// own coordinates: the rectangle a floating surface laid out under gtx may be
// placed in. A window's root sets it to the window's content area before it
// lays out its arrangement; every layout below moves it, with the floating
// surfaces, by [tooltip.AvailableRoomAt] and [tooltip.PlacingFlex].
func WithAvailableRoom(gtx layout.Context, r image.Rectangle) layout.Context {
	return tooltip.WithAvailableRoom(gtx, r)
}
