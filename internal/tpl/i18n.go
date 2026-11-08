package tpl

import (
	"embed"
	"strconv"
	"strings"
)

//go:embed i18n/*.cfg
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
	"fullscreen_exit_hint",
	"fullscreen_enter_hint",
	"scale_zen_hint",
	"scale_exit_zen_hint",
	"scale_enlarge_hint",
	"scale_shrink_hint",
	"imageless_locked_hint",
	"imageless_enable_hint",
	"imageless_disable_hint",
	"auto_choice_on",
	"auto_choice_off",
	"scene_text_placeholder",
	"choice_prefix",
	"no_scene_audio_toast",
	"entry_badge_upper",
	"next",
	"default_chapter",
	"selected_count_prefix",
	"selected_scenes_prefix",
	"delete_selected_scenes_prefix",
	"new_screenplay_modal_title",
	"new_book_modal_title",
	"title_key",
	"new_screenplay_placeholder",
	"new_book_placeholder",
	"create_screenplay_btn",
	"create_book_btn",
	"sp_ins_character",
	"sp_ins_dialogue",
	"sp_ins_paren",
	"sp_ins_paren_sel",
	"sp_ins_action",
	"sp_ins_off_hdr",
	"sp_ins_off_line",
	"sp_ins_vo_hdr",
	"sp_ins_vo_line",
	"sp_ins_cut",
	"sp_ins_fade",
	"sp_ins_subslug",
	"sp_ins_location",
	"err_missing_val_after_op",
	"err_int_type",
	"err_enum_op",
	"err_enum_val",
	"err_bool_op",
	"err_bool_type",
	"err_no_vars",
	"err_unclosed_paren",
	"err_empty_after_logic_op",
	"err_mut_op_in_if",
	"err_comma_in_if",
	"err_missing_var_before_op",
	"err_unknown_var",
	"err_cond_syntax",
	"err_set_syntax_empty",
	"err_set_empty_after_comma",
	"err_op_requires_int",
	"err_neg_requires_bool",
	"err_set_clause_syntax",
	"err_enum_needs_assign",
	"err_var_requires_int",
	"err_cond_op_in_set",
	"err_missing_val_after_compound",
	"err_int_after_compound",
	"err_missing_val_after_eq",
	"err_bare_var_needs_assign",
	"err_exit_empty",
	"err_exit_question",
	"err_exit_self_scene",
	"err_exit_self_chapter",
	"err_exit_unknown_chapter",
	"err_exit_unknown_target",
	"search_badge_scene",
	"search_badge_location",
	"search_badge_dialogue",
	"search_badge_prose",
	"search_badge_choice",
	"undo_toast",
	"redo_toast",
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
		raw, err := i18nCfgFS.ReadFile("i18n/" + code + ".cfg")
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
}

// BuildJSI18nHeader returns the pre-computed JS constant declaration for I18N_LANGS and I18N_TABLE.
func BuildJSI18nHeader() string {
	return cachedJSI18nHeader
}
