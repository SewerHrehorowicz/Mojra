//go:build !(js && wasm)

package actions

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	book "mojra/domain/book/actions"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/engine/osbridge"
	"mojra/engine/transport"
)

const (
	formKeyVoiceSection         = "voice_section"
	formKeyIsolatedVoiceSection = "isolated_voice_section"
	formKeyOpen                 = "open"
	formKeyPCMB64               = "pcm_b64"
	formKeySampleRate           = "sample_rate"
	formKeyStartRatio           = "start_ratio"
	formKeyEndRatio             = "end_ratio"
	formKeyDestructive          = "destructive"
	formValFalse                = "false"

	voicePeakFmt               = "%.3f"
	voicePeakStopped           = "stopped"
	voiceStemSep               = "__"
	voiceTrimPref              = "voicetrim:"
	actSelectSceneVoiceSection = "SelectSceneVoiceSection"
)

var _ = registerActions(map[string]ActionHandler{
	"ToggleSceneVoiceRecorder": toggleSceneVoiceRecorderAction,
	"SelectSceneVoiceSection":  selectSceneVoiceSectionAction,
	"StartSceneVoiceCapture":   startSceneVoiceCaptureAction,
	"PollSceneVoiceCapture":    pollSceneVoiceCaptureAction,
	"StopSceneVoiceCapture":    stopSceneVoiceCaptureAction,
	"SaveSceneVoiceRecording":  saveSceneVoiceRecordingAction,
	"TrimSceneVoice":           trimSceneVoiceAction,
	"DeleteSceneVoice":         deleteSceneVoiceAction,
})

func activeVoiceSectionKeyForRequest(s *state.StudioState, sc *book.Scene, r *http.Request) string {
	var cand string
	if r != nil {
		cand = strings.TrimSpace(r.FormValue(formKeyVoiceSection))
		if cand == "" {
			cand = strings.TrimSpace(r.FormValue(formKeyIsolatedVoiceSection))
		}
	}
	if cand == "" {
		cand = strings.TrimSpace(s.Editor.SelectedVoiceSectionKey)
	}
	return resolveSceneVoiceSectionKey(s, sc, cand)
}

func formVoiceSectionKeyForRequest(s *state.StudioState, sc *book.Scene, r *http.Request) string {
	var cand string
	if r != nil {
		cand = strings.TrimSpace(r.FormValue(formKeyIsolatedVoiceSection))
	}
	if cand == "" {
		cand = strings.TrimSpace(s.Editor.SelectedVoiceSectionKey)
	}
	if cand == "" && r != nil && strings.TrimSpace(r.FormValue(formFieldAction)) != actSelectSceneVoiceSection {
		cand = strings.TrimSpace(r.FormValue(formKeyVoiceSection))
	}
	return resolveSceneVoiceSectionKey(s, sc, cand)
}

func resolveSceneVoiceSectionKey(s *state.StudioState, sc *book.Scene, cand string) string {
	if sc != nil {
		secs := book.ExtractSceneVoiceSections(sc, s.Editor.ActiveBook)
		for _, sec := range secs {
			if strings.EqualFold(sec.Key, cand) {
				return sec.Key
			}
		}
		if len(secs) > 0 {
			return secs[0].Key
		}
	}
	if cand == "" {
		return defaultVoiceSectionKey
	}
	return cand
}

func resolveVoiceActionTargetScene(s *state.StudioState, r *http.Request) *book.Scene {
	_, target := resolveEditorSceneFromRequest(s, r, true)
	return target
}

func flushVoiceTargetChangesIfPresent(s *state.StudioState, target *book.Scene, r *http.Request) (*book.Scene, string) {
	b := s.Editor.ActiveBook
	if target == nil || b == nil || isHistoryPreview(s) || b.IsPacked {
		return target, ""
	}
	if !hasSceneTextModalChanges(s, target, r) {
		return target, ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	book.FinalizeBook(b)
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return target, err.Error()
	}
	reloadActiveBook(s)
	target = resolveVoiceActionTargetScene(s, r)
	if target != nil {
		s.Editor.History.RecordStep(s.Editor.ActiveBook, tr(s, i18n.TrVoiceSavedSceneChanges), historyScopeScene, target.ID, s.Editor.SelectedSceneID)
	}
	return target, ""
}

func toggleSceneVoiceRecorderAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := hasFormScene(r)
	if s.Editor.ActiveBook == nil {
		return frameSceneForm(s, "", hadFormScene, true)
	}
	target := resolveVoiceActionTargetScene(s, r)
	if _, errMsg := flushVoiceTargetChangesIfPresent(s, target, r); errMsg != "" {
		return frameSceneForm(s, errMsg, hadFormScene, true)
	}
	if openParam := strings.TrimSpace(r.FormValue(formKeyOpen)); openParam == formValTrue {
		s.Overlays.VoiceRecorder = true
	} else if openParam == formValFalse {
		s.Overlays.VoiceRecorder = false
	} else {
		s.Overlays.VoiceRecorder = !s.Overlays.VoiceRecorder
	}
	return frameSceneForm(s, "", hadFormScene, true)
}

func selectSceneVoiceSectionAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := hasFormScene(r)
	if !s.Config.StandaloneMode && transport.IsLoopbackRequest(r) {
		s.WithUnlocked(func() {
			_, _, _ = osbridge.StopNativeAudioCapture()
		})
	}
	if s.Editor.ActiveBook == nil {
		return frameSceneForm(s, "", hadFormScene, true)
	}
	target := resolveVoiceActionTargetScene(s, r)
	var errMsg string
	if target, errMsg = flushVoiceTargetChangesIfPresent(s, target, r); errMsg != "" {
		return frameSceneForm(s, errMsg, hadFormScene, true)
	}
	secKey := strings.TrimSpace(r.FormValue(formKeyVoiceSection))
	if target != nil {
		secs := book.ExtractSceneVoiceSections(target, s.Editor.ActiveBook)
		for _, sec := range secs {
			if strings.EqualFold(sec.Key, secKey) {
				s.Editor.SelectedVoiceSectionKey = sec.Key
				return frameSceneForm(s, "", hadFormScene, true)
			}
		}
		if len(secs) > 0 {
			s.Editor.SelectedVoiceSectionKey = secs[0].Key
			return frameSceneForm(s, "", hadFormScene, true)
		}
	}
	if secKey != "" {
		s.Editor.SelectedVoiceSectionKey = secKey
	}
	return frameSceneForm(s, "", hadFormScene, true)
}

func startSceneVoiceCaptureAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	if isHistoryPreview(s) {
		s.Overlays.VoicePeak = voicePeakStopped
		return frameNone(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked || !s.Editor.ActiveBook.Narrated {
		s.Overlays.VoicePeak = voicePeakStopped
		return frameNone(s, tr(s, i18n.TrVoiceNoNarratedBook))
	}
	if !s.Config.StandaloneMode && transport.IsLoopbackRequest(r) {
		var startErr error
		s.WithUnlocked(func() {
			startErr = osbridge.StartNativeAudioCapture()
		})
		if startErr != nil {
			s.Overlays.VoicePeak = voicePeakStopped
			return frameNone(s, startErr.Error())
		}
	}
	return frameNone(s, "")
}

func pollSceneVoiceCaptureAction(s *state.StudioState, _ *http.Request) []transport.ActionFrame {
	peak, active := osbridge.PollNativeAudioCapturePeak()
	if active {
		s.Overlays.VoicePeak = fmt.Sprintf(voicePeakFmt, peak)
	} else {
		s.Overlays.VoicePeak = voicePeakStopped
	}
	return frameNone(s, "")
}

func commitSceneVoiceFile(s *state.StudioState, target *book.Scene, secKey, relPath, historyPrefix string) string {
	if book.HasConditionalVoiceDirectives(target.Text) {
		target.SetSectionVoiceMeta(secKey, relPath, 0, 0)
		s.Editor.SelectedVoiceSectionKey = secKey
	} else {
		target.ExplicitVoice = relPath
		target.VoiceTrimStart = 0
		target.VoiceTrimEnd = 0
	}
	s.Overlays.VoiceRecorder = true
	b := s.Editor.ActiveBook
	book.FinalizeBook(b)
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return err.Error()
	}
	reloadActiveBook(s)
	s.Editor.History.RecordStep(s.Editor.ActiveBook, historyPrefix+relPath, historyScopeScene, target.ID, s.Editor.SelectedSceneID)
	return ""
}

func stopSceneVoiceCaptureAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := hasFormScene(r)
	if isHistoryPreview(s) {
		return frameSceneForm(s, tr(s, i18n.TrHistoryPreviewLocked), hadFormScene, false)
	}
	var pcmLE []byte
	var sampleRate int
	var stopErr error
	if !s.Config.StandaloneMode && transport.IsLoopbackRequest(r) {
		s.WithUnlocked(func() {
			pcmLE, sampleRate, stopErr = osbridge.StopNativeAudioCapture()
		})
	} else {
		return frameSceneForm(s, "", hadFormScene, false)
	}
	if stopErr != nil {
		return frameSceneForm(s, stopErr.Error(), hadFormScene, false)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked || !b.Narrated {
		return frameSceneForm(s, tr(s, i18n.TrVoiceNoNarratedBook), hadFormScene, false)
	}
	target := resolveVoiceActionTargetScene(s, r)
	if target == nil {
		return frameSceneForm(s, tr(s, i18n.TrSceneNotFound), hadFormScene, false)
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if hasSceneTextModalChanges(s, target, r) {
		applySceneFormFields(s, target, r)
	}
	secKey := activeVoiceSectionKeyForRequest(s, target, r)
	relPath, err := book.SaveSceneVoiceSectionPCM(b, target, secKey, pcmLE, sampleRate, 0, 1)
	if err != nil {
		return frameSceneForm(s, err.Error(), hadFormScene, false)
	}
	return frameSceneForm(s, commitSceneVoiceFile(s, target, secKey, relPath, tr(s, i18n.TrVoiceSavedPrefix)), hadFormScene, false)
}

func saveSceneVoiceRecordingAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := hasFormScene(r)
	if isHistoryPreview(s) {
		return frameSceneForm(s, tr(s, i18n.TrHistoryPreviewLocked), hadFormScene, false)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return frameSceneForm(s, tr(s, i18n.TrVoiceNoActiveBook), hadFormScene, false)
	}
	if b.IsPacked {
		return frameSceneForm(s, tr(s, i18n.TrVoicePackedBookReadOnly), hadFormScene, false)
	}
	if !b.Narrated {
		return frameSceneForm(s, tr(s, i18n.TrVoiceNarratedDisabled), hadFormScene, false)
	}
	target := resolveVoiceActionTargetScene(s, r)
	if target == nil {
		return frameSceneForm(s, tr(s, i18n.TrSceneNotFound), hadFormScene, false)
	}

	rawB64 := strings.TrimSpace(r.FormValue(formKeyPCMB64))
	if rawB64 == "" {
		return frameSceneForm(s, tr(s, i18n.TrVoiceMissingData), hadFormScene, false)
	}
	pcmLE, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil || len(pcmLE) < 2 {
		return frameSceneForm(s, tr(s, i18n.TrVoiceInvalidStream), hadFormScene, false)
	}
	sampleRate, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeySampleRate)))
	if sampleRate < 8000 || sampleRate > 65535 {
		sampleRate = 24000
	}
	startRatio, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue(formKeyStartRatio)), 64)
	endRatio, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue(formKeyEndRatio)), 64)

	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if hasSceneTextModalChanges(s, target, r) {
		applySceneFormFields(s, target, r)
	}
	secKey := activeVoiceSectionKeyForRequest(s, target, r)
	relPath, err := book.SaveSceneVoiceSectionPCM(b, target, secKey, pcmLE, sampleRate, startRatio, endRatio)
	if err != nil {
		return frameSceneForm(s, err.Error(), hadFormScene, false)
	}
	return frameSceneForm(s, commitSceneVoiceFile(s, target, secKey, relPath, tr(s, i18n.TrVoiceSavedPrefix)), hadFormScene, false)
}

func trimSceneVoiceAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := hasFormScene(r)
	return frameSceneForm(s, trimSceneVoiceToast(s, r), hadFormScene, true)
}

func trimSceneVoiceToast(s *state.StudioState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked || !b.Narrated {
		return ""
	}
	target := resolveVoiceActionTargetScene(s, r)
	if target == nil {
		return ""
	}
	startRatio, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue(formKeyStartRatio)), 64)
	endRatio, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue(formKeyEndRatio)), 64)
	if startRatio < 0 || startRatio >= 1 {
		startRatio = 0
	}
	if endRatio <= 0 || endRatio > 1 {
		endRatio = 1
	}
	if endRatio <= startRatio+0.001 {
		startRatio = 0
		endRatio = 1
	}

	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if hasSceneTextModalChanges(s, target, r) {
		applySceneFormFields(s, target, r)
	}
	secKey := activeVoiceSectionKeyForRequest(s, target, r)
	s.Editor.SelectedVoiceSectionKey = secKey

	isDestructive := strings.EqualFold(strings.TrimSpace(r.FormValue(formKeyDestructive)), formValTrue)
	if !isDestructive {
		if book.HasConditionalVoiceDirectives(target.Text) {
			book.ExtractSceneVoiceSections(target, b)
			var curVoice string
			if sec := target.FindVoiceSection(secKey); sec != nil {
				curVoice = sec.ExplicitVoice
				if curVoice == "" {
					curVoice = sec.AudioFile
				}
				if curVoice == "" {
					curVoice = sec.TargetFile
				}
			}
			target.SetSectionVoiceMeta(secKey, curVoice, startRatio, endRatio)
		} else {
			if startRatio <= 0.002 && endRatio >= 0.998 {
				target.VoiceTrimStart = 0
				target.VoiceTrimEnd = 0
			} else {
				target.VoiceTrimStart = startRatio
				target.VoiceTrimEnd = endRatio
			}
		}
		s.Overlays.VoiceRecorder = true
		book.FinalizeBook(b)
		if err := book.SaveBookScenesToDisk(b); err != nil {
			return err.Error()
		}
		reloadActiveBook(s)
		s.Editor.History.RecordStep(s.Editor.ActiveBook, tr(s, i18n.TrVoiceTrimmedPrefix)+target.DisplayName(), historyScopeScene, voiceTrimPref+target.ID+":"+secKey, s.Editor.SelectedSceneID)
		return ""
	}

	var relPath string
	var err error
	if rawB64 := strings.TrimSpace(r.FormValue(formKeyPCMB64)); rawB64 != "" {
		pcmLE, decErr := base64.StdEncoding.DecodeString(rawB64)
		if decErr != nil {
			return tr(s, i18n.TrVoiceInvalidTrimData)
		}
		sampleRate, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeySampleRate)))
		if sampleRate < 8000 || sampleRate > 65535 {
			sampleRate = 24000
		}
		relPath, err = book.SaveSceneVoiceSectionPCM(b, target, secKey, pcmLE, sampleRate, startRatio, endRatio)
	} else if book.HasConditionalVoiceDirectives(target.Text) {
		book.ExtractSceneVoiceSections(target, b)
		var curVoice string
		if sec := target.FindVoiceSection(secKey); sec != nil {
			curVoice = sec.ExplicitVoice
			if curVoice == "" {
				curVoice = sec.AudioFile
			}
		}
		stemWithSec := book.SceneVoiceBaseStem(target) + voiceStemSep + secKey
		relPath, err = book.TrimSceneVoiceFile(b, stemWithSec, curVoice, startRatio, endRatio)
	} else {
		curVoice := target.ExplicitVoice
		if curVoice == "" {
			curVoice = target.AudioFile
		}
		relPath, err = book.TrimSceneVoiceFile(b, target.ReferenceKey(), curVoice, startRatio, endRatio)
	}
	if err != nil {
		return err.Error()
	}
	return commitSceneVoiceFile(s, target, secKey, relPath, tr(s, i18n.TrVoiceTrimmedPermPrefix))
}

func deleteSceneVoiceAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := hasFormScene(r)
	if isHistoryPreview(s) {
		return frameSceneForm(s, tr(s, i18n.TrHistoryPreviewLocked), hadFormScene, false)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameSceneForm(s, "", hadFormScene, false)
	}
	target := resolveVoiceActionTargetScene(s, r)
	if target == nil {
		return frameSceneForm(s, "", hadFormScene, false)
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if hasSceneTextModalChanges(s, target, r) {
		applySceneFormFields(s, target, r)
	}
	secKey := activeVoiceSectionKeyForRequest(s, target, r)
	s.Editor.SelectedVoiceSectionKey = secKey
	if err := book.DeleteSceneVoiceSectionFile(b, target, secKey); err != nil {
		return frameSceneForm(s, err.Error(), hadFormScene, false)
	}
	book.FinalizeBook(b)
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return frameSceneForm(s, err.Error(), hadFormScene, false)
	}
	reloadActiveBook(s)
	s.Editor.History.RecordStep(s.Editor.ActiveBook, tr(s, i18n.TrVoiceDeleted), historyScopeScene, target.ID, s.Editor.SelectedSceneID)
	return frameSceneForm(s, "", hadFormScene, false)
}
