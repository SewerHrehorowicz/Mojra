package model

import (
	"strconv"
	"strings"
	"unicode"
)

const (
	VarKindBool = "bool"
	VarKindInt  = "int"
	VarKindEnum = "enum"

	boolTrueLiteral  = "true"
	boolFalseLiteral = "false"
	defaultEnumState = "stan_1"
	ifPrefixLower    = "if:"
	doubleBraceOpen  = "{{"
	doubleBraceClose = "}}"
)

// EnumDef defines a named set of allowed string options for enum variables in book.playtxt.
type EnumDef struct {
	Name   string
	Values []string
}

// VariableDef defines a persistent story variable declared in book.playtxt.
type VariableDef struct {
	Idx          int
	Name         string
	Kind         string // "bool", "int", "enum"
	Meta         string
	BoolVal      bool
	BoolLabel    string
	IntVal       int
	EnumName     string
	EnumVal      string
	Value        string
	Options      []string
	ValuesStr    string
	OptionItems  []SelectOption
	EnumOptions  []SelectOption
	OptCount     int
	CanRemoveOpt bool
	DefaultOpt   string
	CurrentVal   string
	IsBool       bool
	IsInt        bool
	IsEnum       bool
	IsChecked    bool
}

// DefaultValueString returns the canonical string representation of a VariableDef's starting value.
func (varDef VariableDef) DefaultValueString() string {
	switch varDef.Kind {
	case VarKindBool:
		if varDef.BoolVal {
			return boolTrueLiteral
		}
		return boolFalseLiteral
	case VarKindInt:
		return strconv.Itoa(varDef.IntVal)
	case VarKindEnum:
		if varDef.EnumVal != "" {
			return varDef.EnumVal
		}
		if len(varDef.Options) > 0 {
			return varDef.Options[0]
		}
		return defaultEnumState
	default:
		return boolFalseLiteral
	}
}

// SanitizeVarName normalizes a variable or enum identifier (lowercase ASCII/Unicode letters, digits, underscores).
func SanitizeVarName(rawName string) string {
	trimmed := strings.TrimSpace(rawName)
	trimmed = strings.TrimPrefix(trimmed, "$")
	var builder strings.Builder
	for _, charRune := range trimmed {
		if (charRune >= 'a' && charRune <= 'z') || (charRune >= '0' && charRune <= '9') || charRune == '_' {
			builder.WriteRune(charRune)
		} else if charRune >= 'A' && charRune <= 'Z' {
			builder.WriteRune(unicode.ToLower(charRune))
		} else if unicode.IsLetter(charRune) || unicode.IsDigit(charRune) {
			builder.WriteRune(unicode.ToLower(charRune))
		} else if (charRune == ' ' || charRune == '-') && builder.Len() > 0 {
			last := builder.String()
			if !strings.HasSuffix(last, "_") {
				builder.WriteByte('_')
			}
		}
	}
	return strings.Trim(builder.String(), "_")
}

// SanitizeEnumOption normalizes a single enum state token.
func SanitizeEnumOption(rawOption string) string {
	cleaned := strings.TrimSpace(SanitizePlainText(rawOption))
	cleaned = strings.Trim(cleaned, `"'`)
	return SanitizeVarName(cleaned)
}

// SanitizeRuleExpr sanitizes a condition or effect expression while preserving comparison operators (<, >, <=, >=).
func SanitizeRuleExpr(rawExpr string) string {
	cleaned := strings.ReplaceAll(rawExpr, "\x00", "")
	cleaned = plainHTMLTagStripRe.ReplaceAllString(cleaned, "")
	var builder strings.Builder
	builder.Grow(len(cleaned))
	for _, charRune := range cleaned {
		if charRune == '\r' || charRune == '\n' {
			builder.WriteByte(' ')
		} else if charRune == '\t' || charRune >= 0x20 {
			builder.WriteRune(charRune)
		}
	}
	return strings.TrimSpace(builder.String())
}

// StripOuterDoubleBraces removes optional outer {{ and }} around an expression.
func StripOuterDoubleBraces(rawExpr string) string {
	cleaned := SanitizeRuleExpr(rawExpr)
	if strings.HasPrefix(cleaned, doubleBraceOpen) && strings.HasSuffix(cleaned, doubleBraceClose) && len(cleaned) >= 4 {
		inner := strings.TrimSpace(cleaned[2 : len(cleaned)-2])
		if !strings.Contains(inner, doubleBraceClose) {
			return inner
		}
	}
	return cleaned
}

// CleanConditionExpr normalizes a condition expression by stripping optional {{...}} or leading "if:" prefix.
func CleanConditionExpr(rawExpr string) string {
	cleaned := StripOuterDoubleBraces(rawExpr)
	lower := strings.ToLower(cleaned)
	if strings.HasPrefix(lower, ifPrefixLower) {
		cleaned = strings.TrimSpace(cleaned[3:])
	}
	cleaned = strings.TrimSuffix(strings.TrimSpace(cleaned), ";")
	return strings.TrimSpace(cleaned)
}
