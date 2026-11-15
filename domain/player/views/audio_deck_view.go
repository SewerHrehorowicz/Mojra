package views

import (
	"fmt"
	"strings"

	book "mojra/domain/book/actions"
	commonviews "mojra/domain/common/views"
	"mojra/domain/i18n"
	"mojra/domain/player/state"
	"mojra/engine/templates"
)

const (
	zeroDuration   = "0:00"
	speedFmt       = "%.2g"
	versionQuery   = "?v="
	versionFmt     = "%s?v=%d"
	defaultDelayMs = 2000
	msPerSec       = 1000
)

type audioDeckTemplate struct {
	AudioDurationLabel string
	AudioSpeed         string
	AutoChoiceEnabled  bool
	AutoChoiceLabel    string
}

type audioDeckScript struct {
	audioQueue         string
	audioTotalDuration float64
	resumeIndex        int
	resumeOffsetSec    float64
	audioSpeed         float64
	autoChoiceEnabled  bool
	autoChoiceLabel    string
	autoChoiceDelayMs  int
	canAutoAdvance     bool
	noAudioToast       string
}

func AudioDeckView(s *state.PlayerState, b *book.Book, sc *book.Scene, vars map[string]string) templates.TemplateChunk {
	if s != nil && s.Player == nil {
		s.Player = s
	}
	t := &audioDeckTemplate{
		AudioDurationLabel: zeroDuration,
		AudioSpeed:         fmt.Sprintf(speedFmt, 1.0),
	}
	eval := b.EvaluateSceneChoices(sc, vars, "")
	var uiState = s.UIState
	scData := &audioDeckScript{
		audioSpeed:        1,
		autoChoiceDelayMs: defaultDelayMs,
		canAutoAdvance:    !eval.IsTerminal && len(eval.Unlocked) > 0,
		noAudioToast:      commonviews.Tr(uiState, i18n.TrNoSceneAudioToast),
	}
	var queue []book.VoiceQueueItem
	if b != nil && b.Narrated && sc != nil {
		queue = book.EvaluateSceneAudioQueue(sc, vars, b)
		if len(queue) == 0 && sc.AudioURL != "" {
			voiceRel := sc.ExplicitVoice
			if voiceRel == "" {
				voiceRel = sc.AudioFile
			}
			durSec, stamp := book.ReadAudioDurationSecondsAndStamp(b, voiceRel)
			urlStr := sc.AudioURL
			if stamp > 0 && !strings.Contains(urlStr, versionQuery) {
				urlStr = fmt.Sprintf(versionFmt, urlStr, stamp)
			}
			ts, te := sc.NormalizedVoiceTrim()
			queue = []book.VoiceQueueItem{{
				AudioURL:    urlStr,
				TrimStart:   ts,
				TrimEnd:     te,
				DurationSec: durSec * (te - ts),
			}}
		}
		if len(queue) > 0 {
			queueSpec, totalDurSec, totalDurLbl := book.EncodeSceneAudioQueue(queue)
			t.AudioDurationLabel = totalDurLbl
			scData.audioQueue = queueSpec
			scData.audioTotalDuration = totalDurSec
			if s != nil && s.Player.Save != nil {
				scData.resumeIndex, scData.resumeOffsetSec = book.ResolveAudioQueueResume(queue, totalDurSec, s.Player.Save.AudioTime)
			}
		}
	}

	if s != nil && s.Settings != nil {
		speed := s.Settings.PlaybackSpeed
		if speed <= 0 {
			speed = 1
		}
		scData.audioSpeed = speed
		scData.autoChoiceEnabled = s.Settings.AutoChoice
		scData.autoChoiceDelayMs = int(book.NormalizeAutoChoiceDelay(s.Settings.AutoChoiceDelay) * msPerSec)
		t.AudioSpeed = fmt.Sprintf(speedFmt, speed)
		t.AutoChoiceEnabled = s.Settings.AutoChoice
		if s.Settings.AutoChoice {
			t.AutoChoiceLabel = commonviews.Tr(uiState, i18n.TrAutoChoiceOnLabel)
		} else {
			t.AutoChoiceLabel = commonviews.Tr(uiState, i18n.TrAutoChoiceOffLabel)
		}
		scData.autoChoiceLabel = t.AutoChoiceLabel
	}

	return templates.TemplateChunk{
		TemplateData: t,
		ScriptData:   scData,
	}
}
