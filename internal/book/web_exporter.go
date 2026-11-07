package book

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"storyteller/internal/tpl"
)

var (
	wasmCacheMu      sync.Mutex
	cachedBaseWasm   []byte
	cachedWasmExecJS string
)

// FindGoWasmExecJS locates wasm_exec.js inside GOROOT (checking both lib/wasm/wasm_exec.js and misc/wasm/wasm_exec.js).
func FindGoWasmExecJS() (string, error) {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return "", fmt.Errorf("nie można odczytać GOROOT: %w", err)
	}
	goroot := strings.TrimSpace(string(out))
	for _, rel := range []string{
		filepath.Join("lib", "wasm", "wasm_exec.js"),
		filepath.Join("misc", "wasm", "wasm_exec.js"),
	} {
		cand := filepath.Join(goroot, rel)
		if raw, err := os.ReadFile(cand); err == nil && len(raw) > 0 {
			return string(raw), nil
		}
	}
	return "", fmt.Errorf("nie znaleziono pliku wasm_exec.js w GOROOT (%s)", goroot)
}

// BuildBaseWasmAssets compiles build/wasm/mojra.wasm and copies build/wasm/wasm_exec.js inside rootDir.
func BuildBaseWasmAssets(rootDir string) ([]byte, string, error) {
	wasmDir := filepath.Join(rootDir, "build", "wasm")
	if err := os.MkdirAll(wasmDir, 0755); err != nil {
		return nil, "", err
	}
	wasmOutPath := filepath.Join(wasmDir, "mojra.wasm")
	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", wasmOutPath, "./cmd/wasmplayer")
	cmd.Dir = rootDir
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, "", fmt.Errorf("błąd kompilacji mojra.wasm: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	wasmBytes, err := os.ReadFile(wasmOutPath)
	if err != nil {
		return nil, "", err
	}
	wasmExecJS, err := FindGoWasmExecJS()
	if err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(filepath.Join(wasmDir, "wasm_exec.js"), []byte(wasmExecJS), 0644); err != nil {
		return nil, "", err
	}
	wasmCacheMu.Lock()
	cachedBaseWasm = wasmBytes
	cachedWasmExecJS = wasmExecJS
	wasmCacheMu.Unlock()
	return wasmBytes, wasmExecJS, nil
}

func resolveBaseWasmAndExecJS(rootDir string, embeddedFS fs.FS) ([]byte, string, error) {
	wasmCacheMu.Lock()
	if len(cachedBaseWasm) > 0 && cachedWasmExecJS != "" {
		w, j := cachedBaseWasm, cachedWasmExecJS
		wasmCacheMu.Unlock()
		return w, j, nil
	}
	wasmCacheMu.Unlock()

	if embeddedFS != nil {
		wRaw, wErr := fs.ReadFile(embeddedFS, "build/wasm/mojra.wasm")
		jRaw, jErr := fs.ReadFile(embeddedFS, "build/wasm/wasm_exec.js")
		if wErr == nil && jErr == nil && len(wRaw) > 8 && len(jRaw) > 0 {
			wasmCacheMu.Lock()
			cachedBaseWasm = wRaw
			cachedWasmExecJS = string(jRaw)
			wasmCacheMu.Unlock()
			return wRaw, string(jRaw), nil
		}
	}

	candidates := []string{rootDir}
	if wd, err := os.Getwd(); err == nil {
		cur := filepath.Clean(wd)
		for i := 0; i < 4; i++ {
			candidates = append(candidates, cur)
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
	}
	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		wPath := filepath.Join(cand, "build", "wasm", "mojra.wasm")
		jPath := filepath.Join(cand, "build", "wasm", "wasm_exec.js")
		wRaw, wErr := os.ReadFile(wPath)
		jRaw, jErr := os.ReadFile(jPath)
		if wErr == nil && jErr == nil && len(wRaw) > 8 && len(jRaw) > 0 {
			wasmCacheMu.Lock()
			cachedBaseWasm = wRaw
			cachedWasmExecJS = string(jRaw)
			wasmCacheMu.Unlock()
			return wRaw, string(jRaw), nil
		}
	}

	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(cand, "go.mod")); err == nil {
			return BuildBaseWasmAssets(cand)
		}
	}
	return nil, "", fmt.Errorf("brak szablonu runtime mojra.wasm")
}

// RenderWasmSplashHTML renders the minimalist Splash Screen with play.svg, title, and author (no #topbar).
func RenderWasmSplashHTML(b *Book, wasmSrc string, playSVG string) string {
	title := "Mojra"
	author := ""
	bookID := "book"
	imagelessAttr := "false"
	if b != nil {
		if strings.TrimSpace(b.Title) != "" {
			title = strings.TrimSpace(b.Title)
		}
		author = strings.TrimSpace(b.Author)
		if b.ID != "" {
			bookID = b.ID
		}
		if b.Imageless {
			imagelessAttr = "true"
		}
	}
	if strings.TrimSpace(wasmSrc) == "" {
		wasmSrc = "mojra.wasm"
	}
	return tpl.RenderWasmSplashPageHTML(title, author, bookID, imagelessAttr, wasmSrc, playSVG)
}

// ExportWebWASMApp exports a self-contained Web/WASM player directory (<exportDir>/<book.ID>-web/)
// with the book and its media compiled into encrypted, randomized STBC bytecode inside mojra.wasm.
func ExportWebWASMApp(b *Book, exportDir string, rootDir string, embeddedFS fs.FS) (string, error) {
	if b == nil {
		return "", fmt.Errorf("brak aktywnego projektu do eksportu")
	}
	baseWasm, wasmExecJS, err := resolveBaseWasmAndExecJS(rootDir, embeddedFS)
	if err != nil {
		return "", err
	}

	embeddedWasm, err := EmbedSTBCIntoWasm(baseWasm, b, nil)
	if err != nil {
		return "", err
	}

	outDir := filepath.Join(exportDir, SanitizeBookID(b.ID)+"-web")
	if err := os.MkdirAll(filepath.Join(outDir, "fonts"), 0755); err != nil {
		return "", err
	}

	if err := os.WriteFile(filepath.Join(outDir, "mojra.wasm"), embeddedWasm, 0644); err != nil {
		return "", err
	}

	cssStr, jsStr, playSVG, err := buildPlayerWebBundleAssets(rootDir, embeddedFS, wasmExecJS)
	if err != nil {
		return "", err
	}
	relCSS := strings.ReplaceAll(cssStr, `url("/fonts/`, `url("fonts/`)
	if err := os.WriteFile(filepath.Join(outDir, "style.css"), []byte(relCSS), 0644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(outDir, "app.js"), []byte(jsStr), 0644); err != nil {
		return "", err
	}

	indexHTML := RenderWasmSplashHTML(b, "mojra.wasm", playSVG)
	if err := os.WriteFile(filepath.Join(outDir, "index.html"), []byte(indexHTML), 0644); err != nil {
		return "", err
	}

	copyWebFontsAndIcons(outDir, rootDir, embeddedFS)
	return outDir, nil
}

func buildPlayerWebBundleAssets(rootDir string, embeddedFS fs.FS, wasmExecJS string) (cssStr, jsStr, playSVG string, err error) {
	appDir := filepath.Join(rootDir, "app")
	if _, statErr := os.Stat(filepath.Join(appDir, "styles", "player.tss")); statErr == nil {
		cssStr, err = tpl.BundleFilesToString(tpl.CSSPlayerModuleFiles(appDir), true)
		if err != nil {
			return "", "", "", err
		}
		var jsBuf bytes.Buffer
		jsBuf.WriteString(wasmExecJS)
		jsBuf.WriteString("\n;\n")
		for _, p := range tpl.JSPlayerModuleFiles(appDir) {
			raw, rErr := os.ReadFile(p)
			if rErr != nil {
				return "", "", "", rErr
			}
			jsBuf.WriteString(tpl.StripDevOnlyBlocks(string(raw)))
			jsBuf.WriteByte('\n')
		}
		jsStr = jsBuf.String()
		if rawSVG, sErr := os.ReadFile(filepath.Join(appDir, "icons", "play.svg")); sErr == nil {
			playSVG = strings.TrimSpace(string(rawSVG))
		}
		return cssStr, jsStr, playSVG, nil
	}

	if embeddedFS != nil {
		cssStr, err = tpl.BundlePlayerCSSFromFS(embeddedFS, true)
		if err != nil {
			return "", "", "", err
		}
		playerJS, jErr := tpl.BundlePlayerJSFromFS(embeddedFS, true)
		if jErr != nil {
			return "", "", "", jErr
		}
		jsStr = wasmExecJS + "\n;\n" + playerJS
		if rawSVG, sErr := fs.ReadFile(embeddedFS, "app/icons/play.svg"); sErr == nil {
			playSVG = strings.TrimSpace(string(rawSVG))
		}
		return cssStr, jsStr, playSVG, nil
	}
	return "", "", "", fmt.Errorf("brak zasobów app/ do zbudowania paczki Web")
}

func copyWebFontsAndIcons(outDir, rootDir string, embeddedFS fs.FS) {
	writeAsset := func(dstPath string, raw []byte) {
		if err := os.WriteFile(dstPath, raw, 0644); err != nil {
			return
		}
	}
	appDir := filepath.Join(rootDir, "app")
	if entries, err := os.ReadDir(filepath.Join(appDir, "fonts")); err == nil {
		for _, fe := range entries {
			if fe.IsDir() || !strings.HasSuffix(strings.ToLower(fe.Name()), ".ttf") {
				continue
			}
			if raw, rErr := os.ReadFile(filepath.Join(appDir, "fonts", fe.Name())); rErr == nil {
				writeAsset(filepath.Join(outDir, "fonts", fe.Name()), raw)
			}
		}
	} else if embeddedFS != nil {
		if entries, err := fs.ReadDir(embeddedFS, "app/fonts"); err == nil {
			for _, fe := range entries {
				if fe.IsDir() || !strings.HasSuffix(strings.ToLower(fe.Name()), ".ttf") {
					continue
				}
				if raw, rErr := fs.ReadFile(embeddedFS, "app/fonts/"+fe.Name()); rErr == nil {
					writeAsset(filepath.Join(outDir, "fonts", fe.Name()), raw)
				}
			}
		}
	}

	for _, iconFile := range []string{"favicon.ico", "icon.png"} {
		if raw, rErr := os.ReadFile(filepath.Join(appDir, "icons", iconFile)); rErr == nil {
			writeAsset(filepath.Join(outDir, iconFile), raw)
		} else if embeddedFS != nil {
			if raw, rErr := fs.ReadFile(embeddedFS, "app/icons/"+iconFile); rErr == nil {
				writeAsset(filepath.Join(outDir, iconFile), raw)
			}
		}
	}
}
