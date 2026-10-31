package book

import (
	"fmt"
	"sort"
	"strings"
)

// SerializeDataFile serializes book metadata back to data.txt content.
func SerializeDataFile(b *Book) string {
	var sb strings.Builder
	title := SanitizePlainText(b.Title)
	if title == "" {
		title = "Nowa Opowieść"
	}
	author := SanitizePlainText(b.Author)
	if author == "" {
		author = "Autor Nieznany"
	}
	fmt.Fprintf(&sb, "title: %s\n", title)
	fmt.Fprintf(&sb, "author: %s\n", author)
	if desc := SanitizePlainText(b.Description); desc != "" {
		fmt.Fprintf(&sb, "description: %s\n", desc)
	}
	if b.Narrated {
		sb.WriteString("narrated: true\n")
	} else {
		sb.WriteString("narrated: false\n")
	}
	if b.Imageless {
		sb.WriteString("imageless: true\n")
	} else {
		sb.WriteString("imageless: false\n")
	}
	startVal := strings.TrimSpace(b.RawStart)
	if startVal == "" && len(b.Scenes) > 0 {
		if b.Scenes[0].ExplicitID != "" {
			startVal = b.Scenes[0].ExplicitID
		} else {
			startVal = b.Scenes[0].Name
		}
	}
	if startVal != "" {
		fmt.Fprintf(&sb, "start: %s\n", startVal)
	}
	return sb.String()
}

// SerializeScenesForFile formats all scenes belonging to a given source .txt file back into human-friendly syntax.
func SerializeScenesForFile(b *Book, sourceFile string) string {
	var scenes []*Scene
	for _, sc := range b.Scenes {
		sf := sc.SourceFile
		if sf == "" {
			sf = b.ChapterFileNameFor(sc.Chapter, sc.ChapterID)
		}
		if strings.EqualFold(sf, sourceFile) || strings.EqualFold(b.ChapterFileNameFor(sc.Chapter, sc.ChapterID), sourceFile) {
			scenes = append(scenes, sc)
		}
	}

	sort.SliceStable(scenes, func(i, j int) bool {
		return scenes[i].OrderIndex < scenes[j].OrderIndex
	})

	var out strings.Builder
	lastChapter := ""

	for i, sc := range scenes {
		if i > 0 {
			out.WriteString("\n")
		}
		ch := strings.TrimSpace(sc.Chapter)
		chID := strings.TrimSpace(sc.ChapterID)
		if chID == "" {
			if chMeta := b.FindChapter(ch); chMeta != nil && chMeta.ExplicitID != "" {
				chID = chMeta.ExplicitID
			}
		}
		if (ch != "" || chID != "") && !strings.EqualFold(ch, lastChapter) {
			if ch != "" && !strings.EqualFold(ch, chID) {
				fmt.Fprintf(&out, "chapter: %s\n", ch)
			} else if chID == "" {
				fmt.Fprintf(&out, "chapter: %s\n", ch)
			} else {
				out.WriteString("chapter:\n")
			}
			if chID != "" {
				fmt.Fprintf(&out, "id: %s\n", chID)
			}
			out.WriteString("\n")
			lastChapter = ch
		}

		if strings.TrimSpace(sc.Name) != "" {
			fmt.Fprintf(&out, "scene: %s\n", strings.TrimSpace(sc.Name))
		} else if strings.TrimSpace(sc.ExplicitID) == "" {
			fmt.Fprintf(&out, "scene: Scena %d\n", i+1)
		}
		if sc.ExplicitID != "" {
			fmt.Fprintf(&out, "id: %s\n", strings.TrimSpace(sc.ExplicitID))
		}
		if sc.ExplicitImage != "" {
			fmt.Fprintf(&out, "image: %s\n", strings.TrimSpace(sc.ExplicitImage))
		}
		if b.Narrated && sc.ExplicitVoice != "" {
			fmt.Fprintf(&out, "voice: %s\n", strings.TrimSpace(sc.ExplicitVoice))
		}
		if sc.ExplicitEnd {
			out.WriteString("end: true\n")
		} else if len(sc.Choices) == 1 && !sc.Choices[0].IsEnding {
			chItem := sc.Choices[0]
			tgt := strings.TrimSpace(chItem.Target)
			nextRef := ""
			if tgt != "" && !strings.EqualFold(tgt, "Next") && !strings.EqualFold(tgt, "Dalej") {
				nextRef = tgt
			} else if strings.TrimSpace(sc.ExplicitNext) != "" {
				nextRef = strings.TrimSpace(sc.ExplicitNext)
			} else if chItem.TargetSceneID != "" && (i+1 >= len(scenes) || chItem.TargetSceneID != scenes[i+1].ID) {
				for _, cand := range b.Scenes {
					if cand.ID == chItem.TargetSceneID {
						if cand.ExplicitID != "" {
							nextRef = cand.ExplicitID
						} else {
							nextRef = cand.Name
						}
						break
					}
				}
			}
			if nextRef != "" {
				fmt.Fprintf(&out, "next: %s\n", nextRef)
			}
		}
		text := strings.TrimSpace(sc.Text)
		if text != "" {
			fmt.Fprintf(&out, "%s\n", text)
		}

		if len(sc.Choices) >= 2 {
			var validChoices []Choice
			for _, chItem := range sc.Choices {
				if strings.TrimSpace(chItem.Label) != "" && strings.TrimSpace(chItem.Target) != "" {
					validChoices = append(validChoices, chItem)
				}
			}
			if len(validChoices) > 0 {
				out.WriteString("\n")
				for _, chItem := range validChoices {
					fmt.Fprintf(&out, "%s -> %s\n", strings.TrimSpace(chItem.Label), strings.TrimSpace(chItem.Target))
				}
			}
		} else if len(sc.Choices) == 1 && !sc.ExplicitEnd && sc.Choices[0].IsEnding {
			out.WriteString("\nnext: END\n")
		}
	}

	return out.String()
}
