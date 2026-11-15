//go:build !(js && wasm)

package actions

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"mojra/domain/book"
	"mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/engine"
)

const (
	formKeyLicense           = "license"
	formKeyDescription       = "description"
	formKeyStartScene        = "start_scene"
	formKeyStartChapter      = "start_chapter"
	formKeyBookTitle         = "book_title"
	formKeyPanX              = "pan_x"
	formKeyPanY              = "pan_y"
	formKeyZoom              = "zoom"
	formKeyPositions         = "positions"
	formKeyWireKey           = "wire_key"
	formKeyWirePins          = "wire_pins"
	formKeySearchQuery       = "q"
	historyScopeMeta         = "meta"
	historyScopeLayout       = "layout"
	coalesceSaveBookSettings = "savebooksettings"
	coalesceRenameBook       = "renamebook"
	coalesceSetStartChapter  = "setstartchapter"
	coalesceNodeDragBatch    = "nodedrag:batch"
	coalesceNodeDragPref     = "nodedrag:"
	coalesceWirePinPref      = "wirepin:"
	coalesceExportDir        = "exportdir"
)

var _ = registerActions(map[string]ActionHandler{
	"OpenEditorHelpModal":  openEditorHelpModal,
	"CloseEditorHelpModal": closeEditorHelpModal,
	"CloseBookSettings":    closeBookSettings,
	"ToggleBookSettings":   toggleBookSettings,
	"SaveBookSettings":     saveBookSettings,
	"SaveEditorState":      saveEditorViewportAndNodePositions,
	"RenameBook":           renameBook,
	"ToggleBookNarrated":   toggleBookNarrated,
	"ToggleBookImageless":  toggleBookImageless,
	"SetStartScene":        setStartScene,
	"SetBookStartChapter":  setBookStartChapter,
	"AutoLayoutNodes":      autoLayoutNodes,
	"BrowseBookExportFolder": browseBookExportFolder,
	"BrowseBookSaveFolder": browseBookSaveFolder,
	"ToggleStudioSearch":   toggleStudioSearchAction,
	"CloseStudioSearch":    closeStudioSearchAction,
	"FilterStudioSearch":   filterStudioSearchAction,
	"ToggleBookWarnings":   toggleBookWarningsAction,
})

func activateRightPanel(s *state.AppState, panel state.EditorRightPanel) {
	s.Editor.ActivateRightPanel(panel)
	if panel != state.PanelNone && s.Editor.SceneText.WidthMode == state.WidthModeFull {
		s.Editor.SceneText.WidthMode = state.WidthModeHalf
	}
}

func openEditorHelpModal(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	if s.Editor.RightPanel == state.PanelHelp {
		s.Editor.ActivateRightPanel(state.PanelNone)
	} else {
		activateRightPanel(s, state.PanelHelp)
	}
	return frameWorkspace(s, "")
}

func closeEditorHelpModal(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	if s.Editor.RightPanel == state.PanelHelp {
		s.Editor.ActivateRightPanel(state.PanelNone)
	}
	return frameWorkspace(s, "")
}

func closeBookSettings(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	if s.Editor.RightPanel == state.PanelBook || s.Editor.RightPanel == state.PanelChapter {
		s.Editor.ActivateRightPanel(state.PanelNone)
	} else {
		s.Editor.SelectedChapter = ""
	}
	return frameWorkspace(s, "")
}

func toggleBookSettings(s *state.AppState, r *http.Request) []engine.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	open := s.Editor.RightPanel != state.PanelBook
	switch r.FormValue(formKeyOpen) {
	case formValTrue:
		open = true
	case formValFalse:
		open = false
	}
	if open {
		activateRightPanel(s, state.PanelBook)
	} else if s.Editor.RightPanel == state.PanelBook {
		s.Editor.ActivateRightPanel(state.PanelNone)
	}
	return frameWorkspace(s, "")
}

func saveBookSettings(s *state.AppState, r *http.Request) []engine.ActionFrame {
	if isHistoryPreview(s) {
		return frameApp(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	toast := ""
	b := s.Editor.ActiveBook
	if b != nil && !b.IsPacked {
		changed := false
		newTitle := book.SanitizePlainText(r.FormValue(newBookFormTitle))
		if newTitle != "" && newTitle != b.Title {
			b.Title = newTitle
			changed = true
		}
		if _, hasAuthor := r.Form[newBookFormAuthor]; hasAuthor {
			newAuthor := book.SanitizePlainText(r.FormValue(newBookFormAuthor))
			if newAuthor != "" && newAuthor != b.Author {
				b.Author = newAuthor
				changed = true
			}
		}
		if _, hasLic := r.Form[formKeyLicense]; hasLic {
			newLic := book.SanitizePlainText(r.FormValue(formKeyLicense))
			if newLic != b.License {
				b.License = newLic
				changed = true
			}
		}
		if _, hasDesc := r.Form[formKeyDescription]; hasDesc {
			newDesc := book.SanitizePlainText(r.FormValue(formKeyDescription))
			if newDesc != b.Description {
				b.Description = newDesc
				changed = true
			}
		}
		if b.IsScreenplay() {
			if _, hasStd := r.Form[newBookFormStandard]; hasStd {
				newStd := book.NormalizeScriptStandard(r.FormValue(newBookFormStandard))
				if newStd != b.NormalizedStandard() || b.ScriptStandard == "" {
					b.ScriptStandard = newStd
					changed = true
				}
			}
		}
		if startScID := strings.TrimSpace(r.FormValue(formKeyStartScene)); startScID != "" {
			if target := b.FindScene(startScID); target != nil && !strings.EqualFold(b.StartSceneID, target.ID) {
				b.StartSceneID = target.ID
				b.RawStart = target.ReferenceKey()
				if s.Player.Book != nil && s.Player.Book.ID == b.ID && s.Player.Save != nil && len(s.Player.Save.History) == 0 && s.Player.Save.AudioTime == 0 {
					s.Player.Save.CurrentSceneID = target.ID
					s.Player.PersistSave(s.Config.SavesDir)
				}
				changed = true
			}
		}
		if _, hasExpDir := r.Form[formKeyExportDir]; hasExpDir {
			rawExpDir := strings.TrimSpace(r.FormValue(formKeyExportDir))
			newExpDir := ""
			if rawExpDir != "" {
				if expanded, err := book.ExpandLocalPath(rawExpDir); err == nil && expanded != "" {
					newExpDir = expanded
				} else {
					newExpDir = book.SanitizePlainText(rawExpDir)
				}
			}
			if newExpDir != b.ExportDir {
				b.PersistExportDir(s.Config.SavesDir, s.Settings, newExpDir)
				changed = true
			}
		}
		if changed {
			s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
			if err := book.SaveBookMetadataToDisk(b); err != nil {
				toast = err.Error()
			} else {
				reloadActiveBook(s)
				if s.Editor.ActiveBook.IsScreenplay() {
					toast = tr(s, i18n.TrSavedScreenplaySettingsPrefix) + s.Editor.ActiveBook.Title
				} else {
					toast = tr(s, i18n.TrSavedBookSettingsPrefix) + s.Editor.ActiveBook.Title
				}
				s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, historyScopeMeta, coalesceSaveBookSettings, s.Editor.SelectedSceneID)
			}
		}
	}
	return frameApp(s, toast)
}

func renameBook(s *state.AppState, r *http.Request) []engine.ActionFrame {
	if isHistoryPreview(s) {
		return frameApp(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	toast := ""
	b := s.Editor.ActiveBook
	if b != nil && !b.IsPacked {
		newTitle := book.SanitizePlainText(r.FormValue(newBookFormTitle))
		if newTitle == "" {
			newTitle = book.SanitizePlainText(r.FormValue(formKeyBookTitle))
		}
		if newTitle != "" && newTitle != b.Title {
			s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
			b.Title = newTitle
			if err := book.SaveBookMetadataToDisk(b); err != nil {
				toast = err.Error()
			} else {
				reloadActiveBook(s)
				if s.Editor.ActiveBook.IsScreenplay() {
					toast = tr(s, i18n.TrRenamedScreenplayPrefix) + s.Editor.ActiveBook.Title
				} else {
					toast = tr(s, i18n.TrRenamedBookPrefix) + s.Editor.ActiveBook.Title
				}
				s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, historyScopeMeta, coalesceRenameBook, s.Editor.SelectedSceneID)
			}
		}
	}
	return frameApp(s, toast)
}

func setStartScene(s *state.AppState, r *http.Request) []engine.ActionFrame {
	syncSceneTextSettingsOpen(s, r)
	toast := withBookMutation(s, r, false, func(b *book.Book) (string, string, string, bool) {
		scID := strings.TrimSpace(r.FormValue(formKeyScene))
		if scID == "" {
			scID = strings.TrimSpace(r.FormValue(formKeySceneID))
		}
		if scID == "" {
			scID = strings.TrimSpace(r.FormValue(formKeyStartScene))
		}
		if scID == "" {
			scID = s.Editor.SelectedSceneID
		}
		target := b.FindScene(scID)
		if target == nil {
			return "", "", "", false
		}
		if ch := b.FindChapter(target.Chapter); ch != nil {
			ch.ExplicitStart = target.ReferenceKey()
			ch.StartSceneID = target.ID
		}
		b.StartSceneID = target.ID
		b.RawStart = target.ReferenceKey()
		if s.Editor.RightPanel != state.PanelBook {
			s.Editor.SelectedSceneID = target.ID
		}
		if s.Editor.SceneText.IsOpen() {
			s.Editor.SceneText.SceneID = target.ID
		}
		if s.Player.Book != nil && s.Player.Book.ID == b.ID && s.Player.Save != nil && len(s.Player.Save.History) == 0 && s.Player.Save.AudioTime == 0 {
			s.Player.Save.CurrentSceneID = target.ID
			s.Player.PersistSave(s.Config.SavesDir)
		}
		return tr(s, i18n.TrSetStartScenePrefix) + target.DisplayName(), historyScopeMeta, "", true
	})
	return frameUndoRedo(s, toast)
}

func setBookStartChapter(s *state.AppState, r *http.Request) []engine.ActionFrame {
	toast := withBookMutation(s, r, false, func(b *book.Book) (string, string, string, bool) {
		chKey := strings.TrimSpace(r.FormValue(formKeyChapter))
		if chKey == "" {
			chKey = strings.TrimSpace(r.FormValue(formKeyStartChapter))
		}
		targetCh := b.FindChapter(chKey)
		if targetCh == nil {
			return "", "", "", false
		}
		if oldStart := b.FindScene(b.StartSceneID); oldStart != nil {
			if oldCh := b.FindChapter(oldStart.Chapter); oldCh != nil && strings.TrimSpace(oldCh.ExplicitStart) == "" {
				oldCh.ExplicitStart = oldStart.ReferenceKey()
				oldCh.StartSceneID = oldStart.ID
			}
		}
		var targetScene *book.Scene
		if targetCh.StartSceneID != "" {
			targetScene = b.FindScene(targetCh.StartSceneID)
		}
		if targetScene == nil {
			for _, sc := range b.Scenes {
				scCh := strings.TrimSpace(sc.Chapter)
				if scCh == "" {
					scCh = b.DefaultChapterName()
				}
				if strings.EqualFold(scCh, strings.TrimSpace(targetCh.Name)) {
					targetScene = sc
					break
				}
			}
		}
		if targetScene == nil {
			return "", "", "", false
		}
		b.StartSceneID = targetScene.ID
		b.RawStart = targetScene.ReferenceKey()
		if s.Player.Book != nil && s.Player.Book.ID == b.ID && s.Player.Save != nil && len(s.Player.Save.History) == 0 && s.Player.Save.AudioTime == 0 {
			s.Player.Save.CurrentSceneID = targetScene.ID
			s.Player.PersistSave(s.Config.SavesDir)
		}
		return trf(s, i18n.TrSetStartChapterFmt, targetCh.DisplayName(), targetScene.DisplayName()), historyScopeMeta, coalesceSetStartChapter, true
	})
	return frameUndoRedo(s, toast)
}

func toggleBookNarrated(s *state.AppState, r *http.Request) []engine.ActionFrame {
	toast := withBookMutation(s, r, false, func(b *book.Book) (string, string, string, bool) {
		if b.IsScreenplay() {
			return "", "", "", false
		}
		b.Narrated = !b.Narrated
		if !b.Narrated {
			b.AudioFiles = nil
			for _, sc := range b.Scenes {
				sc.ExplicitVoice = ""
				sc.AudioFile = ""
				sc.AudioURL = ""
			}
		}
		if b.Narrated {
			return tr(s, i18n.TrEnabledNarrated), historyScopeMeta, "", true
		}
		return tr(s, i18n.TrDisabledNarrated), historyScopeMeta, "", true
	})
	return frameWorkspace(s, toast)
}

func toggleBookImageless(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	if isHistoryPreview(s) {
		return frameApp(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	toast := ""
	b := s.Editor.ActiveBook
	if b != nil && !b.IsPacked && !b.IsScreenplay() {
		s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
		b.Imageless = !b.Imageless
		if err := book.SaveBookMetadataToDisk(b); err != nil {
			toast = err.Error()
		} else {
			reloadActiveBook(s)
			if s.Editor.ActiveBook.Imageless {
				toast = tr(s, i18n.TrEnabledImageless)
			} else {
				toast = tr(s, i18n.TrDisabledImageless)
			}
			s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, historyScopeMeta, "", s.Editor.SelectedSceneID)
		}
	}
	return frameApp(s, toast)
}

func autoLayoutNodes(s *state.AppState, r *http.Request) []engine.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	s.Overlays.ResetCanvas = true
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	toast := ""
	b := s.Editor.ActiveBook
	if b != nil && !b.IsPacked {
		s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
		book.AutoLayoutBook(b)
		b.HasEditorView = false
		b.EditorZoom = 1.0
		if err := book.SaveBookEditorStateToDisk(b); err != nil {
			return frameUndoRedo(s, err.Error())
		}
		reloadActiveBook(s)
		toast = tr(s, i18n.TrAutoLayoutDone)
		s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, historyScopeLayout, "", s.Editor.SelectedSceneID)
	}
	return frameUndoRedo(s, toast)
}

func applyEditorViewportParams(b *book.Book, r *http.Request) {
	if b == nil {
		return
	}
	if panXStr := strings.TrimSpace(r.FormValue(formKeyPanX)); panXStr != "" {
		if px, err := strconv.Atoi(panXStr); err == nil {
			b.EditorPanX = px
			b.HasEditorView = true
		}
	}
	if panYStr := strings.TrimSpace(r.FormValue(formKeyPanY)); panYStr != "" {
		if py, err := strconv.Atoi(panYStr); err == nil {
			b.EditorPanY = py
			b.HasEditorView = true
		}
	}
	if zoomStr := strings.TrimSpace(r.FormValue(formKeyZoom)); zoomStr != "" {
		if z, err := strconv.ParseFloat(zoomStr, 64); err == nil && z >= 0.2 && z <= 3.0 {
			b.EditorZoom = z
			b.HasEditorView = true
		}
	}
}

func saveEditorViewportAndNodePositions(s *state.AppState, r *http.Request) []engine.ActionFrame {
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameNone(s, "")
	}
	if isHistoryPreview(s) {
		applyEditorViewportParams(b, r)
		return frameNone(s, "")
	}
	scID := strings.TrimSpace(r.FormValue(formKeySceneID))
	batchStr := strings.TrimSpace(r.FormValue(formKeyPositions))
	wireKeyRaw := strings.ToLower(strings.TrimSpace(r.FormValue(formKeyWireKey)))
	wirePinsRaw := strings.TrimSpace(r.FormValue(formKeyWirePins))
	if b.Layout == nil {
		b.Layout = make(map[string]book.NodePosition)
	}
	if b.WirePins == nil {
		b.WirePins = make(map[string][]book.WirePin)
	}
	movedCount := 0
	lastMovedLabel := ""
	lastMovedID := ""
	applyNodeCoord := func(targetID string, x, y int) {
		if sc := b.FindScene(targetID); sc != nil {
			prevPos, ok := b.Layout[sc.ReferenceKey()]
			if ok && prevPos.HasXY && sc.X == x && sc.Y == y {
				return
			}
			s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
			sc.X = x
			sc.Y = y
			pos := book.NodePosition{Row: sc.Row, Col: sc.Col, X: x, Y: y, HasXY: true}
			book.StoreSceneNodePosition(b, sc, pos)
			movedCount++
			lastMovedLabel = sc.DisplayName()
			lastMovedID = sc.ID
		}
	}
	if scID != "" {
		x, errX := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyX)))
		y, errY := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyY)))
		if errX == nil && errY == nil {
			applyNodeCoord(scID, x, y)
		}
	}
	if batchStr != "" {
		for _, entry := range strings.Split(batchStr, ";") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			colonIdx := strings.LastIndexByte(entry, ':')
			if colonIdx <= 0 {
				continue
			}
			idPart := strings.TrimSpace(entry[:colonIdx])
			coords := strings.Split(entry[colonIdx+1:], ",")
			if len(coords) != 2 {
				continue
			}
			x, errX := strconv.Atoi(strings.TrimSpace(coords[0]))
			y, errY := strconv.Atoi(strings.TrimSpace(coords[1]))
			if errX == nil && errY == nil {
				applyNodeCoord(idPart, x, y)
			}
		}
	}
	applyEditorViewportParams(b, r)
	hasWirePinUpdate := false
	setWirePinsIfChanged := func(k string, pins []book.WirePin) {
		prev := b.WirePins[k]
		if len(prev) == len(pins) {
			same := true
			for i := range prev {
				if prev[i].X != pins[i].X || prev[i].Y != pins[i].Y {
					same = false
					break
				}
			}
			if same {
				return
			}
		}
		s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
		if len(pins) > 0 {
			b.WirePins[k] = pins
		} else {
			delete(b.WirePins, k)
		}
		hasWirePinUpdate = true
		wireKeyRaw = k
	}
	if wireKeyRaw != "" {
		setWirePinsIfChanged(wireKeyRaw, book.ParseWirePinList(wirePinsRaw))
	} else if wirePinsRaw == "-" {
		if len(b.WirePins) > 0 {
			s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
			clear(b.WirePins)
			hasWirePinUpdate = true
		}
	} else if strings.Contains(wirePinsRaw, "=") {
		seenKeys := make(map[string]bool)
		for _, item := range strings.Split(wirePinsRaw, "|") {
			eqIdx := strings.IndexByte(item, '=')
			if eqIdx <= 0 {
				continue
			}
			k := strings.ToLower(strings.TrimSpace(item[:eqIdx]))
			if k == "" {
				continue
			}
			seenKeys[k] = true
			setWirePinsIfChanged(k, book.ParseWirePinList(strings.TrimSpace(item[eqIdx+1:])))
		}
		for existingKey := range b.WirePins {
			if !seenKeys[existingKey] {
				setWirePinsIfChanged(existingKey, nil)
			}
		}
	}
	hasNodeMove := movedCount > 0
	if err := book.SaveBookEditorStateToDisk(b); err != nil {
		return frameNone(s, err.Error())
	}
	if hasNodeMove && movedCount > 0 {
		moveLbl := trf(s, i18n.TrMovedNodesMultiFmt, movedCount)
		coalesceKey := coalesceNodeDragBatch
		if movedCount == 1 {
			moveLbl = tr(s, i18n.TrMovedNodeSinglePrefix) + lastMovedLabel
			coalesceKey = coalesceNodeDragPref + lastMovedID
		}
		s.Editor.History.RecordStep(b, moveLbl, historyScopeLayout, coalesceKey, s.Editor.SelectedSceneID)
	} else if hasWirePinUpdate {
		s.Editor.History.RecordStep(b, tr(s, i18n.TrWireRouteChanged), historyScopeLayout, coalesceWirePinPref+wireKeyRaw, s.Editor.SelectedSceneID)
	}
	return frameNone(s, "")
}

func pickExpandedBookDirectory(s *state.AppState, r *http.Request, formKey string) (string, string, bool) {
	if isHistoryPreview(s) {
		return "", tr(s, i18n.TrHistoryPreviewLocked), false
	}
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked {
		return "", "", false
	}
	picked := strings.TrimSpace(r.FormValue(formKey))
	if picked == "" {
		picked = pickNativeBookPathUnlocked(s, true)
	}
	if picked == "" {
		r.Form.Set(formFieldSilent, formValTrue)
		return "", "", false
	}
	expanded, err := book.ExpandLocalPath(picked)
	if err != nil {
		return "", err.Error(), false
	}
	return expanded, "", true
}

func browseBookExportFolder(s *state.AppState, r *http.Request) []engine.ActionFrame {
	expanded, errToast, ok := pickExpandedBookDirectory(s, r, formKeyExportDir)
	if !ok {
		return frameWorkspace(s, errToast)
	}
	b := s.Editor.ActiveBook
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	b.PersistExportDir(s.Config.SavesDir, s.Settings, expanded)
	reloadActiveBook(s)
	toast := tr(s, i18n.TrSetExportDirPrefix) + expanded
	s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, historyScopeMeta, coalesceExportDir, s.Editor.SelectedSceneID)
	return frameWorkspace(s, toast)
}

func browseBookSaveFolder(s *state.AppState, r *http.Request) []engine.ActionFrame {
	expanded, errToast, ok := pickExpandedBookDirectory(s, r, newBookFormSavePath)
	if !ok {
		return frameWorkspace(s, errToast)
	}
	oldRoot := s.Editor.ActiveBook.RootPath
	reloaded, err := book.MoveBookToDirectory(s.Editor.ActiveBook, expanded)
	if err != nil {
		return frameWorkspace(s, err.Error())
	}
	if s.Settings != nil {
		s.Settings.RemoveExternalBook(oldRoot)
		if filepath.Clean(filepath.Dir(reloaded.RootPath)) != filepath.Clean(s.Config.BooksDir) {
			s.Settings.AddExternalBook(reloaded.RootPath)
			reloaded.IsExternal = true
		}
		s.Settings.NewBookDir = filepath.Dir(reloaded.RootPath)
		_ = s.SaveSettings()
	}
	s.Library.Put(reloaded)
	s.Editor.SetBook(reloaded, s.Settings)
	return frameWorkspace(s, tr(s, i18n.TrMovedProjectToPrefix)+reloaded.RootPath)
}

func toggleStudioSearchAction(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.Editor.SearchOpen = !s.Editor.SearchOpen
	if !s.Editor.SearchOpen {
		s.Editor.SearchQuery = ""
	}
	return frameWorkspace(s, "")
}

func closeStudioSearchAction(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.Editor.SearchOpen = false
	s.Editor.SearchQuery = ""
	return frameWorkspace(s, "")
}

func filterStudioSearchAction(s *state.AppState, r *http.Request) []engine.ActionFrame {
	s.Editor.SearchOpen = true
	s.Editor.SearchQuery = strings.TrimSpace(r.FormValue(formKeySearchQuery))
	return frameWorkspace(s, "")
}

func toggleBookWarningsAction(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.Editor.WarningsOpen = !s.Editor.WarningsOpen
	return frameWorkspace(s, "")
}
