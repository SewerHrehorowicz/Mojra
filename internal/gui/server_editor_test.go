package gui

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mojra"
	"mojra/internal/book"
)

func TestEditorUndoRedoAndVersionHistory(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Create a book
	respCreate, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"CreateNewBook"},
		"title":  {"Historia Czasu"},
		"author": {"Autor"},
	})
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}
	_ = respCreate.Body.Close()

	if len(srv.ActiveBook.Scenes) != 2 {
		t.Fatalf("expected 2 initial scenes, got %d", len(srv.ActiveBook.Scenes))
	}

	// 2. Add a standalone scene via CreateSceneAt -> records baseline r1 + unnamed step r2
	respAdd, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"CreateSceneAt"},
		"x":      {"600"},
		"y":      {"240"},
	})
	if err != nil {
		t.Fatalf("CreateSceneAt failed: %v", err)
	}
	addBytes, _ := io.ReadAll(respAdd.Body)
	_ = respAdd.Body.Close()
	if respAdd.Header.Get("X-Can-Undo") != "true" || respAdd.Header.Get("X-Can-Redo") != "false" || respAdd.Header.Get("X-History-Count") != "1" {
		t.Fatalf("unexpected history headers after CreateSceneAt: undo=%q redo=%q count=%q",
			respAdd.Header.Get("X-Can-Undo"), respAdd.Header.Get("X-Can-Redo"), respAdd.Header.Get("X-History-Count"))
	}
	if !strings.Contains(string(addBytes), `action="ToggleEditorHistoryPanel"`) ||
		strings.Contains(string(addBytes), `id="btn-editor-undo"`) ||
		strings.Contains(string(addBytes), `id="btn-editor-redo"`) {
		t.Fatalf("expected only history panel button (and no undo/redo buttons) in editor toolbar HTML")
	}

	// 3. Viewport-only SaveEditorState (pan/zoom) must NOT add an undo step
	respPan, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SaveEditorState"},
		"pan_x":  {"120"},
		"pan_y":  {"80"},
		"zoom":   {"1.25"},
	})
	if err != nil {
		t.Fatalf("SaveEditorState pan failed: %v", err)
	}
	_ = respPan.Body.Close()
	if respPan.Header.Get("X-History-Count") != "1" {
		t.Fatalf("expected viewport pan/zoom not to increment X-History-Count, got %q", respPan.Header.Get("X-History-Count"))
	}

	// 4. Node position SaveEditorState MUST record a layout undo step
	newSceneID := srv.ActiveBook.Scenes[len(srv.ActiveBook.Scenes)-1].ID
	respMove, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":   {"SaveEditorState"},
		"scene_id": {newSceneID},
		"x":        {"720"},
		"y":        {"360"},
	})
	if err != nil {
		t.Fatalf("SaveEditorState node move failed: %v", err)
	}
	_ = respMove.Body.Close()
	if respMove.Header.Get("X-History-Count") != "2" || respMove.Header.Get("X-Can-Undo") != "true" {
		t.Fatalf("expected node move to increment X-History-Count to 2, got %q", respMove.Header.Get("X-History-Count"))
	}

	// 5. UndoEditor twice -> back to baseline (2 scenes, CanUndo=false, CanRedo=true)
	respUndo1, err := http.PostForm(ts.URL+"/api/action", url.Values{"action": {"UndoEditor"}})
	if err != nil {
		t.Fatalf("UndoEditor 1 failed: %v", err)
	}
	_ = respUndo1.Body.Close()
	if respUndo1.Header.Get("X-History-Count") != "1" || respUndo1.Header.Get("X-Can-Redo") != "true" {
		t.Fatalf("unexpected headers after UndoEditor 1: count=%q redo=%q", respUndo1.Header.Get("X-History-Count"), respUndo1.Header.Get("X-Can-Redo"))
	}

	respUndo2, err := http.PostForm(ts.URL+"/api/action", url.Values{"action": {"UndoEditor"}})
	if err != nil {
		t.Fatalf("UndoEditor 2 failed: %v", err)
	}
	_ = respUndo2.Body.Close()
	if respUndo2.Header.Get("X-History-Count") != "0" || respUndo2.Header.Get("X-Can-Undo") != "false" || respUndo2.Header.Get("X-Can-Redo") != "true" {
		t.Fatalf("unexpected headers at baseline after UndoEditor 2: count=%q undo=%q redo=%q",
			respUndo2.Header.Get("X-History-Count"), respUndo2.Header.Get("X-Can-Undo"), respUndo2.Header.Get("X-Can-Redo"))
	}
	if len(srv.ActiveBook.Scenes) != 2 {
		t.Fatalf("expected 2 scenes after undoing back to baseline, got %d", len(srv.ActiveBook.Scenes))
	}

	// 6. RedoEditor twice -> back to 3 scenes
	respRedo1, _ := http.PostForm(ts.URL+"/api/action", url.Values{"action": {"RedoEditor"}})
	_ = respRedo1.Body.Close()
	respRedo2, _ := http.PostForm(ts.URL+"/api/action", url.Values{"action": {"RedoEditor"}})
	_ = respRedo2.Body.Close()
	if len(srv.ActiveBook.Scenes) != 3 || respRedo2.Header.Get("X-History-Count") != "2" {
		t.Fatalf("expected 3 scenes and count=2 after RedoEditor twice, got scenes=%d count=%q",
			len(srv.ActiveBook.Scenes), respRedo2.Header.Get("X-History-Count"))
	}

	// 7. ToggleEditorHistoryPanel and SaveHistoryVersion ("Wersja 1.0")
	respPanel, err := http.PostForm(ts.URL+"/api/action", url.Values{"action": {"ToggleEditorHistoryPanel"}})
	if err != nil {
		t.Fatalf("ToggleEditorHistoryPanel failed: %v", err)
	}
	panelBytes, _ := io.ReadAll(respPanel.Body)
	_ = respPanel.Body.Close()
	if !strings.Contains(string(panelBytes), `class="editor-history-panel"`) || !strings.Contains(string(panelBytes), "Stan początkowy") {
		t.Fatalf("expected open history panel with baseline version in HTML")
	}

	respSaveVer, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":       {"SaveHistoryVersion"},
		"version_name": {"Wersja 1.0"},
	})
	if err != nil {
		t.Fatalf("SaveHistoryVersion failed: %v", err)
	}
	saveVerBytes, _ := io.ReadAll(respSaveVer.Body)
	_ = respSaveVer.Body.Close()
	if respSaveVer.Header.Get("X-History-Count") != "0" || respSaveVer.Header.Get("X-Can-Undo") != "false" {
		t.Fatalf("expected SaveHistoryVersion to collapse unnamed edits (count=0, canUndo=false), got count=%q undo=%q",
			respSaveVer.Header.Get("X-History-Count"), respSaveVer.Header.Get("X-Can-Undo"))
	}
	if !strings.Contains(string(saveVerBytes), "Wersja 1.0") {
		t.Fatalf("expected saved version 'Wersja 1.0' in history panel HTML")
	}

	// 8. RestoreHistoryRevision("r1") -> non-destructively restores "Stan początkowy" (2 scenes) on top of stack
	respRestore, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":   {"RestoreHistoryRevision"},
		"revision": {"r1"},
	})
	if err != nil {
		t.Fatalf("RestoreHistoryRevision failed: %v", err)
	}
	restoreBytes, _ := io.ReadAll(respRestore.Body)
	_ = respRestore.Body.Close()
	if len(srv.ActiveBook.Scenes) != 2 {
		t.Fatalf("expected 2 scenes after restoring r1, got %d", len(srv.ActiveBook.Scenes))
	}
	if respRestore.Header.Get("X-Can-Undo") != "true" || respRestore.Header.Get("X-History-Count") != "1" {
		t.Fatalf("expected non-destructive restore of older version to be undoable (count=1, canUndo=true), got count=%q undo=%q",
			respRestore.Header.Get("X-History-Count"), respRestore.Header.Get("X-Can-Undo"))
	}
	if !strings.Contains(string(restoreBytes), "Wersja 1.0") {
		t.Fatalf("expected 'Wersja 1.0' to remain in named versions list after restoring older version r1")
	}
}

func TestHistoryPreviewClickPlayAndEditLock(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Create a book (2 scenes in "Stan początkowy" r1)
	resp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"CreateNewBook"},
		"title":  {"Podróż w Czasie"},
		"author": {"Autor"},
	})
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}
	_ = resp.Body.Close()

	// 2. Add a 3rd scene and save version "Pierwsze wydanie" (r2)
	resp, err = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"CreateSceneAt"},
		"x":      {"480"},
		"y":      {"120"},
	})
	if err != nil {
		t.Fatalf("CreateSceneAt failed: %v", err)
	}
	_ = resp.Body.Close()
	if len(srv.ActiveBook.Scenes) != 3 {
		t.Fatalf("expected 3 scenes before saving version, got %d", len(srv.ActiveBook.Scenes))
	}

	resp, err = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":       {"SaveHistoryVersion"},
		"version_name": {"Pierwsze wydanie"},
	})
	if err != nil {
		t.Fatalf("SaveHistoryVersion failed: %v", err)
	}
	_ = resp.Body.Close()

	// 3. Click on past version "Stan początkowy" (r1) via PreviewHistoryRevision — must NOT revert on disk
	respPrev, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":   {"PreviewHistoryRevision"},
		"revision": {"r1"},
	})
	if err != nil {
		t.Fatalf("PreviewHistoryRevision failed: %v", err)
	}
	prevHTMLBytes, _ := io.ReadAll(respPrev.Body)
	_ = respPrev.Body.Close()
	prevHTML := string(prevHTMLBytes)

	if respPrev.Header.Get("X-History-Preview") != "true" {
		t.Fatalf("expected X-History-Preview=true when previewing past revision, got %q", respPrev.Header.Get("X-History-Preview"))
	}
	if respPrev.Header.Get("X-History-Count") != "0" {
		t.Fatalf("expected previewing past revision NOT to create a revert commit (X-History-Count=0), got %q", respPrev.Header.Get("X-History-Count"))
	}
	if len(srv.ActiveBook.Scenes) != 2 {
		t.Fatalf("expected 2 scenes in previewed r1 book, got %d", len(srv.ActiveBook.Scenes))
	}
	if !strings.Contains(prevHTML, "PODGLĄD") || !strings.Contains(prevHTML, "studio-preview-banner") {
		t.Fatalf("expected preview badge and banner in rendered editor HTML")
	}

	// Verify disk still has 3 scenes (no revert on disk)
	diskBook, err := book.LoadBookFromDir(srv.ActiveBook.RootPath)
	if err != nil {
		t.Fatalf("LoadBookFromDir failed: %v", err)
	}
	if len(diskBook.Scenes) != 3 {
		t.Fatalf("expected disk book to remain at 3 scenes during preview, got %d", len(diskBook.Scenes))
	}

	// 4. Verify editing is locked while in preview mode
	respMut, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"CreateSceneAt"},
		"x":      {"720"},
		"y":      {"120"},
	})
	if err != nil {
		t.Fatalf("CreateSceneAt during preview failed: %v", err)
	}
	_ = respMut.Body.Close()
	if len(srv.ActiveBook.Scenes) != 2 {
		t.Fatalf("expected CreateSceneAt to be blocked during preview, got %d scenes", len(srv.ActiveBook.Scenes))
	}

	respRename, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"RenameBook"},
		"title":  {"Zmieniony Tytuł"},
	})
	if err != nil {
		t.Fatalf("RenameBook during preview failed: %v", err)
	}
	_ = respRename.Body.Close()
	if srv.ActiveBook.Title != "Podróż w Czasie" {
		t.Fatalf("expected RenameBook to be blocked during preview, got title %q", srv.ActiveBook.Title)
	}

	// 5. Play the previewed history state in the player
	respPlay, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":   {"PlayHistoryRevision"},
		"revision": {"r1"},
	})
	if err != nil {
		t.Fatalf("PlayHistoryRevision failed: %v", err)
	}
	playHTMLBytes, _ := io.ReadAll(respPlay.Body)
	_ = respPlay.Body.Close()
	if srv.Settings.ActiveView != "player" {
		t.Fatalf("expected ActiveView=player after PlayHistoryRevision, got %q", srv.Settings.ActiveView)
	}
	if !strings.Contains(string(playHTMLBytes), "player-preview-banner") {
		t.Fatalf("expected player-preview-banner while playing previewed history state")
	}

	// Advance a choice in player while previewing
	respChoose, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":     {"ChooseBranch"},
		"choice_idx": {"0"},
	})
	if err != nil {
		t.Fatalf("ChooseBranch during preview play failed: %v", err)
	}
	_ = respChoose.Body.Close()
	if srv.Save.CurrentSceneID != srv.ActiveBook.Scenes[1].ID {
		t.Fatalf("expected player to advance to %q in preview, got %q", srv.ActiveBook.Scenes[1].ID, srv.Save.CurrentSceneID)
	}

	// Switch back to editor and click active head version ("r3") to exit preview and unlock editing
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SwitchView"},
		"view":   {"editor"},
	})
	respExit, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":   {"PreviewHistoryRevision"},
		"revision": {"r3"},
	})
	if err != nil {
		t.Fatalf("PreviewHistoryRevision(r2) failed: %v", err)
	}
	_ = respExit.Body.Close()
	if respExit.Header.Get("X-History-Preview") != "false" {
		t.Fatalf("expected X-History-Preview=false after clicking active head revision, got %q", respExit.Header.Get("X-History-Preview"))
	}
	if len(srv.ActiveBook.Scenes) != 3 {
		t.Fatalf("expected 3 scenes restored after exiting preview, got %d", len(srv.ActiveBook.Scenes))
	}

	// Verify editing works again after exiting preview
	respAddAfter, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"CreateSceneAt"},
		"x":      {"720"},
		"y":      {"120"},
	})
	if err != nil {
		t.Fatalf("CreateSceneAt after exiting preview failed: %v", err)
	}
	_ = respAddAfter.Body.Close()
	if len(srv.ActiveBook.Scenes) != 4 {
		t.Fatalf("expected 4 scenes after editing unlocked, got %d", len(srv.ActiveBook.Scenes))
	}
}

func TestChapterPortsAndOneWayChapterConnectionsInStudio(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Create a new book -> must create rozdzial_1.playtxt (not story.playtxt)
	_, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"CreateNewBook"},
		"title":  {"Rzeka Czasu"},
		"author": {"Autor"},
	})
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(srv.ActiveBook.RootPath, "rozdzial_1.playtxt")); err != nil {
		t.Fatalf("expected mandatory split chapter file rozdzial_1.playtxt on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(srv.ActiveBook.RootPath, "story.playtxt")); !os.IsNotExist(err) {
		t.Fatalf("expected no story.playtxt in newly created book")
	}

	// 2. Create a second chapter group ("Rozdział 2")
	_, err = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":  {"CreateChapterGroup"},
		"chapter": {"Rozdział 2"},
	})
	if err != nil {
		t.Fatalf("CreateChapterGroup failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(srv.ActiveBook.RootPath, "rozdzial_2.playtxt")); err != nil {
		t.Fatalf("expected rozdzial_2.playtxt on disk after CreateChapterGroup: %v", err)
	}

	// 3. Connect rozdroze (in Rozdział 1) to chapter:rozdzial_2 via ConnectChoice
	rConn, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":     {"ConnectChoice"},
		"from_scene": {"rozdroze"},
		"choice_idx": {"0"},
		"to_scene":   {"chapter:rozdzial_2"},
	})
	if err != nil {
		t.Fatalf("ConnectChoice to chapter:rozdzial_2 failed: %v", err)
	}
	connHTMLBytes, _ := io.ReadAll(rConn.Body)
	_ = rConn.Body.Close()
	connHTML := string(connHTMLBytes)

	if !strings.Contains(connHTML, `id="studio-canvas"`) ||
		!strings.Contains(connHTML, `id="studio-graph-data"`) ||
		!strings.Contains(connHTML, "outs:rozdzial_2") ||
		!strings.Contains(connHTML, "imp_chapter:rozdzial_2") {
		t.Fatalf("expected studio-canvas, chapter outs:rozdzial_2, and imp_chapter:rozdzial_2 in studio HTML, got:\n%s", connHTML)
	}

	// 4. Attempting a backward chapter connection from Rozdział 2 back to Rozdział 1 must be rejected
	ch2EntryID := srv.ActiveBook.Chapters[1].StartSceneID
	rBack, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":     {"ConnectChoice"},
		"from_scene": {ch2EntryID},
		"choice_idx": {"0"},
		"to_scene":   {"chapter:rozdzial_1"},
	})
	if err != nil {
		t.Fatalf("ConnectChoice backward failed: %v", err)
	}
	_ = rBack.Body.Close()
	if !strings.Contains(rBack.Header.Get("X-Toast"), "jednokierunkowa") {
		t.Fatalf("expected backward chapter transition to be rejected with toast, got X-Toast=%q", rBack.Header.Get("X-Toast"))
	}
}

func TestPlayerSequentialTwoHopPrefetchAndFullAudioPreload(t *testing.T) {
	rootDir := t.TempDir()

	bookDir := filepath.Join(rootDir, "books", "prefetch-test")
	_ = os.MkdirAll(filepath.Join(bookDir, "images"), 0755)
	_ = os.MkdirAll(filepath.Join(bookDir, "voice"), 0755)

	_ = os.WriteFile(filepath.Join(bookDir, "book.playtxt"), []byte("title: Prefetch\nauthor: Autor\nnarrated: true\nstart: s1\n"), 0644)
	_ = os.WriteFile(filepath.Join(bookDir, "rozdzial_1.playtxt"), []byte(`chapter: Rozdział 1
id: rozdzial_1

scene: Scena 1
id: s1
image: images/s1.png
voice: voice/s1.ogg
next: s2
Pierwsza scena.

scene: Scena 2
id: s2
image: images/s2.png
voice: voice/s2.ogg
next: s3
Druga scena (Hop 1).

scene: Scena 3
id: s3
image: images/s3.png
voice: voice/s3.ogg
next: s4
Trzecia scena (Hop 2).

scene: Scena 4
id: s4
image: images/s4.png
voice: voice/s4.ogg
end: true
Czwarta scena (Hop 3 - poza kolejką 2 kroków).
`), 0644)

	for _, id := range []string{"s1", "s2", "s3", "s4"} {
		_ = os.WriteFile(filepath.Join(bookDir, "images", id+".png"), []byte("\x89PNG\r\n\x1a\n"), 0644)
		_ = os.WriteFile(filepath.Join(bookDir, "voice", id+".ogg"), []byte("OggS\x00\x02"), 0644)
	}

	srv := NewServer(rootDir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	rOpen, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"OpenBook"},
		"book":   {"prefetch-test"},
	})
	if err != nil {
		t.Fatalf("OpenBook failed: %v", err)
	}
	openBytes, _ := io.ReadAll(rOpen.Body)
	_ = rOpen.Body.Close()
	openHTML := string(openBytes)

	if !strings.Contains(openHTML, `preload="auto"`) {
		t.Fatalf("expected audio#story-audio to use preload=\"auto\" for full current-scene buffering, got:\n%s", openHTML)
	}
	expectedPrefetch := `data-prefetch-urls="/api/books/prefetch-test/media/images/s2.png;/api/books/prefetch-test/media/voice/s2.ogg;/api/books/prefetch-test/media/images/s3.png;/api/books/prefetch-test/media/voice/s3.ogg"`
	if !strings.Contains(openHTML, expectedPrefetch) {
		t.Fatalf("expected 1-2 hop BFS media prefetch attribute %s in player HTML, got:\n%s", expectedPrefetch, openHTML)
	}
	if strings.Contains(openHTML, "/media/images/s4.png") || strings.Contains(openHTML, "/media/voice/s4.ogg") {
		t.Fatalf("expected 3-hop scene s4 media NOT to be in 1-2 hop prefetch queue")
	}

	// Toggling imageless mode should omit image URLs from data-prefetch-urls while keeping 1-2 hop audio URLs
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{"action": {"ToggleImageless"}})
	rRefresh, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SwitchView"},
		"view":   {"player"},
	})
	if err != nil {
		t.Fatalf("SwitchView player failed: %v", err)
	}
	refBytes, _ := io.ReadAll(rRefresh.Body)
	_ = rRefresh.Body.Close()
	refHTML := string(refBytes)

	expectedAudioOnlyPrefetch := `data-prefetch-urls="/api/books/prefetch-test/media/voice/s2.ogg;/api/books/prefetch-test/media/voice/s3.ogg"`
	if !strings.Contains(refHTML, expectedAudioOnlyPrefetch) {
		t.Fatalf("expected audio-only 1-2 hop prefetch when imageless is enabled, got:\n%s", refHTML)
	}
}

func TestUnifiedPortStylesInCSSAndWebGL(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	cssResp, err := http.Get(ts.URL + "/style.css")
	if err != nil {
		t.Fatalf("GET /style.css failed: %v", err)
	}
	cssBytes, _ := io.ReadAll(cssResp.Body)
	_ = cssResp.Body.Close()
	css := string(cssBytes)

	if !strings.Contains(css, ".custom-select-trigger.choice-port-trigger {\n  width: 28px;\n  height: 28px;\n  padding: 0;\n  border: none;\n  background: transparent;\n  display: inline-flex;\n  align-items: center;\n  justify-content: center;\n  color: var(--border-strong);") {
		t.Fatalf("expected .choice-port-trigger to use wire color var(--border-strong) when inactive")
	}
	if !strings.Contains(css, ".choice-port-icon.port-filled {\n  background: currentColor;\n}") {
		t.Fatalf("expected connected .choice-port-icon.port-filled to be filled with currentColor")
	}

	jsResp, err := http.Get(ts.URL + "/app.js")
	if err != nil {
		t.Fatalf("GET /app.js failed: %v", err)
	}
	jsBytes, _ := io.ReadAll(jsResp.Body)
	_ = jsResp.Body.Close()
	js := string(jsBytes)

	if !strings.Contains(js, "function pushPortQuads(x, y, isConnected, isHoveredOrActive, theme, isUnreachable)") ||
		!strings.Contains(js, "const sp = isConnected ? studioGL.uv.portHover : studioGL.uv.portRing;") {
		t.Fatalf("expected WebGL pushPortQuads to render connected ports filled and unconnected ports hollow with wire color when inactive")
	}
}

func TestScreenplayModeEndToEndFlow(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Create a screenplay project in Polish PISF standard
	customScriptDir := filepath.Join(t.TempDir(), "MojeScenariusze")
	resp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":    {"CreateNewBook"},
		"title":     {"Ostatni Seans"},
		"author":    {"Krzysztof"},
		"mode":      {"screenplay"},
		"standard":  {"pl"},
		"save_path": {customScriptDir},
	})
	if err != nil {
		t.Fatalf("CreateNewBook screenplay failed: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	html := string(bodyBytes)

	if srv.ActiveBook == nil || !srv.ActiveBook.IsScreenplay() {
		t.Fatalf("expected ActiveBook in screenplay mode")
	}
	expectedDiskRoot := filepath.Join(customScriptDir, "ostatni-seans")
	if filepath.Clean(srv.ActiveBook.RootPath) != filepath.Clean(expectedDiskRoot) {
		t.Fatalf("expected screenplay saved at %s, got %s", expectedDiskRoot, srv.ActiveBook.RootPath)
	}
	if _, statErr := os.Stat(filepath.Join(expectedDiskRoot, "book.playtxt")); statErr != nil {
		t.Fatalf("expected book.playtxt on disk at %s: %v", expectedDiskRoot, statErr)
	}
	if !srv.ActiveBook.Imageless {
		t.Fatalf("expected screenplay to always have Imageless=true")
	}
	if len(srv.ActiveBook.Scenes) != 3 ||
		!(srv.ActiveBook.Scenes[0].Y < srv.ActiveBook.Scenes[1].Y && srv.ActiveBook.Scenes[1].Y < srv.ActiveBook.Scenes[2].Y) {
		t.Fatalf("expected new screenplay scenes to have spread-out vertical layout, got Y=(%d, %d, %d)",
			srv.ActiveBook.Scenes[0].Y, srv.ActiveBook.Scenes[1].Y, srv.ActiveBook.Scenes[2].Y)
	}
	if !strings.Contains(html, `id="btn-open-export-modal"`) || !strings.Contains(html, `id="btn-editor-breakdown"`) {
		t.Fatalf("expected export modal button and breakdown button in Node Studio toolbar")
	}
	if strings.Contains(html, `id="canvas-script-summary"`) || strings.Contains(html, `id="btn-export-screenplay-pdf"`) || strings.Contains(html, `id="btn-export-screenplay-fountain"`) {
		t.Fatalf("expected screenplay summary and separate PDF/.fountain buttons removed from Node Studio toolbar")
	}
	if strings.Contains(html, "Obsada / Lokacje") {
		t.Fatalf("expected #btn-editor-breakdown to use an SVG icon rather than text label 'Obsada / Lokacje'")
	}
	if strings.Contains(html, "history-unsaved-badge") {
		t.Fatalf("expected no numeric counter badge next to history icon in toolbar")
	}
	if !strings.Contains(html, `placeholder="Tytuł scenariusza..."`) || !strings.Contains(html, `data-hint="Ustawienia scenariusza"`) {
		t.Fatalf("expected screenplay terminology in editor UI")
	}
	if strings.Contains(html, `action="ToggleBookImageless"`) || strings.Contains(html, `action="ToggleBookNarrated"`) || strings.Contains(html, `name="explicit_image"`) || strings.Contains(html, `book-mode-select`) {
		t.Fatalf("expected screenplay editor to have no narration or image mode controls and no book-to-screenplay mode switcher")
	}

	// 2. Open Scene Text Modal and update slugline, synopsis, and dialogue
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"OpenSceneTextModal"},
		"scene":  {"poczatek"},
	})
	saveResp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":   {"SaveSceneTextModal"},
		"scene_id": {"poczatek"},
		"name":     {"Scena otwarcia"},
		"slugline": {"WN. KINO STUDYJNE - NOC"},
		"synopsis": {"Spotkanie w pustej sali kinowej."},
		"text":     {"Projektor rzuca snop światła.\n\nOPERATOR (OFF)\n(spokojnie)\nZaczynamy projekcję."},
	})
	if err != nil {
		t.Fatalf("SaveSceneTextModal failed: %v", err)
	}
	saveBytes, _ := io.ReadAll(saveResp.Body)
	_ = saveResp.Body.Close()
	saveHTML := string(saveBytes)
	if !strings.Contains(saveHTML, `id="modal-slugline-input"`) || !strings.Contains(saveHTML, "WN. KINO STUDYJNE - NOC") {
		t.Fatalf("expected modal slugline input in #modal-scene-text")
	}
	if !strings.Contains(saveHTML, `id="modal-screenplay-format-bar"`) || !strings.Contains(saveHTML, `data-insert-script-block="dialogue"`) || !strings.Contains(saveHTML, `data-insert-script-char="OPERATOR"`) {
		t.Fatalf("expected screenplay format insertion bar and character pills in #modal-scene-text")
	}

	// 3. Switch to Player view from topbar and verify Courier screenplay blocks
	playResp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SwitchView"},
		"view":   {"player"},
	})
	if err != nil {
		t.Fatalf("SwitchView player failed: %v", err)
	}
	playBytes, _ := io.ReadAll(playResp.Body)
	_ = playResp.Body.Close()
	playHTML := string(playBytes)
	if !strings.Contains(playHTML, `class="sp-slugline"`) ||
		!strings.Contains(playHTML, `class="sp-character"`) ||
		!strings.Contains(playHTML, `class="sp-parenthetical"`) ||
		!strings.Contains(playHTML, `class="sp-dialogue"`) {
		t.Fatalf("expected Courier screenplay blocks in player view, got:\n%s", playHTML)
	}

	if strings.Contains(playHTML, `class="book-warnings-box"`) {
		t.Fatalf("expected warnings to only appear in Node Studio, not in player view")
	}

	// 4. Switch back to Editor, open Breakdown panel, and rename character & location
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SwitchView"},
		"view":   {"editor"},
	})
	bdResp, _ := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"ToggleEditorBreakdownPanel"},
	})
	bdBytes, _ := io.ReadAll(bdResp.Body)
	_ = bdResp.Body.Close()
	bdHTML := string(bdBytes)
	if !strings.Contains(bdHTML, "OPERATOR") || !strings.Contains(bdHTML, "KINO STUDYJNE") {
		t.Fatalf("expected OPERATOR and KINO STUDYJNE in breakdown panel")
	}
	if !strings.Contains(bdHTML, `class="breakdown-acts-bar"`) || !strings.Contains(bdHTML, `class="breakdown-metric-cell"`) || strings.Contains(bdHTML, `breakdown-metric-card`) || strings.Contains(bdHTML, `breakdown-item-card`) || strings.Contains(bdHTML, `breakdown-export-bar`) {
		t.Fatalf("expected minimalist line-based breakdown layout without boxed cards or export button bar")
	}
	if strings.Contains(bdHTML, `<span class="history-version-name">OPERATOR</span>`) {
		t.Fatalf("expected character name not to be duplicated as both static span and input value in breakdown panel")
	}

	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":   {"RenameCharacter"},
		"old_name": {"OPERATOR"},
		"new_name": {"PROJEKCJONISTA"},
	})
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":       {"RenameLocation"},
		"old_location": {"KINO STUDYJNE"},
		"new_location": {"KINO ILUZJON"},
	})
	if !strings.Contains(srv.ActiveBook.Scenes[0].Text, "PROJEKCJONISTA") || !strings.Contains(srv.ActiveBook.Scenes[0].Slugline, "KINO ILUZJON") {
		t.Fatalf("expected global rename of character and location to update scene")
	}

	// 5. Create and switch Git-style draft branches
	brResp, _ := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":      {"CreateHistoryBranch"},
		"branch_name": {"wersja-rezysera"},
	})
	brBytes, _ := io.ReadAll(brResp.Body)
	_ = brResp.Body.Close()
	if !strings.Contains(string(brBytes), "wersja-rezysera") {
		t.Fatalf("expected wersja-rezysera in history panel")
	}
	if !strings.Contains(string(brBytes), `id="history-graph-scroll"`) || !strings.Contains(string(brBytes), `id="history-scroll-more-cue"`) {
		t.Fatalf("expected unified history-graph-scroll and history-scroll-more-cue in history panel HTML")
	}
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SwitchHistoryBranch"},
		"branch": {"main"},
	})
	copyResp, copyErr := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":        {"CopyHistoryRevisionToBranch"},
		"revision":      {"r1"},
		"target_branch": {"wersja-rezysera"},
	})
	if copyErr != nil || copyResp.StatusCode != http.StatusOK {
		t.Fatalf("CopyHistoryRevisionToBranch failed: status=%v err=%v", copyResp.StatusCode, copyErr)
	}
	_ = copyResp.Body.Close()
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SwitchHistoryBranch"},
		"branch": {"main"},
	})

	// 5b. PromptCreateHistoryBranch opens confirmation modal and ConfirmModalAccept creates auto-named "wariant-3"
	promptBrResp, _ := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":      {"PromptCreateHistoryBranch"},
		"branch_name": {""},
	})
	promptBrBytes, _ := io.ReadAll(promptBrResp.Body)
	_ = promptBrResp.Body.Close()
	if !strings.Contains(string(promptBrBytes), "Utwórz nowy wariant") || !strings.Contains(string(promptBrBytes), "wariant-3") {
		t.Fatalf("expected confirmation modal for wariant-3, got: %s", string(promptBrBytes))
	}
	autoBrResp, _ := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"ConfirmModalAccept"},
	})
	autoBrBytes, _ := io.ReadAll(autoBrResp.Body)
	_ = autoBrResp.Body.Close()
	if !strings.Contains(string(autoBrBytes), "wariant-3") || !strings.Contains(string(autoBrBytes), `class="history-active-branch-name"`) {
		t.Fatalf("expected auto-named branch wariant-3 and active branch bar in history graph HTML")
	}
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SwitchHistoryBranch"},
		"branch": {"main"},
	})

	// 6. Verify HTTP download endpoints for PDF and Fountain
	pdfResp, err := http.Get(ts.URL + "/api/export/screenplay.pdf")
	if err != nil || pdfResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/export/screenplay.pdf failed: status=%v err=%v", pdfResp.StatusCode, err)
	}
	pdfData, _ := io.ReadAll(pdfResp.Body)
	_ = pdfResp.Body.Close()
	if !strings.HasPrefix(string(pdfData), "%PDF-1.4") {
		t.Fatalf("expected valid PDF from /api/export/screenplay.pdf")
	}

	fountainResp, err := http.Get(ts.URL + "/api/export/screenplay.fountain")
	if err != nil || fountainResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/export/screenplay.fountain failed: status=%v err=%v", fountainResp.StatusCode, err)
	}
	fountainData, _ := io.ReadAll(fountainResp.Body)
	_ = fountainResp.Body.Close()
	if !strings.Contains(string(fountainData), "WN. KINO ILUZJON - NOC #1#") {
		t.Fatalf("expected numbered slugline in .fountain export, got:\n%s", string(fountainData))
	}

	// 7. Verify OpenExportModal / ExportProject modal flow and ExportScreenplayPDF / ExportScreenplayFountain
	openExpModalResp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"OpenExportModal"},
	})
	if err != nil || openExpModalResp.StatusCode != http.StatusOK {
		t.Fatalf("OpenExportModal failed: status=%v err=%v", openExpModalResp.StatusCode, err)
	}
	openExpBytes, _ := io.ReadAll(openExpModalResp.Body)
	_ = openExpModalResp.Body.Close()
	openExpHTML := string(openExpBytes)
	if !srv.ExportModalOpen || !strings.Contains(openExpHTML, `id="modal-export-project"`) || !strings.Contains(openExpHTML, `data-format="playbook"`) || !strings.Contains(openExpHTML, `data-format="wasm"`) || !strings.Contains(openExpHTML, `data-format="pdf"`) || !strings.Contains(openExpHTML, `3 WYBRANE`) {
		t.Fatalf("expected Export Modal with multi-select playbook/wasm/pdf options and 3 WYBRANE counter, got:\n%s", openExpHTML)
	}

	customExportDir := filepath.Join(rootDir, "custom-exports")
	expProjResp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":     {"ExportProject"},
		"format":     {"pdf"},
		"export_dir": {customExportDir},
	})
	if err != nil || expProjResp.StatusCode != http.StatusOK {
		t.Fatalf("ExportProject (pdf) failed: status=%v err=%v", expProjResp.StatusCode, err)
	}
	_ = expProjResp.Body.Close()
	if srv.ExportModalOpen {
		t.Fatalf("expected ExportModalOpen=false after successful ExportProject")
	}
	if _, err := os.Stat(filepath.Join(customExportDir, srv.ActiveBook.ID+".pdf")); err != nil {
		t.Fatalf("expected PDF exported to customExportDir: %v", err)
	}
	if srv.ActiveBook.ExportDir != customExportDir {
		t.Fatalf("expected ActiveBook.ExportDir=%q, got %q", customExportDir, srv.ActiveBook.ExportDir)
	}
	// Subsequent export without export_dir parameter reuses remembered ActiveBook.ExportDir
	expFountainResp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"ExportProject"},
		"format": {"fountain"},
	})
	if err != nil || expFountainResp.StatusCode != http.StatusOK {
		t.Fatalf("ExportProject (fountain) failed: status=%v err=%v", expFountainResp.StatusCode, err)
	}
	_ = expFountainResp.Body.Close()
	if _, err := os.Stat(filepath.Join(customExportDir, srv.ActiveBook.ID+".fountain")); err != nil {
		t.Fatalf("expected Fountain exported to remembered customExportDir: %v", err)
	}

	// Change export_dir via BrowseBookExportFolder
	updatedExportDir := filepath.Join(rootDir, "updated-exports")
	browseExpResp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":     {"BrowseBookExportFolder"},
		"export_dir": {updatedExportDir},
	})
	if err != nil || browseExpResp.StatusCode != http.StatusOK {
		t.Fatalf("BrowseBookExportFolder failed: status=%v err=%v", browseExpResp.StatusCode, err)
	}
	_ = browseExpResp.Body.Close()
	if srv.ActiveBook.ExportDir != updatedExportDir {
		t.Fatalf("expected ActiveBook.ExportDir updated to %q, got %q", updatedExportDir, srv.ActiveBook.ExportDir)
	}

	// 8. Verify multi-select batch export (playbook + wasm + pdf + fountain + fdx + treatment + book_pdf + vo_csv + actor_pdf + playtxt)
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{"action": {"OpenExportModal"}})
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{"action": {"ToggleExportModalAdvanced"}})
	for _, advFmt := range []string{"fountain", "fdx", "treatment", "book_pdf", "vo_csv", "actor_pdf"} {
		_, _ = http.PostForm(ts.URL+"/api/action", url.Values{"action": {"ToggleExportFormat"}, "format": {advFmt}})
	}
	togResp, _ := http.PostForm(ts.URL+"/api/action", url.Values{"action": {"ToggleExportFormat"}, "format": {"playtxt"}})
	togBytes, _ := io.ReadAll(togResp.Body)
	_ = togResp.Body.Close()
	if !strings.Contains(string(togBytes), "10 WYBRANYCH") || !strings.Contains(string(togBytes), "Eksportuj (10)") {
		t.Fatalf("expected 10 WYBRANYCH and Eksportuj (10) after selecting all 10 formats, got:\n%s", string(togBytes))
	}

	batchExportDir := filepath.Join(rootDir, "batch-exports")
	batchResp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":     {"ExportProject"},
		"export_dir": {batchExportDir},
		"standard":   {"eu"},
	})
	if err != nil || batchResp.StatusCode != http.StatusOK {
		t.Fatalf("batch ExportProject failed: status=%v err=%v", batchResp.StatusCode, err)
	}
	_ = batchResp.Body.Close()

	bookID := srv.ActiveBook.ID
	for _, relCheck := range []string{
		bookID + ".playbook",
		bookID + ".pdf",
		bookID + ".fountain",
		bookID + ".fdx",
		bookID + "-treatment.pdf",
		bookID + "-book.pdf",
		bookID + "-vo.csv",
		bookID + "-actors.pdf",
		filepath.Join(bookID+"-playtxt", "book.playtxt"),
		filepath.Join(bookID+"-web", "index.html"),
		filepath.Join(bookID+"-web", "mojra.wasm"),
		filepath.Join(bookID+"-web", "app.js"),
		filepath.Join(bookID+"-web", "style.css"),
	} {
		if _, err := os.Stat(filepath.Join(batchExportDir, relCheck)); err != nil {
			t.Fatalf("expected batch export artifact %s: %v", relCheck, err)
		}
	}

	// Verify importing exported .fdx directly via OpenStudioProjectFromDisk
	impFDXResp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"OpenStudioProjectFromDisk"},
		"path":   {filepath.Join(batchExportDir, bookID+".fdx")},
	})
	if err != nil || impFDXResp.StatusCode != http.StatusOK {
		t.Fatalf("OpenStudioProjectFromDisk (.fdx) failed: status=%v err=%v", impFDXResp.StatusCode, err)
	}
	_ = impFDXResp.Body.Close()
	if srv.ActiveBook == nil || len(srv.ActiveBook.Scenes) == 0 {
		t.Fatalf("expected ActiveBook populated after importing .fdx")
	}

	// Verify exported Web index.html has Splash Screen with play.svg and NO #topbar, and no separate book.playbook file
	webIndexBytes, _ := os.ReadFile(filepath.Join(batchExportDir, bookID+"-web", "index.html"))
	webIndexHTML := string(webIndexBytes)
	if strings.Contains(webIndexHTML, `id="topbar"`) || !strings.Contains(webIndexHTML, `id="wasm-splash"`) || !strings.Contains(webIndexHTML, `data-wasm-play="true"`) {
		t.Fatalf("expected exported Web index.html to render Splash Screen with Play button and no #topbar, got:\n%s", webIndexHTML)
	}
	if _, err := os.Stat(filepath.Join(batchExportDir, bookID+"-web", "book.playbook")); err == nil {
		t.Fatalf("expected NO separate book.playbook file in exported -web directory (must be embedded in mojra.wasm)")
	}
	exportedWasmBytes, err := os.ReadFile(filepath.Join(batchExportDir, bookID+"-web", "mojra.wasm"))
	if err != nil {
		t.Fatalf("read exported mojra.wasm failed: %v", err)
	}
	extractedFromExport, _, err := book.ExtractSTBCFromWasm(exportedWasmBytes)
	if err != nil || extractedFromExport.ID != bookID {
		t.Fatalf("expected exported mojra.wasm to contain valid encrypted STBC book %q, got err=%v", bookID, err)
	}
}

func TestMultiChapterAutoIDAndCollisionOnConnectChoice(t *testing.T) {
	dir := t.TempDir()
	bookDir := filepath.Join(dir, "books", "rzeka-test")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "book.playtxt"), []byte("title: Test Rzeki\nnarrated: false\nimageless: true\nstart: s_root\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Note: w_gore_umierajacej_rzeki.txt sorts alphabetically BEFORE wioskowe_dziwadlo.txt,
	// while wioskowe_dziwadlo.txt is Chapter 1 (root) in graph reachability order.
	ch1Content := `chapter: Wioskowe dziwadło
id: wioskowe_dziwadlo
start: s_root

scene: Patrol na przystani
id: s_root
Tekst startowy.

Wybór 1 -> Nowa scena 119
Wybór 2 -> chapter:w_gore_umierajacej_rzeki

scene: Nowa scena 119
next: s_end1
Tekst sceny 119 bez jawnego id.

scene: Koszary
id: s_end1
end: true
Koniec 1.
`
	ch2Content := `chapter: W górę umierającej rzeki
id: w_gore_umierajacej_rzeki
start: b1_wejscie

scene: Siedem kanu
id: b1_wejscie
Tekst kanu.

Wybór 1 -> b1_nocleg
Wybór 2 -> Nowa scena 120

scene: Nocleg na korzeniach
id: b1_nocleg
end: true
Tekst noclegu.

scene: Nowa scena 120
end: true
Tekst sceny 120 bez jawnego id.
`
	editorContent := `# Mojra — Stan edytora węzłów (.editor.cfg)
pan_x: 0
pan_y: 0
zoom: 1.00

node: s_root -> 720, 96
node: Nowa scena 119 -> 984, 288
node: s_end1 -> 720, 480
node: b1_wejscie -> 552, 1872
node: b1_nocleg -> 552, 2712
node: Nowa scena 120 -> 336, 2568
`
	if err := os.WriteFile(filepath.Join(bookDir, "wioskowe_dziwadlo.playtxt"), []byte(ch1Content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "w_gore_umierajacej_rzeki.playtxt"), []byte(ch2Content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, ".editor.cfg"), []byte(editorContent), 0644); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(dir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	postForm := func(action string, vals url.Values) {
		t.Helper()
		vals.Set("action", action)
		resp, err := http.PostForm(ts.URL+"/api/action", vals)
		if err != nil {
			t.Fatalf("POST %s failed: %v", action, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Fatalf("POST %s unexpected status %d", action, resp.StatusCode)
		}
	}

	postForm("OpenBook", url.Values{"book": {"rzeka-test"}, "target_view": {"editor"}})

	sc119 := srv.findScene("Nowa scena 119")
	sc120 := srv.findScene("Nowa scena 120")
	if sc119 == nil || sc120 == nil {
		t.Fatalf("expected both Nowa scena 119 and Nowa scena 120 to exist")
	}
	if sc119.ID == sc120.ID {
		t.Fatalf("expected distinct auto-IDs for scenes without ExplicitID, got %q", sc119.ID)
	}

	// Simulate dragging Nowa scena 120 to (384, 2616) and then connecting its next step to b1_nocleg
	postForm("SaveEditorState", url.Values{
		"positions": {sc120.ID + ":384,2616"},
	})
	postForm("ConnectChoice", url.Values{
		"from_scene":  {sc120.ID},
		"from_choice": {"-1"},
		"to_scene":    {"b1_nocleg"},
	})

	sc119After := srv.findScene("Nowa scena 119")
	sc120After := srv.findScene("Nowa scena 120")
	if sc119After == nil || sc120After == nil {
		t.Fatalf("expected scenes after ConnectChoice")
	}
	if sc119After.X != 984 || sc119After.Y != 288 {
		t.Fatalf("Nowa scena 119 coordinates were corrupted after ConnectChoice: got (%d, %d), want (984, 288)", sc119After.X, sc119After.Y)
	}
	if sc120After.X != 384 || sc120After.Y != 2616 {
		t.Fatalf("Nowa scena 120 coordinates were not preserved after ConnectChoice: got (%d, %d), want (384, 2616)", sc120After.X, sc120After.Y)
	}

	// Also verify CreateSceneAt near Chapter 1's bounding box for Chapter 2 nudges coordinates away from Chapter 1
	postForm("CreateSceneAt", url.Values{
		"chapter": {"W górę umierającej rzeki"},
		"x":       {"720"},
		"y":       {"288"},
	})
	scNew := srv.findScene("Nowa scena 7")
	if scNew == nil {
		t.Fatalf("expected newly created scene Nowa scena 7")
	}
	// Chapter 1 spans Y: 96-52=44 .. 480+120+44=644; new node in Chapter 2 must not overlap Chapter 1's frame
	if scNew.X >= 720-36-192-24 && scNew.X <= 984+192+36+24 && scNew.Y-52 < 644+24 && scNew.Y+120+44 > 44-24 {
		t.Fatalf("expected CreateSceneAt in Chapter 2 to avoid overlapping Chapter 1 frame, got (%d, %d)", scNew.X, scNew.Y)
	}
}

func TestWFPProtocolAndZeroRepoSavesOrAppState(t *testing.T) {
	// 1. Verify WFP wire framing round-trip with Polish UTF-8 prose, multiline body, Windows paths, and literal \-> arrows
	orig := url.Values{
		"action":     {"SaveScene"},
		"scene_id":   {"poczatek"},
		"name":       {"Wstęp na przystani"},
		"export_dir": {`C:\Users\Autor\Mojra\Exports`},
		"author":     {""},
		"text":       {"Pierwszy akapit z polskimi znakami: Zażółć gęślą jaźń.\n\nDrugi akapit ze strzałką \\-> w treści."},
	}
	wfpFrame := EncodeWFPForm(orig)
	if !bytes.Contains(wfpFrame, []byte("_body=text\n")) || !bytes.Contains(wfpFrame, []byte("\n\nPierwszy akapit")) {
		t.Fatalf("expected WFP frame to place multiline text in raw body after \\n\\n, got:\n%s", string(wfpFrame))
	}
	decoded := make(url.Values)
	if !DecodeWFPFormBytes(wfpFrame, decoded) {
		t.Fatalf("DecodeWFPFormBytes failed on valid frame:\n%s", string(wfpFrame))
	}
	for k := range orig {
		if decoded.Get(k) != orig.Get(k) {
			t.Fatalf("WFP round-trip mismatch for key %q: got %q, want %q", k, decoded.Get(k), orig.Get(k))
		}
		if _, exists := decoded[k]; !exists {
			t.Fatalf("expected key %q (even when empty) to exist in decoded form", k)
		}
	}

	// 2. Verify POST /api/action accepts WFP wire frames directly
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	postWFP := func(vals url.Values) *http.Response {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/action", bytes.NewReader(EncodeWFPForm(vals)))
		if err != nil {
			t.Fatalf("NewRequest failed: %v", err)
		}
		req.Header.Set("Content-Type", "text/x-wfp;charset=UTF-8")
		req.Header.Set("X-Requested-With", "Mojra")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/action WFP failed: %v", err)
		}
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return resp
	}

	postWFP(url.Values{
		"action":   {"CreateNewBook"},
		"title":    {"Kronika WFP"},
		"author":   {"Jan Kowalski"},
		"narrated": {"false"},
	})
	if srv.ActiveBook == nil || srv.ActiveBook.Title != "Kronika WFP" {
		t.Fatalf("expected CreateNewBook via WFP to create active book Kronika WFP, got %+v", srv.ActiveBook)
	}

	postWFP(url.Values{
		"action":   {"SaveScene"},
		"scene_id": {"poczatek"},
		"name":     {"Scena WFP"},
		"text":     {"Linia pierwsza.\nLinia druga ze strzałką \\-> i polskimi znakami ąęćłńóśźż."},
	})
	sc := srv.findScene("poczatek")
	if sc == nil || sc.Name != "Scena WFP" || !strings.Contains(sc.Text, "Linia druga") || !strings.Contains(sc.Text, "->") {
		t.Fatalf("expected SaveScene via WFP to persist multiline text and arrow, got %+v", sc)
	}

	// 3. Verify that a repository directory (containing go.mod) NEVER receives saves/, settings.ini, .pid, or .history/
	fakeRepo := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeRepo, "go.mod"), []byte("module mojra\n"), 0644); err != nil {
		t.Fatalf("failed to write fake go.mod: %v", err)
	}
	repoBookDir := filepath.Join(fakeRepo, "books", "test-book")
	if err := os.MkdirAll(repoBookDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoBookDir, "book.playtxt"), []byte("title: Repo Book\nstart: s1\n"), 0644); err != nil {
		t.Fatalf("WriteFile book.playtxt failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoBookDir, "story.playtxt"), []byte("scene: Scena 1\nid: s1\nend: true\nTekst.\n"), 0644); err != nil {
		t.Fatalf("WriteFile story.playtxt failed: %v", err)
	}

	StopPreviousInstance(fakeRepo)
	repoSrv := NewServerWithOptions(fakeRepo, mojra.AppFS, true)
	_ = repoSrv.saveAppSettings()
	if repoSrv.ActiveBook != nil {
		_ = book.EnsureBookHistoryBaseline(repoSrv.ActiveBook, "s1")
	}

	if _, err := os.Stat(filepath.Join(fakeRepo, "saves")); !os.IsNotExist(err) {
		t.Fatalf("expected zero saves/ directory inside repository root %q", fakeRepo)
	}
	if _, err := os.Stat(filepath.Join(repoBookDir, ".history")); !os.IsNotExist(err) {
		t.Fatalf("expected zero .history/ directory inside repository book %q", repoBookDir)
	}
	if filepath.Clean(repoSrv.SavesDir) != filepath.Clean(book.DefaultSavesDir()) {
		t.Fatalf("expected repo server SavesDir to use DefaultSavesDir %q, got %q", book.DefaultSavesDir(), repoSrv.SavesDir)
	}

	// Also verify the real workspace repository has no saves/ or dist/saves/ directory
	realRepoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err == nil {
		for _, forbiddenRel := range []string{
			"saves",
			filepath.Join("dist", "saves"),
			filepath.Join("books", "nie-boj-sie-policji-rzecznej", ".history"),
		} {
			forbiddenPath := filepath.Join(realRepoRoot, forbiddenRel)
			if _, statErr := os.Stat(forbiddenPath); !os.IsNotExist(statErr) {
				t.Fatalf("forbidden generated state/saves directory found in repository: %s", forbiddenPath)
			}
		}
	}
}

func TestVariablesPanelChoiceTripleStackAndThreeStateScenePreview(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Create a new interactive book in Playbook Studio
	customDir := filepath.Join(t.TempDir(), "TestVariablesBook")
	_, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":    {"CreateNewBook"},
		"title":     {"Zatopiona Śluza"},
		"author":    {"Autor"},
		"mode":      {"book"},
		"save_path": {customDir},
	})
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}

	// 2. Open Variables Panel (#editor-variables-panel) and add bool, int, and enum variables
	resp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"ToggleEditorVariablesPanel"},
	})
	if err != nil {
		t.Fatalf("ToggleEditorVariablesPanel failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), `id="editor-variables-panel"`) || !strings.Contains(string(body), `data-kind="bool"`) || !strings.Contains(string(body), `data-kind="int"`) || !strings.Contains(string(body), `data-kind="enum"`) {
		t.Fatalf("expected #editor-variables-panel to be rendered and visible")
	}

	for _, kind := range []string{"bool", "int", "enum"} {
		_, err = http.PostForm(ts.URL+"/api/action", url.Values{
			"action": {"AddBookVariable"},
			"kind":   {kind},
		})
		if err != nil {
			t.Fatalf("AddBookVariable(%s) failed: %v", kind, err)
		}
	}
	if len(srv.ActiveBook.Variables) != 3 {
		t.Fatalf("expected 3 variables in ActiveBook, got %d", len(srv.ActiveBook.Variables))
	}

	// Rename and configure variables: latarka (bool), zaufanie (int=0), frakcja (enum=neutralny, rzeczni)
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":   {"UpdateBookVariable"},
		"var_idx":  {"0"},
		"var_name": {"latarka"},
	})
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":    {"UpdateBookVariable"},
		"var_idx":   {"1"},
		"var_name":  {"zaufanie"},
		"var_value": {"0"},
	})
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":    {"UpdateBookVariable"},
		"var_idx":   {"2"},
		"var_name":  {"frakcja"},
		"var_value": {"neutralny, rzeczni"},
	})
	// Test adding and removing individual enum options via AddBookEnumOption / RemoveBookEnumOption and per-row enum_opt_* fields
	resp, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":  {"AddBookEnumOption"},
		"var_idx": {"2"},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "var-enum-options-list") || !strings.Contains(string(body), `name="enum_opt_2"`) {
		t.Fatalf("expected AddBookEnumOption to render 3rd vertical enum option row (enum_opt_2)")
	}
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":     {"UpdateBookVariable"},
		"var_idx":    {"2"},
		"var_name":   {"frakcja"},
		"opt_count":  {"3"},
		"enum_opt_0": {"neutralny"},
		"enum_opt_1": {"rzeczni"},
		"enum_opt_2": {"przemytnicy"},
	})
	if len(srv.ActiveBook.Variables[2].Options) != 3 || srv.ActiveBook.Variables[2].Options[2] != "przemytnicy" {
		t.Fatalf("expected 3 enum options ending with przemytnicy, got %+v", srv.ActiveBook.Variables[2].Options)
	}
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":  {"RemoveBookEnumOption"},
		"var_idx": {"2"},
		"opt_idx": {"2"},
	})
	if len(srv.ActiveBook.Variables[2].Options) != 2 {
		t.Fatalf("expected 2 enum options after RemoveBookEnumOption, got %+v", srv.ActiveBook.Variables[2].Options)
	}

	// 3. Add a second choice to the start scene and verify automatic 4-row exit:/label:/if:/set: choice stack + syntax help
	startID := srv.ActiveBook.StartSceneID
	resp, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"AddChoice"},
		"scene":  {startID},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "choice-triple-stack") || !strings.Contains(string(body), `name="choice_target_0"`) || !strings.Contains(string(body), `name="choice_label_0"`) || !strings.Contains(string(body), `name="choice_cond_0"`) || !strings.Contains(string(body), `name="choice_effects_0"`) {
		t.Fatalf("expected unified 4-row choice-triple-stack (exit:, label:, if:, set:) when variables exist")
	}

	resp, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"ToggleSyntaxHelp"},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), `id="syntax-help-popover"`) || !strings.Contains(string(body), "sumienie &gt;= 2") {
		t.Fatalf("expected syntax help popover with exit:/if:/set: cheat sheet")
	}

	// 4. Open Scene Text Modal, save conditional prose & choice condition, and test 3 preview modes (normal / highlight / output)
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"OpenSceneTextModal"},
		"scene":  {startID},
	})
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":           {"SaveSceneTextModal"},
		"scene_id":         {startID},
		"name":             {"Zatopiona śluza"},
		"scene_effects":    {"zaufanie += 1"},
		"text":             {"if: zaufanie >= 2\nStrażnik patrzy z szacunkiem.\nelse:\nStrażnik patrzy podejrzliwie.\nendif:\nif: latarka\nSnop światła przebija wodę.\nendif:"},
		"choice_count":     {"2"},
		"choice_label_0":   {"Otwórz kratę"},
		"choice_target_0":  {srv.ActiveBook.Scenes[1].ID},
		"choice_cond_0":    {"latarka && zaufanie >= 1"},
		"choice_effects_0": {"frakcja = rzeczni"},
		"choice_label_1":   {"Wycofaj się"},
		"choice_target_1":  {"END"},
	})

	// Switch to 'highlight' preview mode (2nd icon)
	resp, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SetSceneTextPreviewMode"},
		"mode":   {"highlight"},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	html := string(body)
	if !strings.Contains(html, `id="scene-test-vars-bar"`) || !strings.Contains(html, `id="scene-prose-preview-stage"`) || !strings.Contains(html, "prose-syn-brace") {
		t.Fatalf("expected highlight preview mode to show #scene-test-vars-bar and highlighted syntax spans")
	}

	// Toggle test variable latarka=true and increment zaufanie to 2 in the live test bar
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":   {"UpdateSceneTextTestVar"},
		"var_name": {"latarka"},
		"toggle":   {"true"},
	})
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":   {"UpdateSceneTextTestVar"},
		"var_name": {"zaufanie"},
		"delta":    {"2"},
	})

	// Switch to 'output' preview mode (3rd icon: tylko wynikowy tekst)
	resp, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SetSceneTextPreviewMode"},
		"mode":   {"output"},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	html = string(body)
	stageIdx := strings.Index(html, `id="scene-prose-preview-stage"`)
	if stageIdx < 0 {
		t.Fatalf("expected #scene-prose-preview-stage in output preview mode")
	}
	stageEnd := strings.Index(html[stageIdx:], "</div>")
	stageHTML := html[stageIdx : stageIdx+stageEnd]
	if !strings.Contains(stageHTML, "Strażnik patrzy z szacunkiem.") || !strings.Contains(stageHTML, "Snop światła przebija wodę.") || strings.Contains(stageHTML, "endif:") {
		t.Fatalf("expected output preview stage to show only evaluated text without if:/endif: syntax, got:\n%s", stageHTML)
	}
	if strings.Contains(stageHTML, "<p class=\"scene-preview-para\">\n") {
		t.Fatalf("expected .scene-preview-para to have zero leading newline/indentation inside <p>, got:\n%s", stageHTML)
	}

	// Clicking eye toggle again (SetSceneTextPreviewMode without mode) returns to editable mode, and ToggleSceneTestVarsPopover opens popup
	resp, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SetSceneTextPreviewMode"},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if strings.Contains(string(body), `id="scene-prose-preview-stage"`) {
		t.Fatalf("expected clicking eye toggle again to return to normal editable mode")
	}
	resp, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"ToggleSceneTestVarsPopover"},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), `class="scene-test-vars-popover" id="scene-test-vars-bar"`) {
		t.Fatalf("expected ToggleSceneTestVarsPopover to open .scene-test-vars-popover#scene-test-vars-bar")
	}

	// Verify Play mode (#view-player) evaluates if:/else:/endif: blocks using the exact same RenderSceneProseHTML pipeline
	resp, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"PlayFromScene"},
		"scene":  {startID},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	playHTML := string(body)
	if !strings.Contains(playHTML, "Strażnik patrzy z szacunkiem.") || !strings.Contains(playHTML, "Snop światła przebija wodę.") || strings.Contains(playHTML, "endif:") || strings.Contains(playHTML, "else:") || strings.Contains(playHTML, "if: zaufanie") {
		t.Fatalf("expected Play mode (#view-player) to render evaluated conditional prose without raw if:/else:/endif: lines, got:\n%s", playHTML)
	}
}

func TestSeparatePlayerAndStudioBooksAndPlaybookUnpackWarning(t *testing.T) {
	rootDir := t.TempDir()
	booksDir := filepath.Join(rootDir, "books")
	_ = os.MkdirAll(booksDir, 0755)

	// 1. Create a book in the library to read (Book A) and pack a .playbook distribution archive (Book C)
	bookA, err := book.CreateNewBook(booksDir, "Kroniki Czytnika", "Jan Czytelnik", false)
	if err != nil {
		t.Fatalf("CreateNewBook A failed: %v", err)
	}
	bookCSrc, err := book.CreateNewBook(t.TempDir(), "Zamknięty Tom", "Anna Twórczyni", false)
	if err != nil {
		t.Fatalf("CreateNewBook C failed: %v", err)
	}
	bookCSrc.License = "CC BY-NC-ND 4.0 — Zakaz modyfikacji komercyjnej"
	if err := book.SaveBookMetadataToDisk(bookCSrc); err != nil {
		t.Fatalf("SaveBookMetadataToDisk C failed: %v", err)
	}
	playbookPath := filepath.Join(booksDir, "zamkniety-tom.playbook")
	if err := book.PackBookToZip(bookCSrc.RootPath, playbookPath); err != nil {
		t.Fatalf("PackBookToZip C failed: %v", err)
	}

	srv := NewServer(rootDir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 2. Open Book A in library for reading (view=player) -> must NOT open Book A in Studio
	rOpenA, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"OpenBook"},
		"book":   {bookA.ID},
		"view":   {"player"},
	})
	if err != nil {
		t.Fatalf("OpenBook A failed: %v", err)
	}
	_ = rOpenA.Body.Close()
	if srv.PlayerBook == nil || srv.PlayerBook.ID != bookA.ID {
		t.Fatalf("expected PlayerBook=%q, got %+v", bookA.ID, srv.PlayerBook)
	}
	if srv.ActiveBook != nil {
		t.Fatalf("expected Studio ActiveBook to remain nil after opening library book in player, got %+v", srv.ActiveBook)
	}

	// 3. Switch to Studio (view=editor): must show #editor-empty-state, NOT Book A, and must NOT render #editor-perf-overlay in prod
	rEd, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SwitchView"},
		"view":   {"editor"},
	})
	if err != nil {
		t.Fatalf("SwitchView editor failed: %v", err)
	}
	edBytes, _ := io.ReadAll(rEd.Body)
	_ = rEd.Body.Close()
	edHTML := string(edBytes)
	if !strings.Contains(edHTML, `id="editor-empty-state"`) {
		t.Fatalf("expected #editor-empty-state when switching to Studio with no Studio project open")
	}
	if strings.Contains(edHTML, `id="editor-perf-overlay"`) {
		t.Fatalf("expected #editor-perf-overlay to be omitted in production mode")
	}
	if strings.Contains(edHTML, `id="editor-book-title"`) {
		t.Fatalf("expected no #editor-book-title in Studio before a Studio project is opened")
	}

	// Verify production /app.js also strips M1 Max perf overlay
	rJS, err := http.Get(ts.URL + "/app.js")
	if err != nil {
		t.Fatalf("GET /app.js failed: %v", err)
	}
	jsBytes, _ := io.ReadAll(rJS.Body)
	_ = rJS.Body.Close()
	if strings.Contains(string(jsBytes), "M1 Max (10c/32g)") {
		t.Fatalf("expected production /app.js to strip M1 Max perf overlay")
	}

	// 4. Create Book B in Studio while Book A remains open in Player
	rCreateB, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"CreateNewBook"},
		"title":  {"Projekt Studio"},
		"author": {"Piotr Redaktor"},
	})
	if err != nil {
		t.Fatalf("CreateNewBook B failed: %v", err)
	}
	bBytes, _ := io.ReadAll(rCreateB.Body)
	_ = rCreateB.Body.Close()
	bHTML := string(bBytes)
	if srv.ActiveBook == nil || srv.ActiveBook.Title != "Projekt Studio" {
		t.Fatalf("expected Studio ActiveBook='Projekt Studio', got %+v", srv.ActiveBook)
	}
	if srv.PlayerBook == nil || srv.PlayerBook.ID != bookA.ID {
		t.Fatalf("expected PlayerBook to remain Book A (%q) while editing Book B in Studio, got %+v", bookA.ID, srv.PlayerBook)
	}
	if !strings.Contains(bHTML, `id="editor-book-title">Projekt Studio</span>`) || strings.Contains(bHTML, `id="topbar-book-title"`) {
		t.Fatalf("expected Studio topbar to show 'Projekt Studio' and isolate PlayerBook title, got:\n%s", bHTML)
	}

	// 5. Save license in Book B settings and verify it persists in book.playtxt
	rSaveSettings, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":  {"SaveBookSettings"},
		"title":   {"Projekt Studio"},
		"author":  {"Piotr Redaktor"},
		"license": {"Wszelkie prawa zastrzeżone 2026"},
	})
	if err != nil {
		t.Fatalf("SaveBookSettings failed: %v", err)
	}
	_ = rSaveSettings.Body.Close()
	if srv.ActiveBook.License != "Wszelkie prawa zastrzeżone 2026" {
		t.Fatalf("expected ActiveBook.License to be updated, got %q", srv.ActiveBook.License)
	}

	// 6. Attempt to open packed .playbook (Book C) in Studio -> must NOT open directly; must trigger UnpackPlaybookForStudio confirm modal with license warning
	rOpenPacked, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"OpenStudioProject"},
		"book":   {"zamkniety-tom"},
	})
	if err != nil {
		t.Fatalf("OpenStudioProject on .playbook failed: %v", err)
	}
	packedBytes, _ := io.ReadAll(rOpenPacked.Body)
	_ = rOpenPacked.Body.Close()
	packedHTML := string(packedBytes)
	if srv.ConfirmPendingAction != "UnpackPlaybookForStudio" {
		t.Fatalf("expected ConfirmPendingAction='UnpackPlaybookForStudio', got %q", srv.ConfirmPendingAction)
	}
	if !strings.Contains(packedHTML, "CC BY-NC-ND 4.0") || !strings.Contains(packedHTML, "Anna Twórczyni") {
		t.Fatalf("expected unpack confirmation modal to display author and license warning, got:\n%s", packedHTML)
	}
	if srv.ActiveBook.ID == "zamkniety-tom" {
		t.Fatalf("expected packed .playbook not to be opened in Studio before confirmation")
	}

	// 7. Accept confirmation -> unpacks working copy into Studio while keeping PlayerBook on Book A
	rAccept, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"ConfirmModalAccept"},
	})
	if err != nil {
		t.Fatalf("ConfirmModalAccept failed: %v", err)
	}
	_ = rAccept.Body.Close()
	if srv.ActiveBook == nil || srv.ActiveBook.IsPacked || srv.ActiveBook.Title != "Zamknięty Tom" {
		t.Fatalf("expected unpacked working copy of 'Zamknięty Tom' in Studio, got %+v", srv.ActiveBook)
	}
	if srv.PlayerBook == nil || srv.PlayerBook.ID != bookA.ID {
		t.Fatalf("expected PlayerBook to still be Book A (%q), got %+v", bookA.ID, srv.PlayerBook)
	}
}

func TestSceneTextSlideOutPanelUnifiedChoicesAndSyntaxHoverTooltip(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	customDir := filepath.Join(t.TempDir(), "SlideOutBook")
	_, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":    {"CreateNewBook"},
		"title":     {"Rzeczny Patrol"},
		"author":    {"Autor"},
		"mode":      {"book"},
		"save_path": {customDir},
	})
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}

	// 1. Open Variables Panel (#editor-variables-panel) and add enum & bool variables
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"ToggleEditorVariablesPanel"},
	})
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"AddBookVariable"},
		"kind":   {"enum"},
	})
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":    {"UpdateBookVariable"},
		"var_idx":   {"0"},
		"var_name":  {"stan_ducha"},
		"var_value": {"spokojny, czujny, zbuntowany"},
	})
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"AddBookVariable"},
		"kind":   {"bool"},
	})
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":   {"UpdateBookVariable"},
		"var_idx":  {"1"},
		"var_name": {"cwiczenia_z_wioslem"},
	})

	// 2. Open Scene Text Editor on a scene with NO branching choices (single transition) -> must still use .choice-triple-stack (exit:, text:, if:, set:) AND have .has-right-panel when variables panel is open
	startID := srv.ActiveBook.StartSceneID
	resp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"OpenSceneTextModal"},
		"scene":  {startID},
	})
	if err != nil {
		t.Fatalf("OpenSceneTextModal failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	html := string(body)
	if !strings.Contains(html, `class="modal-backdrop scene-text-fullscreen-backdrop mode-half has-right-panel" id="modal-scene-text"`) {
		t.Fatalf("expected #modal-scene-text to have .mode-half .has-right-panel when #editor-variables-panel is open, got:\n%s", html)
	}
	if !strings.Contains(html, `id="btn-collapse-scene-text"`) || !strings.Contains(html, `id="btn-expand-scene-text-full"`) || !strings.Contains(html, "choice-triple-stack") || !strings.Contains(html, `name="choice_target_0"`) || !strings.Contains(html, `name="choice_label_0"`) {
		t.Fatalf("expected unified .choice-triple-stack template (exit:, text:, if:, set:) and 3-stage << / >> buttons, got:\n%s", html)
	}
	if strings.Contains(html, "choice-port-trigger") || strings.Contains(html, "modal-choice-lead-btn") {
		t.Fatalf("expected no legacy choice-port-trigger or modal-choice-lead-btn in #modal-scene-text")
	}

	// 2b. Expand to full width (Stage 2: mode-full)
	respFull, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SetSceneTextWidthMode"},
		"mode":   {"full"},
	})
	if err != nil {
		t.Fatalf("SetSceneTextWidthMode failed: %v", err)
	}
	fullBody, _ := io.ReadAll(respFull.Body)
	_ = respFull.Body.Close()
	if !strings.Contains(string(fullBody), `class="modal-backdrop scene-text-fullscreen-backdrop mode-full has-right-panel" id="modal-scene-text"`) {
		t.Fatalf("expected #modal-scene-text to switch to .mode-full on SetSceneTextWidthMode=full, got:\n%s", string(fullBody))
	}

	// 3. Save an invalid condition expression (stan_ducha = zbuntowany && cwiczenia_z_wioslem++) and verify hover popup data-hint explains the syntax error
	resp, err = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":           {"SaveSceneTextModal"},
		"scene_id":         {startID},
		"name":             {"Odprawa przed wyprawą"},
		"text":             {"Stało się więc.\nif: cwiczenia_z_wioslem\nKakakakaka\nendif:"},
		"choice_target_0":  {srv.ActiveBook.Scenes[1].ID},
		"choice_cond_0":    {"stan_ducha = zbuntowany && cwiczenia_z_wioslem++"},
		"choice_effects_0": {"cwiczenia_z_wioslem = false stan_ducha = zbuntowany"},
	})
	if err != nil {
		t.Fatalf("SaveSceneTextModal failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	html = string(body)
	if !strings.Contains(html, "is-invalid") || !strings.Contains(html, "Błąd w if: operator") || !strings.Contains(html, "służy do zmiany wartości w polu set:") {
		t.Fatalf("expected syntax error data-hint tooltip on invalid if: expression, got:\n%s", html)
	}
	if !strings.Contains(html, `id="modal-scene-prose-hl"`) || !strings.Contains(html, `prose-hl-seg is-syntax`) {
		t.Fatalf("expected #modal-scene-prose-hl with .prose-hl-seg.is-syntax in #modal-scene-text, got:\n%s", html)
	}
	if strings.Contains(html, `Błąd typu: zmienna &#39;cwiczenia_z_wioslem&#39;`) {
		t.Fatalf("expected space-separated set: clauses ('cwiczenia_z_wioslem = false stan_ducha = zbuntowany') to be valid, got:\n%s", html)
	}
}

func TestSceneVoiceSectionsRecorderAndTextIsolation(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Create a narrated book in Studio via CreateNewBook action
	respCreate, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":   {"CreateNewBook"},
		"title":    {"Test Sekcji Głosu"},
		"author":   {"Autor"},
		"narrated": {"true"},
	})
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}
	_ = respCreate.Body.Close()
	if srv.ActiveBook == nil {
		t.Fatalf("expected ActiveBook after CreateNewBook")
	}

	b := srv.ActiveBook
	b.Variables = []book.VariableDef{
		{Name: "sumienie", Kind: book.VarKindInt, IntVal: 1},
	}
	sc := b.Scenes[0]
	sc.ExplicitID = "komora_topielcow"
	sc.Name = "Komora topielców"
	fullSceneText := "Wstęp nad śluzą.\n\nif: sumienie > 0\nKrzysiowi zadrżała dłoń na pręcie.\nelse:\nCofnął stopę i uniósł klapę.\nendif:\n\nFinał w kanale."
	sc.Text = fullSceneText
	book.FinalizeBook(b)
	if err := book.SaveBookMetadataToDisk(b); err != nil {
		t.Fatalf("SaveBookMetadataToDisk failed: %v", err)
	}
	if err := book.SaveBookScenesToDisk(b); err != nil {
		t.Fatalf("SaveBookScenesToDisk failed: %v", err)
	}
	srv.reloadActiveBook()

	resp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"OpenSceneTextModal"},
		"scene":  {"komora_topielcow"},
	})
	if err != nil {
		t.Fatalf("OpenSceneTextModal failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "if: sumienie &gt; 0") {
		t.Fatalf("expected full scene text when recorder is closed, got:\n%s", string(body))
	}

	// Open voice recorder -> should isolate section 01 text and render .voice-rec-sections
	resp, err = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"ToggleSceneVoiceRecorder"},
		"scene":  {"komora_topielcow"},
		"open":   {"true"},
	})
	if err != nil {
		t.Fatalf("ToggleSceneVoiceRecorder failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	html := string(body)
	if !strings.Contains(html, `id="modal-voice-rec-sections"`) || !strings.Contains(html, `SEKCJA 01 / 04 · WSTĘP`) {
		t.Fatalf("expected #modal-voice-rec-sections and SEKCJA 01 / 04 · WSTĘP header when recorder is open, got:\n%s", html)
	}

	// Select section 02_if -> should isolate section 02_if prose and preserve full scene text
	resp, err = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":                 {"SelectSceneVoiceSection"},
		"scene":                  {"komora_topielcow"},
		"voice_section":          {"02_if"},
		"isolated_voice_section": {"01"},
		"text":                   {"Wstęp nad śluzą."},
	})
	if err != nil {
		t.Fatalf("SelectSceneVoiceSection failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	html = string(body)
	if !strings.Contains(html, `SEKCJA 02 / 04 · IF: SUMIENIE &gt; 0`) || !strings.Contains(html, `voice/komora_topielcow__02_if.flac`) {
		t.Fatalf("expected isolated section 02_if header and target file path, got:\n%s", html)
	}
	if !strings.Contains(html, "Krzysiowi zadrżała dłoń na pręcie.") || strings.Contains(html, "Cofnął stopę i uniósł klapę.") {
		t.Fatalf("expected only section 02_if prose in #modal-scene-textarea, got:\n%s", html)
	}

	// Record audio for section 02_if, trim it, and delete it
	pcmBytes := make([]byte, 24000*2)
	for i := 0; i < 24000; i++ {
		pcmBytes[i*2] = 0x30
		pcmBytes[i*2+1] = 0x06
	}
	pcmB64 := base64.StdEncoding.EncodeToString(pcmBytes)
	resp, err = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":        {"SaveSceneVoiceRecording"},
		"scene":         {"komora_topielcow"},
		"voice_section": {"02_if"},
		"pcm_b64":       {pcmB64},
		"sample_rate":   {"24000"},
		"start_ratio":   {"0"},
		"end_ratio":     {"1"},
	})
	if err != nil {
		t.Fatalf("SaveSceneVoiceRecording failed: %v", err)
	}
	_ = resp.Body.Close()
	secFileOnDisk := filepath.Join(srv.ActiveBook.RootPath, "voice", "komora_topielcow__02_if.flac")
	if _, err := os.Stat(secFileOnDisk); err != nil {
		t.Fatalf("expected section file %s to exist on disk: %v", secFileOnDisk, err)
	}

	// Trim section 02_if non-destructively
	resp, err = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":        {"TrimSceneVoice"},
		"scene":         {"komora_topielcow"},
		"voice_section": {"02_if"},
		"start_ratio":   {"0.0500"},
		"end_ratio":     {"0.9200"},
		"destructive":   {"false"},
	})
	if err != nil {
		t.Fatalf("TrimSceneVoice failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), `data-trim-start="0.0500"`) || !strings.Contains(string(body), `data-trim-end="0.9200"`) {
		t.Fatalf("expected non-destructive trim attributes on #modal-voice-rec-strip for 02_if, got:\n%s", string(body))
	}

	// Delete section 02_if audio
	resp, err = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":        {"DeleteSceneVoice"},
		"scene":         {"komora_topielcow"},
		"voice_section": {"02_if"},
	})
	if err != nil {
		t.Fatalf("DeleteSceneVoice failed: %v", err)
	}
	_ = resp.Body.Close()
	if _, err := os.Stat(secFileOnDisk); !os.IsNotExist(err) {
		t.Fatalf("expected section file %s to be deleted after DeleteSceneVoice", secFileOnDisk)
	}

	// Close voice recorder -> full scene text must be restored intact
	resp, err = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":                 {"ToggleSceneVoiceRecorder"},
		"scene":                  {"komora_topielcow"},
		"open":                   {"false"},
		"isolated_voice_section": {"02_if"},
		"text":                   {"Krzysiowi zadrżała dłoń na pręcie."},
	})
	if err != nil {
		t.Fatalf("ToggleSceneVoiceRecorder(false) failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	html = string(body)
	if !strings.Contains(html, "Wstęp nad śluzą.") || !strings.Contains(html, "Cofnął stopę i uniósł klapę.") || !strings.Contains(html, "Finał w kanale.") {
		t.Fatalf("expected full multi-section scene text to be restored after closing voice recorder, got:\n%s", html)
	}
}

func BenchmarkParseActionRequestWFP(b *testing.B) {
	payload := EncodeWFPForm(url.Values{
		"action":    {"SaveEditorState"},
		"scene_id":  {"poczatek"},
		"pan_x":     {"120"},
		"pan_y":     {"-48"},
		"zoom":      {"1.25"},
		"positions": {"poczatek:120,96;scena_2:408,96;scena_3:696,96"},
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := &http.Request{
			Method: http.MethodPost,
			Header: http.Header{"Content-Type": []string{"text/x-wfp"}},
			Body:   io.NopCloser(bytes.NewReader(payload)),
		}
		ParseActionRequest(req)
	}
}
