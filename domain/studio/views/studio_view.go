//go:build !(js && wasm)

package views

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	book "mojra/domain/book/actions"
	commonviews "mojra/domain/common/views"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/engine"
	"mojra/engine/templates"
)

const (
	endTargetID  = "END"
	zeroDuration = "0:00"
	versionFmt   = "%s?v=%d"
	metaSep      = " • "
)

var (
	buildBookModeOptions       = commonviews.BuildBookModeOptions
	buildScriptStandardOptions = commonviews.BuildScriptStandardOptions
)

func LangOf(s *state.StudioState) engine.Lang {
	if s == nil {
		return engine.LangPL
	}
	return commonviews.LangOf(s.UIState)
}

func tr(s *state.StudioState, key engine.TrKey) string {
	if s == nil {
		return engine.T(engine.LangPL, key)
	}
	return commonviews.Tr(s.UIState, key)
}

func trf(s *state.StudioState, key engine.TrKey, args ...any) string {
	if s == nil {
		return engine.Tf(engine.LangPL, key, args...)
	}
	return commonviews.Trf(s.UIState, key, args...)
}

const (
	stdCodePLUpper            = "PL"
	stdLabelPISF              = "PISF"
	studioScreenplayMetricFmt = "%s (1/8)"
	studioMetaSep             = "  ·  "
	booksDirPrefix            = "books/"
	booksDirName              = "books"
	projectsDirName           = "projects"
	bookPlaytxtSuffix         = "/book.playtxt"
	choiceArrowSep            = " -> "
	choicePipeSep             = " | "
	snippetEllipsis           = "..."
)

type studioTemplate struct {
	NodeCanvas               templates.TemplateChunk
	ContextPanel             templates.TemplateChunk
	VariablesPanel           templates.TemplateChunk
	BreakdownPanel           templates.TemplateChunk
	HistoryPanel             templates.TemplateChunk
	HelpPanel                templates.TemplateChunk
	HasEditorBook            bool
	IsScreenplayMode         bool
	BookSettingsOpen         bool
	EditorSettingsHint       string
	SelectedSceneID          string
	SelectedSceneTitle       string
	ShowEditorVariablesPanel bool
	ShowEditorBreakdownPanel bool
	ShowEditorHistoryPanel   bool
	ShowEditorHelpPanel      bool
	IsHistoryPreview         bool
	HistoryPreviewAttr       string
	HistoryPreviewID         string
	HistoryPreviewLabel      string
	StudioSearchOpen         bool
	StudioSearchQuery        string
	StudioSearchCount        int
	StudioSearchItems        []book.StudioSearchItem
	BookWarningsOpen         bool
	HasBookWarnings          bool
	BookWarningCount         int
	HasMultipleBookWarnings  bool
	FirstBookWarning         string
	BookWarnings             []string
	HasStudioHomeProjects    bool
	StudioHomeProjects       []book.Book
	HasStudioPackedBooks     bool
	StudioPackedBooks        []book.Book
}

type studioScript struct {
	isHistoryPreview bool
	selectedSceneId  string
	studioSearchOpen bool
	sceneId          string
	scenePrevId      string
	sceneExplicitId  string
	sceneRef         string
	sceneTitle       string
	sceneChapter     string
	startSceneId     string
	voicePeak        string
	historyCount     int
	canUndo          bool
	undoTitle        string
	canRedo          bool
	redoTitle        string
}

func orderStudioScenes(b *book.Book) []*book.Scene {
	if b == nil || len(b.Scenes) == 0 {
		return nil
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
	return orderedScenes
}

func buildStudioScript(s *state.StudioState) *studioScript {
	b := s.Editor.ActiveBook
	histPreview := !s.Config.StandaloneMode && s.Editor.History.IsPreview(b)
	sc := &studioScript{
		isHistoryPreview: histPreview,
		selectedSceneId:  s.Editor.SelectedSceneID,
		studioSearchOpen: s.Editor.SearchOpen,
		voicePeak:        s.Overlays.VoicePeak,
	}
	if b != nil {
		if s.Editor.SelectedSceneID != "" {
			sc.sceneId = s.Editor.SelectedSceneID
			if s.Editor.LastRenamedFromSceneID != "" && !strings.EqualFold(s.Editor.LastRenamedFromSceneID, s.Editor.SelectedSceneID) {
				sc.scenePrevId = s.Editor.LastRenamedFromSceneID
			}
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
			sc.undoTitle = trf(s, i18n.TrUndoTitleFmt, h.Revisions[curIdx].Label)
		}
	} else if histPreview {
		sc.undoTitle = tr(s, i18n.TrHistoryPreviewLocked)
	} else {
		sc.undoTitle = tr(s, i18n.TrUndoEmptyTitle)
	}
	if sc.canRedo {
		curIdx := h.CursorIndex()
		if curIdx >= 0 && curIdx+1 < len(h.Revisions) {
			sc.redoTitle = trf(s, i18n.TrRedoTitleFmt, h.Revisions[curIdx+1].Label)
		}
	} else if histPreview {
		sc.redoTitle = tr(s, i18n.TrHistoryPreviewLocked)
	} else {
		sc.redoTitle = tr(s, i18n.TrRedoEmptyTitle)
	}
	return sc
}

// StudioScriptView returns a script-only TemplateChunk hydrating StudioController.data on partial updates.
func StudioScriptView(s *state.StudioState) templates.TemplateChunk {
	return templates.TemplateChunk{
		ScriptData: buildStudioScript(s),
	}
}

func StudioView(s *state.StudioState) templates.TemplateChunk {
	b := s.Editor.ActiveBook
	hasEditorBook := s.Editor.HasActiveBook()
	isSp := hasEditorBook && b.IsScreenplay()
	settingsHint := tr(s, i18n.TrEditorBookSettingsHint)
	if isSp {
		settingsHint = tr(s, i18n.TrEditorSpSettingsHint)
	}

	histChunk := EditorHistoryView(s)
	ctxChunk := EditorContextView(s)
	varsChunk := EditorVariablesView(s)
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
		ContextPanel:             ctxChunk,
		VariablesPanel:           varsChunk,
		HistoryPanel:             histChunk,
		HelpPanel:                EditorHelpView(s),
		HasEditorBook:            hasEditorBook,
		IsScreenplayMode:         isSp,
		BookSettingsOpen:         s.Editor.RightPanel == state.PanelBook,
		EditorSettingsHint:       settingsHint,
		ShowEditorVariablesPanel: s.Editor.RightPanel == state.PanelVariables,
		ShowEditorBreakdownPanel: s.Editor.RightPanel == state.PanelBreakdown && isSp,
		ShowEditorHistoryPanel:   s.Editor.RightPanel == state.PanelHistory,
		ShowEditorHelpPanel:      s.Editor.RightPanel == state.PanelHelp,
		IsHistoryPreview:         isHistPreview,
		HistoryPreviewAttr:       histPreviewAttr,
		HistoryPreviewID:         histPreviewID,
		HistoryPreviewLabel:      histPreviewLabel,
	}

	if hasEditorBook && len(b.Scenes) > 0 {
		selected := b.FindScene(b.ResolveValidSceneID(s.Editor.SelectedSceneID))
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
		var allBooks []*book.Book
		if s.Hooks.ListAllBooks != nil {
			allBooks = s.Hooks.ListAllBooks()
		}
		populateStudioHomeContext(s, v, allBooks)
	}
	sc := buildStudioScript(s)
	return templates.TemplateChunk{
		TemplateData: v,
		ScriptData:   sc,
	}
}

func populateStudioHomeContext(s *state.StudioState, v *studioTemplate, books []*book.Book) {
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
	engine.Active.MarkStep(engine.StageStudioHomePackedScan, tPacked, s.Config.BooksDir)
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

func formatStudioProjectMetaLine(b *book.Book, lang engine.Lang) string {
	var typePart, chPart, metricPart string
	if b.IsScreenplay() {
		std := strings.ToUpper(b.NormalizedStandard())
		if std == stdCodePLUpper {
			std = stdLabelPISF
		}
		typePart = engine.Tf(lang, i18n.TrStudioMetaScreenplayFmt, std)
	} else {
		typePart = engine.T(lang, i18n.TrStudioMetaInteractiveBook)
	}

	scenesPart := engine.Plural(lang, len(b.Scenes), i18n.TrStudioScenesOne, i18n.TrStudioScenesFewFmt, i18n.TrStudioScenesManyFmt)

	chCount := len(b.Chapters)
	if chCount < 1 {
		chCount = 1
	}
	if b.IsScreenplay() {
		chPart = engine.Plural(lang, chCount, i18n.TrStudioActsOne, i18n.TrStudioActsFewFmt, i18n.TrStudioActsManyFmt)
	} else {
		chPart = engine.Plural(lang, chCount, i18n.TrStudioChaptersOne, i18n.TrStudioChaptersFewFmt, i18n.TrStudioChaptersManyFmt)
	}

	if b.IsPacked {
		metricPart = engine.T(lang, i18n.TrPlaybookReadonlyBadge)
	} else if b.IsScreenplay() {
		bd := b.ScreenplayBreakdown()
		metricPart = fmt.Sprintf(studioScreenplayMetricFmt, strings.ToUpper(bd.TotalPagesFormatted))
	} else if b.Narrated {
		metricPart = engine.T(lang, i18n.TrStudioMetaNarrated)
	} else if b.Imageless {
		metricPart = engine.T(lang, i18n.TrStudioMetaImageless)
	} else {
		metricPart = engine.T(lang, i18n.TrStudioMetaIllustrated)
	}

	return strings.Join([]string{typePart, scenesPart, chPart, metricPart}, studioMetaSep)
}

func formatStudioProjectPathLine(b *book.Book) string {
	if b.IsPacked {
		return booksDirPrefix + filepath.Base(b.RootPath)
	}
	parent := filepath.Base(filepath.Dir(filepath.Clean(b.RootPath)))
	if parent != booksDirName && parent != projectsDirName {
		parent = projectsDirName
	}
	return parent + "/" + b.ID + bookPlaytxtSuffix
}

func populateEditorContext(s *state.StudioState, v *studioTemplate) {
	b := s.Editor.ActiveBook
	tGraph := time.Now()
	orderedScenes := orderStudioScenes(b)
	v.NodeCanvas = NodeCanvasView(s, orderedScenes)
	engine.Active.MarkStep(engine.StageEditorGraphPayload, tGraph, b.ID)
	tSearch := time.Now()
	populateEditorSearchContext(s, v, orderedScenes)
	engine.Active.MarkStep(engine.StageEditorSearchCtx, tSearch, b.ID)
	tBreakdown := time.Now()
	v.BreakdownPanel = EditorBreakdownView(s)
	engine.Active.MarkStep(engine.StageEditorBreakdownCtx, tBreakdown, b.ID)
}

func populateEditorSearchContext(s *state.StudioState, v *studioTemplate, orderedScenes []*book.Scene) {
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
			choiceParts = append(choiceParts, strings.TrimSpace(ch.Label)+choiceArrowSep+strings.TrimSpace(t))
		}
		choicesStr := strings.Join(choiceParts, choicePipeSep)
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
			defSnip = string(runes[:84]) + snippetEllipsis
		}
		items = append(items, book.StudioSearchItem{
			ID:             sc.ID,
			DisplayTitle:   dispTitle,
			ChapterUpper:   strings.ToUpper(chName),
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
