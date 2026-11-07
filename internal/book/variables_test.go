package book

import (
	"strings"
	"testing"
)

func ValidateConditionExpr(expr string, b *Book) bool {
	return ExplainConditionExprError(expr, b) == ""
}

func ValidateEffectsExpr(expr string, b *Book) bool {
	return ExplainEffectsExprError(expr, b) == ""
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
		"if?: klucz_sluzy",
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
		!strings.Contains(serializedStory, "exit: korytarz\ntext: Otwórz właz kluczem\nif?: klucz_sluzy") {
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
