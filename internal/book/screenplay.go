package book

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"mojra/internal/i18n"
)

const (
	ScriptStandardPL               = "pl"
	ScreenplayInsertDialogue       = "dialogue"
	ScreenplayInsertParenthetical  = "parenthetical"
	ScreenplayInsertAction         = "action"
	ScreenplayInsertDialogueOff    = "dialogue_off"
	ScreenplayInsertDialogueVO     = "dialogue_vo"
	ScreenplayInsertTransitionCut  = "transition_cut"
	ScreenplayInsertTransitionFade = "transition_fade"
	ScreenplayInsertSubslug        = "subslug"
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

// ScreenplayInsertPill represents a quick-insert screenplay block button in the editor.
type ScreenplayInsertPill struct {
	Name          string
	Kind          string
	Label         string
	Hint          string
	Snippet       string
	SelectTarget  string
	DoubleNewline bool
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
	FirstSceneID       string
	DayNightBadge      string
}

// CharacterStat aggregates production breakdown statistics for a character role.
type CharacterStat struct {
	Name              string
	DialogueCues      int
	WordCount         int
	SpeakingSceneIDs  []string
	SilentSceneIDs    []string
	TotalScenes       int
	SpeakingScenes    int
	SilentScenes      int
	LocationsVisited  []string
	EstimatedCallDays int
	FirstSceneID      string
}

// ActStat aggregates page/scene metrics for an Act (chapter in screenplay mode).
type ActStat struct {
	ID              string
	Name            string
	SceneCount      int
	TotalEighths    int
	FormattedLength string
	SharePct        int
	IsDominant      bool
	IsEmpty         bool
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
	return strings.TrimSuffix(FormatEighths(eighths), " str.")
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
	for step := 0; step < 16 && trimmed != ""; step++ {
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

// ScreenplayBlockSnippet returns the localized screenplay snippet, initial selection target,
// and whether a blank line separator should precede the snippet.
func ScreenplayBlockSnippet(standard, kind, charName string, lang i18n.Lang) (snippet, selectTarget string, doubleNewline bool) {
	stdLang := i18n.LangEN
	if NormalizeScriptStandard(standard) == ScriptStandardPL {
		stdLang = i18n.LangPL
	}
	if charName != "" {
		dlgLine := i18n.T(lang, i18n.TrSpInsDialogue)
		return charName + "\n" + dlgLine, dlgLine, true
	}
	switch kind {
	case ScreenplayInsertDialogue:
		charHdr := i18n.T(lang, i18n.TrSpInsCharacter)
		return charHdr + "\n" + i18n.T(lang, i18n.TrSpInsDialogue), charHdr, true
	case ScreenplayInsertParenthetical:
		return i18n.T(lang, i18n.TrSpInsParen), i18n.T(lang, i18n.TrSpInsParenSel), false
	case ScreenplayInsertAction:
		actLine := i18n.T(lang, i18n.TrSpInsAction)
		return actLine, actLine, true
	case ScreenplayInsertDialogueOff:
		charHdr := i18n.T(lang, i18n.TrSpInsCharacter)
		return charHdr + i18n.T(stdLang, i18n.TrSpInsOffHdr) + i18n.T(lang, i18n.TrSpInsOffLine), charHdr, true
	case ScreenplayInsertDialogueVO:
		charHdr := i18n.T(lang, i18n.TrSpInsCharacter)
		return charHdr + i18n.T(lang, i18n.TrSpInsVOHdr) + i18n.T(lang, i18n.TrSpInsVOLine), charHdr, true
	case ScreenplayInsertTransitionCut:
		return i18n.T(stdLang, i18n.TrSpInsCut), "", true
	case ScreenplayInsertTransitionFade:
		return i18n.T(stdLang, i18n.TrSpInsFade), "", true
	case ScreenplayInsertSubslug:
		return i18n.T(stdLang, i18n.TrSpInsSubslug), i18n.T(stdLang, i18n.TrSpInsLocation), true
	default:
		return "", "", false
	}
}
