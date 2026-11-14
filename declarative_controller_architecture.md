# Plan: Declarative Controller Architecture (`data`, `binding()`, `settings`) & Isolated `Player` / `Scene` Triads

## 1. What Is There & What Are the Problems
1. **Duplicated Scene Markup & Nested Structs in `player_view.tpl` / `view_player.go`**:
   - `internal/views/view_player.go` defined multiple structs (`playerView`, `sceneData`, `playerChoiceView`, `playerScreenplayBlockItem`) and nested `current_scene.*` / `history_items.*` inside `playerView`.
   - `app/templates/player_view.tpl` duplicated ~70 lines of `article.scene-card` markup across `{for h in history_items}` and `current_scene`, and manually wired `data-scene-*`, `event:`, `action:`, `kind:`, `type: button`, and `param-*` / `data-*` attributes.
2. **Scattered Controller State & Manual Lifecycle Hooks**:
   - `Player` stored mutable runtime properties (`manualFocusCard`, `activeBookId`, `isAutoAdvancingImplicit`, `implicitOverscrollAccum`, `implicitSpringResetTimer`) alongside `sel`, `cls`, `attr`, `cfg`, and `sensitivity`.
   - `Core.swapTarget` and `Core.boot` manually called `Player.initScrollAndAudio()`.

## 2. Solutions
1. **Strict 1:1 `View` – `Template` – `Controller` Isolation & Single Struct Per View**:
   - **`Player` Triad**:
     - `internal/views/view_player.go`: single `playerView` struct; composes `HistoryScenes` (`[]tpl.TemplateChunk`) and `CurrentScene` (`tpl.TemplateChunk`) without nesting scene fields.
     - `app/templates/player_view.tpl`: pure presentational layout delegating scene cards to `@history_scenes` and `@current_scene`.
     - `app/js/player.js`: `Player` controller (`data`, `binding()`, `settings`) managing only scroll feed, overscroll spring, and `.history-rail-tick` navigation.
   - **`Scene` Triad**:
     - `internal/views/view_scene.go`: single `sceneView` struct (`ViewSceneChunk`) using `[]book.ScreenplayBlock` and `[]book.Choice` directly.
     - `app/templates/scene_view.tpl`: pure presentational `article.scene-card` template used for both history and active scenes.
     - `app/js/scene.js`: `Scene` controller (`data`, `binding()`, `settings`) routing `.contextual-rewind-btn`, `.choice-btn`, and `.choice-reset-btn` actions via `Dispatcher.action(...)`.
2. **Zero Duplicated Server Data in JS**:
   - HTML over the wire (`player_view.tpl` + `scene_view.tpl`) is the single source of truth for scenes and choices; `Player.data` holds only client-side interactive UI/motion state (`bookId`, `sensitivity`, `imageless`, `manualCard`, `isAutoAdvancing`, `overscrollAccum`, `springResetTimer`).
3. **Universal Event & Lifecycle Dispatch in `Dispatcher` + Automatic `type="button"`**:
   - `Dispatcher.register(name, ctrl)` indexes `ctrl.binding()` by event type (`scroll`, `wheel`, `click`, `init`).
   - `Dispatcher.action(name, mapFn)` creates declarative backend action handlers with automatic sibling index `params.idx`.
   - `Dispatcher.mountAll()` runs all `init` bindings on boot and DOM swaps.
   - `internal/tpl/tpl_render.go` automatically emits `type="button"` on `<button>` elements when `type` is omitted.
