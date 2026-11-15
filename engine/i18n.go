package engine

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"mojra/engine/templates"
)

// TrKey identifies a UI string entry in <lang>.cfg.
type TrKey string

// Lang represents the active UI language index in static translation tables.
type Lang uint8

const (
	LangPL Lang = iota
	LangEN
	LangCount

	DefaultLang = LangPL

	cfgExt            = ".cfg"
	trPrefix          = "Tr"
	tplVarPrefix      = "t_"
	escapedNewline    = `\n`
	jsLangsPrefix     = "const I18N_LANGS = {"
	jsTableOpen       = "};\nconst I18N_TABLE = {\n"
	jsRowPrefix       = "  "
	jsArrayOpen       = ": ["
	jsArrayClose      = "],\n"
	jsTableClose      = "};\n"
	ctxKeyLang        = "lang"
	ctxKeyLangUpper   = "lang_upper"
	ctxKeyIsLangEN    = "is_lang_en"
	ctxKeyLangOptions = "lang_options"
	optKeyCode        = "code"
	optKeyUpper       = "upper"
	optKeyName        = "name"
	optKeyIsActive    = "is_active"
)

// LangSpec describes a registered UI language.
type LangSpec struct {
	ID      Lang
	Code    string
	Upper   string
	Name    string
	Aliases []string
}

type tplAnyEntry struct {
	varName string
	val     any
}

// SupportedLanguages is the ordered registry of all supported UI languages.
var SupportedLanguages = [LangCount]LangSpec{
	LangPL: {ID: LangPL, Code: "pl", Upper: "PL", Name: "Polski", Aliases: []string{"pol", "polish"}},
	LangEN: {ID: LangEN, Code: "en", Upper: "EN", Name: "English", Aliases: []string{"eng", "english"}},
}

// I18nDicts holds per-language dictionaries parsed from <code.cfg>.
var I18nDicts [LangCount]map[string]string

// JSI18nTable is the compiled table of client-side UI strings used in app/js/*.js.
// Each entry is [1 + LangCount]string{key, lang0, lang1, ...} with fallback to LangEN then LangPL.
var JSI18nTable [][1 + int(LangCount)]string

var (
	cachedJSI18nHeader  string
	registeredTrKeyMap  map[string]string
	tplEntriesByLang    [LangCount][]tplAnyEntry
	langCodeAnyTable    [LangCount]any
	langUpperAnyTable   [LangCount]any
	langIsENAnyTable    [LangCount]any
	langOptionsAnyTable [LangCount]any
	prewarmedLangRoots  [LangCount]*templates.RenderContext
)

func init() {
	for l := Lang(0); l < LangCount; l++ {
		I18nDicts[l] = make(map[string]string)
	}
	initUIContextTables()
	templates.I18nScriptGenerator = BuildJSI18nHeader
}

// ParseLang resolves a language code or alias (e.g. "en", "pl", "english") to a registered Lang index.
// Defaults to LangPL when unrecognized.
func ParseLang(raw string) Lang {
	clean := strings.ToLower(strings.TrimSpace(raw))
	if clean == "" {
		return LangPL
	}
	for i := Lang(0); i < LangCount; i++ {
		spec := &SupportedLanguages[i]
		if clean == spec.Code {
			return i
		}
		for _, alias := range spec.Aliases {
			if clean == alias {
				return i
			}
		}
	}
	return LangPL
}

// NormalizeLangCode normalizes a raw language string to its canonical lowercase code (e.g. "pl", "en").
func NormalizeLangCode(raw string) string {
	return SupportedLanguages[ParseLang(raw)].Code
}

// NextLang cycles to the next supported UI language in SupportedLanguages order.
func NextLang(cur Lang) Lang {
	if LangCount == 0 {
		return LangPL
	}
	return (cur + 1) % LangCount
}

// ClampLang ensures a Lang index is within [0, LangCount), falling back to DefaultLang.
func ClampLang(l Lang) Lang {
	if l >= LangCount {
		return DefaultLang
	}
	return l
}

func parseI18nCfg(raw string) map[string]string {
	lines := strings.Split(raw, "\n")
	dict := make(map[string]string, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed[0] == '#' || trimmed[0] == ';' {
			continue
		}
		sepIdx := strings.IndexByte(trimmed, ':')
		if sepIdx <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:sepIdx])
		val := strings.TrimSpace(trimmed[sepIdx+1:])
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			if unquoted, err := strconv.Unquote(val); err == nil {
				val = unquoted
			}
		} else {
			val = strings.ReplaceAll(val, escapedNewline, "\n")
		}
		dict[key] = val
	}
	return dict
}

// LookupJSI18n resolves a key in I18nDicts for the given language with fallback to LangEN then LangPL.
func LookupJSI18n(lang Lang, key string) string {
	if lang < LangCount {
		if v := I18nDicts[lang][key]; v != "" {
			return v
		}
	}
	if v := I18nDicts[LangEN][key]; v != "" {
		return v
	}
	return I18nDicts[LangPL][key]
}

// RegisterCatalog loads per-language .cfg dictionaries from cfgFS, registers custom TrKey template variable overrides,
// compiles the client JS i18n table, and pre-warms template language roots.
func RegisterCatalog(cfgFS fs.FS, trKeyMap map[string]string, jsKeys []string) {
	registeredTrKeyMap = trKeyMap
	for l := Lang(0); l < LangCount; l++ {
		code := SupportedLanguages[l].Code
		if cfgFS == nil {
			I18nDicts[l] = make(map[string]string)
			continue
		}
		raw, err := fs.ReadFile(cfgFS, code+cfgExt)
		if err != nil {
			I18nDicts[l] = make(map[string]string)
			continue
		}
		I18nDicts[l] = parseI18nCfg(string(raw))
	}

	JSI18nTable = make([][1 + int(LangCount)]string, len(jsKeys))
	var sb strings.Builder
	sb.WriteString(jsLangsPrefix)
	for l := Lang(0); l < LangCount; l++ {
		if l > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(SupportedLanguages[l].Code)
		sb.WriteString(": ")
		sb.WriteString(strconv.Itoa(int(l)))
	}
	sb.WriteString(jsTableOpen)
	for i, key := range jsKeys {
		JSI18nTable[i][0] = key
		sb.WriteString(jsRowPrefix)
		sb.WriteString(key)
		sb.WriteString(jsArrayOpen)
		for l := Lang(0); l < LangCount; l++ {
			val := LookupJSI18n(l, key)
			JSI18nTable[i][1+int(l)] = val
			if l > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(strconv.Quote(val))
		}
		sb.WriteString(jsArrayClose)
	}
	sb.WriteString(jsTableClose)
	cachedJSI18nHeader = sb.String()
	initUIContextTables()
}

// BuildJSI18nHeader returns the pre-computed JS constant declaration for I18N_LANGS and I18N_TABLE.
func BuildJSI18nHeader() string {
	return cachedJSI18nHeader
}

// TrKeyToTplVar converts a "TrFooBar" key to its template variable name "t_foo_bar".
func TrKeyToTplVar(key string) string {
	if registeredTrKeyMap != nil {
		if mapped, ok := registeredTrKeyMap[key]; ok && mapped != "" {
			return mapped
		}
	}
	s := strings.TrimPrefix(key, trPrefix)
	var b strings.Builder
	b.Grow(len(s) + 8)
	b.WriteString(tplVarPrefix)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			b.WriteByte('_')
			continue
		}
		if c >= 'A' && c <= 'Z' {
			if i > 0 && s[i-1] != '_' {
				prevLower := s[i-1] >= 'a' && s[i-1] <= 'z'
				nextLower := i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z'
				if prevLower || nextLower {
					b.WriteByte('_')
				}
			}
			b.WriteByte(c + ('a' - 'A'))
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func initUIContextTables() {
	var trKeys []string
	for k := range I18nDicts[LangPL] {
		if strings.HasPrefix(k, trPrefix) {
			trKeys = append(trKeys, k)
		}
	}
	tplVars := make([]string, len(trKeys))
	for i, k := range trKeys {
		tplVars[i] = TrKeyToTplVar(k)
	}

	for l := Lang(0); l < LangCount; l++ {
		entries := make([]tplAnyEntry, len(trKeys))
		for i, k := range trKeys {
			entries[i] = tplAnyEntry{
				varName: tplVars[i],
				val:     LookupJSI18n(l, k),
			}
		}
		tplEntriesByLang[l] = entries

		spec := SupportedLanguages[l]
		langCodeAnyTable[l] = spec.Code
		langUpperAnyTable[l] = spec.Upper
		langIsENAnyTable[l] = l == LangEN

		opts := make([]map[string]any, LangCount)
		for optIdx := Lang(0); optIdx < LangCount; optIdx++ {
			optSpec := SupportedLanguages[optIdx]
			opts[optIdx] = map[string]any{
				optKeyCode:     optSpec.Code,
				optKeyUpper:    optSpec.Upper,
				optKeyName:     optSpec.Name,
				optKeyIsActive: optIdx == l,
			}
		}
		langOptionsAnyTable[l] = opts

		vars := make(map[string]any, len(entries)+4)
		PopulateI18nContext(vars, l)
		prewarmedLangRoots[l] = templates.NewRenderContext(vars)
	}
}

// PopulateI18nContext writes all t_* UI strings and language metadata for the given Lang into vars with zero allocations.
func PopulateI18nContext(vars map[string]any, lang Lang) {
	if lang >= LangCount {
		lang = LangPL
	}
	vars[ctxKeyLang] = langCodeAnyTable[lang]
	vars[ctxKeyLangUpper] = langUpperAnyTable[lang]
	vars[ctxKeyIsLangEN] = langIsENAnyTable[lang]
	vars[ctxKeyLangOptions] = langOptionsAnyTable[lang]

	entries := tplEntriesByLang[lang]
	for i := range entries {
		vars[entries[i].varName] = entries[i].val
	}
}

// NewViewContext creates a RenderContext with pre-warmed i18n bindings as parent
// and the view data (either a *Struct or a map[string]any) as child overlay.
func NewViewContext(lang Lang, data any) *templates.RenderContext {
	base := prewarmedLangRoots[ClampLang(lang)]
	if m, ok := data.(map[string]any); ok {
		return base.WithOverlay(m)
	}
	return base.WithStruct(data)
}

func T(l Lang, k TrKey) string {
	return LookupJSI18n(l, string(k))
}

func Tf(l Lang, k TrKey, args ...any) string {
	return fmt.Sprintf(T(l, k), args...)
}

// Plural formats a count using singular/few/many translation keys without language branching in views.
func Plural(l Lang, count int, oneKey, fewKey, manyKey TrKey) string {
	mod10 := count % 10
	mod100 := count % 100
	switch {
	case count == 1:
		return T(l, oneKey)
	case mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14):
		return Tf(l, fewKey, count)
	default:
		return Tf(l, manyKey, count)
	}
}
