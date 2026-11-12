package logging

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Stage identifies a measured startup, HTTP, Cocoa, action, or view rendering phase.
type Stage string

const (
	telemetryLogFileName = "telemetry.log"
	envTelemetry         = "ST_TELEMETRY"
	boolTrue             = "true"
	headerPrefix         = "=== MOJRA TELEMETRY SESSION "
	headerSuffix         = " ===\n"
	unitMillis           = "ms"
	deltaPrefix          = " (d="
	stepPrefix           = " (step="
	stageOpen            = " ["
	stageClose           = "] "

	StageProcessStart     Stage = "boot.process_start"
	StageResolveRoot      Stage = "boot.resolve_root"
	StageLoadSettings     Stage = "boot.load_settings"
	StageLoadTemplates    Stage = "boot.load_templates"
	StageLoadLibrary      Stage = "boot.load_library"
	StageLoadBookItem     Stage = "boot.load_book_item"
	StageInitPlayerState  Stage = "boot.init_player_state"
	StageInitEditorState  Stage = "boot.init_editor_state"
	StageStopPrevPID      Stage = "boot.stop_prev_pid"
	StageNetListen        Stage = "boot.net_listen"
	StageDetectLAN        Stage = "boot.detect_lan"
	StageHTTPIndexRender  Stage = "http.index_render"
	StageHTTPAssetServe   Stage = "http.asset_serve"
	StageCocoaAppLaunch   Stage = "cocoa.app_launch"
	StageCocoaIconDraw    Stage = "cocoa.icon_draw"
	StageCocoaWebViewInit Stage = "cocoa.webview_init"
	StageCocoaLoadRequest Stage = "cocoa.load_request"
	StageCocoaNavCommit   Stage = "cocoa.nav_commit"
	StageCocoaNavFinish   Stage = "cocoa.nav_finish"

	StageActionHandler         Stage = "action.handler"
	StageActionSaveSettings    Stage = "action.save_settings"
	StageActionBuildView       Stage = "action.build_view"
	StageActionRenderTpl       Stage = "action.render_tpl"
	StageActionTotal           Stage = "action.total"
	StageViewLibrary           Stage = "view.library"
	StageStudioHomePackedScan  Stage = "studio.home_packed_scan"
	StageEditorSelectedNode    Stage = "studio.selected_node"
	StageEditorGraphPayload    Stage = "studio.graph_payload"
	StageEditorSearchCtx       Stage = "studio.search_ctx"
	StageEditorBreakdownCtx    Stage = "studio.breakdown_ctx"
	StageEditorHistoryDisk     Stage = "studio.history_disk"
	StageEditorHistoryGraph    Stage = "studio.history_graph"
)

// buildEnabled can be set at link time via -ldflags="-X mojra/internal/logging.buildEnabled=true".
var buildEnabled string

// TelemetryLogger writes low-overhead phase and step timings to saves/telemetry.log when enabled.
type TelemetryLogger struct {
	Enabled      bool
	mu           sync.Mutex
	file         *os.File
	buf          []byte
	t0           time.Time
	prev         time.Time
	loggedAssets map[string]bool
}

// Active is the process-wide telemetry logger instance.
var Active TelemetryLogger

// Init initializes Active when flagEnabled, buildEnabled, or ST_TELEMETRY=1 is active.
func Init(savesDir string, startTime time.Time, flagEnabled bool) {
	envVal := os.Getenv(envTelemetry)
	enabled := flagEnabled || buildEnabled == boolTrue || envVal == "1" || envVal == boolTrue
	if !enabled || savesDir == "" {
		Active.Enabled = false
		return
	}
	if startTime.IsZero() {
		startTime = time.Now()
	}
	_ = os.MkdirAll(savesDir, 0755)
	logPath := filepath.Join(savesDir, telemetryLogFileName)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		Active.Enabled = false
		return
	}

	Active.mu.Lock()
	defer Active.mu.Unlock()
	if Active.file != nil {
		_ = Active.file.Close()
	}
	Active.file = f
	Active.t0 = startTime
	Active.prev = startTime
	Active.loggedAssets = make(map[string]bool, 32)
	Active.Enabled = true
	Active.buf = append(Active.buf[:0], headerPrefix...)
	Active.buf = startTime.AppendFormat(Active.buf, time.RFC3339Nano)
	Active.buf = append(Active.buf, headerSuffix...)
	_, _ = f.Write(Active.buf)
	Active.writeLocked(StageProcessStart, time.Time{}, savesDir)
}

func (l *TelemetryLogger) writeLocked(stage Stage, stepStart time.Time, detail string) {
	if l.file == nil {
		return
	}
	now := time.Now()
	ref := stepStart
	prefix := stepPrefix
	if ref.IsZero() {
		ref = l.prev
		prefix = deltaPrefix
	}
	totalMs := float64(now.Sub(l.t0).Nanoseconds()) / 1e6
	stepMs := float64(now.Sub(ref).Nanoseconds()) / 1e6
	l.prev = now
	l.buf = append(l.buf[:0], '+')
	l.buf = strconv.AppendFloat(l.buf, totalMs, 'f', 2, 64)
	l.buf = append(l.buf, unitMillis...)
	l.buf = append(l.buf, prefix...)
	l.buf = strconv.AppendFloat(l.buf, stepMs, 'f', 2, 64)
	l.buf = append(l.buf, unitMillis...)
	l.buf = append(l.buf, ')')
	l.buf = append(l.buf, stageOpen...)
	l.buf = append(l.buf, stage...)
	l.buf = append(l.buf, stageClose...)
	l.buf = append(l.buf, detail...)
	l.buf = append(l.buf, '\n')
	_, _ = l.file.Write(l.buf)
}

// MarkStep records the duration since stepStart (or delta since the previous mark when stepStart.IsZero())
// along with total elapsed time since process start.
func (l *TelemetryLogger) MarkStep(stage Stage, stepStart time.Time, detail string) {
	if l == nil || !l.Enabled {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writeLocked(stage, stepStart, detail)
}

// MarkAssetOnce logs the first time a given static UI asset is served over HTTP.
func (l *TelemetryLogger) MarkAssetOnce(assetPath string, stepStart time.Time) {
	if l == nil || !l.Enabled {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.loggedAssets == nil {
		l.loggedAssets = make(map[string]bool, 32)
	}
	if l.loggedAssets[assetPath] {
		return
	}
	l.loggedAssets[assetPath] = true
	l.writeLocked(StageHTTPAssetServe, stepStart, assetPath)
}
