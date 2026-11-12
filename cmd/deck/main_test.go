package main

import (
	"path/filepath"
	"testing"
)

func TestBuildDeckPDF(t *testing.T) {
	rootDir := filepath.Clean(filepath.Join("..", ".."))
	if err := buildDeckHTML(rootDir); err != nil {
		t.Fatalf("buildDeckHTML error: %v", err)
	}
	if FindHeadlessPDFBrowser() != "" {
		if _, err := BuildDeckPDF(rootDir, ""); err != nil {
			t.Fatalf("BuildDeckPDF error: %v", err)
		}
	}
}
