package shell

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gioui.org/unit"
)

// arrangementFile is the name the arrangement is kept under, inside the
// application's own directory in the OS config directory:
//
//   - darwin:  ~/Library/Application Support/<app>/layout.json
//   - linux:   $XDG_CONFIG_HOME/<app>/layout.json (or ~/.config/...)
//   - windows: %AppData%\<app>\layout.json
//
// It stands beside the file the window's own frame is kept in, each
// damaged or missing on its own without costing the other.
const arrangementFile = "layout.json"

// arrangementDelay is how long the arrangement must stand still before it
// is written. It is the delay the window's own frame is kept on: dragging a
// boundary and dragging the window's edge are the same gesture at the same
// speed, so the two files settle together rather than one trailing the
// other.
const arrangementDelay = 500 * time.Millisecond

// Arrangement is what a window keeps of its own arrangement across
// launches: how wide the pane stands, how wide the inspector beside the
// content column stands, and whether the pane stands at all.
//
// The shown state is kept because a reader who sends the pane away and
// finds it back on the next launch is reading a defect.
//
// Widths are device-independent, which is what makes a kept width the same
// physical measure on a screen of a different pixel density.
type Arrangement struct {
	PaneWidth      unit.Dp `json:"pane"`
	InspectorWidth unit.Dp `json:"inspector"`
	PaneShown      bool    `json:"paneShown"`
}

// Clamp is the room one width is allowed. Below the minimum a column is
// narrower than what stands in it; above the maximum it is no longer a
// column beside the document but a second one competing with it.
type Clamp struct{ Min, Max unit.Dp }

// clamp answers the nearest width inside c, and w untouched where c states
// no room at all.
func (c Clamp) clamp(w unit.Dp) unit.Dp {
	if c.Max <= 0 {
		return w
	}
	if w < c.Min {
		return c.Min
	}
	if w > c.Max {
		return c.Max
	}
	return w
}

// Limits are the bounds the window states for the widths the shell keeps
// for it. What comes out of the file is a number a text editor can put
// there, and a window the reader cannot use is not worth obeying a file
// for, so everything arriving from outside passes through them.
type Limits struct{ Pane, Inspector Clamp }

// clamped answers the nearest arrangement these limits allow.
func (l Limits) clamped(a Arrangement) Arrangement {
	a.PaneWidth = l.Pane.clamp(a.PaneWidth)
	a.InspectorWidth = l.Inspector.clamp(a.InspectorWidth)
	return a
}

// plausible reports whether a describes real widths. A NaN answers false to
// every comparison a clamp makes and would come out of one untouched, so it
// is caught before the clamp rather than inside it.
func (a Arrangement) plausible() bool {
	for _, v := range []unit.Dp{a.PaneWidth, a.InspectorWidth} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	return true
}

// ArrangementMemory holds a window's arrangement between the frames that
// report it and the file it eventually lands in. A width is written after
// it stands still rather than once per frame, so one drag of a boundary
// produces one write.
//
// Every method takes a nil receiver, because a frame laid out for
// measurement rather than for a reader has no memory behind it — the stored
// renders above all, which arrange a window nobody is sitting at and must
// not write over what a real one kept.
type ArrangementMemory struct {
	// saved is what the window opens with: what was kept, where something
	// was kept that can be laid out, and the defaults otherwise. It is read
	// once at construction and never changes.
	saved  Arrangement
	limits Limits

	path  string
	delay time.Duration
	// write is the one seam this type has. It is a field so a test can
	// count the writes a burst produces without going near a file system.
	write func(Arrangement) error

	mu sync.Mutex
	// current is the last arrangement handed in, the opening one included;
	// pending says whether it still has to be written. have separates the
	// opening arrangement — which is either what the file holds or the
	// defaults, and neither is news — from every later change.
	current Arrangement
	have    bool
	pending bool
	timer   *time.Timer
}

// RememberArrangement opens the arrangement kept for appName, which is the
// key the window gives the shell. def is what a window with nothing kept
// opens on, and limits bound what a kept file may say.
//
// An error means the OS could not say where its config directory is, and
// the nil memory it comes with is a working one: the window opens on the
// defaults and keeps nothing.
func RememberArrangement(appName string, def Arrangement, limits Limits) (*ArrangementMemory, error) {
	if appName == "" {
		return nil, errors.New("shell: remembering an arrangement needs an application name")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return OpenArrangement(filepath.Join(dir, appName, arrangementFile), arrangementDelay, def, limits), nil
}

// OpenArrangement is [RememberArrangement] over a stated path and delay: it
// reads path and returns the memory a window arranges itself from and
// writes back through.
func OpenArrangement(path string, delay time.Duration, def Arrangement, limits Limits) *ArrangementMemory {
	saved := limits.clamped(def)
	if kept, ok := loadArrangement(path); ok {
		saved = limits.clamped(kept)
	}
	m := &ArrangementMemory{saved: saved, limits: limits, path: path, delay: delay}
	m.write = func(a Arrangement) error { return saveArrangement(m.path, a) }
	return m
}

// Arrangement is what the window opens with, the defaults where there is no
// memory at all.
func (m *ArrangementMemory) Arrangement() Arrangement {
	if m == nil {
		return Arrangement{}
	}
	return m.saved
}

// Record takes in one laid-out arrangement. The first writes nothing: it is
// what the window opened with. An arrangement equal to the last one is
// likewise nothing to write, which is what most frames hand in, since a
// frame is laid out whether or not a boundary moved in it.
//
// Anything else restarts the delay, so a drag collapses into the one write
// that follows it.
func (m *ArrangementMemory) Record(a Arrangement) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.have {
		m.current, m.have = a, true
		return
	}
	if a == m.current {
		return
	}
	m.current = a
	m.pending = true
	if m.timer == nil {
		m.timer = time.AfterFunc(m.delay, m.flush)
		return
	}
	m.timer.Reset(m.delay)
}

// flush writes what is pending, if anything. It runs on the timer's own
// goroutine and, through [ArrangementMemory.Stop], on the render goroutine
// as the window closes; a failed write is dropped rather than retried,
// because an arrangement nobody could store is not worth a second window's
// worth of attention.
func (m *ArrangementMemory) flush() {
	if m == nil {
		return
	}
	m.mu.Lock()
	pending, a := m.pending, m.current
	m.pending = false
	m.mu.Unlock()
	if !pending {
		return
	}
	_ = m.write(a)
}

// Stop writes whatever is pending and stops the timer. The window is going
// away, and a boundary the reader moved inside the delay is exactly the
// change they most expect to find again.
func (m *ArrangementMemory) Stop() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.timer != nil {
		m.timer.Stop()
	}
	m.mu.Unlock()
	m.flush()
}

// loadArrangement reads one arrangement from path. Every way of having
// nothing to apply — no file, an unreadable one, damaged JSON, a width that
// is not a number — answers the same false, because they all mean the same
// thing to the caller: open on the defaults.
func loadArrangement(path string) (Arrangement, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Arrangement{}, false
	}
	var a Arrangement
	if err := json.Unmarshal(data, &a); err != nil {
		return Arrangement{}, false
	}
	if !a.plausible() {
		return Arrangement{}, false
	}
	return a, true
}

// saveArrangement writes a to path, creating the intermediate directories.
func saveArrangement(path string, a Arrangement) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
