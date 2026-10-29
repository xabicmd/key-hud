# package-split — split key-hud into cmd/ + internal/ by domain, remove package globals

Branch `structure` · delivery `single-pr` · test runner `go test ./...` + `node --test tests/`

## Specs

**S1 — coupling.** "resuelve el problema de acoplamiento": the five mutable package
globals (`splashRef`, `grouperRef`, `ignoreInjected`, `hookEvents`, `readKeyText`) stop
existing. Input state lives in one explicit `capture.Pipeline` created by `main` and
passed down; no free function mutates package state.

**S2 — structure.** `cmd/key-hud/main.go` + `internal/{capture,stage,platform,config,win32}`.
No `pkg/` (nothing is shared), no `api/` (no protos/OpenAPI), no `models/`, `helpers/`,
`utils/` (explicit anti-pattern in the user's reference text). Dependencies are a DAG:
`win32` and `config` are leaves, `platform -> win32`, `capture -> {win32, config}`,
`stage -> {capture, config, win32}`, `cmd -> all`.

**S3 — scalability seams** (user's stated roadmap: "pantallas de configuracion y que sea
compatible con otras pantallas o modo ventana"): config is its own package so the
settings screen has a home; `platform.PickMonitor`/`platform.StyleAsOverlay` are the
multi-monitor / windowed-mode seam; `stage.Serve` owns the asset origin and `/api/config`.

**S4 — behaviour is unchanged.** Same labels, combos, dead keys, wheel repeat, intro gate,
Ctrl+Alt+Q, single instance, `key-hud.json` handling. No CLI flag changes.

**S5 — tooling keeps working.** `node --test tests/` and the README layout section are
updated to the new paths. NOTE (L3): `build.ps1` and `.github/workflows/ci.yml` do not
exist on `main` — they lived only on the deleted `beam` branch — so there is nothing to
update there.

## Tasks

| ID | Spec | Route | Status | Proof |
| --- | --- | --- | --- | --- |
| T1 | S2 | inline | done | `go build ./...` clean |
| T2 | S1 | inline | done | `go vet ./...` clean, `go test ./...` all ok |
| T3 | S3 | inline | done | `go build`, README layout section rewritten |
| T4 | S5 | inline | done | `node --test tests/` 15/15, README paths updated |
| T5 | S4 | inline | pending | manual: exe launch + hooks + intro + Ctrl+Alt+Q |

## Log

**L1** — user, verbatim: "voy a borrar beam, asi que no lo tengas en cuenta. no es tanto
por estetica sino por escalabilidad, pienso seguir metiendo cosas en la app como pantallas
de configuracion y que sea compatible con otras pantallas o modo ventana, etc. asi que
resuelve el problema de acoplamiento y estructura un poco como te pedi"

**L2** — finding: `input.go:40,148` reads `splashRef` (a UI gate) from the key decoder,
`input.go:217` writes into `hookEvents` created in `hook.go:16`, and `hook.go:31` +
`main.go:64` both initialise `grouperRef`. That is the real coupling; directories only
made it visible.

**L3** — `build.ps1` and `.github/workflows/ci.yml` are absent from `main`; they only
existed on the deleted `beam` branch. S5 is therefore scoped to the Node tests and the
README. `git ls-files` confirms the only non-Go root files are docs, config and tests.

**L4** — decision: the mouse "kind" passed through the grouper was the literal string
`"mouse"`, i.e. the same value as the colour group. Replaced the bare `config.GroupMouse`
uses with a named `kindMouse = config.GroupMouse` constant so the grouper reads as a kind
switch, with no behavioural change.

**L5** — verification evidence: `gofmt -l .` empty; `go vet ./...` clean;
`go test ./...` → capture, config, platform all `ok`; `node --test tests/` → 15 pass,
0 fail; `go build -ldflags "-H windowsgui -s -w" -o key-hud.exe ./cmd/key-hud` produced
the binary. Differential review of `hook.go` and `input.go` against `dc4bf57` shows a
1:1 mechanical translation: same hook installation order, same hotkey registration, same
event buffer size (512), same repeat/rate-limit constants, same emit semantics.

**L6** — T5 stays pending: a global low-level hook, the WebView2 window and the overlay
styling cannot be asserted from a test. Needs one manual launch of the new exe.

**L7** — independent verification (Risk item 6: `hook.go` and `window.go` are not covered
by any test): a read-only verifier diffed every rewritten file against `dc4bf57` and ran
its own differential probes. Verdict PASS, no blockers: 116/116 probe lines identical
(base ↔ new) on every `Event` JSON, the nine mouse payload strings, the 512 buffer, the
label cap, lane cursor, `classify` literals, the gate state machine, both wheel limits,
all Win32 constant values and struct sizes, the flag help text and the monitor error
paths. The nine web assets are byte-identical to the base. It also contributed two
regression suites, now committed: `internal/capture/wire_test.go` (JSON wire format, rate
limits, per-pipeline state, non-blocking buffer) and `internal/stage/serve_test.go`
(embed root, `/api/config` shape, `noCache`, 404).
