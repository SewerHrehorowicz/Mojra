package studio_app

import "embed"

//go:embed modules.cfg templates/*.tpl styles/*.tss controllers/*.js
var FS embed.FS
