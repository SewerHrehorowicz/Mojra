package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTelemetryDisabledByDefaultAndLogsWhenEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	t0 := time.Now().Add(-5 * time.Millisecond)

	Init(tmpDir, t0, false)
	if Active.Enabled {
		t.Fatalf("expected telemetry to remain disabled when flagEnabled=false and env/build flags are unset")
	}
	Active.MarkStep(StageResolveRoot, t0, tmpDir)
	Active.MarkAssetOnce("style.css", t0)
	if _, err := os.Stat(filepath.Join(tmpDir, telemetryLogFileName)); !os.IsNotExist(err) {
		t.Fatalf("expected no %s file when disabled, got err=%v", telemetryLogFileName, err)
	}

	Init(tmpDir, t0, true)
	t.Cleanup(func() {
		Init(tmpDir, time.Time{}, false)
	})
	if !Active.Enabled {
		t.Fatalf("expected telemetry to be enabled when flagEnabled=true")
	}

	tStep := time.Now().Add(-2 * time.Millisecond)
	Active.MarkStep(StageResolveRoot, tStep, tmpDir)
	Active.MarkStep(StageCocoaAppLaunch, time.Time{}, "http://127.0.0.1:8484")
	Active.MarkAssetOnce("style.css", tStep)
	Active.MarkAssetOnce("style.css", tStep)

	raw, err := os.ReadFile(filepath.Join(tmpDir, telemetryLogFileName))
	if err != nil {
		t.Fatalf("failed to read %s: %v", telemetryLogFileName, err)
	}
	content := string(raw)
	for _, want := range []string{
		headerPrefix,
		string(StageProcessStart),
		string(StageResolveRoot),
		string(StageCocoaAppLaunch),
		string(StageHTTPAssetServe),
		stepPrefix,
		deltaPrefix,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected %q in telemetry log, got:\n%s", want, content)
		}
	}
	if got := strings.Count(content, string(StageHTTPAssetServe)); got != 1 {
		t.Fatalf("expected MarkAssetOnce to log style.css once, got %d occurrences:\n%s", got, content)
	}
}
