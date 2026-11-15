package templates

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	modulesCfgFileName     = "modules.cfg"
	uiComponentsDirName    = "ui_components"
	templatesDirName       = "templates"
	stylesDirName          = "styles"
	scriptsDirName         = "scripts"
	controllersDirName     = "controllers"
	iconsDirName           = "icons"
	defaultShellEntryTpl   = "shell_view"
	outIndexHTMLName       = "index.html"
	outStyleCSSName        = "style.css"
	outAppJSName           = "app.js"
	tplExtSuffix           = ".tpl"
	tssExtSuffix           = ".tss"
	jsExtSuffix            = ".js"
	useStrictHeader        = "\"use strict\";\n\n"
	useStrictDouble        = `"use strict";`
	useStrictSingle        = `'use strict';`
	devOnlyCSSStart        = "/* @dev-only-start */"
	devOnlyCSSEnd          = "/* @dev-only-end */"
	devOnlyJSStart         = "// @dev-only-start"
	devOnlyJSEnd           = "// @dev-only-end"
	priorityColorsTSS      = "colors.tss"
	priorityBaseTSS        = "base.tss"
	priorityFormsTSS       = "forms.tss"
	engineScriptFDP        = "scripts/fdp.js"
	engineScriptHTTP       = "scripts/transport_http.js"
	engineScriptWasm       = "scripts/transport_wasm.js"
	engineScriptDispatcher = "scripts/dispatcher.js"
	errReadModulesCfgFmt   = "read modules.cfg: %w"
	errReadStyleFmt        = "read style %s: %w"
	errReadScriptFmt       = "read script %s: %w"
	errMkdirOutFmt         = "create output dir %s: %w"
	errEntryTplMissingFmt  = "entry template %q not found"
	ctxKeyTheme            = "Theme"
	ctxValDark             = "dark"
	ctxKeyUIScale          = "UIScale"
	ctxValSmall            = "small"
	ctxKeyImagelessAttr    = "ImagelessAttr"
	ctxKeyLockedAttr       = "ImagelessLockedAttr"
	ctxKeyZenAttr          = "ZenModeAttr"
	ctxValFalse            = "false"
	ctxKeySensitivity      = "ScrollSensitivity"
	ctxValMedium           = "medium"
	ctxKeyPreviewMode      = "PreviewMode"
	ctxValFull             = "full"
	ctxKeyDevMode          = "DevMode"
	ctxKeyIsDocRoot        = "IsDocumentRoot"
	ctxKeyLang             = "lang"
	ctxValPL               = "pl"
	devModeExpr            = "dev_mode"
	notDevModeExpr         = "not dev_mode"
	bangDevModeExpr        = "!dev_mode"
	devModeCapExpr         = "DevMode"
	notDevModeCapExpr      = "not DevMode"
	bangDevModeCapExpr     = "!DevMode"
)

// StripDevNodesFromTemplates removes "if dev_mode" / "if DevMode" nodes and unwraps "if not dev_mode" control nodes in-place.
func StripDevNodesFromTemplates(loadedTemplates map[string]*ParsedTemplate) {
	for _, parsedTpl := range loadedTemplates {
		if parsedTpl != nil {
			parsedTpl.Nodes = stripDevNodesSlice(parsedTpl.Nodes)
		}
	}
}

func stripDevNodesSlice(rootNodes []*Node) []*Node {
	out := filterDevNodesLevel(rootNodes)
	stack := make([]*[]*Node, 0, 32)
	for idx := range out {
		if len(out[idx].Children) > 0 {
			stack = append(stack, &out[idx].Children)
		}
	}
	for step := 0; step < 8192 && len(stack) > 0; step++ {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		filtered := filterDevNodesLevel(*top)
		*top = filtered
		for idx := range filtered {
			if len(filtered[idx].Children) > 0 {
				stack = append(stack, &filtered[idx].Children)
			}
		}
	}
	return out
}

func filterDevNodesLevel(nodes []*Node) []*Node {
	cur := nodes
	for pass := 0; pass < 8; pass++ {
		var out []*Node
		unwrapped := false
		for idx := 0; idx < len(cur); idx++ {
			nodeItem := cur[idx]
			expr := strings.TrimSpace(nodeItem.Expr)
			if nodeItem.Kind == NodeIf && (strings.EqualFold(expr, devModeExpr) || expr == devModeCapExpr) {
				if idx+1 < len(cur) {
					next := cur[idx+1]
					if next.Kind == NodeElse {
						idx++
						out = append(out, next.Children...)
						unwrapped = true
					} else if next.Kind == NodeElif {
						next.Kind = NodeIf
					}
				}
				continue
			}
			if nodeItem.Kind == NodeIf && (strings.EqualFold(expr, notDevModeExpr) || strings.EqualFold(expr, bangDevModeExpr) || expr == notDevModeCapExpr || expr == bangDevModeCapExpr) {
				out = append(out, nodeItem.Children...)
				unwrapped = true
				for idx+1 < len(cur) && (cur[idx+1].Kind == NodeElif || cur[idx+1].Kind == NodeElse) {
					idx++
				}
				continue
			}
			out = append(out, nodeItem)
		}
		cur = out
		if !unwrapped {
			break
		}
	}
	return cur
}

// I18nScriptGenerator provides the optional client-side I18n dictionary JS preamble.
var I18nScriptGenerator func() string

var engineCoreScriptFiles = [...]string{
	engineScriptFDP,
	engineScriptHTTP,
	engineScriptWasm,
	engineScriptDispatcher,
}

// ParseModulesCfg parses a modules.cfg text payload into an ordered list of module paths.
func ParseModulesCfg(rawCfg string) []string {
	var modules []string
	seen := make(map[string]bool)
	for _, rawLine := range strings.Split(strings.ReplaceAll(rawCfg, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if commentIdx := strings.IndexAny(line, "#;"); commentIdx >= 0 {
			line = strings.TrimSpace(line[:commentIdx])
		}
		for _, part := range strings.Split(line, ",") {
			modName := strings.Trim(strings.TrimSpace(part), "/")
			if modName != "" && !seen[modName] {
				seen[modName] = true
				modules = append(modules, modName)
			}
		}
	}
	return modules
}

// ReadModulesCfg reads and parses modules.cfg from the given shell filesystem.
func ReadModulesCfg(shellFS fs.FS) ([]string, error) {
	rawBytes, readErr := fs.ReadFile(shellFS, modulesCfgFileName)
	if readErr != nil {
		return nil, fmt.Errorf(errReadModulesCfgFmt, readErr)
	}
	return ParseModulesCfg(string(rawBytes)), nil
}

// ResolveModuleFSList resolves the ordered []fs.FS slice from a shell's modules.cfg and a module registry, appending shellFS last.
func ResolveModuleFSList(shellFS fs.FS, registry map[string]fs.FS) ([]fs.FS, []string, error) {
	modNames, cfgErr := ReadModulesCfg(shellFS)
	if cfgErr != nil {
		return nil, nil, cfgErr
	}
	resolved := make([]fs.FS, 0, len(modNames)+1)
	for _, modName := range modNames {
		if modFS, found := registry[modName]; found && modFS != nil {
			resolved = append(resolved, modFS)
		}
	}
	if shellFS != nil {
		resolved = append(resolved, shellFS)
	}
	return resolved, modNames, nil
}

type multiIconsFS struct {
	modules []fs.FS
}

func (composite *multiIconsFS) Open(name string) (fs.File, error) {
	return openFromModulesFS(composite.modules, name)
}

func openFromModulesFS(modules []fs.FS, name string) (fs.File, error) {
	cleanName := strings.TrimPrefix(path.Clean(name), "/")
	for _, modFS := range modules {
		if modFS == nil {
			continue
		}
		if fileHandle, openErr := fs.Sub(modFS, "."); openErr == nil {
			if f, err := fileHandle.Open(path.Join(iconsDirName, cleanName)); err == nil {
				return f, nil
			}
			if f, err := fileHandle.Open(cleanName); err == nil {
				return f, nil
			}
		}
	}
	return nil, fs.ErrNotExist
}

type moduleTplSource struct {
	tplName  string
	rawBytes []byte
}

// LoadTemplatesFromModules compiles all ui_components/*.tpl and templates/*.tpl across an ordered list of module filesystems.
func LoadTemplatesFromModules(moduleFSList []fs.FS) (map[string]*ParsedTemplate, error) {
	iconsLookup := &multiIconsFS{modules: moduleFSList}
	var sources []moduleTplSource

	for _, modFS := range moduleFSList {
		if modFS == nil {
			continue
		}
		for _, subDir := range []string{uiComponentsDirName, templatesDirName} {
			entries, readErr := fs.ReadDir(modFS, subDir)
			if readErr != nil {
				continue
			}
			var fileNames []string
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), tplExtSuffix) {
					fileNames = append(fileNames, entry.Name())
				}
			}
			sort.Strings(fileNames)
			for _, fileName := range fileNames {
				rawBytes, fileErr := fs.ReadFile(modFS, path.Join(subDir, fileName))
				if fileErr != nil {
					return nil, fileErr
				}
				tplName := strings.TrimSuffix(fileName, tplExtSuffix)
				sources = append(sources, moduleTplSource{
					tplName:  tplName,
					rawBytes: rawBytes,
				})
			}
		}
	}

	compiled := make(map[string]*ParsedTemplate, len(sources))
	for _, srcItem := range sources {
		parsed, parseErr := ParseTemplateString(string(srcItem.rawBytes), srcItem.tplName)
		if parseErr != nil {
			return nil, parseErr
		}
		if svgErr := resolveExternalSVGs("", iconsLookup, parsed.Nodes); svgErr != nil {
			return nil, svgErr
		}
		compiled[srcItem.tplName] = parsed
		registerDefsFromNodes(compiled, parsed.Nodes)
	}
	return compiled, nil
}

// BundleModulesCSS transpiles and concatenates styles/*.tss across an ordered list of module filesystems.
func BundleModulesCSS(moduleFSList []fs.FS, prodMode bool) (string, error) {
	switches := DefaultCSSSwitches()
	var out strings.Builder

	for _, modFS := range moduleFSList {
		if modFS == nil {
			continue
		}
		entries, readErr := fs.ReadDir(modFS, stylesDirName)
		if readErr != nil {
			continue
		}
		var fileNames []string
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), tssExtSuffix) {
				fileNames = append(fileNames, entry.Name())
			}
		}
		sortStyleFileNames(fileNames)
		for _, fileName := range fileNames {
			relPath := path.Join(stylesDirName, fileName)
			rawBytes, fileErr := fs.ReadFile(modFS, relPath)
			if fileErr != nil {
				return "", fmt.Errorf(errReadStyleFmt, relPath, fileErr)
			}
			content := string(rawBytes)
			if prodMode {
				content = StripDevOnlyBlocks(content)
			}
			sourceName := strings.TrimSuffix(fileName, tssExtSuffix)
			rendered, transpileErr := TranspileCSSWithSwitches(content, sourceName, nil, switches, prodMode)
			if transpileErr != nil {
				return "", transpileErr
			}
			rendered = strings.TrimSpace(rendered)
			if rendered == "" {
				continue
			}
			if out.Len() > 0 {
				out.WriteString("\n\n")
			}
			out.WriteString(rendered)
		}
	}
	if out.Len() > 0 {
		out.WriteByte('\n')
	}
	return out.String(), nil
}

func sortStyleFileNames(fileNames []string) {
	priorityRank := func(name string) int {
		switch name {
		case priorityColorsTSS:
			return 0
		case priorityBaseTSS:
			return 1
		case priorityFormsTSS:
			return 2
		default:
			return 3
		}
	}
	sort.Slice(fileNames, func(leftIdx, rightIdx int) bool {
		leftRank := priorityRank(fileNames[leftIdx])
		rightRank := priorityRank(fileNames[rightIdx])
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return fileNames[leftIdx] < fileNames[rightIdx]
	})
}

// BundleModulesJS concatenates engine transport scripts and module scripts/controllers into a single JS bundle.
func BundleModulesJS(engineFS fs.FS, moduleFSList []fs.FS, prodMode bool) (string, error) {
	var out strings.Builder
	out.WriteString(useStrictHeader)
	if I18nScriptGenerator != nil {
		out.WriteString(I18nScriptGenerator())
		out.WriteByte('\n')
	}

	if engineFS != nil {
		for _, scriptPath := range engineCoreScriptFiles {
			rawBytes, readErr := fs.ReadFile(engineFS, scriptPath)
			if readErr != nil {
				return "", fmt.Errorf(errReadScriptFmt, scriptPath, readErr)
			}
			appendCleanedJSSource(&out, string(rawBytes), prodMode)
		}
	}

	for _, modFS := range moduleFSList {
		if modFS == nil {
			continue
		}
		scriptFiles := collectModuleJSFiles(modFS, scriptsDirName)
		for _, relPath := range scriptFiles {
			rawBytes, readErr := fs.ReadFile(modFS, relPath)
			if readErr != nil {
				return "", fmt.Errorf(errReadScriptFmt, relPath, readErr)
			}
			appendCleanedJSSource(&out, string(rawBytes), prodMode)
		}
		controllerFiles := collectModuleJSFiles(modFS, controllersDirName)
		for _, relPath := range controllerFiles {
			rawBytes, readErr := fs.ReadFile(modFS, relPath)
			if readErr != nil {
				return "", fmt.Errorf(errReadScriptFmt, relPath, readErr)
			}
			appendCleanedJSSource(&out, string(rawBytes), prodMode)
		}
	}

	return out.String(), nil
}

func collectModuleJSFiles(modFS fs.FS, rootSubDir string) []string {
	var topLevel []string
	var nested []string
	_ = fs.WalkDir(modFS, rootSubDir, func(walkPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), jsExtSuffix) {
			return nil
		}
		relInside := strings.TrimPrefix(walkPath, rootSubDir+"/")
		if strings.Contains(relInside, "/") {
			nested = append(nested, walkPath)
		} else {
			topLevel = append(topLevel, walkPath)
		}
		return nil
	})
	sort.Strings(topLevel)
	sort.Strings(nested)
	return append(topLevel, nested...)
}

func appendCleanedJSSource(out *strings.Builder, content string, prodMode bool) {
	if prodMode {
		content = StripDevOnlyBlock(content, devOnlyJSStart, devOnlyJSEnd)
	}
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, useStrictDouble) {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, useStrictDouble))
	} else if strings.HasPrefix(trimmed, useStrictSingle) {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, useStrictSingle))
	}
	if trimmed == "" {
		return
	}
	out.WriteString(trimmed)
	out.WriteString("\n\n")
}

// ReadModuleAsset looks up a static asset path (such as icons/*, fonts/*, studio-atlas.png) across module filesystems.
func ReadModuleAsset(moduleFSList []fs.FS, assetRelPath string) ([]byte, bool) {
	cleanRel := strings.TrimPrefix(path.Clean(assetRelPath), "/")
	if cleanRel == "" || cleanRel == "." {
		return nil, false
	}
	for _, modFS := range moduleFSList {
		if modFS == nil {
			continue
		}
		if rawBytes, readErr := fs.ReadFile(modFS, cleanRel); readErr == nil {
			return rawBytes, true
		}
	}
	return nil, false
}

// ComputeModulesDiskStamp computes an aggregate modification-time and size stamp across disk module directories.
func ComputeModulesDiskStamp(rootDir string, moduleSubDirs []string) (int64, int64) {
	var maxNano int64
	var totalSize int64
	scanDir := func(dirPath string) {
		_ = filepath.WalkDir(dirPath, func(_ string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return nil
			}
			if info, statErr := entry.Info(); statErr == nil {
				if mtime := info.ModTime().UnixNano(); mtime > maxNano {
					maxNano = mtime
				}
				totalSize += info.Size()
			}
			return nil
		})
	}
	scanDir(filepath.Join(rootDir, "engine", scriptsDirName))
	for _, modRel := range moduleSubDirs {
		scanDir(filepath.Join(rootDir, "domain", filepath.FromSlash(modRel)))
	}
	if maxNano == 0 {
		maxNano = time.Now().UnixNano()
	}
	return maxNano, totalSize
}

// BuildDiskOrEmbeddedModules returns disk-backed fs.FS instances when present in rootDir, falling back to embeddedFSList.
func BuildDiskOrEmbeddedModules(rootDir string, moduleSubDirs []string, embeddedFSList []fs.FS) ([]fs.FS, fs.FS) {
	var engineOut fs.FS
	engineDiskDir := filepath.Join(rootDir, "engine")
	if info, statErr := os.Stat(filepath.Join(engineDiskDir, scriptsDirName)); statErr == nil && info.IsDir() {
		engineOut = os.DirFS(engineDiskDir)
	}
	resolved := make([]fs.FS, 0, len(embeddedFSList))
	for idx, embFS := range embeddedFSList {
		if idx < len(moduleSubDirs) && rootDir != "" {
			diskModDir := filepath.Join(rootDir, "domain", filepath.FromSlash(moduleSubDirs[idx]))
			if info, statErr := os.Stat(diskModDir); statErr == nil && info.IsDir() {
				resolved = append(resolved, os.DirFS(diskModDir))
				continue
			}
		}
		resolved = append(resolved, embFS)
	}
	return resolved, engineOut
}

// DefaultRootContext returns a baseline RenderContext for static shell compilation.
func DefaultRootContext(prodMode bool) *RenderContext {
	return NewContext(map[string]any{
		ctxKeyTheme:         ctxValDark,
		ctxKeyUIScale:       ctxValSmall,
		ctxKeyImagelessAttr: ctxValFalse,
		ctxKeyLockedAttr:    ctxValFalse,
		ctxKeyZenAttr:       ctxValFalse,
		ctxKeySensitivity:   ctxValMedium,
		ctxKeyPreviewMode:   ctxValFull,
		ctxKeyDevMode:       !prodMode,
		ctxKeyIsDocRoot:     true,
		ctxKeyLang:          ctxValPL,
	})
}

// BuildModuleAppToDir compiles entryTemplate ("shell_view"), style.css, and app.js from moduleFSList into outDir.
func BuildModuleAppToDir(engineFS fs.FS, moduleFSList []fs.FS, entryTemplate string, outDir string, prodMode bool) error {
	if mkdirErr := os.MkdirAll(outDir, 0755); mkdirErr != nil {
		return fmt.Errorf(errMkdirOutFmt, outDir, mkdirErr)
	}
	if entryTemplate == "" {
		entryTemplate = defaultShellEntryTpl
	}
	parsedTemplates, tplErr := LoadTemplatesFromModules(moduleFSList)
	if tplErr != nil {
		return tplErr
	}
	if prodMode {
		StripDevNodesFromTemplates(parsedTemplates)
	}
	if _, exists := parsedTemplates[entryTemplate]; !exists {
		return fmt.Errorf(errEntryTplMissingFmt, entryTemplate)
	}

	renderBuf := AcquireBuffer()
	renderErr := RenderTemplateTo(renderBuf, entryTemplate, parsedTemplates, DefaultRootContext(prodMode))
	htmlBytes := append([]byte(nil), renderBuf.Bytes()...)
	ReleaseBuffer(renderBuf)
	if renderErr != nil {
		return renderErr
	}
	if writeErr := os.WriteFile(filepath.Join(outDir, outIndexHTMLName), htmlBytes, 0644); writeErr != nil {
		return writeErr
	}

	cssContent, cssErr := BundleModulesCSS(moduleFSList, prodMode)
	if cssErr != nil {
		return cssErr
	}
	if writeErr := os.WriteFile(filepath.Join(outDir, outStyleCSSName), []byte(cssContent), 0644); writeErr != nil {
		return writeErr
	}

	jsContent, jsErr := BundleModulesJS(engineFS, moduleFSList, prodMode)
	if jsErr != nil {
		return jsErr
	}
	return os.WriteFile(filepath.Join(outDir, outAppJSName), []byte(jsContent), 0644)
}

// StripDevOnlyBlock removes sections enclosed between startMarker and endMarker.
func StripDevOnlyBlock(content, startMarker, endMarker string) string {
	for step := 0; step < 1024; step++ {
		startIdx := strings.Index(content, startMarker)
		if startIdx < 0 {
			break
		}
		endIdx := strings.Index(content[startIdx:], endMarker)
		if endIdx < 0 {
			content = content[:startIdx]
			break
		}
		content = content[:startIdx] + content[startIdx+endIdx+len(endMarker):]
	}
	return content
}
