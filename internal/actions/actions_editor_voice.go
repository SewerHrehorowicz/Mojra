//go:build !(js && wasm)

package actions

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/osbridge"
	"mojra/internal/state"
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

	voicePeakFmt     = "%.3f"
	voicePeakStopped = "stopped"
	voiceStemSep     = "__"
	voiceTrimPref    = "voicetrim:"

	msgVoiceSavedSceneChanges = "Zapisano zmiany w scenie"
	msgVoiceNoNarratedBook    = "Brak aktywnej książki z włączoną narracją"
	msgVoiceNoActiveBook      = "Brak aktywnej książki"
	msgVoicePackedBookReadOnly = "Nie można edytować spakowanego archiwum .playbook bezpośrednio"
	msgVoiceNarratedDisabled  = "Książka nie posiada włączonej narracji (narrated: false)"
	msgVoiceSceneNotFound     = "Nie znaleziono sceny"
	msgVoiceMissingData       = "Brak danych nagrania audio"
	msgVoiceInvalidStream     = "Nieprawidłowy strumień nagrania audio"
	msgVoiceInvalidTrimData   = "Nieprawidłowe dane audio do przycięcia"
	msgVoiceSavedPrefix       = "Zapisano nagranie audio: "
	msgVoiceTrimmedPrefix     = "Przycięto nagranie audio: "
	msgVoiceTrimmedPermPrefix = "Przycięto trwale plik audio: "
	msgVoiceDeleted           = "Usunięto nagranie audio sceny"
)

func activeVoiceSectionKeyForRequest(s *state.AppState, sc *book.Scene, r *http.Request) string {
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

func formVoiceSectionKeyForRequest(s *state.AppState, sc *book.Scene, r *http.Request) string {
	var cand string
	if r != nil {
		cand = strings.TrimSpace(r.FormValue(formKeyIsolatedVoiceSection))
	}
	if cand == "" {
		cand = strings.TrimSpace(s.Editor.SelectedVoiceSectionKey)
	}
	if cand == "" && r != nil && strings.TrimSpace(r.FormValue(formFieldAction)) != SelectSceneVoiceSection {
		cand = strings.TrimSpace(r.FormValue(formKeyVoiceSection))
	}
	return resolveSceneVoiceSectionKey(s, sc, cand)
}

func resolveSceneVoiceSectionKey(s *state.AppState, sc *book.Scene, cand string) string {
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

func resolveVoiceActionTargetScene(s *state.AppState, r *http.Request) *book.Scene {
	_, target := resolveEditorSceneFromRequest(s, r, true)
	return target
}

func flushVoiceTargetChangesIfPresent(s *state.AppState, target *book.Scene, r *http.Request) (*book.Scene, string) {
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
		s.Editor.History.RecordStep(s.Editor.ActiveBook, msgVoiceSavedSceneChanges, historyScopeScene, target.ID, s.Editor.SelectedSceneID)
	}
	return target, ""
}

func toggleSceneVoiceRecorderAction(s *state.AppState, r *http.Request) string {
	if s.Editor.ActiveBook == nil {
		return ""
	}
	target := resolveVoiceActionTargetScene(s, r)
	if _, errMsg := flushVoiceTargetChangesIfPresent(s, target, r); errMsg != "" {
		return errMsg
	}
	if openParam := strings.TrimSpace(r.FormValue(formKeyOpen)); openParam == formValTrue {
		s.Modals.VoiceRecorder = true
	} else if openParam == formValFalse {
		s.Modals.VoiceRecorder = false
	} else {
		s.Modals.VoiceRecorder = !s.Modals.VoiceRecorder
	}
	return ""
}

func selectSceneVoiceSectionAction(s *state.AppState, r *http.Request) string {
	if !s.Config.StandaloneMode && isLoopbackRequest(r) {
		s.WithUnlocked(func() {
			_, _, _ = osbridge.StopNativeAudioCapture()
		})
	}
	if s.Editor.ActiveBook == nil {
		return ""
	}
	target := resolveVoiceActionTargetScene(s, r)
	var errMsg string
	if target, errMsg = flushVoiceTargetChangesIfPresent(s, target, r); errMsg != "" {
		return errMsg
	}
	secKey := strings.TrimSpace(r.FormValue(formKeyVoiceSection))
	if target != nil {
		secs := book.ExtractSceneVoiceSections(target, s.Editor.ActiveBook)
		for _, sec := range secs {
			if strings.EqualFold(sec.Key, secKey) {
				s.Editor.SelectedVoiceSectionKey = sec.Key
				return ""
			}
		}
		if len(secs) > 0 {
			s.Editor.SelectedVoiceSectionKey = secs[0].Key
			return ""
		}
	}
	if secKey != "" {
		s.Editor.SelectedVoiceSectionKey = secKey
	}
	return ""
}

func startSceneVoiceCaptureAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		s.Modals.VoicePeak = voicePeakStopped
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked || !s.Editor.ActiveBook.Narrated {
		s.Modals.VoicePeak = voicePeakStopped
		return msgVoiceNoNarratedBook
	}
	if !s.Config.StandaloneMode && isLoopbackRequest(r) {
		var startErr error
		s.WithUnlocked(func() {
			startErr = osbridge.StartNativeAudioCapture()
		})
		if startErr != nil {
			s.Modals.VoicePeak = voicePeakStopped
			return startErr.Error()
		}
	}
	return ""
}

func pollSceneVoiceCaptureAction(s *state.AppState, _ *http.Request) string {
	peak, active := osbridge.PollNativeAudioCapturePeak()
	if active {
		s.Modals.VoicePeak = fmt.Sprintf(voicePeakFmt, peak)
	} else {
		s.Modals.VoicePeak = voicePeakStopped
	}
	return ""
}

func commitSceneVoiceFile(s *state.AppState, target *book.Scene, secKey, relPath, historyPrefix string) string {
	if book.HasConditionalVoiceDirectives(target.Text) {
		target.SetSectionVoiceMeta(secKey, relPath, 0, 0)
		s.Editor.SelectedVoiceSectionKey = secKey
	} else {
		target.ExplicitVoice = relPath
		target.VoiceTrimStart = 0
		target.VoiceTrimEnd = 0
	}
	s.Modals.VoiceRecorder = true
	b := s.Editor.ActiveBook
	book.FinalizeBook(b)
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return err.Error()
	}
	reloadActiveBook(s)
	s.Editor.History.RecordStep(s.Editor.ActiveBook, historyPrefix+relPath, historyScopeScene, target.ID, s.Editor.SelectedSceneID)
	return ""
}

func stopSceneVoiceCaptureAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	var pcmLE []byte
	var sampleRate int
	var stopErr error
	if !s.Config.StandaloneMode && isLoopbackRequest(r) {
		s.WithUnlocked(func() {
			pcmLE, sampleRate, stopErr = osbridge.StopNativeAudioCapture()
		})
	} else {
		return ""
	}
	if stopErr != nil {
		return stopErr.Error()
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked || !b.Narrated {
		return msgVoiceNoNarratedBook
	}
	target := resolveVoiceActionTargetScene(s, r)
	if target == nil {
		return msgVoiceSceneNotFound
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if hasSceneTextModalChanges(s, target, r) {
		applySceneFormFields(s, target, r)
	}
	secKey := activeVoiceSectionKeyForRequest(s, target, r)
	relPath, err := book.SaveSceneVoiceSectionPCM(b, target, secKey, pcmLE, sampleRate, 0, 1)
	if err != nil {
		return err.Error()
	}
	return commitSceneVoiceFile(s, target, secKey, relPath, msgVoiceSavedPrefix)
}

func saveSceneVoiceRecordingAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return msgVoiceNoActiveBook
	}
	if b.IsPacked {
		return msgVoicePackedBookReadOnly
	}
	if !b.Narrated {
		return msgVoiceNarratedDisabled
	}
	target := resolveVoiceActionTargetScene(s, r)
	if target == nil {
		return msgVoiceSceneNotFound
	}

	rawB64 := strings.TrimSpace(r.FormValue(formKeyPCMB64))
	if rawB64 == "" {
		return msgVoiceMissingData
	}
	pcmLE, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil || len(pcmLE) < 2 {
		return msgVoiceInvalidStream
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
		return err.Error()
	}
	return commitSceneVoiceFile(s, target, secKey, relPath, msgVoiceSavedPrefix)
}

func trimSceneVoiceAction(s *state.AppState, r *http.Request) string {
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
		s.Modals.VoiceRecorder = true
		book.FinalizeBook(b)
		if err := book.SaveBookScenesToDisk(b); err != nil {
			return err.Error()
		}
		reloadActiveBook(s)
		s.Editor.History.RecordStep(s.Editor.ActiveBook, msgVoiceTrimmedPrefix+target.DisplayName(), historyScopeScene, voiceTrimPref+target.ID+":"+secKey, s.Editor.SelectedSceneID)
		return ""
	}

	var relPath string
	var err error
	if rawB64 := strings.TrimSpace(r.FormValue(formKeyPCMB64)); rawB64 != "" {
		pcmLE, decErr := base64.StdEncoding.DecodeString(rawB64)
		if decErr != nil {
			return msgVoiceInvalidTrimData
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
	return commitSceneVoiceFile(s, target, secKey, relPath, msgVoiceTrimmedPermPrefix)
}

func deleteSceneVoiceAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	target := resolveVoiceActionTargetScene(s, r)
	if target == nil {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if hasSceneTextModalChanges(s, target, r) {
		applySceneFormFields(s, target, r)
	}
	secKey := activeVoiceSectionKeyForRequest(s, target, r)
	s.Editor.SelectedVoiceSectionKey = secKey
	if err := book.DeleteSceneVoiceSectionFile(b, target, secKey); err != nil {
		return err.Error()
	}
	book.FinalizeBook(b)
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return err.Error()
	}
	reloadActiveBook(s)
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msgVoiceDeleted, historyScopeScene, target.ID, s.Editor.SelectedSceneID)
	return ""
}
