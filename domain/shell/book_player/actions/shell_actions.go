package actions

import (
	"io/fs"
	"net/url"
	"strconv"
	"strings"

	book "mojra/domain/book/actions"
	"mojra/domain/common"
	commonactions "mojra/domain/common/actions"
	commonstate "mojra/domain/common/state"
	commonviews "mojra/domain/common/views"
	_ "mojra/domain/i18n"
	"mojra/domain/player"
	_ "mojra/domain/player/actions"
	playerviews "mojra/domain/player/views"
	"mojra/domain/shell/book_player"
	"mojra/domain/shell/book_player/state"
	"mojra/domain/shell/book_player/views"
	"mojra/engine/osbridge"
	"mojra/engine/templates"
	"mojra/engine/transport"
)

const (
	actSaveAudioTime   = "SaveAudioTime"
	formKeyAction      = "action"
	formFieldAudioTime = "audio_time"
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

func wireBookPlayerHooks(shell *state.ShellState) {
	if shell == nil || shell.UI == nil {
		return
	}
	shell.UI.Hooks.RenderAppFrames = func() []transport.ActionFrame {
		return []transport.ActionFrame{
			{Target: commonstate.TargetApp, Chunks: []templates.TemplateChunk{views.ShellView(shell)}},
		}
	}
	shell.UI.Hooks.RenderWorkspaceFrames = func() []transport.ActionFrame {
		return []transport.ActionFrame{
			{Target: commonstate.TargetWorkspace, Chunks: []templates.TemplateChunk{playerviews.PlayerView(&shell.Player)}},
		}
	}
	shell.UI.Hooks.RenderModalsFrame = func() transport.ActionFrame {
		return transport.ActionFrame{
			Target: commonstate.TargetModalsRoot,
			Chunks: views.ActiveOverlayViews(shell),
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
	wireBookPlayerHooks(shell)
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
	wireBookPlayerHooks(shell)
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
	if act != actSaveAudioTime {
		if rawAudioTime := strings.TrimSpace(form.Get(formFieldAudioTime)); rawAudioTime != "" {
			if parsedTime, err := strconv.ParseFloat(rawAudioTime, 64); err == nil && parsedTime >= 0 {
				shell.Player.Save.AudioTime = parsedTime
			}
		}
	}

	var actionFrames []transport.ActionFrame
	if handler, ok := commonactions.Handlers[act]; ok && handler != nil {
		actionFrames = handler(shell.UI, &transport.ActionRequest{Form: form, IsLoopback: true})
	}

	shell.Player.SyncSaveHex()
	if shell.Storage != nil {
		_ = shell.Storage.WriteAtomic(shell.Player.Book.ID, []byte(shell.Player.SaveHex))
	}

	hasPlayerChunk := false
	hasUIChunk := false
	for _, af := range actionFrames {
		if af.Target == commonstate.TargetApp || af.Target == commonstate.TargetWorkspace {
			hasPlayerChunk = true
		}
		if af.Target == commonstate.TargetApp || af.Target == commonstate.TargetNone {
			hasUIChunk = true
		}
	}
	var globalScriptChunks []templates.TemplateChunk
	if !hasPlayerChunk {
		playerChunk := playerviews.PlayerView(&shell.Player)
		globalScriptChunks = append(globalScriptChunks, templates.TemplateChunk{ScriptData: playerChunk.ScriptData})
	}
	if !hasUIChunk {
		globalScriptChunks = append(globalScriptChunks, commonviews.UIScriptView(shell.UI))
	}

	lang := commonviews.LangOf(shell.UI)
	resp := transport.RenderActionFramesToFDP(lang, actionFrames, globalScriptChunks, shell.Templates)
	_, _ = shell.UI.ConsumeTransientFeedback()
	return resp
}
