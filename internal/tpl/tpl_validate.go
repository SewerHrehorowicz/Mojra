package tpl

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func validateNodesRecursive(nodes []*Node) error {
	for i, n := range nodes {
		if n.Kind == NodeElif || n.Kind == NodeElse {
			kw := "else"
			if n.Kind == NodeElif {
				kw = "elif " + n.Expr
			}
			if i == 0 {
				return fmt.Errorf("tpl compile error in %s.tpl:%d: %q must immediately follow an 'if' or 'elif' block", n.File, n.Line, kw)
			}
			prevKind := nodes[i-1].Kind
			if prevKind != NodeIf && prevKind != NodeElif && !(n.Kind == NodeElse && prevKind == NodeWith) {
				return fmt.Errorf("tpl compile error in %s.tpl:%d: %q must immediately follow an 'if' or 'elif' block", n.File, n.Line, kw)
			}
		}
		if n.Kind == NodeElement || n.Kind == NodeComponent {
			if err := validateNoInlineCSSOrJS(n); err != nil {
				return err
			}
		}
		if err := validateNodesRecursive(n.Children); err != nil {
			return err
		}
	}
	return nil
}

func unquoteLiteral(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func validateNoInlineCSSOrJS(n *Node) error {
	if !validTagRe.MatchString(n.Tag) {
		return fmt.Errorf(
			"tpl compile error in %s.tpl:%d: invalid or unsafe tag/component name %q",
			n.File, n.Line, n.Tag,
		)
	}
	if n.ID != "" && !validIDRe.MatchString(n.ID) {
		return fmt.Errorf(
			"tpl compile error in %s.tpl:%d: invalid element id %q",
			n.File, n.Line, n.ID,
		)
	}
	for _, cls := range n.Classes {
		if !validIDRe.MatchString(cls) {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: invalid CSS class name %q",
				n.File, n.Line, cls,
			)
		}
	}
	for _, cc := range n.CondClasses {
		if cc.Name == "" || cc.Cond == "" || !validIDRe.MatchString(cc.Name) {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: invalid conditional CSS class %q",
				n.File, n.Line, cc.Name,
			)
		}
	}

	tagLower := strings.ToLower(n.Tag)
	if !n.IsComponent && tagLower == "button" {
		txt := strings.TrimSpace(unquoteLiteral(n.Text))
		if txt == "" && len(n.Children) == 1 && n.Children[0].Kind == NodeText {
			txt = strings.TrimSpace(unquoteLiteral(n.Children[0].Text))
		}
		if txt == "↑" || txt == "↓" {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: up/down reorder arrow buttons (↑/↓) are strictly forbidden",
				n.File, n.Line,
			)
		}
	}
	if !n.IsComponent && tagLower == "style" {
		return fmt.Errorf(
			"tpl compile error in %s.tpl:%d: <style> tags are strictly forbidden in .tpl files; move all CSS rules to app/styles/*.css",
			n.File, n.Line,
		)
	}
	if !n.IsComponent && forbiddenTags[tagLower] {
		return fmt.Errorf(
			"tpl compile error in %s.tpl:%d: <%s> tags are strictly forbidden in .tpl files for security",
			n.File, n.Line, tagLower,
		)
	}

	if !n.IsComponent && tagLower == "script" {
		if n.Text != "" || n.Expr != "" || len(n.Children) > 0 {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: inline JS inside <script> is strictly forbidden in .tpl files; move all JavaScript to app/js/*.js",
				n.File, n.Line,
			)
		}
		hasSrc := false
		for _, a := range n.Attrs {
			if strings.EqualFold(a.Key, "src") {
				srcVal := strings.TrimSpace(unquoteLiteral(a.Val))
				if srcVal != "" {
					if !strings.HasPrefix(srcVal, "/") || strings.HasPrefix(srcVal, "//") || strings.Contains(srcVal, "..") {
						return fmt.Errorf(
							"tpl compile error in %s.tpl:%d: external or non-root script src %q is strictly forbidden",
							n.File, n.Line, a.Val,
						)
					}
					hasSrc = true
				}
			}
		}
		if !hasSrc {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: <script> without [src=\"...\"] is strictly forbidden in .tpl files; move all JavaScript to app/js/*.js",
				n.File, n.Line,
			)
		}
	}

	if !n.IsComponent && tagLower == "svg" {
		if n.Text != "" || n.Expr != "" || len(n.Children) > 0 {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: inline <svg> is strictly forbidden in .tpl files; move all SVG icons to app/icons/*.svg and reference them via svg src: /icons/...",
				n.File, n.Line,
			)
		}
		hasSrc := false
		for _, a := range n.Attrs {
			if strings.EqualFold(a.Key, "src") && strings.TrimSpace(unquoteLiteral(a.Val)) != "" {
				hasSrc = true
			}
		}
		if !hasSrc {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: <svg> without src: is strictly forbidden in .tpl files; move all SVG icons to app/icons/*.svg and reference them via svg src: /icons/...",
				n.File, n.Line,
			)
		}
	}

	for _, a := range n.Attrs {
		keyTrimmed := strings.TrimSpace(a.Key)
		if !attrKeyRe.MatchString(keyTrimmed) {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: invalid attribute name %q",
				n.File, n.Line, a.Key,
			)
		}
		keyLower := strings.ToLower(keyTrimmed)
		rawVal := unquoteLiteral(a.Val)
		valNormalized := normalizeAttrValueForSecurityCheck(rawVal)

		if keyLower == "style" {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: inline CSS attribute [style=%q] is strictly forbidden in .tpl files; use semantic CSS classes in app/styles/*.css instead",
				n.File, n.Line, rawVal,
			)
		}
		if keyLower == "srcdoc" || keyLower == "http-equiv" {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: unsafe attribute [%s=%q] is strictly forbidden in .tpl files",
				n.File, n.Line, a.Key, rawVal,
			)
		}
		if strings.HasPrefix(keyLower, "on") {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: inline JS event handler [%s=%q] is strictly forbidden in .tpl files; bind events in app/js/core.js and app/js/events.js instead",
				n.File, n.Line, a.Key, rawVal,
			)
		}
		if strings.HasPrefix(valNormalized, "javascript:") ||
			strings.HasPrefix(valNormalized, "vbscript:") ||
			strings.HasPrefix(valNormalized, "data:") {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: javascript:, vbscript:, or data: URL in [%s=%q] is strictly forbidden in .tpl files",
				n.File, n.Line, a.Key, rawVal,
			)
		}
	}

	return nil
}

func normalizeAttrValueForSecurityCheck(v string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(v) {
		if r <= 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	s := b.String()
	if strings.Contains(s, "&#") || strings.Contains(s, "&colon;") || strings.Contains(s, "&tab;") || strings.Contains(s, "&newline;") {
		return "javascript:obfuscated-entity"
	}
	return s
}

func validateSafeSVGContent(svgContent, file string, line int, src string) error {
	lower := strings.ToLower(svgContent)
	if !strings.HasPrefix(strings.TrimSpace(lower), "<svg") {
		return fmt.Errorf("tpl compile error in %s.tpl:%d: SVG file %q must start with <svg", file, line, src)
	}
	if strings.Contains(lower, "<script") ||
		strings.Contains(lower, "<foreignobject") ||
		strings.Contains(lower, "<iframe") ||
		strings.Contains(lower, "javascript:") ||
		strings.Contains(lower, "data:text/html") ||
		svgEventRe.MatchString(svgContent) {
		return fmt.Errorf("tpl compile error in %s.tpl:%d: unsafe executable content inside SVG %q is strictly forbidden", file, line, src)
	}
	return nil
}

func resolveExternalSVGs(templatesDir string, embeddedFS fs.FS, nodes []*Node) error {
	appDir := ""
	if templatesDir != "" {
		appDir = filepath.Dir(templatesDir)
	}
	for _, n := range nodes {
		if (!n.IsComponent && strings.EqualFold(n.Tag, "svg")) || n.Kind == NodeSVG {
			var src string
			if n.Kind == NodeSVG {
				src = strings.TrimSpace(unquoteLiteral(n.Expr))
				if src != "" && !strings.HasPrefix(src, "/") && !strings.HasSuffix(src, ".svg") {
					src = "/icons/" + src + ".svg"
				}
			} else {
				for _, a := range n.Attrs {
					if strings.EqualFold(a.Key, "src") {
						src = strings.TrimSpace(unquoteLiteral(a.Val))
						break
					}
				}
			}
			rel := filepath.FromSlash(strings.TrimPrefix(src, "/"))
			if strings.Contains(rel, "..") {
				return fmt.Errorf("tpl compile error in %s.tpl:%d: path traversal in SVG src %q is forbidden", n.File, n.Line, src)
			}
			var raw []byte
			var err error
			if templatesDir != "" {
				svgPath := filepath.Join(appDir, rel)
				raw, err = os.ReadFile(svgPath)
				if err != nil {
					for _, altPath := range []string{
						filepath.Join(templatesDir, rel),
						filepath.Join(appDir, "app", rel),
					} {
						if altRaw, altErr := os.ReadFile(altPath); altErr == nil {
							raw = altRaw
							err = nil
							break
						}
					}
				}
			} else {
				err = os.ErrNotExist
			}
			if err != nil && embeddedFS != nil {
				if embRaw, embErr := fs.ReadFile(embeddedFS, "app/"+strings.TrimPrefix(src, "/")); embErr == nil {
					raw = embRaw
					err = nil
				}
			}
			if err != nil {
				return fmt.Errorf("tpl compile error in %s.tpl:%d: cannot read SVG file %q: %w", n.File, n.Line, src, err)
			}
			svgContent := strings.TrimSpace(string(raw))
			if err := validateSafeSVGContent(svgContent, n.File, n.Line, src); err != nil {
				return err
			}
			n.RawSVG = svgContent
		}
		if err := resolveExternalSVGs(templatesDir, embeddedFS, n.Children); err != nil {
			return err
		}
	}
	return nil
}
