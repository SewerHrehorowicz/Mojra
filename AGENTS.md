# Mojra Architecture & Unbreakable Rules for AI Agents

prompt template that proven to be useful:

implementuj <plan>.md zgodnie z @AGENTS.md nie lap za szerokiego kontekstu, nie czytaj calego repo, kontekst byl przygotowany juz to the point

-> this allowed to cut down cost from 5-10$ to about 0.5$! (but might be one off)

## 0. Architectural constraints
- strings in function bodies are strictly forbidden, excect of 1-2 char separators or whitespaces
- keep data structured in data objects (struct, mini-module, const), not as a map of strings; magic strings always point to deeper archotectural problem
- never use UPPERCASE (`SCREAMING_SNAKE_CASE`) identifiers in JavaScript; always use `camelCase`
- no pass-through functions, they are forbidden
- no duped funcionalities, before implementing check if you have the thing in project already
- use project graph you keep in project root in standarized format (not json); read from it to discover project quickly, write to it after changing anything that is relevant each time
- separation of concerns - app is a set of subsystems that communicate wit each other; state handles app state in memory; views handle preparing data for templates; actions handle url actions & dispatching them to views etc
- no stuffing many fields in one struct just to pass arch tests
- do not think twice over same thing, if you already analyzed and formulated plan, act on it
- during refactors existing files, for each task formulate simple step-by-step plan in *.md, inline in chat in one copyable window, and include:
  - what is there and what are problems
  - what are solutions
  - file by file, method my method or line by line (if relevant) describe proposed changes, where it goes (if mixed responsibility), how to rename it

## 1. ALWAYS Keep the Top Bar (`header.topbar#topbar`) Visible — NEVER VIOLATE
- **THE TOP BAR MUST ALWAYS REMAIN VISIBLE AND INTERACTIVE IN EVERY VIEW AND MODAL**:
  - `header.topbar#topbar` (`app/templates/topbar.tpl`, included at the top of `app/templates/app_shell.tpl`) contains the `Mojra` brand logotype and symbol (`app/icons/mojra.svg`), the primary navigation tabs (`Biblioteka`, active book `<Title>`, `Playbook Studio`), and the global window/UI controls (imageless mode, UI zoom, dark/light theme, fullscreen, and quit `✕`).
  - **NEVER** hide, remove, replace, or cover `header.topbar#topbar` in any view (`#view-library`, `#view-player`, `#view-editor`) or in any modal/overlay—**especially when editing scene text (`#modal-scene-text` / `.scene-text-fullscreen-backdrop`)**.
  - Layout and stacking rules that MUST be preserved forever:
    1. `.app-root` (`app/styles/app.tss`) uses a 2-row grid (`grid-template-rows: auto 1fr`) where row `1` is exclusively `header.topbar#topbar` (`grid-row: 1; z-index: 200`) and row `2` (`grid-row: 2`) contains `main.workspace#workspace` and `.modals-root#modals-root`.
    2. `.modal-backdrop` and `.modal-backdrop.scene-text-fullscreen-backdrop` (`app/styles/modals.tss`) must use `position: absolute; inset: 0` inside `.modals-root` (row `2` below `.topbar`), **NEVER** `position: fixed; inset: 0` over the entire viewport, and their `z-index` (`120`) must always remain lower than `.topbar` (`200`).
    3. Clicking any navigation tab (`SwitchView` / `OpenBook`) in `#topbar` while `#modal-scene-text` (or any modal) is open must close the modal (`s.SceneTextModalID = ""`) and switch cleanly to the requested view.
    4. All right-side topbar actions (`ToggleImageless`, `ToggleUIScale`, `ToggleTheme`, `ToggleFullscreen`, `QuitApp`) must remain accessible and functional while editing scene text in `#modal-scene-text`.

## 2. File & Data Formats — ZERO JSON / ZERO YAML / ZERO SQL
- **NEVER use JSON (`encoding/json`, `JSON.stringify`, `JSON.parse`, `.json` files) anywhere** in backend, frontend, configuration, or disk persistence.
- **Book Metadata (`books/<id>/book.playtxt`)**: Plain UTF-8 text using `key: value` lines (`title:`, `author:`, `description:`, `narrated: true|false`, `imageless: true|false`, `start: <scene_id>`, `cover: <rel_path>`, `enum:`, `var:`).
- **Story Files (`books/<id>/story.playtxt` or per-chapter `<chapter_id>.playtxt`)**: Plain UTF-8 text using structural `key: value` headers (`chapter:`, `scene:`, `id:`, `image:`, `voice:`, `next:`, `end: true`, `text:`), prose paragraphs, and `Choice Label -> TargetSceneID` (or `-> END`) transitions.
  - Escape literal arrows in prose as `\->`.
  - A single outgoing transition MUST always normalize to implicit `Dalej` (`next: <target>` or sequential same-file scene), never a single custom-labeled choice button.
- **Node Studio Layout (`books/<id>/.editor.cfg`)**: Plain text using `pan_x: <int>`, `pan_y: <int>`, `zoom: <float>`, and `node: <scene_ref> -> <x>, <y>` snapped to `EditorGridSize = 24`.
- **App Settings & Saves in Standard OS App Data (`book.DefaultAppDataDir()` / `book.DefaultSavesDir()`)**:
  - Built or running apps and tests must **NEVER** write `saves/`, `settings.ini`, `<book-id>.sav`, `.mojra.pid`, or `.history/` into the repository (`repo/saves/`, `repo/dist/saves/`, `books/<id>/.history/` are strictly forbidden).
  - Keep all app settings (`saves/settings.ini`), player saves (`saves/<book-id>.sav`), process lock files (`saves/.mojra.pid`), version history (`history/`), default projects (`projects/`), and exports (`exports/`) in the standard OS application data directory (`os.UserConfigDir()/Mojra`, i.e., `~/Library/Application Support/Mojra` on macOS, `%AppData%\Mojra` on Windows, `$XDG_CONFIG_HOME/Mojra` on Linux) via `book.DefaultAppDataDir()` and `book.ResolveSavesDir(rootDir)`, and use isolated `os.TempDir()` directories during `go test`.
- **App Settings (`saves/settings.ini`)**: INI sections (`[display]`, `[audio]`, `[library]`) with `key = value` pairs written atomically via `.tmp` + `os.Rename`.
- **Player Progress (`saves/<book-id>.sav`)**: Custom Little-Endian binary format (`STSV\x01`). Frequent `AudioTime` updates MUST patch bytes `[5..21]` in-place via `UpdateSaveAudioTimeInPlace` (`os.File.WriteAt`) without rewriting the file.

## 3. Client-Server Protocol — WFP + HTML-Over-The-Wire (`POST /api/action`)
- All UI mutations and commands go through `POST /api/action` using WFP wire framing (`Content-Type: text/x-wfp;charset=UTF-8`, `key=val` header lines + optional `_body=<key>` and `\n\n` raw UTF-8 body, decoded via pooled `sync.Pool` buffers in `internal/gui/wfp.go`) and must pass `validateSameOriginAction` (`Sec-Fetch-Site`, `Origin`, `Referer`).
- Never return JSON from `/api/action`. Render a `.tpl` template chunk and set `X-Target` (`#app`, `#workspace`, `#modals-root`, `#editor-context-panel`, or `none` with HTTP `204 No Content`).
- Sync state and native host commands exclusively via `X-*` HTTP response headers (`X-Theme`, `X-Imageless`, `X-Imageless-Locked`, `X-UI-Scale`, `X-Audio-Speed`, `X-Auto-Choice`, `X-Auto-Strategy`, `X-Keep-Scroll`, `X-Reset-Canvas`, `X-Device-Mode`, `X-Toast`, `X-Native-Cmd`).
- Declare client actions in `.tpl` via `action: <ActionName>` and `data-*` attributes (`app/js/core.js` maps `data-foo-bar` to form param `foo_bar`).
- Encode batch node positions in `SaveEditorState` as `positions: id1:x1,y1;id2:x2,y2`.

## 4. Template (`.tpl`), Stylesheet (`.tss`) & JS Bundling Discipline
- Edit source templates in `app/templates/*.tpl`, stylesheets in `app/styles/*.tss`, scripts in `app/js/*.js`, and icons in `app/icons/*.svg`.
- Never manually edit generated files in `build/app/{index.html,style.css,app.js}` without updating source files in `app/` and running `tpl.BuildApp` (`./build.sh --macos` or `go test ./...`).
- **`.tpl` Language & Compile-Time Security Gates (`internal/tpl/tpl.go`)**:
  - Prefer conditional classes (`.active?: {is_active}`, `.hidden?: {!is_open}`, `.mode-full?: {mode == "full"}`), conditional attributes (`disabled?: {is_locked}`, `data-hint?: {hint_text}`), `{if ...}` / `{elif ...}` / `{else}` chains, `{for item in list}` loops, and `@component` props + `@slot` over redundant `*_class` string construction in Go.
  - No inline `style="..."` attributes, `<style>` tags, `<script>` blocks (only `<script src="/app.js">`), `on*=` event attributes, `<iframe>`/`<object>`/`<embed>`/`<foreignObject>`, or `javascript:`/`data:` URLs.
  - No inline `<svg>` elements in `.tpl`; put all SVGs in `app/icons/*.svg` and include via `svg.class src: /icons/name.svg`.
  - **Minimal JavaScript Bridge & Zero Dynamic Markup**: Keep business/state logic on the Go side (`internal/gui/`, `internal/book/`) and render all markup in `.tpl` templates. Client JS (`app/js/*.js`) is a minimal declarative bridge for `POST /api/action`, audio/scroll bindings, and WebGL canvas rendering—never create DOM elements dynamically (`document.createElement`) or use HTML string literals (`<div`, `<span`, `<button`, `<input`, `<label`, `<svg`, `</`) in `app/js/*.js` (enforced by `TestNoHTMLStringLiteralsInClientJS`).
  - Dynamic context strings (`{var}`, `"{var}"`) are HTML-escaped and never undergo secondary `{...}` interpolation. `{raw ...}` is forbidden at compile time; use `{prose var}` so the `.tpl` engine escapes and formats plain-text prose via `tpl.RenderSafeProseHTML`.
- **`.tss` Stylesheet Rules (`internal/tpl/tss.go`)**:
  - Indentation-based rules without `{}` or `;`. Define theme/preview tokens in `app/styles/colors.tss` using `light-dark(lightVal, darkVal)` and `--var` references.
  - Wrap device preview rules in `/* @dev-only-start */ ... /* @dev-only-end */` and template nodes in `{if dev_mode}` so production builds strip them completely.
  - Register any new `.tss` or `.js` module in `CSSModuleFiles` / `JSModuleFiles` (and `CSSPlayerModuleFiles` / `JSPlayerModuleFiles` if needed in standalone player mode) in `internal/tpl/bundle.go`.
- **UI & Control Rules**:
  - Store all graphic UI/UX design sheets and visual specifications in `designs/` (`designs/forms.png`, `designs/branch-graph.png`, `designs/breakdown-panel.png`, etc.). **NEVER present UI designs, layouts, or mockups as ASCII art in chat** — always generate a real `.png` design sheet in `designs/` and attach/read it.
  - Keep `border-radius: 0 !important` globally (`base.tss`), except circular node ports, slider thumbs, and progress/notch pills.
  - Never use native `<select>` elements; always use `.custom-select`.
  - **Minimalist Form Design System (`designs/forms.png`)**:
    - **Input & Auto-Resizing Textarea**: Always display a `1px` bottom line (`border-bottom: 1px solid --border-strong` at rest, including placeholder fields like `id:`, and `--text-primary` on focus), with transparent background and zero box borders. All textareas auto-resize to fit content (`rows: 1`, `overflow: hidden !important`, `resize: none !important`, zero scrollbars).
    - **Toggle**: No bottom underline (`border-bottom: none`) and no outer switch border; display the state label on the left and the contextual SVG icon + subtle borderless track (`22×10px`) with a `6×6px` square thumb on the right.
    - **Radio**: No bottom underlines and no borders; render options with solid `8×8px` borderless squares (`--border-subtle` idle, `--border-strong` hover, `--text-primary` active).
    - **Dropdown (`.custom-select`)**: Closed trigger has `border-bottom: 1px solid --border-strong` and right-aligned SVG chevron; when open, remove the trigger's bottom border so it merges into a single `1px solid --text-primary` top edge on `.custom-select-menu` (with `--border-strong` on left/right/bottom).
  - **Minimalist Breakdown Panel (`designs/breakdown-panel.png`)**:
    - Never wrap production metrics, act rows, characters, or locations in enclosed card boxes (`border: 1px solid`) or boxed badges. Use a borderless 2-column cross-line grid (`--border-subtle`) for metrics, a single segmented `3px` proportion bar above `--border-subtle`-separated act rows, and borderless character/location rows with a single `--border-strong` → `--text-primary` bottom line that auto-saves on `Enter`/`blur`/`change`.
  - Encapsulate reusable UI primitives via `.tpl` `@component` definitions (`app/templates/components.tpl`) and avoid nested grouping boxes or redundant borders inside `#editor-context-panel`.
  - Place action icons (`expand/collapse`, `close`, `trash`) on the right side of headers, rows, and cards; use SVG icons (`app/icons/*.svg`) instead of text glyphs (`▾`, `×`, `−`), use `trash.svg` for all delete/remove actions, and reserve `close.svg` strictly for closing modals/popovers.
  - Never add `border-bottom` or `text-decoration: underline` to `.topbar-tab` or `.top-action-btn`.
  - **Toolbar & Toggle Icon Buttons**:
    - Never attach numeric counters, badges, or extra labels (`span.history-unsaved-badge`, counts, pills) inside or next to icon buttons in `#topbar` or `.canvas-toolbar`.
    - Single-icon toggle buttons (icons that do not have separate on/off states, such as `#btn-editor-breakdown`, `ToggleEditorHistoryPanel`, `#btn-book-settings`, `.scene-settings-toggle-btn`, and `#btn-auto-choice-toggle`) must use the universal 4-corner bracket active indicator (`::before` / `::after` in `base.tss`), never a full square border.
    - Never duplicate entity names or values in the same card/row (e.g., never render a static title above an input prefilled with the exact same name in `#editor-breakdown-panel`).
  - Never use up/down arrow buttons (`↑`, `↓`) to reorder scenes, choices, or items (rejected at `.tpl` compile time); use 24px-grid canvas dragging (`data-node-drag-grip`) and wire ports.
  - Never explain the UI inside the UI (never write `"Kliknij przycisk..."`, `"Kliknij aby..."`, `"Przeciągnij..."`, or tutorial hints) and never spam the UI with unnecessary text, subtitles, or redundant headers.
  - Keep canvas node cards (`.timeline-node-card`) read-only on the canvas (grip `⋮⋮`, title, choice labels, ports, `+ Wybór`); all editable inputs and delete buttons belong in `#editor-context-panel`.
  - **Right-Side Studio Panels (`320px` Column) vs Portrait Phones**:
    - `.editor-context-panel`, `.editor-history-panel`, `.editor-help-panel`, `.editor-breakdown-panel`, and `.editor-variables-panel` (`app/styles/editor_inspector.tss`) MUST ALWAYS render as a `320px` right-hand column (`width: 320px; min-width: 320px; max-width: 320px; border-left: 1px solid --border-subtle; flex-shrink: 0`) alongside a visible `#timeline-scroll-viewport` canvas on desktop, tablet, and landscape.
    - Full-width panel mode (`width: 100%; flex: 1; border-left: none`) is strictly restricted to portrait phones (`@media (max-width: 760px) and (orientation: portrait)` and `.device-stage[data-preview="portrait"]`). Never hide `#timeline-scroll-viewport` on desktop when a right-side panel is open.

## 5. Performance, Zero External Dependencies & Repository Hygiene
- **Performance Hard Rules (P0)**:
  - Explicitly evaluate hot-path complexity, memory allocations, request-path I/O, and rendering latency in every plan and code change, and keep canvas/pointer/drag interactions at 120fps.
  - Stream template responses via `tpl.RenderTemplateTo` into pooled `sync.Pool` buffers (`tpl.AcquireBuffer` / `tpl.ReleaseBuffer`) and keep `RenderContext.WithVar` zero-copy.
  - Never re-parse `.tpl`/`.tss` or re-bundle `.js` per request when disk files are unchanged; rely on mtime/size stamp caching (`ensureTemplatesLoaded`, `serveUIAsset`) and verify hot paths with `go test -bench=. -benchmem ./...`.
- **Media, Audio Policy & Book Packaging (`internal/book/`)**:
- **NEVER Generate or Synthesize Audio Samples**:
  - Do NOT generate synthetic/ambient `.flac`, `.ogg`, or `.mp3` audio files in code or during `CreateNewBook`.
  - Books default to `narrated: false` (no audio); when narration is enabled (`narrated: true`), the author records and imports real voiceover files into `voice/`.
- **Strict Media Allowlist & Magic-Byte Verification (`internal/book/media.go`)**:
  - Images: ONLY raster `.png`, `.jpg`, `.jpeg`, `.webp` (verify `\x89PNG`, `\xff\xd8\xff`, `RIFF....WEBP`). Strictly forbid `.svg` and `.gif` in book media.
  - Audio: ONLY `.ogg`, `.flac`, `.mp3` (verify `OggS`, `fLaC`, `ID3`/`\xff\xe0`). Strictly forbid `.wav`.
  - Reject any media file > 100 MB or containing HTML/XML/script signatures in the first 512 bytes.
- **All-or-Nothing Narration Policy**:
  - When `narrated: false`, clear all scene audio fields, omit `voice:` from serialized `.playtxt` files, return HTTP `403` on audio requests, and render `#reader-progress-footer` instead of `#audio-deck`.
  - When `narrated: true`, emit a parser warning for every scene missing a valid audio file.
- **Zero-Copy External Books & Read-Only `.playbook` Archives**:
  - `AddBookFromDisk` registers external paths in `settings.ini` (`ExternalBooks`) without copying files into `books/`.
  - `.playbook` / `.zip` archives (`b.IsPacked == true`) are read-only; reject all mutations in `internal/book/mutator.go` and editor actions.
  - Production bundles (`packageBooksForDist`) must ship only `.playbook` archives inside `dist/.../books/` and never copy raw unpacked source folders.
- **Graph-First Ordering & SSRF Protection**:
  - Order chapters and multi-file scenes by BFS graph reachability from `StartSceneID` (`orderChaptersAndScenesByGraph`), never by numeric filename prefixes (`01_...`).
  - `DownloadBookFromURL` must block loopback (outside `go test`), private, link-local, multicast, and CGNAT IPs in `net.Dialer.DialContext` and cap downloads at 250 MB.

## 6. Zero External Dependencies, Native Host, Hygiene & Automated Test Gates
- `go.mod` must remain 100% dependency-free (Go standard library only).
- Native OS windows (`internal/gui/window_*.go`) use Cocoa + `WKWebView` on macOS, Win32 + `EmbeddedBrowserWebView.dll` + COFF `.syso` resources on Windows, and Chromium/Edge `--app` mode with `zenity`/`kdialog` on Linux.
- Never spawn a native window or OS file picker when `ST_NO_WINDOW=1` or running under `go test`.
- `TestTopbarAlwaysVisibleIncludingSceneTextEditor` (`internal/gui/server_test.go`): Fails if `#topbar` is ever removed, hidden, or covered by `.modal-backdrop` / `.scene-text-fullscreen-backdrop`, or if switching views from `#topbar` fails to exit `#modal-scene-text`.
- **Client JS, Action Contract & Overlay Responsiveness Gates (`internal/gui/server_test.go`)**:
  - `TestClientServerActionAndOverlayContracts`: Every `action:` / `data-backdrop-action:` in `app/templates/*.tpl` and `sendServerAction` / `postServerActionSilent` in `app/js/*.js` must exist in `actionSpecs`; `{for}` loops must never reference out-of-scope loop variables; floating non-interactive overlays (`.modals-root`, `.toast`, `.ui-tooltip`, `.editor-perf-overlay`, `.player-bg-stage`, `.scroll-more-cue`) must keep `pointer-events: none` so they never block clicks.
  - `TestClientJSRuntimeAndInteractiveResponsiveness`: Executes bundled `app.js` (prod/dev) in Node VM against live `httptest.Server` to verify real DOM clicks, form submissions, custom-selects, modal lifecycle, and WebGL canvas hit-testing/gestures. Extend this test whenever adding new interactive UI controls or canvas gestures; never test UI features solely via raw Go `http.PostForm` calls.
- **Repository Hygiene & Post-Work Cleanup**:
  - Apply fixes directly without asking for permission or confirmation first; implement and verify immediately.
  - Keep all rules, configs, and edits strictly inside the repository root; never modify or touch files outside the repository root.
  - Never hardcode absolute local paths, usernames, or secrets in code, templates, or tests.
  - Never leave temporary files, scratch tests, or `.py` scripts in the repository; inspect working tree state, run `go test ./...`, and always build the application for the current platform (`./build.sh --macos` on macOS, `./build.sh --linux` on Linux, `./build.sh --windows` on Windows) after changes.
