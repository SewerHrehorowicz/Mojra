//go:build !(js && wasm)

package tpl

import (
	"regexp"
	"strings"
)

const (
	tssExt            = ".tss"
	cssExt            = ".css"
	cssVarPrefix      = "--"
	tssDollarPrefix   = "$"
	cssVarCallPrefix  = "var("
	cssVarOpenPrefix  = "var(--"
	cssPseudoPrefix   = "::"
	cssRootSelector   = ":root"
	cssContentProp    = "content"
	cssTransitionProp = "transition"
	cssBackgroundProp = "background"
	cssMaskProp       = "mask"
	cssWebkitMaskProp = "-webkit-mask"
	cssDevModeKey     = "dev_mode"

	tssKwSwitch        = "switch"
	tssKwSwitchSpace   = "switch "
	tssKwSwitchParen   = "switch("
	tssKwAtSwitch      = "@switch"
	tssKwAtSwitchSpace = "@switch "
	tssKwAtSwitchParen = "@switch("
	tssKwAtIfSpace     = "@if "
	tssKwAtIf          = "@if"
	tssKwIfSpace       = "if "
	tssKwNotSpace      = "not "
	tssKwBefore        = "before"
	tssKwAfter         = "after"
	tssArrowToken      = "->"

	cssBlockFmt        = "%s {\n%s\n}"
	cssDeclFmt         = "%s%s: %s;"
	cssIndentedDeclFmt = "  %s: %s;"
	cssContentDeclFmt  = "  content: %s;"
	errReadCSSFileFmt  = "read %s: %w"

	errCSSInlineSwitchCountFmt = "css compile error in %s.css:%d: too many inline values (%d) for switch(%s) which has %d cases"
	errCSSSwitchDefCaseFmt     = "css compile error in %s.css:%d: switch case definition %q must be 'case selector' or 'case: selector'"
	errCSSPropSwitchCaseFmt    = "css compile error in %s.css:%d: property switch case %q must be 'case value' or 'case: value'"
	errCSSUnsupportedFlowFmt   = "css compile error in %s.css:%d: unsupported control flow %q"
	errCSSInvalidPropSwitchFmt = "css compile error in %s.css:%d: invalid property name %q before -> switch(%s)"
	errCSSReservedArrowFmt     = "css compile error in %s.css:%d: '->' in CSS is reserved for control structures like '-> switch(...)'; use '%s: {%s}' or '%s: $%s' instead"
	errCSSBareStringRootFmt    = "css compile error in %s.css:%d: bare string %s must be inside a pseudo-element block"
	errCSSUnindentedPropFmt    = "css compile error in %s.css:%d: property %q must be indented under a selector"
	errCSSUnknownSwitchFmt     = "css compile error in %s.css:%d: unknown switch %q"
	errCSSUnknownSwitchCaseFmt = "css compile error in %s.css:%d: unknown case %q in switch(%s)"
)

type cssNodeKind int

const (
	cssKindRoot cssNodeKind = iota
	cssKindRule
	cssKindSwitchDef
	cssKindPropSwitch
	cssKindBlockSwitch
	cssKindSwitchCase
	cssKindCond
)

// CSSSwitchDef holds a named CSS switch mapping case names to CSS selectors.
type CSSSwitchDef struct {
	Name  string
	Order []string
	Cases map[string]string
}

type cssSwitchCaseSpec struct {
	Key      string
	Selector string
}

type cssSwitchSpec struct {
	Name  string
	Cases []cssSwitchCaseSpec
}

type cssDecl struct {
	Key  string
	Val  string
	Line int
}

type cssSwitchCaseVal struct {
	CaseKeys []string
	Val      string
	Line     int
}

type cssNode struct {
	Kind       cssNodeKind
	Selector   string
	PropKey    string
	SwitchName string
	CaseKeys   []string
	Cond       string
	Decls      []cssDecl
	PropCases  []cssSwitchCaseVal
	Children   []*cssNode
	Indent     int
	Line       int
	File       string
}

type cssPropHeader struct {
	Key      string
	Val      string
	HasColon bool
}

type cssPreprocessedLine struct {
	Indent  int
	Content string
	LineNum int
}

var (
	cssPropKeyRe            = regexp.MustCompile(`^(?:\$|--)?-?[a-zA-Z_][a-zA-Z0-9_-]*$`)
	cssSwitchTargetRe       = regexp.MustCompile(`^switch\(\s*([a-zA-Z_][a-zA-Z0-9_-]*)\s*\)$`)
	cssSwitchDefCaseLineRe  = regexp.MustCompile(`^([a-zA-Z0-9_-]+)(?::\s*|\s+)(.+)$`)
	cssPropSwitchCaseLineRe = regexp.MustCompile(`^([a-zA-Z0-9_-]+(?:\s*,\s*[a-zA-Z0-9_-]+)*)(?:\s*:\s*|\s+)(.+)$`)
	cssCaseKeyIdentRe       = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	defaultCSSSwitchSpecs = []cssSwitchSpec{
		{
			Name: "theme",
			Cases: []cssSwitchCaseSpec{
				{Key: "dark", Selector: `:root, body[data-theme="dark"]`},
				{Key: "light", Selector: `body[data-theme="light"]`},
			},
		},
		{
			Name: "preview",
			Cases: []cssSwitchCaseSpec{
				{Key: "full", Selector: `.device-stage[data-preview="full"]`},
				{Key: "portrait", Selector: `.device-stage[data-preview="portrait"]`},
				{Key: "tablet", Selector: `.device-stage[data-preview="tablet"]`},
				{Key: "landscape", Selector: `.device-stage[data-preview="landscape"]`},
			},
		},
	}

	cssPseudoShorthands = map[string]string{
		"before": "::before",
		"after":  "::after",
	}

	cssSpaceSeparatedFuncs = map[string]bool{
		"drop-shadow": true,
		"inset":       true,
		"xywh":        true,
		"rect":        true,
	}

	cssImageLayerTokens = []string{
		"gradient(",
		"url(",
		"image-set(",
		"cross-fade(",
	}

	cssBackgroundModifierKeywords = map[string]bool{
		"left":        true,
		"center":      true,
		"right":       true,
		"top":         true,
		"bottom":      true,
		"repeat":      true,
		"repeat-x":    true,
		"repeat-y":    true,
		"no-repeat":   true,
		"space":       true,
		"round":       true,
		"scroll":      true,
		"fixed":       true,
		"local":       true,
		"border-box":  true,
		"padding-box": true,
		"content-box": true,
		"text":        true,
		"contain":     true,
		"cover":       true,
		"auto":        true,
	}

	multilineCSSProperties = map[string]bool{
		"transition":                 true,
		"transition-property":        true,
		"transition-duration":        true,
		"transition-timing-function": true,
		"transition-delay":           true,
		"background":                 true,
		"background-image":           true,
		"background-position":        true,
		"background-size":            true,
		"background-repeat":          true,
		"background-origin":          true,
		"background-clip":            true,
		"background-attachment":      true,
		"background-blend-mode":      true,
		"box-shadow":                 true,
		"text-shadow":                true,
		"transform":                  true,
		"filter":                     true,
		"backdrop-filter":            true,
		"-webkit-backdrop-filter":    true,
		"animation":                  true,
		"animation-name":             true,
		"animation-duration":         true,
		"animation-timing-function":  true,
		"animation-delay":            true,
		"animation-iteration-count":  true,
		"animation-direction":        true,
		"animation-fill-mode":        true,
		"animation-play-state":       true,
		"font-family":                true,
		"font-feature-settings":      true,
		"font-variation-settings":    true,
		"font-variant":               true,
		"will-change":                true,
		"mask":                       true,
		"mask-image":                 true,
		"mask-position":              true,
		"mask-size":                  true,
		"mask-repeat":                true,
		"mask-origin":                true,
		"mask-clip":                  true,
		"mask-composite":             true,
		"-webkit-mask":               true,
		"-webkit-mask-image":         true,
		"clip-path":                  true,
		"-webkit-clip-path":          true,
		"grid-template":              true,
		"grid-template-areas":        true,
		"grid-template-rows":         true,
		"grid-template-columns":       true,
		"border-image":               true,
		"text-decoration":            true,
		"src":                        true,
		"cursor":                     true,
		"contain":                    true,
		"touch-action":               true,
		"outline":                    true,
		"border":                     true,
		"border-top":                 true,
		"border-right":               true,
		"border-bottom":              true,
		"border-left":                true,
		"padding":                    true,
		"margin":                     true,
		"inset":                      true,
		"gap":                        true,
		"flex":                       true,
		"grid-column":                true,
		"grid-row":                   true,
		"place-items":                true,
		"place-content":              true,
		"place-self":                 true,
	}

	spaceJoinedCSSProperties = map[string]bool{
		"transform":               true,
		"filter":                  true,
		"backdrop-filter":         true,
		"-webkit-backdrop-filter": true,
		"clip-path":               true,
		"-webkit-clip-path":       true,
		"grid-template":           true,
		"grid-template-areas":     true,
		"grid-template-rows":      true,
		"grid-template-columns":    true,
		"border-image":            true,
		"text-decoration":         true,
		"font-variant":            true,
		"contain":                 true,
		"touch-action":            true,
		"outline":                 true,
		"border":                  true,
		"border-top":              true,
		"border-right":            true,
		"border-bottom":           true,
		"border-left":             true,
		"padding":                 true,
		"margin":                  true,
		"inset":                   true,
		"gap":                     true,
		"flex":                    true,
		"grid-column":             true,
		"grid-row":                true,
		"place-items":             true,
		"place-content":           true,
		"place-self":              true,
	}

	cssNestableFunctionNames = map[string]bool{
		"linear-gradient":           true,
		"radial-gradient":           true,
		"conic-gradient":            true,
		"repeating-linear-gradient": true,
		"repeating-radial-gradient": true,
		"repeating-conic-gradient":  true,
		"polygon":                   true,
		"inset":                     true,
		"circle":                    true,
		"ellipse":                   true,
		"xywh":                      true,
		"rect":                      true,
		"path":                      true,
		"steps":                     true,
		"cubic-bezier":              true,
		"color-mix":                 true,
		"light-dark":                true,
		"image-set":                 true,
		"-webkit-image-set":         true,
		"cross-fade":                true,
		"drop-shadow":               true,
		"clamp":                     true,
		"min":                       true,
		"max":                       true,
		"calc":                      true,
		"translate":                 true,
		"translate3d":               true,
		"scale":                     true,
		"scale3d":                   true,
		"rotate":                    true,
		"rotate3d":                  true,
		"skew":                      true,
		"matrix":                    true,
		"matrix3d":                  true,
		"rgb":                       true,
		"rgba":                      true,
		"hsl":                       true,
		"hsla":                      true,
		"oklch":                     true,
		"oklab":                     true,
		"lab":                       true,
		"lch":                       true,
		"color":                     true,
	}
)

func isCustomCSSPropKey(key string) bool {
	return strings.HasPrefix(key, cssVarPrefix) || strings.HasPrefix(key, tssDollarPrefix)
}

func trimCSSTrailingToken(s, suffix string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), suffix))
}

func trimCSSLeadingToken(s, prefix string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), prefix))
}

func splitArrowSides(content string) (string, string, bool) {
	arrowIdx := findUnquotedArrow(content)
	if arrowIdx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(content[:arrowIdx]), strings.TrimSpace(content[arrowIdx+2:]), true
}
