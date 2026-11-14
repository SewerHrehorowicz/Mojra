package i18n

import (
	"embed"
	"strconv"
	"strings"
)

//go:embed *.cfg
var i18nCfgFS embed.FS

// Lang represents the active UI language index in static translation tables.
type Lang uint8

const (
	LangPL Lang = iota
	LangEN
	LangCount
)

// LangSpec describes a registered UI language.
// To add a new language (e.g. French), add LangFR before LangCount, register its LangSpec
// in SupportedLanguages, and add internal/tpl/i18n/<code.cfg>.
type LangSpec struct {
	ID      Lang
	Code    string
	Upper   string
	Name    string
	Aliases []string
}

// SupportedLanguages is the ordered registry of all supported UI languages.
var SupportedLanguages = [LangCount]LangSpec{
	LangPL: {ID: LangPL, Code: "pl", Upper: "PL", Name: "Polski", Aliases: []string{"pol", "polish"}},
	LangEN: {ID: LangEN, Code: "en", Upper: "EN", Name: "English", Aliases: []string{"eng", "english"}},
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

// jsI18nKeys defines the canonical order of client-side JS translation keys.
var jsI18nKeys = [...]string{
	"imageless_locked_hint",
	"imageless_enable_hint",
	"choice_prefix",
	"entry_badge_upper",
	"next",
	"default_chapter",
}

// I18nDicts holds per-language dictionaries parsed from internal/tpl/i18n/<code.cfg>.
var I18nDicts [LangCount]map[string]string

// JSI18nTable is the compiled table of client-side UI strings used in app/js/*.js.
// Each entry is [1 + LangCount]string{key, lang0, lang1, ...} with fallback to LangEN then LangPL.
var JSI18nTable [len(jsI18nKeys)][1 + int(LangCount)]string

var cachedJSI18nHeader string

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
			val = strings.ReplaceAll(val, `\n`, "\n")
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

func init() {
	for l := Lang(0); l < LangCount; l++ {
		code := SupportedLanguages[l].Code
		raw, err := i18nCfgFS.ReadFile(code + ".cfg")
		if err != nil {
			I18nDicts[l] = make(map[string]string)
			continue
		}
		I18nDicts[l] = parseI18nCfg(string(raw))
	}

	var sb strings.Builder
	sb.WriteString("const I18N_LANGS = {")
	for l := Lang(0); l < LangCount; l++ {
		if l > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(SupportedLanguages[l].Code)
		sb.WriteString(": ")
		sb.WriteString(strconv.Itoa(int(l)))
	}
	sb.WriteString("};\nconst I18N_TABLE = {\n")
	for i, key := range jsI18nKeys {
		JSI18nTable[i][0] = key
		sb.WriteString("  ")
		sb.WriteString(key)
		sb.WriteString(": [")
		for l := Lang(0); l < LangCount; l++ {
			val := LookupJSI18n(l, key)
			JSI18nTable[i][1+int(l)] = val
			if l > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(strconv.Quote(val))
		}
		sb.WriteString("],\n")
	}
	sb.WriteString("};\n")
	cachedJSI18nHeader = sb.String()
	initUIContextTables()
}

// BuildJSI18nHeader returns the pre-computed JS constant declaration for I18N_LANGS and I18N_TABLE.
func BuildJSI18nHeader() string {
	return cachedJSI18nHeader
}
