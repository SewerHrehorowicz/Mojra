//go:build !(js && wasm)

package gui

import (
	"fmt"
	"path/filepath"
	"strings"

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
	vars["is_dark"] = theme != "light"
	if theme == "light" {
		vars["theme_toggle_label"] = s.tr(TrThemeDarkLabel)
	} else {
		vars["theme_toggle_label"] = s.tr(TrThemeLightLabel)
	}

	imagelessLocked := s.isBookImageless()
	effectiveImageless := s.isEffectiveImageless()
	vars["imageless_mode"] = effectiveImageless
	vars["imageless"] = effectiveImageless
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
		} else {
			vars["imageless_toggle_label"] = s.tr(TrImagelessOffLabel)
		}
	} else {
		vars["imageless_attr"] = "false"
		vars["imageless_toggle_label"] = s.tr(TrImagelessOnLabel)
	}

	uiScale := s.Settings.UIScale
	if uiScale != "large" {
		uiScale = "small"
	}
	vars["ui_scale"] = uiScale
	vars["is_small_ui"] = uiScale == "small"
	vars["is_large_ui"] = uiScale == "large"

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

	hasEditorBook := s.hasActiveStudioEditor()
	vars["standalone_mode"] = s.StandaloneMode
	vars["has_editor_book"] = hasEditorBook
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
	if hasEditorBook {
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

	showZenMode := view == "player" && s.PlayerBook != nil && !s.PlayerBook.IsScreenplay()
	zenMode := showZenMode && s.ZenMode
	vars["show_zen_mode"] = showZenMode
	vars["zen_mode"] = zenMode
	if zenMode {
		vars["zen_mode_attr"] = "true"
		vars["zen_toggle_title"] = s.tr(TrScaleExitZenHint)
	} else {
		vars["zen_mode_attr"] = "false"
		vars["zen_toggle_title"] = s.tr(TrScaleZenHint)
	}
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

	if hasEditorBook && len(s.ActiveBook.Scenes) > 0 && (needsEditorFull || needsEditorPanelOnly) {
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
		vars["studio_search_count"] = 0
		vars["studio_search_items"] = []map[string]any(nil)
		s.populateEditorHistoryContext(vars)
		if needsEditorFull {
			s.populateStudioHomeContext(vars, books)
		}
	}

	if needsModals {
		s.populateModalsContext(vars)
	}

	return tpl.NewContext(vars)
}

func (s *Server) populateLibraryContext(vars map[string]any, books []*book.Book) {
	bookItems := make([]map[string]any, 0, len(books))
	seenTitleKeys := make(map[string]bool, len(books))
	for _, b := range books {
		titleKey := book.SanitizeBookID(b.Title)
		if titleKey != "" {
			if seenTitleKeys[titleKey] {
				continue
			}
			seenTitleKeys[titleKey] = true
		}
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

func (s *Server) populateStudioHomeContext(vars map[string]any, books []*book.Book) {
	lang := s.lang()
	items := make([]map[string]any, 0, len(books))
	unpackedByID := make(map[string]bool, len(books))
	unpackedByTitle := make(map[string]bool, len(books))

	for _, b := range books {
		if b.IsPacked {
			continue
		}
		titleKey := book.SanitizeBookID(b.Title)
		if titleKey != "" && unpackedByTitle[titleKey] {
			continue
		}
		unpackedByID[b.ID] = true
		if titleKey != "" {
			unpackedByTitle[titleKey] = true
		}
		items = append(items, map[string]any{
			"id":        b.ID,
			"title":     b.Title,
			"author":    b.Author,
			"meta_line": formatStudioProjectMetaLine(b, lang),
			"path_line": formatStudioProjectPathLine(b),
		})
	}

	var extPaths []string
	if s.Settings != nil {
		extPaths = s.Settings.ExternalBooks
	}
	packedBooks := book.LoadPackedBooks(s.BooksDir, extPaths)
	seenPackedRoots := make(map[string]bool, len(packedBooks))
	for _, pb := range packedBooks {
		seenPackedRoots[filepath.Clean(pb.RootPath)] = true
	}
	for _, b := range books {
		if b.IsPacked && !seenPackedRoots[filepath.Clean(b.RootPath)] {
			seenPackedRoots[filepath.Clean(b.RootPath)] = true
			packedBooks = append(packedBooks, b)
		}
	}

	packedItems := make([]map[string]any, 0, len(packedBooks))
	for _, pb := range packedBooks {
		titleKey := book.SanitizeBookID(pb.Title)
		isUnpacked := unpackedByID[pb.ID] || (titleKey != "" && unpackedByTitle[titleKey])
		packedItems = append(packedItems, map[string]any{
			"id":          pb.ID,
			"title":       pb.Title,
			"author":      pb.Author,
			"meta_line":   formatStudioProjectMetaLine(pb, lang),
			"path_line":   formatStudioProjectPathLine(pb),
			"is_unpacked": isUnpacked,
		})
	}

	vars["has_studio_home_projects"] = len(items) > 0
	vars["studio_home_projects"] = items
	vars["has_studio_packed_books"] = len(packedItems) > 0
	vars["studio_packed_books"] = packedItems
}

func formatStudioProjectMetaLine(b *book.Book, lang Lang) string {
	var typePart, scenesPart, chPart, metricPart string
	if b.IsScreenplay() {
		std := strings.ToUpper(b.NormalizedStandard())
		if std == "PL" {
			std = "PISF"
		}
		if lang == LangEN {
			typePart = "SCREENPLAY (" + std + ")"
		} else {
			typePart = "SCENARIUSZ FILMOWY (" + std + ")"
		}
	} else {
		if lang == LangEN {
			typePart = "INTERACTIVE BOOK"
		} else {
			typePart = "KSIĄŻKA INTERAKTYWNA"
		}
	}

	scCount := len(b.Scenes)
	if lang == LangEN {
		if scCount == 1 {
			scenesPart = "1 SCENE"
		} else {
			scenesPart = fmt.Sprintf("%d SCENES", scCount)
		}
	} else {
		if scCount == 1 {
			scenesPart = "1 SCENA"
		} else if scCount%10 >= 2 && scCount%10 <= 4 && (scCount%100 < 12 || scCount%100 > 14) {
			scenesPart = fmt.Sprintf("%d SCENY", scCount)
		} else {
			scenesPart = fmt.Sprintf("%d SCEN", scCount)
		}
	}

	chCount := len(b.Chapters)
	if chCount < 1 {
		chCount = 1
	}
	if b.IsScreenplay() {
		if lang == LangEN {
			if chCount == 1 {
				chPart = "1 ACT"
			} else {
				chPart = fmt.Sprintf("%d ACTS", chCount)
			}
		} else {
			if chCount == 1 {
				chPart = "1 AKT"
			} else if chCount%10 >= 2 && chCount%10 <= 4 && (chCount%100 < 12 || chCount%100 > 14) {
				chPart = fmt.Sprintf("%d AKTY", chCount)
			} else {
				chPart = fmt.Sprintf("%d AKTÓW", chCount)
			}
		}
	} else {
		if lang == LangEN {
			if chCount == 1 {
				chPart = "1 CHAPTER"
			} else {
				chPart = fmt.Sprintf("%d CHAPTERS", chCount)
			}
		} else {
			if chCount == 1 {
				chPart = "1 ROZDZIAŁ"
			} else if chCount%10 >= 2 && chCount%10 <= 4 && (chCount%100 < 12 || chCount%100 > 14) {
				chPart = fmt.Sprintf("%d ROZDZIAŁY", chCount)
			} else {
				chPart = fmt.Sprintf("%d ROZDZIAŁÓW", chCount)
			}
		}
	}

	if b.IsPacked {
		if lang == LangEN {
			metricPart = ".PLAYBOOK (READ-ONLY)"
		} else {
			metricPart = ".PLAYBOOK (TYLKO ODCZYT)"
		}
	} else if b.IsScreenplay() {
		bd := b.ScreenplayBreakdown()
		metricPart = fmt.Sprintf("%s (1/8)", strings.ToUpper(bd.TotalPagesFormatted))
	} else if b.Narrated {
		if lang == LangEN {
			metricPart = "FLAC NARRATION"
		} else {
			metricPart = "NARRACJA FLAC"
		}
	} else if b.Imageless {
		if lang == LangEN {
			metricPart = "IMAGELESS"
		} else {
			metricPart = "BEZ OBRAZÓW"
		}
	} else {
		if lang == LangEN {
			metricPart = "ILLUSTRATED"
		} else {
			metricPart = "ILUSTROWANA"
		}
	}

	return strings.Join([]string{typePart, scenesPart, chPart, metricPart}, "  ·  ")
}

func formatStudioProjectPathLine(b *book.Book) string {
	if b.IsPacked {
		return "books/" + filepath.Base(b.RootPath)
	}
	parent := filepath.Base(filepath.Dir(filepath.Clean(b.RootPath)))
	if parent != "books" && parent != "projects" {
		parent = "projects"
	}
	return parent + "/" + b.ID + "/book.playtxt"
}
