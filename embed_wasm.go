//go:build js && wasm

package storyteller

import "embed"

//go:embed app/templates/player_view.tpl app/templates/scene_choice_arrow.tpl app/templates/modals.tpl app/icons/*.svg
var WasmAppFS embed.FS
