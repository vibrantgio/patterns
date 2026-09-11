// Package outline derives the colour of a surface's own edge — the one line a
// pattern draws around itself to say where it ends.
//
// An outline must be derived from the fill it is drawn against, never named
// as a fixed neutral step: a named step is a pairing rather than a colour, and
// holds its perceptual depth across schemes while the surface under it moves,
// so it fails the contrast floor in one scheme while looking scheme-neutral in
// the source.
//
// It takes that fill as a colour and not as a level, because a raised
// surface is walked from whatever it stands on and has no level to name
// ([tokens.ColorTokens.RaisedOn]).
package outline

import (
	"image/color"

	"github.com/vibrantgio/theme/tokens"
)

// Floor is the contrast floor for a non-text graphic that carries meaning,
// the theme's own. An outline is such a graphic: it is the whole of what says
// a surface is an object rather than a patch of page.
const Floor = tokens.GraphicFloor

// `Color` is the neutral step nearest the ramp's mid-value step that reaches
// Floor against surface, the fill the line is drawn around.
//
// Naming only the inner fill is sufficient because it is the harder of the
// line's two sides: a pattern stands on a surface no lighter than its own
// fill, so a line that clears Floor against the fill clears it against the
// surface outside by more.
func Color(c tokens.ColorTokens, surface color.NRGBA) color.NRGBA {
	return c.MarkOn(tokens.RoleNeutral, surface, Floor)
}
