# patterns

The pattern layer of [Vibrant Gio](https://github.com/vibrantgio), a design
system for native desktop applications on macOS, Windows and Linux, written in
pure Go on [Gio](https://gioui.org). Where components gives you a button, patterns
gives you the seventeen composed things an application is actually made of — an
application shell, a pane, a splitter, a navbar, a sidebar, a virtualised data table, a dialog, a
column of notifications, a hero section.

Every one of them is the part you would otherwise write by hand and get subtly
wrong: the dialog that knows a question from a place and so refuses to let a
stray backdrop click answer the question, the popover that dismisses
when you open another one, the column that stacks and times its
notifications, the table that lays out only the rows you can see. Each pattern reads its visual
values from [theme](https://github.com/vibrantgio/theme)'s theme
observable, and the theme carries the whole look: colour, typography, density
and motion. A window follows the OS between light and dark with no
application code; switching an app to Compact density resizes the navbar,
sidebar items, tabs and table rows as a theme change, not a sweep;
every fill, foreground and seam reads the platform's own name for what that
element is on macOS — the chrome regions the sidebar material, the content
`ControlBackground`, a card the platform's grouped box, a floating surface
the window background under the measured floating shadow, every seam
`Separator` flattened over what is beneath it; shadows are reserved for the
surfaces that float and can leave (the dialog, the popover, the toast) and
for the pane with its rim; the card casts none.

Most packages have two entry points:

- **The live form** — `accordion.Accordion`, `table.Table`, `dialog.Dialog`,
  `shell.Shell`, … — takes an `rx.Observable[theme.Theme]` plus a props
  struct and returns an `rx.Observable[layout.Widget]`. Dynamic state arrives
  as observables too (`dialog.Props.Open`, `table.Props.Items`,
  `accordion.Props.Open`), and interaction state — the `widget.Clickable`s,
  the drag position — is allocated inside the pattern's `rx.Defer` scope so
  it survives the view rebuilds an MVU loop drives.
- **The static form** — `Render` (`RenderStackedPage` in `shell`) — takes
  resolved tokens and the state as plain values and draws one frame with no
  event handling. That is what the golden-image tests drive, and what to use
  for static rendering. Each package's row below gives its own signature.
  Drive patterns through their live entry points unless you are rendering a
  static frame.

  The static signatures share their order: the shaper (absent from `card`
  and `popover`, which draw no text of their own), the props (`table` takes
  its columns, items and sort instead), the state as a plain value, the
  platform colours, the spacing scale, the radius scale where the pattern
  rounds a corner, and the type tail. A pattern that draws one type role
  takes that role's whole `tokens.TextStyle` — typeface, weight, size and
  line height, exactly what the live path reads off the theme — while one
  that sets several (`hero`, `pricing`, `feature`, `testimonial`) takes the
  whole `tokens.Typography` and picks its own roles, as it does live. A
  `tokens.Density` comes last only where the pattern sizes a control:
  `navbar`, `sidebar`, `tabs`, `table`, `shell`, `hero`, `pricing` and
  `dialog` take one; the others, and `table.RenderTextCell`, do not.

`pane` and `splitter` are parts an arrangement lays out each frame, with no
live form: they take the frame's `layout.Context` and the platform colours.

Typography is theme-owned: in the live form every pattern that draws text
shapes with the theme's `Typography.Shaper()` and the type roles it carries,
so leaving `Props.Shaper` nil is the normal case.
`Props.Shaper` is an explicit per-instance override for the rare pattern
instance that must shape with a different shaper than the theme provides.

The source of each pattern is short and free of opaque configuration on
purpose. When a pattern is nearly what you want, copying its file into your
application and editing it is a supported outcome, not a defeat — several are a
couple of hundred lines, and the props struct is not trying to anticipate you.

## Where it sits

The tier above effects in the stack. patterns imports `theme`, `tokens`,
`color` and `typeset` from [theme](https://github.com/vibrantgio/theme);
`badge`, `button`, `composite`, `icon`, `icons`, `layout`, `list`,
`pointershape`, `scrollbar`, `toast` and `tooltip` from
[components](https://github.com/vibrantgio/components); `depth` and `tween`
from [effects](https://github.com/vibrantgio/effects); and `mvu`,
`mvu/desktop` and `mvu/stream` from [mvu](https://github.com/vibrantgio/mvu).
The [organization page](https://github.com/vibrantgio) has the full tier
table.

```sh
go get github.com/vibrantgio/patterns
```

## Packages

**Shells and navigation** — the frame an application lives in.

| Package | |
| --- | --- |
| `shell` | The top-level layout. `TwoColumn` is the two-column arrangement the platform's own sidebar windows stand in: the pane down the leading edge, the toolbar across the content column beside it, an optional inspector down the trailing edge, a footer under each column and a splitter on each boundary. `ThreeColumn` is the arrangement Mail, Notes and Voice Memos are built in: the pane, a `ListColumn` of absolute width on a splitter that keeps its width when the window is resized, and the content column taking the rest, with one toolbar lying across the list and the content and one share per column; `PlatformList` carries the list column's measured width and bounds. `PaneFrame` is the part of either a window arranging its own columns composes from; `Toolbar` is the toolbar across the content column, with `PlatformToolbar` its measured metrics. `RememberArrangement` keeps a window's column widths and the pane's shown state in an `ArrangementMemory`; `PaneWindow` is the window treatment a shell stands in; `EmptyState` is what a region with nothing to show draws in place of its content; `WithAvailableRoom` sets a window's content area as the available room its tooltips are placed within. `Shell` is the stacked page: a pinned navbar over a shell-owned scroll of page sections, stated with `Props` — `Navbar`, `Footer`, `Sections` and `ContentMaxWidth` — and drawn statically by `RenderStackedPage(shaper, props, sections, colors, sp, title, d)`. |
| `pane` | The chrome column set in from the window's leading, top and bottom edges: rounded on all four corners, the platform's rim just inside its edge and the shadow it casts around it, the window background showing on three sides. `Bounds` places it in the window, `Layout` draws it around its contents, `Surface`, `RimColor` and `Shadow` are its fill, rim and shadow by the platform's names, and `DragSpacer` is a space in it that moves the window. Hidden, it takes no width. |
| `splitter` | The control that resizes two regions against each other: the seam between them made operable, drawn at the seam's width in `SeamColor` and thickening while the pointer is on it, with a hit area wider than the line and the resize pointer over it. `State.Layout` draws it and reports the dragged boundary through `Props.OnChange`, kept between `Props.Min` and `Max`; `Props.HitSpan`, a `Span`, bounds the stretch of the line a pointer may take hold of. The caller lays out the regions. |
| `navbar` | A horizontal chrome bar with three slots — leading brand, centred links, trailing actions. The active link carries an underline in the platform's selection colour. Static: `Render(shaper, props, colors, sp, title, d)`. |
| `sidebar` | A collapsible vertical chrome column that swaps between an expanded width (icon + title) and a collapsed width (icon only). The active item is drawn as the platform draws a selected row; `PaintSelection` and `PaintSection` draw a selected row's fill and a section heading on their own. Static: `Render(shaper, props, collapsed, colors, sp, title, section, d)`, with one text style for the rows and one for the section headings. |
| `tabs` | A tab strip with an underline in the platform's selection colour on the selection, plus the content pane below it. Click, Arrow-Left/Right (wrapping), Home and End all change the selection. Static: `Render(shaper, props, selected, colors, sp, title, d)`. |

**Data and content** — the things that hold a screenful of stuff.

| Package | |
| --- | --- |
| `table` | The sortable, virtualised data table, built on `components/list`: only the visible rows lay out, whatever the row count. Sort and filter are external — the `Items` observable emits already-sorted, already-filtered slices and the header reports a sort click through `OnSort`. Row heights follow the theme's density. Static: `Render(shaper, columns, items, sk, colors, sp, header, d)`; `RenderTextCell` draws one text cell. |
| `card` | The one thing that must stand apart: the platform's grouped box, a rounded surface whose fill is a small step from the surface it is in, with optional Header / Body / Footer slots. The box is the whole of how it singles something out — no hairline of its own, never outlined, never drawn in a role, and what the developer wants to say about it is a badge in its header. A card is raised, not floating, so it casts no shadow: a shadow marks what floats and can leave. Static: `Render(props, colors, sp, rad)`. |
| `group` | The page dividing itself: a hairline around related components, drawn on the surface the group is in and taking that surface's own fill, with an optional title top-leading inside it. It paints nothing inside and raises nothing, so what it holds stands on the surface the group is in. The hairline is the seam of two regions sharing one fill — the platform's `Separator` flattened over that fill, the understated line the platform draws. A group may hold a card; it never holds another group, and it is drawn in no role. Static: `Render(shaper, props, colors, sp, rad, title)`. |
| `accordion` | A vertical stack of collapsible sections with a rotating chevron. `SingleOpen` makes activating a closed section first toggle every open peer, so a parent's flip-the-bool handler converges on single-open with no extra bookkeeping. Static: `Render(shaper, props, open, colors, sp, title)`, `open` the open sections by index. |

**Overlays and feedback** — the things that draw over everything else.

| Package | |
| --- | --- |
| `dialog` | A centred dialog over a full-window scrim, its surface the platform's window background under the measured floating shadow and no hairline: header, padded body, footer actions. It comes in the desktop field's two archetypes, and `Props.Decision` is the whole of the choice: a **dismissible dialog** carries a ghost close ×, and Escape and a backdrop click both close it; a **decision dialog** carries no ×, its backdrop is inert, Escape invokes Cancel, and Return invokes the default action — never a destructive one, which is why the default is derived rather than nominated. Tab and Shift+Tab cycle inside either and cannot escape to the background, and only the dialog at the front of the stack receives input — the ones it covers stay painted and go inert. That stack is frame state rather than a bus: `Props.Arbiter` names the set a dialog stacks within — one per window — and unlike popover's single arbiter it is ordered, because a dialog opened over another one covers it and closing the inner one hands the front back. A nil `Arbiter` gets the dialog a stack of its own, so sharing one is the explicit act. Footer actions own their own focus tags, so a focused action shows exactly one ring, and the footer owns their width — the platform's measured dialog button width, which a title too wide for it widens by its own measure. A dialog opens with the keyboard on its body's first focusable, as the platform's sheet shows. Static: `Render(shaper, props, open, colors, sp, rad, title, d)`. |
| `popover` | An anchored floating surface with a triangular tail pointing at a caller-supplied anchor. Outside-click dismissal and popover-vs-popover arbitration are frame state, not a bus: `Props.Arbiter` names the set a popover arbitrates within — one per window — and opening a second popover in that set dismisses the first, in the same frame, from inside the claimant's own layout pass. A nil `Arbiter` gets the popover one of its own, so sharing one is the explicit act. `Props.Open` carries open-ness on a stream; `Props.OpenNow` reads it during layout, for a caller that owns it as frame state. Inside the available room `Props.AvailableRoom` reports, the surface is placed by `components/attachment`: it takes the opposite side when the preferred side has no room and the other holds it whole, takes the side with more room when neither does, is shifted inward when it would cross the room's edge, and scrolls inside itself when taller than the side it wins, the tail pointing at the anchor through all of it. With no room reported it keeps the side asked for and is shifted back inside the frame it is given. Static: `Render(props, open, colors, sp, rad)`. |
| `notifications` | The position-anchored column that receives the application's notifications, places them, stacks them against each other, times them, and presents each one as a toast from [`components/toast`](https://github.com/vibrantgio/components). The pattern owns the queue, the placement and the timing, not the presentation: the toast's fill and its status-coloured leading edge are the component's, and what the column adds under each is the platform's floating shadow through `effects/depth`, because only the placement knows where the surface landed — and a shadow is what says it floats and can leave, which is the whole of what a shadow is for. The queue is the application's, not the package's: `Notify(gtx, …)` lands a `Requested` message, the reducer adds it to a `notifications.Queue` in the model, `Props.Notifications` carries that queue back to the `Column`, and `Expire` brings the removal back as `Expired` at the end of the notification's `Lifetime` (`DefaultLifetime`, 4 s). Only the fade is the frame's: it tweens through `effects/tween` across the theme's `DurSlow` stop. Static: `Render(shaper, props, queued, colors, sp, rad, style)`. |

**Marketing** — the landing-page sections, for the app's own front door.

| Package | |
| --- | --- |
| `hero` | The landing block: optional eyebrow kicker (pure typography — a small understated line, not a component), display title, subtitle, optional visual slot, and a primary/secondary CTA pair. With no visual it is one centred column; with one it splits into two equal columns. Static: `Render(shaper, props, colors, sp, rad, typo, d)`. |
| `feature` | An icon–title–body grid laid out `Columns × N`. The icon slot is opaque — any `layout.Widget`. Static: `Render(shaper, props, colors, sp, typo)`. |
| `pricing` | A row of tier groups — name, price and billing period, a checkmarked feature list, a CTA — with one tier optionally `Recommended`, which draws that tier as a card raised on the content and puts a "Popular" badge on its name row. A row of tiers divides the page; the tier that must stand apart from it is the one card. Static: `Render(shaper, props, colors, sp, rad, typo, d)`. |
| `testimonial` | Quote cards with an author block and an avatar (or an initial in a circular placeholder), as a single centred card or a row of them. Static: `Render(shaper, props, colors, sp, rad, typo)`. |

`dialog/gallery` is a `main` inside this module, not a pattern: it
demonstrates a decision dialog — its Tab cycle, its focus-ring ownership, its
Return-bound default and its inert backdrop. Run it with `go run
./dialog/gallery`.

## Usage

Patterns compose by handing one pattern's stream to another's slot. Here a
landing page mounts the Marketing patterns as the scrolling sections of the
stacked page, which pins the navbar, owns the scroll region and re-emits
whenever any section emits. Note that nothing passes a shaper — the theme
carries the typography:

```go
gap := rx.Of[layout.Widget](pllayout.VSpacer(sectionGapDp))
return shell.Shell(th, shell.Props{
	ContentMaxWidth: contentMaxWidthDp, // centred reading column; navbar stays full-bleed
	Navbar:          navbarProps(mirrorTokens(th), pageHome),
	Sections: []rx.Observable[layout.Widget]{
		hero.Hero(th, heroContent(gotoDocs, gotoAbout)),
		gap,
		feature.Feature(th, featureContent()),
		gap,
		pricing.Pricing(th, pricingContent()),
		gap,
		testimonial.Testimonial(th, testimonialContent()),
	},
})
```

The props are plain data — `hero.Props{Eyebrow, Title, Subtitle, PrimaryCTA,
SecondaryCTA}`, `feature.Props{Columns, Items}` — so the copy lives in its own
file and the layout file stays structural.

A table is columns plus a row stream: `Columns` plus an `Items` observable,
sort and filter left to the caller. This is `ExampleTable`, verbatim, from
[`table/example_test.go`](./table/example_test.go):

```go
type row struct {
	ID    int
	Name  string
	Value float64
}

shaper := tokens.DefaultTypography.DeterministicShaper()
textCell := func(s string) layout.Widget {
	return table.RenderTextCell(shaper, tokens.PlatformLight, tokens.DefaultTypography.SystemFont, s)
}

columns := []table.Column[row]{
	{Header: "ID", Width: unit.Dp(60), Cell: func(r row) layout.Widget { return textCell(strconv.Itoa(r.ID)) }},
	{Header: "Name", Cell: func(r row) layout.Widget { return textCell(r.Name) }},
	{Header: "Value", Width: unit.Dp(120), Cell: func(r row) layout.Widget { return textCell(strconv.FormatFloat(r.Value, 'f', 2, 64)) }},
}

rows := []row{
	{ID: 1, Name: "Alpha", Value: 1.5},
	{ID: 2, Name: "Beta", Value: 3},
	{ID: 3, Name: "Gamma", Value: 4.5},
	{ID: 4, Name: "Delta", Value: 6},
}

_ = table.Table(rx.Of(theme.Default()), table.Props[row]{
	Columns: columns,
	Items:   rx.Of(rows),
})
```

`Cell` is called fresh for every visible row on every frame, so the table holds
no per-row state. Anything stateful in a cell — a checkbox, an editor, a
per-row confirm popover — is kept alive by the caller through
`components/keyed.Defer`, which returns the same pointer for the same row key across
sort, filter and pagination:

```go
checkClicks := keyed.Defer(func(int) *widget.Clickable { return &widget.Clickable{} })

checkboxCell := func(r row) layout.Widget {
	click := checkClicks.For(r.idx) // r.idx is the absolute row index
	return func(gtx layout.Context) layout.Dimensions {
		if click.Clicked(gtx) {
			mvu.MessageOp{Message: ToggleSelect{Row: r.idx}}.Add(gtx.Ops)
		}
		// ... click.Layout, semantic label, draw
	}
}
```

Overlays are folded onto the shell stream and drawn after it, reporting the
shell's dimensions — the dialog scrim and the notifications column both need
the whole window:

```go
notesObs := rx.Map(modelObs, func(m Model) []notifications.Notification { return m.notes.Items() })
notesColumnObs := notifications.Column(th, notifications.Props{
	Position:      notifications.TopRight,
	Notifications: notesObs,
})

return rx.Map(rx.CombineLatest3(shellObs, dialogObs, notesColumnObs),
	func(n rx.Tuple3[layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
		shellW, dialogW, notesW := n.First, n.Second, n.Third
		return func(gtx layout.Context) layout.Dimensions {
			dims := shellW(gtx)
			if dialogW != nil {
				dialogW(gtx)
			}
			if notesW != nil {
				notesW(gtx)
			}
			return dims
		}
	},
)
```

A notification is raised by message, never drawn in place. Inside a frame,
`notifications.Notify(gtx, toast.Success, "Feed added")` lands
`notifications.Requested` on the ops queue; from a command goroutine,
`notifications.Request(toast.Success, "Saved")` is the message to return. The
status is `components/toast`'s, because the toast is what speaks it. The
application reduces both onto a `notifications.Queue` it holds in its model,
and the expiry comes back the same way:

```go
case notifications.Requested:
	queue, n := model.notes.Add(m)
	model.notes = queue
	return model, notifications.Expire(n.ID, n.Lifetime)
case notifications.Expired:
	model.notes = model.notes.Remove(m.ID)
```

## For coding assistants

Read this repository's [AGENTS.md](./AGENTS.md) before you write code against this module.

Build and test the root module with `go build ./...` and `go test ./...`.

To regenerate golden images, name the test packages and put the flag after them.
The `golden` package of the `components` module defines the flag `golden.update` in `golden/golden.go`.
Placed before the packages, `go test` tests the current directory only.
Name only packages that store golden images, because a test binary rejects a flag it never declared.

```sh
go test ./dialog -golden.update
```

## Status

Honest about what does not work yet:

- **`table` has no per-header slot of its own.** Headers are drawn internally from
  `Column.Header` strings, so anything else on a header — a tooltip, a filter
  affordance — has to be positioned by arithmetic over the column widths from
  outside, duplicating the table's private header height to do it.
- **`shell`'s slots are mixed.** In `shell.Props`, `Sections` are
  `rx.Observable[layout.Widget]` but `Footer` is a plain `layout.Widget`, and
  so are `TwoColumn.Main`, `PaneFrame.Main` and `navbar.Props.Actions`. A live `layout.Widget` therefore
  has to be bridged into a plain slot through a cell the caller folds onto
  another stream.
- **Overlays open and close instantly.** `dialog` and `popover` have
  no entrance or exit transition; only `notifications` animates, and only the
  toast's fade-out, whose duration resolves from the theme's motion scale.
- **No responsive behaviour.** `feature`, `pricing` and `testimonial` do not
  collapse to fewer columns or a vertical stack on a narrow window, and
  `popover` does not flip to the anchor's other side when the chosen
  `Placement` has no room; it is only nudged along the cross axis.

## License

MIT — see [LICENSE](./LICENSE).
