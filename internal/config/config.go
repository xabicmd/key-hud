package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// The colour groups are a contract with the page: every Event carries one of
// these names and the web side looks its colours up by that exact string. They
// live here because they are the keys of the "colors" object in the config file.
const (
	GroupLetters = "letters"
	GroupSymbols = "symbols"
	GroupCombo   = "combo"
	GroupMouse   = "mouse"
)

// ColorStyle is the text colour and its two glow layers for one kind of input.
type ColorStyle struct {
	Text    string `json:"text"`
	Glow    string `json:"glow"`
	GlowFar string `json:"glowFar"`
}

// Config is everything worth tuning without recompiling. It is written to disk
// on first run so there is always a file to edit.
type Config struct {
	SpeedPxPerSec float64               `json:"speedPxPerSec"`
	GroupWindowMs int                   `json:"groupWindowMs"`
	Lanes         int                   `json:"lanes"`
	FontSizeVh    float64               `json:"fontSizeVh"`
	TravelEndPx   int                   `json:"travelEndPx"`
	Colors        map[string]ColorStyle `json:"colors"`
}

func Default() Config {
	return Config{
		SpeedPxPerSec: 320,
		GroupWindowMs: 500,
		Lanes:         3,
		FontSizeVh:    17,
		TravelEndPx:   700,
		Colors: map[string]ColorStyle{
			GroupLetters: {Text: "#eafcff", Glow: "#35d6ff", GlowFar: "#0a5f7a"},
			GroupSymbols: {Text: "#ffe9b8", Glow: "#ffb703", GlowFar: "#7a4d00"},
			GroupCombo:   {Text: "#e6d4ff", Glow: "#a06bff", GlowFar: "#43148f"},
			GroupMouse:   {Text: "#ffd9f6", Glow: "#ff3fd0", GlowFar: "#6d0a58"},
		},
	}
}

// Path keeps the config next to the executable, so the shortcut you drop
// in shell:startup and the config file always travel together.
func Path() string {
	exe, err := os.Executable()
	if err != nil {
		return "key-hud.json"
	}
	return filepath.Join(filepath.Dir(exe), "key-hud.json")
}

func Load(path string) Config {
	cfg := Default()

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			WriteDefault(path, cfg)
		} else {
			log.Printf("config: %v (using defaults)", err)
		}
		return cfg
	}

	var user Config
	if err := json.Unmarshal(raw, &user); err != nil {
		// A typo must not stop the app dead; fall back and say so.
		log.Printf("config: %s is not valid JSON: %v (using defaults)", path, err)
		return cfg
	}

	return Merge(cfg, user)
}

func WriteDefault(path string, cfg Config) {
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		log.Printf("config: could not write %s: %v", path, err)
		return
	}
	log.Printf("config: wrote defaults to %s", path)
}

// mergeConfig applies whatever the user actually set and keeps the default for
// everything else, so deleting a key never breaks the app.
func Merge(base, user Config) Config {
	if user.SpeedPxPerSec > 0 {
		base.SpeedPxPerSec = user.SpeedPxPerSec
	}
	if user.GroupWindowMs > 0 {
		base.GroupWindowMs = user.GroupWindowMs
	}
	if user.Lanes > 0 {
		base.Lanes = user.Lanes
	}
	if user.FontSizeVh > 0 {
		base.FontSizeVh = user.FontSizeVh
	}
	if user.TravelEndPx > 0 {
		base.TravelEndPx = user.TravelEndPx
	}

	for name, style := range user.Colors {
		merged, known := base.Colors[name]
		if !known {
			continue // unknown group name: ignore rather than invent one
		}
		if style.Text != "" {
			merged.Text = style.Text
		}
		if style.Glow != "" {
			merged.Glow = style.Glow
		}
		if style.GlowFar != "" {
			merged.GlowFar = style.GlowFar
		}
		base.Colors[name] = merged
	}

	return base
}
