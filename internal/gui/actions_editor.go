package gui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"storyteller/internal/book"
)

func (s *Server) autoSaveSceneAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return ""
	}
	_ = s.saveSceneFromFormWithMode(r, true)
	return ""
}

func (s *Server) saveSceneAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	_ = s.saveSceneFromForm(r)
	return ""
}

func (s *Server) openBookSettings(r *http.Request) string {
	if !s.isHistoryPreview() && r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	s.BookSettingsOpen = true
	s.SelectedChapter = ""
	s.EditorBreakdownPanelOpen = false
	s.EditorVariablesPanelOpen = false
	s.EditorHistoryPanelOpen = false
	s.EditorHelpModalOpen = false
	return ""
}

func (s *Server) closeBookSettings(_ *http.Request) string {
	s.BookSettingsOpen = false
	s.SelectedChapter = ""
	return ""
}

func (s *Server) toggleBookSettings(r *http.Request) string {
	if !s.isHistoryPreview() && r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	switch r.FormValue("open") {
	case "true":
		s.BookSettingsOpen = true
	case "false":
		s.BookSettingsOpen = false
	default:
		s.BookSettingsOpen = !s.BookSettingsOpen
	}
	if s.BookSettingsOpen {
		s.SelectedChapter = ""
		s.EditorBreakdownPanelOpen = false
		s.EditorVariablesPanelOpen = false
		s.EditorHistoryPanelOpen = false
		s.EditorHelpModalOpen = false
		if s.SceneTextModalWidthMode == "full" {
			s.SceneTextModalWidthMode = "half"
		}
	}
	return ""
}

func (s *Server) saveBookSettings(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	toast := ""
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		changed := false
		newTitle := book.SanitizePlainText(r.FormValue("title"))
		if newTitle != "" && newTitle != s.ActiveBook.Title {
			s.ActiveBook.Title = newTitle
			changed = true
		}
		if _, hasAuthor := r.Form["author"]; hasAuthor {
			newAuthor := book.SanitizePlainText(r.FormValue("author"))
			if newAuthor != "" && newAuthor != s.ActiveBook.Author {
				s.ActiveBook.Author = newAuthor
				changed = true
			}
		}
		if _, hasLic := r.Form["license"]; hasLic {
			newLic := book.SanitizePlainText(r.FormValue("license"))
			if newLic != s.ActiveBook.License {
				s.ActiveBook.License = newLic
				changed = true
			}
		}
		if _, hasDesc := r.Form["description"]; hasDesc {
			newDesc := book.SanitizePlainText(r.FormValue("description"))
			if newDesc != s.ActiveBook.Description {
				s.ActiveBook.Description = newDesc
				changed = true
			}
		}
		if s.ActiveBook.IsScreenplay() {
			if _, hasStd := r.Form["standard"]; hasStd {
				newStd := book.NormalizeScriptStandard(r.FormValue("standard"))
				if newStd != s.ActiveBook.NormalizedStandard() || s.ActiveBook.ScriptStandard == "" {
					s.ActiveBook.ScriptStandard = newStd
					changed = true
				}
			}
		}
		if startScID := strings.TrimSpace(r.FormValue("start_scene")); startScID != "" {
			if target := s.findScene(startScID); target != nil && !strings.EqualFold(s.ActiveBook.StartSceneID, target.ID) {
				s.ActiveBook.StartSceneID = target.ID
				s.ActiveBook.RawStart = target.ReferenceKey()
				if s.PlayerBook != nil && s.PlayerBook.ID == s.ActiveBook.ID && s.Save != nil && len(s.Save.History) == 0 && s.Save.AudioTime == 0 {
					s.Save.CurrentSceneID = target.ID
					_ = book.WriteSaveState(s.SavesDir, s.Save)
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
			if newExpDir != s.ActiveBook.ExportDir {
				s.persistActiveBookExportDir(newExpDir)
				changed = true
			}
		}
		if changed {
			s.ensureHistoryBaselineBeforeMutation()
			if err := book.SaveBookMetadataToDisk(s.ActiveBook); err != nil {
				toast = err.Error()
			} else {
				s.reloadActiveBook()
				if s.ActiveBook.IsScreenplay() {
					toast = "Zapisano ustawienia scenariusza: " + s.ActiveBook.Title
				} else {
					toast = "Zapisano ustawienia książki: " + s.ActiveBook.Title
				}
				s.recordHistoryAfterMutation(toast, "meta", "savebooksettings")
			}
		}
	}
	return toast
}

func (s *Server) renameBook(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	toast := ""
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		newTitle := book.SanitizePlainText(r.FormValue("title"))
		if newTitle == "" {
			newTitle = book.SanitizePlainText(r.FormValue("book_title"))
		}
		if newTitle != "" && newTitle != s.ActiveBook.Title {
			s.ensureHistoryBaselineBeforeMutation()
			s.ActiveBook.Title = newTitle
			if err := book.SaveBookMetadataToDisk(s.ActiveBook); err != nil {
				toast = err.Error()
			} else {
				s.reloadActiveBook()
				if s.ActiveBook.IsScreenplay() {
					toast = "Zmieniono tytuł scenariusza na: " + s.ActiveBook.Title
				} else {
					toast = "Zmieniono tytuł książki na: " + s.ActiveBook.Title
				}
				s.recordHistoryAfterMutation(toast, "meta", "renamebook")
			}
		}
	}
	return toast
}

func (s *Server) setStartScene(r *http.Request) string {
	if _, hasSettingsOpen := r.Form["scene_settings_open"]; hasSettingsOpen {
		s.SceneTextModalSettingsOpen = r.FormValue("scene_settings_open") == "true"
	}
	return s.withBookMutation(r, false, func(b *book.Book) (string, string, string, bool) {
		scID := strings.TrimSpace(r.FormValue("scene"))
		if scID == "" {
			scID = strings.TrimSpace(r.FormValue("scene_id"))
		}
		if scID == "" {
			scID = strings.TrimSpace(r.FormValue("start_scene"))
		}
		if scID == "" {
			scID = s.SelectedSceneID
		}
		target := s.findScene(scID)
		if target == nil {
			return "", "", "", false
		}
		if ch := b.FindChapter(target.Chapter); ch != nil {
			ch.ExplicitStart = target.ReferenceKey()
			ch.StartSceneID = target.ID
		}
		b.StartSceneID = target.ID
		b.RawStart = target.ReferenceKey()
		if !s.BookSettingsOpen {
			s.SelectedSceneID = target.ID
		}
		if s.SceneTextModalID != "" {
			s.SceneTextModalID = target.ID
		}
		if s.PlayerBook != nil && s.PlayerBook.ID == b.ID && s.Save != nil && len(s.Save.History) == 0 && s.Save.AudioTime == 0 {
			s.Save.CurrentSceneID = target.ID
			_ = book.WriteSaveState(s.SavesDir, s.Save)
		}
		return "Ustawiono scenę początkową: " + target.DisplayName(), "meta", "", true
	})
}

func (s *Server) setBookStartChapter(r *http.Request) string {
	return s.withBookMutation(r, false, func(b *book.Book) (string, string, string, bool) {
		chKey := strings.TrimSpace(r.FormValue("chapter"))
		if chKey == "" {
			chKey = strings.TrimSpace(r.FormValue("start_chapter"))
		}
		targetCh := b.FindChapter(chKey)
		if targetCh == nil {
			return "", "", "", false
		}
		if oldStart := s.findScene(b.StartSceneID); oldStart != nil {
			if oldCh := b.FindChapter(oldStart.Chapter); oldCh != nil && strings.TrimSpace(oldCh.ExplicitStart) == "" {
				oldCh.ExplicitStart = oldStart.ReferenceKey()
				oldCh.StartSceneID = oldStart.ID
			}
		}
		var targetScene *book.Scene
		if targetCh.StartSceneID != "" {
			targetScene = s.findScene(targetCh.StartSceneID)
		}
		if targetScene == nil {
			for _, sc := range b.Scenes {
				scCh := strings.TrimSpace(sc.Chapter)
				if scCh == "" {
					scCh = "Rozdział 1"
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
		if s.PlayerBook != nil && s.PlayerBook.ID == b.ID && s.Save != nil && len(s.Save.History) == 0 && s.Save.AudioTime == 0 {
			s.Save.CurrentSceneID = targetScene.ID
			_ = book.WriteSaveState(s.SavesDir, s.Save)
		}
		return fmt.Sprintf("Ustawiono rozdział początkowy: %s (%s)", targetCh.DisplayName(), targetScene.DisplayName()), "meta", "setstartchapter", true
	})
}

func (s *Server) toggleBookNarrated(r *http.Request) string {
	return s.withBookMutation(r, false, func(b *book.Book) (string, string, string, bool) {
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

func (s *Server) toggleBookImageless(_ *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	toast := ""
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked && !s.ActiveBook.IsScreenplay() {
		s.ensureHistoryBaselineBeforeMutation()
		s.ActiveBook.Imageless = !s.ActiveBook.Imageless
		if err := book.SaveBookMetadataToDisk(s.ActiveBook); err != nil {
			toast = err.Error()
		} else {
			s.reloadActiveBook()
			if s.ActiveBook.Imageless {
				toast = "Wymuszono tryb bez obrazów dla książki (imageless: true)"
			} else {
				toast = "Wyłączono wymuszanie trybu bez obrazów (imageless: false)"
			}
			s.recordHistoryAfterMutation(toast, "meta", "")
		}
	}
	return toast
}

func (s *Server) promptDeleteScene(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	targetID := strings.TrimSpace(r.FormValue("scene"))
	if targetID == "" {
		targetID = s.SelectedSceneID
	}
	if sc := s.findScene(targetID); sc != nil {
		s.ConfirmTitle = "Usuń scenę"
		s.ConfirmMessage = fmt.Sprintf("Czy na pewno chcesz usunąć scenę „%s” z pliku %s?", sc.DisplayName(), sc.SourceFile)
		s.ConfirmPendingAction = "DeleteScene"
		s.ConfirmPendingParam = sc.ID
	}
	return ""
}

func (s *Server) deleteSceneFromForm(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	targetID := strings.TrimSpace(r.FormValue("scene"))
	if targetID == "" {
		targetID = s.SelectedSceneID
	}
	return s.deleteSceneByID(targetID)
}

func (s *Server) deleteScenesFromForm(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	rawIDs := strings.TrimSpace(r.FormValue("scenes"))
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue("scene_ids"))
	}
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue("scene"))
	}
	return s.deleteScenesByIDs(rawIDs)
}

func (s *Server) selectStudioScene(r *http.Request) string {
	if !s.isHistoryPreview() && r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	if found := s.findScene(scID); found != nil {
		s.SelectedSceneID = found.ID
		if s.SceneTextModalID != "" {
			s.SceneTextModalID = found.ID
		}
	}
	return ""
}

func (s *Server) selectStudioChapter(r *http.Request) string {
	if !s.isHistoryPreview() && r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	chKey := strings.TrimSpace(r.FormValue("chapter"))
	if s.ActiveBook != nil && chKey != "" {
		if foundCh := s.ActiveBook.FindChapter(chKey); foundCh != nil {
			s.SelectedChapter = foundCh.Name
			s.BookSettingsOpen = false
			s.EditorBreakdownPanelOpen = false
			s.EditorVariablesPanelOpen = false
			s.EditorHistoryPanelOpen = false
			s.EditorHelpModalOpen = false
		}
	}
	return ""
}

func (s *Server) insertScene(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	toast := ""
	if s.ActiveBook != nil {
		s.ensureHistoryBaselineBeforeMutation()
		parentID := strings.TrimSpace(r.FormValue("parent"))
		childID := strings.TrimSpace(r.FormValue("child"))
		if parentID == "" && len(s.ActiveBook.Scenes) > 0 {
			parentID = s.ActiveBook.Scenes[len(s.ActiveBook.Scenes)-1].ID
		}
		newName := book.NextUniqueSceneName(s.ActiveBook)
		if inserted, err := book.InsertSceneBetween(s.ActiveBook, parentID, childID, newName); err == nil && inserted != nil {
			s.reloadActiveBook()
			if found := s.findScene(inserted.Name); found != nil {
				s.SelectedSceneID = found.ID
			} else {
				s.SelectedSceneID = inserted.ID
			}
			toast = "Wstawiono nową scenę: " + inserted.Name
			s.recordHistoryAfterMutation(toast, "graph", "")
		} else if err != nil {
			toast = err.Error()
		}
	}
	return toast
}

func (s *Server) insertSceneBefore(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	toast := ""
	if s.ActiveBook != nil {
		s.ensureHistoryBaselineBeforeMutation()
		targetID := strings.TrimSpace(r.FormValue("target"))
		if targetID == "" {
			targetID = strings.TrimSpace(r.FormValue("before"))
		}
		if targetID == "" {
			targetID = strings.TrimSpace(r.FormValue("scene"))
		}
		newName := book.NextUniqueSceneName(s.ActiveBook)
		if inserted, err := book.InsertSceneBefore(s.ActiveBook, targetID, newName); err == nil && inserted != nil {
			s.reloadActiveBook()
			if found := s.findScene(inserted.Name); found != nil {
				s.SelectedSceneID = found.ID
			} else {
				s.SelectedSceneID = inserted.ID
			}
			toast = "Wstawiono nową scenę: " + inserted.Name
			s.recordHistoryAfterMutation(toast, "graph", "")
		} else if err != nil {
			toast = err.Error()
		}
	}
	return toast
}

func (s *Server) createSceneAt(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	if s.ActiveBook == nil && !s.StandaloneMode {
		if createdBook, err := book.CreateNewBook(s.BooksDir, "Nowa Opowieść", "Autor", false); err == nil && createdBook != nil {
			s.setActiveBook(createdBook)
			_ = book.SaveAppSettings(s.SavesDir, s.Settings)
		}
	}
	toast := ""
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		s.ensureHistoryBaselineBeforeMutation()
		x, errX := strconv.Atoi(strings.TrimSpace(r.FormValue("x")))
		y, errY := strconv.Atoi(strings.TrimSpace(r.FormValue("y")))
		hasCoords := errX == nil && errY == nil
		targetCh := strings.TrimSpace(r.FormValue("chapter"))
		newName := book.NextUniqueSceneName(s.ActiveBook)
		if created, err := book.CreateStandaloneSceneInChapter(s.ActiveBook, newName, targetCh, x, y, hasCoords); err == nil && created != nil {
			s.reloadActiveBook()
			if found := s.findScene(created.Name); found != nil {
				s.SelectedSceneID = found.ID
			} else {
				s.SelectedSceneID = created.ID
			}
			toast = "Dodano nową scenę: " + created.Name
			s.recordHistoryAfterMutation(toast, "graph", "")
		} else if err != nil {
			toast = err.Error()
		}
	}
	return toast
}

func (s *Server) autoLayoutNodes(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	toast := ""
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		s.ensureHistoryBaselineBeforeMutation()
		book.AutoLayoutBook(s.ActiveBook)
		s.ActiveBook.HasEditorView = false
		s.ActiveBook.EditorZoom = 1.0
		_ = book.SaveBookEditorStateToDisk(s.ActiveBook)
		s.reloadActiveBook()
		toast = "Automatycznie rozmieszczono węzły na siatce"
		s.recordHistoryAfterMutation(toast, "layout", "")
	}
	return toast
}

func (s *Server) openSceneTextModal(r *http.Request) string {
	if !s.isHistoryPreview() && r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	if scID == "" {
		scID = s.SelectedSceneID
	}
	if scID == "" && s.ActiveBook != nil {
		scID = s.ActiveBook.StartSceneID
		if scID == "" && len(s.ActiveBook.Scenes) > 0 {
			scID = s.ActiveBook.Scenes[0].ID
		}
	}
	if sc := s.findScene(scID); sc != nil {
		if s.SelectedSceneID != sc.ID {
			s.SceneTextModalSettingsOpen = false
		}
		if s.SceneTextModalID == "" {
			s.SceneTextModalWidthMode = "half"
		}
		s.SelectedSceneID = sc.ID
		s.SceneTextModalID = sc.ID
	}
	return ""
}

func (s *Server) setSceneTextWidthModeAction(r *http.Request) string {
	mode := strings.ToLower(strings.TrimSpace(r.FormValue("mode")))
	if mode == "full" {
		s.SceneTextModalWidthMode = "full"
	} else {
		s.SceneTextModalWidthMode = "half"
	}
	return ""
}

func (s *Server) closeSceneTextModal(r *http.Request) string {
	if s.SceneTextModalID != "" && !s.isHistoryPreview() {
		if curSc := s.findScene(s.SceneTextModalID); curSc != nil && s.ActiveBook != nil && !s.ActiveBook.IsPacked {
			if s.hasSceneTextModalChanges(curSc, r) {
				s.ensureHistoryBaselineBeforeMutation()
				s.applySceneFormFields(curSc, r)
				curScLabel := curSc.DisplayName()
				curScID := curSc.ID
				_ = s.commitScenesMutation("Edycja treści sceny: "+curScLabel, "text", "textmodal:"+curScID)
				s.SelectedSceneID = curScID
			}
		}
	}
	s.SceneTextModalID = ""
	s.SceneTestVarsPopoverOpen = false
	return ""
}

func (s *Server) toggleSceneTextSettings(_ *http.Request) string {
	s.SceneTextModalSettingsOpen = !s.SceneTextModalSettingsOpen
	return ""
}

func (s *Server) saveSceneTextModal(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	toast := ""
	if _, hasSettingsOpen := r.Form["scene_settings_open"]; hasSettingsOpen {
		s.SceneTextModalSettingsOpen = r.FormValue("scene_settings_open") == "true"
	}
	scID := strings.TrimSpace(r.FormValue("scene_id"))
	if scID == "" {
		scID = s.SceneTextModalID
	}
	if target := s.findScene(scID); target != nil && s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		s.ensureHistoryBaselineBeforeMutation()
		s.applySceneFormFields(target, r)
		book.FinalizeBook(s.ActiveBook)
		targetLabel := target.DisplayName()
		targetID := target.ID
		s.SelectedSceneID = target.ID
		s.SceneTextModalID = target.ID
		if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
			toast = err.Error()
		} else {
			s.reloadActiveBook()
			toast = "Zapisano treść sceny: " + targetLabel
			s.recordHistoryAfterMutation(toast, "text", "textmodal:"+targetID)
		}
	}
	return toast
}

func (s *Server) saveEditorViewportAndNodePositions(r *http.Request) string {
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	if s.isHistoryPreview() {
		if panXStr := strings.TrimSpace(r.FormValue("pan_x")); panXStr != "" {
			if px, err := strconv.Atoi(panXStr); err == nil {
				s.ActiveBook.EditorPanX = px
				s.ActiveBook.HasEditorView = true
			}
		}
		if panYStr := strings.TrimSpace(r.FormValue("pan_y")); panYStr != "" {
			if py, err := strconv.Atoi(panYStr); err == nil {
				s.ActiveBook.EditorPanY = py
				s.ActiveBook.HasEditorView = true
			}
		}
		if zoomStr := strings.TrimSpace(r.FormValue("zoom")); zoomStr != "" {
			if z, err := strconv.ParseFloat(zoomStr, 64); err == nil && z >= 0.2 && z <= 3.0 {
				s.ActiveBook.EditorZoom = z
				s.ActiveBook.HasEditorView = true
			}
		}
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene_id"))
	batchStr := strings.TrimSpace(r.FormValue("positions"))
	wireKeyRaw := strings.ToLower(strings.TrimSpace(r.FormValue("wire_key")))
	wirePinsRaw := strings.TrimSpace(r.FormValue("wire_pins"))
	hasNodeMove := scID != "" || batchStr != ""
	hasWirePinUpdate := wireKeyRaw != ""
	if hasNodeMove || hasWirePinUpdate {
		s.ensureHistoryBaselineBeforeMutation()
	}
	if s.ActiveBook.Layout == nil {
		s.ActiveBook.Layout = make(map[string]book.NodePosition)
	}
	if s.ActiveBook.WirePins == nil {
		s.ActiveBook.WirePins = make(map[string][]book.WirePin)
	}
	movedCount := 0
	lastMovedLabel := ""
	lastMovedID := ""
	applyNodeCoord := func(targetID string, x, y int) {
		if sc := s.findScene(targetID); sc != nil {
			sc.X = x
			sc.Y = y
			pos := book.NodePosition{Row: sc.Row, Col: sc.Col, X: x, Y: y, HasXY: true}
			book.StoreSceneNodePosition(s.ActiveBook, sc, pos)
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
	if panXStr := strings.TrimSpace(r.FormValue("pan_x")); panXStr != "" {
		if px, err := strconv.Atoi(panXStr); err == nil {
			s.ActiveBook.EditorPanX = px
			s.ActiveBook.HasEditorView = true
		}
	}
	if panYStr := strings.TrimSpace(r.FormValue("pan_y")); panYStr != "" {
		if py, err := strconv.Atoi(panYStr); err == nil {
			s.ActiveBook.EditorPanY = py
			s.ActiveBook.HasEditorView = true
		}
	}
	if zoomStr := strings.TrimSpace(r.FormValue("zoom")); zoomStr != "" {
		if z, err := strconv.ParseFloat(zoomStr, 64); err == nil && z >= 0.2 && z <= 3.0 {
			s.ActiveBook.EditorZoom = z
			s.ActiveBook.HasEditorView = true
		}
	}
	if hasWirePinUpdate {
		pins := book.ParseWirePinList(wirePinsRaw)
		if len(pins) > 0 {
			s.ActiveBook.WirePins[wireKeyRaw] = pins
		} else {
			delete(s.ActiveBook.WirePins, wireKeyRaw)
		}
	}
	_ = book.SaveBookEditorStateToDisk(s.ActiveBook)
	if hasNodeMove && movedCount > 0 {
		moveLbl := fmt.Sprintf("Przesunięto węzły (%d)", movedCount)
		coalesceKey := "nodedrag:batch"
		if movedCount == 1 {
			moveLbl = "Przesunięto węzeł: " + lastMovedLabel
			coalesceKey = "nodedrag:" + lastMovedID
		}
		s.recordHistoryAfterMutation(moveLbl, "layout", coalesceKey)
	} else if hasWirePinUpdate {
		s.recordHistoryAfterMutation("Zmieniono trasę przewodu", "layout", "wirepin:"+wireKeyRaw)
	}
	return ""
}

func (s *Server) promptDeleteScenesFromForm(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	rawIDs := strings.TrimSpace(r.FormValue("scenes"))
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue("scene_ids"))
	}
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue("scene"))
	}
	var matched []*book.Scene
	seen := make(map[string]bool)
	for _, part := range strings.Split(rawIDs, ",") {
		idPart := strings.TrimSpace(part)
		if idPart == "" {
			continue
		}
		if sc := s.findScene(idPart); sc != nil && !seen[sc.ID] {
			seen[sc.ID] = true
			matched = append(matched, sc)
		}
	}
	if len(matched) == 1 {
		sc := matched[0]
		s.ConfirmTitle = "Usuń scenę"
		s.ConfirmMessage = fmt.Sprintf("Czy na pewno chcesz usunąć scenę „%s” z pliku %s?", sc.DisplayName(), sc.SourceFile)
		s.ConfirmPendingAction = "DeleteScene"
		s.ConfirmPendingParam = sc.ID
	} else if len(matched) > 1 {
		var ids []string
		var names []string
		for _, sc := range matched {
			ids = append(ids, sc.ID)
			names = append(names, fmt.Sprintf("„%s”", sc.DisplayName()))
		}
		s.ConfirmTitle = fmt.Sprintf("Usuń zaznaczone sceny (%d)", len(matched))
		s.ConfirmMessage = fmt.Sprintf("Czy na pewno chcesz usunąć zaznaczone sceny (%d): %s?", len(matched), strings.Join(names, ", "))
		s.ConfirmPendingAction = "DeleteScenes"
		s.ConfirmPendingParam = strings.Join(ids, ",")
	}
	return ""
}

func (s *Server) navigateSceneTextModal(r *http.Request) string {
	toast := ""
	s.SceneTextModalSettingsOpen = false
	curID := strings.TrimSpace(r.FormValue("scene_id"))
	if curID == "" {
		curID = s.SceneTextModalID
	}
	if !s.isHistoryPreview() {
		if curSc := s.findScene(curID); curSc != nil && s.ActiveBook != nil && !s.ActiveBook.IsPacked {
			if s.hasSceneTextModalChanges(curSc, r) {
				s.ensureHistoryBaselineBeforeMutation()
				s.applySceneFormFields(curSc, r)
				book.FinalizeBook(s.ActiveBook)
				curScLabel := curSc.DisplayName()
				curScID := curSc.ID
				if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
					toast = err.Error()
				} else {
					s.reloadActiveBook()
					s.recordHistoryAfterMutation("Edycja treści sceny: "+curScLabel, "text", "textmodal:"+curScID)
				}
			}
		}
	}
	targetID := strings.TrimSpace(r.FormValue("scene"))
	if targetID == "" {
		targetID = strings.TrimSpace(r.FormValue("target_scene"))
	}
	var chosenEffects string
	if choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx"))); err == nil {
		if curSc := s.findScene(curID); curSc != nil && choiceIdx >= 0 && choiceIdx < len(curSc.Choices) {
			ch := curSc.Choices[choiceIdx]
			chosenEffects = ch.Effects
			if ch.IsEnding || strings.EqualFold(ch.TargetSceneID, "END") || strings.EqualFold(ch.Target, "END") {
				if s.ActiveBook != nil {
					targetID = s.ActiveBook.StartSceneID
				}
			} else if targetID == "" {
				if ch.TargetSceneID != "" {
					targetID = ch.TargetSceneID
				} else {
					targetID = ch.Target
				}
			}
		}
	}
	if strings.EqualFold(targetID, "END") && s.ActiveBook != nil {
		targetID = s.ActiveBook.StartSceneID
	}
	if nextSc := s.findScene(targetID); nextSc != nil {
		if s.ActiveBook != nil && len(s.ActiveBook.Variables) > 0 {
			s.SceneTextTestVars = book.EnsureSaveVariables(s.ActiveBook, s.SceneTextTestVars)
			if chosenEffects != "" {
				book.ApplyEffects(chosenEffects, s.SceneTextTestVars, s.ActiveBook)
			}
			if nextSc.Effects != "" {
				book.ApplyEffects(nextSc.Effects, s.SceneTextTestVars, s.ActiveBook)
			}
		}
		s.SelectedSceneID = nextSc.ID
		s.SceneTextModalID = nextSc.ID
	} else if s.ActiveBook != nil && len(s.ActiveBook.Scenes) > 0 && targetID == "" {
		s.SelectedSceneID = s.ActiveBook.Scenes[0].ID
		s.SceneTextModalID = s.ActiveBook.Scenes[0].ID
	}
	return toast
}

func (s *Server) toggleEditorBreakdownPanel(r *http.Request) string {
	if !s.isHistoryPreview() && r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	if r.FormValue("book_settings_open") == "false" {
		s.BookSettingsOpen = false
	}
	s.EditorBreakdownPanelOpen = !s.EditorBreakdownPanelOpen
	if s.EditorBreakdownPanelOpen {
		s.BookSettingsOpen = false
		s.SelectedChapter = ""
		s.EditorVariablesPanelOpen = false
		s.EditorHistoryPanelOpen = false
		s.EditorHelpModalOpen = false
		if s.SceneTextModalWidthMode == "full" {
			s.SceneTextModalWidthMode = "half"
		}
	}
	return ""
}

func (s *Server) openEditorBreakdownPanel(r *http.Request) string {
	if !s.isHistoryPreview() && r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	s.EditorBreakdownPanelOpen = true
	s.BookSettingsOpen = false
	s.SelectedChapter = ""
	s.EditorVariablesPanelOpen = false
	s.EditorHistoryPanelOpen = false
	s.EditorHelpModalOpen = false
	return ""
}

func (s *Server) closeEditorBreakdownPanel(_ *http.Request) string {
	s.EditorBreakdownPanelOpen = false
	return ""
}

func (s *Server) renameCharacterAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	oldName := strings.TrimSpace(r.FormValue("old_name"))
	if oldName == "" {
		oldName = strings.TrimSpace(r.FormValue("character"))
	}
	newName := strings.TrimSpace(r.FormValue("new_name"))
	s.ensureHistoryBaselineBeforeMutation()
	count, err := book.RenameScreenplayCharacter(s.ActiveBook, oldName, newName)
	if err != nil {
		return err.Error()
	}
	s.invalidateHistoryCache()
	s.reloadActiveBook()
	if count == 0 {
		return "Brak kwestii do zmiany"
	}
	return fmt.Sprintf("Zmieniono postać %s → %s (%d kwestii)", strings.ToUpper(oldName), strings.ToUpper(newName), count)
}

func (s *Server) renameLocationAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	oldLoc := strings.TrimSpace(r.FormValue("old_location"))
	if oldLoc == "" {
		oldLoc = strings.TrimSpace(r.FormValue("location"))
	}
	newLoc := strings.TrimSpace(r.FormValue("new_location"))
	s.ensureHistoryBaselineBeforeMutation()
	count, err := book.RenameScreenplayLocation(s.ActiveBook, oldLoc, newLoc)
	if err != nil {
		return err.Error()
	}
	s.invalidateHistoryCache()
	s.reloadActiveBook()
	if count == 0 {
		return "Brak lokacji do zmiany"
	}
	return fmt.Sprintf("Zmieniono lokację %s → %s (%d scen)", strings.ToUpper(oldLoc), strings.ToUpper(newLoc), count)
}

func (s *Server) exportScreenplayPDFAction(r *http.Request) string {
	if s.ActiveBook == nil {
		return "Brak aktywnego scenariusza"
	}
	exportDir, errMsg, ok := s.resolveOrPickBookExportDir(r)
	if !ok {
		return errMsg
	}
	pdfBytes, err := book.ExportScreenplayPDF(s.ActiveBook)
	if err != nil {
		return err.Error()
	}
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return err.Error()
	}
	outPath := filepath.Join(exportDir, s.ActiveBook.ID+".pdf")
	if err := os.WriteFile(outPath, pdfBytes, 0644); err != nil {
		return err.Error()
	}
	s.revealExportedFileInOS(outPath)
	return "Wyeksportowano scenariusz PDF: " + outPath
}

func (s *Server) exportScreenplayFountainAction(r *http.Request) string {
	if s.ActiveBook == nil {
		return "Brak aktywnego scenariusza"
	}
	exportDir, errMsg, ok := s.resolveOrPickBookExportDir(r)
	if !ok {
		return errMsg
	}
	fountainText := book.ExportScreenplayFountain(s.ActiveBook)
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return err.Error()
	}
	outPath := filepath.Join(exportDir, s.ActiveBook.ID+".fountain")
	if err := os.WriteFile(outPath, []byte(fountainText), 0644); err != nil {
		return err.Error()
	}
	s.revealExportedFileInOS(outPath)
	return "Wyeksportowano plik Fountain: " + outPath
}

func (s *Server) browseExportModalFolder(r *http.Request) string {
	if fmtVal := strings.ToLower(strings.TrimSpace(r.FormValue("format"))); fmtVal != "" {
		s.ExportModalFormat = fmtVal
	}
	if curDir := strings.TrimSpace(r.FormValue("export_dir")); curDir != "" {
		if expanded, err := book.ExpandLocalPath(curDir); err == nil && expanded != "" {
			s.ExportModalDir = expanded
		}
	}
	picked := strings.TrimSpace(r.FormValue("picked_dir"))
	if picked == "" {
		s.mu.Unlock()
		picked = PickNativeBookPath(true)
		s.mu.Lock()
	}
	if picked != "" {
		if expanded, err := book.ExpandLocalPath(picked); err == nil && expanded != "" {
			picked = expanded
		}
		s.ExportModalDir = picked
		s.persistActiveBookExportDir(picked)
	}
	return ""
}

func (s *Server) exportProjectAction(r *http.Request) string {
	if s.ActiveBook == nil {
		return "Brak aktywnego projektu"
	}
	format := strings.ToLower(strings.TrimSpace(r.FormValue("format")))
	if format == "" {
		format = strings.ToLower(strings.TrimSpace(s.ExportModalFormat))
	}
	if format == "" {
		if s.ActiveBook.IsScreenplay() {
			format = "pdf"
		} else {
			format = "playbook"
		}
	}
	rawDir := strings.TrimSpace(r.FormValue("export_dir"))
	if rawDir == "" {
		rawDir = strings.TrimSpace(s.ExportModalDir)
	}
	if rawDir != "" && r != nil && r.Form != nil {
		r.Form.Set("export_dir", rawDir)
	}
	var toast string
	switch format {
	case "pdf":
		toast = s.exportScreenplayPDFAction(r)
	case "fountain":
		toast = s.exportScreenplayFountainAction(r)
	default:
		toast = s.packActiveBook(r)
	}
	if strings.HasPrefix(toast, "Wyeksportowano ") || strings.HasPrefix(toast, "Spakowano ") {
		s.ExportModalOpen = false
		s.ExportModalFormat = ""
		s.ExportModalDir = ""
	}
	return toast
}

func (s *Server) browseBookExportFolder(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	picked := strings.TrimSpace(r.FormValue("export_dir"))
	if picked == "" {
		s.mu.Unlock()
		picked = PickNativeBookPath(true)
		s.mu.Lock()
	}
	if picked == "" {
		r.Form.Set("silent", "true")
		return ""
	}
	expanded, err := book.ExpandLocalPath(picked)
	if err != nil {
		return err.Error()
	}
	s.ensureHistoryBaselineBeforeMutation()
	s.persistActiveBookExportDir(expanded)
	s.reloadActiveBook()
	toast := "Ustawiono katalog eksportu: " + expanded
	s.recordHistoryAfterMutation(toast, "meta", "exportdir")
	return toast
}

func (s *Server) browseBookSaveFolder(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	picked := strings.TrimSpace(r.FormValue("save_path"))
	if picked == "" {
		s.mu.Unlock()
		picked = PickNativeBookPath(true)
		s.mu.Lock()
	}
	if picked == "" {
		r.Form.Set("silent", "true")
		return ""
	}
	expanded, err := book.ExpandLocalPath(picked)
	if err != nil {
		return err.Error()
	}
	oldRoot := s.ActiveBook.RootPath
	reloaded, err := book.MoveBookToDirectory(s.ActiveBook, expanded)
	if err != nil {
		return err.Error()
	}
	if s.Settings != nil {
		s.Settings.RemoveExternalBook(oldRoot)
		if filepath.Clean(filepath.Dir(reloaded.RootPath)) != filepath.Clean(s.BooksDir) {
			s.Settings.AddExternalBook(reloaded.RootPath)
			reloaded.IsExternal = true
		}
		s.Settings.NewBookDir = filepath.Dir(reloaded.RootPath)
		_ = s.saveAppSettings()
	}
	s.setActiveBook(reloaded)
	return "Przeniesiono projekt do: " + reloaded.RootPath
}

func (s *Server) applySluglineTokenAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	if scID == "" && s.SceneTextModalID != "" {
		scID = s.SceneTextModalID
	}
	if scID == "" {
		scID = s.SelectedSceneID
	}
	target := s.findScene(scID)
	if target == nil {
		return ""
	}
	s.ensureHistoryBaselineBeforeMutation()
	s.applySceneFormFields(target, r)

	info := book.ParseSlugline(target.Slugline)
	prefix := info.IntExt
	if prefix == "" {
		if defInfo := book.ParseSlugline(book.DefaultSluglineForStandard(s.ActiveBook.NormalizedStandard())); defInfo.IntExt != "" {
			prefix = defInfo.IntExt
		} else {
			prefix = "WN."
		}
	}
	loc := info.Location
	if loc == "" {
		if s.ActiveBook.NormalizedStandard() == "pl" {
			loc = "LOKACJA"
		} else {
			loc = "LOCATION"
		}
	}
	tod := info.TimeOfDay
	if tod == "" {
		if s.ActiveBook.NormalizedStandard() == "pl" {
			tod = "DZIEŃ"
		} else {
			tod = "DAY"
		}
	}

	if newPrefix := strings.ToUpper(strings.TrimSpace(r.FormValue("prefix"))); newPrefix != "" {
		prefix = newPrefix
	}
	if newTime := strings.ToUpper(strings.TrimSpace(r.FormValue("time"))); newTime != "" {
		tod = newTime
	}
	target.Slugline = fmt.Sprintf("%s %s - %s", prefix, loc, tod)
	return s.commitScenesMutation("Zaktualizowano nagłówek sceny: "+target.Slugline, "scene", "slugline:"+target.ID)
}

func (s *Server) insertScreenplayBlockAction(r *http.Request) string {
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	if scID == "" && s.SceneTextModalID != "" {
		scID = s.SceneTextModalID
	}
	if scID == "" {
		scID = s.SelectedSceneID
	}
	target := s.findScene(scID)
	if target == nil {
		return ""
	}
	s.ensureHistoryBaselineBeforeMutation()
	s.applySceneFormFields(target, r)

	isPL := s.ActiveBook.NormalizedStandard() == "pl"
	kind := strings.ToLower(strings.TrimSpace(r.FormValue("kind")))
	charName := strings.ToUpper(strings.TrimSpace(book.SanitizePlainText(r.FormValue("character"))))

	snippet := ""
	switch {
	case charName != "":
		snippet = "\n\n" + charName + "\nKwestia dialogowa..."
	case kind == "dialogue":
		snippet = "\n\nPOSTAĆ\nKwestia dialogowa..."
	case kind == "parenthetical":
		snippet = "\n(cicho)\n"
	case kind == "action":
		snippet = "\n\nOpis akcji..."
	case kind == "dialogue_off":
		if isPL {
			snippet = "\n\nPOSTAĆ (OFF)\nKwestia spoza kadru..."
		} else {
			snippet = "\n\nPOSTAĆ (O.S.)\nKwestia spoza kadru..."
		}
	case kind == "dialogue_vo":
		snippet = "\n\nPOSTAĆ (V.O.)\nKwestia narratora..."
	case kind == "transition_cut":
		if isPL {
			snippet = "\n\n> CIĘCIE DO:\n"
		} else {
			snippet = "\n\n> CUT TO:\n"
		}
	case kind == "transition_fade":
		if isPL {
			snippet = "\n\n> ŚCIEMNIENIE.\n"
		} else {
			snippet = "\n\n> FADE OUT.\n"
		}
	case kind == "subslug":
		if isPL {
			snippet = "\n\nWN. LOKACJA - CIĄG DALSZY\n"
		} else {
			snippet = "\n\nINT. LOCATION - CONTINUOUS\n"
		}
	}
	if snippet != "" {
		cur := strings.TrimRight(target.Text, " \t\n")
		if cur == "" {
			target.Text = strings.TrimLeft(snippet, "\n")
		} else {
			target.Text = cur + snippet
		}
		target.HTML = ""
	}
	return s.commitScenesMutation("Wstawiono element scenariusza w scenie: "+target.DisplayName(), "scene", "text:"+target.ID)
}
