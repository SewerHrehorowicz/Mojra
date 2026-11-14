# Mojra View–Template–Controller Architecture (`ARCH.md`)

Canonical specification for how state, views, templates, chunks, and client controllers are structured and how data travels from Go to the client. Both humans and AI agents MUST follow this architecture for every new feature and refactor.

---

## 1. Subsystem Separation of Concerns

```
[ state ]  -->  [ actions ]  -->  [ views ]          -->  [ tpl + fdp ]  ==(FDP / HTTP)==>  [ Dispatcher ]  -->  [ <Name>Controller ]
(in-memory)     (mutations &      (TemplateData +         (key=val +                        (Fdp.decode, DOM     (data, bindings(),
                 routing)          ScriptData)             \n\n HTML)                        swap, hydration)     settings, methods)
```

1. **`internal/state`**: Pure in-memory application state (`AppState`, `PlayerState`, `EditorState`, `Settings`). Zero HTTP, template, or view-formatting logic.
2. **`internal/actions`**: Handles `POST /api/action` requests decoded via `FDP` (`FlatDataProtocol`), mutates `state`, selects the target view chunk (`tpl.TemplateChunk`) and DOM target (`target`), and streams the `FDP` response frame.
3. **`internal/views`**: Transforms `state` into strongly-typed view chunks (`tpl.TemplateChunk`) with strict separation between presentational `TemplateData` (`*<name>Template`) and client controller state `ScriptData` (`*<name>Script`).
   - **Strict Separation Rule**: Each `view_<name>.go` file defines at most one template struct (`<name>Template`) for `.tpl` rendering and one optional script struct (`<name>Script`) for `FDP` controller hydration. Every view is a first-class peer `tpl.TemplateChunk` (there are no "sub-views" or helper view structs)—views compose other views solely via `tpl.TemplateChunk` or `[]tpl.TemplateChunk` fields on `TemplateData`, and reuse domain structs from `internal/book` directly.
4. **`internal/tpl` & `app/templates/*.tpl`**: Pure presentational HTML rendering from `TemplateData` + flat `FDP` key-value serialization exclusively from `ScriptData`.
5. **`app/js/fdp.js` (`Fdp`) & `app/js/dispatcher.js` (`Dispatcher`)**: Universal client codec and runtime. `Fdp` encodes/decodes flat `key=val\n...\n\n<body>` frames in both directions; `Dispatcher` owns `POST /api/action` transport, strict contract hydration (`key in ctrl.data`), `target` DOM swapping, delegated event routing, and `init` lifecycle mounting. There is **no `Core` module**.
6. **`app/js/controllers/<name>_controller.js` (`<Name>Controller`)**: Isolated UI controllers paired 1:1 with their view/template triad, stored in `app/js/controllers/`.

---

## 2. The 1:1 Triad Convention

Every interactive UI unit forms a strict 1:1:1 triad with matching names:

| Go View (`internal/views/`) | Constructor (`tpl.TemplateChunk`) | `TemplateData` / `ScriptData` | Template (`app/templates/`) | JS Controller (`app/js/controllers/`) | Registered Name |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `view_app_shell.go` | `ViewAppShell` | `appShellTemplate` / `appShellScript` | `app_shell.tpl` | `app_shell_controller.js` | `AppShellController` |
| `view_topbar.go` | `ViewTopbar` | `topbarTemplate` / `nil` | `topbar.tpl` | `topbar_controller.js` | `TopbarController` |
| `view_library.go` | `ViewLibrary` | `libraryTemplate` / `libraryScript` | `library_view.tpl` | `library_controller.js` | `LibraryController` |
| `view_player.go` | `ViewPlayer` | `playerTemplate` / `playerScript` | `player_view.tpl` | `player_controller.js` | `PlayerController` |
| `view_scene.go` | `ViewScene` | `sceneTemplate` / `sceneScript` | `scene_view.tpl` | `scene_controller.js` | `SceneController` |
| `view_audio_deck.go` | `ViewAudioDeck` | `audioDeckTemplate` / `audioDeckScript` | `audio_deck.tpl` | `audio_deck_controller.js` | `AudioDeckController` |
| `view_studio.go` | `ViewStudio` | `studioTemplate` / `studioScript` | `studio_view.tpl` | `studio_controller.js` | `StudioController` |
| `view_editor_context_panel.go` | `ViewEditorContextPanel` | `editorContextPanelTemplate` / `editorContextPanelScript` | `editor_context_panel.tpl` | `editor_context_panel_controller.js` | `EditorContextPanelController` |
| `view_editor_breakdown.go` | `ViewEditorBreakdown` | `editorBreakdownTemplate` / `nil` | `editor_breakdown_panel.tpl` | `editor_breakdown_controller.js` | `EditorBreakdownController` |
| `view_editor_variables.go` | `ViewEditorVariables` | `editorVariablesTemplate` / `nil` | `editor_variables_panel.tpl` | `editor_variables_controller.js` | `EditorVariablesController` |
| `view_editor_history.go` | `ViewEditorHistory` | `editorHistoryTemplate` / `nil` | `editor_history_panel.tpl` | `editor_history_controller.js` | `EditorHistoryController` |
| `view_text_editor.go` | `ViewTextEditor` | `textEditorTemplate` / `textEditorScript` | `text_editor.tpl` | `text_editor_controller.js` | `TextEditorController` |
| `view_modal_add_book.go` | `ViewModalAddBook` | `modalAddBookTemplate` / `nil` | `modal_add_book.tpl` | `modal_add_book_controller.js` | `ModalAddBookController` |
| `view_modal_confirm.go` | `ViewModalConfirm` | `modalConfirmTemplate` / `nil` | `confirm_modal` | `modal_confirm_controller.js` | `ModalConfirmController` |
| `view_modal_editor_help.go` | `ViewModalEditorHelp` | `modalEditorHelpTemplate` / `nil` | `modal_editor_help.tpl` | `modal_editor_help_controller.js` | `ModalEditorHelpController` |
| `view_modal_export.go` | `ViewModalExport` | `modalExportTemplate` / `nil` | `modal_export.tpl` | `modal_export_controller.js` | `ModalExportController` |
| `view_modal_new_book.go` | `ViewModalNewBook` | `modalNewBookTemplate` / `nil` | `modal_new_book.tpl` | `modal_new_book_controller.js` | `ModalNewBookController` |
| `view_modal_qr.go` | `ViewModalQR` | `modalQRTemplate` / `modalQRScript` | `modal_qr.tpl` | `modal_qr_controller.js` | `ModalQRController` |
| `view_modal_studio_projects.go` | `ViewModalStudioProjects` | `modalStudioProjectsTemplate` / `nil` | `modal_studio_projects.tpl` | `modal_studio_projects_controller.js` | `ModalStudioProjectsController` |

- **Isolation Rule**: A controller may only query or manipulate DOM elements belonging to its own template, or call public methods on another controller (e.g., `PlayerController` delegates `.scene-card` queries/focus to `SceneController`).

---

## 3. `TemplateChunk` Composition & FlatDataProtocol (`FDP`)

### 3.1 Building Chunks in Go (`internal/views/`)
Every view is an independent `tpl.TemplateChunk` splitting presentational data from controller state (with no `Template string` field):

```go
type TemplateChunk struct {
    TemplateData any // e.g. *playerTemplate -> used ONLY by the .tpl engine
    ScriptData   any // e.g. *playerScript   -> used ONLY by FDP -> ctrl.data
}
```

- **`TemplateData`**: Consumed exclusively by the `.tpl` HTML renderer. Holds presentational strings (`HeaderMeta`, `RailTitle`, `Name`, `Slugline`, `AudioDurationLabel`, `Prose`) and nested `tpl.TemplateChunk` or `[]tpl.TemplateChunk` child chunks. Never serialized into the `FDP` key-value header block.
- **`ScriptData`**: Consumed exclusively by the `FDP` serializer (`ScriptData` -> flat `key=val` lines -> `ctrl.data`). Holds only scalar runtime state required by client JS controllers. When a chunk requires no client state hydration (e.g., historical scene cards), set `ScriptData: nil` so zero keys are emitted.

```go
type playerTemplate struct {
    ImagelessMode bool
    HeaderMeta    string
    HasHistory    bool
    HistoryScenes []tpl.TemplateChunk
    CurrentScene  tpl.TemplateChunk
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
One unified, zero-nesting codec (`Content-Type: text/x-fdp;charset=UTF-8`) handles both requests and responses:

```text
target=#workspace
bookId=metro
sensitivity=medium
imagelessMode=false
_body=html

<div class="player-view">...</div>
```

1. **Request (`Fdp.encode(params)`)**: Client sends `POST /api/action` with flat `key=val` lines + optional `_body=<key>\n\n<raw UTF-8>`.
2. **Response (`Fdp.decode(raw)`)**: Server responds with a single `FDP` frame:
   - **Flat Header Lines (before `\n\n`)**: `target` (`#app`, `#workspace`, `#modals-root`, `#editor-context-panel`, or `none`) + flat `key=val` scalar fields collected from non-nil `chunk.ScriptData` across the `TemplateChunk` tree + `_body=html`. Zero `X-Data` or custom `X-*` HTTP headers.
   - **Raw Body (after `\n\n`)**: Rendered HTML of the root `TemplateChunk.TemplateData` streamed directly from pooled `sync.Pool` buffers (`tpl.RenderTemplateTo`). `Fdp.decode` strips the header lines before `\n\n` so only pure HTML enters `innerHTML`.

### 3.4 Automatic `FDP` Hydration (`chunk.ScriptData -> JS Controller.data`)
- The Go `FDP` encoder walks the `TemplateChunk` tree (traversing child `TemplateChunk` / `[]TemplateChunk` fields on `TemplateData`) and serializes **exclusively** `chunk.ScriptData` as flat `key=val` lines:
  - If `chunk.ScriptData == nil`, nothing is emitted for that chunk.
  - Zero nesting (`Controller.field` prefixes are not needed) and zero field-filtering heuristics (`key == "prose"` or `maxHeaderStrLen = 120` guards are eliminated because presentational strings live solely in `TemplateData`).
  - Zero case-conversion helpers: `ScriptData` fields are emitted 1:1 with their declared names.
- On the client, `Dispatcher.hydrate(decoded)` iterates over the flat `decoded` map from `Fdp.decode` and updates any registered controller whose `ctrl.data` declares that key (`if (key in ctrl.data)`) **before** swapping `decoded.html` into `decoded.target` or running `init` hooks. Presentational fields can never leak into `ctrl.data`.

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
   - Every field in a Go view's `<name>Script` struct (`internal/views/view_<name>.go`) must be declared 1:1 in the paired `<Name>Controller.data` (`app/js/controllers/<name>_controller.js`) with a matching default literal type (`bool` -> `true`/`false`, `int`/`float64` -> numeric literal, `string` -> string literal) so `Dispatcher.hydrate` (`key in ctrl.data`) and `Dispatcher.assignDataValue` coerce values accurately.
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
