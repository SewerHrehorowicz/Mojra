package gui

import (
	"mojra/internal/book"
	"mojra/internal/tpl"
)

// BuildTargetedRenderContext constructs only the data needed for the given template/chunk target.
func (s *Server) BuildTargetedRenderContext(targetTpl string, books []*book.Book) *tpl.RenderContext {
	vars := make(map[string]any, 280)
	lang := s.lang()
	PopulateI18nContext(vars, lang)

	theme := s.Settings.Theme
	if theme != "light" {
		theme = "dark"
	}
	vars["theme"] = theme
	if theme == "light" {
		vars["theme_toggle_label"] = s.tr(TrThemeDarkLabel)
	} else {
		vars["theme_toggle_label"] = s.tr(TrThemeLightLabel)
	}

	imagelessLocked := s.isBookImageless()
	effectiveImageless := s.isEffectiveImageless()
	vars["imageless_mode"] = effectiveImageless
	vars["imageless_locked"] = imagelessLocked
	if imagelessLocked {
		vars["imageless_locked_attr"] = "true"
	} else {
		vars["imageless_locked_attr"] = "false"
	}
	if effectiveImageless {
		vars["imageless_attr"] = "true"
		if imagelessLocked {
			vars["imageless_toggle_label"] = s.tr(TrImagelessLockedLabel)
			vars["imageless_toggle_title"] = s.tr(TrImagelessLockedHint)
		} else {
			vars["imageless_toggle_label"] = s.tr(TrImagelessOffLabel)
			vars["imageless_toggle_title"] = s.tr(TrImagelessEnableHint)
		}
	} else {
		vars["imageless_attr"] = "false"
		vars["imageless_toggle_label"] = s.tr(TrImagelessOnLabel)
		vars["imageless_toggle_title"] = s.tr(TrImagelessDisableHint)
	}

	uiScale := s.Settings.UIScale
	if uiScale != "large" && uiScale != "zen" {
		uiScale = "small"
	}
	vars["ui_scale"] = uiScale
	vars["is_small_ui"] = uiScale == "small"
	vars["is_large_ui"] = uiScale == "large"
	vars["is_zen_ui"] = uiScale == "zen"
	vars["is_non_small_ui"] = uiScale != "small"
	switch uiScale {
	case "large":
		vars["ui_scale_toggle_title"] = s.tr(TrScaleZenHint)
	case "zen":
		vars["ui_scale_toggle_title"] = s.tr(TrScaleExitZenHint)
	default:
		vars["ui_scale_toggle_title"] = s.tr(TrScaleEnlargeHint)
	}

	autoDelay := book.NormalizeAutoChoiceDelay(s.Settings.AutoChoiceDelay)
	vars["auto_choice_delay"] = autoDelay
	vars["is_auto_delay_1"] = autoDelay <= 1.25
	vars["is_auto_delay_2"] = autoDelay > 1.25 && autoDelay <= 2.5
	vars["is_auto_delay_3"] = autoDelay > 2.5 && autoDelay <= 4.0
	vars["is_auto_delay_5"] = autoDelay > 4.0

	scrollSens := book.NormalizeScrollSensitivity(s.Settings.ScrollSensitivity)
	vars["scroll_sensitivity"] = scrollSens
	vars["is_scroll_sens_high"] = scrollSens == "high"
	vars["is_scroll_sens_medium"] = scrollSens == "medium"
	vars["is_scroll_sens_low"] = scrollSens == "low"
	vars["is_scroll_sens_off"] = scrollSens == "off"

	vars["standalone_mode"] = s.StandaloneMode
	vars["has_active_book"] = s.PlayerBook != nil
	vars["has_editor_book"] = s.ActiveBook != nil
	vars["dev_mode"] = s.DevMode
	if s.DevMode {
		previewMode := s.Settings.PreviewMode
		if previewMode != "portrait" && previewMode != "tablet" && previewMode != "landscape" {
			previewMode = "full"
		}
		vars["preview_mode"] = previewMode
	} else {
		vars["preview_mode"] = "full"
	}

	view := s.Settings.ActiveView
	if s.StandaloneMode {
		view = "player"
	} else if view == "player" && s.PlayerBook == nil {
		view = "library"
	}
	vars["is_view_library"] = view == "library"
	vars["is_view_player"] = view == "player"
	vars["is_view_editor"] = view == "editor"

	vars["default_new_book_dir"] = s.defaultNewBookDir()
	vars["default_export_dir"] = s.defaultExportDir()
	if s.PlayerBook != nil {
		vars["active_book_title"] = s.PlayerBook.Title
	} else {
		vars["active_book_title"] = s.tr(TrNavLibrary)
	}
	isScreenplay := false
	if s.ActiveBook != nil {
		vars["editor_book_title"] = s.ActiveBook.Title
		vars["active_book_save_dir"] = s.ActiveBook.RootPath
		vars["active_book_export_dir"] = s.ActiveBook.ExportDir
		vars["active_book_license"] = s.ActiveBook.License
		isScreenplay = s.ActiveBook.IsScreenplay()
		vars["script_standard"] = s.ActiveBook.NormalizedStandard()
	} else {
		vars["editor_book_title"] = ""
		vars["active_book_save_dir"] = ""
		vars["active_book_export_dir"] = ""
		vars["active_book_license"] = ""
		vars["script_standard"] = "pl"
	}
	if view == "player" && s.PlayerBook != nil {
		vars["is_screenplay_mode"] = s.PlayerBook.IsScreenplay()
	} else {
		vars["is_screenplay_mode"] = isScreenplay
	}
	if isScreenplay {
		vars["editor_title_placeholder"] = s.tr(TrEditorSpTitlePlaceholder)
		vars["editor_author_placeholder"] = s.tr(TrEditorSpAuthorPlaceholder)
		vars["editor_description_placeholder"] = s.tr(TrEditorSpDescPlaceholder)
		vars["editor_chapter_title_placeholder"] = s.tr(TrEditorActTitlePlaceholder)
		vars["editor_settings_hint"] = s.tr(TrEditorSpSettingsHint)
	} else {
		vars["editor_title_placeholder"] = s.tr(TrEditorBookTitlePlaceholder)
		vars["editor_author_placeholder"] = s.tr(TrEditorBookAuthorPlaceholder)
		vars["editor_description_placeholder"] = s.tr(TrEditorBookDescPlaceholder)
		vars["editor_chapter_title_placeholder"] = s.tr(TrEditorChapterTitlePlaceholder)
		vars["editor_settings_hint"] = s.tr(TrEditorBookSettingsHint)
	}

	vars["show_reset_progress"] = view == "player" && s.PlayerBook != nil
	isHistPreview := s.isHistoryPreview()
	vars["is_history_preview"] = isHistPreview
	if isHistPreview {
		vars["history_preview_attr"] = "true"
	} else {
		vars["history_preview_attr"] = "false"
	}
	vars["history_preview_id"] = s.PreviewRevisionID
	vars["history_preview_label"] = s.PreviewRevisionLabel

	needsLibrary := targetTpl == "library_view" || ((targetTpl == "index" || targetTpl == "app_shell") && view == "library")
	needsPlayer := targetTpl == "player_view" || ((targetTpl == "index" || targetTpl == "app_shell") && view == "player")
	needsEditorFull := targetTpl == "editor_view" || ((targetTpl == "index" || targetTpl == "app_shell") && view == "editor" && !s.StandaloneMode)
	needsEditorPanelOnly := targetTpl == "editor_context_panel" && !s.StandaloneMode
	needsModals := targetTpl == "modals" || targetTpl == "index" || targetTpl == "app_shell"

	if needsLibrary {
		s.populateLibraryContext(vars, books)
	}

	if needsPlayer && s.PlayerBook != nil && len(s.PlayerBook.Scenes) > 0 {
		s.populatePlayerContext(vars)
	}

	if s.ActiveBook != nil && len(s.ActiveBook.Scenes) > 0 && (needsEditorFull || needsEditorPanelOnly) {
		vars["is_screenplay_mode"] = s.ActiveBook.IsScreenplay()
		warnItems := make([]map[string]any, 0, len(s.ActiveBook.Warnings))
		for _, w := range s.ActiveBook.Warnings {
			warnItems = append(warnItems, map[string]any{"text": w})
		}
		vars["has_book_warnings"] = len(s.ActiveBook.Warnings) > 0
		vars["book_warning_count"] = len(s.ActiveBook.Warnings)
		vars["has_multiple_book_warnings"] = len(s.ActiveBook.Warnings) > 1
		if len(s.ActiveBook.Warnings) > 0 {
			vars["first_book_warning"] = s.ActiveBook.Warnings[0]
		} else {
			vars["first_book_warning"] = ""
		}
		vars["book_warnings"] = warnItems
		isNarrated := !s.ActiveBook.IsScreenplay() && s.ActiveBook.Narrated
		vars["book_narrated"] = isNarrated
		if isNarrated {
			vars["book_narrated_label"] = s.tr(TrBookNarratedReqLabel)
			vars["book_narrated_short_label"] = s.tr(TrRequiredLabel)
		} else {
			vars["book_narrated_label"] = s.tr(TrBookNarratedOffLabel)
			vars["book_narrated_short_label"] = s.tr(TrDisabledLabel)
		}
		vars["book_imageless"] = s.ActiveBook.Imageless
		if s.ActiveBook.Imageless {
			vars["book_imageless_label"] = s.tr(TrBookImagelessOnLabel)
			vars["book_imageless_short_label"] = s.tr(TrNoImagesLabel)
		} else {
			vars["book_imageless_label"] = s.tr(TrBookImagelessOffLabel)
			vars["book_imageless_short_label"] = s.tr(TrWithImagesLabel)
		}

		if needsEditorFull {
			s.populateEditorContext(vars)
		} else if needsEditorPanelOnly {
			s.populateEditorSelectedNodeOnly(vars)
		}
	} else if needsEditorFull || needsEditorPanelOnly {
		vars["has_book_warnings"] = false
		vars["book_warning_count"] = 0
		vars["has_multiple_book_warnings"] = false
		vars["first_book_warning"] = ""
		vars["book_narrated"] = false
		vars["book_narrated_label"] = s.tr(TrBookNarratedOffLabel)
		vars["book_narrated_short_label"] = s.tr(TrDisabledLabel)
		vars["book_imageless"] = false
		vars["book_imageless_label"] = s.tr(TrBookImagelessOffLabel)
		vars["book_imageless_short_label"] = s.tr(TrWithImagesLabel)
		vars["has_selected_node"] = false
		vars["book_settings_open"] = false
		vars["has_selected_chapter"] = false
		vars["is_node_inspector_hidden"] = false
		s.populateEditorHistoryContext(vars)
	}

	if needsModals {
		s.populateModalsContext(vars)
	}

	return tpl.NewContext(vars)
}

func (s *Server) populateLibraryContext(vars map[string]any, books []*book.Book) {
	bookItems := make([]map[string]any, 0, len(books))
	for _, b := range books {
		save := book.LoadSaveState(s.SavesDir, b.ID, b.StartSceneID)
		if !s.isHistoryPreview() && s.PlayerBook != nil && s.PlayerBook.ID == b.ID && s.Save != nil && s.Save.UpdatedAtUnix >= save.UpdatedAtUnix {
			save = s.Save
		}
		hasProgress := len(save.History) > 0 || (save.CurrentSceneID != "" && save.CurrentSceneID != b.StartSceneID) || save.AudioTime > 0.5
		progressPct := 0
		if hasProgress && len(b.Scenes) > 0 {
			visited := make(map[string]bool)
			for _, h := range save.History {
				if h.SceneID != "" {
					visited[h.SceneID] = true
				}
			}
			if save.CurrentSceneID != "" {
				visited[save.CurrentSceneID] = true
			}
			progressPct = (len(visited) * 100) / len(b.Scenes)
			if progressPct < 1 {
				progressPct = 1
			}
			if progressPct > 100 {
				progressPct = 100
			}
		}
		bookItems = append(bookItems, map[string]any{
			"id":           b.ID,
			"title":        b.Title,
			"author":       b.Author,
			"description":  b.Description,
			"cover_url":    b.CoverURL,
			"progress_pct": progressPct,
		})
	}
	vars["books"] = bookItems
}
