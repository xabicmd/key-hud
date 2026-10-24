(() => {
  "use strict";

  const FADE_IN_PX = 90; // distance over which a label fades in
  const FADE_OUT_AT = 0.9; // fraction of the viewport width where fading starts
  const LANE_GAP_EM = 0.6; // clear space kept between two labels on one lane

  // ------------------------------------------------------------------ motion
  //
  // Pure geometry: no DOM, no timers. tests/app.test.js requires this file
  // from Node and checks these functions directly, which is why the browser
  // bootstrap below only runs when `module` does not exist.

  // nextStartX places a new label far enough right that it can never land on
  // top of the label already travelling on the same lane. Every label moves at
  // the same speed and no label ever changes speed, so once this clear space
  // exists it is preserved for the whole trip.
  function nextStartX(vw, previousRight, gapPx) {
    if (!Number.isFinite(previousRight)) return vw;
    return Math.max(vw, previousRight + gapPx);
  }

  // laneWaitMs is how long the trailing edge at `right` still needs to travel
  // before a new label on that lane can start exactly at the right border.
  // Zero means the lane is already clear.
  function laneWaitMs(right, gapPx, vw, speedPxPerSec) {
    if (!Number.isFinite(right)) return 0;
    return ((right + gapPx - vw) / speedPxPerSec) * 1000;
  }

  // labelKeyframes builds one constant-speed sweep. The distance is measured in
  // pixels, from startX to travelEndPx past the left edge, and never depends on
  // the label width, so a short label and a long one cannot drift apart.
  function labelKeyframes(opts) {
    const { startX, vw, speedPxPerSec, travelEndPx } = opts;
    const endX = -travelEndPx;
    const distance = startX - endX;
    const at = (x) => (startX - x) / distance;

    const frames = [
      { transform: `translateX(${startX}px)`, opacity: 0, offset: 0 },
    ];

    // A queued label stays invisible while it waits beyond the right edge.
    const enter = at(vw);
    if (enter > 0) {
      frames.push({
        transform: `translateX(${vw}px)`,
        opacity: 0,
        offset: enter,
      });
    }

    frames.push(
      {
        transform: `translateX(${vw - FADE_IN_PX}px)`,
        opacity: 1,
        offset: at(vw - FADE_IN_PX),
      },
      {
        transform: `translateX(${vw * (1 - FADE_OUT_AT)}px)`,
        opacity: 1,
        offset: at(vw * (1 - FADE_OUT_AT)),
      },
      {
        transform: `translateX(${endX}px)`,
        opacity: 0,
        offset: 1,
      }
    );

    return { frames, duration: (distance / speedPxPerSec) * 1000 };
  }

  // rightEdge is where the trailing edge of a label sits after elapsedMs of
  // travel, given its start position and rendered width.
  function rightEdge(startX, elapsedMs, speedPxPerSec, widthPx) {
    return startX - (speedPxPerSec * elapsedMs) / 1000 + widthPx;
  }

  if (typeof module === "object" && module.exports) {
    module.exports = { nextStartX, laneWaitMs, labelKeyframes, rightEdge };
    return;
  }

  // ------------------------------------------------------------- browser only

  const stage = document.getElementById("stage");

  const FALLBACK = {
    speedPxPerSec: 320,
    groupWindowMs: 500,
    lanes: 3,
    fontSizeVh: 17,
    travelEndPx: 700,
    colors: {
      letters: { text: "#eafcff", glow: "#35d6ff", glowFar: "#0a5f7a" },
      symbols: { text: "#ffe9b8", glow: "#ffb703", glowFar: "#7a4d00" },
      combo: { text: "#e6d4ff", glow: "#a06bff", glowFar: "#43148f" },
      mouse: { text: "#ffd9f6", glow: "#ff3fd0", glowFar: "#6d0a58" },
    },
  };

  let cfg = FALLBACK;
  let fontPx = 16;
  let laneTops = [];
  let laneAnchors = []; // per lane: the last label released there
  let queue = []; // labels waiting for their turn to enter, in creation order
  let pumpTimer = null;
  let current = null;
  let currentGroup = "letters";
  let currentLane = 0;

  function merge(base, user) {
    if (!user || typeof user !== "object") return base;
    return {
      ...base,
      ...user,
      colors: { ...base.colors, ...(user.colors || {}) },
    };
  }

  function layout() {
    fontPx = Math.max(
      8,
      Math.round((window.innerHeight * cfg.fontSizeVh) / 100)
    );
    document.documentElement.style.setProperty("--font-size", fontPx + "px");

    const laneHeight = fontPx * 1.9;
    const count = Math.max(1, Math.floor(cfg.lanes) || 1);
    const slack = Math.max(0, window.innerHeight - count * laneHeight);

    laneTops = [];
    for (let i = 0; i < count; i++) {
      laneTops.push(slack / 2 + i * laneHeight + (laneHeight - fontPx) / 2);
    }

    laneAnchors = [];
  }

  function styleFor(group) {
    const fallback = FALLBACK.colors.letters;
    const c =
      (cfg.colors && (cfg.colors[group] ||
        cfg.colors.letters ||
        cfg.colors[Object.keys(cfg.colors)[0]])) ||
      fallback;

    return {
      color: c.text,
      shadow: `0 0 0.06em ${c.text}, 0 0 0.22em ${c.glow}, 0 0 0.55em ${c.glow}, 0 0 1.10em ${c.glowFar}`,
    };
  }

  function animate(el, startX) {
    const { frames, duration } = labelKeyframes({
      startX,
      vw: window.innerWidth,
      speedPxPerSec: cfg.speedPxPerSec,
      travelEndPx: cfg.travelEndPx,
    });

    const anim = el.animate(frames, {
      duration,
      easing: "linear",
      fill: "forwards",
    });

    anim.onfinish = () => {
      el.remove();
      if (current === el) current = null;
    };
  }

  // laneRightEdge is the trailing edge of the label already on this lane, or
  // -Infinity when the lane is clear and the next label can enter immediately.
  function laneRightEdge(slot) {
    const previous = laneAnchors[slot];
    if (!previous || !previous.isConnected) return -Infinity;
    return previous.getBoundingClientRect().right;
  }

  // pump releases queued labels strictly in creation order.
  //
  // Waiting per lane is not enough: lane 3 could be free while lane 2 was still
  // busy, so the third label entered before the second one and the strip looked
  // like it jumped 1 -> 3 -> 2. A single global queue makes the order you typed
  // the only order that can appear on screen. A blocked head holds the whole
  // queue back; nothing is ever reordered or dropped to catch up.
  function pump() {
    if (pumpTimer !== null) {
      clearTimeout(pumpTimer);
      pumpTimer = null;
    }

    const vw = window.innerWidth;
    const gapPx = fontPx * LANE_GAP_EM;

    while (queue.length) {
      const head = queue[0];
      if (!head.el.isConnected) {
        // The label left the screen before it ever got a turn (it cannot be
        // seen while queued, but a resize or reload may remove it).
        queue.shift();
        continue;
      }

      const right = laneRightEdge(head.slot);
      const startX = nextStartX(vw, right, gapPx);
      if (startX > vw + 0.01) {
        // The lane still has a trailing edge on screen. Wait exactly as long as
        // that edge needs to clear the right border, then try again.
        const waitMs = laneWaitMs(right, gapPx, vw, cfg.speedPxPerSec);
        pumpTimer = setTimeout(pump, Math.max(8, Math.ceil(waitMs)));
        return;
      }

      queue.shift();
      animate(head.el, startX);
      laneAnchors[head.slot] = head.el;
    }
  }

  function createLabel(text, group, lane) {
    const el = document.createElement("div");
    el.className = "key";

    const style = styleFor(group);
    el.style.color = style.color;
    el.style.textShadow = style.shadow;

    const slot = lane % laneTops.length;
    el.style.top = (laneTops[slot] || 0) + "px";
    el.textContent = text;

    // While it waits for its turn the label must be invisible and parked
    // outside the right edge. The animation that starts on release overrides
    // both properties.
    el.style.transform = `translateX(${window.innerWidth}px)`;
    el.style.opacity = "0";

    stage.appendChild(el);

    current = el;
    currentGroup = group;
    currentLane = lane;

    queue.push({ el, slot });
    pump();
  }

  window.__hud = (ev) => {
    if (!ev || !ev.text) return;

    if (ev.kind === "space") {
      // The blank closes the word it belongs to and extends that same label.
      // It must not create a label or take a lane: being invisible, it would
      // look like an empty line and make the words skip a row.
      if (current && current.isConnected) current.textContent += ev.text;
      return;
    }

    if (ev.kind === "append") {
      if (current && current.isConnected) {
        current.textContent += ev.text;
      } else {
        // The label finished travelling before this character landed. Start a
        // replacement with the same look so the text is never silently lost.
        createLabel(ev.text, currentGroup, currentLane);
      }
      return;
    }

    createLabel(ev.text, ev.group || "letters", ev.lane || 0);
  };

  function announceReady() {
    if (typeof window.hudReady !== "function") return;
    try {
      const p = window.hudReady();
      if (p && typeof p.catch === "function") p.catch(() => {});
    } catch (_) {
      /* the Go side falls back to a timer */
    }
  }

  fetch("api/config", { cache: "no-store" })
    .then((r) => (r.ok ? r.json() : null))
    .then((c) => {
      cfg = merge(FALLBACK, c);
    })
    .catch(() => {
      /* keep the built-in defaults */
    })
    .finally(() => {
      layout();
      announceReady();
    });
})();
