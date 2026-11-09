//go:build js && wasm

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"syscall/js"

	"mojra"
	"mojra/internal/book"
	"mojra/internal/gui"
	"mojra/internal/tpl"
)

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

	// Create browser Blob URLs in RAM for all embedded media files.
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

	playerHTML := rt.renderTemplate("player_view")
	modalsHTML := rt.renderTemplate("confirm_modal")

	respVals := url.Values{}
	respVals.Set("X-Book-ID", b.ID)
	respVals.Set("X-Imageless", strconv.FormatBool(b.Imageless))
	respVals.Set("X-Imageless-Locked", strconv.FormatBool(b.Imageless))
	respVals.Set("X-Audio-Speed", fmt.Sprintf("%.2g", rt.save.PlaybackSpeed))
	respVals.Set("X-Auto-Choice", strconv.FormatBool(rt.save.AutoChoice))
	respVals.Set("X-Auto-Delay", fmt.Sprintf("%.2g", book.NormalizeAutoChoiceDelay(rt.save.AutoChoiceDelay)))
	respVals.Set("X-Save-Hex", hex.EncodeToString(book.EncodeSaveStateBytes(rt.save)))
	respVals.Set("html", `<main class="workspace" id="workspace">`+playerHTML+`</main><div class="modals-root" id="modals-root">`+modalsHTML+`</div><div class="toast hidden" id="toast-banner"></div><div class="ui-tooltip hidden" id="ui-tooltip"></div>`)
	return string(gui.EncodeWFPForm(respVals))
}

func wasmAction(_ js.Value, args []js.Value) any {
	if len(args) < 1 || rt.book == nil || rt.save == nil {
		return ""
	}
	rawReq := args[0].String()
	form := make(url.Values, 8)
	if !gui.DecodeWFPFormBytes([]byte(rawReq), form) {
		return ""
	}
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
	tplName := "player_view"
	keepScroll := false
	status := "200"
	var toast string

	switch action {
	case "SaveAudioTime":
		if t, err := strconv.ParseFloat(form.Get("time"), 64); err == nil && t >= 0 {
			rt.save.AudioTime = t
		}
		target = "none"
		tplName = ""
		keepScroll = true
		status = "204"

	case "ChooseBranch":
		if idx, err := strconv.Atoi(form.Get("choice_idx")); err == nil {
			cur := rt.currentScene()
			if cur != nil && idx >= 0 && idx < len(cur.Choices) {
				toast = rt.executeChoiceTransition(cur, cur.Choices[idx])
			}
		}
		if rt.confirmPendingAction != "" {
			target = "#modals-root"
			tplName = "confirm_modal"
			keepScroll = true
		}

	case "AutoAdvanceImplicit":
		cur := rt.currentScene()
		if cur != nil && len(cur.Choices) == 1 && !cur.Choices[0].IsEnding && cur.Choices[0].TargetSceneID != "END" {
			toast = rt.executeChoiceTransition(cur, cur.Choices[0])
		}
		if rt.confirmPendingAction != "" {
			target = "#modals-root"
			tplName = "confirm_modal"
			keepScroll = true
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
			tplName = "confirm_modal"
			keepScroll = true
		}

	case "SetAutoChoiceDelay":
		if d, err := strconv.ParseFloat(strings.TrimSpace(form.Get("delay")), 64); err == nil {
			rt.save.AutoChoiceDelay = book.NormalizeAutoChoiceDelay(d)
		}
		target = "none"
		tplName = ""
		keepScroll = true
		status = "204"

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
		tplName = ""
		keepScroll = true
		status = "204"

	case "ToggleAutoChoice":
		rt.save.AutoChoice = !rt.save.AutoChoice
		target = "none"
		tplName = ""
		keepScroll = true
		status = "204"

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
		tplName = "confirm_modal"
		keepScroll = true

	case "PromptResetProgress":
		rt.confirmTitle = "Wymaż zapisany postęp"
		rt.confirmMessage = fmt.Sprintf("Czy na pewno chcesz wyczyścić historię wyborów i zakładkę dla książki „%s” oraz zacząć od pierwszej sceny?", rt.book.Title)
		rt.confirmPendingAction = "ResetProgress"
		rt.confirmPendingParam = rt.book.ID
		target = "#modals-root"
		tplName = "confirm_modal"
		keepScroll = true

	case "ConfirmModalClose":
		rt.confirmPendingAction = ""
		rt.confirmPendingParam = ""
		target = "#modals-root"
		tplName = "confirm_modal"
		keepScroll = true

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
		tplName = "player_view"

	case "RevealHistory":
		if len(rt.save.History) > 0 {
			rt.historyRevealed = true
			rt.focusedHistoryIdx = len(rt.save.History) - 1
		}
		target = "none"
		tplName = ""
		keepScroll = true
		status = "204"

	case "FocusHistoryStep":
		if stepIdx, err := strconv.Atoi(form.Get("step")); err == nil && stepIdx >= 0 && stepIdx < len(rt.save.History) {
			rt.historyRevealed = true
			rt.focusedHistoryIdx = stepIdx
		}
		target = "none"
		tplName = ""
		keepScroll = true
		status = "204"

	case "FocusPresent":
		rt.focusedHistoryIdx = -1
		rt.historyRevealed = false
		target = "none"
		tplName = ""
		keepScroll = true
		status = "204"
	}

	var renderedHTML string
	if tplName != "" {
		renderedHTML = rt.renderTemplate(tplName)
	}

	respVals := url.Values{}
	respVals.Set("status", status)
	respVals.Set("X-Target", target)
	respVals.Set("X-Keep-Scroll", strconv.FormatBool(keepScroll))
	respVals.Set("X-Imageless", strconv.FormatBool(rt.book.Imageless))
	respVals.Set("X-Imageless-Locked", strconv.FormatBool(rt.book.Imageless))
	respVals.Set("X-Audio-Speed", fmt.Sprintf("%.2g", rt.save.PlaybackSpeed))
	respVals.Set("X-Auto-Choice", strconv.FormatBool(rt.save.AutoChoice))
	respVals.Set("X-Auto-Delay", fmt.Sprintf("%.2g", book.NormalizeAutoChoiceDelay(rt.save.AutoChoiceDelay)))
	respVals.Set("X-Save-Hex", hex.EncodeToString(book.EncodeSaveStateBytes(rt.save)))
	if toast != "" {
		respVals.Set("X-Toast", toast)
	}
	if renderedHTML != "" {
		respVals.Set("html", renderedHTML)
	}
	return string(gui.EncodeWFPForm(respVals))
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

func (r *wasmPlayerRuntime) renderTemplate(name string) string {
	if r.templates == nil {
		return ""
	}
	if _, ok := r.templates[name]; !ok {
		return ""
	}
	vars := make(map[string]any, 24)
	r.populateContext(vars)
	var buf bytes.Buffer
	if err := tpl.RenderTemplateTo(&buf, name, r.templates, tpl.NewContext(vars)); err != nil {
		return ""
	}
	return buf.String()
}

func (r *wasmPlayerRuntime) populateContext(vars map[string]any) {
	gui.PopulateI18nContext(vars, gui.LangPL)
	vars["lang"] = "pl"
	vars["lang_upper"] = "PL"
	vars["is_lang_en"] = false
	b := r.book
	if b == nil || len(b.Scenes) == 0 {
		return
	}
	curScene := r.currentScene()
	if curScene == nil {
		curScene = b.Scenes[0]
	}

	isScreenplay := b.IsScreenplay()
	normStd := b.NormalizedStandard()
	imageless := b.Imageless
	vars["imageless_mode"] = imageless
	vars["is_screenplay_mode"] = isScreenplay
	vars["is_history_preview"] = false

	histCount := len(r.save.History)
	r.save.Variables = book.EnsureSaveVariables(b, r.save.Variables)
	curVars := r.save.Variables
	vars["has_history"] = histCount > 0

	lastImageURL := ""
	if histCount > 0 {
		histList := make([]map[string]any, 0, histCount)
		for idx, entry := range r.save.History {
			pastScene := findSceneByID(b, entry.SceneID)
			if pastScene == nil {
				continue
			}
			if !imageless && pastScene.ImageURL != "" {
				lastImageURL = pastScene.ImageURL
			}
			pastImgURL := ""
			if !imageless && pastScene.ImageURL != "" {
				pastImgURL = pastScene.ImageURL
			}
			pastHeaderMeta := b.Title
			if pastScene.Chapter != "" {
				pastHeaderMeta = b.Title + " • " + pastScene.Chapter
			}
			pastVars := entry.Variables
			if len(pastVars) == 0 {
				pastVars = curVars
			}
			var pastBlocks []map[string]any
			if isScreenplay {
				pastBlocks = buildWasmScreenplayBlocks(pastScene, pastVars, b, pastScene.OrderIndex+1, normStd)
			}
			pastQueue := book.EvaluateSceneAudioQueue(pastScene, pastVars, b)
			pastQueueSpec, _, pastQueueDurLbl := book.EncodeSceneAudioQueue(pastQueue)
			pastAudioURL := pastScene.AudioURL
			pastTrimStart, pastTrimEnd := pastScene.NormalizedVoiceTrim()
			if len(pastQueue) > 0 {
				pastAudioURL = pastQueue[0].AudioURL
				pastTrimStart, pastTrimEnd = book.NormalizedVoiceTrimValues(pastQueue[0].TrimStart, pastQueue[0].TrimEnd)
			}
			histList = append(histList, map[string]any{
				"idx":                  idx,
				"header_meta":          pastHeaderMeta,
				"name":                 pastScene.Name,
				"slugline":             pastScene.Slugline,
				"rail_title":           pastScene.DisplayName(),
				"has_title":            strings.TrimSpace(pastScene.Name) != "",
				"prose":                book.EvaluateSceneProseText(pastScene, pastVars, b),
				"screenplay_blocks":    pastBlocks,
				"has_image":            pastImgURL != "",
				"image_url":            pastImgURL,
				"bg_image_url":         lastImageURL,
				"audio_url":            pastAudioURL,
				"audio_queue":          pastQueueSpec,
				"audio_duration_label": pastQueueDurLbl,
				"audio_trim_start":     fmt.Sprintf("%.4f", pastTrimStart),
				"audio_trim_end":       fmt.Sprintf("%.4f", pastTrimEnd),
			})
		}
		vars["history_items"] = histList
	}

	hasMultipleChoices := len(curScene.Choices) > 1
	curChoices := make([]map[string]any, 0, len(curScene.Choices))
	for idx, ch := range curScene.Choices {
		condMet := book.EvaluateCondition(ch.Condition, curVars, b)
		if !condMet && !ch.ShowWhenLocked {
			continue
		}
		label := ch.Label
		if !hasMultipleChoices {
			label = "Dalej"
		}
		arrow := "»"
		if ch.IsEnding {
			arrow = "KONIEC"
		}
		isImplicitNext := !hasMultipleChoices && !ch.IsEnding && ch.TargetSceneID != "END"
		curChoices = append(curChoices, map[string]any{
			"idx":         idx,
			"label":       label,
			"arrow":       arrow,
			"is_branch":   hasMultipleChoices && !ch.IsEnding,
			"is_implicit": isImplicitNext,
			"is_locked":   !condMet && ch.ShowWhenLocked,
		})
	}

	curImgURL := ""
	if !imageless && curScene.ImageURL != "" {
		curImgURL = curScene.ImageURL
		lastImageURL = curScene.ImageURL
	}
	curHeaderMeta := b.Title
	if curScene.Chapter != "" {
		curHeaderMeta = b.Title + " • " + curScene.Chapter
	}
	var curBlocks []map[string]any
	if isScreenplay {
		curBlocks = buildWasmScreenplayBlocks(curScene, curVars, b, curScene.OrderIndex+1, normStd)
	}
	curQueue := book.EvaluateSceneAudioQueue(curScene, curVars, b)
	curQueueSpec, _, curQueueDurLbl := book.EncodeSceneAudioQueue(curQueue)
	curAudioURL := curScene.AudioURL
	curTrimStart, curTrimEnd := curScene.NormalizedVoiceTrim()
	if len(curQueue) > 0 {
		curAudioURL = curQueue[0].AudioURL
		curTrimStart, curTrimEnd = book.NormalizedVoiceTrimValues(curQueue[0].TrimStart, curQueue[0].TrimEnd)
	}
	vars["current_scene"] = map[string]any{
		"name":                 curScene.Name,
		"slugline":             curScene.Slugline,
		"rail_title":           curScene.DisplayName(),
		"has_title":            strings.TrimSpace(curScene.Name) != "",
		"header_meta":          curHeaderMeta,
		"prose":                book.EvaluateSceneProseText(curScene, curVars, b),
		"screenplay_blocks":    curBlocks,
		"has_image":            curImgURL != "",
		"image_url":            curImgURL,
		"has_bg_image":         !imageless && lastImageURL != "",
		"bg_image_url":         lastImageURL,
		"audio_url":            curAudioURL,
		"audio_queue":          curQueueSpec,
		"audio_duration_label": curQueueDurLbl,
		"audio_trim_start":     fmt.Sprintf("%.4f", curTrimStart),
		"audio_trim_end":       fmt.Sprintf("%.4f", curTrimEnd),
		"is_terminal":          len(curScene.Choices) == 0 || curScene.ExplicitEnd,
		"choices":              curChoices,
	}

	vars["show_audio_deck"] = b.Narrated
	vars["prefetch_media_urls"] = ""

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
	vars["book_progress_pct"] = progressPct
	vars["audio_resume_time"] = fmt.Sprintf("%.1f", r.save.AudioTime)
	vars["audio_speed"] = fmt.Sprintf("%.2g", r.save.PlaybackSpeed)
	vars["auto_choice_enabled"] = r.save.AutoChoice
	if r.save.AutoChoice {
		vars["auto_choice_label"] = "Auto-Wybór: WŁ"
	} else {
		vars["auto_choice_label"] = "Auto-Wybór: WYŁ"
	}
	autoDelay := book.NormalizeAutoChoiceDelay(r.save.AutoChoiceDelay)
	vars["auto_choice_delay"] = fmt.Sprintf("%.2g", autoDelay)
	vars["is_auto_delay_1"] = autoDelay <= 1.25
	vars["is_auto_delay_2"] = autoDelay > 1.25 && autoDelay <= 2.5
	vars["is_auto_delay_3"] = autoDelay > 2.5 && autoDelay <= 4.0
	vars["is_auto_delay_5"] = autoDelay > 4.0

	vars["show_confirm_modal"] = r.confirmPendingAction != ""
	vars["confirm_title"] = r.confirmTitle
	vars["confirm_message"] = r.confirmMessage
	vars["confirm_is_danger"] = false
}

func buildWasmScreenplayBlocks(sc *book.Scene, state map[string]string, b *book.Book, sceneNum int, standard string) []map[string]any {
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
	numbered := normStd == "pl" || normStd == "eu"
	renderedPrimarySlug := false
	out := make([]map[string]any, 0, len(blocks))
	for _, blk := range blocks {
		showNum := false
		if blk.Kind == book.BlockSlugline && !renderedPrimarySlug {
			renderedPrimarySlug = true
			showNum = numbered && sceneNum > 0
		}
		out = append(out, map[string]any{
			"kind":             string(blk.Kind),
			"text":             blk.Text,
			"scene_num":        sceneNum,
			"show_scene_num":   showNum,
			"is_slugline":      blk.Kind == book.BlockSlugline,
			"is_action":        blk.Kind == book.BlockAction,
			"is_character":     blk.Kind == book.BlockCharacter,
			"is_parenthetical": blk.Kind == book.BlockParenthetical,
			"is_dialogue":      blk.Kind == book.BlockDialogue,
			"is_transition":    blk.Kind == book.BlockTransition,
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
