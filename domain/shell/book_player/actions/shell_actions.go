package actions

import (
	"bytes"
	"io/fs"
	"net/url"
	"strings"

	book "mojra/domain/book/actions"
	"mojra/domain/common"
	commonstate "mojra/domain/common/state"
	commonviews "mojra/domain/common/views"
	_ "mojra/domain/i18n"
	"mojra/domain/player"
	playeractions "mojra/domain/player/actions"
	playerviews "mojra/domain/player/views"
	"mojra/domain/shell/book_player"
	"mojra/domain/shell/book_player/state"
	"mojra/domain/shell/book_player/views"
	"mojra/engine"
	"mojra/engine/osbridge"
	"mojra/engine/templates"
	"mojra/engine/transport"
)

const (
	actToggleTheme          = "ToggleTheme"
	actToggleUIScale        = "ToggleUIScale"
	actSetUIScale           = "SetUIScale"
	actToggleImageless      = "ToggleImageless"
	actToggleLanguage       = "ToggleLanguage"
	actSetLanguage          = "SetLanguage"
	actSetScrollSensitivity = "SetScrollSensitivity"

	formKeyAction      = "action"
	formKeyScale       = "scale"
	formKeyLang        = "lang"
	formKeySensitivity = "sensitivity"
	formKeyLevel       = "level"
	themeLight         = "light"
	themeDark          = "dark"
	scaleSmall         = "small"
	scaleLarge         = "large"
	mimeWebP           = "image/webp"
	mimePNG            = "image/png"
	mimeJPEG           = "image/jpeg"
	mimeFLAC           = "audio/flac"
	mimeOGG            = "audio/ogg"
	mimeMP3            = "audio/mpeg"
	mimeOctet          = "application/octet-stream"
	extWebP            = ".webp"
	extPNG             = ".png"
	extJPG             = ".jpg"
	extJPEG            = ".jpeg"
	extFLAC            = ".flac"
	extOGG             = ".ogg"
	extMP3             = ".mp3"
)

func init() {
	book.BookPlayerModules = ModuleFSList()
}

// ModuleFSList returns the ordered []fs.FS list declared in book_player/modules.cfg plus the shell itself.
func ModuleFSList() []fs.FS {
	registry := map[string]fs.FS{
		"common": common.FS,
		"player": player.FS,
	}
	resolved, _, err := templates.ResolveModuleFSList(book_player.FS, registry)
	if err != nil || len(resolved) == 0 {
		return []fs.FS{common.FS, player.FS, book_player.FS}
	}
	return resolved
}

// ResolveMediaMimeType returns the MIME type for a book media asset path.
func ResolveMediaMimeType(relPath string) string {
	lower := strings.ToLower(relPath)
	switch {
	case strings.HasSuffix(lower, extWebP):
		return mimeWebP
	case strings.HasSuffix(lower, extPNG):
		return mimePNG
	case strings.HasSuffix(lower, extJPG), strings.HasSuffix(lower, extJPEG):
		return mimeJPEG
	case strings.HasSuffix(lower, extFLAC):
		return mimeFLAC
	case strings.HasSuffix(lower, extOGG):
		return mimeOGG
	case strings.HasSuffix(lower, extMP3):
		return mimeMP3
	default:
		return mimeOctet
	}
}

// ResolveBookMediaURLs populates CoverURL, ImageURL, and AudioURL on loadedBook using storage.CreateBlobURL.
func ResolveBookMediaURLs(loadedBook *book.Book, media map[string][]byte, storage *osbridge.WasmStorage) {
	if loadedBook == nil || storage == nil {
		return
	}
	resolveURL := func(relPath string) string {
		clean := strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(relPath), "\\", "/"), "/")
		if clean == "" {
			return ""
		}
		rawBytes, ok := media[clean]
		if !ok {
			for k, v := range media {
				if strings.EqualFold(k, clean) {
					rawBytes = v
					clean = k
					ok = true
					break
				}
			}
		}
		if !ok || len(rawBytes) == 0 {
			return ""
		}
		return storage.CreateBlobURL(clean, rawBytes, ResolveMediaMimeType(clean))
	}

	if loadedBook.CoverFile != "" {
		loadedBook.CoverURL = resolveURL(loadedBook.CoverFile)
	}
	for _, sc := range loadedBook.Scenes {
		if sc.ImageFile != "" {
			sc.ImageURL = resolveURL(sc.ImageFile)
		}
		if loadedBook.Narrated {
			if sc.AudioFile != "" {
				sc.AudioURL = resolveURL(sc.AudioFile)
			}
			for idx := range sc.VoiceSections {
				vs := &sc.VoiceSections[idx]
				if vs.AudioFile != "" {
					vs.AudioURL = resolveURL(vs.AudioFile)
				}
			}
		}
	}
}

// BootFromWasmBytes extracts and decrypts the STBC container from a .wasm binary and initializes ShellState.
func BootFromWasmBytes(wasmBytes []byte, savedHex string, storage *osbridge.WasmStorage) (*state.ShellState, string, error) {
	loadedBook, media, err := book.ExtractSTBCFromWasm(wasmBytes)
	if err != nil {
		return nil, "", err
	}
	if storage == nil {
		storage = osbridge.NewWasmStorage()
	}
	ResolveBookMediaURLs(loadedBook, media, storage)

	parsedTemplates, tplErr := templates.LoadTemplatesFromModules(ModuleFSList())
	if tplErr != nil {
		return nil, "", tplErr
	}

	shell := state.NewShellState(loadedBook, media, savedHex, parsedTemplates, storage)
	frames := []transport.ActionFrame{
		{Target: commonstate.TargetApp, Chunks: []templates.TemplateChunk{views.ShellView(shell)}},
	}
	lang := commonviews.LangOf(shell.UI)
	resp := transport.RenderActionFramesToFDP(lang, frames, nil, parsedTemplates)
	return shell, resp, nil
}

// DispatchAction processes an FDP action request against ShellState and returns a multi-frame FDP response string.
func DispatchAction(shell *state.ShellState, rawPayload string) string {
	if shell == nil || shell.Player.Book == nil || shell.Player.Save == nil {
		return ""
	}
	form := make(url.Values, 16)
	rawBytes := []byte(rawPayload)
	if frames := transport.DecodeFDPFrames(rawBytes); len(frames) > 0 {
		form = frames[0]
	} else if !transport.DecodeFDPBytes(rawBytes, form) {
		if parsed, err := url.ParseQuery(rawPayload); err == nil {
			form = parsed
		}
	}
	act := strings.TrimSpace(form.Get(formKeyAction))
	target := commonstate.TargetNone
	toast := ""

	switch act {
	case actToggleTheme:
		if shell.UI.Settings.Theme == themeLight {
			shell.UI.Settings.Theme = themeDark
		} else {
			shell.UI.Settings.Theme = themeLight
		}
	case actToggleUIScale, actSetUIScale:
		reqScale := strings.ToLower(strings.TrimSpace(form.Get(formKeyScale)))
		if reqScale == scaleLarge || reqScale == scaleSmall {
			shell.UI.Settings.UIScale = reqScale
		} else if shell.UI.Settings.UIScale == scaleLarge {
			shell.UI.Settings.UIScale = scaleSmall
		} else {
			shell.UI.Settings.UIScale = scaleLarge
		}
	case actToggleImageless:
		if !shell.Player.Book.IsImageless() {
			shell.UI.Settings.Imageless = !shell.UI.Settings.Imageless
		}
		target = commonstate.TargetWorkspace
	case actToggleLanguage, actSetLanguage:
		reqLang := strings.TrimSpace(form.Get(formKeyLang))
		if reqLang != "" {
			shell.UI.Settings.Language = engine.NormalizeLangCode(reqLang)
		} else {
			next := engine.NextLang(commonviews.LangOf(shell.UI))
			shell.UI.Settings.Language = engine.SupportedLanguages[next].Code
		}
		target = commonstate.TargetApp
	case actSetScrollSensitivity:
		rawSens := strings.TrimSpace(form.Get(formKeySensitivity))
		if rawSens == "" {
			rawSens = strings.TrimSpace(form.Get(formKeyLevel))
		}
		shell.UI.Settings.ScrollSensitivity = book.NormalizeScrollSensitivity(rawSens)
	default:
		target, toast = playeractions.DispatchPlayerStandaloneAction(&shell.Player, act, form)
	}

	shell.UI.Overlays.Toast = toast
	shell.Player.SyncSaveHex()
	if shell.Storage != nil {
		_ = shell.Storage.WriteAtomic(shell.Player.Book.ID, []byte(shell.Player.SaveHex))
	}

	var chunk templates.TemplateChunk
	switch target {
	case commonstate.TargetApp:
		chunk = views.ShellView(shell)
	case commonstate.TargetWorkspace:
		chunk = playerviews.PlayerView(&shell.Player)
	case commonstate.TargetModalsRoot:
		chunks := views.ActiveOverlayViews(shell)
		if len(chunks) > 0 {
			chunk = chunks[0]
		}
	default:
		chunk = playerviews.PlayerView(&shell.Player)
		chunk.TemplateData = nil
	}

	var body []byte
	lang := commonviews.LangOf(shell.UI)
	if target != commonstate.TargetNone && chunk.TemplateData != nil && shell.Templates != nil {
		tplName, tplData := chunk.Resolve()
		if tplName != "" {
			buf := templates.AcquireBuffer()
			ctx := engine.NewViewContext(lang, tplData)
			_ = templates.RenderTemplateTo(buf, tplName, shell.Templates, ctx)
			body = append([]byte(nil), buf.Bytes()...)
			templates.ReleaseBuffer(buf)
		}
	}

	var out bytes.Buffer
	_ = transport.WriteFDPFrames(&out,
		transport.FDPFrame{Target: target, HTMLBody: body, Chunk: chunk},
		transport.FDPFrame{Target: commonstate.TargetNone, Chunk: commonviews.UIScriptView(shell.UI)},
	)
	_, _ = shell.UI.ConsumeTransientFeedback()
	return out.String()
}
