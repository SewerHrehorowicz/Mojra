package book

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseStoryFileAndImplicitNext(t *testing.T) {
	raw := `
chapter: Idziemy do lasu

scene: Mgła
id: mgla_start
Las był cichy, a o 12:00: czekamy przy latarni.
Znak na drzewie pokazywał A \-> B.
W oddali zamajaczyło **światło**.

scene: Rozdroże
id: rozdroze
Ścieżka rozdwaja się przed *starym dębem*.

Skręć w lewo -> strumien_id
Skręć w prawo -> Wzgórze

scene: Strumień
id: strumien_id
end: true
Dotarłeś nad zimny górski potok.

scene: Wzgórze
Wiatr smaga kamienny szczyt.
Koniec wędrówki -> END
`
	b := &Book{
		ID:    "las",
		Title: "Las",
	}
	ParseStoryFile(raw, "story.txt", b)
	if len(b.Scenes) != 4 {
		t.Fatalf("expected 4 scenes, got %d", len(b.Scenes))
	}
	FinalizeBook(b)
	if len(b.Warnings) != 0 {
		t.Fatalf("expected 0 warnings, got: %v", b.Warnings)
	}

	// 1. Check dialogue colon, explicit ID, & escaped arrow in Scene 1
	s0 := b.Scenes[0]
	if s0.ID != "mgla_start" || s0.ExplicitID != "mgla_start" {
		t.Fatalf("expected explicit id mgla_start, got ID=%q ExplicitID=%q", s0.ID, s0.ExplicitID)
	}
	if !strings.Contains(s0.Text, "12:00: czekamy") {
		t.Fatalf("expected dialogue colon to be preserved in text, got: %q", s0.Text)
	}
	if !strings.Contains(s0.Text, "A -> B") {
		t.Fatalf("expected escaped arrow to render as literal ->, got: %q", s0.Text)
	}
	if !strings.Contains(s0.HTML, "<strong>światło</strong>") {
		t.Fatalf("expected bold formatting in HTML, got: %s", s0.HTML)
	}

	// 2. Check implicit Next on Scene 1 (Mgła -> Rozdroże)
	if len(s0.Choices) != 1 || !s0.Choices[0].IsImplicit || s0.Choices[0].TargetSceneID != b.Scenes[1].ID {
		t.Fatalf("expected implicit Next from Mgła to Rozdroże, got: %+v", s0.Choices)
	}

	// 3. Check branching choices on Scene 2 (linking by scene ID "strumien_id" and by scene Name "Wzgórze")
	s1 := b.Scenes[1]
	if len(s1.Choices) != 2 {
		t.Fatalf("expected 2 choices on Rozdroże, got %d", len(s1.Choices))
	}
	if s1.Choices[0].TargetSceneID != "strumien_id" || s1.Choices[1].TargetSceneID != b.Scenes[3].ID {
		t.Fatalf("unexpected branch targets: %+v", s1.Choices)
	}

	// 4. Check timeline row & branch column layout
	if s0.Row != 1 || s0.Col != 1 {
		t.Fatalf("expected Mgła at Row 1 Col 1, got Row %d Col %d", s0.Row, s0.Col)
	}
	if s1.Row != 2 || s1.Col != 1 {
		t.Fatalf("expected Rozdroże at Row 2 Col 1, got Row %d Col %d", s1.Row, s1.Col)
	}
	if b.Scenes[2].Row != 3 || b.Scenes[2].Col != 1 {
		t.Fatalf("expected Strumień at Row 3 Col 1, got Row %d Col %d", b.Scenes[2].Row, b.Scenes[2].Col)
	}
	if b.Scenes[3].Row != 3 || b.Scenes[3].Col != 2 {
		t.Fatalf("expected Wzgórze at Row 3 Col 2, got Row %d Col %d", b.Scenes[3].Row, b.Scenes[3].Col)
	}
}

func TestChapterWithoutExplicitSceneHeader(t *testing.T) {
	raw := `
chapter: Prolog w gospodzie
Deszcz bębnił o szyby starej karczmy.
Wyrusz w drogę -> END
`
	b := &Book{ID: "prolog", Title: "Prolog"}
	ParseStoryFile(raw, "prolog.txt", b)
	if len(b.Scenes) != 1 {
		t.Fatalf("expected 1 auto-created scene under chapter, got %d", len(b.Scenes))
	}
	if b.Scenes[0].Name != "Prolog w gospodzie" || b.Scenes[0].Chapter != "Prolog w gospodzie" {
		t.Fatalf("unexpected scene/chapter name: %+v", b.Scenes[0])
	}
}

func TestSanitizesScriptInjectionInBookText(t *testing.T) {
	raw := `
scene: Próba <script>evil()</script>
To jest akapit z <script>evil()</script> oraz <img src=x onerror=evil()>.
Kliknij <b onload=evil()>tutaj</b> -> END
`
	b := &Book{ID: "sec", Title: "Sec"}
	ParseStoryFile(raw, "story.txt", b)
	if len(b.Scenes) != 1 {
		t.Fatalf("expected 1 scene, got %d", len(b.Scenes))
	}
	html := b.Scenes[0].HTML
	if strings.Contains(html, "<script") || strings.Contains(html, "<img") || strings.Contains(html, "onerror=") || strings.Contains(b.Scenes[0].Name, "<script") {
		t.Fatalf("expected all raw HTML/script tags in story text to be stripped/sanitized, got: %s", html)
	}
}

func TestInsertSceneBetweenAndMediaMatchingNoSVG(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "images"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "voice"), 0755)

	_ = os.WriteFile(filepath.Join(dir, "data.txt"), []byte(`title: Las
author: Autor
narrated: true
start: mgla
`), 0644)

	_ = os.WriteFile(filepath.Join(dir, "story.txt"), []byte(`chapter: Las

scene: Mgła
id: mgla
image: images/mgla_custom.png
voice: voice/mgla_custom.ogg
Pierwsza scena.

scene: Rozdroże
end: true
Druga scena bez obrazu i bez głosu.
`), 0644)

	// Write raster image (.png), audio (.ogg) in subdirectories, and .svg / .wav that MUST be ignored
	_ = os.WriteFile(filepath.Join(dir, "cover.png"), []byte("PNG"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "voice", "mgla_custom.ogg"), []byte("OggS"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "voice", "Rozdroże.wav"), []byte("RIFF"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "images", "mgla_custom.png"), []byte("PNG"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "Rozdroże.svg"), []byte("<svg></svg>"), 0644)

	b, err := LoadBookFromDir(dir)
	if err != nil {
		t.Fatalf("LoadBookFromDir failed: %v", err)
	}
	if b.CoverFile != "cover.png" {
		t.Fatalf("expected cover.png, got %q", b.CoverFile)
	}
	if b.Scenes[0].AudioFile != "voice/mgla_custom.ogg" || b.Scenes[0].ImageFile != "images/mgla_custom.png" {
		t.Fatalf("expected Mgła subdirectory media matched, got audio=%q image=%q", b.Scenes[0].AudioFile, b.Scenes[0].ImageFile)
	}
	if b.Scenes[1].ImageFile != "" || b.Scenes[1].AudioFile != "" {
		t.Fatalf("expected .svg and .wav files in book media to be ignored, got image=%q audio=%q", b.Scenes[1].ImageFile, b.Scenes[1].AudioFile)
	}
	// Since narrated: true and scene 2 ("Rozdroże") has no supported voice file (.wav is ignored), there must be a warning about missing narration!
	if len(b.Warnings) != 1 || !strings.Contains(b.Warnings[0], "Rozdroże") {
		t.Fatalf("expected 1 warning about missing narration for Rozdroże when narrated: true, got: %v", b.Warnings)
	}

	inserted, err := InsertSceneBetween(b, b.Scenes[0].ID, b.Scenes[1].ID, "Leśna Polana")
	if err != nil {
		t.Fatalf("InsertSceneBetween failed: %v", err)
	}
	if len(b.Scenes) != 3 || b.Scenes[1].Name != inserted.Name {
		t.Fatalf("expected 3 scenes with Leśna Polana in the middle, got %d", len(b.Scenes))
	}

	// Reload from disk to verify round-trip serialization (including id:, image:, voice:)
	reloaded, err := LoadBookFromDir(dir)
	if err != nil {
		t.Fatalf("LoadBookFromDir after save failed: %v", err)
	}
	if len(reloaded.Scenes) != 3 || reloaded.Scenes[1].Name != "Leśna Polana" {
		t.Fatalf("expected reloaded book to have 3 scenes with Leśna Polana at index 1, got %+v", reloaded.Scenes)
	}
	if reloaded.Scenes[0].ExplicitID != "mgla" || reloaded.Scenes[0].ExplicitImage != "images/mgla_custom.png" || reloaded.Scenes[0].ExplicitVoice != "voice/mgla_custom.ogg" {
		t.Fatalf("expected explicit headers preserved after save, got: %+v", reloaded.Scenes[0])
	}
}

func TestBinarySaveAndIniSettingsRoundTrip(t *testing.T) {
	savesDir := t.TempDir()

	// 1. Test settings.ini round-trip
	cfg := DefaultAppSettings()
	cfg.Theme = "light"
	cfg.Imageless = true
	cfg.UIScale = "large"
	cfg.PlaybackSpeed = 1.5
	cfg.AutoChoice = true
	cfg.AutoChoiceStrategy = "random"
	if err := SaveAppSettings(savesDir, cfg); err != nil {
		t.Fatalf("SaveAppSettings failed: %v", err)
	}
	loadedCfg := LoadAppSettings(savesDir)
	if loadedCfg.Theme != "light" || !loadedCfg.Imageless || loadedCfg.UIScale != "large" || loadedCfg.PlaybackSpeed != 1.5 || !loadedCfg.AutoChoice || loadedCfg.AutoChoiceStrategy != "random" {
		t.Fatalf("unexpected loaded settings.ini: %+v", loadedCfg)
	}

	// 2. Test binary .sav round-trip & in-place audio time update
	st := &SaveState{
		BookID:             "nie-boj-sie-policji-rzecznej",
		CurrentSceneID:     "s2",
		AudioTime:          12.5,
		PlaybackSpeed:      1.25,
		AutoChoice:         true,
		AutoChoiceStrategy: "first",
		History: []HistoryEntry{
			{
				Step:          1,
				SceneID:       "s1",
				SceneName:     "Wstyd w warsztacie",
				Chapter:       "Wioskowe dziwadło",
				ChoiceLabel:   "Dalej",
				TargetSceneID: "s2",
			},
		},
	}
	if err := WriteSaveState(savesDir, st); err != nil {
		t.Fatalf("WriteSaveState failed: %v", err)
	}
	if err := UpdateSaveAudioTimeInPlace(savesDir, "nie-boj-sie-policji-rzecznej", 48.25, "s1"); err != nil {
		t.Fatalf("UpdateSaveAudioTimeInPlace failed: %v", err)
	}
	loadedSave := LoadSaveState(savesDir, "nie-boj-sie-policji-rzecznej", "s1")
	if loadedSave.CurrentSceneID != "s2" || loadedSave.AudioTime != 48.25 || len(loadedSave.History) != 1 || loadedSave.History[0].SceneName != "Wstyd w warsztacie" {
		t.Fatalf("unexpected loaded binary save: %+v", loadedSave)
	}
}

func TestLoadRepositoryBookAndNextDirective(t *testing.T) {
	b, err := LoadBookFromDir(filepath.Join("..", "..", "books", "nie-boj-sie-policji-rzecznej"))
	if err != nil {
		t.Fatalf("LoadBookFromDir failed: %v", err)
	}
	if b.ID != "nie-boj-sie-policji-rzecznej" || b.Title != "Nie bój się policji rzecznej" {
		t.Fatalf("unexpected book ID/title: id=%q title=%q", b.ID, b.Title)
	}
	if len(b.Warnings) != 0 {
		t.Fatalf("expected 0 warnings in default repository book, got: %v", b.Warnings)
	}
	if len(b.Chapters) != 10 {
		t.Fatalf("expected 10 chapters in default repository book, got %d", len(b.Chapters))
	}
	if len(b.Scenes) != 118 {
		t.Fatalf("expected 118 scenes in default repository book, got %d", len(b.Scenes))
	}
	if len(b.AudioFiles) != 0 {
		t.Fatalf("expected 0 audio/.flac files in embedded repository book, got: %v", b.AudioFiles)
	}
	for _, sc := range b.Scenes {
		if len(sc.Warnings) != 0 {
			t.Fatalf("expected 0 warnings on scene %q, got: %v", sc.ID, sc.Warnings)
		}
		if sc.Unreachable {
			t.Fatalf("expected scene %q in chapter %q to be reachable from start", sc.ID, sc.Chapter)
		}
		if sc.X%EditorGridSize != 0 || sc.Y%EditorGridSize != 0 {
			t.Fatalf("expected scene %q coordinates (%d, %d) to be snapped to %dpx grid", sc.ID, sc.X, sc.Y, EditorGridSize)
		}
		if strings.HasSuffix(strings.ToLower(sc.ExplicitVoice), ".wav") {
			t.Fatalf("expected no .wav references in default book scene %q, got %q", sc.ID, sc.ExplicitVoice)
		}
		if len(sc.Choices) == 1 {
			if sc.Choices[0].Label != "Dalej" || !sc.Choices[0].IsImplicit {
				t.Fatalf("expected single transition on scene %q to always be implicit 'Dalej', got %+v", sc.ID, sc.Choices[0])
			}
		}
	}
}

func TestExternalBooksZeroCopyAndURLDownload(t *testing.T) {
	booksDir := t.TempDir()
	savesDir := t.TempDir()
	externalRoot := t.TempDir()

	// 1. Create an external unpacked book folder outside booksDir
	extBookDir := filepath.Join(externalRoot, "wyspa-mgiel")
	_ = os.MkdirAll(extBookDir, 0755)
	_ = os.WriteFile(filepath.Join(extBookDir, "data.txt"), []byte("title: Wyspa Mgieł\nauthor: Anna Nowak\nnarrated: false\nstart: brzeg\n"), 0644)
	_ = os.WriteFile(filepath.Join(extBookDir, "story.txt"), []byte("chapter: Rozdział I\n\nscene: Brzeg\nid: brzeg\nFale uderzają o skały.\nWejdź w głąb lądu -> latarnia\n\nscene: Latarnia\nid: latarnia\nend: true\nDotarłeś do latarni.\n"), 0644)

	// Verify ResolveDiskBookPaths resolves both directory and direct story.txt path to extBookDir
	resolvedFromFile, err := ResolveDiskBookPaths(filepath.Join(extBookDir, "story.txt"))
	if err != nil || len(resolvedFromFile) != 1 || filepath.Clean(resolvedFromFile[0]) != filepath.Clean(extBookDir) {
		t.Fatalf("expected story.txt path to resolve to parent book dir, got %v (err=%v)", resolvedFromFile, err)
	}

	// 2. Persist external book in settings.ini and verify round-trip
	cfg := DefaultAppSettings()
	cfg.AddExternalBook(extBookDir)
	if err := SaveAppSettings(savesDir, cfg); err != nil {
		t.Fatalf("SaveAppSettings failed: %v", err)
	}
	loadedCfg := LoadAppSettings(savesDir)
	if len(loadedCfg.ExternalBooks) != 1 || filepath.Clean(loadedCfg.ExternalBooks[0]) != filepath.Clean(extBookDir) {
		t.Fatalf("expected 1 external book in settings.ini, got: %+v", loadedCfg.ExternalBooks)
	}

	// Verify zero-copy: booksDir is still completely empty
	entries, _ := os.ReadDir(booksDir)
	if len(entries) != 0 {
		t.Fatalf("expected zero-copy for external disk book (booksDir should be empty), got %d entries", len(entries))
	}

	// 3. Serve a zipped book via HTTP (with a top-level folder prefix and __MACOSX metadata) and download via URL
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	wMac, _ := zw.Create("__MACOSX/ksiega-wiatru/._story.txt")
	_, _ = wMac.Write([]byte("junk"))
	wData, _ := zw.Create("ksiega-wiatru-main/data.txt")
	_, _ = wData.Write([]byte("title: Księga Wiatru\nauthor: Piotr Wiśniewski\nnarrated: false\n"))
	wStory, _ := zw.Create("ksiega-wiatru-main/story.txt")
	_, _ = wStory.Write([]byte("scene: Wichura\nid: wichura\nend: true\nWiatr wieje z północy.\n"))
	_ = zw.Close()

	dlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(zipBuf.Bytes())
	}))
	defer dlServer.Close()

	dlBook, err := DownloadBookFromURL(booksDir, dlServer.URL+"/archive/main.zip")
	if err != nil {
		t.Fatalf("DownloadBookFromURL failed: %v", err)
	}
	if dlBook.Title != "Księga Wiatru" || len(dlBook.Scenes) != 1 || dlBook.Scenes[0].ID != "wichura" {
		t.Fatalf("unexpected downloaded book: %+v", dlBook)
	}

	// 4. Load combined library (URL book in booksDir + external book on disk)
	allBooks, err := LoadLibraryBooks(booksDir, loadedCfg.ExternalBooks)
	if err != nil {
		t.Fatalf("LoadLibraryBooks failed: %v", err)
	}
	if len(allBooks) != 2 {
		t.Fatalf("expected 2 books in combined library, got %d", len(allBooks))
	}
}

func TestUntitledScenesWithExplicitIDAndRoundTrip(t *testing.T) {
	raw := `chapter: Krwawy chrzest

scene: Znak zwiadowcy
id: znak_zwiadowcy
voice: voice/znak_zwiadowcy.ogg
next: pierwsza_szarza
Arvo syczał coś i dawał znaki, żeby się nie ruszali.

id: pierwsza_szarza
voice: voice/pierwsza_szarza.ogg
next: agonia_mattiego
Arvo dał znak, i strzelcy wybiegli, paląc z muszkietów we wskazane miejsca.

scene:
id: agonia_mattiego
voice: voice/agonia_mattiego.ogg
end: true
Cisza nad wodą.
`
	b := &Book{ID: "chrzest", Title: "Chrzest"}
	ParseStoryFile(raw, "story.txt", b)
	FinalizeBook(b)

	if len(b.Scenes) != 3 {
		t.Fatalf("expected 3 scenes, got %d", len(b.Scenes))
	}
	if b.Scenes[0].Name != "Znak zwiadowcy" || b.Scenes[0].ExplicitID != "znak_zwiadowcy" {
		t.Fatalf("unexpected scene 0: %+v", b.Scenes[0])
	}
	if b.Scenes[1].Name != "" || b.Scenes[1].ExplicitID != "pierwsza_szarza" || b.Scenes[1].ID != "pierwsza_szarza" {
		t.Fatalf("expected scene 1 to have empty Name and ExplicitID='pierwsza_szarza', got: %+v", b.Scenes[1])
	}
	if b.Scenes[1].DisplayName() != "pierwsza_szarza" {
		t.Fatalf("expected DisplayName() to fall back to ExplicitID, got %q", b.Scenes[1].DisplayName())
	}
	if b.Scenes[2].Name != "" || b.Scenes[2].ExplicitID != "agonia_mattiego" || b.Scenes[2].ID != "agonia_mattiego" {
		t.Fatalf("expected scene 2 to have empty Name and ExplicitID='agonia_mattiego', got: %+v", b.Scenes[2])
	}
	if b.Scenes[0].Choices[0].TargetSceneID != "pierwsza_szarza" {
		t.Fatalf("expected scene 0 choice to resolve to 'pierwsza_szarza', got %q", b.Scenes[0].Choices[0].TargetSceneID)
	}
	if b.Scenes[1].Choices[0].TargetSceneID != "agonia_mattiego" {
		t.Fatalf("expected scene 1 choice to resolve to 'agonia_mattiego', got %q", b.Scenes[1].Choices[0].TargetSceneID)
	}

	serialized := SerializeScenesForFile(b, "story.txt")
	if strings.Contains(serialized, "scene: Scena") || strings.Count(serialized, "scene:") != 1 {
		t.Fatalf("expected only 1 'scene:' header in serialized output, got:\n%s", serialized)
	}

	b2 := &Book{ID: "chrzest", Title: "Chrzest"}
	ParseStoryFile(serialized, "story.txt", b2)
	FinalizeBook(b2)
	if len(b2.Scenes) != 3 {
		t.Fatalf("expected 3 scenes after round-trip, got %d", len(b2.Scenes))
	}
	if b2.Scenes[1].Name != "" || b2.Scenes[1].ExplicitID != "pierwsza_szarza" {
		t.Fatalf("round-trip lost untitled scene 1: %+v", b2.Scenes[1])
	}
	if b2.Scenes[2].Name != "" || b2.Scenes[2].ExplicitID != "agonia_mattiego" {
		t.Fatalf("round-trip lost untitled scene 2: %+v", b2.Scenes[2])
	}
}

func TestChapterIDAndMultiFileBookWithoutMagicNumbers(t *testing.T) {
	dir := t.TempDir()

	// data.txt specifies start scene "warsztat", which lives in wioskowe_dziwadlo.txt
	// (alphabetically LAST among the three chapter files).
	_ = os.WriteFile(filepath.Join(dir, "data.txt"), []byte(`title: Rzeka
author: Autor
narrated: false
start: warsztat
`), 0644)

	// Chapter 1 file (alphabetically 3rd: wioskowe_dziwadlo.txt)
	_ = os.WriteFile(filepath.Join(dir, "wioskowe_dziwadlo.txt"), []byte(`chapter: Wioskowe dziwadło
id: wioskowe_dziwadlo

scene: Wstyd w warsztacie
id: warsztat
Stało się więc, i Krzyś wstąpił do policji rzecznej.

id: prawy_i_wielki
next: siedem_kanu
Popłynął w końcu Krzyś na poważną akcję.
`), 0644)

	// Chapter 2 file (alphabetically 2nd: w_gore_umierajacej_rzeki.txt) - relies on filename stem for chapter ID
	_ = os.WriteFile(filepath.Join(dir, "w_gore_umierajacej_rzeki.txt"), []byte(`chapter: W górę umierającej rzeki

scene: Siedem kanu
id: siedem_kanu
next: zwiadowca_znikad
Siedem kanu płynęło bezszelestnie.
`), 0644)

	// Chapter 3 file (alphabetically 1st: krew_pod_butelkowcami.txt)
	_ = os.WriteFile(filepath.Join(dir, "krew_pod_butelkowcami.txt"), []byte(`chapter: Krew pod butelkowcami
id: krew_pod_butelkowcami

scene: Zwiadowca znikąd
id: zwiadowca_znikad
end: true
Zwiadowca pojawił się przed nimi.
`), 0644)

	b, err := LoadBookFromDir(dir)
	if err != nil {
		t.Fatalf("LoadBookFromDir failed: %v", err)
	}
	if len(b.Warnings) != 0 {
		t.Fatalf("expected 0 warnings, got: %v", b.Warnings)
	}
	if len(b.Chapters) != 3 {
		t.Fatalf("expected 3 chapters, got %d: %+v", len(b.Chapters), b.Chapters)
	}

	// Verify chapters are ordered by the story graph from start: warsztat, NOT alphabetical filename order
	if b.Chapters[0].Name != "Wioskowe dziwadło" || b.Chapters[0].ExplicitID != "wioskowe_dziwadlo" || b.Chapters[0].FileName() != "wioskowe_dziwadlo.txt" {
		t.Fatalf("unexpected chapter 0: %+v", b.Chapters[0])
	}
	if b.Chapters[1].Name != "W górę umierającej rzeki" || b.Chapters[1].ID != "w_gore_umierajacej_rzeki" || b.Chapters[1].FileName() != "w_gore_umierajacej_rzeki.txt" {
		t.Fatalf("unexpected chapter 1: %+v", b.Chapters[1])
	}
	if b.Chapters[2].Name != "Krew pod butelkowcami" || b.Chapters[2].ExplicitID != "krew_pod_butelkowcami" || b.Chapters[2].FileName() != "krew_pod_butelkowcami.txt" {
		t.Fatalf("unexpected chapter 2: %+v", b.Chapters[2])
	}

	// Verify scenes are ordered by chapter graph order and untitled scene after chapter+id works
	if len(b.Scenes) != 4 {
		t.Fatalf("expected 4 scenes, got %d", len(b.Scenes))
	}
	if b.Scenes[0].ID != "warsztat" || b.Scenes[1].ID != "prawy_i_wielki" || b.Scenes[2].ID != "siedem_kanu" || b.Scenes[3].ID != "zwiadowca_znikad" {
		t.Fatalf("unexpected scene order: %s, %s, %s, %s", b.Scenes[0].ID, b.Scenes[1].ID, b.Scenes[2].ID, b.Scenes[3].ID)
	}

	// Verify round-trip serialization preserves chapter id: header
	serializedCh1 := SerializeScenesForFile(b, "wioskowe_dziwadlo.txt")
	if !strings.Contains(serializedCh1, "chapter: Wioskowe dziwadło\nid: wioskowe_dziwadlo\n\n") {
		t.Fatalf("expected chapter + id header block in serialized output, got:\n%s", serializedCh1)
	}
}

func TestDownloadBookFromURLBlocksSSRFPrivateAndLinkLocalIPs(t *testing.T) {
	booksDir := t.TempDir()
	ssrfTargets := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/book.stbook",
		"http://192.168.1.1/book.stbook",
		"http://172.16.0.1/book.stbook",
		"http://0.0.0.0/book.stbook",
	}
	for _, target := range ssrfTargets {
		_, err := DownloadBookFromURL(booksDir, target)
		if err == nil {
			t.Fatalf("expected SSRF target %s to be blocked, got nil error", target)
		}
		if !strings.Contains(err.Error(), "SSRF") {
			t.Fatalf("expected SSRF error message for %s, got: %v", target, err)
		}
	}
}

func TestNarrationlessAllOrNothingPolicy(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "voice"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "data.txt"), []byte("title: Cicha Księga\nauthor: Autor\nnarrated: false\nstart: s1\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "story.txt"), []byte("chapter: Rozdział 1\n\nscene: Cisza\nid: s1\nvoice: voice/cisza.ogg\nend: true\nTekst bez głosu.\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "voice", "cisza.ogg"), []byte("OggS\x00\x02"), 0644)

	b, err := LoadBookFromDir(dir)
	if err != nil {
		t.Fatalf("LoadBookFromDir failed: %v", err)
	}
	if b.Narrated {
		t.Fatalf("expected Narrated=false")
	}
	if len(b.AudioFiles) != 0 {
		t.Fatalf("expected no AudioFiles when narrated: false, got %v", b.AudioFiles)
	}
	if b.Scenes[0].ExplicitVoice != "" || b.Scenes[0].AudioFile != "" || b.Scenes[0].AudioURL != "" {
		t.Fatalf("expected all voice fields cleared when narrated: false, got ExplicitVoice=%q AudioFile=%q AudioURL=%q", b.Scenes[0].ExplicitVoice, b.Scenes[0].AudioFile, b.Scenes[0].AudioURL)
	}
	serialized := SerializeScenesForFile(b, "story.txt")
	if strings.Contains(serialized, "voice:") {
		t.Fatalf("expected no voice: header serialized when narrated: false, got:\n%s", serialized)
	}
}

func TestOneWayChapterLinksAndSingleEntryPointValidation(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "data.txt"), []byte("title: Jednokierunkowa\nauthor: Autor\nstart: s1\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "rozdzial_1.txt"), []byte(`chapter: Rozdział 1
id: rozdzial_1

scene: Scena 1
id: s1
Pierwsza scena rozdziału 1.
Zostań w rozdziale -> s2
Idź do rozdziału 2 -> chapter:rozdzial_2
Idź do rozdziału 3 -> chapter:rozdzial_3

scene: Scena 2
id: s2
end: true
Druga scena rozdziału 1.
`), 0644)
	_ = os.WriteFile(filepath.Join(dir, "rozdzial_2.txt"), []byte(`chapter: Rozdział 2
id: rozdzial_2

scene: Wejście R2
id: r2_entry
next: r2_mid
Scena wejściowa rozdziału 2.

scene: Środek R2
id: r2_mid
next: chapter:rozdzial_3
Scena środkowa rozdziału 2.
`), 0644)
	_ = os.WriteFile(filepath.Join(dir, "rozdzial_3.txt"), []byte(`chapter: Rozdział 3
id: rozdzial_3

scene: Wejście R3
id: r3_entry
end: true
Finał w rozdziale 3.
`), 0644)

	b, err := LoadBookFromDir(dir)
	if err != nil {
		t.Fatalf("LoadBookFromDir failed: %v", err)
	}
	if len(b.Warnings) != 0 {
		t.Fatalf("expected 0 warnings for valid branching chapter DAG, got: %v", b.Warnings)
	}
	if len(b.Chapters) != 3 {
		t.Fatalf("expected 3 chapters, got %d", len(b.Chapters))
	}
	if b.Chapters[0].StartSceneID != "s1" || b.Chapters[1].StartSceneID != "r2_entry" || b.Chapters[2].StartSceneID != "r3_entry" {
		t.Fatalf("unexpected chapter entry scenes: ch0=%q ch1=%q ch2=%q",
			b.Chapters[0].StartSceneID, b.Chapters[1].StartSceneID, b.Chapters[2].StartSceneID)
	}

	s1 := b.Scenes[0]
	if s1.Choices[0].IsChapterLink || s1.Choices[0].TargetSceneID != "s2" {
		t.Fatalf("expected choice 0 to be same-chapter scene link to s2, got: %+v", s1.Choices[0])
	}
	if !s1.Choices[1].IsChapterLink || s1.Choices[1].TargetChapterID != "rozdzial_2" || s1.Choices[1].TargetSceneID != "r2_entry" {
		t.Fatalf("expected choice 1 to be chapter link to rozdzial_2 (r2_entry), got: %+v", s1.Choices[1])
	}
	if !s1.Choices[2].IsChapterLink || s1.Choices[2].TargetChapterID != "rozdzial_3" || s1.Choices[2].TargetSceneID != "r3_entry" {
		t.Fatalf("expected choice 2 to be chapter link to rozdzial_3 (r3_entry), got: %+v", s1.Choices[2])
	}

	// Verify CanLinkChapters blocks backward chapter links (rozdzial_3 -> rozdzial_2 and rozdzial_2 -> rozdzial_1)
	if ok, _ := b.CanLinkChapters("rozdzial_3", "rozdzial_2"); ok {
		t.Fatalf("expected backward link rozdzial_3 -> rozdzial_2 to be rejected")
	}
	if ok, _ := b.CanLinkChapters("rozdzial_2", "rozdzial_1"); ok {
		t.Fatalf("expected backward link to root chapter rozdzial_1 to be rejected")
	}

	// Verify linking to an internal non-entry scene of another chapter or creating a backward chapter loop emits warnings and clears TargetSceneID
	_ = os.WriteFile(filepath.Join(dir, "rozdzial_3.txt"), []byte(`chapter: Rozdział 3
id: rozdzial_3

scene: Wejście R3
id: r3_entry
Finał z próbą powrotu.
Wróć do R2 -> chapter:rozdzial_2
Skocz do środka R2 -> r2_mid
`), 0644)
	bInvalid, err := LoadBookFromDir(dir)
	if err != nil {
		t.Fatalf("LoadBookFromDir invalid failed: %v", err)
	}
	if len(bInvalid.Warnings) < 2 {
		t.Fatalf("expected warnings for backward chapter link and internal cross-chapter scene link, got: %v", bInvalid.Warnings)
	}
}





