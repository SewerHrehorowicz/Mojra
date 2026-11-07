package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCodebaseAudit(t *testing.T) {
	rootDir := filepath.Clean(filepath.Join("..", ".."))
	findings, err := Run(rootDir)
	if err != nil {
		t.Fatalf("audit.Run failed: %v", err)
	}

	var buf bytes.Buffer
	filter := os.Getenv("ST_AUDIT_FILTER")
	lang := os.Getenv("ST_AUDIT_LANG")
	count := PrintReportWithLang(&buf, findings, filter, lang)
	t.Log("\n" + buf.String())

	if os.Getenv("ST_AUDIT_STRICT") == "1" && count > 0 {
		t.Fatalf("tryb ST_AUDIT_STRICT=1: wykryto %d problemów w audycie statycznym", count)
	}
}
