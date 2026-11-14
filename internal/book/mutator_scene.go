//go:build !(js && wasm)

package book

import (
	"fmt"
	"strings"
)

const (
	defaultChoiceLabelPl   = "Dalej"
	defaultChoiceTargetEnd = "END"
	defaultChoiceTargetNxt = "Next"
	choiceLabelFmtPl       = "Wybór %d"
	choiceFirstLabelPl     = "Wybór 1"
	defaultNewSceneTextPl  = "Nowy fragment opowieści..."
	defaultFallbackNamePl  = "Nowa scena 1"
	errMinOneScenePl       = "Książka musi zawierać przynajmniej jedną scenę"
)

func MaterializeChoicesForEdit(b *Book, sc *Scene) []Choice {
	if sc == nil {
		return nil
	}
	var list []Choice
	for _, ch := range sc.Choices {
		tgt := ch.Target
		if ch.IsImplicit && !ch.IsChapterLink && (strings.EqualFold(tgt, defaultChoiceTargetNxt) || strings.EqualFold(tgt, defaultChoiceLabelPl) || tgt == "") && b != nil {
			if tSc := b.FindScene(ch.TargetSceneID); tSc != nil {
				tgt = tSc.ReferenceKey()
			}
		}
		lbl := ch.Label
		if lbl == "" {
			lbl = defaultChoiceLabelPl
		}
		if tgt != "" {
			list = append(list, Choice{
				Label: lbl, Target: tgt, Condition: ch.Condition, Effects: ch.Effects,
				ShowWhenLocked: ch.ShowWhenLocked, TargetSceneID: ch.TargetSceneID,
				TargetChapterID: ch.TargetChapterID, IsChapterLink: ch.IsChapterLink,
				IsEnding: ch.IsEnding || strings.EqualFold(tgt, defaultChoiceTargetEnd),
			})
		}
	}
	return list
}

func AddChoiceToScene(b *Book, target *Scene) {
	if target == nil {
		return
	}
	choices := MaterializeChoicesForEdit(b, target)
	if len(choices) == 1 && (choices[0].Label == "" || strings.EqualFold(choices[0].Label, defaultChoiceLabelPl)) {
		choices[0].Label = choiceFirstLabelPl
	}
	targetCount := len(choices) + 1
	if targetCount < 2 {
		targetCount = 2
	}
	for len(choices) < targetCount {
		choices = append(choices, Choice{
			Label: fmt.Sprintf(choiceLabelFmtPl, len(choices)+1), Target: defaultChoiceTargetEnd,
			TargetSceneID: defaultChoiceTargetEnd, IsEnding: true,
		})
	}
	target.Choices, target.ExplicitNext, target.ExplicitEnd = choices, "", false
}

func RemoveChoiceFromScene(b *Book, target *Scene, choiceIdx int) {
	if target == nil {
		return
	}
	choices := MaterializeChoicesForEdit(b, target)
	if choiceIdx >= 0 && choiceIdx < len(choices) {
		choices = append(choices[:choiceIdx], choices[choiceIdx+1:]...)
	}
	if len(choices) == 0 || (len(choices) == 1 && (choices[0].IsEnding || strings.EqualFold(choices[0].Target, defaultChoiceTargetEnd))) {
		target.Choices, target.ExplicitNext, target.ExplicitEnd = nil, "", true
		return
	}
	if len(choices) == 1 {
		choices[0].Label, choices[0].IsImplicit = defaultChoiceLabelPl, true
		target.Choices, target.ExplicitNext, target.ExplicitEnd = choices, choices[0].Target, false
		return
	}
	target.Choices, target.ExplicitNext, target.ExplicitEnd = choices, "", false
}

func ReorderSceneChoice(b *Book, target *Scene, fromIdx, toIdx int) bool {
	if target == nil || fromIdx == toIdx {
		return false
	}
	choices := MaterializeChoicesForEdit(b, target)
	if len(choices) < 2 || fromIdx < 0 || fromIdx >= len(choices) || toIdx < 0 || toIdx >= len(choices) {
		return false
	}
	moved := choices[fromIdx]
	without := append(append([]Choice(nil), choices[:fromIdx]...), choices[fromIdx+1:]...)
	reordered := append(append(append([]Choice(nil), without[:toIdx]...), moved), without[toIdx:]...)
	target.Choices, target.ExplicitNext, target.ExplicitEnd = reordered, "", false
	return true
}

func BranchChoiceToNewScene(b *Book, target *Scene, choiceIdx int) (*Scene, error) {
	if b == nil || target == nil {
		return nil, nil
	}
	choices := MaterializeChoicesForEdit(b, target)
	newName, allocID := NextUniqueSceneName(b), b.NextUniqueAutoSceneID(nil)
	b.BindAutoSceneID(newName, allocID)
	newScene := &Scene{
		ID: allocID, ExplicitID: allocID, Name: newName,
		Chapter: target.Chapter, Text: defaultNewSceneTextPl, SourceFile: target.SourceFile,
	}
	if choiceIdx >= 0 && choiceIdx < len(choices) {
		choices[choiceIdx].Target, choices[choiceIdx].IsEnding = newScene.Name, false
		if strings.TrimSpace(choices[choiceIdx].Label) == "" {
			choices[choiceIdx].Label = defaultChoiceLabelPl
		}
	} else {
		choices = append(choices, Choice{Label: fmt.Sprintf(choiceLabelFmtPl, len(choices)+1), Target: newScene.Name})
	}
	target.Choices, target.ExplicitEnd = choices, false
	if len(choices) == 1 {
		target.ExplicitNext = newScene.Name
	}
	insertPos := len(b.Scenes)
	for i, sc := range b.Scenes {
		if sc.ID == target.ID {
			insertPos = i + 1
			break
		}
	}
	b.Scenes = append(b.Scenes[:insertPos], append([]*Scene{newScene}, b.Scenes[insertPos:]...)...)
	FinalizeBook(b)
	StoreSceneNodePosition(b, newScene, NodePosition{Row: newScene.Row, Col: newScene.Col, X: newScene.X, Y: newScene.Y, HasXY: true})
	if err := SaveBookScenesToDisk(b); err != nil {
		return nil, err
	}
	return newScene, nil
}

func RebindRenamedScene(b *Book, target *Scene, oldID, oldName, oldExplicitID string) bool {
	if b == nil || target == nil || (strings.EqualFold(oldName, target.Name) && strings.EqualFold(oldExplicitID, target.ExplicitID)) {
		return false
	}
	if target.ExplicitID != "" {
		target.ID = target.ExplicitID
	}
	newID, newRef, oldNorm := target.ID, target.ReferenceKey(), NormalizeLookupKey(oldName)
	matchesOld := func(val string) bool {
		v := strings.TrimSpace(val)
		return v != "" && ((oldID != "" && strings.EqualFold(v, oldID)) ||
			(oldExplicitID != "" && strings.EqualFold(v, oldExplicitID)) ||
			(oldName != "" && (strings.EqualFold(v, oldName) || (oldNorm != "" && NormalizeLookupKey(v) == oldNorm))))
	}
	if b.Layout != nil {
		keys := []string{oldExplicitID, strings.ToLower(oldExplicitID), oldName, oldNorm, oldID, strings.ToLower(oldID)}
		for _, k := range keys {
			if pos, ok := b.Layout[k]; k != "" && ok {
				for _, delK := range keys {
					delete(b.Layout, delK)
				}
				StoreSceneNodePosition(b, target, pos)
				break
			}
		}
	}
	if b.WirePins != nil && oldID != "" && newID != "" && !strings.EqualFold(oldID, newID) {
		remapped := make(map[string][]WirePin, len(b.WirePins))
		for wk, pins := range b.WirePins {
			if parts := strings.Split(wk, "|"); len(parts) == 3 {
				if matchesOld(parts[0]) {
					parts[0] = newID
				}
				if matchesOld(parts[2]) {
					parts[2] = newID
				}
				wk = strings.Join(parts, "|")
			}
			remapped[wk] = pins
		}
		b.WirePins = remapped
	}
	if b.HasEditorView {
		if err := SaveBookEditorStateToDisk(b); err != nil {
			b.Warnings = append(b.Warnings, err.Error())
		}
	}
	if matchesOld(b.StartSceneID) || matchesOld(b.RawStart) {
		b.RawStart, b.StartSceneID = newRef, newID
	}
	for i := range b.Chapters {
		if ch := &b.Chapters[i]; matchesOld(ch.StartSceneID) || matchesOld(ch.ExplicitStart) {
			if ch.ExplicitStart != "" {
				ch.ExplicitStart = newRef
			}
			ch.StartSceneID = newID
		}
	}
	for _, other := range b.Scenes {
		if matchesOld(other.ExplicitNext) {
			other.ExplicitNext = newRef
		}
		for i := range other.Choices {
			c := &other.Choices[i]
			if !c.IsChapterLink && !strings.EqualFold(c.Target, defaultChoiceTargetNxt) && !strings.EqualFold(c.Target, defaultChoiceTargetEnd) {
				if matchesOld(c.Target) || matchesOld(c.TargetSceneID) {
					c.Target, c.TargetSceneID = newRef, newID
				}
			} else if c.IsImplicit && matchesOld(c.TargetSceneID) {
				c.TargetSceneID = newID
			}
		}
	}
	return true
}

func DeleteScenesByIDs(b *Book, rawIDs string, allowFallbackWhenEmpty bool) ([]*Scene, error) {
	if b == nil || len(b.Scenes) == 0 || (!allowFallbackWhenEmpty && len(b.Scenes) <= 1) {
		return nil, fmt.Errorf(errMinOneScenePl)
	}
	delIDs, delExp, delNames := make(map[string]bool), make(map[string]bool), make(map[string]bool)
	var deleted []*Scene
	for _, part := range strings.Split(rawIDs, ",") {
		if sc := b.FindScene(strings.TrimSpace(part)); sc != nil && !delIDs[sc.ID] {
			delIDs[sc.ID] = true
			if sc.ExplicitID != "" {
				delExp[strings.ToLower(strings.TrimSpace(sc.ExplicitID))] = true
			}
			if sc.Name != "" {
				delNames[NormalizeLookupKey(sc.Name)] = true
			}
			deleted = append(deleted, sc)
		}
	}
	if len(deleted) == 0 || (len(deleted) == 1 && len(b.Scenes) <= 1) {
		if len(deleted) == 1 {
			return nil, fmt.Errorf(errMinOneScenePl)
		}
		return nil, nil
	}
	first := b.Scenes[0]
	srcFile := first.SourceFile
	if srcFile == "" || strings.EqualFold(srcFile, DefaultStoryFile) || strings.EqualFold(srcFile, BookDataFileName) {
		srcFile = b.ChapterFileNameFor(first.Chapter, first.ChapterID)
	}
	var remaining []*Scene
	for _, sc := range b.Scenes {
		if !delIDs[sc.ID] {
			remaining = append(remaining, sc)
		}
	}
	for _, sc := range deleted {
		b.UnbindAutoSceneName(sc.Name)
		b.RetireSceneID(sc.ID, sc.ExplicitID)
		for _, k := range []string{sc.ID, sc.ExplicitID, strings.ToLower(sc.ExplicitID), sc.Name, NormalizeLookupKey(sc.Name)} {
			delete(b.Layout, k)
		}
	}
	if len(remaining) == 0 {
		remaining = []*Scene{{Name: defaultFallbackNamePl, Chapter: first.Chapter, ChapterID: first.ChapterID, Text: defaultNewSceneTextPl, SourceFile: srcFile, ExplicitEnd: true}}
	}
	b.Scenes = remaining
	matchesDel := func(val string) bool {
		v := strings.TrimSpace(val)
		return v != "" && (delIDs[v] || delExp[strings.ToLower(v)] || delNames[NormalizeLookupKey(v)])
	}
	for i := range b.Chapters {
		if matchesDel(b.Chapters[i].StartSceneID) || matchesDel(b.Chapters[i].ExplicitStart) {
			b.Chapters[i].ExplicitStart = ""
		}
	}
	for _, sc := range b.Scenes {
		if matchesDel(sc.ExplicitNext) {
			sc.ExplicitNext = ""
		}
		var kept []Choice
		for _, ch := range sc.Choices {
			if ch.IsImplicit || (!delIDs[ch.TargetSceneID] && !matchesDel(ch.Target)) {
				kept = append(kept, ch)
			}
		}
		sc.Choices = kept
	}
	if matchesDel(b.StartSceneID) || matchesDel(b.RawStart) {
		b.RawStart, b.StartSceneID = b.Scenes[0].ReferenceKey(), b.Scenes[0].ID
	}
	FinalizeBook(b)
	if err := SaveBookScenesToDisk(b); err != nil {
		return nil, err
	}
	return deleted, nil
}
