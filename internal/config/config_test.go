package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigIsComplete(t *testing.T) {
	cfg := Default()

	if cfg.SpeedPxPerSec <= 0 {
		t.Fatalf("speedPxPerSec = %v, want > 0", cfg.SpeedPxPerSec)
	}
	if cfg.GroupWindowMs != 500 {
		t.Fatalf("groupWindowMs = %d, want 500", cfg.GroupWindowMs)
	}
	if cfg.Lanes < 1 {
		t.Fatalf("lanes = %d, want >= 1", cfg.Lanes)
	}

	// Every group the grouper can emit must have a colour, otherwise a label
	// would fall back to the first frame's hardcoded white.
	for _, group := range []string{GroupLetters, GroupSymbols, GroupCombo, GroupMouse} {
		style, ok := cfg.Colors[group]
		if !ok {
			t.Fatalf("missing colour group %q", group)
		}
		if style.Text == "" || style.Glow == "" || style.GlowFar == "" {
			t.Fatalf("group %q has an empty colour: %+v", group, style)
		}
	}
}

func TestMergeConfigKeepsDefaultsForUnsetFields(t *testing.T) {
	base := Default()
	user := Config{SpeedPxPerSec: 900}

	got := Merge(base, user)

	if got.SpeedPxPerSec != 900 {
		t.Fatalf("speedPxPerSec = %v, want 900", got.SpeedPxPerSec)
	}
	// Everything the user did not mention must survive untouched.
	if got.GroupWindowMs != base.GroupWindowMs {
		t.Fatalf("groupWindowMs = %d, want %d", got.GroupWindowMs, base.GroupWindowMs)
	}
	if got.Lanes != base.Lanes {
		t.Fatalf("lanes = %d, want %d", got.Lanes, base.Lanes)
	}
	if got.Colors[GroupMouse] != base.Colors[GroupMouse] {
		t.Fatalf("mouse colour = %+v, want %+v", got.Colors[GroupMouse], base.Colors[GroupMouse])
	}
}

func TestMergeConfigOverridesOnlyGivenColourFields(t *testing.T) {
	base := Default()
	user := Config{Colors: map[string]ColorStyle{
		GroupLetters: {Glow: "#ff0000"},
	}}

	got := Merge(base, user)

	if got.Colors[GroupLetters].Glow != "#ff0000" {
		t.Fatalf("glow = %q, want #ff0000", got.Colors[GroupLetters].Glow)
	}
	// A partial style must not blank out the fields it left alone.
	if got.Colors[GroupLetters].Text != base.Colors[GroupLetters].Text {
		t.Fatalf("text = %q, want the default %q", got.Colors[GroupLetters].Text, base.Colors[GroupLetters].Text)
	}
	if got.Colors[GroupLetters].GlowFar != base.Colors[GroupLetters].GlowFar {
		t.Fatalf("glowFar = %q, want the default %q", got.Colors[GroupLetters].GlowFar, base.Colors[GroupLetters].GlowFar)
	}
}

func TestMergeConfigIgnoresUnknownGroup(t *testing.T) {
	base := Default()
	user := Config{Colors: map[string]ColorStyle{
		"nonsense": {Text: "#123456"},
	}}

	got := Merge(base, user)

	if _, ok := got.Colors["nonsense"]; ok {
		t.Fatal("an unknown colour group was invented")
	}
	if len(got.Colors) != len(base.Colors) {
		t.Fatalf("colour group count = %d, want %d", len(got.Colors), len(base.Colors))
	}
}

func TestLoadConfigWritesDefaultsWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key-hud.json")

	got := Load(path)
	if got.SpeedPxPerSec != Default().SpeedPxPerSec {
		t.Fatalf("speedPxPerSec = %v, want the default", got.SpeedPxPerSec)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("default config was not written: %v", err)
	}

	// The file on disk must be valid JSON that round-trips into a Config.
	var onDisk Config
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("written config is not valid JSON: %v", err)
	}
	if onDisk.GroupWindowMs != 500 {
		t.Fatalf("written groupWindowMs = %d, want 500", onDisk.GroupWindowMs)
	}
}

func TestLoadConfigFallsBackOnInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key-hud.json")
	if err := os.WriteFile(path, []byte("{ this is not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A typo in the config must never stop the app from starting.
	got := Load(path)
	if got.SpeedPxPerSec != Default().SpeedPxPerSec {
		t.Fatalf("speedPxPerSec = %v, want the default", got.SpeedPxPerSec)
	}
}

func TestLoadConfigReadsUserValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key-hud.json")
	body := `{
  "speedPxPerSec": 640,
  "groupWindowMs": 250,
  "lanes": 2,
  "colors": {
    "combo": { "text": "#ffffff", "glow": "#00ff00", "glowFar": "#003300" }
  }
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Load(path)

	if got.SpeedPxPerSec != 640 || got.GroupWindowMs != 250 || got.Lanes != 2 {
		t.Fatalf("scalars = %+v", got)
	}
	if got.Colors[GroupCombo].Glow != "#00ff00" {
		t.Fatalf("combo glow = %q", got.Colors[GroupCombo].Glow)
	}
	// Groups absent from the file keep their defaults.
	if got.Colors[GroupMouse] != Default().Colors[GroupMouse] {
		t.Fatalf("mouse colour = %+v", got.Colors[GroupMouse])
	}
}
