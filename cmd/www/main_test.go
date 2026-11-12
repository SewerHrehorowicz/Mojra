package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWWW(t *testing.T) {
	rootDir := filepath.Clean(filepath.Join("..", ".."))
	if err := buildSite(rootDir); err != nil {
		t.Fatalf("buildSite error: %v", err)
	}
	outDir := filepath.Join(rootDir, "dist", "www")
	for _, name := range []string{"index.html", "inwestorzy.html", "style.css", "app.js"} {
		data, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("missing generated file %s: %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("empty generated file %s", name)
		}
	}
	indexHTML, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(indexHTML), `src="app.js"`) {
		t.Fatalf("expected relative app.js script tag in dist/www/index.html")
	}
}
