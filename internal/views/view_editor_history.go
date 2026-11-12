//go:build !(js && wasm)

package views

import (
	"fmt"
	"strings"
	"time"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/logging"
	"mojra/internal/state"
)

const (
	keyIsHistoryPreview       = "is_history_preview"
	keyHistoryPreviewAttr     = "history_preview_attr"
	keyHistoryPreviewID       = "history_preview_id"
	keyHistoryPreviewLabel    = "history_preview_label"
	keyShowEditorHistoryPanel = "show_editor_history_panel"
	keyActiveBranchName       = "active_branch_name"
	keyHistoryGraphRows       = "history_graph_rows"
	keyDefaultNextVersionName = "default_next_version_name"
	keyDefaultNextBranchName  = "default_next_branch_name"

	branchMain       = "main"
	branchBookAlias  = "book"
	revInitialID     = "r1"
	attrTrue         = "true"
	attrFalse        = "false"
	historyTimeToday = "15:04"
	historyTimeDate  = "01-02 15:04"
	historyMetaSep   = " • "
	historyMetaFmt   = "%s • %s"
)

type historyGraphLaneView struct {
	Branch           string `tpl:"branch"`
	HoverHint        string `tpl:"hover_hint"`
	IsBranchRail     bool   `tpl:"is_branch_rail"`
	IsOtherBranch    bool   `tpl:"is_other_branch"`
	HasRailUp        bool   `tpl:"has_rail_up"`
	HasRailDown      bool   `tpl:"has_rail_down"`
	IsActiveRailUp   bool   `tpl:"is_active_rail_up"`
	IsActiveRailDown bool   `tpl:"is_active_rail_down"`
	IsDashedRailUp   bool   `tpl:"is_dashed_rail_up"`
	IsDashedRailDown bool   `tpl:"is_dashed_rail_down"`
	HasNode          bool   `tpl:"has_node"`
	IsForkCorner     bool   `tpl:"is_fork_corner"`
	HasForkHoriz     bool   `tpl:"has_fork_horiz"`
	IsActiveFork     bool   `tpl:"is_active_fork"`
	IsCompactIn      bool   `tpl:"is_compact_in"`
	IsCompactOut     bool   `tpl:"is_compact_out"`
	HasCompactHoriz  bool   `tpl:"has_compact_horiz"`
	IsActiveCompact  bool   `tpl:"is_active_compact"`
	HasStemStart     bool   `tpl:"has_stem_start"`
	HasStemThrough   bool   `tpl:"has_stem_through"`
}

type historyGraphRowView struct {
	ID              string                 `tpl:"id"`
	Label           string                 `tpl:"label"`
	MetaLabel       string                 `tpl:"meta_label"`
	Branch          string                 `tpl:"branch"`
	HoverHint       string                 `tpl:"hover_hint"`
	IsUnnamedStep   bool                   `tpl:"is_unnamed_step"`
	IsCurrent       bool                   `tpl:"is_current"`
	IsActiveHead    bool                   `tpl:"is_active_head"`
	IsPreview       bool                   `tpl:"is_preview"`
	IsUndone        bool                   `tpl:"is_undone"`
	IsActiveLineage bool                   `tpl:"is_active_lineage"`
	IsActiveBranch  bool                   `tpl:"is_active_branch"`
	IsBranchHead    bool                   `tpl:"is_branch_head"`
	Lanes           []historyGraphLaneView `tpl:"lanes"`
}

func formatHistoryCompactTime(ts int64) string {
	if ts <= 0 {
		return "—"
	}
	t := time.Unix(ts, 0)
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format(historyTimeToday)
	}
	return t.Format(historyTimeDate)
}

func normalizeGraphBranchName(raw string) string {
	br := book.SanitizeBookID(strings.TrimSpace(raw))
	if br == "" || br == branchBookAlias {
		return branchMain
	}
	return br
}

func buildHistoryGraphRows(
	b *book.Book,
	h *book.BookHistory,
	curIdx, viewedIdx int,
	isHistPreview bool,
	lang i18n.Lang,
) ([]historyGraphRowView, map[string]string) {
	revHints, branchHints := book.BuildHistoryHoverSummaries(b, h)
	if h == nil || len(h.Revisions) == 0 {
		nowStr := formatHistoryCompactTime(time.Now().Unix())
		scenesCount := 0
		if b != nil {
			scenesCount = len(b.Scenes)
		}
		defHint := i18n.Tf(lang, i18n.TrHistoryMainScenesFmt, scenesCount)
		initLabel := i18n.T(lang, i18n.TrHistoryInitialState)
		branchHints[branchMain] = defHint
		row := historyGraphRowView{
			ID:              revInitialID,
			Label:           initLabel,
			MetaLabel:       revInitialID + historyMetaSep + nowStr,
			Branch:          branchMain,
			HoverHint:       defHint,
			IsUnnamedStep:   false,
			IsCurrent:       true,
			IsActiveHead:    true,
			IsPreview:       false,
			IsUndone:        false,
			IsActiveLineage: true,
			IsActiveBranch:  true,
			IsBranchHead:    true,
			Lanes: []historyGraphLaneView{
				{
					Branch:           branchMain,
					HoverHint:        defHint,
					IsBranchRail:     true,
					IsOtherBranch:    false,
					HasRailUp:        false,
					HasRailDown:      false,
					IsActiveRailUp:   false,
					IsActiveRailDown: false,
					IsDashedRailUp:   false,
					IsDashedRailDown: false,
					HasNode:          true,
					IsForkCorner:     false,
					HasForkHoriz:     false,
					IsActiveFork:     false,
					IsCompactIn:      false,
					IsCompactOut:     false,
					HasCompactHoriz:  false,
					IsActiveCompact:  false,
					HasStemStart:     true,
					HasStemThrough:   false,
				},
			},
		}
		return []historyGraphRowView{row}, branchHints
	}

	activeBranchName := h.NormalizedActiveBranch()

	// 1. Canonical branch list in fixed creation order from h.Branches
	var branches []book.HistoryBranch
	branchIdxByName := make(map[string]int)
	addBranch := func(br book.HistoryBranch) {
		name := normalizeGraphBranchName(br.Name)
		if _, exists := branchIdxByName[name]; exists {
			return
		}
		br.Name = name
		branchIdxByName[name] = len(branches)
		branches = append(branches, br)
	}
	for _, br := range h.Branches {
		addBranch(br)
	}
	if _, hasMain := branchIdxByName[branchMain]; !hasMain {
		addBranch(book.HistoryBranch{Name: branchMain, HeadRevID: h.Revisions[0].ID, BaseRevID: h.Revisions[0].ID})
	}
	for _, rev := range h.Revisions {
		brName := normalizeGraphBranchName(rev.Branch)
		if _, exists := branchIdxByName[brName]; !exists {
			addBranch(book.HistoryBranch{Name: brName, HeadRevID: rev.ID, BaseRevID: rev.ID})
		}
	}

	// Map revisions by ID
	revByID := make(map[string]book.HistoryRevision, len(h.Revisions))
	revPosByID := make(map[string]int, len(h.Revisions))
	for i, rev := range h.Revisions {
		revByID[rev.ID] = rev
		revPosByID[rev.ID] = i
	}

	// Identify branch head revisions and floor Named Version on active branch
	branchHeadRevID := make(map[string]string, len(branches))
	for _, br := range branches {
		if br.HeadRevID != "" {
			branchHeadRevID[br.Name] = br.HeadRevID
		}
	}
	for _, rev := range h.Revisions {
		brName := normalizeGraphBranchName(rev.Branch)
		if branchHeadRevID[brName] == "" || rev.Named {
			branchHeadRevID[brName] = rev.ID
		}
	}
	for _, br := range branches {
		if br.HeadRevID != "" {
			if _, ok := revByID[br.HeadRevID]; ok {
				branchHeadRevID[br.Name] = br.HeadRevID
			}
		}
	}

	branchIdxs := h.ActiveBranchRevisions()
	curBranchPos := len(branchIdxs) - 1
	floorBranchPos := 0
	for j, idx := range branchIdxs {
		if idx == curIdx {
			curBranchPos = j
		}
		if h.Revisions[idx].Named {
			floorBranchPos = j
		}
	}

	// 2. Build unified row list:
	//    Top: unnamed steps on active branch above floorBranchPos (newest to oldest)
	//    Main: named versions (plus any branch head/base) across all branches (newest to oldest)
	type graphRowEntry struct {
		revIdx        int
		rev           book.HistoryRevision
		branch        string
		isUnnamedStep bool
		isUndone      bool
	}
	var entries []graphRowEntry
	includedRevIdx := make(map[int]bool)

	for j := len(branchIdxs) - 1; j > floorBranchPos; j-- {
		idx := branchIdxs[j]
		rev := h.Revisions[idx]
		if !rev.Named {
			entries = append(entries, graphRowEntry{
				revIdx:        idx,
				rev:           rev,
				branch:        activeBranchName,
				isUnnamedStep: true,
				isUndone:      j > curBranchPos,
			})
			includedRevIdx[idx] = true
		}
	}

	mustIncludeRevID := make(map[string]bool)
	for _, br := range branches {
		if br.HeadRevID != "" {
			mustIncludeRevID[br.HeadRevID] = true
		}
		if br.BaseRevID != "" {
			mustIncludeRevID[br.BaseRevID] = true
		}
	}

	for i := len(h.Revisions) - 1; i >= 0; i-- {
		if includedRevIdx[i] {
			continue
		}
		rev := h.Revisions[i]
		if rev.Named || mustIncludeRevID[rev.ID] || i == curIdx || i == viewedIdx {
			entries = append(entries, graphRowEntry{
				revIdx:        i,
				rev:           rev,
				branch:        normalizeGraphBranchName(rev.Branch),
				isUnnamedStep: !rev.Named,
				isUndone:      false,
			})
			includedRevIdx[i] = true
		}
	}

	R := len(entries)
	rowIdxByRevID := make(map[string]int, R)
	for r, e := range entries {
		rowIdxByRevID[e.rev.ID] = r
	}

	// 3. Compute active lineage (all revisions on the ancestor path of the active branch down to r1)
	activeAncestorRevs := make(map[string]bool)
	for _, e := range entries {
		if e.branch == activeBranchName && !e.isUndone {
			activeAncestorRevs[e.rev.ID] = true
		}
	}
	walkID := h.Cursor
	if walkID == "" && len(branchIdxs) > 0 {
		walkID = h.Revisions[branchIdxs[len(branchIdxs)-1]].ID
	}
	visitedWalk := make(map[string]bool)
	for walkID != "" && !visitedWalk[walkID] {
		visitedWalk[walkID] = true
		activeAncestorRevs[walkID] = true
		rev, ok := revByID[walkID]
		if !ok {
			break
		}
		if rev.ParentID != "" {
			walkID = rev.ParentID
			continue
		}
		brName := normalizeGraphBranchName(rev.Branch)
		if bIdx, ok := branchIdxByName[brName]; ok && branches[bIdx].BaseRevID != "" && branches[bIdx].BaseRevID != walkID {
			walkID = branches[bIdx].BaseRevID
			continue
		}
		pos := revPosByID[walkID]
		nextWalk := ""
		for p := pos - 1; p >= 0; p-- {
			if normalizeGraphBranchName(h.Revisions[p].Branch) == brName {
				nextWalk = h.Revisions[p].ID
				break
			}
		}
		walkID = nextWalk
	}
	if len(h.Revisions) > 0 {
		activeAncestorRevs[h.Revisions[0].ID] = true
	}

	// 4. Determine topRow, botRow (fork base row), and activeTopRow for each branch
	B := len(branches)
	topRow := make([]int, B)
	botRow := make([]int, B)
	forkTargetRow := make([]int, B)
	forkTargetBranch := make([]int, B)
	activeTopRow := make([]int, B)
	for bIdx := 0; bIdx < B; bIdx++ {
		topRow[bIdx] = -1
		botRow[bIdx] = -1
		forkTargetRow[bIdx] = -1
		forkTargetBranch[bIdx] = -1
		activeTopRow[bIdx] = R + 1
	}

	for r, e := range entries {
		bIdx, ok := branchIdxByName[e.branch]
		if !ok {
			continue
		}
		if topRow[bIdx] < 0 || r < topRow[bIdx] {
			topRow[bIdx] = r
		}
		if r > botRow[bIdx] {
			botRow[bIdx] = r
		}
		if activeAncestorRevs[e.rev.ID] && r < activeTopRow[bIdx] {
			activeTopRow[bIdx] = r
		}
	}

	// Resolve fork base row for each non-root branch
	findDisplayedAncestorRow := func(startRevID string) int {
		curr := startRevID
		seen := make(map[string]bool)
		for curr != "" && !seen[curr] {
			seen[curr] = true
			if rIdx, ok := rowIdxByRevID[curr]; ok {
				return rIdx
			}
			rev, ok := revByID[curr]
			if !ok {
				break
			}
			curr = rev.ParentID
		}
		return -1
	}

	for bIdx := 0; bIdx < B; bIdx++ {
		br := branches[bIdx]
		if bIdx == 0 || br.Name == branchMain {
			if topRow[bIdx] < 0 {
				topRow[bIdx] = 0
			}
			botRow[bIdx] = R - 1
			continue
		}
		baseCand := br.BaseRevID
		if botRow[bIdx] >= 0 {
			oldestOwnRev := entries[botRow[bIdx]].rev
			if oldestOwnRev.ParentID != "" {
				baseCand = oldestOwnRev.ParentID
			}
		}
		baseR := findDisplayedAncestorRow(baseCand)
		if baseR < 0 && R > 0 {
			baseR = R - 1
		}
		if topRow[bIdx] < 0 && baseR >= 0 {
			topRow[bIdx] = baseR
		}
		if baseR > botRow[bIdx] {
			botRow[bIdx] = baseR
			forkTargetRow[bIdx] = baseR
			if parentIdx, ok := branchIdxByName[entries[baseR].branch]; ok {
				forkTargetBranch[bIdx] = parentIdx
			} else {
				forkTargetBranch[bIdx] = 0
			}
		}
	}

	// Propagate activeTopRow from active child branches into their parent branch at forkTargetRow
	for bIdx := B - 1; bIdx >= 1; bIdx-- {
		if activeTopRow[bIdx] <= botRow[bIdx] && forkTargetRow[bIdx] >= 0 && forkTargetBranch[bIdx] >= 0 {
			pIdx := forkTargetBranch[bIdx]
			if forkTargetRow[bIdx] < activeTopRow[pIdx] {
				activeTopRow[pIdx] = forkTargetRow[bIdx]
			}
		}
	}

	// Find last unnamed row index on active branch so dashed rail connects down to the first named version
	firstNamedRowOnActive := -1
	hasUnnamedOnActive := false
	for r, e := range entries {
		if e.branch == activeBranchName {
			if e.isUnnamedStep {
				hasUnnamedOnActive = true
			} else if firstNamedRowOnActive < 0 {
				firstNamedRowOnActive = r
				break
			}
		}
	}

	// 5. Assign compacted columns per row:
	//    At row r, all branches bIdx with topRow[bIdx] >= 0 && r <= botRow[bIdx] occupy columns 0..K-1 in fixed creation order.
	colOfBranch := make([][]int, R)
	branchAtCol := make([][]int, R)
	for r := 0; r < R; r++ {
		colOfBranch[r] = make([]int, B)
		for bIdx := 0; bIdx < B; bIdx++ {
			colOfBranch[r][bIdx] = -1
		}
		var cols []int
		for bIdx := 0; bIdx < B; bIdx++ {
			if topRow[bIdx] < 0 {
				continue
			}
			if r <= botRow[bIdx] {
				colOfBranch[r][bIdx] = len(cols)
				cols = append(cols, bIdx)
			}
		}
		branchAtCol[r] = cols
	}

	// 6. Build output rows and lane cells
	outRows := make([]historyGraphRowView, 0, R)
	for r, e := range entries {
		rev := e.rev
		rowBranchIdx := branchIdxByName[e.branch]
		nodeCol := colOfBranch[r][rowBranchIdx]
		if nodeCol < 0 {
			nodeCol = 0
		}

		numCols := len(branchAtCol[r])
		if r > 0 && len(branchAtCol[r-1]) > numCols {
			// Check if any branch compacted from r-1 to r; include its previous column so the ╯ turn renders cleanly
			for bIdx := 0; bIdx < B; bIdx++ {
				curC := colOfBranch[r][bIdx]
				prevC := colOfBranch[r-1][bIdx]
				if curC >= 0 && prevC > curC && prevC+1 > numCols {
					numCols = prevC + 1
				}
			}
		}
		if numCols < 1 {
			numCols = 1
		}

		// Check fork connectors on this row (branches that have forkTargetRow[bIdx] == r)
		maxForkCol := -1
		maxActiveForkCol := -1
		forkCornerAtCol := make(map[int]bool)
		activeForkCornerAtCol := make(map[int]bool)
		for bIdx := 1; bIdx < B; bIdx++ {
			if forkTargetRow[bIdx] == r {
				c := colOfBranch[r][bIdx]
				if c > nodeCol {
					forkCornerAtCol[c] = true
					if c > maxForkCol {
						maxForkCol = c
					}
					if activeTopRow[bIdx] <= r {
						activeForkCornerAtCol[c] = true
						if c > maxActiveForkCol {
							maxActiveForkCol = c
						}
					}
				}
			}
		}

		// Check compaction shifts on this row (branches where colOfBranch[r][bIdx] < colOfBranch[r-1][bIdx])
		compactInAtCol := make(map[int]bool)
		compactOutAtCol := make(map[int]bool)
		compactHorizAtCol := make(map[int]bool)
		activeCompactAtCol := make(map[int]bool)
		compactOutBranchAtCol := make(map[int]int)
		if r > 0 {
			for bIdx := 0; bIdx < B; bIdx++ {
				curC := colOfBranch[r][bIdx]
				prevC := colOfBranch[r-1][bIdx]
				if curC >= 0 && prevC > curC {
					isAct := r >= activeTopRow[bIdx]
					compactInAtCol[curC] = true
					compactOutAtCol[prevC] = true
					compactOutBranchAtCol[prevC] = bIdx
					if isAct {
						activeCompactAtCol[curC] = true
						activeCompactAtCol[prevC] = true
					}
					for c := curC + 1; c < prevC; c++ {
						compactHorizAtCol[c] = true
						if isAct {
							activeCompactAtCol[c] = true
						}
					}
				}
			}
		}

		lanes := make([]historyGraphLaneView, 0, numCols)
		for c := 0; c < numCols; c++ {
			bIdx := -1
			if c < len(branchAtCol[r]) {
				bIdx = branchAtCol[r][c]
			} else if outBIdx, ok := compactOutBranchAtCol[c]; ok {
				bIdx = outBIdx
			}

			cellBranch := ""
			cellHint := ""
			isOtherBranch := false
			hasRailUp := false
			hasRailDown := false
			isActiveRailUp := false
			isActiveRailDown := false
			isDashedRailUp := false
			isDashedRailDown := false

			if bIdx >= 0 {
				cellBranch = branches[bIdx].Name
				cellHint = branchHints[cellBranch]
				isOtherBranch = cellBranch != activeBranchName
				if c < len(branchAtCol[r]) {
					hasRailUp = r > topRow[bIdx] && !compactInAtCol[c]
					hasRailDown = r >= topRow[bIdx] && r < botRow[bIdx]
					isActiveRailUp = hasRailUp && r > activeTopRow[bIdx]
					isActiveRailDown = hasRailDown && r >= activeTopRow[bIdx]
					if cellBranch == activeBranchName && hasUnnamedOnActive && firstNamedRowOnActive > 0 {
						isDashedRailUp = hasRailUp && r <= firstNamedRowOnActive
						isDashedRailDown = hasRailDown && r < firstNamedRowOnActive
					}
				}
			}

			isForkCorner := forkCornerAtCol[c]
			if isForkCorner {
				hasRailDown = false
			}
			hasForkHoriz := maxForkCol > nodeCol && c > nodeCol && c < maxForkCol
			isActiveFork := activeForkCornerAtCol[c] || (maxActiveForkCol > nodeCol && c > nodeCol && c < maxActiveForkCol)

			lanes = append(lanes, historyGraphLaneView{
				Branch:           cellBranch,
				HoverHint:        cellHint,
				IsBranchRail:     bIdx >= 0 && (hasRailUp || hasRailDown || isForkCorner || c == nodeCol),
				IsOtherBranch:    isOtherBranch,
				HasRailUp:        hasRailUp,
				HasRailDown:      hasRailDown,
				IsActiveRailUp:   isActiveRailUp,
				IsActiveRailDown: isActiveRailDown,
				IsDashedRailUp:   isDashedRailUp,
				IsDashedRailDown: isDashedRailDown,
				HasNode:          c == nodeCol,
				IsForkCorner:     isForkCorner,
				HasForkHoriz:     hasForkHoriz,
				IsActiveFork:     isActiveFork,
				IsCompactIn:      compactInAtCol[c],
				IsCompactOut:     compactOutAtCol[c],
				HasCompactHoriz:  compactHorizAtCol[c],
				IsActiveCompact:  activeCompactAtCol[c],
				HasStemStart:     c == nodeCol,
				HasStemThrough:   c > nodeCol,
			})
		}

		isActiveHead := e.revIdx == curIdx
		isViewed := e.revIdx == viewedIdx
		isPreview := isHistPreview && isViewed
		isActiveLineage := activeAncestorRevs[rev.ID]
		isActiveBranch := e.branch == activeBranchName
		isBranchHead := branchHeadRevID[e.branch] == rev.ID

		timeStr := formatHistoryCompactTime(rev.TimeUnix)
		metaLabel := fmt.Sprintf(historyMetaFmt, rev.ID, timeStr)
		if isPreview {
			metaLabel = i18n.T(lang, i18n.TrHistoryPreviewPrefix) + rev.ID
		}

		rowHint := revHints[rev.ID]
		if rowHint == "" {
			rowHint = branchHints[e.branch]
		}

		outRows = append(outRows, historyGraphRowView{
			ID:              rev.ID,
			Label:           rev.Label,
			MetaLabel:       metaLabel,
			Branch:          e.branch,
			HoverHint:       rowHint,
			IsUnnamedStep:   e.isUnnamedStep,
			IsCurrent:       isViewed,
			IsActiveHead:    isActiveHead,
			IsPreview:       isPreview,
			IsUndone:        e.isUndone,
			IsActiveLineage: isActiveLineage,
			IsActiveBranch:  isActiveBranch,
			IsBranchHead:    isBranchHead,
			Lanes:           lanes,
		})
	}

	return outRows, branchHints
}

func populateEditorHistoryContext(s *state.AppState, vars map[string]any) {
	var graphRows []historyGraphRowView
	activeBranchName := branchMain
	namedVersionsCount := 0
	branchesCount := 0
	isHistoryPanelOpen := s.Editor.RightPanel == state.PanelHistory

	b := s.Editor.ActiveBook
	if s.Editor.HasActiveBook() && b != nil && !s.Config.StandaloneMode {
		isHistPreview := s.Editor.History.IsPreview(b)
		if !b.IsPacked && (isHistoryPanelOpen || isHistPreview) {
			if isHistoryPanelOpen && !isHistPreview {
				s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
			}
			tDisk := time.Now()
			h := s.Editor.History.RefreshFromDisk(b)
			logging.Active.MarkStep(logging.StageEditorHistoryDisk, tDisk, b.ID)
			if h != nil {
				activeBranchName = h.NormalizedActiveBranch()
				branchesCount = len(h.Branches)
				curIdx := h.CursorIndex()
				previewIdx := -1
				var previewID, previewLabel string
				if h.Preview != nil {
					previewID = h.Preview.RevisionID
					previewLabel = h.Preview.RevisionLabel
				}
				if previewID != "" {
					for i := range h.Revisions {
						if h.Revisions[i].ID == previewID {
							previewIdx = i
							break
						}
					}
					if previewIdx == curIdx {
						h.Preview = nil
						previewID = ""
						previewLabel = ""
						previewIdx = -1
					} else if previewIdx >= 0 {
						previewLabel = h.Revisions[previewIdx].Label
						h.Preview.RevisionLabel = previewLabel
					}
				}
				isHistPreview = previewIdx >= 0 && previewIdx != curIdx
				viewedIdx := curIdx
				if isHistPreview {
					viewedIdx = previewIdx
				}
				vars[keyIsHistoryPreview] = isHistPreview
				if isHistPreview {
					vars[keyHistoryPreviewAttr] = attrTrue
				} else {
					vars[keyHistoryPreviewAttr] = attrFalse
				}
				vars[keyHistoryPreviewID] = previewID
				vars[keyHistoryPreviewLabel] = previewLabel

				if isHistoryPanelOpen {
					tGraph := time.Now()
					graphRows, _ = buildHistoryGraphRows(b, h, curIdx, viewedIdx, isHistPreview, LangOf(s))
					logging.Active.MarkStep(logging.StageEditorHistoryGraph, tGraph, b.ID)
				}

				for _, idx := range h.ActiveBranchRevisions() {
					if h.Revisions[idx].Named {
						namedVersionsCount++
					}
				}
			}
		}
		if isHistoryPanelOpen && len(graphRows) == 0 {
			graphRows, _ = buildHistoryGraphRows(b, nil, -1, -1, false, LangOf(s))
		}
		if namedVersionsCount == 0 {
			namedVersionsCount = 1
		}
		if branchesCount == 0 {
			branchesCount = 1
		}
	}

	vars[keyShowEditorHistoryPanel] = isHistoryPanelOpen
	vars[keyActiveBranchName] = activeBranchName
	vars[keyHistoryGraphRows] = graphRows
	vars[keyDefaultNextVersionName] = trf(s, i18n.TrHistoryDefaultVersionFmt, namedVersionsCount+1)
	vars[keyDefaultNextBranchName] = trf(s, i18n.TrHistoryDefaultBranchFmt, branchesCount+1)
}
