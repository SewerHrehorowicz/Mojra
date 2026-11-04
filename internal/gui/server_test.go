package gui

import (
	"archive/zip"
	"bytes"
	"image/color"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"storyteller"
	"storyteller/internal/book"
	"storyteller/internal/tpl"
)

func TestServerActionLifecycleAndSecurity(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, storyteller.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Create a new book via declarative action CreateNewBook (form-encoded, zero JSON)
	form := url.Values{}
	form.Set("action", "CreateNewBook")
	form.Set("title", "Tajemnica Zamku")
	form.Set("author", "Jan Kowalski")
	form.Set("narrated", "true")

	resp, err := http.PostForm(ts.URL+"/api/action", form)
	if err != nil {
		t.Fatalf("POST /api/action CreateNewBook failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf("expected strict CSP header, got %q", csp)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	html := string(bodyBytes)
	if !strings.Contains(html, "Tajemnica Zamku") || !strings.Contains(html, "Początek") {
		t.Fatalf("expected rendered studio HTML for Tajemnica Zamku, got:\n%s", html)
	}

	// 2. Insert a new scene via declarative action InsertScene using explicit scene IDs
	insForm := url.Values{}
	insForm.Set("action", "InsertScene")
	insForm.Set("parent", "poczatek")
	insForm.Set("child", "rozdroze")
	insResp, err := http.PostForm(ts.URL+"/api/action", insForm)
	if err != nil {
		t.Fatalf("POST /api/action InsertScene failed: %v", err)
	}
	defer insResp.Body.Close()
	insHTML, _ := io.ReadAll(insResp.Body)
	if !strings.Contains(string(insHTML), "Nowa scena 3") {
		t.Fatalf("expected inserted scene Nowa scena 3 in rendered HTML")
	}

	// 2b. Add a choice to a scene via declarative action AddChoice
	addChForm := url.Values{}
	addChForm.Set("action", "AddChoice")
	addChForm.Set("scene", "poczatek")
	addChResp, err := http.PostForm(ts.URL+"/api/action", addChForm)
	if err != nil {
		t.Fatalf("POST /api/action AddChoice failed: %v", err)
	}
	defer addChResp.Body.Close()
	addChHTML, _ := io.ReadAll(addChResp.Body)
	if !strings.Contains(string(addChHTML), "choice_label_1") {
		t.Fatalf("expected second choice input choice_label_1 in rendered studio HTML")
	}
	if strings.Contains(string(addChHTML), "<select") {
		t.Fatalf("expected no native system <select> dropdowns in editor HTML")
	}
	if !strings.Contains(string(addChHTML), `id="studio-canvas"`) || !strings.Contains(string(addChHTML), `id="studio-graph-data"`) || !strings.Contains(string(addChHTML), `id="btn-context-add-node"`) {
		t.Fatalf("expected studio-canvas, studio-graph-data, and context add node button in editor HTML")
	}
	if !strings.Contains(string(addChHTML), `class="canvas-toolbar-left"`) ||
		!strings.Contains(string(addChHTML), `class="canvas-toolbar-center"`) ||
		!strings.Contains(string(addChHTML), `class="canvas-toolbar-right"`) {
		t.Fatalf("expected left, center, and right sections in .canvas-toolbar")
	}
	if strings.Contains(string(addChHTML), `data-canvas-add-node`) ||
		strings.Contains(string(addChHTML), `action="CreateChapterGroup"`) {
		t.Fatalf("expected scene and chapter creation buttons to be removed from .canvas-toolbar")
	}
	if strings.Contains(string(addChHTML), "PRZESUŃ") || strings.Contains(string(addChHTML), "node-grip-label") {
		t.Fatalf("expected no PRZESUŃ label text on node cards")
	}
	if !strings.Contains(string(addChHTML), "editor-context-panel") || !strings.Contains(string(addChHTML), "context-danger-zone") || !strings.Contains(string(addChHTML), "context-delete-btn") {
		t.Fatalf("expected contextual left panel with danger zone delete button in editor HTML")
	}
	if !strings.Contains(string(addChHTML), "editor-multi-panel") || !strings.Contains(string(addChHTML), "context-multi-scene-list") || !strings.Contains(string(addChHTML), "PromptDeleteScenes") {
		t.Fatalf("expected multi-scene panel with scene list and PromptDeleteScenes button in editor HTML")
	}
	if !strings.Contains(string(addChHTML), "node | id:poczatek") || !strings.Contains(string(addChHTML), "port | scene:poczatek | idx:1 | label:Wybór 2 | target_id:END |") {
		t.Fatalf("expected serialized node and unlinked port record in #studio-graph-data, got:\n%s", string(addChHTML))
	}

	// 2b-extra. Create a standalone scene at explicit canvas coordinates via CreateSceneAt
	createAtForm := url.Values{}
	createAtForm.Set("action", "CreateSceneAt")
	createAtForm.Set("x", "820")
	createAtForm.Set("y", "340")
	createAtResp, err := http.PostForm(ts.URL+"/api/action", createAtForm)
	if err != nil {
		t.Fatalf("POST /api/action CreateSceneAt failed: %v", err)
	}
	createAtHTML, _ := io.ReadAll(createAtResp.Body)
	_ = createAtResp.Body.Close()
	if !strings.Contains(string(createAtHTML), "x:820 | y:340") {
		t.Fatalf("expected created node at (820, 340) in rendered HTML")
	}

	// 2b-multi-delete. Create an extra scene and delete multiple scenes via PromptDeleteScenes + ConfirmModalAccept
	createAt2Form := url.Values{}
	createAt2Form.Set("action", "CreateSceneAt")
	createAt2Form.Set("x", "820")
	createAt2Form.Set("y", "460")
	createAt2Resp, err := http.PostForm(ts.URL+"/api/action", createAt2Form)
	if err != nil {
		t.Fatalf("POST /api/action CreateSceneAt 2 failed: %v", err)
	}
	_ = createAt2Resp.Body.Close()

	if len(srv.ActiveBook.Scenes) < 4 {
		t.Fatalf("expected at least 4 scenes before multi-delete, got %d", len(srv.ActiveBook.Scenes))
	}
	delSceneA := srv.ActiveBook.Scenes[len(srv.ActiveBook.Scenes)-2].ID
	delNameA := srv.ActiveBook.Scenes[len(srv.ActiveBook.Scenes)-2].Name
	delSceneB := srv.ActiveBook.Scenes[len(srv.ActiveBook.Scenes)-1].ID
	delNameB := srv.ActiveBook.Scenes[len(srv.ActiveBook.Scenes)-1].Name
	countBeforeMultiDel := len(srv.ActiveBook.Scenes)

	promptMultiDelForm := url.Values{}
	promptMultiDelForm.Set("action", "PromptDeleteScenes")
	promptMultiDelForm.Set("scenes", delSceneA+","+delSceneB)
	promptMultiDelResp, err := http.PostForm(ts.URL+"/api/action", promptMultiDelForm)
	if err != nil {
		t.Fatalf("POST /api/action PromptDeleteScenes failed: %v", err)
	}
	promptMultiDelHTML, _ := io.ReadAll(promptMultiDelResp.Body)
	_ = promptMultiDelResp.Body.Close()
	if !strings.Contains(string(promptMultiDelHTML), "Usuń zaznaczone sceny (2)") {
		t.Fatalf("expected multi-delete confirmation modal title, got:\n%s", string(promptMultiDelHTML))
	}

	confirmMultiDelForm := url.Values{}
	confirmMultiDelForm.Set("action", "ConfirmModalAccept")
	confirmMultiDelResp, err := http.PostForm(ts.URL+"/api/action", confirmMultiDelForm)
	if err != nil {
		t.Fatalf("POST /api/action ConfirmModalAccept for DeleteScenes failed: %v", err)
	}
	_ = confirmMultiDelResp.Body.Close()
	if len(srv.ActiveBook.Scenes) != countBeforeMultiDel-2 {
		t.Fatalf("expected %d scenes after multi-delete, got %d", countBeforeMultiDel-2, len(srv.ActiveBook.Scenes))
	}
	for _, sc := range srv.ActiveBook.Scenes {
		if sc.Name == delNameA || sc.Name == delNameB {
			t.Fatalf("expected deleted scenes %q and %q to be removed, found %q", delNameA, delNameB, sc.Name)
		}
	}

	// 2c. Open scene text modal and save updated text
	openModalForm := url.Values{}
	openModalForm.Set("action", "OpenSceneTextModal")
	openModalForm.Set("scene", "poczatek")
	openModalResp, err := http.PostForm(ts.URL+"/api/action", openModalForm)
	if err != nil {
		t.Fatalf("POST /api/action OpenSceneTextModal failed: %v", err)
	}
	openModalHTML, _ := io.ReadAll(openModalResp.Body)
	_ = openModalResp.Body.Close()
	if !strings.Contains(string(openModalHTML), "modal-scene-text") || !strings.Contains(string(openModalHTML), "scene-text-fullscreen-backdrop") || !strings.Contains(string(openModalHTML), "player-layout") {
		t.Fatalf("expected fullscreen book-view modal-scene-text in rendered HTML")
	}
	if !strings.Contains(string(openModalHTML), "NavigateSceneTextModal") || !strings.Contains(string(openModalHTML), "scene-text-back-btn") {
		t.Fatalf("expected interactive story navigation choices and styled back button in modal-scene-text")
	}
	if !strings.Contains(string(openModalHTML), `class="scene-title scene-title-present node-inline-title"`) || !strings.Contains(string(openModalHTML), `name="choice_label_0"`) || !strings.Contains(string(openModalHTML), "scroll-more-cue") {
		t.Fatalf("expected editable title input, editable choice inputs, and scroll-more-cue in modal-scene-text, got:\n%s", string(openModalHTML))
	}
	if !strings.Contains(string(openModalHTML), `data-scene-settings-toggle="true"`) ||
		!strings.Contains(string(openModalHTML), `id="modal-scene-settings-block"`) ||
		!strings.Contains(string(openModalHTML), `name="explicit_id"`) ||
		!strings.Contains(string(openModalHTML), `class="node-start-indicator"`) ||
		strings.Contains(string(openModalHTML), `id="modal-btn-set-start-scene"`) ||
		!strings.Contains(string(openModalHTML), `name="explicit_image"`) {
		t.Fatalf("expected settings toggle button, start indicator next to title, and settings block (id, image) in modal-scene-text, got:\n%s", string(openModalHTML))
	}
	if srv.ActiveBook.Narrated && !strings.Contains(string(openModalHTML), `name="explicit_voice"`) {
		t.Fatalf("expected explicit_voice selector in modal-scene-text when book is narrated")
	}
	if !srv.ActiveBook.Narrated && strings.Contains(string(openModalHTML), `name="explicit_voice"`) {
		t.Fatalf("did not expect explicit_voice selector in modal-scene-text when book is not narrated")
	}

	navModalForm := url.Values{}
	navModalForm.Set("action", "NavigateSceneTextModal")
	navModalForm.Set("scene_id", "poczatek")
	navModalForm.Set("choice_idx", "0")
	navModalResp, err := http.PostForm(ts.URL+"/api/action", navModalForm)
	if err != nil {
		t.Fatalf("POST /api/action NavigateSceneTextModal failed: %v", err)
	}
	navModalHTML, _ := io.ReadAll(navModalResp.Body)
	_ = navModalResp.Body.Close()
	if !strings.Contains(string(navModalHTML), `value="s1"`) || !strings.Contains(string(navModalHTML), "Nowa scena 3") {
		t.Fatalf("expected NavigateSceneTextModal to advance modal to s1 (Nowa scena 3), got:\n%s", string(navModalHTML))
	}
	if strings.Contains(string(navModalHTML), "node-start-btn") || strings.Contains(string(navModalHTML), `action="SetStartScene"`) {
		t.Fatalf("did not expect star button in scene text editor modal, got:\n%s", string(navModalHTML))
	}
	if !strings.Contains(string(navModalHTML), "scene-incoming-list") || !strings.Contains(string(navModalHTML), `data-scene="poczatek"`) {
		t.Fatalf("expected upward navigation (scene-incoming-list) pointing back to poczatek, got:\n%s", string(navModalHTML))
	}

	saveModalForm := url.Values{}
	saveModalForm.Set("action", "SaveSceneTextModal")
	saveModalForm.Set("scene_id", "poczatek")
	saveModalForm.Set("name", "Początek Przygody")
	saveModalForm.Set("text", "Pierwsze zdanie nowej treści. Drugie zdanie, które nie jest wyświetlane w skrócie.")
	saveModalForm.Set("choice_label_0", "Wejdź do zamku")
	saveModalResp, err := http.PostForm(ts.URL+"/api/action", saveModalForm)
	if err != nil {
		t.Fatalf("POST /api/action SaveSceneTextModal failed: %v", err)
	}
	saveModalHTML, _ := io.ReadAll(saveModalResp.Body)
	_ = saveModalResp.Body.Close()
	if !strings.Contains(string(saveModalHTML), "modal-scene-text") || srv.SceneTextModalID != "poczatek" {
		t.Fatalf("expected SaveSceneTextModal to keep modal-scene-text open on poczatek, got:\n%s", string(saveModalHTML))
	}
	if !strings.Contains(string(saveModalHTML), `value="Początek Przygody"`) || !strings.Contains(string(saveModalHTML), `value="Wejdź do zamku"`) {
		t.Fatalf("expected SaveSceneTextModal to persist updated title and choice label in modal HTML, got:\n%s", string(saveModalHTML))
	}
	if !strings.Contains(string(saveModalHTML), `class="node-icon-btn node-media-add-btn modal-add-choice-btn"`) || !strings.Contains(string(saveModalHTML), `name="choice_target_0"`) {
		t.Fatalf("expected modal-add-choice-btn and choice_target_0 selector in modal-scene-text, got:\n%s", string(saveModalHTML))
	}

	// Add a choice directly inside modal-scene-text, edit its target, then remove it
	addModalChoiceForm := url.Values{}
	addModalChoiceForm.Set("action", "AddChoice")
	addModalChoiceForm.Set("scene", "poczatek")
	addModalChoiceResp, err := http.PostForm(ts.URL+"/api/action", addModalChoiceForm)
	if err != nil {
		t.Fatalf("POST /api/action AddChoice in modal failed: %v", err)
	}
	addModalChoiceHTML, _ := io.ReadAll(addModalChoiceResp.Body)
	_ = addModalChoiceResp.Body.Close()
	if addModalChoiceResp.Header.Get("X-Target") != "#modals-root" || !strings.Contains(string(addModalChoiceHTML), "modal-scene-text") || !strings.Contains(string(addModalChoiceHTML), `name="choice_target_2"`) {
		t.Fatalf("expected AddChoice in modal-scene-text to return #modals-root with choice_target_2, got X-Target=%q:\n%s", addModalChoiceResp.Header.Get("X-Target"), string(addModalChoiceHTML))
	}

	editTargetModalForm := url.Values{}
	editTargetModalForm.Set("action", "SaveSceneTextModal")
	editTargetModalForm.Set("scene_id", "poczatek")
	editTargetModalForm.Set("name", "Początek Przygody")
	editTargetModalForm.Set("text", "Pierwsze zdanie nowej treści. Drugie zdanie, które nie jest wyświetlane w skrócie.")
	editTargetModalForm.Set("choice_label_2", "Wycofaj się")
	editTargetModalForm.Set("choice_target_2", "END")
	editTargetModalResp, err := http.PostForm(ts.URL+"/api/action", editTargetModalForm)
	if err != nil {
		t.Fatalf("POST /api/action SaveSceneTextModal target update failed: %v", err)
	}
	editTargetModalHTML, _ := io.ReadAll(editTargetModalResp.Body)
	_ = editTargetModalResp.Body.Close()
	if !strings.Contains(string(editTargetModalHTML), `value="Wycofaj się"`) {
		t.Fatalf("expected updated choice_label_2 in modal HTML")
	}
	if !strings.Contains(string(editTargetModalHTML), "choice-port-icon port-filled") ||
		!strings.Contains(string(editTargetModalHTML), `<span class="choice-port-icon"></span>`) ||
		!strings.Contains(string(editTargetModalHTML), "choice-trash-icon") ||
		!strings.Contains(string(editTargetModalHTML), `action="PromptRemoveChoice"`) ||
		strings.Contains(string(editTargetModalHTML), "BranchChoiceScene") ||
		strings.Contains(string(editTargetModalHTML), "node-choice-arrow") {
		t.Fatalf("expected port circle icons (filled for connected, empty for END), trash icon with PromptRemoveChoice, and no plus/arrow in modal choices, got:\n%s", string(editTargetModalHTML))
	}

	promptRemChoiceForm := url.Values{}
	promptRemChoiceForm.Set("action", "PromptRemoveChoice")
	promptRemChoiceForm.Set("scene", "poczatek")
	promptRemChoiceForm.Set("choice_idx", "2")
	promptRemChoiceResp, err := http.PostForm(ts.URL+"/api/action", promptRemChoiceForm)
	if err != nil {
		t.Fatalf("POST /api/action PromptRemoveChoice in modal failed: %v", err)
	}
	promptRemChoiceHTML, _ := io.ReadAll(promptRemChoiceResp.Body)
	_ = promptRemChoiceResp.Body.Close()
	if !strings.Contains(string(promptRemChoiceHTML), `id="modal-confirm"`) || !strings.Contains(string(promptRemChoiceHTML), "Wycofaj się") || !strings.Contains(string(promptRemChoiceHTML), `id="modal-scene-text"`) {
		t.Fatalf("expected PromptRemoveChoice to open #modal-confirm alongside #modal-scene-text, got:\n%s", string(promptRemChoiceHTML))
	}

	confirmRemChoiceForm := url.Values{}
	confirmRemChoiceForm.Set("action", "ConfirmModalAccept")
	confirmRemChoiceResp, err := http.PostForm(ts.URL+"/api/action", confirmRemChoiceForm)
	if err != nil {
		t.Fatalf("POST /api/action ConfirmModalAccept for RemoveChoice in modal failed: %v", err)
	}
	confirmRemChoiceHTML, _ := io.ReadAll(confirmRemChoiceResp.Body)
	_ = confirmRemChoiceResp.Body.Close()
	if strings.Contains(string(confirmRemChoiceHTML), `value="Wycofaj się"`) || !strings.Contains(string(confirmRemChoiceHTML), `id="modal-scene-text"`) {
		t.Fatalf("expected ConfirmModalAccept to remove choice_2 and keep #modal-scene-text open, got:\n%s", string(confirmRemChoiceHTML))
	}

	closeModalForm := url.Values{}
	closeModalForm.Set("action", "CloseSceneTextModal")
	closeModalResp, err := http.PostForm(ts.URL+"/api/action", closeModalForm)
	if err != nil {
		t.Fatalf("POST /api/action CloseSceneTextModal failed: %v", err)
	}
	closeModalHTML, _ := io.ReadAll(closeModalResp.Body)
	_ = closeModalResp.Body.Close()
	if !strings.Contains(string(closeModalHTML), `class="btn btn-sm node-edit-text-btn"`) || strings.Contains(string(closeModalHTML), "Drugie zdanie") {
		t.Fatalf("expected .node-edit-text-btn in context panel after closing modal, got:\n%s", string(closeModalHTML))
	}

	// 2d. Insert a new scene before an existing scene via InsertSceneBefore
	insBeforeForm := url.Values{}
	insBeforeForm.Set("action", "InsertSceneBefore")
	insBeforeForm.Set("target", "rozdroze")
	insBeforeResp, err := http.PostForm(ts.URL+"/api/action", insBeforeForm)
	if err != nil {
		t.Fatalf("POST /api/action InsertSceneBefore failed: %v", err)
	}
	insBeforeHTML, _ := io.ReadAll(insBeforeResp.Body)
	_ = insBeforeResp.Body.Close()
	if !strings.Contains(string(insBeforeHTML), "Nowa scena 4") {
		t.Fatalf("expected Nowa scena 4 after InsertSceneBefore")
	}

	// 2e. Save node position and viewport to editor.txt via SaveEditorState and connect wire via ConnectChoice
	edStateForm := url.Values{}
	edStateForm.Set("action", "SaveEditorState")
	edStateForm.Set("scene_id", "poczatek")
	edStateForm.Set("x", "540")
	edStateForm.Set("y", "120")
	edStateForm.Set("pan_x", "-80")
	edStateForm.Set("pan_y", "30")
	edStateForm.Set("zoom", "1.15")
	edStateResp, err := http.PostForm(ts.URL+"/api/action", edStateForm)
	if err != nil {
		t.Fatalf("POST /api/action SaveEditorState failed: %v", err)
	}
	_ = edStateResp.Body.Close()
	editorTxtBytes, err := os.ReadFile(filepath.Join(rootDir, "books", "tajemnica-zamku", "editor.txt"))
	if err != nil {
		t.Fatalf("expected editor.txt in book directory: %v", err)
	}
	if !strings.Contains(string(editorTxtBytes), "540, 120") || !strings.Contains(string(editorTxtBytes), "zoom: 1.15") {
		t.Fatalf("unexpected editor.txt content:\n%s", string(editorTxtBytes))
	}

	// 2f. Batch SaveEditorState with negative coordinates (no canvas bounds)
	batchForm := url.Values{}
	batchForm.Set("action", "SaveEditorState")
	batchForm.Set("positions", "poczatek:-240,-120;rozdroze:480,240")
	batchResp, err := http.PostForm(ts.URL+"/api/action", batchForm)
	if err != nil {
		t.Fatalf("POST /api/action batch SaveEditorState failed: %v", err)
	}
	_ = batchResp.Body.Close()
	batchEditorBytes, _ := os.ReadFile(filepath.Join(rootDir, "books", "tajemnica-zamku", "editor.txt"))
	if !strings.Contains(string(batchEditorBytes), "-240, -120") || !strings.Contains(string(batchEditorBytes), "480, 240") {
		t.Fatalf("expected negative and batch coordinates in editor.txt, got:\n%s", string(batchEditorBytes))
	}

	// 2g. Rename chapter group and run AutoLayoutNodes
	renameChForm := url.Values{}
	renameChForm.Set("action", "RenameChapter")
	renameChForm.Set("old_chapter", "Rozdział 1")
	renameChForm.Set("new_chapter", "Akt I: Twierdza")
	renameChResp, err := http.PostForm(ts.URL+"/api/action", renameChForm)
	if err != nil {
		t.Fatalf("POST /api/action RenameChapter failed: %v", err)
	}
	renameChHTML, _ := io.ReadAll(renameChResp.Body)
	_ = renameChResp.Body.Close()
	if !strings.Contains(string(renameChHTML), "Akt I: Twierdza") ||
		!strings.Contains(string(renameChHTML), "chapter | ") ||
		!strings.Contains(string(renameChHTML), "name:Akt I: Twierdza") ||
		!strings.Contains(string(renameChHTML), "chapter-group-title-input") {
		t.Fatalf("expected renamed chapter group in rendered HTML, got:\n%s", string(renameChHTML))
	}

	autoLayoutForm := url.Values{}
	autoLayoutForm.Set("action", "AutoLayoutNodes")
	autoLayoutResp, err := http.PostForm(ts.URL+"/api/action", autoLayoutForm)
	if err != nil {
		t.Fatalf("POST /api/action AutoLayoutNodes failed: %v", err)
	}
	_ = autoLayoutResp.Body.Close()

	// 2h. Open and close Editor Help Modal
	helpForm := url.Values{}
	helpForm.Set("action", "OpenEditorHelpModal")
	helpResp, err := http.PostForm(ts.URL+"/api/action", helpForm)
	if err != nil {
		t.Fatalf("POST /api/action OpenEditorHelpModal failed: %v", err)
	}
	helpHTML, _ := io.ReadAll(helpResp.Body)
	_ = helpResp.Body.Close()
	if !strings.Contains(string(helpHTML), `id="editor-help-panel"`) || strings.Contains(string(helpHTML), `id="editor-help-panel" class="editor-help-panel hidden"`) {
		t.Fatalf("expected visible #editor-help-panel in rendered workspace HTML")
	}

	closeHelpForm := url.Values{}
	closeHelpForm.Set("action", "CloseEditorHelpModal")
	closeHelpResp, _ := http.PostForm(ts.URL+"/api/action", closeHelpForm)
	if closeHelpResp != nil {
		_ = closeHelpResp.Body.Close()
	}

	connForm := url.Values{}
	connForm.Set("action", "ConnectChoice")
	connForm.Set("from_scene", "poczatek")
	connForm.Set("choice_idx", "0")
	connForm.Set("to_scene", "rozdroze")
	connResp, err := http.PostForm(ts.URL+"/api/action", connForm)
	if err != nil {
		t.Fatalf("POST /api/action ConnectChoice failed: %v", err)
	}
	_ = connResp.Body.Close()

	// 3. Switch to player view and choose a branch
	playForm := url.Values{}
	playForm.Set("action", "SwitchView")
	playForm.Set("view", "player")
	playResp, err := http.PostForm(ts.URL+"/api/action", playForm)
	if err != nil {
		t.Fatalf("SwitchView player failed: %v", err)
	}
	playBytes, _ := io.ReadAll(playResp.Body)
	_ = playResp.Body.Close()
	if !strings.Contains(string(playBytes), "player-bg-stage") {
		t.Fatalf("expected fullscreen player-bg-stage in rendered player HTML")
	}

	chooseForm := url.Values{}
	chooseForm.Set("action", "ChooseBranch")
	chooseForm.Set("choice_idx", "0")
	chooseResp, err := http.PostForm(ts.URL+"/api/action", chooseForm)
	if err != nil {
		t.Fatalf("ChooseBranch failed: %v", err)
	}
	_ = chooseResp.Body.Close()

	// Verify binary save file (.sav) exists on disk
	if _, err := os.Stat(filepath.Join(rootDir, "saves", "tajemnica-zamku.sav")); err != nil {
		t.Fatalf("expected binary save file tajemnica-zamku.sav on disk: %v", err)
	}

	// 4. Verify PNG cover and subdirectory image are served and SVG media is forbidden
	coverResp, err := http.Get(ts.URL + "/api/books/tajemnica-zamku/media/cover.png")
	if err != nil {
		t.Fatalf("GET cover.png failed: %v", err)
	}
	defer coverResp.Body.Close()
	if coverResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for cover.png, got %d", coverResp.StatusCode)
	}

	subImgResp, err := http.Get(ts.URL + "/api/books/tajemnica-zamku/media/images/poczatek.png")
	if err != nil {
		t.Fatalf("GET images/poczatek.png failed: %v", err)
	}
	defer subImgResp.Body.Close()
	if subImgResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for subdirectory images/poczatek.png, got %d", subImgResp.StatusCode)
	}

	_ = os.WriteFile(filepath.Join(rootDir, "books", "tajemnica-zamku", "bad.svg"), []byte("<svg></svg>"), 0644)
	svgResp, err := http.Get(ts.URL + "/api/books/tajemnica-zamku/media/bad.svg")
	if err != nil {
		t.Fatalf("GET bad.svg failed: %v", err)
	}
	defer svgResp.Body.Close()
	if svgResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for .svg book media, got %d", svgResp.StatusCode)
	}

	// 5. Verify QR code PNG generation is disabled in production mode and enabled in dev mode
	qrProdResp, err := http.Get(ts.URL + "/api/lan-qr.png")
	if err != nil {
		t.Fatalf("GET /api/lan-qr.png in prod mode failed: %v", err)
	}
	_ = qrProdResp.Body.Close()
	if qrProdResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for /api/lan-qr.png in prod mode, got %d", qrProdResp.StatusCode)
	}

	devQRServer := NewServerWithOptions(rootDir, storyteller.AppFS, true)
	devQRTS := httptest.NewServer(devQRServer.Handler())
	defer devQRTS.Close()

	qrResp, err := http.Get(devQRTS.URL + "/api/lan-qr.png")
	if err != nil {
		t.Fatalf("GET /api/lan-qr.png in dev mode failed: %v", err)
	}
	defer qrResp.Body.Close()
	var qrBuf bytes.Buffer
	_, _ = qrBuf.ReadFrom(qrResp.Body)
	if qrResp.StatusCode != http.StatusOK || !bytes.HasPrefix(qrBuf.Bytes(), []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("expected valid PNG from /api/lan-qr.png in dev mode, status=%d", qrResp.StatusCode)
	}

	// 6. Verify bundled primary fonts are served from embedded AppFS
	for _, fontFile := range []string{
		"Aleo-Regular.ttf",
		"Aleo-Bold.ttf",
		"Lato-Light.ttf",
		"Lato-LightItalic.ttf",
		"Lato-Regular.ttf",
		"Lato-Italic.ttf",
		"Lato-Bold.ttf",
	} {
		fResp, err := http.Get(ts.URL + "/fonts/" + fontFile)
		if err != nil {
			t.Fatalf("GET /fonts/%s failed: %v", fontFile, err)
		}
		fBytes, _ := io.ReadAll(fResp.Body)
		_ = fResp.Body.Close()
		if fResp.StatusCode != http.StatusOK || len(fBytes) < 1024 {
			t.Fatalf("expected 200 OK and non-empty font data for /fonts/%s, status=%d len=%d", fontFile, fResp.StatusCode, len(fBytes))
		}
	}
}

func TestMultiBookLibraryDiskURLAndPerBookProgress(t *testing.T) {
	rootDir := t.TempDir()
	externalDir := t.TempDir()

	// Create an external book on disk outside rootDir/books
	diskBookPath := filepath.Join(externalDir, "nocny-pociag")
	_ = os.MkdirAll(diskBookPath, 0755)
	_ = os.WriteFile(filepath.Join(diskBookPath, "data.txt"), []byte("title: Nocny Pociąg\nauthor: Marek H.\nnarrated: false\nstart: peron\n"), 0644)
	_ = os.WriteFile(filepath.Join(diskBookPath, "story.txt"), []byte(`chapter: Odjazd

scene: Peron we mgle
id: peron
Pociąg czeka na torze trzecim.
Wsiądź do wagonu -> przedzial

scene: Przedział
id: przedzial
end: true
Zająłeś miejsce przy oknie.
`), 0644)

	// Prepare a downloadable .stbook archive served over HTTP
	var stbookBuf bytes.Buffer
	zw := zip.NewWriter(&stbookBuf)
	fData, _ := zw.Create("data.txt")
	_, _ = fData.Write([]byte("title: Echo Głębin\nauthor: Klara M.\nnarrated: false\nstart: zanurzenie\n"))
	fStory, _ := zw.Create("story.txt")
	_, _ = fStory.Write([]byte(`chapter: Głębia

scene: Zanurzenie
id: zanurzenie
Batyskaf powoli opada w ciemność.
Włącz reflektory -> rafa

scene: Koralowa Rafa
id: rafa
end: true
Światło wydobywa z mroku ruiny.
`))
	_ = zw.Close()

	remoteSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(stbookBuf.Bytes())
	}))
	defer remoteSrv.Close()

	srv := NewServer(rootDir, storyteller.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 0. Empty library shows "No books in library page"
	emptyResp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / on empty library failed: %v", err)
	}
	emptyHTML, _ := io.ReadAll(emptyResp.Body)
	_ = emptyResp.Body.Close()
	if !strings.Contains(string(emptyHTML), "No books in library page") {
		t.Fatalf("expected 'No books in library page' when library has no books, got:\n%s", string(emptyHTML))
	}

	// 1. Add book from disk via AddBookFromDisk action (must NOT copy into rootDir/books)
	addDiskForm := url.Values{}
	addDiskForm.Set("action", "AddBookFromDisk")
	addDiskForm.Set("path", diskBookPath)
	respDisk, err := http.PostForm(ts.URL+"/api/action", addDiskForm)
	if err != nil {
		t.Fatalf("AddBookFromDisk failed: %v", err)
	}
	diskHTML, _ := io.ReadAll(respDisk.Body)
	_ = respDisk.Body.Close()
	if !strings.Contains(string(diskHTML), "Nocny Pociąg") || strings.Contains(string(diskHTML), "No books in library page") {
		t.Fatalf("expected Nocny Pociąg and no empty-state message in library HTML after AddBookFromDisk")
	}
	if _, err := os.Stat(filepath.Join(rootDir, "books", "nocny-pociag")); !os.IsNotExist(err) {
		t.Fatalf("expected external disk book NOT to be copied into rootDir/books")
	}

	// 2. Add book via URL (.stbook) via AddBookFromURL action
	addURLForm := url.Values{}
	addURLForm.Set("action", "AddBookFromURL")
	addURLForm.Set("url", remoteSrv.URL+"/echo-glebin.stbook")
	respURL, err := http.PostForm(ts.URL+"/api/action", addURLForm)
	if err != nil {
		t.Fatalf("AddBookFromURL failed: %v", err)
	}
	urlHTML, _ := io.ReadAll(respURL.Body)
	_ = respURL.Body.Close()
	if !strings.Contains(string(urlHTML), "Echo Głębin") {
		t.Fatalf("expected Echo Głębin in HTML after AddBookFromURL")
	}
	if _, err := os.Stat(filepath.Join(rootDir, "books", "echo-glebin.stbook")); err != nil {
		t.Fatalf("expected downloaded echo-glebin.stbook in rootDir/books: %v", err)
	}

	// 3. Play Book 1 (nocny-pociag), advance to scene "przedzial" and record audio_time = 23.5
	open1 := url.Values{}
	open1.Set("action", "OpenBook")
	open1.Set("book", "nocny-pociag")
	open1.Set("view", "player")
	r1, _ := http.PostForm(ts.URL+"/api/action", open1)
	_ = r1.Body.Close()

	choose1 := url.Values{}
	choose1.Set("action", "ChooseBranch")
	choose1.Set("choice_idx", "0")
	r2, _ := http.PostForm(ts.URL+"/api/action", choose1)
	_ = r2.Body.Close()

	// Switch directly to Book 2 (echo-glebin) while passing audio_time for Book 1
	open2 := url.Values{}
	open2.Set("action", "OpenBook")
	open2.Set("book", "echo-glebin")
	open2.Set("view", "player")
	open2.Set("audio_time", "23.5")
	r3, _ := http.PostForm(ts.URL+"/api/action", open2)
	r3Bytes, _ := io.ReadAll(r3.Body)
	_ = r3.Body.Close()
	if !strings.Contains(string(r3Bytes), "Zanurzenie") {
		t.Fatalf("expected Book 2 (Echo Głębin) to start at Zanurzenie")
	}

	// Verify Book 1's save state has scene "przedzial" AND audio_time 23.5 preserved
	save1 := book.LoadSaveState(filepath.Join(rootDir, "saves"), "nocny-pociag", "peron")
	if save1.CurrentSceneID != "przedzial" || save1.AudioTime != 23.5 || len(save1.History) != 1 {
		t.Fatalf("expected Book 1 progress preserved (scene=przedzial, audio=23.5, history=1), got %+v", save1)
	}

	// Verify Book 2's save state is independent (at start scene "zanurzenie", history=0)
	save2 := book.LoadSaveState(filepath.Join(rootDir, "saves"), "echo-glebin", "zanurzenie")
	if save2.CurrentSceneID != "zanurzenie" || len(save2.History) != 0 {
		t.Fatalf("expected Book 2 progress to be at start scene, got %+v", save2)
	}

	// Advance Book 2 to "rafa"
	choose2 := url.Values{}
	choose2.Set("action", "ChooseBranch")
	choose2.Set("choice_idx", "0")
	r4, _ := http.PostForm(ts.URL+"/api/action", choose2)
	_ = r4.Body.Close()

	// Re-open Book 1 and verify it resumes at "Przedział" with its own history
	reopen1 := url.Values{}
	reopen1.Set("action", "OpenBook")
	reopen1.Set("book", "nocny-pociag")
	reopen1.Set("view", "player")
	r5, _ := http.PostForm(ts.URL+"/api/action", reopen1)
	r5HTML, _ := io.ReadAll(r5.Body)
	_ = r5.Body.Close()
	if !strings.Contains(string(r5HTML), "Przedział") || !strings.Contains(string(r5HTML), "Peron we mgle") {
		t.Fatalf("expected Book 1 to resume at Przedział with Peron we mgle in history")
	}

	// 4. Reset Book 2 progress from library without affecting Book 1 progress
	promptReset2 := url.Values{}
	promptReset2.Set("action", "PromptResetProgress")
	promptReset2.Set("book", "echo-glebin")
	r6, _ := http.PostForm(ts.URL+"/api/action", promptReset2)
	r6Bytes, _ := io.ReadAll(r6.Body)
	_ = r6.Body.Close()
	if !strings.Contains(string(r6Bytes), `class="btn btn-primary" type="button" action="ConfirmModalAccept"`) || strings.Contains(string(r6Bytes), `btn-danger`) {
		t.Fatalf("expected PromptResetProgress confirmation modal to use non-danger btn-primary, got:\n%s", string(r6Bytes))
	}

	confirmReset2 := url.Values{}
	confirmReset2.Set("action", "ConfirmModalAccept")
	r7, _ := http.PostForm(ts.URL+"/api/action", confirmReset2)
	_ = r7.Body.Close()

	save1After := book.LoadSaveState(filepath.Join(rootDir, "saves"), "nocny-pociag", "peron")
	save2After := book.LoadSaveState(filepath.Join(rootDir, "saves"), "echo-glebin", "zanurzenie")
	if save1After.CurrentSceneID != "przedzial" || len(save1After.History) != 1 {
		t.Fatalf("expected Book 1 progress untouched after resetting Book 2, got %+v", save1After)
	}
	if save2After.CurrentSceneID != "zanurzenie" || len(save2After.History) != 0 {
		t.Fatalf("expected Book 2 progress reset to zanurzenie, got %+v", save2After)
	}

	// 5. Unlink external disk book from library and verify files on disk are NOT deleted
	promptRem := url.Values{}
	promptRem.Set("action", "PromptRemoveBook")
	promptRem.Set("book", "nocny-pociag")
	r8, _ := http.PostForm(ts.URL+"/api/action", promptRem)
	_ = r8.Body.Close()

	confirmRem := url.Values{}
	confirmRem.Set("action", "ConfirmModalAccept")
	r9, _ := http.PostForm(ts.URL+"/api/action", confirmRem)
	_ = r9.Body.Close()

	if _, err := os.Stat(filepath.Join(diskBookPath, "story.txt")); err != nil {
		t.Fatalf("expected original files of external book on disk to remain untouched after removing from library: %v", err)
	}
}

func TestDevicePreviewStrippedInProductionAndEnabledInDev(t *testing.T) {
	rootDir := t.TempDir()
	savesDir := filepath.Join(rootDir, "saves")
	_ = os.MkdirAll(savesDir, 0755)
	_ = os.WriteFile(filepath.Join(savesDir, "settings.ini"), []byte("[display]\ntheme = dark\npreview_mode = portrait\n"), 0644)

	// 1. Production server (default NewServer) must strip device preview completely
	prodSrv := NewServer(rootDir, storyteller.AppFS)
	prodTS := httptest.NewServer(prodSrv.Handler())
	defer prodTS.Close()

	resp, err := http.Get(prodTS.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	indexHTML, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	for _, forbidden := range []string{"device-preview-bar", "device-stage", "device-frame", "SetDeviceMode", "Telefon (Pionowo)", "id=\"btn-open-qr\""} {
		if strings.Contains(string(indexHTML), forbidden) {
			t.Fatalf("expected %q stripped from production GET /, got:\n%s", forbidden, string(indexHTML))
		}
	}

	form := url.Values{}
	form.Set("action", "SetDeviceMode")
	form.Set("mode", "landscape")
	actResp, err := http.PostForm(prodTS.URL+"/api/action", form)
	if err != nil {
		t.Fatalf("POST /api/action SetDeviceMode failed: %v", err)
	}
	_ = actResp.Body.Close()
	if gotHdr := actResp.Header.Get("X-Device-Mode"); gotHdr != "" {
		t.Fatalf("expected no X-Device-Mode header in production mode, got %q", gotHdr)
	}

	// Verify QR code markup is stripped from #modal-qr in production mode
	qrModalForm := url.Values{}
	qrModalForm.Set("action", "OpenQRModal")
	prodQRModalResp, err := http.PostForm(prodTS.URL+"/api/action", qrModalForm)
	if err != nil {
		t.Fatalf("POST /api/action OpenQRModal failed: %v", err)
	}
	prodQRModalHTML, _ := io.ReadAll(prodQRModalResp.Body)
	_ = prodQRModalResp.Body.Close()
	if strings.Contains(string(prodQRModalHTML), "/api/lan-qr.png") || strings.Contains(string(prodQRModalHTML), "qr-frame") {
		t.Fatalf("expected QR code elements to be omitted from #modal-qr in production mode, got:\n%s", string(prodQRModalHTML))
	}

	// 2. Development server (NewServerWithOptions with devMode=true) includes device preview and QR code
	devSrv := NewServerWithOptions(rootDir, storyteller.AppFS, true)
	devTS := httptest.NewServer(devSrv.Handler())
	defer devTS.Close()

	devResp, err := http.Get(devTS.URL + "/")
	if err != nil {
		t.Fatalf("GET / in dev mode failed: %v", err)
	}
	devHTML, _ := io.ReadAll(devResp.Body)
	_ = devResp.Body.Close()
	if !strings.Contains(string(devHTML), "device-preview-bar") || !strings.Contains(string(devHTML), "SetDeviceMode") || !strings.Contains(string(devHTML), `action="OpenQRModal"`) {
		t.Fatalf("expected device-preview-bar and OpenQRModal in dev mode GET /")
	}

	devQRModalResp, err := http.PostForm(devTS.URL+"/api/action", qrModalForm)
	if err != nil {
		t.Fatalf("POST /api/action OpenQRModal in dev mode failed: %v", err)
	}
	devQRModalHTML, _ := io.ReadAll(devQRModalResp.Body)
	_ = devQRModalResp.Body.Close()
	if !strings.Contains(string(devQRModalHTML), "/api/lan-qr.png") || !strings.Contains(string(devQRModalHTML), "qr-frame") || !strings.Contains(string(devQRModalHTML), "ExtractEmbeddedBook") {
		t.Fatalf("expected QR code and ExtractEmbeddedBook elements in #modal-qr in dev mode")
	}

	// 3. ExtractEmbeddedBook in dev mode extracts books/nie-boj-sie-policji-rzecznej on demand with 0 warnings
	extractForm := url.Values{}
	extractForm.Set("action", "ExtractEmbeddedBook")
	extResp, err := http.PostForm(devTS.URL+"/api/action", extractForm)
	if err != nil {
		t.Fatalf("POST /api/action ExtractEmbeddedBook failed: %v", err)
	}
	_ = extResp.Body.Close()
	if devSrv.ActiveBook == nil || devSrv.ActiveBook.ID != "nie-boj-sie-policji-rzecznej" {
		t.Fatalf("expected ActiveBook to be nie-boj-sie-policji-rzecznej after ExtractEmbeddedBook, got %+v", devSrv.ActiveBook)
	}
	if len(devSrv.ActiveBook.Warnings) != 0 {
		t.Fatalf("expected 0 warnings in extracted embedded book, got: %v", devSrv.ActiveBook.Warnings)
	}
}

func TestLibraryCoverCardAndBranchingChoicesGuard(t *testing.T) {
	rootDir := t.TempDir()
	sampleBookDir := filepath.Join(rootDir, "books", "nie-boj-sie-policji-rzecznej")
	if err := os.MkdirAll(sampleBookDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	_ = os.WriteFile(filepath.Join(sampleBookDir, "data.txt"), []byte("title: Nie bój się policji rzecznej\nauthor: Autor\nnarrated: false\nstart: warsztat\n"), 0644)
	_ = os.WriteFile(filepath.Join(sampleBookDir, "story.txt"), []byte(`chapter: Wioskowe dziwadło

scene: Wstyd w warsztacie
id: warsztat
Stało się więc, i Krzyś wstąpił do policji rzecznej.

Idź do kapitana Arvo -> sikawa_kapitana
Ucieknij nad brzeg rzeki -> samotnosc_nad_woda

scene: Samotność nad wodą
id: samotnosc_nad_woda
next: sikawa_kapitana
Krzyś zaszył się w gęstych trzcinach przy starym pomoście.

scene: Sikawa kapitana
id: sikawa_kapitana
end: true
Dopiero kapitan Arvo wszystko mu wytłumaczył.
`), 0644)
	srv := NewServer(rootDir, storyteller.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Init starts in library view: verify topbar tabs, + Dodaj książkę button, and book cover card structure
	initForm := url.Values{}
	initForm.Set("action", "Init")
	r1, err := http.PostForm(ts.URL+"/api/action", initForm)
	if err != nil {
		t.Fatalf("POST /api/action Init failed: %v", err)
	}
	libHTMLBytes, _ := io.ReadAll(r1.Body)
	_ = r1.Body.Close()
	libHTML := string(libHTMLBytes)

	if !strings.Contains(libHTML, `<span class="brand-logo">Mojra</span>`) ||
		!strings.Contains(libHTML, `<span class="brand-logo brand-logo-short">M</span>`) ||
		!strings.Contains(libHTML, `data-view="library"`) ||
		!strings.Contains(libHTML, `id="tab-player"`) ||
		!strings.Contains(libHTML, `data-view="editor"`) ||
		!strings.Contains(libHTML, `class="tab-icon"`) ||
		!strings.Contains(libHTML, `id="btn-topbar-menu"`) ||
		!strings.Contains(libHTML, `id="icon-theme-sun"`) ||
		!strings.Contains(libHTML, `id="icon-theme-moon"`) ||
		strings.Contains(libHTML, `action="OpenQRModal"`) {
		t.Fatalf("expected responsive topbar with ST short logo, tab icons, hamburger menu, sun/moon theme icons, and no gear icon in prod, got:\n%s", libHTML)
	}

	secStart := strings.Index(libHTML, `<section class="view-panel" id="view-library">`)
	secEnd := strings.Index(libHTML, `</section>`)
	if secStart < 0 || secEnd <= secStart {
		t.Fatalf("expected #view-library section in HTML, got:\n%s", libHTML)
	}
	libSection := libHTML[secStart:secEnd]

	if !strings.Contains(libSection, `action="OpenAddBookModal"`) {
		t.Fatalf("expected OpenAddBookModal (+ Dodaj książkę) in #view-library, got:\n%s", libSection)
	}
	cardStart := strings.Index(libSection, `<article class="book-card">`)
	if cardStart < 0 || strings.Count(libSection[cardStart:], "<button") != 1 || !strings.Contains(libSection[cardStart:], `class="book-cover-wrap"`) {
		t.Fatalf("regression in .book-card: expected only 1 button (.book-cover-wrap with play icon on cover), got:\n%s", libSection)
	}
	if !strings.Contains(libSection, `<progress class="book-progress-bar"`) {
		t.Fatalf("expected visual progress bar in #view-library, got:\n%s", libSection)
	}
	for _, forbidden := range []string{"book-progress-text", "%", "book-card-actions", "Kontynuuj słuchanie</button>", "Resetuj postęp"} {
		if strings.Contains(libSection, forbidden) {
			t.Fatalf("regression in #view-library: forbidden element/text %q found in:\n%s", forbidden, libSection)
		}
	}

	// 2. Open book in player view: scene 1 ("Wstyd w warsztacie") has 2 choices (.choice-branch-icon),
	// and choosing choice 0 leads to scene 2 ("Zdrada ojca") which has 1 choice ("»")
	openForm := url.Values{}
	openForm.Set("action", "OpenBook")
	openForm.Set("book", "nie-boj-sie-policji-rzecznej")
	openForm.Set("view", "player")
	openResp, err := http.PostForm(ts.URL+"/api/action", openForm)
	if err != nil {
		t.Fatalf("OpenBook player failed: %v", err)
	}
	playerHTMLBytes, _ := io.ReadAll(openResp.Body)
	_ = openResp.Body.Close()
	playerHTML := string(playerHTMLBytes)
	if openResp.Header.Get("X-Reset-Canvas") != "true" {
		t.Fatalf("expected X-Reset-Canvas=true on OpenBook, got %q", openResp.Header.Get("X-Reset-Canvas"))
	}
	if !strings.Contains(playerHTML, `class="choice-branch-icon"`) {
		t.Fatalf("expected multiple choices to render branching arrow icon (.choice-branch-icon), got:\n%s", playerHTML)
	}

	f := url.Values{}
	f.Set("action", "ChooseBranch")
	f.Set("choice_idx", "1")
	resp, err := http.PostForm(ts.URL+"/api/action", f)
	if err != nil {
		t.Fatalf("ChooseBranch failed: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	playerHTML = string(b)
	if !strings.Contains(playerHTML, `<span class="choice-arrow">»</span>`) || !strings.Contains(playerHTML, `<span class="choice-label-text">Dalej</span>`) {
		t.Fatalf("expected implicit next to render 'Dalej' and '»' arrow, got:\n%s", playerHTML)
	}

	// 3. Clear title of scene "sikawa_kapitana" (which has explicit_id="sikawa_kapitana") via SaveScene,
	// then verify player renders it with .scene-untitled and without .scene-header-block (single separator line)
	saveUntitled := url.Values{}
	saveUntitled.Set("action", "SaveScene")
	saveUntitled.Set("scene_id", "sikawa_kapitana")
	saveUntitled.Set("explicit_id", "sikawa_kapitana")
	saveUntitled.Set("name", "")
	rSave, err := http.PostForm(ts.URL+"/api/action", saveUntitled)
	if err != nil {
		t.Fatalf("SaveScene untitled failed: %v", err)
	}
	_ = rSave.Body.Close()

	playUntitled := url.Values{}
	playUntitled.Set("action", "PlayFromScene")
	playUntitled.Set("scene", "sikawa_kapitana")
	rPlay, err := http.PostForm(ts.URL+"/api/action", playUntitled)
	if err != nil {
		t.Fatalf("PlayFromScene untitled failed: %v", err)
	}
	untitledBytes, _ := io.ReadAll(rPlay.Body)
	_ = rPlay.Body.Close()
	untitledHTML := string(untitledBytes)

	activeStageIdx := strings.Index(untitledHTML, `id="active-scene-stage"`)
	if activeStageIdx < 0 {
		t.Fatalf("expected #active-scene-stage in player HTML")
	}
	activeStageChunk := untitledHTML[activeStageIdx:]
	if !strings.Contains(activeStageChunk, "scene-untitled") {
		t.Fatalf("expected .scene-untitled class on untitled active scene, got:\n%s", activeStageChunk)
	}
	if strings.Contains(activeStageChunk, `class="scene-header-block"`) {
		t.Fatalf("expected untitled scene to omit .scene-header-block so only single separator line is displayed, got:\n%s", activeStageChunk)
	}

	// 4. Verify ToggleImageless disables images and adds .is-imageless to #player-layout
	imgForm := url.Values{}
	imgForm.Set("action", "ToggleImageless")
	rImg, err := http.PostForm(ts.URL+"/api/action", imgForm)
	if err != nil {
		t.Fatalf("ToggleImageless failed: %v", err)
	}
	imgBytes, _ := io.ReadAll(rImg.Body)
	_ = rImg.Body.Close()
	imgHTML := string(imgBytes)
	if rImg.Header.Get("X-Imageless") != "true" || rImg.Header.Get("X-Keep-Scroll") != "true" {
		t.Fatalf("expected X-Imageless=true and X-Keep-Scroll=true headers, got %q / %q", rImg.Header.Get("X-Imageless"), rImg.Header.Get("X-Keep-Scroll"))
	}
	if !strings.Contains(imgHTML, `class="player-layout is-imageless"`) {
		t.Fatalf("expected .player-layout.is-imageless in imageless mode, got:\n%s", imgHTML)
	}
	if strings.Contains(imgHTML, `class="player-bg-img"`) || strings.Contains(imgHTML, `class="scene-portrait-img"`) {
		t.Fatalf("expected scene images to be omitted in imageless mode, got:\n%s", imgHTML)
	}

	// 5. Verify ToggleUIScale and SetUIScale switch between small and large UI controls
	scaleForm := url.Values{}
	scaleForm.Set("action", "ToggleUIScale")
	rScale, err := http.PostForm(ts.URL+"/api/action", scaleForm)
	if err != nil {
		t.Fatalf("ToggleUIScale failed: %v", err)
	}
	scaleBytes, _ := io.ReadAll(rScale.Body)
	_ = rScale.Body.Close()
	if rScale.Header.Get("X-UI-Scale") != "large" || rScale.Header.Get("X-Keep-Scroll") != "true" || rScale.Header.Get("X-Target") != "none" {
		t.Fatalf("expected X-UI-Scale=large, X-Keep-Scroll=true, and X-Target=none headers, got %q / %q / %q", rScale.Header.Get("X-UI-Scale"), rScale.Header.Get("X-Keep-Scroll"), rScale.Header.Get("X-Target"))
	}
	if len(scaleBytes) != 0 || rScale.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 No Content with empty body for ToggleUIScale, got status=%d len=%d", rScale.StatusCode, len(scaleBytes))
	}
	if !strings.Contains(libHTML, `id="btn-toggle-ui-scale"`) {
		t.Fatalf("expected #btn-toggle-ui-scale in topbar HTML")
	}

	setSmallForm := url.Values{}
	setSmallForm.Set("action", "SetUIScale")
	setSmallForm.Set("scale", "small")
	rSetSmall, err := http.PostForm(ts.URL+"/api/action", setSmallForm)
	if err != nil {
		t.Fatalf("SetUIScale failed: %v", err)
	}
	_ = rSetSmall.Body.Close()
	if rSetSmall.Header.Get("X-UI-Scale") != "small" {
		t.Fatalf("expected X-UI-Scale=small header after SetUIScale(small), got %q", rSetSmall.Header.Get("X-UI-Scale"))
	}

	// 6. Verify QuitApp closes immediately without any confirmation modal
	quitForm := url.Values{}
	quitForm.Set("action", "QuitApp")
	rQuit, err := http.PostForm(ts.URL+"/api/action", quitForm)
	if err != nil {
		t.Fatalf("QuitApp failed: %v", err)
	}
	quitBytes, _ := io.ReadAll(rQuit.Body)
	_ = rQuit.Body.Close()
	quitHTML := string(quitBytes)
	if rQuit.Header.Get("X-Native-Cmd") != "quit" {
		t.Fatalf("expected X-Native-Cmd=quit header on QuitApp, got %q", rQuit.Header.Get("X-Native-Cmd"))
	}
	if strings.Contains(quitHTML, `id="modal-confirm"`) || strings.Contains(quitHTML, "Zamknij StoryTeller") {
		t.Fatalf("expected no confirmation prompt when closing app, got:\n%s", quitHTML)
	}

	// 7. Verify targeted HTML chunks (X-Target) instead of full #app shell on partial actions
	swEd := url.Values{}
	swEd.Set("action", "SwitchView")
	swEd.Set("view", "editor")
	rSwEd, err := http.PostForm(ts.URL+"/api/action", swEd)
	if err != nil {
		t.Fatalf("SwitchView editor failed: %v", err)
	}
	_ = rSwEd.Body.Close()

	// 7a. SelectStudioScene returns only #editor-context-panel (not the canvas or topbar)
	selForm := url.Values{}
	selForm.Set("action", "SelectStudioScene")
	selForm.Set("scene", "samotnosc_nad_woda")
	rSel, err := http.PostForm(ts.URL+"/api/action", selForm)
	if err != nil {
		t.Fatalf("SelectStudioScene failed: %v", err)
	}
	selBytes, _ := io.ReadAll(rSel.Body)
	_ = rSel.Body.Close()
	selHTML := string(selBytes)
	if rSel.Header.Get("X-Target") != "#editor-context-panel" {
		t.Fatalf("expected X-Target=#editor-context-panel for SelectStudioScene, got %q", rSel.Header.Get("X-Target"))
	}
	if !strings.Contains(selHTML, `id="editor-context-panel"`) || !strings.Contains(selHTML, "Samotność nad wodą") {
		t.Fatalf("expected #editor-context-panel chunk for Samotność nad wodą, got:\n%s", selHTML)
	}
	for _, unrequested := range []string{`id="topbar"`, `id="timeline-scroll-viewport"`, `id="studio-canvas"`} {
		if strings.Contains(selHTML, unrequested) {
			t.Fatalf("SelectStudioScene chunk must not include %s, got:\n%s", unrequested, selHTML)
		}
	}

	// 7b. OpenEditorHelpModal returns #workspace with right-column #editor-help-panel visible
	helpModalForm := url.Values{}
	helpModalForm.Set("action", "OpenEditorHelpModal")
	rHelp, err := http.PostForm(ts.URL+"/api/action", helpModalForm)
	if err != nil {
		t.Fatalf("OpenEditorHelpModal failed: %v", err)
	}
	helpChunkBytes, _ := io.ReadAll(rHelp.Body)
	_ = rHelp.Body.Close()
	helpChunk := string(helpChunkBytes)
	if rHelp.Header.Get("X-Target") != "#workspace" {
		t.Fatalf("expected X-Target=#workspace for OpenEditorHelpModal, got %q", rHelp.Header.Get("X-Target"))
	}
	if !strings.Contains(helpChunk, `id="editor-help-panel"`) || strings.Contains(helpChunk, `id="editor-help-panel" class="editor-help-panel hidden"`) {
		t.Fatalf("expected visible #editor-help-panel in workspace chunk, got:\n%s", helpChunk)
	}
	if strings.Contains(helpChunk, `id="topbar"`) {
		t.Fatalf("workspace chunk must not include #topbar, got:\n%s", helpChunk)
	}

	// 7c. ChooseBranch returns #workspace (player_view) without #topbar
	swPlay := url.Values{}
	swPlay.Set("action", "SwitchView")
	swPlay.Set("view", "player")
	rSwPlay, _ := http.PostForm(ts.URL+"/api/action", swPlay)
	_ = rSwPlay.Body.Close()

	chBranch := url.Values{}
	chBranch.Set("action", "ChooseBranch")
	chBranch.Set("choice_idx", "0")
	rChBranch, err := http.PostForm(ts.URL+"/api/action", chBranch)
	if err != nil {
		t.Fatalf("ChooseBranch failed: %v", err)
	}
	chBranchBytes, _ := io.ReadAll(rChBranch.Body)
	_ = rChBranch.Body.Close()
	chBranchHTML := string(chBranchBytes)
	if rChBranch.Header.Get("X-Target") != "#workspace" {
		t.Fatalf("expected X-Target=#workspace for ChooseBranch, got %q", rChBranch.Header.Get("X-Target"))
	}
	if !strings.Contains(chBranchHTML, `id="view-player"`) || strings.Contains(chBranchHTML, `id="topbar"`) {
		t.Fatalf("expected ChooseBranch to render only #view-player without #topbar, got:\n%s", chBranchHTML)
	}
}

func TestStandaloneBookModeSecurity(t *testing.T) {
	rootDir := t.TempDir()
	booksDir := filepath.Join(rootDir, "books")
	_ = os.MkdirAll(booksDir, 0755)

	createdBook, err := book.CreateNewBook(booksDir, "Samotna Wyspa", "Autor Komercyjny", true)
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}
	// Add an image to the second scene ("rozdroze") so we can test unvisited scene media protection
	rozdrozeImgPath := filepath.Join(createdBook.RootPath, "images", "rozdroze.png")
	_ = os.WriteFile(rozdrozeImgPath, []byte("\x89PNG\r\n\x1a\nfake"), 0644)
	createdBook.Scenes[1].ExplicitImage = "images/rozdroze.png"
	_ = book.SaveBookScenesToDisk(createdBook)
	reloadedBook, err := book.LoadBookFromDir(createdBook.RootPath)
	if err != nil {
		t.Fatalf("LoadBookFromDir failed: %v", err)
	}

	// Write a fake disk override in rootDir/app/app.js and rootDir/build/app/app.js to verify standalone mode ignores disk overrides
	_ = os.MkdirAll(filepath.Join(rootDir, "app"), 0755)
	_ = os.MkdirAll(filepath.Join(rootDir, "build", "app"), 0755)
	_ = os.WriteFile(filepath.Join(rootDir, "app", "app.js"), []byte("FAKE_DISK_OVERRIDE"), 0644)
	_ = os.WriteFile(filepath.Join(rootDir, "build", "app", "app.js"), []byte("FAKE_DISK_OVERRIDE"), 0644)

	srv := NewStandaloneServer(rootDir, storyteller.AppFS, reloadedBook)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Initial GET / renders player view with the book title and omits Library & Node Studio tabs
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	htmlBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	html := string(htmlBytes)
	if !strings.Contains(html, `<span class="brand-logo">Samotna Wyspa</span>`) {
		t.Fatalf("expected book title in .brand-logo in standalone mode, got:\n%s", html)
	}
	for _, forbidden := range []string{"id=\"tab-library\"", "id=\"tab-editor\"", "id=\"view-library\"", "id=\"view-editor\"", "Playbook Studio"} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("expected %q to be absent in standalone mode HTML", forbidden)
		}
	}

	// 2. Verify GET /app.js serves embedded asset and ignores local disk overrides
	jsResp, err := http.Get(ts.URL + "/app.js")
	if err != nil {
		t.Fatalf("GET /app.js failed: %v", err)
	}
	jsBody, _ := io.ReadAll(jsResp.Body)
	_ = jsResp.Body.Close()
	if strings.Contains(string(jsBody), "FAKE_DISK_OVERRIDE") {
		t.Fatalf("expected standalone server to ignore local disk app.js overrides")
	}

	// 3. Verify all Editor & Library actions are rejected with 403 Forbidden
	for _, forbiddenAction := range []string{
		"SwitchView",
		"OpenBook",
		"CreateNewBook",
		"AddBookFromDisk",
		"AddBookFromURL",
		"SaveScene",
		"AutoSaveScene",
		"SaveEditorState",
		"CreateSceneAt",
		"InsertScene",
		"InsertSceneBefore",
		"PromptDeleteScene",
		"PromptDeleteScenes",
		"DeleteScene",
		"DeleteScenes",
		"ConnectChoice",
		"AddChoice",
		"PromptRemoveChoice",
		"RemoveChoice",
		"BranchChoiceScene",
		"RenameChapter",
		"AssignChapterToScenes",
		"CreateChapterGroup",
		"AutoLayoutNodes",
		"PackBook",
		"RemoveBook",
		"UndoEditor",
		"RedoEditor",
		"ToggleEditorHistoryPanel",
		"OpenEditorHistoryPanel",
		"CloseEditorHistoryPanel",
		"SaveHistoryVersion",
		"RestoreHistoryRevision",
	} {
		f := url.Values{}
		f.Set("action", forbiddenAction)
		rAct, err := http.PostForm(ts.URL+"/api/action", f)
		if err != nil {
			t.Fatalf("POST /api/action %s failed: %v", forbiddenAction, err)
		}
		_ = rAct.Body.Close()
		if rAct.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for action %q in standalone mode, got %d", forbiddenAction, rAct.StatusCode)
		}
	}

	// 4. Verify media route restricts unvisited scenes and allows cover + active/visited scenes
	coverResp, _ := http.Get(ts.URL + "/api/books/samotna-wyspa/media/cover.png")
	_ = coverResp.Body.Close()
	if coverResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for cover.png in standalone mode, got %d", coverResp.StatusCode)
	}

	curImgResp, _ := http.Get(ts.URL + "/api/books/samotna-wyspa/media/images/poczatek.png")
	_ = curImgResp.Body.Close()
	if curImgResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for active scene image images/poczatek.png, got %d", curImgResp.StatusCode)
	}

	unvisitedResp, _ := http.Get(ts.URL + "/api/books/samotna-wyspa/media/images/rozdroze.png")
	_ = unvisitedResp.Body.Close()
	if unvisitedResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for unvisited scene media images/rozdroze.png, got %d", unvisitedResp.StatusCode)
	}

	// Advance to "rozdroze" and verify both history ("poczatek") and current ("rozdroze") media are now unlocked
	advForm := url.Values{}
	advForm.Set("action", "ChooseBranch")
	advForm.Set("choice_idx", "0")
	rAdv, err := http.PostForm(ts.URL+"/api/action", advForm)
	if err != nil {
		t.Fatalf("ChooseBranch in standalone mode failed: %v", err)
	}
	_ = rAdv.Body.Close()
	if rAdv.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for ChooseBranch in standalone mode, got %d", rAdv.StatusCode)
	}

	visitedResp, _ := http.Get(ts.URL + "/api/books/samotna-wyspa/media/images/rozdroze.png")
	_ = visitedResp.Body.Close()
	if visitedResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for newly unlocked scene media images/rozdroze.png, got %d", visitedResp.StatusCode)
	}

	histImgResp, _ := http.Get(ts.URL + "/api/books/samotna-wyspa/media/images/poczatek.png")
	_ = histImgResp.Body.Close()
	if histImgResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for history scene media images/poczatek.png, got %d", histImgResp.StatusCode)
	}
}

func TestSecurityCSRFMediaMagicBytesAndExternalTxtValidation(t *testing.T) {
	rootDir := t.TempDir()
	booksDir := filepath.Join(rootDir, "books")
	_ = os.MkdirAll(booksDir, 0755)

	createdBook, err := book.CreateNewBook(booksDir, "Bezpieczna Twierdza", "Strażnik", true)
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}

	// Write disguised HTML/script files with media extensions inside the book
	disguisedPNG := filepath.Join(createdBook.RootPath, "images", "disguised.png")
	disguisedOGG := filepath.Join(createdBook.RootPath, "voice", "disguised.ogg")
	disguisedFLAC := filepath.Join(createdBook.RootPath, "voice", "disguised.flac")
	disguisedMP3 := filepath.Join(createdBook.RootPath, "voice", "disguised.mp3")
	disguisedWAV := filepath.Join(createdBook.RootPath, "voice", "disguised.wav")
	_ = os.WriteFile(disguisedPNG, []byte("<!DOCTYPE html><html><script>evil()</script></html>"), 0644)
	_ = os.WriteFile(disguisedOGG, []byte("<html><body><script>evil()</script></body></html>"), 0644)
	_ = os.WriteFile(disguisedFLAC, []byte("<html><body><script>evil()</script></body></html>"), 0644)
	_ = os.WriteFile(disguisedMP3, []byte("#!/bin/sh\necho pwned\n"), 0644)
	_ = os.WriteFile(disguisedWAV, []byte("RIFF\x24\x00\x00\x00WAVEfmt "), 0644)

	srv := NewServer(rootDir, storyteller.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Cross-origin POST /api/action requests must be rejected with 403 Forbidden
	for _, tc := range []struct {
		name   string
		header string
		val    string
	}{
		{"cross-origin header", "Origin", "https://evil.example"},
		{"null origin header", "Origin", "null"},
		{"cross-site sec-fetch-site", "Sec-Fetch-Site", "cross-site"},
		{"mismatched referer", "Referer", "https://attacker.example/csrf.html"},
	} {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/action", strings.NewReader("action=ToggleTheme"))
		if err != nil {
			t.Fatalf("NewRequest failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set(tc.header, tc.val)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s request failed: %v", tc.name, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for %s (%s: %s), got %d", tc.name, tc.header, tc.val, resp.StatusCode)
		}
	}

	// 2. Disguised script/HTML files renamed to .png, .ogg, .flac, .mp3 and dropped .wav format must be rejected with 403 Forbidden and never sniff as text/html
	for _, mediaRel := range []string{"images/disguised.png", "voice/disguised.ogg", "voice/disguised.flac", "voice/disguised.mp3", "voice/disguised.wav"} {
		resp, err := http.Get(ts.URL + "/api/books/" + createdBook.ID + "/media/" + mediaRel)
		if err != nil {
			t.Fatalf("GET media %s failed: %v", mediaRel, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for %s, got %d", mediaRel, resp.StatusCode)
		}
		if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
			t.Fatalf("disguised media %s must never return text/html Content-Type, got %q", mediaRel, resp.Header.Get("Content-Type"))
		}
	}

	// 2b. Adding media from disk for the edited book in the Editor (AddBookMediaFromDisk)
	extMediaDir := t.TempDir()
	extImgPath := filepath.Join(extMediaDir, "nowa_ilustracja.png")
	extAudPath := filepath.Join(extMediaDir, "nowy_glos.flac")
	extWavPath := filepath.Join(extMediaDir, "stary.wav")
	_ = book.GenerateAtmosphericPNG(extImgPath, 64, 64, color.RGBA{10, 20, 30, 255}, color.RGBA{200, 180, 120, 255}, 1)
	_ = os.WriteFile(extAudPath, []byte("fLaC\x80\x00\x00\x22"), 0644)
	_ = os.WriteFile(extWavPath, []byte("RIFF\x24\x00\x00\x00WAVEfmt "), 0644)

	firstSceneID := srv.ActiveBook.Scenes[0].ID
	addImgForm := url.Values{
		"action": {"AddBookMediaFromDisk"},
		"kind":   {"image"},
		"scene":  {firstSceneID},
		"path":   {extImgPath},
	}
	rImg, err := http.PostForm(ts.URL+"/api/action", addImgForm)
	if err != nil {
		t.Fatalf("AddBookMediaFromDisk (image) failed: %v", err)
	}
	_ = rImg.Body.Close()
	if rImg.StatusCode != http.StatusOK {
		t.Fatalf("AddBookMediaFromDisk (image) status = %d", rImg.StatusCode)
	}
	if srv.ActiveBook.Scenes[0].ExplicitImage != "images/nowa_ilustracja.png" {
		t.Fatalf("expected scene ExplicitImage to be images/nowa_ilustracja.png, got %q", srv.ActiveBook.Scenes[0].ExplicitImage)
	}

	addAudForm := url.Values{
		"action": {"AddBookMediaFromDisk"},
		"kind":   {"audio"},
		"scene":  {firstSceneID},
		"path":   {extAudPath},
	}
	rAud, err := http.PostForm(ts.URL+"/api/action", addAudForm)
	if err != nil {
		t.Fatalf("AddBookMediaFromDisk (audio) failed: %v", err)
	}
	_ = rAud.Body.Close()
	if rAud.StatusCode != http.StatusOK {
		t.Fatalf("AddBookMediaFromDisk (audio) status = %d", rAud.StatusCode)
	}
	if srv.ActiveBook.Scenes[0].ExplicitVoice != "voice/nowy_glos.flac" {
		t.Fatalf("expected scene ExplicitVoice to be voice/nowy_glos.flac, got %q", srv.ActiveBook.Scenes[0].ExplicitVoice)
	}

	addWavForm := url.Values{
		"action": {"AddBookMediaFromDisk"},
		"kind":   {"audio"},
		"scene":  {firstSceneID},
		"path":   {extWavPath},
	}
	rWav, err := http.PostForm(ts.URL+"/api/action", addWavForm)
	if err != nil {
		t.Fatalf("AddBookMediaFromDisk (wav) failed: %v", err)
	}
	_ = rWav.Body.Close()
	if srv.ActiveBook.Scenes[0].ExplicitVoice == "voice/stary.wav" {
		t.Fatalf("expected .wav file to be rejected by AddBookMediaFromDisk")
	}

	// 3. AddBookFromDisk must reject directories containing only unrelated non-book .txt files
	unrelatedDir := t.TempDir()
	unrelatedTxt := filepath.Join(unrelatedDir, "secret_notes.txt")
	_ = os.WriteFile(unrelatedTxt, []byte("To są prywatne notatki użytkownika bez nagłówków StoryTeller.\nHasło: tajne123\n"), 0644)

	form := url.Values{}
	form.Set("action", "AddBookFromDisk")
	form.Set("path", unrelatedDir)
	rDisk, err := http.PostForm(ts.URL+"/api/action", form)
	if err != nil {
		t.Fatalf("AddBookFromDisk failed: %v", err)
	}
	bodyBytes, _ := io.ReadAll(rDisk.Body)
	_ = rDisk.Body.Close()
	if strings.Contains(string(bodyBytes), "secret_notes") || strings.Contains(string(bodyBytes), "Hasło: tajne123") {
		t.Fatalf("expected unrelated .txt directory to be rejected, but content leaked into response")
	}
}

func TestTopbarAlwaysVisibleIncludingSceneTextEditor(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))

	// 1. Verify AGENTS.md exists and enforces the topbar rule
	agentsBytes, err := os.ReadFile(filepath.Join(repoRoot, "AGENTS.md"))
	if err != nil {
		t.Fatalf("expected AGENTS.md at repository root: %v", err)
	}
	agentsText := string(agentsBytes)
	for _, required := range []string{
		"ALWAYS Keep the Top Bar",
		"header.topbar#topbar",
		"#modal-scene-text",
		"scene-text-fullscreen-backdrop",
	} {
		if !strings.Contains(agentsText, required) {
			t.Fatalf("AGENTS.md missing required topbar rule %q", required)
		}
	}

	// 2. Rebuild app assets and verify CSS/template invariants so #topbar is never covered by #modal-scene-text
	appDir := filepath.Join(repoRoot, "app")
	if err := tpl.BuildApp(appDir); err != nil {
		t.Fatalf("tpl.BuildApp failed: %v", err)
	}

	cssBytes, err := os.ReadFile(filepath.Join(repoRoot, "build", "app", "style.css"))
	if err != nil {
		t.Fatalf("failed to read compiled style.css: %v", err)
	}
	css := string(cssBytes)

	if !strings.Contains(css, "grid-template-rows: auto 1fr") {
		t.Fatalf("expected .app-root to use 'grid-template-rows: auto 1fr' so #topbar stays in row 1")
	}
	topbarIdx := strings.Index(css, ".topbar {")
	if topbarIdx < 0 {
		t.Fatalf("expected .topbar rule in compiled style.css")
	}
	topbarBlock := css[topbarIdx : topbarIdx+strings.Index(css[topbarIdx:], "}")]
	if !strings.Contains(topbarBlock, "grid-row: 1") || !strings.Contains(topbarBlock, "z-index: 200") {
		t.Fatalf("expected .topbar to have grid-row: 1 and z-index: 200, got:\n%s", topbarBlock)
	}

	backdropIdx := strings.Index(css, ".modal-backdrop {")
	if backdropIdx < 0 {
		t.Fatalf("expected .modal-backdrop rule in compiled style.css")
	}
	backdropBlock := css[backdropIdx : backdropIdx+strings.Index(css[backdropIdx:], "}")]
	if !strings.Contains(backdropBlock, "position: absolute") || strings.Contains(backdropBlock, "position: fixed") {
		t.Fatalf(".modal-backdrop must use position: absolute inside row 2 (.modals-root) and NEVER position: fixed over #topbar, got:\n%s", backdropBlock)
	}

	// 3. Verify runtime behavior when opening #modal-scene-text and clicking topbar navigation tabs
	rootDir := t.TempDir()
	srv := NewServer(rootDir, storyteller.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	createForm := url.Values{
		"action":   {"CreateNewBook"},
		"title":    {"Księga Rzeki"},
		"author":   {"Autor"},
		"narrated": {"false"},
	}
	rCreate, err := http.PostForm(ts.URL+"/api/action", createForm)
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}
	_ = rCreate.Body.Close()

	// Open scene text modal with form scene_id (returns full app_shell) and verify #topbar is present alongside #modal-scene-text
	openFullForm := url.Values{
		"action":   {"OpenSceneTextModal"},
		"scene":    {"poczatek"},
		"scene_id": {"poczatek"},
	}
	rOpenFull, err := http.PostForm(ts.URL+"/api/action", openFullForm)
	if err != nil {
		t.Fatalf("OpenSceneTextModal failed: %v", err)
	}
	openFullBytes, _ := io.ReadAll(rOpenFull.Body)
	_ = rOpenFull.Body.Close()
	openFullHTML := string(openFullBytes)
	if !strings.Contains(openFullHTML, `<header class="topbar" id="topbar">`) ||
		!strings.Contains(openFullHTML, `data-view="library"`) ||
		!strings.Contains(openFullHTML, `id="tab-player"`) ||
		!strings.Contains(openFullHTML, `data-view="editor"`) ||
		!strings.Contains(openFullHTML, `id="modal-scene-text"`) {
		t.Fatalf("expected #topbar with all navigation tabs to remain present when #modal-scene-text is open, got:\n%s", openFullHTML)
	}

	// Clicking a topbar tab (SwitchView) while #modal-scene-text is open must close #modal-scene-text and switch cleanly
	swForm := url.Values{
		"action": {"SwitchView"},
		"view":   {"editor"},
	}
	rSw, err := http.PostForm(ts.URL+"/api/action", swForm)
	if err != nil {
		t.Fatalf("SwitchView from scene text modal failed: %v", err)
	}
	swBytes, _ := io.ReadAll(rSw.Body)
	_ = rSw.Body.Close()
	swHTML := string(swBytes)
	if srv.SceneTextModalID != "" || strings.Contains(swHTML, `id="modal-scene-text"`) {
		t.Fatalf("expected SwitchView from #topbar to close #modal-scene-text, got SceneTextModalID=%q", srv.SceneTextModalID)
	}
	if !strings.Contains(swHTML, `<header class="topbar" id="topbar">`) || !strings.Contains(swHTML, `id="view-editor"`) {
		t.Fatalf("expected #topbar and #view-editor after SwitchView, got:\n%s", swHTML)
	}
}

func TestBookImagelessSettingAndTopbarLock(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, storyteller.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Create a normal book (imageless: false) and verify #btn-toggle-imageless is active
	normalForm := url.Values{
		"action":    {"CreateNewBook"},
		"title":     {"Ilustrowana Opowieść"},
		"author":    {"Autor"},
		"narrated":  {"false"},
		"imageless": {"false"},
	}
	rNorm, err := http.PostForm(ts.URL+"/api/action", normalForm)
	if err != nil {
		t.Fatalf("CreateNewBook normal failed: %v", err)
	}
	normBytes, _ := io.ReadAll(rNorm.Body)
	_ = rNorm.Body.Close()
	normHTML := string(normBytes)
	if rNorm.Header.Get("X-Imageless") != "false" || rNorm.Header.Get("X-Imageless-Locked") != "false" {
		t.Fatalf("expected X-Imageless=false and X-Imageless-Locked=false for normal book, got %q / %q", rNorm.Header.Get("X-Imageless"), rNorm.Header.Get("X-Imageless-Locked"))
	}
	if strings.Contains(normHTML, `id="btn-toggle-imageless" type="button" action="ToggleImageless" disabled="disabled"`) {
		t.Fatalf("expected #btn-toggle-imageless to be active (not disabled) when non-imageless book is loaded")
	}
	if !strings.Contains(normHTML, `action="ToggleBookImageless"`) || !strings.Contains(normHTML, `action="ToggleBookNarrated"`) {
		t.Fatalf("expected ToggleBookImageless and ToggleBookNarrated in book settings form")
	}

	// 2. Toggle book imageless mode ON in Node Studio via ToggleBookImageless
	toggleBookImg := url.Values{
		"action": {"ToggleBookImageless"},
	}
	rTogOn, err := http.PostForm(ts.URL+"/api/action", toggleBookImg)
	if err != nil {
		t.Fatalf("ToggleBookImageless ON failed: %v", err)
	}
	togOnBytes, _ := io.ReadAll(rTogOn.Body)
	_ = rTogOn.Body.Close()
	togOnHTML := string(togOnBytes)

	if !srv.ActiveBook.Imageless {
		t.Fatalf("expected ActiveBook.Imageless=true after ToggleBookImageless")
	}
	if rTogOn.Header.Get("X-Imageless") != "true" || rTogOn.Header.Get("X-Imageless-Locked") != "true" {
		t.Fatalf("expected X-Imageless=true and X-Imageless-Locked=true when imageless book is loaded, got %q / %q", rTogOn.Header.Get("X-Imageless"), rTogOn.Header.Get("X-Imageless-Locked"))
	}
	if !strings.Contains(togOnHTML, `class="top-action-btn active is-disabled" id="btn-toggle-imageless" type="button" action="ToggleImageless" disabled="disabled"`) {
		t.Fatalf("expected #btn-toggle-imageless to be disabled/inactive when imageless book is open, got:\n%s", togOnHTML)
	}
	if !strings.Contains(togOnHTML, `class="action-icon hidden" id="icon-imageless-off"`) ||
		strings.Contains(togOnHTML, `class="action-icon hidden" id="icon-imageless-on"`) {
		t.Fatalf("expected #btn-toggle-imageless to show no-image icon (#icon-imageless-on) when imageless book is open")
	}

	// Verify data.txt persisted imageless: true
	dataBytes, err := os.ReadFile(filepath.Join(srv.ActiveBook.RootPath, "data.txt"))
	if err != nil || !strings.Contains(string(dataBytes), "imageless: true") {
		t.Fatalf("expected data.txt to contain 'imageless: true', got:\n%s", string(dataBytes))
	}

	// 3. Attempting ToggleImageless while imageless book is open must NOT disable imageless mode
	tryToggle := url.Values{
		"action": {"ToggleImageless"},
	}
	rTry, err := http.PostForm(ts.URL+"/api/action", tryToggle)
	if err != nil {
		t.Fatalf("ToggleImageless failed: %v", err)
	}
	_ = rTry.Body.Close()
	if rTry.Header.Get("X-Imageless") != "true" || rTry.Header.Get("X-Imageless-Locked") != "true" || srv.Settings.Imageless {
		t.Fatalf("expected ToggleImageless to be ignored while imageless book is loaded, got X-Imageless=%q Settings.Imageless=%v", rTry.Header.Get("X-Imageless"), srv.Settings.Imageless)
	}

	// 4. Switch to player view and verify book is forced to imageless mode (no scene images rendered)
	swPlay := url.Values{
		"action": {"SwitchView"},
		"view":   {"player"},
	}
	rPlay, err := http.PostForm(ts.URL+"/api/action", swPlay)
	if err != nil {
		t.Fatalf("SwitchView player failed: %v", err)
	}
	playBytes, _ := io.ReadAll(rPlay.Body)
	_ = rPlay.Body.Close()
	playHTML := string(playBytes)
	if !strings.Contains(playHTML, `class="player-layout is-imageless"`) || strings.Contains(playHTML, `class="player-bg-img"`) {
		t.Fatalf("expected player view to be forced into .is-imageless without scene images, got:\n%s", playHTML)
	}

	// 5. Toggle book imageless mode OFF in Node Studio -> topbar button becomes active again
	rTogOff, err := http.PostForm(ts.URL+"/api/action", toggleBookImg)
	if err != nil {
		t.Fatalf("ToggleBookImageless OFF failed: %v", err)
	}
	togOffBytes, _ := io.ReadAll(rTogOff.Body)
	_ = rTogOff.Body.Close()
	togOffHTML := string(togOffBytes)
	if srv.ActiveBook.Imageless || rTogOff.Header.Get("X-Imageless-Locked") != "false" || rTogOff.Header.Get("X-Imageless") != "false" {
		t.Fatalf("expected imageless lock released after ToggleBookImageless OFF, got Book.Imageless=%v X-Imageless-Locked=%q", srv.ActiveBook.Imageless, rTogOff.Header.Get("X-Imageless-Locked"))
	}
	if strings.Contains(togOffHTML, `id="btn-toggle-imageless" type="button" action="ToggleImageless" disabled="disabled"`) {
		t.Fatalf("expected #btn-toggle-imageless to become active again when no imageless book is loaded")
	}
}

func TestNodeEditorEmptyLibraryRenameFitAndRootScene(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, storyteller.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. With no books in library (ActiveBook == nil), switching to Node Studio ("editor") MUST open #view-editor
	swForm := url.Values{
		"action": {"SwitchView"},
		"view":   {"editor"},
	}
	rSw, err := http.PostForm(ts.URL+"/api/action", swForm)
	if err != nil {
		t.Fatalf("SwitchView to editor with empty library failed: %v", err)
	}
	swBytes, _ := io.ReadAll(rSw.Body)
	_ = rSw.Body.Close()
	swHTML := string(swBytes)
	if !strings.Contains(swHTML, `id="view-editor"`) || !strings.Contains(swHTML, `id="editor-empty-state"`) || !strings.Contains(swHTML, `action="OpenNewBookModal"`) {
		t.Fatalf("expected #view-editor with #editor-empty-state and OpenNewBookModal when library is empty, got:\n%s", swHTML)
	}

	// 2. Open new book modal and create a new book from Node Editor
	openModalForm := url.Values{
		"action": {"OpenNewBookModal"},
	}
	rModal, err := http.PostForm(ts.URL+"/api/action", openModalForm)
	if err != nil {
		t.Fatalf("OpenNewBookModal failed: %v", err)
	}
	modalBytes, _ := io.ReadAll(rModal.Body)
	_ = rModal.Body.Close()
	if !strings.Contains(string(modalBytes), `data-backdrop-action="CloseNewBookModal"`) || !strings.Contains(string(modalBytes), `action="CreateNewBook"`) {
		t.Fatalf("expected new book modal and CreateNewBook form, got:\n%s", string(modalBytes))
	}

	createForm := url.Values{
		"action":   {"CreateNewBook"},
		"title":    {"Stara Kronika"},
		"author":   {"Autor"},
		"narrated": {"false"},
	}
	rCreate, err := http.PostForm(ts.URL+"/api/action", createForm)
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}
	createBytes, _ := io.ReadAll(rCreate.Body)
	_ = rCreate.Body.Close()
	createHTML := string(createBytes)
	if !strings.Contains(createHTML, `id="editor-book-title"`) || !strings.Contains(createHTML, `id="book-settings-title-input"`) || !strings.Contains(createHTML, `value="Stara Kronika"`) {
		t.Fatalf("expected non-editable #editor-book-title and editable #book-settings-title-input with 'Stara Kronika', got:\n%s", createHTML)
	}
	if !strings.Contains(createHTML, `data-canvas-zoom="fit"`) {
		t.Fatalf("expected data-canvas-zoom=fit in editor toolbar, got:\n%s", createHTML)
	}
	if !strings.Contains(createHTML, "start:1") ||
		!strings.Contains(createHTML, `class="node-start-indicator"`) ||
		!strings.Contains(createHTML, `id="btn-book-settings"`) ||
		!strings.Contains(createHTML, `book-start-scene-select`) ||
		strings.Contains(createHTML, `id="btn-set-start-scene"`) {
		t.Fatalf("expected start:1, read-only start indicator, and book settings start selector without scene SetStartScene button, got:\n%s", createHTML)
	}

	// 3. Alter book title via RenameBook
	renameForm := url.Values{
		"action": {"RenameBook"},
		"title":  {"Nowa Kronika Rzeki"},
	}
	rRename, err := http.PostForm(ts.URL+"/api/action", renameForm)
	if err != nil {
		t.Fatalf("RenameBook failed: %v", err)
	}
	renameBytes, _ := io.ReadAll(rRename.Body)
	_ = rRename.Body.Close()
	renameHTML := string(renameBytes)
	if srv.ActiveBook == nil || srv.ActiveBook.Title != "Nowa Kronika Rzeki" {
		t.Fatalf("expected ActiveBook.Title to be 'Nowa Kronika Rzeki', got %+v", srv.ActiveBook)
	}
	if !strings.Contains(renameHTML, `value="Nowa Kronika Rzeki"`) {
		t.Fatalf("expected updated book title in #editor-book-title-input, got:\n%s", renameHTML)
	}
	dataTxtBytes, err := os.ReadFile(filepath.Join(srv.ActiveBook.RootPath, "data.txt"))
	if err != nil || !strings.Contains(string(dataTxtBytes), "title: Nowa Kronika Rzeki") {
		t.Fatalf("expected data.txt to persist 'title: Nowa Kronika Rzeki', got:\n%s", string(dataTxtBytes))
	}

	// 4. Select second scene ("rozdroze") and set it as root/start scene via SetStartScene
	setStartForm := url.Values{
		"action": {"SetStartScene"},
		"scene":  {"rozdroze"},
	}
	rStart, err := http.PostForm(ts.URL+"/api/action", setStartForm)
	if err != nil {
		t.Fatalf("SetStartScene failed: %v", err)
	}
	startBytes, _ := io.ReadAll(rStart.Body)
	_ = rStart.Body.Close()
	startHTML := string(startBytes)
	if srv.ActiveBook.StartSceneID != "rozdroze" {
		t.Fatalf("expected StartSceneID='rozdroze', got %q", srv.ActiveBook.StartSceneID)
	}
	if !strings.Contains(startHTML, `class="node-start-indicator"`) {
		t.Fatalf("expected active start scene indicator in inspector after SetStartScene, got:\n%s", startHTML)
	}
	dataTxtAfterStart, err := os.ReadFile(filepath.Join(srv.ActiveBook.RootPath, "data.txt"))
	if err != nil || !strings.Contains(string(dataTxtAfterStart), "start: rozdroze") {
		t.Fatalf("expected data.txt to persist 'start: rozdroze', got:\n%s", string(dataTxtAfterStart))
	}

	// 5. Open Book Settings via ToggleBookSettings and Chapter Settings via SelectStudioChapter / SetChapterStartScene
	rBookSettings, err := http.PostForm(ts.URL+"/api/action", url.Values{"action": {"ToggleBookSettings"}})
	if err != nil {
		t.Fatalf("ToggleBookSettings failed: %v", err)
	}
	bookSettingsBytes, _ := io.ReadAll(rBookSettings.Body)
	_ = rBookSettings.Body.Close()
	if !srv.BookSettingsOpen || !strings.Contains(string(bookSettingsBytes), `id="editor-book-form"`) {
		t.Fatalf("expected BookSettingsOpen=true and #editor-book-form, got:\n%s", string(bookSettingsBytes))
	}

	rSelCh, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":  {"SelectStudioChapter"},
		"chapter": {"Rozdział 1"},
	})
	if err != nil {
		t.Fatalf("SelectStudioChapter failed: %v", err)
	}
	selChBytes, _ := io.ReadAll(rSelCh.Body)
	_ = rSelCh.Body.Close()
	if srv.BookSettingsOpen || srv.SelectedChapter != "Rozdział 1" || !strings.Contains(string(selChBytes), `id="editor-chapter-form"`) {
		t.Fatalf("expected SelectedChapter='Rozdział 1' and #editor-chapter-form, got:\n%s", string(selChBytes))
	}

	rChStart, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":  {"SetChapterStartScene"},
		"chapter": {"Rozdział 1"},
		"scene":   {"poczatek"},
	})
	if err != nil {
		t.Fatalf("SetChapterStartScene failed: %v", err)
	}
	_ = rChStart.Body.Close()
	if len(srv.ActiveBook.Chapters) == 0 || srv.ActiveBook.Chapters[0].StartSceneID != "poczatek" {
		t.Fatalf("expected Chapter[0].StartSceneID='poczatek', got %+v", srv.ActiveBook.Chapters)
	}
	chFileBytes, err := os.ReadFile(filepath.Join(srv.ActiveBook.RootPath, "rozdzial_1.txt"))
	if err != nil || !strings.Contains(string(chFileBytes), "start: poczatek") {
		t.Fatalf("expected rozdzial_1.txt to persist chapter 'start: poczatek', got:\n%s", string(chFileBytes))
	}
}

func TestNarrationlessAllOrNothingAndMinimalProgressBar(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, storyteller.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Create a narrationless book (narrated: false)
	createForm := url.Values{
		"action":   {"CreateNewBook"},
		"title":    {"Cicha Opowieść"},
		"author":   {"Autor"},
		"narrated": {"false"},
	}
	rCreate, err := http.PostForm(ts.URL+"/api/action", createForm)
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}
	edBytes, _ := io.ReadAll(rCreate.Body)
	_ = rCreate.Body.Close()
	edHTML := string(edBytes)

	// Verify narrationless book has no voice files or voice headers on disk
	textFileName := "story.txt"
	if len(srv.ActiveBook.TextFiles) > 0 && srv.ActiveBook.TextFiles[0] != "" {
		textFileName = srv.ActiveBook.TextFiles[0]
	}
	storyBytes, err := os.ReadFile(filepath.Join(srv.ActiveBook.RootPath, textFileName))
	if err != nil {
		t.Fatalf("failed to read %s: %v", textFileName, err)
	}
	if strings.Contains(string(storyBytes), "voice:") {
		t.Fatalf("expected narrationless story.txt to have no voice: headers, got:\n%s", string(storyBytes))
	}
	for _, sc := range srv.ActiveBook.Scenes {
		if sc.ExplicitVoice != "" || sc.AudioFile != "" || sc.AudioURL != "" {
			t.Fatalf("expected narrationless scene %q to have no voice, got %+v", sc.ID, sc)
		}
	}

	// Verify Node Studio inspector does NOT render voice: field or audio import button when narrated: false
	if strings.Contains(edHTML, `name="explicit_voice"`) || strings.Contains(edHTML, `data-kind="audio"`) {
		t.Fatalf("expected Node Studio inspector to hide voice controls when book is narrationless, got:\n%s", edHTML)
	}

	// Verify attempting to save explicit_voice or import audio on a narrationless book is rejected/ignored
	saveForm := url.Values{
		"action":         {"SaveScene"},
		"scene_id":       {"poczatek"},
		"explicit_voice": {"voice/poczatek.flac"},
	}
	rSave, err := http.PostForm(ts.URL+"/api/action", saveForm)
	if err != nil {
		t.Fatalf("SaveScene failed: %v", err)
	}
	_ = rSave.Body.Close()
	if srv.ActiveBook.Scenes[0].ExplicitVoice != "" || srv.ActiveBook.Scenes[0].AudioURL != "" {
		t.Fatalf("expected explicit_voice to be ignored when saving scene in narrationless book")
	}

	extDir := t.TempDir()
	extAudio := filepath.Join(extDir, "test.flac")
	_ = os.WriteFile(extAudio, []byte("fLaC\x80\x00\x00\x22"), 0644)
	addAudForm := url.Values{
		"action": {"AddBookMediaFromDisk"},
		"kind":   {"audio"},
		"scene":  {"poczatek"},
		"path":   {extAudio},
	}
	rAddAud, err := http.PostForm(ts.URL+"/api/action", addAudForm)
	if err != nil {
		t.Fatalf("AddBookMediaFromDisk failed: %v", err)
	}
	_ = rAddAud.Body.Close()
	if srv.ActiveBook.Scenes[0].ExplicitVoice != "" || len(srv.ActiveBook.AudioFiles) != 0 {
		t.Fatalf("expected audio import to be rejected when book is narrationless")
	}

	// 2. Switch to read mode (player view) when narrationless:
	// Do NOT show playhead (#btn-audio-play) or scrubber (#audio-seek-slider) or #audio-deck at all;
	// Instead show minimalistic progress bar (#reader-progress-footer / #reader-book-progress) with NO percentage or text.
	swPlay := url.Values{
		"action": {"SwitchView"},
		"view":   {"player"},
	}
	rPlay, err := http.PostForm(ts.URL+"/api/action", swPlay)
	if err != nil {
		t.Fatalf("SwitchView player failed: %v", err)
	}
	playBytes, _ := io.ReadAll(rPlay.Body)
	_ = rPlay.Body.Close()
	playHTML := string(playBytes)

	for _, forbidden := range []string{
		`id="audio-deck"`,
		`id="story-audio"`,
		`id="btn-audio-play"`,
		`id="audio-seek-slider"`,
		`id="audio-time-current"`,
		`id="audio-time-total"`,
	} {
		if strings.Contains(playHTML, forbidden) {
			t.Fatalf("expected %s to be absent in read mode when book is narrationless, got:\n%s", forbidden, playHTML)
		}
	}

	footerStart := strings.Index(playHTML, `<footer class="reader-progress-footer" id="reader-progress-footer">`)
	if footerStart < 0 {
		t.Fatalf("expected #reader-progress-footer in read mode when book is narrationless, got:\n%s", playHTML)
	}
	footerEnd := strings.Index(playHTML[footerStart:], `</footer>`)
	if footerEnd < 0 {
		t.Fatalf("expected closing </footer> for #reader-progress-footer")
	}
	footerChunk := playHTML[footerStart : footerStart+footerEnd+len("</footer>")]
	if !strings.Contains(footerChunk, `<progress class="reader-book-progress" value="50" max="100"></progress>`) {
		t.Fatalf("expected minimalistic <progress> bar at 50 (1 of 2 scenes) in #reader-progress-footer, got:\n%s", footerChunk)
	}
	if strings.Contains(footerChunk, "%") {
		t.Fatalf("expected no percentage sign or text in #reader-progress-footer, got:\n%s", footerChunk)
	}

	// Advance to scene 2 ("rozdroze") and verify progress bar advances to 100
	advForm := url.Values{
		"action":     {"ChooseBranch"},
		"choice_idx": {"0"},
	}
	rAdv, err := http.PostForm(ts.URL+"/api/action", advForm)
	if err != nil {
		t.Fatalf("ChooseBranch failed: %v", err)
	}
	advBytes, _ := io.ReadAll(rAdv.Body)
	_ = rAdv.Body.Close()
	advHTML := string(advBytes)
	if !strings.Contains(advHTML, `<progress class="reader-book-progress" value="100" max="100"></progress>`) {
		t.Fatalf("expected .reader-book-progress value=100 on scene 2 of 2, got:\n%s", advHTML)
	}

	// 3. Toggle narration ON in Node Studio -> voice selector appears in editor and audio deck appears in player
	swEd := url.Values{
		"action": {"SwitchView"},
		"view":   {"editor"},
	}
	rSwEd, _ := http.PostForm(ts.URL+"/api/action", swEd)
	_ = rSwEd.Body.Close()

	togNarr := url.Values{
		"action": {"ToggleBookNarrated"},
	}
	rTogOn, err := http.PostForm(ts.URL+"/api/action", togNarr)
	if err != nil {
		t.Fatalf("ToggleBookNarrated ON failed: %v", err)
	}
	togOnBytes, _ := io.ReadAll(rTogOn.Body)
	_ = rTogOn.Body.Close()
	togOnHTML := string(togOnBytes)
	if !srv.ActiveBook.Narrated {
		t.Fatalf("expected ActiveBook.Narrated=true after ToggleBookNarrated")
	}
	if !strings.Contains(togOnHTML, `name="explicit_voice"`) || !strings.Contains(togOnHTML, `data-kind="audio"`) {
		t.Fatalf("expected voice controls to appear in Node Studio when book narration is enabled, got:\n%s", togOnHTML)
	}
}

func TestUnreachableScenesGrayedOutInNodeEditor(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, storyteller.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"CreateNewBook"},
		"title":  {"Test Unreachable"},
		"author": {"Autor"},
	})

	createResp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"CreateSceneAt"},
		"x":      {"480"},
		"y":      {"480"},
	})
	if err != nil {
		t.Fatalf("CreateSceneAt failed: %v", err)
	}
	createBytes, _ := io.ReadAll(createResp.Body)
	_ = createResp.Body.Close()
	createHTML := string(createBytes)

	if !strings.Contains(createHTML, "unreachable:1") {
		t.Fatalf("expected unconnected scene to have unreachable:1 in editor graph payload, got:\n%s", createHTML)
	}

	orphanID := srv.ActiveBook.Scenes[len(srv.ActiveBook.Scenes)-1].ID
	if !srv.ActiveBook.Scenes[len(srv.ActiveBook.Scenes)-1].Unreachable {
		t.Fatalf("expected newly created standalone scene %s to be marked Unreachable", orphanID)
	}

	connResp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":     {"ConnectChoice"},
		"from_scene": {"rozdroze"},
		"choice_idx": {"0"},
		"to_scene":   {orphanID},
	})
	if err != nil {
		t.Fatalf("ConnectChoice failed: %v", err)
	}
	connBytes, _ := io.ReadAll(connResp.Body)
	_ = connResp.Body.Close()
	connHTML := string(connBytes)

	if strings.Contains(connHTML, "unreachable:1") {
		t.Fatalf("expected no unreachable:1 once all scenes are reachable, got:\n%s", connHTML)
	}
}

func TestEditorUndoRedoAndVersionHistory(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, storyteller.AppFS)
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
	srv := NewServer(rootDir, storyteller.AppFS)
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
	srv := NewServer(rootDir, storyteller.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Create a new book -> must create rozdzial_1.txt (not story.txt)
	_, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"CreateNewBook"},
		"title":  {"Rzeka Czasu"},
		"author": {"Autor"},
	})
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(srv.ActiveBook.RootPath, "rozdzial_1.txt")); err != nil {
		t.Fatalf("expected mandatory split chapter file rozdzial_1.txt on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(srv.ActiveBook.RootPath, "story.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected no story.txt in newly created book")
	}

	// 2. Create a second chapter group ("Rozdział 2")
	_, err = http.PostForm(ts.URL+"/api/action", url.Values{
		"action":  {"CreateChapterGroup"},
		"chapter": {"Rozdział 2"},
	})
	if err != nil {
		t.Fatalf("CreateChapterGroup failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(srv.ActiveBook.RootPath, "rozdzial_2.txt")); err != nil {
		t.Fatalf("expected rozdzial_2.txt on disk after CreateChapterGroup: %v", err)
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

	_ = os.WriteFile(filepath.Join(bookDir, "data.txt"), []byte("title: Prefetch\nauthor: Autor\nnarrated: true\nstart: s1\n"), 0644)
	_ = os.WriteFile(filepath.Join(bookDir, "rozdzial_1.txt"), []byte(`chapter: Rozdział 1
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

	srv := NewServer(rootDir, storyteller.AppFS)
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
	srv := NewServer(rootDir, storyteller.AppFS)
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

func TestAllActionSpecsHaveHandlers(t *testing.T) {
	if len(actionSpecs) == 0 {
		t.Fatal("expected actionSpecs to be non-empty")
	}
	for name, spec := range actionSpecs {
		if spec.Handler == nil {
			t.Fatalf("action %q in actionSpecs has a nil Handler", name)
		}
	}
}

func TestScreenplayModeEndToEndFlow(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, storyteller.AppFS)
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
	if _, statErr := os.Stat(filepath.Join(expectedDiskRoot, "data.txt")); statErr != nil {
		t.Fatalf("expected data.txt on disk at %s: %v", expectedDiskRoot, statErr)
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
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"DeleteHistoryBranch"},
		"branch": {"wersja-rezysera"},
	})

	// 5b. PromptCreateHistoryBranch opens confirmation modal and ConfirmModalAccept creates auto-named "wariant-2"
	promptBrResp, _ := http.PostForm(ts.URL+"/api/action", url.Values{
		"action":      {"PromptCreateHistoryBranch"},
		"branch_name": {""},
	})
	promptBrBytes, _ := io.ReadAll(promptBrResp.Body)
	_ = promptBrResp.Body.Close()
	if !strings.Contains(string(promptBrBytes), "Utwórz nowy wariant") || !strings.Contains(string(promptBrBytes), "wariant-2") {
		t.Fatalf("expected confirmation modal for wariant-2, got: %s", string(promptBrBytes))
	}
	autoBrResp, _ := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"ConfirmModalAccept"},
	})
	autoBrBytes, _ := io.ReadAll(autoBrResp.Body)
	_ = autoBrResp.Body.Close()
	if !strings.Contains(string(autoBrBytes), "wariant-2") || !strings.Contains(string(autoBrBytes), `class="history-active-branch-name"`) {
		t.Fatalf("expected auto-named branch wariant-2 and active branch bar in history graph HTML")
	}
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SwitchHistoryBranch"},
		"branch": {"main"},
	})
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"DeleteHistoryBranch"},
		"branch": {"wariant-2"},
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
	if !srv.ExportModalOpen || !strings.Contains(openExpHTML, `id="modal-export-project"`) || !strings.Contains(openExpHTML, `data-select-value="pdf"`) || !strings.Contains(openExpHTML, `data-select-value="fountain"`) || !strings.Contains(openExpHTML, `data-select-value="playbook"`) {
		t.Fatalf("expected Export Modal with pdf/fountain/playbook dropdown options, got:\n%s", openExpHTML)
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
}

func TestClientServerActionAndOverlayContracts(t *testing.T) {
	appDir := filepath.Clean(filepath.Join("..", "..", "app"))

	// 1. Verify every action referenced in .tpl templates exists in actionSpecs
	tplFiles, err := filepath.Glob(filepath.Join(appDir, "templates", "*.tpl"))
	if err != nil || len(tplFiles) == 0 {
		t.Fatalf("failed to glob .tpl files: %v", err)
	}
	tplActionRe := regexp.MustCompile(`(?m)^\s*(?:action|data-backdrop-action):\s*([A-Za-z0-9_]+)\s*$`)
	forLoopRe := regexp.MustCompile(`^\s*\{for\s+([A-Za-z0-9_]+)\s+in\s+[A-Za-z0-9_.]+\}\s*$`)
	propRefRe := regexp.MustCompile(`\{([a-z][A-Za-z0-9_]*)\.[A-Za-z0-9_]+\}`)

	knownGlobalObjects := map[string]bool{
		"current_scene": true,
		"selected_node": true,
		"selected_chapter": true,
	}

	for _, path := range tplFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		src := string(raw)
		base := filepath.Base(path)
		for _, m := range tplActionRe.FindAllStringSubmatch(src, -1) {
			act := m[1]
			if _, ok := actionSpecs[act]; !ok {
				t.Fatalf("template %s references unknown server action %q", base, act)
			}
		}

		// Verify {for item in list} blocks never reference sibling loop variables out of scope
		lines := strings.Split(src, "\n")
		type loopScope struct {
			indent  int
			varName string
		}
		var stack []loopScope
		for lineNo, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			for len(stack) > 0 && indent <= stack[len(stack)-1].indent {
				stack = stack[:len(stack)-1]
			}
			if fm := forLoopRe.FindStringSubmatch(line); fm != nil {
				stack = append(stack, loopScope{indent: indent, varName: fm[1]})
				continue
			}
			if len(stack) > 0 {
				for _, pm := range propRefRe.FindAllStringSubmatch(line, -1) {
					objName := pm[1]
					if knownGlobalObjects[objName] {
						continue
					}
					inScope := false
					for _, sc := range stack {
						if sc.varName == objName {
							inScope = true
							break
						}
					}
					if !inScope {
						t.Fatalf("template %s:%d references %q outside its {for} loop scope (active scopes: %+v)", base, lineNo+1, pm[0], stack)
					}
				}
			}
		}
	}

	// 2. Verify every literal action dispatched from app/js/*.js exists in actionSpecs
	jsFiles := tpl.JSModuleFiles(appDir)
	jsActionRe := regexp.MustCompile(`(?:sendServerAction|postServerActionSilent)\(\s*"([A-Za-z0-9_]+)"`)
	for _, path := range jsFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		src := string(raw)
		base := filepath.Base(path)
		for _, m := range jsActionRe.FindAllStringSubmatch(src, -1) {
			act := m[1]
			if _, ok := actionSpecs[act]; !ok {
				t.Fatalf("JS module %s dispatches unknown server action %q", base, act)
			}
		}
	}

	// 3. Verify CSS overlay pointer-events and stacking contexts never block UI interaction
	cssProd, err := tpl.BundleFilesToString(tpl.CSSModuleFiles(appDir), true)
	if err != nil {
		t.Fatalf("BundleFilesToString(CSS) failed: %v", err)
	}
	for _, check := range []struct {
		block string
		prop  string
	}{
		{block: ".modals-root {", prop: "pointer-events: none;"},
		{block: ".modals-root {", prop: "grid-row: 2;"},
		{block: ".modal-backdrop {", prop: "pointer-events: auto;"},
		{block: ".modal-backdrop {", prop: "position: absolute;"},
		{block: ".toast {", prop: "pointer-events: none;"},
		{block: ".ui-tooltip {", prop: "pointer-events: none;"},
		{block: ".editor-perf-overlay {", prop: "pointer-events: none;"},
		{block: ".player-bg-stage {", prop: "pointer-events: none;"},
		{block: ".scroll-more-cue {", prop: "pointer-events: none;"},
	} {
		idx := strings.Index(cssProd, check.block)
		if idx < 0 {
			t.Fatalf("expected CSS selector block %s in compiled stylesheet", check.block)
		}
		endIdx := strings.Index(cssProd[idx:], "}")
		if endIdx < 0 || !strings.Contains(cssProd[idx:idx+endIdx], check.prop) {
			t.Fatalf("expected %s to contain %q, got:\n%s", check.block, check.prop, cssProd[idx:idx+endIdx])
		}
	}
}

func TestClientJSRuntimeAndInteractiveResponsiveness(t *testing.T) {
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found in PATH; skipping live JS DOM responsiveness test")
	}

	appDir := filepath.Clean(filepath.Join("..", "..", "app"))

	// Verify syntax & clean compilation of all 3 JS bundles (prod, dev, standalone player)
	prodJS, err := tpl.BundleFilesToString(tpl.JSModuleFiles(appDir), true)
	if err != nil {
		t.Fatalf("BundleFilesToString(prod) failed: %v", err)
	}
	devJS, err := tpl.BundleFilesToString(tpl.JSModuleFiles(appDir), false)
	if err != nil {
		t.Fatalf("BundleFilesToString(dev) failed: %v", err)
	}
	playerJS, err := tpl.BundleFilesToString(tpl.JSPlayerModuleFiles(appDir), true)
	if err != nil {
		t.Fatalf("BundleFilesToString(player) failed: %v", err)
	}

	// Run live end-to-end DOM + event-delegation + WebGL canvas + form submission harness in Node
	harnessScript := `
"use strict";
const vm = require("vm");

function parseSimpleStyle(raw) {
  const s = {
    left: "",
    top: "",
    width: "",
    height: "",
    cursor: "",
    transform: "",
    minHeight: "",
    setProperty(k, v) { this[k] = String(v); },
    getPropertyValue(k) { return this[k] || ""; }
  };
  if (raw) {
    raw.split(";").forEach((pair) => {
      const idx = pair.indexOf(":");
      if (idx > 0) {
        const k = pair.slice(0, idx).trim();
        const v = pair.slice(idx + 1).trim();
        s[k] = v;
      }
    });
  }
  return s;
}

function splitTopLevelCommas(sel) {
  const out = [];
  let depth = 0;
  let cur = "";
  for (let i = 0; i < sel.length; i++) {
    const ch = sel[i];
    if (ch === "[") depth++;
    else if (ch === "]") depth = Math.max(0, depth - 1);
    if (ch === "," && depth === 0) {
      if (cur.trim()) out.push(cur.trim());
      cur = "";
    } else {
      cur += ch;
    }
  }
  if (cur.trim()) out.push(cur.trim());
  return out;
}

function splitDescendantTokens(sel) {
  const out = [];
  let depth = 0;
  let cur = "";
  for (let i = 0; i < sel.length; i++) {
    const ch = sel[i];
    if (ch === "[") depth++;
    else if (ch === "]") depth = Math.max(0, depth - 1);
    if (/\s/.test(ch) && depth === 0) {
      if (cur.trim()) out.push(cur.trim());
      cur = "";
    } else {
      cur += ch;
    }
  }
  if (cur.trim()) out.push(cur.trim());
  return out;
}

function matchesCompoundSelector(el, compound) {
  if (!el || el.nodeType !== 1) return false;
  let rem = compound.trim();
  if (!rem) return false;

  const attrMatches = [];
  rem = rem.replace(/\[([^\]]+)\]/g, (_, inner) => {
    attrMatches.push(inner.trim());
    return "";
  });

  const classMatches = [];
  rem = rem.replace(/\.([A-Za-z0-9_-]+)/g, (_, cls) => {
    classMatches.push(cls);
    return "";
  });

  let idMatch = "";
  rem = rem.replace(/#([A-Za-z0-9_-]+)/g, (_, id) => {
    idMatch = id;
    return "";
  });

  const tagMatch = rem.trim().toLowerCase();
  if (tagMatch && tagMatch !== "*" && el.tagName.toLowerCase() !== tagMatch) {
    return false;
  }
  if (idMatch && el.id !== idMatch) {
    return false;
  }
  for (let i = 0; i < classMatches.length; i++) {
    if (!el.classList.contains(classMatches[i])) return false;
  }
  for (let i = 0; i < attrMatches.length; i++) {
    const expr = attrMatches[i];
    const caretEq = expr.indexOf("^=");
    const plainEq = expr.indexOf("=");
    if (caretEq > 0) {
      const k = expr.slice(0, caretEq).trim();
      const v = expr.slice(caretEq + 2).trim().replace(/^["']|["']$/g, "");
      const actual = el.getAttribute(k);
      if (actual === null || !String(actual).startsWith(v)) return false;
    } else if (plainEq > 0) {
      const k = expr.slice(0, plainEq).trim();
      const v = expr.slice(plainEq + 1).trim().replace(/^["']|["']$/g, "");
      const actual = el.getAttribute(k);
      if (actual === null || String(actual) !== v) return false;
    } else {
      if (!el.hasAttribute(expr)) return false;
    }
  }
  return true;
}

function matchesSelector(el, selector) {
  if (!el || el.nodeType !== 1 || !selector) return false;
  const groups = splitTopLevelCommas(selector);
  for (let g = 0; g < groups.length; g++) {
    const chain = splitDescendantTokens(groups[g]);
    if (chain.length === 0) continue;
    if (!matchesCompoundSelector(el, chain[chain.length - 1])) continue;
    let cur = el.parentElement;
    let idx = chain.length - 2;
    while (idx >= 0 && cur) {
      if (matchesCompoundSelector(cur, chain[idx])) {
        idx--;
      }
      cur = cur.parentElement;
    }
    if (idx < 0) return true;
  }
  return false;
}

function decodeEntities(str) {
  if (!str) return "";
  return String(str)
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&amp;/g, "&");
}

class DOMElement {
  constructor(tagName, ownerDocument) {
    this.nodeType = 1;
    this.tagName = String(tagName || "DIV").toUpperCase();
    this.ownerDocument = ownerDocument;
    this.parentElement = null;
    this.children = [];
    this.childNodes = [];
    this._attrs = new Map();
    this._classes = new Set();
    this._listeners = new Map();
    this.style = parseSimpleStyle("");
    this.scrollTop = 0;
    this.scrollLeft = 0;
    this.scrollHeight = 600;
    this.clientWidth = 1200;
    this.clientHeight = 700;
    this.offsetWidth = 180;
    this.offsetHeight = 36;
    this.offsetTop = 0;
    this.selectionStart = 0;
    this.selectionEnd = 0;
    this.complete = true;
    this.readyState = 4;
    this.paused = true;
    this.ended = false;
    this.currentTime = 0;
    this.duration = 30;
    this.playbackRate = 1.0;
    this.width = 1200;
    this.height = 700;
    this._text = "";
    this._value = undefined;

    const self = this;
    this.classList = {
      add(...names) {
        names.forEach((n) => { if (n) self._classes.add(n); });
        self._syncClassAttr();
      },
      remove(...names) {
        names.forEach((n) => self._classes.delete(n));
        self._syncClassAttr();
      },
      toggle(name, force) {
        const shouldHave = force !== undefined ? Boolean(force) : !self._classes.has(name);
        if (shouldHave) self._classes.add(name);
        else self._classes.delete(name);
        self._syncClassAttr();
        return shouldHave;
      },
      contains(name) {
        return self._classes.has(name);
      }
    };

    this.dataset = new Proxy({}, {
      get(_, prop) {
        if (typeof prop !== "string") return undefined;
        const attrName = "data-" + prop.replace(/[A-Z]/g, (m) => "-" + m.toLowerCase());
        const val = self.getAttribute(attrName);
        return val === null ? undefined : val;
      },
      set(_, prop, val) {
        const attrName = "data-" + prop.replace(/[A-Z]/g, (m) => "-" + m.toLowerCase());
        self.setAttribute(attrName, String(val));
        return true;
      },
      ownKeys() {
        const keys = [];
        self._attrs.forEach((_, k) => {
          if (k.startsWith("data-")) {
            const camel = k.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase());
            keys.push(camel);
          }
        });
        return keys;
      },
      getOwnPropertyDescriptor(_, prop) {
        const attrName = "data-" + String(prop).replace(/[A-Z]/g, (m) => "-" + m.toLowerCase());
        if (self._attrs.has(attrName)) {
          return { enumerable: true, configurable: true, value: self._attrs.get(attrName) };
        }
        return undefined;
      }
    });
  }

  _syncClassAttr() {
    if (this._classes.size > 0) {
      this._attrs.set("class", Array.from(this._classes).join(" "));
    } else {
      this._attrs.delete("class");
    }
  }

  get isConnected() {
    let cur = this;
    while (cur) {
      if (cur === this.ownerDocument.documentElement || cur === this.ownerDocument.body) return true;
      cur = cur.parentElement;
    }
    return false;
  }

  get offsetParent() {
    return this.parentElement;
  }

  get id() {
    return this._attrs.get("id") || "";
  }
  set id(v) {
    this.setAttribute("id", v);
  }

  get name() {
    return this._attrs.get("name") || "";
  }
  set name(v) {
    this.setAttribute("name", v);
  }

  get type() {
    return this._attrs.get("type") || (this.tagName === "BUTTON" ? "submit" : "");
  }
  set type(v) {
    this.setAttribute("type", v);
  }

  get disabled() {
    return this._attrs.has("disabled");
  }
  set disabled(v) {
    if (v) this._attrs.set("disabled", "disabled");
    else this._attrs.delete("disabled");
  }

  get value() {
    if (this._value !== undefined) return this._value;
    if (this.tagName === "TEXTAREA") return this.textContent;
    return this._attrs.get("value") || "";
  }
  set value(v) {
    this._value = String(v);
    this._attrs.set("value", String(v));
  }

  hasAttribute(k) {
    return this._attrs.has(String(k).toLowerCase());
  }

  getAttribute(k) {
    const key = String(k).toLowerCase();
    return this._attrs.has(key) ? this._attrs.get(key) : null;
  }

  setAttribute(k, v) {
    const key = String(k).toLowerCase();
    const val = String(v);
    this._attrs.set(key, val);
    if (key === "class") {
      this._classes.clear();
      val.split(/\s+/).forEach((c) => { if (c) this._classes.add(c); });
    } else if (key === "value" && this._value === undefined) {
      this._value = val;
    }
  }

  removeAttribute(k) {
    const key = String(k).toLowerCase();
    this._attrs.delete(key);
    if (key === "class") this._classes.clear();
  }

  appendChild(child) {
    if (!child) return null;
    if (child.parentElement) {
      const idx = child.parentElement.children.indexOf(child);
      if (idx >= 0) child.parentElement.children.splice(idx, 1);
    }
    child.parentElement = this;
    this.children.push(child);
    this.childNodes = this.children;
    return child;
  }

  get textContent() {
    let out = this._text || "";
    for (let i = 0; i < this.children.length; i++) {
      out += this.children[i].textContent;
    }
    return out;
  }
  set textContent(v) {
    this.children = [];
    this.childNodes = [];
    this._text = String(v || "");
    if (this.tagName === "TEXTAREA") {
      this._value = this._text;
    }
  }

  get innerHTML() {
    if (this.children.length === 0 && !this._text) return "";
    return this._text + this.children.map((c) => c.outerHTML).join("");
  }
  set innerHTML(html) {
    this.children = [];
    this.childNodes = [];
    this._text = "";
    if (!html) return;
    const nodes = parseHTMLFragment(String(html), this.ownerDocument);
    nodes.forEach((n) => this.appendChild(n));
  }

  get outerHTML() {
    const tag = this.tagName.toLowerCase();
    let attrs = "";
    this._attrs.forEach((v, k) => {
      attrs += " " + k + '="' + String(v).replace(/"/g, "&quot;") + '"';
    });
    return "<" + tag + attrs + ">" + this.innerHTML + "</" + tag + ">";
  }
  set outerHTML(html) {
    const parent = this.parentElement;
    if (!parent) return;
    const idx = parent.children.indexOf(this);
    if (idx < 0) return;
    const nodes = parseHTMLFragment(String(html), this.ownerDocument);
    this.parentElement = null;
    parent.children.splice(idx, 1, ...nodes);
    nodes.forEach((n) => { n.parentElement = parent; });
    parent.childNodes = parent.children;
  }

  insertAdjacentHTML(pos, html) {
    const nodes = parseHTMLFragment(String(html), this.ownerDocument);
    if (pos === "afterbegin") {
      nodes.forEach((n) => { n.parentElement = this; });
      this.children.unshift(...nodes);
      this.childNodes = this.children;
    } else {
      nodes.forEach((n) => this.appendChild(n));
    }
  }

  matches(sel) {
    return matchesSelector(this, sel);
  }

  closest(sel) {
    let cur = this;
    while (cur && cur.nodeType === 1) {
      if (matchesSelector(cur, sel)) return cur;
      cur = cur.parentElement;
    }
    return null;
  }

  contains(other) {
    let cur = other;
    while (cur) {
      if (cur === this) return true;
      cur = cur.parentElement;
    }
    return false;
  }

  querySelector(sel) {
    const all = this.querySelectorAll(sel);
    return all.length > 0 ? all[0] : null;
  }

  querySelectorAll(sel) {
    const res = [];
    const walk = (node) => {
      for (let i = 0; i < node.children.length; i++) {
        const ch = node.children[i];
        if (matchesSelector(ch, sel)) res.push(ch);
        walk(ch);
      }
    };
    walk(this);
    return res;
  }

  getBoundingClientRect() {
    return { left: 0, top: 0, width: this.clientWidth || 1200, height: this.clientHeight || 700, right: this.clientWidth || 1200, bottom: this.clientHeight || 700 };
  }

  focus() {
    this.ownerDocument.activeElement = this;
  }
  blur() {
    if (this.ownerDocument.activeElement === this) {
      this.ownerDocument.activeElement = this.ownerDocument.body;
    }
  }
  select() {}
  setSelectionRange(s, e) {
    this.selectionStart = s;
    this.selectionEnd = e;
  }
  load() {}
  play() {
    this.paused = false;
    return Promise.resolve();
  }
  pause() {
    this.paused = true;
  }

  getContext(kind) {
    if (kind !== "webgl" && kind !== "experimental-webgl") return null;
    if (!this._gl) {
      const self = this;
      this._glDrawCalls = 0;
      this._glLastInstCount = 0;
      const instExt = {
        vertexAttribDivisorANGLE() {},
        drawArraysInstancedANGLE(_mode, _first, _count, primcount) {
          self._glDrawCalls++;
          self._glLastInstCount = primcount;
        }
      };
      this._gl = {
        VERTEX_SHADER: 35633,
        FRAGMENT_SHADER: 35632,
        COMPILE_STATUS: 35713,
        LINK_STATUS: 35714,
        ARRAY_BUFFER: 34962,
        STATIC_DRAW: 35044,
        DYNAMIC_DRAW: 35048,
        FLOAT: 5126,
        UNSIGNED_BYTE: 5121,
        TEXTURE_2D: 3553,
        TEXTURE0: 33984,
        TEXTURE1: 33985,
        RGBA: 6408,
        LINEAR: 9729,
        CLAMP_TO_EDGE: 33071,
        TEXTURE_MIN_FILTER: 10241,
        TEXTURE_MAG_FILTER: 10240,
        TEXTURE_WRAP_S: 10242,
        TEXTURE_WRAP_T: 10243,
        BLEND: 3042,
        SRC_ALPHA: 770,
        ONE_MINUS_SRC_ALPHA: 771,
        TRIANGLES: 4,
        getExtension(name) { return name === "ANGLE_instanced_arrays" ? instExt : null; },
        createShader() { return {}; },
        shaderSource() {},
        compileShader() {},
        getShaderParameter() { return true; },
        createProgram() { return {}; },
        attachShader() {},
        linkProgram() {},
        getProgramParameter() { return true; },
        useProgram() {},
        createBuffer() { return {}; },
        bindBuffer() {},
        bufferData() {},
        bufferSubData() {},
        getAttribLocation() { return 0; },
        enableVertexAttribArray() {},
        vertexAttribPointer() {},
        getUniformLocation() { return {}; },
        uniform1i() {},
        uniform1f() {},
        uniform2f() {},
        uniform3f() {},
        createTexture() { return {}; },
        activeTexture() {},
        bindTexture() {},
        texParameteri() {},
        texImage2D() {},
        enable() {},
        blendFunc() {},
        viewport() {}
      };
    }
    return this._gl;
  }

  addEventListener(type, fn) {
    if (!this._listeners.has(type)) this._listeners.set(type, []);
    this._listeners.get(type).push(fn);
  }
  removeEventListener(type, fn) {
    const arr = this._listeners.get(type);
    if (!arr) return;
    const idx = arr.indexOf(fn);
    if (idx >= 0) arr.splice(idx, 1);
  }
  dispatchEvent(ev) {
    ev.target = ev.target || this;
    const path = [];
    let cur = this;
    while (cur) {
      path.push(cur);
      cur = cur.parentElement;
    }
    path.push(this.ownerDocument);
    for (let i = 0; i < path.length; i++) {
      if (ev._stopped) break;
      const node = path[i];
      ev.currentTarget = node;
      const list = node._listeners ? node._listeners.get(ev.type) : null;
      if (list) {
        const copy = list.slice();
        for (let j = 0; j < copy.length; j++) {
          copy[j].call(node, ev);
          if (ev._ImmediateStopped) break;
        }
      }
    }
    return !ev.defaultPrevented;
  }
}

const VOID_TAGS = new Set(["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr", "path", "rect", "circle", "line", "polyline", "polygon"]);

function parseHTMLFragment(html, doc) {
  const root = new DOMElement("fragment", doc);
  const stack = [root];
  const tagRe = /<!--[\s\S]*?-->|<!DOCTYPE[^>]*>|<\/([A-Za-z0-9_-]+)\s*>|<([A-Za-z0-9_-]+)\b([^>]*?)>|([^<]+)/gi;
  const attrRe = /([^\s/>=]+)(?:=(?:"([^"]*)"|'([^']*)'|([^\s"'>]+)))?/g;

  let m;
  while ((m = tagRe.exec(html)) !== null) {
    const closeTag = m[1];
    const openTag = m[2];
    const rawAttr = m[3] || "";
    const selfClose = rawAttr.trim().endsWith("/");
    const attrStr = selfClose ? rawAttr.replace(/\/\s*$/, "") : rawAttr;
    const textChunk = m[4];

    if (closeTag) {
      const upper = closeTag.toUpperCase();
      for (let i = stack.length - 1; i >= 1; i--) {
        if (stack[i].tagName === upper) {
          stack.length = i;
          break;
        }
      }
    } else if (openTag) {
      const lower = openTag.toLowerCase();
      if (lower === "doctype") continue;
      const el = new DOMElement(openTag, doc);
      let am;
      attrRe.lastIndex = 0;
      while ((am = attrRe.exec(attrStr)) !== null) {
        const ak = am[1];
        const av = am[2] !== undefined ? am[2] : am[3] !== undefined ? am[3] : am[4] !== undefined ? am[4] : "";
        el.setAttribute(ak, decodeEntities(av));
      }
      stack[stack.length - 1].appendChild(el);
      if (!selfClose && !VOID_TAGS.has(lower)) {
        stack.push(el);
      }
    } else if (textChunk !== undefined) {
      const decoded = decodeEntities(textChunk);
      const top = stack[stack.length - 1];
      if (top.children.length === 0) {
        top._text += decoded;
        if (top.tagName === "TEXTAREA") {
          top._value = top._text;
        }
      } else {
        const span = new DOMElement("#text", doc);
        span._text = decoded;
        top._text += decoded;
      }
    }
  }
  return root.children.slice().map((c) => {
    c.parentElement = null;
    return c;
  });
}

class CustomFormData {
  constructor(formEl) {
    this._pairs = [];
    if (formEl && formEl.querySelectorAll) {
      const fields = formEl.querySelectorAll("input, textarea, select");
      for (let i = 0; i < fields.length; i++) {
        const f = fields[i];
        if (f.disabled) continue;
        const name = f.getAttribute("name");
        if (!name) continue;
        const type = (f.getAttribute("type") || "").toLowerCase();
        if (type === "checkbox" || type === "radio") {
          if (!f.checked && !f.hasAttribute("checked")) continue;
        }
        this._pairs.push([name, f.value || ""]);
      }
    }
  }
  set(k, v) {
    const key = String(k);
    const val = String(v);
    for (let i = 0; i < this._pairs.length; i++) {
      if (this._pairs[i][0] === key) {
        this._pairs[i][1] = val;
        return;
      }
    }
    this._pairs.push([key, val]);
  }
  forEach(cb) {
    for (let i = 0; i < this._pairs.length; i++) {
      cb(this._pairs[i][1], this._pairs[i][0]);
    }
  }
}

function makeEvent(type, props) {
  return Object.assign({
    type: type,
    button: 0,
    clientX: 0,
    clientY: 0,
    key: "",
    code: "",
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    altKey: false,
    cancelable: true,
    defaultPrevented: false,
    _stopped: false,
    _ImmediateStopped: false,
    preventDefault() { this.defaultPrevented = true; },
    stopPropagation() { this._stopped = true; },
    stopImmediatePropagation() { this._stopped = true; this._ImmediateStopped = true; }
  }, props || {});
}

(async () => {
  const baseURL = process.env.ST_TEST_BASE_URL;
  const appJSSrc = process.env.ST_TEST_APP_JS;
  const actionLog = [];
  const inFlight = new Set();
  process.on("unhandledRejection", (reason) => {
    console.error("UNHANDLED_REJECTION=" + (reason && reason.stack ? reason.stack : String(reason)));
    process.exit(1);
  });

  const initResp = await fetch(baseURL + "/");
  if (!initResp.ok) {
    throw new Error("GET / failed: " + initResp.status);
  }
  const initHTML = await initResp.text();

  const doc = {
    nodeType: 9,
    _listeners: new Map(),
    documentElement: null,
    body: null,
    activeElement: null,
    fullscreenElement: null,
    addEventListener(type, fn) {
      if (!this._listeners.has(type)) this._listeners.set(type, []);
      this._listeners.get(type).push(fn);
    },
    removeEventListener() {},
    dispatchEvent(ev) {
      ev.target = ev.target || this.body || this;
      const list = this._listeners.get(ev.type) || [];
      list.slice().forEach((fn) => fn.call(this, ev));
      return !ev.defaultPrevented;
    },
    querySelector(sel) {
      return this.documentElement ? this.documentElement.querySelector(sel) : null;
    },
    querySelectorAll(sel) {
      return this.documentElement ? this.documentElement.querySelectorAll(sel) : [];
    },
    getElementById(id) {
      return this.querySelector("#" + id);
    },
    elementFromPoint() {
      return this.body;
    }
  };
  doc.ownerDocument = doc;

  const parsedRoot = parseHTMLFragment(initHTML, doc);
  const htmlEl = parsedRoot.find((el) => el.tagName === "HTML") || parsedRoot[0];
  doc.documentElement = htmlEl;
  doc.body = htmlEl.querySelector("body") || htmlEl;
  doc.activeElement = doc.body;

  const win = {
    document: doc,
    navigator: { userAgent: "Mozilla/5.0 StoryTellerTest/1.0" },
    innerWidth: 1280,
    innerHeight: 800,
    devicePixelRatio: 2,
    __STUDIO_MSDF_DATA: "",
    CSS: { escape: (s) => String(s) },
    FormData: CustomFormData,
    URLSearchParams: URLSearchParams,
    AbortController: AbortController,
    TextDecoder: TextDecoder,
    Uint8Array: Uint8Array,
    Float32Array: Float32Array,
    Uint32Array: Uint32Array,
    ArrayBuffer: ArrayBuffer,
    Map: Map,
    Set: Set,
    WeakMap: WeakMap,
    WeakSet: WeakSet,
    Promise: Promise,
    Math: Math,
    Number: Number,
    String: String,
    Boolean: Boolean,
    Object: Object,
    Array: Array,
    RegExp: RegExp,
    Error: Error,
    Date: Date,
    Infinity: Infinity,
    NaN: NaN,
    parseInt: parseInt,
    parseFloat: parseFloat,
    isNaN: isNaN,
    console: console,
    performance: { now: () => Date.now() },
    requestAnimationFrame: (cb) => { cb(Date.now()); return 1; },
    cancelAnimationFrame: () => {},
    setTimeout: setTimeout,
    clearTimeout: clearTimeout,
    ResizeObserver: class { observe() {} disconnect() {} },
    Image: class { set src(v) { this._src = v; if (this.onload) this.onload(); } },
    addEventListener(type, fn) { doc.addEventListener("win:" + type, fn); },
    close() {},
    __trackAction: (p) => {
      inFlight.add(p);
      p.finally(() => inFlight.delete(p));
      return p;
    },
    fetch: async (urlStr, opts) => {
      const fullURL = urlStr.startsWith("http") ? urlStr : baseURL + urlStr;
      let bodyStr = "";
      if (opts && opts.body) {
        bodyStr = String(opts.body);
        const ct = opts.headers && opts.headers["Content-Type"] ? String(opts.headers["Content-Type"]) : "";
        if (!ct.includes("text/x-wfp")) {
          throw new Error("Expected WFP Content-Type text/x-wfp, got: " + ct);
        }
        const parsed = typeof win.decodeWFP === "function" ? win.decodeWFP(bodyStr) : {};
        const act = parsed.action;
        if (act) actionLog.push(act + "(" + bodyStr.replace(/\n/g, "|") + ")");
      }
      const p = fetch(fullURL, opts);
      inFlight.add(p);
      try {
        const res = await p;
        return res;
      } finally {
        inFlight.delete(p);
      }
    }
  };
  win.window = win;
  win.globalThis = win;

  vm.createContext(win);
  vm.runInContext(appJSSrc, win, { filename: "app.js" });
  win.__inFlight = inFlight;
  vm.runInContext(
    "const _origSend = sendServerAction; sendServerAction = async function(act, params) { const p = (async () => { try { return await _origSend(act, params); } catch (e) { console.error('SEND_ACTION_ERR=' + act + ':' + (e && e.stack ? e.stack : String(e))); throw e; } })(); __inFlight.add(p); try { return await p; } finally { __inFlight.delete(p); } };",
    win
  );

  // Fire DOMContentLoaded to bind declarative action bridge & canvas gestures
  doc.dispatchEvent(makeEvent("DOMContentLoaded", { target: doc }));

  const waitForActions = async () => {
    while (inFlight.size > 0) {
      await Promise.all(Array.from(inFlight));
    }
    await new Promise((r) => setTimeout(r, 5));
    while (inFlight.size > 0) {
      await Promise.all(Array.from(inFlight));
    }
  };
  const clickEl = async (selectorOrEl) => {
    const el = typeof selectorOrEl === "string" ? doc.querySelector(selectorOrEl) : selectorOrEl;
    if (!el) throw new Error("Missing element to click: " + selectorOrEl);
    el.dispatchEvent(makeEvent("click", { target: el }));
    await waitForActions();
  };

  // 1. Switch to Node Studio (#view-editor) via Topbar tab click
  await clickEl('button.topbar-tab[data-view="editor"]');
  if (!doc.querySelector("#view-editor")) {
    const appChildren = doc.querySelector("#app") ? doc.querySelector("#app").children.map((c) => c.tagName + "#" + c.id).join(",") : "NONE";
    const allIds = doc.querySelectorAll("*").map((c) => c.tagName + "#" + c.id).filter((s) => !s.endsWith("#")).join(",");
    throw new Error("Expected #view-editor; actionLog=" + actionLog.join(" | ") + " appChildren=" + appChildren + " allIds=" + allIds);
  }

  // 2. Click OpenNewBookModal button in Node Studio
  await clickEl('button[action="OpenNewBookModal"]');
  const newBookTitle = doc.querySelector("#new-book-title");
  if (!newBookTitle) {
    throw new Error("Expected #new-book-title in #modals-root after OpenNewBookModal");
  }

  // 3. Test Escape key closing modal
  doc.body.dispatchEvent(makeEvent("keydown", { key: "Escape", code: "Escape", target: doc.body }));
  await waitForActions();
  if (doc.querySelector("#new-book-title")) {
    throw new Error("Expected Escape key to close NewBookModal");
  }

  // 4. Re-open NewBookModal and submit form via button[type="submit"] click
  await clickEl('button[action="OpenNewBookModal"]');
  const titleInp = doc.querySelector("#new-book-title");
  titleInp.value = "Książka Testowa JS";
  await clickEl('form[action="CreateNewBook"] button[type="submit"]');

  const viewport = doc.querySelector("#timeline-scroll-viewport");
  const canvas = doc.querySelector("#studio-canvas");
  if (!viewport || !canvas) {
    throw new Error("Expected #timeline-scroll-viewport and #studio-canvas after CreateNewBook");
  }
  const nodesCount = vm.runInContext("studioGraphState.nodes.length", win);
  if (nodesCount < 1 || !canvas._gl || canvas._glDrawCalls < 1) {
    throw new Error("Expected WebGL canvas to initialize and draw nodes, got nodes=" + nodesCount + " actionLog=" + actionLog.join(" | ") + " toast=" + (doc.querySelector("#toast-banner") ? doc.querySelector("#toast-banner").textContent : ""));
  }

  // 5. Click empty canvas to clear selection, then click node card on WebGL canvas
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: 10, clientY: 10, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: 10, clientY: 10, target: canvas }));
  await waitForActions();
  if (!doc.querySelector("#editor-context-form").classList.contains("hidden")) {
    throw new Error("Expected #editor-context-form to hide after clicking empty canvas");
  }

  const nodeScreenPt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const z = editorCanvasState.zoom; return { x: Math.round((n.x + 60) * z + editorCanvasState.panX), y: Math.round((n.y + 42) * z + editorCanvasState.panY), titleX: Math.round((n.x + 60) * z + editorCanvasState.panX), titleY: Math.round((n.y + 16) * z + editorCanvasState.panY), addX: Math.round((n.x + 20) * z + editorCanvasState.panX), addY: Math.round((n.y + n.h - 20) * z + editorCanvasState.panY), editX: Math.round((n.x + n.w - 20) * z + editorCanvasState.panX), editY: Math.round((n.y + n.h - 20) * z + editorCanvasState.panY), gripX: Math.round((n.x + 12) * z + editorCanvasState.panX), gripY: Math.round((n.y + 16) * z + editorCanvasState.panY) }; })()", win);

  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: nodeScreenPt.x, clientY: nodeScreenPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: nodeScreenPt.x, clientY: nodeScreenPt.y, target: canvas }));
  await waitForActions();
  if (!doc.querySelector("#editor-context-form").classList.contains("hidden") || !doc.querySelector("#editor-context-panel").classList.contains("hidden")) {
    throw new Error("Expected #editor-context-panel and #editor-context-form to remain hidden when clicking a node card on canvas");
  }
  if (!vm.runInContext("selectedNodeIds.has(studioGraphState.nodes[0].id)", win)) {
    throw new Error("Expected clicking node card on canvas to select the node");
  }

  // 5b. Click node title on WebGL canvas to edit title inline via #canvas-node-title-input
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: nodeScreenPt.titleX, clientY: nodeScreenPt.titleY, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: nodeScreenPt.titleX, clientY: nodeScreenPt.titleY, target: canvas }));
  const nodeTitleInp = doc.querySelector("#canvas-node-title-input");
  if (!nodeTitleInp || nodeTitleInp.classList.contains("hidden")) {
    throw new Error("Expected clicking node title on canvas to open #canvas-node-title-input");
  }
  const expectedTitleFont = vm.runInContext("(13 * (editorCanvasState.zoom || 1)).toFixed(2) + 'px'", win);
  if (nodeTitleInp.style.fontSize !== expectedTitleFont) {
    throw new Error("Expected #canvas-node-title-input fontSize " + expectedTitleFont + ", got " + nodeTitleInp.style.fontSize);
  }
  const expectedTitleLeft = vm.runInContext("((studioGraphState.nodes[0].x + 28) * (editorCanvasState.zoom || 1) + (editorCanvasState.panX || 0)).toFixed(2) + 'px'", win);
  if (nodeTitleInp.style.left !== expectedTitleLeft) {
    throw new Error("Expected #canvas-node-title-input left " + expectedTitleLeft + ", got " + nodeTitleInp.style.left);
  }
  nodeTitleInp.value = "Scena Startowa Inline";
  nodeTitleInp.dispatchEvent(makeEvent("keydown", { key: "Enter", target: nodeTitleInp }));
  await waitForActions();
  if (vm.runInContext("studioGraphState.nodes[0].name", win) !== "Scena Startowa Inline") {
    throw new Error("Expected Enter on #canvas-node-title-input to update node name on canvas");
  }

  // 6. Click add-choice icon button (branch-plus) directly on WebGL canvas node card footer
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: nodeScreenPt.addX, clientY: nodeScreenPt.addY, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: nodeScreenPt.addX, clientY: nodeScreenPt.addY, target: canvas }));
  await waitForActions();
  const portCountAfterAdd = vm.runInContext("studioGraphState.nodes[0].ports.length", win);
  if (portCountAfterAdd < 2) {
    throw new Error("Expected canvas add-choice icon click to add choices (ports >= 2), got " + portCountAfterAdd);
  }

  // 6b. Click choice row label on WebGL canvas node card to edit choice inline via #canvas-choice-input
  const choiceLabelPt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const z = editorCanvasState.zoom; return { x: Math.round((n.x + 50) * z + editorCanvasState.panX), y: Math.round((n.y + 36 + 14) * z + editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: choiceLabelPt.x, clientY: choiceLabelPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: choiceLabelPt.x, clientY: choiceLabelPt.y, target: canvas }));
  const choiceInp = doc.querySelector("#canvas-choice-input");
  if (!choiceInp || choiceInp.classList.contains("hidden")) {
    throw new Error("Expected clicking choice label on canvas node to open #canvas-choice-input");
  }
  const expectedChoiceFont = vm.runInContext("(11 * (editorCanvasState.zoom || 1)).toFixed(2) + 'px'", win);
  if (choiceInp.style.fontSize !== expectedChoiceFont) {
    throw new Error("Expected #canvas-choice-input fontSize " + expectedChoiceFont + ", got " + choiceInp.style.fontSize);
  }
  choiceInp.value = "Wybór Inline 1";
  choiceInp.dispatchEvent(makeEvent("keydown", { key: "Enter", target: choiceInp }));
  await waitForActions();
  if (vm.runInContext("studioGraphState.nodes[0].ports[0].label", win) !== "Wybór Inline 1") {
    throw new Error("Expected Enter on #canvas-choice-input to update choice label on canvas");
  }

  // 6c. Connect choice ports to two parallel child scenes, then click gray 8x8 branch-radio square on secondary parallel node to promote it to primary
  const rootId = vm.runInContext("studioGraphState.nodes[0].id", win);
  await vm.runInContext("sendServerAction('BranchChoiceScene', { scene: '" + rootId + "', choice_idx: '0' })", win);
  await waitForActions();
  await vm.runInContext("sendServerAction('BranchChoiceScene', { scene: '" + rootId + "', choice_idx: '1' })", win);
  await waitForActions();
  const secRadioPt = vm.runInContext("(() => { const sec = studioGraphState.nodes.find((n) => n.branchRole === 'secondary'); if (!sec) return null; const z = editorCanvasState.zoom; return { id: sec.id, x: Math.round((sec.branchRadioX + 4) * z + editorCanvasState.panX), y: Math.round((sec.branchRadioY + 4) * z + editorCanvasState.panY) }; })()", win);
  if (!secRadioPt) {
    throw new Error("Expected parallel child node with branchRole=secondary on canvas");
  }
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: secRadioPt.x, clientY: secRadioPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: secRadioPt.x, clientY: secRadioPt.y, target: canvas }));
  await waitForActions();
  const promotedRole = vm.runInContext("(() => { const n = studioGraphState.nodeById.get('" + secRadioPt.id + "'); return n ? n.branchRole : ''; })()", win);
  if (promotedRole !== "primary") {
    throw new Error("Expected clicking gray branch-radio square to promote node to primary, got " + promotedRole);
  }

  // 6d. Drag choice-grip handle on WebGL canvas node card from row 0 to row 1 to reorder choices live + persist via ReorderChoice
  const beforeDragZeroLabel = vm.runInContext("studioGraphState.nodes[0].ports[0].label", win);
  const choiceGripPt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const hit = hitTestStudioCanvas(n.x + 10, n.y + 36 + 14); if (!hit || hit.type !== 'choice-grip') return null; const z = editorCanvasState.zoom; return { x: Math.round((n.x + 10) * z + editorCanvasState.panX), y0: Math.round((n.y + 36 + 14) * z + editorCanvasState.panY), y1: Math.round((n.y + 36 + 42) * z + editorCanvasState.panY) }; })()", win);
  if (!choiceGripPt) {
    throw new Error("Expected hitTestStudioCanvas to return choice-grip on left side of choice row");
  }
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: choiceGripPt.x, clientY: choiceGripPt.y0, target: canvas }));
  canvas.dispatchEvent(makeEvent("mousemove", { button: 0, clientX: choiceGripPt.x, clientY: choiceGripPt.y1, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: choiceGripPt.x, clientY: choiceGripPt.y1, target: canvas }));
  await waitForActions();
  const reorderedSecondLabel = vm.runInContext("studioGraphState.nodes[0].ports[1].label", win);
  if (reorderedSecondLabel !== beforeDragZeroLabel) {
    throw new Error("Expected choice-grip drag to reorder '" + beforeDragZeroLabel + "' from idx 0 to idx 1, got " + reorderedSecondLabel);
  }
  const directRightForkPtsLen = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const rightWire = studioGraphState.wires.find((w) => w.fromNode === n && w.toNode && w.x2 >= n.x + n.w + 16); return rightWire && rightWire.pts ? rightWire.pts.length : 0; })()", win);
  if (directRightForkPtsLen !== 3) {
    throw new Error("Expected right-branch choice wire to route as a clean 3-point L-wire without extra kinks, got pts.length=" + directRightForkPtsLen);
  }

  // 7. Drag node grip on WebGL canvas (+48px X, -24px Y) and verify grid snap + SaveEditorState
  const startNodeX = vm.runInContext("studioGraphState.nodes[0].x", win);
  const updatedGripPt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const z = editorCanvasState.zoom; return { x: Math.round((n.x + 12) * z + editorCanvasState.panX), y: Math.round((n.y + 16) * z + editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: updatedGripPt.x, clientY: updatedGripPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mousemove", { button: 0, clientX: updatedGripPt.x + 48, clientY: updatedGripPt.y - 24, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: updatedGripPt.x + 48, clientY: updatedGripPt.y - 24, target: canvas }));
  await waitForActions();
  const movedNodeX = vm.runInContext("studioGraphState.nodes[0].x", win);
  if (movedNodeX !== startNodeX + 48) {
    throw new Error("Expected node grip drag to snap +48px on 24px grid, got " + startNodeX + " -> " + movedNodeX);
  }

  // 7a. Verify node-on-node collision prevention (48px = 2 grid jumps), single-input socket color inheritance, and frustum culling
  const collisionPrevented = vm.runInContext("(() => { const n0 = studioGraphState.nodes[0]; const n1 = studioGraphState.nodes[1]; const dx = n1.x - n0.x; const dy = n1.y - n0.y; const res = resolveValidNodeDragDelta([{ node: n0, sceneId: n0.id, startX: n0.x, startY: n0.y }], dx, dy, 0, 0); const nx = snapToEditorGrid(n0.x + res.dx); const ny = snapToEditorGrid(n0.y + res.dy); return !(nx < n1.right + 48 && nx + n0.w + 48 > n1.left && ny < n1.bottom + 48 && ny + n0.h + 48 > n1.top); })()", win);
  if (!collisionPrevented) {
    throw new Error("Expected resolveValidNodeDragDelta to enforce 48px (2 grid jumps) spacing between nodes");
  }
  const singleInColoredOk = vm.runInContext("(() => { const w = studioGraphState.wires.find((it) => it.fromNode && it.fromNode.hasChoices && it.fromNode.ports && it.fromNode.ports.length >= 2 && it.toNode && studioGraphState.wires.filter((x) => x.toNode === it.toNode).length === 1); return Boolean(w && w.colorIdx >= 0 && w.toNode.inColorIdx === w.colorIdx); })()", win);
  if (!singleInColoredOk) {
    throw new Error("Expected single-input target node to inherit its incoming choice wire colorIdx on its input socket");
  }
  const multiInDistinctColorsOk = vm.runInContext("(() => { const byT = new Map(); for (const w of studioGraphState.wires) { if (!w.targetGroupKey) continue; const g = byT.get(w.targetGroupKey) || []; g.push(w); byT.set(w.targetGroupKey, g); } for (const g of byT.values()) { if (g.length >= 2 && g.length <= 8) { const s = new Set(g.map((w) => w.colorIdx)); if (s.size !== g.length) return false; } } return true; })()", win);
  if (!multiInDistinctColorsOk) {
    throw new Error("Expected all incoming wires to any multi-input target to have distinct colorIdx values");
  }
  const culledQuadCount = vm.runInContext("(() => { const savedPanX = editorCanvasState.panX; const savedPanY = editorCanvasState.panY; editorCanvasState.panX = 200000; editorCanvasState.panY = 200000; renderStudioCanvas(); const c = studioGL.staticInstCount; editorCanvasState.panX = savedPanX; editorCanvasState.panY = savedPanY; renderStudioCanvas(); return c; })()", win);
  if (culledQuadCount !== 1) {
    throw new Error("Expected frustum culling to emit only 1 background grid quad when all nodes are off-screen, got " + culledQuadCount);
  }

  // 7b. Drag a wire segment on WebGL canvas to insert a control pin (wire-pin), then double-click pin to remove it
  const wireMidScreenPt = vm.runInContext("(() => { const w = studioGraphState.wires[0]; const p0 = w.pts[0]; const p1 = w.pts[1]; const wx = (p0.x + p1.x) * 0.5; const wy = (p0.y + p1.y) * 0.5; const z = editorCanvasState.zoom; return { x: Math.round(wx * z + editorCanvasState.panX), y: Math.round(wy * z + editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: wireMidScreenPt.x, clientY: wireMidScreenPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mousemove", { button: 0, clientX: wireMidScreenPt.x + 36, clientY: wireMidScreenPt.y + 24, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: wireMidScreenPt.x + 36, clientY: wireMidScreenPt.y + 24, target: canvas }));
  await waitForActions();
  const pinCountAfterDrag = vm.runInContext("studioGraphState.wires[0].pins ? studioGraphState.wires[0].pins.length : 0", win);
  if (pinCountAfterDrag !== 1) {
    throw new Error("Expected dragging wire segment to create 1 control pin, got " + pinCountAfterDrag);
  }
  const pinDirsOk = vm.runInContext("(() => { const mockWire = { pts: [{ x: 200, y: 200 }, { x: 200, y: 300 }, { x: 50, y: 300 }, { x: 50, y: 80 }, { x: 320, y: 80 }, { x: 320, y: 160 }] }; return resolveWirePinDirection(mockWire, { x: 200, y: 250 }) === 'down' && resolveWirePinDirection(mockWire, { x: 120, y: 300 }) === 'left' && resolveWirePinDirection(mockWire, { x: 50, y: 180 }) === 'up' && resolveWirePinDirection(mockWire, { x: 180, y: 80 }) === 'right'; })()", win);
  if (!pinDirsOk) {
    throw new Error("Expected resolveWirePinDirection to resolve down, left, up, and right along wire segments");
  }
  const pinScreenPt = vm.runInContext("(() => { const p = studioGraphState.wires[0].pins[0]; const z = editorCanvasState.zoom; return { x: Math.round(p.x * z + editorCanvasState.panX), y: Math.round(p.y * z + editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("dblclick", { button: 0, clientX: pinScreenPt.x, clientY: pinScreenPt.y, target: canvas }));
  await waitForActions();
  const pinCountAfterDblClick = vm.runInContext("studioGraphState.wires[0].pins ? studioGraphState.wires[0].pins.length : 0", win);
  if (pinCountAfterDblClick !== 0) {
    throw new Error("Expected double-clicking wire-pin to remove it, got " + pinCountAfterDblClick);
  }

  // 8. Double-click node card on WebGL canvas to open #modal-scene-text
  const afterMovePt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const z = editorCanvasState.zoom; return { x: Math.round((n.x + 60) * z + editorCanvasState.panX), y: Math.round((n.y + 20) * z + editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("dblclick", { button: 0, clientX: afterMovePt.x, clientY: afterMovePt.y, target: canvas }));
  await waitForActions();
  const modalTa = doc.querySelector("#modal-scene-textarea");
  if (!modalTa) {
    throw new Error("Expected double-clicking node card on canvas to open #modal-scene-text");
  }

  // 9. Toggle scene settings inside #modal-scene-text, edit textarea, and click back button
  await clickEl("[data-scene-settings-toggle]");
  const settingsBlock = doc.querySelector("#modal-scene-settings-block");
  if (!settingsBlock || settingsBlock.classList.contains("hidden")) {
    throw new Error("Expected [data-scene-settings-toggle] to unhide #modal-scene-settings-block");
  }
  modalTa.value = "Treść zapisana przez mostek JS.";
  modalTa.dispatchEvent(makeEvent("input", { target: modalTa }));
  await clickEl('button.scene-text-back-btn[action="CloseSceneTextModal"]');
  if (doc.querySelector("#modal-scene-text")) {
    throw new Error("Expected CloseSceneTextModal to close #modal-scene-text");
  }

  // 10. Topbar toggles (Theme, UI Scale, Hamburger Menu)
  await clickEl("#btn-topbar-menu");
  if (!doc.querySelector("#topbar-actions-menu").classList.contains("open")) {
    throw new Error("Expected #btn-topbar-menu to open #topbar-actions-menu");
  }
  await clickEl("#btn-toggle-ui-scale");
  if (doc.body.getAttribute("data-ui-scale") !== "large") {
    throw new Error("Expected #btn-toggle-ui-scale to set data-ui-scale=large");
  }
  await clickEl('button[action="ToggleTheme"]');
  if (doc.body.getAttribute("data-theme") !== "light") {
    throw new Error("Expected ToggleTheme to set data-theme=light");
  }

  // 11. Switch to Player view via Topbar and click a ChoiceBranch button
  await clickEl('#tab-player');
  if (!doc.querySelector("#view-player")) {
    throw new Error("Expected #view-player after clicking #tab-player");
  }
  const proseText = doc.querySelector("#active-scene-stage .scene-prose").textContent;
  if (!proseText.includes("Treść zapisana przez mostek JS.")) {
    throw new Error("Expected dirty textarea auto-flush on CloseSceneTextModal to persist prose in player view");
  }
  await clickEl('#active-scene-stage button.choice-btn[action="ChooseBranch"]');
  if (!doc.querySelector("#history-stack")) {
    throw new Error("Expected #history-stack in player view after clicking ChooseBranch");
  }

  // 12. Switch to Editor, create a Screenplay project via custom-select mode picker, test Breakdown Panel forms & jump
  await clickEl('button.topbar-tab[data-view="editor"]');
  await clickEl('button[action="OpenNewBookModal"]');
  await clickEl('button[data-form-toggle="narrated"]');
  if (doc.querySelector('#new-book-narrated-field input[name="narrated"]').value !== "true") {
    throw new Error("Expected clicking [data-form-toggle=narrated] to toggle hidden input value to true");
  }
  await clickEl('.new-book-mode-select [data-select-toggle]');
  await clickEl('.new-book-mode-select .custom-select-option[data-select-value="screenplay"]');
  if (doc.querySelector("#new-book-standard-field").classList.contains("hidden")) {
    throw new Error("Expected selecting screenplay mode in custom-select to reveal #new-book-standard-field");
  }
  await clickEl('#new-book-standard-field label.form-radio-option input[value="eu"]');
  if (!doc.querySelector('#new-book-standard-field input[value="eu"]').checked) {
    throw new Error("Expected clicking .form-radio-option to check EU standard radio input");
  }
  doc.querySelector("#new-book-title").value = "Scenariusz Responsywny";
  await clickEl('form[action="CreateNewBook"] button[type="submit"]');

  // Open Scene Text Modal in screenplay mode via edit-scene-btn icon on node card footer and click [data-insert-script-block="dialogue"] pill
  const spEditBtnPt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const z = editorCanvasState.zoom; return { x: Math.round((n.x + n.w - 50) * z + editorCanvasState.panX), y: Math.round((n.y + n.h - 20) * z + editorCanvasState.panY) }; })()", win);
  const spDelHitType = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const h = hitTestStudioCanvas(n.x + n.w - 20, n.y + n.h - 20); return h ? h.type : ''; })()", win);
  if (spDelHitType !== "delete-scene-btn") {
    throw new Error("Expected delete-scene-btn after edit-scene-btn on node card footer, got " + spDelHitType);
  }
  const spCanvas0 = doc.querySelector("#studio-canvas");
  spCanvas0.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: spEditBtnPt.x, clientY: spEditBtnPt.y, target: spCanvas0 }));
  spCanvas0.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: spEditBtnPt.x, clientY: spEditBtnPt.y, target: spCanvas0 }));
  await waitForActions();
  await clickEl('button[data-insert-script-block="dialogue"]');
  const spTaVal = doc.querySelector("#modal-scene-textarea").value;
  if (!spTaVal.includes("POSTAĆ")) {
    throw new Error("Expected [data-insert-script-block=dialogue] to insert POSTAĆ block into #modal-scene-textarea");
  }
  await clickEl('button.scene-text-back-btn[action="CloseSceneTextModal"]');

  await clickEl("#btn-editor-breakdown");
  if (!doc.querySelector("#editor-breakdown-panel") || doc.querySelector("#editor-breakdown-panel").classList.contains("hidden")) {
    throw new Error("Expected #btn-editor-breakdown to open #editor-breakdown-panel");
  }

  // Clear canvas selection first, then click .breakdown-jump-btn[action="SelectStudioScene"]
  const spCanvas = doc.querySelector("#studio-canvas");
  spCanvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: 5, clientY: 5, target: spCanvas }));
  spCanvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: 5, clientY: 5, target: spCanvas }));
  await waitForActions();
  await clickEl('.breakdown-jump-btn[action="SelectStudioScene"]');
  if (vm.runInContext("selectedNodeIds.size", win) !== 1) {
    throw new Error("Expected clicking .breakdown-jump-btn after canvas deselect to select the target scene on canvas");
  }

  // Auto-save Character Rename and Location Rename inputs in Breakdown Panel on change/blur
  const charRenameForm = doc.querySelector('form.breakdown-rename-form[action="RenameCharacter"]');
  if (!charRenameForm) {
    throw new Error("Expected form.breakdown-rename-form[action=RenameCharacter] in #editor-breakdown-panel");
  }
  const newCharInp = charRenameForm.querySelector('input[name="new_name"]');
  newCharInp.value = "DETEKTYW";
  newCharInp.dispatchEvent(makeEvent("change", { target: newCharInp }));
  await waitForActions();

  const locRenameForm = doc.querySelector('form.breakdown-rename-form[action="RenameLocation"]');
  if (!locRenameForm) {
    throw new Error("Expected form.breakdown-rename-form[action=RenameLocation] in #editor-breakdown-panel");
  }
  const newLocInp = locRenameForm.querySelector('input[name="new_location"]');
  newLocInp.value = "PORT RZECZNY";
  newLocInp.dispatchEvent(makeEvent("blur", { target: newLocInp }));
  await waitForActions();

  console.log("STATUS=OK");
  console.log("ACTIONS=" + actionLog.join(","));
})().catch((err) => {
  console.error("FATAL=" + (err && err.stack ? err.stack : String(err)));
  process.exit(1);
});
`

	for _, bundleCase := range []struct {
		name string
		code string
	}{
		{name: "prod_app_js", code: prodJS},
		{name: "dev_app_js", code: devJS},
	} {
		caseDir := t.TempDir()
		srv := NewServer(caseDir, storyteller.AppFS)
		ts := httptest.NewServer(srv.Handler())

		cmd := exec.Command(nodeBin, "-e", harnessScript)
		cmd.Env = append(os.Environ(),
			"ST_TEST_BASE_URL="+ts.URL,
			"ST_TEST_APP_JS="+bundleCase.code,
		)
		out, err := cmd.CombinedOutput()
		ts.Close()
		if err != nil {
			t.Fatalf("%s live DOM responsiveness test failed: %v\nOutput:\n%s", bundleCase.name, err, string(out))
		}
		if !strings.Contains(string(out), "STATUS=OK") {
			t.Fatalf("expected STATUS=OK from %s harness, got:\n%s", bundleCase.name, string(out))
		}
		if srv.ActiveBook == nil || !srv.ActiveBook.IsScreenplay() {
			t.Fatalf("%s: expected active screenplay book after JS test", bundleCase.name)
		}
		if !strings.Contains(srv.ActiveBook.Scenes[0].Text, "DETEKTYW") || !strings.Contains(srv.ActiveBook.Scenes[0].Slugline, "PORT RZECZNY") {
			t.Fatalf("%s: expected DOM-submitted RenameCharacter and RenameLocation forms to update scene on server, got slugline=%q text=%q",
				bundleCase.name, srv.ActiveBook.Scenes[0].Slugline, srv.ActiveBook.Scenes[0].Text)
		}
	}

	// Verify standalone player.js bundle syntax & clean initialization in Node VM
	playerCheckCmd := exec.Command(nodeBin, "--check")
	playerCheckCmd.Stdin = strings.NewReader(playerJS)
	if out, err := playerCheckCmd.CombinedOutput(); err != nil {
		t.Fatalf("standalone player.js syntax check failed: %v\n%s", err, string(out))
	}
}

func TestMultiChapterAutoIDAndCollisionOnConnectChoice(t *testing.T) {
	dir := t.TempDir()
	bookDir := filepath.Join(dir, "books", "rzeka-test")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "data.txt"), []byte("title: Test Rzeki\nnarrated: false\nimageless: true\nstart: s_root\n"), 0644); err != nil {
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
	editorContent := `# StoryTeller — Stan edytora węzłów (editor.txt)
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
	if err := os.WriteFile(filepath.Join(bookDir, "wioskowe_dziwadlo.txt"), []byte(ch1Content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "w_gore_umierajacej_rzeki.txt"), []byte(ch2Content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "editor.txt"), []byte(editorContent), 0644); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(dir, storyteller.AppFS)
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
		"export_dir": {`C:\Users\Autor\StoryTeller\Exports`},
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
	srv := NewServer(rootDir, storyteller.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	postWFP := func(vals url.Values) *http.Response {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/action", bytes.NewReader(EncodeWFPForm(vals)))
		if err != nil {
			t.Fatalf("NewRequest failed: %v", err)
		}
		req.Header.Set("Content-Type", "text/x-wfp;charset=UTF-8")
		req.Header.Set("X-Requested-With", "StoryTeller")
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
	if err := os.WriteFile(filepath.Join(fakeRepo, "go.mod"), []byte("module storyteller\n"), 0644); err != nil {
		t.Fatalf("failed to write fake go.mod: %v", err)
	}
	repoBookDir := filepath.Join(fakeRepo, "books", "test-book")
	if err := os.MkdirAll(repoBookDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoBookDir, "data.txt"), []byte("title: Repo Book\nstart: s1\n"), 0644); err != nil {
		t.Fatalf("WriteFile data.txt failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoBookDir, "story.txt"), []byte("scene: Scena 1\nid: s1\nend: true\nTekst.\n"), 0644); err != nil {
		t.Fatalf("WriteFile story.txt failed: %v", err)
	}

	StopPreviousInstance(fakeRepo)
	repoSrv := NewServerWithOptions(fakeRepo, storyteller.AppFS, true)
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












