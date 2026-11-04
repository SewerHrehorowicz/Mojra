package gui

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
)

var wfpBufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

// AcquireWFPBuffer obtains a pooled bytes.Buffer for zero-allocation WFP request decoding.
func AcquireWFPBuffer() *bytes.Buffer {
	b := wfpBufPool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

// ReleaseWFPBuffer returns a bytes.Buffer to the WFP pool if within size limits.
func ReleaseWFPBuffer(b *bytes.Buffer) {
	if b != nil && b.Cap() <= 1<<20 {
		b.Reset()
		wfpBufPool.Put(b)
	}
}

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

func isWFPRequestPayload(contentType string, raw []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "wfp") || strings.HasPrefix(ct, "text/plain") {
		return true
	}
	if bytes.IndexByte(raw, '\n') >= 0 {
		return true
	}
	if bytes.IndexByte(raw, '&') < 0 && bytes.IndexByte(raw, '%') < 0 && bytes.IndexByte(raw, '+') < 0 {
		return true
	}
	return false
}

// ParseActionRequest populates r.Form and r.PostForm from a WFP wire frame (or URL-encoded fallback) using pooled buffers.
func ParseActionRequest(r *http.Request) {
	if r == nil {
		return
	}
	if r.Form == nil {
		r.Form = make(url.Values, 8)
	}
	if r.URL != nil && r.URL.RawQuery != "" {
		if qVals, err := url.ParseQuery(r.URL.RawQuery); err == nil {
			for k, v := range qVals {
				r.Form[k] = v
			}
		}
	}
	if r.Body == nil {
		r.PostForm = r.Form
		return
	}
	buf := AcquireWFPBuffer()
	defer ReleaseWFPBuffer(buf)
	_, _ = buf.ReadFrom(io.LimitReader(r.Body, 4<<20))
	raw := buf.Bytes()
	if len(raw) == 0 {
		r.PostForm = r.Form
		return
	}
	if isWFPRequestPayload(r.Header.Get("Content-Type"), raw) {
		if DecodeWFPFormBytes(raw, r.Form) {
			r.PostForm = r.Form
			return
		}
	}
	if formVals, err := url.ParseQuery(string(raw)); err == nil {
		for k, v := range formVals {
			r.Form[k] = v
		}
	}
	r.PostForm = r.Form
}
