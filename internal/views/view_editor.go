//go:build !(js && wasm)

package views

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/logging"
	"mojra/internal/state"
)

type studioHomeProjectItem struct {
	Book       *book.Book `tpl:"*"`
	MetaLine   string     `tpl:"meta_line"`
	PathLine   string     `tpl:"path_line"`
	IsUnpacked bool       `tpl:"is_unpacked"`
}

func populateEditorCommonVars(s *state.AppState, vars map[string]any, full bool) {
	b := s.Editor.ActiveBook
	hasEditorBook := s.Editor.HasActiveBook()
	vars["has_editor_book"] = hasEditorBook
	vars["book"] = b
	vars["default_new_book_dir"] = book.ResolveNewBookDir(s.Settings)
	vars["default_export_dir"] = b.ResolveExportDir(s.Settings)

	isScreenplay := false
	if hasEditorBook {
		isScreenplay = b.IsScreenplay()
		vars["script_standard"] = b.NormalizedStandard()
	} else {
		vars["script_standard"] = "pl"
	}
	vars["is_screenplay_mode"] = isScreenplay

	if isScreenplay {
		vars["editor_title_placeholder"] = tr(s, i18n.TrEditorSpTitlePlaceholder)
		vars["editor_author_placeholder"] = tr(s, i18n.TrEditorSpAuthorPlaceholder)
		vars["editor_description_placeholder"] = tr(s, i18n.TrEditorSpDescPlaceholder)
		vars["editor_chapter_title_placeholder"] = tr(s, i18n.TrEditorActTitlePlaceholder)
		vars["editor_settings_hint"] = tr(s, i18n.TrEditorSpSettingsHint)
	} else {
		vars["editor_title_placeholder"] = tr(s, i18n.TrEditorBookTitlePlaceholder)
		vars["editor_author_placeholder"] = tr(s, i18n.TrEditorBookAuthorPlaceholder)
		vars["editor_description_placeholder"] = tr(s, i18n.TrEditorBookDescPlaceholder)
		vars["editor_chapter_title_placeholder"] = tr(s, i18n.TrEditorChapterTitlePlaceholder)
		vars["editor_settings_hint"] = tr(s, i18n.TrEditorBookSettingsHint)
	}

	isHistPreview := !s.Config.StandaloneMode && s.Editor.History.IsPreview(b)
	vars[keyIsHistoryPreview] = isHistPreview
	if isHistPreview {
		vars[keyHistoryPreviewAttr] = attrTrue
	} else {
		vars[keyHistoryPreviewAttr] = attrFalse
	}
	if isHistPreview && s.Editor.History.Preview != nil {
		vars[keyHistoryPreviewID] = s.Editor.History.Preview.RevisionID
		vars[keyHistoryPreviewLabel] = s.Editor.History.Preview.RevisionLabel
	} else {
		vars[keyHistoryPreviewID] = ""
		vars[keyHistoryPreviewLabel] = ""
	}

	if hasEditorBook && len(b.Scenes) > 0 {
		warnItems := make([]map[string]any, 0, len(b.Warnings))
		for _, w := range b.Warnings {
			warnItems = append(warnItems, map[string]any{"text": w})
		}
		vars["has_book_warnings"] = len(b.Warnings) > 0
		vars["book_warning_count"] = len(b.Warnings)
		vars["has_multiple_book_warnings"] = len(b.Warnings) > 1
		if len(b.Warnings) > 0 {
			vars["first_book_warning"] = b.Warnings[0]
		} else {
			vars["first_book_warning"] = ""
		}
		vars["book_warnings"] = warnItems
		isNarrated := !b.IsScreenplay() && b.Narrated
		vars["book_narrated"] = isNarrated
		if isNarrated {
			vars["book_narrated_label"] = tr(s, i18n.TrBookNarratedReqLabel)
			vars["book_narrated_short_label"] = tr(s, i18n.TrRequiredLabel)
		} else {
			vars["book_narrated_label"] = tr(s, i18n.TrBookNarratedOffLabel)
			vars["book_narrated_short_label"] = tr(s, i18n.TrDisabledLabel)
		}
		vars["book_imageless"] = b.Imageless
		if b.Imageless {
			vars["book_imageless_label"] = tr(s, i18n.TrBookImagelessOnLabel)
			vars["book_imageless_short_label"] = tr(s, i18n.TrNoImagesLabel)
		} else {
			vars["book_imageless_label"] = tr(s, i18n.TrBookImagelessOffLabel)
			vars["book_imageless_short_label"] = tr(s, i18n.TrWithImagesLabel)
		}
	} else {
		vars["has_book_warnings"] = false
		vars["book_warning_count"] = 0
		vars["has_multiple_book_warnings"] = false
		vars["first_book_warning"] = ""
		vars["book_narrated"] = false
		vars["book_narrated_label"] = tr(s, i18n.TrBookNarratedOffLabel)
		vars["book_narrated_short_label"] = tr(s, i18n.TrDisabledLabel)
		vars["book_imageless"] = false
		vars["book_imageless_label"] = tr(s, i18n.TrBookImagelessOffLabel)
		vars["book_imageless_short_label"] = tr(s, i18n.TrWithImagesLabel)
		vars["has_selected_node"] = false
		vars["book_settings_open"] = false
		vars["has_selected_chapter"] = false
		vars["is_node_inspector_hidden"] = false
		vars["studio_search_count"] = 0
		vars["studio_search_items"] = []map[string]any(nil)
		populateEditorHistoryContext(s, vars)
		if full {
			populateStudioHomeContext(s, vars, s.Library.Books)
		}
	}
}

func ViewEditor(s *state.AppState) map[string]any {
	vars := make(map[string]any, 96)
	populateEditorCommonVars(s, vars, true)
	if s.Editor.HasActiveBook() && len(s.Editor.ActiveBook.Scenes) > 0 {
		populateEditorContext(s, vars)
	}
	return vars
}

func populateStudioHomeContext(s *state.AppState, vars map[string]any, books []*book.Book) {
	lang := LangOf(s)
	items := make([]studioHomeProjectItem, 0, len(books))
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
		items = append(items, studioHomeProjectItem{
			Book:     b,
			MetaLine: formatStudioProjectMetaLine(b, lang),
			PathLine: formatStudioProjectPathLine(b),
		})
	}

	var extPaths []string
	if s.Settings != nil {
		extPaths = s.Settings.ExternalBooks
	}
	tPacked := time.Now()
	packedBooks := book.LoadPackedBooks(s.Config.BooksDir, extPaths)
	logging.Active.MarkStep(logging.StageStudioHomePackedScan, tPacked, s.Config.BooksDir)
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

	packedItems := make([]studioHomeProjectItem, 0, len(packedBooks))
	for _, pb := range packedBooks {
		titleKey := book.SanitizeBookID(pb.Title)
		isUnpacked := unpackedByID[pb.ID] || (titleKey != "" && unpackedByTitle[titleKey])
		packedItems = append(packedItems, studioHomeProjectItem{
			Book:       pb,
			MetaLine:   formatStudioProjectMetaLine(pb, lang),
			PathLine:   formatStudioProjectPathLine(pb),
			IsUnpacked: isUnpacked,
		})
	}

	vars["has_studio_home_projects"] = len(items) > 0
	vars["studio_home_projects"] = items
	vars["has_studio_packed_books"] = len(packedItems) > 0
	vars["studio_packed_books"] = packedItems
}

func formatStudioProjectMetaLine(b *book.Book, lang i18n.Lang) string {
	var typePart, scenesPart, chPart, metricPart string
	if b.IsScreenplay() {
		std := strings.ToUpper(b.NormalizedStandard())
		if std == "PL" {
			std = "PISF"
		}
		if lang == i18n.LangEN {
			typePart = "SCREENPLAY (" + std + ")"
		} else {
			typePart = "SCENARIUSZ FILMOWY (" + std + ")"
		}
	} else {
		if lang == i18n.LangEN {
			typePart = "INTERACTIVE BOOK"
		} else {
			typePart = "KSIĄŻKA INTERAKTYWNA"
		}
	}

	scCount := len(b.Scenes)
	if lang == i18n.LangEN {
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
		if lang == i18n.LangEN {
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
		if lang == i18n.LangEN {
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
		if lang == i18n.LangEN {
			metricPart = ".PLAYBOOK (READ-ONLY)"
		} else {
			metricPart = ".PLAYBOOK (TYLKO ODCZYT)"
		}
	} else if b.IsScreenplay() {
		bd := b.ScreenplayBreakdown()
		metricPart = fmt.Sprintf("%s (1/8)", strings.ToUpper(bd.TotalPagesFormatted))
	} else if b.Narrated {
		if lang == i18n.LangEN {
			metricPart = "FLAC NARRATION"
		} else {
			metricPart = "NARRACJA FLAC"
		}
	} else if b.Imageless {
		if lang == i18n.LangEN {
			metricPart = "IMAGELESS"
		} else {
			metricPart = "BEZ OBRAZÓW"
		}
	} else {
		if lang == i18n.LangEN {
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

func populateEditorContext(s *state.AppState, vars map[string]any) {
	b := s.Editor.ActiveBook
	tSel := time.Now()
	populateEditorSelectedNodeOnly(s, vars)
	logging.Active.MarkStep(logging.StageEditorSelectedNode, tSel, b.ID)
	selected := b.FindScene(s.Editor.SelectedSceneID)
	if selected == nil {
		selected = b.Scenes[0]
	}

	orderedScenes := make([]*book.Scene, len(b.Scenes))
	copy(orderedScenes, b.Scenes)
	sort.SliceStable(orderedScenes, func(i, j int) bool {
		ri, rj := max(orderedScenes[i].Row, 1), max(orderedScenes[j].Row, 1)
		if ri != rj {
			return ri < rj
		}
		return orderedScenes[i].Col < orderedScenes[j].Col
	})

	edZoom := b.EditorZoom
	if edZoom < 0.2 || edZoom > 3.0 {
		edZoom = 1.0
	}
	tGraph := time.Now()
	vars["studio_graph_payload"] = buildStudioGraphPayload(b, edZoom, selected.ID, orderedScenes)
	logging.Active.MarkStep(logging.StageEditorGraphPayload, tGraph, b.ID)
	tSearch := time.Now()
	populateEditorSearchContext(s, vars, orderedScenes)
	logging.Active.MarkStep(logging.StageEditorSearchCtx, tSearch, b.ID)
	tBreakdown := time.Now()
	populateEditorBreakdownContext(s, vars)
	logging.Active.MarkStep(logging.StageEditorBreakdownCtx, tBreakdown, b.ID)
	populateEditorHistoryContext(s, vars)
}

func populateEditorSearchContext(s *state.AppState, vars map[string]any, orderedScenes []*book.Scene) {
	b := s.Editor.ActiveBook
	if b == nil || len(orderedScenes) == 0 {
		vars["studio_search_count"] = 0
		vars["studio_search_items"] = []map[string]any(nil)
		return
	}
	badgeScene := strings.TrimSuffix(tr(s, i18n.TrSpSceneUpperKey), ":")
	isSp := b.IsScreenplay()
	defCh := b.DefaultChapterName()

	items := make([]map[string]any, 0, len(orderedScenes))
	for _, sc := range orderedScenes {
		chName := strings.TrimSpace(sc.Chapter)
		if chName == "" {
			chName = defCh
		}
		dispTitle := sc.DisplayName()
		if isSp && strings.TrimSpace(sc.Slugline) != "" {
			dispTitle = strings.TrimSpace(sc.Slugline)
		}
		var choiceParts []string
		for _, ch := range sc.Choices {
			if ch.IsImplicit && len(sc.Choices) == 1 {
				continue
			}
			t := ch.Target
			if t == "" {
				t = ch.TargetSceneID
			}
			choiceParts = append(choiceParts, strings.TrimSpace(ch.Label)+" -> "+strings.TrimSpace(t))
		}
		flatText := strings.Join(strings.Fields(sc.Text), " ")
		defSnip := strings.Join(strings.Fields(sc.Synopsis), " ")
		if defSnip == "" {
			defSnip = firstSentenceSnippet(flatText)
		}
		runes := []rune(defSnip)
		if len(runes) > 84 {
			defSnip = string(runes[:84]) + "..."
		}
		items = append(items, map[string]any{
			"id":              sc.ID,
			"display_title":   dispTitle,
			"title":           sc.Name,
			"slugline":        sc.Slugline,
			"chapter":         chName,
			"chapter_upper":   strings.ToUpper(chName),
			"synopsis":        sc.Synopsis,
			"text_b64":        base64.StdEncoding.EncodeToString([]byte(sc.Text)),
			"choices":         strings.Join(choiceParts, " | "),
			"default_badge":   badgeScene,
			"default_snippet": defSnip,
			"is_selected":     sc.ID == s.Editor.SelectedSceneID,
		})
	}
	vars["studio_search_count"] = len(items)
	vars["studio_search_items"] = items
}

func firstSentenceSnippet(s string) string {
	s = strings.TrimSpace(s)
	for i, r := range s {
		if r == '.' || r == '!' || r == '?' {
			return strings.TrimSpace(s[:i+len(string(r))])
		}
	}
	return s
}

func sanitizeGraphPayloadField(v string) string {
	v = strings.ReplaceAll(v, "\r", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	v = strings.ReplaceAll(v, "|", "¦")
	return strings.TrimSpace(v)
}

func boolToPayloadBit(b bool) int {
	if b {
		return 1
	}
	return 0
}

func buildStudioGraphPayload(b *book.Book, edZoom float64, selectedSceneID string, orderedScenes []*book.Scene) string {
	if b == nil {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(orderedScenes)*160 + len(b.Chapters)*120 + 64)
	fmt.Fprintf(&sb, "viewport | pan_x:%d | pan_y:%d | zoom:%.2f | has_view:%d\n",
		b.EditorPanX, b.EditorPanY, edZoom, boolToPayloadBit(b.HasEditorView))

	for chIdx, chItem := range b.Chapters {
		chName := strings.TrimSpace(chItem.Name)
		if chName == "" {
			continue
		}
		var chScenes []*book.Scene
		for _, sc := range b.Scenes {
			scCh := strings.TrimSpace(sc.Chapter)
			if scCh == "" {
				scCh = b.DefaultChapterName()
			}
			if strings.EqualFold(scCh, chName) {
				chScenes = append(chScenes, sc)
			}
		}
		if len(chScenes) == 0 {
			continue
		}
		minX, maxX := chScenes[0].X, chScenes[0].X+book.NodeCardWidth
		minY, maxY := chScenes[0].Y, chScenes[0].Y+book.NodeMinLogicalHeight
		for _, sc := range chScenes[1:] {
			if sc.X < minX {
				minX = sc.X
			}
			if sc.X+book.NodeCardWidth > maxX {
				maxX = sc.X + book.NodeCardWidth
			}
			if sc.Y < minY {
				minY = sc.Y
			}
			if sc.Y+book.NodeMinLogicalHeight > maxY {
				maxY = sc.Y + book.NodeMinLogicalHeight
			}
		}
		chRef := chItem.ReferenceKey()
		isRoot := chIdx == 0
		if b.StartSceneID != "" {
			isRoot = false
			for _, sc := range chScenes {
				if strings.EqualFold(sc.ID, b.StartSceneID) {
					isRoot = true
					break
				}
			}
		}
		var outRefs []string
		seenTargets := make(map[string]bool)
		for _, sc := range chScenes {
			for _, ch := range sc.Choices {
				if ch.IsEnding || !ch.IsChapterLink || ch.TargetChapterID == "" {
					continue
				}
				tCh := b.FindChapter(ch.TargetChapterID)
				if tCh == nil {
					continue
				}
				tRef := tCh.ReferenceKey()
				tKey := strings.ToLower(tRef)
				if seenTargets[tKey] {
					continue
				}
				seenTargets[tKey] = true
				outRefs = append(outRefs, sanitizeGraphPayloadField(tRef))
			}
		}
		startSceneID := chItem.StartSceneID
		if startSceneID == "" && len(chScenes) > 0 {
			startSceneID = chScenes[0].ID
		}
		x := minX - book.ChapterFramePadX
		y := minY - book.ChapterFramePadTop
		w := (maxX - minX) + book.ChapterFramePadX2
		h := (maxY - minY) + book.ChapterFramePadYTotal
		fmt.Fprintf(&sb, "chapter | id:%s | ref:%s | name:%s | root:%d | start:%s | x:%d | y:%d | w:%d | h:%d | outs:%s\n",
			sanitizeGraphPayloadField(chItem.ID),
			sanitizeGraphPayloadField(chRef),
			sanitizeGraphPayloadField(chName),
			boolToPayloadBit(isRoot),
			sanitizeGraphPayloadField(startSceneID),
			x, y, w, h,
			strings.Join(outRefs, ","))
	}

	branchRoles, branchParents := book.ComputeBranchRoles(b)

	for _, sc := range orderedScenes {
		id := sc.ID
		name := sc.Name
		if name == "" {
			name = id
		}
		chapter := strings.TrimSpace(sc.Chapter)
		if chapter == "" {
			chapter = b.DefaultChapterName()
		}
		isSelected := sc.ID == selectedSceneID
		isStart := false
		if b.StartSceneID != "" {
			isStart = strings.EqualFold(sc.ID, b.StartSceneID)
		} else if len(b.Scenes) > 0 {
			isStart = sc.ID == b.Scenes[0].ID
		}
		isEntry := isStart
		if !isEntry {
			if chMeta := b.FindChapter(sc.Chapter); chMeta != nil && chMeta.StartSceneID != "" {
				isEntry = strings.EqualFold(sc.ID, chMeta.StartSceneID)
			}
		}
		hasChoices := len(sc.Choices) >= 2
		impTarget := ""
		impChapter := ""
		if !hasChoices && len(sc.Choices) == 1 && !sc.Choices[0].IsEnding {
			impTarget = sc.Choices[0].TargetSceneID
			if sc.Choices[0].IsChapterLink && sc.Choices[0].TargetChapterID != "" {
				impChapter = sc.Choices[0].TargetChapterID
				if tCh := b.FindChapter(impChapter); tCh != nil {
					impChapter = tCh.ReferenceKey()
				}
			}
		}
		branchRole := branchRoles[id]
		branchParent := branchParents[id]

		fmt.Fprintf(&sb, "node | id:%s | name:%s | chapter:%s | x:%d | y:%d | selected:%d | start:%d | entry:%d | unreachable:%d | has_choices:%d | imp_target:%s | imp_chapter:%s | branch_role:%s | branch_parent:%s\n",
			sanitizeGraphPayloadField(id),
			sanitizeGraphPayloadField(name),
			sanitizeGraphPayloadField(chapter),
			sc.X, sc.Y,
			boolToPayloadBit(isSelected),
			boolToPayloadBit(isStart),
			boolToPayloadBit(isEntry),
			boolToPayloadBit(sc.Unreachable),
			boolToPayloadBit(hasChoices),
			sanitizeGraphPayloadField(impTarget),
			sanitizeGraphPayloadField(impChapter),
			sanitizeGraphPayloadField(branchRole),
			sanitizeGraphPayloadField(branchParent))

		if hasChoices {
			inspectorChoices := sc.Choices
			if len(inspectorChoices) == 0 || sc.ExplicitEnd {
				inspectorChoices = []book.Choice{
					{
						Label:      "Dalej",
						Target:     "END",
						IsEnding:   true,
						IsImplicit: true,
					},
				}
			}
			for chIdx, ch := range inspectorChoices {
				lbl := ch.Label
				targetChapterRef := ""
				if !ch.IsEnding && ch.IsChapterLink && ch.TargetChapterID != "" {
					targetChapterRef = ch.TargetChapterID
					if tCh := b.FindChapter(ch.TargetChapterID); tCh != nil {
						targetChapterRef = tCh.ReferenceKey()
					}
				}
				hasCond := book.CleanConditionExpr(ch.Condition) != ""
				fmt.Fprintf(&sb, "port | scene:%s | idx:%d | label:%s | target_id:%s | target_chapter:%s | cond:%d\n",
					sanitizeGraphPayloadField(id),
					chIdx,
					sanitizeGraphPayloadField(lbl),
					sanitizeGraphPayloadField(ch.TargetSceneID),
					sanitizeGraphPayloadField(targetChapterRef),
					boolToPayloadBit(hasCond))
			}
		}
	}

	if len(b.WirePins) > 0 {
		wireKeys := make([]string, 0, len(b.WirePins))
		for k, pins := range b.WirePins {
			if k != "" && len(pins) > 0 {
				wireKeys = append(wireKeys, k)
			}
		}
		sort.Strings(wireKeys)
		for _, k := range wireKeys {
			pins := b.WirePins[k]
			var parts []string
			for _, p := range pins {
				parts = append(parts, fmt.Sprintf("%d,%d", p.X, p.Y))
			}
			fmt.Fprintf(&sb, "wire_pins | key:%s | pins:%s\n",
				sanitizeGraphPayloadField(k),
				strings.Join(parts, ";"))
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}
