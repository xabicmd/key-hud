package capture

import "sync/atomic"

// This file owns the intro screen: the three lines the window shows on launch
// and the single key that leaves them. The page paints them; the gate below
// decides when the strip starts showing what you type.
//
// The gate sits before the input pipeline, not after it: while the intro is on
// screen a key is dropped without touching the grouping, lane or repeat state,
// so the first label after the start lands exactly where it would on a strip
// that had just been opened.
//
// It is an explicit value passed in Options, not a package global: a pipeline
// without a gate simply has no intro, and nothing can install one behind its
// back.

// Gate remembers whether the strip is still waiting for its start key.
// The hook thread reads it on every input; main waits for the start signal.
type Gate struct {
	pending atomic.Bool // the intro is still on screen
	eaten   atomic.Bool // the start press is still held, so its repeats stay silent
	started chan struct{}
}

// NewGate arms the intro. The pipeline built with this gate stays silent until
// WaitStart returns.
func NewGate() *Gate {
	g := &Gate{started: make(chan struct{}, 1)}
	g.pending.Store(true)
	return g
}

// Pending reports whether the intro is still on screen.
func (g *Gate) Pending() bool {
	return g.pending.Load()
}

// PassKey reports whether the input pipeline should see this key transition.
//
// While the intro is on screen only Enter is meaningful: it starts the strip
// and is swallowed, and every other key is dropped. The start press stays
// silent until it is released, so holding Enter does not paint an ENTER label
// over the fresh screen.
func (g *Gate) PassKey(vk uint32, down bool) bool {
	if vk == vkReturn && g.eaten.Load() {
		if !down {
			g.eaten.Store(false)
		}
		return false
	}
	if !g.Pending() {
		return true
	}
	if down && vk == vkReturn && g.pending.CompareAndSwap(true, false) {
		g.eaten.Store(true)
		// The hook callback runs inside a Windows timeout window: signal
		// without blocking and let main do the WebView2 call.
		select {
		case g.started <- struct{}{}:
		default:
		}
	}
	return false
}

// PassMouse reports whether the input pipeline should see a mouse event. The
// intro is keyboard-only: a click must not paint a label behind the text.
func (g *Gate) PassMouse() bool {
	return !g.Pending()
}

// WaitStart blocks until the start key arrives.
func (g *Gate) WaitStart() {
	<-g.started
}
