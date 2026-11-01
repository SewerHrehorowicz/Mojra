package book

import (
	"bytes"
	"strings"
	"testing"
)

func TestScreenplayParserAndRoundTrip(t *testing.T) {
	b := &Book{ID: "test-script"}
	dataRaw := "title: Nocny Pociąg\nauthor: Jan Kowalski\ndescription: Dramat psychologiczny\nmode: screenplay\nstandard: pl\nstart: sc1\n"
	ParseDataFile(dataRaw, b)

	if !b.IsScreenplay() || b.NormalizedMode() != "screenplay" {
		t.Fatalf("expected screenplay mode, got %q", b.Mode)
	}
	if b.NormalizedStandard() != "pl" {
		t.Fatalf("expected standard pl, got %q", b.ScriptStandard)
	}

	storyRaw := `chapter: Akt I
id: akt_1

scene: Przedział
id: sc1
slugline: WN. PRZEDZIAŁ POCIĄGU - NOC
synopsis: Bohaterowie rozmawiają w jadącym pociągu.
next: sc2
Pociąg pędzi przez ulewę. Za oknem migają światła semaforów.

ŁUKASZ (OFF)
(szeptem)
Nie zatrzymamy się aż do granicy.

MARTA
Spójrz na zegarek.

> CIĘCIE DO:

scene: Korytarz
id: sc2
end: true
WN. KORYTARZ WAGONU - NOC

ŁUKASZ stoi przy oknie.
`
	ParseStoryFile(storyRaw, "akt_1.txt", b)
	FinalizeBook(b)

	if len(b.Scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(b.Scenes))
	}
	sc1 := b.Scenes[0]
	if sc1.Slugline != "WN. PRZEDZIAŁ POCIĄGU - NOC" {
		t.Fatalf("unexpected sc1 slugline: %q", sc1.Slugline)
	}
	if sc1.Synopsis != "Bohaterowie rozmawiają w jadącym pociągu." {
		t.Fatalf("unexpected sc1 synopsis: %q", sc1.Synopsis)
	}

	sc2 := b.Scenes[1]
	if sc2.Slugline != "WN. KORYTARZ WAGONU - NOC" {
		t.Fatalf("expected auto-extracted slugline on sc2, got %q", sc2.Slugline)
	}

	blocks := ParseScreenplaySceneBlocks(sc1, "pl")
	expectedKinds := []ScreenplayBlockKind{
		BlockSlugline,
		BlockAction,
		BlockCharacter,
		BlockParenthetical,
		BlockDialogue,
		BlockCharacter,
		BlockDialogue,
		BlockTransition,
	}
	if len(blocks) != len(expectedKinds) {
		t.Fatalf("expected %d blocks in sc1, got %d: %+v", len(expectedKinds), len(blocks), blocks)
	}
	for i, kind := range expectedKinds {
		if blocks[i].Kind != kind {
			t.Fatalf("block %d: expected kind %q, got %q (%q)", i, kind, blocks[i].Kind, blocks[i].Text)
		}
	}
	if blocks[2].CharacterName != "ŁUKASZ" || blocks[2].CharacterExt != "(OFF)" {
		t.Fatalf("expected character ŁUKASZ with ext (OFF), got %q %q", blocks[2].CharacterName, blocks[2].CharacterExt)
	}

	// Verify round-trip serialization of data.txt and story file
	serializedData := SerializeDataFile(b)
	if !strings.Contains(serializedData, "mode: screenplay\n") || !strings.Contains(serializedData, "standard: pl\n") {
		t.Fatalf("serialized data.txt missing mode/standard:\n%s", serializedData)
	}
	serializedStory := SerializeScenesForFile(b, "akt_1.txt")
	if !strings.Contains(serializedStory, "slugline: WN. PRZEDZIAŁ POCIĄGU - NOC\n") {
		t.Fatalf("serialized story missing slugline:\n%s", serializedStory)
	}
	if !strings.Contains(serializedStory, "synopsis: Bohaterowie rozmawiają w jadącym pociągu.\n") {
		t.Fatalf("serialized story missing synopsis:\n%s", serializedStory)
	}

	b2 := &Book{ID: "test-script-2"}
	ParseDataFile(serializedData, b2)
	ParseStoryFile(serializedStory, "akt_1.txt", b2)
	FinalizeBook(b2)
	if !b2.IsScreenplay() || len(b2.Scenes) != 2 || b2.Scenes[0].Slugline != sc1.Slugline || b2.Scenes[0].Synopsis != sc1.Synopsis {
		t.Fatalf("round-trip failed: %+v", b2.Scenes[0])
	}
}

func TestScreenplayPaginationAndFormatGuards(t *testing.T) {
	if LinesPerPageForStandard("pl") != 54 || LinesPerPageForStandard("eu") != 54 {
		t.Fatalf("expected 54 lines/page for pl/eu")
	}
	if LinesPerPageForStandard("us") != 55 {
		t.Fatalf("expected 55 lines/page for us")
	}

	// Build a screenplay triggering all 4 scene guards:
	// 1) Invalid slugline (missing prefix/time)
	// 2) Long scene (> 3 pages = > 24/8)
	// 3) Monologue > 8 wrapped lines
	// 4) Character typo (Levenshtein 1 from character with >= 3 cues)
	var longDialogue strings.Builder
	for i := 0; i < 15; i++ {
		longDialogue.WriteString("To jest bardzo długie zdanie monologu wypowiadane bez żadnej przerwy ani didaskaliów. ")
	}
	var longAction strings.Builder
	for i := 0; i < 190; i++ {
		longAction.WriteString("Bohater idzie przez długi korytarz pełen cieni i dymu. ")
	}

	b := &Book{
		ID:             "guards-test",
		Title:          "Test Walidacji",
		Mode:           "screenplay",
		ScriptStandard: "pl",
		StartSceneID:   "s1",
		Scenes: []*Scene{
			{
				ID:         "s1",
				ExplicitID: "s1",
				Name:       "Scena bez nagłówka",
				Chapter:    "Akt I",
				Slugline:   "KAWIARNIA BEZ PREFIKSU",
				Text:       "MAREK\nPierwsza kwestia.\n\nMAREK\nDruga kwestia.\n\nMAREK\nTrzecia kwestia.\n\nMARKE\nKwestia z literówką.\n\nMAREK\n" + longDialogue.String(),
			},
			{
				ID:         "s2",
				ExplicitID: "s2",
				Name:       "Scena ponad trzy strony",
				Chapter:    "Akt II",
				Slugline:   "WN. HALA FABRYCZNA - NOC",
				Text:       longAction.String(),
			},
		},
	}
	FinalizeBook(b)

	joinedWarnings := strings.Join(b.Warnings, "\n")
	if !strings.Contains(joinedWarnings, "nie zaczyna się od WN./PL.") {
		t.Fatalf("expected slugline prefix guard warning, got:\n%s", joinedWarnings)
	}
	if !strings.Contains(joinedWarnings, "zbyt długi ciągły monolog") {
		t.Fatalf("expected monologue guard warning, got:\n%s", joinedWarnings)
	}
	if !strings.Contains(joinedWarnings, "możliwa literówka w nazwie postaci „MARKE”") {
		t.Fatalf("expected character typo guard warning for MARKE vs MAREK, got:\n%s", joinedWarnings)
	}
	if !strings.Contains(joinedWarnings, "przekracza 3 strony") {
		t.Fatalf("expected >3 pages scene length guard warning, got:\n%s", joinedWarnings)
	}
}

func TestScreenplayBreakdownAndGlobalRename(t *testing.T) {
	tmpDir := t.TempDir()
	b, err := CreateNewBookWithMode(tmpDir, " Kryminał Portowy ", "Autor", "screenplay", "pl", false, false)
	if err != nil {
		t.Fatalf("CreateNewBookWithMode failed: %v", err)
	}
	if len(b.Chapters) != 3 || b.Chapters[0].Name != "Akt I" {
		t.Fatalf("expected 3 acts starting with Akt I, got %+v", b.Chapters)
	}

	b.Scenes[0].Slugline = "WN. KOMISARIAT - DZIEŃ"
	b.Scenes[0].Text = "KOMISARZ (45) przegląda akta.\n\nKOMISARZ\nWezwijcie świadka."
	b.Scenes[1].Slugline = "PL. Nabrzeże Portowe - NOC"
	b.Scenes[1].Text = "KOMISARZ idzie wzdłuż dźwigów bez słowa.\n\nŚWIADEK\nWidziałem kuter o północy."
	b.Scenes[2].Slugline = "WN. KOMISARIAT - NOC"
	b.Scenes[2].Text = "KOMISARZ\nZamykamy sprawę."
	FinalizeBook(b)
	if err := SaveBookScenesToDisk(b); err != nil {
		t.Fatalf("SaveBookScenesToDisk failed: %v", err)
	}

	bd := AnalyzeScreenplay(b)
	if len(bd.Locations) != 2 {
		t.Fatalf("expected 2 grouped locations, got %d: %+v", len(bd.Locations), bd.Locations)
	}
	var komisariat *LocationStat
	for i := range bd.Locations {
		if bd.Locations[i].Name == "KOMISARIAT" {
			komisariat = &bd.Locations[i]
			break
		}
	}
	if komisariat == nil || komisariat.SceneCount != 2 || komisariat.DayScenes != 1 || komisariat.NightScenes != 1 {
		t.Fatalf("unexpected KOMISARIAT location stats: %+v", komisariat)
	}

	var komisarz *CharacterStat
	for i := range bd.Characters {
		if bd.Characters[i].Name == "KOMISARZ" {
			komisarz = &bd.Characters[i]
			break
		}
	}
	if komisarz == nil {
		t.Fatalf("expected KOMISARZ in breakdown characters")
	}
	if komisarz.DialogueCues != 2 || len(komisarz.SpeakingSceneIDs) != 2 || len(komisarz.SilentSceneIDs) != 1 {
		t.Fatalf("expected KOMISARZ to have 2 cues, 2 speaking scenes, and 1 silent scene; got %+v", komisarz)
	}

	// Global Character Rename
	renamedCues, err := RenameScreenplayCharacter(b, "KOMISARZ", "INSPEKTOR")
	if err != nil || renamedCues != 2 {
		t.Fatalf("RenameScreenplayCharacter failed: count=%d err=%v", renamedCues, err)
	}

	// Global Location Rename
	renamedLocs, err := RenameScreenplayLocation(b, "KOMISARIAT", "KOMENDA GŁÓWNA")
	if err != nil || renamedLocs != 2 {
		t.Fatalf("RenameScreenplayLocation failed: count=%d err=%v", renamedLocs, err)
	}

	bdAfter := AnalyzeScreenplay(b)
	if bdAfter.Characters[0].Name != "INSPEKTOR" {
		t.Fatalf("expected renamed character INSPEKTOR, got %q", bdAfter.Characters[0].Name)
	}
	foundKomenda := false
	for _, loc := range bdAfter.Locations {
		if loc.Name == "KOMENDA GŁÓWNA" {
			foundKomenda = true
		}
	}
	if !foundKomenda {
		t.Fatalf("expected renamed location KOMENDA GŁÓWNA in breakdown: %+v", bdAfter.Locations)
	}
}

func TestScreenplayPDFAndFountainExport(t *testing.T) {
	tmpDir := t.TempDir()
	b, err := CreateNewBookWithMode(tmpDir, "Zażółć Gęślą Jaźń", "Łukasz Żółć", "screenplay", "pl", false, false)
	if err != nil {
		t.Fatalf("CreateNewBookWithMode: %v", err)
	}

	// Create a long speech in scene 1 to force a page break with (WIĘCEJ) and (CIĄG DALSZY)
	var longLines []string
	for i := 0; i < 60; i++ {
		longLines = append(longLines, "Każdy wiersz dialogu przybliża nas do dolnego marginesu strony scenariusza.")
	}
	b.Scenes[0].Slugline = "WN. STUDIO FILMOWE - DZIEŃ"
	b.Scenes[0].Synopsis = "Scena otwierająca z długim dialogiem."
	b.Scenes[0].Text = "ŁUKASZ\n" + strings.Join(longLines, "\n")
	FinalizeBook(b)

	pdfBytes, err := ExportScreenplayPDF(b)
	if err != nil {
		t.Fatalf("ExportScreenplayPDF failed: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-1.4\n")) {
		t.Fatalf("invalid PDF header")
	}
	if !bytes.Contains(pdfBytes, []byte("/MediaBox [ 0 0 595.28 841.89 ]")) {
		t.Fatalf("expected A4 MediaBox for pl standard")
	}
	if !bytes.Contains(pdfBytes, []byte("/Aogonek")) || !bytes.Contains(pdfBytes, []byte("/Lslash")) {
		t.Fatalf("expected Polish Central European glyph mappings in PDF")
	}
	// Check that (WIĘCEJ) and (CIĄG DALSZY) were emitted across the page split
	encodedWiecej := encodePDFPolishBytes("(WIĘCEJ)")
	if !bytes.Contains(pdfBytes, []byte(pdfLiteralString("(WIĘCEJ)"))) && !bytes.Contains(pdfBytes, encodedWiecej) {
		t.Fatalf("expected (WIĘCEJ) pagination marker in multi-page dialogue PDF")
	}
	if !bytes.Contains(pdfBytes, []byte(pdfLiteralString("ŁUKASZ (CIĄG DALSZY)"))) {
		t.Fatalf("expected ŁUKASZ (CIĄG DALSZY) continuation header in PDF")
	}

	// Switch to US standard and verify US Letter MediaBox and (MORE)/(CONT'D)
	b.ScriptStandard = "us"
	pdfUS, err := ExportScreenplayPDF(b)
	if err != nil {
		t.Fatalf("ExportScreenplayPDF (US) failed: %v", err)
	}
	if !bytes.Contains(pdfUS, []byte("/MediaBox [ 0 0 612.00 792.00 ]")) {
		t.Fatalf("expected US Letter MediaBox for us standard")
	}
	if !bytes.Contains(pdfUS, []byte(pdfLiteralString("(MORE)"))) || !bytes.Contains(pdfUS, []byte(pdfLiteralString("ŁUKASZ (CONT'D)"))) {
		t.Fatalf("expected (MORE) and (CONT'D) in US PDF")
	}

	// Verify .fountain export
	b.ScriptStandard = "pl"
	fountain := ExportScreenplayFountain(b)
	if !strings.Contains(fountain, "Title: Zażółć Gęślą Jaźń\n") {
		t.Fatalf("fountain export missing Title:\n%s", fountain)
	}
	if !strings.Contains(fountain, "# Akt I\n") {
		t.Fatalf("fountain export missing Act heading:\n%s", fountain)
	}
	if !strings.Contains(fountain, "WN. STUDIO FILMOWE - DZIEŃ #1#\n") {
		t.Fatalf("fountain export missing numbered slugline:\n%s", fountain)
	}
	if !strings.Contains(fountain, "= Scena otwierająca z długim dialogiem.\n") {
		t.Fatalf("fountain export missing synopsis:\n%s", fountain)
	}
}
