package gui

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"storyteller/internal/book"
)

func (s *Server) handleEditorAction(action string, w http.ResponseWriter, r *http.Request) (toast string, wroteResponse bool, handled bool) {
	if t, ok := s.handleEditorHistoryAction(action, r); ok {
		return t, false, true
	}

	switch action {
	case "AutoSaveScene":
		_ = s.saveSceneFromFormWithMode(r, true)
		s.writeEditorHistoryHeaders(w)
		w.WriteHeader(http.StatusNoContent)
		return "", true, true

	case "SaveEditorState":
		s.saveEditorViewportAndNodePositions(r)
		s.writeEditorHistoryHeaders(w)
		w.WriteHeader(http.StatusNoContent)
		return "", true, true

	case "RenameBook":
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
					toast = "Zmieniono tytuł książki na: " + s.ActiveBook.Title
					s.recordHistoryAfterMutation(toast, "meta", "renamebook")
				}
			}
		}
		if r.FormValue("silent") == "true" {
			s.writeEditorHistoryHeaders(w)
			w.WriteHeader(http.StatusNoContent)
			return "", true, true
		}
		return toast, false, true

	case "SetStartScene":
		if _, hasSettingsOpen := r.Form["scene_settings_open"]; hasSettingsOpen {
			s.SceneTextModalSettingsOpen = r.FormValue("scene_settings_open") == "true"
		}
		toast = s.withBookMutation(r, true, func(b *book.Book) (string, string, string, bool) {
			scID := strings.TrimSpace(r.FormValue("scene"))
			if scID == "" {
				scID = strings.TrimSpace(r.FormValue("scene_id"))
			}
			if scID == "" {
				scID = s.SelectedSceneID
			}
			target := s.findScene(scID)
			if target == nil {
				return "", "", "", false
			}
			b.StartSceneID = target.ID
			b.RawStart = target.ReferenceKey()
			s.SelectedSceneID = target.ID
			if s.SceneTextModalID != "" {
				s.SceneTextModalID = target.ID
			}
			if s.Save != nil && len(s.Save.History) == 0 && s.Save.AudioTime == 0 {
				s.Save.CurrentSceneID = target.ID
				_ = book.WriteSaveState(s.SavesDir, s.Save)
			}
			return "Ustawiono scenę początkową: " + target.DisplayName(), "meta", "", true
		})
		return toast, false, true

	case "ToggleBookNarrated":
		toast = s.withBookMutation(r, false, func(b *book.Book) (string, string, string, bool) {
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
		return toast, false, true

	case "ToggleBookImageless":
		if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
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
		return toast, false, true

	case "PromptDeleteScene":
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
		return "", false, true

	case "PromptDeleteScenes":
		s.promptDeleteScenesFromForm(r)
		return "", false, true

	case "SelectStudioScene":
		if r.FormValue("scene_id") != "" {
			_ = s.saveSceneFromForm(r)
		}
		scID := strings.TrimSpace(r.FormValue("scene"))
		if found := s.findScene(scID); found != nil {
			s.SelectedSceneID = found.ID
		}
		return "", false, true

	case "InsertScene":
		if r.FormValue("scene_id") != "" {
			_ = s.saveSceneFromForm(r)
		}
		if s.ActiveBook != nil {
			s.ensureHistoryBaselineBeforeMutation()
			parentID := strings.TrimSpace(r.FormValue("parent"))
			childID := strings.TrimSpace(r.FormValue("child"))
			if parentID == "" && len(s.ActiveBook.Scenes) > 0 {
				parentID = s.ActiveBook.Scenes[len(s.ActiveBook.Scenes)-1].ID
			}
			newName := fmt.Sprintf("Nowa scena %d", len(s.ActiveBook.Scenes)+1)
			if inserted, err := book.InsertSceneBetween(s.ActiveBook, parentID, childID, newName); err == nil && inserted != nil {
				s.reloadActiveBook()
				s.SelectedSceneID = inserted.ID
				toast = "Wstawiono nową scenę: " + inserted.Name
				s.recordHistoryAfterMutation(toast, "graph", "")
			} else if err != nil {
				toast = err.Error()
			}
		}
		return toast, false, true

	case "InsertSceneBefore":
		if r.FormValue("scene_id") != "" {
			_ = s.saveSceneFromForm(r)
		}
		if s.ActiveBook != nil {
			s.ensureHistoryBaselineBeforeMutation()
			targetID := strings.TrimSpace(r.FormValue("target"))
			if targetID == "" {
				targetID = strings.TrimSpace(r.FormValue("before"))
			}
			if targetID == "" {
				targetID = strings.TrimSpace(r.FormValue("scene"))
			}
			newName := fmt.Sprintf("Nowa scena %d", len(s.ActiveBook.Scenes)+1)
			if inserted, err := book.InsertSceneBefore(s.ActiveBook, targetID, newName); err == nil && inserted != nil {
				s.reloadActiveBook()
				s.SelectedSceneID = inserted.ID
				toast = "Wstawiono nową scenę: " + inserted.Name
				s.recordHistoryAfterMutation(toast, "graph", "")
			} else if err != nil {
				toast = err.Error()
			}
		}
		return toast, false, true

	case "CreateSceneAt":
		if r.FormValue("scene_id") != "" {
			_ = s.saveSceneFromForm(r)
		}
		if s.ActiveBook == nil && !s.StandaloneMode {
			if createdBook, err := book.CreateNewBook(s.BooksDir, "Nowa Opowieść", "Autor", false); err == nil && createdBook != nil {
				s.setActiveBook(createdBook)
				_ = book.SaveAppSettings(s.SavesDir, s.Settings)
			}
		}
		if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
			s.ensureHistoryBaselineBeforeMutation()
			x, errX := strconv.Atoi(strings.TrimSpace(r.FormValue("x")))
			y, errY := strconv.Atoi(strings.TrimSpace(r.FormValue("y")))
			hasCoords := errX == nil && errY == nil
			targetCh := strings.TrimSpace(r.FormValue("chapter"))
			newName := fmt.Sprintf("Nowa scena %d", len(s.ActiveBook.Scenes)+1)
			if created, err := book.CreateStandaloneSceneInChapter(s.ActiveBook, newName, targetCh, x, y, hasCoords); err == nil && created != nil {
				s.reloadActiveBook()
				s.SelectedSceneID = created.ID
				toast = "Dodano nową scenę: " + created.Name
				s.recordHistoryAfterMutation(toast, "graph", "")
			} else if err != nil {
				toast = err.Error()
			}
		}
		return toast, false, true

	case "AutoLayoutNodes":
		if r.FormValue("scene_id") != "" {
			_ = s.saveSceneFromForm(r)
		}
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
		return toast, false, true

	case "RenameChapter":
		return s.renameActiveBookChapter(r), false, true

	case "AssignChapterToScenes":
		return s.assignChapterToScenes(r), false, true

	case "CreateChapterGroup":
		return s.createChapterGroup(r), false, true

	case "OpenSceneTextModal":
		if r.FormValue("scene_id") != "" {
			_ = s.saveSceneFromForm(r)
		}
		scID := strings.TrimSpace(r.FormValue("scene"))
		if scID == "" {
			scID = strings.TrimSpace(r.FormValue("scene_id"))
		}
		if sc := s.findScene(scID); sc != nil {
			s.SelectedSceneID = sc.ID
			s.SceneTextModalID = sc.ID
			s.SceneTextModalSettingsOpen = false
		}
		return "", false, true

	case "CloseSceneTextModal":
		if s.SceneTextModalID != "" {
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
		s.SceneTextModalSettingsOpen = false
		return "", false, true

	case "ToggleSceneTextSettings":
		s.SceneTextModalSettingsOpen = !s.SceneTextModalSettingsOpen
		return "", false, true

	case "NavigateSceneTextModal":
		return s.navigateSceneTextModal(r), false, true

	case "SaveSceneTextModal":
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
		return toast, false, true

	case "SaveScene":
		_ = s.saveSceneFromForm(r)
		return "", false, true

	case "ConnectChoice":
		return s.connectChoiceOnScene(r), false, true

	case "ConnectChapter":
		return s.connectChapterTransition(r), false, true

	case "AddChoice":
		return s.addChoiceToScene(r), false, true

	case "PromptRemoveChoice":
		return s.promptRemoveChoiceFromScene(r), false, true

	case "RemoveChoice":
		return s.removeChoiceFromScene(r), false, true

	case "BranchChoiceScene":
		return s.branchSceneFromChoice(r), false, true
	}

	return "", false, false
}

func (s *Server) saveEditorViewportAndNodePositions(r *http.Request) {
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return
	}
	scID := strings.TrimSpace(r.FormValue("scene_id"))
	batchStr := strings.TrimSpace(r.FormValue("positions"))
	hasNodeMove := scID != "" || batchStr != ""
	if hasNodeMove {
		s.ensureHistoryBaselineBeforeMutation()
	}
	if s.ActiveBook.Layout == nil {
		s.ActiveBook.Layout = make(map[string]book.NodePosition)
	}
	movedCount := 0
	lastMovedLabel := ""
	lastMovedID := ""
	applyNodeCoord := func(targetID string, x, y int) {
		if sc := s.findScene(targetID); sc != nil {
			sc.X = x
			sc.Y = y
			pos := book.NodePosition{Row: sc.Row, Col: sc.Col, X: x, Y: y, HasXY: true}
			s.ActiveBook.Layout[sc.ID] = pos
			if sc.ExplicitID != "" {
				s.ActiveBook.Layout[sc.ExplicitID] = pos
				s.ActiveBook.Layout[strings.ToLower(sc.ExplicitID)] = pos
			}
			if sc.Name != "" {
				s.ActiveBook.Layout[sc.Name] = pos
				s.ActiveBook.Layout[book.NormalizeLookupKey(sc.Name)] = pos
			}
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
	_ = book.SaveBookEditorStateToDisk(s.ActiveBook)
	if hasNodeMove && movedCount > 0 {
		moveLbl := fmt.Sprintf("Przesunięto węzły (%d)", movedCount)
		coalesceKey := "nodedrag:batch"
		if movedCount == 1 {
			moveLbl = "Przesunięto węzeł: " + lastMovedLabel
			coalesceKey = "nodedrag:" + lastMovedID
		}
		s.recordHistoryAfterMutation(moveLbl, "layout", coalesceKey)
	}
}

func (s *Server) promptDeleteScenesFromForm(r *http.Request) {
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
}

func (s *Server) navigateSceneTextModal(r *http.Request) string {
	toast := ""
	s.SceneTextModalSettingsOpen = false
	curID := strings.TrimSpace(r.FormValue("scene_id"))
	if curID == "" {
		curID = s.SceneTextModalID
	}
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
	targetID := strings.TrimSpace(r.FormValue("scene"))
	if targetID == "" {
		targetID = strings.TrimSpace(r.FormValue("target_scene"))
	}
	if choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx"))); err == nil {
		if curSc := s.findScene(curID); curSc != nil && choiceIdx >= 0 && choiceIdx < len(curSc.Choices) {
			ch := curSc.Choices[choiceIdx]
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
		s.SelectedSceneID = nextSc.ID
		s.SceneTextModalID = nextSc.ID
	} else if s.ActiveBook != nil && len(s.ActiveBook.Scenes) > 0 && targetID == "" {
		s.SelectedSceneID = s.ActiveBook.Scenes[0].ID
		s.SceneTextModalID = s.ActiveBook.Scenes[0].ID
	}
	return toast
}
