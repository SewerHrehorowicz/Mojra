package book_player

import "embed"

//go:embed modules.cfg templates/*.tpl styles/*.tss
var FS embed.FS
