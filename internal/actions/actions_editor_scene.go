//go:build !(js && wasm)

package actions

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

func splitInlineChoiceLabelAndTarget(raw string) (label string, target string, hasArrow bool) {
	cleaned := book.SanitizePlainText(raw)
	if idx := strings.LastIndex(cleaned, "->"); idx >= 0 {
		if idx == 0 || cleaned[idx-1] != '\\' {
			lbl := strings.TrimSpace(cleaned[:idx])
			tgt := strings.TrimSpace(cleaned[idx+2:])
			return lbl, tgt, true
		}
	}
	return strings.TrimSpace(cleaned), "", false
}

func activeVoiceSectionKeyForRequest(s *state.AppState, sc *book.Scene, r *http.Request) string {
	var cand string
	if r != nil {
		cand = strings.TrimSpace(r.FormValue("voice_section"))
		if cand == "" {
			cand = strings.TrimSpace(r.FormValue("isolated_voice_section"))
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
		cand = strings.TrimSpace(r.FormValue("isolated_voice_section"))
	}
	if cand == "" {
		cand = strings.TrimSpace(s.Editor.SelectedVoiceSectionKey)
	}
	if cand == "" && r != nil && strings.TrimSpace(r.FormValue("action")) != SelectSceneVoiceSection {
		cand = strings.TrimSpace(r.FormValue("voice_section"))
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
		return "01"
	}
	return cand
}

func hasSceneTextModalChanges(s *state.AppState, curSc *book.Scene, r *http.Request) bool {
	if curSc == nil {
		return false
	}
	b := s.Editor.ActiveBook
	if _, hasText := r.Form["text"]; hasText {
		if isolatedKey := strings.TrimSpace(r.FormValue("isolated_voice_section")); isolatedKey != "" {
			if book.ReplaceSceneVoiceSectionText(curSc.Text, isolatedKey, r.FormValue("text")) != curSc.Text {
				return true
			}
		} else if book.SanitizePlainText(r.FormValue("text")) != curSc.Text {
			return true
		}
	}
	if _, hasName := r.Form["name"]; hasName {
		newName := book.SanitizePlainText(r.FormValue("name"))
		if (newName != "" || curSc.ExplicitID != "") && newName != curSc.Name {
			return true
		}
	}
	if _, hasID := r.Form["explicit_id"]; hasID {
		if book.SanitizeSceneID(r.FormValue("explicit_id")) != curSc.ExplicitID {
			return true
		}
	}
	if _, hasSlug := r.Form["slugline"]; hasSlug {
		if strings.ToUpper(strings.TrimSpace(book.SanitizePlainText(r.FormValue("slugline")))) != curSc.Slugline {
			return true
		}
	}
	if _, hasSyn := r.Form["synopsis"]; hasSyn {
		if strings.TrimSpace(book.SanitizePlainText(r.FormValue("synopsis"))) != curSc.Synopsis {
			return true
		}
	}
	if _, hasEff := r.Form["scene_effects"]; hasEff {
		if book.StripOuterDoubleBraces(r.FormValue("scene_effects")) != curSc.Effects {
			return true
		}
	}
	if _, hasCh := r.Form["chapter"]; hasCh {
		newCh := strings.TrimSpace(book.SanitizePlainText(r.FormValue("chapter")))
		if newCh != "" && !strings.EqualFold(newCh, strings.TrimSpace(curSc.Chapter)) {
			return true
		}
	}
	if _, hasImg := r.Form["explicit_image"]; hasImg {
		curImg := curSc.ExplicitImage
		if curImg == "" && curSc.ImageFile != "" {
			curImg = curSc.ImageFile
		}
		if book.SanitizeRelativeMediaPath(r.FormValue("explicit_image")) != curImg {
			return true
		}
	}
	if b != nil && b.Narrated {
		if _, hasVoice := r.Form["explicit_voice"]; hasVoice {
			newVoice := book.SanitizeRelativeMediaPath(r.FormValue("explicit_voice"))
			if book.HasConditionalVoiceDirectives(curSc.Text) {
				secKey := formVoiceSectionKeyForRequest(s, curSc, r)
				book.ExtractSceneVoiceSections(curSc, b)
				var curVoice string
				if sec := curSc.FindVoiceSection(secKey); sec != nil {
					curVoice = sec.ExplicitVoice
					if curVoice == "" && sec.AudioFile != "" {
						curVoice = sec.AudioFile
					}
				}
				if newVoice != curVoice {
					return true
				}
			} else {
				curVoice := curSc.ExplicitVoice
				if curVoice == "" && curSc.AudioFile != "" {
					curVoice = curSc.AudioFile
				}
				if newVoice != curVoice {
					return true
				}
			}
		}
	}
	for i, ch := range curSc.Choices {
		lblKey := fmt.Sprintf("choice_label_%d", i)
		var inlineTgt string
		var hasArrow bool
		if _, hasLbl := r.Form[lblKey]; hasLbl {
			var newLbl string
			newLbl, inlineTgt, hasArrow = splitInlineChoiceLabelAndTarget(r.FormValue(lblKey))
			if newLbl == "" {
				newLbl = fmt.Sprintf("Wybór %d", i+1)
			}
			if newLbl != ch.Label {
				return true
			}
		}
		tgtKey := fmt.Sprintf("choice_target_%d", i)
		newTgt := ""
		if hasArrow && inlineTgt != "" {
			newTgt = inlineTgt
		} else if _, hasTgt := r.Form[tgtKey]; hasTgt {
			newTgt = book.SanitizePlainText(r.FormValue(tgtKey))
		}
		if newTgt != "" && !strings.EqualFold(newTgt, ch.Target) && !strings.EqualFold(newTgt, ch.TargetSceneID) {
			return true
		}
		condKey := fmt.Sprintf("choice_cond_%d", i)
		if _, hasCond := r.Form[condKey]; hasCond {
			rawCond := book.StripOuterDoubleBraces(r.FormValue(condKey))
			showLocked := strings.HasPrefix(rawCond, "?")
			if showLocked {
				rawCond = strings.TrimSpace(rawCond[1:])
			}
			if rawCond != ch.Condition || showLocked != ch.ShowWhenLocked {
				return true
			}
		}
		effKey := fmt.Sprintf("choice_effects_%d", i)
		if _, hasEff := r.Form[effKey]; hasEff {
			if book.StripOuterDoubleBraces(r.FormValue(effKey)) != ch.Effects {
				return true
			}
		}
	}
	if len(curSc.Choices) == 0 || curSc.ExplicitEnd {
		if _, hasTgt0 := r.Form["choice_target_0"]; hasTgt0 {
			newTgt := book.SanitizePlainText(r.FormValue("choice_target_0"))
			if newTgt != "" && !strings.EqualFold(newTgt, "END") {
				return true
			}
		}
	}
	return false
}

func applySceneFormFields(s *state.AppState, target *book.Scene, r *http.Request) {
	b := s.Editor.ActiveBook
	if target == nil || b == nil {
		return
	}
	if formSceneID := strings.TrimSpace(r.FormValue("scene_id")); formSceneID != "" && !strings.EqualFold(formSceneID, target.ID) {
		if !(s.Editor.SceneText.InitialID != "" && strings.EqualFold(formSceneID, s.Editor.SceneText.InitialID) && strings.EqualFold(s.Editor.SceneText.SceneID, target.ID)) {
			return
		}
	}
	oldID := target.ID
	oldName := target.Name
	oldExplicitID := target.ExplicitID
	if _, hasID := r.Form["explicit_id"]; hasID {
		rawExp := strings.TrimSpace(r.FormValue("explicit_id"))
		if rawExp != "" {
			cleanExp := book.SanitizeSceneID(rawExp)
			if cleanExp != "" && !strings.EqualFold(cleanExp, oldExplicitID) && !strings.EqualFold(cleanExp, oldID) {
				cleanExp = book.NextUniqueSceneSlug(b, cleanExp, target)
			}
			if cleanExp != "" {
				target.ExplicitID = cleanExp
			}
		} else if oldExplicitID != "" {
			target.ExplicitID = oldExplicitID
		} else if oldID != "" {
			target.ExplicitID = oldID
		}
	}
	if _, hasName := r.Form["name"]; hasName {
		newName := book.SanitizePlainText(r.FormValue("name"))
		if newName != "" || target.ExplicitID != "" {
			target.Name = newName
		}
	}
	if target.ExplicitID == "" {
		if oldExplicitID != "" {
			target.ExplicitID = oldExplicitID
		} else if oldID != "" {
			target.ExplicitID = oldID
		} else {
			target.ExplicitID = b.NextUniqueAutoSceneID(nil)
		}
	}
	if b != nil {
		if oldName != "" && !strings.EqualFold(target.Name, oldName) {
			b.UnbindAutoSceneName(oldName)
		}
		if target.ExplicitID != "" && !strings.EqualFold(target.ExplicitID, oldID) {
			b.UnbindAutoSceneName(target.Name)
			if book.ParseAutoSceneSeq(oldID) > 0 {
				b.RetireSceneID(oldID)
			}
			if book.ParseAutoSceneSeq(oldExplicitID) > 0 {
				b.RetireSceneID(oldExplicitID)
			}
		}
	}
	if _, hasSlug := r.Form["slugline"]; hasSlug {
		target.Slugline = strings.ToUpper(strings.TrimSpace(book.SanitizePlainText(r.FormValue("slugline"))))
	}
	if _, hasSyn := r.Form["synopsis"]; hasSyn {
		target.Synopsis = strings.TrimSpace(book.SanitizePlainText(r.FormValue("synopsis")))
	}
	if _, hasEff := r.Form["scene_effects"]; hasEff {
		target.Effects = book.CleanEffectsExpr(r.FormValue("scene_effects"))
	}
	if _, hasImg := r.Form["explicit_image"]; hasImg {
		target.ExplicitImage = book.SanitizeRelativeMediaPath(r.FormValue("explicit_image"))
		if target.ExplicitImage == "" {
			target.ImageFile = ""
			target.ImageURL = ""
		}
	}
	if _, hasText := r.Form["text"]; hasText {
		if isolatedKey := strings.TrimSpace(r.FormValue("isolated_voice_section")); isolatedKey != "" {
			target.Text = book.ReplaceSceneVoiceSectionText(target.Text, isolatedKey, r.FormValue("text"))
		} else {
			target.Text = book.SanitizePlainText(r.FormValue("text"))
		}
	}

	if !b.Narrated {
		target.ExplicitVoice = ""
		target.VoiceTrimStart = 0
		target.VoiceTrimEnd = 0
		target.SectionVoices = nil
		target.AudioFile = ""
		target.AudioURL = ""
	} else if _, hasVoice := r.Form["explicit_voice"]; hasVoice {
		newVoice := book.SanitizeRelativeMediaPath(r.FormValue("explicit_voice"))
		if book.HasConditionalVoiceDirectives(target.Text) {
			secKey := activeVoiceSectionKeyForRequest(s, target, r)
			book.ExtractSceneVoiceSections(target, b)
			var prevVoice string
			var prevTS, prevTE float64 = 0, 1
			if sec := target.FindVoiceSection(secKey); sec != nil {
				prevVoice = sec.ExplicitVoice
				if prevVoice == "" {
					prevVoice = sec.AudioFile
				}
				prevTS, prevTE = sec.TrimStart, sec.TrimEnd
			}
			if newVoice == "" {
				target.DeleteSectionVoiceMeta(secKey)
			} else if !strings.EqualFold(newVoice, prevVoice) {
				target.SetSectionVoiceMeta(secKey, newVoice, 0, 1)
			} else if book.HasVoiceTrimValues(prevTS, prevTE) {
				target.SetSectionVoiceMeta(secKey, newVoice, prevTS, prevTE)
			}
		} else {
			if newVoice != target.ExplicitVoice {
				target.VoiceTrimStart = 0
				target.VoiceTrimEnd = 0
			}
			target.ExplicitVoice = newVoice
			if target.ExplicitVoice == "" {
				target.AudioFile = ""
				target.AudioURL = ""
			}
		}
	}
	chapterChanged := false
	act := strings.TrimSpace(r.FormValue("action"))
	if _, hasChapter := r.Form["chapter"]; hasChapter && act != "CreateSceneAt" && act != "CreateChapterGroup" && act != "SelectStudioChapter" && act != "PromptDeleteChapter" && act != "DeleteChapter" {
		newCh := book.SanitizePlainText(r.FormValue("chapter"))
		if !strings.EqualFold(strings.TrimSpace(target.Chapter), newCh) {
			chapterChanged = true
		}
		target.Chapter = newCh
	}
	if rawSF := r.FormValue("source_file"); rawSF != "" && !chapterChanged {
		sf := filepath.Base(book.SanitizePlainText(rawSF))
		if sf != "" && sf != "." {
			target.SourceFile = sf
		}
	}
	if chapterChanged && b != nil {
		if chMeta := b.FindChapter(target.Chapter); chMeta != nil {
			target.ChapterID = chMeta.ExplicitID
			if target.ChapterID == "" {
				target.ChapterID = chMeta.ID
			}
			if chMeta.SourceFile != "" {
				target.SourceFile = chMeta.SourceFile
			}
		} else {
			target.ChapterID = book.NextUniqueChapterSlug(b, target.Chapter, "")
			target.SourceFile = b.ChapterFileNameFor(target.Chapter, target.ChapterID)
		}
	}
	if target.SourceFile == "" || target.SourceFile == "." || strings.EqualFold(target.SourceFile, book.DefaultStoryFile) || strings.EqualFold(target.SourceFile, book.BookDataFileName) {
		target.SourceFile = b.ChapterFileNameFor(target.Chapter, target.ChapterID)
	}
	if _, hasExplicitEnd := r.Form["explicit_end"]; hasExplicitEnd {
		target.ExplicitEnd = r.FormValue("explicit_end") == "true"
	}

	resolveEditedChoiceTarget := func(tgt string) string {
		tgt = strings.TrimSpace(tgt)
		if tgt == "" || strings.EqualFold(tgt, "END") || strings.EqualFold(tgt, "Next") || strings.EqualFold(tgt, "Dalej") {
			return tgt
		}
		if chRef, isCh := book.ParseChapterTarget(tgt); isCh {
			if chRef == "__next__" {
				for _, candCh := range b.Chapters {
					if ok, _ := b.CanLinkChapters(target.Chapter, candCh.ReferenceKey()); ok {
						return "chapter:" + candCh.ReferenceKey()
					}
				}
				return ""
			}
			if targetCh := b.FindChapter(chRef); targetCh != nil {
				if ok, _ := b.CanLinkChapters(target.Chapter, targetCh.ReferenceKey()); ok {
					return "chapter:" + targetCh.ReferenceKey()
				}
			}
			return ""
		}
		if destSc := b.FindScene(tgt); destSc != nil {
			if !strings.EqualFold(strings.TrimSpace(destSc.Chapter), strings.TrimSpace(target.Chapter)) {
				targetCh := b.FindChapter(destSc.Chapter)
				if targetCh != nil && strings.EqualFold(destSc.ID, targetCh.StartSceneID) {
					if ok, _ := b.CanLinkChapters(target.Chapter, targetCh.ReferenceKey()); ok {
						return "chapter:" + targetCh.ReferenceKey()
					}
				}
				return ""
			}
			if destSc.ExplicitID != "" && strings.EqualFold(tgt, destSc.ExplicitID) {
				return destSc.ExplicitID
			}
			return destSc.ReferenceKey()
		}
		return tgt
	}

	if len(target.Choices) >= 2 {
		updatedChoices := make([]book.Choice, 0, len(target.Choices))
		for i, ch := range target.Choices {
			lblKey := fmt.Sprintf("choice_label_%d", i)
			tgtKey := fmt.Sprintf("choice_target_%d", i)
			condKey := fmt.Sprintf("choice_cond_%d", i)
			effKey := fmt.Sprintf("choice_effects_%d", i)
			newLbl := ch.Label
			var inlineTgt string
			var hasInlineArrow bool
			if _, ok := r.Form[lblKey]; ok {
				newLbl, inlineTgt, hasInlineArrow = splitInlineChoiceLabelAndTarget(r.FormValue(lblKey))
			}
			if newLbl == "" {
				newLbl = fmt.Sprintf("Wybór %d", i+1)
			}
			newTgt := ch.Target
			if hasInlineArrow && inlineTgt != "" {
				newTgt = inlineTgt
			} else if _, ok := r.Form[tgtKey]; ok {
				newTgt = book.SanitizePlainText(r.FormValue(tgtKey))
			}
			if newTgt == "" {
				newTgt = ch.TargetSceneID
			}
			newTgt = resolveEditedChoiceTarget(newTgt)
			if newTgt == "" {
				newTgt = ch.Target
			}
			newCond := ch.Condition
			showLocked := ch.ShowWhenLocked
			if _, ok := r.Form[condKey]; ok {
				rawCond := book.StripOuterDoubleBraces(r.FormValue(condKey))
				showLocked = strings.HasPrefix(rawCond, "?")
				if showLocked {
					rawCond = strings.TrimSpace(rawCond[1:])
				}
				newCond = rawCond
			}
			newEff := ch.Effects
			if _, ok := r.Form[effKey]; ok {
				newEff = book.StripOuterDoubleBraces(r.FormValue(effKey))
			}
			if newTgt != "" {
				updatedChoices = append(updatedChoices, book.Choice{
					Label:          newLbl,
					Target:         newTgt,
					Condition:      newCond,
					ShowWhenLocked: showLocked,
					Effects:        newEff,
					IsEnding:       strings.EqualFold(newTgt, "END"),
				})
			}
		}
		target.Choices = updatedChoices
		target.ExplicitNext = ""
		target.ExplicitEnd = false
	} else {
		var prevCond, prevEff string
		var prevLocked bool
		if len(target.Choices) == 1 {
			prevCond = target.Choices[0].Condition
			prevLocked = target.Choices[0].ShowWhenLocked
			prevEff = target.Choices[0].Effects
		}
		if _, ok := r.Form["choice_cond_0"]; ok {
			rawCond := book.StripOuterDoubleBraces(r.FormValue("choice_cond_0"))
			prevLocked = strings.HasPrefix(rawCond, "?")
			if prevLocked {
				rawCond = strings.TrimSpace(rawCond[1:])
			}
			prevCond = rawCond
		}
		if _, ok := r.Form["choice_effects_0"]; ok {
			prevEff = book.StripOuterDoubleBraces(r.FormValue("choice_effects_0"))
		}
		_, inlineTgt0, hasArrow0 := splitInlineChoiceLabelAndTarget(r.FormValue("choice_label_0"))
		_, hasTgt0 := r.Form["choice_target_0"]
		if hasTgt0 || (hasArrow0 && inlineTgt0 != "") {
			tgt := book.SanitizePlainText(r.FormValue("choice_target_0"))
			if hasArrow0 && inlineTgt0 != "" {
				tgt = inlineTgt0
			}
			tgt = resolveEditedChoiceTarget(tgt)
			if strings.EqualFold(tgt, "END") || tgt == "" {
				target.Choices = nil
				target.ExplicitNext = ""
				target.ExplicitEnd = true
			} else {
				target.ExplicitEnd = false
				target.ExplicitNext = tgt
				target.Choices = []book.Choice{
					{
						Label:          "Dalej",
						Target:         tgt,
						Condition:      prevCond,
						ShowWhenLocked: prevLocked,
						Effects:        prevEff,
						IsImplicit:     true,
					},
				}
			}
		} else if len(target.Choices) == 1 {
			target.Choices[0].Condition = prevCond
			target.Choices[0].ShowWhenLocked = prevLocked
			target.Choices[0].Effects = prevEff
		}
	}

	if !strings.EqualFold(oldName, target.Name) || !strings.EqualFold(oldExplicitID, target.ExplicitID) {
		if target.ExplicitID != "" {
			target.ID = target.ExplicitID
		}
		newID := target.ID
		newRef := target.ReferenceKey()
		oldNorm := book.NormalizeLookupKey(oldName)
		matchesOldRef := func(val string) bool {
			v := strings.TrimSpace(val)
			if v == "" {
				return false
			}
			if oldID != "" && strings.EqualFold(v, oldID) {
				return true
			}
			if oldExplicitID != "" && strings.EqualFold(v, oldExplicitID) {
				return true
			}
			if oldName != "" && (strings.EqualFold(v, oldName) || (oldNorm != "" && book.NormalizeLookupKey(v) == oldNorm)) {
				return true
			}
			return false
		}
		if b.Layout != nil {
			var pos book.NodePosition
			foundPos := false
			for _, k := range []string{oldExplicitID, strings.ToLower(oldExplicitID), oldName, oldNorm, oldID, strings.ToLower(oldID)} {
				if k != "" {
					if found, ok := b.Layout[k]; ok {
						pos = found
						foundPos = true
						break
					}
				}
			}
			if foundPos {
				for _, k := range []string{oldExplicitID, strings.ToLower(oldExplicitID), oldName, oldNorm, oldID, strings.ToLower(oldID)} {
					if k != "" {
						delete(b.Layout, k)
					}
				}
				book.StoreSceneNodePosition(b, target, pos)
			}
		}
		if b.WirePins != nil && oldID != "" && newID != "" && !strings.EqualFold(oldID, newID) {
			remappedPins := make(map[string][]book.WirePin, len(b.WirePins))
			for wk, pins := range b.WirePins {
				parts := strings.Split(wk, "|")
				if len(parts) == 3 {
					if matchesOldRef(parts[0]) {
						parts[0] = newID
					}
					if matchesOldRef(parts[2]) {
						parts[2] = newID
					}
					wk = strings.Join(parts, "|")
				}
				remappedPins[wk] = pins
			}
			b.WirePins = remappedPins
		}
		if b.HasEditorView {
			if err := book.SaveBookEditorStateToDisk(b); err != nil {
				b.Warnings = append(b.Warnings, err.Error())
			}
		}
		if matchesOldRef(b.StartSceneID) || matchesOldRef(b.RawStart) {
			b.RawStart = newRef
			b.StartSceneID = newID
		}
		for i := range b.Chapters {
			ch := &b.Chapters[i]
			if matchesOldRef(ch.StartSceneID) || matchesOldRef(ch.ExplicitStart) {
				if ch.ExplicitStart != "" {
					ch.ExplicitStart = newRef
				}
				ch.StartSceneID = newID
			}
		}
		for _, other := range b.Scenes {
			if matchesOldRef(other.ExplicitNext) {
				other.ExplicitNext = newRef
			}
			for i := range other.Choices {
				if !other.Choices[i].IsChapterLink && !strings.EqualFold(other.Choices[i].Target, "Next") && !strings.EqualFold(other.Choices[i].Target, "END") {
					if matchesOldRef(other.Choices[i].Target) || matchesOldRef(other.Choices[i].TargetSceneID) {
						other.Choices[i].Target = newRef
						other.Choices[i].TargetSceneID = newID
					}
				} else if other.Choices[i].IsImplicit && matchesOldRef(other.Choices[i].TargetSceneID) {
					other.Choices[i].TargetSceneID = newID
				}
			}
		}
		if s.Player.Save != nil && matchesOldRef(s.Player.Save.CurrentSceneID) {
			s.Player.Save.CurrentSceneID = newID
		}
	}
	if r.FormValue("is_start_scene") == "true" {
		b.StartSceneID = target.ID
		b.RawStart = target.ReferenceKey()
	}
}

func materializeChoicesForEdit(b *book.Book, sc *book.Scene) []book.Choice {
	var list []book.Choice
	for _, ch := range sc.Choices {
		tgt := ch.Target
		if ch.IsImplicit && !ch.IsChapterLink && (strings.EqualFold(tgt, "Next") || strings.EqualFold(tgt, "Dalej") || tgt == "") {
			if tSc := b.FindScene(ch.TargetSceneID); tSc != nil {
				tgt = tSc.ReferenceKey()
			}
		}
		lbl := ch.Label
		if lbl == "" {
			lbl = "Dalej"
		}
		if tgt != "" {
			list = append(list, book.Choice{
				Label:           lbl,
				Target:          tgt,
				Condition:       ch.Condition,
				Effects:         ch.Effects,
				ShowWhenLocked:  ch.ShowWhenLocked,
				TargetSceneID:   ch.TargetSceneID,
				TargetChapterID: ch.TargetChapterID,
				IsChapterLink:   ch.IsChapterLink,
				IsEnding:        ch.IsEnding || strings.EqualFold(tgt, "END"),
			})
		}
	}
	return list
}

func selectStudioScene(s *state.AppState, r *http.Request) string {
	scID := strings.TrimSpace(r.FormValue("scene"))
	if found := s.Editor.ActiveBook.FindScene(scID); found != nil {
		if s.Editor.SelectedSceneID != found.ID {
			s.Editor.SelectedVoiceSectionKey = "01"
		}
		s.Editor.SelectedSceneID = found.ID
		s.Editor.SelectedChapter = ""
		if s.Editor.RightPanel == state.PanelBook || s.Editor.RightPanel == state.PanelChapter {
			s.Editor.RightPanel = state.PanelNone
		}
		if s.Editor.SceneText.IsOpen() {
			s.Editor.SceneText.SyncSceneID(found.ID)
		}
	}
	return ""
}

func selectPrimaryBranchScene(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	target := b.FindScene(scID)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	for _, sc := range b.Scenes {
		if len(sc.Choices) >= 2 {
			sc.Choices = materializeChoicesForEdit(b, sc)
		}
	}
	if !book.PromoteSceneToPrimaryPath(b, target.ID) {
		r.Form.Set("silent", "true")
		return ""
	}
	return commitScenesMutation(s, "Ustawiono główną ścieżkę (primary): "+target.DisplayName(), "graph", "")
}

func resolveEditorSceneFromRequest(s *state.AppState, r *http.Request, fallbackActive bool) (string, *book.Scene) {
	scID := strings.TrimSpace(r.FormValue("scene"))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	if fallbackActive {
		if scID == "" && s.Editor.SceneText.IsOpen() {
			scID = s.Editor.SceneText.SceneID
		}
		if scID == "" {
			scID = s.Editor.SelectedSceneID
		}
	}
	return scID, s.Editor.ActiveBook.FindScene(scID)
}

func commitTargetSceneChoicesMutation(s *state.AppState, target *book.Scene, msgPrefix string) string {
	b := s.Editor.ActiveBook
	book.FinalizeBook(b)
	book.PushCollidingNodesDown(b, target)
	newID := target.ID
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return err.Error()
	}
	reloadActiveBook(s)
	s.Editor.SelectedSceneID = newID
	s.Editor.SelectedChapter = ""
	if s.Editor.RightPanel == state.PanelBook || s.Editor.RightPanel == state.PanelChapter {
		s.Editor.RightPanel = state.PanelNone
	}
	if s.Editor.SceneText.IsOpen() {
		s.Editor.SceneText.SyncSceneID(newID)
	}
	msg := msgPrefix + target.DisplayName()
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, "graph", "", s.Editor.SelectedSceneID)
	return msg
}

func addChoiceToScene(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	syncSceneTextSettingsOpen(s, r)
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	_, target := resolveEditorSceneFromRequest(s, r, false)
	if target == nil {
		return "Nie znaleziono sceny"
	}

	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	choices := materializeChoicesForEdit(b, target)

	if len(choices) == 1 && (choices[0].Label == "" || strings.EqualFold(choices[0].Label, "Dalej")) {
		choices[0].Label = "Wybór 1"
	}
	if len(choices) < 2 {
		for len(choices) < 2 {
			newLabel := fmt.Sprintf("Wybór %d", len(choices)+1)
			choices = append(choices, book.Choice{
				Label:         newLabel,
				Target:        "END",
				TargetSceneID: "END",
				IsEnding:      true,
			})
		}
	} else {
		newLabel := fmt.Sprintf("Wybór %d", len(choices)+1)
		choices = append(choices, book.Choice{
			Label:         newLabel,
			Target:        "END",
			TargetSceneID: "END",
			IsEnding:      true,
		})
	}
	target.Choices = choices
	target.ExplicitNext = ""
	target.ExplicitEnd = false

	return commitTargetSceneChoicesMutation(s, target, "Dodano nowy wybór do sceny: ")
}

func removeChoiceFromScene(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	_, target := resolveEditorSceneFromRequest(s, r, false)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx")))
	if err != nil {
		return ""
	}

	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	return removeChoiceOnTarget(s, target, choiceIdx)
}

func removeChoiceBySceneAndIndex(s *state.AppState, scID string, choiceIdx int) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	target := b.FindScene(scID)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	return removeChoiceOnTarget(s, target, choiceIdx)
}

func removeChoiceOnTarget(s *state.AppState, target *book.Scene, choiceIdx int) string {
	choices := materializeChoicesForEdit(s.Editor.ActiveBook, target)
	if choiceIdx >= 0 && choiceIdx < len(choices) {
		choices = append(choices[:choiceIdx], choices[choiceIdx+1:]...)
	}
	if len(choices) == 1 {
		if choices[0].IsEnding || strings.EqualFold(choices[0].Target, "END") {
			target.Choices = nil
			target.ExplicitNext = ""
			target.ExplicitEnd = true
		} else {
			choices[0].Label = "Dalej"
			choices[0].IsImplicit = true
			target.ExplicitNext = choices[0].Target
			target.Choices = choices
			target.ExplicitEnd = false
		}
	} else if len(choices) == 0 {
		target.Choices = nil
		target.ExplicitNext = ""
		target.ExplicitEnd = true
	} else {
		target.Choices = choices
		target.ExplicitNext = ""
		target.ExplicitEnd = false
	}

	return commitTargetSceneChoicesMutation(s, target, "Usunięto wybór ze sceny: ")
}

func reorderChoiceOnScene(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	syncSceneTextSettingsOpen(s, r)
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	_, target := resolveEditorSceneFromRequest(s, r, true)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	fromIdx, err1 := strconv.Atoi(strings.TrimSpace(r.FormValue("from_idx")))
	toIdx, err2 := strconv.Atoi(strings.TrimSpace(r.FormValue("to_idx")))
	if err1 != nil || err2 != nil || fromIdx == toIdx {
		return ""
	}

	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	choices := materializeChoicesForEdit(b, target)
	if len(choices) < 2 || fromIdx < 0 || fromIdx >= len(choices) || toIdx < 0 || toIdx >= len(choices) {
		return ""
	}

	moved := choices[fromIdx]
	without := make([]book.Choice, 0, len(choices)-1)
	without = append(without, choices[:fromIdx]...)
	without = append(without, choices[fromIdx+1:]...)

	reordered := make([]book.Choice, 0, len(choices))
	reordered = append(reordered, without[:toIdx]...)
	reordered = append(reordered, moved)
	reordered = append(reordered, without[toIdx:]...)
	target.Choices = reordered
	target.ExplicitNext = ""
	target.ExplicitEnd = false

	return commitTargetSceneChoicesMutation(s, target, "Zmieniono kolejność wyborów: ")
}

func branchSceneFromChoice(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	_, target := resolveEditorSceneFromRequest(s, r, false)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx")))
	if err != nil {
		choiceIdx = -1
	}

	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	choices := materializeChoicesForEdit(b, target)

	newName := book.NextUniqueSceneName(b)
	allocID := b.NextUniqueAutoSceneID(nil)
	b.BindAutoSceneID(newName, allocID)
	newScene := &book.Scene{
		ID:         allocID,
		ExplicitID: allocID,
		Name:       newName,
		Chapter:    target.Chapter,
		Text:       "Nowy fragment opowieści...",
		SourceFile: target.SourceFile,
	}

	if choiceIdx >= 0 && choiceIdx < len(choices) {
		choices[choiceIdx].Target = newScene.Name
		choices[choiceIdx].IsEnding = false
		if strings.TrimSpace(choices[choiceIdx].Label) == "" {
			choices[choiceIdx].Label = "Dalej"
		}
	} else {
		choices = append(choices, book.Choice{
			Label:  fmt.Sprintf("Wybór %d", len(choices)+1),
			Target: newScene.Name,
		})
	}
	target.Choices = choices
	target.ExplicitEnd = false
	if len(choices) == 1 {
		target.ExplicitNext = newScene.Name
	}

	parentIdx := len(b.Scenes) - 1
	for i, sc := range b.Scenes {
		if sc.ID == target.ID {
			parentIdx = i
			break
		}
	}
	insertPos := parentIdx + 1
	b.Scenes = append(b.Scenes[:insertPos], append([]*book.Scene{newScene}, b.Scenes[insertPos:]...)...)

	book.FinalizeBook(b)
	book.StoreSceneNodePosition(b, newScene, book.NodePosition{
		Row:   newScene.Row,
		Col:   newScene.Col,
		X:     newScene.X,
		Y:     newScene.Y,
		HasXY: true,
	})
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return err.Error()
	}
	reloadActiveBook(s)
	s.Editor.SelectedChapter = ""
	if s.Editor.RightPanel == state.PanelBook || s.Editor.RightPanel == state.PanelChapter {
		s.Editor.RightPanel = state.PanelNone
	}
	if s.Editor.SceneText.IsOpen() {
		s.Editor.SelectedSceneID = target.ID
		s.Editor.SceneText.SyncSceneID(target.ID)
	} else if found := s.Editor.ActiveBook.FindScene(newScene.Name); found != nil {
		s.Editor.SelectedSceneID = found.ID
	} else {
		s.Editor.SelectedSceneID = newScene.ID
	}
	msg := "Utworzono scenę docelową: " + newScene.Name
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, "graph", "", s.Editor.SelectedSceneID)
	return msg
}

func autoSaveSceneAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return ""
	}
	if msg := saveSceneFromFormWithMode(s, r, true); msg != "" && !strings.HasPrefix(msg, "Zapisano zmiany w pliku ") && msg != "Nie znaleziono sceny" {
		return msg
	}
	return ""
}

func saveSceneAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	if msg := saveSceneFromForm(s, r); msg != "" && !strings.HasPrefix(msg, "Zapisano zmiany w pliku ") && msg != "Nie znaleziono sceny" {
		return msg
	}
	return ""
}

func saveSceneFromForm(s *state.AppState, r *http.Request) string {
	return saveSceneFromFormWithMode(s, r, false)
}

func saveSceneFromFormWithMode(s *state.AppState, r *http.Request, isAutoSave bool) string {
	if isHistoryPreview(s) {
		if isAutoSave {
			return ""
		}
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene_id"))
	var target *book.Scene
	if scID != "" {
		target = b.FindScene(scID)
	}
	if target == nil && s.Editor.SceneText.IsOpen() && (scID == "" || strings.EqualFold(scID, s.Editor.SceneText.SceneID) || (s.Editor.SceneText.InitialID != "" && strings.EqualFold(scID, s.Editor.SceneText.InitialID))) {
		target = b.FindScene(s.Editor.SceneText.SceneID)
	}
	if target == nil && s.Editor.SceneText.IsOpen() {
		target = b.FindScene(s.Editor.SceneText.SceneID)
	}
	if target == nil && s.Editor.SelectedSceneID != "" {
		target = b.FindScene(s.Editor.SelectedSceneID)
	}
	if target == nil {
		return "Nie znaleziono sceny"
	}
	prevTargetID := target.ID

	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)

	book.FinalizeBook(b)
	newID := target.ID
	targetLabel := target.DisplayName()
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return err.Error()
	}
	reloadActiveBook(s)
	if newID != "" {
		if prevTargetID != "" && !strings.EqualFold(prevTargetID, newID) {
			s.Editor.LastRenamedFromSceneID = prevTargetID
		}
		s.Editor.SelectedSceneID = newID
		if s.Editor.SceneText.IsOpen() && (strings.EqualFold(s.Editor.SceneText.SceneID, prevTargetID) || strings.EqualFold(s.Editor.SceneText.SceneID, scID) || (s.Editor.SceneText.InitialID != "" && strings.EqualFold(s.Editor.SceneText.InitialID, scID))) {
			s.Editor.SceneText.SceneID = newID
		}
	}
	coalesceKey := ""
	if r != nil && r.FormValue("action") == "UndoEditor" {
		coalesceKey = "major"
	} else if newID != "" {
		coalesceKey = "autosave:" + newID
	}
	s.Editor.History.RecordStep(s.Editor.ActiveBook, "Edycja sceny: "+targetLabel, "scene", coalesceKey, s.Editor.SelectedSceneID)
	return "Zapisano zmiany w pliku " + target.SourceFile
}

func finishInsertedScene(s *state.AppState, inserted *book.Scene, err error) string {
	if err != nil {
		return err.Error()
	}
	if inserted == nil {
		return ""
	}
	reloadActiveBook(s)
	if found := s.Editor.ActiveBook.FindScene(inserted.Name); found != nil {
		s.Editor.SelectedSceneID = found.ID
	} else {
		s.Editor.SelectedSceneID = inserted.ID
	}
	s.Editor.SelectedChapter = ""
	if s.Editor.RightPanel == state.PanelBook || s.Editor.RightPanel == state.PanelChapter {
		s.Editor.RightPanel = state.PanelNone
	}
	if s.Editor.SceneText.IsOpen() {
		s.Editor.SceneText.SyncSceneID(s.Editor.SelectedSceneID)
	}
	toast := "Wstawiono nową scenę: " + inserted.Name
	s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, "graph", "", s.Editor.SelectedSceneID)
	return toast
}

func insertScene(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	parentID := strings.TrimSpace(r.FormValue("parent"))
	childID := strings.TrimSpace(r.FormValue("child"))
	if parentID == "" && len(b.Scenes) > 0 {
		parentID = b.Scenes[len(b.Scenes)-1].ID
	}
	newName := book.NextUniqueSceneName(b)
	inserted, err := book.InsertSceneBetween(b, parentID, childID, newName)
	return finishInsertedScene(s, inserted, err)
}

func insertSceneBefore(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	targetID := strings.TrimSpace(r.FormValue("target"))
	if targetID == "" {
		targetID = strings.TrimSpace(r.FormValue("before"))
	}
	if targetID == "" {
		targetID = strings.TrimSpace(r.FormValue("scene"))
	}
	newName := book.NextUniqueSceneName(b)
	inserted, err := book.InsertSceneBefore(b, targetID, newName)
	return finishInsertedScene(s, inserted, err)
}

func createSceneAt(s *state.AppState, r *http.Request) string {
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
	toast := ""
	b := s.Editor.ActiveBook
	if b != nil && !b.IsPacked {
		s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
		x, errX := strconv.Atoi(strings.TrimSpace(r.FormValue("x")))
		y, errY := strconv.Atoi(strings.TrimSpace(r.FormValue("y")))
		hasCoords := errX == nil && errY == nil
		targetCh := strings.TrimSpace(r.FormValue("chapter"))
		newName := book.NextUniqueSceneName(b)
		if created, err := book.CreateStandaloneSceneInChapter(b, newName, targetCh, x, y, hasCoords); err == nil && created != nil {
			reloadActiveBook(s)
			if found := s.Editor.ActiveBook.FindScene(created.Name); found != nil {
				s.Editor.SelectedSceneID = found.ID
			} else {
				s.Editor.SelectedSceneID = created.ID
			}
			s.Editor.SelectedChapter = ""
			if s.Editor.RightPanel == state.PanelBook || s.Editor.RightPanel == state.PanelChapter {
				s.Editor.RightPanel = state.PanelNone
			}
			if s.Editor.SceneText.IsOpen() {
				s.Editor.SceneText.SyncSceneID(s.Editor.SelectedSceneID)
			}
			toast = "Dodano nową scenę: " + created.Name
			s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, "graph", "", s.Editor.SelectedSceneID)
		} else if err != nil {
			toast = err.Error()
		}
	}
	return toast
}

func deleteSceneFromForm(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	targetID := strings.TrimSpace(r.FormValue("scene"))
	if targetID == "" {
		targetID = s.Editor.SelectedSceneID
	}
	return deleteSceneByID(s, targetID)
}

func deleteScenesFromForm(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	rawIDs := strings.TrimSpace(r.FormValue("scenes"))
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue("scene_ids"))
	}
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue("scene"))
	}
	return deleteScenesByIDs(s, rawIDs)
}

func deleteSceneByID(s *state.AppState, sceneID string) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || len(b.Scenes) <= 1 {
		return "Książka musi zawierać przynajmniej jedną scenę"
	}
	delIdx := -1
	var delName string
	var delLabel string
	for i, sc := range b.Scenes {
		if sc.ID == sceneID {
			delIdx = i
			delName = sc.Name
			delLabel = sc.DisplayName()
			break
		}
	}
	if delIdx < 0 {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if delSc := b.Scenes[delIdx]; delSc != nil {
		b.UnbindAutoSceneName(delSc.Name)
		b.RetireSceneID(delSc.ID, delSc.ExplicitID)
	}
	b.Scenes = append(b.Scenes[:delIdx], b.Scenes[delIdx+1:]...)
	delete(b.Layout, sceneID)

	if strings.EqualFold(b.StartSceneID, sceneID) || (delName != "" && strings.EqualFold(b.RawStart, delName)) {
		b.RawStart = b.Scenes[0].ReferenceKey()
		b.StartSceneID = b.Scenes[0].ID
	}
	for i := range b.Chapters {
		ch := &b.Chapters[i]
		if strings.EqualFold(ch.StartSceneID, sceneID) ||
			strings.EqualFold(ch.ExplicitStart, sceneID) ||
			(delName != "" && strings.EqualFold(ch.ExplicitStart, delName)) {
			ch.ExplicitStart = ""
		}
	}

	for _, sc := range b.Scenes {
		var kept []book.Choice
		for _, ch := range sc.Choices {
			if !ch.IsImplicit && (ch.TargetSceneID == sceneID || (delName != "" && strings.EqualFold(ch.Target, delName))) {
				continue
			}
			kept = append(kept, ch)
		}
		sc.Choices = kept
	}

	book.FinalizeBook(b)
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return err.Error()
	}
	reloadActiveBook(s)
	if len(s.Editor.ActiveBook.Scenes) > 0 {
		s.Editor.SelectedSceneID = s.Editor.ActiveBook.Scenes[0].ID
	}
	msg := "Usunięto scenę: " + delLabel
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, "graph", "", s.Editor.SelectedSceneID)
	return msg
}

func deleteScenesByIDs(s *state.AppState, rawIDs string) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || len(b.Scenes) == 0 {
		return ""
	}
	toDeleteIDs := make(map[string]bool)
	toDeleteExplicitIDs := make(map[string]bool)
	toDeleteNames := make(map[string]bool)
	var deletedScenes []*book.Scene

	for _, part := range strings.Split(rawIDs, ",") {
		idPart := strings.TrimSpace(part)
		if idPart == "" {
			continue
		}
		if sc := b.FindScene(idPart); sc != nil && !toDeleteIDs[sc.ID] {
			toDeleteIDs[sc.ID] = true
			if sc.ExplicitID != "" {
				toDeleteExplicitIDs[strings.ToLower(strings.TrimSpace(sc.ExplicitID))] = true
			}
			if sc.Name != "" {
				toDeleteNames[book.NormalizeLookupKey(sc.Name)] = true
			}
			deletedScenes = append(deletedScenes, sc)
		}
	}
	if len(deletedScenes) == 0 {
		return ""
	}
	if len(deletedScenes) == 1 {
		return deleteSceneByID(s, deletedScenes[0].ID)
	}

	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	firstChapter := b.Scenes[0].Chapter
	firstChapterID := b.Scenes[0].ChapterID
	firstSourceFile := b.Scenes[0].SourceFile
	if firstSourceFile == "" || strings.EqualFold(firstSourceFile, book.DefaultStoryFile) || strings.EqualFold(firstSourceFile, book.BookDataFileName) {
		firstSourceFile = b.ChapterFileNameFor(firstChapter, firstChapterID)
	}

	var remaining []*book.Scene
	for _, sc := range b.Scenes {
		if !toDeleteIDs[sc.ID] {
			remaining = append(remaining, sc)
		}
	}

	for _, sc := range deletedScenes {
		b.UnbindAutoSceneName(sc.Name)
		b.RetireSceneID(sc.ID, sc.ExplicitID)
		delete(b.Layout, sc.ID)
		if sc.ExplicitID != "" {
			delete(b.Layout, sc.ExplicitID)
			delete(b.Layout, strings.ToLower(sc.ExplicitID))
		}
		if sc.Name != "" {
			delete(b.Layout, sc.Name)
			delete(b.Layout, book.NormalizeLookupKey(sc.Name))
		}
	}

	if len(remaining) == 0 {
		fallback := &book.Scene{
			Name:        "Nowa scena 1",
			Chapter:     firstChapter,
			ChapterID:   firstChapterID,
			Text:        "Nowy fragment opowieści...",
			SourceFile:  firstSourceFile,
			ExplicitEnd: true,
		}
		remaining = []*book.Scene{fallback}
	}

	b.Scenes = remaining

	for _, sc := range b.Scenes {
		if sc.ExplicitNext != "" {
			nextTrim := strings.TrimSpace(sc.ExplicitNext)
			nextLower := strings.ToLower(nextTrim)
			nextNorm := book.NormalizeLookupKey(nextTrim)
			if toDeleteIDs[nextTrim] || (nextLower != "" && toDeleteExplicitIDs[nextLower]) || (nextNorm != "" && toDeleteNames[nextNorm]) {
				sc.ExplicitNext = ""
			}
		}
		var kept []book.Choice
		for _, ch := range sc.Choices {
			tgtTrim := strings.TrimSpace(ch.Target)
			tgtLower := strings.ToLower(tgtTrim)
			tgtNorm := book.NormalizeLookupKey(tgtTrim)
			if !ch.IsImplicit && (toDeleteIDs[ch.TargetSceneID] || toDeleteIDs[tgtTrim] || (tgtLower != "" && toDeleteExplicitIDs[tgtLower]) || (tgtNorm != "" && toDeleteNames[tgtNorm])) {
				continue
			}
			kept = append(kept, ch)
		}
		sc.Choices = kept
	}

	if rawStart := strings.TrimSpace(b.RawStart); rawStart != "" {
		rawLower := strings.ToLower(rawStart)
		rawNorm := book.NormalizeLookupKey(rawStart)
		if toDeleteIDs[b.StartSceneID] || toDeleteIDs[rawStart] || toDeleteExplicitIDs[rawLower] || (rawNorm != "" && toDeleteNames[rawNorm]) {
			b.RawStart = b.Scenes[0].ReferenceKey()
			b.StartSceneID = b.Scenes[0].ID
		}
	}

	book.FinalizeBook(b)
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return err.Error()
	}
	reloadActiveBook(s)
	if len(s.Editor.ActiveBook.Scenes) > 0 {
		s.Editor.SelectedSceneID = s.Editor.ActiveBook.Scenes[0].ID
	}
	msg := fmt.Sprintf("Usunięto zaznaczone sceny (%d)", len(deletedScenes))
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, "graph", "", s.Editor.SelectedSceneID)
	return msg
}

func prepareScreenplaySceneEdit(s *state.AppState, r *http.Request) *book.Scene {
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked {
		return nil
	}
	_, target := resolveEditorSceneFromRequest(s, r, true)
	if target == nil {
		return nil
	}
	s.Editor.History.EnsureBaseline(s.Editor.ActiveBook, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	return target
}

func applySluglineTokenAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	target := prepareScreenplaySceneEdit(s, r)
	if target == nil {
		return ""
	}
	b := s.Editor.ActiveBook

	info := book.ParseSlugline(target.Slugline)
	prefix := info.IntExt
	if prefix == "" {
		if defInfo := book.ParseSlugline(book.DefaultSluglineForStandard(b.NormalizedStandard())); defInfo.IntExt != "" {
			prefix = defInfo.IntExt
		} else {
			prefix = "WN."
		}
	}
	loc := info.Location
	if loc == "" {
		if b.NormalizedStandard() == "pl" {
			loc = "LOKACJA"
		} else {
			loc = "LOCATION"
		}
	}
	tod := info.TimeOfDay
	if tod == "" {
		if b.NormalizedStandard() == "pl" {
			tod = "DZIEŃ"
		} else {
			tod = "DAY"
		}
	}

	if newPrefix := strings.ToUpper(strings.TrimSpace(r.FormValue("prefix"))); newPrefix != "" {
		prefix = newPrefix
	}
	if newTime := strings.ToUpper(strings.TrimSpace(r.FormValue("time"))); newTime != "" {
		tod = newTime
	}
	target.Slugline = fmt.Sprintf("%s %s - %s", prefix, loc, tod)
	return commitScenesMutation(s, "Zaktualizowano nagłówek sceny: "+target.Slugline, "scene", "slugline:"+target.ID)
}

func insertScreenplayBlockAction(s *state.AppState, r *http.Request) string {
	target := prepareScreenplaySceneEdit(s, r)
	if target == nil {
		return ""
	}
	b := s.Editor.ActiveBook

	isPL := b.NormalizedStandard() == "pl"
	kind := strings.ToLower(strings.TrimSpace(r.FormValue("kind")))
	charName := strings.ToUpper(strings.TrimSpace(book.SanitizePlainText(r.FormValue("character"))))

	snippet := ""
	switch {
	case charName != "":
		snippet = "\n\n" + charName + "\nKwestia dialogowa..."
	case kind == "dialogue":
		snippet = "\n\nPOSTAĆ\nKwestia dialogowa..."
	case kind == "parenthetical":
		snippet = "\n(cicho)\n"
	case kind == "action":
		snippet = "\n\nOpis akcji..."
	case kind == "dialogue_off":
		if isPL {
			snippet = "\n\nPOSTAĆ (OFF)\nKwestia spoza kadru..."
		} else {
			snippet = "\n\nPOSTAĆ (O.S.)\nKwestia spoza kadru..."
		}
	case kind == "dialogue_vo":
		snippet = "\n\nPOSTAĆ (V.O.)\nKwestia narratora..."
	case kind == "transition_cut":
		if isPL {
			snippet = "\n\n> CIĘCIE DO:\n"
		} else {
			snippet = "\n\n> CUT TO:\n"
		}
	case kind == "transition_fade":
		if isPL {
			snippet = "\n\n> ŚCIEMNIENIE.\n"
		} else {
			snippet = "\n\n> FADE OUT.\n"
		}
	case kind == "subslug":
		if isPL {
			snippet = "\n\nWN. LOKACJA - CIĄG DALSZY\n"
		} else {
			snippet = "\n\nINT. LOCATION - CONTINUOUS\n"
		}
	}
	if snippet != "" {
		cur := strings.TrimRight(target.Text, " \t\n")
		if cur == "" {
			target.Text = strings.TrimLeft(snippet, "\n")
		} else {
			target.Text = cur + snippet
		}
		target.HTML = ""
	}
	return commitScenesMutation(s, "Wstawiono element scenariusza w scenie: "+target.DisplayName(), "scene", "text:"+target.ID)
}

func resolveVoiceActionTargetScene(s *state.AppState, r *http.Request) *book.Scene {
	_, target := resolveEditorSceneFromRequest(s, r, true)
	return target
}

func toggleSceneVoiceRecorderAction(s *state.AppState, r *http.Request) string {
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	if target := resolveVoiceActionTargetScene(s, r); target != nil && !isHistoryPreview(s) && !b.IsPacked {
		if hasSceneTextModalChanges(s, target, r) {
			s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
			applySceneFormFields(s, target, r)
			book.FinalizeBook(b)
			if err := book.SaveBookScenesToDisk(b); err != nil {
				return err.Error()
			}
			reloadActiveBook(s)
			s.Editor.History.RecordStep(s.Editor.ActiveBook, "Zapisano zmiany w scenie", "scene", target.ID, s.Editor.SelectedSceneID)
		}
	}
	if openParam := strings.TrimSpace(r.FormValue("open")); openParam == "true" {
		s.Modals.VoiceRecorder = true
	} else if openParam == "false" {
		s.Modals.VoiceRecorder = false
	} else {
		s.Modals.VoiceRecorder = !s.Modals.VoiceRecorder
	}
	return ""
}

func selectSceneVoiceSectionAction(s *state.AppState, r *http.Request) string {
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	target := resolveVoiceActionTargetScene(s, r)
	if target != nil && !isHistoryPreview(s) && !b.IsPacked {
		if hasSceneTextModalChanges(s, target, r) {
			s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
			applySceneFormFields(s, target, r)
			book.FinalizeBook(b)
			if err := book.SaveBookScenesToDisk(b); err != nil {
				return err.Error()
			}
			reloadActiveBook(s)
			target = resolveVoiceActionTargetScene(s, r)
			if target != nil {
				s.Editor.History.RecordStep(s.Editor.ActiveBook, "Zapisano zmiany w scenie", "scene", target.ID, s.Editor.SelectedSceneID)
			}
		}
	}
	secKey := strings.TrimSpace(r.FormValue("voice_section"))
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

type voiceCaptureCtxKey struct{}

type voiceCaptureResult struct {
	pcmLE      []byte
	sampleRate int
	err        error
}

func startSceneVoiceCaptureAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked || !s.Editor.ActiveBook.Narrated {
		return "Brak aktywnej książki z włączoną narracją"
	}
	if res, ok := r.Context().Value(voiceCaptureCtxKey{}).(voiceCaptureResult); ok && res.err != nil {
		return res.err.Error()
	}
	return ""
}

func pollSceneVoiceCaptureAction(_ *state.AppState, _ *http.Request) string {
	return ""
}

func commitSceneVoiceFile(s *state.AppState, target *book.Scene, secKey, relPath string, err error, historyPrefix string) string {
	if err != nil {
		return err.Error()
	}
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
	s.Editor.History.RecordStep(s.Editor.ActiveBook, historyPrefix+relPath, "scene", target.ID, s.Editor.SelectedSceneID)
	return ""
}

func stopSceneVoiceCaptureAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	res, ok := r.Context().Value(voiceCaptureCtxKey{}).(voiceCaptureResult)
	if !ok {
		return ""
	}
	if res.err != nil {
		return res.err.Error()
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked || !b.Narrated {
		return "Brak aktywnej książki z włączoną narracją"
	}
	target := resolveVoiceActionTargetScene(s, r)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if hasSceneTextModalChanges(s, target, r) {
		applySceneFormFields(s, target, r)
	}
	secKey := activeVoiceSectionKeyForRequest(s, target, r)
	relPath, err := book.SaveSceneVoiceSectionPCM(b, target, secKey, res.pcmLE, res.sampleRate, 0, 1)
	return commitSceneVoiceFile(s, target, secKey, relPath, err, "Zapisano nagranie audio: ")
}

func saveSceneVoiceRecordingAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return "Brak aktywnej książki"
	}
	if b.IsPacked {
		return "Nie można edytować spakowanego archiwum .playbook bezpośrednio"
	}
	if !b.Narrated {
		return "Książka nie posiada włączonej narracji (narrated: false)"
	}
	target := resolveVoiceActionTargetScene(s, r)
	if target == nil {
		return "Nie znaleziono sceny"
	}

	rawB64 := strings.TrimSpace(r.FormValue("pcm_b64"))
	if rawB64 == "" {
		return "Brak danych nagrania audio"
	}
	pcmLE, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil || len(pcmLE) < 2 {
		return "Nieprawidłowy strumień nagrania audio"
	}
	sampleRate, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("sample_rate")))
	if sampleRate < 8000 || sampleRate > 65535 {
		sampleRate = 24000
	}
	startRatio, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("start_ratio")), 64)
	endRatio, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("end_ratio")), 64)

	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if hasSceneTextModalChanges(s, target, r) {
		applySceneFormFields(s, target, r)
	}
	secKey := activeVoiceSectionKeyForRequest(s, target, r)
	relPath, err := book.SaveSceneVoiceSectionPCM(b, target, secKey, pcmLE, sampleRate, startRatio, endRatio)
	return commitSceneVoiceFile(s, target, secKey, relPath, err, "Zapisano nagranie audio: ")
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
	startRatio, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("start_ratio")), 64)
	endRatio, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("end_ratio")), 64)
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

	isDestructive := strings.EqualFold(strings.TrimSpace(r.FormValue("destructive")), "true")
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
		s.Editor.History.RecordStep(s.Editor.ActiveBook, "Przycięto nagranie audio: "+target.DisplayName(), "scene", "voicetrim:"+target.ID+":"+secKey, s.Editor.SelectedSceneID)
		return ""
	}

	var relPath string
	var err error
	if rawB64 := strings.TrimSpace(r.FormValue("pcm_b64")); rawB64 != "" {
		pcmLE, decErr := base64.StdEncoding.DecodeString(rawB64)
		if decErr != nil {
			return "Nieprawidłowe dane audio do przycięcia"
		}
		sampleRate, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("sample_rate")))
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
		stemWithSec := book.SceneVoiceBaseStem(target) + "__" + secKey
		relPath, err = book.TrimSceneVoiceFile(b, stemWithSec, curVoice, startRatio, endRatio)
	} else {
		curVoice := target.ExplicitVoice
		if curVoice == "" {
			curVoice = target.AudioFile
		}
		relPath, err = book.TrimSceneVoiceFile(b, target.ReferenceKey(), curVoice, startRatio, endRatio)
	}
	return commitSceneVoiceFile(s, target, secKey, relPath, err, "Przycięto trwale plik audio: ")
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
	s.Editor.History.RecordStep(s.Editor.ActiveBook, "Usunięto nagranie audio sceny", "scene", target.ID, s.Editor.SelectedSceneID)
	return ""
}
