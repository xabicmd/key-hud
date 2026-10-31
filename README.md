# key-hud

A decorative fullscreen strip that shows every key and mouse button you press
anywhere in Windows, as pixel text that sweeps right-to-left and fades out.

Built for a 1920x480 secondary display: borderless, topmost, no scrollbars, no
close button, no F11 required.

![four colour groups sweeping across the strip](docs/preview.png)

## How it works

```
WH_KEYBOARD_LL + WH_MOUSE_LL (locked OS thread, Win32 message pump)
        │  Event{new|append}
        ▼
   grouper ── lane + colour group ──► WebView2 (HTML/CSS, embedded)
```

- The hooks are global: they fire no matter which application has focus.
- A `WS_POPUP` window sized to the target monitor rect *is* fullscreen. There is
  no browser chrome to hide.
- The UI is plain HTML/CSS served from an embedded filesystem on a loopback
  port, so web fonts and normal browser semantics work.
- Only one copy can run. A second launch exits, because two copies would install
  two sets of hooks and paint every keystroke twice.

## Start screen

On launch the strip stays silent and shows three lines:

```
KEY-HUD BY XABICMD
START (ENTER)
TO CLOSE (CTRL+ALT+Q)
```

Press **Enter** to dismiss them and start capturing. Until then every key and
mouse event is swallowed before it becomes a label, so nothing typed into the
intro can shift the lane rotation or leak a stray label once the strip starts.
**Ctrl+Alt+Q** still closes the window while the intro is up.

## Requirements

- Windows 10/11 with the WebView2 runtime (preinstalled on current builds).
- Go 1.22+ to build.

## Build and run

```powershell
.\build.ps1                    # or .\build.bat; accepts -Output <name>.exe
.\key-hud.exe
```

`build.ps1` forces a full rebuild (`-a`) from a temporary clean copy of the
sources and supports `-Output <path>.exe` to place the binary elsewhere.
`-H windowsgui` suppresses the console window; omit `-H windowsgui` during
development to see logs on stderr.


## Project layout

One binary, several packages. `cmd/key-hud` is the entry point and the only
place that wires the parts together; everything else lives under `internal/`,
split by domain rather than by layer:

```
cmd/key-hud/          flags, wiring, process lifetime
internal/win32/       DLL bindings, constants and C struct layouts (no decisions)
internal/config/      key-hud.json: defaults, merge, write-on-first-run
internal/platform/    monitor enumeration, overlay window styling, single instance
internal/capture/     global hooks, key translation, grouping: one Event per input
internal/stage/       embedded web assets, loopback origin, push into the page
```

The dependencies form a DAG, so a package can only see what it legitimately
needs:

| Package | Responsibility | Imports |
| --- | --- | --- |
| `cmd/key-hud` | flags and wiring: config, monitor, window, gate, hooks | all |
| `internal/stage` | embedded assets, the loopback HTTP origin and the push into the page | `capture`, `config`, `win32` |
| `internal/capture` | global Win32 hooks, one decoded event to one label instruction | `config`, `win32` |
| `internal/platform` | monitor enumeration and `-monitor` parsing, overlay styling, single-instance mutex | `win32` |
| `internal/config` | `key-hud.json`: defaults, merge and write-on-first-run | — |
| `internal/win32` | DLL functions, constants and C struct layouts | — |

Inside `internal/capture` the input state is one explicit `capture.Pipeline`
built by `main` and passed down: the held keys, the repeat runs, the lane
rotation and the optional intro `Gate` all live in that value. Nothing is
shared through package globals, so a second pipeline in a test starts from
zero and cannot see the first one.

Each `foo.go` has a `foo_test.go` next to it. Go tests must live in the package
they test: they call unexported functions, and the toolchain only compiles
`_test.go` files together with the package in the same directory. The `tests/`
directory is not Go tests; it holds the Node tests for the page, and they reach
the modules under `internal/stage/web/` with a relative import.

## How keys become labels

Lane rotation is 1 → 2 → 3 → 1. A keystroke starts a **new** label when the gap
since the previous key is **greater than 500 ms**, when the input is a **mouse
event**, a **key combination**, or an **action key**. Otherwise the character is
appended to the label already travelling.

Only the four real modifiers (`ctrl`, `shift`, `alt`, `win`) are turned into a
combination prefix. A letter that is still held down because the next key was
pressed a few milliseconds later is just fast typing — that is why "hola" reads
as one word instead of `HO` + `L+A`.

Shift and AltGr are read as what they type, not as key combinations. The app asks
the active keyboard layout what the press produces, so `shift+2` shows `@`,
`shift+\` shows `|`, and a dead accent followed by a letter shows the accented
letter (`´` then `a` gives `Á`). A dead key on its own has not typed anything
yet, so it shows nothing; the character appears with the key that composes it.

Only a press that types no character at all is a combination: `CTRL+C`,
`CTRL+ALT+DELETE`, `ALT+TAB`, `SHIFT+F5`. Left alt with ctrl is a command, not
AltGr, so it stays a combination; right alt is AltGr and it types.

Action keys never join a word and always get their own label: `BACKSPACE`, `TAB`,
`ENTER`, `ESC`, `DELETE`, `INSERT`, the arrows (`←` `↑` `→` `↓`), `HOME`/`END`,
`PAGE UP`/`PAGE DOWN`, the numpad and the function keys.

Repeating one of those is a single gesture. Holding backspace, or pressing it
five times to delete five characters, shows one `BACKSPACE`; the same goes for
tab, the arrow keys, `PAGE UP`/`PAGE DOWN` and `HOME`/`END`. A different key, a
typed character or a combination ends the run.

A **bare space** is the one exception. It ends the word and leaves its blank
behind it — the blank becomes part of the word's own label — but it does **not**
take a lane. The blank is invisible, so giving it a lane would look like an empty
line and push every following word one row further, which reads as a broken
1 → 2 → 3 rotation. Only labels you can actually see take a turn. A combination
such as `CTRL+SPACE` is a normal combination and keeps its own lane.

| You type | What appears |
| --- | --- |
| `h` `o` `l` `a` within 500 ms of each other | one label `HOLA` |
| `hola`, then a > 500 ms pause, then `m` | `HOLA` on lane 1, `M` on lane 2 |
| `te` space `comparto` space `un` space `ejemplo` | `TE` on lane 1, `COMPARTO` on lane 2, `UN` on lane 3, `EJEMPLO` on lane 1 |
| `ctrl` + `c` | `CTRL+C` on the next lane, always its own label |
| `shift` + `2` (types `@`) | `@`: the character, not the keys |
| `shift` + `\` (types `\|`) | `\|` |
| `´` then `a` | `Á`: the dead accent and the letter are one character |
| right alt + `e` (types `é`) | `É` |
| `ctrl` + `alt` + `delete` | `CTRL+ALT+DELETE` on the next lane |
| `alt` + `tab` | `ALT+TAB` on the next lane |
| `backspace` five times in a row | one `BACKSPACE` |
| the wheel up five notches in a row | one `WHEEL UP` |
| `left` `up` `right` `down` arrows | `←` `↑` `→` `↓` |
| any mouse button | `CLICK` / `RIGHT CLICK` / `WHEEL UP` … on the next lane |

A label is capped at 16 characters so an endless stream of fast typing cannot
grow one wider than the screen.

Every label travels at exactly the same speed. The animation distance is a fixed
pixel value (`viewport width + travelEndPx`) that does not depend on how wide the
label is, so a short `A` and a long `CTRL+SHIFT+F23` can never drift apart.

Because the speed never changes, a new label cannot overtake the one already
travelling on its lane. Instead it starts as far right as needed to clear that
label's trailing edge, plus 0.6 em of breathing room. A burst of combinations
therefore queues just outside the right edge and slides in one after another
instead of stacking on top of each other in the middle of the strip. Labels are
never dropped to make room: a busy strip delays the newest label, it never picks
which keypress to forget.

Waiting per lane is not enough. If lane 2 was still busy while lane 3 was free,
the third label entered before the second one and the strip appeared to jump
1 → 3 → 2. Labels now pass through one global queue and enter strictly in the
order they were typed: lane 1, lane 2, lane 3, lane 1. A blocked label holds the
whole queue back for a moment; nothing is ever reordered to catch up.

## Configuration

The app writes `key-hud.json` next to the executable on first run and reads it
back on every start. Delete it to get the defaults again. A typo is not fatal:
invalid JSON falls back to the defaults and logs why.

```json
{
  "speedPxPerSec": 320,
  "groupWindowMs": 500,
  "lanes": 3,
  "fontSizeVh": 17,
  "travelEndPx": 700,
  "colors": {
    "letters": { "text": "#eafcff", "glow": "#35d6ff", "glowFar": "#0a5f7a" },
    "symbols": { "text": "#ffe9b8", "glow": "#ffb703", "glowFar": "#7a4d00" },
    "combo":   { "text": "#e6d4ff", "glow": "#a06bff", "glowFar": "#43148f" },
    "mouse":   { "text": "#ffd9f6", "glow": "#ff3fd0", "glowFar": "#6d0a58" }
  }
}
```

| Key | Meaning |
| --- | --- |
| `speedPxPerSec` | Horizontal speed of every label. |
| `groupWindowMs` | Gap above which a keystroke starts a new label instead of appending. |
| `lanes` | Number of horizontal lanes the labels rotate through. |
| `fontSizeVh` | Glyph size as a percentage of screen height, snapped to whole pixels. |
| `travelEndPx` | How far past the left edge a label travels before it is removed. |
| `colors` | One entry per group; any group you leave out keeps its default. |

The four colour groups:

| Group | Covers |
| --- | --- |
| `letters` | A single letter: `A`, `Z`, `Ñ`. |
| `symbols` | Anything that is not a letter and not a combination: `1`, `@`, `;`, `ESC`, `F24`. |
| `combo` | Keys pressed together that type no character: `CTRL+C`, `CTRL+ALT+DELETE`. A typed `+` is text, not a combination. |
| `mouse` | Every mouse button and wheel event, including `CTRL+CLICK`. |

Each group has three colours: `text` is the glyph, `glow` is the tight halo,
`glowFar` is the wide bloom. Leave a field out and it keeps its default; delete a
group and it keeps its default too.

Changes are picked up on the next start, not live.

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-monitor` | `1920x480` | Target screen: `WxH`, a 1-based index, or `primary`. |
| `-config` | next to the exe | Path to a different `key-hud.json`. |
| `-topmost` | `true` | Keep the strip above other windows. |
| `-no-activate` | `false` | Never take keyboard focus. |
| `-ignore-injected` | `false` | Ignore synthetic input from macros or remote sessions. |
| `-debug` | `false` | Enable devtools and the context menu. |

If the requested monitor is not found the app exits with the list of available
screens instead of guessing.

## Exit

There is no close button. Press **Ctrl+Alt+Q**, or kill the process. Press
**Enter** first if the intro screen is still up.

## Customizing further

Colour and speed live in `key-hud.json`. Anything beyond that lives in
`internal/stage/web/`, which is embedded into the binary at build time:

- `internal/stage/web/style.css` — layout, the CRT scanline and vignette layers.
  Both are purely cosmetic: delete those two divs and nothing breaks.
- `internal/stage/web/splash.css` — the intro screen layout. Its colours come
  from `key-hud.json`.
- `internal/stage/web/splash.js` — the intro screen: the three lines,
  `window.__hudStart` and the `hudIntro` read that recovers a start that happened
  before the page loaded.
- `internal/stage/web/app.js` — entry point: fetches `/api/config` and exposes
  `window.__hud`.
- `internal/stage/web/stage.js` — the label elements, the per-lane anchors and the
  release queue.
- `internal/stage/web/motion.js` — pure sweep geometry. The Node tests import this
  file directly.
- `internal/stage/web/config.js` — fallback config, the merge with `key-hud.json`,
  colour styles.

The page uses native ES modules: `index.html` loads `app.js` with
`type="module"` and the other files import each other over the loopback origin.
There is no bundler and no build step. `internal/stage/web/package.json` only
tells Node that directory is an ES module tree; the browser ignores it.

Motion only touches `transform` and `opacity`, so it runs on the compositor and
you can layer on `filter`, `mix-blend-mode` or SVG effects without slowing the
capture path down.

The label vocabulary lives in `internal/capture/keys.go`; the translation of a
key press into the character the active layout types lives in
`internal/capture/typing.go`.

## Known limits

- **The exe cannot run from a tree with a Low mandatory integrity label.** A
  sandbox tool can mark a tree (for example `C:\Users\neoho\www`, set as
  `Etiqueta obligatoria\Nivel obligatorio bajo`, `(OI)(CI)`); every copied or
  freshly built exe inside inherits the label, launches as a low-integrity
  process, and WebView2 then fails with `0x800700AA` in
  `CreateCoreWebView2Controller`. The jchv/go-webview2 binding checks
  `int64(res) < 0`, misses the positive `0x8007xxxx` HRESULT, and panics with
  a nil pointer dereference instead of reporting the error. The same binary
  outside the tree runs fine; stripping the DACL never fixes it, only the
  integrity label does. Fix for one project folder (leave the rest of the
  sandbox alone):
  ```powershell
  icacls C:\Users\neoho\www\key-hud /setintegritylevel (OI)(CI)M /T
  ```
- Keystrokes typed into elevated (Administrator) windows are invisible to a
  non-elevated hook, thanks to UIPI. Run the app elevated if you need them.
- `Ctrl+Alt+Del` and the secure desktop cannot be captured by any user-mode
  hook.
- The character is translated with the layout of the window in front, and the
  translation is read-only (`flagNoStateChange`). ToUnicodeEx without that flag
  writes to the kernel-mode keyboard buffer, which every application shares: the
  accent the user is typing gets eaten and no app types `á` or `ñ` until this one
  closes. Because the pending accent lives in that shared buffer, an accent
  pressed in one app and a letter typed in another can still be shown as one
  character.
- No autostart is installed. Add a shortcut to `shell:startup` yourself if you
  want it on boot.

## Engram project identity

`.engram/config.json` tells the Engram memory system which project this
repository is:

```json
{
  "project_name": "key-hud"
}
```

Create or recreate it with:

```powershell
engram init key-hud
```

It is committed on purpose. Without the file Engram guesses the project from the
git remote, and when the working directory sits above several repositories that
guess is ambiguous and writes are refused. With the file present, any tool
running inside `key-hud` records to the `key-hud` project deterministically,
regardless of what the parent directory contains.

Nothing in the app reads this file. Delete it if you do not use Engram.

Note that it only resolves paths *inside* this repository. An MCP server or
agent process started from the parent directory cannot use it, because project
detection runs from the process's own working directory before the repository is
entered.

## Tests

```powershell
go test ./...
node --test tests/
```

The Go tests cover key naming, modifier normalization, combination ordering, the
character the active layout types (a pending accent still composes, and
translating never changes the shared keyboard state), the
collapse of a repeated action key, the grouping window, lane rotation, colour
grouping and config merging. The Node
tests import `internal/stage/web/motion.js` directly and pin the motion geometry:
constant speed for short and queued labels, the original fade-in/fade-out
positions, the wait that keeps two labels on one lane from overlapping, and the
rule that a label enters at the screen edge exactly when its lane is clear. One
more test boots `internal/stage/web/app.js` in Node with a stubbed document, so a
broken module graph or a broken `window.__hud` wiring fails there instead of only
in the window. The splash has its own pair: `gate_test.go` pins the gate that
mutes input until Enter and keeps a held Enter silent, and
`tests/splash.test.mjs` pins the three lines and the two calls that drop them.
The Win32 hooks and the WebView2 window are verified manually.

## License

key-hud is free software: you can redistribute it and/or modify it under the
terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version.

It is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY;
without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR
PURPOSE. See the GNU General Public License for more details.

The full text is in [`LICENSE`](LICENSE).

Copyright (C) 2026 @xabicmd
