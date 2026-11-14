//go:build prod_embed && !(js && wasm)

package mojra

import "embed"

//go:embed app/templates/*.tpl app/styles/*.tss app/js/*.js app/js/controllers/*.js app/icons/*.svg app/icons/favicon.ico app/icons/icon.png app/fonts/* build/app/index.html build/app/app.js build/app/style.css build/app/studio-atlas.png build/wasm/mojra.wasm.gz build/wasm/wasm_exec.js
var AppFS embed.FS
