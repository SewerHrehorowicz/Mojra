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
			canonicalizeAutoSceneIDsAfterReorder(b, sceneMap)
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
		if ch.ExplicitStart == "" {
			if meta, ok := prevChapterMeta[strings.ToLower(strings.TrimSpace(ch.Name))]; ok && meta.ExplicitStart != "" {
				ch.ExplicitStart = meta.ExplicitStart
			} else if meta, ok := prevChapterMeta[strings.ToLower(strings.TrimSpace(ch.ExplicitID))]; ok && meta.ExplicitStart != "" {
				ch.ExplicitStart = meta.ExplicitStart
			} else if meta, ok := prevChapterMeta[strings.ToLower(strings.TrimSpace(ch.ID))]; ok && meta.ExplicitStart != "" {
				ch.ExplicitStart = meta.ExplicitStart
			}
		}
		if len(ch.SceneIDs) > 0 {
			ch.StartSceneID = ch.SceneIDs[0]
			for _, sid := range ch.SceneIDs {
				if strings.EqualFold(sid, b.StartSceneID) {
					ch.StartSceneID = b.StartSceneID
					break
				}
			}
			if expStart := strings.TrimSpace(ch.ExplicitStart); expStart != "" {
				normStart := NormalizeLookupKey(expStart)
				for _, sid := range ch.SceneIDs {
					sc := sceneMap[sid]
					if sc == nil {
						continue
					}
					if strings.EqualFold(sc.ID, expStart) ||
						(sc.ExplicitID != "" && strings.EqualFold(sc.ExplicitID, expStart)) ||
						(normStart != "" && NormalizeLookupKey(sc.Name) == normStart) {
						ch.StartSceneID = sc.ID
						break
					}
				}
			}
		}
	}
}

func canonicalizeAutoSceneIDsAfterReorder(b *Book, sceneMap map[string]*Scene) {
	if b == nil || len(b.Scenes) == 0 {
		return
	}
	usedIDs := make(map[string]bool, len(b.Scenes))
	hasAuto := false
	for _, sc := range b.Scenes {
		if sc.ExplicitID != "" {
			usedIDs[strings.ToLower(sc.ExplicitID)] = true
		} else {
			hasAuto = true
		}
	}
	if !hasAuto {
		return
	}

	idRemap := make(map[string]string)
	nextAuto := 1
	for _, sc := range b.Scenes {
		if sc.ExplicitID != "" {
			continue
		}
		for {
			cand := fmt.Sprintf("s%d", nextAuto)
			nextAuto++
			if !usedIDs[strings.ToLower(cand)] {
				usedIDs[strings.ToLower(cand)] = true
				if sc.ID != cand {
					idRemap[sc.ID] = cand
					sc.ID = cand
				}
				break
			}
		}
	}
	if len(idRemap) == 0 {
		return
	}

	for k := range sceneMap {
		delete(sceneMap, k)
	}
	for _, sc := range b.Scenes {
		sceneMap[sc.ID] = sc
		for i := range sc.Choices {
			if newTgt, ok := idRemap[sc.Choices[i].TargetSceneID]; ok {
				sc.Choices[i].TargetSceneID = newTgt
			}
		}
	}
	if newStart, ok := idRemap[b.StartSceneID]; ok {
		b.StartSceneID = newStart
	}
}

const (
	EditorGridSize = 24
	maxCanvasCoord = 1_000_000
)

func clampCanvasCoord(v int) int {
	if v > maxCanvasCoord {
		return maxCanvasCoord
	}
	if v < -maxCanvasCoord {
		return -maxCanvasCoord
	}
	return v
}

// SnapToGrid rounds an integer coordinate to the nearest multiple of EditorGridSize (24px).
func SnapToGrid(v int) int {
	v = clampCanvasCoord(v)
	if v >= 0 {
		return ((v + EditorGridSize/2) / EditorGridSize) * EditorGridSize
	}
	return -(((-v + EditorGridSize/2) / EditorGridSize) * EditorGridSize)
}

// StoreSceneNodePosition records a scene's position in b.Layout using stable keys.
// When a scene has no ExplicitID and has a non-empty Name, it stores only under Name / normalized Name
// and removes any ephemeral positional auto-ID ("s1", "s2") from b.Layout so reordering never swaps coordinates.
func StoreSceneNodePosition(b *Book, sc *Scene, pos NodePosition) {
	if b == nil || sc == nil {
		return
	}
	if b.Layout == nil {
		b.Layout = make(map[string]NodePosition)
	}
	trimmedName := strings.TrimSpace(sc.Name)
	if sc.ExplicitID != "" {
		b.Layout[sc.ExplicitID] = pos
		b.Layout[strings.ToLower(sc.ExplicitID)] = pos
		if sc.ID != "" {
			b.Layout[sc.ID] = pos
			b.Layout[strings.ToLower(sc.ID)] = pos
		}
	} else if trimmedName != "" {
		if sc.ID != "" && !strings.EqualFold(sc.ID, trimmedName) && !strings.EqualFold(sc.ID, NormalizeLookupKey(trimmedName)) {
			delete(b.Layout, sc.ID)
			delete(b.Layout, strings.ToLower(sc.ID))
		}
	} else if sc.ID != "" {
		b.Layout[sc.ID] = pos
		b.Layout[strings.ToLower(sc.ID)] = pos
	}
	if trimmedName != "" {
		b.Layout[sc.Name] = pos
		if norm := NormalizeLookupKey(sc.Name); norm != "" {
			b.Layout[norm] = pos
		}
	}
}

// AutoLayoutBook clears manual layout overrides and computes a clean, chapter-grouped,
// grid-aligned hierarchical layout for all scenes in the book.
func AutoLayoutBook(b *Book) {
	if b == nil || len(b.Scenes) == 0 {
		return
	}
	b.Layout = make(map[string]NodePosition)
	b.WirePins = make(map[string][]WirePin)
	ComputeTimelineLayout(b)
	for _, sc := range b.Scenes {
		pos := NodePosition{
			Row:   sc.Row,
			Col:   sc.Col,
			X:     sc.X,
			Y:     sc.Y,
			HasXY: true,
		}
		StoreSceneNodePosition(b, sc, pos)
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
		colSpacing    = 264 // 11 * 24px (192px card + 72px gap)
		baseRowH      = 168 // 7 * 24px
		chapterGap    = 144 // 6 * 24px vertical gap between chapter tiers
		chapterColGap = 144 // 6 * 24px horizontal gap between parallel chapter frames
		centerX       = 960 // 40 * 24px
		startY        = 96  // 4 * 24px
		cardHalfW     = 96
		cardW         = 192
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

	type chLayoutPlan struct {
		key        string
		idx        int
		scenes     []*Scene
		relX       map[string]int
		relY       map[string]int
		minRelX    int
		maxRelX    int
		localEndY  int
		frameW     int
		tier       int
		centerX    int
		startY     int
	}

	plans := make([]*chLayoutPlan, 0, len(chapterOrder))
	planByKey := make(map[string]*chLayoutPlan, len(chapterOrder))

	for _, chKey := range chapterOrder {
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

		byRow := make(map[int][]*Scene)
		var rows []int
		for _, sc := range chScenes {
			if _, ok := byRow[sc.Row]; !ok {
				rows = append(rows, sc.Row)
			}
			byRow[sc.Row] = append(byRow[sc.Row], sc)
		}
		sort.Ints(rows)

		relX := make(map[string]int, len(chScenes))
		relY := make(map[string]int, len(chScenes))
		minRelX := 0
		maxRelX := 0
		hasRelBounds := false
		localY := 0

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
			rowStartRelX := SnapToGrid(-totalSpan/2 - cardHalfW)

			maxChoices := 1
			maxWarnings := 0
			for i, sc := range band {
				rx := rowStartRelX + i*colSpacing
				ry := localY
				relX[sc.ID] = rx
				relY[sc.ID] = ry
				if !hasRelBounds || rx < minRelX {
					minRelX = rx
				}
				if !hasRelBounds || rx+cardW > maxRelX {
					maxRelX = rx + cardW
				}
				hasRelBounds = true
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
			maxSceneH := 74
			if maxChoices >= 2 {
				maxSceneH = 74 + maxChoices*26
			}
			if minStep := ((maxSceneH + 48 + 23) / 24) * 24; rowStep < minStep {
				rowStep = minStep
			}
			localY += rowStep
		}

		plan := &chLayoutPlan{
			key:       chKey,
			idx:       len(plans),
			scenes:    chScenes,
			relX:      relX,
			relY:      relY,
			minRelX:   minRelX,
			maxRelX:   maxRelX,
			localEndY: localY,
			frameW:    (maxRelX - minRelX) + 72,
			tier:      -1,
			centerX:   centerX,
			startY:    startY,
		}
		plans = append(plans, plan)
		planByKey[chKey] = plan
	}

	if len(plans) > 0 {
		outEdges := make(map[string][]string, len(plans))
		outSeen := make(map[string]map[string]bool, len(plans))
		for _, uPlan := range plans {
			uKey := uPlan.key
			orderedScenes := make([]*Scene, len(uPlan.scenes))
			copy(orderedScenes, uPlan.scenes)
			sort.SliceStable(orderedScenes, func(i, j int) bool {
				if orderedScenes[i].Row != orderedScenes[j].Row {
					return orderedScenes[i].Row < orderedScenes[j].Row
				}
				if orderedScenes[i].Col != orderedScenes[j].Col {
					return orderedScenes[i].Col < orderedScenes[j].Col
				}
				return orderedScenes[i].OrderIndex < orderedScenes[j].OrderIndex
			})
			for _, sc := range orderedScenes {
				for _, ch := range sc.Choices {
					if ch.IsEnding {
						continue
					}
					vKey := ""
					if ch.IsChapterLink && ch.TargetChapterID != "" {
						if tCh := b.FindChapter(ch.TargetChapterID); tCh != nil {
							vKey = strings.ToLower(strings.TrimSpace(tCh.Name))
						}
					}
					if vKey == "" && ch.TargetSceneID != "" && ch.TargetSceneID != "END" {
						if tSc := sceneMap[ch.TargetSceneID]; tSc != nil {
							tName := strings.TrimSpace(tSc.Chapter)
							if tName == "" {
								tName = "Rozdział 1"
							}
							vKey = strings.ToLower(tName)
						}
					}
					if vKey == "" || vKey == uKey || planByKey[vKey] == nil {
						continue
					}
					if outSeen[uKey] == nil {
						outSeen[uKey] = make(map[string]bool)
					}
					if !outSeen[uKey][vKey] {
						outSeen[uKey][vKey] = true
						outEdges[uKey] = append(outEdges[uKey], vKey)
					}
				}
			}
		}

		bfsTier := make(map[string]int, len(plans))
		for _, p := range plans {
			bfsTier[p.key] = -1
		}
		rootKey := plans[0].key
		bfsTier[rootKey] = 0
		q := []string{rootKey}
		for len(q) > 0 {
			u := q[0]
			q = q[1:]
			for _, v := range outEdges[u] {
				if bfsTier[v] == -1 {
					bfsTier[v] = bfsTier[u] + 1
					q = append(q, v)
				}
			}
		}

		bfsParents := make(map[string]map[string]bool, len(plans))
		for _, uPlan := range plans {
			u := uPlan.key
			if bfsTier[u] < 0 {
				continue
			}
			for _, v := range outEdges[u] {
				if bfsTier[v] > bfsTier[u] {
					if bfsParents[v] == nil {
						bfsParents[v] = make(map[string]bool)
					}
					bfsParents[v][u] = true
				}
			}
		}

		areParallelSiblings := func(u, v string) bool {
			if bfsTier[u] < 0 || bfsTier[u] != bfsTier[v] {
				return false
			}
			if len(outEdges[u]) == 0 || len(outEdges[v]) == 0 {
				return false
			}
			for p := range bfsParents[u] {
				if bfsParents[v][p] {
					return true
				}
			}
			return false
		}

		for _, p := range plans {
			p.tier = bfsTier[p.key]
		}
		for pass := 0; pass < len(plans); pass++ {
			changed := false
			for _, uPlan := range plans {
				if uPlan.tier < 0 {
					continue
				}
				for _, vKey := range outEdges[uPlan.key] {
					vPlan := planByKey[vKey]
					if vPlan == nil || uPlan.idx >= vPlan.idx {
						continue
					}
					if areParallelSiblings(uPlan.key, vKey) {
						continue
					}
					if vPlan.tier < uPlan.tier+1 {
						vPlan.tier = uPlan.tier + 1
						changed = true
					}
				}
			}
			if !changed {
				break
			}
		}

		maxTier := 0
		for _, p := range plans {
			if p.tier > maxTier {
				maxTier = p.tier
			}
		}
		for _, p := range plans {
			if p.tier < 0 {
				maxTier++
				p.tier = maxTier
			}
		}

		tiers := make([][]*chLayoutPlan, maxTier+1)
		for _, p := range plans {
			tiers[p.tier] = append(tiers[p.tier], p)
		}

		curY := startY
		for t := 0; t <= maxTier; t++ {
			tierPlans := tiers[t]
			if len(tierPlans) == 0 {
				continue
			}
			if t > 0 {
				curY += chapterGap
			}

			if len(tierPlans) > 1 {
				prefByKey := make(map[string]float64, len(tierPlans))
				for _, cPlan := range tierPlans {
					sumPref := 0.0
					cntPref := 0
					for _, pPlan := range plans {
						if pPlan.tier < 0 || pPlan.tier >= t {
							continue
						}
						var forwardTargets []string
						for _, vk := range outEdges[pPlan.key] {
							if vp := planByKey[vk]; vp != nil && vp.tier > pPlan.tier {
								forwardTargets = append(forwardTargets, vk)
							}
						}
						for rankIdx, vk := range forwardTargets {
							if vk == cPlan.key {
								offset := (float64(rankIdx) - float64(len(forwardTargets)-1)*0.5) * float64(colSpacing)
								sumPref += float64(pPlan.centerX) + offset
								cntPref++
							}
						}
					}
					if cntPref > 0 {
						prefByKey[cPlan.key] = sumPref / float64(cntPref)
					} else {
						prefByKey[cPlan.key] = float64(centerX + cPlan.idx*colSpacing)
					}
				}

				sort.SliceStable(tierPlans, func(i, j int) bool {
					pi, pj := prefByKey[tierPlans[i].key], prefByKey[tierPlans[j].key]
					if pi != pj {
						return pi < pj
					}
					return tierPlans[i].idx < tierPlans[j].idx
				})

				totalTierW := (len(tierPlans) - 1) * chapterColGap
				for _, p := range tierPlans {
					totalTierW += p.frameW
				}
				curFrameLeftX := centerX - totalTierW/2
				for _, p := range tierPlans {
					leftOff := p.minRelX - 36
					rightOff := p.maxRelX + 36
					p.centerX = SnapToGrid(curFrameLeftX - leftOff)
					curFrameLeftX = (p.centerX + rightOff) + chapterColGap
				}
			} else {
				tierPlans[0].centerX = centerX
			}

			maxLocalEndY := 0
			for _, p := range tierPlans {
				p.startY = SnapToGrid(curY)
				if p.localEndY > maxLocalEndY {
					maxLocalEndY = p.localEndY
				}
			}
			curY = SnapToGrid(curY + maxLocalEndY)
		}
	}

	defaultCoords := make(map[string][2]int, len(b.Scenes))
	for _, p := range plans {
		for _, sc := range p.scenes {
			dx := SnapToGrid(p.centerX + p.relX[sc.ID])
			dy := SnapToGrid(p.startY + p.relY[sc.ID])
			defaultCoords[sc.ID] = [2]int{dx, dy}
		}
	}

	hasManual := make(map[string]bool, len(b.Scenes))
	for _, sc := range b.Scenes {
		def := defaultCoords[sc.ID]
		if pos, ok := lookupNodePosition(b.Layout, sc); ok && pos.HasXY {
			sc.X = pos.X
			sc.Y = pos.Y
			hasManual[sc.ID] = true
		} else {
			sc.X = def[0]
			sc.Y = def[1]
		}
	}

	resolveNodeAndChapterOverlaps(b, chapterOrder, sceneMap, hasManual)
}

func sceneCanvasHeight(sc *Scene) int {
	if sc == nil {
		return 74
	}
	if len(sc.Choices) >= 2 {
		return 74 + len(sc.Choices)*26
	}
	return 74
}

func nodesOverlapWithMargin(ax, ay, ah, bx, by, bh, margin int) bool {
	const nodeW = 192
	return ax < bx+nodeW+margin &&
		ax+nodeW+margin > bx &&
		ay < by+bh+margin &&
		ay+ah+margin > by
}

func resolveNodeAndChapterOverlaps(b *Book, chapterOrder []string, sceneMap map[string]*Scene, hasManual map[string]bool) {
	if b == nil || len(b.Scenes) == 0 {
		return
	}
	const (
		nodeW      = 192
		nodeMargin = 48
		chGap      = 48
	)

	// Build parent and child lookup within the same chapter so newly created scenes without manual XY
	// land near their connected neighbor in the same chapter rather than colliding with other chapters.
	parentInChapter := make(map[string]*Scene, len(b.Scenes))
	childInChapter := make(map[string]*Scene, len(b.Scenes))
	for _, sc := range b.Scenes {
		scCh := strings.ToLower(strings.TrimSpace(sc.Chapter))
		if scCh == "" {
			scCh = "rozdział 1"
		}
		for _, ch := range sc.Choices {
			if ch.IsEnding || ch.TargetSceneID == "" || ch.TargetSceneID == "END" {
				continue
			}
			target := sceneMap[ch.TargetSceneID]
			if target == nil {
				continue
			}
			tCh := strings.ToLower(strings.TrimSpace(target.Chapter))
			if tCh == "" {
				tCh = "rozdział 1"
			}
			if scCh == tCh {
				if parentInChapter[target.ID] == nil {
					parentInChapter[target.ID] = sc
				}
				if childInChapter[sc.ID] == nil {
					childInChapter[sc.ID] = target
				}
			}
		}
	}

	type chGroup struct {
		key       string
		scenes    []*Scene
		anyManual bool
		left      int
		top       int
		right     int
		bottom    int
	}

	recomputeBounds := func(g *chGroup) {
		if len(g.scenes) == 0 {
			return
		}
		minX, minY := g.scenes[0].X, g.scenes[0].Y
		maxX := g.scenes[0].X + nodeW
		h0 := sceneCanvasHeight(g.scenes[0])
		if h0 < 120 {
			h0 = 120
		}
		maxY := g.scenes[0].Y + h0
		for _, sc := range g.scenes[1:] {
			if sc.X < minX {
				minX = sc.X
			}
			if sc.Y < minY {
				minY = sc.Y
			}
			if sc.X+nodeW > maxX {
				maxX = sc.X + nodeW
			}
			sh := sceneCanvasHeight(sc)
			if sh < 120 {
				sh = 120
			}
			if sc.Y+sh > maxY {
				maxY = sc.Y + sh
			}
		}
		g.left = minX - 36
		g.top = minY - 52
		g.right = maxX + 36
		g.bottom = maxY + 44
	}

	var groups []*chGroup
	for _, chKey := range chapterOrder {
		var chScenes []*Scene
		anyManualInCh := false
		for _, sc := range b.Scenes {
			name := strings.TrimSpace(sc.Chapter)
			if name == "" {
				name = "Rozdział 1"
			}
			if strings.ToLower(name) == chKey {
				chScenes = append(chScenes, sc)
				if hasManual[sc.ID] {
					anyManualInCh = true
				}
			}
		}
		if len(chScenes) == 0 {
			continue
		}
		g := &chGroup{key: chKey, scenes: chScenes, anyManual: anyManualInCh}
		recomputeBounds(g)
		groups = append(groups, g)
	}

	for _, g := range groups {
		chScenes := g.scenes
		anyManualInCh := g.anyManual
		placed := make([]*Scene, 0, len(chScenes))
		for _, sc := range chScenes {
			if hasManual[sc.ID] {
				placed = append(placed, sc)
			}
		}

		collides := func(cx, cy, sh int, checkOtherChapters bool) bool {
			for _, pl := range placed {
				if nodesOverlapWithMargin(cx, cy, sh, pl.X, pl.Y, sceneCanvasHeight(pl), nodeMargin) {
					return true
				}
			}
			for _, otherG := range groups {
				if otherG == g {
					continue
				}
				for _, pl := range otherG.scenes {
					if nodesOverlapWithMargin(cx, cy, sh, pl.X, pl.Y, sceneCanvasHeight(pl), nodeMargin) {
						return true
					}
				}
			}
			if checkOtherChapters && len(groups) > 1 {
				minX, minY := cx, cy
				maxX := cx + nodeW
				nh := sh
				if nh < 120 {
					nh = 120
				}
				maxY := cy + nh
				for _, pl := range placed {
					if pl.X < minX {
						minX = pl.X
					}
					if pl.Y < minY {
						minY = pl.Y
					}
					if pl.X+nodeW > maxX {
						maxX = pl.X + nodeW
					}
					ph := sceneCanvasHeight(pl)
					if ph < 120 {
						ph = 120
					}
					if pl.Y+ph > maxY {
						maxY = pl.Y + ph
					}
				}
				cLeft := minX - 36
				cTop := minY - 52
				cRight := maxX + 36
				cBottom := maxY + 44
				for _, otherG := range groups {
					if otherG == g {
						continue
					}
					if cLeft < otherG.right+24 &&
						cRight+24 > otherG.left &&
						cTop < otherG.bottom+24 &&
						cBottom+24 > otherG.top {
						return true
					}
				}
			}
			return false
		}

		for _, sc := range chScenes {
			if !hasManual[sc.ID] {
				sh := sceneCanvasHeight(sc)
				if anyManualInCh {
					if p := parentInChapter[sc.ID]; p != nil {
						sc.X = SnapToGrid(p.X)
						sc.Y = SnapToGrid(p.Y + sceneCanvasHeight(p) + 72)
					} else if c := childInChapter[sc.ID]; c != nil {
						sc.X = SnapToGrid(c.X)
						sc.Y = SnapToGrid(c.Y - sh - 72)
					} else if len(placed) > 0 {
						last := placed[len(placed)-1]
						sc.X = SnapToGrid(last.X)
						sc.Y = SnapToGrid(last.Y + sceneCanvasHeight(last) + 72)
					}
				}
				if collides(sc.X, sc.Y, sh, true) {
					baseX, baseY := sc.X, sc.Y
					found := false
					for _, enforceCh := range []bool{true, false} {
						for step := 1; step <= 40 && !found; step++ {
							candidates := [][2]int{
								{baseX, SnapToGrid(baseY + step*24)},
								{SnapToGrid(baseX - step*240), baseY},
								{SnapToGrid(baseX + step*240), baseY},
								{SnapToGrid(baseX - step*24), baseY},
								{SnapToGrid(baseX + step*24), baseY},
								{baseX, SnapToGrid(baseY - step*24)},
							}
							for _, cand := range candidates {
								if !collides(cand[0], cand[1], sh, enforceCh) {
									sc.X = cand[0]
									sc.Y = cand[1]
									found = true
									break
								}
							}
						}
						if found {
							break
						}
					}
				}
				placed = append(placed, sc)
				recomputeBounds(g)
			}
		}
		recomputeBounds(g)
	}

	// 2. Ensure chapter frames without manual anchors do not overlap earlier chapters
	chaptersOverlap := func(a, b *chGroup, gap int) bool {
		return a.left < b.right+gap &&
			a.right+gap > b.left &&
			a.top < b.bottom+gap &&
			a.bottom+gap > b.top
	}

	for i := 1; i < len(groups); i++ {
		cur := groups[i]
		if cur.anyManual {
			continue
		}
		for pass := 0; pass < 16; pass++ {
			overlapped := false
			for j := 0; j < i; j++ {
				prev := groups[j]
				if !chaptersOverlap(cur, prev, 24) {
					continue
				}
				overlapped = true
				pushRight := SnapToGrid(prev.right + chGap - cur.left)
				pushLeft := SnapToGrid(prev.left - chGap - cur.right)
				pushDown := SnapToGrid(prev.bottom + chGap - cur.top)

				dx, dy := 0, pushDown
				if absInt(cur.top-prev.top) < (prev.bottom-prev.top)/2 {
					if cur.left >= prev.left && pushRight > 0 && pushRight < pushDown {
						dx, dy = pushRight, 0
					} else if cur.left < prev.left && -pushLeft > 0 && -pushLeft < pushDown {
						dx, dy = pushLeft, 0
					}
				}
				if dx == 0 && dy <= 0 {
					dy = EditorGridSize
				}
				for _, sc := range cur.scenes {
					sc.X = SnapToGrid(sc.X + dx)
					sc.Y = SnapToGrid(sc.Y + dy)
				}
				recomputeBounds(cur)
			}
			if !overlapped {
				break
			}
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
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
	if sc.Name != "" {
		if pos, ok := layout[NormalizeLookupKey(sc.Name)]; ok {
			return pos, true
		}
		if pos, ok := layout[sc.Name]; ok {
			return pos, true
		}
	}
	if sc.ID != "" {
		if pos, ok := layout[strings.ToLower(sc.ID)]; ok {
			return pos, true
		}
		if pos, ok := layout[sc.ID]; ok {
			return pos, true
		}
	}
	return NodePosition{}, false
}

// ParseEditorFile parses editor.txt containing Node Editor canvas viewport, node (x, y) coordinates, and wire control pins.
func ParseEditorFile(raw string, b *Book) {
	if b.Layout == nil {
		b.Layout = make(map[string]NodePosition)
	}
	if b.WirePins == nil {
		b.WirePins = make(map[string][]WirePin)
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
				b.EditorPanX = clampCanvasCoord(v)
				b.HasEditorView = true
			}
		case "pan_y":
			if v, err := strconv.Atoi(val); err == nil {
				b.EditorPanY = clampCanvasCoord(v)
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
				pos := NodePosition{X: clampCanvasCoord(x), Y: clampCanvasCoord(y), HasXY: true}
				b.Layout[nodeKey] = pos
				b.Layout[strings.ToLower(nodeKey)] = pos
				b.Layout[NormalizeLookupKey(nodeKey)] = pos
			}
		case "wire":
			arrowIdx := strings.LastIndex(val, "->")
			if arrowIdx <= 0 {
				continue
			}
			wireKey := strings.ToLower(strings.TrimSpace(val[:arrowIdx]))
			pinsRaw := strings.TrimSpace(val[arrowIdx+2:])
			if wireKey == "" || pinsRaw == "" {
				continue
			}
			pins := ParseWirePinList(pinsRaw)
			if len(pins) > 0 {
				b.WirePins[wireKey] = pins
			}
		}
	}
}

// ParseWirePinList parses a semicolon-separated list of "x,y" pin coordinates.
func ParseWirePinList(raw string) []WirePin {
	var pins []WirePin
	for _, seg := range strings.Split(raw, ";") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		parts := strings.Split(seg, ",")
		if len(parts) != 2 {
			continue
		}
		x, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
		y, errY := strconv.Atoi(strings.TrimSpace(parts[1]))
		if errX == nil && errY == nil {
			pins = append(pins, WirePin{X: clampCanvasCoord(x), Y: clampCanvasCoord(y)})
		}
	}
	return pins
}

// SerializeEditorFile serializes Node Editor canvas viewport, node (x, y) positions, and wire pins into editor.txt format.
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
	if len(b.WirePins) > 0 {
		keys := make([]string, 0, len(b.WirePins))
		for k, pins := range b.WirePins {
			if k != "" && len(pins) > 0 {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			pins := b.WirePins[k]
			var parts []string
			for _, p := range pins {
				parts = append(parts, fmt.Sprintf("%d,%d", p.X, p.Y))
			}
			fmt.Fprintf(&sb, "wire: %s -> %s\n", k, strings.Join(parts, ";"))
		}
	}
	return sb.String()
}
