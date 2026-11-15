package actions

import (
	"math/rand"
	"net/url"
	"strconv"
	"strings"

	book "mojra/domain/book/actions"
	commonstate "mojra/domain/common/state"
	commonviews "mojra/domain/common/views"
	"mojra/domain/i18n"
	"mojra/domain/player/state"
	"mojra/engine"
)

const (
	targetSelApp       = "#app"
	targetSelWorkspace = "#workspace"
	targetSelModals    = "#modals-root"
	targetSelNone      = "none"

	formFieldAudioTime = "audio_time"
	formKeyTime        = "time"
	formKeyChoiceIdx   = "choice_idx"
	formKeyStep        = "step"
	formKeyDelay       = "delay"
	formKeySensitivity = "sensitivity"
	formValEND         = "END"
	strategyRandom     = "random"

	PendingRewindStep    = "RewindStep"
	PendingResetProgress = "ResetProgress"

	actSaveAudioTime       = "SaveAudioTime"
	actChooseBranch        = "ChooseBranch"
	actAutoAdvanceImplicit = "AutoAdvanceImplicit"
	actAudioEnded          = "AudioEnded"
	actSetAutoChoiceDelay  = "SetAutoChoiceDelay"
	actCycleAudioSpeed     = "CycleAudioSpeed"
	actToggleAutoChoice    = "ToggleAutoChoice"
	actPromptRewindStep    = "PromptRewindStep"
	actPromptResetProgress = "PromptResetProgress"
	actConfirmModalClose   = "ConfirmModalClose"
	actConfirmModalAccept  = "ConfirmModalAccept"
	actRevealHistory       = "RevealHistory"
	actFocusHistoryStep    = "FocusHistoryStep"
	actFocusPresent        = "FocusPresent"
)

var audioSpeedSteps = [...]float64{0.75, 1.0, 1.25, 1.5, 2.0}

func tr(playerState *state.PlayerState, key engine.TrKey) string {
	return commonviews.Tr(playerState.UIState, key)
}

func trf(playerState *state.PlayerState, key engine.TrKey, args ...any) string {
	return commonviews.Trf(playerState.UIState, key, args...)
}

func isHistoryPreview(playerState *state.PlayerState) bool {
	return playerState != nil && !playerState.Config.StandaloneMode && playerState.IsHistoryPreview
}

// ExecutePlayerChoiceTransition applies a player choice transition, records history, and persists save state when applicable.
func ExecutePlayerChoiceTransition(playerState *state.PlayerState, fromScene *book.Scene, choice book.Choice) string {
	if playerState == nil || playerState.Book == nil || playerState.Save == nil {
		return ""
	}
	targetBook := playerState.Book
	saveState := playerState.Save
	saveState.Variables = book.EnsureSaveVariables(targetBook, saveState.Variables)
	if !book.EvaluateCondition(choice.Condition, saveState.Variables, targetBook) {
		return ""
	}
	if choice.IsEnding || choice.TargetSceneID == formValEND {
		playerState.Confirm.Prompt(commonstate.ConfirmState{
			Title:         tr(playerState, i18n.TrConfirmEndingTitle),
			Message:       tr(playerState, i18n.TrConfirmEndingMsg),
			PendingAction: PendingResetProgress,
			PendingParam:  "",
			Danger:        false,
			AllowLAN:      true,
		})
		return ""
	}
	nextScene := targetBook.FindScene(choice.TargetSceneID)
	if nextScene == nil {
		return tr(playerState, i18n.TrTargetSceneNotFoundPrefix) + choice.Target
	}

	prevVarsSnapshot := book.CloneVariablesMap(saveState.Variables)
	if choice.Effects != "" {
		book.ApplyEffects(choice.Effects, saveState.Variables, targetBook)
	}
	if nextScene.Effects != "" {
		book.ApplyEffects(nextScene.Effects, saveState.Variables, targetBook)
	}

	saveState.History = append(saveState.History, book.HistoryEntry{
		Step:          len(saveState.History) + 1,
		SceneID:       fromScene.ID,
		SceneName:     fromScene.Name,
		Chapter:       fromScene.Chapter,
		ChoiceLabel:   choice.Label,
		TargetSceneID: nextScene.ID,
		Variables:     prevVarsSnapshot,
	})
	saveState.CurrentSceneID = nextScene.ID
	saveState.AudioTime = 0
	playerState.HistoryRevealed = false
	playerState.FocusedHistoryIdx = -1
	if !playerState.Config.StandaloneMode && !isHistoryPreview(playerState) {
		playerState.PersistSave(playerState.Config.SavesDir)
	}
	return ""
}

// ExecutePlayerAudioEnded evaluates auto-choice transition when scene audio finishes.
func ExecutePlayerAudioEnded(playerState *state.PlayerState) string {
	if playerState == nil || playerState.Book == nil {
		return ""
	}
	autoChoice := (playerState.Settings != nil && playerState.Settings.AutoChoice) || (playerState.Save != nil && playerState.Save.AutoChoice)
	if !autoChoice {
		return ""
	}
	targetBook := playerState.Book
	curScene := targetBook.CurrentScene(playerState.Save)
	if curScene == nil || len(curScene.Choices) == 0 || curScene.ExplicitEnd {
		return ""
	}
	var curVars map[string]string
	if playerState.Save != nil {
		playerState.Save.Variables = book.EnsureSaveVariables(targetBook, playerState.Save.Variables)
		curVars = playerState.Save.Variables
	} else {
		curVars = book.DefaultVariablesMap(targetBook)
	}
	eval := targetBook.EvaluateSceneChoices(curScene, curVars, "")
	candidates := eval.UnlockedNonEnding
	if len(candidates) == 0 {
		candidates = eval.Unlocked
	}
	if len(candidates) == 0 {
		return ""
	}
	chosen := candidates[0]
	strategy := ""
	if playerState.Settings != nil && playerState.Settings.AutoChoiceStrategy != "" {
		strategy = playerState.Settings.AutoChoiceStrategy
	} else if playerState.Save != nil {
		strategy = playerState.Save.AutoChoiceStrategy
	}
	if strategy == strategyRandom && len(candidates) > 1 {
		chosen = candidates[rand.Intn(len(candidates))]
	}
	return ExecutePlayerChoiceTransition(playerState, curScene, chosen)
}

// CyclePlayerAudioSpeed advances playback speed to the next step and updates settings/save.
func CyclePlayerAudioSpeed(playerState *state.PlayerState) {
	if playerState == nil {
		return
	}
	curSpeed := 1.0
	if playerState.Settings != nil && playerState.Settings.PlaybackSpeed > 0 {
		curSpeed = playerState.Settings.PlaybackSpeed
	} else if playerState.Save != nil && playerState.Save.PlaybackSpeed > 0 {
		curSpeed = playerState.Save.PlaybackSpeed
	}
	nextSpeed := 1.0
	for idx, stepVal := range audioSpeedSteps {
		if stepVal == curSpeed {
			nextSpeed = audioSpeedSteps[(idx+1)%len(audioSpeedSteps)]
			break
		}
	}
	if playerState.Settings != nil {
		playerState.Settings.PlaybackSpeed = nextSpeed
		if !playerState.Config.StandaloneMode {
			playerState.SaveSettings()
		}
	}
	if playerState.Save != nil {
		playerState.Save.PlaybackSpeed = nextSpeed
		if !playerState.Config.StandaloneMode && !isHistoryPreview(playerState) {
			playerState.PersistSave(playerState.Config.SavesDir)
		}
	}
}

// TogglePlayerAutoChoice toggles automatic choice selection and updates settings/save.
func TogglePlayerAutoChoice(playerState *state.PlayerState) {
	if playerState == nil {
		return
	}
	nextVal := true
	if playerState.Settings != nil {
		nextVal = !playerState.Settings.AutoChoice
		playerState.Settings.AutoChoice = nextVal
		if !playerState.Config.StandaloneMode {
			playerState.SaveSettings()
		}
	} else if playerState.Save != nil {
		nextVal = !playerState.Save.AutoChoice
	}
	if playerState.Save != nil {
		playerState.Save.AutoChoice = nextVal
		if !playerState.Config.StandaloneMode && !isHistoryPreview(playerState) {
			playerState.PersistSave(playerState.Config.SavesDir)
		}
	}
}

// SetPlayerAutoChoiceDelay updates the automatic choice delay in seconds.
func SetPlayerAutoChoiceDelay(playerState *state.PlayerState, rawDelay string) {
	if playerState == nil {
		return
	}
	parsedDelay, err := strconv.ParseFloat(strings.TrimSpace(rawDelay), 64)
	if err != nil {
		return
	}
	normDelay := book.NormalizeAutoChoiceDelay(parsedDelay)
	if playerState.Settings != nil {
		playerState.Settings.AutoChoiceDelay = normDelay
		if !playerState.Config.StandaloneMode {
			playerState.SaveSettings()
		}
	}
	if playerState.Save != nil {
		playerState.Save.AutoChoiceDelay = normDelay
		if !playerState.Config.StandaloneMode && !isHistoryPreview(playerState) {
			playerState.PersistSave(playerState.Config.SavesDir)
		}
	}
}

// PromptPlayerRewindStep opens the confirmation view for rewinding to a history step.
func PromptPlayerRewindStep(playerState *state.PlayerState, rawStep string) {
	if playerState == nil || playerState.Save == nil {
		return
	}
	stepIdx, err := strconv.Atoi(strings.TrimSpace(rawStep))
	if err != nil || stepIdx < 0 || stepIdx >= len(playerState.Save.History) {
		return
	}
	entry := playerState.Save.History[stepIdx]
	sceneName := entry.SceneName
	if playerState.Book != nil {
		if foundScene := playerState.Book.FindScene(entry.SceneID); foundScene != nil {
			sceneName = foundScene.DisplayName()
		}
	}
	if sceneName == "" {
		sceneName = entry.SceneID
	}
	playerState.Confirm.Prompt(commonstate.ConfirmState{
		Title:         tr(playerState, i18n.TrRewindSceneHint),
		Message:       trf(playerState, i18n.TrConfirmRewindMsgFmt, sceneName),
		PendingAction: PendingRewindStep,
		PendingParam:  strconv.Itoa(stepIdx),
		Danger:        false,
		AllowLAN:      true,
	})
}

// RewindPlayerToStep rewinds the active player progress to stepIdx.
func RewindPlayerToStep(playerState *state.PlayerState, rawStep string) string {
	if playerState == nil || playerState.Save == nil {
		return ""
	}
	stepIdx, err := strconv.Atoi(strings.TrimSpace(rawStep))
	if err != nil || stepIdx < 0 || stepIdx >= len(playerState.Save.History) {
		return ""
	}
	targetSceneID := playerState.Save.History[stepIdx].SceneID
	if len(playerState.Save.History[stepIdx].Variables) > 0 {
		playerState.Save.Variables = book.CloneVariablesMap(playerState.Save.History[stepIdx].Variables)
	} else if playerState.Book != nil {
		playerState.Save.Variables = book.DefaultVariablesMap(playerState.Book)
		if targetSc := playerState.Book.FindScene(targetSceneID); targetSc != nil && targetSc.Effects != "" {
			book.ApplyEffects(targetSc.Effects, playerState.Save.Variables, playerState.Book)
		}
	}
	playerState.Save.History = playerState.Save.History[:stepIdx]
	playerState.Save.CurrentSceneID = targetSceneID
	playerState.Save.AudioTime = 0
	playerState.HistoryRevealed = false
	playerState.FocusedHistoryIdx = -1
	if !playerState.Config.StandaloneMode && !isHistoryPreview(playerState) {
		playerState.PersistSave(playerState.Config.SavesDir)
	}
	return tr(playerState, i18n.TrRewoundStoryToast)
}

// ResetPlayerBookProgress resets the active book's save state to the starting scene.
func ResetPlayerBookProgress(playerState *state.PlayerState) string {
	if playerState == nil || playerState.Book == nil || playerState.Save == nil {
		return ""
	}
	startSceneID := playerState.Book.ResolveValidSceneID(playerState.Book.StartSceneID)
	playerState.Save.CurrentSceneID = startSceneID
	playerState.Save.AudioTime = 0
	playerState.Save.History = nil
	playerState.Save.Variables = book.DefaultVariablesMap(playerState.Book)
	if curScene := playerState.Book.FindScene(startSceneID); curScene != nil && curScene.Effects != "" {
		book.ApplyEffects(curScene.Effects, playerState.Save.Variables, playerState.Book)
	}
	playerState.HistoryRevealed = false
	playerState.FocusedHistoryIdx = -1
	if !playerState.Config.StandaloneMode && !isHistoryPreview(playerState) {
		playerState.PersistSave(playerState.Config.SavesDir)
	}
	return tr(playerState, i18n.TrResetProgressToast)
}

// DispatchPlayerStandaloneAction executes a standalone/WASM player action and returns the target selector and toast message.
func DispatchPlayerStandaloneAction(playerState *state.PlayerState, actionName string, form url.Values) (string, string) {
	if playerState == nil || playerState.Book == nil || playerState.Save == nil {
		return targetSelNone, ""
	}
	if actionName != actSaveAudioTime {
		if rawAudioTime := strings.TrimSpace(form.Get(formFieldAudioTime)); rawAudioTime != "" {
			if parsedTime, err := strconv.ParseFloat(rawAudioTime, 64); err == nil && parsedTime >= 0 {
				playerState.Save.AudioTime = parsedTime
			}
		}
	}

	target := targetSelWorkspace
	toast := ""

	switch actionName {
	case actSaveAudioTime:
		if parsedTime, err := strconv.ParseFloat(form.Get(formKeyTime), 64); err == nil && parsedTime >= 0 {
			playerState.Save.AudioTime = parsedTime
		}
		target = targetSelNone

	case actChooseBranch:
		if choiceIdx, err := strconv.Atoi(form.Get(formKeyChoiceIdx)); err == nil {
			curScene := playerState.Book.CurrentScene(playerState.Save)
			if curScene != nil && choiceIdx >= 0 && choiceIdx < len(curScene.Choices) {
				toast = ExecutePlayerChoiceTransition(playerState, curScene, curScene.Choices[choiceIdx])
			}
		}
		if playerState.Confirm.IsOpen() {
			target = targetSelModals
		}

	case actAutoAdvanceImplicit:
		curScene := playerState.Book.CurrentScene(playerState.Save)
		if curScene != nil && len(curScene.Choices) == 1 && !curScene.Choices[0].IsEnding && curScene.Choices[0].TargetSceneID != formValEND {
			toast = ExecutePlayerChoiceTransition(playerState, curScene, curScene.Choices[0])
		}
		if playerState.Confirm.IsOpen() {
			target = targetSelModals
		}

	case actAudioEnded:
		toast = ExecutePlayerAudioEnded(playerState)
		if playerState.Confirm.IsOpen() {
			target = targetSelModals
		}

	case actSetAutoChoiceDelay:
		SetPlayerAutoChoiceDelay(playerState, form.Get(formKeyDelay))
		target = targetSelNone

	case actCycleAudioSpeed:
		CyclePlayerAudioSpeed(playerState)
		target = targetSelNone

	case actToggleAutoChoice:
		TogglePlayerAutoChoice(playerState)
		target = targetSelNone

	case actPromptRewindStep:
		PromptPlayerRewindStep(playerState, form.Get(formKeyStep))
		target = targetSelModals

	case actPromptResetProgress:
		playerState.Confirm.Prompt(commonstate.ConfirmState{
			Title:         tr(playerState, i18n.TrConfirmResetTitle),
			Message:       trf(playerState, i18n.TrConfirmResetMsgFmt, playerState.Book.Title),
			PendingAction: PendingResetProgress,
			PendingParam:  playerState.Book.ID,
			Danger:        false,
			AllowLAN:      true,
		})
		target = targetSelModals

	case actConfirmModalClose:
		playerState.Confirm.Reset()
		target = targetSelModals

	case actConfirmModalAccept:
		pending := playerState.Confirm.PendingAction
		param := playerState.Confirm.PendingParam
		playerState.Confirm.Reset()
		switch pending {
		case PendingRewindStep:
			toast = RewindPlayerToStep(playerState, param)
		case PendingResetProgress:
			toast = ResetPlayerBookProgress(playerState)
		}
		target = targetSelWorkspace

	case actRevealHistory:
		if len(playerState.Save.History) > 0 {
			playerState.HistoryRevealed = true
			playerState.FocusedHistoryIdx = len(playerState.Save.History) - 1
		}
		target = targetSelNone

	case actFocusHistoryStep:
		if stepIdx, err := strconv.Atoi(form.Get(formKeyStep)); err == nil && stepIdx >= 0 && stepIdx < len(playerState.Save.History) {
			playerState.HistoryRevealed = true
			playerState.FocusedHistoryIdx = stepIdx
		}
		target = targetSelNone

	case actFocusPresent:
		playerState.FocusedHistoryIdx = -1
		playerState.HistoryRevealed = false
		target = targetSelNone
	}

	return target, toast
}
