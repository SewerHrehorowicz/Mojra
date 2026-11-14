//go:build !(js && wasm)

package actions

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
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
	formKeyTime            = "time"
	formKeyKind            = "kind"
	formKeyCharacter       = "character"
	formKeyX               = "x"
	formKeyY               = "y"
	formValDialogue        = "dialogue"
	formValParenthetical   = "parenthetical"
	formValAction          = "action"
	formValDialogueOff     = "dialogue_off"
	formValDialogueVO      = "dialogue_vo"
	formValTransitionCut   = "transition_cut"
	formValTransitionFade  = "transition_fade"
	formValSubslug         = "subslug"
	formValEND             = "END"
	formValDalej           = "Dalej"
	formValNext            = "Next"
	formValNextChToken     = "__next__"
	formValChTargetPref    = "chapter:"
	formValWybory          = "Wybór %d"
	formValLangPL          = "pl"
	historyScopeScene      = "scene"
	historyScopeGraph      = "graph"
	coalesceMajor          = "major"
	coalesceAutoSavePref   = "autosave:"
	coalesceSluglinePref   = "slugline:"
	coalesceTextPref       = "text:"

	msgSceneNotFound         = "Nie znaleziono sceny"
	msgSavedInFilePrefix     = "Zapisano zmiany w pliku "
	msgEditScenePrefix       = "Edycja sceny: "
	msgPromotedPrimaryPrefix = "Ustawiono główną ścieżkę (primary): "
	msgChoiceAddedPrefix     = "Dodano nowy wybór do sceny: "
	msgChoiceRemovedPrefix   = "Usunięto wybór ze sceny: "
	msgChoiceReorderedPrefix = "Zmieniono kolejność wyborów: "
	msgChoiceBranchedPrefix  = "Utworzono scenę docelową: "
	msgSceneInsertedPrefix   = "Wstawiono nową scenę: "
	msgSceneCreatedAtPrefix  = "Dodano nową scenę: "
	msgSceneDeletedFmt       = "Usunięto scenę: %s"
	msgScenesDeletedMultiFmt = "Usunięto zaznaczone sceny (%d)"
	msgSluglineUpdatedPrefix = "Zaktualizowano nagłówek sceny: "
	msgSpBlockInsertedPrefix = "Wstawiono element scenariusza w scenie: "
	msgDefaultBookTitle      = "Nowa Opowieść"
	msgDefaultBookAuthor     = "Autor"
	slugDefaultPrefixPL      = "WN."
	slugDefaultLocPL         = "LOKACJA"
	slugDefaultLocEN         = "LOCATION"
	slugDefaultTimePL        = "DZIEŃ"
	slugDefaultTimeEN        = "DAY"
	slugFormatStr            = "%s %s - %s"
	snippetDialogueLine      = "\nKwestia dialogowa..."
	snippetDialogueFull      = "\n\nPOSTAĆ\nKwestia dialogowa..."
	snippetParen             = "\n(cicho)\n"
	snippetAction            = "\n\nOpis akcji..."
	snippetOffPL             = "\n\nPOSTAĆ (OFF)\nKwestia spoza kadru..."
	snippetOffEN             = "\n\nPOSTAĆ (O.S.)\nKwestia spoza kadru..."
	snippetVO                = "\n\nPOSTAĆ (V.O.)\nKwestia narratora..."
	snippetCutPL             = "\n\n> CIĘCIE DO:\n"
	snippetCutEN             = "\n\n> CUT TO:\n"
	snippetFadePL            = "\n\n> ŚCIEMNIENIE.\n"
	snippetFadeEN            = "\n\n> FADE OUT.\n"
	snippetSubslugPL         = "\n\nWN. LOKACJA - CIĄG DALSZY\n"
	snippetSubslugEN         = "\n\nINT. LOCATION - CONTINUOUS\n"
)

func splitInlineChoiceLabelAndTarget(raw string) (label string, target string, hasArrow bool) {
	cleaned := book.SanitizePlainText(raw)
	if idx := strings.LastIndex(cleaned, "->"); idx >= 0 {
		if idx == 0 || cleaned[idx-1] != '\\' {
			return strings.TrimSpace(cleaned[:idx]), strings.TrimSpace(cleaned[idx+2:]), true
		}
	}
	return strings.TrimSpace(cleaned), "", false
}

func resolveEditorSceneFromRequest(s *state.AppState, r *http.Request, fallbackActive bool) (string, *book.Scene) {
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

func selectStudioScene(s *state.AppState, r *http.Request) string {
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
	target := b.FindScene(strings.TrimSpace(r.FormValue(formKeyScene)))
	if target == nil {
		return msgSceneNotFound
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	for _, sc := range b.Scenes {
		if len(sc.Choices) >= 2 {
			sc.Choices = book.MaterializeChoicesForEdit(b, sc)
		}
	}
	if !book.PromoteSceneToPrimaryPath(b, target.ID) {
		r.Form.Set(formFieldSilent, formValTrue)
		return ""
	}
	return commitScenesMutation(s, msgPromotedPrimaryPrefix+target.DisplayName(), historyScopeGraph, "")
}

func hasSceneTextModalChanges(s *state.AppState, curSc *book.Scene, r *http.Request) bool {
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
				newLbl = fmt.Sprintf(formValWybory, i+1)
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

func applySceneFormFields(s *state.AppState, target *book.Scene, r *http.Request) {
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
	if _, hasChapter := r.Form[formKeyChapter]; hasChapter && act != CreateSceneAt && act != CreateChapterGroup && act != SelectStudioChapter {
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

	applySceneFormChoices(b, target, r)

	if book.RebindRenamedScene(b, target, oldID, oldName, oldExplicitID) && s.Player.Save != nil {
		if strings.EqualFold(s.Player.Save.CurrentSceneID, oldID) || strings.EqualFold(s.Player.Save.CurrentSceneID, oldName) || strings.EqualFold(s.Player.Save.CurrentSceneID, oldExplicitID) {
			s.Player.Save.CurrentSceneID = target.ID
		}
	}
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

func applySceneFormChoices(b *book.Book, target *book.Scene, r *http.Request) {
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
				newLbl = fmt.Sprintf(formValWybory, i+1)
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
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, historyScopeGraph, "", s.Editor.SelectedSceneID)
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
		return msgSceneNotFound
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	book.AddChoiceToScene(b, target)
	return commitTargetSceneChoicesMutation(s, target, msgChoiceAddedPrefix)
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
		return msgSceneNotFound
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyChoiceIdx)))
	if err != nil {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	book.RemoveChoiceFromScene(b, target, choiceIdx)
	return commitTargetSceneChoicesMutation(s, target, msgChoiceRemovedPrefix)
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
		return msgSceneNotFound
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	book.RemoveChoiceFromScene(b, target, choiceIdx)
	return commitTargetSceneChoicesMutation(s, target, msgChoiceRemovedPrefix)
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
		return msgSceneNotFound
	}
	fromIdx, err1 := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyFromIdx)))
	toIdx, err2 := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyToIdx)))
	if err1 != nil || err2 != nil || fromIdx == toIdx {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	if !book.ReorderSceneChoice(b, target, fromIdx, toIdx) {
		return ""
	}
	return commitTargetSceneChoicesMutation(s, target, msgChoiceReorderedPrefix)
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
		return msgSceneNotFound
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyChoiceIdx)))
	if err != nil {
		choiceIdx = -1
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	newScene, brErr := book.BranchChoiceToNewScene(b, target, choiceIdx)
	if brErr != nil {
		return brErr.Error()
	}
	if newScene == nil {
		return ""
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
	msg := msgChoiceBranchedPrefix + newScene.Name
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, historyScopeGraph, "", s.Editor.SelectedSceneID)
	return msg
}

func autoSaveSceneAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return ""
	}
	if msg := saveSceneFromFormWithMode(s, r, true); msg != "" && !strings.HasPrefix(msg, msgSavedInFilePrefix) && msg != msgSceneNotFound {
		return msg
	}
	return ""
}

func saveSceneAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	if msg := saveSceneFromForm(s, r); msg != "" && !strings.HasPrefix(msg, msgSavedInFilePrefix) && msg != msgSceneNotFound {
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
	scID := strings.TrimSpace(r.FormValue(formKeySceneID))
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
		return msgSceneNotFound
	}
	prevTargetID := target.ID
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	book.FinalizeBook(b)
	newID, targetLabel := target.ID, target.DisplayName()
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
	if r != nil && r.FormValue(formFieldAction) == UndoEditor {
		coalesceKey = coalesceMajor
	} else if newID != "" {
		coalesceKey = coalesceAutoSavePref + newID
	}
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msgEditScenePrefix+targetLabel, historyScopeScene, coalesceKey, s.Editor.SelectedSceneID)
	return msgSavedInFilePrefix + target.SourceFile
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
	toast := msgSceneInsertedPrefix + inserted.Name
	s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, historyScopeGraph, "", s.Editor.SelectedSceneID)
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
	parentID := strings.TrimSpace(r.FormValue(formKeyParent))
	childID := strings.TrimSpace(r.FormValue(formKeyChild))
	if parentID == "" && len(b.Scenes) > 0 {
		parentID = b.Scenes[len(b.Scenes)-1].ID
	}
	inserted, err := book.InsertSceneBetween(b, parentID, childID, book.NextUniqueSceneName(b))
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
	targetID := strings.TrimSpace(r.FormValue(formKeyTarget))
	if targetID == "" {
		targetID = strings.TrimSpace(r.FormValue(formKeyBefore))
	}
	if targetID == "" {
		targetID = strings.TrimSpace(r.FormValue(formKeyScene))
	}
	inserted, err := book.InsertSceneBefore(b, targetID, book.NextUniqueSceneName(b))
	return finishInsertedScene(s, inserted, err)
}

func createSceneAt(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	if s.Editor.ActiveBook == nil && !s.Config.StandaloneMode {
		if createdBook, err := book.CreateNewBook(s.Config.BooksDir, msgDefaultBookTitle, msgDefaultBookAuthor, false); err == nil && createdBook != nil {
			s.Library.Put(createdBook)
			s.Editor.SetBook(createdBook, s.Settings)
			_ = s.SaveSettings()
		}
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	x, errX := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyX)))
	y, errY := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyY)))
	created, err := book.CreateStandaloneSceneInChapter(b, book.NextUniqueSceneName(b), strings.TrimSpace(r.FormValue(formKeyChapter)), x, y, errX == nil && errY == nil)
	if err != nil {
		return err.Error()
	}
	if created == nil {
		return ""
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
	toast := msgSceneCreatedAtPrefix + created.Name
	s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, historyScopeGraph, "", s.Editor.SelectedSceneID)
	return toast
}

func deleteSceneFromForm(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	targetID := strings.TrimSpace(r.FormValue(formKeyScene))
	if targetID == "" {
		targetID = s.Editor.SelectedSceneID
	}
	return deleteSceneByID(s, targetID)
}

func deleteScenesFromForm(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	rawIDs := strings.TrimSpace(r.FormValue(formKeyScenes))
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue(formKeySceneIDs))
	}
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue(formKeyScene))
	}
	return deleteScenesByIDs(s, rawIDs)
}

func deleteSceneByID(s *state.AppState, sceneID string) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	deleted, err := book.DeleteScenesByIDs(b, sceneID, false)
	if err != nil {
		return err.Error()
	}
	if len(deleted) == 0 {
		return msgSceneNotFound
	}
	return finalizeScenesDeletion(s, deleted)
}

func deleteScenesByIDs(s *state.AppState, rawIDs string) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	deleted, err := book.DeleteScenesByIDs(b, rawIDs, true)
	if err != nil {
		return err.Error()
	}
	if len(deleted) == 0 {
		return ""
	}
	return finalizeScenesDeletion(s, deleted)
}

func finalizeScenesDeletion(s *state.AppState, deleted []*book.Scene) string {
	reloadActiveBook(s)
	b := s.Editor.ActiveBook
	s.Editor.SelectedSceneID = b.StartSceneID
	if b.FindScene(s.Editor.SelectedSceneID) == nil && len(b.Scenes) > 0 {
		s.Editor.SelectedSceneID = b.Scenes[0].ID
	}
	if s.Editor.SceneText.IsOpen() && b.FindScene(s.Editor.SceneText.SceneID) == nil {
		s.Editor.SceneText.Close()
	}
	msg := fmt.Sprintf(msgScenesDeletedMultiFmt, len(deleted))
	if len(deleted) == 1 {
		msg = fmt.Sprintf(msgSceneDeletedFmt, deleted[0].DisplayName())
	}
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, historyScopeGraph, "", s.Editor.SelectedSceneID)
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
			prefix = slugDefaultPrefixPL
		}
	}
	loc, tod := info.Location, info.TimeOfDay
	if loc == "" {
		if b.NormalizedStandard() == formValLangPL {
			loc = slugDefaultLocPL
		} else {
			loc = slugDefaultLocEN
		}
	}
	if tod == "" {
		if b.NormalizedStandard() == formValLangPL {
			tod = slugDefaultTimePL
		} else {
			tod = slugDefaultTimeEN
		}
	}
	if newPrefix := strings.ToUpper(strings.TrimSpace(r.FormValue(formKeyPrefix))); newPrefix != "" {
		prefix = newPrefix
	}
	if newTime := strings.ToUpper(strings.TrimSpace(r.FormValue(formKeyTime))); newTime != "" {
		tod = newTime
	}
	target.Slugline = fmt.Sprintf(slugFormatStr, prefix, loc, tod)
	return commitScenesMutation(s, msgSluglineUpdatedPrefix+target.Slugline, historyScopeScene, coalesceSluglinePref+target.ID)
}

func insertScreenplayBlockAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	target := prepareScreenplaySceneEdit(s, r)
	if target == nil {
		return ""
	}
	isPL := s.Editor.ActiveBook.NormalizedStandard() == formValLangPL
	kind := strings.ToLower(strings.TrimSpace(r.FormValue(formKeyKind)))
	charName := strings.ToUpper(strings.TrimSpace(book.SanitizePlainText(r.FormValue(formKeyCharacter))))
	var snippet string
	switch {
	case charName != "":
		snippet = "\n\n" + charName + snippetDialogueLine
	case kind == formValDialogue:
		snippet = snippetDialogueFull
	case kind == formValParenthetical:
		snippet = snippetParen
	case kind == formValAction:
		snippet = snippetAction
	case kind == formValDialogueOff:
		if isPL {
			snippet = snippetOffPL
		} else {
			snippet = snippetOffEN
		}
	case kind == formValDialogueVO:
		snippet = snippetVO
	case kind == formValTransitionCut:
		if isPL {
			snippet = snippetCutPL
		} else {
			snippet = snippetCutEN
		}
	case kind == formValTransitionFade:
		if isPL {
			snippet = snippetFadePL
		} else {
			snippet = snippetFadeEN
		}
	case kind == formValSubslug:
		if isPL {
			snippet = snippetSubslugPL
		} else {
			snippet = snippetSubslugEN
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
	return commitScenesMutation(s, msgSpBlockInsertedPrefix+target.DisplayName(), historyScopeScene, coalesceTextPref+target.ID)
}
