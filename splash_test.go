package main

import "testing"

// The intro is the only moment when the strip must stay silent. These tests pin
// the gate the hook consults before any key becomes a label; the drawing itself
// is covered by the Node tests in tests/splash.test.mjs.

func TestSplashMutesEverythingUntilEnter(t *testing.T) {
	g := newSplashGate()

	if g.passKey('A', true) {
		t.Fatal("a letter reached the strip while the intro was on screen")
	}
	if g.passMouse() {
		t.Fatal("a mouse event reached the strip while the intro was on screen")
	}
	if !g.Pending() {
		t.Fatal("ordinary input dismissed the intro")
	}
}

func TestSplashEnterStartsAndIsConsumed(t *testing.T) {
	g := newSplashGate()

	if g.passKey(vkReturn, true) {
		t.Fatal("the start key must not become a label")
	}
	if g.Pending() {
		t.Fatal("Enter did not dismiss the intro")
	}
	if !g.passKey('A', true) {
		t.Fatal("the strip stayed muted after the start key")
	}
	if !g.passMouse() {
		t.Fatal("mouse events stayed muted after the start key")
	}
}

func TestSplashKeepsAHeldEnterSilent(t *testing.T) {
	g := newSplashGate()
	g.passKey(vkReturn, true)

	if g.passKey(vkReturn, true) {
		t.Fatal("an auto-repeat of the start key leaked a label")
	}
	if g.passKey(vkReturn, false) {
		t.Fatal("the release of the start key leaked an event")
	}
	if !g.passKey(vkReturn, true) {
		t.Fatal("a fresh Enter press must reach the strip once it started")
	}
}

func TestSplashSignalsStartExactlyOnce(t *testing.T) {
	g := newSplashGate()
	g.passKey(vkReturn, true)
	g.passKey(vkReturn, false)
	g.passKey(vkReturn, true)

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
