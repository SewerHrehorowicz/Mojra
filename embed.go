//go:build !prod_embed

package storyteller

import "embed"

//go:embed app/templates/*.tpl app/styles/*.tss app/js/*.js app/icons/*.svg app/icons/favicon.ico app/icons/icon.png app/fonts/* all:books/nie-boj-sie-policji-rzecznej
var AppFS embed.FS
