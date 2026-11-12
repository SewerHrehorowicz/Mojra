package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mojra/internal/tpl"
)

// WriteTextGraph formats and writes the plain-text AI dependency graph to outPath.
func WriteTextGraph(outPath string, g GraphView) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.Grow(64 * 1024)
	buf.WriteString(hdrPrefix)
	buf.WriteByte('\n')
	buf.WriteString(hdrScope)
	buf.WriteString(g.Scope)
	buf.WriteByte('\n')
	fmt.Fprintf(&buf, hdrStatsFmt, g.TotalDirs, g.TotalFiles, g.TotalSymbols)

	for _, dir := range g.Dirs {
		fmt.Fprintf(&buf, lineDirFmt, dir.Path)
		for _, file := range dir.Files {
			fmt.Fprintf(&buf, lineFileFmt, file.RelPath)
			for _, sym := range file.Symbols {
				fmt.Fprintf(&buf, lineSymFmt, sym.Kind, sym.Name, sym.LineRange, sym.ID)
				for _, f := range sym.Fields {
					fmt.Fprintf(&buf, lineFieldFmt, f)
				}
				for _, out := range sym.OutEdges {
					fmt.Fprintf(&buf, lineOutEdgeFmt, out.Kind, out.TargetLabel, out.TargetID)
				}
				for _, in := range sym.InEdges {
					fmt.Fprintf(&buf, lineInEdgeFmt, in.Kind, in.TargetLabel, in.TargetID)
				}
			}
		}
	}

	return os.WriteFile(outPath, buf.Bytes(), 0644)
}

// WriteHTMLGraph compiles graph.tss and graph.tpl via internal/tpl and writes a self-contained HTML file to outPath.
func WriteHTMLGraph(tplDir, outPath string, g GraphView) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}

	tssPath := filepath.Join(tplDir, tssFileNameGraph)
	cssStr, err := tpl.TranspileCSSFilesToString([]string{tssPath}, true, nil)
	if err != nil {
		return err
	}

	templates, err := tpl.LoadTemplatesFromDir(tplDir, nil)
	if err != nil {
		return err
	}

	ctx := tpl.NewContext(nil).WithStruct(&g)
	var htmlBuf bytes.Buffer
	htmlBuf.Grow(128 * 1024)
	if err := tpl.RenderTemplateTo(&htmlBuf, tplNameGraph, templates, ctx); err != nil {
		return err
	}

	inlinedStyle := styleOpenTag + cssStr + styleCloseTag
	finalHTML := strings.Replace(htmlBuf.String(), cssHrefPlaceholder, inlinedStyle, 1)
	return os.WriteFile(outPath, []byte(finalHTML), 0644)
}
