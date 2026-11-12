package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCodebaseAudit(t *testing.T) {
	rootDir := filepath.Clean(filepath.Join("..", ".."))
	filter := os.Getenv("ST_AUDIT_FILTER")
	lang := os.Getenv("ST_AUDIT_LANG")
	sev := os.Getenv("ST_AUDIT_SEVERITY")

	if os.Getenv("ST_AUDIT_MAGIC") == "1" {
		magicFindings, err := RunMagic(rootDir)
		if err != nil {
			t.Fatalf("audit.RunMagic failed: %v", err)
		}
		var magicBuf bytes.Buffer
		PrintMagicReportWithLang(&magicBuf, magicFindings, filter, lang, sev)
		_, _ = WriteAuditLog(rootDir, "audit_strings.log", magicBuf.Bytes())
		t.Log("\n" + magicBuf.String())
		return
	}

	if os.Getenv("ST_AUDIT_ARCH") == "1" {
		archFindings, err := RunArch(rootDir)
		if err != nil {
			t.Fatalf("audit.RunArch failed: %v", err)
		}
		var archBuf bytes.Buffer
		PrintArchReportWithLang(&archBuf, archFindings, filter, lang, sev)
		_, _ = WriteAuditLog(rootDir, "audit_arch.log", archBuf.Bytes())
		t.Log("\n" + archBuf.String())
		return
	}

	findings, err := Run(rootDir)
	if err != nil {
		t.Fatalf("audit.Run failed: %v", err)
	}

	var buf bytes.Buffer
	count := PrintReportWithOptions(&buf, findings, filter, lang, sev)
	_, _ = WriteAuditLog(rootDir, "audit_static.log", buf.Bytes())
	t.Log("\n" + buf.String())

	if archFindings, err := RunArch(rootDir); err == nil {
		var archBuf bytes.Buffer
		PrintArchReportWithLang(&archBuf, archFindings, filter, lang, sev)
		_, _ = WriteAuditLog(rootDir, "audit_arch.log", archBuf.Bytes())
	}
	if magicFindings, err := RunMagic(rootDir); err == nil {
		var magicBuf bytes.Buffer
		PrintMagicReportWithLang(&magicBuf, magicFindings, filter, lang, sev)
		_, _ = WriteAuditLog(rootDir, "audit_strings.log", magicBuf.Bytes())
	}

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
