//go:build !(js && wasm)

package book

import (
	"fmt"
	"sort"
	"strings"
)

// VOCueRow represents a single voiceover or actor dialogue cue extracted from a project.
type VOCueRow struct {
	Index         int
	Chapter       string
	SceneID       string
	SceneTitle    string
	Character     string
	Parenthetical string
	Line          string
	VoiceFile     string
	WordCount     int
}

// CollectProjectVOCues extracts all voiceover / actor cues across the project in graph order.
func CollectProjectVOCues(b *Book) []VOCueRow {
	if b == nil || len(b.Scenes) == 0 {
		return nil
	}
	normStd := b.NormalizedStandard()
	isSp := b.IsScreenplay()
	var rows []VOCueRow
	seq := 1

	for _, sc := range b.Scenes {
		chName := strings.TrimSpace(sc.Chapter)
		if chName == "" {
			chName = b.DefaultChapterName()
		}
		scTitle := sc.DisplayName()
		if isSp && strings.TrimSpace(sc.Slugline) != "" {
			scTitle = strings.TrimSpace(sc.Slugline)
		}
		voicePath := strings.TrimSpace(sc.ExplicitVoice)
		if voicePath == "" && sc.AudioFile != "" {
			voicePath = sc.AudioFile
		}
		if voicePath == "" {
			voicePath = "voice/" + sc.ID + ".flac"
		}

		if isSp {
			blocks := ParseScreenplaySceneBlocks(sc, normStd)
			curChar := ""
			curParen := ""
			for _, blk := range blocks {
				switch blk.Kind {
				case BlockCharacter:
					curChar = strings.TrimSpace(blk.CharacterName)
					if curChar == "" {
						curChar = strings.ToUpper(strings.TrimSpace(blk.Text))
					}
					curParen = ""
				case BlockParenthetical:
					curParen = strings.TrimSpace(blk.Text)
				case BlockDialogue:
					lineTxt := strings.Join(strings.Fields(blk.Text), " ")
					if lineTxt == "" {
						continue
					}
					speaker := curChar
					if speaker == "" {
						speaker = "NARRATOR"
					}
					rows = append(rows, VOCueRow{
						Index:         seq,
						Chapter:       chName,
						SceneID:       sc.ID,
						SceneTitle:    scTitle,
						Character:     speaker,
						Parenthetical: curParen,
						Line:          lineTxt,
						VoiceFile:     voicePath,
						WordCount:     len(strings.Fields(lineTxt)),
					})
					seq++
					curParen = ""
				}
			}
			continue
		}

		if HasConditionalVoiceDirectives(sc.Text) {
			secs := ExtractSceneVoiceSections(sc, b)
			for _, sec := range secs {
				lineTxt := strings.Join(strings.Fields(strings.ReplaceAll(sec.Text, `\->`, `->`)), " ")
				if lineTxt == "" {
					continue
				}
				secVoice := strings.TrimSpace(sec.AudioFile)
				if secVoice == "" {
					secVoice = strings.TrimSpace(sec.ExplicitVoice)
				}
				if secVoice == "" {
					secVoice = sec.TargetFile
				}
				rows = append(rows, VOCueRow{
					Index:         seq,
					Chapter:       chName,
					SceneID:       sc.ID,
					SceneTitle:    scTitle,
					Character:     "LEKTOR",
					Parenthetical: sec.Label,
					Line:          lineTxt,
					VoiceFile:     secVoice,
					WordCount:     len(strings.Fields(lineTxt)),
				})
				seq++
			}
			continue
		}

		lineTxt := strings.Join(strings.Fields(strings.ReplaceAll(sc.Text, `\->`, `->`)), " ")
		if lineTxt == "" {
			continue
		}
		rows = append(rows, VOCueRow{
			Index:      seq,
			Chapter:    chName,
			SceneID:    sc.ID,
			SceneTitle: scTitle,
			Character:  "LEKTOR",
			Line:       lineTxt,
			VoiceFile:  voicePath,
			WordCount:  len(strings.Fields(lineTxt)),
		})
		seq++
	}
	return rows
}

func csvEscapeField(val string) string {
	clean := strings.ReplaceAll(strings.ReplaceAll(val, "\r\n", " "), "\n", " ")
	if strings.ContainsAny(clean, `";,`) {
		return `"` + strings.ReplaceAll(clean, `"`, `""`) + `"`
	}
	return clean
}

// ExportVOTableCSV exports a semicolon-separated UTF-8 BOM CSV recording sheet for VO studios and actors.
func ExportVOTableCSV(b *Book) string {
	rows := CollectProjectVOCues(b)
	var sb strings.Builder
	sb.WriteString("\xef\xbb\xbf")
	sb.WriteString("Nr;Akt_Rozdzial;ID_Sceny;Scena;Postac;Wskazowka;Kwestia;Plik_Audio;Slowa\n")
	for _, r := range rows {
		fmt.Fprintf(&sb, "%d;%s;%s;%s;%s;%s;%s;%s;%d\n",
			r.Index,
			csvEscapeField(r.Chapter),
			csvEscapeField(r.SceneID),
			csvEscapeField(r.SceneTitle),
			csvEscapeField(r.Character),
			csvEscapeField(r.Parenthetical),
			csvEscapeField(r.Line),
			csvEscapeField(r.VoiceFile),
			r.WordCount,
		)
	}
	return sb.String()
}

// ExportActorScriptsPDF generates a print-ready A4 PDF with dialogue sides grouped by character/voice actor.
func ExportActorScriptsPDF(b *Book) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf("brak aktywnego projektu")
	}
	rows := CollectProjectVOCues(b)
	w := newPDFLinearPageWriter(595.28, 841.89, 72.0, 72.0, 64.0, 13.0, CourierLinesPerPageA4)

	title := strings.ToUpper(strings.TrimSpace(b.Title))
	if title == "" {
		title = "BEZ TYTUŁU"
	}
	w.writeWrapped(title+" — SKRYPTY AKTORSKIE / LEKTORSKIE", 72.0, 60, true, 13.0)
	w.blankLine()

	if len(rows) == 0 {
		w.writeWrapped("Brak kwestii dialogowych lub lektorskich w projekcie.", 72.0, 60, false, 11.0)
		return w.bytes(), nil
	}

	byChar := make(map[string][]VOCueRow)
	var charOrder []string
	for _, r := range rows {
		if _, exists := byChar[r.Character]; !exists {
			charOrder = append(charOrder, r.Character)
		}
		byChar[r.Character] = append(byChar[r.Character], r)
	}
	sort.Strings(charOrder)

	for cIdx, charName := range charOrder {
		cues := byChar[charName]
		totalWords := 0
		for _, c := range cues {
			totalWords += c.WordCount
		}
		if cIdx > 0 {
			w.blankLine()
		}
		w.ensureLines(4)
		w.writeWrapped(fmt.Sprintf("POSTAĆ: %s (%d kwestii • %d słów)", charName, len(cues), totalWords), 72.0, 60, true, 12.0)
		w.blankLine()

		for _, c := range cues {
			w.ensureLines(3)
			meta := fmt.Sprintf("[#%d • %s • %s]", c.Index, c.SceneTitle, c.VoiceFile)
			w.writeWrapped(meta, 72.0, 62, true, 10.0)
			if c.Parenthetical != "" {
				w.writeWrapped(c.Parenthetical, 86.0, 58, false, 10.5)
			}
			w.writeWrapped(c.Line, 86.0, 58, false, 11.0)
			w.blankLine()
		}
	}

	return w.bytes(), nil
}
