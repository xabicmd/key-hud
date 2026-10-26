// Pure sweep geometry: no DOM, no timers, no config lookup.
//
// tests/app.test.mjs imports this file from Node and checks every function
// directly, which is why nothing here may touch the document.

const FADE_IN_PX = 90; // distance over which a label fades in
const FADE_OUT_AT = 0.9; // fraction of the viewport width where fading starts

// nextStartX places a new label far enough right that it can never land on
// top of the label already travelling on the same lane. Every label moves at
// the same speed and no label ever changes speed, so once this clear space
// exists it is preserved for the whole trip.
export function nextStartX(vw, previousRight, gapPx) {
  if (!Number.isFinite(previousRight)) return vw;
  return Math.max(vw, previousRight + gapPx);
}

// laneWaitMs is how long the trailing edge at `right` still needs to travel
// before a new label on that lane can start exactly at the right border.
// Zero means the lane is already clear.
export function laneWaitMs(right, gapPx, vw, speedPxPerSec) {
  if (!Number.isFinite(right)) return 0;
  return ((right + gapPx - vw) / speedPxPerSec) * 1000;
}

// labelKeyframes builds one constant-speed sweep. The distance is measured in
// pixels, from startX to travelEndPx past the left edge, and never depends on
// the label width, so a short label and a long one cannot drift apart.
export function labelKeyframes(opts) {
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
export function rightEdge(startX, elapsedMs, speedPxPerSec, widthPx) {
  return startX - (speedPxPerSec * elapsedMs) / 1000 + widthPx;
}
