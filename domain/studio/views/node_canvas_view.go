//go:build !(js && wasm)

package views

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	book "mojra/domain/book/actions"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/engine/templates"
)

const (
	graphChapterFmt = "chapter | id:%s | ref:%s | name:%s | root:%d | start:%s | outs:%s\n"
	graphNodeFmt    = "node | id:%s | name:%s | chapter:%s | start:%d | entry:%d | unreachable:%d | has_choices:%d | imp_target:%s | imp_chapter:%s | branch_role:%s | branch_parent:%s\n"
	graphPortFmt    = "port | scene:%s | idx:%d | label:%s | target_id:%s | target_chapter:%s | cond:%d\n"
	layoutPosFmt    = "pos | id:%s | x:%d | y:%d\n"
	layoutPinFmt    = "pin | key:%s | pts:%s\n"
	layoutPtFmt     = "%d,%d"
	layoutHashFmt   = "%08x"
	endTargetToken  = "END"
)

type nodeCanvasTemplate struct {
	EditorChapterTitlePlaceholder string
	IsHistoryPreview              bool
	IsScreenplayMode              bool
}

type nodeCanvasScript struct {
	isHistoryPreview bool
	selectedSceneId  string
	startSceneId     string
	sceneId          string
	scenePrevId      string
	bookGraph        string
	canvasLayout     string
	layoutHash       string
	savedPanX        int
	savedPanY        int
	savedZoom        float64
	hasView          bool
	resetCanvas      bool
}

// NodeCanvasView builds the TemplateChunk (HTML + ScriptData) for the Playbook Studio node canvas.
func NodeCanvasView(s *state.StudioState, orderedScenes []*book.Scene) templates.TemplateChunk {
	b := s.Editor.ActiveBook
	isSp := b != nil && b.IsScreenplay()
	chTitlePlaceholder := tr(s, i18n.TrEditorChapterTitlePlaceholder)
	if isSp {
		chTitlePlaceholder = tr(s, i18n.TrEditorActTitlePlaceholder)
	}
	isHistPreview := !s.Config.StandaloneMode && s.Editor.History.IsPreview(b)
	return templates.TemplateChunk{
		TemplateData: &nodeCanvasTemplate{
			EditorChapterTitlePlaceholder: chTitlePlaceholder,
			IsHistoryPreview:              isHistPreview,
			IsScreenplayMode:              isSp,
		},
		ScriptData: buildNodeCanvasScript(s, orderedScenes, true),
	}
}

// NodeCanvasScriptView returns a script-only TemplateChunk hydrating NodeCanvasController.data on partial updates.
func NodeCanvasScriptView(s *state.StudioState) templates.TemplateChunk {
	return templates.TemplateChunk{
		ScriptData: buildNodeCanvasScript(s, nil, true),
	}
}

func buildNodeCanvasScript(s *state.StudioState, orderedScenes []*book.Scene, includeLayout bool) *nodeCanvasScript {
	b := s.Editor.ActiveBook
	histPreview := !s.Config.StandaloneMode && s.Editor.History.IsPreview(b)
	sc := &nodeCanvasScript{
		isHistoryPreview: histPreview,
		selectedSceneId:  s.Editor.SelectedSceneID,
		savedZoom:        1.0,
		resetCanvas:      s.Overlays.ResetCanvas,
	}
	if b == nil {
		return sc
	}
	edZoom := b.EditorZoom
	if edZoom < 0.2 || edZoom > 3.0 {
		edZoom = 1.0
	}
	sc.savedPanX = b.EditorPanX
	sc.savedPanY = b.EditorPanY
	sc.savedZoom = edZoom
	sc.hasView = b.HasEditorView
	if s.Editor.SelectedSceneID != "" {
		sc.sceneId = s.Editor.SelectedSceneID
		if s.Editor.LastRenamedFromSceneID != "" && !strings.EqualFold(s.Editor.LastRenamedFromSceneID, s.Editor.SelectedSceneID) {
			sc.scenePrevId = s.Editor.LastRenamedFromSceneID
		}
		s.Editor.LastRenamedFromSceneID = ""
	}
	if b.StartSceneID != "" {
		sc.startSceneId = b.StartSceneID
	}
	if s.Editor.HasActiveBook() && len(b.Scenes) > 0 {
		layoutPayload, layoutHash := buildCanvasLayoutPayload(s, b)
		sc.layoutHash = layoutHash
		if includeLayout || s.Overlays.ResetCanvas {
			sc.canvasLayout = layoutPayload
		}
		if len(orderedScenes) == 0 {
			orderedScenes = orderStudioScenes(b)
		}
		sc.bookGraph = buildBookGraphPayload(s, b, orderedScenes, tr(s, i18n.TrNext))
	}
	return sc
}

func buildBookGraphPayload(s *state.StudioState, b *book.Book, orderedScenes []*book.Scene, defaultChoiceLabel string) string {
	if b == nil {
		return ""
	}
	portOffset := 0
	bitScale := 1
	if s != nil && s.Hooks.UnmaskStudioInt != nil {
		portOffset = s.Hooks.UnmaskStudioInt(0x4F, 48)
		bitScale = s.Hooks.UnmaskStudioInt(0x4D^1, 56)
	}
	var sb strings.Builder
	sb.Grow(len(orderedScenes)*140 + len(b.Chapters)*96)

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
		fmt.Fprintf(&sb, graphChapterFmt,
			sanitizeGraphPayloadField(chItem.ID),
			sanitizeGraphPayloadField(chRef),
			sanitizeGraphPayloadField(chName),
			boolToPayloadBit(isRoot),
			sanitizeGraphPayloadField(startSceneID),
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

		fmt.Fprintf(&sb, graphNodeFmt,
			sanitizeGraphPayloadField(id),
			sanitizeGraphPayloadField(name),
			sanitizeGraphPayloadField(chapter),
			boolToPayloadBit(isStart)*bitScale,
			boolToPayloadBit(isEntry)*bitScale,
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
						Label:      defaultChoiceLabel,
						Target:     endTargetToken,
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
				fmt.Fprintf(&sb, graphPortFmt,
					sanitizeGraphPayloadField(id),
					chIdx+portOffset,
					sanitizeGraphPayloadField(lbl),
					sanitizeGraphPayloadField(ch.TargetSceneID),
					sanitizeGraphPayloadField(targetChapterRef),
					boolToPayloadBit(hasCond))
			}
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

func buildCanvasLayoutPayload(s *state.StudioState, b *book.Book) (string, string) {
	if b == nil || len(b.Scenes) == 0 {
		return "", ""
	}
	offsetX := 0
	offsetY := 0
	if s != nil && s.Hooks.UnmaskStudioInt != nil {
		offsetX = s.Hooks.UnmaskStudioInt(0x52, 32)
		offsetY = s.Hooks.UnmaskStudioInt(0x4A, 40)
	}
	var sb strings.Builder
	sb.Grow(len(b.Scenes)*40 + len(b.WirePins)*48)
	for _, sc := range b.Scenes {
		fmt.Fprintf(&sb, layoutPosFmt, sanitizeGraphPayloadField(sc.ID), sc.X+offsetX, sc.Y+offsetY)
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
				parts = append(parts, fmt.Sprintf(layoutPtFmt, p.X, p.Y))
			}
			fmt.Fprintf(&sb, layoutPinFmt,
				sanitizeGraphPayloadField(k),
				strings.Join(parts, ";"))
		}
	}
	payload := strings.TrimSuffix(sb.String(), "\n")
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(payload))
	return payload, fmt.Sprintf(layoutHashFmt, hasher.Sum32())
}
