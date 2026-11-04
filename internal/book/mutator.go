package book

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image/color"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func writeFileIfChanged(path string, data []byte, perm os.FileMode) error {
	if linfo, err := os.Lstat(path); err == nil && linfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("odmowa zapisu przez dowiązanie symboliczne (symlink): %s", path)
	}
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	return os.WriteFile(path, data, perm)
}

// SaveBookMetadataToDisk writes data.txt for an unpacked book.
func SaveBookMetadataToDisk(b *Book) error {
	if b.IsPacked {
		return fmt.Errorf("nie można edytować spakowanego archiwum .stbook bezpośrednio")
	}
	dataPath := filepath.Join(b.RootPath, "data.txt")
	return writeFileIfChanged(dataPath, []byte(SerializeDataFile(b)), 0644)
}

// SaveBookScenesToDisk writes all modified source .txt files, data.txt, and editor.txt back to the book directory.
func SaveBookScenesToDisk(b *Book) error {
	if b.IsPacked {
		return fmt.Errorf("nie można edytować spakowanego archiwum .stbook bezpośrednio; rozpakuj książkę do katalogu books/%s", b.ID)
	}
	if err := os.MkdirAll(b.RootPath, 0755); err != nil {
		return err
	}

	_ = SaveBookMetadataToDisk(b)

	filesSet := make(map[string]bool)
	trackedFiles := make(map[string]bool)
	activeFiles := make(map[string]bool)
	for _, tf := range b.TextFiles {
		filesSet[tf] = true
		trackedFiles[strings.ToLower(filepath.Base(tf))] = true
	}
	defaultCh := "Rozdział 1"
	defaultFile := "rozdzial_1.txt"
	if b.IsScreenplay() {
		defaultCh = "Akt I"
		defaultFile = "akt_1.txt"
	}
	for _, sc := range b.Scenes {
		if strings.TrimSpace(sc.Chapter) == "" {
			sc.Chapter = defaultCh
		}
		sf := b.ChapterFileNameFor(sc.Chapter, sc.ChapterID)
		sc.SourceFile = sf
		filesSet[sf] = true
		activeFiles[strings.ToLower(filepath.Base(sf))] = true
	}
	for i := range b.Chapters {
		ch := &b.Chapters[i]
		ch.SourceFile = ch.FileName()
	}
	if len(filesSet) == 0 {
		filesSet[defaultFile] = true
		activeFiles[defaultFile] = true
	}

	var updatedTextFiles []string
	for sf := range filesSet {
		cleanName := filepath.Base(sf)
		lowerClean := strings.ToLower(cleanName)
		if !strings.HasSuffix(lowerClean, ".txt") || lowerClean == "data.txt" || lowerClean == "editor.txt" || lowerClean == "nodes.txt" {
			cleanName = "rozdzial_1.txt"
			lowerClean = "rozdzial_1.txt"
		}
		if !activeFiles[lowerClean] {
			if trackedFiles[lowerClean] {
				_ = os.Remove(filepath.Join(b.RootPath, cleanName))
			}
			continue
		}
		serialized := SerializeScenesForFile(b, cleanName)
		outPath := filepath.Join(b.RootPath, cleanName)
		if strings.TrimSpace(serialized) == "" && len(activeFiles) > 0 && !activeFiles[lowerClean] {
			if trackedFiles[lowerClean] {
				_ = os.Remove(outPath)
			}
			continue
		}
		if err := writeFileIfChanged(outPath, []byte(serialized), 0644); err != nil {
			return err
		}
		updatedTextFiles = append(updatedTextFiles, cleanName)
	}
	sortStoryFiles(updatedTextFiles)
	b.TextFiles = updatedTextFiles

	_ = SaveBookEditorStateToDisk(b)
	return nil
}

// SaveBookEditorStateToDisk writes editor.txt inside the book directory with node (x, y) coordinates and canvas pan/zoom.
func SaveBookEditorStateToDisk(b *Book) error {
	if b == nil || b.IsPacked || b.RootPath == "" {
		return nil
	}
	editorPath := filepath.Join(b.RootPath, "editor.txt")
	return writeFileIfChanged(editorPath, []byte(SerializeEditorFile(b)), 0644)
}

// NextUniqueSceneName returns a default scene title ("Nowa scena N") that does not collide with any existing scene in the book.
func NextUniqueSceneName(b *Book) string {
	n := 1
	if b != nil {
		n = len(b.Scenes) + 1
	}
	for {
		cand := fmt.Sprintf("Nowa scena %d", n)
		normCand := NormalizeLookupKey(cand)
		taken := false
		if b != nil {
			for _, sc := range b.Scenes {
				if strings.EqualFold(strings.TrimSpace(sc.Name), cand) ||
					(normCand != "" && NormalizeLookupKey(sc.Name) == normCand) ||
					strings.EqualFold(sc.ExplicitID, cand) ||
					strings.EqualFold(sc.ID, cand) {
					taken = true
					break
				}
			}
		}
		if !taken {
			return cand
		}
		n++
	}
}

// InsertSceneBetween inserts a new scene into the book between parentSceneID and childSceneID
// (or after parentSceneID if childSceneID is empty), updates links, and writes back to the .txt file.
func InsertSceneBetween(b *Book, parentSceneID, childSceneID, newSceneName string) (*Scene, error) {
	if b.IsPacked {
		return nil, fmt.Errorf("książka jest spakowana (.stbook)")
	}
	cleanName := SanitizePlainText(newSceneName)
	if cleanName == "" {
		cleanName = NextUniqueSceneName(b)
	}

	var parent *Scene
	parentIdx := len(b.Scenes) - 1
	for i, sc := range b.Scenes {
		if sc.ID == parentSceneID {
			parent = sc
			parentIdx = i
			break
		}
	}

	chapterName := "Rozdział 1"
	chapterID := ""
	sourceFile := "rozdzial_1.txt"
	if parent != nil {
		if parent.Chapter != "" {
			chapterName = parent.Chapter
		}
		if parent.ChapterID != "" {
			chapterID = parent.ChapterID
		}
		if parent.SourceFile != "" && !strings.EqualFold(parent.SourceFile, "story.txt") {
			sourceFile = parent.SourceFile
		} else {
			sourceFile = b.ChapterFileNameFor(chapterName, chapterID)
		}
	} else if len(b.Chapters) > 0 {
		lastCh := b.Chapters[len(b.Chapters)-1]
		chapterName = lastCh.Name
		chapterID = lastCh.ExplicitID
		sourceFile = lastCh.FileName()
	}

	newScene := &Scene{
		Name:       cleanName,
		Chapter:    chapterName,
		ChapterID:  chapterID,
		Text:       "Nowy fragment opowieści...",
		SourceFile: sourceFile,
	}
	if b.IsScreenplay() {
		newScene.Slugline = DefaultSluglineForStandard(b.NormalizedStandard())
	}

	if parent != nil {
		relinked := false
		for i := range parent.Choices {
			ch := &parent.Choices[i]
			if childSceneID != "" && ch.TargetSceneID == childSceneID {
				var childName string
				for _, sc := range b.Scenes {
					if sc.ID == childSceneID {
						if sc.ExplicitID != "" {
							childName = sc.ExplicitID
						} else {
							childName = sc.Name
						}
						break
					}
				}
				if ch.IsImplicit {
					ch.IsImplicit = false
					ch.Label = "Dalej"
				}
				ch.Target = newScene.Name
				if childName != "" {
					newScene.Choices = append(newScene.Choices, Choice{
						Label:  "Dalej",
						Target: childName,
					})
				}
				relinked = true
				break
			}
		}
		if !relinked {
			if len(parent.Choices) == 1 && parent.Choices[0].IsImplicit {
				oldTargetID := parent.Choices[0].TargetSceneID
				parent.Choices = []Choice{{
					Label:  "Dalej",
					Target: newScene.Name,
				}}
				for _, sc := range b.Scenes {
					if sc.ID == oldTargetID {
						targetRef := sc.Name
						if sc.ExplicitID != "" {
							targetRef = sc.ExplicitID
						}
						newScene.Choices = []Choice{{
							Label:  "Dalej",
							Target: targetRef,
						}}
						break
					}
				}
			} else {
				parent.ExplicitEnd = false
				parent.Choices = append(parent.Choices, Choice{
					Label:  "Przejdź dalej",
					Target: newScene.Name,
				})
			}
		}
	}

	insertPos := parentIdx + 1
	if insertPos < 0 || insertPos > len(b.Scenes) {
		insertPos = len(b.Scenes)
	}
	b.Scenes = append(b.Scenes[:insertPos], append([]*Scene{newScene}, b.Scenes[insertPos:]...)...)

	FinalizeBook(b)
	StoreSceneNodePosition(b, newScene, NodePosition{Row: newScene.Row, Col: newScene.Col, X: newScene.X, Y: newScene.Y, HasXY: true})
	if err := SaveBookScenesToDisk(b); err != nil {
		return nil, err
	}
	return newScene, nil
}

// InsertSceneBefore inserts a new scene directly before targetSceneID, relinking incoming transitions
// (or updating the book start scene if targetSceneID was the root scene) and writing back to disk.
func InsertSceneBefore(b *Book, targetSceneID, newSceneName string) (*Scene, error) {
	if b.IsPacked {
		return nil, fmt.Errorf("książka jest spakowana (.stbook)")
	}
	cleanName := SanitizePlainText(newSceneName)
	if cleanName == "" {
		cleanName = NextUniqueSceneName(b)
	}

	var target *Scene
	targetIdx := 0
	for i, sc := range b.Scenes {
		if sc.ID == targetSceneID {
			target = sc
			targetIdx = i
			break
		}
	}
	if target == nil && len(b.Scenes) > 0 {
		target = b.Scenes[0]
		targetIdx = 0
	}

	chapterName := "Rozdział 1"
	chapterID := ""
	sourceFile := "rozdzial_1.txt"
	targetRef := ""
	if target != nil {
		if target.Chapter != "" {
			chapterName = target.Chapter
		}
		if target.ChapterID != "" {
			chapterID = target.ChapterID
		}
		if target.SourceFile != "" && !strings.EqualFold(target.SourceFile, "story.txt") {
			sourceFile = target.SourceFile
		} else {
			sourceFile = b.ChapterFileNameFor(chapterName, chapterID)
		}
		if target.ExplicitID != "" {
			targetRef = target.ExplicitID
		} else {
			targetRef = target.Name
		}
	}

	newScene := &Scene{
		Name:       cleanName,
		Chapter:    chapterName,
		ChapterID:  chapterID,
		Text:       "Nowy fragment opowieści...",
		SourceFile: sourceFile,
	}
	if b.IsScreenplay() {
		newScene.Slugline = DefaultSluglineForStandard(b.NormalizedStandard())
	}
	if targetRef != "" {
		newScene.Choices = []Choice{
			{
				Label:  "Dalej",
				Target: targetRef,
			},
		}
	}

	relinkedCount := 0
	if target != nil {
		for _, sc := range b.Scenes {
			for i := range sc.Choices {
				ch := &sc.Choices[i]
				if ch.TargetSceneID == target.ID {
					if ch.IsImplicit {
						ch.IsImplicit = false
						ch.Label = "Dalej"
					}
					ch.Target = newScene.Name
					relinkedCount++
				}
			}
		}
		if relinkedCount == 0 && (b.StartSceneID == target.ID || targetIdx == 0) {
			b.StartSceneID = newScene.Name
			b.RawStart = newScene.Name
		}
	}

	b.Scenes = append(b.Scenes[:targetIdx], append([]*Scene{newScene}, b.Scenes[targetIdx:]...)...)

	FinalizeBook(b)
	StoreSceneNodePosition(b, newScene, NodePosition{Row: newScene.Row, Col: newScene.Col, X: newScene.X, Y: newScene.Y, HasXY: true})
	if err := SaveBookScenesToDisk(b); err != nil {
		return nil, err
	}
	return newScene, nil
}

// CreateStandaloneScene appends a new unconnected scene to the book and optionally sets its (x, y) node coordinates.
func CreateStandaloneScene(b *Book, newSceneName string, x, y int, hasCoords bool) (*Scene, error) {
	return CreateStandaloneSceneInChapter(b, newSceneName, "", x, y, hasCoords)
}

// CreateStandaloneSceneInChapter appends a new unconnected scene in the specified chapter (or last chapter if empty)
// without any artificial coordinate bounds.
func CreateStandaloneSceneInChapter(b *Book, newSceneName, targetChapter string, x, y int, hasCoords bool) (*Scene, error) {
	if b.IsPacked {
		return nil, fmt.Errorf("książka jest spakowana (.stbook)")
	}
	cleanName := SanitizePlainText(newSceneName)
	if cleanName == "" {
		cleanName = NextUniqueSceneName(b)
	}

	chapterName := SanitizePlainText(targetChapter)
	chapterID := ""
	sourceFile := "rozdzial_1.txt"
	insertIdx := len(b.Scenes)
	matchedChapter := false

	if len(b.Scenes) > 0 {
		last := b.Scenes[len(b.Scenes)-1]
		if chapterName == "" {
			if hasCoords {
				px := x + 96
				py := y + 48
				type chBounds struct {
					name    string
					id      string
					minX    int
					minY    int
					maxX    int
					maxY    int
					hasNode bool
				}
				var groups []*chBounds
				byCh := make(map[string]*chBounds)
				for _, sc := range b.Scenes {
					chN := strings.TrimSpace(sc.Chapter)
					if chN == "" {
						chN = "Rozdział 1"
					}
					k := strings.ToLower(chN)
					g, ok := byCh[k]
					if !ok {
						g = &chBounds{name: chN, id: sc.ChapterID}
						byCh[k] = g
						groups = append(groups, g)
					}
					if g.id == "" && sc.ChapterID != "" {
						g.id = sc.ChapterID
					}
					sx, sy := sc.X, sc.Y
					sw := 192
					sh := sceneCanvasHeight(sc)
					if sh < 120 {
						sh = 120
					}
					if !g.hasNode {
						g.minX, g.minY = sx-36, sy-52
						g.maxX, g.maxY = sx+sw+36, sy+sh+44
						g.hasNode = true
					} else {
						if sx-36 < g.minX {
							g.minX = sx - 36
						}
						if sy-52 < g.minY {
							g.minY = sy - 52
						}
						if sx+sw+36 > g.maxX {
							g.maxX = sx + sw + 36
						}
						if sy+sh+44 > g.maxY {
							g.maxY = sy + sh + 44
						}
					}
				}
				bestDistSq := int(^uint(0) >> 1)
				for _, g := range groups {
					dx := 0
					if px < g.minX {
						dx = g.minX - px
					} else if px > g.maxX {
						dx = px - g.maxX
					}
					dy := 0
					if py < g.minY {
						dy = g.minY - py
					} else if py > g.maxY {
						dy = py - g.maxY
					}
					dSq := dx*dx + dy*dy
					if dSq < bestDistSq {
						bestDistSq = dSq
						chapterName = g.name
						chapterID = g.id
					}
				}
			}
			if chapterName == "" && last.Chapter != "" {
				chapterName = last.Chapter
				chapterID = last.ChapterID
			}
			for i, sc := range b.Scenes {
				if strings.EqualFold(strings.TrimSpace(sc.Chapter), chapterName) {
					insertIdx = i + 1
					matchedChapter = true
					if chapterID == "" && sc.ChapterID != "" {
						chapterID = sc.ChapterID
					}
				}
			}
			sourceFile = b.ChapterFileNameFor(chapterName, chapterID)
		} else {
			for i, sc := range b.Scenes {
				if strings.EqualFold(strings.TrimSpace(sc.Chapter), chapterName) {
					insertIdx = i + 1
					matchedChapter = true
					if sc.ChapterID != "" {
						chapterID = sc.ChapterID
					}
					if sc.SourceFile != "" && !strings.EqualFold(sc.SourceFile, "story.txt") {
						sourceFile = sc.SourceFile
					}
				}
			}
			if !matchedChapter {
				chapterID = SanitizeChapterID(chapterName)
				sourceFile = b.ChapterFileNameFor(chapterName, chapterID)
			} else if sourceFile == "" {
				sourceFile = b.ChapterFileNameFor(chapterName, chapterID)
			}
		}
	}
	if chapterName == "" {
		if b.IsScreenplay() {
			chapterName = "Akt I"
			chapterID = "akt_1"
			sourceFile = "akt_1.txt"
		} else {
			chapterName = "Rozdział 1"
			chapterID = "rozdzial_1"
			sourceFile = "rozdzial_1.txt"
		}
	}

	if hasCoords {
		x, y = resolveNonCollidingStandaloneCoords(b, chapterName, x, y)
	}

	newScene := &Scene{
		Name:        cleanName,
		Chapter:     chapterName,
		ChapterID:   chapterID,
		Text:        "Nowy fragment opowieści...",
		SourceFile:  sourceFile,
		ExplicitEnd: true,
	}
	if b.IsScreenplay() {
		newScene.Slugline = DefaultSluglineForStandard(b.NormalizedStandard())
	}

	if insertIdx > 0 && insertIdx <= len(b.Scenes) {
		prev := b.Scenes[insertIdx-1]
		if len(prev.Choices) == 1 && prev.Choices[0].IsImplicit && (strings.EqualFold(prev.Choices[0].Target, "Next") || strings.EqualFold(prev.Choices[0].Target, "Dalej")) {
			if insertIdx < len(b.Scenes) {
				nextSc := b.Scenes[insertIdx]
				ref := nextSc.Name
				if nextSc.ExplicitID != "" {
					ref = nextSc.ExplicitID
				}
				prev.Choices[0].Target = ref
				prev.ExplicitNext = ref
			} else {
				prev.Choices = nil
				prev.ExplicitEnd = true
			}
		}
	}

	if insertIdx >= 0 && insertIdx < len(b.Scenes) {
		b.Scenes = append(b.Scenes[:insertIdx], append([]*Scene{newScene}, b.Scenes[insertIdx:]...)...)
	} else {
		b.Scenes = append(b.Scenes, newScene)
	}

	if b.Layout == nil {
		b.Layout = make(map[string]NodePosition)
	}
	if hasCoords {
		pos := NodePosition{X: x, Y: y, HasXY: true}
		StoreSceneNodePosition(b, newScene, pos)
	}

	FinalizeBook(b)
	if hasCoords {
		newScene.X = x
		newScene.Y = y
		pos := NodePosition{Row: newScene.Row, Col: newScene.Col, X: x, Y: y, HasXY: true}
		StoreSceneNodePosition(b, newScene, pos)
	}
	if err := SaveBookScenesToDisk(b); err != nil {
		return nil, err
	}
	return newScene, nil
}

func resolveNonCollidingStandaloneCoords(b *Book, targetChapter string, baseX, baseY int) (int, int) {
	if b == nil || len(b.Scenes) == 0 {
		return baseX, baseY
	}
	const (
		nodeW      = 192
		nodeH      = 74
		nodeMargin = 48
	)
	targetChKey := strings.ToLower(strings.TrimSpace(targetChapter))

	type chRect struct {
		key    string
		left   int
		top    int
		right  int
		bottom int
	}
	byCh := make(map[string]*chRect)
	var otherChapters []*chRect
	var targetRect *chRect

	for _, sc := range b.Scenes {
		chN := strings.TrimSpace(sc.Chapter)
		if chN == "" {
			chN = "Rozdział 1"
		}
		k := strings.ToLower(chN)
		sh := sceneCanvasHeight(sc)
		if sh < 120 {
			sh = 120
		}
		l := sc.X - 36
		t := sc.Y - 52
		r := sc.X + nodeW + 36
		bt := sc.Y + sh + 44
		rect, ok := byCh[k]
		if !ok {
			rect = &chRect{key: k, left: l, top: t, right: r, bottom: bt}
			byCh[k] = rect
			if k == targetChKey {
				targetRect = rect
			} else {
				otherChapters = append(otherChapters, rect)
			}
		} else {
			if l < rect.left {
				rect.left = l
			}
			if t < rect.top {
				rect.top = t
			}
			if r > rect.right {
				rect.right = r
			}
			if bt > rect.bottom {
				rect.bottom = bt
			}
		}
	}

	if len(otherChapters) == 0 {
		return baseX, baseY
	}

	collidesChapterAt := func(cx, cy int) bool {
		cLeft := cx - 36
		cTop := cy - 52
		cRight := cx + nodeW + 36
		cBottom := cy + 120 + 44
		if targetRect != nil {
			if targetRect.left < cLeft {
				cLeft = targetRect.left
			}
			if targetRect.top < cTop {
				cTop = targetRect.top
			}
			if targetRect.right > cRight {
				cRight = targetRect.right
			}
			if targetRect.bottom > cBottom {
				cBottom = targetRect.bottom
			}
		}
		for _, oc := range otherChapters {
			if cLeft < oc.right+24 &&
				cRight+24 > oc.left &&
				cTop < oc.bottom+24 &&
				cBottom+24 > oc.top {
				return true
			}
		}
		return false
	}

	if !collidesChapterAt(baseX, baseY) {
		return baseX, baseY
	}

	collidesAt := func(cx, cy int) bool {
		for _, sc := range b.Scenes {
			if nodesOverlapWithMargin(cx, cy, nodeH, sc.X, sc.Y, sceneCanvasHeight(sc), nodeMargin) {
				return true
			}
		}
		return collidesChapterAt(cx, cy)
	}
	for step := 1; step <= 48; step++ {
		stepPx := step * EditorGridSize
		candidates := [][2]int{
			{baseX, SnapToGrid(baseY + stepPx)},
			{SnapToGrid(baseX + stepPx), baseY},
			{SnapToGrid(baseX - stepPx), baseY},
			{baseX, SnapToGrid(baseY - stepPx)},
			{SnapToGrid(baseX + stepPx), SnapToGrid(baseY + stepPx)},
			{SnapToGrid(baseX - stepPx), SnapToGrid(baseY + stepPx)},
			{SnapToGrid(baseX - step*240), baseY},
			{SnapToGrid(baseX + step*240), baseY},
		}
		for _, cand := range candidates {
			if !collidesAt(cand[0], cand[1]) {
				return cand[0], cand[1]
			}
		}
	}
	return baseX, baseY
}

// CreateNewBook scaffolds a brand new book directory with data.txt, story.txt, cover.png, and optional narration/imageless mode.
func CreateNewBook(booksDir, title, author string, narrated bool, imageless ...bool) (*Book, error) {
	isImageless := len(imageless) > 0 && imageless[0]
	return CreateNewBookWithMode(booksDir, title, author, "book", "pl", narrated, isImageless)
}

// CreateNewBookWithMode scaffolds a brand new book or screenplay project directory.
func CreateNewBookWithMode(booksDir, title, author, mode, standard string, narrated, imageless bool) (*Book, error) {
	normMode := NormalizeBookMode(mode)
	normStd := NormalizeScriptStandard(standard)

	cleanTitle := SanitizePlainText(title)
	if cleanTitle == "" {
		if normMode == "screenplay" {
			cleanTitle = "Nowy Scenariusz"
		} else {
			cleanTitle = "Nowa Opowieść"
		}
	}
	cleanAuthor := SanitizePlainText(author)
	if cleanAuthor == "" {
		cleanAuthor = "Autor"
	}
	id := SanitizeBookID(cleanTitle)
	cleanBaseDir := filepath.Clean(strings.TrimSpace(booksDir))
	bookDir := filepath.Join(cleanBaseDir, id)
	if SanitizeBookID(filepath.Base(cleanBaseDir)) == id && !HasBookFilesInDir(cleanBaseDir) {
		if entries, err := os.ReadDir(cleanBaseDir); err == nil && len(entries) == 0 {
			bookDir = cleanBaseDir
		}
	}
	if bookDir != cleanBaseDir {
		if _, err := os.Stat(bookDir); err == nil {
			id = fmt.Sprintf("%s-%d", id, time.Now().Unix()%1000)
			bookDir = filepath.Join(cleanBaseDir, id)
		}
	}
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		return nil, err
	}
	if normMode == "screenplay" {
		narrated = false
		imageless = true
	} else {
		if err := os.MkdirAll(filepath.Join(bookDir, "images"), 0755); err != nil {
			return nil, err
		}
		if narrated {
			if err := os.MkdirAll(filepath.Join(bookDir, "voice"), 0755); err != nil {
				return nil, err
			}
		}
	}

	narratedStr := "false"
	if narrated {
		narratedStr = "true"
	}
	imagelessStr := "false"
	if imageless {
		imagelessStr = "true"
	}

	if normMode == "screenplay" {
		defaultSlug := DefaultSluglineForStandard(normStd)
		b := &Book{
			ID:             id,
			Title:          cleanTitle,
			Author:         cleanAuthor,
			Description:    "Scenariusz filmowy utworzony w Playbook Studio.",
			Mode:           "screenplay",
			ScriptStandard: normStd,
			StartSceneID:   "poczatek",
			RawStart:       "poczatek",
			Narrated:       narrated,
			Imageless:      true,
			RootPath:       bookDir,
			Chapters: []Chapter{
				{ID: "akt_1", ExplicitID: "akt_1", Name: "Akt I", SourceFile: "akt_1.txt"},
				{ID: "akt_2", ExplicitID: "akt_2", Name: "Akt II", SourceFile: "akt_2.txt"},
				{ID: "akt_3", ExplicitID: "akt_3", Name: "Akt III", SourceFile: "akt_3.txt"},
			},
			Scenes: []*Scene{
				{
					ID:           "poczatek",
					ExplicitID:   "poczatek",
					Name:         "Scena 1",
					Slugline:     defaultSlug,
					Chapter:      "Akt I",
					ChapterID:    "akt_1",
					ExplicitNext: "chapter:akt_2",
					Text:         "Opis akcji w pierwszym akcie.",
					SourceFile:   "akt_1.txt",
					Choices:      []Choice{{Label: "Dalej", Target: "chapter:akt_2", IsImplicit: true}},
				},
				{
					ID:           "rozdroze",
					ExplicitID:   "rozdroze",
					Name:         "Scena 2",
					Slugline:     defaultSlug,
					Chapter:      "Akt II",
					ChapterID:    "akt_2",
					ExplicitNext: "chapter:akt_3",
					Text:         "Opis akcji w drugim akcie.",
					SourceFile:   "akt_2.txt",
					Choices:      []Choice{{Label: "Dalej", Target: "chapter:akt_3", IsImplicit: true}},
				},
				{
					ID:          "final",
					ExplicitID:  "final",
					Name:        "Scena 3",
					Slugline:    defaultSlug,
					Chapter:     "Akt III",
					ChapterID:   "akt_3",
					ExplicitEnd: true,
					Text:        "Opis akcji w trzecim akcie.",
					SourceFile:  "akt_3.txt",
				},
			},
		}
		FinalizeBook(b)
		AutoLayoutBook(b)
		if err := SaveBookScenesToDisk(b); err != nil {
			return nil, err
		}
		_ = os.Remove(filepath.Join(bookDir, "editor.txt"))
		_ = GenerateAtmosphericPNG(filepath.Join(bookDir, "cover.png"), 400, 560, color.RGBA{24, 30, 44, 255}, color.RGBA{214, 158, 76, 255}, 1)
		return LoadBookFromDir(bookDir)
	}

	dataContent := fmt.Sprintf("title: %s\nauthor: %s\ndescription: Interaktywna opowieść utworzona w Playbook Studio.\nnarrated: %s\nimageless: %s\nstart: poczatek\n", cleanTitle, cleanAuthor, narratedStr, imagelessStr)
	if err := os.WriteFile(filepath.Join(bookDir, "data.txt"), []byte(dataContent), 0644); err != nil {
		return nil, err
	}

	storyContent := `chapter: Rozdział 1
id: rozdzial_1

scene: Początek drogi
id: poczatek
image: images/poczatek.png
next: rozdroze
Stoisz u progu nowej przygody. Wiatr niesie odległe echo dawnej pieśni.

scene: Rozdroże
id: rozdroze
end: true
Dotarłeś do celu podróży. Możesz rozbudować tę historię w Playbook Studio.
`
	if err := os.WriteFile(filepath.Join(bookDir, "rozdzial_1.txt"), []byte(storyContent), 0644); err != nil {
		return nil, err
	}

	_ = GenerateAtmosphericPNG(filepath.Join(bookDir, "cover.png"), 400, 560, color.RGBA{24, 30, 44, 255}, color.RGBA{214, 158, 76, 255}, 1)
	_ = GenerateAtmosphericPNG(filepath.Join(bookDir, "images", "poczatek.png"), 640, 360, color.RGBA{20, 32, 38, 255}, color.RGBA{120, 180, 165, 255}, 2)

	return LoadBookFromDir(bookDir)
}

// ImportBookMediaFromDisk validates and copies one or more image (.png, .jpg, .jpeg, .webp)
// or audio (.ogg, .flac, .mp3) files from disk into the unpacked book's images/ or voice/ subdirectory,
// returning the relative book paths (e.g. "images/foo.png" or "voice/bar.ogg").
func ImportBookMediaFromDisk(b *Book, rawPaths []string, kind string) ([]string, error) {
	if b == nil {
		return nil, fmt.Errorf("brak aktywnej książki")
	}
	if b.IsPacked {
		return nil, fmt.Errorf("nie można dodawać plików multimedialnych do spakowanego archiwum .stbook")
	}
	if strings.TrimSpace(b.RootPath) == "" {
		return nil, fmt.Errorf("brak ścieżki katalogu książki na dysku")
	}

	normKind := strings.ToLower(strings.TrimSpace(kind))
	if !b.Narrated && (normKind == "audio" || normKind == "voice") {
		return nil, fmt.Errorf("książka nie posiada narracji (narrated: false) — nagrania głosowe są wyłączone")
	}
	var candidateFiles []string

	for _, rawEntry := range rawPaths {
		for _, part := range strings.Split(strings.ReplaceAll(rawEntry, "\r\n", "\n"), "\n") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			absPath, err := ExpandLocalPath(part)
			if err != nil {
				return nil, err
			}
			info, err := os.Stat(absPath)
			if err != nil {
				return nil, fmt.Errorf("nie znaleziono pliku na dysku: %s", absPath)
			}
			if info.IsDir() {
				entries, rErr := os.ReadDir(absPath)
				if rErr != nil {
					return nil, rErr
				}
				for _, e := range entries {
					if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
						continue
					}
					childPath := filepath.Join(absPath, e.Name())
					switch normKind {
					case "image", "img":
						if isRasterImageFilename(childPath) {
							candidateFiles = append(candidateFiles, childPath)
						}
					case "audio", "voice":
						if b.Narrated && isAudioFilename(childPath) {
							candidateFiles = append(candidateFiles, childPath)
						}
					default:
						if isRasterImageFilename(childPath) || (b.Narrated && isAudioFilename(childPath)) {
							candidateFiles = append(candidateFiles, childPath)
						}
					}
				}
				continue
			}
			candidateFiles = append(candidateFiles, absPath)
		}
	}

	if len(candidateFiles) == 0 {
		return nil, fmt.Errorf("nie wybrano żadnych obsługiwanych plików multimedialnych")
	}

	cleanRoot := canonicalPathKey(b.RootPath)
	var imported []string

	for _, absPath := range candidateFiles {
		ext := strings.ToLower(filepath.Ext(absPath))
		if ext == ".wav" {
			return nil, fmt.Errorf("format .wav nie jest obsługiwany (zbyt duży rozmiar) — użyj .ogg, .flac lub .mp3")
		}
		isImg := isRasterImageFilename(absPath)
		isAud := isAudioFilename(absPath)

		if isAud && !b.Narrated {
			return nil, fmt.Errorf("książka nie posiada narracji (narrated: false) — nagrania głosowe są wyłączone")
		}
		if (normKind == "image" || normKind == "img") && !isImg {
			return nil, fmt.Errorf("nieobsługiwany format obrazu „%s” (dozwolone: .png, .jpg, .jpeg, .webp)", filepath.Base(absPath))
		}
		if (normKind == "audio" || normKind == "voice") && !isAud {
			return nil, fmt.Errorf("nieobsługiwany format audio „%s” (dozwolone: .ogg, .flac, .mp3)", filepath.Base(absPath))
		}
		if !isImg && !isAud {
			return nil, fmt.Errorf("nieobsługiwany format pliku „%s” (obrazy: .png, .jpg, .webp; audio: .ogg, .flac, .mp3)", filepath.Base(absPath))
		}

		srcFile, err := os.Open(absPath)
		if err != nil {
			return nil, fmt.Errorf("nie można otworzyć pliku „%s”: %w", filepath.Base(absPath), err)
		}
		info, statErr := srcFile.Stat()
		if statErr != nil || info.IsDir() {
			_ = srcFile.Close()
			return nil, fmt.Errorf("nieprawidłowy plik multimedialny: %s", filepath.Base(absPath))
		}
		if info.Size() <= 0 {
			_ = srcFile.Close()
			return nil, fmt.Errorf("plik „%s” jest pusty", filepath.Base(absPath))
		}
		if info.Size() > maxMediaEntryBytes {
			_ = srcFile.Close()
			return nil, fmt.Errorf("plik „%s” przekracza dopuszczalny limit rozmiaru (100 MB)", filepath.Base(absPath))
		}

		var hdr [512]byte
		n, _ := io.ReadFull(srcFile, hdr[:])
		if !validateMediaHeader(absPath, hdr[:n]) {
			_ = srcFile.Close()
			return nil, fmt.Errorf("plik „%s” ma nieprawidłowy lub uszkodzony nagłówek formatu", filepath.Base(absPath))
		}

		// If the file is already inside the book's directory tree, reference it directly without copying over itself
		cleanSrc := canonicalPathKey(absPath)
		if strings.HasPrefix(cleanSrc, cleanRoot+string(os.PathSeparator)) {
			_ = srcFile.Close()
			if rel, rErr := filepath.Rel(cleanRoot, cleanSrc); rErr == nil {
				relSlash := SanitizeRelativeMediaPath(filepath.ToSlash(rel))
				if relSlash != "" {
					imported = append(imported, relSlash)
					continue
				}
			}
		}

		subDir := "voice"
		if isImg {
			subDir = "images"
		}
		baseName := filepath.Base(SanitizeRelativeMediaPath(filepath.Base(absPath)))
		baseName = strings.TrimLeft(baseName, ".")
		if baseName == "" || baseName == "." {
			_ = srcFile.Close()
			return nil, fmt.Errorf("nieprawidłowa nazwa pliku: %s", filepath.Base(absPath))
		}

		targetDir := filepath.Join(b.RootPath, subDir)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			_ = srcFile.Close()
			return nil, err
		}

		dstPath := filepath.Join(targetDir, baseName)
		if _, err := srcFile.Seek(0, io.SeekStart); err != nil {
			_ = srcFile.Close()
			return nil, err
		}

		tmpDst := dstPath + ".tmp"
		_ = os.Remove(tmpDst)
		outFile, err := os.OpenFile(tmpDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			_ = srcFile.Close()
			return nil, fmt.Errorf("błąd zapisu pliku multimedialnego „%s”: %w", baseName, err)
		}
		_, copyErr := io.Copy(outFile, io.LimitReader(srcFile, maxMediaEntryBytes+1))
		closeErr := outFile.Close()
		_ = srcFile.Close()
		if copyErr != nil || closeErr != nil {
			_ = os.Remove(tmpDst)
			if copyErr != nil {
				return nil, copyErr
			}
			return nil, closeErr
		}
		_ = os.Remove(dstPath)
		if err := os.Rename(tmpDst, dstPath); err != nil {
			_ = os.Remove(tmpDst)
			return nil, err
		}

		imported = append(imported, subDir+"/"+baseName)
	}

	return imported, nil
}

// PackBookToZip packages an unpacked book directory into a single .stbook archive (including subdirectories).
// It streams directly to an atomic .tmp file on disk, using zip.Deflate for .txt files and zip.Store for media files.
func PackBookToZip(bookDir, outZipPath string) error {
	if err := os.MkdirAll(filepath.Dir(outZipPath), 0755); err != nil {
		return err
	}
	tmpZipPath := outZipPath + ".tmp"
	_ = os.Remove(tmpZipPath)

	outFile, err := os.OpenFile(tmpZipPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	zw := zip.NewWriter(outFile)
	walkErr := filepath.WalkDir(bookDir, func(path string, d fs.DirEntry, wErr error) error {
		if wErr != nil {
			return wErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != bookDir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") || !IsSafePathInsideRoot(bookDir, path) {
			return nil
		}
		rel, err := filepath.Rel(bookDir, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		info, err := srcFile.Stat()
		if err != nil {
			_ = srcFile.Close()
			return err
		}

		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			_ = srcFile.Close()
			return err
		}
		hdr.Name = relSlash
		if isAudioFilename(relSlash) || isRasterImageFilename(relSlash) {
			hdr.Method = zip.Store
		} else {
			hdr.Method = zip.Deflate
		}

		w, err := zw.CreateHeader(hdr)
		if err != nil {
			_ = srcFile.Close()
			return err
		}
		_, copyErr := io.Copy(w, srcFile)
		closeErr := srcFile.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	zwCloseErr := zw.Close()
	fileCloseErr := outFile.Close()
	if walkErr != nil || zwCloseErr != nil || fileCloseErr != nil {
		_ = os.Remove(tmpZipPath)
		if walkErr != nil {
			return walkErr
		}
		if zwCloseErr != nil {
			return zwCloseErr
		}
		return fileCloseErr
	}
	_ = os.Remove(outZipPath)
	if err := os.Rename(tmpZipPath, outZipPath); err != nil {
		_ = os.Remove(tmpZipPath)
		return err
	}
	return nil
}

// RenameScreenplayCharacter updates all matching character dialogue cues across all scenes in the book.
func RenameScreenplayCharacter(b *Book, oldName, newName string) (int, error) {
	if b == nil {
		return 0, fmt.Errorf("brak aktywnej książki")
	}
	if b.IsPacked {
		return 0, fmt.Errorf("nie można edytować spakowanego archiwum .stbook bezpośrednio")
	}
	oldBase, _ := SplitCharacterCue(SanitizePlainText(oldName))
	newBase, _ := SplitCharacterCue(SanitizePlainText(newName))
	if oldBase == "" || newBase == "" {
		return 0, fmt.Errorf("podaj prawidłową obecną i nową nazwę postaci")
	}
	if strings.EqualFold(oldBase, newBase) {
		return 0, nil
	}

	updatedCount := 0
	for _, sc := range b.Scenes {
		lines := strings.Split(strings.ReplaceAll(sc.Text, "\r\n", "\n"), "\n")
		changed := false
		prevBlank := true
		for i := 0; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" {
				prevBlank = true
				continue
			}
			hasNext := i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != ""
			if _, baseName, ext, ok := isCharacterCueLine(trimmed, prevBlank, hasNext); ok && strings.EqualFold(baseName, oldBase) {
				prefix := ""
				if strings.HasPrefix(trimmed, "@") {
					prefix = "@"
				}
				if ext != "" {
					lines[i] = prefix + newBase + " " + ext
				} else {
					lines[i] = prefix + newBase
				}
				changed = true
				updatedCount++
			}
			prevBlank = false
		}
		if changed {
			sc.Text = strings.Join(lines, "\n")
			sc.HTML = ""
			sc.htmlSourceText = ""
		}
	}

	if updatedCount > 0 {
		FinalizeBook(b)
		if err := SaveBookScenesToDisk(b); err != nil {
			return updatedCount, err
		}
		_ = RecordBookHistoryStep(b, fmt.Sprintf("Zmiana postaci: %s → %s", oldBase, newBase), "rename_character", "", b.StartSceneID)
	}
	return updatedCount, nil
}

// RenameScreenplayLocation updates all matching slugline location segments across all scenes in the book.
func RenameScreenplayLocation(b *Book, oldLoc, newLoc string) (int, error) {
	if b == nil {
		return 0, fmt.Errorf("brak aktywnej książki")
	}
	if b.IsPacked {
		return 0, fmt.Errorf("nie można edytować spakowanego archiwum .stbook bezpośrednio")
	}
	targetLoc := strings.ToUpper(strings.TrimSpace(SanitizePlainText(oldLoc)))
	replacementLoc := strings.ToUpper(strings.TrimSpace(SanitizePlainText(newLoc)))
	if targetLoc == "" || replacementLoc == "" {
		return 0, fmt.Errorf("podaj prawidłową obecną i nową nazwę lokacji")
	}
	if strings.EqualFold(targetLoc, replacementLoc) {
		return 0, nil
	}

	rebuildSlugline := func(raw string) (string, bool) {
		info := ParseSlugline(raw)
		if !strings.EqualFold(info.Location, targetLoc) {
			return raw, false
		}
		prefix := info.IntExt
		if prefix == "" {
			prefix = DefaultSluglineForStandard(b.NormalizedStandard())
			if pInfo := ParseSlugline(prefix); pInfo.IntExt != "" {
				prefix = pInfo.IntExt
			}
		}
		if info.TimeOfDay != "" {
			return fmt.Sprintf("%s %s - %s", prefix, replacementLoc, info.TimeOfDay), true
		}
		return fmt.Sprintf("%s %s", prefix, replacementLoc), true
	}

	updatedCount := 0
	for _, sc := range b.Scenes {
		sceneChanged := false
		if strings.TrimSpace(sc.Slugline) != "" {
			if rebuilt, ok := rebuildSlugline(sc.Slugline); ok {
				sc.Slugline = rebuilt
				sceneChanged = true
			}
		}
		lines := strings.Split(strings.ReplaceAll(sc.Text, "\r\n", "\n"), "\n")
		textChanged := false
		prevBlank := true
		for i := 0; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" {
				prevBlank = true
				continue
			}
			if prevBlank && IsSluglineLine(trimmed) {
				if rebuilt, ok := rebuildSlugline(trimmed); ok {
					lines[i] = rebuilt
					textChanged = true
					sceneChanged = true
				}
			}
			prevBlank = false
		}
		if textChanged {
			sc.Text = strings.Join(lines, "\n")
			sc.HTML = ""
			sc.htmlSourceText = ""
		}
		if sceneChanged {
			updatedCount++
		}
	}

	if updatedCount > 0 {
		FinalizeBook(b)
		if err := SaveBookScenesToDisk(b); err != nil {
			return updatedCount, err
		}
		_ = RecordBookHistoryStep(b, fmt.Sprintf("Zmiana lokacji: %s → %s", targetLoc, replacementLoc), "rename_location", "", b.StartSceneID)
	}
	return updatedCount, nil
}

// MoveBookToDirectory moves an unpacked book folder to targetParentDir/<bookDirName> and reloads it.
func MoveBookToDirectory(b *Book, targetParentDir string) (*Book, error) {
	if b == nil {
		return nil, fmt.Errorf("brak aktywnej książki")
	}
	if b.IsPacked {
		return nil, fmt.Errorf("nie można przenieść spakowanego archiwum .stbook")
	}
	oldRoot := filepath.Clean(strings.TrimSpace(b.RootPath))
	if oldRoot == "" || oldRoot == "." {
		return nil, fmt.Errorf("brak ścieżki źródłowej projektu")
	}
	cleanParent := filepath.Clean(strings.TrimSpace(targetParentDir))
	if cleanParent == "" || cleanParent == "." {
		return nil, fmt.Errorf("wskaż prawidłowy folder docelowy na dysku")
	}
	if abs, err := filepath.Abs(cleanParent); err == nil {
		cleanParent = abs
	}
	if err := os.MkdirAll(cleanParent, 0755); err != nil {
		return nil, err
	}
	folderName := filepath.Base(oldRoot)
	if folderName == "" || folderName == "." || folderName == string(filepath.Separator) {
		folderName = SanitizeBookID(b.ID)
	}
	newRoot := filepath.Join(cleanParent, folderName)
	if filepath.Clean(newRoot) == filepath.Clean(oldRoot) {
		return b, nil
	}
	if _, err := os.Stat(newRoot); err == nil {
		for suffix := 2; ; suffix++ {
			cand := filepath.Join(cleanParent, fmt.Sprintf("%s-%d", folderName, suffix))
			if _, stErr := os.Stat(cand); os.IsNotExist(stErr) {
				newRoot = cand
				break
			}
		}
	}
	if err := os.Rename(oldRoot, newRoot); err != nil {
		if copyErr := copyBookDirectoryAll(oldRoot, newRoot); copyErr != nil {
			return nil, copyErr
		}
		_ = os.RemoveAll(oldRoot)
	}
	reloaded, err := LoadBookFromPath(newRoot)
	if err != nil {
		return nil, err
	}
	if b.ID != "" && reloaded.ID != b.ID {
		RebindBookID(reloaded, b.ID)
	}
	return reloaded, nil
}

func copyBookDirectoryAll(srcDir, dstDir string) error {
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path != srcDir && !IsSafePathInsideRoot(srcDir, path) {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		mode := os.FileMode(0644)
		if err == nil {
			mode = info.Mode().Perm()
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, raw, mode)
	})
}
