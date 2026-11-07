package tpl

import (
	"strconv"
	"strings"
)

// Lang represents the active UI language index in static translation tables.
type Lang uint8

const (
	LangPL Lang = iota
	LangEN
	LangCount
)

// LangSpec describes a registered UI language.
// To add a new language (e.g. French), add LangFR before LangCount, register its LangSpec
// in SupportedLanguages, and provide its dictionary in jsI18nDicts and internal/gui/i18n_<code.go>.
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
}

// jsI18nDicts holds per-language dictionaries for client-side JS strings.
var jsI18nDicts = [LangCount]map[string]string{
	LangPL: {
		"fullscreen_exit_hint":           "Wróć do trybu okienkowego",
		"fullscreen_enter_hint":          "Przełącz pełny ekran",
		"scale_zen_hint":                 "Tryb Zen — sama treść",
		"scale_exit_zen_hint":            "Wyjdź z trybu Zen (małe UI)",
		"scale_enlarge_hint":             "Powiększ przyciski i kontrolki (+ / Ctrl++)",
		"imageless_locked_hint":          "Obrazy wyłączone w ustawieniach książki (imageless: true)",
		"imageless_enable_hint":          "Włącz obrazy (tryb bez obrazów aktywny)",
		"imageless_disable_hint":         "Wyłącz obrazy (tryb tekstowy na środku)",
		"auto_choice_on":                 "Auto-Wybór: WŁ",
		"auto_choice_off":                "Auto-Wybór: WYŁ",
		"scene_text_placeholder":         "Wpisz treść sceny...",
		"choice_prefix":                  "Wybór ",
		"no_scene_audio_toast":           "Ta scena nie posiada przypisanego pliku audio.",
		"entry_badge_upper":              "WEJŚCIE",
		"next":                           "Dalej",
		"default_chapter":                "Rozdział 1",
		"selected_count_prefix":          "Zaznaczono: ",
		"selected_scenes_prefix":         "Zaznaczone sceny (",
		"delete_selected_scenes_prefix":  "Usuń zaznaczone sceny (",
		"new_screenplay_modal_title":     "Utwórz Nowy Scenariusz",
		"new_book_modal_title":           "Utwórz Nową Książkę",
		"title_key":                      "tytuł:",
		"new_screenplay_placeholder":     "np. Ostatni Rejs",
		"new_book_placeholder":           "np. Tajemnica Starego Młyna",
		"create_screenplay_btn":          "Utwórz scenariusz",
		"create_book_btn":                "Utwórz książkę",
		"sp_ins_character":               "POSTAĆ",
		"sp_ins_dialogue":                "Kwestia dialogowa...",
		"sp_ins_paren":                   "(cicho)\n",
		"sp_ins_paren_sel":               "cicho",
		"sp_ins_action":                  "Opis akcji...",
		"sp_ins_off_hdr":                 " (OFF)\n",
		"sp_ins_off_line":                "Kwestia spoza kadru...",
		"sp_ins_vo_hdr":                  " (V.O.)\n",
		"sp_ins_vo_line":                 "Kwestia narratora...",
		"sp_ins_cut":                     "> CIĘCIE DO:\n",
		"sp_ins_fade":                    "> ŚCIEMNIENIE.\n",
		"sp_ins_subslug":                 "WN. LOKACJA - CIĄG DALSZY\n",
		"sp_ins_location":                "LOKACJA",
		"err_missing_val_after_op":       "Błąd składni: brak wartości po operatorze '%s' dla zmiennej '%s'",
		"err_int_type":                   "Błąd typu: zmienna '%s' (int) wymaga liczby całkowitej, otrzymano '%s'",
		"err_enum_op":                    "Błąd operatora: zmienna '%s' (enum) obsługuje tylko '=' / '==' lub '!='",
		"err_enum_val":                   "Nieznana wartość '%s' dla zmiennej '%s' (dozwolone: %s)",
		"err_bool_op":                    "Błąd operatora: zmienna '%s' (bool) obsługuje tylko '=' / '==' lub '!='",
		"err_bool_type":                  "Błąd typu: zmienna '%s' (bool) wymaga wartości true/false lub samej nazwy zmiennej",
		"err_no_vars":                    "Brak zadeklarowanych zmiennych w książce",
		"err_unclosed_paren":             "Błąd składni: niedomknięty nawias w warunku",
		"err_empty_after_logic_op":       "Błąd składni: brak wyrażenia po operatorze logicznym",
		"err_mut_op_in_if":               "Błąd w if: operator '%s' służy do zmiany wartości w polu set:, a w if: podaj porównanie",
		"err_comma_in_if":                "Błąd składni w if: użyj '&&' (oraz) lub '||' (lub) zamiast przecinka/średnika",
		"err_missing_var_before_op":      "Błąd składni: brak nazwy zmiennej przed operatorem '%s'",
		"err_unknown_var":                "Nieznana zmienna '%s' — zadeklaruj ją w panelu Zmienne {}",
		"err_cond_syntax":                "Błąd składni w warunku: '%s'",
		"err_set_syntax_empty":           "Błąd składni w polu set:",
		"err_set_empty_after_comma":      "Błąd składni w set: puste wyrażenie po przecinku",
		"err_op_requires_int":            "Błąd typu: operator '%s' wymaga zmiennej typu int (zmienna '%s' jest typu %s)",
		"err_neg_requires_bool":          "Błąd typu: negacja '!%s' wymaga zmiennej typu bool (jest typu %s)",
		"err_set_clause_syntax":          "Błąd składni w set: '%s'",
		"err_enum_needs_assign":          "Błąd typu: dla zmiennej '%s' (enum) użyj przypisania '='",
		"err_var_requires_int":           "Błąd typu: zmienna '%s' wymaga liczby całkowitej",
		"err_cond_op_in_set":             "Błąd w set: użyj przypisania ('=', '+=', '-=', '++', '--'), a nie operatora warunkowego",
		"err_missing_val_after_compound": "Błąd składni: brak wartości po '%s' dla zmiennej '%s'",
		"err_int_after_compound":         "Błąd typu: zmienna '%s' (int) wymaga liczby całkowitej po '%s', otrzymano '%s'",
		"err_missing_val_after_eq":       "Błąd składni: brak przypisywanej wartości po '=' dla zmiennej '%s'",
		"err_bare_var_needs_assign":      "Błąd składni: dla zmiennej '%s' (%s) podaj przypisanie (np. %s = wartość)",
	},
	LangEN: {
		"fullscreen_exit_hint":           "Exit fullscreen",
		"fullscreen_enter_hint":          "Toggle fullscreen",
		"scale_zen_hint":                 "Zen mode — content only",
		"scale_exit_zen_hint":            "Exit Zen mode (small UI)",
		"scale_enlarge_hint":             "Enlarge buttons and controls (+ / Ctrl++)",
		"imageless_locked_hint":          "Images disabled in book settings (imageless: true)",
		"imageless_enable_hint":          "Enable images (imageless mode active)",
		"imageless_disable_hint":         "Disable images (centered text mode)",
		"auto_choice_on":                 "Auto-Choice: ON",
		"auto_choice_off":                "Auto-Choice: OFF",
		"scene_text_placeholder":         "Enter scene text...",
		"choice_prefix":                  "Choice ",
		"no_scene_audio_toast":           "This scene has no assigned audio file.",
		"entry_badge_upper":              "ENTRY",
		"next":                           "Next",
		"default_chapter":                "Chapter 1",
		"selected_count_prefix":          "Selected: ",
		"selected_scenes_prefix":         "Selected scenes (",
		"delete_selected_scenes_prefix":  "Delete selected scenes (",
		"new_screenplay_modal_title":     "Create New Screenplay",
		"new_book_modal_title":           "Create New Book",
		"title_key":                      "title:",
		"new_screenplay_placeholder":     "e.g. The Last Voyage",
		"new_book_placeholder":           "e.g. Mystery of the Old Mill",
		"create_screenplay_btn":          "Create screenplay",
		"create_book_btn":                "Create book",
		"sp_ins_character":               "CHARACTER",
		"sp_ins_dialogue":                "Dialogue line...",
		"sp_ins_paren":                   "(quietly)\n",
		"sp_ins_paren_sel":               "quietly",
		"sp_ins_action":                  "Action description...",
		"sp_ins_off_hdr":                 " (O.S.)\n",
		"sp_ins_off_line":                "Off-screen line...",
		"sp_ins_vo_hdr":                  " (V.O.)\n",
		"sp_ins_vo_line":                 "Voice-over line...",
		"sp_ins_cut":                     "> CUT TO:\n",
		"sp_ins_fade":                    "> FADE OUT.\n",
		"sp_ins_subslug":                 "INT. LOCATION - CONTINUOUS\n",
		"sp_ins_location":                "LOCATION",
		"err_missing_val_after_op":       "Syntax error: missing value after operator '%s' for variable '%s'",
		"err_int_type":                   "Type error: variable '%s' (int) requires an integer, got '%s'",
		"err_enum_op":                    "Operator error: variable '%s' (enum) only supports '=' / '==' or '!='",
		"err_enum_val":                   "Unknown value '%s' for variable '%s' (allowed: %s)",
		"err_bool_op":                    "Operator error: variable '%s' (bool) only supports '=' / '==' or '!='",
		"err_bool_type":                  "Type error: variable '%s' (bool) requires true/false or a variable name",
		"err_no_vars":                    "No variables declared in book",
		"err_unclosed_paren":             "Syntax error: unclosed parenthesis in condition",
		"err_empty_after_logic_op":       "Syntax error: missing expression after logical operator",
		"err_mut_op_in_if":               "Error in if: operator '%s' mutates values in set:, use a comparison in if:",
		"err_comma_in_if":                "Syntax error in if: use '&&' (and) or '||' (or) instead of comma/semicolon",
		"err_missing_var_before_op":      "Syntax error: missing variable name before operator '%s'",
		"err_unknown_var":                "Unknown variable '%s' — declare it in the Variables {} panel",
		"err_cond_syntax":                "Syntax error in condition: '%s'",
		"err_set_syntax_empty":           "Syntax error in set: field",
		"err_set_empty_after_comma":      "Syntax error in set: empty expression after comma",
		"err_op_requires_int":            "Type error: operator '%s' requires an int variable ('%s' is %s)",
		"err_neg_requires_bool":          "Type error: negation '!%s' requires a bool variable (is %s)",
		"err_set_clause_syntax":          "Syntax error in set: '%s'",
		"err_enum_needs_assign":          "Type error: use '=' assignment for enum variable '%s'",
		"err_var_requires_int":           "Type error: variable '%s' requires an integer",
		"err_cond_op_in_set":             "Error in set: use assignment ('=', '+=', '-=', '++', '--'), not a comparison operator",
		"err_missing_val_after_compound": "Syntax error: missing value after '%s' for variable '%s'",
		"err_int_after_compound":         "Type error: variable '%s' (int) requires an integer after '%s', got '%s'",
		"err_missing_val_after_eq":       "Syntax error: missing assigned value after '=' for variable '%s'",
		"err_bare_var_needs_assign":      "Syntax error: variable '%s' (%s) requires an assignment (e.g. %s = value)",
	},
}

// JSI18nTable is the compiled table of client-side UI strings used in app/js/*.js.
// Each entry is [1 + LangCount]string{key, lang0, lang1, ...} with fallback to LangEN then LangPL.
var JSI18nTable [len(jsI18nKeys)][1 + int(LangCount)]string

var cachedJSI18nHeader string

// LookupJSI18n resolves a key in jsI18nDicts for the given language with fallback to LangEN then LangPL.
func LookupJSI18n(lang Lang, key string) string {
	if lang < LangCount {
		if v := jsI18nDicts[lang][key]; v != "" {
			return v
		}
	}
	if v := jsI18nDicts[LangEN][key]; v != "" {
		return v
	}
	return jsI18nDicts[LangPL][key]
}

// HasJSI18nKeyInLang reports whether the language dictionary explicitly defines a non-empty translation for key.
func HasJSI18nKeyInLang(lang Lang, key string) bool {
	if lang >= LangCount {
		return false
	}
	return strings.TrimSpace(jsI18nDicts[lang][key]) != ""
}

func init() {
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

