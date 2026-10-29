import test from "node:test";
import assert from "node:assert/strict";

import { createSplash } from "../internal/stage/web/splash.js";

// The intro is the first thing the window shows. These tests pin the three
// lines and the two calls Go uses to drop them; the keyboard side lives in
// splash_test.go.

function makeElement() {
  return {
    className: "",
    style: {},
    textContent: "",
    isConnected: true,
    children: [],
    appendChild(child) {
      this.children.push(child);
      child.parent = this;
    },
    remove() {
      this.isConnected = false;
    },
  };
}

const LINES = ["key-hud by xabicmd", "start (enter)", "to close (ctrl+alt+q)"];

// The module only needs to create elements: nothing here reads the real DOM.
globalThis.document = { createElement: () => makeElement() };

test("the intro fills the layer with the three lines in order", () => {
  globalThis.window = { innerWidth: 1920, innerHeight: 480 };
  const layer = makeElement();

  createSplash(layer);

  assert.deepEqual(
    layer.children.map((line) => line.textContent),
    LINES
  );
  assert.ok(
    layer.children.every((line) => line.className === "splash-line"),
    "every line needs the splash-line class"
  );
});

test("__hudStart drops the intro", () => {
  globalThis.window = { innerWidth: 1920, innerHeight: 480 };
  const layer = makeElement();
  createSplash(layer);

  assert.equal(typeof window.__hudStart, "function", "__hudStart was not installed");
  window.__hudStart();

  assert.equal(layer.isConnected, false, "the intro stayed on screen");
});

test("each line wears its palette: symbol yellow title, letter start, mouse pink exit", () => {
  globalThis.window = { innerWidth: 1920, innerHeight: 480 };
  const layer = makeElement();
  const splash = createSplash(layer);

  splash.configure({
    colors: {
      letters: { text: "#letters", glow: "#letters-glow", glowFar: "#letters-far" },
      symbols: { text: "#symbols", glow: "#symbols-glow", glowFar: "#symbols-far" },
      combo: { text: "#combo", glow: "#combo-glow", glowFar: "#combo-far" },
      mouse: { text: "#mouse", glow: "#mouse-glow", glowFar: "#mouse-far" },
    },
  });

  const [title, start, close] = layer.children;
  assert.equal(title.style.color, "#symbols", "the title must use the symbol yellow");
  assert.equal(start.style.color, "#letters", "start must stay in the letter colour");
  assert.equal(close.style.color, "#mouse", "to close must use the mouse pink");
});

test("a start that happened before the page loaded is recovered", async () => {
  globalThis.window = {
    innerWidth: 1920,
    innerHeight: 480,
    hudIntro: async () => false,
  };
  const layer = makeElement();
  createSplash(layer);

  await new Promise((resolve) => setTimeout(resolve, 0));

  assert.equal(
    layer.isConnected,
    false,
    "the intro survived a start that already happened"
  );
});
