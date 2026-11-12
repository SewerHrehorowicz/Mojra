# Plan: Generator zasobów `.tpl`/`.tss` (`internal/generators`) + Natywny Splash Screen

## 1. Co jest obecnie i jakie są problemy
- `cmd/deck/main.go` zawiera zaszytą logikę kompilacji `.tpl` + stylów `app` i `www` oraz wykrywanie headless Chromium (`FindHeadlessPDFBrowser`) wyłącznie pod PDF.
- Brak reużywalnego modułu do podglądu szablonów `.tpl` (łączących style `app/styles/*.tss` i `www/style.tss`) w przeglądarce oraz rasteryzacji ich do PNG (`--screenshot`).
- Okno natywne (`internal/osbridge/window_*.go`) pokazuje się dopiero po starcie serwera HTTP i załadowaniu książek, bez natywnej bitmapy splash.

## 2. Rozwiązanie
- Wydzielenie pakietu `internal/generators` obsługującego:
  1. Bundlowanie szablonów `.tpl` ze stylami `app/styles/*.tss` + `www/style.tss` + lokalnym `.tss` i fontami/ikonami.
  2. Serwer podglądu na żywo w przeglądarce (`ServePreview`).
  3. Rasteryzację do `.png` (`RenderPNG` przez `--headless=new --screenshot --window-size=WxH`) oraz eksport `.pdf` (`RenderPDF`, współdzielony z `cmd/deck` — zero duplikacji).
- Dodanie szablonu `generators/splash/splash.tpl` + `splash.tss`, komendy CLI `cmd/gen` (lub subkomendy `mojra gen`) generującej `app/icons/splash.png` (`//go:embed` w `embed.go`) oraz natywnego okna splash w `internal/osbridge/`.

## 3. Zmiany plik po pliku i metoda po metodzie

### `internal/generators/browser.go` (przeniesienie z `cmd/deck/main.go`)
- `FindHeadlessBrowser() string`: przeniesione i przemianowane z `cmd/deck.FindHeadlessPDFBrowser`.
- `RenderPNG(browserBin, htmlPath, outPNGPath string, width, height int) error`: uruchamia headless Chromium z `--screenshot=<outPNGPath>` i `--window-size=<W>,<H> --hide-scrollbars`.
- `RenderPDF(browserBin, htmlPath, outPDFPath string) error`: przeniesione z `cmd/deck.renderHTMLFileToPDF`.

### `internal/generators/bundle.go` (nowy plik)
- `Spec struct`: pola `Name`, `TplDir`, `TplName`, `IncludeAppCSS bool`, `IncludeWWWCSS bool`, `ExtraTSS []string`, `Width int`, `Height int`, `OutPNG string`.
- `BuildStaticBundle(rootDir string, spec Spec, ctx *tpl.RenderContext) (htmlPath string, err error)`: kompiluje `.tpl` przez `tpl.LoadTemplatesFromDir` + `tpl.TranspileStaticHTMLWithContext` i łączy `tpl.CSSModuleFiles(appDir)` + `www/style.tss` + `spec.ExtraTSS` przez `tpl.BundleCSSFiles`.

### `internal/generators/preview.go` (nowy plik)
- `ServePreview(rootDir string, spec Spec, port int) error`: serwuje skompilowany bundle, fonty (`/fonts/`) i ikony (`/icons/`) z odświeżaniem mtime bez cachowania przy edycji `.tpl`/`.tss`.

### `generators/splash/splash.tpl` i `generators/splash/splash.tss` (nowe pliki)
- Szablon splasha wykorzystujący tokeny z `app/styles/colors.tss`, typografię z `www/style.tss` oraz `svg src: /icons/mojra.svg`.

### `cmd/deck/main.go` (refaktor)
- Usunięcie zduplikowanego `FindHeadlessPDFBrowser` i `renderHTMLFileToPDF` na rzecz `generators.FindHeadlessBrowser` i `generators.RenderPDF`.

### `cmd/mojra/main.go` + `embed.go` + `internal/osbridge/window_*.go`
- `cmd/mojra/main.go`: dodanie subkomendy `gen` (`mojra gen splash --preview` / `mojra gen splash --render` -> zapisuje `app/icons/splash.png`).
- `embed.go`: dodanie `//go:embed app/icons/splash.png` (`SplashPNG []byte`).
- `internal/osbridge/window_darwin.go` i `window_windows.go`: wyświetlenie bezramkowego okna z bitmapą `SplashPNG` na starcie i zamknięcie go w `didFinishNavigation` / `NavigationCompleted`.
- `docs/project.graph`: aktualizacja grafu projektu przez `go run ./cmd/projgraph`.