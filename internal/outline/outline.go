// Package outline derives the ink of a surface's own edge — the one line a
// pattern draws around itself to say where it ends.
//
// Every outline in this repository used to name neutral step 500, and a named
// rung is a pairing rather than a colour. The neutral ramps are paired: light
// and dark are realized at the same perceptual depths from opposite ends, so
// step 500 is the one rung that barely moves between schemes while the ground
// under it moves the whole way. An outlined card measured 2.35:1 in the light
// scheme and 5.94:1 in the dark; a dialog's edge 1.95:1 light; a popover's
// 1.42:1 — under WCAG 1.4.11's 3:1 floor in the scheme most people read in,
// from six lines of code that all look scheme-neutral.
//
// Asking the ramp instead needs to know nothing about either scheme. It needs
// to know one thing only, which is the ground the line is drawn against.
package outline

import (
	"image/color"

	"github.com/vibrantgio/theme/tokens"
)

// Floor is WCAG 1.4.11's contrast floor for a graphic that carries meaning
// without being text — 3:1. An outline is exactly such a graphic: it is the
// whole of what says a card is an object rather than a patch of page, so it
// is not decoration and owes its ground this much.
const Floor = 3.0

// Ink is the neutral rung nearest the ramp's mid-value step that reaches
// Floor against the fill at the given storey.
//
// ground is the storey of the fill the line is drawn against, named in the
// same vocabulary the pattern uses to paint that fill (tokens.SurfaceAt). A
// pattern's outline circles a surface the pattern paints itself, so unlike a
// component — which is put somewhere by someone else and has to be handed its
// ground — a pattern always knows this: the outlined card's own level-1 fill,
// the dialog's level 2, the popover's level 3.
//
// The fill inside is also the harder of the line's two sides, which is why
// naming it is enough. A pattern stands on a plane no deeper than its own
// fill — that is what raising it means — and in the light scheme a deeper
// ground draws a deeper rung, so the ink chosen for the fill clears the
// shallower plane outside by more: the level-1 rung measures 3.55:1 on the
// card's own fill and 4.03:1 on the window ground beneath it, and the level-2
// rung 4.51:1 inside a dialog against 6.19:1 on the window. In the dark
// scheme the walk answers one rung throughout and every pairing clears.
func Ink(c tokens.ColorTokens, ground tokens.ElevationLevel) color.NRGBA {
	return c.MarkOn(tokens.RoleNeutral, c.SurfaceAt(ground), Floor)
}
