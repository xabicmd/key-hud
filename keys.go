package main

import (
	"fmt"
	"strings"
)

// Virtual-key codes used to detect modifier state.
const (
	vkShift    = 0x10
	vkControl  = 0x11
	vkAlt      = 0x12
	vkLShift   = 0xA0
	vkRShift   = 0xA1
	vkLControl = 0xA2
	vkRControl = 0xA3
	vkLAlt     = 0xA4
	vkRAlt     = 0xA5
	vkLWin     = 0x5B
	vkRWin     = 0x5C
	vkWin      = 0x5B
	vkSpace    = 0x20
)

// spaceRune is what a bare space leaves behind: the blank character itself, so
// the word it closes gets a trailing gap instead of the word "space". The name
// "space" is still used inside combinations such as "ctrl+space".
const spaceRune = " "

// modOrder is the order modifiers are rendered in a combination.
var modOrder = []struct {
	vk   uint32
	name string
}{
	{vkControl, "ctrl"},
	{vkShift, "shift"},
	{vkAlt, "alt"},
	{vkWin, "win"},
}

func isModifier(vk uint32) bool {
	switch vk {
	case vkControl, vkShift, vkAlt, vkLControl, vkRControl, vkLShift, vkRShift, vkLAlt, vkRAlt, vkLWin, vkRWin:
		return true
	}
	return false
}

// canonMod collapses left/right variants (and the generic codes Windows
// sometimes reports) into a single canonical modifier code.
func canonMod(vk uint32) uint32 {
	switch vk {
	case vkLControl, vkRControl:
		return vkControl
	case vkLShift, vkRShift:
		return vkShift
	case vkLAlt, vkRAlt:
		return vkAlt
	case vkLWin, vkRWin:
		return vkWin
	}
	return vk
}

// specialKeys names the keys that are not typed text. Arrow keys keep their
// glyph: "→" reads at a glance, where "right" has to be read word by word.
var specialKeys = map[uint32]string{
	0x08: "backspace",
	0x09: "tab",
	0x0D: "enter",
	0x13: "pause",
	0x14: "caps",
	0x1B: "esc",
	0x20: "space",
	0x21: "page up",
	0x22: "page down",
	0x23: "end",
	0x24: "home",
	0x25: "←",
	0x26: "↑",
	0x27: "→",
	0x28: "↓",
	0x2C: "print screen",
	0x2D: "insert",
	0x2E: "delete",
	0x5B: "win",
	0x5C: "win",
	0x5D: "menu",
	0x90: "num lock",
	0x91: "scroll lock",
	0xA0: "shift",
	0xA1: "shift",
	0xA2: "ctrl",
	0xA3: "ctrl",
	0xA4: "alt",
	0xA5: "alt",
	0xBA: ";",
	0xBB: "=",
	0xBC: ",",
	0xBD: "-",
	0xBE: ".",
	0xBF: "/",
	0xC0: "`",
	0xDB: "[",
	0xDC: "\\",
	0xDD: "]",
	0xDE: "'",
	0x6A: "numpad *",
	0x6B: "numpad +",
	0x6D: "numpad -",
	0x6E: "numpad .",
	0x6F: "numpad /",
	0x60: "numpad 0",
	0x61: "numpad 1",
	0x62: "numpad 2",
	0x63: "numpad 3",
	0x64: "numpad 4",
	0x65: "numpad 5",
	0x66: "numpad 6",
	0x67: "numpad 7",
	0x68: "numpad 8",
	0x69: "numpad 9",
}

func keyLabel(vk uint32) string {
	if name, ok := specialKeys[vk]; ok {
		return name
	}
	if vk >= 0x70 && vk <= 0x87 {
		return fmt.Sprintf("f%d", vk-0x6F)
	}
	if vk >= 0x30 && vk <= 0x39 {
		return string(rune(vk))
	}
	if vk >= 0x41 && vk <= 0x5A {
		// Lowercase keeps single keys readable next to "ctrl+c".
		return strings.ToLower(string(rune(vk)))
	}
	return fmt.Sprintf("vk%02x", vk)
}
