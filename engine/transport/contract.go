package transport

import (
	"net/url"

	"mojra/engine/templates"
)

const (
	// ContentTypeFDP is the canonical MIME type for the Flat Data Protocol (FDP v2).
	ContentTypeFDP = "text/x-fdp;charset=UTF-8"
)

// ActionRequest carries decoded form values and origin metadata independent of HTTP or WASM transport.
type ActionRequest struct {
	Form       url.Values
	IsLoopback bool
}

// FormValue returns the first value for key from req.Form.
func (req *ActionRequest) FormValue(key string) string {
	if req == nil || req.Form == nil {
		return ""
	}
	return req.Form.Get(key)
}

// ActionFrame describes a rendered DOM target and its associated view chunks returned by an action handler.
type ActionFrame struct {
	Target string
	Chunks []templates.TemplateChunk
}

// FDPFrame represents a single controller-isolated frame in a multi-frame FDP payload.
type FDPFrame struct {
	Target   string
	HTMLBody []byte
	Chunk    templates.TemplateChunk
}
