package book

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (b *Book) ensureChapter(name string) {
	b.ensureChapterWithMeta(name, "", "", "")
}

func (b *Book) ensureChapterWithMeta(name, fallbackID, explicitID, sourceFile string) {
	cleanName := strings.TrimSpace(name)
	cleanExp := SanitizeSceneID(explicitID)
	cleanID := cleanExp
	if cleanID == "" {
		cleanID = SanitizeSceneID(fallbackID)
	}
	if cleanID == "" {
		cleanID = SanitizeSceneID(cleanName)
	}
	for i := range b.Chapters {
		ch := &b.Chapters[i]
		if (cleanName != "" && strings.EqualFold(ch.Name, cleanName)) ||
			(cleanExp != "" && (strings.EqualFold(ch.ExplicitID, cleanExp) || strings.EqualFold(ch.ID, cleanExp) || strings.EqualFold(ch.Name, cleanExp))) {
			if cleanName != "" && (ch.Name == "" || strings.EqualFold(ch.Name, ch.ExplicitID) || strings.EqualFold(ch.Name, ch.ID)) {
				ch.Name = cleanName
			}
			if cleanExp != "" {
				ch.ExplicitID = cleanExp
				ch.ID = cleanExp
			} else if ch.ID == "" && cleanID != "" {
				ch.ID = cleanID
			}
			if sourceFile != "" && ch.SourceFile == "" {
				ch.SourceFile = sourceFile
			}
			return
		}
	}
	b.Chapters = append(b.Chapters, Chapter{
		ID:         cleanID,
		ExplicitID: cleanExp,
		Name:       cleanName,
		SourceFile: sourceFile,
	})
}

func (b *Book) addSceneToChapter(chapterName, sceneID string) {
	b.addSceneToChapterWithMeta(chapterName, "", "", sceneID)
}

func (b *Book) addSceneToChapterWithMeta(chapterName, explicitID, sourceFile, sceneID string) {
	cleanName := strings.TrimSpace(chapterName)
	cleanExp := SanitizeSceneID(explicitID)
	fallbackID := ""
	if sourceFile != "" && !strings.EqualFold(filepath.Base(sourceFile), "story.txt") {
		stem := strings.TrimSuffix(filepath.Base(sourceFile), filepath.Ext(sourceFile))
		fallbackID = SanitizeSceneID(stem)
	}
	cleanID := cleanExp
	if cleanID == "" {
		cleanID = fallbackID
	}
	if cleanID == "" {
		cleanID = SanitizeSceneID(cleanName)
	}

	for i := range b.Chapters {
		ch := &b.Chapters[i]
		if strings.EqualFold(ch.Name, cleanName) ||
			(cleanExp != "" && (strings.EqualFold(ch.ExplicitID, cleanExp) || strings.EqualFold(ch.ID, cleanExp))) {
			if cleanExp != "" && ch.ExplicitID == "" {
				ch.ExplicitID = cleanExp
				ch.ID = cleanExp
			}
			if ch.ID == "" && cleanID != "" {
				ch.ID = cleanID
			}
			if sourceFile != "" && ch.SourceFile == "" {
				ch.SourceFile = sourceFile
			}
			for _, existing := range ch.SceneIDs {
				if existing == sceneID {
					return
				}
			}
			ch.SceneIDs = append(ch.SceneIDs, sceneID)
			return
		}
	}
	b.Chapters = append(b.Chapters, Chapter{
		ID:         cleanID,
		ExplicitID: cleanExp,
		Name:       cleanName,
		SourceFile: sourceFile,
		SceneIDs:   []string{sceneID},
	})
}

// ParseChapterTarget checks whether a choice/next target string is a chapter transition
// (e.g. "chapter:rozdzial_2", "rozdział:Rozdział 2", "rozdzial:w_gore_rzeki") and returns the target chapter key.
func ParseChapterTarget(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	colonIdx := strings.IndexByte(trimmed, ':')
	if colonIdx <= 0 {
		return "", false
	}
	prefix := strings.ToLower(strings.TrimSpace(trimmed[:colonIdx]))
	switch prefix {
	case "chapter", "rozdzial", "rozdział", "act", "akt":
		val := strings.TrimSpace(trimmed[colonIdx+1:])
		return val, val != ""
	default:
		return "", false
	}
}

// FindChapter returns the Chapter matching the given name or ID, or nil if not found.
func (b *Book) FindChapter(nameOrID string) *Chapter {
	key := strings.TrimSpace(nameOrID)
	if chRef, ok := ParseChapterTarget(key); ok {
		key = chRef
	}
	if key == "" {
		return nil
	}
	idKey := SanitizeSceneID(key)
	chSlug := SanitizeChapterID(key)
	for i := range b.Chapters {
		ch := &b.Chapters[i]
		if strings.EqualFold(strings.TrimSpace(ch.Name), key) ||
			(idKey != "" && (strings.EqualFold(ch.ExplicitID, idKey) || strings.EqualFold(ch.ID, idKey) || strings.EqualFold(SanitizeSceneID(ch.Name), idKey))) ||
			(chSlug != "" && (strings.EqualFold(SanitizeChapterID(ch.ExplicitID), chSlug) || strings.EqualFold(SanitizeChapterID(ch.ID), chSlug) || strings.EqualFold(SanitizeChapterID(ch.Name), chSlug))) {
			return ch
		}
	}
	return nil
}

// CanLinkChapters checks whether a chapter transition from fromChapter to toChapter is valid
// under the one-way chapter DAG rules (no self-link, no link into the root start chapter, no cycle).
func (b *Book) CanLinkChapters(fromChapterNameOrID, toChapterNameOrID string) (bool, string) {
	if b == nil {
		return false, "Brak aktywnej książki"
	}
	fromCh := b.FindChapter(fromChapterNameOrID)
	toCh := b.FindChapter(toChapterNameOrID)
	if fromCh == nil || toCh == nil {
		return false, "Nie znaleziono docelowego rozdziału"
	}
	fromKey := strings.ToLower(strings.TrimSpace(fromCh.ID))
	toKey := strings.ToLower(strings.TrimSpace(toCh.ID))
	if fromKey == "" {
		fromKey = strings.ToLower(fromCh.ReferenceKey())
	}
	if toKey == "" {
		toKey = strings.ToLower(toCh.ReferenceKey())
	}
	if fromKey == toKey || strings.EqualFold(fromCh.Name, toCh.Name) {
		return false, "Przejście rozdziału nie może wskazywać na ten sam rozdział"
	}

	rootKey := b.rootChapterKey()
	if rootKey != "" && toKey == rootKey {
		return false, "Zmiana rozdziału jest jednokierunkowa — nie można wracać do pierwszego rozdziału"
	}

	edges := make(map[string][]string, len(b.Chapters))
	for _, sc := range b.Scenes {
		scCh := b.FindChapter(sc.Chapter)
		if scCh == nil {
			continue
		}
		srcKey := strings.ToLower(scCh.ReferenceKey())
		for _, ch := range sc.Choices {
			if ch.IsEnding || !ch.IsChapterLink || ch.TargetChapterID == "" {
				continue
			}
			if dstCh := b.FindChapter(ch.TargetChapterID); dstCh != nil {
				dstKey := strings.ToLower(dstCh.ReferenceKey())
				if srcKey != "" && dstKey != "" && srcKey != dstKey {
					edges[srcKey] = append(edges[srcKey], dstKey)
				}
			}
		}
	}

	if chapterCanReach(edges, toKey, fromKey) {
		return false, "Zmiana rozdziału jest jednokierunkowa — nie można wracać do wcześniejszego rozdziału"
	}
	return true, ""
}

func (b *Book) rootChapterKey() string {
	if b == nil || len(b.Scenes) == 0 {
		return ""
	}
	startID := b.StartSceneID
	for _, sc := range b.Scenes {
		if strings.EqualFold(sc.ID, startID) {
			if ch := b.FindChapter(sc.Chapter); ch != nil {
				return strings.ToLower(ch.ReferenceKey())
			}
			return strings.ToLower(SanitizeSceneID(sc.Chapter))
		}
	}
	if len(b.Chapters) > 0 {
		return strings.ToLower(b.Chapters[0].ReferenceKey())
	}
	return strings.ToLower(SanitizeSceneID(b.Scenes[0].Chapter))
}

func chapterCanReach(edges map[string][]string, startKey, targetKey string) bool {
	startKey = strings.ToLower(strings.TrimSpace(startKey))
	targetKey = strings.ToLower(strings.TrimSpace(targetKey))
	if startKey == "" || targetKey == "" {
		return false
	}
	if startKey == targetKey {
		return true
	}
	visited := map[string]bool{startKey: true}
	queue := []string{startKey}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, nxt := range edges[curr] {
			if nxt == targetKey {
				return true
			}
			if !visited[nxt] {
				visited[nxt] = true
				queue = append(queue, nxt)
			}
		}
	}
	return false
}

// IsMultiFileBook reports whether the book uses separate per-chapter .txt files rather than a single story.txt.
func (b *Book) IsMultiFileBook() bool {
	return true
}

// ChapterFileNameFor returns the target .txt filename for a chapter, using its ID primarily.
func (b *Book) ChapterFileNameFor(chapterName, chapterID string) string {
	if cleanID := SanitizeChapterID(chapterID); cleanID != "" {
		return cleanID + ".txt"
	}
	if ch := b.FindChapter(chapterName); ch != nil {
		return ch.FileName()
	}
	if cleanNameID := SanitizeChapterID(chapterName); cleanNameID != "" {
		return cleanNameID + ".txt"
	}
	return "rozdzial_1.txt"
}

// FinalizeBook resolves scene IDs (including explicit id: headers), implicit "Next" transitions,
// choice targets (by scene id or scene name), start scene, chapters, and timeline row/column positions.
func FinalizeBook(b *Book) {
	if len(b.Scenes) == 0 {
		return
	}

	// Preserve existing chapter metadata (ExplicitID, ID, SourceFile) before rebuilding chapters
	prevChapterMeta := make(map[string]Chapter, len(b.Chapters))
	for _, ch := range b.Chapters {
		if k := strings.ToLower(strings.TrimSpace(ch.Name)); k != "" {
			prevChapterMeta[k] = ch
		}
		if k := strings.ToLower(strings.TrimSpace(ch.ExplicitID)); k != "" {
			prevChapterMeta[k] = ch
		}
		if k := strings.ToLower(strings.TrimSpace(ch.ID)); k != "" {
			prevChapterMeta[k] = ch
		}
	}

	if !b.Narrated {
		b.AudioFiles = nil
		for _, sc := range b.Scenes {
			sc.ExplicitVoice = ""
			sc.AudioFile = ""
			sc.AudioURL = ""
		}
	}
	if b.IsScreenplay() {
		b.Imageless = true
		b.ImageFiles = nil
		for _, sc := range b.Scenes {
			sc.ExplicitImage = ""
			sc.ImageFile = ""
			sc.ImageURL = ""
		}
	}

	usedIDs := make(map[string]bool, len(b.Scenes))
	for _, sc := range b.Scenes {
		sc.Warnings = nil
		if sc.ExplicitID != "" {
			cleanID := SanitizeSceneID(sc.ExplicitID)
			sc.ExplicitID = cleanID
			if cleanID != "" {
				lowerID := strings.ToLower(cleanID)
				if usedIDs[lowerID] {
					b.Warnings = append(b.Warnings, fmt.Sprintf("Duplikat id sceny „%s” w scenie „%s”", cleanID, sc.DisplayName()))
				}
				usedIDs[lowerID] = true
				sc.ID = cleanID
			}
		}
	}

	defaultChName := "Rozdział 1"
	if b.IsScreenplay() {
		defaultChName = "Akt I"
	}

	nextAuto := 1
	for i, sc := range b.Scenes {
		if sc.ExplicitID == "" {
			if strings.TrimSpace(sc.Name) == "" {
				sc.Name = fmt.Sprintf("Scena %d", i+1)
			}
			for {
				cand := fmt.Sprintf("s%d", nextAuto)
				nextAuto++
				if !usedIDs[strings.ToLower(cand)] {
					sc.ID = cand
					usedIDs[strings.ToLower(cand)] = true
					break
				}
			}
		}
		sc.OrderIndex = i
		if sc.Chapter == "" {
			if sc.ChapterID != "" {
				sc.Chapter = sc.ChapterID
			} else if sc.SourceFile != "" && !strings.EqualFold(filepath.Base(sc.SourceFile), "story.txt") {
				stem := strings.TrimSuffix(filepath.Base(sc.SourceFile), filepath.Ext(sc.SourceFile))
				sc.Chapter = SanitizeSceneID(stem)
			} else {
				sc.Chapter = defaultChName
			}
		}
		if sc.ChapterID == "" {
			if meta, ok := prevChapterMeta[strings.ToLower(strings.TrimSpace(sc.Chapter))]; ok && meta.ExplicitID != "" {
				sc.ChapterID = meta.ExplicitID
			}
		}
		if b.IsScreenplay() && strings.TrimSpace(sc.Slugline) == "" && strings.TrimSpace(sc.Text) != "" {
			if extracted, rest, ok := ExtractLeadingSlugline(sc.Text); ok {
				sc.Slugline = extracted
				sc.Text = rest
			}
		}
		if !b.deferProseHTML {
			sc.EnsureHTML()
		} else if sc.HTML != "" && sc.htmlSourceText != sc.Text {
			sc.HTML = ""
			sc.htmlSourceText = ""
		}
	}

	sceneByID := make(map[string]*Scene, len(b.Scenes))
	sceneByName := make(map[string]*Scene, len(b.Scenes))
	for _, sc := range b.Scenes {
		sceneByID[strings.ToLower(sc.ID)] = sc
		if sc.ExplicitID != "" {
			sceneByID[strings.ToLower(sc.ExplicitID)] = sc
		}
		if norm := NormalizeLookupKey(sc.Name); norm != "" {
			if _, exists := sceneByName[norm]; !exists {
				sceneByName[norm] = sc
			}
		}
	}

	// Resolve StartSceneID (can match scene ID, explicit ID, or scene Name)
	startCandidate := strings.TrimSpace(b.RawStart)
	if startCandidate == "" {
		startCandidate = strings.TrimSpace(b.StartSceneID)
	}
	if startCandidate == "" {
		b.StartSceneID = b.Scenes[0].ID
	} else if sc, ok := sceneByID[strings.ToLower(startCandidate)]; ok {
		b.StartSceneID = sc.ID
	} else if sc, ok := sceneByName[NormalizeLookupKey(startCandidate)]; ok {
		b.StartSceneID = sc.ID
	} else {
		b.Warnings = append(b.Warnings, fmt.Sprintf("data.txt: scena startowa „%s” nie istnieje (użyto pierwszej sceny „%s”)", startCandidate, b.Scenes[0].DisplayName()))
		b.StartSceneID = b.Scenes[0].ID
	}

	// Build initial chapter index and assign each chapter's single entry point (StartSceneID)
	b.Chapters = nil
	for _, sc := range b.Scenes {
		expID := sc.ChapterID
		sf := sc.SourceFile
		chKey := strings.ToLower(strings.TrimSpace(sc.Chapter))
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
				matched := ""
				normStart := NormalizeLookupKey(expStart)
				for _, sid := range ch.SceneIDs {
					sc := sceneByID[strings.ToLower(sid)]
					if sc == nil {
						continue
					}
					if strings.EqualFold(sc.ID, expStart) ||
						(sc.ExplicitID != "" && strings.EqualFold(sc.ExplicitID, expStart)) ||
						(normStart != "" && NormalizeLookupKey(sc.Name) == normStart) {
						matched = sc.ID
						break
					}
				}
				if matched != "" {
					ch.StartSceneID = matched
				} else {
					b.Warnings = append(b.Warnings, fmt.Sprintf("Rozdział „%s”: scena początkowa „%s” nie istnieje w tym rozdziale", ch.DisplayName(), expStart))
				}
			}
		}
	}

	rootChKey := b.rootChapterKey()
	chapterEdges := make(map[string][]string, len(b.Chapters))
	tryLinkChapters := func(sc *Scene, ch *Choice, fromCh, targetCh *Chapter) bool {
		if targetCh == nil {
			return false
		}
		fromKey := ""
		if fromCh != nil {
			fromKey = strings.ToLower(fromCh.ReferenceKey())
		} else {
			fromKey = strings.ToLower(SanitizeSceneID(sc.Chapter))
		}
		toKey := strings.ToLower(targetCh.ReferenceKey())
		if fromKey == toKey || (fromCh != nil && strings.EqualFold(fromCh.Name, targetCh.Name)) {
			ch.TargetSceneID = ""
			ch.TargetChapterID = ""
			ch.IsChapterLink = false
			warnMsg := fmt.Sprintf("Scena „%s”: przejście rozdziału nie może wskazywać na ten sam rozdział „%s”", sc.DisplayName(), targetCh.DisplayName())
			sc.Warnings = append(sc.Warnings, warnMsg)
			b.Warnings = append(b.Warnings, warnMsg)
			return true
		}
		if (rootChKey != "" && toKey == rootChKey) || chapterCanReach(chapterEdges, toKey, fromKey) {
			ch.TargetSceneID = ""
			ch.TargetChapterID = ""
			ch.IsChapterLink = false
			warnMsg := fmt.Sprintf("Scena „%s”: przejście do rozdziału „%s” jest niedozwolone (zmiana rozdziału jest jednokierunkowa)", sc.DisplayName(), targetCh.DisplayName())
			sc.Warnings = append(sc.Warnings, warnMsg)
			b.Warnings = append(b.Warnings, warnMsg)
			return true
		}
		chapterEdges[fromKey] = append(chapterEdges[fromKey], toKey)
		ch.IsChapterLink = true
		ch.TargetChapterID = targetCh.ReferenceKey()
		ch.Target = "chapter:" + targetCh.ReferenceKey()
		ch.TargetSceneID = targetCh.StartSceneID
		if len(sc.Choices) == 1 && sc.ExplicitNext != "" {
			sc.ExplicitNext = ch.Target
		}
		return true
	}

	// Resolve choices and enforce that a single transition is always implicit "Dalej" (never a custom choice title)
	for i, sc := range b.Scenes {
		var keptChoices []Choice
		if !sc.ExplicitEnd {
			for _, ch := range sc.Choices {
				if !ch.IsImplicit || (!strings.EqualFold(ch.Target, "Next") && !strings.EqualFold(ch.Target, "Dalej") && strings.TrimSpace(ch.Target) != "") {
					keptChoices = append(keptChoices, ch)
				}
			}
		}
		sc.Choices = keptChoices

		sameChapterNext := i+1 < len(b.Scenes) &&
			strings.EqualFold(strings.TrimSpace(sc.Chapter), strings.TrimSpace(b.Scenes[i+1].Chapter)) &&
			(sc.SourceFile == "" || b.Scenes[i+1].SourceFile == "" || strings.EqualFold(sc.SourceFile, b.Scenes[i+1].SourceFile))
		if !sc.ExplicitEnd && len(sc.Choices) == 0 && sameChapterNext {
			sc.Choices = append(sc.Choices, Choice{
				Label:         "Dalej",
				Target:        "Next",
				TargetSceneID: b.Scenes[i+1].ID,
				IsImplicit:    true,
			})
		}

		if len(sc.Choices) == 1 {
			sc.Choices[0].Label = "Dalej"
			sc.Choices[0].IsImplicit = true
		} else if len(sc.Choices) >= 2 {
			for j := range sc.Choices {
				sc.Choices[j].IsImplicit = false
			}
		}

		fromCh := b.FindChapter(sc.Chapter)
		for j := range sc.Choices {
			ch := &sc.Choices[j]
			ch.IsChapterLink = false
			ch.TargetChapterID = ""
			tgt := strings.TrimSpace(ch.Target)
			if strings.EqualFold(tgt, "END") || strings.EqualFold(tgt, "KONIEC") {
				ch.IsEnding = true
				ch.TargetSceneID = "END"
				continue
			}
			if strings.EqualFold(tgt, "NEXT") || strings.EqualFold(tgt, "DALEJ") {
				if sameChapterNext {
					ch.TargetSceneID = b.Scenes[i+1].ID
				} else {
					ch.IsEnding = true
					ch.TargetSceneID = "END"
				}
				continue
			}
			if chRef, isChTarget := ParseChapterTarget(tgt); isChTarget {
				targetCh := b.FindChapter(chRef)
				if targetCh == nil {
					ch.TargetSceneID = ""
					warnMsg := fmt.Sprintf("Scena „%s”: wybór „%s” wskazuje na nieistniejący rozdział „%s”", sc.DisplayName(), ch.Label, chRef)
					sc.Warnings = append(sc.Warnings, warnMsg)
					b.Warnings = append(b.Warnings, warnMsg)
					continue
				}
				tryLinkChapters(sc, ch, fromCh, targetCh)
				continue
			}
			var targetSc *Scene
			if found, ok := sceneByID[strings.ToLower(tgt)]; ok {
				targetSc = found
			} else if found, ok := sceneByName[NormalizeLookupKey(tgt)]; ok {
				targetSc = found
			}
			if targetSc != nil {
				if strings.EqualFold(strings.TrimSpace(targetSc.Chapter), strings.TrimSpace(sc.Chapter)) {
					ch.TargetSceneID = targetSc.ID
					continue
				}
				targetCh := b.FindChapter(targetSc.Chapter)
				if targetCh != nil && strings.EqualFold(targetSc.ID, targetCh.StartSceneID) {
					tryLinkChapters(sc, ch, fromCh, targetCh)
					continue
				}
				ch.TargetSceneID = ""
				chDisplay := targetSc.Chapter
				if targetCh != nil {
					chDisplay = targetCh.DisplayName()
				}
				warnMsg := fmt.Sprintf("Scena „%s”: przejście między rozdziałami może prowadzić wyłącznie do początku rozdziału „%s” (nie do wewnętrznej sceny „%s”)", sc.DisplayName(), chDisplay, targetSc.DisplayName())
				sc.Warnings = append(sc.Warnings, warnMsg)
				b.Warnings = append(b.Warnings, warnMsg)
				continue
			}
			if targetCh := b.FindChapter(tgt); targetCh != nil {
				tryLinkChapters(sc, ch, fromCh, targetCh)
				continue
			}
			ch.TargetSceneID = ""
			warnMsg := fmt.Sprintf("Scena „%s”: wybór „%s” wskazuje na nieistniejącą scenę lub id „%s”", sc.DisplayName(), ch.Label, ch.Target)
			sc.Warnings = append(sc.Warnings, warnMsg)
			b.Warnings = append(b.Warnings, warnMsg)
		}
	}

	// Order chapters (and multi-file scene groups) by graph reachability starting from StartSceneID
	// so disk filename order is completely irrelevant.
	orderChaptersAndScenesByGraph(b, prevChapterMeta)

	for i, sc := range b.Scenes {
		sc.OrderIndex = i
		if !b.deferProseHTML {
			sc.EnsureHTML()
		}
	}
	if b.IsScreenplay() {
		ValidateScreenplayGuards(b)
	}

	ComputeTimelineLayout(b)
}
