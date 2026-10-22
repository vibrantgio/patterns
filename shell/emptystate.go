package shell

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"

	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// EmptyStateSizeSp is the size one line of a column's empty state is set at.
// MEASURED off reference/macos/mail-empty-column-light.png and
// mail-empty-column-dark.png at 1x: the glyphs carry a cap band 20 px tall,
// which at a cap height of 0.709 em is 28 sp. No role in the type set states
// it — it stands above every one of them — so the number is carried here.
const EmptyStateSizeSp = 28

// EmptyState paints the one line a column with nothing to show stands in
// place of its content, and the reason there is none: nothing selected,
// nothing in the collection, content still arriving, a read that failed. It
// is read and never operated, and it is not content: it does not stand on
// the column's measure.
//
// THE CALLER HANDS IT THE COLUMN'S FULL FRAME, the toolbar's share
// included, because that is the frame the line centres in. MEASURED off the
// pair above: the line's own frame is 35 px tall at window y 450 and the
// column's spans y 30–905, so both centres stand at 467.5, where a line
// centred on the row below the toolbar would stand 26 px lower.
//
// The colour is TertiaryLabel over fill, the fill the column paints.
// MEASURED off the same pair: the glyphs plateau at #bdbdbd light on
// #ffffff and #595f62 dark on #232a2e, which is TertiaryLabel flattened
// over the column and no coverage of SecondaryLabel.
//
// The line is set in the typeface and the weight the body role carries, at
// the measured size, and it is one line: a frame too narrow for it
// truncates it rather than wrapping it.
func EmptyState(gtx layout.Context, sh *text.Shaper, c tokens.PlatformColors, typ tokens.Typography,
	fill color.NRGBA, msg string) layout.Dimensions {
	frame := gtx.Constraints.Max
	if msg == "" || frame.X <= 0 || frame.Y <= 0 {
		return layout.Dimensions{Size: frame}
	}
	style := typ.BodyLarge
	style.Size = EmptyStateSizeSp
	// The line box is the text's own, so the frame's floor is what centres
	// it: typeset splits the surplus above and below the box it laid out.
	style.LineHeight = 0
	tgtx := gtx
	tgtx.Constraints = layout.Exact(frame)
	typeset.TextAligned(tgtx, sh, msg, style, vgcolor.Flatten(c.TertiaryLabel, fill), 1,
		font.Normal, text.Middle)
	return layout.Dimensions{Size: frame}
}
