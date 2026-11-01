package gui

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"storyteller/internal/book"
)

func (s *Server) renameActiveBookChapter(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	oldCh := book.SanitizePlainText(r.FormValue("old_chapter"))
	newCh := book.SanitizePlainText(r.FormValue("new_chapter"))
	if oldCh == "" || newCh == "" || oldCh == newCh {
		return ""
	}
	return s.withBookMutation(r, false, func(b *book.Book) (string, string, string, bool) {
		oldRef := book.SanitizeSceneID(oldCh)
		newRef := book.SanitizeSceneID(newCh)
		for i := range b.Chapters {
			if strings.EqualFold(strings.TrimSpace(b.Chapters[i].Name), oldCh) {
				oldRef = b.Chapters[i].ReferenceKey()
				b.Chapters[i].Name = newCh
				if b.Chapters[i].ExplicitID != "" {
					newRef = b.Chapters[i].ExplicitID
				} else if newRef != "" {
					b.Chapters[i].ID = newRef
				}
			}
		}
		updated := 0
		for _, sc := range b.Scenes {
			scCh := strings.TrimSpace(sc.Chapter)
			if scCh == "" {
				scCh = "Rozdział 1"
			}
			if strings.EqualFold(scCh, oldCh) {
				sc.Chapter = newCh
				if sc.ChapterID == "" || strings.EqualFold(sc.ChapterID, oldRef) {
					sc.ChapterID = newRef
				}
				sc.SourceFile = b.ChapterFileNameFor(sc.Chapter, sc.ChapterID)
				updated++
			}
			if oldRef != "" && newRef != "" && !strings.EqualFold(oldRef, newRef) {
				if chRef, ok := book.ParseChapterTarget(sc.ExplicitNext); ok && (strings.EqualFold(chRef, oldRef) || strings.EqualFold(chRef, oldCh)) {
					sc.ExplicitNext = "chapter:" + newRef
				}
				for j := range sc.Choices {
					if chRef, ok := book.ParseChapterTarget(sc.Choices[j].Target); ok && (strings.EqualFold(chRef, oldRef) || strings.EqualFold(chRef, oldCh)) {
						sc.Choices[j].Target = "chapter:" + newRef
						sc.Choices[j].TargetChapterID = newRef
					}
				}
			}
		}
		if updated == 0 {
			return "", "", "", false
		}
		if strings.EqualFold(s.SelectedChapter, oldCh) {
			s.SelectedChapter = newCh
		}
		if b.IsScreenplay() {
			return fmt.Sprintf("Zmieniono akt na „%s” (%d scen)", newCh, updated), "chapter", "renamechapter", true
		}
		return fmt.Sprintf("Zmieniono rozdział na „%s” (%d scen)", newCh, updated), "chapter", "renamechapter", true
	})
}

func (s *Server) setChapterStartScene(r *http.Request) string {
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	chKey := strings.TrimSpace(r.FormValue("chapter"))
	if chKey == "" {
		chKey = strings.TrimSpace(r.FormValue("old_chapter"))
	}
	if chKey == "" {
		chKey = s.SelectedChapter
	}
	scKey := strings.TrimSpace(r.FormValue("scene"))
	if scKey == "" {
		scKey = strings.TrimSpace(r.FormValue("start_scene"))
	}
	if chKey == "" || scKey == "" {
		return ""
	}
	return s.withBookMutation(r, false, func(b *book.Book) (string, string, string, bool) {
		ch := b.FindChapter(chKey)
		target := s.findScene(scKey)
		if ch == nil || target == nil {
			return "", "", "", false
		}
		targetCh := strings.TrimSpace(target.Chapter)
		if targetCh == "" {
			targetCh = "Rozdział 1"
		}
		if !strings.EqualFold(targetCh, strings.TrimSpace(ch.Name)) {
			return "", "", "", false
		}
		ch.ExplicitStart = target.ReferenceKey()
		ch.StartSceneID = target.ID
		s.SelectedChapter = ch.Name
		s.BookSettingsOpen = false
		s.EditorBreakdownPanelOpen = false
		s.EditorHistoryPanelOpen = false
		s.EditorHelpModalOpen = false
		if b.IsScreenplay() {
			return fmt.Sprintf("Ustawiono scenę początkową aktu „%s”: %s", ch.DisplayName(), target.DisplayName()), "chapter", "setchapterstart", true
		}
		return fmt.Sprintf("Ustawiono scenę początkową rozdziału „%s”: %s", ch.DisplayName(), target.DisplayName()), "chapter", "setchapterstart", true
	})
}

func (s *Server) saveChapterSettings(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	toast := s.renameActiveBookChapter(r)
	if startSc := strings.TrimSpace(r.FormValue("start_scene")); startSc != "" {
		if t2 := s.setChapterStartScene(r); t2 != "" {
			toast = t2
		}
	}
	return toast
}

func (s *Server) assignChapterToScenes(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	targetCh := book.SanitizePlainText(r.FormValue("chapter"))
	rawIDs := strings.TrimSpace(r.FormValue("scene_ids"))
	if targetCh == "" || rawIDs == "" {
		return ""
	}
	return s.withBookMutation(r, false, func(b *book.Book) (string, string, string, bool) {
		var targetChID, targetSF string
		if chMeta := b.FindChapter(targetCh); chMeta != nil {
			targetChID = chMeta.ExplicitID
			targetSF = chMeta.SourceFile
		}
		if targetSF == "" && b.IsMultiFileBook() {
			targetSF = b.ChapterFileNameFor(targetCh, targetChID)
		}
		updated := 0
		for _, idPart := range strings.Split(rawIDs, ",") {
			idPart = strings.TrimSpace(idPart)
			if sc := s.findScene(idPart); sc != nil {
				sc.Chapter = targetCh
				sc.ChapterID = targetChID
				if targetSF != "" {
					sc.SourceFile = targetSF
				}
				updated++
			}
		}
		if updated == 0 {
			return "", "", "", false
		}
		if b.IsScreenplay() {
			return fmt.Sprintf("Przypisano %d scen do aktu „%s”", updated, targetCh), "chapter", "", true
		}
		return fmt.Sprintf("Przypisano %d scen do rozdziału „%s”", updated, targetCh), "chapter", "", true
	})
}

func (s *Server) createChapterGroup(r *http.Request) string {
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
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	s.ensureHistoryBaselineBeforeMutation()
	chName := book.SanitizePlainText(r.FormValue("chapter"))
	if chName == "" {
		if s.ActiveBook.IsScreenplay() {
			chName = fmt.Sprintf("Akt %d", len(s.ActiveBook.Chapters)+1)
		} else {
			chName = fmt.Sprintf("Rozdział %d", len(s.ActiveBook.Chapters)+1)
		}
	}
	x, errX := strconv.Atoi(strings.TrimSpace(r.FormValue("x")))
	y, errY := strconv.Atoi(strings.TrimSpace(r.FormValue("y")))
	hasCoords := errX == nil && errY == nil
	if !hasCoords && len(s.ActiveBook.Scenes) > 0 {
		maxY := s.ActiveBook.Scenes[0].Y
		for _, sc := range s.ActiveBook.Scenes {
			if sc.Y > maxY {
				maxY = sc.Y
			}
		}
		x = 768
		y = book.SnapToGrid(maxY + 552)
		hasCoords = true
	}
	newName := book.NextUniqueSceneName(s.ActiveBook)
	created, err := book.CreateStandaloneSceneInChapter(s.ActiveBook, newName, chName, x, y, hasCoords)
	if err != nil {
		return err.Error()
	}
	if created == nil {
		return ""
	}
	s.reloadActiveBook()
	if found := s.findScene(created.Name); found != nil {
		s.SelectedSceneID = found.ID
	} else {
		s.SelectedSceneID = created.ID
	}
	toast := fmt.Sprintf("Utworzono rozdział „%s”", chName)
	if s.ActiveBook != nil && s.ActiveBook.IsScreenplay() {
		toast = fmt.Sprintf("Utworzono akt „%s”", chName)
	}
	s.recordHistoryAfterMutation(toast, "chapter", "")
	return toast
}

func applyChapterLinkToScene(fromSc *book.Scene, idx int, targetCh *book.Chapter) (changed bool, toast string) {
	targetRef := "chapter:" + targetCh.ReferenceKey()
	if len(fromSc.Choices) <= 1 {
		fromSc.ExplicitEnd = false
		fromSc.ExplicitNext = targetRef
		fromSc.Choices = []book.Choice{
			{
				Label:           "Dalej",
				Target:          targetRef,
				TargetSceneID:   targetCh.StartSceneID,
				TargetChapterID: targetCh.ReferenceKey(),
				IsChapterLink:   true,
				IsImplicit:      true,
			},
		}
		return true, "Połączono z rozdziałem: " + targetCh.DisplayName()
	}
	if idx >= 0 && idx < len(fromSc.Choices) {
		fromSc.ExplicitEnd = false
		fromSc.Choices[idx].Target = targetRef
		fromSc.Choices[idx].TargetSceneID = targetCh.StartSceneID
		fromSc.Choices[idx].TargetChapterID = targetCh.ReferenceKey()
		fromSc.Choices[idx].IsChapterLink = true
		fromSc.Choices[idx].IsImplicit = false
		fromSc.Choices[idx].IsEnding = false
		if strings.TrimSpace(fromSc.Choices[idx].Label) == "" {
			fromSc.Choices[idx].Label = "Przejdź do: " + targetCh.DisplayName()
		}
		return true, "Połączono z rozdziałem: " + targetCh.DisplayName()
	}
	return false, ""
}

func (s *Server) connectChoiceOnScene(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	fromID := strings.TrimSpace(r.FormValue("from_scene"))
	if fromID == "" {
		fromID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	toID := strings.TrimSpace(r.FormValue("to_scene"))
	if toChParam := strings.TrimSpace(r.FormValue("to_chapter")); toChParam != "" && toID == "" {
		toID = "chapter:" + toChParam
	}
	idx, errIdx := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx")))
	fromSc := s.findScene(fromID)
	if errIdx != nil || fromSc == nil {
		return ""
	}

	s.ensureHistoryBaselineBeforeMutation()
	if r.FormValue("scene_id") != "" {
		s.applySceneFormFields(fromSc, r)
	}
	changed := false
	toast := ""

	if strings.EqualFold(toID, "END") {
		if len(fromSc.Choices) <= 1 {
			fromSc.Choices = nil
			fromSc.ExplicitNext = ""
			fromSc.ExplicitEnd = true
			changed = true
			toast = "Ustawiono scenę jako końcową (END)"
		} else if idx >= 0 && idx < len(fromSc.Choices) {
			fromSc.Choices[idx].Target = "END"
			fromSc.Choices[idx].TargetSceneID = "END"
			fromSc.Choices[idx].TargetChapterID = ""
			fromSc.Choices[idx].IsChapterLink = false
			fromSc.Choices[idx].IsEnding = true
			changed = true
			toast = "Połączono z: KONIEC (END)"
		}
	} else if chRef, isChTarget := book.ParseChapterTarget(toID); isChTarget {
		if chRef == "__next__" {
			for _, candCh := range s.ActiveBook.Chapters {
				if ok, _ := s.ActiveBook.CanLinkChapters(fromSc.Chapter, candCh.ReferenceKey()); ok {
					chRef = candCh.ReferenceKey()
					break
				}
			}
			if chRef == "__next__" {
				return "Utwórz najpierw kolejny rozdział (+ Rozdział), aby wyprowadzić przejście z rozdziału"
			}
		}
		targetCh := s.ActiveBook.FindChapter(chRef)
		if targetCh == nil {
			return "Nie znaleziono docelowego rozdziału: " + chRef
		}
		if ok, reason := s.ActiveBook.CanLinkChapters(fromSc.Chapter, targetCh.ReferenceKey()); !ok {
			return reason
		}
		changed, toast = applyChapterLinkToScene(fromSc, idx, targetCh)
	} else if toSc := s.findScene(toID); toSc != nil {
		if !strings.EqualFold(strings.TrimSpace(toSc.Chapter), strings.TrimSpace(fromSc.Chapter)) {
			targetCh := s.ActiveBook.FindChapter(toSc.Chapter)
			if targetCh == nil || !strings.EqualFold(toSc.ID, targetCh.StartSceneID) {
				return "Przejście między rozdziałami może prowadzić wyłącznie do początku kolejnego rozdziału"
			}
			if ok, reason := s.ActiveBook.CanLinkChapters(fromSc.Chapter, targetCh.ReferenceKey()); !ok {
				return reason
			}
			changed, toast = applyChapterLinkToScene(fromSc, idx, targetCh)
		} else {
			targetRef := toSc.ReferenceKey()
			if len(fromSc.Choices) <= 1 {
				fromSc.ExplicitEnd = false
				fromSc.ExplicitNext = targetRef
				fromSc.Choices = []book.Choice{
					{
						Label:         "Dalej",
						Target:        targetRef,
						TargetSceneID: toSc.ID,
						IsImplicit:    true,
					},
				}
				changed = true
				toast = "Połączono następny krok z: " + toSc.DisplayName()
			} else if idx >= 0 && idx < len(fromSc.Choices) {
				fromSc.ExplicitEnd = false
				fromSc.Choices[idx].Target = targetRef
				fromSc.Choices[idx].TargetSceneID = toSc.ID
				fromSc.Choices[idx].TargetChapterID = ""
				fromSc.Choices[idx].IsChapterLink = false
				fromSc.Choices[idx].IsImplicit = false
				fromSc.Choices[idx].IsEnding = false
				if strings.TrimSpace(fromSc.Choices[idx].Label) == "" {
					fromSc.Choices[idx].Label = "Przejdź do: " + toSc.DisplayName()
				}
				changed = true
				toast = "Połączono z: " + toSc.DisplayName()
			}
		}
	}
	if !changed {
		return toast
	}
	nonEndingTargets := 0
	for _, ch := range fromSc.Choices {
		if !ch.IsEnding && ch.Target != "" && !strings.EqualFold(ch.Target, "END") && !strings.EqualFold(ch.TargetSceneID, "END") {
			nonEndingTargets++
		}
	}
	resToast := s.commitScenesMutation(toast, "graph", "")
	if nonEndingTargets >= 2 && resToast == toast {
		return toast + " (rozwidlenie — kliknij kwadrat na równoległym węźle, aby wybrać główną ścieżkę)"
	}
	return resToast
}

func (s *Server) connectChapterTransition(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	fromChRaw := strings.TrimSpace(r.FormValue("from_chapter"))
	oldTargetRaw := strings.TrimSpace(r.FormValue("old_target_chapter"))
	toChRaw := strings.TrimSpace(r.FormValue("to_chapter"))
	fromCh := s.ActiveBook.FindChapter(fromChRaw)
	if fromCh == nil {
		return "Nie znaleziono rozdziału źródłowego"
	}
	s.ensureHistoryBaselineBeforeMutation()
	changed := false
	toast := ""

	if strings.EqualFold(toChRaw, "END") {
		for _, sc := range s.ActiveBook.Scenes {
			if !strings.EqualFold(strings.TrimSpace(sc.Chapter), strings.TrimSpace(fromCh.Name)) {
				continue
			}
			if len(sc.Choices) == 1 && sc.Choices[0].IsChapterLink &&
				(oldTargetRaw == "" || strings.EqualFold(sc.Choices[0].TargetChapterID, oldTargetRaw)) {
				sc.Choices = nil
				sc.ExplicitNext = ""
				sc.ExplicitEnd = true
				changed = true
			} else {
				for j := range sc.Choices {
					if sc.Choices[j].IsChapterLink && (oldTargetRaw == "" || strings.EqualFold(sc.Choices[j].TargetChapterID, oldTargetRaw)) {
						sc.Choices[j].Target = "END"
						sc.Choices[j].TargetSceneID = "END"
						sc.Choices[j].TargetChapterID = ""
						sc.Choices[j].IsChapterLink = false
						sc.Choices[j].IsEnding = true
						changed = true
					}
				}
			}
		}
		if changed {
			toast = "Rozłączono wyjście rozdziału: " + fromCh.DisplayName()
		}
	} else if toCh := s.ActiveBook.FindChapter(toChRaw); toCh != nil {
		if ok, reason := s.ActiveBook.CanLinkChapters(fromCh.ReferenceKey(), toCh.ReferenceKey()); !ok {
			return reason
		}
		targetRef := "chapter:" + toCh.ReferenceKey()
		for _, sc := range s.ActiveBook.Scenes {
			if !strings.EqualFold(strings.TrimSpace(sc.Chapter), strings.TrimSpace(fromCh.Name)) {
				continue
			}
			for j := range sc.Choices {
				if sc.Choices[j].IsChapterLink && (oldTargetRaw == "" || strings.EqualFold(sc.Choices[j].TargetChapterID, oldTargetRaw)) {
					sc.Choices[j].Target = targetRef
					sc.Choices[j].TargetSceneID = toCh.StartSceneID
					sc.Choices[j].TargetChapterID = toCh.ReferenceKey()
					sc.Choices[j].IsEnding = false
					if len(sc.Choices) == 1 {
						sc.ExplicitNext = targetRef
					}
					changed = true
				}
			}
		}
		if !changed {
			var exitScene *book.Scene
			if sel := s.findScene(s.SelectedSceneID); sel != nil && strings.EqualFold(strings.TrimSpace(sel.Chapter), strings.TrimSpace(fromCh.Name)) && len(sel.Choices) <= 1 {
				exitScene = sel
			}
			if exitScene == nil {
				for i := len(s.ActiveBook.Scenes) - 1; i >= 0; i-- {
					cand := s.ActiveBook.Scenes[i]
					if strings.EqualFold(strings.TrimSpace(cand.Chapter), strings.TrimSpace(fromCh.Name)) {
						if exitScene == nil {
							exitScene = cand
						}
						if cand.ExplicitEnd || len(cand.Choices) == 0 || (len(cand.Choices) == 1 && cand.Choices[0].IsEnding) {
							exitScene = cand
							break
						}
					}
				}
			}
			if exitScene != nil {
				exitScene.ExplicitEnd = false
				exitScene.ExplicitNext = targetRef
				exitScene.Choices = []book.Choice{
					{
						Label:           "Dalej",
						Target:          targetRef,
						TargetSceneID:   toCh.StartSceneID,
						TargetChapterID: toCh.ReferenceKey(),
						IsChapterLink:   true,
						IsImplicit:      true,
					},
				}
				changed = true
			}
		}
		if changed {
			toast = fmt.Sprintf("Połączono rozdział „%s” z „%s”", fromCh.DisplayName(), toCh.DisplayName())
		}
	}
	if !changed {
		return toast
	}
	return s.commitScenesMutation(toast, "graph", "")
}
