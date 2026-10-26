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
    // configure applies the strip palette: the title wears the symbol colour
    // (numbers, backspace), the exit hint the combination colour and the start
    // hint the letter colour.
    configure(cfg) {
      const symbols = styleFor(cfg.colors, "symbols");
      const combo = styleFor(cfg.colors, "combo");
      const letters = styleFor(cfg.colors, "letters");

      lines[0].style.color = symbols.color;
      lines[0].style.textShadow = symbols.shadow;
      lines[1].style.color = letters.color;
      lines[1].style.textShadow = letters.shadow;
      lines[2].style.color = combo.color;
      lines[2].style.textShadow = combo.shadow;
    },
    hide,
  };
}
