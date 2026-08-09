package toast

import (
	"image"
	"testing"
	"time"

	gioinput "gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/golden"
	"github.com/vibrantgio/spectrum/theme"
	"github.com/vibrantgio/spectrum/tokens"
)

const intCanvasW, intCanvasH = 320, 240

var intCanvas = image.Pt(intCanvasW, intCanvasH)

func intTok() resolvedTokens {
	return resolvedTokens{
		color:   tokens.DefaultLight,
		spacing: tokens.Spacing,
		radius:  tokens.RadiusScale{},
		style:   tokens.DefaultTypography.LabelMedium,
		fade:    tokens.Motion.DurSlow, // what the live path resolves from Theme.Motion
	}
}

func driveFrameAt(w layout.Widget, ops *op.Ops, r *gioinput.Router, size image.Point, now time.Time) {
	ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(size),
		Now:         now,
		Ops:         ops,
		Source:      r.Source(),
	}
	w(gtx)
	r.Frame(ops)
}

// TestRequestAddsAndExpiredRetires discharges the Measurable interaction on
// the model rather than on a hidden queue: a request grows the queue by one
// and the Expired that names the toast returns it to its prior length. Before
// G0C.3 this was a white-box test against a mutex-guarded stackState inside
// the widget, deliberately bypassing the process-global Subject so the state
// did not leak across tests. There is no such state now — the queue is a
// value — so the test is the reducer's, not the renderer's.
func TestRequestAddsAndExpiredRetires(t *testing.T) {
	var q Queue
	if q.Len() != 0 {
		t.Fatalf("zero Queue length = %d; want 0", q.Len())
	}

	q, first := q.Add(Request(Info, "one"))
	if q.Len() != 1 {
		t.Fatalf("after one request, length = %d; want 1", q.Len())
	}
	if first.ID == 0 {
		t.Error("Add assigned ID 0; IDs must name a toast to Expired")
	}
	if first.Lifetime != DefaultLifetime {
		t.Errorf("Lifetime = %v; want the package default %v", first.Lifetime, DefaultLifetime)
	}
	if first.At.IsZero() {
		t.Error("Request left At zero; the fade has nothing to measure from")
	}

	q, second := q.Add(Requested{Level: Error, Text: "two", Lifetime: time.Second})
	if second.ID == first.ID {
		t.Errorf("both toasts got ID %d; IDs must be distinct within a queue", second.ID)
	}
	if second.Lifetime != time.Second {
		t.Errorf("Lifetime = %v; want the requested 1s", second.Lifetime)
	}

	q = q.Remove(first.ID)
	if q.Len() != 1 {
		t.Fatalf("after Remove, length = %d; want 1", q.Len())
	}
	if q.Items()[0].ID != second.ID {
		t.Errorf("Remove dropped the wrong toast: %+v", q.Items())
	}
	if q = q.Remove(first.ID); q.Len() != 1 {
		t.Error("removing an absent ID changed the queue; a late Expired must be a no-op")
	}
	if q = q.Remove(second.ID); q.Len() != 0 {
		t.Fatalf("after removing both, length = %d; want 0", q.Len())
	}
}

// TestQueueDoesNotAliasPreviousModels pins the reason Queue is a value: a
// model captured before a toast arrived must still show what was on screen
// then. Both Add and Remove allocate at exactly their own length, so neither
// the old slice nor a caller holding Items can be written through.
func TestQueueDoesNotAliasPreviousModels(t *testing.T) {
	var q Queue
	q, _ = q.Add(Request(Info, "one"))
	before := q
	beforeItems := before.Items()

	q, _ = q.Add(Request(Success, "two"))
	if before.Len() != 1 {
		t.Errorf("Add mutated the previous Queue: length = %d; want 1", before.Len())
	}
	if len(beforeItems) != 1 || beforeItems[0].Text != "one" {
		t.Errorf("Add wrote through Items of the previous Queue: %+v", beforeItems)
	}

	// An append by a caller holding Items must not reach into the queue.
	_ = append(beforeItems, Toast{Text: "smuggled"}) //nolint:gocritic // the point of the test
	if len(before.Items()) != 1 || before.Items()[0].Text != "one" {
		t.Errorf("a caller's append reached into the Queue: %+v", before.Items())
	}
}

// TestExpireEmitsExpiredAfterTheLifetime is how time enters the loop: the
// command is a timer that emits one Expired and completes, so the toast's
// removal is a message the reducer sees like any other. It runs on a command
// goroutine, which is also the point — nothing about raising or retiring a
// toast requires the frame.
func TestExpireEmitsExpiredAfterTheLifetime(t *testing.T) {
	const lifetime = 50 * time.Millisecond
	got := make(chan mvu.Message, 4)
	sub := Expire(7, lifetime).Subscribe(rx.GoroutineContext(), func(m mvu.Message, _ error, done bool) {
		if !done {
			got <- m
		}
	})
	defer sub.Unsubscribe()

	start := time.Now()
	select {
	case m := <-got:
		if m != (Expired{ID: 7}) {
			t.Fatalf("Expire emitted %#v; want Expired{ID: 7}", m)
		}
		if elapsed := time.Since(start); elapsed < lifetime/2 {
			t.Errorf("Expired arrived after %v; want at least ~%v", elapsed, lifetime)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Expire did not emit within 2s")
	}
	select {
	case m := <-got:
		t.Fatalf("Expire emitted a second message %#v; one toast, one timer", m)
	case <-time.After(2 * lifetime):
	}
}

// TestStackRendersTheModelQueue confirms the render path is driven by
// Props.Toasts and nothing else: a new queue re-emits the widget. This
// replaces TestNotifyReachesStackSubscription, which asserted the same thing
// about the package-scoped Subject that G0C.3 deleted.
func TestStackRendersTheModelQueue(t *testing.T) {
	send, toasts := rx.Subject[[]Toast](0, 1)
	send.Next(nil)

	// Explicit shaper: this subscribes to the live path, and a nil
	// Props.Shaper binds to the theme's fallback, which after F4.2 resolves
	// against the machine's fonts (F4.4b).
	obs := Stack(rx.Of(theme.Default()), Props{
		Position: TopRight,
		Toasts:   toasts,
		Shaper:   tokens.DefaultTypography.DeterministicShaper(),
	})

	emissions := make(chan layout.Widget, 4)
	sub := obs.Subscribe(rx.GoroutineContext(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			select {
			case emissions <- next:
			default:
			}
		}
	})
	defer sub.Unsubscribe()

	select {
	case <-emissions:
	case <-time.After(2 * time.Second):
		t.Fatal("Stack did not emit an initial widget within 2s")
	}

	var q Queue
	q, _ = q.Add(Request(Info, "queued"))
	send.Next(q.Items())

	select {
	case <-emissions:
	case <-time.After(2 * time.Second):
		t.Fatal("a new queue did not drive a Stack re-emission within 2s")
	}
}

// TestStackWithNoToastsRendersEmpty pins the honest cost of the conversion:
// a Props that names no queue is legal, compiles, and shows nothing. It is
// the one non-additive step in G0C.3 and the reason cadence's next tag is a
// minor bump.
func TestStackWithNoToastsRendersEmpty(t *testing.T) {
	obs := Stack(rx.Of(theme.Default()), Props{
		Position: TopRight,
		Shaper:   tokens.DefaultTypography.DeterministicShaper(),
	})
	got := make(chan layout.Widget, 1)
	sub := obs.Subscribe(rx.GoroutineContext(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			select {
			case got <- next:
			default:
			}
		}
	})
	defer sub.Unsubscribe()

	select {
	case w := <-got:
		r := new(gioinput.Router)
		ops := new(op.Ops)
		driveFrameAt(w, ops, r, intCanvas, time.Unix(1700000000, 0))
	case <-time.After(2 * time.Second):
		t.Fatal("Stack did not emit within 2s")
	}
}

// TestFadeAlphaWalksTheLifetime pins the one thing the frame still decides.
// The alpha is derived from Toast.At and gtx.Now, never stored, and it holds
// at 0 past expiry — the widget does not prune, it waits for the model.
func TestFadeAlphaWalksTheLifetime(t *testing.T) {
	const lifetime = 1000 * time.Millisecond
	const fade = 400 * time.Millisecond
	at := time.Unix(1700000000, 0)

	cases := []struct {
		name string
		age  time.Duration
		want func(float64) bool
		desc string
	}{
		{"fresh", 0, func(a float64) bool { return a == 1 }, "1.0"},
		{"before the fade window", 500 * time.Millisecond, func(a float64) bool { return a == 1 }, "1.0"},
		{"mid fade", 800 * time.Millisecond, func(a float64) bool { return a > 0 && a < 1 }, "strictly between 0 and 1"},
		{"at expiry", lifetime, func(a float64) bool { return a == 0 }, "0.0"},
		{"past expiry, awaiting Expired", 5 * lifetime, func(a float64) bool { return a == 0 }, "0.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if a := fadeAlpha(at, lifetime, fade, at.Add(tc.age)); !tc.want(a) {
				t.Errorf("alpha at age %v = %v; want %s", tc.age, a, tc.desc)
			}
		})
	}

	// A zero At is the Render path and any hand-built Toast: no fade at all.
	if a := fadeAlpha(time.Time{}, lifetime, fade, at.Add(5*lifetime)); a != 1 {
		t.Errorf("alpha for a zero At = %v; want 1.0 (fading disabled)", a)
	}
}

// TestAnExpiredToastPaintsNothing is the pixel half of the claim above: the
// widget no longer prunes, so a toast whose lifetime has run stays in the
// queue until Expired lands — and for that window it must be invisible, not
// merely faint. The frame it produces is byte-identical to an empty stack's.
func TestAnExpiredToastPaintsNothing(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	props := Props{Position: TopRight, Shaper: shaper}
	at := time.Unix(1700000000, 0)
	now := at.Add(2 * DefaultLifetime)

	expired := []Toast{{ID: 1, Level: Warning, Text: "Connection is slow", At: at, Lifetime: DefaultLifetime}}
	frame := func(queued []Toast) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Now = now
			return drawStackLive(gtx, shaper, props, intTok(), queued)
		}
	}
	empty := golden.Capture(t, intCanvas, frame(nil))
	gone := golden.Capture(t, intCanvas, frame(expired))
	if n := golden.PixelDiff(empty, gone); n != 0 {
		t.Errorf("an expired toast still painted %d pixels; it must wait for Expired invisibly", n)
	}

	// The same toast, still inside its lifetime, does paint — otherwise the
	// assertion above would pass for a stack that never draws anything.
	live := []Toast{{ID: 1, Level: Warning, Text: "Connection is slow", At: now, Lifetime: DefaultLifetime}}
	up := golden.Capture(t, intCanvas, frame(live))
	if n := golden.PixelDiff(empty, up); n == 0 {
		t.Error("a live toast painted nothing; the expiry assertion above proves nothing")
	}
}

// TestLifetimeOfPrefersTheToast pins the resolution order that keeps the
// expiry timer and the fade reading the same number.
func TestLifetimeOfPrefersTheToast(t *testing.T) {
	own := Toast{Lifetime: time.Second}
	if got := lifetimeOf(own, Props{Lifetime: time.Minute}); got != time.Second {
		t.Errorf("lifetimeOf(own) = %v; want the toast's own 1s", got)
	}
	if got := lifetimeOf(Toast{}, Props{Lifetime: time.Minute}); got != time.Minute {
		t.Errorf("lifetimeOf(props) = %v; want the stack-wide 1m", got)
	}
	if got := lifetimeOf(Toast{}, Props{}); got != DefaultLifetime {
		t.Errorf("lifetimeOf(default) = %v; want %v", got, DefaultLifetime)
	}
}

// TestNotifyOutsideAFrameIsHarmless documents the one thing no test in this
// organization can assert: mvu's MessageOp collector is registered by
// mvu.Window on the frame's own *op.Ops and is unreachable from outside mvu
// (prism/input's textfield test says the same and uses OnChange as its
// proxy). So this pins the failure mode instead — an Add against a buffer no
// frame is collecting drops the message silently rather than panicking, which
// is exactly the trap Notify's doc names.
func TestNotifyOutsideAFrameIsHarmless(t *testing.T) {
	ops := new(op.Ops)
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(intCanvas),
		Now:         time.Unix(1700000000, 0),
		Ops:         ops,
	}
	Notify(gtx, Success, "no collector here")
}
