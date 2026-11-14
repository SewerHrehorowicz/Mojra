package tpl

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

const (
	ContentTypeFDP   = "text/x-fdp;charset=UTF-8"
	fdpKeyAction     = "action"
	fdpKeyText       = "text"
	fdpKeyTarget     = "target"
	fdpKeyKeepScroll = "keepScroll"
	fdpKeyResetCanv  = "resetCanvas"
	fdpKeyToast      = "toast"
	fdpKeyNativeCmd  = "nativeCmd"
	fdpKeyVoicePeak  = "voicePeak"
	fdpKeySaveHex    = "saveHex"
	fdpKeyModalID    = "modalId"
	fdpKeyBody       = "_body"
	fdpBodyHTML      = "html"

	trueLiteral      = "true"
	falseLiteral     = "false"
	floatFmt         = "%.4g"
	maxDataWalkSteps = 512

	fdpLineBreakChars = "\n\r"
	fdpEscapeChars    = "\n\r\\"
	fdpEscNL          = `\n`
	fdpEscCR          = `\r`
	fdpEscBS          = `\\`
)

var (
	fdpSepLF   = []byte("\n\n")
	fdpSepCRLF = []byte("\r\n\r\n")
)

// FDPFrameMeta holds transport-level metadata serialized into the flat FDP header block.
type FDPFrameMeta struct {
	Target      string
	KeepScroll  bool
	ResetCanvas bool
	Toast       string
	NativeCmd   string
	VoicePeak   string
	SaveHex     string
	ModalID     string
}

// EscapeFDPLineVal escapes backslashes and line breaks for single-line FDP header values.
func EscapeFDPLineVal(s string) string {
	if !strings.ContainsAny(s, fdpEscapeChars) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			b.WriteString(fdpEscNL)
		case '\r':
			b.WriteString(fdpEscCR)
		case '\\':
			b.WriteString(fdpEscBS)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// UnescapeFDPLineVal unescapes backslashes and line breaks from a single-line FDP header value.
func UnescapeFDPLineVal(s string) string {
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

// EncodeFDPForm serializes key/value parameters into an FDP wire frame (key=val lines + optional \n\n raw body).
func EncodeFDPForm(vals url.Values) []byte {
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
		if strings.ContainsAny(v, fdpLineBreakChars) {
			if bodyKey == "" || k == fdpKeyText || k == fdpBodyHTML {
				bodyKey = k
				bodyVal = v
			}
		}
	}
	if bodyKey == "" {
		if htmlVal := vals.Get(fdpBodyHTML); htmlVal != "" {
			bodyKey = fdpBodyHTML
			bodyVal = htmlVal
		}
	}

	var b bytes.Buffer
	b.Grow(128 + len(bodyVal))
	if act := vals.Get(fdpKeyAction); act != "" {
		b.WriteString(fdpKeyAction)
		b.WriteByte('=')
		b.WriteString(EscapeFDPLineVal(act))
		b.WriteByte('\n')
	}
	for _, k := range keys {
		if k == fdpKeyAction {
			continue
		}
		v := vals.Get(k)
		if k == bodyKey {
			b.WriteString(fdpKeyBody)
			b.WriteByte('=')
			b.WriteString(k)
			b.WriteByte('\n')
			continue
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(EscapeFDPLineVal(v))
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

// DecodeFDPBytes decodes an FDP wire frame (key=val header lines + optional \n\n raw body) into dst.
func DecodeFDPBytes(raw []byte, dst url.Values) bool {
	if len(raw) == 0 || dst == nil {
		return false
	}
	head := raw
	var body []byte
	hasBody := false
	if idx := bytes.Index(raw, fdpSepLF); idx >= 0 {
		head = raw[:idx]
		body = raw[idx+2:]
		hasBody = true
	} else if idx := bytes.Index(raw, fdpSepCRLF); idx >= 0 {
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
		if k == fdpKeyBody {
			bodyKey = strings.TrimSpace(string(v))
			continue
		}
		dst[k] = []string{UnescapeFDPLineVal(string(v))}
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

// DecodeFDPResponse parses an FDP response payload into header key-values and the HTML body.
func DecodeFDPResponse(raw []byte) (url.Values, string) {
	vals := make(url.Values, 16)
	if !DecodeFDPBytes(raw, vals) {
		return vals, string(raw)
	}
	return vals, vals.Get(fdpBodyHTML)
}

func writeFDPLine(buf *bytes.Buffer, key, val string) {
	if key == "" {
		return
	}
	buf.WriteString(key)
	buf.WriteByte('=')
	buf.WriteString(EscapeFDPLineVal(val))
	buf.WriteByte('\n')
}

func writeFDPBoolLine(buf *bytes.Buffer, key string, val bool) {
	if val {
		writeFDPLine(buf, key, trueLiteral)
	} else {
		writeFDPLine(buf, key, falseLiteral)
	}
}

// WriteFDPResponse serializes meta, all non-nil ScriptData across the chunks tree, and optional htmlBody as a flat FDP frame.
func WriteFDPResponse(w io.Writer, meta FDPFrameMeta, htmlBody []byte, chunks ...TemplateChunk) error {
	buf := AcquireBuffer()
	defer ReleaseBuffer(buf)

	if meta.Target != "" {
		writeFDPLine(buf, fdpKeyTarget, meta.Target)
	}
	if meta.KeepScroll {
		writeFDPLine(buf, fdpKeyKeepScroll, trueLiteral)
	}
	if meta.ResetCanvas {
		writeFDPLine(buf, fdpKeyResetCanv, trueLiteral)
	}
	if meta.Toast != "" {
		writeFDPLine(buf, fdpKeyToast, meta.Toast)
	}
	if meta.NativeCmd != "" {
		writeFDPLine(buf, fdpKeyNativeCmd, meta.NativeCmd)
	}
	if meta.VoicePeak != "" {
		writeFDPLine(buf, fdpKeyVoicePeak, meta.VoicePeak)
	}
	if meta.SaveHex != "" {
		writeFDPLine(buf, fdpKeySaveHex, meta.SaveHex)
	}
	if meta.ModalID != "" {
		writeFDPLine(buf, fdpKeyModalID, meta.ModalID)
	}

	queue := make([]TemplateChunk, 0, len(chunks)+8)
	queue = append(queue, chunks...)
	for step := 0; step < maxDataWalkSteps && len(queue) > 0; step++ {
		cur := queue[0]
		queue = queue[1:]
		if cur.ScriptData != nil {
			writeFDPScriptValue(buf, cur.ScriptData)
		}
		tplData := cur.TemplateData
		if tplData == nil {
			tplData = cur.Data
		}
		if tplData != nil {
			queue = collectChildChunks(tplData, queue)
		}
	}

	if len(htmlBody) > 0 {
		writeFDPLine(buf, fdpKeyBody, fdpBodyHTML)
		buf.WriteByte('\n')
		buf.Write(htmlBody)
	} else if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] == '\n' {
		buf.Truncate(buf.Len() - 1)
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func writeFDPScriptValue(buf *bytes.Buffer, script any) {
	if m, ok := script.(map[string]any); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			switch typed := m[k].(type) {
			case bool:
				writeFDPBoolLine(buf, k, typed)
			case int:
				writeFDPLine(buf, k, strconv.Itoa(typed))
			case float64:
				writeFDPLine(buf, k, fmt.Sprintf(floatFmt, typed))
			case string:
				writeFDPLine(buf, k, typed)
			}
		}
		return
	}
	rv := reflect.ValueOf(script)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return
	}
	rt := rv.Type()
	n := rt.NumField()
	for i := 0; i < n; i++ {
		sf := rt.Field(i)
		fv := rv.Field(i)
		key := sf.Name
		switch fv.Kind() {
		case reflect.Bool:
			writeFDPBoolLine(buf, key, fv.Bool())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			writeFDPLine(buf, key, strconv.FormatInt(fv.Int(), 10))
		case reflect.Float32, reflect.Float64:
			writeFDPLine(buf, key, fmt.Sprintf(floatFmt, fv.Float()))
		case reflect.String:
			writeFDPLine(buf, key, fv.String())
		}
	}
}

func collectChildChunks(tplData any, queue []TemplateChunk) []TemplateChunk {
	if m, ok := tplData.(map[string]any); ok {
		var singleChunks []TemplateChunk
		for _, v := range m {
			switch typed := v.(type) {
			case TemplateChunk:
				singleChunks = append(singleChunks, typed)
			case []TemplateChunk:
				queue = append(queue, typed...)
			}
		}
		return append(queue, singleChunks...)
	}
	rv := reflect.ValueOf(tplData)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return queue
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return queue
	}
	var singleChunks []TemplateChunk
	rt := rv.Type()
	n := rt.NumField()
	for i := 0; i < n; i++ {
		sf := rt.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		fv := rv.Field(i)
		if !fv.CanInterface() {
			continue
		}
		switch typed := fv.Interface().(type) {
		case TemplateChunk:
			singleChunks = append(singleChunks, typed)
		case []TemplateChunk:
			queue = append(queue, typed...)
		}
	}
	return append(queue, singleChunks...)
}
