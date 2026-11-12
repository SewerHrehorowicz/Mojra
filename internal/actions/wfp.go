package actions

import (
	"bytes"
	"net/url"
	"sort"
	"strings"
)

var (
	wfpSepLF   = []byte("\n\n")
	wfpSepCRLF = []byte("\r\n\r\n")
)

// EscapeWFPLineVal escapes backslashes and line breaks for single-line WFP header values.
func EscapeWFPLineVal(s string) string {
	if !strings.ContainsAny(s, "\n\r\\") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func unescapeWFPLineVal(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			default:
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// EncodeWFPForm serializes key/value parameters into a WFP wire frame (key=val lines + optional \n\n raw body).
func EncodeWFPForm(vals url.Values) []byte {
	if len(vals) == 0 {
		return nil
	}
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var bodyKey, bodyVal string
	for _, k := range keys {
		v := vals.Get(k)
		if strings.ContainsAny(v, "\n\r") {
			if bodyKey == "" || k == "text" {
				bodyKey = k
				bodyVal = v
			}
		}
	}

	var b bytes.Buffer
	b.Grow(128 + len(bodyVal))
	if act := vals.Get("action"); act != "" {
		b.WriteString("action=")
		b.WriteString(EscapeWFPLineVal(act))
		b.WriteByte('\n')
	}
	for _, k := range keys {
		if k == "action" {
			continue
		}
		v := vals.Get(k)
		if k == bodyKey {
			b.WriteString("_body=")
			b.WriteString(k)
			b.WriteByte('\n')
			continue
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(EscapeWFPLineVal(v))
		b.WriteByte('\n')
	}
	if bodyKey != "" {
		b.WriteByte('\n')
		b.WriteString(bodyVal)
	} else if b.Len() > 0 && b.Bytes()[b.Len()-1] == '\n' {
		b.Truncate(b.Len() - 1)
	}
	return b.Bytes()
}

// DecodeWFPFormBytes decodes a WFP wire frame (key=val header lines + optional \n\n raw body) into dst.
func DecodeWFPFormBytes(raw []byte, dst url.Values) bool {
	if len(raw) == 0 || dst == nil {
		return false
	}
	head := raw
	var body []byte
	hasBody := false
	if idx := bytes.Index(raw, wfpSepLF); idx >= 0 {
		head = raw[:idx]
		body = raw[idx+2:]
		hasBody = true
	} else if idx := bytes.Index(raw, wfpSepCRLF); idx >= 0 {
		head = raw[:idx]
		body = raw[idx+4:]
		hasBody = true
	}

	var bodyKey string
	hasKV := false
	rem := head
	for len(rem) > 0 {
		var line []byte
		if nl := bytes.IndexByte(rem, '\n'); nl >= 0 {
			line = rem[:nl]
			rem = rem[nl+1:]
		} else {
			line = rem
			rem = nil
		}
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if len(line) == 0 {
			continue
		}
		eq := bytes.IndexByte(line, '=')
		if eq <= 0 {
			return false
		}
		hasKV = true
		k := string(bytes.TrimSpace(line[:eq]))
		v := line[eq+1:]
		if k == "_body" {
			bodyKey = strings.TrimSpace(string(v))
			continue
		}
		dst[k] = []string{unescapeWFPLineVal(string(v))}
	}
	if !hasKV {
		return false
	}
	if bodyKey != "" && (hasBody || len(body) > 0) {
		dst[bodyKey] = []string{string(body)}
	} else if bodyKey != "" {
		if _, exists := dst[bodyKey]; !exists {
			dst[bodyKey] = []string{""}
		}
	}
	return true
}
