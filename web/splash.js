import { styleFor } from "./config.js";

// The intro screen: what the strip is, how to start it, how to leave it.
//
// Go owns the keyboard, so this module never listens for keys. It fills the
// #splash layer with the three lines and exposes the two calls Go makes around
// them: window.__hudStart drops them, and window.hudIntro answers whether they
// are still up when the page boots after the start key was already pressed.

const LINES = ["key-hud by xabicmd", "start (enter)", "to close (ctrl+alt+q)"];

export function createSplash(root) {
  const lines = LINES.map((text) => {
    const line = document.createElement("div");
    line.className = "splash-line";
    line.textContent = text;
    root.appendChild(line);
    return line;
  });

  let shown = true;

  function hide() {
    if (!shown) return;
    shown = false;
    root.remove();
  }

  window.__hudStart = hide;

  // The Enter press can land before this module runs. Ask Go once instead of
  // trusting that the push arrived after the document existed.
  if (typeof window.hudIntro === "function") {
    try {
      Promise.resolve(window.hudIntro())
        .then((pending) => {
          if (pending === false) hide();
        })
        .catch(() => {});
    } catch (_) {
      /* a failed read must not break the boot; Go still pushes __hudStart */
    }
  }

  return {
    // configure applies the strip palette: the title wears the symbol yellow,
    // the start hint the letter colour and the exit hint the mouse pink (the
    // same colour a click gets).
    configure(cfg) {
      const title = styleFor(cfg.colors, "symbols");
      const letters = styleFor(cfg.colors, "letters");
      const mouse = styleFor(cfg.colors, "mouse");

      lines[0].style.color = title.color;
      lines[0].style.textShadow = title.shadow;
      lines[1].style.color = letters.color;
      lines[1].style.textShadow = letters.shadow;
      lines[2].style.color = mouse.color;
      lines[2].style.textShadow = mouse.shadow;
    },
    hide,
  };
}
