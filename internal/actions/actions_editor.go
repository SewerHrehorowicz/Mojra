//go:build !(js && wasm)

package actions

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

func activateRightPanel(s *state.AppState, panel state.EditorRightPanel) {
	s.Editor.ActivateRightPanel(panel)
	if panel != state.PanelNone && s.Editor.SceneText.WidthMode == state.WidthModeFull {
		s.Editor.SceneText.WidthMode = state.WidthModeHalf
	}
}

func openEditorHelpModal(s *state.AppState, _ *http.Request) string {
	if s.Editor.RightPanel == state.PanelHelp {
		s.Editor.ActivateRightPanel(state.PanelNone)
	} else {
		activateRightPanel(s, state.PanelHelp)
	}
	return ""
}

func closeEditorHelpModal(s *state.AppState, _ *http.Request) string {
	if s.Editor.RightPanel == state.PanelHelp {
		s.Editor.ActivateRightPanel(state.PanelNone)
	}
	return ""
}

func closeBookSettings(s *state.AppState, _ *http.Request) string {
	if s.Editor.RightPanel == state.PanelBook || s.Editor.RightPanel == state.PanelChapter {
		s.Editor.ActivateRightPanel(state.PanelNone)
	} else {
		s.Editor.SelectedChapter = ""
	}
	return ""
}

func toggleBookSettings(s *state.AppState, r *http.Request) string {
	open := s.Editor.RightPanel != state.PanelBook
	switch r.FormValue("open") {
	case "true":
		open = true
	case "false":
		open = false
	}
	if open {
		activateRightPanel(s, state.PanelBook)
	} else if s.Editor.RightPanel == state.PanelBook {
		s.Editor.ActivateRightPanel(state.PanelNone)
	}
	return ""
}

func saveBookSettings(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	toast := ""
	b := s.Editor.ActiveBook
	if b != nil && !b.IsPacked {
		changed := false
		newTitle := book.SanitizePlainText(r.FormValue("title"))
		if newTitle != "" && newTitle != b.Title {
			b.Title = newTitle
			changed = true
		}
		if _, hasAuthor := r.Form["author"]; hasAuthor {
			newAuthor := book.SanitizePlainText(r.FormValue("author"))
			if newAuthor != "" && newAuthor != b.Author {
				b.Author = newAuthor
				changed = true
			}
		}
		if _, hasLic := r.Form["license"]; hasLic {
			newLic := book.SanitizePlainText(r.FormValue("license"))
			if newLic != b.License {
				b.License = newLic
				changed = true
			}
		}
		if _, hasDesc := r.Form["description"]; hasDesc {
			newDesc := book.SanitizePlainText(r.FormValue("description"))
			if newDesc != b.Description {
				b.Description = newDesc
				changed = true
			}
		}
		if b.IsScreenplay() {
			if _, hasStd := r.Form["standard"]; hasStd {
				newStd := book.NormalizeScriptStandard(r.FormValue("standard"))
				if newStd != b.NormalizedStandard() || b.ScriptStandard == "" {
					b.ScriptStandard = newStd
					changed = true
				}
			}
		}
		if startScID := strings.TrimSpace(r.FormValue("start_scene")); startScID != "" {
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
		if _, hasExpDir := r.Form["export_dir"]; hasExpDir {
			rawExpDir := strings.TrimSpace(r.FormValue("export_dir"))
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
					toast = "Zapisano ustawienia scenariusza: " + s.Editor.ActiveBook.Title
				} else {
					toast = "Zapisano ustawienia książki: " + s.Editor.ActiveBook.Title
				}
				s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, "meta", "savebooksettings", s.Editor.SelectedSceneID)
			}
		}
	}
	return toast
}

func renameBook(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	toast := ""
	b := s.Editor.ActiveBook
	if b != nil && !b.IsPacked {
		newTitle := book.SanitizePlainText(r.FormValue("title"))
		if newTitle == "" {
			newTitle = book.SanitizePlainText(r.FormValue("book_title"))
		}
		if newTitle != "" && newTitle != b.Title {
			s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
			b.Title = newTitle
			if err := book.SaveBookMetadataToDisk(b); err != nil {
				toast = err.Error()
			} else {
				reloadActiveBook(s)
				if s.Editor.ActiveBook.IsScreenplay() {
					toast = "Zmieniono tytuł scenariusza na: " + s.Editor.ActiveBook.Title
				} else {
					toast = "Zmieniono tytuł książki na: " + s.Editor.ActiveBook.Title
				}
				s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, "meta", "renamebook", s.Editor.SelectedSceneID)
			}
		}
	}
	return toast
}

func setStartScene(s *state.AppState, r *http.Request) string {
	syncSceneTextSettingsOpen(s, r)
	return withBookMutation(s, r, false, func(b *book.Book) (string, string, string, bool) {
		scID := strings.TrimSpace(r.FormValue("scene"))
		if scID == "" {
			scID = strings.TrimSpace(r.FormValue("scene_id"))
		}
		if scID == "" {
			scID = strings.TrimSpace(r.FormValue("start_scene"))
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
		return "Ustawiono scenę początkową: " + target.DisplayName(), "meta", "", true
	})
}

func setBookStartChapter(s *state.AppState, r *http.Request) string {
	return withBookMutation(s, r, false, func(b *book.Book) (string, string, string, bool) {
		chKey := strings.TrimSpace(r.FormValue("chapter"))
		if chKey == "" {
			chKey = strings.TrimSpace(r.FormValue("start_chapter"))
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
		return fmt.Sprintf("Ustawiono rozdział początkowy: %s (%s)", targetCh.DisplayName(), targetScene.DisplayName()), "meta", "setstartchapter", true
	})
}

func toggleBookNarrated(s *state.AppState, r *http.Request) string {
	return withBookMutation(s, r, false, func(b *book.Book) (string, string, string, bool) {
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
			return "Włączono wymaganą narrację (narrated: true)", "meta", "", true
		}
		return "Wyłączono narrację książki (narrated: false)", "meta", "", true
	})
}

func toggleBookImageless(s *state.AppState, _ *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
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
				toast = "Wymuszono tryb bez obrazów dla książki (imageless: true)"
			} else {
				toast = "Wyłączono wymuszanie trybu bez obrazów (imageless: false)"
			}
			s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, "meta", "", s.Editor.SelectedSceneID)
		}
	}
	return toast
}

func autoLayoutNodes(s *state.AppState, _ *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	toast := ""
	b := s.Editor.ActiveBook
	if b != nil && !b.IsPacked {
		s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
		book.AutoLayoutBook(b)
		b.HasEditorView = false
		b.EditorZoom = 1.0
		if err := book.SaveBookEditorStateToDisk(b); err != nil {
			return err.Error()
		}
		reloadActiveBook(s)
		toast = "Automatycznie rozmieszczono węzły na siatce"
		s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, "layout", "", s.Editor.SelectedSceneID)
	}
	return toast
}

func applyEditorViewportParams(b *book.Book, r *http.Request) {
	if b == nil {
		return
	}
	if panXStr := strings.TrimSpace(r.FormValue("pan_x")); panXStr != "" {
		if px, err := strconv.Atoi(panXStr); err == nil {
			b.EditorPanX = px
			b.HasEditorView = true
		}
	}
	if panYStr := strings.TrimSpace(r.FormValue("pan_y")); panYStr != "" {
		if py, err := strconv.Atoi(panYStr); err == nil {
			b.EditorPanY = py
			b.HasEditorView = true
		}
	}
	if zoomStr := strings.TrimSpace(r.FormValue("zoom")); zoomStr != "" {
		if z, err := strconv.ParseFloat(zoomStr, 64); err == nil && z >= 0.2 && z <= 3.0 {
			b.EditorZoom = z
			b.HasEditorView = true
		}
	}
}

func saveEditorViewportAndNodePositions(s *state.AppState, r *http.Request) string {
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	if isHistoryPreview(s) {
		applyEditorViewportParams(b, r)
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene_id"))
	batchStr := strings.TrimSpace(r.FormValue("positions"))
	wireKeyRaw := strings.ToLower(strings.TrimSpace(r.FormValue("wire_key")))
	wirePinsRaw := strings.TrimSpace(r.FormValue("wire_pins"))
	hasNodeMove := scID != "" || batchStr != ""
	hasWirePinUpdate := wireKeyRaw != ""
	if hasNodeMove || hasWirePinUpdate {
		s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	}
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
		x, errX := strconv.Atoi(strings.TrimSpace(r.FormValue("x")))
		y, errY := strconv.Atoi(strings.TrimSpace(r.FormValue("y")))
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
	if hasWirePinUpdate {
		pins := book.ParseWirePinList(wirePinsRaw)
		if len(pins) > 0 {
			b.WirePins[wireKeyRaw] = pins
		} else {
			delete(b.WirePins, wireKeyRaw)
		}
	}
	if err := book.SaveBookEditorStateToDisk(b); err != nil {
		return err.Error()
	}
	if hasNodeMove && movedCount > 0 {
		moveLbl := fmt.Sprintf("Przesunięto węzły (%d)", movedCount)
		coalesceKey := "nodedrag:batch"
		if movedCount == 1 {
			moveLbl = "Przesunięto węzeł: " + lastMovedLabel
			coalesceKey = "nodedrag:" + lastMovedID
		}
		s.Editor.History.RecordStep(b, moveLbl, "layout", coalesceKey, s.Editor.SelectedSceneID)
	} else if hasWirePinUpdate {
		s.Editor.History.RecordStep(b, "Zmieniono trasę przewodu", "layout", "wirepin:"+wireKeyRaw, s.Editor.SelectedSceneID)
	}
	return ""
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
		r.Form.Set("silent", "true")
		return "", "", false
	}
	expanded, err := book.ExpandLocalPath(picked)
	if err != nil {
		return "", err.Error(), false
	}
	return expanded, "", true
}

func browseBookExportFolder(s *state.AppState, r *http.Request) string {
	expanded, errToast, ok := pickExpandedBookDirectory(s, r, "export_dir")
	if !ok {
		return errToast
	}
	b := s.Editor.ActiveBook
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	b.PersistExportDir(s.Config.SavesDir, s.Settings, expanded)
	reloadActiveBook(s)
	toast := "Ustawiono katalog eksportu: " + expanded
	s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, "meta", "exportdir", s.Editor.SelectedSceneID)
	return toast
}

func browseBookSaveFolder(s *state.AppState, r *http.Request) string {
	expanded, errToast, ok := pickExpandedBookDirectory(s, r, "save_path")
	if !ok {
		return errToast
	}
	oldRoot := s.Editor.ActiveBook.RootPath
	reloaded, err := book.MoveBookToDirectory(s.Editor.ActiveBook, expanded)
	if err != nil {
		return err.Error()
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
	return "Przeniesiono projekt do: " + reloaded.RootPath
}
