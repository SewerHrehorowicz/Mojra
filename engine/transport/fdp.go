package transport

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
	fdpKeyController     = "controller"
	fdpKeyTarget         = "target"
	fdpTargetNone        = "none"
	fdpKeyBody           = "_body"
	fdpBodyHTML          = "html"
	scriptStructSuffix   = "Script"
	templateStructSuffix = "Template"
	controllerNameSuffix = "Controller"
	fdpFrameSep          = "\n---\n"
	upperAcronymUI       = "ui"
	upperAcronymQR       = "qr"

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

// EscapeFDPLineVal escapes backslashes and line breaks for single-line FDP header values.
func EscapeFDPLineVal(rawVal string) string {
	if !strings.ContainsAny(rawVal, fdpEscapeChars) {
		return rawVal
	}
	var out strings.Builder
	out.Grow(len(rawVal) + 8)
	for idx := 0; idx < len(rawVal); idx++ {
		switch rawVal[idx] {
		case '\n':
			out.WriteString(fdpEscNL)
		case '\r':
			out.WriteString(fdpEscCR)
		case '\\':
			out.WriteString(fdpEscBS)
		default:
			out.WriteByte(rawVal[idx])
		}
	}
	return out.String()
}

// UnescapeFDPLineVal unescapes backslashes and line breaks from a single-line FDP header value.
func UnescapeFDPLineVal(rawVal string) string {
	if !strings.ContainsRune(rawVal, '\\') {
		return rawVal
	}
	var out strings.Builder
	out.Grow(len(rawVal))
	for idx := 0; idx < len(rawVal); idx++ {
		if rawVal[idx] == '\\' && idx+1 < len(rawVal) {
			idx++
			switch rawVal[idx] {
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case '\\':
				out.WriteByte('\\')
			case '"':
				out.WriteByte('"')
			default:
				out.WriteByte('\\')
				out.WriteByte(rawVal[idx])
			}
			continue
		}
		out.WriteByte(rawVal[idx])
	}
	return out.String()
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
		keyStr := string(bytes.TrimSpace(line[:eq]))
		valBytes := line[eq+1:]
		if keyStr == fdpKeyBody {
			bodyKey = strings.TrimSpace(string(valBytes))
			continue
		}
		dst[keyStr] = []string{UnescapeFDPLineVal(string(valBytes))}
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
	if stem == upperAcronymUI || stem == upperAcronymQR {
		return strings.ToUpper(stem) + controllerNameSuffix
	}
	return strings.ToUpper(stem[:1]) + stem[1:] + controllerNameSuffix
}

func scriptControllerName(script any) string {
	if script == nil {
		return ""
	}
	refType := reflect.TypeOf(script)
	for refType.Kind() == reflect.Pointer {
		refType = refType.Elem()
	}
	name := strings.TrimSuffix(refType.Name(), scriptStructSuffix)
	return formatControllerName(name)
}

func chunkControllerName(chunk templates.TemplateChunk) string {
	if chunk.ScriptData != nil {
		if ctrlName := scriptControllerName(chunk.ScriptData); ctrlName != "" {
			return ctrlName
		}
	}
	if chunk.TemplateData != nil {
		refType := reflect.TypeOf(chunk.TemplateData)
		for refType.Kind() == reflect.Pointer {
			refType = refType.Elem()
		}
		name := strings.TrimSuffix(refType.Name(), templateStructSuffix)
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
func WriteFDPFrames(writer io.Writer, frames ...FDPFrame) error {
	buf := templates.AcquireBuffer()
	defer templates.ReleaseBuffer(buf)

	expanded := make([]FDPFrame, 0, len(frames)+4)
	seenCtrl := make(map[string]bool, len(frames)+4)

	for _, frame := range frames {
		ctrl := chunkControllerName(frame.Chunk)
		expanded = append(expanded, frame)
		if ctrl != "" {
			seenCtrl[ctrl] = true
		}

		if frame.Chunk.TemplateData != nil {
			queue := collectChildChunks(frame.Chunk.TemplateData, nil)
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

	for frameIdx, frame := range expanded {
		if frameIdx > 0 {
			buf.WriteString(fdpFrameSep)
		}
		if ctrl := chunkControllerName(frame.Chunk); ctrl != "" {
			writeFDPLine(buf, fdpKeyController, ctrl)
		}
		if frame.Target != "" {
			writeFDPLine(buf, fdpKeyTarget, frame.Target)
		}
		if frame.Chunk.ScriptData != nil {
			writeFDPScriptValue(buf, frame.Chunk.ScriptData)
		}
		if len(frame.HTMLBody) > 0 {
			writeFDPLine(buf, fdpKeyBody, fdpBodyHTML)
			buf.WriteByte('\n')
			buf.Write(bytes.TrimRight(frame.HTMLBody, fdpLineBreakChars))
		} else if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] == '\n' {
			buf.Truncate(buf.Len() - 1)
		}
	}

	_, writeErr := writer.Write(buf.Bytes())
	return writeErr
}

func writeFDPScriptValue(buf *bytes.Buffer, script any) {
	refVal := reflect.ValueOf(script)
	for refVal.Kind() == reflect.Pointer || refVal.Kind() == reflect.Interface {
		if refVal.IsNil() {
			return
		}
		refVal = refVal.Elem()
	}
	if !refVal.IsValid() || refVal.Kind() != reflect.Struct {
		return
	}
	refType := refVal.Type()
	numFields := refType.NumField()
	for fieldIdx := 0; fieldIdx < numFields; fieldIdx++ {
		structField := refType.Field(fieldIdx)
		fieldVal := refVal.Field(fieldIdx)
		key := lowerFirstChar(structField.Name)
		switch fieldVal.Kind() {
		case reflect.Bool:
			writeFDPBoolLine(buf, key, fieldVal.Bool())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			writeFDPLine(buf, key, strconv.FormatInt(fieldVal.Int(), 10))
		case reflect.Float32, reflect.Float64:
			writeFDPLine(buf, key, fmt.Sprintf(floatFmt, fieldVal.Float()))
		case reflect.String:
			writeFDPLine(buf, key, fieldVal.String())
		}
	}
}

func lowerFirstChar(ident string) string {
	if ident == "" {
		return ""
	}
	firstByte := ident[0]
	if firstByte >= 'A' && firstByte <= 'Z' {
		return string(firstByte+('a'-'A')) + ident[1:]
	}
	return ident
}

func collectChildChunks(tplData any, queue []templates.TemplateChunk) []templates.TemplateChunk {
	refVal := reflect.ValueOf(tplData)
	for refVal.Kind() == reflect.Pointer || refVal.Kind() == reflect.Interface {
		if refVal.IsNil() {
			return queue
		}
		refVal = refVal.Elem()
	}
	if !refVal.IsValid() || refVal.Kind() != reflect.Struct {
		return queue
	}
	var singleChunks []templates.TemplateChunk
	refType := refVal.Type()
	numFields := refType.NumField()
	for fieldIdx := 0; fieldIdx < numFields; fieldIdx++ {
		structField := refType.Field(fieldIdx)
		if structField.PkgPath != "" {
			continue
		}
		fieldVal := refVal.Field(fieldIdx)
		if !fieldVal.CanInterface() {
			continue
		}
		switch typed := fieldVal.Interface().(type) {
		case templates.TemplateChunk:
			singleChunks = append(singleChunks, typed)
		case []templates.TemplateChunk:
			queue = append(queue, typed...)
		}
	}
	return append(queue, singleChunks...)
}
