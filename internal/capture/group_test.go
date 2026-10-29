package capture

import (
	"testing"
	"time"

	"keyhud/internal/config"
)

func TestGrouperFastTypingFormsOneLabel(t *testing.T) {
	g := newGrouper(500*time.Millisecond, 3)
	t0 := time.Unix(0, 0)

	first := g.feed(t0, "key", "h")
	if first.Kind != "new" || first.Lane != 0 || first.Group != config.GroupLetters {
		t.Fatalf("first input = %+v", first)
	}

	for i, ch := range []string{"o", "l", "a"} {
		at := t0.Add(time.Duration(i+1) * 100 * time.Millisecond)
		ev := g.feed(at, "key", ch)
		if ev.Kind != "append" || ev.Text != ch {
			t.Fatalf("character %d = %+v, want append %q", i, ev, ch)
		}
	}
}

func TestGrouperPauseStartsNewLabelOnNextLane(t *testing.T) {
	g := newGrouper(500*time.Millisecond, 3)
	t0 := time.Unix(0, 0)

	g.feed(t0, "key", "h")

	// Exactly at the window still joins; just past it does not.
	if ev := g.feed(t0.Add(500*time.Millisecond), "key", "x"); ev.Kind != "append" {
		t.Fatalf("at the window boundary: %+v, want append", ev)
	}

	ev := g.feed(t0.Add(1200*time.Millisecond), "key", "m")
	if ev.Kind != "new" || ev.Lane != 1 || ev.Text != "m" {
		t.Fatalf("after the pause: %+v", ev)
	}
}

func TestGrouperComboIsAtomic(t *testing.T) {
	g := newGrouper(500*time.Millisecond, 3)
	t0 := time.Unix(0, 0)

	combo := g.feed(t0, kindCombo, "ctrl+c")
	if combo.Kind != "new" || combo.Group != config.GroupCombo || combo.Lane != 0 {
		t.Fatalf("combo = %+v", combo)
	}

	// A plain key right after a combo must not be swallowed by it.
	ev := g.feed(t0.Add(100*time.Millisecond), "key", "d")
	if ev.Kind != "new" || ev.Lane != 1 || ev.Group != config.GroupLetters {
		t.Fatalf("key after combo = %+v, want a new label on lane 1", ev)
	}
}

func TestGrouperMouseIsAtomic(t *testing.T) {
	g := newGrouper(500*time.Millisecond, 3)
	t0 := time.Unix(0, 0)

	click := g.feed(t0, "mouse", "click")
	if click.Kind != "new" || click.Group != config.GroupMouse || click.Lane != 0 {
		t.Fatalf("click = %+v", click)
	}

	ev := g.feed(t0.Add(80*time.Millisecond), "key", "a")
	if ev.Kind != "new" || ev.Lane != 1 {
		t.Fatalf("key after click = %+v, want a new label on lane 1", ev)
	}
}

func TestGrouperActionKeyIsAtomic(t *testing.T) {
	g := newGrouper(500*time.Millisecond, 3)
	t0 := time.Unix(0, 0)

	g.feed(t0, kindKey, "h")

	// Backspace arrives inside the grouping window but must still end the word
	// and get a lane of its own.
	back := g.feed(t0.Add(100*time.Millisecond), kindAction, "backspace")
	if back.Kind != "new" || back.Lane != 1 || back.Group != config.GroupSymbols {
		t.Fatalf("backspace = %+v, want its own label on lane 1", back)
	}

	next := g.feed(t0.Add(200*time.Millisecond), kindKey, "x")
	if next.Kind != "new" || next.Lane != 2 {
		t.Fatalf("key after backspace = %+v, want a new label on lane 2", next)
	}
}

func TestGrouperBareSpaceEndsTheWordWithoutALane(t *testing.T) {
	g := newGrouper(500*time.Millisecond, 3)
	t0 := time.Unix(0, 0)

	if ev := g.feed(t0, kindKey, "h"); ev.Kind != "new" || ev.Lane != 0 {
		t.Fatalf("first letter = %+v", ev)
	}

	space := g.feed(t0.Add(100*time.Millisecond), kindBreak, " ")
	if space.Kind != "space" || space.Text != " " {
		t.Fatalf("space = %+v, want a blank appended to the current label", space)
	}
	// No lane was allocated: the event carries no lane and the cursor is intact.

	next := g.feed(t0.Add(200*time.Millisecond), kindKey, "x")
	if next.Kind != "new" || next.Lane != 1 {
		t.Fatalf("key after space = %+v, want a new label on lane 1", next)
	}
}

// The exact sequence from the bug report: words must land on 1, 2, 3, 1 even
// though a space is typed between every pair.
func TestGrouperWordsRotateLanesAcrossSpaces(t *testing.T) {
	g := newGrouper(500*time.Millisecond, 3)
	t0 := time.Unix(0, 0)

	words := []string{"te", "comparto", "un", "ejemplo"}
	want := []int{0, 1, 2, 0}
	at := t0

	for i, word := range words {
		at = at.Add(150 * time.Millisecond)
		lane := -1
		for _, r := range word {
			ev := g.feed(at, kindKey, string(r))
			at = at.Add(60 * time.Millisecond)
			if ev.Kind == "new" {
				lane = ev.Lane
			}
		}
		if lane < 0 {
			t.Fatalf("word %q never started a label", word)
		}
		if lane != want[i] {
			t.Fatalf("word %q landed on lane %d, want %d", word, lane, want[i])
		}
		// The space between words must not consume the next lane.
		g.feed(at, kindBreak, " ")
		at = at.Add(120 * time.Millisecond)
	}
}

func TestGrouperLaneRotation(t *testing.T) {
	g := newGrouper(500*time.Millisecond, 3)
	t0 := time.Unix(0, 0)

	want := []int{0, 1, 2, 0, 1}
	for i, w := range want {
		ev := g.feed(t0.Add(time.Duration(i)*time.Second), "key", "a")
		if ev.Kind != "new" {
			t.Fatalf("group %d = %+v, want new", i, ev)
		}
		if ev.Lane != w {
			t.Fatalf("group %d landed on lane %d, want %d", i, ev.Lane, w)
		}
	}
}

func TestGrouperSplitsOverlongLabel(t *testing.T) {
	g := newGrouper(500*time.Millisecond, 3)
	t0 := time.Unix(0, 0)

	for i := 0; i < maxLabelRunes; i++ {
		at := t0.Add(time.Duration(i) * 50 * time.Millisecond)
		want := "append"
		if i == 0 {
			want = "new"
		}
		if ev := g.feed(at, "key", "a"); ev.Kind != want {
			t.Fatalf("run %d = %+v, want %s", i, ev, want)
		}
	}

	at := t0.Add(time.Duration(maxLabelRunes+1) * 50 * time.Millisecond)
	ev := g.feed(at, "key", "a")
	if ev.Kind != "new" {
		t.Fatalf("run past the cap = %+v, want a new label", ev)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		kind, label, want string
	}{
		{"key", "a", config.GroupLetters},
		{"key", "z", config.GroupLetters},
		{"key", "1", config.GroupSymbols},
		{"key", ";", config.GroupSymbols},
		{"key", "space", config.GroupSymbols},
		{"key", "enter", config.GroupSymbols},
		{"key", "f24", config.GroupSymbols},
		{"action", "backspace", config.GroupSymbols},
		{"action", "tab", config.GroupSymbols},
		{"combo", "ctrl+c", config.GroupCombo},
		{"combo", "ctrl+shift+alt+win+p", config.GroupCombo},
		// A typed "+" is text, not a combination.
		{"key", "+", config.GroupSymbols},
		{"mouse", "click", config.GroupMouse},
		{"mouse", "wheel up", config.GroupMouse},
		// A modified mouse event stays a mouse event.
		{"mouse", "ctrl+click", config.GroupMouse},
	}

	for _, c := range cases {
		if got := classify(c.kind, c.label); got != c.want {
			t.Errorf("classify(%q, %q) = %q, want %q", c.kind, c.label, got, c.want)
		}
	}
}

func TestClassifyHandlesMultibyte(t *testing.T) {
	// A single non-ASCII letter must not be mistaken for a multi-character key.
	if got := classify("key", "ñ"); got != config.GroupLetters {
		t.Fatalf("classify(ñ) = %q, want %q", got, config.GroupLetters)
	}
}

// A typed symbol joins the word even when it is the plus sign: it is text, not
// a combination of keys.
func TestGrouperTypedPlusIsText(t *testing.T) {
	g := newGrouper(500*time.Millisecond, 3)
	t0 := time.Unix(0, 0)

	if ev := g.feed(t0, kindKey, "c"); ev.Kind != "new" {
		t.Fatalf("first letter = %+v", ev)
	}
	ev := g.feed(t0.Add(80*time.Millisecond), kindKey, "+")
	if ev.Kind != "append" || ev.Text != "+" {
		t.Fatalf("typed plus = %+v, want it appended to the word", ev)
	}
}

func TestGrouperUsesConfiguredWindow(t *testing.T) {
	// The 500 ms rule must come from the config, not from a hardcoded constant.
	g := newGrouper(250*time.Millisecond, 2)
	t0 := time.Unix(0, 0)

	g.feed(t0, "key", "a")
	if ev := g.feed(t0.Add(200*time.Millisecond), "key", "b"); ev.Kind != "append" {
		t.Fatalf("inside a 250 ms window: %+v, want append", ev)
	}
	if ev := g.feed(t0.Add(1000*time.Millisecond), "key", "c"); ev.Kind != "new" {
		t.Fatalf("past a 250 ms window: %+v, want a new label", ev)
	}
}
