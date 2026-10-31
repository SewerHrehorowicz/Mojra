package gui

import (
	"fmt"

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
		vars["theme_sun_icon_class"] = "hidden"
		vars["theme_moon_icon_class"] = ""
	} else {
		vars["theme_toggle_label"] = "Jasny"
		vars["theme_sun_icon_class"] = ""
		vars["theme_moon_icon_class"] = "hidden"
	}

	imagelessLocked := s.isBookImageless()
	effectiveImageless := s.isEffectiveImageless()
	vars["imageless_mode"] = effectiveImageless
	vars["imageless_locked"] = imagelessLocked
	if imagelessLocked {
		vars["imageless_locked_attr"] = "true"
		vars["imageless_disabled"] = "true"
	} else {
		vars["imageless_locked_attr"] = "false"
		vars["imageless_disabled"] = "false"
	}
	if effectiveImageless {
		vars["imageless_attr"] = "true"
		vars["imageless_class"] = "is-imageless"
		if imagelessLocked {
			vars["imageless_btn_class"] = "active is-disabled"
			vars["imageless_toggle_label"] = "Wyłączone (wymuszone)"
			vars["imageless_toggle_title"] = "Obrazy wyłączone w ustawieniach książki (imageless: true)"
		} else {
			vars["imageless_btn_class"] = "active"
			vars["imageless_toggle_label"] = "Wyłączone"
			vars["imageless_toggle_title"] = "Włącz obrazy (tryb bez obrazów aktywny)"
		}
		vars["imageless_icon_on_class"] = "hidden"
		vars["imageless_icon_off_class"] = ""
	} else {
		vars["imageless_attr"] = "false"
		vars["imageless_class"] = ""
		vars["imageless_btn_class"] = ""
		vars["imageless_icon_on_class"] = ""
		vars["imageless_icon_off_class"] = "hidden"
		vars["imageless_toggle_label"] = "Włączone"
		vars["imageless_toggle_title"] = "Wyłącz obrazy (tryb tekstowy na środku)"
	}

	uiScale := s.Settings.UIScale
	if uiScale != "large" {
		uiScale = "small"
	}
	vars["ui_scale"] = uiScale
	vars["is_large_ui"] = uiScale == "large"
	vars["ui_scale_is_large"] = uiScale == "large"
	if uiScale == "large" {
		vars["ui_scale_btn_class"] = "active"
		vars["zoom_in_icon_class"] = "hidden"
		vars["zoom_out_icon_class"] = ""
		vars["ui_scale_toggle_label"] = "Duży"
		vars["ui_scale_label"] = "Duży"
		vars["ui_scale_toggle_title"] = "Pomniejsz przyciski i kontrolki (- / Ctrl+-)"
	} else {
		vars["ui_scale_btn_class"] = ""
		vars["zoom_in_icon_class"] = ""
		vars["zoom_out_icon_class"] = "hidden"
		vars["ui_scale_toggle_label"] = "Mały"
		vars["ui_scale_label"] = "Mały"
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
		if previewMode == "full" {
			vars["mode_full_active"] = "active"
		}
		if previewMode == "portrait" {
			vars["mode_portrait_active"] = "active"
		}
		if previewMode == "landscape" {
			vars["mode_landscape_active"] = "active"
		}
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

	switch view {
	case "player":
		vars["tab_player_active"] = "active"
	case "editor":
		vars["tab_editor_active"] = "active"
	default:
		vars["tab_library_active"] = "active"
	}

	if s.ActiveBook != nil {
		vars["active_book_title"] = s.ActiveBook.Title
		vars["can_edit_active_book"] = !s.ActiveBook.IsPacked
	} else {
		vars["active_book_title"] = "Biblioteka"
		vars["can_edit_active_book"] = false
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
			vars["book_narrated_class"] = "active"
		} else {
			vars["book_narrated_label"] = "Narracja książki: WYŁĄCZONA (narrated: false)"
			vars["book_narrated_class"] = ""
		}
		vars["book_imageless"] = s.ActiveBook.Imageless
		if s.ActiveBook.Imageless {
			vars["book_imageless_label"] = "Tryb obrazów książki: BEZ OBRAZÓW (imageless: true)"
			vars["book_imageless_class"] = "active"
		} else {
			vars["book_imageless_label"] = "Tryb obrazów książki: Z OBRAZAMI (imageless: false)"
			vars["book_imageless_class"] = ""
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
		vars["book_narrated_class"] = ""
		vars["editor_pan_x"] = 0
		vars["editor_pan_y"] = 0
		vars["editor_zoom"] = "1.00"
		vars["has_editor_view"] = false
		vars["has_selected_node"] = false
		s.populateEditorHistoryContext(vars)
	}

	if needsModals {
		s.populateModalsContext(vars)
	}

	return tpl.NewContext(vars)
}

func (s *Server) populateLibraryContext(vars map[string]any, books []*book.Book) {
	vars["book_count"] = len(books)
	bookItems := make([]map[string]any, 0, len(books))
	for _, b := range books {
		save := book.LoadSaveState(s.SavesDir, b.ID, b.StartSceneID)
		if s.ActiveBook != nil && s.ActiveBook.ID == b.ID && s.Save != nil && s.Save.UpdatedAtUnix >= save.UpdatedAtUnix {
			save = s.Save
		}
		curName := ""
		for _, sc := range b.Scenes {
			if sc.ID == save.CurrentSceneID {
				curName = sc.DisplayName()
				break
			}
		}
		if curName == "" && len(b.Scenes) > 0 {
			curName = b.Scenes[0].DisplayName()
		}
		hasProgress := len(save.History) > 0 || (save.CurrentSceneID != "" && save.CurrentSceneID != b.StartSceneID) || save.AudioTime > 0.5
		playLabel := "Słuchaj od początku"
		if hasProgress {
			playLabel = "Kontynuuj słuchanie"
		}
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
		progressLabel := fmt.Sprintf("Postęp: %d%%", progressPct)
		bookmarkLabel := "Nie rozpoczęto"
		if hasProgress {
			if curName != "" {
				bookmarkLabel = fmt.Sprintf("Krok %d • %s", len(save.History)+1, curName)
			} else {
				bookmarkLabel = fmt.Sprintf("Krok %d", len(save.History)+1)
			}
		}
		storageLabel := "Katalog lokalny"
		if b.IsExternal {
			if b.IsPacked {
				storageLabel = "Dysk (.stbook / .zip): " + b.RootPath
			} else {
				storageLabel = "Dysk (folder): " + b.RootPath
			}
		} else if b.IsPacked {
			storageLabel = "Pakiet .stbook"
		}
		canRemove := b.IsExternal || b.ID != "nie-boj-sie-policji-rzecznej"
		canEdit := !b.IsPacked
		isActiveBook := s.ActiveBook != nil && s.ActiveBook.ID == b.ID
		activeCardClass := ""
		if isActiveBook {
			activeCardClass = "book-card-active"
		}
		narratedLabel := "Narracja: brak (narrated: false)"
		if b.Narrated {
			narratedLabel = "Narracja: pełna (narrated: true)"
		}
		warnItems := make([]map[string]any, 0, len(b.Warnings))
		for _, w := range b.Warnings {
			warnItems = append(warnItems, map[string]any{"text": w})
		}
		bookItems = append(bookItems, map[string]any{
			"id":                b.ID,
			"title":             b.Title,
			"author":            b.Author,
			"description":       b.Description,
			"cover_url":         b.CoverURL,
			"scene_count":       len(b.Scenes),
			"chapter_count":     len(b.Chapters),
			"narrated":          b.Narrated,
			"narrated_label":    narratedLabel,
			"imageless":         b.Imageless,
			"is_packed":         b.IsPacked,
			"is_external":       b.IsExternal,
			"can_edit":          canEdit,
			"can_remove":        canRemove,
			"is_active":         isActiveBook,
			"active_card_class": activeCardClass,
			"storage_label":     storageLabel,
			"has_progress":      hasProgress,
			"progress_pct":      progressPct,
			"progress_label":    progressLabel,
			"bookmark_label":    bookmarkLabel,
			"step_num":          len(save.History) + 1,
			"current_scene":     curName,
			"play_label":        playLabel,
			"has_warnings":      len(b.Warnings) > 0,
			"warning_count":     len(b.Warnings),
			"warnings":          warnItems,
		})
	}
	vars["books"] = bookItems
}
