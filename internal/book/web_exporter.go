//go:build !(js && wasm)

package book

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"mojra/internal/tpl"
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
	var gzBuf bytes.Buffer
	if gzW, gzErr := gzip.NewWriterLevel(&gzBuf, gzip.BestCompression); gzErr == nil {
		if _, wErr := gzW.Write(wasmBytes); wErr == nil && gzW.Close() == nil {
			if err := os.WriteFile(wasmOutPath+".gz", gzBuf.Bytes(), 0644); err != nil {
				return nil, "", err
			}
		}
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
		jRaw, jErr := fs.ReadFile(embeddedFS, "build/wasm/wasm_exec.js")
		if jErr == nil && len(jRaw) > 0 {
			if gzRaw, gzErr := fs.ReadFile(embeddedFS, "build/wasm/mojra.wasm.gz"); gzErr == nil && len(gzRaw) > 0 {
				if gr, rErr := gzip.NewReader(bytes.NewReader(gzRaw)); rErr == nil {
					wRaw, uErr := io.ReadAll(gr)
					_ = gr.Close()
					if uErr == nil && len(wRaw) > 8 {
						wasmCacheMu.Lock()
						cachedBaseWasm = wRaw
						cachedWasmExecJS = string(jRaw)
						wasmCacheMu.Unlock()
						return wRaw, string(jRaw), nil
					}
				}
			}
			if wRaw, wErr := fs.ReadFile(embeddedFS, "build/wasm/mojra.wasm"); wErr == nil && len(wRaw) > 8 {
				wasmCacheMu.Lock()
				cachedBaseWasm = wRaw
				cachedWasmExecJS = string(jRaw)
				wasmCacheMu.Unlock()
				return wRaw, string(jRaw), nil
			}
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

func renderWasmSplashHTMLWithCover(b *Book, coverSrc string, wasmSrc string, playSVG string) string {
	title := "Mojra"
	author := ""
	description := ""
	bookID := "book"
	imagelessAttr := "false"
	if b != nil {
		if strings.TrimSpace(b.Title) != "" {
			title = strings.TrimSpace(b.Title)
		}
		author = strings.TrimSpace(b.Author)
		description = strings.TrimSpace(b.Description)
		if b.ID != "" {
			bookID = b.ID
		}
		if b.Imageless {
			imagelessAttr = "true"
			coverSrc = ""
		}
	}
	if strings.TrimSpace(wasmSrc) == "" {
		wasmSrc = "mojra.wasm"
	}
	return tpl.RenderWasmSplashPageHTML(title, author, description, coverSrc, bookID, imagelessAttr, wasmSrc, playSVG)
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

	coverFilename := ""
	if !b.Imageless && b.CoverFile != "" {
		cleanCover := SanitizeRelativeMediaPath(b.CoverFile)
		if rawCover, cErr := readBookMediaRawBytes(b, cleanCover); cErr == nil && len(rawCover) > 0 {
			ext := strings.ToLower(filepath.Ext(cleanCover))
			if ext == "" {
				ext = ".jpg"
			}
			coverFilename = "cover" + ext
			if err := os.WriteFile(filepath.Join(outDir, coverFilename), rawCover, 0644); err != nil {
				return "", err
			}
		}
	}

	indexHTML := renderWasmSplashHTMLWithCover(b, coverFilename, "mojra.wasm", playSVG)
	if err := os.WriteFile(filepath.Join(outDir, "index.html"), []byte(indexHTML), 0644); err != nil {
		return "", err
	}

	copyWebFontsAndIcons(outDir, rootDir, embeddedFS)
	return outDir, nil
}

// EmbedDemoBookIntoWWW embeds the sample book into dist/www/mojra.wasm for the static showcase site.
func EmbedDemoBookIntoWWW(rootDir string, baseWasm []byte) error {
	if len(baseWasm) == 0 {
		var err error
		baseWasm, _, err = resolveBaseWasmAndExecJS(rootDir, nil)
		if err != nil {
			return err
		}
	}
	booksDir := filepath.Join(rootDir, "books")
	demoDir := filepath.Join(booksDir, "nie-boj-sie-policji-rzecznej")
	if !HasBookFilesInDir(demoDir) {
		entries, err := os.ReadDir(booksDir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				cand := filepath.Join(booksDir, e.Name())
				if HasBookFilesInDir(cand) {
					demoDir = cand
					break
				}
			}
		}
	}
	sampleBook, err := LoadBookFromDir(demoDir)
	if err != nil || sampleBook == nil {
		return fmt.Errorf("nie można wczytać książki demonstracyjnej z %s: %w", demoDir, err)
	}
	embeddedWasm, err := EmbedSTBCIntoWasm(baseWasm, sampleBook, nil)
	if err != nil {
		return err
	}
	wwwOutDir := filepath.Join(rootDir, "dist", "www")
	if err := os.MkdirAll(wwwOutDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(wwwOutDir, "mojra.wasm"), embeddedWasm, 0644)
}

func buildPlayerWebBundleAssets(rootDir string, embeddedFS fs.FS, wasmExecJS string) (cssStr, jsStr, playSVG string, err error) {
	appDir := filepath.Join(rootDir, "app")
	if _, statErr := os.Stat(filepath.Join(appDir, "styles", "player.tss")); statErr == nil {
		cssStr, err = tpl.BundleFilesToString(tpl.CSSPlayerModuleFiles(appDir), true)
		if err != nil {
			return "", "", "", err
		}
		playerJS, jErr := tpl.BundleFilesToString(tpl.JSPlayerModuleFiles(appDir), true)
		if jErr != nil {
			return "", "", "", jErr
		}
		var jsBuf bytes.Buffer
		jsBuf.WriteString(wasmExecJS)
		jsBuf.WriteString("\n;\n")
		jsBuf.WriteString(playerJS)
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

// CollectBookMediaBytes reads all validated audio and raster image files referenced by or present in b.
func CollectBookMediaBytes(b *Book) (map[string][]byte, error) {
	out := make(map[string][]byte)
	if b == nil {
		return out, nil
	}
	var totalBytes int64
	addFile := func(rel string) error {
		clean := SanitizeRelativeMediaPath(rel)
		if clean == "" {
			return nil
		}
		if _, exists := out[clean]; exists {
			return nil
		}
		if !b.Narrated && isAudioFilename(clean) {
			return nil
		}
		if !isAudioFilename(clean) && !isRasterImageFilename(clean) {
			return nil
		}
		data, err := readBookMediaRawBytes(b, clean)
		if err != nil {
			return nil
		}
		totalBytes += int64(len(data))
		if totalBytes > MaxZipTotalUncompressedBytes {
			return fmt.Errorf("łączny rozmiar multimediów przekracza limit (500 MB)")
		}
		out[clean] = data
		return nil
	}

	if b.CoverFile != "" {
		if err := addFile(b.CoverFile); err != nil {
			return nil, err
		}
	}
	for _, img := range b.ImageFiles {
		if err := addFile(img); err != nil {
			return nil, err
		}
	}
	if b.Narrated {
		for _, aud := range discoverBookAudioFilesForSections(b) {
			if err := addFile(aud); err != nil {
				return nil, err
			}
		}
	}
	for _, sc := range b.Scenes {
		if sc.ImageFile != "" {
			if err := addFile(sc.ImageFile); err != nil {
				return nil, err
			}
		}
		if b.Narrated {
			if sc.AudioFile != "" {
				if err := addFile(sc.AudioFile); err != nil {
					return nil, err
				}
			}
			for _, sec := range ExtractSceneVoiceSections(sc, b) {
				if sec.AudioFile != "" {
					if err := addFile(sec.AudioFile); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return out, nil
}

func readBookMediaRawBytes(b *Book, cleanRel string) ([]byte, error) {
	if b == nil || b.RootPath == "" {
		return nil, os.ErrNotExist
	}
	if !b.IsPacked {
		full := filepath.Join(b.RootPath, filepath.FromSlash(cleanRel))
		if !IsSafePathInsideRoot(b.RootPath, full) {
			return nil, os.ErrPermission
		}
		f, err := os.Open(full)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, maxMediaEntryBytes+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxMediaEntryBytes || !validateMediaHeader(cleanRel, data) {
			return nil, fmt.Errorf("nieprawidłowy plik multimedialny: %s", cleanRel)
		}
		return data, nil
	}

	zr, err := zip.OpenReader(b.RootPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	if f := findZipEntryByRelPath(zr.File, cleanRel); f != nil {
		var dummy int64
		return readValidatedZipMediaBytes(f, cleanRel, &dummy)
	}
	return nil, os.ErrNotExist
}

// CompileEncryptedSTBC compiles a Book and its media into a polymorphic STBC binary stream,
// encrypts it with a per-build random AES-256-GCM key, and signs it with Ed25519.
func CompileEncryptedSTBC(b *Book, media map[string][]byte) (ciphertext []byte, key [32]byte, nonce [12]byte, permSeed [16]byte, sig [64]byte, err error) {
	if b == nil {
		return nil, key, nonce, permSeed, sig, fmt.Errorf("brak książki do kompilacji STBC")
	}
	if media == nil {
		media, err = CollectBookMediaBytes(b)
		if err != nil {
			return nil, key, nonce, permSeed, sig, err
		}
	}

	if _, err = io.ReadFull(rand.Reader, key[:]); err != nil {
		return nil, key, nonce, permSeed, sig, err
	}
	if _, err = io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, key, nonce, permSeed, sig, err
	}
	if _, err = io.ReadFull(rand.Reader, permSeed[:]); err != nil {
		return nil, key, nonce, permSeed, sig, err
	}

	encOps, _ := stbcOpcodeTable(permSeed)
	var plain bytes.Buffer
	plain.WriteString(stbcHeaderMagic)

	plain.WriteByte(encOps[stbcOpBookMeta])
	writeSTBCString(&plain, b.ID)
	writeSTBCString(&plain, b.Title)
	writeSTBCString(&plain, b.Author)
	writeSTBCString(&plain, b.License)
	writeSTBCString(&plain, b.Description)
	writeSTBCString(&plain, b.NormalizedMode())
	writeSTBCString(&plain, b.NormalizedStandard())
	writeSTBCString(&plain, b.StartSceneID)
	writeSTBCString(&plain, b.CoverFile)
	var flags byte
	if b.Narrated {
		flags |= 1
	}
	if b.Imageless {
		flags |= 2
	}
	plain.WriteByte(flags)

	for _, ed := range b.Enums {
		plain.WriteByte(encOps[stbcOpEnumDef])
		writeSTBCString(&plain, ed.Name)
		_ = binary.Write(&plain, binary.LittleEndian, uint16(len(ed.Values)))
		for _, v := range ed.Values {
			writeSTBCString(&plain, v)
		}
	}

	for _, vd := range b.Variables {
		plain.WriteByte(encOps[stbcOpVarDef])
		writeSTBCString(&plain, vd.Name)
		writeSTBCString(&plain, vd.Kind)
		if vd.BoolVal {
			plain.WriteByte(1)
		} else {
			plain.WriteByte(0)
		}
		_ = binary.Write(&plain, binary.LittleEndian, int64(vd.IntVal))
		writeSTBCString(&plain, vd.EnumName)
		writeSTBCString(&plain, vd.EnumVal)
		_ = binary.Write(&plain, binary.LittleEndian, uint16(len(vd.Options)))
		for _, opt := range vd.Options {
			writeSTBCString(&plain, opt)
		}
	}

	for _, ch := range b.Chapters {
		plain.WriteByte(encOps[stbcOpChapter])
		writeSTBCString(&plain, ch.ID)
		writeSTBCString(&plain, ch.ExplicitID)
		writeSTBCString(&plain, ch.Name)
		writeSTBCString(&plain, ch.StartSceneID)
		_ = binary.Write(&plain, binary.LittleEndian, uint16(len(ch.SceneIDs)))
		for _, sid := range ch.SceneIDs {
			writeSTBCString(&plain, sid)
		}
	}

	for _, sc := range b.Scenes {
		plain.WriteByte(encOps[stbcOpScene])
		writeSTBCString(&plain, sc.ID)
		writeSTBCString(&plain, sc.ExplicitID)
		writeSTBCString(&plain, sc.Name)
		writeSTBCString(&plain, sc.Slugline)
		writeSTBCString(&plain, sc.Synopsis)
		writeSTBCString(&plain, sc.Effects)
		writeSTBCString(&plain, sc.Chapter)
		writeSTBCString(&plain, sc.ChapterID)
		writeSTBCString(&plain, SerializeSceneProseWithSectionVoices(sc, b))
		writeSTBCString(&plain, sc.AudioFile)
		writeSTBCString(&plain, sc.ImageFile)
		_ = binary.Write(&plain, binary.LittleEndian, int32(sc.OrderIndex))
		var scFlags byte
		if sc.ExplicitEnd {
			scFlags |= 1
		}
		hasTrim := b.Narrated && sc.HasVoiceTrim()
		if hasTrim {
			scFlags |= 2
		}
		plain.WriteByte(scFlags)
		if hasTrim {
			ts, te := sc.NormalizedVoiceTrim()
			_ = binary.Write(&plain, binary.LittleEndian, float32(ts))
			_ = binary.Write(&plain, binary.LittleEndian, float32(te))
		}
		_ = binary.Write(&plain, binary.LittleEndian, uint16(len(sc.Choices)))
		for _, ch := range sc.Choices {
			plain.WriteByte(encOps[stbcOpChoice])
			writeSTBCString(&plain, ch.Label)
			writeSTBCString(&plain, ch.Target)
			writeSTBCString(&plain, ch.TargetSceneID)
			writeSTBCString(&plain, ch.TargetChapterID)
			writeSTBCString(&plain, ch.Condition)
			writeSTBCString(&plain, ch.Effects)
			var cFlags byte
			if ch.ShowWhenLocked {
				cFlags |= 1
			}
			if ch.IsChapterLink {
				cFlags |= 2
			}
			if ch.IsImplicit {
				cFlags |= 4
			}
			if ch.IsEnding {
				cFlags |= 8
			}
			plain.WriteByte(cFlags)
		}
	}

	mediaKeys := make([]string, 0, len(media))
	for k := range media {
		mediaKeys = append(mediaKeys, k)
	}
	sort.Strings(mediaKeys)
	for _, rel := range mediaKeys {
		raw := media[rel]
		if len(raw) == 0 {
			continue
		}
		plain.WriteByte(encOps[stbcOpMedia])
		writeSTBCString(&plain, rel)
		_ = binary.Write(&plain, binary.LittleEndian, uint32(len(raw)))
		plain.Write(raw)
	}

	plain.WriteByte(encOps[stbcOpEnd])

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, key, nonce, permSeed, sig, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, key, nonce, permSeed, sig, err
	}
	ciphertext = gcm.Seal(nil, nonce[:], plain.Bytes(), permSeed[:])

	_, priv := stbcKeypair()
	msgHash := stbcSignatureDigest(ciphertext, key, nonce, permSeed)
	signature := ed25519.Sign(priv, msgHash[:])
	copy(sig[:], signature)
	return ciphertext, key, nonce, permSeed, sig, nil
}

// EmbedSTBCIntoWasm appends a Wasm custom section ("mojra_stbc") containing the encrypted STBC payload
// and the 148-byte cryptographic trailer directly into a compiled .wasm binary.
func EmbedSTBCIntoWasm(baseWasm []byte, b *Book, media map[string][]byte) ([]byte, error) {
	if len(baseWasm) < 8 || !bytes.Equal(baseWasm[:4], []byte("\x00asm")) {
		return nil, fmt.Errorf("nieprawidłowy nagłówek pliku .wasm")
	}
	baseWasm = StripEmbeddedSTBCFromWasm(baseWasm)

	ciphertext, key, nonce, permSeed, sig, err := CompileEncryptedSTBC(b, media)
	if err != nil {
		return nil, err
	}

	secName := "mojra_stbc"
	var secHeader bytes.Buffer
	writeULEB128(&secHeader, uint64(len(secName)))
	secHeader.WriteString(secName)

	secPayloadLen := secHeader.Len() + len(ciphertext) + stbcTrailerSize
	var customSec bytes.Buffer
	customSec.WriteByte(0x00)
	writeULEB128(&customSec, uint64(secPayloadLen))
	customSec.Write(secHeader.Bytes())

	payloadOffset := uint64(len(baseWasm) + customSec.Len())
	customSec.Write(ciphertext)

	var trailer [stbcTrailerSize]byte
	binary.LittleEndian.PutUint64(trailer[0:8], payloadOffset)
	binary.LittleEndian.PutUint64(trailer[8:16], uint64(len(ciphertext)))
	copy(trailer[16:48], key[:])
	copy(trailer[48:60], nonce[:])
	copy(trailer[60:76], permSeed[:])
	copy(trailer[76:140], sig[:])
	copy(trailer[140:148], stbcTrailerMagic)
	customSec.Write(trailer[:])

	out := make([]byte, 0, len(baseWasm)+customSec.Len())
	out = append(out, baseWasm...)
	out = append(out, customSec.Bytes()...)
	return out, nil
}

// StripEmbeddedSTBCFromWasm removes an existing embedded STBC custom section if present, returning the clean base .wasm.
func StripEmbeddedSTBCFromWasm(wasmBytes []byte) []byte {
	if len(wasmBytes) < 8+stbcTrailerSize {
		return wasmBytes
	}
	trailer := wasmBytes[len(wasmBytes)-stbcTrailerSize:]
	if string(trailer[140:148]) != stbcTrailerMagic {
		return wasmBytes
	}
	payloadOffset := binary.LittleEndian.Uint64(trailer[0:8])
	payloadLen := binary.LittleEndian.Uint64(trailer[8:16])
	endOffset := uint64(len(wasmBytes) - stbcTrailerSize)
	if payloadOffset < 20 || payloadOffset+payloadLen != endOffset {
		return wasmBytes
	}
	pos := 8
	for pos < len(wasmBytes) {
		secStart := pos
		secID := wasmBytes[pos]
		pos++
		secLen, n := readULEB128(wasmBytes[pos:])
		if n <= 0 {
			break
		}
		pos += n
		if secID == 0 && pos+int(secLen) == len(wasmBytes) {
			return wasmBytes[:secStart]
		}
		pos += int(secLen)
	}
	return wasmBytes
}

func writeULEB128(buf *bytes.Buffer, v uint64) {
	for step := 0; step < 10; step++ {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf.WriteByte(b)
		if v == 0 {
			break
		}
	}
}

func readULEB128(b []byte) (uint64, int) {
	var val uint64
	var shift uint
	for i := 0; i < len(b) && i < 10; i++ {
		byt := b[i]
		val |= uint64(byt&0x7f) << shift
		if (byt & 0x80) == 0 {
			return val, i + 1
		}
		shift += 7
	}
	return 0, 0
}

func writeSTBCString(buf *bytes.Buffer, s string) {
	raw := []byte(s)
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(raw)))
	buf.Write(raw)
}
