# Mojra View–Template–Controller Architecture (`ARCH.md`)

Canonical specification for how state, views, templates, chunks, and client controllers are structured and how data travels from Go to the client. Both humans and AI agents MUST follow this architecture for every new feature and refactor.

---

## 1. Subsystem Separation of Concerns (`engine/` vs `domain/`)

```
[ domain/state ] --> [ domain/actions ] --> [ domain/views ]   --> [ engine/templates + engine/fdp ] ==(FDP / HTTP)==> [ Dispatcher ] --> [ <Name>Controller ]
(in-memory)          (ActionHandler ->      (TemplateData +        (key=val + \n\n HTML)                                (Fdp.decode, DOM   (data, bindings(),
                      []ActionFrame)         ScriptData)                                                                 swap, hydration)   settings, methods)
```

### 1.1 Generic Engine (`engine/` — Zero Imports from `domain/`)
1. **`engine/templates`**: Compiler, validator, and streaming renderer for `.tpl` templates (`TemplateChunk`, `RenderTemplateTo`) and `.tss` stylesheets.
2. **`engine/osbridge`**: Native OS windowing (Cocoa/`WKWebView`, Win32, Linux, Android), native file pickers, and audio capture.
3. **`engine`** (`server.go`, `dispatcher.go`, `fdp.go`, `bundle.go`, `i18n.go`, `qr.go`, `telemetry.go`):
   - `EngineServer` (`server.go`): Generic HTTP server (`GET /`, static/bundled asset cache, CSP/COOP/COEP headers, `GET /api/lan-qr.png`, `POST /api/action`).
   - `dispatcher.go` & `fdp.go`: `FlatDataProtocol` (`FDP`) multi-frame codec (`DecodeFDPBytes`, `WriteFDPFrames`), same-origin/CSRF/loopback validation, and `WriteRenderedActionFrames`.
   - `bundle.go`: `.tpl`/`.tss`/`.js` bundler parameterized via `JSHeaderBuilder`, `GeneratedAssetBuilders`, and `NonPlayerModulePrefixes`.
   - `i18n.go`: Generic `.cfg` catalog parser, `RegisterCatalog`, `T`/`Tf`/`Plural`, `PopulateI18nContext`, `NewViewContext`, and `BuildJSI18nHeader`.

### 1.2 Application Domain (`domain/` — Built on `engine/`)
1. **`domain` (`domain/app_main.go`)**: Application entry point (`NewServerWithOptions`) that boots `state.AppState` (`settings.ini`, library `books/`, `PlayerState`, `EditorState`) and wires `views.AppShellView`, `actions.HandleAction`, and `/api/books/` media routes into `engine.EngineServer`.
2. **`domain/i18n` (`keys.go`, `pl.cfg`, `en.cfg`)**: Embedded Mojra translation catalogs and `Tr*` keys registered via `engine.RegisterCatalog`.
3. **`domain/book`**: `.playtxt`/`.playbook` model, parser, mutator, exporters, media verifier, and `BuildStudioUIAtlas` (`studio_atlas.go`).
4. **`domain/state`**: Pure in-memory application state (`AppState`, `PlayerState`, `EditorState`, `Settings`). Zero HTTP, template, or view-formatting logic.
5. **`domain/actions`**: Handles `POST /api/action` requests decoded via `FDP`, mutates `state`, and returns explicit `[]engine.ActionFrame` slices (`frameApp`, `frameWorkspace`, `frameModals`, `frameInspector`, `frameSceneForm`, `frameUndoRedo`). Each `<name>_actions.go` registers its own `ActionHandler` functions directly into `Handlers` (`registerActions` / `registerLANActions` / `registerStandaloneActions`)—there is **no `specs.go`**, **no `action.go`**, and **no `ActionSpec` struct**.
6. **`domain/views`**: Transforms `state` into strongly-typed view chunks (`templates.TemplateChunk`) with strict separation between presentational `TemplateData` (`*<name>Template`) and client controller state `ScriptData` (`*<name>Script`).
   - **Strict Separation Rule**: Each `<name>_view.go` file defines at most one template struct (`<name>Template`) for `.tpl` rendering and one optional script struct (`<name>Script`) for `FDP` controller hydration. Every view is a first-class peer `templates.TemplateChunk` (there are no "sub-views" or helper view structs)—views compose other views solely via `templates.TemplateChunk` or `[]templates.TemplateChunk` fields on `TemplateData`, and reuse domain structs from `domain/book` directly.
7. **`app/js/fdp.js` (`Fdp`) & `app/js/dispatcher.js` (`Dispatcher`)**: Universal client codec and runtime. `Fdp` encodes/decodes flat `key=val\n...\n\n<body>` frames in both directions; `Dispatcher` owns `POST /api/action` transport, strict contract hydration (`key in ctrl.data`), `target` DOM swapping, delegated event routing, and `init` lifecycle mounting. There is **no `Core` module**.
8. **`app/js/controllers/<name>_controller.js` (`<Name>Controller`)**: Isolated UI controllers paired 1:1 with their view/template triad, stored in `app/js/controllers/`.

---

## 2. The 1:1 Triad Convention

Every interactive UI unit forms a strict 1:1:1 triad derived by convention from its domain `<name>` (`snake_case` / `camelCase` / `PascalCase` / `kebab-case`) with the role suffix at the end and zero presentation-style leaks (`modal_`, `_panel`):

| Artifact | Naming Convention | Example (`player`) | Example (`editor_context`) |
| :--- | :--- | :--- | :--- |
| Go View (`domain/views/`) | `<name>_view.go` | `player_view.go` | `editor_context_view.go` |
| Go Constructor | `<PascalName>View` | `PlayerView` | `EditorContextView` |
| `TemplateData` / `ScriptData` | `<camelName>Template` / `<camelName>Script` | `playerTemplate` / `playerScript` | `editorContextTemplate` / `editorContextScript` |
| Template (`app/templates/`) | `<name>_view.tpl` | `player_view.tpl` | `editor_context_view.tpl` |
| JS Controller (`app/js/controllers/`) | `<name>_controller.js` (`<PascalName>Controller`) | `player_controller.js` (`PlayerController`) | `editor_context_controller.js` (`EditorContextController`) |
| Go State (`domain/state/`) | `<name>_state.go` | `player_state.go` | `studio_state.go` |
| Go Actions (`domain/actions/`) | `<name>_actions.go` | `player_actions.go` | `studio_scene_actions.go` |
| Root DOM ID | `#<kebab-name>-view` | `#player-view` | `#editor-context-view` |

- **Zero Hardcoded Maps**:
  - `chunkTemplateName` derives `<name>_view` automatically from `*<camelName>Template`.
  - `scriptControllerName` derives `<PascalName>Controller` automatically from `*<camelName>Script`.
- **Isolation Rule**: A controller may only query or manipulate DOM elements belonging to its own template, or call public methods on another controller (e.g., `PlayerController` delegates `.scene-card` queries/focus to `SceneController`).

---

## 3. `TemplateChunk` Composition & FlatDataProtocol (`FDP`)

### 3.1 Building Chunks in Go (`domain/views/`)
Every view is an independent `templates.TemplateChunk` splitting presentational data from controller state (with no `Template string` field):

```go
type TemplateChunk struct {
    TemplateData any // e.g. *playerTemplate -> used ONLY by the .tpl engine
    ScriptData   any // e.g. *playerScript   -> used ONLY by FDP -> ctrl.data
}
```

- **`TemplateData`**: Consumed exclusively by the `.tpl` HTML renderer. Holds presentational strings (`HeaderMeta`, `RailTitle`, `Name`, `Slugline`, `AudioDurationLabel`, `Prose`) and nested `templates.TemplateChunk` or `[]templates.TemplateChunk` child chunks. Never serialized into the `FDP` key-value header block.
- **`ScriptData`**: Consumed exclusively by the `FDP` serializer (`ScriptData` -> flat `key=val` lines -> `ctrl.data`). Holds only scalar runtime state required by client JS controllers. When a chunk requires no client state hydration (e.g., historical scene cards), set `ScriptData: nil` so zero keys are emitted.

```go
type playerTemplate struct {
    ImagelessMode bool
    HeaderMeta    string
    HasHistory    bool
    HistoryScenes []templates.TemplateChunk
    CurrentScene  templates.TemplateChunk
}

type playerScript struct {
    bookId        string
    sensitivity   string
    imagelessMode bool
}
```

- **Zero Name-Translation Layer & Zero `tpl:"..."` Tags**:
  - Never add `tpl:"..."` struct tags or name-case conversion helpers (`pascalToSnake`, `snakeToCamelKey`, `TemplateToControllerName`).
  - Field names in `TemplateData` and `.tpl` match 1:1 (`{if ImagelessMode}`, `{HeaderMeta}`, `@HistoryScenes`).
  - Field names in `ScriptData` and `ctrl.data` match 1:1 (flat keys, zero nesting).

### 3.2 Rendering Chunks in `.tpl`
In `.tpl` files, `@FieldName` renders a `tpl.TemplateChunk` or `[]tpl.TemplateChunk` in its own isolated `RenderContext`:

```tpl
{if HasHistory}
    .history-stack#history-stack
        @HistoryScenes

.active-scene-stage#active-scene-stage
    @CurrentScene
```

- When `@CurrentScene` is evaluated, `tpl_render.go` resolves `playerTemplate.CurrentScene` (`TemplateChunk{TemplateData: &sceneTemplate{...}, ScriptData: &sceneScript{...}}`) and renders `scene_view.tpl` with `sceneTemplate` as its context root.
- When `@HistoryScenes` is evaluated, `tpl_render.go` loops over the `[]tpl.TemplateChunk` slice (`ScriptData: nil` on historical items) and renders each item's `TemplateData` in order.

### 3.3 FlatDataProtocol (`FDP` — `fdp.go` / `fdp.js`)
One unified, zero-nesting multi-frame codec (`Content-Type: text/x-fdp;charset=UTF-8`) handles both requests and responses (`1 frame = 1 controller`, separated by `\n---\n`):

```text
controller=PlayerController
target=#workspace
bookId=metro
sensitivity=medium
imagelessMode=false
_body=html

<section class="view-panel" id="player-view">...</section>
---
controller=SceneController
target=none
isPresent=true
---
controller=AppShellController
target=none
theme=dark
```

1. **Request (`Fdp.encode(params)`)**: Client sends `POST /api/action` with flat `key=val` lines + optional `_body=<key>\n\n<raw UTF-8>`.
2. **Response (`Fdp.decode(raw)`)**: Server responds with `1..N` controller-isolated `FDP` frames separated by `\n---\n`:
   - **Flat Header Lines (before `\n\n`)**: `controller=<Name>Controller` + `target` (`#app`, `#workspace`, `#modals-root`, `#editor-context-view`, or `none`) + flat `key=val` scalar fields from that controller's `chunk.ScriptData` + optional `_body=html`. Zero `X-Data` or custom `X-*` HTTP headers.
   - **Raw Body (after `\n\n`)**: Rendered HTML of the frame's `TemplateChunk.TemplateData` streamed directly from pooled `sync.Pool` buffers (`tpl.RenderTemplateTo`). `Fdp.decode` returns an array of decoded frames.

### 3.4 Automatic `FDP` Hydration (`chunk.ScriptData -> JS Controller.data`)
- `WriteFDPFrames` emits one frame per controller (`controller=<Name>Controller`), walking child `TemplateChunk`s in `TemplateData` and emitting `target=none` script frames for any child chunk with non-nil `ScriptData`.
- On the client, `Dispatcher.applyResponse` first hydrates `this.controllers[frame.controller].data` (`if (key in ctrl.data)`) for every frame, then swaps DOM targets for frames with `target != "none"`, and finally invokes `ctrl.onHydrate(frame, action, params)` exclusively on `this.controllers[frame.controller]`. Frame order does not matter.

---

## 4. Pure Presentational `.tpl` Rules

`.tpl` files are strictly for **visual structure and text presentation**:

1. **No Event or Controller Wiring in `.tpl`**:
   - Never write `event: scroll`, `event: wheel`, `action: ...`, `kind: ...`, or `param-*` in `.tpl`. All event bindings live in the controller's `bindings()` method.
2. **No `type: button` Boilerplate**:
   - `internal/tpl/tpl_render.go` automatically outputs `type="button"` on every `<button>` unless an explicit `type` (e.g., `type: submit`) is set.
3. **No State-Shuttling `data-*` Attributes**:
   - Never serialize scene/audio/view state onto DOM nodes via `data-scene-*` or `data-audio-*` just to read it back in JS with `getAttribute(...)`. State travels via `FDP` (`ScriptData`) into `this.data` or lives in the rendered DOM structure itself.
   - `data-hint` is permitted solely for declarative UI tooltips handled by `AppShellController`.

---

## 5. JS Controller Specification (`app/js/controllers/<name>_controller.js`)

Every JS controller MUST follow this exact shape and register itself with `Dispatcher.register`:

```js
"use strict";

const PlayerController = {
  // 1. DATA CONTRACT & LIVE STATE
  // Hydrated automatically from Go (FDP ScriptData) + holds local runtime state for this controller.
  data: {
    bookId: "",
    sensitivity: "medium",
    imagelessMode: false,
    isAutoAdvancing: false,
    overscrollAccum: 0,
    springResetTimer: null
  },

  // 2. DECLARATIVE DOM & LIFECYCLE BINDINGS
  // Key = CSS selector. Value = map of DOM event names (or "init") to handler functions.
  // Exempt from magic-string audit rules so selectors and backend action names remain readable in one place.
  bindings() {
    return {
      "#story-scroll-feed": {
        init: this.focusPresent,
        scroll: this.updateSceneView,
        wheel: this.handleSceneScroll
      },
      ".history-rail-tick": {
        click: this.focusTick
      }
    };
  },

  // 3. STATIC CONFIGURATION & CONSTANTS
  // camelCase keys only (no SCREAMING_SNAKE_CASE). Zero magic strings inside methods.
  settings: {
    maxPullPx: 84,
    pullExp: 1.45,
    cueMarginPx: 12,
    springResetMs: 160,
    advanceLockMs: 520,
    probeRatio: 0.35,
    wheelThresholds: { high: 180, medium: 320, low: 520, off: Infinity },
    selectors: {
      feed: "#story-scroll-feed",
      meta: "#reader-global-meta",
      metaText: "#reader-global-meta-text",
      histStack: "#history-stack",
      activeStage: "#active-scene-stage",
      railTicks: "#story-history-rail .history-rail-tick",
      bgStage: "#player-bg-stage",
      bgImg: "#player-bg-img",
      scrollCue: ".scroll-more-cue",
      confirmModal: "#modal-confirm"
    },
    classes: {
      overscroll: "is-overscrolling",
      active: "active",
      hidden: "hidden"
    },
    actions: {
      autoAdvance: "AutoAdvanceImplicit"
    }
  },

  // 4. CONTROLLER METHODS
  // Always invoked with `this` bound to the controller: fn(params, matchedEl, event).
  // Operate on `this.data` and `this.settings`.
  focusPresent(_params, feed) {
    // ...
  },

  focusTick(params) {
    // params.idx is pre-indexed once on DOM mount/swap (el._idx) and read in O(1) on click.
  },

  updateSceneView(_params, feed) {
    // ...
  },

  handleSceneScroll(_params, feed, e) {
    // ...
  }
};

Dispatcher.register("PlayerController", PlayerController);
```

### Mandatory Controller Rules
1. **Four Top-Level Sections Only (`JS_CONTROLLER_CONTRACT`)**:
   - `data`: plain object defining all synced (`ScriptData`) and local state fields with typed default values (`camelCase` keys only).
   - `bindings()`: method returning the selector-to-events map.
   - `settings`: static constants, thresholds, selectors, and CSS class names used by methods.
   - Methods (`camelCase`): all behavior, always reading/mutating `this.data` and reading `this.settings`.
2. **Zero Logic in `bindings()` — Local Method vs Pure Backend Action (`JS_BINDINGS_LOGIC`)**:
   - `bindings()` is strictly declarative and MUST contain **zero inline logic, zero arrow functions (`=>`), zero anonymous functions, and zero parameter-mapping callbacks**.
   - Every event handler value in `bindings()` must be one of two forms:
     - **Pure backend Go action**: `Dispatcher.action("ActionName")` (single action-name argument only), used when default form/`data-*` parameters are sent directly without transformation.
     - **Local controller method**: `this.<methodName>` (where `<methodName>` is defined on the same controller), required whenever parameter mapping, `p.idx` forwarding, DOM inspection, or local state updates are needed before calling `Dispatcher.post(...)`:
       ```js
       "article.scene-card button.choice-btn": {
         click: this.chooseBranch
       }
       ```
3. **1:1 `ScriptData` <-> `Controller.data` Mapping (`JS_CONTROLLER_DATA_MAP`)**:
   - Every field in a Go view's `<name>Script` struct (`internal/views/<name>_view.go`) must be declared 1:1 in the paired `<Name>Controller.data` (`app/js/controllers/<name>_controller.js`) with a matching default literal type (`bool` -> `true`/`false`, `int`/`float64` -> numeric literal, `string` -> string literal) so `Dispatcher.hydrate` (`key in ctrl.data`) and `Dispatcher.assignDataValue` coerce values accurately.
   - Every `this.data.<field>` and `<Name>Controller.data.<field>` access in JS must reference a key declared in that controller's `data` object.
4. **Pre-Indexed Sibling Index (`params.idx` in $O(1)$)**:
   - On initial boot and after every DOM swap (`swapTarget`), `Dispatcher.mountAll()` queries each bound selector **once** and assigns `el._idx = k` on every matched element.
   - When an event fires (`click`, `scroll`, `wheel`, etc.), `Dispatcher` reads `params.idx = matchedEl._idx` in $O(1)$ without calling `querySelectorAll` on the hot path.
   - Lists of buttons (`.history-rail-tick`, `.choice-btn`, `.contextual-rewind-btn`) never need `data-idx` or `param-idx` in `.tpl`.
5. **Universal `init` Lifecycle (No Manual `init*()` Calls in `swapTarget`)**:
   - Never add controller-specific boot calls into `Dispatcher.swapTarget` or `Dispatcher.boot`.
   - Declare `init: this.myInitMethod` on the root selector of the view in `bindings()`. On every DOM boot or swap, `Dispatcher.mountAll(root, skipInit)` **always** re-indexes `el._idx` across bound selectors, and (unless `keepScroll: true` sets `skipInit = true`) runs `myInitMethod.call(ctrl, { idx: el._idx }, el)` on every matched `init` element.
6. **Coding & Audit Constraints**:
   - **No strings in function bodies** (except 1–2 char separators/whitespace); put selectors, class names, and action names in `this.settings` — **except inside `bindings()`**, where selector keys and `Dispatcher.action("...")` strings are explicitly allowed and exempt from magic-string audits.
   - **No `SCREAMING_SNAKE_CASE`** anywhere in JS; always use `camelCase`.
   - **No pass-through functions** (such as `qs`/`qsa` wrappers).
   - **No dynamic HTML creation** (`document.createElement`, HTML string literals) in JS.

---

## 6. End-to-End Lifecycle Summary

1. **Initial Boot (`GET /` -> `Dispatcher.boot()`)**:
   - Server renders `app_shell.tpl` containing the active workspace view chunk (`library_view`, `player_view`, or `studio_view`).
   - Browser loads `/app.js`; `DOMContentLoaded` triggers `Dispatcher.boot()`.
   - `Dispatcher.register(...)` has already indexed all controller `bindings()` by event type (`click`, `scroll`, `wheel`, `keydown`, `init`, etc.).
   - `Dispatcher.mountAll()` pre-indexes `el._idx` for all bound selectors and runs every `init` binding whose selector exists in the initial DOM.
2. **User Interaction (`click` / `scroll` / `wheel` / `input`)**:
   - `Dispatcher.handle(e)` looks up `this.bindingsByEvent[e.type]` in $O(1)$.
   - Matches `e.target.closest(entry.selector)` and reads the pre-indexed `params.idx = matchedEl._idx` in $O(1)$ (zero `querySelectorAll` per event).
   - Calls `entry.handler.call(entry.ctrl, params, matchedEl, e)`.
   - If the handler is local (e.g., `PlayerController.updateSceneView`), it updates DOM classes/transforms at 120fps using `this.data` and `this.settings`.
   - If the handler is `Dispatcher.action("ChooseBranch", ...)`, it sends `POST /api/action` with an `FDP` payload (`Fdp.encode`).
3. **Server Response (`POST /api/action` -> `Dispatcher.applyResponse()`)**:
   - Go action mutates `state`, builds the `tpl.TemplateChunk` tree (`TemplateData` + `ScriptData`, e.g., `playerTemplate`/`playerScript` + child `sceneTemplate`/`sceneScript` chunks), and streams one unified `FDP` frame (`target=#workspace` + flat `key=val` lines from `ScriptData` + `_body=html\n\n` + rendered HTML from `TemplateData`).
   - `Fdp.decode(raw)` parses the flat `key=val` header map and extracts `decoded.html`.
   - `Dispatcher.hydrate(decoded)` updates matching declared keys (`key in ctrl.data`) on `PlayerController.data`, `SceneController.data`, and `AppShellController.data`.
   - `Dispatcher.swapTarget(decoded.target, decoded.html, decoded)` replaces `#workspace.innerHTML` and runs `Dispatcher.mountAll(root, keepScroll)` to re-index `el._idx` on all newly mounted elements and (when `!keepScroll`) invoke `init` handlers.
