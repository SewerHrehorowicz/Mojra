package actions

import (
	"bytes"
	"html"
	"regexp"
	"strings"

	"mojra/domain/book/model"
	"mojra/engine/templates"
)

type (
	Book                 = model.Book
	Chapter              = model.Chapter
	Scene                = model.Scene
	Choice               = model.Choice
	ChoiceEval           = model.ChoiceEval
	SceneChoicesEval     = model.SceneChoicesEval
	VoiceSectionMeta     = model.VoiceSectionMeta
	VoiceSection         = model.VoiceSection
	VoiceQueueItem       = model.VoiceQueueItem
	SelectOption         = model.SelectOption
	NodePosition         = model.NodePosition
	WirePin              = model.WirePin
	PackedMediaEntry     = model.PackedMediaEntry
	ExportFormatItem     = model.ExportFormatItem
	StudioSearchItem     = model.StudioSearchItem
	EditorNodeView       = model.EditorNodeView
	EnumDef              = model.EnumDef
	VariableDef          = model.VariableDef
	HistoryEntry         = model.HistoryEntry
	SaveState            = model.SaveState
	EditorBookUIState    = model.EditorBookUIState
	AppSettings          = model.AppSettings
	ScreenplayBlockKind  = model.ScreenplayBlockKind
	ScreenplayBlock      = model.ScreenplayBlock
	SluglineInfo         = model.SluglineInfo
	SceneScriptMetrics   = model.SceneScriptMetrics
	ScreenplayInsertPill = model.ScreenplayInsertPill
	LocationStat         = model.LocationStat
	CharacterStat        = model.CharacterStat
	ActStat              = model.ActStat
	ScreenplayBreakdown  = model.ScreenplayBreakdown
	HistoryBranch        = model.HistoryBranch
	BranchComparison     = model.BranchComparison
	HistoryRevision      = model.HistoryRevision
	HistoryGraphLane     = model.HistoryGraphLane
	HistoryGraphRow      = model.HistoryGraphRow
	HistoryPreview       = model.HistoryPreview
	BookHistory          = model.BookHistory
)

const (
	PlaytxtExt                     = model.PlaytxtExt
	PlaybookExt                    = model.PlaybookExt
	ZipExt                         = model.ZipExt
	BookDataFileName               = model.BookDataFileName
	EditorStateFileName            = model.EditorStateFileName
	DefaultStoryFile               = model.DefaultStoryFile
	DefaultChapterFile             = model.DefaultChapterFile
	DefaultActFile                 = model.DefaultActFile
	DefaultBookChapterName         = model.DefaultBookChapterName
	DefaultScreenplayActName       = model.DefaultScreenplayActName
	TargetEnd                      = model.TargetEnd
	TokenTrue                      = model.TokenTrue
	TokenFalse                     = model.TokenFalse
	PrefixIf                       = model.PrefixIf
	PrefixElif                     = model.PrefixElif
	PrefixSet                      = model.PrefixSet
	DefaultAudioDuration           = model.DefaultAudioDuration
	DefaultTrimStartStr            = model.DefaultTrimStartStr
	DefaultTrimEndStr              = model.DefaultTrimEndStr
	MaxZipTotalUncompressedBytes   = model.MaxZipTotalUncompressedBytes
	MaxZipSingleTextBytes          = model.MaxZipSingleTextBytes
	SaveFileHeaderMagic            = model.SaveFileHeaderMagic
	DefaultAutoChoiceDelay         = model.DefaultAutoChoiceDelay
	BookModeBook                   = model.BookModeBook
	BookModeScreenplay             = model.BookModeScreenplay
	ScriptStandardPL               = model.ScriptStandardPL
	ScriptStandardEU               = model.ScriptStandardEU
	ScriptStandardUS               = model.ScriptStandardUS
	ScreenplayInsertDialogue       = model.ScreenplayInsertDialogue
	ScreenplayInsertParenthetical  = model.ScreenplayInsertParenthetical
	ScreenplayInsertAction         = model.ScreenplayInsertAction
	ScreenplayInsertDialogueOff    = model.ScreenplayInsertDialogueOff
	ScreenplayInsertDialogueVO     = model.ScreenplayInsertDialogueVO
	ScreenplayInsertTransitionCut  = model.ScreenplayInsertTransitionCut
	ScreenplayInsertTransitionFade = model.ScreenplayInsertTransitionFade
	ScreenplayInsertSubslug        = model.ScreenplayInsertSubslug
	CourierLinesPerPageA4          = model.CourierLinesPerPageA4
	CourierLinesPerPageLetter      = model.CourierLinesPerPageLetter
	CourierMaxColsAction           = model.CourierMaxColsAction
	CourierMaxColsSlugline         = model.CourierMaxColsSlugline
	CourierMaxColsDialogue         = model.CourierMaxColsDialogue
	CourierMaxColsParenthetical    = model.CourierMaxColsParenthetical
	CourierMaxColsCharacter        = model.CourierMaxColsCharacter
	CourierMaxColsTransition       = model.CourierMaxColsTransition
	BlockSlugline                  = model.BlockSlugline
	BlockAction                    = model.BlockAction
	BlockCharacter                 = model.BlockCharacter
	BlockParenthetical             = model.BlockParenthetical
	BlockDialogue                  = model.BlockDialogue
	BlockTransition                = model.BlockTransition
	VarKindBool                    = model.VarKindBool
	VarKindInt                     = model.VarKindInt
	VarKindEnum                    = model.VarKindEnum
	EditorGridSize                 = model.EditorGridSize
	NodeCardWidth                  = model.NodeCardWidth
	NodeCardHalfWidth              = model.NodeCardHalfWidth
	NodeCardBaseHeight             = model.NodeCardBaseHeight
	NodeChoiceRowHeight            = model.NodeChoiceRowHeight
	NodeMinLogicalHeight           = model.NodeMinLogicalHeight
	NodeColSpacing                 = model.NodeColSpacing
	ChapterFramePadX               = model.ChapterFramePadX
	ChapterFramePadX2              = model.ChapterFramePadX2
	ChapterFramePadTop             = model.ChapterFramePadTop
	ChapterFramePadBottom          = model.ChapterFramePadBottom
	ChapterFramePadYTotal          = model.ChapterFramePadYTotal
	NewChapterDefaultOffsetX       = model.NewChapterDefaultOffsetX
	NewChapterDefaultOffsetY       = model.NewChapterDefaultOffsetY
	ExportFmtPlaybook              = model.ExportFmtPlaybook
	ExportFmtWASM                  = model.ExportFmtWASM
	ExportFmtPDF                   = model.ExportFmtPDF
	ExportFmtFountain              = model.ExportFmtFountain
	ExportFmtFDX                   = model.ExportFmtFDX
	ExportFmtTreatment             = model.ExportFmtTreatment
	ExportFmtBookPDF               = model.ExportFmtBookPDF
	ExportFmtVOCSV                 = model.ExportFmtVOCSV
	ExportFmtActorPDF              = model.ExportFmtActorPDF
	ExportFmtPlaytxt               = model.ExportFmtPlaytxt
	ExportSuffixPlaybook           = model.ExportSuffixPlaybook
	ExportSuffixWASM               = model.ExportSuffixWASM
	ExportSuffixPDF                = model.ExportSuffixPDF
	ExportSuffixFountain           = model.ExportSuffixFountain
	ExportSuffixFDX                = model.ExportSuffixFDX
	ExportSuffixTreatment          = model.ExportSuffixTreatment
	ExportSuffixBookPDF            = model.ExportSuffixBookPDF
	ExportSuffixVOCSV              = model.ExportSuffixVOCSV
	ExportSuffixActorPDF           = model.ExportSuffixActorPDF
	ExportSuffixPlaytxt            = model.ExportSuffixPlaytxt

	proseLineBreakTag   = "<br />"
	proseParaOpenTag    = "<p>"
	proseParaCloseTag   = "</p>"
	proseEmptyParaTag   = "<p></p>"
	proseDividerTag     = `<hr class="scene-divider" />`
	proseBlockquoteOpen = "<blockquote>"
	proseBlockquoteEnd  = "</blockquote>"
	proseStrongReplace  = "<strong>$1</strong>"
	proseEmReplace      = "<em>$1</em>"
	proseDividerDashes  = "---"
	proseDividerStars   = "***"
	sigDoctype          = "<!doctype"
	sigHTML             = "<html"
	sigScript           = "<script"
	sigSVG              = "<svg"
	sigXML              = "<?xml"
	sigShebang          = "#!/"
	sigELF              = "\x7felf"
)

var (
	SanitizePlainText         = model.SanitizePlainText
	SanitizeBookID            = model.SanitizeBookID
	SanitizeSceneID           = model.SanitizeSceneID
	SanitizeChapterID         = model.SanitizeChapterID
	NormalizeLookupKey        = model.NormalizeLookupKey
	SanitizeRelativeMediaPath = model.SanitizeRelativeMediaPath
	ParseAutoSceneSeq         = model.ParseAutoSceneSeq
	ParseDefaultSceneNameSeq  = model.ParseDefaultSceneNameSeq
	ParseChapterTarget        = model.ParseChapterTarget
	SanitizeVarName           = model.SanitizeVarName
	SanitizeEnumOption        = model.SanitizeEnumOption
	SanitizeRuleExpr          = model.SanitizeRuleExpr
	StripOuterDoubleBraces    = model.StripOuterDoubleBraces
	CleanConditionExpr        = model.CleanConditionExpr
	NormalizeAutoChoiceDelay  = model.NormalizeAutoChoiceDelay
	NormalizedVoiceTrimValues = model.NormalizedVoiceTrimValues
	HasVoiceTrimValues        = model.HasVoiceTrimValues
	NormalizeBookMode         = model.NormalizeBookMode
	NormalizeScriptStandard   = model.NormalizeScriptStandard
	IsNumberedStandard        = model.IsNumberedStandard
	DefaultChapterNameForKind = model.DefaultChapterNameForKind
	SnapToGrid                = model.SnapToGrid
	StoreSceneNodePosition    = model.StoreSceneNodePosition
	ParseWirePinList          = model.ParseWirePinList
	IsValidExportFormat       = model.IsValidExportFormat
	ExportFormats             = model.ExportFormats

	proseInlineBoldRe   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	proseInlineItalicRe = regexp.MustCompile(`\*([^*]+)\*`)
	htmlTagStripRe      = regexp.MustCompile(`<[a-zA-Z/!?][^>]*>`)
	safeIDRe            = regexp.MustCompile(`^[\p{L}\p{N}_\-]+$`)
	headerKeyRe         = regexp.MustCompile(`^[a-zA-ZąćęłńóśźżĄĆĘŁŃÓŚŹŻ_]+$`)
)

func init() {
	templates.ProseFormatter = RenderSafeProseHTML
	model.FormatProseHTMLHook = RenderSafeProseHTML
	model.EvaluateConditionHook = EvaluateCondition
}

// RenderSafeProseHTML converts plain text into safe HTML paragraphs after escaping all HTML special characters.
func RenderSafeProseHTML(rawText string) string {
	lines := strings.Split(strings.ReplaceAll(rawText, "\r\n", "\n"), "\n")
	var blocks []string
	var para []string

	flushPara := func() {
		if len(para) == 0 {
			return
		}
		joined := strings.Join(para, proseLineBreakTag)
		blocks = append(blocks, proseParaOpenTag+joined+proseParaCloseTag)
		para = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flushPara()
			continue
		}
		if trimmed == proseDividerDashes || trimmed == proseDividerStars {
			flushPara()
			blocks = append(blocks, proseDividerTag)
			continue
		}
		if strings.HasPrefix(trimmed, ">") {
			flushPara()
			quoteText := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			blocks = append(blocks, proseBlockquoteOpen+formatProseInlineSafe(quoteText)+proseBlockquoteEnd)
			continue
		}
		para = append(para, formatProseInlineSafe(trimmed))
	}
	flushPara()

	if len(blocks) == 0 {
		return proseEmptyParaTag
	}
	return strings.Join(blocks, "\n")
}

func formatProseInlineSafe(rawSegment string) string {
	clean := SanitizePlainText(rawSegment)
	var escapedBuf bytes.Buffer
	escapedBuf.WriteString(html.EscapeString(clean))
	escaped := escapedBuf.String()
	escaped = proseInlineBoldRe.ReplaceAllString(escaped, proseStrongReplace)
	escaped = proseInlineItalicRe.ReplaceAllString(escaped, proseEmReplace)
	return escaped
}

func hasDisallowedMediaSignature(lowerProbe string) bool {
	for _, sig := range []string{sigDoctype, sigHTML, sigScript, sigSVG, sigXML, sigShebang, sigELF} {
		if strings.Contains(lowerProbe, sig) {
			return true
		}
	}
	return false
}
