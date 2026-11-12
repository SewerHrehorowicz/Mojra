package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAndRenderProjectGraph(t *testing.T) {
	rootDir := filepath.Clean(filepath.Join("..", ".."))
	tmpDir := t.TempDir()
	outTxt := filepath.Join(tmpDir, "project.graph")
	outHTML := filepath.Join(tmpDir, "project_graph.html")
	tplDir := filepath.Join(rootDir, defaultTplDirRel)

	cfg := Config{
		RootDir:     rootDir,
		FilterDirs:  []string{"cmd/projgraph", "internal/tpl"},
		OutTxtPath:  outTxt,
		OutHTMLPath: outHTML,
	}

	g, err := BuildGraph(cfg)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}
	if g.TotalDirs != 2 {
		t.Fatalf("expected 2 dirs for filter [cmd/projgraph, internal/tpl], got %d", g.TotalDirs)
	}
	if g.TotalFiles == 0 || g.TotalSymbols == 0 {
		t.Fatalf("expected non-empty files/symbols, got files=%d symbols=%d", g.TotalFiles, g.TotalSymbols)
	}

	if err := WriteTextGraph(outTxt, g); err != nil {
		t.Fatalf("WriteTextGraph failed: %v", err)
	}
	txtRaw, err := os.ReadFile(outTxt)
	if err != nil {
		t.Fatalf("read text graph failed: %v", err)
	}
	txtStr := string(txtRaw)
	for _, needle := range []string{
		"dir cmd/projgraph",
		"file cmd/projgraph/graph_builder.go",
		"func BuildGraph",
		"-> calls: internal/tpl/LoadTemplatesFromDir",
	} {
		if !strings.Contains(txtStr, needle) {
			t.Errorf("expected text graph to contain %q", needle)
		}
	}

	if err := WriteHTMLGraph(tplDir, outHTML, g); err != nil {
		t.Fatalf("WriteHTMLGraph failed: %v", err)
	}
	htmlRaw, err := os.ReadFile(outHTML)
	if err != nil {
		t.Fatalf("read html graph failed: %v", err)
	}
	htmlStr := string(htmlRaw)
	for _, needle := range []string{
		"<style>",
		`<details class="node-branch tree-dir"`,
		`id="cmd-projgraph-BuildGraph"`,
		`href="#internal-tpl-LoadTemplatesFromDir"`,
	} {
		if !strings.Contains(htmlStr, needle) {
			t.Errorf("expected HTML graph to contain %q", needle)
		}
	}
	if strings.Contains(htmlStr, cssHrefPlaceholder) {
		t.Errorf("expected %s to be replaced by inlined <style>", cssHrefPlaceholder)
	}
}
