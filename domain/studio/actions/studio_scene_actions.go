//go:build !(js && wasm)

package actions

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	book "mojra/domain/book/actions"
	commonstate "mojra/domain/common/state"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/domain/studio/views"
	"mojra/engine/templates"
	"mojra/engine/transport"
)

const (
	formKeyScenes          = "scenes"
	formKeySceneIDs        = "scene_ids"
	formKeyParent          = "parent"
	formKeyChild           = "child"
	formKeyTarget          = "target"
	formKeyBefore          = "before"
	formKeyFromIdx         = "from_idx"
	formKeyToIdx           = "to_idx"
	formKeyText            = "text"
	formKeyName            = "name"
	formKeyExplicitID      = "explicit_id"
	formKeySlugline        = "slugline"
	formKeySynopsis        = "synopsis"
	formKeySceneEffects    = "scene_effects"
	formKeyChapter         = "chapter"
	formKeySourceFile      = "source_file"
	formKeyExplicitEnd     = "explicit_end"
	formKeyExplicitImage   = "explicit_image"
	formKeyExplicitVoice   = "explicit_voice"
	formKeyIsStartScene    = "is_start_scene"
	formKeyChoiceLabelPfx  = "choice_label_"
	formKeyChoiceTargetPfx = "choice_target_"
	formKeyChoiceCondPfx   = "choice_cond_"
	formKeyChoiceEffPfx    = "choice_effects_"
	formKeyChoiceLabel0    = "choice_label_0"
	formKeyChoiceTarget0   = "choice_target_0"
	formKeyChoiceCond0     = "choice_cond_0"
	formKeyChoiceEffects0  = "choice_effects_0"
	formKeyPrefix          = "prefix"
	formKeyKind            = "kind"
	formKeyCharacter       = "character"
	formKeyX               = "x"
	formKeyY               = "y"
	formValDalej           = "Dalej"
	formValNext            = "Next"
	formValNextChToken     = "__next__"
	formValChTargetPref    = "chapter:"
	historyScopeScene      = "scene"
	historyScopeGraph      = "graph"
	coalesceMajor          = "major"
	coalesceAutoSavePref   = "autosave:"
	coalesceSluglinePref   = "slugline:"
	coalesceTextPref       = "text:"
	slugFormatStr          = "%s %s - %s"
	actCreateSceneAt       = "CreateSceneAt"
	actCreateChapterGroup  = "CreateChapterGroup"
	actSelectStudioChapter = "SelectStudioChapter"
)

var _ = registerActions(map[string]ActionHandler{
	"SelectStudioScene":        selectStudioScene,
	"SelectPrimaryBranchScene": selectPrimaryBranchScene,
	"SaveScene":                saveSceneAction,
	"AutoSaveScene":            autoSaveSceneAction,
	"AddChoice":                addChoiceToScene,
	"RemoveChoice":             removeChoiceFromScene,
	"ReorderChoice":            reorderChoiceOnScene,
	"BranchChoiceScene":        branchSceneFromChoice,
	"InsertScene":              insertScene,
	"InsertSceneBefore":        insertSceneBefore,
	"CreateSceneAt":            createSceneAt,
	"DeleteScene":              deleteScenesFromForm,
	"DeleteScenes":             deleteScenesFromForm,
	"ApplySluglineToken":       applySluglineTokenAction,
	"InsertScreenplayBlock":    insertScreenplayBlockAction,
})

func splitInlineChoiceLabelAndTarget(raw string) (label string, target string, hasArrow bool) {
	cleaned := book.SanitizePlainText(raw)
	if idx := strings.LastIndex(cleaned, "->"); idx >= 0 {
		if idx == 0 || cleaned[idx-1] != '\\' {
			return strings.TrimSpace(cleaned[:idx]), strings.TrimSpace(cleaned[idx+2:]), true
		}
	}
	return strings.TrimSpace(cleaned), "", false
}

func resolveEditorSceneFromRequest(s *state.StudioState, r *http.Request, fallbackActive bool) (string, *book.Scene) {
	if s.Editor.ActiveBook == nil {
		return "", nil
	}
	var scID string
	if r != nil {
		scID = strings.TrimSpace(r.FormValue(formKeyScene))
		if scID == "" {
			scID = strings.TrimSpace(r.FormValue(formKeySceneID))
		}
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

func selectStudioScene(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := flushFormSceneIfPresent(s, r)
	scID := strings.TrimSpace(r.FormValue(formKeyScene))
	if found := s.Editor.ActiveBook.FindScene(scID); found != nil {
		if s.Editor.SelectedSceneID != found.ID {
			s.Editor.SelectedVoiceSectionKey = defaultVoiceSectionKey
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
	return frameInspector(s, "", hadFormScene)
}

func selectPrimaryBranchScene(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	if isHistoryPreview(s) {
		return frameWorkspace(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameWorkspace(s, "")
	}
	target := b.FindScene(strings.TrimSpace(r.FormValue(formKeyScene)))
	if target == nil {
		return frameWorkspace(s, tr(s, i18n.TrSceneNotFound))
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	for _, sc := range b.Scenes {
		if len(sc.Choices) >= 2 {
			sc.Choices = book.MaterializeChoicesForEdit(b, sc)
		}
	}
	if !book.PromoteSceneToPrimaryPath(b, target.ID) {
		r.Form.Set(formFieldSilent, formValTrue)
		return frameWorkspace(s, "")
	}
	return frameWorkspace(s, commitScenesMutation(s, tr(s, i18n.TrPromotedPrimaryPrefix)+target.DisplayName(), historyScopeGraph, ""))
}

func hasSceneTextModalChanges(s *state.StudioState, curSc *book.Scene, r *http.Request) bool {
	if curSc == nil || r == nil {
		return false
	}
	b := s.Editor.ActiveBook
	if _, hasText := r.Form[formKeyText]; hasText {
		if isolatedKey := strings.TrimSpace(r.FormValue(formKeyIsolatedVoiceSection)); isolatedKey != "" {
			if book.ReplaceSceneVoiceSectionText(curSc.Text, isolatedKey, r.FormValue(formKeyText)) != curSc.Text {
				return true
			}
		} else if book.SanitizePlainText(r.FormValue(formKeyText)) != curSc.Text {
			return true
		}
	}
	if _, hasName := r.Form[formKeyName]; hasName {
		newName := book.SanitizePlainText(r.FormValue(formKeyName))
		if (newName != "" || curSc.ExplicitID != "") && newName != curSc.Name {
			return true
		}
	}
	if _, hasID := r.Form[formKeyExplicitID]; hasID && book.SanitizeSceneID(r.FormValue(formKeyExplicitID)) != curSc.ExplicitID {
		return true
	}
	if _, hasSlug := r.Form[formKeySlugline]; hasSlug && strings.ToUpper(strings.TrimSpace(book.SanitizePlainText(r.FormValue(formKeySlugline)))) != curSc.Slugline {
		return true
	}
	if _, hasSyn := r.Form[formKeySynopsis]; hasSyn && strings.TrimSpace(book.SanitizePlainText(r.FormValue(formKeySynopsis))) != curSc.Synopsis {
		return true
	}
	if _, hasEff := r.Form[formKeySceneEffects]; hasEff && book.StripOuterDoubleBraces(r.FormValue(formKeySceneEffects)) != curSc.Effects {
		return true
	}
	if _, hasCh := r.Form[formKeyChapter]; hasCh {
		newCh := strings.TrimSpace(book.SanitizePlainText(r.FormValue(formKeyChapter)))
		if newCh != "" && !strings.EqualFold(newCh, strings.TrimSpace(curSc.Chapter)) {
			return true
		}
	}
	if _, hasImg := r.Form[formKeyExplicitImage]; hasImg {
		curImg := curSc.ExplicitImage
		if curImg == "" && curSc.ImageFile != "" {
			curImg = curSc.ImageFile
		}
		if book.SanitizeRelativeMediaPath(r.FormValue(formKeyExplicitImage)) != curImg {
			return true
		}
	}
	if b != nil && b.Narrated {
		if _, hasVoice := r.Form[formKeyExplicitVoice]; hasVoice {
			newVoice := book.SanitizeRelativeMediaPath(r.FormValue(formKeyExplicitVoice))
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
		idxStr := strconv.Itoa(i)
		lblKey, tgtKey := formKeyChoiceLabelPfx+idxStr, formKeyChoiceTargetPfx+idxStr
		var inlineTgt string
		var hasArrow bool
		if _, hasLbl := r.Form[lblKey]; hasLbl {
			var newLbl string
			newLbl, inlineTgt, hasArrow = splitInlineChoiceLabelAndTarget(r.FormValue(lblKey))
			if newLbl == "" {
				newLbl = trf(s, i18n.TrChoiceDefaultFmt, i+1)
			}
			if newLbl != ch.Label {
				return true
			}
		}
		newTgt := ""
		if hasArrow && inlineTgt != "" {
			newTgt = inlineTgt
		} else if _, hasTgt := r.Form[tgtKey]; hasTgt {
			newTgt = book.SanitizePlainText(r.FormValue(tgtKey))
		}
		if newTgt != "" && !strings.EqualFold(newTgt, ch.Target) && !strings.EqualFold(newTgt, ch.TargetSceneID) {
			return true
		}
		if _, hasCond := r.Form[formKeyChoiceCondPfx+idxStr]; hasCond {
			rawCond := book.StripOuterDoubleBraces(r.FormValue(formKeyChoiceCondPfx + idxStr))
			showLocked := strings.HasPrefix(rawCond, "?")
			if showLocked {
				rawCond = strings.TrimSpace(rawCond[1:])
			}
			if rawCond != ch.Condition || showLocked != ch.ShowWhenLocked {
				return true
			}
		}
		if _, hasEff := r.Form[formKeyChoiceEffPfx+idxStr]; hasEff && book.StripOuterDoubleBraces(r.FormValue(formKeyChoiceEffPfx+idxStr)) != ch.Effects {
			return true
		}
	}
	if len(curSc.Choices) == 0 || curSc.ExplicitEnd {
		if _, hasTgt0 := r.Form[formKeyChoiceTarget0]; hasTgt0 {
			if newTgt := book.SanitizePlainText(r.FormValue(formKeyChoiceTarget0)); newTgt != "" && !strings.EqualFold(newTgt, formValEND) {
				return true
			}
		}
	}
	return false
}

func applySceneFormFields(s *state.StudioState, target *book.Scene, r *http.Request) {
	b := s.Editor.ActiveBook
	if target == nil || b == nil || r == nil {
		return
	}
	if formSceneID := strings.TrimSpace(r.FormValue(formKeySceneID)); formSceneID != "" && !strings.EqualFold(formSceneID, target.ID) {
		if !(s.Editor.SceneText.InitialID != "" && strings.EqualFold(formSceneID, s.Editor.SceneText.InitialID) && strings.EqualFold(s.Editor.SceneText.SceneID, target.ID)) {
			return
		}
	}
	oldID, oldName, oldExplicitID := target.ID, target.Name, target.ExplicitID
	if _, hasID := r.Form[formKeyExplicitID]; hasID {
		rawExp := strings.TrimSpace(r.FormValue(formKeyExplicitID))
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
	if _, hasName := r.Form[formKeyName]; hasName {
		newName := book.SanitizePlainText(r.FormValue(formKeyName))
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
	if _, hasSlug := r.Form[formKeySlugline]; hasSlug {
		target.Slugline = strings.ToUpper(strings.TrimSpace(book.SanitizePlainText(r.FormValue(formKeySlugline))))
	}
	if _, hasSyn := r.Form[formKeySynopsis]; hasSyn {
		target.Synopsis = strings.TrimSpace(book.SanitizePlainText(r.FormValue(formKeySynopsis)))
	}
	if _, hasEff := r.Form[formKeySceneEffects]; hasEff {
		target.Effects = book.CleanEffectsExpr(r.FormValue(formKeySceneEffects))
	}
	if _, hasImg := r.Form[formKeyExplicitImage]; hasImg {
		target.ExplicitImage = book.SanitizeRelativeMediaPath(r.FormValue(formKeyExplicitImage))
		if target.ExplicitImage == "" {
			target.ImageFile, target.ImageURL = "", ""
		}
	}
	if _, hasText := r.Form[formKeyText]; hasText {
		if isolatedKey := strings.TrimSpace(r.FormValue(formKeyIsolatedVoiceSection)); isolatedKey != "" {
			target.Text = book.ReplaceSceneVoiceSectionText(target.Text, isolatedKey, r.FormValue(formKeyText))
		} else {
			target.Text = book.SanitizePlainText(r.FormValue(formKeyText))
		}
	}

	if !b.Narrated {
		target.ExplicitVoice, target.VoiceTrimStart, target.VoiceTrimEnd = "", 0, 0
		target.SectionVoices, target.AudioFile, target.AudioURL = nil, "", ""
	} else if _, hasVoice := r.Form[formKeyExplicitVoice]; hasVoice {
		newVoice := book.SanitizeRelativeMediaPath(r.FormValue(formKeyExplicitVoice))
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
				target.VoiceTrimStart, target.VoiceTrimEnd = 0, 0
			}
			target.ExplicitVoice = newVoice
			if target.ExplicitVoice == "" {
				target.AudioFile, target.AudioURL = "", ""
			}
		}
	}
	chapterChanged := false
	act := strings.TrimSpace(r.FormValue(formFieldAction))
	if _, hasChapter := r.Form[formKeyChapter]; hasChapter && act != actCreateSceneAt && act != actCreateChapterGroup && act != actSelectStudioChapter {
		newCh := book.SanitizePlainText(r.FormValue(formKeyChapter))
		if !strings.EqualFold(strings.TrimSpace(target.Chapter), newCh) {
			chapterChanged = true
		}
		target.Chapter = newCh
	}
	if rawSF := r.FormValue(formKeySourceFile); rawSF != "" && !chapterChanged {
		if sf := filepath.Base(book.SanitizePlainText(rawSF)); sf != "" && sf != "." {
			target.SourceFile = sf
		}
	}
	if chapterChanged {
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
	if _, hasExplicitEnd := r.Form[formKeyExplicitEnd]; hasExplicitEnd {
		target.ExplicitEnd = r.FormValue(formKeyExplicitEnd) == formValTrue
	}

	applySceneFormChoices(s, b, target, r)

	_ = book.RebindRenamedScene(b, target, oldID, oldName, oldExplicitID)
	if r.FormValue(formKeyIsStartScene) == formValTrue {
		b.StartSceneID, b.RawStart = target.ID, target.ReferenceKey()
	}
}

func resolveEditedChoiceTarget(b *book.Book, target *book.Scene, tgt string) string {
	tgt = strings.TrimSpace(tgt)
	if tgt == "" || strings.EqualFold(tgt, formValEND) || strings.EqualFold(tgt, formValNext) || strings.EqualFold(tgt, formValDalej) {
		return tgt
	}
	if chRef, isCh := book.ParseChapterTarget(tgt); isCh {
		if chRef == formValNextChToken {
			for _, candCh := range b.Chapters {
				if ok, _ := b.CanLinkChapters(target.Chapter, candCh.ReferenceKey()); ok {
					return formValChTargetPref + candCh.ReferenceKey()
				}
			}
			return ""
		}
		if targetCh := b.FindChapter(chRef); targetCh != nil {
			if ok, _ := b.CanLinkChapters(target.Chapter, targetCh.ReferenceKey()); ok {
				return formValChTargetPref + targetCh.ReferenceKey()
			}
		}
		return ""
	}
	if destSc := b.FindScene(tgt); destSc != nil {
		if !strings.EqualFold(strings.TrimSpace(destSc.Chapter), strings.TrimSpace(target.Chapter)) {
			if targetCh := b.FindChapter(destSc.Chapter); targetCh != nil && strings.EqualFold(destSc.ID, targetCh.StartSceneID) {
				if ok, _ := b.CanLinkChapters(target.Chapter, targetCh.ReferenceKey()); ok {
					return formValChTargetPref + targetCh.ReferenceKey()
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

func applySceneFormChoices(s *state.StudioState, b *book.Book, target *book.Scene, r *http.Request) {
	if len(target.Choices) >= 2 {
		updatedChoices := make([]book.Choice, 0, len(target.Choices))
		for i, ch := range target.Choices {
			idxStr := strconv.Itoa(i)
			lblKey, tgtKey := formKeyChoiceLabelPfx+idxStr, formKeyChoiceTargetPfx+idxStr
			condKey, effKey := formKeyChoiceCondPfx+idxStr, formKeyChoiceEffPfx+idxStr
			newLbl := ch.Label
			var inlineTgt string
			var hasInlineArrow bool
			if _, ok := r.Form[lblKey]; ok {
				newLbl, inlineTgt, hasInlineArrow = splitInlineChoiceLabelAndTarget(r.FormValue(lblKey))
			}
			if newLbl == "" {
				newLbl = trf(s, i18n.TrChoiceDefaultFmt, i+1)
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
			newTgt = resolveEditedChoiceTarget(b, target, newTgt)
			if newTgt == "" {
				newTgt = ch.Target
			}
			newCond, showLocked := ch.Condition, ch.ShowWhenLocked
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
					Label: newLbl, Target: newTgt, Condition: newCond, ShowWhenLocked: showLocked,
					Effects: newEff, IsEnding: strings.EqualFold(newTgt, formValEND),
				})
			}
		}
		target.Choices, target.ExplicitNext, target.ExplicitEnd = updatedChoices, "", false
		return
	}

	var prevCond, prevEff string
	var prevLocked bool
	if len(target.Choices) == 1 {
		prevCond, prevLocked, prevEff = target.Choices[0].Condition, target.Choices[0].ShowWhenLocked, target.Choices[0].Effects
	}
	if _, ok := r.Form[formKeyChoiceCond0]; ok {
		rawCond := book.StripOuterDoubleBraces(r.FormValue(formKeyChoiceCond0))
		prevLocked = strings.HasPrefix(rawCond, "?")
		if prevLocked {
			rawCond = strings.TrimSpace(rawCond[1:])
		}
		prevCond = rawCond
	}
	if _, ok := r.Form[formKeyChoiceEffects0]; ok {
		prevEff = book.StripOuterDoubleBraces(r.FormValue(formKeyChoiceEffects0))
	}
	_, inlineTgt0, hasArrow0 := splitInlineChoiceLabelAndTarget(r.FormValue(formKeyChoiceLabel0))
	_, hasTgt0 := r.Form[formKeyChoiceTarget0]
	if hasTgt0 || (hasArrow0 && inlineTgt0 != "") {
		tgt := book.SanitizePlainText(r.FormValue(formKeyChoiceTarget0))
		if hasArrow0 && inlineTgt0 != "" {
			tgt = inlineTgt0
		}
		tgt = resolveEditedChoiceTarget(b, target, tgt)
		if strings.EqualFold(tgt, formValEND) || tgt == "" {
			target.Choices, target.ExplicitNext, target.ExplicitEnd = nil, "", true
		} else {
			target.ExplicitEnd, target.ExplicitNext = false, tgt
			target.Choices = []book.Choice{{
				Label: formValDalej, Target: tgt, Condition: prevCond,
				ShowWhenLocked: prevLocked, Effects: prevEff, IsImplicit: true,
			}}
		}
	} else if len(target.Choices) == 1 {
		target.Choices[0].Condition, target.Choices[0].ShowWhenLocked, target.Choices[0].Effects = prevCond, prevLocked, prevEff
	}
}

func commitTargetSceneChoicesMutation(s *state.StudioState, target *book.Scene, msgPrefix string) string {
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
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, historyScopeGraph, "", s.Editor.SelectedSceneID)
	return msg
}

func addChoiceToScene(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := hasFormScene(r)
	prevSceneTextID := s.Editor.SceneText.SceneID
	reqSc := strings.TrimSpace(r.FormValue(formKeyScene))
	toast := ""
	if isHistoryPreview(s) {
		toast = tr(s, i18n.TrHistoryPreviewLocked)
	} else {
		syncSceneTextSettingsOpen(s, r)
		b := s.Editor.ActiveBook
		if b != nil {
			_, target := resolveEditorSceneFromRequest(s, r, false)
			if target == nil {
				toast = tr(s, i18n.TrSceneNotFound)
			} else {
				s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
				applySceneFormFields(s, target, r)
				book.AddChoiceToScene(b, target)
				toast = commitTargetSceneChoicesMutation(s, target, tr(s, i18n.TrChoiceAddedPrefix))
			}
		}
	}
	if s.Editor.SceneText.IsOpen() && !hadFormScene && reqSc != "" && !strings.EqualFold(reqSc, prevSceneTextID) {
		return frameApp(s, toast)
	}
	return frameSceneForm(s, toast, hadFormScene, false)
}

func removeChoiceFromScene(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := hasFormScene(r)
	if isHistoryPreview(s) {
		return frameSceneForm(s, tr(s, i18n.TrHistoryPreviewLocked), hadFormScene, false)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return frameSceneForm(s, "", hadFormScene, false)
	}
	_, target := resolveEditorSceneFromRequest(s, r, false)
	if target == nil {
		return frameSceneForm(s, tr(s, i18n.TrSceneNotFound), hadFormScene, false)
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyChoiceIdx)))
	if err != nil {
		return frameSceneForm(s, "", hadFormScene, false)
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	book.RemoveChoiceFromScene(b, target, choiceIdx)
	return frameSceneForm(s, commitTargetSceneChoicesMutation(s, target, tr(s, i18n.TrChoiceRemovedPrefix)), hadFormScene, false)
}

func removeChoiceBySceneAndIndex(s *state.StudioState, scID string, choiceIdx int) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	target := b.FindScene(scID)
	if target == nil {
		return tr(s, i18n.TrSceneNotFound)
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	book.RemoveChoiceFromScene(b, target, choiceIdx)
	return commitTargetSceneChoicesMutation(s, target, tr(s, i18n.TrChoiceRemovedPrefix))
}

func reorderChoiceOnScene(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := hasFormScene(r)
	if isHistoryPreview(s) {
		return frameSceneForm(s, tr(s, i18n.TrHistoryPreviewLocked), hadFormScene, false)
	}
	syncSceneTextSettingsOpen(s, r)
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameSceneForm(s, "", hadFormScene, false)
	}
	_, target := resolveEditorSceneFromRequest(s, r, true)
	if target == nil {
		return frameSceneForm(s, tr(s, i18n.TrSceneNotFound), hadFormScene, false)
	}
	fromIdx, err1 := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyFromIdx)))
	toIdx, err2 := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyToIdx)))
	if err1 != nil || err2 != nil || fromIdx == toIdx {
		return frameSceneForm(s, "", hadFormScene, false)
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	if !book.ReorderSceneChoice(b, target, fromIdx, toIdx) {
		return frameSceneForm(s, "", hadFormScene, false)
	}
	return frameSceneForm(s, commitTargetSceneChoicesMutation(s, target, tr(s, i18n.TrChoiceReorderedPrefix)), hadFormScene, false)
}

func branchSceneFromChoice(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return frameUndoRedo(s, "")
	}
	_, target := resolveEditorSceneFromRequest(s, r, false)
	if target == nil {
		return frameUndoRedo(s, tr(s, i18n.TrSceneNotFound))
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyChoiceIdx)))
	if err != nil {
		choiceIdx = -1
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	newScene, brErr := book.BranchChoiceToNewScene(b, target, choiceIdx)
	if brErr != nil {
		return frameUndoRedo(s, brErr.Error())
	}
	if newScene == nil {
		return frameUndoRedo(s, "")
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
	msg := tr(s, i18n.TrChoiceBranchedPrefix) + newScene.Name
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, historyScopeGraph, "", s.Editor.SelectedSceneID)
	return frameUndoRedo(s, msg)
}

type sceneFormSaveMode uint8

const (
	sceneSaveForm sceneFormSaveMode = iota
	sceneSaveAuto
	sceneSaveTextModal
	sceneFlushTextModal
)

func autoSaveSceneAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	toast := ""
	if !isHistoryPreview(s) {
		savedPref := tr(s, i18n.TrSavedInFilePrefix)
		notFound := tr(s, i18n.TrSceneNotFound)
		if _, msg := commitSceneForm(s, r, sceneSaveAuto); msg != "" && !strings.HasPrefix(msg, savedPref) && msg != notFound {
			toast = msg
		}
	}
	s.Overlays.Toast = toast
	if s.Settings.ActiveView == string(commonstate.ViewEditor) && !s.Config.StandaloneMode && !s.Editor.StudioHome {
		if s.Editor.SceneText.IsOpen() {
			return []transport.ActionFrame{
				{Target: targetSelInspector, Chunks: []templates.TemplateChunk{views.EditorContextView(s)}},
				{Target: targetSelNone, Chunks: []templates.TemplateChunk{views.TextEditorScriptView(s)}},
			}
		}
		if r.FormValue(formFieldSilent) == formValTrue {
			return frameNone(s, toast, views.EditorContextScriptView(s))
		}
		return []transport.ActionFrame{
			{Target: targetSelInspector, Chunks: []templates.TemplateChunk{views.EditorContextView(s)}},
		}
	}
	return frameNone(s, toast)
}

func saveSceneAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	if isHistoryPreview(s) {
		return frameWorkspace(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	toast := ""
	savedPref := tr(s, i18n.TrSavedInFilePrefix)
	notFound := tr(s, i18n.TrSceneNotFound)
	if _, msg := commitSceneForm(s, r, sceneSaveForm); msg != "" && !strings.HasPrefix(msg, savedPref) && msg != notFound {
		toast = msg
	}
	return frameWorkspace(s, toast)
}

func commitSceneForm(s *state.StudioState, r *http.Request, mode sceneFormSaveMode) (bool, string) {
	if isHistoryPreview(s) {
		if mode == sceneSaveAuto || mode == sceneFlushTextModal {
			return false, ""
		}
		return false, tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return false, ""
	}
	scID := strings.TrimSpace(r.FormValue(formKeySceneID))
	var target *book.Scene
	if mode == sceneFlushTextModal {
		curID := scID
		if curID == "" {
			curID = s.Editor.SceneText.SceneID
		}
		target = b.FindScene(curID)
	} else if mode == sceneSaveTextModal {
		target = resolveSceneTextTarget(s, r)
	} else {
		if scID != "" {
			target = b.FindScene(scID)
		}
		if target == nil && s.Editor.SceneText.IsOpen() {
			target = b.FindScene(s.Editor.SceneText.SceneID)
		}
		if target == nil && s.Editor.SelectedSceneID != "" {
			target = b.FindScene(s.Editor.SelectedSceneID)
		}
	}
	if target == nil {
		if mode == sceneSaveTextModal || mode == sceneFlushTextModal {
			return false, ""
		}
		return false, tr(s, i18n.TrSceneNotFound)
	}
	if mode == sceneFlushTextModal && !hasSceneTextModalChanges(s, target, r) {
		return false, ""
	}
	prevTargetID := target.ID
	if mode == sceneSaveTextModal {
		if formPrev := book.SanitizeSceneID(scID); formPrev != "" {
			prevTargetID = formPrev
		}
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	book.FinalizeBook(b)
	newID, targetLabel := target.ID, target.DisplayName()
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return false, err.Error()
	}
	reloadActiveBook(s)
	if newID != "" {
		if prevTargetID != "" && !strings.EqualFold(prevTargetID, newID) {
			s.Editor.LastRenamedFromSceneID = prevTargetID
		}
		s.Editor.SelectedSceneID = newID
		if mode == sceneSaveTextModal || mode == sceneFlushTextModal {
			s.Editor.SceneText.SyncSceneID(newID)
		} else if s.Editor.SceneText.IsOpen() && (strings.EqualFold(s.Editor.SceneText.SceneID, prevTargetID) || strings.EqualFold(s.Editor.SceneText.SceneID, scID) || (s.Editor.SceneText.InitialID != "" && strings.EqualFold(s.Editor.SceneText.InitialID, scID))) {
			s.Editor.SceneText.SceneID = newID
		}
	}
	if mode == sceneFlushTextModal {
		histLabel := trf(s, i18n.TrHistSceneTextEditedFmt, targetLabel)
		s.Editor.History.RecordStep(s.Editor.ActiveBook, histLabel, histKindText, coalesceTextModalPref+newID, s.Editor.SelectedSceneID)
		return true, ""
	}
	if mode == sceneSaveTextModal {
		toast := trf(s, i18n.TrSceneTextSavedFmt, targetLabel)
		s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, histKindText, coalesceTextModalPref+newID, s.Editor.SelectedSceneID)
		return true, toast
	}
	coalesceKey := ""
	if r != nil && r.FormValue(formFieldAction) == actUndoEditor {
		coalesceKey = coalesceMajor
	} else if newID != "" {
		coalesceKey = coalesceAutoSavePref + newID
	}
	s.Editor.History.RecordStep(s.Editor.ActiveBook, tr(s, i18n.TrEditScenePrefix)+targetLabel, historyScopeScene, coalesceKey, s.Editor.SelectedSceneID)
	return true, tr(s, i18n.TrSavedInFilePrefix) + target.SourceFile
}

func finishInsertedScene(s *state.StudioState, inserted *book.Scene, err error) string {
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
	toast := tr(s, i18n.TrSceneInsertedPrefix) + inserted.Name
	s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, historyScopeGraph, "", s.Editor.SelectedSceneID)
	return toast
}

func insertScene(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return frameUndoRedo(s, "")
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	parentID := strings.TrimSpace(r.FormValue(formKeyParent))
	childID := strings.TrimSpace(r.FormValue(formKeyChild))
	if parentID == "" && len(b.Scenes) > 0 {
		parentID = b.Scenes[len(b.Scenes)-1].ID
	}
	inserted, err := book.InsertSceneBetween(b, parentID, childID, book.NextUniqueSceneName(b))
	return frameUndoRedo(s, finishInsertedScene(s, inserted, err))
}

func insertSceneBefore(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return frameUndoRedo(s, "")
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	targetID := strings.TrimSpace(r.FormValue(formKeyTarget))
	if targetID == "" {
		targetID = strings.TrimSpace(r.FormValue(formKeyBefore))
	}
	if targetID == "" {
		targetID = strings.TrimSpace(r.FormValue(formKeyScene))
	}
	inserted, err := book.InsertSceneBefore(b, targetID, book.NextUniqueSceneName(b))
	return frameUndoRedo(s, finishInsertedScene(s, inserted, err))
}

func createSceneAt(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	hadEditorBook := s.Editor.ActiveBook != nil
	if !hadEditorBook && !s.Config.StandaloneMode {
		if createdBook, err := book.CreateNewBook(s.Config.BooksDir, tr(s, i18n.TrDefaultBookTitle), tr(s, i18n.TrAuthorPlaceholder), false); err == nil && createdBook != nil {
			if s.Hooks.OnBookMutated != nil {
				s.Hooks.OnBookMutated(createdBook)
			}
			s.Editor.SetBook(createdBook, s.Settings)
			s.SaveSettings()
		}
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameUndoRedo(s, "")
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	x, errX := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyX)))
	y, errY := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyY)))
	created, err := book.CreateStandaloneSceneInChapter(b, book.NextUniqueSceneName(b), strings.TrimSpace(r.FormValue(formKeyChapter)), x, y, errX == nil && errY == nil)
	if err != nil {
		return frameUndoRedo(s, err.Error())
	}
	if created == nil {
		return frameUndoRedo(s, "")
	}
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
	toast := tr(s, i18n.TrSceneCreatedAtPrefix) + created.Name
	s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, historyScopeGraph, "", s.Editor.SelectedSceneID)
	if !hadEditorBook {
		return frameApp(s, toast)
	}
	return frameUndoRedo(s, toast)
}

func deleteScenesFromForm(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	if isHistoryPreview(s) {
		return frameApp(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	rawIDs := strings.TrimSpace(r.FormValue(formKeyScenes))
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue(formKeySceneIDs))
	}
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue(formKeyScene))
	}
	if rawIDs == "" {
		rawIDs = s.Editor.SelectedSceneID
	}
	return frameApp(s, deleteScenesByIDs(s, rawIDs))
}

func deleteScenesByIDs(s *state.StudioState, rawIDs string) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	isMulti := strings.Contains(rawIDs, ",")
	deleted, err := book.DeleteScenesByIDs(b, rawIDs, isMulti)
	if err != nil {
		return err.Error()
	}
	if len(deleted) == 0 {
		if !isMulti {
			return tr(s, i18n.TrSceneNotFound)
		}
		return ""
	}
	return finalizeScenesDeletion(s, deleted)
}

func finalizeScenesDeletion(s *state.StudioState, deleted []*book.Scene) string {
	reloadActiveBook(s)
	b := s.Editor.ActiveBook
	s.Editor.SelectedSceneID = b.ResolveValidSceneID(b.StartSceneID)
	if s.Editor.SceneText.IsOpen() && b.FindScene(s.Editor.SceneText.SceneID) == nil {
		s.Editor.SceneText.Close()
	}
	msg := trf(s, i18n.TrScenesDeletedMultiFmt, len(deleted))
	if len(deleted) == 1 {
		msg = trf(s, i18n.TrSceneDeletedFmt, deleted[0].DisplayName())
	}
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, historyScopeGraph, "", s.Editor.SelectedSceneID)
	return msg
}

func prepareScreenplaySceneEdit(s *state.StudioState, r *http.Request) *book.Scene {
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

func applySluglineTokenAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := hasFormScene(r)
	if isHistoryPreview(s) {
		return frameSceneForm(s, tr(s, i18n.TrHistoryPreviewLocked), hadFormScene, false)
	}
	target := prepareScreenplaySceneEdit(s, r)
	if target == nil {
		return frameSceneForm(s, "", hadFormScene, false)
	}
	b := s.Editor.ActiveBook
	std := b.NormalizedStandard()
	prefixes, times := book.SluglineHelperPills(std)
	info := book.ParseSlugline(target.Slugline)
	prefix := info.IntExt
	if prefix == "" {
		if defInfo := book.ParseSlugline(book.DefaultSluglineForStandard(std)); defInfo.IntExt != "" {
			prefix = defInfo.IntExt
		} else if len(prefixes) > 0 {
			prefix = prefixes[0]
		}
	}
	loc, tod := info.Location, info.TimeOfDay
	if loc == "" {
		loc = tr(s, i18n.TrSpInsLocation)
	}
	if tod == "" && len(times) > 0 {
		tod = times[0]
	}
	if newPrefix := strings.ToUpper(strings.TrimSpace(r.FormValue(formKeyPrefix))); newPrefix != "" {
		prefix = newPrefix
	}
	if newTime := strings.ToUpper(strings.TrimSpace(r.FormValue(formKeyTime))); newTime != "" {
		tod = newTime
	}
	target.Slugline = fmt.Sprintf(slugFormatStr, prefix, loc, tod)
	return frameSceneForm(s, commitScenesMutation(s, tr(s, i18n.TrSluglineUpdatedPrefix)+target.Slugline, historyScopeScene, coalesceSluglinePref+target.ID), hadFormScene, false)
}

func insertScreenplayBlockAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	hadFormScene := hasFormScene(r)
	if isHistoryPreview(s) {
		return frameSceneForm(s, tr(s, i18n.TrHistoryPreviewLocked), hadFormScene, false)
	}
	target := prepareScreenplaySceneEdit(s, r)
	if target == nil {
		return frameSceneForm(s, "", hadFormScene, false)
	}
	kind := strings.ToLower(strings.TrimSpace(r.FormValue(formKeyKind)))
	charName := strings.ToUpper(strings.TrimSpace(book.SanitizePlainText(r.FormValue(formKeyCharacter))))
	if charName != "" && kind == "" {
		kind = book.ScreenplayInsertDialogue
	}
	snippet, _, dblNl := book.ScreenplayBlockSnippet(s.Editor.ActiveBook.NormalizedStandard(), kind, charName, views.LangOf(s))
	if snippet != "" {
		cur := strings.TrimRight(target.Text, " \t\n")
		if cur == "" {
			target.Text = strings.TrimLeft(snippet, "\n")
		} else if dblNl {
			target.Text = cur + "\n\n" + strings.TrimLeft(snippet, "\n")
		} else {
			target.Text = cur + "\n" + strings.TrimLeft(snippet, "\n")
		}
		target.HTML = ""
	}
	return frameSceneForm(s, commitScenesMutation(s, tr(s, i18n.TrSpBlockInsertedPrefix)+target.DisplayName(), historyScopeScene, coalesceTextPref+target.ID), hadFormScene, false)
}

const (
	actUndoEditor          = "UndoEditor"
	formKeyTime            = "time"
	pendingDeleteScenes    = "DeleteScenes"
	pendingRemoveChoice    = "RemoveChoice"
	quotedNameFmt          = "„%s”"
	pendingParamFmt        = "%s:%d"
)

var _ = registerActions(map[string]ActionHandler{
	"PromptDeleteScene":  promptDeleteScenesFromForm,
	"PromptDeleteScenes": promptDeleteScenesFromForm,
	"PromptRemoveChoice": promptRemoveChoiceFromScene,
})

func promptDeleteScenesFromForm(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	if isHistoryPreview(s) {
		return frameModals(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return frameModals(s, "")
	}
	rawIDs := strings.TrimSpace(r.FormValue(formKeyScenes))
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue(formKeySceneIDs))
	}
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue(formKeyScene))
	}
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue(formKeySceneID))
	}
	if rawIDs == "" {
		rawIDs = s.Editor.SelectedSceneID
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
		s.Confirm.Prompt(commonstate.ConfirmState{
			Title:         tr(s, i18n.TrDeleteSceneBtn),
			Message:       trf(s, i18n.TrDeleteSceneConfirmFmt, sc.DisplayName(), sc.SourceFile),
			PendingAction: pendingDeleteScenes,
			PendingParam:  sc.ID,
			Danger:        true,
			OnAccept: func() []transport.ActionFrame {
				toast := deleteScenesByIDs(s, sc.ID)
				return frameApp(s, toast)
			},
		})
	} else if len(matched) > 1 {
		var ids []string
		var names []string
		for _, sc := range matched {
			ids = append(ids, sc.ID)
			names = append(names, fmt.Sprintf(quotedNameFmt, sc.DisplayName()))
		}
		joinedIDs := strings.Join(ids, ",")
		s.Confirm.Prompt(commonstate.ConfirmState{
			Title:         trf(s, i18n.TrDeleteScenesTitleFmt, len(matched)),
			Message:       trf(s, i18n.TrDeleteScenesConfirmFmt, len(matched), strings.Join(names, ", ")),
			PendingAction: pendingDeleteScenes,
			PendingParam:  joinedIDs,
			Danger:        true,
			OnAccept: func() []transport.ActionFrame {
				toast := deleteScenesByIDs(s, joinedIDs)
				return frameApp(s, toast)
			},
		})
	}
	return frameModals(s, "")
}

func promptRemoveChoiceFromScene(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	if isHistoryPreview(s) {
		return frameModals(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	syncSceneTextSettingsOpen(s, r)
	b := s.Editor.ActiveBook
	if b == nil {
		return frameModals(s, "")
	}
	scID, target := resolveEditorSceneFromRequest(s, r, true)
	if target == nil {
		return frameModals(s, "")
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyChoiceIdx)))
	if err != nil {
		return frameModals(s, "")
	}
	if s.Editor.SceneText.IsOpen() {
		if saved, _ := flushSceneTextChanges(s, r); saved {
			target = s.Editor.ActiveBook.FindScene(s.Editor.SceneText.SceneID)
		}
	} else if !b.IsPacked {
		_ = flushFormSceneIfPresent(s, r)
		target = s.Editor.ActiveBook.FindScene(scID)
		if target == nil {
			target = s.Editor.ActiveBook.FindScene(s.Editor.SelectedSceneID)
		}
	}
	if target == nil || choiceIdx < 0 || choiceIdx >= len(target.Choices) {
		return frameModals(s, "")
	}
	chLabel := strings.TrimSpace(target.Choices[choiceIdx].Label)
	if chLabel == "" {
		chLabel = trf(s, i18n.TrChoiceDefaultFmt, choiceIdx+1)
	}
	targetID := target.ID
	s.Confirm.Prompt(commonstate.ConfirmState{
		Title:         tr(s, i18n.TrDeleteChoiceTitle),
		Message:       trf(s, i18n.TrDeleteChoiceConfirmFmt, chLabel, target.DisplayName()),
		PendingAction: pendingRemoveChoice,
		PendingParam:  fmt.Sprintf(pendingParamFmt, targetID, choiceIdx),
		Danger:        true,
		OnAccept: func() []transport.ActionFrame {
			toast := removeChoiceBySceneAndIndex(s, targetID, choiceIdx)
			if s.Editor.SceneText.IsOpen() {
				var modalsFrame transport.ActionFrame
				if s.Hooks.RenderModalsFrame != nil {
					modalsFrame = s.Hooks.RenderModalsFrame()
				}
				s.Overlays.Toast = toast
				return []transport.ActionFrame{
					modalsFrame,
					{Target: targetSelInspector, Chunks: []templates.TemplateChunk{views.EditorContextView(s)}},
				}
			}
			return frameApp(s, toast)
		},
	})
	return frameModals(s, "")
}

func ExecutePendingStudioConfirm(s *state.StudioState, pendingAction, pendingParam string) ([]transport.ActionFrame, bool) {
	switch pendingAction {
	case UnpackPlaybookForStudio:
		toast := UnpackPlaybookWorkingCopyForStudio(s, pendingParam)
		return frameAppResetCanvas(s, toast), true
	case pendingDeleteScenes:
		toast := deleteScenesByIDs(s, pendingParam)
		return frameUndoRedo(s, toast), true
	case CreateHistoryBranch:
		toast := createHistoryBranchByName(s, pendingParam, "")
		return frameUndoRedo(s, toast), true
	case pendingRemoveChoice:
		parts := strings.SplitN(pendingParam, ":", 2)
		if len(parts) == 2 {
			if idx, err := strconv.Atoi(parts[1]); err == nil {
				toast := removeChoiceBySceneAndIndex(s, parts[0], idx)
				return frameSceneForm(s, toast, false, true), true
			}
		}
		return frameUndoRedo(s, ""), true
	}
	return nil, false
}

func AddBookMediaFromDiskPaths(s *state.StudioState, paths []string, kind, sceneID string) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return tr(s, i18n.TrVoiceNoActiveBook)
	}
	if b.IsPacked {
		return tr(s, i18n.TrVoicePackedBookReadOnly)
	}
	imported, err := book.ImportBookMediaFromDisk(b, paths, kind)
	if err != nil {
		return err.Error()
	}
	if len(imported) == 0 {
		return ""
	}
	if sceneID == "" {
		sceneID = s.Editor.SelectedSceneID
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if target := b.FindScene(sceneID); target != nil {
		assignedImg := false
		assignedAud := false
		for _, rel := range imported {
			if !assignedImg && (kind == mediaKindImage || kind == mediaKindImg || kind == "") && book.IsRasterImagePath(rel) {
				target.ExplicitImage = rel
				assignedImg = true
			}
			if !assignedAud && b.Narrated && (kind == mediaKindAudio || kind == mediaKindVoice || kind == "") && book.IsAudioPath(rel) {
				target.ExplicitVoice = rel
				assignedAud = true
			}
		}
		s.Editor.SelectedSceneID = target.ID
	}
	book.FinalizeBook(b)
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return err.Error()
	}
	reloadActiveBook(s)
	if len(imported) == 1 {
		msg := tr(s, i18n.TrAddedMediaSinglePrefix) + imported[0]
		s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, historyScopeScene, "", s.Editor.SelectedSceneID)
		return msg
	}
	msg := trf(s, i18n.TrAddedMediaMultiFmt, len(imported))
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, historyScopeScene, "", s.Editor.SelectedSceneID)
	return msg
}
