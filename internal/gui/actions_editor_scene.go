//go:build !(js && wasm)

package gui

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"mojra/internal/book"
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

func (s *Server) activeVoiceSectionKeyForRequest(sc *book.Scene, r *http.Request) string {
	var cand string
	if r != nil {
		cand = strings.TrimSpace(r.FormValue("voice_section"))
		if cand == "" {
			cand = strings.TrimSpace(r.FormValue("isolated_voice_section"))
		}
	}
	if cand == "" {
		cand = strings.TrimSpace(s.SelectedVoiceSectionKey)
	}
	if sc != nil {
		secs := book.ExtractSceneVoiceSections(sc, s.ActiveBook)
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

func (s *Server) hasSceneTextModalChanges(curSc *book.Scene, r *http.Request) bool {
	if curSc == nil {
		return false
	}
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
	if s.ActiveBook != nil && s.ActiveBook.Narrated {
		if _, hasVoice := r.Form["explicit_voice"]; hasVoice {
			newVoice := book.SanitizeRelativeMediaPath(r.FormValue("explicit_voice"))
			if book.HasConditionalVoiceDirectives(curSc.Text) {
				secKey := s.activeVoiceSectionKeyForRequest(curSc, r)
				book.ExtractSceneVoiceSections(curSc, s.ActiveBook)
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

func isPlaceholderSceneName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(lower, "nowa scena") || strings.HasPrefix(lower, "new scene")
}

func isSlugDerivedFromTitle(slug, title string) bool {
	if slug == "" || title == "" {
		return false
	}
	base := book.SanitizeChapterID(title)
	if base == "" {
		base = book.SanitizeSceneID(title)
	}
	if base == "" {
		return false
	}
	return strings.EqualFold(slug, base) || strings.HasPrefix(strings.ToLower(slug), strings.ToLower(base)+"_")
}

func (s *Server) applySceneFormFields(target *book.Scene, r *http.Request) {
	if target == nil || s.ActiveBook == nil {
		return
	}
	if formSceneID := strings.TrimSpace(r.FormValue("scene_id")); formSceneID != "" && !strings.EqualFold(formSceneID, target.ID) && !strings.EqualFold(s.SceneTextModalID, target.ID) {
		return
	}
	oldID := target.ID
	oldName := target.Name
	oldExplicitID := target.ExplicitID
	if _, hasID := r.Form["explicit_id"]; hasID {
		rawExp := strings.TrimSpace(r.FormValue("explicit_id"))
		if rawExp != "" {
			cleanExp := book.SanitizeSceneID(rawExp)
			if cleanExp != "" && !strings.EqualFold(cleanExp, oldExplicitID) && !strings.EqualFold(cleanExp, oldID) {
				cleanExp = book.NextUniqueSceneSlug(s.ActiveBook, cleanExp, target)
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
		} else if s.ActiveBook != nil {
			target.ExplicitID = s.ActiveBook.NextUniqueAutoSceneID(nil)
		}
	}
	if s.ActiveBook != nil {
		if oldName != "" && !strings.EqualFold(target.Name, oldName) {
			s.ActiveBook.UnbindAutoSceneName(oldName)
		}
		if target.ExplicitID != "" && !strings.EqualFold(target.ExplicitID, oldID) {
			s.ActiveBook.UnbindAutoSceneName(target.Name)
			if book.ParseAutoSceneSeq(oldID) > 0 {
				s.ActiveBook.RetireSceneID(oldID)
			}
			if book.ParseAutoSceneSeq(oldExplicitID) > 0 {
				s.ActiveBook.RetireSceneID(oldExplicitID)
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

	if !s.ActiveBook.Narrated {
		target.ExplicitVoice = ""
		target.VoiceTrimStart = 0
		target.VoiceTrimEnd = 0
		target.SectionVoices = nil
		target.AudioFile = ""
		target.AudioURL = ""
	} else if _, hasVoice := r.Form["explicit_voice"]; hasVoice {
		newVoice := book.SanitizeRelativeMediaPath(r.FormValue("explicit_voice"))
		if book.HasConditionalVoiceDirectives(target.Text) {
			secKey := s.activeVoiceSectionKeyForRequest(target, r)
			book.ExtractSceneVoiceSections(target, s.ActiveBook)
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
	action := strings.TrimSpace(r.FormValue("action"))
	if _, hasChapter := r.Form["chapter"]; hasChapter && action != "CreateSceneAt" && action != "CreateChapterGroup" && action != "SelectStudioChapter" && action != "PromptDeleteChapter" && action != "DeleteChapter" {
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
	if chapterChanged && s.ActiveBook != nil {
		if chMeta := s.ActiveBook.FindChapter(target.Chapter); chMeta != nil {
			target.ChapterID = chMeta.ExplicitID
			if target.ChapterID == "" {
				target.ChapterID = chMeta.ID
			}
			if chMeta.SourceFile != "" {
				target.SourceFile = chMeta.SourceFile
			}
		} else {
			target.ChapterID = book.NextUniqueChapterSlug(s.ActiveBook, target.Chapter, "")
			target.SourceFile = s.ActiveBook.ChapterFileNameFor(target.Chapter, target.ChapterID)
		}
	}
	if target.SourceFile == "" || target.SourceFile == "." || strings.EqualFold(target.SourceFile, book.DefaultStoryFile) || strings.EqualFold(target.SourceFile, book.BookDataFileName) {
		target.SourceFile = s.ActiveBook.ChapterFileNameFor(target.Chapter, target.ChapterID)
	}
	if _, hasExplicitEnd := r.Form["explicit_end"]; hasExplicitEnd {
		target.ExplicitEnd = r.FormValue("explicit_end") == "true"
	}

	if countStr := strings.TrimSpace(r.FormValue("choice_count")); countStr != "" {
		if count, err := strconv.Atoi(countStr); err == nil && count >= 2 {
			var parsedChoices []book.Choice
			for i := 0; i < count; i++ {
				lbl, inlineTgt, hasArrow := splitInlineChoiceLabelAndTarget(r.FormValue(fmt.Sprintf("choice_label_%d", i)))
				tgt := book.SanitizePlainText(r.FormValue(fmt.Sprintf("choice_target_%d", i)))
				if hasArrow && inlineTgt != "" {
					tgt = inlineTgt
				}
				if tgt == "" && i < len(target.Choices) {
					tgt = target.Choices[i].Target
				}
				if lbl == "" && tgt != "" {
					lbl = fmt.Sprintf("Wybór %d", i+1)
				}
				var cond, eff string
				var showLocked bool
				if i < len(target.Choices) {
					cond = target.Choices[i].Condition
					eff = target.Choices[i].Effects
					showLocked = target.Choices[i].ShowWhenLocked
				}
				condKey := fmt.Sprintf("choice_cond_%d", i)
				if _, hasCond := r.Form[condKey]; hasCond {
					rawCond := book.CleanConditionExpr(r.FormValue(condKey))
					showLocked = strings.HasPrefix(rawCond, "?")
					if showLocked {
						rawCond = strings.TrimSpace(rawCond[1:])
					}
					cond = rawCond
				}
				effKey := fmt.Sprintf("choice_effects_%d", i)
				if _, hasEff := r.Form[effKey]; hasEff {
					eff = book.CleanEffectsExpr(r.FormValue(effKey))
				}
				if lbl != "" && tgt != "" {
					parsedChoices = append(parsedChoices, book.Choice{
						Label:          lbl,
						Target:         tgt,
						Condition:      cond,
						Effects:        eff,
						ShowWhenLocked: showLocked,
						IsEnding:       strings.EqualFold(tgt, "END"),
					})
				}
			}
			target.Choices = parsedChoices
			if len(parsedChoices) > 0 {
				target.ExplicitEnd = false
				target.ExplicitNext = ""
			}
		}
	} else if _, hasLines := r.Form["choices_lines"]; hasLines {
		var parsedChoices []book.Choice
		rawChoices := strings.ReplaceAll(r.FormValue("choices_lines"), "\r\n", "\n")
		for _, line := range strings.Split(rawChoices, "\n") {
			trimmed := strings.TrimSpace(line)
			if idx := strings.Index(trimmed, " -> "); idx > 0 {
				lbl, cond, eff, showLocked := book.ParseChoiceRuleTokens(trimmed[:idx])
				lbl = book.SanitizePlainText(lbl)
				tgt := book.SanitizePlainText(trimmed[idx+4:])
				if lbl != "" && tgt != "" {
					parsedChoices = append(parsedChoices, book.Choice{
						Label:          lbl,
						Target:         tgt,
						Condition:      cond,
						Effects:        eff,
						ShowWhenLocked: showLocked,
						IsEnding:       strings.EqualFold(tgt, "END"),
					})
				}
			}
		}
		target.Choices = parsedChoices
	} else if len(target.Choices) >= 2 {
		for i := range target.Choices {
			lblKey := fmt.Sprintf("choice_label_%d", i)
			var inlineTgt string
			var hasArrow bool
			if _, hasLbl := r.Form[lblKey]; hasLbl {
				var lbl string
				lbl, inlineTgt, hasArrow = splitInlineChoiceLabelAndTarget(r.FormValue(lblKey))
				if lbl == "" {
					lbl = fmt.Sprintf("Wybór %d", i+1)
				}
				target.Choices[i].Label = lbl
			}
			tgtKey := fmt.Sprintf("choice_target_%d", i)
			tgt := ""
			if hasArrow && inlineTgt != "" {
				tgt = inlineTgt
			} else if _, hasTgt := r.Form[tgtKey]; hasTgt {
				tgt = book.SanitizePlainText(r.FormValue(tgtKey))
			}
			if tgt != "" {
				target.Choices[i].Target = tgt
				target.Choices[i].IsEnding = strings.EqualFold(tgt, "END")
			}
			condKey := fmt.Sprintf("choice_cond_%d", i)
			if _, hasCond := r.Form[condKey]; hasCond {
				rawCond := book.CleanConditionExpr(r.FormValue(condKey))
				showLocked := strings.HasPrefix(rawCond, "?")
				if showLocked {
					rawCond = strings.TrimSpace(rawCond[1:])
				}
				target.Choices[i].Condition = rawCond
				target.Choices[i].ShowWhenLocked = showLocked
			}
			effKey := fmt.Sprintf("choice_effects_%d", i)
			if _, hasEff := r.Form[effKey]; hasEff {
				target.Choices[i].Effects = book.CleanEffectsExpr(r.FormValue(effKey))
			}
		}
	} else if _, hasTgt0 := r.Form["choice_target_0"]; hasTgt0 {
		prevCond := ""
		prevLocked := false
		prevEff := ""
		if len(target.Choices) > 0 {
			prevCond = target.Choices[0].Condition
			prevLocked = target.Choices[0].ShowWhenLocked
			prevEff = target.Choices[0].Effects
		}
		if _, hasCond := r.Form["choice_cond_0"]; hasCond {
			condRaw := book.CleanConditionExpr(r.FormValue("choice_cond_0"))
			if strings.HasPrefix(condRaw, "?") {
				prevLocked = true
				prevCond = strings.TrimSpace(condRaw[1:])
			} else {
				prevLocked = false
				prevCond = condRaw
			}
		}
		if _, hasEff := r.Form["choice_effects_0"]; hasEff {
			prevEff = book.CleanEffectsExpr(r.FormValue("choice_effects_0"))
		}
		if tgt := book.SanitizePlainText(r.FormValue("choice_target_0")); tgt != "" {
			if strings.EqualFold(tgt, "END") {
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
		if s.ActiveBook.Layout != nil {
			var pos book.NodePosition
			foundPos := false
			for _, k := range []string{oldExplicitID, strings.ToLower(oldExplicitID), oldName, oldNorm, oldID, strings.ToLower(oldID)} {
				if k != "" {
					if found, ok := s.ActiveBook.Layout[k]; ok {
						pos = found
						foundPos = true
						break
					}
				}
			}
			if foundPos {
				for _, k := range []string{oldExplicitID, strings.ToLower(oldExplicitID), oldName, oldNorm, oldID, strings.ToLower(oldID)} {
					if k != "" {
						delete(s.ActiveBook.Layout, k)
					}
				}
				book.StoreSceneNodePosition(s.ActiveBook, target, pos)
			}
		}
		if s.ActiveBook.WirePins != nil && oldID != "" && newID != "" && !strings.EqualFold(oldID, newID) {
			remappedPins := make(map[string][]book.WirePin, len(s.ActiveBook.WirePins))
			for wk, pins := range s.ActiveBook.WirePins {
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
			s.ActiveBook.WirePins = remappedPins
		}
		if s.ActiveBook.HasEditorView {
			if err := book.SaveBookEditorStateToDisk(s.ActiveBook); err != nil {
				s.ActiveBook.Warnings = append(s.ActiveBook.Warnings, err.Error())
			}
		}
		if matchesOldRef(s.ActiveBook.StartSceneID) || matchesOldRef(s.ActiveBook.RawStart) {
			s.ActiveBook.RawStart = newRef
			s.ActiveBook.StartSceneID = newID
		}
		for i := range s.ActiveBook.Chapters {
			ch := &s.ActiveBook.Chapters[i]
			if matchesOldRef(ch.StartSceneID) || matchesOldRef(ch.ExplicitStart) {
				if ch.ExplicitStart != "" {
					ch.ExplicitStart = newRef
				}
				ch.StartSceneID = newID
			}
		}
		for _, other := range s.ActiveBook.Scenes {
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
		if s.Save != nil && matchesOldRef(s.Save.CurrentSceneID) {
			s.Save.CurrentSceneID = newID
		}
	}
	if r.FormValue("is_start_scene") == "true" {
		s.ActiveBook.StartSceneID = target.ID
		s.ActiveBook.RawStart = target.ReferenceKey()
	}
}

func (s *Server) materializeChoicesForEdit(sc *book.Scene) []book.Choice {
	var list []book.Choice
	for _, ch := range sc.Choices {
		tgt := ch.Target
		if ch.IsImplicit && !ch.IsChapterLink && (strings.EqualFold(tgt, "Next") || strings.EqualFold(tgt, "Dalej") || tgt == "") {
			if tSc := s.findScene(ch.TargetSceneID); tSc != nil {
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

func (s *Server) selectPrimaryBranchScene(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	target := s.findScene(scID)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	s.ensureHistoryBaselineBeforeMutation()
	for _, sc := range s.ActiveBook.Scenes {
		if len(sc.Choices) >= 2 {
			sc.Choices = s.materializeChoicesForEdit(sc)
		}
	}
	if !book.PromoteSceneToPrimaryPath(s.ActiveBook, target.ID) {
		r.Form.Set("silent", "true")
		return ""
	}
	return s.commitScenesMutation("Ustawiono główną ścieżkę (primary): "+target.DisplayName(), "graph", "")
}

func (s *Server) resolveEditorSceneFromRequest(r *http.Request, fallbackActive bool) (string, *book.Scene) {
	scID := strings.TrimSpace(r.FormValue("scene"))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	if fallbackActive {
		if scID == "" && s.SceneTextModalID != "" {
			scID = s.SceneTextModalID
		}
		if scID == "" {
			scID = s.SelectedSceneID
		}
	}
	return scID, s.findScene(scID)
}

func (s *Server) commitTargetSceneChoicesMutation(target *book.Scene, msgPrefix string) string {
	prevID := target.ID
	book.FinalizeBook(s.ActiveBook)
	newID := target.ID
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	if s.SceneTextModalID != "" {
		if strings.EqualFold(s.SceneTextModalID, prevID) || strings.EqualFold(s.SceneTextModalID, newID) {
			s.SceneTextModalID = newID
			s.SceneTextModalInitialID = newID
			s.SelectedSceneID = newID
		} else {
			s.SelectedSceneID = s.SceneTextModalID
		}
	} else {
		s.SelectedSceneID = newID
	}
	msg := msgPrefix + target.DisplayName()
	s.recordHistoryAfterMutation(msg, "graph", "")
	return msg
}

func (s *Server) addChoiceToScene(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if _, hasSettingsOpen := r.Form["scene_settings_open"]; hasSettingsOpen {
		s.SceneTextModalSettingsOpen = r.FormValue("scene_settings_open") == "true"
	}
	if s.ActiveBook == nil {
		return ""
	}
	_, target := s.resolveEditorSceneFromRequest(r, false)
	if target == nil {
		return "Nie znaleziono sceny"
	}

	s.ensureHistoryBaselineBeforeMutation()
	s.applySceneFormFields(target, r)
	choices := s.materializeChoicesForEdit(target)

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

	return s.commitTargetSceneChoicesMutation(target, "Dodano nowy wybór do sceny: ")
}

func (s *Server) promptRemoveChoiceFromScene(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if _, hasSettingsOpen := r.Form["scene_settings_open"]; hasSettingsOpen {
		s.SceneTextModalSettingsOpen = r.FormValue("scene_settings_open") == "true"
	}
	if s.ActiveBook == nil {
		return ""
	}
	scID, target := s.resolveEditorSceneFromRequest(r, true)
	if target == nil {
		return ""
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx")))
	if err != nil {
		return ""
	}

	if s.SceneTextModalID != "" {
		if s.hasSceneTextModalChanges(target, r) && !s.ActiveBook.IsPacked {
			s.ensureHistoryBaselineBeforeMutation()
			s.applySceneFormFields(target, r)
			book.FinalizeBook(s.ActiveBook)
			targetID := target.ID
			targetLabel := target.DisplayName()
			if saveErr := book.SaveBookScenesToDisk(s.ActiveBook); saveErr == nil {
				s.reloadActiveBook()
				s.SelectedSceneID = targetID
				s.SceneTextModalID = targetID
				s.recordHistoryAfterMutation("Edycja treści sceny: "+targetLabel, "text", "textmodal:"+targetID)
				target = s.findScene(targetID)
			}
		}
	} else if !s.ActiveBook.IsPacked {
		s.flushFormSceneIfPresent(r)
		target = s.findScene(scID)
		if target == nil {
			target = s.findScene(s.SelectedSceneID)
		}
	}
	if target == nil || choiceIdx < 0 || choiceIdx >= len(target.Choices) {
		return ""
	}
	chLabel := strings.TrimSpace(target.Choices[choiceIdx].Label)
	if chLabel == "" {
		chLabel = fmt.Sprintf("Wybór %d", choiceIdx+1)
	}
	s.ConfirmTitle = "Usuń wybór"
	s.ConfirmMessage = fmt.Sprintf("Czy na pewno chcesz usunąć wybór „%s” ze sceny „%s”?", chLabel, target.DisplayName())
	s.ConfirmPendingAction = "RemoveChoice"
	s.ConfirmPendingParam = fmt.Sprintf("%s:%d", target.ID, choiceIdx)
	return ""
}

func (s *Server) removeChoiceFromScene(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil {
		return ""
	}
	_, target := s.resolveEditorSceneFromRequest(r, false)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx")))
	if err != nil {
		return ""
	}

	s.ensureHistoryBaselineBeforeMutation()
	s.applySceneFormFields(target, r)
	return s.removeChoiceOnTarget(target, choiceIdx)
}

func (s *Server) removeChoiceBySceneAndIndex(scID string, choiceIdx int) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil {
		return ""
	}
	target := s.findScene(scID)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	s.ensureHistoryBaselineBeforeMutation()
	return s.removeChoiceOnTarget(target, choiceIdx)
}

func (s *Server) removeChoiceOnTarget(target *book.Scene, choiceIdx int) string {
	choices := s.materializeChoicesForEdit(target)
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

	return s.commitTargetSceneChoicesMutation(target, "Usunięto wybór ze sceny: ")
}

func (s *Server) reorderChoiceOnScene(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if _, hasSettingsOpen := r.Form["scene_settings_open"]; hasSettingsOpen {
		s.SceneTextModalSettingsOpen = r.FormValue("scene_settings_open") == "true"
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	_, target := s.resolveEditorSceneFromRequest(r, true)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	fromIdx, err1 := strconv.Atoi(strings.TrimSpace(r.FormValue("from_idx")))
	toIdx, err2 := strconv.Atoi(strings.TrimSpace(r.FormValue("to_idx")))
	if err1 != nil || err2 != nil || fromIdx == toIdx {
		return ""
	}

	s.ensureHistoryBaselineBeforeMutation()
	s.applySceneFormFields(target, r)
	choices := s.materializeChoicesForEdit(target)
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

	return s.commitTargetSceneChoicesMutation(target, "Zmieniono kolejność wyborów: ")
}

func (s *Server) branchSceneFromChoice(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil {
		return ""
	}
	_, target := s.resolveEditorSceneFromRequest(r, false)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx")))
	if err != nil {
		choiceIdx = -1
	}

	s.ensureHistoryBaselineBeforeMutation()
	s.applySceneFormFields(target, r)
	choices := s.materializeChoicesForEdit(target)

	newName := book.NextUniqueSceneName(s.ActiveBook)
	allocID := s.ActiveBook.NextUniqueAutoSceneID(nil)
	s.ActiveBook.BindAutoSceneID(newName, allocID)
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

	parentIdx := len(s.ActiveBook.Scenes) - 1
	for i, sc := range s.ActiveBook.Scenes {
		if sc.ID == target.ID {
			parentIdx = i
			break
		}
	}
	insertPos := parentIdx + 1
	s.ActiveBook.Scenes = append(s.ActiveBook.Scenes[:insertPos], append([]*book.Scene{newScene}, s.ActiveBook.Scenes[insertPos:]...)...)

	book.FinalizeBook(s.ActiveBook)
	book.StoreSceneNodePosition(s.ActiveBook, newScene, book.NodePosition{
		Row:   newScene.Row,
		Col:   newScene.Col,
		X:     newScene.X,
		Y:     newScene.Y,
		HasXY: true,
	})
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	if s.SceneTextModalID != "" {
		s.SelectedSceneID = target.ID
		s.SceneTextModalID = target.ID
	} else if found := s.findScene(newScene.Name); found != nil {
		s.SelectedSceneID = found.ID
	} else {
		s.SelectedSceneID = newScene.ID
	}
	msg := "Utworzono scenę docelową: " + newScene.Name
	s.recordHistoryAfterMutation(msg, "graph", "")
	return msg
}

func (s *Server) saveSceneFromForm(r *http.Request) string {
	return s.saveSceneFromFormWithMode(r, false)
}

func (s *Server) saveSceneFromFormWithMode(r *http.Request, isAutoSave bool) string {
	if s.isHistoryPreview() {
		if isAutoSave {
			return ""
		}
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil {
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene_id"))
	var target *book.Scene
	if scID != "" {
		target = s.findScene(scID)
	}
	if target == nil && s.SceneTextModalID != "" && (scID == "" || strings.EqualFold(scID, s.SceneTextModalID) || (s.SceneTextModalInitialID != "" && strings.EqualFold(scID, s.SceneTextModalInitialID))) {
		target = s.findScene(s.SceneTextModalID)
	}
	if target == nil && s.SceneTextModalID != "" {
		target = s.findScene(s.SceneTextModalID)
	}
	if target == nil && s.SelectedSceneID != "" {
		target = s.findScene(s.SelectedSceneID)
	}
	if target == nil {
		return "Nie znaleziono sceny"
	}
	prevTargetID := target.ID

	s.ensureHistoryBaselineBeforeMutation()
	s.applySceneFormFields(target, r)

	book.FinalizeBook(s.ActiveBook)
	newID := target.ID
	targetLabel := target.DisplayName()
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	if newID != "" {
		if prevTargetID != "" && !strings.EqualFold(prevTargetID, newID) {
			s.LastRenamedFromSceneID = prevTargetID
		}
		s.SelectedSceneID = newID
		if s.SceneTextModalID != "" && (strings.EqualFold(s.SceneTextModalID, prevTargetID) || strings.EqualFold(s.SceneTextModalID, scID) || (s.SceneTextModalInitialID != "" && strings.EqualFold(s.SceneTextModalInitialID, scID))) {
			s.SceneTextModalID = newID
		}
	}
	coalesceKey := ""
	if r != nil && r.FormValue("action") == "UndoEditor" {
		coalesceKey = "major"
	} else if newID != "" {
		coalesceKey = "autosave:" + newID
	}
	s.recordHistoryAfterMutation("Edycja sceny: "+targetLabel, "scene", coalesceKey)
	return "Zapisano zmiany w pliku " + target.SourceFile
}

func (s *Server) deleteSceneByID(sceneID string) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || len(s.ActiveBook.Scenes) <= 1 {
		return "Książka musi zawierać przynajmniej jedną scenę"
	}
	delIdx := -1
	var delName string
	var delLabel string
	for i, sc := range s.ActiveBook.Scenes {
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
	s.ensureHistoryBaselineBeforeMutation()
	if delSc := s.ActiveBook.Scenes[delIdx]; delSc != nil {
		s.ActiveBook.UnbindAutoSceneName(delSc.Name)
		s.ActiveBook.RetireSceneID(delSc.ID, delSc.ExplicitID)
	}
	s.ActiveBook.Scenes = append(s.ActiveBook.Scenes[:delIdx], s.ActiveBook.Scenes[delIdx+1:]...)
	delete(s.ActiveBook.Layout, sceneID)

	if strings.EqualFold(s.ActiveBook.StartSceneID, sceneID) || (delName != "" && strings.EqualFold(s.ActiveBook.RawStart, delName)) {
		s.ActiveBook.RawStart = s.ActiveBook.Scenes[0].ReferenceKey()
		s.ActiveBook.StartSceneID = s.ActiveBook.Scenes[0].ID
	}
	for i := range s.ActiveBook.Chapters {
		ch := &s.ActiveBook.Chapters[i]
		if strings.EqualFold(ch.StartSceneID, sceneID) ||
			strings.EqualFold(ch.ExplicitStart, sceneID) ||
			(delName != "" && strings.EqualFold(ch.ExplicitStart, delName)) {
			ch.ExplicitStart = ""
		}
	}

	for _, sc := range s.ActiveBook.Scenes {
		var kept []book.Choice
		for _, ch := range sc.Choices {
			if !ch.IsImplicit && (ch.TargetSceneID == sceneID || (delName != "" && strings.EqualFold(ch.Target, delName))) {
				continue
			}
			kept = append(kept, ch)
		}
		sc.Choices = kept
	}

	book.FinalizeBook(s.ActiveBook)
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	if len(s.ActiveBook.Scenes) > 0 {
		s.SelectedSceneID = s.ActiveBook.Scenes[0].ID
	}
	msg := "Usunięto scenę: " + delLabel
	s.recordHistoryAfterMutation(msg, "graph", "")
	return msg
}

func (s *Server) deleteScenesByIDs(rawIDs string) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || len(s.ActiveBook.Scenes) == 0 {
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
		if sc := s.findScene(idPart); sc != nil && !toDeleteIDs[sc.ID] {
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
		return s.deleteSceneByID(deletedScenes[0].ID)
	}

	s.ensureHistoryBaselineBeforeMutation()
	firstChapter := s.ActiveBook.Scenes[0].Chapter
	firstChapterID := s.ActiveBook.Scenes[0].ChapterID
	firstSourceFile := s.ActiveBook.Scenes[0].SourceFile
	if firstSourceFile == "" || strings.EqualFold(firstSourceFile, book.DefaultStoryFile) || strings.EqualFold(firstSourceFile, book.BookDataFileName) {
		firstSourceFile = s.ActiveBook.ChapterFileNameFor(firstChapter, firstChapterID)
	}

	var remaining []*book.Scene
	for _, sc := range s.ActiveBook.Scenes {
		if !toDeleteIDs[sc.ID] {
			remaining = append(remaining, sc)
		}
	}

	for _, sc := range deletedScenes {
		s.ActiveBook.UnbindAutoSceneName(sc.Name)
		s.ActiveBook.RetireSceneID(sc.ID, sc.ExplicitID)
		delete(s.ActiveBook.Layout, sc.ID)
		if sc.ExplicitID != "" {
			delete(s.ActiveBook.Layout, sc.ExplicitID)
			delete(s.ActiveBook.Layout, strings.ToLower(sc.ExplicitID))
		}
		if sc.Name != "" {
			delete(s.ActiveBook.Layout, sc.Name)
			delete(s.ActiveBook.Layout, book.NormalizeLookupKey(sc.Name))
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

	s.ActiveBook.Scenes = remaining

	for _, sc := range s.ActiveBook.Scenes {
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

	if rawStart := strings.TrimSpace(s.ActiveBook.RawStart); rawStart != "" {
		rawLower := strings.ToLower(rawStart)
		rawNorm := book.NormalizeLookupKey(rawStart)
		if toDeleteIDs[s.ActiveBook.StartSceneID] || toDeleteIDs[rawStart] || toDeleteExplicitIDs[rawLower] || (rawNorm != "" && toDeleteNames[rawNorm]) {
			s.ActiveBook.RawStart = s.ActiveBook.Scenes[0].ReferenceKey()
			s.ActiveBook.StartSceneID = s.ActiveBook.Scenes[0].ID
		}
	}

	book.FinalizeBook(s.ActiveBook)
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	if len(s.ActiveBook.Scenes) > 0 {
		s.SelectedSceneID = s.ActiveBook.Scenes[0].ID
	}
	msg := fmt.Sprintf("Usunięto zaznaczone sceny (%d)", len(deletedScenes))
	s.recordHistoryAfterMutation(msg, "graph", "")
	return msg
}

func (s *Server) resolveVoiceActionTargetScene(r *http.Request) *book.Scene {
	_, target := s.resolveEditorSceneFromRequest(r, true)
	return target
}

func (s *Server) toggleSceneVoiceRecorderAction(r *http.Request) string {
	if s.ActiveBook == nil {
		return ""
	}
	if target := s.resolveVoiceActionTargetScene(r); target != nil && !s.isHistoryPreview() && !s.ActiveBook.IsPacked {
		if s.hasSceneTextModalChanges(target, r) {
			s.ensureHistoryBaselineBeforeMutation()
			s.applySceneFormFields(target, r)
			book.FinalizeBook(s.ActiveBook)
			if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
				return err.Error()
			}
			s.reloadActiveBook()
			s.recordHistoryAfterMutation("Zapisano zmiany w scenie", "scene", target.ID)
		}
	}
	if openParam := strings.TrimSpace(r.FormValue("open")); openParam == "true" {
		s.VoiceRecorderOpen = true
	} else if openParam == "false" {
		s.VoiceRecorderOpen = false
	} else {
		s.VoiceRecorderOpen = !s.VoiceRecorderOpen
	}
	return ""
}

func (s *Server) selectSceneVoiceSectionAction(r *http.Request) string {
	if s.ActiveBook == nil {
		return ""
	}
	_, _, _ = StopNativeAudioCapture()
	target := s.resolveVoiceActionTargetScene(r)
	if target != nil && !s.isHistoryPreview() && !s.ActiveBook.IsPacked {
		if s.hasSceneTextModalChanges(target, r) {
			s.ensureHistoryBaselineBeforeMutation()
			s.applySceneFormFields(target, r)
			book.FinalizeBook(s.ActiveBook)
			if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
				return err.Error()
			}
			s.reloadActiveBook()
			target = s.resolveVoiceActionTargetScene(r)
			if target != nil {
				s.recordHistoryAfterMutation("Zapisano zmiany w scenie", "scene", target.ID)
			}
		}
	}
	secKey := strings.TrimSpace(r.FormValue("voice_section"))
	if target != nil {
		secs := book.ExtractSceneVoiceSections(target, s.ActiveBook)
		for _, sec := range secs {
			if strings.EqualFold(sec.Key, secKey) {
				s.SelectedVoiceSectionKey = sec.Key
				return ""
			}
		}
		if len(secs) > 0 {
			s.SelectedVoiceSectionKey = secs[0].Key
			return ""
		}
	}
	if secKey != "" {
		s.SelectedVoiceSectionKey = secKey
	}
	return ""
}

func (s *Server) startSceneVoiceCaptureAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked || !s.ActiveBook.Narrated {
		return "Brak aktywnej książki z włączoną narracją"
	}
	if err := StartNativeAudioCapture(); err != nil {
		return err.Error()
	}
	return ""
}

func (s *Server) pollSceneVoiceCaptureAction(r *http.Request) string {
	return ""
}

func (s *Server) commitSceneVoiceFile(target *book.Scene, secKey string, relPath string, err error, historyPrefix string) string {
	if err != nil {
		return err.Error()
	}
	if book.HasConditionalVoiceDirectives(target.Text) {
		target.SetSectionVoiceMeta(secKey, relPath, 0, 0)
		s.SelectedVoiceSectionKey = secKey
	} else {
		target.ExplicitVoice = relPath
		target.VoiceTrimStart = 0
		target.VoiceTrimEnd = 0
	}
	s.VoiceRecorderOpen = true
	book.FinalizeBook(s.ActiveBook)
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	s.recordHistoryAfterMutation(historyPrefix+relPath, "scene", target.ID)
	return ""
}

func (s *Server) stopSceneVoiceCaptureAction(r *http.Request) string {
	if s.isHistoryPreview() {
		_, _, _ = StopNativeAudioCapture()
		return historyPreviewLockedToast
	}
	pcmLE, sampleRate, err := StopNativeAudioCapture()
	if err != nil {
		return err.Error()
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked || !s.ActiveBook.Narrated {
		return "Brak aktywnej książki z włączoną narracją"
	}
	target := s.resolveVoiceActionTargetScene(r)
	if target == nil {
		return "Nie znaleziono sceny"
	}
	s.ensureHistoryBaselineBeforeMutation()
	if s.hasSceneTextModalChanges(target, r) {
		s.applySceneFormFields(target, r)
	}
	secKey := s.activeVoiceSectionKeyForRequest(target, r)
	relPath, err := book.SaveSceneVoiceSectionPCM(s.ActiveBook, target, secKey, pcmLE, sampleRate, 0, 1)
	return s.commitSceneVoiceFile(target, secKey, relPath, err, "Zapisano nagranie audio: ")
}

func (s *Server) saveSceneVoiceRecordingAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil {
		return "Brak aktywnej książki"
	}
	if s.ActiveBook.IsPacked {
		return "Nie można edytować spakowanego archiwum .stbook bezpośrednio"
	}
	if !s.ActiveBook.Narrated {
		return "Książka nie posiada włączonej narracji (narrated: false)"
	}
	target := s.resolveVoiceActionTargetScene(r)
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

	s.ensureHistoryBaselineBeforeMutation()
	if s.hasSceneTextModalChanges(target, r) {
		s.applySceneFormFields(target, r)
	}
	secKey := s.activeVoiceSectionKeyForRequest(target, r)
	relPath, err := book.SaveSceneVoiceSectionPCM(s.ActiveBook, target, secKey, pcmLE, sampleRate, startRatio, endRatio)
	return s.commitSceneVoiceFile(target, secKey, relPath, err, "Zapisano nagranie audio: ")
}

func (s *Server) trimSceneVoiceAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked || !s.ActiveBook.Narrated {
		return ""
	}
	target := s.resolveVoiceActionTargetScene(r)
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

	s.ensureHistoryBaselineBeforeMutation()
	if s.hasSceneTextModalChanges(target, r) {
		s.applySceneFormFields(target, r)
	}
	secKey := s.activeVoiceSectionKeyForRequest(target, r)
	s.SelectedVoiceSectionKey = secKey

	isDestructive := strings.EqualFold(strings.TrimSpace(r.FormValue("destructive")), "true")
	if !isDestructive {
		if book.HasConditionalVoiceDirectives(target.Text) {
			book.ExtractSceneVoiceSections(target, s.ActiveBook)
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
		s.VoiceRecorderOpen = true
		book.FinalizeBook(s.ActiveBook)
		if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
			return err.Error()
		}
		s.reloadActiveBook()
		s.recordHistoryAfterMutation("Przycięto nagranie audio: "+target.DisplayName(), "scene", "voicetrim:"+target.ID+":"+secKey)
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
		relPath, err = book.SaveSceneVoiceSectionPCM(s.ActiveBook, target, secKey, pcmLE, sampleRate, startRatio, endRatio)
	} else if book.HasConditionalVoiceDirectives(target.Text) {
		book.ExtractSceneVoiceSections(target, s.ActiveBook)
		var curVoice string
		if sec := target.FindVoiceSection(secKey); sec != nil {
			curVoice = sec.ExplicitVoice
			if curVoice == "" {
				curVoice = sec.AudioFile
			}
		}
		stemWithSec := book.SceneVoiceBaseStem(target) + "__" + secKey
		relPath, err = book.TrimSceneVoiceFile(s.ActiveBook, stemWithSec, curVoice, startRatio, endRatio)
	} else {
		curVoice := target.ExplicitVoice
		if curVoice == "" {
			curVoice = target.AudioFile
		}
		relPath, err = book.TrimSceneVoiceFile(s.ActiveBook, target.ReferenceKey(), curVoice, startRatio, endRatio)
	}
	return s.commitSceneVoiceFile(target, secKey, relPath, err, "Przycięto trwale plik audio: ")
}

func (s *Server) deleteSceneVoiceAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	target := s.resolveVoiceActionTargetScene(r)
	if target == nil {
		return ""
	}
	s.ensureHistoryBaselineBeforeMutation()
	if s.hasSceneTextModalChanges(target, r) {
		s.applySceneFormFields(target, r)
	}
	secKey := s.activeVoiceSectionKeyForRequest(target, r)
	s.SelectedVoiceSectionKey = secKey
	if err := book.DeleteSceneVoiceSectionFile(s.ActiveBook, target, secKey); err != nil {
		return err.Error()
	}
	book.FinalizeBook(s.ActiveBook)
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	s.recordHistoryAfterMutation("Usunięto nagranie audio sceny", "scene", target.ID)
	return ""
}

