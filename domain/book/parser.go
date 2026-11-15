//go:build !(js && wasm)

package book

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ParseDataFile parses book-level metadata from book.playtxt.
func ParseDataFile(content string, b *Book) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for lineIdx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		idx := strings.IndexByte(trimmed, ':')
		if idx <= 0 {
			b.Warnings = append(b.Warnings, fmt.Sprintf("book.playtxt:%d: nieprawidłowy wiersz konfiguracji „%s” (oczekiwano klucz: wartość)", lineIdx+1, trimmed))
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
		val := SanitizePlainText(trimmed[idx+1:])
		switch key {
		case "title":
			if val != "" {
				b.Title = val
			}
		case "author":
			if val != "" {
				b.Author = val
			}
		case "license":
			if val != "" {
				b.License = val
			}
		case "description":
			if val != "" {
				b.Description = val
			}
		case "start":
			if val != "" {
				b.StartSceneID = val
				b.RawStart = val
			}
		case "cover":
			if val != "" {
				safeRel := SanitizeRelativeMediaPath(val)
				if safeRel != "" {
					b.CoverFile = safeRel
				} else {
					b.Warnings = append(b.Warnings, fmt.Sprintf("book.playtxt:%d: nieprawidłowa ścieżka okładki „%s”", lineIdx+1, val))
				}
			}
		case "narrated":
			vLower := strings.ToLower(strings.TrimSpace(val))
			b.Narrated = !b.IsScreenplay() && (vLower == "true" || vLower == "1")
		case "imageless":
			vLower := strings.ToLower(strings.TrimSpace(val))
			b.Imageless = b.IsScreenplay() || vLower == "true" || vLower == "1"
		case "mode":
			if val != "" {
				b.Mode = NormalizeBookMode(val)
				if b.IsScreenplay() {
					b.Narrated = false
					b.Imageless = true
				}
			}
		case "standard":
			if val != "" {
				b.ScriptStandard = NormalizeScriptStandard(val)
			}
		case "enum":
			if ed, ok := ParseEnumDeclaration(val); ok {
				b.Enums = append(b.Enums, ed)
			}
		case "var":
			if vd, ok := ParseVariableDeclaration(val, b.Enums); ok {
				b.Variables = append(b.Variables, vd)
			}
		case "export_dir":
			// Ignored in book.playtxt for security: export target paths are stored in local saves/settings.ini, never trusted from imported books.
		case "auto_delay":
			// Accepted book-level hint for inter-scene pause; runtime delay is configured via saves/settings.ini.
		default:
			b.Warnings = append(b.Warnings, fmt.Sprintf("book.playtxt:%d: nieznany klucz „%s”", lineIdx+1, key))
		}
	}
	ResolveBookVariableTypes(b)
}

// ParseStoryFile parses chapters, scenes, optional id:/image:/voice: headers, prose, and choices from a .playtxt file.
// If a chapter header is followed directly by an id: header (before any blank line), that id belongs to the chapter.
// If a chapter header is followed directly by prose without an explicit "scene:" header,
// the chapter name is automatically used as the scene name.
func ParseStoryFile(content string, sourceFileName string, b *Book) {
	rawLines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	baseFile := filepath.Base(sourceFileName)
	fileStem := strings.TrimSuffix(baseFile, filepath.Ext(baseFile))
	fileChapterID := ""
	if !strings.EqualFold(baseFile, DefaultStoryFile) && !strings.EqualFold(baseFile, BookDataFileName) && fileStem != "" {
		fileChapterID = SanitizeSceneID(fileStem)
	}

	currentChapter := ""
	currentChapterExplicitID := ""
	if fileChapterID != "" {
		currentChapter = fileChapterID
	} else if len(b.Chapters) > 0 {
		lastCh := b.Chapters[len(b.Chapters)-1]
		currentChapter = lastCh.Name
		currentChapterExplicitID = lastCh.ExplicitID
	}

	var currentScene *Scene
	var proseLines []string
	pendingSceneID := ""
	pendingImage := ""
	pendingVoice := ""
	pendingVoiceTrimStart := 0.0
	pendingVoiceTrimEnd := 0.0
	pendingSlugline := ""
	pendingSynopsis := ""
	pendingEffects := ""
	sawSceneHeader := false
	headerBlockEnded := false
	inChapterHeaderBlock := false
	inExitChoiceBlock := false

	defaultChName := b.DefaultChapterName()

	trimTrailingSceneSeparators := func() {
		for len(proseLines) > 0 {
			last := strings.TrimSpace(proseLines[len(proseLines)-1])
			if last == "" || last == "---" {
				proseLines = proseLines[:len(proseLines)-1]
			} else {
				break
			}
		}
	}

	flushScene := func() {
		if currentScene == nil {
			return
		}
		if strings.TrimSpace(currentScene.Name) == "" && strings.TrimSpace(currentScene.ExplicitID) == "" {
			currentScene.Name = fmt.Sprintf("Scena %d", len(b.Scenes)+1)
		}
		trimTrailingSceneSeparators()
		rawProse := strings.TrimSpace(strings.Join(proseLines, "\n"))
		currentScene.Text = SanitizePlainText(rawProse)
		if b.IsScreenplay() && strings.TrimSpace(currentScene.Slugline) == "" && currentScene.Text != "" {
			if extracted, rest, ok := ExtractLeadingSlugline(currentScene.Text); ok {
				currentScene.Slugline = extracted
				currentScene.Text = rest
			}
		}
		NormalizeSceneInlineSectionVoices(currentScene)
		if !b.deferProseHTML {
			currentScene.EnsureHTML()
		}
		currentScene.OrderIndex = len(b.Scenes)
		b.Scenes = append(b.Scenes, currentScene)
		b.addSceneToChapterWithMeta(currentScene.Chapter, currentScene.ChapterID, currentScene.SourceFile, currentScene.ID)
		currentScene = nil
		proseLines = nil
		sawSceneHeader = false
		headerBlockEnded = false
		inExitChoiceBlock = false
	}

	ensureCurrentChapter := func() {
		if currentChapter == "" {
			if currentChapterExplicitID != "" {
				currentChapter = currentChapterExplicitID
			} else if fileChapterID != "" {
				currentChapter = fileChapterID
			} else {
				currentChapter = defaultChName
			}
		}
		b.ensureChapterWithMeta(currentChapter, fileChapterID, currentChapterExplicitID, sourceFileName)
	}

	startScene := func(sceneName string, explicitSceneHeader bool) {
		flushScene()
		inChapterHeaderBlock = false
		cleanName := SanitizePlainText(sceneName)
		ensureCurrentChapter()
		explicitID := pendingSceneID
		pendingSceneID = ""
		sceneID := explicitID
		if sceneID == "" {
			if bound := b.LookupBoundAutoSceneID(cleanName); bound != "" {
				sceneID = bound
			} else if !b.HasPersistedAutoIDs {
				sceneID = fmt.Sprintf("s%d", len(b.Scenes)+1)
			}
		}
		imgVal := pendingImage
		pendingImage = ""
		voiceVal := pendingVoice
		pendingVoice = ""
		voiceTrimStartVal := pendingVoiceTrimStart
		voiceTrimEndVal := pendingVoiceTrimEnd
		pendingVoiceTrimStart = 0
		pendingVoiceTrimEnd = 0
		slugVal := pendingSlugline
		pendingSlugline = ""
		synVal := pendingSynopsis
		pendingSynopsis = ""
		effVal := pendingEffects
		pendingEffects = ""

		currentScene = &Scene{
			ID:             sceneID,
			ExplicitID:     explicitID,
			Name:           cleanName,
			Slugline:       slugVal,
			Synopsis:       synVal,
			Effects:        effVal,
			Chapter:        currentChapter,
			ChapterID:      currentChapterExplicitID,
			ExplicitImage:  imgVal,
			ExplicitVoice:  voiceVal,
			VoiceTrimStart: voiceTrimStartVal,
			VoiceTrimEnd:   voiceTrimEndVal,
			SourceFile:     sourceFileName,
		}
		sawSceneHeader = explicitSceneHeader
		headerBlockEnded = false
		inExitChoiceBlock = false
	}

	ensureCurrentSceneStarted := func() {
		if currentScene != nil {
			return
		}
		ensureCurrentChapter()
		startScene(currentChapter, false)
	}

	for lineIdx, line := range rawLines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Optional "---" scene separator (at top of scene or after choices)
		if trimmed == "---" && (currentScene == nil || len(proseLines) == 0 || inExitChoiceBlock || (len(currentScene.Choices) > 0 && !currentScene.Choices[0].IsImplicit)) {
			inExitChoiceBlock = false
			continue
		}

		// Check structural headers: chapter:, scene:, id:, slugline:, synopsis:, image:, voice:, exit:, label:, if:, set:, end:, text:
		if key, val, ok := parseStructuralHeader(trimmed); ok {
			switch key {
			case "chapter", "act":
				inExitChoiceBlock = false
				flushScene()
				rawChVal := SanitizePlainText(val)
				titleVal, hasTitle := headerBlockHasTitle(rawLines, lineIdx)
				hasID := headerBlockHasID(rawLines, lineIdx)
				if hasTitle {
					candID := SanitizeSceneID(rawChVal)
					currentChapterExplicitID = candID
					currentChapter = SanitizePlainText(titleVal)
					if currentChapter == "" {
						currentChapter = candID
					}
					if currentChapter != "" {
						b.ensureChapterWithMeta(currentChapter, candID, candID, sourceFileName)
					}
				} else if !hasID && rawChVal != "" && SanitizeSceneID(rawChVal) == rawChVal {
					currentChapterExplicitID = rawChVal
					currentChapter = rawChVal
					b.ensureChapterWithMeta(currentChapter, rawChVal, rawChVal, sourceFileName)
				} else {
					currentChapter = rawChVal
					currentChapterExplicitID = ""
					if currentChapter != "" {
						b.ensureChapterWithMeta(currentChapter, fileChapterID, "", sourceFileName)
					}
				}
				inChapterHeaderBlock = true
				continue

			case "scene":
				inChapterHeaderBlock = false
				inExitChoiceBlock = false
				rawScVal := SanitizePlainText(val)
				titleVal, hasTitle := headerBlockHasTitle(rawLines, lineIdx)
				hasID := headerBlockHasID(rawLines, lineIdx)
				if hasTitle {
					candID := SanitizeSceneID(rawScVal)
					if currentScene != nil && !sawSceneHeader && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
						if candID != "" {
							currentScene.ExplicitID = candID
							currentScene.ID = candID
						}
						currentScene.Name = SanitizePlainText(titleVal)
						sawSceneHeader = true
					} else {
						if candID != "" {
							pendingSceneID = candID
						}
						startScene(titleVal, true)
					}
				} else if !hasID && rawScVal != "" && SanitizeSceneID(rawScVal) == rawScVal {
					if currentScene != nil && !sawSceneHeader && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
						currentScene.ExplicitID = rawScVal
						currentScene.ID = rawScVal
						sawSceneHeader = true
					} else {
						pendingSceneID = rawScVal
						startScene("", true)
					}
				} else if currentScene != nil && !sawSceneHeader && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
					currentScene.Name = rawScVal
					sawSceneHeader = true
				} else {
					startScene(rawScVal, true)
				}
				continue

			case "title":
				inExitChoiceBlock = false
				cleanTitle := SanitizePlainText(val)
				if inChapterHeaderBlock && currentScene == nil {
					if cleanTitle != "" {
						if currentChapterExplicitID == "" && currentChapter != "" && SanitizeSceneID(currentChapter) == currentChapter {
							currentChapterExplicitID = currentChapter
						}
						currentChapter = cleanTitle
						cID := currentChapterExplicitID
						if cID == "" {
							cID = fileChapterID
						}
						b.ensureChapterWithMeta(currentChapter, cID, currentChapterExplicitID, sourceFileName)
					}
				} else if currentScene != nil && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
					if cleanTitle != "" {
						if currentScene.ExplicitID == "" && currentScene.Name != "" && SanitizeSceneID(currentScene.Name) == currentScene.Name {
							currentScene.ExplicitID = currentScene.Name
							currentScene.ID = currentScene.Name
						}
						currentScene.Name = cleanTitle
					}
				}
				continue

			case "slugline":
				inChapterHeaderBlock = false
				inExitChoiceBlock = false
				cleanSlug := strings.TrimSpace(SanitizePlainText(val))
				if currentScene != nil && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
					currentScene.Slugline = cleanSlug
				} else {
					pendingSlugline = cleanSlug
				}
				continue

			case "synopsis":
				inChapterHeaderBlock = false
				inExitChoiceBlock = false
				cleanSyn := strings.TrimSpace(SanitizePlainText(val))
				if currentScene != nil && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
					currentScene.Synopsis = cleanSyn
				} else {
					pendingSynopsis = cleanSyn
				}
				continue

			case "set":
				inChapterHeaderBlock = false
				cleanEff := CleanEffectsExpr(val)
				if inExitChoiceBlock && currentScene != nil && len(currentScene.Choices) > 0 {
					last := &currentScene.Choices[len(currentScene.Choices)-1]
					if last.Effects == "" {
						last.Effects = cleanEff
					} else if cleanEff != "" {
						last.Effects += "; " + cleanEff
					}
					continue
				}
				if currentScene != nil && !headerBlockEnded && len(proseLines) == 0 {
					if currentScene.Effects == "" {
						currentScene.Effects = cleanEff
					} else if cleanEff != "" {
						currentScene.Effects += "; " + cleanEff
					}
				} else if currentScene == nil {
					pendingEffects = cleanEff
				} else if len(currentScene.Choices) > 0 {
					last := &currentScene.Choices[len(currentScene.Choices)-1]
					if last.Effects == "" {
						last.Effects = cleanEff
					} else if cleanEff != "" {
						last.Effects += "; " + cleanEff
					}
				}
				continue

			case "id":
				inExitChoiceBlock = false
				candidateID := SanitizeSceneID(val)
				if candidateID == "" {
					b.Warnings = append(b.Warnings, fmt.Sprintf("%s:%d: pusty lub nieprawidłowy identyfikator id: „%s”", sourceFileName, lineIdx+1, val))
				} else if inChapterHeaderBlock && currentScene == nil && currentChapterExplicitID == "" {
					currentChapterExplicitID = candidateID
					if currentChapter == "" {
						currentChapter = candidateID
					}
					b.ensureChapterWithMeta(currentChapter, candidateID, candidateID, sourceFileName)
				} else if currentScene != nil && currentScene.ExplicitID == "" && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
					currentScene.ExplicitID = candidateID
					currentScene.ID = candidateID
				} else {
					pendingSceneID = candidateID
					startScene("", false)
				}
				continue

			case "start":
				if inChapterHeaderBlock && currentScene == nil {
					startRef := SanitizePlainText(val)
					ensureCurrentChapter()
					if startRef != "" {
						if ch := b.FindChapter(currentChapter); ch != nil {
							ch.ExplicitStart = startRef
						}
					}
					continue
				}

			case "image":
				inChapterHeaderBlock = false
				inExitChoiceBlock = false
				safeImg := SanitizeRelativeMediaPath(val)
				if val != "" && safeImg == "" {
					b.Warnings = append(b.Warnings, fmt.Sprintf("%s:%d: nieprawidłowa ścieżka obrazu image: „%s”", sourceFileName, lineIdx+1, val))
				} else if currentScene != nil && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
					currentScene.ExplicitImage = safeImg
				} else {
					pendingImage = safeImg
				}
				continue

			case "voice":
				inChapterHeaderBlock = false
				inExitChoiceBlock = false
				rawVoicePath, vts, vte := ParseVoiceHeaderValue(val)
				safeVoice := SanitizeRelativeMediaPath(rawVoicePath)
				if rawVoicePath != "" && safeVoice == "" {
					b.Warnings = append(b.Warnings, fmt.Sprintf("%s:%d: nieprawidłowa ścieżka nagrania voice: „%s”", sourceFileName, lineIdx+1, val))
				} else if currentScene != nil && (headerBlockEnded || len(proseLines) > 0) && len(currentScene.Choices) == 0 && (proseLinesHaveCondDirective(proseLines) || strings.Contains(strings.ToLower(safeVoice), "__")) {
					headerBlockEnded = true
					proseLines = append(proseLines, line)
				} else if currentScene != nil && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
					currentScene.ExplicitVoice = safeVoice
					if HasVoiceTrimValues(vts, vte) {
						currentScene.VoiceTrimStart = vts
						currentScene.VoiceTrimEnd = vte
					}
				} else {
					pendingVoice = safeVoice
					if HasVoiceTrimValues(vts, vte) {
						pendingVoiceTrimStart = vts
						pendingVoiceTrimEnd = vte
					}
				}
				continue

			case "voice_trim":
				inChapterHeaderBlock = false
				inExitChoiceBlock = false
				if ts, te, okTrim := parseVoiceTrimHeader(val); okTrim {
					if currentScene != nil && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
						currentScene.VoiceTrimStart = ts
						currentScene.VoiceTrimEnd = te
					} else {
						pendingVoiceTrimStart = ts
						pendingVoiceTrimEnd = te
					}
				}
				continue

			case "exit", "next":
				inChapterHeaderBlock = false
				ensureCurrentSceneStarted()
				nextVal := SanitizePlainText(val)
				hasLabel := exitBlockHasLabel(rawLines, lineIdx)
				if !hasLabel && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
					inExitChoiceBlock = false
					if nextVal != "" {
						currentScene.ExplicitNext = nextVal
						isEnd := strings.EqualFold(nextVal, "END")
						currentScene.Choices = []Choice{
							{
								Label:      "Dalej",
								Target:     nextVal,
								IsEnding:   isEnd,
								IsImplicit: true,
							},
						}
					}
				} else {
					trimTrailingSceneSeparators()
					if len(currentScene.Choices) == 1 && currentScene.Choices[0].IsImplicit && currentScene.Choices[0].Condition == "" && currentScene.Choices[0].Effects == "" && hasLabel {
						currentScene.Choices = nil
						currentScene.ExplicitNext = ""
					}
					isEnd := strings.EqualFold(nextVal, "END")
					currentScene.Choices = append(currentScene.Choices, Choice{
						Label:      "Dalej",
						Target:     nextVal,
						IsEnding:   isEnd,
						IsImplicit: !hasLabel,
					})
					if !hasLabel && len(currentScene.Choices) == 1 {
						currentScene.ExplicitNext = nextVal
					} else {
						currentScene.ExplicitNext = ""
					}
					inExitChoiceBlock = true
				}
				continue

			case "label":
				inChapterHeaderBlock = false
				ensureCurrentSceneStarted()
				cleanLbl := SanitizePlainText(val)
				if inExitChoiceBlock && len(currentScene.Choices) > 0 {
					last := &currentScene.Choices[len(currentScene.Choices)-1]
					last.Label = cleanLbl
					last.IsImplicit = false
					currentScene.ExplicitNext = ""
				} else {
					trimTrailingSceneSeparators()
					currentScene.Choices = append(currentScene.Choices, Choice{
						Label:      cleanLbl,
						IsImplicit: false,
					})
					currentScene.ExplicitNext = ""
					inExitChoiceBlock = true
				}
				continue

			case "if":
				inChapterHeaderBlock = false
				ensureCurrentSceneStarted()
				if inExitChoiceBlock && len(currentScene.Choices) > 0 {
					last := &currentScene.Choices[len(currentScene.Choices)-1]
					cleanCond := CleanConditionExpr(val)
					if strings.HasPrefix(cleanCond, "?") {
						last.ShowWhenLocked = true
						cleanCond = strings.TrimSpace(strings.TrimPrefix(cleanCond, "?"))
					}
					last.Condition = cleanCond
					last.IsImplicit = false
					continue
				}
				headerBlockEnded = true
				proseLines = append(proseLines, line)
				continue

			case "else", "endif", "elif":
				inChapterHeaderBlock = false
				inExitChoiceBlock = false
				ensureCurrentSceneStarted()
				headerBlockEnded = true
				proseLines = append(proseLines, line)
				continue

			case "end":
				inChapterHeaderBlock = false
				inExitChoiceBlock = false
				ensureCurrentSceneStarted()
				vLower := strings.ToLower(val)
				currentScene.ExplicitEnd = vLower == "" || vLower == "true" || vLower == "1"
				continue

			case "text":
				inChapterHeaderBlock = false
				ensureCurrentSceneStarted()
				cleanTxt := SanitizePlainText(val)
				if inExitChoiceBlock && len(currentScene.Choices) > 0 {
					last := &currentScene.Choices[len(currentScene.Choices)-1]
					last.Label = cleanTxt
					last.IsImplicit = false
					currentScene.ExplicitNext = ""
					continue
				}
				inExitChoiceBlock = false
				if cleanTxt != "" {
					proseLines = append(proseLines, cleanTxt)
				}
				continue
			}
		}

		// Detect suspicious unknown header right at the top of a scene (before any prose lines)
		if currentScene != nil && len(proseLines) == 0 && len(currentScene.Choices) == 0 && trimmed != "" {
			if colonIdx := strings.IndexByte(trimmed, ':'); colonIdx > 0 {
				candKey := strings.TrimSpace(trimmed[:colonIdx])
				if headerKeyRe.MatchString(candKey) && !strings.Contains(candKey, " ") {
					b.Warnings = append(b.Warnings, fmt.Sprintf("%s:%d: nierozpoznany klucz nagłówka „%s:” w scenie „%s” (dozwolone: id, slugline, synopsis, set, image, voice, exit, end)", sourceFileName, lineIdx+1, candKey, currentScene.DisplayName()))
				}
			}
		}

		if trimmed == "" {
			inChapterHeaderBlock = false
			inExitChoiceBlock = false
			if currentScene != nil {
				headerBlockEnded = true
				if len(proseLines) > 0 && (len(currentScene.Choices) == 0 || (len(currentScene.Choices) == 1 && currentScene.Choices[0].IsImplicit)) {
					proseLines = append(proseLines, "")
				}
			}
			continue
		}

		// If text/choices appear after a chapter header without an explicit scene: line,
		// create a scene named after the current chapter (or default).
		ensureCurrentSceneStarted()

		// Check if line is a choice: "Choice Label -> Target Scene"
		// Escaped arrow "\->" is treated as literal prose text, not a choice transition.
		if strings.Contains(trimmed, `\->`) {
			unescaped := strings.ReplaceAll(line, `\->`, `->`)
			proseLines = append(proseLines, unescaped)
			continue
		}

		if label, target, isChoice := parseChoiceLine(trimmed); isChoice {
			cleanLabel, cond, eff, showLocked := ParseChoiceRuleTokens(label)
			isEnd := strings.EqualFold(target, "END") || strings.EqualFold(target, "KONIEC")
			currentScene.Choices = append(currentScene.Choices, Choice{
				Label:          SanitizePlainText(cleanLabel),
				Target:         SanitizePlainText(target),
				Condition:      SanitizeRuleExpr(cond),
				Effects:        SanitizeRuleExpr(eff),
				ShowWhenLocked: showLocked,
				IsEnding:       isEnd,
			})
			continue
		}

		if strings.Contains(trimmed, "->") {
			b.Warnings = append(b.Warnings, fmt.Sprintf("%s:%d: niepełny zapis wyboru „%s” w scenie „%s” (wymagany format: Tekst wyboru -> Cel)", sourceFileName, lineIdx+1, trimmed, currentScene.DisplayName()))
		}

		proseLines = append(proseLines, line)
	}

	flushScene()
}

func headerBlockHasTitle(rawLines []string, startIdx int) (string, bool) {
	for i := startIdx + 1; i < len(rawLines) && i <= startIdx+6; i++ {
		t := strings.TrimSpace(rawLines[i])
		if t == "" || t == "---" {
			return "", false
		}
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") {
			continue
		}
		k, v, ok := parseStructuralHeader(t)
		if !ok {
			return "", false
		}
		switch k {
		case "title":
			return v, true
		case "chapter", "act", "scene", "exit", "next":
			return "", false
		}
	}
	return "", false
}

func headerBlockHasID(rawLines []string, startIdx int) bool {
	for i := startIdx + 1; i < len(rawLines) && i <= startIdx+6; i++ {
		t := strings.TrimSpace(rawLines[i])
		if t == "" || t == "---" {
			return false
		}
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") {
			continue
		}
		k, _, ok := parseStructuralHeader(t)
		if !ok {
			return false
		}
		switch k {
		case "id":
			return true
		case "chapter", "act", "scene", "exit", "next":
			return false
		}
	}
	return false
}

func exitBlockHasLabel(rawLines []string, startIdx int) bool {
	for i := startIdx + 1; i < len(rawLines) && i <= startIdx+6; i++ {
		t := strings.TrimSpace(rawLines[i])
		if t == "" || t == "---" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") {
			if t == "" || t == "---" {
				return false
			}
			continue
		}
		k, _, ok := parseStructuralHeader(t)
		if !ok {
			return false
		}
		switch k {
		case "label", "text":
			return true
		case "if", "set":
			continue
		default:
			return false
		}
	}
	return false
}

func parseStructuralHeader(trimmed string) (string, string, bool) {
	idx := strings.IndexByte(trimmed, ':')
	if idx <= 0 {
		return "", "", false
	}
	key := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
	val := strings.TrimSpace(trimmed[idx+1:])
	switch key {
	case "chapter", "act",
		"scene",
		"title",
		"id",
		"slugline",
		"synopsis",
		"set",
		"start",
		"image",
		"voice",
		"voice_trim",
		"exit", "next",
		"label",
		"if",
		"elif", "else", "endif",
		"end",
		"text":
		return key, val, true
	default:
		return "", "", false
	}
}

func parseChoiceLine(trimmed string) (string, string, bool) {
	idx := strings.Index(trimmed, " -> ")
	if idx <= 0 {
		return "", "", false
	}
	label := strings.TrimSpace(trimmed[:idx])
	target := strings.TrimSpace(trimmed[idx+4:])
	if label == "" || target == "" {
		return "", "", false
	}
	return label, target, true
}

func proseLinesHaveCondDirective(lines []string) bool {
	for _, ln := range lines {
		if kind, _, ok := parseProseBlockDirective(strings.TrimSpace(ln), nil, nil); ok {
			switch kind {
			case "if", "elif", "else", "endif":
				return true
			}
		}
	}
	return false
}
