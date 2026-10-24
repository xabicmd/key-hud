package main

import (
	"time"
	"unicode"
	"unicode/utf8"
)

// Event is one instruction for the browser: either start a new label on a lane,
// append text to the label that is currently travelling, or close the current
// word without starting a label (a bare space).
type Event struct {
	Kind  string `json:"kind"`            // "new", "append" or "space"
	Text  string `json:"text"`            // what to render
	Group string `json:"group,omitempty"` // colour group, only meaningful for "new"
	Lane  int    `json:"lane"`            // vertical lane, only meaningful for "new"
}

const (
	kindKey    = "key"    // types text, may join the current word
	kindAction = "action" // performs an action, always its own label
	kindCombo  = "combo"  // a key combination, always its own label
	kindBreak  = "break"  // ends the word without taking a lane (a bare space)
)

const (
	groupLetters = "letters"
	groupSymbols = "symbols"
	groupCombo   = "combo"
	groupMouse   = "mouse"
)

// classify decides which colour group a captured input belongs to. The kind
// decides it: a typed "+" is text, not a combination. Mouse wins over
// everything: a ctrl+click is still a mouse event.
func classify(kind, label string) string {
	switch kind {
	case groupMouse:
		return groupMouse
	case kindCombo:
		return groupCombo
	}
	if r, ok := loneRune(label); ok && unicode.IsLetter(r) {
		return groupLetters
	}
	return groupSymbols
}

func loneRune(s string) (rune, bool) {
	if utf8.RuneCountInString(s) != 1 {
		return 0, false
	}
	for _, r := range s {
		return r, true
	}
	return 0, false
}

// grouper turns a stream of individual inputs into labels.
//
// Typing quickly should read as one word: "hola" is a single label. A pause, an
// action key (space, backspace, tab…), a key combination or a mouse button ends
// the current label and starts the next one on the following lane.
//
// It is used from the hook thread only, so it carries no locking.
// maxLabelRunes stops an unbroken stream of keystrokes from growing a single
// label wider than the screen. It only ever splits plain typing; a combination
// is always one atomic label.
const maxLabelRunes = 16

type grouper struct {
	window time.Duration
	lanes  int

	cursor int // next lane to use
	active bool
	atomic bool // the current label was born from a combo, an action or a mouse event
	chars  int  // runes already shown in the current label
	last   time.Time
}

func newGrouper(window time.Duration, lanes int) *grouper {
	if lanes < 1 {
		lanes = 1
	}
	return &grouper{window: window, lanes: lanes}
}

// feed decides whether this input continues the label on screen or starts a
// fresh one, and returns the instruction for the browser.
func (g *grouper) feed(now time.Time, kind, label string) Event {
	if kind == kindBreak {
		// A bare space closes the word and leaves its blank behind it, but it
		// must not take a lane. The blank is invisible, so a lane of its own
		// would look like an empty line and push the words out of rotation:
		// "te" -> lane 1, [space] -> lane 2, "comparto" -> lane 3, and the user
		// sees 1, 3, 2 instead of 1, 2, 3.
		g.last = now
		g.active = false
		g.atomic = false
		return Event{Kind: "space", Text: label}
	}

	isMouse := kind == groupMouse
	atomic := isMouse || kind == kindAction || kind == kindCombo
	gap := now.Sub(g.last)

	joins := g.active && !g.atomic && !atomic &&
		gap <= g.window &&
		g.chars+utf8.RuneCountInString(label) <= maxLabelRunes

	g.last = now

	if joins {
		g.chars += utf8.RuneCountInString(label)
		return Event{Kind: "append", Text: label}
	}

	lane := g.cursor % g.lanes
	g.cursor++
	g.active = true
	g.atomic = atomic
	g.chars = utf8.RuneCountInString(label)

	return Event{Kind: "new", Text: label, Group: classify(kind, label), Lane: lane}
}
