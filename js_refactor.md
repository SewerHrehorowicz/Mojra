# Plan: Eliminate `Core`, Universal `X-Data` Sync, Controller Isolation (`AppShellController`, `PlayerController`, `SceneController`) & `bindings()` Audit Exemption

## 1. What Is There & What Are the Problems
1. **`app/js/core.js` (`Core`) Is a Junk Drawer**:
   - Mixes response transport/DOM swapping (`applyHeaders`, `swapTarget`, `boot`), app shell UI (`theme`, `ui-scale`, `zen`, `device-mode`, `toast`, `tooltip`, `fullscreen`, `Escape` modal, `native-cmd`), and forbidden 1-line pass-through helpers (`qs`, `qsa`).
2. **Scattered `X-*` Headers Instead of Generic Struct Sync**:
   - `internal/actions/actions_dispatch.go` sets 20+ ad-hoc `X-*` headers parsed manually on the client.
3. **Mixed Responsibility Between `Player` and `Scene`**:
   - `app/js/player.js` manipulates `.scene-card` internals (`.scene-portrait-img`, `.choice-btn`, `.scene-focused`, `.scene-dimmed`, card offsets) that belong to `SceneController`.
4. **Naming & Audit Rules**:
   - Controllers lack the `<name>_controller.js` (`<Name>Controller`) convention and `bindings()` method name; `internal/audit` does not yet exempt `bindings()` from magic-string checks.

---

## 2. Solutions
1. **Delete `app/js/core.js` Completely**:
   - Move `applyHeaders`, `swapTarget`, and `boot` into `Dispatcher` (`app/js/dispatcher.js`).
   - Extract shell/window concerns (`theme`, `uiScale`, `zenMode`, `deviceMode`, `toast`, `tooltip`, `fullscreen`, `nativeCmd`, `Escape` backdrop close) into `AppShellController` (`app/js/app_shell_controller.js` — 1:1 with `view_app_shell.go` and `app_shell.tpl`).
   - Replace `Core.qs` / `Core.qsa` with direct `document.querySelector` / `Array.from(root.querySelectorAll(...))`.
2. **Universal `X-Data` Header (Generic Go Struct -> JS `ctrl.data` Sync)**:
   - `internal/tpl/tpl_data_header.go`: `EncodeControllerDataHeader(chunk TemplateChunk) string` reflects over `chunk.Data` (and nested `TemplateChunk` / `[]TemplateChunk`), mapping `<name>_view` -> `<Name>Controller` and serializing scalar fields into `X-Data` (`PlayerController.imagelessMode=false;SceneController.isBranching=true;...`).
   - `Dispatcher.hydrate(headers)` parses `X-Data` and updates `this.controllers[ctrlName].data` 1:1.
3. **Strict Controller Isolation & `<name>_controller.js` Convention**:
   - `AppShellController` (`app/js/app_shell_controller.js`) <-> `app_shell.tpl` <-> `view_app_shell.go`.
   - `PlayerController` (`app/js/player_controller.js`) <-> `player_view.tpl` <-> `view_player.go`.
   - `SceneController` (`app/js/scene_controller.js`) <-> `scene_view.tpl` <-> `view_scene.go`.
4. **`bindings()` Audit Exemption**:
   - Exempt `bindings()` methods in `internal/audit/audit_arch.go` and `internal/audit/audit_magic.go`.

---

## 3. File-by-File & Method-by-Method Changes

### A. `internal/tpl/tpl_data_header.go` (New), `internal/actions/actions_dispatch.go` & `cmd/wasmplayer/main.go`
- **`internal/tpl/tpl_data_header.go`**:
  - `EncodeControllerDataHeader(chunk TemplateChunk) string`: converts template name (`app_shell` -> `AppShellController`, `player_view` -> `PlayerController`, `scene_view` -> `SceneController`), reflects scalar struct fields (`bool`, `int`, `float64`, `string`), and recurses into `TemplateChunk` / `[]TemplateChunk`.
- **`internal/actions/actions_dispatch.go` & `cmd/wasmplayer/main.go`**:
  - Set `X-Data: <EncodeControllerDataHeader(chunk)>` on action responses.

### B. `app/js/dispatcher.js` (Absorbs Transport & Lifecycle from `Core`)
- **`register(name, ctrl)`**: indexes `ctrl.bindings()` by event name.
- **`hydrate(headers)`**: parses `X-Data` and updates `this.controllers[ctrl].data` 1:1.
- **`applyResponse(resp, action, params, targetOverride)`**: runs `this.hydrate(resp.headers)`, notifies `AppShellController.syncHeaders(resp, action, params)`, and calls `this.swapTarget(...)`.
- **`swapTarget(targetSel, html, resp, action, params)`**: swaps DOM target, runs `this.mountAll()` when `!keepScroll`, and triggers active studio/editor hooks.
- **`boot()`**: binds global listeners (`this.bind()`) and runs `this.mountAll()`.

### C. `app/js/app_shell_controller.js` (Replaces `app/js/core.js`)
- **`AppShellController` (`Dispatcher.register("AppShellController", AppShellController)`)**:
  - `data`: `{ theme: "dark", uiScale: "small", imageless: false, imagelessLocked: false, zenMode: false, deviceMode: "full", lang: "pl", toast: "", nativeCmd: "" }`
  - `bindings()`:
    - `"body": { init: this.initShell, keydown: this.onKeydown, mouseover: this.onTooltipOver, mouseout: this.onTooltipOut, mousemove: this.onTooltipMove, mousedown: this.hideTooltip, wheel: this.hideTooltip }`
  - Methods: `initShell()`, `syncHeaders(resp, action, params)`, `showToast(msg)`, `onKeydown(params, el, e)`, `onTooltipOver`, `onTooltipOut`, `onTooltipMove`, `hideTooltip`.

### D. `app/js/scene_controller.js` (Renamed from `app/js/scene.js`)
- **`SceneController` (`Dispatcher.register("SceneController", SceneController)`)**:
  - `data`: `{ isPresent: true, isScreenplayMode: false, hasTitle: false, hasImage: false, imageUrl: "", isTerminal: false, isBranching: false, focusedIdx: 0, manualCard: null }`
  - `bindings()`:
    - `"article.scene-card button.contextual-rewind-btn": { click: Dispatcher.action("PromptRewindStep", (p) => ({ step: p.idx })) }`
    - `"article.scene-card button.choice-btn": { click: Dispatcher.action("ChooseBranch", (p) => ({ choice_idx: p.idx })) }`
    - `"article.scene-card button.choice-reset-btn": { click: Dispatcher.action("PromptResetProgress") }`
  - Methods (moved from `player.js`): `cards(root)`, `cardStartTop(feed, card, metaH)`, `hasImplicitChoice(root)`, `syncFocus(feed, probeTop, atBottom)` -> toggles `.scene-focused`/`.scene-dimmed` and returns `{ bestIdx, total, portraitUrl }`.

### E. `app/js/player_controller.js` (Renamed from `app/js/player.js`)
- **`PlayerController` (`Dispatcher.register("PlayerController", PlayerController)`)**:
  - `data`: `{ bookId: "", sensitivity: "medium", imagelessMode: false, isAutoAdvancing: false, overscrollAccum: 0, springResetTimer: null }`
  - `bindings()`:
    - `"#story-scroll-feed": { scroll: this.updateSceneView, wheel: this.handleSceneScroll, init: this.focusPresent }`
    - `".history-rail-tick": { click: this.focusTick }`
  - Methods: `wasmSaveKey`, `saveWasmProgress`, `bootWasm`, `applySpringPull`, `resetSpringPull`, `scheduleSpringBack`, `stepOverscroll`, `triggerImplicitNextAdvance`, `updateFeedScrollCue`, `updateSceneView`, `focusPresent`, `focusTick`, `handleSceneScroll` (delegating `.scene-card` operations to `SceneController`).

### F. Remaining `app/js/*.js` Modules & `internal/audit/audit_*.go`
- Replace `Core.qs` / `Core.qsa` / `Core.showToast` / `Core.swapTarget` / `Core.applyHeaders` across `app/js/*.js` with `document.querySelector` / `AppShellController.showToast` / `Dispatcher.swapTarget` / `Dispatcher.applyResponse`.
- In `internal/audit/audit_arch.go` & `internal/audit/audit_magic.go`: exempt `bindings()` methods from magic-string findings.