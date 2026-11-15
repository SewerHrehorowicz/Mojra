package model

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	PlaytxtExt                     = ".playtxt"
	PlaybookExt                    = ".playbook"
	ZipExt                         = ".zip"
	BookDataFileName               = "book.playtxt"
	EditorStateFileName            = ".editor.cfg"
	DefaultStoryFile               = "story.playtxt"
	DefaultChapterFile             = "rozdzial_1.playtxt"
	DefaultActFile                 = "akt_1.playtxt"
	DefaultBookChapterName         = "Rozdział 1"
	DefaultScreenplayActName       = "Akt I"
	TargetEnd                      = "END"
	TokenTrue                      = "true"
	TokenFalse                     = "false"
	PrefixIf                       = "if:"
	PrefixElif                     = "elif:"
	PrefixSet                      = "set:"
	DefaultAudioDuration           = "0:00"
	DefaultTrimStartStr            = "0.0000"
	DefaultTrimEndStr              = "1.0000"
	MaxZipTotalUncompressedBytes   = 500 * 1024 * 1024
	MaxZipSingleTextBytes          = 4 * 1024 * 1024
	SaveFileHeaderMagic            = "STSV\x01"
	DefaultAutoChoiceDelay         = 2.0
	BookModeBook                   = "book"
	BookModeScreenplay             = "screenplay"
	ScriptStandardPL               = "pl"
	ScriptStandardEU               = "eu"
	ScriptStandardUS               = "us"
	ScreenplayInsertDialogue       = "dialogue"
	ScreenplayInsertParenthetical  = "parenthetical"
	ScreenplayInsertAction         = "action"
	ScreenplayInsertDialogueOff    = "dialogue_off"
	ScreenplayInsertDialogueVO     = "dialogue_vo"
	ScreenplayInsertTransitionCut  = "transition_cut"
	ScreenplayInsertTransitionFade = "transition_fade"
	ScreenplayInsertSubslug        = "subslug"
	CourierLinesPerPageA4          = 54
	CourierLinesPerPageLetter      = 55
	CourierMaxColsAction           = 60
	CourierMaxColsSlugline         = 60
	CourierMaxColsDialogue         = 35
	CourierMaxColsParenthetical    = 25
	CourierMaxColsCharacter        = 38
	CourierMaxColsTransition       = 60

	ExportFmtPlaybook     = "playbook"
	ExportFmtWASM         = "wasm"
	ExportFmtPDF          = "pdf"
	ExportFmtFountain     = "fountain"
	ExportFmtFDX          = "fdx"
	ExportFmtTreatment    = "treatment"
	ExportFmtBookPDF      = "book_pdf"
	ExportFmtVOCSV        = "vo_csv"
	ExportFmtActorPDF     = "actor_pdf"
	ExportFmtPlaytxt      = "playtxt"
	ExportSuffixPlaybook  = ".playbook"
	ExportSuffixWASM      = ".wasm"
	ExportSuffixPDF       = ".pdf"
	ExportSuffixFountain  = ".fountain"
	ExportSuffixFDX       = ".fdx"
	ExportSuffixTreatment = "-treatment.pdf"
	ExportSuffixBookPDF   = "-book.pdf"
	ExportSuffixVOCSV     = "-vo.csv"
	ExportSuffixActorPDF  = "-actors.pdf"
	ExportSuffixPlaytxt   = ".playtxt"

	fallbackBookID       = "book"
	chapterTargetChapter = "chapter"
	chapterTargetRozdz1  = "rozdzial"
	chapterTargetRozdz2  = "rozdział"
	chapterTargetAct     = "act"
	chapterTargetAkt     = "akt"
	prefixNowaScenaSpace = "nowa scena "
	prefixNowaScenaSlug  = "nowa_scena_"
	prefixScenaSpace     = "scena "
	prefixScenaSlug      = "scena_"
	condPrefixIf         = "if:"
	condPrefixElif       = "elif:"
	condPrefixSet        = "set:"
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

var (
	plainHTMLTagStripRe = regexp.MustCompile(`<[a-zA-Z/!?][^>]*>`)
	polishLowerReplacer = strings.NewReplacer(
		"ą", "a", "ć", "c", "ę", "e", "ł", "l", "ń", "n", "ó", "o", "ś", "s", "ź", "z", "ż", "z",
	)
	polishCaseReplacer = strings.NewReplacer(
		"ą", "a", "ć", "c", "ę", "e", "ł", "l", "ń", "n", "ó", "o", "ś", "s", "ź", "z", "ż", "z",
		"Ą", "A", "Ć", "C", "Ę", "E", "Ł", "L", "Ń", "N", "Ó", "O", "Ś", "S", "Ź", "Z", "Ż", "Z",
		" ", "_",
	)
)

// SanitizePlainText strips HTML tags, control characters, and dangerous payloads from plain text.
func SanitizePlainText(rawText string) string {
	cleanText := strings.ReplaceAll(rawText, "\x00", "")
	cleanText = plainHTMLTagStripRe.ReplaceAllString(cleanText, "")
	var builder strings.Builder
	builder.Grow(len(cleanText))
	runes := []rune(cleanText)
	inDoubleBraces := 0
	for idx := 0; idx < len(runes); idx++ {
		charRune := runes[idx]
		if charRune == '{' && idx+1 < len(runes) && runes[idx+1] == '{' {
			inDoubleBraces++
			builder.WriteRune('{')
			builder.WriteRune('{')
			idx++
			continue
		}
		if charRune == '}' && idx+1 < len(runes) && runes[idx+1] == '}' && inDoubleBraces > 0 {
			inDoubleBraces--
			builder.WriteRune('}')
			builder.WriteRune('}')
			idx++
			continue
		}
		if charRune == '\n' {
			inDoubleBraces = 0
		}
		if charRune == '<' || charRune == '>' {
			if inDoubleBraces > 0 {
				builder.WriteRune(charRune)
				continue
			}
			cur := builder.String()
			lastNL := strings.LastIndexByte(cur, '\n')
			linePrefix := cur
			if lastNL >= 0 {
				linePrefix = cur[lastNL+1:]
			}
			if isConditionalOrRuleLinePrefix(linePrefix) {
				builder.WriteRune(charRune)
				continue
			}
			if charRune == '>' && builder.Len() > 0 && strings.HasSuffix(cur, "-") {
				builder.WriteRune(charRune)
			} else if charRune == '>' && strings.TrimSpace(linePrefix) == "" {
				builder.WriteRune(charRune)
			}
			continue
		}
		if charRune == '\n' || charRune == '\t' || charRune >= 0x20 {
			builder.WriteRune(charRune)
		}
	}
	return strings.TrimSpace(builder.String())
}

func isConditionalOrRuleLinePrefix(linePrefix string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(linePrefix))
	for _, prefix := range []string{condPrefixIf, condPrefixElif, condPrefixSet} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

// SanitizeChapterID normalizes a chapter title or ID into a clean lowercase ASCII/underscore identifier.
func SanitizeChapterID(rawID string) string {
	cleaned := strings.ToLower(strings.TrimSpace(SanitizePlainText(rawID)))
	if cleaned == "" {
		return ""
	}
	cleaned = polishLowerReplacer.Replace(cleaned)
	var builder strings.Builder
	lastUnderscore := false
	for _, charRune := range cleaned {
		if (charRune >= 'a' && charRune <= 'z') || (charRune >= '0' && charRune <= '9') || charRune == '-' {
			builder.WriteRune(charRune)
			lastUnderscore = false
		} else if unicode.IsLetter(charRune) || unicode.IsDigit(charRune) {
			builder.WriteRune(charRune)
			lastUnderscore = false
		} else if !lastUnderscore && builder.Len() > 0 {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(builder.String(), "_-")
}

// SanitizeSceneID normalizes a user-provided scene ID so it can be used safely in links and DOM attributes.
func SanitizeSceneID(rawID string) string {
	cleaned := strings.TrimSpace(SanitizePlainText(rawID))
	if cleaned == "" {
		return ""
	}
	cleaned = polishCaseReplacer.Replace(cleaned)
	var builder strings.Builder
	lastUnderscore := false
	for _, charRune := range cleaned {
		if (charRune >= 'a' && charRune <= 'z') || (charRune >= 'A' && charRune <= 'Z') || (charRune >= '0' && charRune <= '9') || charRune == '-' {
			builder.WriteRune(charRune)
			lastUnderscore = false
		} else if charRune == '_' {
			if !lastUnderscore && builder.Len() > 0 {
				builder.WriteByte('_')
				lastUnderscore = true
			}
		} else if unicode.IsLetter(charRune) || unicode.IsDigit(charRune) {
			builder.WriteRune(charRune)
			lastUnderscore = false
		}
	}
	res := strings.Trim(builder.String(), "_-")
	if len(res) > 80 {
		res = res[:80]
	}
	return res
}

// SanitizeRelativeMediaPath normalizes a relative path while rejecting path traversal ("..") or absolute paths.
func SanitizeRelativeMediaPath(rawPath string) string {
	cleaned := strings.TrimSpace(SanitizePlainText(rawPath))
	if cleaned == "" {
		return ""
	}
	slashed := filepath.ToSlash(strings.ReplaceAll(cleaned, "\\", "/"))
	slashed = strings.TrimPrefix(slashed, "./")
	slashed = strings.TrimLeft(slashed, "/")
	if slashed == "" || strings.Contains(slashed, "..") || strings.Contains(slashed, ":") {
		return ""
	}
	parts := strings.Split(slashed, "/")
	var safeParts []string
	for _, segment := range parts {
		segment = strings.TrimSpace(segment)
		if segment == "" || segment == "." || segment == ".." {
			return ""
		}
		safeParts = append(safeParts, segment)
	}
	return strings.Join(safeParts, "/")
}

// NormalizeLookupKey normalizes a scene or file name for case-insensitive matching.
func NormalizeLookupKey(rawName string) string {
	return strings.ToLower(strings.TrimSpace(rawName))
}

// SanitizeBookID produces a safe URL/filename slug.
func SanitizeBookID(rawID string) string {
	cleaned := strings.ToLower(SanitizePlainText(rawID))
	var builder strings.Builder
	for _, charRune := range cleaned {
		if (charRune >= 'a' && charRune <= 'z') || (charRune >= '0' && charRune <= '9') || charRune == '-' || charRune == '_' {
			builder.WriteRune(charRune)
		} else if unicode.IsLetter(charRune) || unicode.IsDigit(charRune) {
			builder.WriteRune(charRune)
		} else if builder.Len() > 0 {
			builder.WriteByte('-')
		}
	}
	res := strings.Trim(builder.String(), "-")
	if res == "" {
		return fallbackBookID
	}
	return res
}

// ParseAutoSceneSeq extracts N from an auto scene ID of the form "s<N>" (N >= 1), or returns 0.
func ParseAutoSceneSeq(rawID string) int {
	trimmed := strings.ToLower(strings.TrimSpace(rawID))
	if len(trimmed) < 2 || trimmed[0] != 's' {
		return 0
	}
	for idx := 1; idx < len(trimmed); idx++ {
		if trimmed[idx] < '0' || trimmed[idx] > '9' {
			return 0
		}
	}
	seqNum, parseErr := strconv.Atoi(trimmed[1:])
	if parseErr != nil || seqNum <= 0 {
		return 0
	}
	return seqNum
}

// ParseDefaultSceneNameSeq extracts N from default scene titles like "Nowa scena N" or "Scena N".
func ParseDefaultSceneNameSeq(rawName string) int {
	lower := strings.ToLower(strings.TrimSpace(rawName))
	for _, prefix := range []string{prefixNowaScenaSpace, prefixNowaScenaSlug, prefixScenaSpace, prefixScenaSlug} {
		if strings.HasPrefix(lower, prefix) {
			rest := strings.TrimSpace(lower[len(prefix):])
			if seqNum, parseErr := strconv.Atoi(rest); parseErr == nil && seqNum > 0 {
				return seqNum
			}
		}
	}
	return 0
}

// ParseChapterTarget checks whether a choice/next target string is a chapter transition.
func ParseChapterTarget(rawTarget string) (string, bool) {
	trimmed := strings.TrimSpace(rawTarget)
	colonIdx := strings.IndexByte(trimmed, ':')
	if colonIdx <= 0 {
		return "", false
	}
	prefix := strings.ToLower(strings.TrimSpace(trimmed[:colonIdx]))
	switch prefix {
	case chapterTargetChapter, chapterTargetRozdz1, chapterTargetRozdz2, chapterTargetAct, chapterTargetAkt:
		targetVal := strings.TrimSpace(trimmed[colonIdx+1:])
		return targetVal, targetVal != ""
	default:
		return "", false
	}
}

var ExportFormats = [...]string{
	ExportFmtPlaybook,
	ExportFmtWASM,
	ExportFmtPDF,
	ExportFmtFountain,
	ExportFmtFDX,
	ExportFmtTreatment,
	ExportFmtBookPDF,
	ExportFmtVOCSV,
	ExportFmtActorPDF,
	ExportFmtPlaytxt,
}

func IsValidExportFormat(formatID string) bool {
	for _, fmtName := range ExportFormats {
		if fmtName == formatID {
			return true
		}
	}
	return false
}
