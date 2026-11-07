package gui

import (
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"mojra/internal/book"
)

func (s *Server) saveAudioTime(r *http.Request) string {
	if s.PlayerBook != nil && s.Save != nil {
		if t, err := strconv.ParseFloat(r.FormValue("time"), 64); err == nil && t >= 0 {
			s.Save.AudioTime = t
			s.persistAudioTimeInPlace(t)
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
			var curVars map[string]string
			if s.Save != nil {
				s.Save.Variables = book.EnsureSaveVariables(s.PlayerBook, s.Save.Variables)
				curVars = s.Save.Variables
			} else if s.PlayerBook != nil {
				curVars = book.DefaultVariablesMap(s.PlayerBook)
			}
			var unlocked []book.Choice
			var unlockedNonEnding []book.Choice
			for _, ch := range cur.Choices {
				if !book.EvaluateCondition(ch.Condition, curVars, s.PlayerBook) {
					continue
				}
				unlocked = append(unlocked, ch)
				if !ch.IsEnding && !strings.EqualFold(ch.TargetSceneID, "END") {
					unlockedNonEnding = append(unlockedNonEnding, ch)
				}
			}
			candidates := unlockedNonEnding
			if len(candidates) == 0 {
				candidates = unlocked
			}
			if len(candidates) > 0 {
				chosen := candidates[0]
				if s.Settings.AutoChoiceStrategy == "random" && len(candidates) > 1 {
					chosen = candidates[rand.Intn(len(candidates))]
				}
				return s.executeChoiceTransition(cur, chosen)
			}
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

func (s *Server) promptRewindStep(r *http.Request) string {
	if stepIdx, err := strconv.Atoi(r.FormValue("step")); err == nil && s.Save != nil && stepIdx >= 0 && stepIdx < len(s.Save.History) {
		entry := s.Save.History[stepIdx]
		sceneName := entry.SceneName
		if sc := s.findPlayerScene(entry.SceneID); sc != nil {
			sceneName = sc.DisplayName()
		}
		if sceneName == "" {
			sceneName = entry.SceneID
		}
		s.ConfirmTitle = s.tr(TrRewindSceneHint)
		s.ConfirmMessage = s.trf(TrConfirmRewindMsgFmt, sceneName)
		s.ConfirmPendingAction = "RewindStep"
		s.ConfirmPendingParam = strconv.Itoa(stepIdx)
	}
	return ""
}

func (s *Server) promptResetProgress(r *http.Request) string {
	targetBook := s.PlayerBook
	if reqBookID := strings.TrimSpace(r.FormValue("book")); reqBookID != "" {
		if found, err := s.findBookByID(book.SanitizeBookID(reqBookID)); err == nil && found != nil {
			targetBook = found
		}
	}
	if targetBook != nil {
		s.ConfirmTitle = s.tr(TrConfirmResetTitle)
		s.ConfirmMessage = s.trf(TrConfirmResetMsgFmt, targetBook.Title)
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
	s.persistSettings()
	if s.Save != nil {
		s.Save.PlaybackSpeed = next
		s.persistSaveState()
	}
	return ""
}

func (s *Server) toggleAutoChoice(_ *http.Request) string {
	s.Settings.AutoChoice = !s.Settings.AutoChoice
	s.persistSettings()
	if s.Save != nil {
		s.Save.AutoChoice = s.Settings.AutoChoice
		s.persistSaveState()
	}
	return ""
}

func (s *Server) setAutoChoiceDelay(r *http.Request) string {
	raw := strings.TrimSpace(r.FormValue("delay"))
	if d, err := strconv.ParseFloat(raw, 64); err == nil {
		s.Settings.AutoChoiceDelay = book.NormalizeAutoChoiceDelay(d)
		s.persistSettings()
		if s.Save != nil {
			s.Save.AutoChoiceDelay = s.Settings.AutoChoiceDelay
			s.persistSaveState()
		}
	}
	return ""
}

func (s *Server) setScrollSensitivity(r *http.Request) string {
	s.Settings.ScrollSensitivity = book.NormalizeScrollSensitivity(r.FormValue("sensitivity"))
	s.persistSettings()
	return ""
}

func (s *Server) playFromScene(r *http.Request) string {
	if scID := strings.TrimSpace(r.FormValue("scene")); scID != "" {
		s.SelectedSceneID = scID
	}
	if s.ActiveBook != nil && s.SelectedSceneID != "" {
		if !s.isHistoryPreview() && (s.PlayerBook == nil || s.PlayerBook.ID != s.ActiveBook.ID || s.Save == nil) {
			s.setPlayerBook(s.ActiveBook)
		} else if s.PlayerBook == nil {
			s.PlayerBook = s.ActiveBook
		}
		if s.Save != nil {
			if !strings.EqualFold(s.Save.CurrentSceneID, s.SelectedSceneID) {
				truncated := false
				for i, h := range s.Save.History {
					if strings.EqualFold(h.SceneID, s.SelectedSceneID) {
						s.Save.History = s.Save.History[:i]
						truncated = true
						break
					}
				}
				if !truncated {
					s.Save.History = nil
				}
			}
			s.Save.CurrentSceneID = s.SelectedSceneID
			s.Save.AudioTime = 0
			if len(s.SceneTextTestVars) > 0 {
				s.Save.Variables = book.EnsureSaveVariables(s.PlayerBook, s.SceneTextTestVars)
			} else {
				s.Save.Variables = book.EnsureSaveVariables(s.PlayerBook, s.Save.Variables)
			}
			s.persistSaveState()
		}
		s.Settings.ActiveView = "player"
		s.SceneTextModalID = ""
		s.persistSettings()
	}
	return ""
}

func (s *Server) executeChoiceTransition(fromScene *book.Scene, choice book.Choice) string {
	if s.PlayerBook == nil || s.Save == nil {
		return ""
	}
	s.Save.Variables = book.EnsureSaveVariables(s.PlayerBook, s.Save.Variables)
	if !book.EvaluateCondition(choice.Condition, s.Save.Variables, s.PlayerBook) {
		return ""
	}
	if choice.IsEnding || choice.TargetSceneID == "END" {
		s.ConfirmTitle = s.tr(TrConfirmEndingTitle)
		s.ConfirmMessage = s.tr(TrConfirmEndingMsg)
		s.ConfirmPendingAction = "ResetProgress"
		s.ConfirmPendingParam = ""
		return ""
	}
	nextScene := s.findPlayerScene(choice.TargetSceneID)
	if nextScene == nil {
		return s.tr(TrTargetSceneNotFoundPrefix) + choice.Target
	}

	prevVarsSnapshot := book.CloneVariablesMap(s.Save.Variables)
	if choice.Effects != "" {
		book.ApplyEffects(choice.Effects, s.Save.Variables, s.PlayerBook)
	}
	if nextScene.Effects != "" {
		book.ApplyEffects(nextScene.Effects, s.Save.Variables, s.PlayerBook)
	}

	s.Save.History = append(s.Save.History, book.HistoryEntry{
		Step:          len(s.Save.History) + 1,
		SceneID:       fromScene.ID,
		SceneName:     fromScene.Name,
		Chapter:       fromScene.Chapter,
		ChoiceLabel:   choice.Label,
		TargetSceneID: nextScene.ID,
		Variables:     prevVarsSnapshot,
	})
	s.Save.CurrentSceneID = nextScene.ID
	s.Save.AudioTime = 0
	s.HistoryRevealed = false
	s.FocusedHistoryIdx = -1
	s.persistSaveState()
	return ""
}
