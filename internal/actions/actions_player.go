//go:build !(js && wasm)

package actions

import (
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

func saveAudioTime(s *state.AppState, r *http.Request) string {
	if s.Player.Book != nil && s.Player.Save != nil {
		if t, err := strconv.ParseFloat(r.FormValue("time"), 64); err == nil && t >= 0 {
			s.Player.Save.AudioTime = t
			if !isHistoryPreview(s) {
				if err := book.UpdateSaveAudioTimeInPlace(s.Config.SavesDir, s.Player.Book.ID, t, s.Player.Book.StartSceneID); err != nil {
					s.Player.PersistSave(s.Config.SavesDir)
				}
			}
		}
	}
	return ""
}

func chooseBranch(s *state.AppState, r *http.Request) string {
	idx, err := strconv.Atoi(r.FormValue("choice_idx"))
	cur := s.Player.Book.CurrentScene(s.Player.Save)
	if err == nil && cur != nil && idx >= 0 && idx < len(cur.Choices) {
		return executeChoiceTransition(s, cur, cur.Choices[idx])
	}
	return ""
}

func autoAdvanceImplicit(s *state.AppState, _ *http.Request) string {
	cur := s.Player.Book.CurrentScene(s.Player.Save)
	if cur != nil && len(cur.Choices) == 1 && !cur.Choices[0].IsEnding && cur.Choices[0].TargetSceneID != "END" {
		return executeChoiceTransition(s, cur, cur.Choices[0])
	}
	return ""
}

func audioEnded(s *state.AppState, _ *http.Request) string {
	if s.Settings.AutoChoice {
		cur := s.Player.Book.CurrentScene(s.Player.Save)
		if cur != nil && len(cur.Choices) > 0 && !cur.ExplicitEnd {
			var curVars map[string]string
			if s.Player.Save != nil {
				s.Player.Save.Variables = book.EnsureSaveVariables(s.Player.Book, s.Player.Save.Variables)
				curVars = s.Player.Save.Variables
			} else if s.Player.Book != nil {
				curVars = book.DefaultVariablesMap(s.Player.Book)
			}
			var unlocked []book.Choice
			var unlockedNonEnding []book.Choice
			for _, ch := range cur.Choices {
				if !book.EvaluateCondition(ch.Condition, curVars, s.Player.Book) {
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
				return executeChoiceTransition(s, cur, chosen)
			}
		}
	}
	return ""
}

func revealHistory(s *state.AppState, _ *http.Request) string {
	if s.Player.Save != nil && len(s.Player.Save.History) > 0 {
		s.Player.HistoryRevealed = true
		s.Player.FocusedHistoryIdx = len(s.Player.Save.History) - 1
	}
	return ""
}

func focusHistoryStep(s *state.AppState, r *http.Request) string {
	if stepIdx, err := strconv.Atoi(r.FormValue("step")); err == nil && s.Player.Save != nil && stepIdx >= 0 && stepIdx < len(s.Player.Save.History) {
		s.Player.HistoryRevealed = true
		s.Player.FocusedHistoryIdx = stepIdx
	}
	return ""
}

func focusPresent(s *state.AppState, _ *http.Request) string {
	s.Player.FocusedHistoryIdx = -1
	s.Player.HistoryRevealed = false
	return ""
}

func cycleAudioSpeed(s *state.AppState, _ *http.Request) string {
	steps := []float64{0.75, 1.0, 1.25, 1.5, 2.0}
	next := 1.0
	for i, st := range steps {
		if st == s.Settings.PlaybackSpeed {
			next = steps[(i+1)%len(steps)]
			break
		}
	}
	s.Settings.PlaybackSpeed = next
	_ = s.SaveSettings()
	if s.Player.Save != nil {
		s.Player.Save.PlaybackSpeed = next
		if !isHistoryPreview(s) {
			s.Player.PersistSave(s.Config.SavesDir)
		}
	}
	return ""
}

func toggleAutoChoice(s *state.AppState, _ *http.Request) string {
	s.Settings.AutoChoice = !s.Settings.AutoChoice
	_ = s.SaveSettings()
	if s.Player.Save != nil {
		s.Player.Save.AutoChoice = s.Settings.AutoChoice
		if !isHistoryPreview(s) {
			s.Player.PersistSave(s.Config.SavesDir)
		}
	}
	return ""
}

func setAutoChoiceDelay(s *state.AppState, r *http.Request) string {
	raw := strings.TrimSpace(r.FormValue("delay"))
	if d, err := strconv.ParseFloat(raw, 64); err == nil {
		s.Settings.AutoChoiceDelay = book.NormalizeAutoChoiceDelay(d)
		_ = s.SaveSettings()
		if s.Player.Save != nil {
			s.Player.Save.AutoChoiceDelay = s.Settings.AutoChoiceDelay
			if !isHistoryPreview(s) {
				s.Player.PersistSave(s.Config.SavesDir)
			}
		}
	}
	return ""
}

func setScrollSensitivity(s *state.AppState, r *http.Request) string {
	s.Settings.ScrollSensitivity = book.NormalizeScrollSensitivity(r.FormValue("sensitivity"))
	_ = s.SaveSettings()
	return ""
}

func playFromScene(s *state.AppState, r *http.Request) string {
	if scID := strings.TrimSpace(r.FormValue("scene")); scID != "" {
		s.Editor.SelectedSceneID = scID
	}
	b := s.Editor.ActiveBook
	if b != nil && s.Editor.SelectedSceneID != "" {
		if !isHistoryPreview(s) && (s.Player.Book == nil || s.Player.Book.ID != b.ID || s.Player.Save == nil) {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, b)
		} else if s.Player.Book == nil {
			s.Player.Book = b
		}
		if s.Player.Save != nil {
			if !strings.EqualFold(s.Player.Save.CurrentSceneID, s.Editor.SelectedSceneID) {
				truncated := false
				for i, h := range s.Player.Save.History {
					if strings.EqualFold(h.SceneID, s.Editor.SelectedSceneID) {
						if len(h.Variables) > 0 {
							s.Player.Save.Variables = book.CloneVariablesMap(h.Variables)
						}
						s.Player.Save.History = s.Player.Save.History[:i]
						truncated = true
						break
					}
				}
				if !truncated {
					s.Player.Save.History = nil
					s.Player.Save.Variables = nil
				}
			}
			s.Player.Save.CurrentSceneID = s.Editor.SelectedSceneID
			s.Player.Save.AudioTime = 0
			book.EnsureHistoryFromStart(s.Player.Book, s.Player.Save)
			if len(s.Editor.SceneText.TestVars) > 0 {
				s.Player.Save.Variables = book.EnsureSaveVariables(s.Player.Book, s.Editor.SceneText.TestVars)
			} else {
				s.Player.Save.Variables = book.EnsureSaveVariables(s.Player.Book, s.Player.Save.Variables)
			}
			if !isHistoryPreview(s) {
				s.Player.PersistSave(s.Config.SavesDir)
			}
		}
		s.Editor.SaveBookUIState(s.Settings)
		s.Settings.ActiveView = state.ViewPlayer.String()
		s.Editor.SceneText.SceneID = ""
		s.Editor.SyncToSettings(s.Settings)
		_ = s.SaveSettings()
	}
	return ""
}

func executeChoiceTransition(s *state.AppState, fromScene *book.Scene, choice book.Choice) string {
	if s.Player.Book == nil || s.Player.Save == nil {
		return ""
	}
	s.Player.Save.Variables = book.EnsureSaveVariables(s.Player.Book, s.Player.Save.Variables)
	if !book.EvaluateCondition(choice.Condition, s.Player.Save.Variables, s.Player.Book) {
		return ""
	}
	if choice.IsEnding || choice.TargetSceneID == "END" {
		s.ModalConfirm.Prompt(state.ModalConfirmState{
			Title:         tr(s, i18n.TrConfirmEndingTitle),
			Message:       tr(s, i18n.TrConfirmEndingMsg),
			PendingAction: ResetProgress,
			PendingParam:  "",
			Danger:        false,
		})
		return ""
	}
	nextScene := s.Player.Book.FindScene(choice.TargetSceneID)
	if nextScene == nil {
		return tr(s, i18n.TrTargetSceneNotFoundPrefix) + choice.Target
	}

	prevVarsSnapshot := book.CloneVariablesMap(s.Player.Save.Variables)
	if choice.Effects != "" {
		book.ApplyEffects(choice.Effects, s.Player.Save.Variables, s.Player.Book)
	}
	if nextScene.Effects != "" {
		book.ApplyEffects(nextScene.Effects, s.Player.Save.Variables, s.Player.Book)
	}

	s.Player.Save.History = append(s.Player.Save.History, book.HistoryEntry{
		Step:          len(s.Player.Save.History) + 1,
		SceneID:       fromScene.ID,
		SceneName:     fromScene.Name,
		Chapter:       fromScene.Chapter,
		ChoiceLabel:   choice.Label,
		TargetSceneID: nextScene.ID,
		Variables:     prevVarsSnapshot,
	})
	s.Player.Save.CurrentSceneID = nextScene.ID
	s.Player.Save.AudioTime = 0
	s.Player.HistoryRevealed = false
	s.Player.FocusedHistoryIdx = -1
	if !isHistoryPreview(s) {
		s.Player.PersistSave(s.Config.SavesDir)
	}
	return ""
}
