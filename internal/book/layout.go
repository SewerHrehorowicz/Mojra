package book

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func orderChaptersAndScenesByGraph(b *Book, prevChapterMeta map[string]Chapter) {
	if len(b.Scenes) == 0 {
		return
	}

	sceneMap := make(map[string]*Scene, len(b.Scenes))
	for _, sc := range b.Scenes {
		sceneMap[sc.ID] = sc
	}

	startID := b.StartSceneID
	if _, ok := sceneMap[startID]; !ok {
		startID = b.Scenes[0].ID
	}

	var chapterKeys []string
	chapterSeen := make(map[string]bool)
	recordChapter := func(chName string) {
		key := strings.ToLower(strings.TrimSpace(chName))
		if key != "" && !chapterSeen[key] {
			chapterSeen[key] = true
			chapterKeys = append(chapterKeys, key)
		}
	}

	visited := make(map[string]bool)
	queue := []string{startID}
	visited[startID] = true
	for len(queue) > 0 {
		currID := queue[0]
		queue = queue[1:]
		sc := sceneMap[currID]
		if sc == nil {
			continue
		}
		recordChapter(sc.Chapter)
		for _, ch := range sc.Choices {
			if ch.IsEnding || ch.TargetSceneID == "" || ch.TargetSceneID == "END" {
				continue
			}
			if !visited[ch.TargetSceneID] {
				visited[ch.TargetSceneID] = true
				queue = append(queue, ch.TargetSceneID)
			}
		}
	}

	for _, sc := range b.Scenes {
		sc.Unreachable = !visited[sc.ID]
		recordChapter(sc.Chapter)
	}

	// When scenes come from multiple .txt files, order b.Scenes by the discovered chapter graph order
	// while preserving the exact relative order of scenes within each chapter/file.
	if b.IsMultiFileBook() && len(chapterKeys) > 1 {
		reordered := make([]*Scene, 0, len(b.Scenes))
		for _, chKey := range chapterKeys {
			for _, sc := range b.Scenes {
				if strings.ToLower(strings.TrimSpace(sc.Chapter)) == chKey {
					reordered = append(reordered, sc)
				}
			}
		}
		if len(reordered) == len(b.Scenes) {
			for i, sc := range reordered {
				sc.OrderIndex = i
			}
			b.Scenes = reordered
		}
	}

	// Rebuild b.Chapters in graph order, restoring ExplicitID, ID, SourceFile, and StartSceneID
	b.Chapters = nil
	for _, chKey := range chapterKeys {
		for _, sc := range b.Scenes {
			if strings.ToLower(strings.TrimSpace(sc.Chapter)) == chKey {
				expID := sc.ChapterID
				sf := sc.SourceFile
				if meta, ok := prevChapterMeta[chKey]; ok {
					if expID == "" {
						expID = meta.ExplicitID
					}
					if sf == "" {
						sf = meta.SourceFile
					}
				}
				b.addSceneToChapterWithMeta(sc.Chapter, expID, sf, sc.ID)
			}
		}
	}
	for i := range b.Chapters {
		ch := &b.Chapters[i]
		if ch.ID == "" {
			ch.ID = ch.ReferenceKey()
		}
		if len(ch.SceneIDs) > 0 {
			ch.StartSceneID = ch.SceneIDs[0]
			for _, sid := range ch.SceneIDs {
				if strings.EqualFold(sid, b.StartSceneID) {
					ch.StartSceneID = b.StartSceneID
					break
				}
			}
		}
	}
}

const EditorGridSize = 24

// SnapToGrid rounds an integer coordinate to the nearest multiple of EditorGridSize (24px).
func SnapToGrid(v int) int {
	if v >= 0 {
		return ((v + EditorGridSize/2) / EditorGridSize) * EditorGridSize
	}
	return -(((-v + EditorGridSize/2) / EditorGridSize) * EditorGridSize)
}

// AutoLayoutBook clears manual layout overrides and computes a clean, chapter-grouped,
// grid-aligned hierarchical layout for all scenes in the book.
func AutoLayoutBook(b *Book) {
	if b == nil || len(b.Scenes) == 0 {
		return
	}
	b.Layout = make(map[string]NodePosition)
	ComputeTimelineLayout(b)
	for _, sc := range b.Scenes {
		pos := NodePosition{
			Row:   sc.Row,
			Col:   sc.Col,
			X:     sc.X,
			Y:     sc.Y,
			HasXY: true,
		}
		b.Layout[sc.ID] = pos
		if sc.ExplicitID != "" {
			b.Layout[sc.ExplicitID] = pos
			b.Layout[strings.ToLower(sc.ExplicitID)] = pos
		}
		if sc.Name != "" {
			b.Layout[sc.Name] = pos
			b.Layout[NormalizeLookupKey(sc.Name)] = pos
		}
	}
}

// ComputeTimelineLayout assigns Row (vertical chronological step) and Col (parallel branch column)
// to every scene in the book, respecting manual overrides in b.Layout if present.
func ComputeTimelineLayout(b *Book) {
	if len(b.Scenes) == 0 {
		return
	}
	if b.Layout == nil {
		b.Layout = make(map[string]NodePosition)
	}

	sceneMap := make(map[string]*Scene, len(b.Scenes))
	for _, sc := range b.Scenes {
		sceneMap[sc.ID] = sc
		sc.Row = 0
		sc.Col = 0
	}

	type queueItem struct {
		id  string
		row int
		col int
	}

	startID := b.StartSceneID
	if _, ok := sceneMap[startID]; !ok {
		startID = b.Scenes[0].ID
	}

	visited := make(map[string]bool)
	rowColsUsed := make(map[int]map[int]bool)

	occupy := func(r, c int) int {
		if r < 1 {
			r = 1
		}
		if c < 1 {
			c = 1
		}
		if rowColsUsed[r] == nil {
			rowColsUsed[r] = make(map[int]bool)
		}
		for rowColsUsed[r][c] {
			c++
		}
		rowColsUsed[r][c] = true
		return c
	}

	queue := []queueItem{{id: startID, row: 1, col: 1}}
	visited[startID] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		sc := sceneMap[curr.id]
		if sc == nil {
			continue
		}

		r, c := curr.row, curr.col
		if manual, ok := b.Layout[sc.ID]; ok && manual.Row > 0 && manual.Col > 0 {
			r = manual.Row
			c = manual.Col
		}
		c = occupy(r, c)
		sc.Row = r
		sc.Col = c

		branchOffset := 0
		for _, ch := range sc.Choices {
			if ch.IsEnding || ch.TargetSceneID == "" || ch.TargetSceneID == "END" {
				continue
			}
			if !visited[ch.TargetSceneID] {
				visited[ch.TargetSceneID] = true
				queue = append(queue, queueItem{
					id:  ch.TargetSceneID,
					row: r + 1,
					col: c + branchOffset,
				})
				branchOffset++
			}
		}
	}

	// Assign any unvisited scenes to subsequent rows
	maxRow := 1
	for _, sc := range b.Scenes {
		sc.Unreachable = !visited[sc.ID]
		if sc.Row > maxRow {
			maxRow = sc.Row
		}
	}
	for _, sc := range b.Scenes {
		if sc.Row == 0 {
			maxRow++
			r := maxRow
			c := 1
			if manual, ok := b.Layout[sc.ID]; ok && manual.Row > 0 && manual.Col > 0 {
				r = manual.Row
				c = manual.Col
			}
			c = occupy(r, c)
			sc.Row = r
			sc.Col = c
		}
	}

	const (
		colSpacing = 264 // 11 * 24px (192px card + 72px gap)
		baseRowH   = 168 // 7 * 24px
		chapterGap = 120 // 5 * 24px extra vertical gap between chapter groups
		centerX    = 960 // 40 * 24px
		startY     = 96  // 4 * 24px
		cardHalfW  = 96
	)

	// Group scenes by chapter order and then by BFS row so chapter groups never overlap
	chapterOrder := make([]string, 0, len(b.Chapters)+1)
	chapterSeen := make(map[string]bool)
	for _, ch := range b.Chapters {
		key := strings.ToLower(strings.TrimSpace(ch.Name))
		if key != "" && !chapterSeen[key] {
			chapterSeen[key] = true
			chapterOrder = append(chapterOrder, key)
		}
	}
	for _, sc := range b.Scenes {
		chName := strings.TrimSpace(sc.Chapter)
		if chName == "" {
			chName = "Rozdział 1"
		}
		key := strings.ToLower(chName)
		if !chapterSeen[key] {
			chapterSeen[key] = true
			chapterOrder = append(chapterOrder, key)
		}
	}

	defaultCoords := make(map[string][2]int, len(b.Scenes))
	curY := startY

	for chIdx, chKey := range chapterOrder {
		var chScenes []*Scene
		for _, sc := range b.Scenes {
			name := strings.TrimSpace(sc.Chapter)
			if name == "" {
				name = "Rozdział 1"
			}
			if strings.ToLower(name) == chKey {
				chScenes = append(chScenes, sc)
			}
		}
		if len(chScenes) == 0 {
			continue
		}
		if chIdx > 0 {
			curY += chapterGap
		}

		byRow := make(map[int][]*Scene)
		var rows []int
		for _, sc := range chScenes {
			if _, ok := byRow[sc.Row]; !ok {
				rows = append(rows, sc.Row)
			}
			byRow[sc.Row] = append(byRow[sc.Row], sc)
		}
		sort.Ints(rows)

		for _, r := range rows {
			band := byRow[r]
			sort.SliceStable(band, func(i, j int) bool {
				if band[i].Col != band[j].Col {
					return band[i].Col < band[j].Col
				}
				return band[i].OrderIndex < band[j].OrderIndex
			})
			cnt := len(band)
			totalSpan := (cnt - 1) * colSpacing
			rowStartX := SnapToGrid(centerX - totalSpan/2 - cardHalfW)

			maxChoices := 1
			maxWarnings := 0
			for i, sc := range band {
				dx := rowStartX + i*colSpacing
				dy := curY
				defaultCoords[sc.ID] = [2]int{dx, dy}
				if len(sc.Choices) > maxChoices {
					maxChoices = len(sc.Choices)
				}
				if len(sc.Warnings) > maxWarnings {
					maxWarnings = len(sc.Warnings)
				}
			}

			rowStep := baseRowH
			if maxChoices > 1 {
				rowStep += SnapToGrid((maxChoices - 1) * 28)
			}
			if maxWarnings > 0 {
				rowStep += 24
			}
			curY += rowStep
		}
	}

	for _, sc := range b.Scenes {
		def := defaultCoords[sc.ID]
		if pos, ok := lookupNodePosition(b.Layout, sc); ok && pos.HasXY {
			sc.X = pos.X
			sc.Y = pos.Y
		} else {
			sc.X = def[0]
			sc.Y = def[1]
		}
	}
}

func lookupNodePosition(layout map[string]NodePosition, sc *Scene) (NodePosition, bool) {
	if len(layout) == 0 || sc == nil {
		return NodePosition{}, false
	}
	if sc.ExplicitID != "" {
		if pos, ok := layout[strings.ToLower(sc.ExplicitID)]; ok {
			return pos, true
		}
		if pos, ok := layout[sc.ExplicitID]; ok {
			return pos, true
		}
	}
	if pos, ok := layout[sc.ID]; ok {
		return pos, true
	}
	if sc.Name != "" {
		if pos, ok := layout[NormalizeLookupKey(sc.Name)]; ok {
			return pos, true
		}
		if pos, ok := layout[sc.Name]; ok {
			return pos, true
		}
	}
	return NodePosition{}, false
}

// ParseEditorFile parses editor.txt containing Node Editor canvas viewport and node (x, y) coordinates.
func ParseEditorFile(raw string, b *Book) {
	if b.Layout == nil {
		b.Layout = make(map[string]NodePosition)
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		colonIdx := strings.IndexByte(trimmed, ':')
		if colonIdx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trimmed[:colonIdx]))
		val := strings.TrimSpace(trimmed[colonIdx+1:])
		switch key {
		case "pan_x":
			if v, err := strconv.Atoi(val); err == nil {
				b.EditorPanX = v
				b.HasEditorView = true
			}
		case "pan_y":
			if v, err := strconv.Atoi(val); err == nil {
				b.EditorPanY = v
				b.HasEditorView = true
			}
		case "zoom":
			if v, err := strconv.ParseFloat(val, 64); err == nil && v >= 0.2 && v <= 3.0 {
				b.EditorZoom = v
				b.HasEditorView = true
			}
		case "node":
			arrowIdx := strings.LastIndex(val, "->")
			if arrowIdx <= 0 {
				continue
			}
			nodeKey := strings.TrimSpace(val[:arrowIdx])
			coordPart := strings.TrimSpace(val[arrowIdx+2:])
			parts := strings.Split(coordPart, ",")
			if len(parts) != 2 || nodeKey == "" {
				continue
			}
			x, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
			y, errY := strconv.Atoi(strings.TrimSpace(parts[1]))
			if errX == nil && errY == nil {
				pos := NodePosition{X: x, Y: y, HasXY: true}
				b.Layout[nodeKey] = pos
				b.Layout[strings.ToLower(nodeKey)] = pos
				b.Layout[NormalizeLookupKey(nodeKey)] = pos
			}
		}
	}
}

// SerializeEditorFile serializes Node Editor canvas viewport and node (x, y) positions into editor.txt format.
func SerializeEditorFile(b *Book) string {
	var sb strings.Builder
	sb.WriteString("# StoryTeller — Stan edytora węzłów (editor.txt)\n")
	zoom := b.EditorZoom
	if zoom < 0.2 || zoom > 3.0 {
		zoom = 1.0
	}
	fmt.Fprintf(&sb, "pan_x: %d\n", b.EditorPanX)
	fmt.Fprintf(&sb, "pan_y: %d\n", b.EditorPanY)
	fmt.Fprintf(&sb, "zoom: %.2f\n\n", zoom)
	for _, sc := range b.Scenes {
		ref := sc.ID
		if sc.ExplicitID != "" {
			ref = sc.ExplicitID
		} else if sc.Name != "" {
			ref = sc.Name
		}
		fmt.Fprintf(&sb, "node: %s -> %d, %d\n", ref, sc.X, sc.Y)
	}
	return sb.String()
}
