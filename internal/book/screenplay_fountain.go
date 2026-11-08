package book

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// resolveChoiceTargetSceneID resolves the target scene ID for a choice, including chapter links.
func resolveChoiceTargetSceneID(b *Book, ch Choice) string {
	if ch.IsEnding || strings.EqualFold(ch.TargetSceneID, "END") || strings.EqualFold(ch.Target, "END") {
		return ""
	}
	if ch.TargetSceneID != "" {
		return ch.TargetSceneID
	}
	if ch.TargetChapterID != "" && b != nil {
		if targetCh := b.FindChapter(ch.TargetChapterID); targetCh != nil {
			return targetCh.StartSceneID
		}
	}
	if chRef, ok := ParseChapterTarget(ch.Target); ok && b != nil {
		if targetCh := b.FindChapter(chRef); targetCh != nil {
			return targetCh.StartSceneID
		}
	}
	return ""
}

// LinearizeScreenplayScenes returns scenes in deterministic primary-path order starting from b.StartSceneID.
// Only scenes along the active primary path from StartSceneID are included; secondary branches and unlinked nodes are omitted.
func LinearizeScreenplayScenes(b *Book) []*Scene {
	if b == nil || len(b.Scenes) == 0 {
		return nil
	}
	byID := make(map[string]*Scene, len(b.Scenes))
	for _, sc := range b.Scenes {
		byID[strings.ToLower(sc.ID)] = sc
	}

	visited := make(map[string]bool, len(b.Scenes))
	var out []*Scene

	curr := byID[strings.ToLower(b.StartSceneID)]
	if curr == nil {
		curr = b.Scenes[0]
	}
	for curr != nil && !visited[strings.ToLower(curr.ID)] {
		visited[strings.ToLower(curr.ID)] = true
		out = append(out, curr)
		var nextSc *Scene
		for _, ch := range curr.Choices {
			targetID := resolveChoiceTargetSceneID(b, ch)
			if targetID == "" {
				continue
			}
			if cand := byID[strings.ToLower(targetID)]; cand != nil && !visited[strings.ToLower(cand.ID)] {
				nextSc = cand
				break
			}
		}
		curr = nextSc
	}
	return out
}

// ComputeBranchRoles identifies scenes on parallel branches (diverging from any reachable fork before reconvergence).
// Returns roles[sceneID] = "primary" | "secondary" and parents[sceneID] = forkParentSceneID.
func ComputeBranchRoles(b *Book) (map[string]string, map[string]string) {
	roles := make(map[string]string)
	parents := make(map[string]string)
	if b == nil || len(b.Scenes) == 0 {
		return roles, parents
	}

	byID := make(map[string]*Scene, len(b.Scenes))
	for _, sc := range b.Scenes {
		byID[strings.ToLower(sc.ID)] = sc
	}

	validOutTargets := func(sc *Scene) []string {
		var targets []string
		seen := make(map[string]bool, len(sc.Choices))
		for _, ch := range sc.Choices {
			tID := resolveChoiceTargetSceneID(b, ch)
			if tID == "" {
				continue
			}
			cand := byID[strings.ToLower(tID)]
			if cand == nil || strings.EqualFold(cand.ID, sc.ID) {
				continue
			}
			lower := strings.ToLower(cand.ID)
			if !seen[lower] {
				seen[lower] = true
				targets = append(targets, cand.ID)
			}
		}
		return targets
	}

	reachableFrom := func(startID string, stopID string) map[string]bool {
		vis := make(map[string]bool, len(b.Scenes))
		startSc := byID[strings.ToLower(startID)]
		if startSc == nil {
			return vis
		}
		queue := []*Scene{startSc}
		vis[startSc.ID] = true
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			if stopID != "" && strings.EqualFold(curr.ID, stopID) {
				continue
			}
			for _, tID := range validOutTargets(curr) {
				nextSc := byID[strings.ToLower(tID)]
				if nextSc != nil && !vis[nextSc.ID] {
					vis[nextSc.ID] = true
					queue = append(queue, nextSc)
				}
			}
		}
		return vis
	}

	startID := b.StartSceneID
	if byID[strings.ToLower(startID)] == nil {
		startID = b.Scenes[0].ID
	}
	reachableAll := reachableFrom(startID, "")

	primaryScenes := LinearizeScreenplayScenes(b)
	onPrimary := make(map[string]bool, len(primaryScenes))
	for _, sc := range primaryScenes {
		onPrimary[sc.ID] = true
	}

	for _, forkSc := range b.Scenes {
		if !reachableAll[forkSc.ID] {
			continue
		}
		targets := validOutTargets(forkSc)
		if len(targets) < 2 {
			continue
		}
		branchReach := make([]map[string]bool, len(targets))
		for i, tID := range targets {
			branchReach[i] = reachableFrom(tID, forkSc.ID)
		}
		for _, candSc := range b.Scenes {
			if candSc.ID == forkSc.ID {
				continue
			}
			inCount := 0
			for i := range targets {
				if branchReach[i][candSc.ID] {
					inCount++
				}
			}
			// If reachable from at least one branch of forkSc, but not all branches of forkSc,
			// candSc is on a parallel branch of forkSc (not a reconvergence node).
			if inCount > 0 && inCount < len(targets) {
				if onPrimary[candSc.ID] {
					roles[candSc.ID] = "primary"
				} else if roles[candSc.ID] != "primary" {
					roles[candSc.ID] = "secondary"
				}
				if parents[candSc.ID] == "" {
					parents[candSc.ID] = forkSc.ID
				}
			}
		}
	}

	// Also detect parallel connected entry scenes within the same chapter (where both have outgoing transitions
	// and no intra-chapter incoming edges).
	intraIncoming := make(map[string]bool, len(b.Scenes))
	for _, sc := range b.Scenes {
		for _, tID := range validOutTargets(sc) {
			if targetSc := byID[strings.ToLower(tID)]; targetSc != nil {
				if strings.EqualFold(strings.TrimSpace(sc.Chapter), strings.TrimSpace(targetSc.Chapter)) {
					intraIncoming[targetSc.ID] = true
				}
			}
		}
	}
	for i := range b.Chapters {
		ch := &b.Chapters[i]
		var roots []*Scene
		for _, sc := range b.Scenes {
			if !strings.EqualFold(strings.TrimSpace(sc.Chapter), strings.TrimSpace(ch.Name)) {
				continue
			}
			if intraIncoming[sc.ID] {
				continue
			}
			if len(validOutTargets(sc)) == 0 && !sc.ExplicitEnd && sc.ID != ch.StartSceneID && sc.ID != b.StartSceneID {
				continue
			}
			roots = append(roots, sc)
		}
		if len(roots) >= 2 {
			for _, rSc := range roots {
				if onPrimary[rSc.ID] {
					roles[rSc.ID] = "primary"
				} else if roles[rSc.ID] != "primary" {
					roles[rSc.ID] = "secondary"
				}
				if parents[rSc.ID] == "" {
					parents[rSc.ID] = "chapter:" + ch.ReferenceKey()
				}
			}
		}
	}

	return roles, parents
}

// PromoteSceneToPrimaryPath reorders choices at fork scenes so the path from StartSceneID through targetSceneID becomes primary.
func PromoteSceneToPrimaryPath(b *Book, targetSceneID string) bool {
	if b == nil || len(b.Scenes) == 0 || strings.TrimSpace(targetSceneID) == "" {
		return false
	}
	byID := make(map[string]*Scene, len(b.Scenes))
	for _, sc := range b.Scenes {
		byID[strings.ToLower(sc.ID)] = sc
	}
	targetSc := byID[strings.ToLower(strings.TrimSpace(targetSceneID))]
	if targetSc == nil {
		return false
	}
	startSc := byID[strings.ToLower(b.StartSceneID)]
	if startSc == nil {
		startSc = b.Scenes[0]
	}

	// BFS from startSc to targetSc to find the directed path
	prevScene := make(map[string]*Scene, len(b.Scenes))
	visited := make(map[string]bool, len(b.Scenes))
	queue := []*Scene{startSc}
	visited[startSc.ID] = true
	found := startSc.ID == targetSc.ID

	for len(queue) > 0 && !found {
		curr := queue[0]
		queue = queue[1:]
		for _, ch := range curr.Choices {
			tID := resolveChoiceTargetSceneID(b, ch)
			if tID == "" {
				continue
			}
			nextSc := byID[strings.ToLower(tID)]
			if nextSc != nil && !visited[nextSc.ID] {
				visited[nextSc.ID] = true
				prevScene[nextSc.ID] = curr
				if nextSc.ID == targetSc.ID {
					found = true
					break
				}
				queue = append(queue, nextSc)
			}
		}
	}

	promoteChildOnParent := func(parentSc, childSc *Scene) bool {
		if parentSc == nil || childSc == nil || len(parentSc.Choices) < 2 {
			return false
		}
		matchIdx := -1
		for i, ch := range parentSc.Choices {
			tID := resolveChoiceTargetSceneID(b, ch)
			if strings.EqualFold(tID, childSc.ID) {
				matchIdx = i
				break
			}
		}
		if matchIdx <= 0 {
			return false
		}
		picked := parentSc.Choices[matchIdx]
		reordered := make([]Choice, 0, len(parentSc.Choices))
		reordered = append(reordered, picked)
		for i, ch := range parentSc.Choices {
			if i != matchIdx {
				reordered = append(reordered, ch)
			}
		}
		parentSc.Choices = reordered
		parentSc.ExplicitNext = ""
		parentSc.ExplicitEnd = false
		return true
	}

	changed := false
	if found {
		curr := targetSc
		for curr != nil && curr.ID != startSc.ID {
			parent := prevScene[curr.ID]
			if parent == nil {
				break
			}
			if promoteChildOnParent(parent, curr) {
				changed = true
			}
			curr = parent
		}
	} else {
		for _, sc := range b.Scenes {
			if promoteChildOnParent(sc, targetSc) {
				changed = true
				break
			}
		}
		if !changed {
			if ch := b.FindChapter(targetSc.Chapter); ch != nil {
				isRootCh := false
				for _, sc := range b.Scenes {
					if strings.EqualFold(strings.TrimSpace(sc.Chapter), strings.TrimSpace(ch.Name)) && strings.EqualFold(sc.ID, b.StartSceneID) {
						isRootCh = true
						break
					}
				}
				if !strings.EqualFold(ch.StartSceneID, targetSc.ID) {
					ch.StartSceneID = targetSc.ID
					ch.ExplicitStart = targetSc.ReferenceKey()
					if isRootCh {
						b.StartSceneID = targetSc.ID
						b.RawStart = targetSc.ReferenceKey()
					}
					changed = true
				}
			}
		}
	}

	if changed {
		FinalizeBook(b)
	}
	return changed
}

// ExportScreenplayFountain exports a Book into a standard UTF-8 .fountain screenplay document.
func ExportScreenplayFountain(b *Book) string {
	if b == nil {
		return ""
	}
	normStd := b.NormalizedStandard()
	numbered := normStd == "pl" || normStd == "eu"

	title := strings.TrimSpace(b.Title)
	if title == "" {
		title = "Bez tytułu"
	}
	author := strings.TrimSpace(b.Author)
	if author == "" {
		author = "Autor"
	}

	branchName := "main"
	if h, err := LoadBookHistory(b); err == nil && h != nil {
		branchName = h.NormalizedActiveBranch()
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Title: %s\n", title)
	if normStd == "pl" {
		sb.WriteString("Credit: scenariusz\n")
	} else {
		sb.WriteString("Credit: Written by\n")
	}
	fmt.Fprintf(&sb, "Author: %s\n", author)
	if desc := strings.TrimSpace(b.Description); desc != "" {
		fmt.Fprintf(&sb, "Notes: %s\n", desc)
	}
	fmt.Fprintf(&sb, "Draft date: %s (%s)\n\n", time.Now().Format("2006-01-02"), branchName)

	scenes := LinearizeScreenplayScenes(b)
	lastAct := ""

	for i, sc := range scenes {
		sceneNum := i + 1
		actName := strings.TrimSpace(sc.Chapter)
		if actName != "" && !strings.EqualFold(actName, lastAct) {
			fmt.Fprintf(&sb, "# %s\n\n", actName)
			lastAct = actName
		}

		blocks := ParseScreenplaySceneBlocks(sc, normStd)
		hasSlug := len(blocks) > 0 && blocks[0].Kind == BlockSlugline
		if !hasSlug {
			slug := strings.ToUpper(strings.TrimSpace(sc.Slugline))
			if slug == "" {
				slug = DefaultSluglineForStandard(normStd)
			}
			blocks = append([]ScreenplayBlock{{Kind: BlockSlugline, Text: slug}}, blocks...)
		}

		renderedPrimarySlug := false
		for j, blk := range blocks {
			switch blk.Kind {
			case BlockSlugline:
				slugTxt := strings.ToUpper(strings.TrimSpace(blk.Text))
				if !renderedPrimarySlug {
					renderedPrimarySlug = true
					if numbered {
						if IsSluglineLine(slugTxt) && !strings.HasPrefix(slugTxt, ".") {
							fmt.Fprintf(&sb, "%s #%d#\n", slugTxt, sceneNum)
						} else {
							fmt.Fprintf(&sb, ".%s #%d#\n", slugTxt, sceneNum)
						}
					} else {
						if IsSluglineLine(slugTxt) && !strings.HasPrefix(slugTxt, ".") {
							fmt.Fprintf(&sb, "%s\n", slugTxt)
						} else {
							fmt.Fprintf(&sb, ".%s\n", slugTxt)
						}
					}
					if syn := strings.TrimSpace(sc.Synopsis); syn != "" {
						fmt.Fprintf(&sb, "= %s\n", syn)
					}
					sb.WriteString("\n")
				} else {
					fmt.Fprintf(&sb, "%s\n\n", slugTxt)
				}
			case BlockAction:
				fmt.Fprintf(&sb, "%s\n\n", strings.TrimSpace(blk.Text))
			case BlockCharacter:
				fmt.Fprintf(&sb, "%s\n", strings.TrimSpace(blk.Text))
			case BlockParenthetical:
				fmt.Fprintf(&sb, "%s\n", strings.TrimSpace(blk.Text))
			case BlockDialogue:
				fmt.Fprintf(&sb, "%s\n", strings.TrimSpace(blk.Text))
				// Add trailing blank line unless next block is a parenthetical continuing the same speech
				if j+1 >= len(blocks) || blocks[j+1].Kind != BlockParenthetical {
					sb.WriteString("\n")
				}
			case BlockTransition:
				trans := strings.TrimSpace(blk.Text)
				if !strings.HasSuffix(trans, "TO:") && !strings.HasSuffix(trans, "DO:") {
					fmt.Fprintf(&sb, "> %s\n\n", trans)
				} else {
					fmt.Fprintf(&sb, "%s\n\n", trans)
				}
			}
		}
	}

	return strings.TrimSpace(sb.String()) + "\n"
}

var fountainSceneNumSuffixRe = regexp.MustCompile(`\s*#[A-Za-z0-9.\-]+#\s*$`)

// ImportedSceneDraft holds a parsed scene before building a Book graph.
type ImportedSceneDraft struct {
	ActName  string
	Slugline string
	Title    string
	Synopsis string
	Body     string
}

// ImportScreenplayFileToDir imports a .fountain or .fdx screenplay file into destParentDir as a Playbook Studio project.
func ImportScreenplayFileToDir(srcPath, destParentDir string) (*Book, error) {
	cleanPath := filepath.Clean(strings.TrimSpace(srcPath))
	rawBytes, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, err
	}
	base := filepath.Base(cleanPath)
	ext := strings.ToLower(filepath.Ext(base))
	fallbackTitle := strings.TrimSuffix(base, filepath.Ext(base))
	raw := string(rawBytes)
	switch ext {
	case ".fdx":
		return ImportScreenplayFDX(raw, fallbackTitle, destParentDir)
	case ".fountain":
		return ImportScreenplayFountain(raw, fallbackTitle, destParentDir)
	default:
		return nil, fmt.Errorf("nieobsługiwany format scenariusza: %s (oczekiwano .fountain lub .fdx)", ext)
	}
}

// ImportScreenplayFountain parses a UTF-8 .fountain screenplay into a Playbook Studio node-graph Book saved under destParentDir.
func ImportScreenplayFountain(raw, fallbackTitle, destParentDir string) (*Book, error) {
	normalized := strings.ReplaceAll(strings.TrimPrefix(raw, "\xef\xbb\xbf"), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	title := ""
	author := "Autor"
	description := ""
	idx := 0

	// 1. Parse optional Fountain Title Page key: value block at top of file
	var curMetaKey string
	for idx < len(lines) {
		ln := lines[idx]
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" {
			idx++
			break
		}
		if colon := strings.IndexByte(ln, ':'); colon > 0 && !strings.HasPrefix(ln, " ") && !strings.HasPrefix(ln, "\t") {
			candidateKey := strings.ToLower(strings.TrimSpace(ln[:colon]))
			if candidateKey == "title" || candidateKey == "credit" || candidateKey == "author" || candidateKey == "authors" || candidateKey == "source" || candidateKey == "draft date" || candidateKey == "contact" || candidateKey == "notes" || candidateKey == "copyright" {
				curMetaKey = candidateKey
				val := strings.TrimSpace(ln[colon+1:])
				applyFountainMeta(curMetaKey, val, &title, &author, &description)
				idx++
				continue
			}
		}
		if curMetaKey != "" && (strings.HasPrefix(ln, " ") || strings.HasPrefix(ln, "\t")) {
			applyFountainMeta(curMetaKey, trimmed, &title, &author, &description)
			idx++
			continue
		}
		// Not a title page header; rewind to start of document
		if curMetaKey == "" {
			idx = 0
		}
		break
	}
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSpace(fallbackTitle)
	}

	curAct := ""
	var drafts []ImportedSceneDraft
	var curScene *ImportedSceneDraft
	var bodyLines []string

	flushScene := func() {
		if curScene == nil {
			return
		}
		curScene.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
		drafts = append(drafts, *curScene)
		curScene = nil
		bodyLines = nil
	}

	for ; idx < len(lines); idx++ {
		rawLn := lines[idx]
		trimmed := strings.TrimSpace(rawLn)

		// Act / Section heading: # Act Name (single '#', not scene number '#1#')
		if strings.HasPrefix(trimmed, "#") && !strings.HasSuffix(trimmed, "#") {
			hashes := 0
			for hashes < len(trimmed) && trimmed[hashes] == '#' {
				hashes++
			}
			secTitle := strings.TrimSpace(trimmed[hashes:])
			if secTitle != "" && hashes == 1 {
				flushScene()
				curAct = secTitle
				continue
			}
		}

		// Forced slugline (.SLUG) or standard slugline (WN./PL./INT./EXT.)
		isForcedSlug := strings.HasPrefix(trimmed, ".") && len(trimmed) > 1 && trimmed[1] != '.'
		if isForcedSlug || IsSluglineLine(trimmed) {
			flushScene()
			slug := trimmed
			if isForcedSlug {
				slug = strings.TrimSpace(slug[1:])
			}
			slug = strings.TrimSpace(fountainSceneNumSuffixRe.ReplaceAllString(slug, ""))
			slug = strings.ToUpper(slug)
			curScene = &ImportedSceneDraft{
				ActName:  curAct,
				Slugline: slug,
				Title:    slug,
			}
			continue
		}

		// Synopsis line (= synopsis)
		if strings.HasPrefix(trimmed, "=") && !strings.HasPrefix(trimmed, "===") {
			syn := strings.TrimSpace(trimmed[1:])
			if curScene != nil && syn != "" {
				if curScene.Synopsis == "" {
					curScene.Synopsis = syn
				} else {
					curScene.Synopsis += " " + syn
				}
			}
			continue
		}

		// Forced transition (> CUT TO:)
		if strings.HasPrefix(trimmed, ">") && !strings.HasSuffix(trimmed, "<") {
			trimmed = strings.TrimSpace(trimmed[1:])
			rawLn = trimmed
		}

		if curScene == nil {
			if trimmed == "" {
				continue
			}
			curScene = &ImportedSceneDraft{
				ActName:  curAct,
				Slugline: "",
			}
		}
		bodyLines = append(bodyLines, rawLn)
	}
	flushScene()

	return buildImportedScreenplayBook(title, author, description, drafts, destParentDir)
}

func applyFountainMeta(key, val string, title, author, description *string) {
	if val == "" {
		return
	}
	switch key {
	case "title":
		if *title == "" || strings.EqualFold(*title, "Bez tytułu") {
			*title = val
		} else {
			*title += " " + val
		}
	case "author", "authors":
		if *author == "" || *author == "Autor" {
			*author = val
		} else {
			*author += ", " + val
		}
	case "notes":
		if *description == "" {
			*description = val
		} else {
			*description += " " + val
		}
	}
}

func buildImportedScreenplayBook(title, author, description string, drafts []ImportedSceneDraft, destParentDir string) (*Book, error) {
	cleanTitle := SanitizePlainText(title)
	if cleanTitle == "" {
		cleanTitle = "Zaimportowany Scenariusz"
	}
	cleanAuthor := SanitizePlainText(author)
	if cleanAuthor == "" {
		cleanAuthor = "Autor"
	}
	cleanDesc := SanitizePlainText(description)
	if cleanDesc == "" {
		cleanDesc = "Scenariusz zaimportowany do Playbook Studio."
	}

	// Detect PL vs US standard from sluglines
	plCount, intlCount := 0, 0
	for _, d := range drafts {
		up := strings.ToUpper(strings.TrimSpace(d.Slugline))
		if strings.HasPrefix(up, "WN.") || strings.HasPrefix(up, "PL.") || strings.HasPrefix(up, "WN/PL") {
			plCount++
		} else if strings.HasPrefix(up, "INT.") || strings.HasPrefix(up, "EXT.") || strings.HasPrefix(up, "I/E") {
			intlCount++
		}
	}
	std := "pl"
	if intlCount > plCount {
		std = "us"
	}
	defActName := DefaultScreenplayActName
	if std != "pl" {
		defActName = "Act I"
	}
	defSlug := DefaultSluglineForStandard(std)

	if len(drafts) == 0 {
		drafts = []ImportedSceneDraft{
			{
				ActName:  defActName,
				Slugline: defSlug,
				Title:    "Scena 1",
				Body:     "",
			},
		}
	}

	id := SanitizeBookID(cleanTitle)
	cleanBaseDir := filepath.Clean(strings.TrimSpace(destParentDir))
	if cleanBaseDir == "" || cleanBaseDir == "." {
		cleanBaseDir = DefaultProjectsDir()
	}
	bookDir := filepath.Join(cleanBaseDir, id)
	if _, err := os.Stat(bookDir); err == nil {
		id = fmt.Sprintf("%s-%d", id, time.Now().Unix()%10000)
		bookDir = filepath.Join(cleanBaseDir, id)
	}
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		return nil, err
	}

	var chapters []Chapter
	chapterByLower := make(map[string]int)
	usedChIDs := make(map[string]bool)
	usedSceneIDs := make(map[string]bool)

	ensureChapter := func(actName string) Chapter {
		name := strings.TrimSpace(actName)
		if name == "" {
			name = defActName
		}
		key := strings.ToLower(name)
		if idx, ok := chapterByLower[key]; ok {
			return chapters[idx]
		}
		baseID := SanitizeSceneID(name)
		if baseID == "" {
			baseID = fmt.Sprintf("akt_%d", len(chapters)+1)
		}
		chID := baseID
		seq := 2
		for usedChIDs[chID] {
			chID = fmt.Sprintf("%s_%d", baseID, seq)
			seq++
		}
		usedChIDs[chID] = true
		ch := Chapter{
			ID:         chID,
			ExplicitID: chID,
			Name:       name,
			SourceFile: chID + PlaytxtExt,
		}
		chapterByLower[key] = len(chapters)
		chapters = append(chapters, ch)
		return ch
	}

	scenes := make([]*Scene, 0, len(drafts))
	for i, d := range drafts {
		ch := ensureChapter(d.ActName)
		slug := strings.TrimSpace(d.Slugline)
		if slug == "" {
			slug = defSlug
		}
		scTitle := strings.TrimSpace(d.Title)
		if scTitle == "" {
			scTitle = fmt.Sprintf("Scena %d", i+1)
		}
		baseID := SanitizeSceneID(scTitle)
		if baseID == "" || baseID == "scena" {
			baseID = fmt.Sprintf("scena_%d", i+1)
		}
		if len(baseID) > 36 {
			baseID = strings.Trim(baseID[:36], "_")
		}
		scID := baseID
		seq := 2
		for usedSceneIDs[scID] {
			scID = fmt.Sprintf("%s_%d", baseID, seq)
			seq++
		}
		usedSceneIDs[scID] = true

		sc := &Scene{
			ID:         scID,
			ExplicitID: scID,
			Name:       scTitle,
			Slugline:   slug,
			Synopsis:   strings.TrimSpace(d.Synopsis),
			Chapter:    ch.Name,
			ChapterID:  ch.ID,
			Text:       strings.TrimSpace(d.Body),
			SourceFile: ch.SourceFile,
		}
		scenes = append(scenes, sc)
	}

	// Wire sequential transitions within acts and across acts
	for i, sc := range scenes {
		if i+1 < len(scenes) {
			nextSc := scenes[i+1]
			if strings.EqualFold(sc.Chapter, nextSc.Chapter) {
				sc.ExplicitNext = nextSc.ID
				sc.Choices = []Choice{{Label: "Dalej", Target: nextSc.ID, TargetSceneID: nextSc.ID, IsImplicit: true}}
			} else {
				targetRef := "chapter:" + nextSc.ChapterID
				sc.ExplicitNext = targetRef
				sc.Choices = []Choice{{Label: "Dalej", Target: targetRef, TargetChapterID: nextSc.ChapterID, IsChapterLink: true, IsImplicit: true}}
			}
		} else {
			sc.ExplicitEnd = true
		}
	}

	for i := range chapters {
		for _, sc := range scenes {
			if strings.EqualFold(sc.Chapter, chapters[i].Name) {
				chapters[i].StartSceneID = sc.ID
				chapters[i].ExplicitStart = sc.ID
				break
			}
		}
	}

	startID := scenes[0].ID
	b := &Book{
		ID:             id,
		Title:          cleanTitle,
		Author:         cleanAuthor,
		Description:    cleanDesc,
		Mode:           "screenplay",
		ScriptStandard: std,
		StartSceneID:   startID,
		RawStart:       startID,
		Narrated:       false,
		Imageless:      true,
		RootPath:       bookDir,
		Chapters:       chapters,
		Scenes:         scenes,
	}
	FinalizeBook(b)
	AutoLayoutBook(b)
	if err := SaveBookScenesToDisk(b); err != nil {
		return nil, err
	}
	_ = GenerateAtmosphericPNG(filepath.Join(bookDir, "cover.png"), 400, 560, color.RGBA{24, 30, 44, 255}, color.RGBA{214, 158, 76, 255}, 1)
	return LoadBookFromDir(bookDir)
}
