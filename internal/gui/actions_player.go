package gui

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"storyteller/internal/book"
)

func (s *Server) saveAudioTime(r *http.Request) string {
	if s.ActiveBook != nil && s.Save != nil {
		if t, err := strconv.ParseFloat(r.FormValue("time"), 64); err == nil && t >= 0 {
			s.Save.AudioTime = t
			if !s.isHistoryPreview() {
				_ = book.UpdateSaveAudioTimeInPlace(s.SavesDir, s.ActiveBook.ID, t, s.ActiveBook.StartSceneID)
			}
		}
	}
	return ""
}

func (s *Server) chooseBranch(r *http.Request) string {
	idx, err := strconv.Atoi(r.FormValue("choice_idx"))
	cur := s.currentScene()
	if err == nil && cur != nil && idx >= 0 && idx < len(cur.Choices) {
		return s.executeChoiceTransition(cur, cur.Choices[idx])
	}
	return ""
}

func (s *Server) autoAdvanceImplicit(_ *http.Request) string {
	cur := s.currentScene()
	if cur != nil && len(cur.Choices) == 1 && !cur.Choices[0].IsEnding && cur.Choices[0].TargetSceneID != "END" {
		return s.executeChoiceTransition(cur, cur.Choices[0])
	}
	return ""
}

func (s *Server) audioEnded(_ *http.Request) string {
	if s.Settings.AutoChoice {
		cur := s.currentScene()
		if cur != nil && len(cur.Choices) > 0 && !cur.ExplicitEnd {
			idx := rand.Intn(len(cur.Choices))
			return s.executeChoiceTransition(cur, cur.Choices[idx])
		}
	}
	return ""
}

func (s *Server) revealHistory(_ *http.Request) string {
	if s.Save != nil && len(s.Save.History) > 0 {
		s.HistoryRevealed = true
		s.FocusedHistoryIdx = len(s.Save.History) - 1
	}
	return ""
}

func (s *Server) focusHistoryStep(r *http.Request) string {
	if stepIdx, err := strconv.Atoi(r.FormValue("step")); err == nil && s.Save != nil && stepIdx >= 0 && stepIdx < len(s.Save.History) {
		s.HistoryRevealed = true
		s.FocusedHistoryIdx = stepIdx
	}
	return ""
}

func (s *Server) focusPresent(_ *http.Request) string {
	s.FocusedHistoryIdx = -1
	s.HistoryRevealed = false
	return ""
}

func (s *Server) rewindStepNow(r *http.Request) string {
	if stepIdx, err := strconv.Atoi(r.FormValue("step")); err == nil && s.Save != nil && stepIdx >= 0 && stepIdx < len(s.Save.History) {
		targetSceneID := s.Save.History[stepIdx].SceneID
		s.Save.History = s.Save.History[:stepIdx]
		s.Save.CurrentSceneID = targetSceneID
		s.Save.AudioTime = 0
		s.HistoryRevealed = false
		s.FocusedHistoryIdx = -1
		_ = book.WriteSaveState(s.SavesDir, s.Save)
		return "Cofnięto opowieść do wybranej sceny."
	}
	return ""
}

func (s *Server) promptRewindStep(r *http.Request) string {
	if stepIdx, err := strconv.Atoi(r.FormValue("step")); err == nil && s.Save != nil && stepIdx >= 0 && stepIdx < len(s.Save.History) {
		entry := s.Save.History[stepIdx]
		sceneName := entry.SceneName
		if sc := s.findScene(entry.SceneID); sc != nil {
			sceneName = sc.DisplayName()
		}
		if sceneName == "" {
			sceneName = entry.SceneID
		}
		s.ConfirmTitle = "Wznów od tej sceny"
		s.ConfirmMessage = fmt.Sprintf("Czy na pewno chcesz wznowić opowieść od sceny „%s”? Późniejszy postęp oraz dokonane wybory zostaną bezpowrotnie wymazane.", sceneName)
		s.ConfirmPendingAction = "RewindStep"
		s.ConfirmPendingParam = strconv.Itoa(stepIdx)
	}
	return ""
}

func (s *Server) promptResetProgress(r *http.Request) string {
	targetBook := s.ActiveBook
	if reqBookID := strings.TrimSpace(r.FormValue("book")); reqBookID != "" {
		if found, err := s.findBookByID(book.SanitizeBookID(reqBookID)); err == nil && found != nil {
			targetBook = found
		}
	}
	if targetBook != nil {
		s.ConfirmTitle = "Wymaż zapisany postęp"
		s.ConfirmMessage = fmt.Sprintf("Czy na pewno chcesz wyczyścić historię wyborów i zakładkę dla książki „%s” oraz zacząć od pierwszej sceny?", targetBook.Title)
		s.ConfirmPendingAction = "ResetProgress"
		s.ConfirmPendingParam = targetBook.ID
	}
	return ""
}

func (s *Server) cycleAudioSpeed(_ *http.Request) string {
	steps := []float64{0.75, 1.0, 1.25, 1.5, 2.0}
	next := 1.0
	for i, st := range steps {
		if st == s.Settings.PlaybackSpeed {
			next = steps[(i+1)%len(steps)]
			break
		}
	}
	s.Settings.PlaybackSpeed = next
	_ = book.SaveAppSettings(s.SavesDir, s.Settings)
	if s.Save != nil {
		s.Save.PlaybackSpeed = next
		if !s.isHistoryPreview() {
			_ = book.WriteSaveState(s.SavesDir, s.Save)
		}
	}
	return ""
}

func (s *Server) toggleAutoChoice(_ *http.Request) string {
	s.Settings.AutoChoice = !s.Settings.AutoChoice
	_ = book.SaveAppSettings(s.SavesDir, s.Settings)
	if s.Save != nil {
		s.Save.AutoChoice = s.Settings.AutoChoice
		if !s.isHistoryPreview() {
			_ = book.WriteSaveState(s.SavesDir, s.Save)
		}
	}
	return ""
}

func (s *Server) toggleAutoStrategy(_ *http.Request) string {
	if s.Settings.AutoChoiceStrategy == "random" {
		s.Settings.AutoChoiceStrategy = "first"
	} else {
		s.Settings.AutoChoiceStrategy = "random"
	}
	_ = book.SaveAppSettings(s.SavesDir, s.Settings)
	if s.Save != nil {
		s.Save.AutoChoiceStrategy = s.Settings.AutoChoiceStrategy
		if !s.isHistoryPreview() {
			_ = book.WriteSaveState(s.SavesDir, s.Save)
		}
	}
	return ""
}

func (s *Server) playFromScene(r *http.Request) string {
	if !s.isHistoryPreview() && r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	if scID := strings.TrimSpace(r.FormValue("scene")); scID != "" {
		s.SelectedSceneID = scID
	}
	if s.ActiveBook != nil && s.Save != nil && s.SelectedSceneID != "" {
		s.Save.CurrentSceneID = s.SelectedSceneID
		s.Save.AudioTime = 0
		if !s.isHistoryPreview() {
			_ = book.WriteSaveState(s.SavesDir, s.Save)
		}
		s.Settings.ActiveView = "player"
		s.SceneTextModalID = ""
		_ = book.SaveAppSettings(s.SavesDir, s.Settings)
	}
	return ""
}

func (s *Server) executeChoiceTransition(fromScene *book.Scene, choice book.Choice) string {
	if choice.IsEnding || choice.TargetSceneID == "END" {
		s.ConfirmTitle = "Zakończenie opowieści"
		s.ConfirmMessage = "Dotarłeś do zakończenia tej ścieżki. Czy chcesz zresetować postęp i rozpocząć od pierwszej sceny?"
		s.ConfirmPendingAction = "ResetProgress"
		s.ConfirmPendingParam = ""
		return ""
	}
	nextScene := s.findScene(choice.TargetSceneID)
	if nextScene == nil {
		return "Nie znaleziono docelowej sceny: " + choice.Target
	}

	s.Save.History = append(s.Save.History, book.HistoryEntry{
		Step:          len(s.Save.History) + 1,
		SceneID:       fromScene.ID,
		SceneName:     fromScene.Name,
		Chapter:       fromScene.Chapter,
		ChoiceLabel:   choice.Label,
		TargetSceneID: nextScene.ID,
	})
	s.Save.CurrentSceneID = nextScene.ID
	s.Save.AudioTime = 0
	s.SelectedSceneID = nextScene.ID
	s.HistoryRevealed = false
	s.FocusedHistoryIdx = -1
	if !s.isHistoryPreview() {
		_ = book.WriteSaveState(s.SavesDir, s.Save)
	}
	return ""
}
