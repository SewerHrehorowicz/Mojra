//go:build prod_embed

package storyteller

import "embed"

//go:embed app/templates/*.tpl app/icons/*.svg app/icons/favicon.ico app/icons/icon.png app/fonts/* build/app/index.html build/app/app.js build/app/style.css build/app/studio-atlas.png books/nie-boj-sie-policji-rzecznej
var AppFS embed.FS
