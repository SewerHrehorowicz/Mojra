package transport

import "mojra/engine/templates"

const (
	// ContentTypeFDP is the canonical MIME type for the Flat Data Protocol (FDP v2).
	ContentTypeFDP = "text/x-fdp;charset=UTF-8"
)

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
