import { laneWaitMs, labelKeyframes, nextStartX } from "./motion.js";
import { styleFor } from "./config.js";

// Clear space kept between two labels travelling on the same lane.
const LANE_GAP_EM = 0.6;

// createStage owns everything that touches the document: the label elements,
// the per-lane anchors and the single release queue. Geometry comes from
// motion.js, colours come from config.js, and nothing here reads the network.
export function createStage(root) {
  let cfg = null;
  let fontPx = 16;
  let laneTops = [];
  let laneAnchors = []; // per lane: the last label released there
  let queue = []; // labels waiting for their turn to enter, in creation order
  let pumpTimer = null;
  let current = null;
  let currentGroup = "letters";
  let currentLane = 0;

  // layout measures the strip from the config: one font size for every label
  // and the vertical centre of each lane.
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

    const style = styleFor(cfg.colors, group);
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

    root.appendChild(el);

    current = el;
    currentGroup = group;
    currentLane = lane;

    queue.push({ el, slot });
    pump();
  }

  // handle applies one instruction coming from Go.
  function handle(ev) {
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
  }

  return {
    configure(next) {
      cfg = next;
    },
    layout,
    handle,
  };
}
