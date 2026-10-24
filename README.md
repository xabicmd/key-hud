# key-hud

A decorative fullscreen strip that shows every key and mouse button you press
anywhere in Windows, as pixel text that sweeps right-to-left and fades out.

Built for a 1920x480 secondary display: borderless, topmost, no scrollbars, no
close button, no F11 required.

![labels sweeping across the strip](docs/preview.png)

## How it works

```
WH_KEYBOARD_LL + WH_MOUSE_LL (locked OS thread, Win32 message pump)
        │  chan Event
        ▼
   main goroutine ── Dispatch ──► WebView2 (HTML/CSS, embedded)
```

- The hooks are global: they fire no matter which application has focus.
- A `WS_POPUP` window sized to the target monitor rect *is* fullscreen. There is
  no browser chrome to hide.
- The UI is plain HTML/CSS served from an embedded filesystem on a loopback
  port, so web fonts and normal browser semantics work.

## Requirements

- Windows 10/11 with the WebView2 runtime (preinstalled on current builds).
- Go 1.22+ to build.

## Build and run

```powershell
go build -ldflags "-H windowsgui -s -w" -o key-hud.exe .
.\key-hud.exe
```

`-H windowsgui` suppresses the console window. Omit it during development to
see logs on stderr.

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-monitor` | `1920x480` | Target screen: `WxH`, a 1-based index, or `primary`. |
| `-dur` | `7` | Seconds a label takes to cross the strip. |
| `-lanes` | `0` | Horizontal lanes; `0` fits as many as the height allows. |
| `-topmost` | `true` | Keep the strip above other windows. |
| `-no-activate` | `false` | Never take keyboard focus. |
| `-ignore-injected` | `false` | Ignore synthetic input from macros or remote sessions. |
| `-debug` | `false` | Enable devtools and the context menu. |

If the requested monitor is not found the app exits with the list of available
screens instead of guessing.

## Exit

There is no close button. Press **Ctrl+Alt+Q**, or kill the process.

## Customizing the look

Every knob lives in the `:root` block of `web/style.css`:

```css
--key-glow: #35d6ff;     /* keyboard glyph glow */
--mouse-glow: #ff3fd0;   /* mouse event glow */
--font-size: 17vh;       /* overridden in JS, snapped to whole pixels */
--travel: 7s;            /* matches -dur unless you override it */
```

Because the strip is plain DOM + CSS, motion and glow are compositor-only
(`transform`/`opacity`), so you can layer on filters, blends, or SVG effects
without touching the capture path. `#scan` and `#vignette` are purely cosmetic;
delete those two divs and nothing breaks.

Lane assignment, label creation and the animation live in `web/app.js`. The
label vocabulary lives in `keys.go`.

## Known limits

- Keystrokes typed into elevated (Administrator) windows are invisible to a
  non-elevated hook, thanks to UIPI. Run the app elevated if you need them.
- `Ctrl+Alt+Del` and the secure desktop cannot be captured by any user-mode
  hook.
- No autostart is installed. Add a shortcut to `shell:startup` yourself if you
  want it on boot.

## Tests

```powershell
go test ./...
```

Covers key naming, modifier normalization, and combination ordering. The Win32
hooks and the WebView2 window are verified manually.
