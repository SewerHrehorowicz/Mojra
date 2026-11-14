//go:build js && wasm

package mojra

import "embed"

//go:embed app/templates/player_view.tpl app/templates/scene_view.tpl app/templates/audio_deck.tpl app/templates/scene_choice_arrow.tpl app/icons/reset.svg app/icons/asterisk.svg app/icons/play.svg app/icons/pause.svg app/icons/branch.svg
var WasmAppFS embed.FS
