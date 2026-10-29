import test from "node:test";
import assert from "node:assert/strict";

// Boots the page in Node with a minimal document stub. Splitting app.js into
// modules moved the risk from "the maths is wrong" (covered by app.test.mjs) to
// "the module graph or the wiring is wrong", which only shows up at runtime in
// the browser. This test closes that gap.

function makeElement() {
  return {
    className: "",
    style: {},
    textContent: "",
    isConnected: true,
    appended: [],
    frames: null,
    opts: null,
    appendChild(child) {
      this.appended.push(child);
      child.parent = this;
    },
    remove() {
      this.isConnected = false;
    },
    animate(frames, opts) {
      this.frames = frames;
      this.opts = opts;
      return { onfinish: null };
    },
    getBoundingClientRect() {
      return { right: 500 };
    },
  };
}

const nextMicrotasks = () => new Promise((resolve) => setTimeout(resolve, 0));

test("the page boots, applies /api/config and renders instructions", async () => {
  const stage = makeElement();
  const htmlStyle = {
    setProperty(name, value) {
      this[name] = value;
    },
  };

  globalThis.document = {
    getElementById: (id) => (id === "stage" ? stage : makeElement()),
    createElement: () => makeElement(),
    documentElement: { style: htmlStyle },
  };
  globalThis.window = { innerWidth: 1920, innerHeight: 480, hudReady: null };
  globalThis.fetch = async () => ({
    ok: true,
    json: async () => ({
      speedPxPerSec: 320,
      lanes: 2,
      colors: {
        combo: { text: "#ffffff", glow: "#00ff00", glowFar: "#003300" },
      },
    }),
  });

  await import("../internal/stage/web/app.js");
  await nextMicrotasks();

  assert.equal(typeof window.__hud, "function", "window.__hud was not installed");
  assert.equal(htmlStyle["--font-size"], "82px", "layout did not run");

  window.__hud({ kind: "new", text: "ctrl+c", group: "combo", lane: 0 });

  const [combo] = stage.appended;
  assert.ok(combo, "no label was appended");
  assert.equal(combo.textContent, "ctrl+c");
  assert.equal(combo.style.color, "#ffffff", "/api/config colours not applied");
  assert.match(combo.style.transform, /translateX\(1920px\)/);
  assert.equal(combo.opts.easing, "linear");
  assert.ok(combo.frames.length >= 3, "the sweep has too few keyframes");

  window.__hud({ kind: "append", text: "+" });
  assert.equal(combo.textContent, "ctrl+c+", "append did not extend the label");

  window.__hud({ kind: "new", text: "a", group: "letters", lane: 1 });
  const letter = stage.appended[1];
  assert.ok(letter, "the second label was not appended");
  assert.notEqual(letter.style.top, combo.style.top, "two lanes share one top");

  window.__hud({ kind: "space", text: " " });
  assert.equal(letter.textContent, "a ", "a bare space must extend its own label");
  assert.equal(stage.appended.length, 2, "a bare space must not create a label");
});
