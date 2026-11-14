//go:build !(js && wasm)

package views

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/logging"
	"mojra/internal/state"
	"mojra/internal/tpl"
)

const (
	undoTitlePrefix    = "Cofnij: "
	undoShortcutSuffix = " (Ctrl+Z)"
	undoEmptyTitle     = "Brak zmian do cofnięcia od ostatniej wersji (Ctrl+Z)"
	redoTitlePrefix    = "Ponów: "
	redoShortcutSuffix = " (Ctrl+Shift+Z)"
	redoEmptyTitle     = "Brak zmian do ponowienia (Ctrl+Shift+Z)"
)

type studioTemplate struct {
	ContextPanel                  tpl.TemplateChunk
	VariablesPanel                tpl.TemplateChunk
	BreakdownPanel                tpl.TemplateChunk
	HistoryPanel                  tpl.TemplateChunk
	HasEditorBook                 bool
	IsScreenplayMode              bool
	BookSettingsOpen              bool
	EditorSettingsHint            string
	EditorChapterTitlePlaceholder string
	SelectedSceneID               string
	SelectedSceneTitle            string
	ShowEditorVariablesPanel      bool
	ShowEditorBreakdownPanel      bool
	ShowEditorHistoryPanel        bool
	ShowEditorHelpPanel           bool
	IsHistoryPreview              bool
	HistoryPreviewAttr            string
	HistoryPreviewID              string
	HistoryPreviewLabel           string
	StudioGraphPayload            string
	StudioSearchOpen              bool
	StudioSearchQuery             string
	StudioSearchCount             int
	StudioSearchItems             []book.StudioSearchItem
	BookWarningsOpen              bool
	HasBookWarnings               bool
	BookWarningCount              int
	HasMultipleBookWarnings       bool
	FirstBookWarning              string
	BookWarnings                  []string
	HasStudioHomeProjects         bool
	StudioHomeProjects            []book.Book
	HasStudioPackedBooks          bool
	StudioPackedBooks             []book.Book
}

type studioScript struct {
	isHistoryPreview bool
	historyPreview   bool
	selectedSceneId  string
	studioSearchOpen bool
	sceneId          string
	scenePrevId      string
	sceneExplicitId  string
	sceneRef         string
	sceneTitle       string
	sceneChapter     string
	startSceneId     string
	historyCount     int
	canUndo          bool
	undoTitle        string
	canRedo          bool
	redoTitle        string
}

func buildStudioScript(s *state.AppState) *studioScript {
	b := s.Editor.ActiveBook
	histPreview := !s.Config.StandaloneMode && s.Editor.History.IsPreview(b)
	sc := &studioScript{
		isHistoryPreview: histPreview,
		historyPreview:   histPreview,
		selectedSceneId:  s.Editor.SelectedSceneID,
		studioSearchOpen: s.Editor.SearchOpen,
	}
	if b != nil {
		if s.Editor.SelectedSceneID != "" {
			sc.sceneId = s.Editor.SelectedSceneID
			if s.Editor.LastRenamedFromSceneID != "" && !strings.EqualFold(s.Editor.LastRenamedFromSceneID, s.Editor.SelectedSceneID) {
				sc.scenePrevId = s.Editor.LastRenamedFromSceneID
			}
			s.Editor.LastRenamedFromSceneID = ""
			if sel := b.FindScene(s.Editor.SelectedSceneID); sel != nil {
				sc.sceneExplicitId = sel.ExplicitID
				sc.sceneRef = url.QueryEscape(sel.ReferenceKey())
				sc.sceneTitle = url.QueryEscape(sel.DisplayName())
				sc.sceneChapter = url.QueryEscape(sel.Chapter)
			}
		}
		if b.StartSceneID != "" {
			sc.startSceneId = b.StartSceneID
		}
	}
	if s.Config.StandaloneMode {
		return sc
	}
	h := s.Editor.History.RefreshFromDisk(b)
	if h == nil {
		return sc
	}
	sc.historyCount = h.UnnamedEditsCount()
	sc.canUndo = !histPreview && h.CanUndo()
	sc.canRedo = !histPreview && h.CanRedo()
	if sc.canUndo {
		curIdx := h.CursorIndex()
		if curIdx >= 0 && curIdx < len(h.Revisions) {
			sc.undoTitle = undoTitlePrefix + h.Revisions[curIdx].Label + undoShortcutSuffix
		}
	} else if histPreview {
		sc.undoTitle = tr(s, i18n.TrHistoryPreviewLocked)
	} else {
		sc.undoTitle = undoEmptyTitle
	}
	if sc.canRedo {
		curIdx := h.CursorIndex()
		if curIdx >= 0 && curIdx+1 < len(h.Revisions) {
			sc.redoTitle = redoTitlePrefix + h.Revisions[curIdx+1].Label + redoShortcutSuffix
		}
	} else if histPreview {
		sc.redoTitle = tr(s, i18n.TrHistoryPreviewLocked)
	} else {
		sc.redoTitle = redoEmptyTitle
	}
	return sc
}

// ViewStudioScript returns a script-only TemplateChunk hydrating Studio/history controller state on partial updates.
func ViewStudioScript(s *state.AppState) tpl.TemplateChunk {
	return tpl.TemplateChunk{
		ScriptData: buildStudioScript(s),
	}
}

func ViewStudio(s *state.AppState) tpl.TemplateChunk {
	b := s.Editor.ActiveBook
	hasEditorBook := s.Editor.HasActiveBook()
	isSp := hasEditorBook && b.IsScreenplay()
	settingsHint := tr(s, i18n.TrEditorBookSettingsHint)
	chTitlePlaceholder := tr(s, i18n.TrEditorChapterTitlePlaceholder)
	if isSp {
		settingsHint = tr(s, i18n.TrEditorSpSettingsHint)
		chTitlePlaceholder = tr(s, i18n.TrEditorActTitlePlaceholder)
	}

	histChunk := ViewEditorHistory(s)
	ctxChunk := ViewEditorContextPanel(s)
	varsChunk := ViewEditorVariables(s)

	isHistPreview := !s.Config.StandaloneMode && s.Editor.History.IsPreview(b)
	histPreviewAttr := attrFalse
	var histPreviewID, histPreviewLabel string
	if isHistPreview {
		histPreviewAttr = attrTrue
		if s.Editor.History.Preview != nil {
			histPreviewID = s.Editor.History.Preview.RevisionID
			histPreviewLabel = s.Editor.History.Preview.RevisionLabel
		}
	}

	v := &studioTemplate{
		ContextPanel:                  ctxChunk,
		VariablesPanel:                varsChunk,
		HistoryPanel:                  histChunk,
		HasEditorBook:                 hasEditorBook,
		IsScreenplayMode:              isSp,
		BookSettingsOpen:              s.Editor.RightPanel == state.PanelBook,
		EditorSettingsHint:            settingsHint,
		EditorChapterTitlePlaceholder: chTitlePlaceholder,
		ShowEditorVariablesPanel:      s.Editor.RightPanel == state.PanelVariables,
		ShowEditorBreakdownPanel:      s.Editor.RightPanel == state.PanelBreakdown && isSp,
		ShowEditorHistoryPanel:        s.Editor.RightPanel == state.PanelHistory,
		ShowEditorHelpPanel:           s.Editor.RightPanel == state.PanelHelp,
		IsHistoryPreview:              isHistPreview,
		HistoryPreviewAttr:            histPreviewAttr,
		HistoryPreviewID:              histPreviewID,
		HistoryPreviewLabel:           histPreviewLabel,
	}

	if hasEditorBook && len(b.Scenes) > 0 {
		selected := b.FindScene(s.Editor.SelectedSceneID)
		if selected == nil {
			selected = b.Scenes[0]
		}
		v.SelectedSceneID = selected.ID
		v.SelectedSceneTitle = selected.DisplayName()
		v.StudioSearchOpen = s.Editor.SearchOpen
		v.StudioSearchQuery = s.Editor.SearchQuery
		v.BookWarningsOpen = s.Editor.WarningsOpen
		v.HasBookWarnings = len(b.Warnings) > 0
		v.BookWarningCount = len(b.Warnings)
		v.HasMultipleBookWarnings = len(b.Warnings) > 1
		if len(b.Warnings) > 0 {
			v.FirstBookWarning = b.Warnings[0]
			v.BookWarnings = append([]string(nil), b.Warnings...)
		}
		populateEditorContext(s, v)
	} else {
		populateStudioHomeContext(s, v, s.Library.Books)
	}
	return tpl.TemplateChunk{
		TemplateData: v,
		ScriptData:   buildStudioScript(s),
	}
}

func populateStudioHomeContext(s *state.AppState, v *studioTemplate, books []*book.Book) {
	lang := LangOf(s)
	items := make([]book.Book, 0, len(books))
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
		item := *b
		item.MetaLine = formatStudioProjectMetaLine(b, lang)
		item.PathLine = formatStudioProjectPathLine(b)
		items = append(items, item)
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

	packedItems := make([]book.Book, 0, len(packedBooks))
	for _, pb := range packedBooks {
		titleKey := book.SanitizeBookID(pb.Title)
		isUnpacked := unpackedByID[pb.ID] || (titleKey != "" && unpackedByTitle[titleKey])
		item := *pb
		item.MetaLine = formatStudioProjectMetaLine(pb, lang)
		item.PathLine = formatStudioProjectPathLine(pb)
		item.IsUnpacked = isUnpacked
		packedItems = append(packedItems, item)
	}

	v.HasStudioHomeProjects = len(items) > 0
	v.StudioHomeProjects = items
	v.HasStudioPackedBooks = len(packedItems) > 0
	v.StudioPackedBooks = packedItems
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

func populateEditorContext(s *state.AppState, v *studioTemplate) {
	b := s.Editor.ActiveBook
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
	v.StudioGraphPayload = buildStudioGraphPayload(b, edZoom, selected.ID, orderedScenes)
	logging.Active.MarkStep(logging.StageEditorGraphPayload, tGraph, b.ID)
	tSearch := time.Now()
	populateEditorSearchContext(s, v, orderedScenes)
	logging.Active.MarkStep(logging.StageEditorSearchCtx, tSearch, b.ID)
	tBreakdown := time.Now()
	v.BreakdownPanel = ViewEditorBreakdown(s)
	logging.Active.MarkStep(logging.StageEditorBreakdownCtx, tBreakdown, b.ID)
}

func populateEditorSearchContext(s *state.AppState, v *studioTemplate, orderedScenes []*book.Scene) {
	b := s.Editor.ActiveBook
	if b == nil || len(orderedScenes) == 0 {
		return
	}

	badgeScene := strings.TrimSuffix(tr(s, i18n.TrSpSceneUpperKey), ":")
	isSp := b.IsScreenplay()
	defCh := b.DefaultChapterName()
	queryLower := strings.ToLower(strings.TrimSpace(s.Editor.SearchQuery))

	items := make([]book.StudioSearchItem, 0, len(orderedScenes))
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
		choicesStr := strings.Join(choiceParts, " | ")
		flatText := strings.Join(strings.Fields(sc.Text), " ")
		if queryLower != "" && !matchesEditorSearchScene(sc, dispTitle, chName, choicesStr, flatText, queryLower) {
			continue
		}
		defSnip := strings.Join(strings.Fields(sc.Synopsis), " ")
		if defSnip == "" {
			defSnip = firstSentenceSnippet(flatText)
		}
		runes := []rune(defSnip)
		if len(runes) > 84 {
			defSnip = string(runes[:84]) + "..."
		}
		items = append(items, book.StudioSearchItem{
			ID:             sc.ID,
			DisplayTitle:   dispTitle,
			Title:          sc.Name,
			Slugline:       sc.Slugline,
			Chapter:        chName,
			ChapterUpper:   strings.ToUpper(chName),
			Synopsis:       sc.Synopsis,
			TextB64:        base64.StdEncoding.EncodeToString([]byte(sc.Text)),
			Choices:        choicesStr,
			DefaultBadge:   badgeScene,
			DefaultSnippet: defSnip,
			IsSelected:     sc.ID == s.Editor.SelectedSceneID,
		})
	}
	v.StudioSearchCount = len(items)
	v.StudioSearchItems = items
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

func matchesEditorSearchScene(sc *book.Scene, dispTitle, chName, choicesStr, flatText, queryLower string) bool {
	haystack := strings.ToLower(sc.ID + " " + dispTitle + " " + sc.Name + " " + sc.Slugline + " " + chName + " " + sc.Synopsis + " " + choicesStr + " " + flatText)
	return strings.Contains(haystack, queryLower)
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
