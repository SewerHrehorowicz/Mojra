package book

import (
	"strings"
	"testing"
)

func ValidateConditionExpr(expr string, b *Book) bool {
	return ExplainConditionExprErrorLang(expr, b, "pl") == ""
}

func ValidateEffectsExpr(expr string, b *Book) bool {
	return ExplainEffectsExprErrorLang(expr, b, "pl") == ""
}

func TestVariablesParseSerializeEvaluateAndSaveRoundTrip(t *testing.T) {
	rawData := strings.Join([]string{
		"title: Zatopiona Śluza",
		"author: Tester",
		"start: sluza",
		"enum: frakcje = neutralny, rzeczni, przemytnicy",
		"var: latarka = false",
		"var: klucz_sluzy = false",
		"var: zaufanie = 0",
		"var: naboje = 6",
		"var: frakcja = neutralny",
	}, "\n")

	b := &Book{ID: "sluza-test"}
	ParseDataFile(rawData, b)
	if len(b.Enums) != 1 || b.Enums[0].Name != "frakcje" || len(b.Enums[0].Values) != 3 {
		t.Fatalf("expected 1 enum with 3 values, got %+v", b.Enums)
	}
	if len(b.Variables) != 5 {
		t.Fatalf("expected 5 variables, got %d: %+v", len(b.Variables), b.Variables)
	}
	frakVar := b.FindVariable("frakcja")
	if frakVar == nil || frakVar.Kind != VarKindEnum || frakVar.EnumName != "frakcje" || len(frakVar.Options) != 3 {
		t.Fatalf("expected frakcja linked to enum frakcje, got %+v", frakVar)
	}

	serializedData := SerializeDataFile(b)
	if !strings.Contains(serializedData, "enum: frakcje = neutralny, rzeczni, przemytnicy") ||
		!strings.Contains(serializedData, "var: latarka = false") ||
		!strings.Contains(serializedData, "var: naboje = 6") ||
		!strings.Contains(serializedData, "var: frakcja = neutralny") {
		t.Fatalf("unexpected serialized data.txt:\n%s", serializedData)
	}

	rawStory := strings.Join([]string{
		"scene: sluza",
		"title: Zatopiona śluza",
		"set: +odwiedzono_sluze; zaufanie += 1",
		"",
		"if: zaufanie >= 2",
		"Strażnik patrzy z szacunkiem.",
		"else:",
		"Strażnik patrzy podejrzliwie.",
		"endif:",
		"if: latarka",
		"Snop światła przebija mętną wodę.",
		"else:",
		"W ciemności słyszysz tylko plusk wody.",
		"endif:",
		"",
		"exit: korytarz",
		"text: Otwórz kratę",
		"if: latarka && (klucz_sluzy || zaufanie >= 2)",
		"set: -naboje; frakcja = rzeczni",
		"",
		"exit: korytarz",
		"text: Otwórz właz kluczem",
		"if: ? klucz_sluzy",
		"",
		"exit: ucieczka",
		"text: Wycofaj się w trzciny",
		"set: -zaufanie",
		"",
		"---",
		"",
		"scene: korytarz",
		"title: Korytarz",
		"",
		"Koniec korytarza.",
		"",
		"end: true",
		"",
		"---",
		"",
		"scene: ucieczka",
		"title: Ucieczka",
		"",
		"Uciekłeś w szuwary.",
		"",
		"end: true",
	}, "\n")

	ParseStoryFile(rawStory, "story.playtxt", b)
	if len(b.Scenes) != 3 {
		t.Fatalf("expected 3 scenes, got %d", len(b.Scenes))
	}
	sc := b.Scenes[0]
	if sc.Effects != "+odwiedzono_sluze; zaufanie += 1" {
		t.Fatalf("expected scene effects '+odwiedzono_sluze; zaufanie += 1', got %q", sc.Effects)
	}
	if len(sc.Choices) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(sc.Choices))
	}
	if sc.Choices[0].Condition != "latarka && (klucz_sluzy || zaufanie >= 2)" || sc.Choices[0].Effects != "-naboje; frakcja = rzeczni" || sc.Choices[0].Label != "Otwórz kratę" {
		t.Fatalf("unexpected choice 0: %+v", sc.Choices[0])
	}
	if !sc.Choices[1].ShowWhenLocked || sc.Choices[1].Condition != "klucz_sluzy" {
		t.Fatalf("expected choice 1 to have ShowWhenLocked=true and condition 'klucz_sluzy', got %+v", sc.Choices[1])
	}
	if sc.Choices[2].Condition != "" || sc.Choices[2].Effects != "-zaufanie" {
		t.Fatalf("unexpected choice 2: %+v", sc.Choices[2])
	}

	serializedStory := SerializeScenesForFile(b, "story.playtxt")
	if !strings.Contains(serializedStory, "scene: sluza\ntitle: Zatopiona śluza\nset: +odwiedzono_sluze; zaufanie += 1") ||
		!strings.Contains(serializedStory, "exit: korytarz\ntext: Otwórz kratę\nif: latarka && (klucz_sluzy || zaufanie >= 2)\nset: -naboje; frakcja = rzeczni") ||
		!strings.Contains(serializedStory, "exit: korytarz\ntext: Otwórz właz kluczem\nif: ? klucz_sluzy") {
		t.Fatalf("unexpected serialized story.playtxt:\n%s", serializedStory)
	}

	state := DefaultVariablesMap(b)
	ApplyEffects(sc.Effects, state, b)
	if state["odwiedzono_sluze"] != "true" || state["zaufanie"] != "1" {
		t.Fatalf("unexpected state after scene effects: %+v", state)
	}
	if EvaluateCondition(sc.Choices[0].Condition, state, b) {
		t.Fatalf("expected choice 0 condition to be false when latarka=false")
	}
	outProseFalse := EvaluateProseText(sc.Text, state, b)
	if !strings.Contains(outProseFalse, "podejrzliwie") || !strings.Contains(outProseFalse, "W ciemności słyszysz") || strings.Contains(outProseFalse, "Snop światła") {
		t.Fatalf("unexpected evaluated prose for false state:\n%s", outProseFalse)
	}

	ApplyEffects("+latarka, zaufanie += 1", state, b)
	if !EvaluateCondition(sc.Choices[0].Condition, state, b) {
		t.Fatalf("expected choice 0 condition to be true when latarka=true && zaufanie=2")
	}
	outProseTrue := EvaluateProseText(sc.Text, state, b)
	if !strings.Contains(outProseTrue, "z szacunkiem") || !strings.Contains(outProseTrue, "Snop światła") || strings.Contains(outProseTrue, "W ciemności") {
		t.Fatalf("unexpected evaluated prose for true state:\n%s", outProseTrue)
	}

	// Verify 3 preview modes (normal, highlight, output) + live textarea backdrop highlight segments
	hlSegs := BuildSceneProseHighlightSegments(sc.Text, state, b)
	foundActiveBranchBright := false
	foundInactiveBranchDimmed := false
	for _, seg := range hlSegs {
		txt, _ := seg["text"].(string)
		syn, _ := seg["is_syntax"].(bool)
		if strings.Contains(txt, "Strażnik patrzy z szacunkiem.") && !syn {
			foundActiveBranchBright = true
		}
		if strings.Contains(txt, "Strażnik patrzy podejrzliwie.") && syn {
			foundInactiveBranchDimmed = true
		}
	}
	if !foundActiveBranchBright || !foundInactiveBranchDimmed {
		t.Fatalf("expected active conditional branch to be bright (is_syntax=false) and inactive branch dimmed (is_syntax=true), got %+v", hlSegs)
	}
	normalParas := BuildSceneProsePreviewParagraphs(sc.Text, "normal", state, b)
	highlightParas := BuildSceneProsePreviewParagraphs(sc.Text, "highlight", state, b)
	outputParas := BuildSceneProsePreviewParagraphs(sc.Text, "output", state, b)
	if len(normalParas) == 0 || len(highlightParas) == 0 || len(outputParas) == 0 {
		t.Fatalf("expected non-empty paragraphs for all 3 preview modes")
	}

	// Verify binary .sav persistence of Variables and History step snapshots + UpdateSaveAudioTimeInPlace compatibility
	savesDir := t.TempDir()
	save := &SaveState{
		BookID:         "sluza-test",
		CurrentSceneID: "korytarz",
		AudioTime:      12.5,
		PlaybackSpeed:  1.25,
		Variables:      CloneVariablesMap(state),
		History: []HistoryEntry{
			{
				Step:          1,
				SceneID:       "sluza",
				SceneName:     "Zatopiona śluza",
				ChoiceLabel:   "Otwórz kratę",
				TargetSceneID: "korytarz",
				Variables:     map[string]string{"latarka": "false", "zaufanie": "1"},
			},
		},
	}
	if err := WriteSaveState(savesDir, save); err != nil {
		t.Fatalf("WriteSaveState failed: %v", err)
	}
	if err := UpdateSaveAudioTimeInPlace(savesDir, "sluza-test", 42.75, "korytarz"); err != nil {
		t.Fatalf("UpdateSaveAudioTimeInPlace failed: %v", err)
	}
	loaded := LoadSaveState(savesDir, "sluza-test", "sluza")
	if loaded.AudioTime != 42.75 || loaded.Variables["latarka"] != "true" || loaded.Variables["zaufanie"] != "2" {
		t.Fatalf("unexpected loaded save state: %+v", loaded)
	}
	if len(loaded.History) != 1 || loaded.History[0].Variables["latarka"] != "false" {
		t.Fatalf("expected history step variables snapshot preserved, got %+v", loaded.History)
	}

	if !ValidateConditionExpr("latarka && (klucz_sluzy || zaufanie >= 2)", b) {
		t.Fatalf("expected valid condition expression to pass")
	}
	if ValidateConditionExpr("dasdsd", b) {
		t.Fatalf("expected unknown variable 'dasdsd' in condition to fail validation")
	}
	if !ValidateEffectsExpr("-naboje, frakcja = rzeczni", b) {
		t.Fatalf("expected valid effects expression to pass")
	}
	if ValidateEffectsExpr("+dasdsd", b) || ValidateEffectsExpr("frakcja = nieznana", b) {
		t.Fatalf("expected unknown variable or invalid enum state in effects to fail validation")
	}
}

func TestConditionalVoiceSectionsAutoLinkQueueAndExport(t *testing.T) {
	tmpDir := t.TempDir()
	b, err := CreateNewBook(tmpDir, "Komora Test", "Autor", true, false)
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}
	b.Variables = []VariableDef{
		{Name: "sumienie", Kind: VarKindInt, IntVal: 0},
	}
	sc := b.Scenes[0]
	sc.ExplicitID = "komora_topielcow"
	sc.Name = "Komora topielców"
	sc.Text = strings.Join([]string{
		"Wstęp nad rzeką.",
		"",
		"if: sumienie > 0",
		"Krzysiowi zadrżała dłoń na żelaznym pręcie.",
		"else:",
		"Cofnął stopę i uniósł ciężką żeliwną klapę.",
		"endif:",
		"",
		"Ostatni akapit finałowy.",
	}, "\n")

	secs := ExtractSceneVoiceSections(sc, b)
	if len(secs) != 4 {
		t.Fatalf("expected 4 voice sections (01, 02_if, 03_else, 04), got %d: %+v", len(secs), secs)
	}
	expectedKeys := []string{"01", "02_if", "03_else", "04"}
	expectedTargets := []string{
		"voice/komora_topielcow__01.flac",
		"voice/komora_topielcow__02_if.flac",
		"voice/komora_topielcow__03_else.flac",
		"voice/komora_topielcow__04.flac",
	}
	for i, sec := range secs {
		if sec.Key != expectedKeys[i] || sec.TargetFile != expectedTargets[i] {
			t.Fatalf("section %d: expected key=%q target=%q, got key=%q target=%q",
				i, expectedKeys[i], expectedTargets[i], sec.Key, sec.TargetFile)
		}
	}
	if secs[1].Text != "Krzysiowi zadrżała dłoń na żelaznym pręcie." {
		t.Fatalf("expected isolated text for 02_if, got %q", secs[1].Text)
	}

	// Record sections 01, 02_if, 04 (leaving 03_else unrecorded to verify specific warning)
	sr := 24000
	makePCM := func(durSec int) []byte {
		buf := make([]byte, sr*durSec*2)
		for i := 0; i < sr*durSec; i++ {
			buf[i*2] = 0x40
			buf[i*2+1] = 0x08
		}
		return buf
	}
	if _, err := SaveSceneVoiceSectionPCM(b, sc, "01", makePCM(2), sr, 0, 1); err != nil {
		t.Fatalf("SaveSceneVoiceSectionPCM 01 failed: %v", err)
	}
	if _, err := SaveSceneVoiceSectionPCM(b, sc, "02_if", makePCM(3), sr, 0, 1); err != nil {
		t.Fatalf("SaveSceneVoiceSectionPCM 02_if failed: %v", err)
	}
	if _, err := SaveSceneVoiceSectionPCM(b, sc, "04", makePCM(2), sr, 0, 1); err != nil {
		t.Fatalf("SaveSceneVoiceSectionPCM 04 failed: %v", err)
	}
	sc.SetSectionVoiceMeta("02_if", "voice/komora_topielcow__02_if.flac", 0.10, 0.90)

	FinalizeBook(b)
	foundMissingElseWarn := false
	for _, w := range sc.Warnings {
		if strings.Contains(w, "03 · ELSE") && strings.Contains(w, "voice/komora_topielcow__03_else.flac") {
			foundMissingElseWarn = true
		}
	}
	if !foundMissingElseWarn {
		t.Fatalf("expected parser warning pointing to missing 03_else recording, got: %+v", sc.Warnings)
	}

	// Verify .playtxt serialization and parsing round-trip with inline section voice + trim
	if err := SaveBookScenesToDisk(b); err != nil {
		t.Fatalf("SaveBookScenesToDisk failed: %v", err)
	}
	reloaded, err := LoadBookFromDir(b.RootPath)
	if err != nil {
		t.Fatalf("LoadBookFromDir failed: %v", err)
	}
	relSc := reloaded.Scenes[0]
	if strings.Contains(relSc.Text, "voice:") {
		t.Fatalf("expected inline voice: directives to be stripped from scene prose text, got:\n%s", relSc.Text)
	}
	sec02 := relSc.FindVoiceSection("02_if")
	if sec02 == nil || !sec02.HasAudio || !sec02.HasTrim || sec02.TrimStart < 0.09 || sec02.TrimEnd > 0.91 {
		t.Fatalf("expected reloaded 02_if section to preserve audio and trim [0.10..0.90], got: %+v", sec02)
	}

	// Verify EvaluateSceneAudioQueue for sumienie=1 ([01, 02_if, 04])
	queueTrue := EvaluateSceneAudioQueue(relSc, map[string]string{"sumienie": "1"}, reloaded)
	if len(queueTrue) != 3 || queueTrue[0].Key != "01" || queueTrue[1].Key != "02_if" || queueTrue[2].Key != "04" {
		t.Fatalf("expected active audio queue [01, 02_if, 04] when sumienie=1, got: %+v", queueTrue)
	}
	specStr, totalDur, _ := EncodeSceneAudioQueue(queueTrue)
	if !strings.Contains(specStr, "komora_topielcow__01.flac") || !strings.Contains(specStr, "komora_topielcow__02_if.flac") || totalDur <= 5.0 {
		t.Fatalf("unexpected encoded audio queue spec=%q totalDur=%.2f", specStr, totalDur)
	}

	// Verify CollectProjectVOCues exports separate rows per conditional section (4 for komora_topielcow + 1 for rozdroze = 5)
	cues := CollectProjectVOCues(reloaded)
	if len(cues) != 5 {
		t.Fatalf("expected 5 VO cue rows (4 sections in komora_topielcow + 1 in rozdroze), got %d: %+v", len(cues), cues)
	}
	if cues[0].VoiceFile != "voice/komora_topielcow__01.flac" || cues[1].VoiceFile != "voice/komora_topielcow__02_if.flac" || cues[2].VoiceFile != "voice/komora_topielcow__03_else.flac" || cues[3].VoiceFile != "voice/komora_topielcow__04.flac" {
		t.Fatalf("unexpected VO cue voice files: %+v", cues)
	}

	// Verify STBC compilation packs all section .flac files
	mediaMap, err := CollectBookMediaBytes(reloaded)
	if err != nil {
		t.Fatalf("CollectBookMediaBytes failed: %v", err)
	}
	cipher, key, nonce, tag, perm, err := CompileEncryptedSTBC(reloaded, mediaMap)
	if err != nil {
		t.Fatalf("CompileEncryptedSTBC failed: %v", err)
	}
	decBook, decMedia, err := DecodeEncryptedSTBC(cipher, key, nonce, tag, perm)
	if err != nil {
		t.Fatalf("DecodeEncryptedSTBC failed: %v", err)
	}
	for _, k := range []string{"voice/komora_topielcow__01.flac", "voice/komora_topielcow__02_if.flac", "voice/komora_topielcow__04.flac"} {
		if len(decMedia[k]) == 0 {
			t.Fatalf("expected STBC media to contain %q", k)
		}
	}
	if len(decBook.Scenes[0].VoiceSections) != 4 {
		t.Fatalf("expected decoded STBC scene to have 4 voice sections, got %d", len(decBook.Scenes[0].VoiceSections))
	}
}
