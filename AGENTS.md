# AGENTS.md — patterns

The pattern layer of the Vibrant Gio design system: nineteen composed
application patterns assembled from components widgets and effects — shell,
navbar, sidebar, table, tabs, pagination, modal, popover, tooltip, toast,
alert, tag, card, accordion, breadcrumb, hero, feature, pricing and
testimonial.

**Layer.** Tier 4 of ADR-001's stack, `mvu → theme → components → effects →
patterns → markdown`, alongside markdown: composed patterns, and the top of
the design system proper. Its root module imports `components`, `effects`,
`mvu` and `theme`, and reaches `font` and `svg` through them. That
direction is measured rather than typed — `scripts/check-layers.sh --edges`
reports the graph and `scripts/sync-agents.sh` renders these sentences from
it — so correcting them here changes nothing. The other direction is
measured too and deliberately not written down: the gate checks the graph
both ways, but a public API's consumers are unknowable, so this file says
what its module needs and never who needs it.

**Read the canonical guide before you write code against this module.** It is
the organization's one agent guide — the module inventory with current tags,
the application skeleton, the MVU loop and rx semantics, typography, and the
pitfalls that are not guessable. It lives exactly once, in `vibrantgio/.github`,
and this file links it rather than copying it:

    https://raw.githubusercontent.com/vibrantgio/.github/master/llms.txt

**Module.** `github.com/vibrantgio/patterns`, one module at the repository
root.

**Build and test.** From the repository root:

    go build ./... && go test ./...

**Golden images.** Tests in 19 packages compare rendered output against
PNGs committed under `testdata/golden/`. They render through
`github.com/vibrantgio/components/golden`, which declares `-golden.update`
and is the organization's only golden harness. Do not inline a copy of it,
and do not declare a second `-golden.update`: two registrations of one flag
name in a single test binary panic in `flag.Bool` at init, before any test
runs. When a change legitimately moves pixels, regenerate them within the
same change, look at what came out, and say so in the commit. From the
repository root:

    go test ./accordion ./alert ./breadcrumb ./card ./feature ./hero ./modal ./navbar ./pagination ./popover ./pricing ./shell ./sidebar ./table ./tabs ./tag ./testimonial ./toast ./tooltip -golden.update

Both halves of that line matter. `go test` cannot tell that an unfamiliar
flag is boolean, so a flag placed before the packages swallows them: `go
test -golden.update ./...` tests whatever package the repository root
holds, not `./...`. And `./...` cannot stand in for the list — this module
has test packages that store no goldens, and a test binary rejects a flag
it never declared.

**A green CI run does not say these images matched. They are compared only
on a developer's machine, and that is deliberate.** The harness answers a
failed `headless.NewWindow` with `t.Skipf`, a skipped test passes, and the
runner has no GL driver for it to open — so the pixels and the build status
are independent facts. The `build` job's *Were the golden images compared,
or skipped?* step, added by F5.4, publishes which of the two happened as a
workflow annotation, readable without a token at `GET
/repos/vibrantgio/patterns/commits/<sha>/check-runs`; it has answered
SKIPPED on every run. F5.7 then measured the alternative rather than
leaving it as an open question. Adding the drivers gio's own Linux CI
installs — `libegl1`, `libegl-mesa0`, `libglx-mesa0`, `libgl1-mesa-dri`,
`mesa-libgallium`, `libgbm1`, `mesa-vulkan-drivers` — does work: on pulse
the verdict flipped to COMPARED on the next run. Nine of that repository's
twenty-one images then failed, 12782 pixels apart, while the three drawn on
the CPU still matched exactly. Every golden in the organization was
recorded on macOS, so the gate would not be asserting that the images are
right, only that Linux mesa and Metal rasterise identically — which they do
not, and need not. **So CI gates the build and the tests, never the
pixels**, and moving an image is checked where it is regenerated.

**A golden test pins its faces; application code does not.** Every golden
and pixel test here builds its shaper with
`tokens.DefaultTypography.DeterministicShaper()` — the default typography's
faces and nothing else, system fonts off, so the stored PNGs are the same
on every machine. Applications call `Shaper()` instead, which falls back to
the platform's own fonts so that text outside Roboto and Roboto Mono still
resolves. The two are not interchangeable: a golden written against
`Shaper()` passes on the machine that wrote it and fails on one with a
different font set, which is the failure the split constructor exists to
make impossible.

When a test genuinely needs a glyph the default faces lack, widen the
collection rather than reach for the system:

    tokens.DefaultTypography.WithFaces(notosansmono.FontFace()).DeterministicShaper()

Then assert that the shaper resolved the rune, rather than storing the
result as pixels. A stored image proves the glyph came out somewhere; only
the assertion says which face drew it.

**Coordination in this repository is ADR-008's, and none of it is a bus.**
Four packages here used to export an `rx.Observable` that widgets published
to — `popover.Arbitration`, `tooltip.Arbitration`, `modal.Stack` and
`toast.Notifications`. Three of the four had no subscriber anywhere in the
organization; all four are gone, along with their snapshot types. What
replaced them:

- **`popover`, `tooltip` and `modal` arbitrate through a plain value.**
  `NewArbiter()` returns one, `Props.Arbiter` takes it, and the value *is* the
  scope — widgets sharing an arbiter arbitrate with one another and with
  nobody else, which makes one per window the right grain. There is no
  package-level default: a nil `Arbiter` means *arbitrate alone*, so two
  popovers that both leave it nil stay open together. No mutex and no atomics,
  because one Gio frame runs on one goroutine, and the doc comment on each
  arbiter says so — that sentence is the invariant.
- **A claim must be an edge, and a level must be latched.** The claim happens
  on the first frame a widget is drawn open, and it dismisses the incumbent
  from inside the claimant's own layout pass. Under the old per-frame poll a
  level-guarded claim was survivable; under the direct write two participants
  trade the top every frame. `tooltip`'s dwell is the case that found this.
- **`toast` is the one that went to the model.** `toast.Requested` and
  `toast.Expired` are messages, `toast.Queue` is the state they reduce onto,
  `toast.Expire(id, lifetime)` is the timer command, and `Props.Toasts` is how
  the stack reads them. `Notify` kept its name and broke its signature — it
  needs a `layout.Context` to reach `mvu.MessageOp`, and there is no shim that
  both compiles and works, so it fails loudly at every call site instead.
  A `Stack` handed no toasts renders an empty column forever and nothing
  fails at build time; `TestStackWithNoToastsRendersEmpty` pins that.

`scripts/check-subjects.sh` in `vibrantgio/.github` is the gate. It will not
catch the thing that actually went wrong here — an exported observable with no
reader looks exactly like a working one — so when you export an observable
from this repository, go and find its subscribers first.

**Never let a golden helper close over the parent test's `*testing.T`.** Pass
the subtest's own `t` down as a parameter. This is not style: it is why this
repository was red on all sixteen CI runs between B3.5 and F5.7 while green on
every developer machine, and the failure is invisible in both directions until
you know it.

`sidebar`'s active-tint test built its `render` closure in the parent test's
scope and then called it from inside two `t.Run` subtests, where an inner `t`
shadows the outer name at every call site but not inside the closure. Locally
that never matters, because `headless.NewWindow` succeeds and the harness
never reaches its skip path. On a runner it does: `golden.Capture` answers
with `t.Skipf`, `t.Skipf` is `runtime.Goexit`, and a Goexit taken on the
parent's `t` from the subtest's goroutine unwinds the subtest without
finishing it. The testing package reports that as

    test executed panic(nil) or runtime.Goexit:
    subtest may have called FailNow on a parent test

which is a failure, not a skip — so the repository failed for exactly the
reason its images were never compared.

F5.5 deleted the eighteen inlined harnesses that used to live here, one per
component package.
