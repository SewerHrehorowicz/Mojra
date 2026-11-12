package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var dirFlag string
	var outTxtFlag string
	var outHTMLFlag string

	flag.StringVar(&dirFlag, flagDirName, "", flagDirDesc)
	flag.StringVar(&outTxtFlag, flagOutTxtName, defaultOutTxtRel, flagOutTxtDesc)
	flag.StringVar(&outHTMLFlag, flagOutHTMLName, defaultOutHTMLRel, flagOutHTMLDesc)
	flag.Parse()

	rootDir, err := os.Getwd()
	if err != nil {
		rootDir = "."
	}
	for step := 0; step < 6; step++ {
		if info, statErr := os.Stat(filepath.Join(rootDir, goModFile)); statErr == nil && !info.IsDir() {
			break
		}
		parent := filepath.Dir(rootDir)
		if parent == rootDir {
			break
		}
		rootDir = parent
	}

	var filters []string
	if dirFlag != "" {
		for _, part := range strings.Split(dirFlag, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				filters = append(filters, filepath.ToSlash(trimmed))
			}
		}
	}
	for _, arg := range flag.Args() {
		trimmed := strings.TrimSpace(arg)
		if trimmed != "" {
			filters = append(filters, filepath.ToSlash(trimmed))
		}
	}

	outTxtPath := outTxtFlag
	if !filepath.IsAbs(outTxtPath) {
		outTxtPath = filepath.Join(rootDir, outTxtPath)
	}
	outHTMLPath := outHTMLFlag
	if !filepath.IsAbs(outHTMLPath) {
		outHTMLPath = filepath.Join(rootDir, outHTMLPath)
	}
	tplDir := filepath.Join(rootDir, defaultTplDirRel)

	cfg := Config{
		RootDir:     rootDir,
		FilterDirs:  filters,
		OutTxtPath:  outTxtPath,
		OutHTMLPath: outHTMLPath,
	}

	graph, err := BuildGraph(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, errBuildGraphFmt, err)
		os.Exit(1)
	}

	if err := WriteTextGraph(cfg.OutTxtPath, graph); err != nil {
		fmt.Fprintf(os.Stderr, errWriteTxtFmt, err)
		os.Exit(1)
	}

	if err := WriteHTMLGraph(tplDir, cfg.OutHTMLPath, graph); err != nil {
		fmt.Fprintf(os.Stderr, errWriteHTMLFmt, err)
		os.Exit(1)
	}
}
