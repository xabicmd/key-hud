package capture

import (
	"strings"
	"time"

	"keyhud/internal/win32"
)

// This file turns a decoded Windows message into one instruction for the UI:
// what the key types, whether it is a combination, or whether it is a repeated
// gesture that must stay silent.

// Options configures one pipeline. The zero value is usable: the gate is
// absent, synthetic input is shown and grouping falls back to the defaults.
type Options struct {
	// Gate, when set, filters input before it can become a label. The intro
	// screen uses it to stay silent until the strip starts.
	Gate *Gate

	// IgnoreInjected drops synthetic input (macros, remote sessions).
	IgnoreInjected bool

	// GroupWindow is how long a pause keeps two keystrokes in one word.
	GroupWindow time.Duration

	// Lanes is how many vertical lanes the strip rotates through.
	Lanes int
}

// Pipeline owns everything the input side remembers between two events. It is
// created once by main and used only from the hook thread, which is locked to
// its OS thread, so it carries no locking.
//
// Keeping the state in a value instead of package variables is what makes the
// dependency explicit: a pipeline needs a gate, a grouping window and a lane
// count to exist, and nothing else can reach into it by accident.
type Pipeline struct {
	gate           *Gate
	ignoreInjected bool
	events         chan Event
	grouper        *grouper

	// readKeyText asks the active layout what a key press types. It is a field
	// so tests can answer without depending on the installed layout.
	readKeyText func(vk, scan uint32) keyText

	pressed      map[uint32]bool
	heldMods     map[uint32]bool
	rightAlt     bool // AltGr is right alt plus the ctrl Windows adds for it
	lastAction   uint32
	lastActionAt time.Time
	lastWheel    time.Time

	// The wheel has no virtual-key code, so its repeat run is keyed by the
	// direction that was shown last: spinning the wheel up ten notches is one
	// gesture and shows one label, exactly like holding an action key.
	lastWheelRun   string
	lastWheelRunAt time.Time
}

// New builds the input pipeline. It does not install any hook: Start does.
func New(opts Options) *Pipeline {
	if opts.GroupWindow <= 0 {
		opts.GroupWindow = 500 * time.Millisecond
	}
	p := &Pipeline{
		gate:           opts.Gate,
		ignoreInjected: opts.IgnoreInjected,
		grouper:        newGrouper(opts.GroupWindow, opts.Lanes),
		pressed:        map[uint32]bool{},
		heldMods:       map[uint32]bool{},
	}
	p.readKeyText = p.layoutKeyText
	return p
}

// onKey decodes one raw keyboard hook message and feeds the pipeline.
func (p *Pipeline) onKey(lParam uintptr, down bool) {
	kb := (*win32.KbdLLHookStruct)(win32.PtrAt(lParam))
	if p.ignoreInjected && kb.Flags&win32.LLKHF_INJECTED != 0 {
		return
	}
	// The intro screen swallows input before it becomes an instruction, so
	// nothing below can advance a lane or a repeat run behind it.
	if p.gate != nil && !p.gate.PassKey(kb.VkCode, down) {
		return
	}
	scan := kb.ScanCode
	if scan == 0 {
		scan = scanCode(kb.VkCode)
	}
	p.handleKey(kb.VkCode, scan, down)
}

// handleKey applies one key transition and returns the instruction to publish,
// if any. Auto-repeats, modifier bookkeeping and key releases produce none.
//
// What a key types is asked to the active layout, so the strip shows the
// character that really appears: "@" instead of "shift+2", "á" instead of a
// dead accent followed by "a". Only presses that type nothing (ctrl+c,
// alt+tab, backspace, f5) are shown as keys.
//
// Only real modifiers become a combination prefix. A letter that is still held
// down because the next key was pressed a few milliseconds later is just fast
// typing, not "a+b"; treating it as a modifier is what turned "hola" into
// "ho" + "l+a".
func (p *Pipeline) handleKey(vk, scan uint32, down bool) (Event, bool) {
	if !down {
		delete(p.pressed, vk)
		if isModifier(vk) {
			delete(p.heldMods, canonMod(vk))
			if vk == vkRAlt {
				p.rightAlt = false
			}
		}
		return Event{}, false
	}

	// Auto-repeat: Windows keeps sending keydowns while a key is held. Show the
	// press once, not sixty times a second.
	if p.pressed[vk] {
		return Event{}, false
	}
	p.pressed[vk] = true

	if isModifier(vk) {
		p.heldMods[canonMod(vk)] = true
		if vk == vkRAlt {
			p.rightAlt = true
		}
		return Event{}, false
	}

	if p.typingModifiers() {
		switch typed := p.readKeyText(vk, scan); {
		case typed.dead:
			// The accent waits for the next key. Showing a label now would
			// claim a character that is not on screen yet.
			return Event{}, false
		case typed.text != "":
			p.clearActionRun()
			if typed.text == spaceRune {
				// A bare space ends the word and leaves the blank behind it,
				// but it never takes a lane: an invisible label would make the
				// visible words look like they skip a line.
				return p.emit(kindBreak, spaceRune), true
			}
			return p.emit(kindKey, typed.text), true
		}
	}

	if len(p.heldMods) > 0 {
		p.clearActionRun()
		return p.emit(kindCombo, p.buildCombo(vk)), true
	}

	// Backspace, tab, an arrow, page up/down, home/end: pressed again and
	// again, it is one gesture and it shows once.
	if p.repeatsAction(vk) {
		return Event{}, false
	}
	// An action key ends the wheel run: a scroll after backspace is not the
	// same gesture as the scroll before it.
	p.lastWheelRun, p.lastWheelRunAt = "", time.Time{}
	return p.emit(kindAction, keyLabel(vk)), true
}

// repeatsAction reports whether this action key continues the run of the same
// key that is already on screen. Holding a key repeats it by itself, and
// deleting ten characters is ten presses of one key: showing ten BACKSPACE
// labels describes the implementation, not what the user did.
func (p *Pipeline) repeatsAction(vk uint32) bool {
	now := time.Now()
	same := vk == p.lastAction && now.Sub(p.lastActionAt) <= p.grouper.window
	p.lastAction, p.lastActionAt = vk, now
	return same
}

// clearActionRun ends the run, so the same action key after a typed character
// or a combination shows again. It also ends the wheel run: a scroll is only
// one gesture while nothing else happens in between.
func (p *Pipeline) clearActionRun() {
	p.lastAction, p.lastActionAt = 0, time.Time{}
	p.lastWheelRun, p.lastWheelRunAt = "", time.Time{}
}

// onButton publishes one mouse button or wheel movement. The vertical wheel
// repeats like an action key: scrolling up several notches is one gesture and
// shows one label, because a fast wheel would otherwise push everything else
// off the strip. The sideways wheel and the buttons keep their own handling.
func (p *Pipeline) onButton(name string) (Event, bool) {
	// The intro is keyboard-only: a click must not paint a label behind it.
	if p.gate != nil && !p.gate.PassMouse() {
		return Event{}, false
	}

	now := time.Now()
	if name == "wheel up" || name == "wheel down" {
		same := name == p.lastWheelRun && now.Sub(p.lastWheelRunAt) <= p.grouper.window
		p.lastWheelRun, p.lastWheelRunAt = name, now
		if same {
			return Event{}, false
		}
		// The scroll ends a run of action keys: backspace after it shows
		// again.
		p.lastAction, p.lastActionAt = 0, time.Time{}
		return p.emit(kindMouse, p.buildComboLabel(name)), true
	}
	// A fast sideways wheel can fire dozens of events per second and would
	// push everything else off the strip.
	if strings.HasPrefix(name, "wheel") {
		if now.Sub(p.lastWheel) < 70*time.Millisecond {
			return Event{}, false
		}
		p.lastWheel = now
	}
	p.clearActionRun()
	return p.emit(kindMouse, p.buildComboLabel(name)), true
}

// typingModifiers reports whether the modifiers held only shift which
// character a key types: shift alone, or AltGr. Right alt is AltGr on Windows
// (the system adds the ctrl that the layouts expect for it). Ctrl, left alt,
// win or any other combination is a command, and a command never types a
// character.
func (p *Pipeline) typingModifiers() bool {
	for m := range p.heldMods {
		switch m {
		case vkShift:
		case vkControl, vkAlt:
			if !p.rightAlt {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// buildCombo prefixes the active modifiers to the name of a virtual key.
func (p *Pipeline) buildCombo(vk uint32) string {
	return p.buildComboLabel(keyLabel(vk))
}

// buildComboLabel prefixes the active modifiers to the given key name:
// "ctrl+shift+p". Other keys held at the same time are ignored on purpose.
func (p *Pipeline) buildComboLabel(key string) string {
	parts := make([]string, 0, 5)
	for _, m := range modOrder {
		if p.heldMods[m.vk] {
			parts = append(parts, m.name)
		}
	}
	parts = append(parts, key)
	return strings.Join(parts, "+")
}

// emit asks the grouper what to do with this input and hands the resulting
// instruction to the UI. It never blocks: it must not stall the hook callback.
// It returns the instruction so callers and tests can see it.
func (p *Pipeline) emit(kind, label string) Event {
	ev := p.grouper.feed(time.Now(), kind, label)
	select {
	case p.events <- ev:
	default:
	}
	return ev
}
