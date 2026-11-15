//go:build !(js && wasm)

package actions

import (
	"net/http"
	"strconv"
	"strings"

	book "mojra/domain/book/actions"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/engine/transport"
)

const (
	formKeyOldChapter       = "old_chapter"
	formKeyNewChapter       = "new_chapter"
	formKeyFromScene        = "from_scene"
	formKeyToScene          = "to_scene"
	formKeyToChapter        = "to_chapter"
	formKeyFromChoice       = "from_choice"
	formKeyFlushForm        = "flush_form"
	formKeyFromChapter      = "from_chapter"
	formKeyOldTargetChapter = "old_target_chapter"
	historyScopeChapter     = "chapter"
	coalesceRenameChapter   = "renamechapter"
	coalesceSetChapterStart = "setchapterstart"
)

var _ = registerActions(map[string]ActionHandler{
	"SelectStudioChapter":   selectStudioChapter,
	"SetChapterStartScene":  setChapterStartScene,
	"SaveChapterSettings":   saveChapterSettings,
	"RenameChapter":         renameActiveBookChapter,
	"AssignChapterToScenes": assignChapterToScenes,
	"CreateChapterGroup":    createChapterGroup,
	"ConnectChoice":         connectChoiceOnScene,
	"ConnectChapter":        connectChapterTransition,
})

func selectStudioChapter(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := flushFormSceneIfPresent(s, r)
	chKey := strings.TrimSpace(r.FormValue(formKeyChapter))
	if s.Editor.ActiveBook != nil && chKey != "" {
		if foundCh := s.Editor.ActiveBook.FindChapter(chKey); foundCh != nil {
			s.Editor.SelectedChapter = foundCh.Name
			activateRightPanel(s, state.PanelChapter)
		}
	}
	return frameInspector(s, "", hadFormScene)
}

func renameActiveBookChapterToast(s *state.StudioState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked {
		return ""
	}
	oldCh := book.SanitizePlainText(r.FormValue(formKeyOldChapter))
	newCh := book.SanitizePlainText(r.FormValue(formKeyNewChapter))
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
					sc.ExplicitNext = formValChTargetPref + newRef
				}
				for j := range sc.Choices {
					if chRef, ok := book.ParseChapterTarget(sc.Choices[j].Target); ok && (strings.EqualFold(chRef, oldRef) || strings.EqualFold(chRef, oldCh) || strings.EqualFold(book.SanitizeChapterID(chRef), book.SanitizeChapterID(oldCh))) {
						sc.Choices[j].Target = formValChTargetPref + newRef
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
			return trf(s, i18n.TrRenamedActFmt, newCh, updated), historyScopeChapter, coalesceRenameChapter, true
		}
		return trf(s, i18n.TrRenamedChapterFmt, newCh, updated), historyScopeChapter, coalesceRenameChapter, true
	})
}

func renameActiveBookChapter(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	return frameUndoRedo(s, renameActiveBookChapterToast(s, r))
}

func setChapterStartSceneToast(s *state.StudioState, r *http.Request) string {
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked {
		return ""
	}
	chKey := strings.TrimSpace(r.FormValue(formKeyChapter))
	if chKey == "" {
		chKey = strings.TrimSpace(r.FormValue(formKeyOldChapter))
	}
	if chKey == "" {
		chKey = s.Editor.SelectedChapter
	}
	scKey := strings.TrimSpace(r.FormValue(formKeyScene))
	if scKey == "" {
		scKey = strings.TrimSpace(r.FormValue(formKeyStartScene))
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
			return trf(s, i18n.TrSetActStartSceneFmt, ch.DisplayName(), target.DisplayName()), historyScopeChapter, coalesceSetChapterStart, true
		}
		return trf(s, i18n.TrSetChapterStartSceneFmt, ch.DisplayName(), target.DisplayName()), historyScopeChapter, coalesceSetChapterStart, true
	})
}

func setChapterStartScene(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	return frameUndoRedo(s, setChapterStartSceneToast(s, r))
}

func saveChapterSettings(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	toast := renameActiveBookChapterToast(s, r)
	if startSc := strings.TrimSpace(r.FormValue(formKeyStartScene)); startSc != "" {
		if t2 := setChapterStartSceneToast(s, r); t2 != "" {
			toast = t2
		}
	}
	return frameUndoRedo(s, toast)
}

func assignChapterToScenes(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked {
		return frameUndoRedo(s, "")
	}
	targetCh := book.SanitizePlainText(r.FormValue(formKeyChapter))
	rawIDs := strings.TrimSpace(r.FormValue(formKeySceneIDs))
	if targetCh == "" || rawIDs == "" {
		return frameUndoRedo(s, "")
	}
	toast := withBookMutation(s, r, false, func(b *book.Book) (string, string, string, bool) {
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
			return trf(s, i18n.TrAssignedScenesToActFmt, updated, targetCh), historyScopeChapter, "", true
		}
		return trf(s, i18n.TrAssignedScenesToChFmt, updated, targetCh), historyScopeChapter, "", true
	})
	return frameUndoRedo(s, toast)
}

func createChapterGroup(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	hadEditorBook := s.Editor.ActiveBook != nil
	if !hadEditorBook && !s.Config.StandaloneMode {
		if createdBook, err := book.CreateNewBook(s.Config.BooksDir, tr(s, i18n.TrDefaultBookTitle), tr(s, i18n.TrAuthorPlaceholder), false); err == nil && createdBook != nil {
			if s.Hooks.OnBookMutated != nil {
				s.Hooks.OnBookMutated(createdBook)
			}
			s.Editor.SetBook(createdBook, s.Settings)
			s.SaveSettings()
		}
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameUndoRedo(s, "")
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	chName := book.SanitizePlainText(r.FormValue(formKeyChapter))
	if chName == "" {
		if b.IsScreenplay() {
			chName = trf(s, i18n.TrDefaultActFmt, len(b.Chapters)+1)
		} else {
			chName = trf(s, i18n.TrDefaultChapterFmt, len(b.Chapters)+1)
		}
	}
	x, errX := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyX)))
	y, errY := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyY)))
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
		return frameUndoRedo(s, err.Error())
	}
	if created == nil {
		return frameUndoRedo(s, "")
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
	toast := trf(s, i18n.TrCreatedChapterFmt, chName)
	if s.Editor.ActiveBook != nil && s.Editor.ActiveBook.IsScreenplay() {
		toast = trf(s, i18n.TrCreatedActFmt, chName)
	}
	s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, historyScopeChapter, "", s.Editor.SelectedSceneID)
	if !hadEditorBook {
		return frameApp(s, toast)
	}
	return frameUndoRedo(s, toast)
}

func applyChapterLinkToScene(s *state.StudioState, fromSc *book.Scene, idx int, targetCh *book.Chapter) (changed bool, toast string) {
	targetRef := formValChTargetPref + targetCh.ReferenceKey()
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
				Label:           formValDalej,
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
		return true, tr(s, i18n.TrConnectedToChapterPrefix) + targetCh.DisplayName()
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
			fromSc.Choices[idx].Label = tr(s, i18n.TrChoiceGoToPrefix) + targetCh.DisplayName()
		}
		return true, tr(s, i18n.TrConnectedToChapterPrefix) + targetCh.DisplayName()
	}
	return false, ""
}

func connectChoiceOnScene(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameUndoRedo(s, "")
	}
	fromID := strings.TrimSpace(r.FormValue(formKeyFromScene))
	if fromID == "" {
		fromID = strings.TrimSpace(r.FormValue(formKeySceneID))
	}
	toID := strings.TrimSpace(r.FormValue(formKeyToScene))
	if toChParam := strings.TrimSpace(r.FormValue(formKeyToChapter)); toChParam != "" && toID == "" {
		toID = formValChTargetPref + toChParam
	}
	rawIdx := strings.TrimSpace(r.FormValue(formKeyChoiceIdx))
	if rawIdx == "" {
		rawIdx = strings.TrimSpace(r.FormValue(formKeyFromChoice))
	}
	idx, errIdx := strconv.Atoi(rawIdx)
	if errIdx != nil && rawIdx == "" {
		idx = 0
		errIdx = nil
	}
	fromSc := b.FindScene(fromID)
	if errIdx != nil || fromSc == nil {
		return frameUndoRedo(s, "")
	}

	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if r.FormValue(formKeyFlushForm) == formValTrue && r.FormValue(formKeySceneID) != "" {
		applySceneFormFields(s, fromSc, r)
	}
	changed := false
	toast := ""

	if strings.EqualFold(toID, formValEND) {
		if len(fromSc.Choices) <= 1 {
			fromSc.Choices = nil
			fromSc.ExplicitNext = ""
			fromSc.ExplicitEnd = true
			changed = true
			toast = tr(s, i18n.TrSetSceneAsEnd)
		} else if idx >= 0 && idx < len(fromSc.Choices) {
			fromSc.Choices[idx].Target = formValEND
			fromSc.Choices[idx].TargetSceneID = formValEND
			fromSc.Choices[idx].TargetChapterID = ""
			fromSc.Choices[idx].IsChapterLink = false
			fromSc.Choices[idx].IsEnding = true
			changed = true
			toast = tr(s, i18n.TrConnectedToEnd)
		}
	} else if chRef, isChTarget := book.ParseChapterTarget(toID); isChTarget {
		if chRef == formValNextChToken {
			for _, candCh := range b.Chapters {
				if ok, _ := b.CanLinkChapters(fromSc.Chapter, candCh.ReferenceKey()); ok {
					chRef = candCh.ReferenceKey()
					break
				}
			}
			if chRef == formValNextChToken {
				return frameUndoRedo(s, tr(s, i18n.TrCreateNextChapterFirst))
			}
		}
		targetCh := b.FindChapter(chRef)
		if targetCh == nil {
			return frameUndoRedo(s, tr(s, i18n.TrTargetChapterNotFoundPrefix)+chRef)
		}
		if ok, reason := b.CanLinkChapters(fromSc.Chapter, targetCh.ReferenceKey()); !ok {
			return frameUndoRedo(s, reason)
		}
		changed, toast = applyChapterLinkToScene(s, fromSc, idx, targetCh)
	} else if toSc := b.FindScene(toID); toSc != nil {
		if !strings.EqualFold(strings.TrimSpace(toSc.Chapter), strings.TrimSpace(fromSc.Chapter)) {
			targetCh := b.FindChapter(toSc.Chapter)
			if targetCh == nil || !strings.EqualFold(toSc.ID, targetCh.StartSceneID) {
				return frameUndoRedo(s, tr(s, i18n.TrCrossChapterOnlyStart))
			}
			if ok, reason := b.CanLinkChapters(fromSc.Chapter, targetCh.ReferenceKey()); !ok {
				return frameUndoRedo(s, reason)
			}
			changed, toast = applyChapterLinkToScene(s, fromSc, idx, targetCh)
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
						Label:          formValDalej,
						Target:         targetRef,
						TargetSceneID:  toSc.ID,
						IsImplicit:     true,
						Condition:      prevCond,
						ShowWhenLocked: prevLocked,
						Effects:        prevEff,
					},
				}
				changed = true
				toast = tr(s, i18n.TrConnectedNextStepPrefix) + toSc.DisplayName()
			} else if idx >= 0 && idx < len(fromSc.Choices) {
				fromSc.ExplicitEnd = false
				fromSc.Choices[idx].Target = targetRef
				fromSc.Choices[idx].TargetSceneID = toSc.ID
				fromSc.Choices[idx].TargetChapterID = ""
				fromSc.Choices[idx].IsChapterLink = false
				fromSc.Choices[idx].IsImplicit = false
				fromSc.Choices[idx].IsEnding = false
				if strings.TrimSpace(fromSc.Choices[idx].Label) == "" {
					fromSc.Choices[idx].Label = tr(s, i18n.TrChoiceGoToPrefix) + toSc.DisplayName()
				}
				changed = true
				toast = tr(s, i18n.TrConnectedToPrefix) + toSc.DisplayName()
			}
		}
	}
	if !changed {
		return frameUndoRedo(s, toast)
	}
	nonEndingTargets := 0
	for _, ch := range fromSc.Choices {
		if !ch.IsEnding && ch.Target != "" && !strings.EqualFold(ch.Target, formValEND) && !strings.EqualFold(ch.TargetSceneID, formValEND) {
			nonEndingTargets++
		}
	}
	resToast := commitScenesMutation(s, toast, historyScopeGraph, "")
	if nonEndingTargets >= 2 && resToast == toast {
		return frameUndoRedo(s, toast+tr(s, i18n.TrBranchingSuffix))
	}
	return frameUndoRedo(s, resToast)
}

func connectChapterTransition(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameUndoRedo(s, "")
	}
	fromChRaw := strings.TrimSpace(r.FormValue(formKeyFromChapter))
	oldTargetRaw := strings.TrimSpace(r.FormValue(formKeyOldTargetChapter))
	toChRaw := strings.TrimSpace(r.FormValue(formKeyToChapter))
	fromCh := b.FindChapter(fromChRaw)
	if fromCh == nil {
		return frameUndoRedo(s, tr(s, i18n.TrSourceChapterNotFound))
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	changed := false
	toast := ""

	if strings.EqualFold(toChRaw, formValEND) {
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
						sc.Choices[j].Target = formValEND
						sc.Choices[j].TargetSceneID = formValEND
						sc.Choices[j].TargetChapterID = ""
						sc.Choices[j].IsChapterLink = false
						sc.Choices[j].IsEnding = true
						changed = true
					}
				}
			}
		}
		if changed {
			toast = tr(s, i18n.TrDisconnectedChapterPrefix) + fromCh.DisplayName()
		}
	} else if toCh := b.FindChapter(toChRaw); toCh != nil {
		if ok, reason := b.CanLinkChapters(fromCh.ReferenceKey(), toCh.ReferenceKey()); !ok {
			return frameUndoRedo(s, reason)
		}
		targetRef := formValChTargetPref + toCh.ReferenceKey()
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
						Label:           formValDalej,
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
			toast = trf(s, i18n.TrConnectedChaptersFmt, fromCh.DisplayName(), toCh.DisplayName())
		}
	}
	if !changed {
		return frameUndoRedo(s, toast)
	}
	return frameUndoRedo(s, commitScenesMutation(s, toast, historyScopeGraph, ""))
}
