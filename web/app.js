(() => {
  "use strict";

  const params = new URLSearchParams(location.search);
  const stage = document.getElementById("stage");

  const durParam = parseFloat(params.get("dur"));
  const travel = Number.isFinite(durParam) && durParam > 0 ? durParam : 7;

  // Snap the glyph size to whole pixels. A bitmap-designed font renders crisp
  // only at integer sizes; sub-pixel sizes give you a smeared, non-pixel look.
  const fontPx = Math.max(8, Math.round(window.innerHeight * 0.17));
  document.documentElement.style.setProperty("--font-size", fontPx + "px");

  const laneHeight = fontPx * 1.9;
  const lanesParam = parseInt(params.get("lanes"), 10);
  const lanes = Number.isFinite(lanesParam) && lanesParam > 0
    ? lanesParam
    : Math.max(1, Math.floor(window.innerHeight / laneHeight));

  const slack = Math.max(0, window.innerHeight - lanes * laneHeight);
  const laneTops = [];
  for (let i = 0; i < lanes; i++) {
    laneTops.push(slack / 2 + i * laneHeight + (laneHeight - fontPx) / 2);
  }

  // Give each new label the lane that has been idle longest. Cheap, and it
  // keeps fast typing from stacking labels on top of each other.
  const lastSpawn = new Array(lanes).fill(-Infinity);
  const MAX_ITEMS = 48;

  function pickLane() {
    let best = 0;
    for (let i = 1; i < lanes; i++) {
      if (lastSpawn[i] < lastSpawn[best]) best = i;
    }
    lastSpawn[best] = performance.now();
    return best;
  }

  window.__hud = (ev) => {
    if (!ev || !ev.label) return;

    const el = document.createElement("div");
    el.className = ev.kind === "mouse" ? "key mouse" : "key";
    el.textContent = ev.label;
    el.style.top = laneTops[pickLane()] + "px";
    el.style.animationDuration = travel + "s";
    el.addEventListener("animationend", () => el.remove(), { once: true });

    stage.appendChild(el);
    while (stage.childElementCount > MAX_ITEMS) {
      stage.firstElementChild.remove();
    }
  };

  // Tell Go the page is alive so it can install the global hooks without
  // dropping the first keystrokes.
  try {
    const ready = window.hudReady;
    if (typeof ready === "function") {
      const p = ready();
      if (p && typeof p.catch === "function") p.catch(() => {});
    }
  } catch (_) {
    /* the Go side falls back to a timer */
  }
})();
