package engine

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"mojra/engine/templates"
)

const (
	ContentTypeFDP       = "text/x-fdp;charset=UTF-8"
	fdpKeyController     = "controller"
	fdpKeyTarget         = "target"
	fdpTargetNone        = "none"
	fdpKeyBody           = "_body"
	fdpBodyHTML          = "html"
	scriptStructSuffix   = "Script"
	templateStructSuffix = "Template"
	controllerNameSuffix = "Controller"
	fdpFrameSep          = "\n---\n"

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
	fdpSepLF         = []byte("\n\n")
	fdpSepCRLF       = []byte("\r\n\r\n")
	fdpFrameSepBytes = []byte(fdpFrameSep)
)

// FDPFrame represents a single controller-isolated frame in a multi-frame FDP payload.
type FDPFrame struct {
	Target   string
	HTMLBody []byte
	Chunk    templates.TemplateChunk
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

// DecodeFDPFrames splits a multi-frame FDP payload on \n---\n and decodes each frame into url.Values.
func DecodeFDPFrames(raw []byte) []url.Values {
	if len(raw) == 0 {
		return nil
	}
	parts := bytes.Split(raw, fdpFrameSepBytes)
	out := make([]url.Values, 0, len(parts))
	for _, part := range parts {
		if len(bytes.TrimSpace(part)) == 0 {
			continue
		}
		vals := make(url.Values, 16)
		if DecodeFDPBytes(part, vals) {
			out = append(out, vals)
		}
	}
	return out
}

func formatControllerName(stem string) string {
	if stem == "" {
		return ""
	}
	return strings.ToUpper(stem[:1]) + stem[1:] + controllerNameSuffix
}

func scriptControllerName(script any) string {
	if script == nil {
		return ""
	}
	rt := reflect.TypeOf(script)
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	name := strings.TrimSuffix(rt.Name(), scriptStructSuffix)
	return formatControllerName(name)
}

func chunkControllerName(chunk templates.TemplateChunk) string {
	if chunk.ScriptData != nil {
		if c := scriptControllerName(chunk.ScriptData); c != "" {
			return c
		}
	}
	if chunk.TemplateData != nil {
		rt := reflect.TypeOf(chunk.TemplateData)
		for rt.Kind() == reflect.Pointer {
			rt = rt.Elem()
		}
		name := strings.TrimSuffix(rt.Name(), templateStructSuffix)
		return formatControllerName(name)
	}
	return ""
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

// WriteFDPFrames serializes 1..N controller-isolated FDP frames separated by \n---\n.
func WriteFDPFrames(w io.Writer, frames ...FDPFrame) error {
	buf := templates.AcquireBuffer()
	defer templates.ReleaseBuffer(buf)

	expanded := make([]FDPFrame, 0, len(frames)+4)
	seenCtrl := make(map[string]bool, len(frames)+4)

	for _, f := range frames {
		ctrl := chunkControllerName(f.Chunk)
		expanded = append(expanded, f)
		if ctrl != "" {
			seenCtrl[ctrl] = true
		}

		if f.Chunk.TemplateData != nil {
			queue := collectChildChunks(f.Chunk.TemplateData, nil)
			for step := 0; step < maxDataWalkSteps && len(queue) > 0; step++ {
				cur := queue[0]
				queue = queue[1:]
				if cur.ScriptData != nil {
					childCtrl := scriptControllerName(cur.ScriptData)
					if childCtrl != "" && !seenCtrl[childCtrl] {
						seenCtrl[childCtrl] = true
						expanded = append(expanded, FDPFrame{
							Target: fdpTargetNone,
							Chunk:  templates.TemplateChunk{ScriptData: cur.ScriptData},
						})
					}
				}
				if cur.TemplateData != nil {
					queue = collectChildChunks(cur.TemplateData, queue)
				}
			}
		}
	}

	for i, f := range expanded {
		if i > 0 {
			buf.WriteString(fdpFrameSep)
		}
		if ctrl := chunkControllerName(f.Chunk); ctrl != "" {
			writeFDPLine(buf, fdpKeyController, ctrl)
		}
		if f.Target != "" {
			writeFDPLine(buf, fdpKeyTarget, f.Target)
		}
		if f.Chunk.ScriptData != nil {
			writeFDPScriptValue(buf, f.Chunk.ScriptData)
		}
		if len(f.HTMLBody) > 0 {
			writeFDPLine(buf, fdpKeyBody, fdpBodyHTML)
			buf.WriteByte('\n')
			buf.Write(bytes.TrimRight(f.HTMLBody, fdpLineBreakChars))
		} else if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] == '\n' {
			buf.Truncate(buf.Len() - 1)
		}
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func writeFDPScriptValue(buf *bytes.Buffer, script any) {
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

func collectChildChunks(tplData any, queue []templates.TemplateChunk) []templates.TemplateChunk {
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
	var singleChunks []templates.TemplateChunk
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
		case templates.TemplateChunk:
			singleChunks = append(singleChunks, typed)
		case []templates.TemplateChunk:
			queue = append(queue, typed...)
		}
	}
	return append(queue, singleChunks...)
}
