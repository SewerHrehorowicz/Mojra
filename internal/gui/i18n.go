package gui

import (
	"fmt"

	"storyteller/internal/tpl"
)

// Lang aliases tpl.Lang so all packages share the same extensible language registry.
type Lang = tpl.Lang

const (
	LangPL    = tpl.LangPL
	LangEN    = tpl.LangEN
	LangCount = tpl.LangCount
)

// TrKey identifies a UI string entry in the zero-allocation [trKeyCount][LangCount]string table.
type TrKey uint16

const (
	TrPreviewPhone TrKey = iota
	TrPreviewTablet
	TrPreviewDesktop

	TrNavLibrary
	TrNavStudio
	TrMenu
	TrResetStoryHint
	TrToggleThemeHint
	TrToggleLangHint
	TrAppSettingsHint
	TrFullscreenHint

	TrScaleZenHint
	TrScaleExitZenHint
	TrScaleEnlargeHint
	TrImagelessLockedHint
	TrImagelessEnableHint
	TrImagelessDisableHint
	TrAutoChoiceOnLabel
	TrAutoChoiceOffLabel

	TrAddBookBtn
	TrExtractEmbeddedBook
	TrBookshelfEmpty

	TrHistoryPreviewBanner
	TrEditingLocked
	TrBackToEditor
	TrBackToCurrentVersion
	TrRewindSceneHint
	TrEndScreenplayReset
	TrEndStoryReset
	TrHistoryRailAria
	TrNext
	TrEndUpper

	TrNoActiveProject
	TrNewBookHint
	TrOpenProjectHint
	TrExportProjectHint
	TrZeroSelected
	TrFitNodesHint
	TrAutoLayoutHint
	TrVariablesTitle
	TrBreakdownHint
	TrHistoryPanelHint
	TrEditorHelpHint
	TrPlayBtn
	TrRestoreForEditing

	TrHelpPanelTitle
	TrHelpK1
	TrHelpD1
	TrHelpK2
	TrHelpD2
	TrHelpK3
	TrHelpD3
	TrHelpK4
	TrHelpD4
	TrHelpK5
	TrHelpD5

	TrSceneTitlePlaceholder
	TrChoiceTextPlaceholder
	TrCtxAddNode
	TrCtxAddAct
	TrCtxAddChapter
	TrCtxFitView
	TrNoOpenProject
	TrCreateNewBookBtn
	TrOpenProjectBtn

	TrChapterPlaceholder
	TrStartSceneBadgeHint
	TrChapterEntryHint
	TrSceneNameOptPlaceholder
	TrSceneIDPlaceholder
	TrSpSceneKey
	TrSluglinePlaceholder
	TrDescKey
	TrSynopsisPlaceholder
	TrOpenSceneTextHint
	TrPreviewContentBtn
	TrEditContentBtn
	TrPlayFromSceneHint
	TrPlayFromSceneBtn
	TrExitTargetPlaceholder
	TrAddChoiceBtn
	TrDeleteSceneBtn

	TrActSettingsTitle
	TrChapterSettingsTitle
	TrChapterNamePlaceholder
	TrEntryKey
	TrMetricScenes
	TrMetricLength
	TrMetricBranches
	TrMetricExits

	TrStandardKey
	TrActKey
	TrChapterKey
	TrNarrationKey
	TrImagesKey
	TrAuthorKey
	TrLicenseKey
	TrLicensePlaceholder
	TrFolderDirKey
	TrChangeProjectDirHint
	TrExportKey
	TrChangeExportDirHint
	TrSelectedScenesLabel
	TrDeleteSelectedScenes

	TrCloseVariablesPanel
	TrAddEnumOptHint
	TrEnumOptPlaceholder
	TrRemoveEnumOptHint

	TrBreakdownPanelTitle
	TrBdParamsLabel
	TrBdVolume
	TrBdScreenTime
	TrBdShootDays
	TrBdDaysUnit
	TrBdSceneCount
	TrBdIntExt
	TrBdDayNight
	TrBdActBalance
	TrBdCastLabel
	TrCharNamePlaceholder
	TrBdCues
	TrBdWords
	TrBdSpoken
	TrBdBg
	TrNoCharactersEmpty
	TrBdLocationsLabel
	TrLocNamePlaceholder
	TrBdDay
	TrBdNight
	TrNoLocationsEmpty

	TrHistoryPanelTitle
	TrSaveVersionBtn
	TrAddBranchBtn

	TrSettingsModalTitle
	TrSettingsLangKey
	TrThemeKey
	TrScaleKey
	TrScaleDefault
	TrScaleLarge
	TrLANQRDesc
	TrLANQRAlt
	TrClose
	TrCancel
	TrConfirmBtn

	TrNewScreenplayModalTitle
	TrNewBookModalTitle
	TrTitleKey
	TrAuthorPlaceholder
	TrModeKey
	TrRequiredLabel
	TrDisabledLabel
	TrWithImagesLabel
	TrNoImagesLabel
	TrFolderKey
	TrChooseFolderHint
	TrCreateScreenplayBtn
	TrCreateBookBtn

	TrExportModalTitle
	TrFormatsKey
	TrAdvancedUpper
	TrExportBtn

	TrAddBookModalTitle
	TrAddDiskHdr
	TrAddDiskDesc
	TrChooseBookDirBtn
	TrChooseBookFileBtn
	TrAddDiskPathPlaceholder
	TrAddFromDiskBtn
	TrAddURLHdr
	TrAddURLDesc
	TrAddURLPlaceholder
	TrDownloadFromURLBtn

	TrOpenProjectModalTitle
	TrChooseProjectDirBtn
	TrUnpackPlaybookCopyBtn
	TrStudioProjectPathPlaceholder
	TrOpenBtn

	TrStartSceneRootHint
	TrPreviewReadonlyBadge
	TrSceneSettingsHint
	TrTestStateTitle
	TrNoVarsInProject
	TrSpSceneUpperKey
	TrDescUpperKey
	TrInsertUpperKey
	TrCharUpperKey
	TrSceneTextPlaceholder
	TrGoToTargetSceneHint

	TrHelpMK1
	TrHelpMD1
	TrHelpMK2
	TrHelpMD2
	TrHelpMK3
	TrHelpMD3
	TrHelpMK4
	TrHelpMD4
	TrHelpMK5
	TrHelpMD5
	TrHelpMK6
	TrHelpMD6
	TrHelpMK7
	TrHelpMD7
	TrHelpMK8
	TrHelpMD8
	TrGotItBtn

	TrAutoChoiceDelayKey
	TrScrollSensitivityKey
	TrScrollSensHigh
	TrScrollSensMedium
	TrScrollSensLow
	TrScrollSensOff
	TrVoiceTrimDestructiveHint

	// Dynamic / server-side UI strings (formerly inline trPair / lang == LangEN)
	TrThemeDarkLabel
	TrThemeLightLabel
	TrImagelessLockedLabel
	TrImagelessOffLabel
	TrImagelessOnLabel
	TrEditorSpTitlePlaceholder
	TrEditorSpAuthorPlaceholder
	TrEditorSpDescPlaceholder
	TrEditorActTitlePlaceholder
	TrEditorSpSettingsHint
	TrEditorBookTitlePlaceholder
	TrEditorBookAuthorPlaceholder
	TrEditorBookDescPlaceholder
	TrEditorChapterTitlePlaceholder
	TrEditorBookSettingsHint
	TrBookNarratedReqLabel
	TrBookNarratedOffLabel
	TrBookImagelessOnLabel
	TrBookImagelessOffLabel
	TrMediaNoneLabel
	TrAddImageFromDiskOpt
	TrAddAudioFromDiskOpt
	TrWordsCountFmt
	TrModeInteractiveBook
	TrModeScreenplay
	TrHistoryMainScenesFmt
	TrHistoryInitialState
	TrHistoryPreviewPrefix
	TrHistoryDefaultVersionFmt
	TrHistoryDefaultBranchFmt
	TrEndOfPathMeta
	TrSyntaxHelpSetDesc
	TrSyntaxHelpExitDesc
	TrProjectKindBadge
	TrPlaybookReadonlyBadge
	TrNewScreenplayTitlePlaceholder
	TrNewBookTitlePlaceholder
	TrExportFmtWebApp
	TrExportFmtPDFScreenplay
	TrExportFmtFountain
	TrExportFmtPlaytxt
	TrExportSelectedOne
	TrExportSelectedFewFmt
	TrExportSelectedManyFmt
	TrSpPillDialogueLabel
	TrSpPillDialogueHint
	TrSpPillParenLabelPLStd
	TrSpPillParenLabelIntlStd
	TrSpPillParenHintPLStd
	TrSpPillParenHintIntlStd
	TrSpPillActionLabel
	TrSpPillActionHintPLStd
	TrSpPillActionHintIntlStd
	TrSpPillOffLabelPLStd
	TrSpPillOffLabelIntlStd
	TrSpPillOffHintPLStd
	TrSpPillOffHintIntlStd
	TrSpPillVOLabel
	TrSpPillVOHintPLStd
	TrSpPillVOHintIntlStd
	TrSpPillCutHintPLStd
	TrSpPillCutHintIntlStd
	TrSpPillFadeHintPLStd
	TrSpPillFadeHintIntlStd
	TrSpPillSubslugLabelIntlStd
	TrSpPillSubslugHintPLStd
	TrSpPillSubslugHintIntlStd
	TrSpPillCharDialogueHintFmt
	TrRewoundStoryToast
	TrResetProgressToast
	TrConfirmRewindMsgFmt
	TrConfirmResetTitle
	TrConfirmResetMsgFmt
	TrConfirmEndingTitle
	TrConfirmEndingMsg
	TrTargetSceneNotFoundPrefix
	TrOpenedInStudioLicenseFmt
	TrOpenedInStudioPrefix
	TrUnknownAuthor
	TrConfirmPlaybookReadonlyTitle
	TrConfirmPlaybookUnpackMsgFmt

	trKeyCount
)

// i18nDicts registers per-language translation tables (defined in i18n_<code.go>).
var i18nDicts = [LangCount]*[trKeyCount]string{
	LangPL: &i18nDictPL,
	LangEN: &i18nDictEN,
}

// i18nTable is a static zero-allocation lookup table indexed by [TrKey][Lang], compiled from i18nDicts with fallback.
var i18nTable [trKeyCount][LangCount]string

// tplI18nBindings maps template context keys (t_*) to TrKey indices in i18nTable.
var tplI18nBindings = [...]struct {
	varName string
	key     TrKey
}{
	{"t_preview_phone", TrPreviewPhone},
	{"t_preview_tablet", TrPreviewTablet},
	{"t_preview_desktop", TrPreviewDesktop},

	{"t_nav_library", TrNavLibrary},
	{"t_nav_studio", TrNavStudio},
	{"t_menu", TrMenu},
	{"t_reset_story_hint", TrResetStoryHint},
	{"t_toggle_theme_hint", TrToggleThemeHint},
	{"t_toggle_lang_hint", TrToggleLangHint},
	{"t_app_settings_hint", TrAppSettingsHint},
	{"t_fullscreen_hint", TrFullscreenHint},

	{"t_add_book_btn", TrAddBookBtn},
	{"t_extract_embedded_book", TrExtractEmbeddedBook},
	{"t_bookshelf_empty", TrBookshelfEmpty},

	{"t_history_preview_banner", TrHistoryPreviewBanner},
	{"t_editing_locked", TrEditingLocked},
	{"t_back_to_editor", TrBackToEditor},
	{"t_back_to_current_version", TrBackToCurrentVersion},
	{"t_rewind_scene_hint", TrRewindSceneHint},
	{"t_end_screenplay_reset", TrEndScreenplayReset},
	{"t_end_story_reset", TrEndStoryReset},
	{"t_history_rail_aria", TrHistoryRailAria},

	{"t_no_active_project", TrNoActiveProject},
	{"t_new_book_hint", TrNewBookHint},
	{"t_open_project_hint", TrOpenProjectHint},
	{"t_export_project_hint", TrExportProjectHint},
	{"t_zero_selected", TrZeroSelected},
	{"t_fit_nodes_hint", TrFitNodesHint},
	{"t_auto_layout_hint", TrAutoLayoutHint},
	{"t_variables_title", TrVariablesTitle},
	{"t_breakdown_hint", TrBreakdownHint},
	{"t_history_panel_hint", TrHistoryPanelHint},
	{"t_editor_help_hint", TrEditorHelpHint},
	{"t_play_btn", TrPlayBtn},
	{"t_restore_for_editing", TrRestoreForEditing},

	{"t_help_panel_title", TrHelpPanelTitle},
	{"t_help_k1", TrHelpK1},
	{"t_help_d1", TrHelpD1},
	{"t_help_k2", TrHelpK2},
	{"t_help_d2", TrHelpD2},
	{"t_help_k3", TrHelpK3},
	{"t_help_d3", TrHelpD3},
	{"t_help_k4", TrHelpK4},
	{"t_help_d4", TrHelpD4},
	{"t_help_k5", TrHelpK5},
	{"t_help_d5", TrHelpD5},

	{"t_scene_title_placeholder", TrSceneTitlePlaceholder},
	{"t_choice_text_placeholder", TrChoiceTextPlaceholder},
	{"t_ctx_add_node", TrCtxAddNode},
	{"t_ctx_add_act", TrCtxAddAct},
	{"t_ctx_add_chapter", TrCtxAddChapter},
	{"t_ctx_fit_view", TrCtxFitView},
	{"t_no_open_project", TrNoOpenProject},
	{"t_create_new_book_btn", TrCreateNewBookBtn},
	{"t_open_project_btn", TrOpenProjectBtn},

	{"t_chapter_placeholder", TrChapterPlaceholder},
	{"t_start_scene_badge_hint", TrStartSceneBadgeHint},
	{"t_chapter_entry_hint", TrChapterEntryHint},
	{"t_scene_name_opt_placeholder", TrSceneNameOptPlaceholder},
	{"t_scene_id_placeholder", TrSceneIDPlaceholder},
	{"t_sp_scene_key", TrSpSceneKey},
	{"t_slugline_placeholder", TrSluglinePlaceholder},
	{"t_desc_key", TrDescKey},
	{"t_synopsis_placeholder", TrSynopsisPlaceholder},
	{"t_open_scene_text_hint", TrOpenSceneTextHint},
	{"t_preview_content_btn", TrPreviewContentBtn},
	{"t_edit_content_btn", TrEditContentBtn},
	{"t_play_from_scene_hint", TrPlayFromSceneHint},
	{"t_play_from_scene_btn", TrPlayFromSceneBtn},
	{"t_exit_target_placeholder", TrExitTargetPlaceholder},
	{"t_add_choice_btn", TrAddChoiceBtn},
	{"t_delete_scene_btn", TrDeleteSceneBtn},

	{"t_act_settings_title", TrActSettingsTitle},
	{"t_chapter_settings_title", TrChapterSettingsTitle},
	{"t_chapter_name_placeholder", TrChapterNamePlaceholder},
	{"t_entry_key", TrEntryKey},
	{"t_metric_scenes", TrMetricScenes},
	{"t_metric_length", TrMetricLength},
	{"t_metric_branches", TrMetricBranches},
	{"t_metric_exits", TrMetricExits},

	{"t_standard_key", TrStandardKey},
	{"t_act_key", TrActKey},
	{"t_chapter_key", TrChapterKey},
	{"t_narration_key", TrNarrationKey},
	{"t_images_key", TrImagesKey},
	{"t_author_key", TrAuthorKey},
	{"t_license_key", TrLicenseKey},
	{"t_license_placeholder", TrLicensePlaceholder},
	{"t_folder_dir_key", TrFolderDirKey},
	{"t_change_project_dir_hint", TrChangeProjectDirHint},
	{"t_export_key", TrExportKey},
	{"t_change_export_dir_hint", TrChangeExportDirHint},
	{"t_selected_scenes_label", TrSelectedScenesLabel},
	{"t_delete_selected_scenes", TrDeleteSelectedScenes},

	{"t_close_variables_panel", TrCloseVariablesPanel},
	{"t_add_enum_opt_hint", TrAddEnumOptHint},
	{"t_enum_opt_placeholder", TrEnumOptPlaceholder},
	{"t_remove_enum_opt_hint", TrRemoveEnumOptHint},

	{"t_breakdown_panel_title", TrBreakdownPanelTitle},
	{"t_bd_params_label", TrBdParamsLabel},
	{"t_bd_volume", TrBdVolume},
	{"t_bd_screen_time", TrBdScreenTime},
	{"t_bd_shoot_days", TrBdShootDays},
	{"t_bd_days_unit", TrBdDaysUnit},
	{"t_bd_scene_count", TrBdSceneCount},
	{"t_bd_int_ext", TrBdIntExt},
	{"t_bd_day_night", TrBdDayNight},
	{"t_bd_act_balance", TrBdActBalance},
	{"t_bd_cast_label", TrBdCastLabel},
	{"t_char_name_placeholder", TrCharNamePlaceholder},
	{"t_bd_cues", TrBdCues},
	{"t_bd_words", TrBdWords},
	{"t_bd_spoken", TrBdSpoken},
	{"t_bd_bg", TrBdBg},
	{"t_no_characters_empty", TrNoCharactersEmpty},
	{"t_bd_locations_label", TrBdLocationsLabel},
	{"t_loc_name_placeholder", TrLocNamePlaceholder},
	{"t_bd_day", TrBdDay},
	{"t_bd_night", TrBdNight},
	{"t_no_locations_empty", TrNoLocationsEmpty},

	{"t_history_panel_title", TrHistoryPanelTitle},
	{"t_save_version_btn", TrSaveVersionBtn},
	{"t_add_branch_btn", TrAddBranchBtn},

	{"t_settings_modal_title", TrSettingsModalTitle},
	{"t_settings_lang_key", TrSettingsLangKey},
	{"t_theme_key", TrThemeKey},
	{"t_scale_key", TrScaleKey},
	{"t_scale_default", TrScaleDefault},
	{"t_scale_large", TrScaleLarge},
	{"t_lan_qr_desc", TrLANQRDesc},
	{"t_lan_qr_alt", TrLANQRAlt},
	{"t_close", TrClose},
	{"t_cancel", TrCancel},
	{"t_confirm_btn", TrConfirmBtn},

	{"t_new_screenplay_modal_title", TrNewScreenplayModalTitle},
	{"t_new_book_modal_title", TrNewBookModalTitle},
	{"t_title_key", TrTitleKey},
	{"t_author_placeholder", TrAuthorPlaceholder},
	{"t_mode_key", TrModeKey},
	{"t_required_label", TrRequiredLabel},
	{"t_disabled_label", TrDisabledLabel},
	{"t_with_images_label", TrWithImagesLabel},
	{"t_no_images_label", TrNoImagesLabel},
	{"t_folder_key", TrFolderKey},
	{"t_choose_folder_hint", TrChooseFolderHint},
	{"t_create_screenplay_btn", TrCreateScreenplayBtn},
	{"t_create_book_btn", TrCreateBookBtn},

	{"t_export_modal_title", TrExportModalTitle},
	{"t_formats_key", TrFormatsKey},
	{"t_advanced_upper", TrAdvancedUpper},
	{"t_export_btn", TrExportBtn},

	{"t_add_book_modal_title", TrAddBookModalTitle},
	{"t_add_disk_hdr", TrAddDiskHdr},
	{"t_add_disk_desc", TrAddDiskDesc},
	{"t_choose_book_dir_btn", TrChooseBookDirBtn},
	{"t_choose_book_file_btn", TrChooseBookFileBtn},
	{"t_add_disk_path_placeholder", TrAddDiskPathPlaceholder},
	{"t_add_from_disk_btn", TrAddFromDiskBtn},
	{"t_add_url_hdr", TrAddURLHdr},
	{"t_add_url_desc", TrAddURLDesc},
	{"t_add_url_placeholder", TrAddURLPlaceholder},
	{"t_download_from_url_btn", TrDownloadFromURLBtn},

	{"t_open_project_modal_title", TrOpenProjectModalTitle},
	{"t_choose_project_dir_btn", TrChooseProjectDirBtn},
	{"t_unpack_playbook_copy_btn", TrUnpackPlaybookCopyBtn},
	{"t_studio_project_path_placeholder", TrStudioProjectPathPlaceholder},
	{"t_open_btn", TrOpenBtn},

	{"t_start_scene_root_hint", TrStartSceneRootHint},
	{"t_preview_readonly_badge", TrPreviewReadonlyBadge},
	{"t_scene_settings_hint", TrSceneSettingsHint},
	{"t_test_state_title", TrTestStateTitle},
	{"t_no_vars_in_project", TrNoVarsInProject},
	{"t_sp_scene_upper_key", TrSpSceneUpperKey},
	{"t_desc_upper_key", TrDescUpperKey},
	{"t_insert_upper_key", TrInsertUpperKey},
	{"t_char_upper_key", TrCharUpperKey},
	{"t_scene_text_placeholder", TrSceneTextPlaceholder},
	{"t_go_to_target_scene_hint", TrGoToTargetSceneHint},

	{"t_help_m_k1", TrHelpMK1},
	{"t_help_m_d1", TrHelpMD1},
	{"t_help_m_k2", TrHelpMK2},
	{"t_help_m_d2", TrHelpMD2},
	{"t_help_m_k3", TrHelpMK3},
	{"t_help_m_d3", TrHelpMD3},
	{"t_help_m_k4", TrHelpMK4},
	{"t_help_m_d4", TrHelpMD4},
	{"t_help_m_k5", TrHelpMK5},
	{"t_help_m_d5", TrHelpMD5},
	{"t_help_m_k6", TrHelpMK6},
	{"t_help_m_d6", TrHelpMD6},
	{"t_help_m_k7", TrHelpMK7},
	{"t_help_m_d7", TrHelpMD7},
	{"t_help_m_k8", TrHelpMK8},
	{"t_help_m_d8", TrHelpMD8},
	{"t_got_it_btn", TrGotItBtn},

	{"t_auto_choice_delay_key", TrAutoChoiceDelayKey},
	{"t_scroll_sensitivity_key", TrScrollSensitivityKey},
	{"t_scroll_sens_high", TrScrollSensHigh},
	{"t_scroll_sens_medium", TrScrollSensMedium},
	{"t_scroll_sens_low", TrScrollSensLow},
	{"t_scroll_sens_off", TrScrollSensOff},
	{"t_voice_trim_destructive_hint", TrVoiceTrimDestructiveHint},
}

// i18nAnyTable holds pre-boxed interface{} values for i18nTable so PopulateI18nContext performs zero heap allocations.
var i18nAnyTable [LangCount][trKeyCount]any

// pre-boxed language metadata for zero-allocation PopulateI18nContext.
var (
	langCodeAnyTable    [LangCount]any
	langUpperAnyTable   [LangCount]any
	langIsENAnyTable    [LangCount]any
	langOptionsAnyTable [LangCount]any
)

func init() {
	for l := Lang(0); l < LangCount; l++ {
		dict := i18nDicts[l]
		for k := TrKey(0); k < trKeyCount; k++ {
			val := ""
			if dict != nil {
				val = dict[k]
			}
			if val == "" && i18nDicts[LangEN] != nil {
				val = i18nDicts[LangEN][k]
			}
			if val == "" && i18nDicts[LangPL] != nil {
				val = i18nDicts[LangPL][k]
			}
			i18nTable[k][l] = val
			i18nAnyTable[l][k] = val
		}

		spec := tpl.SupportedLanguages[l]
		langCodeAnyTable[l] = spec.Code
		langUpperAnyTable[l] = spec.Upper
		langIsENAnyTable[l] = l == LangEN

		opts := make([]map[string]any, LangCount)
		for optIdx := Lang(0); optIdx < LangCount; optIdx++ {
			optSpec := tpl.SupportedLanguages[optIdx]
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

	row := &i18nAnyTable[lang]
	for i := range tplI18nBindings {
		b := &tplI18nBindings[i]
		vars[b.varName] = row[b.key]
	}
}

func trLang(l Lang, k TrKey) string {
	if l >= LangCount {
		l = LangPL
	}
	return i18nTable[k][l]
}

func trfLang(l Lang, k TrKey, args ...any) string {
	return fmt.Sprintf(trLang(l, k), args...)
}

func (s *Server) lang() Lang {
	if s == nil || s.Settings == nil {
		return LangPL
	}
	return tpl.ParseLang(s.Settings.Language)
}

func (s *Server) tr(k TrKey) string {
	return i18nTable[k][s.lang()]
}

func (s *Server) trf(k TrKey, args ...any) string {
	return fmt.Sprintf(s.tr(k), args...)
}
