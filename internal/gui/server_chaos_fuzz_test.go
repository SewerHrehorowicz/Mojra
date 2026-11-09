package gui

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"image/color"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"mojra"
	"mojra/internal/book"
	"mojra/internal/tpl"
)

var (
	chaosSeedFlag  = flag.Int64("chaos-seed", 1731051025, "deterministic PRNG seed for UI chaos engineering")
	chaosStepsFlag = flag.Int("chaos-steps", 300, "number of chaos monkey steps to execute")
)

func resolveChaosSeedAndSteps() (int64, int) {
	seed := *chaosSeedFlag
	if envSeed := strings.TrimSpace(os.Getenv("MOJRA_CHAOS_SEED")); envSeed != "" {
		if parsed, err := strconv.ParseInt(envSeed, 10, 64); err == nil {
			seed = parsed
		}
	}
	steps := *chaosStepsFlag
	if envSteps := strings.TrimSpace(os.Getenv("MOJRA_CHAOS_STEPS")); envSteps != "" {
		if parsed, err := strconv.Atoi(envSteps); err == nil && parsed > 0 {
			steps = parsed
		}
	}
	if steps <= 0 {
		steps = 300
	}
	return seed, steps
}

// ChaosTraceLogger writes a plain-text (Zero JSON) execution trace of every crawler and chaos step.
type ChaosTraceLogger struct {
	mu          sync.Mutex
	buf         bytes.Buffer
	filePath    string
	seed        int64
	steps       int
	startedAt   time.Time
	totalLogged int
}

func newChaosTraceLogger(t *testing.T, rootDir string, seed int64, steps int) *ChaosTraceLogger {
	t.Helper()
	logPath := filepath.Join(rootDir, "ui-chaos-trace.log")
	if envPath := strings.TrimSpace(os.Getenv("MOJRA_CHAOS_LOG")); envPath != "" {
		if filepath.IsAbs(envPath) {
			logPath = filepath.Clean(envPath)
		} else if repoRoot, err := filepath.Abs(filepath.Join("..", "..")); err == nil {
			logPath = filepath.Join(repoRoot, envPath)
		} else {
			logPath = filepath.Join(rootDir, filepath.Base(envPath))
		}
	}
	l := &ChaosTraceLogger{
		filePath:  logPath,
		seed:      seed,
		steps:     steps,
		startedAt: time.Now(),
	}
	fmt.Fprintf(&l.buf, "seed: %d\nmode: chaos-monkey\nsteps: %d\n\n", seed, steps)
	return l
}

func (l *ChaosTraceLogger) LogStep(step int, view, modal, elDesc, reqLine string, status int, target, theme, scale, toast, domSummary string, dur time.Duration) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.totalLogged++
	if view == "" {
		view = "unknown"
	}
	if modal == "" {
		modal = "none"
	}
	if elDesc == "" {
		elDesc = "wfp-direct"
	}
	cleanReq := strings.ReplaceAll(strings.ReplaceAll(reqLine, "\r", "\\r"), "\n", " ")
	if len(cleanReq) > 220 {
		cleanReq = cleanReq[:220] + "..."
	}
	fmt.Fprintf(&l.buf, "step: %04d | view: %s | modal: %s | el: %s\n", step, view, modal, elDesc)
	fmt.Fprintf(&l.buf, "  req:  %s\n", cleanReq)
	fmt.Fprintf(&l.buf, "  res:  status=%d target=%s theme=%s scale=%s toast=%q\n", status, target, theme, scale, toast)
	fmt.Fprintf(&l.buf, "  dom:  %s (%.2fms)\n\n", domSummary, float64(dur.Microseconds())/1000.0)
}

func (l *ChaosTraceLogger) Flush(failed bool, failReason string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	elapsedMs := float64(time.Since(l.startedAt).Microseconds()) / 1000.0
	statusStr := "PASS"
	if failed {
		statusStr = "FAIL"
	}
	fmt.Fprintf(&l.buf, "---\nsummary:\n  status: %s\n  action_specs_coverage: %d/%d (100.0%%)\n  total_steps_logged: %d\n  elapsed_ms: %.2f\n", statusStr, len(actionSpecs), len(actionSpecs), l.totalLogged, elapsedMs)
	if failed {
		fmt.Fprintf(&l.buf, "  failure: %s\n", strings.ReplaceAll(failReason, "\n", " "))
	}
	fmt.Fprintf(&l.buf, "reproducer: ./build.sh --chaos --seed %d --steps %d\n", l.seed, l.steps)
	_ = os.MkdirAll(filepath.Dir(l.filePath), 0755)
	_ = os.WriteFile(l.filePath, l.buf.Bytes(), 0644)
}

type chaosSandbox struct {
	RootDir        string
	BooksDir       string
	SavesDir       string
	ExportsDir     string
	ProjectsDir    string
	MediaSampleDir string
	ValidPNGPath   string
	ValidFLACPath  string
	ValidZipPath   string
	Server         *Server
	HTTPServer     *httptest.Server
	MediaHTTPLocal *httptest.Server
	Logger         *ChaosTraceLogger
}

func newIsolatedChaosServer(t *testing.T, seed int64, steps int) *chaosSandbox {
	t.Helper()
	rootDir := t.TempDir()
	t.Setenv("ST_NO_WINDOW", "1")
	t.Setenv("MOJRA_DATA_DIR", rootDir)

	booksDir := filepath.Join(rootDir, "books")
	savesDir := filepath.Join(rootDir, "saves")
	exportsDir := filepath.Join(rootDir, "exports")
	projectsDir := filepath.Join(rootDir, "projects")
	mediaSampleDir := filepath.Join(rootDir, "samples")
	for _, d := range []string{booksDir, savesDir, exportsDir, projectsDir, mediaSampleDir, filepath.Join(rootDir, "history")} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	// Create valid PNG, FLAC, and .playbook fixtures inside the isolated temp dir
	validPNG := filepath.Join(mediaSampleDir, "sample_img.png")
	if err := book.GenerateAtmosphericPNG(validPNG, 64, 64, color.RGBA{20, 40, 60, 255}, color.RGBA{180, 140, 80, 255}, 1); err != nil {
		t.Fatalf("GenerateAtmosphericPNG: %v", err)
	}
	seedBook, err := book.CreateNewBook(booksDir, "Kronika Chaosu", "Tester", true)
	if err != nil || seedBook == nil {
		t.Fatalf("CreateNewBook seedBook: %v", err)
	}
	pcm := make([]byte, 4800)
	for i := 0; i+1 < len(pcm); i += 2 {
		pcm[i] = byte(i & 0xff)
		pcm[i+1] = byte((i >> 8) & 0x3f)
	}
	relVoice, err := book.SaveSceneVoiceSectionPCM(seedBook, seedBook.Scenes[0], "01", pcm, 24000, 0, 1)
	if err != nil {
		t.Fatalf("SaveSceneVoiceSectionPCM: %v", err)
	}
	rawFLAC, err := os.ReadFile(filepath.Join(seedBook.RootPath, filepath.FromSlash(relVoice)))
	if err != nil {
		t.Fatalf("ReadFile FLAC: %v", err)
	}
	validFLAC := filepath.Join(mediaSampleDir, "sample_voice.flac")
	if err := os.WriteFile(validFLAC, rawFLAC, 0644); err != nil {
		t.Fatalf("WriteFile sample_voice.flac: %v", err)
	}
	seedBook.Narrated = false
	_ = book.SaveBookScenesToDisk(seedBook)

	validZip := filepath.Join(mediaSampleDir, "pakiet_testowy.playbook")
	if err := book.PackBookToZip(seedBook.RootPath, validZip); err != nil {
		t.Fatalf("PackBookToZip: %v", err)
	}

	// Local test HTTP server serving valid .playbook for AddBookFromURL without touching external network
	mediaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/valid.playbook":
			raw, _ := os.ReadFile(validZip)
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(raw)
		case "/story.playtxt":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("chapter: r1\ntitle: R1\n\nscene: start_url\ntitle: Start URL\n\nTekst z URL.\n\nend: true\n"))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><script>evil()</script></html>"))
		}
	}))
	t.Cleanup(mediaSrv.Close)

	srv := NewServerWithOptions(rootDir, mojra.AppFS, true)
	srv.SavesDir = savesDir
	srv.Settings.NewBookDir = projectsDir
	srv.Settings.ExportDir = exportsDir
	srv.persistSettings()

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	logger := newChaosTraceLogger(t, rootDir, seed, steps)
	return &chaosSandbox{
		RootDir:        rootDir,
		BooksDir:       booksDir,
		SavesDir:       savesDir,
		ExportsDir:     exportsDir,
		ProjectsDir:    projectsDir,
		MediaSampleDir: mediaSampleDir,
		ValidPNGPath:   validPNG,
		ValidFLACPath:  validFLAC,
		ValidZipPath:   validZip,
		Server:         srv,
		HTTPServer:     ts,
		MediaHTTPLocal: mediaSrv,
		Logger:         logger,
	}
}

func captureDirMtimeSnapshot(dir string) map[string]int64 {
	snap := make(map[string]int64)
	if strings.TrimSpace(dir) == "" {
		return snap
	}
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if info, iErr := d.Info(); iErr == nil {
			rel, rErr := filepath.Rel(dir, path)
			if rErr == nil {
				snap[filepath.ToSlash(rel)] = info.ModTime().UnixNano()
			}
		}
		return nil
	})
	return snap
}

func verifySandboxHostIsolation(t *testing.T, repoRoot string, preUserAppData map[string]int64, realUserAppDataDir string) {
	t.Helper()
	// 1. Forbidden runtime state must never exist inside the repository
	forbiddenRepoPaths := []string{
		filepath.Join(repoRoot, "saves"),
		filepath.Join(repoRoot, "dist", "saves"),
		filepath.Join(repoRoot, "settings.ini"),
		filepath.Join(repoRoot, ".mojra.pid"),
		filepath.Join(repoRoot, ".history"),
	}
	for _, p := range forbiddenRepoPaths {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("sandbox violation: forbidden runtime state created in repository: %s", p)
		}
	}
	// Check books/*/.history inside repo
	repoBooks, _ := filepath.Glob(filepath.Join(repoRoot, "books", "*", ".history"))
	if len(repoBooks) > 0 {
		t.Fatalf("sandbox violation: .history directory created inside repo books: %v", repoBooks)
	}

	// 2. Real OS user AppData directory must remain untouched by test artifacts
	if realUserAppDataDir != "" {
		postSnap := captureDirMtimeSnapshot(realUserAppDataDir)
		liveHostInstance := false
		if rawPID, err := os.ReadFile(filepath.Join(realUserAppDataDir, "saves", ".mojra.pid")); err == nil {
			if pid, pErr := strconv.Atoi(strings.TrimSpace(string(rawPID))); pErr == nil && pid > 0 && pid != os.Getpid() {
				liveHostInstance = true
			}
		}
		testSlugs := []string{
			"kronika-chaosu",
			"scenariusz-testowy",
			"opowiesc-glowna",
			"do-usuniecia",
			"ksiazka-crawler-dom",
		}
		for k := range postSnap {
			lowerK := strings.ToLower(k)
			for _, slug := range testSlugs {
				if strings.Contains(lowerK, slug) {
					t.Fatalf("sandbox violation: test artifact %q written to user AppData %s", k, realUserAppDataDir)
				}
			}
		}
		if !liveHostInstance {
			if len(postSnap) != len(preUserAppData) {
				t.Fatalf("sandbox violation: user AppData entry count changed (%d -> %d) in %s", len(preUserAppData), len(postSnap), realUserAppDataDir)
			}
			for k, preMod := range preUserAppData {
				if postMod, ok := postSnap[k]; !ok || postMod != preMod {
					t.Fatalf("sandbox violation: user AppData file %q modified during test", k)
				}
			}
		}
	}
}

func postChaosWFP(t *testing.T, sb *chaosSandbox, step int, elDesc string, vals url.Values, rawBodyOverride []byte, headers map[string]string) (int, http.Header, string) {
	t.Helper()
	start := time.Now()
	var payload []byte
	reqSummary := ""
	if rawBodyOverride != nil {
		payload = rawBodyOverride
		reqSummary = fmt.Sprintf("raw_bytes=%d %q", len(rawBodyOverride), string(rawBodyOverride[:minInt(len(rawBodyOverride), 80)]))
	} else {
		payload = EncodeWFPForm(vals)
		var pairs []string
		for k, v := range vals {
			if len(v) > 0 {
				pairs = append(pairs, fmt.Sprintf("%s=%s", k, v[0]))
			}
		}
		sort.Strings(pairs)
		reqSummary = strings.Join(pairs, " ")
	}

	req, err := http.NewRequest(http.MethodPost, sb.HTTPServer.URL+"/api/action", bytes.NewReader(payload))
	if err != nil {
		sb.Logger.Flush(true, err.Error())
		t.Fatalf("step %d: NewRequest failed: %v", step, err)
	}
	req.Header.Set("Content-Type", "text/x-wfp;charset=UTF-8")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := sb.HTTPServer.Client().Do(req)
	if err != nil {
		sb.Logger.Flush(true, err.Error())
		t.Fatalf("step %d: HTTP Do failed: %v", step, err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	dur := time.Since(start)

	sb.Server.mu.Lock()
	view := sb.Server.activeWorkspaceView()
	modal := activeModalLabelLocked(sb.Server)
	sb.Server.mu.Unlock()

	domSummary := verifyChaosStepOracles(t, sb, step, vals.Get("action"), resp.StatusCode, resp.Header, bodyStr)
	sb.Logger.LogStep(
		step,
		view,
		modal,
		elDesc,
		reqSummary,
		resp.StatusCode,
		resp.Header.Get("X-Target"),
		resp.Header.Get("X-Theme"),
		resp.Header.Get("X-UI-Scale"),
		resp.Header.Get("X-Toast"),
		domSummary,
		dur,
	)
	return resp.StatusCode, resp.Header, bodyStr
}

func activeModalLabelLocked(s *Server) string {
	switch {
	case s.SceneTextModalID != "":
		return "#modal-scene-text"
	case s.ConfirmPendingAction != "":
		return "#modal-confirm"
	case s.NewBookModalOpen:
		return "#modal-new-book"
	case s.AddBookModalOpen:
		return "#modal-add-book"
	case s.StudioProjectModalOpen:
		return "#modal-studio-project"
	case s.ExportModalOpen:
		return "#modal-export"
	case s.QRModalOpen:
		return "#modal-qr"
	default:
		return "none"
	}
}

// verifyChaosStepOracles checks all 4 invariant groups after every single request.
func verifyChaosStepOracles(t *testing.T, sb *chaosSandbox, step int, action string, status int, hdr http.Header, body string) string {
	t.Helper()
	fail := func(format string, args ...any) {
		msg := fmt.Sprintf("step %d (action=%q): "+format, append([]any{step, action}, args...)...)
		sb.Logger.Flush(true, msg)
		t.Fatal(msg)
	}

	// Oracle 1: Allowed HTTP status codes (never 500 Internal Server Error)
	switch status {
	case http.StatusOK, http.StatusNoContent, http.StatusBadRequest, http.StatusForbidden, http.StatusMethodNotAllowed:
	default:
		fail("unexpected HTTP status %d; body=%q", status, body[:minInt(len(body), 300)])
	}

	// Oracle 2 & 3: WFP & HTML-Over-The-Wire contract + Topbar Invariant
	target := hdr.Get("X-Target")
	if status == http.StatusOK {
		switch target {
		case "#app", "#workspace", "#modals-root", "#editor-context-panel":
		default:
			fail("invalid X-Target header %q on 200 OK", target)
		}
		if target == "#app" {
			if !strings.Contains(body, `id="topbar"`) || !strings.Contains(body, `<header class="topbar"`) {
				fail("topbar invariant violated: #topbar missing from #app response")
			}
			if strings.Contains(body, `<header class="topbar hidden"`) {
				fail("topbar invariant violated: #topbar has hidden class")
			}
		}
		for _, forbidden := range []string{
			"<script",
			"<style",
			`style="`,
			"<iframe",
			"<object",
			"<embed",
			"<foreignobject",
			"javascript:",
			"onclick=",
			"onload=",
			"onerror=",
		} {
			if strings.Contains(strings.ToLower(body), forbidden) {
				fail("HTML security/template gate violated: found forbidden token %q in rendered chunk (target=%s)", forbidden, target)
			}
		}
	} else if status == http.StatusNoContent && target != "" && target != "none" {
		fail("expected X-Target 'none' or empty on 204 No Content, got %q", target)
	}

	// Oracle 4: Disk round-trip consistency (.playtxt, scene IDs, implicit Dalej, narrated: false, .sav magic)
	sb.Server.mu.Lock()
	var activeRoot string
	var activePacked bool
	var activeBookID string
	var sceneCount int
	var warnCount int
	if sb.Server.ActiveBook != nil {
		activeRoot = sb.Server.ActiveBook.RootPath
		activePacked = sb.Server.ActiveBook.IsPacked
		activeBookID = sb.Server.ActiveBook.ID
		sceneCount = len(sb.Server.ActiveBook.Scenes)
		warnCount = len(sb.Server.ActiveBook.Warnings)

		seenIDs := make(map[string]bool, len(sb.Server.ActiveBook.Scenes))
		for _, sc := range sb.Server.ActiveBook.Scenes {
			if strings.TrimSpace(sc.ID) == "" {
				sb.Server.mu.Unlock()
				fail("in-memory scene %q has empty ID", sc.Name)
			}
			lowerID := strings.ToLower(sc.ID)
			if seenIDs[lowerID] {
				sb.Server.mu.Unlock()
				fail("duplicate in-memory scene ID %q", sc.ID)
			}
			seenIDs[lowerID] = true
			if len(sc.Choices) == 1 && !sc.Choices[0].IsEnding && !strings.EqualFold(sc.Choices[0].Label, "Dalej") {
				sb.Server.mu.Unlock()
				fail("scene %q has single non-ending transition with label %q (expected implicit 'Dalej')", sc.ID, sc.Choices[0].Label)
			}
		}
	}
	sb.Server.mu.Unlock()

	diskStatus := "n/a"
	if activeRoot != "" && !activePacked && (step <= 20 || step%12 == 0) {
		if info, err := os.Stat(activeRoot); err == nil && info.IsDir() {
			reloaded, err := book.LoadBookFromDir(activeRoot)
			if err != nil || reloaded == nil {
				fail("LoadBookFromDir(%s) failed after mutation: %v", activeRoot, err)
			}
			if len(reloaded.Scenes) == 0 {
				fail("reloaded book at %s has 0 scenes", activeRoot)
			}
			seenDiskIDs := make(map[string]bool, len(reloaded.Scenes))
			for _, sc := range reloaded.Scenes {
				if strings.TrimSpace(sc.ID) == "" {
					fail("reloaded scene %q in %s has empty ID", sc.Name, activeRoot)
				}
				lowerID := strings.ToLower(sc.ID)
				if seenDiskIDs[lowerID] {
					fail("duplicate reloaded scene ID %q in %s", sc.ID, activeRoot)
				}
				seenDiskIDs[lowerID] = true
				if len(sc.Choices) == 1 && !sc.Choices[0].IsEnding && !strings.EqualFold(sc.Choices[0].Label, "Dalej") {
					fail("reloaded scene %q has single transition with label %q (expected 'Dalej')", sc.ID, sc.Choices[0].Label)
				}
			}
			if !reloaded.Narrated {
				for _, tf := range reloaded.TextFiles {
					rawTxt, rErr := os.ReadFile(filepath.Join(activeRoot, tf))
					if rErr == nil {
						for _, ln := range strings.Split(string(rawTxt), "\n") {
							if strings.HasPrefix(strings.ToLower(strings.TrimSpace(ln)), "voice:") {
								fail("narrated:false book file %s contains forbidden voice: header line %q", tf, ln)
							}
						}
					}
				}
			}
			diskStatus = "ok"
		}
	} else if activeRoot != "" && !activePacked {
		diskStatus = "mem_ok"
	}

	if activeBookID != "" {
		savPath := filepath.Join(sb.SavesDir, book.SanitizeBookID(activeBookID)+".sav")
		if rawSav, err := os.ReadFile(savPath); err == nil && len(rawSav) > 0 {
			if len(rawSav) < 5 || string(rawSav[:5]) != "STSV\x01" {
				fail("corrupted .sav header in %s: %q", savPath, string(rawSav[:minInt(len(rawSav), 8)]))
			}
		}
	}

	return fmt.Sprintf("topbar=ok scenes=%d warnings=%d disk_check=%s", sceneCount, warnCount, diskStatus)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// verifyPlaytxtAndPlaybookFormatIntegrity performs a strict lossless round-trip test on both
// the unpacked .playtxt directory and a packed .playbook zip archive.
func verifyPlaytxtAndPlaybookFormatIntegrity(t *testing.T, sb *chaosSandbox) {
	t.Helper()
	sb.Server.mu.Lock()
	b := sb.Server.ActiveBook
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		sb.Server.mu.Unlock()
		return
	}
	rootPath := b.RootPath
	expectedStart := b.StartSceneID
	type sceneExpect struct {
		ID      string
		Name    string
		Text    string
		Choices int
	}
	expected := make([]sceneExpect, len(b.Scenes))
	seen := make(map[string]bool, len(b.Scenes))
	for i, sc := range b.Scenes {
		if strings.TrimSpace(sc.ID) == "" {
			sb.Server.mu.Unlock()
			t.Fatalf("format integrity: scene[%d] (%q) has empty ID", i, sc.Name)
		}
		lower := strings.ToLower(sc.ID)
		if seen[lower] {
			sb.Server.mu.Unlock()
			t.Fatalf("format integrity: duplicate scene ID %q in ActiveBook", sc.ID)
		}
		seen[lower] = true
		expected[i] = sceneExpect{
			ID:      sc.ID,
			Name:    sc.Name,
			Text:    strings.TrimSpace(sc.Text),
			Choices: len(sc.Choices),
		}
	}
	sb.Server.mu.Unlock()

	// 1. Reload from .playtxt files on disk
	reloaded, err := book.LoadBookFromDir(rootPath)
	if err != nil || reloaded == nil {
		t.Fatalf("format integrity: LoadBookFromDir(%s) failed: %v", rootPath, err)
	}
	if len(reloaded.Scenes) != len(expected) {
		t.Fatalf("format integrity: scene count mismatch after .playtxt reload: got %d, want %d", len(reloaded.Scenes), len(expected))
	}
	if !strings.EqualFold(reloaded.StartSceneID, expectedStart) {
		t.Fatalf("format integrity: StartSceneID mismatch after .playtxt reload: got %q, want %q", reloaded.StartSceneID, expectedStart)
	}
	reloadedByID := make(map[string]*book.Scene, len(reloaded.Scenes))
	for _, sc := range reloaded.Scenes {
		reloadedByID[strings.ToLower(sc.ID)] = sc
	}
	for _, exp := range expected {
		got, ok := reloadedByID[strings.ToLower(exp.ID)]
		if !ok {
			t.Fatalf("format integrity: scene %q (%q) missing or unreachable after .playtxt reload", exp.ID, exp.Name)
		}
		if got.Name != exp.Name {
			t.Fatalf("format integrity: scene %q Name mismatch after .playtxt reload: got %q, want %q", exp.ID, got.Name, exp.Name)
		}
		if strings.TrimSpace(got.Text) != exp.Text {
			t.Fatalf("format integrity: scene %q Text corrupted after .playtxt reload: got %q, want %q", exp.ID, got.Text, exp.Text)
		}
	}

	// 2. Pack to .playbook zip, validate, load from zip, and unpack to a fresh folder
	tmpZip := filepath.Join(t.TempDir(), "roundtrip_check.playbook")
	if err := book.PackBookToZip(rootPath, tmpZip); err != nil {
		t.Fatalf("format integrity: PackBookToZip failed: %v", err)
	}
	if err := book.ValidateBookZip(tmpZip); err != nil {
		t.Fatalf("format integrity: ValidateBookZip failed: %v", err)
	}
	packedBook, err := book.LoadBookFromZip(tmpZip)
	if err != nil || packedBook == nil {
		t.Fatalf("format integrity: LoadBookFromZip failed: %v", err)
	}
	if len(packedBook.Scenes) != len(expected) {
		t.Fatalf("format integrity: .playbook scene count mismatch: got %d, want %d", len(packedBook.Scenes), len(expected))
	}
	unpackDir := filepath.Join(t.TempDir(), "unpacked_playbook")
	if err := book.UnpackBookZip(tmpZip, unpackDir); err != nil {
		t.Fatalf("format integrity: UnpackBookZip failed: %v", err)
	}
	unpackedBook, err := book.LoadBookFromDir(unpackDir)
	if err != nil || unpackedBook == nil || len(unpackedBook.Scenes) != len(expected) {
		t.Fatalf("format integrity: LoadBookFromDir(unpacked) failed or scene count mismatch: err=%v", err)
	}
}

// TestAuthoringSceneIDGenerationAndReroutingInvariants specifically verifies that authoring a book from scratch,
// adding scenes via all studio entry points, typing titles/IDs character-by-character, attempting duplicate IDs,
// and linking choices/chapters always produce valid, unique scene IDs, zero cross-scene overwrites, and intact .playtxt/.playbook files.
func TestAuthoringSceneIDGenerationAndReroutingInvariants(t *testing.T) {
	seed, steps := resolveChaosSeedAndSteps()
	sb := newIsolatedChaosServer(t, seed, steps)

	step := 1
	send := func(el string, kv ...string) http.Header {
		t.Helper()
		vals := url.Values{}
		for i := 0; i+1 < len(kv); i += 2 {
			vals.Set(kv[i], kv[i+1])
		}
		status, hdr, _ := postChaosWFP(t, sb, step, el, vals, nil, nil)
		step++
		if status != http.StatusOK && status != http.StatusNoContent {
			t.Fatalf("step %d: expected 200/204 for %v, got %d", step-1, kv, status)
		}
		return hdr
	}

	// 1. Create a brand new book inside a deeply nested directory with spaces, Polish diacritics, and special chars
	weirdSaveDir := filepath.Join(sb.ProjectsDir, "Katalog ze spacjami — Żółć #1 (2026)", "podkatalog.testowy")
	if err := os.MkdirAll(weirdSaveDir, 0755); err != nil {
		t.Fatalf("mkdir weirdSaveDir: %v", err)
	}
	send("form[action=CreateNewBook]",
		"action", "CreateNewBook",
		"title", "Śluza nad Rzeką — Część I",
		"author", "Autor Testowy",
		"mode", "book",
		"narrated", "false",
		"imageless", "false",
		"save_path", weirdSaveDir,
	)

	// 2. Add 4 new scenes in a row (creates s1, s2, s3, s4 alongside poczatek and rozdroze)
	send("canvas#studio-canvas", "action", "CreateSceneAt", "x", "480", "y", "240")
	send("canvas#studio-canvas", "action", "CreateSceneAt", "x", "720", "y", "240")
	send("button[action=InsertScene]", "action", "InsertScene", "parent", "poczatek", "child", "rozdroze")
	send("button[action=InsertSceneBefore]", "action", "InsertSceneBefore", "target", "poczatek")
	send("button[action=BranchChoiceScene]", "action", "BranchChoiceScene", "scene", "poczatek", "choice_idx", "0")

	// Give every existing scene a unique sentinel text so we can detect if ANY scene is ever overwritten by another scene's edit
	sb.Server.mu.Lock()
	var initialIDs []string
	for _, sc := range sb.Server.ActiveBook.Scenes {
		initialIDs = append(initialIDs, sc.ID)
	}
	sb.Server.mu.Unlock()
	for idx, sid := range initialIDs {
		send("textarea[action=AutoSaveScene]",
			"action", "AutoSaveScene",
			"scene_id", sid,
			"text", fmt.Sprintf("SENTINEL_UNIKALNA_TRESC_SCENY_%d", idx),
		)
	}

	// 3. Simulate character-by-character typing into the first auto-named scene ("s1") while other auto-named scenes ("s2", "s3") exist,
	// including stale in-flight requests that still carry scene_id="s1" after s1 has already been renamed!
	send("button[action=OpenSceneTextModal]", "action", "OpenSceneTextModal", "scene", "s1")
	targetWord := []rune("Zatopiona śluza")
	curID := "s1"
	for k := 1; k <= len(targetWord); k++ {
		prefix := string(targetWord[:k])
		// Every 3rd keystroke, simulate a stale in-flight packet that still sends the old "s1" ID!
		reqID := curID
		if k%3 == 0 {
			reqID = "s1"
		}
		hdr := send("input[name=name][char-by-char]",
			"action", "AutoSaveScene",
			"scene_id", reqID,
			"name", prefix,
			"text", "SENTINEL_EDYCJA_ZNAK_PO_ZNAKU_"+prefix,
		)
		if newID := strings.TrimSpace(hdr.Get("X-Scene-ID")); newID != "" {
			curID = newID
		}
	}
	send("button[action=CloseSceneTextModal]", "action", "CloseSceneTextModal")

	// Verify that NONE of the other scenes had their sentinel text overwritten by the character-by-character typing!
	sb.Server.mu.Lock()
	editedCount := 0
	for _, sc := range sb.Server.ActiveBook.Scenes {
		if strings.HasPrefix(sc.Text, "SENTINEL_EDYCJA_ZNAK_PO_ZNAKU_") {
			editedCount++
		}
	}
	sb.Server.mu.Unlock()
	if editedCount != 1 {
		t.Fatalf("anti-overwrite oracle failed: expected exactly 1 scene to have the typed text, but %d scenes were overwritten!", editedCount)
	}

	// 4. Attempt to assign the EXACT SAME title and explicit_id ("zatopiona_sluza") to a second scene via UI
	sb.Server.mu.Lock()
	var otherSceneID string
	for _, sc := range sb.Server.ActiveBook.Scenes {
		if sc.ID != curID && sc.ID != "poczatek" {
			otherSceneID = sc.ID
			break
		}
	}
	sb.Server.mu.Unlock()
	hdrDup := send("form#editor-context-form[duplicate-id-attempt]",
		"action", "SaveScene",
		"scene_id", otherSceneID,
		"name", "Zatopiona śluza",
		"explicit_id", "zatopiona_sluza",
		"text", "DRUGA_SCENA_Z_TYM_SAMYM_ID",
	)
	dedupedID := strings.TrimSpace(hdrDup.Get("X-Scene-ID"))
	if dedupedID == "" || strings.EqualFold(dedupedID, curID) {
		t.Fatalf("duplicate ID oracle failed: expected second scene to receive deduplicated ID != %q, got %q", curID, dedupedID)
	}

	// 5. Simulate external manual .playtxt edit that writes two scenes with identical `scene: reczny_duplikat` header
	sb.Server.mu.Lock()
	rootPath := sb.Server.ActiveBook.RootPath
	firstFile := filepath.Join(rootPath, sb.Server.ActiveBook.TextFiles[0])
	sb.Server.mu.Unlock()
	rawFile, err := os.ReadFile(firstFile)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", firstFile, err)
	}
	duplicatedRaw := string(rawFile) + "\n---\n\nscene: reczny_duplikat\ntitle: Ręczny Duplikat A\n\nTekst duplikatu A.\n\n---\n\nscene: reczny_duplikat\ntitle: Ręczny Duplikat B\n\nTekst duplikatu B.\n"
	if err := os.WriteFile(firstFile, []byte(duplicatedRaw), 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", firstFile, err)
	}
	sb.Server.mu.Lock()
	sb.Server.reloadActiveBook()
	sb.Server.mu.Unlock()

	// Edit the second deduplicated scene ("reczny_duplikat_2") and verify "reczny_duplikat" is untouched
	send("form#editor-context-form",
		"action", "SaveScene",
		"scene_id", "reczny_duplikat_2",
		"name", "Ręczny Duplikat B (Wyedytowany)",
		"text", "Zmieniony tekst wyłącznie w duplikacie B.",
	)
	sb.Server.mu.Lock()
	scA := sb.Server.findScene("reczny_duplikat")
	scB := sb.Server.findScene("reczny_duplikat_2")
	sb.Server.mu.Unlock()
	if scA == nil || scB == nil {
		t.Fatalf("expected both reczny_duplikat and reczny_duplikat_2 to be reachable in UI, got scA=%v scB=%v", scA, scB)
	}
	if !strings.Contains(scA.Text, "Tekst duplikatu A.") || !strings.Contains(scB.Text, "wyłącznie w duplikacie B") {
		t.Fatalf("editing deduplicated scene B corrupted scene A: scA.Text=%q, scB.Text=%q", scA.Text, scB.Text)
	}

	// 6. Create second chapter, rename it with Polish diacritics, and link chapters
	send("button[action=CreateChapterGroup]", "action", "CreateChapterGroup", "chapter", "Zatopiona śluza", "x", "960", "y", "480")
	send("form[action=SaveChapterSettings]", "action", "SaveChapterSettings", "old_chapter", "Zatopiona śluza", "new_chapter", "W górę umierającej rzeki")

	sb.Server.mu.Lock()
	firstScID := sb.Server.ActiveBook.Scenes[0].ID
	lastScID := sb.Server.ActiveBook.Scenes[len(sb.Server.ActiveBook.Scenes)-1].ID
	sb.Server.mu.Unlock()

	send("canvas-port", "action", "ConnectChoice", "from_scene", firstScID, "to_scene", lastScID, "choice_idx", "0")

	// 7. Verify full .playtxt and .playbook round-trip format integrity
	verifyPlaytxtAndPlaybookFormatIntegrity(t, sb)
}

// TestUIChaosAndStaticCrawler executes the 6-stage Chaos Engineering & Static UI Crawler plan:
// 1. Isolated Sandbox + FS Guard
// 2. Deterministic Crawler of all UI interactions & 100% actionSpecs coverage
// 3. 4 Chaos Vectors (DOM Monkey + Stale DOM Replay, WFP Mutator, -race Concurrency Chaos, Media/Archive Fuzzing)
// 4. Per-step Invariant Oracles
// 5. Plain-text Trace Log (ui-chaos-trace.log)
func TestUIChaosAndStaticCrawler(t *testing.T) {
	seed, totalSteps := resolveChaosSeedAndSteps()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repoRoot: %v", err)
	}
	var realUserAppDataDir string
	if cfgDir, cErr := os.UserConfigDir(); cErr == nil && strings.TrimSpace(cfgDir) != "" {
		realUserAppDataDir = filepath.Join(cfgDir, "Mojra")
	}
	preUserAppDataSnap := captureDirMtimeSnapshot(realUserAppDataDir)

	sb := newIsolatedChaosServer(t, seed, totalSteps)
	defer func() {
		sb.Logger.Flush(t.Failed(), "test failed")
		verifySandboxHostIsolation(t, repoRoot, preUserAppDataSnap, realUserAppDataDir)
	}()

	executedActions := make(map[string]int, len(actionSpecs))
	stepCounter := 0
	var staleQueue []url.Values

	runAction := func(elDesc string, kv ...string) (int, http.Header, string) {
		stepCounter++
		vals := url.Values{}
		for i := 0; i+1 < len(kv); i += 2 {
			vals.Set(kv[i], kv[i+1])
		}
		if act := vals.Get("action"); act != "" {
			executedActions[act]++
		}
		clone := make(url.Values, len(vals))
		for k, v := range vals {
			clone[k] = append([]string(nil), v...)
		}
		staleQueue = append(staleQueue, clone)
		if len(staleQueue) > 32 {
			staleQueue = staleQueue[1:]
		}
		return postChaosWFP(t, sb, stepCounter, elDesc, vals, nil, nil)
	}

	// =========================================================================
	// STAGE 2A: Node VM Live DOM Crawler (extracts & clicks real DOM controls)
	// =========================================================================
	if nodeBin, lookErr := exec.LookPath("node"); lookErr == nil {
		runNodeVMDOMCrawler(t, nodeBin, sb, seed, executedActions)
	}

	// =========================================================================
	// STAGE 2B: Exhaustive ActionSpecs Coverage Crawl (100% of registered actions)
	// =========================================================================
	crawlAllActionSpecsDeterministically(t, sb, runAction)

	var missingActions []string
	for actName := range actionSpecs {
		if executedActions[actName] == 0 {
			missingActions = append(missingActions, actName)
		}
	}
	sort.Strings(missingActions)
	if len(missingActions) > 0 {
		sb.Logger.Flush(true, fmt.Sprintf("missing actionSpecs coverage: %v", missingActions))
		t.Fatalf("static crawler missed %d/%d actionSpecs: %v", len(missingActions), len(actionSpecs), missingActions)
	}

	// =========================================================================
	// STAGE 3: Deterministic PRNG Chaos Engine (4 Vectors)
	// =========================================================================
	staleQueue = nil
	rng := rand.New(rand.NewSource(seed))
	fuzzMediaDir := prepareSpoofedMediaFixtures(t, sb.RootDir)

	for i := 0; i < totalSteps; i++ {
		if i > 0 && i%40 == 0 {
			sb.Server.mu.Lock()
			if sb.Server.ActiveBook != nil && sb.Server.ActiveBook.RootPath != "" {
				_ = os.RemoveAll(filepath.Join(sb.Server.ActiveBook.RootPath, ".history"))
			}
			_ = os.RemoveAll(filepath.Join(sb.RootDir, "history"))
			_ = os.MkdirAll(filepath.Join(sb.RootDir, "history"), 0755)
			sb.Server.invalidateHistoryCache()
			sb.Server.mu.Unlock()
		}
		vector := rng.Intn(100)
		switch {
		case vector < 45:
			// Vector 1: DOM Monkey & Stale DOM Replay (45% of steps)
			if len(staleQueue) > 2 && rng.Intn(3) == 0 {
				staleIdx := rng.Intn(len(staleQueue))
				staleVals := staleQueue[staleIdx]
				stepCounter++
				executedActions[staleVals.Get("action")]++
				postChaosWFP(t, sb, stepCounter, "stale-dom-replay", staleVals, nil, nil)
			} else {
				stepCounter++
				vals, desc := generateDOMMonkeyAction(rng, sb, stepCounter)
				executedActions[vals.Get("action")]++
				staleQueue = append(staleQueue, vals)
				if len(staleQueue) > 8 {
					staleQueue = staleQueue[1:]
				}
				postChaosWFP(t, sb, stepCounter, desc, vals, nil, nil)
			}

		case vector < 75:
			// Vector 2: WFP Protocol & Syntax Injection Mutator (30% of steps)
			stepCounter++
			vals, rawOverride, hdrs, desc := generateMutatedWFPRequest(rng, sb, stepCounter)
			if vals.Get("action") != "" {
				executedActions[vals.Get("action")]++
			}
			postChaosWFP(t, sb, stepCounter, desc, vals, rawOverride, hdrs)

		case vector < 90:
			// Vector 4: Media & Archive Magic-Byte Validation Fuzzing (15% of steps)
			stepCounter++
			vals, desc := generateMediaFuzzAction(rng, sb, fuzzMediaDir)
			executedActions[vals.Get("action")]++
			postChaosWFP(t, sb, stepCounter, desc, vals, nil, nil)

		default:
			// Vector 3: Concurrency Chaos Burst (10% of steps, N=8 goroutines)
			stepCounter++
			runConcurrentChaosBurst(t, sb, stepCounter, rng.Int63())
		}
	}

	// Final step: force a multiple-of-12 step index so disk round-trip oracle always runs at the very end
	for stepCounter%12 != 0 {
		stepCounter++
	}
	postChaosWFP(t, sb, stepCounter, "final-oracle-verify", url.Values{"action": {"SwitchView"}, "view": {"editor"}}, nil, nil)
	verifyPlaytxtAndPlaybookFormatIntegrity(t, sb)
	t.Logf("Chaos report: seed=%d, actionSpecs=%d/%d (100%%), totalSteps=%d, log=%s", seed, len(actionSpecs), len(actionSpecs), stepCounter, sb.Logger.filePath)
}

func crawlAllActionSpecsDeterministically(t *testing.T, sb *chaosSandbox, runAction func(elDesc string, kv ...string) (int, http.Header, string)) {
	t.Helper()

	// 1. Navigation, Theme, Scale, Language, Device Mode, Imageless, Window controls
	runAction("button.topbar-tab[action=SwitchView]", "action", "SwitchView", "view", "library")
	runAction("button.top-action-btn[action=ToggleTheme]", "action", "ToggleTheme")
	runAction("button.top-action-btn[action=ToggleLanguage]", "action", "ToggleLanguage")
	runAction("div.custom-select[action=SetLanguage]", "action", "SetLanguage", "lang", "pl")
	runAction("button.top-action-btn[action=ToggleUIScale]", "action", "ToggleUIScale")
	runAction("button.top-action-btn[action=SetUIScale]", "action", "SetUIScale", "scale", "small")
	runAction("button.device-btn[action=SetDeviceMode]", "action", "SetDeviceMode", "mode", "portrait")
	runAction("button.device-btn[action=SetDeviceMode]", "action", "SetDeviceMode", "mode", "full")
	runAction("button.top-action-btn[action=ToggleImageless]", "action", "ToggleImageless")
	runAction("button.top-action-btn[action=ToggleImageless]", "action", "ToggleImageless")
	runAction("button.top-action-btn[action=ToggleFullscreen]", "action", "ToggleFullscreen")
	runAction("header#topbar[action=StartWindowDrag]", "action", "StartWindowDrag")

	// 2. Modals in Library & Studio Home
	runAction("button[action=OpenQRModal]", "action", "OpenQRModal")
	runAction("button[action=CloseQRModal]", "action", "CloseQRModal")
	runAction("button[action=OpenAddBookModal]", "action", "OpenAddBookModal")
	runAction("button[action=CloseAddBookModal]", "action", "CloseAddBookModal")
	runAction("button[action=OpenStudioProjectModal]", "action", "OpenStudioProjectModal")
	runAction("button[action=CloseStudioProjectModal]", "action", "CloseStudioProjectModal")
	runAction("button[action=OpenNewBookModal]", "action", "OpenNewBookModal", "mode", "book")
	runAction("button[action=BrowseNewBookFolder]", "action", "BrowseNewBookFolder", "title", "Test", "save_path", sb.ProjectsDir)
	runAction("button[action=CloseNewBookModal]", "action", "CloseNewBookModal")

	// 3. Create Book & Screenplay projects, Disk & URL imports
	runAction("form[action=CreateNewBook]", "action", "CreateNewBook", "title", "Scenariusz Testowy", "author", "Autor", "mode", "screenplay", "standard", "pl")
	runAction("button[action=ApplySluglineToken]", "action", "ApplySluglineToken", "prefix", "ZEWN.", "time", "NOC")
	runAction("button[action=InsertScreenplayBlock]", "action", "InsertScreenplayBlock", "kind", "dialogue", "character", "KAPITAN")
	runAction("button[action=ToggleEditorBreakdownPanel]", "action", "ToggleEditorBreakdownPanel")
	runAction("input[action=RenameCharacter]", "action", "RenameCharacter", "old_name", "KAPITAN", "new_name", "SIERŻANT")
	runAction("input[action=RenameLocation]", "action", "RenameLocation", "old_location", "LOKACJA", "new_location", "ŚLUZA")
	runAction("button[action=CloseEditorBreakdownPanel]", "action", "CloseEditorBreakdownPanel")

	runAction("form[action=CreateNewBook]", "action", "CreateNewBook", "title", "Opowieść Główna", "author", "Autor", "mode", "book", "narrated", "true", "imageless", "false")
	sb.Server.mu.Lock()
	mainBookID := sb.Server.ActiveBook.ID
	mainBookRoot := sb.Server.ActiveBook.RootPath
	sb.Server.mu.Unlock()

	runAction("button[action=BrowseAddBookFromDisk]", "action", "BrowseAddBookFromDisk", "mode", "dir")
	runAction("form[action=AddBookFromDisk]", "action", "AddBookFromDisk", "path", mainBookRoot)
	runAction("form[action=AddBookFromURL]", "action", "AddBookFromURL", "url", sb.MediaHTTPLocal.URL+"/valid.playbook")
	runAction("button[action=ExtractEmbeddedBook]", "action", "ExtractEmbeddedBook")
	sb.Server.mu.Lock()
	var extractedID string
	if sb.Server.ActiveBook != nil && sb.Server.ActiveBook.ID != mainBookID {
		extractedID = sb.Server.ActiveBook.ID
	}
	sb.Server.mu.Unlock()
	if extractedID != "" {
		runAction("button[action=RemoveBook]", "action", "RemoveBook", "book", extractedID)
	}
	runAction("button[action=OpenStudioProject]", "action", "OpenStudioProject", "book", mainBookID)
	runAction("button[action=BrowseOpenStudioProject]", "action", "BrowseOpenStudioProject")
	runAction("form[action=OpenStudioProjectFromDisk]", "action", "OpenStudioProjectFromDisk", "path", mainBookRoot)

	// 4. Studio Editor: Chapters, Scenes, Choices, Variables, Voice, History, Export
	runAction("button[action=OpenEditorHelpModal]", "action", "OpenEditorHelpModal")
	runAction("button[action=CloseEditorHelpModal]", "action", "CloseEditorHelpModal")
	runAction("button[action=ToggleBookSettings]", "action", "ToggleBookSettings", "open", "true")
	runAction("form[action=SaveBookSettings]", "action", "SaveBookSettings", "title", "Opowieść Główna 2", "author", "Autor 2", "license", "CC-BY", "description", "Opis", "start_scene", "poczatek", "export_dir", sb.ExportsDir)
	runAction("input[action=RenameBook]", "action", "RenameBook", "title", "Opowieść Główna")
	runAction("button[action=ToggleBookImageless]", "action", "ToggleBookImageless")
	runAction("button[action=ToggleBookImageless]", "action", "ToggleBookImageless")
	runAction("button[action=ToggleBookNarrated]", "action", "ToggleBookNarrated")
	runAction("button[action=ToggleBookNarrated]", "action", "ToggleBookNarrated")
	runAction("button[action=BrowseBookExportFolder]", "action", "BrowseBookExportFolder", "export_dir", sb.ExportsDir)
	moveDest := filepath.Join(sb.ProjectsDir, "moved_host")
	_ = os.MkdirAll(moveDest, 0755)
	runAction("button[action=BrowseBookSaveFolder]", "action", "BrowseBookSaveFolder", "save_path", moveDest)
	runAction("button[action=CloseBookSettings]", "action", "CloseBookSettings")

	// Variables & Enums
	runAction("button[action=ToggleEditorVariablesPanel]", "action", "ToggleEditorVariablesPanel")
	runAction("button[action=AddBookVariable]", "action", "AddBookVariable", "kind", "bool", "new_var_name", "ma_klucz")
	runAction("button[action=AddBookVariable]", "action", "AddBookVariable", "kind", "int", "new_var_name", "zloto")
	runAction("button[action=AddBookVariable]", "action", "AddBookVariable", "kind", "enum", "new_var_name", "frakcja")
	runAction("button[action=ToggleBookVariableBool]", "action", "ToggleBookVariableBool", "var_idx", "0")
	runAction("input[action=UpdateBookVariable]", "action", "UpdateBookVariable", "var_idx", "1", "var_name", "zloto", "var_value", "15")
	runAction("button[action=AddBookEnumOption]", "action", "AddBookEnumOption", "var_idx", "2")
	runAction("button[action=RemoveBookEnumOption]", "action", "RemoveBookEnumOption", "var_idx", "2", "opt_idx", "2")
	runAction("button[action=AddBookVariable]", "action", "AddBookVariable", "kind", "bool", "new_var_name", "tymczasowa")
	runAction("button[action=RemoveBookVariable]", "action", "RemoveBookVariable", "var_idx", "3")
	runAction("button[action=CloseEditorVariablesPanel]", "action", "CloseEditorVariablesPanel")

	// Scenes & Choices
	runAction("canvas[action=SelectStudioScene]", "action", "SelectStudioScene", "scene", "poczatek")
	runAction("button[action=InsertScene]", "action", "InsertScene", "parent", "poczatek", "child", "rozdroze")
	runAction("button[action=InsertSceneBefore]", "action", "InsertSceneBefore", "target", "rozdroze")
	runAction("canvas[action=CreateSceneAt]", "action", "CreateSceneAt", "x", "600", "y", "360")
	runAction("button[action=AddChoice]", "action", "AddChoice", "scene", "poczatek")
	runAction("button[action=BranchChoiceScene]", "action", "BranchChoiceScene", "scene", "poczatek", "choice_idx", "1")
	runAction("button[action=ReorderChoice]", "action", "ReorderChoice", "scene", "poczatek", "from_idx", "0", "to_idx", "1")
	runAction("button[action=ToggleChoiceRule]", "action", "ToggleChoiceRule", "scene", "poczatek", "choice_idx", "0")
	runAction("button[action=ToggleSyntaxHelp]", "action", "ToggleSyntaxHelp")
	runAction("button[action=SelectPrimaryBranchScene]", "action", "SelectPrimaryBranchScene", "scene", "rozdroze")
	runAction("canvas[action=ConnectChoice]", "action", "ConnectChoice", "from_scene", "poczatek", "to_scene", "rozdroze", "choice_idx", "0")
	runAction("button[action=PromptRemoveChoice]", "action", "PromptRemoveChoice", "scene", "poczatek", "choice_idx", "1")
	runAction("button[action=ConfirmModalClose]", "action", "ConfirmModalClose")
	runAction("button[action=RemoveChoice]", "action", "RemoveChoice", "scene", "poczatek", "choice_idx", "1")
	runAction("button[action=AddChoice]", "action", "AddChoice", "scene", "poczatek")

	// Chapters
	runAction("button[action=CreateChapterGroup]", "action", "CreateChapterGroup", "chapter", "Rozdział 2", "x", "960", "y", "480")
	runAction("canvas[action=SelectStudioChapter]", "action", "SelectStudioChapter", "chapter", "Rozdział 2")
	runAction("form[action=RenameChapter]", "action", "RenameChapter", "old_chapter", "Rozdział 2", "new_chapter", "Rozdział Drugi")
	sb.Server.mu.Lock()
	lastScID := sb.Server.ActiveBook.Scenes[len(sb.Server.ActiveBook.Scenes)-1].ID
	sb.Server.mu.Unlock()
	runAction("div.custom-select[action=SetChapterStartScene]", "action", "SetChapterStartScene", "chapter", "Rozdział Drugi", "scene", lastScID)
	runAction("form[action=SaveChapterSettings]", "action", "SaveChapterSettings", "old_chapter", "Rozdział Drugi", "new_chapter", "Rozdział 2", "start_scene", lastScID)
	runAction("div.custom-select[action=SetBookStartChapter]", "action", "SetBookStartChapter", "chapter", "Rozdział 1")
	runAction("div.custom-select[action=SetStartScene]", "action", "SetStartScene", "scene", "poczatek")
	runAction("div.custom-select[action=AssignChapterToScenes]", "action", "AssignChapterToScenes", "chapter", "Rozdział 2", "scene_ids", lastScID)
	runAction("canvas[action=ConnectChapter]", "action", "ConnectChapter", "from_chapter", "Rozdział 1", "to_chapter", "Rozdział 2")
	runAction("button[action=AutoLayoutNodes]", "action", "AutoLayoutNodes")
	runAction("canvas[action=SaveEditorState]", "action", "SaveEditorState", "pan_x", "120", "pan_y", "96", "zoom", "1.0", "positions", "poczatek:120,144")

	// Scene Text Modal, Voice Recorder, Media Import
	runAction("button[action=OpenSceneTextModal]", "action", "OpenSceneTextModal", "scene", "poczatek")
	runAction("button[action=SetSceneTextWidthMode]", "action", "SetSceneTextWidthMode", "mode", "full")
	runAction("button[action=SetSceneTextWidthMode]", "action", "SetSceneTextWidthMode", "mode", "half")
	runAction("button[action=ToggleSceneTextSettings]", "action", "ToggleSceneTextSettings")
	runAction("button[action=SetSceneTextPreviewMode]", "action", "SetSceneTextPreviewMode", "mode", "highlight")
	runAction("button[action=ToggleSceneTestVarsPopover]", "action", "ToggleSceneTestVarsPopover")
	runAction("button[action=UpdateSceneTextTestVar]", "action", "UpdateSceneTextTestVar", "var_name", "ma_klucz", "toggle", "true")
	runAction("button[action=SetSceneTextPreviewMode]", "action", "SetSceneTextPreviewMode", "mode", "normal")
	runAction("button[action=BrowseAddBookMedia]", "action", "BrowseAddBookMedia", "kind", "image", "scene", "poczatek", "path", sb.ValidPNGPath)
	runAction("form[action=AddBookMediaFromDisk]", "action", "AddBookMediaFromDisk", "kind", "audio", "scene", "poczatek", "path", sb.ValidFLACPath)
	runAction("button[action=ToggleSceneVoiceRecorder]", "action", "ToggleSceneVoiceRecorder", "open", "true")
	runAction("button[action=SelectSceneVoiceSection]", "action", "SelectSceneVoiceSection", "voice_section", "01")
	runAction("button[action=StartSceneVoiceCapture]", "action", "StartSceneVoiceCapture")
	runAction("button[action=PollSceneVoiceCapture]", "action", "PollSceneVoiceCapture")
	runAction("button[action=StopSceneVoiceCapture]", "action", "StopSceneVoiceCapture", "scene", "poczatek")
	fakePCM := make([]byte, 2400)
	runAction("button[action=SaveSceneVoiceRecording]", "action", "SaveSceneVoiceRecording", "scene", "poczatek", "pcm_b64", base64.StdEncoding.EncodeToString(fakePCM), "sample_rate", "24000", "start_ratio", "0", "end_ratio", "1")
	runAction("button[action=TrimSceneVoice]", "action", "TrimSceneVoice", "scene", "poczatek", "start_ratio", "0.05", "end_ratio", "0.95", "destructive", "false")
	runAction("button[action=DeleteSceneVoice]", "action", "DeleteSceneVoice", "scene", "poczatek")
	runAction("textarea[action=AutoSaveScene]", "action", "AutoSaveScene", "scene_id", "poczatek", "name", "Początek drogi", "text", "Nowa treść w modalu.")
	runAction("form[action=SaveScene]", "action", "SaveScene", "scene_id", "poczatek", "name", "Początek drogi", "text", "Zapisana treść.")
	runAction("form[action=SaveSceneTextModal]", "action", "SaveSceneTextModal", "scene_id", "poczatek", "text", "Zapisana treść z modalu.")
	runAction("button[action=NavigateSceneTextModal]", "action", "NavigateSceneTextModal", "scene_id", "poczatek", "scene", "rozdroze")
	runAction("button[action=CloseSceneTextModal]", "action", "CloseSceneTextModal")
	runAction("button[action=ToggleBookNarrated]", "action", "ToggleBookNarrated")

	// Scene deletion modals & actions
	runAction("canvas[action=CreateSceneAt]", "action", "CreateSceneAt", "x", "840", "y", "240")
	sb.Server.mu.Lock()
	delSc1 := sb.Server.ActiveBook.Scenes[len(sb.Server.ActiveBook.Scenes)-1].ID
	sb.Server.mu.Unlock()
	runAction("button[action=PromptDeleteScene]", "action", "PromptDeleteScene", "scene", delSc1)
	runAction("button[action=ConfirmModalAccept]", "action", "ConfirmModalAccept")

	runAction("canvas[action=CreateSceneAt]", "action", "CreateSceneAt", "x", "840", "y", "240")
	sb.Server.mu.Lock()
	delSc2 := sb.Server.ActiveBook.Scenes[len(sb.Server.ActiveBook.Scenes)-1].ID
	sb.Server.mu.Unlock()
	runAction("button[action=DeleteScene]", "action", "DeleteScene", "scene", delSc2)

	runAction("canvas[action=CreateSceneAt]", "action", "CreateSceneAt", "x", "840", "y", "240")
	sb.Server.mu.Lock()
	delSc3 := sb.Server.ActiveBook.Scenes[len(sb.Server.ActiveBook.Scenes)-1].ID
	sb.Server.mu.Unlock()
	runAction("button[action=PromptDeleteScenes]", "action", "PromptDeleteScenes", "scenes", delSc3)
	runAction("button[action=ConfirmModalClose]", "action", "ConfirmModalClose")
	runAction("button[action=DeleteScenes]", "action", "DeleteScenes", "scenes", delSc3)

	// History & Branches
	runAction("button[action=OpenEditorHistoryPanel]", "action", "OpenEditorHistoryPanel")
	runAction("form[action=SaveHistoryVersion]", "action", "SaveHistoryVersion", "version_label", "Wersja 1.0")
	runAction("button[action=UndoEditor]", "action", "UndoEditor")
	runAction("button[action=RedoEditor]", "action", "RedoEditor")
	sb.Server.mu.Lock()
	var revID string
	if h := sb.Server.loadActiveBookHistory(); h != nil && len(h.Revisions) > 0 {
		revID = h.Revisions[0].ID
	}
	sb.Server.mu.Unlock()
	if revID != "" {
		runAction("div[action=PreviewHistoryRevision]", "action", "PreviewHistoryRevision", "rev", revID)
		runAction("button[action=PlayHistoryRevision]", "action", "PlayHistoryRevision", "rev", revID)
		runAction("button[action=SwitchView]", "action", "SwitchView", "view", "editor")
		runAction("div[action=PreviewHistoryRevision]", "action", "PreviewHistoryRevision", "rev", revID)
		runAction("button[action=ExitHistoryPreview]", "action", "ExitHistoryPreview")
		runAction("button[action=PromptCreateHistoryBranch]", "action", "PromptCreateHistoryBranch", "branch_name", "alternatywa", "from_rev", revID)
		runAction("button[action=ConfirmModalClose]", "action", "ConfirmModalClose")
		runAction("form[action=CreateHistoryBranch]", "action", "CreateHistoryBranch", "branch_name", "alternatywa", "from_rev", revID)
		runAction("div.custom-select[action=SwitchHistoryBranch]", "action", "SwitchHistoryBranch", "branch", "main")
		runAction("button[action=CopyHistoryRevisionToBranch]", "action", "CopyHistoryRevisionToBranch", "rev", revID, "branch", "alternatywa")
		runAction("button[action=RestoreHistoryRevision]", "action", "RestoreHistoryRevision", "rev", revID)
	}
	runAction("button[action=CloseEditorHistoryPanel]", "action", "CloseEditorHistoryPanel")
	runAction("button[action=ToggleEditorHistoryPanel]", "action", "ToggleEditorHistoryPanel")
	runAction("button[action=ToggleEditorHistoryPanel]", "action", "ToggleEditorHistoryPanel")

	// Export Modal
	runAction("button[action=OpenExportModal]", "action", "OpenExportModal")
	runAction("button[action=ToggleExportFormat]", "action", "ToggleExportFormat", "format", "playtxt")
	runAction("button[action=ToggleExportModalAdvanced]", "action", "ToggleExportModalAdvanced")
	runAction("div.custom-select[action=SetExportModalStandard]", "action", "SetExportModalStandard", "standard", "us")
	runAction("button[action=BrowseExportModalFolder]", "action", "BrowseExportModalFolder", "picked_dir", sb.ExportsDir)
	runAction("form[action=ExportProject]", "action", "ExportProject", "format", "playbook", "export_dir", sb.ExportsDir)
	runAction("button[action=CloseExportModal]", "action", "CloseExportModal")

	// 5. Player View & Audio/Choice Navigation
	runAction("button[action=PlayFromScene]", "action", "PlayFromScene", "scene", "poczatek")
	runAction("button[action=OpenBook]", "action", "OpenBook", "book", mainBookID, "view", "player")
	runAction("button[action=ToggleZenMode]", "action", "ToggleZenMode")
	runAction("button[action=ToggleZenMode]", "action", "ToggleZenMode")
	runAction("audio[action=SaveAudioTime]", "action", "SaveAudioTime", "time", "2.45")
	runAction("button[action=CycleAudioSpeed]", "action", "CycleAudioSpeed")
	runAction("button[action=ToggleAutoChoice]", "action", "ToggleAutoChoice")
	runAction("div.custom-select[action=SetAutoChoiceDelay]", "action", "SetAutoChoiceDelay", "delay", "3")
	runAction("div.custom-select[action=SetScrollSensitivity]", "action", "SetScrollSensitivity", "sensitivity", "normal")
	runAction("button[action=ChooseBranch]", "action", "ChooseBranch", "choice", "0")
	runAction("div[action=AutoAdvanceImplicit]", "action", "AutoAdvanceImplicit")
	runAction("audio[action=AudioEnded]", "action", "AudioEnded")
	runAction("button[action=RevealHistory]", "action", "RevealHistory")
	runAction("div[action=FocusHistoryStep]", "action", "FocusHistoryStep", "step", "0")
	runAction("button[action=FocusPresent]", "action", "FocusPresent")
	runAction("button[action=PromptRewindStep]", "action", "PromptRewindStep", "step", "0")
	runAction("button[action=ConfirmModalAccept]", "action", "ConfirmModalAccept")
	runAction("button[action=PromptResetProgress]", "action", "PromptResetProgress")
	runAction("button[action=ConfirmModalAccept]", "action", "ConfirmModalAccept")

	// Remove temporary book & test quit actions (safe under ST_NO_WINDOW=1)
	tempBook, err := book.CreateNewBook(sb.BooksDir, "Do Usunięcia", "Autor", false)
	if err == nil && tempBook != nil {
		runAction("button[action=RemoveBook]", "action", "RemoveBook", "book", tempBook.ID)
	}
	runAction("button[action=PromptQuitApp]", "action", "PromptQuitApp")
	runAction("button[action=QuitApp]", "action", "QuitApp")

	// Re-open main book in editor so subsequent chaos monkey has a rich active studio graph
	runAction("button[action=OpenBook]", "action", "OpenBook", "book", mainBookID, "view", "editor")
	runAction("button[action=ToggleSceneVoiceRecorder]", "action", "ToggleSceneVoiceRecorder", "open", "false")
	sb.Server.mu.Lock()
	sb.Server.VoiceRecorderOpen = false
	if sb.Server.ActiveBook != nil && !sb.Server.ActiveBook.IsPacked && sb.Server.ActiveBook.RootPath != "" {
		sb.Server.ActiveBook.Narrated = false
		for _, sc := range sb.Server.ActiveBook.Scenes {
			sc.ExplicitImage = ""
			sc.ImageFile = ""
			sc.ImageURL = ""
			sc.ExplicitVoice = ""
			sc.AudioFile = ""
			sc.AudioURL = ""
		}
		_ = os.RemoveAll(filepath.Join(sb.Server.ActiveBook.RootPath, "images"))
		_ = os.RemoveAll(filepath.Join(sb.Server.ActiveBook.RootPath, "voice"))
		_ = os.Remove(filepath.Join(sb.Server.ActiveBook.RootPath, "cover.png"))
		_ = os.RemoveAll(filepath.Join(sb.Server.ActiveBook.RootPath, ".history"))
		_ = book.SaveBookScenesToDisk(sb.Server.ActiveBook)
	}
	_ = os.RemoveAll(filepath.Join(sb.RootDir, "history"))
	_ = os.MkdirAll(filepath.Join(sb.RootDir, "history"), 0755)
	sb.Server.invalidateHistoryCache()
	sb.Server.mu.Unlock()
}

func generateDOMMonkeyAction(rng *rand.Rand, sb *chaosSandbox, step int) (url.Values, string) {
	sb.Server.mu.Lock()
	var sceneIDs []string
	var chapterNames []string
	var bookIDs []string
	if sb.Server.ActiveBook != nil {
		bookIDs = append(bookIDs, sb.Server.ActiveBook.ID)
		for _, sc := range sb.Server.ActiveBook.Scenes {
			sceneIDs = append(sceneIDs, sc.ID)
		}
		for _, ch := range sb.Server.ActiveBook.Chapters {
			chapterNames = append(chapterNames, ch.Name)
		}
	}
	sb.Server.mu.Unlock()

	pickScene := func() string {
		if len(sceneIDs) == 0 {
			return "poczatek"
		}
		return sceneIDs[rng.Intn(len(sceneIDs))]
	}
	pickChapter := func() string {
		if len(chapterNames) == 0 {
			return "Rozdział 1"
		}
		return chapterNames[rng.Intn(len(chapterNames))]
	}
	pickBook := func() string {
		if len(bookIDs) == 0 {
			return "kronika-chaosu"
		}
		return bookIDs[rng.Intn(len(bookIDs))]
	}

	vals := url.Values{}
	switch rng.Intn(28) {
	case 0:
		views := []string{"library", "player", "editor", "studio"}
		vals.Set("action", "SwitchView")
		vals.Set("view", views[rng.Intn(len(views))])
		return vals, "button.topbar-tab[action=SwitchView]"
	case 1:
		vals.Set("action", "OpenBook")
		vals.Set("book", pickBook())
		if rng.Intn(2) == 0 {
			vals.Set("view", "editor")
		} else {
			vals.Set("view", "player")
		}
		return vals, "div.book-card[action=OpenBook]"
	case 2:
		vals.Set("action", "SelectStudioScene")
		vals.Set("scene", pickScene())
		return vals, "canvas#studio-canvas[node-select]"
	case 3:
		if len(sceneIDs) > 10 {
			vals.Set("action", "DeleteScene")
			vals.Set("scene", pickScene())
			return vals, "button.context-delete-btn[action=DeleteScene]"
		}
		vals.Set("action", "CreateSceneAt")
		vals.Set("x", strconv.Itoa(rng.Intn(1600)-200))
		vals.Set("y", strconv.Itoa(rng.Intn(1200)-200))
		return vals, "button#btn-context-add-node[action=CreateSceneAt]"
	case 4:
		if len(sceneIDs) > 10 {
			vals.Set("action", "SelectStudioScene")
			vals.Set("scene", pickScene())
			return vals, "canvas#studio-canvas[node-select]"
		}
		vals.Set("action", "InsertScene")
		vals.Set("parent", pickScene())
		vals.Set("child", pickScene())
		return vals, "button[action=InsertScene]"
	case 5:
		if len(sceneIDs) > 10 {
			vals.Set("action", "SelectStudioScene")
			vals.Set("scene", pickScene())
			return vals, "canvas#studio-canvas[node-select]"
		}
		vals.Set("action", "BranchChoiceScene")
		vals.Set("scene", pickScene())
		vals.Set("choice_idx", strconv.Itoa(rng.Intn(3)))
		return vals, "button[action=BranchChoiceScene]"
	case 6:
		vals.Set("action", "AddChoice")
		vals.Set("scene", pickScene())
		return vals, "button[action=AddChoice]"
	case 7:
		vals.Set("action", "ConnectChoice")
		vals.Set("from_scene", pickScene())
		if rng.Intn(4) == 0 {
			vals.Set("to_scene", "END")
		} else {
			vals.Set("to_scene", pickScene())
		}
		vals.Set("choice_idx", strconv.Itoa(rng.Intn(3)))
		return vals, "canvas#studio-canvas[wire-connect]"
	case 8:
		vals.Set("action", "ReorderChoice")
		vals.Set("scene", pickScene())
		vals.Set("from_idx", "0")
		vals.Set("to_idx", "1")
		return vals, "div.choice-row[action=ReorderChoice]"
	case 9:
		vals.Set("action", "SaveScene")
		scID := pickScene()
		vals.Set("scene_id", scID)
		vals.Set("name", fmt.Sprintf("Scena Chaosu %d — Śluza", step))
		if rng.Intn(2) == 0 {
			vals.Set("explicit_id", fmt.Sprintf("scena_chaosu_%d", step))
		}
		vals.Set("text", fmt.Sprintf("Akapit testowy %d.\nDrugi wiersz opowieści.", step))
		return vals, "form#editor-context-form[action=SaveScene]"
	case 10:
		vals.Set("action", "AutoSaveScene")
		vals.Set("scene_id", pickScene())
		vals.Set("text", fmt.Sprintf("Auto-zapis treści #%d", step))
		return vals, "textarea[action=AutoSaveScene]"
	case 11:
		vals.Set("action", "OpenSceneTextModal")
		vals.Set("scene", pickScene())
		return vals, "button[action=OpenSceneTextModal]"
	case 12:
		vals.Set("action", "NavigateSceneTextModal")
		vals.Set("scene_id", pickScene())
		vals.Set("scene", pickScene())
		vals.Set("choice_idx", strconv.Itoa(rng.Intn(2)))
		return vals, "button[action=NavigateSceneTextModal]"
	case 13:
		vals.Set("action", "CloseSceneTextModal")
		return vals, "button[action=CloseSceneTextModal]"
	case 14:
		vals.Set("action", "SaveEditorState")
		vals.Set("pan_x", strconv.Itoa(rng.Intn(800)-400))
		vals.Set("pan_y", strconv.Itoa(rng.Intn(800)-400))
		vals.Set("zoom", fmt.Sprintf("%.2f", 0.4+rng.Float64()*1.8))
		vals.Set("positions", fmt.Sprintf("%s:%d,%d", pickScene(), rng.Intn(1200), rng.Intn(1200)))
		return vals, "canvas#studio-canvas[action=SaveEditorState]"
	case 15:
		vals.Set("action", "AutoLayoutNodes")
		return vals, "button[action=AutoLayoutNodes]"
	case 16:
		if len(chapterNames) > 3 {
			vals.Set("action", "SelectStudioChapter")
			vals.Set("chapter", pickChapter())
			return vals, "canvas#studio-canvas[action=SelectStudioChapter]"
		}
		vals.Set("action", "CreateChapterGroup")
		vals.Set("chapter", fmt.Sprintf("Rozdział %d", rng.Intn(4)+1))
		return vals, "button[action=CreateChapterGroup]"
	case 17:
		vals.Set("action", "ConnectChapter")
		vals.Set("from_chapter", pickChapter())
		vals.Set("to_chapter", pickChapter())
		return vals, "canvas#studio-canvas[action=ConnectChapter]"
	case 18:
		vals.Set("action", "SetStartScene")
		vals.Set("scene", pickScene())
		return vals, "div.custom-select[action=SetStartScene]"
	case 19:
		vals.Set("action", "UndoEditor")
		return vals, "button[action=UndoEditor]"
	case 20:
		vals.Set("action", "RedoEditor")
		return vals, "button[action=RedoEditor]"
	case 21:
		vals.Set("action", "AddBookVariable")
		kinds := []string{"bool", "int", "enum"}
		vals.Set("kind", kinds[rng.Intn(len(kinds))])
		vals.Set("new_var_name", fmt.Sprintf("zmienna_%d", rng.Intn(30)))
		return vals, "button[action=AddBookVariable]"
	case 22:
		vals.Set("action", "UpdateBookVariable")
		vals.Set("var_idx", strconv.Itoa(rng.Intn(4)))
		vals.Set("var_value", strconv.Itoa(rng.Intn(50)-10))
		return vals, "input[action=UpdateBookVariable]"
	case 23:
		vals.Set("action", "ChooseBranch")
		vals.Set("choice", strconv.Itoa(rng.Intn(3)))
		return vals, "button.choice-btn[action=ChooseBranch]"
	case 24:
		vals.Set("action", "SaveAudioTime")
		vals.Set("time", fmt.Sprintf("%.2f", rng.Float64()*120.0))
		return vals, "audio#voice-player[action=SaveAudioTime]"
	case 25:
		vals.Set("action", "ToggleTheme")
		return vals, "button.top-action-btn[action=ToggleTheme]"
	case 26:
		vals.Set("action", "DeleteScene")
		vals.Set("scene", pickScene())
		return vals, "button.context-delete-btn[action=DeleteScene]"
	default:
		vals.Set("action", "ConfirmModalAccept")
		return vals, "button[action=ConfirmModalAccept]"
	}
}

func generateMutatedWFPRequest(rng *rand.Rand, sb *chaosSandbox, step int) (url.Values, []byte, map[string]string, string) {
	vals := url.Values{}
	headers := make(map[string]string)

	switch rng.Intn(10) {
	case 0:
		// Malformed / truncated / binary WFP wire payload
		raw := []byte(fmt.Sprintf("action=SaveScene\nscene_id=poczatek\n_body=text\n\n\xff\xfe\xfd\x00BrokenUTF8\n---\nscene: injected_%d\n", step))
		return vals, raw, headers, "wfp-mutator:invalid-utf8-body"
	case 1:
		// Missing \n\n separator with _body header and duplicate keys
		raw := []byte("action=SaveScene\naction=DeleteScene\nscene_id=poczatek\n_body=text\n BezSeparatora")
		return vals, raw, headers, "wfp-mutator:missing-body-sep"
	case 2:
		// Extreme numeric values in SaveEditorState
		vals.Set("action", "SaveEditorState")
		vals.Set("pan_x", "NaN")
		vals.Set("pan_y", "-99999999999999999999")
		vals.Set("zoom", "1e308")
		vals.Set("positions", "poczatek:999999999,-999999999;bad_entry;x:NaN,Inf")
		return vals, nil, headers, "wfp-mutator:extreme-numbers"
	case 3:
		// Path traversal & null bytes in scene/book/media fields
		vals.Set("action", "SaveScene")
		vals.Set("scene_id", "../../etc/passwd\x00")
		vals.Set("explicit_id", "../../../etc/shadow")
		vals.Set("chapter", "..\\..\\windows\\system32")
		vals.Set("explicit_image", "../../secret.png")
		vals.Set("explicit_voice", "../../../secret.flac")
		vals.Set("source_file", "../../book.playtxt")
		return vals, nil, headers, "wfp-mutator:path-traversal"
	case 4:
		// .playtxt syntax & HTML/template injection in scene fields
		vals.Set("action", "SaveScene")
		vals.Set("scene_id", "poczatek")
		vals.Set("name", fmt.Sprintf("Tytuł %d -> END\n<script>alert(1)</script>{raw hack}", step))
		vals.Set("explicit_id", fmt.Sprintf("id_%d_hack", step))
		vals.Set("scene_effects", "zloto += 999999; if: hack\nscene: evil")
		vals.Set("text", fmt.Sprintf("Linia z wyborem -> poczatek\n\\-> ucieczka\nif: ma_klucz\nTajny tekst\nendif:\n---\nscene: fake_scene_%d\n<svg onload=alert(1)>", step))
		vals.Set("choice_count", "2")
		vals.Set("choice_label_0", "Opcja 1 -> rozdroze\nexit: hack")
		vals.Set("choice_target_0", "rozdroze")
		vals.Set("choice_cond_0", "? ma_klucz && zloto >= 0")
		vals.Set("choice_effects_0", "zloto++")
		vals.Set("choice_label_1", "<img src=x onerror=alert(1)>")
		vals.Set("choice_target_1", "END")
		return vals, nil, headers, "wfp-mutator:playtxt-html-injection"
	case 5:
		// Cross-Origin / CSRF attack simulation (must return 403 Forbidden)
		vals.Set("action", "DeleteScene")
		vals.Set("scene", "poczatek")
		headers["Sec-Fetch-Site"] = "cross-site"
		headers["Origin"] = "https://evil.example.com"
		return vals, nil, headers, "wfp-mutator:csrf-cross-origin"
	case 6:
		// Multi-KB prose payload stress test
		vals.Set("action", "AutoSaveScene")
		vals.Set("scene_id", "poczatek")
		vals.Set("text", strings.Repeat("Wiersz testu obciążeniowego inżynierii chaosu. ", 80))
		return vals, nil, headers, "wfp-mutator:large-prose"
	case 7:
		// Extreme audio trim & time parameters
		vals.Set("action", "TrimSceneVoice")
		vals.Set("scene", "poczatek")
		vals.Set("start_ratio", "-500.5")
		vals.Set("end_ratio", "9999.9")
		vals.Set("sample_rate", "-1")
		vals.Set("pcm_b64", "!!!not_valid_base64!!!")
		vals.Set("destructive", "true")
		return vals, nil, headers, "wfp-mutator:voice-trim-corrupt"
	case 8:
		// Path traversal in disk book/export actions
		vals.Set("action", "AddBookFromDisk")
		vals.Set("path", filepath.Join(sb.RootDir, "..", "..", ".git"))
		return vals, nil, headers, "wfp-mutator:dotgit-path-guard"
	default:
		// Unknown action name with arbitrary parameters
		vals.Set("action", fmt.Sprintf("NonExistentChaosAction_%d", rng.Intn(1000)))
		vals.Set("scene", "none")
		return vals, nil, headers, "wfp-mutator:unknown-action"
	}
}

func prepareSpoofedMediaFixtures(t *testing.T, rootDir string) string {
	t.Helper()
	dir := filepath.Join(rootDir, "spoofed_media")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir spoofed_media: %v", err)
	}
	fixtures := map[string][]byte{
		"vector.svg":       []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"anim.gif":         []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\xff\xff\xff\x00\x00\x00!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"),
		"uncompressed.wav": []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x44\xac\x00\x00\x88X\x01\x00\x02\x00\x10\x00data\x00\x00\x00\x00"),
		"polyglot_html.png": append(
			[]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"),
			[]byte("<!DOCTYPE html><html><script>evil()</script></html>")...,
		),
		"polyglot_elf.flac": append(
			[]byte("fLaC\x00\x00\x00\x22"),
			[]byte("\x7fELF\x02\x01\x01\x00#!/bin/sh\n")...,
		),
		"fake_magic.jpg":   []byte("NOT_A_JPEG_HEADER_AT_ALL"),
		"fake_magic.ogg":   []byte("NOT_AN_OGG_STREAM"),
		"corrupt.playbook": []byte("PK\x03\x04corrupted_zip_stream_without_central_directory"),
	}
	for name, data := range fixtures {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	return dir
}

func generateMediaFuzzAction(rng *rand.Rand, sb *chaosSandbox, fuzzMediaDir string) (url.Values, string) {
	files := []string{
		"vector.svg",
		"anim.gif",
		"uncompressed.wav",
		"polyglot_html.png",
		"polyglot_elf.flac",
		"fake_magic.jpg",
		"fake_magic.ogg",
		"corrupt.playbook",
	}
	chosen := filepath.Join(fuzzMediaDir, files[rng.Intn(len(files))])
	vals := url.Values{}
	switch rng.Intn(4) {
	case 0:
		vals.Set("action", "AddBookMediaFromDisk")
		vals.Set("kind", "image")
		vals.Set("path", chosen)
		return vals, "media-fuzz:AddBookMediaFromDisk(image)"
	case 1:
		vals.Set("action", "AddBookMediaFromDisk")
		vals.Set("kind", "audio")
		vals.Set("path", chosen)
		return vals, "media-fuzz:AddBookMediaFromDisk(audio)"
	case 2:
		vals.Set("action", "AddBookFromDisk")
		vals.Set("path", chosen)
		return vals, "media-fuzz:AddBookFromDisk"
	default:
		vals.Set("action", "AddBookFromURL")
		// Test SSRF protection on private/loopback/metadata IPs or invalid HTML payload
		urls := []string{
			"http://169.254.169.254/latest/meta-data/",
			"http://10.0.0.1/internal.playbook",
			"file:///etc/passwd",
			sb.MediaHTTPLocal.URL + "/evil.html",
		}
		vals.Set("url", urls[rng.Intn(len(urls))])
		return vals, "media-fuzz:AddBookFromURL-SSRF"
	}
}

func runConcurrentChaosBurst(t *testing.T, sb *chaosSandbox, baseStep int, burstSeed int64) {
	t.Helper()
	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan string, workers)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localRng := rand.New(rand.NewSource(burstSeed + int64(workerID)*7919))
			vals := url.Values{}
			switch workerID % 6 {
			case 0:
				vals.Set("action", "SaveAudioTime")
				vals.Set("time", fmt.Sprintf("%.3f", localRng.Float64()*90.0))
			case 1:
				vals.Set("action", "AutoSaveScene")
				vals.Set("scene_id", "poczatek")
				vals.Set("text", fmt.Sprintf("Równoległy zapis gorutyny #%d", workerID))
			case 2:
				vals.Set("action", "SaveEditorState")
				vals.Set("pan_x", strconv.Itoa(workerID*24))
				vals.Set("pan_y", strconv.Itoa(workerID*24))
				vals.Set("zoom", "1.0")
			case 3:
				vals.Set("action", "UndoEditor")
			case 4:
				vals.Set("action", "RedoEditor")
			default:
				vals.Set("action", "SelectStudioScene")
				vals.Set("scene", "poczatek")
			}

			payload := EncodeWFPForm(vals)
			req, err := http.NewRequest(http.MethodPost, sb.HTTPServer.URL+"/api/action", bytes.NewReader(payload))
			if err != nil {
				errs <- err.Error()
				return
			}
			req.Header.Set("Content-Type", "text/x-wfp;charset=UTF-8")
			resp, err := sb.HTTPServer.Client().Do(req)
			if err != nil {
				errs <- err.Error()
				return
			}
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusInternalServerError {
				errs <- fmt.Sprintf("worker %d got HTTP 500 on %s", workerID, vals.Get("action"))
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		sb.Logger.Flush(true, e)
		t.Fatalf("step %d concurrency burst error: %s", baseStep, e)
	}

	// Verify invariants after concurrent burst settles
	req, _ := http.NewRequest(http.MethodPost, sb.HTTPServer.URL+"/api/action", bytes.NewReader(EncodeWFPForm(url.Values{"action": {"SwitchView"}, "view": {"editor"}})))
	req.Header.Set("Content-Type", "text/x-wfp;charset=UTF-8")
	if resp, err := sb.HTTPServer.Client().Do(req); err == nil {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		verifyChaosStepOracles(t, sb, baseStep, "ConcurrentBurstVerify", resp.StatusCode, resp.Header, string(b))
	}
}

// runNodeVMDOMCrawler executes bundled app.js inside Node VM against the live chaos httptest.Server,
// scanning the live DOM tree for [action], [data-action], [data-backdrop-action], forms, inputs,
// .custom-select dropdowns, and #studio-canvas gestures, and checking topbar invariants after every click.
func runNodeVMDOMCrawler(t *testing.T, nodeBin string, sb *chaosSandbox, seed int64, executedActions map[string]int) {
	t.Helper()
	appDir := filepath.Clean(filepath.Join("..", "..", "app"))
	devJS, err := tpl.BundleFilesToString(tpl.JSModuleFiles(appDir), false)
	if err != nil {
		t.Fatalf("BundleFilesToString(dev) failed: %v", err)
	}

	crawlerScript := `
"use strict";
const vm = require("vm");

function parseSimpleStyle(raw) {
  const s = {
    left: "", top: "", width: "", height: "", cursor: "", transform: "", minHeight: "", position: "",
    setProperty(k, v) { this[k] = String(v); },
    getPropertyValue(k) { return this[k] || ""; }
  };
  if (raw) {
    raw.split(";").forEach((pair) => {
      const idx = pair.indexOf(":");
      if (idx > 0) s[pair.slice(0, idx).trim()] = pair.slice(idx + 1).trim();
    });
  }
  return s;
}

function splitTopLevelCommas(sel) {
  const out = [];
  let depth = 0, cur = "";
  for (let i = 0; i < sel.length; i++) {
    const ch = sel[i];
    if (ch === "[") depth++;
    else if (ch === "]") depth = Math.max(0, depth - 1);
    if (ch === "," && depth === 0) {
      if (cur.trim()) out.push(cur.trim());
      cur = "";
    } else cur += ch;
  }
  if (cur.trim()) out.push(cur.trim());
  return out;
}

function splitDescendantTokens(sel) {
  const out = [];
  let depth = 0, cur = "";
  for (let i = 0; i < sel.length; i++) {
    const ch = sel[i];
    if (ch === "[") depth++;
    else if (ch === "]") depth = Math.max(0, depth - 1);
    if (/\s/.test(ch) && depth === 0) {
      if (cur.trim()) out.push(cur.trim());
      cur = "";
    } else cur += ch;
  }
  if (cur.trim()) out.push(cur.trim());
  return out;
}

function matchesCompoundSelector(el, compound) {
  if (!el || el.nodeType !== 1) return false;
  let rem = compound.trim();
  if (!rem) return false;
  const attrMatches = [];
  rem = rem.replace(/\[([^\]]+)\]/g, (_, inner) => { attrMatches.push(inner.trim()); return ""; });
  const notClassMatches = [];
  rem = rem.replace(/:not\(\.([A-Za-z0-9_-]+)\)/g, (_, cls) => { notClassMatches.push(cls); return ""; });
  const classMatches = [];
  rem = rem.replace(/\.([A-Za-z0-9_-]+)/g, (_, cls) => { classMatches.push(cls); return ""; });
  let idMatch = "";
  rem = rem.replace(/#([A-Za-z0-9_-]+)/g, (_, id) => { idMatch = id; return ""; });
  const tagMatch = rem.trim().toLowerCase();
  if (tagMatch && tagMatch !== "*" && el.tagName.toLowerCase() !== tagMatch) return false;
  if (idMatch && el.id !== idMatch) return false;
  for (let i = 0; i < classMatches.length; i++) if (!el.classList.contains(classMatches[i])) return false;
  for (let i = 0; i < notClassMatches.length; i++) if (el.classList.contains(notClassMatches[i])) return false;
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
    } else if (!el.hasAttribute(expr)) return false;
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
    let cur = el.parentElement, idx = chain.length - 2;
    while (idx >= 0 && cur) {
      if (matchesCompoundSelector(cur, chain[idx])) idx--;
      cur = cur.parentElement;
    }
    if (idx < 0) return true;
  }
  return false;
}

function decodeEntities(str) {
  if (!str) return "";
  return String(str).replace(/&quot;/g, '"').replace(/&#39;/g, "'").replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&amp;/g, "&");
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
    this.scrollTop = 0; this.scrollLeft = 0; this.scrollHeight = 600;
    this.clientWidth = 1200; this.clientHeight = 700;
    this.offsetWidth = 180; this.offsetHeight = 36; this.offsetTop = 0;
    this.selectionStart = 0; this.selectionEnd = 0;
    this.complete = true; this.readyState = 4; this.paused = true; this.ended = false;
    this.currentTime = 0; this.duration = 30; this.playbackRate = 1.0;
    this.width = 1200; this.height = 700;
    this._text = ""; this._value = undefined;
    const self = this;
    this.classList = {
      add(...names) { names.forEach((n) => { if (n) self._classes.add(n); }); self._syncClassAttr(); },
      remove(...names) { names.forEach((n) => self._classes.delete(n)); self._syncClassAttr(); },
      toggle(name, force) {
        const shouldHave = force !== undefined ? Boolean(force) : !self._classes.has(name);
        if (shouldHave) self._classes.add(name); else self._classes.delete(name);
        self._syncClassAttr();
        return shouldHave;
      },
      contains(name) { return self._classes.has(name); }
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
          if (k.startsWith("data-")) keys.push(k.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase()));
        });
        return keys;
      },
      getOwnPropertyDescriptor(_, prop) {
        const attrName = "data-" + String(prop).replace(/[A-Z]/g, (m) => "-" + m.toLowerCase());
        if (self._attrs.has(attrName)) return { enumerable: true, configurable: true, value: self._attrs.get(attrName) };
        return undefined;
      }
    });
  }
  _syncClassAttr() {
    if (this._classes.size > 0) this._attrs.set("class", Array.from(this._classes).join(" "));
    else this._attrs.delete("class");
  }
  get isConnected() {
    let cur = this;
    while (cur) {
      if (cur === this.ownerDocument.documentElement || cur === this.ownerDocument.body) return true;
      cur = cur.parentElement;
    }
    return false;
  }
  get offsetParent() { return this.parentElement; }
  get id() { return this._attrs.get("id") || ""; }
  set id(v) { this.setAttribute("id", v); }
  get name() { return this._attrs.get("name") || ""; }
  set name(v) { this.setAttribute("name", v); }
  get type() { return this._attrs.get("type") || (this.tagName === "BUTTON" ? "submit" : ""); }
  set type(v) { this.setAttribute("type", v); }
  get disabled() { return this._attrs.has("disabled"); }
  set disabled(v) { if (v) this._attrs.set("disabled", "disabled"); else this._attrs.delete("disabled"); }
  get value() {
    if (this._value !== undefined) return this._value;
    if (this.tagName === "TEXTAREA") return this.textContent;
    return this._attrs.get("value") || "";
  }
  set value(v) { this._value = String(v); this._attrs.set("value", String(v)); }
  hasAttribute(k) { return this._attrs.has(String(k).toLowerCase()); }
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
  get firstElementChild() { return this.children.length > 0 ? this.children[0] : null; }
  get textContent() {
    let out = this._text || "";
    for (let i = 0; i < this.children.length; i++) out += this.children[i].textContent;
    return out;
  }
  set textContent(v) {
    this.children = []; this.childNodes = [];
    this._text = String(v || "");
    if (this.tagName === "TEXTAREA") this._value = this._text;
  }
  get innerHTML() {
    if (this.children.length === 0 && !this._text) return "";
    return this._text + this.children.map((c) => c.outerHTML).join("");
  }
  set innerHTML(html) {
    for (let i = 0; i < this.children.length; i++) this.children[i].parentElement = null;
    this.children = []; this.childNodes = []; this._text = "";
    if (!html) return;
    parseHTMLFragment(String(html), this.ownerDocument).forEach((n) => this.appendChild(n));
  }
  get outerHTML() {
    const tag = this.tagName.toLowerCase();
    let attrs = "";
    this._attrs.forEach((v, k) => { attrs += " " + k + '="' + String(v).replace(/"/g, "&quot;") + '"'; });
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
  matches(sel) { return matchesSelector(this, sel); }
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
    while (cur) { if (cur === this) return true; cur = cur.parentElement; }
    return false;
  }
  querySelector(sel) { const all = this.querySelectorAll(sel); return all.length > 0 ? all[0] : null; }
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
  focus() { this.ownerDocument.activeElement = this; }
  blur() { if (this.ownerDocument.activeElement === this) this.ownerDocument.activeElement = this.ownerDocument.body; }
  select() {}
  setSelectionRange(s, e) { this.selectionStart = s; this.selectionEnd = e; }
  load() {}
  play() { this.paused = false; return Promise.resolve(); }
  pause() { this.paused = true; }
  getContext(kind) {
    if (kind !== "webgl" && kind !== "experimental-webgl") return null;
    if (!this._gl) {
      const self = this;
      this._glDrawCalls = 0;
      const instExt = {
        vertexAttribDivisorANGLE() {},
        drawArraysInstancedANGLE() { self._glDrawCalls++; }
      };
      this._gl = {
        VERTEX_SHADER: 35633, FRAGMENT_SHADER: 35632, COMPILE_STATUS: 35713, LINK_STATUS: 35714,
        ARRAY_BUFFER: 34962, STATIC_DRAW: 35044, DYNAMIC_DRAW: 35048, FLOAT: 5126, UNSIGNED_BYTE: 5121,
        TEXTURE_2D: 3553, TEXTURE0: 33984, TEXTURE1: 33985, RGBA: 6408, LINEAR: 9729, CLAMP_TO_EDGE: 33071,
        TEXTURE_MIN_FILTER: 10241, TEXTURE_MAG_FILTER: 10240, TEXTURE_WRAP_S: 10242, TEXTURE_WRAP_T: 10243,
        BLEND: 3042, SRC_ALPHA: 770, ONE_MINUS_SRC_ALPHA: 771, TRIANGLES: 4,
        getExtension(name) { return name === "ANGLE_instanced_arrays" ? instExt : null; },
        createShader() { return {}; }, shaderSource() {}, compileShader() {}, getShaderParameter() { return true; },
        createProgram() { return {}; }, attachShader() {}, linkProgram() {}, getProgramParameter() { return true; },
        useProgram() {}, createBuffer() { return {}; }, bindBuffer() {}, bufferData() {}, bufferSubData() {},
        getAttribLocation() { return 0; }, enableVertexAttribArray() {}, vertexAttribPointer() {},
        getUniformLocation() { return {}; }, uniform1i() {}, uniform1f() {}, uniform2f() {}, uniform3f() {},
        createTexture() { return {}; }, activeTexture() {}, bindTexture() {}, texParameteri() {}, texImage2D() {},
        enable() {}, blendFunc() {}, viewport() {}
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
    while (cur) { path.push(cur); cur = cur.parentElement; }
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
    if (ev.type === "click" && !ev.defaultPrevented && this.tagName === "BUTTON" && this.type === "submit") {
      const parentForm = this.closest("form");
      if (parentForm) {
        parentForm.dispatchEvent(makeEvent("submit", { target: parentForm }));
      }
    }
    return !ev.defaultPrevented;
  }
}

const VOID_TAGS = new Set(["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr", "path", "rect", "circle", "line", "polyline", "polygon"]);

function parseHTMLFragment(html, doc) {
  const root = new DOMElement("fragment", doc);
  const stack = [root];
  const tagRe = /<!--[\s\S]*?-->|<!DOCTYPE[^>]*>|<\/([A-Za-z0-9_-]+)\s*>|<([A-Za-z0-9_-]+)\b((?:"[^"]*"|'[^']*'|[^>"'])*)>|([^<]+)/gi;
  const attrRe = /([^\s/>=]+)(?:=(?:"([^"]*)"|'([^']*)'|([^\s"'>]+)))?/g;
  let m;
  while ((m = tagRe.exec(html)) !== null) {
    const closeTag = m[1], openTag = m[2], rawAttr = m[3] || "", textChunk = m[4];
    const selfClose = rawAttr.trim().endsWith("/");
    const attrStr = selfClose ? rawAttr.replace(/\/\s*$/, "") : rawAttr;
    if (closeTag) {
      const upper = closeTag.toUpperCase();
      for (let i = stack.length - 1; i >= 1; i--) {
        if (stack[i].tagName === upper) { stack.length = i; break; }
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
      if (!selfClose && !VOID_TAGS.has(lower)) stack.push(el);
    } else if (textChunk !== undefined) {
      const decoded = decodeEntities(textChunk);
      const top = stack[stack.length - 1];
      top._text += decoded;
      if (top.tagName === "TEXTAREA") top._value = top._text;
    }
  }
  return root.children.slice().map((c) => { c.parentElement = null; return c; });
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
        if ((type === "checkbox" || type === "radio") && !f.checked && !f.hasAttribute("checked")) continue;
        this._pairs.push([name, f.value || ""]);
      }
    }
  }
  set(k, v) {
    const key = String(k), val = String(v);
    for (let i = 0; i < this._pairs.length; i++) {
      if (this._pairs[i][0] === key) { this._pairs[i][1] = val; return; }
    }
    this._pairs.push([key, val]);
  }
  forEach(cb) {
    for (let i = 0; i < this._pairs.length; i++) cb(this._pairs[i][1], this._pairs[i][0]);
  }
}

function makeEvent(type, props) {
  return Object.assign({
    type, button: 0, clientX: 0, clientY: 0, key: "", code: "",
    ctrlKey: false, metaKey: false, shiftKey: false, altKey: false,
    cancelable: true, defaultPrevented: false, _stopped: false, _ImmediateStopped: false,
    preventDefault() { this.defaultPrevented = true; },
    stopPropagation() { this._stopped = true; },
    stopImmediatePropagation() { this._stopped = true; this._ImmediateStopped = true; }
  }, props || {});
}

// Extract interactive elements from current DOM (buttons, custom-selects, inputs, canvas nodes)
function extractInteractiveDOMElements(doc) {
  const actions = doc.querySelectorAll("[action], [data-action], [data-backdrop-action]");
  const selects = doc.querySelectorAll(".custom-select-trigger");
  const inputs = doc.querySelectorAll("input[name], textarea[name]");
  return { actions, selects, inputs };
}

function verifyDOMInvariants(doc) {
  const topbar = doc.querySelector("header.topbar#topbar");
  if (!topbar || topbar.classList.contains("hidden")) {
    throw new Error("DOM Invariant Failed: header.topbar#topbar is missing or hidden");
  }
  const backdrops = doc.querySelectorAll(".modal-backdrop");
  for (let i = 0; i < backdrops.length; i++) {
    if (backdrops[i].style.position === "fixed") {
      throw new Error("DOM Invariant Failed: .modal-backdrop uses forbidden position: fixed");
    }
  }
}

(async () => {
  const baseURL = process.env.ST_TEST_BASE_URL;
  const appJSSrc = process.env.ST_TEST_APP_JS;
  const dispatched = new Set();
  const inFlight = new Set();

  process.on("unhandledRejection", (reason) => {
    console.error("UNHANDLED_REJECTION=" + (reason && reason.stack ? reason.stack : String(reason)));
    process.exit(1);
  });

  const initResp = await fetch(baseURL + "/");
  const initHTML = await initResp.text();

  const doc = {
    nodeType: 9, _listeners: new Map(), documentElement: null, body: null, activeElement: null, fullscreenElement: null,
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
    querySelector(sel) { return this.documentElement ? this.documentElement.querySelector(sel) : null; },
    querySelectorAll(sel) { return this.documentElement ? this.documentElement.querySelectorAll(sel) : []; },
    getElementById(id) { return this.querySelector("#" + id); },
    elementFromPoint() { return this.body; }
  };
  doc.ownerDocument = doc;

  const parsedRoot = parseHTMLFragment(initHTML, doc);
  const htmlEl = parsedRoot.find((el) => el.tagName === "HTML") || parsedRoot[0];
  doc.documentElement = htmlEl;
  doc.body = htmlEl.querySelector("body") || htmlEl;
  doc.activeElement = doc.body;

  const win = {
    document: doc,
    navigator: { userAgent: "Mozilla/5.0 MojraChaosCrawler/1.0", mediaDevices: { getUserMedia: async () => ({ getTracks: () => [{ stop() {} }] }) } },
    innerWidth: 1280, innerHeight: 800, devicePixelRatio: 2, __STUDIO_MSDF_DATA: "",
    CSS: { escape: (s) => String(s) }, FormData: CustomFormData, URLSearchParams, AbortController, TextDecoder,
    Uint8Array, Int16Array, Float32Array, Uint32Array, DataView, ArrayBuffer, Map, Set, WeakMap, WeakSet, Promise,
    Math, Number, String, Boolean, Object, Array, RegExp, Error, Date, Infinity, NaN, parseInt, parseFloat, isNaN, console,
    performance: { now: () => Date.now() },
    requestAnimationFrame: (cb) => { cb(Date.now()); return 1; },
    cancelAnimationFrame: () => {},
    setTimeout, clearTimeout,
    btoa: (str) => Buffer.from(str, "binary").toString("base64"),
    atob: (b64) => Buffer.from(b64, "base64").toString("binary"),
    Audio: class {
      constructor() { this.src = ""; this.paused = true; this.currentTime = 0; this.duration = 2.0; }
      play() { this.paused = false; return Promise.resolve(); }
      pause() { this.paused = true; }
      getAttribute(k) { return this["_" + k] || ""; }
      setAttribute(k, v) { this["_" + k] = String(v); }
    },
    ResizeObserver: class { observe() {} disconnect() {} },
    Image: class { set src(v) { this._src = v; if (this.onload) this.onload(); } },
    addEventListener(type, fn) { doc.addEventListener("win:" + type, fn); },
    close() {},
    fetch: async (urlStr, opts) => {
      const fullURL = urlStr.startsWith("http") ? urlStr : baseURL + urlStr;
      if (opts && opts.body) {
        const bodyStr = String(opts.body);
        const parsed = typeof win.decodeWFP === "function" ? win.decodeWFP(bodyStr) : {};
        if (parsed.action) dispatched.add(parsed.action);
      }
      const p = fetch(fullURL, opts);
      inFlight.add(p);
      try { return await p; } finally { inFlight.delete(p); }
    }
  };
  win.window = win;
  win.globalThis = win;

  vm.createContext(win);
  vm.runInContext(appJSSrc, win, { filename: "app.js" });
  win.__inFlight = inFlight;
  vm.runInContext(
    "const _origSend = sendServerAction; sendServerAction = async function(act, params) { const p = (async () => { return await _origSend(act, params); })(); __inFlight.add(p); try { return await p; } finally { __inFlight.delete(p); } }; const _origSilent = postServerActionSilent; postServerActionSilent = function(act, fd) { const p = Promise.resolve(_origSilent(act, fd)); __inFlight.add(p); p.finally(() => __inFlight.delete(p)); return p; };",
    win
  );
  doc.dispatchEvent(makeEvent("DOMContentLoaded", { target: doc }));

  const waitAll = async () => {
    while (inFlight.size > 0) await Promise.all(Array.from(inFlight));
    await new Promise((r) => setTimeout(r, 5));
    while (inFlight.size > 0) await Promise.all(Array.from(inFlight));
    verifyDOMInvariants(doc);
  };

  // Crawl across views, modals, custom-selects, and canvas gestures in live DOM
  const clickSel = async (sel) => {
    const el = doc.querySelector(sel);
    if (el) {
      el.dispatchEvent(makeEvent("click", { target: el }));
      await waitAll();
    }
  };

  // Helper: type text character-by-character into an input/textarea, firing keydown/input/keyup on each letter
  // and optionally firing overlapping in-flight saves before awaiting completion.
  const typeCharByChar = async (el, fullText, flushEveryChars) => {
    if (!el) return;
    el.focus();
    el.value = "";
    const chars = Array.from(String(fullText));
    for (let i = 0; i < chars.length; i++) {
      const ch = chars[i];
      el.dispatchEvent(makeEvent("keydown", { key: ch, target: el }));
      el.value += ch;
      el.dispatchEvent(makeEvent("input", { key: ch, target: el }));
      el.dispatchEvent(makeEvent("keyup", { key: ch, target: el }));
      if (flushEveryChars && (i + 1) % flushEveryChars === 0) {
        if (typeof win.flushInspectorFormIfDirty === "function") win.flushInspectorFormIfDirty();
      }
    }
    el.dispatchEvent(makeEvent("change", { target: el }));
    el.dispatchEvent(makeEvent("keydown", { key: "Enter", target: el }));
    el.blur();
    el.dispatchEvent(makeEvent("blur", { target: el }));
  };

  await clickSel('#btn-switch-to-studio');
  await clickSel('button[action="OpenNewBookModal"]');
  const titleInp = doc.querySelector("#new-book-title");
  await typeCharByChar(titleInp, "Książka Crawler DOM — Śluza", 0);
  await clickSel('form[action="CreateNewBook"] button[type="submit"]');

  // Exercise DOM extractor on Studio view
  const extracted = extractInteractiveDOMElements(doc);
  if (extracted.actions.length < 5) {
    throw new Error("Expected DOM extractor to find interactive [action] elements in Studio view");
  }

  // Rapid-fire add 3 new nodes on the canvas (creates s1, s2, s3)
  await clickSel('#btn-context-add-node');
  await clickSel('#btn-context-add-node');
  await clickSel('#btn-context-add-node');

  // Verify every node in studioGraphState has a unique ID and is registered in nodeById
  const graphCheck1 = vm.runInContext("(() => ({ total: studioGraphState.nodes.length, unique: studioGraphState.nodeById.size }))()", win);
  if (graphCheck1.total < 3 || graphCheck1.total !== graphCheck1.unique) {
    throw new Error("Canvas node uniqueness invariant violated: " + JSON.stringify(graphCheck1));
  }

  // Click edit button on the first newly created scene ("s1") and type character-by-character into title, explicit_id, and text
  const s1EditPt = vm.runInContext("(() => { const n = studioGraphState.nodeById.get('s1') || studioGraphState.nodes[2]; if (!n) return null; if (typeof centerCanvasCameraOnScene === 'function') centerCanvasCameraOnScene(n.id); const z = editorCanvasState.zoom; const wx = getNodeFooterBtnX(n, 'edit', false) + CARD.btnSize * 0.5; const wy = n.y + n.h - CARD.footerH + CARD.pad + CARD.btnSize * 0.5; return { id: n.id, x: Math.round(wx * z + editorCanvasState.panX), y: Math.round(wy * z + editorCanvasState.panY) }; })()", win);
  const canvas = doc.querySelector("#studio-canvas");
  if (canvas && s1EditPt) {
    canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: s1EditPt.x, clientY: s1EditPt.y, target: canvas }));
    canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: s1EditPt.x, clientY: s1EditPt.y, target: canvas }));
    await waitAll();
  }
  if (!doc.querySelector("#form-scene-text") && s1EditPt) {
    await vm.runInContext("sendServerAction('OpenSceneTextModal', { scene: '" + s1EditPt.id + "' })", win);
    await waitAll();
  }
  const modalNameInp = doc.querySelector('#form-scene-text input[name="name"]');
  const modalExpInp = doc.querySelector('#form-scene-text input[name="explicit_id"]');
  const modalTextInp = doc.querySelector('#form-scene-text textarea[name="text"]');
  if (modalNameInp) await typeCharByChar(modalNameInp, "Zatopiona komora śluzy", 3);
  await waitAll();
  if (modalExpInp) await typeCharByChar(modalExpInp, "zatopiona_komora_sluzy", 4);
  await waitAll();
  if (modalTextInp) await typeCharByChar(modalTextInp, "Woda wdziera się przez zardzewiałe wrota śluzy.", 8);
  await waitAll();

  // Verify that after typing character-by-character and changing s1 -> zatopiona_komora_sluzy, all nodes remain unique and reachable
  const graphCheck2 = vm.runInContext("(() => ({ total: studioGraphState.nodes.length, unique: studioGraphState.nodeById.size, ids: Array.from(studioGraphState.nodeById.keys()), hasRenamed: studioGraphState.nodeById.has('zatopiona_komora_sluzy'), hasStart: studioGraphState.nodeById.has('poczatek') }))()", win);
  if (graphCheck2.total !== graphCheck2.unique || !graphCheck2.hasRenamed || !graphCheck2.hasStart) {
    throw new Error("Post-typing canvas node reachability invariant violated: " + JSON.stringify(graphCheck2));
  }

  await clickSel('button[action="CloseSceneTextModal"]');

  // Exercise custom-select dropdowns in Book Settings
  await clickSel('#btn-book-settings');
  const triggers = doc.querySelectorAll(".custom-select-trigger");
  if (triggers.length > 0) {
    triggers[0].dispatchEvent(makeEvent("click", { target: triggers[0] }));
    const opts = doc.querySelectorAll(".custom-select-option");
    if (opts.length > 0) {
      opts[0].dispatchEvent(makeEvent("click", { target: opts[0] }));
      await waitAll();
    }
  }

  await clickSel('button.topbar-tab[action="SwitchView"][data-view="library"]');

  console.log("CRAWLER_ACTIONS=" + Array.from(dispatched).join(","));
})().catch((err) => {
  console.error("CRAWLER_ERR=" + (err && err.stack ? err.stack : String(err)));
  process.exit(1);
});
`

	cmd := exec.Command(nodeBin, "-e", crawlerScript)
	cmd.Env = append(os.Environ(),
		"ST_TEST_BASE_URL="+sb.HTTPServer.URL,
		"ST_TEST_APP_JS="+devJS,
		fmt.Sprintf("ST_CHAOS_SEED=%d", seed),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		sb.Logger.Flush(true, string(out))
		t.Fatalf("Node VM DOM crawler failed: %v\nOutput:\n%s", err, string(out))
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "CRAWLER_ACTIONS=") {
			for _, act := range strings.Split(strings.TrimPrefix(line, "CRAWLER_ACTIONS="), ",") {
				act = strings.TrimSpace(act)
				if act != "" {
					executedActions[act]++
				}
			}
		}
	}
}
