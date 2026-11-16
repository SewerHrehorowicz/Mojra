package license

import "embed"

//go:embed templates/*.tpl styles/*.tss controllers/*.js
var FS embed.FS
