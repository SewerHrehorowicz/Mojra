//go:build js && wasm

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"syscall/js"

	"mojra"
	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/tpl"
)

type playerTemplate struct {
	ImagelessMode     bool
	IsScreenplayMode  bool
	IsHistoryPreview  bool
	HasHistory        bool
	HistoryScenes     []tpl.TemplateChunk
	HistoryTicks      []string
	HeaderMeta        string
	RailTitle         string
	HasBgImage        bool
	BgImageURL        string
	CurrentScene      tpl.TemplateChunk
	ShowAudioDeck     bool
	AudioDeck         tpl.TemplateChunk
	PrefetchMediaURLs string
	BookProgressPct   int
}

type playerScript struct {
	bookId        string
	sensitivity   string
	imagelessMode bool
	saveHex       string
}

type sceneChoiceItem struct {
	Label          string
	ShowWhenLocked bool
	IsEnding       bool
}

type screenplayBlockItem struct {
	Kind string
	Text string
}

type sceneTemplate struct {
	IsPresent        bool
	IsScreenplayMode bool
	SceneNum         int
	ShowSceneNum     bool
	Name             string
	Slugline         string
	RailTitle        string
	HasTitle         bool
	Prose            string
	ScreenplayBlocks []screenplayBlockItem
	HasImage         bool
	ImageURL         string
	IsTerminal       bool
	IsBranching      bool
	Choices          []sceneChoiceItem
}

type sceneScript struct {
	isPresent        bool
	isScreenplayMode bool
	hasTitle         bool
	hasImage         bool
	imageUrl         string
	isTerminal       bool
	isBranching      bool
}

type audioDeckTemplate struct {
	AudioDurationLabel string
	AudioSpeed         string
	AutoChoiceEnabled  bool
	AutoChoiceLabel    string
}

type audioDeckScript struct {
	audioQueue         string
	audioTotalDuration float64
	resumeIndex        int
	resumeOffsetSec    float64
	audioSpeed         float64
	autoChoiceEnabled  bool
	autoChoiceLabel    string
	autoChoiceDelayMs  int
	canAutoAdvance     bool
	noAudioToast       string
}

type confirmTemplate struct {
	ShowConfirmModal bool
	ConfirmTitle     string
	ConfirmMessage   string
	ConfirmIsDanger  bool
}

type appShellScript struct {
	imageless       bool
	imagelessLocked bool
	toast           string
}

type wasmPlayerRuntime struct {
	book                 *book.Book
	media                map[string][]byte
	blobURLs             map[string]string
	save                 *book.SaveState
	templates            map[string]*tpl.ParsedTemplate
	confirmTitle         string
	confirmMessage       string
	confirmPendingAction string
	confirmPendingParam  string
	historyRevealed      bool
	focusedHistoryIdx    int
}

var rt = &wasmPlayerRuntime{
	blobURLs:          make(map[string]string),
	focusedHistoryIdx: -1,
}

func main() {
	templates, err := tpl.LoadTemplatesFromDir("", mojra.WasmAppFS)
	if err == nil {
		rt.templates = templates
	}

	js.Global().Set("__wasmBoot", js.FuncOf(wasmBoot))
	js.Global().Set("__wasmAction", js.FuncOf(wasmAction))

	select {}
}

func wasmBoot(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return ""
	}
	wasmArr := args[0]
	wasmLen := wasmArr.Get("byteLength").Int()
	if wasmLen <= 0 {
		return ""
	}
	wasmBytes := make([]byte, wasmLen)
	js.CopyBytesToGo(wasmBytes, wasmArr)

	var savedHex string
	if len(args) >= 2 && args[1].Type() == js.TypeString {
		savedHex = strings.TrimSpace(args[1].String())
	}

	b, media, err := book.ExtractSTBCFromWasm(wasmBytes)
	if err != nil || b == nil {
		return ""
	}
	rt.book = b
	rt.media = media
	rt.blobURLs = make(map[string]string, len(media))

	urlGlobal := js.Global().Get("URL")
	blobCtor := js.Global().Get("Blob")
	uint8Ctor := js.Global().Get("Uint8Array")
	arrCtor := js.Global().Get("Array")

	if !urlGlobal.IsUndefined() && !blobCtor.IsUndefined() && !uint8Ctor.IsUndefined() {
		for rel, raw := range media {
			if len(raw) == 0 {
				continue
			}
			u8 := uint8Ctor.New(len(raw))
			js.CopyBytesToJS(u8, raw)
			parts := arrCtor.New(1)
			parts.SetIndex(0, u8)
			opts := js.Global().Get("Object").New()
			opts.Set("type", wasmMediaMime(rel))
			blob := blobCtor.New(parts, opts)
			blobURL := urlGlobal.Call("createObjectURL", blob).String()
			rt.blobURLs[strings.ToLower(filepath.ToSlash(rel))] = blobURL
		}
	}

	applyBlobURLsToBook(b, rt.blobURLs)

	var savedBytes []byte
	if savedHex != "" {
		if dec, hErr := hex.DecodeString(savedHex); hErr == nil {
			savedBytes = dec
		}
	}
	startSceneID := b.StartSceneID
	if findSceneByID(b, startSceneID) == nil && len(b.Scenes) > 0 {
		startSceneID = b.Scenes[0].ID
	}
	rt.save = book.DecodeSaveStateBytes(b.ID, savedBytes, startSceneID)
	if findSceneByID(b, rt.save.CurrentSceneID) == nil && len(b.Scenes) > 0 {
		rt.save.CurrentSceneID = startSceneID
	}
	rt.save.Variables = book.EnsureSaveVariables(b, rt.save.Variables)
	if len(rt.save.History) == 0 {
		if cur := findSceneByID(b, rt.save.CurrentSceneID); cur != nil && cur.Effects != "" {
			book.ApplyEffects(cur.Effects, rt.save.Variables, b)
		}
	}

	playerChunk := rt.buildPlayerChunk()
	confirmChunk := rt.buildConfirmChunk()
	playerHTML := rt.renderChunk(playerChunk)
	modalsHTML := rt.renderChunk(confirmChunk)

	fullHTML := `<main class="workspace" id="workspace">` + playerHTML + `</main><div class="modals-root" id="modals-root">` + modalsHTML + `</div><div class="toast hidden" id="toast-banner"></div><div class="ui-tooltip hidden" id="ui-tooltip"></div>`
	var out bytes.Buffer
	_ = tpl.WriteFDPFrames(
		&out,
		tpl.FDPFrame{Target: "#app", HTMLBody: []byte(fullHTML), Chunk: playerChunk},
		tpl.FDPFrame{Target: "none", Chunk: tpl.TemplateChunk{ScriptData: &appShellScript{imageless: b.Imageless, imagelessLocked: b.Imageless}}},
	)
	return out.String()
}

func wasmAction(_ js.Value, args []js.Value) any {
	if len(args) < 1 || rt.book == nil || rt.save == nil {
		return ""
	}
	rawReq := args[0].String()
	frames := tpl.DecodeFDPFrames([]byte(rawReq))
	if len(frames) == 0 {
		return ""
	}
	form := frames[0]
	action := strings.TrimSpace(form.Get("action"))
	if action == "" {
		return ""
	}

	if action != "SaveAudioTime" {
		if atStr := strings.TrimSpace(form.Get("audio_time")); atStr != "" {
			if t, err := strconv.ParseFloat(atStr, 64); err == nil && t >= 0 {
				rt.save.AudioTime = t
			}
		}
	}

	target := "#workspace"
	var toast string

	switch action {
	case "SaveAudioTime":
		if t, err := strconv.ParseFloat(form.Get("time"), 64); err == nil && t >= 0 {
			rt.save.AudioTime = t
		}
		target = "none"

	case "ChooseBranch":
		if idx, err := strconv.Atoi(form.Get("choice_idx")); err == nil {
			cur := rt.currentScene()
			if cur != nil && idx >= 0 && idx < len(cur.Choices) {
				toast = rt.executeChoiceTransition(cur, cur.Choices[idx])
			}
		}
		if rt.confirmPendingAction != "" {
			target = "#modals-root"
		}

	case "AutoAdvanceImplicit":
		cur := rt.currentScene()
		if cur != nil && len(cur.Choices) == 1 && !cur.Choices[0].IsEnding && cur.Choices[0].TargetSceneID != "END" {
			toast = rt.executeChoiceTransition(cur, cur.Choices[0])
		}
		if rt.confirmPendingAction != "" {
			target = "#modals-root"
		}

	case "AudioEnded":
		if rt.save.AutoChoice {
			cur := rt.currentScene()
			if cur != nil && len(cur.Choices) > 0 && !cur.ExplicitEnd {
				rt.save.Variables = book.EnsureSaveVariables(rt.book, rt.save.Variables)
				var unlocked []book.Choice
				var unlockedNonEnding []book.Choice
				for _, ch := range cur.Choices {
					if !book.EvaluateCondition(ch.Condition, rt.save.Variables, rt.book) {
						continue
					}
					unlocked = append(unlocked, ch)
					if !ch.IsEnding && !strings.EqualFold(ch.TargetSceneID, "END") {
						unlockedNonEnding = append(unlockedNonEnding, ch)
					}
				}
				candidates := unlockedNonEnding
				if len(candidates) == 0 {
					candidates = unlocked
				}
				if len(candidates) > 0 {
					chosen := candidates[0]
					if rt.save.AutoChoiceStrategy == "random" && len(candidates) > 1 {
						chosen = candidates[rand.Intn(len(candidates))]
					}
					toast = rt.executeChoiceTransition(cur, chosen)
				}
			}
		}
		if rt.confirmPendingAction != "" {
			target = "#modals-root"
		}

	case "SetAutoChoiceDelay":
		if d, err := strconv.ParseFloat(strings.TrimSpace(form.Get("delay")), 64); err == nil {
			rt.save.AutoChoiceDelay = book.NormalizeAutoChoiceDelay(d)
		}
		target = "none"

	case "CycleAudioSpeed":
		steps := []float64{0.75, 1.0, 1.25, 1.5, 2.0}
		next := 1.0
		for i, st := range steps {
			if st == rt.save.PlaybackSpeed {
				next = steps[(i+1)%len(steps)]
				break
			}
		}
		rt.save.PlaybackSpeed = next
		target = "none"

	case "ToggleAutoChoice":
		rt.save.AutoChoice = !rt.save.AutoChoice
		target = "none"

	case "PromptRewindStep":
		if stepIdx, err := strconv.Atoi(form.Get("step")); err == nil && stepIdx >= 0 && stepIdx < len(rt.save.History) {
			entry := rt.save.History[stepIdx]
			sceneName := entry.SceneName
			if sc := findSceneByID(rt.book, entry.SceneID); sc != nil {
				sceneName = sc.DisplayName()
			}
			if sceneName == "" {
				sceneName = entry.SceneID
			}
			rt.confirmTitle = "Wznów od tej sceny"
			rt.confirmMessage = fmt.Sprintf("Czy na pewno chcesz wznowić opowieść od sceny „%s”? Późniejszy postęp oraz dokonane wybory zostaną bezpowrotnie wymazane.", sceneName)
			rt.confirmPendingAction = "RewindStep"
			rt.confirmPendingParam = strconv.Itoa(stepIdx)
		}
		target = "#modals-root"

	case "PromptResetProgress":
		rt.confirmTitle = "Wymaż zapisany postęp"
		rt.confirmMessage = fmt.Sprintf("Czy na pewno chcesz wyczyścić historię wyborów i zakładkę dla książki „%s” oraz zacząć od pierwszej sceny?", rt.book.Title)
		rt.confirmPendingAction = "ResetProgress"
		rt.confirmPendingParam = rt.book.ID
		target = "#modals-root"

	case "ConfirmModalClose":
		rt.confirmPendingAction = ""
		rt.confirmPendingParam = ""
		target = "#modals-root"

	case "ConfirmModalAccept":
		pending := rt.confirmPendingAction
		param := rt.confirmPendingParam
		rt.confirmPendingAction = ""
		rt.confirmPendingParam = ""
		switch pending {
		case "RewindStep":
			if stepIdx, err := strconv.Atoi(param); err == nil && stepIdx >= 0 && stepIdx < len(rt.save.History) {
				targetSceneID := rt.save.History[stepIdx].SceneID
				if len(rt.save.History[stepIdx].Variables) > 0 {
					rt.save.Variables = book.CloneVariablesMap(rt.save.History[stepIdx].Variables)
				}
				rt.save.History = rt.save.History[:stepIdx]
				rt.save.CurrentSceneID = targetSceneID
				rt.save.AudioTime = 0
				rt.historyRevealed = false
				rt.focusedHistoryIdx = -1
				toast = "Cofnięto opowieść do wybranej sceny."
			}
		case "ResetProgress":
			startSceneID := rt.book.StartSceneID
			if findSceneByID(rt.book, startSceneID) == nil && len(rt.book.Scenes) > 0 {
				startSceneID = rt.book.Scenes[0].ID
			}
			rt.save.CurrentSceneID = startSceneID
			rt.save.AudioTime = 0
			rt.save.History = nil
			rt.save.Variables = book.DefaultVariablesMap(rt.book)
			if cur := findSceneByID(rt.book, startSceneID); cur != nil && cur.Effects != "" {
				book.ApplyEffects(cur.Effects, rt.save.Variables, rt.book)
			}
			rt.historyRevealed = false
			rt.focusedHistoryIdx = -1
			toast = "Wyczyszczono postęp. Zaczynasz opowieść od początku."
		}
		target = "#workspace"

	case "RevealHistory":
		if len(rt.save.History) > 0 {
			rt.historyRevealed = true
			rt.focusedHistoryIdx = len(rt.save.History) - 1
		}
		target = "none"

	case "FocusHistoryStep":
		if stepIdx, err := strconv.Atoi(form.Get("step")); err == nil && stepIdx >= 0 && stepIdx < len(rt.save.History) {
			rt.historyRevealed = true
			rt.focusedHistoryIdx = stepIdx
		}
		target = "none"

	case "FocusPresent":
		rt.focusedHistoryIdx = -1
		rt.historyRevealed = false
		target = "none"
	}

	var fdpFrames []tpl.FDPFrame
	switch target {
	case "#workspace":
		playerChunk := rt.buildPlayerChunk()
		fdpFrames = append(fdpFrames, tpl.FDPFrame{
			Target:   "#workspace",
			HTMLBody: []byte(rt.renderChunk(playerChunk)),
			Chunk:    playerChunk,
		})
	case "#modals-root":
		confirmChunk := rt.buildConfirmChunk()
		fdpFrames = append(fdpFrames, tpl.FDPFrame{
			Target:   "#modals-root",
			HTMLBody: []byte(rt.renderChunk(confirmChunk)),
			Chunk:    confirmChunk,
		})
	default:
		playerChunk := rt.buildPlayerChunk()
		fdpFrames = append(fdpFrames, tpl.FDPFrame{
			Target: "none",
			Chunk:  playerChunk,
		})
	}
	fdpFrames = append(fdpFrames, tpl.FDPFrame{
		Target: "none",
		Chunk: tpl.TemplateChunk{
			ScriptData: &appShellScript{
				imageless:       rt.book.Imageless,
				imagelessLocked: rt.book.Imageless,
				toast:           toast,
			},
		},
	})

	var out bytes.Buffer
	_ = tpl.WriteFDPFrames(&out, fdpFrames...)
	return out.String()
}

func (r *wasmPlayerRuntime) currentScene() *book.Scene {
	if r.book == nil || len(r.book.Scenes) == 0 {
		return nil
	}
	if r.save != nil {
		if sc := findSceneByID(r.book, r.save.CurrentSceneID); sc != nil {
			return sc
		}
	}
	if sc := findSceneByID(r.book, r.book.StartSceneID); sc != nil {
		return sc
	}
	return r.book.Scenes[0]
}

func (r *wasmPlayerRuntime) executeChoiceTransition(fromScene *book.Scene, choice book.Choice) string {
	if r.book == nil || r.save == nil {
		return ""
	}
	r.save.Variables = book.EnsureSaveVariables(r.book, r.save.Variables)
	if !book.EvaluateCondition(choice.Condition, r.save.Variables, r.book) {
		return ""
	}
	if choice.IsEnding || choice.TargetSceneID == "END" {
		r.confirmTitle = "Zakończenie opowieści"
		r.confirmMessage = "Dotarłeś do zakończenia tej ścieżki. Czy chcesz zresetować postęp i rozpocząć od pierwszej sceny?"
		r.confirmPendingAction = "ResetProgress"
		r.confirmPendingParam = ""
		return ""
	}
	nextScene := findSceneByID(r.book, choice.TargetSceneID)
	if nextScene == nil {
		return "Nie znaleziono docelowej sceny: " + choice.Target
	}

	prevVarsSnapshot := book.CloneVariablesMap(r.save.Variables)
	if choice.Effects != "" {
		book.ApplyEffects(choice.Effects, r.save.Variables, r.book)
	}
	if nextScene.Effects != "" {
		book.ApplyEffects(nextScene.Effects, r.save.Variables, r.book)
	}

	r.save.History = append(r.save.History, book.HistoryEntry{
		Step:          len(r.save.History) + 1,
		SceneID:       fromScene.ID,
		SceneName:     fromScene.Name,
		Chapter:       fromScene.Chapter,
		ChoiceLabel:   choice.Label,
		TargetSceneID: nextScene.ID,
		Variables:     prevVarsSnapshot,
	})
	r.save.CurrentSceneID = nextScene.ID
	r.save.AudioTime = 0
	r.historyRevealed = false
	r.focusedHistoryIdx = -1
	return ""
}

func (r *wasmPlayerRuntime) renderChunk(chunk tpl.TemplateChunk) string {
	if r.templates == nil {
		return ""
	}
	tplName, tplData := chunk.Resolve()
	if tplName == "" {
		return ""
	}
	baseVars := make(map[string]any, 64)
	i18n.PopulateI18nContext(baseVars, i18n.LangPL)
	ctx := tpl.NewContext(baseVars).WithStruct(tplData)
	var buf bytes.Buffer
	if err := tpl.RenderTemplateTo(&buf, tplName, r.templates, ctx); err != nil {
		return ""
	}
	return buf.String()
}

func (r *wasmPlayerRuntime) buildConfirmChunk() tpl.TemplateChunk {
	return tpl.TemplateChunk{
		TemplateData: &confirmTemplate{
			ShowConfirmModal: r.confirmPendingAction != "",
			ConfirmTitle:     r.confirmTitle,
			ConfirmMessage:   r.confirmMessage,
			ConfirmIsDanger:  false,
		},
	}
}

func (r *wasmPlayerRuntime) buildPlayerChunk() tpl.TemplateChunk {
	b := r.book
	v := &playerTemplate{}
	sc := &playerScript{
		sensitivity: "medium",
	}
	if r.save != nil {
		sc.saveHex = hex.EncodeToString(book.EncodeSaveStateBytes(r.save))
	}
	if b == nil || len(b.Scenes) == 0 {
		return tpl.TemplateChunk{TemplateData: v, ScriptData: sc}
	}
	curScene := r.currentScene()
	if curScene == nil {
		curScene = b.Scenes[0]
	}

	isScreenplay := b.IsScreenplay()
	normStd := b.NormalizedStandard()
	imageless := b.Imageless
	v.ImagelessMode = imageless
	v.IsScreenplayMode = isScreenplay
	sc.bookId = b.ID
	sc.imagelessMode = imageless

	histCount := len(r.save.History)
	r.save.Variables = book.EnsureSaveVariables(b, r.save.Variables)
	curVars := r.save.Variables
	v.HasHistory = histCount > 0

	lastImageURL := ""
	if histCount > 0 {
		histChunks := make([]tpl.TemplateChunk, 0, histCount)
		histTicks := make([]string, 0, histCount)
		for _, entry := range r.save.History {
			pastScene := findSceneByID(b, entry.SceneID)
			if pastScene == nil {
				continue
			}
			if !imageless && pastScene.ImageURL != "" {
				lastImageURL = pastScene.ImageURL
			}
			pastVars := entry.Variables
			if len(pastVars) == 0 {
				pastVars = curVars
			}
			histChunks = append(histChunks, buildWasmSceneChunk(b, pastScene, pastVars, normStd, imageless, false))
			histTicks = append(histTicks, pastScene.DisplayName())
		}
		v.HistoryScenes = histChunks
		v.HistoryTicks = histTicks
	}

	if !imageless && curScene.ImageURL != "" {
		lastImageURL = curScene.ImageURL
	}
	curHeaderMeta := b.Title
	if curScene.Chapter != "" {
		curHeaderMeta = b.Title + " • " + curScene.Chapter
	}
	curQueue := book.EvaluateSceneAudioQueue(curScene, curVars, b)
	if len(curQueue) == 0 && curScene.AudioURL != "" {
		ts, te := curScene.NormalizedVoiceTrim()
		curQueue = []book.VoiceQueueItem{{
			AudioURL:  curScene.AudioURL,
			TrimStart: ts,
			TrimEnd:   te,
		}}
	}
	curQueueSpec, curQueueDurSec, curQueueDurLbl := book.EncodeSceneAudioQueue(curQueue)
	resumeIdx, resumeOffsetSec := book.ResolveAudioQueueResume(curQueue, curQueueDurSec, r.save.AudioTime)
	v.HeaderMeta = curHeaderMeta
	v.RailTitle = curScene.DisplayName()
	v.HasBgImage = !imageless && lastImageURL != ""
	v.BgImageURL = lastImageURL
	v.CurrentScene = buildWasmSceneChunk(b, curScene, curVars, normStd, imageless, true)
	v.ShowAudioDeck = b.Narrated

	autoChoiceLabel := "Auto-Wybór: WYŁ"
	if r.save.AutoChoice {
		autoChoiceLabel = "Auto-Wybór: WŁ"
	}
	autoDelay := book.NormalizeAutoChoiceDelay(r.save.AutoChoiceDelay)
	speed := r.save.PlaybackSpeed
	if speed <= 0 {
		speed = 1
	}
	if b.Narrated {
		v.AudioDeck = tpl.TemplateChunk{
			TemplateData: &audioDeckTemplate{
				AudioDurationLabel: curQueueDurLbl,
				AudioSpeed:         fmt.Sprintf("%.2g", speed),
				AutoChoiceEnabled:  r.save.AutoChoice,
				AutoChoiceLabel:    autoChoiceLabel,
			},
			ScriptData: &audioDeckScript{
				audioQueue:         curQueueSpec,
				audioTotalDuration: curQueueDurSec,
				resumeIndex:        resumeIdx,
				resumeOffsetSec:    resumeOffsetSec,
				audioSpeed:         speed,
				autoChoiceEnabled:  r.save.AutoChoice,
				autoChoiceLabel:    autoChoiceLabel,
				autoChoiceDelayMs:  int(autoDelay * 1000),
				canAutoAdvance:     !curScene.ExplicitEnd && len(curScene.Choices) > 0,
				noAudioToast:       "Ta scena nie posiada przypisanego pliku audio.",
			},
		}
	}

	visited := make(map[string]bool)
	for _, h := range r.save.History {
		if h.SceneID != "" {
			visited[h.SceneID] = true
		}
	}
	if curScene.ID != "" {
		visited[curScene.ID] = true
	}
	progressPct := (len(visited) * 100) / len(b.Scenes)
	if progressPct < 1 {
		progressPct = 1
	}
	if progressPct > 100 {
		progressPct = 100
	}
	v.BookProgressPct = progressPct
	return tpl.TemplateChunk{
		TemplateData: v,
		ScriptData:   sc,
	}
}

func buildWasmSceneChunk(b *book.Book, sc *book.Scene, vars map[string]string, normStd string, imageless, isPresent bool) tpl.TemplateChunk {
	imgURL := ""
	if !imageless && sc.ImageURL != "" {
		imgURL = sc.ImageURL
	}
	isScreenplay := b.IsScreenplay()
	sceneNum := sc.OrderIndex + 1
	showSceneNum := false
	var blocks []screenplayBlockItem
	if isScreenplay {
		blocks = buildWasmScreenplayBlocks(sc, vars, b, normStd)
		showSceneNum = (normStd == "pl" || normStd == "eu") && sceneNum > 0
	}
	hasMultiple := len(sc.Choices) > 1
	hasTitle := strings.TrimSpace(sc.Name) != ""
	hasImage := imgURL != ""
	isTerminal := len(sc.Choices) == 0 || sc.ExplicitEnd
	var choices []sceneChoiceItem
	if isPresent {
		choices = make([]sceneChoiceItem, 0, len(sc.Choices))
		for _, ch := range sc.Choices {
			condMet := book.EvaluateCondition(ch.Condition, vars, b)
			if !condMet && !ch.ShowWhenLocked {
				continue
			}
			label := ch.Label
			if !hasMultiple {
				label = "Dalej"
			}
			choices = append(choices, sceneChoiceItem{
				Label:          label,
				ShowWhenLocked: !condMet && ch.ShowWhenLocked,
				IsEnding:       ch.IsEnding,
			})
		}
	}
	var scriptData any
	if isPresent {
		scriptData = &sceneScript{
			isPresent:        isPresent,
			isScreenplayMode: isScreenplay,
			hasTitle:         hasTitle,
			hasImage:         hasImage,
			imageUrl:         imgURL,
			isTerminal:       isTerminal,
			isBranching:      hasMultiple,
		}
	}
	return tpl.TemplateChunk{
		TemplateData: &sceneTemplate{
			IsPresent:        isPresent,
			IsScreenplayMode: isScreenplay,
			SceneNum:         sceneNum,
			ShowSceneNum:     showSceneNum,
			Name:             sc.Name,
			Slugline:         sc.Slugline,
			RailTitle:        sc.DisplayName(),
			HasTitle:         hasTitle,
			Prose:            book.EvaluateSceneProseText(sc, vars, b),
			ScreenplayBlocks: blocks,
			HasImage:         hasImage,
			ImageURL:         imgURL,
			IsTerminal:       isTerminal,
			IsBranching:      hasMultiple,
			Choices:          choices,
		},
		ScriptData: scriptData,
	}
}

func buildWasmScreenplayBlocks(sc *book.Scene, state map[string]string, b *book.Book, standard string) []screenplayBlockItem {
	if sc == nil {
		return nil
	}
	evalSc := sc
	if book.HasDynamicProse(sc.Text) {
		clone := *sc
		clone.Text = book.EvaluateProseText(sc.Text, state, b)
		evalSc = &clone
	}
	normStd := book.NormalizeScriptStandard(standard)
	blocks := book.ParseScreenplaySceneBlocks(evalSc, normStd)
	if len(blocks) == 0 {
		return nil
	}
	out := make([]screenplayBlockItem, 0, len(blocks))
	for _, blk := range blocks {
		out = append(out, screenplayBlockItem{
			Kind: string(blk.Kind),
			Text: blk.Text,
		})
	}
	return out
}

func findSceneByID(b *book.Book, id string) *book.Scene {
	if b == nil || id == "" {
		return nil
	}
	for _, sc := range b.Scenes {
		if sc.ID == id || strings.EqualFold(sc.ID, id) {
			return sc
		}
	}
	return nil
}

func applyBlobURLsToBook(b *book.Book, blobs map[string]string) {
	if b == nil {
		return
	}
	if b.CoverFile != "" {
		if u, ok := blobs[strings.ToLower(filepath.ToSlash(b.CoverFile))]; ok {
			b.CoverURL = u
		}
	}
	for _, sc := range b.Scenes {
		if sc.AudioFile != "" && b.Narrated {
			if u, ok := blobs[strings.ToLower(filepath.ToSlash(sc.AudioFile))]; ok {
				sc.AudioURL = u
			}
		}
		for i := range sc.VoiceSections {
			if sc.VoiceSections[i].AudioFile != "" && b.Narrated {
				if u, ok := blobs[strings.ToLower(filepath.ToSlash(sc.VoiceSections[i].AudioFile))]; ok {
					sc.VoiceSections[i].AudioURL = u
				}
			}
		}
		if sc.ImageFile != "" {
			if u, ok := blobs[strings.ToLower(filepath.ToSlash(sc.ImageFile))]; ok {
				sc.ImageURL = u
			}
		}
	}
}

func wasmMediaMime(relPath string) string {
	switch strings.ToLower(filepath.Ext(relPath)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".ogg":
		return "audio/ogg"
	case ".flac":
		return "audio/flac"
	case ".mp3":
		return "audio/mpeg"
	default:
		return "application/octet-stream"
	}
}
