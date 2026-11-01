package gui

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"storyteller/internal/book"
)

func (s *Server) hasSceneTextModalChanges(curSc *book.Scene, r *http.Request) bool {
	if curSc == nil {
		return false
	}
	if _, hasText := r.Form["text"]; hasText {
		if book.SanitizePlainText(r.FormValue("text")) != curSc.Text {
			return true
		}
	}
	if _, hasName := r.Form["name"]; hasName {
		newName := book.SanitizePlainText(r.FormValue("name"))
		if (newName != "" || curSc.ExplicitID != "") && newName != curSc.Name {
			return true
		}
	}
	if _, hasID := r.Form["explicit_id"]; hasID {
		if book.SanitizeSceneID(r.FormValue("explicit_id")) != curSc.ExplicitID {
			return true
		}
	}
	if _, hasSlug := r.Form["slugline"]; hasSlug {
		if strings.ToUpper(strings.TrimSpace(book.SanitizePlainText(r.FormValue("slugline")))) != curSc.Slugline {
			return true
		}
	}
	if _, hasSyn := r.Form["synopsis"]; hasSyn {
		if strings.TrimSpace(book.SanitizePlainText(r.FormValue("synopsis"))) != curSc.Synopsis {
			return true
		}
	}
	if _, hasImg := r.Form["explicit_image"]; hasImg {
		curImg := curSc.ExplicitImage
		if curImg == "" && curSc.ImageFile != "" {
			curImg = curSc.ImageFile
		}
		if book.SanitizeRelativeMediaPath(r.FormValue("explicit_image")) != curImg {
			return true
		}
	}
	if s.ActiveBook != nil && s.ActiveBook.Narrated {
		if _, hasVoice := r.Form["explicit_voice"]; hasVoice {
			curVoice := curSc.ExplicitVoice
			if curVoice == "" && curSc.AudioFile != "" {
				curVoice = curSc.AudioFile
			}
			if book.SanitizeRelativeMediaPath(r.FormValue("explicit_voice")) != curVoice {
				return true
			}
		}
	}
	for i, ch := range curSc.Choices {
		lblKey := fmt.Sprintf("choice_label_%d", i)
		if _, hasLbl := r.Form[lblKey]; hasLbl {
			newLbl := book.SanitizePlainText(r.FormValue(lblKey))
			if newLbl == "" {
				newLbl = fmt.Sprintf("Wybór %d", i+1)
			}
			if newLbl != ch.Label {
				return true
			}
		}
		tgtKey := fmt.Sprintf("choice_target_%d", i)
		if _, hasTgt := r.Form[tgtKey]; hasTgt {
			newTgt := book.SanitizePlainText(r.FormValue(tgtKey))
			if newTgt != "" && !strings.EqualFold(newTgt, ch.Target) && !strings.EqualFold(newTgt, ch.TargetSceneID) {
				return true
			}
		}
	}
	if len(curSc.Choices) == 0 || curSc.ExplicitEnd {
		if _, hasTgt0 := r.Form["choice_target_0"]; hasTgt0 {
			newTgt := book.SanitizePlainText(r.FormValue("choice_target_0"))
			if newTgt != "" && !strings.EqualFold(newTgt, "END") {
				return true
			}
		}
	}
	return false
}

func (s *Server) applySceneFormFields(target *book.Scene, r *http.Request) {
	if target == nil || s.ActiveBook == nil {
		return
	}
	if formSceneID := strings.TrimSpace(r.FormValue("scene_id")); formSceneID != "" && !strings.EqualFold(formSceneID, target.ID) && !strings.EqualFold(s.SceneTextModalID, target.ID) {
		return
	}
	oldName := target.Name
	oldExplicitID := target.ExplicitID
	if _, hasID := r.Form["explicit_id"]; hasID {
		target.ExplicitID = book.SanitizeSceneID(r.FormValue("explicit_id"))
	}
	if _, hasName := r.Form["name"]; hasName {
		newName := book.SanitizePlainText(r.FormValue("name"))
		if newName != "" || target.ExplicitID != "" {
			target.Name = newName
		}
	}
	if _, hasSlug := r.Form["slugline"]; hasSlug {
		target.Slugline = strings.ToUpper(strings.TrimSpace(book.SanitizePlainText(r.FormValue("slugline"))))
	}
	if _, hasSyn := r.Form["synopsis"]; hasSyn {
		target.Synopsis = strings.TrimSpace(book.SanitizePlainText(r.FormValue("synopsis")))
	}
	if _, hasImg := r.Form["explicit_image"]; hasImg {
		target.ExplicitImage = book.SanitizeRelativeMediaPath(r.FormValue("explicit_image"))
		if target.ExplicitImage == "" {
			target.ImageFile = ""
			target.ImageURL = ""
		}
	}
	if !s.ActiveBook.Narrated {
		target.ExplicitVoice = ""
		target.AudioFile = ""
		target.AudioURL = ""
	} else if _, hasVoice := r.Form["explicit_voice"]; hasVoice {
		target.ExplicitVoice = book.SanitizeRelativeMediaPath(r.FormValue("explicit_voice"))
		if target.ExplicitVoice == "" {
			target.AudioFile = ""
			target.AudioURL = ""
		}
	}
	chapterChanged := false
	if _, hasChapter := r.Form["chapter"]; hasChapter {
		newCh := book.SanitizePlainText(r.FormValue("chapter"))
		if !strings.EqualFold(strings.TrimSpace(target.Chapter), newCh) {
			chapterChanged = true
		}
		target.Chapter = newCh
	}
	if rawSF := r.FormValue("source_file"); rawSF != "" && !chapterChanged {
		sf := filepath.Base(book.SanitizePlainText(rawSF))
		if sf != "" && sf != "." {
			target.SourceFile = sf
		}
	}
	if chapterChanged && s.ActiveBook != nil {
		if chMeta := s.ActiveBook.FindChapter(target.Chapter); chMeta != nil {
			target.ChapterID = chMeta.ExplicitID
			if chMeta.SourceFile != "" {
				target.SourceFile = chMeta.SourceFile
			}
		} else if s.ActiveBook.IsMultiFileBook() {
			target.ChapterID = book.SanitizeSceneID(target.Chapter)
			target.SourceFile = s.ActiveBook.ChapterFileNameFor(target.Chapter, target.ChapterID)
		}
	}
	if target.SourceFile == "" || target.SourceFile == "." || strings.EqualFold(target.SourceFile, "story.txt") {
		target.SourceFile = s.ActiveBook.ChapterFileNameFor(target.Chapter, target.ChapterID)
	}
	if _, hasExplicitEnd := r.Form["explicit_end"]; hasExplicitEnd {
		target.ExplicitEnd = r.FormValue("explicit_end") == "true"
	}
	if _, hasText := r.Form["text"]; hasText {
		target.Text = book.SanitizePlainText(r.FormValue("text"))
	}

	if countStr := strings.TrimSpace(r.FormValue("choice_count")); countStr != "" {
		if count, err := strconv.Atoi(countStr); err == nil && count >= 2 {
			var parsedChoices []book.Choice
			for i := 0; i < count; i++ {
				lbl := book.SanitizePlainText(r.FormValue(fmt.Sprintf("choice_label_%d", i)))
				tgt := book.SanitizePlainText(r.FormValue(fmt.Sprintf("choice_target_%d", i)))
				if tgt == "" && i < len(target.Choices) {
					tgt = target.Choices[i].Target
				}
				if lbl == "" && tgt != "" {
					lbl = fmt.Sprintf("Wybór %d", i+1)
				}
				if lbl != "" && tgt != "" {
					parsedChoices = append(parsedChoices, book.Choice{
						Label:    lbl,
						Target:   tgt,
						IsEnding: strings.EqualFold(tgt, "END"),
					})
				}
			}
			target.Choices = parsedChoices
			if len(parsedChoices) > 0 {
				target.ExplicitEnd = false
				target.ExplicitNext = ""
			}
		}
	} else if _, hasLines := r.Form["choices_lines"]; hasLines {
		var parsedChoices []book.Choice
		rawChoices := strings.ReplaceAll(r.FormValue("choices_lines"), "\r\n", "\n")
		for _, line := range strings.Split(rawChoices, "\n") {
			trimmed := strings.TrimSpace(line)
			if idx := strings.Index(trimmed, " -> "); idx > 0 {
				lbl := book.SanitizePlainText(trimmed[:idx])
				tgt := book.SanitizePlainText(trimmed[idx+4:])
				if lbl != "" && tgt != "" {
					parsedChoices = append(parsedChoices, book.Choice{
						Label:    lbl,
						Target:   tgt,
						IsEnding: strings.EqualFold(tgt, "END"),
					})
				}
			}
		}
		target.Choices = parsedChoices
	} else if len(target.Choices) >= 2 {
		for i := range target.Choices {
			lblKey := fmt.Sprintf("choice_label_%d", i)
			if _, hasLbl := r.Form[lblKey]; hasLbl {
				lbl := book.SanitizePlainText(r.FormValue(lblKey))
				if lbl == "" {
					lbl = fmt.Sprintf("Wybór %d", i+1)
				}
				target.Choices[i].Label = lbl
			}
			tgtKey := fmt.Sprintf("choice_target_%d", i)
			if _, hasTgt := r.Form[tgtKey]; hasTgt {
				if tgt := book.SanitizePlainText(r.FormValue(tgtKey)); tgt != "" {
					target.Choices[i].Target = tgt
					target.Choices[i].IsEnding = strings.EqualFold(tgt, "END")
				}
			}
		}
	} else if _, hasTgt0 := r.Form["choice_target_0"]; hasTgt0 {
		if tgt := book.SanitizePlainText(r.FormValue("choice_target_0")); tgt != "" {
			if strings.EqualFold(tgt, "END") {
				target.Choices = nil
				target.ExplicitNext = ""
				target.ExplicitEnd = true
			} else {
				target.ExplicitEnd = false
				target.ExplicitNext = tgt
				target.Choices = []book.Choice{
					{
						Label:      "Dalej",
						Target:     tgt,
						IsImplicit: true,
					},
				}
			}
		}
	}

	if !strings.EqualFold(oldName, target.Name) || (oldExplicitID != "" && !strings.EqualFold(oldExplicitID, target.ExplicitID)) {
		newRef := target.ReferenceKey()
		if s.ActiveBook.Layout != nil && oldName != "" && !strings.EqualFold(oldName, target.Name) && target.Name != "" {
			if pos, ok := s.ActiveBook.Layout[oldName]; ok {
				s.ActiveBook.Layout[target.Name] = pos
				s.ActiveBook.Layout[book.NormalizeLookupKey(target.Name)] = pos
			} else if pos, ok := s.ActiveBook.Layout[book.NormalizeLookupKey(oldName)]; ok {
				s.ActiveBook.Layout[target.Name] = pos
				s.ActiveBook.Layout[book.NormalizeLookupKey(target.Name)] = pos
			}
			if s.ActiveBook.HasEditorView {
				_ = book.SaveBookEditorStateToDisk(s.ActiveBook)
			}
		}
		if strings.EqualFold(s.ActiveBook.StartSceneID, target.ID) ||
			(oldName != "" && strings.EqualFold(s.ActiveBook.RawStart, oldName)) ||
			(oldExplicitID != "" && strings.EqualFold(s.ActiveBook.RawStart, oldExplicitID)) {
			s.ActiveBook.RawStart = newRef
			s.ActiveBook.StartSceneID = target.ID
		}
		for i := range s.ActiveBook.Chapters {
			ch := &s.ActiveBook.Chapters[i]
			if strings.EqualFold(ch.StartSceneID, target.ID) ||
				(oldName != "" && strings.EqualFold(ch.ExplicitStart, oldName)) ||
				(oldExplicitID != "" && strings.EqualFold(ch.ExplicitStart, oldExplicitID)) {
				if ch.ExplicitStart != "" {
					ch.ExplicitStart = newRef
				}
				ch.StartSceneID = target.ID
			}
		}
		for _, other := range s.ActiveBook.Scenes {
			if oldName != "" && !strings.EqualFold(oldName, target.Name) && strings.EqualFold(other.ExplicitNext, oldName) {
				other.ExplicitNext = newRef
			} else if oldExplicitID != "" && !strings.EqualFold(oldExplicitID, target.ExplicitID) && strings.EqualFold(other.ExplicitNext, oldExplicitID) {
				other.ExplicitNext = newRef
			}
			for i := range other.Choices {
				if !strings.EqualFold(other.Choices[i].Target, "Next") {
					if oldName != "" && !strings.EqualFold(oldName, target.Name) && strings.EqualFold(other.Choices[i].Target, oldName) {
						other.Choices[i].Target = newRef
					} else if oldExplicitID != "" && !strings.EqualFold(oldExplicitID, target.ExplicitID) && strings.EqualFold(other.Choices[i].Target, oldExplicitID) {
						other.Choices[i].Target = newRef
					}
				}
			}
		}
	}
	if r.FormValue("is_start_scene") == "true" {
		s.ActiveBook.StartSceneID = target.ID
		s.ActiveBook.RawStart = target.ReferenceKey()
	}
}

func (s *Server) materializeChoicesForEdit(sc *book.Scene) []book.Choice {
	var list []book.Choice
	for _, ch := range sc.Choices {
		tgt := ch.Target
		if ch.IsImplicit && !ch.IsChapterLink && (strings.EqualFold(tgt, "Next") || strings.EqualFold(tgt, "Dalej") || tgt == "") {
			if tSc := s.findScene(ch.TargetSceneID); tSc != nil {
				tgt = tSc.ReferenceKey()
			}
		}
		lbl := ch.Label
		if lbl == "" {
			lbl = "Dalej"
		}
		if tgt != "" {
			list = append(list, book.Choice{
				Label:           lbl,
				Target:          tgt,
				TargetSceneID:   ch.TargetSceneID,
				TargetChapterID: ch.TargetChapterID,
				IsChapterLink:   ch.IsChapterLink,
				IsEnding:        ch.IsEnding || strings.EqualFold(tgt, "END"),
			})
		}
	}
	return list
}

func (s *Server) addChoiceToScene(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if _, hasSettingsOpen := r.Form["scene_settings_open"]; hasSettingsOpen {
		s.SceneTextModalSettingsOpen = r.FormValue("scene_settings_open") == "true"
	}
	if s.ActiveBook == nil {
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	target := s.findScene(scID)
	if target == nil {
		return "Nie znaleziono sceny"
	}

	s.ensureHistoryBaselineBeforeMutation()
	s.applySceneFormFields(target, r)
	choices := s.materializeChoicesForEdit(target)

	if len(choices) == 1 && (choices[0].Label == "" || strings.EqualFold(choices[0].Label, "Dalej")) {
		choices[0].Label = "Wybór 1"
	}
	if len(choices) < 2 {
		for len(choices) < 2 {
			newLabel := fmt.Sprintf("Wybór %d", len(choices)+1)
			choices = append(choices, book.Choice{
				Label:         newLabel,
				Target:        "END",
				TargetSceneID: "END",
				IsEnding:      true,
			})
		}
	} else {
		newLabel := fmt.Sprintf("Wybór %d", len(choices)+1)
		choices = append(choices, book.Choice{
			Label:         newLabel,
			Target:        "END",
			TargetSceneID: "END",
			IsEnding:      true,
		})
	}
	target.Choices = choices
	target.ExplicitNext = ""
	target.ExplicitEnd = false

	book.FinalizeBook(s.ActiveBook)
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	s.SelectedSceneID = target.ID
	if s.SceneTextModalID != "" {
		s.SceneTextModalID = target.ID
	}
	msg := "Dodano nowy wybór do sceny: " + target.DisplayName()
	s.recordHistoryAfterMutation(msg, "graph", "")
	return msg
}

func (s *Server) promptRemoveChoiceFromScene(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if _, hasSettingsOpen := r.Form["scene_settings_open"]; hasSettingsOpen {
		s.SceneTextModalSettingsOpen = r.FormValue("scene_settings_open") == "true"
	}
	if s.ActiveBook == nil {
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	if scID == "" {
		scID = s.SceneTextModalID
	}
	if scID == "" {
		scID = s.SelectedSceneID
	}
	target := s.findScene(scID)
	if target == nil {
		return ""
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx")))
	if err != nil {
		return ""
	}

	if s.SceneTextModalID != "" {
		if s.hasSceneTextModalChanges(target, r) && !s.ActiveBook.IsPacked {
			s.ensureHistoryBaselineBeforeMutation()
			s.applySceneFormFields(target, r)
			book.FinalizeBook(s.ActiveBook)
			targetID := target.ID
			targetLabel := target.DisplayName()
			if saveErr := book.SaveBookScenesToDisk(s.ActiveBook); saveErr == nil {
				s.reloadActiveBook()
				s.SelectedSceneID = targetID
				s.SceneTextModalID = targetID
				s.recordHistoryAfterMutation("Edycja treści sceny: "+targetLabel, "text", "textmodal:"+targetID)
				target = s.findScene(targetID)
			}
		}
	} else if r.FormValue("scene_id") != "" && !s.ActiveBook.IsPacked {
		_ = s.saveSceneFromForm(r)
		target = s.findScene(scID)
		if target == nil {
			target = s.findScene(s.SelectedSceneID)
		}
	}
	if target == nil || choiceIdx < 0 || choiceIdx >= len(target.Choices) {
		return ""
	}
	chLabel := strings.TrimSpace(target.Choices[choiceIdx].Label)
	if chLabel == "" {
		chLabel = fmt.Sprintf("Wybór %d", choiceIdx+1)
	}
	s.ConfirmTitle = "Usuń wybór"
	s.ConfirmMessage = fmt.Sprintf("Czy na pewno chcesz usunąć wybór „%s” ze sceny „%s”?", chLabel, target.DisplayName())
	s.ConfirmPendingAction = "RemoveChoice"
	s.ConfirmPendingParam = fmt.Sprintf("%s:%d", target.ID, choiceIdx)
	return ""
}

func (s *Server) removeChoiceFromScene(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil {
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	target := s.findScene(scID)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx")))
	if err != nil {
		return ""
	}

	s.ensureHistoryBaselineBeforeMutation()
	s.applySceneFormFields(target, r)
	return s.removeChoiceOnTarget(target, choiceIdx)
}

func (s *Server) removeChoiceBySceneAndIndex(scID string, choiceIdx int) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil {
		return ""
	}
	target := s.findScene(scID)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	s.ensureHistoryBaselineBeforeMutation()
	return s.removeChoiceOnTarget(target, choiceIdx)
}

func (s *Server) removeChoiceOnTarget(target *book.Scene, choiceIdx int) string {
	choices := s.materializeChoicesForEdit(target)
	if choiceIdx >= 0 && choiceIdx < len(choices) {
		choices = append(choices[:choiceIdx], choices[choiceIdx+1:]...)
	}
	if len(choices) == 1 {
		if choices[0].IsEnding || strings.EqualFold(choices[0].Target, "END") {
			target.Choices = nil
			target.ExplicitNext = ""
			target.ExplicitEnd = true
		} else {
			choices[0].Label = "Dalej"
			choices[0].IsImplicit = true
			target.ExplicitNext = choices[0].Target
			target.Choices = choices
			target.ExplicitEnd = false
		}
	} else if len(choices) == 0 {
		target.Choices = nil
		target.ExplicitNext = ""
		target.ExplicitEnd = true
	} else {
		target.Choices = choices
		target.ExplicitNext = ""
		target.ExplicitEnd = false
	}

	book.FinalizeBook(s.ActiveBook)
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	s.SelectedSceneID = target.ID
	if s.SceneTextModalID != "" {
		s.SceneTextModalID = target.ID
	}
	msg := "Usunięto wybór ze sceny: " + target.DisplayName()
	s.recordHistoryAfterMutation(msg, "graph", "")
	return msg
}

func (s *Server) branchSceneFromChoice(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil {
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	target := s.findScene(scID)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx")))
	if err != nil {
		choiceIdx = -1
	}

	s.ensureHistoryBaselineBeforeMutation()
	s.applySceneFormFields(target, r)
	choices := s.materializeChoicesForEdit(target)

	newName := fmt.Sprintf("Nowa scena %d", len(s.ActiveBook.Scenes)+1)
	newScene := &book.Scene{
		Name:       newName,
		Chapter:    target.Chapter,
		Text:       "Nowy fragment opowieści...",
		SourceFile: target.SourceFile,
	}

	if choiceIdx >= 0 && choiceIdx < len(choices) {
		choices[choiceIdx].Target = newScene.Name
		choices[choiceIdx].IsEnding = false
		if strings.TrimSpace(choices[choiceIdx].Label) == "" {
			choices[choiceIdx].Label = "Dalej"
		}
	} else {
		choices = append(choices, book.Choice{
			Label:  fmt.Sprintf("Wybór %d", len(choices)+1),
			Target: newScene.Name,
		})
	}
	target.Choices = choices
	target.ExplicitEnd = false
	if len(choices) == 1 {
		target.ExplicitNext = newScene.Name
	}

	parentIdx := len(s.ActiveBook.Scenes) - 1
	for i, sc := range s.ActiveBook.Scenes {
		if sc.ID == target.ID {
			parentIdx = i
			break
		}
	}
	insertPos := parentIdx + 1
	s.ActiveBook.Scenes = append(s.ActiveBook.Scenes[:insertPos], append([]*book.Scene{newScene}, s.ActiveBook.Scenes[insertPos:]...)...)

	book.FinalizeBook(s.ActiveBook)
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	if s.SceneTextModalID != "" {
		s.SelectedSceneID = target.ID
		s.SceneTextModalID = target.ID
	} else {
		s.SelectedSceneID = newScene.ID
	}
	msg := "Utworzono scenę docelową: " + newScene.Name
	s.recordHistoryAfterMutation(msg, "graph", "")
	return msg
}

func (s *Server) saveSceneFromForm(r *http.Request) string {
	return s.saveSceneFromFormWithMode(r, false)
}

func (s *Server) saveSceneFromFormWithMode(r *http.Request, isAutoSave bool) string {
	if s.isHistoryPreview() {
		if isAutoSave {
			return ""
		}
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil {
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene_id"))
	target := s.findScene(scID)
	if target == nil && s.SceneTextModalID != "" {
		target = s.findScene(s.SceneTextModalID)
	}
	if target == nil {
		return "Nie znaleziono sceny"
	}

	s.ensureHistoryBaselineBeforeMutation()
	s.applySceneFormFields(target, r)

	book.FinalizeBook(s.ActiveBook)
	newID := target.ID
	targetLabel := target.DisplayName()
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	if newID != "" {
		s.SelectedSceneID = newID
		if s.SceneTextModalID != "" {
			s.SceneTextModalID = newID
		}
	}
	coalesceKey := ""
	if newID != "" {
		coalesceKey = "autosave:" + newID
	}
	s.recordHistoryAfterMutation("Edycja sceny: "+targetLabel, "scene", coalesceKey)
	return "Zapisano zmiany w pliku " + target.SourceFile
}

func (s *Server) deleteSceneByID(sceneID string) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || len(s.ActiveBook.Scenes) <= 1 {
		return "Książka musi zawierać przynajmniej jedną scenę"
	}
	delIdx := -1
	var delName string
	var delLabel string
	for i, sc := range s.ActiveBook.Scenes {
		if sc.ID == sceneID {
			delIdx = i
			delName = sc.Name
			delLabel = sc.DisplayName()
			break
		}
	}
	if delIdx < 0 {
		return ""
	}
	s.ensureHistoryBaselineBeforeMutation()
	s.ActiveBook.Scenes = append(s.ActiveBook.Scenes[:delIdx], s.ActiveBook.Scenes[delIdx+1:]...)
	delete(s.ActiveBook.Layout, sceneID)

	if strings.EqualFold(s.ActiveBook.StartSceneID, sceneID) || (delName != "" && strings.EqualFold(s.ActiveBook.RawStart, delName)) {
		s.ActiveBook.RawStart = s.ActiveBook.Scenes[0].ReferenceKey()
		s.ActiveBook.StartSceneID = s.ActiveBook.Scenes[0].ID
	}
	for i := range s.ActiveBook.Chapters {
		ch := &s.ActiveBook.Chapters[i]
		if strings.EqualFold(ch.StartSceneID, sceneID) ||
			strings.EqualFold(ch.ExplicitStart, sceneID) ||
			(delName != "" && strings.EqualFold(ch.ExplicitStart, delName)) {
			ch.ExplicitStart = ""
		}
	}

	for _, sc := range s.ActiveBook.Scenes {
		var kept []book.Choice
		for _, ch := range sc.Choices {
			if !ch.IsImplicit && (ch.TargetSceneID == sceneID || (delName != "" && strings.EqualFold(ch.Target, delName))) {
				continue
			}
			kept = append(kept, ch)
		}
		sc.Choices = kept
	}

	book.FinalizeBook(s.ActiveBook)
	_ = book.SaveBookScenesToDisk(s.ActiveBook)
	s.reloadActiveBook()
	if len(s.ActiveBook.Scenes) > 0 {
		s.SelectedSceneID = s.ActiveBook.Scenes[0].ID
	}
	msg := "Usunięto scenę: " + delLabel
	s.recordHistoryAfterMutation(msg, "graph", "")
	return msg
}

func (s *Server) deleteScenesByIDs(rawIDs string) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || len(s.ActiveBook.Scenes) == 0 {
		return ""
	}
	toDeleteIDs := make(map[string]bool)
	toDeleteExplicitIDs := make(map[string]bool)
	toDeleteNames := make(map[string]bool)
	var deletedScenes []*book.Scene

	for _, part := range strings.Split(rawIDs, ",") {
		idPart := strings.TrimSpace(part)
		if idPart == "" {
			continue
		}
		if sc := s.findScene(idPart); sc != nil && !toDeleteIDs[sc.ID] {
			toDeleteIDs[sc.ID] = true
			if sc.ExplicitID != "" {
				toDeleteExplicitIDs[strings.ToLower(strings.TrimSpace(sc.ExplicitID))] = true
			}
			if sc.Name != "" {
				toDeleteNames[book.NormalizeLookupKey(sc.Name)] = true
			}
			deletedScenes = append(deletedScenes, sc)
		}
	}
	if len(deletedScenes) == 0 {
		return ""
	}
	if len(deletedScenes) == 1 {
		return s.deleteSceneByID(deletedScenes[0].ID)
	}

	s.ensureHistoryBaselineBeforeMutation()
	firstChapter := s.ActiveBook.Scenes[0].Chapter
	firstChapterID := s.ActiveBook.Scenes[0].ChapterID
	firstSourceFile := s.ActiveBook.Scenes[0].SourceFile
	if firstSourceFile == "" || strings.EqualFold(firstSourceFile, "story.txt") {
		firstSourceFile = s.ActiveBook.ChapterFileNameFor(firstChapter, firstChapterID)
	}

	var remaining []*book.Scene
	for _, sc := range s.ActiveBook.Scenes {
		if !toDeleteIDs[sc.ID] {
			remaining = append(remaining, sc)
		}
	}

	for _, sc := range deletedScenes {
		delete(s.ActiveBook.Layout, sc.ID)
		if sc.ExplicitID != "" {
			delete(s.ActiveBook.Layout, sc.ExplicitID)
			delete(s.ActiveBook.Layout, strings.ToLower(sc.ExplicitID))
		}
		if sc.Name != "" {
			delete(s.ActiveBook.Layout, sc.Name)
			delete(s.ActiveBook.Layout, book.NormalizeLookupKey(sc.Name))
		}
	}

	if len(remaining) == 0 {
		fallback := &book.Scene{
			Name:        "Nowa scena 1",
			Chapter:     firstChapter,
			ChapterID:   firstChapterID,
			Text:        "Nowy fragment opowieści...",
			SourceFile:  firstSourceFile,
			ExplicitEnd: true,
		}
		remaining = []*book.Scene{fallback}
	}

	s.ActiveBook.Scenes = remaining

	for _, sc := range s.ActiveBook.Scenes {
		if sc.ExplicitNext != "" {
			nextTrim := strings.TrimSpace(sc.ExplicitNext)
			nextLower := strings.ToLower(nextTrim)
			nextNorm := book.NormalizeLookupKey(nextTrim)
			if toDeleteIDs[nextTrim] || (nextLower != "" && toDeleteExplicitIDs[nextLower]) || (nextNorm != "" && toDeleteNames[nextNorm]) {
				sc.ExplicitNext = ""
			}
		}
		var kept []book.Choice
		for _, ch := range sc.Choices {
			tgtTrim := strings.TrimSpace(ch.Target)
			tgtLower := strings.ToLower(tgtTrim)
			tgtNorm := book.NormalizeLookupKey(tgtTrim)
			if !ch.IsImplicit && (toDeleteIDs[ch.TargetSceneID] || toDeleteIDs[tgtTrim] || (tgtLower != "" && toDeleteExplicitIDs[tgtLower]) || (tgtNorm != "" && toDeleteNames[tgtNorm])) {
				continue
			}
			kept = append(kept, ch)
		}
		sc.Choices = kept
	}

	if rawStart := strings.TrimSpace(s.ActiveBook.RawStart); rawStart != "" {
		rawLower := strings.ToLower(rawStart)
		rawNorm := book.NormalizeLookupKey(rawStart)
		if toDeleteIDs[s.ActiveBook.StartSceneID] || toDeleteIDs[rawStart] || toDeleteExplicitIDs[rawLower] || (rawNorm != "" && toDeleteNames[rawNorm]) {
			s.ActiveBook.RawStart = s.ActiveBook.Scenes[0].ReferenceKey()
			s.ActiveBook.StartSceneID = s.ActiveBook.Scenes[0].ID
		}
	}

	book.FinalizeBook(s.ActiveBook)
	_ = book.SaveBookScenesToDisk(s.ActiveBook)
	s.reloadActiveBook()
	if len(s.ActiveBook.Scenes) > 0 {
		s.SelectedSceneID = s.ActiveBook.Scenes[0].ID
	}
	msg := fmt.Sprintf("Usunięto zaznaczone sceny (%d)", len(deletedScenes))
	s.recordHistoryAfterMutation(msg, "graph", "")
	return msg
}
