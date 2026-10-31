# AGENTS.md — key-hud

Windows-only desktop app: an always-on-top strip that shows every keystroke via
WebView2. One Go binary, no bundler, native ES modules in the embedded web
assets. Needs Go 1.22+ and Node 18+ (built-in `node:test`).

## Commands (run on a Windows host)

```powershell
go test ./...                  # Go tests (package-local, they touch Win32)
node --test tests/             # Node tests: motion geometry, app.js boot, splash
node --test tests/app.test.mjs # single focused file
go vet ./...
go build -ldflags "-H windowsgui -s -w" -o key-hud.exe ./cmd/key-hud
```

- Tests assume Windows: the `internal/win32` syscall bindings do not build
  anywhere else.
- Omit `-H windowsgui` while developing to see logs on stderr.
- There is no CI configured on `main`; run the commands above yourself.

## Architecture

- `cmd/key-hud/main.go` is the only wiring point: config → monitor → window →
  capture pipeline → WebView2 stage. Keep it the single importer of everything.
- Package dependencies form a DAG, downward only:
  `stage` imports `capture`, `config`, `win32`; `capture` imports `config`,
  `win32`; `platform` imports `win32`; `config` and `win32` import nothing.
- Input state lives in one `capture.Pipeline` value built by `main` and passed
  down — no package globals. A second pipeline in a test starts from zero. Do
  not reintroduce package-level mutable state.
- `internal/win32` holds DLL bindings, constants and C struct layouts only — it
  makes no decisions; keep policy out of it.
- The UI lives in `internal/stage/web/`, embedded with `embed` and served over a
  loopback HTTP origin. Native ES modules (`index.html` loads `app.js` with
  `type="module"`); there is no build step or bundler.
- `internal/stage/web/motion.js` is pure sweep geometry and is pinned byte-level
  by `tests/app.test.mjs`; a geometry change must keep those tests green.

## Traps an agent will hit

- **The app exits "immediately" on second launch by design.** A
  `Local\key-hud-single-instance` mutex makes a second copy exit silently. When
  a launch seems to crash at start, first check whether a copy is already
  running before reading code.
- **`ToUnicodeEx` in `internal/capture/typing.go` must keep
  `flagNoStateChange` (`0x04`).** Without it the translation writes the shared
  kernel keyboard buffer and eats the user's pending accent system-wide — no
  `á`/`ñ` types in any app until this one closes. Never "simplify" that call.
- **A low mandatory integrity label on a tree breaks WebView2 startup.** An exe
  carrying the label launches as a low-integrity process and panics on start:
  CreateCoreWebView2Controller returns 0x800700AA and jchv/go-webview2 checks
  `int64(res) < 0`, missing the positive 0x8007xxxx HRESULT, so the handler
  dereferences a nil controller. Observed with exes under
  `C:\Users\neoho\www` (sandboxed tree, label inherited by every file). It is
  NOT a build-cache problem and NOT the NTFS DACL; the identical binary runs
  elsewhere. The project folder was fixed with
  `icacls C:\Users\neoho\www\key-hud /setintegritylevel (OI)(CI)M /T`; a fresh
  exe in other low-labeled trees inherits the label and panics again. See the
  "Build and run" comment in build.ps1 and README Known limits.
- **A freshly built exe can panic at startup** with a nil pointer dereference in
  `CreateCoreWebView2ControllerCompleted` (go-webview2). Before blaming code
  from the panic signature alone, first rule out the integrity-label trap
  above, then a stale Go build cache (`go build -a` or `go clean -cache`).
- **Global hooks cannot see keystrokes typed into elevated (Administrator)
  windows** — UIPI, not a bug. `Ctrl+Alt+Del` is unreachable by user-mode hooks.
- **`key-hud.json` is written next to the exe on first run** (gitignored).
  Invalid JSON is not fatal: it logs why and falls back to defaults. Delete the
  file to reset. Config changes are picked up on the next start, never live.
- **Default `-monitor` is `1920x480`** — the developer's secondary strip. If
  that screen is not attached the app exits with the list of available screens
  instead of guessing; use `-monitor primary` or `-monitor WxH`. The WebView2
  cache lives under `os.UserCacheDir()\key-hud\webview2`; deleting it resets
  browser state.

## Verification gap

The Win32 hooks, the WebView2 window and the actual rendered strip are only
verified manually — `go test` and the Node suite cannot catch a broken window.
Before declaring UI/behavior work done, build the exe, run it, and look at the
strip. `-debug` enables devtools and the context menu.
