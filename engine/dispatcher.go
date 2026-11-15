//go:build !(js && wasm)

package engine

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mojra/engine/templates"
)

const (
	headerContentType  = "Content-Type"
	headerSecFetchSite = "Sec-Fetch-Site"
	headerOrigin       = "Origin"
	headerReferer      = "Referer"
	fdpMimePrefix      = "text/x-fdp"
	wfpMimePrefix      = "text/x-wfp"
	secFetchSameOrigin = "same-origin"
	secFetchNone       = "none"
	originNull         = "null"
	hostLocalhost      = "localhost"
)

// ActionFrame describes a rendered DOM target and its associated view chunks returned by an action handler.
type ActionFrame struct {
	Target string
	Chunks []templates.TemplateChunk
}

func isFDPRequestPayload(contentType string, raw []byte) bool {
	lowerCT := strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(lowerCT, fdpMimePrefix) || strings.HasPrefix(lowerCT, wfpMimePrefix) {
		return true
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	return bytes.IndexByte(trimmed, '\n') >= 0 || !bytes.Contains(trimmed, []byte("&"))
}

// ParseActionRequest decodes an incoming action payload (FDP or URL-encoded form) into request.Form and request.PostForm.
func ParseActionRequest(request *http.Request) {
	if request == nil || request.Body == nil {
		return
	}
	request.Body = http.MaxBytesReader(nil, request.Body, 32<<20)
	contentType := request.Header.Get(headerContentType)
	buf := templates.AcquireBuffer()
	defer templates.ReleaseBuffer(buf)
	if _, err := io.Copy(buf, request.Body); err != nil {
		return
	}
	raw := buf.Bytes()
	if isFDPRequestPayload(contentType, raw) {
		vals := make(url.Values, 16)
		if DecodeFDPBytes(raw, vals) {
			request.Form = vals
			request.PostForm = vals
			return
		}
	}
	request.Body = io.NopCloser(bytes.NewReader(raw))
	_ = request.ParseForm()
}

// ValidateSameOriginAction verifies Sec-Fetch-Site, Origin, and Referer headers against request.Host.
func ValidateSameOriginAction(request *http.Request) bool {
	if secSite := strings.ToLower(strings.TrimSpace(request.Header.Get(headerSecFetchSite))); secSite != "" {
		if secSite != secFetchSameOrigin && secSite != secFetchNone {
			return false
		}
	}
	if origin := strings.TrimSpace(request.Header.Get(headerOrigin)); origin != "" {
		if strings.EqualFold(origin, originNull) {
			return false
		}
		parsedOrigin, err := url.Parse(origin)
		if err != nil || parsedOrigin.Host == "" || !strings.EqualFold(parsedOrigin.Host, request.Host) {
			return false
		}
	}
	if referer := strings.TrimSpace(request.Header.Get(headerReferer)); referer != "" {
		parsedReferer, err := url.Parse(referer)
		if err != nil || parsedReferer.Host == "" || !strings.EqualFold(parsedReferer.Host, request.Host) {
			return false
		}
	}
	return true
}

// IsLoopbackRequest reports whether request originated from a loopback address.
func IsLoopbackRequest(request *http.Request) bool {
	if request == nil {
		return true
	}
	remoteAddr := strings.TrimSpace(request.RemoteAddr)
	if remoteAddr == "" {
		return true
	}
	host := remoteAddr
	if splitHost, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = splitHost
	} else {
		host = strings.Trim(remoteAddr, "[]")
	}
	if strings.EqualFold(host, hostLocalhost) {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// WriteRenderedActionFrames renders the given ActionFrames and global script chunks into a multi-frame FDP HTTP response.
func WriteRenderedActionFrames(
	writer http.ResponseWriter,
	lang Lang,
	actionFrames []ActionFrame,
	globalScriptChunks []templates.TemplateChunk,
	loadedTemplates map[string]*templates.ParsedTemplate,
) {
	fdpFrames := make([]FDPFrame, 0, len(actionFrames)+len(globalScriptChunks))
	for _, actionFrame := range actionFrames {
		if actionFrame.Target == fdpTargetNone {
			for _, chunk := range actionFrame.Chunks {
				fdpFrames = append(fdpFrames, FDPFrame{
					Target: fdpTargetNone,
					Chunk:  chunk,
				})
			}
			continue
		}
		if len(actionFrame.Chunks) == 0 {
			fdpFrames = append(fdpFrames, FDPFrame{
				Target: actionFrame.Target,
			})
			continue
		}
		htmlBuf := templates.AcquireBuffer()
		tRender := time.Now()
		for _, chunk := range actionFrame.Chunks {
			tplName, tplData := chunk.Resolve()
			if tplName != "" && loadedTemplates != nil {
				ctx := NewViewContext(lang, tplData)
				if err := templates.RenderTemplateTo(htmlBuf, tplName, loadedTemplates, ctx); err != nil {
					templates.ReleaseBuffer(htmlBuf)
					http.Error(writer, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
		Active.MarkStep(StageActionRenderTpl, tRender, actionFrame.Target)
		bodyCopy := append([]byte(nil), htmlBuf.Bytes()...)
		templates.ReleaseBuffer(htmlBuf)

		fdpFrames = append(fdpFrames, FDPFrame{
			Target:   actionFrame.Target,
			HTMLBody: bodyCopy,
			Chunk:    actionFrame.Chunks[0],
		})
		for i := 1; i < len(actionFrame.Chunks); i++ {
			if actionFrame.Chunks[i].ScriptData != nil {
				fdpFrames = append(fdpFrames, FDPFrame{
					Target: fdpTargetNone,
					Chunk:  templates.TemplateChunk{ScriptData: actionFrame.Chunks[i].ScriptData},
				})
			}
		}
	}

	for _, scriptChunk := range globalScriptChunks {
		fdpFrames = append(fdpFrames, FDPFrame{
			Target: fdpTargetNone,
			Chunk:  scriptChunk,
		})
	}

	writer.Header().Set(headerContentType, ContentTypeFDP)
	_ = WriteFDPFrames(writer, fdpFrames...)
}
