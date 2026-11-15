//go:build !(js && wasm)

package actions

import (
	"fmt"
	"path/filepath"
	"strings"

	"mojra/domain/book/model"
)

// FinalizeBook resolves scene IDs (including explicit id: headers), implicit "Next" transitions,
// choice targets (by scene id or scene name), start scene, chapters, and timeline row/column positions.
func FinalizeBook(b *Book) {
	if len(b.Scenes) == 0 {
		return
	}

	// Preserve existing chapter metadata (ExplicitID, ID, SourceFile) before rebuilding chapters
	prevChapterMeta := make(map[string]Chapter, len(b.Chapters))
	for _, ch := range b.Chapters {
		if k := strings.ToLower(strings.TrimSpace(ch.Name)); k != "" {
			prevChapterMeta[k] = ch
		}
		if k := strings.ToLower(strings.TrimSpace(ch.ExplicitID)); k != "" {
			prevChapterMeta[k] = ch
		}
		if k := strings.ToLower(strings.TrimSpace(ch.ID)); k != "" {
			prevChapterMeta[k] = ch
		}
	}

	if !b.Narrated {
		b.AudioFiles = nil
		for _, sc := range b.Scenes {
			sc.ExplicitVoice = ""
			sc.VoiceTrimStart = 0
			sc.VoiceTrimEnd = 0
			sc.SectionVoices = nil
			sc.AudioFile = ""
			sc.AudioURL = ""
		}
	}
	if b.IsScreenplay() {
		b.Imageless = true
		b.ImageFiles = nil
		for _, sc := range b.Scenes {
			sc.ExplicitImage = ""
			sc.ImageFile = ""
			sc.ImageURL = ""
		}
	}

	usedIDs := make(map[string]bool, len(b.Scenes))
	for _, sc := range b.Scenes {
		sc.Warnings = nil
		if sc.ExplicitID != "" {
			cleanID := SanitizeSceneID(sc.ExplicitID)
			sc.ExplicitID = cleanID
			if cleanID != "" {
				lowerID := strings.ToLower(cleanID)
				if usedIDs[lowerID] {
					b.Warnings = append(b.Warnings, fmt.Sprintf("Duplikat id sceny „%s” w scenie „%s”", cleanID, sc.DisplayName()))
					for suffix := 2; ; suffix++ {
						cand := fmt.Sprintf("%s_%d", cleanID, suffix)
						if !usedIDs[strings.ToLower(cand)] && !b.IsSceneIDRetired(cand) {
							cleanID = cand
							lowerID = strings.ToLower(cand)
							sc.ExplicitID = cleanID
							break
						}
					}
				}
				usedIDs[lowerID] = true
				if sc.ID != "" && !strings.EqualFold(sc.ID, cleanID) && ParseAutoSceneSeq(sc.ID) > 0 {
					b.RetireSceneID(sc.ID)
				}
				sc.ID = cleanID
				b.UnbindAutoSceneName(sc.Name)
				if ParseAutoSceneSeq(cleanID) > 0 {
					b.RetireSceneID(cleanID)
				}
			}
		}
	}

	defaultChName := b.DefaultChapterName()

	hadPersistedAutoIDs := b.HasPersistedAutoIDs
	nextAuto := 1
	for i, sc := range b.Scenes {
		if sc.ExplicitID == "" {
			if strings.TrimSpace(sc.Name) == "" {
				sc.Name = fmt.Sprintf("Scena %d", i+1)
			}
			assigned := ""
			if bound := b.LookupBoundAutoSceneID(sc.Name); bound != "" && !usedIDs[strings.ToLower(bound)] {
				assigned = bound
			} else if sc.ID != "" && !usedIDs[strings.ToLower(sc.ID)] && (!hadPersistedAutoIDs || !b.IsSceneIDRetired(sc.ID)) {
				assigned = strings.ToLower(sc.ID)
			} else if !hadPersistedAutoIDs {
				for maxTries := len(usedIDs) + len(b.Scenes) + len(b.RetiredSceneIDs) + 32; maxTries > 0; maxTries-- {
					cand := fmt.Sprintf("s%d", nextAuto)
					nextAuto++
					if !usedIDs[strings.ToLower(cand)] && !b.IsSceneIDRetired(cand) {
						assigned = cand
						break
					}
				}
			}
			if assigned == "" {
				assigned = b.NextUniqueAutoSceneID(usedIDs)
			}
			sc.ID = assigned
			usedIDs[strings.ToLower(assigned)] = true
			b.BindAutoSceneID(sc.Name, assigned)
		}
		sc.OrderIndex = i
		if sc.Chapter == "" {
			if sc.ChapterID != "" {
				sc.Chapter = sc.ChapterID
			} else if sc.SourceFile != "" && !strings.EqualFold(filepath.Base(sc.SourceFile), DefaultStoryFile) && !strings.EqualFold(filepath.Base(sc.SourceFile), BookDataFileName) {
				stem := strings.TrimSuffix(filepath.Base(sc.SourceFile), filepath.Ext(sc.SourceFile))
				sc.Chapter = SanitizeSceneID(stem)
			} else {
				sc.Chapter = defaultChName
			}
		}
		if sc.ChapterID == "" {
			if meta, ok := prevChapterMeta[strings.ToLower(strings.TrimSpace(sc.Chapter))]; ok && meta.ExplicitID != "" {
				sc.ChapterID = meta.ExplicitID
			}
		}
		if b.IsScreenplay() && strings.TrimSpace(sc.Slugline) == "" && strings.TrimSpace(sc.Text) != "" {
			if extracted, rest, ok := ExtractLeadingSlugline(sc.Text); ok {
				sc.Slugline = extracted
				sc.Text = rest
			}
		}
		if !b.DeferProseHTML {
			sc.EnsureHTML()
		} else if sc.HTML != "" && sc.HTMLSourceText != sc.Text {
			sc.HTML = ""
			sc.HTMLSourceText = ""
		}
	}

	sceneByID := make(map[string]*Scene, len(b.Scenes))
	sceneByName := make(map[string]*Scene, len(b.Scenes))
	for _, sc := range b.Scenes {
		sceneByID[strings.ToLower(sc.ID)] = sc
		if sc.ExplicitID != "" {
			sceneByID[strings.ToLower(sc.ExplicitID)] = sc
		}
		if norm := NormalizeLookupKey(sc.Name); norm != "" {
			if _, exists := sceneByName[norm]; !exists {
				sceneByName[norm] = sc
			}
		}
	}

	// Resolve StartSceneID (can match scene ID, explicit ID, or scene Name)
	startCandidate := strings.TrimSpace(b.RawStart)
	if startCandidate == "" {
		startCandidate = strings.TrimSpace(b.StartSceneID)
	}
	if startCandidate == "" {
		b.StartSceneID = b.Scenes[0].ID
	} else if sc, ok := sceneByID[strings.ToLower(startCandidate)]; ok {
		b.StartSceneID = sc.ID
	} else if sc, ok := sceneByName[NormalizeLookupKey(startCandidate)]; ok {
		b.StartSceneID = sc.ID
	} else {
		b.Warnings = append(b.Warnings, fmt.Sprintf("book.playtxt: scena startowa „%s” nie istnieje (użyto pierwszej sceny „%s”)", startCandidate, b.Scenes[0].DisplayName()))
		b.StartSceneID = b.Scenes[0].ID
	}

	// Build initial chapter index and assign each chapter's single entry point (StartSceneID)
	b.Chapters = nil
	for _, sc := range b.Scenes {
		b.AppendSceneToChaptersFromMeta(sc, prevChapterMeta)
	}
	b.ResolveChapterEntryPoints(prevChapterMeta, func(sid string) *Scene {
		return sceneByID[strings.ToLower(sid)]
	}, true)

	rootChKey := b.RootChapterKey()
	chapterEdges := make(map[string][]string, len(b.Chapters))
	tryLinkChapters := func(sc *Scene, ch *Choice, fromCh, targetCh *Chapter) bool {
		if targetCh == nil {
			return false
		}
		fromKey := ""
		if fromCh != nil {
			fromKey = strings.ToLower(fromCh.ReferenceKey())
		} else {
			fromKey = strings.ToLower(SanitizeSceneID(sc.Chapter))
		}
		toKey := strings.ToLower(targetCh.ReferenceKey())
		if fromKey == toKey || (fromCh != nil && strings.EqualFold(fromCh.Name, targetCh.Name)) {
			ch.TargetSceneID = ""
			ch.TargetChapterID = ""
			ch.IsChapterLink = false
			warnMsg := fmt.Sprintf("Scena „%s”: przejście rozdziału nie może wskazywać na ten sam rozdział „%s”", sc.DisplayName(), targetCh.DisplayName())
			sc.Warnings = append(sc.Warnings, warnMsg)
			b.Warnings = append(b.Warnings, warnMsg)
			return true
		}
		if (rootChKey != "" && toKey == rootChKey) || model.ChapterCanReach(chapterEdges, toKey, fromKey) {
			ch.TargetSceneID = ""
			ch.TargetChapterID = ""
			ch.IsChapterLink = false
			warnMsg := fmt.Sprintf("Scena „%s”: przejście do rozdziału „%s” jest niedozwolone (zmiana rozdziału jest jednokierunkowa)", sc.DisplayName(), targetCh.DisplayName())
			sc.Warnings = append(sc.Warnings, warnMsg)
			b.Warnings = append(b.Warnings, warnMsg)
			return true
		}
		chapterEdges[fromKey] = append(chapterEdges[fromKey], toKey)
		ch.IsChapterLink = true
		ch.TargetChapterID = targetCh.ReferenceKey()
		ch.Target = "chapter:" + targetCh.ReferenceKey()
		ch.TargetSceneID = targetCh.StartSceneID
		if len(sc.Choices) == 1 && sc.ExplicitNext != "" {
			sc.ExplicitNext = ch.Target
		}
		return true
	}

	// Resolve choices and enforce that a single transition is always implicit "Dalej" (never a custom choice title)
	for i, sc := range b.Scenes {
		var keptChoices []Choice
		if !sc.ExplicitEnd {
			for _, ch := range sc.Choices {
				if !ch.IsImplicit || (!strings.EqualFold(ch.Target, "Next") && !strings.EqualFold(ch.Target, "Dalej") && strings.TrimSpace(ch.Target) != "") {
					keptChoices = append(keptChoices, ch)
				}
			}
		}
		sc.Choices = keptChoices

		sameChapterNext := i+1 < len(b.Scenes) &&
			strings.EqualFold(strings.TrimSpace(sc.Chapter), strings.TrimSpace(b.Scenes[i+1].Chapter)) &&
			(sc.SourceFile == "" || b.Scenes[i+1].SourceFile == "" || strings.EqualFold(sc.SourceFile, b.Scenes[i+1].SourceFile))
		if !sc.ExplicitEnd && len(sc.Choices) == 0 && sameChapterNext {
			sc.Choices = append(sc.Choices, Choice{
				Label:         "Dalej",
				Target:        "Next",
				TargetSceneID: b.Scenes[i+1].ID,
				IsImplicit:    true,
			})
		}

		if len(sc.Choices) == 1 {
			sc.Choices[0].Label = "Dalej"
			sc.Choices[0].IsImplicit = true
		} else if len(sc.Choices) >= 2 {
			for j := range sc.Choices {
				sc.Choices[j].IsImplicit = false
			}
		}

		fromCh := b.FindChapter(sc.Chapter)
		for j := range sc.Choices {
			ch := &sc.Choices[j]
			ch.IsChapterLink = false
			ch.TargetChapterID = ""
			tgt := strings.TrimSpace(ch.Target)
			if strings.EqualFold(tgt, "END") || strings.EqualFold(tgt, "KONIEC") {
				ch.IsEnding = true
				ch.TargetSceneID = "END"
				continue
			}
			if strings.EqualFold(tgt, "NEXT") || strings.EqualFold(tgt, "DALEJ") {
				if sameChapterNext {
					ch.TargetSceneID = b.Scenes[i+1].ID
				} else {
					ch.IsEnding = true
					ch.TargetSceneID = "END"
				}
				continue
			}
			if chRef, isChTarget := ParseChapterTarget(tgt); isChTarget {
				targetCh := b.FindChapter(chRef)
				if targetCh == nil {
					ch.TargetSceneID = ""
					warnMsg := fmt.Sprintf("Scena „%s”: wybór „%s” wskazuje na nieistniejący rozdział „%s”", sc.DisplayName(), ch.Label, chRef)
					sc.Warnings = append(sc.Warnings, warnMsg)
					b.Warnings = append(b.Warnings, warnMsg)
					continue
				}
				tryLinkChapters(sc, ch, fromCh, targetCh)
				continue
			}
			var targetSc *Scene
			if found, ok := sceneByID[strings.ToLower(tgt)]; ok {
				targetSc = found
			} else if found, ok := sceneByName[NormalizeLookupKey(tgt)]; ok {
				targetSc = found
			}
			if targetSc != nil {
				if strings.EqualFold(strings.TrimSpace(targetSc.Chapter), strings.TrimSpace(sc.Chapter)) {
					ch.TargetSceneID = targetSc.ID
					continue
				}
				targetCh := b.FindChapter(targetSc.Chapter)
				if targetCh != nil && strings.EqualFold(targetSc.ID, targetCh.StartSceneID) {
					tryLinkChapters(sc, ch, fromCh, targetCh)
					continue
				}
				ch.TargetSceneID = ""
				chDisplay := targetSc.Chapter
				if targetCh != nil {
					chDisplay = targetCh.DisplayName()
				}
				warnMsg := fmt.Sprintf("Scena „%s”: przejście między rozdziałami może prowadzić wyłącznie do początku rozdziału „%s” (nie do wewnętrznej sceny „%s”)", sc.DisplayName(), chDisplay, targetSc.DisplayName())
				sc.Warnings = append(sc.Warnings, warnMsg)
				b.Warnings = append(b.Warnings, warnMsg)
				continue
			}
			if targetCh := b.FindChapter(tgt); targetCh != nil {
				tryLinkChapters(sc, ch, fromCh, targetCh)
				continue
			}
			ch.TargetSceneID = ""
			warnMsg := fmt.Sprintf("Scena „%s”: wybór „%s” wskazuje na nieistniejącą scenę lub id „%s”", sc.DisplayName(), ch.Label, ch.Target)
			sc.Warnings = append(sc.Warnings, warnMsg)
			b.Warnings = append(b.Warnings, warnMsg)
		}
	}

	// Order chapters (and multi-file scene groups) by graph reachability starting from StartSceneID
	// so disk filename order is completely irrelevant.
	b.HasPersistedAutoIDs = hadPersistedAutoIDs
	orderChaptersAndScenesByGraph(b, prevChapterMeta)
	b.HasPersistedAutoIDs = true

	if b.Narrated && len(b.AudioFiles) == 0 {
		b.AudioFiles = discoverBookAudioFilesForSections(b)
	}
	for i, sc := range b.Scenes {
		sc.OrderIndex = i
		NormalizeSceneInlineSectionVoices(sc)
		secs := ExtractSceneVoiceSections(sc, b)
		if HasConditionalVoiceDirectives(sc.Text) && b.Narrated {
			sc.AudioFile = ""
			sc.AudioURL = ""
			for _, sec := range secs {
				if sec.HasAudio && sc.AudioFile == "" {
					sc.AudioFile = sec.AudioFile
					sc.AudioURL = encodeMediaURL(b.ID, sec.AudioFile)
				}
			}
			appendSceneVoiceSectionWarnings(b, sc)
		}
		if !b.DeferProseHTML {
			sc.EnsureHTML()
		}
	}
	if b.IsScreenplay() {
		ValidateScreenplayGuards(b)
	}

	ComputeTimelineLayout(b)
}
