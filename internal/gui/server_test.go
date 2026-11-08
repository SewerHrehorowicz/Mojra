package gui

import (
	"archive/zip"
	"bytes"
	"image/color"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mojra"
	"mojra/internal/book"
	"mojra/internal/tpl"
)

func NewServer(rootDir string, embeddedFS fs.FS) *Server {
	return NewServerWithOptions(rootDir, embeddedFS, false)
}

func NewStandaloneServer(rootDir string, embeddedFS fs.FS, standaloneBook *book.Book) *Server {
	savesDir := book.ResolveSavesDir(rootDir)
	_ = os.MkdirAll(savesDir, 0755)

	settings := book.LoadAppSettings(savesDir)
	settings.PreviewMode = "full"
	settings.ActiveView = "player"

	s := &Server{
		RootDir:           rootDir,
		BooksDir:          filepath.Join(rootDir, "books"),
		SavesDir:          savesDir,
		AppDir:            filepath.Join(rootDir, "app"),
		EmbeddedFS:        embeddedFS,
		DevMode:           false,
		StandaloneMode:    true,
		ShutdownCh:        make(chan struct{}),
		Settings:          settings,
		FocusedHistoryIdx: -1,
	}
	s.ensureTemplatesLoaded()

	if standaloneBook != nil {
		s.StandaloneBookID = standaloneBook.ID
		s.setActiveBook(standaloneBook)
	}
	return s
}

func TestServerActionLifecycleAndSecurity(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
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
	if !strings.Contains(string(saveModalHTML), `value="Początek Przygody"`) || !strings.Contains(string(saveModalHTML), `>Wejdź do zamku</textarea>`) {
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
	if !strings.Contains(string(editTargetModalHTML), `>Wycofaj się</textarea>`) {
		t.Fatalf("expected updated choice_label_2 in modal HTML")
	}
	if !strings.Contains(string(editTargetModalHTML), "choice-exit-input") ||
		!strings.Contains(string(editTargetModalHTML), `value="Nowa scena 3"`) ||
		strings.Contains(string(editTargetModalHTML), "choice-port-trigger") ||
		!strings.Contains(string(editTargetModalHTML), "choice-trash-icon") ||
		!strings.Contains(string(editTargetModalHTML), `action="PromptRemoveChoice"`) ||
		strings.Contains(string(editTargetModalHTML), "BranchChoiceScene") ||
		strings.Contains(string(editTargetModalHTML), "node-choice-arrow") {
		t.Fatalf("expected choice-exit-input, no choice-port-trigger, and trash icon with PromptRemoveChoice in modal choices, got:\n%s", string(editTargetModalHTML))
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

	// 2e. Save node position and viewport to .editor.cfg via SaveEditorState and connect wire via ConnectChoice
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
	editorTxtBytes, err := os.ReadFile(filepath.Join(rootDir, "books", "tajemnica-zamku", ".editor.cfg"))
	if err != nil {
		t.Fatalf("expected .editor.cfg in book directory: %v", err)
	}
	if !strings.Contains(string(editorTxtBytes), "540, 120") || !strings.Contains(string(editorTxtBytes), "zoom: 1.15") {
		t.Fatalf("unexpected .editor.cfg content:\n%s", string(editorTxtBytes))
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
	batchEditorBytes, _ := os.ReadFile(filepath.Join(rootDir, "books", "tajemnica-zamku", ".editor.cfg"))
	if !strings.Contains(string(batchEditorBytes), "-240, -120") || !strings.Contains(string(batchEditorBytes), "480, 240") {
		t.Fatalf("expected negative and batch coordinates in .editor.cfg, got:\n%s", string(batchEditorBytes))
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

	devQRServer := NewServerWithOptions(rootDir, mojra.AppFS, true)
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
	_ = os.WriteFile(filepath.Join(diskBookPath, "book.playtxt"), []byte("title: Nocny Pociąg\nauthor: Marek H.\nnarrated: false\nstart: peron\n"), 0644)
	_ = os.WriteFile(filepath.Join(diskBookPath, "story.playtxt"), []byte(`chapter: Odjazd

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
	fData, _ := zw.Create("book.playtxt")
	_, _ = fData.Write([]byte("title: Echo Głębin\nauthor: Klara M.\nnarrated: false\nstart: zanurzenie\n"))
	fStory, _ := zw.Create("story.playtxt")
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

	srv := NewServer(rootDir, mojra.AppFS)
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
	remForm := url.Values{}
	remForm.Set("action", "RemoveBook")
	remForm.Set("book", "nocny-pociag")
	r8, _ := http.PostForm(ts.URL+"/api/action", remForm)
	_ = r8.Body.Close()

	if _, err := os.Stat(filepath.Join(diskBookPath, "story.playtxt")); err != nil {
		t.Fatalf("expected original files of external book on disk to remain untouched after removing from library: %v", err)
	}
}

func TestDevicePreviewStrippedInProductionAndEnabledInDev(t *testing.T) {
	rootDir := t.TempDir()
	savesDir := filepath.Join(rootDir, "saves")
	_ = os.MkdirAll(savesDir, 0755)
	_ = os.WriteFile(filepath.Join(savesDir, "settings.ini"), []byte("[display]\ntheme = dark\npreview_mode = portrait\n"), 0644)

	// 1. Production server (default NewServer) must strip device preview completely
	prodSrv := NewServer(rootDir, mojra.AppFS)
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
	devSrv := NewServerWithOptions(rootDir, mojra.AppFS, true)
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
	for _, wantPreviewAttr := range []string{`data-mode="portrait"`, `data-mode="tablet"`, `data-mode="full"`} {
		if !strings.Contains(string(devHTML), wantPreviewAttr) {
			t.Fatalf("expected %s in dev mode GET /", wantPreviewAttr)
		}
	}
	for _, forbiddenChrome := range []string{"device-notch", "device-home-bar", "Widok ekranu:", "iPhone", "iPad"} {
		if strings.Contains(string(devHTML), forbiddenChrome) {
			t.Fatalf("expected minimalist generic device preview without %q in dev mode GET /", forbiddenChrome)
		}
	}
	devCSSResp, err := http.Get(devTS.URL + "/style.css")
	if err != nil {
		t.Fatalf("GET /style.css in dev mode failed: %v", err)
	}
	devCSS, _ := io.ReadAll(devCSSResp.Body)
	_ = devCSSResp.Body.Close()
	for _, wantCSS := range []string{"--preview-stage-top", "--preview-shadow-ambient", `.device-stage[data-preview="tablet"]`} {
		if !strings.Contains(string(devCSS), wantCSS) {
			t.Fatalf("expected %q in dev mode /style.css", wantCSS)
		}
	}

	tabletForm := url.Values{
		"action": {"SetDeviceMode"},
		"mode":   {"tablet"},
	}
	tabletResp, err := http.PostForm(devTS.URL+"/api/action", tabletForm)
	if err != nil {
		t.Fatalf("POST /api/action SetDeviceMode(tablet) failed: %v", err)
	}
	_ = tabletResp.Body.Close()
	if gotHdr := tabletResp.Header.Get("X-Device-Mode"); gotHdr != "tablet" {
		t.Fatalf("expected X-Device-Mode=tablet in dev mode, got %q", gotHdr)
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
	_ = os.WriteFile(filepath.Join(sampleBookDir, "book.playtxt"), []byte("title: Nie bój się policji rzecznej\nauthor: Autor\nnarrated: false\nstart: warsztat\n"), 0644)
	_ = os.WriteFile(filepath.Join(sampleBookDir, "story.playtxt"), []byte(`chapter: Wioskowe dziwadło

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
	srv := NewServer(rootDir, mojra.AppFS)
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

	if !strings.Contains(libHTML, `class="brand-mark"`) ||
		!strings.Contains(libHTML, `<span class="brand-logo">Mojra</span>`) ||
		strings.Contains(libHTML, `<span class="brand-logo brand-logo-short">M</span>`) ||
		!strings.Contains(libHTML, `id="btn-open-settings"`) ||
		!strings.Contains(libHTML, `action="OpenQRModal"`) ||
		!strings.Contains(libHTML, `id="btn-toggle-fullscreen"`) ||
		!strings.Contains(libHTML, `id="btn-switch-to-studio"`) ||
		!strings.Contains(libHTML, `data-view="studio"`) {
		t.Fatalf("expected isolated Reader Home topbar with brand-mark, Mojra logo, settings gear icon, fullscreen button, and bottom Studio switch, got:\n%s", libHTML)
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
	cardChunk := ""
	if cardStart >= 0 {
		cardChunk = libSection[cardStart:]
		if endArt := strings.Index(cardChunk, "</article>"); endArt > 0 {
			cardChunk = cardChunk[:endArt]
		}
	}
	if cardStart < 0 || strings.Count(cardChunk, "<button") != 1 || !strings.Contains(cardChunk, `class="book-cover-wrap"`) {
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

	// 3. Verify opening a book in player view did NOT automatically open it in Studio,
	// then explicitly open it in Studio via OpenStudioProject, clear title of scene "sikawa_kapitana" via SaveScene,
	// and verify player renders it with .scene-untitled and without .scene-header-block (single separator line)
	if srv.ActiveBook != nil {
		t.Fatalf("expected OpenBook(view=player) not to auto-open book in Studio, got ActiveBook=%+v", srv.ActiveBook)
	}
	openStudio := url.Values{
		"action": {"OpenStudioProject"},
		"book":   {"nie-boj-sie-policji-rzecznej"},
	}
	rOpenStudio, err := http.PostForm(ts.URL+"/api/action", openStudio)
	if err != nil {
		t.Fatalf("OpenStudioProject failed: %v", err)
	}
	_ = rOpenStudio.Body.Close()

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
	if strings.Contains(libHTML, `id="btn-toggle-zen"`) {
		t.Fatalf("expected #btn-toggle-zen to NOT be rendered in library view")
	}

	rScaleBack, err := http.PostForm(ts.URL+"/api/action", scaleForm)
	if err != nil {
		t.Fatalf("second ToggleUIScale failed: %v", err)
	}
	_ = rScaleBack.Body.Close()
	if rScaleBack.Header.Get("X-UI-Scale") != "small" {
		t.Fatalf("expected X-UI-Scale=small on second ToggleUIScale (never zen), got %q", rScaleBack.Header.Get("X-UI-Scale"))
	}

	// Switch back to player view and verify ToggleZenMode works exclusively in book reader context
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{"action": {"SwitchView"}, "view": {"player"}})
	zenForm := url.Values{"action": {"ToggleZenMode"}}
	rZenOn, err := http.PostForm(ts.URL+"/api/action", zenForm)
	if err != nil {
		t.Fatalf("ToggleZenMode failed: %v", err)
	}
	_ = rZenOn.Body.Close()
	if rZenOn.Header.Get("X-Zen-Mode") != "true" {
		t.Fatalf("expected X-Zen-Mode=true in player view, got %q", rZenOn.Header.Get("X-Zen-Mode"))
	}
	rZenOff, err := http.PostForm(ts.URL+"/api/action", zenForm)
	if err != nil {
		t.Fatalf("ToggleZenMode off failed: %v", err)
	}
	_ = rZenOff.Body.Close()
	if rZenOff.Header.Get("X-Zen-Mode") != "false" {
		t.Fatalf("expected X-Zen-Mode=false after second ToggleZenMode, got %q", rZenOff.Header.Get("X-Zen-Mode"))
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

	setSensForm := url.Values{}
	setSensForm.Set("action", "SetScrollSensitivity")
	setSensForm.Set("sensitivity", "low")
	rSetSens, err := http.PostForm(ts.URL+"/api/action", setSensForm)
	if err != nil {
		t.Fatalf("SetScrollSensitivity failed: %v", err)
	}
	_ = rSetSens.Body.Close()
	if rSetSens.Header.Get("X-Scroll-Sensitivity") != "low" {
		t.Fatalf("expected X-Scroll-Sensitivity=low header after SetScrollSensitivity(low), got %q", rSetSens.Header.Get("X-Scroll-Sensitivity"))
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
	if strings.Contains(quitHTML, `id="modal-confirm"`) || strings.Contains(quitHTML, "Zamknij Mojra") {
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

	srv := NewStandaloneServer(rootDir, mojra.AppFS, reloadedBook)
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
		"ExportProject",
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

	srv := NewServer(rootDir, mojra.AppFS)
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

	// 1b. Remote LAN client (non-loopback RemoteAddr) may only execute safe player/display actions, blocking editor/disk mutations with 403 Forbidden
	for _, blockedLANAction := range []string{"RemoveBook", "AddBookFromDisk", "CreateNewBook", "SaveScene", "QuitApp"} {
		lanReq := httptest.NewRequest(http.MethodPost, "/api/action", strings.NewReader("action="+blockedLANAction))
		lanReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		lanReq.RemoteAddr = "192.168.1.42:51234"
		lanRec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(lanRec, lanReq)
		if lanRec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for remote LAN action %q, got %d", blockedLANAction, lanRec.Code)
		}
	}
	for _, allowedLANAction := range []string{"ToggleTheme", "ChooseBranch"} {
		lanReq := httptest.NewRequest(http.MethodPost, "/api/action", strings.NewReader("action="+allowedLANAction))
		lanReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		lanReq.RemoteAddr = "192.168.1.42:51234"
		lanRec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(lanRec, lanReq)
		if lanRec.Code != http.StatusOK && lanRec.Code != http.StatusNoContent {
			t.Fatalf("expected 200/204 for safe remote LAN action %q, got %d", allowedLANAction, lanRec.Code)
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

	rOpenEd, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"OpenStudioProject"},
		"book":   {createdBook.ID},
	})
	if err != nil {
		t.Fatalf("OpenStudioProject failed: %v", err)
	}
	_ = rOpenEd.Body.Close()

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
	_ = os.WriteFile(unrelatedTxt, []byte("To są prywatne notatki użytkownika bez nagłówków Mojra.\nHasło: tajne123\n"), 0644)

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
	if !strings.Contains(css, `body[data-zen="true"] .scene-text-editor-feed .reader-global-meta`) {
		t.Fatalf("expected .scene-text-editor-feed .reader-global-meta to remain visible in zen mode")
	}
	for _, panelSel := range []string{".editor-context-panel {", ".editor-history-panel,\n.editor-help-panel {", ".editor-breakdown-panel,\n.editor-variables-panel {"} {
		idx := strings.Index(css, panelSel)
		if idx < 0 {
			t.Fatalf("missing base rule %s in style.css", panelSel)
		}
		block := css[idx:]
		if end := strings.Index(block, "}"); end > 0 {
			block = block[:end]
		}
		if !strings.Contains(block, "width: 320px") || strings.Contains(block, "width: 100%") {
			t.Fatalf("expected %s to stay a 320px right-side column on desktop (never width: 100%%), got:\n%s", panelSel, block)
		}
	}
	if strings.Contains(css, "\n.editor-workspace-body:has(> aside:not(.hidden)) .timeline-scroll-viewport {") {
		t.Fatalf(".timeline-scroll-viewport must NEVER be hidden globally by :has(> aside:not(.hidden)) outside portrait phone media queries")
	}

	// 3. Verify runtime behavior when opening #modal-scene-text and clicking topbar navigation tabs
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
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
		!strings.Contains(openFullHTML, `id="tab-studio-home"`) ||
		!strings.Contains(openFullHTML, `id="editor-book-title">Księga Rzeki</span>`) ||
		!strings.Contains(openFullHTML, `id="modal-scene-text"`) {
		t.Fatalf("expected #topbar with Studio navigation and title to remain present when #modal-scene-text is open, got:\n%s", openFullHTML)
	}
	modalPart := openFullHTML[strings.Index(openFullHTML, `id="modal-scene-text"`):]
	if strings.Contains(modalPart, "Graj od tej sceny") || strings.Contains(modalPart, "Usuń scenę") || strings.Contains(modalPart, "Wróć do edytora") {
		t.Fatalf("expected icon-only buttons without text descriptions in .scene-text-editor-top-bar, got:\n%s", modalPart)
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
	if strings.Contains(string(swBytes), `class="timeline-scroll-viewport hidden"`) {
		t.Fatalf("#timeline-scroll-viewport must never be hidden on desktop when an editor panel is open")
	}
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
	srv := NewServer(rootDir, mojra.AppFS)
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
	_ = rTogOn.Body.Close()

	if !srv.ActiveBook.Imageless {
		t.Fatalf("expected ActiveBook.Imageless=true after ToggleBookImageless")
	}
	if rTogOn.Header.Get("X-Imageless") != "true" || rTogOn.Header.Get("X-Imageless-Locked") != "true" {
		t.Fatalf("expected X-Imageless=true and X-Imageless-Locked=true when imageless book is loaded, got %q / %q", rTogOn.Header.Get("X-Imageless"), rTogOn.Header.Get("X-Imageless-Locked"))
	}
	rSettingsOn, err := http.PostForm(ts.URL+"/api/action", url.Values{"action": {"OpenQRModal"}})
	if err != nil {
		t.Fatalf("OpenQRModal failed: %v", err)
	}
	settingsOnBytes, _ := io.ReadAll(rSettingsOn.Body)
	_ = rSettingsOn.Body.Close()
	settingsOnHTML := string(settingsOnBytes)
	if !strings.Contains(settingsOnHTML, `id="btn-toggle-imageless" type="button" action="ToggleImageless" disabled="disabled"`) {
		t.Fatalf("expected #btn-toggle-imageless to be disabled/inactive in settings modal when imageless book is open, got:\n%s", settingsOnHTML)
	}
	if !strings.Contains(settingsOnHTML, `class="action-icon hidden" id="icon-imageless-off"`) ||
		strings.Contains(settingsOnHTML, `class="action-icon hidden" id="icon-imageless-on"`) {
		t.Fatalf("expected #btn-toggle-imageless to show no-image icon (#icon-imageless-on) when imageless book is open")
	}
	_, _ = http.PostForm(ts.URL+"/api/action", url.Values{"action": {"CloseQRModal"}})

	// Verify book.playtxt persisted imageless: true
	dataBytes, err := os.ReadFile(filepath.Join(srv.ActiveBook.RootPath, "book.playtxt"))
	if err != nil || !strings.Contains(string(dataBytes), "imageless: true") {
		t.Fatalf("expected book.playtxt to contain 'imageless: true', got:\n%s", string(dataBytes))
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
	srv := NewServer(rootDir, mojra.AppFS)
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
	dataTxtBytes, err := os.ReadFile(filepath.Join(srv.ActiveBook.RootPath, "book.playtxt"))
	if err != nil || !strings.Contains(string(dataTxtBytes), "title: Nowa Kronika Rzeki") {
		t.Fatalf("expected book.playtxt to persist 'title: Nowa Kronika Rzeki', got:\n%s", string(dataTxtBytes))
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
	dataTxtAfterStart, err := os.ReadFile(filepath.Join(srv.ActiveBook.RootPath, "book.playtxt"))
	if err != nil || !strings.Contains(string(dataTxtAfterStart), "start: rozdroze") {
		t.Fatalf("expected book.playtxt to persist 'start: rozdroze', got:\n%s", string(dataTxtAfterStart))
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
	chFileBytes, err := os.ReadFile(filepath.Join(srv.ActiveBook.RootPath, "rozdzial_1.playtxt"))
	if err != nil || !strings.Contains(string(chFileBytes), "start: poczatek") {
		t.Fatalf("expected rozdzial_1.playtxt to persist chapter 'start: poczatek', got:\n%s", string(chFileBytes))
	}
}

func TestNarrationlessAllOrNothingAndMinimalProgressBar(t *testing.T) {
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
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
	textFileName := "story.playtxt"
	if len(srv.ActiveBook.TextFiles) > 0 && srv.ActiveBook.TextFiles[0] != "" {
		textFileName = srv.ActiveBook.TextFiles[0]
	}
	storyBytes, err := os.ReadFile(filepath.Join(srv.ActiveBook.RootPath, textFileName))
	if err != nil {
		t.Fatalf("failed to read %s: %v", textFileName, err)
	}
	if strings.Contains(string(storyBytes), "voice:") {
		t.Fatalf("expected narrationless story.playtxt to have no voice: headers, got:\n%s", string(storyBytes))
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
	srv := NewServer(rootDir, mojra.AppFS)
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

func TestMultilingualUISwitchAndI18nTableCompleteness(t *testing.T) {
	// 1. Verify all TrKey entries are non-empty across all registered languages in SupportedLanguages (.cfg dictionaries).
	for l := Lang(0); l < LangCount; l++ {
		spec := tpl.SupportedLanguages[l]
		if spec.Code == "" || spec.Upper == "" || spec.Name == "" {
			t.Fatalf("SupportedLanguages[%d] has empty metadata: %+v", l, spec)
		}
		if len(tpl.I18nDicts[l]) == 0 {
			t.Fatalf("tpl.I18nDicts[%d] (%s.cfg) is empty", l, spec.Code)
		}
		for k := TrKey(0); k < trKeyCount; k++ {
			keyName := trKeyNames[k]
			if strings.TrimSpace(keyName) == "" {
				t.Fatalf("trKeyNames[%d] is empty", k)
			}
			if strings.TrimSpace(tpl.I18nDicts[l][keyName]) == "" {
				t.Fatalf("i18n/%s.cfg key %q (TrKey=%d) is empty or missing", spec.Code, keyName, k)
			}
			if strings.TrimSpace(i18nTable[k][l]) == "" {
				t.Fatalf("i18nTable[%d][%s] is empty", k, spec.Code)
			}
		}
	}

	// 1b. Verify internal/gui/*.go has zero inline trPair(pl, en) calls.
	guiGoMatches, err := filepath.Glob("*.go")
	if err != nil || len(guiGoMatches) == 0 {
		t.Fatalf("failed to glob internal/gui/*.go: %v", err)
	}
	for _, p := range guiGoMatches {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		raw, rErr := os.ReadFile(p)
		if rErr != nil {
			t.Fatalf("read %s: %v", p, rErr)
		}
		if strings.Contains(string(raw), "trPair(") {
			t.Fatalf("%s contains forbidden inline trPair(pl, en); add a TrKey to i18n_<lang>.go instead", p)
		}
	}

	// 2. Verify all tplI18nBindings are referenced in app/templates/*.tpl and vice-versa.
	tplMatches, err := filepath.Glob(filepath.Join("..", "..", "app", "templates", "*.tpl"))
	if err != nil || len(tplMatches) == 0 {
		t.Fatalf("failed to glob app/templates/*.tpl: %v", err)
	}
	var allTplBuilder strings.Builder
	for _, p := range tplMatches {
		raw, rErr := os.ReadFile(p)
		if rErr != nil {
			t.Fatalf("read %s: %v", p, rErr)
		}
		allTplBuilder.Write(raw)
		allTplBuilder.WriteByte('\n')
	}
	allTpl := allTplBuilder.String()

	boundVars := make(map[string]bool, len(tplI18nBindings))
	for _, b := range tplI18nBindings {
		boundVars[b.varName] = true
		if !strings.Contains(allTpl, "{"+b.varName+"}") {
			t.Fatalf("tplI18nBindings var %q is not used in app/templates/*.tpl", b.varName)
		}
	}
	tVarRe := regexp.MustCompile(`\{(t_[a-z0-9_]+)\}`)
	for _, m := range tVarRe.FindAllStringSubmatch(allTpl, -1) {
		if !boundVars[m[1]] {
			t.Fatalf("template variable %q in .tpl is missing from tplI18nBindings", m[1])
		}
	}

	// 2b. Verify all tpl.JSI18nTable entries are non-empty in every registered language, bijectively mapped to tr()/trf()/trStd() in app/js/*.js,
	// and that app/js/*.js contains zero hardcoded Polish string literals or legacy trUI() calls.
	jsKeySet := make(map[string]bool, len(tpl.JSI18nTable))
	for _, e := range tpl.JSI18nTable {
		key := e[0]
		if strings.TrimSpace(key) == "" {
			t.Fatalf("tpl.JSI18nTable entry %+v has empty key", e)
		}
		for l := Lang(0); l < LangCount; l++ {
			if strings.TrimSpace(tpl.I18nDicts[l][key]) == "" || strings.TrimSpace(e[1+int(l)]) == "" {
				t.Fatalf("tpl.JSI18nTable key %q is empty for language %q", key, tpl.SupportedLanguages[l].Code)
			}
		}
		if jsKeySet[key] {
			t.Fatalf("duplicate key %q in tpl.JSI18nTable", key)
		}
		jsKeySet[key] = true
	}

	jsMatches, err := filepath.Glob(filepath.Join("..", "..", "app", "js", "*.js"))
	if err != nil || len(jsMatches) == 0 {
		t.Fatalf("failed to glob app/js/*.js: %v", err)
	}
	jsTrCallRe := regexp.MustCompile(`\b(?:tr|trf)\(\s*"([a-z0-9_]+)"|\btrStd\([^,]+,\s*"([a-z0-9_]+)"`)
	jsQuotedPolishRe := regexp.MustCompile(`"(?:[^"\\\n\r]|\\.)*[ąęćłńóśźżĄĘĆŁŃÓŚŹŻ](?:[^"\\\n\r]|\\.)*"|'(?:[^'\\\n\r]|\\.)*[ąęćłńóśźżĄĘĆŁŃÓŚŹŻ](?:[^'\\\n\r]|\\.)*'`)
	usedJSKeys := make(map[string]bool, len(tpl.JSI18nTable))
	for _, p := range jsMatches {
		raw, rErr := os.ReadFile(p)
		if rErr != nil {
			t.Fatalf("read %s: %v", p, rErr)
		}
		src := string(raw)
		base := filepath.Base(p)
		if strings.Contains(src, "trUI(") {
			t.Fatalf("%s contains forbidden inline trUI(pl, en); use tr(key) / trf(key) backed by tpl.JSI18nTable", base)
		}
		if m := jsQuotedPolishRe.FindString(src); m != "" {
			t.Fatalf("%s contains hardcoded Polish string literal %s; move it to tpl.JSI18nTable", base, m)
		}
		for _, m := range jsTrCallRe.FindAllStringSubmatch(src, -1) {
			k := m[1]
			if k == "" {
				k = m[2]
			}
			if !jsKeySet[k] {
				t.Fatalf("%s references unknown JS i18n key %q (missing from tpl.JSI18nTable)", base, k)
			}
			usedJSKeys[k] = true
		}
	}
	for _, e := range tpl.JSI18nTable {
		if !usedJSKeys[e[0]] {
			t.Fatalf("tpl.JSI18nTable key %q is unused in app/js/*.js", e[0])
		}
	}

	// 2c. Verify book.ExplainConditionExprErrorLang and book.ExplainEffectsExprErrorLang localize to PL and EN.
	if gotEN := book.ExplainConditionExprErrorLang("nieznana_zmienna", nil, "en"); !strings.Contains(gotEN, "No variables declared") {
		t.Fatalf("expected English condition validation message for lang=en, got %q", gotEN)
	}
	if gotPL := book.ExplainConditionExprErrorLang("nieznana_zmienna", nil, "pl"); !strings.Contains(gotPL, "Brak zadeklarowanych") {
		t.Fatalf("expected Polish condition validation message for lang=pl, got %q", gotPL)
	}

	// 3. Verify PopulateI18nContext performs zero heap allocations on a pre-sized map.
	prealloc := make(map[string]any, len(tplI18nBindings)+8)
	PopulateI18nContext(prealloc, LangPL)
	allocs := testing.AllocsPerRun(100, func() {
		PopulateI18nContext(prealloc, LangEN)
	})
	if allocs > 0 {
		t.Fatalf("expected 0 allocations in PopulateI18nContext, got %v", allocs)
	}

	// 4. Verify ToggleLanguage and SetLanguage switch UI between PL and EN and persist to settings.ini.
	rootDir := t.TempDir()
	srv := NewServer(rootDir, mojra.AppFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Default is PL
	getResp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	initBytes, _ := io.ReadAll(getResp.Body)
	_ = getResp.Body.Close()
	initHTML := string(initBytes)
	if !strings.Contains(initHTML, `data-lang="pl"`) || !strings.Contains(initHTML, `+ Dodaj książkę`) {
		t.Fatalf("expected default PL UI on initial load, got:\n%s", initHTML)
	}

	// ToggleLanguage -> EN
	togResp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"ToggleLanguage"},
	})
	if err != nil {
		t.Fatalf("ToggleLanguage failed: %v", err)
	}
	togBytes, _ := io.ReadAll(togResp.Body)
	_ = togResp.Body.Close()
	togHTML := string(togBytes)
	if got := togResp.Header.Get("X-Lang"); got != "en" {
		t.Fatalf("expected X-Lang=en after ToggleLanguage, got %q", got)
	}
	if !strings.Contains(togHTML, `+ Add book`) || !strings.Contains(togHTML, `Switch to authoring environment`) {
		t.Fatalf("expected English UI labels after ToggleLanguage, got:\n%s", togHTML)
	}

	loadedCfg := book.LoadAppSettings(srv.SavesDir)
	if loadedCfg.Language != "en" {
		t.Fatalf("expected persisted settings.ini lang=en, got %q", loadedCfg.Language)
	}

	// SetLanguage back to PL
	setResp, err := http.PostForm(ts.URL+"/api/action", url.Values{
		"action": {"SetLanguage"},
		"lang":   {"pl"},
	})
	if err != nil {
		t.Fatalf("SetLanguage(pl) failed: %v", err)
	}
	setBytes, _ := io.ReadAll(setResp.Body)
	_ = setResp.Body.Close()
	setHTML := string(setBytes)
	if got := setResp.Header.Get("X-Lang"); got != "pl" {
		t.Fatalf("expected X-Lang=pl after SetLanguage(pl), got %q", got)
	}
	if !strings.Contains(setHTML, `+ Dodaj książkę`) || !strings.Contains(setHTML, `Przełącz na środowisko autorskie`) {
		t.Fatalf("expected Polish UI labels after SetLanguage(pl), got:\n%s", setHTML)
	}
}

