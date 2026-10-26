import test from "node:test";
import assert from "node:assert/strict";

import {
  nextStartX,
  laneWaitMs,
  labelKeyframes,
  rightEdge,
} from "../web/motion.js";

const VW = 1920;
const SPEED = 320; // key-hud.json default
const TRAVEL_END = 700; // key-hud.json default

const xOf = (frame) => {
  const m = /^translateX\((-?[\d.]+)px\)$/.exec(frame.transform);
  assert.ok(m, `unexpected transform: ${frame.transform}`);
  return Number(m[1]);
};

const speedOf = ({ frames, duration }) => {
  const distance = xOf(frames[0]) - xOf(frames[frames.length - 1]);
  return distance / (duration / 1000);
};

test("a label enters at the screen edge when its lane is clear", () => {
  assert.equal(nextStartX(VW, -Infinity, 48), VW);
});

test("a label whose lane is clear again enters at the screen edge", () => {
  // Previous label already fully inside the viewport: its right edge is 500px.
  assert.equal(nextStartX(VW, 500, 48), VW);
});

test("a label waits behind the one already on the lane", () => {
  const previousRight = rightEdge(VW, 100, SPEED, 800); // 1920 - 32 + 800
  assert.equal(previousRight, 2688);
  assert.equal(nextStartX(VW, previousRight, 48), 2736);
});

test("a clear lane needs no wait at all", () => {
  assert.equal(laneWaitMs(-Infinity, 48, VW, SPEED), 0);
  // Right edge exactly one gap short of the border: already clear.
  assert.equal(laneWaitMs(VW - 48, 48, VW, SPEED), 0);
});

test("the wait is exactly the time the trailing edge needs to clear", () => {
  const gap = 48;
  const width = 800;
  const previousRight = rightEdge(VW, 0, SPEED, width); // 2720

  const wait = laneWaitMs(previousRight, gap, VW, SPEED);
  assert.equal(wait, 2650);

  // After that wait the lane is clear enough for a label to start at the edge,
  // so the FIFO queue releases exactly when the previous label is out of the way.
  const clearedRight = previousRight - (SPEED * wait) / 1000;
  assert.equal(nextStartX(VW, clearedRight, gap), VW);
});

test("a lane whose label just appeared makes the next one wait", () => {
  // A label that has this instant started at the edge still sticks out by its
  // own width, so the next one on that lane cannot start yet.
  const waited = laneWaitMs(VW + 800, 48, VW, SPEED);
  assert.equal(waited, (848 / SPEED) * 1000);
  assert.ok(waited > 0);
});

test("two labels on one lane never overlap and keep the same gap", () => {
  const gap = 48;
  const width = 800;
  const firstStart = VW;
  const spawnAt = 100; // ms after the first label started

  const secondStart = nextStartX(
    VW,
    rightEdge(firstStart, spawnAt, SPEED, width),
    gap
  );

  for (const t of [spawnAt, 500, 2000, 6000]) {
    const firstRight = rightEdge(firstStart, t, SPEED, width);
    const secondLeft = secondStart - (SPEED * (t - spawnAt)) / 1000;
    assert.equal(
      secondLeft - firstRight,
      gap,
      `labels must stay ${gap}px apart at t=${t}ms`
    );
  }
});

test("a queued label travels at exactly the same speed", () => {
  const first = labelKeyframes({
    startX: VW,
    vw: VW,
    speedPxPerSec: SPEED,
    travelEndPx: TRAVEL_END,
  });
  const queued = labelKeyframes({
    startX: 5555,
    vw: VW,
    speedPxPerSec: SPEED,
    travelEndPx: TRAVEL_END,
  });

  assert.equal(speedOf(first), SPEED);
  assert.equal(speedOf(queued), SPEED);
});

test("frames advance in time and move left for a queued label", () => {
  const { frames } = labelKeyframes({
    startX: 4000,
    vw: VW,
    speedPxPerSec: SPEED,
    travelEndPx: TRAVEL_END,
  });

  assert.equal(frames[0].offset, 0);
  assert.equal(frames[frames.length - 1].offset, 1);
  assert.equal(frames[0].opacity, 0);
  assert.equal(frames[frames.length - 1].opacity, 0);
  assert.ok(frames.some((f) => f.opacity === 1), "label must become visible");

  for (let i = 1; i < frames.length; i++) {
    assert.ok(
      frames[i].offset > frames[i - 1].offset,
      `frame ${i} must come after frame ${i - 1}`
    );
    assert.ok(
      xOf(frames[i]) < xOf(frames[i - 1]),
      `frame ${i} must sit left of frame ${i - 1}`
    );
  }

  assert.equal(xOf(frames[frames.length - 1]), -TRAVEL_END);
});

test("an unqueued label keeps the original fade-in and fade-out geometry", () => {
  const { frames, duration } = labelKeyframes({
    startX: VW,
    vw: VW,
    speedPxPerSec: SPEED,
    travelEndPx: TRAVEL_END,
  });

  const distance = VW + TRAVEL_END;
  assert.equal(duration, (distance / SPEED) * 1000);
  assert.deepEqual(
    frames.map((f) => f.offset),
    [
      0,
      (VW - (VW - 90)) / distance,
      (VW - VW * (1 - 0.9)) / distance,
      1,
    ]
  );
  assert.equal(xOf(frames[1]), VW - 90);
  assert.ok(Math.abs(xOf(frames[2]) - VW * 0.1) < 1e-9);
});
