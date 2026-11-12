package i18n

import (
	"fmt"
	"strings"
)

// TrKey identifies a UI string entry in internal/i18n/<lang>.cfg.
type TrKey string

const (
	TrPreviewPhone   TrKey = "TrPreviewPhone"
	TrPreviewTablet  TrKey = "TrPreviewTablet"
	TrPreviewDesktop TrKey = "TrPreviewDesktop"

	TrNavLibrary      TrKey = "TrNavLibrary"
	TrNavStudio       TrKey = "TrNavStudio"
	TrMenu            TrKey = "TrMenu"
	TrResetStoryHint  TrKey = "TrResetStoryHint"
	TrToggleThemeHint TrKey = "TrToggleThemeHint"
	TrToggleLangHint  TrKey = "TrToggleLangHint"
	TrAppSettingsHint TrKey = "TrAppSettingsHint"
	TrFullscreenHint  TrKey = "TrFullscreenHint"

	TrScaleZenHint         TrKey = "TrScaleZenHint"
	TrScaleExitZenHint     TrKey = "TrScaleExitZenHint"
	TrScaleEnlargeHint     TrKey = "TrScaleEnlargeHint"
	TrScaleShrinkHint      TrKey = "TrScaleShrinkHint"
	TrImagelessLockedHint  TrKey = "TrImagelessLockedHint"
	TrImagelessEnableHint  TrKey = "TrImagelessEnableHint"
	TrImagelessDisableHint TrKey = "TrImagelessDisableHint"
	TrAutoChoiceOnLabel    TrKey = "TrAutoChoiceOnLabel"
	TrAutoChoiceOffLabel   TrKey = "TrAutoChoiceOffLabel"

	TrAddBookBtn          TrKey = "TrAddBookBtn"
	TrExtractEmbeddedBook TrKey = "TrExtractEmbeddedBook"
	TrBookshelfEmpty      TrKey = "TrBookshelfEmpty"

	TrHistoryPreviewBanner TrKey = "TrHistoryPreviewBanner"
	TrEditingLocked        TrKey = "TrEditingLocked"
	TrBackToEditor         TrKey = "TrBackToEditor"
	TrBackToCurrentVersion TrKey = "TrBackToCurrentVersion"
	TrRewindSceneHint      TrKey = "TrRewindSceneHint"
	TrEndScreenplayReset   TrKey = "TrEndScreenplayReset"
	TrEndStoryReset        TrKey = "TrEndStoryReset"
	TrHistoryRailAria      TrKey = "TrHistoryRailAria"
	TrNext                 TrKey = "TrNext"
	TrEndUpper             TrKey = "TrEndUpper"

	TrNoActiveProject   TrKey = "TrNoActiveProject"
	TrNewBookHint       TrKey = "TrNewBookHint"
	TrOpenProjectHint   TrKey = "TrOpenProjectHint"
	TrExportProjectHint TrKey = "TrExportProjectHint"
	TrZeroSelected      TrKey = "TrZeroSelected"
	TrFitNodesHint      TrKey = "TrFitNodesHint"
	TrAutoLayoutHint    TrKey = "TrAutoLayoutHint"
	TrVariablesTitle    TrKey = "TrVariablesTitle"
	TrBreakdownHint     TrKey = "TrBreakdownHint"
	TrHistoryPanelHint  TrKey = "TrHistoryPanelHint"
	TrEditorHelpHint    TrKey = "TrEditorHelpHint"
	TrPlayBtn           TrKey = "TrPlayBtn"
	TrRestoreForEditing TrKey = "TrRestoreForEditing"

	TrHelpPanelTitle TrKey = "TrHelpPanelTitle"
	TrHelpK1         TrKey = "TrHelpK1"
	TrHelpD1         TrKey = "TrHelpD1"
	TrHelpK2         TrKey = "TrHelpK2"
	TrHelpD2         TrKey = "TrHelpD2"
	TrHelpK3         TrKey = "TrHelpK3"
	TrHelpD3         TrKey = "TrHelpD3"
	TrHelpK4         TrKey = "TrHelpK4"
	TrHelpD4         TrKey = "TrHelpD4"
	TrHelpK5         TrKey = "TrHelpK5"
	TrHelpD5         TrKey = "TrHelpD5"

	TrSceneTitlePlaceholder TrKey = "TrSceneTitlePlaceholder"
	TrChoiceTextPlaceholder TrKey = "TrChoiceTextPlaceholder"
	TrCtxAddNode            TrKey = "TrCtxAddNode"
	TrCtxAddAct             TrKey = "TrCtxAddAct"
	TrCtxAddChapter         TrKey = "TrCtxAddChapter"
	TrCtxFitView            TrKey = "TrCtxFitView"
	TrNoOpenProject         TrKey = "TrNoOpenProject"
	TrCreateNewBookBtn      TrKey = "TrCreateNewBookBtn"
	TrOpenProjectBtn        TrKey = "TrOpenProjectBtn"

	TrChapterPlaceholder      TrKey = "TrChapterPlaceholder"
	TrStartSceneBadgeHint     TrKey = "TrStartSceneBadgeHint"
	TrChapterEntryHint        TrKey = "TrChapterEntryHint"
	TrSceneNameOptPlaceholder TrKey = "TrSceneNameOptPlaceholder"
	TrSceneIDPlaceholder      TrKey = "TrSceneIDPlaceholder"
	TrSpSceneKey              TrKey = "TrSpSceneKey"
	TrSluglinePlaceholder     TrKey = "TrSluglinePlaceholder"
	TrDescKey                 TrKey = "TrDescKey"
	TrSynopsisPlaceholder     TrKey = "TrSynopsisPlaceholder"
	TrOpenSceneTextHint       TrKey = "TrOpenSceneTextHint"
	TrPreviewContentBtn       TrKey = "TrPreviewContentBtn"
	TrEditContentBtn          TrKey = "TrEditContentBtn"
	TrPlayFromSceneHint       TrKey = "TrPlayFromSceneHint"
	TrPlayFromSceneBtn        TrKey = "TrPlayFromSceneBtn"
	TrExitTargetPlaceholder   TrKey = "TrExitTargetPlaceholder"
	TrAddChoiceBtn            TrKey = "TrAddChoiceBtn"
	TrDeleteSceneBtn          TrKey = "TrDeleteSceneBtn"

	TrActSettingsTitle       TrKey = "TrActSettingsTitle"
	TrChapterSettingsTitle   TrKey = "TrChapterSettingsTitle"
	TrChapterNamePlaceholder TrKey = "TrChapterNamePlaceholder"
	TrEntryKey               TrKey = "TrEntryKey"
	TrMetricScenes           TrKey = "TrMetricScenes"
	TrMetricLength           TrKey = "TrMetricLength"
	TrMetricBranches         TrKey = "TrMetricBranches"
	TrMetricExits            TrKey = "TrMetricExits"

	TrStandardKey          TrKey = "TrStandardKey"
	TrActKey               TrKey = "TrActKey"
	TrChapterKey           TrKey = "TrChapterKey"
	TrNarrationKey         TrKey = "TrNarrationKey"
	TrImagesKey            TrKey = "TrImagesKey"
	TrAuthorKey            TrKey = "TrAuthorKey"
	TrLicenseKey           TrKey = "TrLicenseKey"
	TrLicensePlaceholder   TrKey = "TrLicensePlaceholder"
	TrFolderDirKey         TrKey = "TrFolderDirKey"
	TrChangeProjectDirHint TrKey = "TrChangeProjectDirHint"
	TrExportKey            TrKey = "TrExportKey"
	TrChangeExportDirHint  TrKey = "TrChangeExportDirHint"
	TrSelectedScenesLabel  TrKey = "TrSelectedScenesLabel"
	TrDeleteSelectedScenes TrKey = "TrDeleteSelectedScenes"

	TrCloseVariablesPanel TrKey = "TrCloseVariablesPanel"
	TrAddEnumOptHint      TrKey = "TrAddEnumOptHint"
	TrEnumOptPlaceholder  TrKey = "TrEnumOptPlaceholder"
	TrRemoveEnumOptHint   TrKey = "TrRemoveEnumOptHint"

	TrBreakdownPanelTitle TrKey = "TrBreakdownPanelTitle"
	TrBdParamsLabel       TrKey = "TrBdParamsLabel"
	TrBdVolume            TrKey = "TrBdVolume"
	TrBdScreenTime        TrKey = "TrBdScreenTime"
	TrBdShootDays         TrKey = "TrBdShootDays"
	TrBdDaysUnit          TrKey = "TrBdDaysUnit"
	TrBdSceneCount        TrKey = "TrBdSceneCount"
	TrBdIntExt            TrKey = "TrBdIntExt"
	TrBdDayNight          TrKey = "TrBdDayNight"
	TrBdActBalance        TrKey = "TrBdActBalance"
	TrBdCastLabel         TrKey = "TrBdCastLabel"
	TrCharNamePlaceholder TrKey = "TrCharNamePlaceholder"
	TrBdCues              TrKey = "TrBdCues"
	TrBdWords             TrKey = "TrBdWords"
	TrBdSpoken            TrKey = "TrBdSpoken"
	TrBdBg                TrKey = "TrBdBg"
	TrNoCharactersEmpty   TrKey = "TrNoCharactersEmpty"
	TrBdLocationsLabel    TrKey = "TrBdLocationsLabel"
	TrLocNamePlaceholder  TrKey = "TrLocNamePlaceholder"
	TrBdDay               TrKey = "TrBdDay"
	TrBdNight             TrKey = "TrBdNight"
	TrNoLocationsEmpty    TrKey = "TrNoLocationsEmpty"

	TrHistoryPanelTitle TrKey = "TrHistoryPanelTitle"
	TrSaveVersionBtn    TrKey = "TrSaveVersionBtn"
	TrAddBranchBtn      TrKey = "TrAddBranchBtn"

	TrSettingsModalTitle TrKey = "TrSettingsModalTitle"
	TrSettingsLangKey    TrKey = "TrSettingsLangKey"
	TrThemeKey           TrKey = "TrThemeKey"
	TrScaleKey           TrKey = "TrScaleKey"
	TrScaleDefault       TrKey = "TrScaleDefault"
	TrScaleLarge         TrKey = "TrScaleLarge"
	TrLANQRDesc          TrKey = "TrLANQRDesc"
	TrLANQRAlt           TrKey = "TrLANQRAlt"
	TrClose              TrKey = "TrClose"
	TrCancel             TrKey = "TrCancel"
	TrConfirmBtn         TrKey = "TrConfirmBtn"

	TrNewScreenplayModalTitle TrKey = "TrNewScreenplayModalTitle"
	TrNewBookModalTitle       TrKey = "TrNewBookModalTitle"
	TrTitleKey                TrKey = "TrTitleKey"
	TrAuthorPlaceholder       TrKey = "TrAuthorPlaceholder"
	TrModeKey                 TrKey = "TrModeKey"
	TrRequiredLabel           TrKey = "TrRequiredLabel"
	TrDisabledLabel           TrKey = "TrDisabledLabel"
	TrWithImagesLabel         TrKey = "TrWithImagesLabel"
	TrNoImagesLabel           TrKey = "TrNoImagesLabel"
	TrFolderKey               TrKey = "TrFolderKey"
	TrChooseFolderHint        TrKey = "TrChooseFolderHint"
	TrCreateScreenplayBtn     TrKey = "TrCreateScreenplayBtn"
	TrCreateBookBtn           TrKey = "TrCreateBookBtn"

	TrExportModalTitle TrKey = "TrExportModalTitle"
	TrFormatsKey       TrKey = "TrFormatsKey"
	TrAdvancedUpper    TrKey = "TrAdvancedUpper"
	TrExportBtn        TrKey = "TrExportBtn"

	TrAddBookModalTitle      TrKey = "TrAddBookModalTitle"
	TrAddDiskHdr             TrKey = "TrAddDiskHdr"
	TrAddDiskDesc            TrKey = "TrAddDiskDesc"
	TrChooseBookDirBtn       TrKey = "TrChooseBookDirBtn"
	TrChooseBookFileBtn      TrKey = "TrChooseBookFileBtn"
	TrAddDiskPathPlaceholder TrKey = "TrAddDiskPathPlaceholder"
	TrAddFromDiskBtn         TrKey = "TrAddFromDiskBtn"
	TrAddURLHdr              TrKey = "TrAddURLHdr"
	TrAddURLDesc             TrKey = "TrAddURLDesc"
	TrAddURLPlaceholder      TrKey = "TrAddURLPlaceholder"
	TrDownloadFromURLBtn     TrKey = "TrDownloadFromURLBtn"

	TrOpenProjectModalTitle        TrKey = "TrOpenProjectModalTitle"
	TrChooseProjectDirBtn          TrKey = "TrChooseProjectDirBtn"
	TrUnpackPlaybookCopyBtn        TrKey = "TrUnpackPlaybookCopyBtn"
	TrStudioProjectPathPlaceholder TrKey = "TrStudioProjectPathPlaceholder"
	TrOpenBtn                      TrKey = "TrOpenBtn"

	TrStartSceneRootHint   TrKey = "TrStartSceneRootHint"
	TrPreviewReadonlyBadge TrKey = "TrPreviewReadonlyBadge"
	TrSceneSettingsHint    TrKey = "TrSceneSettingsHint"
	TrTestStateTitle       TrKey = "TrTestStateTitle"
	TrNoVarsInProject      TrKey = "TrNoVarsInProject"
	TrSpSceneUpperKey      TrKey = "TrSpSceneUpperKey"
	TrDescUpperKey         TrKey = "TrDescUpperKey"
	TrInsertUpperKey       TrKey = "TrInsertUpperKey"
	TrCharUpperKey         TrKey = "TrCharUpperKey"
	TrSceneTextPlaceholder TrKey = "TrSceneTextPlaceholder"
	TrGoToTargetSceneHint  TrKey = "TrGoToTargetSceneHint"

	TrHelpMK1  TrKey = "TrHelpMK1"
	TrHelpMD1  TrKey = "TrHelpMD1"
	TrHelpMK2  TrKey = "TrHelpMK2"
	TrHelpMD2  TrKey = "TrHelpMD2"
	TrHelpMK3  TrKey = "TrHelpMK3"
	TrHelpMD3  TrKey = "TrHelpMD3"
	TrHelpMK4  TrKey = "TrHelpMK4"
	TrHelpMD4  TrKey = "TrHelpMD4"
	TrHelpMK5  TrKey = "TrHelpMK5"
	TrHelpMD5  TrKey = "TrHelpMD5"
	TrHelpMK6  TrKey = "TrHelpMK6"
	TrHelpMD6  TrKey = "TrHelpMD6"
	TrHelpMK7  TrKey = "TrHelpMK7"
	TrHelpMD7  TrKey = "TrHelpMD7"
	TrHelpMK8  TrKey = "TrHelpMK8"
	TrHelpMD8  TrKey = "TrHelpMD8"
	TrGotItBtn TrKey = "TrGotItBtn"

	TrAutoChoiceDelayKey       TrKey = "TrAutoChoiceDelayKey"
	TrScrollSensitivityKey     TrKey = "TrScrollSensitivityKey"
	TrScrollSensHigh           TrKey = "TrScrollSensHigh"
	TrScrollSensMedium         TrKey = "TrScrollSensMedium"
	TrScrollSensLow            TrKey = "TrScrollSensLow"
	TrScrollSensOff            TrKey = "TrScrollSensOff"
	TrVoiceTrimDestructiveHint TrKey = "TrVoiceTrimDestructiveHint"

	TrThemeDarkLabel                 TrKey = "TrThemeDarkLabel"
	TrThemeLightLabel                TrKey = "TrThemeLightLabel"
	TrImagelessLockedLabel           TrKey = "TrImagelessLockedLabel"
	TrImagelessOffLabel              TrKey = "TrImagelessOffLabel"
	TrImagelessOnLabel               TrKey = "TrImagelessOnLabel"
	TrEditorSpTitlePlaceholder       TrKey = "TrEditorSpTitlePlaceholder"
	TrEditorSpAuthorPlaceholder      TrKey = "TrEditorSpAuthorPlaceholder"
	TrEditorSpDescPlaceholder        TrKey = "TrEditorSpDescPlaceholder"
	TrEditorActTitlePlaceholder      TrKey = "TrEditorActTitlePlaceholder"
	TrEditorSpSettingsHint           TrKey = "TrEditorSpSettingsHint"
	TrEditorBookTitlePlaceholder     TrKey = "TrEditorBookTitlePlaceholder"
	TrEditorBookAuthorPlaceholder    TrKey = "TrEditorBookAuthorPlaceholder"
	TrEditorBookDescPlaceholder      TrKey = "TrEditorBookDescPlaceholder"
	TrEditorChapterTitlePlaceholder  TrKey = "TrEditorChapterTitlePlaceholder"
	TrEditorBookSettingsHint         TrKey = "TrEditorBookSettingsHint"
	TrBookNarratedReqLabel           TrKey = "TrBookNarratedReqLabel"
	TrBookNarratedOffLabel           TrKey = "TrBookNarratedOffLabel"
	TrBookImagelessOnLabel           TrKey = "TrBookImagelessOnLabel"
	TrBookImagelessOffLabel          TrKey = "TrBookImagelessOffLabel"
	TrMediaNoneLabel                 TrKey = "TrMediaNoneLabel"
	TrAddImageFromDiskOpt            TrKey = "TrAddImageFromDiskOpt"
	TrAddAudioFromDiskOpt            TrKey = "TrAddAudioFromDiskOpt"
	TrWordsCountFmt                  TrKey = "TrWordsCountFmt"
	TrModeInteractiveBook            TrKey = "TrModeInteractiveBook"
	TrModeScreenplay                 TrKey = "TrModeScreenplay"
	TrHistoryMainScenesFmt           TrKey = "TrHistoryMainScenesFmt"
	TrHistoryInitialState            TrKey = "TrHistoryInitialState"
	TrHistoryPreviewPrefix           TrKey = "TrHistoryPreviewPrefix"
	TrHistoryDefaultVersionFmt       TrKey = "TrHistoryDefaultVersionFmt"
	TrHistoryDefaultBranchFmt        TrKey = "TrHistoryDefaultBranchFmt"
	TrEndOfPathMeta                  TrKey = "TrEndOfPathMeta"
	TrSyntaxHelpSetDesc              TrKey = "TrSyntaxHelpSetDesc"
	TrSyntaxHelpExitDesc             TrKey = "TrSyntaxHelpExitDesc"
	TrProjectKindBadge               TrKey = "TrProjectKindBadge"
	TrPlaybookReadonlyBadge          TrKey = "TrPlaybookReadonlyBadge"
	TrNewScreenplayTitlePlaceholder  TrKey = "TrNewScreenplayTitlePlaceholder"
	TrNewBookTitlePlaceholder        TrKey = "TrNewBookTitlePlaceholder"
	TrExportFmtWebApp                TrKey = "TrExportFmtWebApp"
	TrExportFmtPDFScreenplay         TrKey = "TrExportFmtPDFScreenplay"
	TrExportFmtFountain              TrKey = "TrExportFmtFountain"
	TrExportFmtPlaytxt               TrKey = "TrExportFmtPlaytxt"
	TrExportSelectedOne              TrKey = "TrExportSelectedOne"
	TrExportSelectedFewFmt           TrKey = "TrExportSelectedFewFmt"
	TrExportSelectedManyFmt          TrKey = "TrExportSelectedManyFmt"
	TrSpPillDialogueLabel            TrKey = "TrSpPillDialogueLabel"
	TrSpPillDialogueHint             TrKey = "TrSpPillDialogueHint"
	TrSpPillParenLabelPLStd          TrKey = "TrSpPillParenLabelPLStd"
	TrSpPillParenLabelIntlStd        TrKey = "TrSpPillParenLabelIntlStd"
	TrSpPillParenHintPLStd           TrKey = "TrSpPillParenHintPLStd"
	TrSpPillParenHintIntlStd         TrKey = "TrSpPillParenHintIntlStd"
	TrSpPillActionLabel              TrKey = "TrSpPillActionLabel"
	TrSpPillActionHintPLStd          TrKey = "TrSpPillActionHintPLStd"
	TrSpPillActionHintIntlStd        TrKey = "TrSpPillActionHintIntlStd"
	TrSpPillOffLabelPLStd            TrKey = "TrSpPillOffLabelPLStd"
	TrSpPillOffLabelIntlStd          TrKey = "TrSpPillOffLabelIntlStd"
	TrSpPillOffHintPLStd             TrKey = "TrSpPillOffHintPLStd"
	TrSpPillOffHintIntlStd           TrKey = "TrSpPillOffHintIntlStd"
	TrSpPillVOLabel                  TrKey = "TrSpPillVOLabel"
	TrSpPillVOHintPLStd              TrKey = "TrSpPillVOHintPLStd"
	TrSpPillVOHintIntlStd            TrKey = "TrSpPillVOHintIntlStd"
	TrSpPillCutHintPLStd             TrKey = "TrSpPillCutHintPLStd"
	TrSpPillCutHintIntlStd           TrKey = "TrSpPillCutHintIntlStd"
	TrSpPillFadeHintPLStd            TrKey = "TrSpPillFadeHintPLStd"
	TrSpPillFadeHintIntlStd          TrKey = "TrSpPillFadeHintIntlStd"
	TrSpPillSubslugLabelIntlStd      TrKey = "TrSpPillSubslugLabelIntlStd"
	TrSpPillSubslugHintPLStd         TrKey = "TrSpPillSubslugHintPLStd"
	TrSpPillSubslugHintIntlStd       TrKey = "TrSpPillSubslugHintIntlStd"
	TrSpPillCharDialogueHintFmt      TrKey = "TrSpPillCharDialogueHintFmt"
	TrRewoundStoryToast              TrKey = "TrRewoundStoryToast"
	TrResetProgressToast             TrKey = "TrResetProgressToast"
	TrConfirmRewindMsgFmt            TrKey = "TrConfirmRewindMsgFmt"
	TrConfirmResetTitle              TrKey = "TrConfirmResetTitle"
	TrConfirmResetMsgFmt             TrKey = "TrConfirmResetMsgFmt"
	TrConfirmEndingTitle             TrKey = "TrConfirmEndingTitle"
	TrConfirmEndingMsg               TrKey = "TrConfirmEndingMsg"
	TrTargetSceneNotFoundPrefix      TrKey = "TrTargetSceneNotFoundPrefix"
	TrOpenedInStudioLicenseFmt       TrKey = "TrOpenedInStudioLicenseFmt"
	TrOpenedInStudioPrefix           TrKey = "TrOpenedInStudioPrefix"
	TrUnknownAuthor                  TrKey = "TrUnknownAuthor"
	TrConfirmPlaybookReadonlyTitle   TrKey = "TrConfirmPlaybookReadonlyTitle"
	TrConfirmPlaybookUnpackMsgFmt    TrKey = "TrConfirmPlaybookUnpackMsgFmt"
	TrStudioSearchHint               TrKey = "TrStudioSearchHint"
	TrStudioSearchPlaceholder        TrKey = "TrStudioSearchPlaceholder"
	TrExportFmtFDX                   TrKey = "TrExportFmtFDX"
	TrExportFmtTreatmentPDF          TrKey = "TrExportFmtTreatmentPDF"
	TrExportFmtBookPDF               TrKey = "TrExportFmtBookPDF"
	TrExportFmtVOCSV                 TrKey = "TrExportFmtVOCSV"
	TrExportFmtActorPDF              TrKey = "TrExportFmtActorPDF"
	TrNewBookShortBtn                TrKey = "TrNewBookShortBtn"
	TrCreateNewScreenplayBtn         TrKey = "TrCreateNewScreenplayBtn"
	TrOpenProjectFromDiskBtn         TrKey = "TrOpenProjectFromDiskBtn"
	TrHomeSwitchToStudioPrefix       TrKey = "TrHomeSwitchToStudioPrefix"
	TrHomeSwitchToReaderPrefix       TrKey = "TrHomeSwitchToReaderPrefix"
	TrHistoryPreviewLocked           TrKey = "TrHistoryPreviewLocked"
	TrSceneTextSavedFmt              TrKey = "TrSceneTextSavedFmt"
	TrHistSceneTextEditedFmt         TrKey = "TrHistSceneTextEditedFmt"
)

type tplAnyEntry struct {
	varName string
	val     any
}

var (
	tplEntriesByLang    [LangCount][]tplAnyEntry
	langCodeAnyTable    [LangCount]any
	langUpperAnyTable   [LangCount]any
	langIsENAnyTable    [LangCount]any
	langOptionsAnyTable [LangCount]any
)

// TrKeyToTplVar converts a "TrFooBar" key to its template variable name "t_foo_bar".
func TrKeyToTplVar(key string) string {
	s := strings.TrimPrefix(key, "Tr")
	s = strings.ReplaceAll(s, "LANQR", "LanQr")
	s = strings.ReplaceAll(s, "HelpMK", "HelpM_K")
	s = strings.ReplaceAll(s, "HelpMD", "HelpM_D")
	var b strings.Builder
	b.Grow(len(s) + 8)
	b.WriteString("t_")
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			b.WriteByte('_')
			continue
		}
		if c >= 'A' && c <= 'Z' {
			if i > 0 && s[i-1] != '_' {
				prevLowerOrDigit := (s[i-1] >= 'a' && s[i-1] <= 'z') || (s[i-1] >= '0' && s[i-1] <= '9')
				nextLower := i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z'
				if prevLowerOrDigit || nextLower {
					b.WriteByte('_')
				}
			}
			b.WriteByte(c + ('a' - 'A'))
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func initUIContextTables() {
	var trKeys []string
	for k := range I18nDicts[LangPL] {
		if strings.HasPrefix(k, "Tr") {
			trKeys = append(trKeys, k)
		}
	}
	tplVars := make([]string, len(trKeys))
	for i, k := range trKeys {
		tplVars[i] = TrKeyToTplVar(k)
	}

	for l := Lang(0); l < LangCount; l++ {
		entries := make([]tplAnyEntry, len(trKeys))
		for i, k := range trKeys {
			entries[i] = tplAnyEntry{
				varName: tplVars[i],
				val:     LookupJSI18n(l, k),
			}
		}
		tplEntriesByLang[l] = entries

		spec := SupportedLanguages[l]
		langCodeAnyTable[l] = spec.Code
		langUpperAnyTable[l] = spec.Upper
		langIsENAnyTable[l] = l == LangEN

		opts := make([]map[string]any, LangCount)
		for optIdx := Lang(0); optIdx < LangCount; optIdx++ {
			optSpec := SupportedLanguages[optIdx]
			opts[optIdx] = map[string]any{
				"code":      optSpec.Code,
				"upper":     optSpec.Upper,
				"name":      optSpec.Name,
				"is_active": optIdx == l,
			}
		}
		langOptionsAnyTable[l] = opts
	}
}

// PopulateI18nContext writes all t_* UI strings and language metadata for the given Lang into vars with zero allocations.
func PopulateI18nContext(vars map[string]any, lang Lang) {
	if lang >= LangCount {
		lang = LangPL
	}
	vars["lang"] = langCodeAnyTable[lang]
	vars["lang_upper"] = langUpperAnyTable[lang]
	vars["is_lang_en"] = langIsENAnyTable[lang]
	vars["lang_options"] = langOptionsAnyTable[lang]

	entries := tplEntriesByLang[lang]
	for i := range entries {
		vars[entries[i].varName] = entries[i].val
	}
}

func T(l Lang, k TrKey) string {
	return LookupJSI18n(l, string(k))
}

func Tf(l Lang, k TrKey, args ...any) string {
	return fmt.Sprintf(T(l, k), args...)
}
