package gui

import (
	"storyteller/internal/book"
	"storyteller/internal/tpl"
)

// BuildRenderContext constructs the data map consumed by the .tpl engine (`->` mappings, loops, and conditionals).
func (s *Server) BuildRenderContext(books []*book.Book) *tpl.RenderContext {
	return s.BuildTargetedRenderContext("app_shell", books)
}

// BuildTargetedRenderContext constructs only the data needed for the given template/chunk target.
func (s *Server) BuildTargetedRenderContext(targetTpl string, books []*book.Book) *tpl.RenderContext {
	vars := make(map[string]any)

	theme := s.Settings.Theme
	if theme != "light" {
		theme = "dark"
	}
	vars["theme"] = theme
	if theme == "light" {
		vars["theme_toggle_label"] = "Ciemny"
	} else {
		vars["theme_toggle_label"] = "Jasny"
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
			vars["imageless_toggle_label"] = "Wyłączone (wymuszone)"
			vars["imageless_toggle_title"] = "Obrazy wyłączone w ustawieniach książki (imageless: true)"
		} else {
			vars["imageless_toggle_label"] = "Wyłączone"
			vars["imageless_toggle_title"] = "Włącz obrazy (tryb bez obrazów aktywny)"
		}
	} else {
		vars["imageless_attr"] = "false"
		vars["imageless_toggle_label"] = "Włączone"
		vars["imageless_toggle_title"] = "Wyłącz obrazy (tryb tekstowy na środku)"
	}

	uiScale := s.Settings.UIScale
	if uiScale != "large" {
		uiScale = "small"
	}
	vars["ui_scale"] = uiScale
	vars["is_large_ui"] = uiScale == "large"
	if uiScale == "large" {
		vars["ui_scale_toggle_title"] = "Pomniejsz przyciski i kontrolki (- / Ctrl+-)"
	} else {
		vars["ui_scale_toggle_title"] = "Powiększ przyciski i kontrolki (+ / Ctrl++)"
	}

	vars["standalone_mode"] = s.StandaloneMode
	vars["has_active_book"] = s.ActiveBook != nil
	vars["dev_mode"] = s.DevMode
	if s.DevMode {
		previewMode := s.Settings.PreviewMode
		if previewMode != "portrait" && previewMode != "landscape" {
			previewMode = "full"
		}
		vars["preview_mode"] = previewMode
	} else {
		vars["preview_mode"] = "full"
	}

	view := s.Settings.ActiveView
	if s.StandaloneMode {
		view = "player"
	} else if view == "player" && s.ActiveBook == nil {
		view = "library"
	}
	vars["is_view_library"] = view == "library"
	vars["is_view_player"] = view == "player"
	vars["is_view_editor"] = view == "editor"

	if s.ActiveBook != nil {
		vars["active_book_title"] = s.ActiveBook.Title
	} else {
		vars["active_book_title"] = "Biblioteka"
	}

	vars["show_reset_progress"] = view == "player" && s.ActiveBook != nil

	needsLibrary := targetTpl == "library_view" || ((targetTpl == "index" || targetTpl == "app_shell") && view == "library")
	needsPlayer := targetTpl == "player_view" || ((targetTpl == "index" || targetTpl == "app_shell") && view == "player")
	needsEditorFull := targetTpl == "editor_view" || ((targetTpl == "index" || targetTpl == "app_shell") && view == "editor" && !s.StandaloneMode)
	needsEditorPanelOnly := targetTpl == "editor_context_panel" && !s.StandaloneMode
	needsModals := targetTpl == "modals" || targetTpl == "index" || targetTpl == "app_shell"

	if needsLibrary {
		s.populateLibraryContext(vars, books)
	}

	if s.ActiveBook != nil && len(s.ActiveBook.Scenes) > 0 && (needsPlayer || needsEditorFull || needsEditorPanelOnly) {
		warnItems := make([]map[string]any, 0, len(s.ActiveBook.Warnings))
		for _, w := range s.ActiveBook.Warnings {
			warnItems = append(warnItems, map[string]any{"text": w})
		}
		vars["has_book_warnings"] = len(s.ActiveBook.Warnings) > 0
		vars["book_warning_count"] = len(s.ActiveBook.Warnings)
		vars["book_warnings"] = warnItems
		vars["book_narrated"] = s.ActiveBook.Narrated
		if s.ActiveBook.Narrated {
			vars["book_narrated_label"] = "Narracja książki: WYMAGANA (narrated: true)"
		} else {
			vars["book_narrated_label"] = "Narracja książki: WYŁĄCZONA (narrated: false)"
		}
		vars["book_imageless"] = s.ActiveBook.Imageless
		if s.ActiveBook.Imageless {
			vars["book_imageless_label"] = "Tryb obrazów książki: BEZ OBRAZÓW (imageless: true)"
		} else {
			vars["book_imageless_label"] = "Tryb obrazów książki: Z OBRAZAMI (imageless: false)"
		}

		if needsPlayer {
			s.populatePlayerContext(vars)
		}
		if needsEditorFull {
			s.populateEditorContext(vars)
		} else if needsEditorPanelOnly {
			s.populateEditorSelectedNodeOnly(vars)
		}
	} else if needsEditorFull || needsEditorPanelOnly {
		vars["has_book_warnings"] = false
		vars["book_warning_count"] = 0
		vars["book_narrated"] = false
		vars["book_narrated_label"] = "Narracja książki: WYŁĄCZONA (narrated: false)"
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
		if s.ActiveBook != nil && s.ActiveBook.ID == b.ID && s.Save != nil && s.Save.UpdatedAtUnix >= save.UpdatedAtUnix {
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
		warnItems := make([]map[string]any, 0, len(b.Warnings))
		for _, w := range b.Warnings {
			warnItems = append(warnItems, map[string]any{"text": w})
		}
		bookItems = append(bookItems, map[string]any{
			"id":            b.ID,
			"title":         b.Title,
			"author":        b.Author,
			"description":   b.Description,
			"cover_url":     b.CoverURL,
			"progress_pct":  progressPct,
			"has_warnings":  len(b.Warnings) > 0,
			"warning_count": len(b.Warnings),
			"warnings":      warnItems,
		})
	}
	vars["books"] = bookItems
}
