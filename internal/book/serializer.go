package book

import (
	"fmt"
	"sort"
	"strings"
)

func sanitizeSingleLineMeta(s string) string {
	s = SanitizePlainText(s)
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// SerializeDataFile serializes book metadata back to data.txt content.
func SerializeDataFile(b *Book) string {
	var sb strings.Builder
	title := sanitizeSingleLineMeta(b.Title)
	if title == "" {
		title = "Nowa Opowieść"
	}
	author := sanitizeSingleLineMeta(b.Author)
	if author == "" {
		author = "Autor Nieznany"
	}
	fmt.Fprintf(&sb, "title: %s\n", title)
	fmt.Fprintf(&sb, "author: %s\n", author)
	if lic := sanitizeSingleLineMeta(b.License); lic != "" {
		fmt.Fprintf(&sb, "license: %s\n", lic)
	}
	if desc := sanitizeSingleLineMeta(b.Description); desc != "" {
		fmt.Fprintf(&sb, "description: %s\n", desc)
	}
	if b.IsScreenplay() {
		sb.WriteString("mode: screenplay\n")
		fmt.Fprintf(&sb, "standard: %s\n", b.NormalizedStandard())
	} else if strings.TrimSpace(b.Mode) != "" {
		sb.WriteString("mode: book\n")
		if strings.TrimSpace(b.ScriptStandard) != "" {
			fmt.Fprintf(&sb, "standard: %s\n", b.NormalizedStandard())
		}
	}
	if !b.IsScreenplay() {
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
	for _, ed := range b.Enums {
		if ed.Name != "" && len(ed.Values) > 0 {
			fmt.Fprintf(&sb, "enum: %s = %s\n", ed.Name, strings.Join(ed.Values, ", "))
		}
	}
	for _, vd := range b.Variables {
		if vd.Name == "" {
			continue
		}
		switch vd.Kind {
		case VarKindBool:
			if vd.BoolVal {
				fmt.Fprintf(&sb, "var: %s = true\n", vd.Name)
			} else {
				fmt.Fprintf(&sb, "var: %s = false\n", vd.Name)
			}
		case VarKindInt:
			fmt.Fprintf(&sb, "var: %s = %d\n", vd.Name, vd.IntVal)
		case VarKindEnum:
			if vd.EnumName != "" || len(vd.Options) <= 1 {
				fmt.Fprintf(&sb, "var: %s = %s\n", vd.Name, vd.DefaultValueString())
			} else {
				fmt.Fprintf(&sb, "var: %s = %s\n", vd.Name, strings.Join(vd.Options, ", "))
			}
		}
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
			out.WriteString("\n---\n\n")
		}
		ch := strings.TrimSpace(sc.Chapter)
		chID := strings.TrimSpace(sc.ChapterID)
		chMeta := b.FindChapter(ch)
		if chID == "" && chMeta != nil && chMeta.ExplicitID != "" {
			chID = chMeta.ExplicitID
		}
		if (ch != "" || chID != "") && !strings.EqualFold(ch, lastChapter) {
			if chID != "" {
				fmt.Fprintf(&out, "chapter: %s\n", chID)
				if ch != "" && !strings.EqualFold(ch, chID) {
					fmt.Fprintf(&out, "title: %s\n", ch)
				}
			} else if ch != "" {
				fmt.Fprintf(&out, "chapter: %s\n", ch)
			} else {
				out.WriteString("chapter:\n")
			}
			if chMeta != nil && strings.TrimSpace(chMeta.ExplicitStart) != "" {
				fmt.Fprintf(&out, "start: %s\n", strings.TrimSpace(chMeta.ExplicitStart))
			}
			out.WriteString("\n")
			lastChapter = ch
		}

		scID := strings.TrimSpace(sc.ExplicitID)
		scTitle := strings.TrimSpace(sc.Name)
		if scID != "" {
			fmt.Fprintf(&out, "scene: %s\n", scID)
			if scTitle != "" && !strings.EqualFold(scTitle, scID) {
				fmt.Fprintf(&out, "title: %s\n", scTitle)
			}
		} else if scTitle != "" {
			fmt.Fprintf(&out, "scene: %s\n", scTitle)
		} else {
			fmt.Fprintf(&out, "scene: Scena %d\n", i+1)
		}
		if slug := sanitizeSingleLineMeta(sc.Slugline); slug != "" {
			fmt.Fprintf(&out, "slugline: %s\n", slug)
		}
		if syn := sanitizeSingleLineMeta(sc.Synopsis); syn != "" {
			fmt.Fprintf(&out, "synopsis: %s\n", syn)
		}
		if eff := CleanEffectsExpr(sc.Effects); eff != "" {
			fmt.Fprintf(&out, "set: %s\n", eff)
		}
		if !b.IsScreenplay() && sc.ExplicitImage != "" {
			fmt.Fprintf(&out, "image: %s\n", strings.TrimSpace(sc.ExplicitImage))
		}
		if b.Narrated && sc.ExplicitVoice != "" {
			fmt.Fprintf(&out, "voice: %s\n", strings.TrimSpace(sc.ExplicitVoice))
		}
		text := strings.TrimSpace(sc.Text)
		if text != "" {
			fmt.Fprintf(&out, "\n%s\n", text)
		}

		if sc.ExplicitEnd {
			out.WriteString("\nend: true\n")
		} else if len(sc.Choices) == 1 && !sc.Choices[0].IsEnding {
			chItem := sc.Choices[0]
			tgt := strings.TrimSpace(chItem.Target)
			nextRef := ""
			if tgt != "" && !strings.EqualFold(tgt, "Next") && !strings.EqualFold(tgt, "Dalej") {
				nextRef = tgt
			} else if strings.TrimSpace(sc.ExplicitNext) != "" {
				nextRef = strings.TrimSpace(sc.ExplicitNext)
			} else if chItem.TargetSceneID != "" {
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
				fmt.Fprintf(&out, "\nexit: %s\n", nextRef)
				if cond := CleanConditionExpr(chItem.Condition); cond != "" {
					if chItem.ShowWhenLocked {
						fmt.Fprintf(&out, "if?: %s\n", cond)
					} else {
						fmt.Fprintf(&out, "if: %s\n", cond)
					}
				}
				if eff := CleanEffectsExpr(chItem.Effects); eff != "" {
					fmt.Fprintf(&out, "set: %s\n", eff)
				}
			}
		} else if len(sc.Choices) >= 2 {
			var validChoices []Choice
			for _, chItem := range sc.Choices {
				if strings.TrimSpace(chItem.Label) != "" && strings.TrimSpace(chItem.Target) != "" {
					validChoices = append(validChoices, chItem)
				}
			}
			for _, chItem := range validChoices {
				out.WriteString("\n")
				fmt.Fprintf(&out, "exit: %s\n", strings.TrimSpace(chItem.Target))
				fmt.Fprintf(&out, "text: %s\n", sanitizeSingleLineMeta(chItem.Label))
				if cond := CleanConditionExpr(chItem.Condition); cond != "" {
					if chItem.ShowWhenLocked {
						fmt.Fprintf(&out, "if?: %s\n", cond)
					} else {
						fmt.Fprintf(&out, "if: %s\n", cond)
					}
				}
				if eff := CleanEffectsExpr(chItem.Effects); eff != "" {
					fmt.Fprintf(&out, "set: %s\n", eff)
				}
			}
		} else if len(sc.Choices) == 1 && !sc.ExplicitEnd && sc.Choices[0].IsEnding {
			out.WriteString("\nexit: END\n")
		}
	}

	return out.String()
}
