//go:build !(js && wasm)

package book

import (
	"fmt"
	"strings"
)

// ExportBookPDF generates a literary A4 PDF along the primary path of an interactive book (mode: book).
func ExportBookPDF(b *Book) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf("brak aktywnej książki")
	}
	w := newPDFLinearPageWriter(595.28, 841.89, 72.0, 72.0, 68.0, 14.0, 50)

	title := strings.TrimSpace(b.Title)
	if title == "" {
		title = "Bez tytułu"
	}
	author := strings.TrimSpace(b.Author)
	if author == "" {
		author = "Autor"
	}

	w.writeWrapped(strings.ToUpper(title), 72.0, 58, true, 16.0)
	w.writeWrapped(author, 72.0, 62, false, 12.0)
	if desc := strings.TrimSpace(b.Description); desc != "" {
		w.blankLine()
		w.writeWrapped(desc, 72.0, 62, false, 11.0)
	}
	if lic := strings.TrimSpace(b.EffectiveLicense()); lic != "" {
		w.blankLine()
		w.writeWrapped(lic, 72.0, 62, false, 10.0)
	}
	w.flushPage()

	scenes := LinearizeScreenplayScenes(b)
	if len(scenes) == 0 {
		scenes = b.Scenes
	}
	defaultVars := DefaultVariablesMap(b)
	lastChapter := ""

	for i, sc := range scenes {
		chName := strings.TrimSpace(sc.Chapter)
		if chName == "" {
			chName = b.DefaultChapterName()
		}
		if !strings.EqualFold(chName, lastChapter) {
			if i > 0 {
				w.blankLine()
			}
			w.ensureLines(4)
			w.writeWrapped(strings.ToUpper(chName), 72.0, 58, true, 13.0)
			w.blankLine()
			lastChapter = chName
		}

		scTitle := strings.TrimSpace(sc.Name)
		if scTitle != "" && !strings.EqualFold(scTitle, chName) {
			w.ensureLines(3)
			w.writeWrapped(scTitle, 72.0, 60, true, 12.0)
		}

		rawProse := strings.ReplaceAll(sc.Text, `\->`, `->`)
		rawProse = EvaluateProseText(rawProse, defaultVars, b)
		paragraphs := strings.Split(strings.ReplaceAll(rawProse, "\r\n", "\n"), "\n\n")
		for _, para := range paragraphs {
			cleanPara := strings.Join(strings.Fields(para), " ")
			if cleanPara == "" {
				continue
			}
			w.writeWrapped(cleanPara, 72.0, 62, false, 11.5)
			w.blankLine()
		}
	}

	return w.bytes(), nil
}
