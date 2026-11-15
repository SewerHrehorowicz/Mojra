package studio

import "embed"

//go:embed templates/*.tpl ui_components/*.tpl styles/*.tss controllers/*.js scripts/* scripts/node_canvas/* studio-atlas.png
var FS embed.FS
