package gui

import (
	"fmt"
	"strings"

	"storyteller/internal/book"
)

func buildScreenplayFormatInsertPills(standard string, b *book.Book) ([]map[string]any, []map[string]any) {
	normStd := book.NormalizeScriptStandard(standard)
	var insertPills []map[string]any
	if normStd == "pl" {
		insertPills = []map[string]any{
			{"kind": "dialogue", "label": "+ Dialog", "hint": "Wstaw blok postaci i kwestii dialogowej"},
			{"kind": "parenthetical", "label": "+ Didaskalia (wtrącenie)", "hint": "Wstaw wskazówkę aktorską w nawiasie, np. (cicho)"},
			{"kind": "action", "label": "+ Opis akcji", "hint": "Wstaw nowy akapit opisu akcji (didaskalia sceniczne)"},
			{"kind": "dialogue_off", "label": "+ Dialog (OFF)", "hint": "Wstaw kwestię postaci spoza kadru (OFF)"},
			{"kind": "dialogue_vo", "label": "+ Narracja (V.O.)", "hint": "Wstaw głos spoza obrazu / narratora (V.O.)"},
			{"kind": "transition_cut", "label": "+ CIĘCIE DO:", "hint": "Wstaw przejście montażowe CIĘCIE DO:"},
			{"kind": "transition_fade", "label": "+ ŚCIEMNIENIE.", "hint": "Wstaw przejście ŚCIEMNIENIE."},
			{"kind": "subslug", "label": "+ Podscena (WN./PL.)", "hint": "Wstaw śródtytuł podsceny w tej samej scenie"},
		}
	} else {
		insertPills = []map[string]any{
			{"kind": "dialogue", "label": "+ Dialog", "hint": "Wstaw blok postaci i kwestii dialogowej"},
			{"kind": "parenthetical", "label": "+ Didaskalia (...)", "hint": "Wstaw wskazówkę aktorską w nawiasie"},
			{"kind": "action", "label": "+ Opis akcji", "hint": "Wstaw nowy akapit opisu akcji"},
			{"kind": "dialogue_off", "label": "+ Dialog (O.S.)", "hint": "Wstaw kwestię postaci spoza kadru (O.S.)"},
			{"kind": "dialogue_vo", "label": "+ Narracja (V.O.)", "hint": "Wstaw głos narratora (V.O.)"},
			{"kind": "transition_cut", "label": "+ CUT TO:", "hint": "Wstaw przejście montażowe CUT TO:"},
			{"kind": "transition_fade", "label": "+ FADE OUT.", "hint": "Wstaw przejście FADE OUT."},
			{"kind": "subslug", "label": "+ Podscena (INT./EXT.)", "hint": "Wstaw śródtytuł podsceny"},
		}
	}

	var charPills []map[string]any
	if b != nil {
		bd := book.AnalyzeScreenplay(b)
		limit := len(bd.Characters)
		if limit > 8 {
			limit = 8
		}
		for i := 0; i < limit; i++ {
			chName := bd.Characters[i].Name
			charPills = append(charPills, map[string]any{
				"name":  chName,
				"label": "@ " + chName,
				"hint":  "Wstaw kwestię dialogową postaci " + chName,
			})
		}
	}
	return insertPills, charPills
}

func buildExportFormatOptions(rawFormat string, isScreenplay bool) (string, string, []map[string]any) {
	norm := strings.ToLower(strings.TrimSpace(rawFormat))
	if norm == "stbook" {
		norm = "playbook"
	}
	if isScreenplay {
		if norm != "pdf" && norm != "fountain" && norm != "playbook" {
			norm = "pdf"
		}
		defs := []struct {
			val   string
			label string
		}{
			{"pdf", "Scenariusz PDF (.pdf)"},
			{"fountain", "Scenariusz Fountain (.fountain)"},
			{"playbook", "Playbook (.playbook)"},
		}
		var opts []map[string]any
		activeLabel := defs[0].label
		for _, d := range defs {
			active := d.val == norm
			if active {
				activeLabel = d.label
			}
			opts = append(opts, map[string]any{
				"value":     d.val,
				"label":     d.label,
				"is_active": active,
			})
		}
		return norm, activeLabel, opts
	}

	norm = "playbook"
	activeLabel := "Playbook (.playbook)"
	opts := []map[string]any{
		{
			"value":     "playbook",
			"label":     activeLabel,
			"is_active": true,
		},
	}
	return norm, activeLabel, opts
}

func (s *Server) populateModalsContext(vars map[string]any) {
	if s.QRModalOpen && s.DevMode && !s.StandaloneMode {
		lanIP := DetectLANIP()
		port := s.Port
		if port == 0 {
			port = 8484
		}
		vars["lan_url"] = fmt.Sprintf("http://%s:%d", lanIP, port)
	}
	vars["show_qr_modal"] = s.QRModalOpen
	vars["show_confirm_modal"] = s.ConfirmPendingAction != ""
	vars["confirm_title"] = s.ConfirmTitle
	vars["confirm_message"] = s.ConfirmMessage
	vars["confirm_is_danger"] = s.ConfirmPendingAction == "RemoveBook" || s.ConfirmPendingAction == "DeleteScene" || s.ConfirmPendingAction == "DeleteScenes" || s.ConfirmPendingAction == "RemoveChoice"
	vars["show_new_book_modal"] = s.NewBookModalOpen
	vars["show_export_modal"] = s.ExportModalOpen
	vars["show_add_book_modal"] = s.AddBookModalOpen
	vars["show_studio_project_modal"] = s.StudioProjectModalOpen
	vars["show_editor_help_modal"] = s.EditorHelpModalOpen

	if s.StudioProjectModalOpen && !s.StandaloneMode {
		allBooks, _ := s.loadBooks()
		studioProjects := make([]map[string]any, 0, len(allBooks))
		for _, b := range allBooks {
			kindBadge := "PROJEKT"
			if b.IsPacked {
				kindBadge = ".PLAYBOOK (TYLKO ODCZYT)"
			}
			isCurrent := s.ActiveBook != nil && s.ActiveBook.ID == b.ID && !b.IsPacked
			studioProjects = append(studioProjects, map[string]any{
				"id":         b.ID,
				"title":      b.Title,
				"author":     b.Author,
				"license":    b.EffectiveLicense(),
				"is_packed":  b.IsPacked,
				"kind_badge": kindBadge,
				"is_current": isCurrent,
			})
		}
		vars["has_studio_projects"] = len(studioProjects) > 0
		vars["studio_projects"] = studioProjects
	} else {
		vars["has_studio_projects"] = false
	}

	isScreenplayBook := s.ActiveBook != nil && s.ActiveBook.IsScreenplay()
	exportFmt, exportFmtLabel, exportFmtOpts := buildExportFormatOptions(s.ExportModalFormat, isScreenplayBook)
	vars["export_modal_format"] = exportFmt
	vars["export_modal_format_label"] = exportFmtLabel
	vars["export_modal_format_options"] = exportFmtOpts
	exportDir := strings.TrimSpace(s.ExportModalDir)
	if exportDir == "" {
		exportDir = s.defaultExportDir()
	}
	vars["export_modal_dir"] = exportDir

	draftMode := book.NormalizeBookMode(s.NewBookDraftMode)
	draftStd := book.NormalizeScriptStandard(s.NewBookDraftStandard)
	newModeLabel, newModeOpts := buildBookModeOptions(draftMode)
	newStdLabel, newStdOpts := buildScriptStandardOptions(draftStd)
	vars["new_book_mode"] = draftMode
	vars["new_book_is_screenplay"] = draftMode == "screenplay"
	vars["new_book_mode_label"] = newModeLabel
	vars["new_book_mode_options"] = newModeOpts
	vars["new_book_standard"] = draftStd
	vars["new_book_standard_label"] = newStdLabel
	vars["new_book_standard_options"] = newStdOpts
	vars["new_book_title"] = s.NewBookDraftTitle
	vars["new_book_author"] = s.NewBookDraftAuthor
	vars["new_book_narrated"] = s.NewBookDraftNarrated
	if s.NewBookDraftNarrated {
		vars["new_book_narrated_val"] = "true"
		vars["new_book_narrated_short_label"] = "Wymagana"
	} else {
		vars["new_book_narrated_val"] = "false"
		vars["new_book_narrated_short_label"] = "Wyłączona"
	}
	vars["new_book_imageless"] = s.NewBookDraftImageless
	if s.NewBookDraftImageless {
		vars["new_book_imageless_val"] = "true"
		vars["new_book_imageless_short_label"] = "Bez obrazów"
	} else {
		vars["new_book_imageless_val"] = "false"
		vars["new_book_imageless_short_label"] = "Z obrazami"
	}
	vars["new_book_default_dir"] = s.defaultNewBookDir()
	if draftMode == "screenplay" {
		vars["new_book_title_placeholder"] = "np. Ostatni Rejs"
	} else {
		vars["new_book_title_placeholder"] = "np. Tajemnica Starego Młyna"
	}

	if s.SceneTextModalID != "" {
		if modalSc := s.findScene(s.SceneTextModalID); modalSc != nil {
			vars["show_scene_text_modal"] = true
			vars["scene_text_modal_id"] = modalSc.ID
			vars["scene_text_modal_name"] = modalSc.DisplayName()
			vars["scene_text_modal_raw_name"] = modalSc.Name
			vars["scene_text_modal_explicit_id"] = modalSc.ExplicitID
			vars["scene_text_modal_slugline"] = modalSc.Slugline
			vars["scene_text_modal_synopsis"] = modalSc.Synopsis
			vars["scene_text_modal_text"] = modalSc.Text

			if s.ActiveBook != nil && s.ActiveBook.IsScreenplay() {
				normStd := s.ActiveBook.NormalizedStandard()
				metrics := book.ComputeSceneScriptMetrics(modalSc, modalSc.OrderIndex+1, normStd)
				vars["scene_text_modal_length_label"] = metrics.FormattedLength
				prefixPills, timePills := buildSluglineHelperPills(normStd)
				vars["slugline_prefix_pills"] = prefixPills
				vars["slugline_time_pills"] = timePills
				insertPills, charPills := buildScreenplayFormatInsertPills(normStd, s.ActiveBook)
				vars["screenplay_insert_pills"] = insertPills
				vars["screenplay_character_pills"] = charPills
				vars["has_screenplay_character_pills"] = len(charPills) > 0
			} else {
				vars["scene_text_modal_length_label"] = ""
				vars["has_screenplay_character_pills"] = false
			}

			isStart := false
			if s.ActiveBook != nil {
				if s.ActiveBook.StartSceneID != "" {
					isStart = strings.EqualFold(modalSc.ID, s.ActiveBook.StartSceneID)
				} else if len(s.ActiveBook.Scenes) > 0 {
					isStart = modalSc.ID == s.ActiveBook.Scenes[0].ID
				}
			}
			vars["scene_text_modal_is_start"] = isStart

			curChName := strings.TrimSpace(modalSc.Chapter)
			if curChName == "" {
				if s.ActiveBook != nil && s.ActiveBook.IsScreenplay() {
					curChName = "Akt I"
				} else {
					curChName = "Rozdział 1"
				}
			}
			chKey := "rozdział:"
			if s.ActiveBook != nil && s.ActiveBook.IsScreenplay() {
				chKey = "akt:"
			}
			var modalChOptions []map[string]any
			if s.ActiveBook != nil {
				modalChOptions = make([]map[string]any, 0, len(s.ActiveBook.Chapters)+1)
				matchedCh := false
				for _, chItem := range s.ActiveBook.Chapters {
					isAct := strings.EqualFold(curChName, strings.TrimSpace(chItem.Name))
					if isAct {
						matchedCh = true
					}
					modalChOptions = append(modalChOptions, map[string]any{
						"value":     chItem.Name,
						"label":     chItem.Name,
						"is_active": isAct,
					})
				}
				if !matchedCh && curChName != "" {
					modalChOptions = append(modalChOptions, map[string]any{
						"value":     curChName,
						"label":     curChName,
						"is_active": true,
					})
				}
			}
			vars["scene_text_modal_chapter"] = curChName
			vars["scene_text_modal_chapter_key"] = chKey
			vars["scene_text_modal_chapter_options"] = modalChOptions

			curImg, curImgLabel, imgOptions, curVoice, curVoiceLabel, voiceOptions := s.buildSceneMediaOptions(modalSc, true)
			vars["scene_text_modal_current_image"] = curImg
			vars["scene_text_modal_current_image_label"] = curImgLabel
			vars["scene_text_modal_image_options"] = imgOptions
			vars["scene_text_modal_book_narrated"] = s.ActiveBook != nil && !s.ActiveBook.IsScreenplay() && s.ActiveBook.Narrated
			vars["scene_text_modal_current_voice"] = curVoice
			vars["scene_text_modal_current_voice_label"] = curVoiceLabel
			vars["scene_text_modal_voice_options"] = voiceOptions

			settingsOpenVal := "false"
			if s.SceneTextModalSettingsOpen {
				settingsOpenVal = "true"
			}
			hasBookVars := s.ActiveBook != nil && len(s.ActiveBook.Variables) > 0
			vars["has_book_variables"] = hasBookVars
			isContextPanelHidden := !s.BookSettingsOpen && (s.SelectedChapter == "" || s.ActiveBook == nil || s.ActiveBook.FindChapter(s.SelectedChapter) == nil)
			vars["scene_text_has_right_panel"] = !isContextPanelHidden || s.EditorVariablesPanelOpen || s.EditorBreakdownPanelOpen || s.EditorHistoryPanelOpen || s.EditorHelpModalOpen
			isFullWidth := s.SceneTextModalWidthMode == "full"
			vars["scene_text_is_full_width"] = isFullWidth
			vars["scene_text_is_half_width"] = !isFullWidth
			vars["scene_text_modal_settings_open"] = s.SceneTextModalSettingsOpen
			vars["scene_text_modal_settings_open_val"] = settingsOpenVal
			effModalStr := book.CleanEffectsExpr(modalSc.Effects)
			effModalErr := ""
			if effModalStr != "" {
				effModalErr = book.ExplainEffectsExprError(modalSc.Effects, s.ActiveBook)
			}
			vars["scene_text_modal_effects"] = effModalStr
			vars["scene_text_modal_has_effects"] = effModalStr != ""
			vars["scene_text_modal_effects_invalid"] = effModalErr != ""
			vars["scene_text_modal_effects_error_hint"] = effModalErr
			vars["scene_text_modal_effects_disabled"] = s.isHistoryPreview() || !hasBookVars
			vars["book_vars_spec"] = book.EncodeBookVariablesSpec(s.ActiveBook)
			vars["book_var_suggestions"] = buildBookVarSuggestions(s.ActiveBook)
			vars["book_enum_suggestions"] = buildBookEnumValueSuggestions(s.ActiveBook)
			vars["book_target_suggestions"] = s.buildBookTargetSuggestions(modalSc)
			vars["show_syntax_help_popover"] = hasBookVars && s.SyntaxHelpPopoverOpen
			vars["syn_open"] = "{{"
			vars["syn_close"] = "}}"
			vars["syntax_help_title"] = "Składnia exit: / if: / set:"
			vars["syntax_help_cond_rows"] = []map[string]any{
				{"code": "if: odkryta_prawda && sumienie >= 2", "desc": "bool / int / enum (&&, ||, !, ==, !=, >=, <=)"},
				{"code": "set: odkryta_prawda = true; sumienie++", "desc": "efekty rozdzielone średnikiem ; (=, +=, -=, ++, --)"},
				{"code": "exit: ogien_w_kaplicy", "desc": "wyjście do sceny, chapter:<id> lub END"},
			}
			vars["syntax_help_eff_rows"] = []map[string]any{
				{"code": "set: !klucz; klucz = false", "desc": "bool toggle (!zmienna) lub jawnie (= true / false)"},
				{"code": "set: naboje -= 2; frakcja = rzeczni", "desc": "int (+=, -=, ++, --) lub enum (=)"},
			}

			previewMode := s.SceneTextPreviewMode
			if !hasBookVars || (previewMode != "highlight" && previewMode != "output") {
				previewMode = "normal"
			}
			vars["scene_text_preview_mode"] = previewMode
			vars["is_scene_preview_normal"] = previewMode == "normal"
			vars["is_scene_preview_highlight"] = previewMode == "highlight"
			vars["is_scene_preview_output"] = previewMode == "output"

			testState := book.EnsureSaveVariables(s.ActiveBook, s.SceneTextTestVars)
			s.SceneTextTestVars = testState
			var testVarItems []map[string]any
			if s.ActiveBook != nil {
				book.ResolveBookVariableTypes(s.ActiveBook)
				for _, vd := range s.ActiveBook.Variables {
					curVal := testState[vd.Name]
					isTrue := curVal == "true"
					var enumOpts []map[string]any
					if vd.Kind == book.VarKindEnum {
						for _, opt := range vd.Options {
							enumOpts = append(enumOpts, map[string]any{
								"value":     opt,
								"label":     opt,
								"is_active": strings.EqualFold(opt, curVal),
							})
						}
					}
					testVarItems = append(testVarItems, map[string]any{
						"name":         vd.Name,
						"kind":         vd.Kind,
						"is_bool":      vd.Kind == book.VarKindBool,
						"is_int":       vd.Kind == book.VarKindInt,
						"is_enum":      vd.Kind == book.VarKindEnum,
						"bool_val":     isTrue,
						"value":        curVal,
						"enum_options": enumOpts,
					})
				}
			}
			vars["scene_test_vars"] = testVarItems
			vars["has_scene_test_vars"] = len(testVarItems) > 0
			vars["show_scene_test_vars_bar"] = s.SceneTestVarsPopoverOpen || (previewMode == "highlight" && len(testVarItems) > 0)
			vars["scene_test_vars_spec"] = book.EncodeVariablesStateSpec(testState, s.ActiveBook)
			vars["scene_text_modal_hl_segments"] = book.BuildSceneProseHighlightSegments(modalSc.Text, testState, s.ActiveBook)
			vars["scene_text_modal_preview_paragraphs"] = book.BuildSceneProsePreviewParagraphs(modalSc.Text, previewMode, testState, s.ActiveBook)
			vars["scene_text_modal_output_html"] = book.RenderSceneProseHTML(modalSc, testState, s.ActiveBook)

			modalHeaderMeta := ""
			if s.ActiveBook != nil {
				modalHeaderMeta = s.ActiveBook.Title
			}
			if modalSc.Chapter != "" {
				if modalHeaderMeta != "" {
					modalHeaderMeta += " • " + modalSc.Chapter
				} else {
					modalHeaderMeta = modalSc.Chapter
				}
			}
			vars["scene_text_modal_header_meta"] = modalHeaderMeta

			imageless := s.isEffectiveImageless()
			modalImgURL := ""
			if !imageless {
				modalImgURL = modalSc.ImageURL
				if modalImgURL == "" {
					modalImgURL = s.resolveInheritedSceneImageURL(modalSc)
				}
			}
			vars["scene_text_modal_has_image"] = modalImgURL != ""
			vars["scene_text_modal_image_url"] = modalImgURL
			vars["scene_text_modal_is_imageless"] = imageless || modalImgURL == ""

			var incomingScenes []map[string]any
			if s.ActiveBook != nil {
				seenIncoming := make(map[string]bool)
				for _, sc := range s.ActiveBook.Scenes {
					if sc == nil || sc.ID == modalSc.ID {
						continue
					}
					scHasMulti := len(sc.Choices) > 1
					for _, ch := range sc.Choices {
						if ch.IsEnding || !strings.EqualFold(ch.TargetSceneID, modalSc.ID) {
							continue
						}
						choiceLbl := strings.TrimSpace(ch.Label)
						if !scHasMulti && (ch.IsImplicit || strings.EqualFold(choiceLbl, "Dalej")) {
							choiceLbl = ""
						}
						dedupKey := sc.ID + "\x00" + choiceLbl
						if seenIncoming[dedupKey] {
							continue
						}
						seenIncoming[dedupKey] = true
						incomingScenes = append(incomingScenes, map[string]any{
							"scene_id":         sc.ID,
							"scene_name":       sc.DisplayName(),
							"choice_label":     choiceLbl,
							"has_choice_label": choiceLbl != "",
						})
					}
				}
			}
			vars["scene_text_modal_incoming"] = incomingScenes
			vars["scene_text_modal_has_incoming"] = len(incomingScenes) > 0

			startID := ""
			if s.ActiveBook != nil {
				startID = s.ActiveBook.StartSceneID
				if startID == "" && len(s.ActiveBook.Scenes) > 0 {
					startID = s.ActiveBook.Scenes[0].ID
				}
			}
			vars["scene_text_modal_start_id"] = startID
			isTerminal := len(modalSc.Choices) == 0 || modalSc.ExplicitEnd
			vars["scene_text_modal_is_terminal"] = isTerminal
			termVal, termLabel, termOpts := s.buildChoiceTargetOptions(
				modalSc,
				s.terminalChoiceDescriptor(),
			)
			vars["scene_text_modal_terminal_selected_value"] = termVal
			vars["scene_text_modal_terminal_selected_label"] = termLabel
			vars["scene_text_modal_terminal_port_filled"] = termVal != "" && !strings.EqualFold(termVal, "END")
			vars["scene_text_modal_terminal_target_options"] = termOpts

			choicesSource := modalSc.Choices
			if isTerminal {
				choicesSource = []book.Choice{
					{
						Label:      "Dalej",
						Target:     "END",
						IsEnding:   true,
						IsImplicit: true,
					},
				}
			}
			hasMulti := len(choicesSource) > 1
			modalChoices := make([]map[string]any, 0, len(choicesSource))
			for idx, ch := range choicesSource {
				lbl := ch.Label
				if !hasMulti && lbl == "" {
					lbl = "Dalej"
				}
				arr := "»"
				if ch.IsEnding {
					arr = "↺"
				}
				selectedVal, selectedLabel, targetOptions := s.buildChoiceTargetOptions(modalSc, ch)
				isFilled := !ch.IsEnding &&
					((ch.TargetSceneID != "" && !strings.EqualFold(ch.TargetSceneID, "END")) ||
						(ch.IsChapterLink && ch.TargetChapterID != "")) &&
					!strings.EqualFold(selectedVal, "END")
				condInputVal := book.CleanConditionExpr(ch.Condition)
				if ch.ShowWhenLocked && condInputVal != "" && !strings.HasPrefix(condInputVal, "?") {
					condInputVal = "? " + condInputVal
				}
				effInputVal := book.CleanEffectsExpr(ch.Effects)
				hasCond := condInputVal != ""
				hasEff := effInputVal != ""
				hasRule := hasCond || hasEff
				ruleKey := fmt.Sprintf("%s:%d", modalSc.ID, idx)
				isRuleOpen := hasBookVars
				if hasBookVars && s.ChoiceRulesOpen != nil {
					if expOpen, exists := s.ChoiceRulesOpen[ruleKey]; exists {
						isRuleOpen = expOpen
					}
				}
				condMet := book.EvaluateCondition(ch.Condition, testState, s.ActiveBook)
				if previewMode == "output" && hasCond && !condMet && !ch.ShowWhenLocked {
					continue
				}
				condErrHint := ""
				if hasCond {
					condErrHint = book.ExplainConditionExprError(ch.Condition, s.ActiveBook)
				}
				effErrHint := ""
				if hasEff {
					effErrHint = book.ExplainEffectsExprError(ch.Effects, s.ActiveBook)
				}
				arrowSuffix := ""
				if isFilled && selectedVal != "" && !strings.EqualFold(selectedVal, "END") {
					arrowSuffix = " -> " + selectedVal
				}
				inlineVal := lbl + arrowSuffix
				targetScID := ch.TargetSceneID
				if ch.IsEnding && targetScID == "" {
					targetScID = startID
				}
				modalChoices = append(modalChoices, map[string]any{
					"idx":               idx,
					"label":             lbl,
					"arrow_suffix":      arrowSuffix,
					"inline_value":      inlineVal,
					"arrow":             arr,
					"is_branch":         !ch.IsEnding,
					"is_editable":       true,
					"target_scene_id":   targetScID,
					"selected_value":    selectedVal,
					"selected_label":    selectedLabel,
					"is_filled":         isFilled,
					"target_options":    targetOptions,
					"condition":         condInputVal,
					"effects":           effInputVal,
					"has_condition":     hasCond,
					"has_effects":       hasEff,
					"has_rule":          hasRule,
					"is_rule_open":      isRuleOpen,
					"is_cond_invalid":   condErrHint != "",
					"is_eff_invalid":    effErrHint != "",
					"cond_error_hint":   condErrHint,
					"eff_error_hint":    effErrHint,
					"is_cond_unmet":     previewMode == "highlight" && hasCond && !condMet,
					"is_locked_preview": previewMode == "output" && hasCond && !condMet && ch.ShowWhenLocked,
				})
			}
			vars["scene_text_modal_choices"] = modalChoices
		} else {
			s.SceneTextModalID = ""
		}
	}
}

func (s *Server) resolveInheritedSceneImageURL(target *book.Scene) string {
	if target == nil || s.ActiveBook == nil || len(s.ActiveBook.Scenes) == 0 {
		return ""
	}
	if target.ImageURL != "" {
		return target.ImageURL
	}
	startSc := s.findScene(s.ActiveBook.StartSceneID)
	if startSc == nil {
		startSc = s.ActiveBook.Scenes[0]
	}
	type bfsItem struct {
		sc  *book.Scene
		img string
	}
	visited := make(map[string]bool, len(s.ActiveBook.Scenes))
	queue := []bfsItem{{sc: startSc, img: startSc.ImageURL}}
	visited[startSc.ID] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		curImg := cur.img
		if cur.sc.ImageURL != "" {
			curImg = cur.sc.ImageURL
		}
		if cur.sc.ID == target.ID {
			if curImg != "" {
				return curImg
			}
			break
		}
		for _, ch := range cur.sc.Choices {
			if ch.IsEnding || ch.TargetSceneID == "" || strings.EqualFold(ch.TargetSceneID, "END") {
				continue
			}
			nextSc := s.findScene(ch.TargetSceneID)
			if nextSc != nil && !visited[nextSc.ID] {
				visited[nextSc.ID] = true
				nextImg := curImg
				if nextSc.ImageURL != "" {
					nextImg = nextSc.ImageURL
				}
				queue = append(queue, bfsItem{sc: nextSc, img: nextImg})
			}
		}
	}
	lastImg := ""
	for _, sc := range s.ActiveBook.Scenes {
		if sc == nil {
			continue
		}
		if sc.ImageURL != "" {
			lastImg = sc.ImageURL
		}
		if sc.ID == target.ID && lastImg != "" {
			return lastImg
		}
	}
	return lastImg
}
