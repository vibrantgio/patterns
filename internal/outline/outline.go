// Package outline derives the ink of a surface's own edge — the one line a
// pattern draws around itself to say where it ends.
//
// An outline must be derived from the ground it is drawn against, never named
// as a fixed neutral rung: a named rung is a pairing rather than a colour, and
// holds its perceptual depth across schemes while the ground under it moves,
// so it fails the contrast floor in one scheme while looking scheme-neutral in
// the source.
package outline

import (
	"image/color"

	"github.com/vibrantgio/theme/tokens"
)

// Floor is WCAG 1.4.11's contrast floor for a non-text graphic that carries
// meaning — 3:1. An outline is such a graphic: it is the whole of what says a
// surface is an object rather than a patch of page.
const Floor = 3.0

// Ink is the neutral rung nearest the ramp's mid-value step that reaches
// Floor against the fill at the given storey.
//
// ground is the storey of the fill the line is drawn against, named in the
// same vocabulary the pattern uses to paint that fill (tokens.SurfaceAt).
//
// Naming only the inner fill is sufficient because it is the harder of the
// line's two sides: a pattern stands on a plane no deeper than its own fill,
// so ink that clears Floor against the fill clears it against the shallower
// plane outside by more.
func Ink(c tokens.ColorTokens, ground tokens.ElevationLevel) color.NRGBA {
	return c.MarkOn(tokens.RoleNeutral, c.SurfaceAt(ground), Floor)
}
