package book

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ParseDataFile parses book-level metadata from data.txt.
func ParseDataFile(content string, b *Book) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for lineIdx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		idx := strings.IndexByte(trimmed, ':')
		if idx <= 0 {
			b.Warnings = append(b.Warnings, fmt.Sprintf("data.txt:%d: nieprawidłowy wiersz konfiguracji „%s” (oczekiwano klucz: wartość)", lineIdx+1, trimmed))
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
		val := SanitizePlainText(trimmed[idx+1:])
		switch key {
		case "title", "tytul", "tytuł":
			if val != "" {
				b.Title = val
			}
		case "author", "autor":
			if val != "" {
				b.Author = val
			}
		case "description", "opis", "summary":
			if val != "" {
				b.Description = val
			}
		case "start", "first_scene", "pierwsza_scena":
			if val != "" {
				b.StartSceneID = val
				b.RawStart = val
			}
		case "cover", "okladka", "okładka":
			if val != "" {
				safeRel := SanitizeRelativeMediaPath(val)
				if safeRel != "" {
					b.CoverFile = safeRel
				} else {
					b.Warnings = append(b.Warnings, fmt.Sprintf("data.txt:%d: nieprawidłowa ścieżka okładki „%s”", lineIdx+1, val))
				}
			}
		case "narrated", "narracja", "voiced":
			vLower := strings.ToLower(strings.TrimSpace(val))
			b.Narrated = vLower == "true" || vLower == "1" || vLower == "yes" || vLower == "tak"
		case "imageless", "bez_obrazow", "bez_obrazów", "text_only":
			vLower := strings.ToLower(strings.TrimSpace(val))
			b.Imageless = vLower == "true" || vLower == "1" || vLower == "yes" || vLower == "tak"
		default:
			b.Warnings = append(b.Warnings, fmt.Sprintf("data.txt:%d: nieznany klucz „%s”", lineIdx+1, key))
		}
	}
}

// ParseStoryFile parses chapters, scenes, optional id:/image:/voice: headers, prose, and choices from a .txt file.
// If a chapter header is followed directly by an id: header (before any blank line), that id belongs to the chapter.
// If a chapter header is followed directly by prose without an explicit "scene:" header,
// the chapter name is automatically used as the scene name.
func ParseStoryFile(content string, sourceFileName string, b *Book) {
	rawLines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	baseFile := filepath.Base(sourceFileName)
	fileStem := strings.TrimSuffix(baseFile, filepath.Ext(baseFile))
	fileChapterID := ""
	if !strings.EqualFold(baseFile, "story.txt") && fileStem != "" {
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
	sawSceneHeader := false
	headerBlockEnded := false
	inChapterHeaderBlock := false

	flushScene := func() {
		if currentScene == nil {
			return
		}
		if strings.TrimSpace(currentScene.Name) == "" && strings.TrimSpace(currentScene.ExplicitID) == "" {
			currentScene.Name = fmt.Sprintf("Scena %d", len(b.Scenes)+1)
		}
		rawProse := strings.TrimSpace(strings.Join(proseLines, "\n"))
		currentScene.Text = SanitizePlainText(rawProse)
		currentScene.HTML = RenderSafeProseHTML(rawProse)
		currentScene.OrderIndex = len(b.Scenes)
		b.Scenes = append(b.Scenes, currentScene)
		b.addSceneToChapterWithMeta(currentScene.Chapter, currentScene.ChapterID, currentScene.SourceFile, currentScene.ID)
		currentScene = nil
		proseLines = nil
		sawSceneHeader = false
		headerBlockEnded = false
	}

	startScene := func(sceneName string, explicitSceneHeader bool) {
		flushScene()
		inChapterHeaderBlock = false
		cleanName := SanitizePlainText(sceneName)
		if currentChapter == "" {
			if currentChapterExplicitID != "" {
				currentChapter = currentChapterExplicitID
			} else if fileChapterID != "" {
				currentChapter = fileChapterID
			} else {
				currentChapter = "Rozdział 1"
			}
			b.ensureChapterWithMeta(currentChapter, fileChapterID, currentChapterExplicitID, sourceFileName)
		}
		explicitID := pendingSceneID
		pendingSceneID = ""
		sceneID := explicitID
		if sceneID == "" {
			sceneID = fmt.Sprintf("s%d", len(b.Scenes)+1)
		}
		imgVal := pendingImage
		pendingImage = ""
		voiceVal := pendingVoice
		pendingVoice = ""

		currentScene = &Scene{
			ID:            sceneID,
			ExplicitID:    explicitID,
			Name:          cleanName,
			Chapter:       currentChapter,
			ChapterID:     currentChapterExplicitID,
			ExplicitImage: imgVal,
			ExplicitVoice: voiceVal,
			SourceFile:    sourceFileName,
		}
		sawSceneHeader = explicitSceneHeader
		headerBlockEnded = false
	}

	for lineIdx, line := range rawLines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Check structural headers: chapter:, scene:, id:, image:, voice:, end:, text:
		if key, val, ok := parseStructuralHeader(trimmed); ok {
			switch key {
			case "chapter", "rozdzial", "rozdział":
				flushScene()
				chName := SanitizePlainText(val)
				currentChapter = chName
				currentChapterExplicitID = ""
				inChapterHeaderBlock = true
				if currentChapter != "" {
					b.ensureChapterWithMeta(currentChapter, fileChapterID, "", sourceFileName)
				}
				continue

			case "scene", "scena":
				inChapterHeaderBlock = false
				if currentScene != nil && !sawSceneHeader && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
					currentScene.Name = SanitizePlainText(val)
					sawSceneHeader = true
				} else {
					startScene(val, true)
				}
				continue

			case "id":
				candidateID := SanitizeSceneID(val)
				if candidateID == "" {
					b.Warnings = append(b.Warnings, fmt.Sprintf("%s:%d: pusty lub nieprawidłowy identyfikator id: „%s”", sourceFileName, lineIdx+1, val))
				} else if inChapterHeaderBlock && currentScene == nil {
					currentChapterExplicitID = candidateID
					if currentChapter == "" {
						currentChapter = candidateID
					}
					b.ensureChapterWithMeta(currentChapter, candidateID, candidateID, sourceFileName)
					inChapterHeaderBlock = false
				} else if currentScene != nil && currentScene.ExplicitID == "" && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
					currentScene.ExplicitID = candidateID
					currentScene.ID = candidateID
				} else {
					pendingSceneID = candidateID
					startScene("", false)
				}
				continue

			case "image", "obraz", "img":
				inChapterHeaderBlock = false
				safeImg := SanitizeRelativeMediaPath(val)
				if val != "" && safeImg == "" {
					b.Warnings = append(b.Warnings, fmt.Sprintf("%s:%d: nieprawidłowa ścieżka obrazu image: „%s”", sourceFileName, lineIdx+1, val))
				} else if currentScene != nil && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
					currentScene.ExplicitImage = safeImg
				} else {
					pendingImage = safeImg
				}
				continue

			case "voice", "audio", "glos", "głos":
				inChapterHeaderBlock = false
				safeVoice := SanitizeRelativeMediaPath(val)
				if val != "" && safeVoice == "" {
					b.Warnings = append(b.Warnings, fmt.Sprintf("%s:%d: nieprawidłowa ścieżka nagrania voice: „%s”", sourceFileName, lineIdx+1, val))
				} else if currentScene != nil && !headerBlockEnded && len(proseLines) == 0 && len(currentScene.Choices) == 0 {
					currentScene.ExplicitVoice = safeVoice
				} else {
					pendingVoice = safeVoice
				}
				continue

			case "next", "dalej":
				inChapterHeaderBlock = false
				if currentScene == nil {
					if currentChapter == "" {
						if currentChapterExplicitID != "" {
							currentChapter = currentChapterExplicitID
						} else if fileChapterID != "" {
							currentChapter = fileChapterID
						} else {
							currentChapter = "Rozdział 1"
						}
					}
					startScene(currentChapter, false)
				}
				nextVal := SanitizePlainText(val)
				if nextVal != "" {
					currentScene.ExplicitNext = nextVal
					isEnd := strings.EqualFold(nextVal, "END") || strings.EqualFold(nextVal, "KONIEC")
					currentScene.Choices = []Choice{
						{
							Label:      "Dalej",
							Target:     nextVal,
							IsEnding:   isEnd,
							IsImplicit: true,
						},
					}
				}
				continue

			case "end", "koniec":
				inChapterHeaderBlock = false
				if currentScene == nil {
					if currentChapter == "" {
						if currentChapterExplicitID != "" {
							currentChapter = currentChapterExplicitID
						} else if fileChapterID != "" {
							currentChapter = fileChapterID
						} else {
							currentChapter = "Rozdział 1"
						}
					}
					startScene(currentChapter, false)
				}
				vLower := strings.ToLower(val)
				currentScene.ExplicitEnd = vLower == "" || vLower == "true" || vLower == "1" || vLower == "yes" || vLower == "tak"
				continue

			case "text", "tekst":
				inChapterHeaderBlock = false
				if currentScene == nil {
					if currentChapter == "" {
						if currentChapterExplicitID != "" {
							currentChapter = currentChapterExplicitID
						} else if fileChapterID != "" {
							currentChapter = fileChapterID
						} else {
							currentChapter = "Rozdział 1"
						}
					}
					startScene(currentChapter, false)
				}
				if val != "" {
					proseLines = append(proseLines, val)
				}
				continue
			}
		}

		// Detect suspicious unknown header right at the top of a scene (before any prose lines)
		if currentScene != nil && len(proseLines) == 0 && len(currentScene.Choices) == 0 && trimmed != "" {
			if colonIdx := strings.IndexByte(trimmed, ':'); colonIdx > 0 {
				candKey := strings.TrimSpace(trimmed[:colonIdx])
				if headerKeyRe.MatchString(candKey) && !strings.Contains(candKey, " ") {
					b.Warnings = append(b.Warnings, fmt.Sprintf("%s:%d: nierozpoznany klucz nagłówka „%s:” w scenie „%s” (dozwolone: id, image, voice, next, end)", sourceFileName, lineIdx+1, candKey, currentScene.DisplayName()))
				}
			}
		}

		if trimmed == "" {
			inChapterHeaderBlock = false
			if currentScene != nil {
				headerBlockEnded = true
				if len(proseLines) > 0 {
					proseLines = append(proseLines, "")
				}
			}
			continue
		}

		// If text/choices appear after a chapter header without an explicit scene: line,
		// create a scene named after the current chapter (or default).
		if currentScene == nil {
			if currentChapter == "" {
				if currentChapterExplicitID != "" {
					currentChapter = currentChapterExplicitID
				} else if fileChapterID != "" {
					currentChapter = fileChapterID
				} else {
					currentChapter = "Rozdział 1"
				}
				b.ensureChapterWithMeta(currentChapter, fileChapterID, currentChapterExplicitID, sourceFileName)
			}
			startScene(currentChapter, false)
		}

		// Check if line is a choice: "Choice Label -> Target Scene"
		// Escaped arrow "\->" is treated as literal prose text, not a choice transition.
		if strings.Contains(trimmed, `\->`) {
			unescaped := strings.ReplaceAll(line, `\->`, `->`)
			proseLines = append(proseLines, unescaped)
			continue
		}

		if label, target, isChoice := parseChoiceLine(trimmed); isChoice {
			isEnd := strings.EqualFold(target, "END") || strings.EqualFold(target, "KONIEC")
			currentScene.Choices = append(currentScene.Choices, Choice{
				Label:    SanitizePlainText(label),
				Target:   SanitizePlainText(target),
				IsEnding: isEnd,
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

func parseStructuralHeader(trimmed string) (string, string, bool) {
	idx := strings.IndexByte(trimmed, ':')
	if idx <= 0 {
		return "", "", false
	}
	key := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
	val := strings.TrimSpace(trimmed[idx+1:])
	switch key {
	case "chapter", "rozdzial", "rozdział",
		"scene", "scena",
		"id",
		"image", "obraz", "img",
		"voice", "audio", "glos", "głos",
		"next", "dalej",
		"end", "koniec",
		"text", "tekst":
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
