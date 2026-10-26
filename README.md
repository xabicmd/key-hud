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

## How keys become labels

Lane rotation is 1 → 2 → 3 → 1. A keystroke starts a **new** label when the gap
since the previous key is **greater than 500 ms**, when the input is a **mouse
event**, a **key combination**, or an **action key**. Otherwise the character is
appended to the label already travelling.

Only the four real modifiers (`ctrl`, `shift`, `alt`, `win`) are turned into a
combination prefix. A letter that is still held down because the next key was
pressed a few milliseconds later is just fast typing — that is why "hola" reads
as one word instead of `HO` + `L+A`.

Shift is special-cased by what the key produces. Shift on a letter is just an
uppercase letter, so holding shift to type `HOLA` still gives one word. Shift on
a digit or a punctuation key is how you type `@ # % :`, and those are shown as
the key combination on their own label.

Action keys never join a word and always get their own label: `BACKSPACE`, `TAB`,
`ENTER`, `ESC`, `DELETE`, `INSERT`, the arrows, `HOME`/`END`, `PAGE UP`/`PAGE
DOWN`, the numpad and the function keys.

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
| `shift` + `2` (types `@`) | `SHIFT+2` on the next lane: the keys, not the symbol |
| `ctrl` + `alt` + `delete` | `CTRL+ALT+DELETE` on the next lane |
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
| `symbols` | Anything that is not a letter and not a combination: `1`, `;`, `ESC`, `SPACE`, `F24`. |
| `combo` | Anything containing `+`: `CTRL+C`, `CTRL+SHIFT+P`. |
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

There is no close button. Press **Ctrl+Alt+Q**, or kill the process.

## Customizing further

Colour and speed live in `key-hud.json`. Anything beyond that lives in `web/`,
which is embedded into the binary at build time:

- `web/style.css` — layout, the CRT scanline and vignette layers. Both are purely
  cosmetic: delete those two divs and nothing breaks.
- `web/app.js` — lane layout and the animation.

Motion only touches `transform` and `opacity`, so it runs on the compositor and
you can layer on `filter`, `mix-blend-mode` or SVG effects without slowing the
capture path down.

The label vocabulary lives in `keys.go`.

## Known limits

- Keystrokes typed into elevated (Administrator) windows are invisible to a
  non-elevated hook, thanks to UIPI. Run the app elevated if you need them.
- `Ctrl+Alt+Del` and the secure desktop cannot be captured by any user-mode
  hook.
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
grouping window, lane rotation, colour grouping and config merging. The Node
tests load `web/app.js` directly and pin the motion geometry: constant speed for
short and queued labels, the original fade-in/fade-out positions, the wait that
keeps two labels on one lane from overlapping, and the rule that a label enters
at the screen edge exactly when its lane is clear. The Win32 hooks and the
WebView2 window are verified manually.
