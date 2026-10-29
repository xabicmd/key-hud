package capture

import "testing"

// The intro is the only moment when the strip must stay silent. These tests pin
// the gate the hook consults before any key becomes a label; the drawing itself
// is covered by the Node tests in tests/splash.test.mjs.

func TestSplashMutesEverythingUntilEnter(t *testing.T) {
	g := NewGate()

	if g.PassKey('A', true) {
		t.Fatal("a letter reached the strip while the intro was on screen")
	}
	if g.PassMouse() {
		t.Fatal("a mouse event reached the strip while the intro was on screen")
	}
	if !g.Pending() {
		t.Fatal("ordinary input dismissed the intro")
	}
}

func TestSplashEnterStartsAndIsConsumed(t *testing.T) {
	g := NewGate()

	if g.PassKey(vkReturn, true) {
		t.Fatal("the start key must not become a label")
	}
	if g.Pending() {
		t.Fatal("Enter did not dismiss the intro")
	}
	if !g.PassKey('A', true) {
		t.Fatal("the strip stayed muted after the start key")
	}
	if !g.PassMouse() {
		t.Fatal("mouse events stayed muted after the start key")
	}
}

func TestSplashKeepsAHeldEnterSilent(t *testing.T) {
	g := NewGate()
	g.PassKey(vkReturn, true)

	if g.PassKey(vkReturn, true) {
		t.Fatal("an auto-repeat of the start key leaked a label")
	}
	if g.PassKey(vkReturn, false) {
		t.Fatal("the release of the start key leaked an event")
	}
	if !g.PassKey(vkReturn, true) {
		t.Fatal("a fresh Enter press must reach the strip once it started")
	}
}

func TestSplashSignalsStartExactlyOnce(t *testing.T) {
	g := NewGate()
	g.PassKey(vkReturn, true)
	g.PassKey(vkReturn, false)
	g.PassKey(vkReturn, true)

	select {
	case <-g.started:
	default:
		t.Fatal("the start key did not signal the page")
	}
	select {
	case <-g.started:
		t.Fatal("the start signal fired twice")
	default:
	}
}
