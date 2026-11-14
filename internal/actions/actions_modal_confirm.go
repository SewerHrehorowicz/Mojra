//go:build !(js && wasm)

package actions

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/osbridge"
	"mojra/internal/state"
)

func closeConfirmModal(s *state.AppState, _ *http.Request) string {
	s.ModalConfirm.Reset()
	return ""
}

func acceptConfirmModalAction(s *state.AppState, _ *http.Request) string {
	toast, _ := acceptConfirmModal(s)
	return toast
}

func acceptConfirmModal(s *state.AppState) (toast, nativeCmd string) {
	pending := s.ModalConfirm.PendingAction
	param := s.ModalConfirm.PendingParam
	s.ModalConfirm.Reset()

	switch pending {
	case RewindStep:
		if stepIdx, err := strconv.Atoi(param); err == nil && s.Player.Save != nil && stepIdx >= 0 && stepIdx < len(s.Player.Save.History) {
			targetSceneID := s.Player.Save.History[stepIdx].SceneID
			if len(s.Player.Save.History[stepIdx].Variables) > 0 {
				s.Player.Save.Variables = book.CloneVariablesMap(s.Player.Save.History[stepIdx].Variables)
			} else if s.Player.Book != nil {
				s.Player.Save.Variables = book.DefaultVariablesMap(s.Player.Book)
				if targetSc := s.Player.Book.FindScene(targetSceneID); targetSc != nil && targetSc.Effects != "" {
					book.ApplyEffects(targetSc.Effects, s.Player.Save.Variables, s.Player.Book)
				}
			}
			s.Player.Save.History = s.Player.Save.History[:stepIdx]
			s.Player.Save.CurrentSceneID = targetSceneID
			s.Player.Save.AudioTime = 0
			s.Player.HistoryRevealed = false
			s.Player.FocusedHistoryIdx = -1
			if !isHistoryPreview(s) {
				s.Player.PersistSave(s.Config.SavesDir)
			}
			toast = tr(s, i18n.TrRewoundStoryToast)
		}
	case ResetProgress:
		targetID := param
		if targetID == "" && s.Player.Book != nil {
			targetID = s.Player.Book.ID
		}
		if targetID != "" {
			if isHistoryPreview(s) && s.Player.Book != nil && s.Player.Book.ID == targetID {
				startSceneID := s.Player.Book.StartSceneID
				if s.Player.Book.FindScene(startSceneID) == nil && len(s.Player.Book.Scenes) > 0 {
					startSceneID = s.Player.Book.Scenes[0].ID
				}
				s.Player.Save = &book.SaveState{
					BookID:             s.Player.Book.ID,
					CurrentSceneID:     startSceneID,
					PlaybackSpeed:      s.Settings.PlaybackSpeed,
					AutoChoice:         s.Settings.AutoChoice,
					AutoChoiceStrategy: s.Settings.AutoChoiceStrategy,
					AutoChoiceDelay:    s.Settings.AutoChoiceDelay,
				}
				s.Player.HistoryRevealed = false
				s.Player.FocusedHistoryIdx = -1
			} else {
				_ = book.DeleteSaveState(s.Config.SavesDir, targetID)
				if s.Player.Book != nil && s.Player.Book.ID == targetID {
					s.Player.Save = book.LoadSaveState(s.Config.SavesDir, s.Player.Book.ID, s.Player.Book.StartSceneID)
					s.Player.HistoryRevealed = false
					s.Player.FocusedHistoryIdx = -1
				}
			}
			toast = tr(s, i18n.TrResetProgressToast)
		}
	case UnpackPlaybookForStudio:
		toast = unpackPlaybookWorkingCopyForStudio(s, param)
	case RemoveBook:
		toast = removeBookByID(s, param)
	case DeleteScene:
		toast = deleteSceneByID(s, param)
	case DeleteScenes:
		toast = deleteScenesByIDs(s, param)
	case CreateHistoryBranch:
		toast = createHistoryBranchByName(s, param, "")
	case RemoveChoice:
		if sep := strings.LastIndex(param, ":"); sep > 0 {
			scID := param[:sep]
			if choiceIdx, err := strconv.Atoi(param[sep+1:]); err == nil {
				toast = removeChoiceBySceneAndIndex(s, scID, choiceIdx)
			}
		}
	case QuitApp:
		nativeCmd = "quit"
		go func() {
			s.Config.TriggerShutdown()
			osbridge.TerminateNativeWindow()
		}()
	}
	return toast, nativeCmd
}

func promptRewindStep(s *state.AppState, r *http.Request) string {
	if stepIdx, err := strconv.Atoi(r.FormValue("step")); err == nil && s.Player.Save != nil && stepIdx >= 0 && stepIdx < len(s.Player.Save.History) {
		entry := s.Player.Save.History[stepIdx]
		sceneName := entry.SceneName
		if sc := s.Player.Book.FindScene(entry.SceneID); sc != nil {
			sceneName = sc.DisplayName()
		}
		if sceneName == "" {
			sceneName = entry.SceneID
		}
		s.ModalConfirm.Prompt(state.ModalConfirmState{
			Title:         tr(s, i18n.TrRewindSceneHint),
			Message:       trf(s, i18n.TrConfirmRewindMsgFmt, sceneName),
			PendingAction: RewindStep,
			PendingParam:  strconv.Itoa(stepIdx),
			Danger:        false,
		})
	}
	return ""
}

func promptResetProgress(s *state.AppState, r *http.Request) string {
	targetBook := s.Player.Book
	if reqBookID := strings.TrimSpace(r.FormValue("book")); reqBookID != "" {
		if found := s.Library.Get(book.SanitizeBookID(reqBookID)); found != nil {
			targetBook = found
		}
	}
	if targetBook != nil {
		s.ModalConfirm.Prompt(state.ModalConfirmState{
			Title:         tr(s, i18n.TrConfirmResetTitle),
			Message:       trf(s, i18n.TrConfirmResetMsgFmt, targetBook.Title),
			PendingAction: ResetProgress,
			PendingParam:  targetBook.ID,
			Danger:        false,
		})
	}
	return ""
}

func promptDeleteScene(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	targetID := strings.TrimSpace(r.FormValue("scene"))
	if targetID == "" {
		targetID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	if targetID == "" {
		targetID = s.Editor.SelectedSceneID
	}
	if sc := b.FindScene(targetID); sc != nil {
		s.ModalConfirm.Prompt(state.ModalConfirmState{
			Title:         "Usuń scenę",
			Message:       fmt.Sprintf("Czy na pewno chcesz usunąć scenę „%s” z pliku %s?", sc.DisplayName(), sc.SourceFile),
			PendingAction: DeleteScene,
			PendingParam:  sc.ID,
			Danger:        true,
		})
	}
	return ""
}

func promptDeleteScenesFromForm(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
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
		if sc := b.FindScene(idPart); sc != nil && !seen[sc.ID] {
			seen[sc.ID] = true
			matched = append(matched, sc)
		}
	}
	if len(matched) == 1 {
		sc := matched[0]
		s.ModalConfirm.Prompt(state.ModalConfirmState{
			Title:         "Usuń scenę",
			Message:       fmt.Sprintf("Czy na pewno chcesz usunąć scenę „%s” z pliku %s?", sc.DisplayName(), sc.SourceFile),
			PendingAction: DeleteScene,
			PendingParam:  sc.ID,
			Danger:        true,
		})
	} else if len(matched) > 1 {
		var ids []string
		var names []string
		for _, sc := range matched {
			ids = append(ids, sc.ID)
			names = append(names, fmt.Sprintf("„%s”", sc.DisplayName()))
		}
		s.ModalConfirm.Prompt(state.ModalConfirmState{
			Title:         fmt.Sprintf("Usuń zaznaczone sceny (%d)", len(matched)),
			Message:       fmt.Sprintf("Czy na pewno chcesz usunąć zaznaczone sceny (%d): %s?", len(matched), strings.Join(names, ", ")),
			PendingAction: DeleteScenes,
			PendingParam:  strings.Join(ids, ","),
			Danger:        true,
		})
	}
	return ""
}

func promptRemoveChoiceFromScene(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	syncSceneTextSettingsOpen(s, r)
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	scID, target := resolveEditorSceneFromRequest(s, r, true)
	if target == nil {
		return ""
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx")))
	if err != nil {
		return ""
	}

	if s.Editor.SceneText.IsOpen() {
		if saved, _ := flushSceneTextChanges(s, r); saved {
			target = s.Editor.ActiveBook.FindScene(s.Editor.SceneText.SceneID)
		}
	} else if !b.IsPacked {
		flushFormSceneIfPresent(s, r)
		target = s.Editor.ActiveBook.FindScene(scID)
		if target == nil {
			target = s.Editor.ActiveBook.FindScene(s.Editor.SelectedSceneID)
		}
	}
	if target == nil || choiceIdx < 0 || choiceIdx >= len(target.Choices) {
		return ""
	}
	chLabel := strings.TrimSpace(target.Choices[choiceIdx].Label)
	if chLabel == "" {
		chLabel = fmt.Sprintf("Wybór %d", choiceIdx+1)
	}
	s.ModalConfirm.Prompt(state.ModalConfirmState{
		Title:         "Usuń wybór",
		Message:       fmt.Sprintf("Czy na pewno chcesz usunąć wybór „%s” ze sceny „%s”?", chLabel, target.DisplayName()),
		PendingAction: RemoveChoice,
		PendingParam:  fmt.Sprintf("%s:%d", target.ID, choiceIdx),
		Danger:        true,
	})
	return ""
}

func promptCreateHistoryBranchAction(s *state.AppState, r *http.Request) string {
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	rawBranch := ""
	if r != nil {
		rawBranch = strings.TrimSpace(r.FormValue("branch_name"))
		if rawBranch == "" {
			rawBranch = strings.TrimSpace(r.FormValue("branch"))
		}
	}
	cleanBranch := ""
	if rawBranch != "" {
		cleanBranch = book.SanitizeBookID(rawBranch)
	}
	h := s.Editor.History.RefreshFromDisk(b)
	if cleanBranch == "" || cleanBranch == "book" {
		startSeq := 2
		if h != nil {
			startSeq = len(h.Branches) + 1
		}
		for seq := startSeq; ; seq++ {
			cand := "wariant-" + strconv.Itoa(seq)
			exists := false
			if h != nil {
				for _, br := range h.Branches {
					if strings.EqualFold(br.Name, cand) {
						exists = true
						break
					}
				}
			}
			if !exists {
				cleanBranch = cand
				break
			}
		}
	}
	if h != nil {
		for _, br := range h.Branches {
			if strings.EqualFold(br.Name, cleanBranch) {
				return "Gałąź „" + cleanBranch + "” już istnieje"
			}
		}
	}
	s.ModalConfirm.Prompt(state.ModalConfirmState{
		Title:         "Utwórz nowy wariant",
		Message:       "Czy na pewno chcesz utworzyć nowy wariant „" + cleanBranch + "”? Nowy tor jest trwałą częścią historii projektu.",
		PendingAction: CreateHistoryBranch,
		PendingParam:  cleanBranch,
		Danger:        false,
	})
	return ""
}
