// The values the strip falls back to when key-hud.json is missing or partial.
//
// They mirror defaultConfig() in config.go. Keep the two in sync: the Go side
// writes that file, and this object covers the window between page load and
// the /api/config response.

export const FALLBACK = {
  speedPxPerSec: 320,
  groupWindowMs: 500,
  lanes: 3,
  fontSizeVh: 17,
  travelEndPx: 700,
  colors: {
    letters: { text: "#eafcff", glow: "#35d6ff", glowFar: "#0a5f7a" },
    symbols: { text: "#ffe9b8", glow: "#ffb703", glowFar: "#7a4d00" },
    combo: { text: "#ffd6ef", glow: "#ff2d95", glowFar: "#6b0040" },
    mouse: { text: "#ffd9f6", glow: "#ff3fd0", glowFar: "#6d0a58" },
  },
};

// mergeConfig lets a partial key-hud.json override only the keys it sets.
export function mergeConfig(base, user) {
  if (!user || typeof user !== "object") return base;
  return {
    ...base,
    ...user,
    colors: { ...base.colors, ...(user.colors || {}) },
  };
}

// styleFor returns the text colour and the three glow layers for one group.
// An unknown group falls back to the letter colours instead of disappearing.
export function styleFor(colors, group) {
  const fallback = FALLBACK.colors.letters;
  const c =
    (colors &&
      (colors[group] ||
        colors.letters ||
        colors[Object.keys(colors)[0]])) ||
    fallback;

  return {
    color: c.text,
    shadow: `0 0 0.06em ${c.text}, 0 0 0.22em ${c.glow}, 0 0 0.55em ${c.glow}, 0 0 1.10em ${c.glowFar}`,
  };
}
