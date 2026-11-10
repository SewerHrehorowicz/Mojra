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

	if count > 0 {
		t.Fatalf("wykryto %d problemów w audycie statycznym:\n%s", count, buf.String())
	}
}

func TestAuditRejectsMoreThanFourCommaSelectorsInTSS(t *testing.T) {
	validSrc := `
.btn-a, .btn-b, .btn-c, .btn-d
  color --text-primary
`
	if got := auditTSSSourceSelectorLists("app/styles/test.tss", validSrc); len(got) != 0 {
		t.Fatalf("expected 0 findings for 4 selectors, got %d: %v", len(got), got)
	}

	invalidMultiline := `
.btn-1::before,
.btn-2::before,
.btn-3::before,
.btn-4::before,
.btn-5::before
  content ""
`
	got := auditTSSSourceSelectorLists("app/styles/test.tss", invalidMultiline)
	if len(got) != 1 || got[0].Category != "TPL_ARCH" {
		t.Fatalf("expected 1 TPL_ARCH finding for >4 comma-separated selectors, got: %v", got)
	}
}
