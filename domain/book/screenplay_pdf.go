//go:build !(js && wasm)

package book

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type pdfTextRun struct {
	x    float64
	y    float64
	bold bool
	size float64
	text string
}

type pdfPage struct {
	runs []pdfTextRun
}

// encodePDFPolishByte maps Unicode runes (including Polish diacritics and typography) to single-byte CP1250/WinAnsi codes
// matching the custom /Encoding /Differences table in the generated PDF.
func encodePDFPolishBytes(s string) []byte {
	var out []byte
	for _, r := range s {
		switch r {
		case 'Ą':
			out = append(out, 0xA5)
		case 'Ć':
			out = append(out, 0xC6)
		case 'Ę':
			out = append(out, 0xCA)
		case 'Ł':
			out = append(out, 0xA3)
		case 'Ń':
			out = append(out, 0xD1)
		case 'Ó':
			out = append(out, 0xD3)
		case 'Ś':
			out = append(out, 0x8C)
		case 'Ź':
			out = append(out, 0x8F)
		case 'Ż':
			out = append(out, 0xAF)
		case 'ą':
			out = append(out, 0xB9)
		case 'ć':
			out = append(out, 0xE6)
		case 'ę':
			out = append(out, 0xEA)
		case 'ł':
			out = append(out, 0xB3)
		case 'ń':
			out = append(out, 0xF1)
		case 'ó':
			out = append(out, 0xF3)
		case 'ś':
			out = append(out, 0x9C)
		case 'ź':
			out = append(out, 0x9F)
		case 'ż':
			out = append(out, 0xBF)
		case '„':
			out = append(out, 0x84)
		case '”', '“':
			out = append(out, 0x94)
		case '–':
			out = append(out, 0x96)
		case '—':
			out = append(out, 0x97)
		case '…':
			out = append(out, 0x85)
		case '•':
			out = append(out, 0x95)
		case '’', '‘':
			out = append(out, '\'')
		default:
			if r >= 0x20 && r <= 0x7E {
				out = append(out, byte(r))
			} else if r >= 0xA0 && r <= 0xFF {
				out = append(out, byte(r))
			} else {
				out = append(out, '?')
			}
		}
	}
	return out
}

func pdfLiteralString(s string) string {
	encoded := encodePDFPolishBytes(s)
	var sb strings.Builder
	sb.WriteByte('(')
	for _, b := range encoded {
		switch b {
		case '(', ')', '\\':
			sb.WriteByte('\\')
			sb.WriteByte(b)
		default:
			if b >= 0x20 && b <= 0x7E {
				sb.WriteByte(b)
			} else {
				fmt.Fprintf(&sb, "\\%03o", b)
			}
		}
	}
	sb.WriteByte(')')
	return sb.String()
}

func buildPolishToUnicodeCMap() string {
	return `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def
/CMapName /PolishCMap def
/CMapType 2 def
1 begincodespacerange
<00> <FF>
endcodespacerange
24 beginbfchar
<84> <201E>
<85> <2026>
<8C> <015A>
<8F> <0179>
<94> <201D>
<95> <2022>
<96> <2013>
<97> <2014>
<9C> <015B>
<9F> <017A>
<A3> <0141>
<A5> <0104>
<AF> <017B>
<B3> <0142>
<B9> <0105>
<BF> <017C>
<C6> <0106>
<CA> <0118>
<D1> <0143>
<D3> <00D3>
<E6> <0107>
<EA> <0119>
<F1> <0144>
<F3> <00F3>
endbfchar
endcmap
CMapName currentdict /CMap defineresource pop
end
end`
}

// ExportScreenplayPDF generates a print-ready PDF 1.4 screenplay document in pure Go (zero external dependencies)
// supporting Polish/European A4 (595.28 x 841.89 pt) and US Letter (612.0 x 792.0 pt), full Polish diacritics,
// Title Page, Lista Postaci / Zestawienie Lokacji, numbered sluglines, and (WIĘCEJ)/(CIĄG DALSZY) page breaks.
func ExportScreenplayPDF(b *Book) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf("brak aktywnego projektu scenariusza")
	}

	normStd := b.NormalizedStandard()
	isPL := normStd == "pl"
	isEU := normStd == "eu"
	numberedScenes := isPL || isEU

	pageWidth := 595.28
	pageHeight := 841.89
	maxLinesPerPage := CourierLinesPerPageA4
	if normStd == "us" {
		pageWidth = 612.0
		pageHeight = 792.0
		maxLinesPerPage = CourierLinesPerPageLetter
	}

	const (
		fontSize    = 12.0
		lineHeight  = 12.0
		charWidthPt = 7.2 // 12pt Courier pitch (10 cpi = 7.2pt/char)
		topMargin   = 72.0
		leftActionX = 108.0 // 1.5 in binding margin
		leftDialX   = 180.0 // 2.5 in dialogue margin
		leftParenX  = 216.0 // 3.0 in parenthetical margin
		leftCharX   = 252.0 // 3.5 in character cue margin
		leftNumX    = 78.0
		rightMargin = 72.0
	)

	moreMarker := "(MORE)"
	contdSuffix := "(CONT'D)"
	if isPL {
		moreMarker = "(WIĘCEJ)"
		contdSuffix = "(CIĄG DALSZY)"
	}

	var pages []pdfPage

	// 1. Page 1: Title Page
	titlePage := pdfPage{}
	title := strings.ToUpper(strings.TrimSpace(b.Title))
	if title == "" {
		title = "BEZ TYTUŁU"
	}
	author := strings.TrimSpace(b.Author)
	if author == "" {
		author = "Autor"
	}
	titleLines := WrapCourierText(title, 44)
	titleY := pageHeight * 0.60
	for _, tl := range titleLines {
		tlWidth := float64(utf8.RuneCountInString(tl)) * charWidthPt
		tx := (pageWidth - tlWidth) / 2.0
		if tx < leftActionX {
			tx = leftActionX
		}
		titlePage.runs = append(titlePage.runs, pdfTextRun{
			x: tx, y: titleY, bold: true, size: fontSize, text: tl,
		})
		titleY -= lineHeight * 1.5
	}

	creditLabel := "Written by"
	if isPL {
		creditLabel = "scenariusz:"
	}
	creditWidth := float64(utf8.RuneCountInString(creditLabel)) * charWidthPt
	titlePage.runs = append(titlePage.runs, pdfTextRun{
		x: (pageWidth - creditWidth) / 2.0, y: titleY - 24.0, bold: false, size: fontSize, text: creditLabel,
	})
	authorWidth := float64(utf8.RuneCountInString(author)) * charWidthPt
	titlePage.runs = append(titlePage.runs, pdfTextRun{
		x: (pageWidth - authorWidth) / 2.0, y: titleY - 42.0, bold: false, size: fontSize, text: author,
	})

	if desc := strings.TrimSpace(b.Description); desc != "" {
		descLines := WrapCourierText(desc, 50)
		dy := titleY - 84.0
		for _, dl := range descLines {
			dlW := float64(utf8.RuneCountInString(dl)) * charWidthPt
			titlePage.runs = append(titlePage.runs, pdfTextRun{
				x: (pageWidth - dlW) / 2.0, y: dy, bold: false, size: 10.0, text: dl,
			})
			dy -= lineHeight
		}
	}

	branchName := "main"
	if h, err := LoadBookHistory(b); err == nil && h != nil {
		branchName = h.NormalizedActiveBranch()
	}
	draftFooter := fmt.Sprintf("Wersja: %s | %s", branchName, time.Now().Format("2006-01-02"))
	if !isPL {
		draftFooter = fmt.Sprintf("Draft: %s | %s", branchName, time.Now().Format("2006-01-02"))
	}
	titlePage.runs = append(titlePage.runs, pdfTextRun{
		x: leftActionX, y: 84.0, bold: false, size: 10.0, text: draftFooter,
	})
	pages = append(pages, titlePage)

	// 2. Page 2 (in PL/EU mode): Lista Postaci & Zestawienie Lokacji
	bd := AnalyzeScreenplay(b)
	if (isPL || isEU) && (len(bd.Characters) > 0 || len(bd.Locations) > 0) {
		castPage := pdfPage{}
		cy := pageHeight - topMargin
		castHeader := "LISTA POSTACI / ZESTAWIENIE LOKACJI"
		if isEU {
			castHeader = "CAST & LOCATION BREAKDOWN"
		}
		castPage.runs = append(castPage.runs, pdfTextRun{
			x: leftActionX, y: cy, bold: true, size: fontSize, text: castHeader,
		})
		cy -= lineHeight * 2.0

		if len(bd.Characters) > 0 {
			subH := "OBSADA (LISTA POSTACI):"
			if isEU {
				subH = "CAST:"
			}
			castPage.runs = append(castPage.runs, pdfTextRun{
				x: leftActionX, y: cy, bold: true, size: fontSize, text: subH,
			})
			cy -= lineHeight * 1.2
			for _, ch := range bd.Characters {
				if cy < 96.0 {
					break
				}
				line := fmt.Sprintf("- %-24s (kwestie: %d, sceny: %d, dni zdjęciowe: ~%d)", ch.Name, ch.DialogueCues, ch.TotalScenes, ch.EstimatedCallDays)
				castPage.runs = append(castPage.runs, pdfTextRun{
					x: leftActionX, y: cy, bold: false, size: 10.5, text: line,
				})
				cy -= lineHeight
			}
			cy -= lineHeight
		}

		if len(bd.Locations) > 0 && cy >= 120.0 {
			subL := "LOKACJE:"
			if isEU {
				subL = "LOCATIONS:"
			}
			castPage.runs = append(castPage.runs, pdfTextRun{
				x: leftActionX, y: cy, bold: true, size: fontSize, text: subL,
			})
			cy -= lineHeight * 1.2
			for _, loc := range bd.Locations {
				if cy < 84.0 {
					break
				}
				line := fmt.Sprintf("- %-28s (%s, sceny: %d, objętość: %s)", loc.Name, loc.IntExt, loc.SceneCount, loc.FormattedLength)
				castPage.runs = append(castPage.runs, pdfTextRun{
					x: leftActionX, y: cy, bold: false, size: 10.5, text: line,
				})
				cy -= lineHeight
			}
		}
		pages = append(pages, castPage)
	}

	// 3. Script Body Pages with Orphan & (WIĘCEJ)/(CIĄG DALSZY) Pagination
	scriptPageNum := 1
	curPage := pdfPage{}
	curLine := 0

	lineY := func(lineIdx int) float64 {
		return (pageHeight - topMargin) - float64(lineIdx)*lineHeight
	}

	startNewScriptPage := func() {
		pages = append(pages, curPage)
		scriptPageNum++
		curPage = pdfPage{}
		curLine = 0
		pageNumStr := fmt.Sprintf("%d.", scriptPageNum)
		numX := pageWidth - rightMargin - float64(len(pageNumStr))*charWidthPt
		curPage.runs = append(curPage.runs, pdfTextRun{
			x: numX, y: pageHeight - 48.0, bold: false, size: fontSize, text: pageNumStr,
		})
	}

	ensureRemainingLines := func(minNeeded int) {
		if curLine > 0 && (maxLinesPerPage-curLine) < minNeeded {
			startNewScriptPage()
		}
	}

	addBlankLine := func() {
		if curLine > 0 {
			curLine++
			if curLine >= maxLinesPerPage {
				startNewScriptPage()
			}
		}
	}

	scenes := LinearizeScreenplayScenes(b)
	for idx, sc := range scenes {
		sceneNum := idx + 1
		blocks := ParseScreenplaySceneBlocks(sc, normStd)
		if len(blocks) == 0 || blocks[0].Kind != BlockSlugline {
			slug := strings.ToUpper(strings.TrimSpace(sc.Slugline))
			if slug == "" {
				slug = DefaultSluglineForStandard(normStd)
			}
			blocks = append([]ScreenplayBlock{{Kind: BlockSlugline, Text: slug}}, blocks...)
		}

		if idx > 0 {
			addBlankLine()
		}

		renderedPrimarySlug := false
		activeCharCue := ""

		for bIdx := 0; bIdx < len(blocks); bIdx++ {
			blk := blocks[bIdx]
			switch blk.Kind {
			case BlockSlugline:
				if bIdx > 0 {
					addBlankLine()
				}
				// Never orphan a slugline at the bottom of a page (keep at least 3 lines)
				ensureRemainingLines(3)
				slugLines := WrapCourierText(strings.ToUpper(blk.Text), CourierMaxColsSlugline)
				if len(slugLines) == 0 {
					slugLines = []string{strings.ToUpper(blk.Text)}
				}
				for lIdx, sl := range slugLines {
					if curLine >= maxLinesPerPage {
						startNewScriptPage()
					}
					y := lineY(curLine)
					if !renderedPrimarySlug && lIdx == 0 && numberedScenes {
						numLabel := fmt.Sprintf("%d.", sceneNum)
						curPage.runs = append(curPage.runs, pdfTextRun{
							x: leftNumX, y: y, bold: true, size: fontSize, text: numLabel,
						})
					}
					curPage.runs = append(curPage.runs, pdfTextRun{
						x: leftActionX, y: y, bold: true, size: fontSize, text: sl,
					})
					curLine++
				}
				renderedPrimarySlug = true

			case BlockAction:
				addBlankLine()
				actLines := WrapCourierText(blk.Text, CourierMaxColsAction)
				for _, al := range actLines {
					if curLine >= maxLinesPerPage {
						startNewScriptPage()
					}
					curPage.runs = append(curPage.runs, pdfTextRun{
						x: leftActionX, y: lineY(curLine), bold: false, size: fontSize, text: al,
					})
					curLine++
				}

			case BlockCharacter:
				addBlankLine()
				// Never orphan a character cue at the bottom of a page (keep cue + at least 2 lines)
				ensureRemainingLines(3)
				activeCharCue = blk.CharacterName
				if activeCharCue == "" {
					activeCharCue = blk.Text
				}
				curPage.runs = append(curPage.runs, pdfTextRun{
					x: leftCharX, y: lineY(curLine), bold: false, size: fontSize, text: blk.Text,
				})
				curLine++

			case BlockParenthetical:
				ensureRemainingLines(2)
				pLines := WrapCourierText(blk.Text, CourierMaxColsParenthetical)
				for _, pl := range pLines {
					if curLine >= maxLinesPerPage {
						startNewScriptPage()
					}
					curPage.runs = append(curPage.runs, pdfTextRun{
						x: leftParenX, y: lineY(curLine), bold: false, size: fontSize, text: pl,
					})
					curLine++
				}

			case BlockDialogue:
				dLines := WrapCourierText(blk.Text, CourierMaxColsDialogue)
				charBase := blk.CharacterName
				if charBase == "" {
					charBase = activeCharCue
				}
				if charBase == "" {
					charBase = "POSTAĆ"
				}
				dIdx := 0
				for dIdx < len(dLines) {
					rem := maxLinesPerPage - curLine
					remainingDialogue := len(dLines) - dIdx
					if remainingDialogue <= rem {
						for ; dIdx < len(dLines); dIdx++ {
							curPage.runs = append(curPage.runs, pdfTextRun{
								x: leftDialX, y: lineY(curLine), bold: false, size: fontSize, text: dLines[dIdx],
							})
							curLine++
						}
						break
					}
					// Need to split dialogue across page break with (WIĘCEJ)/(MORE) and CHARACTER (CIĄG DALSZY)/(CONT'D)
					if rem <= 2 {
						startNewScriptPage()
						contCue := fmt.Sprintf("%s %s", charBase, contdSuffix)
						curPage.runs = append(curPage.runs, pdfTextRun{
							x: leftCharX, y: lineY(curLine), bold: false, size: fontSize, text: contCue,
						})
						curLine++
						continue
					}
					linesBeforeBreak := rem - 1
					for k := 0; k < linesBeforeBreak && dIdx < len(dLines); k++ {
						curPage.runs = append(curPage.runs, pdfTextRun{
							x: leftDialX, y: lineY(curLine), bold: false, size: fontSize, text: dLines[dIdx],
						})
						curLine++
						dIdx++
					}
					// Emit (WIĘCEJ) / (MORE) on last line of current page
					curPage.runs = append(curPage.runs, pdfTextRun{
						x: leftCharX, y: lineY(curLine), bold: false, size: fontSize, text: moreMarker,
					})
					curLine++

					// Start next page with CHARACTER (CIĄG DALSZY) / (CONT'D)
					startNewScriptPage()
					contCue := fmt.Sprintf("%s %s", charBase, contdSuffix)
					curPage.runs = append(curPage.runs, pdfTextRun{
						x: leftCharX, y: lineY(curLine), bold: false, size: fontSize, text: contCue,
					})
					curLine++
				}

			case BlockTransition:
				addBlankLine()
				ensureRemainingLines(2)
				tText := strings.ToUpper(strings.TrimSpace(blk.Text))
				tw := float64(utf8.RuneCountInString(tText)) * charWidthPt
				tx := pageWidth - rightMargin - tw
				if tx < leftActionX {
					tx = leftActionX
				}
				curPage.runs = append(curPage.runs, pdfTextRun{
					x: tx, y: lineY(curLine), bold: false, size: fontSize, text: tText,
				})
				curLine++
			}
		}
	}

	if len(curPage.runs) > 0 {
		pages = append(pages, curPage)
	}

	return serializePDFDocument(pages, pageWidth, pageHeight), nil
}

func serializePDFDocument(pages []pdfPage, pageWidth, pageHeight float64) []byte {
	// Object allocation:
	// 1: Catalog
	// 2: Pages
	// 3: Encoding (/Differences for Polish CP1250/WinAnsi)
	// 4: ToUnicode CMap stream
	// 5: Font F1 (Courier)
	// 6: Font F2 (Courier-Bold)
	// 7..6+2*len(pages): Page object + Content stream object pairs
	totalObjs := 6 + 2*len(pages)
	offsets := make([]int, totalObjs+1)

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")

	writeObj := func(id int, body string) {
		offsets[id] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, body)
	}

	writeObj(1, "<< /Type /Catalog /Pages 2 0 R >>")

	var kids []string
	for i := range pages {
		pageObjID := 7 + i*2
		kids = append(kids, fmt.Sprintf("%d 0 R", pageObjID))
	}
	writeObj(2, fmt.Sprintf("<< /Type /Pages /Count %d /Kids [ %s ] >>", len(pages), strings.Join(kids, " ")))

	// Object 3: Polish Central European Font Encoding
	encodingDiffs := `[ 132 /quotedblbase /ellipsis 140 /Sacute 143 /Zacute 148 /quotedblright /bullet /endash /emdash 156 /sacute 159 /zacute 163 /Lslash 165 /Aogonek 175 /Zdotaccent 179 /lslash 185 /aogonek 191 /zdotaccent 198 /Cacute 202 /Eogonek 209 /Nacute 211 /Oacute 230 /cacute 234 /eogonek 241 /nacute 243 /oacute ]`
	writeObj(3, fmt.Sprintf("<< /Type /Encoding /BaseEncoding /WinAnsiEncoding /Differences %s >>", encodingDiffs))

	// Object 4: ToUnicode CMap stream
	cmapData := buildPolishToUnicodeCMap()
	writeObj(4, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(cmapData), cmapData))

	// Object 5 & 6: Courier & Courier-Bold fonts
	writeObj(5, "<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding 3 0 R /ToUnicode 4 0 R >>")
	writeObj(6, "<< /Type /Font /Subtype /Type1 /BaseFont /Courier-Bold /Encoding 3 0 R /ToUnicode 4 0 R >>")

	for i, pg := range pages {
		pageObjID := 7 + i*2
		contentObjID := pageObjID + 1

		var streamBuf strings.Builder
		streamBuf.WriteString("BT\n")
		for _, run := range pg.runs {
			fontRef := "/F1"
			if run.bold {
				fontRef = "/F2"
			}
			sz := run.size
			if sz <= 0 {
				sz = 12.0
			}
			fmt.Fprintf(&streamBuf, "%s %.2f Tf\n1 0 0 1 %.2f %.2f Tm\n%s Tj\n",
				fontRef, sz, run.x, run.y, pdfLiteralString(run.text))
		}
		streamBuf.WriteString("ET")
		contentStr := streamBuf.String()

		writeObj(pageObjID, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [ 0 0 %.2f %.2f ] /Resources << /Font << /F1 5 0 R /F2 6 0 R >> >> /Contents %d 0 R >>",
			pageWidth, pageHeight, contentObjID,
		))
		writeObj(contentObjID, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(contentStr), contentStr))
	}

	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", totalObjs+1)
	for id := 1; id <= totalObjs; id++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[id])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", totalObjs+1, xrefOffset)
	return buf.Bytes()
}

type pdfLinearPageWriter struct {
	pageWidth   float64
	pageHeight  float64
	leftX       float64
	rightMargin float64
	topMargin   float64
	lineHeight  float64
	maxLines    int
	pages       []pdfPage
	curPage     pdfPage
	curLine     int
	pageNo      int
}

func newPDFLinearPageWriter(pageWidth, pageHeight, leftX, rightMargin, topMargin, lineHeight float64, maxLines int) *pdfLinearPageWriter {
	return &pdfLinearPageWriter{
		pageWidth:   pageWidth,
		pageHeight:  pageHeight,
		leftX:       leftX,
		rightMargin: rightMargin,
		topMargin:   topMargin,
		lineHeight:  lineHeight,
		maxLines:    maxLines,
		pageNo:      1,
	}
}

func (w *pdfLinearPageWriter) lineY(lineIdx int) float64 {
	return w.pageHeight - w.topMargin - float64(lineIdx)*w.lineHeight
}

func (w *pdfLinearPageWriter) flushPage() {
	if len(w.curPage.runs) == 0 {
		return
	}
	w.pages = append(w.pages, w.curPage)
	w.curPage = pdfPage{}
	w.curLine = 0
	w.pageNo++
	numStr := fmt.Sprintf("%d.", w.pageNo)
	numW := float64(utf8.RuneCountInString(numStr)) * 7.2
	w.curPage.runs = append(w.curPage.runs, pdfTextRun{
		x:    w.pageWidth - w.rightMargin - numW,
		y:    w.pageHeight - 36.0,
		bold: false,
		size: 11.0,
		text: numStr,
	})
}

func (w *pdfLinearPageWriter) ensureLines(n int) {
	if w.curLine+n > w.maxLines {
		w.flushPage()
	}
}

func (w *pdfLinearPageWriter) blankLine() {
	if w.curLine > 0 {
		w.curLine++
		if w.curLine >= w.maxLines {
			w.flushPage()
		}
	}
}

func (w *pdfLinearPageWriter) writeWrapped(text string, x float64, maxChars int, bold bool, size float64) {
	for _, ln := range WrapCourierText(text, maxChars) {
		w.ensureLines(1)
		w.curPage.runs = append(w.curPage.runs, pdfTextRun{
			x:    x,
			y:    w.lineY(w.curLine),
			bold: bold,
			size: size,
			text: ln,
		})
		w.curLine++
	}
}

func (w *pdfLinearPageWriter) bytes() []byte {
	if len(w.curPage.runs) > 0 {
		w.pages = append(w.pages, w.curPage)
		w.curPage = pdfPage{}
	}
	if len(w.pages) == 0 {
		w.pages = append(w.pages, pdfPage{})
	}
	return serializePDFDocument(w.pages, w.pageWidth, w.pageHeight)
}

// ExportScreenplayTreatmentPDF generates a PISF/production-ready A4/Letter Treatment & Scene Outline PDF.
func ExportScreenplayTreatmentPDF(b *Book) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf("brak aktywnego projektu")
	}
	normStd := b.NormalizedStandard()
	isPL := normStd == "pl"
	pageW, pageH, maxLines := 595.28, 841.89, CourierLinesPerPageA4
	if normStd == "us" {
		pageW, pageH, maxLines = 612.0, 792.0, CourierLinesPerPageLetter
	}
	w := newPDFLinearPageWriter(pageW, pageH, 72.0, 72.0, 64.0, 13.0, maxLines)

	title := strings.ToUpper(strings.TrimSpace(b.Title))
	if title == "" {
		title = "BEZ TYTUŁU"
	}
	sub := "TREATMENT / DRABINKA SCENARIUSZOWA"
	if !isPL {
		sub = "TREATMENT / SCENE OUTLINE"
	}
	w.writeWrapped(title, 72.0, 60, true, 14.0)
	w.writeWrapped(sub+" — "+strings.TrimSpace(b.Author), 72.0, 62, false, 11.0)
	if desc := strings.TrimSpace(b.Description); desc != "" {
		w.blankLine()
		w.writeWrapped(desc, 72.0, 62, false, 11.0)
	}
	w.blankLine()

	scenes := LinearizeScreenplayScenes(b)
	lastAct := ""
	for i, sc := range scenes {
		actName := strings.TrimSpace(sc.Chapter)
		if actName != "" && !strings.EqualFold(actName, lastAct) {
			w.blankLine()
			w.ensureLines(3)
			w.writeWrapped(strings.ToUpper(actName), 72.0, 60, true, 12.0)
			lastAct = actName
		}
		w.ensureLines(3)
		slug := strings.ToUpper(strings.TrimSpace(sc.Slugline))
		if slug == "" {
			slug = strings.ToUpper(sc.DisplayName())
		}
		metrics := ComputeSceneScriptMetrics(sc, i+1, normStd)
		hdr := fmt.Sprintf("%d. %s (%s)", i+1, slug, metrics.FormattedLength)
		w.writeWrapped(hdr, 72.0, 62, true, 11.0)

		syn := strings.TrimSpace(sc.Synopsis)
		if syn == "" {
			syn = strings.Join(strings.Fields(sc.Text), " ")
			runes := []rune(syn)
			if len(runes) > 320 {
				syn = string(runes[:320]) + "..."
			}
		}
		if syn != "" {
			w.writeWrapped(syn, 86.0, 58, false, 11.0)
		}
		w.blankLine()
	}
	return w.bytes(), nil
}
