package book

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ScreenplayBlockKind classifies elements inside a screenplay scene.
type ScreenplayBlockKind string

const (
	BlockSlugline      ScreenplayBlockKind = "slugline"
	BlockAction        ScreenplayBlockKind = "action"
	BlockCharacter     ScreenplayBlockKind = "character"
	BlockParenthetical ScreenplayBlockKind = "parenthetical"
	BlockDialogue      ScreenplayBlockKind = "dialogue"
	BlockTransition    ScreenplayBlockKind = "transition"
)

// ScreenplayBlock represents a single parsed screenplay block within a scene.
type ScreenplayBlock struct {
	Kind          ScreenplayBlockKind
	Text          string
	CharacterName string
	CharacterExt  string
	WrappedLines  int
}

// SluglineInfo holds the deterministically parsed components of a scene slugline.
type SluglineInfo struct {
	Raw            string
	IntExt         string
	Location       string
	TimeOfDay      string
	HasValidPrefix bool
	HasTimeOfDay   bool
	IsInt          bool
	IsExt          bool
	IsDay          bool
	IsNight        bool
}

// SceneScriptMetrics holds deterministic page/eighths and timing metrics for a single scene.
type SceneScriptMetrics struct {
	SceneID          string
	SceneNumber      int
	Lines            int
	Eighths          int
	FormattedLength  string
	EstimatedSeconds int
}

// LocationStat aggregates production breakdown statistics for a single location.
type LocationStat struct {
	Name               string
	IntExt             string
	SceneCount         int
	TotalEighths       int
	FormattedLength    string
	DayScenes          int
	NightScenes        int
	IntScenes          int
	ExtScenes          int
	EstimatedShootDays int
	SceneIDs           []string
}

// CharacterStat aggregates production breakdown statistics for a character role.
type CharacterStat struct {
	Name              string
	DialogueCues      int
	WordCount         int
	SpeakingSceneIDs  []string
	SilentSceneIDs    []string
	TotalScenes       int
	LocationsVisited  []string
	EstimatedCallDays int
}

// ActStat aggregates page/scene metrics for an Act (chapter in screenplay mode).
type ActStat struct {
	ID              string
	Name            string
	SceneCount      int
	TotalEighths    int
	FormattedLength string
	SharePct        int
}

// ScreenplayBreakdown contains the complete deterministic breakdown of a screenplay project.
type ScreenplayBreakdown struct {
	Standard              string
	TotalScenes           int
	TotalLines            int
	TotalEighths          int
	TotalPagesFormatted   string
	EstimatedMinutes      int
	EstimatedRuntimeLabel string
	IntScenes             int
	ExtScenes             int
	IntPct                int
	ExtPct                int
	DayScenes             int
	NightScenes           int
	DayPct                int
	NightPct              int
	EstimatedShootDays    int
	ToolbarSummary        string
	Acts                  []ActStat
	Characters            []CharacterStat
	Locations             []LocationStat
	SceneMetrics          map[string]SceneScriptMetrics
}

const (
	CourierLinesPerPageA4     = 54
	CourierLinesPerPageLetter = 55

	CourierMaxColsAction        = 60
	CourierMaxColsSlugline      = 60
	CourierMaxColsDialogue      = 35
	CourierMaxColsParenthetical = 25
	CourierMaxColsCharacter     = 38
	CourierMaxColsTransition    = 60
)

type sluglinePrefixSpec struct {
	token string
	norm  string
	isInt bool
	isExt bool
}

var (
	charParenSuffixRe = regexp.MustCompile(`\s*\([^)]*\)\s*$`)

	sluglinePrefixes = []sluglinePrefixSpec{
		{"WN./PL.", "WN./PL.", true, true},
		{"PL./WN.", "WN./PL.", true, true},
		{"WN/PL.", "WN./PL.", true, true},
		{"WN/PL", "WN./PL.", true, true},
		{"INT./EXT.", "INT./EXT.", true, true},
		{"EXT./INT.", "INT./EXT.", true, true},
		{"INT/EXT.", "INT./EXT.", true, true},
		{"INT/EXT", "INT./EXT.", true, true},
		{"I/E.", "INT./EXT.", true, true},
		{"I/E", "INT./EXT.", true, true},
		{"WN.", "WN.", true, false},
		{"WN ", "WN.", true, false},
		{"PL.", "PL.", false, true},
		{"PL ", "PL.", false, true},
		{"INT.", "INT.", true, false},
		{"INT ", "INT.", true, false},
		{"EXT.", "EXT.", false, true},
		{"EXT ", "EXT.", false, true},
	}

	sluglineTimeSeparators = []string{" - ", " – ", " — ", "- ", " -"}

	sluglineNightTokens = []string{
		"NOC", "WIECZ", "ZMIERZCH", "PÓŁNOC", "POLNOC",
		"NIGHT", "EVENING", "DUSK", "SUNSET", "MIDNIGHT",
	}
)

// DefaultSluglineForStandard returns the default slugline template for a regional screenplay standard.
func DefaultSluglineForStandard(standard string) string {
	if NormalizeScriptStandard(standard) == "pl" {
		return "WN. LOKACJA - DZIEŃ"
	}
	return "INT. LOCATION - DAY"
}

// LinesPerPageForStandard returns 54 for PL/EU (A4) or 55 for US (Letter).
func LinesPerPageForStandard(standard string) int {
	if NormalizeScriptStandard(standard) == "us" {
		return CourierLinesPerPageLetter
	}
	return CourierLinesPerPageA4
}

// FormatEighths formats a count of 1/8th pages into industry notation (e.g. "6/8 str.", "1 2/8 str.", "2 str.").
func FormatEighths(eighths int) string {
	if eighths <= 0 {
		eighths = 1
	}
	whole := eighths / 8
	rem := eighths % 8
	if whole == 0 {
		return fmt.Sprintf("%d/8 str.", rem)
	}
	if rem == 0 {
		return fmt.Sprintf("%d str.", whole)
	}
	return fmt.Sprintf("%d %d/8 str.", whole, rem)
}

// FormatEighthsCompact formats eighths concisely for compact badges (e.g. "6/8", "1 2/8", "2").
func FormatEighthsCompact(eighths int) string {
	if eighths <= 0 {
		eighths = 1
	}
	whole := eighths / 8
	rem := eighths % 8
	if whole == 0 {
		return fmt.Sprintf("%d/8", rem)
	}
	if rem == 0 {
		return fmt.Sprintf("%d", whole)
	}
	return fmt.Sprintf("%d %d/8", whole, rem)
}

// IsSluglineLine checks if a trimmed line matches a Polish/European/US or forced Fountain slugline.
func IsSluglineLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, ".") && !strings.HasPrefix(trimmed, "..") && len(trimmed) > 1 {
		r, _ := utf8.DecodeRuneInString(trimmed[1:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	info := ParseSlugline(trimmed)
	return info.HasValidPrefix
}

// ExtractLeadingSlugline checks if the first non-empty line of raw text is a slugline.
// If so, it returns the normalized slugline, the remaining body text, and true.
func ExtractLeadingSlugline(raw string) (slugline string, remaining string, ok bool) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	firstIdx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			firstIdx = i
			break
		}
	}
	if firstIdx < 0 {
		return "", raw, false
	}
	firstLine := strings.TrimSpace(lines[firstIdx])
	if !IsSluglineLine(firstLine) {
		return "", raw, false
	}
	if strings.HasPrefix(firstLine, ".") && !strings.HasPrefix(firstLine, "..") {
		firstLine = strings.TrimSpace(firstLine[1:])
	}
	rest := strings.TrimSpace(strings.Join(lines[firstIdx+1:], "\n"))
	return strings.ToUpper(firstLine), rest, true
}

// ParseSlugline deterministically parses a scene heading into Int/Ext prefix, Location name, and Time of Day.
func ParseSlugline(raw string) SluglineInfo {
	clean := strings.TrimSpace(raw)
	if strings.HasPrefix(clean, ".") && !strings.HasPrefix(clean, "..") {
		clean = strings.TrimSpace(clean[1:])
	}
	info := SluglineInfo{Raw: clean}
	if clean == "" {
		return info
	}

	upper := strings.ToUpper(clean)
	rest := upper
	for _, p := range sluglinePrefixes {
		if strings.HasPrefix(upper, p.token) {
			after := strings.TrimSpace(clean[len(p.token):])
			after = strings.TrimLeft(after, ".- ")
			info.HasValidPrefix = true
			info.IntExt = p.norm
			info.IsInt = p.isInt
			info.IsExt = p.isExt
			rest = strings.ToUpper(strings.TrimSpace(after))
			break
		}
	}

	// Split Location and TimeOfDay on the last " - " or " – " or " — "
	sepIdx := -1
	sepLen := 0
	for _, sep := range sluglineTimeSeparators {
		if idx := strings.LastIndex(rest, sep); idx > 0 {
			sepIdx = idx
			sepLen = len(sep)
			break
		}
	}

	if sepIdx > 0 {
		info.Location = strings.TrimSpace(rest[:sepIdx])
		info.TimeOfDay = strings.TrimSpace(rest[sepIdx+sepLen:])
	} else {
		info.Location = strings.TrimSpace(rest)
	}

	if info.TimeOfDay != "" {
		info.HasTimeOfDay = true
		tod := info.TimeOfDay
		for _, tok := range sluglineNightTokens {
			if strings.Contains(tod, tok) {
				info.IsNight = true
				break
			}
		}
		if !info.IsNight {
			info.IsDay = true
		}
	}

	return info
}

func isTransitionLine(trimmed string, prevBlank bool) (string, bool) {
	if trimmed == "" {
		return "", false
	}
	if strings.HasPrefix(trimmed, ">") && !strings.HasSuffix(trimmed, "<") {
		t := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
		if t != "" {
			return strings.ToUpper(t), true
		}
	}
	if !prevBlank {
		return "", false
	}
	upper := strings.ToUpper(trimmed)
	if upper != trimmed || !hasAnyLetter(trimmed) || utf8.RuneCountInString(upper) > 36 {
		return "", false
	}
	if strings.HasSuffix(upper, " DO:") || strings.HasSuffix(upper, " TO:") || strings.HasSuffix(upper, " IN:") || strings.HasSuffix(upper, " OUT:") || strings.HasSuffix(upper, " OUT.") {
		return upper, true
	}
	return "", false
}

func hasAnyLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func isAllUppercaseCue(s string) bool {
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			if unicode.IsLower(r) {
				return false
			}
		}
	}
	return hasLetter
}

// SplitCharacterCue splits a character cue like "ŁUKASZ (OFF)" into base name "ŁUKASZ" and extension "(OFF)".
func SplitCharacterCue(raw string) (baseName string, ext string) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "@")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return "", ""
	}
	var extensions []string
	for {
		loc := charParenSuffixRe.FindStringIndex(trimmed)
		if loc == nil || loc[0] <= 0 {
			break
		}
		extPart := strings.TrimSpace(trimmed[loc[0]:])
		extensions = append([]string{extPart}, extensions...)
		trimmed = strings.TrimSpace(trimmed[:loc[0]])
	}
	return strings.ToUpper(trimmed), strings.Join(extensions, " ")
}

func isCharacterCueLine(trimmed string, prevBlank bool, hasNextNonEmpty bool) (displayCue, baseName, ext string, ok bool) {
	if trimmed == "" {
		return "", "", "", false
	}
	if strings.HasPrefix(trimmed, "@") && len(trimmed) > 1 {
		cue := strings.TrimSpace(trimmed[1:])
		if cue != "" {
			base, e := SplitCharacterCue(cue)
			if base != "" {
				disp := base
				if e != "" {
					disp = base + " " + e
				}
				return disp, base, e, true
			}
		}
	}
	if !prevBlank || !hasNextNonEmpty {
		return "", "", "", false
	}
	if IsSluglineLine(trimmed) {
		return "", "", "", false
	}
	if _, isTrans := isTransitionLine(trimmed, prevBlank); isTrans {
		return "", "", "", false
	}
	if strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
		return "", "", "", false
	}
	if utf8.RuneCountInString(trimmed) > CourierMaxColsCharacter {
		return "", "", "", false
	}
	base, e := SplitCharacterCue(trimmed)
	if base == "" || !isAllUppercaseCue(base) {
		return "", "", "", false
	}
	// Reject lines ending with sentence punctuation typical of action lines unless followed by extension
	if e == "" && (strings.HasSuffix(base, ".") || strings.HasSuffix(base, "!") || strings.HasSuffix(base, "?") || strings.HasSuffix(base, ":")) {
		return "", "", "", false
	}
	disp := base
	if e != "" {
		disp = base + " " + e
	}
	return disp, base, e, true
}

func wrapCourierLineCount(text string, maxCols int) int {
	if maxCols <= 0 {
		maxCols = CourierMaxColsAction
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	paragraphs := strings.Split(strings.ReplaceAll(trimmed, "\r\n", "\n"), "\n")
	totalLines := 0
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			totalLines++
			continue
		}
		words := strings.Fields(p)
		if len(words) == 0 {
			totalLines++
			continue
		}
		lines := 1
		col := 0
		for _, w := range words {
			wLen := utf8.RuneCountInString(w)
			if col == 0 {
				col = wLen
				for col > maxCols {
					lines++
					col -= maxCols
				}
			} else if col+1+wLen <= maxCols {
				col += 1 + wLen
			} else {
				lines++
				col = wLen
				for col > maxCols {
					lines++
					col -= maxCols
				}
			}
		}
		totalLines += lines
	}
	if totalLines < 1 {
		totalLines = 1
	}
	return totalLines
}

// WrapCourierText splits text into physical lines of at most maxCols runes for Courier pagination/PDF rendering.
func WrapCourierText(text string, maxCols int) []string {
	if maxCols <= 0 {
		maxCols = CourierMaxColsAction
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	paragraphs := strings.Split(strings.ReplaceAll(trimmed, "\r\n", "\n"), "\n")
	var out []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			out = append(out, "")
			continue
		}
		words := strings.Fields(p)
		if len(words) == 0 {
			continue
		}
		var cur strings.Builder
		col := 0
		for _, w := range words {
			wRunes := []rune(w)
			wLen := len(wRunes)
			if col == 0 {
				for wLen > maxCols {
					out = append(out, string(wRunes[:maxCols]))
					wRunes = wRunes[maxCols:]
					wLen = len(wRunes)
				}
				cur.WriteString(string(wRunes))
				col = wLen
			} else if col+1+wLen <= maxCols {
				cur.WriteByte(' ')
				cur.WriteString(w)
				col += 1 + wLen
			} else {
				out = append(out, cur.String())
				cur.Reset()
				for wLen > maxCols {
					out = append(out, string(wRunes[:maxCols]))
					wRunes = wRunes[maxCols:]
					wLen = len(wRunes)
				}
				cur.WriteString(string(wRunes))
				col = wLen
			}
		}
		if cur.Len() > 0 {
			out = append(out, cur.String())
		}
	}
	return out
}

// ParseScreenplayText parses Fountain-compatible screenplay text into structured blocks.
func ParseScreenplayText(raw string, standard string) []ScreenplayBlock {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	var blocks []ScreenplayBlock
	var actionLines []string
	var dialogueLines []string
	activeCharacter := ""
	activeCharacterExt := ""
	inDialogue := false
	prevBlank := true

	flushAction := func() {
		if len(actionLines) == 0 {
			return
		}
		txt := strings.TrimSpace(strings.Join(actionLines, "\n"))
		if txt != "" {
			blocks = append(blocks, ScreenplayBlock{
				Kind:         BlockAction,
				Text:         txt,
				WrappedLines: wrapCourierLineCount(txt, CourierMaxColsAction),
			})
		}
		actionLines = nil
	}

	flushDialogue := func() {
		if len(dialogueLines) == 0 {
			return
		}
		txt := strings.TrimSpace(strings.Join(dialogueLines, "\n"))
		if txt != "" {
			blocks = append(blocks, ScreenplayBlock{
				Kind:          BlockDialogue,
				Text:          txt,
				CharacterName: activeCharacter,
				CharacterExt:  activeCharacterExt,
				WrappedLines:  wrapCourierLineCount(txt, CourierMaxColsDialogue),
			})
		}
		dialogueLines = nil
	}

	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			flushDialogue()
			flushAction()
			inDialogue = false
			activeCharacter = ""
			activeCharacterExt = ""
			prevBlank = true
			continue
		}

		hasNextNonEmpty := i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != ""

		// 1. Check Slugline (when preceded by blank line or at start)
		if prevBlank && IsSluglineLine(trimmed) {
			flushDialogue()
			flushAction()
			inDialogue = false
			slugText := trimmed
			if strings.HasPrefix(slugText, ".") && !strings.HasPrefix(slugText, "..") {
				slugText = strings.TrimSpace(slugText[1:])
			}
			slugText = strings.ToUpper(slugText)
			blocks = append(blocks, ScreenplayBlock{
				Kind:         BlockSlugline,
				Text:         slugText,
				WrappedLines: wrapCourierLineCount(slugText, CourierMaxColsSlugline),
			})
			prevBlank = false
			continue
		}

		// 2. Check Transition
		if transText, ok := isTransitionLine(trimmed, prevBlank); ok {
			flushDialogue()
			flushAction()
			inDialogue = false
			blocks = append(blocks, ScreenplayBlock{
				Kind:         BlockTransition,
				Text:         transText,
				WrappedLines: wrapCourierLineCount(transText, CourierMaxColsTransition),
			})
			prevBlank = false
			continue
		}

		// 3. Check Character Cue
		if dispCue, baseName, ext, ok := isCharacterCueLine(trimmed, prevBlank, hasNextNonEmpty); ok {
			flushDialogue()
			flushAction()
			inDialogue = true
			activeCharacter = baseName
			activeCharacterExt = ext
			blocks = append(blocks, ScreenplayBlock{
				Kind:          BlockCharacter,
				Text:          dispCue,
				CharacterName: baseName,
				CharacterExt:  ext,
				WrappedLines:  1,
			})
			prevBlank = false
			continue
		}

		// 4. Inside dialogue block: check Parenthetical or Dialogue continuation
		if inDialogue {
			if strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
				flushDialogue()
				blocks = append(blocks, ScreenplayBlock{
					Kind:          BlockParenthetical,
					Text:          trimmed,
					CharacterName: activeCharacter,
					CharacterExt:  activeCharacterExt,
					WrappedLines:  wrapCourierLineCount(trimmed, CourierMaxColsParenthetical),
				})
			} else {
				dialogueLines = append(dialogueLines, trimmed)
			}
			prevBlank = false
			continue
		}

		// 5. Default: Action block
		actionLines = append(actionLines, trimmed)
		prevBlank = false
	}

	flushDialogue()
	flushAction()
	return blocks
}

// ParseScreenplaySceneBlocks combines the scene's Slugline header and Text body into screenplay blocks without duplicating the heading.
func ParseScreenplaySceneBlocks(sc *Scene, standard string) []ScreenplayBlock {
	if sc == nil {
		return nil
	}
	bodyBlocks := ParseScreenplayText(sc.Text, standard)
	slug := strings.ToUpper(strings.TrimSpace(sc.Slugline))
	if strings.HasPrefix(slug, ".") && !strings.HasPrefix(slug, "..") {
		slug = strings.TrimSpace(slug[1:])
	}
	if slug == "" {
		return bodyBlocks
	}
	if len(bodyBlocks) > 0 && bodyBlocks[0].Kind == BlockSlugline && strings.EqualFold(strings.TrimSpace(bodyBlocks[0].Text), slug) {
		bodyBlocks[0].Text = slug
		return bodyBlocks
	}
	headBlock := ScreenplayBlock{
		Kind:         BlockSlugline,
		Text:         slug,
		WrappedLines: wrapCourierLineCount(slug, CourierMaxColsSlugline),
	}
	out := make([]ScreenplayBlock, 0, len(bodyBlocks)+1)
	out = append(out, headBlock)
	out = append(out, bodyBlocks...)
	return out
}

// ComputeSceneScriptMetrics calculates deterministic Courier line count, eighths of a page, and runtime in seconds.
func ComputeSceneScriptMetrics(sc *Scene, sceneNum int, standard string) SceneScriptMetrics {
	normStd := NormalizeScriptStandard(standard)
	linesPerPage := LinesPerPageForStandard(normStd)
	if sc == nil {
		return SceneScriptMetrics{
			SceneNumber:      sceneNum,
			Lines:            1,
			Eighths:          1,
			FormattedLength:  FormatEighths(1),
			EstimatedSeconds: 8,
		}
	}
	blocks := ParseScreenplaySceneBlocks(sc, normStd)
	totalLines := 0
	for i, blk := range blocks {
		if i > 0 {
			switch blk.Kind {
			case BlockSlugline, BlockAction, BlockCharacter, BlockTransition:
				totalLines++ // Blank spacing line before block
			}
		}
		w := blk.WrappedLines
		if w < 1 {
			w = 1
		}
		totalLines += w
	}
	if totalLines < 1 {
		totalLines = 1
	}
	eighths := int(math.Round(float64(totalLines) / (float64(linesPerPage) / 8.0)))
	if eighths < 1 {
		eighths = 1
	}
	estSec := int(math.Round((float64(eighths) / 8.0) * 60.0))
	if estSec < 5 {
		estSec = 5
	}
	return SceneScriptMetrics{
		SceneID:          sc.ID,
		SceneNumber:      sceneNum,
		Lines:            totalLines,
		Eighths:          eighths,
		FormattedLength:  FormatEighths(eighths),
		EstimatedSeconds: estSec,
	}
}

func levenshteinRunes(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	la := len(ra)
	lb := len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	if la-lb > 2 || lb-la > 2 {
		return 3
	}
	// Damerau-Levenshtein (optimal string alignment) so adjacent transpositions (e.g. MARKE vs MAREK) have distance 1
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 0
			if ra[i-1] != rb[j-1] {
				cost = 1
			}
			del := d[i-1][j] + 1
			ins := d[i][j-1] + 1
			sub := d[i-1][j-1] + cost
			m := del
			if ins < m {
				m = ins
			}
			if sub < m {
				m = sub
			}
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				trans := d[i-2][j-2] + 1
				if trans < m {
					m = trans
				}
			}
			d[i][j] = m
		}
	}
	return d[la][lb]
}

// ValidateScreenplayGuards runs the 5 deterministic screenplay format guards and appends warnings to Scene.Warnings and Book.Warnings.
func ValidateScreenplayGuards(b *Book) {
	if b == nil || !b.IsScreenplay() {
		return
	}
	normStd := b.NormalizedStandard()

	type charOccurrence struct {
		count      int
		firstScene *Scene
	}
	charCounts := make(map[string]*charOccurrence)
	var charOrder []string

	actEighths := make(map[string]int)
	totalEighths := 0

	for i, sc := range b.Scenes {
		metrics := ComputeSceneScriptMetrics(sc, i+1, normStd)
		totalEighths += metrics.Eighths
		actKey := strings.ToLower(strings.TrimSpace(sc.Chapter))
		actEighths[actKey] += metrics.Eighths

		// Guard 1: Slugline Guard
		slugText := strings.TrimSpace(sc.Slugline)
		blocks := ParseScreenplaySceneBlocks(sc, normStd)
		if slugText == "" && len(blocks) > 0 && blocks[0].Kind == BlockSlugline {
			slugText = blocks[0].Text
		}
		if slugText == "" {
			msg := fmt.Sprintf("Scena „%s”: brak nagłówka sceny (Slugline, np. WN. LOKACJA - DZIEŃ)", sc.DisplayName())
			sc.Warnings = append(sc.Warnings, msg)
			b.Warnings = append(b.Warnings, msg)
		} else {
			info := ParseSlugline(slugText)
			if !info.HasValidPrefix {
				msg := fmt.Sprintf("Scena „%s”: nagłówek sceny „%s” nie zaczyna się od WN./PL. ani INT./EXT.", sc.DisplayName(), slugText)
				sc.Warnings = append(sc.Warnings, msg)
				b.Warnings = append(b.Warnings, msg)
			} else if !info.HasTimeOfDay || info.Location == "" {
				msg := fmt.Sprintf("Scena „%s”: nagłówek sceny „%s” nie zawiera pory dnia po myślniku (np. - DZIEŃ lub - NOC)", sc.DisplayName(), slugText)
				sc.Warnings = append(sc.Warnings, msg)
				b.Warnings = append(b.Warnings, msg)
			}
		}

		// Guard 2: Scene Length Guard (> 3 pages = > 24/8)
		if metrics.Eighths > 24 {
			msg := fmt.Sprintf("Scena „%s”: długość sceny przekracza 3 strony (%s) — rozważ podział sceny", sc.DisplayName(), metrics.FormattedLength)
			sc.Warnings = append(sc.Warnings, msg)
			b.Warnings = append(b.Warnings, msg)
		}

		// Guard 3: Monologue Guard (> 8 wrapped lines or > 120 words in a single continuous dialogue block)
		for _, blk := range blocks {
			if blk.Kind == BlockCharacter && blk.CharacterName != "" {
				entry, exists := charCounts[blk.CharacterName]
				if !exists {
					entry = &charOccurrence{firstScene: sc}
					charCounts[blk.CharacterName] = entry
					charOrder = append(charOrder, blk.CharacterName)
				}
				entry.count++
			}
			if blk.Kind == BlockDialogue {
				wordCount := len(strings.Fields(blk.Text))
				if blk.WrappedLines > 8 || wordCount > 120 {
					who := blk.CharacterName
					if who == "" {
						who = "POSTAĆ"
					}
					msg := fmt.Sprintf("Scena „%s”: zbyt długi ciągły monolog postaci %s (%d wierszy) — dodaj didaskalia lub reakcję", sc.DisplayName(), who, blk.WrappedLines)
					sc.Warnings = append(sc.Warnings, msg)
					b.Warnings = append(b.Warnings, msg)
				}
			}
		}
	}

	// Guard 4: Character Typo Guard (Levenshtein distance 1 from a character with >= 3 cues, appearing only once)
	for _, rareName := range charOrder {
		rareEntry := charCounts[rareName]
		if rareEntry.count != 1 || utf8.RuneCountInString(rareName) < 3 {
			continue
		}
		for _, freqName := range charOrder {
			if freqName == rareName {
				continue
			}
			freqEntry := charCounts[freqName]
			if freqEntry.count < 3 {
				continue
			}
			if levenshteinRunes(rareName, freqName) == 1 {
				msg := fmt.Sprintf("Scena „%s”: możliwa literówka w nazwie postaci „%s” (podobne do „%s”, które występuje %d razy)", rareEntry.firstScene.DisplayName(), rareName, freqName, freqEntry.count)
				rareEntry.firstScene.Warnings = append(rareEntry.firstScene.Warnings, msg)
				b.Warnings = append(b.Warnings, msg)
				break
			}
		}
	}

	// Guard 5: Act & Script Pacing Guard
	if len(b.Chapters) >= 3 && totalEighths >= 8 {
		firstAct := b.Chapters[0]
		act1E := actEighths[strings.ToLower(strings.TrimSpace(firstAct.Name))]
		if act1E*100 > totalEighths*35 {
			pct := (act1E * 100) / totalEighths
			msg := fmt.Sprintf("Balans aktów: %s stanowi %d%% objętości scenariusza (zalecane maks. 35%% dla pierwszego aktu)", firstAct.DisplayName(), pct)
			b.Warnings = append(b.Warnings, msg)
		}
	}
	if totalEighths > 1040 { // > 130 pages
		msg := fmt.Sprintf("Objętość scenariusza (%s) przekracza standardowy limit 130 stron pełnego metrażu", FormatEighths(totalEighths))
		b.Warnings = append(b.Warnings, msg)
	}
}

// AnalyzeScreenplay computes the deterministic Cast, Location, Shoot-Day, Call-Day, and Act pacing breakdown.
func AnalyzeScreenplay(b *Book) *ScreenplayBreakdown {
	normStd := "pl"
	if b != nil {
		normStd = b.NormalizedStandard()
	}
	bd := &ScreenplayBreakdown{
		Standard:     normStd,
		SceneMetrics: make(map[string]SceneScriptMetrics),
	}
	if b == nil || len(b.Scenes) == 0 {
		bd.TotalPagesFormatted = FormatEighths(1)
		bd.EstimatedMinutes = 1
		bd.EstimatedRuntimeLabel = "~1 min"
		bd.ToolbarSummary = "1/8 str. • ~1 min"
		return bd
	}

	activeScenes := LinearizeScreenplayScenes(b)
	if len(activeScenes) == 0 {
		activeScenes = b.Scenes
	}
	activeSet := make(map[string]bool, len(activeScenes))
	for _, sc := range activeScenes {
		activeSet[sc.ID] = true
	}

	bd.TotalScenes = len(activeScenes)

	type locAccum struct {
		stat     LocationStat
		sceneSet map[string]bool
	}
	locMap := make(map[string]*locAccum)
	var locOrder []string

	type charAccum struct {
		name         string
		cues         int
		words        int
		speakingSet  map[string]bool
		speakingIDs  []string
		silentSet    map[string]bool
		silentIDs    []string
		locSet       map[string]bool
		locs         []string
		clusterEight map[string]int
	}
	charMap := make(map[string]*charAccum)
	var charOrder []string

	sceneBlocksMap := make(map[string][]ScreenplayBlock, len(b.Scenes))
	sceneSlugInfo := make(map[string]SluglineInfo, len(b.Scenes))

	// Populate SceneMetrics for all scenes so editor modals always have metrics
	for i, sc := range b.Scenes {
		bd.SceneMetrics[sc.ID] = ComputeSceneScriptMetrics(sc, i+1, normStd)
	}

	// First pass on active primary path: metrics, locations, and speaking characters
	for i, sc := range activeScenes {
		metrics := ComputeSceneScriptMetrics(sc, i+1, normStd)
		bd.SceneMetrics[sc.ID] = metrics
		bd.TotalLines += metrics.Lines
		bd.TotalEighths += metrics.Eighths

		blocks := ParseScreenplaySceneBlocks(sc, normStd)
		sceneBlocksMap[sc.ID] = blocks

		slugRaw := strings.TrimSpace(sc.Slugline)
		if slugRaw == "" && len(blocks) > 0 && blocks[0].Kind == BlockSlugline {
			slugRaw = blocks[0].Text
		}
		sInfo := ParseSlugline(slugRaw)
		sceneSlugInfo[sc.ID] = sInfo

		if sInfo.IsInt {
			bd.IntScenes++
		}
		if sInfo.IsExt {
			bd.ExtScenes++
		}
		if sInfo.IsDay {
			bd.DayScenes++
		}
		if sInfo.IsNight {
			bd.NightScenes++
		}

		locName := sInfo.Location
		if locName == "" {
			locName = "NIEOKREŚLONA LOKACJA"
		}
		locKey := strings.ToUpper(strings.TrimSpace(locName))
		la, exists := locMap[locKey]
		if !exists {
			intExt := sInfo.IntExt
			if intExt == "" {
				intExt = "WN."
			}
			la = &locAccum{
				stat: LocationStat{
					Name:   locKey,
					IntExt: intExt,
				},
				sceneSet: make(map[string]bool),
			}
			locMap[locKey] = la
			locOrder = append(locOrder, locKey)
		}
		if !la.sceneSet[sc.ID] {
			la.sceneSet[sc.ID] = true
			la.stat.SceneCount++
			la.stat.SceneIDs = append(la.stat.SceneIDs, sc.ID)
		}
		la.stat.TotalEighths += metrics.Eighths
		if sInfo.IsDay {
			la.stat.DayScenes++
		}
		if sInfo.IsNight {
			la.stat.NightScenes++
		}
		if sInfo.IsInt {
			la.stat.IntScenes++
		}
		if sInfo.IsExt {
			la.stat.ExtScenes++
		}

		for _, blk := range blocks {
			if blk.Kind == BlockCharacter && blk.CharacterName != "" {
				cKey := blk.CharacterName
				ca, ok := charMap[cKey]
				if !ok {
					ca = &charAccum{
						name:         cKey,
						speakingSet:  make(map[string]bool),
						silentSet:    make(map[string]bool),
						locSet:       make(map[string]bool),
						clusterEight: make(map[string]int),
					}
					charMap[cKey] = ca
					charOrder = append(charOrder, cKey)
				}
				ca.cues++
				if !ca.speakingSet[sc.ID] {
					ca.speakingSet[sc.ID] = true
					ca.speakingIDs = append(ca.speakingIDs, sc.ID)
					clusterKey := locKey + "|" + sInfo.TimeOfDay
					ca.clusterEight[clusterKey] += metrics.Eighths
				}
				if locKey != "" && !ca.locSet[locKey] {
					ca.locSet[locKey] = true
					ca.locs = append(ca.locs, locKey)
				}
			} else if blk.Kind == BlockDialogue && blk.CharacterName != "" {
				if ca, ok := charMap[blk.CharacterName]; ok {
					ca.words += len(strings.Fields(blk.Text))
				}
			}
		}
	}

	// Second pass: detect silent (non-speaking) appearances of known characters in BlockAction
	if len(charOrder) > 0 {
		for _, sc := range activeScenes {
			blocks := sceneBlocksMap[sc.ID]
			sInfo := sceneSlugInfo[sc.ID]
			metrics := bd.SceneMetrics[sc.ID]
			locKey := strings.ToUpper(strings.TrimSpace(sInfo.Location))
			if locKey == "" {
				locKey = "NIEOKREŚLONA LOKACJA"
			}

			var actionTextBuilder strings.Builder
			for _, blk := range blocks {
				if blk.Kind == BlockAction {
					actionTextBuilder.WriteString(" ")
					actionTextBuilder.WriteString(blk.Text)
				}
			}
			actionText := strings.ToUpper(actionTextBuilder.String())
			if actionText == "" {
				continue
			}

			for _, cKey := range charOrder {
				ca := charMap[cKey]
				if ca.speakingSet[sc.ID] || ca.silentSet[sc.ID] {
					continue
				}
				if containsWholeWord(actionText, cKey) {
					ca.silentSet[sc.ID] = true
					ca.silentIDs = append(ca.silentIDs, sc.ID)
					clusterKey := locKey + "|" + sInfo.TimeOfDay
					ca.clusterEight[clusterKey] += metrics.Eighths
					if locKey != "" && !ca.locSet[locKey] {
						ca.locSet[locKey] = true
						ca.locs = append(ca.locs, locKey)
					}
				}
			}
		}
	}

	// Finalize Locations
	totalShootDays := 0
	for _, locKey := range locOrder {
		la := locMap[locKey]
		la.stat.FormattedLength = FormatEighths(la.stat.TotalEighths)
		shootDays := int(math.Ceil(float64(la.stat.TotalEighths) / 32.0))
		if shootDays < 1 {
			shootDays = 1
		}
		la.stat.EstimatedShootDays = shootDays
		totalShootDays += shootDays
		bd.Locations = append(bd.Locations, la.stat)
	}
	sort.SliceStable(bd.Locations, func(i, j int) bool {
		if bd.Locations[i].TotalEighths != bd.Locations[j].TotalEighths {
			return bd.Locations[i].TotalEighths > bd.Locations[j].TotalEighths
		}
		return bd.Locations[i].Name < bd.Locations[j].Name
	})
	bd.EstimatedShootDays = totalShootDays

	// Finalize Characters
	for _, cKey := range charOrder {
		ca := charMap[cKey]
		callDays := 0
		for _, clEighths := range ca.clusterEight {
			days := int(math.Ceil(float64(clEighths) / 32.0))
			if days < 1 {
				days = 1
			}
			callDays += days
		}
		if callDays < 1 && (len(ca.speakingIDs)+len(ca.silentIDs)) > 0 {
			callDays = 1
		}
		bd.Characters = append(bd.Characters, CharacterStat{
			Name:              ca.name,
			DialogueCues:      ca.cues,
			WordCount:         ca.words,
			SpeakingSceneIDs:  ca.speakingIDs,
			SilentSceneIDs:    ca.silentIDs,
			TotalScenes:       len(ca.speakingIDs) + len(ca.silentIDs),
			LocationsVisited:  ca.locs,
			EstimatedCallDays: callDays,
		})
	}
	sort.SliceStable(bd.Characters, func(i, j int) bool {
		if bd.Characters[i].DialogueCues != bd.Characters[j].DialogueCues {
			return bd.Characters[i].DialogueCues > bd.Characters[j].DialogueCues
		}
		return bd.Characters[i].Name < bd.Characters[j].Name
	})

	// Finalize Acts
	var actParts []string
	for _, ch := range b.Chapters {
		actE := 0
		scCount := 0
		for _, sc := range activeScenes {
			if strings.EqualFold(strings.TrimSpace(sc.Chapter), strings.TrimSpace(ch.Name)) {
				scCount++
				actE += bd.SceneMetrics[sc.ID].Eighths
			}
		}
		share := 0
		if bd.TotalEighths > 0 {
			share = (actE * 100) / bd.TotalEighths
		}
		bd.Acts = append(bd.Acts, ActStat{
			ID:              ch.ReferenceKey(),
			Name:            ch.DisplayName(),
			SceneCount:      scCount,
			TotalEighths:    actE,
			FormattedLength: FormatEighths(actE),
			SharePct:        share,
		})
		actParts = append(actParts, fmt.Sprintf("%s: %s", ch.DisplayName(), FormatEighthsCompact(actE)))
	}

	if bd.TotalEighths < 1 {
		bd.TotalEighths = 1
	}
	bd.TotalPagesFormatted = FormatEighths(bd.TotalEighths)
	estMin := int(math.Round(float64(bd.TotalEighths) / 8.0))
	if estMin < 1 {
		estMin = 1
	}
	bd.EstimatedMinutes = estMin
	bd.EstimatedRuntimeLabel = fmt.Sprintf("~%d min", estMin)

	if bd.IntScenes+bd.ExtScenes > 0 {
		tot := bd.IntScenes + bd.ExtScenes
		bd.IntPct = (bd.IntScenes * 100) / tot
		bd.ExtPct = 100 - bd.IntPct
	}
	if bd.DayScenes+bd.NightScenes > 0 {
		tot := bd.DayScenes + bd.NightScenes
		bd.DayPct = (bd.DayScenes * 100) / tot
		bd.NightPct = 100 - bd.DayPct
	}

	if len(actParts) > 0 {
		bd.ToolbarSummary = fmt.Sprintf("%s • %s • %s", bd.TotalPagesFormatted, bd.EstimatedRuntimeLabel, strings.Join(actParts, " / "))
	} else {
		bd.ToolbarSummary = fmt.Sprintf("%s • %s", bd.TotalPagesFormatted, bd.EstimatedRuntimeLabel)
	}

	return bd
}

func containsWholeWord(haystackUpper, needleUpper string) bool {
	if needleUpper == "" || haystackUpper == "" {
		return false
	}
	idx := 0
	for {
		pos := strings.Index(haystackUpper[idx:], needleUpper)
		if pos < 0 {
			return false
		}
		absStart := idx + pos
		absEnd := absStart + len(needleUpper)
		beforeOK := true
		if absStart > 0 {
			r, _ := utf8.DecodeLastRuneInString(haystackUpper[:absStart])
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				beforeOK = false
			}
		}
		afterOK := true
		if absEnd < len(haystackUpper) {
			r, _ := utf8.DecodeRuneInString(haystackUpper[absEnd:])
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				afterOK = false
			}
		}
		if beforeOK && afterOK {
			return true
		}
		idx = absStart + len(needleUpper)
		if idx >= len(haystackUpper) {
			return false
		}
	}
}
