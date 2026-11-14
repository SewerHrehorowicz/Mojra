package tpl

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTranspileDirectoryToPlainHTML(t *testing.T) {
	dir := t.TempDir()
	templatesDir := filepath.Join(dir, "templates")
	iconsDir := filepath.Join(dir, "icons")
	_ = os.MkdirAll(templatesDir, 0755)
	_ = os.MkdirAll(iconsDir, 0755)

	outFile := filepath.Join(dir, "index.html")

	_ = os.WriteFile(filepath.Join(iconsDir, "play.svg"), []byte(`<svg viewBox="0 0 24 24"><polygon points="6,4 20,12 6,20"/></svg>`), 0644)

	_ = os.WriteFile(filepath.Join(templatesDir, "topbar.tpl"), []byte(`
header.topbar#main-top
    span.brand "Mojra"
    svg.icon src: /icons/play.svg
`), 0644)

	_ = os.WriteFile(filepath.Join(templatesDir, "index.tpl"), []byte(`
html lang: pl
    head
        meta charset: UTF-8
        title "Mojra"
    body
        #app
            @topbar
            label for: book-title "Tytuł książki"
            input#book-title
                type: text
                required: true
                placeholder: "Szepczący Las"
        script src: /app.js
`), 0644)

	if err := TranspileDirectoryToHTML(templatesDir, outFile); err != nil {
		t.Fatalf("TranspileDirectoryToHTML failed: %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	html := string(data)
	if !strings.Contains(html, `<header class="topbar" id="main-top">`) ||
		!strings.Contains(html, `<span class="brand">Mojra</span>`) ||
		!strings.Contains(html, `<svg class="icon" viewBox="0 0 24 24">`) ||
		!strings.Contains(html, `<input id="book-title" type="text" required="required" placeholder="Szepczący Las" />`) {
		t.Fatalf("unexpected HTML output:\n%s", html)
	}
}

func TestCompilerRejectsAllCodeInjectionAndInlineStyles(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantSub string
	}{
		{
			name:    "rejects inline style attribute",
			src:     `div.box style: "color:red;" "Test"`,
			wantSub: "inline CSS attribute",
		},
		{
			name:    "rejects style tag",
			src:     `style ".box { color: red; }"`,
			wantSub: "<style> tags are strictly forbidden",
		},
		{
			name:    "rejects inline event handler attribute",
			src:     `button.btn onclick: "evil()" "Klik"`,
			wantSub: "inline JS event handler",
		},
		{
			name:    "rejects inline script tag",
			src:     `script "evil()"`,
			wantSub: "inline JS inside <script> is strictly forbidden",
		},
		{
			name:    "rejects javascript url scheme",
			src:     `a href: "javascript:evil()" "Link"`,
			wantSub: "javascript:, vbscript:, or data: URL",
		},
		{
			name:    "rejects obfuscated entity javascript url",
			src:     `a href: "&#106;avascript:evil()" "Link"`,
			wantSub: "javascript:, vbscript:, or data: URL",
		},
		{
			name:    "rejects data url scheme",
			src:     `a href: "data:text/html,<script>evil()</script>" "Link"`,
			wantSub: "javascript:, vbscript:, or data: URL",
		},
		{
			name:    "rejects iframe tag",
			src:     `iframe src: "/test"`,
			wantSub: "<iframe> tags are strictly forbidden",
		},
		{
			name:    "rejects malformed tag injection",
			src:     `div<script> "Test"`,
			wantSub: "invalid or unsafe tag/component name",
		},
		{
			name:    "rejects external script src",
			src:     `script src: "https://evil.example/app.js"`,
			wantSub: "external or non-root script src",
		},
		{
			name:    "rejects inline svg tag",
			src:     ".btn\n    svg viewBox: \"0 0 24 24\"\n        circle cx: 12 cy: 12 r: 3",
			wantSub: "inline <svg> is strictly forbidden",
		},
		{
			name:    "rejects up/down reorder arrow button",
			src:     `button.btn "↑"`,
			wantSub: "up/down reorder arrow buttons",
		},
		{
			name:    "rejects dangling else without if",
			src:     "{else}\n    span \"Fallback\"",
			wantSub: "must immediately follow an 'if' or 'elif' block",
		},
		{
			name:    "rejects legacy arrow syntax",
			src:     "span.title -> book_title",
			wantSub: "'->' syntax is not supported",
		},
		{
			name:    "rejects legacy question-equals syntax",
			src:     "button.btn.active?=is_active \"Klik\"",
			wantSub: "'?=' syntax is not supported",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTemplateString(tc.src, "rejected_template")
			if err == nil {
				t.Fatalf("expected hard compile error for %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantSub, err)
			}
		})
	}
}

func TestBuildWorkspaceAppAssets(t *testing.T) {
	appDir := filepath.Clean(filepath.Join("..", "..", "app"))
	if _, err := os.Stat(filepath.Join(appDir, "templates", "index.tpl")); err != nil {
		t.Skip("workspace app/templates/index.tpl not found")
	}
	if err := BuildApp(appDir); err != nil {
		t.Fatalf("BuildApp on workspace appDir failed: %v", err)
	}

	outDir := filepath.Join(filepath.Dir(appDir), "build", "app")
	indexBytes, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatalf("read build/app/index.html: %v", err)
	}
	styleBytes, err := os.ReadFile(filepath.Join(outDir, "style.css"))
	if err != nil {
		t.Fatalf("read build/app/style.css: %v", err)
	}
	jsBytes, err := os.ReadFile(filepath.Join(outDir, "app.js"))
	if err != nil {
		t.Fatalf("read build/app/app.js: %v", err)
	}
	for _, polishRune := range []string{"Ą", "Ć", "Ę", "Ł", "Ń", "Ó", "Ś", "Ź", "Ż", "ą", "ć", "ę", "ł", "ń", "ó", "ś", "ź", "ż"} {
		for _, fontPrefix := range []string{"0|", "1|", "2|"} {
			token := fontPrefix + polishRune + "|"
			if !strings.Contains(string(jsBytes), token) {
				t.Fatalf("expected MSDF glyph metric %q in build/app/app.js", token)
			}
		}
	}

	for _, marker := range []string{"device-preview-bar", "device-stage", "device-frame", "SetDeviceMode", "Telefon (Pionowo)"} {
		if strings.Contains(string(indexBytes), marker) {
			t.Fatalf("expected %q to be stripped from production build/app/index.html", marker)
		}
	}
	for _, marker := range []string{".device-preview-bar", ".device-stage", ".device-frame", ".device-notch", "data-preview="} {
		if strings.Contains(string(styleBytes), marker) {
			t.Fatalf("expected %q to be stripped from production build/app/style.css", marker)
		}
	}
	if strings.Contains(string(styleBytes), ":root {\n    display: none") {
		t.Fatalf("expected @media child selectors not to collapse into :root { display: none } in build/app/style.css")
	}
	for _, wantMediaSel := range []string{
		"@media (max-width: 900px) {\n  .topbar {\n    padding: 6px;",
		".desktop-only-btn {\n    display: none !important;\n  }",
		".topbar-hamburger-btn,\n.top-action-btn.topbar-hamburger-btn {\n  display: none !important;\n}",
	} {
		if !strings.Contains(string(styleBytes), wantMediaSel) {
			t.Fatalf("expected %q in build/app/style.css", wantMediaSel)
		}
	}
	// Regression guard: never allow underlines (border-bottom or text-decoration: underline) on topbar tabs or icon buttons
	topbarTSSBytes, err := os.ReadFile(filepath.Join(appDir, "styles", "topbar.tss"))
	if err != nil {
		t.Fatalf("read topbar.tss: %v", err)
	}
	if strings.Count(string(topbarTSSBytes), "border-bottom") != 1 || strings.Contains(string(topbarTSSBytes), "underline") {
		t.Fatalf("regression in topbar.tss: only .topbar itself may have border-bottom; tabs and icons must never have underlines")
	}
	for _, marker := range []string{"device-preview-bar", "SetDeviceMode", "X-Device-Mode", "#device-stage"} {
		if strings.Contains(string(jsBytes), marker) {
			t.Fatalf("expected %q to be stripped from production build/app/app.js", marker)
		}
	}

	// Regression guard for library_view.tpl:
	// - Inside article.book-card, no buttons except the cover play button (.book-cover-wrap)
	// - Top-level #btn-open-add-book for importing books
	// - No percentage progress text (.book-progress-text or "{b.progress_pct}%")
	libTplBytes, err := os.ReadFile(filepath.Join(appDir, "templates", "library_view.tpl"))
	if err != nil {
		t.Fatalf("read library_view.tpl: %v", err)
	}
	libTpl := string(libTplBytes)
	if !strings.Contains(libTpl, "#btn-open-add-book") || !strings.Contains(libTpl, "button.book-cover-wrap") {
		t.Fatalf("regression in library_view.tpl: expected #btn-open-add-book and button.book-cover-wrap, got:\n%s", libTpl)
	}
	cardStart := strings.Index(libTpl, "article.book-card")
	cardBlock := ""
	if cardStart >= 0 {
		cardBlock = libTpl[cardStart:]
		if footerIdx := strings.Index(cardBlock, ".home-mode-footer"); footerIdx > 0 {
			cardBlock = cardBlock[:footerIdx]
		}
	}
	if cardStart < 0 || strings.Count(cardBlock, "button.") != 1 {
		t.Fatalf("regression in library_view.tpl: expected only 1 button inside article.book-card (button.book-cover-wrap), got:\n%s", libTpl)
	}
	for _, forbiddenLibToken := range []string{
		"book-progress-text",
		"progress_pct}%",
		"book-card-actions",
		"book-card-footer",
		"book-bookmark-pill",
		"library-toolbar",
	} {
		if strings.Contains(libTpl, forbiddenLibToken) {
			t.Fatalf("regression in library_view.tpl: forbidden token %q must not be present", forbiddenLibToken)
		}
	}

	// Verify BuildPlayerOnlyApp omits editor and library assets
	playerOutDir := t.TempDir()
	if err := BuildPlayerOnlyApp(appDir, playerOutDir); err != nil {
		t.Fatalf("BuildPlayerOnlyApp failed: %v", err)
	}
	playerJSBytes, err := os.ReadFile(filepath.Join(playerOutDir, "app.js"))
	if err != nil {
		t.Fatalf("read player-only app.js: %v", err)
	}
	playerCSSBytes, err := os.ReadFile(filepath.Join(playerOutDir, "style.css"))
	if err != nil {
		t.Fatalf("read player-only style.css: %v", err)
	}
	if !strings.Contains(string(playerJSBytes), "focusPresent") {
		t.Fatalf("expected focusPresent in player-only app.js")
	}
	for _, forbiddenEditorJS := range []string{"const StudioView =", "const EditorWires =", "EDITOR_GRID_SIZE"} {
		if strings.Contains(string(playerJSBytes), forbiddenEditorJS) {
			t.Fatalf("expected %q to be omitted from player-only app.js", forbiddenEditorJS)
		}
	}
	for _, forbiddenEditorCSS := range []string{".timeline-node-card", ".bookshelf-grid"} {
		if strings.Contains(string(playerCSSBytes), forbiddenEditorCSS) {
			t.Fatalf("expected %q to be omitted from player-only style.css", forbiddenEditorCSS)
		}
	}
}

func TestMultilineAttributesWithMapping(t *testing.T) {
	src := `
.custom-select-menu
    {for img_opt in image_options}
        button.custom-select-option .active?: {img_opt.is_active}
            type: button
            data-select-value: "{img_opt.value}"
            data-select-label: "{img_opt.label}"
            {img_opt.label}
`
	parsed, err := ParseTemplateString(src, "multiline_test")
	if err != nil {
		t.Fatalf("ParseTemplateString failed: %v", err)
	}
	components := map[string]*ParsedTemplate{"multiline_test": parsed}
	ctx := NewContext(map[string]any{
		"image_options": []map[string]any{
			{
				"is_active": true,
				"value":     "forest.png",
				"label":     "forest.png",
			},
		},
	})
	html := RenderTemplate("multiline_test", components, ctx)
	want := `<button class="custom-select-option active" type="button" data-select-value="forest.png" data-select-label="forest.png">forest.png</button>`
	if !strings.Contains(html, want) {
		t.Fatalf("expected %q in rendered HTML, got:\n%s", want, html)
	}
}

func TestTranspileTSSTemplateSyntax(t *testing.T) {
	src := `
switch theme
  dark: :root, body[data-theme="dark"]
  light: body[data-theme="light"]

switch preview
  full: .device-stage[data-preview="full"]
  portrait: .device-stage[data-preview="portrait"]
  landscape: .device-stage[data-preview="landscape"]

$bg-obsidian -> switch(theme) #0 | #f

$border-strong -> switch(theme)
  dark: #52
  light: #9

$font-sans: "Lato"

@font-face
  font-family: "Lato"
  src: url("/fonts/Lato-Regular.ttf") format("truetype")
  font-weight: 400
  font-style: normal
  font-display: block

.preview-mode-btn:hover
  position: {variable}
  color: rgb(128, 0, 0)
  before: "interpolated string {variable}"

.thing
  background: $bg-obsidian
  width: $custom-width(100%)
  padding -> switch(preview)
    full: 0
    portrait, landscape: 18px
  -> switch(preview)
    portrait
      max-width: 393px
      border: 1px solid $border-strong
`
	ctx := NewContext(map[string]any{
		"variable": "relative",
	})

	css, err := TranspileCSSString(src, "test_styles", ctx)
	if err != nil {
		t.Fatalf("TranspileCSSString failed: %v", err)
	}

	checks := []string{
		":root,\nbody[data-theme=\"dark\"] {\n  --bg-obsidian: #000000;\n  --border-strong: #525252;\n  --font-sans: \"Lato\";\n}",
		"body[data-theme=\"light\"] {\n  --bg-obsidian: #ffffff;\n  --border-strong: #999999;\n}",
		"@font-face {\n  font-family: \"Lato\";\n  src: url(\"/fonts/Lato-Regular.ttf\") format(\"truetype\");\n  font-weight: 400;\n  font-style: normal;\n  font-display: block;\n}",
		".preview-mode-btn:hover {\n  position: relative;\n  color: rgb(128, 0, 0);\n}",
		".preview-mode-btn:hover::before {\n  content: \"interpolated string relative\";\n}",
		".thing {\n  background: var(--bg-obsidian);\n  width: var(--custom-width, 100%);\n}",
		".device-stage[data-preview=\"full\"] .thing {\n  padding: 0;\n}",
		".device-stage[data-preview=\"portrait\"] .thing,\n.device-stage[data-preview=\"landscape\"] .thing {\n  padding: 18px;\n}",
		".device-stage[data-preview=\"portrait\"] .thing {\n  max-width: 393px;\n  border: 1px solid var(--border-strong);\n}",
	}

	for _, want := range checks {
		if !strings.Contains(css, want) {
			t.Fatalf("expected CSS output to contain:\n%s\n\ngot full CSS:\n%s", want, css)
		}
	}
}

func TestTranspileCSSAlignedTSSSyntax(t *testing.T) {
	src := `
:root, body[data-theme="dark"]
  color-scheme dark

body[data-theme="light"]
  color-scheme light

@switch preview
  full      .device-stage[data-preview="full"]
  portrait  .device-stage[data-preview="portrait"]
  landscape .device-stage[data-preview="landscape"]

:root
  --bg-obsidian light-dark(#fff, #000)
  --border-strong light-dark(#999999, #525252)

.device-stage
  width 100vw
  height 100vh
  display flex
  background --bg-obsidian
  padding switch(preview)
    portrait, landscape 18px

.device-notch::after
  content ""
  width 48px
  height 3px
  background --border-strong

.device-frame
  position relative
  width --custom-width(100%)

  @switch preview
    full
      flex 1
    portrait, landscape
      border 1px solid --border-strong
      margin auto
`
	css, err := TranspileCSSString(src, "css_aligned_test", nil)
	if err != nil {
		t.Fatalf("TranspileCSSString failed: %v", err)
	}

	checks := []string{
		":root,\nbody[data-theme=\"dark\"] {\n  color-scheme: dark;\n}",
		"body[data-theme=\"light\"] {\n  color-scheme: light;\n}",
		":root {\n  --bg-obsidian: light-dark(#fff, #000);\n  --border-strong: light-dark(#999999, #525252);\n}",
		".device-stage {\n  width: 100vw;\n  height: 100vh;\n  display: flex;\n  background: var(--bg-obsidian);\n}",
		".device-stage[data-preview=\"portrait\"],\n.device-stage[data-preview=\"landscape\"] {\n  padding: 18px;\n}",
		".device-notch::after {\n  content: \"\";\n  width: 48px;\n  height: 3px;\n  background: var(--border-strong);\n}",
		".device-frame {\n  position: relative;\n  width: var(--custom-width, 100%);\n}",
		".device-stage[data-preview=\"full\"] .device-frame {\n  flex: 1;\n}",
		".device-stage[data-preview=\"portrait\"] .device-frame,\n.device-stage[data-preview=\"landscape\"] .device-frame {\n  border: 1px solid var(--border-strong);\n  margin: auto;\n}",
	}

	for _, want := range checks {
		if !strings.Contains(css, want) {
			t.Fatalf("expected CSS output to contain:\n%s\n\ngot full CSS:\n%s", want, css)
		}
	}
}

func TestTranspileTSSMultilinePropertiesAndFunctions(t *testing.T) {
	src := `
.btn
  transition
    border-color 0.15s ease
    background 0.15s ease
    color 0.15s ease
    opacity 0.15s ease
  background
    linear-gradient(to right)
      currentColor 5px
      transparent 5px
      transparent calc(100% - 5px)
      currentColor calc(100% - 5px)
    0 0 / 100% 1px no-repeat
  box-shadow
    inset 0 0 0 2px --bg-obsidian
    inset 0 0 0 4px --text-primary
  transform
    translate3d(0, -2px, 0)
    scale(1.02)
  filter
    blur(8px)
    brightness(0.9)
  clip-path
    polygon()
      0% 0%
      100% 0%
      100% 100%
  grid-template-areas
    "topbar topbar"
    "canvas inspector"

.multi-grad
  background
    radial-gradient(circle at 50% 0%)
      --accent-glow 0%
      transparent 70%
    linear-gradient(180deg)
      --bg-elevated 0%
      --bg-obsidian 100%
  mask-image linear-gradient(to bottom)
    transparent 0%
    black 100%
  box-shadow switch(theme)
    dark
      0 6px 18px rgba(0, 0, 0, 0.45)
      0 2px 6px rgba(0, 0, 0, 0.3)
    light
      0 2px 8px rgba(0, 0, 0, 0.06)
`
	css, err := TranspileCSSString(src, "multiline_props_test", nil)
	if err != nil {
		t.Fatalf("TranspileCSSString failed: %v", err)
	}

	checks := []string{
		"transition: border-color 0.15s ease, background 0.15s ease, color 0.15s ease, opacity 0.15s ease;",
		"background: linear-gradient(to right, currentColor 5px, transparent 5px, transparent calc(100% - 5px), currentColor calc(100% - 5px)) 0 0 / 100% 1px no-repeat;",
		"box-shadow: inset 0 0 0 2px var(--bg-obsidian), inset 0 0 0 4px var(--text-primary);",
		"transform: translate3d(0, -2px, 0) scale(1.02);",
		"filter: blur(8px) brightness(0.9);",
		"clip-path: polygon(0% 0%, 100% 0%, 100% 100%);",
		"grid-template-areas: \"topbar topbar\" \"canvas inspector\";",
		"background: radial-gradient(circle at 50% 0%, var(--accent-glow) 0%, transparent 70%), linear-gradient(180deg, var(--bg-elevated) 0%, var(--bg-obsidian) 100%);",
		"mask-image: linear-gradient(to bottom, transparent 0%, black 100%);",
		"box-shadow: 0 6px 18px rgba(0, 0, 0, 0.45), 0 2px 6px rgba(0, 0, 0, 0.3);",
		"box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);",
	}

	for _, want := range checks {
		if !strings.Contains(css, want) {
			t.Fatalf("expected CSS output to contain:\n%s\n\ngot full CSS:\n%s", want, css)
		}
	}
}

func TestNoSecondaryInterpolationOnDynamicContextValues(t *testing.T) {
	src := `
div.card
    h2.title {book_title}
    p.sub "Autor: {book_author}"
`
	parsed, err := ParseTemplateString(src, "sec_interp")
	if err != nil {
		t.Fatalf("ParseTemplateString failed: %v", err)
	}
	components := map[string]*ParsedTemplate{"sec_interp": parsed}
	ctx := NewContext(map[string]any{
		"book_title":          "Księga {secret_internal_var}",
		"book_author":         "Jan {secret_internal_var}",
		"secret_internal_var": "TOP_SECRET_LEAK",
	})
	html := RenderTemplate("sec_interp", components, ctx)
	if strings.Contains(html, "TOP_SECRET_LEAK") {
		t.Fatalf("dynamic context value underwent secondary template interpolation:\n%s", html)
	}
	if !strings.Contains(html, `<h2 class="title">Księga {secret_internal_var}</h2>`) {
		t.Fatalf("expected literal '{secret_internal_var}' in rendered title, got:\n%s", html)
	}
	if !strings.Contains(html, `<p class="sub">Autor: Jan {secret_internal_var}</p>`) {
		t.Fatalf("expected literal '{secret_internal_var}' in rendered author, got:\n%s", html)
	}
}

func TestConditionalClassesAttrsElifElseAndComponentSlots(t *testing.T) {
	cardCompSrc := `
.panel-card .is-highlighted?: {active}
    header.panel-hdr {title}
    .panel-body
        @slot
`
	pageSrc := `
.page
    button.tab-btn#main-btn .active?: {is_active} .is-locked?: {is_locked} .mode-full?: {mode == "full"}
        disabled?: {is_locked}
        data-hint?: {hint_text}
        {is_active ? "Aktywny" : "Nieaktywny"}
    {if view == "library"}
        span.view-tag "Biblioteka"
    {elif view == "editor"}
        span.view-tag "Edytor"
    {else}
        span.view-tag "Odtwarzacz"
    @panel_card title: "Karta Sceny" active: {is_active}
        ul.items
            {for idx, item in items}
                li.item .first?: {idx == 0} "{idx}: {item.name}"
`
	cardComp, err := ParseTemplateString(cardCompSrc, "panel_card")
	if err != nil {
		t.Fatalf("ParseTemplateString(panel_card) failed: %v", err)
	}
	pageTpl, err := ParseTemplateString(pageSrc, "page")
	if err != nil {
		t.Fatalf("ParseTemplateString(page) failed: %v", err)
	}
	components := map[string]*ParsedTemplate{
		"panel_card": cardComp,
		"page":       pageTpl,
	}

	ctx := NewContext(map[string]any{
		"is_active": true,
		"is_locked": true,
		"mode":      "full",
		"hint_text": "Podpowiedź",
		"view":      "editor",
		"items": []map[string]any{
			{"name": "Prolog"},
			{"name": "Rozdział 1"},
		},
	})
	html := RenderTemplate("page", components, ctx)
	for _, want := range []string{
		`<button class="tab-btn active is-locked mode-full" id="main-btn" type="button" disabled="disabled" data-hint="Podpowiedź">Aktywny</button>`,
		`<span class="view-tag">Edytor</span>`,
		`<div class="panel-card is-highlighted">`,
		`<header class="panel-hdr">Karta Sceny</header>`,
		`<li class="item first">0: Prolog</li>`,
		`<li class="item">1: Rozdział 1</li>`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected %q in rendered output, got:\n%s", want, html)
		}
	}
}

func TestNoHTMLStringLiteralsInClientJS(t *testing.T) {
	appDir := filepath.Clean(filepath.Join("..", "..", "app"))
	jsFiles := JSModuleFiles(appDir)
	forbidden := []string{"<div", "<span", "<button", "<input", "<label", "<svg", "</", "createElement("}
	for _, path := range jsFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		src := string(raw)
		for _, tok := range forbidden {
			if strings.Contains(src, tok) {
				t.Fatalf("forbidden dynamic markup token %q found in %s; render all HTML via server .tpl templates", tok, filepath.Base(path))
			}
		}
	}
}

func BenchmarkRenderTemplateAppShell(b *testing.B) {
	appDir := filepath.Clean(filepath.Join("..", "..", "app"))
	templates, err := LoadTemplatesFromDir(filepath.Join(appDir, "templates"), nil)
	if err != nil {
		b.Skipf("skipping benchmark: %v", err)
	}
	ctx := NewContext(map[string]any{
		"theme":                  "dark",
		"imageless_mode":         false,
		"imageless_locked":       false,
		"imageless_locked_attr":  "false",
		"imageless_toggle_title": "Wyłącz obrazy",
		"ui_scale":               "small",
		"is_large_ui":            false,
		"ui_scale_toggle_title":  "Powiększ",
		"is_view_player":         true,
		"has_active_book":        true,
		"active_book_title":      "Nie bój się policji rzecznej",
		"show_reset_progress":    true,
		"workspace_view": TemplateChunk{
			Template: "player_view",
			TemplateData: map[string]any{
				"ImagelessMode": false,
				"HeaderMeta":    "Nie bój się policji rzecznej • Rozdział 1",
				"RailTitle":     "Wstyd w warsztacie",
				"HasBgImage":    true,
				"BgImageURL":    "/api/books/test/media/images/sc1.png",
				"ShowAudioDeck": true,
				"AudioDeck": TemplateChunk{
					Template: "audio_deck",
					TemplateData: map[string]any{
						"AudioURL":           "/api/books/test/media/voice/sc1.ogg",
						"AudioDurationLabel": "1:20",
						"AudioTrimStart":     "0.0000",
						"AudioTrimEnd":       "1.0000",
						"AudioResumeTime":    "0.0",
						"AudioSpeed":         "1",
						"AutoChoiceEnabled":  true,
						"AutoChoiceLabel":    "Auto-Wybór: WŁ",
						"AutoChoiceDelay":    "2",
					},
				},
				"CurrentScene": TemplateChunk{
					Template: "scene_view",
					TemplateData: map[string]any{
						"IsPresent":   true,
						"Name":        "Wstyd w warsztacie",
						"RailTitle":   "Wstyd w warsztacie",
						"HasTitle":    true,
						"Prose":       "Krótki akapit tekstu sceny testowej.",
						"HasImage":    true,
						"ImageURL":    "/api/books/test/media/images/sc1.png",
						"IsTerminal":  false,
						"IsBranching": true,
						"Choices": []map[string]any{
							{"Label": "Wejdź do warsztatu", "ShowWhenLocked": false, "IsEnding": false},
							{"Label": "Odejdź w stronę rzeki", "ShowWhenLocked": false, "IsEnding": false},
						},
					},
				},
			},
		},
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderTemplateTo(io.Discard, "app_shell", templates, ctx)
	}
}

func RenderTemplate(tplName string, components map[string]*ParsedTemplate, ctx *RenderContext) string {
	buf := AcquireBuffer()
	defer ReleaseBuffer(buf)
	_ = RenderTemplateTo(buf, tplName, components, ctx)
	return buf.String()
}

func TranspileCSSString(source, name string, ctx *RenderContext) (string, error) {
	switches := DefaultCSSSwitches()
	return transpileCSSWithSwitches(source, name, ctx, switches, false)
}

func BuildPlayerOnlyApp(appDir, outDir string) error {
	if outDir == "" {
		outDir = filepath.Join(filepath.Dir(filepath.Clean(appDir)), "build", "app")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	templates, err := LoadTemplatesFromDir(filepath.Join(appDir, "templates"), nil)
	if err != nil {
		return err
	}
	delete(templates, "library_view")
	delete(templates, "studio_view")
	delete(templates, "editor_context_panel")
	delete(templates, "editor_history_panel")
	delete(templates, "editor_breakdown_panel")
	delete(templates, "editor_variables_panel")
	StripDevNodesFromTemplates(templates)
	idxTpl := templates["index"]
	htmlStr := TranspileStaticHTML(idxTpl, templates)
	if err := os.WriteFile(filepath.Join(outDir, "index.html"), []byte(htmlStr), 0644); err != nil {
		return err
	}
	if err := BundleCSSFiles(CSSPlayerModuleFiles(appDir), filepath.Join(outDir, "style.css")); err != nil {
		return err
	}
	return BundleJSFiles(JSPlayerModuleFiles(appDir), filepath.Join(outDir, "app.js"))
}




