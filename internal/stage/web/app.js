import { FALLBACK, mergeConfig } from "./config.js";
import { createStage } from "./stage.js";
import { createSplash } from "./splash.js";

// Entry point: load the config, measure the stages, and expose the single
// function Go calls for every captured input.

const stage = createStage(document.getElementById("stage"));
const splash = createSplash(document.getElementById("splash"));

let cfg = FALLBACK;
// Configure before the round-trip, not after: an instruction that arrives
// early must still find colours instead of throwing.
stage.configure(cfg);
splash.configure(cfg);

// The page announces itself only after the config arrived and the lanes are
// measured, so Go can install the hooks without losing a keystroke.
function announceReady() {
  if (typeof window.hudReady !== "function") return;
  try {
    const p = window.hudReady();
    if (p && typeof p.catch === "function") p.catch(() => {});
  } catch (_) {
    /* the Go side falls back to a timer */
  }
}

// Defined immediately, before the config round-trip: a keystroke that arrives
// early is dropped by the page, exactly as it was before.
window.__hud = (ev) => stage.handle(ev);

fetch("api/config", { cache: "no-store" })
  .then((r) => (r.ok ? r.json() : null))
  .then((c) => {
    cfg = mergeConfig(FALLBACK, c);
  })
  .catch(() => {
    /* keep the built-in defaults */
  })
  .finally(() => {
    stage.configure(cfg);
    splash.configure(cfg);
    stage.layout();
    announceReady();
  });
