package book

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"mojra/internal/tpl"
)

var (
	htmlTagStripRe = regexp.MustCompile(`<[a-zA-Z/!?][^>]*>`)
	safeIDRe       = regexp.MustCompile(`^[\p{L}\p{N}_\-]+$`)
	headerKeyRe    = regexp.MustCompile(`^[a-zA-ZąćęłńóśźżĄĆĘŁŃÓŚŹŻ_]+$`)
)

// SanitizePlainText strips any HTML tags, control characters, and dangerous payloads
// so book text and metadata are guaranteed to be pure plain text without executable code.
func SanitizePlainText(s string) string {
	return tpl.SanitizePlainText(s)
}

// SanitizeChapterID normalizes a chapter title or ID into a clean lowercase ASCII/underscore identifier
// suitable for chapter:<id> links and <chapter_id>.txt filenames (e.g. "Rozdział 2" -> "rozdzial_2").
func SanitizeChapterID(raw string) string {
	cleaned := strings.ToLower(strings.TrimSpace(SanitizePlainText(raw)))
	if cleaned == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"ą", "a",
		"ć", "c",
		"ę", "e",
		"ł", "l",
		"ń", "n",
		"ó", "o",
		"ś", "s",
		"ź", "z",
		"ż", "z",
	)
	cleaned = replacer.Replace(cleaned)
	var b strings.Builder
	lastUnderscore := false
	for _, r := range cleaned {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
			lastUnderscore = false
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
		} else if !lastUnderscore && b.Len() > 0 {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_-")
}

// SanitizeSceneID normalizes a user-provided scene ID so it can be used safely in links and DOM attributes.
func SanitizeSceneID(raw string) string {
	cleaned := strings.TrimSpace(SanitizePlainText(raw))
	if cleaned == "" {
		return ""
	}
	cleaned = strings.ReplaceAll(cleaned, " ", "_")
	if !safeIDRe.MatchString(cleaned) {
		var b strings.Builder
		for _, r := range cleaned {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
				b.WriteRune(r)
			}
		}
		cleaned = strings.Trim(b.String(), "_-")
	}
	return cleaned
}

// SanitizeRelativeMediaPath normalizes a relative path (supporting subdirectories like "images/scene.png" or "voice/ch1/s1.ogg")
// while rejecting path traversal ("..") or absolute paths.
func SanitizeRelativeMediaPath(raw string) string {
	cleaned := strings.TrimSpace(SanitizePlainText(raw))
	if cleaned == "" {
		return ""
	}
	slashed := filepath.ToSlash(strings.ReplaceAll(cleaned, "\\", "/"))
	slashed = strings.TrimPrefix(slashed, "./")
	slashed = strings.TrimLeft(slashed, "/")
	if slashed == "" || strings.Contains(slashed, "..") || strings.Contains(slashed, ":") {
		return ""
	}
	parts := strings.Split(slashed, "/")
	var safeParts []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "." || p == ".." {
			return ""
		}
		safeParts = append(safeParts, p)
	}
	return strings.Join(safeParts, "/")
}

// NormalizeLookupKey normalizes a scene or file name for case-insensitive matching (no numeric prefix magic).
func NormalizeLookupKey(s string) string {
	s = strings.TrimSpace(s)
	return strings.ToLower(s)
}

// SanitizeBookID produces a safe URL/filename slug.
func SanitizeBookID(s string) string {
	s = strings.ToLower(SanitizePlainText(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	res := strings.Trim(b.String(), "-")
	if res == "" {
		return "book"
	}
	return res
}

// RenderSafeProseHTML converts plain text into safe HTML paragraphs (`<p>`, `<strong>`, `<em>`, `<blockquote>`, `<hr>`)
// after escaping all HTML special characters.
func RenderSafeProseHTML(raw string) string {
	return tpl.RenderSafeProseHTML(raw)
}

func hasDisallowedMediaSignature(lowerProbe string) bool {
	for _, sig := range []string{"<!doctype", "<html", "<script", "<svg", "<?xml", "#!/", "\x7felf"} {
		if strings.Contains(lowerProbe, sig) {
			return true
		}
	}
	return false
}
