package capture

import (
	"encoding/json"
	"testing"
	"time"
)

// The page reads these JSON keys by name. Renaming a field here would only fail
// in the window, so the wire format is pinned against literal strings: the
// browser contract is part of the capture package's behaviour, not a detail.
func wire(t *testing.T, label string, ev Event, ok bool, want string) {
	t.Helper()
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("%s: marshal: %v", label, err)
	}
	if !ok {
		t.Fatalf("%s: dropped, want %s", label, want)
	}
	if string(b) != want {
		t.Fatalf("%s = %s, want %s", label, b, want)
	}
}

func TestEventWireFormatIsStable(t *testing.T) {
	p := New(Options{GroupWindow: 500 * time.Millisecond, Lanes: 3})
	p.readKeyText = func(vk, scan uint32) keyText {
		switch {
		case vk == 0x20:
			return keyText{text: " "}
		case vk >= 0x41 && vk <= 0x5A:
			return keyText{text: string(rune(vk + 0x20))}
		}
		return keyText{}
	}

	ev, ok := p.handleKey(0x41, 0, true)
	wire(t, "letter", ev, ok, `{"kind":"new","text":"a","group":"letters","lane":0}`)
	p.handleKey(0x41, 0, false)
	ev, ok = p.handleKey(0x42, 0, true)
	wire(t, "append", ev, ok, `{"kind":"append","text":"b","lane":0}`)
	p.handleKey(0x42, 0, false)
	ev, ok = p.handleKey(0x20, 0, true)
	wire(t, "space", ev, ok, `{"kind":"space","text":" ","lane":0}`)
	p.handleKey(0x20, 0, false)

	p.handleKey(0xA2, 0, true)
	ev, ok = p.handleKey(0x43, 0, true)
	wire(t, "combo", ev, ok, `{"kind":"new","text":"ctrl+c","group":"combo","lane":1}`)
	p.handleKey(0x43, 0, false)
	p.handleKey(0xA2, 0, false)

	ev, ok = p.onButton("wheel up")
	wire(t, "wheel up", ev, ok, `{"kind":"new","text":"wheel up","group":"mouse","lane":2}`)
	if _, ok := p.onButton("wheel up"); ok {
		t.Fatal("a second wheel notch inside the window must stay silent")
	}
	if _, ok := p.onButton("wheel right"); !ok {
		t.Fatal("the first sideways notch must show")
	}
	if _, ok := p.onButton("wheel right"); ok {
		t.Fatal("the 70ms sideways rate limit is gone")
	}
}

// A pipeline owns its state and its event buffer: two pipelines must never
// share a grouper, and a full buffer must drop instead of blocking the hook.
func TestPipelineOwnsItsStateAndBuffer(t *testing.T) {
	p := New(Options{GroupWindow: 500 * time.Millisecond, Lanes: 2})
	if p.readKeyText == nil {
		t.Fatal("New must install layoutKeyText as the default translator")
	}
	p.events = make(chan Event, 512)
	if cap(p.events) != 512 {
		t.Fatalf("event buffer = %d, want 512", cap(p.events))
	}
	for i := 0; i < 600; i++ {
		p.emit(kindKey, "x")
	}
	if len(p.events) != 512 {
		t.Fatalf("emit blocked or grew: len = %d", len(p.events))
	}
	other := New(Options{GroupWindow: 500 * time.Millisecond, Lanes: 2})
	if other.grouper == p.grouper || other.pressed == nil {
		t.Fatal("two pipelines must not share state")
	}
}
