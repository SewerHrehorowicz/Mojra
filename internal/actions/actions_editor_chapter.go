//go:build !(js && wasm)

package actions

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

func selectStudioChapter(s *state.AppState, r *http.Request) string {
	chKey := strings.TrimSpace(r.FormValue("chapter"))
	if s.Editor.ActiveBook != nil && chKey != "" {
		if foundCh := s.Editor.ActiveBook.FindChapter(chKey); foundCh != nil {
			s.Editor.SelectedChapter = foundCh.Name
			activateRightPanel(s, state.PanelChapter)
		}
	}
	return ""
}

func renameActiveBookChapter(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked {
		return ""
	}
	oldCh := book.SanitizePlainText(r.FormValue("old_chapter"))
	newCh := book.SanitizePlainText(r.FormValue("new_chapter"))
	if oldCh == "" || newCh == "" || oldCh == newCh {
		return ""
	}
	return withBookMutation(s, r, false, func(b *book.Book) (string, string, string, bool) {
		oldRef := book.SanitizeChapterID(oldCh)
		if oldRef == "" {
			oldRef = book.SanitizeSceneID(oldCh)
		}
		newRef := book.NextUniqueChapterSlug(b, newCh, oldCh)
		for i := range b.Chapters {
			if strings.EqualFold(strings.TrimSpace(b.Chapters[i].Name), oldCh) ||
				strings.EqualFold(b.Chapters[i].ID, oldCh) ||
				strings.EqualFold(b.Chapters[i].ExplicitID, oldCh) {
				prevRef := b.Chapters[i].ReferenceKey()
				if prevRef != "" {
					oldRef = prevRef
				}
				oldSlug := book.SanitizeChapterID(b.Chapters[i].Name)
				oldScSlug := book.SanitizeSceneID(b.Chapters[i].Name)
				b.Chapters[i].Name = newCh
				if b.Chapters[i].ExplicitID == "" ||
					strings.EqualFold(b.Chapters[i].ExplicitID, oldSlug) ||
					strings.EqualFold(b.Chapters[i].ExplicitID, oldScSlug) ||
					strings.EqualFold(b.Chapters[i].ExplicitID, oldRef) {
					b.Chapters[i].ExplicitID = newRef
					b.Chapters[i].ID = newRef
					b.Chapters[i].SourceFile = b.ChapterFileNameFor(newCh, newRef)
				} else {
					newRef = b.Chapters[i].ExplicitID
				}
			}
		}
		updated := 0
		for _, sc := range b.Scenes {
			scCh := strings.TrimSpace(sc.Chapter)
			if scCh == "" {
				scCh = b.DefaultChapterName()
			}
			if strings.EqualFold(scCh, oldCh) || (oldRef != "" && strings.EqualFold(sc.ChapterID, oldRef)) {
				sc.Chapter = newCh
				sc.ChapterID = newRef
				sc.SourceFile = b.ChapterFileNameFor(sc.Chapter, sc.ChapterID)
				updated++
			}
			if oldRef != "" && newRef != "" && !strings.EqualFold(oldRef, newRef) {
				if chRef, ok := book.ParseChapterTarget(sc.ExplicitNext); ok && (strings.EqualFold(chRef, oldRef) || strings.EqualFold(chRef, oldCh) || strings.EqualFold(book.SanitizeChapterID(chRef), book.SanitizeChapterID(oldCh))) {
					sc.ExplicitNext = "chapter:" + newRef
				}
				for j := range sc.Choices {
					if chRef, ok := book.ParseChapterTarget(sc.Choices[j].Target); ok && (strings.EqualFold(chRef, oldRef) || strings.EqualFold(chRef, oldCh) || strings.EqualFold(book.SanitizeChapterID(chRef), book.SanitizeChapterID(oldCh))) {
						sc.Choices[j].Target = "chapter:" + newRef
						sc.Choices[j].TargetChapterID = newRef
					}
				}
			}
		}
		if updated == 0 {
			return "", "", "", false
		}
		if strings.EqualFold(s.Editor.SelectedChapter, oldCh) {
			s.Editor.SelectedChapter = newCh
		}
		if b.IsScreenplay() {
			return fmt.Sprintf("Zmieniono akt na „%s” (%d scen)", newCh, updated), "chapter", "renamechapter", true
		}
		return fmt.Sprintf("Zmieniono rozdział na „%s” (%d scen)", newCh, updated), "chapter", "renamechapter", true
	})
}

func setChapterStartScene(s *state.AppState, r *http.Request) string {
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked {
		return ""
	}
	chKey := strings.TrimSpace(r.FormValue("chapter"))
	if chKey == "" {
		chKey = strings.TrimSpace(r.FormValue("old_chapter"))
	}
	if chKey == "" {
		chKey = s.Editor.SelectedChapter
	}
	scKey := strings.TrimSpace(r.FormValue("scene"))
	if scKey == "" {
		scKey = strings.TrimSpace(r.FormValue("start_scene"))
	}
	if chKey == "" || scKey == "" {
		return ""
	}
	return withBookMutation(s, r, false, func(b *book.Book) (string, string, string, bool) {
		ch := b.FindChapter(chKey)
		target := b.FindScene(scKey)
		if ch == nil || target == nil {
			return "", "", "", false
		}
		targetCh := strings.TrimSpace(target.Chapter)
		if targetCh == "" {
			targetCh = b.DefaultChapterName()
		}
		if !strings.EqualFold(targetCh, strings.TrimSpace(ch.Name)) {
			return "", "", "", false
		}
		ch.ExplicitStart = target.ReferenceKey()
		ch.StartSceneID = target.ID
		s.Editor.SelectedChapter = ch.Name
		activateRightPanel(s, state.PanelChapter)
		if b.IsScreenplay() {
			return fmt.Sprintf("Ustawiono scenę początkową aktu „%s”: %s", ch.DisplayName(), target.DisplayName()), "chapter", "setchapterstart", true
		}
		return fmt.Sprintf("Ustawiono scenę początkową rozdziału „%s”: %s", ch.DisplayName(), target.DisplayName()), "chapter", "setchapterstart", true
	})
}

func saveChapterSettings(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	toast := renameActiveBookChapter(s, r)
	if startSc := strings.TrimSpace(r.FormValue("start_scene")); startSc != "" {
		if t2 := setChapterStartScene(s, r); t2 != "" {
			toast = t2
		}
	}
	return toast
}

func assignChapterToScenes(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked {
		return ""
	}
	targetCh := book.SanitizePlainText(r.FormValue("chapter"))
	rawIDs := strings.TrimSpace(r.FormValue("scene_ids"))
	if targetCh == "" || rawIDs == "" {
		return ""
	}
	return withBookMutation(s, r, false, func(b *book.Book) (string, string, string, bool) {
		var targetChID, targetSF string
		if chMeta := b.FindChapter(targetCh); chMeta != nil {
			targetChID = chMeta.ExplicitID
			targetSF = chMeta.SourceFile
		}
		if targetSF == "" {
			targetSF = b.ChapterFileNameFor(targetCh, targetChID)
		}
		updated := 0
		for _, idPart := range strings.Split(rawIDs, ",") {
			idPart = strings.TrimSpace(idPart)
			if sc := b.FindScene(idPart); sc != nil {
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

func createChapterGroup(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	if s.Editor.ActiveBook == nil && !s.Config.StandaloneMode {
		if createdBook, err := book.CreateNewBook(s.Config.BooksDir, "Nowa Opowieść", "Autor", false); err == nil && createdBook != nil {
			s.Library.Put(createdBook)
			s.Editor.SetBook(createdBook, s.Settings)
			_ = s.SaveSettings()
		}
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	chName := book.SanitizePlainText(r.FormValue("chapter"))
	if chName == "" {
		if b.IsScreenplay() {
			chName = fmt.Sprintf("Akt %d", len(b.Chapters)+1)
		} else {
			chName = fmt.Sprintf("Rozdział %d", len(b.Chapters)+1)
		}
	}
	x, errX := strconv.Atoi(strings.TrimSpace(r.FormValue("x")))
	y, errY := strconv.Atoi(strings.TrimSpace(r.FormValue("y")))
	hasCoords := errX == nil && errY == nil
	if !hasCoords && len(b.Scenes) > 0 {
		maxY := b.Scenes[0].Y
		for _, sc := range b.Scenes {
			if sc.Y > maxY {
				maxY = sc.Y
			}
		}
		x = book.NewChapterDefaultOffsetX
		y = book.SnapToGrid(maxY + book.NewChapterDefaultOffsetY)
		hasCoords = true
	}
	newName := book.NextUniqueSceneName(b)
	created, err := book.CreateStandaloneSceneInChapter(b, newName, chName, x, y, hasCoords)
	if err != nil {
		return err.Error()
	}
	if created == nil {
		return ""
	}
	reloadActiveBook(s)
	if found := s.Editor.ActiveBook.FindScene(created.Name); found != nil {
		s.Editor.SelectedSceneID = found.ID
	} else {
		s.Editor.SelectedSceneID = created.ID
	}
	if s.Editor.SceneText.IsOpen() {
		s.Editor.SceneText.SyncSceneID(s.Editor.SelectedSceneID)
	}
	toast := fmt.Sprintf("Utworzono rozdział „%s”", chName)
	if s.Editor.ActiveBook != nil && s.Editor.ActiveBook.IsScreenplay() {
		toast = fmt.Sprintf("Utworzono akt „%s”", chName)
	}
	s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, "chapter", "", s.Editor.SelectedSceneID)
	return toast
}

func applyChapterLinkToScene(fromSc *book.Scene, idx int, targetCh *book.Chapter) (changed bool, toast string) {
	targetRef := "chapter:" + targetCh.ReferenceKey()
	if len(fromSc.Choices) <= 1 {
		var prevCond, prevEff string
		var prevLocked bool
		if len(fromSc.Choices) == 1 {
			prevCond = fromSc.Choices[0].Condition
			prevLocked = fromSc.Choices[0].ShowWhenLocked
			prevEff = fromSc.Choices[0].Effects
		}
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
				Condition:       prevCond,
				ShowWhenLocked:  prevLocked,
				Effects:         prevEff,
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

func connectChoiceOnScene(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
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
	rawIdx := strings.TrimSpace(r.FormValue("choice_idx"))
	if rawIdx == "" {
		rawIdx = strings.TrimSpace(r.FormValue("from_choice"))
	}
	idx, errIdx := strconv.Atoi(rawIdx)
	if errIdx != nil && rawIdx == "" {
		idx = 0
		errIdx = nil
	}
	fromSc := b.FindScene(fromID)
	if errIdx != nil || fromSc == nil {
		return ""
	}

	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if r.FormValue("flush_form") == "true" && r.FormValue("scene_id") != "" {
		applySceneFormFields(s, fromSc, r)
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
			for _, candCh := range b.Chapters {
				if ok, _ := b.CanLinkChapters(fromSc.Chapter, candCh.ReferenceKey()); ok {
					chRef = candCh.ReferenceKey()
					break
				}
			}
			if chRef == "__next__" {
				return "Utwórz najpierw kolejny rozdział (+ Rozdział), aby wyprowadzić przejście z rozdziału"
			}
		}
		targetCh := b.FindChapter(chRef)
		if targetCh == nil {
			return "Nie znaleziono docelowego rozdziału: " + chRef
		}
		if ok, reason := b.CanLinkChapters(fromSc.Chapter, targetCh.ReferenceKey()); !ok {
			return reason
		}
		changed, toast = applyChapterLinkToScene(fromSc, idx, targetCh)
	} else if toSc := b.FindScene(toID); toSc != nil {
		if !strings.EqualFold(strings.TrimSpace(toSc.Chapter), strings.TrimSpace(fromSc.Chapter)) {
			targetCh := b.FindChapter(toSc.Chapter)
			if targetCh == nil || !strings.EqualFold(toSc.ID, targetCh.StartSceneID) {
				return "Przejście między rozdziałami może prowadzić wyłącznie do początku kolejnego rozdziału"
			}
			if ok, reason := b.CanLinkChapters(fromSc.Chapter, targetCh.ReferenceKey()); !ok {
				return reason
			}
			changed, toast = applyChapterLinkToScene(fromSc, idx, targetCh)
		} else {
			targetRef := toSc.ReferenceKey()
			if len(fromSc.Choices) <= 1 {
				var prevCond, prevEff string
				var prevLocked bool
				if len(fromSc.Choices) == 1 {
					prevCond = fromSc.Choices[0].Condition
					prevLocked = fromSc.Choices[0].ShowWhenLocked
					prevEff = fromSc.Choices[0].Effects
				}
				fromSc.ExplicitEnd = false
				fromSc.ExplicitNext = targetRef
				fromSc.Choices = []book.Choice{
					{
						Label:          "Dalej",
						Target:         targetRef,
						TargetSceneID:  toSc.ID,
						IsImplicit:     true,
						Condition:      prevCond,
						ShowWhenLocked: prevLocked,
						Effects:        prevEff,
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
	resToast := commitScenesMutation(s, toast, "graph", "")
	if nonEndingTargets >= 2 && resToast == toast {
		return toast + " (rozwidlenie ścieżki)"
	}
	return resToast
}

func connectChapterTransition(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	fromChRaw := strings.TrimSpace(r.FormValue("from_chapter"))
	oldTargetRaw := strings.TrimSpace(r.FormValue("old_target_chapter"))
	toChRaw := strings.TrimSpace(r.FormValue("to_chapter"))
	fromCh := b.FindChapter(fromChRaw)
	if fromCh == nil {
		return "Nie znaleziono rozdziału źródłowego"
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	changed := false
	toast := ""

	if strings.EqualFold(toChRaw, "END") {
		for _, sc := range b.Scenes {
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
	} else if toCh := b.FindChapter(toChRaw); toCh != nil {
		if ok, reason := b.CanLinkChapters(fromCh.ReferenceKey(), toCh.ReferenceKey()); !ok {
			return reason
		}
		targetRef := "chapter:" + toCh.ReferenceKey()
		for _, sc := range b.Scenes {
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
			if sel := b.FindScene(s.Editor.SelectedSceneID); sel != nil && strings.EqualFold(strings.TrimSpace(sel.Chapter), strings.TrimSpace(fromCh.Name)) && len(sel.Choices) <= 1 {
				exitScene = sel
			}
			if exitScene == nil {
				for i := len(b.Scenes) - 1; i >= 0; i-- {
					cand := b.Scenes[i]
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
	return commitScenesMutation(s, toast, "graph", "")
}
